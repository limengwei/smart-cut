package pipeline

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"smart-cut/internal/adapter"
	"smart-cut/internal/eventbus"
	"smart-cut/internal/model"
)

// —— TranscribeStep ——

type TranscribeStep struct {
	whisper adapter.WhisperAdapter
	ffmpeg  adapter.FFmpegAdapter
	bus     *eventbus.EventBus
	opts    adapter.WhisperOptions
}
func NewTranscribeStep(whisper adapter.WhisperAdapter, ffmpeg adapter.FFmpegAdapter, opts adapter.WhisperOptions, bus *eventbus.EventBus) *TranscribeStep {
	return &TranscribeStep{whisper: whisper, ffmpeg: ffmpeg, opts: opts, bus: bus}
	return &TranscribeStep{whisper: whisper, ffmpeg: ffmpeg, opts: opts}
}

func (s *TranscribeStep) Name() string { return "transcribe" }

func (s *TranscribeStep) Run(ctx *Context, reporter ProgressReporter) error {
	reporter.Report("transcribe", "preparing audio", 0.1)

	mediaPath := ctx.Project.Media.Path

	// whisper.cpp 要求 16kHz 单声道 wav，先用 ffmpeg 预处理
	tmpDir, err := os.MkdirTemp("", "whisper-audio-")
	if err != nil {
		return fmt.Errorf("transcribe: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	wavPath := filepath.Join(tmpDir, "audio16k.wav")
	if err := s.ffmpeg.ExtractAudio16kWav(ctx.Cancel, mediaPath, wavPath); err != nil {
		return fmt.Errorf("transcribe: extract audio: %w", err)
	}

	reporter.Report("transcribe", "running whisper", 0.3)
	// 流式回调：实时进度 + 逐句字幕推送
	onProgress := func(p float64) {
		// whisper 进度映射到 0.3-0.95 区间（留 0.05 给收尾）
		reporter.Report("transcribe", fmt.Sprintf("whisper %d%%", int(p*100)), 0.3+p*0.65)
	}
	onSegment := func(seg model.Segment) {
		log.Printf("[Transcribe] 流式句段 #%d %dms-%dms: %s", seg.ID, seg.StartMs, seg.EndMs, seg.Text)
		if s.bus != nil {
			s.bus.Emit("transcript:segment", seg)
		}
	}

	transcript, err := s.whisper.TranscribeStream(ctx.Cancel, wavPath, s.opts, onProgress, onSegment)
	if err != nil {
		return fmt.Errorf("transcribe: %w", err)
	}


	reporter.Report("transcribe", "completed", 1.0)
	reporter.Done("transcribe", transcript)

	return nil
}

// —— AnalyzeStep ——

type AnalyzeStep struct {
	llm    adapter.LLMAdapter
	llmCfg model.LLMConfig
	rules  *SilenceDetector
}

func NewAnalyzeStep(llm adapter.LLMAdapter, llmCfg model.LLMConfig, silenceMs int) *AnalyzeStep {
	return &AnalyzeStep{
		llm:    llm,
		llmCfg: llmCfg,
		rules:  NewSilenceDetector(silenceMs),
	}
}

func (s *AnalyzeStep) Name() string { return "analyze" }

func (s *AnalyzeStep) Run(ctx *Context, reporter ProgressReporter) error {
	if ctx.Transcript == nil {
		return fmt.Errorf("analyze: transcript is nil")
	}
	log.Printf("[Step] analyze start: projectID=%s segments=%d silenceMs=%d", ctx.Project.ID, len(ctx.Transcript.Segments), s.rules.thresholdMs)

	reporter.Report("analyze", "detecting silence", 0.2)

	ruleCuts := s.rules.Detect(ctx.Transcript)
	log.Printf("[Step] analyze rule-based silence cuts: %d", len(ruleCuts))

	reporter.Report("analyze", "calling LLM", 0.4)

	llmReq := model.LLMAnalysisRequest{
		Language: ctx.Transcript.Language,
	}
	for _, seg := range ctx.Transcript.Segments {
		llmReq.Segments = append(llmReq.Segments, model.LLMSegment{
			ID:      seg.ID,
			StartMs: seg.StartMs,
			EndMs:   seg.EndMs,
			Text:    seg.Text,
		})
	}

	// 流式调用：用 token 计数估算进度（区间 0.4→0.8）
	// 估算预期 token：按输入文本总长度的 1.5 倍粗估（含 JSON 结构开销）
	estimatedTokens := 0
	for _, seg := range llmReq.Segments {
		estimatedTokens += len(seg.Text)
	}
	if estimatedTokens < 1 {
		estimatedTokens = 1
	}
	estimatedTokens = int(float64(estimatedTokens) * 1.5)
	receivedTokens := 0
	onToken := func(delta string) {
		receivedTokens += len(delta)
		ratio := float64(receivedTokens) / float64(estimatedTokens)
		if ratio > 1 {
			ratio = 1
		}
		reporter.Report("analyze", fmt.Sprintf("LLM 流式接收 %d/%d", receivedTokens, estimatedTokens), 0.4+ratio*0.4)
	}

	llmResult, err := s.llm.AnalyzeStream(ctx.Cancel, llmReq, s.llmCfg, onToken)
	if err != nil {
		reporter.Report("analyze", fmt.Sprintf("LLM failed, using rules only: %v", err), 0.6)
		log.Printf("[Step] analyze LLM failed (fallback to rules only): %v", err)
		llmResult = &model.LLMAnalysisResult{}
	}
	if llmResult != nil {
		log.Printf("[Step] analyze LLM result: removeSegmentIds=%d items=%d", len(llmResult.RemoveSegmentIDs), len(llmResult.Items))
	}

	reporter.Report("analyze", "merging results", 0.8)

	cutList := mergeAnalysisResults(ctx.Transcript, ruleCuts, llmResult)
	cutList.ProjectID = ctx.Project.ID
	cutList.Normalize()

	ctx.CutList = cutList

	reporter.Report("analyze", "completed", 1.0)
	reporter.Done("analyze", cutList)

	return nil
}

// —— ExportStep ——

type ExportStep struct {
	ffmpeg     adapter.FFmpegAdapter
	exportOpts model.ExportOptions
}

func NewExportStep(ffmpeg adapter.FFmpegAdapter, opts model.ExportOptions) *ExportStep {
	return &ExportStep{ffmpeg: ffmpeg, exportOpts: opts}
}

func (s *ExportStep) Name() string { return "export" }

func (s *ExportStep) Run(ctx *Context, reporter ProgressReporter) error {
	if ctx.CutList == nil {
		return fmt.Errorf("export: cutlist is nil")
	}

	reporter.Report("export", "preparing segments", 0.1)

	keepSegments := ctx.CutList.KeepSegments()
	if len(keepSegments) == 0 {
		return fmt.Errorf("export: no keep segments")
	}

	reporter.Report("export", "concatenating video", 0.3)

	sourcePath := ctx.Project.Media.Path
	outPath := s.exportOpts.OutputPath
	if outPath == "" {
		outPath = filepath.Join(ctx.Project.WorkDir, "export.mp4")
	}

	var err error
	if s.exportOpts.Mode == model.ExportLossless {
		err = s.ffmpeg.ConcatLossless(ctx.Cancel, keepSegments, sourcePath, outPath)
	} else {
		err = s.ffmpeg.ConcatReencode(ctx.Cancel, keepSegments, sourcePath, outPath, model.EncodeOpts{
			VideoCodec: "libx264",
			AudioCodec: "aac",
			Crf:        23,
			Preset:     "medium",
		})
	}
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}

	ctx.ExportPath = outPath

	reporter.Report("export", "completed", 1.0)
	reporter.Done("export", outPath)

	return nil
}

// —— SubtitleStep (MVP 占位) ——

type SubtitleStep struct{}

func NewSubtitleStep() *SubtitleStep {
	return &SubtitleStep{}
}

func (s *SubtitleStep) Name() string { return "subtitle" }

func (s *SubtitleStep) Run(ctx *Context, reporter ProgressReporter) error {
	reporter.Report("subtitle", "skipped (MVP)", 1.0)
	return nil
}

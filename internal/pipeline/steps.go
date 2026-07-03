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

	ctx.Transcript = transcript

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

	sourcePath := ctx.Project.Media.Path
	media := ctx.Project.Media

	outPath := s.exportOpts.OutputPath
	if outPath == "" {
		outPath = filepath.Join(ctx.Project.WorkDir, "export.mp4")
	}

	// 按导出模式分支：
	// - lossless（默认）：逐段提取（内嵌音频淡入淡出 + HDR tone map + 竖屏缩放）+ concat demuxer 真无损拼接
	// - reencode：单 filter_complex 重编码拼接（保留兼容旧路径）
	if s.exportOpts.Mode == model.ExportReencode {
		reporter.Report("export", "concatenating (reencode)", 0.3)
		if err := s.ffmpeg.ConcatReencode(ctx.Cancel, keepSegments, sourcePath, outPath, model.EncodeOpts{
			VideoCodec: "libx264",
			AudioCodec: "aac",
			Crf:        23,
			Preset:     "medium",
		}); err != nil {
			return fmt.Errorf("export: reencode concat: %w", err)
		}
		ctx.ExportPath = outPath
		reporter.Report("export", "completed", 1.0)
		reporter.Done("export", outPath)
		return nil
	}

	// lossless 路径：逐段提取 + 字幕 overlay + concat demuxer
	tmpDir := filepath.Join(ctx.Project.WorkDir, "cuts")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return fmt.Errorf("export: create cuts dir: %w", err)
	}

	reporter.Report("export", fmt.Sprintf("extracting %d segments", len(keepSegments)), 0.2)

	var segPaths []string
	for i, seg := range keepSegments {
		segID := fmt.Sprintf("%03d", i+1)
		startSec := float64(seg.StartMs) / 1000.0
		endSec := float64(seg.EndMs) / 1000.0
		segOutPath := filepath.Join(tmpDir, fmt.Sprintf("keep_%s.mp4", segID))

		if err := s.ffmpeg.ExtractSegment(ctx.Cancel, sourcePath, startSec, endSec, media, segOutPath); err != nil {
			return fmt.Errorf("export: extract segment %d: %w", i+1, err)
		}

		if ctx.SubtitleClips != nil {
			if subtitlePath, ok := ctx.SubtitleClips[segID]; ok {
				overlayPath := filepath.Join(tmpDir, fmt.Sprintf("keep_%s_overlay.mp4", segID))
				if err := s.ffmpeg.OverlaySegment(ctx.Cancel, segOutPath, subtitlePath, overlayPath); err != nil {
					return fmt.Errorf("export: overlay segment %d: %w", i+1, err)
				}
				segOutPath = overlayPath
			}
		}

		segPaths = append(segPaths, segOutPath)

		reporter.Report("export", fmt.Sprintf("extracted %d/%d", i+1, len(keepSegments)), 0.2+0.5*float64(i+1)/float64(len(keepSegments)))
	}

	reporter.Report("export", "concatenating", 0.7)

	if err := s.ffmpeg.ConcatDemuxer(ctx.Cancel, segPaths, outPath); err != nil {
		return fmt.Errorf("export: concat: %w", err)
	}

	ctx.ExportPath = outPath

	reporter.Report("export", "completed", 1.0)
	reporter.Done("export", outPath)

	return nil
}

// —— SubtitleStep（整句高亮字幕渲染）——

type SubtitleStep struct {
	remotion adapter.RemotionAdapter
}

func NewSubtitleStep(remotion adapter.RemotionAdapter) *SubtitleStep {
	return &SubtitleStep{remotion: remotion}
}

func (s *SubtitleStep) Name() string { return "subtitle" }

func (s *SubtitleStep) Run(ctx *Context, reporter ProgressReporter) error {
	if ctx.Transcript == nil || len(ctx.Transcript.Segments) == 0 {
		reporter.Report("subtitle", "no transcript segments, skipping", 1.0)
		return nil
	}
	if ctx.CutList == nil {
		reporter.Report("subtitle", "no cutlist, skipping", 1.0)
		return nil
	}

	keepSegments := ctx.CutList.KeepSegments()
	if len(keepSegments) == 0 {
		reporter.Report("subtitle", "no keep segments", 1.0)
		return nil
	}

	reporter.Report("subtitle", fmt.Sprintf("rendering %d segments", len(keepSegments)), 0.0)

	clipDir := filepath.Join(ctx.Project.WorkDir, "subtitle_clips")
	if err := os.MkdirAll(clipDir, 0755); err != nil {
		return fmt.Errorf("subtitle: create clip dir: %w", err)
	}

	clips := make(map[string]string)
	completed := 0
	total := len(keepSegments)

	for i, seg := range keepSegments {
		segID := fmt.Sprintf("%03d", i+1)

		// 筛出落在本 keep 段内的 transcript 句段，并偏移为段内相对时间
		var relSegs []model.Segment
		for _, ts := range ctx.Transcript.Segments {
			if ts.EndMs <= seg.StartMs || ts.StartMs >= seg.EndMs {
				continue // 不相交
			}
			start := ts.StartMs - seg.StartMs
			if start < 0 {
				start = 0
			}
			end := ts.EndMs - seg.StartMs
			if end > seg.EndMs-seg.StartMs {
				end = seg.EndMs - seg.StartMs
			}
			relSegs = append(relSegs, model.Segment{
				ID:      ts.ID,
				StartMs: start,
				EndMs:   end,
				Text:    ts.Text,
			})
		}

		req := adapter.SubtitleSegmentRequest{
			SegmentID: segID,
			StartMs:   seg.StartMs,
			EndMs:     seg.EndMs,
			Segments:  relSegs,
			Style:     ctx.Project.Settings.SubtitleStyle,
			Width:     ctx.Project.Media.Width,
			Height:    ctx.Project.Media.Height,
			Fps:       ctx.Project.Media.Fps,
			OutputDir: clipDir,
		}

		// 失败回退：记日志但不中断，该段跳过字幕（见 spec 6.2 第 3 条）
		clipPath, err := s.remotion.RenderSegment(ctx.Cancel, req, func(ratio float64) {
			overall := (float64(completed) + ratio) / float64(total)
			reporter.Report("subtitle", fmt.Sprintf("rendering %d/%d", i+1, total), overall)
		})
		if err != nil {
			log.Printf("[Subtitle] 段 %d 渲染失败，跳过该段字幕: %v", i+1, err)
			completed++
			continue
		}
		clips[segID] = clipPath
		completed++
		reporter.Report("subtitle", fmt.Sprintf("rendered %d/%d", i+1, total), float64(completed)/float64(total))
	}

	ctx.SubtitleClips = clips
	reporter.Report("subtitle", fmt.Sprintf("completed (%d/%d succeeded)", len(clips), total), 1.0)
	return nil
}

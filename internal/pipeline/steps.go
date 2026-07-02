package pipeline

import (
	"fmt"
	"path/filepath"

	"smart-cut/internal/adapter"
	"smart-cut/internal/model"
)

// —— TranscribeStep ——

type TranscribeStep struct {
	whisper adapter.WhisperAdapter
	ffmpeg  adapter.FFmpegAdapter
	opts    adapter.WhisperOptions
}

func NewTranscribeStep(whisper adapter.WhisperAdapter, ffmpeg adapter.FFmpegAdapter, opts adapter.WhisperOptions) *TranscribeStep {
	return &TranscribeStep{whisper: whisper, ffmpeg: ffmpeg, opts: opts}
}

func (s *TranscribeStep) Name() string { return "transcribe" }

func (s *TranscribeStep) Run(ctx *Context, reporter ProgressReporter) error {
	reporter.Report("transcribe", "preparing audio", 0.1)

	mediaPath := ctx.Project.Media.Path

	reporter.Report("transcribe", "running whisper", 0.3)

	transcript, err := s.whisper.Transcribe(ctx.Cancel, mediaPath, s.opts)
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

	reporter.Report("analyze", "detecting silence", 0.2)

	ruleCuts := s.rules.Detect(ctx.Transcript)

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

	llmResult, err := s.llm.Analyze(ctx.Cancel, llmReq, s.llmCfg)
	if err != nil {
		reporter.Report("analyze", fmt.Sprintf("LLM failed, using rules only: %v", err), 0.6)
		llmResult = &model.LLMAnalysisResult{}
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

package pipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smart-cut/internal/adapter"
	"smart-cut/internal/model"
)

type mockWhisperAdapter struct {
	transcript *model.Transcript
	err        error
}

func (m *mockWhisperAdapter) Transcribe(ctx context.Context, mediaPath string, opts adapter.WhisperOptions) (*model.Transcript, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.transcript, nil
}

func (m *mockWhisperAdapter) TranscribeStream(ctx context.Context, mediaPath string, opts adapter.WhisperOptions,
	onProgress func(progress float64), onSegment func(seg model.Segment)) (*model.Transcript, error) {
	return m.Transcribe(ctx, mediaPath, opts)
}

type mockLLMAdapter struct {
	result *model.LLMAnalysisResult
	err    error
}

func (m *mockLLMAdapter) Analyze(ctx context.Context, req model.LLMAnalysisRequest, cfg model.LLMConfig) (*model.LLMAnalysisResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func (m *mockLLMAdapter) AnalyzeStream(ctx context.Context, req model.LLMAnalysisRequest, cfg model.LLMConfig, onToken func(delta string)) (*model.LLMAnalysisResult, error) {
	return m.Analyze(ctx, req, cfg)
}

func TestAnalyzeStep_Run_Success(t *testing.T) {
	transcript := &model.Transcript{
		Language: "zh",
		Segments: []model.Segment{
			{ID: 0, StartMs: 0, EndMs: 1000, Text: "嗯"},
			{ID: 1, StartMs: 1000, EndMs: 2000, Text: "大家好"},
		},
	}

	llmResult := &model.LLMAnalysisResult{
		RemoveSegmentIDs: []int{0},
		Items: []model.LLMAnalysisItem{
			{SegmentID: 0, Reason: model.ReasonFiller, Confidence: 0.9, Note: "语气词"},
		},
	}

	step := NewAnalyzeStep(&mockLLMAdapter{result: llmResult}, model.LLMConfig{}, 800)

	ctx := &Context{
		Project:    &model.Project{ID: "p1"},
		Transcript: transcript,
		Cancel:     context.Background(),
	}

	reporter := &mockReporter{}
	err := step.Run(ctx, reporter)

	require.NoError(t, err)
	require.NotNil(t, ctx.CutList)
	assert.Equal(t, "p1", ctx.CutList.ProjectID)
	assert.True(t, len(ctx.CutList.Segments) >= 2)
}

func TestAnalyzeStep_Run_NilTranscript(t *testing.T) {
	step := NewAnalyzeStep(&mockLLMAdapter{}, model.LLMConfig{}, 800)

	ctx := &Context{
		Project: &model.Project{ID: "p1"},
		Cancel:  context.Background(),
	}

	err := step.Run(ctx, &mockReporter{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transcript is nil")
}

func TestAnalyzeStep_Run_LLMFailure_UsesRulesOnly(t *testing.T) {
	transcript := &model.Transcript{
		Language: "zh",
		Segments: []model.Segment{
			{ID: 0, StartMs: 0, EndMs: 1000},
			{ID: 1, StartMs: 2500, EndMs: 3000},
		},
	}

	step := NewAnalyzeStep(
		&mockLLMAdapter{err: context.DeadlineExceeded},
		model.LLMConfig{},
		800,
	)

	ctx := &Context{
		Project:    &model.Project{ID: "p1"},
		Transcript: transcript,
		Cancel:     context.Background(),
	}

	err := step.Run(ctx, &mockReporter{})
	require.NoError(t, err)
	require.NotNil(t, ctx.CutList)

	hasSilence := false
	for _, seg := range ctx.CutList.Segments {
		if seg.Reason == model.ReasonSilence {
			hasSilence = true
			break
		}
	}
	assert.True(t, hasSilence)
}

func TestExportStep_Run_NoCutList(t *testing.T) {
	step := NewExportStep(nil, model.ExportOptions{})

	ctx := &Context{
		Project: &model.Project{ID: "p1"},
		Cancel:  context.Background(),
	}

	err := step.Run(ctx, &mockReporter{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cutlist is nil")
}

func TestExportStep_Run_NoKeepSegments(t *testing.T) {
	step := NewExportStep(nil, model.ExportOptions{})

	ctx := &Context{
		Project: &model.Project{ID: "p1"},
		CutList: &model.CutList{
			Segments: []model.CutSegment{
				{Decision: model.CutRemove, StartMs: 0, EndMs: 1000},
			},
		},
		Cancel: context.Background(),
	}

	err := step.Run(ctx, &mockReporter{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no keep segments")
}

func TestSubtitleStep_Run_MVPSkips(t *testing.T) {
	step := NewSubtitleStep(nil)
	ctx := &Context{Cancel: context.Background()}

	err := step.Run(ctx, &mockReporter{})
	require.NoError(t, err)
}

func TestTranscribeStep_Name(t *testing.T) {
	step := NewTranscribeStep(nil, nil, adapter.WhisperOptions{}, nil)
	assert.Equal(t, "transcribe", step.Name())
}

type mockFFmpeg struct {
	extractErr           error
	overlayErr           error
	concatErr            error
	overlayWithOverlayErr error
	overlayPIPErr         error
	extractedPaths       []string
	overlayedPaths       []string
}

func (m *mockFFmpeg) Probe(ctx context.Context, path string) (*model.MediaFile, error) {
	return &model.MediaFile{Width: 1920, Height: 1080, Fps: 30, HasAudio: true}, nil
}

func (m *mockFFmpeg) ExtractWaveform(ctx context.Context, mediaPath, outPng string) error {
	return nil
}

func (m *mockFFmpeg) ExtractAudio16kWav(ctx context.Context, mediaPath, outWav string) error {
	return nil
}

func (m *mockFFmpeg) ExtractWaveformPeaks(ctx context.Context, mediaPath string, durationMs int64, buckets int) (*model.WaveformPeaks, error) {
	return nil, nil
}

func (m *mockFFmpeg) ExtractSegment(ctx context.Context, sourcePath string, segStartSec, segEndSec float64, media model.MediaFile, outPath string) error {
	m.extractedPaths = append(m.extractedPaths, outPath)
	return m.extractErr
}

func (m *mockFFmpeg) ConcatDemuxer(ctx context.Context, segmentPaths []string, outPath string) error {
	return m.concatErr
}

func (m *mockFFmpeg) ConcatReencode(ctx context.Context, segments []model.KeepSegment, sourcePath, outPath string, opts model.EncodeOpts) error {
	return nil
}

func (m *mockFFmpeg) MuxSubtitle(ctx context.Context, videoPath, subtitleClipPath, outPath string) error {
	return nil
}

func (m *mockFFmpeg) OverlaySegment(ctx context.Context, videoPath, subtitlePath, outPath string) error {
	m.overlayedPaths = append(m.overlayedPaths, outPath)
	return m.overlayErr
}

func (m *mockFFmpeg) OverlaySegmentWithOverlay(ctx context.Context, videoPath, subtitlePath, overlayPath, outPath string) error {
	return m.overlayWithOverlayErr
}

func (m *mockFFmpeg) OverlaySegmentPIP(ctx context.Context, videoPath, overlayPath, outPath string) error {
	return m.overlayPIPErr
}

func TestExportStep_Run_WithSubtitleClips(t *testing.T) {
	mock := &mockFFmpeg{}
	step := NewExportStep(mock, model.ExportOptions{Mode: model.ExportLossless})

	ctx := &Context{
		Project: &model.Project{
			ID:      "p1",
			WorkDir: t.TempDir(),
			Media:   model.MediaFile{Width: 1920, Height: 1080, Fps: 30, HasAudio: true, Path: "/fake/source.mp4"},
		},
		CutList: &model.CutList{
			Segments: []model.CutSegment{
				{ID: "1", Decision: model.CutKeep, StartMs: 0, EndMs: 2000},
			},
		},
		SubtitleClips: map[string]string{
			"001": "/fake/subtitle_001.mp4",
		},
		Cancel: context.Background(),
	}

	err := step.Run(ctx, &mockReporter{})
	require.NoError(t, err)

	assert.Len(t, mock.extractedPaths, 1)
	assert.Len(t, mock.overlayedPaths, 1)
}

func TestExportStep_Run_NoSubtitleClips_NoOverlay(t *testing.T) {
	mock := &mockFFmpeg{}
	step := NewExportStep(mock, model.ExportOptions{Mode: model.ExportLossless})

	ctx := &Context{
		Project: &model.Project{
			ID:      "p1",
			WorkDir: t.TempDir(),
			Media:   model.MediaFile{Width: 1920, Height: 1080, Fps: 30, HasAudio: true, Path: "/fake/source.mp4"},
		},
		CutList: &model.CutList{
			Segments: []model.CutSegment{
				{ID: "1", Decision: model.CutKeep, StartMs: 0, EndMs: 2000},
			},
		},
		SubtitleClips: nil,
		Cancel:        context.Background(),
	}

	err := step.Run(ctx, &mockReporter{})
	require.NoError(t, err)

	assert.Len(t, mock.extractedPaths, 1)
	assert.Len(t, mock.overlayedPaths, 0)
}

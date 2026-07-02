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
	step := NewSubtitleStep()
	ctx := &Context{Cancel: context.Background()}

	err := step.Run(ctx, &mockReporter{})
	require.NoError(t, err)
}

func TestTranscribeStep_Name(t *testing.T) {
	step := NewTranscribeStep(nil, nil, adapter.WhisperOptions{}, nil)
	assert.Equal(t, "transcribe", step.Name())
}

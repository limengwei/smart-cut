package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"smart-cut/internal/model"
)

func TestSilenceDetector_DetectsGap(t *testing.T) {
	transcript := &model.Transcript{
		Segments: []model.Segment{
			{ID: 0, StartMs: 0, EndMs: 1000},
			{ID: 1, StartMs: 1000, EndMs: 2000},
			{ID: 2, StartMs: 3000, EndMs: 4000},
		},
	}

	detector := NewSilenceDetector(800)
	cuts := detector.Detect(transcript)

	assert.Len(t, cuts, 1)
	assert.Equal(t, int64(2000), cuts[0].StartMs)
	assert.Equal(t, int64(3000), cuts[0].EndMs)
	assert.Equal(t, model.CutRemove, cuts[0].Decision)
	assert.Equal(t, model.ReasonSilence, cuts[0].Reason)
}

func TestSilenceDetector_NoGapBelowThreshold(t *testing.T) {
	transcript := &model.Transcript{
		Segments: []model.Segment{
			{ID: 0, StartMs: 0, EndMs: 1000},
			{ID: 1, StartMs: 1500, EndMs: 2000},
		},
	}

	detector := NewSilenceDetector(800)
	cuts := detector.Detect(transcript)

	assert.Empty(t, cuts)
}

func TestSilenceDetector_SingleSegment(t *testing.T) {
	transcript := &model.Transcript{
		Segments: []model.Segment{{ID: 0, StartMs: 0, EndMs: 1000}},
	}

	detector := NewSilenceDetector(800)
	cuts := detector.Detect(transcript)

	assert.Empty(t, cuts)
}

func TestSilenceDetector_NilTranscript(t *testing.T) {
	detector := NewSilenceDetector(800)
	cuts := detector.Detect(nil)

	assert.Empty(t, cuts)
}

func TestSilenceDetector_DefaultThreshold(t *testing.T) {
	detector := NewSilenceDetector(0)
	assert.Equal(t, int64(800), detector.thresholdMs)
}

func TestMergeAnalysisResults_OnlyRules(t *testing.T) {
	transcript := &model.Transcript{
		Segments: []model.Segment{
			{ID: 0, StartMs: 0, EndMs: 1000},
			{ID: 1, StartMs: 2000, EndMs: 3000},
		},
	}

	ruleCuts := []model.CutSegment{
		{ID: "silence-1", StartMs: 1000, EndMs: 2000, Decision: model.CutRemove, Reason: model.ReasonSilence},
	}

	cutList := mergeAnalysisResults(transcript, ruleCuts, nil)

	assert.Len(t, cutList.Segments, 3)
	assert.Equal(t, model.CutKeep, cutList.Segments[0].Decision)
	assert.Equal(t, model.CutRemove, cutList.Segments[1].Decision)
	assert.Equal(t, model.CutKeep, cutList.Segments[2].Decision)
}

func TestMergeAnalysisResults_OnlyLLM(t *testing.T) {
	transcript := &model.Transcript{
		Segments: []model.Segment{
			{ID: 0, StartMs: 0, EndMs: 1000, Text: "嗯"},
			{ID: 1, StartMs: 1000, EndMs: 2000, Text: "大家好"},
		},
	}

	llmResult := &model.LLMAnalysisResult{
		RemoveSegmentIDs: []int{0},
		Items: []model.LLMAnalysisItem{
			{SegmentID: 0, Reason: model.ReasonFiller, Confidence: 0.95, Note: "语气词"},
		},
	}

	cutList := mergeAnalysisResults(transcript, nil, llmResult)

	assert.Len(t, cutList.Segments, 2)
	assert.Equal(t, model.CutRemove, cutList.Segments[0].Decision)
	assert.Equal(t, model.ReasonFiller, cutList.Segments[0].Reason)
	assert.Equal(t, model.CutKeep, cutList.Segments[1].Decision)
}

func TestMergeAnalysisResults_NoRemoves(t *testing.T) {
	transcript := &model.Transcript{
		Segments: []model.Segment{
			{ID: 0, StartMs: 0, EndMs: 5000},
		},
	}

	cutList := mergeAnalysisResults(transcript, nil, &model.LLMAnalysisResult{})

	assert.Len(t, cutList.Segments, 1)
	assert.Equal(t, model.CutKeep, cutList.Segments[0].Decision)
	assert.Equal(t, int64(0), cutList.Segments[0].StartMs)
	assert.Equal(t, int64(5000), cutList.Segments[0].EndMs)
}

func TestFillKeepSegments_FillsCorrectly(t *testing.T) {
	cutList := &model.CutList{
		Segments: []model.CutSegment{
			{StartMs: 1000, EndMs: 2000, Decision: model.CutRemove},
		},
	}
	transcript := &model.Transcript{
		Segments: []model.Segment{
			{StartMs: 0, EndMs: 3000},
		},
	}

	result := fillKeepSegments(cutList, transcript)

	assert.Len(t, result.Segments, 3)
	assert.Equal(t, model.CutKeep, result.Segments[0].Decision)
	assert.Equal(t, int64(0), result.Segments[0].StartMs)
	assert.Equal(t, model.CutRemove, result.Segments[1].Decision)
	assert.Equal(t, model.CutKeep, result.Segments[2].Decision)
	assert.Equal(t, int64(3000), result.Segments[2].EndMs)
}

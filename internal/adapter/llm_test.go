package adapter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smart-cut/internal/model"
)

func TestParseLLMResponse_ValidJSON(t *testing.T) {
	content := `{
		"removeSegmentIds": [1, 3],
		"items": [
			{"segmentId": 1, "reason": "filler", "confidence": 0.95, "note": "语气词'嗯'"},
			{"segmentId": 3, "reason": "dup_or_error", "confidence": 0.8, "note": "重复了上一句"}
		]
	}`

	result, err := parseLLMResponse(content)
	require.NoError(t, err)

	assert.Equal(t, []int{1, 3}, result.RemoveSegmentIDs)
	assert.Len(t, result.Items, 2)
	assert.Equal(t, 1, result.Items[0].SegmentID)
	assert.Equal(t, model.ReasonFiller, result.Items[0].Reason)
	assert.Equal(t, 0.95, result.Items[0].Confidence)
	assert.Equal(t, model.ReasonDupOrError, result.Items[1].Reason)
}

func TestParseLLMResponse_MarkdownWrapped(t *testing.T) {
	content := "```json\n" + `{
		"removeSegmentIds": [2],
		"items": [
			{"segmentId": 2, "reason": "silence", "confidence": 0.9, "note": "停顿过长"}
		]
	}` + "\n```"

	result, err := parseLLMResponse(content)
	require.NoError(t, err)
	assert.Equal(t, []int{2}, result.RemoveSegmentIDs)
	assert.Len(t, result.Items, 1)
}

func TestParseLLMResponse_EmptyItems(t *testing.T) {
	content := `{"removeSegmentIds": [], "items": []}`

	result, err := parseLLMResponse(content)
	require.NoError(t, err)
	assert.Empty(t, result.RemoveSegmentIDs)
	assert.Empty(t, result.Items)
}

func TestParseLLMResponse_InvalidJSON(t *testing.T) {
	content := `这不是JSON`

	_, err := parseLLMResponse(content)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "llm parse response")
}

func TestBuildSystemPrompt_ContainsInstructions(t *testing.T) {
	req := model.LLMAnalysisRequest{Language: "zh"}
	prompt := buildSystemPrompt(req)

	assert.Contains(t, prompt, "filler")
	assert.Contains(t, prompt, "silence")
	assert.Contains(t, prompt, "dup_or_error")
	assert.Contains(t, prompt, "JSON")
}

func TestBuildUserPrompt_ContainsSegments(t *testing.T) {
	req := model.LLMAnalysisRequest{
		Language: "zh",
		Segments: []model.LLMSegment{
			{ID: 0, StartMs: 0, EndMs: 1000, Text: "大家好"},
			{ID: 1, StartMs: 1000, EndMs: 2000, Text: "嗯今天聊聊"},
		},
	}

	prompt := buildUserPrompt(req)

	assert.Contains(t, prompt, "ID:0")
	assert.Contains(t, prompt, "大家好")
	assert.Contains(t, prompt, "ID:1")
	assert.Contains(t, prompt, "嗯今天聊聊")
	assert.Contains(t, prompt, "zh")
}

func TestCleanJSONResponse_PlainJSON(t *testing.T) {
	input := `{"key": "value"}`
	result := cleanJSONResponse(input)
	assert.Equal(t, `{"key": "value"}`, result)
}

func TestCleanJSONResponse_MarkdownBlock(t *testing.T) {
	input := "```json\n{\"key\": \"value\"}\n```"
	result := cleanJSONResponse(input)
	assert.Contains(t, result, `"key"`)
	assert.NotContains(t, result, "```json")
}

func TestLLMAnalyze_MissingConfig(t *testing.T) {
	adapter := NewLLMAdapter()
	req := model.LLMAnalysisRequest{Language: "zh"}

	_, err := adapter.Analyze(nil, req, model.LLMConfig{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

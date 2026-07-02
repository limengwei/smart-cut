package adapter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseWhisperJSON_ValidOutput(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "whisper_output.json"))
	require.NoError(t, err)

	transcript, err := parseWhisperJSON(data)
	require.NoError(t, err)

	assert.Len(t, transcript.Segments, 2)

	// 第一段
	assert.Equal(t, 0, transcript.Segments[0].ID)
	assert.Equal(t, int64(0), transcript.Segments[0].StartMs)
	assert.Equal(t, int64(1500), transcript.Segments[0].EndMs)
	assert.Equal(t, "大家好今天来聊聊", transcript.Segments[0].Text)

	// 第二段
	assert.Equal(t, 1, transcript.Segments[1].ID)
	assert.Equal(t, int64(1500), transcript.Segments[1].StartMs)
	assert.Equal(t, int64(3000), transcript.Segments[1].EndMs)
	assert.Equal(t, "嗯怎么用AI剪辑视频", transcript.Segments[1].Text)
}

func TestParseWhisperJSON_EmptyTranscription(t *testing.T) {
	data := []byte(`{"transcription": []}`)

	transcript, err := parseWhisperJSON(data)
	require.NoError(t, err)
	assert.Empty(t, transcript.Segments)
}

func TestParseWhisperJSON_InvalidJSON(t *testing.T) {
	data := []byte(`{invalid json}`)

	_, err := parseWhisperJSON(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "whisper parse json")
}

func TestParseWhisperJSON_StripsLeadingSpace(t *testing.T) {
	data := []byte(`{
		"transcription": [
			{
				"timestamps": {"from": "00:00:00,000", "to": "00:00:01,000"},
				"offsets": {"from": 0, "to": 1000},
				"text": " 没有前导空格的文本"
			}
		]
	}`)

	transcript, err := parseWhisperJSON(data)
	require.NoError(t, err)
	assert.Equal(t, "没有前导空格的文本", transcript.Segments[0].Text)
}

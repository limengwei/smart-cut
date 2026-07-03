package adapter

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseWorkerStdout_Done(t *testing.T) {
	input := `{"type":"progress","progress":0.5}
{"type":"progress","progress":1.0}
{"type":"done","outputPath":"/tmp/sub_x.mp4"}
`
	var lastProgress float64
	path, err := parseWorkerStdout(strings.NewReader(input), func(r float64) { lastProgress = r })
	require.NoError(t, err)
	assert.Equal(t, "/tmp/sub_x.mp4", path)
	assert.Equal(t, 1.0, lastProgress)
}

func TestParseWorkerStdout_Error(t *testing.T) {
	input := `{"type":"error","message":"render failed: composition not found"}`
	_, err := parseWorkerStdout(strings.NewReader(input), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "render failed")
}

func TestParseWorkerStdout_SkipsNonJSON(t *testing.T) {
	input := `Remotion render starting...
{"type":"progress","progress":0.3}
some debug line
{"type":"done","outputPath":"/out/y.mp4"}
`
	path, err := parseWorkerStdout(strings.NewReader(input), nil)
	require.NoError(t, err)
	assert.Equal(t, "/out/y.mp4", path)
}

func TestParseWorkerStdout_NoDoneReturnsEmpty(t *testing.T) {
	input := `{"type":"progress","progress":0.5}`
	path, err := parseWorkerStdout(strings.NewReader(input), nil)
	require.NoError(t, err)
	assert.Equal(t, "", path)
}

func TestParseWorkerStdout_Empty(t *testing.T) {
	path, err := parseWorkerStdout(strings.NewReader(""), nil)
	require.NoError(t, err)
	assert.Equal(t, "", path)
}

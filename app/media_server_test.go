package app

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMediaServer_ServesRegisteredFile(t *testing.T) {
	ms, err := NewMediaServer()
	require.NoError(t, err)
	defer ms.Shutdown()

	dir := t.TempDir()
	content := []byte("fake-media-content")
	path := filepath.Join(dir, "video.mp4")
	require.NoError(t, os.WriteFile(path, content, 0644))

	ms.Register("proj-1", path)
	url := ms.URL("proj-1")

	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, content, body)
}

func TestMediaServer_UnknownProjectReturns404(t *testing.T) {
	ms, err := NewMediaServer()
	require.NoError(t, err)
	defer ms.Shutdown()

	resp, err := http.Get(ms.URL("nope"))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

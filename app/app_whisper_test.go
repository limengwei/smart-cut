package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveWhisperModel_DirFindsBin(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ggml-medium.bin"), []byte("x"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ggml-base.bin"), []byte("x"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("x"), 0644))

	got, err := resolveWhisperModel(dir)
	require.NoError(t, err)
	// base 优先级高于 medium
	assert.Equal(t, filepath.Join(dir, "ggml-base.bin"), got)
}

func TestResolveWhisperModel_EmptyDirReturnsError(t *testing.T) {
	dir := t.TempDir()
	_, err := resolveWhisperModel(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未找到")
}

func TestResolveWhisperModel_DirectBinFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "model.bin")
	require.NoError(t, os.WriteFile(f, []byte("x"), 0644))

	got, err := resolveWhisperModel(f)
	require.NoError(t, err)
	assert.Equal(t, f, got)
}

func TestResolveWhisperModel_NonBinFileRejected(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "model.txt")
	require.NoError(t, os.WriteFile(f, []byte("x"), 0644))

	_, err := resolveWhisperModel(f)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不是 .bin")
}

func TestResolveWhisperModel_NotExistReturnsError(t *testing.T) {
	_, err := resolveWhisperModel(filepath.Join(t.TempDir(), "nope"))
	assert.Error(t, err)
}

package config

import (
	"os"
	"path/filepath"
	"testing"

	"smart-cut/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_FileNotExist_ReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	mgr := NewConfigManager(dir)

	settings, err := mgr.Load()
	require.NoError(t, err)
	assert.NotNil(t, settings)
	assert.Equal(t, "dark", settings.Theme)
	assert.Equal(t, "gpt-4o-mini", settings.DefaultLLM.Model)
	assert.Equal(t, "https://api.openai.com/v1", settings.DefaultLLM.BaseURL)
	assert.Empty(t, settings.Binaries)
}

func TestSave_And_Load_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	mgr := NewConfigManager(dir)

	original := &model.GlobalSettings{
		Binaries: map[string]string{
			"ffmpeg":      "/usr/local/bin/ffmpeg",
			"whisper-cli": "/opt/whisper/whisper-cli",
		},
		WhisperModelDir: "/opt/whisper/models",
		DefaultLLM: model.LLMConfig{
			BaseURL: "https://api.deepseek.com/v1",
			APIKey:  "sk-test-123",
			Model:   "deepseek-chat",
		},
		Theme: "light",
	}

	err := mgr.Save(original)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(dir, "config.json"))
	require.NoError(t, err)

	loaded, err := mgr.Load()
	require.NoError(t, err)
	assert.Equal(t, original, loaded)
}

func TestSave_Nil_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	mgr := NewConfigManager(dir)

	err := mgr.Save(nil)
	assert.Error(t, err)
}

func TestSave_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "deep")
	mgr := NewConfigManager(dir)

	err := mgr.Save(defaultSettings())
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(dir, "config.json"))
	require.NoError(t, err)
}

func TestLoad_InvalidJSON_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	err := os.WriteFile(path, []byte("{invalid json"), 0644)
	require.NoError(t, err)

	mgr := NewConfigManager(dir)
	_, err = mgr.Load()
	assert.Error(t, err)
}

func TestNewConfigManager_EmptyDir_UsesHome(t *testing.T) {
	mgr := NewConfigManager("")
	assert.Contains(t, mgr.configPath, ".smart-cut")
}
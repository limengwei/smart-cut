package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"smart-cut/internal/model"
)

type ConfigManager struct {
	configPath string
}

func NewConfigManager(configDir string) *ConfigManager {
	if configDir == "" {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".smart-cut")
	}
	return &ConfigManager{
		configPath: filepath.Join(configDir, "config.json"),
	}
}

func (m *ConfigManager) Load() (*model.GlobalSettings, error) {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultSettings(), nil
		}
		return nil, err
	}

	var settings model.GlobalSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

func (m *ConfigManager) Save(settings *model.GlobalSettings) error {
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.configPath, data, 0644)
}

func defaultSettings() *model.GlobalSettings {
	return &model.GlobalSettings{
		Binaries:        map[string]string{},
		WhisperModelDir: "",
		DefaultLLM: model.LLMConfig{
			BaseURL: "https://api.openai.com/v1",
			APIKey:  "",
			Model:   "gpt-4o-mini",
		},
		Theme: "dark",
	}
}
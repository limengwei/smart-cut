package model

// GlobalSettings 全局设置（存 ~/.smart-cut/config.json）
type GlobalSettings struct {
	Binaries        map[string]string `json:"binaries"` // name→path，空则用随包或 PATH
	WhisperModelDir string            `json:"whisperModelDir"`
	DefaultLLM      LLMConfig         `json:"defaultLLM"`
	Theme           string            `json:"theme"` // light/dark
}

// LLMConfig LLM 配置（OpenAI 兼容）
type LLMConfig struct {
	BaseURL string `json:"baseUrl"` // 如 https://api.openai.com/v1
	APIKey  string `json:"apiKey"`
	Model   string `json:"model"` // 如 gpt-4o-mini
}

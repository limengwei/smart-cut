package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"smart-cut/internal/model"
)

// LLMAdapter 定义 LLM 分析接口
type LLMAdapter interface {
	Analyze(ctx context.Context, req model.LLMAnalysisRequest, cfg model.LLMConfig) (*model.LLMAnalysisResult, error)
}

// openAIAdapter 是基于 OpenAI 兼容 API 的 LLMAdapter 实现
type openAIAdapter struct {
	httpClient *http.Client
}

// NewLLMAdapter 创建基于 OpenAI 兼容 HTTP API 的 Adapter
func NewLLMAdapter() LLMAdapter {
	return &openAIAdapter{
		httpClient: &http.Client{},
	}
}

// chatCompletionRequest 是 OpenAI Chat Completions API 的请求体
type chatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatCompletionResponse 是 OpenAI Chat Completions API 的响应体
type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (a *openAIAdapter) Analyze(ctx context.Context, req model.LLMAnalysisRequest, cfg model.LLMConfig) (*model.LLMAnalysisResult, error) {
	if cfg.BaseURL == "" || cfg.APIKey == "" || cfg.Model == "" {
		return nil, fmt.Errorf("llm analyze: BaseURL, APIKey, Model are required")
	}

	// 构建 system prompt
	systemPrompt := buildSystemPrompt(req)
	userPrompt := buildUserPrompt(req)

	chatReq := chatCompletionRequest{
		Model: cfg.Model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.3,
	}

	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("llm analyze: marshal request: %w", err)
	}

	url := cfg.BaseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("llm analyze: create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llm analyze: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("llm analyze: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("llm analyze: decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("llm analyze: no choices in response")
	}

	return parseLLMResponse(chatResp.Choices[0].Message.Content)
}

// buildSystemPrompt 构建 LLM 的 system prompt
func buildSystemPrompt(req model.LLMAnalysisRequest) string {
	return `你是一个专业的口播视频剪辑助手。你的任务是分析视频转录文本，识别需要删除的片段。

需要识别的片段类型：
1. filler（语气词）：如 "嗯"、"啊"、"那个"、"就是说" 等无意义的填充词
2. silence（停顿）：长时间的沉默或无意义的停顿
3. dup_or_error（重复/口误）：重复说同一句话、说错后重新说的部分

请以 JSON 格式返回分析结果，格式如下：
{
  "removeSegmentIds": [需要删除的句段ID列表],
  "items": [
    {
      "segmentId": 句段ID,
      "reason": "filler" | "silence" | "dup_or_error",
      "confidence": 0.0-1.0,
      "note": "简要说明原因"
    }
  ]
}

只返回 JSON，不要包含其他文字。`
}

// buildUserPrompt 构建 LLM 的 user prompt
func buildUserPrompt(req model.LLMAnalysisRequest) string {
	var sb bytes.Buffer
	sb.WriteString(fmt.Sprintf("语言: %s\n\n句段列表:\n", req.Language))
	for _, seg := range req.Segments {
		sb.WriteString(fmt.Sprintf("[ID:%d] %dms-%dms: %s\n", seg.ID, seg.StartMs, seg.EndMs, seg.Text))
	}
	return sb.String()
}

// parseLLMResponse 解析 LLM 返回的 JSON 文本
func parseLLMResponse(content string) (*model.LLMAnalysisResult, error) {
	// LLM 可能在 JSON 前后加 markdown 标记，清理一下
	content = cleanJSONResponse(content)

	var result model.LLMAnalysisResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("llm parse response: %w (content: %s)", err, content)
	}
	return &result, nil
}

// cleanJSONResponse 清理 LLM 返回中可能的 markdown 代码块标记
func cleanJSONResponse(s string) string {
	// 去除 ```json ... ``` 包裹
	if len(s) > 10 && s[:7] == "```json" {
		s = s[7:]
	}
	if len(s) > 3 && s[:3] == "```" {
		s = s[3:]
	}
	// 去除末尾的 ```
	if len(s) > 3 && s[len(s)-3:] == "```" {
		s = s[:len(s)-3]
	}

	return s
}

package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"smart-cut/internal/model"
)

// WhisperAdapter 定义语音转文字的接口
type WhisperAdapter interface {
	Transcribe(ctx context.Context, mediaPath string, opts WhisperOptions) (*model.Transcript, error)
}

// WhisperOptions whisper.cpp 调用选项
type WhisperOptions struct {
	Language  string // "zh"/"auto"
	ModelPath string // 如 /path/to/ggml-base.bin
	WordLevel bool   // 是否输出词级时间戳
}

// whisperCLIAdapter 是 WhisperAdapter 的具体实现（调用 whisper-cli 二进制）
type whisperCLIAdapter struct {
	resolver *BinaryResolver
}

// NewWhisperAdapter 创建基于 whisper-cli 二进制的 Adapter
func NewWhisperAdapter(resolver *BinaryResolver) WhisperAdapter {
	return &whisperCLIAdapter{resolver: resolver}
}

// whisperJSONOutput 是 whisper.cpp -ojf 输出的 JSON 结构
type whisperJSONOutput struct {
	Transcription []struct {
		Timestamps struct {
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"timestamps"`
		Offsets struct {
			From int64 `json:"from"`
			To   int64 `json:"to"`
		} `json:"offsets"`
		Text string `json:"text"`
	} `json:"transcription"`
}

func (a *whisperCLIAdapter) Transcribe(ctx context.Context, mediaPath string, opts WhisperOptions) (*model.Transcript, error) {
	binaryPath, err := a.resolver.Resolve("whisper-cli")
	if err != nil {
		return nil, fmt.Errorf("whisper transcribe: %w", err)
	}

	// whisper.cpp 要求 16kHz 单声道 wav，调用方应提前用 ffmpeg 转好
	// 输出 JSON 到临时文件
	tmpDir, err := os.MkdirTemp("", "whisper-")
	if err != nil {
		return nil, fmt.Errorf("whisper transcribe: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	outputBase := filepath.Join(tmpDir, "output")

	args := []string{
		"-m", opts.ModelPath,
		"-f", mediaPath,
		"-of", outputBase,
		"-ojf",
		"-l", opts.Language,
	}
	if opts.WordLevel {
		args = append(args, "-owts")
	}

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("whisper transcribe: %w", err)
	}

	// 读取 JSON 输出
	jsonPath := outputBase + ".json"
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("whisper transcribe: read output json: %w", err)
	}

	return parseWhisperJSON(data)
}

// parseWhisperJSON 解析 whisper.cpp -ojf 输出为 model.Transcript
func parseWhisperJSON(data []byte) (*model.Transcript, error) {
	var raw whisperJSONOutput
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("whisper parse json: %w", err)
	}

	transcript := &model.Transcript{
		Segments: make([]model.Segment, 0, len(raw.Transcription)),
	}

	for i, seg := range raw.Transcription {
		// 清理文本前导空格
		text := seg.Text
		if len(text) > 0 && text[0] == ' ' {
			text = text[1:]
		}

		segment := model.Segment{
			ID:      i,
			StartMs: seg.Offsets.From,
			EndMs:   seg.Offsets.To,
			Text:    text,
		}

		transcript.Segments = append(transcript.Segments, segment)
	}

	return transcript, nil
}

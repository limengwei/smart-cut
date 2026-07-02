package adapter

import (
	"bytes"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"smart-cut/internal/model"
)

// WhisperAdapter 定义语音转文字的接口
type WhisperAdapter interface {
	Transcribe(ctx context.Context, mediaPath string, opts WhisperOptions) (*model.Transcript, error)
	// TranscribeStream 流式转录：实时回调进度百分比(0-1)和已识别句段
	TranscribeStream(ctx context.Context, mediaPath string, opts WhisperOptions,
		onProgress func(progress float64), onSegment func(seg model.Segment)) (*model.Transcript, error)
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
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("whisper transcribe: %w (stderr: %s)", err, stderrBuf.String())
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

// —— 流式转录 ——

// whisperProgressRe 匹配 stderr 的进度行: "whisper_print_progress_callback: progress =  5%"
var whisperProgressRe = regexp.MustCompile(`progress\s*=\s*(\d+)%`)

// whisperSegmentRe 匹配 stdout 的句段行: "[00:00:00.000 --> 00:00:05.000]   文本"
var whisperSegmentRe = regexp.MustCompile(
	`\[(\d{2}):(\d{2}):(\d{2})\.(\d{3})\s*-->\s*(\d{2}):(\d{2}):(\d{2})\.(\d{3})\]\s*(.*)`)

func (a *whisperCLIAdapter) TranscribeStream(ctx context.Context, mediaPath string, opts WhisperOptions,
	onProgress func(progress float64), onSegment func(seg model.Segment)) (*model.Transcript, error) {
	binaryPath, err := a.resolver.Resolve("whisper-cli")
	if err != nil {
		return nil, fmt.Errorf("whisper transcribe: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "whisper-")
	if err != nil {
		return nil, fmt.Errorf("whisper transcribe: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	outputBase := filepath.Join(tmpDir, "output")

	// 关键：-pp 启用进度回调到 stderr；不加 -np 以保留 stdout 实时句段输出
	args := []string{
		"-m", opts.ModelPath,
		"-f", mediaPath,
		"-of", outputBase,
		"-ojf",
		"-l", opts.Language,
		"-pp",
	}
	if opts.WordLevel {
		args = append(args, "-owts")
	}

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("whisper transcribe: stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("whisper transcribe: stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("whisper transcribe: start: %w", err)
	}

	var (
		stderrBuf   bytes.Buffer
		streamSegs  []model.Segment
		segMu       sync.Mutex
		wg          sync.WaitGroup
	)

	// stdout 扫描器：解析实时句段
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdoutPipe)
		scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		segIdx := 0
		for scanner.Scan() {
			line := scanner.Text()
			if startMs, endMs, text, ok := parseSegmentLine(line); ok {
				seg := model.Segment{
					ID:      segIdx,
					StartMs: startMs,
					EndMs:   endMs,
					Text:    text,
				}
				segIdx++
				segMu.Lock()
				streamSegs = append(streamSegs, seg)
				segMu.Unlock()
				if onSegment != nil {
					onSegment(seg)
				}
			}
		}
	}()

	// stderr 扫描器：解析进度百分比（同时累积到 buffer 供错误诊断）
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			stderrBuf.WriteString(line + "\n")
			if onProgress != nil {
				if m := whisperProgressRe.FindStringSubmatch(line); m != nil {
					pct, _ := strconv.Atoi(m[1])
					onProgress(float64(pct) / 100.0)
				}
			}
		}
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("whisper transcribe: %w (stderr: %s)", err, stderrBuf.String())
	}

	log.Printf("[Whisper] 流式完成: streamSegs=%d", len(streamSegs))

	// 优先以 -ojf JSON 为权威结果（含完整结构），失败则回退用 stdout 累积的句段
	jsonPath := outputBase + ".json"
	if data, err := os.ReadFile(jsonPath); err == nil {
		if t, err := parseWhisperJSON(data); err == nil {
			return t, nil
		}
		log.Printf("[Whisper] JSON 解析失败，回退 stdout 句段: %v", err)
	}

	// 回退：用 stdout 累积的句段
	segMu.Lock()
	defer segMu.Unlock()
	if len(streamSegs) == 0 {
		return nil, fmt.Errorf("whisper transcribe: 无任何句段输出 (stderr: %s)", stderrBuf.String())
	}
	return &model.Transcript{Segments: streamSegs}, nil
}

// parseSegmentLine 解析 stdout 的一行句段，返回 (startMs, endMs, text, ok)
func parseSegmentLine(line string) (startMs, endMs int64, text string, ok bool) {
	m := whisperSegmentRe.FindStringSubmatch(line)
	if m == nil {
		return 0, 0, "", false
	}
	startMs = tsToMs(m[1], m[2], m[3], m[4])
	endMs = tsToMs(m[5], m[6], m[7], m[8])
	text = strings.TrimSpace(m[9])
	return startMs, endMs, text, true
}

// tsToMs 把 HH:MM:SS.mmm 各部分转成毫秒
func tsToMs(h, mi, s, ms string) int64 {
	hh, _ := strconv.ParseInt(h, 10, 64)
	mmi, _ := strconv.ParseInt(mi, 10, 64)
	ss, _ := strconv.ParseInt(s, 10, 64)
	mss, _ := strconv.ParseInt(ms, 10, 64)
	return (hh*3600+mmi*60+ss)*1000 + mss
}

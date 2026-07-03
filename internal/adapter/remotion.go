package adapter

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"smart-cut/internal/model"
)

// RemotionAdapter 封装对 Node render-worker.js 的调用
type RemotionAdapter interface {
	RenderSegment(ctx context.Context, req SubtitleSegmentRequest, onProgress func(ratio float64)) (clipPath string, err error)
}

// SubtitleSegmentRequest 渲染单个 keep 段字幕的请求
type SubtitleSegmentRequest struct {
	SegmentID string              `json:"segmentId"` // keep 段标识，用于命名输出文件
	StartMs   int64               `json:"startMs"`   // 段在原视频的起点（用于 worker 日志，不传给 Composition）
	EndMs     int64               `json:"endMs"`
	Segments  []model.Segment     `json:"segments"` // 落在本 keep 段内的字幕句段（已偏移为段内相对时间）
	Style     model.SubtitleStyle `json:"style"`
	Width     int                 `json:"width"`     // 视频帧宽（来自 MediaFile.Width）
	Height    int                 `json:"height"`    // 视频帧高
	Fps       float64             `json:"fps"`       // 视频帧率（来自 MediaFile.Fps）
	OutputDir string              `json:"outputDir"` // 段字幕 mp4 输出目录
}

// workerInput 传给 render-worker.js stdin 的 JSON 结构（与 SubtitleSegmentRequest 几乎一致，额外带 outputPath）
type workerInput struct {
	SegmentID  string              `json:"segmentId"`
	StartMs    int64               `json:"startMs"`
	EndMs      int64               `json:"endMs"`
	Segments   []model.Segment     `json:"segments"`
	Style      model.SubtitleStyle `json:"style"`
	Width      int                 `json:"width"`
	Height     int                 `json:"height"`
	Fps        float64             `json:"fps"`
	OutputPath string              `json:"outputPath"`
}

// workerOutput render-worker.js stdout 按行输出的结构
type workerOutput struct {
	Type       string  `json:"type"`       // "progress" | "done" | "error"
	Progress   float64 `json:"progress"`   // 仅 type=progress
	OutputPath string  `json:"outputPath"` // 仅 type=done
	Message    string  `json:"message"`    // 仅 type=error
}

// remotionCLIAdapter 是 RemotionAdapter 的具体实现
type remotionCLIAdapter struct {
	resolver     *BinaryResolver
	workerScript string // render-worker.js 的绝对路径
}

// NewRemotionAdapter 创建 RemotionAdapter
// resolver: 用于查找 node 二进制
// workerScript: render-worker.js 的绝对路径（如 resources/remotion/render-worker.js）
func NewRemotionAdapter(resolver *BinaryResolver, workerScript string) RemotionAdapter {
	return &remotionCLIAdapter{resolver: resolver, workerScript: workerScript}
}

// RenderSegment 渲染单个 keep 段的字幕透明 mp4
func (a *remotionCLIAdapter) RenderSegment(ctx context.Context, req SubtitleSegmentRequest, onProgress func(ratio float64)) (string, error) {
	nodePath, err := a.resolver.Resolve("node")
	if err != nil {
		return "", fmt.Errorf("remotion render: %w", err)
	}

	if req.EndMs <= req.StartMs {
		return "", fmt.Errorf("remotion render: invalid segment duration %d-%d", req.StartMs, req.EndMs)
	}

	outputPath := filepath.Join(req.OutputDir, fmt.Sprintf("subtitle_%s.mp4", req.SegmentID))

	input := workerInput{
		SegmentID:  req.SegmentID,
		StartMs:    req.StartMs,
		EndMs:      req.EndMs,
		Segments:   req.Segments,
		Style:      req.Style,
		Width:      req.Width,
		Height:     req.Height,
		Fps:        req.Fps,
		OutputPath: outputPath,
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("remotion render: marshal input: %w", err)
	}

	cmd := exec.CommandContext(ctx, nodePath, a.workerScript)
	cmd.Stdin = strings.NewReader(string(inputJSON) + "\n")

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("remotion render: stdout pipe: %w", err)
	}
	var stderrBuf strings.Builder

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("remotion render: start: %w", err)
	}

	// 并发 drain stderr，避免 worker 写满 stderr pipe buffer 后阻塞 stdout（防死锁）
	var stderrWG sync.WaitGroup
	stderrPipe, err := cmd.StderrPipe()
	if err == nil {
		stderrWG.Add(1)
		go func() {
			defer stderrWG.Done()
			scanner := bufio.NewScanner(stderrPipe)
			scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
			for scanner.Scan() {
				stderrBuf.WriteString(scanner.Text() + "\n")
			}
		}()
	}

	resultPath, perr := parseWorkerStdout(stdoutPipe, onProgress)

	werr := cmd.Wait()
	stderrWG.Wait() // 确保 stderr drain 完成，stderrBuf 内容完整

	// 优先返回 worker 自报的具体错误（perr），再回退到进程退出错误
	if perr != nil {
		return "", fmt.Errorf("remotion render: %w (stderr: %s)", perr, stderrBuf.String())
	}
	if werr != nil {
		return "", fmt.Errorf("remotion render: worker exit: %w (stderr: %s)", werr, stderrBuf.String())
	}
	if resultPath == "" {
		return "", fmt.Errorf("remotion render: worker 未输出 DONE（stderr: %s）", stderrBuf.String())
	}
	return resultPath, nil
}

// parseWorkerStdout 按行扫描 worker stdout，解析 JSON 行（progress/done/error）
// 纯函数（接收 io.Reader 接口），便于测试
func parseWorkerStdout(r interface {
	Read(p []byte) (n int, err error)
}, onProgress func(ratio float64)) (outputPath string, err error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var out workerOutput
		if jerr := json.Unmarshal([]byte(line), &out); jerr != nil {
			continue // 非 JSON 行跳过（worker 可能输出调试日志）
		}
		switch out.Type {
		case "progress":
			if onProgress != nil {
				onProgress(out.Progress)
			}
		case "done":
			return out.OutputPath, nil
		case "error":
			return "", fmt.Errorf("worker error: %s", out.Message)
		}
	}
	if serr := scanner.Err(); serr != nil {
		return "", fmt.Errorf("remotion render: read stdout: %w", serr)
	}
	return "", nil
}

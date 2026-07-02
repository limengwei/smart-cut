# Smart-Cut Plan 2: Adapter 层

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现三个核心 Adapter（Whisper / FFmpeg / LLM）及其接口定义、BinaryResolver，配齐基于 fixture 的单元测试。

**Architecture:** 每个 Adapter 封装一个外部依赖（二进制或 HTTP API），通过 exec 调用或 HTTP 请求，解析输出为 model 层数据结构。Adapter 之间互不依赖，可独立测试。BinaryResolver 负责外部二进制的三级查找（用户配置 → 随包 → 系统 PATH）。

**Tech Stack:** Go, `os/exec`, `net/http`, `encoding/json`, `regexp`, testify, fixture 文件

---

## File Structure

```
smart-cut/
├── internal/
│   ├── adapter/
│   │   ├── binary.go              # BinaryResolver 三级查找
│   │   ├── binary_test.go         # BinaryResolver 测试
│   │   ├── whisper.go             # WhisperAdapter 接口 + 实现
│   │   ├── whisper_test.go        # WhisperAdapter 测试（fixture 解析）
│   │   ├── ffmpeg.go              # FFmpegAdapter 接口 + 实现
│   │   ├── ffmpeg_test.go         # FFmpegAdapter 测试（stderr 解析）
│   │   ├── llm.go                 # LLMAdapter 接口 + 实现
│   │   ├── llm_test.go            # LLMAdapter 测试（HTTP mock）
│   │   └── testdata/              # fixture 文件目录
│   │       ├── whisper_output.json    # whisper.cpp -ojf 输出样本
│   │       └── ffprobe_output.json    # ffprobe -of json 输出样本
└── go.mod
```

---

### Task 1: BinaryResolver — 二进制查找

**Files:**
- Create: `internal/adapter/binary.go`
- Create: `internal/adapter/binary_test.go`

- [ ] **Step 1: 编写 binary.go**

创建 `internal/adapter/binary.go`：

```go
package adapter

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
)

// BinaryResolver 负责查找外部二进制文件
// 查找优先级：customPath → resources/bin → 系统 PATH
type BinaryResolver struct {
	customPaths map[string]string // 用户配置的路径
	bundleDir   string            // 随包二进制目录
}

// NewBinaryResolver 创建 BinaryResolver
// customPaths: 用户在设置里配置的 name→path 映射
// bundleDir: 随包二进制目录（如 resources/bin）
func NewBinaryResolver(customPaths map[string]string, bundleDir string) *BinaryResolver {
	return &BinaryResolver{
		customPaths: customPaths,
		bundleDir:   bundleDir,
	}
}

// Resolve 查找指定名称的二进制文件
// name: 二进制名称（如 "ffmpeg"、"whisper-cli"）
func (r *BinaryResolver) Resolve(name string) (string, error) {
	// 1. 用户配置的路径
	if path, ok := r.customPaths[name]; ok && path != "" {
		if _, err := exec.LookPath(path); err == nil {
			return path, nil
		}
	}

	// 2. 随包目录
	if r.bundleDir != "" {
		exeName := name
		if runtime.GOOS == "windows" {
			exeName = name + ".exe"
		}
		bundlePath := filepath.Join(r.bundleDir, exeName)
		if _, err := exec.LookPath(bundlePath); err == nil {
			return bundlePath, nil
		}
	}

	// 3. 系统 PATH
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("binary %q not found in custom paths, bundle dir, or system PATH", name)
	}
	return path, nil
}
```

- [ ] **Step 2: 编写 binary_test.go**

创建 `internal/adapter/binary_test.go`：

```go
package adapter

import (
	"os/exec"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// systemCommandName 返回一个系统一定存在的命令名
func systemCommandName() string {
	if runtime.GOOS == "windows" {
		return "cmd"
	}
	return "sh"
}

func TestBinaryResolver_CustomPath(t *testing.T) {
	// 用系统自带的命令模拟：先从 PATH 找到它，再作为 customPath 传入
	name := systemCommandName()
	path, err := exec.LookPath(name)
	require.NoError(t, err)

	resolver := NewBinaryResolver(map[string]string{"test-bin": path}, "")

	resolved, err := resolver.Resolve("test-bin")
	require.NoError(t, err)
	assert.Equal(t, path, resolved)
}

func TestBinaryResolver_SystemPATH(t *testing.T) {
	// 不提供 customPath 和 bundleDir，走系统 PATH
	resolver := NewBinaryResolver(nil, "")

	path, err := resolver.Resolve(systemCommandName())
	require.NoError(t, err)
	assert.NotEmpty(t, path)
}

func TestBinaryResolver_NotFound(t *testing.T) {
	resolver := NewBinaryResolver(nil, "")

	_, err := resolver.Resolve("this-binary-does-not-exist-12345")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
```

- [ ] **Step 3: 运行测试**

```bash
go test ./internal/adapter/ -v -run TestBinaryResolver
```

Expected: 全部 PASS。

- [ ] **Step 4: Commit**

```bash
git add internal/adapter/binary.go internal/adapter/binary_test.go
git commit -m "feat(adapter): add BinaryResolver with 3-tier lookup"
```

---

### Task 2: WhisperAdapter — 接口与 fixture

**Files:**
- Create: `internal/adapter/testdata/whisper_output.json`
- Create: `internal/adapter/whisper.go`

- [ ] **Step 1: 创建 whisper.cpp JSON 输出 fixture**

创建 `internal/adapter/testdata/whisper_output.json`（这是 whisper.cpp `-ojf` 格式的真实样本）：

```json
{
  "transcription": [
    {
      "timestamps": {
        "from": "00:00:00,000",
        "to": "00:00:01,500"
      },
      "offsets": {
        "from": 0,
        "to": 1500
      },
      "text": " 大家好今天来聊聊"
    },
    {
      "timestamps": {
        "from": "00:00:01,500",
        "to": "00:00:03,000"
      },
      "offsets": {
        "from": 1500,
        "to": 3000
      },
      "text": " 嗯怎么用AI剪辑视频"
    }
  ]
}
```

- [ ] **Step 2: 编写 whisper.go**

创建 `internal/adapter/whisper.go`：

```go
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

		// 词级时间戳暂不解析（需要 -owts 格式，结构不同）
		// MVP 用句级即可，词级在后续 plan 补充

		transcript.Segments = append(transcript.Segments, segment)
	}

	return transcript, nil
}
```

- [ ] **Step 3: 验证编译**

```bash
go build ./internal/adapter/
```

Expected: 无报错。

- [ ] **Step 4: Commit**

```bash
git add internal/adapter/whisper.go internal/adapter/testdata/whisper_output.json
git commit -m "feat(adapter): add WhisperAdapter with JSON parsing"
```

---

### Task 3: WhisperAdapter — 单元测试

**Files:**
- Create: `internal/adapter/whisper_test.go`

- [ ] **Step 1: 编写 whisper_test.go**

创建 `internal/adapter/whisper_test.go`：

```go
package adapter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseWhisperJSON_ValidOutput(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "whisper_output.json"))
	require.NoError(t, err)

	transcript, err := parseWhisperJSON(data)
	require.NoError(t, err)

	assert.Len(t, transcript.Segments, 2)

	// 第一段
	assert.Equal(t, 0, transcript.Segments[0].ID)
	assert.Equal(t, int64(0), transcript.Segments[0].StartMs)
	assert.Equal(t, int64(1500), transcript.Segments[0].EndMs)
	assert.Equal(t, "大家好今天来聊聊", transcript.Segments[0].Text)

	// 第二段
	assert.Equal(t, 1, transcript.Segments[1].ID)
	assert.Equal(t, int64(1500), transcript.Segments[1].StartMs)
	assert.Equal(t, int64(3000), transcript.Segments[1].EndMs)
	assert.Equal(t, "嗯怎么用AI剪辑视频", transcript.Segments[1].Text)
}

func TestParseWhisperJSON_EmptyTranscription(t *testing.T) {
	data := []byte(`{"transcription": []}`)

	transcript, err := parseWhisperJSON(data)
	require.NoError(t, err)
	assert.Empty(t, transcript.Segments)
}

func TestParseWhisperJSON_InvalidJSON(t *testing.T) {
	data := []byte(`{invalid json}`)

	_, err := parseWhisperJSON(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "whisper parse json")
}

func TestParseWhisperJSON_StripsLeadingSpace(t *testing.T) {
	data := []byte(`{
		"transcription": [
			{
				"timestamps": {"from": "00:00:00,000", "to": "00:00:01,000"},
				"offsets": {"from": 0, "to": 1000},
				"text": " 没有前导空格的文本"
			}
		]
	}`)

	transcript, err := parseWhisperJSON(data)
	require.NoError(t, err)
	assert.Equal(t, "没有前导空格的文本", transcript.Segments[0].Text)
}
```

- [ ] **Step 2: 运行测试**

```bash
go test ./internal/adapter/ -v -run TestParseWhisperJSON
```

Expected: 全部 PASS。

- [ ] **Step 3: Commit**

```bash
git add internal/adapter/whisper_test.go
git commit -m "test(adapter): add WhisperAdapter JSON parsing tests"
```

---

### Task 4: FFmpegAdapter — 接口与 Probe 实现

**Files:**
- Create: `internal/adapter/ffmpeg.go`

- [ ] **Step 1: 编写 ffmpeg.go**

创建 `internal/adapter/ffmpeg.go`：

```go
package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"smart-cut/internal/model"
)

// FFmpegAdapter 定义视频处理接口
type FFmpegAdapter interface {
	Probe(ctx context.Context, path string) (*model.MediaFile, error)
	ExtractWaveform(ctx context.Context, mediaPath, outPng string) error
	ConcatLossless(ctx context.Context, segments []model.KeepSegment, sourcePath, outPath string) error
	ConcatReencode(ctx context.Context, segments []model.KeepSegment, sourcePath, outPath string, opts model.EncodeOpts) error
	MuxSubtitle(ctx context.Context, videoPath, subtitleClipPath, outPath string) error
}

// ffmpegAdapter 是 FFmpegAdapter 的具体实现
type ffmpegAdapter struct {
	resolver *BinaryResolver
}

// NewFFmpegAdapter 创建基于 ffmpeg/ffprobe 二进制的 Adapter
func NewFFmpegAdapter(resolver *BinaryResolver) FFmpegAdapter {
	return &ffmpegAdapter{resolver: resolver}
}

// ffprobeJSONOutput 是 ffprobe -of json 的输出结构
type ffprobeJSONOutput struct {
	Streams []struct {
		CodecType  string `json:"codec_type"`
		CodecName  string `json:"codec_name"`
		Width      int    `json:"width"`
		Height     int    `json:"height"`
		RFrameRate string `json:"r_frame_rate"`
		Duration   string `json:"duration"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
		FormatName string `json:"format_name"`
	} `json:"format"`
}

func (a *ffmpegAdapter) Probe(ctx context.Context, path string) (*model.MediaFile, error) {
	probePath, err := a.resolver.Resolve("ffprobe")
	if err != nil {
		return nil, fmt.Errorf("ffprobe: %w", err)
	}

	cmd := exec.CommandContext(ctx, probePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		path,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe: %w", err)
	}

	return parseFFprobeJSON(output)
}

// parseFFprobeJSON 解析 ffprobe JSON 输出为 MediaFile
func parseFFprobeJSON(data []byte) (*model.MediaFile, error) {
	var raw ffprobeJSONOutput
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("ffprobe parse: %w", err)
	}

	media := &model.MediaFile{}

	// 从 streams 提取视频和音频信息
	for _, stream := range raw.Streams {
		if stream.CodecType == "video" {
			media.Width = stream.Width
			media.Height = stream.Height
			media.Fps = parseFrameRate(stream.RFrameRate)
		}
		if stream.CodecType == "audio" {
			media.HasAudio = true
		}
	}

	// 从 format 提取时长和格式
	media.DurationMs = parseDurationMs(raw.Format.Duration)
	media.Format = raw.Format.FormatName

	return media, nil
}

// parseFrameRate 解析 ffprobe 的帧率字符串（如 "30000/1001"）
func parseFrameRate(rate string) float64 {
	if rate == "" {
		return 0
	}
	parts := strings.Split(rate, "/")
	if len(parts) == 2 {
		num, err1 := strconv.ParseFloat(parts[0], 64)
		den, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 == nil && err2 == nil && den != 0 {
			return num / den
		}
	}
	f, err := strconv.ParseFloat(rate, 64)
	if err != nil {
		return 0
	}
	return f
}

// parseDurationMs 解析 ffprobe 的时长字符串（秒，浮点）转为毫秒
func parseDurationMs(duration string) int64 {
	if duration == "" {
		return 0
	}
	f, err := strconv.ParseFloat(duration, 64)
	if err != nil {
		return 0
	}
	return int64(f * 1000)
}

func (a *ffmpegAdapter) ExtractWaveform(ctx context.Context, mediaPath, outPng string) error {
	binaryPath, err := a.resolver.Resolve("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg waveform: %w", err)
	}

	// 生成波形图：转为单声道，用 showwavespic 滤镜
	cmd := exec.CommandContext(ctx, binaryPath,
		"-i", mediaPath,
		"-filter_complex", "showwavespic=s=1280x120:colors=white",
		"-frames:v", "1",
		"-y",
		outPng,
	)
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg waveform: %w", err)
	}
	return nil
}

func (a *ffmpegAdapter) ConcatLossless(ctx context.Context, segments []model.KeepSegment, sourcePath, outPath string) error {
	binaryPath, err := a.resolver.Resolve("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg concat lossless: %w", err)
	}

	// 用 concat demuxer 实现无损拼接
	// 需要先生成 concat 列表文件，再用 -c copy 拼接
	// 这里用单条命令 + filter_complex 实现（适用于段数较少的场景）
	if len(segments) == 0 {
		return fmt.Errorf("ffmpeg concat: no segments to concat")
	}

	// 构建 filter_complex
	var filters []string
	for i, seg := range segments {
		startSec := float64(seg.StartMs) / 1000.0
		endSec := float64(seg.EndMs) / 1000.0
		filters = append(filters, fmt.Sprintf("[0:v]trim=start=%f:end=%f,setpts=PTS-STARTPTS[v%d];[0:a]atrim=start=%f:end=%f,asetpts=PTS-STARTPTS[a%d]",
			startSec, endSec, i, startSec, endSec, i))
	}

	// 连接所有段
	var inputs []string
	var concatV []string
	var concatA []string
	for i := range segments {
		concatV = append(concatV, fmt.Sprintf("[v%d]", i))
		concatA = append(concatA, fmt.Sprintf("[a%d]", i))
	}
	concatFilter := strings.Join(concatV, "") + fmt.Sprintf("concat=n=%d:v=1:a=0[vout]", len(segments)) +
		";" + strings.Join(concatA, "") + fmt.Sprintf("concat=n=%d:v=0:a=1[aout]", len(segments))

	filterComplex := strings.Join(filters, ";") + ";" + concatFilter

	args := []string{
		"-i", sourcePath,
		"-filter_complex", filterComplex,
		"-map", "[vout]",
		"-map", "[aout]",
		"-c:v", "copy",
		"-c:a", "copy",
		"-y",
		outPath,
	}
	_ = inputs

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg concat lossless: %w", err)
	}
	return nil
}

func (a *ffmpegAdapter) ConcatReencode(ctx context.Context, segments []model.KeepSegment, sourcePath, outPath string, opts model.EncodeOpts) error {
	binaryPath, err := a.resolver.Resolve("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg concat reencode: %w", err)
	}

	if len(segments) == 0 {
		return fmt.Errorf("ffmpeg concat: no segments to concat")
	}

	// 构建 filter_complex（同 lossless，但输出用重编码）
	var filters []string
	for i, seg := range segments {
		startSec := float64(seg.StartMs) / 1000.0
		endSec := float64(seg.EndMs) / 1000.0
		filters = append(filters, fmt.Sprintf("[0:v]trim=start=%f:end=%f,setpts=PTS-STARTPTS[v%d];[0:a]atrim=start=%f:end=%f,asetpts=PTS-STARTPTS[a%d]",
			startSec, endSec, i, startSec, endSec, i))
	}

	var concatV []string
	var concatA []string
	for i := range segments {
		concatV = append(concatV, fmt.Sprintf("[v%d]", i))
		concatA = append(concatA, fmt.Sprintf("[a%d]", i))
	}
	concatFilter := strings.Join(concatV, "") + fmt.Sprintf("concat=n=%d:v=1:a=0[vout]", len(segments)) +
		";" + strings.Join(concatA, "") + fmt.Sprintf("concat=n=%d:v=0:a=1[aout]", len(segments))

	filterComplex := strings.Join(filters, ";") + ";" + concatFilter

	videoCodec := opts.VideoCodec
	if videoCodec == "" {
		videoCodec = "libx264"
	}
	audioCodec := opts.AudioCodec
	if audioCodec == "" {
		audioCodec = "aac"
	}
	crf := opts.Crf
	if crf == 0 {
		crf = 23
	}
	preset := opts.Preset
	if preset == "" {
		preset = "medium"
	}

	args := []string{
		"-i", sourcePath,
		"-filter_complex", filterComplex,
		"-map", "[vout]",
		"-map", "[aout]",
		"-c:v", videoCodec,
		"-c:a", audioCodec,
		"-crf", strconv.Itoa(crf),
		"-preset", preset,
		"-y",
		outPath,
	}

	if opts.VideoBitrate != "" {
		args = append([]string{"-b:v", opts.VideoBitrate}, args...)
	}

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg concat reencode: %w", err)
	}
	return nil
}

func (a *ffmpegAdapter) MuxSubtitle(ctx context.Context, videoPath, subtitleClipPath, outPath string) error {
	binaryPath, err := a.resolver.Resolve("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg mux subtitle: %w", err)
	}

	// 将字幕片段叠加到视频上
	cmd := exec.CommandContext(ctx, binaryPath,
		"-i", videoPath,
		"-i", subtitleClipPath,
		"-filter_complex", "[1:v]scale=iw:ih[overlay];[0:v][overlay]overlay=0:0",
		"-c:a", "copy",
		"-y",
		outPath,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg mux subtitle: %w", err)
	}
	return nil
}

// ParseFFmpegProgress 从 ffmpeg stderr 行解析进度（time=00:01:23.456）
var ffmpegProgressRegex = regexp.MustCompile(`time=(\d+):(\d+):(\d+).(\d+)`)

// ParseFFmpegProgress 解析 ffmpeg stderr 的 time= 行，返回已处理毫秒数
func ParseFFmpegProgress(line string) int64 {
	matches := ffmpegProgressRegex.FindStringSubmatch(line)
	if len(matches) != 5 {
		return -1
	}
	hours, _ := strconv.Atoi(matches[1])
	minutes, _ := strconv.Atoi(matches[2])
	seconds, _ := strconv.Atoi(matches[3])
	// ffmpeg 的百分秒部分可能是 1-3 位
	cs, _ := strconv.Atoi(matches[4])
	for i := len(matches[4]); i < 3; i++ {
		cs *= 10
	}
	return int64(hours*3600000 + minutes*60000 + seconds*1000 + cs)
}
```

- [ ] **Step 2: 验证编译**

```bash
go build ./internal/adapter/
```

Expected: 无报错。

- [ ] **Step 3: Commit**

```bash
git add internal/adapter/ffmpeg.go
git commit -m "feat(adapter): add FFmpegAdapter with Probe, Concat, Waveform, MuxSubtitle"
```

---

### Task 5: FFmpegAdapter — 单元测试

**Files:**
- Create: `internal/adapter/testdata/ffprobe_output.json`
- Create: `internal/adapter/ffmpeg_test.go`

- [ ] **Step 1: 创建 ffprobe fixture**

创建 `internal/adapter/testdata/ffprobe_output.json`：

```json
{
  "streams": [
    {
      "codec_type": "video",
      "codec_name": "h264",
      "width": 1920,
      "height": 1080,
      "r_frame_rate": "30/1",
      "duration": "10.500000"
    },
    {
      "codec_type": "audio",
      "codec_name": "aac",
      "duration": "10.500000"
    }
  ],
  "format": {
    "duration": "10.500000",
    "format_name": "mov,mp4,m4a,3gp,3g2,mj2"
  }
}
```

- [ ] **Step 2: 编写 ffmpeg_test.go**

创建 `internal/adapter/ffmpeg_test.go`：

```go
package adapter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFFprobeJSON_ValidOutput(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "ffprobe_output.json"))
	require.NoError(t, err)

	media, err := parseFFprobeJSON(data)
	require.NoError(t, err)

	assert.Equal(t, 1920, media.Width)
	assert.Equal(t, 1080, media.Height)
	assert.Equal(t, 30.0, media.Fps)
	assert.Equal(t, int64(10500), media.DurationMs)
	assert.True(t, media.HasAudio)
	assert.Equal(t, "mov,mp4,m4a,3gp,3g2,mj2", media.Format)
}

func TestParseFFprobeJSON_NoAudio(t *testing.T) {
	data := []byte(`{
		"streams": [
			{"codec_type": "video", "width": 1280, "height": 720, "r_frame_rate": "25/1", "duration": "5.0"}
		],
		"format": {"duration": "5.0", "format_name": "mp4"}
	}`)

	media, err := parseFFprobeJSON(data)
	require.NoError(t, err)

	assert.False(t, media.HasAudio)
	assert.Equal(t, 1280, media.Width)
	assert.Equal(t, int64(5000), media.DurationMs)
}

func TestParseFFprobeJSON_InvalidJSON(t *testing.T) {
	_, err := parseFFprobeJSON([]byte(`{invalid}`))
	assert.Error(t, err)
}

func TestParseFrameRate_Fraction(t *testing.T) {
	assert.Equal(t, 30.0, parseFrameRate("30/1"))
	assert.Equal(t, 29.97, parseFrameRate("30000/1001"))
}

func TestParseFrameRate_Decimal(t *testing.T) {
	assert.Equal(t, 24.0, parseFrameRate("24"))
}

func TestParseFrameRate_Empty(t *testing.T) {
	assert.Equal(t, 0.0, parseFrameRate(""))
}

func TestParseDurationMs_Valid(t *testing.T) {
	assert.Equal(t, int64(10500), parseDurationMs("10.500000"))
	assert.Equal(t, int64(1000), parseDurationMs("1.0"))
}

func TestParseDurationMs_Empty(t *testing.T) {
	assert.Equal(t, int64(0), parseDurationMs(""))
}

func TestParseFFmpegProgress_ValidTime(t *testing.T) {
	// 标准 4 位 time= 格式
	ms := ParseFFmpegProgress("frame=  123 fps=30 q=24.0 size=     256kB time=00:01:23.45 bitrate=  30.0kbits/s")
	assert.Equal(t, int64(83450), ms)
}

func TestParseFFmpegProgress_HoursMinutesSeconds(t *testing.T) {
	ms := ParseFFmpegProgress("time=01:02:03.50")
	assert.Equal(t, int64(3723500), ms)
}

func TestParseFFmpegProgress_NoMatch(t *testing.T) {
	ms := ParseFFmpegProgress("some random stderr line")
	assert.Equal(t, int64(-1), ms)
}

func TestParseFFmpegProgress_EmptyLine(t *testing.T) {
	ms := ParseFFmpegProgress("")
	assert.Equal(t, int64(-1), ms)
}
```

- [ ] **Step 3: 运行测试**

```bash
go test ./internal/adapter/ -v -run "TestParseFFprobe|TestParseFrameRate|TestParseDurationMs|TestParseFFmpegProgress"
```

Expected: 全部 PASS。

- [ ] **Step 4: Commit**

```bash
git add internal/adapter/testdata/ffprobe_output.json internal/adapter/ffmpeg_test.go
git commit -m "test(adapter): add FFmpegAdapter parsing and progress tests"
```

---

### Task 6: LLMAdapter — 接口与实现

**Files:**
- Create: `internal/adapter/llm.go`

- [ ] **Step 1: 编写 llm.go**

创建 `internal/adapter/llm.go`：

```go
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
	Model       string             `json:"model"`
	Messages    []chatMessage      `json:"messages"`
	Temperature float64            `json:"temperature"`
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
	s = bytes.NewBufferString(s).String()

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

	return bytes.NewBufferString(s).String()
}
```

- [ ] **Step 2: 验证编译**

```bash
go build ./internal/adapter/
```

Expected: 无报错。

- [ ] **Step 3: Commit**

```bash
git add internal/adapter/llm.go
git commit -m "feat(adapter): add LLMAdapter with OpenAI-compatible API"
```

---

### Task 7: LLMAdapter — 单元测试

**Files:**
- Create: `internal/adapter/llm_test.go`

- [ ] **Step 1: 编写 llm_test.go**

创建 `internal/adapter/llm_test.go`：

```go
package adapter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smart-cut/internal/model"
)

func TestParseLLMResponse_ValidJSON(t *testing.T) {
	content := `{
		"removeSegmentIds": [1, 3],
		"items": [
			{"segmentId": 1, "reason": "filler", "confidence": 0.95, "note": "语气词'嗯'"},
			{"segmentId": 3, "reason": "dup_or_error", "confidence": 0.8, "note": "重复了上一句"}
		]
	}`

	result, err := parseLLMResponse(content)
	require.NoError(t, err)

	assert.Equal(t, []int{1, 3}, result.RemoveSegmentIDs)
	assert.Len(t, result.Items, 2)
	assert.Equal(t, 1, result.Items[0].SegmentID)
	assert.Equal(t, model.ReasonFiller, result.Items[0].Reason)
	assert.Equal(t, 0.95, result.Items[0].Confidence)
	assert.Equal(t, model.ReasonDupOrError, result.Items[1].Reason)
}

func TestParseLLMResponse_MarkdownWrapped(t *testing.T) {
	content := "```json\n" + `{
		"removeSegmentIds": [2],
		"items": [
			{"segmentId": 2, "reason": "silence", "confidence": 0.9, "note": "停顿过长"}
		]
	}` + "\n```"

	result, err := parseLLMResponse(content)
	require.NoError(t, err)
	assert.Equal(t, []int{2}, result.RemoveSegmentIDs)
	assert.Len(t, result.Items, 1)
}

func TestParseLLMResponse_EmptyItems(t *testing.T) {
	content := `{"removeSegmentIds": [], "items": []}`

	result, err := parseLLMResponse(content)
	require.NoError(t, err)
	assert.Empty(t, result.RemoveSegmentIDs)
	assert.Empty(t, result.Items)
}

func TestParseLLMResponse_InvalidJSON(t *testing.T) {
	content := `这不是JSON`

	_, err := parseLLMResponse(content)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "llm parse response")
}

func TestBuildSystemPrompt_ContainsInstructions(t *testing.T) {
	req := model.LLMAnalysisRequest{Language: "zh"}
	prompt := buildSystemPrompt(req)

	assert.Contains(t, prompt, "filler")
	assert.Contains(t, prompt, "silence")
	assert.Contains(t, prompt, "dup_or_error")
	assert.Contains(t, prompt, "JSON")
}

func TestBuildUserPrompt_ContainsSegments(t *testing.T) {
	req := model.LLMAnalysisRequest{
		Language: "zh",
		Segments: []model.LLMSegment{
			{ID: 0, StartMs: 0, EndMs: 1000, Text: "大家好"},
			{ID: 1, StartMs: 1000, EndMs: 2000, Text: "嗯今天聊聊"},
		},
	}

	prompt := buildUserPrompt(req)

	assert.Contains(t, prompt, "ID:0")
	assert.Contains(t, prompt, "大家好")
	assert.Contains(t, prompt, "ID:1")
	assert.Contains(t, prompt, "嗯今天聊聊")
	assert.Contains(t, prompt, "zh")
}

func TestCleanJSONResponse_PlainJSON(t *testing.T) {
	input := `{"key": "value"}`
	result := cleanJSONResponse(input)
	assert.Equal(t, `{"key": "value"}`, result)
}

func TestCleanJSONResponse_MarkdownBlock(t *testing.T) {
	input := "```json\n{\"key\": \"value\"}\n```"
	result := cleanJSONResponse(input)
	assert.Contains(t, result, `"key"`)
	assert.NotContains(t, result, "```json")
}

func TestLLMAnalyze_MissingConfig(t *testing.T) {
	adapter := NewLLMAdapter()
	req := model.LLMAnalysisRequest{Language: "zh"}

	_, err := adapter.Analyze(nil, req, model.LLMConfig{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}
```

- [ ] **Step 2: 运行全部 adapter 测试**

```bash
go test ./internal/adapter/ -v
```

Expected: 全部 PASS。

- [ ] **Step 3: Commit**

```bash
git add internal/adapter/llm_test.go
git commit -m "test(adapter): add LLMAdapter parsing and prompt tests"
```

---

### Task 8: 最终验证

**Files:**
- 无文件变更，仅验证

- [ ] **Step 1: 全量编译 + 测试**

```bash
go build ./internal/adapter/
go test ./internal/adapter/ -v
```

Expected: 编译无报错，全部测试 PASS。

- [ ] **Step 2: model 层回归测试**

```bash
go test ./internal/model/ -v
```

Expected: 仍然全部 PASS（确认没有破坏 model 层）。

- [ ] **Step 3: 主程序编译**

```bash
go build .
```

Expected: 无报错。

- [ ] **Step 4: 最终 commit**

```bash
git add -A
git commit -m "chore: plan 2 complete - adapter layer ready"
```

---

## 完成标准

Plan 2 完成后应满足：
1. ✅ `go build ./internal/adapter/` 无报错
2. ✅ `go test ./internal/adapter/ -v` 全部 PASS
3. ✅ `go test ./internal/model/ -v` 仍然全部 PASS
4. ✅ `go build .` 主程序编译通过
5. ✅ `internal/adapter/` 包含：
   - `binary.go` — BinaryResolver（三级查找）
   - `whisper.go` — WhisperAdapter（接口 + whisper-cli 实现 + JSON 解析）
   - `ffmpeg.go` — FFmpegAdapter（接口 + ffmpeg/ffprobe 实现 + Probe/Concat/Waveform/MuxSubtitle）
   - `llm.go` — LLMAdapter（接口 + OpenAI 兼容 HTTP 实现 + prompt 构建 + 响应解析）
   - 每个文件配齐单测
   - `testdata/` 目录含 fixture 文件

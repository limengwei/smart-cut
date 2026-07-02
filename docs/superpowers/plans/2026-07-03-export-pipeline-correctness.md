 # 导出管线生产正确性改造 Implementation Plan

 > **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

 **Goal:** 修复 Smart-Cut 导出管线的三个生产正确性缺陷（切点爆音、竖屏压扁、假无损），并增加 HDR 源的 tone mapping，使导出产物在任何源视频上都正确无瑕。

 **Architecture:** 将 ExportStep 从"单 filter_complex 一次性 trim+concat"重构为"逐段提取 → concat demuxer"模式。逐段提取阶段内嵌音频淡入淡出（防爆音）+ HDR tone mapping（防过啸）+ 竖屏感知缩放（防压扁），然后用 concat demuxer + `-c copy` 真无损拼接。同时扩展 Probe 采集 `color_transfer` 字段。

 **Tech Stack:** Go 1.25, ffmpeg/ffprobe（exec 调用），Go testify

 **前置条件：** Plan 1-5 已完成，主线 transcribe→analyze→edit→export 已跑通。当前 `internal/adapter/ffmpeg.go` 的 `ConcatLossless`/`ConcatReencode` 使用单 filter_complex 路径（需重构）。

 **参考依据：** [docs/superpowers/follow-ups/2026-07-03-video-use-borrow-points.md](file:///d:/workspace/go/src/smart-cut/docs/superpowers/follow-ups/2026-07-03-video-use-borrow-points.md) 的 A1/A2/A3/B1 项，源自 [browser-use/video-use](https://github.com/browser-use/video-use) 的生产验证经验。

 ---

 ## File Structure

 本 plan 涉及的文件变更：

 ```
 smart-cut/
 ├── internal/
 │   ├── model/
 │   │   └── project.go                # 修改：MediaFile 新增 ColorTransfer 字段
 │   └── adapter/
 │       ├── ffmpeg.go                  # 重构：Probe 采集 color_transfer；ConcatLossless/ConcatReencode 改逐段提取
 │       ├── ffmpeg_test.go             # 修改：扩展 ffprobe 解析测试覆盖 color_transfer
 │       ├── ffmpeg_concat_test.go      # 新建：逐段提取 + concat 的纯函数测试
 │       └── testdata/
 │           └── ffprobe_output.json    # 修改：新增 color_transfer 字段
 └── internal/pipeline/
     └── steps.go                       # 修改：顺手修 NewTranscribeStep unreachable code bug
 ```

 ---

 ### Task 1: 扩展 MediaFile 与 Probe 采集 color_transfer

 **Files:**
 - Modify: `internal/model/project.go`
 - Modify: `internal/adapter/ffmpeg.go`
 - Modify: `internal/adapter/testdata/ffprobe_output.json`
 - Modify: `internal/adapter/ffmpeg_test.go`

 - [ ] **Step 1: MediaFile 新增 ColorTransfer 字段**

 编辑 `internal/model/project.go`，在 `MediaFile` 结构体的 `HasAudio` 字段下方新增：

 ```go
 // MediaFile 媒体文件元信息
 type MediaFile struct {
 	Path         string  `json:"path"`
 	DurationMs   int64   `json:"durationMs"`
 	Format       string  `json:"format"`
 	Width        int     `json:"width"`
 	Height       int     `json:"height"`
 	Fps          float64 `json:"fps"`
 	HasAudio     bool    `json:"hasAudio"`
 	ColorTransfer string `json:"colorTransfer"` // 颜色传输函数，如 smpte2084(PQ)/arib-std-b67(HLG)，用于 HDR 检测
 }
 ```

 - [ ] **Step 2: ffprobe JSON 结构体新增 ColorTransfer 字段**

 编辑 `internal/adapter/ffmpeg.go`，在 `ffprobeJSONOutput.Streams` 结构体内（`Duration string` 之后）新增字段：

 ```go
 type ffprobeJSONOutput struct {
 	Streams []struct {
 		CodecType     string `json:"codec_type"`
 		CodecName     string `json:"codec_name"`
 		Width         int    `json:"width"`
 		Height        int    `json:"height"`
 		RFrameRate    string `json:"r_frame_rate"`
 		Duration      string `json:"duration"`
 		ColorTransfer string `json:"color_transfer"`
 	} `json:"streams"`
 	Format struct {
 		Duration   string `json:"duration"`
 		FormatName string `json:"format_name"`
 	} `json:"format"`
 }
 ```

 - [ ] **Step 3: parseFFprobeJSON 解析 ColorTransfer**

 编辑 `internal/adapter/ffmpeg.go` 的 `parseFFprobeJSON`，在 video stream 分支内（`media.Fps = parseFrameRate(stream.RFrameRate)` 之后）新增：

 ```go
 for _, stream := range raw.Streams {
 	if stream.CodecType == "video" {
 		media.Width = stream.Width
 		media.Height = stream.Height
 		media.Fps = parseFrameRate(stream.RFrameRate)
 		media.ColorTransfer = stream.ColorTransfer
 	}
 	if stream.CodecType == "audio" {
 		media.HasAudio = true
 	}
 }
 ```

 - [ ] **Step 4: 更新 testdata/ffprobe_output.json 增加 color_transfer**

 用以下内容替换 `internal/adapter/testdata/ffprobe_output.json`：

 ```json
 {
   "streams": [
     {
       "codec_type": "video",
       "codec_name": "h264",
       "width": 1920,
       "height": 1080,
       "r_frame_rate": "30/1",
       "duration": "10.500000",
       "color_transfer": "bt709"
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

 - [ ] **Step 5: 扩展 ffmpeg_test.go 验证 ColorTransfer 解析**

 编辑 `internal/adapter/ffmpeg_test.go`，在 `TestParseFFprobeJSON_ValidOutput` 的断言块末尾（`assert.Equal(t, "mov,mp4,m4a,3gp,3g2,mj2", media.Format)` 之后）新增：

 ```go
 	assert.Equal(t, "bt709", media.ColorTransfer)
 ```

 并在文件末尾新增一个 HDR 检测专用测试：

 ```go
 func TestParseFFprobeJSON_HDRTransfer(t *testing.T) {
 	data := []byte(`{
 		"streams": [
 			{"codec_type": "video", "width": 1920, "height": 1080, "r_frame_rate": "30/1", "duration": "10.5", "color_transfer": "arib-std-b67"}
 		],
 		"format": {"duration": "10.5", "format_name": "mp4"}
 	}`)

 	media, err := parseFFprobeJSON(data)
 	require.NoError(t, err)
 	assert.Equal(t, "arib-std-b67", media.ColorTransfer)
 }
 ```

 - [ ] **Step 6: 验证编译 + 测试通过**

 ```bash
 go build ./internal/model/ ./internal/adapter/
 go test ./internal/adapter/ -run TestParseFFprobeJSON -v -count=1
 ```

 预期：3 个 ffprobe 测试全部 PASS（ValidOutput 现在断言 ColorTransfer="bt709"，新增 HDRTransfer 断言 arib-std-b67）。

 - [ ] **Step 7: Commit**

 ```bash
 git add internal/model/project.go internal/adapter/ffmpeg.go internal/adapter/ffmpeg_test.go internal/adapter/testdata/ffprobe_output.json
 git commit -m "feat(ffmpeg): probe color_transfer for HDR detection"
 ```

 ---

 ### Task 2: 新增 HDR 检测与 tone mapping filter 构造（纯函数）

 **Files:**
 - Modify: `internal/adapter/ffmpeg.go`
 - Create: `internal/adapter/ffmpeg_concat_test.go`

 - [ ] **Step 1: 新增 HDR 检测与 tone map chain 常量**

 编辑 `internal/adapter/ffmpeg.go`，在 `ffmpegProgressRegex` 变量声明上方新增：

 ```go
 // HDRTransfers 触发 tone mapping 的传输函数集合（PQ/HDR10 与 HLG）
 var HDRTransfers = map[string]bool{
 	"smpte2084":    true, // PQ (HDR10)
 	"arib-std-b67": true, // HLG
 }

 // TonemapChain HDR → SDR 的 zscale+tonemap filter 链
 // 顺序：线性化 → 浮点 → bt709 色域 → hable tonemap → bt709 传输 → yuv420p
 const TonemapChain = "zscale=t=linear:npl=100," +
 	"format=gbrpf32le," +
 	"zscale=p=bt709," +
 	"tonemap=tonemap=hable:desat=0," +
 	"zscale=t=bt709:m=bt709:r=tv," +
 	"format=yuv420p"

 // IsHDR 判断媒体是否为 HDR 源（PQ 或 HLG 传输函数）
 func IsHDR(transfer string) bool {
 	return HDRTransfers[transfer]
 }

 // BuildVFChain 构造视频 filter 链（纯函数，可测试）
 // 顺序：HDR tone map（仅 HDR 源）→ 缩放（竖屏感知）
 // portrait=true 时按高度缩放，否则按宽度
 func BuildVFChain(colorTransfer string, portrait bool, targetWidth, targetHeight int, extraFilters []string) string {
 	var parts []string
 	if IsHDR(colorTransfer) {
 		parts = append(parts, TonemapChain)
 	}
 	// 竖屏感知缩放：保持目标短边，长边 -2 自动对齐
 	var scale string
 	if portrait {
 		scale = fmt.Sprintf("scale=-2:%d", targetHeight)
 	} else {
 		scale = fmt.Sprintf("scale=%d:-2", targetWidth)
 	}
 	parts = append(parts, scale)
 	parts = append(parts, extraFilters...)
 	return strings.Join(parts, ",")
 }

 // IsPortrait 判断是否竖屏（height > width）
 func IsPortrait(width, height int) bool {
 	return height > width
 }

 // BuildAudioFadeChain 构造 30ms 音频淡入淡出 filter 链（防爆音）
 // durationSec 为本段时长（秒）
 func BuildAudioFadeChain(durationSec float64) string {
 	fadeOutStart := durationSec - 0.03
 	if fadeOutStart < 0 {
 		fadeOutStart = 0
 	}
 	return fmt.Sprintf("afade=t=in:st=0:d=0.03,afade=t=out:st=%.3f:d=0.03", fadeOutStart)
 }
 ```

 - [ ] **Step 2: 新增 ffmpeg_concat_test.go 纯函数测试**

 创建 `internal/adapter/ffmpeg_concat_test.go`：

 ```go
 package adapter

 import (
 	"strings"
 	"testing"

 	"github.com/stretchr/testify/assert"
 )

 func TestIsHDR_PQ(t *testing.T) {
 	assert.True(t, IsHDR("smpte2084"))
 }

 func TestIsHDR_HLG(t *testing.T) {
 	assert.True(t, IsHDR("arib-std-b67"))
 }

 func TestIsHDR_SDR(t *testing.T) {
 	assert.False(t, IsHDR("bt709"))
 }

 func TestIsHDR_Empty(t *testing.T) {
 	assert.False(t, IsHDR(""))
 }

 func TestIsPortrait_Landscape(t *testing.T) {
 	assert.False(t, IsPortrait(1920, 1080))
 }

 func TestIsPortrait_Portrait(t *testing.T) {
 	assert.True(t, IsPortrait(1080, 1920))
 }

 func TestIsPortrait_Square(t *testing.T) {
 	assert.False(t, IsPortrait(1080, 1080))
 }

 func TestBuildVFChain_SDR_Landscape(t *testing.T) {
 	chain := BuildVFChain("bt709", false, 1920, 1080, nil)
 	// SDR 无 tonemap，只有 scale
 	assert.False(t, strings.Contains(chain, "tonemap"))
 	assert.True(t, strings.Contains(chain, "scale=1920:-2"))
 }

 func TestBuildVFChain_HDR_Landscape(t *testing.T) {
 	chain := BuildVFChain("smpte2084", false, 1920, 1080, nil)
 	// HDR 应含 tonemap 链 + scale
 	assert.True(t, strings.Contains(chain, "tonemap=hable"))
 	assert.True(t, strings.Contains(chain, "scale=1920:-2"))
 }

 func TestBuildVFChain_Portrait(t *testing.T) {
 	chain := BuildVFChain("bt709", true, 1920, 1080, nil)
 	// 竖屏按高度缩放
 	assert.True(t, strings.Contains(chain, "scale=-2:1080"))
 }

 func TestBuildVFChain_WithExtraFilters(t *testing.T) {
 	chain := BuildVFChain("bt709", false, 1920, 1080, []string{"eq=contrast=1.03"})
 	assert.True(t, strings.Contains(chain, "eq=contrast=1.03"))
 }

 func TestBuildAudioFadeChain_Normal(t *testing.T) {
 	chain := BuildAudioFadeChain(5.0)
 	assert.True(t, strings.Contains(chain, "afade=t=in:st=0:d=0.03"))
 	assert.True(t, strings.Contains(chain, "afade=t=out:st=4.970:d=0.03"))
 }

 func TestBuildAudioFadeChain_VeryShort(t *testing.T) {
 	// 短于 30ms 的段：fadeOutStart 钳为 0，不产生负值
 	chain := BuildAudioFadeChain(0.02)
 	assert.True(t, strings.Contains(chain, "afade=t=out:st=0.000:d=0.03"))
 }
 ```

 - [ ] **Step 3: 运行测试**

 ```bash
 go test ./internal/adapter/ -run "TestIsHDR|TestIsPortrait|TestBuildVFChain|TestBuildAudioFadeChain" -v -count=1
 ```

 预期：12 个测试全部 PASS。

 - [ ] **Step 4: Commit**

 ```bash
 git add internal/adapter/ffmpeg.go internal/adapter/ffmpeg_concat_test.go
 git commit -m "feat(ffmpeg): add HDR detection, portrait-aware scale, and audio fade chain builders"
 ```

 ---

 ### Task 3: 重构 FFmpegAdapter 接口为逐段提取 + concat demuxer

 **Files:**
 - Modify: `internal/adapter/ffmpeg.go`

 - [ ] **Step 1: 扩展 FFmpegAdapter 接口新增三个方法**

 编辑 `internal/adapter/ffmpeg.go`，将 `FFmpegAdapter` interface 替换为：

 ```go
 type FFmpegAdapter interface {
 	Probe(ctx context.Context, path string) (*model.MediaFile, error)
 	ExtractWaveform(ctx context.Context, mediaPath, outPng string) error
 	ExtractAudio16kWav(ctx context.Context, mediaPath, outWav string) error
 	ExtractWaveformPeaks(ctx context.Context, mediaPath string, durationMs int64, buckets int) (*model.WaveformPeaks, error)

 	// ExtractSegment 逐段提取（内嵌音频淡入淡出 + HDR tone map + 竖屏感知缩放）
 	// sourcePath: 源视频；segStartSec/segEndSec: 段起止秒；
 	// media: 源媒体元信息（用于 HDR/竖屏判断）；outPath: 输出 mp4
 	ExtractSegment(ctx context.Context, sourcePath string, segStartSec, segEndSec float64, media model.MediaFile, outPath string) error

 	// ConcatDemuxer 用 concat demuxer + -c copy 真无损拼接
 	// segmentPaths: 已提取的段 mp4 列表；outPath: 输出
 	ConcatDemuxer(ctx context.Context, segmentPaths []string, outPath string) error

 	// ConcatReencode 重编码拼接（保留旧接口，逐段提取 + 重编码 concat，用于需要统一编码参数的场景）
 	ConcatReencode(ctx context.Context, segments []model.KeepSegment, sourcePath, outPath string, opts model.EncodeOpts) error

 	MuxSubtitle(ctx context.Context, videoPath, subtitleClipPath, outPath string) error
 }
 ```

 **注意**：移除旧的 `ConcatLossless`（被 `ExtractSegment` + `ConcatDemuxer` 组合替代）。

 - [ ] **Step 2: 实现 ExtractSegment**

 编辑 `internal/adapter/ffmpeg.go`，删除旧的 `ConcatLossless` 方法，在 `ExtractAudio16kWav` 方法之后新增：

 ```go
 // ExtractSegment 逐段提取：-ss before -i 快速精确 seek + 内嵌 vf/af
 func (a *ffmpegAdapter) ExtractSegment(ctx context.Context, sourcePath string, segStartSec, segEndSec float64, media model.MediaFile, outPath string) error {
 	binaryPath, err := a.resolver.Resolve("ffmpeg")
 	if err != nil {
 		return fmt.Errorf("ffmpeg extract segment: %w", err)
 	}

 	duration := segEndSec - segStartSec
 	if duration <= 0 {
 		return fmt.Errorf("ffmpeg extract segment: invalid duration %.3f", duration)
 	}

 	portrait := IsPortrait(media.Width, media.Height)
 	vf := BuildVFChain(media.ColorTransfer, portrait, 1920, 1080, nil)
	// 构造 args：-af 仅在有音频时加入（无音频段加 -af 会报错）；outPath 必须在最后
	args := []string{
		"-y",
		"-ss", fmt.Sprintf("%.3f", segStartSec),
		"-i", sourcePath,
		"-t", fmt.Sprintf("%.3f", duration),
		"-vf", vf,
		"-c:v", "libx264", "-preset", "fast", "-crf", "20",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "192k", "-ar", "48000",
		"-movflags", "+faststart",
	}
	if media.HasAudio {
		args = append(args, "-af", BuildAudioFadeChain(duration))
	}
	args = append(args, outPath)

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, binaryPath, args...)
 	cmd.Stderr = &stderr
 	if err := cmd.Run(); err != nil {
 		return fmt.Errorf("ffmpeg extract segment: %w (stderr: %s)", err, stderr.String())
 	}
 	return nil
 }


> **实现要点**：(1) `-af` 仅在 `media.HasAudio` 时加入（无音频段加 `-af` 会报错）；(2) `-af` 是输出选项须在 `-i` 之后；(3) `outPath` 必须是 args 最后一个位置参数，故先 append `-af` 再 append `outPath`。

 - [ ] **Step 3: 实现 ConcatDemuxer**

> 编辑 `internal/adapter/ffmpeg.go`，在 `ExtractSegment` 方法之后新增：

 ```go
 // ConcatDemuxer 用 concat demuxer + -c copy 真无损拼接（要求各段编码参数一致）
 func (a *ffmpegAdapter) ConcatDemuxer(ctx context.Context, segmentPaths []string, outPath string) error {
 	binaryPath, err := a.resolver.Resolve("ffmpeg")
 	if err != nil {
 		return fmt.Errorf("ffmpeg concat demuxer: %w", err)
 	}
 	if len(segmentPaths) == 0 {
 		return fmt.Errorf("ffmpeg concat demuxer: no segments")
 	}

 	// 写 concat 列表文件到 outPath 同目录
 	listPath := outPath + ".concat.txt"
 	var listContent strings.Builder
 	for _, p := range segmentPaths {
 		// 用绝对路径，避免工作目录问题
 		abs, err := filepath.Abs(p)
 		if err != nil {
 			abs = p
 		}
 		listContent.WriteString(fmt.Sprintf("file '%s'\n", abs))
 	}
 	if err := os.WriteFile(listPath, []byte(listContent.String()), 0644); err != nil {
 		return fmt.Errorf("ffmpeg concat demuxer: write list: %w", err)
 	}

 	args := []string{
 		"-y",
 		"-f", "concat", "-safe", "0",
 		"-i", listPath,
 		"-c", "copy",
 		"-movflags", "+faststart",
 		outPath,
 	}

 	var stderr bytes.Buffer
 	cmd := exec.CommandContext(ctx, binaryPath, args...)
 	cmd.Stderr = &stderr
 	if err := cmd.Run(); err != nil {
 		return fmt.Errorf("ffmpeg concat demuxer: %w (stderr: %s)", err, stderr.String())
 	}
 	return nil
 }
 ```

 - [ ] **Step 4: 补充 import**

> 编辑 `internal/adapter/ffmpeg.go` 的 import 块，确保含 `"os"` 和 `"path/filepath"`（若已有则跳过）：

 ```go
 import (
 	"bytes"
 	"context"
 	"encoding/binary"
 	"encoding/json"
 	"fmt"
 	"io"
 	"os"
 	"os/exec"
 	"path/filepath"
 	"regexp"
 	"strconv"
 	"strings"

 	"smart-cut/internal/model"
 )
 ```

 - [ ] **Step 5: 验证编译**

> ```bash
 go build ./internal/adapter/
 ```

> 预期：无报错。若报 `ConcatLossless` 未定义（来自 ExportStep 调用），属正常 —— Task 4 会修复 ExportStep。

 - [ ] **Step 6: Commit**

> ```bash
 git add internal/adapter/ffmpeg.go
 git commit -m "refactor(ffmpeg): replace ConcatLossless with ExtractSegment + ConcatDemuxer"
 ```

 ---

 ### Task 4: 适配 ExportStep 使用逐段提取 + concat

 **Files:**
 - Modify: `internal/pipeline/steps.go`
 - Modify: `internal/pipeline/steps_test.go`（若存在 mock，需更新接口）

 - [ ] **Step 1: 检查现有 steps_test.go 的 mock 是否依赖旧接口**

> 先查看测试文件是否 mock 了 FFmpegAdapter：

 ```bash
 go doc -all ./internal/pipeline/ 2>&1 | findstr /i "ffmpeg"
 ```

> 若有 mock，需在 Step 2 同步更新 mock 实现新接口（ExtractSegment + ConcatDemuxer）。

 - [ ] **Step 2: 重写 ExportStep.Run**

> 编辑 `internal/pipeline/steps.go`，将 `ExportStep` 的 `Run` 方法替换为：

 ```go
 func (s *ExportStep) Run(ctx *Context, reporter ProgressReporter) error {
 	if ctx.CutList == nil {
 		return fmt.Errorf("export: cutlist is nil")
 	}

 	reporter.Report("export", "preparing segments", 0.1)

 	keepSegments := ctx.CutList.KeepSegments()
 	if len(keepSegments) == 0 {
 		return fmt.Errorf("export: no keep segments")
 	}

 	sourcePath := ctx.Project.Media.Path
 	media := ctx.Project.Media

 	// 逐段提取到临时目录
 	tmpDir := filepath.Join(ctx.Project.WorkDir, "cuts")
 	if err := os.MkdirAll(tmpDir, 0755); err != nil {
 		return fmt.Errorf("export: create cuts dir: %w", err)
 	}

 	reporter.Report("export", fmt.Sprintf("extracting %d segments", len(keepSegments)), 0.2)

 	var segPaths []string
 	for i, seg := range keepSegments {
 		startSec := float64(seg.StartMs) / 1000.0
 		endSec := float64(seg.EndMs) / 1000.0
 		outPath := filepath.Join(tmpDir, fmt.Sprintf("keep_%03d.mp4", i+1))

 		if err := s.ffmpeg.ExtractSegment(ctx.Cancel, sourcePath, startSec, endSec, media, outPath); err != nil {
 			return fmt.Errorf("export: extract segment %d: %w", i+1, err)
 		}
 		segPaths = append(segPaths, outPath)

 		reporter.Report("export", fmt.Sprintf("extracted %d/%d", i+1, len(keepSegments)), 0.2+0.5*float64(i+1)/float64(len(keepSegments)))
 	}

 	// 拼接
 	reporter.Report("export", "concatenating", 0.7)

 	outPath := s.exportOpts.OutputPath
 	if outPath == "" {
 		outPath = filepath.Join(ctx.Project.WorkDir, "export.mp4")
 	}

 	if err := s.ffmpeg.ConcatDemuxer(ctx.Cancel, segPaths, outPath); err != nil {
 		return fmt.Errorf("export: concat: %w", err)
 	}

 	ctx.ExportPath = outPath

 	reporter.Report("export", "completed", 1.0)
 	reporter.Done("export", outPath)

 	return nil
 }
 ```

 - [ ] **Step 3: 顺手修复 NewTranscribeStep unreachable code**

> 编辑 `internal/pipeline/steps.go` 的 `NewTranscribeStep`，删除第二行 return（line 24）：

 修改前：
 ```go
 func NewTranscribeStep(whisper adapter.WhisperAdapter, ffmpeg adapter.FFmpegAdapter, opts adapter.WhisperOptions, bus *eventbus.EventBus) *TranscribeStep {
 	return &TranscribeStep{whisper: whisper, ffmpeg: ffmpeg, opts: opts, bus: bus}
 	return &TranscribeStep{whisper: whisper, ffmpeg: ffmpeg, opts: opts}
 }
 ```

 修改后：
 ```go
 func NewTranscribeStep(whisper adapter.WhisperAdapter, ffmpeg adapter.FFmpegAdapter, opts adapter.WhisperOptions, bus *eventbus.EventBus) *TranscribeStep {
 	return &TranscribeStep{whisper: whisper, ffmpeg: ffmpeg, opts: opts, bus: bus}
 }
 ```

 - [ ] **Step 4: 更新 steps_test.go 的 mock（如存在）**

> 若 `internal/pipeline/steps_test.go` 中有 FFmpegAdapter 的 mock，需补充 `ExtractSegment` 和 `ConcatDemuxer` 方法。在每个 mock 结构体上新增：

 ```go
 func (m *mockFFmpeg) ExtractSegment(ctx context.Context, sourcePath string, segStartSec, segEndSec float64, media model.MediaFile, outPath string) error {
 	return nil
 }

 func (m *mockFFmpeg) ConcatDemuxer(ctx context.Context, segmentPaths []string, outPath string) error {
 	return nil
 }
 ```

> 并移除对 `ConcatLossless` 的 mock（若存在）。

 - [ ] **Step 5: 验证编译 + 测试**

> ```bash
 go build ./...
 go test ./internal/pipeline/ -v -count=1
 ```

> 预期：编译通过，pipeline 测试全部 PASS。

 - [ ] **Step 6: Commit**

> ```bash
 git add internal/pipeline/steps.go internal/pipeline/steps_test.go
 git commit -m "refactor(export): use per-segment extract + concat demuxer; fix unreachable code"
 ```

 ---

 ### Task 5: 全量验证

 **Files:** 无（仅运行验证命令）

 - [ ] **Step 1: Go 全量编译**

> ```bash
 go build ./...
 ```

> 预期：成功（忽略 `build/ios` 缺 main 的平台噪音）。

 - [ ] **Step 2: go vet**

> ```bash
 go vet ./internal/... ./app/...
 ```

> 预期：无 unreachable code 警告（Task 4 Step 3 已修）。

 - [ ] **Step 3: 全量测试**

> ```bash
 go test ./internal/... ./app/... -count=1
 ```

> 预期：所有包 PASS，新增的 HDR/竖屏/音频淡入淡出测试通过。

 - [ ] **Step 4: 手动冒烟（需 ffmpeg 可用）**

> 执行 `wails3 dev`，新建项目 → 转录 → 分析 → 导出，确认：
 - [ ] 导出 mp4 切点处无爆音（对比改造前）
 - [ ] 竖屏源导出方向正确（无压扁）
 - [ ] 普通 SDR 源导出正常（回归无破坏）
 - [ ] HDR 源导出无过啸（若有 HDR 测试素材）

 ---

 ## Self-Review

 **1. Spec 覆盖检查（对照 follow-ups A1/A2/A3/B1）：**

 | follow-up 项 | 对应任务 | 状态 |
 |---|---|---|
 | A1 切点 30ms 音频淡入淡出 | Task 2 BuildAudioFadeChain + Task 3 ExtractSegment 内嵌 | ✅ |
 | A2 HDR tone mapping | Task 1 ColorTransfer 采集 + Task 2 IsHDR/TonemapChain + Task 3 ExtractSegment 前置 | ✅ |
 | A3 竖屏方向保持 | Task 2 IsPortrait/BuildVFChain + Task 3 ExtractSegment 应用 | ✅ |
 | B1 ConcatLossless 改逐段提取+concat demuxer | Task 3 ExtractSegment+ConcatDemuxer + Task 4 ExportStep 适配 | ✅ |

> **2. 占位符扫描：**
 - 无 "TBD"/"TODO"/"类似上面"。
 - 所有代码步骤均含完整可运行代码。
 - Task 4 Step 1/4 对 steps_test.go mock 的处理为条件步骤（"若存在"），因当前未读取该文件内容 —— 执行时先读，按实际情况决定是否补 mock。这是必要的探查而非占位。

 **3. 类型一致性：**
 - `MediaFile.ColorTransfer`：Task 1 定义，Task 3 `ExtractSegment` 签名引用 `media model.MediaFile` 读取该字段 ✅
 - `ExtractSegment(ctx, sourcePath, segStartSec, segEndSec float64, media model.MediaFile, outPath)` 签名：Task 3 interface 定义、Task 3 实现、Task 4 ExportStep 调用三处一致 ✅
 - `ConcatDemuxer(ctx, segmentPaths []string, outPath)` 签名：Task 3 interface 定义、Task 3 实现、Task 4 ExportStep 调用三处一致 ✅
 - `BuildVFChain(colorTransfer, portrait, targetWidth, targetHeight, extraFilters)` 签名：Task 2 定义、Task 2 测试、Task 3 ExtractSegment 调用一致 ✅
 - `BuildAudioFadeChain(durationSec)` 签名：Task 2 定义、Task 2 测试、Task 3 ExtractSegment 调用一致 ✅
 - 旧的 `ConcatLossless` 被 `ExtractSegment`+`ConcatDemuxer` 替代，ExportStep 不再调用 ✅

 **4. 风险点（执行时关注）：**
 - **concat demuxer 要求各段编码参数一致**：ExtractSegment 用固定 libx264/CRF 20/aac 192k，所有段参数一致，满足要求。
 - **`-ss` before `-i` 的精度**：快速 seek 可能不精确到帧，但对音频淡入淡出 30ms 容差足够。若发现首帧黑屏，可改用 `-ss` after `-i`（慢但精确），代价是逐段提取变慢。
 - **Audio-only 段**：若 keep 段无音频（HasAudio=false），`-af` 会报错。需在 ExtractSegment 内判断 `media.HasAudio`，无音频时跳过 af。**执行时补这个判断**（Task 3 Step 2 实现里加 `if media.HasAudio` 分支）。
 - **大段提取内存**：逐段提取每段独立解码-编码，长视频（>30min）耗时线性增长，MVP 可接受。

 ---

 ## Execution Handoff

 Plan 已完成并保存到 `docs/superpowers/plans/2026-07-03-export-pipeline-correctness.md`。

 两种执行方式：

 **1. Subagent-Driven（推荐）** — 每个 Task 派发独立 subagent，任务间两阶段审查，快速迭代。

 **2. Inline Execution** — 在当前会话内按 Task 顺序执行，带检查点。

 **建议**：Task 1-2 独立可并行（model 扩展 + 纯函数）；Task 3 依赖 Task 1/2；Task 4 依赖 Task 3；Task 5 最后验证。建议串行执行保证接口一致性。

# Smart-Cut Plan 5: Timeline 编辑器

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现工作台（Workbench）核心 —— Timeline 编辑器（波形/字幕/剪切三轨道）+ 视频预览 + AI 建议面板，完成"转录 → 分析 → 编辑 → 导出"的完整工作流交互。

**Architecture:** Timeline 三轨道基于 `<canvas>` + React 自绘（不依赖 UI 库）。波形数据改为峰值采样数组（替代 Plan 4 的 PNG），以支持缩放与点击交互。时间↔像素映射通过纯函数计算。播放头通过 zustand store 同步视频与时间轴。媒体文件通过后端本地 HTTP 服务暴露给 webview（规避 file:// 安全限制）。

**Tech Stack:** Go（Wails3 v3.0.0-alpha.96, net/http），React 18, TypeScript, zustand, Tailwind CSS, lucide-react, @wailsio/runtime

**前置条件：** Plan 4 已完成 —— App 门面、API client、events.ts（已修复 `ev.data` 包装）、settings/project store、Shadcn 基础组件、Workbench 占位页均已就绪。

---

## File Structure

```
smart-cut/
├── internal/
│   ├── model/
│   │   └── waveform.go              # 新建：WaveformPeaks 类型
│   └── adapter/
│       └── ffmpeg.go                # 修改：新增 ExtractWaveformPeaks
├── app/
│   ├── app_async.go                 # 修改：新增 GetWaveformPeaks + GetMediaURL
│   ├── app_media.go                 # 新建：本地 HTTP 媒体服务
│   └── media_server_test.go         # 新建：媒体路由测试
├── internal/adapter/
│   └── ffmpeg_waveform_test.go      # 新建：computePeaks 纯函数测试
├── main.go                          # 修改：启动媒体 HTTP 服务
└── frontend/
    └── src/
        ├── api/
        │   ├── types.ts             # 修改：新增 WaveformPeaks 类型
        │   └── client.ts            # 修改：新增 getWaveformPeaks + getMediaURL
        ├── lib/
        │   └── timeline.ts          # 新建：时间↔像素映射、格式化、zoom 计算
        ├── stores/
        │   └── workbench.ts         # 新建：工作台状态（数据/视口/播放头/选中）
        ├── components/
        │   ├── timeline/
        │   │   ├── WaveformTrack.tsx   # 新建：canvas 波形渲染 + 点击跳转
        │   │   ├── SubtitleTrack.tsx   # 新建：字幕轨 + 点击跳转
        │   │   ├── CutTrack.tsx        # 新建：剪切轨（拖拽边界/右键切换/选中）
        │   │   ├── Playhead.tsx        # 新建：播放头覆盖层
        │   │   ├── ZoomControls.tsx    # 新建：缩放 [+] [-] [适应]
        │   │   └── Timeline.tsx        # 新建：三轨道容器
        │   ├── VideoPreview.tsx     # 新建：video 标签 + 播放控制
        │   └── AISuggestions.tsx    # 新建：右侧 AI 建议接受/拒绝面板
        └── pages/
            └── Workbench.tsx        # 重写：TopBar + 工作流 + 组装
```

---

### Task 1: 后端 —— 波形峰值提取 + 本地媒体 HTTP 服务

**Files:**
- Create: `internal/model/waveform.go`
- Modify: `internal/adapter/ffmpeg.go`（新增接口方法 + 实现）
- Create: `internal/adapter/ffmpeg_waveform_test.go`
- Modify: `internal/service/transcribe.go`（peaks 缓存 + GetWaveformPeaks）
- Modify: `app/app_async.go`（新增 GetWaveformPeaks）
- Create: `app/app_media.go`
- Create: `app/media_server_test.go`
- Modify: `app/app_async.go`（新增 GetMediaURL）
- Modify: `main.go`（启动媒体服务）

**背景：** Plan 4 的 `ExtractWaveform` 生成 PNG，无法支持缩放/点击交互。本任务新增 `ExtractWaveformPeaks` 返回 int16 峰值数组，前端用 canvas 自绘。同时新增本地 HTTP 服务暴露媒体文件给 webview。

- [ ] **Step 1: 创建 model/waveform.go**

创建 `internal/model/waveform.go`：

```go
package model

// WaveformPeaks 波形峰值采样数据（供前端 canvas 渲染）
// 每个桶存储 PCM int16 的 min/max，前端归一化时除以 32768
type WaveformPeaks struct {
	DurationMs int64   `json:"durationMs"` // 媒体总时长（毫秒）
	SampleRate int     `json:"sampleRate"` // PCM 采样率（Hz）
	Buckets    int     `json:"buckets"`    // 桶数量
	Mins       []int16 `json:"mins"`       // 每桶最小值
	Maxs       []int16 `json:"maxs"`       // 每桶最大值
}
```

- [ ] **Step 2: 修改 FFmpegAdapter 接口 + 实现**

编辑 `internal/adapter/ffmpeg.go`：

2a. 在 `FFmpegAdapter` interface 中新增方法（在现有 `ExtractWaveform` 下方）：

```go
type FFmpegAdapter interface {
	Probe(ctx context.Context, path string) (*model.MediaFile, error)
	ExtractWaveform(ctx context.Context, mediaPath, outPng string) error
	ExtractWaveformPeaks(ctx context.Context, mediaPath string, durationMs int64, buckets int) (*model.WaveformPeaks, error)
	ConcatLossless(ctx context.Context, segments []model.KeepSegment, sourcePath, outPath string) error
	ConcatReencode(ctx context.Context, segments []model.KeepSegment, sourcePath, outPath string, opts model.EncodeOpts) error
	MuxSubtitle(ctx context.Context, videoPath, subtitleClipPath, outPath string) error
}
```

2b. 在文件 import 块中增加 `"bytes"` 和 `"encoding/binary"` 和 `"io"`（若已存在则跳过）。

2c. 在文件末尾新增实现。注意将 PCM → 峰值的逻辑抽成可测试的纯函数：

```go
// extractWaveformPeaks 提取波形峰值采样数据
func (a *ffmpegAdapter) ExtractWaveformPeaks(ctx context.Context, mediaPath string, durationMs int64, buckets int) (*model.WaveformPeaks, error) {
	binaryPath, err := a.resolver.Resolve("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("ffmpeg waveform peaks: %w", err)
	}

	if buckets <= 0 {
		buckets = 2000
	}

	const sampleRate = 8000
	totalSamples := int(int64(sampleRate) * durationMs / 1000)
	if totalSamples <= 0 {
		return nil, fmt.Errorf("invalid duration %dms for waveform", durationMs)
	}
	samplesPerBucket := totalSamples / buckets
	if samplesPerBucket < 1 {
		samplesPerBucket = 1
	}

	// ffmpeg 输出 raw PCM s16le mono
	cmd := exec.CommandContext(ctx, binaryPath,
		"-i", mediaPath,
		"-f", "s16le",
		"-acodec", "pcm_s16le",
		"-ac", "1",
		"-ar", fmt.Sprintf("%d", sampleRate),
		"-",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg waveform pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("ffmpeg waveform start: %w", err)
	}

	mins, maxs := computePeaksFromReader(stdout, samplesPerBucket, buckets)

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("ffmpeg waveform wait: %w (%s)", err, stderr.String())
	}

	return &model.WaveformPeaks{
		DurationMs: durationMs,
		SampleRate: sampleRate,
		Buckets:    len(mins),
		Mins:       mins,
		Maxs:       maxs,
	}, nil
}

// computePeaksFromReader 从 PCM int16 流计算每桶 min/max 峰值
func computePeaksFromReader(r io.Reader, samplesPerBucket, buckets int) (mins, maxs []int16) {
	mins = make([]int16, 0, buckets)
	maxs = make([]int16, 0, buckets)
	buf := make([]byte, samplesPerBucket*2)

	for i := 0; i < buckets; i++ {
		n, err := io.ReadFull(r, buf)
		if n == 0 {
			break
		}
		readSamples := n / 2
		var minV, maxV int16
		for j := 0; j < readSamples; j++ {
			v := int16(binary.LittleEndian.Uint16(buf[j*2 : j*2+2]))
			if j == 0 || v < minV {
				minV = v
			}
			if j == 0 || v > maxV {
				maxV = v
			}
		}
		mins = append(mins, minV)
		maxs = append(maxs, maxV)
		if err != nil {
			break
		}
	}
	return mins, maxs
}

// computePeaks 纯函数版本（用于测试，不依赖 IO）
func computePeaks(samples []int16, samplesPerBucket int) (mins, maxs []int16) {
	if samplesPerBucket < 1 {
		samplesPerBucket = 1
	}
	buckets := (len(samples) + samplesPerBucket - 1) / samplesPerBucket
	mins = make([]int16, 0, buckets)
	maxs = make([]int16, 0, buckets)
	for i := 0; i < len(samples); i += samplesPerBucket {
		end := i + samplesPerBucket
		if end > len(samples) {
			end = len(samples)
		}
		var minV, maxV int16
		for j, v := range samples[i:end] {
			if j == 0 || v < minV {
				minV = v
			}
			if j == 0 || v > maxV {
				maxV = v
			}
		}
		mins = append(mins, minV)
		maxs = append(maxs, maxV)
	}
	return mins, maxs
}
```

- [ ] **Step 3: 创建 ffmpeg_waveform_test.go**

创建 `internal/adapter/ffmpeg_waveform_test.go`，测试纯函数 `computePeaks`：

```go
package adapter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComputePeaks_BasicBuckets(t *testing.T) {
	// 8 个采样，每桶 2 个 → 4 桶
	samples := []int16{100, -200, 300, -400, 500, -600, 700, -800}
	mins, maxs := computePeaks(samples, 2)

	assert.Len(t, mins, 4)
	assert.Len(t, maxs, 4)

	assert.Equal(t, int16(-200), mins[0])
	assert.Equal(t, int16(100), maxs[0])

	assert.Equal(t, int16(-400), mins[1])
	assert.Equal(t, int16(300), maxs[1])

	assert.Equal(t, int16(-600), mins[2])
	assert.Equal(t, int16(500), maxs[2])

	assert.Equal(t, int16(-800), mins[3])
	assert.Equal(t, int16(700), maxs[3])
}

func TestComputePeaks_LastBucketPartial(t *testing.T) {
	// 5 个采样，每桶 2 个 → 3 桶（最后一桶只有 1 个采样）
	samples := []int16{10, 20, 30, 40, 50}
	mins, maxs := computePeaks(samples, 2)

	assert.Len(t, mins, 3)
	assert.Len(t, maxs, 3)

	// 桶0: [10,20]
	assert.Equal(t, int16(10), mins[0])
	assert.Equal(t, int16(20), maxs[0])
	// 桶2: [50]
	assert.Equal(t, int16(50), mins[2])
	assert.Equal(t, int16(50), maxs[2])
}

func TestComputePeaks_SamplesPerBucketClampedTo1(t *testing.T) {
	samples := []int16{5, -5, 10, -10}
	mins, maxs := computePeaks(samples, 0) // 0 应被钳为 1

	assert.Len(t, mins, 4)
	assert.Equal(t, int16(5), maxs[0])
	assert.Equal(t, int16(-5), mins[1])
}

func TestComputePeaks_EmptySamples(t *testing.T) {
	mins, maxs := computePeaks([]int16{}, 2)
	assert.Empty(t, mins)
	assert.Empty(t, maxs)
}
```

- [ ] **Step 4: 运行测试**

```bash
go test ./internal/adapter/... -run TestComputePeaks -v -count=1
```

预期：4 个测试全部 PASS。

- [ ] **Step 5: 修改 TranscribeService —— peaks 缓存**

编辑 `internal/service/transcribe.go`：

5a. 在 `TranscribeService` 结构体中增加字段（在 `transcripts sync.Map` 下方）：

```go
type TranscribeService struct {
	whisper     adapter.WhisperAdapter
	ffmpeg      adapter.FFmpegAdapter
	bus         *eventbus.EventBus
	editSvc     *EditService
	transcripts sync.Map // projectID → *model.Transcript
	peaks       sync.Map // projectID → *model.WaveformPeaks
}
```

5b. 在文件末尾新增 `GetWaveformPeaks` 方法（在 `ExtractWaveform` 方法之后）：

```go
// GetWaveformPeaks 获取项目的波形峰值（不存在则提取）
func (s *TranscribeService) GetWaveformPeaks(ctx context.Context, project *model.Project) (*model.WaveformPeaks, error) {
	if cached, ok := s.peaks.Load(project.ID); ok {
		return cached.(*model.WaveformPeaks), nil
	}

	peaks, err := s.ffmpeg.ExtractWaveformPeaks(ctx, project.Media.Path, project.Media.DurationMs, 2000)
	if err != nil {
		return nil, err
	}

	s.peaks.Store(project.ID, peaks)
	return peaks, nil
}
```

- [ ] **Step 6: 修改 app/app_async.go —— GetWaveformPeaks 绑定**

编辑 `app/app_async.go`，在 `GetWaveform` 方法之后新增：

```go
// GetWaveformPeaks 获取波形峰值采样数据（供前端 canvas 渲染）
func (a *App) GetWaveformPeaks(projectID string) (*model.WaveformPeaks, error) {
	project, err := a.GetProject(projectID)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	peaks, err := a.transcribeService.GetWaveformPeaks(ctx, project)
	if err != nil {
		return nil, NewAppError(ErrCodeInternal, "提取波形数据失败", err.Error())
	}
	return peaks, nil
}
```

- [ ] **Step 7: 创建 app/app_media.go —— 本地媒体 HTTP 服务**

创建 `app/app_media.go`。此服务用独立 HTTP server 暴露媒体文件给 webview，规避 file:// 安全限制：

```go
package app

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
)

// mediaServer 本地 HTTP 服务，按 projectID 暴露媒体文件
type mediaServer struct {
	mu       sync.RWMutex
	paths    map[string]string // projectID → 媒体文件绝对路径
	server   *http.Server
	baseURL  string
}

// NewMediaServer 创建并启动本地媒体服务（监听随机端口）
func NewMediaServer() (*mediaServer, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("media server listen: %w", err)
	}

	ms := &mediaServer{
		paths: make(map[string]string),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/media/", ms.handleMedia)

	ms.server = &http.Server{Handler: mux}
	ms.baseURL = "http://" + ln.Addr().String()

	go func() {
		_ = ms.server.Serve(ln)
	}()

	return ms, nil
}

// Register 注册项目媒体文件路径
func (ms *mediaServer) Register(projectID, mediaPath string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.paths[projectID] = mediaPath
}

// URL 返回项目媒体文件的访问 URL
func (ms *mediaServer) URL(projectID string) string {
	return fmt.Sprintf("%s/media/%s", ms.baseURL, projectID)
}

// Shutdown 关闭服务
func (ms *mediaServer) Shutdown() {
	_ = ms.server.Shutdown(context.Background())
}

// handleMedia 处理 /media/<projectID> 请求，支持 Range（视频 seek）
func (ms *mediaServer) handleMedia(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Path[len("/media/"):]
	if projectID == "" {
		http.Error(w, "missing project id", http.StatusBadRequest)
		return
	}

	ms.mu.RLock()
	path, ok := ms.paths[projectID]
	ms.mu.RUnlock()
	if !ok {
		http.Error(w, "project media not found", http.StatusNotFound)
		return
	}

	// 使用 http.ServeFile 自动处理 Range 请求与 MIME
	http.ServeFile(w, r, path)
}

// 防止未使用导入（当 ServeFile 不直接用 io 时保留）
var _ = io.EOF
var _ = strconv.Itoa
```

- [ ] **Step 8: 创建 app/media_server_test.go**

创建 `app/media_server_test.go`：

```go
package app

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMediaServer_ServesRegisteredFile(t *testing.T) {
	ms, err := NewMediaServer()
	require.NoError(t, err)
	defer ms.Shutdown()

	// 创建临时文件
	dir := t.TempDir()
	content := []byte("fake-media-content")
	path := filepath.Join(dir, "video.mp4")
	require.NoError(t, os.WriteFile(path, content, 0644))

	ms.Register("proj-1", path)
	url := ms.URL("proj-1")

	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, content, body)
}

func TestMediaServer_UnknownProjectReturns404(t *testing.T) {
	ms, err := NewMediaServer()
	require.NoError(t, err)
	defer ms.Shutdown()

	resp, err := http.Get(ms.URL("nope"))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
```

- [ ] **Step 9: 运行媒体服务测试**

```bash
go test ./app/... -run TestMediaServer -v -count=1
```

预期：2 个测试 PASS。

- [ ] **Step 10: 在 app/app_async.go 新增 GetMediaURL + App 持有 mediaServer**

10a. 编辑 `app/app.go`，在 `App` 结构体中增加字段（在 `binaryResolver` 之后）：

```go
type App struct {
	ctx               context.Context
	projectService    *service.ProjectService
	transcribeService *service.TranscribeService
	analyzeService    *service.AnalyzeService
	editService       *service.EditService
	exportService     *service.ExportService
	configManager     *config.ConfigManager
	binaryResolver    *adapter.BinaryResolver
	mediaServer       *mediaServer

	mu       sync.RWMutex
	projects map[string]*model.Project
}
```

10b. 修改 `app/app.go` 的 `NewApp` 签名，增加 `mediaServer *mediaServer` 参数（放在 `binaryResolver` 之后）。在 CreateProject / OpenProject 成功后注册媒体路径。

修改后的 `NewApp`：

```go
func NewApp(
	projectService *service.ProjectService,
	transcribeService *service.TranscribeService,
	analyzeService *service.AnalyzeService,
	editService *service.EditService,
	exportService *service.ExportService,
	configManager *config.ConfigManager,
	binaryResolver *adapter.BinaryResolver,
	mediaServer *mediaServer,
) *App {
	return &App{
		projectService:    projectService,
		transcribeService: transcribeService,
		analyzeService:    analyzeService,
		editService:       editService,
		exportService:     exportService,
		configManager:     configManager,
		binaryResolver:    binaryResolver,
		mediaServer:       mediaServer,
		projects:          make(map[string]*model.Project),
	}
}
```

10c. 修改 `app/app.go` 的 `CreateProject`，在 `a.projects[project.ID] = project` 之后、`a.mu.Unlock()` 之前注册媒体路径：

```go
func (a *App) CreateProject(name, mediaPath string) (*model.Project, error) {
	project, err := a.projectService.CreateProject(name, mediaPath)
	if err != nil {
		return nil, NewAppError(ErrCodeInternal, "创建项目失败", err.Error())
	}

	a.mu.Lock()
	a.projects[project.ID] = project
	if a.mediaServer != nil {
		a.mediaServer.Register(project.ID, project.Media.Path)
	}
	a.mu.Unlock()

	return project, nil
}
```

对 `OpenProject` 做相同改动（在 `a.projects[project.ID] = project` 之后增加 mediaServer.Register）。

10d. 在 `app/app_async.go` 新增 `GetMediaURL`：

```go
// GetMediaURL 获取项目媒体文件的 webview 可访问 URL
func (a *App) GetMediaURL(projectID string) (string, error) {
	if _, err := a.GetProject(projectID); err != nil {
		return "", err
	}
	if a.mediaServer == nil {
		return "", NewAppError(ErrCodeInternal, "媒体服务未启动", "")
	}
	return a.mediaServer.URL(projectID), nil
}
```

- [ ] **Step 11: 修改 main.go —— 创建并启动 mediaServer**

编辑 `main.go`：

11a. 在 import 中无需新增（app 包已导入）。

11b. 在 `appInstance := app.NewApp(...)` 之前创建 mediaServer。修改后的相关片段：

```go
	// 6. 媒体服务（本地 HTTP，供 webview 访问媒体文件）
	mediaServer, err := app.NewMediaServer()
	if err != nil {
		log.Fatalf("启动媒体服务失败: %v", err)
	}
	defer mediaServer.Shutdown()

	// 7. API Layer (App)
	appInstance := app.NewApp(
		projectService,
		transcribeService,
		analyzeService,
		editService,
		exportService,
		configManager,
		resolver,
		mediaServer,
	)
```

11c. 确认 `main.go` 已导入 `log` 包（用于 `log.Fatalf`），无需新增导入。

- [ ] **Step 12: 编译 + 全量测试**

```bash
go build ./...
go test ./internal/... ./app/... -count=1
```

确保全部通过（原有测试 + 新增 computePeaks 4 个 + mediaServer 2 个）。

- [ ] **Step 13: 提交**

```bash
git add internal/model/waveform.go internal/adapter/ffmpeg.go internal/adapter/ffmpeg_waveform_test.go internal/service/transcribe.go app/app_async.go app/app_media.go app/media_server_test.go app/app.go main.go
git commit -m "feat(media): add waveform peaks extraction and local media HTTP server"
```

---

### Task 2: 前端 —— 类型扩展 + Timeline 工具函数 + Workbench store

**Files:**
- Modify: `frontend/src/api/types.ts`
- Modify: `frontend/src/api/client.ts`
- Create: `frontend/src/lib/timeline.ts`
- Create: `frontend/src/stores/workbench.ts`

- [ ] **Step 1: 扩展 types.ts —— 新增 WaveformPeaks**

编辑 `frontend/src/api/types.ts`，在文件末尾追加：

```typescript
export interface WaveformPeaks {
  durationMs: number;
  sampleRate: number;
  buckets: number;
  mins: number[];
  maxs: number[];
}
```

- [ ] **Step 2: 扩展 client.ts —— 新增两个 API**

编辑 `frontend/src/api/client.ts`：

2a. 在顶部 import 中增加 `WaveformPeaks`：

```typescript
import type {
  Project,
  CutList,
  CutSegment,
  Transcript,
  ExportOptions,
  GlobalSettings,
  MediaFile,
  WaveformPeaks,
} from "./types";
```

2b. 在文件末尾追加：

```typescript
export async function getWaveformPeaks(projectID: string): Promise<WaveformPeaks> {
  return App.GetWaveformPeaks(projectID);
}

export async function getMediaURL(projectID: string): Promise<string> {
  return App.GetMediaURL(projectID);
}
```

2c. 同步更新 `frontend/src/api/bindings.d.ts`，在 declare module 块内新增两行：

```typescript
  export function GetWaveformPeaks(projectID: string): Promise<WaveformPeaks>;
  export function GetMediaURL(projectID: string): Promise<string>;
```

并在该文件的 import type 列表中追加 `WaveformPeaks`（从 "./types"）。

- [ ] **Step 3: 创建 lib/timeline.ts —— 时间↔像素映射工具**

创建 `frontend/src/lib/timeline.ts`：

```typescript
export interface Viewport {
  durationMs: number;
  visibleWidth: number;
  scrollMs: number;
  pxPerMs: number;
}

export const TRACK_HEIGHT = 80;
export const PLAYHEAD_HALF_WIDTH = 1;

export function fitPxPerMs(durationMs: number, visibleWidth: number): number {
  if (durationMs <= 0 || visibleWidth <= 0) return 0;
  return visibleWidth / durationMs;
}

export function buildViewport(
  durationMs: number,
  visibleWidth: number,
  zoom: number,
  scrollMs: number
): Viewport {
  const fit = fitPxPerMs(durationMs, visibleWidth);
  return {
    durationMs,
    visibleWidth,
    scrollMs,
    pxPerMs: fit * zoom,
  };
}

export function timeToX(ms: number, vp: Viewport): number {
  return (ms - vp.scrollMs) * vp.pxPerMs;
}

export function xToTime(x: number, vp: Viewport): number {
  if (vp.pxPerMs <= 0) return 0;
  return vp.scrollMs + x / vp.pxPerMs;
}

export function clampMs(ms: number, durationMs: number): number {
  return Math.max(0, Math.min(ms, durationMs));
}

export function formatTimecode(ms: number): string {
  const safe = Math.max(0, ms);
  const totalSec = safe / 1000;
  const m = Math.floor(totalSec / 60);
  const s = Math.floor(totalSec % 60);
  const cs = Math.floor((safe % 1000) / 10);
  return `${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}.${String(cs).padStart(2, "0")}`;
}
```

- [ ] **Step 4: 创建 stores/workbench.ts —— 工作台状态**

创建 `frontend/src/stores/workbench.ts`：

```typescript
import { create } from "zustand";
import type { Transcript, CutList, WaveformPeaks } from "../api/types";

export type WorkflowStage = "idle" | "transcribing" | "analyzing" | "ready" | "exporting";

interface WorkbenchStore {
  projectID: string | null;
  transcript: Transcript | null;
  cutList: CutList | null;
  peaks: WaveformPeaks | null;
  mediaURL: string | null;

  loading: boolean;
  error: string;

  stage: WorkflowStage;
  progress: number;
  stageStep: string;

  playheadMs: number;
  isPlaying: boolean;

  zoom: number;
  scrollMs: number;

  selectedSegmentId: string | null;

  setProjectID: (id: string) => void;
  setTranscript: (t: Transcript | null) => void;
  setCutList: (c: CutList | null) => void;
  setPeaks: (p: WaveformPeaks | null) => void;
  setMediaURL: (u: string | null) => void;

  setLoading: (b: boolean) => void;
  setError: (e: string) => void;

  setStage: (s: WorkflowStage) => void;
  setProgress: (p: number, step: string) => void;

  setPlayhead: (ms: number) => void;
  setPlaying: (p: boolean) => void;

  setZoom: (z: number) => void;
  setScroll: (ms: number) => void;
  zoomBy: (factor: number, durationMs: number, visibleWidth: number) => void;
  zoomFit: () => void;

  selectSegment: (id: string | null) => void;

  reset: () => void;
}

export const useWorkbenchStore = create<WorkbenchStore>((set, get) => ({
  projectID: null,
  transcript: null,
  cutList: null,
  peaks: null,
  mediaURL: null,

  loading: false,
  error: "",

  stage: "idle",
  progress: 0,
  stageStep: "",

  playheadMs: 0,
  isPlaying: false,

  zoom: 1,
  scrollMs: 0,

  selectedSegmentId: null,

  setProjectID: (id) => set({ projectID: id }),
  setTranscript: (t) => set({ transcript: t }),
  setCutList: (c) => set({ cutList: c }),
  setPeaks: (p) => set({ peaks: p }),
  setMediaURL: (u) => set({ mediaURL: u }),

  setLoading: (b) => set({ loading: b }),
  setError: (e) => set({ error: e }),

  setStage: (s) => set({ stage: s }),
  setProgress: (p, step) => set({ progress: p, stageStep: step }),

  setPlayhead: (ms) => set({ playheadMs: ms }),
  setPlaying: (p) => set({ isPlaying: p }),

  setZoom: (z) => set({ zoom: Math.max(1, Math.min(z, 50)) }),
  setScroll: (ms) => set({ scrollMs: ms }),

  zoomBy: (factor, durationMs, visibleWidth) => {
    const cur = get();
    const newZoom = Math.max(1, Math.min(cur.zoom * factor, 50));
    const playhead = cur.playheadMs;
    const fit = durationMs > 0 ? visibleWidth / durationMs : 0;
    const newPxPerMs = fit * newZoom;
    const targetScroll = playhead - visibleWidth / 2 / (newPxPerMs || 1);
    set({ zoom: newZoom, scrollMs: Math.max(0, targetScroll) });
  },
  zoomFit: () => set({ zoom: 1, scrollMs: 0 }),

  selectSegment: (id) => set({ selectedSegmentId: id }),

  reset: () =>
    set({
      projectID: null,
      transcript: null,
      cutList: null,
      peaks: null,
      mediaURL: null,
      loading: false,
      error: "",
      stage: "idle",
      progress: 0,
      stageStep: "",
      playheadMs: 0,
      isPlaying: false,
      zoom: 1,
      scrollMs: 0,
      selectedSegmentId: null,
    }),
}));
```

- [ ] **Step 5: 验证前端编译**

```bash
cd frontend && npx tsc --noEmit
```

预期：无错误。

- [ ] **Step 6: 提交**

```bash
git add frontend/src/api/types.ts frontend/src/api/client.ts frontend/src/api/bindings.d.ts frontend/src/lib/timeline.ts frontend/src/stores/workbench.ts
git commit -m "feat(frontend): add timeline utils, waveform types, and workbench store"
```

---

### Task 3: 前端 —— WaveformTrack + SubtitleTrack

**Files:**
- Create: `frontend/src/components/timeline/WaveformTrack.tsx`
- Create: `frontend/src/components/timeline/SubtitleTrack.tsx`

- [ ] **Step 1: 创建 WaveformTrack.tsx —— canvas 波形渲染**

创建 `frontend/src/components/timeline/WaveformTrack.tsx`：

```tsx
import { useEffect, useRef } from "react";
import type { WaveformPeaks } from "../../api/types";
import { timeToX, xToTime, TRACK_HEIGHT, type Viewport } from "../../lib/timeline";

interface Props {
  peaks: WaveformPeaks | null;
  viewport: Viewport;
  onSeek: (ms: number) => void;
}

export function WaveformTrack({ peaks, viewport, onSeek }: Props) {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const dpr = window.devicePixelRatio || 1;
    canvas.width = viewport.visibleWidth * dpr;
    canvas.height = TRACK_HEIGHT * dpr;
    canvas.style.width = `${viewport.visibleWidth}px`;
    canvas.style.height = `${TRACK_HEIGHT}px`;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);

    ctx.clearRect(0, 0, viewport.visibleWidth, TRACK_HEIGHT);

    if (!peaks || peaks.mins.length === 0) {
      ctx.fillStyle = "#52525b";
      ctx.font = "12px sans-serif";
      ctx.fillText("（无波形数据）", 8, TRACK_HEIGHT / 2);
      return;
    }

    const midY = TRACK_HEIGHT / 2;
    const bucketCount = peaks.mins.length;

    ctx.strokeStyle = "#a1a1aa";
    ctx.lineWidth = 1;
    ctx.beginPath();
    for (let x = 0; x < viewport.visibleWidth; x++) {
      const t = xToTime(x, viewport);
      const idx = Math.floor((t / peaks.durationMs) * bucketCount);
      if (idx < 0 || idx >= bucketCount) continue;
      const minNorm = peaks.mins[idx] / 32768;
      const maxNorm = peaks.maxs[idx] / 32768;
      const yMin = midY - maxNorm * midY * 0.9;
      const yMax = midY - minNorm * midY * 0.9;
      ctx.moveTo(x + 0.5, yMin);
      ctx.lineTo(x + 0.5, yMax);
    }
    ctx.stroke();
  }, [peaks, viewport]);

  const handleClick = (e: React.MouseEvent<HTMLCanvasElement>) => {
    const rect = canvasRef.current?.getBoundingClientRect();
    if (!rect) return;
    const x = e.clientX - rect.left;
    onSeek(Math.max(0, xToTime(x, viewport)));
  };

  return (
    <canvas
      ref={canvasRef}
      onClick={handleClick}
      className="cursor-pointer border-b border-border bg-zinc-900"
      style={{ display: "block" }}
    />
  );
}
```

- [ ] **Step 2: 创建 SubtitleTrack.tsx —— 字幕轨**

创建 `frontend/src/components/timeline/SubtitleTrack.tsx`：

```tsx
import type { Transcript, Segment } from "../../api/types";
import { timeToX, type Viewport } from "../../lib/timeline";

interface Props {
  transcript: Transcript | null;
  viewport: Viewport;
  onSeek: (ms: number) => void;
}

const SUB_HEIGHT = 56;

export function SubtitleTrack({ transcript, viewport, onSeek }: Props) {
  const segments: Segment[] = transcript?.segments ?? [];

  return (
    <div
      className="relative overflow-hidden border-b border-border bg-zinc-900"
      style={{ height: SUB_HEIGHT }}
    >
      {segments.map((seg) => {
        const x = timeToX(seg.startMs, viewport);
        const width = (seg.endMs - seg.startMs) * viewport.pxPerMs;
        if (x + width < 0 || x > viewport.visibleWidth) return null;
        return (
          <button
            key={seg.id}
            onClick={() => onSeek(seg.startMs)}
            className="absolute top-1 m-px truncate rounded bg-zinc-700 px-1.5 py-1 text-left text-xs text-zinc-200 hover:bg-zinc-600"
            style={{ left: `${Math.max(0, x)}px`, width: `${Math.max(20, width - 2)}px`, height: SUB_HEIGHT - 8 }}
            title={seg.text}
          >
            <span className="line-clamp-2">{seg.text}</span>
          </button>
        );
      })}
      {segments.length === 0 && (
        <span className="absolute left-2 top-2 text-xs text-zinc-500">（无字幕数据，请先转录）</span>
      )}
    </div>
  );
}
```

> **注意：** `line-clamp-2` 需要 `@tailwindcss/line-clamp` 插件（Tailwind 3.3+ 已内置）。若项目 Tailwind 版本低于 3.3，需在 tailwind.config 中启用插件。当前项目 Tailwind 配置已支持（Plan 4 配置）。

- [ ] **Step 3: 验证编译**

```bash
cd frontend && npx tsc --noEmit
```

- [ ] **Step 4: 提交**

```bash
git add frontend/src/components/timeline/WaveformTrack.tsx frontend/src/components/timeline/SubtitleTrack.tsx
git commit -m "feat(timeline): add waveform canvas track and subtitle track"
```

---

### Task 4: 前端 —— CutTrack + Playhead + ZoomControls + Timeline 容器

**Files:**
- Create: `frontend/src/components/timeline/CutTrack.tsx`
- Create: `frontend/src/components/timeline/Playhead.tsx`
- Create: `frontend/src/components/timeline/ZoomControls.tsx`
- Create: `frontend/src/components/timeline/Timeline.tsx`

- [ ] **Step 1: 创建 CutTrack.tsx —— 剪切轨（拖拽边界 + 右键切换 + 选中）**

创建 `frontend/src/components/timeline/CutTrack.tsx`：

```tsx
import { useRef } from "react";
import type { CutList, CutSegment } from "../../api/types";
import { timeToX, xToTime, clampMs, type Viewport } from "../../lib/timeline";

interface Props {
  cutList: CutList | null;
  viewport: Viewport;
  selectedSegmentId: string | null;
  onSelect: (id: string | null) => void;
  onToggle: (segID: string) => void;
  onDragBoundary: (seg: CutSegment, side: "start" | "end", newMs: number) => void;
  onAddManual: (startMs: number, endMs: number) => void;
}

const CUT_HEIGHT = 56;
const HANDLE_WIDTH = 8;

export function CutTrack({
  cutList,
  viewport,
  selectedSegmentId,
  onSelect,
  onToggle,
  onDragBoundary,
  onAddManual,
}: Props) {
  const dragRef = useRef<{ seg: CutSegment; side: "start" | "end" } | null>(null);
  const segments: CutSegment[] = cutList?.segments ?? [];

  const handleMouseDownBoundary = (
    e: React.MouseEvent,
    seg: CutSegment,
    side: "start" | "end"
  ) => {
    e.stopPropagation();
    e.preventDefault();
    dragRef.current = { seg, side };

    const move = (ev: MouseEvent) => {
      if (!dragRef.current) return;
      const rect = e.currentTarget
        ? (e.currentTarget as HTMLElement).getBoundingClientRect()
        : null;
      // 使用 track 容器坐标；这里用 document 计算
      const trackEl = document.getElementById("cut-track-inner");
      const r = trackEl?.getBoundingClientRect();
      if (!r) return;
      const x = ev.clientX - r.left;
      const ms = clampMs(xToTime(x, viewport), viewport.durationMs);
      onDragBoundary(dragRef.current.seg, dragRef.current.side, ms);
    };
    const up = () => {
      dragRef.current = null;
      window.removeEventListener("mousemove", move);
      window.removeEventListener("mouseup", up);
    };
    window.addEventListener("mousemove", move);
    window.addEventListener("mouseup", up);
  };

  const handleDoubleClickEmpty = (e: React.MouseEvent) => {
    if (e.target !== e.currentTarget) return;
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    const x = e.clientX - rect.left;
    const startMs = clampMs(xToTime(x, viewport), viewport.durationMs);
    const endMs = clampMs(startMs + 1000, viewport.durationMs);
    if (endMs > startMs) onAddManual(startMs, endMs);
  };

  return (
    <div
      className="relative border-b border-border bg-zinc-950"
      style={{ height: CUT_HEIGHT }}
    >
      <div
        id="cut-track-inner"
        className="relative h-full w-full"
        style={{ width: viewport.visibleWidth }}
        onDoubleClick={handleDoubleClickEmpty}
        onClick={(e) => {
          if (e.target === e.currentTarget) onSelect(null);
        }}
      >
        {segments.map((seg) => {
          const x = timeToX(seg.startMs, viewport);
          const width = (seg.endMs - seg.startMs) * viewport.pxPerMs;
          if (x + width < 0 || x > viewport.visibleWidth) return null;
          const isRemove = seg.decision === "remove";
          const isSelected = seg.id === selectedSegmentId;
          const baseColor = isRemove
            ? "bg-red-900/60 border-red-600"
            : "bg-emerald-900/60 border-emerald-600";
          const selectedRing = isSelected ? "ring-2 ring-yellow-400" : "";
          return (
            <div
              key={seg.id}
              className={`absolute top-1 flex items-center justify-center border ${baseColor} ${selectedRing} cursor-pointer text-xs`}
              style={{
                left: `${Math.max(0, x)}px`,
                width: `${Math.max(HANDLE_WIDTH, width)}px`,
                height: CUT_HEIGHT - 8,
              }}
              onClick={(e) => {
                e.stopPropagation();
                onSelect(seg.id);
              }}
              onContextMenu={(e) => {
                e.preventDefault();
                onToggle(seg.id);
              }}
              title={`${seg.decision} | ${seg.reason}${seg.note ? " | " + seg.note : ""}`}
            >
              {/* 左边界拖拽手柄 */}
              <div
                className="absolute left-0 top-0 h-full cursor-ew-resize bg-black/30"
                style={{ width: HANDLE_WIDTH }}
                onMouseDown={(e) => handleMouseDownBoundary(e, seg, "start")}
              />
              <span className="pointer-events-none truncate px-2 text-zinc-200">
                {isRemove ? "✂ 删除" : "▶ 保留"}
              </span>
              {/* 右边界拖拽手柄 */}
              <div
                className="absolute right-0 top-0 h-full cursor-ew-resize bg-black/30"
                style={{ width: HANDLE_WIDTH }}
                onMouseDown={(e) => handleMouseDownBoundary(e, seg, "end")}
              />
            </div>
          );
        })}
        {segments.length === 0 && (
          <span className="absolute left-2 top-2 text-xs text-zinc-500">
            （无剪切清单，请先分析。双击空白处可手动添加剪切段）
          </span>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: 创建 Playhead.tsx —— 播放头覆盖层**

创建 `frontend/src/components/timeline/Playhead.tsx`：

```tsx
import { timeToX, type Viewport } from "../../lib/timeline";

interface Props {
  viewport: Viewport;
  playheadMs: number;
  onSeek: (ms: number) => void;
  totalHeight: number;
}

export function Playhead({ viewport, playheadMs, onSeek, totalHeight }: Props) {
  const x = timeToX(playheadMs, viewport);

  const handleMouseDown = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    const trackEl = document.getElementById("timeline-scroll-area");
    const r = trackEl?.getBoundingClientRect();
    if (!r) return;

    const move = (ev: MouseEvent) => {
      const px = ev.clientX - r.left;
      const ms = Math.max(0, px / viewport.pxPerMs + viewport.scrollMs);
      onSeek(ms);
    };
    const up = () => {
      window.removeEventListener("mousemove", move);
      window.removeEventListener("mouseup", up);
    };
    window.addEventListener("mousemove", move);
    window.addEventListener("mouseup", up);
  };

  if (x < -10 || x > viewport.visibleWidth + 10) return null;

  return (
    <div
      className="pointer-events-none absolute top-0 z-20"
      style={{ left: `${x}px`, height: totalHeight }}
    >
      <div className="w-0.5 h-full bg-yellow-400" />
      <div
        className="pointer-events-auto absolute -left-1.5 -top-0.5 h-3 w-3.5 cursor-ew-resize rounded-t bg-yellow-400"
        onMouseDown={handleMouseDown}
      />
    </div>
  );
}
```

- [ ] **Step 3: 创建 ZoomControls.tsx —— 缩放控件**

创建 `frontend/src/components/timeline/ZoomControls.tsx`：

```tsx
import { ZoomIn, ZoomOut, Maximize2 } from "lucide-react";
import { Button } from "../ui/button";

interface Props {
  zoom: number;
  onZoomIn: () => void;
  onZoomOut: () => void;
  onFit: () => void;
}

export function ZoomControls({ zoom, onZoomIn, onZoomOut, onFit }: Props) {
  return (
    <div className="flex items-center gap-1">
      <Button variant="ghost" size="icon" onClick={onZoomOut} title="缩小">
        <ZoomOut className="h-4 w-4" />
      </Button>
      <span className="w-12 text-center text-xs text-muted-foreground">
        {zoom.toFixed(1)}x
      </span>
      <Button variant="ghost" size="icon" onClick={onZoomIn} title="放大">
        <ZoomIn className="h-4 w-4" />
      </Button>
      <Button variant="ghost" size="icon" onClick={onFit} title="适应窗口">
        <Maximize2 className="h-4 w-4" />
      </Button>
    </div>
  );
}
```

- [ ] **Step 4: 创建 Timeline.tsx —— 三轨道容器**

创建 `frontend/src/components/timeline/Timeline.tsx`：

```tsx
import { useEffect, useRef, useState } from "react";
import type { Transcript, CutList, WaveformPeaks, CutSegment } from "../../api/types";
import { buildViewport, type Viewport } from "../../lib/timeline";
import { WaveformTrack } from "./WaveformTrack";
import { SubtitleTrack } from "./SubtitleTrack";
import { CutTrack } from "./CutTrack";
import { Playhead } from "./Playhead";
import { ZoomControls } from "./ZoomControls";

interface Props {
  durationMs: number;
  transcript: Transcript | null;
  cutList: CutList | null;
  peaks: WaveformPeaks | null;
  zoom: number;
  scrollMs: number;
  playheadMs: number;
  selectedSegmentId: string | null;
  onSeek: (ms: number) => void;
  onSetScroll: (ms: number) => void;
  onZoomIn: () => void;
  onZoomOut: () => void;
  onZoomFit: () => void;
  onSelectSegment: (id: string | null) => void;
  onToggleSegment: (segID: string) => void;
  onDragBoundary: (seg: CutSegment, side: "start" | "end", newMs: number) => void;
  onAddManual: (startMs: number, endMs: number) => void;
}

export function Timeline(props: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(1000);

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const update = () => setWidth(el.clientWidth);
    update();
    const ro = new ResizeObserver(update);
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  const viewport: Viewport = buildViewport(props.durationMs, width, props.zoom, props.scrollMs);

  const totalHeight = 80 + 56 + 56; // waveform + subtitle + cut

  const handleScroll = (e: React.UIEvent<HTMLDivElement>) => {
    const el = e.currentTarget;
    if (viewport.pxPerMs <= 0) return;
    const scrollMs = el.scrollLeft / viewport.pxPerMs;
    props.onSetScroll(scrollMs);
  };

  return (
    <div className="flex flex-col border-t border-border bg-background">
      <div className="flex items-center justify-between px-3 py-1.5">
        <span className="text-xs font-medium text-muted-foreground">时间轴</span>
        <ZoomControls
          zoom={props.zoom}
          onZoomIn={props.onZoomIn}
          onZoomOut={props.onZoomOut}
          onFit={props.onZoomFit}
        />
      </div>

      <div
        ref={containerRef}
        id="timeline-scroll-area"
        className="relative overflow-x-auto overflow-y-hidden"
        onScroll={handleScroll}
      >
        <div style={{ width: `${Math.max(width, props.durationMs * viewport.pxPerMs)}px` }} className="relative">
          <WaveformTrack peaks={props.peaks} viewport={viewport} onSeek={props.onSeek} />
          <SubtitleTrack transcript={props.transcript} viewport={viewport} onSeek={props.onSeek} />
          <CutTrack
            cutList={props.cutList}
            viewport={viewport}
            selectedSegmentId={props.selectedSegmentId}
            onSelect={props.onSelectSegment}
            onToggle={props.onToggleSegment}
            onDragBoundary={props.onDragBoundary}
            onAddManual={props.onAddManual}
          />
          <Playhead
            viewport={viewport}
            playheadMs={props.playheadMs}
            onSeek={props.onSeek}
            totalHeight={totalHeight}
          />
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 5: 验证编译**

```bash
cd frontend && npx tsc --noEmit
```

- [ ] **Step 6: 提交**

```bash
git add frontend/src/components/timeline/CutTrack.tsx frontend/src/components/timeline/Playhead.tsx frontend/src/components/timeline/ZoomControls.tsx frontend/src/components/timeline/Timeline.tsx
git commit -m "feat(timeline): add cut track, playhead, zoom controls, and timeline container"
```

---

### Task 5: 前端 —— VideoPreview + AISuggestions + Workbench 组装

**Files:**
- Create: `frontend/src/components/VideoPreview.tsx`
- Create: `frontend/src/components/AISuggestions.tsx`
- Modify: `frontend/src/pages/Workbench.tsx`（重写）

- [ ] **Step 1: 创建 VideoPreview.tsx —— 视频预览 + 播放控制**

创建 `frontend/src/components/VideoPreview.tsx`：

```tsx
import { useEffect, useRef } from "react";
import { Play, Pause, SkipBack, SkipForward } from "lucide-react";
import { Button } from "./ui/button";
import { formatTimecode } from "../lib/timeline";

interface Props {
  src: string | null;
  isPlaying: boolean;
  playheadMs: number;
  loopSegment: { startMs: number; endMs: number } | null;
  onTimeUpdate: (ms: number) => void;
  onTogglePlay: () => void;
  onSeek: (ms: number) => void;
  durationMs: number;
}

export function VideoPreview({
  src,
  isPlaying,
  playheadMs,
  loopSegment,
  onTimeUpdate,
  onTogglePlay,
  onSeek,
  durationMs,
}: Props) {
  const videoRef = useRef<HTMLVideoElement>(null);

  useEffect(() => {
    const v = videoRef.current;
    if (!v) return;
    if (isPlaying) {
      v.play().catch(() => {});
    } else {
      v.pause();
    }
  }, [isPlaying]);

  useEffect(() => {
    const v = videoRef.current;
    if (!v) return;
    const delta = Math.abs(v.currentTime * 1000 - playheadMs);
    if (delta > 350) {
      v.currentTime = playheadMs / 1000;
    }
  }, [playheadMs]);

  const handleTimeUpdate = () => {
    const v = videoRef.current;
    if (!v) return;
    const ms = v.currentTime * 1000;
    onTimeUpdate(ms);
    if (loopSegment && ms >= loopSegment.endMs) {
      v.currentTime = loopSegment.startMs / 1000;
    }
  };

  return (
    <div className="flex flex-1 flex-col items-center justify-center bg-zinc-950 p-4">
      <div className="relative w-full max-w-3xl">
        {src ? (
          <video
            ref={videoRef}
            src={src}
            onTimeUpdate={handleTimeUpdate}
            onLoadedMetadata={(e) => e.currentTarget.volume = 1}
            className="w-full rounded-lg"
            controls={false}
          />
        ) : (
          <div className="flex aspect-video w-full items-center justify-center rounded-lg border border-border bg-zinc-900 text-muted-foreground">
            （无媒体）
          </div>
        )}
      </div>

      <div className="mt-3 flex w-full max-w-3xl items-center justify-center gap-3">
        <Button variant="ghost" size="icon" onClick={() => onSeek(Math.max(0, playheadMs - 5000))} title="后退 5 秒">
          <SkipBack className="h-4 w-4" />
        </Button>
        <Button variant="default" size="icon" onClick={onTogglePlay} title={isPlaying ? "暂停" : "播放"}>
          {isPlaying ? <Pause className="h-4 w-4" /> : <Play className="h-4 w-4" />}
        </Button>
        <Button variant="ghost" size="icon" onClick={() => onSeek(Math.min(durationMs, playheadMs + 5000))} title="前进 5 秒">
          <SkipForward className="h-4 w-4" />
        </Button>
        <span className="ml-2 font-mono text-sm text-muted-foreground">
          {formatTimecode(playheadMs)} / {formatTimecode(durationMs)}
        </span>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: 创建 AISuggestions.tsx —— 右侧 AI 建议面板**

创建 `frontend/src/components/AISuggestions.tsx`：

```tsx
import { Check, X, RefreshCw } from "lucide-react";
import type { CutList, CutSegment } from "../api/types";
import { Button } from "./ui/button";

interface Props {
  cutList: CutList | null;
  selectedSegmentId: string | null;
  onSelect: (id: string) => void;
  onAccept: (segID: string) => void;
  onReject: (segID: string) => void;
  loading: boolean;
}

const reasonLabel: Record<string, string> = {
  filler: "语气词",
  silence: "停顿/沉默",
  dup_or_error: "重复/口误",
  manual: "手动",
};

export function AISuggestions({
  cutList,
  selectedSegmentId,
  onSelect,
  onAccept,
  onReject,
  loading,
}: Props) {
  const removeSegs: CutSegment[] =
    cutList?.segments.filter((s) => s.decision === "remove" && s.source === "ai") ?? [];

  return (
    <aside className="flex w-64 flex-col border-l border-border bg-zinc-900">
      <div className="flex items-center justify-between border-b border-border px-3 py-2">
        <span className="text-sm font-medium">AI 建议</span>
        {loading && <RefreshCw className="h-3.5 w-3.5 animate-spin text-muted-foreground" />}
      </div>

      <div className="flex-1 overflow-auto">
        {removeSegs.length === 0 && (
          <p className="px-3 py-4 text-xs text-muted-foreground">
            暂无 AI 建议的删除段。完成分析后将在此显示。
          </p>
        )}
        {removeSegs.map((seg) => {
          const isSelected = seg.id === selectedSegmentId;
          const dur = ((seg.endMs - seg.startMs) / 1000).toFixed(2);
          return (
            <div
              key={seg.id}
              className={`cursor-pointer border-b border-border px-3 py-2 text-xs hover:bg-zinc-800 ${
                isSelected ? "bg-zinc-800 ring-1 ring-yellow-400" : ""
              }`}
              onClick={() => onSelect(seg.id)}
            >
              <div className="flex items-center justify-between">
                <span className="rounded bg-red-900/60 px-1.5 py-0.5 text-[10px] text-red-200">
                  {reasonLabel[seg.reason] ?? seg.reason} · {dur}s
                </span>
                <span className="text-muted-foreground">
                  {(seg.confidence * 100).toFixed(0)}%
                </span>
              </div>
              {seg.note && <p className="mt-1 line-clamp-2 text-muted-foreground">{seg.note}</p>}
              <div className="mt-1.5 flex gap-1">
                <Button size="sm" variant="outline" onClick={(e) => { e.stopPropagation(); onAccept(seg.id); }}>
                  <Check className="mr-1 h-3 w-3" /> 保留此段
                </Button>
                <Button size="sm" variant="ghost" onClick={(e) => { e.stopPropagation(); onReject(seg.id); }} title="确认删除">
                  <X className="h-3 w-3" />
                </Button>
              </div>
            </div>
          );
        })}
      </div>
    </aside>
  );
}
```

> **语义说明：** "AI 建议"面板列出被 AI 标记为 `remove` 的段。"保留此段"= 把该段切回 `keep`（接受建议的反面：用户不同意删）。"确认删除"按钮用 X 图标表示维持删除状态（已默认）。按钮命名遵循"用户在面板里对建议做决策"的直觉。`onAccept(segID)` 在 Workbench 中实现为 toggle 到 keep。

- [ ] **Step 3: 重写 Workbench.tsx —— 完整工作台**

编辑 `frontend/src/pages/Workbench.tsx`，替换占位内容为完整实现：

```tsx
import { useEffect, useCallback } from "react";
import { useParams } from "react-router-dom";
import { Mic, BrainCircuit, Download, Loader2 } from "lucide-react";
import { Button } from "../components/ui/button";
import { Timeline } from "../components/timeline/Timeline";
import { VideoPreview } from "../components/VideoPreview";
import { AISuggestions } from "../components/AISuggestions";
import { useWorkbenchStore, type WorkflowStage } from "../stores/workbench";
import { useProjectStore } from "../stores/project";
import {
  getProject,
  getTranscript,
  getCutList,
  getWaveformPeaks,
  getMediaURL,
  startTranscribe,
  startAnalyze,
  startExport,
  addCutSegment,
  updateCutSegment,
  removeCutSegment,
} from "../api/client";
import {
  onProgress,
  onTranscriptReady,
  onCutListReady,
} from "../api/events";
import type { CutSegment, ExportOptions } from "../api/types";

const stageButtonConfig: Record<
  string,
  { label: string; icon: typeof Mic; stage: WorkflowStage }
> = {
  transcribe: { label: "转录", icon: Mic, stage: "transcribing" },
  analyze: { label: "AI 分析", icon: BrainCircuit, stage: "analyzing" },
  export: { label: "导出", icon: Download, stage: "exporting" },
};

export function Workbench() {
  const { id } = useParams<{ id: string }>();
  const wb = useWorkbenchStore();
  const currentProject = useProjectStore((s) => s.currentProject);
  const setCurrentProject = useProjectStore((s) => s.setCurrentProject);

  const loadAll = useCallback(async (projectID: string) => {
    wb.setLoading(true);
    wb.setError("");
    try {
      const project = currentProject?.id === projectID ? currentProject : await getProject(projectID);
      setCurrentProject(project);
      wb.setProjectID(projectID);

      const [url] = await Promise.all([getMediaURL(projectID)]);
      wb.setMediaURL(url);

      // 尝试加载已有数据（失败说明尚未生成，忽略）
      try {
        const t = await getTranscript(projectID);
        wb.setTranscript(t);
      } catch {
        wb.setTranscript(null);
      }
      try {
        const c = await getCutList(projectID);
        wb.setCutList(c);
        wb.setStage("ready");
      } catch {
        wb.setCutList(null);
      }
      try {
        const p = await getWaveformPeaks(projectID);
        wb.setPeaks(p);
      } catch {
        wb.setPeaks(null);
      }
    } catch (e) {
      wb.setError("加载数据失败: " + String(e));
    } finally {
      wb.setLoading(false);
    }
  }, [currentProject, setCurrentProject, wb]);

  useEffect(() => {
    if (!id) return;
    loadAll(id);
    return () => wb.reset();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  // 订阅事件
  useEffect(() => {
    const off1 = onProgress((ev) => {
      wb.setProgress(ev.progress, ev.step);
      if (ev.stage === "transcribe" && ev.status === "running") wb.setStage("transcribing");
      if (ev.stage === "analyze" && ev.status === "running") wb.setStage("analyzing");
      if (ev.stage === "export" && ev.status === "running") wb.setStage("exporting");
      if (ev.status === "error") wb.setError(ev.error ?? "任务失败");
    });
    const off2 = onTranscriptReady((t) => {
      wb.setTranscript(t);
      if (id) getWaveformPeaks(id).then(wb.setPeaks).catch(() => {});
    });
    const off3 = onCutListReady((c) => {
      wb.setCutList(c);
      wb.setStage("ready");
    });
    return () => {
      off1();
      off2();
      off3();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  const durationMs = currentProject?.media.durationMs ?? 0;

  const handleSeek = (ms: number) => {
    wb.setPlayhead(Math.max(0, Math.min(ms, durationMs)));
  };

  const handleTranscribe = async () => {
    if (!id) return;
    wb.setStage("transcribing");
    wb.setProgress(0, "启动转录");
    try {
      await startTranscribe(id);
    } catch (e) {
      wb.setError("启动转录失败: " + String(e));
      wb.setStage("idle");
    }
  };

  const handleAnalyze = async () => {
    if (!id) return;
    wb.setStage("analyzing");
    wb.setProgress(0, "启动分析");
    try {
      await startAnalyze(id);
    } catch (e) {
      wb.setError("启动分析失败: " + String(e));
      wb.setStage("ready");
    }
  };

  const handleExport = async () => {
    if (!id) return;
    wb.setStage("exporting");
    wb.setProgress(0, "启动导出");
    const opts: ExportOptions = {
      mode: "lossless",
      includeSubtitle: false,
      outputPath: `${currentProject?.workDir ?? "."}/export.mp4`,
    };
    try {
      await startExport(id, opts);
    } catch (e) {
      wb.setError("启动导出失败: " + String(e));
      wb.setStage("ready");
    }
  };

  const handleToggleSegment = async (segID: string) => {
    if (!id || !wb.cutList) return;
    const seg = wb.cutList.segments.find((s) => s.id === segID);
    if (!seg) return;
    const updated: CutSegment = {
      ...seg,
      decision: seg.decision === "keep" ? "remove" : "keep",
    };
    try {
      await updateCutSegment(id, updated);
      wb.setCutList({ ...wb.cutList, segments: wb.cutList.segments.map((s) => (s.id === segID ? updated : s)) });
    } catch (e) {
      wb.setError("切换失败: " + String(e));
    }
  };

  const handleAccept = async (segID: string) => {
    // 接受建议的反面：用户选择保留该段（切回 keep）
    if (!id || !wb.cutList) return;
    const seg = wb.cutList.segments.find((s) => s.id === segID);
    if (!seg) return;
    const updated: CutSegment = { ...seg, decision: "keep" };
    try {
      await updateCutSegment(id, updated);
      wb.setCutList({ ...wb.cutList, segments: wb.cutList.segments.map((s) => (s.id === segID ? updated : s)) });
    } catch (e) {
      wb.setError("操作失败: " + String(e));
    }
  };

  const handleDragBoundary = async (seg: CutSegment, side: "start" | "end", newMs: number) => {
    if (!id || !wb.cutList) return;
    const updated: CutSegment =
      side === "start"
        ? { ...seg, startMs: Math.min(newMs, seg.endMs - 50) }
        : { ...seg, endMs: Math.max(newMs, seg.startMs + 50) };
    try {
      await updateCutSegment(id, updated);
      wb.setCutList({ ...wb.cutList, segments: wb.cutList.segments.map((s) => (s.id === seg.id ? updated : s)) });
    } catch (e) {
      wb.setError("调整失败: " + String(e));
    }
  };

  const handleAddManual = async (startMs: number, endMs: number) => {
    if (!id) return;
    const newSeg: CutSegment = {
      id: `manual-${Date.now()}`,
      startMs,
      endMs,
      decision: "remove",
      reason: "manual",
      source: "manual",
      confidence: 1,
      note: "",
    };
    try {
      await addCutSegment(id, newSeg);
      const fresh = await getCutList(id);
      wb.setCutList(fresh);
    } catch (e) {
      wb.setError("添加失败: " + String(e));
    }
  };

  const loopSegment = wb.selectedSegmentId && wb.cutList
    ? (() => {
        const seg = wb.cutList.segments.find((s) => s.id === wb.selectedSegmentId);
        return seg ? { startMs: seg.startMs, endMs: seg.endMs } : null;
      })()
    : null;

  const busy = wb.stage === "transcribing" || wb.stage === "analyzing" || wb.stage === "exporting";

  return (
    <div className="flex h-full flex-col">
      {/* TopBar */}
      <div className="flex items-center justify-between border-b border-border bg-zinc-900 px-4 py-2">
        <div className="flex items-center gap-3">
          <h1 className="text-sm font-semibold">{currentProject?.name ?? "工作台"}</h1>
          {busy && (
            <span className="flex items-center gap-1 text-xs text-muted-foreground">
              <Loader2 className="h-3 w-3 animate-spin" />
              {wb.stage === "transcribing" ? "转录中" : wb.stage === "analyzing" ? "分析中" : "导出中"}
              {wb.progress > 0 ? ` ${(wb.progress * 100).toFixed(0)}%` : ""}
            </span>
          )}
        </div>
        <div className="flex items-center gap-2">
          <Button size="sm" variant="outline" onClick={handleTranscribe} disabled={busy}>
            <Mic className="mr-1.5 h-3.5 w-3.5" /> 转录
          </Button>
          <Button size="sm" variant="outline" onClick={handleAnalyze} disabled={busy || !wb.transcript}>
            <BrainCircuit className="mr-1.5 h-3.5 w-3.5" /> 分析
          </Button>
          <Button size="sm" variant="default" onClick={handleExport} disabled={busy || !wb.cutList}>
            <Download className="mr-1.5 h-3.5 w-3.5" /> 导出
          </Button>
        </div>
      </div>

      {wb.error && (
        <div className="bg-red-950/60 px-4 py-1.5 text-xs text-red-200">
          {wb.error}
        </div>
      )}

      {/* 主区域 */}
      <div className="flex flex-1 overflow-hidden">
        <VideoPreview
          src={wb.mediaURL}
          isPlaying={wb.isPlaying}
          playheadMs={wb.playheadMs}
          loopSegment={loopSegment}
          onTimeUpdate={wb.setPlayhead}
          onTogglePlay={() => wb.setPlaying(!wb.isPlaying)}
          onSeek={handleSeek}
          durationMs={durationMs}
        />
        <AISuggestions
          cutList={wb.cutList}
          selectedSegmentId={wb.selectedSegmentId}
          onSelect={wb.selectSegment}
          onAccept={handleAccept}
          onReject={wb.selectSegment}
          loading={wb.stage === "analyzing"}
        />
      </div>

      {/* Timeline */}
      <Timeline
        durationMs={durationMs}
        transcript={wb.transcript}
        cutList={wb.cutList}
        peaks={wb.peaks}
        zoom={wb.zoom}
        scrollMs={wb.scrollMs}
        playheadMs={wb.playheadMs}
        selectedSegmentId={wb.selectedSegmentId}
        onSeek={handleSeek}
        onSetScroll={wb.setScroll}
        onZoomIn={() => wb.zoomBy(1.5, durationMs, 1000)}
        onZoomOut={() => wb.zoomBy(1 / 1.5, durationMs, 1000)}
        onZoomFit={wb.zoomFit}
        onSelectSegment={wb.selectSegment}
        onToggleSegment={handleToggleSegment}
        onDragBoundary={handleDragBoundary}
        onAddManual={handleAddManual}
      />
    </div>
  );
}
```

> **注意：** `onReject` 暂复用 `wb.selectSegment`（仅选中，维持删除状态）。`getCutList` / `addCutSegment` / `updateCutSegment` / `removeCutSegment` / `startTranscribe` / `startAnalyze` / `startExport` / `getTranscript` / `getProject` 应已在 Plan 4 的 client.ts 中定义。若某些导出名与 Plan 4 不一致，执行时以 client.ts 实际导出为准对齐。

- [ ] **Step 4: 验证编译**

```bash
cd frontend && npx tsc --noEmit
```

预期：无错误。若 `Project.workDir` 字段报错，确认 types.ts 已含该字段（Plan 4 已定义，见上文验证）。

- [ ] **Step 5: 提交**

```bash
git add frontend/src/components/VideoPreview.tsx frontend/src/components/AISuggestions.tsx frontend/src/pages/Workbench.tsx
git commit -m "feat(workbench): integrate video preview, AI suggestions panel, and full timeline workflow"
```

---

### Task 6: 最终验证

**Files:** 无（仅运行验证命令）

- [ ] **Step 1: 后端全量编译 + 测试**

```bash
go build ./...
go test ./internal/... ./app/... -count=1 -v 2>&1 | tail -n 40
```

预期：
- `go build ./...` 成功（忽略预存的 ios 相关错误，与本任务无关）
- 新增测试 `TestComputePeaks_*`（4 个）+ `TestMediaServer_*`（2 个）全部 PASS
- 原有 config / service / pipeline 测试仍全部 PASS

- [ ] **Step 2: 前端类型检查**

```bash
cd frontend && npx tsc --noEmit
```

预期：零错误。

- [ ] **Step 3: 前端构建（可选，确认产物可打包）**

```bash
cd frontend && npm run build
```

预期：构建成功，`dist/` 生成。若 Tailwind 插件 `line-clamp` 报错，确认 Tailwind 版本 ≥ 3.3（已内置）。

- [ ] **Step 4: 手动冒烟检查清单（执行时人工核对）**

构建完成后，运行 `wails3 dev`，在应用中确认：
- [ ] 新建项目 → 跳转 Workbench，视频可播放
- [ ] 点击"转录"→ 进度推进 → 波形出现
- [ ] 点击"分析"→ AI 建议面板出现删除段
- [ ] 点击波形/字幕段 → 视频跳转，播放头同步
- [ ] 拖拽剪切段边界 → 时间更新
- [ ] 右键剪切段 → keep/remove 切换
- [ ] 双击剪切轨空白 → 新增手动段
- [ ] 选中段 → 视频 loop 播放该段
- [ ] 缩放 [+] [-] [适应] → 波形/段宽度变化
- [ ] 点击"导出"→ 进度推进（实际导出依赖 Plan 6/7，MVP 验证调用链通即可）

---

## Self-Review（编写者自查，非执行步骤）

**1. Spec 覆盖检查（对照设计文档 Timeline 相关需求）：**

| 设计文档需求 | 对应任务 | 状态 |
|---|---|---|
| 波形用 `<canvas>` 自绘（数据来自 FFmpegAdapter） | Task 1 后端峰值提取 + Task 3 WaveformTrack | ✅ |
| SubtitleTrack 字幕轨 | Task 3 SubtitleTrack | ✅ |
| CutTrack 剪切轨（keep/remove 可视化） | Task 4 CutTrack | ✅ |
| 点击/拖拽波形或字幕 → 视频跳转 | Task 3 onSeek + Task 5 handleSeek | ✅ |
| 拖拽边界微调时间 | Task 4 handleMouseDownBoundary | ✅ |
| Ctrl + 滚轮缩放（以播放头为中心） | Task 2 zoomBy + Task 4 ZoomControls（按钮版） | ⚠️ 按钮版，滚轮交互留待后续 |
| Playhead 同步 video.currentTime | Task 5 VideoPreview 双向同步 | ✅ |
| 右键切换 keep/remove | Task 4 onContextMenu | ✅ |
| AI 建议逐条接受/拒绝 | Task 5 AISuggestions | ✅ |
| 进度回传显示 | Task 5 TopBar 进度条 + onProgress 订阅 | ✅ |

**说明：** 设计文档提到的"Ctrl + 滚轮缩放"本 Plan 用按钮缩放（ZoomControls）实现等效能力，以降低首次实现的鼠标事件复杂度。滚轮缩放可在后续迭代加入 Timeline 的 onWheel handler（zoomBy 调用已就绪）。核心交互（缩放能力本身）已覆盖。

**2. 占位符扫描：**
- 无 "TBD" / "TODO" / "实现略" / "类似上面"。
- 所有代码步骤均含完整可运行代码。
- getProject 等依赖已在 Plan 4 client.ts 验证存在（见上文 Step 3 验证）。

**3. 类型一致性：**
- `WaveformPeaks`：Go `model/waveform.go`（DurationMs/SampleRate/Buckets/Mins/Maxs）↔ TS `types.ts`（同名同序）✅
- `CutSegment.source` 类型 `CutSource = "ai" | "manual"`，AISuggestions 用 `seg.source === "ai"` ✅
- `computePeaks` 在 Task 1 Step 2c 定义，Step 3 测试引用同名 ✅
- `buildViewport` 返回 `Viewport`（含 pxPerMs），WaveformTrack/CutTrack/Playhead 均引用 `Viewport.pxPerMs` ✅
- `useWorkbenchStore.zoomBy(factor, durationMs, visibleWidth)` 签名与 Workbench 调用 `wb.zoomBy(1.5, durationMs, 1000)` 一致 ✅
- `NewMediaServer`（Step 7 导出构造函数）被 main.go 与 media_server_test.go 一致引用 ✅

**4. 风险点（执行时关注）：**
- **媒体 HTTP 服务端口：** 随机端口每次启动不同，GetMediaURL 动态返回，无硬编码，安全。
- **Range 请求：** `http.ServeFile` 原生支持，视频 seek 可工作。
- **波形提取耗时：** 30s 视频 ~1-2s（8000Hz PCM），首次提取后缓存于 `peaks sync.Map`。
- **大视频内存：** PCM 全量读入计算峰值，10 分钟视频约 4.8MB PCM（8000Hz×600s×2字节），可接受。

---

## Execution Handoff

Plan 已完成并保存到 `docs/superpowers/plans/2026-07-02-smart-cut-plan-5-timeline.md`。

两种执行方式：

**1. Subagent-Driven（推荐）** — 每个 Task 派发独立 subagent，任务间两阶段审查（spec 审查 + 代码质量审查），快速迭代。适合本 Plan（6 个 Task，后端/前端分层清晰）。

**2. Inline Execution** — 在当前会话内按 Task 顺序执行，带检查点。

**建议：** Task 1（后端）与 Task 2（前端基础）可并行派发；Task 3-5（前端组件）建议串行（共享 timeline.ts/store 类型，避免冲突）；Task 6 最后统一验证。

请选择执行方式。
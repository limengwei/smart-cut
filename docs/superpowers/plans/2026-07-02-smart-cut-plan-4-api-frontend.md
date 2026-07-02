# Smart-Cut Plan 4: API Layer + 前端基础

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现 API Layer（App 门面 + Wails3 Service 注册 + 前后端通信）、Config 层（全局设置持久化）、前端基础（路由、布局、API client、设置页、新建项目页）。

**Architecture:** App 作为 API 门面，持有所有 Service 引用，负责参数转换与状态管理（当前项目/转录结果）。通过 Wails3 `application.NewService` 注册，前端通过自动生成的 bindings 调用。EventBus 通过 `app.Event.Emit` 推送事件到前端。

**Tech Stack:** Go (Wails3 v3.0.0-alpha.96), React 18, TypeScript, react-router-dom v6, Shadcn/ui, Tailwind CSS, zustand, @wailsio/runtime

**Key Wails3 API（经实证确认）：**
- Service 注册：`application.NewService(&app.App{})` — 传指针
- 事件发射：`app.Event.Emit("name", data)` — 单数 `Event`，无 ctx 参数
- 全局获取 app：`application.Get()` 返回 `*application.App`
- 前端事件订阅：`import { Events } from "@wailsio/runtime"; Events.On("name", cb)`
- 前端调用 binding：自动生成到 `frontend/bindings/<module>/<pkg>/`

---

## File Structure

```
smart-cut/
├── app/                          # API Layer（新建）
│   ├── app.go                    # App 门面 + 项目管理 + CutList 编辑
│   ├── app_async.go              # 异步任务 + 查询
│   ├── app_settings.go           # 设置 + ProbeBinary
│   └── errors.go                 # AppError 统一错误类型
├── internal/
│   ├── config/
│   │   ├── config.go             # ConfigManager（GlobalSettings 持久化）
│   │   └── config_test.go        # ConfigManager 测试
│   ├── eventbus/
│   │   └── eventbus.go           # 修正注释（app.Events → app.Event）
│   └── service/
│       └── transcribe.go         # 增加 GetTranscript + transcript 存储
├── main.go                       # 装配所有 Service + EventBus 连接
├── frontend/
│   └── src/
│       ├── api/
│       │   ├── client.ts         # Wails binding 调用封装
│       │   ├── events.ts         # 事件订阅封装
│       │   └── types.ts          # TypeScript 类型定义（镜像 Go model）
│       ├── components/
│       │   └── ui/               # Shadcn 组件
│       │       ├── button.tsx
│       │       ├── input.tsx
│       │       ├── label.tsx
│       │       ├── card.tsx
│       │       ├── select.tsx
│       │       ├── switch.tsx
│       │       ├── toast.tsx
│       │       ├── toaster.tsx
│       │       └── use-toast.ts
│       ├── layouts/
│       │   └── AppLayout.tsx     # 主布局（侧边栏 + 内容区）
│       ├── pages/
│       │   ├── NewProject.tsx    # 新建/导入项目
│       │   ├── Settings.tsx      # 全局设置
│       │   └── Workbench.tsx     # 工作台占位（Plan 5 实现）
│       ├── stores/
│       │   ├── settings.ts       # 设置 zustand store
│       │   └── project.ts        # 当前项目 zustand store
│       ├── lib/
│       │   └── utils.ts          # cn() 已存在
│       ├── App.tsx               # 路由配置
│       └── main.tsx              # 入口（已存在，可能微调）
```

---

### Task 1: Config 层 — GlobalSettings 持久化

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: 编写 config.go**

创建 `internal/config/config.go`：

```go
package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"smart-cut/internal/model"
)

// ConfigManager 管理全局设置的加载与保存
// 配置文件位置：~/.smart-cut/config.json
type ConfigManager struct {
	configPath string
}

// NewConfigManager 创建 ConfigManager
// configDir 为空时默认使用 ~/.smart-cut
func NewConfigManager(configDir string) *ConfigManager {
	if configDir == "" {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".smart-cut")
	}
	return &ConfigManager{
		configPath: filepath.Join(configDir, "config.json"),
	}
}

// Load 加载全局设置，文件不存在时返回默认值
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

// Save 保存全局设置
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

// defaultSettings 返回默认全局设置
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
```

- [ ] **Step 2: 编写 config_test.go**

创建 `internal/config/config_test.go`：

```go
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
			"ffmpeg":       "/usr/local/bin/ffmpeg",
			"whisper-cli":  "/opt/whisper/whisper-cli",
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

	// 验证文件确实被创建
	_, err = os.Stat(filepath.Join(dir, "config.json"))
	require.NoError(t, err)

	loaded, err := mgr.Load()
	require.NoError(t, err)
	assert.Equal(t, original.Theme, loaded.Theme)
	assert.Equal(t, original.DefaultLLM.APIKey, loaded.DefaultLLM.APIKey)
	assert.Equal(t, original.Binaries["ffmpeg"], loaded.Binaries["ffmpeg"])
	assert.Equal(t, original.WhisperModelDir, loaded.WhisperModelDir)
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
	// 不应 panic，且 configPath 应包含 .smart-cut
	assert.Contains(t, mgr.configPath, ".smart-cut")
}
```

- [ ] **Step 3: 运行测试**

```bash
go test ./internal/config/... -v -count=1
```

确保全部通过。

---

### Task 2: AppError + 修正 EventBus 注释 + TranscribeService.GetTranscript

**Files:**
- Create: `app/errors.go`
- Edit: `internal/eventbus/eventbus.go`（仅修正注释）
- Edit: `internal/service/transcribe.go`（增加 transcript 存储与 GetTranscript 方法）
- Edit: `internal/service/transcribe.go`（可选：增加 transcript 测试）

- [ ] **Step 1: 创建 app/errors.go**

创建 `app/errors.go`：

```go
package app

import "fmt"

// ErrCode 错误码
type ErrCode string

const (
	ErrCodeEnv        ErrCode = "env"        // 环境错误（二进制缺失等）
	ErrCodeParam      ErrCode = "param"      // 参数错误
	ErrCodeTranscribe ErrCode = "transcribe" // 转录错误
	ErrCodeLLM        ErrCode = "llm"        // LLM 错误
	ErrCodeExport     ErrCode = "export"     // 导出错误
	ErrCodeCanceled   ErrCode = "canceled"   // 用户取消
	ErrCodeInternal   ErrCode = "internal"   // 内部错误
)

// AppError 统一错误类型（前端可解析 code 做不同处理）
type AppError struct {
	Code    ErrCode `json:"code"`
	Message string  `json:"message"` // 用户友好描述
	Detail  string  `json:"detail"`   // 原始错误（折叠显示）
}

func (e *AppError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// NewAppError 创建 AppError
func NewAppError(code ErrCode, message string, detail string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Detail:  detail,
	}
}

// WrapError 将普通 error 包装为 AppError
func WrapError(code ErrCode, err error) *AppError {
	if err == nil {
		return nil
	}
	return &AppError{
		Code:    code,
		Message: err.Error(),
		Detail:  err.Error(),
	}
}
```

- [ ] **Step 2: 修正 eventbus.go 注释**

编辑 `internal/eventbus/eventbus.go`，将注释中的 `app.Events.Emit` 改为 `app.Event.Emit`（Wails3 v3 正确 API 是单数 `Event`，且不需要 ctx 参数）。

具体修改：
- 第 10 行注释：`// 在 Wails 环境下，EmitFunc 调用 app.Events.Emit` → `// 在 Wails 环境下，EmitFunc 调用 app.Event.Emit`
- 第 18 行注释：`// emitFunc: 实际推送函数（Wails 里是 app.Events.Emit 的包装）` → `// emitFunc: 实际推送函数（Wails 里是 app.Event.Emit 的包装）`

- [ ] **Step 3: 修改 TranscribeService — 增加 transcript 存储**

编辑 `internal/service/transcribe.go`：

1. 在 import 中增加 `"sync"`
2. 在 `TranscribeService` 结构体中增加 `transcripts sync.Map` 字段（projectID → *model.Transcript）
3. 在 `StartTranscribe` 的 goroutine 中，step.Run 成功后存储 transcript：`s.transcripts.Store(project.ID, ctx.Transcript)`
4. 新增 `GetTranscript(projectID string)` 方法

修改后的完整文件：

```go
package service

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"smart-cut/internal/adapter"
	"smart-cut/internal/eventbus"
	"smart-cut/internal/model"
	"smart-cut/internal/pipeline"
)

// TranscribeService 编排转录流程
type TranscribeService struct {
	whisper     adapter.WhisperAdapter
	ffmpeg      adapter.FFmpegAdapter
	bus         *eventbus.EventBus
	editSvc     *EditService
	transcripts sync.Map // projectID → *model.Transcript
}

// NewTranscribeService 创建 TranscribeService
func NewTranscribeService(whisper adapter.WhisperAdapter, ffmpeg adapter.FFmpegAdapter, bus *eventbus.EventBus, editSvc *EditService) *TranscribeService {
	return &TranscribeService{whisper: whisper, ffmpeg: ffmpeg, bus: bus, editSvc: editSvc}
}

// StartTranscribe 启动转录任务（异步）
func (s *TranscribeService) StartTranscribe(project *model.Project, modelPath string) string {
	taskID := fmt.Sprintf("transcribe-%s", project.ID)

	go func() {
		ctx := &pipeline.Context{
			Project: project,
			Cancel:  context.Background(),
		}

		reporter := pipeline.NewEventBusReporter(s.bus, taskID)

		step := pipeline.NewTranscribeStep(s.whisper, s.ffmpeg, adapter.WhisperOptions{
			Language:  "zh",
			ModelPath: modelPath,
			WordLevel: false,
		})

		if err := step.Run(ctx, reporter); err != nil {
			s.bus.EmitProgress(model.ProgressEvent{
				TaskID: taskID,
				Stage:  "transcribe",
				Status: model.TaskError,
				Error:  err.Error(),
			})
			return
		}

		// 存储 transcript 供后续 GetTranscript 查询
		s.transcripts.Store(project.ID, ctx.Transcript)

		s.bus.Emit("transcript:ready", ctx.Transcript)
	}()

	return taskID
}

// GetTranscript 获取项目的转录结果
func (s *TranscribeService) GetTranscript(projectID string) (*model.Transcript, error) {
	val, ok := s.transcripts.Load(projectID)
	if !ok {
		return nil, fmt.Errorf("transcript not found for project %s", projectID)
	}
	return val.(*model.Transcript), nil
}

// ProbeMedia 探测媒体文件信息（同步）
func (s *TranscribeService) ProbeMedia(ctx context.Context, path string) (*model.MediaFile, error) {
	return s.ffmpeg.Probe(ctx, path)
}

// ExtractWaveform 提取波形图（同步）
func (s *TranscribeService) ExtractWaveform(ctx context.Context, project *model.Project) error {
	waveformPath := filepath.Join(project.WorkDir, "waveform.png")
	return s.ffmpeg.ExtractWaveform(ctx, project.Media.Path, waveformPath)
}
```

- [ ] **Step 4: 运行测试确保无回归**

```bash
go test ./internal/... -count=1
go build ./app/...
```

---

### Task 3: App 门面 — 项目管理 + CutList 编辑

**Files:**
- Create: `app/app.go`

- [ ] **Step 1: 创建 app/app.go**

创建 `app/app.go`，包含 App 结构体定义、构造函数、项目管理方法、CutList 编辑方法。

```go
package app

import (
	"context"
	"fmt"
	"sync"

	"smart-cut/internal/adapter"
	"smart-cut/internal/config"
	"smart-cut/internal/model"
	"smart-cut/internal/service"
)

// App 是 API 层门面，注册为 Wails3 Service
// 前端通过自动生成的 bindings 调用其导出方法
type App struct {
	ctx               context.Context
	projectService    *service.ProjectService
	transcribeService *service.TranscribeService
	analyzeService    *service.AnalyzeService
	editService       *service.EditService
	exportService     *service.ExportService
	configManager     *config.ConfigManager
	binaryResolver    *adapter.BinaryResolver

	mu       sync.RWMutex
	projects map[string]*model.Project // 已加载的项目（projectID → Project）
}

// NewApp 创建 App
func NewApp(
	projectService *service.ProjectService,
	transcribeService *service.TranscribeService,
	analyzeService *service.AnalyzeService,
	editService *service.EditService,
	exportService *service.ExportService,
	configManager *config.ConfigManager,
	binaryResolver *adapter.BinaryResolver,
) *App {
	return &App{
		projectService:    projectService,
		transcribeService: transcribeService,
		analyzeService:    analyzeService,
		editService:       editService,
		exportService:     exportService,
		configManager:     configManager,
		binaryResolver:    binaryResolver,
		projects:          make(map[string]*model.Project),
	}
}

// --- 项目管理（同步） ---

// CreateProject 创建新项目
func (a *App) CreateProject(name, mediaPath string) (*model.Project, error) {
	project, err := a.projectService.CreateProject(name, mediaPath)
	if err != nil {
		return nil, NewAppError(ErrCodeInternal, "创建项目失败", err.Error())
	}

	a.mu.Lock()
	a.projects[project.ID] = project
	a.mu.Unlock()

	return project, nil
}

// OpenProject 打开已有项目（通过 project.json 路径）
func (a *App) OpenProject(projectPath string) (*model.Project, error) {
	project, err := a.projectService.OpenProject(projectPath)
	if err != nil {
		return nil, NewAppError(ErrCodeParam, "打开项目失败", err.Error())
	}

	a.mu.Lock()
	a.projects[project.ID] = project
	a.mu.Unlock()

	return project, nil
}

// SaveProject 保存项目
func (a *App) SaveProject(p model.Project) error {
	err := a.projectService.SaveProject(&p)
	if err != nil {
		return NewAppError(ErrCodeInternal, "保存项目失败", err.Error())
	}

	a.mu.Lock()
	a.projects[p.ID] = &p
	a.mu.Unlock()

	return nil
}

// GetProject 获取已加载的项目
func (a *App) GetProject(projectID string) (*model.Project, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	project, ok := a.projects[projectID]
	if !ok {
		return nil, NewAppError(ErrCodeParam, fmt.Sprintf("项目 %s 未加载", projectID), "")
	}
	return project, nil
}

// --- CutList 编辑（同步） ---

// GetCutList 获取项目的剪切清单
func (a *App) GetCutList(projectID string) (*model.CutList, error) {
	cl, err := a.editService.GetCutList(projectID)
	if err != nil {
		return nil, NewAppError(ErrCodeInternal, "获取剪切清单失败", err.Error())
	}
	return cl, nil
}

// AddCutSegment 添加一个剪切段
func (a *App) AddCutSegment(projectID string, seg model.CutSegment) error {
	if err := a.editService.AddCutSegment(projectID, seg); err != nil {
		return NewAppError(ErrCodeInternal, "添加剪切段失败", err.Error())
	}
	return nil
}

// UpdateCutSegment 更新一个剪切段
func (a *App) UpdateCutSegment(projectID string, seg model.CutSegment) error {
	if err := a.editService.UpdateCutSegment(projectID, seg); err != nil {
		return NewAppError(ErrCodeInternal, "更新剪切段失败", err.Error())
	}
	return nil
}

// RemoveCutSegment 删除一个剪切段
func (a *App) RemoveCutSegment(projectID, segID string) error {
	if err := a.editService.RemoveCutSegment(projectID, segID); err != nil {
		return NewAppError(ErrCodeInternal, "删除剪切段失败", err.Error())
	}
	return nil
}
```

- [ ] **Step 2: 编译验证**

```bash
go build ./app/...
```

---

### Task 4: App 异步任务 + 查询

**Files:**
- Create: `app/app_async.go`

- [ ] **Step 1: 创建 app/app_async.go**

```go
package app

import (
	"context"
	"path/filepath"

	"smart-cut/internal/model"
)

// --- 主流程（异步，立即返回 taskID，结果走 Event） ---

// StartTranscribe 启动转录任务
func (a *App) StartTranscribe(projectID string) (string, error) {
	project, err := a.GetProject(projectID)
	if err != nil {
		return "", err
	}

	// 获取 whisper 模型路径
	settings, err := a.configManager.Load()
	if err != nil {
		return "", NewAppError(ErrCodeEnv, "加载设置失败", err.Error())
	}

	modelPath := settings.WhisperModelDir
	if modelPath == "" {
		return "", NewAppError(ErrCodeEnv, "未配置 Whisper 模型目录，请先在设置中配置", "")
	}

	taskID := a.transcribeService.StartTranscribe(project, modelPath)
	return taskID, nil
}

// StartAnalyze 启动分析任务
func (a *App) StartAnalyze(projectID string) (string, error) {
	project, err := a.GetProject(projectID)
	if err != nil {
		return "", err
	}

	transcript, err := a.transcribeService.GetTranscript(projectID)
	if err != nil {
		return "", NewAppError(ErrCodeParam, "转录结果不存在，请先完成转录", err.Error())
	}

	taskID := a.analyzeService.StartAnalyze(project, transcript)
	return taskID, nil
}

// StartExport 启动导出任务
func (a *App) StartExport(projectID string, opts model.ExportOptions) (string, error) {
	project, err := a.GetProject(projectID)
	if err != nil {
		return "", err
	}

	cl, err := a.editService.GetCutList(projectID)
	if err != nil {
		return "", NewAppError(ErrCodeParam, "剪切清单不存在，请先完成分析", err.Error())
	}

	taskID := a.exportService.StartExport(project, cl, opts)
	return taskID, nil
}

// --- 查询（同步） ---

// GetTranscript 获取转录结果
func (a *App) GetTranscript(projectID string) (*model.Transcript, error) {
	t, err := a.transcribeService.GetTranscript(projectID)
	if err != nil {
		return nil, NewAppError(ErrCodeInternal, "获取转录结果失败", err.Error())
	}
	return t, nil
}

// GetWaveform 获取波形图路径（不存在则先提取）
func (a *App) GetWaveform(projectID string) (string, error) {
	project, err := a.GetProject(projectID)
	if err != nil {
		return "", err
	}

	waveformPath := filepath.Join(project.WorkDir, "waveform.png")

	// 如果波形图已存在，直接返回路径
	ctx := context.Background()
	err = a.transcribeService.ExtractWaveform(ctx, project)
	if err != nil {
		return "", NewAppError(ErrCodeInternal, "提取波形失败", err.Error())
	}

	return waveformPath, nil
}

// ProbeMedia 探测媒体文件信息
func (a *App) ProbeMedia(path string) (*model.MediaFile, error) {
	ctx := context.Background()
	mf, err := a.transcribeService.ProbeMedia(ctx, path)
	if err != nil {
		return nil, NewAppError(ErrCodeParam, "媒体文件探测失败", err.Error())
	}
	return mf, nil
}
```

- [ ] **Step 2: 编译验证**

```bash
go build ./app/...
```

---

### Task 5: App 设置 + ProbeBinary + main.go 装配

**Files:**
- Create: `app/app_settings.go`
- Edit: `main.go`

- [ ] **Step 1: 创建 app/app_settings.go**

```go
package app

import (
	"context"
	"os/exec"
	"strings"

	"smart-cut/internal/model"
)

// --- 设置（同步） ---

// GetSettings 获取全局设置
func (a *App) GetSettings() (*model.GlobalSettings, error) {
	settings, err := a.configManager.Load()
	if err != nil {
		return nil, NewAppError(ErrCodeInternal, "加载设置失败", err.Error())
	}
	return settings, nil
}

// SaveSettings 保存全局设置
func (a *App) SaveSettings(s model.GlobalSettings) error {
	if err := a.configManager.Save(&s); err != nil {
		return NewAppError(ErrCodeInternal, "保存设置失败", err.Error())
	}
	return nil
}

// ProbeBinary 探测二进制文件路径与版本
// name: "ffmpeg" / "ffprobe" / "whisper-cli"
// 返回: 路径、版本字符串、错误
func (a *App) ProbeBinary(name string) (string, string, error) {
	path, err := a.binaryResolver.Resolve(name)
	if err != nil {
		return "", "", NewAppError(ErrCodeEnv, "未找到二进制文件 "+name, err.Error())
	}

	version := probeBinaryVersion(name, path)
	return path, version, nil
}

// probeBinaryVersion 运行二进制的 --version 参数获取版本
func probeBinaryVersion(name, path string) string {
	var cmd *exec.Cmd
	switch name {
	case "ffmpeg":
		cmd = exec.Command(path, "-version")
	case "ffprobe":
		cmd = exec.Command(path, "-version")
	case "whisper-cli":
		cmd = exec.Command(path, "--help")
	default:
		cmd = exec.Command(path, "--version")
	}

	ctx := context.Background()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}

	// 取第一行，提取版本号
	lines := strings.Split(string(out), "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return ""
}
```

- [ ] **Step 2: 重写 main.go — 装配所有 Service + EventBus 连接**

编辑 `main.go`，创建所有依赖并注册 App 为 Wails3 Service：

```go
package main

import (
	"embed"
	"log"

	"smart-cut/app"
	"smart-cut/internal/adapter"
	"smart-cut/internal/config"
	"smart-cut/internal/eventbus"
	"smart-cut/internal/service"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// 1. Config
	configManager := config.NewConfigManager("")

	// 2. EventBus（emitFunc 先为空，app 创建后注入）
	bus := eventbus.NewEventBus(nil)

	// 3. BinaryResolver（从配置加载自定义路径）
	settings, _ := configManager.Load()
	resolver := adapter.NewBinaryResolver(settings.Binaries, "resources/bin")

	// 4. Adapters
	whisperAdapter := adapter.NewWhisperAdapter(resolver)
	ffmpegAdapter := adapter.NewFFmpegAdapter(resolver)
	llmAdapter := adapter.NewLLMAdapter()

	// 5. Services
	projectService := service.NewProjectService("")
	editService := service.NewEditService()
	transcribeService := service.NewTranscribeService(whisperAdapter, ffmpegAdapter, bus, editService)
	analyzeService := service.NewAnalyzeService(llmAdapter, bus, editService)
	exportService := service.NewExportService(ffmpegAdapter, bus)

	// 6. API Layer (App)
	appInstance := app.NewApp(
		projectService,
		transcribeService,
		analyzeService,
		editService,
		exportService,
		configManager,
		resolver,
	)

	// 7. Wails App
	wailsApp := application.New(application.Options{
		Name:        "smart-cut",
		Description: "AI 口播视频自动剪辑工具",
		Services: []application.Service{
			application.NewService(appInstance),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	// 8. 注入 Wails 事件发射能力到 EventBus
	bus.SetEmitFunc(func(name string, data interface{}) {
		wailsApp.Event.Emit(name, data)
	})

	// 9. 创建窗口
	wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Smart-Cut",
		Width:            1440,
		Height:           900,
		MinWidth:         1280,
		MinHeight:        720,
		BackgroundColour: application.NewRGB(24, 24, 27),
		URL:              "/",
	})

	err := wailsApp.Run()
	if err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 3: 编译验证**

```bash
go build ./...
```

确保整个项目编译通过（包括 main.go）。

---

### Task 6: 前端 — API client + 类型定义 + 事件订阅

**Files:**
- Create: `frontend/src/api/types.ts`
- Create: `frontend/src/api/events.ts`
- Create: `frontend/src/api/client.ts`
- Create: `frontend/src/api/bindings.d.ts`（类型声明 shim）

**前置条件：** 后端 Task 1-5 已完成，`go build ./...` 通过。

- [ ] **Step 1: 创建 types.ts — 镜像 Go model 的 TypeScript 类型**

创建 `frontend/src/api/types.ts`：

```typescript
// 镜像 smart-cut/internal/model 的类型定义

export type ProjectStatus = "draft" | "transcribed" | "analyzed" | "exported";
export type ExportMode = "lossless" | "reencode";
export type CutDecision = "keep" | "remove";
export type CutReason = "filler" | "silence" | "dup_or_error" | "manual";
export type CutSource = "ai" | "manual";
export type TaskStatus = "running" | "done" | "error";

export interface MediaFile {
  path: string;
  durationMs: number;
  format: string;
  width: number;
  height: number;
  fps: number;
  hasAudio: boolean;
}

export interface SubtitleStyle {
  fontFamily: string;
  fontSize: number;
  color: string;
  highlight: string;
  position: string;
  bgColor: string;
  bgOpacity: number;
}

export interface LLMConfig {
  baseUrl: string;
  apiKey: string;
  model: string;
}

export interface ProjectSettings {
  exportMode: ExportMode;
  silenceMs: number;
  fillerDict: string[];
  llmConfig: LLMConfig;
  subtitleStyle: SubtitleStyle;
}

export interface Project {
  id: string;
  name: string;
  createdAt: string;
  updatedAt: string;
  workDir: string;
  media: MediaFile;
  status: ProjectStatus;
  settings: ProjectSettings;
}

export interface Word {
  text: string;
  startMs: number;
  endMs: number;
  confidence: number;
}

export interface Segment {
  id: number;
  text: string;
  startMs: number;
  endMs: number;
  words: Word[];
}

export interface Transcript {
  language: string;
  segments: Segment[];
  text: string;
}

export interface CutSegment {
  id: string;
  startMs: number;
  endMs: number;
  decision: CutDecision;
  reason: CutReason;
  source: CutSource;
  confidence: number;
  note: string;
}

export interface CutList {
  projectId: string;
  segments: CutSegment[];
  version: number;
}

export interface KeepSegment {
  startMs: number;
  endMs: number;
}

export interface ExportOptions {
  mode: ExportMode;
  includeSubtitle: boolean;
  outputPath: string;
}

export interface EncodeOpts {
  videoCodec: string;
  audioCodec: string;
  videoBitrate: string;
  crf: number;
  preset: string;
}

export interface GlobalSettings {
  binaries: Record<string, string>;
  whisperModelDir: string;
  defaultLLM: LLMConfig;
  theme: string;
}

export interface ProgressEvent {
  taskId: string;
  stage: string;
  step: string;
  progress: number;
  status: TaskStatus;
  error?: string;
  payload?: unknown;
}

export interface AppError {
  code: string;
  message: string;
  detail: string;
}
```

- [ ] **Step 2: 创建 events.ts — 事件订阅封装**

创建 `frontend/src/api/events.ts`：

```typescript
import { Events } from "@wailsio/runtime";
import type { ProgressEvent, Transcript, CutList } from "./types";

export function onProgress(cb: (event: ProgressEvent) => void): () => void {
  return Events.On("progress", (event: ProgressEvent) => cb(event));
}

export function onTranscriptReady(cb: (transcript: Transcript) => void): () => void {
  return Events.On("transcript:ready", (transcript: Transcript) => cb(transcript));
}

export function onCutListReady(cb: (cutList: CutList) => void): () => void {
  return Events.On("cutlist:ready", (cutList: CutList) => cb(cutList));
}

export function onExportDone(cb: (exportPath: string) => void): () => void {
  return Events.On("export:done", (exportPath: string) => cb(exportPath));
}

export function onLog(cb: (logLine: string) => void): () => void {
  return Events.On("log", (logLine: string) => cb(logLine));
}

/** 取消所有事件订阅（页面卸载时调用） */
export function offAll(): void {
  Events.Off("progress");
  Events.Off("transcript:ready");
  Events.Off("cutlist:ready");
  Events.Off("export:done");
  Events.Off("log");
}
```

- [ ] **Step 3: 创建 bindings.d.ts — 类型声明 shim**

创建 `frontend/src/api/bindings.d.ts`，为 Wails 自动生成的 binding 文件提供类型声明，避免 TypeScript 编译报错（bindings 在 `wails3 dev`/`wails3 build` 时自动生成）：

```typescript
// 为 Wails3 自动生成的 bindings 提供类型声明
// 实际文件在 wails3 dev/build 时生成到 frontend/bindings/smart-cut/app/

declare module "../../bindings/smart-cut/app/app.js" {
  import type {
    Project,
    CutList,
    CutSegment,
    Transcript,
    ExportOptions,
    GlobalSettings,
    MediaFile,
  } from "./types";

  export function CreateProject(name: string, mediaPath: string): Promise<Project>;
  export function OpenProject(projectPath: string): Promise<Project>;
  export function SaveProject(p: Project): Promise<void>;
  export function GetProject(projectID: string): Promise<Project>;

  export function GetCutList(projectID: string): Promise<CutList>;
  export function AddCutSegment(projectID: string, seg: CutSegment): Promise<void>;
  export function UpdateCutSegment(projectID: string, seg: CutSegment): Promise<void>;
  export function RemoveCutSegment(projectID: string, segID: string): Promise<void>;

  export function StartTranscribe(projectID: string): Promise<string>;
  export function StartAnalyze(projectID: string): Promise<string>;
  export function StartExport(projectID: string, opts: ExportOptions): Promise<string>;

  export function GetTranscript(projectID: string): Promise<Transcript>;
  export function GetWaveform(projectID: string): Promise<string>;
  export function ProbeMedia(path: string): Promise<MediaFile>;

  export function GetSettings(): Promise<GlobalSettings>;
  export function SaveSettings(s: GlobalSettings): Promise<void>;
  export function ProbeBinary(name: string): Promise<{ path: string; version: string }>;
}
```

- [ ] **Step 4: 创建 client.ts — API 调用封装**

创建 `frontend/src/api/client.ts`：

```typescript
// 统一封装所有后端 API 调用
// binding 文件由 wails3 dev/build 自动生成
import * as App from "../../bindings/smart-cut/app/app.js";
import type {
  Project,
  CutList,
  CutSegment,
  Transcript,
  ExportOptions,
  GlobalSettings,
  MediaFile,
} from "./types";

// --- 项目管理 ---

export async function createProject(name: string, mediaPath: string): Promise<Project> {
  return App.CreateProject(name, mediaPath);
}

export async function openProject(projectPath: string): Promise<Project> {
  return App.OpenProject(projectPath);
}

export async function saveProject(p: Project): Promise<void> {
  return App.SaveProject(p);
}

export async function getProject(projectID: string): Promise<Project> {
  return App.GetProject(projectID);
}

// --- CutList 编辑 ---

export async function getCutList(projectID: string): Promise<CutList> {
  return App.GetCutList(projectID);
}

export async function addCutSegment(projectID: string, seg: CutSegment): Promise<void> {
  return App.AddCutSegment(projectID, seg);
}

export async function updateCutSegment(projectID: string, seg: CutSegment): Promise<void> {
  return App.UpdateCutSegment(projectID, seg);
}

export async function removeCutSegment(projectID: string, segID: string): Promise<void> {
  return App.RemoveCutSegment(projectID, segID);
}

// --- 异步任务 ---

export async function startTranscribe(projectID: string): Promise<string> {
  return App.StartTranscribe(projectID);
}

export async function startAnalyze(projectID: string): Promise<string> {
  return App.StartAnalyze(projectID);
}

export async function startExport(projectID: string, opts: ExportOptions): Promise<string> {
  return App.StartExport(projectID, opts);
}

// --- 查询 ---

export async function getTranscript(projectID: string): Promise<Transcript> {
  return App.GetTranscript(projectID);
}

export async function getWaveform(projectID: string): Promise<string> {
  return App.GetWaveform(projectID);
}

export async function probeMedia(path: string): Promise<MediaFile> {
  return App.ProbeMedia(path);
}

// --- 设置 ---

export async function getSettings(): Promise<GlobalSettings> {
  return App.GetSettings();
}

export async function saveSettings(s: GlobalSettings): Promise<void> {
  return App.SaveSettings(s);
}

export async function probeBinary(name: string): Promise<{ path: string; version: string }> {
  return App.ProbeBinary(name);
}
```

- [ ] **Step 5: 验证前端编译（bindings 尚未生成时会有类型报错，属正常）**

```bash
cd frontend && npx tsc --noEmit 2>&1 | head -20
```

> **注意：** 如果 `frontend/bindings/smart-cut/app/app.js` 尚未生成（需要运行 `wails3 dev`），TypeScript 会报找不到模块。这是预期的。bindings.d.ts shim 应该能覆盖。如果仍报错，确认 `tsconfig.json` 的 `paths` 映射正确。

---

### Task 7: 前端 — 路由 + Layout + 主题切换

**Files:**
- Edit: `frontend/src/App.tsx`
- Create: `frontend/src/layouts/AppLayout.tsx`
- Create: `frontend/src/stores/settings.ts`
- Create: `frontend/src/stores/project.ts`

- [ ] **Step 1: 创建 settings store**

创建 `frontend/src/stores/settings.ts`：

```typescript
import { create } from "zustand";
import { getSettings, saveSettings } from "../api/client";
import type { GlobalSettings } from "../api/types";

interface SettingsStore {
  settings: GlobalSettings | null;
  loading: boolean;
  loadSettings: () => Promise<void>;
  updateSettings: (s: GlobalSettings) => Promise<void>;
}

export const useSettingsStore = create<SettingsStore>((set) => ({
  settings: null,
  loading: false,

  loadSettings: async () => {
    set({ loading: true });
    try {
      const settings = await getSettings();
      set({ settings, loading: false });
      applyTheme(settings.theme);
    } catch (e) {
      set({ loading: false });
      console.error("加载设置失败:", e);
    }
  },

  updateSettings: async (s: GlobalSettings) => {
    await saveSettings(s);
    set({ settings: s });
    applyTheme(s.theme);
  },
}));

function applyTheme(theme: string) {
  if (theme === "dark") {
    document.documentElement.classList.add("dark");
  } else {
    document.documentElement.classList.remove("dark");
  }
}
```

- [ ] **Step 2: 创建 project store**

创建 `frontend/src/stores/project.ts`：

```typescript
import { create } from "zustand";
import type { Project } from "../api/types";

interface ProjectStore {
  currentProject: Project | null;
  setCurrentProject: (p: Project | null) => void;
}

export const useProjectStore = create<ProjectStore>((set) => ({
  currentProject: null,
  setCurrentProject: (p) => set({ currentProject: p }),
}));
```

- [ ] **Step 3: 创建 AppLayout**

创建 `frontend/src/layouts/AppLayout.tsx`：

```tsx
import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { Settings, FolderPlus, Scissors } from "lucide-react";
import { cn } from "../lib/utils";

export function AppLayout() {
  const navigate = useNavigate();

  const navItems = [
    { to: "/project/new", label: "新建项目", icon: FolderPlus },
    { to: "/settings", label: "设置", icon: Settings },
  ];

  return (
    <div className="flex h-screen w-screen overflow-hidden bg-background text-foreground">
      {/* 侧边栏 */}
      <aside className="flex w-16 flex-col items-center gap-4 border-r border-border bg-zinc-900 py-4">
        <div
          className="flex h-10 w-10 cursor-pointer items-center justify-center rounded-lg bg-primary text-primary-foreground"
          onClick={() => navigate("/")}
          title="Smart-Cut"
        >
          <Scissors className="h-5 w-5" />
        </div>

        <nav className="flex flex-1 flex-col gap-2">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) =>
                cn(
                  "flex h-10 w-10 items-center justify-center rounded-lg transition-colors",
                  isActive
                    ? "bg-accent text-accent-foreground"
                    : "text-muted-foreground hover:bg-accent/50 hover:text-foreground"
                )
              }
              title={item.label}
            >
              <item.icon className="h-5 w-5" />
            </NavLink>
          ))}
        </nav>
      </aside>

      {/* 主内容区 */}
      <main className="flex-1 overflow-auto">
        <Outlet />
      </main>
    </div>
  );
}
```

- [ ] **Step 4: 重写 App.tsx — 路由配置**

编辑 `frontend/src/App.tsx`：

```tsx
import { useEffect } from "react";
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { AppLayout } from "./layouts/AppLayout";
import { NewProject } from "./pages/NewProject";
import { Settings } from "./pages/Settings";
import { Workbench } from "./pages/Workbench";
import { useSettingsStore } from "./stores/settings";

function App() {
  const loadSettings = useSettingsStore((s) => s.loadSettings);

  useEffect(() => {
    loadSettings();
  }, [loadSettings]);

  return (
    <BrowserRouter>
      <Routes>
        <Route element={<AppLayout />}>
          <Route path="/" element={<Navigate to="/project/new" replace />} />
          <Route path="/project/new" element={<NewProject />} />
          <Route path="/project/:id" element={<Workbench />} />
          <Route path="/settings" element={<Settings />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}

export default App;
```

- [ ] **Step 5: 创建占位页面（Task 8-9 会填充）**

创建 `frontend/src/pages/Workbench.tsx`（占位，Plan 5 实现 Timeline）：

```tsx
export function Workbench() {
  return (
    <div className="flex h-full items-center justify-center">
      <p className="text-muted-foreground">工作台（Plan 5 实现 Timeline 编辑器）</p>
    </div>
  );
}
```

临时占位 `frontend/src/pages/NewProject.tsx` 和 `frontend/src/pages/Settings.tsx`（避免编译报错，Task 8-9 会实现完整版）：

```tsx
// NewProject.tsx (临时占位)
export function NewProject() {
  return <div className="p-8 text-lg">新建项目（Task 9 实现）</div>;
}
```

```tsx
// Settings.tsx (临时占位)
export function Settings() {
  return <div className="p-8 text-lg">设置（Task 8 实现）</div>;
}
```

- [ ] **Step 6: 验证前端编译**

```bash
cd frontend && npx tsc --noEmit
```

---

### Task 8: 前端 — Shadcn 基础组件 + 设置页

**Files:**
- Create: `frontend/src/components/ui/button.tsx`
- Create: `frontend/src/components/ui/input.tsx`
- Create: `frontend/src/components/ui/label.tsx`
- Create: `frontend/src/components/ui/card.tsx`
- Create: `frontend/src/components/ui/select.tsx`
- Create: `frontend/src/components/ui/switch.tsx`
- Edit: `frontend/src/pages/Settings.tsx`

- [ ] **Step 1: 创建 Shadcn UI 组件**

创建以下组件文件（标准 Shadcn/ui 源码，基于 Radix + Tailwind）：

**`frontend/src/components/ui/button.tsx`**：

```tsx
import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "../../lib/utils";

const buttonVariants = cva(
  "inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50",
  {
    variants: {
      variant: {
        default: "bg-primary text-primary-foreground shadow hover:bg-primary/90",
        destructive: "bg-destructive text-destructive-foreground shadow-sm hover:bg-destructive/90",
        outline: "border border-input bg-background shadow-sm hover:bg-accent hover:text-accent-foreground",
        secondary: "bg-secondary text-secondary-foreground shadow-sm hover:bg-secondary/80",
        ghost: "hover:bg-accent hover:text-accent-foreground",
        link: "text-primary underline-offset-4 hover:underline",
      },
      size: {
        default: "h-9 px-4 py-2",
        sm: "h-8 rounded-md px-3 text-xs",
        lg: "h-10 rounded-md px-8",
        icon: "h-9 w-9",
      },
    },
    defaultVariants: { variant: "default", size: "default" },
  }
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  asChild?: boolean;
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, asChild = false, ...props }, ref) => {
    const Comp = asChild ? Slot : "button";
    return <Comp className={cn(buttonVariants({ variant, size, className }))} ref={ref} {...props} />;
  }
);
Button.displayName = "Button";

export { Button, buttonVariants };
```

**`frontend/src/components/ui/input.tsx`**：

```tsx
import * as React from "react";
import { cn } from "../../lib/utils";

export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {}

const Input = React.forwardRef<HTMLInputElement, InputProps>(({ className, type, ...props }, ref) => {
  return (
    <input
      type={type}
      className={cn(
        "flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50",
        className
      )}
      ref={ref}
      {...props}
    />
  );
});
Input.displayName = "Input";

export { Input };
```

**`frontend/src/components/ui/label.tsx`**：

```tsx
import * as React from "react";
import * as LabelPrimitive from "@radix-ui/react-label";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "../../lib/utils";

const labelVariants = cva(
  "text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
);

const Label = React.forwardRef<
  React.ElementRef<typeof LabelPrimitive.Root>,
  React.ComponentPropsWithoutRef<typeof LabelPrimitive.Root> & VariantProps<typeof labelVariants>
>(({ className, ...props }, ref) => (
  <LabelPrimitive.Root ref={ref} className={cn(labelVariants(), className)} {...props} />
));
Label.displayName = LabelPrimitive.Root.displayName;

export { Label };
```

**`frontend/src/components/ui/card.tsx`**：

```tsx
import * as React from "react";
import { cn } from "../../lib/utils";

const Card = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div
      ref={ref}
      className={cn("rounded-xl border bg-card text-card-foreground shadow", className)}
      {...props}
    />
  )
);
Card.displayName = "Card";

const CardHeader = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div ref={ref} className={cn("flex flex-col space-y-1.5 p-6", className)} {...props} />
  )
);
CardHeader.displayName = "CardHeader";

const CardTitle = React.forwardRef<HTMLParagraphElement, React.HTMLAttributes<HTMLHeadingElement>>(
  ({ className, ...props }, ref) => (
    <h3 ref={ref} className={cn("font-semibold leading-none tracking-tight", className)} {...props} />
  )
);
CardTitle.displayName = "CardTitle";

const CardDescription = React.forwardRef<
  HTMLParagraphElement,
  React.HTMLAttributes<HTMLParagraphElement>
>(({ className, ...props }, ref) => (
  <p ref={ref} className={cn("text-sm text-muted-foreground", className)} {...props} />
));
CardDescription.displayName = "CardDescription";

const CardContent = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div ref={ref} className={cn("p-6 pt-0", className)} {...props} />
  )
);
CardContent.displayName = "CardContent";

const CardFooter = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div ref={ref} className={cn("flex items-center p-6 pt-0", className)} {...props} />
  )
);
CardFooter.displayName = "CardFooter";

export { Card, CardHeader, CardFooter, CardTitle, CardDescription, CardContent };
```

**`frontend/src/components/ui/switch.tsx`**：

```tsx
import * as React from "react";
import * as SwitchPrimitives from "@radix-ui/react-switch";
import { cn } from "../../lib/utils";

const Switch = React.forwardRef<
  React.ElementRef<typeof SwitchPrimitives.Root>,
  React.ComponentPropsWithoutRef<typeof SwitchPrimitives.Root>
>(({ className, ...props }, ref) => (
  <SwitchPrimitives.Root
    className={cn(
      "peer inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full border-2 border-transparent shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:cursor-not-allowed disabled:opacity-50 data-[state=checked]:bg-primary data-[state=unchecked]:bg-input",
      className
    )}
    {...props}
    ref={ref}
  >
    <SwitchPrimitives.Thumb
      className={cn(
        "pointer-events-none block h-4 w-4 rounded-full bg-background shadow-lg ring-0 transition-transform data-[state=checked]:translate-x-4 data-[state=unchecked]:translate-x-0"
      )}
    />
  </SwitchPrimitives.Root>
));
Switch.displayName = SwitchPrimitives.Root.displayName;

export { Switch };
```

- [ ] **Step 2: 实现设置页**

编辑 `frontend/src/pages/Settings.tsx`：

```tsx
import { useEffect, useState } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/card";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { Label } from "../components/ui/label";
import { Switch } from "../components/ui/switch";
import { useSettingsStore } from "../stores/settings";
import { probeBinary } from "../api/client";
import type { GlobalSettings } from "../api/types";

export function Settings() {
  const { settings, loadSettings, updateSettings } = useSettingsStore();
  const [form, setForm] = useState<GlobalSettings | null>(null);
  const [saving, setSaving] = useState(false);
  const [probeResults, setProbeResults] = useState<Record<string, { path: string; version: string } | null>>({});

  useEffect(() => {
    loadSettings();
  }, [loadSettings]);

  useEffect(() => {
    if (settings) setForm(settings);
  }, [settings]);

  if (!form) return <div className="p-8">加载中...</div>;

  const handleProbe = async (name: string) => {
    try {
      const result = await probeBinary(name);
      setProbeResults((prev) => ({ ...prev, [name]: result }));
    } catch (e) {
      setProbeResults((prev) => ({ ...prev, [name]: null }));
      console.error(`${name} 探测失败:`, e);
    }
  };

  const handleSave = async () => {
    if (!form) return;
    setSaving(true);
    try {
      await updateSettings(form);
    } finally {
      setSaving(false);
    }
  };

  const binaries = ["ffmpeg", "ffprobe", "whisper-cli"];

  return (
    <div className="mx-auto max-w-2xl space-y-6 p-8">
      <h1 className="text-2xl font-bold">设置</h1>

      {/* 二进制路径 */}
      <Card>
        <CardHeader>
          <CardTitle>二进制路径</CardTitle>
          <CardDescription>配置外部工具路径，留空则使用随包或系统 PATH</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {binaries.map((name) => (
            <div key={name} className="space-y-2">
              <Label htmlFor={`bin-${name}`}>{name}</Label>
              <div className="flex gap-2">
                <Input
                  id={`bin-${name}`}
                  value={form.binaries[name] || ""}
                  onChange={(e) =>
                    setForm({
                      ...form,
                      binaries: { ...form.binaries, [name]: e.target.value },
                    })
                  }
                  placeholder={`留空使用系统 PATH`}
                />
                <Button variant="outline" size="sm" onClick={() => handleProbe(name)}>
                  探测
                </Button>
              </div>
              {probeResults[name] && (
                <p className="text-xs text-muted-foreground">
                  ✓ {probeResults[name]!.version}
                </p>
              )}
              {probeResults[name] === null && (
                <p className="text-xs text-destructive">✗ 未找到</p>
              )}
            </div>
          ))}
        </CardContent>
      </Card>

      {/* Whisper 模型 */}
      <Card>
        <CardHeader>
          <CardTitle>Whisper 模型</CardTitle>
          <CardDescription>ggml 模型文件所在目录</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-2">
            <Label htmlFor="whisper-model">模型目录</Label>
            <Input
              id="whisper-model"
              value={form.whisperModelDir}
              onChange={(e) => setForm({ ...form, whisperModelDir: e.target.value })}
              placeholder="/path/to/models"
            />
          </div>
        </CardContent>
      </Card>

      {/* LLM 配置 */}
      <Card>
        <CardHeader>
          <CardTitle>LLM 配置</CardTitle>
          <CardDescription>OpenAI 兼容 API（用于语气词/重复检测）</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="llm-baseurl">API Base URL</Label>
            <Input
              id="llm-baseurl"
              value={form.defaultLLM.baseUrl}
              onChange={(e) =>
                setForm({
                  ...form,
                  defaultLLM: { ...form.defaultLLM, baseUrl: e.target.value },
                })
              }
              placeholder="https://api.openai.com/v1"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="llm-apikey">API Key</Label>
            <Input
              id="llm-apikey"
              type="password"
              value={form.defaultLLM.apiKey}
              onChange={(e) =>
                setForm({
                  ...form,
                  defaultLLM: { ...form.defaultLLM, apiKey: e.target.value },
                })
              }
              placeholder="sk-..."
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="llm-model">模型</Label>
            <Input
              id="llm-model"
              value={form.defaultLLM.model}
              onChange={(e) =>
                setForm({
                  ...form,
                  defaultLLM: { ...form.defaultLLM, model: e.target.value },
                })
              }
              placeholder="gpt-4o-mini"
            />
          </div>
        </CardContent>
      </Card>

      {/* 主题 */}
      <Card>
        <CardHeader>
          <CardTitle>外观</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-between">
            <Label htmlFor="theme-switch">暗色模式</Label>
            <Switch
              id="theme-switch"
              checked={form.theme === "dark"}
              onCheckedChange={(checked) =>
                setForm({ ...form, theme: checked ? "dark" : "light" })
              }
            />
          </div>
        </CardContent>
      </Card>

      <Button onClick={handleSave} disabled={saving}>
        {saving ? "保存中..." : "保存设置"}
      </Button>
    </div>
  );
}
```

- [ ] **Step 3: 验证编译**

```bash
cd frontend && npx tsc --noEmit
```

---

### Task 9: 前端 — 新建项目页

**Files:**
- Edit: `frontend/src/pages/NewProject.tsx`

- [ ] **Step 1: 实现新建项目页**

编辑 `frontend/src/pages/NewProject.tsx`：

```tsx
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/card";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { Label } from "../components/ui/label";
import { createProject, probeMedia } from "../api/client";
import { useProjectStore } from "../stores/project";
import type { MediaFile } from "../api/types";

export function NewProject() {
  const navigate = useNavigate();
  const setCurrentProject = useProjectStore((s) => s.setCurrentProject);

  const [name, setName] = useState("");
  const [mediaPath, setMediaPath] = useState("");
  const [mediaInfo, setMediaInfo] = useState<MediaFile | null>(null);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState("");

  const handleProbe = async () => {
    if (!mediaPath) return;
    try {
      const info = await probeMedia(mediaPath);
      setMediaInfo(info);
      setError("");
    } catch (e) {
      setError("媒体文件探测失败: " + String(e));
      setMediaInfo(null);
    }
  };

  const handleCreate = async () => {
    if (!name || !mediaPath) {
      setError("请填写项目名称和媒体文件路径");
      return;
    }

    setCreating(true);
    setError("");
    try {
      const project = await createProject(name, mediaPath);
      setCurrentProject(project);
      navigate(`/project/${project.id}`);
    } catch (e) {
      setError("创建项目失败: " + String(e));
    } finally {
      setCreating(false);
    }
  };

  return (
    <div className="mx-auto max-w-2xl space-y-6 p-8">
      <div>
        <h1 className="text-2xl font-bold">新建项目</h1>
        <p className="mt-1 text-sm text-muted-foreground">导入视频文件，开始 AI 自动剪辑</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>项目信息</CardTitle>
          <CardDescription>设置项目名称并选择要剪辑的视频文件</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="project-name">项目名称</Label>
            <Input
              id="project-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="我的口播视频"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="media-path">视频文件路径</Label>
            <div className="flex gap-2">
              <Input
                id="media-path"
                value={mediaPath}
                onChange={(e) => setMediaPath(e.target.value)}
                placeholder="/path/to/video.mp4"
              />
              <Button variant="outline" size="sm" onClick={handleProbe} disabled={!mediaPath}>
                探测
              </Button>
            </div>
          </div>

          {mediaInfo && (
            <div className="rounded-lg border border-border bg-muted/30 p-4 text-sm">
              <div className="grid grid-cols-2 gap-2">
                <span className="text-muted-foreground">分辨率:</span>
                <span>{mediaInfo.width}×{mediaInfo.height}</span>
                <span className="text-muted-foreground">帧率:</span>
                <span>{mediaInfo.fps.toFixed(2)} fps</span>
                <span className="text-muted-foreground">时长:</span>
                <span>{(mediaInfo.durationMs / 1000).toFixed(1)} 秒</span>
                <span className="text-muted-foreground">音频:</span>
                <span>{mediaInfo.hasAudio ? "有" : "无"}</span>
              </div>
            </div>
          )}

          {error && <p className="text-sm text-destructive">{error}</p>}

          <Button onClick={handleCreate} disabled={creating || !name || !mediaPath}>
            {creating ? "创建中..." : "创建项目"}
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
```

- [ ] **Step 2: 验证编译**

```bash
cd frontend && npx tsc --noEmit
```

---

### Task 10: 最终验证

- [ ] **Step 1: 后端编译与测试**

```bash
go build ./...
go test ./internal/... -count=1
```

确保 69+ 个测试全部通过（原有 69 + 新增 config 测试 5 个）。

- [ ] **Step 2: 前端编译**

```bash
cd frontend && npx tsc --noEmit
```

- [ ] **Step 3: 生成 Wails bindings + 前端构建**

```bash
cd d:\workspace\go\src\smart-cut
wails3 build -platform windows/amd64
```

或者用 dev 模式生成 bindings：

```bash
wails3 dev -config ./build/config.yml
```

> **注意：** bindings 会自动生成到 `frontend/bindings/smart-cut/app/app.js`。生成后 `client.ts` 中的 import 才能解析。

- [ ] **Step 4: 验证 bindings 生成正确**

检查 `frontend/bindings/smart-cut/app/` 目录下是否生成了 `app.js`，且包含以下方法：
- CreateProject, OpenProject, SaveProject, GetProject
- GetCutList, AddCutSegment, UpdateCutSegment, RemoveCutSegment
- StartTranscribe, StartAnalyze, StartExport
- GetTranscript, GetWaveform, ProbeMedia
- GetSettings, SaveSettings, ProbeBinary

- [ ] **Step 5: 运行应用手动验证**

```bash
wails3 dev -config ./build/config.yml
```

验证：
1. 应用启动，显示新建项目页
2. 点击侧边栏"设置"图标，设置页正常加载
3. 设置页填写信息并保存，重启后设置仍在
4. 探测二进制按钮正常工作（ffmpeg/ffprobe/whisper-cli）

- [ ] **Step 6: Git 提交**

按逻辑分组提交：
1. `feat(config): add ConfigManager for GlobalSettings persistence`
2. `feat(app): add API layer with App facade, AppError, and all bindings`
3. `fix(eventbus): correct Wails3 Event API name in comments`
4. `feat(frontend): add API client, router, layout, and Shadcn components`
5. `feat(frontend): add Settings page and New Project page`
6. `docs: add plan 4 - API layer and frontend basics`

---

## 注意事项

1. **Wails3 Event API**：Go 端用 `app.Event.Emit`（单数），JS 端用 `Events.On`（复数）。这是 Wails3 v3 的不对称设计。

2. **Bindings 生成时机**：`frontend/bindings/` 目录下的文件由 `wails3 dev` 或 `wails3 build` 自动生成。在未运行这些命令前，TypeScript 可能报模块找不到。`bindings.d.ts` shim 文件用于缓解此问题。

3. **App 是单一 Service**：所有 API 方法都在 `App` 结构体上，注册为一个 Wails3 Service。前端通过 `frontend/bindings/smart-cut/app/app.js` 调用。

4. **异步约定**：`StartTranscribe`/`StartAnalyze`/`StartExport` 立即返回 taskID，不阻塞。结果通过 EventBus → `app.Event.Emit` → 前端 `Events.On` 推送。

5. **主题切换**：通过 `<html>` 元素的 `dark` class 控制，CSS 变量在 `index.css` 中已定义。

6. **probeBinaryVersion**：运行二进制的 `--version` 参数，取第一行输出。whisper-cli 用 `--help`（因为它可能不支持 `--version`）。

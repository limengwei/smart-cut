# Smart-Cut Plan 1: 脚手架与数据模型

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在现有 Wails3 空壳脚手架基础上，改造为完整的 Smart-Cut 项目骨架：TypeScript 化前端、集成 Tailwind+Shadcn UI 体系、建立后端 Go 目录结构、实现全部核心数据模型（model 包）并配齐单测。

**Architecture:** Wails3 (Go 后端) + React 18 + TypeScript 前端。前端用 Shadcn/ui + Tailwind CSS + Radix Primitives。后端按设计文档分 5 层（API/Service/Pipeline/Adapter/model），本 plan 只建 model 层和目录骨架。

**Tech Stack:** Go 1.25, Wails3 v3.0.0-alpha.96, React 18, TypeScript 5, Tailwind CSS 3, Shadcn/ui, zustand, lucide-react, Vitest, Go testify

---

## File Structure

本 plan 涉及的文件变更：

```
smart-cut/
├── go.mod                              # 改 module 名
├── main.go                             # 改造（删除示例代码）
├── greetservice.go                     # 删除
├── internal/
│   ├── model/
│   │   ├── project.go                  # 新建：Project, MediaFile, ProjectSettings 等
│   │   ├── transcript.go               # 新建：Transcript, Word, Segment
│   │   ├── cutlist.go                  # 新建：CutList, CutSegment, CutDecision 等
│   │   ├── llm.go                      # 新建：LLM 分析请求/响应契约
│   │   ├── event.go                    # 新建：ProgressEvent, TaskStatus 等
│   │   ├── settings.go                 # 新建：GlobalSettings, LLMConfig 等
│   │   └── export.go                   # 新建：ExportOptions, EncodeOpts 等
│   ├── service/                        # 新建空目录（加 .gitkeep）
│   ├── pipeline/                       # 新建空目录
│   ├── adapter/                        # 新建空目录
│   ├── eventbus/                       # 新建空目录
│   └── config/                         # 新建空目录
├── frontend/
│   ├── package.json                    # 改造：加完整依赖
│   ├── tsconfig.json                   # 改造：严格 TS 配置
│   ├── vite.config.ts                  # 改名 .js→.ts + 改造
│   ├── tailwind.config.js              # 新建
│   ├── postcss.config.js               # 新建
│   ├── components.json                 # 新建：Shadcn 配置
│   ├── .eslintrc.cjs                   # 新建
│   ├── .prettierrc                     # 新建
│   ├── index.html                      # 改造：title + 入口改 tsx
│   ├── src/
│   │   ├── main.tsx                    # 改名 jsx→tsx + 改造
│   │   ├── App.tsx                     # 改名 jsx→tsx + 改造为空壳
│   │   ├── index.css                   # 新建：Tailwind 指令 + CSS 变量
│   │   ├── lib/
│   │   │   └── utils.ts                # 新建：cn() 工具函数
│   │   ├── pages/                      # 新建空目录
│   │   ├── components/                 # 新建空目录
│   │   ├── stores/                     # 新建空目录
│   │   ├── api/                        # 新建空目录
│   │   └── remotion/                   # 新建空目录
│   └── public/
│       └── style.css                   # 删除（被 index.css 替代）
└── testdata/                           # 新建空目录（加 .gitkeep）
```

---

### Task 1: 验证 Wails3 基础空壳能启动

**Files:**
- 无文件变更，仅验证

- [ ] **Step 1: 安装前端基础依赖**

在 `frontend/` 目录执行：

```bash
cd frontend && npm install
```

Expected: 生成 `node_modules/`，无报错。

- [ ] **Step 2: 启动 Wails3 dev 模式验证**

在项目根目录执行：

```bash
wails3 dev -config ./build/config.yml -port 9245
```

Expected: 弹出桌面窗口，显示 "Wails + React" 页面，有 Greet 输入框和时钟。确认后关闭窗口。

- [ ] **Step 3: 确认 Go 编译通过**

```bash
go build ./...
```

Expected: 无报错。

---

### Task 2: 改造 Go module 名并清理示例代码

**Files:**
- Modify: `go.mod`
- Delete: `greetservice.go`
- Modify: `main.go`

- [ ] **Step 1: 改 module 名为 smart-cut**

将 `go.mod` 第一行 `module changeme` 改为 `module smart-cut`。

修改后的 `go.mod` 第一行：
```go
module smart-cut
```

- [ ] **Step 2: 删除示例 GreetService**

删除文件 `greetservice.go`。

- [ ] **Step 3: 改造 main.go 为干净入口**

用以下内容替换 `main.go` 全部内容：

```go
package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := application.New(application.Options{
		Name:        "smart-cut",
		Description: "AI 口播视频自动剪辑工具",
		Services: []application.Service{
			// Services will be registered here in later plans
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Smart-Cut",
		BackgroundColour: application.NewRGB(24, 24, 27),
		URL:              "/",
	})

	err := app.Run()
	if err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 4: 验证编译**

```bash
go build ./...
```

Expected: 无报错。

- [ ] **Step 5: Commit**

```bash
git add go.mod main.go
git rm greetservice.go
git commit -m "chore: rename module to smart-cut, clean up scaffold demo"
```

---

### Task 3: 建立后端 Go 目录结构

**Files:**
- Create: `internal/model/.gitkeep`
- Create: `internal/service/.gitkeep`
- Create: `internal/pipeline/.gitkeep`
- Create: `internal/adapter/.gitkeep`
- Create: `internal/eventbus/.gitkeep`
- Create: `internal/config/.gitkeep`
- Create: `testdata/.gitkeep`

- [ ] **Step 1: 创建目录结构**

```bash
mkdir -p internal/model internal/service internal/pipeline internal/adapter internal/eventbus internal/config testdata
```

- [ ] **Step 2: 添加 .gitkeep 占位**

在每个空目录下创建 `.gitkeep` 空文件，确保 git 能追踪空目录。

```bash
touch internal/model/.gitkeep internal/service/.gitkeep internal/pipeline/.gitkeep internal/adapter/.gitkeep internal/eventbus/.gitkeep internal/config/.gitkeep testdata/.gitkeep
```

- [ ] **Step 3: Commit**

```bash
git add internal/ testdata/
git commit -m "chore: create backend directory structure"
```

---

### Task 4: 实现 model 层 — project.go

**Files:**
- Create: `internal/model/project.go`

- [ ] **Step 1: 编写 project.go**

创建 `internal/model/project.go`，内容如下：

```go
package model

import "time"

// ProjectStatus 表示工程的当前阶段
type ProjectStatus string

const (
	StatusDraft     ProjectStatus = "draft"     // 刚创建，未转录
	StatusTranscribed ProjectStatus = "transcribed" // 已转录
	StatusAnalyzed  ProjectStatus = "analyzed"  // 已分析
	StatusExported  ProjectStatus = "exported"  // 已导出
)

// ExportMode 导出模式
type ExportMode string

const (
	ExportLossless ExportMode = "lossless" // -c copy 流复制
	ExportReencode ExportMode = "reencode" // 重编码
)

// Project 一个剪辑工程
type Project struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	WorkDir   string         `json:"workDir"`
	Media     MediaFile      `json:"media"`
	Status    ProjectStatus  `json:"status"`
	Settings  ProjectSettings `json:"settings"`
}

// MediaFile 媒体文件元信息
type MediaFile struct {
	Path       string  `json:"path"`
	DurationMs int64   `json:"durationMs"`
	Format     string  `json:"format"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	Fps        float64 `json:"fps"`
	HasAudio   bool    `json:"hasAudio"`
}

// ProjectSettings 工程级设置
type ProjectSettings struct {
	ExportMode    ExportMode    `json:"exportMode"`
	SilenceMs     int           `json:"silenceMs"`
	FillerDict    []string      `json:"fillerDict"`
	LLMConfig     LLMConfig     `json:"llmConfig"`
	SubtitleStyle SubtitleStyle `json:"subtitleStyle"`
}

// SubtitleStyle 字幕样式
type SubtitleStyle struct {
	FontFamily string `json:"fontFamily"`
	FontSize   int    `json:"fontSize"`
	Color      string `json:"color"`       // hex 如 #FFFFFF
	Highlight  string `json:"highlight"`   // 当前词高亮色
	Position   string `json:"position"`    // bottom/center/top
	BgColor    string `json:"bgColor"`     // 背景色，可透明
	BgOpacity  float64 `json:"bgOpacity"`  // 0-1
}
```

- [ ] **Step 2: 验证编译**

```bash
go build ./internal/model/
```

Expected: 无报错。

- [ ] **Step 3: Commit**

```bash
git add internal/model/project.go
git commit -m "feat(model): add Project, MediaFile, ProjectSettings types"
```

---

### Task 5: 实现 model 层 — transcript.go

**Files:**
- Create: `internal/model/transcript.go`

- [ ] **Step 1: 编写 transcript.go**

创建 `internal/model/transcript.go`，内容如下：

```go
package model

// Transcript 转录结果
type Transcript struct {
	Language string    `json:"language"` // zh/en/...
	Words    []Word    `json:"words"`
	Segments []Segment `json:"segments"`
}

// Word 词级单元（带时间戳）
type Word struct {
	Text       string  `json:"text"`
	StartMs    int64   `json:"startMs"`
	EndMs      int64   `json:"endMs"`
	Confidence float64 `json:"confidence"`
}

// Segment 句级单元
type Segment struct {
	ID      int    `json:"id"`
	StartMs int64  `json:"startMs"`
	EndMs   int64  `json:"endMs"`
	Text    string `json:"text"`
	WordIDs []int  `json:"wordIds"` // 指向 Words 的索引
}
```

- [ ] **Step 2: 验证编译**

```bash
go build ./internal/model/
```

Expected: 无报错。

- [ ] **Step 3: Commit**

```bash
git add internal/model/transcript.go
git commit -m "feat(model): add Transcript, Word, Segment types"
```

---

### Task 6: 实现 model 层 — cutlist.go + 单测

**Files:**
- Create: `internal/model/cutlist.go`
- Create: `internal/model/cutlist_test.go`

- [ ] **Step 1: 编写 cutlist.go**

创建 `internal/model/cutlist.go`，内容如下：

```go
package model

// CutDecision 剪切决定
type CutDecision string

const (
	CutKeep   CutDecision = "keep"
	CutRemove CutDecision = "remove"
)

// CutReason 剪切原因
type CutReason string

const (
	ReasonFiller     CutReason = "filler"
	ReasonSilence    CutReason = "silence"
	ReasonDupOrError CutReason = "dup_or_error"
	ReasonManual     CutReason = "manual"
)

// CutSource 来源（AI 或手动）
type CutSource string

const (
	SourceAI     CutSource = "ai"
	SourceManual CutSource = "manual"
)

// CutSegment 一个剪切时间段
type CutSegment struct {
	ID         string      `json:"id"`
	StartMs    int64       `json:"startMs"`
	EndMs      int64       `json:"endMs"`
	Decision   CutDecision `json:"decision"`
	Reason     CutReason   `json:"reason"`
	Source     CutSource   `json:"source"`
	Confidence float64     `json:"confidence"`
	Note       string      `json:"note"`
}

// CutList 剪切清单（核心数据，贯穿全流程）
type CutList struct {
	ProjectID string       `json:"projectId"`
	Segments  []CutSegment `json:"segments"`
	Version   int          `json:"version"`
}

// KeepSegment 导出时用于 ffmpeg 的保留段（只有起止时间）
type KeepSegment struct {
	StartMs int64 `json:"startMs"`
	EndMs   int64 `json:"endMs"`
}

// KeepSegments 从 CutList 提取所有 keep 段，返回 KeepSegment 列表
func (cl *CutList) KeepSegments() []KeepSegment {
	var result []KeepSegment
	for _, seg := range cl.Segments {
		if seg.Decision == CutKeep {
			result = append(result, KeepSegment{
				StartMs: seg.StartMs,
				EndMs:   seg.EndMs,
			})
		}
	}
	return result
}

// Normalize 规范化 CutList：
// 1. 按 StartMs 升序排序
// 2. 合并相邻同 Decision 的段
// 3. 裁剪重叠段
func (cl *CutList) Normalize() {
	if len(cl.Segments) == 0 {
		return
	}

	// 1. 按 StartMs 排序（插入排序，段数通常不大）
	for i := 1; i < len(cl.Segments); i++ {
		for j := i; j > 0 && cl.Segments[j].StartMs < cl.Segments[j-1].StartMs; j-- {
			cl.Segments[j], cl.Segments[j-1] = cl.Segments[j-1], cl.Segments[j]
		}
	}

	// 2. 裁剪重叠 + 合并相邻同 Decision
	normalized := []CutSegment{cl.Segments[0]}
	for i := 1; i < len(cl.Segments); i++ {
		last := &normalized[len(normalized)-1]
		curr := cl.Segments[i]

		if curr.StartMs < last.EndMs {
			// 重叠：裁剪 curr 的起点到 last 的终点
			curr.StartMs = last.EndMs
			if curr.StartMs >= curr.EndMs {
				continue // 裁剪后无效，跳过
			}
		}

		if curr.Decision == last.Decision && curr.StartMs == last.EndMs {
			// 相邻同 Decision：合并
			last.EndMs = curr.EndMs
		} else {
			normalized = append(normalized, curr)
		}
	}
	cl.Segments = normalized
	cl.Version++
}
```

- [ ] **Step 2: 安装 testify**

```bash
go get github.com/stretchr/testify
```

- [ ] **Step 3: 编写 cutlist_test.go**

创建 `internal/model/cutlist_test.go`，内容如下：

```go
package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeepSegments_OnlyReturnsKeep(t *testing.T) {
	cl := &CutList{
		Segments: []CutSegment{
			{ID: "1", StartMs: 0, EndMs: 1000, Decision: CutKeep},
			{ID: "2", StartMs: 1000, EndMs: 1500, Decision: CutRemove},
			{ID: "3", StartMs: 1500, EndMs: 3000, Decision: CutKeep},
		},
	}

	keeps := cl.KeepSegments()

	assert.Len(t, keeps, 2)
	assert.Equal(t, int64(0), keeps[0].StartMs)
	assert.Equal(t, int64(1000), keeps[0].EndMs)
	assert.Equal(t, int64(1500), keeps[1].StartMs)
	assert.Equal(t, int64(3000), keeps[1].EndMs)
}

func TestKeepSegments_EmptyList(t *testing.T) {
	cl := &CutList{Segments: []CutSegment{}}
	keeps := cl.KeepSegments()
	assert.Empty(t, keeps)
}

func TestNormalize_SortsByStartMs(t *testing.T) {
	cl := &CutList{
		Segments: []CutSegment{
			{ID: "3", StartMs: 2000, EndMs: 3000, Decision: CutKeep},
			{ID: "1", StartMs: 0, EndMs: 1000, Decision: CutKeep},
			{ID: "2", StartMs: 1000, EndMs: 2000, Decision: CutRemove},
		},
	}

	cl.Normalize()

	assert.Equal(t, "1", cl.Segments[0].ID)
	assert.Equal(t, "2", cl.Segments[1].ID)
	assert.Equal(t, "3", cl.Segments[2].ID)
}

func TestNormalize_MergesAdjacentSameDecision(t *testing.T) {
	cl := &CutList{
		Segments: []CutSegment{
			{ID: "1", StartMs: 0, EndMs: 1000, Decision: CutKeep},
			{ID: "2", StartMs: 1000, EndMs: 2000, Decision: CutKeep},
		},
	}

	cl.Normalize()

	assert.Len(t, cl.Segments, 1)
	assert.Equal(t, int64(0), cl.Segments[0].StartMs)
	assert.Equal(t, int64(2000), cl.Segments[0].EndMs)
}

func TestNormalize_TrimsOverlap(t *testing.T) {
	cl := &CutList{
		Segments: []CutSegment{
			{ID: "1", StartMs: 0, EndMs: 1500, Decision: CutKeep},
			{ID: "2", StartMs: 1000, EndMs: 2000, Decision: CutRemove},
		},
	}

	cl.Normalize()

	// 第一段保持 0-1500
	assert.Equal(t, int64(0), cl.Segments[0].StartMs)
	assert.Equal(t, int64(1500), cl.Segments[0].EndMs)
	// 第二段被裁剪为 1500-2000
	assert.Equal(t, int64(1500), cl.Segments[1].StartMs)
	assert.Equal(t, int64(2000), cl.Segments[1].EndMs)
}

func TestNormalize_DropsFullyContainedSegment(t *testing.T) {
	cl := &CutList{
		Segments: []CutSegment{
			{ID: "1", StartMs: 0, EndMs: 3000, Decision: CutKeep},
			{ID: "2", StartMs: 1000, EndMs: 2000, Decision: CutRemove},
		},
	}

	cl.Normalize()

	// 第二段被完全包含，裁剪后无效，应被丢弃
	assert.Len(t, cl.Segments, 1)
	assert.Equal(t, "1", cl.Segments[0].ID)
}

func TestNormalize_IncrementsVersion(t *testing.T) {
	cl := &CutList{
		Segments: []CutSegment{
			{ID: "1", StartMs: 0, EndMs: 1000, Decision: CutKeep},
		},
		Version: 0,
	}

	cl.Normalize()

	assert.Equal(t, 1, cl.Version)
}

func TestNormalize_EmptyList(t *testing.T) {
	cl := &CutList{Segments: []CutSegment{}}
	cl.Normalize()
	assert.Empty(t, cl.Segments)
}
```

- [ ] **Step 4: 运行测试验证通过**

```bash
go test ./internal/model/ -v -run TestKeepSegments
go test ./internal/model/ -v -run TestNormalize
```

Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/model/cutlist.go internal/model/cutlist_test.go go.mod go.sum
git commit -m "feat(model): add CutList with Normalize and KeepSegments + tests"
```

---

### Task 7: 实现 model 层 — llm.go

**Files:**
- Create: `internal/model/llm.go`

- [ ] **Step 1: 编写 llm.go**

创建 `internal/model/llm.go`，内容如下：

```go
package model

// LLMAnalysisRequest 送入 LLM 的请求（只送句级文本+时间，节省 token）
type LLMAnalysisRequest struct {
	Language string       `json:"language"`
	Segments []LLMSegment `json:"segments"`
	Settings ProjectSettings `json:"settings"`
}

// LLMSegment 送入 LLM 的句段简化结构
type LLMSegment struct {
	ID      int    `json:"id"`
	StartMs int64  `json:"startMs"`
	EndMs   int64  `json:"endMs"`
	Text    string `json:"text"`
}

// LLMAnalysisResult LLM 返回的分析结果
type LLMAnalysisResult struct {
	RemoveSegmentIDs []int             `json:"removeSegmentIds"`
	Items            []LLMAnalysisItem `json:"items"`
}

// LLMAnalysisItem 单个分析项
type LLMAnalysisItem struct {
	SegmentID  int       `json:"segmentId"`
	Reason     CutReason `json:"reason"`
	Confidence float64   `json:"confidence"`
	Note       string    `json:"note"`
}
```

- [ ] **Step 2: 验证编译**

```bash
go build ./internal/model/
```

Expected: 无报错。

- [ ] **Step 3: Commit**

```bash
git add internal/model/llm.go
git commit -m "feat(model): add LLM analysis request/response types"
```

---

### Task 8: 实现 model 层 — event.go

**Files:**
- Create: `internal/model/event.go`

- [ ] **Step 1: 编写 event.go**

创建 `internal/model/event.go`，内容如下：

```go
package model

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskRunning TaskStatus = "running"
	TaskDone    TaskStatus = "done"
	TaskError   TaskStatus = "error"
)

// ProgressEvent 进度事件（通过 EventBus/Wails Event 推送到前端）
type ProgressEvent struct {
	TaskID   string      `json:"taskId"`
	Stage    string      `json:"stage"`    // transcribe/analyze/edit/export/subtitle
	Step     string      `json:"step"`     // 当前步骤描述
	Progress float64     `json:"progress"` // 0-1
	Status   TaskStatus  `json:"status"`
	Error    string      `json:"error,omitempty"`
	Payload  interface{} `json:"payload,omitempty"`
}
```

- [ ] **Step 2: 验证编译**

```bash
go build ./internal/model/
```

Expected: 无报错。

- [ ] **Step 3: Commit**

```bash
git add internal/model/event.go
git commit -m "feat(model): add ProgressEvent and TaskStatus types"
```

---

### Task 9: 实现 model 层 — settings.go + export.go

**Files:**
- Create: `internal/model/settings.go`
- Create: `internal/model/export.go`

- [ ] **Step 1: 编写 settings.go**

创建 `internal/model/settings.go`，内容如下：

```go
package model

// GlobalSettings 全局设置（存 ~/.smart-cut/config.json）
type GlobalSettings struct {
	Binaries        map[string]string `json:"binaries"`        // name→path，空则用随包或 PATH
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
```

- [ ] **Step 2: 编写 export.go**

创建 `internal/model/export.go`，内容如下：

```go
package model

// ExportOptions 导出选项
type ExportOptions struct {
	Mode            ExportMode `json:"mode"`            // lossless/reencode
	IncludeSubtitle bool       `json:"includeSubtitle"` // 是否合成字幕
	OutputPath      string    `json:"outputPath"`
}

// EncodeOpts 重编码参数
type EncodeOpts struct {
	VideoCodec string `json:"videoCodec"` // 如 libx264
	AudioCodec string `json:"audioCodec"` // 如 aac
	VideoBitrate string `json:"videoBitrate"` // 如 2M
	Crf         int    `json:"crf"`         // 0-51，越小质量越高，默认 23
	Preset      string `json:"preset"`      // ultrafast...veryslow
}

// SubtitleRenderRequest Remotion 字幕渲染请求
type SubtitleRenderRequest struct {
	Words   []Word   `json:"words"`
	CutList *CutList `json:"cutList"`
	Style   SubtitleStyle `json:"style"`
	OutputDir string `json:"outputDir"`
}
```

- [ ] **Step 3: 验证整个 model 包编译**

```bash
go build ./internal/model/
```

Expected: 无报错。

- [ ] **Step 4: 运行全部 model 测试**

```bash
go test ./internal/model/ -v
```

Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/model/settings.go internal/model/export.go
git commit -m "feat(model): add GlobalSettings, LLMConfig, ExportOptions, SubtitleRenderRequest"
```

---

### Task 10: 改造前端 — package.json 完整依赖

**Files:**
- Modify: `frontend/package.json`

- [ ] **Step 1: 用完整依赖替换 package.json**

用以下内容替换 `frontend/package.json` 全部内容：

```json
{
  "name": "frontend",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build:dev": "vite build --minify false --mode development",
    "build": "vite build --mode production",
    "preview": "vite preview",
    "lint": "eslint src --ext .ts,.tsx",
    "format": "prettier --write src"
  },
  "dependencies": {
    "@wailsio/runtime": "latest",
    "react": "^18.2.0",
    "react-dom": "^18.2.0",
    "react-router-dom": "^6.22.0",
    "zustand": "^4.5.0",
    "lucide-react": "^0.344.0",
    "class-variance-authority": "^0.7.0",
    "clsx": "^2.1.0",
    "tailwind-merge": "^2.2.0",
    "@radix-ui/react-dialog": "^1.0.5",
    "@radix-ui/react-popover": "^1.0.7",
    "@radix-ui/react-tooltip": "^1.0.7",
    "@radix-ui/react-select": "^2.0.0",
    "@radix-ui/react-slider": "^1.1.2",
    "@radix-ui/react-switch": "^1.0.3",
    "@radix-ui/react-tabs": "^1.0.4",
    "@radix-ui/react-toast": "^1.1.5",
    "@radix-ui/react-label": "^2.0.2",
    "@radix-ui/react-slot": "^1.0.2"
  },
  "devDependencies": {
    "@types/react": "^18.2.43",
    "@types/react-dom": "^18.2.17",
    "@vitejs/plugin-react": "^6.0.0",
    "vite": "^8.0.5",
    "typescript": "^5.3.3",
    "tailwindcss": "^3.4.1",
    "postcss": "^8.4.35",
    "autoprefixer": "^10.4.17",
    "eslint": "^8.56.0",
    "@typescript-eslint/eslint-plugin": "^7.0.0",
    "@typescript-eslint/parser": "^7.0.0",
    "eslint-plugin-react-hooks": "^4.6.0",
    "eslint-plugin-react-refresh": "^0.4.5",
    "prettier": "^3.2.5",
    "prettier-plugin-tailwindcss": "^0.5.11",
    "vitest": "^1.3.0",
    "@testing-library/react": "^14.2.0",
    "@testing-library/jest-dom": "^6.4.0",
    "jsdom": "^24.0.0"
  }
}
```

- [ ] **Step 2: 安装依赖**

```bash
cd frontend && npm install
```

Expected: 安装完成无报错。可能有 peer dependency 警告，忽略即可。

- [ ] **Step 3: Commit**

```bash
git add frontend/package.json frontend/package-lock.json
git commit -m "chore(frontend): add TypeScript, Tailwind, Shadcn, zustand, Vitest dependencies"
```

---

### Task 11: 配置 TypeScript + Vite

**Files:**
- Modify: `frontend/tsconfig.json`
- Delete: `frontend/vite.config.js`
- Create: `frontend/vite.config.ts`

- [ ] **Step 1: 改造 tsconfig.json 为严格 TS 配置**

用以下内容替换 `frontend/tsconfig.json` 全部内容：

```json
{
  "compilerOptions": {
    "target": "ESNext",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "allowJs": false,
    "strict": true,
    "skipLibCheck": true,
    "esModuleInterop": true,
    "allowSyntheticDefaultImports": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",
    "baseUrl": ".",
    "paths": {
      "@/*": ["./src/*"]
    }
  },
  "include": ["src", "bindings"]
}
```

- [ ] **Step 2: 创建 vite.config.ts**

删除 `frontend/vite.config.js`，创建 `frontend/vite.config.ts`：

```typescript
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import wails from "@wailsio/runtime/plugins/vite";
import path from "path";

export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  plugins: [react(), wails("./bindings")],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
});
```

- [ ] **Step 3: Commit**

```bash
git add frontend/tsconfig.json frontend/vite.config.ts
git rm frontend/vite.config.js
git commit -m "chore(frontend): configure strict TypeScript and Vite with path alias"
```

---

### Task 12: 配置 Tailwind CSS + 暗色模式

**Files:**
- Create: `frontend/tailwind.config.js`
- Create: `frontend/postcss.config.js`
- Create: `frontend/src/index.css`
- Delete: `frontend/public/style.css`

- [ ] **Step 1: 创建 tailwind.config.js**

创建 `frontend/tailwind.config.js`：

```javascript
/** @type {import('tailwindcss').Config} */
export default {
  darkMode: ["class"],
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        border: "hsl(var(--border))",
        input: "hsl(var(--input))",
        ring: "hsl(var(--ring))",
        background: "hsl(var(--background))",
        foreground: "hsl(var(--foreground))",
        primary: {
          DEFAULT: "hsl(var(--primary))",
          foreground: "hsl(var(--primary-foreground))",
        },
        secondary: {
          DEFAULT: "hsl(var(--secondary))",
          foreground: "hsl(var(--secondary-foreground))",
        },
        destructive: {
          DEFAULT: "hsl(var(--destructive))",
          foreground: "hsl(var(--destructive-foreground))",
        },
        muted: {
          DEFAULT: "hsl(var(--muted))",
          foreground: "hsl(var(--muted-foreground))",
        },
        accent: {
          DEFAULT: "hsl(var(--accent))",
          foreground: "hsl(var(--accent-foreground))",
        },
        card: {
          DEFAULT: "hsl(var(--card))",
          foreground: "hsl(var(--card-foreground))",
        },
      },
      borderRadius: {
        lg: "var(--radius)",
        md: "calc(var(--radius) - 2px)",
        sm: "calc(var(--radius) - 4px)",
      },
    },
  },
  plugins: [],
};
```

- [ ] **Step 2: 创建 postcss.config.js**

创建 `frontend/postcss.config.js`：

```javascript
export default {
  plugins: {
    tailwindcss: {},
    autoprefixer: {},
  },
};
```

- [ ] **Step 3: 创建 src/index.css（含暗色模式 CSS 变量）**

创建 `frontend/src/index.css`：

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

@layer base {
  :root {
    --background: 0 0% 100%;
    --foreground: 240 10% 3.9%;
    --card: 0 0% 100%;
    --card-foreground: 240 10% 3.9%;
    --primary: 240 5.9% 10%;
    --primary-foreground: 0 0% 98%;
    --secondary: 240 4.8% 95.9%;
    --secondary-foreground: 240 5.9% 10%;
    --muted: 240 4.8% 95.9%;
    --muted-foreground: 240 3.8% 46.1%;
    --accent: 240 4.8% 95.9%;
    --accent-foreground: 240 5.9% 10%;
    --destructive: 0 84.2% 60.2%;
    --destructive-foreground: 0 0% 98%;
    --border: 240 5.9% 90%;
    --input: 240 5.9% 90%;
    --ring: 240 5.9% 10%;
    --radius: 0.5rem;
  }

  .dark {
    --background: 240 10% 3.9%;
    --foreground: 0 0% 98%;
    --card: 240 10% 3.9%;
    --card-foreground: 0 0% 98%;
    --primary: 0 0% 98%;
    --primary-foreground: 240 5.9% 10%;
    --secondary: 240 3.7% 15.9%;
    --secondary-foreground: 0 0% 98%;
    --muted: 240 3.7% 15.9%;
    --muted-foreground: 240 5% 64.9%;
    --accent: 240 3.7% 15.9%;
    --accent-foreground: 0 0% 98%;
    --destructive: 0 62.8% 30.6%;
    --destructive-foreground: 0 0% 98%;
    --border: 240 3.7% 15.9%;
    --input: 240 3.7% 15.9%;
    --ring: 240 4.9% 83.9%;
  }
}

@layer base {
  * {
    @apply border-border;
  }
  body {
    @apply bg-background text-foreground;
    margin: 0;
    font-family: Inter, system-ui, sans-serif;
  }
}
```

- [ ] **Step 4: 删除旧的 style.css**

删除文件 `frontend/public/style.css`。

- [ ] **Step 5: Commit**

```bash
git add frontend/tailwind.config.js frontend/postcss.config.js frontend/src/index.css
git rm frontend/public/style.css
git commit -m "chore(frontend): configure Tailwind CSS with dark mode CSS variables"
```

---

### Task 13: JSX → TSX 迁移 + 前端入口改造

**Files:**
- Delete: `frontend/src/App.jsx`
- Delete: `frontend/src/main.jsx`
- Create: `frontend/src/main.tsx`
- Create: `frontend/src/App.tsx`
- Create: `frontend/src/lib/utils.ts`
- Modify: `frontend/index.html`

- [ ] **Step 1: 创建 lib/utils.ts（Shadcn 标配 cn 函数）**

创建 `frontend/src/lib/utils.ts`：

```typescript
import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
```

- [ ] **Step 2: 创建 main.tsx**

删除 `frontend/src/main.jsx`，创建 `frontend/src/main.tsx`：

```typescript
import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import "./index.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
```

- [ ] **Step 3: 创建 App.tsx（空壳）**

删除 `frontend/src/App.jsx`，创建 `frontend/src/App.tsx`：

```typescript
function App() {
  return (
    <div className="flex h-screen w-screen items-center justify-center bg-background text-foreground">
      <div className="text-center">
        <h1 className="text-2xl font-bold">Smart-Cut</h1>
        <p className="mt-2 text-muted-foreground">AI 口播视频自动剪辑工具</p>
      </div>
    </div>
  );
}

export default App;
```

- [ ] **Step 4: 更新 index.html**

将 `frontend/index.html` 中的 `<script src="/src/main.jsx">` 改为 `<script src="/src/main.tsx">`，title 改为 Smart-Cut。

修改后的 `frontend/index.html`：

```html
<!DOCTYPE html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <link rel="icon" type="image/svg+xml" href="/wails.png" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Smart-Cut</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 5: Commit**

```bash
git add frontend/src/main.tsx frontend/src/App.tsx frontend/src/lib/utils.ts frontend/index.html
git rm frontend/src/App.jsx frontend/src/main.jsx
git commit -m "refactor(frontend): migrate JSX to TSX, create clean App shell"
```

---

### Task 14: 建立前端目录结构

**Files:**
- Create: `frontend/src/pages/.gitkeep`
- Create: `frontend/src/components/.gitkeep`
- Create: `frontend/src/stores/.gitkeep`
- Create: `frontend/src/api/.gitkeep`
- Create: `frontend/src/remotion/.gitkeep`

- [ ] **Step 1: 创建目录**

```bash
cd frontend/src && mkdir -p pages components stores api remotion
```

- [ ] **Step 2: 添加 .gitkeep**

```bash
touch pages/.gitkeep components/.gitkeep stores/.gitkeep api/.gitkeep remotion/.gitkeep
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/ frontend/src/components/ frontend/src/stores/ frontend/src/api/ frontend/src/remotion/
git commit -m "chore(frontend): create directory structure for pages, components, stores, api, remotion"
```

---

### Task 15: 配置 ESLint + Prettier

**Files:**
- Create: `frontend/.eslintrc.cjs`
- Create: `frontend/.prettierrc`

- [ ] **Step 1: 创建 .eslintrc.cjs**

创建 `frontend/.eslintrc.cjs`：

```javascript
module.exports = {
  root: true,
  env: { browser: true, es2020: true },
  extends: [
    "eslint:recommended",
    "plugin:@typescript-eslint/recommended",
    "plugin:react-hooks/recommended",
  ],
  ignorePatterns: ["dist", ".eslintrc.cjs", "bindings"],
  parser: "@typescript-eslint/parser",
  plugins: ["react-refresh"],
  rules: {
    "react-refresh/only-export-components": [
      "warn",
      { allowConstantExport: true },
    ],
    "@typescript-eslint/no-unused-vars": ["warn", { argsIgnorePattern: "^_" }],
  },
};
```

- [ ] **Step 2: 创建 .prettierrc**

创建 `frontend/.prettierrc`：

```json
{
  "semi": true,
  "singleQuote": false,
  "tabWidth": 2,
  "trailingComma": "all",
  "printWidth": 100,
  "plugins": ["prettier-plugin-tailwindcss"]
}
```

- [ ] **Step 3: 验证 lint 能跑**

```bash
cd frontend && npx eslint src --ext .ts,.tsx
```

Expected: 无错误（可能有 warning，可接受）。

- [ ] **Step 4: Commit**

```bash
git add frontend/.eslintrc.cjs frontend/.prettierrc
git commit -m "chore(frontend): configure ESLint and Prettier"
```

---

### Task 16: 创建 Shadcn components.json 配置

**Files:**
- Create: `frontend/components.json`

- [ ] **Step 1: 创建 components.json**

创建 `frontend/components.json`（Shadcn/ui 初始化配置，用于后续 `npx shadcn-ui@latest add <component>` 命令）：

```json
{
  "$schema": "https://ui.shadcn.com/schema.json",
  "style": "default",
  "rsc": false,
  "tsx": true,
  "tailwind": {
    "config": "tailwind.config.js",
    "css": "src/index.css",
    "baseColor": "zinc",
    "cssVariables": true
  },
  "aliases": {
    "components": "@/components",
    "utils": "@/lib/utils"
  }
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/components.json
git commit -m "chore(frontend): add Shadcn/ui components.json config"
```

---

### Task 17: 最终验证 — 全量编译 + dev 启动

**Files:**
- 无文件变更，仅验证

- [ ] **Step 1: Go 全量编译 + 测试**

```bash
go build ./...
go test ./internal/model/ -v
```

Expected: 编译无报错，全部测试 PASS。

- [ ] **Step 2: 前端构建验证**

```bash
cd frontend && npm run build
```

Expected: Vite 构建成功，生成 `frontend/dist/` 目录。

- [ ] **Step 3: Wails3 dev 启动验证**

```bash
cd .. && wails3 dev -config ./build/config.yml -port 9245
```

Expected: 弹出桌面窗口，显示 "Smart-Cut" 标题和 "AI 口播视频自动剪辑工具" 副标题，暗色背景。确认后关闭。

- [ ] **Step 4: 最终 commit**

```bash
git add -A
git commit -m "chore: plan 1 complete - scaffold and data model ready"
```

---

## 完成标准

Plan 1 完成后应满足：
1. ✅ `go build ./...` 无报错
2. ✅ `go test ./internal/model/ -v` 全部 PASS
3. ✅ `frontend/` 有完整 node_modules，TypeScript + Tailwind 可用
4. ✅ `wails3 dev` 能启动并显示 Smart-Cut 空壳窗口
5. ✅ 后端 `internal/model/` 包含全部 7 个文件（project/transcript/cutlist/llm/event/settings/export）
6. ✅ 前端目录结构齐全（pages/components/stores/api/remotion/lib）
7. ✅ ESLint + Prettier 配置就绪

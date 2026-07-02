# Smart-Cut 设计文档

- **日期**: 2026-07-02
- **项目**: Smart-Cut —— AI 口播视频自动剪辑工具
- **状态**: 已确认，待写实施计划

## 1. 概述

Smart-Cut 是一个面向口播类视频的桌面端 AI 自动剪辑工具。核心价值是：基于 whisper.cpp 转录 + LLM 分析，自动识别并删除语气词、停顿、重复口误等"废话片段"，并提供可视化时间轴供用户微调，最终通过 ffmpeg 精确拼接导出。

## 2. 技术栈

| 层 | 技术 |
|---|---|
| 桌面框架 | Wails3 (Pre-release) |
| 前端 | React 18 + TypeScript + zustand |
| 后端 | Go |
| 语音识别 | whisper.cpp（本地 exec 调用）|
| 视频处理 | ffmpeg / ffprobe（exec 调用）|
| 视频生成 | Remotion（React 同构，动态字幕）|
| LLM | OpenAI 兼容 HTTP API |

**关键选型理由**：
- 前端选用 React 而非 Vue，目的是与 Remotion 同构，无缝集成 `<Player>` 实时字幕预览。
- whisper.cpp 选本地 exec 方案（非 cgo），避免交叉编译复杂度，且离线、隐私友好。
- LLM 仅用 OpenAI 兼容协议（覆盖 OpenAI、DeepSeek、通义、本地 Ollama/LM Studio 等），降低抽象成本。

## 3. MVP 功能范围

### 3.1 剪切能力
- 去除语气词（嗯/啊/那个 等，可配置词典）
- 去除停顿/沉默（可设阈值，默认 >800ms）
- 去除重复/口误（由 LLM 识别）
- 手动剪切点（用户在时间轴上标记）

### 3.2 AI 策略
- **LLM 为主**：句级文本送 LLM，由 LLM 判断哪些句段应删除（语气词/重复/口误）
- **规则为辅**：沉默检测用规则（基于 whisper 词时间戳的间隔）

### 3.3 导出
- 双模式可选：
  - 无损拼接（`-c copy`，快，关键帧精度）
  - 重编码拼接（帧级精度，慢）

### 3.4 字幕
- Remotion 动态字幕，逐字高亮跟随语音
- 字幕样式可配置

## 4. 架构（方案 A：事件驱动 + 管道式）

### 4.1 分层

```
L1. UI Layer (React + Remotion Player)
L2. API Layer (Go, Wails3 App)
L3. Service Layer (Go, 业务编排)
L4. Pipeline Layer (Go, 阶段化任务调度)
L5. Adapter Layer (Go, 外部依赖封装)
Cross: EventBus + ProgressReporter
```

### 4.2 模块职责

| 模块 | 职责 | 不做什么 |
|---|---|---|
| UI | 展示与交互 | 不直接调 ffmpeg/whisper |
| API Layer | RPC 入口、参数转换 | 不含业务逻辑 |
| Service | 业务流程编排、决策 | 不直接 exec 二进制 |
| Pipeline | 任务分阶段、进度统一 | 不含具体业务判断 |
| Adapter | 与外部世界通信 | 不含业务规则 |
| EventBus | 异步消息推送 | 不含业务 |

**设计原则**：
- 依赖方向自上而下，下层不 import 上层
- Adapter 可替换（接口定义）
- Pipeline 与 Service 解耦：Service 决定"做什么"，Pipeline 决定"怎么推进度"

## 5. 核心数据结构

时间单位统一为**毫秒（int64）**。

### 5.1 项目与媒体

```go
type Project struct {
    ID         string         `json:"id"`
    Name       string         `json:"name"`
    CreatedAt  time.Time      `json:"createdAt"`
    UpdatedAt  time.Time      `json:"updatedAt"`
    WorkDir    string         `json:"workDir"`
    Media      MediaFile      `json:"media"`
    Status     ProjectStatus  `json:"status"` // draft/transcribed/analyzed/exported
    Settings   ProjectSettings `json:"settings"`
}

type MediaFile struct {
    Path       string  `json:"path"`
    DurationMs int64   `json:"durationMs"`
    Format     string  `json:"format"`
    Width      int     `json:"width"`
    Height     int     `json:"height"`
    Fps        float64 `json:"fps"`
    HasAudio   bool    `json:"hasAudio"`
}

type ProjectSettings struct {
    ExportMode    ExportMode    `json:"exportMode"`    // lossless/reencode
    SilenceMs     int           `json:"silenceMs"`     // 默认 800
    FillerDict    []string      `json:"fillerDict"`
    LLMConfig     LLMConfig     `json:"llmConfig"`
    SubtitleStyle SubtitleStyle `json:"subtitleStyle"`
}
```

### 5.2 转录结果

```go
type Transcript struct {
    Language string    `json:"language"`
    Words    []Word    `json:"words"`
    Segments []Segment `json:"segments"`
}

type Word struct {
    Text      string  `json:"text"`
    StartMs   int64   `json:"startMs"`
    EndMs     int64   `json:"endMs"`
    Confidence float64 `json:"confidence"`
}

type Segment struct {
    ID      int    `json:"id"`
    StartMs int64  `json:"startMs"`
    EndMs   int64  `json:"endMs"`
    Text    string `json:"text"`
    WordIDs []int  `json:"wordIds"`
}
```

### 5.3 剪切清单（核心数据）

```go
type CutDecision string
const (
    CutKeep   CutDecision = "keep"
    CutRemove CutDecision = "remove"
)

type CutReason string
const (
    ReasonFiller     CutReason = "filler"
    ReasonSilence    CutReason = "silence"
    ReasonDupOrError CutReason = "dup_or_error"
    ReasonManual     CutReason = "manual"
)

type CutSource string
const (
    SourceAI    CutSource = "ai"
    SourceManual CutSource = "manual"
)

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

type CutList struct {
    ProjectID string       `json:"projectId"`
    Segments  []CutSegment `json:"segments"` // 按 StartMs 升序、不重叠、keep/remove 交替
    Version   int          `json:"version"`
}
```

**约束（EditService 强制保证）**：
- 按 `StartMs` 升序
- 相邻 segment 不重叠（重叠时合并/裁剪）
- `keep` 与 `remove` 交替，导出按 `keep` 段拼接

### 5.4 LLM 分析契约

```go
type LLMAnalysisRequest struct {
    Language string       `json:"language"`
    Segments []LLMSegment `json:"segments"`
    Settings ProjectSettings `json:"settings"`
}

type LLMSegment struct {
    ID      int    `json:"id"`
    StartMs int64  `json:"startMs"`
    EndMs   int64  `json:"endMs"`
    Text    string `json:"text"`
}

type LLMAnalysisResult struct {
    RemoveSegmentIDs []int             `json:"removeSegmentIds"`
    Items            []LLMAnalysisItem `json:"items"`
}

type LLMAnalysisItem struct {
    SegmentID  int       `json:"segmentId"`
    Reason     CutReason `json:"reason"`
    Confidence float64   `json:"confidence"`
    Note       string    `json:"note"`
}
```

LLM 只看句级、不看词级（节省 token）。

### 5.5 进度事件

```go
type ProgressEvent struct {
    TaskID   string      `json:"taskId"`
    Stage    string      `json:"stage"`    // transcribe/analyze/edit/export/subtitle
    Step     string      `json:"step"`
    Progress float64     `json:"progress"` // 0-1
    Status   TaskStatus  `json:"status"`   // running/done/error
    Error    string      `json:"error,omitempty"`
    Payload  interface{} `json:"payload,omitempty"`
}
```

## 6. Pipeline 与 Adapter 接口

### 6.1 Step / Pipeline 契约

```go
type Context struct {
    Project    *model.Project
    Transcript *model.Transcript
    CutList    *model.CutList
    ExportPath string
    Cancel     context.Context
}

type Step interface {
    Name() string
    Run(ctx *Context, reporter ProgressReporter) error
}

type ProgressReporter interface {
    Report(stage, step string, progress float64)
    Error(stage string, err error)
}

type Pipeline struct {
    steps    []Step
    reporter ProgressReporter
}
```

- `Context` 是阶段间数据载体
- 任一 Step 失败 → 中止并推送 Error
- `ctx.Cancel` 支持用户取消

### 6.2 MVP 四个 Step

| Step | 输入 | 输出 | 调用 |
|---|---|---|---|
| TranscribeStep | MediaFile | Transcript | WhisperAdapter |
| AnalyzeStep | Transcript | CutList（AI 版） | LLMAdapter + 规则 |
| SubtitleStep | Transcript + CutList | 字幕 mp4 片段 | RemotionAdapter |
| ExportStep | CutList + 字幕片段 | 最终 mp4 | FFmpegAdapter |

SubtitleStep 由用户开关控制（不要字幕则跳过）。

### 6.3 Adapter 接口

```go
type WhisperAdapter interface {
    Transcribe(ctx context.Context, mediaPath string, opts WhisperOptions) (*model.Transcript, error)
}

type LLMAdapter interface {
    Analyze(ctx context.Context, req model.LLMAnalysisRequest) (*model.LLMAnalysisResult, error)
}

type FFmpegAdapter interface {
    Probe(ctx context.Context, path string) (*model.MediaFile, error)
    ExtractWaveform(ctx context.Context, mediaPath, outPng string) error
    ConcatLossless(ctx context.Context, segments []KeepSegment, outPath string) error
    ConcatReencode(ctx context.Context, segments []KeepSegment, outPath string, opts EncodeOpts) error
    MuxSubtitle(ctx context.Context, videoPath, subtitleClipPath, outPath string) error
}

type RemotionAdapter interface {
    RenderSubtitle(ctx context.Context, req SubtitleRenderRequest) (clipPath string, err error)
}
```

### 6.4 外部二进制实现与进度获取

| Adapter | 实现方式 | 进度获取 |
|---|---|---|
| WhisperAdapter | `exec.Command("whisper-cli", ...)` + 解析 `-ojf` JSON | 解析 stderr 进度百分比 |
| LLMAdapter | `net/http` OpenAI Chat Completions（stream） | 流式 token 计数估算 |
| FFmpegAdapter | `exec.Command("ffmpeg"/"ffprobe", ...)` | 解析 stderr `time=00:01:23` |
| RemotionAdapter | `exec.Command("render-worker", ...)` | 解析 stdout |

### 6.5 BinaryResolver

```go
type BinaryResolver interface {
    Resolve(name string) (path string, err error)
}
```

查找优先级：
1. 用户配置的绝对路径
2. 随包 `resources/bin/<name>(.exe)`
3. 系统 PATH

### 6.6 错误处理约定

- Adapter 返回包装错误：`fmt.Errorf("whisper transcribe: %w", err)`，保留原始 stderr
- Adapter 不重试（MVP 暂不实现，未来在 Step 层用装饰器统一加）
- 用户可见错误 → EventBus 推 `Status=error`，前端 toast

## 7. UI 与 API

### 7.0 前端 UI 技术栈（方案 D）

采用 **Shadcn/ui + Tailwind CSS + Radix Primitives** 组合：

```
React 18 + TypeScript
├── Shadcn/ui          # 复制源码到本地的组件集合（基于 Radix 封装 + Tailwind 样式）
├── Tailwind CSS 3     # 原子样式
├── Radix Primitives   # 底层无障碍交互组件（Dialog/Popover/Tooltip 等共 40+）
├── lucide-react       # 图标库（Shadcn 默认搭配）
├── zustand            # 状态管理
└── class-variance-authority + clsx + tailwind-merge  # Shadcn 样式拼接标配
```

**选型理由**：
- 无运行时体积（Shadcn 是复制源码模式，不像 Antd/MUI 需打包整个库）
- 完全可定制（组件源码在项目内，可自由改）
- 原生暗色模式支持（CSS 变量切换，剪辑工具必备）
- 现代审美（参考 Linear / Descript / CapCut 桌面版风格）
- Radix 提供无障碍交互层，Timeline 等核心自定义组件也可复用其 Primitive

**适用边界**：
- Shadcn/Tailwind 负责：Button/Input/Dialog/Popover/Sidebar/Settings 表单等通用 UI
- **不依赖**任何 UI 库：Timeline、波形轨、剪切轨、播放头等核心交互组件全部基于 `<canvas>` + React 自绘

### 7.1 前端页面结构

```
App
├── /project/new          新建/导入页
├── /project/:id          工作台
│   ├── TopBar            项目名、导出按钮、全局进度条
│   ├── Sidebar           项目设置
│   ├── MainArea
│   │   ├── VideoPreview  video 标签 + Remotion <Player> 叠加字幕
│   │   └── Timeline      时间轴（核心交互）
│   └── RightPanel        AI 建议列表（逐条接受/拒绝）
└── /settings             全局设置（二进制路径、默认 LLM）
```

### 7.2 Timeline（三轨道）

```
┌──────────────────────────────────────────────┐
│ 波形轨     ▁▂▃█▆▃▂▁▂▃█▇▄▂▁                    │
├──────────────────────────────────────────────┤
│ 字幕轨     [Hello world]  [今天聊聊...]       │
├──────────────────────────────────────────────┤
│ 剪切轨     ■keep  □remove  ■keep  □remove    │
└──────────────────────────────────────────────┘
       ▲ 播放头（拖拽定位）
       缩放 [+] [-] [适应]
```

交互：
- 点击/拖拽波形或字幕 → 视频跳转
- 剪切轨：拖拽边界微调时间、右键切换 keep/remove、双击新建手动 segment
- Ctrl + 滚轮缩放
- 选中 segment 时视频 loop 播放该段

实现：波形用 `<canvas>` 自绘（数据来自 FFmpegAdapter.ExtractWaveform）；播放头用 `useSyncExternalStore`。

### 7.3 Remotion 字幕预览

```tsx
const SubtitleComp = ({ words, cutList, style }) => {
  const frame = useCurrentFrame();
  const timeMs = (frame / FPS) * 1000;
  const activeWord = findActiveWord(words, cutList, timeMs);
  return <SubtitleText active={activeWord} style={style} />;
};

<RemotionPlayer component={SubtitleComp} durationInFrames={...} fps={30} />;
```

### 7.4 API Binding（前后端契约）

```go
type App struct {
    ctx context.Context
    projectService    *service.ProjectService
    transcribeService *service.TranscribeService
    analyzeService    *service.AnalyzeService
    editService       *service.EditService
    exportService     *service.ExportService
}

// 项目管理（同步）
func (a *App) CreateProject(req CreateProjectRequest) (*Project, error)
func (a *App) OpenProject(id string) (*Project, error)
func (a *App) SaveProject(p Project) error

// 主流程（异步，立即返回 taskID，结果走 Event）
func (a *App) StartTranscribe(projectID string) (taskID string, err error)
func (a *App) StartAnalyze(projectID string) (taskID string, err error)
func (a *App) StartExport(projectID string, opts ExportOptions) (taskID string, err error)

// CutList 编辑（同步）
func (a *App) UpdateCutSegment(projectID string, seg CutSegment) error
func (a *App) AddCutSegment(projectID string, seg CutSegment) error
func (a *App) RemoveCutSegment(projectID string, segID string) error
func (a *App) GetCutList(projectID string) (*CutList, error)

// 查询
func (a *App) GetTranscript(projectID string) (*Transcript, error)
func (a *App) GetWaveform(projectID string) (string, error)

// 设置
func (a *App) GetSettings() (*GlobalSettings, error)
func (a *App) SaveSettings(s GlobalSettings) error
func (a *App) ProbeBinary(name string) (path string, version string, err error)

// 取消
func (a *App) CancelTask(taskID string) error
```

### 7.5 Event 通道（后端 → 前端）

```go
app.Events.Emit(ctx, "progress", progressEvent)
app.Events.Emit(ctx, "transcript:ready", transcript)
app.Events.Emit(ctx, "cutlist:ready", cutList)
app.Events.Emit(ctx, "export:done", exportPath)
app.Events.Emit(ctx, "log", logLine)
```

前端 `Events.On("progress", cb)` 订阅。

### 7.6 异步约定

所有 `Start*` 方法立即返回 taskID，不阻塞 binding（避免 WebView 卡顿）。结果与进度全部走 Event。

## 8. 错误处理

### 8.1 错误分类

| 错误类型 | 来源 | 处理 |
|---|---|---|
| 环境错误 | 二进制缺失/版本不符 | 启动 ProbeBinary 检测 + 引导设置 |
| 参数错误 | 格式不支持、文件不可读 | API 层校验，同步返回 |
| 转录错误 | whisper 崩溃、模型缺失 | Step 包装 stderr，Event 推 error |
| LLM 错误 | 401/429/超时 | Adapter 识别状态码，前端重试按钮 |
| 导出错误 | 磁盘满、关键帧对齐失败 | 中止 Pipeline + 清理临时文件 |
| 取消 | 用户主动 | ctx.Cancel + 清理临时文件 |

### 8.2 统一错误类型

```go
type AppError struct {
    Code    ErrCode `json:"code"`    // env/param/transcribe/llm/export/canceled
    Message string  `json:"message"` // 用户友好描述
    Detail  string  `json:"detail"`  // 原始 stderr/api resp（折叠）
}
```

### 8.3 临时文件

```
<WorkDir>/<projectID>/
├── source.mp4
├── whisper.json
├── waveform.png
├── cuts/
│   ├── keep_001.mp4
│   └── ...
├── subtitle_clip.mp4
└── export.mp4
```

- 任务失败/取消时 `defer cleanupCuts()` 清理 `cuts/`
- `source`/`whisper.json`/`waveform.png` 保留（重试复用）
- 提供"清理工程缓存"按钮

## 9. 测试策略

金字塔型，越下层覆盖率越高。

| 层 | 策略 | 工具 |
|---|---|---|
| Adapter | 重点。预录 stderr/JSON fixture | Go testify + fixture |
| Step | Mock Adapter，验证编排/进度/Context | testify + gomock |
| Service | 业务规则（CutList 合并、沉默阈值）纯函数 | testify |
| API Layer | 轻量集成 | - |
| 前端 | Timeline 交互、CutList 编辑 | vitest + RTL |
| E2E | MVP 暂不做 | - |

关键单测点：
- `CutList.Normalize()`
- `SilenceDetector`
- `WhisperAdapter.parseJSON()`
- `FFmpegAdapter.parseProgress()`

测试数据：随包 30 秒口播样本视频（含语气词/停顿）作为黄金用例。

## 10. 打包发布

### 10.1 随包目录

```
smart-cut(.exe)
resources/
├── bin/
│   ├── ffmpeg(.exe)
│   ├── ffprobe(.exe)
│   └── whisper-cli(.exe)
├── models/
│   └── ggml-base.bin          # 默认模型（~140MB）
└── remotion/
    └── render-worker(.exe)    # pkg 预编译的 Node 渲染脚本
```

### 10.2 Remotion 打包策略

用 `pkg` 把 Remotion 渲染脚本预编译为单二进制 `render-worker`，用户无需安装 Node。代价：包体积 +80~120MB。

### 10.3 Wails3 打包

```bash
wails3 build -platform windows/amd64
wails3 build -platform darwin/universal
```

### 10.4 Whisper 模型

默认随包带 `ggml-base.bin`，设置页可下载 small/medium/large。

## 11. 配置

```go
type GlobalSettings struct {
    Binaries        map[string]string `json:"binaries"`  // name→path
    WhisperModelDir string            `json:"whisperModelDir"`
    DefaultLLM      LLMConfig         `json:"defaultLLM"`
    Theme           string            `json:"theme"`
}

type LLMConfig struct {
    BaseURL string `json:"baseUrl"`
    APIKey  string `json:"apiKey"`   // 明文存（桌面单用户应用）
    Model   string `json:"model"`
}
```

- 配置存 `~/.smart-cut/config.json`
- API Key 明文存储（桌面单用户应用可接受，不做加密避免过度设计）
- 所有处理本地完成，LLM 仅接收文本，隐私可控

## 12. 目录结构

```
smart-cut/
├── main.go
├── app/                          # API Layer
├── internal/
│   ├── model/                    # 数据结构
│   ├── service/                  # 业务编排
│   ├── pipeline/                 # Step + Pipeline
│   ├── adapter/                  # Whisper/FFmpeg/LLM/Remotion
│   ├── eventbus/                 # 进度/事件
│   └── config/                   # 配置
├── frontend/
│   ├── src/
│   │   ├── pages/
│   │   ├── components/           # Timeline/VideoPreview/RemotionPlayer
│   │   ├── stores/               # zustand
│   │   ├── remotion/             # 字幕 Composition
│   │   └── api/
│   └── package.json
├── resources/                    # 随包资源
└── testdata/                     # 黄金样本
```

## 13. 已知风险

| 风险 | 严重度 | 对策 |
|---|---|---|
| Wails3 Pre-release 不稳定 | 高 | 锁定版本/commit，关键 API 写适配层 |
| Remotion 渲染耗时 | 中 | 片段短（几十秒），预览用 Player 实时 |
| whisper.cpp 跨平台打包 | 中 | 模型路径处理、CPU/GPU 后端 |
| 精确帧剪切 vs 流复制 | 中 | 双模式可选 |
| 大文件/长视频性能 | 中 | 波形缓存、预览异步 |
| Remotion 包体积大 | 中 | pkg 预编译，接受 +100MB |

## 14. 未来扩展（非 MVP）

- LLM 多 Provider 抽象层（Anthropic/Google）
- faster-whisper / GPU 加速可选
- 片头片尾模板（Remotion Composition）
- 批量处理多个视频
- E2E 测试（待 Wails3 工具成熟）

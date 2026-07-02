# Smart-Cut Plan 3: Pipeline + Service 层

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现 Pipeline（阶段化任务调度 + 进度推送）和 Service（业务编排层），包括沉默检测规则、LLM 分析结果合并、CutList 生成与管理。

**Architecture:** Pipeline 串联多个 Step，通过 ProgressReporter 推送进度事件。Service 层调用 Adapter 完成业务流程（转录→分析→导出），不含外部二进制调用逻辑。EventBus 封装 Wails 事件推送。

**Tech Stack:** Go, context, sync, testify, gomock（可选）

---

## File Structure

```
smart-cut/
├── internal/
│   ├── eventbus/
│   │   ├── eventbus.go              # EventBus 封装
│   │   └── eventbus_test.go         # EventBus 测试
│   ├── pipeline/
│   │   ├── pipeline.go              # Pipeline + Step 接口 + Context
│   │   ├── reporter.go              # ProgressReporter 实现
│   │   ├── steps.go                 # 4 个 Step 实现
│   │   ├── pipeline_test.go         # Pipeline 测试
│   │   └── steps_test.go            # Step 测试（用 mock adapter）
│   └── service/
│       ├── project.go               # ProjectService（项目生命周期）
│       ├── transcribe.go            # TranscribeService（转录编排）
│       ├── analyze.go               # AnalyzeService（分析编排 + 规则）
│       ├── edit.go                  # EditService（CutList 管理）
│       ├── export.go                # ExportService（导出编排）
│       ├── analyze_test.go          # 分析逻辑测试
│       └── edit_test.go             # CutList 管理测试
```

---

### Task 1: EventBus — 事件推送封装

**Files:**
- Create: `internal/eventbus/eventbus.go`
- Create: `internal/eventbus/eventbus_test.go`

- [ ] **Step 1: 编写 eventbus.go**

创建 `internal/eventbus/eventbus.go`：

```go
package eventbus

import (
	"sync"

	"smart-cut/internal/model"
)

// EventBus 封装事件推送能力
// 在 Wails 环境下，EmitFunc 调用 app.Events.Emit
// 在测试环境下，可注入 mock
type EventBus struct {
	mu       sync.RWMutex
	emitFunc func(eventName string, data interface{})
}

// NewEventBus 创建 EventBus
// emitFunc: 实际推送函数（Wails 里是 app.Events.Emit 的包装）
func NewEventBus(emitFunc func(string, interface{})) *EventBus {
	return &EventBus{emitFunc: emitFunc}
}

// EmitProgress 推送进度事件
func (b *EventBus) EmitProgress(event model.ProgressEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.emitFunc != nil {
		b.emitFunc("progress", event)
	}
}

// Emit 推送任意事件
func (b *EventBus) Emit(eventName string, data interface{}) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.emitFunc != nil {
		b.emitFunc(eventName, data)
	}
}

// SetEmitFunc 替换推送函数（用于运行时注入 Wails app）
func (b *EventBus) SetEmitFunc(emitFunc func(string, interface{})) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.emitFunc = emitFunc
}
```

- [ ] **Step 2: 编写 eventbus_test.go**

创建 `internal/eventbus/eventbus_test.go`：

```go
package eventbus

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"smart-cut/internal/model"
)

func TestEventBus_EmitProgress(t *testing.T) {
	var mu sync.Mutex
	var received model.ProgressEvent

	bus := NewEventBus(func(name string, data interface{}) {
		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, "progress", name)
		received = data.(model.ProgressEvent)
	})

	event := model.ProgressEvent{
		TaskID:   "task-1",
		Stage:    "transcribe",
		Progress: 0.5,
		Status:   model.TaskRunning,
	}

	bus.EmitProgress(event)

	assert.Equal(t, "task-1", received.TaskID)
	assert.Equal(t, "transcribe", received.Stage)
	assert.Equal(t, 0.5, received.Progress)
}

func TestEventBus_Emit(t *testing.T) {
	var receivedName string
	var receivedData interface{}

	bus := NewEventBus(func(name string, data interface{}) {
		receivedName = name
		receivedData = data
	})

	bus.Emit("custom-event", "hello")

	assert.Equal(t, "custom-event", receivedName)
	assert.Equal(t, "hello", receivedData)
}

func TestEventBus_NilEmitFunc(t *testing.T) {
	// 不注入 emitFunc，不应 panic
	bus := NewEventBus(nil)
	bus.EmitProgress(model.ProgressEvent{TaskID: "test"})
	bus.Emit("anything", nil)
}

func TestEventBus_SetEmitFunc(t *testing.T) {
	bus := NewEventBus(nil)

	var received bool
	bus.SetEmitFunc(func(name string, data interface{}) {
		received = true
	})

	bus.Emit("test", nil)
	assert.True(t, received)
}
```

- [ ] **Step 3: 运行测试**

```bash
go test ./internal/eventbus/ -v
```

Expected: 全部 PASS。

- [ ] **Step 4: Commit**

```bash
git add internal/eventbus/
git commit -m "feat(eventbus): add EventBus with thread-safe emit and runtime injection"
```

---

### Task 2: Pipeline — Step 接口与 Pipeline 调度

**Files:**
- Create: `internal/pipeline/pipeline.go`
- Create: `internal/pipeline/reporter.go`

- [ ] **Step 1: 编写 pipeline.go**

创建 `internal/pipeline/pipeline.go`：

```go
package pipeline

import (
	"context"
	"fmt"

	"smart-cut/internal/model"
)

// Context 在 Pipeline 各 Step 间共享数据
type Context struct {
	Project    *model.Project
	Transcript *model.Transcript
	CutList    *model.CutList
	ExportPath string
	Cancel     context.Context
}

// Step 定义一个处理阶段
type Step interface {
	Name() string
	Run(ctx *Context, reporter ProgressReporter) error
}

// ProgressReporter 进度推送接口
type ProgressReporter interface {
	Report(stage, step string, progress float64)
	Error(stage string, err error)
	Done(stage string, payload interface{})
}

// Pipeline 串联多个 Step
type Pipeline struct {
	steps    []Step
	reporter ProgressReporter
}

// NewPipeline 创建 Pipeline
func NewPipeline(reporter ProgressReporter) *Pipeline {
	return &Pipeline{reporter: reporter}
}

// AddStep 添加一个 Step
func (p *Pipeline) AddStep(step Step) {
	p.steps = append(p.steps, step)
}

// Execute 按顺序执行所有 Step
func (p *Pipeline) Execute(ctx *Context) error {
	total := len(p.steps)
	for i, step := range p.steps {
		select {
		case <-ctx.Cancel.Done():
			return ctx.Cancel.Err()
		default:
		}

		// 推送阶段开始
		if p.reporter != nil {
			p.reporter.Report(step.Name(), fmt.Sprintf("Step %d/%d: %s", i+1, total, step.Name()), float64(i)/float64(total))
		}

		if err := step.Run(ctx, p.reporter); err != nil {
			if p.reporter != nil {
				p.reporter.Error(step.Name(), err)
			}
			return fmt.Errorf("step %s: %w", step.Name(), err)
		}
	}

	// 全部完成
	if p.reporter != nil {
		p.reporter.Done("pipeline", nil)
	}
	return nil
}
```

- [ ] **Step 2: 编写 reporter.go**

创建 `internal/pipeline/reporter.go`：

```go
package pipeline

import (
	"smart-cut/internal/eventbus"
	"smart-cut/internal/model"
)

// eventBusReporter 是 ProgressReporter 的实现，通过 EventBus 推送进度
type eventBusReporter struct {
	bus    *eventbus.EventBus
	taskID string
}

// NewEventBusReporter 创建基于 EventBus 的 ProgressReporter
func NewEventBusReporter(bus *eventbus.EventBus, taskID string) ProgressReporter {
	return &eventBusReporter{bus: bus, taskID: taskID}
}

func (r *eventBusReporter) Report(stage, step string, progress float64) {
	r.bus.EmitProgress(model.ProgressEvent{
		TaskID:   r.taskID,
		Stage:    stage,
		Step:     step,
		Progress: progress,
		Status:   model.TaskRunning,
	})
}

func (r *eventBusReporter) Error(stage string, err error) {
	r.bus.EmitProgress(model.ProgressEvent{
		TaskID: r.taskID,
		Stage:  stage,
		Status: model.TaskError,
		Error:  err.Error(),
	})
}

func (r *eventBusReporter) Done(stage string, payload interface{}) {
	r.bus.EmitProgress(model.ProgressEvent{
		TaskID:  r.taskID,
		Stage:   stage,
		Status:  model.TaskDone,
		Payload: payload,
	})
}

// noopReporter 是 ProgressReporter 的空实现（用于测试）
type noopReporter struct{}

// NewNoopReporter 创建空 ProgressReporter
func NewNoopReporter() ProgressReporter {
	return &noopReporter{}
}

func (r *noopReporter) Report(stage, step string, progress float64) {}
func (r *noopReporter) Error(stage string, err error)               {}
func (r *noopReporter) Done(stage string, payload interface{})      {}
```

- [ ] **Step 3: 验证编译**

```bash
go build ./internal/pipeline/
```

Expected: 无报错。

- [ ] **Step 4: Commit**

```bash
git add internal/pipeline/
git commit -m "feat(pipeline): add Pipeline, Step interface, and ProgressReporter"
```

---

### Task 3: Pipeline — 单元测试

**Files:**
- Create: `internal/pipeline/pipeline_test.go`

- [ ] **Step 1: 编写 pipeline_test.go**

创建 `internal/pipeline/pipeline_test.go`：

```go
package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStep 用于测试的 Step 实现
type mockStep struct {
	name string
	fn   func(ctx *Context, reporter ProgressReporter) error
}

func (s *mockStep) Name() string { return s.name }
func (s *mockStep) Run(ctx *Context, reporter ProgressReporter) error {
	if s.fn != nil {
		return s.fn(ctx, reporter)
	}
	return nil
}

// mockReporter 用于测试的 ProgressReporter
type mockReporter struct {
	reports []reportEntry
	errors  []errorEntry
	dones   []doneEntry
}

type reportEntry struct {
	stage, step string
	progress    float64
}
type errorEntry struct {
	stage string
	err   error
}
type doneEntry struct {
	stage   string
	payload interface{}
}

func (r *mockReporter) Report(stage, step string, progress float64) {
	r.reports = append(r.reports, reportEntry{stage, step, progress})
}
func (r *mockReporter) Error(stage string, err error) {
	r.errors = append(r.errors, errorEntry{stage, err})
}
func (r *mockReporter) Done(stage string, payload interface{}) {
	r.dones = append(r.dones, doneEntry{stage, payload})
}

func TestPipeline_Execute_AllStepsSucceed(t *testing.T) {
	reporter := &mockReporter{}
	p := NewPipeline(reporter)

	executed := []string{}
	p.AddStep(&mockStep{name: "step1", fn: func(ctx *Context, r ProgressReporter) error {
		executed = append(executed, "step1")
		return nil
	}})
	p.AddStep(&mockStep{name: "step2", fn: func(ctx *Context, r ProgressReporter) error {
		executed = append(executed, "step2")
		return nil
	}})

	ctx := &Context{Cancel: context.Background()}
	err := p.Execute(ctx)

	require.NoError(t, err)
	assert.Equal(t, []string{"step1", "step2"}, executed)
	assert.Len(t, reporter.reports, 2)
	assert.Len(t, reporter.dones, 1)
	assert.Equal(t, "pipeline", reporter.dones[0].stage)
	assert.Empty(t, reporter.errors)
}

func TestPipeline_Execute_StepFails(t *testing.T) {
	reporter := &mockReporter{}
	p := NewPipeline(reporter)

	stepErr := errors.New("boom")
	p.AddStep(&mockStep{name: "failing", fn: func(ctx *Context, r ProgressReporter) error {
		return stepErr
	}})
	p.AddStep(&mockStep{name: "never-run", fn: func(ctx *Context, r ProgressReporter) error {
		t.Fatal("should not run after failure")
		return nil
	}})

	ctx := &Context{Cancel: context.Background()}
	err := p.Execute(ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failing")
	assert.Contains(t, err.Error(), "boom")
	assert.Len(t, reporter.errors, 1)
	assert.Equal(t, "failing", reporter.errors[0].stage)
}

func TestPipeline_Execute_Cancelled(t *testing.T) {
	reporter := &mockReporter{}
	p := NewPipeline(reporter)

	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	p.AddStep(&mockStep{name: "step1", fn: func(ctx *Context, r ProgressReporter) error {
		t.Fatal("should not run when cancelled")
		return nil
	}})

	ctx := &Context{Cancel: cancelCtx}
	err := p.Execute(ctx)

	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestPipeline_Execute_EmptyPipeline(t *testing.T) {
	reporter := &mockReporter{}
	p := NewPipeline(reporter)

	ctx := &Context{Cancel: context.Background()}
	err := p.Execute(ctx)

	require.NoError(t, err)
	assert.Len(t, reporter.dones, 1)
}

func TestPipeline_ProgressCalculation(t *testing.T) {
	reporter := &mockReporter{}
	p := NewPipeline(reporter)

	p.AddStep(&mockStep{name: "a"})
	p.AddStep(&mockStep{name: "b"})
	p.AddStep(&mockStep{name: "c"})

	ctx := &Context{Cancel: context.Background()}
	_ = p.Execute(ctx)

	// 3 steps: progress at step 0 = 0/3 = 0, step 1 = 1/3, step 2 = 2/3
	assert.Len(t, reporter.reports, 3)
	assert.Equal(t, 0.0, reporter.reports[0].progress)
	assert.InDelta(t, 0.333, reporter.reports[1].progress, 0.01)
	assert.InDelta(t, 0.666, reporter.reports[2].progress, 0.01)
}

func TestNoopReporter_DoesNotPanic(t *testing.T) {
	r := NewNoopReporter()
	r.Report("a", "b", 0.5)
	r.Error("a", errors.New("test"))
	r.Done("a", "payload")
}
```

- [ ] **Step 2: 运行测试**

```bash
go test ./internal/pipeline/ -v
```

Expected: 全部 PASS。

- [ ] **Step 3: Commit**

```bash
git add internal/pipeline/pipeline_test.go
git commit -m "test(pipeline): add Pipeline execution and cancellation tests"
```

---

### Task 4: Steps — 4 个处理阶段实现

**Files:**
- Create: `internal/pipeline/steps.go`

- [ ] **Step 1: 编写 steps.go**

创建 `internal/pipeline/steps.go`：

```go
package pipeline

import (
	"context"
	"fmt"
	"path/filepath"

	"smart-cut/internal/adapter"
	"smart-cut/internal/model"
)

// —— TranscribeStep: 调用 Whisper 转录 ——

// TranscribeStep 转录阶段
type TranscribeStep struct {
	whisper adapter.WhisperAdapter
	ffmpeg  adapter.FFmpegAdapter
	opts    adapter.WhisperOptions
}

// NewTranscribeStep 创建转录 Step
func NewTranscribeStep(whisper adapter.WhisperAdapter, ffmpeg adapter.FFmpegAdapter, opts adapter.WhisperOptions) *TranscribeStep {
	return &TranscribeStep{whisper: whisper, ffmpeg: ffmpeg, opts: opts}
}

func (s *TranscribeStep) Name() string { return "transcribe" }

func (s *TranscribeStep) Run(ctx *Context, reporter ProgressReporter) error {
	reporter.Report("transcribe", "preparing audio", 0.1)

	// 1. 用 ffmpeg 将原视频转为 16kHz 单声道 wav（whisper.cpp 要求）
	wavPath := filepath.Join(ctx.Project.WorkDir, "audio_16k.wav")
	// 这里用 ffmpeg 的 ExtractWaveform 不合适，需要单独的转码方法
	// MVP 阶段：假设输入已是 wav 或 whisper 能直接处理
	// 后续可在 FFmpegAdapter 加 ConvertToWav 方法
	// 暂时直接把原视频路径喂给 whisper
	mediaPath := ctx.Project.Media.Path
	_ = wavPath

	reporter.Report("transcribe", "running whisper", 0.3)

	// 2. 调用 Whisper 转录
	transcript, err := s.whisper.Transcribe(ctx.Cancel, mediaPath, s.opts)
	if err != nil {
		return fmt.Errorf("transcribe: %w", err)
	}

	ctx.Transcript = transcript

	reporter.Report("transcribe", "completed", 1.0)
	reporter.Done("transcribe", transcript)

	return nil
}

// —— AnalyzeStep: 调用 LLM 分析 + 规则合并 ——

// AnalyzeStep 分析阶段
type AnalyzeStep struct {
	llm     adapter.LLMAdapter
	llmCfg  model.LLMConfig
	rules   *SilenceDetector
}

// NewAnalyzeStep 创建分析 Step
func NewAnalyzeStep(llm adapter.LLMAdapter, llmCfg model.LLMConfig, silenceMs int) *AnalyzeStep {
	return &AnalyzeStep{
		llm:    llm,
		llmCfg: llmCfg,
		rules:  NewSilenceDetector(silenceMs),
	}
}

func (s *AnalyzeStep) Name() string { return "analyze" }

func (s *AnalyzeStep) Run(ctx *Context, reporter ProgressReporter) error {
	if ctx.Transcript == nil {
		return fmt.Errorf("analyze: transcript is nil")
	}

	reporter.Report("analyze", "detecting silence", 0.2)

	// 1. 规则检测：沉默段
	ruleCuts := s.rules.Detect(ctx.Transcript)

	reporter.Report("analyze", "calling LLM", 0.4)

	// 2. LLM 分析
	llmReq := model.LLMAnalysisRequest{
		Language: ctx.Transcript.Language,
	}
	for _, seg := range ctx.Transcript.Segments {
		llmReq.Segments = append(llmReq.Segments, model.LLMSegment{
			ID:      seg.ID,
			StartMs: seg.StartMs,
			EndMs:   seg.EndMs,
			Text:    seg.Text,
		})
	}

	llmResult, err := s.llm.Analyze(ctx.Cancel, llmReq, s.llmCfg)
	if err != nil {
		// LLM 失败不阻塞，只用规则结果
		reporter.Report("analyze", fmt.Sprintf("LLM failed, using rules only: %v", err), 0.6)
		llmResult = &model.LLMAnalysisResult{}
	}

	reporter.Report("analyze", "merging results", 0.8)

	// 3. 合并规则结果 + LLM 结果
	cutList := mergeAnalysisResults(ctx.Transcript, ruleCuts, llmResult)
	cutList.ProjectID = ctx.Project.ID
	cutList.Normalize()

	ctx.CutList = cutList

	reporter.Report("analyze", "completed", 1.0)
	reporter.Done("analyze", cutList)

	return nil
}

// —— ExportStep: 调用 ffmpeg 导出 ——

// ExportStep 导出阶段
type ExportStep struct {
	ffmpeg     adapter.FFmpegAdapter
	exportOpts model.ExportOptions
}

// NewExportStep 创建导出 Step
func NewExportStep(ffmpeg adapter.FFmpegAdapter, opts model.ExportOptions) *ExportStep {
	return &ExportStep{ffmpeg: ffmpeg, exportOpts: opts}
}

func (s *ExportStep) Name() string { return "export" }

func (s *ExportStep) Run(ctx *Context, reporter ProgressReporter) error {
	if ctx.CutList == nil {
		return fmt.Errorf("export: cutlist is nil")
	}

	reporter.Report("export", "preparing segments", 0.1)

	keepSegments := ctx.CutList.KeepSegments()
	if len(keepSegments) == 0 {
		return fmt.Errorf("export: no keep segments")
	}

	reporter.Report("export", "concatenating video", 0.3)

	sourcePath := ctx.Project.Media.Path
	outPath := s.exportOpts.OutputPath
	if outPath == "" {
		outPath = filepath.Join(ctx.Project.WorkDir, "export.mp4")
	}

	var err error
	if s.exportOpts.Mode == model.ExportLossless {
		err = s.ffmpeg.ConcatLossless(ctx.Cancel, keepSegments, sourcePath, outPath)
	} else {
		err = s.ffmpeg.ConcatReencode(ctx.Cancel, keepSegments, sourcePath, outPath, model.EncodeOpts{
			VideoCodec: "libx264",
			AudioCodec: "aac",
			Crf:        23,
			Preset:     "medium",
		})
	}
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}

	ctx.ExportPath = outPath

	reporter.Report("export", "completed", 1.0)
	reporter.Done("export", outPath)

	return nil
}

// —— SubtitleStep: 调用 Remotion 渲染字幕（MVP 占位）——

// SubtitleStep 字幕渲染阶段
// MVP 阶段暂不实现 Remotion 集成，此 Step 为占位
type SubtitleStep struct{}

// NewSubtitleStep 创建字幕 Step
func NewSubtitleStep() *SubtitleStep {
	return &SubtitleStep{}
}

func (s *SubtitleStep) Name() string { return "subtitle" }

func (s *SubtitleStep) Run(ctx *Context, reporter ProgressReporter) error {
	// MVP: 跳过字幕渲染
	reporter.Report("subtitle", "skipped (MVP)", 1.0)
	return nil
}
```

- [ ] **Step 2: 验证编译**

```bash
go build ./internal/pipeline/
```

Expected: 无报错。

- [ ] **Step 3: Commit**

```bash
git add internal/pipeline/steps.go
git commit -m "feat(pipeline): add TranscribeStep, AnalyzeStep, ExportStep, SubtitleStep"
```

---

### Task 5: SilenceDetector + 结果合并逻辑

**Files:**
- Create: `internal/pipeline/silence.go`

- [ ] **Step 1: 编写 silence.go**

创建 `internal/pipeline/silence.go`：

```go
package pipeline

import (
	"fmt"

	"smart-cut/internal/model"
)

// SilenceDetector 检测沉默/停顿段
type SilenceDetector struct {
	thresholdMs int64 // 沉默阈值（毫秒）
}

// NewSilenceDetector 创建沉默检测器
func NewSilenceDetector(thresholdMs int) *SilenceDetector {
	if thresholdMs <= 0 {
		thresholdMs = 800 // 默认 800ms
	}
	return &SilenceDetector{thresholdMs: int64(thresholdMs)}
}

// Detect 检测转录结果中的沉默段
// 沉默 = 相邻两个 segment 之间的时间间隔超过阈值
func (d *SilenceDetector) Detect(transcript *model.Transcript) []model.CutSegment {
	if transcript == nil || len(transcript.Segments) < 2 {
		return nil
	}

	var cuts []model.CutSegment

	for i := 1; i < len(transcript.Segments); i++ {
		prev := transcript.Segments[i-1]
		curr := transcript.Segments[i]

		gap := curr.StartMs - prev.EndMs
		if gap >= d.thresholdMs {
			cuts = append(cuts, model.CutSegment{
				ID:        fmt.Sprintf("silence-%d", i),
				StartMs:   prev.EndMs,
				EndMs:     curr.StartMs,
				Decision:  model.CutRemove,
				Reason:    model.ReasonSilence,
				Source:    model.SourceAI,
				Confidence: 0.9,
				Note:      fmt.Sprintf("沉默 %dms", gap),
			})
		}
	}

	return cuts
}

// mergeAnalysisResults 合并规则检测结果和 LLM 分析结果，生成最终 CutList
func mergeAnalysisResults(transcript *model.Transcript, ruleCuts []model.CutSegment, llmResult *model.LLMAnalysisResult) *model.CutList {
	// 先收集所有需要 remove 的段
	removeMap := make(map[int]model.LLMAnalysisItem)
	if llmResult != nil {
		for _, item := range llmResult.Items {
			removeMap[item.SegmentID] = item
		}
	}

	var segments []model.CutSegment

	// 1. 遍历 transcript segments，LLM 标记为 remove 的转为 CutSegment
	if transcript != nil {
		for _, seg := range transcript.Segments {
			if item, ok := removeMap[seg.ID]; ok {
				segments = append(segments, model.CutSegment{
					ID:         fmt.Sprintf("llm-%d", seg.ID),
					StartMs:    seg.StartMs,
					EndMs:      seg.EndMs,
					Decision:   model.CutRemove,
					Reason:     item.Reason,
					Source:     model.SourceAI,
					Confidence: item.Confidence,
					Note:       item.Note,
				})
			}
		}
	}

	// 2. 加入规则检测的沉默段
	segments = append(segments, ruleCuts...)

	// 3. 构建完整 CutList：在 remove 段之间的间隙填充 keep 段
	cutList := &model.CutList{
		Segments: segments,
	}
	cutList.Normalize()

	// 4. 填充 keep 段
	cutList = fillKeepSegments(cutList, transcript)

	return cutList
}

// fillKeepSegments 在 remove 段之间填充 keep 段，形成完整的 keep/remove 交替列表
func fillKeepSegments(cutList *model.CutList, transcript *model.Transcript) *model.CutList {
	if transcript == nil || len(transcript.Segments) == 0 {
		return cutList
	}

	totalStart := transcript.Segments[0].StartMs
	totalEnd := transcript.Segments[len(transcript.Segments)-1].EndMs

	if len(cutList.Segments) == 0 {
		// 没有 remove 段，全部保留
		cutList.Segments = []model.CutSegment{{
			ID:       "keep-all",
			StartMs:  totalStart,
			EndMs:    totalEnd,
			Decision: model.CutKeep,
			Source:   model.SourceAI,
		}}
		return cutList
	}

	// 在 remove 段之间插入 keep 段
	var result []model.CutSegment

	// 开头到第一个 remove 之间
	if cutList.Segments[0].StartMs > totalStart {
		result = append(result, model.CutSegment{
			ID:       "keep-0",
			StartMs:  totalStart,
			EndMs:    cutList.Segments[0].StartMs,
			Decision: model.CutKeep,
			Source:   model.SourceAI,
		})
	}

	for i, seg := range cutList.Segments {
		result = append(result, seg)

		// 当前 remove 段结束到下一个 remove 段开始之间
		if i < len(cutList.Segments)-1 {
			next := cutList.Segments[i+1]
			if seg.EndMs < next.StartMs {
				result = append(result, model.CutSegment{
					ID:       fmt.Sprintf("keep-%d", i+1),
					StartMs:  seg.EndMs,
					EndMs:    next.StartMs,
					Decision: model.CutKeep,
					Source:   model.SourceAI,
				})
			}
		}
	}

	// 最后一个 remove 段到结尾之间
	last := cutList.Segments[len(cutList.Segments)-1]
	if last.EndMs < totalEnd {
		result = append(result, model.CutSegment{
			ID:       "keep-end",
			StartMs:  last.EndMs,
			EndMs:    totalEnd,
			Decision: model.CutKeep,
			Source:   model.SourceAI,
		})
	}

	cutList.Segments = result
	return cutList
}
```

- [ ] **Step 2: 验证编译**

```bash
go build ./internal/pipeline/
```

Expected: 无报错。

- [ ] **Step 3: Commit**

```bash
git add internal/pipeline/silence.go
git commit -m "feat(pipeline): add SilenceDetector and analysis result merging logic"
```

---

### Task 6: Steps + SilenceDetector — 单元测试

**Files:**
- Create: `internal/pipeline/steps_test.go`
- Create: `internal/pipeline/silence_test.go`

- [ ] **Step 1: 编写 silence_test.go**

创建 `internal/pipeline/silence_test.go`：

```go
package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"smart-cut/internal/model"
)

func TestSilenceDetector_DetectsGap(t *testing.T) {
	transcript := &model.Transcript{
		Segments: []model.Segment{
			{ID: 0, StartMs: 0, EndMs: 1000},
			{ID: 1, StartMs: 1000, EndMs: 2000},
			// 1000ms 沉默
			{ID: 2, StartMs: 3000, EndMs: 4000},
		},
	}

	detector := NewSilenceDetector(800)
	cuts := detector.Detect(transcript)

	assert.Len(t, cuts, 1)
	assert.Equal(t, int64(2000), cuts[0].StartMs)
	assert.Equal(t, int64(3000), cuts[0].EndMs)
	assert.Equal(t, model.CutRemove, cuts[0].Decision)
	assert.Equal(t, model.ReasonSilence, cuts[0].Reason)
}

func TestSilenceDetector_NoGapBelowThreshold(t *testing.T) {
	transcript := &model.Transcript{
		Segments: []model.Segment{
			{ID: 0, StartMs: 0, EndMs: 1000},
			{ID: 1, StartMs: 1500, EndMs: 2000}, // 500ms gap < 800ms
		},
	}

	detector := NewSilenceDetector(800)
	cuts := detector.Detect(transcript)

	assert.Empty(t, cuts)
}

func TestSilenceDetector_SingleSegment(t *testing.T) {
	transcript := &model.Transcript{
		Segments: []model.Segment{{ID: 0, StartMs: 0, EndMs: 1000}},
	}

	detector := NewSilenceDetector(800)
	cuts := detector.Detect(transcript)

	assert.Empty(t, cuts)
}

func TestSilenceDetector_NilTranscript(t *testing.T) {
	detector := NewSilenceDetector(800)
	cuts := detector.Detect(nil)

	assert.Empty(t, cuts)
}

func TestSilenceDetector_DefaultThreshold(t *testing.T) {
	detector := NewSilenceDetector(0)
	assert.Equal(t, int64(800), detector.thresholdMs)
}

func TestMergeAnalysisResults_OnlyRules(t *testing.T) {
	transcript := &model.Transcript{
		Segments: []model.Segment{
			{ID: 0, StartMs: 0, EndMs: 1000},
			{ID: 1, StartMs: 2000, EndMs: 3000}, // 1000ms gap
		},
	}

	ruleCuts := []model.CutSegment{
		{ID: "silence-1", StartMs: 1000, EndMs: 2000, Decision: model.CutRemove, Reason: model.ReasonSilence},
	}

	cutList := mergeAnalysisResults(transcript, ruleCuts, nil)

	// 应该有 3 段：keep(0-1000), remove(1000-2000), keep(2000-3000)
	assert.Len(t, cutList.Segments, 3)
	assert.Equal(t, model.CutKeep, cutList.Segments[0].Decision)
	assert.Equal(t, model.CutRemove, cutList.Segments[1].Decision)
	assert.Equal(t, model.CutKeep, cutList.Segments[2].Decision)
}

func TestMergeAnalysisResults_OnlyLLM(t *testing.T) {
	transcript := &model.Transcript{
		Segments: []model.Segment{
			{ID: 0, StartMs: 0, EndMs: 1000, Text: "嗯"},
			{ID: 1, StartMs: 1000, EndMs: 2000, Text: "大家好"},
		},
	}

	llmResult := &model.LLMAnalysisResult{
		RemoveSegmentIDs: []int{0},
		Items: []model.LLMAnalysisItem{
			{SegmentID: 0, Reason: model.ReasonFiller, Confidence: 0.95, Note: "语气词"},
		},
	}

	cutList := mergeAnalysisResults(transcript, nil, llmResult)

	// 应该有 2 段：remove(0-1000), keep(1000-2000)
	assert.Len(t, cutList.Segments, 2)
	assert.Equal(t, model.CutRemove, cutList.Segments[0].Decision)
	assert.Equal(t, model.ReasonFiller, cutList.Segments[0].Reason)
	assert.Equal(t, model.CutKeep, cutList.Segments[1].Decision)
}

func TestMergeAnalysisResults_NoRemoves(t *testing.T) {
	transcript := &model.Transcript{
		Segments: []model.Segment{
			{ID: 0, StartMs: 0, EndMs: 5000},
		},
	}

	cutList := mergeAnalysisResults(transcript, nil, &model.LLMAnalysisResult{})

	// 没有 remove，应该整段 keep
	assert.Len(t, cutList.Segments, 1)
	assert.Equal(t, model.CutKeep, cutList.Segments[0].Decision)
	assert.Equal(t, int64(0), cutList.Segments[0].StartMs)
	assert.Equal(t, int64(5000), cutList.Segments[0].EndMs)
}

func TestFillKeepSegments_FillsCorrectly(t *testing.T) {
	cutList := &model.CutList{
		Segments: []model.CutSegment{
			{StartMs: 1000, EndMs: 2000, Decision: model.CutRemove},
		},
	}
	transcript := &model.Transcript{
		Segments: []model.Segment{
			{StartMs: 0, EndMs: 3000},
		},
	}

	result := fillKeepSegments(cutList, transcript)

	// keep(0-1000), remove(1000-2000), keep(2000-3000)
	assert.Len(t, result.Segments, 3)
	assert.Equal(t, model.CutKeep, result.Segments[0].Decision)
	assert.Equal(t, int64(0), result.Segments[0].StartMs)
	assert.Equal(t, model.CutRemove, result.Segments[1].Decision)
	assert.Equal(t, model.CutKeep, result.Segments[2].Decision)
	assert.Equal(t, int64(3000), result.Segments[2].EndMs)
}
```

- [ ] **Step 2: 编写 steps_test.go**

创建 `internal/pipeline/steps_test.go`：

```go
package pipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smart-cut/internal/model"
)

// mockWhisperAdapter 用于测试
type mockWhisperAdapter struct {
	transcript *model.Transcript
	err        error
}

func (m *mockWhisperAdapter) Transcribe(ctx context.Context, mediaPath string, opts interface{}) (*model.Transcript, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.transcript, nil
}

// mockLLMAdapter 用于测试
type mockLLMAdapter struct {
	result *model.LLMAnalysisResult
	err    error
}

func (m *mockLLMAdapter) Analyze(ctx context.Context, req model.LLMAnalysisRequest, cfg model.LLMConfig) (*model.LLMAnalysisResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func TestAnalyzeStep_Run_Success(t *testing.T) {
	transcript := &model.Transcript{
		Language: "zh",
		Segments: []model.Segment{
			{ID: 0, StartMs: 0, EndMs: 1000, Text: "嗯"},
			{ID: 1, StartMs: 1000, EndMs: 2000, Text: "大家好"},
		},
	}

	llmResult := &model.LLMAnalysisResult{
		RemoveSegmentIDs: []int{0},
		Items: []model.LLMAnalysisItem{
			{SegmentID: 0, Reason: model.ReasonFiller, Confidence: 0.9, Note: "语气词"},
		},
	}

	step := NewAnalyzeStep(&mockLLMAdapter{result: llmResult}, model.LLMConfig{}, 800)

	ctx := &Context{
		Project:    &model.Project{ID: "p1"},
		Transcript: transcript,
		Cancel:     context.Background(),
	}

	reporter := &mockReporter{}
	err := step.Run(ctx, reporter)

	require.NoError(t, err)
	require.NotNil(t, ctx.CutList)
	assert.Equal(t, "p1", ctx.CutList.ProjectID)
	assert.True(t, len(ctx.CutList.Segments) >= 2)
}

func TestAnalyzeStep_Run_NilTranscript(t *testing.T) {
	step := NewAnalyzeStep(&mockLLMAdapter{}, model.LLMConfig{}, 800)

	ctx := &Context{
		Project: &model.Project{ID: "p1"},
		Cancel:  context.Background(),
	}

	err := step.Run(ctx, &mockReporter{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transcript is nil")
}

func TestAnalyzeStep_Run_LLMFailure_UsesRulesOnly(t *testing.T) {
	transcript := &model.Transcript{
		Language: "zh",
		Segments: []model.Segment{
			{ID: 0, StartMs: 0, EndMs: 1000},
			{ID: 1, StartMs: 2500, EndMs: 3000}, // 1500ms gap > 800ms
		},
	}

	// LLM 返回错误
	step := NewAnalyzeStep(
		&mockLLMAdapter{err: context.DeadlineExceeded},
		model.LLMConfig{},
		800,
	)

	ctx := &Context{
		Project:    &model.Project{ID: "p1"},
		Transcript: transcript,
		Cancel:     context.Background(),
	}

	err := step.Run(ctx, &mockReporter{})
	require.NoError(t, err) // LLM 失败不阻塞
	require.NotNil(t, ctx.CutList)

	// 应该有规则检测到的沉默段
	hasSilence := false
	for _, seg := range ctx.CutList.Segments {
		if seg.Reason == model.ReasonSilence {
			hasSilence = true
			break
		}
	}
	assert.True(t, hasSilence)
}

func TestExportStep_Run_NoCutList(t *testing.T) {
	step := NewExportStep(nil, model.ExportOptions{})

	ctx := &Context{
		Project: &model.Project{ID: "p1"},
		Cancel:  context.Background(),
	}

	err := step.Run(ctx, &mockReporter{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cutlist is nil")
}

func TestExportStep_Run_NoKeepSegments(t *testing.T) {
	step := NewExportStep(nil, model.ExportOptions{})

	ctx := &Context{
		Project: &model.Project{ID: "p1"},
		CutList: &model.CutList{
			Segments: []model.CutSegment{
				{Decision: model.CutRemove, StartMs: 0, EndMs: 1000},
			},
		},
		Cancel: context.Background(),
	}

	err := step.Run(ctx, &mockReporter{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no keep segments")
}

func TestSubtitleStep_Run_MVPSkips(t *testing.T) {
	step := NewSubtitleStep()
	ctx := &Context{Cancel: context.Background()}

	err := step.Run(ctx, &mockReporter{})
	require.NoError(t, err)
}

func TestTranscribeStep_Name(t *testing.T) {
	step := NewTranscribeStep(nil, nil, adapter.WhisperOptions{})
	assert.Equal(t, "transcribe", step.Name())
}
```

- [ ] **Step 3: 修复编译问题**

steps_test.go 里的 mockWhisperAdapter.Transcribe 签名需要匹配 adapter.WhisperAdapter 接口。如果编译失败，调整 mock 的签名使其匹配 `adapter.WhisperOptions` 参数类型。需要 import `smart-cut/internal/adapter`。

- [ ] **Step 4: 运行测试**

```bash
go test ./internal/pipeline/ -v
```

Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/pipeline/steps_test.go internal/pipeline/silence_test.go
git commit -m "test(pipeline): add SilenceDetector, merge logic, and Step tests"
```

---

### Task 7: Service 层 — ProjectService + EditService

**Files:**
- Create: `internal/service/project.go`
- Create: `internal/service/edit.go`
- Create: `internal/service/edit_test.go`

- [ ] **Step 1: 编写 project.go**

创建 `internal/service/project.go`：

```go
package service

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"smart-cut/internal/model"
)

// ProjectService 管理项目生命周期
type ProjectService struct {
	projectsDir string // 项目存储根目录
}

// NewProjectService 创建 ProjectService
func NewProjectService(projectsDir string) *ProjectService {
	if projectsDir == "" {
		projectsDir = filepath.Join(os.TempDir(), "smart-cut-projects")
	}
	return &ProjectService{projectsDir: projectsDir}
}

// CreateProject 创建新项目
func (s *ProjectService) CreateProject(name, mediaPath string) (*model.Project, error) {
	id := fmt.Sprintf("proj-%d", time.Now().UnixMilli())

	workDir := filepath.Join(s.projectsDir, id)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}

	project := &model.Project{
		ID:        id,
		Name:      name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		WorkDir:   workDir,
		Media: model.MediaFile{
			Path: mediaPath,
		},
		Status: model.StatusDraft,
		Settings: model.ProjectSettings{
			ExportMode: model.ExportReencode,
			SilenceMs:  800,
			FillerDict: []string{"嗯", "啊", "那个", "就是说"},
			SubtitleStyle: model.SubtitleStyle{
				FontFamily: "Microsoft YaHei",
				FontSize:   48,
				Color:      "#FFFFFF",
				Highlight:  "#FFD700",
				Position:   "bottom",
				BgColor:    "#000000",
				BgOpacity:  0.5,
			},
		},
	}

	if err := s.SaveProject(project); err != nil {
		return nil, err
	}

	return project, nil
}

// SaveProject 保存项目到文件
func (s *ProjectService) SaveProject(p *model.Project) error {
	p.UpdatedAt = time.Now()

	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("save project: %w", err)
	}

	path := filepath.Join(p.WorkDir, "project.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("save project: %w", err)
	}

	return nil
}

// OpenProject 从文件加载项目
func (s *ProjectService) OpenProject(projectPath string) (*model.Project, error) {
	data, err := os.ReadFile(projectPath)
	if err != nil {
		return nil, fmt.Errorf("open project: %w", err)
	}

	var project model.Project
	if err := json.Unmarshal(data, &project); err != nil {
		return nil, fmt.Errorf("open project: %w", err)
	}

	return &project, nil
}

// GetProjectPath 获取项目文件路径
func (s *ProjectService) GetProjectPath(projectID string) string {
	return filepath.Join(s.projectsDir, projectID, "project.json")
}
```

- [ ] **Step 2: 编写 edit.go**

创建 `internal/service/edit.go`：

```go
package service

import (
	"fmt"
	"sync"

	"smart-cut/internal/model"
)

// EditService 管理 CutList 的编辑操作
type EditService struct {
	mu        sync.RWMutex
	cutLists  map[string]*model.CutList // projectID → CutList（内存缓存）
}

// NewEditService 创建 EditService
func NewEditService() *EditService {
	return &EditService{
		cutLists: make(map[string]*model.CutList),
	}
}

// GetCutList 获取项目的剪切清单
func (s *EditService) GetCutList(projectID string) (*model.CutList, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cl, ok := s.cutLists[projectID]
	if !ok {
		return nil, fmt.Errorf("cutlist not found for project %s", projectID)
	}
	return cl, nil
}

// SetCutList 设置项目的剪切清单（分析完成后调用）
func (s *EditService) SetCutList(projectID string, cl *model.CutList) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cl.ProjectID = projectID
	s.cutLists[projectID] = cl
}

// AddCutSegment 添加一个剪切段
func (s *EditService) AddCutSegment(projectID string, seg model.CutSegment) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cl, ok := s.cutLists[projectID]
	if !ok {
		return fmt.Errorf("cutlist not found for project %s", projectID)
	}

	seg.Source = model.SourceManual
	cl.Segments = append(cl.Segments, seg)
	cl.Normalize()

	return nil
}

// UpdateCutSegment 更新一个剪切段
func (s *EditService) UpdateCutSegment(projectID string, seg model.CutSegment) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cl, ok := s.cutLists[projectID]
	if !ok {
		return fmt.Errorf("cutlist not found for project %s", projectID)
	}

	for i, existing := range cl.Segments {
		if existing.ID == seg.ID {
			seg.Source = model.SourceManual
			cl.Segments[i] = seg
			cl.Normalize()
			return nil
		}
	}

	return fmt.Errorf("segment %s not found", seg.ID)
}

// RemoveCutSegment 删除一个剪切段
func (s *EditService) RemoveCutSegment(projectID, segID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cl, ok := s.cutLists[projectID]
	if !ok {
		return fmt.Errorf("cutlist not found for project %s", projectID)
	}

	for i, seg := range cl.Segments {
		if seg.ID == segID {
			cl.Segments = append(cl.Segments[:i], cl.Segments[i+1:]...)
			cl.Normalize()
			return nil
		}
	}

	return fmt.Errorf("segment %s not found", segID)
}

// ToggleSegment 切换段的 keep/remove
func (s *EditService) ToggleSegment(projectID, segID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cl, ok := s.cutLists[projectID]
	if !ok {
		return fmt.Errorf("cutlist not found for project %s", projectID)
	}

	for i, seg := range cl.Segments {
		if seg.ID == segID {
			if seg.Decision == model.CutKeep {
				cl.Segments[i].Decision = model.CutRemove
			} else {
				cl.Segments[i].Decision = model.CutKeep
			}
			cl.Segments[i].Source = model.SourceManual
			cl.Normalize()
			return nil
		}
	}

	return fmt.Errorf("segment %s not found", segID)
}
```

- [ ] **Step 3: 编写 edit_test.go**

创建 `internal/service/edit_test.go`：

```go
package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smart-cut/internal/model"
)

func TestEditService_SetAndGetCutList(t *testing.T) {
	svc := NewEditService()

	cl := &model.CutList{
		Segments: []model.CutSegment{
			{ID: "1", StartMs: 0, EndMs: 1000, Decision: model.CutKeep},
		},
	}

	svc.SetCutList("p1", cl)

	result, err := svc.GetCutList("p1")
	require.NoError(t, err)
	assert.Equal(t, "p1", result.ProjectID)
	assert.Len(t, result.Segments, 1)
}

func TestEditService_GetCutList_NotFound(t *testing.T) {
	svc := NewEditService()

	_, err := svc.GetCutList("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestEditService_AddCutSegment(t *testing.T) {
	svc := NewEditService()
	svc.SetCutList("p1", &model.CutList{
		Segments: []model.CutSegment{
			{ID: "keep-1", StartMs: 0, EndMs: 1000, Decision: model.CutKeep},
		},
	})

	err := svc.AddCutSegment("p1", model.CutSegment{
		ID:       "manual-1",
		StartMs:  2000,
		EndMs:    3000,
		Decision: model.CutRemove,
	})
	require.NoError(t, err)

	cl, _ := svc.GetCutList("p1")
	assert.Len(t, cl.Segments, 2)

	// 新段应标记为 manual
	var manualSeg *model.CutSegment
	for i := range cl.Segments {
		if cl.Segments[i].ID == "manual-1" {
			manualSeg = &cl.Segments[i]
		}
	}
	require.NotNil(t, manualSeg)
	assert.Equal(t, model.SourceManual, manualSeg.Source)
}

func TestEditService_UpdateCutSegment(t *testing.T) {
	svc := NewEditService()
	svc.SetCutList("p1", &model.CutList{
		Segments: []model.CutSegment{
			{ID: "seg-1", StartMs: 0, EndMs: 1000, Decision: model.CutKeep},
		},
	})

	err := svc.UpdateCutSegment("p1", model.CutSegment{
		ID:       "seg-1",
		StartMs:  0,
		EndMs:    2000,
		Decision: model.CutRemove,
	})
	require.NoError(t, err)

	cl, _ := svc.GetCutList("p1")
	assert.Equal(t, int64(2000), cl.Segments[0].EndMs)
	assert.Equal(t, model.CutRemove, cl.Segments[0].Decision)
	assert.Equal(t, model.SourceManual, cl.Segments[0].Source)
}

func TestEditService_UpdateCutSegment_NotFound(t *testing.T) {
	svc := NewEditService()
	svc.SetCutList("p1", &model.CutList{})

	err := svc.UpdateCutSegment("p1", model.CutSegment{ID: "nonexistent"})
	assert.Error(t, err)
}

func TestEditService_RemoveCutSegment(t *testing.T) {
	svc := NewEditService()
	svc.SetCutList("p1", &model.CutList{
		Segments: []model.CutSegment{
			{ID: "seg-1", StartMs: 0, EndMs: 1000, Decision: model.CutKeep},
			{ID: "seg-2", StartMs: 1000, EndMs: 2000, Decision: model.CutRemove},
		},
	})

	err := svc.RemoveCutSegment("p1", "seg-2")
	require.NoError(t, err)

	cl, _ := svc.GetCutList("p1")
	assert.Len(t, cl.Segments, 1)
	assert.Equal(t, "seg-1", cl.Segments[0].ID)
}

func TestEditService_ToggleSegment(t *testing.T) {
	svc := NewEditService()
	svc.SetCutList("p1", &model.CutList{
		Segments: []model.CutSegment{
			{ID: "seg-1", Decision: model.CutKeep},
		},
	})

	// keep → remove
	err := svc.ToggleSegment("p1", "seg-1")
	require.NoError(t, err)

	cl, _ := svc.GetCutList("p1")
	assert.Equal(t, model.CutRemove, cl.Segments[0].Decision)

	// remove → keep
	err = svc.ToggleSegment("p1", "seg-1")
	require.NoError(t, err)

	cl, _ = svc.GetCutList("p1")
	assert.Equal(t, model.CutKeep, cl.Segments[0].Decision)
}
```

- [ ] **Step 4: 运行测试**

```bash
go test ./internal/service/ -v
```

Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/service/
git commit -m "feat(service): add ProjectService and EditService with CutList management"
```

---

### Task 8: Service 层 — TranscribeService + AnalyzeService + ExportService

**Files:**
- Create: `internal/service/transcribe.go`
- Create: `internal/service/analyze.go`
- Create: `internal/service/export.go`

- [ ] **Step 1: 编写 transcribe.go**

创建 `internal/service/transcribe.go`：

```go
package service

import (
	"context"
	"fmt"
	"path/filepath"

	"smart-cut/internal/adapter"
	"smart-cut/internal/eventbus"
	"smart-cut/internal/model"
	"smart-cut/internal/pipeline"
)

// TranscribeService 编排转录流程
type TranscribeService struct {
	whisper   adapter.WhisperAdapter
	ffmpeg    adapter.FFmpegAdapter
	bus       *eventbus.EventBus
	editSvc   *EditService
}

// NewTranscribeService 创建 TranscribeService
func NewTranscribeService(whisper adapter.WhisperAdapter, ffmpeg adapter.FFmpegAdapter, bus *eventbus.EventBus, editSvc *EditService) *TranscribeService {
	return &TranscribeService{whisper: whisper, ffmpeg: ffmpeg, bus: bus, editSvc: editSvc}
}

// StartTranscribe 启动转录任务（异步）
// 返回 taskID，结果通过 EventBus 推送
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

		// 推送转录结果
		s.bus.Emit("transcript:ready", ctx.Transcript)
	}()

	return taskID
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

- [ ] **Step 2: 编写 analyze.go**

创建 `internal/service/analyze.go`：

```go
package service

import (
	"context"
	"fmt"

	"smart-cut/internal/adapter"
	"smart-cut/internal/eventbus"
	"smart-cut/internal/model"
	"smart-cut/internal/pipeline"
)

// AnalyzeService 编排分析流程
type AnalyzeService struct {
	llm     adapter.LLMAdapter
	bus     *eventbus.EventBus
	editSvc *EditService
}

// NewAnalyzeService 创建 AnalyzeService
func NewAnalyzeService(llm adapter.LLMAdapter, bus *eventbus.EventBus, editSvc *EditService) *AnalyzeService {
	return &AnalyzeService{llm: llm, bus: bus, editSvc: editSvc}
}

// StartAnalyze 启动分析任务（异步）
func (s *AnalyzeService) StartAnalyze(project *model.Project, transcript *model.Transcript) string {
	taskID := fmt.Sprintf("analyze-%s", project.ID)

	go func() {
		ctx := &pipeline.Context{
			Project:    project,
			Transcript: transcript,
			Cancel:     context.Background(),
		}

		reporter := pipeline.NewEventBusReporter(s.bus, taskID)

		step := pipeline.NewAnalyzeStep(s.llm, project.Settings.LLMConfig, project.Settings.SilenceMs)

		if err := step.Run(ctx, reporter); err != nil {
			s.bus.EmitProgress(model.ProgressEvent{
				TaskID: taskID,
				Stage:  "analyze",
				Status: model.TaskError,
				Error:  err.Error(),
			})
			return
		}

		// 存入 EditService
		s.editSvc.SetCutList(project.ID, ctx.CutList)

		// 推送分析结果
		s.bus.Emit("cutlist:ready", ctx.CutList)
	}()

	return taskID
}
```

- [ ] **Step 3: 编写 export.go**

创建 `internal/service/export.go`：

```go
package service

import (
	"context"
	"fmt"

	"smart-cut/internal/adapter"
	"smart-cut/internal/eventbus"
	"smart-cut/internal/model"
	"smart-cut/internal/pipeline"
)

// ExportService 编排导出流程
type ExportService struct {
	ffmpeg adapter.FFmpegAdapter
	bus    *eventbus.EventBus
}

// NewExportService 创建 ExportService
func NewExportService(ffmpeg adapter.FFmpegAdapter, bus *eventbus.EventBus) *ExportService {
	return &ExportService{ffmpeg: ffmpeg, bus: bus}
}

// StartExport 启动导出任务（异步）
func (s *ExportService) StartExport(project *model.Project, cutList *model.CutList, opts model.ExportOptions) string {
	taskID := fmt.Sprintf("export-%s", project.ID)

	go func() {
		ctx := &pipeline.Context{
			Project: project,
			CutList: cutList,
			Cancel:  context.Background(),
		}

		reporter := pipeline.NewEventBusReporter(s.bus, taskID)

		step := pipeline.NewExportStep(s.ffmpeg, opts)

		if err := step.Run(ctx, reporter); err != nil {
			s.bus.EmitProgress(model.ProgressEvent{
				TaskID: taskID,
				Stage:  "export",
				Status: model.TaskError,
				Error:  err.Error(),
			})
			return
		}

		s.bus.Emit("export:done", ctx.ExportPath)
	}()

	return taskID
}
```

- [ ] **Step 4: 验证编译**

```bash
go build ./internal/service/
```

Expected: 无报错。注意 project.go 需要 import `encoding/json`。

- [ ] **Step 5: Commit**

```bash
git add internal/service/transcribe.go internal/service/analyze.go internal/service/export.go
git commit -m "feat(service): add TranscribeService, AnalyzeService, ExportService"
```

---

### Task 9: 最终验证

**Files:**
- 无文件变更，仅验证

- [ ] **Step 1: 全量编译**

```bash
go build ./internal/...
go build .
```

Expected: 无报错。

- [ ] **Step 2: 全量测试**

```bash
go test ./internal/... -v
```

Expected: 全部 PASS（eventbus + pipeline + service + adapter + model）。

- [ ] **Step 3: 最终 commit**

```bash
git add -A
git commit -m "chore: plan 3 complete - pipeline and service layer ready"
```

---

## 完成标准

Plan 3 完成后应满足：
1. ✅ `go build ./internal/...` 无报错
2. ✅ `go test ./internal/... -v` 全部 PASS
3. ✅ `go build .` 主程序编译通过
4. ✅ `internal/eventbus/` — EventBus（线程安全、可注入 emitFunc）
5. ✅ `internal/pipeline/` — Pipeline + Step + ProgressReporter + 4个Step实现 + SilenceDetector + 结果合并
6. ✅ `internal/service/` — ProjectService + EditService + TranscribeService + AnalyzeService + ExportService

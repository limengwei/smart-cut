package pipeline

import (
	"context"
	"fmt"

	"smart-cut/internal/model"
)

// Context 在 Pipeline 各 Step 间共享数据
type Context struct {
	Project       *model.Project
	Transcript    *model.Transcript
	CutList       *model.CutList
	ExportPath    string
	SubtitleClips map[string]string // segID → 字幕透明 mp4 路径（仅 IncludeSubtitle=true 时填充）
	OverlayClips  map[string]string // segID → overlay 透明 mp4 路径
	Cancel        context.Context
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

	if p.reporter != nil {
		p.reporter.Done("pipeline", nil)
	}
	return nil
}

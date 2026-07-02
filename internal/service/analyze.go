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
		cancelCtx, cancel := context.WithCancel(context.Background())
		ctx := &pipeline.Context{
			Project:    project,
			Transcript: transcript,
			Cancel:     cancelCtx,
		}
		_ = cancel

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

		s.editSvc.SetCutList(project.ID, ctx.CutList)
		s.bus.Emit("cutlist:ready", ctx.CutList)
	}()

	return taskID
}

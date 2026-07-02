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
		cancelCtx, cancel := context.WithCancel(context.Background())
		ctx := &pipeline.Context{
			Project: project,
			CutList: cutList,
			Cancel:  cancelCtx,
		}
		_ = cancel

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

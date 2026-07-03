package service

import (
	"context"
	"fmt"
	"log"

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
func (s *ExportService) StartExport(project *model.Project, cutList *model.CutList, transcript *model.Transcript, opts model.ExportOptions, remotionAdp adapter.RemotionAdapter) string {
	taskID := fmt.Sprintf("export-%s", project.ID)

	go func() {
		cancelCtx, cancel := context.WithCancel(context.Background())
		ctx := &pipeline.Context{
			Project:    project,
			CutList:    cutList,
			Transcript: transcript,
			Cancel:     cancelCtx,
		}
		_ = cancel

		reporter := pipeline.NewEventBusReporter(s.bus, taskID)

		// 1. 字幕渲染（仅 IncludeSubtitle=true 且有 RemotionAdapter）
		if opts.IncludeSubtitle && remotionAdp != nil && transcript != nil && len(transcript.Segments) > 0 {
			subStep := pipeline.NewSubtitleStep(remotionAdp)
			if err := subStep.Run(ctx, reporter); err != nil {
				log.Printf("[Export] SubtitleStep 失败（继续无字幕导出）: %v", err)
				ctx.SubtitleClips = nil
			}
		}

		// 2. 视频导出
		step := pipeline.NewExportStep(s.ffmpeg, opts)

		if err := step.Run(ctx, reporter); err != nil {
			log.Printf("[Export] 任务失败 projectID=%s err=%v", project.ID, err)
			s.bus.EmitProgress(model.ProgressEvent{
				TaskID: taskID,
				Stage:  "export",
				Status: model.TaskError,
				Error:  err.Error(),
			})
			return
		}

		log.Printf("[Export] 任务完成 projectID=%s out=%s", project.ID, ctx.ExportPath)
		s.bus.Emit("export:done", ctx.ExportPath)
	}()

	return taskID
}

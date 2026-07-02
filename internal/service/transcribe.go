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
	whisper adapter.WhisperAdapter
	ffmpeg  adapter.FFmpegAdapter
	bus     *eventbus.EventBus
	editSvc *EditService
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

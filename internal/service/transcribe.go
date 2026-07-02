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
	peaks       sync.Map // projectID → *model.WaveformPeaks
}

// NewTranscribeService 创建 TranscribeService
func NewTranscribeService(whisper adapter.WhisperAdapter, ffmpeg adapter.FFmpegAdapter, bus *eventbus.EventBus, editSvc *EditService) *TranscribeService {
	return &TranscribeService{whisper: whisper, ffmpeg: ffmpeg, bus: bus, editSvc: editSvc}
}

// StartTranscribe 启动转录任务（异步）
func (s *TranscribeService) StartTranscribe(project *model.Project, modelPath string) string {
	taskID := fmt.Sprintf("transcribe-%s", project.ID)

	go func() {
		cancelCtx, cancel := context.WithCancel(context.Background())
		ctx := &pipeline.Context{
			Project: project,
			Cancel:  cancelCtx,
		}
		_ = cancel // 保留 cancel 引用，后续 Task 支持取消时使用

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
	transcript, ok := val.(*model.Transcript)
	if !ok {
		return nil, fmt.Errorf("invalid transcript type for project %s", projectID)
	}
	return transcript, nil
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

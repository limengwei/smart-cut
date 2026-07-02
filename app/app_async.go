package app

import (
	"context"
	"path/filepath"

	"smart-cut/internal/model"
)

func (a *App) StartTranscribe(projectID string) (string, error) {
	project, err := a.GetProject(projectID)
	if err != nil {
		return "", err
	}

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

func (a *App) GetTranscript(projectID string) (*model.Transcript, error) {
	t, err := a.transcribeService.GetTranscript(projectID)
	if err != nil {
		return nil, NewAppError(ErrCodeInternal, "获取转录结果失败", err.Error())
	}
	return t, nil
}

func (a *App) GetWaveform(projectID string) (string, error) {
	project, err := a.GetProject(projectID)
	if err != nil {
		return "", err
	}

	waveformPath := filepath.Join(project.WorkDir, "waveform.png")

	ctx := context.Background()
	err = a.transcribeService.ExtractWaveform(ctx, project)
	if err != nil {
		return "", NewAppError(ErrCodeInternal, "提取波形失败", err.Error())
	}

	return waveformPath, nil
}

func (a *App) ProbeMedia(path string) (*model.MediaFile, error) {
	ctx := context.Background()
	mf, err := a.transcribeService.ProbeMedia(ctx, path)
	if err != nil {
		return nil, NewAppError(ErrCodeParam, "媒体文件探测失败", err.Error())
	}
	return mf, nil
}
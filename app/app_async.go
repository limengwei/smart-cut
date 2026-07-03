package app

import (
	"context"
	"log"
	"path/filepath"

	"smart-cut/internal/adapter"
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

	resolved, err := resolveWhisperModel(modelPath)
	if err != nil {
		return "", NewAppError(ErrCodeEnv, "Whisper 模型解析失败", err.Error())
	}

	log.Printf("[App] StartTranscribe: projectID=%s model=%s media=%s", projectID, resolved, project.Media.Path)
	taskID := a.transcribeService.StartTranscribe(project, resolved)
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

	// LLM 配置：项目级为空时回退用全局 defaultLLM
	if project.Settings.LLMConfig.BaseURL == "" || project.Settings.LLMConfig.APIKey == "" || project.Settings.LLMConfig.Model == "" {
		settings, err := a.configManager.Load()
		if err != nil {
			return "", NewAppError(ErrCodeEnv, "加载设置失败", err.Error())
		}
		log.Printf("[App] StartAnalyze: 项目级 LLMConfig 不完整，回退全局 defaultLLM (model=%s)", settings.DefaultLLM.Model)
		if project.Settings.LLMConfig.BaseURL == "" {
			project.Settings.LLMConfig.BaseURL = settings.DefaultLLM.BaseURL
		}
		if project.Settings.LLMConfig.APIKey == "" {
			project.Settings.LLMConfig.APIKey = settings.DefaultLLM.APIKey
		}
		if project.Settings.LLMConfig.Model == "" {
			project.Settings.LLMConfig.Model = settings.DefaultLLM.Model
		}
	}
	// 最终校验：API Key 仍为空则提前报错（避免 LLM 请求静默失败）
	if project.Settings.LLMConfig.APIKey == "" {
		log.Printf("[App] StartAnalyze: API Key 为空，拒绝启动分析")
		return "", NewAppError(ErrCodeLLM, "未配置 LLM API Key", "请在设置页填写 LLM API Key 后重试")
	}

	log.Printf("[App] StartAnalyze: projectID=%s transcriptSegs=%d llmModel=%s", projectID, len(transcript.Segments), project.Settings.LLMConfig.Model)
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

	// 获取转录结果（字幕渲染需要，未转录则为 nil）
	var transcript *model.Transcript
	if t, err := a.transcribeService.GetTranscript(projectID); err == nil {
		transcript = t
	}

	var remotionAdp adapter.RemotionAdapter
	if a.subtitleService != nil {
		remotionAdp = a.subtitleService.Adapter()
	}

	taskID := a.exportService.StartExport(project, cl, transcript, opts, remotionAdp)
	return taskID, nil
}

func (a *App) GetTranscript(projectID string) (*model.Transcript, error) {
	t, err := a.transcribeService.GetTranscript(projectID)
	if err != nil {
		// 未转录是正常的初始状态，返回 nil 而非错误（避免 wails 刷 ERR 日志）
		return nil, nil
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

// GetWaveformPeaks 获取波形峰值采样数据（供前端 canvas 渲染）
func (a *App) GetWaveformPeaks(projectID string) (*model.WaveformPeaks, error) {
	project, err := a.GetProject(projectID)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	peaks, err := a.transcribeService.GetWaveformPeaks(ctx, project)
	if err != nil {
		return nil, NewAppError(ErrCodeInternal, "提取波形数据失败", err.Error())
	}
	return peaks, nil
}

// GetMediaURL 获取项目媒体文件的 webview 可访问 URL
func (a *App) GetMediaURL(projectID string) (string, error) {
	if _, err := a.GetProject(projectID); err != nil {
		return "", err
	}
	if a.mediaServer == nil {
		return "", NewAppError(ErrCodeInternal, "媒体服务未启动", "")
	}
	return a.mediaServer.URL(projectID), nil
}

func (a *App) ProbeMedia(path string) (*model.MediaFile, error) {
	ctx := context.Background()
	mf, err := a.transcribeService.ProbeMedia(ctx, path)
	if err != nil {
		return nil, NewAppError(ErrCodeParam, "媒体文件探测失败", err.Error())
	}
	return mf, nil
}

// SubtitleConfig 前端 Player 所需的字幕配置
type SubtitleConfig struct {
	Segments []model.Segment     `json:"segments"`
	Style    model.SubtitleStyle `json:"style"`
}

// GetSubtitleConfig 返回前端 Player 所需的字幕配置（句段 + 样式）
func (a *App) GetSubtitleConfig(projectID string) (*SubtitleConfig, error) {
	project, err := a.GetProject(projectID)
	if err != nil {
		return nil, err
	}
	var segments []model.Segment
	if t, err := a.transcribeService.GetTranscript(projectID); err == nil {
		segments = t.Segments
	}
	return &SubtitleConfig{
		Segments: segments,
		Style:    project.Settings.SubtitleStyle,
	}, nil
}

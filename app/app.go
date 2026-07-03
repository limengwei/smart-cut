package app

import (
	"context"
	"fmt"
	"sync"

	"smart-cut/internal/adapter"
	"smart-cut/internal/config"
	"smart-cut/internal/model"
	"smart-cut/internal/service"
)

type App struct {
	projectService    *service.ProjectService
	transcribeService *service.TranscribeService
	analyzeService    *service.AnalyzeService
	editService       *service.EditService
	exportService     *service.ExportService
	subtitleService   *service.SubtitleService
	configManager     *config.ConfigManager
	binaryResolver    *adapter.BinaryResolver
	mediaServer       *mediaServer

	mu       sync.RWMutex
	projects map[string]*model.Project
}

func NewApp(
	projectService *service.ProjectService,
	transcribeService *service.TranscribeService,
	analyzeService *service.AnalyzeService,
	editService *service.EditService,
	exportService *service.ExportService,
	subtitleService *service.SubtitleService,
	configManager *config.ConfigManager,
	binaryResolver *adapter.BinaryResolver,
	mediaServer *mediaServer,
) *App {
	return &App{
		projectService:    projectService,
		transcribeService: transcribeService,
		analyzeService:    analyzeService,
		editService:       editService,
		exportService:     exportService,
		subtitleService:   subtitleService,
		configManager:     configManager,
		binaryResolver:    binaryResolver,
		mediaServer:       mediaServer,
		projects:          make(map[string]*model.Project),
	}
}

func (a *App) CreateProject(name, mediaPath string) (*model.Project, error) {
	project, err := a.projectService.CreateProject(name, mediaPath)
	if err != nil {
		return nil, NewAppError(ErrCodeInternal, "创建项目失败", err.Error())
	}

	// 探测媒体元数据（时长、分辨率、帧率、是否有音轨），填充后重新保存
	media, err := a.transcribeService.ProbeMedia(context.Background(), project.Media.Path)
	if err != nil {
		return nil, NewAppError(ErrCodeEnv, "媒体文件探测失败", err.Error())
	}
	media.Path = project.Media.Path
	project.Media = *media
	if err := a.projectService.SaveProject(project); err != nil {
		return nil, NewAppError(ErrCodeInternal, "保存项目元数据失败", err.Error())
	}

	a.mu.Lock()
	a.projects[project.ID] = project
	if a.mediaServer != nil {
		a.mediaServer.Register(project.ID, project.Media.Path)
	}
	a.mu.Unlock()

	return project, nil
}

func (a *App) OpenProject(projectPath string) (*model.Project, error) {
	project, err := a.projectService.OpenProject(projectPath)
	if err != nil {
		return nil, NewAppError(ErrCodeParam, "打开项目失败", err.Error())
	}

	a.mu.Lock()
	a.projects[project.ID] = project
	if a.mediaServer != nil {
		a.mediaServer.Register(project.ID, project.Media.Path)
	}
	a.mu.Unlock()

	return project, nil
}

func (a *App) SaveProject(p model.Project) error {
	err := a.projectService.SaveProject(&p)
	if err != nil {
		return NewAppError(ErrCodeInternal, "保存项目失败", err.Error())
	}

	a.mu.Lock()
	a.projects[p.ID] = &p
	a.mu.Unlock()

	return nil
}

func (a *App) GetProject(projectID string) (*model.Project, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	project, ok := a.projects[projectID]
	if !ok {
		return nil, NewAppError(ErrCodeParam, fmt.Sprintf("项目 %s 未加载", projectID), "")
	}
	return project, nil
}

func (a *App) GetCutList(projectID string) (*model.CutList, error) {
	cl, err := a.editService.GetCutList(projectID)
	if err != nil {
		// 未分析是正常的初始状态，返回 nil 而非错误（避免 wails 刷 ERR 日志）
		return nil, nil
	}
	return cl, nil
}

func (a *App) AddCutSegment(projectID string, seg model.CutSegment) error {
	if err := a.editService.AddCutSegment(projectID, seg); err != nil {
		return NewAppError(ErrCodeInternal, "添加剪切段失败", err.Error())
	}
	return nil
}

func (a *App) UpdateCutSegment(projectID string, seg model.CutSegment) error {
	if err := a.editService.UpdateCutSegment(projectID, seg); err != nil {
		return NewAppError(ErrCodeInternal, "更新剪切段失败", err.Error())
	}
	return nil
}

func (a *App) RemoveCutSegment(projectID string, segID string) error {
	if err := a.editService.RemoveCutSegment(projectID, segID); err != nil {
		return NewAppError(ErrCodeInternal, "删除剪切段失败", err.Error())
	}
	return nil
}

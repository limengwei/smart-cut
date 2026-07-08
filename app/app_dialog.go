package app

import (
	"smart-cut/internal/model"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type FileFilter struct {
	DisplayName string
	Pattern     string
}

func (a *App) PickFile(title string, filters []FileFilter) string {
	dialog := application.Get().Dialog.OpenFile().
		SetTitle(title).
		CanChooseFiles(true).
		CanChooseDirectories(false)

	for _, f := range filters {
		dialog.AddFilter(f.DisplayName, f.Pattern)
	}

	result, err := dialog.PromptForSingleSelection()
	if err != nil {
		return ""
	}
	return result
}

func (a *App) PickDirectory(title string) string {
	dialog := application.Get().Dialog.OpenFile().
		SetTitle(title).
		CanChooseDirectories(true).
		CanChooseFiles(false)

	result, err := dialog.PromptForSingleSelection()
	if err != nil {
		return ""
	}
	return result
}

func (a *App) ListProjects() []*model.Project {
	projects, err := a.projectService.ListProjects()
	if err != nil {
		return []*model.Project{}
	}
	return projects
}
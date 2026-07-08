package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"smart-cut/internal/model"
)

// ProjectService 管理项目生命周期
type ProjectService struct {
	projectsDir string // 项目存储根目录
}

// NewProjectService 创建 ProjectService
func NewProjectService(projectsDir string) *ProjectService {
	if projectsDir == "" {
		projectsDir = filepath.Join(os.TempDir(), "smart-cut-projects")
	}
	return &ProjectService{projectsDir: projectsDir}
}

// CreateProject 创建新项目
func (s *ProjectService) CreateProject(name, mediaPath string) (*model.Project, error) {
	id := fmt.Sprintf("proj-%d", time.Now().UnixMilli())

	workDir := filepath.Join(s.projectsDir, id)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}

	project := &model.Project{
		ID:        id,
		Name:      name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		WorkDir:   workDir,
		Media: model.MediaFile{
			Path: mediaPath,
		},
		Status: model.StatusDraft,
		Settings: model.ProjectSettings{
			ExportMode: model.ExportReencode,
			SilenceMs:  800,
			FillerDict: []string{"嗯", "啊", "那个", "就是说"},
			SubtitleStyle: model.SubtitleStyle{
				FontFamily: "Microsoft YaHei",
				FontSize:   48,
				Color:      "#FFFFFF",
				Highlight:  "#FFD700",
				Position:   "bottom",
				BgColor:    "#000000",
				BgOpacity:  0.5,
			},
		},
	}

	if err := s.SaveProject(project); err != nil {
		return nil, err
	}

	return project, nil
}

// SaveProject 保存项目到文件
func (s *ProjectService) SaveProject(p *model.Project) error {
	p.UpdatedAt = time.Now()

	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("save project: %w", err)
	}

	path := filepath.Join(p.WorkDir, "project.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("save project: %w", err)
	}

	return nil
}

// OpenProject 从文件加载项目
func (s *ProjectService) OpenProject(projectPath string) (*model.Project, error) {
	data, err := os.ReadFile(projectPath)
	if err != nil {
		return nil, fmt.Errorf("open project: %w", err)
	}

	var project model.Project
	if err := json.Unmarshal(data, &project); err != nil {
		return nil, fmt.Errorf("open project: %w", err)
	}

	return &project, nil
}

// OpenProjectByID 通过项目 ID 加载项目
func (s *ProjectService) OpenProjectByID(projectID string) (*model.Project, error) {
	projectPath := filepath.Join(s.projectsDir, projectID, "project.json")
	return s.OpenProject(projectPath)
}

// GetProjectPath 获取项目文件路径
func (s *ProjectService) GetProjectPath(projectID string) string {
	return filepath.Join(s.projectsDir, projectID, "project.json")
}

// ListProjects 扫描项目目录，返回所有项目列表（按更新时间倒序）
func (s *ProjectService) ListProjects() ([]*model.Project, error) {
	entries, err := os.ReadDir(s.projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*model.Project{}, nil
		}
		return nil, fmt.Errorf("list projects: %w", err)
	}

	var projects []*model.Project
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectPath := filepath.Join(s.projectsDir, entry.Name(), "project.json")
		data, err := os.ReadFile(projectPath)
		if err != nil {
			continue
		}

		var project model.Project
		if err := json.Unmarshal(data, &project); err != nil {
			continue
		}

		projects = append(projects, &project)
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].UpdatedAt.After(projects[j].UpdatedAt)
	})

	return projects, nil
}

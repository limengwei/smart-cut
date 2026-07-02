package service

import (
	"fmt"
	"sync"

	"smart-cut/internal/model"
)

// EditService 管理 CutList 的编辑操作
type EditService struct {
	mu        sync.RWMutex
	cutLists  map[string]*model.CutList // projectID → CutList（内存缓存）
}

// NewEditService 创建 EditService
func NewEditService() *EditService {
	return &EditService{
		cutLists: make(map[string]*model.CutList),
	}
}

// GetCutList 获取项目的剪切清单
func (s *EditService) GetCutList(projectID string) (*model.CutList, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cl, ok := s.cutLists[projectID]
	if !ok {
		return nil, fmt.Errorf("cutlist not found for project %s", projectID)
	}
	return cl, nil
}

// SetCutList 设置项目的剪切清单（分析完成后调用）
func (s *EditService) SetCutList(projectID string, cl *model.CutList) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cl.ProjectID = projectID
	s.cutLists[projectID] = cl
}

// AddCutSegment 添加一个剪切段
func (s *EditService) AddCutSegment(projectID string, seg model.CutSegment) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cl, ok := s.cutLists[projectID]
	if !ok {
		return fmt.Errorf("cutlist not found for project %s", projectID)
	}

	seg.Source = model.SourceManual
	cl.Segments = append(cl.Segments, seg)
	cl.Normalize()

	return nil
}

// UpdateCutSegment 更新一个剪切段
func (s *EditService) UpdateCutSegment(projectID string, seg model.CutSegment) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cl, ok := s.cutLists[projectID]
	if !ok {
		return fmt.Errorf("cutlist not found for project %s", projectID)
	}

	for i, existing := range cl.Segments {
		if existing.ID == seg.ID {
			seg.Source = model.SourceManual
			cl.Segments[i] = seg
			cl.Normalize()
			return nil
		}
	}

	return fmt.Errorf("segment %s not found", seg.ID)
}

// RemoveCutSegment 删除一个剪切段
func (s *EditService) RemoveCutSegment(projectID, segID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cl, ok := s.cutLists[projectID]
	if !ok {
		return fmt.Errorf("cutlist not found for project %s", projectID)
	}

	for i, seg := range cl.Segments {
		if seg.ID == segID {
			cl.Segments = append(cl.Segments[:i], cl.Segments[i+1:]...)
			cl.Normalize()
			return nil
		}
	}

	return fmt.Errorf("segment %s not found", segID)
}

// ToggleSegment 切换段的 keep/remove
func (s *EditService) ToggleSegment(projectID, segID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cl, ok := s.cutLists[projectID]
	if !ok {
		return fmt.Errorf("cutlist not found for project %s", projectID)
	}

	for i, seg := range cl.Segments {
		if seg.ID == segID {
			if seg.Decision == model.CutKeep {
				cl.Segments[i].Decision = model.CutRemove
			} else {
				cl.Segments[i].Decision = model.CutKeep
			}
			cl.Segments[i].Source = model.SourceManual
			cl.Normalize()
			return nil
		}
	}

	return fmt.Errorf("segment %s not found", segID)
}

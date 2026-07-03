package service

import (
	"smart-cut/internal/adapter"
)

// SubtitleService 编排字幕渲染（薄封装，实际逻辑在 SubtitleStep）
type SubtitleService struct {
	remotion adapter.RemotionAdapter
}

// NewSubtitleService 创建 SubtitleService
func NewSubtitleService(remotion adapter.RemotionAdapter) *SubtitleService {
	return &SubtitleService{remotion: remotion}
}

// Adapter 暴露给 ExportService 串联 SubtitleStep
func (s *SubtitleService) Adapter() adapter.RemotionAdapter {
	return s.remotion
}

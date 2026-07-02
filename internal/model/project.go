package model

import "time"

// ProjectStatus 表示工程的当前阶段
type ProjectStatus string

const (
	StatusDraft       ProjectStatus = "draft"       // 刚创建，未转录
	StatusTranscribed ProjectStatus = "transcribed" // 已转录
	StatusAnalyzed    ProjectStatus = "analyzed"    // 已分析
	StatusExported    ProjectStatus = "exported"    // 已导出
)

// ExportMode 导出模式
type ExportMode string

const (
	ExportLossless ExportMode = "lossless" // -c copy 流复制
	ExportReencode ExportMode = "reencode" // 重编码
)

// Project 一个剪辑工程
type Project struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
	WorkDir   string          `json:"workDir"`
	Media     MediaFile       `json:"media"`
	Status    ProjectStatus   `json:"status"`
	Settings  ProjectSettings `json:"settings"`
}

// MediaFile 媒体文件元信息
type MediaFile struct {
	Path          string  `json:"path"`
	DurationMs    int64   `json:"durationMs"`
	Format        string  `json:"format"`
	Width         int     `json:"width"`
	Height        int     `json:"height"`
	Fps           float64 `json:"fps"`
	HasAudio      bool    `json:"hasAudio"`
	ColorTransfer string  `json:"colorTransfer"` // 颜色传输函数,如 smpte2084(PQ)/arib-std-b67(HLG),用于 HDR 检测
}

// ProjectSettings 工程级设置
type ProjectSettings struct {
	ExportMode    ExportMode    `json:"exportMode"`
	SilenceMs     int           `json:"silenceMs"`
	FillerDict    []string      `json:"fillerDict"`
	LLMConfig     LLMConfig     `json:"llmConfig"`
	SubtitleStyle SubtitleStyle `json:"subtitleStyle"`
}

// SubtitleStyle 字幕样式
type SubtitleStyle struct {
	FontFamily string  `json:"fontFamily"`
	FontSize   int     `json:"fontSize"`
	Color      string  `json:"color"`     // hex 如 #FFFFFF
	Highlight  string  `json:"highlight"` // 当前词高亮色
	Position   string  `json:"position"`  // bottom/center/top
	BgColor    string  `json:"bgColor"`   // 背景色，可透明
	BgOpacity  float64 `json:"bgOpacity"` // 0-1
}

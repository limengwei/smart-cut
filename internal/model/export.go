package model

// ExportOptions 导出选项
type ExportOptions struct {
	Mode            ExportMode `json:"mode"`            // lossless/reencode
	IncludeSubtitle bool       `json:"includeSubtitle"` // 是否合成字幕
	OutputPath      string     `json:"outputPath"`
}

// EncodeOpts 重编码参数
type EncodeOpts struct {
	VideoCodec   string `json:"videoCodec"`   // 如 libx264
	AudioCodec   string `json:"audioCodec"`   // 如 aac
	VideoBitrate string `json:"videoBitrate"` // 如 2M
	Crf          int    `json:"crf"`          // 0-51，越小质量越高，默认 23
	Preset       string `json:"preset"`       // ultrafast...veryslow
}

// SubtitleRenderRequest Remotion 字幕渲染请求
type SubtitleRenderRequest struct {
	Words     []Word        `json:"words"`
	CutList   *CutList      `json:"cutList"`
	Style     SubtitleStyle `json:"style"`
	OutputDir string        `json:"outputDir"`
}

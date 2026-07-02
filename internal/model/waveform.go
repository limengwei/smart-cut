package model

// WaveformPeaks 波形峰值采样数据（供前端 canvas 渲染）
// 每个桶存储 PCM int16 的 min/max，前端归一化时除以 32768
type WaveformPeaks struct {
	DurationMs int64   `json:"durationMs"` // 媒体总时长（毫秒）
	SampleRate int     `json:"sampleRate"` // PCM 采样率（Hz）
	Buckets    int     `json:"buckets"`    // 桶数量
	Mins       []int16 `json:"mins"`       // 每桶最小值
	Maxs       []int16 `json:"maxs"`       // 每桶最大值
}

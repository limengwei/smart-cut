package model

// LLMAnalysisRequest 送入 LLM 的请求（只送句级文本+时间，节省 token）
type LLMAnalysisRequest struct {
	Language string          `json:"language"`
	Segments []LLMSegment    `json:"segments"`
	Settings ProjectSettings `json:"settings"`
}

// LLMSegment 送入 LLM 的句段简化结构
type LLMSegment struct {
	ID      int    `json:"id"`
	StartMs int64  `json:"startMs"`
	EndMs   int64  `json:"endMs"`
	Text    string `json:"text"`
}

// LLMAnalysisResult LLM 返回的分析结果
type LLMAnalysisResult struct {
	RemoveSegmentIDs []int             `json:"removeSegmentIds"`
	Items            []LLMAnalysisItem `json:"items"`
}

// LLMAnalysisItem 单个分析项
type LLMAnalysisItem struct {
	SegmentID  int       `json:"segmentId"`
	Reason     CutReason `json:"reason"`
	Confidence float64   `json:"confidence"`
	Note       string    `json:"note"`
}

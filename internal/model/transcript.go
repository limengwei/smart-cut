package model

// Transcript 转录结果
type Transcript struct {
	Language string    `json:"language"` // zh/en/...
	Words    []Word    `json:"words"`
	Segments []Segment `json:"segments"`
}

// Word 词级单元（带时间戳）
type Word struct {
	Text       string  `json:"text"`
	StartMs    int64   `json:"startMs"`
	EndMs      int64   `json:"endMs"`
	Confidence float64 `json:"confidence"`
}

// Segment 句级单元
type Segment struct {
	ID      int    `json:"id"`
	StartMs int64  `json:"startMs"`
	EndMs   int64  `json:"endMs"`
	Text    string `json:"text"`
	WordIDs []int  `json:"wordIds"` // 指向 Words 的索引
}

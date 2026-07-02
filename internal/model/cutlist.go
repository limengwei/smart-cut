package model

// CutDecision 剪切决定
type CutDecision string

const (
	CutKeep   CutDecision = "keep"
	CutRemove CutDecision = "remove"
)

// CutReason 剪切原因
type CutReason string

const (
	ReasonFiller     CutReason = "filler"
	ReasonSilence    CutReason = "silence"
	ReasonDupOrError CutReason = "dup_or_error"
	ReasonManual     CutReason = "manual"
)

// CutSource 来源（AI 或手动）
type CutSource string

const (
	SourceAI     CutSource = "ai"
	SourceManual CutSource = "manual"
)

// CutSegment 一个剪切时间段
type CutSegment struct {
	ID         string      `json:"id"`
	StartMs    int64       `json:"startMs"`
	EndMs      int64       `json:"endMs"`
	Decision   CutDecision `json:"decision"`
	Reason     CutReason   `json:"reason"`
	Source     CutSource   `json:"source"`
	Confidence float64     `json:"confidence"`
	Note       string      `json:"note"`
}

// CutList 剪切清单（核心数据，贯穿全流程）
type CutList struct {
	ProjectID string       `json:"projectId"`
	Segments  []CutSegment `json:"segments"`
	Version   int          `json:"version"`
}

// KeepSegment 导出时用于 ffmpeg 的保留段（只有起止时间）
type KeepSegment struct {
	StartMs int64 `json:"startMs"`
	EndMs   int64 `json:"endMs"`
}

// KeepSegments 从 CutList 提取所有 keep 段，返回 KeepSegment 列表
func (cl *CutList) KeepSegments() []KeepSegment {
	var result []KeepSegment
	for _, seg := range cl.Segments {
		if seg.Decision == CutKeep {
			result = append(result, KeepSegment{
				StartMs: seg.StartMs,
				EndMs:   seg.EndMs,
			})
		}
	}
	return result
}

// Normalize 规范化 CutList：
// 1. 按 StartMs 升序排序
// 2. 合并相邻同 Decision 的段
// 3. 裁剪重叠段
func (cl *CutList) Normalize() {
	if len(cl.Segments) == 0 {
		return
	}

	// 1. 按 StartMs 排序（插入排序，段数通常不大）
	for i := 1; i < len(cl.Segments); i++ {
		for j := i; j > 0 && cl.Segments[j].StartMs < cl.Segments[j-1].StartMs; j-- {
			cl.Segments[j], cl.Segments[j-1] = cl.Segments[j-1], cl.Segments[j]
		}
	}

	// 2. 裁剪重叠 + 合并相邻同 Decision
	normalized := []CutSegment{cl.Segments[0]}
	for i := 1; i < len(cl.Segments); i++ {
		last := &normalized[len(normalized)-1]
		curr := cl.Segments[i]

		if curr.StartMs < last.EndMs {
			// 重叠：裁剪 curr 的起点到 last 的终点
			curr.StartMs = last.EndMs
			if curr.StartMs >= curr.EndMs {
				continue // 裁剪后无效，跳过
			}
		}

		if curr.Decision == last.Decision && curr.StartMs == last.EndMs {
			// 相邻同 Decision：合并
			last.EndMs = curr.EndMs
		} else {
			normalized = append(normalized, curr)
		}
	}
	cl.Segments = normalized
	cl.Version++
}

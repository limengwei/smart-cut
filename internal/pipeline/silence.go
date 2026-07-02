package pipeline

import (
	"fmt"

	"smart-cut/internal/model"
)

// SilenceDetector 检测沉默/停顿段
type SilenceDetector struct {
	thresholdMs int64
}

// NewSilenceDetector 创建沉默检测器
func NewSilenceDetector(thresholdMs int) *SilenceDetector {
	if thresholdMs <= 0 {
		thresholdMs = 800
	}
	return &SilenceDetector{thresholdMs: int64(thresholdMs)}
}

// Detect 检测转录结果中的沉默段
func (d *SilenceDetector) Detect(transcript *model.Transcript) []model.CutSegment {
	if transcript == nil || len(transcript.Segments) < 2 {
		return nil
	}

	var cuts []model.CutSegment

	for i := 1; i < len(transcript.Segments); i++ {
		prev := transcript.Segments[i-1]
		curr := transcript.Segments[i]

		gap := curr.StartMs - prev.EndMs
		if gap >= d.thresholdMs {
			cuts = append(cuts, model.CutSegment{
				ID:         fmt.Sprintf("silence-%d", i),
				StartMs:    prev.EndMs,
				EndMs:      curr.StartMs,
				Decision:   model.CutRemove,
				Reason:     model.ReasonSilence,
				Source:     model.SourceAI,
				Confidence: 0.9,
				Note:       fmt.Sprintf("沉默 %dms", gap),
			})
		}
	}

	return cuts
}

// mergeAnalysisResults 合并规则检测结果和 LLM 分析结果
func mergeAnalysisResults(transcript *model.Transcript, ruleCuts []model.CutSegment, llmResult *model.LLMAnalysisResult) *model.CutList {
	removeMap := make(map[int]model.LLMAnalysisItem)
	if llmResult != nil {
		for _, item := range llmResult.Items {
			removeMap[item.SegmentID] = item
		}
	}

	var segments []model.CutSegment

	if transcript != nil {
		for _, seg := range transcript.Segments {
			if item, ok := removeMap[seg.ID]; ok {
				segments = append(segments, model.CutSegment{
					ID:         fmt.Sprintf("llm-%d", seg.ID),
					StartMs:    seg.StartMs,
					EndMs:      seg.EndMs,
					Decision:   model.CutRemove,
					Reason:     item.Reason,
					Source:     model.SourceAI,
					Confidence: item.Confidence,
					Note:       item.Note,
				})
			}
		}
	}

	segments = append(segments, ruleCuts...)

	cutList := &model.CutList{
		Segments: segments,
	}
	cutList.Normalize()

	cutList = fillKeepSegments(cutList, transcript)

	return cutList
}

// fillKeepSegments 在 remove 段之间填充 keep 段
func fillKeepSegments(cutList *model.CutList, transcript *model.Transcript) *model.CutList {
	if transcript == nil || len(transcript.Segments) == 0 {
		return cutList
	}

	totalStart := transcript.Segments[0].StartMs
	totalEnd := transcript.Segments[len(transcript.Segments)-1].EndMs

	if len(cutList.Segments) == 0 {
		cutList.Segments = []model.CutSegment{{
			ID:       "keep-all",
			StartMs:  totalStart,
			EndMs:    totalEnd,
			Decision: model.CutKeep,
			Source:   model.SourceAI,
		}}
		return cutList
	}

	var result []model.CutSegment

	if cutList.Segments[0].StartMs > totalStart {
		result = append(result, model.CutSegment{
			ID:       "keep-0",
			StartMs:  totalStart,
			EndMs:    cutList.Segments[0].StartMs,
			Decision: model.CutKeep,
			Source:   model.SourceAI,
		})
	}

	for i, seg := range cutList.Segments {
		result = append(result, seg)

		if i < len(cutList.Segments)-1 {
			next := cutList.Segments[i+1]
			if seg.EndMs < next.StartMs {
				result = append(result, model.CutSegment{
					ID:       fmt.Sprintf("keep-%d", i+1),
					StartMs:  seg.EndMs,
					EndMs:    next.StartMs,
					Decision: model.CutKeep,
					Source:   model.SourceAI,
				})
			}
		}
	}

	last := cutList.Segments[len(cutList.Segments)-1]
	if last.EndMs < totalEnd {
		result = append(result, model.CutSegment{
			ID:       "keep-end",
			StartMs:  last.EndMs,
			EndMs:    totalEnd,
			Decision: model.CutKeep,
			Source:   model.SourceAI,
		})
	}

	cutList.Segments = result
	return cutList
}

package pipeline

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"smart-cut/internal/adapter"
	"smart-cut/internal/model"
)

type OverlayStep struct {
	remotion adapter.RemotionAdapter
}

func NewOverlayStep(remotion adapter.RemotionAdapter) *OverlayStep {
	return &OverlayStep{remotion: remotion}
}

func (s *OverlayStep) Name() string { return "overlay" }

func (s *OverlayStep) Run(ctx *Context, reporter ProgressReporter) error {
	overlayItems := ctx.Project.Settings.OverlayItems
	if len(overlayItems) == 0 {
		reporter.Report("overlay", "no overlay items, skipping", 1.0)
		return nil
	}
	if ctx.CutList == nil {
		reporter.Report("overlay", "no cutlist, skipping", 1.0)
		return nil
	}

	keepSegments := ctx.CutList.KeepSegments()
	if len(keepSegments) == 0 {
		reporter.Report("overlay", "no keep segments", 1.0)
		return nil
	}

	reporter.Report("overlay", fmt.Sprintf("rendering %d segments", len(keepSegments)), 0.0)

	clipDir := filepath.Join(ctx.Project.WorkDir, "overlay_clips")
	if err := os.MkdirAll(clipDir, 0755); err != nil {
		return fmt.Errorf("overlay: create clip dir: %w", err)
	}

	clips := make(map[string]string)
	completed := 0
	total := len(keepSegments)

	for i, seg := range keepSegments {
		segID := fmt.Sprintf("%03d", i+1)

		var relItems []model.OverlayItem
		for _, item := range overlayItems {
			if item.EndMs <= seg.StartMs || item.StartMs >= seg.EndMs {
				continue
			}
			start := item.StartMs - seg.StartMs
			if start < 0 {
				start = 0
			}
			end := item.EndMs - seg.StartMs
			if end > seg.EndMs-seg.StartMs {
				end = seg.EndMs - seg.StartMs
			}
			relItems = append(relItems, model.OverlayItem{
				ID:        item.ID,
				Type:      item.Type,
				Mode:      item.Mode,
				StartMs:   start,
				EndMs:     end,
				Animation: item.Animation,
				Position:  item.Position,
				Content:   item.Content,
			})
		}

		if len(relItems) == 0 {
			completed++
			continue
		}

		req := adapter.OverlaySegmentRequest{
			SegmentID:    segID,
			StartMs:      seg.StartMs,
			EndMs:        seg.EndMs,
			OverlayItems: relItems,
			Style:        ctx.Project.Settings.OverlayStyle,
			Width:        ctx.Project.Media.Width,
			Height:       ctx.Project.Media.Height,
			Fps:          ctx.Project.Media.Fps,
			OutputDir:    clipDir,
		}

		clipPath, err := s.remotion.RenderOverlaySegment(ctx.Cancel, req, func(ratio float64) {
			overall := (float64(completed) + ratio) / float64(total)
			reporter.Report("overlay", fmt.Sprintf("rendering %d/%d", i+1, total), overall)
		})
		if err != nil {
			log.Printf("[Overlay] 段 %d 渲染失败，跳过该段 overlay: %v", i+1, err)
			completed++
			continue
		}
		clips[segID] = clipPath
		completed++
		reporter.Report("overlay", fmt.Sprintf("rendered %d/%d", i+1, total), float64(completed)/float64(total))
	}

	ctx.OverlayClips = clips
	reporter.Report("overlay", fmt.Sprintf("completed (%d/%d succeeded)", len(clips), total), 1.0)
	return nil
}
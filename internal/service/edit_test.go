package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smart-cut/internal/model"
)

func TestEditService_SetAndGetCutList(t *testing.T) {
	svc := NewEditService()

	cl := &model.CutList{
		Segments: []model.CutSegment{
			{ID: "1", StartMs: 0, EndMs: 1000, Decision: model.CutKeep},
		},
	}

	svc.SetCutList("p1", cl)

	result, err := svc.GetCutList("p1")
	require.NoError(t, err)
	assert.Equal(t, "p1", result.ProjectID)
	assert.Len(t, result.Segments, 1)
}

func TestEditService_GetCutList_NotFound(t *testing.T) {
	svc := NewEditService()

	_, err := svc.GetCutList("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestEditService_AddCutSegment(t *testing.T) {
	svc := NewEditService()
	svc.SetCutList("p1", &model.CutList{
		Segments: []model.CutSegment{
			{ID: "keep-1", StartMs: 0, EndMs: 1000, Decision: model.CutKeep},
		},
	})

	err := svc.AddCutSegment("p1", model.CutSegment{
		ID:       "manual-1",
		StartMs:  2000,
		EndMs:    3000,
		Decision: model.CutRemove,
	})
	require.NoError(t, err)

	cl, _ := svc.GetCutList("p1")
	assert.Len(t, cl.Segments, 2)

	// 新段应标记为 manual
	var manualSeg *model.CutSegment
	for i := range cl.Segments {
		if cl.Segments[i].ID == "manual-1" {
			manualSeg = &cl.Segments[i]
		}
	}
	require.NotNil(t, manualSeg)
	assert.Equal(t, model.SourceManual, manualSeg.Source)
}

func TestEditService_UpdateCutSegment(t *testing.T) {
	svc := NewEditService()
	svc.SetCutList("p1", &model.CutList{
		Segments: []model.CutSegment{
			{ID: "seg-1", StartMs: 0, EndMs: 1000, Decision: model.CutKeep},
		},
	})

	err := svc.UpdateCutSegment("p1", model.CutSegment{
		ID:       "seg-1",
		StartMs:  0,
		EndMs:    2000,
		Decision: model.CutRemove,
	})
	require.NoError(t, err)

	cl, _ := svc.GetCutList("p1")
	assert.Equal(t, int64(2000), cl.Segments[0].EndMs)
	assert.Equal(t, model.CutRemove, cl.Segments[0].Decision)
	assert.Equal(t, model.SourceManual, cl.Segments[0].Source)
}

func TestEditService_UpdateCutSegment_NotFound(t *testing.T) {
	svc := NewEditService()
	svc.SetCutList("p1", &model.CutList{})

	err := svc.UpdateCutSegment("p1", model.CutSegment{ID: "nonexistent"})
	assert.Error(t, err)
}

func TestEditService_RemoveCutSegment(t *testing.T) {
	svc := NewEditService()
	svc.SetCutList("p1", &model.CutList{
		Segments: []model.CutSegment{
			{ID: "seg-1", StartMs: 0, EndMs: 1000, Decision: model.CutKeep},
			{ID: "seg-2", StartMs: 1000, EndMs: 2000, Decision: model.CutRemove},
		},
	})

	err := svc.RemoveCutSegment("p1", "seg-2")
	require.NoError(t, err)

	cl, _ := svc.GetCutList("p1")
	assert.Len(t, cl.Segments, 1)
	assert.Equal(t, "seg-1", cl.Segments[0].ID)
}

func TestEditService_ToggleSegment(t *testing.T) {
	svc := NewEditService()
	svc.SetCutList("p1", &model.CutList{
		Segments: []model.CutSegment{
			{ID: "seg-1", Decision: model.CutKeep},
		},
	})

	// keep -> remove
	err := svc.ToggleSegment("p1", "seg-1")
	require.NoError(t, err)

	cl, _ := svc.GetCutList("p1")
	assert.Equal(t, model.CutRemove, cl.Segments[0].Decision)

	// remove -> keep
	err = svc.ToggleSegment("p1", "seg-1")
	require.NoError(t, err)

	cl, _ = svc.GetCutList("p1")
	assert.Equal(t, model.CutKeep, cl.Segments[0].Decision)
}

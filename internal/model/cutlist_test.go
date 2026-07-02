package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeepSegments_OnlyReturnsKeep(t *testing.T) {
	cl := &CutList{
		Segments: []CutSegment{
			{ID: "1", StartMs: 0, EndMs: 1000, Decision: CutKeep},
			{ID: "2", StartMs: 1000, EndMs: 1500, Decision: CutRemove},
			{ID: "3", StartMs: 1500, EndMs: 3000, Decision: CutKeep},
		},
	}

	keeps := cl.KeepSegments()

	assert.Len(t, keeps, 2)
	assert.Equal(t, int64(0), keeps[0].StartMs)
	assert.Equal(t, int64(1000), keeps[0].EndMs)
	assert.Equal(t, int64(1500), keeps[1].StartMs)
	assert.Equal(t, int64(3000), keeps[1].EndMs)
}

func TestKeepSegments_EmptyList(t *testing.T) {
	cl := &CutList{Segments: []CutSegment{}}
	keeps := cl.KeepSegments()
	assert.Empty(t, keeps)
}

func TestNormalize_SortsByStartMs(t *testing.T) {
	cl := &CutList{
		Segments: []CutSegment{
			{ID: "3", StartMs: 2000, EndMs: 3000, Decision: CutKeep},
			{ID: "1", StartMs: 0, EndMs: 1000, Decision: CutKeep},
			{ID: "2", StartMs: 1000, EndMs: 2000, Decision: CutRemove},
		},
	}

	cl.Normalize()

	assert.Equal(t, "1", cl.Segments[0].ID)
	assert.Equal(t, "2", cl.Segments[1].ID)
	assert.Equal(t, "3", cl.Segments[2].ID)
}

func TestNormalize_MergesAdjacentSameDecision(t *testing.T) {
	cl := &CutList{
		Segments: []CutSegment{
			{ID: "1", StartMs: 0, EndMs: 1000, Decision: CutKeep},
			{ID: "2", StartMs: 1000, EndMs: 2000, Decision: CutKeep},
		},
	}

	cl.Normalize()

	assert.Len(t, cl.Segments, 1)
	assert.Equal(t, int64(0), cl.Segments[0].StartMs)
	assert.Equal(t, int64(2000), cl.Segments[0].EndMs)
}

func TestNormalize_TrimsOverlap(t *testing.T) {
	cl := &CutList{
		Segments: []CutSegment{
			{ID: "1", StartMs: 0, EndMs: 1500, Decision: CutKeep},
			{ID: "2", StartMs: 1000, EndMs: 2000, Decision: CutRemove},
		},
	}

	cl.Normalize()

	// 第一段保持 0-1500
	assert.Equal(t, int64(0), cl.Segments[0].StartMs)
	assert.Equal(t, int64(1500), cl.Segments[0].EndMs)
	// 第二段被裁剪为 1500-2000
	assert.Equal(t, int64(1500), cl.Segments[1].StartMs)
	assert.Equal(t, int64(2000), cl.Segments[1].EndMs)
}

func TestNormalize_DropsFullyContainedSegment(t *testing.T) {
	cl := &CutList{
		Segments: []CutSegment{
			{ID: "1", StartMs: 0, EndMs: 3000, Decision: CutKeep},
			{ID: "2", StartMs: 1000, EndMs: 2000, Decision: CutRemove},
		},
	}

	cl.Normalize()

	// 第二段被完全包含，裁剪后无效，应被丢弃
	assert.Len(t, cl.Segments, 1)
	assert.Equal(t, "1", cl.Segments[0].ID)
}

func TestNormalize_IncrementsVersion(t *testing.T) {
	cl := &CutList{
		Segments: []CutSegment{
			{ID: "1", StartMs: 0, EndMs: 1000, Decision: CutKeep},
		},
		Version: 0,
	}

	cl.Normalize()

	assert.Equal(t, 1, cl.Version)
}

func TestNormalize_EmptyList(t *testing.T) {
	cl := &CutList{Segments: []CutSegment{}}
	cl.Normalize()
	assert.Empty(t, cl.Segments)
}

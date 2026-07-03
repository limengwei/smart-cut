package adapter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildOverlayFilter_ContainsOverlay(t *testing.T) {
	filter := buildOverlayFilter()
	assert.Contains(t, filter, "overlay=0:0")
}

func TestBuildOverlayFilter_ContainsFormat(t *testing.T) {
	filter := buildOverlayFilter()
	assert.Contains(t, filter, "format=rgba")
}

func TestBuildOverlayFilter_FormatsOverlayFirst(t *testing.T) {
	filter := buildOverlayFilter()
	formatIdx := 0
	overlayIdx := 0
	for i := 0; i < len(filter); i++ {
		if i+len("format=rgba") <= len(filter) && filter[i:i+len("format=rgba")] == "format=rgba" {
			formatIdx = i
		}
		if i+len("overlay=0:0") <= len(filter) && filter[i:i+len("overlay=0:0")] == "overlay=0:0" {
			overlayIdx = i
		}
	}
	assert.True(t, formatIdx < overlayIdx, "format=rgba should come before overlay=0:0")
}
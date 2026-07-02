package adapter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComputePeaks_BasicBuckets(t *testing.T) {
	samples := []int16{100, -200, 300, -400, 500, -600, 700, -800}
	mins, maxs := computePeaks(samples, 2)

	assert.Len(t, mins, 4)
	assert.Len(t, maxs, 4)

	assert.Equal(t, int16(-200), mins[0])
	assert.Equal(t, int16(100), maxs[0])

	assert.Equal(t, int16(-400), mins[1])
	assert.Equal(t, int16(300), maxs[1])

	assert.Equal(t, int16(-600), mins[2])
	assert.Equal(t, int16(500), maxs[2])

	assert.Equal(t, int16(-800), mins[3])
	assert.Equal(t, int16(700), maxs[3])
}

func TestComputePeaks_LastBucketPartial(t *testing.T) {
	samples := []int16{10, 20, 30, 40, 50}
	mins, maxs := computePeaks(samples, 2)

	assert.Len(t, mins, 3)
	assert.Len(t, maxs, 3)

	assert.Equal(t, int16(10), mins[0])
	assert.Equal(t, int16(20), maxs[0])
	assert.Equal(t, int16(50), mins[2])
	assert.Equal(t, int16(50), maxs[2])
}

func TestComputePeaks_SamplesPerBucketClampedTo1(t *testing.T) {
	samples := []int16{5, -5, 10, -10}
	mins, maxs := computePeaks(samples, 0)

	assert.Len(t, mins, 4)
	assert.Equal(t, int16(5), maxs[0])
	assert.Equal(t, int16(-5), mins[1])
}

func TestComputePeaks_EmptySamples(t *testing.T) {
	mins, maxs := computePeaks([]int16{}, 2)
	assert.Empty(t, mins)
	assert.Empty(t, maxs)
}

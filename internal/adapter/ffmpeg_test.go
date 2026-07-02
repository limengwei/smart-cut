package adapter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFFprobeJSON_ValidOutput(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "ffprobe_output.json"))
	require.NoError(t, err)

	media, err := parseFFprobeJSON(data)
	require.NoError(t, err)

	assert.Equal(t, 1920, media.Width)
	assert.Equal(t, 1080, media.Height)
	assert.Equal(t, 30.0, media.Fps)
	assert.Equal(t, int64(10500), media.DurationMs)
	assert.True(t, media.HasAudio)
	assert.Equal(t, "mov,mp4,m4a,3gp,3g2,mj2", media.Format)
	assert.Equal(t, "bt709", media.ColorTransfer)
}

func TestParseFFprobeJSON_NoAudio(t *testing.T) {
	data := []byte(`{
		"streams": [
			{"codec_type": "video", "width": 1280, "height": 720, "r_frame_rate": "25/1", "duration": "5.0"}
		],
		"format": {"duration": "5.0", "format_name": "mp4"}
	}`)

	media, err := parseFFprobeJSON(data)
	require.NoError(t, err)

	assert.False(t, media.HasAudio)
	assert.Equal(t, 1280, media.Width)
	assert.Equal(t, int64(5000), media.DurationMs)
	assert.Equal(t, "", media.ColorTransfer)
}

func TestParseFFprobeJSON_InvalidJSON(t *testing.T) {
	_, err := parseFFprobeJSON([]byte(`{invalid}`))
	assert.Error(t, err)
}

func TestParseFFprobeJSON_HDRTransfer(t *testing.T) {
	data := []byte(`{
		"streams": [
			{"codec_type": "video", "width": 1920, "height": 1080, "r_frame_rate": "30/1", "duration": "10.5", "color_transfer": "arib-std-b67"}
		],
		"format": {"duration": "10.5", "format_name": "mp4"}
	}`)

	media, err := parseFFprobeJSON(data)
	require.NoError(t, err)
	assert.Equal(t, "arib-std-b67", media.ColorTransfer)
}

func TestParseFrameRate_Fraction(t *testing.T) {
	assert.Equal(t, 30.0, parseFrameRate("30/1"))
	// 30000/1001 = 29.97002997...，用 InDelta 做浮点近似比较
	assert.InDelta(t, 29.97, parseFrameRate("30000/1001"), 0.001)
}

func TestParseFrameRate_Decimal(t *testing.T) {
	assert.Equal(t, 24.0, parseFrameRate("24"))
}

func TestParseFrameRate_Empty(t *testing.T) {
	assert.Equal(t, 0.0, parseFrameRate(""))
}

func TestParseDurationMs_Valid(t *testing.T) {
	assert.Equal(t, int64(10500), parseDurationMs("10.500000"))
	assert.Equal(t, int64(1000), parseDurationMs("1.0"))
}

func TestParseDurationMs_Empty(t *testing.T) {
	assert.Equal(t, int64(0), parseDurationMs(""))
}

func TestParseFFmpegProgress_ValidTime(t *testing.T) {
	// 标准 4 位 time= 格式
	ms := ParseFFmpegProgress("frame=  123 fps=30 q=24.0 size=     256kB time=00:01:23.45 bitrate=  30.0kbits/s")
	assert.Equal(t, int64(83450), ms)
}

func TestParseFFmpegProgress_HoursMinutesSeconds(t *testing.T) {
	ms := ParseFFmpegProgress("time=01:02:03.50")
	assert.Equal(t, int64(3723500), ms)
}

func TestParseFFmpegProgress_NoMatch(t *testing.T) {
	ms := ParseFFmpegProgress("some random stderr line")
	assert.Equal(t, int64(-1), ms)
}

func TestParseFFmpegProgress_EmptyLine(t *testing.T) {
	ms := ParseFFmpegProgress("")
	assert.Equal(t, int64(-1), ms)
}

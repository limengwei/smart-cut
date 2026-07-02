package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"smart-cut/internal/model"
)

// FFmpegAdapter 定义视频处理接口
type FFmpegAdapter interface {
	Probe(ctx context.Context, path string) (*model.MediaFile, error)
	ExtractWaveform(ctx context.Context, mediaPath, outPng string) error
	ConcatLossless(ctx context.Context, segments []model.KeepSegment, sourcePath, outPath string) error
	ConcatReencode(ctx context.Context, segments []model.KeepSegment, sourcePath, outPath string, opts model.EncodeOpts) error
	MuxSubtitle(ctx context.Context, videoPath, subtitleClipPath, outPath string) error
}

// ffmpegAdapter 是 FFmpegAdapter 的具体实现
type ffmpegAdapter struct {
	resolver *BinaryResolver
}

// NewFFmpegAdapter 创建基于 ffmpeg/ffprobe 二进制的 Adapter
func NewFFmpegAdapter(resolver *BinaryResolver) FFmpegAdapter {
	return &ffmpegAdapter{resolver: resolver}
}

// ffprobeJSONOutput 是 ffprobe -of json 的输出结构
type ffprobeJSONOutput struct {
	Streams []struct {
		CodecType  string `json:"codec_type"`
		CodecName  string `json:"codec_name"`
		Width      int    `json:"width"`
		Height     int    `json:"height"`
		RFrameRate string `json:"r_frame_rate"`
		Duration   string `json:"duration"`
	} `json:"streams"`
	Format struct {
		Duration   string `json:"duration"`
		FormatName string `json:"format_name"`
	} `json:"format"`
}

func (a *ffmpegAdapter) Probe(ctx context.Context, path string) (*model.MediaFile, error) {
	probePath, err := a.resolver.Resolve("ffprobe")
	if err != nil {
		return nil, fmt.Errorf("ffprobe: %w", err)
	}

	cmd := exec.CommandContext(ctx, probePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		path,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe: %w", err)
	}

	return parseFFprobeJSON(output)
}

// parseFFprobeJSON 解析 ffprobe JSON 输出为 MediaFile
func parseFFprobeJSON(data []byte) (*model.MediaFile, error) {
	var raw ffprobeJSONOutput
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("ffprobe parse: %w", err)
	}

	media := &model.MediaFile{}

	// 从 streams 提取视频和音频信息
	for _, stream := range raw.Streams {
		if stream.CodecType == "video" {
			media.Width = stream.Width
			media.Height = stream.Height
			media.Fps = parseFrameRate(stream.RFrameRate)
		}
		if stream.CodecType == "audio" {
			media.HasAudio = true
		}
	}

	// 从 format 提取时长和格式
	media.DurationMs = parseDurationMs(raw.Format.Duration)
	media.Format = raw.Format.FormatName

	return media, nil
}

// parseFrameRate 解析 ffprobe 的帧率字符串（如 "30000/1001"）
func parseFrameRate(rate string) float64 {
	if rate == "" {
		return 0
	}
	parts := strings.Split(rate, "/")
	if len(parts) == 2 {
		num, err1 := strconv.ParseFloat(parts[0], 64)
		den, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 == nil && err2 == nil && den != 0 {
			return num / den
		}
	}
	f, err := strconv.ParseFloat(rate, 64)
	if err != nil {
		return 0
	}
	return f
}

// parseDurationMs 解析 ffprobe 的时长字符串（秒，浮点）转为毫秒
func parseDurationMs(duration string) int64 {
	if duration == "" {
		return 0
	}
	f, err := strconv.ParseFloat(duration, 64)
	if err != nil {
		return 0
	}
	return int64(f * 1000)
}

func (a *ffmpegAdapter) ExtractWaveform(ctx context.Context, mediaPath, outPng string) error {
	binaryPath, err := a.resolver.Resolve("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg waveform: %w", err)
	}

	// 生成波形图：转为单声道，用 showwavespic 滤镜
	cmd := exec.CommandContext(ctx, binaryPath,
		"-i", mediaPath,
		"-filter_complex", "showwavespic=s=1280x120:colors=white",
		"-frames:v", "1",
		"-y",
		outPng,
	)
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg waveform: %w", err)
	}
	return nil
}

func (a *ffmpegAdapter) ConcatLossless(ctx context.Context, segments []model.KeepSegment, sourcePath, outPath string) error {
	binaryPath, err := a.resolver.Resolve("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg concat lossless: %w", err)
	}

	if len(segments) == 0 {
		return fmt.Errorf("ffmpeg concat: no segments to concat")
	}

	// 构建 filter_complex
	var filters []string
	for i, seg := range segments {
		startSec := float64(seg.StartMs) / 1000.0
		endSec := float64(seg.EndMs) / 1000.0
		filters = append(filters, fmt.Sprintf("[0:v]trim=start=%f:end=%f,setpts=PTS-STARTPTS[v%d];[0:a]atrim=start=%f:end=%f,asetpts=PTS-STARTPTS[a%d]",
			startSec, endSec, i, startSec, endSec, i))
	}

	// 连接所有段
	var concatV []string
	var concatA []string
	for i := range segments {
		concatV = append(concatV, fmt.Sprintf("[v%d]", i))
		concatA = append(concatA, fmt.Sprintf("[a%d]", i))
	}
	concatFilter := strings.Join(concatV, "") + fmt.Sprintf("concat=n=%d:v=1:a=0[vout]", len(segments)) +
		";" + strings.Join(concatA, "") + fmt.Sprintf("concat=n=%d:v=0:a=1[aout]", len(segments))

	filterComplex := strings.Join(filters, ";") + ";" + concatFilter

	args := []string{
		"-i", sourcePath,
		"-filter_complex", filterComplex,
		"-map", "[vout]",
		"-map", "[aout]",
		"-c:v", "copy",
		"-c:a", "copy",
		"-y",
		outPath,
	}

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg concat lossless: %w", err)
	}
	return nil
}

func (a *ffmpegAdapter) ConcatReencode(ctx context.Context, segments []model.KeepSegment, sourcePath, outPath string, opts model.EncodeOpts) error {
	binaryPath, err := a.resolver.Resolve("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg concat reencode: %w", err)
	}

	if len(segments) == 0 {
		return fmt.Errorf("ffmpeg concat: no segments to concat")
	}

	// 构建 filter_complex（同 lossless，但输出用重编码）
	var filters []string
	for i, seg := range segments {
		startSec := float64(seg.StartMs) / 1000.0
		endSec := float64(seg.EndMs) / 1000.0
		filters = append(filters, fmt.Sprintf("[0:v]trim=start=%f:end=%f,setpts=PTS-STARTPTS[v%d];[0:a]atrim=start=%f:end=%f,asetpts=PTS-STARTPTS[a%d]",
			startSec, endSec, i, startSec, endSec, i))
	}

	var concatV []string
	var concatA []string
	for i := range segments {
		concatV = append(concatV, fmt.Sprintf("[v%d]", i))
		concatA = append(concatA, fmt.Sprintf("[a%d]", i))
	}
	concatFilter := strings.Join(concatV, "") + fmt.Sprintf("concat=n=%d:v=1:a=0[vout]", len(segments)) +
		";" + strings.Join(concatA, "") + fmt.Sprintf("concat=n=%d:v=0:a=1[aout]", len(segments))

	filterComplex := strings.Join(filters, ";") + ";" + concatFilter

	videoCodec := opts.VideoCodec
	if videoCodec == "" {
		videoCodec = "libx264"
	}
	audioCodec := opts.AudioCodec
	if audioCodec == "" {
		audioCodec = "aac"
	}
	crf := opts.Crf
	if crf == 0 {
		crf = 23
	}
	preset := opts.Preset
	if preset == "" {
		preset = "medium"
	}

	args := []string{
		"-i", sourcePath,
		"-filter_complex", filterComplex,
		"-map", "[vout]",
		"-map", "[aout]",
		"-c:v", videoCodec,
		"-c:a", audioCodec,
		"-crf", strconv.Itoa(crf),
		"-preset", preset,
		"-y",
		outPath,
	}

	if opts.VideoBitrate != "" {
		args = append([]string{"-b:v", opts.VideoBitrate}, args...)
	}

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg concat reencode: %w", err)
	}
	return nil
}

func (a *ffmpegAdapter) MuxSubtitle(ctx context.Context, videoPath, subtitleClipPath, outPath string) error {
	binaryPath, err := a.resolver.Resolve("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg mux subtitle: %w", err)
	}

	// 将字幕片段叠加到视频上
	cmd := exec.CommandContext(ctx, binaryPath,
		"-i", videoPath,
		"-i", subtitleClipPath,
		"-filter_complex", "[1:v]scale=iw:ih[overlay];[0:v][overlay]overlay=0:0",
		"-c:a", "copy",
		"-y",
		outPath,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg mux subtitle: %w", err)
	}
	return nil
}

// ffmpegProgressRegex 匹配 ffmpeg stderr 的 time= 行
var ffmpegProgressRegex = regexp.MustCompile(`time=(\d+):(\d+):(\d+).(\d+)`)

// ParseFFmpegProgress 解析 ffmpeg stderr 的 time= 行，返回已处理毫秒数
func ParseFFmpegProgress(line string) int64 {
	matches := ffmpegProgressRegex.FindStringSubmatch(line)
	if len(matches) != 5 {
		return -1
	}
	hours, _ := strconv.Atoi(matches[1])
	minutes, _ := strconv.Atoi(matches[2])
	seconds, _ := strconv.Atoi(matches[3])
	// ffmpeg 的百分秒部分可能是 1-3 位
	cs, _ := strconv.Atoi(matches[4])
	for i := len(matches[4]); i < 3; i++ {
		cs *= 10
	}
	return int64(hours*3600000 + minutes*60000 + seconds*1000 + cs)
}

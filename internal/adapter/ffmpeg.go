package adapter

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"smart-cut/internal/model"
)

// FFmpegAdapter 定义视频处理接口
type FFmpegAdapter interface {
	Probe(ctx context.Context, path string) (*model.MediaFile, error)
	ExtractWaveform(ctx context.Context, mediaPath, outPng string) error
	ExtractAudio16kWav(ctx context.Context, mediaPath, outWav string) error
	ExtractWaveformPeaks(ctx context.Context, mediaPath string, durationMs int64, buckets int) (*model.WaveformPeaks, error)

	// ExtractSegment 逐段提取（内嵌音频淡入淡出 + HDR tone map + 竖屏感知缩放）
	// sourcePath: 源视频；segStartSec/segEndSec: 段起止秒；
	// media: 源媒体元信息（用于 HDR/竖屏判断）；outPath: 输出 mp4
	ExtractSegment(ctx context.Context, sourcePath string, segStartSec, segEndSec float64, media model.MediaFile, outPath string) error

	// ConcatDemuxer 用 concat demuxer + -c copy 真无损拼接
	// segmentPaths: 已提取的段 mp4 列表；outPath: 输出
	ConcatDemuxer(ctx context.Context, segmentPaths []string, outPath string) error

	// OverlaySegment 将字幕透明 mp4 叠加到本段视频上，输出带字幕的视频段
	// videoPath: ExtractSegment 产出的视频段；subtitlePath: SubtitleStep 产出的字幕透明 mp4
	// outPath: 叠加后的输出段（替换原 videoPath 进入 concat 拼接）
	// 假定 videoPath 和 subtitlePath 分辨率一致（均为 source 的 W×H）
	OverlaySegment(ctx context.Context, videoPath, subtitlePath, outPath string) error

	// ConcatReencode 重编码拼接（保留旧接口，逐段提取 + 重编码 concat，用于需要统一编码参数的场景）
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
		CodecType     string `json:"codec_type"`
		CodecName     string `json:"codec_name"`
		Width         int    `json:"width"`
		Height        int    `json:"height"`
		RFrameRate    string `json:"r_frame_rate"`
		Duration      string `json:"duration"`
		ColorTransfer string `json:"color_transfer"`
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
			media.ColorTransfer = stream.ColorTransfer
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

// ExtractAudio16kWav 提取音频并转码为 16kHz 单声道 PCM wav（whisper.cpp 要求的输入格式）
func (a *ffmpegAdapter) ExtractAudio16kWav(ctx context.Context, mediaPath, outWav string) error {
	binaryPath, err := a.resolver.Resolve("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg extract audio: %w", err)
	}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, binaryPath,
		"-i", mediaPath,
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-acodec", "pcm_s16le",
		"-y",
		outWav,
	)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg extract audio: %w (stderr: %s)", err, stderr.String())
	}
	return nil
}

// ExtractSegment 逐段提取：-ss before -i 快速精确 seek + 内嵌 vf/af
func (a *ffmpegAdapter) ExtractSegment(ctx context.Context, sourcePath string, segStartSec, segEndSec float64, media model.MediaFile, outPath string) error {
	binaryPath, err := a.resolver.Resolve("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg extract segment: %w", err)
	}

	duration := segEndSec - segStartSec
	if duration <= 0 {
		return fmt.Errorf("ffmpeg extract segment: invalid duration %.3f", duration)
	}

	portrait := IsPortrait(media.Width, media.Height)
	vf := BuildVFChain(media.ColorTransfer, portrait, 1920, 1080, nil)
	// 构造 args：-af 仅在有音频时加入（无音频段加 -af 会报错）；outPath 必须在最后
	args := []string{
		"-y",
		"-ss", fmt.Sprintf("%.3f", segStartSec),
		"-i", sourcePath,
		"-t", fmt.Sprintf("%.3f", duration),
		"-vf", vf,
		"-c:v", "libx264", "-preset", "fast", "-crf", "20",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "192k", "-ar", "48000",
		"-movflags", "+faststart",
	}
	if media.HasAudio {
		args = append(args, "-af", BuildAudioFadeChain(duration))
	}
	args = append(args, outPath)

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg extract segment: %w (stderr: %s)", err, stderr.String())
	}
	return nil
}

// ConcatDemuxer 用 concat demuxer + -c copy 真无损拼接（要求各段编码参数一致）
func (a *ffmpegAdapter) ConcatDemuxer(ctx context.Context, segmentPaths []string, outPath string) error {
	binaryPath, err := a.resolver.Resolve("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg concat demuxer: %w", err)
	}
	if len(segmentPaths) == 0 {
		return fmt.Errorf("ffmpeg concat demuxer: no segments")
	}

	// 写 concat 列表文件到 outPath 同目录
	listPath := outPath + ".concat.txt"
	var listContent strings.Builder
	for _, p := range segmentPaths {
		// 用绝对路径，避免工作目录问题
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		listContent.WriteString(fmt.Sprintf("file '%s'\n", abs))
	}
	if err := os.WriteFile(listPath, []byte(listContent.String()), 0644); err != nil {
		return fmt.Errorf("ffmpeg concat demuxer: write list: %w", err)
	}

	args := []string{
		"-y",
		"-f", "concat", "-safe", "0",
		"-i", listPath,
		"-c", "copy",
		"-movflags", "+faststart",
		outPath,
	}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg concat demuxer: %w (stderr: %s)", err, stderr.String())
	}
	// 成功后清理 list 文件（失败时保留以便排障）
	_ = os.Remove(listPath)
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

// OverlaySegment 将字幕透明 mp4 叠加到本段视频上，输出带字幕的视频段
func (a *ffmpegAdapter) OverlaySegment(ctx context.Context, videoPath, subtitlePath, outPath string) error {
	binaryPath, err := a.resolver.Resolve("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg overlay segment: %w", err)
	}

	args := []string{
		"-y",
		"-i", videoPath,
		"-i", subtitlePath,
		"-filter_complex", "[1:v]format=rgba[sub];[0:v][sub]overlay=0:0",
		"-c:v", "libx264", "-preset", "fast", "-crf", "20",
		"-pix_fmt", "yuv420p",
		"-c:a", "copy",
		"-movflags", "+faststart",
		outPath,
	}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg overlay segment: %w (stderr: %s)", err, stderr.String())
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

// hdrTransfers 触发 tone mapping 的传输函数集合（PQ/HDR10 与 HLG）
var hdrTransfers = map[string]bool{
	"smpte2084":    true, // PQ (HDR10)
	"arib-std-b67": true, // HLG
}

// TonemapChain HDR → SDR 的 zscale+tonemap filter 链
// 顺序：线性化 → 浮点 → bt709 色域 → hable tonemap → bt709 传输 → yuv420p
const TonemapChain = "zscale=t=linear:npl=100," +
	"format=gbrpf32le," +
	"zscale=p=bt709," +
	"tonemap=tonemap=hable:desat=0," +
	"zscale=t=bt709:m=bt709:r=tv," +
	"format=yuv420p"

// IsHDR 判断媒体是否为 HDR 源（PQ 或 HLG 传输函数）
func IsHDR(transfer string) bool {
	return hdrTransfers[transfer]
}

// BuildVFChain 构造视频 filter 链（纯函数，可测试）
// 顺序：HDR tone map（仅 HDR 源）→ 缩放（竖屏感知）
func BuildVFChain(colorTransfer string, portrait bool, targetWidth, targetHeight int, extraFilters []string) string {
	var parts []string
	if IsHDR(colorTransfer) {
		parts = append(parts, TonemapChain)
	}
	// 竖屏感知缩放：保持目标短边，长边 -2 自动对齐
	var scale string
	if portrait {
		scale = fmt.Sprintf("scale=-2:%d", targetHeight)
	} else {
		scale = fmt.Sprintf("scale=%d:-2", targetWidth)
	}
	parts = append(parts, scale)
	parts = append(parts, extraFilters...)
	return strings.Join(parts, ",")
}

// IsPortrait 判断是否竖屏（height > width）
func IsPortrait(width, height int) bool {
	return height > width
}

// audioFadeSec 切点音频淡入淡出时长（秒），防爆音
const audioFadeSec = 0.03

// BuildAudioFadeChain 构造 30ms 音频淡入淡出 filter 链（防爆音）
// durationSec 为本段时长（秒）
func BuildAudioFadeChain(durationSec float64) string {
	fadeOutStart := durationSec - audioFadeSec
	if fadeOutStart < 0 {
		fadeOutStart = 0
	}
	return fmt.Sprintf("afade=t=in:st=0:d=%.3f,afade=t=out:st=%.3f:d=%.3f", audioFadeSec, fadeOutStart, audioFadeSec)
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

// ExtractWaveformPeaks 提取波形峰值采样数据
func (a *ffmpegAdapter) ExtractWaveformPeaks(ctx context.Context, mediaPath string, durationMs int64, buckets int) (*model.WaveformPeaks, error) {
	binaryPath, err := a.resolver.Resolve("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("ffmpeg waveform peaks: %w", err)
	}

	if buckets <= 0 {
		buckets = 2000
	}

	const sampleRate = 8000
	totalSamples := int(int64(sampleRate) * durationMs / 1000)
	if totalSamples <= 0 {
		return nil, fmt.Errorf("invalid duration %dms for waveform", durationMs)
	}
	samplesPerBucket := totalSamples / buckets
	if samplesPerBucket < 1 {
		samplesPerBucket = 1
	}

	cmd := exec.CommandContext(ctx, binaryPath,
		"-i", mediaPath,
		"-f", "s16le",
		"-acodec", "pcm_s16le",
		"-ac", "1",
		"-ar", fmt.Sprintf("%d", sampleRate),
		"-",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg waveform pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("ffmpeg waveform start: %w", err)
	}

	mins, maxs := computePeaksFromReader(stdout, samplesPerBucket, buckets)

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("ffmpeg waveform wait: %w (%s)", err, stderr.String())
	}

	return &model.WaveformPeaks{
		DurationMs: durationMs,
		SampleRate: sampleRate,
		Buckets:    len(mins),
		Mins:       mins,
		Maxs:       maxs,
	}, nil
}

// computePeaksFromReader 从 PCM int16 流计算每桶 min/max 峰值
func computePeaksFromReader(r io.Reader, samplesPerBucket, buckets int) (mins, maxs []int16) {
	mins = make([]int16, 0, buckets)
	maxs = make([]int16, 0, buckets)
	buf := make([]byte, samplesPerBucket*2)

	for i := 0; i < buckets; i++ {
		n, err := io.ReadFull(r, buf)
		if n == 0 {
			break
		}
		readSamples := n / 2
		var minV, maxV int16
		for j := 0; j < readSamples; j++ {
			v := int16(binary.LittleEndian.Uint16(buf[j*2 : j*2+2]))
			if j == 0 || v < minV {
				minV = v
			}
			if j == 0 || v > maxV {
				maxV = v
			}
		}
		mins = append(mins, minV)
		maxs = append(maxs, maxV)
		if err != nil {
			break
		}
	}
	return mins, maxs
}

// computePeaks 纯函数版本（用于测试，不依赖 IO）
func computePeaks(samples []int16, samplesPerBucket int) (mins, maxs []int16) {
	if samplesPerBucket < 1 {
		samplesPerBucket = 1
	}
	buckets := (len(samples) + samplesPerBucket - 1) / samplesPerBucket
	mins = make([]int16, 0, buckets)
	maxs = make([]int16, 0, buckets)
	for i := 0; i < len(samples); i += samplesPerBucket {
		end := i + samplesPerBucket
		if end > len(samples) {
			end = len(samples)
		}
		var minV, maxV int16
		for j, v := range samples[i:end] {
			if j == 0 || v < minV {
				minV = v
			}
			if j == 0 || v > maxV {
				maxV = v
			}
		}
		mins = append(mins, minV)
		maxs = append(maxs, maxV)
	}
	return mins, maxs
}

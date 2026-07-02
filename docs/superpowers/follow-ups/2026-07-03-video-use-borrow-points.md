 # video-use 借鉴点 —— 后续独立任务清单

 - **日期**: 2026-07-03
 - **来源**: [browser-use/video-use](https://github.com/browser-use/video-use) 分析（同赛道开源项目：对话式 AI 口播视频剪辑）
 - **状态**: 待排期，独立于 Remotion 字幕系统（2026-07-03-spec）执行

 ## 背景

 video-use 是 browser-use 团队开源的 Claude Code skill（Python helpers + SKILL.md 规则书），定位与 Smart-Cut 高度重叠（口播视频自动剪辑），但走纯对话/LLM 主导路线，无 GUI。其生产级 ffmpeg 工程经验经过真实视频验证，多条规则直接适用 Smart-Cut，且暴露了 Smart-Cut 当前导出管线的正确性缺陷。

 本文记录可借鉴点，作为后续独立任务排期。**不并入 Remotion 字幕系统 spec**，避免范围蔓延。

 ---

 ## A. 强烈建议（影响导出正确性，应优先排期）

 ### A1. 切点 30ms 音频淡入淡出（防爆音）

 - **问题**：当前 [ConcatLossless](file:///d:/workspace/go/src/smart-cut/internal/adapter/ffmpeg.go) 用 `trim` 直接拼接，切点处波形不连续，会产生可听见爆音（pop）。
 - **借鉴**：video-use Hard Rule 3 —— 每个切点边界加 `afade=t=in:st=0:d=0.03,afade=t=out:st={dur-0.03}:d=0.03`。
 - **实现位置**：`FFmpegAdapter` 的逐段提取/拼接逻辑。
 - **代价**：极低（每段加两个 afade filter）。
 - **注意**：与 A3（逐段提取重构）天然契合，建议合并实现。

 ### A2. HDR → SDR tone mapping（防过曝）

 - **问题**：iPhone 默认 HLG HDR（Rec.2020），mirrorless 常见 PQ。直接 `yuv420p10le→yuv420p` 只降位深不调色调，8-bit 输出仍带 HLG/PQ 传输元数据，上传社交平台或屏幕录制会过曝/过饱和。QuickTime 本地播放可能隐藏此问题。
 - **借鉴**：video-use 检测 `color_transfer ∈ {smpte2084, arib-std-b67}` 后，前置 `zscale=t=linear → tonemap=hable → zscale=t=bt709:m=bt709:r=tv → format=yuv420p` 链。
 - **实现位置**：
   - `FFmpegAdapter.Probe` / `parseFFprobeJSON` 增加 `color_transfer` 字段采集（`model.MediaFile` 加字段）。
   - `ConcatReencode` 的 `-vf` 链按需前置 tonemap。
 - **代价**：中（需扩展 Probe 输出 + vf 链条件拼装）。

 ### A3. 竖屏方向保持（防压扁）

 - **问题**：当前导出 `scale` 默认按宽度，竖屏（h>w）会被压扁。
 - **借鉴**：video-use `is_portrait_source()` —— ffprobe 检测 h>w，竖屏改用 `scale=-2:H`，横屏用 `scale=W:-2`。
 - **实现位置**：`FFmpegAdapter` 的 scale filter 构造逻辑。
 - **代价**：低。

 ---

 ## B. 架构反思（影响"无损"承诺，需评估）

 ### B1. ConcatLossless 改为"逐段提取 + concat demuxer"

 - **现状**：[ConcatLossless](file:///d:/workspace/go/src/smart-cut/internal/adapter/ffmpeg.go) 用单个大 `filter_complex` 把所有 `trim`+`concat` 串起来，再 `-c:v copy`。
 - **问题**：`filter_complex` 的 trim/concat 在解码域运行，`-c copy` 在此场景不生效（会报错或被忽略），实际是重编码路径却承诺无损。video-use Hard Rule 2 指出：单次 filtergraph 会让每段被双重编码。
 - **借鉴**：video-use render.py —— `extract_segment`（`-ss` before `-i` 快速精确 seek + `-t` + 内嵌 grade/afade）逐段产出独立 mp4 → `concat_segments`（`-f concat -c copy` concat demuxer）真无损拼接。
 - **收益**：
   - 真正无损（concat demuxer + `-c copy` 不重编码）。
   - 天然支持逐段嵌入音频淡入淡出（A1）、色彩校正、HDR tone mapping（A2）。
   - 与 Remotion 字幕系统"逐段渲染后拼接"决策同构 —— 视频段和字幕段共用逐段提取-拼接管线。
 - **代价**：中（重构 ExportStep + FFmpegAdapter，需保证 concat demuxer 要求各段编码参数一致）。
 - **决策**：建议在 Remotion 字幕系统之后单独排期，或与之合并（字幕段拼接本就需要逐段管线）。

 ---

 ## C. 可选优化（非必需）

 ### C1. LLM 输入短语级压缩

 - **借鉴**：video-use `pack_transcripts.py` 把词级转录压成短语级 markdown（按 ≥0.5s 沉默或换人断句），1 小时素材 ~12KB，token 降到原始 JSON 的 1/10。
 - **现状**：Smart-Cut [AnalyzeStep](file:///d:/workspace/go/src/smart-cut/internal/pipeline/steps.go) 按句级 segment 送 LLM，压缩收益较小。
 - **决策**：仅当 token 成瓶颈或 LLM 决策质量不足时考虑。当前句级已够用。

 ### C2. 输出时间轴偏移公式（已验证设计正确）

 - **借鉴**：video-use `output_time = word.start - segment_start + segment_offset`。
 - **现状**：Smart-Cut Remotion spec §3.1 已采用"段内时间偏移在 Service 层做"，思路一致。
 - **决策**：无需额外工作，spec 设计已被外部项目验证。记录备查。

 ---

 ## D. 不采纳（与 Smart-Cut 定位冲突）

 ### D1. self-eval 自评循环

 - **原因**：video-use 渲染后在每个切点 ±1.5s 跑 filmstrip+waveform PNG 自检（最多 3 轮）。Smart-Cut 是 GUI 时间轴用户肉眼微调路线，自评与定位冲突。
 - **备注**：导出后自动质检（如检测切点爆音提示用户）作为未来扩展可考虑，但 MVP 不做。

 ### D2. 纯文本驱动剪辑

 - **原因**：video-use LLM 读转录文本下全盘决策。Smart-Cut 是 LLM 仅标语气词/重复、用户时间轴主导，两者根本路线不同。

 ---

 ## 排期建议

 | 优先级 | 任务 | 理由 |
 |---|---|---|
 | P1 | A1 + A3 + B1 合并实现 | 共同重构 FFmpegAdapter 拼接管线，一次改动解决爆音+竖屏+真无损 |
 | P1 | A2 HDR tone mapping | iPhone 用户导出过啸是硬伤，独立实现成本低 |
 | P3 | C1 短语级压缩 | 仅当 LLM 链路 token 成瓶颈时 |
 | 不做 | D1 / D2 | 与定位冲突 |

 **与 Remotion 字幕系统的关系**：B1（逐段提取重构）与字幕系统的"逐段渲染后拼接"高度同构，若排期相邻建议合并为一个大任务，避免两次重构 ExportStep。

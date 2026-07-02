 # Smart-Cut Remotion 字幕系统设计

 - **日期**: 2026-07-03
 - **项目**: Smart-Cut —— AI 口播视频自动剪辑工具
 - **状态**: 已确认，待写实施计划
 - **前置**: Plan 1-5 已完成（脚手架/Adapters/Pipeline-Service/API-Frontend/Timeline），主线 transcribe→analyze→edit→export 已跑通；`SubtitleStep` 当前为 `skipped (MVP)` 占位，`frontend/src/remotion/` 仅有 `.gitkeep`。

 ## 1. 目标

 实现 Smart-Cut 的最后一块 MVP 能力：**Remotion 动态字幕（逐字高亮）**，覆盖预览与导出两个场景，并复用现有 pipeline/eventbus 进度机制。

 ### 1.1 MVP 范围

 - **Player 实时预览**：在 `VideoPreview` 上叠加 Remotion `<Player>`，随视频播放逐字高亮字幕。
 - **字幕渲染 + 烘入导出**：`SubtitleStep` 调用 Node 渲染 worker 产出透明字幕 mp4 片段，`ExportStep` 在拼接时 overlay 到视频。
 - **渲染进度回传**：新增 `stage=subtitle` 进度事件，与现有 transcribe/analyze/export 机制一致。
 - **样式可配置**：字幕字体/字号/颜色/高亮色/位置/背景色/透明度可配置，改动后 Player 实时刷新。

 ### 1.2 非目标（留作未来）

 - pkg 预编译 render-worker 为单二进制（MVP 用 Node 脚本，打包阶段再考虑）
 - "清理工程缓存"按钮
 - 字幕模板/片头片尾（非字幕功能）

 ## 2. 关键技术决策

 | 决策点 | 选定方案 | 理由 |
 |---|---|---|
 | 技术路线 | Player + Node 脚本 worker 导出 | 既支持预览逐字高亮，又支持导出产物含字幕。MVP 不打包 worker，开发机装 Node 即可 |
 | 渲染时机 | 逐段渲染后拼接 | 渲染时长 = 各段时长之和（最短），且视频和字幕片段一一对应拼接逻辑清晰 |
 | 逐字高亮数据源 | whisper 词级时间戳 | `Word.StartMs/EndMs` 最准确；词级时间戳缺失或异常时该段跳过字幕（回退到无字幕导出） |
 | Worker 交付形态 | Node 脚本（`resources/remotion/render-worker.js`） | 开发优先，免去 pkg 构建流水线；打包阶段再考虑 pkg/Bun |
 | 透明字幕格式 | webm/vp8（带 alpha） | alpha 支持好、体积小；实现时实测 webview 与 ffmpeg overlay 兼容性，不兼容则切 ProRes 4444 |

 ## 3. 架构与分层

 沿用现有 5 层架构，新增 Remotion 组件按层归位：

 ```
 L1. UI Layer
     ├── RemotionPlayer.tsx         # <Player> 包装，叠加在 <video> 上，逐字高亮
     └── remotion/SubtitleComp.tsx  # 字幕 Composition（纯 React 组件，前后端同构）
 L2. API Layer (App)
     └── GetSubtitleConfig          # 新增：返回当前 SubtitleStyle + Words 给前端 Player
 L3. Service Layer
     ├── SubtitleService            # 新增：编排 SubtitleStep + 进度
     └── ExportService              # 修改：opts.IncludeSubtitle=true 时串联 SubtitleStep
 L4. Pipeline Layer
     ├── SubtitleStep               # 重写：调用 RemotionAdapter 渲染每段字幕 mp4
     └── Context                    # 新增字段：SubtitleClips map[string]string
 L5. Adapter Layer
     ├── RemotionAdapter            # 新增：exec node + render-worker.js，解析 stdout 进度
     └── FFmpegAdapter              # 复用现有 MuxSubtitle / overlay 能力

 resources/remotion/
     ├── render-worker.js           # Node 脚本，stdin 接 JSON，调 @remotion/renderer
     └── package.json               # remotion / react / react-dom 依赖
 ```

 ### 3.1 设计原则

 - **SubtitleComp 前后端同构**：浏览器喂给 `<Player>` 预览，Node 里 render-worker import 同一文件渲染。这是选 Remotion 的核心理由，Composition 不感知运行环境。
 - **RemotionAdapter 与 Whisper/FFmpeg 同级**：通过 `BinaryResolver.Resolve("node")` 解析 Node 二进制路径，worker 脚本路径单独配置。
 - **SubtitleStep 作为独立 Step**：由 ExportService 在 `IncludeSubtitle=true` 时串联（而非硬编码进 ExportStep），保持 ExportStep 单一职责。
 - **段内时间偏移在 Service 层做**：传给 SubtitleComp 的 `words[].startMs` 一律是相对于段起点的偏移（段内 frame 0 = 段 startMs）。浏览器预览（整片 frame 0 = 视频 0ms）和逐段渲染（段内 frame 0 = 段 startMs）用同一套渲染逻辑，只是数据预处理不同。

 ## 4. 数据流与接口契约

 ### 4.1 SubtitleComp 输入契约（前后端同构核心）

 Remotion Composition 的 props 必须纯 JSON 可序列化（浏览器和 Node 都能消费）：

 ```typescript
 interface SubtitleCompProps {
   words: { text: string; startMs: number; endMs: number }[];
   style: SubtitleStyle;  // 复用 model.SubtitleStyle
 }
 ```

 **时间基准约定**：`words[].startMs` 一律是**相对于段起点**的偏移。调用方（Service 层）负责偏移计算，Composition 本身不感知绝对时间。

 **句分组逻辑**：复用 `transcript.segments` 的 WordIDs 分组，显示包含 `activeIndex`（当前时间命中的词）的那个 segment 的文本。active 词用 highlight 色，其余用 color 色。

 ### 4.2 RemotionAdapter 接口

 ```go
 type RemotionAdapter interface {
     RenderSegment(ctx context.Context, req SubtitleSegmentRequest) (clipPath string, err error)
 }

 type SubtitleSegmentRequest struct {
     SegmentID    string             // keep 段标识，用于命名输出文件
     StartMs      int64              // 段在原视频的起点（用于 worker 日志，不传给 Composition）
     EndMs        int64
     Words        []model.Word       // 已偏移为段内相对时间
     Style        model.SubtitleStyle
    Width        int                // 视频帧宽（来自 MediaFile.Width，喂给 renderMedia）
    Height       int                // 视频帧高
    Fps          float64            // 视频帧率（来自 MediaFile.Fps）
     OutputDir    string             // 段字幕 mp4 输出目录
 }
 ```

Worker 调用：`exec.CommandContext(ctx, nodePath, workerScriptPath)`，通过 stdin 传入 JSON（避免命令行长度限制，词数多时安全）。worker 解析后调 `@remotion/renderer.renderMedia`，输出 `<OutputDir>/subtitle_<SegmentID>.mp4`。`Width`/`Height`/`Fps` 取自 `project.Media`，保证字幕透明层与视频帧尺寸、帧率严格一致，overlay 时不错位。

 ### 4.3 逐段渲染产物 → 拼接

 ```
 <WorkDir>/<projectID>/subtitle_clips/
 ├── subtitle_<seg1ID>.mp4   # 段1字幕透明层
 ├── subtitle_<seg2ID>.mp4   # 段2字幕透明层
 └── ...
 ```

 `ExportStep` 拼接时，每个 keep 段的 filter 从单纯 `trim` 升级为 `trim + overlay[对应字幕片段]`，拼接逻辑不变。字幕片段与视频段一一对应，时长严格匹配（Composition `durationInFrames` 按 `endMs-startMs` 精确计算）。

 ### 4.4 进度事件契约

 复用现有 `model.ProgressEvent`：

 - `Stage = "subtitle"`，`Step` 形如 `"rendering 3/12"`（当前段/总段数）
 - `Progress` = 已完成段进度 / 总段数（0-1）
 - 失败时 `Status = "error"`，前端 Workbench 的 `onProgress` 需补一个 `ev.stage === "subtitle"` 分支

 ### 4.5 ExportService 编排变化

 `IncludeSubtitle=true` 时，ExportService 的 pipeline 从单步变两步：

 ```
 SubtitleStep（渲染所有段字幕） → ExportStep（拼接视频 + overlay 字幕）
 ```

 `ExportStep` 通过 `pipeline.Context` 新增字段 `SubtitleClips map[string]string`（segID → clipPath）读取字幕片段路径表。`IncludeSubtitle=false` 时跳过 SubtitleStep，`Context.SubtitleClips` 为 nil，ExportStep 走原逻辑。

 ## 5. 前端 RemotionPlayer 与样式配置

 ### 5.1 RemotionPlayer.tsx —— 叠加层

 `VideoPreview` 内部，`<video>` 之上叠加一个绝对定位的 `RemotionPlayer`：

 ```tsx
 <div className="relative">
   <video ref={videoRef} src={src} onTimeUpdate={...} />
   {subtitleEnabled && transcript?.words && (
     <div className="absolute inset-0 pointer-events-none">
       <RemotionPlayer
         component={SubtitleComp}
         inputProps={{ words: relativeWords, style: subtitleStyle }}
         durationInFrames={framesFromMs(durationMs)}
         fps={30}
         compositionWidth={mediaWidth}
         compositionHeight={mediaHeight}
         style={{ width: "100%" }}
         initiallyPlaying={false}
         loop={false}
       />
     </div>
   )}
 </div>
 ```

 **同步机制（单时钟）**：

 - Remotion `<Player>` 设置 `initiallyPlaying={false}`，不自动推进时间。
 - 时间唯一真源是 `<video>` 的 `currentTime`（经 `onTimeUpdate` → `store.playheadMs`）。
 - 用 `useEffect` 监听 `playheadMs`，调 `playerRef.current().seekToFrame(frameFromMs(playheadMs))`，与 video.currentTime 双向同步（复用现有 VideoPreview 的 350ms 阈值跳帧逻辑）。
 - 逐字高亮纯靠 seek 响应 playheadMs，不让 Player 自己跑时间线，避免双时钟漂移。
 - 刷新频率取决于 video 的 `timeupdate` 事件（约 4-66Hz），对逐字高亮够用。

 ### 5.2 SubtitleComp.tsx —— 同构组件

 ```tsx
 import { AbsoluteFill, useCurrentFrame, useVideoConfig } from "remotion";

 export const SubtitleComp: React.FC<{ words: SubtitleWord[]; style: SubtitleStyle }> = ({
   words, style,
 }) => {
   const frame = useCurrentFrame();
   const { fps } = useVideoConfig();
   const timeMs = (frame / fps) * 1000;

   // 找当前时间应高亮的词
   const activeIndex = words.findIndex(
     (w) => timeMs >= w.startMs && timeMs < w.endMs
   );
   if (activeIndex < 0) return <AbsoluteFill />;  // 无命中，空帧

   // 渲染包含 activeIndex 的那个 segment 的所有词
   // active 词用 highlight 色，其余用 color 色
   return <AbsoluteFill>...</AbsoluteFill>;
 };
 ```

 句分组复用 `transcript.segments` 的 WordIDs，显示包含 `activeIndex` 的那个 segment 的文本。

 ### 5.3 样式配置入口（Workbench 可折叠侧边面板）

 在 Workbench 新增一个可折叠侧边面板（项目级），编辑 `ProjectSettings.SubtitleStyle`：

 - 字体、字号、颜色、高亮色、位置（bottom/center/top）、背景色、背景透明度
 - 配置存入 `ProjectSettings.SubtitleStyle`（已定义），通过 `SaveProject` 持久化
 - 变更后 Player 的 `inputProps.style` 变化 → React 自动重渲染，预览即时刷新
 - 导出时 SubtitleService 从 `project.Settings.SubtitleStyle` 读取传给 worker

 **位置选择理由**：字幕样式与具体项目强相关（不同视频字体/字号不同），放 Workbench 项目级侧边面板语义最清晰，而非全局 Settings 页。

 ## 6. 错误处理、回退与临时文件

 ### 6.1 错误分类与处理策略

 | 错误类型 | 来源 | 处理 |
 |---|---|---|
 | Node 缺失 | `BinaryResolver.Resolve("node")` 失败 | 启动时 ProbeBinary 检测 + 引导；运行时返回 `ErrCodeEnv` |
 | Worker 脚本缺失 | `resources/remotion/render-worker.js` 不存在 | `ErrCodeEnv`，提示重装/检查目录 |
 | Worker 渲染失败 | worker 进程退出码非 0 / stdout 解析失败 | 包装 stderr，`ErrCodeInternal`，**自动回退无字幕导出**（见 6.2） |
 | 渲染超时 | 单段渲染 > 5 分钟（可配） | `ctx.Cancel` + 告警，清理半成品 |
 | ffmpeg overlay 失败 | overlay 失败 | 中止 ExportStep，清理临时文件 |
 | 词级时间戳缺失 | `transcript.Words` 为空 | SubtitleService 直接跳过字幕，正常无字幕导出 |

 ### 6.2 关键回退策略：字幕失败不阻断导出

 字幕是"锦上添花"，不应因字幕问题导致整个导出失败。**分层降级**：

 1. `transcript.Words` 为空 → SubtitleService 不启动 SubtitleStep，ExportStep 走原逻辑（无字幕）
 2. Worker 不可用（Node 缺失/脚本缺失）→ 同上，推一条 `log` 事件提示"字幕不可用，已跳过"
 3. 某段渲染失败 → 该段字幕跳过（不 overlay），其余段正常拼接并 overlay。字幕缺失段视频仍保留
 4. overlay 失败 → ExportStep 整体失败（视频拼接已破坏无法回退），清理 cuts/ 重建

 **实现位置**：`SubtitleStep.Run` 内对每段渲染做 err 判断，失败的段记入日志但不 return err，最终 `Context.SubtitleClips` 只含成功的段。`ExportStep` 对每段检查 `SubtitleClips[segID]` 是否存在，存在才 overlay。

 ### 6.3 临时文件生命周期

 ```
 <WorkDir>/<projectID>/
 ├── subtitle_clips/          # 新增
 │   ├── subtitle_<segID>.mp4
 │   └── subtitle_<segID>.mp4
 ├── cuts/                    # 现有
 │   └── keep_001.mp4
 └── export.mp4
 ```

 **清理规则**（与主设计文档 8.3 一致）：

 - `subtitle_clips/` 与 `cuts/` 同生命周期：任务失败/取消时清理，成功导出后保留（便于调整重导）
 - 在 SubtitleStep 和 ExportStep 各加 `defer cleanupDir()` 模式，清理各自目录
 - "清理工程缓存"按钮留作未来扩展，MVP 不做

 ### 6.4 Worker 进程契约

 `render-worker.js` 的 stdin/stdout 协议：

 ```
 stdin  → 单行 JSON: { segmentID, startMs, endMs, words, style, outputPath, width, height, fps }
 stdout → 进度行: PROGRESS <0-1>
          完成行: DONE <outputPath>
          错误行: ERROR <message>
 stderr → 原始 Remotion 日志（调试用）
 退出码 → 0 成功 / 1 失败
 ```

 `RemotionAdapter` 按行扫描 stdout：

 - 遇 `PROGRESS` → 更新 reporter（总进度 = (已完成段 + 当前段进度) / 总段数）
 - 遇 `DONE` → 收集路径
 - 遇 `ERROR` → 记录并跳过该段（回退 6.2 第 3 条）

 **超时与取消**：`exec.CommandContext` 天然支持 `ctx.Cancel`；另用 `context.WithTimeout(ctx, 5*time.Minute)` 包一层防卡死。worker 收到 SIGTERM（Node 进程）优雅退出 —— Remotion renderer 支持 AbortSignal，worker 内部转发。

 ## 7. 测试策略

 | 层 | 策略 | 工具 |
 |---|---|---|
 | RemotionAdapter | 重点。预录 worker stdout fixture（PROGRESS/DONE/ERROR 三类），mock exec | testify + fixture |
 | SubtitleStep | Mock RemotionAdapter，验证逐段调用、失败回退、Context.SubtitleClips 填充 | testify |
 | ExportStep（overlay 分支） | Mock FFmpegAdapter，验证有字幕片段时 overlay filter 拼接、无字幕片段时走原逻辑 | testify |
 | SubtitleComp | vitest + RTL，验证 active 词高亮、无命中时空帧 | vitest |
 | render-worker.js | 端到端冒烟（需 Node + Remotion 安装，CI 可选） | 手动 |

 关键单测点：
 - `RemotionAdapter.parseWorkerStdout`（PROGRESS/DONE/ERROR 解析）
 - `SubtitleStep` 逐段失败不中断、SubtitleClips 只含成功段
 - `ExportStep` overlay 分支 filter 构造
 - `SubtitleComp` active 词判定与高亮渲染

 ## 8. 与现有代码的衔接点

 - **`internal/pipeline/steps.go`**：现有 `SubtitleStep` 占位需重写；`NewTranscribeStep` 有两行 `return` 的 unreachable code bug 顺手修掉（与本任务无关但影响 `go vet`）。
 - **`internal/pipeline/pipeline.go`**：`Context` 结构体新增 `SubtitleClips map[string]string` 字段。
 - **`internal/service/export.go`**：`StartExport` 根据 `opts.IncludeSubtitle` 决定是否串联 SubtitleStep。
 - **`internal/adapter/ffmpeg.go`**：`ConcatLossless`/`ConcatReencode` 需支持可选的 overlay 字幕片段（参数扩展或新增方法，实现时定）。
 - **`app/app_async.go`**：新增 `GetSubtitleConfig` 绑定；`StartExport` 已接收 `opts`，无需改签名。
 - **`frontend/src/components/VideoPreview.tsx`**：叠加 `RemotionPlayer`。
 - **`frontend/src/pages/Workbench.tsx`**：新增可折叠字幕样式侧边面板；`onProgress` 补 `subtitle` 分支。
 - **`frontend/src/stores/workbench.ts`**：新增 `subtitleStyle`、`subtitleEnabled` 状态。
 - **`frontend/package.json`**：新增 `remotion`、`@remotion/player` 依赖。
 - **`resources/remotion/`**：新建 `render-worker.js` + `package.json`。

 ## 9. 已知风险

 | 风险 | 严重度 | 对策 |
 |---|---|---|
 | Remotion Player 与 webview 性能（大视频帧刷新） | 中 | seek 驱动而非自动播放，刷新频率受限于 timeupdate；必要时节流 |
 | webm/vp8 alpha 在 ffmpeg overlay 的兼容性 | 中 | 实测，不兼容切 ProRes 4444 |
 | Node 路径跨平台差异（nvm/系统 Node） | 中 | BinaryResolver 三级查找：用户配置 → 随包 → PATH |
 | 逐段渲染串行总时长（10 分钟视频 = 10 分钟渲染） | 中 | MVP 接受串行；未来 worker 内部并行或预编译加速 |
 | Remotion 在 Node 端的 Composition 注册需匹配 Player 端 | 中 | SubtitleComp 同构单一文件，两端 import 同一路径 |
 | 单段 Composition durationInFrames 与视频段时长不匹配导致 overlay 错位 | 高 | 按段 ms 精确换算帧数，容差 < 1 帧；ExportStep overlay 用最短时长兜底 |

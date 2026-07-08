export type ProjectStatus = "draft" | "transcribed" | "analyzed" | "exported";
export type ExportMode = "lossless" | "reencode";
export type CutDecision = "keep" | "remove";
export type CutReason = "filler" | "silence" | "dup_or_error" | "manual";
export type CutSource = "ai" | "manual";
export type TaskStatus = "running" | "done" | "error";

export interface MediaFile {
  path: string;
  durationMs: number;
  format: string;
  width: number;
  height: number;
  fps: number;
  hasAudio: boolean;
}

export interface SubtitleStyle {
  fontFamily: string;
  fontSize: number;
  color: string;
  highlight: string;
  position: string;
  bgColor: string;
  bgOpacity: number;
}

export interface LLMConfig {
  baseUrl: string;
  apiKey: string;
  model: string;
}

export interface ProjectSettings {
  exportMode: ExportMode;
  silenceMs: number;
  fillerDict: string[];
  llmConfig: LLMConfig;
  subtitleStyle: SubtitleStyle;
  overlayItems: OverlayItem[];
  overlayStyle: OverlayStyle;
}

export interface Project {
  id: string;
  name: string;
  createdAt: string;
  updatedAt: string;
  workDir: string;
  media: MediaFile;
  status: ProjectStatus;
  settings: ProjectSettings;
}

export interface Word {
  text: string;
  startMs: number;
  endMs: number;
  confidence: number;
}

export interface Segment {
  id: number;
  text: string;
  startMs: number;
  endMs: number;
  words: Word[];
}

export interface Transcript {
  language: string;
  segments: Segment[];
  text: string;
}

export interface CutSegment {
  id: string;
  startMs: number;
  endMs: number;
  decision: CutDecision;
  reason: CutReason;
  source: CutSource;
  confidence: number;
  note: string;
}

export interface CutList {
  projectId: string;
  segments: CutSegment[];
  version: number;
}

export interface KeepSegment {
  startMs: number;
  endMs: number;
}

export interface ExportOptions {
  mode: ExportMode;
  includeSubtitle: boolean;
  outputPath: string;
}

export interface EncodeOpts {
  videoCodec: string;
  audioCodec: string;
  videoBitrate: string;
  crf: number;
  preset: string;
}

export interface GlobalSettings {
  binaries: Record<string, string>;
  whisperModelDir: string;
  defaultLLM: LLMConfig;
  theme: string;
}

export interface ProgressEvent {
  taskId: string;
  stage: string;
  step: string;
  progress: number;
  status: TaskStatus;
  error?: string;
  payload?: unknown;
}

export interface AppError {
  code: string;
  message: string;
  detail: string;
}

export interface WaveformPeaks {
  durationMs: number;
  sampleRate: number;
  buckets: number;
  mins: number[];
  maxs: number[];
}

export interface SubtitleConfig {
  segments: Segment[];
  style: SubtitleStyle;
}

export interface FileFilter {
  displayName: string;
  pattern: string;
}

export type OverlayDisplayMode = "overlay" | "fullscreen";
export type OverlayType = "card";

export interface AnimationConfig {
  in: string;
  out: string;
  duration: number;
}

export interface PositionConfig {
  x: string;
  y: string;
}

export interface CardContent {
  title: string;
  body?: string;
  bulletPoints?: string[];
  icon?: string;
  bigNumber?: string;
  accentColor?: string;
  bgColor?: string;
}

export interface OverlayItem {
  id: string;
  type: OverlayType;
  mode: OverlayDisplayMode;
  startMs: number;
  endMs: number;
  animation: AnimationConfig;
  position: PositionConfig;
  content: CardContent;
}

export interface OverlayStyle {
  accentColor: string;
  cardBgColor: string;
  cardRadius: number;
  fontFamily: string;
  showAnimations: boolean;
}

export interface OverlayConfig {
  items: OverlayItem[];
  style: OverlayStyle;
}

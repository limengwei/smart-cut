declare module "../../bindings/smart-cut/app/app.js" {
  import type {
    Project,
    CutList,
    CutSegment,
    Transcript,
    ExportOptions,
    GlobalSettings,
    MediaFile,
    WaveformPeaks,
    SubtitleConfig,
  } from "./types";

  export function CreateProject(name: string, mediaPath: string): Promise<Project>;
  export function OpenProject(projectPath: string): Promise<Project>;
  export function SaveProject(p: Project): Promise<void>;
  export function GetProject(projectID: string): Promise<Project>;

  export function GetCutList(projectID: string): Promise<CutList>;
  export function AddCutSegment(projectID: string, seg: CutSegment): Promise<void>;
  export function UpdateCutSegment(projectID: string, seg: CutSegment): Promise<void>;
  export function RemoveCutSegment(projectID: string, segID: string): Promise<void>;

  export function StartTranscribe(projectID: string): Promise<string>;
  export function StartAnalyze(projectID: string): Promise<string>;
  export function StartExport(projectID: string, opts: ExportOptions): Promise<string>;

  export function GetTranscript(projectID: string): Promise<Transcript>;
  export function GetWaveform(projectID: string): Promise<string>;
  export function ProbeMedia(path: string): Promise<MediaFile>;

  export function GetSettings(): Promise<GlobalSettings>;
  export function SaveSettings(s: GlobalSettings): Promise<void>;
  export function ProbeBinary(name: string): Promise<{ path: string; version: string }>;

  export function GetWaveformPeaks(projectID: string): Promise<WaveformPeaks>;
  export function GetMediaURL(projectID: string): Promise<string>;

  export function GetSubtitleConfig(projectID: string): Promise<SubtitleConfig>;
}

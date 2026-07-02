import { Events } from "@wailsio/runtime";
import type { ProgressEvent, Transcript, CutList, Segment } from "./types";

interface WailsEventPayload {
  name: string;
  data: unknown;
  sender?: string;
}

export function onProgress(cb: (event: ProgressEvent) => void): () => void {
  return Events.On("progress", ((ev: WailsEventPayload) => cb(ev.data as ProgressEvent)) as any);
}

export function onTranscriptReady(cb: (transcript: Transcript) => void): () => void {
  return Events.On("transcript:ready", ((ev: WailsEventPayload) => cb(ev.data as Transcript)) as any);
}

export function onTranscriptSegment(cb: (segment: Segment) => void): () => void {
  return Events.On("transcript:segment", ((ev: WailsEventPayload) => cb(ev.data as Segment)) as any);
}

export function onCutListReady(cb: (cutList: CutList) => void): () => void {
  return Events.On("cutlist:ready", ((ev: WailsEventPayload) => cb(ev.data as CutList)) as any);
}

export function onExportDone(cb: (exportPath: string) => void): () => void {
  return Events.On("export:done", ((ev: WailsEventPayload) => cb(ev.data as string)) as any);
}

export function onLog(cb: (logLine: string) => void): () => void {
  return Events.On("log", ((ev: WailsEventPayload) => cb(ev.data as string)) as any);
}

export function offAll(): void {
  Events.Off("progress");
  Events.Off("transcript:ready");
  Events.Off("cutlist:ready");
  Events.Off("export:done");
  Events.Off("log");
}

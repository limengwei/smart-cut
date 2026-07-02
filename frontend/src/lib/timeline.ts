export interface Viewport {
  durationMs: number;
  visibleWidth: number;
  scrollMs: number;
  pxPerMs: number;
}

export const TRACK_HEIGHT = 80;
export const PLAYHEAD_HALF_WIDTH = 1;

export function fitPxPerMs(durationMs: number, visibleWidth: number): number {
  if (durationMs <= 0 || visibleWidth <= 0) return 0;
  return visibleWidth / durationMs;
}

export function buildViewport(
  durationMs: number,
  visibleWidth: number,
  zoom: number,
  scrollMs: number
): Viewport {
  const fit = fitPxPerMs(durationMs, visibleWidth);
  return {
    durationMs,
    visibleWidth,
    scrollMs,
    pxPerMs: fit * zoom,
  };
}

export function timeToX(ms: number, vp: Viewport): number {
  return (ms - vp.scrollMs) * vp.pxPerMs;
}

export function xToTime(x: number, vp: Viewport): number {
  if (vp.pxPerMs <= 0) return 0;
  return vp.scrollMs + x / vp.pxPerMs;
}

export function clampMs(ms: number, durationMs: number): number {
  return Math.max(0, Math.min(ms, durationMs));
}

export function formatTimecode(ms: number): string {
  const safe = Math.max(0, ms);
  const totalSec = safe / 1000;
  const m = Math.floor(totalSec / 60);
  const s = Math.floor(totalSec % 60);
  const cs = Math.floor((safe % 1000) / 10);
  return `${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}.${String(cs).padStart(2, "0")}`;
}

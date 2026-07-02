import { useEffect, useRef } from "react";
import type { WaveformPeaks } from "../../api/types";
import { timeToX, xToTime, TRACK_HEIGHT, type Viewport } from "../../lib/timeline";

interface Props {
  peaks: WaveformPeaks | null;
  viewport: Viewport;
  onSeek: (ms: number) => void;
}

export function WaveformTrack({ peaks, viewport, onSeek }: Props) {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const dpr = window.devicePixelRatio || 1;
    canvas.width = viewport.visibleWidth * dpr;
    canvas.height = TRACK_HEIGHT * dpr;
    canvas.style.width = `${viewport.visibleWidth}px`;
    canvas.style.height = `${TRACK_HEIGHT}px`;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);

    ctx.clearRect(0, 0, viewport.visibleWidth, TRACK_HEIGHT);

    if (!peaks || peaks.mins.length === 0) {
      ctx.fillStyle = "#52525b";
      ctx.font = "12px sans-serif";
      ctx.fillText("（无波形数据）", 8, TRACK_HEIGHT / 2);
      return;
    }

    const midY = TRACK_HEIGHT / 2;
    const bucketCount = peaks.mins.length;

    ctx.strokeStyle = "#a1a1aa";
    ctx.lineWidth = 1;
    ctx.beginPath();
    for (let x = 0; x < viewport.visibleWidth; x++) {
      const t = xToTime(x, viewport);
      const idx = Math.floor((t / peaks.durationMs) * bucketCount);
      if (idx < 0 || idx >= bucketCount) continue;
      const minNorm = peaks.mins[idx] / 32768;
      const maxNorm = peaks.maxs[idx] / 32768;
      const yMin = midY - maxNorm * midY * 0.9;
      const yMax = midY - minNorm * midY * 0.9;
      ctx.moveTo(x + 0.5, yMin);
      ctx.lineTo(x + 0.5, yMax);
    }
    ctx.stroke();
  }, [peaks, viewport]);

  const handleClick = (e: React.MouseEvent<HTMLCanvasElement>) => {
    const rect = canvasRef.current?.getBoundingClientRect();
    if (!rect) return;
    const x = e.clientX - rect.left;
    onSeek(Math.max(0, xToTime(x, viewport)));
  };

  return (
    <canvas
      ref={canvasRef}
      onClick={handleClick}
      className="cursor-pointer border-b border-border bg-zinc-900"
      style={{ display: "block" }}
    />
  );
}

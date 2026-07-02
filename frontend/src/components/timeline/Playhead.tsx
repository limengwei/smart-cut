import { timeToX, type Viewport } from "../../lib/timeline";

interface Props {
  viewport: Viewport;
  playheadMs: number;
  onSeek: (ms: number) => void;
  totalHeight: number;
}

export function Playhead({ viewport, playheadMs, onSeek, totalHeight }: Props) {
  const x = timeToX(playheadMs, viewport);

  const handleMouseDown = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    const trackEl = document.getElementById("timeline-scroll-area");
    const r = trackEl?.getBoundingClientRect();
    if (!r) return;

    const move = (ev: MouseEvent) => {
      const px = ev.clientX - r.left;
      const ms = Math.max(0, px / viewport.pxPerMs + viewport.scrollMs);
      onSeek(ms);
    };
    const up = () => {
      window.removeEventListener("mousemove", move);
      window.removeEventListener("mouseup", up);
    };
    window.addEventListener("mousemove", move);
    window.addEventListener("mouseup", up);
  };

  if (x < -10 || x > viewport.visibleWidth + 10) return null;

  return (
    <div
      className="pointer-events-none absolute top-0 z-20"
      style={{ left: `${x}px`, height: totalHeight }}
    >
      <div className="w-0.5 h-full bg-yellow-400" />
      <div
        className="pointer-events-auto absolute -left-1.5 -top-0.5 h-3 w-3.5 cursor-ew-resize rounded-t bg-yellow-400"
        onMouseDown={handleMouseDown}
      />
    </div>
  );
}

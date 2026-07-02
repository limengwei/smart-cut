import { useRef } from "react";
import type { CutList, CutSegment } from "../../api/types";
import { timeToX, xToTime, clampMs, type Viewport } from "../../lib/timeline";

interface Props {
  cutList: CutList | null;
  viewport: Viewport;
  selectedSegmentId: string | null;
  onSelect: (id: string | null) => void;
  onToggle: (segID: string) => void;
  onDragBoundary: (seg: CutSegment, side: "start" | "end", newMs: number) => void;
  onAddManual: (startMs: number, endMs: number) => void;
}

const CUT_HEIGHT = 56;
const HANDLE_WIDTH = 8;

export function CutTrack({
  cutList,
  viewport,
  selectedSegmentId,
  onSelect,
  onToggle,
  onDragBoundary,
  onAddManual,
}: Props) {
  const dragRef = useRef<{ seg: CutSegment; side: "start" | "end" } | null>(null);
  const segments: CutSegment[] = cutList?.segments ?? [];

  const handleMouseDownBoundary = (
    e: React.MouseEvent,
    seg: CutSegment,
    side: "start" | "end"
  ) => {
    e.stopPropagation();
    e.preventDefault();
    dragRef.current = { seg, side };

    const move = (ev: MouseEvent) => {
      if (!dragRef.current) return;
      const trackEl = document.getElementById("cut-track-inner");
      const r = trackEl?.getBoundingClientRect();
      if (!r) return;
      const x = ev.clientX - r.left;
      const ms = clampMs(xToTime(x, viewport), viewport.durationMs);
      onDragBoundary(dragRef.current.seg, dragRef.current.side, ms);
    };
    const up = () => {
      dragRef.current = null;
      window.removeEventListener("mousemove", move);
      window.removeEventListener("mouseup", up);
    };
    window.addEventListener("mousemove", move);
    window.addEventListener("mouseup", up);
  };

  const handleDoubleClickEmpty = (e: React.MouseEvent) => {
    if (e.target !== e.currentTarget) return;
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    const x = e.clientX - rect.left;
    const startMs = clampMs(xToTime(x, viewport), viewport.durationMs);
    const endMs = clampMs(startMs + 1000, viewport.durationMs);
    if (endMs > startMs) onAddManual(startMs, endMs);
  };

  return (
    <div
      className="relative border-b border-border bg-zinc-950"
      style={{ height: CUT_HEIGHT }}
    >
      <div
        id="cut-track-inner"
        className="relative h-full w-full"
        style={{ width: viewport.visibleWidth }}
        onDoubleClick={handleDoubleClickEmpty}
        onClick={(e) => {
          if (e.target === e.currentTarget) onSelect(null);
        }}
      >
        {segments.map((seg) => {
          const x = timeToX(seg.startMs, viewport);
          const width = (seg.endMs - seg.startMs) * viewport.pxPerMs;
          if (x + width < 0 || x > viewport.visibleWidth) return null;
          const isRemove = seg.decision === "remove";
          const isSelected = seg.id === selectedSegmentId;
          const baseColor = isRemove
            ? "bg-red-900/60 border-red-600"
            : "bg-emerald-900/60 border-emerald-600";
          const selectedRing = isSelected ? "ring-2 ring-yellow-400" : "";
          return (
            <div
              key={seg.id}
              className={`absolute top-1 flex items-center justify-center border ${baseColor} ${selectedRing} cursor-pointer text-xs`}
              style={{
                left: `${Math.max(0, x)}px`,
                width: `${Math.max(HANDLE_WIDTH, width)}px`,
                height: CUT_HEIGHT - 8,
              }}
              onClick={(e) => {
                e.stopPropagation();
                onSelect(seg.id);
              }}
              onContextMenu={(e) => {
                e.preventDefault();
                onToggle(seg.id);
              }}
              title={`${seg.decision} | ${seg.reason}${seg.note ? " | " + seg.note : ""}`}
            >
              <div
                className="absolute left-0 top-0 h-full cursor-ew-resize bg-black/30"
                style={{ width: HANDLE_WIDTH }}
                onMouseDown={(e) => handleMouseDownBoundary(e, seg, "start")}
              />
              <span className="pointer-events-none truncate px-2 text-zinc-200">
                {isRemove ? "✂ 删除" : "▶ 保留"}
              </span>
              <div
                className="absolute right-0 top-0 h-full cursor-ew-resize bg-black/30"
                style={{ width: HANDLE_WIDTH }}
                onMouseDown={(e) => handleMouseDownBoundary(e, seg, "end")}
              />
            </div>
          );
        })}
        {segments.length === 0 && (
          <span className="absolute left-2 top-2 text-xs text-zinc-500">
            （无剪切清单，请先分析。双击空白处可手动添加剪切段）
          </span>
        )}
      </div>
    </div>
  );
}

import type { Transcript, Segment } from "../../api/types";
import { timeToX, type Viewport } from "../../lib/timeline";

interface Props {
  transcript: Transcript | null;
  viewport: Viewport;
  onSeek: (ms: number) => void;
}

const SUB_HEIGHT = 56;

export function SubtitleTrack({ transcript, viewport, onSeek }: Props) {
  const segments: Segment[] = transcript?.segments ?? [];

  return (
    <div
      className="relative overflow-hidden border-b border-border bg-zinc-900"
      style={{ height: SUB_HEIGHT }}
    >
      {segments.map((seg) => {
        const x = timeToX(seg.startMs, viewport);
        const width = (seg.endMs - seg.startMs) * viewport.pxPerMs;
        if (x + width < 0 || x > viewport.visibleWidth) return null;
        return (
          <button
            key={seg.id}
            onClick={() => onSeek(seg.startMs)}
            className="absolute top-1 m-px truncate rounded bg-zinc-700 px-1.5 py-1 text-left text-xs text-zinc-200 hover:bg-zinc-600"
            style={{ left: `${Math.max(0, x)}px`, width: `${Math.max(20, width - 2)}px`, height: SUB_HEIGHT - 8 }}
            title={seg.text}
          >
            <span className="line-clamp-2">{seg.text}</span>
          </button>
        );
      })}
      {segments.length === 0 && (
        <span className="absolute left-2 top-2 text-xs text-zinc-500">（无字幕数据，请先转录）</span>
      )}
    </div>
  );
}

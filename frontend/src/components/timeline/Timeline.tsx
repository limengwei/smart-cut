import { useEffect, useRef, useState } from "react";
import type { Transcript, CutList, WaveformPeaks, CutSegment } from "../../api/types";
import { buildViewport, type Viewport } from "../../lib/timeline";
import { WaveformTrack } from "./WaveformTrack";
import { SubtitleTrack } from "./SubtitleTrack";
import { CutTrack } from "./CutTrack";
import { Playhead } from "./Playhead";
import { ZoomControls } from "./ZoomControls";

interface Props {
  durationMs: number;
  transcript: Transcript | null;
  cutList: CutList | null;
  peaks: WaveformPeaks | null;
  zoom: number;
  scrollMs: number;
  playheadMs: number;
  selectedSegmentId: string | null;
  onSeek: (ms: number) => void;
  onSetScroll: (ms: number) => void;
  onZoomIn: () => void;
  onZoomOut: () => void;
  onZoomFit: () => void;
  onSelectSegment: (id: string | null) => void;
  onToggleSegment: (segID: string) => void;
  onDeleteSegment: (segID: string) => void;
  onDragBoundary: (seg: CutSegment, side: "start" | "end", newMs: number) => void;
  onAddManual: (startMs: number, endMs: number) => void;
}

export function Timeline(props: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(1000);

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const update = () => setWidth(el.clientWidth);
    update();
    const ro = new ResizeObserver(update);
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  const viewport: Viewport = buildViewport(props.durationMs, width, props.zoom, props.scrollMs);

  const totalHeight = 80 + 56 + 56;

  const handleScroll = (e: React.UIEvent<HTMLDivElement>) => {
    const el = e.currentTarget;
    if (viewport.pxPerMs <= 0) return;
    const scrollMs = el.scrollLeft / viewport.pxPerMs;
    props.onSetScroll(scrollMs);
  };

  return (
    <div className="flex flex-col border-t border-border bg-background">
      <div className="flex items-center justify-between px-3 py-1.5">
        <span className="text-xs font-medium text-muted-foreground">时间轴</span>
        <ZoomControls
          zoom={props.zoom}
          onZoomIn={props.onZoomIn}
          onZoomOut={props.onZoomOut}
          onFit={props.onZoomFit}
        />
      </div>

      <div
        ref={containerRef}
        id="timeline-scroll-area"
        className="relative overflow-x-auto overflow-y-hidden"
        onScroll={handleScroll}
      >
        <div style={{ width: `${Math.max(width, props.durationMs * viewport.pxPerMs)}px` }} className="relative">
          <WaveformTrack peaks={props.peaks} viewport={viewport} onSeek={props.onSeek} />
          <SubtitleTrack transcript={props.transcript} viewport={viewport} onSeek={props.onSeek} />
          <CutTrack
            cutList={props.cutList}
            viewport={viewport}
            selectedSegmentId={props.selectedSegmentId}
            onSelect={props.onSelectSegment}
            onToggle={props.onToggleSegment}
            onDelete={props.onDeleteSegment}
            onDragBoundary={props.onDragBoundary}
            onAddManual={props.onAddManual}
          />
          <Playhead
            viewport={viewport}
            playheadMs={props.playheadMs}
            onSeek={props.onSeek}
            totalHeight={totalHeight}
          />
        </div>
      </div>
    </div>
  );
}

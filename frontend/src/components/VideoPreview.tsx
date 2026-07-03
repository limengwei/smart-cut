import { useEffect, useRef } from "react";
import { Play, Pause, SkipBack, SkipForward } from "lucide-react";
import { Button } from "./ui/button";
import { formatTimecode } from "../lib/timeline";
import { RemotionPlayer } from "./RemotionPlayer";
import { useWorkbenchStore } from "../stores/workbench";

interface Props {
  src: string | null;
  isPlaying: boolean;
  playheadMs: number;
  loopSegment: { startMs: number; endMs: number } | null;
  onTimeUpdate: (ms: number) => void;
  onTogglePlay: () => void;
  onSeek: (ms: number) => void;
  durationMs: number;
  mediaWidth: number;
  mediaHeight: number;
  mediaFps: number;
}

export function VideoPreview({
  src,
  isPlaying,
  playheadMs,
  loopSegment,
  onTimeUpdate,
  onTogglePlay,
  onSeek,
  durationMs,
  mediaWidth,
  mediaHeight,
  mediaFps,
}: Props) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const subtitleEnabled = useWorkbenchStore((s) => s.subtitleEnabled);
  const subtitleConfig = useWorkbenchStore((s) => s.subtitleConfig);

  useEffect(() => {
    const v = videoRef.current;
    if (!v) return;
    if (isPlaying) {
      v.play().catch(() => {});
    } else {
      v.pause();
    }
  }, [isPlaying]);

  useEffect(() => {
    const v = videoRef.current;
    if (!v) return;
    const delta = Math.abs(v.currentTime * 1000 - playheadMs);
    if (delta > 350) {
      v.currentTime = playheadMs / 1000;
    }
  }, [playheadMs]);

  const handleTimeUpdate = () => {
    const v = videoRef.current;
    if (!v) return;
    const ms = v.currentTime * 1000;
    onTimeUpdate(ms);
    if (loopSegment && ms >= loopSegment.endMs) {
      v.currentTime = loopSegment.startMs / 1000;
    }
  };

  return (
    <div className="flex flex-1 flex-col items-center justify-center bg-zinc-950 p-4">
      <div className="relative w-full max-w-3xl">
        {src ? (
          <>
            <video
              ref={videoRef}
              src={src}
              onTimeUpdate={handleTimeUpdate}
              onLoadedMetadata={(e) => (e.currentTarget.volume = 1)}
              className="w-full rounded-lg"
              controls={false}
            />
            {subtitleEnabled && subtitleConfig && subtitleConfig.segments.length > 0 && (
              <div className="pointer-events-none absolute inset-0">
                <RemotionPlayer
                  config={subtitleConfig}
                  playheadMs={playheadMs}
                  durationMs={durationMs}
                  width={mediaWidth}
                  height={mediaHeight}
                  fps={mediaFps}
                />
              </div>
            )}
          </>
        ) : (
          <div className="flex aspect-video w-full items-center justify-center rounded-lg border border-border bg-zinc-900 text-muted-foreground">
            （无媒体）
          </div>
        )}
      </div>

      <div className="mt-3 flex w-full max-w-3xl items-center justify-center gap-3">
        <Button variant="ghost" size="icon" onClick={() => onSeek(Math.max(0, playheadMs - 5000))} title="后退 5 秒">
          <SkipBack className="h-4 w-4" />
        </Button>
        <Button variant="default" size="icon" onClick={onTogglePlay} title={isPlaying ? "暂停" : "播放"}>
          {isPlaying ? <Pause className="h-4 w-4" /> : <Play className="h-4 w-4" />}
        </Button>
        <Button variant="ghost" size="icon" onClick={() => onSeek(Math.min(durationMs, playheadMs + 5000))} title="前进 5 秒">
          <SkipForward className="h-4 w-4" />
        </Button>
        <span className="ml-2 font-mono text-sm text-muted-foreground">
          {formatTimecode(playheadMs)} / {formatTimecode(durationMs)}
        </span>
      </div>
    </div>
  );
}

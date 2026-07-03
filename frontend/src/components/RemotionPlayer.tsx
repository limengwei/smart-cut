import { useEffect, useRef } from "react";
import { Player, PlayerRef } from "@remotion/player";
import { SubtitleComp } from "../remotion/SubtitleComp";
import type { SubtitleConfig } from "../api/types";

interface Props {
  config: SubtitleConfig;
  playheadMs: number;
  durationMs: number;
  width: number;
  height: number;
  fps: number;
}

const FPS_FALLBACK = 30;

export function RemotionPlayer({ config, playheadMs, durationMs, width, height, fps }: Props) {
  const playerRef = useRef<PlayerRef>(null);
  const effectiveFps = fps > 0 ? fps : FPS_FALLBACK;
  const durationInFrames = Math.max(1, Math.round((durationMs / 1000) * effectiveFps));
  const frameFromMs = (ms: number) => Math.round((ms / 1000) * effectiveFps);

  useEffect(() => {
    const player = playerRef.current;
    if (!player) return;
    const targetFrame = frameFromMs(playheadMs);
    const currentFrame = player.getCurrentFrame();
    if (Math.abs(targetFrame - currentFrame) > 1) {
      player.seekTo(frameFromMs(playheadMs));
    }
  }, [playheadMs, effectiveFps]);

  return (
    <Player
      ref={playerRef}
      component={SubtitleComp}
      inputProps={{ segments: config.segments, style: config.style }}
      durationInFrames={durationInFrames}
      fps={effectiveFps}
      compositionWidth={width}
      compositionHeight={height}
      style={{ width: "100%", height: "100%" }}
      autoPlay={false}
      loop={false}
      controls={false}
    />
  );
}

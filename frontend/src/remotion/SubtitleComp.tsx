import { AbsoluteFill, useCurrentFrame, useVideoConfig } from "remotion";
import type { SubtitleStyle } from "../api/types";

interface SubtitleCompSegment {
  id: number;
  text: string;
  startMs: number;
  endMs: number;
}

interface Props {
  segments: SubtitleCompSegment[];
  style: SubtitleStyle;
}

function positionToStyle(position: string): React.CSSProperties {
  switch (position) {
    case "top":
      return { top: "8%" };
    case "center":
      return { top: "50%", transform: "translateY(-50%)" };
    case "bottom":
    default:
      return { bottom: "12%" };
  }
}

function hexToRgba(hex: string, opacity: number): string {
  const h = hex.replace("#", "");
  if (h.length !== 6) return hex;
  const r = parseInt(h.slice(0, 2), 16);
  const g = parseInt(h.slice(2, 4), 16);
  const b = parseInt(h.slice(4, 6), 16);
  return `rgba(${r}, ${g}, ${b}, ${opacity})`;
}

export const SubtitleComp: React.FC<Props> = ({ segments, style }) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const timeMs = (frame / fps) * 1000;

  const active = segments.find((s) => timeMs >= s.startMs && timeMs < s.endMs);
  if (!active) return <AbsoluteFill />;

  const fontFamily = style.fontFamily || "sans-serif";
  const fontSize = style.fontSize || 48;
  const color = style.color || "#FFFFFF";
  const highlight = style.highlight || color;
  const bgColor = style.bgColor
    ? hexToRgba(style.bgColor, style.bgOpacity ?? 0.6)
    : "transparent";

  return (
    <AbsoluteFill>
      <div
        style={{
          position: "absolute",
          left: "50%",
          transform: "translateX(-50%)",
          maxWidth: "80%",
          ...positionToStyle(style.position),
          fontFamily,
          fontSize,
          fontWeight: "bold",
          color: highlight,
          backgroundColor: bgColor,
          padding: "0.3em 0.6em",
          borderRadius: "0.2em",
          textAlign: "center",
          lineHeight: 1.4,
          whiteSpace: "pre-wrap",
        }}
      >
        {active.text}
      </div>
    </AbsoluteFill>
  );
};

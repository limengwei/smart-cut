import { AbsoluteFill, useCurrentFrame, useVideoConfig } from "remotion";
import type { OverlayItem, OverlayStyle } from "../api/types";

interface Props {
  items: OverlayItem[];
  style: OverlayStyle;
}

function positionToStyle(pos: OverlayItem["position"]): React.CSSProperties {
  const style: React.CSSProperties = { position: "absolute" };
  if (pos.x === "center") { style.left = "50%"; style.transform = "translateX(-50%)"; }
  else if (pos.x === "left") style.left = "5%";
  else style.right = "5%";
  if (pos.y === "center") style.top = "50%";
  else if (pos.y === "top") style.top = "8%";
  else style.bottom = "12%";
  return style;
}

function getAnimatedStyle(item: OverlayItem, timeMs: number): React.CSSProperties {
  const localTime = timeMs - item.startMs;
  const animDuration = (item.animation && item.animation.duration) || 300;
  let opacity = 1;
  let translateY = 0;
  let scale = 1;

  if (localTime < animDuration) {
    const progress = localTime / animDuration;
    switch (item.animation?.in) {
      case "fade": opacity = progress; break;
      case "slide": opacity = progress; translateY = (1 - progress) * 40; break;
      case "scale": opacity = progress; scale = 0.8 + 0.2 * progress; break;
    }
  }
  const totalDuration = item.endMs - item.startMs;
  const exitStart = totalDuration - animDuration;
  if (localTime > exitStart) {
    const exitProgress = (localTime - exitStart) / animDuration;
    switch (item.animation?.out) {
      case "fade": opacity = 1 - exitProgress; break;
      case "slide": opacity = 1 - exitProgress; translateY = exitProgress * 40; break;
      case "scale": opacity = 1 - exitProgress; scale = 1 - 0.2 * exitProgress; break;
    }
  }

  return { opacity, transform: `translateY(${translateY}px) scale(${scale})` };
}

const OverlayCard: React.FC<{ item: OverlayItem; style: OverlayStyle }> = ({ item, style }) => {
  const c = item.content;
  const accentColor = c.accentColor || style.accentColor || "#3b82f6";
  const bgColor = style.cardBgColor || "rgba(0,0,0,0.85)";
  const borderRadius = style.cardRadius || 12;
  const fontFamily = style.fontFamily || "sans-serif";

  return (
    <div style={{
      backgroundColor: bgColor,
      borderRadius,
      padding: "20px 28px",
      color: "#fff",
      borderLeft: `4px solid ${accentColor}`,
      fontFamily,
      maxWidth: "80%",
    }}>
      {(c.icon || c.title) && (
        <div style={{ fontSize: 24, fontWeight: "bold", marginBottom: c.bigNumber ? 8 : 12 }}>
          {c.icon ? c.icon + " " : ""}{c.title}
        </div>
      )}
      {c.bigNumber && (
        <div style={{ fontSize: 64, fontWeight: "bold", color: accentColor, lineHeight: 1.1 }}>
          {c.bigNumber}
        </div>
      )}
      {c.body && (
        <div style={{ fontSize: 18, lineHeight: 1.6, opacity: 0.9 }}>{c.body}</div>
      )}
      {c.bulletPoints && c.bulletPoints.length > 0 && (
        <ul style={{ fontSize: 18, lineHeight: 1.8, paddingLeft: 24, margin: 0 }}>
          {c.bulletPoints.map((p, i) => <li key={i}>{p}</li>)}
        </ul>
      )}
    </div>
  );
};

const FullscreenCard: React.FC<{ item: OverlayItem; style: OverlayStyle }> = ({ item, style }) => {
  const c = item.content;
  const bgColor = c.bgColor || "#0f172a";
  const accentColor = c.accentColor || style.accentColor || "#3b82f6";
  const fontFamily = style.fontFamily || "sans-serif";

  return (
    <AbsoluteFill style={{ backgroundColor: bgColor, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", padding: "60px 80px", fontFamily }}>
      {c.icon && <div style={{ fontSize: 72, marginBottom: 24 }}>{c.icon}</div>}
      {c.title && <div style={{ fontSize: 40, fontWeight: "bold", color: "#f5f5f7", textAlign: "center", marginBottom: 16 }}>{c.title}</div>}
      {c.bigNumber && <div style={{ fontSize: 96, fontWeight: "bold", color: accentColor, lineHeight: 1 }}>{c.bigNumber}</div>}
      {c.body && <div style={{ fontSize: 22, color: "#94a3b8", marginTop: 16, textAlign: "center" }}>{c.body}</div>}
      {c.bulletPoints && c.bulletPoints.length > 0 && (
        <div style={{ fontSize: 24, color: "#cbd5e1", lineHeight: 2.2, textAlign: "center", marginTop: 16 }}>
          {c.bulletPoints.map((p, i) => <div key={i}>{p}</div>)}
        </div>
      )}
    </AbsoluteFill>
  );
};

export const OverlayComp: React.FC<Props> = ({ items, style }) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const timeMs = (frame / fps) * 1000;

  const activeItems = items.filter(
    item => timeMs >= item.startMs && timeMs < item.endMs
  );

  if (activeItems.length === 0) return <AbsoluteFill />;

  return (
    <AbsoluteFill>
      {activeItems.map(item => {
        const animStyle = getAnimatedStyle(item, timeMs);
        const posStyle = positionToStyle(item.position);
        return (
          <div key={item.id} style={{ ...posStyle, ...animStyle }}>
            {item.mode === "fullscreen"
              ? <FullscreenCard item={item} style={style} />
              : <OverlayCard item={item} style={style} />
            }
          </div>
        );
      })}
    </AbsoluteFill>
  );
};
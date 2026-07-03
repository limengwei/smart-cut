import { bundle } from "@remotion/bundler";
import { renderMedia, selectComposition } from "@remotion/renderer";
import path from "path";
import { fileURLToPath } from "url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

function readStdin() {
  return new Promise((resolve, reject) => {
    let data = "";
    process.stdin.setEncoding("utf8");
    process.stdin.on("data", (chunk) => (data += chunk));
    process.stdin.on("end", () => resolve(data.trim()));
    process.stdin.on("error", reject);
  });
}

function emit(obj) {
  process.stdout.write(JSON.stringify(obj) + "\n");
}

function fail(message) {
  emit({ type: "error", message });
  process.exit(1);
}

async function main() {
  const raw = await readStdin();
  if (!raw) {
    fail("empty stdin");
    return;
  }

  let input;
  try {
    input = JSON.parse(raw);
  } catch (e) {
    fail("invalid JSON: " + e.message);
    return;
  }

  const { startMs, endMs, segments, style, width, height, fps, outputPath } = input;
  const effectiveFps = fps > 0 ? fps : 30;
  const safeWidth = width > 0 ? width : 1920;
  const safeHeight = height > 0 ? height : 1080;
  const durationMs = endMs - startMs;
  if (!(durationMs > 0)) {
    fail("invalid segment duration: endMs=" + endMs + " startMs=" + startMs);
    return;
  }
  const durationInFrames = Math.max(1, Math.round((durationMs / 1000) * effectiveFps));

  if (!outputPath) {
    fail("missing outputPath");
    return;
  }

  emit({ type: "progress", progress: 0 });

  const entryPath = path.join(__dirname, "entry.tsx");
  let serveUrl;
  try {
    serveUrl = await bundle({ entryPoint: entryPath });
  } catch (e) {
    fail("bundle failed: " + e.message);
    return;
  }
  emit({ type: "progress", progress: 0.5 });

  let comp;
  try {
    comp = await selectComposition({
      serveUrl,
      id: "subtitle",
      inputProps: { segments, style },
    });
  } catch (e) {
    fail("selectComposition failed: " + e.message);
    return;
  }
  emit({ type: "progress", progress: 0.6 });

  try {
    await renderMedia({
      composition: { ...comp, durationInFrames, fps: effectiveFps, width: safeWidth, height: safeHeight },
      serveUrl,
      codec: "vp8",
      imageFormat: "png",
      outputLocation: outputPath,
      onProgress: ({ progress }) => emit({ type: "progress", progress: 0.6 + progress * 0.4 }),
    });
  } catch (e) {
    fail("renderMedia failed: " + e.message);
    return;
  }

  emit({ type: "done", outputPath });
}

main().catch((e) => {
  fail(String(e && e.message ? e.message : e));
});

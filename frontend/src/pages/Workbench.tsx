import { useEffect, useCallback } from "react";
import { useParams } from "react-router-dom";
import { Mic, BrainCircuit, Download, Loader2 } from "lucide-react";
import { Button } from "../components/ui/button";
import { Timeline } from "../components/timeline/Timeline";
import { VideoPreview } from "../components/VideoPreview";
import { SubtitleStylePanel } from "../components/SubtitleStylePanel";
import { AISuggestions } from "../components/AISuggestions";
import { useWorkbenchStore, type WorkflowStage } from "../stores/workbench";
import { useProjectStore } from "../stores/project";
import {
  getProject,
  getTranscript,
  getCutList,
  getWaveformPeaks,
  getMediaURL,
  startTranscribe,
  startAnalyze,
  startExport,
  addCutSegment,
  updateCutSegment,
  removeCutSegment,
  saveProject,
  getSubtitleConfig,
} from "../api/client";
import {
  onProgress,
  onTranscriptReady,
  onTranscriptSegment,
  onCutListReady,
} from "../api/events";
import type { CutSegment, ExportOptions, SubtitleStyle } from "../api/types";

const stageButtonConfig: Record<
  string,
  { label: string; icon: typeof Mic; stage: WorkflowStage }
> = {
  transcribe: { label: "转录", icon: Mic, stage: "transcribing" },
  analyze: { label: "AI 分析", icon: BrainCircuit, stage: "analyzing" },
  export: { label: "导出", icon: Download, stage: "exporting" },
};
void stageButtonConfig;

const defaultSubtitleStyle: SubtitleStyle = {
  fontFamily: "sans-serif",
  fontSize: 48,
  color: "#FFFFFF",
  highlight: "#FFEB3B",
  position: "bottom",
  bgColor: "#000000",
  bgOpacity: 0.6,
};

export function Workbench() {
  const { id } = useParams<{ id: string }>();
  const wb = useWorkbenchStore();
  const currentProject = useProjectStore((s) => s.currentProject);
  const setCurrentProject = useProjectStore((s) => s.setCurrentProject);

  const loadAll = useCallback(async (projectID: string) => {
    wb.setLoading(true);
    wb.setError("");
    try {
      const project = currentProject?.id === projectID ? currentProject : await getProject(projectID);
      setCurrentProject(project);
      wb.setProjectID(projectID);

      const [url] = await Promise.all([getMediaURL(projectID)]);
      wb.setMediaURL(url);

      try {
        const t = await getTranscript(projectID);
        wb.setTranscript(t);
      } catch {
        wb.setTranscript(null);
      }
      try {
        const c = await getCutList(projectID);
        wb.setCutList(c);
        wb.setStage("ready");
      } catch {
        wb.setCutList(null);
      }
      try {
        const p = await getWaveformPeaks(projectID);
        wb.setPeaks(p);
      } catch {
        wb.setPeaks(null);
      }
      try {
        const sc = await getSubtitleConfig(projectID);
        wb.setSubtitleConfig(sc);
        wb.setSubtitleStyle(sc.style);
        wb.setSubtitleEnabled(sc.segments.length > 0);
      } catch {
        wb.setSubtitleConfig(null);
      }
    } catch (e) {
      wb.setError("加载数据失败: " + String(e));
    } finally {
      wb.setLoading(false);
    }
  }, [currentProject, setCurrentProject, wb]);

  useEffect(() => {
    if (!id) return;
    loadAll(id);
    return () => wb.reset();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  useEffect(() => {
    const off1 = onProgress((ev) => {
      wb.setProgress(ev.progress, ev.step);
      // 任务完成 → 切回 ready；任务运行中 → 对应进行态
      if (ev.status === "done") {
        wb.setStage("ready");
        wb.setProgress(1, "完成");
        return;
      }
      if (ev.stage === "transcribe" && ev.status === "running") wb.setStage("transcribing");
      if (ev.stage === "analyze" && ev.status === "running") wb.setStage("analyzing");
      if (ev.stage === "export" && ev.status === "running") wb.setStage("exporting");
      if (ev.stage === "subtitle" && ev.status === "running") {
        // subtitle 进度在 export 流程内，仅记录 step
      }
      if (ev.status === "error") wb.setError(ev.error ?? "任务失败");
    });
    const off2 = onTranscriptReady((t) => {
      wb.setTranscript(t);
      // 转录完成 → 切回 ready（兜底，progress done 可能已触发，这里确保复位）
      wb.setStage("ready");
      if (id) getWaveformPeaks(id).then(wb.setPeaks).catch(() => {});
    });
    // 流式字幕：每收到一句就增量追加到字幕轨
    const off2b = onTranscriptSegment((seg) => {
      wb.appendSegment(seg);
    });
    const off3 = onCutListReady((c) => {
      wb.setCutList(c);
      wb.setStage("ready");
    });
    return () => {
      off1();
      off2();
      off2b();
      off3();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  const durationMs = currentProject?.media.durationMs ?? 0;

  const handleSeek = (ms: number) => {
    wb.setPlayhead(Math.max(0, Math.min(ms, durationMs)));
  };

  const handleTranscribe = async () => {
    if (!id) return;
    wb.setStage("transcribing");
    wb.setProgress(0, "启动转录");
    wb.setTranscript(null); // 清空旧字幕，准备流式接收
    try {
      await startTranscribe(id);
    } catch (e) {
      wb.setError("启动转录失败: " + String(e));
      wb.setStage("idle");
    }
  };

  const handleAnalyze = async () => {
    if (!id) return;
    wb.setStage("analyzing");
    wb.setProgress(0, "启动分析");
    try {
      await startAnalyze(id);
    } catch (e) {
      wb.setError("启动分析失败: " + String(e));
      wb.setStage("ready");
    }
  };

  const handleExport = async () => {
    if (!id) return;
    wb.setStage("exporting");
    wb.setProgress(0, "启动导出");
    const opts: ExportOptions = {
      mode: "lossless",
      includeSubtitle: false,
      outputPath: `${currentProject?.workDir ?? "."}/export.mp4`,
    };
    try {
      await startExport(id, opts);
    } catch (e) {
      wb.setError("启动导出失败: " + String(e));
      wb.setStage("ready");
    }
  };

  const handleToggleSegment = async (segID: string) => {
    if (!id || !wb.cutList) return;
    const seg = wb.cutList.segments.find((s) => s.id === segID);
    if (!seg) return;
    const updated: CutSegment = {
      ...seg,
      decision: seg.decision === "keep" ? "remove" : "keep",
    };
    try {
      await updateCutSegment(id, updated);
      wb.setCutList({ ...wb.cutList, segments: wb.cutList.segments.map((s) => (s.id === segID ? updated : s)) });
    } catch (e) {
      wb.setError("切换失败: " + String(e));
    }
  };

  const handleDeleteSegment = async (segID: string) => {
    if (!id || !wb.cutList) return;
    try {
      await removeCutSegment(id, segID);
      const fresh = await getCutList(id);
      wb.setCutList(fresh);
      if (wb.selectedSegmentId === segID) wb.selectSegment(null);
    } catch (e) {
      wb.setError("删除失败: " + String(e));
    }
  };

  const handleAccept = async (segID: string) => {
    if (!id || !wb.cutList) return;
    const seg = wb.cutList.segments.find((s) => s.id === segID);
    if (!seg) return;
    const updated: CutSegment = { ...seg, decision: "keep" };
    try {
      await updateCutSegment(id, updated);
      wb.setCutList({ ...wb.cutList, segments: wb.cutList.segments.map((s) => (s.id === segID ? updated : s)) });
    } catch (e) {
      wb.setError("操作失败: " + String(e));
    }
  };

  const handleDragBoundary = async (seg: CutSegment, side: "start" | "end", newMs: number) => {
    if (!id || !wb.cutList) return;
    const updated: CutSegment =
      side === "start"
        ? { ...seg, startMs: Math.min(newMs, seg.endMs - 50) }
        : { ...seg, endMs: Math.max(newMs, seg.startMs + 50) };
    try {
      await updateCutSegment(id, updated);
      wb.setCutList({ ...wb.cutList, segments: wb.cutList.segments.map((s) => (s.id === seg.id ? updated : s)) });
    } catch (e) {
      wb.setError("调整失败: " + String(e));
    }
  };

  const handleAddManual = async (startMs: number, endMs: number) => {
    if (!id) return;
    const newSeg: CutSegment = {
      id: `manual-${Date.now()}`,
      startMs,
      endMs,
      decision: "remove",
      reason: "manual",
      source: "manual",
      confidence: 1,
      note: "",
    };
    try {
      await addCutSegment(id, newSeg);
      const fresh = await getCutList(id);
      wb.setCutList(fresh);
    } catch (e) {
      wb.setError("添加失败: " + String(e));
    }
  };

  const handleSubtitleStyleChange = async (s: SubtitleStyle) => {
    wb.setSubtitleStyle(s);
    if (currentProject) {
      const updated = { ...currentProject, settings: { ...currentProject.settings, subtitleStyle: s } };
      setCurrentProject(updated);
      try {
        await saveProject(updated);
        if (wb.subtitleConfig) wb.setSubtitleConfig({ ...wb.subtitleConfig, style: s });
      } catch (e) {
        wb.setError("保存字幕样式失败: " + String(e));
      }
    }
  };

  const loopSegment = wb.selectedSegmentId && wb.cutList
    ? (() => {
        const seg = wb.cutList.segments.find((s) => s.id === wb.selectedSegmentId);
        return seg ? { startMs: seg.startMs, endMs: seg.endMs } : null;
      })()
    : null;

  const busy = wb.stage === "transcribing" || wb.stage === "analyzing" || wb.stage === "exporting";

  return (
    <div className="flex h-full flex-col">
      {/* TopBar */}
      <div className="flex items-center justify-between border-b border-border bg-zinc-900 px-4 py-2">
        <div className="flex items-center gap-3">
          <h1 className="text-sm font-semibold">{currentProject?.name ?? "工作台"}</h1>
          {busy && (
            <span className="flex items-center gap-1 text-xs text-muted-foreground">
              <Loader2 className="h-3 w-3 animate-spin" />
              {wb.stage === "transcribing" ? "转录中" : wb.stage === "analyzing" ? "分析中" : "导出中"}
              {wb.progress > 0 ? ` ${(wb.progress * 100).toFixed(0)}%` : ""}
            </span>
          )}
        </div>
        <div className="flex items-center gap-2">
          <Button size="sm" variant="outline" onClick={handleTranscribe} disabled={busy}>
            <Mic className="mr-1.5 h-3.5 w-3.5" /> 转录
          </Button>
          <Button size="sm" variant="outline" onClick={handleAnalyze} disabled={busy || !wb.transcript}>
            <BrainCircuit className="mr-1.5 h-3.5 w-3.5" /> 分析
          </Button>
          <Button size="sm" variant="default" onClick={handleExport} disabled={busy || !wb.cutList}>
            <Download className="mr-1.5 h-3.5 w-3.5" /> 导出
          </Button>
        </div>
      </div>

      {wb.error && (
        <div className="bg-red-950/60 px-4 py-1.5 text-xs text-red-200">
          {wb.error}
        </div>
      )}

      {/* 主区域 */}
      <div className="flex flex-1 overflow-hidden">
        <VideoPreview
          src={wb.mediaURL}
          isPlaying={wb.isPlaying}
          playheadMs={wb.playheadMs}
          loopSegment={loopSegment}
          onTimeUpdate={wb.setPlayhead}
          onTogglePlay={() => wb.setPlaying(!wb.isPlaying)}
          onSeek={handleSeek}
          durationMs={durationMs}
          mediaWidth={currentProject?.media.width ?? 1920}
          mediaHeight={currentProject?.media.height ?? 1080}
          mediaFps={currentProject?.media.fps ?? 30}
        />
        <SubtitleStylePanel
          enabled={wb.subtitleEnabled}
          style={wb.subtitleStyle ?? defaultSubtitleStyle}
          onToggleEnabled={wb.setSubtitleEnabled}
          onChangeStyle={handleSubtitleStyleChange}
        />
        <AISuggestions
          cutList={wb.cutList}
          selectedSegmentId={wb.selectedSegmentId}
          onSelect={wb.selectSegment}
          onAccept={handleAccept}
          onReject={handleDeleteSegment}
          loading={wb.stage === "analyzing"}
        />
      </div>

      {/* Timeline */}
      <Timeline
        durationMs={durationMs}
        transcript={wb.transcript}
        cutList={wb.cutList}
        peaks={wb.peaks}
        zoom={wb.zoom}
        scrollMs={wb.scrollMs}
        playheadMs={wb.playheadMs}
        selectedSegmentId={wb.selectedSegmentId}
        onSeek={handleSeek}
        onSetScroll={wb.setScroll}
        onZoomIn={() => wb.zoomBy(1.5, durationMs, 1000)}
        onZoomOut={() => wb.zoomBy(1 / 1.5, durationMs, 1000)}
        onZoomFit={wb.zoomFit}
        onSelectSegment={wb.selectSegment}
        onToggleSegment={handleToggleSegment}
        onDeleteSegment={handleDeleteSegment}
        onDragBoundary={handleDragBoundary}
        onAddManual={handleAddManual}
      />
    </div>
  );
}

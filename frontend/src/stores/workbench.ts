import { create } from "zustand";
import type {
  Transcript,
  CutList,
  WaveformPeaks,
  Segment,
  SubtitleStyle,
  SubtitleConfig,
} from "../api/types";

export type WorkflowStage = "idle" | "transcribing" | "analyzing" | "ready" | "exporting";

interface WorkbenchStore {
  projectID: string | null;
  transcript: Transcript | null;
  cutList: CutList | null;
  peaks: WaveformPeaks | null;
  mediaURL: string | null;

  loading: boolean;
  error: string;

  stage: WorkflowStage;
  progress: number;
  stageStep: string;

  playheadMs: number;
  isPlaying: boolean;

  zoom: number;
  scrollMs: number;

  selectedSegmentId: string | null;

  subtitleEnabled: boolean;
  subtitleStyle: SubtitleStyle | null;
  subtitleConfig: SubtitleConfig | null;

  setSubtitleEnabled: (b: boolean) => void;
  setSubtitleStyle: (s: SubtitleStyle) => void;
  setSubtitleConfig: (c: SubtitleConfig | null) => void;

  setProjectID: (id: string) => void;
  setTranscript: (t: Transcript | null) => void;
  appendSegment: (seg: Segment) => void;
  setCutList: (c: CutList | null) => void;
  setPeaks: (p: WaveformPeaks | null) => void;
  setMediaURL: (u: string | null) => void;

  setLoading: (b: boolean) => void;
  setError: (e: string) => void;

  setStage: (s: WorkflowStage) => void;
  setProgress: (p: number, step: string) => void;

  setPlayhead: (ms: number) => void;
  setPlaying: (p: boolean) => void;

  setZoom: (z: number) => void;
  setScroll: (ms: number) => void;
  zoomBy: (factor: number, durationMs: number, visibleWidth: number) => void;
  zoomFit: () => void;

  selectSegment: (id: string | null) => void;

  reset: () => void;
}

export const useWorkbenchStore = create<WorkbenchStore>((set, get) => ({
  projectID: null,
  transcript: null,
  cutList: null,
  peaks: null,
  mediaURL: null,

  loading: false,
  error: "",

  stage: "idle",
  progress: 0,
  stageStep: "",

  playheadMs: 0,
  isPlaying: false,

  zoom: 1,
  scrollMs: 0,

  selectedSegmentId: null,

  subtitleEnabled: false,
  subtitleStyle: null,
  subtitleConfig: null,

  setSubtitleEnabled: (b) => set({ subtitleEnabled: b }),
  setSubtitleStyle: (s) => set({ subtitleStyle: s }),
  setSubtitleConfig: (c) => set({ subtitleConfig: c }),

  setProjectID: (id) => set({ projectID: id }),
  setTranscript: (t) => set({ transcript: t }),
  appendSegment: (seg) =>
    set((state) => {
      if (!state.transcript) {
        return { transcript: { language: "", text: "", segments: [seg] } };
      }
      return { transcript: { ...state.transcript, segments: [...state.transcript.segments, seg] } };
    }),
  setCutList: (c) => set({ cutList: c }),
  setPeaks: (p) => set({ peaks: p }),
  setMediaURL: (u) => set({ mediaURL: u }),

  setLoading: (b) => set({ loading: b }),
  setError: (e) => set({ error: e }),

  setStage: (s) => set({ stage: s }),
  setProgress: (p, step) => set({ progress: p, stageStep: step }),

  setPlayhead: (ms) => set({ playheadMs: ms }),
  setPlaying: (p) => set({ isPlaying: p }),

  setZoom: (z) => set({ zoom: Math.max(1, Math.min(z, 50)) }),
  setScroll: (ms) => set({ scrollMs: ms }),

  zoomBy: (factor, durationMs, visibleWidth) => {
    const cur = get();
    const newZoom = Math.max(1, Math.min(cur.zoom * factor, 50));
    const playhead = cur.playheadMs;
    const fit = durationMs > 0 ? visibleWidth / durationMs : 0;
    const newPxPerMs = fit * newZoom;
    const targetScroll = playhead - visibleWidth / 2 / (newPxPerMs || 1);
    set({ zoom: newZoom, scrollMs: Math.max(0, targetScroll) });
  },
  zoomFit: () => set({ zoom: 1, scrollMs: 0 }),

  selectSegment: (id) => set({ selectedSegmentId: id }),

  reset: () =>
    set({
      projectID: null,
      transcript: null,
      cutList: null,
      peaks: null,
      mediaURL: null,
      loading: false,
      error: "",
      stage: "idle",
      progress: 0,
      stageStep: "",
      playheadMs: 0,
      isPlaying: false,
      zoom: 1,
      scrollMs: 0,
      selectedSegmentId: null,
      subtitleEnabled: false,
      subtitleStyle: null,
      subtitleConfig: null,
    }),
}));

import { Check, X, RefreshCw } from "lucide-react";
import type { CutList, CutSegment } from "../api/types";
import { Button } from "./ui/button";

interface Props {
  cutList: CutList | null;
  selectedSegmentId: string | null;
  onSelect: (id: string) => void;
  onAccept: (segID: string) => void;
  onReject: (segID: string) => void;
  loading: boolean;
}

const reasonLabel: Record<string, string> = {
  filler: "语气词",
  silence: "停顿/沉默",
  dup_or_error: "重复/口误",
  manual: "手动",
};

export function AISuggestions({
  cutList,
  selectedSegmentId,
  onSelect,
  onAccept,
  onReject,
  loading,
}: Props) {
  const removeSegs: CutSegment[] =
    cutList?.segments.filter((s) => s.decision === "remove" && s.source === "ai") ?? [];

  return (
    <aside className="flex w-64 flex-col border-l border-border bg-zinc-900">
      <div className="flex items-center justify-between border-b border-border px-3 py-2">
        <span className="text-sm font-medium">AI 建议</span>
        {loading && <RefreshCw className="h-3.5 w-3.5 animate-spin text-muted-foreground" />}
      </div>

      <div className="flex-1 overflow-auto">
        {removeSegs.length === 0 && (
          <p className="px-3 py-4 text-xs text-muted-foreground">
            暂无 AI 建议的删除段。完成分析后将在此显示。
          </p>
        )}
        {removeSegs.map((seg) => {
          const isSelected = seg.id === selectedSegmentId;
          const dur = ((seg.endMs - seg.startMs) / 1000).toFixed(2);
          return (
            <div
              key={seg.id}
              className={`cursor-pointer border-b border-border px-3 py-2 text-xs hover:bg-zinc-800 ${
                isSelected ? "bg-zinc-800 ring-1 ring-yellow-400" : ""
              }`}
              onClick={() => onSelect(seg.id)}
            >
              <div className="flex items-center justify-between">
                <span className="rounded bg-red-900/60 px-1.5 py-0.5 text-[10px] text-red-200">
                  {reasonLabel[seg.reason] ?? seg.reason} · {dur}s
                </span>
                <span className="text-muted-foreground">
                  {(seg.confidence * 100).toFixed(0)}%
                </span>
              </div>
              {seg.note && <p className="mt-1 line-clamp-2 text-muted-foreground">{seg.note}</p>}
              <div className="mt-1.5 flex gap-1">
                <Button size="sm" variant="outline" onClick={(e) => { e.stopPropagation(); onAccept(seg.id); }}>
                  <Check className="mr-1 h-3 w-3" /> 保留此段
                </Button>
                <Button size="sm" variant="ghost" onClick={(e) => { e.stopPropagation(); onReject(seg.id); }} title="确认删除">
                  <X className="h-3 w-3" />
                </Button>
              </div>
            </div>
          );
        })}
      </div>
    </aside>
  );
}

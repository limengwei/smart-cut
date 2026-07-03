import { useState } from "react";
import { ChevronDown, ChevronRight } from "lucide-react";
import { Button } from "./ui/button";
import { Input } from "./ui/input";
import { Label } from "./ui/label";
import { Switch } from "./ui/switch";
import type { SubtitleStyle } from "../api/types";

interface Props {
  enabled: boolean;
  style: SubtitleStyle;
  onToggleEnabled: (b: boolean) => void;
  onChangeStyle: (s: SubtitleStyle) => void;
}

const POSITION_OPTIONS = ["bottom", "center", "top"] as const;

export function SubtitleStylePanel({ enabled, style, onToggleEnabled, onChangeStyle }: Props) {
  const [expanded, setExpanded] = useState(false);

  const update = (patch: Partial<SubtitleStyle>) => onChangeStyle({ ...style, ...patch });

  return (
    <div className="border-b border-border bg-zinc-900">
      <button
        className="flex w-full items-center justify-between px-3 py-2 text-left text-sm font-medium"
        onClick={() => setExpanded((v) => !v)}
      >
        <span className="flex items-center gap-2">
          {expanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
          字幕样式
        </span>
        <Switch
          checked={enabled}
          onCheckedChange={onToggleEnabled}
          onClick={(e) => e.stopPropagation()}
        />
      </button>

      {expanded && (
        <div className="space-y-3 px-3 pb-3">
          <div className="grid grid-cols-2 gap-2">
            <div>
              <Label className="text-xs">字体</Label>
              <Input value={style.fontFamily} onChange={(e) => update({ fontFamily: e.target.value })} className="h-8 text-xs" />
            </div>
            <div>
              <Label className="text-xs">字号</Label>
              <Input
                type="number"
                value={style.fontSize}
                onChange={(e) => update({ fontSize: Number(e.target.value) })}
                className="h-8 text-xs"
              />
            </div>
          </div>
          <div className="grid grid-cols-2 gap-2">
            <div>
              <Label className="text-xs">字幕色</Label>
              <Input type="color" value={style.color} onChange={(e) => update({ color: e.target.value })} className="h-8 p-1" />
            </div>
            <div>
              <Label className="text-xs">高亮色</Label>
              <Input type="color" value={style.highlight} onChange={(e) => update({ highlight: e.target.value })} className="h-8 p-1" />
            </div>
          </div>
          <div className="grid grid-cols-2 gap-2">
            <div>
              <Label className="text-xs">背景色</Label>
              <Input type="color" value={style.bgColor} onChange={(e) => update({ bgColor: e.target.value })} className="h-8 p-1" />
            </div>
            <div>
              <Label className="text-xs">背景透明度</Label>
              <Input
                type="number"
                min={0}
                max={1}
                step={0.1}
                value={style.bgOpacity}
                onChange={(e) => update({ bgOpacity: Number(e.target.value) })}
                className="h-8 text-xs"
              />
            </div>
          </div>
          <div>
            <Label className="text-xs">位置</Label>
            <div className="flex gap-1">
              {POSITION_OPTIONS.map((pos) => (
                <Button
                  key={pos}
                  size="sm"
                  variant={style.position === pos ? "default" : "outline"}
                  onClick={() => update({ position: pos })}
                  className="h-7 text-xs"
                >
                  {pos}
                </Button>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

import { ZoomIn, ZoomOut, Maximize2 } from "lucide-react";
import { Button } from "../ui/button";

interface Props {
  zoom: number;
  onZoomIn: () => void;
  onZoomOut: () => void;
  onFit: () => void;
}

export function ZoomControls({ zoom, onZoomIn, onZoomOut, onFit }: Props) {
  return (
    <div className="flex items-center gap-1">
      <Button variant="ghost" size="icon" onClick={onZoomOut} title="缩小">
        <ZoomOut className="h-4 w-4" />
      </Button>
      <span className="w-12 text-center text-xs text-muted-foreground">
        {zoom.toFixed(1)}x
      </span>
      <Button variant="ghost" size="icon" onClick={onZoomIn} title="放大">
        <ZoomIn className="h-4 w-4" />
      </Button>
      <Button variant="ghost" size="icon" onClick={onFit} title="适应窗口">
        <Maximize2 className="h-4 w-4" />
      </Button>
    </div>
  );
}

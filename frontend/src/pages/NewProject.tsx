import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/card";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { Label } from "../components/ui/label";
import { createProject, probeMedia, pickFile } from "../api/client";
import { useProjectStore } from "../stores/project";
import type { MediaFile } from "../api/types";

export function NewProject() {
  const navigate = useNavigate();
  const setCurrentProject = useProjectStore((s) => s.setCurrentProject);

  const [name, setName] = useState("");
  const [mediaPath, setMediaPath] = useState("");
  const [mediaInfo, setMediaInfo] = useState<MediaFile | null>(null);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState("");

  const handleProbe = async () => {
    if (!mediaPath) return;
    try {
      const info = await probeMedia(mediaPath);
      setMediaInfo(info);
      setError("");
    } catch (e) {
      setError("媒体文件探测失败: " + String(e));
      setMediaInfo(null);
    }
  };

  const handlePickFile = async () => {
    try {
      const path = await pickFile("选择视频文件", [
        { displayName: "视频文件", pattern: "*.mp4;*.mov;*.avi;*.mkv;*.webm;*.flv" },
      ]);
      if (path) {
        setMediaPath(path);
        const info = await probeMedia(path);
        setMediaInfo(info);
        setError("");
      }
    } catch (e) {
      setError("选择文件失败: " + String(e));
    }
  };

  const handleCreate = async () => {
    if (!name || !mediaPath) {
      setError("请填写项目名称和媒体文件路径");
      return;
    }

    setCreating(true);
    setError("");
    try {
      const project = await createProject(name, mediaPath);
      setCurrentProject(project);
      navigate(`/project/${project.id}`);
    } catch (e) {
      setError("创建项目失败: " + String(e));
    } finally {
      setCreating(false);
    }
  };

  return (
    <div className="mx-auto max-w-2xl space-y-6 p-8">
      <div>
        <h1 className="text-2xl font-bold">新建项目</h1>
        <p className="mt-1 text-sm text-muted-foreground">导入视频文件，开始 AI 自动剪辑</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>项目信息</CardTitle>
          <CardDescription>设置项目名称并选择要剪辑的视频文件</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="project-name">项目名称</Label>
            <Input
              id="project-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="我的口播视频"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="media-path">视频文件路径</Label>
            <div className="flex gap-2">
              <Input
                id="media-path"
                value={mediaPath}
                onChange={(e) => setMediaPath(e.target.value)}
                placeholder="/path/to/video.mp4"
              />
              <Button variant="outline" size="sm" onClick={handlePickFile}>
                选择文件
              </Button>
              <Button variant="outline" size="sm" onClick={handleProbe} disabled={!mediaPath}>
                探测
              </Button>
            </div>
          </div>

          {mediaInfo && (
            <div className="rounded-lg border border-border bg-muted/30 p-4 text-sm">
              <div className="grid grid-cols-2 gap-2">
                <span className="text-muted-foreground">分辨率:</span>
                <span>{mediaInfo.width}×{mediaInfo.height}</span>
                <span className="text-muted-foreground">帧率:</span>
                <span>{mediaInfo.fps.toFixed(2)} fps</span>
                <span className="text-muted-foreground">时长:</span>
                <span>{(mediaInfo.durationMs / 1000).toFixed(1)} 秒</span>
                <span className="text-muted-foreground">音频:</span>
                <span>{mediaInfo.hasAudio ? "有" : "无"}</span>
              </div>
            </div>
          )}

          {error && <p className="text-sm text-destructive">{error}</p>}

          <Button onClick={handleCreate} disabled={creating || !name || !mediaPath}>
            {creating ? "创建中..." : "创建项目"}
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
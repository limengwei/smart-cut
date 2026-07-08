import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Card, CardContent } from "../components/ui/card";
import { Button } from "../components/ui/button";
import { listProjects } from "../api/client";
import type { Project } from "../api/types";
import { FolderPlus, Film, Clock, Calendar } from "lucide-react";

const statusLabels: Record<string, string> = {
  draft: "草稿",
  transcribed: "已转录",
  analyzed: "已分析",
  exported: "已导出",
};

const statusColors: Record<string, string> = {
  draft: "bg-zinc-600 text-zinc-200",
  transcribed: "bg-blue-600 text-blue-100",
  analyzed: "bg-amber-600 text-amber-100",
  exported: "bg-emerald-600 text-emerald-100",
};

function formatDuration(ms: number): string {
  const totalSeconds = Math.floor(ms / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${seconds.toString().padStart(2, "0")}`;
}

function formatDate(dateStr: string): string {
  const d = new Date(dateStr);
  return d.toLocaleDateString("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function Home() {
  const navigate = useNavigate();
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    listProjects()
      .then(setProjects)
      .catch(() => setProjects([]))
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <p className="text-sm text-muted-foreground">加载中...</p>
      </div>
    );
  }

  if (projects.length === 0) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-4">
        <FolderPlus className="h-12 w-12 text-muted-foreground" />
        <p className="text-sm text-muted-foreground">暂无项目</p>
        <Button onClick={() => navigate("/project/new")}>创建第一个项目</Button>
      </div>
    );
  }

  return (
    <div className="space-y-6 p-8">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">我的项目</h1>
          <p className="mt-1 text-sm text-muted-foreground">共 {projects.length} 个项目</p>
        </div>
        <Button onClick={() => navigate("/project/new")}>
          <FolderPlus className="mr-2 h-4 w-4" /> 新建项目
        </Button>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {projects.map((project) => (
          <Card
            key={project.id}
            className="cursor-pointer transition-colors hover:border-primary/50 hover:bg-accent/50"
            onClick={() => navigate(`/project/${project.id}`)}
          >
            <CardContent className="p-4">
              <div className="flex items-start justify-between gap-2">
                <h3 className="truncate font-semibold">{project.name}</h3>
                <span className={`shrink-0 rounded-full px-2 py-0.5 text-xs ${statusColors[project.status] ?? statusColors.draft}`}>
                  {statusLabels[project.status] ?? project.status}
                </span>
              </div>

              <div className="mt-3 space-y-1.5 text-xs text-muted-foreground">
                <div className="flex items-center gap-1.5">
                  <Film className="h-3.5 w-3.5" />
                  <span className="truncate">{project.media.path}</span>
                </div>
                {project.media.durationMs > 0 && (
                  <div className="flex items-center gap-1.5">
                    <Clock className="h-3.5 w-3.5" />
                    <span>{formatDuration(project.media.durationMs)}</span>
                  </div>
                )}
                <div className="flex items-center gap-1.5">
                  <Calendar className="h-3.5 w-3.5" />
                  <span>{formatDate(project.updatedAt)}</span>
                </div>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  );
}
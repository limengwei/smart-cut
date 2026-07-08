import { useEffect, useState } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/card";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { Label } from "../components/ui/label";
import { Switch } from "../components/ui/switch";
import { useSettingsStore } from "../stores/settings";
import { probeBinary, pickFile, pickDirectory } from "../api/client";
import type { GlobalSettings } from "../api/types";

export function Settings() {
  const { settings, loadSettings, updateSettings } = useSettingsStore();
  const [form, setForm] = useState<GlobalSettings | null>(null);
  const [saving, setSaving] = useState(false);
  const [probeResults, setProbeResults] = useState<Record<string, { path: string; version: string } | null>>({});

  useEffect(() => {
    loadSettings();
  }, [loadSettings]);

  useEffect(() => {
    if (settings) setForm(settings);
  }, [settings]);

  if (!form) return <div className="p-8">加载中...</div>;

  const handleProbe = async (name: string) => {
    try {
      const result = await probeBinary(name);
      setProbeResults((prev) => ({ ...prev, [name]: result }));
    } catch (e) {
      setProbeResults((prev) => ({ ...prev, [name]: null }));
      console.error(`${name} 探测失败:`, e);
    }
  };

  const handleSave = async () => {
    if (!form) return;
    setSaving(true);
    try {
      await updateSettings(form);
    } finally {
      setSaving(false);
    }
  };

  const binaries = ["ffmpeg", "ffprobe", "whisper-cli"];

  return (
    <div className="mx-auto max-w-2xl space-y-6 p-8">
      <h1 className="text-2xl font-bold">设置</h1>

      <Card>
        <CardHeader>
          <CardTitle>二进制路径</CardTitle>
          <CardDescription>配置外部工具路径，留空则使用随包或系统 PATH</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {binaries.map((name) => (
            <div key={name} className="space-y-2">
              <Label htmlFor={`bin-${name}`}>{name}</Label>
              <div className="flex gap-2">
                <Input
                  id={`bin-${name}`}
                  value={form.binaries[name] || ""}
                  onChange={(e) =>
                    setForm({
                      ...form,
                      binaries: { ...form.binaries, [name]: e.target.value },
                    })
                  }
                  placeholder={`留空使用系统 PATH`}
                />
                <Button variant="outline" size="sm" onClick={async () => {
                  try {
                    const path = await pickFile("选择二进制文件", []);
                    if (path) setForm({ ...form, binaries: { ...form.binaries, [name]: path } });
                  } catch (e) { console.error("选择文件失败:", e); }
                }}>浏览</Button>
                <Button variant="outline" size="sm" onClick={() => handleProbe(name)}>
                  探测
                </Button>
              </div>
              {probeResults[name] && (
                <p className="text-xs text-muted-foreground">
                  ✓ {probeResults[name]!.version}
                </p>
              )}
              {probeResults[name] === null && (
                <p className="text-xs text-destructive">✗ 未找到</p>
              )}
            </div>
          ))}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Whisper 模型</CardTitle>
          <CardDescription>ggml 模型文件所在目录</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-2">
            <Label htmlFor="whisper-model">模型目录</Label>
            <div className="flex gap-2">
            <Input
              id="whisper-model"
              value={form.whisperModelDir}
              onChange={(e) => setForm({ ...form, whisperModelDir: e.target.value })}
              placeholder="/path/to/models"
            />
            <Button variant="outline" size="sm" onClick={async () => {
              try {
                const path = await pickDirectory("选择模型目录");
                if (path) setForm({ ...form, whisperModelDir: path });
              } catch (e) { console.error("选择目录失败:", e); }
            }}>浏览</Button>
          </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>LLM 配置</CardTitle>
          <CardDescription>OpenAI 兼容 API（用于语气词/重复检测）</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="llm-baseurl">API Base URL</Label>
            <Input
              id="llm-baseurl"
              value={form.defaultLLM.baseUrl}
              onChange={(e) =>
                setForm({
                  ...form,
                  defaultLLM: { ...form.defaultLLM, baseUrl: e.target.value },
                })
              }
              placeholder="https://api.openai.com/v1"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="llm-apikey">API Key</Label>
            <Input
              id="llm-apikey"
              type="password"
              value={form.defaultLLM.apiKey}
              onChange={(e) =>
                setForm({
                  ...form,
                  defaultLLM: { ...form.defaultLLM, apiKey: e.target.value },
                })
              }
              placeholder="sk-..."
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="llm-model">模型</Label>
            <Input
              id="llm-model"
              value={form.defaultLLM.model}
              onChange={(e) =>
                setForm({
                  ...form,
                  defaultLLM: { ...form.defaultLLM, model: e.target.value },
                })
              }
              placeholder="gpt-4o-mini"
            />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>外观</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-between">
            <Label htmlFor="theme-switch">暗色模式</Label>
            <Switch
              id="theme-switch"
              checked={form.theme === "dark"}
              onCheckedChange={(checked) =>
                setForm({ ...form, theme: checked ? "dark" : "light" })
              }
            />
          </div>
        </CardContent>
      </Card>

      <Button onClick={handleSave} disabled={saving}>
        {saving ? "保存中..." : "保存设置"}
      </Button>
    </div>
  );
}
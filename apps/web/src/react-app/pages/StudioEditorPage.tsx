import { useEffect, useRef, useState } from "react";
import { useWorkspace } from "../page";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Disclosure } from "../controls";
import { ActionFeedback } from "../components";
import { worldDefinitionError } from "@/v3/proto/components/world-definition.js";
import { errorMessage } from "@/v3/domain-presentation.mjs";
const labels: Record<string, string> = {
  background: "世界背景",
  description: "描述",
  flavor: "风格",
  rules: "规则",
  atmosphere: "氛围",
  floor: "楼层",
  layout: "布局",
  occupation: "职业",
  biography: "人物背景",
};
const ordinaryFields: Record<string, string[]> = {
  world: ["background", "description", "flavor", "rules"],
  place: ["description", "atmosphere", "floor", "layout"],
  npc: ["description", "occupation", "biography"],
  item: ["description"],
  outfit: ["description"],
};
export default function StudioEditorPage({
  resourceId,
  kind = "world",
}: {
  resourceId: string;
  kind?: string;
}) {
  const w = useWorkspace(),
    [resource, setResource] = useState<any>(null),
    [name, setName] = useState(""),
    [body, setBody] = useState<Record<string, any>>({}),
    [advanced, setAdvanced] = useState("{}"),
    [advancedDirty, setAdvancedDirty] = useState(false),
    [busy, setBusy] = useState(false),
    [loading, setLoading] = useState(resourceId !== "new"),
    [error, setError] = useState("");
  const draft = useRef({ name, body, advanced, advancedDirty });
  draft.current = { name, body, advanced, advancedDirty };
  const actualKind = resource?.kind || kind,
    actualId = resource?.id || (resourceId === "new" ? null : resourceId);
  useEffect(() => {
    let active = true;
    if (resourceId === "new") {
      setLoading(false);
      return;
    }
    setLoading(true);
    void w.services.studioClient
      .resource(resourceId)
      .then((value: any) => {
        if (!active) return;
        setResource(value);
        setName(value.name || "");
        setBody(value.body || {});
        setAdvanced(JSON.stringify(value.body || {}, null, 2));
        setAdvancedDirty(false);
      })
      .catch((cause: unknown) => {
        if (active) setError(errorMessage(cause));
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [w, resourceId]);
  async function save() {
    if (busy) return;
    const submitted = draft.current;
    setError("");
    let payload: Record<string, unknown>;
    try {
      payload = submitted.advancedDirty
        ? JSON.parse(submitted.advanced)
        : submitted.body;
      if (!payload || typeof payload !== "object" || Array.isArray(payload))
        throw Error("资源内容需要是一个对象。");
      if (!submitted.name.trim()) throw Error("请填写资源名称。");
      if (actualKind === "world") {
        const invalid = worldDefinitionError(payload);
        if (invalid) throw Error(invalid);
      }
    } catch (cause) {
      setError(errorMessage(cause));
      return;
    }
    setBusy(true);
    const command = w.command(
      "创作资源",
      "studio-save:" + String(actualId || actualKind),
      {
        kind: actualKind,
        name: submitted.name.trim(),
        body: payload,
        ...(resource ? { expectedVersion: resource.draft_version } : {}),
        reason: actualId ? "更新创作资源" : "创建创作资源",
      },
      (value: any, key: string) =>
        w.services.studioClient.save(actualId, {
          ...value,
          idempotencyKey: key,
        }),
      { success: "资源草稿已保存，发布后才能作为已发布底稿使用" },
    );
    try {
      const saved = await command.completion;
      setResource(saved);
      if (JSON.stringify(draft.current) === JSON.stringify(submitted)) {
        setName(saved.name);
        setBody(saved.body || payload);
        setAdvanced(JSON.stringify(saved.body || payload, null, 2));
        setAdvancedDirty(false);
      }
    } catch (cause) {
      setError(errorMessage(cause));
    } finally {
      setBusy(false);
    }
  }
  async function publish() {
    if (!resource || busy) return;
    setBusy(true);
    setError("");
    const command = w.command(
      "创作资源",
      "studio-publish:" + resource.id,
      { expectedRevision: resource.revision, reason: "发布已保存的资源定义" },
      (value: any, key: string) =>
        w.client.publishStudioResource(resource.id, {
          ...value,
          idempotencyKey: key,
        }),
      { success: "已发布保存的资源定义" },
    );
    try {
      await command.completion;
      setResource(await w.services.studioClient.resource(resource.id));
    } catch (cause) {
      setError(errorMessage(cause));
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="grid gap-5">
      <Button variant="outline" className="w-fit" asChild>
        <a href="#/studio">返回创作中心</a>
      </Button>
      <Card>
        <CardHeader>
          <CardTitle>{actualId ? "编辑资源" : "创建资源"}</CardTitle>
        </CardHeader>
        <CardContent>
          {loading ? (
            <p role="status">正在读取资源…</p>
          ) : (
            <form
              className="grid gap-5"
              onSubmit={(e) => {
                e.preventDefault();
                void save();
              }}
            >
              <div className="grid gap-2">
                <Label htmlFor="resource-name">名称</Label>
                <Input
                  id="resource-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  disabled={busy}
                />
              </div>
              {(ordinaryFields[actualKind] || []).map((field) => (
                <div className="grid gap-2" key={field}>
                  <Label htmlFor={"resource-" + field}>
                    {labels[field] || field}
                  </Label>
                  <Textarea
                    id={"resource-" + field}
                    value={typeof body[field] === "string" ? body[field] : ""}
                    onChange={(e) => {
                      const value = { ...body, [field]: e.target.value };
                      setBody(value);
                      if (!advancedDirty)
                        setAdvanced(JSON.stringify(value, null, 2));
                    }}
                    disabled={busy || advancedDirty}
                  />
                </div>
              ))}
              <Disclosure title="完整资源字段（高级）">
                <Label htmlFor="resource-json">
                  保留已有结构，仅修改需要调整的字段
                </Label>
                <Textarea
                  id="resource-json"
                  className="min-h-60 font-mono"
                  value={advanced}
                  onChange={(e) => {
                    setAdvanced(e.target.value);
                    setAdvancedDirty(true);
                  }}
                  disabled={busy}
                />
              </Disclosure>
              {error && (
                <p role="alert" className="text-destructive">
                  {error}
                </p>
              )}
              <ActionFeedback labels={["创作资源"]} />
              <div className="flex flex-wrap gap-3">
                <Button type="submit" disabled={busy}>
                  {busy ? "正在处理…" : "保存草稿"}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  disabled={!resource || busy}
                  onClick={() => void publish()}
                >
                  发布已保存版本
                </Button>
                {resource && (
                  <Button variant="link" asChild>
                    <a
                      href={"#/studio/edit/" + encodeURIComponent(resource.id)}
                    >
                      打开已保存资源
                    </a>
                  </Button>
                )}
              </div>
            </form>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

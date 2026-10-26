import { useEffect, useState } from "react";
import { useWorkspace } from "../page";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import {
  Accordion,
  AccordionItem,
  AccordionTrigger,
  AccordionContent,
} from "@/components/ui/accordion";
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
} from "@/components/ui/alert-dialog";
import SemanticTree from "../shared/SemanticTree";
import { ActionFeedback } from "../components";
import { Disclosure } from "../controls";
import { errorMessage, stateLabel } from "@/v3/domain-presentation.mjs";
export default function CharacterToolsPage({
  fid,
  tool,
  tab = "foundation",
}: {
  fid: string;
  tool: string;
  tab?: string;
}) {
  const w = useWorkspace(),
    [data, setData] = useState<any>(null),
    [assets, setAssets] = useState<any[]>([]),
    [baseline, setBaseline] = useState<number | null>(null),
    [error, setError] = useState(""),
    [loading, setLoading] = useState(true),
    [busy, setBusy] = useState(false),
    [revision, refresh] = useState(0),
    [reason, setReason] = useState(""),
    [decision, setDecision] = useState<any>(null);
  const scope = w.currentScope();
  useEffect(() => {
    let active = true;
    setLoading(true);
    setError("");
    setData(null);
    async function read() {
      if (tool === "avatar") {
        const [avatar, library] = await Promise.all([
          w.client.displayAvatar(fid),
          w.client.ownerMediaLibrary({
            scope: "fluctlight",
            fluctlightId: fid,
            kind: "image",
            hidden: false,
          }),
        ]);
        if (active) {
          setData(avatar);
          setAssets(library.items);
        }
      } else if (tool === "identity") {
        const value = await w.client.visualIdentityManagement(fid);
        if (active) setData(value);
      } else if (tab === "developing") {
        const value = await w.client.developingSelf(fid);
        if (active) setData(value);
      } else {
        const [page, current] = await Promise.all([
          w.client.governancePage(
            fid,
            tab === "capabilities" ? "capability" : "foundation",
            { limit: 50 },
          ),
          w.client.detail(fid),
        ]);
        if (active) {
          setData(page);
          setBaseline(current.current_revision);
        }
      }
    }
    void read()
      .catch((cause) => {
        if (active) setError(errorMessage(cause));
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [w, fid, tool, tab, revision]);
  async function chooseAvatar(asset: any) {
    if (busy || !Number.isSafeInteger(data?.revision)) return;
    setBusy(true);
    const body = {
      mode: asset ? "asset" : "canonical",
      ...(asset
        ? {
            assetId: asset.asset_id || asset.id,
            expectedAssetRevision: asset.asset_revision,
          }
        : {}),
      expectedRevision: data.revision,
      reason: reason.trim() || "所有者更新显示头像",
    };
    const command = w.command(
      "角色工具",
      "display-avatar:" + fid,
      body,
      (value: any, key: string) =>
        w.client.selectDisplayAvatar(fid, { ...value, idempotencyKey: key }),
      {
        fid,
        success: "显示头像已保存",
        onSaved: async () => {
          if (w.currentScope() === scope) {
            refresh((x) => x + 1);
            await w.refreshDirectory();
          }
        },
      },
    );
    try {
      await command.completion;
    } catch (cause) {
      if (w.currentScope() === scope) setError(errorMessage(cause));
    } finally {
      if (w.currentScope() === scope) setBusy(false);
    }
  }
  async function submitDecision() {
    if (!decision || busy || !reason.trim()) return;
    setBusy(true);
    setError("");
    const captured = decision,
      body =
        tab === "capabilities"
          ? {
              status: captured.action,
              note: reason.trim(),
              expectedRevision: captured.row.revision,
            }
          : { expectedRevision: baseline, reason: reason.trim() };
    const command = w.command(
      "角色工具",
      "review:" + captured.row.id + ":" + captured.action,
      body,
      (value: any, key: string) =>
        tab === "capabilities"
          ? w.client.reviewScopedCapabilityRequest(fid, captured.row.id, {
              ...value,
              idempotencyKey: key,
            })
          : captured.action === "accepted"
            ? w.client.acceptFoundationRevision(fid, captured.row.id, value)
            : w.client.rejectFoundationRevision(fid, captured.row.id, value),
      {
        fid,
        success: "决定已确认",
        onSaved: () => {
          if (w.currentScope() === scope) refresh((x) => x + 1);
        },
      },
    );
    try {
      await command.completion;
      if (w.currentScope() === scope) setDecision(null);
    } catch (cause) {
      if (w.currentScope() === scope) setError(errorMessage(cause));
    } finally {
      if (w.currentScope() === scope) setBusy(false);
    }
  }
  const rows = Array.isArray(data?.items) ? data.items : [];
  return (
    <div className="grid gap-5">
      <div className="flex gap-3">
        <Button variant="outline" asChild>
          <a href={"#/c/" + encodeURIComponent(fid) + "/profile"}>
            返回角色详情
          </a>
        </Button>
        <Button
          variant="outline"
          disabled={loading || busy}
          onClick={() => refresh((x) => x + 1)}
        >
          重新读取
        </Button>
      </div>
      {loading && <p role="status">正在读取当前记录…</p>}
      {error && !decision && (
        <p role="alert" className="text-destructive">
          {error}
        </p>
      )}
      <ActionFeedback labels={["角色工具"]} />
      {tool === "avatar" && data && (
        <Card>
          <CardHeader>
            <CardTitle>显示头像</CardTitle>
          </CardHeader>
          <CardContent className="grid gap-4">
            {data.avatar?.url && (
              <img
                className="size-24 rounded-full object-cover"
                src={data.avatar.url}
                alt="当前显示头像"
              />
            )}
            <Label htmlFor="avatar-reason">修改说明</Label>
            <Textarea
              id="avatar-reason"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              placeholder="可说明这次更换的原因"
            />
            <Button
              variant="outline"
              disabled={busy}
              onClick={() => void chooseAvatar(null)}
            >
              使用已确认的标准形象
            </Button>
            <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
              {assets
                .filter((asset) => asset.readable)
                .map((asset) => (
                  <Button
                    variant="outline"
                    key={asset.id}
                    className="h-auto flex-col p-2"
                    disabled={busy}
                    onClick={() => void chooseAvatar(asset)}
                  >
                    <img
                      src={asset.url || w.services.mediaClient.url(asset.id)}
                      alt={asset.title || "可选头像"}
                      className="aspect-square w-full rounded object-cover"
                    />
                    <span>{asset.title || "未命名图片"}</span>
                  </Button>
                ))}
            </div>
            {!assets.length && <p>当前没有可供选择的图片。</p>}
          </CardContent>
        </Card>
      )}
      {tool === "identity" && data && (
        <Card>
          <CardHeader>
            <CardTitle>已记录的视觉身份</CardTitle>
            <Badge variant="secondary">
              {stateLabel(data.status, "状态待确认")}
            </Badge>
          </CardHeader>
          <CardContent className="grid gap-4">
            {["canonical_asset_id", "character_sheet_asset_id"].map(
              (key) =>
                data[key] && (
                  <img
                    key={key}
                    src={w.services.mediaClient.url(data[key])}
                    alt={
                      key === "canonical_asset_id"
                        ? "已确认参考图"
                        : "角色设定图"
                    }
                    className="max-h-96 max-w-full object-contain"
                  />
                ),
            )}
            <SemanticTree value={data} />
            <Disclosure title="完整原始记录">
              <pre className="approved-json">
                {JSON.stringify(data, null, 2)}
              </pre>
            </Disclosure>
          </CardContent>
        </Card>
      )}
      {tool === "governance" && tab === "developing" && data && (
        <Card>
          <CardHeader>
            <CardTitle>发展中的自我</CardTitle>
          </CardHeader>
          <CardContent>
            <SemanticTree value={data} />
          </CardContent>
        </Card>
      )}
      {tool === "governance" && tab !== "developing" && (
        <>
          <h2>{tab === "capabilities" ? "能力申请" : "身份核心提议"}</h2>
          {!loading && !rows.length && !error && <p>当前没有对应记录。</p>}
          <Accordion type="multiple">
            {rows.map((row: any) => (
              <AccordionItem key={row.id} value={row.id}>
                <AccordionTrigger>
                  {row.title || row.reason || "资料提议"} ·{" "}
                  {stateLabel(row.status, "状态待确认")}
                </AccordionTrigger>
                <AccordionContent>
                  <SemanticTree value={row.description || row.changes || row} />
                  {row.rationale && <p>{row.rationale}</p>}
                  <Disclosure title="完整提议与来源">
                    <pre className="approved-json">
                      {JSON.stringify(row, null, 2)}
                    </pre>
                  </Disclosure>
                  {["proposed", "reviewing"].includes(row.status) && (
                    <div className="flex gap-3">
                      <Button
                        disabled={
                          busy ||
                          (!Number.isSafeInteger(baseline) &&
                            tab !== "capabilities")
                        }
                        onClick={() => {
                          setReason("");
                          setDecision({ row, action: "accepted" });
                        }}
                      >
                        接受
                      </Button>
                      <Button
                        variant="outline"
                        disabled={busy}
                        onClick={() => {
                          setReason("");
                          setDecision({ row, action: "rejected" });
                        }}
                      >
                        拒绝
                      </Button>
                    </div>
                  )}
                </AccordionContent>
              </AccordionItem>
            ))}
          </Accordion>
        </>
      )}
      <AlertDialog
        open={!!decision}
        onOpenChange={(open) => {
          if (!open && !busy) setDecision(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              确认{decision?.action === "accepted" ? "接受" : "拒绝"}这条提议
            </AlertDialogTitle>
            <AlertDialogDescription>
              决定将提交到当前角色的原记录，版本变化时会要求重新核对。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <Label htmlFor="decision-reason">决定原因</Label>
          <Textarea
            id="decision-reason"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            disabled={busy}
          />
          {error && (
            <p role="alert" className="text-destructive">
              {error}
            </p>
          )}
          <AlertDialogFooter>
            <AlertDialogCancel disabled={busy}>取消</AlertDialogCancel>
            <Button
              disabled={busy || !reason.trim()}
              onClick={() => void submitDecision()}
            >
              {busy ? "正在提交…" : "确认提交"}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

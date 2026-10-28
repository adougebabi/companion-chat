import { useEffect, useId, useMemo, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Label, FieldInput, ChoiceSelect, Disclosure } from "../controls";
import { useWorkspace } from "../page";
import { useCursorPage } from "../hooks/use-cursor-page";
import CursorControls from "./CursorControls";
import WorkspaceDialog from "./WorkspaceDialog";
import SemanticTree from "./SemanticTree";
import JsonDetails from "./JsonDetails";
import { studioVersion } from "@/approved-ui/studio-presentation.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";
import { persistentCommand } from "@/v3/providers/persistent-command.mjs";
import { studioDraftStorage } from "@/v3/proto/components/studio-draft-storage.js";
import { confirm } from "@/v3/proto/components/overlay.js";
import { errorMessage } from "@/v3/domain-presentation.mjs";
export default function StudioLifecycle({
  resource,
  onChanged,
  historyOpen,
  onHistoryOpenChange,
  historyReturnFocus,
}: any) {
  const formId = useId(),
    [copyOpen, setCopyOpen] = useState(false),
    [localHistoryOpen, setLocalHistoryOpen] = useState(false);
  const copyTrigger = useRef<HTMLElement | null>(null);
  const w = useWorkspace(),
    owner = w.state.session.actorId,
    service = w.services.studioClient,
    slot = `fluctlight-studio:${owner}:${resource.id}`,
    active = useRef(false),
    epoch = useRef(0),
    publicationEpoch = useRef(0),
    latest = useRef(resource);
  latest.current = resource;
  const [draft, setDraft] = useState<any>(() => {
      try {
        return (
          JSON.parse(
            studioDraftStorage().getItem(slot + ":lifecycle-draft") || "null",
          ) || {
            name: resource.name + " 副本",
            source: resource.draft_version
              ? "draft:" + resource.draft_version
              : "published:" + resource.published_version,
            reason: "",
          }
        );
      } catch {
        return null;
      }
    }),
    [pending, setPending] = useState<any>(null),
    [receipt, setReceipt] = useState<any>(null),
    [copy, setCopy] = useState<any>(null),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [publication, setPublication] = useState<any>(null);
  const live = () =>
    active.current &&
    latest.current.id === resource.id &&
    w.state.session?.actorId === owner;
  const command = useMemo(
    () =>
      persistentCommand({
        key: slot + ":lifecycle",
        storage: studioDraftStorage,
        execute: async (value: any, key: string) => {
          if (!live()) throw Error("会话或资源范围已切换");
          if (!["copy", "archive", "restore"].includes(value.operation))
            throw Error("未知资源管理操作");
          const result = await service[value.operation](resource.id, {
            ...value.body,
            idempotencyKey: key,
          });
          const current = studioDraftStorage().getItem(slot + ":lifecycle");
          if (
            w.state.session?.actorId === owner &&
            current &&
            JSON.parse(current).idempotencyKey === key
          ) {
            if (typeof result.id !== "string" || !result.id)
              throw Error("管理操作回执缺少资源标识，请核对原提交");
            studioDraftStorage().setItem(
              slot + ":lifecycle-receipt",
              JSON.stringify({
                sourceId: resource.id,
                targetId: result.id,
                operation: value.operation,
              }),
            );
          }
          return result;
        },
      }),
    [slot, service],
  );
  const versions = useCursorPage(
    (cursor) => service.versions(resource.id, { cursor }),
    JSON.stringify([owner, resource.id, "publications"]),
  );
  async function readReceipt() {
    const raw = studioDraftStorage().getItem(slot + ":lifecycle-receipt");
    if (!raw || !live()) return;
    const token = ++epoch.current;
    try {
      const pointer = JSON.parse(raw);
      setReceipt(pointer);
      if (
        pointer.sourceId !== resource.id ||
        typeof pointer.targetId !== "string"
      )
        throw Error("管理结果标识无法读取，请保留原记录");
      const value = await service.resource(pointer.targetId);
      if (
        !live() ||
        token !== epoch.current ||
        studioDraftStorage().getItem(slot + ":lifecycle-receipt") !== raw
      )
        return;
      if (
        value.id !== pointer.targetId ||
        (value.owner_actor_id && value.owner_actor_id !== owner)
      )
        throw Error("管理结果的资源范围不匹配");
      if (pointer.operation === "copy") {
        setCopy(value);
        setCopyOpen(false);
      } else await onChanged(value);
      if (
        !live() ||
        token !== epoch.current ||
        studioDraftStorage().getItem(slot + ":lifecycle-receipt") !== raw
      )
        return;
      studioDraftStorage().removeItem(slot + ":lifecycle-receipt");
      setReceipt(null);
      setError("");
    } catch (cause) {
      if (live())
        setError("操作已有回执，实际结果尚未读回：" + errorMessage(cause));
    }
  }
  useEffect(() => {
    active.current = true;
    try {
      setPending(command.pending());
      void readReceipt();
    } catch (cause) {
      setError(errorMessage(cause));
    }
    return () => {
      active.current = false;
      epoch.current++;
    };
  }, [command]);
  function edit(values: any) {
    if (!live() || !draft) return;
    const next = { ...draft, ...values };
    try {
      studioDraftStorage().setItem(
        slot + ":lifecycle-draft",
        JSON.stringify(next),
      );
      setDraft(next);
    } catch (cause) {
      setError(errorMessage(cause));
    }
  }
  async function submit(operation: string, body: any, retry = false) {
    if (busy || !live()) return;
    setBusy(true);
    setError("");
    try {
      await (retry ? command.retry() : command.submit({ operation, body }));
      if (live()) await readReceipt();
    } catch (cause) {
      if (live()) setError(errorMessage(cause));
    } finally {
      if (live()) {
        setBusy(false);
        setPending(command.pending());
      }
    }
  }
  async function archive() {
    const operation = resource.status === "archived" ? "restore" : "archive",
      snapshot = { id: resource.id, revision: resource.revision };
    if (
      !(await confirm({
        title: operation === "archive" ? "归档此定义" : "恢复此定义",
        text:
          operation === "archive"
            ? "发布版本、任务与素材会保留。仍被使用的定义由服务端拒绝归档。"
            : "恢复后不会自动应用到角色。",
        type: operation === "archive" ? "danger" : "default",
      })) ||
      !live() ||
      latest.current.id !== snapshot.id
    )
      return;
    await submit(operation, {
      expectedRevision: snapshot.revision,
      reason:
        operation === "archive"
          ? "所有者归档未被引用的定义"
          : "所有者恢复定义目录",
    });
  }
  const sources = [
    ...(resource.draft_version
      ? [
          {
            value: "draft:" + resource.draft_version,
            label: "已保存草稿 " + resource.draft_version,
          },
        ]
      : []),
    ...(resource.published_version
      ? [
          {
            value: "published:" + resource.published_version,
            label: "已发布 " + resource.published_version,
          },
        ]
      : []),
  ];
  if (draft && !sources.some((option) => option.value === draft.source))
    sources.push({ value: draft.source, label: "先前选择 " + draft.source });
  return (
    <>
      <Card className="panel r-padded r-form-stack">
        <h2 className="r-inline-title">资源管理</h2>
        <Button
          type="button"
          variant="ghost"
          className="r-left-action"
          onClick={(event) => {
            copyTrigger.current = event.currentTarget;
            setCopyOpen(true);
          }}
        >
          创建独立副本
        </Button>
        <Button
          type="button"
          variant="ghost"
          className="r-left-action"
          onClick={() => {
            const url = URL.createObjectURL(
              new Blob([JSON.stringify(resource, null, 2)], {
                type: "application/json",
              }),
            );
            const anchor = document.createElement("a");
            anchor.href = url;
            anchor.download =
              resource.name +
              "-" +
              (resource.draft_version || "definition") +
              ".json";
            anchor.click();
            setTimeout(() => URL.revokeObjectURL(url), 0);
          }}
        >
          导出当前定义
        </Button>
        <Button
          type="button"
          variant="ghost"
          className="r-left-action"
          disabled={busy || !!pending || !!receipt || !draft}
          onClick={() => void archive()}
        >
          {resource.status === "archived" ? "恢复资源" : "归档资源"}
        </Button>
        {error && (
          <Alert variant="destructive">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}
        {copy && (
          <Button
            type="button"
            variant="link"
            onClick={() =>
              w.navigatePath(
                "/studio/" +
                  (copy.kind === "npc" ? "npc" : "edit") +
                  "/" +
                  encodeURIComponent(copy.id),
              )
            }
          >
            查看独立副本：{copy.name}
          </Button>
        )}
        {receipt && (
          <Button
            type="button"
            variant="outline"
            disabled={busy}
            onClick={() => void readReceipt()}
          >
            读取上次操作结果
          </Button>
        )}
        {pending && (
          <Alert>
            <AlertDescription>
              <p>原管理提交仍待核对，内容和身份保持。</p>
              <JsonDetails value={pending.value} title="原管理提交" />
              {pending.rejection ? (
                <Button
                  type="button"
                  variant="outline"
                  disabled={busy}
                  onClick={async () => {
                    try {
                      const value = await service.resource(resource.id);
                      if (!live()) return;
                      command.releaseRejected();
                      await onChanged(value);
                      if (live()) {
                        setPending(command.pending());
                        setError("已采用最新管理版本，当前输入仍保留。");
                      }
                    } catch (cause) {
                      if (live()) setError(errorMessage(cause));
                    }
                  }}
                >
                  明确采用最新管理版本
                </Button>
              ) : (
                <Button
                  type="button"
                  variant="outline"
                  disabled={busy}
                  onClick={() =>
                    void submit(
                      pending.value.operation,
                      pending.value.body,
                      true,
                    )
                  }
                >
                  重试原管理提交
                </Button>
              )}
            </AlertDescription>
          </Alert>
        )}
      </Card>
      <WorkspaceDialog
        open={copyOpen}
        onOpenChange={setCopyOpen}
        title="创建独立副本"
        returnFocus={() => copyTrigger.current}
        description="使用明确的保存或发布版本，建立未发布的独立草稿。"
        className="replica-studio"
        footer={
          <>
            <Button variant="outline" onClick={() => setCopyOpen(false)}>
              取消
            </Button>
            <Button
              type="submit"
              form={formId}
              disabled={busy || !!pending || !!receipt || !draft}
            >
              创建副本
            </Button>
          </>
        }
      >
        {draft ? (
          <form
            id={formId}
            className="r-form-stack"
            onSubmit={(event) => {
              event.preventDefault();
              const [source, ...version] = draft.source.split(":");
              void submit("copy", {
                expectedRevision: resource.revision,
                source,
                sourceVersion: version.join(":"),
                name: draft.name,
                reason: draft.reason,
              });
            }}
          >
            <Label htmlFor="copy-resource-name">
              副本名称
              <FieldInput
                id="copy-resource-name"
                required
                maxLength={200}
                value={draft.name}
                onChange={(event: any) => edit({ name: event.target.value })}
              />
            </Label>
            <Label htmlFor="copy-resource-source">
              复制来源
              <ChoiceSelect
                id="copy-resource-source"
                value={draft.source}
                onChange={(event: any) => edit({ source: event.target.value })}
              >
                {sources.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </ChoiceSelect>
            </Label>
            <Label htmlFor="copy-resource-reason">
              复制原因
              <FieldInput
                id="copy-resource-reason"
                required
                maxLength={1000}
                value={draft.reason}
                onChange={(event: any) => edit({ reason: event.target.value })}
              />
            </Label>
            {error && (
              <Alert variant="destructive">
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}
          </form>
        ) : (
          <p role="alert">资源管理草稿无法读取，原记录保留。</p>
        )}
      </WorkspaceDialog>
      <WorkspaceDialog
        open={historyOpen ?? localHistoryOpen}
        onOpenChange={(open) => {
          setLocalHistoryOpen(open);
          onHistoryOpenChange?.(open);
        }}
        title="发布版本历史"
        returnFocus={historyReturnFocus}
        description="每个发布版本保留当时的完整定义。"
        className="replica-studio"
      >
        <div className="r-form-stack">
          {versions.items.map((entry: any) => (
            <button
              key={entry.version}
              className="r-version-row"
              onClick={async () => {
                const token = ++publicationEpoch.current;
                try {
                  const value = await service.publication(
                    resource.id,
                    entry.version,
                  );
                  if (!live() || token !== publicationEpoch.current) return;
                  if (
                    value.id !== resource.id ||
                    value.version !== entry.version ||
                    value.immutable !== true
                  )
                    throw Error("精确发布版本尚未完整确认");
                  setPublication(value);
                } catch (cause) {
                  if (live()) setError(errorMessage(cause));
                }
              }}
            >
              <span>
                <b>
                  {entry.name} · {studioVersion(entry.version)}
                </b>
                <small>
                  {entry.created_at
                    ? userInstant(entry.created_at)
                    : "时间未记录"}
                </small>
              </span>
              {entry.version === resource.published_version && (
                <span className="r-muted">当前可用</span>
              )}
              <span>查看</span>
            </button>
          ))}
          {!versions.items.length && !versions.loading && (
            <p>尚未发布任何版本。先保存草稿，再发布为可用版本。</p>
          )}
          <CursorControls page={versions} label="发布版本" />
          {error && <p role="alert">{error}</p>}
          {publication && (
            <>
              <h3>{studioVersion(publication.version)} 的完整定义</h3>
              <SemanticTree value={publication.body} />
              <JsonDetails value={publication} title="不可变发布定义与来源" />
              <Button
                onClick={() => {
                  edit({ source: "published:" + publication.version });
                  setLocalHistoryOpen(false);
                  onHistoryOpenChange?.(false);
                  copyTrigger.current = historyReturnFocus?.() || null;
                  setCopyOpen(true);
                }}
              >
                以此版本创建独立副本
              </Button>
            </>
          )}
        </div>
      </WorkspaceDialog>
    </>
  );
}

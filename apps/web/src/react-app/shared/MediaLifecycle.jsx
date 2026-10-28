import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Disclosure, Textarea } from "../controls";
import WorkspaceCommandDialog from "./WorkspaceCommandDialog";
import WorkspaceInfo from "./WorkspaceInfo";
import useWorkspaceRead from "./useWorkspaceRead";
import useMediaFollow from "./useMediaFollow";
import { MediaReader, MediaMore } from "./MediaParts";
import {
  mediaReferencePath,
  requiredText,
} from "@/approved-ui/media-presentation.mjs";
import { studioDraftStorage } from "@/v3/proto/components/studio-draft-storage.js";
export default function MediaLifecycle({ model: m, asset, onAsset }) {
  const [deletionId, setDeletionId] = useState(asset.deletion_id || ""),
    [action, setAction] = useState(null),
    trigger = useRef(null);
  const references = useWorkspaceRead({
    identity: ["media-references", asset.id],
    characterScoped: false,
    read: (cursor) => m.service.references(asset.id, cursor),
  });
  const deletion = useMediaFollow({
    identity: ["media-deletion", asset.id, deletionId],
    enabled: !!deletionId,
    read: async () => {
      const r = await m.service.deletion(deletionId);
      if (r.id !== deletionId || r.asset_id !== asset.id)
        throw Error("删除记录范围不匹配。");
      const latest = await m.readAsset(asset.id);
      return { id: r.id, deletion: r, asset: latest };
    },
    pending: (r) =>
      ["pending", "running"].includes(r.deletion.status) ||
      (r.deletion.status === "failed" &&
        ["pending", "started", "retry", "cancel_requested"].includes(
          r.deletion.execution_status,
        )),
  });
  const current = deletion.items[0]?.deletion;
  useEffect(() => {
    if (deletion.items[0] && m.live()) onAsset(deletion.items[0].asset);
  }, [deletion.items[0]]);
  useEffect(() => {
    if (asset.deletion_id && !deletionId) setDeletionId(asset.deletion_id);
  }, [asset.deletion_id]);
  function begin(retry, event) {
    m.assertCurrent();
    trigger.current = event.currentTarget;
    const key = retry ? "retry:" + current.id : "delete";
    setAction({
      identity: "media-delete:" + asset.id + ":" + key,
      scope: ["media-lifecycle", asset.id, key],
      storageKey: `fluctlight-media:${m.owner}:${asset.id}:replica-lifecycle:${key}`,
      title: retry ? "重试失败的原删除" : "删除指定媒体版本",
      className: "replica-media",
      description:
        "先禁止读取，再由后台任务核对并删除指定对象版本。已有实际引用会阻止受理。",
      submitLabel: retry ? "确认重试原删除" : "确认申请删除",
      preAdmissionConflicts: [
        "revision_conflict",
        "media_asset_referenced",
        "media_delete_execution_active",
      ],
      initial: {
        revision: retry ? current.revision : asset.asset_revision,
        reason: "",
        confirm: false,
        referenceCount: references.ready
          ? references.extra.reference_count
          : null,
        status: current?.status,
        executionStatus: current?.execution_status,
      },
      validate: (d) => {
        requiredText(d.reason, "删除或重试原因", 1000);
        if (!retry && !d.confirm) throw Error("请明确确认删除该对象版本。");
        if (
          !retry &&
          (!Number.isSafeInteger(d.referenceCount) || d.referenceCount !== 0)
        )
          throw Error(
            "请先核对实际引用总数，仍有引用或尚未确认时不能申请删除。",
          );
        if (
          retry &&
          (d.status !== "failed" ||
            ["pending", "started", "retry", "cancel_requested"].includes(
              d.executionStatus,
            ))
        )
          throw Error("只有失败的原删除可以重试。");
      },
      build: (d) => ({
        ...(retry
          ? { expectedRevision: d.revision }
          : { expectedAssetRevision: d.revision, confirm: true }),
        reason: d.reason,
      }),
      execute: async (body, key) => {
        m.assertCurrent();
        const r = await (retry
          ? m.service.retryDeletion(current.id, {
              ...body,
              idempotencyKey: key,
            })
          : m.service.deleteAsset(asset.id, { ...body, idempotencyKey: key }));
        if (
          !r.id ||
          (retry && r.id !== current.id) ||
          r.asset_id !== asset.id ||
          !Number.isSafeInteger(r.revision)
        )
          throw Error("删除受理回执尚未完整确认。");
        return r;
      },
      readLatest: async () => ({
        asset: await m.readAsset(asset.id),
        references: await m.service.references(asset.id, ""),
        deletion: retry ? await m.service.deletion(current.id) : null,
      }),
      adoptLatest: (d, row) => {
        if (!Number.isSafeInteger(row.references.reference_count))
          throw Error("实际引用总数尚未确认。");
        if (m.live()) references.adopt(row.references);
        return {
          ...d,
          revision: retry ? row.deletion.revision : row.asset.asset_revision,
          referenceCount: row.references.reference_count,
          status: row.deletion?.status,
          executionStatus: row.deletion?.execution_status,
        };
      },
      onSaved: async (r) => {
        const [fresh, row] = await Promise.all([
          m.service.deletion(r.id),
          m.readAsset(asset.id),
        ]);
        if (
          fresh.id !== r.id ||
          fresh.asset_id !== asset.id ||
          fresh.revision < r.revision
        )
          throw Error("原删除受理记录尚未读回。");
        if (m.live()) {
          setDeletionId(r.id);
          onAsset(row);
          await references.load();
        }
      },
      fields: (d, edit) => (
        <div className="r-form-stack">
          <label className="replica-field">
            <span>删除或重试原因</span>
            <Textarea
              aria-label="删除或重试原因"
              value={d.reason}
              onChange={(e) => edit({ reason: e.target.value })}
            />
          </label>
          {!retry && (
            <label className="companion-checkbox-row">
              <Checkbox
                aria-label="确认删除指定对象版本"
                checked={d.confirm}
                onCheckedChange={(confirm) => edit({ confirm: !!confirm })}
              />
              确认删除指定对象版本；已删除字节不能从此页撤回。
            </label>
          )}
        </div>
      ),
    });
  }
  return (
    <Disclosure title="实际引用与删除">
      <div className="r-form-stack">
        {(() => {
          try {
            const slot = `fluctlight-media:${m.owner}:${asset.id}:replica-lifecycle:delete`;
            return [":command", ":command:outcome"].some((suffix) =>
              studioDraftStorage().getItem(slot + suffix),
            ) ? (
              <Button variant="outline" onClick={(e) => begin(false, e)}>
                继续核对原删除提交
              </Button>
            ) : null;
          } catch {
            return null;
          }
        })()}
        <WorkspaceInfo>
          引用总数包含其他受保护范围；当前页没有引用不代表可以删除。失败删除可恢复执行，已删除字节没有此页恢复接口。
        </WorkspaceInfo>
        <MediaReader source={references} label="实际引用" />
        <p>
          实际引用总数：
          {references.ready &&
          Number.isSafeInteger(references.extra.reference_count)
            ? references.extra.reference_count
            : "待读取"}
        </p>
        {references.items.map((r) => (
          <div className="media-reference-row" key={r.id}>
            <span>
              {{
                display_avatar: "显示头像",
                visual_identity: "规范视觉身份",
                visual_attempt: "视觉尝试",
                visual_session: "视觉会话",
                studio_material: "创作素材",
                item: "物品",
                model_reference_grant: "图片参考授权",
                conversation: "会话",
                conversation_message: "会话消息",
              }[r.target_type] || "实际引用"}
            </span>
            {mediaReferencePath(r, asset.id) && (
              <a href={mediaReferencePath(r, asset.id)}>处理这项引用</a>
            )}
          </div>
        ))}
        <MediaMore source={references} label="引用" />
        <Button
          variant="outline"
          onClick={() => {
            void references.load();
            if (deletionId) void deletion.load();
          }}
        >
          重新查询状态与引用
        </Button>
        <Button
          variant="destructive"
          disabled={
            !!deletionId || !["ready", "unavailable"].includes(asset.status)
          }
          onClick={(e) => begin(false, e)}
        >
          申请删除指定版本
        </Button>
        <MediaReader source={deletion} label="原删除状态" />
        {current && (
          <>
            <p>
              {{
                pending: "删除已受理，等待执行",
                running: "正在核对并删除对象版本",
                failed: "删除未完成",
                deleted: "指定对象版本已确认删除",
              }[current.status] || "删除状态待确认"}
            </p>
            <p>
              尝试 {current.attempt} ·{" "}
              {current.error_code || "实际任务按原记录继续"}
            </p>
            {current.task_id && (
              <a href={"#/diag/task/" + encodeURIComponent(current.task_id)}>
                查看原删除任务
              </a>
            )}
            <Button
              variant="outline"
              disabled={
                current.status !== "failed" ||
                ["pending", "started", "retry", "cancel_requested"].includes(
                  current.execution_status,
                )
              }
              onClick={(e) => begin(true, e)}
            >
              重试失败的原删除
            </Button>
          </>
        )}
        {deletion.paused && (
          <p>自动跟进已暂停，可以重新查询；后台删除仍继续。</p>
        )}
      </div>
      {action && (
        <WorkspaceCommandDialog
          action={action}
          onClose={() => setAction(null)}
          returnFocus={() => trigger.current}
        />
      )}
    </Disclosure>
  );
}

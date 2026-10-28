import { useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { ChoiceSelect, Disclosure, Textarea, FieldInput } from "../controls";
import WorkspaceCommandDialog from "./WorkspaceCommandDialog";
import WorkspaceInfo from "./WorkspaceInfo";
import useWorkspaceRead from "./useWorkspaceRead";
import { MediaReader, MediaMore } from "./MediaParts";
import { studioDraftStorage } from "@/v3/proto/components/studio-draft-storage.js";
import {
  referencePurposes,
  requiredText,
} from "@/approved-ui/media-presentation.mjs";
function GrantTarget({ model: m, draft: d, edit }) {
  const [query, setQuery] = useState("");
  const resources = useWorkspaceRead({
    identity: ["grant-target-options", query],
    characterScoped: false,
    enabled: d.targetScope === "studio_resource",
    read: (cursor) => m.w.services.studioClient.list({ q: query, cursor }),
  });
  const options =
    d.targetScope === "fluctlight" ? m.w.state.characters : resources.items;
  return (
    <>
      <label className="replica-field">
        <span>授权目标类型</span>
        <ChoiceSelect
          aria-label="授权目标类型"
          value={d.targetScope}
          onChange={(e) => edit({ targetScope: e.target.value, targetId: "" })}
        >
          <option value="fluctlight">角色</option>
          <option value="studio_resource">创作资源</option>
        </ChoiceSelect>
      </label>
      {d.targetScope === "studio_resource" && (
        <>
          <FieldInput
            aria-label="筛选目标创作资源"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          <MediaReader source={resources} label="目标资源" />
          <MediaMore source={resources} label="目标资源" />
        </>
      )}
      <label className="replica-field">
        <span>明确授权目标</span>
        <ChoiceSelect
          aria-label="明确授权目标"
          value={d.targetId}
          onChange={(e) => edit({ targetId: e.target.value })}
        >
          <option value="">请选择目标</option>
          {d.targetId && !options.some((r) => r.id === d.targetId) && (
            <option value={d.targetId}>原已选目标（名称待读取）</option>
          )}
          {options.map((r) => (
            <option value={r.id} key={r.id}>
              {r.name}
            </option>
          ))}
        </ChoiceSelect>
      </label>
    </>
  );
}
export default function MediaGrants({ model: m, asset }) {
  const [revoked, setRevoked] = useState(false),
    [action, setAction] = useState(null),
    [record, setRecord] = useState(null),
    [query, setQuery] = useState(""),
    trigger = useRef(null),
    grantsRef = useRef(null);
  const grants = useWorkspaceRead({
    identity: ["media-grants", asset.id, revoked],
    characterScoped: false,
    read: (cursor) =>
      m.service.grants({
        asset_id: asset.id,
        status: revoked ? "revoked" : "active",
        cursor,
      }),
  });
  grantsRef.current = grants;
  const resources = useWorkspaceRead({
    identity: ["media-grant-resources", query],
    characterScoped: false,
    read: (cursor) => m.w.services.studioClient.list({ q: query, cursor }),
  });
  const records = useWorkspaceRead({
    identity: ["media-grant-records", record],
    characterScoped: false,
    enabled: !!record,
    read: (cursor) => m.service.grantRecords(record.id, record.kind, cursor),
  });
  function base(key) {
    return {
      identity: "grant:" + asset.id + ":" + key,
      className: "replica-media",
      scope: ["media-grant", asset.id, key],
      storageKey: `fluctlight-media:${m.owner}:${asset.id}:replica-grant:${key}`,
    };
  }
  function recovery(key) {
    try {
      const slot = `fluctlight-media:${m.owner}:${asset.id}:replica-grant:${key}`;
      return [":command", ":command:outcome"].some((suffix) =>
        studioDraftStorage().getItem(slot + suffix),
      );
    } catch {
      return false;
    }
  }
  function create(event) {
    m.assertCurrent();
    trigger.current = event.currentTarget;
    setAction({
      ...base("create"),
      title: "限定图片参考用途",
      description: "参考不证明图中人物身份，也不会写成角色事实。",
      submitLabel: "保存参考授权",
      initial: {
        targetScope: m.target ? "fluctlight" : "studio_resource",
        targetId: m.target || "",
        purpose: "",
        reason: "",
        confirm: false,
        assetRevision: asset.asset_revision,
      },
      validate: (d) => {
        requiredText(d.reason, "授权原因", 1000);
        if (!d.targetId || !referencePurposes[d.purpose] || !d.confirm)
          throw Error("请选择目标、用途并明确勾选授权确认。");
        if (
          !asset.readable ||
          asset.kind !== "image" ||
          asset.byte_size > 32 * 1024 * 1024
        )
          throw Error("只能授权可读取且不超过32MiB的图片。");
        if (
          d.targetScope === "fluctlight" &&
          !m.w.state.characters.some(
            (c) =>
              c.id === d.targetId &&
              !["retired", "archived", "已退休", "已归档"].includes(c.status),
          )
        )
          throw Error("所选角色当前不可写。");
      },
      build: (d) => ({
        assetId: asset.id,
        expectedAssetRevision: d.assetRevision,
        targetScope: d.targetScope,
        targetId: d.targetId,
        purpose: d.purpose,
        confirm: true,
        reason: d.reason,
      }),
      execute: async (body, key) => {
        m.assertCurrent();
        const r = await m.service.createGrant({ ...body, idempotencyKey: key });
        if (
          !r.id ||
          r.asset_id !== asset.id ||
          r.status !== "active" ||
          r.target_id !== body.targetId ||
          r.target_scope !== body.targetScope ||
          r.purpose !== body.purpose
        )
          throw Error("授权回执尚未完整确认。");
        return r;
      },
      afterAcknowledged: (d, r) => ({ ...d, lastId: r.id }),
      readLatest: async () => ({ asset: await m.readAsset(asset.id) }),
      adoptLatest: (d, row) => ({
        ...d,
        assetRevision: row.asset.asset_revision,
      }),
      onSaved: async (r, body) => {
        const g = await m.service.grant(r.id);
        if (
          g.id !== r.id ||
          g.asset_id !== asset.id ||
          g.target_id !== body.targetId ||
          g.target_scope !== body.targetScope ||
          g.purpose !== body.purpose ||
          g.revision < r.revision
        )
          throw Error("原图片参考授权尚未完整读回。");
        if (m.live()) await grantsRef.current.load();
      },
      fields: (d, edit) => (
        <div className="r-form-stack">
          <GrantTarget model={m} draft={d} edit={edit} />
          <label className="replica-field">
            <span>参考用途</span>
            <ChoiceSelect
              aria-label="参考用途"
              value={d.purpose}
              onChange={(e) => edit({ purpose: e.target.value })}
            >
              <option value="">请选择用途</option>
              {Object.entries(referencePurposes).map(([key, label]) => (
                <option key={key} value={key}>
                  {label}
                </option>
              ))}
            </ChoiceSelect>
          </label>
          <label className="replica-field">
            <span>授权原因</span>
            <Textarea
              aria-label="授权原因"
              value={d.reason}
              onChange={(e) => edit({ reason: e.target.value })}
            />
          </label>
          <label className="companion-checkbox-row">
            <Checkbox
              aria-label="确认传送参考图片"
              checked={d.confirm}
              onCheckedChange={(confirm) => edit({ confirm: !!confirm })}
            />
            我确认允许图片在所选目标和用途中传给已配置的生成服务。
          </label>
          <WorkspaceInfo>
            人物画面参考不能替代角色规范身份。撤销只能阻止尚未消费的新执行，已传出的图片不能通过撤销收回。
          </WorkspaceInfo>
        </div>
      ),
    });
  }
  function revoke(g, event) {
    m.assertCurrent();
    trigger.current = event.currentTarget;
    setAction({
      ...base("revoke:" + g.id),
      title: "撤销这份参考授权？",
      description:
        "尚未消费图片的新执行将被阻止，已发送内容和生成结果保留来源。",
      submitLabel: "确认撤销授权",
      initial: { revision: g.revision, reason: "" },
      validate: (d) => requiredText(d.reason, "撤销原因", 1000),
      build: (d) => ({ expectedRevision: d.revision, reason: d.reason }),
      execute: async (body, key) => {
        m.assertCurrent();
        const r = await m.service.revokeGrant(g.id, {
          ...body,
          idempotencyKey: key,
        });
        if (r.id !== g.id || r.asset_id !== asset.id || r.status !== "revoked")
          throw Error("撤销回执尚未确认。");
        return r;
      },
      readLatest: () => m.service.grant(g.id),
      adoptLatest: (d, row) => ({ ...d, revision: row.revision }),
      onSaved: async (r) => {
        const fresh = await m.service.grant(g.id);
        if (
          fresh.id !== g.id ||
          fresh.revision < r.revision ||
          fresh.status !== "revoked"
        )
          throw Error("撤销后的实际授权尚未读回。");
        if (m.live()) await grantsRef.current.load();
      },
      fields: (d, edit) => (
        <label className="replica-field">
          <span>撤销原因</span>
          <Textarea
            aria-label="撤销原因"
            value={d.reason}
            onChange={(e) => edit({ reason: e.target.value })}
          />
        </label>
      ),
    });
  }
  return (
    <Disclosure title="模型参考授权" open>
      <div className="r-form-stack">
        <WorkspaceInfo>
          图片最多32MiB，只授权给明确目标和用途；授权不证明身份或画面事实。
        </WorkspaceInfo>
        <div className="r-inline-actions">
          {recovery("create") && (
            <Button variant="outline" onClick={create}>
              继续核对原参考授权
            </Button>
          )}
          <Button
            variant="outline"
            disabled={
              asset.kind !== "image" ||
              !asset.readable ||
              asset.byte_size > 32 * 1024 * 1024
            }
            onClick={create}
          >
            新增参考授权
          </Button>
          <Button variant="ghost" onClick={() => setRevoked((v) => !v)}>
            {revoked ? "查看有效授权" : "查看已撤销授权"}
          </Button>
        </div>
        <Disclosure title="搜索目标创作资源">
          <FieldInput
            aria-label="搜索目标创作资源"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          <MediaReader source={resources} label="目标创作资源" />
          {resources.items.map((r) => (
            <p key={r.id}>{r.name}</p>
          ))}
          <MediaMore source={resources} label="目标资源" />
        </Disclosure>
        <MediaReader source={grants} label="参考授权" />
        {grants.ready && !grants.items.length && <p>当前没有这类参考授权。</p>}
        {grants.items.map((g) => (
          <div className="media-grant-row" key={g.id}>
            <b>
              {referencePurposes[g.purpose] || "参考用途待确认"} ·{" "}
              {g.status === "active" ? "有效" : "已撤销"}
            </b>
            <a
              href={
                g.target_scope === "fluctlight"
                  ? "#/c/" + encodeURIComponent(g.target_id) + "/profile"
                  : "#/studio/edit/" + encodeURIComponent(g.target_id)
              }
            >
              查看授权目标
            </a>
            {g.status === "active" && (
              <Button variant="outline" onClick={(e) => revoke(g, e)}>
                撤销授权
              </Button>
            )}
            {g.status !== "active" && recovery("revoke:" + g.id) && (
              <Button variant="outline" onClick={(e) => revoke(g, e)}>
                继续核对原撤销
              </Button>
            )}
            {g.status === "active" && g.target_scope === "fluctlight" && (
              <a
                href={
                  "#/c/" +
                  encodeURIComponent(g.target_id) +
                  "/media/generate?grant=" +
                  encodeURIComponent(g.id)
                }
              >
                用这份授权提出图片请求
              </a>
            )}
            <Button
              variant="ghost"
              onClick={() => setRecord({ id: g.id, kind: "history" })}
            >
              授权历史
            </Button>
            <Button
              variant="ghost"
              onClick={() => setRecord({ id: g.id, kind: "uses" })}
            >
              实际使用记录
            </Button>
          </div>
        ))}
        <MediaMore source={grants} label="授权" />
        {record && (
          <>
            <MediaReader source={records} label="授权记录" />
            {records.items.map((row, index) => (
              <Disclosure
                key={row.id || index}
                title={
                  record.kind === "uses" ? "实际任务使用记录" : "授权修订记录"
                }
              >
                <pre className="replica-source-record">
                  {JSON.stringify(row, null, 2)}
                </pre>
              </Disclosure>
            ))}
            {records.ready && !records.items.length && <p>尚无这类记录。</p>}
            <MediaMore source={records} label="记录" />
          </>
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

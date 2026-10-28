import { useRef, useState } from "react";
import {
  ArrowUpRightIcon,
  CheckIcon,
  RefreshCwIcon,
  ShieldCheckIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Textarea, FieldInput, Disclosure, Badge } from "../controls";
import SemanticTree from "./SemanticTree";
import JsonDetails from "./JsonDetails";
import WorkspaceDialog from "./WorkspaceDialog";
import WorkspaceCommandDialog from "./WorkspaceCommandDialog";
import WorkspaceInfo from "./WorkspaceInfo";
import LoadingState from "./LoadingState";
import useWorkspaceRead from "./useWorkspaceRead";
import { EmptyState, InlineFeedback } from "./Presentation";
import { stateLabel } from "@/v3/domain-presentation.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";
const foundationLabels = {
  proposed: "待审阅",
  accepted: "已接受",
  rejected: "保留原设定",
};
const capabilityLabels = {
  proposed: "新申请",
  reviewing: "评估中",
  accepted: "需求已接受",
  fulfilled: "已记录交付",
  rejected: "未接受",
  cancelled: "需求已取消",
};
const transitions = {
  proposed: [
    ["reviewing", "开始评估"],
    ["accepted", "接受需求"],
    ["rejected", "拒绝申请"],
    ["cancelled", "取消申请"],
  ],
  reviewing: [
    ["accepted", "接受需求"],
    ["rejected", "拒绝申请"],
    ["cancelled", "取消申请"],
  ],
  accepted: [
    ["fulfilled", "记录已交付实现"],
    ["cancelled", "取消需求"],
  ],
};
const field = (label, child) => (
  <label className="replica-field lp-wide">
    <span>{label}</span>
    {child}
  </label>
);
function Reader({ source, label }) {
  return (
    <>
      {source.busy && <LoadingState label={"正在读取" + label + "…"} />}{" "}
      {source.error && (
        <InlineFeedback role="alert">
          {source.error}
          <Button variant="outline" onClick={source.retry}>
            重新读取{label}
          </Button>
        </InlineFeedback>
      )}
    </>
  );
}
export default function ProfileGovernance({
  model: m,
  open,
  onClose,
  initialTab = "foundation",
  foundation,
  returnFocus,
}) {
  const w = m.w,
    fid = m.fid,
    owner = w.state.session.actorId;
  const [tab, setTab] = useState(initialTab),
    [action, setAction] = useState(null),
    [selected, setSelected] = useState(null),
    [notice, setNotice] = useState("");
  const commandTrigger = useRef(null);
  const check = (row, id) => {
    if (!row?.id || row.fluctlight_id !== fid || (id && row.id !== id))
      throw Error("治理对象范围不匹配。");
    return row;
  };
  const capabilities = useWorkspaceRead({
    identity: ["profile-capabilities"],
    enabled: open,
    read: async (cursor) => {
      const page = await w.client.governancePage(fid, "capability", {
        cursor,
        limit: 20,
      });
      return { ...page, items: page.items.map((row) => check(row)) };
    },
  });
  const capDetail = useWorkspaceRead({
    identity: ["profile-capability", selected],
    enabled: open && !!selected,
    read: async () => ({
      items: [
        check(
          await w.client.governanceObject(fid, "capability", selected),
          selected,
        ),
      ],
      next_cursor: "",
    }),
  });
  const history = useWorkspaceRead({
    identity: ["profile-capability-history", selected],
    enabled: open && !!selected,
    read: (cursor) =>
      w.client.governanceHistory(fid, "capability", selected, cursor),
  });
  const claims = useWorkspaceRead({
    identity: ["profile-governance-self"],
    enabled: open && tab === "self",
    read: async (cursor) => {
      const page = await w.client.selfClaims(fid, {
        status: "active",
        cursor,
        limit: 20,
      });
      return { ...page, items: page.items.map((row) => check(row)) };
    },
  });
  const source =
    tab === "foundation"
      ? foundation
      : tab === "capabilities"
        ? capabilities
        : claims;
  const current = capDetail.items[0];
  const refreshCapabilities = () =>
    Promise.all([
      capabilities.load(),
      ...(selected ? [capDetail.load(), history.load()] : []),
    ]);
  function foundationCommand(row, operation, event) {
    if (!m.live() || m.readOnly.value) return;
    commandTrigger.current = event.currentTarget;
    const id = row.id,
      isRestore = operation === "restore",
      wanted = operation === "reject" ? "rejected" : "accepted";
    setAction({
      identity: "profile-foundation:" + id + ":" + operation,
      scope: ["profile-foundation", fid, id, operation],
      storageKey:
        "fluctlight-profile:" +
        owner +
        ":" +
        fid +
        ":foundation:" +
        id +
        ":" +
        operation,
      casOnly: true,
      title: isRestore
        ? "恢复历史身份内容"
        : operation === "accept"
          ? "接受这条提议"
          : "保留原设定",
      description:
        "通过正式身份修订命令处理，实际穿着、位置和生活历史继续由对应记录维护。",
      submitLabel: isRestore
        ? "确认历史恢复"
        : operation === "accept"
          ? "接受并应用"
          : "确认保留原设定",
      initial: {
        revision: m.snapshot.value.foundationRevision,
        proposalStatus: row.status,
        proposalBase: row.base_revision,
        reason: "",
      },
      validate: (d) => {
        if (!d.reason.trim() || [...d.reason.trim()].length > 1024)
          throw Error("请填写操作说明，最多1024个字符。");
        if (!Number.isSafeInteger(d.revision))
          throw Error("当前身份版本尚未完整读到。");
        if (!isRestore && d.proposalStatus !== "proposed")
          throw Error("这条提议已经处理，请返回列表核对历史。");
        if (operation === "accept" && d.proposalBase !== d.revision)
          throw Error("提议基线已过期，请保留变更并对照当前身份重新提出。");
      },
      build: (d) => ({
        expectedRevision: d.revision,
        reason: d.reason.trim(),
        ...(isRestore ? { targetRevision: row.revision } : {}),
      }),
      execute: async (value) => {
        if (!m.live() || m.readOnly.value)
          throw Error("角色范围或可写状态已变化。");
        const result = await (isRestore
          ? w.client.rollbackFoundationRevision(fid, value)
          : operation === "accept"
            ? w.client.acceptFoundationRevision(fid, id, value)
            : w.client.rejectFoundationRevision(fid, id, value));
        if (
          !result?.id ||
          (!isRestore && result.id !== id) ||
          result.fluctlight_id !== fid ||
          result.status !== wanted ||
          !Number.isSafeInteger(result.revision)
        )
          throw Error("治理回执尚未完整确认，请核对实际版本。");
        return result;
      },
      readLatest: async () => {
        const [raw, proposal] = await Promise.all([
          w.client.detail(fid),
          w.client.governanceObject(fid, "foundation", id),
        ]);
        if (raw.id !== fid) throw Error("角色范围不匹配。");
        return { detail: raw, proposal: check(proposal, id) };
      },
      adoptLatest: (d, latest) => ({
        ...d,
        revision: latest.detail.current_revision,
        proposalStatus: latest.proposal.status,
        proposalBase: latest.proposal.base_revision,
      }),
      onSaved: async (result) => {
        const saved = check(
          await w.client.governanceObject(fid, "foundation", result.id),
          result.id,
        );
        if (saved.status !== wanted)
          throw Error("操作已受理，治理状态尚未读回。");
        if (m.live()) {
          if (
            !(await m.read({
              minimumRevision: operation === "reject" ? null : result.revision,
            }))
          )
            throw Error(m.error.value || "身份版本尚未读回。");
          await foundation.load();
          setNotice("治理结果已读回，当前身份版本以下方实际记录为准。");
        }
      },
      fields: (d, edit) =>
        field(
          "操作说明",
          <Textarea
            aria-label="身份提议操作说明"
            value={d.reason}
            onChange={(e) => edit({ reason: e.target.value })}
          />,
        ),
    });
  }
  function capabilityCommand(row, status, event) {
    if (!m.live() || m.readOnly.value) return;
    commandTrigger.current = event.currentTarget;
    const id = row.id,
      label =
        transitions[row.status]?.find(([value]) => value === status)?.[1] ||
        "审核申请";
    setAction({
      identity: "profile-capability:" + id + ":" + status,
      scope: ["profile-capability", fid, id, status],
      storageKey:
        "fluctlight-profile:" +
        owner +
        ":" +
        fid +
        ":capability:" +
        id +
        ":" +
        status,
      title: label,
      description: "保存真实需求审核；Tool 的实际授权与执行策略分别管理。",
      submitLabel: label,
      initial: {
        revision: row.revision,
        note: "",
        version: row.registered_capability?.version || "",
      },
      preAdmissionConflicts: ["capability_review_conflict"],
      validate: (d) => {
        if (!d.note.trim() || d.note.trim().length > 2000)
          throw Error("请填写审核说明，最多2000个字符。");
        if (
          status === "fulfilled" &&
          (!d.version.trim() || d.version.trim().length > 128)
        )
          throw Error("请核对实际已注册的实现版本。");
      },
      build: (d) => ({
        status,
        note: d.note.trim(),
        expectedRevision: d.revision,
        ...(status === "fulfilled"
          ? { capabilityVersion: d.version.trim() }
          : {}),
      }),
      execute: async (value, key) => {
        if (!m.live() || m.readOnly.value)
          throw Error("角色范围或可写状态已变化。");
        const result = await w.client.reviewScopedCapabilityRequest(fid, id, {
          ...value,
          idempotencyKey: key,
        });
        check(result, id);
        if (!Number.isSafeInteger(result.revision) || result.status !== status)
          throw Error("审核回执尚未完整确认，原提交仍保留。");
        return result;
      },
      readLatest: async () =>
        check(await w.client.governanceObject(fid, "capability", id), id),
      adoptLatest: (d, latest) => ({ ...d, revision: latest.revision }),
      onSaved: async (result) => {
        const saved = check(
          await w.client.governanceObject(fid, "capability", id),
          id,
        );
        if (
          saved.revision < result.revision ||
          (saved.revision === result.revision && saved.status !== status)
        )
          throw Error("审核已保存，当前申请尚未完整读回。");
        if (m.live()) {
          capDetail.adopt({ items: [saved], next_cursor: "" });
          await capabilities.load();
          await history.load();
          setNotice("审核结果已读回；实际能力授权保持独立。");
        }
      },
      fields: (d, edit) => (
        <>
          {field(
            "审核说明",
            <Textarea
              aria-label="能力审核说明"
              value={d.note}
              onChange={(e) => edit({ note: e.target.value })}
            />,
          )}
          {status === "fulfilled" &&
            field(
              "实际实现版本",
              <FieldInput
                aria-label="能力实现版本"
                value={d.version}
                onChange={(e) => edit({ version: e.target.value })}
              />,
            )}
        </>
      ),
    });
  }
  return (
    <>
      <WorkspaceDialog
        className="replica-profile"
        open={open && !action}
        onOpenChange={(value) => {
          if (!value) onClose();
        }}
        title="身份与能力"
        description="审阅稳定设定的变化，以及角色提出的能力需求。"
        wide
        returnFocus={returnFocus}
      >
        <div className="companion-stack">
          <Tabs
            value={tab}
            onValueChange={(value) => {
              setTab(value);
              setSelected(null);
            }}
          >
            <TabsList variant="line" className="replica-segment-tabs">
              <TabsTrigger value="foundation">身份核心提议</TabsTrigger>
              <TabsTrigger value="capabilities">能力申请</TabsTrigger>
              <TabsTrigger value="self">发展中的自我</TabsTrigger>
            </TabsList>
            {notice && <WorkspaceInfo>{notice}</WorkspaceInfo>}
            <TabsContent value="foundation">
              <div className="companion-action-row">
                <Button
                  variant="ghost"
                  disabled={foundation.busy}
                  onClick={foundation.load}
                >
                  <RefreshCwIcon size={14} />
                  刷新身份提议
                </Button>
                <Button variant="link" asChild>
                  <a href={m.governance("foundation")}>
                    完整修订与提议工具
                    <ArrowUpRightIcon size={13} />
                  </a>
                </Button>
              </div>
              <Reader source={foundation} label="身份提议" />
              <div className="companion-stack">
                {foundation.items.map((row) => {
                  const stale =
                      row.status === "proposed" &&
                      row.base_revision !== m.snapshot.value.foundationRevision,
                    before = row.base_snapshot || {
                      ...m.core.value,
                      personality: m.detail.value?.personality,
                    };
                  return (
                    <div className="companion-proposal" key={row.id}>
                      <div className="companion-row-between">
                        <h3>
                          {row.source === "initialization"
                            ? "角色初始资料"
                            : "身份修订 v" + row.revision}
                        </h3>
                        <Badge variant="outline">
                          {foundationLabels[row.status] ||
                            stateLabel(row.status)}
                        </Badge>
                      </div>
                      <p>
                        {row.reason || "未提供额外说明。"} ·{" "}
                        {userInstant(row.created_at)}
                      </p>
                      <Disclosure title="变更与当前设定对照">
                        <div className="companion-diff">
                          <div>
                            <span>
                              {row.base_snapshot
                                ? "提议时的基线"
                                : "当前读到的设定"}
                            </span>
                            <SemanticTree
                              value={before}
                              identity={m.core.value.identity}
                            />
                          </div>
                          <div>
                            <span>本次提议的变更</span>
                            <SemanticTree
                              value={row.changes}
                              identity={m.core.value.identity}
                            />
                          </div>
                        </div>
                      </Disclosure>
                      <JsonDetails value={row} title="完整提议与来源" />
                      {stale && (
                        <WorkspaceInfo>
                          提议基线已经过期，可保留变更并对照当前身份重新提出。
                        </WorkspaceInfo>
                      )}
                      <div className="companion-action-row">
                        {row.status === "proposed" ? (
                          <>
                            <Button
                              variant="outline"
                              disabled={m.readOnly.value}
                              onClick={(e) =>
                                foundationCommand(row, "reject", e)
                              }
                            >
                              保留原设定
                            </Button>
                            <Button
                              disabled={
                                m.readOnly.value ||
                                stale ||
                                !Number.isSafeInteger(
                                  m.snapshot.value.foundationRevision,
                                )
                              }
                              onClick={(e) =>
                                foundationCommand(row, "accept", e)
                              }
                            >
                              接受这条提议
                              <CheckIcon size={13} />
                            </Button>
                          </>
                        ) : row.status === "accepted" &&
                          row.revision < m.snapshot.value.foundationRevision ? (
                          <Button
                            variant="outline"
                            disabled={m.readOnly.value}
                            onClick={(e) =>
                              foundationCommand(row, "restore", e)
                            }
                          >
                            恢复此版身份内容
                          </Button>
                        ) : null}
                      </div>
                    </div>
                  );
                })}
              </div>
              {foundation.ready &&
                !foundation.items.length &&
                !foundation.error && (
                  <EmptyState>
                    <h3>当前没有身份修订提议</h3>
                    <p>新的提议会带着来源和修改前后的对比，等待审阅。</p>
                  </EmptyState>
                )}
              {foundation.cursor && (
                <Button
                  variant="outline"
                  disabled={foundation.busy}
                  onClick={foundation.more}
                >
                  加载更多身份修订
                </Button>
              )}
            </TabsContent>
            <TabsContent value="capabilities">
              <WorkspaceInfo>
                申请审核与 Tool
                授权分别管理。记录已交付需要匹配真实注册的实现版本。
              </WorkspaceInfo>
              <Button
                variant="ghost"
                disabled={capabilities.busy || capDetail.busy || history.busy}
                onClick={refreshCapabilities}
              >
                <RefreshCwIcon size={14} />
                刷新能力申请
              </Button>
              {selected ? (
                <>
                  <Button variant="link" onClick={() => setSelected(null)}>
                    返回能力申请
                  </Button>
                  <Reader source={capDetail} label="申请详情" />
                  {current && (
                    <div className="companion-stack">
                      <div className="companion-proposal">
                        <div className="companion-row-between">
                          <h3>{current.title}</h3>
                          <Badge variant="outline">
                            {capabilityLabels[current.status]}
                          </Badge>
                        </div>
                        <p>{current.description}</p>
                        <p>{current.rationale}</p>
                        {current.status === "accepted" && (
                          <WorkspaceInfo>
                            {current.registered_capability?.available
                              ? "已注册实现版本：" +
                                current.registered_capability.version
                              : "当前未读到可交付的注册实现。"}
                          </WorkspaceInfo>
                        )}
                        <div className="companion-action-row">
                          {(transitions[current.status] || []).map(
                            ([status, label]) => (
                              <Button
                                key={status}
                                variant={
                                  status === "accepted" ||
                                  status === "fulfilled"
                                    ? "default"
                                    : "outline"
                                }
                                disabled={
                                  m.readOnly.value ||
                                  (status === "fulfilled" &&
                                    !current.registered_capability?.available)
                                }
                                onClick={(e) =>
                                  capabilityCommand(current, status, e)
                                }
                              >
                                {label}
                              </Button>
                            ),
                          )}
                        </div>
                        <JsonDetails
                          value={current}
                          title="完整能力申请与来源"
                        />
                      </div>
                      <Disclosure title="审核历史">
                        <Reader source={history} label="审核历史" />
                        {history.items.map((row, index) => (
                          <JsonDetails
                            key={row.id || index}
                            value={row}
                            title={row.note || row.reason || "审核记录"}
                          />
                        ))}
                        {history.cursor && (
                          <Button
                            variant="outline"
                            disabled={history.busy}
                            onClick={history.more}
                          >
                            加载更多审核记录
                          </Button>
                        )}
                      </Disclosure>
                    </div>
                  )}
                </>
              ) : (
                <>
                  <Reader source={capabilities} label="能力申请" />
                  {capabilities.items.map((row) => (
                    <div className="companion-capability" key={row.id}>
                      <span className="companion-icon-tile">
                        <ShieldCheckIcon size={20} />
                      </span>
                      <div>
                        <h3>{row.title}</h3>
                        <p>{row.rationale || row.description}</p>
                        <Badge variant="outline">
                          {capabilityLabels[row.status]}
                        </Badge>
                      </div>
                      <Button
                        variant="outline"
                        onClick={() => setSelected(row.id)}
                      >
                        查看与审核
                      </Button>
                    </div>
                  ))}
                  {capabilities.ready &&
                    !capabilities.items.length &&
                    !capabilities.error && (
                      <EmptyState>
                        <h3>当前没有能力申请</h3>
                        <p>真实运行中提出的需求会带着来源出现在这里。</p>
                      </EmptyState>
                    )}
                  {capabilities.cursor && (
                    <Button
                      variant="outline"
                      disabled={capabilities.busy}
                      onClick={capabilities.more}
                    >
                      加载更多能力申请
                    </Button>
                  )}
                </>
              )}
            </TabsContent>
            <TabsContent value="self">
              <Reader source={claims} label="自我认识" />
              {claims.items.map((row) => (
                <div className="companion-reading-block" key={row.id}>
                  <h3>{row.claim}</h3>
                  <p>
                    {row.source_valid ? "来源已读到" : "来源需要重新核对"}
                    {row.expired ? " · 已超过有效期限" : ""}
                  </p>
                  <JsonDetails value={row} title="完整认识与证据" />
                </div>
              ))}
              {claims.ready && !claims.items.length && !claims.error && (
                <EmptyState>
                  <h3>当前没有已记录的认识</h3>
                </EmptyState>
              )}
              {claims.cursor && (
                <Button
                  variant="outline"
                  disabled={claims.busy}
                  onClick={claims.more}
                >
                  加载更多认识
                </Button>
              )}
              <Button
                variant="outline"
                onClick={() =>
                  w.navigatePath(
                    "/c/" + encodeURIComponent(fid) + "/growth?tab=self",
                  )
                }
              >
                去记忆中查看完整来源
                <ArrowUpRightIcon size={14} />
              </Button>
            </TabsContent>
          </Tabs>
        </div>
      </WorkspaceDialog>
      {action && (
        <WorkspaceCommandDialog
          action={action}
          onClose={() => setAction(null)}
          returnFocus={() => commandTrigger.current}
        />
      )}
    </>
  );
}

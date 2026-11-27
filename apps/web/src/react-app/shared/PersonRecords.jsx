import { ResponsiveTabsList } from '@/components/patterns';
import { useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Tabs, TabsTrigger } from "@/components/ui/tabs";
import { Badge, Disclosure, Textarea } from "../controls";
import WorkspaceDialog from "./WorkspaceDialog";
import WorkspaceCommandDialog from "./WorkspaceCommandDialog";
import WorkspaceInfo from "./WorkspaceInfo";
import useWorkspaceRead from "./useWorkspaceRead";
import { PeopleFact, PeopleReader } from "./PeopleParts";
import JsonDetails from "./JsonDetails";
import { validTimeRange } from "./schedule-presentation.mjs";
import { isInternalId } from "@/v3/internal-id.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";
const labels = {
  encounters: "接触记录",
  presences: "在场记录",
  opportunities: "相遇机会",
};
const states = {
  recorded: "接触已记录",
  pending: "等待判断",
  deferred: "稍后判断",
  contacted: "接触已记录",
  declined: "接触未获接受",
  passed: "没有选择接触",
  expired: "窗口已结束",
  failed: "判断失败",
  active: "有效记录",
  ended: "已结束",
  cancelled: "已取消",
};
const downloadTitles = {
  encounters: "原始接触记录（诊断）",
  presences: "原始在场记录（诊断）",
  opportunities: "原始相遇机会记录（诊断）",
};
const resultReasons = new Map([
  ["deferred_attempts_exhausted", "已达到延后判断次数上限"],
  ["actual_presence_changed", "实际同地条件已经变化"],
  ["No actual decision receipt was committed", "未保存实际判断结果"],
  ["Actual shared presence no longer holds", "实际同地条件已失效"],
  ["Current autonomy policy blocks this automatic encounter", "当前自主策略限制了这次自动接触"],
]);
const readableText = (value) =>
  typeof value === "string" && value.trim() && !isInternalId(value) ? value : "";
const validInstant = (value) =>
  typeof value === "string" && Number.isFinite(Date.parse(value));
const recordInstant = (row) =>
  [row.occurred_at, row.start_at, row.created_at].find(validInstant);
function recordStatus(row, kind) {
  const candidates = kind === "encounters"
    ? [row.outcome, row.result?.status, row.status]
    : [row.status, row.result?.status];
  const status = candidates.find((value) => Object.hasOwn(states, value));
  if (status) return states[status];
  return kind === "encounters" && row.life_event_id ? states.recorded : "";
}

function RecordSummary({ row, kind, placeName }) {
  const instant = recordInstant(row);
  const range = validTimeRange(row.start_at, row.end_at);
  const place = placeName(row.place_id);
  const summary = readableText(row.summary) || readableText(row.activity);
  const status = recordStatus(row, kind);
  const result = row.result;
  const resultText = [result, result?.summary, result?.message, result?.detail, result?.reason]
    .map((value) => resultReasons.get(value) || readableText(value))
    .filter((value) => value && value !== summary && value !== status);
  const outcomes = [...new Set([
    status,
    ...resultText,
    kind === "encounters" && row.relationship_id && "已关联关系记录",
    kind === "encounters" && row.memory_id && "已关联记忆记录",
  ].filter(Boolean))];
  return (
    <>
      {(instant || range) && (
        <PeopleFact label={kind === "encounters" ? "接触时间" : "记录时间"}>
          {range && !validInstant(row.occurred_at) ? (
            <>
              <time dateTime={row.start_at}>{userInstant(row.start_at)}</time>
              {" 至 "}
              <time dateTime={row.end_at}>{userInstant(row.end_at)}</time>
            </>
          ) : <time dateTime={instant}>{userInstant(instant)}</time>}
        </PeopleFact>
      )}
      {place && <PeopleFact label="地点">{place}</PeopleFact>}
      {summary && <PeopleFact label={kind === "encounters" ? "发生事项" : "事项"}>{summary}</PeopleFact>}
      {outcomes.length > 0 && (
        <PeopleFact label="结果">
          {outcomes.map((text) => <p key={text}>{text}</p>)}
        </PeopleFact>
      )}
    </>
  );
}

export default function PersonRecords({ model: m, npc, placeName, onUpdated }) {
  const [kind, setKind] = useState("encounters"),
    [selection, setSelection] = useState(null),
    [action, setAction] = useState(null),
    trigger = useRef(null),
    commandTrigger = useRef(null);
  const check = (row) => {
    if (!row?.id || row.npc_id !== npc.id || row.world_id !== npc.world_id)
      throw Error("接触或在场记录不属于当前人物。");
    return row;
  };
  const records = useWorkspaceRead({
    identity: ["npc-records", npc.id, kind],
    read: async (cursor) => {
      const page = await m.w.client.npcRecords(m.fid, npc.id, kind, cursor);
      return { ...page, items: page.items.map(check) };
    },
  });
  const detail = useWorkspaceRead({
    identity: ["npc-opportunity", selection?.id],
    enabled: selection?.kind === "opportunities",
    read: async () => ({
      items: [
        check(await m.w.client.npcOpportunity(m.fid, npc.id, selection.id)),
      ],
      next_cursor: "",
    }),
  });
  const selected =
    selection?.kind === "opportunities"
      ? detail.items[0]
      : records.items.find((row) => row.id === selection?.id);
  function retry(event) {
    if (!selected) return;
    m.assertWritable();
    commandTrigger.current = event.currentTarget;
    const id = selected.id;
    setAction({
      identity: "npc-opportunity:" + id,
      scope: ["people-opportunity", m.fid, npc.id, id],
      storageKey: `fluctlight-people:${m.owner}:${m.fid}:opportunity:${id}`,
      title: "请求重新判断这次相遇",
      description: "重新核对原窗口、在场和人格条件，不强制接触或建立结识。",
      submitLabel: "请求重新判断",
      initial: { revision: selected.revision, reason: "" },
      validate: (d) => {
        if (!d.reason.trim() || [...d.reason.trim()].length > 500)
          throw Error("请填写重新判断说明，最多500个字符。");
      },
      build: (d) => ({ expectedRevision: d.revision, reason: d.reason.trim() }),
      execute: async (body, key) => {
        m.assertWritable();
        const r = await m.w.client.retryNPCOpportunity(m.fid, npc.id, id, {
          ...body,
          idempotencyKey: key,
        });
        if (
          r.id !== id ||
          r.status !== "pending" ||
          r.acquaintance_created !== false ||
          !Number.isSafeInteger(r.revision)
        )
          throw Error("相遇重新判断回执尚未完整确认。");
        return r;
      },
      readLatest: async () =>
        check(await m.w.client.npcOpportunity(m.fid, npc.id, id)),
      adoptLatest: (d, row) => {
        if (!["failed", "deferred"].includes(row.status))
          throw Error("原机会状态已变化，请返回查看实际结果。");
        return { ...d, revision: row.revision };
      },
      onSaved: async (r) => {
        const row = check(await m.w.client.npcOpportunity(m.fid, npc.id, id));
        if (row.revision < r.revision) throw Error("相遇机会新版本尚未读回。");
        if (m.live()) {
          detail.adopt({ items: [row], next_cursor: "" });
          records.adopt({
            ...records.extra,
            items: records.items.map((old) => (old.id === id ? row : old)),
            next_cursor: records.cursor,
          });
          await onUpdated();
        }
      },
      fields: (d, edit) => (
        <label className="replica-field">
          <span>重新判断说明</span>
          <Textarea
            aria-label="重新判断说明"
            value={d.reason}
            onChange={(e) => edit({ reason: e.target.value })}
          />
        </label>
      ),
    });
  }
  return (
    <>
      <Disclosure title="接触、在场与相遇记录">
        <Tabs
          value={kind}
          onValueChange={(value) => {
            setKind(value);
            setSelection(null);
          }}
        >
          <ResponsiveTabsList value={kind} variant="line">
            {Object.entries(labels).map(([value, label]) => (
              <TabsTrigger key={value} value={value}>
                {label}
              </TabsTrigger>
            ))}
          </ResponsiveTabsList>
        </Tabs>
        <Button variant="ghost" disabled={records.busy} onClick={records.load}>
          刷新{labels[kind]}
        </Button>
        <PeopleReader source={records} label={labels[kind]} />
        <div className="companion-timeline">
          {records.items.map((row) => {
            const instant = recordInstant(row);
            const summary = readableText(row.summary) || readableText(row.activity);
            const status = recordStatus(row, kind);
            return (
              <div key={row.id}>
                <span className="companion-timeline-dot" />
                {instant && <small><time dateTime={instant}>{userInstant(instant)}</time></small>}
                <h4>{summary || status || labels[kind]}</h4>
                {summary && status && <Badge variant="outline">{status}</Badge>}
                {kind === "presences" && (
                  <p className="companion-caption">
                    在场来自实际运行记录，历史结束记录不代表现在仍在场。
                  </p>
                )}
                {kind === "opportunities" && (
                  <p className="companion-caption">
                    是否已结识仍以真实关系记录为准。
                  </p>
                )}
                <Button
                  variant="link"
                  onClick={(event) => {
                    trigger.current = event.currentTarget;
                    setSelection({ id: row.id, kind });
                  }}
                >
                  详情
                </Button>
              </div>
            );
          })}
        </div>
        {records.ready && !records.items.length && !records.error && (
          <p className="companion-body-text">当前没有已读{labels[kind]}。</p>
        )}
        {records.cursor && (
          <Button
            variant="outline"
            disabled={records.busy}
            onClick={records.more}
          >
            加载更多{labels[kind]}
          </Button>
        )}
      </Disclosure>
      <WorkspaceDialog
        className="replica-profile replica-people"
        wide
        open={!!selection && !action}
        title={selection ? labels[selection.kind] : "人物记录"}
        description="查看已保存的事项与结果。"
        returnFocus={() => trigger.current}
        onOpenChange={(v) => {
          if (!v) setSelection(null);
        }}
        footer={
          selected &&
          selection?.kind === "opportunities" &&
          ["failed", "deferred"].includes(selected.status) && (
            <Button
              variant="outline"
              disabled={
                m.readOnly ||
                !Number.isSafeInteger(selected.revision) ||
                !Number.isFinite(Date.parse(selected.expires_at)) ||
                Date.parse(selected.expires_at) <= Date.now()
              }
              onClick={retry}
            >
              请求重新判断
            </Button>
          )
        }
      >
        <div className="companion-stack">
          {selection?.kind === "opportunities" && (
            <PeopleReader source={detail} label="相遇机会详情" />
          )}
          {selected && (
            <>
              <RecordSummary row={selected} kind={selection.kind} placeName={placeName} />
              <WorkspaceInfo>
                接触未获接受、尚待判断或仅有在场记录，都不会据此显示已经结识。
              </WorkspaceInfo>
              {selected.life_event_id && (
                <Button variant="link" asChild>
                  <a
                    href={
                      "#/c/" +
                      encodeURIComponent(m.fid) +
                        "/world/event/" +
                      encodeURIComponent(selected.life_event_id)
                    }
                  >
                    查看实际生活事件
                  </a>
                </Button>
              )}
              {selected.task_id && (
                <Button variant="link" asChild>
                  <a
                    href={"#/diag/task/" + encodeURIComponent(selected.task_id)}
                  >
                    查看原任务
                  </a>
                </Button>
              )}
              <JsonDetails value={selected} title={downloadTitles[selection.kind]} downloadOnly />
            </>
          )}
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

import { useEffect, useRef, useState } from "react";
import {
  ArrowUpRightIcon,
  CheckCircle2Icon,
  Edit3Icon,
  FlagIcon,
  PauseIcon,
  PlayIcon,
  PlusIcon,
  RefreshCwIcon,
  SearchIcon,
  XIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Progress } from "@/components/ui/progress";
import { Checkbox } from "@/components/ui/checkbox";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import {
  Surface,
  FieldInput,
  Textarea,
  ChoiceSelect,
  Badge,
  Disclosure,
} from "../controls";
import { useWorkspace } from "../page";
import WorkspacePageHeader from "../shared/WorkspacePageHeader";
import WorkspaceDialog from "../shared/WorkspaceDialog";
import WorkspaceCommandDialog from "../shared/WorkspaceCommandDialog";
import WorkspaceInfo from "../shared/WorkspaceInfo";
import LoadingState from "../shared/LoadingState";
import PageSkeleton from "../shared/PageSkeleton";
import { EmptyState, InlineFeedback,Feedback,ResultFeedback } from "../shared/Presentation";
import JsonDetails from "../shared/JsonDetails";
import { jsonValueSnapshot } from "@/state/view-model";
import {
  goalAssessment,
  goalLabels,
  goalScopeLabel,
} from "@/approved-ui/goal-presentation.mjs";
import { zonedInput, zonedInputInstant } from "@/v3/zoned-input.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";
import { errorMessage, errorRecord } from "@/v3/domain-presentation.mjs";
import { goalExecutionFacts, goalRecordStatus, goalRecordSummary, goalRecordText } from "@/approved-ui/goal-record-presentation.mjs";
const terminalStatuses = ["completed", "cancelled", "abandoned"];
const evaluationPending = ["pending", "processing", "retry", "queued"];
function GoalRecordFacts({ facts }) {
  return facts.filter(([, value]) => value).map(([label, value]) => (
    <p className="lp-muted" key={label}>{label}：{value}</p>
  ));
}
function GoalRecordDownload({ value, title }) {
  return value && Object.keys(value).length > 0
    ? <JsonDetails value={value} title={title} downloadOnly />
    : null;
}
function GoalDefinitionSummary({ outcome, motivation, criteria, deadline, deadlinePolicy, scope, status, reason, ownerProtected }) {
  const conditions = Array.isArray(criteria) ? criteria.map(goalRecordText).filter(Boolean) : [];
  return <GoalRecordFacts facts={[
    ["期望结果", goalRecordText(outcome)],
    ["动机", goalRecordText(motivation)],
    ["完成条件", conditions.join("；")],
    ["期望完成时间", deadline && userInstant(deadline)],
    ["截止策略", deadlinePolicy === "hard" ? "硬期限" : deadlinePolicy === "soft" ? "软期限" : ""],
    ["目标范围", scope && goalScopeLabel(scope)],
    ["状态", goalLabels[status] || goalRecordStatus(status)],
    ["所有者保护", typeof ownerProtected === "boolean" ? ownerProtected ? "已开启" : "未开启" : ""],
    ["操作原因", goalRecordText(reason)],
  ]}/>;
}
function latestGoalSummary(goal) {
  return goal && <GoalDefinitionSummary outcome={goal.desired_outcome} motivation={goal.motivation} criteria={goal.success_criteria}
    deadline={goal.deadline} deadlinePolicy={goal.deadline_policy} scope={goal.scope} status={goal.status} ownerProtected={goal.owner_protected} />;
}
function GoalPlanRecords({ goal }) {
  const rows = value => Array.isArray(value)
    ? value.filter(row => row && typeof row === "object" && Object.keys(row).length > 0) : [];
  const stages = rows(goal.stages), commitments = rows(goal.commitments);
  const dependencies = rows(goal.dependencies);
  const stageName = id => goalRecordText(stages.find(stage => stage.id === id)?.purpose);
  const criteria = values => {
    const texts = Array.isArray(values) ? values.map(row => goalRecordText(row?.text ?? row)).filter(Boolean) : [];
    return texts.length > 0 && <ul className="lp-copy">{texts.map((text, index) => <li key={index}>{text}</li>)}</ul>;
  };
  return <>
    {dependencies.length > 0 && (
      <section aria-label="目标前置条件">
        <h2 className="r-inline-title">先完成这些期待</h2>
        {dependencies.map(row => (
          <div className="replica-goal-record" key={row.prerequisite_id}>
            {row.desired_outcome && <p>{goalRecordText(row.desired_outcome)}</p>}
            {typeof row.satisfied === "boolean" && <span className="lp-muted">{row.satisfied
              ? "前置目标已完成" : "前置目标尚未完成"}</span>}
          </div>
        ))}
      </section>
    )}
    {stages.length > 0 && (
      <section aria-label="目标阶段">
        <h2 className="r-inline-title">阶段与推进条件</h2>
        {stages.map(row => {
          const status = goalRecordStatus(row.status);
          const prerequisites = row.dependency_ids?.map(stageName).filter(Boolean).join("、");
          return (
            <article className="replica-goal-record" key={row.id}>
              <div className="lp-row lp-between">
                <h3 className="lp-title">{goalRecordText(row.purpose) || "目标阶段"}</h3>
                {status && <Badge variant="outline">{status}</Badge>}
              </div>
              <GoalRecordFacts facts={[
                ["推进方式", goalRecordText(row.strategy)],
                ["开始依据", goalRecordText(row.entry_basis)],
                ["结束依据", goalRecordText(row.exit_basis)],
                ["前置阶段", prerequisites],
              ]}/>
              {criteria(row.criteria)}
              <GoalRecordDownload value={row} title="原始目标阶段记录（诊断）" />
            </article>
          );
        })}
      </section>
    )}
    {commitments.length > 0 && (
      <section aria-label="目标行动承诺">
        <h2 className="r-inline-title">接下来的行动承诺</h2>
        {commitments.map(row => {
          const status = goalRecordStatus(row.status);
          return (
            <article className="replica-goal-record" key={row.id}>
              <div className="lp-row lp-between">
                <h3 className="lp-title">{goalRecordText(row.expected_result) || "行动承诺"}</h3>
                {status && <Badge variant="outline">{status}</Badge>}
              </div>
              <GoalRecordFacts facts={[
                ["所属阶段", stageName(row.stage_id)],
                ["窗口开始", row.window_start && userInstant(row.window_start)],
                ["窗口结束", row.window_end && userInstant(row.window_end)],
                ["推进机会", goalRecordText(row.opportunity_condition)],
                ["当前阻碍", goalRecordText(row.blocker)],
              ]}/>
              {criteria(row.criteria)}
              <GoalRecordDownload value={row} title="原始行动承诺记录（诊断）" />
            </article>
          );
        })}
      </section>
    )}
  </>;
}
const field = (label, child, wide = false) => (
  <label className={"replica-field" + (wide ? " lp-wide" : "")}>
    <span>{label}</span>
    {child}
  </label>
);
const metric = (value) =>
  typeof value === "number" && Number.isFinite(value)
    ? new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 2 }).format(value)
    : "";
/** @param {{route:any,onGo?:(page:string)=>void}} props */
export default function GoalsPage({ route, onGo }) {
  const routeGoalId=route?.feature?.native==='goal'?route.feature.params.goalId:'';
  const w = useWorkspace(),
    c = w.current.value,
    fid = c.id,
    owner = w.state.session.actorId,
    scope = JSON.stringify([owner, fid]);
  const [filter, setFilter] = useState("active"),
    [query, setQuery] = useState(""),
    [partitions, setPartitions] = useState({
      current: { items: [], cursor: "", ready: false },
      history: { items: [], cursor: "", ready: false },
    }),
    [loading, setLoading] = useState({}),
    [errors, setErrors] = useState({});
  const [selectedId, setSelectedId] = useState(""),
    [detail, setDetail] = useState(null),
    [detailLoading, setDetailLoading] = useState(false),
    [detailError, setDetailError] = useState(""),
    [detailFailure, setDetailFailure] = useState(null),
    [action, setAction] = useState(null),
    [notice, setNotice] = useState(null);
  const [records, setRecords] = useState({
      evidence: { items: [], cursor: "", ready: false },
      history: { items: [], cursor: "", ready: false },
    }),
    [recordErrors, setRecordErrors] = useState({}),
    [recordLoading, setRecordLoading] = useState({});
  const [evaluationPointer, setEvaluationPointer] = useState(null),
    [evaluationNote, setEvaluationNote] = useState(null),
    [evaluationRefresh, setEvaluationRefresh] = useState(0);
  const mounted = useRef(false),
    epochs = useRef({}),
    detailEpoch = useRef(0),
    recordEpochs = useRef({}),
    cache = useRef(new Map()),
    stamps = useRef(new Map()),
    readSequence = useRef(0),
    trigger = useRef(null),
    retryReads = useRef({}),
    detailId = useRef("");
  const live = () => mounted.current && !!fid && (!routeGoalId||!route.fid||route.fid===fid) && w.isCurrent(owner, fid),
    readonly = ["retired", "archived", "已退休", "已归档"].includes(c.status);
  const all = [
    ...new Map(
      [...partitions.current.items, ...partitions.history.items].map((goal) => [
        goal.id,
        cache.current.get(goal.id) || goal,
      ]),
    ).values(),
  ];
  const visible = all.filter(
    (goal) =>
      (filter === "all" || goal.status === filter) &&
      [goal.desired_outcome, goal.motivation]
        .filter(Boolean)
        .join(" ")
        .toLowerCase()
        .includes(query.trim().toLowerCase()),
  );
  const selected =
    detail?.id === selectedId ? detail : cache.current.get(selectedId);
  function adopt(goal, stamp = ++readSequence.current) {
    if (!goal || typeof goal.id !== "string" || !goal.id)
      throw Error("目标身份尚未完整读到。");
    if (goal.fluctlight_id && goal.fluctlight_id !== fid)
      throw Error("目标范围不匹配，未采用这次结果。");
    const old = cache.current.get(goal.id);
    if (
      old &&
      (Number(goal.revision) < Number(old.revision) ||
        (goal.revision === old.revision &&
          stamp < (stamps.current.get(goal.id) || 0)))
    )
      return old;
    const next = { ...old, ...jsonValueSnapshot(goal) };
    cache.current.set(goal.id, next);
    stamps.current.set(goal.id, stamp);
    return next;
  }
  async function readPartition(
    kind,
    append = false,
    cursor = append ? partitions[kind].cursor : "",
  ) {
    if (!live()) return;
    const stamp = ++readSequence.current,
      token = (epochs.current[kind] || 0) + 1;
    epochs.current[kind] = token;
    setLoading((old) => ({ ...old, [kind]: true }));
    setErrors((old) => ({ ...old, [kind]: "" }));
    try {
      const page = await w.client.goals(fid, {
        history: kind === "history",
        limit: 50,
        ...(cursor ? { cursor } : {}),
      });
      if (!live() || epochs.current[kind] !== token) return;
      const items = page.items.map((goal) => adopt(goal, stamp));
      setPartitions((old) => ({
        ...old,
        [kind]: {
          items: append
            ? [
                ...old[kind].items,
                ...items.filter(
                  (goal) =>
                    !old[kind].items.some((value) => value.id === goal.id),
                ),
              ]
            : items,
          cursor: page.next_cursor || "",
          ready: true,
        },
      }));
    } catch (cause) {
      if (live() && epochs.current[kind] === token) {
        retryReads.current["list:" + kind] = () =>
          void readPartition(kind, append, cursor);
        setErrors((old) => ({
          ...old,
          [kind]: errorMessage(cause, "目标目录暂时无法读取。"),
        }));
      }
    } finally {
      if (live() && epochs.current[kind] === token)
        setLoading((old) => ({ ...old, [kind]: false }));
    }
  }
  async function readDetail(id = selectedId) {
    if (!live() || !id) return;
    const stamp = ++readSequence.current,
      token = ++detailEpoch.current;
    setDetailLoading(true);
    setDetailError("");
    setDetailFailure(null);
    try {
      const goal = await w.client.goalDetail(fid, id);
      if (goal.id !== id) throw Error("目标身份不匹配，保留已读取的内容。");
      if (!live() || token !== detailEpoch.current) return;
      const value = adopt(goal, stamp);
      if (detailId.current === id) setDetail(value);
      return value;
    } catch (cause) {
      if (live() && token === detailEpoch.current){
        setDetailError(errorMessage(cause, "目标详情尚未读到。"));
        setDetailFailure(errorRecord(cause));
      }
      throw cause;
    } finally {
      if (live() && token === detailEpoch.current) setDetailLoading(false);
    }
  }
  async function readRecords(
    kind,
    id,
    append = false,
    cursor = append ? records[kind].cursor : "",
  ) {
    if (!live() || !id) return;
    const token = (recordEpochs.current[kind] || 0) + 1;
    recordEpochs.current[kind] = token;
    setRecordLoading((old) => ({ ...old, [kind]: true }));
    setRecordErrors((old) => ({ ...old, [kind]: "" }));
    try {
      const page = await (kind === "evidence"
        ? w.client.goalEvidence(fid, id, cursor)
        : w.client.goalHistory(fid, id, cursor));
      if (
        !live() ||
        detailId.current !== id ||
        recordEpochs.current[kind] !== token
      )
        return;
      if (!Array.isArray(page?.items) || page.items.some(item => !item || typeof item !== 'object' || Array.isArray(item)))
        throw Error('目标记录尚未完整读到，请重新读取。');
      setRecords((old) => ({
        ...old,
        [kind]: {
          items: append
            ? [
                ...old[kind].items,
                ...page.items.filter(
                  (item) =>
                    !item.id ||
                    !old[kind].items.some((value) => value.id === item.id),
                ),
              ]
            : page.items,
          cursor: page.next_cursor || "",
          ready: true,
        },
      }));
    } catch (cause) {
      if (
        live() &&
        detailId.current === id &&
        recordEpochs.current[kind] === token
      ) {
        retryReads.current["record:" + kind] = () =>
          void readRecords(kind, id, append, cursor);
        setRecordErrors((old) => ({ ...old, [kind]: errorMessage(cause) }));
      }
    } finally {
      if (
        live() &&
        detailId.current === id &&
        recordEpochs.current[kind] === token
      )
        setRecordLoading((old) => ({ ...old, [kind]: false }));
    }
  }
  async function refreshSelectedRecords(id) {
    if (!live() || detailId.current !== id) return;
    await Promise.all([readRecords("evidence", id), readRecords("history", id)]);
  }
  useEffect(() => {
    mounted.current = true;
    void readPartition("current");
    void readPartition("history");
    return () => {
      mounted.current = false;
      detailEpoch.current++;
    };
  }, [scope]);
  useEffect(()=>{
    if(!routeGoalId||!live())return;
    openGoal({id:routeGoalId},{currentTarget:document.querySelector('.replica-goals .replica-page-heading h1')},true);
  },[scope,routeGoalId]);
  function openGoal(goal, event, unread=false) {
    trigger.current = event?.currentTarget || document.activeElement;
    detailId.current = goal.id;
    setSelectedId(goal.id);
    setDetail(cache.current.get(goal.id) || (unread?null:goal));
    setRecords({
      evidence: { items: [], cursor: "", ready: false },
      history: { items: [], cursor: "", ready: false },
    });
    setRecordErrors({});
    setRecordLoading({});
    void readDetail(goal.id).catch(() => {});
    void readRecords("evidence", goal.id);
    void readRecords("history", goal.id);
  }
  function closeDetail() {
    detailId.current = "";
    setSelectedId("");
    setDetail(null);
    detailEpoch.current++;
    setRecordErrors({});
    setRecordLoading({});
    if(routeGoalId)w.navigatePath('/c/'+encodeURIComponent(fid)+'/life/goals');
  }
  function saveEvaluationPointer(result) {
    const value = {
      goalId: result.goal_id,
      requestId: result.evaluation_request_id,
    };
    if (typeof value.requestId !== "string" || !value.requestId)
      throw Error("核对回执尚未完整读到，原提交已保留。");
    if (w.state.session?.actorId === owner)
      sessionStorage.setItem(
        "fluctlight-goals:" + owner + ":" + fid + ":evaluation",
        JSON.stringify(value),
      );
    if (live()) setEvaluationPointer(value);
  }
  useEffect(() => {
    setEvaluationNote(null);
    setEvaluationPointer(null);
    try {
      const saved = JSON.parse(
        sessionStorage.getItem(
          "fluctlight-goals:" + owner + ":" + fid + ":evaluation",
        ) || "null",
      );
      if (saved?.goalId && saved?.requestId) setEvaluationPointer(saved);
    } catch {}
  }, [scope]);
  useEffect(() => {
    if (!evaluationPointer) return;
    let active = true,
      timer,
      reading = false,
      count = 0;
    const note=(kind,text,failure)=>setEvaluationNote({kind,text,failure,...evaluationPointer});
    async function read() {
      if (!active || !live() || document.hidden || reading) return;
      reading = true;
      try {
        const goal = await w.client.goalDetail(fid, evaluationPointer.goalId);
        if (!active || !live()) return;
        if (goal.id !== evaluationPointer.goalId)
          throw Error("评估目标身份不匹配，原核对身份仍保留。");
        const value = adopt(goal);
        if (detailId.current === goal.id) setDetail(value);
        const request = goal.execution?.evaluation_request;
        if (request?.request_id === evaluationPointer.requestId) {
          if(["succeeded", "completed"].includes(request.status)){
            setEvaluationNote(null);
            setNotice({kind:'information',text:'本次核对已结束，目标完成状态与条件证据以实际记录为准。'});
          }else if(request.status==='failed')note('action-required','本次进展核对未完成，可在原目标中查看评估记录。');
          else if(['superseded','cancelled'].includes(request.status))setEvaluationNote(null);
          else note('progress','正在跟进原进展核对请求，尚未确认目标完成。');
          if (
            [
              "succeeded",
              "completed",
              "failed",
              "superseded",
              "cancelled",
            ].includes(request.status)
          ) {
            if (w.state.session?.actorId === owner)
              sessionStorage.removeItem(
                "fluctlight-goals:" + owner + ":" + fid + ":evaluation",
              );
            setEvaluationPointer(null);
            void readPartition("current");
            void readPartition("history");
            void refreshSelectedRecords(goal.id);
            return;
          }
        } else
          note('warning','原核对请求仍保留，当前状态尚未读全。可重新读取或查看原目标。');
      } catch (cause) {
        if (active && live())
          note('warning','进展状态暂时无法读取，原核对请求仍保留。',cause);
      } finally {
        reading = false;
        if (active && live() && !document.hidden && ++count < 100)
          timer = setTimeout(read, 3000);
        else if (active && live() && count >= 100)
          note('action-required','自动跟进已暂停，原核对请求仍保留。可以刷新状态继续查看。');
      }
    }
    const visible = () => {
      clearTimeout(timer);
      if (!document.hidden) void read();
    };
    document.addEventListener("visibilitychange", visible);
    void read();
    return () => {
      active = false;
      clearTimeout(timer);
      document.removeEventListener("visibilitychange", visible);
    };
  }, [evaluationPointer?.requestId, scope, evaluationRefresh]);
  function editor(goal = null) {
    if (!live() || readonly) return;
    trigger.current = document.activeElement;
    let acknowledgedId = goal?.id || null;
    const initial = {
      goalId: goal?.id || null,
      revision: goal?.revision || 0,
      title: goal?.desired_outcome || "",
      motivation: goal?.motivation || "",
      criteria: (goal?.success_criteria || []).join("\n"),
      deadlineLocal: zonedInput(goal?.deadline, "Asia/Shanghai"),
      originalDeadline: goal?.deadline || "",
      deadlinePolicy: goal?.deadline_policy || "soft",
      scope: goal?.scope || "general",
      profileId: goal?.profile_id || "",
      targetActorId: goal?.target_actor_id || "",
      ownerProtected: !!goal?.owner_protected,
      candidateOnly: goal?.status === "candidate",
      reason: "",
    };
    setAction({
      identity: "goal-editor:" + fid + ":" + (goal?.id || "new"),
      scope: ["goal-editor", fid, goal?.id || "new"],
      storageKey:
        "fluctlight-goals:" +
        owner +
        ":" +
        fid +
        ":edit:" +
        (goal?.id || "new"),
      initial,
      title: goal ? "编辑目标" : "写下一个新的期待",
      submitLabel: "保存目标",
      description: "保存目标定义；完成条件由后续实际证据评估。",
      pendingSummary: ({body}) => body && <GoalDefinitionSummary outcome={body.desiredOutcome} motivation={body.motivation}
        criteria={body.successCriteria} deadline={body.deadline} deadlinePolicy={body.deadlinePolicy} scope={body.scope}
        status={body.candidateOnly ? "candidate" : ""} reason={body.reason} ownerProtected={body.ownerProtected} />,
      reviewSummary: latestGoalSummary,
      validate: (d) => {
        if (
          !d.title.trim() ||
          !d.motivation.trim() ||
          !d.criteria.split("\n").some((value) => value.trim())
        )
          throw Error("请填写想做什么、动机和至少一项完成条件。");
        if (!d.reason.trim() || [...d.reason].length > 1000)
          throw Error("请填写操作原因（最多1000个字符）。");
      },
      build: (d) => ({
        id: d.goalId,
        body: {
          expectedRevision: d.revision,
          desiredOutcome: d.title.trim(),
          motivation: d.motivation.trim(),
          successCriteria: d.criteria
            .split("\n")
            .map((value) => value.trim())
            .filter(Boolean),
          deadlinePolicy: d.deadlinePolicy,
          ownerProtected: d.ownerProtected,
          reason: d.reason.trim(),
          ...(d.deadlineLocal
            ? {
                deadline: zonedInputInstant(
                  d.deadlineLocal,
                  "Asia/Shanghai",
                  d.originalDeadline,
                ),
              }
            : { clearDeadline: true }),
          ...(!d.goalId
            ? {
                scope: d.scope,
                candidateOnly: d.candidateOnly,
                ...(d.profileId ? { profileId: d.profileId } : {}),
                ...(d.targetActorId ? { targetActorId: d.targetActorId } : {}),
              }
            : {}),
        },
      }),
      execute: async (value, key) => {
        if (!w.isCurrent(owner, fid))
          throw Error("角色范围已经切换，未开始新的目标提交。");
        const result = value.id
          ? await w.client.goalCommand(fid, value.id, "update", {
              ...value.body,
              idempotencyKey: key,
            })
          : await w.client.createGoal(fid, {
              ...value.body,
              idempotencyKey: key,
            });
        if (
          !result?.goal_id ||
          !Number.isSafeInteger(result.revision) ||
          (value.id && result.goal_id !== value.id)
        )
          throw Error("目标回执尚未完整读到，原输入与身份已保留。");
        acknowledgedId = result.goal_id;
        return result;
      },
      readLatest: (draft) =>
        acknowledgedId || draft.goalId
          ? w.client.goalDetail(fid, acknowledgedId || draft.goalId)
          : Promise.reject(Error("原创建结果尚未确认，请先核对原提交。")),
      adoptLatest: (d, latest) => ({
        ...d,
        goalId: latest.id,
        revision: latest.revision,
      }),
      afterAcknowledged: (d, result) => ({
        ...d,
        goalId: result.goal_id,
        revision: result.revision,
      }),
      onSaved: async (result, value) => {
        acknowledgedId = result.goal_id;
        const saved = await w.client.goalDetail(fid, result.goal_id);
        if (saved.id !== result.goal_id || !Number.isSafeInteger(saved.revision) || saved.revision < result.revision)
          throw Error("保存已受理，目标版本尚未读回。");
        const body = value.body;
        const deadlineMatches = body.clearDeadline
          ? saved.deadline == null || saved.deadline === ""
          : Number.isFinite(Date.parse(saved.deadline)) && Date.parse(saved.deadline) === Date.parse(body.deadline);
        if (
          saved.revision === result.revision &&
          (saved.desired_outcome !== body.desiredOutcome ||
            saved.motivation !== body.motivation ||
            JSON.stringify(saved.success_criteria) !== JSON.stringify(body.successCriteria) ||
            saved.deadline_policy !== body.deadlinePolicy ||
            saved.owner_protected !== body.ownerProtected ||
            !deadlineMatches ||
            (!value.id && (saved.scope !== body.scope ||
              (body.profileId && saved.profile_id !== body.profileId) ||
              (body.targetActorId && saved.target_actor_id !== body.targetActorId) ||
              (!body.targetActorId && body.scope === "relationship" && saved.target_actor_id !== owner))))
        )
          throw Error("保存已受理，读回内容尚未确认一致。");
        if (!live()) return;
        const confirmed = adopt(saved);
        const alreadySelected = detailId.current === confirmed.id;
        if (!alreadySelected) openGoal(confirmed);
        else setDetail(confirmed);
        setNotice({kind:'success',text:"目标已保存并读回，完成状态仍根据实际证据确认。"});
        await Promise.all([readPartition("current"), readPartition("history"),
          ...(alreadySelected ? [refreshSelectedRecords(confirmed.id)] : []),
        ]);
      },
      fields: (d, edit) => (
        <>
          {field(
            "想做什么",
            <FieldInput
              aria-label="目标内容"
              value={d.title}
              onChange={(event) => edit({ title: event.target.value })}
            />,
            true,
          )}
          {field(
            "为什么想做",
            <Textarea
              aria-label="目标动机"
              value={d.motivation}
              onChange={(event) => edit({ motivation: event.target.value })}
            />,
            true,
          )}
          {field(
            "目标范围",
            <ChoiceSelect
              aria-label="目标范围"
              value={d.scope}
              disabled={!!d.goalId}
              onChange={(event) => edit({ scope: event.target.value })}
            >
              <option value="general">一般目标</option>
              <option value="relationship">关系目标</option>
            </ChoiceSelect>,
          )}
          {field(
            "截止策略",
            <ChoiceSelect
              aria-label="截止策略"
              value={d.deadlinePolicy}
              onChange={(event) => edit({ deadlinePolicy: event.target.value })}
            >
              <option value="soft">软期限</option>
              <option value="hard">硬期限</option>
            </ChoiceSelect>,
          )}
          {field(
            "期待完成时间（北京时间，可留空）",
            <FieldInput
              type="datetime-local"
              aria-label="目标截止时间"
              value={d.deadlineLocal}
              onChange={(event) => edit({ deadlineLocal: event.target.value })}
            />,
            true,
          )}
          {field(
            "完成条件（每行一项）",
            <Textarea
              aria-label="目标完成条件"
              value={d.criteria}
              onChange={(event) => edit({ criteria: event.target.value })}
            />,
            true,
          )}
          <div className="lp-wide">
            <Disclosure title="归属与保护">
              {!d.goalId && field(
                "所属人格标识（选填）",
                <FieldInput
                  value={d.profileId}
                  onChange={(event) => edit({ profileId: event.target.value })}
                />,
              )}
              {!d.goalId && field(
                "关联对象标识（选填）",
                <FieldInput
                  value={d.targetActorId}
                  onChange={(event) =>
                    edit({ targetActorId: event.target.value })
                  }
                />,
              )}
              <label className="lp-row">
                <Checkbox
                  checked={d.ownerProtected}
                  onCheckedChange={(checked) =>
                    edit({ ownerProtected: !!checked })
                  }
                />
                所有者保护
              </label>
              {!d.goalId && (
                <label className="lp-row">
                  <Checkbox
                    checked={d.candidateOnly}
                    onCheckedChange={(checked) =>
                      edit({ candidateOnly: !!checked })
                    }
                  />
                  先作为候选目标保存
                </label>
              )}
            </Disclosure>
          </div>
          {field(
            "操作原因",
            <Textarea
              aria-label="目标操作原因"
              value={d.reason}
              onChange={(event) => edit({ reason: event.target.value })}
            />,
            true,
          )}
        </>
      ),
    });
  }
  function command(goal, operation) {
    if (!live() || readonly) return;
    const names = {
      pause: "暂停目标",
      resume: goal.status === "candidate" ? "采用候选目标" : "继续目标",
      cancel: "取消目标",
      abandon: "放下目标",
      reassess: "核对进展",
    };
    trigger.current = document.activeElement;
    setAction({
      identity: "goal-" + operation + ":" + goal.id,
      scope: ["goal-command", fid, goal.id, operation],
      storageKey:
        "fluctlight-goals:" +
        owner +
        ":" +
        fid +
        ":" +
        goal.id +
        ":" +
        operation,
      title: names[operation],
      submitLabel: names[operation],
      initial: { reason: "", revision: goal.revision },
      pendingSummary: ({reason}) => <GoalDefinitionSummary outcome={goal.desired_outcome} reason={reason} />,
      reviewSummary: latestGoalSummary,
      description:
        operation === "reassess"
          ? "请求按实际条件和证据核对，不会直接标记目标完成。"
          : "目标、来源和历史会保留，操作以实际结果为准。",
      validate: (d) => {
        if (!d.reason.trim() || [...d.reason].length > 1000)
          throw Error("请填写操作原因（最多1000个字符）。");
      },
      build: (d) => ({ expectedRevision: d.revision, reason: d.reason.trim() }),
      execute: async (value, key) => {
        if (!w.isCurrent(owner, fid)) throw Error("目标范围已经切换。");
        const result = await w.client.goalCommand(fid, goal.id, operation, {
          ...value,
          idempotencyKey: key,
        });
        if (
          result?.goal_id !== goal.id ||
          !Number.isSafeInteger(result.revision) ||
          (operation === "reassess" && !result.evaluation_request_id)
        )
          throw Error("目标命令回执尚未完整读到，原身份已保留。");
        if (operation === "reassess") saveEvaluationPointer(result);
        return result;
      },
      readLatest: () => w.client.goalDetail(fid, goal.id),
      adoptLatest: (d, latest) => ({ ...d, revision: latest.revision }),
      onSaved: async (result) => {
        if (operation === "reassess") saveEvaluationPointer(result);
        const saved = await w.client.goalDetail(fid, goal.id);
        if (saved.id !== goal.id)
          throw Error("命令已有回执，目标身份尚未完整读回。");
        if (!Number.isSafeInteger(saved.revision) || saved.revision < result.revision ||
          (saved.revision === result.revision && Object.hasOwn(goalLabels, result.status) && saved.status !== result.status))
          throw Error("命令已有回执，目标版本或状态尚未完整读回。");
        if (!live()) return;
        const confirmed = adopt(saved);
        if (detailId.current === goal.id) setDetail(confirmed);
        if(operation!=="reassess")setNotice({kind:'success',text:"目标状态已读回。"});
        await Promise.all([readPartition("current"), readPartition("history"), refreshSelectedRecords(goal.id)]);
      },
      fields: (d, edit) =>
        field(
          "操作原因",
          <Textarea
            aria-label="目标操作原因"
            value={d.reason}
            onChange={(event) => edit({ reason: event.target.value })}
          />,
          true,
        ),
    });
  }
  const assessment = selected ? goalAssessment(selected) : null;
  const executionFacts = goalExecutionFacts(selected?.execution);
  const ready =
    filter === "completed"
      ? partitions.history.ready
      : filter === "all"
        ? partitions.current.ready && partitions.history.ready
        : partitions.current.ready;
  const anyLoading = Object.values(loading).some(Boolean),
    selectedTerminal = terminalStatuses.includes(selected?.status),
    evalRequest = selected?.execution?.evaluation_request;
  return (
    <div className="replica-life replica-goals">
      <WorkspacePageHeader
        eyebrow="LIFE / THINGS THAT MATTER"
        title={c.name + "想做的事"}
        description="一些长远的期待，一些今天就想迈出的小步。"
        actions={
          <>
            <Button disabled={readonly} onClick={() => editor()}>
              <PlusIcon size={15} />
              添加目标
            </Button>
          </>
        }
      />
      <div className="lp-stack">
        <Tabs className="lp-stack" value={filter} onValueChange={setFilter}>
          <div className="lp-toolbar">
            <TabsList variant="line" className="replica-segment-tabs">
              <TabsTrigger value="active">
                正在追寻{" "}
                <span>
                  {partitions.current.ready
                    ? all.filter((goal) => goal.status === "active").length +
                      (partitions.current.cursor ? "+" : "")
                    : "…"}
                </span>
              </TabsTrigger>
              <TabsTrigger value="paused">暂时搁置</TabsTrigger>
              <TabsTrigger value="completed">已经做到</TabsTrigger>
              <TabsTrigger value="all">全部</TabsTrigger>
            </TabsList>
            <div className="replica-search replica-goal-search" data-has-query={!!query}>
              <SearchIcon size={15} />
              <Input
                aria-label="搜索目标"
                placeholder="搜索目标…"
                value={query}
                onChange={(event) => setQuery(event.target.value)}
              />
              {query && (
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label="清除目标搜索"
                  onClick={() => setQuery("")}
                >
                  <XIcon size={13} />
                </Button>
              )}
              <Button
                variant="ghost"
                size="icon"
                aria-label="刷新目标"
                disabled={anyLoading}
                onClick={() => {
                  void readPartition("current");
                  void readPartition("history");
                }}
              >
                <RefreshCwIcon size={15} />
              </Button>
            </div>
          </div>
          <TabsContent value={filter}>
            <ResultFeedback notice={notice} onDismiss={original=>setNotice(current=>current===original?null:current)}/>
            {evaluationNote && <Feedback kind={evaluationNote.kind} failure={evaluationNote.failure} actions={<>{cache.current.has(evaluationNote.goalId)&&<Button variant="outline" onClick={event=>openGoal(cache.current.get(evaluationNote.goalId),event)}>查看原目标与核对记录</Button>}<Button variant="ghost" onClick={()=>{saveEvaluationPointer({goal_id:evaluationNote.goalId,evaluation_request_id:evaluationNote.requestId});setEvaluationRefresh(value=>value+1);}}>刷新核对状态</Button></>}>{evaluationNote.text}</Feedback>}
            {Object.entries(errors)
              .filter(([, value]) => value)
              .map(([kind, value]) => (
                <InlineFeedback kind={partitions[kind].ready?'warning':'error'} key={kind}>
                  <p>{value}</p>
                  <Button
                    variant="outline"
                    disabled={loading[kind]}
                    onClick={() => retryReads.current["list:" + kind]?.()}
                  >
                    重新读取{kind === "history" ? "历史目标" : "当前目标"}
                  </Button>
                </InlineFeedback>
              ))}
            {anyLoading && (ready?<LoadingState label="正在读取目标…" />:<PageSkeleton contentOnly label="正在读取目标…"/>)}
            <div className="lp-three" data-testid="goal-grid">
              {visible.map((goal) => {
                const a = goalAssessment(goal);
                return (
                  <Surface className="lp-goal-card" key={goal.id}>
                    <Button
                      variant="ghost"
                      className="replica-goal-open"
                      aria-label={"查看目标 " + goal.desired_outcome}
                      onClick={(event) => openGoal(goal, event)}
                    >
                      <div className="lp-row lp-between">
                        <span className="lp-chip">
                          <FlagIcon size={11} />
                          {goalScopeLabel(goal.scope)}
                        </span>
                        <Badge
                          variant="outline"
                          className="lp-status"
                          data-status={
                            goal.status === "active" ? "pending" : goal.status
                          }
                        >
                          {goalLabels[goal.status] || goalRecordStatus(goal.status)}
                        </Badge>
                      </div>
                      <h3>{goal.desired_outcome}</h3>
                      {goal.motivation && goal.motivation !== goal.desired_outcome && <p>{goal.motivation}</p>}
                      <Progress
                        value={a.progress === null ? null : a.progress * 100}
                        className="lp-progress"
                        aria-label="证据评估进度"
                      />
                      <div className="lp-row lp-between">
                        <span className="lp-muted">
                          {a.verifiedCount === null
                            ? `条件评估待读取 · 共 ${a.criteria.length} 项条件`
                            : `已核实 ${a.verifiedCount} / ${a.criteria.length} 项条件`}
                        </span>
                        {goal.deadline && <span className="lp-muted">{userInstant(goal.deadline)}</span>}
                      </div>
                    </Button>
                  </Surface>
                );
              })}
            </div>
            {!visible.length &&
              !anyLoading &&
              ready &&
              !Object.values(errors).some(Boolean) && (
                <EmptyState variant="page">
                  <h3>这一页暂时没有目标</h3>
                  <p>换一个筛选，或写下一个新的期待。</p>
                </EmptyState>
              )}
            {Object.values(partitions).some((value) => value.cursor) && (
              <div className="lp-action-row">
                {Object.entries(partitions)
                  .filter(
                    ([kind, value]) =>
                      value.cursor &&
                      (filter === "all" ||
                        (filter === "completed"
                          ? kind === "history"
                          : kind === "current")),
                  )
                  .map(([kind]) => (
                    <Button
                      key={kind}
                      variant="outline"
                      disabled={loading[kind]}
                      onClick={() => void readPartition(kind, true)}
                    >
                      加载更多{kind === "history" ? "历史目标" : "当前目标"}
                    </Button>
                  ))}
              </div>
            )}
          </TabsContent>
        </Tabs>
        <Surface className="lp-pad">
          <div className="lp-row lp-between">
            <div>
              <h3 className="lp-title">生活会继续，期待也是。</h3>
              <p className="lp-muted">
                完成状态根据实际经历核对，目标也会在相处中自然生长。
              </p>
            </div>
            <div className="lp-action-row">
              <Button variant="link" asChild>
                <a href={"#/c/" + encodeURIComponent(fid) + "/life/planning"}>
                  独立目标规划与治理
                </a>
              </Button>
              <Button variant="outline" onClick={() => onGo?onGo("聊天"):w.navigatePath?.("/c/"+encodeURIComponent(fid)+"/chat")}>
                和{c.name}聊聊最近的想法
                <ArrowUpRightIcon size={14} />
              </Button>
            </div>
          </div>
        </Surface>
      </div>
      <WorkspaceDialog
        open={!!selectedId && !action}
        returnFocus={() => trigger.current}
        onOpenChange={(open) => {
          if (!open) closeDetail();
        }}
        title={selected?.desired_outcome || "目标详情"}
        description={<span className="inline-flex flex-wrap items-center gap-2"><span>{"属于"+c.name+"的一个期待"}</span>{selected&&<Badge variant="outline">{goalLabels[selected.status]||"状态待确认"}</Badge>}</span>}
        footer={
          selected && !selectedTerminal ? (
            <>
              <Button
                variant="outline"
                disabled={readonly || detailLoading}
                onClick={() => editor(selected)}
              >
                <Edit3Icon size={14} />
                编辑
              </Button>
              {selected.status === "active" ? (
                <Button
                  variant="outline"
                  disabled={readonly}
                  onClick={() => command(selected, "pause")}
                >
                  <PauseIcon size={14} />
                  暂停
                </Button>
              ) : (
                ["paused", "candidate"].includes(selected.status) && (
                  <Button
                    variant="outline"
                    disabled={readonly}
                    onClick={() => command(selected, "resume")}
                  >
                    <PlayIcon size={14} />
                    {selected.status === "candidate" ? "采用候选" : "继续"}
                  </Button>
                )
              )}
              <Button
                disabled={
                  readonly ||
                  detailLoading ||
                  !["active", "paused"].includes(selected.status) ||
                  evaluationPending.includes(evalRequest?.status)
                }
                onClick={() => command(selected, "reassess")}
              >
                <RefreshCwIcon size={14} />
                核对进展
              </Button>
            </>
          ) : undefined
        }
      >
        {routeGoalId&&<nav className="ui-actions" aria-label="目标上下文"><Button asChild variant="link"><a href={'#/c/'+encodeURIComponent(fid)+'/life/goals'}>目标列表</a></Button><Button asChild variant="link"><a href={'#/c/'+encodeURIComponent(fid)+'/life/planning'}>独立规划</a></Button></nav>}
        {detailLoading && <LoadingState label="正在读取目标详情…" />}
        {detailError && (
          <InlineFeedback role="alert">
            <p>{detailError}</p>
            <Button
              variant="outline"
              disabled={detailLoading}
              onClick={() => void readDetail().catch(() => {})}
            >
              重新读取目标详情
            </Button>
            <GoalRecordDownload value={detailFailure} title="原始目标读取错误记录（诊断）"/>
          </InlineFeedback>
        )}
        {selected && (
          <div className="lp-stack">
            {selected.motivation && <p className="lp-copy">{selected.motivation}</p>}
            <div className="lp-row">
              <span className="lp-chip">{goalScopeLabel(selected.scope)}</span>
              {metric(selected.importance) && <span className="lp-chip">重要程度 · {metric(selected.importance)}</span>}
              {metric(selected.urgency) && <span className="lp-chip">紧迫程度 · {metric(selected.urgency)}</span>}
              {selected.deadline && (
                <span className="lp-chip">
                  期待在 {userInstant(selected.deadline)} 前完成
                </span>
              )}
            </div>
            {assessment.criteria.length > 0 && <section>
              <div className="replica-section-heading">
                <h2>怎样算做到？</h2>
              </div>
              <div className="lp-check-list">
                {assessment.criteria.map((item, index) => (
                  <div className="lp-check-line" key={item.id || index}>
                    {item.verdict === "satisfied" ? (
                      <CheckCircle2Icon size={16} />
                    ) : (
                      <span className="replica-criterion-dot" />
                    )}
                    <div>
                      {item.text}
                      <div className="lp-muted">
                        {item.verdict === "satisfied"
                          ? "已由当前版本评估确认"
                          : item.verdict === "not_satisfied"
                            ? "本次评估尚未满足"
                            : "等待实际证据核对"}
                      </div>
                      {goalRecordText(item.reason) && <p className="lp-muted">{goalRecordText(item.reason)}</p>}
                    </div>
                  </div>
                ))}
              </div>
            </section>}
            {assessment.progress!==null?<WorkspaceInfo>当前证据评估进度：{Math.round(assessment.progress*100)}%。目标完成状态以实际领域记录为准。</WorkspaceInfo>:<p className="lp-muted">尚无当前版本的完成条件评估。</p>}
            <GoalPlanRecords goal={selected}/>
            {(evaluationPointer?.goalId === selected.id ||
              evalRequest?.request_id) && (
              <Button
                variant="link"
                onClick={() => {
                  if (
                    evaluationPointer?.goalId !== selected.id &&
                    evalRequest?.request_id
                  )
                    saveEvaluationPointer({
                      goal_id: selected.id,
                      evaluation_request_id: evalRequest.request_id,
                    });
                  setEvaluationRefresh((value) => value + 1);
                }}
              >
                刷新核对状态
              </Button>
            )}
            {executionFacts.length > 0 && (
              <Disclosure title="推进条件与核对状态">
                <GoalRecordFacts facts={executionFacts} />
              </Disclosure>
            )}
            {["evidence", "history"].map((kind) => (
              records[kind].ready&&!recordLoading[kind]&&!recordErrors[kind]&&!records[kind].items.length&&!records[kind].cursor
                ? <p key={kind} className="lp-muted">{kind==="evidence"?"尚无证据来源。":"尚无目标修订历史。"}</p>
                : <Disclosure
                key={kind}
                title={kind === "evidence" ? "证据来源" : "目标修订历史"}
              >
                {recordLoading[kind] && <LoadingState label="正在读取记录…" />}
                {recordErrors[kind] && (
                  <InlineFeedback role="alert">
                    <p>{recordErrors[kind]}</p>
                    <Button
                      variant="outline"
                      onClick={() => retryReads.current["record:" + kind]?.()}
                    >
                      重新读取{kind === "evidence" ? "证据" : "修订"}
                    </Button>
                  </InlineFeedback>
                )}
                {records[kind].items.map((record, index) => {
                  if (!Object.keys(record).length) return null;
                  const {title, status, facts} = goalRecordSummary(record, kind);
                  return (
                    <div className="replica-goal-record" key={record.id || index}>
                      <p>{title}</p>
                      {status && <Badge variant="outline">{status}</Badge>}
                      <GoalRecordFacts facts={facts} />
                      <GoalRecordDownload value={record} title={kind === "evidence" ? "原始目标证据记录（诊断）" : "原始目标修订记录（诊断）"} />
                    </div>
                  );
                })}
                {records[kind].cursor && (
                  <Button
                    variant="outline"
                    disabled={recordLoading[kind]}
                    onClick={() => void readRecords(kind, selected.id, true)}
                  >
                    加载更多{kind === "evidence" ? "证据" : "修订"}
                  </Button>
                )}
              </Disclosure>
            ))}
            <GoalRecordDownload value={selected} title="原始目标与推进记录（诊断）" />
            {!selectedTerminal && (
              <div className="lp-action-row">
                <Button
                  variant="outline"
                  disabled={readonly}
                  onClick={() => command(selected, "abandon")}
                >
                  放下这个目标
                  <ArrowUpRightIcon size={12} />
                </Button>
                <Button
                  variant="outline"
                  disabled={readonly}
                  onClick={() => command(selected, "cancel")}
                >
                  取消这个目标
                </Button>
              </div>
            )}
          </div>
        )}
      </WorkspaceDialog>
      {action && (
        <WorkspaceCommandDialog
          action={action}
          onClose={() => setAction(null)}
          returnFocus={() => trigger.current}
        />
      )}
    </div>
  );
}

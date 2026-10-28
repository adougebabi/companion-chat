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
import { EmptyState, InlineFeedback } from "../shared/Presentation";
import JsonDetails from "../shared/JsonDetails";
import { jsonValueSnapshot } from "@/state/view-model";
import {
  goalAssessment,
  goalLabels,
  goalScopeLabel,
} from "@/approved-ui/goal-presentation.mjs";
import { zonedInput, zonedInputInstant } from "@/v3/zoned-input.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";
import { errorMessage, stateLabel } from "@/v3/domain-presentation.mjs";
const terminalStatuses = ["completed", "cancelled", "abandoned"];
const evaluationPending = ["pending", "processing", "retry", "queued"];
function GoalPlanRecords({ goal }) {
  const rows = value => Array.isArray(value)
    ? value.filter(row => row && typeof row === "object") : [];
  const stages = rows(goal.stages), commitments = rows(goal.commitments);
  const dependencies = rows(goal.dependencies);
  const stageName = id => stages.find(stage => stage.id === id)?.purpose
    || "阶段未在本次记录中读到";
  const criteria = values => Array.isArray(values) && values.length > 0 && (
    <ul className="lp-copy">{values.map((row, index) => (
      <li key={row?.id || index}>
        {typeof row === "string" ? row : row?.text || "完成条件尚未完整读到"}
      </li>
    ))}</ul>
  );
  return <>
    {dependencies.length > 0 && (
      <section aria-label="目标前置条件">
        <h2 className="r-inline-title">先完成这些期待</h2>
        {dependencies.map(row => (
          <div className="replica-goal-record" key={row.prerequisite_id}>
            <p>{row.desired_outcome}</p>
            <span className="lp-muted">{row.satisfied === true
              ? "前置目标已完成" : "前置目标尚未完成"}</span>
          </div>
        ))}
      </section>
    )}
    {stages.length > 0 && (
      <section aria-label="目标阶段">
        <h2 className="r-inline-title">阶段与推进条件</h2>
        {stages.map(row => (
          <article className="replica-goal-record" key={row.id}>
            <div className="lp-row lp-between">
              <h3 className="lp-title">{row.purpose || "已记录的阶段"}</h3>
              <Badge variant="outline">{goalLabels[row.status] || stateLabel(row.status)}</Badge>
            </div>
            {row.strategy && <p className="lp-copy">{row.strategy}</p>}
            {row.entry_basis && <p className="lp-small">开始依据：{row.entry_basis}</p>}
            {row.exit_basis && <p className="lp-small">结束依据：{row.exit_basis}</p>}
            {criteria(row.criteria)}
            {row.dependency_ids?.length > 0 && (
              <p className="lp-muted">前置阶段：{row.dependency_ids.map(stageName).join("、")}</p>
            )}
            <JsonDetails value={row} title="阶段版本与完整来源"/>
          </article>
        ))}
      </section>
    )}
    {commitments.length > 0 && (
      <section aria-label="目标行动承诺">
        <h2 className="r-inline-title">接下来的行动承诺</h2>
        {commitments.map(row => (
          <article className="replica-goal-record" key={row.id}>
            <div className="lp-row lp-between">
              <h3 className="lp-title">{row.expected_result || "已记录的行动承诺"}</h3>
              <Badge variant="outline">{goalLabels[row.status] || stateLabel(row.status)}</Badge>
            </div>
            {row.stage_id && <p className="lp-muted">所属阶段：{stageName(row.stage_id)}</p>}
            {(row.window_start || row.window_end) && <p className="lp-small">
              时间窗口：{row.window_start ? userInstant(row.window_start) : "开始时间未限定"}
              {" — "}{row.window_end ? userInstant(row.window_end) : "结束时间未限定"}
            </p>}
            {row.opportunity_condition && <p className="lp-small">推进机会：{row.opportunity_condition}</p>}
            {row.blocker && <p className="lp-small">当前阻碍：{row.blocker}</p>}
            {criteria(row.criteria)}
            <JsonDetails value={row} title="承诺版本与完整来源"/>
          </article>
        ))}
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
    : "尚未提供";
export default function GoalsPage({ route, onGo }) {
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
    [action, setAction] = useState(null),
    [notice, setNotice] = useState("");
  const [records, setRecords] = useState({
      evidence: { items: [], cursor: "", ready: false },
      history: { items: [], cursor: "", ready: false },
    }),
    [recordErrors, setRecordErrors] = useState({}),
    [recordLoading, setRecordLoading] = useState({});
  const [evaluationPointer, setEvaluationPointer] = useState(null),
    [evaluationNote, setEvaluationNote] = useState(""),
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
  const live = () => mounted.current && !!fid && w.isCurrent(owner, fid),
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
    try {
      const goal = await w.client.goalDetail(fid, id);
      if (goal.id !== id) throw Error("目标身份不匹配，保留已读取的内容。");
      if (!live() || token !== detailEpoch.current) return;
      const value = adopt(goal, stamp);
      if (detailId.current === id) setDetail(value);
      return value;
    } catch (cause) {
      if (live() && token === detailEpoch.current)
        setDetailError(errorMessage(cause, "目标详情尚未读到。"));
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
  function openGoal(goal, event) {
    trigger.current = event?.currentTarget || document.activeElement;
    detailId.current = goal.id;
    setSelectedId(goal.id);
    setDetail(cache.current.get(goal.id) || goal);
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
          setEvaluationNote(
            ["succeeded", "completed"].includes(request.status)
              ? "本次核对已结束，完成状态与条件证据以下方真实记录为准。"
              : request.status === "failed"
                ? "本次核对未完成，请查看原目标和评估记录。"
                : "正在跟进原进展核对请求。",
          );
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
          setEvaluationNote(
            "已保留原核对身份，当前读取未返回该请求的完整状态，请核对目标评估与修订记录。",
          );
      } catch (cause) {
        if (active && live())
          setEvaluationNote(
            "进展读取未完成，保留原核对身份。" + errorMessage(cause),
          );
      } finally {
        reading = false;
        if (active && live() && !document.hidden && ++count < 100)
          timer = setTimeout(read, 3000);
        else if (active && live() && count >= 100)
          setEvaluationNote(
            "自动跟进已暂停，原核对身份仍保留，可以刷新核对状态继续。",
          );
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
        setNotice("目标已保存并读回，完成状态仍根据实际证据确认。");
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
              {field(
                "人格 ID（新建可选）",
                <FieldInput
                  value={d.profileId}
                  disabled={!!d.goalId}
                  onChange={(event) => edit({ profileId: event.target.value })}
                />,
              )}
              {field(
                "关联 Actor ID（新建可选）",
                <FieldInput
                  value={d.targetActorId}
                  disabled={!!d.goalId}
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
        setNotice(
          operation === "reassess"
            ? "进展核对请求已提交，正在跟进真实评估结果。"
            : "目标状态已读回。",
        );
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
            {notice && (
              <div className="lp-notice lp-row lp-between">
                <span>{notice}</span>
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label="关闭提示"
                  onClick={() => setNotice("")}
                >
                  <XIcon size={13} />
                </Button>
              </div>
            )}
            {evaluationNote && <WorkspaceInfo>{evaluationNote}</WorkspaceInfo>}
            {Object.entries(errors)
              .filter(([, value]) => value)
              .map(([kind, value]) => (
                <InlineFeedback role="alert" key={kind}>
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
            {anyLoading && <LoadingState label="正在读取目标…" />}
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
                          {goalLabels[goal.status] || goal.status}
                        </Badge>
                      </div>
                      <h3>{goal.desired_outcome}</h3>
                      <p>{goal.motivation || "动机尚未提供"}</p>
                      <Progress
                        value={a.progress === null ? null : a.progress * 100}
                        className="lp-progress"
                        aria-label="证据评估进度"
                      />
                      <div className="lp-row lp-between">
                        <span className="lp-muted">
                          {a.verifiedCount === null
                            ? "条件评估待读取"
                            : `已核实 ${a.verifiedCount} / ${a.criteria.length} 项条件`}
                        </span>
                        <span className="lp-muted">
                          {goal.deadline
                            ? userInstant(goal.deadline)
                            : "慢慢来，不设期限"}
                        </span>
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
                <EmptyState>
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
              <Button variant="outline" onClick={() => onGo?.("聊天")}>
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
        description={"属于" + c.name + "的一个期待"}
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
          </InlineFeedback>
        )}
        {selected && (
          <div className="lp-stack">
            <p className="lp-copy">{selected.motivation}</p>
            <div className="lp-row">
              <span className="lp-chip">{goalScopeLabel(selected.scope)}</span>
              <span className="lp-chip">
                重要程度 · {metric(selected.importance)}
              </span>
              <span className="lp-chip">
                紧迫程度 · {metric(selected.urgency)}
              </span>
              {selected.deadline && (
                <span className="lp-chip">
                  期待在 {userInstant(selected.deadline)} 前完成
                </span>
              )}
            </div>
            <section>
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
                      {item.reason && (
                        <p className="lp-muted">
                          {item.reason === "no applicable evidence"
                            ? "尚无适用证据"
                            : item.reason}
                        </p>
                      )}
                    </div>
                  </div>
                ))}
              </div>
              {!assessment.criteria.length && (
                <p className="lp-muted">
                  完成条件尚未完整读到，请查看原目标记录。
                </p>
              )}
            </section>
            <WorkspaceInfo>
              {assessment.progress === null
                ? "当前未提供证据评估进度。"
                : "当前证据评估进度：" +
                  Math.round(assessment.progress * 100) +
                  "%。"}{" "}
              目标完成状态以实际领域记录为准。
            </WorkspaceInfo>
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
            {selected.execution && (
              <Disclosure title="推进条件与核对状态">
                <JsonDetails value={selected.execution} title="实际推进记录" />
              </Disclosure>
            )}
            {["evidence", "history"].map((kind) => (
              <Disclosure
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
                {records[kind].items.map((record, index) => (
                  <div className="replica-goal-record" key={record.id || index}>
                    <p>
                      {typeof record.reason === "string"
                        ? record.reason
                        : typeof record.source?.summary === "string"
                          ? record.source.summary
                          : {
                              create: "创建目标",
                              update: "修改目标",
                              pause: "暂停目标",
                              resume: "继续目标",
                              abandon: "放下目标",
                              cancel: "取消目标",
                            }[record.operation] || "目标记录"}
                    </p>
                    <p className="lp-muted">
                      {userInstant(
                        record.created_at || record.source?.recorded_at,
                      )}
                    </p>
                    <JsonDetails value={record} title="完整记录与来源" />
                  </div>
                ))}
                {records[kind].ready &&
                  !records[kind].items.length &&
                  !recordErrors[kind] && (
                    <p className="lp-muted">当前读取范围没有记录。</p>
                  )}
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
            <JsonDetails value={selected} title="完整目标与来源" />
            {!selectedTerminal && (
              <div className="lp-action-row">
                <Button
                  variant="link"
                  disabled={readonly}
                  onClick={() => command(selected, "abandon")}
                >
                  放下这个目标
                  <ArrowUpRightIcon size={12} />
                </Button>
                <Button
                  variant="link"
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

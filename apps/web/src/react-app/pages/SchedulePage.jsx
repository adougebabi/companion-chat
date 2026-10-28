import { useCallback, useEffect, useRef, useState } from "react";
import {
  ArrowRightIcon,
  BookOpenIcon,
  CalendarDaysIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  Clock3Icon,
  Edit3Icon,
  LeafIcon,
  SparklesIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Badge, Disclosure, FieldInput, Surface, Textarea } from "../controls";
import { EmptyState, InlineFeedback } from "../shared/Presentation";
import LoadingState from "../shared/LoadingState";
import WorkspacePageHeader from "../shared/WorkspacePageHeader";
import WorkspaceDialog from "../shared/WorkspaceDialog";
import ScheduleTimeline from "../shared/ScheduleTimeline";
import WorkspaceInfo from "../shared/WorkspaceInfo";
import RequestStatus from "../shared/RequestStatus";
import DateField, { civilDate } from "../shared/DateField";
import { jsonValueSnapshot } from "@/state/view-model";
import { ActionFeedback } from "../components";
import { useWorkspace } from "../page";
import { day } from "@/state/workspace.mjs";
import { scheduleProjection } from "@/approved-ui/presentation.mjs";
import { zonedInput, zonedInputInstant } from "@/v3/zoned-input.mjs";
import { errorMessage } from "@/v3/domain-presentation.mjs";
import { requestDecisionText } from "@/v3/proto/components/request-failure.js";
import { userInstant } from "@/v3/presentation-time.mjs";
const tabs = ["timeline", "suggestions", "habits"];
const shiftDate = (date, step) => {
  const value = new Date(date + "T12:00:00Z");
  value.setUTCDate(value.getUTCDate() + step);
  return value.toISOString().slice(0, 10);
};
const pendingStatuses = ["pending", "processing", "deferred", "awaiting_goal"];
const requestLabels = {
  pending: "等待回应",
  processing: "正在考虑",
  accepted: "已接受，行动待证据",
  completed: "已处理",
  rejected: "暂未采纳",
  deferred: "稍后决定",
  awaiting_goal: "等待目标规划",
  cancelled: "已取消",
  failed: "处理失败",
};
function Field({ label, children, wide = false }) {
  return (
    <label className={"replica-field" + (wide ? " lp-wide" : "")}>
      <span>{label}</span>
      {children}
    </label>
  );
}
function habitEntries(detail) {
  const profiles = detail?.core_persona?.personality_system?.profiles || [],
    active = profiles.find(
      (profile) =>
        profile.id === detail?.personality_runtime?.active_profile_id,
    );
  const values = [
    ["当前人格的生活倾向", active?.habits],
    [
      "生活习惯",
      detail?.life_profile?.life_habits ??
        detail?.core_persona?.life_profile?.life_habits,
    ],
  ];
  return values.flatMap(([title, value]) =>
    typeof value === "string" && value.trim()
      ? [{ title, text: value }]
      : Array.isArray(value)
        ? value.map((item, index) => ({
            title: title + " " + (index + 1),
            text:
              typeof item === "string"
                ? item
                : typeof item?.description === "string"
                  ? item.description
                  : JSON.stringify(item),
          }))
        : [],
  );
}
export default function SchedulePage({ route, onRecoverInitial = null }) {
  const w = useWorkspace(),
    c = w.current.value,
    owner = w.state.session.actorId,
    fid = c.id,
    life = w.services.lifeClient,
    scope = JSON.stringify([owner, fid]);
  const routeQuery =
    route?.query ||
    w.state.route?.query ||
    new URLSearchParams(location.hash.split("?")[1] || "");
  const queryDate = routeQuery.get("date") || "",
    queryTab = routeQuery.get("tab") || "";
  const fallbackToday = day(new Date(), c.timezone || "Asia/Shanghai");
  const [date, setDate] = useState(() =>
      civilDate(queryDate) ? queryDate : fallbackToday,
    ),
    [tab, setTab] = useState(() =>
      tabs.includes(queryTab) ? queryTab : "timeline",
    );
  const [snapshot, setSnapshot] = useState(null),
    [loading, setLoading] = useState(false),
    [readError, setReadError] = useState("");
  const [requests, setRequests] = useState([]),
    [requestCursor, setRequestCursor] = useState(""),
    [requestLoading, setRequestLoading] = useState(false),
    [requestError, setRequestError] = useState(""),
    [requestReady, setRequestReady] = useState(false);
  const [selected, setSelected] = useState(null),
    [draft, setDraft] = useState(null),
    [formError, setFormError] = useState(""),
    [busy, setBusy] = useState(false);
  const [itemPending, setItemPending] = useState(null),
    [pendingError, setPendingError] = useState(""),
    [review, setReview] = useState(null),
    [planReview, setPlanReview] = useState(null);
  const [planOpen, setPlanOpen] = useState(false),
    [planDate, setPlanDate] = useState(date),
    [planReason, setPlanReason] = useState(""),
    [planPending, setPlanPending] = useState(null),
    [planError, setPlanError] = useState("");
  const [receipt, setReceipt] = useState(null),
    [receiptOpen, setReceiptOpen] = useState(false);
  const mounted = useRef(false),
    readEpoch = useRef(0),
    requestEpoch = useRef(0),
    cache = useRef(new Map()),
    running = useRef(false),
    requestRetry = useRef(null),
    trigger = useRef(null),
    planZone = useRef(""),
    planDrafts = useRef(new Map()),
    planReviewTrigger = useRef(null);
  const live = useCallback(
    () => mounted.current && !!fid && w.isCurrent(owner, fid),
    [w, owner, fid],
  );
  const current = snapshot?.date === date ? snapshot : null,
    calendar = current?.calendar,
    detail = current?.detail,
    schedule =
      calendar?.days?.find((item) => item.local_date === date)?.schedule ??
      (calendar?.unavailable ? calendar.days[0]?.schedule : null);
  const actualDate = calendar?.unavailable ? calendar.from : date,
    today = calendar?.today || fallbackToday;
  const readonly = [detail?.status, c.status].some((status) =>
    ["retired", "archived", "已退休", "已归档"].includes(status),
  );
  const rows = (schedule?.items || []).map((item) =>
    scheduleProjection(item, schedule, w.clock),
  );
  const suggestions = requests.filter(
      (request) => request.kind === "schedule_item_suggestion",
    ),
    pending = suggestions.filter((request) =>
      pendingStatuses.includes(request.status),
    );
  const storageKey = "fluctlight:schedule-receipt:" + scope;
  function readPending() {
    if (!live()) return;
    try {
      setItemPending(life.scheduleItemPending(fid));
      setPendingError("");
    } catch (cause) {
      setItemPending(null);
      setPendingError(errorMessage(cause));
    }
  }
  function readPlanPending(target) {
    try {
      const value = life.schedulePending(fid, target);
      if (live()) setPlanPending(value);
      return value;
    } catch (cause) {
      if (live()) {
        setPlanError(errorMessage(cause));
        setPlanPending({ unreadable: true });
      }
      return { unreadable: true };
    }
  }
  async function readCalendar(target = date) {
    const token = ++readEpoch.current;
    if (!live()) return;
    setLoading(true);
    setReadError("");
    if (cache.current.has(target)) setSnapshot(cache.current.get(target));
    try {
      const detailRead = w.detail();
      const calendarRead = life
        .schedules(fid, { from: target, to: target })
        .catch(async (cause) => {
          if (![404, 405].includes(cause?.status)) throw cause;
          const value = await detailRead,
            knownDate = value.schedule?.local_date || value.context?.local_date;
          if (!knownDate) throw cause;
          return {
            unavailable: true,
            timezone:
              value.schedule?.timezone ||
              value.identity?.timezone ||
              "Asia/Shanghai",
            today: knownDate,
            from: knownDate,
            to: knownDate,
            context_revision: value.context?.context_revision,
            days: [
              {
                local_date: knownDate,
                schedule: value.schedule || null,
                request: null,
              },
            ],
          };
        });
      const [value, page] = await Promise.all([detailRead, calendarRead]);
      if (!live() || token !== readEpoch.current) return;
      if (
        !page.unavailable &&
        !page.days?.some((item) => item.local_date === target)
      )
        throw Error("服务未返回所选日期，未把其他日期当作这一天。");
      if (!value || (value.id && value.id !== fid))
        throw Error("角色资料范围尚未确认，保留已读取的日程。");
      new Intl.DateTimeFormat("zh-CN", { timeZone: page.timezone }).format(
        new Date(),
      );
      for (const dateRow of page.days || []) {
        const record = dateRow.schedule;
        if (
          record &&
          (!Array.isArray(record.items) ||
            record.items.some(
              (item) =>
                !item ||
                typeof item.id !== "string" ||
                typeof item.start_at !== "string" ||
                typeof item.end_at !== "string",
            ))
        )
          throw Error("日程条目尚未完整读到，保留上次已读取的安排。");
      }
      const next = {
        date: target,
        detail: jsonValueSnapshot(value),
        calendar: jsonValueSnapshot(page),
      };
      cache.current.set(target, next);
      setSnapshot(next);
    } catch (cause) {
      if (live() && token === readEpoch.current)
        setReadError(errorMessage(cause, "这一天的日程暂时无法读取。"));
    } finally {
      if (live() && token === readEpoch.current) setLoading(false);
    }
  }
  async function readRequests(
    append = false,
    originalCursor = append ? requestCursor : "",
  ) {
    const token = ++requestEpoch.current;
    if (!live()) return;
    setRequestLoading(true);
    setRequestError("");
    try {
      const page = await life.requests(fid, originalCursor || undefined);
      if (!live() || token !== requestEpoch.current) return;
      setRequests((old) =>
        append
          ? [
              ...old,
              ...page.items.filter(
                (item) => !old.some((value) => value.id === item.id),
              ),
            ]
          : page.items,
      );
      setRequestCursor(page.next_cursor || "");
      setRequestReady(true);
    } catch (cause) {
      if (live() && token === requestEpoch.current) {
        requestRetry.current = () => void readRequests(append, originalCursor);
        setRequestError(errorMessage(cause, "日程建议暂时无法读取。"));
      }
    } finally {
      if (live() && token === requestEpoch.current) setRequestLoading(false);
    }
  }
  useEffect(() => {
    mounted.current = true;
    readPending();
    void readRequests();
    try {
      const saved = JSON.parse(sessionStorage.getItem(storageKey) || "null");
      if (
        saved &&
        ["owner", "schedule"].includes(saved.kind) &&
        typeof saved.id === "string"
      )
        setReceipt(saved);
    } catch {}
    return () => {
      mounted.current = false;
      readEpoch.current++;
      requestEpoch.current++;
    };
  }, [scope]);
  useEffect(() => {
    if (civilDate(queryDate)) setDate(queryDate);
    if (tabs.includes(queryTab)) setTab(queryTab);
  }, [queryDate, queryTab]);
  useEffect(() => {
    void readCalendar(date);
  }, [scope, date]);
  function navigate(nextDate = date, nextTab = tab) {
    setDate(nextDate);
    setTab(nextTab);
    w.navigatePath(
      "/c/" +
        encodeURIComponent(fid) +
        "/life/schedule?date=" +
        encodeURIComponent(nextDate) +
        "&tab=" +
        nextTab,
    );
  }
  function checkpoint(kind, value, localDate) {
    const id = kind === "owner" ? value.request?.id : value.request_id;
    if (typeof id !== "string" || !id)
      throw Error("提交回执没有返回可恢复的请求身份，请核对原提交。");
    const identity = { kind, id, localDate };
    if (w.state.session?.actorId === owner)
      try {
        sessionStorage.setItem(storageKey, JSON.stringify(identity));
      } catch {}
    if (live()) {
      setReceipt(identity);
      setReceiptOpen(true);
    }
    return identity;
  }
  function openItem(item, event) {
    if (!live()) return;
    trigger.current = event?.currentTarget || document.activeElement;
    setSelected({
      item: structuredClone(item),
      detail: structuredClone({ ...detail, schedule }),
      date: actualDate,
    });
    setDraft(null);
    setFormError("");
  }
  function editItem(cancel = false) {
    const item = selected.item._source;
    setDraft({
      activity: item.activity || "",
      start: zonedInput(item.start_at, "Asia/Shanghai"),
      end: zonedInput(item.end_at, "Asia/Shanghai"),
      location: item.location || "",
      scene: item.scene || "",
      placeId: item.place_id || "",
      reason: "",
      cancel,
    });
    setFormError("");
  }
  async function runItem(operation = "update") {
    if (
      !live() ||
      running.current ||
      readonly ||
      !selected ||
      itemPending ||
      pendingError
    )
      return;
    const source = selected.item._source,
      submitted = structuredClone(draft);
    if (source.action_status === "completed" || source.status === "cancelled") {
      setFormError("已执行或取消的安排保持历史事实，不能直接改写。");
      return;
    }
    if (selected.date !== today) {
      setFormError("只能向今天的安排提出条目建议，请核对所选日期。");
      return;
    }
    if (
      !submitted?.reason.trim() ||
      (operation !== "cancel" && !submitted?.activity.trim())
    ) {
      setFormError("请填写活动内容和建议原因。");
      return;
    }
    let body;
    try {
      body =
        operation === "cancel"
          ? { reason: submitted.reason.trim() }
          : {
              activity: submitted.activity.trim(),
              location: submitted.location,
              scene: submitted.scene,
              placeId: submitted.placeId,
              reason: submitted.reason.trim(),
              startAt: zonedInputInstant(
                submitted.start,
                "Asia/Shanghai",
                source.start_at,
              ),
              endAt: zonedInputInstant(
                submitted.end,
                "Asia/Shanghai",
                source.end_at,
              ),
            };
      if (
        operation !== "cancel" &&
        Date.parse(body.endAt) <= Date.parse(body.startAt)
      )
        throw Error("结束时间需要晚于开始时间。");
    } catch (cause) {
      setFormError(errorMessage(cause));
      return;
    }
    running.current = true;
    setBusy(true);
    setFormError("");
    try {
      const value = await life.scheduleItem(
        fid,
        selected.detail,
        source,
        body,
        operation,
      );
      checkpoint("owner", value, selected.date);
      if (live()) {
        setSelected(null);
        setDraft(null);
        navigate(date, "suggestions");
        void readRequests();
      }
    } catch (cause) {
      if (live())
        setFormError(
          errorMessage(cause, "建议提交尚未确认，输入与原提交已保留。"),
        );
    } finally {
      running.current = false;
      if (live()) {
        setBusy(false);
        readPending();
      }
    }
  }
  async function retryItem(rebase = false) {
    if (!live() || running.current || readonly) return;
    running.current = true;
    setBusy(true);
    setFormError("");
    try {
      const value = rebase
        ? await life.rebaseScheduleItem(fid, review.detail, review.key)
        : await life.retryScheduleItem(fid);
      checkpoint("owner", value, date);
      if (live()) {
        setReview(null);
        setSelected(null);
        setDraft(null);
        navigate(date, "suggestions");
        void readRequests();
      }
    } catch (cause) {
      if (live()) setFormError(errorMessage(cause));
    } finally {
      running.current = false;
      if (live()) {
        setBusy(false);
        readPending();
      }
    }
  }
  async function reviewOriginal() {
    if (!live() || running.current) return;
    running.current = true;
    setBusy(true);
    setFormError("");
    try {
      const value = await life.detail(fid);
      if (live())
        setReview({
          detail: value,
          key: itemPending.idempotencyKey,
          original: itemPending.value,
        });
    } catch (cause) {
      if (live()) setFormError(errorMessage(cause));
    } finally {
      running.current = false;
      if (live()) setBusy(false);
    }
  }
  function openPlan(event) {
    if (!live()) return;
    trigger.current = event?.currentTarget || document.activeElement;
    planDrafts.current.clear();
    setPlanDate(actualDate);
    setPlanReason("");
    setPlanError("");
    planZone.current = calendar?.timezone || c.timezone || "Asia/Shanghai";
    const original = readPlanPending(actualDate);
    if (original?.value) {
      setPlanDate(original.value.localDate);
      setPlanReason(original.value.reason);
    }
    setPlanOpen(true);
  }
  function changePlanDate(target) {
    if (!planPending) planDrafts.current.set(planDate, planReason);
    setPlanDate(target);
    setPlanError("");
    const original = readPlanPending(target);
    if (original?.value) {
      setPlanDate(original.value.localDate);
      setPlanReason(original.value.reason);
    } else setPlanReason(planDrafts.current.get(target) ?? planReason);
  }
  async function reviewPlanOriginal(event) {
    if (!live() || running.current || readonly || !planPending?.rejection)
      return;
    planReviewTrigger.current = event.currentTarget;
    const original = planPending;
    running.current = true;
    setBusy(true);
    setPlanError("");
    try {
      const latest = await life.schedules(fid, {
        from: original.value.localDate,
        to: original.value.localDate,
      });
      if (
        !Array.isArray(latest.days) ||
        !latest.days.some(
          (item) => item.local_date === original.value.localDate,
        ) ||
        typeof latest.context_revision !== "string" ||
        !latest.context_revision ||
        !latest.timezone
      )
        throw Error("原日期或生活版本尚未完整读到，请重新核对。");
      if (live())
        setPlanReview({
          calendar: structuredClone(latest),
          date: original.value.localDate,
          key: original.idempotencyKey,
          original: structuredClone(original.value),
        });
    } catch (cause) {
      if (live()) setPlanError(errorMessage(cause));
    } finally {
      running.current = false;
      if (live()) setBusy(false);
    }
  }
  async function submitRebasedPlan() {
    if (!live() || running.current || readonly || !planReview) return;
    const review = planReview;
    running.current = true;
    setBusy(true);
    setPlanError("");
    try {
      const value = await life.rebaseSchedule(
        fid,
        review.date,
        review.calendar,
        review.key,
      );
      checkpoint("schedule", value, review.date);
      if (live()) {
        setPlanReview(null);
        setPlanOpen(false);
        navigate(review.date, "timeline");
        void readCalendar(review.date);
      }
    } catch (cause) {
      if (live()) setPlanError(errorMessage(cause));
    } finally {
      running.current = false;
      if (live()) {
        setBusy(false);
        readPlanPending(review.date);
      }
    }
  }
  function releaseOriginalPlan() {
    if (!live() || running.current || readonly || !planPending?.rejection)
      return;
    try {
      life.releaseSchedule(fid, planPending.value.localDate);
      setPlanPending(null);
      setPlanError("");
      setPlanReview(null);
    } catch (cause) {
      setPlanError(errorMessage(cause));
    }
  }
  async function submitPlan(retry = false) {
    if (!live() || running.current || readonly || calendar?.unavailable) return;
    if (!civilDate(planDate) || !planReason.trim()) {
      setPlanError("请选择有效日期并填写安排说明。");
      return;
    }
    const submitted = { date: planDate, reason: planReason.trim() };
    running.current = true;
    setBusy(true);
    setPlanError("");
    try {
      let value;
      if (retry) value = await life.retrySchedule(fid, submitted.date);
      else {
        if (readPlanPending(submitted.date)) return;
        const latest = await life.schedules(fid, {
          from: submitted.date,
          to: submitted.date,
        });
        if (!live()) return;
        if (
          typeof latest.context_revision !== "string" ||
          !latest.context_revision
        )
          throw Error("生活版本尚未完整读到，请重新读取后再安排。");
        if (latest.timezone !== planZone.current)
          throw Error("角色时区已经变化，请保留输入并重新核对。");
        const target = latest.days.find(
          (item) => item.local_date === submitted.date,
        );
        if (!target) throw Error("没有读到所选日期，不能提交新的安排。");
        if (target.schedule)
          throw Error(
            "这一天已有日程记录，原日程保持。可以向今天已有条目提出调整建议。",
          );
        value = await life.submitSchedule(fid, {
          localDate: submitted.date,
          expectedRevision: 0,
          expectedLifeContextRevision: latest.context_revision,
          reason: submitted.reason,
        });
      }
      checkpoint("schedule", value, submitted.date);
      if (live()) {
        setPlanOpen(false);
        navigate(submitted.date, "timeline");
        void readCalendar(submitted.date);
      }
    } catch (cause) {
      if (live())
        setPlanError(errorMessage(cause, "安排请求尚未确认，原输入已保留。"));
    } finally {
      running.current = false;
      if (live()) {
        setBusy(false);
        readPlanPending(submitted.date);
      }
    }
  }
  function releaseOriginal() {
    if (!live() || running.current || readonly) return;
    try {
      life.releaseScheduleItem(fid);
      readPending();
    } catch (cause) {
      setFormError(errorMessage(cause));
    }
  }
  function recoverInitial() {
    if (!live() || readonly) return;
    const operation = w.command(
      "日程",
      "recover-initial-schedule",
      { reason: "恢复当前角色的原日程准备任务" },
      (body, key) =>
        w.client.recoverInitialSchedule(fid, { ...body, idempotencyKey: key }),
      {
        onSaved: () => w.loadLife(),
        success: "已核对原日程准备请求，请查看实际状态",
      },
    );
    void operation.completion.catch(() => {});
  }
  const habits = habitEntries(detail),
    currentDay = calendar?.days?.find((item) => item.local_date === actualDate),
    protectedRecord = !!schedule;
  const terminal = () => {
    void readCalendar(date);
    void readRequests();
  };
  const field = (name, value) => setDraft((old) => ({ ...old, [name]: value }));
  return (
    <div className="replica-life replica-schedule">
      <WorkspacePageHeader
        eyebrow="LIFE / A DAY AT A TIME"
        title="日程"
        description="看看今天如何展开，也为接下来的生活留一点期待。"
        actions={
          <Button disabled={readonly || !calendar} onClick={openPlan}>
            <SparklesIcon size={15} />
            规划日程
          </Button>
        }
      />
      <Tabs
        value={tab}
        onValueChange={(value) => navigate(date, value)}
        className="lp-stack"
      >
        <div className="lp-toolbar">
          <TabsList variant="line" className="replica-segment-tabs">
            <TabsTrigger value="timeline">日程时间线</TabsTrigger>
            <TabsTrigger value="suggestions">
              调整建议{" "}
              <span>
                {requestReady
                  ? pending.length + (requestCursor ? "+" : "")
                  : "…"}
              </span>
            </TabsTrigger>
            <TabsTrigger value="habits">生活倾向</TabsTrigger>
          </TabsList>
          <div className="lp-date">
            <Button
              variant="ghost"
              size="icon"
              aria-label="前一天"
              disabled={calendar?.unavailable}
              onClick={() => navigate(shiftDate(date, -1))}
            >
              <ChevronLeftIcon size={15} />
            </Button>
            <DateField
              aria-label="查看日程日期"
              value={actualDate}
              disabled={calendar?.unavailable}
              onChange={(event) => navigate(event.target.value)}
            />
            <Button
              variant="ghost"
              size="icon"
              aria-label="后一天"
              disabled={calendar?.unavailable}
              onClick={() => navigate(shiftDate(date, 1))}
            >
              <ChevronRightIcon size={15} />
            </Button>
            <Button variant="outline" onClick={() => navigate(today)}>
              今天
            </Button>
          </div>
        </div>
        {readError && (
          <InlineFeedback role="alert">
            <p>{readError}</p>
            <Button
              variant="outline"
              disabled={loading}
              onClick={() => void readCalendar(date)}
            >
              重新读取这一天
            </Button>
          </InlineFeedback>
        )}
        {pendingError && (
          <InlineFeedback role="alert">{pendingError}</InlineFeedback>
        )}
        {itemPending && (
          <Surface className="lp-pad replica-pending-command">
            <h3>继续上次日程建议</h3>
            <p>
              {itemPending.rejection
                ? "原建议因版本变化未被接受。请核对新版本，内容与原因保持。"
                : "原提交的受理结果尚未确认。保留原输入与身份，先核对它。"}
            </p>
            <p>
              {itemPending.value.request.activity || "取消日程的建议"} ·{" "}
              {userInstant(itemPending.value.request.startAt)}
            </p>
            <div className="lp-action-row">
              {itemPending.rejection ? (
                <>
                  <Button
                    variant="outline"
                    disabled={busy || readonly}
                    onClick={() => void reviewOriginal()}
                  >
                    保留原建议，核对最新日程
                  </Button>
                  <Button
                    variant="ghost"
                    disabled={busy || readonly}
                    onClick={releaseOriginal}
                  >
                    放弃这条未接受的建议
                  </Button>
                </>
              ) : (
                <Button
                  disabled={busy || readonly}
                  onClick={() => void retryItem()}
                >
                  查询或重试原提交
                </Button>
              )}
            </div>
          </Surface>
        )}
        {formError && !selected && (
          <InlineFeedback role="alert">{formError}</InlineFeedback>
        )}
        {calendar?.unavailable && (
          <InlineFeedback variant="default">
            当前服务仅提供已读取的今天，跨日查询与生成需要核对当前 Core
            服务版本。
          </InlineFeedback>
        )}
        <TabsContent value={tab}>
          {tab === "timeline" ? (
            <div className="lp-grid">
              <Surface className="lp-pad">
                <div className="replica-section-heading">
                  <div>
                    <h2>
                      {Number(actualDate.slice(5, 7)) +
                        "月" +
                        Number(actualDate.slice(8, 10)) +
                        "日"}{" "}
                      ·{" "}
                      {actualDate === today
                        ? "今天"
                        : actualDate > today
                          ? "未来的安排"
                          : "过去的安排"}
                    </h2>
                    <p>
                      {current
                        ? rows.length + " 项安排"
                        : loading
                          ? "正在读取安排"
                          : "安排尚未读到"}{" "}
                      · {calendar?.timezone || c.timezone || "时区待读取"}
                    </p>
                  </div>
                  <Button
                    variant="link"
                    disabled={loading}
                    onClick={() => void readCalendar(date)}
                  >
                    刷新
                  </Button>
                </div>
                {loading && <LoadingState label="正在读取这一天的日程…" />}
                {rows.length ? (
                  <ScheduleTimeline items={rows} onSelect={openItem} />
                ) : (
                  !loading &&
                  !readError &&
                  current && (
                    <EmptyState>
                      <h3>
                        {protectedRecord
                          ? "这一天已有日程记录，当前没有条目"
                          : "这一天，还是空白的一页"}
                      </h3>
                      <p>
                        {protectedRecord
                          ? "生成操作会保留整份已有日程。"
                          : "可以请求为这一天安排生活，已经确认的日程会被保留。"}
                      </p>
                      <Button
                        variant="outline"
                        disabled={readonly || calendar?.unavailable}
                        onClick={openPlan}
                      >
                        规划这一天
                      </Button>
                      {date === today && !protectedRecord && (
                        <Button
                          variant="link"
                          disabled={readonly}
                          onClick={onRecoverInitial || recoverInitial}
                        >
                          恢复初始日程
                        </Button>
                      )}
                    </EmptyState>
                  )
                )}
              </Surface>
              <div className="lp-stack">
                <Surface className="lp-pad">
                  <div className="replica-section-heading">
                    <h2>日程小记</h2>
                  </div>
                  <div className="lp-runtime-item">
                    <span className="lp-icon-box">
                      <CalendarDaysIcon size={16} />
                    </span>
                    <div>
                      <span className="lp-muted">已确认安排</span>
                      <strong className="lp-title">
                        {current
                          ? rows.length + " 项"
                          : loading
                            ? "读取中"
                            : "尚未读到"}
                      </strong>
                    </div>
                  </div>
                  <div className="lp-runtime-item">
                    <span className="lp-icon-box">
                      <Clock3Icon size={16} />
                    </span>
                    <div>
                      <span className="lp-muted">已读取待处理建议</span>
                      <strong className="lp-title">
                        {requestReady
                          ? pending.length + (requestCursor ? "+" : "") + " 条"
                          : requestLoading
                            ? "读取中"
                            : "尚未读到"}
                      </strong>
                    </div>
                  </div>
                  {requestError && (
                    <InlineFeedback role="alert">
                      <p>{requestError}</p>
                      <Button
                        variant="outline"
                        disabled={requestLoading}
                        onClick={() => requestRetry.current?.()}
                      >
                        重新读取建议
                      </Button>
                    </InlineFeedback>
                  )}
                  <WorkspaceInfo>
                    修改会先成为建议，等{c.name}确认后，再更新已接受的安排。
                  </WorkspaceInfo>
                </Surface>
                {pending.length > 0 && (
                  <Button
                    variant="ghost"
                    className="lp-memory-card"
                    onClick={() => navigate(date, "suggestions")}
                  >
                    <span className="lp-title">
                      有 {pending.length} 条已读取建议在等待回应
                    </span>
                    <span className="lp-muted">查看建议与处理结果 →</span>
                  </Button>
                )}
                {currentDay?.request && (
                  <Surface className="lp-pad">
                    <p className="lp-muted">这一天的原规划请求</p>
                    <Button
                      variant="link"
                      onClick={() => {
                        setReceipt({
                          kind: "schedule",
                          id: currentDay.request.request_id,
                          localDate: actualDate,
                        });
                        setReceiptOpen(true);
                      }}
                    >
                      查看进度与结果
                    </Button>
                  </Surface>
                )}
              </div>
            </div>
          ) : tab === "suggestions" ? (
            <div className="lp-stack">
              <div className="lp-row lp-between">
                <span className="lp-muted">
                  已读取 {suggestions.length} 条建议
                  {requestCursor ? "，仍可继续读取" : ""}
                </span>
                <div className="lp-row">
                  {receipt && (
                    <Button variant="link" onClick={() => setReceiptOpen(true)}>
                      查看上次请求
                    </Button>
                  )}
                  <Button
                    variant="outline"
                    disabled={requestLoading}
                    onClick={() => void readRequests()}
                  >
                    刷新建议
                  </Button>
                </div>
              </div>
              {requestLoading && <LoadingState label="正在读取日程建议…" />}
              {requestError && (
                <InlineFeedback role="alert">
                  <p>{requestError}</p>
                  <Button
                    variant="outline"
                    disabled={requestLoading}
                    onClick={() => requestRetry.current?.()}
                  >
                    重新读取建议
                  </Button>
                </InlineFeedback>
              )}
              {suggestions.map((request) => (
                <article className="lp-suggestion" key={request.id}>
                  <div className="lp-row lp-between">
                    <Badge
                      variant="outline"
                      className="lp-status"
                      data-status={request.status}
                    >
                      {requestLabels[request.status] || "状态尚未确认"}
                    </Badge>
                    <span className="lp-muted">
                      {userInstant(request.created_at)} 提出
                    </span>
                  </div>
                  <div className="lp-change">
                    <div>
                      <span className="lp-label">原安排</span>
                      <p>
                        {typeof request.payload?.before?.activity === "string"
                          ? request.payload.before.activity
                          : "原日程与版本保留在请求来源中。"}
                      </p>
                    </div>
                    <ArrowRightIcon size={18} />
                    <div>
                      <span className="lp-label">你的建议</span>
                      <p>
                        {typeof request.payload?.activity === "string" &&
                        request.payload.activity
                          ? request.payload.activity
                          : "建议取消这段安排"}
                        <br />
                        {userInstant(request.payload?.start_at)}
                      </p>
                    </div>
                  </div>
                  <p className="lp-small">
                    {requestDecisionText(request) ||
                      "已提交建议，正在等待真实处理结果。"}
                  </p>
                  <Button
                    variant="link"
                    onClick={() => {
                      setReceipt({
                        kind: "owner",
                        id: request.id,
                        localDate: date,
                      });
                      setReceiptOpen(true);
                    }}
                  >
                    查看建议进度与结果
                  </Button>
                </article>
              ))}
              {!suggestions.length && !requestLoading && !requestError && (
                <Surface className="lp-pad">
                  <EmptyState>
                    <h3>还没有读到日程调整建议</h3>
                    <p>点击日程中的一项安排，可以提出修改。</p>
                    <Button
                      variant="outline"
                      onClick={() => navigate(date, "timeline")}
                    >
                      查看日程
                    </Button>
                  </EmptyState>
                </Surface>
              )}
              {requestCursor && (
                <Button
                  variant="outline"
                  disabled={requestLoading}
                  onClick={() => void readRequests(true)}
                >
                  加载更多建议
                </Button>
              )}
            </div>
          ) : (
            <div className="lp-three">
              {habits.map((habit, index) => (
                <Surface className="lp-pad" key={habit.title + index}>
                  <LeafIcon size={22} />
                  <h3 className="lp-title replica-habit-title">
                    {habit.title}
                  </h3>
                  <p className="lp-small">{habit.text}</p>
                  <p className="lp-muted replica-next-note">
                    软倾向 · 不等于固定日程
                  </p>
                </Surface>
              ))}
              {loading && <LoadingState label="正在读取生活倾向…" />}
              {!loading && !habits.length && !readError && (
                <EmptyState>
                  <h3>生活习惯，等相处中慢慢了解</h3>
                  <p>当前还没有确认的生活倾向。</p>
                </EmptyState>
              )}
            </div>
          )}
        </TabsContent>
        <Disclosure title="约定、意向与日程来源">
          <p className="lp-muted">
            计划与实际执行分别记录，约定是否履行需要实际证据。
          </p>
          <div className="lp-action-row">
            <Button variant="link" asChild>
              <a
                href={
                  "#/c/" +
                  encodeURIComponent(fid) +
                  "/life/schedule/planning?section=promises"
                }
              >
                查看约定与意向
              </a>
            </Button>
            <Button variant="link" asChild>
              <a href={"#/c/" + encodeURIComponent(fid) + "/life/timeline"}>
                生活时间线
              </a>
            </Button>
          </div>
        </Disclosure>
      </Tabs>
      <WorkspaceDialog
        open={!!selected}
        busy={busy}
        returnFocus={() => trigger.current}
        onOpenChange={(open) => {
          if (!open && !busy) {
            setSelected(null);
            setDraft(null);
            setFormError("");
          }
        }}
        title={draft ? "提出日程调整" : "这一段安排"}
        description={
          selected ? selected.date + " · 时间按北京时间显示" : undefined
        }
        footer={
          draft ? (
            <>
              <Button
                variant="outline"
                disabled={busy}
                onClick={() => {
                  setDraft(null);
                  setFormError("");
                }}
              >
                取消修改
              </Button>
              <Button
                disabled={busy || readonly || !!itemPending || !!pendingError}
                onClick={() => void runItem(draft.cancel ? "cancel" : "update")}
              >
                {busy
                  ? "正在提交…"
                  : draft.cancel
                    ? "提交取消建议"
                    : "提交调整建议"}
              </Button>
            </>
          ) : (
            <>
              <Button variant="outline" onClick={() => setSelected(null)}>
                关闭
              </Button>
              <Button
                disabled={
                  readonly ||
                  busy ||
                  !selected?.item.editable ||
                  selected?.date !== today ||
                  selected?.item._source.action_status === "completed" ||
                  selected?.item._source.status === "cancelled" ||
                  !!itemPending ||
                  !!pendingError
                }
                onClick={() => editItem()}
              >
                <Edit3Icon size={14} />
                提出修改
              </Button>
            </>
          )
        }
      >
        {selected &&
          (draft ? (
            <div className="lp-stack">
              {!draft.cancel && (
                <div className="lp-form">
                  <Field label="开始时间（北京时间）">
                    <FieldInput
                      type="datetime-local"
                      aria-label="建议开始时间"
                      value={draft.start}
                      disabled={busy || !!itemPending}
                      onChange={(event) => field("start", event.target.value)}
                    />
                  </Field>
                  <Field label="结束时间（北京时间）">
                    <FieldInput
                      type="datetime-local"
                      aria-label="建议结束时间"
                      value={draft.end}
                      disabled={busy || !!itemPending}
                      onChange={(event) => field("end", event.target.value)}
                    />
                  </Field>
                  <Field label="活动内容" wide>
                    <FieldInput
                      aria-label="建议活动内容"
                      value={draft.activity}
                      disabled={busy || !!itemPending}
                      onChange={(event) =>
                        field("activity", event.target.value)
                      }
                    />
                  </Field>
                  <Field label="地点" wide>
                    <FieldInput
                      aria-label="建议地点"
                      value={draft.location}
                      disabled={busy || !!itemPending}
                      onChange={(event) =>
                        field("location", event.target.value)
                      }
                    />
                  </Field>
                </div>
              )}
              <div className="lp-form">
                <Field label="想对她说" wide>
                  <Textarea
                    aria-label="日程建议原因"
                    maxLength={500}
                    value={draft.reason}
                    disabled={busy || !!itemPending}
                    onChange={(event) => field("reason", event.target.value)}
                  />
                </Field>
              </div>
              {!draft.cancel && (
                <Disclosure title="场景与精确地点">
                  <Field label="场景">
                    <FieldInput
                      value={draft.scene}
                      disabled={busy || !!itemPending}
                      onChange={(event) => field("scene", event.target.value)}
                    />
                  </Field>
                  <Field label="地点 ID">
                    <FieldInput
                      value={draft.placeId}
                      disabled={busy || !!itemPending}
                      onChange={(event) => field("placeId", event.target.value)}
                    />
                  </Field>
                </Disclosure>
              )}
              <WorkspaceInfo>
                提交后会显示待处理建议，当前日程保持原样，直到建议被实际采纳。
              </WorkspaceInfo>
              {formError && (
                <InlineFeedback role="alert">{formError}</InlineFeedback>
              )}
              {itemPending && (
                <p role="status">
                  原提交身份已保留，关闭此处可继续核对原建议。
                </p>
              )}
            </div>
          ) : (
            <div className="lp-stack">
              <h3 className="lp-title">{selected.item.text}</h3>
              <dl className="replica-key-values">
                <dt>时间</dt>
                <dd>
                  {userInstant(selected.item._source.start_at)} —{" "}
                  {userInstant(selected.item._source.end_at)}
                </dd>
                <dt>地点</dt>
                <dd>{selected.item._source.location || "尚未记录"}</dd>
                <dt>类型</dt>
                <dd>{selected.item._source.scene || "尚未记录"}</dd>
                <dt>状态</dt>
                <dd>
                  {selected.item._source.action_status === "in_progress"
                    ? "正在执行"
                    : selected.item._source.action_status === "completed"
                      ? "已完成执行"
                      : "计划安排，实际执行另行确认"}
                </dd>
              </dl>
              {selected.date !== today && (
                <p className="lp-muted">
                  过去和未来的安排供查阅，条目建议面向今天。
                </p>
              )}
              <Disclosure title="原日程与来源">
                <pre className="replica-source-record">
                  {JSON.stringify(selected.item._source, null, 2)}
                </pre>
                <Button
                  variant="outline"
                  disabled={
                    readonly ||
                    busy ||
                    selected.date !== today ||
                    !!itemPending ||
                    !!pendingError ||
                    selected.item._source.action_status === "completed" ||
                    selected.item._source.status === "cancelled"
                  }
                  onClick={() => editItem(true)}
                >
                  建议取消这段安排
                </Button>
              </Disclosure>
            </div>
          ))}
      </WorkspaceDialog>
      <WorkspaceDialog
        open={planOpen}
        busy={busy}
        returnFocus={() => trigger.current}
        onOpenChange={(open) => {
          if (!busy) setPlanOpen(open);
        }}
        title="为一天安排生活"
        description="根据已有的生活倾向补齐空白日期，已有日程记录会受到保护。"
        footer={
          <>
            <Button
              variant="outline"
              disabled={busy}
              onClick={() => setPlanOpen(false)}
            >
              取消
            </Button>
            <Button
              disabled={
                busy ||
                readonly ||
                !!planPending ||
                !!calendar?.unavailable ||
                (planDate === actualDate && protectedRecord)
              }
              onClick={() => void submitPlan()}
            >
              {busy ? "正在提交…" : "请求安排日程"}
            </Button>
          </>
        }
      >
        <div className="lp-stack">
          <Field label="日期">
            <DateField
              aria-label="规划日期"
              value={planDate}
              disabled={busy || !!planPending}
              onChange={(event) => changePlanDate(event.target.value)}
            />
          </Field>
          <Field label="安排说明">
            <Textarea
              aria-label="安排说明"
              maxLength={500}
              value={planReason}
              disabled={busy || !!planPending}
              onChange={(event) => setPlanReason(event.target.value)}
            />
          </Field>
          <dl className="replica-key-values">
            <dt>角色</dt>
            <dd>{c.name}</dd>
            <dt>归档时区</dt>
            <dd>{planZone.current}</dd>
          </dl>
          <WorkspaceInfo>
            {planDate === actualDate && protectedRecord
              ? `这一天已有日程记录，包含 ${rows.length} 项安排。整份记录会受到保护，请选择空白日期，或向今天的条目提出建议。`
              : "只为缺少日程记录的日期规划，已有记录即使暂时没有条目，也会保留。"}
          </WorkspaceInfo>
          {planPending && (
            <InlineFeedback variant="default">
              <p>
                {planPending.rejection
                  ? "原规划因版本变化未被接受，原日期与说明已保留。"
                  : "原提交回执尚未确认，输入与身份已经保留。"}
              </p>
              {planPending.rejection ? (
                <div className="lp-action-row">
                  <Button
                    disabled={busy || readonly}
                    onClick={(event) => void reviewPlanOriginal(event)}
                  >
                    保留原规划，核对最新版本
                  </Button>
                  <Button
                    variant="ghost"
                    disabled={busy || readonly}
                    onClick={releaseOriginalPlan}
                  >
                    放弃这条未接受的规划请求
                  </Button>
                </div>
              ) : (
                <Button
                  disabled={busy || planPending.unreadable}
                  onClick={() => void submitPlan(true)}
                >
                  查询或重试原规划
                </Button>
              )}
            </InlineFeedback>
          )}
          {planError && (
            <InlineFeedback role="alert">{planError}</InlineFeedback>
          )}
        </div>
      </WorkspaceDialog>
      <WorkspaceDialog
        open={!!planReview}
        busy={busy}
        returnFocus={() => planReviewTrigger.current}
        onOpenChange={(open) => {
          if (!open && !busy) setPlanReview(null);
        }}
        title="核对原规划的新版本"
        description="仅明确未被受理的版本冲突可以在确认后采用新基线。"
        footer={
          <>
            <Button
              variant="outline"
              disabled={busy}
              onClick={() => setPlanReview(null)}
            >
              取消
            </Button>
            <Button
              disabled={
                busy ||
                readonly ||
                !!planReview?.calendar.days.find(
                  (item) => item.local_date === planReview.date,
                )?.schedule
              }
              onClick={() => void submitRebasedPlan()}
            >
              保留原规划并重新提交
            </Button>
          </>
        }
      >
        <div className="lp-stack">
          <p>规划日期：{planReview?.date}</p>
          <p>归档时区：{planReview?.calendar.timezone}</p>
          <p>原说明：{planReview?.original.reason}</p>
          <p>
            日期和说明保持原样，采用刚刚核对的生活版本。已有日程记录仍受到保护。
          </p>
          {planReview?.calendar.days.find(
            (item) => item.local_date === planReview.date,
          )?.schedule && (
            <InlineFeedback role="alert">
              这一天已有日程记录，不能重新生成。可以返回并放弃这条未被接受的规划请求。
            </InlineFeedback>
          )}
          {planError && (
            <InlineFeedback role="alert">{planError}</InlineFeedback>
          )}
        </div>
      </WorkspaceDialog>
      <WorkspaceDialog
        open={!!review}
        busy={busy}
        onOpenChange={(open) => {
          if (!open && !busy) setReview(null);
        }}
        title="保留原建议，核对最新日程"
        description="仅明确未被接受的版本冲突可以采用新基线。"
        footer={
          <>
            <Button
              variant="outline"
              disabled={busy}
              onClick={() => setReview(null)}
            >
              取消
            </Button>
            <Button
              disabled={busy || readonly}
              onClick={() => void retryItem(true)}
            >
              保留原建议并重新提交
            </Button>
          </>
        }
      >
        <p>
          将采用最新日程版本 {review?.detail.schedule?.revision ?? "尚未读到"}{" "}
          核对。时间、地点、内容和原因保持原提交。
        </p>
        <pre className="replica-source-record">
          {review && JSON.stringify(review.original.request, null, 2)}
        </pre>
        {formError && <InlineFeedback role="alert">{formError}</InlineFeedback>}
      </WorkspaceDialog>
      <WorkspaceDialog
        open={receiptOpen && !!receipt}
        onOpenChange={setReceiptOpen}
        title={
          receipt?.kind === "schedule" ? "日程安排进度" : "日程建议处理进度"
        }
        description="跟进原请求，关闭不会取消后台处理。"
      >
        {receipt && (
          <RequestStatus
            key={receipt.kind + receipt.id}
            fid={fid}
            requestId={receipt.id}
            kind={receipt.kind}
            onTerminal={terminal}
          />
        )}
      </WorkspaceDialog>
    </div>
  );
}

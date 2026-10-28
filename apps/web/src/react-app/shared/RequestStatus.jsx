import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Badge, Disclosure } from "../controls";
import { InlineFeedback } from "./Presentation";
import LoadingState from "./LoadingState";
import { jsonValueSnapshot } from "@/state/view-model";
import { useWorkspace } from "../page";
import { errorMessage } from "@/v3/domain-presentation.mjs";
import { requestDecisionText } from "@/v3/proto/components/request-failure.js";
const ownerLabels = {
  pending: "等待回应",
  processing: "正在考虑",
  accepted: "已接受，等待实际行动",
  completed: "本次处理已结束",
  rejected: "暂未采纳",
  deferred: "稍后决定",
  awaiting_goal: "等待目标规划",
  cancelled: "已取消",
  failed: "处理失败",
};
const scheduleLabels = {
  queued: "等待规划",
  running: "正在规划安排",
  completed: "已读回处理结果",
  failed: "这次安排未完成",
  conflict: "资料已经变化",
};
export default function RequestStatus({
  fid,
  requestId,
  kind = "owner",
  initial = null,
  onTerminal,
  ownerScoped = false,
}) {
  const w = useWorkspace(),
    owner = w.state.session.actorId,
    service = w.services.lifeClient;
  const [record, setRecord] = useState(initial),
    [loading, setLoading] = useState(false),
    [error, setError] = useState(""),
    [paused, setPaused] = useState(false);
  const latest = useRef(initial),
    notify = useRef(onTerminal),
    query = useRef(null);
  notify.current = onTerminal;
  useEffect(() => {
    let alive = true,
      inFlight = false,
      timer,
      reads = 0,
      notified = false;
    const live = () =>
      alive &&
      !!fid &&
      !!requestId &&
      (ownerScoped
        ? w.state.session?.actorId === owner
        : w.isCurrent(owner, fid));
    const pending = (value) =>
      kind === "schedule"
        ? ["queued", "running"].includes(value?.status)
        : ["pending", "processing", "deferred", "awaiting_goal"].includes(
            value?.status,
          );
    async function read(manual = false) {
      clearTimeout(timer);
      if (!live() || inFlight || document.hidden) return;
      if (manual) {
        reads = 0;
        setPaused(false);
      }
      inFlight = true;
      setLoading(true);
      try {
        const value =
          kind === "schedule"
            ? await service.scheduleRequest(fid, requestId)
            : await service.request(fid, requestId);
        if (!live()) return;
        if (
          value.fluctlight_id !== fid ||
          (kind === "owner" ? value.id : value.request_id) !== requestId
        )
          throw Error("请求范围不匹配，未采用这次读取结果。");
        if (!(
          Number.isSafeInteger(value.revision) &&
          Number.isSafeInteger(latest.current?.revision) &&
          value.revision < latest.current.revision
        )) {
          latest.current = jsonValueSnapshot(value);
          setRecord(latest.current);
        }
        setError("");
        if (!pending(latest.current) && !notified) {
          notified = true;
          Promise.resolve(notify.current?.(latest.current)).catch((cause) => {
            if (live())
              setError(
                "请求结果已保留，页面读回未完成：" + errorMessage(cause),
              );
          });
        }
      } catch (cause) {
        if (live())
          setError(
            "更新失败，保留上次状态。" +
              errorMessage(cause, "暂时无法读取请求。"),
          );
      } finally {
        inFlight = false;
        reads++;
        if (live()) {
          setLoading(false);
          if (
            (!latest.current || pending(latest.current)) &&
            reads < 150 &&
            !document.hidden
          )
            timer = setTimeout(
              () => void read(),
              kind === "schedule" ? 3000 : 2000,
            );
          else if (latest.current && pending(latest.current) && reads >= 150)
            setPaused(true);
        }
      }
    }
    const visible = () => {
      clearTimeout(timer);
      if (!document.hidden && reads < 150) void read();
    };
    query.current = () => void read(true);
    document.addEventListener("visibilitychange", visible);
    void read();
    return () => {
      alive = false;
      clearTimeout(timer);
      document.removeEventListener("visibilitychange", visible);
      query.current = null;
    };
  }, [w, owner, fid, requestId, kind, service, ownerScoped]);
  const labels = kind === "schedule" ? scheduleLabels : ownerLabels;
  const result = record?.result || {};
  const completed = kind === "schedule" && record?.status === "completed";
  const outcome = completed
    ? result.disposition === "preserved"
      ? "这一天已有日程，原安排已保留。"
      : result.disposition === "already_scheduled"
        ? "已定位原有安排，执行结果仍单独记录。"
        : result.disposition === "scheduled"
          ? "已读回真实日程条目，执行与履约仍单独记录。"
          : "日程处理结果已读回，请核对这一天的实际安排。"
    : record && kind === "owner"
      ? requestDecisionText(record)
      : "";
  return (
    <div className="replica-request-status" data-testid="request-status">
      {!record && loading && <LoadingState label="正在读取原请求…" />}
      {record && (
        <>
          <Badge
            variant="outline"
            className="lp-status"
            data-status={record.status}
          >
            {labels[record.status] || "状态尚未确认"}
          </Badge>
          {record.local_date && (
            <p className="lp-small">日程日期：{record.local_date}</p>
          )}
          {outcome && <p className="lp-small">{outcome}</p>}
          {kind === "schedule" &&
            ["failed", "conflict"].includes(record.status) && (
              <p className="lp-small">
                {record.error_code === "schedule_scope_changed"
                  ? "角色、人格或世界已经变化，请核对资料。"
                  : record.error_code === "schedule_task_stopped"
                    ? "原任务已停止，已保存的结果仍保留。"
                    : record.error_code === "schedule_request_date_elapsed"
                      ? "目标日期已经过去，没有补造历史日程。"
                      : "本次处理没有形成新的安排，请查看原任务原因。"}
              </p>
            )}
          <p className="lp-muted">
            {kind === "schedule"
              ? "关闭此处不会取消后台任务，也不会重新生成。"
              : "建议由角色判断，受理回执与实际行动结果分别记录。"}
          </p>
          {record.tasks_state === "failed" && (
            <p role="status">关联任务尚未完整读到，可以重新读取。</p>
          )}
          <div className="lp-action-row">
            {kind === "owner" && (
              <Button variant="link" asChild>
                <a href={"#/diag/request/" + encodeURIComponent(requestId)}>
                  查看原请求与关联任务
                </a>
              </Button>
            )}
            {record.task_id && (
              <Button variant="link" asChild>
                <a href={"#/diag/task/" + encodeURIComponent(record.task_id)}>
                  查看关联任务
                </a>
              </Button>
            )}
            {record.tasks?.map((task) => (
              <Button key={task.task_id} variant="link" asChild>
                <a href={"#/diag/task/" + encodeURIComponent(task.task_id)}>
                  查看任务 · {task.status}
                </a>
              </Button>
            ))}
          </div>
          <Disclosure title="原请求与运行记录">
            <pre className="replica-source-record">
              {JSON.stringify(record, null, 2)}
            </pre>
          </Disclosure>
        </>
      )}
      {paused && (
        <p role="status">
          自动更新已暂停，可以点击刷新原请求继续；后台任务仍按实际状态处理。
        </p>
      )}
      {error && <InlineFeedback role="alert">{error}</InlineFeedback>}
      <Button
        variant="outline"
        disabled={loading}
        onClick={() => query.current?.()}
      >
        {loading ? "正在读取…" : "刷新原请求"}
      </Button>
    </div>
  );
}

import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { Feedback, type FeedbackKind } from "@/react-app/shared/Presentation";

export type AsyncTaskState = "queued" | "running" | "succeeded" | "failed" | "cancelled";

const kinds: Record<AsyncTaskState, FeedbackKind> = {
  queued: "progress",
  running: "progress",
  succeeded: "success",
  failed: "error",
  cancelled: "information",
};
const defaultTitles: Record<AsyncTaskState, string> = {
  queued: "已受理，等待处理",
  running: "正在处理",
  succeeded: "处理完成",
  failed: "处理未完成",
  cancelled: "已取消",
};

/**
 * AsyncTaskStatus — one presentation for long-running work (generation,
 * planning, imports). Accepted ≠ completed: `queued` and `running` never read
 * as success. `failed` offers retry when `onRetry` is given; technical detail
 * stays behind a disclosure.
 */
export function AsyncTaskStatus({
  state,
  title,
  message,
  progress,
  updatedAt,
  onRetry,
  retryLabel = "重试",
  onCancel,
  cancelLabel = "取消任务",
  details,
}: {
  state: AsyncTaskState;
  title?: ReactNode;
  message?: ReactNode;
  /** 0–100 when the producer reports real progress; omit otherwise. */
  progress?: number;
  updatedAt?: ReactNode;
  onRetry?: () => void;
  retryLabel?: string;
  onCancel?: () => void;
  cancelLabel?: string;
  details?: ReactNode;
}) {
  const active = state === "queued" || state === "running";
  const actions = (state === "failed" && onRetry) || (active && onCancel) ? (
    <>
      {state === "failed" && onRetry && <Button type="button" variant="outline" onClick={onRetry}>{retryLabel}</Button>}
      {active && onCancel && <Button type="button" variant="ghost" onClick={onCancel}>{cancelLabel}</Button>}
    </>
  ) : undefined;
  return (
    <Feedback
      kind={kinds[state]}
      data-task-state={state}
      title={title ?? defaultTitles[state]}
      actions={actions}
      details={details}
    >
      {message && <span className="block">{message}</span>}
      {state === "running" && typeof progress === "number" && (
        <Progress value={Math.max(0, Math.min(100, progress))} aria-label="处理进度" className="mt-2" />
      )}
      {updatedAt && <span className="ui-task-status__time block text-xs text-muted-foreground">{updatedAt}</span>}
    </Feedback>
  );
}

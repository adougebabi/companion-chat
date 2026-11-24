import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { EmptyState, Feedback } from "@/react-app/shared/Presentation";
import LoadingState from "@/react-app/shared/LoadingState";
import { cn } from "@/lib/utils";

/**
 * DataState — the single loading / failure / empty / ready contract for lists
 * and details.
 * - loading without data: skeleton; loading with data: keep data, show progress.
 * - error without data: error + retry; error with data: warning, keep data.
 * - empty: EmptyState with optional action.
 */
export function DataState({
  label,
  loading = false,
  error,
  empty = false,
  hasData = !empty,
  emptyTitle,
  emptyDescription,
  emptyAction,
  onRetry,
  children,
}: {
  /** What is being read, e.g. "约定". Used in status text. */
  label: string;
  loading?: boolean;
  error?: ReactNode;
  empty?: boolean;
  hasData?: boolean;
  emptyTitle?: ReactNode;
  emptyDescription?: ReactNode;
  emptyAction?: ReactNode;
  onRetry?: () => void;
  children?: ReactNode;
}) {
  const retry = onRetry && <Button type="button" variant="outline" onClick={onRetry}>重新读取{label}</Button>;
  if (loading && !hasData) return <LoadingState label={`正在读取${label}…`} />;
  if (error && !hasData) return <Feedback kind="error" title={`${label}暂时无法读取`} actions={retry}>{error}</Feedback>;
  return (
    <div data-ui-data-state="" className="ui-stack">
      {loading && <Feedback kind="progress">正在重新读取{label}，上次确认的内容仍保留。</Feedback>}
      {error && <Feedback kind="warning" title={`${label}暂未读全`} actions={retry}>{error}<span className="block">上次确认的内容仍保留。</span></Feedback>}
      {empty ? (
        <EmptyState>
          <div className="ui-stack text-center">
            <strong>{emptyTitle ?? `还没有${label}`}</strong>
            {emptyDescription && <p className="text-sm text-muted-foreground">{emptyDescription}</p>}
            {emptyAction && <div className="ui-actions justify-center">{emptyAction}</div>}
          </div>
        </EmptyState>
      ) : children}
    </div>
  );
}

/** RecordList — vertical list of records; rows wrap instead of overflowing. */
export function RecordList({ label, children, className }: { label: string; children: ReactNode; className?: string }) {
  return <ul aria-label={label} data-ui-record-list="" className={cn("ui-record-list", className)}>{children}</ul>;
}

export function RecordListItem({
  title,
  meta,
  status,
  actions,
  children,
}: {
  title: ReactNode;
  meta?: ReactNode;
  status?: ReactNode;
  actions?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <li data-ui-record-item="" className="ui-record-item">
      <div className="ui-record-item__main">
        <div className="ui-record-item__heading">
          <strong className="ui-record-item__title">{title}</strong>
          {status && <Badge variant="secondary">{status}</Badge>}
        </div>
        {meta && <div className="ui-record-item__meta">{meta}</div>}
        {children}
      </div>
      {actions && <div className="ui-actions ui-record-item__actions">{actions}</div>}
    </li>
  );
}

/** DetailFields — label/value pairs; stacked on phones, two columns wider. */
export function DetailFields({
  items,
  emptyText = "尚未记录",
}: {
  items: { label: ReactNode; value?: ReactNode }[];
  emptyText?: string;
}) {
  return (
    <dl data-ui-detail-fields="" className="ui-detail-fields">
      {items.map((item, index) => (
        <div key={index} className="ui-detail-fields__row">
          <dt>{item.label}</dt>
          <dd>{item.value == null || item.value === "" ? <span className="text-muted-foreground">{emptyText}</span> : item.value}</dd>
        </div>
      ))}
    </dl>
  );
}

import {ReadState} from "@/react-app/shared/Presentation";
import { useCallback, useEffect, useState, useRef } from "react";
import { useWorkspace } from "../page";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Disclosure } from "../controls";
import { stateLabel, errorMessage } from "@/v3/domain-presentation.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";
export type DiagnosticTarget = {
  kind: string;
  id?: string;
  recordKind?: string;
  query?: string;
};
export default function DiagnosticsDetailPage({
  target,
}: {
  target: DiagnosticTarget;
}) {
  const w = useWorkspace(),
    [data, setData] = useState<any>(null),
    [error, setError] = useState(""),
    [loading, setLoading] = useState(false),
    [revision, refresh] = useState(0);
  const lastTarget = useRef("");
  useEffect(() => {
    let live = true;
    setLoading(true);
    setError("");
    const identity = JSON.stringify(target);
    if (lastTarget.current !== identity) {
      lastTarget.current = identity;
      setData(null);
    }
    const service = w.services.diagnosticsClient;
    const read =
      target.kind === "request"
        ? w.client.ownerRequest(target.id)
        : target.kind === "task"
          ? service.workflow(target.id)
          : target.kind === "record"
            ? service.record(target.recordKind, target.id)
            : service.page(
                target.kind === "pending" ? "workflow-intents" : target.kind,
                { limit: 50, ...(target.query ? { q: target.query } : {}) },
              );
    void read
      .then((result: any) => {
        if (live) setData(result);
      })
      .catch((cause: unknown) => {
        if (live) setError(errorMessage(cause));
      })
      .finally(() => {
        if (live) setLoading(false);
      });
    return () => {
      live = false;
    };
  }, [w, target.kind, target.id, target.recordKind, target.query, revision]);
  const title =
    target.kind === "request"
      ? "原请求详情"
      : target.kind === "task"
        ? "原任务状态"
        : target.kind === "pending"
          ? "后台任务与待处理"
          : "运行记录";
  const rows = Array.isArray(data?.items) ? data.items : data ? [data] : [];
  return (
    <div className="grid gap-5">
      <div className="flex flex-wrap items-center gap-3">
        <Button variant="outline" asChild>
          <a href="#/diag">返回管理中心</a>
        </Button>
        <Button
          variant="outline"
          disabled={loading}
          onClick={() => refresh((value) => value + 1)}
        >
          重新读取
        </Button>
      </div>
      {loading && <ReadState role="status">正在读取原记录…</ReadState>}
      {error && (
        <p role="alert" className="text-destructive">
          {error}
        </p>
      )}
      {!loading && !error && !rows.length && <p>当前没有对应记录。</p>}
      {rows.map((row: any, index: number) => {
        const value = row.request || row.data || row;
        return (
          <Card key={row.id || row.ref || index}>
            <CardHeader>
              <CardTitle>{value.title || value.label || title}</CardTitle>
              {value.status && (
                <Badge variant="secondary">
                  {stateLabel(value.status, "状态待确认")}
                </Badge>
              )}
            </CardHeader>
            <CardContent className="grid gap-3">
              {(row.occurred_at || value.created_at) && (
                <p>{userInstant(row.occurred_at || value.created_at)}</p>
              )}
              {(value.reason || value.summary || value.error_message) && (
                <p>{value.reason || value.summary || value.error_message}</p>
              )}
              <Disclosure title="原始记录与技术详情">
                <pre className="approved-json whitespace-pre-wrap break-words">
                  {JSON.stringify(row, null, 2)}
                </pre>
              </Disclosure>
            </CardContent>
          </Card>
        );
      })}
    </div>
  );
}

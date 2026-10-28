import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { useWorkspace } from "../page";
import { useCursorPage } from "../hooks/use-cursor-page";
import CursorControls from "../shared/CursorControls";
import MaterialRetention from "../shared/MaterialRetention";
import JsonDetails from "../shared/JsonDetails";
import LoadingState from "../shared/LoadingState";
import { errorMessage } from "@/v3/domain-presentation.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";
export default function StudioMaterialPage({
  id,
  embedded = false,
}: {
  id: string;
  embedded?: boolean;
}) {
  const w = useWorkspace(),
    owner = w.state.session.actorId,
    service = w.services.studioClient,
    [data, setData] = useState<any>(null),
    [error, setError] = useState(""),
    [open, setOpen] = useState(false),
    [revision, refresh] = useState(0),
    epoch = useRef(0);
  const history = useCursorPage(
    (cursor) => service.materialHistory(id, cursor),
    JSON.stringify([owner, id, revision]),
  );
  useEffect(() => {
    const token = ++epoch.current;
    let active = true;
    void service
      .material(id)
      .then((value: any) => {
        if (
          !active ||
          token !== epoch.current ||
          w.state.session?.actorId !== owner
        )
          return;
        if (
          value.id !== id ||
          (value.owner_actor_id && value.owner_actor_id !== owner)
        )
          throw Error("创作素材范围不匹配");
        setData(value);
        setError("");
      })
      .catch((cause: any) => {
        if (active && token === epoch.current) setError(errorMessage(cause));
      });
    return () => {
      active = false;
      epoch.current++;
    };
  }, [service, id, owner, revision]);
  return (
    <section className="replica-studio r-form-stack">
      {!embedded && (
        <Button
          type="button"
          variant="outline"
          className="w-fit"
          onClick={() =>
            w.navigatePath(
              data
                ? "/studio/edit/" +
                    encodeURIComponent(data.resource_id) +
                    "?job=" +
                    encodeURIComponent(data.job_id)
                : "/studio",
            )
          }
        >
          返回原资源与任务
        </Button>
      )}
      {error && (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      {!data && !error && <LoadingState label="正在读取创作素材…" />}
      {data && (
        <>
          <Card>
            <CardHeader>
              <CardTitle>{data.label || "创作素材"}</CardTitle>
              <Badge variant="secondary">
                {data.released_at ? "已取消保留" : "当前保留为资源素材"}
              </Badge>
            </CardHeader>
            <CardContent className="grid gap-4">
              {data.text && (
                <p className="whitespace-pre-wrap break-words">{data.text}</p>
              )}
              {data.asset_id && (
                <Button
                  type="button"
                  variant="link"
                  onClick={() =>
                    w.navigatePath(
                      "/media/" + encodeURIComponent(data.asset_id),
                    )
                  }
                >
                  查看图片与实际引用
                </Button>
              )}
              <Button
                type="button"
                variant="outline"
                onClick={() => setOpen(true)}
              >
                {data.released_at ? "恢复保留" : "取消保留"}
              </Button>
              <JsonDetails
                value={{
                  job_id: data.job_id,
                  source_digest: data.source_digest,
                  created_at: data.created_at,
                }}
                title="原始来源"
              />
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>保留状态修订</CardTitle>
            </CardHeader>
            <CardContent className="grid gap-3">
              {history.items.map((row: any, index: number) => (
                <div key={row.id || index}>
                  <p>
                    版本 {row.revision} · {userInstant(row.created_at)}
                  </p>
                  <p>{row.reason}</p>
                  <JsonDetails value={row} />
                </div>
              ))}
              {!history.items.length && !history.loading && (
                <p>尚无已记录的保留状态修订。</p>
              )}
              <CursorControls page={history} label="素材历史" />
            </CardContent>
          </Card>
          {open && (
            <MaterialRetention
              material={data}
              open
              onClose={() => setOpen(false)}
              onSaved={(value: any) => {
                setData(value);
                refresh((value) => value + 1);
              }}
            />
          )}
        </>
      )}
    </section>
  );
}

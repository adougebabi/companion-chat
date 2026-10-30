import { Trace, TraceLine, TraceTitle, TraceIcon } from "@/components/ui/trace";
import { Spinner } from "@/components/ui/spinner";
import { CheckIcon, CircleIcon, AlertCircleIcon } from "lucide-react";
import { InlineFeedback } from "./Presentation";
import { ArrowUpRightIcon as ExternalLinkIcon } from "lucide-react";
import { Progress } from "@/components/ui/progress";
import { Badge } from "@/components/ui/badge";
import { Disclosure } from "../controls";
import { ActionButton } from "@/react-app/controls";
import { displayText, listValues, Fragment } from "@/react-app/render-helpers";
function linkedTaskId(href) {
  const match = /^#\/diag\/task\/([^/?#]+)$/.exec(href || "");
  if (!match) return null;
  try { return decodeURIComponent(match[1]) || null; } catch { return null; }
}
function StageActions({stage, props}) {
  const taskId = linkedTaskId(stage.href);
  return (
    <div className="row">
      {stage.state === "failed" && stage.configSection ? (
        <ActionButton variant="ghost" asChild key={0}>
          <a
            key={0}

            href={
              "#/settings?section=" +
              encodeURIComponent(stage.configSection) + "&returnTo=" + encodeURIComponent(globalThis.location?.hash || "#/library")
            }
          >
            {"去配置中心 "}
            <ExternalLinkIcon
              className="inline size-3.5 shrink-0"
              aria-hidden="true"
            />
          </a>
        </ActionButton>
      ) : null}
      {stage.retryable && stage.state !== 'blocked' && typeof props.onRecover==='function' ? (
        <ActionButton
          variant={stage.state==='failed'?'default':'outline'}
          key={1}
          type="button"

          disabled={props.busy}
          onClick={($event) => props.onRecover?.(stage)}
        >
          {displayText(props.busy ? "处理中…" : stage.state === "failed" ? "重试此环节" : "恢复原任务")}
        </ActionButton>
      ) : null}
      {taskId && typeof props.onOpenTask === "function" ? (
        <ActionButton variant="ghost" type="button"
          onClick={(event) => props.onOpenTask(taskId, event.currentTarget)}>
          {stage.linkLabel || "查看任务记录"}
        </ActionButton>
      ) : stage.href ? (
        <ActionButton variant="ghost" asChild key={2}>
          <a
            key={2}
            href={
              (stage.href.startsWith('#/diag/task/')?'#/diag?view=tasks&taskId='+encodeURIComponent(stage.href.slice('#/diag/task/'.length)):stage.href) +
              (stage.href.includes("?")||stage.href.startsWith('#/diag/task/') ? "&" : "?") +
              "returnTo=" +
              encodeURIComponent(
                globalThis.location?.hash || "#/library",
              )
            }
          >
            {stage.linkLabel || (stage.href.startsWith('#/diag/task/')?'查看任务记录':'查看准备记录')}
          </a>
        </ActionButton>
      ) : null}
    </div>
  );
}
export default function PreparationSteps(input = {}) {
  const __props = {
    stages: [],
    busy: false,
    ...input,
  };
  const blocker = __props.stages.find(stage => stage.state === 'failed') || __props.stages.find(stage => stage.state === 'blocked');
  const labels = {
    ready: "已完成",
    running: "进行中",
    pending: "等待中",
    failed: "失败",
    blocked: "等待前序恢复",
    unknown: "状态待确认",
  };
  return (
    <section
      className={__props.replica ? "creation-actual-preparation" : "grid gap-4"}
    >
      <Progress
        value={
          __props.stages.length
            ? (__props.stages.filter((stage) => stage.state === "ready")
                .length /
                __props.stages.length) *
              100
            : 0
        }
        aria-label="已读回确认的准备环节"
      />
      {!__props.replica && <p className="text-sm text-muted-foreground">
        已完成{" "}
        {__props.stages.filter((stage) => stage.state === "ready").length} /{" "}
        {__props.stages.length} 个准备环节
      </p>}
      <Trace
        className={__props.replica ? "preparation-list" : undefined}
        role="list"
        aria-label="准备相遇的实际进度"
      >
        <Fragment>
          {listValues(__props.stages).map((stage) => {
            return (
              <div
                role="listitem"
                className={
                  __props.replica
                    ? "preparation-item " + stage.state
                    : "grid gap-3 rounded-lg border border-border p-4"
                }
                key={stage.key}
                data-testid={"prep-step-" + stage.key}
                data-state={labels[stage.state] || "状态待确认"}
              >
                <div className="grid min-w-0 gap-3">
                  <div className="flex min-w-0 flex-wrap items-center justify-between gap-2">
                    <TraceLine
                      status={
                        stage.state === "failed"
                          ? "error"
                          : stage.state === "running"
                            ? "running"
                            : stage.state === "ready"
                              ? undefined
                              : "pending"
                      }
                    >
                      <TraceIcon>
                        {stage.state === "running" ? (
                          <Spinner />
                        ) : stage.state === "ready" ? (
                          <CheckIcon />
                        ) : stage.state === "failed" ? (
                          <AlertCircleIcon />
                        ) : (
                          <CircleIcon />
                        )}
                      </TraceIcon>
                      <TraceTitle className="whitespace-normal">
                        {stage.title}
                      </TraceTitle>
                    </TraceLine>
                    <Badge
                      variant={
                        stage.state === "failed" && !__props.replica ? "destructive" : "secondary"
                      }
                      role="status"
                      className={__props.replica && stage.state === 'failed' ? 'creation-needs-attention' : undefined}
                    >
                      {__props.replica && stage.state === 'failed' ? '需处理' : labels[stage.state] || "状态待确认"}
                    </Badge>
                  </div>
                  {(!__props.replica || stage.state !== 'blocked' && stage !== blocker) && stage.note &&
                    stage.note !== (labels[stage.state] || "状态待确认") && (
                      __props.replica && ['ready','running','pending'].includes(stage.state) ? <p className="creation-prep-note">{displayText(stage.note)}</p> : <InlineFeedback
                        kind={stage.state==='failed'?'error':stage.state==='unknown'?'action-required':stage.state==='running'||stage.state==='pending'?'progress':'information'}
                        className={
                          stage.state === "failed"
                            ? ""
                            : "border-0 bg-transparent p-0"
                        }
                      >
                        {displayText(stage.note)}
                      </InlineFeedback>
                    )}
                  {(!__props.replica || stage.state !== 'blocked' && stage !== blocker) && <StageActions stage={stage} props={__props}/>}
                </div>
              </div>
            );
          })}
        </Fragment>
      </Trace>
      {__props.replica && blocker && <div className="creation-preparation-blocker">
        <InlineFeedback className="creation-preparation-feedback" kind={blocker.state === 'failed' ? 'error' : 'information'}>
          {blocker.note || `「${blocker.title}」需要处理，后续环节等待恢复。`}
        </InlineFeedback>
        <StageActions stage={blocker} props={__props}/>
      </div>}
      {__props.replica && __props.stages.some(stage => stage.state === 'blocked' && stage !== blocker && stage.href) && <Disclosure title="其他准备记录">
        {__props.stages.filter(stage => stage.state === 'blocked' && stage !== blocker && stage.href).map(stage => <div key={stage.key}><p>{stage.title}</p>{stage.note && <p className="text-sm text-muted-foreground">{stage.note}</p>}<StageActions stage={stage} props={__props}/></div>)}
      </Disclosure>}
    </section>
  );
}

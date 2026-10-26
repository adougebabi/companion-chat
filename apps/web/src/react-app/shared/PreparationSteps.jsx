import { cn } from "@/lib/utils";
import { ActionButton } from "@/react-app/controls";
import { displayText, listValues, Fragment } from "@/react-app/render-helpers";
export default function PreparationSteps(input = {}) {
  const __props = {
    busy: false,
    ...input,
  };
  const labels = {
    ready: "已完成",
    running: "进行中",
    pending: "等待中",
    failed: "失败",
    blocked: "等待前序恢复",
    unknown: "状态待确认",
  };
  return (
    <ol className="preparation-list">
      <Fragment>
        {listValues(__props.stages).map((stage, index) => {
          return (
            <li
              key={stage.key}
              data-testid={"prep-step-" + stage.key}
              data-state={labels[stage.state] || "状态待确认"}
            >
              <span className="preparation-number">
                {displayText(stage.state === "ready" ? "✓" : index + 1)}
              </span>
              <div className="preparation-copy">
                <div className="row">
                  <b>{displayText(stage.title)}</b>
                  <span className="spacer" />
                  <span
                    className={cn([
                      "preparation-state",
                      {
                        "is-failed": stage.state === "failed",
                        "is-current": stage.state === "running",
                      },
                    ])}
                    role="status"
                  >
                    {displayText(labels[stage.state] || "状态待确认")}
                  </span>
                </div>
                <p
                  className={cn(
                    stage.state === "failed" ? "inline-error" : "muted",
                  )}
                >
                  {displayText(stage.note)}
                </p>
                <div className="row">
                  {stage.state === "failed" && stage.configSection ? (
                    <ActionButton asChild className="text-button" key={0}>
                      <a
                        key={0}
                        className="text-button"
                        href={
                          "#/settings?section=" +
                          encodeURIComponent(stage.configSection)
                        }
                      >
                        {"去配置中心 ↗"}
                      </a>
                    </ActionButton>
                  ) : null}
                  {stage.retryable ? (
                    <ActionButton
                      key={1}
                      type="button"
                      className="button"
                      disabled={__props.busy}
                      onClick={($event) => __props.onRecover?.(stage)}
                    >
                      {displayText(__props.busy ? "处理中…" : "恢复此环节")}
                    </ActionButton>
                  ) : null}
                  {stage.href ? (
                    <ActionButton asChild className="text-button" key={2}>
                      <a key={2} className="text-button" href={stage.href}>
                        {"查看原任务与结果 ↗"}
                      </a>
                    </ActionButton>
                  ) : null}
                </div>
              </div>
            </li>
          );
        })}
      </Fragment>
    </ol>
  );
}

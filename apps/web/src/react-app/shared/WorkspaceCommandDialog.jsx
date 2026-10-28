import {
  useEffect,
  useMemo,
  useSyncExternalStore,
  useId,
  useState,
} from "react";
import { Button } from "@/components/ui/button";
import { createDomainCommand } from "@/state/domain-command.mjs";
import { useWorkspace, afterPaint } from "../page";
import WorkspaceDialog from "./WorkspaceDialog";
import { InlineFeedback } from "./Presentation";
import JsonDetails from "./JsonDetails";
import ConfirmationFrame from "./ConfirmationFrame";
import {
  AlertDialogAction,
  AlertDialogCancel,
} from "@/components/ui/alert-dialog";
function InlineCommand({ title, children, footer, className = "" }) {
  return (
    <section
      data-slot="workspace-command-inline"
      className={"replica-inline-command lp-stack " + className}
      aria-label={title}
    >
      {children}
      <div className="lp-action-row">{footer}</div>
    </section>
  );
}
export default function WorkspaceCommandDialog({
  action,
  onClose,
  returnFocus,
}) {
  const w = useWorkspace(),
    owner = w.state.session.actorId,
    formId = useId(),
    inline = action.presentation === "inline",
    Frame = inline ? InlineCommand : WorkspaceDialog;
  const model = useMemo(
    () =>
      createDomainCommand({
        ...action,
        owner,
        ownerCurrent: () => w.state.session?.actorId === owner,
      }),
    [owner, action.identity],
  );
  const state = useSyncExternalStore(
    model.subscribe,
    model.getSnapshot,
    model.getSnapshot,
  );
  const [discardOpen, setDiscardOpen] = useState(false);
  useEffect(() => {
    model.start();
    return () => model.dispose();
  }, [model]);
  useEffect(() => {
    if (state.finished && !action.keepOpen) {
      onClose();
      if (inline)
        void afterPaint(() => {
          const target = returnFocus?.();
          if (target?.isConnected) target.focus({ preventScroll: true });
        });
    }
  }, [state.finished]);
  const discardAndClose = () => {
    if (action.resetDraft) {
      model.edit(action.resetDraft(state.draft));
      return;
    }
    model.discardDraft();
    onClose();
    if (inline)
      void afterPaint(() => {
        const target = returnFocus?.();
        if (target?.isConnected) target.focus({ preventScroll: true });
      });
  };
  const dismiss = () => {
    let dirty = false;
    if (action.confirmDiscard) {
      try {
        dirty = action.isDirty
          ? action.isDirty(state.draft)
          : JSON.stringify(state.draft) !== JSON.stringify(action.initial);
      } catch {
        dirty = true;
      }
    }
    if (
      dirty &&
      !state.busy &&
      !state.pending &&
      !state.readbackPending &&
      !state.accepted
    ) {
      setDiscardOpen(true);
      return;
    }
    discardAndClose();
  };
  return (
    <>
      <Frame
        open
        wide={action.wide}
        className={action.className}
        returnFocus={returnFocus}
        onOpenChange={(open) => {
          if (!open) dismiss();
        }}
        title={action.title}
        description={
          action.description ||
          "核对后提交；未确认的原请求和已接受的回执会保留。"
        }
        footer={
          <>
            <Button variant="outline" onClick={dismiss}>
              {action.cancelLabel || "取消"}
            </Button>
            <Button
              type="submit"
              form={formId}
              disabled={
                state.busy ||
                !!state.pending ||
                state.readbackPending ||
                state.needsRebase ||
                state.finished ||
                action.disabled ||
                (action.canSubmit && !action.canSubmit(state.draft))
              }
            >
              {state.busy ? "正在处理…" : action.submitLabel || "确认提交"}
            </Button>
          </>
        }
      >
        <div className="lp-stack">
          {state.error && (
            <InlineFeedback role="alert">{state.error}</InlineFeedback>
          )}
          {state.notice && (
            <InlineFeedback variant="default" role="status">
              {state.notice}
            </InlineFeedback>
          )}
          {state.draft && (
            <form
              id={formId}
              className="lp-form"
              onSubmit={(event) => {
                event.preventDefault();
                void model.submit();
              }}
            >
              {action.fields(state.draft, model.edit)}
            </form>
          )}
          <div className="lp-action-row">
            {state.pending && !state.pending.rejection && (
              <Button
                variant="outline"
                disabled={state.busy || action.disabled}
                onClick={() => void model.submit(true)}
              >
                核对或重试原提交
              </Button>
            )}
            {action.readLatest && (
              <Button
                variant="outline"
                disabled={
                  state.busy ||
                  state.readbackPending ||
                  (!!state.pending && !state.pending.rejection)
                }
                onClick={() => void model.reviewLatest()}
              >
                读取最新版本并保留输入
              </Button>
            )}
            {state.readbackPending && (
              <Button
                variant="outline"
                disabled={state.busy}
                onClick={() => void model.finishReadback()}
              >
                只读恢复页面结果
              </Button>
            )}
          </div>
          {state.pending && (
            <JsonDetails value={state.pending.value} title="原提交内容" />
          )}
          {state.review && (
            <section className="lp-stack">
              <h3>核对最新对象与版本</h3>
              <JsonDetails value={state.review.object} title="最新对象与来源" />
              <Button
                disabled={state.busy}
                onClick={() => model.confirmLatest()}
              >
                已核对，采用最新基线
              </Button>
            </section>
          )}
        </div>
      </Frame>
      {discardOpen && (
        <ConfirmationFrame
          title={action.discardTitle || "放弃这次资料修改？"}
          description="未保存的修改将被放弃，已经保存的资料保持不变。"
          onClose={() => setDiscardOpen(false)}
        >
          <AlertDialogCancel>继续编辑</AlertDialogCancel>
          <AlertDialogAction
            onClick={() => {
              setDiscardOpen(false);
              discardAndClose();
            }}
          >
            放弃修改
          </AlertDialogAction>
        </ConfirmationFrame>
      )}
    </>
  );
}

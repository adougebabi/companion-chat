import {
  useEffect,
  useMemo,
  useSyncExternalStore,
  useId,
  useState,
  useRef,
} from "react";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import { createDomainCommand } from "@/state/domain-command.mjs";
import { useWorkspace, afterPaint } from "../page";
import WorkspaceDialog from "./WorkspaceDialog";
import { InlineFeedback } from "./Presentation";
import JsonDetails from "./JsonDetails";
import ConfirmationFrame from "./ConfirmationFrame";
import CommandSyncAction from './CommandSyncAction';
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
  const [discardOpen, setDiscardOpen] = useState(false),
    [editedError, setEditedError] = useState(""),
    reviewHeading=useRef(null);
  useEffect(()=>{if(state.review)void afterPaint(()=>{if(reviewHeading.current?.isConnected){reviewHeading.current.scrollIntoView({block:'nearest'});reviewHeading.current.focus({preventScroll:true});}});},[state.review]);
  const syncAvailable=action.readLatest&&(!action.canReviewLatest||action.canReviewLatest(state.draft));
  const recoveryMessage = state.readbackPending
    ? "原操作已有回执，请只读恢复页面结果。"
    : state.pending && !state.pending.rejection
      ? "上一次提交的结果尚未确认，请先核对或重试原提交。"
      : state.needsRebase || state.pending?.rejection
        ? "请先同步并核对最新资料，当前输入仍会保留。"
        : "";
  const visibleError = state.error && state.error !== editedError ? state.error : "";
  const editDraft = (values) => {
    if (!state.pending && !state.readbackPending && !state.needsRebase)
      setEditedError(state.error);
    model.edit(values);
  };
  const submitCommand = async (retry = false) => {
    setEditedError("");
    await model.submit(retry);
    if (model.live() && model.state.error) toast.error(model.state.error);
  };
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
          <div className="replica-command-footer">
            {(visibleError || recoveryMessage) && (
              <InlineFeedback kind={state.readbackPending||state.pending&&!state.pending.rejection?'action-required':state.needsRebase||state.pending?.rejection?'warning':'error'}>
                {visibleError || recoveryMessage}
              </InlineFeedback>
            )}
            <div className="replica-command-footer-actions">
              {state.pending && !state.pending.rejection && (
                <Button
                  variant="outline"
                  disabled={state.busy || action.disabled}
                  onClick={() => void submitCommand(true)}
                >
                  核对或重试原提交
                </Button>
              )}
              {syncAvailable&&<CommandSyncAction state={state} onReview={model.reviewLatest} placement={state.needsRebase||state.pending?.rejection?'recovery':'footer'}/>}
              {state.readbackPending && (
                <Button
                  variant="outline"
                  disabled={state.busy}
                  onClick={() => void model.finishReadback()}
                >
                  只读恢复页面结果
                </Button>
              )}
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
            </div>
          </div>
        }
      >
        <div className="lp-stack">
          {state.notice && !visibleError && (
            <InlineFeedback kind="information">
              {state.notice}
            </InlineFeedback>
          )}
          {state.draft && (
            <form
              id={formId}
              className="lp-form"
              onSubmit={(event) => {
                event.preventDefault();
                void submitCommand();
              }}
            >
              {action.fields(state.draft, editDraft)}
            </form>
          )}
          {state.pending && (
            <JsonDetails value={state.pending.value} title="原提交内容" />
          )}
          {state.review && (
            <section className="lp-stack">
              <h3 ref={reviewHeading} tabIndex={-1}>核对最新对象与版本</h3>
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

import * as React from "react";
import { Button } from "@/components/ui/button";
import { ResponsiveSheet, type ResponsiveSheetProps } from "@/components/ui/responsive-sheet";
import { Feedback } from "@/react-app/shared/Presentation";

/**
 * DetailSheet — read-only record view. Viewing and editing are distinct
 * actions (.impeccable.md); a DetailSheet never submits.
 */
export function DetailSheet({
  description = "查看当前记录与相关信息",
  hideDescription = true,
  ...props
}: ResponsiveSheetProps) {
  return <ResponsiveSheet description={description} hideDescription={hideDescription} {...props} />;
}

/**
 * EditSheet — form in a ResponsiveSheet with a pinned cancel/submit footer.
 * The form element owns submission (Enter submits, FormData is native);
 * while `busy` the sheet cannot be dismissed and the submit is locked. An
 * `error` is shown at the top of the body and receives focus.
 */
export function EditSheet({
  onSubmit,
  submitLabel = "保存",
  busyLabel = "正在保存…",
  cancelLabel = "取消",
  submitDisabled = false,
  busy = false,
  error,
  extraActions,
  children,
  onOpenChange,
  onOpenAutoFocus,
  description = "修改后保存；取消不会提交任何内容。",
  ...props
}: Omit<ResponsiveSheetProps, "footer"> & {
  onSubmit: (data: FormData, event: React.FormEvent<HTMLFormElement>) => void;
  submitLabel?: string;
  busyLabel?: string;
  cancelLabel?: string;
  submitDisabled?: boolean;
  error?: React.ReactNode;
  extraActions?: React.ReactNode;
}) {
  const formId = React.useId();
  const errorRef = React.useRef<HTMLDivElement>(null);
  React.useEffect(() => {
    if (error) errorRef.current?.focus({ preventScroll: false });
  }, [error]);
  return (
    <ResponsiveSheet
      {...props}
      description={description}
      busy={busy}
      onOpenChange={onOpenChange}
      onOpenAutoFocus={(event) => {
        // An error present on open is the first thing to read.
        if (error) { event.preventDefault(); errorRef.current?.focus(); }
        onOpenAutoFocus?.(event);
      }}
      footer={
        <>
          {extraActions}
          <Button type="button" variant="outline" disabled={busy} onClick={() => onOpenChange?.(false)}>
            {cancelLabel}
          </Button>
          <Button type="submit" form={formId} disabled={busy || submitDisabled} aria-busy={busy || undefined}>
            {busy ? busyLabel : submitLabel}
          </Button>
        </>
      }
    >
      <form
        id={formId}
        noValidate={false}
        className="ui-stack ui-edit-form"
        onSubmit={(event) => {
          event.preventDefault();
          if (busy || submitDisabled) return;
          if (!event.currentTarget.reportValidity()) return;
          onSubmit(new FormData(event.currentTarget), event);
        }}
      >
        {error && (
          <div ref={errorRef} tabIndex={-1} className="outline-none">
            <Feedback kind="error">{error}</Feedback>
          </div>
        )}
        <fieldset disabled={busy} className="ui-stack ui-edit-form__fields">
          {children}
        </fieldset>
      </form>
    </ResponsiveSheet>
  );
}

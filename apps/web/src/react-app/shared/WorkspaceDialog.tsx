import type { ReactNode } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
export default function WorkspaceDialog({
  open,
  onOpenChange,
  title,
  description,
  children,
  footer,
  wide = false,
  returnFocus,
  busy = false,
  className = "",
  testId,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description?: ReactNode;
  children: ReactNode;
  footer?: ReactNode;
  wide?: boolean;
  returnFocus?: () => HTMLElement | null;
  busy?: boolean;
  className?: string;
  testId?:string;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        data-testid={testId}
        showCloseButton={!busy}
        onEscapeKeyDown={(event) => {
          if (busy) event.preventDefault();
        }}
        onInteractOutside={(event) => {
          if (busy) event.preventDefault();
        }}
        onCloseAutoFocus={(event) => {
          const source = returnFocus?.();
          const target =
            source?.isConnected &&
            !source.matches(':disabled,[aria-disabled="true"]')
              ? source
              : document.querySelector<HTMLElement>(".replica-page-heading h1");
          if (target?.isConnected) {
            event.preventDefault();
            target.focus({ preventScroll: true });
          }
        }}
        className={
          "react-workspace replica-dialog" +
          (wide ? " replica-dialog-wide" : "") +
          " " +
          className
        }
      >
        <DialogHeader className="text-center sm:text-left">
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>
            {description || "查看详细信息与相关操作。"}
          </DialogDescription>
        </DialogHeader>
        <div className="replica-dialog-body">{children}</div>
        {footer && <DialogFooter>{footer}</DialogFooter>}
      </DialogContent>
    </Dialog>
  );
}

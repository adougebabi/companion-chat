import * as React from "react";
import { cn } from "@/lib/utils";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";

/**
 * ResponsiveSheet — the single layout pattern for complex settings/edit forms.
 *
 * Geometry is owned here and in `styles/overlay.css` (selector
 * `[data-responsive-sheet]`). Business CSS must not override width, height,
 * padding or overflow of the sheet or its parts; see `.impeccable.md`.
 *
 * - Phone (< 640px): full viewport width and `100dvh`; close via the 44px close
 *   button or Escape.
 * - Tablet/desktop: right side sheet, `--overlay-sheet-width` (or `-wide`),
 *   never wider than the viewport.
 * - Header and footer never scroll; the body is the only scroller.
 */
export type ResponsiveSheetSize = "default" | "wide";

export type ResponsiveSheetProps = {
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  /** Optional trigger element rendered with `asChild`. */
  trigger?: React.ReactElement;
  title: React.ReactNode;
  description?: React.ReactNode;
  /** Keep the description for assistive technology only. */
  hideDescription?: boolean;
  footer?: React.ReactNode;
  size?: ResponsiveSheetSize;
  portalContainer?: React.ComponentProps<typeof SheetContent>["portalContainer"];
  className?: string;
  bodyClassName?: string;
  bodyRef?: React.Ref<HTMLDivElement>;
  children?: React.ReactNode;
} & Pick<
  React.ComponentProps<typeof SheetContent>,
  "onCloseAutoFocus" | "onOpenAutoFocus" | "onEscapeKeyDown" | "onInteractOutside"
>;

export function ResponsiveSheet({
  open,
  onOpenChange,
  trigger,
  title,
  description,
  hideDescription = false,
  footer,
  size = "default",
  portalContainer,
  className,
  bodyClassName,
  bodyRef,
  children,
  ...contentProps
}: ResponsiveSheetProps) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      {trigger && <SheetTrigger asChild>{trigger}</SheetTrigger>}
      <SheetContent
        side="right"
        portalContainer={portalContainer}
        data-responsive-sheet=""
        data-size={size}
        className={cn("responsive-sheet", className)}
        {...contentProps}
      >
        <SheetHeader data-responsive-sheet-part="header">
          <SheetTitle>{title}</SheetTitle>
          <SheetDescription className={hideDescription ? "sr-only" : undefined}>
            {description ?? "查看资料并完成当前操作。"}
          </SheetDescription>
        </SheetHeader>
        <div
          ref={bodyRef}
          data-responsive-sheet-part="body"
          className={cn("min-w-0", bodyClassName)}
        >
          {children}
        </div>
        {footer && (
          <SheetFooter data-responsive-sheet-part="footer">{footer}</SheetFooter>
        )}
      </SheetContent>
    </Sheet>
  );
}

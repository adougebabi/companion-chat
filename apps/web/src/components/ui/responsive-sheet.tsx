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
  /** Keep title + description for assistive technology only (content renders its own heading). */
  hideHeader?: boolean;
  footer?: React.ReactNode;
  size?: ResponsiveSheetSize;
  /** Phone presentation; larger screens retain the side sheet. */
  mobileMode?: "page" | "sheet" | "dialog";
  portalContainer?: React.ComponentProps<typeof SheetContent>["portalContainer"];
  className?: string;
  bodyClassName?: string;
  bodyRef?: React.Ref<HTMLDivElement>;
  children?: React.ReactNode;
  /** While busy the sheet cannot be dismissed (Escape, outside, close). */
  busy?: boolean;
  showCloseButton?: boolean;
  closeTestId?: string;
  "data-testid"?: string;
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
  hideHeader = false,
  footer,
  size = "default",
  mobileMode = "page",
  portalContainer,
  className,
  bodyClassName,
  bodyRef,
  children,
  busy = false,
  showCloseButton = true,
  closeTestId,
  onEscapeKeyDown,
  onInteractOutside,
  onOpenAutoFocus,
  ...contentProps
}: ResponsiveSheetProps) {
  const titleRef = React.useRef<HTMLHeadingElement>(null);
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      {trigger && <SheetTrigger asChild>{trigger}</SheetTrigger>}
      <SheetContent
        side="right"
        portalContainer={portalContainer}
        data-responsive-sheet=""
        data-size={size}
        data-mobile-mode={mobileMode}
        aria-busy={busy || undefined}
        className={cn("responsive-sheet", className)}
        showCloseButton={showCloseButton && !busy}
        closeButtonProps={closeTestId ? ({ "data-testid": closeTestId } as React.ComponentProps<"button">) : undefined}
        onOpenAutoFocus={(event) => {
          onOpenAutoFocus?.(event);
          if (!event.defaultPrevented && window.matchMedia('(width < 640px)').matches) {
            event.preventDefault();
            titleRef.current?.focus();
          }
        }}
        onEscapeKeyDown={(event) => { if (busy) event.preventDefault(); onEscapeKeyDown?.(event); }}
        onInteractOutside={(event) => { if (busy) event.preventDefault(); onInteractOutside?.(event); }}
        {...contentProps}
      >
        <SheetHeader data-responsive-sheet-part="header" className={hideHeader ? "sr-only" : undefined}>
          <SheetTitle ref={titleRef} tabIndex={-1}>{title}</SheetTitle>
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

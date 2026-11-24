import * as React from "react";
import { cn } from "@/lib/utils";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

/**
 * ResponsiveTabsList — a single-row tab list that never wraps or clips tabs out
 * of reach. When the tabs exceed the container it scrolls horizontally, fades
 * the edge that hides more tabs and keeps the active tab fully visible.
 * Styles: `[data-responsive-tabs]` in styles/overlay.css.
 */
export function ResponsiveTabsList({
  className,
  value,
  ...props
}: React.ComponentProps<typeof TabsList> & { value?: string }) {
  const ref = React.useRef<HTMLDivElement>(null);
  React.useLayoutEffect(() => {
    const list = ref.current;
    if (!list) return;
    const update = () => {
      const max = list.scrollWidth - list.clientWidth;
      list.dataset.overflowStart = String(max > 1 && list.scrollLeft > 1);
      list.dataset.overflowEnd = String(max > 1 && list.scrollLeft < max - 1);
    };
    update();
    list.addEventListener("scroll", update, { passive: true });
    const observer = typeof ResizeObserver === "function" ? new ResizeObserver(update) : null;
    observer?.observe(list);
    return () => {
      list.removeEventListener("scroll", update);
      observer?.disconnect();
    };
  }, []);
  React.useLayoutEffect(() => {
    const list = ref.current;
    const active = list?.querySelector<HTMLElement>('[data-slot="tabs-trigger"][data-state="active"]');
    if (!list || !active) return;
    const start = active.offsetLeft - 8;
    const end = active.offsetLeft + active.offsetWidth + 8;
    if (start < list.scrollLeft) list.scrollLeft = start;
    else if (end > list.scrollLeft + list.clientWidth) list.scrollLeft = end - list.clientWidth;
  }, [value]);
  return (
    <TabsList
      ref={ref}
      data-responsive-tabs=""
      className={cn(
        "h-auto max-w-full flex-nowrap justify-start group-data-horizontal/tabs:h-auto",
        className,
      )}
      {...props}
    />
  );
}

export type ResponsiveTabItem = {
  value: string;
  label: React.ReactNode;
  disabled?: boolean;
  content: React.ReactNode;
};

/** ResponsiveTabs — tabs + panels for native React pages. */
export function ResponsiveTabs({
  label,
  items,
  value,
  defaultValue,
  onValueChange,
  className,
}: {
  label: string;
  items: ResponsiveTabItem[];
  value?: string;
  defaultValue?: string;
  onValueChange?: (value: string) => void;
  className?: string;
}) {
  const [internal, setInternal] = React.useState(defaultValue ?? items[0]?.value ?? "");
  const current = value ?? internal;
  return (
    <Tabs
      value={current}
      onValueChange={(next) => {
        if (value === undefined) setInternal(next);
        onValueChange?.(next);
      }}
      className={cn("min-w-0 w-full", className)}
    >
      <ResponsiveTabsList value={current} aria-label={label}>
        {items.map((item) => (
          <TabsTrigger key={item.value} value={item.value} disabled={item.disabled} className="shrink-0 flex-none whitespace-nowrap">
            {item.label}
          </TabsTrigger>
        ))}
      </ResponsiveTabsList>
      {items.map((item) => (
        <TabsContent key={item.value} value={item.value} className="min-w-0">
          {item.content}
        </TabsContent>
      ))}
    </Tabs>
  );
}

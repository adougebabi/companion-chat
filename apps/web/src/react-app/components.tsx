import {ToggleGroup,ToggleGroupItem} from '@/components/ui/toggle-group';
import {Alert,AlertDescription} from '@/components/ui/alert';
import {toast} from '@/v3/proto/components/toast.js';
import * as React from "react";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from "@/components/ui/sheet";
import { ActionButton, FieldInput, Disclosure } from "./controls";
import { useWorkspace } from "./page";
import { renderGarmentArt } from "./renderers/GarmentArt";
import { selector } from "../state/view-model";

export function CurrentOutfit({ compact = false }: { compact?: boolean }) {
  const w = useWorkspace();
  const state = React.useSyncExternalStore<{
    errors: Record<string, string>;
    loading: Record<string, boolean>;
  }>(w.subscribe, w.getSnapshot, w.getSnapshot);
  const character = w.current.value;
  const status = state.errors.wardrobe
    ? "穿着读取未完成，保留上次确认的记录。"
    : state.loading.wardrobe
      ? "正在读取实际穿着…"
      : "";
  const names = (slot: string) =>
    !Array.isArray(character._wardrobe?.wearing?.worn_items)
      ? "状态待读取"
      : (character.wearing[slot] || [])
          .map(
            (id: string) =>
              character.garments.find((item: any) => item.id === id)?.name ||
              character._wardrobe.wearing.worn_items.find(
                (item: any) => (item.item_id || item.id) === id,
              )?.description ||
              "衣物名称待读取",
          )
          .filter(Boolean)
          .join("、") || "未登记";
  return (
    <section
      className={"outfit-block" + (compact ? " outfit-compact" : "")}
      data-testid="current-outfit"
    >
      <div className="row">
        <b>今日穿着</b>
        <span className="spacer" />
        <ActionButton
          className="text-button"
          onClick={() => {
            w.state.outfitTab = "为她搭配";
            w.navigate("衣柜");
          }}
        >
          去编辑 ↗
        </ActionButton>
      </div>
      {status && (
        <p className="muted" role="status">
          {status}
        </p>
      )}
      <div className={compact ? "outfit-tags" : "outfit-lines"}>
        {["外套", "上衣", "内衣", "内裤", "下衣", "袜子", "鞋子", "配饰"].map(
          (slot) => (
            <div key={slot}>
              <small>{slot}</small>
              <span>{names(slot)}</span>
            </div>
          ),
        )}
      </div>
    </section>
  );
}
export function ActionFeedback({
  labels,
  operation = "",
  excludeOperationPrefix = "",
  showStatus = false,
}: any) {
  const w = useWorkspace();
  React.useSyncExternalStore(w.subscribe, w.getSnapshot, w.getSnapshot);
  const latest = w.state.operations
    .filter(
      (r: any) =>
        r.fid === w.state.selectedId &&
        labels.includes(r.label) &&
        (!operation || r.operation === operation) &&
        (!excludeOperationPrefix ||
          !r.operation?.startsWith(excludeOperationPrefix)),
    )
    .at(-1);
  React.useEffect(()=>{if(latest?.requestId&&latest.status==='success'&&latest.response)toast(latest.response,'success');},[latest?.id,latest?.status,latest?.response]);
  if (!latest) return null;
  return (
    <>
      {showStatus && latest.status !== "error" && (
        <p className="muted" role="status" data-testid="request-status">
          {latest.status === "pending"
            ? "换装请求已受理，等待处理；当前穿着以实际记录为准。"
            : latest.response || "请求已处理，请核对实际结果。"}
        </p>
      )}
      {latest.status === "error" && (
        <Alert variant="destructive"><AlertDescription>
          {latest.reason}{" "}
          <ActionButton className="text-button" onClick={() => latest.run?.()}>
            {latest.actionText || "重试"}
          </ActionButton>
        </AlertDescription></Alert>
      )}
      {latest.readbackError && (
        <p className="inline-error" role="alert">
          {latest.readbackError}{" "}
          <ActionButton
            className="text-button"
            onClick={() => latest.refreshResult()}
          >
            重新读取页面结果
          </ActionButton>
        </p>
      )}
      {latest.originalInput && (
        <Disclosure title="查看等待核对的原提交" className="inline-error">
          <pre>{latest.originalInput}</pre>
        </Disclosure>
      )}
      {latest.review && (
        <div className="inline-error">
          <p>
            原提交已明确因版本冲突被拒绝，草稿仍保留。读取最新对象后，请重新核对变更。
          </p>
          <ActionButton className="button" onClick={() => latest.release()}>
            读取最新版本并保留草稿
          </ActionButton>
        </div>
      )}
    </>
  );
}
export function FilterChips({
  search = "",
  filters,
  groups,
  placeholder = "搜索…",
  testId,
  ...props
}: any) {
  const prefix = testId || props["test-id"];
  const choose = (key: string, value: string) =>
    props.onUpdateFilters?.({
      ...filters,
      [key]: filters[key] === value ? "" : value,
    });
  return (
    <section className="filter-chips" data-testid={prefix}>
      <div className="filter-search-row">
        <FieldInput
          value={search}
          type="search"
          placeholder={placeholder}
          aria-label={placeholder}
          data-testid={`${prefix}-search`}
          onChange={(e) => props.onUpdateSearch?.(e.target.value)}
        />
        {(search.trim() || Object.values(filters).some(Boolean)) && (
          <ActionButton
            type="button"
            className="text-button"
            data-testid={`${prefix}-clear`}
            onClick={() => {
              props.onUpdateSearch?.("");
              props.onUpdateFilters?.(
                Object.fromEntries(groups.map((g: any) => [g.key, ""])),
              );
              props.onClear?.();
            }}
          >
            清除
          </ActionButton>
        )}
      </div>
      {groups.map((group:any)=><div key={group.key} className="flex min-w-0 items-center gap-3"><small className="shrink-0">{group.label}</small><ToggleGroup type="single" variant="outline" aria-label={group.label} value={filters[group.key]?'option:'+group.options.indexOf(filters[group.key]):'all'} onValueChange={key=>{const value=key.startsWith('option:')?group.options[Number(key.slice(7))]:'';props.onUpdateFilters?.({...filters,[group.key]:value});}} className="max-w-full justify-start overflow-x-auto"><ToggleGroupItem value="all" data-testid={`${prefix}-${group.key}-all`}>全部</ToggleGroupItem>{group.options.map((value:string,index:number)=><ToggleGroupItem key={value} value={'option:'+index} data-testid={`${prefix}-${group.key}-${value}`}>{value}</ToggleGroupItem>)}</ToggleGroup></div>)}
    </section>
  );
}
export function DetailDrawer({
  title,
  testId,
  children,
  onClose,
  ...props
}: any) {
  const [previous] = React.useState(
    () => document.activeElement as HTMLElement | null,
  );
  const prefix = testId || props["test-id"];
  return (
    <Sheet
      open
      onOpenChange={(open) => {
        if (!open) onClose?.();
      }}
    >
      <SheetContent
        data-testid={prefix}
        className="react-workspace product-sheet overflow-y-auto"
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          (previous?.isConnected
            ? previous
            : document.querySelector<HTMLElement>("main")
          )?.focus({ preventScroll: true });
        }}
      >
        <SheetHeader>
          <SheetTitle>{title}</SheetTitle>
          <SheetDescription className="sr-only">
            查看和编辑当前对象
          </SheetDescription>
          <ActionButton
            type="button"
            className="text-button"
            data-testid={`${prefix}-close`}
            onClick={onClose}
          >
            关闭
          </ActionButton>
        </SheetHeader>
        <div className="px-6 pb-6">{children}</div>
      </SheetContent>
    </Sheet>
  );
}
export function GarmentArt(props: any) {
  return renderGarmentArt({ props })();
}

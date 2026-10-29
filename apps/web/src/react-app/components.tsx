import {InlineFeedback as OperationStatus} from "@/react-app/shared/Presentation";
import {InlineFeedback,Feedback} from "@/react-app/shared/Presentation";
import {ArrowUpRightIcon as ExternalLinkIcon,MoreHorizontalIcon as MoreActionsIcon} from "lucide-react";
import {ToggleGroup,ToggleGroupItem} from '@/components/ui/toggle-group';
import {Alert,AlertDescription} from '@/components/ui/alert';
import {toast} from '@/v3/proto/components/toast.js';
import {InputGroup,InputGroupAddon,InputGroupButton} from '@/components/ui/input-group';
import {SearchIcon,XIcon} from 'lucide-react';
import {ScrollArea} from '@/components/ui/scroll-area';
import * as React from "react";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,SheetFooter,
} from "@/components/ui/sheet";
import { ActionButton, FieldInput, Disclosure } from "./controls";
import { useWorkspace } from "./page";
import { renderGarmentArt } from "./renderers/GarmentArt";
import { selector } from "../state/view-model";

const announcedOperations=new WeakMap<object,Set<string>>();

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
        <ActionButton variant="ghost"

          onClick={() => {
            w.state.outfitTab = "为她搭配";
            w.navigate("衣柜");
          }}
        >
          去编辑 <ExternalLinkIcon className="inline size-3.5 shrink-0" aria-hidden="true"/>
        </ActionButton>
      </div>
      {status && (
        <OperationStatus variant="default" className="muted" role="status">
          {status}
        </OperationStatus>
      )}
      <div className={compact ? "outfit-tags" : "outfit-lines"}>
        {["外套", "上衣", "内衣", "内裤", "下衣", "袜子", "鞋子", "配饰"].map(
          (slot) => (
            <div key={slot}>
              <small>{slot}</small>
              <span data-empty={["未登记","状态待读取","衣物名称待读取"].includes(names(slot))}>{names(slot)}</span>
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
  React.useEffect(()=>{
    if(showStatus||!latest?.requestId||latest.restored||latest.readbackError||latest.requestUnresolved||latest.status!=='success'||!latest.response)return;
    const key=[w.state.session?.actorId,latest.fid,latest.id,latest.response].join(':'),seen=announcedOperations.get(w)||new Set<string>();
    if(seen.has(key))return;seen.add(key);announcedOperations.set(w,seen);
    toast(latest.response,latest.requestStatus==='accepted'?'info':'success');
  },[w,showStatus,latest?.id,latest?.status,latest?.response,latest?.readbackError,latest?.requestUnresolved]);
  if (!latest) return null;
  return (
    <>
      {showStatus && latest.status !== "error" && !latest.readbackError && !latest.review && (
        <OperationStatus kind={latest.status==='pending'||latest.readbackPending?'progress':latest.requestStatus==='accepted'?'information':'success'} className="muted" data-testid="request-status">
          {latest.admissionPending?'正在提交请求，等待实际回执。':latest.readbackPending?'原操作已有回执，正在读取实际结果…':latest.status === "pending"
            ? latest.operation === "persona-switch"
              ? "人格切换请求已受理，等待处理；当前人格以实际记录为准。"
              : latest.operation?.startsWith("wear-outfit:")
                ? "换装请求已受理，等待处理；当前穿着以实际记录为准。"
                : "请求已受理，等待处理；结果以实际记录为准。"
            : latest.response || "请求已处理，请核对实际结果。"}
        </OperationStatus>
      )}
      {latest.status === "error" && !latest.readbackError && !latest.review && (
        <Feedback kind={latest.outcomeUnknown||latest.requestUnresolved||latest.readbackPending?'action-required':latest.rejected?'warning':'error'} details={latest.originalInput&&<pre className="whitespace-pre-wrap break-words">{latest.originalInput}</pre>} detailsLabel="查看等待核对的原提交">
          {latest.reason}{" "}
          {typeof latest.run==='function'&&<ActionButton variant="ghost"  onClick={() => latest.run()}>
            {latest.actionText || "重试"}
          </ActionButton>}
        </Feedback>
      )}
      {latest.readbackError && (
        <Feedback kind={latest.status==='error'&&!latest.outcomeUnknown&&!latest.requestUnresolved&&!latest.readbackPending?'error':'action-required'} details={latest.originalInput&&<pre className="whitespace-pre-wrap break-words">{latest.originalInput}</pre>} detailsLabel="查看等待核对的原提交">
          {latest.status==='error'&&latest.reason&&<p>{latest.reason}{typeof latest.run==='function'&&<ActionButton variant="ghost" onClick={()=>latest.run()}>{latest.actionText||'查看原请求'}</ActionButton>}</p>}
          <p>{latest.readbackError}</p>
          <ActionButton variant="ghost"

            onClick={() => latest.refreshResult()}
          >
            重新读取页面结果
          </ActionButton>
        </Feedback>
      )}
      {latest.originalInput && latest.status!=="error" && !latest.readbackError && !latest.review && (
        <InlineFeedback kind="action-required">
          <Disclosure title="查看等待核对的原提交"><ScrollArea className="h-64"><pre className="whitespace-pre-wrap break-words">{latest.originalInput}</pre></ScrollArea></Disclosure>
        </InlineFeedback>
      )}
      {latest.review && (
        <Feedback kind="warning" details={latest.originalInput&&<pre className="whitespace-pre-wrap break-words">{latest.originalInput}</pre>} detailsLabel="查看等待核对的原提交">
          <p>
            原提交已明确因版本冲突被拒绝，草稿仍保留。读取最新对象后，请重新核对变更。
          </p>
          <ActionButton variant="outline"  onClick={() => latest.release()}>
            读取最新版本并保留草稿
          </ActionButton>
        </Feedback>
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
    <section className="grid min-w-0 gap-3" data-testid={prefix}>
      <InputGroup className="h-auto min-h-11">
        <InputGroupAddon><SearchIcon/></InputGroupAddon>
        <FieldInput
          data-slot="input-group-control"
          className="min-w-0 flex-1 border-0 bg-transparent shadow-none focus-visible:ring-0"
          value={search}
          type="search"
          placeholder={placeholder}
          aria-label={placeholder}
          data-testid={`${prefix}-search`}
          onChange={(e) => props.onUpdateSearch?.(e.target.value)}
        />
        {(search.trim() || Object.values(filters).some(Boolean)) && (
          <InputGroupAddon align="inline-end"><InputGroupButton
            type="button"

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
          </InputGroupButton></InputGroupAddon>
        )}
      </InputGroup>
      {groups.filter((group:any)=>group.options.length).map((group:any)=><div key={group.key} className="flex min-w-0 items-start gap-3"><small className="shrink-0 pt-2">{group.label}</small><ToggleGroup type="single" variant="outline" aria-label={group.label} value={filters[group.key]?'option:'+group.options.indexOf(filters[group.key]):'all'} onValueChange={key=>{const value=key.startsWith('option:')?group.options[Number(key.slice(7))]:'';props.onUpdateFilters?.({...filters,[group.key]:value});}} className="max-w-full flex-wrap justify-start gap-1"><ToggleGroupItem value="all" data-testid={`${prefix}-${group.key}-all`}>全部</ToggleGroupItem>{group.options.map((value:string,index:number)=><ToggleGroupItem key={value} value={'option:'+index} className="max-w-full whitespace-normal break-all" data-testid={`${prefix}-${group.key}-${value}`}>{value}</ToggleGroupItem>)}</ToggleGroup></div>)}
    </section>
  );
}
export function DetailDrawer({
  title,
  testId,
  children,
  onClose,
  returnFocus,footer,
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
        showCloseButton={false}
        className="react-workspace product-sheet gap-0 overflow-hidden"
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          const target = returnFocus?.() || previous;
          (target?.isConnected
            ? target
            : document.querySelector<HTMLElement>("main")
          )?.focus({ preventScroll: true });
        }}
      >
        <SheetHeader className="relative shrink-0 border-b pr-16">
          <SheetTitle>{title}</SheetTitle>
          <SheetDescription className="sr-only">
            查看当前记录与相关信息
          </SheetDescription>
          <ActionButton variant="ghost"
            type="button"
            size="icon"
            className="touch-target absolute right-3 top-3"
            aria-label="关闭"
            data-testid={`${prefix}-close`}
            onClick={onClose}
          >
            <XIcon aria-hidden="true"/>
          </ActionButton>
        </SheetHeader>
        <div className="min-h-0 flex-1 overflow-y-auto px-6 py-5">{children}</div>
        {footer&&<SheetFooter className="shrink-0 border-t border-border p-4 sm:flex-row sm:justify-end">{footer}</SheetFooter>}
      </SheetContent>
    </Sheet>
  );
}
export function GarmentArt(props: any) {
  return renderGarmentArt({ props })();
}

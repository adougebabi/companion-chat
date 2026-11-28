import { ResponsiveTabsList } from '@/components/patterns';
import { useEffect, useId, useRef, useState } from "react";
import {
  CameraIcon,
  CheckIcon,
  Edit3Icon,
  EyeIcon,
  PlusIcon,
  SearchIcon,
  ShirtIcon,
  SlidersHorizontalIcon,
  XIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { Tabs, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import {
  AlertDialogCancel,
  AlertDialogAction,
} from "@/components/ui/alert-dialog";
import {
  Surface,
  FieldInput,
  ChoiceSelect,
  Disclosure,
  MenuDisclosure,
  Badge,
} from "../controls";
import { useWorkspace } from "../page";
import { readValue, jsonValueSnapshot } from "@/state/view-model";
import { garmentView, slots } from "@/state/workspace.mjs";
import { errorMessage } from "@/v3/domain-presentation.mjs";
import { isInternalId } from "@/v3/internal-id.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";
import {
  clothingSeasons,
  clothingTypeOf,
  clothingTypeOptions,
  wardrobeAvailabilityOptions,
  wardrobeCategoryLabel,
  wardrobeOwnershipOptions,
} from "@/lib/wardrobe-form";
import WorkspacePageHeader from "../shared/WorkspacePageHeader";
import WorkspaceDialog from "../shared/WorkspaceDialog";
import WardrobeItemArt from "../shared/WardrobeItemArt";
import { mediaClient } from "@/v3/proto/services.js";
import WearingSummary from "../shared/WearingSummary";
import ConfirmationFrame from "../shared/ConfirmationFrame";
import JsonDetails from "../shared/JsonDetails";
import LoadingState from "../shared/LoadingState";
import { EmptyState, Feedback, InlineFeedback } from "../shared/Presentation";
import { currentReadError } from "@/approved-ui/read-feedback.mjs";
import { serviceUnavailableNotice, settingsHref } from "@/approved-ui/service-unavailable.mjs";
// Keep the page shell (header, tabs, filters) and explain a failed read inside the content area.
function WardrobeReadNotice({ w, error, cause, ready }) {
  const notice = serviceUnavailableNotice(cause, error),
    retry = (
      <Button
        type="button"
        variant="outline"
        disabled={!!w.state.loading?.wardrobe}
        onClick={() => void w.loadWardrobe().catch(() => {})}
      >
        重新读取
      </Button>
    ),
    settings = (
      <Button asChild variant={ready ? "ghost" : "default"}>
        <a href={settingsHref(notice.configSection)}>前往管理中心</a>
      </Button>
    );
  if (ready)
    return (
      <Feedback
        kind="warning"
        className="mb-4"
        title="衣柜暂未更新"
        actions={<>{retry}{settings}</>}
      >
        <p>{notice.reason}</p>
        <p>上次确认的衣服和方案仍保留。</p>
      </Feedback>
    );
  return (
    <EmptyState variant="page" role="alert" data-testid="wardrobe-read-error">
      <h3>衣柜暂时没有读到</h3>
      <p>{notice.reason}</p>
      <div className="flex flex-wrap justify-center gap-2">
        {retry}
        {settings}
      </div>
    </EmptyState>
  );
}
const tabKeys = {
  全部衣服: "garments",
  方案库: "outfits",
  为她搭配: "compose",
};
const types = {
  外套: "outer",
  上衣: "upper_body",
  内衣: "underwear",
  内裤: "panties",
  下衣: "lower_body",
  袜子: "socks",
  鞋子: "shoes",
  配饰: "accessory",
};
const field = (label, control, group = false) => {
  const Tag = group ? "div" : "label";
  return (
    <Tag className="replica-field">
      <span>{label}</span>
      {control}
    </Tag>
  );
};
const art = (g, showStatus = false, allowRetry = false) => <WardrobeItemArt type={types[g.type] || g.type} name={g.name} media={g._source?.media || g.media} urlForAsset={mediaClient.url} showStatus={showStatus} allowRetry={allowRetry} />;

function recordText(value) {
  const text = typeof value === "string" ? value.trim() : "";
  if (isInternalId(text) || /^(?:outfit|inventory|wardrobe|item|profile)_[a-z0-9_-]+$/i.test(text)) return "";
  return text ? value : "";
}

function recordTime(value) {
  return typeof value === "string" && Number.isFinite(Date.parse(value)) ? userInstant(value) : "";
}

function RecordFacts({ facts, title }) {
  const visible = facts.filter(([, value]) => value);
  if (!visible.length) return null;
  return (
    <div className="r-muted">
      {title && <h4>{title}</h4>}
      {visible.map(([label, value]) => <p key={label}>{label}：{value}</p>)}
    </div>
  );
}

function WardrobeRecordDownload({ value, kind, history = false }) {
  if (!value || !Object.keys(value).length) return null;
  const outfit = kind === "outfit";
  return (
    <JsonDetails
      value={value}
      title={outfit ? "原始穿搭方案（诊断）" : "原始服装记录（诊断）"}
      filename={`wardrobe-${outfit ? "outfit" : "item"}${history ? "-history" : ""}.json`}
      downloadOnly
    />
  );
}

const clothingCategory = value => value?.category && recordText(wardrobeCategoryLabel(value.category));

function revisionFacts(value, kind) {
  if (!value) return [];
  if (kind === "outfit") {
    const tags = Array.isArray(value.tags) ? value.tags.map(recordText).filter(Boolean) : [];
    const members = Array.isArray(value.items)
      ? value.items.map((item) => recordText(item?.description) || recordText(item?.name)).filter(Boolean)
      : [];
    return [
      ["方案名称", recordText(value.name)],
      ["场景", tags.join("、")],
      ["搭配成员", members.join("、")],
    ];
  }
  const clothingType = clothingTypeOf(value);
  const seasons = Array.isArray(value.seasons)
    ? clothingSeasons.filter(({ value: season }) => value.seasons.includes(season)).map(({ label }) => label)
    : [];
  return [
    ["服装描述", recordText(value.description) || recordText(value.name)],
    ["服装分类", clothingCategory(value)],
    ["服装类型", clothingTypeOptions.find(({ value: type }) => type === clothingType)?.label],
    ["适合季节", seasons.join("、")],
    ["拥有关系", wardrobeOwnershipOptions.find(({ value: state }) => state === value.ownership)?.label],
    ["可用状态", wardrobeAvailabilityOptions.find(({ value: state }) => state === value.availability)?.label],
    ["归档时间", recordTime(value.archived_at)],
    ["使用活动", recordText(value.activity)],
  ];
}

function WardrobeRevision({ item, kind }) {
  const useEvent = item.kind === "use_event";
  const reason = recordText(item.reason);
  const auditReason = recordText(item.owner_audit?.reason);
  const usageLabel = item.operation === "start" ? "开始使用" : item.operation === "stop" ? "结束使用" : "使用记录";
  const title = useEvent ? usageLabel : kind === "outfit" ? "方案修订" : "服装修订";
  return (
    <article>
      <h4>{title}</h4>
      <RecordFacts facts={[
        [useEvent ? "发生时间" : "修订时间", recordTime(item.occurred_at) || recordTime(item.created_at)],
        ["修订原因", reason || auditReason],
        ["修正说明", reason && reason !== auditReason ? auditReason : ""],
        ["使用活动", useEvent ? recordText(item.payload?.activity) : ""],
      ]} />
      <RecordFacts title="变更前" facts={revisionFacts(item.before_value, kind)} />
      <RecordFacts title="变更后" facts={revisionFacts(item.after_value, kind)} />
      <WardrobeRecordDownload value={item} kind={kind} history />
    </article>
  );
}

function RecordHistory({ fid, id, kind }) {
  const w = useWorkspace(),
    scope = w.currentScope(),
    [page, setPage] = useState({ items: [], cursor: "", ready: false }),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const epoch = useRef(0),
    active = useRef(false),
    retry = useRef(null);
  async function load(append = false, cursor = append ? page.cursor : "") {
    const token = ++epoch.current;
    setBusy(true);
    setError("");
    try {
      const result = await (kind === "outfit"
        ? w.services.inventoryClient.outfitHistory(fid, id, cursor)
        : w.services.inventoryClient.history(fid, id, cursor));
      if (
        !active.current ||
        token !== epoch.current ||
        w.currentScope() !== scope
      )
        return;
      setPage((old) => ({
        items: append ? [...old.items, ...result.items] : result.items,
        cursor: result.next_cursor || "",
        ready: true,
      }));
    } catch (cause) {
      if (
        active.current &&
        token === epoch.current &&
        w.currentScope() === scope
      ) {
        setError(errorMessage(cause));
        retry.current = () => void load(append, cursor);
      }
    } finally {
      if (active.current && token === epoch.current) setBusy(false);
    }
  }
  useEffect(() => {
    active.current = true;
    setPage({ items: [], cursor: "", ready: false });
    void load();
    return () => {
      active.current = false;
      epoch.current++;
    };
  }, [scope, id, kind]);
  return (
    <Disclosure title="登记与修订历史">
      {busy && <LoadingState label="正在读取修订历史…" />}
      {error && (
        <InlineFeedback role="alert">
          {error}
          <Button variant="outline" onClick={() => retry.current?.()}>
            重新读取修订
          </Button>
        </InlineFeedback>
      )}
      {page.items.map((item, index) => item && Object.keys(item).length > 0 && (
        <WardrobeRevision key={item.id || index} item={item} kind={kind} />
      ))}
      {page.ready && !page.items.length && !error && (
        <p className="r-muted">当前读取范围没有修订记录。</p>
      )}
      {page.cursor && (
        <Button
          variant="outline"
          disabled={busy}
          onClick={() => void load(true)}
        >
          加载更多修订
        </Button>
      )}
    </Disclosure>
  );
}
export function WardrobePageView({ view, route, preparation = null }) {
  const w = useWorkspace(),
    c = readValue(view.character),
    fid = c.id,
    scope = w.currentScope(),
    ready = !!c._wardrobe,
    readError = currentReadError(w.state, "wardrobe", scope),
    readonly = ["retired", "archived", "已退休", "已归档"].includes(c.status),
    formId = useId();
  const [composeStep,setComposeStep] = useState(1);
  useEffect(() => {setComposeStep(1);}, [view.tab.value, view.editingId.value]);
  const [search, setSearch] = useState(""),
    [archived, setArchived] = useState(false),
    [archivedPage, setArchivedPage] = useState({
      items: [],
      cursor: "",
      ready: false,
    }),
    [archiveBusy, setArchiveBusy] = useState(false),
    [archiveError, setArchiveError] = useState(""),
    [tagsOpen, setTagsOpen] = useState(false),
    [activeTag, setActiveTag] = useState("all"),
    [confirmBusy, setConfirmBusy] = useState(false),
    [confirmError, setConfirmError] = useState("");
  const archiveEpoch = useRef(0),
    mounted = useRef(true),
    archiveRetry = useRef(null),
    trigger = useRef(null);
  const routeQuery = route?.query?.toString() || "";
  useEffect(() => {
    const query = new URLSearchParams(routeQuery);
    if (query.has("q")) setSearch(query.get("q") || "");
    if (query.has("state")) setArchived(query.get("state") === "archived");
    const tab = {clothes:"全部衣服",garments:"全部衣服",plans:"方案库",outfits:"方案库",compose:"为她搭配"}[query.get("tab")];
    if (tab) view.tab.value = tab;
  }, [routeQuery, scope]);
  const all = archived ? archivedPage.items : c.garments,
    filtered = all.filter(
      (g) =>
        (!view.type.value || g.type === view.type.value) &&
        (!view.season.value || g.seasons.includes(view.season.value)) &&
        g.name.toLowerCase().includes(search.trim().toLowerCase()),
    );
  const worn = c._wardrobe?.wearing?.worn_items,
    wearingIds = Array.isArray(worn)
      ? new Set(worn.map((g) => g.item_id || g.id))
      : new Set();
  const preview = c.outfits.find((o) => o.id === view.preview.value),
    draft = view.garment.value,
    selected = Object.entries(view.items.value).flatMap(([slot, ids]) =>
      ids.map((id) => ({
        slot,
        id,
        garment: c.garments.find((g) => g.id === id),
      })),
    );
  const validOutfit = (o) =>
    Object.values(o.items || {}).flat().length > 0 &&
    Object.values(o.items || {})
      .flat()
      .every((id) =>
        c.garments.some(
          (g) =>
            g.id === id &&
            g._source?.availability === "available" &&
            !g._source.archived_at,
        ),
      );
  const reset = () => {
    view.editingId.value = null;
    view.items.value = {};
    view.name.value = "";
    view.draftError.value = "";
  };
  const openGarment = (g, event) => {
    trigger.current = event?.currentTarget || document.activeElement;
    view.garment.value = g
      ? jsonValueSnapshot(g)
      : {
          name: "",
          type: "上衣",
          seasons: [],
          ownership: "unknown",
          availability: "available",
        };
  };
  async function readArchived(
    append = false,
    cursor = append ? archivedPage.cursor : "",
  ) {
    const token = ++archiveEpoch.current;
    setArchiveBusy(true);
    setArchiveError("");
    try {
      const page = await w.services.inventoryClient.list(fid, {
        kind: "wearable",
        state: "archived",
        ...(cursor ? { cursor } : {}),
      });
      if (
        !mounted.current ||
        token !== archiveEpoch.current ||
        w.currentScope() !== scope
      )
        return;
      setArchivedPage((old) => ({
        items: append
          ? [...old.items, ...page.items.map(garmentView)]
          : page.items.map(garmentView),
        cursor: page.next_cursor || "",
        ready: true,
      }));
    } catch (cause) {
      if (
        mounted.current &&
        token === archiveEpoch.current &&
        w.currentScope() === scope
      ) {
        setArchiveError(errorMessage(cause));
        archiveRetry.current = () => void readArchived(append, cursor);
      }
    } finally {
      if (mounted.current && token === archiveEpoch.current)
        setArchiveBusy(false);
    }
  }
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
      archiveEpoch.current++;
    };
  }, [scope]);
  useEffect(() => {
    if (archived && ready) void readArchived();
  }, [archived, ready, c._wardrobe?.catalog?.revision]);
  const clear = () => {
    setSearch("");
    view.type.value = "";
    view.season.value = "";
    setArchived(false);
  };
  const requestWear = (o) => {
    view.confirm(
      "请" + c.name + "换上「" + o.name + "」？",
      "请求受理后跟进实际处理结果，当前穿着以正式读回为准。",
      () => view.wear(o),
      "发送换装请求",
    );
  };
  const readNotice = readError ? (
    <WardrobeReadNotice
      w={w}
      error={readError}
      cause={w.state.errorCauses?.wardrobe}
      ready={ready}
    />
  ) : null;
  return (
    <div className="replica-resource replica-wardrobe replica-life">
      <WorkspacePageHeader
        eyebrow="WARDROBE / WORN WITH A STORY"
        title={c.name + "的衣柜"}
        description="喜欢的衣服，合适的搭配，还有今天的自己。"
        actions={
          <>
            <Button
              variant="outline"
              onClick={() =>
                w.navigatePath(
                  "/c/" + encodeURIComponent(fid) + "/media/generate",
                )
              }
            >
              <CameraIcon size={15} />
              拍一张穿搭
            </Button>
            <Button
              disabled={!ready || readonly}
              onClick={(event) => openGarment(null, event)}
            >
              <PlusIcon size={16} />
              添加衣服
            </Button>
          </>
        }
      />
      {w.state.loading.wardrobe && <LoadingState label="正在读取衣柜…" />}
      <Surface className="r-wearing-strip">
        <div className="r-wearing-icon">
          <ShirtIcon size={25} strokeWidth={1.3} />
        </div>
        <div>
          <span className="r-tiny-label">TODAY'S OUTFIT</span>
          <h3>今天穿着</h3>
          <WearingSummary character={c} error={readError} loading={w.state.loading.wardrobe} />
          {(!readError || ready) && preparation}
        </div>
        <Badge variant="outline">
          {Array.isArray(worn) ? "当前穿着" : "待读取"}
        </Badge>
      </Surface>
      <Tabs
        value={tabKeys[view.tab.value] || "garments"}
        onValueChange={(value) => {
          view.tab.value = Object.keys(tabKeys).find(
            (key) => tabKeys[key] === value,
          );
        }}
      >
        <ResponsiveTabsList value={tabKeys[view.tab.value] || "garments"} variant="line">
          <TabsTrigger value="garments">
            全部衣服 <span>{ready ? c.garments.length : "…"}</span>
          </TabsTrigger>
          <TabsTrigger value="outfits">
            穿搭方案 <span>{ready ? c.outfits.length : "…"}</span>
          </TabsTrigger>
          <TabsTrigger value="compose">搭配工作台</TabsTrigger>
        </ResponsiveTabsList>
        <TabsContent value="garments">
          <div className="r-toolbar r-wrap wardrobe-filters">
            <div className="r-search">
              <SearchIcon size={15} />
              <Input
                aria-label="搜索衣服"
                placeholder="找到一件喜欢的衣服"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
            </div>
            <ChoiceSelect
              aria-label="按类型筛选"
              value={view.type.value}
              onChange={(e) => (view.type.value = e.target.value)}
            >
              <option value="">所有类型</option>
              {slots.map((s) => (
                <option key={s}>{s}</option>
              ))}
            </ChoiceSelect>
            <ChoiceSelect
              aria-label="按季节筛选"
              value={view.season.value}
              onChange={(e) => (view.season.value = e.target.value)}
            >
              <option value="">所有季节</option>
              {["春", "夏", "秋", "冬"].map((s) => (
                <option key={s} value={s}>
                  {s}季
                </option>
              ))}
            </ChoiceSelect>
            <label className="r-inline-check">
              <Checkbox
                checked={archived}
                onCheckedChange={(value) => setArchived(!!value)}
              />
              已归档
            </label>
          </div>
          {readNotice}
          {archiveBusy && <LoadingState label="正在读取已归档衣服…" />}
          {archiveError && (
            <InlineFeedback role="alert">
              {archiveError}
              <Button
                variant="outline"
                onClick={() => archiveRetry.current?.()}
              >
                重新读取归档衣服
              </Button>
            </InlineFeedback>
          )}
          {ready && <div className="r-garment-grid garment-grid">
            {filtered.map((g) => (
              <Surface key={g.id} className="r-garment-card">
                <Button
                  variant="ghost"
                  className="r-garment-image"
                  aria-label={"查看" + g.name}
                  onClick={(event) => openGarment(g, event)}
                >
                  {art(g, true)}
                  {wearingIds.has(g.id) && (
                    <span className="r-floating-badge">正在穿</span>
                  )}
                </Button>
                <div className="r-card-body">
                  <span className="r-tiny-label">{g.type}</span>
                  <h3>{g.name}</h3>
                  <div className="r-inline-actions">
                    {g.seasons.map((s) => (
                      <span className="r-season-tag" key={s}>
                        {s}
                      </span>
                    ))}
                    <MenuDisclosure
                      title={<Edit3Icon size={13} />}
                      aria-label={g.name + "的衣服操作"}
                    >
                      <Button
                        variant="ghost"
                        onClick={(event) => openGarment(g, event)}
                      >
                        {g._source?.archived_at ? "查看资料" : "编辑"}
                      </Button>
                      {!g._source?.archived_at && (
                        <Button
                          variant="ghost"
                          disabled={readonly}
                          onClick={() => view.removeGarment(g.id)}
                        >
                          归档
                        </Button>
                      )}
                    </MenuDisclosure>
                  </div>
                </div>
              </Surface>
            ))}
          </div>}
          {ready && !readError &&
            (!archived || archivedPage.ready) &&
            !filtered.length &&
            !archiveBusy &&
            !archiveError && (
              <EmptyState variant="page">
                <h3>没有符合条件的衣服。</h3>
                <p>试试调整类型、季节或搜索条件。</p>
                <Button variant="outline" onClick={clear}>
                  清除筛选
                </Button>
              </EmptyState>
            )}
          {archivedPage.cursor && archived && (
            <Button
              variant="outline"
              disabled={archiveBusy}
              onClick={() => void readArchived(true)}
            >
              加载更多归档衣服
            </Button>
          )}
        </TabsContent>
        <TabsContent value="outfits">
          <div className="r-toolbar r-wrap">
            <div className="r-filter-pills">
              {["all", ...c.outfitTags].map((tag) => (
                <Button
                  key={tag}
                  variant="ghost"
                  className={activeTag === tag ? "active" : ""}
                  onClick={() => setActiveTag(tag)}
                >
                  {tag === "all" ? "全部场景" : view.tagLabel(tag)}
                </Button>
              ))}
            </div>
            <Button
              variant="ghost"
              disabled={!ready || readonly}
              onClick={(event) => {
                trigger.current = event.currentTarget;
                setTagsOpen(true);
              }}
            >
              <SlidersHorizontalIcon size={14} />
              管理标签
            </Button>
            <Button
              disabled={!ready || readonly}
              onClick={() => {
                reset();
                view.tab.value = "为她搭配";
              }}
            >
              <PlusIcon size={14} />
              搭配新方案
            </Button>
          </div>
          {readNotice}
          {ready && <div className="r-outfit-grid">
            {c.outfits
              .filter(
                (o) =>
                  activeTag === "all" ||
                  o.tags?.includes(activeTag) ||
                  o.tag === activeTag,
              )
              .map((o) => (
                <Surface key={o.id} className="r-outfit-card">
                  <div className="r-outfit-preview">
                    {Object.values(o.items)
                      .flat()
                      .slice(0, 4)
                      .map((id) => {
                        const g = c.garments.find((g) => g.id === id) || {
                          name:
                            recordText(o._source?.items?.find((g) => g.id === id)
                              ?.description) || "衣物名称待读取",
                          type: "配饰",
                        };
                        return (
                          <WardrobeItemArt
                            key={id}
                            type={types[g.type] || g.type}
                            name={g.name}
                            media={g._source?.media || g.media}
                            urlForAsset={mediaClient.url}
                          />
                        );
                      })}
                    <span className="r-floating-badge">
                      {view.tagLabel(o.tag) || "未分类"}
                    </span>
                  </div>
                  <div className="r-card-body">
                    <h3>{o.name}</h3>
                    <p>
                      {Object.values(o.items).flat().length} 件服装
                    </p>
                    {!validOutfit(o) && (
                      <small className="r-warning">
                        部分成员当前不可穿，请先调整方案。
                      </small>
                    )}
                    <div className="r-card-footer">
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={(event) => {
                          trigger.current = event.currentTarget;
                          view.preview.value = o.id;
                        }}
                      >
                        <EyeIcon size={14} />
                        预览
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        aria-label={"请" + c.name + "换上「" + o.name + "」"}
                        disabled={view.busy.value || !validOutfit(o)}
                        onClick={() => requestWear(o)}
                      >
                        请换上这套
                      </Button>
                    </div>
                    <view.ActionFeedback
                      labels={["换装"]}
                      operation={"wear-outfit:" + o.id}
                      showStatus
                    />
                  </div>
                </Surface>
              ))}
          </div>}
          {ready && !readError &&
            !c.outfits.filter(
              (o) =>
                activeTag === "all" ||
                o.tags?.includes(activeTag) ||
                o.tag === activeTag,
            ).length && (
              <EmptyState variant="page">
                <h3>还没有符合场景的穿搭方案</h3>
                <Button
                  disabled={readonly}
                  onClick={() => (view.tab.value = "为她搭配")}
                >
                  开始搭配
                </Button>
              </EmptyState>
            )}
        </TabsContent>
        <TabsContent value="compose">
          {readNotice}
          {ready && <><nav className="mobile-compose-steps" aria-label="搭配步骤"><Button variant={composeStep===1?'secondary':'ghost'} onClick={() => setComposeStep(1)}>1 选择衣物</Button><Button variant={composeStep===2?'secondary':'ghost'} disabled={!selected.length} onClick={() => setComposeStep(2)}>2 命名与保存</Button></nav><div className="r-compose-layout" data-compose-step={composeStep}>
            <Surface className="r-padded">
              <h2>挑选今天的灵感</h2>
              <p className="r-muted">每个部位选一件，配饰可以多选。</p>
              <div className="mobile-only"><ChoiceSelect aria-label="挑选衣物的场景" value={view.tag.value} onChange={e => view.tag.value=e.target.value}>{c.outfitTags.map(t => <option key={t} value={t}>{view.tagLabel(t)}</option>)}</ChoiceSelect></div>
              <div className="r-slot-list">
                {slots.map((slot) => {
                  const group = c.garments.filter((g) => g.type === slot);
                  return group.length ? (
                    <section key={slot}>
                      <h4>
                        {slot}
                        <span>{group.length}</span>
                      </h4>
                      <div className="r-compose-garments">
                        {group.map((g) => (
                          <Button
                            key={g.id}
                            variant="ghost"
                            aria-label={g.name}
                            aria-pressed={
                              !!view.items.value[slot]?.includes(g.id)
                            }
                            disabled={!view.tag.value || readonly}
                            className={
                              "r-compose-item choice" +
                              (view.items.value[slot]?.includes(g.id)
                                ? " selected"
                                : "")
                            }
                            onClick={() =>
                              view.choose(slot, g.id, slot === "配饰")
                            }
                          >
                            {art(g)}
                            <span>{g.name}</span>
                            {view.items.value[slot]?.includes(g.id) && (
                              <i>
                                <CheckIcon size={11} />
                              </i>
                            )}
                          </Button>
                        ))}
                      </div>
                    </section>
                  ) : null;
                })}
              </div>
              {ready && !c.garments.length && (
                <p className="r-muted">先补录已知衣物，再开始搭配。</p>
              )}
            </Surface>
            <Surface className="r-compose-summary">
              <span className="r-tiny-label">YOUR COMBINATION</span>
              <h2>{view.name.value || "一套新的日常"}</h2>
              <div className="r-selected-clothes">
                {selected.length ? (
                  selected.map(({ id, slot, garment: g }) => (
                    <div key={slot + id}>
                      {art(g || { name: "衣物待读取", type: slot })}
                      <span>
                        <b>{g?.name || "当前目录没有这件衣物"}</b>
                        <small>{slot}</small>
                      </span>
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label={"移除" + (g?.name || "这件衣物")}
                        onClick={() =>
                          (view.items.value[slot] = view.items.value[
                            slot
                          ].filter((value) => value !== id))
                        }
                      >
                        <XIcon size={13} />
                      </Button>
                    </div>
                  ))
                ) : (
                  <EmptyState variant="page">
                    <h3>慢慢挑，不着急</h3>
                    <p>从左边选几件喜欢的衣服。</p>
                  </EmptyState>
                )}
              </div>
              {view.duplicate.value && (
                <InlineFeedback>方案名称已存在，请换一个名称。</InlineFeedback>
              )}
              {view.draftError.value && (
                <InlineFeedback role="alert">
                  {view.draftError.value}
                </InlineFeedback>
              )}
              <form
                className="r-form-stack"
                onSubmit={(e) => {
                  e.preventDefault();
                  void view.saveOutfit();
                }}
              >
                {field(
                  "方案名称",
                  <FieldInput
                    aria-label="方案名称"
                    value={view.name.value}
                    onChange={(e) => (view.name.value = e.target.value)}
                    placeholder="例如：书店里的慢时光"
                  />,
                )}
                {field(
                  "适合的场景",
                  <ChoiceSelect
                    aria-label="搭配标签"
                    value={view.tag.value}
                    onChange={(e) => (view.tag.value = e.target.value)}
                  >
                    {c.outfitTags.map((t) => (
                      <option key={t} value={t}>
                        {view.tagLabel(t)}
                      </option>
                    ))}
                  </ChoiceSelect>,
                )}
                {!c.outfitTags.length && (
                  <Button
                    type="button"
                    variant="link"
                    onClick={(event) => {
                      trigger.current = event.currentTarget;
                      setTagsOpen(true);
                    }}
                  >
                    先添加场景标签
                  </Button>
                )}
                <Button
                  disabled={
                    view.busy.value ||
                    view.duplicate.value ||
                    !view.tag.value ||
                    !view.name.value.trim() ||
                    !selected.length
                  }
                >
                  {view.editingId.value ? "保存方案修改" : "保存为穿搭方案"}
                  <CheckIcon size={14} />
                </Button>
                <small className="r-muted">
                  保存后，可在方案库单独请求换装。
                </small>
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() => {
                    const editing = !!view.editingId.value;
                    reset();
                    if (editing) view.tab.value = "方案库";
                  }}
                >
                  {view.editingId.value ? "取消编辑" : "重新搭配"}
                </Button>
              </form>
            </Surface>
          </div><div className="mobile-compose-actions"><Button variant="outline" onClick={() => {setComposeStep(composeStep===1?2:1);document.querySelector('.workspace-page')?.scrollTo({top:0});}} disabled={composeStep===1 && !selected.length}>{composeStep===1 ? `下一步 · 已选 ${selected.length} 件` : '返回选择衣物'}</Button></div></>}
        </TabsContent>
      </Tabs>
      <WorkspaceDialog
        className="replica-wardrobe"
        open={!!draft}
        onOpenChange={(open) => {
          if (!open) view.garment.value = null;
        }}
        returnFocus={() => trigger.current}
        title={draft ? (draft._source ? "服装资料" : "添加衣服") : "服装资料"}
        description="补录或修正已经知道的衣物信息。"
        footer={
          draft ? (
            <>
              <Button
                variant="outline"
                onClick={() => (view.garment.value = null)}
              >
                取消
              </Button>
              {!draft._source?.archived_at && (
                <Button
                  form={formId}
                  type="submit"
                  disabled={view.busy.value || !draft.name.trim()}
                >
                  保存衣服
                </Button>
              )}
            </>
          ) : undefined
        }
      >
        {draft && (
          <form
            id={formId}
            data-testid="garment-drawer"
            className="r-form-stack"
            onSubmit={(e) => {
              e.preventDefault();
              void view.saveGarment();
            }}
          >
            <view.ActionFeedback labels={["衣柜"]} />
            <div className="r-garment-dialog-art">{art(draft, true, true)}</div>
            {field(
              "衣服名称 / 描述",
              <FieldInput
                aria-label="衣服名称"
                disabled={readonly || !!draft._source?.archived_at}
                value={draft.name}
                onChange={(e) => (draft.name = e.target.value)}
              />,
            )}
            <p className="r-muted">填写便于辨认的衣服名称。</p>
            {field(
              "服装类型",
              <ChoiceSelect
                aria-label="服装类型"
                disabled={readonly || !!draft._source?.archived_at}
                value={draft.type}
                onChange={(e) => (draft.type = e.target.value)}
              >
                {slots.map((s) => (
                  <option key={s}>{s}</option>
                ))}
              </ChoiceSelect>,
            )}
            {field(
              "适合季节",
              <div className="r-check-list">
                {["春", "夏", "秋", "冬"].map((s) => (
                  <label key={s}>
                    <Checkbox
                      disabled={readonly || !!draft._source?.archived_at}
                      checked={draft.seasons.includes(s)}
                      onCheckedChange={(checked) =>
                        (draft.seasons = checked
                          ? [...draft.seasons, s]
                          : draft.seasons.filter((v) => v !== s))
                      }
                    />
                    {s}季
                  </label>
                ))}
              </div>,
              true,
            )}
            <div className="r-form-grid">
              {field(
                "拥有关系",
                <ChoiceSelect
                  aria-label="拥有关系"
                  disabled={readonly || !!draft._source?.archived_at}
                  value={
                    draft.ownership || draft._source?.ownership || "unknown"
                  }
                  onChange={(e) => (draft.ownership = e.target.value)}
                >
                  <option value="owned">已拥有</option>
                  <option value="borrowed">借用</option>
                  <option value="unknown">尚未确认</option>
                </ChoiceSelect>,
              )}
              {field(
                "可用状态",
                <ChoiceSelect
                  aria-label="可用状态"
                  disabled={readonly || !!draft._source?.archived_at}
                  value={
                    draft.availability ||
                    draft._source?.availability ||
                    "available"
                  }
                  onChange={(e) => (draft.availability = e.target.value)}
                >
                  <option value="available">可穿着</option>
                  <option value="unavailable">暂不可用</option>
                  <option value="lost">已遗失</option>
                </ChoiceSelect>,
              )}
            </div>
            {draft._source && (
              <>
                <RecordFacts facts={[
                  ["服装描述", draft._source.description !== draft.name ? recordText(draft._source.description) : ""],
                  ["登记时间", recordTime(draft._source.acquired_at)],
                  ["最近修改", recordTime(draft._source.updated_at)],
                  ["归档时间", recordTime(draft._source.archived_at)],
                ]} />
                <RecordHistory fid={fid} id={draft.id} kind="garment" />
                <RecordFacts facts={[["服装分类", clothingCategory(draft._source)]]} />
                <WardrobeRecordDownload value={draft._source} kind="garment" />
                {draft._source.archived_at ? (
                  <p className="r-muted">
                    这件衣服已归档，历史和方案引用仍保留。
                  </p>
                ) : (
                  <Button
                    type="button"
                    variant="ghost"
                    disabled={view.busy.value}
                    onClick={() => view.removeGarment(draft.id)}
                  >
                    归档服装
                  </Button>
                )}
              </>
            )}
          </form>
        )}
      </WorkspaceDialog>
      <WorkspaceDialog
        className="replica-wardrobe"
        open={!!preview}
        onOpenChange={(open) => {
          if (!open) view.preview.value = null;
        }}
        returnFocus={() => trigger.current}
        title={preview?.name || "穿搭预览"}
        description={
          (view.tagLabel(preview?.tag) || "未分类") + " · 已保存方案"
        }
        footer={
          preview ? (
            <>
              <Button
                variant="outline"
                disabled={readonly}
                onClick={() => {
                  view.edit(preview);
                  view.preview.value = null;
                }}
              >
                编辑方案
              </Button>
              <Button
                disabled={view.busy.value || !validOutfit(preview)}
                onClick={() => requestWear(preview)}
              >
                请换上这套
              </Button>
            </>
          ) : undefined
        }
      >
        {preview && (
          <div className="r-form-stack">
            <div className="r-selected-clothes">
              {Object.values(preview.items)
                .flat()
                .map((id) => {
                  const g = c.garments.find((g) => g.id === id);
                  return (
                    <div key={id}>
                      {art(g || { type: "配饰", name: "衣物待读取" })}
                      <span>
                        <b>
                          {g?.name ||
                            recordText(preview._source?.items?.find((g) => g.id === id)
                              ?.description) ||
                            "衣物名称待读取"}
                        </b>
                        <small>{g?.type || "当前目录未找到"}</small>
                      </span>
                      <Badge variant="outline">
                        {g
                          ? g._source?.availability === "available"
                            ? "可穿"
                            : "暂不可穿"
                          : "待读取"}
                      </Badge>
                    </div>
                  );
                })}
            </div>
            <div className="r-tag-edit">
              <FieldInput
                aria-label="方案新名称"
                value={
                  view.renameOutfit.value === preview.id
                    ? view.renameValue.value
                    : preview.name
                }
                onChange={(e) => {
                  view.renameOutfit.value = preview.id;
                  view.renameValue.value = e.target.value;
                }}
              />
              <Button
                variant="outline"
                disabled={
                  view.busy.value ||
                  view.renameOutfit.value !== preview.id ||
                  !view.renameValue.value.trim()
                }
                onClick={() => void view.rename(preview)}
              >
                重命名
              </Button>
            </div>
            <view.ActionFeedback labels={["衣柜", "换装"]} />
            <RecordFacts facts={[
              ["场景", preview.tags?.slice(1).map((tag) => recordText(view.tagLabel(tag))).filter(Boolean).join("、")],
              ["保存时间", recordTime(preview._source?.created_at)],
              ["最近修改", recordTime(preview._source?.updated_at)],
            ]} />
            <RecordHistory fid={fid} id={preview.id} kind="outfit" />
            <WardrobeRecordDownload value={preview._source} kind="outfit" />
          </div>
        )}
      </WorkspaceDialog>
      <WorkspaceDialog
        className="replica-wardrobe"
        open={tagsOpen}
        returnFocus={() => trigger.current}
        onOpenChange={setTagsOpen}
        title="管理穿搭场景"
        description="删除场景标签会保留已有方案。"
      >
        <div className="r-form-stack tag-list">
          {c.outfitTags.map((tag) => (
            <div className="r-tag-edit wardrobe-tag" key={tag}>
              <Badge variant="outline">{view.tagLabel(tag)}</Badge>
              <MenuDisclosure
                title={<SlidersHorizontalIcon size={14} />}
                aria-label={view.tagLabel(tag) + "的标签操作"}
              >
                <Button
                  variant="ghost"
                  disabled={view.busy.value}
                  onClick={() => {
                    view.renameTag.value = tag;
                    view.newTag.value = tag;
                  }}
                >
                  改名
                </Button>
                <Button
                  variant="ghost"
                  disabled={view.busy.value}
                  onClick={() => view.removeTag(tag)}
                >
                  删除
                </Button>
              </MenuDisclosure>
            </div>
          ))}
          <form
            className="r-tag-edit"
            onSubmit={(e) => {
              e.preventDefault();
              void view.saveTags();
            }}
          >
            <FieldInput
              aria-label="标签名称"
              placeholder={view.renameTag.value ? "新标签名" : "新的场景标签"}
              value={view.newTag.value}
              onChange={(e) => (view.newTag.value = e.target.value)}
            />
            <Button disabled={view.busy.value || !view.newTag.value.trim()}>
              {view.renameTag.value ? "保存标签" : "添加"}
            </Button>
          </form>
          {view.renameTag.value && (
            <Button
              variant="ghost"
              onClick={() => {
                view.renameTag.value = "";
                view.newTag.value = "";
              }}
            >
              取消改名
            </Button>
          )}
          <view.ActionFeedback labels={["衣柜"]} />
        </div>
      </WorkspaceDialog>
      {view.confirmation.value && (
        <ConfirmationFrame
          title={view.confirmation.value.title}
          description={view.confirmation.value.message}
          onClose={() => {
            if (!confirmBusy) {
              view.confirmation.value = null;
              setConfirmError("");
            }
          }}
        >
          <>
            {confirmError && (
              <InlineFeedback role="alert">{confirmError}</InlineFeedback>
            )}
            <AlertDialogCancel disabled={confirmBusy}>取消</AlertDialogCancel>
            <AlertDialogAction
              disabled={confirmBusy}
              onClick={async (e) => {
                e.preventDefault();
                setConfirmBusy(true);
                setConfirmError("");
                try {
                  const result = view.confirmation.value.run();
                  const saved = await (result?.completion || result);
                  if (saved === false)
                    setConfirmError(
                      w.state.operations.filter((op) => op.fid === fid).at(-1)
                        ?.reason || "操作未完成，原输入和请求记录已保留。",
                    );
                  else {
                    view.confirmation.value = null;
                    if (
                      draft?.id &&
                      !w.current.value.garments.some((g) => g.id === draft.id)
                    )
                      view.garment.value = null;
                  }
                } catch (cause) {
                  setConfirmError(errorMessage(cause));
                } finally {
                  setConfirmBusy(false);
                }
              }}
            >
              {confirmBusy ? "正在处理…" : view.confirmation.value.confirmLabel}
            </AlertDialogAction>
          </>
        </ConfirmationFrame>
      )}
      {!draft && !preview && !tagsOpen && (
        <view.ActionFeedback
          labels={["衣柜", "换装"]}
          excludeOperationPrefix={
            view.tab.value === "方案库" ? "wear-outfit:" : ""
          }
        />
      )}
    </div>
  );
}

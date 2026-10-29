import {
  Globe2,
  UserRound,
  MapPin,
  Package,
  Shirt,
  CalendarDays,
  NotebookPen,
  Palette,
  Wand2,
  Plus,
  ArrowUpRight,
  MoreHorizontal,
  RefreshCw,
  Search,
  ChevronRight,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from "@/components/ui/dropdown-menu";
import {
  Surface,
  Badge,
  ChoiceSelect,
  FieldInput,
  Textarea,
} from "../controls";
import WorkspacePageHeader from "../shared/WorkspacePageHeader";
import WorkspaceCommandDialog from "../shared/WorkspaceCommandDialog";
import WorkspaceInfo from "../shared/WorkspaceInfo";
import { WorldArt } from "../shared/WorkspaceArtwork";
import { MediaReader, MediaMore } from "../shared/MediaParts";
import {
  studioKindNames,
  studioVersion,
  studioSummary,
  studioCompactTime,
} from "@/approved-ui/studio-presentation.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";
const icons = {
  world: Globe2,
  character_template: UserRound,
  place: MapPin,
  item: Package,
  outfit: Shirt,
  npc: UserRound,
  event_template: CalendarDays,
  persona_fragment: NotebookPen,
  visual_style: Palette,
};
export default function StudioDirectoryView({ model: m }) {
  const available = m.list.items.filter(
    (r) => r.published_version && r.status !== "archived",
  ).length;
  return (
    <div className="replica-studio resource-page">
      <WorkspacePageHeader
        eyebrow={m.fixedKind==='world'?'YOUR WORLD / A PLACE TO BELONG':'STUDIO / ROOM FOR IMAGINATION'}
        title={m.fixedKind === "world" ? "世界" : "创作中心"}
        description={m.fixedKind==='world'?'保存背景、规则和地点设定，之后可在创建角色时使用。':'给想象一个落点，再让它进入生活。'}
        actions={
          <>
            {m.fixedKind==='world'?<Button onClick={()=>m.create('world')}><Plus size={16}/>创建世界底稿</Button>:<DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button>
                  <Plus size={16} />
                  新建资源
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                {Object.entries(studioKindNames)
                  .filter(([type]) => !m.fixedKind || type === m.fixedKind)
                  .map(([type, label]) => (
                    <DropdownMenuItem
                      key={type}
                      onSelect={() => m.create(type)}
                    >
                      {label}
                    </DropdownMenuItem>
                  ))}
              </DropdownMenuContent>
            </DropdownMenu>}
          </>
        }
      />
      <div className="r-studio-intro">
        <span className="r-soft-icon">
          <Wand2 size={24} />
        </span>
        <div>
          <h3>{m.fixedKind==='world'?'从一份世界设定开始':'从一份设定，开始更大的世界'}</h3>
          <p>{m.fixedKind==='world'?'这里保存可复用的世界底稿。角色的运行世界会在创建时独立建立。':'世界、角色、物品与风格，在这里保存、完善和复用。'}</p>
        </div>
        <div className="r-studio-stat">
          <strong>
            {m.list.ready ? available + (m.list.cursor ? "+" : "") : "—"}
          </strong>
          <span>{m.fixedKind==='world'?'已读可用底稿':'已读可用资源'}</span>
        </div>
      </div>
      <form
        className="r-toolbar r-wrap"
        onSubmit={(event) => {
          event.preventDefault();
          if (m.search.trim() === m.query) void m.list.load();
          else m.setQuery(m.search.trim());
        }}
      >
        <div className="r-search">
          <Search size={15} />
          <FieldInput
            id="studio-query"
            aria-label="搜索资源名称"
            type="search"
            placeholder="搜索资源名称"
            value={m.search}
            onChange={(event) => m.setSearch(event.target.value)}
          />
          <div className="r-search-actions">
            <Button type="button" variant="ghost" size="icon" aria-label="刷新创作资源" disabled={m.list.busy} onClick={m.list.load}>
              <RefreshCw size={14}/>
            </Button>
            <Button variant="ghost" size="icon" type="submit" title="查询创作资源" aria-label="查询">
              <ChevronRight size={14}/><span className="sr-only">查询</span>
            </Button>
          </div>
        </div>
        {!m.fixedKind && (
          <ChoiceSelect
            aria-label="资源类型"
            value={m.kind}
            onChange={(event) => m.setKind(event.target.value)}
          >
            <option value="">全部类型</option>
            {Object.entries(studioKindNames).map(([type, label]) => (
              <option key={type} value={type}>
                {label}
              </option>
            ))}
          </ChoiceSelect>
        )}
        <label className="r-inline-check">
          <Checkbox
            aria-label="已归档资源"
            checked={m.archived}
            onCheckedChange={(checked) => m.setArchived(!!checked)}
          />
          已归档
        </label>
      </form>
      {(m.legacy || m.legacyError) && (
        <WorkspaceInfo>
          {m.legacyError || "旧版目录的资源管理原提交仍待核对。"}
          {m.legacy && (
            <Button variant="outline" onClick={m.recoverLegacy}>
              继续核对旧版原提交
            </Button>
          )}
        </WorkspaceInfo>
      )}
      {m.recovery.map((row) => (
        <WorkspaceInfo key={row.slot}>
          上次资源管理操作的输入或回执仍保留。
          <Button
            variant="outline"
            onClick={(event) => m.manage(row, event, row.operation)}
          >
            继续核对{row.operation === "archive" ? "归档" : "恢复"}「{row.name}
            」
          </Button>
        </WorkspaceInfo>
      ))}
      <MediaReader source={m.list} label="资源" />
      <div className="r-resource-grid">
        {m.list.items.map((row) => {
          const Icon = icons[row.kind] || Package;
          return (
            <Surface className="panel r-resource-card" key={row.id}>
              <button
                className={
                  "r-resource-thumb" + (row.kind === "world" ? " scenic" : "")
                }
                aria-label={"打开资源 " + row.name}
                onClick={() => m.edit(row)}
              >
                {row.kind === "world" ? (
                  <WorldArt
                    kind={/森林|forest/i.test(row.name) ? "forest" : "city"}
                  />
                ) : (
                  <>
                    <span className={"r-resource-icon r-kind-" + row.kind}>
                      <Icon size={30} strokeWidth={1.3} />
                    </span>
                    <span className="r-resource-watermark">
                      {studioKindNames[row.kind] || "资源"}
                    </span>
                  </>
                )}
              </button>
              <div className="r-card-body">
                <div className="r-inline-between">
                  <span className="r-tiny-label">
                    {studioKindNames[row.kind] || "创作资源"}
                  </span>
                  <Badge
                    variant="outline"
                    className="r-resource-state"
                    data-status={row.status}
                    data-published={!!row.published_version&&row.status!=='archived'}
                  >
                    <i aria-hidden="true"/>
                    {row.status === "archived"
                      ? "已归档"
                      : row.published_version
                        ? "已发布"
                        : "草稿"}
                  </Badge>
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label={row.name + "的操作"}
                        ref={(node) => {
                          if (node) m.actionTriggers.current.set(row.id, node);
                          else m.actionTriggers.current.delete(row.id);
                        }}
                      >
                        <MoreHorizontal size={15} />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem onSelect={() => m.edit(row)}>
                        编辑资源
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        disabled={
                          !Number.isSafeInteger(row.revision) ||
                          !!m.legacy ||
                          !!m.legacyError
                        }
                        onSelect={(event) => m.manage(row, event)}
                      >
                        {row.status === "archived" ? "恢复资源" : "归档资源"}
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>
                <h3>{row.name}</h3>
                <p className="r-clamp-2">{studioSummary(row)}</p>
                <div className="r-card-footer">
                  <small
                    title={
                      row.updated_at ? userInstant(row.updated_at) : undefined
                    }
                  >
                    {studioVersion(row.published_version, "未发布")}
                    {row.updated_at
                      ? " · " + studioCompactTime(userInstant(row.updated_at))
                      : ""}
                  </small>
                  <Button size="sm" variant="ghost" onClick={() => m.edit(row)}>
                    查看与编辑
                    <ArrowUpRight size={13} />
                  </Button>
                </div>
              </div>
            </Surface>
          );
        })}
      </div>
      {m.list.ready && !m.list.items.length && (
        <Surface className="panel r-padded studio-empty">
          <h2>没有找到相关资源</h2>
          <p>调整筛选，或创建一份新的设定。</p>
          <Button onClick={() => m.create()}>新建资源</Button>
        </Surface>
      )}
      <MediaMore source={m.list} label="资源" />
      {m.action && (
        <WorkspaceCommandDialog
          action={{
            ...m.action,
            fields: (d, edit) => (
              <label className="replica-field">
                <span>操作原因</span>
                <Textarea
                  aria-label="操作原因"
                  value={d.reason}
                  onChange={(event) => edit({ reason: event.target.value })}
                />
              </label>
            ),
          }}
          onClose={() => m.setAction(null)}
          returnFocus={() => m.trigger.current}
        />
      )}
    </div>
  );
}

import { useEffect, useState } from "react";
import {
  ArrowLeftIcon,
  ArrowRightIcon,
  ArrowUpRightIcon,
  CheckCircle2Icon,
  ChevronRightIcon,
  FolderOpenIcon,
  Globe2Icon,
  HistoryIcon,
  MapPinIcon,
  PlusIcon,
  RefreshCwIcon,
  SearchIcon,
  UserRoundIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Checkbox } from "@/components/ui/checkbox";
import { Surface, Badge, Disclosure, FieldInput } from "../controls";
import { ActionFeedback } from "../components";
import WorkspacePageHeader from "../shared/WorkspacePageHeader";
import WorkspaceInfo from "../shared/WorkspaceInfo";
import { WorldArt, worldArtworkKind as decorativeKind } from "../shared/WorkspaceArtwork";
import LoadingState from "../shared/LoadingState";
import { EmptyState, InlineFeedback } from "../shared/Presentation";
import SemanticTree from "../shared/SemanticTree";
import JsonDetails from "../shared/JsonDetails";
import CharacterAvatar from "../shared/CharacterAvatar";
import WorldPlaceHistory from "../shared/WorldPlaceHistory";
import {
  WorldItemsPanel,
  WorldEventsPanel,
} from "../shared/WorldRuntimePanels";
import WorldField from "../shared/WorldFields";
const Search = ({ value, onChange, label }) => (
  <div className="r-search">
    <SearchIcon size={15} aria-hidden="true"/>
    <FieldInput
      type="search"
      aria-label={label}
      placeholder={label}
      value={value}
      onChange={(e) => onChange(e.target.value)}
    />
  </div>
);
const Tiny = ({ children }) => <span className="r-tiny-label">{children}</span>;
export function WorldPageView({ view: m }) {
  const [archived, setArchived] = useState(false);
  useEffect(() => setArchived(false), [m.selected]);
  const viewOf = (id) => m.catalog.extra.snapshots?.[id],
    residents = (id) => [
      ...(id === m.binding?.world_id
        ? [
            {
              id: m.c.id,
              name: m.c.name,
              occupation: m.c.occupation,
              fluctlight: true,
            },
          ]
        : []),
      ...(viewOf(id)?.npcs || []).filter((p) => p.status === "active"),
    ],
    places = (id) =>
      (viewOf(id)?.places || []).filter((p) => p.status !== "archived");
  const groups = m.world
    ? [...(m.world.venue_categories || []).map((s) => ({ ...s, places: [] }))]
    : [];
  for (const p of m.snapshot?.places || []) {
    if (!archived && p.status === "archived") continue;
    let group = groups.find((g) => g.id === p.venue_category_id);
    if (!group) {
      group = groups.find((g) => g.id === "unclassified");
      if (!group) {
        group = { id: "unclassified", name: "未分类地点", places: [] };
        groups.push(group);
      }
    }
    group.places.push(p);
  }
  const filtered = groups
    .filter((g) => m.category === "all" || g.id === m.category)
    .flatMap((g) =>
      g.places
        .filter((p) =>
          (p.name + " " + (p.alias || "") + " " + (p.desc || "") + " " + g.name)
            .toLocaleLowerCase()
            .includes(m.search.trim().toLocaleLowerCase()),
        )
        .map((p) => ({ p, group: g })),
    );
  const studio = (kind) => "/studio?kind=" + kind;
  const go = (path) => m.w.navigatePath(path);
  return (
    <div className="replica-world resource-page">
      <WorkspacePageHeader
        eyebrow="YOUR WORLD / A PLACE TO BELONG"
        title={m.world?.name || "世界"}
        description={
          m.world
            ? "场所、风景与相遇，让生活有迹可循。"
            : "每个人，都在自己的风景里生活。"
        }
        actions={
          <>
            <Button
              variant="ghost"
              size="icon"
              aria-label="刷新世界目录"
              disabled={m.catalog.busy}
              onClick={m.refresh}
            >
              <RefreshCwIcon size={15} />
            </Button>
            <Button variant="outline" onClick={() => go(studio("world"))}>
              <FolderOpenIcon size={15} />
              世界底稿
            </Button>
            <Button
              data-testid="world-add"
              disabled={m.readOnly}
              onClick={(e) => m.editWorld(null, e)}
            >
              <PlusIcon size={16} />
              新建世界
            </Button>
          </>
        }
      />
      {m.catalog.busy && <LoadingState label="正在读取世界与地点…" />}
      {m.catalog.error && (
        <InlineFeedback role="alert">
          {m.catalog.error}
          <Button variant="outline" onClick={m.catalog.retry}>
            重新读取世界目录
          </Button>
        </InlineFeedback>
      )}
      {m.notice && <WorkspaceInfo>{m.notice}</WorkspaceInfo>}
      {m.error && <InlineFeedback role="alert">{m.error}</InlineFeedback>}
      {m.createRecovery && (
        <WorkspaceInfo>
          世界创建的输入或原提交仍待核对。
          <Button
            variant="outline"
            disabled={m.readOnly}
            onClick={(e) => m.editWorld(null, e)}
          >
            继续核对世界创建
          </Button>
        </WorkspaceInfo>
      )}
      {m.selected && !m.world && m.catalog.ready && (
        <WorkspaceInfo>
          当前角色的世界目录没有找到这个世界。
          <Button variant="outline" onClick={m.back}>
            全部世界
          </Button>
        </WorkspaceInfo>
      )}
      {!m.world ? (
        <>
          <div className="r-toolbar">
            <Search
              value={m.query}
              onChange={m.setQuery}
              label="搜索世界名称或类型"
            />
            <span className="r-muted">
              {m.catalog.ready ? m.catalog.items.length : "…"} 个世界
            </span>
          </div>
          {m.catalog.ready && !m.catalog.items.length && !m.catalog.error && (
            <EmptyState>
              <h3>还没有建立生活世界</h3>
              <p>创建一个世界，或从已有世界底稿开始。</p>
            </EmptyState>
          )}
          <div className="r-world-grid" data-testid="world-grid">
            {m.catalog.items
              .filter((row) =>
                (row.name + row.flavor)
                  .toLocaleLowerCase()
                  .includes(m.query.trim().toLocaleLowerCase()),
              )
              .map((row) => (
                <Surface
                  className="r-world-card"
                  key={row.id}
                  data-testid={"world-card-" + row.id}
                >
                  <Button
                    className="r-world-image"
                    variant="ghost"
                    aria-label={"进入世界 " + row.name}
                    onClick={() => m.openWorld(row.id)}
                  >
                    <span aria-hidden="true">
                      <WorldArt kind={decorativeKind(row)} />
                    </span>
                    {m.binding?.world_id === row.id && (
                      <span className="r-floating-badge">
                        <CheckCircle2Icon size={12} />
                        当前世界
                      </span>
                    )}
                  </Button>
                  <div className="r-card-body">
                    <Tiny>{row.flavor || "自定义世界"}</Tiny>
                    <h2>{row.name}</h2>
                    <p className="r-clamp-2">
                      {row.background || "世界背景尚未补充。"}
                    </p>
                    <div className="r-world-meta">
                      <span>
                        <MapPinIcon size={13} />
                        {places(row.id).length} 处地点
                      </span>
                      <span>
                        <UserRoundIcon size={13} />
                        {residents(row.id).length} 位已读人物
                      </span>
                    </div>
                    <div className="r-card-footer">
                      <div className="r-mini-residents">
                        {residents(row.id)
                          .slice(0, 5)
                          .map((p) => (
                            <CharacterAvatar
                              key={p.id}
                              name={p.name}
                              src={p.fluctlight?m.c._directory?.avatar:undefined}
                              color={p.fluctlight?m.c.color:undefined}
                            />
                          ))}
                      </div>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => m.openWorld(row.id)}
                      >
                        进入世界
                        <ArrowRightIcon size={14} />
                      </Button>
                    </div>
                  </div>
                </Surface>
              ))}
            <Button
              variant="ghost"
              className="r-create-world"
              onClick={() => go(studio("world") + "&new=1")}
            >
              <span>
                <PlusIcon size={23} />
              </span>
              <h3>从一个想法开始</h3>
              <p>把新的风景，写成世界底稿</p>
              <small>
                创建可复用的世界设定
                <ArrowUpRightIcon size={12} />
              </small>
            </Button>
          </div>
          {m.query &&
            m.catalog.ready &&
            !m.catalog.items.some((row) =>
              (row.name + row.flavor)
                .toLocaleLowerCase()
                .includes(m.query.trim().toLocaleLowerCase()),
            ) && (
              <EmptyState>
                <h3>没有找到匹配的世界</h3>
                <Button variant="outline" onClick={() => m.setQuery("")}>
                  清除搜索
                </Button>
              </EmptyState>
            )}
          <Surface className="r-current-world-strip">
            <span className="r-soft-icon">
              <Globe2Icon size={21} />
            </span>
            <div>
              <h3>
                {m.c.name}的当前世界：
                {m.activeWorld?.name ||
                  (m.catalog.ready ? "尚未绑定" : "待读取")}
              </h3>
              <p>实际位置与活动以此刻记录为准。</p>
            </div>
            <Button
              variant="outline"
              onClick={() =>
                go("/c/" + encodeURIComponent(m.fid) + "/life/now")
              }
            >
              看看此刻
              <ArrowUpRightIcon size={14} />
            </Button>
          </Surface>
        </>
      ) : (
        <>
          <div className="r-breadcrumb">
            <Button variant="ghost" size="sm" onClick={m.back}>
              <ArrowLeftIcon size={14} />
              全部世界
            </Button>
            <ChevronRightIcon size={12} />
            <span>{m.world.name}</span>
          </div>
          <Surface className="r-world-hero">
            <span className="r-world-hero-art" aria-hidden="true">
              <WorldArt kind={decorativeKind(m.world)} />
            </span>
            <div>
              <Tiny>{m.world.flavor}</Tiny>
              <h2>一座属于日常的世界</h2>
              <p>{m.world.background || "这个世界的背景尚未补充。"}</p>
              <div className="r-inline-actions">
                <Badge variant="outline">
                  {m.world.id === m.binding?.world_id
                    ? "当前运行世界"
                    : "可供选择"}
                </Badge>
                <span className="r-muted">
                  {groups.length} 类场地 · {places(m.world.id).length} 处地点
                </span>
              </div>
              <div className="r-inline-actions">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={m.readOnly}
                  onClick={(e) => m.editWorld(m.world, e)}
                >
                  编辑世界
                </Button>
                {m.world.id !== m.binding?.world_id && (
                  <Button
                    size="sm"
                    disabled={m.readOnly}
                    onClick={m.selectWorld}
                  >
                    切换到此世界
                  </Button>
                )}
              </div>
            </div>
          </Surface>
          <Tabs value={m.tab} onValueChange={m.setTab}>
            <TabsList variant="line" className="replica-segment-tabs">
              <TabsTrigger value="places">
                场地与地点 <span>{places(m.world.id).length}</span>
              </TabsTrigger>
              <TabsTrigger value="settings">世界设定</TabsTrigger>
              <TabsTrigger value="items">物品目录</TabsTrigger>
              <TabsTrigger value="events">世界事件</TabsTrigger>
            </TabsList>
            <TabsContent value="places">
              <div className="r-place-workspace">
                <Surface className="r-place-tree">
                  <h3>场地分类</h3>
                  <Button
                    variant="ghost"
                    className={m.category === "all" ? "active" : ""}
                    onClick={() => m.setCategory("all")}
                  >
                    <Globe2Icon size={15} />
                    全部地点
                    <span>
                      {groups.reduce((n, g) => n + g.places.length, 0)}
                    </span>
                  </Button>
                  {groups.map((g) => (
                    <Button
                      variant="ghost"
                      key={g.id}
                      className={m.category === g.id ? "active" : ""}
                      onClick={() => m.setCategory(g.id)}
                    >
                      <FolderOpenIcon size={15} />
                      {g.name}
                      <span>{g.places.length}</span>
                    </Button>
                  ))}
                  <Button
                    variant="ghost"
                    size="sm"
                    className="r-add-category"
                    disabled={m.readOnly}
                    onClick={m.addCategory}
                  >
                    <PlusIcon size={13} />
                    添加分类
                  </Button>
                </Surface>
                <div>
                  <div className="r-toolbar">
                    <Search
                      label="搜索场地或地点"
                      value={m.search}
                      onChange={m.setSearch}
                    />
                    <Button
                      size="sm"
                      disabled={m.readOnly || !m.world.venue_categories?.length}
                      onClick={(e) =>
                        m.editPlace(
                          null,
                          m.category === "all" || m.category === "unclassified"
                            ? undefined
                            : m.category,
                          e,
                        )
                      }
                    >
                      <PlusIcon size={14} />
                      添加地点
                    </Button>
                    <label className="r-inline-check">
                      <Checkbox
                        checked={archived}
                        onCheckedChange={(v) => setArchived(!!v)}
                      />
                      显示已归档地点
                    </label>
                  </div>
                  <div className="r-place-grid">
                    {filtered.map(({ p, group }) => (
                      <Surface className="r-place-card" key={p.id}>
                        <div className="r-place-top">
                          <span className="r-soft-icon">
                            <MapPinIcon size={18} />
                          </span>
                          <Tiny>
                            {group.name} · {p.floor || "未标注"}
                          </Tiny>
                        </div>
                        <h3>
                          {p.alias || p.name}
                          {p.status === "archived" && (
                            <Badge variant="outline">已归档</Badge>
                          )}
                        </h3>
                        <p>{p.desc || "尚未提供场景描述。"}</p>
                        <div className="r-place-detail">
                          <span>{p.atmosphere}</span>
                          <SemanticTree
                            value={
                              Array.isArray(p.furniture) &&
                              p.furniture.every(
                                (value) => typeof value === "string",
                              )
                                ? p.furniture.join("、")
                                : p.furniture
                            }
                          />
                        </div>
                        <div className="r-card-footer">
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={(e) =>
                              m.setHistory({
                                place: p,
                                world: m.world,
                                trigger: e.currentTarget,
                              })
                            }
                          >
                            <HistoryIcon size={13} />
                            修订记录
                          </Button>
                          <Button
                            variant="outline"
                            size="sm"
                            disabled={m.readOnly}
                            onClick={(e) => m.editPlace(p, group.id, e)}
                          >
                            查看与编辑
                            <ArrowUpRightIcon size={13} />
                          </Button>
                        </div>
                      </Surface>
                    ))}
                  </div>
                  {!groups.length ? (
                    <EmptyState>
                      <h3>世界还没有场地</h3>
                      <p>先创建一个分类，再把具体地点放进去。</p>
                      <Button disabled={m.readOnly} onClick={m.addCategory}>
                        创建场地分类
                      </Button>
                    </EmptyState>
                  ) : (
                    !filtered.length && (
                      <EmptyState>
                        <h3>
                          {m.search
                            ? "没有找到匹配的地点"
                            : "这个分类还没有地点"}
                        </h3>
                        {m.search && (
                          <Button
                            variant="outline"
                            onClick={() => m.setSearch("")}
                          >
                            清除地点搜索
                          </Button>
                        )}
                      </EmptyState>
                    )
                  )}
                </div>
              </div>
            </TabsContent>
            <TabsContent value="settings">
              <div className="r-two-column">
                <Surface className="r-padded">
                  <div className="section-heading">
                    <h2>世界背景</h2>
                    <Button
                      variant="ghost"
                      size="sm"
                      disabled={m.readOnly}
                      onClick={(e) => m.editWorld(m.world, e)}
                    >
                      编辑
                    </Button>
                  </div>
                  <p className="r-prose">{m.world.background || "尚未补充"}</p>
                  <h3 className="r-subtitle">基础规则</h3>
                  <p className="r-prose">{m.world.rules || "尚未补充"}</p>
                  <Disclosure title="完整世界资料与来源">
                    <JsonDetails value={m.world} title="当前世界完整记录" />
                  </Disclosure>
                  <div className="r-inline-actions">
                    <Button
                      variant="outline"
                      disabled={
                        m.readOnly || m.world.id === m.binding?.world_id
                      }
                      onClick={m.removeWorld}
                    >
                      移除无引用世界
                    </Button>
                    <Button variant="link" asChild>
                      <a
                        href={
                          "#/c/" +
                          encodeURIComponent(m.fid) +
                          "/worlds/" +
                          encodeURIComponent(m.world.id) +
                          "/edit"
                        }
                      >
                        完整世界工具
                        <ArrowUpRightIcon size={13} />
                      </a>
                    </Button>
                  </div>
                </Surface>
                <Surface className="r-padded">
                  <div className="section-heading">
                    <h2>在这里生活</h2>
                  </div>
                  {residents(m.world.id).map((p) => (
                    <Button
                      variant="ghost"
                      className="r-person-row"
                      key={p.id}
                      onClick={() =>
                        go(
                          p.fluctlight
                            ? "/c/" + encodeURIComponent(p.id) + "/profile"
                            : "/c/" +
                                encodeURIComponent(m.fid) +
                                "/people/npc/" +
                                encodeURIComponent(p.id),
                        )
                      }
                    >
                      <CharacterAvatar
                        name={p.name}
                        initial={[...p.name].at(-1)}
                      />
                      <span>
                        <b>{p.name}</b>
                        <small>{p.role || p.occupation}</small>
                      </span>
                      <ArrowUpRightIcon size={14} />
                    </Button>
                  ))}
                  {!residents(m.world.id).length && (
                    <p className="r-muted">尚未读到在此生活的人物。</p>
                  )}
                  <WorkspaceInfo>
                    浏览和编辑世界，不会移动角色的当前位置。
                  </WorkspaceInfo>
                  <Disclosure title="扩写已有场地与地点">
                    <WorldField
                      label="扩展方向"
                      value={m.instruction}
                      onChange={m.setInstruction}
                    />
                    <Button
                      variant="outline"
                      disabled={m.readOnly || m.busy}
                      onClick={() => void m.expandWorld()}
                    >
                      AI 扩展世界
                    </Button>
                    <p className="r-fineprint">
                      使用实际创作任务的结果，保留原任务和分阶段提交。
                    </p>
                  </Disclosure>
                </Surface>
              </div>
            </TabsContent>
            <TabsContent value="items">
              {m.world.id === m.binding?.world_id ? (
                <WorldItemsPanel key={m.world.id} model={m} />
              ) : (
                <WorkspaceInfo>
                  物品目录随当前运行世界读取。当前正在浏览另一个世界，切换后可查看该世界的目录。
                </WorkspaceInfo>
              )}
            </TabsContent>
            <TabsContent value="events">
              {m.world.id === m.binding?.world_id ? (
                <WorldEventsPanel key={m.world.id} model={m} />
              ) : (
                <WorkspaceInfo>
                  事件列表随当前运行世界读取。浏览这里的世界资料不会触发事件或切换世界。
                </WorkspaceInfo>
              )}
            </TabsContent>
          </Tabs>
        </>
      )}
      <ActionFeedback labels={["世界"]} />
      {m.history && (
        <WorldPlaceHistory
          model={m}
          selection={m.history}
          onClose={() => m.setHistory(null)}
        />
      )}
    </div>
  );
}

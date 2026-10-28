import {
  ChevronRightIcon,
  MapPinIcon,
  PlusIcon,
  RefreshCwIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { ChoiceSelect, FieldInput, Badge } from "../controls";
import WorkspacePageHeader from "../shared/WorkspacePageHeader";
import WorkspaceInfo from "../shared/WorkspaceInfo";
import { EmptyState } from "../shared/Presentation";
import { PeopleReader } from "../shared/PeopleParts";
import AboutYouPanel from "../shared/AboutYouPanel";
import PersonDetail from "../shared/PersonDetail";
import PersonDefinition from "../shared/PersonDefinition";
import {
  relationshipFor,
  npcLocation,
} from "@/approved-ui/people-presentation.mjs";
export default function PeoplePageView({ model: m }) {
  if (!m.fid)
    return (
      <WorkspaceInfo>
        先选择一位角色，再查看世界人物和关于你的资料。
        <Button variant="outline" onClick={() => m.w.navigatePath("/library")}>
          打开角色库
        </Button>
      </WorkspaceInfo>
    );
  const profile = m.profile.items[0],
    world = m.catalog.extra.world,
    places = m.catalog.extra.places || [],
    activeProfile = profile?.personality_runtime?.active_profile_id;
  const rows = m.catalog.items
    .map((npc) => ({
      npc,
      relation: relationshipFor(
        profile?.relationships,
        npc.actor_id,
        activeProfile,
        m.fid,
      ),
    }))
    .filter(
      ({ npc, relation }) =>
        (npc.name + " " + npc.role + " " + npc.bio)
          .toLocaleLowerCase()
          .includes(m.search.trim().toLocaleLowerCase()) &&
        (m.filter === "archived"
          ? npc.status === "archived"
          : npc.status !== "archived") &&
        (m.filter === "known"
          ? !!relation
          : m.filter === "unknown"
            ? !relation
            : true),
    );
  const clear = () => {
    m.setSearch("");
    m.setFilter("all");
  };
  return (
    <div className="replica-profile replica-people companion-page">
      <WorkspacePageHeader
        eyebrow="WORLD / PEOPLE & CONNECTIONS"
        title="每一次相遇，都留下联系。"
        description={
          (world?.name ? world.name + "里的" : "世界中的") +
          "人，以及你们之间慢慢建立的了解。"
        }
        actions={
          <>
            <Button
              variant="ghost"
              size="icon"
              aria-label="刷新人物目录"
              disabled={m.catalog.busy || m.profile.busy}
              onClick={m.refresh}
            >
              <RefreshCwIcon size={15} />
            </Button>
            <Button
              disabled={m.readOnly}
              onClick={(e) => {
                m.trigger.current = e.currentTarget;
                m.setDefinitionOpen(true);
              }}
            >
              <PlusIcon size={15} />
              新增世界人物
            </Button>
          </>
        }
      />
      <Tabs
        value={m.tab}
        onValueChange={m.setTab}
        className="profile-main-tabs"
      >
        <TabsList variant="line" className="replica-segment-tabs">
          <TabsTrigger value="people">
            世界中的人{" "}
            <span>{m.catalog.ready ? m.catalog.items.length : "…"}</span>
          </TabsTrigger>
          <TabsTrigger value="you">
            {m.actorId === m.owner ? "关于你" : "参与者资料"}
          </TabsTrigger>
          <TabsTrigger value="relationship">
            {m.actorId === m.owner ? "你们的关系" : "与该参与者的关系"}
          </TabsTrigger>
        </TabsList>
        <TabsContent value="people">
          <div className="people-tab-body">
            <PeopleReader source={m.catalog} label="世界人物" />
            <PeopleReader source={m.profile} label="关系范围" />
            <div className="companion-toolbar">
              <div className="people-search">
                <FieldInput
                  type="search"
                  aria-label="搜索名字、身份或故事"
                  placeholder="搜索名字、身份或故事…"
                  value={m.search}
                  onChange={(e) => m.setSearch(e.target.value)}
                />
              </div>
              <ChoiceSelect
                aria-label="人物范围"
                disabled={!m.profile.ready}
                value={m.filter}
                onChange={(e) => m.setFilter(e.target.value)}
              >
                <option value="all">全部人物</option>
                <option value="known">已经结识</option>
                <option value="unknown">尚未结识</option>
                <option value="archived">已归档人物</option>
              </ChoiceSelect>
            </div>
            <div className="companion-people-grid">
              {rows.map(({ npc, relation }) => {
                const location = npcLocation(npc, places);
                return (
                  <Button
                    variant="ghost"
                    className="companion-person-card"
                    key={npc.id}
                    aria-label={"查看人物 " + npc.name}
                    onClick={(e) => {
                      m.trigger.current = e.currentTarget;
                      m.setSelected(npc.id);
                    }}
                  >
                    <div className="companion-row-between">
                      <span className="companion-person-avatar">
                        {[...npc.name].at(-1)}
                      </span>
                      <Badge
                        variant="outline"
                        data-known={m.profile.ready ? !!relation : undefined}
                      >
                        {!m.profile.ready
                          ? "关系待读取"
                          : relation
                            ? "已经结识"
                            : "尚未结识"}
                      </Badge>
                      {npc.status === "archived" && (
                        <Badge variant="outline">已归档</Badge>
                      )}
                    </div>
                    <h3>{npc.name}</h3>
                    <span className="companion-person-job">
                      {npc.role || "身份尚未提供"}
                    </span>
                    <p>{npc.bio || "人物背景尚未提供。"}</p>
                    <div className="companion-person-bottom">
                      <span>
                        <MapPinIcon size={12} />
                        {location.label} · {location.name}
                      </span>
                      <ChevronRightIcon size={15} />
                    </div>
                  </Button>
                );
              })}
            </div>
            {m.catalog.ready && !rows.length && !m.catalog.error && (
              <EmptyState>
                <h3>
                  {m.search || m.filter !== "all"
                    ? "没有找到这个人"
                    : "这个世界还没有已读人物"}
                </h3>
                <p>
                  {m.search || m.filter !== "all"
                    ? "试试其他名字，或清除当前筛选。"
                    : "可以先保存人物定义，再发布并明确应用到世界。"}
                </p>
                {(m.search || m.filter !== "all") && (
                  <Button variant="outline" onClick={clear}>
                    清除筛选
                  </Button>
                )}
              </EmptyState>
            )}
            <WorkspaceInfo>
              加入人物目录不代表已经相识。关系与接触记录以实际保存的来源为准，预计地点也不等于实际在场。
            </WorkspaceInfo>
          </div>
        </TabsContent>
        <TabsContent value="you">
          <AboutYouPanel key={m.actorId} model={m} />
        </TabsContent>
        <TabsContent value="relationship">
          <AboutYouPanel
            key={m.actorId + ":relationship"}
            model={m}
            section="relationship"
          />
        </TabsContent>
      </Tabs>
      {m.selected && (
        <PersonDetail
          key={m.selected}
          model={m}
          npcId={m.selected}
          initialEdit={m.initialEdit}
          onClose={() => m.setSelected(null)}
        />
      )}
      {m.definitionOpen && (
        <PersonDefinition
          model={m}
          onClose={() => m.setDefinitionOpen(false)}
        />
      )}
    </div>
  );
}

import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import { useWorkspace } from "../page";
import useWorkspaceRead from "../shared/useWorkspaceRead";
import PeoplePageView from "../views/PeoplePage";
import { profileSnapshotFromDetail } from "@/v3/proto/data/profile.js";
/** @param {{route?: {query?: URLSearchParams}, initialTab?: string, initialNpcId?: string, initialEdit?: boolean, initialCreate?: boolean}} [input] */
export default function PeoplePage({
  route,
  initialTab,
  initialNpcId,
  initialEdit = false,
  initialCreate = false,
} = {}) {
  const w = useWorkspace();
  useSyncExternalStore(w.subscribe, w.getSnapshot, w.getSnapshot);
  const c = w.current.value,
    fid = c.id,
    owner = w.state.session.actorId,
    actorQuery = route?.query?.get("actor"),
    actorId = !actorQuery || actorQuery === "owner" ? owner : actorQuery;
  const tabFromRoute = () =>
    initialTab ||
    (["you", "relationship", "people"].includes(route?.query?.get("tab"))
      ? route.query.get("tab")
      : actorQuery
        ? "you"
        : "people");
  const [tab, setTab] = useState(tabFromRoute),
    [search, setSearch] = useState(""),
    [filter, setFilter] = useState("all"),
    [selected, setSelected] = useState(initialNpcId || null),
    [definitionOpen, setDefinitionOpen] = useState(initialCreate);
  const scope = JSON.stringify([owner, fid, actorId]),
    scopeRef = useRef(scope),
    active = useRef(false),
    trigger = useRef(null),
    catalogRef = useRef(null),
    profileRef = useRef(null);
  scopeRef.current = scope;
  useEffect(() => {
    active.current = true;
    return () => {
      active.current = false;
    };
  }, []);
  useEffect(() => {
    setTab(tabFromRoute());
    setSelected(initialNpcId || null);
    setDefinitionOpen(initialCreate);
  }, [
    scope,
    route?.query?.toString(),
    initialTab,
    initialNpcId,
    initialCreate,
    initialEdit,
  ]);
  const live = () =>
    active.current && !!fid && w.isCurrent(owner, fid) && scopeRef.current === scope;
  const readProfile = async () => {
    const row = await w.client.detail(fid);
    if (row.id !== fid) throw Error("角色资料范围不匹配。");
    return row;
  };
  const profile = useWorkspaceRead({
    identity: ["people-profile"],
    read: async () => ({ items: [await readProfile()], next_cursor: "" }),
  });
  profileRef.current = profile;
  async function readCatalog() {
    const [page, view] = await Promise.all([
      w.client.listRuntimeNPCs(fid),
      w.client.world(fid),
    ]);
    if (!Array.isArray(page.items) || page.next_cursor)
      throw Error("人物目录尚未完整返回。");
    if (view.world && view.world.fluctlight_id !== fid)
      throw Error("当前世界不属于此角色。");
    if (
      page.items.some(
        (row) => !row.id || !row.actor_id || row.world_id !== view.world?.id,
      ) ||
      !Array.isArray(view.places) ||
      view.places.some((row) => row.world_id !== view.world?.id)
    )
      throw Error("人物、地点与当前世界范围不匹配，请重新读取。");
    return { ...page, world: view.world, places: view.places };
  }
  const catalog = useWorkspaceRead({
    identity: ["people-catalog"],
    read: readCatalog,
  });
  catalogRef.current = catalog;
  const readOnly =
    ["retired", "archived", "已退休", "已归档"].includes(c.status) ||
    ["retired", "archived"].includes(profile.items[0]?.status);
  function assertWritable() {
    if (
      !live() ||
      ["retired", "archived", "已退休", "已归档"].includes(
        w.current.value.status,
      ) ||
      ["retired", "archived"].includes(profileRef.current.items[0]?.status)
    )
      throw Error("账号、角色或可写状态已经变化。");
  }
  async function adoptNpc(row) {
    if (!live()) return;
    const source = catalogRef.current;
    if (row.world_id !== source.extra.world?.id)
      throw Error("人物已不在当前读取的世界中，请刷新目录。");
    source.adopt({
      ...source.extra,
      items: source.items.some((p) => p.id === row.id)
        ? source.items.map((p) => (p.id === row.id ? row : p))
        : [...source.items, row],
      next_cursor: "",
    });
    await source.load();
  }
  const displayName = profile.items[0]?.identity?.name || c.name;
  const displayCharacter = {
    ...c,
    name: displayName,
    initial: [...displayName][0],
    avatar: profileSnapshotFromDetail(profile.items[0] || {}, c._directory)
      .avatar,
  };
  const model = {
    w,
    c: displayCharacter,
    fid,
    owner,
    actorId,
    live,
    scope,
    profile,
    catalog,
    readOnly,
    assertWritable,
    readProfile,
    readCatalog,
    adoptNpc,
    tab,
    setTab,
    search,
    setSearch,
    filter,
    setFilter,
    selected,
    setSelected,
    definitionOpen,
    setDefinitionOpen,
    initialEdit,
    trigger,
    refresh: () => Promise.all([profile.load(), catalog.load()]),
  };
  return <PeoplePageView model={model} />;
}

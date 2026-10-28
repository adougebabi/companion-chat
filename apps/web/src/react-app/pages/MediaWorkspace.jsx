import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import { useWorkspace } from "../page";
import useWorkspaceRead from "../shared/useWorkspaceRead";
import MediaWorkspaceView from "../views/MediaWorkspace";
import { checkMediaAsset } from "@/approved-ui/media-presentation.mjs";
import { sourceReturn } from "@/v3/proto/components/source-navigation.js";
/** @param {{route:any,initialTab?:string,initialAssetId?:string,initialEdit?:boolean}} input */
export default function MediaWorkspace({
  route,
  initialTab,
  initialAssetId,
  initialEdit = false,
}) {
  const w = useWorkspace();
  useSyncExternalStore(w.subscribe, w.getSnapshot, w.getSnapshot);
  const scopedFid = /^\/c\/[^/]+\/media(?:\/|$)/.test(route.path || "")
    ? route.fid || ""
    : "";
  const owner = w.state.session.actorId,
    scope = JSON.stringify([
      owner,
      route.fid || "",
      route.query?.toString(),
      initialTab,
      initialAssetId,
    ]);
  const fromRoute = () =>
    initialTab ||
    (["gallery", "identity", "generate", "avatar"].includes(
      route.query?.get("tab"),
    )
      ? route.query.get("tab")
      : "gallery");
  const initialFilter = () => ({
    q: route.query?.get("q") || "",
    fid: scopedFid || route.query?.get("fid") || "",
    kind: route.query?.get("kind") || "",
    origin: route.query?.get("scope") || "all",
    hidden: route.query?.get("hidden") === "true",
  });
  const [tab, setTab] = useState(fromRoute),
    [target, setTarget] = useState(
      scopedFid || route.query?.get("fid") || w.current?.value?.id || "",
    ),
    [search, setSearch] = useState(route.query?.get("q") || ""),
    [filter, setFilter] = useState(initialFilter),
    [selected, setSelected] = useState(initialAssetId || null),
    [uploadOpen, setUploadOpen] = useState(false);
  const active = useRef(false),
    scopeRef = useRef(scope),
    trigger = useRef(null),
    libraryRef = useRef(null);
  scopeRef.current = JSON.stringify([scope, target]);
  useEffect(() => {
    active.current = true;
    return () => {
      active.current = false;
    };
  }, []);
  useEffect(() => {
    setTab(fromRoute());
    setTarget(
      scopedFid || route.query?.get("fid") || w.current?.value?.id || "",
    );
    setSelected(initialAssetId || null);
    setSearch(route.query?.get("q") || "");
    setFilter(initialFilter());
  }, [scope]);
  useEffect(() => {
    if (!scopedFid && !route.query?.get("fid"))
      setTarget(w.current?.value?.id || "");
  }, [w.current?.value?.id, scopedFid, route.query?.get("fid")]);
  const live = () =>
    active.current &&
    scopeRef.current === JSON.stringify([scope, target]) &&
    w.state.session.actorId === owner;
  const assertCurrent = () => {
    if (!live()) throw Error("账号或媒体范围已经变化，请返回原范围核对。");
  };
  const actor = w.state.characters.find((c) => c.id === target),
    readOnly =
      !target ||
      !actor ||
      ["retired", "archived", "已退休", "已归档"].includes(actor?.status) ||
      ["retired", "archived"].includes(actor?._directory?.status);
  const assertWritable = () => {
    assertCurrent();
    if (
      !w.state.characters.some(
        (c) =>
          c.id === target &&
          !["retired", "archived", "已退休", "已归档"].includes(c.status),
      )
    )
      throw Error("当前角色已不可写，请先核对。");
  };
  const service = w.services.mediaClient;
  const library = useWorkspaceRead({
    identity: ["media-library", filter],
    characterScoped: false,
    read: (cursor) =>
      service.list({
        q: filter.q || undefined,
        fluctlightId: filter.fid || undefined,
        kind: filter.kind || undefined,
        scope: filter.origin,
        hidden: filter.hidden,
        cursor,
      }),
  });
  libraryRef.current = library;
  const readAsset = async (id, explicitFid = "") =>
    checkMediaAsset(await service.asset(id, explicitFid || undefined), id);
  function adoptAsset(row) {
    if (!live()) return;
    const s = libraryRef.current,
      previous = s.items.find((a) => a.id === row.id);
    if (
      previous &&
      (previous.asset_revision > row.asset_revision ||
        previous.metadata_revision > row.metadata_revision)
    )
      return;
    const matches = row.status !== "deleted" && row.hidden === filter.hidden;
    const items = s.items.flatMap((a) =>
      a.id === row.id ? (matches ? [row] : []) : [a],
    );
    s.adopt({ ...s.extra, items, next_cursor: s.cursor });
  }
  const model = {
    w,
    owner,
    scope,
    live,
    assertCurrent,
    assertWritable,
    readOnly,
    service,
    target,
    actor,
    setTarget,
    tab,
    setTab,
    filter,
    setFilter,
    search,
    setSearch,
    selected,
    setSelected,
    uploadOpen,
    setUploadOpen,
    trigger,
    library,
    readAsset,
    adoptAsset,
    initialEdit,
    initialAssetId,
    route,
    openAsset: (id, source) => {
      trigger.current = source;
      setSelected(id);
    },
    closeAsset: () => {
      setSelected(null);
      if (initialAssetId)
        w.navigatePath(
          sourceReturn(
            { query: Object.fromEntries(route.query || []) },
            scopedFid
              ? "#/c/" + encodeURIComponent(scopedFid) + "/media"
              : "#/media",
          ).slice(1),
        );
    },
  };
  return <MediaWorkspaceView model={model} />;
}

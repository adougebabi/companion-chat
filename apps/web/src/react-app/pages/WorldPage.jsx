import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import { useWorkspace, afterPaint } from "../page";
import { WorldPageView } from "../views/WorldPage";
import WorldField, {
  WorldForm,
  WorldEditFields,
  PlaceEditFields,
} from "../shared/WorldFields";
import WorkspaceCommandDialog from "../shared/WorkspaceCommandDialog";
import useWorkspaceRead from "../shared/useWorkspaceRead";
import {
  worldDraft,
  worldWrite,
  placeDraft,
  placeWrite,
  acknowledgeWorldDraft,
  rebaseWorldDraft,
  acknowledgePlaceDraft,
  rebasePlaceDraft,
} from "@/approved-ui/world-fields.mjs";
import { runWorldExpansion } from "@/approved-ui/world-expansion.mjs";
import { randomId } from "@/random-id.mjs";
import { errorMessage } from "@/v3/domain-presentation.mjs";
export default function WorldPage({ initialWorldId = "" } = {}) {
  const w = useWorkspace();
  useSyncExternalStore(w.subscribe, w.getSnapshot, w.getSnapshot);
  const c = w.current.value,
    fid = c.id,
    owner = w.state.session.actorId;
  function readCreateRecovery() {
    const key = `fluctlight-world-framework:${owner}:${fid}:replica:world:new`;
    try {
      return [":draft", ":command", ":command:outcome"].some((part) =>
        sessionStorage.getItem(key + part),
      );
    } catch {
      return false;
    }
  }
  const [selected, setSelected] = useState(initialWorldId),
    [tab, setTab] = useState("places"),
    [category, setCategory] = useState("all"),
    [query, setQuery] = useState(""),
    [search, setSearch] = useState(""),
    [action, setAction] = useState(null),
    [history, setHistory] = useState(null),
    [instruction, setInstruction] = useState(""),
    [notice, setNotice] = useState(""),
    [error, setError] = useState(""),
    [createRecovery, setCreateRecovery] = useState(readCreateRecovery);
  function dismissAction() {
    setAction(null);
    setCreateRecovery(readCreateRecovery());
  }
  const active = useRef(true),
    trigger = useRef(null),
    listTop = useRef(0),
    catalogRef = useRef(null);
  useEffect(() => {
    active.current = true;
    return () => {
      active.current = false;
    };
  }, []);
  const live = () => active.current && w.isCurrent(owner, fid);
  const readOnly =
    ["retired", "archived", "已退休", "已归档"].includes(c.status) ||
    ["retired", "archived"].includes(c.detail?.status);
  const assertWritable = () => {
    if (
      !live() ||
      ["retired", "archived", "已退休", "已归档"].includes(
        w.current.value.status,
      ) ||
      ["retired", "archived"].includes(w.current.value.detail?.status)
    )
      throw Error("账号、角色或可写状态已经变化，请回到原范围核对。");
  };
  async function readCatalog() {
    const page = await w.client.worldCatalog(fid);
    if (
      !Array.isArray(page.items)
    )
      throw Error("世界目录范围尚未完整确认。");
    const views = await Promise.all(
      page.items.map(async (row) => {
        const view = await w.client.catalogWorld(fid, row.id);
        if (
          view.world?.id !== row.id ||
          !Array.isArray(view.places) ||
          view.places.some((p) => p.world_id !== row.id) ||
          !Array.isArray(view.npcs) ||
          view.npcs.some((p) => p.world_id !== row.id)
        )
          throw Error("世界、地点与人物范围不匹配。");
        return [row.id, view];
      }),
    );
    return { ...page, snapshots: Object.fromEntries(views) };
  }
  const catalog = useWorkspaceRead({
    identity: ["world-catalog"],
    read: readCatalog,
  });
  catalogRef.current = catalog;
  const world = catalog.items.find((row) => row.id === selected),
    snapshot = catalog.extra.snapshots?.[selected],
    binding = catalog.extra.active_binding,
    activeWorld = catalog.items.find((row) => row.id === binding?.world_id);
  const busy = w.state.operations.some(
    (row) =>
      row.fid === fid && row.label === "世界" && row.status === "pending",
  );
  function openWorld(id) {
    listTop.current = document.querySelector(".workspace-page")?.scrollTop || 0;
    setSelected(id);
    setCategory("all");
    setSearch("");
    setTab("places");
    void afterPaint(() =>
      document.querySelector(".workspace-page")?.scrollTo({ top: 0 }),
    );
  }
  function back() {
    setSelected("");
    void afterPaint(() =>
      document
        .querySelector(".workspace-page")
        ?.scrollTo({ top: listTop.current }),
    );
  }
  function start(identity, spec, event) {
    assertWritable();
    if (event) trigger.current = event.currentTarget;
    setAction({
      identity,
      scope: ["world", fid, identity],
      storageKey: `fluctlight-world-framework:${owner}:${fid}:replica:${identity}`,
      ...spec,
    });
  }
  function verifyResult(result, id, status) {
    if (
      !result?.id ||
      (id && result.id !== id) ||
      (status && result.status !== status) ||
      !Number.isSafeInteger(result.revision)
    )
      throw Error("原提交回执尚未完整确认，请核对原请求。");
    return result;
  }
  async function savedWorld(
    result,
    { deleted = false, selectedWorld = false } = {},
  ) {
    const page = await readCatalog();
    if (!live()) return;
    if (deleted) {
      if (page.items.some((row) => row.id === result.id))
        throw Error("世界移除结果尚未读回。");
    } else if (selectedWorld) {
      const b = page.active_binding;
      if (
        !b ||
        b.revision < result.revision ||
        (b.revision === result.revision && b.world_id !== result.world_id)
      )
        throw Error("世界绑定版本尚未读回。");
    } else {
      const row = page.items.find((row) => row.id === result.id);
      if (!row || row.revision < result.revision)
        throw Error("世界保存版本尚未读回。");
    }
    catalogRef.current.adopt(page);
    if(selectedWorld){
      await w.detail();
      if(!live())return;
    }
    setNotice(
      selectedWorld
        ? "世界绑定已读回，角色的实际地点仍以生活记录为准。"
        : "已读回实际世界资料与地点。",
    );
    if (deleted && selected === result.id) setSelected("");
  }
  function editWorld(row, event) {
    const id = row?.id || null;
    start(
      "world:" + (id || "new"),
      {
        title: id ? "编辑世界资料" : "创建一个新世界",
        description: "让名字、背景和规则，成为这个世界的起点。",
        submitLabel: "保存世界",
        initial: worldDraft(row),
        validate: worldWrite,
        build: worldWrite,
        execute: async (value, key) => {
          assertWritable();
          return verifyResult(
            await (value.id
              ? w.client.updateCatalogWorld(fid, value.id, {
                  ...value.body,
                  idempotencyKey: key,
                })
              : w.client.createCatalogWorld(fid, {
                  ...value.body,
                  idempotencyKey: key,
                })),
            value.id,
          );
        },
        afterAcknowledged: acknowledgeWorldDraft,
        readLatest: async (draft) =>
          draft.id ? (await w.client.catalogWorld(fid, draft.id)).world : null,
        adoptLatest: rebaseWorldDraft,
        onSaved: async (result) => {
          await savedWorld(result);
          if (live() && !id) setSelected(result.id);
        },
        fields: (d, edit) => <WorldEditFields draft={d} edit={edit} />,
      },
      event,
    );
  }
  function addCategory(event) {
    if (!world) return;
    const id = world.id;
    start(
      "category:" + id,
      {
        title: "添加场地分类",
        description: "把相关的地点放在一起，例如街区、自然或生活空间。",
        submitLabel: "添加分类",
        initial: {
          name: "",
          revision: world.revision,
          worldName: world.name,
          categories: world.venue_categories || [],
          categoryId: "venue_" + randomId(),
        },
        validate: (d) => {
          if (!d.name.trim() || [...d.name.trim()].length > 128)
            throw Error("请填写分类名称，最多128个字符。");
          if (
            d.categories.length >= 32 ||
            d.categories.some((row) => row.name === d.name.trim())
          )
            throw Error("已有同名分类或分类数量已满。");
        },
        build: (d) => ({
          name: d.worldName,
          expectedRevision: d.revision,
          venueCategories: [
            ...d.categories,
            { id: d.categoryId, name: d.name.trim() },
          ],
          reason: "所有者新增场地分类",
        }),
        execute: async (body, key) => {
          assertWritable();
          return verifyResult(
            await w.client.updateCatalogWorld(fid, id, {
              ...body,
              idempotencyKey: key,
            }),
            id,
          );
        },
        readLatest: async () => (await w.client.catalogWorld(fid, id)).world,
        adoptLatest: (draft, row) => ({
          ...draft,
          revision: row.revision,
          worldName: row.name,
          categories: row.venue_categories || [],
        }),
        onSaved: savedWorld,
        fields: (d, edit) => (
          <WorldForm>
            <WorldField
              label="分类名称"
              required
              value={d.name}
              onChange={(name) => edit({ name })}
            />
          </WorldForm>
        ),
      },
      event,
    );
  }
  function editPlace(row, categoryId, event) {
    if (!world) return;
    const wid = world.id,
      id = row?.id || null,
      categories = world.venue_categories || [],
      places = snapshot.places;
    start(
      "place:" + wid + ":" + (id || "new"),
      {
        title: id ? row.alias || row.name : "添加具体地点",
        description:
          "编辑地点资料。已有正式名称和分类保持固定，别名可用于补充称呼。",
        wide: true,
        submitLabel: "保存地点",
        initial: placeDraft(row, world, categoryId),
        validate: placeWrite,
        build: placeWrite,
        execute: async (value, key) => {
          assertWritable();
          return verifyResult(
            await (value.id
              ? w.client.updateWorldPlace(fid, value.id, {
                  ...value.body,
                  idempotencyKey: key,
                })
              : w.client.createWorldPlace(fid, {
                  ...value.body,
                  idempotencyKey: key,
                })),
            value.id,
          );
        },
        afterAcknowledged: acknowledgePlaceDraft,
        readLatest: async (draft) => {
          const view = await w.client.catalogWorld(fid, wid);
          const row = draft.id
            ? view.places.find((p) => p.id === draft.id)
            : null;
          if (draft.id && !row) throw Error("原地点不在当前世界中。");
          return row;
        },
        adoptLatest: rebasePlaceDraft,
        onSaved: async (result) => {
          const page = await readCatalog(),
            p = page.snapshots[wid]?.places.find((p) => p.id === result.id);
          if (!p || p.revision < result.revision)
            throw Error("地点保存版本尚未读回。");
          if (live()) {
            catalogRef.current.adopt(page);
            setNotice("地点已保存，实际位置仍以生活记录为准。");
          }
        },
        fields: (d, edit) => (
          <PlaceEditFields
            draft={d}
            edit={edit}
            categories={categories}
            places={places}
            onHistory={() => {
              const p =
                catalogRef.current.extra.snapshots?.[wid]?.places.find(
                  (p) => p.id === d.id,
                ) || d.source;
              if (p) {
                setAction(null);
                setHistory({ place: p, world, trigger: trigger.current });
              }
            }}
          />
        ),
      },
      event,
    );
  }
  function selectWorld(event) {
    if (!world) return;
    const id = world.id;
    start(
      "select:" + id,
      {
        title: "切换到「" + world.name + "」？",
        description: "保留原世界的生活记录。切换后，角色的位置需要重新确认。",
        submitLabel: "切换世界",
        initial: { revision: binding?.revision || 0 },
        build: (d) => ({
          worldId: id,
          expectedRevision: d.revision,
          reason: "所有者显式切换世界",
        }),
        execute: async (body, key) => {
          assertWritable();
          const result = verifyResult(
            await w.client.selectWorld(fid, { ...body, idempotencyKey: key }),
            id,
            "selected",
          );
          if (result.world_id !== id) throw Error("世界绑定回执范围不匹配。");
          return result;
        },
        readLatest: readCatalog,
        adoptLatest: (d, p) => ({
          ...d,
          revision: p.active_binding?.revision || 0,
        }),
        onSaved: (r) => savedWorld(r, { selectedWorld: true }),
        fields: () => (
          <WorldForm>
            <p className="r-prose">
              确认将当前角色绑定到这个世界。浏览和编辑世界资料本身不会触发切换。
            </p>
          </WorldForm>
        ),
      },
      event,
    );
  }
  function removeWorld(event) {
    if (!world) return;
    const id = world.id;
    start(
      "remove:" + id,
      {
        title: "移除「" + world.name + "」？",
        description:
          "仅能移除没有绑定或引用的世界，后端会再次检查地点、人物和历史引用。",
        submitLabel: "确认移除",
        initial: { revision: world.revision, reason: "" },
        validate: (d) => {
          if (!d.reason.trim() || [...d.reason.trim()].length > 1000)
            throw Error("请填写移除说明，最多1000个字符。");
        },
        build: (d) => ({
          expectedRevision: d.revision,
          reason: d.reason.trim(),
        }),
        execute: async (body, key) => {
          assertWritable();
          const r = await w.client.deleteCatalogWorld(fid, id, {
            ...body,
            idempotencyKey: key,
          });
          if (r.id !== id || r.status !== "deleted")
            throw Error("移除回执未完整确认。");
          return r;
        },
        readLatest: async () => (await w.client.catalogWorld(fid, id)).world,
        adoptLatest: (d, row) => ({ ...d, revision: row.revision }),
        onSaved: (r) => savedWorld(r, { deleted: true }),
        fields: (d, edit) => (
          <WorldForm>
            <WorldField
              label="移除说明"
              multiline
              value={d.reason}
              onChange={(reason) => edit({ reason })}
            />
          </WorldForm>
        ),
      },
      event,
    );
  }
  async function expandWorld() {
    if (!world || busy) return;
    assertWritable();
    setError("");
    const record = runWorldExpansion({
      w,
      fid,
      owner,
      target: world,
      instruction,
      onSaved: async () => {
        const page = await readCatalog();
        if (live()) catalogRef.current.adopt(page);
      },
    });
    try {
      await record.completion;
    } catch (cause) {
      if (live()) setError(errorMessage(cause));
    }
  }
  return (
    <>
      <WorldPageView
        view={{
          w,
          c,
          fid,
          owner,
          catalog,
          world,
          snapshot,
          activeWorld,
          binding,
          selected,
          query,
          setQuery,
          search,
          setSearch,
          category,
          setCategory,
          tab,
          setTab,
          openWorld,
          back,
          editWorld,
          editPlace,
          addCategory,
          selectWorld,
          removeWorld,
          readOnly,
          notice,
          error,
          createRecovery,
          history,
          setHistory,
          instruction,
          setInstruction,
          expandWorld,
          busy,
          live,
          assertWritable,
          readCatalog,
          refresh: catalog.load,
        }}
      />
      {action && (
        <WorkspaceCommandDialog
          key={action.identity}
          action={action}
          onClose={dismissAction}
          returnFocus={() => trigger.current}
        />
      )}
    </>
  );
}

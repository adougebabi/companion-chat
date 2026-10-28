const clone = (value) => JSON.parse(JSON.stringify(value));
const text = (value) =>
  typeof value === "string"
    ? value
    : value == null
      ? ""
      : JSON.stringify(value, null, 2);
const limits = {
  name: 256,
  background: 16000,
  rules: 8000,
  flavor: 1000,
  description: 8000,
  alias: 256,
  group: 256,
  floor: 128,
  scene: 1000,
  atmosphere: 2000,
  layout: 8000,
};
export function worldFieldText(value) {
  return text(value);
}
export function validateWorldText(values, required = ["name"]) {
  for (const key of required)
    if (!String(values[key] || "").trim())
      throw Error(key === "name" ? "请填写名称。" : "请填写" + key + "。");
  for (const [key, limit] of Object.entries(limits))
    if (values[key] != null && [...String(values[key])].length > limit)
      throw Error("内容超过该字段允许的长度，请缩短后保存。");
}
export function worldDraft(row) {
  return {
    id: row?.id || null,
    revision: row?.revision ?? 0,
    name: row?.name || "",
    flavor: row?.flavor || "",
    background: row?.background || "",
    rules: row?.rules || "",
    description: row?.description || "",
    itemPool: text(row?.item_pool || []),
    source: row ? clone(row) : null,
  };
}
function collection(value) {
  if (typeof value !== "string") throw Error("请填写文本或JSON数组。");
  if (value.trim().startsWith("[")) {
    let parsed;
    try {
      parsed = JSON.parse(value);
    } catch {
      throw Error("JSON数组格式无效，请保留原字段或修正后保存。");
    }
    if (!Array.isArray(parsed) || parsed.length > 200)
      throw Error("列表最多200项。");
    if (new TextEncoder().encode(JSON.stringify(parsed)).length > 32000)
      throw Error("列表内容超过允许大小。");
    return parsed;
  }
  if ([...value].length > 16000) throw Error("列表文本最多16000个字符。");
  return value;
}
export function worldWrite(d) {
  validateWorldText(d);
  const body = {
    name: d.name.trim(),
    flavor: d.flavor,
    background: d.background,
    rules: d.rules,
    description: d.description,
    reason: "所有者保存世界资料",
    ...(d.id ? { expectedRevision: d.revision } : {}),
  };
  if (!d.source || d.itemPool !== text(d.source.item_pool || []))
    body.itemPool = collection(d.itemPool);
  return { id: d.id, body };
}
export function placeDraft(row, world, category) {
  return {
    id: row?.id || null,
    worldId: world.id,
    revision: row?.revision ?? 0,
    name: row?.name || "",
    description: row?.desc || "",
    alias: row?.alias || "",
    category:
      row?.venue_category_id ||
      category ||
      world.venue_categories?.[0]?.id ||
      "",
    floor: row?.floor || "",
    scene: row?.scene || "",
    atmosphere: row?.atmosphere || "",
    layout: row?.layout || "",
    furniture: text(row?.furniture ?? ""),
    connections: text(row?.connections ?? ""),
    roomItems: text(row?.room_items ?? ""),
    parentPlaceId: row?.parent_place_id || "",
    connectionPlaceIds: clone(row?.connection_place_ids || []),
    status: row?.status || "active",
    source: row ? clone(row) : null,
  };
}
export function placeWrite(d) {
  validateWorldText(d);
  if (!d.category && !d.id) throw Error("请先选择场地分类。");
  if (d.source?.name && d.name !== d.source.name)
    throw Error("正式名称固定，请修改别名。");
  if (d.source?.venue_category_id && d.category !== d.source.venue_category_id)
    throw Error("已有场地分类固定，请保留原分类。");
  if (d.parentPlaceId === d.id || d.connectionPlaceIds.includes(d.id))
    throw Error("地点不能连接到自身。");
  const body = {
    worldId: d.worldId,
    expectedRevision: d.revision,
    name: d.name.trim(),
    description: d.description,
    alias: d.alias,
    group: d.source?.group || "",
    floor: d.floor,
    scene: d.scene,
    atmosphere: d.atmosphere,
    layout: d.layout,
    status: d.status,
    reason: "所有者保存世界中的具体地点",
    ...(d.category ? { venueCategoryId: d.category } : {}),
  };
  for (const [input, key] of [
    ["furniture", "furniture"],
    ["connections", "connections"],
    ["roomItems", "room_items"],
  ]) {
    // Omit unchanged nested values so optional-field merging preserves unknown keys exactly.
    if (!d.source || d[input] !== text(d.source[key] ?? ""))
      body[input] = collection(d[input]);
  }
  if (!d.source || d.parentPlaceId !== (d.source.parent_place_id || ""))
    body.parentPlaceId = d.parentPlaceId || null;
  if (
    !d.source ||
    JSON.stringify(d.connectionPlaceIds) !==
      JSON.stringify(d.source.connection_place_ids || [])
  )
    body.connectionPlaceIds = clone(d.connectionPlaceIds);
  return { id: d.id, body };
}
const same = (a, b) => JSON.stringify(a) === JSON.stringify(b);
export function acknowledgeWorldDraft(draft, result, executed) {
  const body = executed.body;
  const source = {
    ...(draft.source || {}),
    id: result.id,
    revision: result.revision,
  };
  for (const key of ["name", "background", "rules", "flavor", "description"])
    if (Object.hasOwn(body, key)) source[key] = body[key];
  if (Object.hasOwn(body, "itemPool")) source.item_pool = clone(body.itemPool);
  return { ...draft, id: result.id, revision: result.revision, source };
}
export function rebaseWorldDraft(draft, latest) {
  if (!latest) return { ...draft, revision: 0, source: null };
  const before = worldDraft(draft.source),
    after = worldDraft(latest),
    next = {
      ...draft,
      id: latest.id,
      revision: latest.revision,
      source: clone(latest),
    };
  for (const key of [
    "name",
    "flavor",
    "background",
    "rules",
    "description",
    "itemPool",
  ])
    if (same(draft[key], before[key])) next[key] = after[key];
  return next;
}
export function acknowledgePlaceDraft(draft, result, executed) {
  const body = executed.body,
    source = {
      ...(draft.source || {}),
      id: result.id,
      world_id: body.worldId,
      name: body.name,
      desc: body.description || "",
      revision: result.revision,
    };
  for (const [input, key] of Object.entries({
    alias: "alias",
    group: "group",
    floor: "floor",
    scene: "scene",
    atmosphere: "atmosphere",
    layout: "layout",
    furniture: "furniture",
    connections: "connections",
    roomItems: "room_items",
    parentPlaceId: "parent_place_id",
    connectionPlaceIds: "connection_place_ids",
    venueCategoryId: "venue_category_id",
    status: "status",
  }))
    if (Object.hasOwn(body, input)) source[key] = clone(body[input]);
  const unappliedIdentity = { ...(draft.unappliedIdentity || {}) };
  if (!executed.id && draft.name !== body.name)
    unappliedIdentity.name = draft.name;
  if (!executed.id && draft.category !== (body.venueCategoryId || ""))
    unappliedIdentity.category = draft.category;
  return {
    ...draft,
    id: result.id,
    revision: result.revision,
    source,
    name: source.name,
    category: source.venue_category_id || draft.category,
    unappliedIdentity,
  };
}
export function rebasePlaceDraft(draft, latest) {
  if (!latest) return { ...draft, revision: 0, source: null };
  const world = { id: latest.world_id },
    before = placeDraft(draft.source, world),
    after = placeDraft(latest, world),
    next = {
      ...draft,
      id: latest.id,
      revision: latest.revision,
      source: clone(latest),
    };
  for (const key of [
    "description",
    "alias",
    "floor",
    "scene",
    "atmosphere",
    "layout",
    "furniture",
    "connections",
    "roomItems",
    "parentPlaceId",
    "connectionPlaceIds",
    "status",
  ])
    if (same(draft[key], before[key])) next[key] = after[key];
  next.name = latest.name;
  if (latest.venue_category_id) {
    if (
      draft.category !== before.category &&
      draft.category !== latest.venue_category_id
    )
      next.unappliedIdentity = {
        ...(draft.unappliedIdentity || {}),
        category: draft.category,
      };
    next.category = latest.venue_category_id;
  }
  return next;
}

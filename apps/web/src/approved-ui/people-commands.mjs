import { zonedInputInstant } from "../v3/zoned-input.mjs";
const clone = (value) => JSON.parse(JSON.stringify(value));
export function npcDraft(row) {
  return {
    name: row.name,
    role: row.role || "",
    background: row.bio || "",
    homePlaceId: row.home_place_id || "",
    routine: clone(row.routine || []),
    status: row.status,
    enabled: row.enabled !== false,
    positionMode: row.position_mode || "unknown",
    revision: row.revision,
    source: clone(row),
  };
}
export function npcWrite(d) {
  if (
    !d.name.trim() ||
    d.name.trim().length > 256 ||
    d.role.length > 512 ||
    d.background.length > 16000
  )
    throw Error("请填写人物名称，并核对姓名、身份与背景长度。");
  if (
    !["active", "archived"].includes(d.status) ||
    !["unknown", "routine", "fixed"].includes(d.positionMode) ||
    !Array.isArray(d.routine) ||
    d.routine.length > 42
  )
    throw Error("人物状态或生活规律无效。");
  return {
    name: d.name.trim(),
    role: d.role,
    background: d.background,
    homePlaceId: d.homePlaceId || null,
    routine: clone(d.routine),
    status: d.status,
    enabled: d.enabled,
    positionMode: d.positionMode,
    expectedRevision: d.revision,
    reason: "所有者编辑运行人物资料",
  };
}
export function acknowledgeNpcDraft(d, result, body) {
  return {
    ...d,
    revision: result.revision,
    source: {
      ...d.source,
      name: body.name,
      role: body.role,
      bio: body.background,
      home_place_id: body.homePlaceId,
      routine: clone(body.routine),
      status: body.status,
      enabled: body.enabled,
      position_mode: body.positionMode,
      revision: result.revision,
    },
  };
}
export function rebaseNpcDraft(d, row) {
  const before = npcDraft(d.source),
    after = npcDraft(row),
    next = { ...d, revision: row.revision, source: clone(row) };
  for (const key of [
    "name",
    "role",
    "background",
    "homePlaceId",
    "routine",
    "status",
    "enabled",
    "positionMode",
  ])
    if (JSON.stringify(d[key]) === JSON.stringify(before[key]))
      next[key] = after[key];
  return next;
}
export function encounterWrite(d) {
  if (!d.confirmed) throw Error("请确认这次接触在角色生活中实际发生。");
  if (!d.placeId || !d.summary.trim() || [...d.summary.trim()].length > 2000)
    throw Error("请选择实际地点并填写经过，最多2000个字符。");
  const instant = (value) => {
    if (d.exact) {
      if (
        !/^\d{4}-\d\d-\d\dT\d\d:\d\d(?::\d\d(?:\.\d+)?)?(?:Z|[+-]\d\d:\d\d)$/.test(
          value,
        ) ||
        !Number.isFinite(Date.parse(value))
      )
        throw Error("请填写包含时区偏移的有效时间。");
      return value;
    }
    return zonedInputInstant(value, d.zone);
  };
  const startAt = instant(d.startAt),
    endAt = instant(d.endAt),
    start = Date.parse(startAt),
    end = Date.parse(endAt);
  if (end <= start || end > Date.now() || end - start > 86400000)
    throw Error("结束时间须晚于开始、不得在未来，接触区间最多24小时。");
  const refs = d.evidence
    .split("\n")
    .map((value) => value.trim())
    .filter(Boolean);
  if (
    !refs.length ||
    refs.length > 32 ||
    refs.some((value) => new TextEncoder().encode(value).length > 256)
  )
    throw Error("请填写1–32条真实证据引用，每条最多256个UTF-8字节。");
  if (!d.lifeRevision) throw Error("生活上下文版本尚未读到。");
  return {
    confirmed: true,
    summary: d.summary,
    startAt,
    endAt,
    placeId: d.placeId,
    evidenceRefs: refs,
    expectedLifeContextRevision: d.lifeRevision,
    reason: "所有者确认记录角色生活中实际发生的接触",
  };
}

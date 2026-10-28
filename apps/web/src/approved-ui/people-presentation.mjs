import {
  actorUserBackgroundFields,
  backgroundDraft,
} from "../lib/actor-user-background.ts";
const clone = (value) => JSON.parse(JSON.stringify(value));
export const aboutFields = [
  ["full_name", "姓名"],
  ["name", "称呼"],
  ["age", "年龄"],
  ["gender", "性别"],
  ["occupation", "职业 / 身份"],
  ["relationship_description", "你填写的关系"],
  ["wants_you_to_know", "你想让她知道的事"],
  ["interests", "喜好"],
  ["preferences", "交流偏好"],
  ["habits", "习惯"],
  ["rules", "边界"],
];
export const extraAboutFields = actorUserBackgroundFields
  .filter((field) => !aboutFields.some(([key]) => key === field.key))
  .map(({ key, label }) => [key, label]);
export function relationshipFor(rows, target, profile, fid) {
  const scoped = (rows || []).filter(
    (row) =>
      row.target_actor_id === target &&
      (!row.owner_fluctlight_id || row.owner_fluctlight_id === fid),
  );
  return (
    (profile && scoped.find((row) => row.profile_id === profile)) ||
    scoped.find((row) => row.profile_id == null) ||
    null
  );
}
export function relationMetrics(row) {
  const labels = {
    familiarity: "熟悉程度",
    trust: "信任",
    comfort: "相处的舒适度",
    intimacy: "亲近程度",
    respect: "尊重",
  };
  return Object.entries(row?.metrics || {})
    .filter(
      ([key, value]) =>
        labels[key] &&
        typeof value === "number" &&
        Number.isFinite(value) &&
        value >= 0 &&
        value <= 1,
    )
    .map(([key, value]) => ({ key, label: labels[key], value }));
}
export function npcLocation(npc, places, at = Date.now()) {
  const name = (id) =>
    places.find((place) => place.id === id)?.alias ||
    places.find((place) => place.id === id)?.name ||
    "地点名称待读取";
  const p = npc.current_presence;
  if (
    p?.status === "active" &&
    p.definition_revision === npc.revision &&
    Date.parse(p.start_at) <= at &&
    at < Date.parse(p.end_at) &&
    !p.ended_at
  )
    return { label: "已读在场地点", name: name(p.place_id), source: p };
  if (npc.planned_position?.place_id)
    return {
      label: "预计地点",
      name: name(npc.planned_position.place_id),
      source: npc.planned_position,
    };
  if (npc.home_place_id)
    return { label: "默认地点", name: name(npc.home_place_id), source: null };
  return { label: "地点", name: "尚未设定", source: null };
}
function normalized(fields, key) {
  return key === "meeting_confirmed"
    ? fields[key] === "true"
      ? true
      : fields[key] === "false"
        ? false
        : null
    : String(fields[key] || "").trim() || null;
}
export function authoredDraft(value) {
  return {
    fields: backgroundDraft(value.background || {}),
    baseBackground: clone(value.background || {}),
    contextVersion: value.context_version,
    factsRevision: value.factsRevision,
    reason: "",
    operation: "change",
  };
}
export function backgroundPatch(draft) {
  const before = backgroundDraft(draft.baseBackground),
    patch = {};
  for (const { key } of actorUserBackgroundFields) {
    const value = normalized(draft.fields, key);
    if (value !== normalized(before, key)) patch[key] = value;
  }
  return patch;
}
export function validateAuthoredDraft(draft, owner) {
  if (!["correct", "change"].includes(draft.operation))
    throw Error("请选择本次修改含义。");
  const reason = draft.reason.trim();
  if (
    !reason ||
    reason.length > (owner ? 500 : 1000) ||
    (!owner && new TextEncoder().encode(reason).length > 1000)
  )
    throw Error(
      owner
        ? "请填写修改说明，最多500个字符。"
        : "请填写修改说明，最多1000个UTF-8字节。",
    );
  const patch = backgroundPatch(draft);
  if (!Object.keys(patch).length) throw Error("资料没有变化，尚未提交保存。");
  for (const value of Object.values(patch))
    if (typeof value === "string" && value.length > 1024)
      throw Error("每项资料最多1024个字符，输入已保留。");
  if (patch.timezone) {
    try {
      if (patch.timezone === "Local") throw Error();
      new Intl.DateTimeFormat("zh-CN", { timeZone: patch.timezone });
    } catch {
      throw Error("请填写有效的IANA时区，例如Asia/Shanghai。");
    }
  }
  if (owner ? !draft.factsRevision : !draft.contextVersion)
    throw Error("资料版本尚未读到，请重新读取。");
}
export function authoredWrite(draft) {
  return {
    input: {
      background: backgroundPatch(draft),
      operation: draft.operation,
      reason: draft.reason.trim(),
      expectedContextVersion: draft.contextVersion,
    },
    factsRevision: draft.factsRevision,
  };
}
export function acknowledgeAuthoredDraft(draft, result) {
  return {
    ...draft,
    baseBackground: clone(result.background),
    contextVersion: result.context_version || draft.contextVersion,
    factsRevision: result.current_facts_revision || draft.factsRevision,
  };
}
export function rebaseAuthoredDraft(draft, latest) {
  const before = backgroundDraft(draft.baseBackground),
    fresh = backgroundDraft(latest.background),
    fields = { ...draft.fields };
  for (const { key } of actorUserBackgroundFields)
    if (draft.fields[key] === before[key]) fields[key] = fresh[key];
  return {
    ...draft,
    fields,
    baseBackground: clone(latest.background),
    contextVersion: latest.context_version,
    factsRevision: latest.factsRevision,
  };
}
export function backgroundReadbackProven(receipt, latest, executed, owner) {
  const equalBackground = () =>
    Object.entries(receipt.background || {}).every(
      ([key, value]) => latest.background?.[key] === value,
    );
  if (owner) {
    if (receipt.current_facts_revision === latest.factsRevision)
      return equalBackground();
    const old = /^facts_gen_(\d+)$/.exec(receipt.current_facts_revision || ""),
      now = /^facts_gen_(\d+)$/.exec(latest.factsRevision || "");
    return !!(old && now && BigInt(now[1]) > BigInt(old[1]));
  } else if (receipt.context_version === latest.context_version)
    return equalBackground();
  const known = new Set(
    [...(latest.facts || []), ...(latest.history || [])].map(
      (row) => row.fact_id,
    ),
  );
  const written = Object.keys(executed.input.background).map((attribute) =>
    (receipt.facts || []).find((row) => row.attribute === attribute),
  );
  return (
    written.length > 0 &&
    written.every((row) => row?.fact_id && known.has(row.fact_id))
  );
}

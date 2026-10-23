const clone = (value) =>
  value === undefined ? undefined : JSON.parse(JSON.stringify(value));
export const identityFields = {
  name: "名字",
  nickname: "昵称",
  gender: "性别",
  age: "年龄",
  occupation: "职业",
  height: "身高说明",
  height_cm: "身高（厘米）",
  blood_type: "血型",
  birthplace: "出生地",
  residence: "居住地",
  timezone: "生活时区",
  birthday: "生日",
  background_story: "简介与经历",
  background: "背景",
  biography: "生平",
  core_values: "核心价值观",
  worldview: "世界观",
  notes: "备注",
};
export const behavioralFields = {
  response_style: "回复风格",
  message_length: "消息长短",
  emoji_frequency: "表情频率",
  punctuation_style: "标点习惯",
  humor_style: "幽默方式",
  sarcasm_tendency: "调侃倾向",
  directness: "直接程度",
  initiative: "主动性",
  topic_initiation: "开启话题",
  silence_tolerance: "对沉默的接受程度",
  response_delay: "回应节奏",
  emotional_expression: "情绪表达",
  conflict_style: "分歧处理",
  refusal_style: "拒绝方式",
  intimacy_expression: "亲密表达",
};
const policyNumbers = new Set([
  "emoji_frequency",
  "sarcasm_tendency",
  "directness",
  "initiative",
  "topic_initiation",
  "silence_tolerance",
  "response_delay",
  "emotional_expression",
]);
const technical = (key) =>
  /^(?:id|schema_version|provenance|extensions|evidence_refs|update_policy|active_profile_id)$/.test(
    key,
  ) || /_(?:id|ids|version|revision|at)$/.test(key);
const readOnlyLife = new Set(["wardrobe_items", "relationship_seeds"]);
const equal = (a, b) => JSON.stringify(a) === JSON.stringify(b);
export function coreFromDetail(detail) {
  return {
    identity: clone(detail.identity || detail.core_persona?.identity || {}),
    behavioral_policy: clone(
      detail.behavioral_policy || detail.core_persona?.behavioral_policy || {},
    ),
    life_profile: clone(
      detail.life_profile || detail.core_persona?.life_profile || {},
    ),
  };
}
/** Editable leaves come from the actual record, with canonical empty identity/policy fields. */
export function profileFields(core, label = (key) => key) {
  const fields = [];
  function walk(value, path) {
    const key = String(path.at(-1));
    if (technical(key) || (path[0] === "life_profile" && readOnlyLife.has(key)))
      return;
    if (value && typeof value === "object" && !Array.isArray(value)) {
      for (const [key, child] of Object.entries(value))
        walk(child, [...path, key]);
      return;
    }
    if (Array.isArray(value) && value.some((v) => v && typeof v === "object")) {
      value.forEach((child, index) => walk(child, [...path, index]));
      return;
    }
    if (
      Array.isArray(value) &&
      !value.length &&
      ["recurring_commitments", "injuries"].includes(key)
    )
      return;
    const numeric =
        typeof value === "number" ||
        (path[0] === "behavioral_policy" && policyNumbers.has(key)),
      kind =
        Array.isArray(value) ||
        (path[0] === "identity" && key === "core_values")
          ? "lines"
          : typeof value === "boolean"
            ? "boolean"
            : numeric
              ? "number"
              : "text";
    const display =
      kind === "lines"
        ? Array.isArray(value)
          ? value.join("\n")
          : (value ?? "")
        : (value ?? "");
    fields.push({
      path,
      label: path
        .slice(1)
        .map((p) =>
          typeof p === "number"
            ? `第 ${p + 1} 项`
            : identityFields[p] || behavioralFields[p] || (label(p)==="自定义字段"?"补充说明":label(p)),
        )
        .join(" · "),
      kind,
      itemType:
        Array.isArray(value) &&
        value.length &&
        value.every((v) => typeof v === "number")
          ? "number"
          : Array.isArray(value) &&
              value.length &&
              value.every((v) => typeof v === "boolean")
            ? "boolean"
            : "string",
      min: numeric ? 0 : undefined,
      max:
        path[0] === "behavioral_policy" &&
        policyNumbers.has(key) &&
        key !== "response_delay"
          ? 1
          : undefined,
      value: display,
      initial: display,
    });
  }
  const identity = {
    ...Object.fromEntries(
      Object.keys(identityFields)
        .filter((k) => k !== "background")
        .map((k) => [k, undefined]),
    ),
    ...core.identity,
  };
  for (const [key, value] of Object.entries(identity))
    walk(value, ["identity", key]);
  const policy = {
    ...Object.fromEntries(
      Object.keys(behavioralFields).map((k) => [k, undefined]),
    ),
    ...core.behavioral_policy,
  };
  for (const [key, value] of Object.entries(policy))
    walk(value, ["behavioral_policy", key]);
  for (const [key, value] of Object.entries(core.life_profile || {}))
    walk(value, ["life_profile", key]);
  return fields;
}
function set(root, path, value) {
  let target = root;
  for (const [i, key] of path.slice(0, -1).entries()) {
    target[key] ??= typeof path[i + 1] === "number" ? [] : {};
    target = target[key];
  }
  target[path.at(-1)] = value;
}
function difference(before, after) {
  if (equal(before, after)) return undefined;
  if (after && typeof after === "object" && !Array.isArray(after)) {
    const patch = {};
    for (const [key, value] of Object.entries(after)) {
      const change = difference(before?.[key], value);
      if (change !== undefined) patch[key] = change;
    }
    return Object.keys(patch).length ? patch : undefined;
  }
  return clone(after);
}
export function profileChanges(base, fields) {
  const edited = clone(base);
  for (const field of fields) {
    if (equal(field.value, field.initial)) continue;
    let value = field.value;
    if (field.kind === "number") {
      value = value === "" ? null : Number(value);
      if (
        value !== null &&
        (!Number.isFinite(value) ||
          value < 0 ||
          (field.max != null && value > field.max))
      )
        throw Error(field.label + "：请填写有效的非负数值");
    } else if (field.kind === "lines") {
      value = String(value)
        .split("\n")
        .map((v) => v.trim())
        .filter(Boolean);
      if (field.itemType === "number") {
        value = value.map(Number);
        if (value.some((v) => !Number.isFinite(v)))
          throw Error(field.label + "：每行应为有效数值");
      }
      if (field.itemType === "boolean") {
        if (value.some((v) => !["true", "false", "是", "否"].includes(v)))
          throw Error(field.label + "：每行请填写是或否");
        value = value.map((v) => v === "true" || v === "是");
      }
    } else if (field.kind === "boolean")
      value = value === true || value === "true";
    else value = String(value);
    if (field.path.join(".") === "identity.timezone" && value) {
      try {
        new Intl.DateTimeFormat("zh-CN", { timeZone: value });
      } catch {
        throw Error("请填写有效的 IANA 生活时区，例如 Asia/Shanghai");
      }
    }
    set(edited, field.path, value);
  }
  return difference(base, edited) || {};
}
export function applyProfileChanges(base, patch) {
  const next = clone(base);
  for (const [key, value] of Object.entries(patch)) {
    if (value && typeof value === "object" && !Array.isArray(value))
      next[key] = applyProfileChanges(next[key] || {}, value);
    else next[key] = clone(value);
  }
  return next;
}

export function rebaseProfileFields(base, current, label, submitted) {
  const comparison = submitted && new Map(submitted.map((f) => [f.path.join("."), f.value]));
  const edits = new Map(current.filter((f) => !equal(f.value, comparison ? comparison.get(f.path.join(".")) : f.initial))
    .map((f) => [f.path.join("."), f.value]));
  const next = profileFields(base, label);
  for (const field of next) {
    if (edits.has(field.path.join("."))) field.value = edits.get(field.path.join("."));
  }
  return next;
}

export function profileSaveCommand({ client, fid, live }) {
  return async (value, key) => {
    if (!live()) throw Error("账号或角色已变化，请回到原资料核对");
    let proposal;
    try {
      proposal = await client.submitFoundationRevision(fid, {
        ...value,
        idempotencyKey: key,
      });
      if (!live()) throw Error("账号或角色已变化，原提议已保留");
      if (proposal.fluctlight_id !== fid || !proposal.id)
        throw Error("修订提议的角色范围不匹配");
      return await client.acceptFoundationRevision(fid, proposal.id, {
        expectedRevision: value.expectedRevision,
        reason: value.reason,
      });
    } catch (error) {
      if (
        proposal &&
        !["foundation_proposal_stale", "revision_conflict"].includes(error.code)
      ) {
        const retained = new Error(error.message, { cause: error });
        retained.code = error.code;
        throw retained;
      }
      throw error;
    }
  };
}

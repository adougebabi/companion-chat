const responseStyles = { warm: "温暖", concise: "简洁", warm_concise: "温暖简洁" };
const conflictStyles = { direct: "直接", gentle: "温和" };
const initiativeModes = { active: "主动" };
const communicationTones = { warm: responseStyles.warm };

// Core also stores descriptive text in these fields. Keep the code labels
// scoped to the field so e.g. active never inherits the lifecycle status label.
const enums = {
  response_style: { label: "回复风格", values: responseStyles },
  reply_style: { label: "回复风格", values: responseStyles },
  message_length: { label: "消息长度", editorLabel: "消息长短", values: { short: "简短", medium: "适中", long: "详细" } },
  punctuation_style: { label: "标点风格", editorLabel: "标点习惯", values: { natural: "自然" } },
  humor_style: { label: "幽默风格", editorLabel: "幽默方式", values: { gentle: "温和" } },
  conflict_style: { label: "冲突风格", editorLabel: "分歧处理", values: conflictStyles },
  refusal_style: { label: "拒绝风格", editorLabel: "拒绝方式", values: { clear: "清晰", active: "主动拒绝" } },
  intimacy_expression: { label: "亲密表达", values: { reserved: "含蓄" } },
  tone: { label: "语气", values: communicationTones },
  initiative: { label: "主动性", values: initiativeModes },
  support_style: { label: "支持方式", values: {} },
  attachment_style: { label: "依恋方式", values: { secure: "安全型" }, fallback: "自定义依恋方式" },
  "personality_system.mode": { label: "人格模式", values: { single: "单一人格", multiple: "多重人格" }, fallback: "自定义人格模式" },
  "communication.tone": { label: "交流语气", values: communicationTones },
  "response.style": { label: "回应方式", values: responseStyles },
  "initiative.mode": { label: "主动沟通方式", values: initiativeModes },
  "conflict.approach": { label: "冲突处理方式", values: conflictStyles },
  "support.style": { label: "支持方式", values: {} },
};

export const profileEnumOptions = field => Object.entries(Object.hasOwn(enums, field) ? enums[field].values : {});
export const profileEnumFields = Object.keys(enums);

const scales = {
  emoji_frequency: { label: "表情频率" },
  sarcasm_tendency: { label: "讽刺倾向", editorLabel: "调侃倾向" },
  directness: { label: "直接性", editorLabel: "直接程度" },
  topic_initiation: { label: "发起话题", editorLabel: "开启话题" },
  silence_tolerance: { label: "沉默容忍度", editorLabel: "对沉默的接受程度" },
  response_delay: { label: "回复延迟", editorLabel: "回应节奏" },
  emotional_expression: { label: "情绪表达" },
};

export function profileFieldLabel(field, editor = false) {
  const entry = Object.hasOwn(enums, field) ? enums[field] : Object.hasOwn(scales, field) ? scales[field] : undefined;
  return (editor && entry?.editorLabel) || entry?.label || "";
}

export function profileFieldPath(field, parent = "") {
  const nested = parent + "." + field;
  return Object.hasOwn(enums, nested) ? nested : field;
}

export function profileEnumLabel(value, field) {
  if (!Object.hasOwn(enums, field)) return value;
  const { values, fallback = "其他方式" } = enums[field];
  const code = value.trim().toLowerCase();
  if (Object.hasOwn(values, code)) return values[code];
  return /^(?=[a-z\d_.-]*[a-z])[a-z\d_.-]+$/.test(code) ? fallback : value;
}

const reflectionSubjects = {
  "baseline.pleasure": "愉悦程度", "baseline.arousal": "唤醒程度", "baseline.dominance": "掌控感",
  "traits.openness": "开放性", "traits.conscientiousness": "尽责性", "traits.extraversion": "外向性",
  "traits.agreeableness": "亲和性", "traits.emotional_stability": "情绪稳定性",
  "expression.warmth": "表达温暖度", "expression.initiative": "表达主动性", "expression.playfulness": "表达活泼度",
};

export function reflectionSubject(field) {
  const path = typeof field === "string" ? field.replace(/^(?:behavioral_policy|personality)\./, "") : "";
  const label = Object.hasOwn(reflectionSubjects, path) ? reflectionSubjects[path] : profileFieldLabel(path);
  return { path, label };
}

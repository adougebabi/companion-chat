import { behavioralFields, identityFields } from "@/approved-ui/profile-fields.mjs";
import { profileEnumFields, profileEnumLabel } from "@/approved-ui/behavior-enum-labels.mjs";
import { portraitPresentation, profileExpressionTextFields, workingPersonaPresentation } from "@/approved-ui/profile-presentation.mjs";
import { stateLabel } from "@/v3/domain-presentation.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";
import { narrativeFields, readingExtensions, readingFields, readingText, selectReadingFields } from "@/approved-ui/reading-fields.mjs";
import { readingFieldLabel } from "@/v3/proto/components/reading-fields.js";
import { creationScaleHints } from "./CreationScaleHint";
import JsonDetails from "./JsonDetails";
import SemanticTree, { hasVisibleSemanticValue } from "./SemanticTree";
import { currentActivityDisplay } from "./schedule-presentation.mjs";

const internalToken = /\b(?:res|world_item|world|place|req|fluctlight|actor|profile|task|audit)_[a-z\d_-]+\b/gi;
const isoInstant = /\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(?::\d{2}(?:\.\d+)?)?(?:Z|[+-]\d{2}:?\d{2})/gi;

export function profileText(value) {
  if (typeof value !== "string") return "";
  return value.replace(isoInstant, (time) => Number.isFinite(Date.parse(time)) ? userInstant(time) : "")
    .replace(internalToken, "")
    .replace(/(?:表达画像|表达版本)[\s·]*v\d+(?:\.\d+)*[\s·]*(?:当前|兼容)?协议/gi, "当前表达画像")
    .trim();
}

const scalars = (keys) => Object.fromEntries(keys.map((key) => [key, true]));
const traits = scalars([
  "openness", "conscientiousness", "extraversion", "agreeableness", "neuroticism",
  "curiosity", "independence", "patience", "empathy", "assertiveness", "humor",
  "sociability", "risk_tolerance",
]);
const expression = {
  ...scalars([...Object.keys(behavioralFields), ...profileEnumFields, ...profileExpressionTextFields]),
  communication: { tone: true },
  response: { style: true },
  initiative: { mode: true },
  conflict: { approach: true },
  support: { style: true },
};
const narrative = Object.fromEntries([
  "title", "name", "label", "text", "summary", "description", "content", "value",
  "reason", "likes", "dislikes", "stable_preferences", "habits", "boundaries",
].map(key => [key, narrativeFields]));
const location = { ...narrative, address: true, city: true, region: true, country: true };
const appearance = {
  ...narrative,
  ...readingFields("hair hair_color hair_style hair_length eyes eye_color body_type skin_tone build injuries daily_outfit_preferences style_preferences physical_features"),
  wardrobe_items: readingFields("name description category slot clothing_type ownership available currently_worn seasons color material"),
};
const identityShape = {
  ...scalars(Object.keys(identityFields)), ...readingFields("tags tagline education nationality languages hobbies"),
  residence: location, birthplace: location,
};
export const personalityShape = {
  ...traits, ...expression, ...narrative, traits, expression,
  ...readingFields("core_mechanisms language_expression behavior_boundaries emotional_state stable_motivations temperament sensitivities rituals fears desires"),
};
export const behaviorShape = {
  ...expression, ...narrative, expression,
  ...readingFields("voice body_language speech_patterns behavior_boundaries scenario_behavior output_preferences intimacy_progression behavior_state_machine behavior_loops"),
};
export const lifeShape = {
  ...narrative, ...readingFields("preferences life_habits character_constraints recurring_commitments media_preferences social_background"),
  relationship_seeds: {
    ...readingFields("role summary trend emotional_association"),
    metrics: narrativeFields,
  },
  appearance,
};
export const systemShape = {
  mode: true,
  ...readingFields("core_relationship core_conflict integration shared_memory_policy switching forced_activation influence conflict_resolution behavior_state_machine"),
  takeover_rules: readingFields("condition description priority cooldown_seconds enabled"),
};
const personaShape = {
  description: true, identity: identityShape, personality: personalityShape,
  behavioral_policy: behaviorShape, life_profile: lifeShape,
  ...readingFields("voice body_language desires fears secrets behavior_loops behavior_state_machine scenario_behavior intimacy_progression output_preferences emotional_state stable_motivations"),
};
export const profileShape = { ...personaShape, personality_system: { ...systemShape, profiles: { name: true, ...personaShape } } };
const contextShape = {
  location, ...readingFields("scene activity doing"),
};
export const currentStateShape = {
  appearance, ...readingFields("mood momentum energy"),
  ...contextShape, context: contextShape, scene_image: readingFields("label description"),
};

export function profileFieldsForReading(value, fields, showClears = false) {
  return selectReadingFields(value, fields, {
    showClears,
    text: text => profileText(readingText(text)),
  });
}

/** @param {{value: any, shape?: object | true | typeof narrativeFields, identity?: object, title?: string, showClears?: boolean, present?: (value: any) => any, field?: string}} props */
export function ProfileFieldsSummary({ value, shape = profileShape, identity, title, showClears, present, field = "" }) {
  const selected = profileFieldsForReading(value, shape, showClears);
  const summary = present ? present(selected) : selected;
  if (!hasVisibleSemanticValue(summary, "", identity)) return null;
  return <>
    {title && <h4 className="font-medium">{title}</h4>}
    <SemanticTree value={summary} field={field} identity={identity} reading />
  </>;
}

export function ProfileDiagnostic({ value, title }) {
  if (value == null || value === "" || (typeof value === "object" && !Object.keys(value).length)) return null;
  return <JsonDetails value={value} title={title} downloadOnly />;
}

function ExtensionValue({ value }) {
  if (Array.isArray(value)) {
    return <ul className="semantic-list">{value.map((item, index) => (
      <li key={index}><ExtensionValue value={item} /></li>
    ))}</ul>;
  }
  if (value && typeof value === "object") {
    return <dl className="approved-facts">{Object.entries(value).map(([field, item]) => {
      const translated = readingFieldLabel(field);
      const label = translated === "补充说明" ? field : translated;
      return <div key={field}>
        <dt>{/\p{Script=Han}/u.test(label) && !/[a-z\d]+_[a-z\d]+/i.test(label) ? label : "补充说明"}</dt>
        <dd><ExtensionValue value={item} /></dd>
      </div>;
    })}</dl>;
  }
  return <span>{typeof value === "boolean" ? (value ? "是" : "否") : value}</span>;
}

/** @param {{value: unknown}} props */
export function ProfileExtensionsSummary({ value }) {
  return [...readingExtensions(value)].map((item, index) => (
    <dl className="approved-facts" key={index}>
      <div><dt>补充说明</dt><dd><ExtensionValue value={item} /></dd></div>
    </dl>
  ));
}

export function ProfileTime({ value }) {
  return typeof value === "string" && Number.isFinite(Date.parse(value))
    ? <time>{userInstant(value)}</time>
    : null;
}

/** @param {{value: unknown, label?: string}} props */
export function ProfileNote({ value, label }) {
  const text = profileText(value);
  return text ? <p>{label && <b>{label}：</b>}{text}</p> : null;
}

const degreeLabels = ["很低", "较低", "适中", "较高", "很高"];
const expressionDegrees = {
  emoji_frequency: [creationScaleHints.emoji_frequency[0], "较少使用表情", "适中", "较常使用表情", "经常使用表情"],
  sarcasm_tendency: [creationScaleHints.sarcasm_tendency[0], "较少调侃", "适中", "较常调侃", "经常调侃"],
  directness: ["很委婉", creationScaleHints.directness[0], "适中", creationScaleHints.directness[1], "很直接"],
  initiative: degreeLabels,
  topic_initiation: ["很少开启话题", creationScaleHints.topic_initiation[0], "适中", creationScaleHints.topic_initiation[1], "经常开启话题"],
  silence_tolerance: degreeLabels,
  emotional_expression: ["很克制", creationScaleHints.emotional_expression[0], "适中", creationScaleHints.emotional_expression[1], "情绪表达很明显"],
};

function scaleNumber(value) {
  return typeof value === "string" && /^\d+(?:\.\d+)?$/.test(value) ? Number(value) : value;
}

function expressionValue(field, value) {
  const score = scaleNumber(value);
  if (!Number.isFinite(score) || score < 0) return value;
  if (field === "response_delay") {
    if (score === 0) return "即时回复";
    if (score < 1) return "不到 1 秒";
    const seconds = Math.round(score);
    if (seconds < 60) return `约 ${seconds} 秒`;
    const remainder = seconds % 60;
    return `约 ${Math.floor(seconds / 60)} 分钟${remainder ? ` ${remainder} 秒` : ""}`;
  }
  if (!Object.hasOwn(expressionDegrees, field) || score > 1) return value;
  const index = [0.2, 0.4, 0.6, 0.8].filter(bound => score >= bound).length;
  return expressionDegrees[field][index];
}

export function describeProfileExpression(value, field = "") {
  if (Array.isArray(value)) return value.map(item => describeProfileExpression(item, field));
  if (value && typeof value === "object") {
    return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, describeProfileExpression(item, key)]));
  }
  return expressionValue(field, value);
}

export function describeProfileCurrentState(state) {
  if (!state || typeof state !== "object" || Array.isArray(state)) return state;
  return {
    ...state,
    activity: currentActivityDisplay(state.activity),
    ...(state.context && typeof state.context === "object" && !Array.isArray(state.context)
      ? { context: { ...state.context, activity: currentActivityDisplay(state.context.activity) } }
      : {}),
  };
}

/** @param {{ value: any, identity?: object, activeProfileId?: string | null, localizePolicy?: boolean }} props */
export function WorkingPortraitSummary({ value, identity, activeProfileId, localizePolicy = false }) {
  const working = workingPersonaPresentation(value);
  if (activeProfileId !== undefined && typeof working?.profile_id === "string" && working.profile_id !== activeProfileId) {
    return <p>当前人格的表达画像尚未读回，可下载留存记录核对。</p>;
  }
  const text = working?.portrait_text ?? working?.working_persona?.portrait_text;
  const rows = typeof text === "string" ? portraitPresentation(text, { includeOccurrences: true }) : [];
  const reading = rows.flatMap((row) => {
    const source = row.text?.match(/^用户原文(?:\s*[:：]\s*|\s+)([a-z][a-z\d_]*(?:\.[a-z][a-z\d_]*)*)\s*[:：]\s*(.*)$/);
    if (source) {
      const [, field, raw] = source;
      const value = profileFieldsForReading({ [field]: raw }, narrativeFields)?.[field];
      return value === undefined ? [] : [{ field, value, userSource: true }];
    }
    if (row.field) {
      const value = profileFieldsForReading(row.value, true);
      return value === undefined ? [] : [{ ...row, value }];
    }
    if (/^\s*[a-z][a-z\d_.]*\s*[:：]/i.test(row.text)) return [];
    const text = profileText(row.text);
    return text ? [{ text }] : [];
  });
  if (!reading.length) {
    return <ProfileFieldsSummary value={working?.working_persona ?? working} shape={personalityShape} identity={identity}
      present={localizePolicy ? describeProfileExpression : undefined} />;
  }
  return <div data-testid="current-working-portrait">
    {reading.map((line, index) => {
      if (!line.field) return <p key={index}>{line.text}</p>;
      const policy = localizePolicy && Object.hasOwn(behavioralFields, line.field);
      const value = policy ? expressionValue(line.field, line.value) : line.value;
      return <p key={index}>
        {line.userSource && <span>用户原文 · </span>}
        <b>{readingFieldLabel(line.field)}：</b>
        {policy || line.userSource || profileExpressionTextFields.includes(line.field)
          ? typeof value === "string" ? profileEnumLabel(value, line.field) : value
          : <SemanticTree value={value} field={line.field} identity={identity} reading />}
        {line.occurrences > 1 && <small className="r-muted">（原文出现 {line.occurrences} 次）</small>}
      </p>;
    })}
  </div>;
}

export function ClaimReadingSummary({ claim, category }) {
  const source = {
    reflection: "反思", conversation: "真实交流", message: "真实交流",
    diary: "日记", memory: "记忆", event: "生活经历", owner: "所有者记录",
  }[claim.provenance?.kind];
  return <>
    <ProfileNote value={category} label="认识类别" />
    {Number.isFinite(claim.confidence) && <p>置信度 {claim.confidence}</p>}
    <ProfileNote value={claim.evidence_summary || claim.provenance?.summary || source} label="形成依据" />
    {claim.owner_review && <ProfileNote value={claim.owner_review.reason} label="所有者复核意见" />}
    <ClaimValueSummary value={claim.value} />
    <ProfileTime value={claim.created_at} />
  </>;
}

/** @param {{ value: any, title?: string }} props */
export function ClaimValueSummary({ value, title = "认识内容" }) {
  return <ProfileFieldsSummary value={value} shape={narrativeFields} title={title} />;
}

const capabilityShape = {
  ...narrative,
  ...readingFields("input inputs desired_input output outputs desired_output parameters limits limitations constraints restrictions preconditions prerequisites examples example error_handling errors failure_modes use_cases requirements assumptions output_format success_criteria"),
};

export function CapabilitySummary({ record }) {
  const sideEffect = { read: "只读", write: "会修改资料", external: "可能影响外部服务", none: "无副作用" }[record.side_effect_class];
  const priority = { low: "低", normal: "普通", medium: "中", high: "高", urgent: "紧急" }[record.priority];
  return <>
    <ProfileNote value={record.description} />
    <ProfileNote value={record.rationale || record.reason} label="申请原因" />
    <ProfileFieldsSummary value={record.desired_contract} shape={capabilityShape} title="期望能力" />
    <ProfileNote value={sideEffect} label="影响范围" />
    <ProfileNote value={priority} label="优先级" />
    <ProfileNote value={record.review_note} label="审核意见" />
    {typeof record.registered_capability?.available === "boolean" &&
      <p>{record.registered_capability.available ? "已有可交付的注册实现" : "当前未读到可交付的注册实现"}</p>}
  </>;
}

export function GovernanceHistorySummary({ entry, showExtensions = false }) {
  const action = { accept: "已接受", reject: "已拒绝", restore: "已恢复", propose: "已提出" }[entry.action];
  const status = entry.status || entry.action;
  return <div className="grid gap-2">
    {status && <p>{action || stateLabel(status, "已记录")}</p>}
    <ProfileTime value={entry.occurred_at || entry.created_at} />
    <ProfileNote value={entry.reason || entry.note} />
    {showExtensions && <ProfileExtensionsSummary value={entry} />}
    <ProfileDiagnostic value={entry} title="原始治理历史记录（诊断）" />
  </div>;
}

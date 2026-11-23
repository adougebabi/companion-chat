import { behavioralFields, identityFields } from "@/approved-ui/profile-fields.mjs";
import { portraitPresentation, workingPersonaPresentation } from "@/approved-ui/profile-presentation.mjs";
import { stateLabel } from "@/v3/domain-presentation.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";
import { semanticLabel } from "@/v3/proto/components/semantic-fields.js";
import JsonDetails from "./JsonDetails";
import SemanticTree, { hasVisibleSemanticValue } from "./SemanticTree";

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
const expression = scalars([...Object.keys(behavioralFields), "reply_style", "tone", "speech_style", "speech_patterns", "body_language"]);
const narrative = scalars([
  "title", "name", "label", "text", "summary", "description", "content", "value",
  "reason", "likes", "dislikes", "stable_preferences", "habits", "boundaries",
]);
const location = { ...narrative, address: true, city: true, region: true, country: true };
const appearance = {
  ...narrative,
  ...scalars(["hair_color", "hair_style", "hair_length", "injuries", "daily_outfit_preferences", "style_preferences"]),
  physical_features: scalars(["hair_color", "hair_style", "hair_length", "eye_color", "height", "height_cm", "chest_cup", "description"]),
};
const identityShape = {
  ...scalars(Object.keys(identityFields)), tags: true, tagline: true,
  residence: location, birthplace: location,
};
export const personalityShape = {
  ...traits, ...expression, ...narrative, traits, expression,
  core_mechanisms: narrative, language_expression: { ...expression, ...narrative },
  behavior_boundaries: narrative,
};
export const behaviorShape = {
  ...expression, ...narrative, expression, voice: { ...expression, ...narrative },
  behavior_boundaries: narrative, scenario_behavior: narrative,
  output_preferences: narrative, intimacy_progression: narrative,
};
export const lifeShape = {
  ...narrative, preferences: { ...narrative, interests: true, clothing_style: true }, life_habits: narrative,
  character_constraints: narrative, recurring_commitments: { ...narrative, frequency: true }, appearance,
};
export const systemShape = {
  mode: true, core_relationship: narrative, core_conflict: narrative,
  integration: narrative, shared_memory_policy: narrative,
};
export const profileShape = {
  description: true, identity: identityShape, personality: personalityShape,
  behavioral_policy: behaviorShape, life_profile: lifeShape, personality_system: systemShape,
  voice: { ...expression, ...narrative }, desires: true, fears: true, secrets: true,
  behavior_loops: narrative,
};
const contextShape = {
  location, scene: narrative, activity: narrative, doing: narrative,
};
export const currentStateShape = {
  appearance, mood: { ...narrative, intensity: true }, momentum: narrative,
  ...contextShape, context: contextShape, energy: narrative, scene_image: { label: true },
};

export function profileFieldsForReading(value, fields, showClears = false) {
  if (Array.isArray(value)) {
    const items = value.map((item) => profileFieldsForReading(item, fields))
      .filter((item) => item !== undefined);
    return items.length ? items : undefined;
  }
  if (value === "" && showClears) return "清空该设定";
  if (typeof value === "string") return profileText(value) || undefined;
  if (typeof value === "boolean" || Number.isFinite(value)) return value;
  if (!value || typeof value !== "object") return undefined;
  if (fields === true) return profileFieldsForReading(value.value, true);

  const entries = Object.entries(fields).flatMap(([key, shape]) => {
    const item = profileFieldsForReading(value[key], shape, showClears);
    return item === undefined ? [] : [[key, item]];
  });
  return entries.length ? Object.fromEntries(entries) : undefined;
}

/** @param {{value: any, shape?: object | true, identity?: object, title?: string, showClears?: boolean}} props */
export function ProfileFieldsSummary({ value, shape = profileShape, identity, title, showClears }) {
  const summary = profileFieldsForReading(value, shape, showClears);
  if (!hasVisibleSemanticValue(summary, "", identity)) return null;
  return <>
    {title && <h4 className="font-medium">{title}</h4>}
    <SemanticTree value={summary} identity={identity} />
  </>;
}

export function ProfileDiagnostic({ value, title }) {
  if (value == null || value === "" || (typeof value === "object" && !Object.keys(value).length)) return null;
  return <JsonDetails value={value} title={title} downloadOnly />;
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

/** @param {{ value: any, identity?: object, activeProfileId?: string | null }} props */
export function WorkingPortraitSummary({ value, identity, activeProfileId }) {
  const working = workingPersonaPresentation(value);
  if (activeProfileId !== undefined && typeof working?.profile_id === "string" && working.profile_id !== activeProfileId) {
    return <p>当前人格的表达画像尚未读回，可下载留存记录核对。</p>;
  }
  const text = working?.portrait_text ?? working?.working_persona?.portrait_text;
  const rows = typeof text === "string" ? portraitPresentation(text, { includeOccurrences: true }) : [];
  const reading = rows.flatMap((line) => {
    if (line.field) {
      const value = profileFieldsForReading(line.value, true);
      return value === undefined ? [] : [{ ...line, value }];
    }
    if (/^\s*[a-z][a-z\d_]*\s*[:：]/i.test(line.text)) return [];
    const text = profileText(line.text);
    return text ? [{ text }] : [];
  });
  if (!reading.length) {
    return <ProfileFieldsSummary value={working?.working_persona ?? working} shape={personalityShape} identity={identity} />;
  }
  return <div data-testid="current-working-portrait">
    {reading.map((line, index) => line.field ? (
      <p key={index}>
        <b>{semanticLabel(line.field)}：</b>
        <SemanticTree value={line.value} field={line.field} identity={identity} />
        {line.occurrences > 1 && <small className="r-muted">（原文出现 {line.occurrences} 次）</small>}
      </p>
    ) : <p key={index}>{line.text}</p>)}
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
  return <ProfileFieldsSummary value={value} shape={narrative} title={title} />;
}

export function CapabilitySummary({ record }) {
  const sideEffect = { read: "只读", write: "会修改资料", external: "可能影响外部服务", none: "无副作用" }[record.side_effect_class];
  const priority = { low: "低", normal: "普通", medium: "中", high: "高", urgent: "紧急" }[record.priority];
  return <>
    <ProfileNote value={record.description} />
    <ProfileNote value={record.rationale || record.reason} label="申请原因" />
    <ProfileFieldsSummary value={record.desired_contract} shape={narrative} title="期望能力" />
    <ProfileNote value={sideEffect} label="影响范围" />
    <ProfileNote value={priority} label="优先级" />
    <ProfileNote value={record.review_note} label="审核意见" />
    {typeof record.registered_capability?.available === "boolean" &&
      <p>{record.registered_capability.available ? "已有可交付的注册实现" : "当前未读到可交付的注册实现"}</p>}
  </>;
}

export function GovernanceHistorySummary({ entry }) {
  const action = { accept: "已接受", reject: "已拒绝", restore: "已恢复", propose: "已提出" }[entry.action];
  const status = entry.status || entry.action;
  return <div className="grid gap-2">
    {status && <p>{action || stateLabel(status, "已记录")}</p>}
    <ProfileTime value={entry.occurred_at || entry.created_at} />
    <ProfileNote value={entry.reason || entry.note} />
    <ProfileDiagnostic value={entry} title="原始治理历史记录（诊断）" />
  </div>;
}

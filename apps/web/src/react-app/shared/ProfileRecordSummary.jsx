import { behavioralFields, identityFields } from "@/approved-ui/profile-fields.mjs";
import { portraitPresentation, workingPersonaPresentation } from "@/approved-ui/profile-presentation.mjs";
import { stateLabel } from "@/v3/domain-presentation.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";
import { narrativeFields, readingFields, readingText, selectReadingFields } from "@/approved-ui/reading-fields.mjs";
import { readingFieldLabel } from "@/v3/proto/components/reading-fields.js";
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

/** @param {{value: any, shape?: object | true | typeof narrativeFields, identity?: object, title?: string, showClears?: boolean}} props */
export function ProfileFieldsSummary({ value, shape = profileShape, identity, title, showClears }) {
  const summary = profileFieldsForReading(value, shape, showClears);
  if (!hasVisibleSemanticValue(summary, "", identity)) return null;
  return <>
    {title && <h4 className="font-medium">{title}</h4>}
    <SemanticTree value={summary} identity={identity} reading />
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
        <b>{readingFieldLabel(line.field)}：</b>
        <SemanticTree value={line.value} field={line.field} identity={identity} reading />
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

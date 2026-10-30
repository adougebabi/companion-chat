import { useEffect, useId, useRef, useState } from "react";
import {
  ArrowRightIcon,
  ArrowUpRightIcon,
  Edit3Icon,
  MapPinIcon,
  QuoteIcon,
  RefreshCwIcon,
  RotateCcwIcon,
  ShieldCheckIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Progress } from "@/components/ui/progress";
import {
  AlertDialogAction,
  AlertDialogCancel,
} from "@/components/ui/alert-dialog";
import {
  Surface,
  ChoiceSelect,
  Textarea,
  FieldInput,
  Badge,
  Disclosure,
} from "../controls";
import CharacterAvatar from "../shared/CharacterAvatar";
import SemanticTree, { hasVisibleSemanticValue } from "../shared/SemanticTree";
import ProfileForm from "../shared/ProfileForm";
import ProfileGovernance from "../shared/ProfileGovernance";
import ConfirmationFrame from "../shared/ConfirmationFrame";
import WorkspacePageHeader from "../shared/WorkspacePageHeader";
import WorkspaceDialog from "../shared/WorkspaceDialog";
import WorkspaceInfo from "../shared/WorkspaceInfo";
import LoadingState from "../shared/LoadingState";
import JsonDetails from "../shared/JsonDetails";
import RequestStatus from "../shared/RequestStatus";
import WearingSummary from "../shared/WearingSummary";
import useWorkspaceRead from "../shared/useWorkspaceRead";
import { EmptyState, InlineFeedback } from "../shared/Presentation";
import { ActionFeedback } from "../components";
import { profileChanges } from "@/approved-ui/profile-fields.mjs";
import {
  semanticEntries,
  workingPersonaPresentation,
  portraitPresentation,
} from "@/approved-ui/profile-presentation.mjs";
import { semanticLabel } from "@/v3/proto/components/semantic-fields.js";
import { stateLabel } from "@/v3/domain-presentation.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";
const traitLabels = {
  openness: "开放性",
  conscientiousness: "尽责性",
  extraversion: "外向性",
  agreeableness: "亲和性",
  neuroticism: "情绪敏感度",
  curiosity: "好奇心",
  independence: "独立性",
  patience: "耐心",
  empathy: "共情",
  assertiveness: "主见",
  humor: "幽默感",
  sociability: "社交倾向",
  risk_tolerance: "冒险倾向",
};
const field = (label, control) => (
  <label className="replica-field">
    <span>{label}</span>
    {control}
  </label>
);
function Value({ value, field = "", identity }) {
  return hasVisibleSemanticValue(value, field, identity) ? (
    <SemanticTree value={value} field={field} identity={identity} />
  ) : (
    <span className="profile-empty-value">尚未提供</span>
  );
}
function Fact({ label, value, field, identity }) {
  return (
    <div className="profile-key-value">
      <span>{label}</span>
      <div>
        <Value value={value} field={field} identity={identity} />
      </div>
    </div>
  );
}
function OverviewFacts({entries,identity}) {
  const visible=entries.filter(([label,key,value])=>hasVisibleSemanticValue(value,key,identity));
  const missing=entries.filter(([label,key,value])=>!hasVisibleSemanticValue(value,key,identity));
  return <>{visible.length>0&&<div className="companion-fact-grid">{visible.map(([label,key,value])=><Fact key={key} label={label} value={value} field={key} identity={identity}/>)}</div>}{missing.length>0&&<p className="profile-empty-value">尚未填写：{missing.map(([label])=>label).join("、")}。可以通过本区编辑入口补充。</p>}</>;
}
function Section({ title, description, action, children, className = "" }) {
  return (
    <Surface className={"companion-profile-section " + className}>
      <div className="profile-section-heading">
        <div>
          <h2>{title}</h2>
          {description && <p>{description}</p>}
        </div>
        {action}
      </div>
      {children}
    </Surface>
  );
}
function Block({ title, value, identity, field = "" }) {
  return (
    <div className="companion-reading-block">
      <h3>{title}</h3>
      <Value value={value} field={field} identity={identity} />
    </div>
  );
}
function traitRows(value) {
  const rows = [];
  for (const [key, item] of semanticEntries(value || {})) {
    if (key === "traits" && item && typeof item === "object") {
      for (const [child, score] of semanticEntries(item)) {
        if (
          traitLabels[child] &&
          typeof score === "number" &&
          Number.isFinite(score) &&
          score >= 0 &&
          score <= 1
        )
          rows.push({
            key: "traits:" + child,
            label: "性格维度 · " + traitLabels[child],
            score,
          });
      }
    } else if (
      traitLabels[key] &&
      typeof item === "number" &&
      Number.isFinite(item) &&
      item >= 0 &&
      item <= 1
    )
      rows.push({ key, label: traitLabels[key], score: item });
  }
  return rows;
}
export default function CharacterDetailPageView({ model: m }) {
  const w = m.w,
    c = w.current.value,
    d = m.detail.value || {},
    core = m.core.value,
    identity = core.identity,
    snapshot = m.snapshot.value,
    personality = d.personality || d.core_persona?.personality || {},
    dimensions = traitRows(personality),
    current = m.currentPersona.value;
  const [tab, setTab] = useState("overview"),
    [editGroup, setEditGroup] = useState("basic"),
    [cancelOpen, setCancelOpen] = useState(false),
    [switchOpen, setSwitchOpen] = useState(false),
    [govOpen, setGovOpen] = useState(false),
    [govTab, setGovTab] = useState("foundation"),
    [sourcesOpen, setSourcesOpen] = useState(false);
  const editTrigger = useRef(null),
    switchTrigger = useRef(null),
    govTrigger = useRef(null),
    sourceTrigger = useRef(null),
    formId = useId();
  const foundation = useWorkspaceRead({
    identity: ["profile-foundation", snapshot.foundationRevision],
    enabled: tab === "history" || govOpen,
    read: async (cursor) => {
      const page = await w.client.governancePage(m.fid, "foundation", {
        cursor,
        limit: 20,
      });
      if (page.items.some((row) => row.fluctlight_id !== m.fid))
        throw Error("身份修订目录范围不匹配。");
      return page;
    },
  });
  const claims = useWorkspaceRead({
    identity: ["profile-live-self"],
    enabled: tab === "life",
    read: async (cursor) => {
      const page = await w.client.selfClaims(m.fid, {
        status: "active",
        cursor,
        limit: 5,
      });
      if (page.items.some((row) => row.fluctlight_id !== m.fid))
        throw Error("认识范围不匹配。");
      return page;
    },
  });
  useEffect(() => {
    if (tab === "life" && typeof w.loadWardrobe === "function")
      void w.loadWardrobe().catch(() => {});
  }, [tab, m.fid]);
  const dirty = (() => {
    try {
      return (
        m.editing.value &&
        Object.keys(profileChanges(core, m.fields.value)).length > 0
      );
    } catch {
      return true;
    }
  })();
  const personaOptions = m.personas.value.filter(
    (p) =>
      p.id !== snapshot.activeProfileId &&
      (!snapshot.switchTargetIds || snapshot.switchTargetIds.includes(p.id)),
  );
  const retainedTarget = m.personas.value.find(
    (p) =>
      p.id === m.target.value &&
      !personaOptions.some((option) => option.id === p.id),
  );
  const pending = m.pendingPersona.value,
    request = m.personaRecord.value,
    canOpenSwitch = m.personas.value.length > 1 && !m.readOnly.value;
  useEffect(() => {
    const receipt = m.personaReceipt.value;
    if (
      switchOpen &&
      m.personaFeedback.value &&
      receipt?.status === "completed" &&
      !m.personaSubmitted.value &&
      !m.personaReadbackPending.value &&
      !m.target.value &&
      !m.reason.value
    )
      setSwitchOpen(false);
  }, [m.personaFeedback.value, m.personaReadbackPending.value]);
  const edit = (group, event) => {
    editTrigger.current = event.currentTarget;
    setEditGroup(group);
    m.edit();
  };
  const openGov = (group, event) => {
    govTrigger.current = event.currentTarget;
    setGovTab(group);
    setGovOpen(true);
  };
  const cancel = () => {
    if (m.saving.value) return;
    if (dirty) setCancelOpen(true);
    else m.editing.value = false;
  };
  const refresh = () =>
    m.last?.value?.readbackPending
      ? m.recoverProfileReadback()
      : m.read({ preserve: m.editing.value });
  const biography = snapshot.bio || "",
    tags = Array.isArray(identity.tags)
      ? identity.tags.filter((v) => typeof v === "string")
      : [];
  const policyFact = (key) => {
    const policy = core.behavioral_policy;
    const value = Object.hasOwn(policy, key)
      ? policy[key]
      : Object.hasOwn(policy.expression || {}, key)
        ? policy.expression[key]
        : key === "response_style"
          ? policy.reply_style
          : undefined;
    return key === "message_length"
      ? { short: "简短", medium: "适中", long: "详细" }[value] || value
      : value;
  };
  const working = workingPersonaPresentation(
      snapshot.groups.find((group) => group.key === "working")?.value || {},
    ),
    workingProfileMismatch = typeof working?.profile_id === "string" && working.profile_id !== snapshot.activeProfileId,
    workingPortrait = !workingProfileMismatch && typeof working?.portrait_text === "string" ? working.portrait_text : "",
    remainingSystem = Object.fromEntries(
      Object.entries(d.core_persona?.personality_system || {}).filter(
        ([key]) => key !== "profiles",
      ),
    );
  return (
    <div className="replica-profile companion-page">
      <WorkspacePageHeader
        eyebrow="LIFE / A PERSON WITH A STORY"
        title={"认识更完整的" + m.title.value + "。"}
        description="来处、性格、习惯，以及相处中慢慢长出的自己。"
        actions={
          <>
            <Button
              variant="ghost"
              size="icon"
              aria-label="刷新角色资料"
              disabled={
                m.readingDetail.value || m.saving.value || m.switching.value
              }
              onClick={() => void refresh()}
            >
              <RefreshCwIcon size={15} />
            </Button>
            <Button variant="outline" onClick={(e) => openGov("foundation", e)}>
              <ShieldCheckIcon size={15} />
              身份与能力
            </Button>
            <Button
              data-testid="detail-edit"
              disabled={!m.detail.value || m.readOnly.value}
              onClick={(e) => edit("basic", e)}
            >
              <Edit3Icon size={15} />
              编辑资料
            </Button>
          </>
        }
      />
      <Surface className="companion-profile-hero">
        <CharacterAvatar
          className="avatar-xl"
          name={m.title.value}
          initial={[...m.title.value][0]}
          color={c.color}
          src={m.avatar.value}
        />
        <div className="companion-profile-intro">
          <span className="companion-eyebrow">
            A PERSON, NOT JUST A PROFILE
          </span>
          <h2>
            {m.title.value}
            {identity.nickname && <span>{identity.nickname}</span>}
          </h2>
          <p>{m.subtitle.value || "已确认的资料会显示在这里。"}</p>
          <div className="companion-tag-row">
            {tags.map((tag) => (
              <Badge variant="secondary" key={tag}>
                {tag}
              </Badge>
            ))}
            {m.world.value?.name && (
              <Badge variant="outline">
                <MapPinIcon size={11} />
                {m.world.value.name}
              </Badge>
            )}
          </div>
        </div>
        <div className="companion-profile-quote">
          <QuoteIcon size={19} />
          <p>
            {typeof identity.tagline === "string" && identity.tagline.trim()
              ? identity.tagline : typeof biography === "string"
              ? biography ||
                (!m.detail.value
                  ? "角色资料待读取。"
                  : "还没有提供简介，之后可以慢慢补充。")
              : "完整简介见下方档案。"}
          </p>
          <Badge variant="outline">
            {current?.name ||
              (!m.detail.value ? "待读取人格" : "当前人格尚未确认")}
          </Badge>
        </div>
      </Surface>
      {m.readingDetail.value && !m.detail.value && (
        <LoadingState label="正在读取角色资料…" />
      )}
      {m.error.value && !m.editing.value && (
        <InlineFeedback role="alert">
          {m.error.value}
          <Button variant="outline" onClick={() => void refresh()}>
            重新读取角色资料
          </Button>
        </InlineFeedback>
      )}
      {d.persona_preparation && d.persona_preparation.status !== "ready" && (
        <WorkspaceInfo>
          {d.persona_preparation.status === "failed"
            ? "表达画像准备遇到问题，可继续原角色的准备。"
            : "角色资料已保存，正在后台整理表达画像。"}
          {d.persona_preparation.can_recover && (
            <Button
              variant="link"
              disabled={
                m.saving.value ||
                m.editing.value ||
                m.readOnly.value ||
                !Number.isSafeInteger(snapshot.foundationRevision)
              }
              onClick={() => void m.recoverPreparation()}
            >
              继续准备
            </Button>
          )}
        </WorkspaceInfo>
      )}
      {m.personaReadbackPending.value && m.personaSubmitted.value && (
        <WorkspaceInfo>
          人格切换请求与当前状态仍在跟进，原目标和原因会保留。
          <Button
            variant="outline"
            disabled={m.readingDetail.value}
            onClick={() => void m.readPersonaOutcome()}
          >
            重新读取人格切换结果
          </Button>
        </WorkspaceInfo>
      )}
      {m.personaFeedback.value && (
        <WorkspaceInfo>{m.personaFeedback.value}</WorkspaceInfo>
      )}
      {m.pendingProfile.value && !m.editing.value && (
        <WorkspaceInfo>
          上次资料保存的回执仍待核对，当前资料以实际读回为准。
          <Button
            variant="outline"
            disabled={!m.detail.value || m.saving.value || m.readOnly.value}
            onClick={() => void m.resumeProfileSave()}
          >
            核对或重试原资料保存
          </Button>
        </WorkspaceInfo>
      )}
      {m.personaRequestId.value &&
        m.personaSubmitted.value &&
        !request?.requestUnresolved && (
          <RequestStatus
            fid={m.fid}
            requestId={m.personaRequestId.value}
            initial={m.personaReceipt.value?.request}
            onTerminal={(value) => void m.readPersonaOutcome(value)}
          />
        )}
      <div hidden={!m.detail.value}>
        <Tabs value={tab} onValueChange={setTab} className="profile-main-tabs">
          <TabsList variant="line" className="replica-segment-tabs">
            <TabsTrigger value="overview">身份与背景</TabsTrigger>
            <TabsTrigger value="persona">性格与人格</TabsTrigger>
            <TabsTrigger value="life">生活与此刻</TabsTrigger>
            <TabsTrigger value="history">
              修订与来源{" "}
              <span>{m.detail.value ? snapshot.revisions.length : "…"}</span>
            </TabsTrigger>
          </TabsList>
          <TabsContent value="overview">
            <div className="companion-two-columns">
              <div className="companion-stack">
                <Section
                  title="身份与背景"
                  action={
                    <Button
                      variant="ghost"
                      size="sm"
                      disabled={m.readOnly.value || !m.detail.value}
                      onClick={(e) => edit("basic", e)}
                    >
                      <Edit3Icon size={13} />
                      编辑
                    </Button>
                  }
                >
                  <OverviewFacts identity={identity} entries={[["称呼","nickname",identity.nickname],["年龄","age",identity.age],["职业","occupation",identity.occupation],["居住地","residence",identity.residence],["生活时区","timezone",identity.timezone],["出生地","birthplace",identity.birthplace]]}/>
                  {hasVisibleSemanticValue(biography)&&<Block title="简介与经历" value={biography} identity={identity}/>}
                  {hasVisibleSemanticValue(identity.biography) &&
                    identity.biography !== biography && (
                      <Block
                        title="生平"
                        value={identity.biography}
                        identity={identity}
                      />
                    )}
                </Section>
                <Section
                  title="表达与相处"
                  action={
                    <Button
                      variant="ghost"
                      size="sm"
                      disabled={m.readOnly.value || !m.detail.value}
                      onClick={(e) => edit("expression", e)}
                    >
                      调整表达
                      <ArrowRightIcon size={13} />
                    </Button>
                  }
                >
                  <OverviewFacts identity={identity} entries={[["回复风格","response_style",policyFact("response_style")],["消息长短","message_length",policyFact("message_length")],["分歧处理","conflict_style",policyFact("conflict_style")],["拒绝方式","refusal_style",policyFact("refusal_style")]]}/>
                  {hasVisibleSemanticValue(identity.core_values)&&<Block title="核心价值观" value={identity.core_values} identity={identity}/>}
                  <Disclosure title="完整表达与相处设定">
                    <SemanticTree
                      value={core.behavioral_policy}
                      identity={identity}
                    />
                  </Disclosure>
                </Section>
              </div>
              <div className="companion-stack">
                <Section
                  title="共同性格"
                  description="这些倾向构成了相处时的底色。"
                >
                  {dimensions.length ? (
                    <div className="companion-dimension-list">
                      {dimensions.map((item) => (
                        <div key={item.key}>
                          <div>
                            <span>{item.label}</span>
                            <strong>{Math.round(item.score * 100)}%</strong>
                          </div>
                          <Progress
                            value={item.score * 100}
                            aria-label={item.label}
                          />
                        </div>
                      ))}
                    </div>
                  ) : (
                    <p className="companion-body-text">
                      当前尚未提供可读取的性格维度，这里保留留白。
                    </p>
                  )}
                  <Disclosure title="完整性格与来源">
                    <SemanticTree
                      value={personality}
                      field="personality"
                      identity={identity}
                    />
                    <JsonDetails value={personality} title="性格完整记录" />
                  </Disclosure>
                </Section>
                <Surface className="companion-portrait-note">
                  <span className="companion-eyebrow">WORKING PERSONA</span>
                  <div className="companion-row-between"><h3>当前表达画像</h3>{typeof working?.status==="string"&&working.status&&<Badge variant="outline">{stateLabel(working.status)}</Badge>}</div>
                  {workingProfileMismatch?<p>当前人格的表达画像尚未读回，可核对保留的完整记录。</p>:workingPortrait?<div data-testid="current-working-portrait">{portraitPresentation(workingPortrait,{includeOccurrences:true}).map((line,index)=><p key={index}>{line.field?<><b>{semanticLabel(line.field)}：</b><Value value={line.value} field={line.field} identity={identity}/>{line.occurrences>1&&<small className="r-muted">（原文出现 {line.occurrences} 次）</small>}</>:line.text||'\u00a0'}</p>)}</div>:<Value value={working} identity={identity} />}
                  {(workingPortrait||workingProfileMismatch)&&<JsonDetails value={working} title="表达画像完整记录与来源"/>}
                  <span>人格设定与最近生活共同影响此刻的表达</span>
                </Surface>
              </div>
            </div>
          </TabsContent>
          <TabsContent value="persona">
            <div className="companion-two-columns">
              <div className="companion-stack">
                <Section
                  title="人格档案"
                  description="共同设定之上，每个人格保留自己的表达与边界。"
                  action={
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={!canOpenSwitch}
                      onClick={(event) => {
                        switchTrigger.current = event.currentTarget;
                        if (
                          !m.personaSubmitted.value &&
                          !m.pendingPersona.value
                        ) {
                          m.personaFeedback.value = "";
                          m.error.value = "";
                        }
                        setSwitchOpen(true);
                      }}
                    >
                      <RotateCcwIcon size={13} />
                      切换人格
                    </Button>
                  }
                >
                  <div className="companion-persona-list">
                    {m.personas.value.map((p) => (
                      <article
                        className={
                          "companion-persona-card" +
                          (p.id === snapshot.activeProfileId ? " active" : "")
                        }
                        key={p.id}
                      >
                        <div className="companion-row-between">
                          <h3>{p.name || "尚未命名的人格"}</h3>
                          <Badge variant="outline">
                            {p.id === snapshot.activeProfileId
                              ? "当前人格"
                              : "人格档案"}
                          </Badge>
                        </div>
                        <SemanticTree
                          value={m.personaValue(p)}
                          identity={identity}
                        />
                      </article>
                    ))}
                  </div>
                  {m.detail.value && !m.personas.value.length && (
                    <EmptyState>
                      <h3>尚未登记人格档案</h3>
                    </EmptyState>
                  )}
                </Section>
                <Section title="人格之间的关系与协作">
                  <p className="companion-body-text">
                    人格切换和生活事实分别记录，实际穿着、所在世界和已经发生的经历继续以对应读面为准。
                  </p>
                  <Value value={remainingSystem} identity={identity} />
                </Section>
              </div>
              <Section
                title="人格切换记录"
                description="每次实际切换的目标、原因和版本可在这里查阅。"
              >
                {snapshot.history.length ? (
                  <div className="companion-timeline">
                    {snapshot.history.map((item, index) => (
                      <div key={item.audit_id || item.id || index}>
                        <span className="companion-timeline-dot" />
                        <small>
                          {userInstant(item.occurred_at || item.created_at)}
                        </small>
                        <h4>
                          {m.personas.value.find(
                            (p) => p.id === item.from_profile_id,
                          )?.name || "原人格"}
                          <ArrowRightIcon size={12} />
                          {m.personas.value.find(
                            (p) => p.id === item.to_profile_id,
                          )?.name || "目标人格"}
                        </h4>
                        <p>{item.reason || "未记录额外说明。"}</p>
                        <JsonDetails value={item} title="完整切换与版本记录" />
                      </div>
                    ))}
                  </div>
                ) : m.detail.value ? (
                  <EmptyState>
                    <h3>还没有读到人格切换记录</h3>
                  </EmptyState>
                ) : (
                  <p className="profile-empty-value">记录待读取。</p>
                )}
                {snapshot.historyHasMore && (
                  <p className="companion-caption">
                    这里展示最近的记录，完整来源保留在治理与诊断中。
                  </p>
                )}
                <ActionFeedback
                  labels={["角色详情"]}
                  operation="persona-switch"
                  showStatus
                />
              </Section>
            </div>
          </TabsContent>
          <TabsContent value="life">
            <div className="companion-two-columns">
              <Section
                title="生活设定"
                action={
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={m.readOnly.value || !m.detail.value}
                    onClick={(e) => edit("life", e)}
                  >
                    <Edit3Icon size={13} />
                    编辑
                  </Button>
                }
              >
                {[
                  ["兴趣与偏好", core.life_profile.preferences],
                  ["日常习惯", core.life_profile.life_habits],
                  ["相处边界", core.life_profile.character_constraints],
                  ["外貌特征", core.life_profile.appearance],
                  [
                    "生活设定的其他记录",
                    Object.fromEntries(
                      Object.entries(core.life_profile).filter(
                        ([key]) =>
                          ![
                            "preferences",
                            "life_habits",
                            "character_constraints",
                            "appearance",
                            "wardrobe_items",
                          ].includes(key),
                      ),
                    ),
                  ],
                ].map(([title, value]) => (
                  <Block
                    key={title}
                    title={title}
                    value={value}
                    identity={identity}
                  />
                ))}
              </Section>
              <div className="companion-stack">
                <Section title="此刻状态">
                  <Fact
                    label="所在世界"
                    value={m.world.value?.name}
                    identity={identity}
                  />
                  {m.worldLoading.value && (
                    <LoadingState label="正在读取生活世界…" />
                  )}
                  {m.worldError.value && (
                    <InlineFeedback role="alert">
                      {m.worldError.value}
                      <Button
                        variant="outline"
                        onClick={() => void m.readWorld()}
                      >
                        重新读取生活世界
                      </Button>
                    </InlineFeedback>
                  )}
                  <Fact
                    label="此刻地点"
                    value={d.context?.location}
                    identity={identity}
                  />
                  <Fact
                    label="正在做"
                    value={d.context?.activity}
                    identity={identity}
                  />
                  <Fact
                    label="心情"
                    value={d.inner_state?.mood || d.current_state?.mood}
                    identity={identity}
                  />
                  <div className="profile-key-value">
                    <span>当前穿着</span>
                    <WearingSummary
                      character={c}
                      loading={!!w.state.loading?.wardrobe}
                      error={w.state.errors?.wardrobe || ""}
                    />
                  </div>
                  <Disclosure title="实际外观与状态记录">
                    <Value value={d.current_state} identity={identity} />
                    <JsonDetails value={d.current_state} title="原始当前状态" />
                  </Disclosure>
                  <div className="companion-action-row">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() =>
                        w.navigatePath(
                          "/c/" + encodeURIComponent(m.fid) + "/life/now",
                        )
                      }
                    >
                      查看此刻
                      <ArrowRightIcon size={12} />
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() =>
                        w.navigatePath(
                          "/c/" + encodeURIComponent(m.fid) + "/life/wardrobe",
                        )
                      }
                    >
                      查看衣柜
                    </Button>
                  </div>
                  <p className="companion-caption">
                    这里展示实际状态，编辑设定不会直接覆盖它们。
                  </p>
                </Section>
                <Section title="发展中的自我">
                  {claims.busy && <LoadingState label="正在读取自我认识…" />}
                  {claims.error && (
                    <InlineFeedback role="alert">
                      {claims.error}
                      <Button variant="outline" onClick={claims.retry}>
                        重新读取自我认识
                      </Button>
                    </InlineFeedback>
                  )}
                  {claims.items.map((row) => (
                    <p className="companion-body-text" key={row.id}>
                      {row.claim}
                    </p>
                  ))}
                  {claims.ready && !claims.items.length && !claims.error && (
                    <p className="companion-body-text">
                      当前还没有读到已记录的认识。
                    </p>
                  )}
                  <Disclosure title="档案中的发展记录">
                    <Value value={d.developing_self} identity={identity} />
                  </Disclosure>
                  <Button
                    variant="ghost"
                    className="companion-full-link"
                    onClick={() =>
                      w.navigatePath(
                        "/c/" + encodeURIComponent(m.fid) + "/growth?tab=self",
                      )
                    }
                  >
                    查看认识与来源
                    <ArrowRightIcon size={14} />
                  </Button>
                </Section>
              </div>
            </div>
          </TabsContent>
          <TabsContent value="history">
            <div className="companion-two-columns">
              <Section
                title="资料修订记录"
                description="当前设定与每次修改的来由，都可以回看。"
              >
                {foundation.busy && <LoadingState label="正在读取资料修订…" />}
                {foundation.error && (
                  <InlineFeedback role="alert">
                    {foundation.error}
                    <Button variant="outline" onClick={foundation.retry}>
                      重新读取资料修订
                    </Button>
                  </InlineFeedback>
                )}
                <div className="companion-timeline">
                  {foundation.items.map((item) => (
                    <div key={item.id}>
                      <span className="companion-timeline-dot" />
                      <small>
                        {userInstant(item.accepted_at || item.created_at)}
                      </small>
                      <h4>
                        {item.source === "initialization"
                          ? "角色初始资料"
                          : "身份修订 v" + item.revision}
                        <Badge variant="outline">
                          {stateLabel(item.status)}
                        </Badge>
                      </h4>
                      <p>{item.reason || "未记录额外说明。"}</p>
                      <JsonDetails value={item} title="该次修订与来源" />
                    </div>
                  ))}
                </div>
                {foundation.ready &&
                  !foundation.items.length &&
                  !foundation.error && (
                    <EmptyState>
                      <h3>尚未读到资料修订记录</h3>
                    </EmptyState>
                  )}
                {foundation.cursor && (
                  <Button
                    variant="outline"
                    disabled={foundation.busy}
                    onClick={foundation.more}
                  >
                    加载更多资料修订
                  </Button>
                )}
              </Section>
              <div className="companion-stack">
                <Section title="身份核心与提议">
                  <p className="companion-body-text">
                    生活中形成的变化，可以先作为提议审阅，再纳入稳定设定。
                  </p>
                  <Badge variant="outline">
                    {m.detail.value
                      ? snapshot.revisions.filter(
                          (row) => row.status === "proposed",
                        ).length
                      : "…"}{" "}
                    条已读待审阅提议
                  </Badge>
                  <Button
                    className="companion-full-link"
                    variant="outline"
                    onClick={(e) => openGov("foundation", e)}
                  >
                    审阅身份提议
                    <ArrowRightIcon size={14} />
                  </Button>
                </Section>
                <Section title="来源与版本">
                  <Fact
                    label="初始来源"
                    value={
                      d.initialization_source?.mode || d.initialization_mode
                    }
                    identity={identity}
                  />
                  <Fact
                    label="身份核心版本"
                    value={snapshot.foundationRevision}
                    identity={identity}
                  />
                  <Fact
                    label="人格运行版本"
                    value={snapshot.runtimeRevision}
                    identity={identity}
                  />
                  <Button
                    variant="ghost"
                    className="companion-full-link"
                    onClick={(e) => {
                      sourceTrigger.current = e.currentTarget;
                      setSourcesOpen(true);
                    }}
                  >
                    查看完整设定与来源
                    <ArrowRightIcon size={13} />
                  </Button>
                  <Button
                    variant="ghost"
                    className="companion-full-link"
                    asChild
                  >
                    <a
                      href={
                        "#/c/" + encodeURIComponent(m.fid) + "/media/avatar"
                      }
                    >
                      更换显示头像
                      <ArrowUpRightIcon size={13} />
                    </a>
                  </Button>
                </Section>
              </div>
            </div>
          </TabsContent>
        </Tabs>
      </div>
      <WorkspaceDialog
        open={m.editing.value}
        busy={m.saving.value}
        wide
        className="replica-profile"
        returnFocus={() => editTrigger.current}
        onOpenChange={(open) => {
          if (!open) cancel();
        }}
        title={"编辑" + m.title.value + "的资料"}
        description="编辑稳定设定。穿着、位置和已经发生的事情，继续以生活记录为准。"
        footer={
          <>
            <Button
              variant="outline"
              disabled={m.saving.value}
              onClick={cancel}
            >
              取消
            </Button>
            <Button
              data-testid="detail-save"
              type="submit"
              form={formId}
              disabled={
                m.saving.value || m.readbackPending.value || m.readOnly.value
              }
            >
              {m.saving.value ? "保存中…" : "保存资料"}
            </Button>
          </>
        }
      >
        <form
          id={formId}
          className="companion-form-stack"
          noValidate
          onSubmit={async (e) => {
            e.preventDefault();
            const name = m.fields.value.find(
              (f) => f.path.join(".") === "identity.name",
            );
            if (!String(name?.value || "").trim()) {
              setEditGroup("basic");
              requestAnimationFrame(() =>
                document.querySelector('[aria-label="名字"]')?.focus(),
              );
            }
            await m.save();
          }}
        >
          <ProfileForm
            replica
            fields={m.fields.value}
            group={editGroup}
            onGroupChange={setEditGroup}
            disabled={m.readOnly.value}
          />
          {m.error.value && (
            <InlineFeedback role="alert">{m.error.value}</InlineFeedback>
          )}
          <ActionFeedback labels={["角色详情"]} />
          {m.pendingProfile.value && (
            <Button
              type="button"
              variant="outline"
              disabled={m.saving.value || m.readOnly.value}
              onClick={() => void m.resumeProfileSave()}
            >
              核对或重试原资料保存
            </Button>
          )}
          {m.readbackPending.value && (
            <Button
              variant="outline"
              type="button"
              onClick={() => void m.recoverProfileReadback()}
            >
              重新读取已保存资料
            </Button>
          )}
          {m.conflict.value && (
            <Button
              variant="outline"
              type="button"
              onClick={() => void m.rebase()}
            >
              读取最新资料，保留当前编辑
            </Button>
          )}
        </form>
      </WorkspaceDialog>
      {cancelOpen && (
        <ConfirmationFrame
          title="放下这次修改？"
          description="尚未保存的修改会被放弃，已保存的资料会保留。"
          onClose={() => setCancelOpen(false)}
        >
          <AlertDialogCancel>继续编辑</AlertDialogCancel>
          <AlertDialogAction
            onClick={() => {
              m.editing.value = false;
              setCancelOpen(false);
            }}
          >
            放弃修改
          </AlertDialogAction>
        </ConfirmationFrame>
      )}
      <WorkspaceDialog
        open={switchOpen && !m.readOnly.value}
        className="replica-profile"
        returnFocus={() => switchTrigger.current}
        onOpenChange={setSwitchOpen}
        title="切换当前人格"
        description={
          "现在是「" +
          (current?.name || "尚未确认") +
          "」。提出请求后，以实际处理结果为准。"
        }
        footer={
          <>
            <Button variant="outline" onClick={() => setSwitchOpen(false)}>
              取消
            </Button>
            <Button
              form={formId + "-persona"}
              type="submit"
              disabled={
                m.switching.value ||
                m.readOnly.value ||
                request?.requestUnresolved ||
                m.personaReadbackPending.value ||
                !!pending ||
                !m.target.value ||
                !m.reason.value.trim() ||
                !m.confirm.value
              }
            >
              确认切换
            </Button>
          </>
        }
      >
        <form
          id={formId + "-persona"}
          className="companion-form-stack"
          onSubmit={(e) => {
            e.preventDefault();
            void m.switchPersona();
          }}
        >
          {field(
            "目标人格",
            <ChoiceSelect
              aria-label="目标人格"
              value={m.target.value}
              onChange={(e) => {
                m.target.value = e.target.value;
                m.confirm.value = false;
              }}
            >
              <option value="">请选择</option>
              {retainedTarget && (
                <option value={retainedTarget.id}>
                  {retainedTarget.name || "尚未命名的人格"}（原请求目标）
                </option>
              )}
              {personaOptions.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name || "尚未命名的人格"}
                </option>
              ))}
            </ChoiceSelect>,
          )}
          {field(
            "切换原因",
            <Textarea
              aria-label="切换原因"
              value={m.reason.value}
              onChange={(e) => {
                m.reason.value = e.target.value;
                m.confirm.value = false;
              }}
              maxLength={1000}
            />,
          )}
          <label className="companion-checkbox-row">
            <FieldInput
              type="checkbox"
              aria-label="确认向当前角色提出切换"
              checked={m.confirm.value}
              onChange={(e) => (m.confirm.value = e.target.checked)}
            />
            我确认向当前角色提出这次人格切换
          </label>
          {!personaOptions.length && (
            <WorkspaceInfo>当前未读到可向其申请切换的其他人格。</WorkspaceInfo>
          )}
          {pending && (
            <WorkspaceInfo>
              原切换结果尚未确认。
              {pending.rejection
                ? "请核对版本和原操作记录。"
                : "请查询或重试原提交，当前新输入会保留。"}
            </WorkspaceInfo>
          )}
          {pending && !pending.rejection && (
            <Button
              type="button"
              variant="outline"
              disabled={m.switching.value}
              onClick={() => void m.resumePersona()}
            >
              核对或重试原切换
            </Button>
          )}
          {m.personaReadbackPending.value && (
            <Button
              type="button"
              variant="outline"
              disabled={m.readingDetail.value}
              onClick={() => void m.readPersonaOutcome()}
            >
              重新读取人格切换结果
            </Button>
          )}
          {m.error.value && (
            <InlineFeedback role="alert">{m.error.value}</InlineFeedback>
          )}
          {m.personaFeedback.value && (
            <WorkspaceInfo>{m.personaFeedback.value}</WorkspaceInfo>
          )}
          <ActionFeedback
            labels={["角色详情"]}
            operation="persona-switch"
            showStatus
          />
        </form>
      </WorkspaceDialog>
      {govOpen && (
        <ProfileGovernance
          model={m}
          open
          onClose={() => setGovOpen(false)}
          initialTab={govTab}
          foundation={foundation}
          returnFocus={() => govTrigger.current}
        />
      )}
      <WorkspaceDialog
        open={sourcesOpen}
        wide
        className="replica-profile"
        returnFocus={() => sourceTrigger.current}
        onOpenChange={setSourcesOpen}
        title="完整设定与来源"
        description="读取当前正式档案、工作人格与版本记录。"
      >
        <SemanticTree
          value={{
            identity: d.identity,
            personality: d.personality,
            behavioral_policy: d.behavioral_policy,
            life_profile: d.life_profile,
          }}
          identity={identity}
        />
        <JsonDetails value={d} title="完整档案、版本与来源（可下载）" />
      </WorkspaceDialog>
      {!m.editing.value && tab !== "persona" && (
        <ActionFeedback labels={["角色详情"]} />
      )}
      <div className="profile-secondary-links">
        <Button variant="link" asChild>
          <a href={m.governance("capabilities")}>完整能力申请与历史</a>
        </Button>
        <Button variant="link" onClick={(e) => openGov("self", e)}>
          发展中的自我
        </Button>
      </div>
    </div>
  );
}

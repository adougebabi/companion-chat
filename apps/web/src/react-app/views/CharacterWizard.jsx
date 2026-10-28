import { useRef, useState } from "react";
import {
  ArrowLeft,
  ArrowRight,
  BookOpen,
  Braces,
  Check,
  ChevronRight,
  FilePlus2,
  Globe2,
  LayoutTemplate,
  LoaderCircle,
  Plus,
  RefreshCw,
  Search,
  ShieldCheck,
  Sparkles,
  Sprout,
  UserRound,
  WandSparkles,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Slider } from "@/components/ui/slider";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Surface,
  FieldInput,
  Textarea,
  ChoiceSelect,
  Disclosure,
  Label,
  Badge,
} from "../controls";
import WorkspacePageHeader from "../shared/WorkspacePageHeader";
import WorkspaceDialog from "../shared/WorkspaceDialog";
import WorkspaceInfo from "../shared/WorkspaceInfo";
import { InlineFeedback } from "../shared/Presentation";
import LoadingState from "../shared/LoadingState";
import ProfileField from "../shared/ProfileField";
import PreparationSteps from "../shared/PreparationSteps";
import SemanticTree from "../shared/SemanticTree";
import JsonDetails from "../shared/JsonDetails";
import Combobox from "../shared/Combobox";
import { WorldArt } from "../shared/WorkspaceArtwork";
import WorkspaceCharacterSketch from "../shared/WorkspaceCharacterSketch";
const steps = [
  ["生活世界", "故事从哪里开始", Globe2],
  ["认识角色", "选择一种认识的方式", Sparkles],
  ["资料预览", "把每个细节看清楚", UserRound],
  ["准备相遇", "让生活慢慢展开", WandSparkles],
];
const sources = [
  ["blank", "从空白开始", "名字之外，留给相处", FilePlus2],
  ["description", "一段描述", "用文字勾勒这个人", BookOpen],
  ["json", "导入 JSON", "带来已有的完整设定", Braces],
  ["template", "角色模板", "从发布的底稿出发", LayoutTemplate],
];
const sections = [
  ["identity", "身份与背景"],
  ["personality", "性格与表达"],
  ["life", "生活设定"],
  ["relationships", "人格与关系"],
  ["json", "完整设定"],
];
const identityOrder = ['name','nickname','gender','age','occupation','birthday','timezone','residence','birthplace','height_cm','blood_type','core_values','background_story','worldview','notes'];
const traits = {
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
const lifeFields = {
  appearance: "外貌印象",
  social_background: "社会背景",
  preferences: "喜欢与不喜欢",
  life_habits: "生活习惯",
  recurring_commitments: "长期约定",
  character_constraints: "边界与约束",
  wardrobe_items: "初始衣物",
  media_preferences: "媒体偏好",
};
const copy = (v) => JSON.parse(JSON.stringify(v));
function download(value, name) {
  downloadText(JSON.stringify(value, null, 2), name);
}
function downloadText(text, name) {
  const url = URL.createObjectURL(
      new Blob([text], { type: "application/json" }),
    ),
    a = document.createElement("a");
  a.href = url;
  a.download = name;
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 0);
}
function KeyValue({ label, children }) {
  return (
    <div className="creation-key-value">
      <span>{label}</span>
      <div>{children ?? "尚未提供"}</div>
    </div>
  );
}
function Segment({ value, onChange, items, label }) {
  return (
    <Tabs value={value} onValueChange={onChange} className="creation-segments">
      <TabsList
        variant="line"
        aria-label={label}
        className="creation-segment-tabs"
      >
        {items.map(([id, text]) => (
          <TabsTrigger value={id} key={id}>
            {text}
          </TabsTrigger>
        ))}
      </TabsList>
    </Tabs>
  );
}
export default function CharacterWizardView({ model: m }) {
  const { c, draft: d, step, busy, pending, currentStep: index } = m,
    locked = !!d.activatedId || !!pending,
    fields = m.previewFields;
  const [worldTab, setWorldTab] = useState(
      d.worldSource
        ? "existing"
        : d.worldResourceId
          ? "resource"
          : d.worldChoiceConfirmed && d.useDefaultWorld
            ? "daily"
            : "existing",
    ),
    [search, setSearch] = useState(""),
    [reviewTab, setReviewTab] = useState("identity"),
    [snapshotOpen, setSnapshotOpen] = useState(false),
    [resetOpen, setResetOpen] = useState(false),
    [structured, setStructured] = useState(null),
    [structuredError, setStructuredError] = useState("");
  const snapshotTrigger = useRef(null),
    recoveryTrigger = useRef(null),
    resetTrigger = useRef(null),
    structuredTrigger = useRef(null);
  let definition = null,
    definitionError = "";
  try {
    definition = c.previewDefinition();
  } catch (cause) {
    definitionError = cause.message;
  }
  const identityName = fields.find(
    (f) => f.path.join(".") === "identity.name",
  )?.value;
  const name = String(identityName ?? d.name ?? "").trim() || "新的相识",
    worldName =
      d.useDefaultWorld && d.worldChoiceConfirmed
        ? "日常生活世界"
        : d.worldName ||
          d.worldSource?.name ||
          m.selectedPublication?.name ||
          "待选择";
  const selectedWorld = m.existingWorlds.find(
    (w) =>
      w.id === d.worldSource?.worldId &&
      w.fluctlightId === d.worldSource?.fluctlightId,
  );
  const groupName = m.groups.find((g) => g.id === d.groupId)?.name || "未分组",
    done = m.progress?.stages.filter((s) => s.state === "ready").length || 0,
    worldBindingConfirmed =
      !!d.worldSelectionConfirmed ||
      m.progress?.stages.some((s) => s.key === "world" && s.state === "ready");
  const edit = (key, value) => {
    if (busy || locked) return;
    d[key] = value;
    c.invalidate();
  };
  const back = () => {
    if (busy || locked) return;
    m.prev();
  };
  const applyDefinition = (next, options) =>
    c.updatePreviewDefinition(next, options);
  const editStructure = (event, label, path, value) => {
    if (!definition || definitionError || busy || locked) return;
    if (d.previewJsonText !== undefined) {
      m.showError("请先应用或还原完整 JSON 修改，再编辑其他结构。");
      return;
    }
    structuredTrigger.current = event.currentTarget;
    setStructured({
      label,
      path,
      text: JSON.stringify(
        value === undefined ? (path[0] === "core_persona" ? {} : []) : value,
        null,
        2,
      ),
      kind: Array.isArray(value)
        ? "array"
        : value === undefined
          ? "missing"
          : value === null
            ? "null"
            : typeof value,
      base: JSON.stringify(definition),
    });
    setStructuredError("");
  };
  const setPath = (root, path, value) => {
    let at = root;
    for (const k of path.slice(0, -1)) at = at[k] ??= {};
    at[path.at(-1)] = value;
    return root;
  };
  const renderStructure = (label, path, value) => (
    <Surface className="creation-structured" key={path.join(".")}>
      <div className="creation-section-heading">
        <h2>{label}</h2>
        <Button
          type="button"
          size="sm"
          variant="ghost"
          disabled={busy || locked || !!definitionError}
          onClick={(event) => editStructure(event, label, path, value)}
        >
          编辑内容
        </Button>
      </div>
      <SemanticTree value={value} />
    </Surface>
  );
  const next = () => {
    if (index === 0) m.canContinue && c.next();
    else if (index === 1) m.canContinue && c.analyze();
    else if (index === 2) m.canContinue && c.activate();
  };
  const worldRows = (
    worldTab === "resource" ? m.worlds : m.existingWorlds
  ).filter((row) =>
    [row.name, row.characterName].some((t) => String(t || "").includes(search)),
  );
  return (
    <div ref={m.rootRef} className="replica-creation creation-page">
      <WorkspacePageHeader
        eyebrow="A NEW BEGINNING / 新的相识"
        title={
          [
            "先为相遇，选一个世界。",
            "你想怎样认识这个人？",
            "把轮廓，变成鲜活的人。",
            m.progress?.complete
              ? "一切就绪，去相遇吧。"
              : "生活，正在慢慢展开。",
          ][index]
        }
        description={
          [
            "世界是日常的背景。可以复用已有配置，也可以从一份新底稿开始。",
            "不必一次写完所有细节，预览时还可以继续调整。",
            "按主题逐组确认。未提供的信息会留白，完整结构与扩展字段都会保留。",
            "角色、世界与每一项准备各自记录进度，离开页面后也可以回来继续。",
          ][index]
        }
        actions={
          <Button variant="ghost" asChild>
            <a href="#/library">
              <ArrowLeft size={15} />
              返回角色库
            </a>
          </Button>
        }
      />
      <nav className="creation-steps" aria-label="创建步骤">
        {steps.map(([label, note, Icon], i) => (
          <button
            type="button"
            key={label}
            className={
              "creation-step" +
              (index === i ? " active" : "") +
              (index > i ? " done" : "")
            }
            aria-current={index === i ? "step" : undefined}
            disabled={busy || locked || i > index || (i === 3 && index < 3)}
            onClick={() => m.goStep(i)}
          >
            <span className="creation-step-icon">
              {index > i ? <Check size={17} /> : <Icon size={17} />}
            </span>
            <span>
              <strong>{label}</strong>
              <small>{note}</small>
            </span>
            <em>{String(i + 1).padStart(2, "0")}</em>
            {i < 3 && (
              <ChevronRight className="creation-step-arrow" size={14} />
            )}
          </button>
        ))}
      </nav>
      {m.error && (
        <InlineFeedback role="alert">
          {m.error}
          {m.feedback?.type === "configuration" && (
            <Button variant="ghost" asChild>
              <a href={m.configHref("模型用途")}>去配置中心</a>
            </Button>
          )}
        </InlineFeedback>
      )}
      {Object.keys(m.fieldErrors).length > 0 && (
        <InlineFeedback role="alert">
          <ul>
            {Object.values(m.fieldErrors).map((text) => (
              <li key={text}>{text}</li>
            ))}
          </ul>
        </InlineFeedback>
      )}
      {pending && (
        <WorkspaceInfo>
          <p>原激活结果尚未确认，当前资料和请求身份已保留。请先核对原提交。</p>
          <JsonDetails value={pending.value} title="原激活提交" />
          {pending.rejection ? (
            <Button
              variant="outline"
              disabled={busy}
              onClick={() => c.reanalyze()}
            >
              保留输入并重新整理
            </Button>
          ) : (
            <Button
              variant="outline"
              disabled={busy}
              onClick={() => void c.activate(true)}
            >
              查询或重试原激活
            </Button>
          )}
        </WorkspaceInfo>
      )}
      <div className="creation-layout">
        <div className="creation-main">
          {index === 0 && (
            <>
              <Segment
                label="生活世界来源"
                value={worldTab}
                onChange={setWorldTab}
                items={[
                  ["existing", "已有角色的世界"],
                  ["resource", "已发布的世界底稿"],
                  ["daily", "日常生活世界"],
                ]}
              />
              {worldTab === "daily" ? (
                <Surface
                  className={
                    "creation-daily" +
                    (d.useDefaultWorld && d.worldChoiceConfirmed
                      ? " selected"
                      : "")
                  }
                >
                  <WorldArt kind="city" />
                  <div>
                    <Badge variant="outline">轻松开始</Badge>
                    <h2>日常生活世界</h2>
                    <p>
                      从日常生活配置出发，确认创建后为角色建立独立的世界。之后可以继续完善背景、规则和地点。
                    </p>
                    <Button
                      onClick={() => m.chooseWorld(null)}
                      disabled={busy || locked}
                    >
                      {d.useDefaultWorld && d.worldChoiceConfirmed ? (
                        <Check size={15} />
                      ) : (
                        <Plus size={15} />
                      )}
                      使用日常生活世界
                    </Button>
                  </div>
                </Surface>
              ) : (
                <>
                  <div className="creation-world-toolbar">
                    <span>
                      {worldTab === "existing"
                        ? "复用配置，开启独立的生活"
                        : "采用明确的已发布版本"}
                    </span>
                    <div className="creation-search">
                      <Search size={15} />
                      <FieldInput
                        aria-label="搜索世界名字"
                        type="search"
                        value={search}
                        onChange={(e) => setSearch(e.target.value)}
                        placeholder="搜索世界名字"
                      />
                    </div>
                  </div>
                  {worldTab === "existing" && m.existingWorldLoading && (
                    <LoadingState label="正在读取已有世界…" />
                  )}
                  {worldTab === "resource" && m.sourceLoading && (
                    <LoadingState label="正在读取已发布的世界底稿…" />
                  )}
                  {(worldTab === "existing"
                    ? m.existingWorldError
                    : m.sourceError) && (
                    <InlineFeedback role="alert">
                      {worldTab === "existing"
                        ? m.existingWorldError
                        : m.sourceError}
                      <Button
                        variant="outline"
                        disabled={m.existingWorldLoading}
                        onClick={() =>
                          void (worldTab === "existing"
                            ? m.loadExistingWorlds(true)
                            : m.retrySources("world"))
                        }
                      >
                        重新读取世界
                      </Button>
                    </InlineFeedback>
                  )}
                  {worldTab === "existing" &&
                    !m.existingWorldLoading &&
                    m.worldsConfirmed &&
                    !m.existingWorlds.length && (
                      <WorkspaceInfo>
                        已确认没有已有生活世界，可以选择已发布底稿、日常生活世界，或先创建世界设定。
                      </WorkspaceInfo>
                    )}
                  <div className="creation-world-grid">
                    {worldRows.map((row) => {
                      const existing = worldTab === "existing",
                        selected = existing
                          ? d.worldSource?.worldId === row.id &&
                            d.worldSource?.fluctlightId === row.fluctlightId
                          : d.worldResourceId === row.id &&
                            d.worldResourceVersion === row.published_version;
                      return (
                        <button
                          type="button"
                          className={
                            "creation-world-card" +
                            (selected ? " selected" : "")
                          }
                          aria-pressed={selected}
                          key={(row.fluctlightId || "resource") + ":" + row.id}
                          disabled={
                            busy || locked || (!existing && !row.publication)
                          }
                          onClick={() =>
                            existing
                              ? m.chooseExistingWorld(row)
                              : m.chooseWorld(row)
                          }
                        >
                          <div>
                            <WorldArt
                              kind={
                                /森林|forest/i.test(row.name)
                                  ? "forest"
                                  : "city"
                              }
                            />
                            <span className="creation-select-mark">
                              {selected && <Check size={12} />}
                            </span>
                            <Badge variant="outline">
                              {existing ? "配置来源" : "世界底稿"}
                            </Badge>
                          </div>
                          <section>
                            <h3>{row.name}</h3>
                            <p>
                              {row.background ||
                                row.description ||
                                row.publication?.body?.background ||
                                row.publication?.body?.description ||
                                row.publicationError ||
                                "选择这份已读配置，核对后为新角色建立独立世界。"}
                            </p>
                            <footer>
                              <span>
                                {existing
                                  ? "来自 " + row.characterName
                                  : row.published_version + " · 已发布"}
                              </span>
                              <span>{existing ? "独立复用" : "固定版本"}</span>
                            </footer>
                          </section>
                        </button>
                      );
                    })}
                    <a
                      className="creation-new-world"
                      href="#/studio/edit/new?kind=world"
                    >
                      <Plus size={24} />
                      <strong>创建新的世界底稿</strong>
                      <p>
                        定义背景、规则与地点
                        <br />
                        保存发布后，带它回到这里。
                      </p>
                      <span>
                        去创作中心
                        <ArrowRight size={12} />
                      </span>
                    </a>
                  </div>
                  {worldTab === "resource" && m.worldCursor && (
                    <Button
                      variant="outline"
                      onClick={() => void m.loadSources("world", true)}
                    >
                      读取更多世界
                    </Button>
                  )}
                </>
              )}
              {d.worldChoiceConfirmed && (
                <WorkspaceInfo>
                  {d.worldSource
                    ? `将复用「${d.worldSource.name}」版本 ${d.worldSource.revision} 的背景、规则和地点；人物、物品实例和生活历史分别建立。`
                    : d.worldResourceId
                      ? `本次固定使用「${worldName} · ${d.worldResourceVersion}」。之后的底稿修改由你明确重新选择。`
                      : "已明确选择日常生活配置。世界创建与绑定将在原角色的准备流程中读回。"}
                </WorkspaceInfo>
              )}
              {m.selectedPublicationError && (
                <InlineFeedback role="alert">
                  {m.selectedPublicationError}
                  <Button variant="outline" onClick={m.readSelectedPublication}>
                    重新读取所选世界版本
                  </Button>
                </InlineFeedback>
              )}
              {m.selectedPublicationLoading && (
                <LoadingState label="正在读取所选世界的精确发布版本…" />
              )}
              <Button
                variant="ghost"
                disabled={m.existingWorldLoading}
                onClick={() => void m.loadExistingWorlds(true)}
              >
                <RefreshCw size={14} />
                重新读取已有世界
              </Button>
            </>
          )}
          {index === 1 && (
            <div className="creation-stack">
              {d.rejectedPreviewText !== undefined && (
                <Surface className="creation-stack">
                  <h2>保留的预览副本</h2>
                  <p className="creation-muted">
                    上次创建未被接受，预览中的修改保留在这里。可以修正或导出，使用副本时会重新整理与审阅。
                  </p>
                  <Textarea
                    className="creation-json"
                    aria-label="保留的预览副本 JSON"
                    rows={10}
                    disabled={busy || locked}
                    value={d.rejectedPreviewText}
                    onChange={(event) => {
                      d.rejectedPreviewText = event.target.value;
                      c.persist();
                    }}
                  />
                  <div className="creation-action-row">
                    <Button
                      variant="outline"
                      disabled={busy || locked}
                      onClick={() => {
                        c.changeMode("json");
                        d.json = d.rejectedPreviewText;
                        c.persist();
                      }}
                    >
                      使用副本作为 JSON 输入
                    </Button>
                    <Button
                      variant="ghost"
                      onClick={() =>
                        downloadText(
                          d.rejectedPreviewText,
                          "character-preview-copy.json",
                        )
                      }
                    >
                      导出保留的副本
                    </Button>
                  </div>
                </Surface>
              )}
              <div
                className="creation-source-grid"
                role="group"
                aria-label="创建方式"
              >
                {sources.map(([value, label, note, Icon]) => (
                  <button
                    type="button"
                    key={value}
                    disabled={busy || locked}
                    aria-pressed={d.mode === value}
                    className={
                      "creation-source" + (d.mode === value ? " selected" : "")
                    }
                    onClick={() => c.changeMode(value)}
                  >
                    <Icon size={20} />
                    <strong>{label}</strong>
                    <small>{note}</small>
                    {d.mode === value && <Check size={12} />}
                  </button>
                ))}
              </div>
              <Surface className="creation-input-panel">
                <fieldset disabled={busy || locked} className="creation-stack">
                  <div className="creation-form-grid">
                    <Label htmlFor="create-name">
                      {d.mode === "blank" ? "角色名字 *" : "角色名字（可选）"}
                      <FieldInput
                        id="create-name"
                        aria-label="角色名字"
                        data-testid="wizard-name"
                        maxLength={200}
                        value={d.name}
                        onChange={(e) => edit("name", e.target.value)}
                        placeholder="你想怎么称呼对方？"
                      />
                      <small>
                        {d.mode === "blank"
                          ? "空白角色需要一个名字。"
                          : "填写后将作为这次创建的名字。"}
                      </small>
                    </Label>
                    <Label htmlFor="create-timezone">
                      生活时区 *
                      <Combobox
                        id="create-timezone"
                        label="生活时区"
                        value={d.timezone}
                        disabled={busy || locked}
                        allowCustom
                        options={[
                          "Asia/Shanghai",
                          "Asia/Hong_Kong",
                          "Asia/Tokyo",
                          "Asia/Singapore",
                          "Europe/London",
                          "America/New_York",
                          "UTC",
                          ...(Intl.supportedValuesOf?.("timeZone") || []),
                        ]}
                        onChange={(value) => edit("timezone", value)}
                      />
                      <small>日程与生活节奏都以这个时区为准。</small>
                    </Label>
                    <Label className="full-field">
                      角色分组
                      <ChoiceSelect
                        aria-label="角色分组"
                        disabled={m.groupsLoading}
                        value={d.groupId || ""}
                        onChange={(e) => {
                          d.groupId = e.target.value;
                          c.persist();
                        }}
                      >
                        <option value="">未分组</option>
                        {d.groupId &&
                          !m.groups.some((g) => g.id === d.groupId) && (
                            <option value={d.groupId}>
                              原选择（待读取核对）
                            </option>
                          )}
                        {m.groups.map((group) => (
                          <option key={group.id} value={group.id}>
                            {group.name}
                          </option>
                        ))}
                      </ChoiceSelect>
                    </Label>
                    {m.groupsLoading && (
                      <p className="creation-muted full-field" role="status">
                        正在读取角色分组…
                      </p>
                    )}
                    {m.groupsError && (
                      <InlineFeedback role="alert" className="full-field">
                        {m.groupsError}
                        <Button
                          type="button"
                          variant="outline"
                          onClick={() => void m.loadGroups()}
                        >
                          重新读取分组
                        </Button>
                      </InlineFeedback>
                    )}
                  </div>
                  <div className="creation-divider" />
                  {d.mode === "blank" && (
                    <WorkspaceInfo>
                      <strong>把未知，留给以后的相处。</strong>
                      <p>
                        这条路径跳过分析，只确认名字与时区。更多认识会在后续生活中逐渐形成。
                      </p>
                    </WorkspaceInfo>
                  )}
                  {d.mode === "description" && (
                    <>
                      <div className="creation-section-heading">
                        <h2>说说这个人</h2>
                      </div>
                      <Textarea
                        aria-label="角色描述"
                        data-testid="wizard-description"
                        rows={8}
                        value={d.bio}
                        onChange={(e) => edit("bio", e.target.value)}
                        placeholder="TA 是谁，在哪里生活？有什么经历、性格、习惯，或者不愿被触碰的边界？"
                      />
                      <div className="creation-field-foot">
                        <span>
                          可以写身份、性格、经历、生活偏好与相处方式。
                        </span>
                        <span>{[...d.bio].length} 字</span>
                      </div>
                      <Disclosure title="还有一些相处期待？">
                        <div className="creation-form-grid">
                          <Label>
                            性格期待
                            <FieldInput
                              aria-label="性格期待"
                              value={d.traits || ""}
                              onChange={(e) => edit("traits", e.target.value)}
                            />
                          </Label>
                          <Label>
                            说话方式
                            <FieldInput
                              aria-label="说话方式"
                              value={d.speakingStyle || ""}
                              onChange={(e) =>
                                edit("speakingStyle", e.target.value)
                              }
                            />
                          </Label>
                          <Label className="full-field">
                            核心价值观
                            <FieldInput
                              aria-label="核心价值观期望"
                              value={d.coreValues || ""}
                              onChange={(e) =>
                                edit("coreValues", e.target.value)
                              }
                            />
                          </Label>
                        </div>
                      </Disclosure>
                    </>
                  )}
                  {d.mode === "json" && (
                    <>
                      <div className="creation-section-heading">
                        <h2>结构化的角色设定</h2>
                        <Button variant="ghost" size="sm" asChild>
                          <a
                            href="/templates/yaoguang-character-init-template.v2.json"
                            download
                          >
                            下载模板
                          </a>
                        </Button>
                        <Label>
                          选择文件
                          <FieldInput
                            aria-label="选择 JSON 文件"
                            type="file"
                            accept="application/json,.json"
                            onChange={(e) =>
                              void c.importFile(e.target.files?.[0])
                            }
                          />
                        </Label>
                      </div>
                      <Textarea
                        className="creation-json"
                        aria-label="创建 JSON"
                        data-testid="wizard-json"
                        rows={12}
                        spellCheck={false}
                        value={d.json}
                        onChange={(e) => edit("json", e.target.value)}
                      />
                      <div className="creation-field-foot">
                        <span>{d.fileName || "也可以直接粘贴并修正 JSON"}</span>
                        <span>
                          {new TextEncoder()
                            .encode(d.json)
                            .length.toLocaleString()}{" "}
                          / 60,000 字节
                        </span>
                      </div>
                      <p className="creation-muted">
                        填写已知的人物资料，保留模板结构；不要导入聊天存档或分析凭证。
                      </p>
                    </>
                  )}
                  {d.mode === "template" && (
                    <>
                      <div className="creation-section-heading">
                        <h2>已发布的角色模板</h2>
                        <Button variant="ghost" size="sm" asChild>
                          <a href="#/studio/edit/new?kind=character_template">
                            <Plus size={14} />
                            创建模板
                          </a>
                        </Button>
                      </div>
                      <div className="creation-template-list">
                        {m.templates.map((t) => (
                          <button
                            type="button"
                            className={d.templateId === t.id ? "selected" : ""}
                            aria-pressed={d.templateId === t.id}
                            key={t.id}
                            disabled={!t.publication}
                            onClick={() =>
                              m.templateChanged({ target: { value: t.id } })
                            }
                          >
                            <span className="creation-template-icon">
                              <BookOpen size={19} />
                            </span>
                            <span>
                              <strong>{t.name}</strong>
                              <small>
                                {t.publication?.body?.description ||
                                  t.publicationError ||
                                  "已发布的角色底稿"}
                              </small>
                            </span>
                            <Badge variant="outline">
                              {t.published_version}
                            </Badge>
                            {d.templateId === t.id && <Check size={14} />}
                          </button>
                        ))}
                      </div>
                      {d.templateId && (
                        <WorkspaceInfo>
                          使用精确发布版本 {d.templateVersion}
                        </WorkspaceInfo>
                      )}
                      {m.templateError && (
                        <InlineFeedback role="alert">
                          {m.templateError}
                          <Button
                            variant="outline"
                            onClick={() =>
                              void m.retrySources("character_template")
                            }
                          >
                            重新读取模板
                          </Button>
                        </InlineFeedback>
                      )}
                      {m.templateLoading && (
                        <LoadingState label="正在读取已发布的角色模板…" />
                      )}
                      {!m.templates.length &&
                        !m.templateError &&
                        !m.templateLoading && (
                          <p className="creation-muted">
                            还没有读到已发布模板，可重新读取或选择其他方式。
                          </p>
                        )}
                      {m.templateCursor && (
                        <Button
                          variant="outline"
                          onClick={() =>
                            void m.loadSources("character_template", true)
                          }
                        >
                          读取更多模板
                        </Button>
                      )}
                    </>
                  )}
                </fieldset>
              </Surface>
              {m.configured === false && d.mode === "description" && (
                <WorkspaceInfo>
                  尚未配置通用模型。请连接模型服务后整理资料。
                  <Button variant="outline" asChild>
                    <a href={m.configHref("模型用途")}>先连接模型</a>
                  </Button>
                </WorkspaceInfo>
              )}
              {step === "analysis" && (
                <Surface className="creation-analyzing">
                  <LoaderCircle className="animate-spin" size={24} />
                  <div>
                    <strong>正在整理这次的角色设定…</strong>
                    <p>
                      原输入保留，已等待{" "}
                      {Math.max(
                        0,
                        Math.floor(
                          (m.analysisNow -
                            (m.analysisStartedAt || m.analysisNow)) /
                            1000,
                        ),
                      )}{" "}
                      秒。
                    </p>
                  </div>
                  <Button variant="ghost" onClick={() => c.cancelAnalysis()}>
                    停止本次等待
                  </Button>
                </Surface>
              )}
            </div>
          )}
          {index === 2 &&
            (d.mode === "blank" ? (
              <Surface className="creation-blank-preview">
                <span>
                  <UserRound size={30} />
                </span>
                <h2>{d.name}</h2>
                <p>一位等待慢慢认识的新朋友</p>
                <div>
                  <KeyValue label="生活时区">{d.timezone}</KeyValue>
                  <KeyValue label="生活世界">{worldName}</KeyValue>
                  <KeyValue label="角色分组">{groupName}</KeyValue>
                </div>
                <WorkspaceInfo>
                  空白初始化不携带分析来源或完整人物设定。名字与时区确认后，分组和世界按各自准备流程完成。
                </WorkspaceInfo>
              </Surface>
            ) : (
              <Surface className="creation-review">
                <Segment
                  label="资料预览主题"
                  value={reviewTab}
                  onChange={setReviewTab}
                  items={sections}
                />
                {definitionError && (
                  <InlineFeedback role="alert">
                    {definitionError}
                  </InlineFeedback>
                )}
                {reviewTab === "identity" && (
                  <>
                    <div className="creation-section-intro">
                      <h2>身份与背景</h2>
                      <p>先认识是谁，再了解一路走来的经历。</p>
                    </div>
                    <div className="creation-form-grid">
                      {fields
                        .filter((f) => f.path[0] === "identity")
                        .sort((a,b)=>(identityOrder.indexOf(a.path[1])<0?identityOrder.length:identityOrder.indexOf(a.path[1]))-(identityOrder.indexOf(b.path[1])<0?identityOrder.length:identityOrder.indexOf(b.path[1])))
                        .map((f) => (
                          <ProfileField
                            key={JSON.stringify(f.path)}
                            field={f}
                            disabled={busy || locked}
                          />
                        ))}
                    </div>
                  </>
                )}
                {reviewTab === "personality" && (
                  <>
                    <div className="creation-section-intro">
                      <h2>共同性格</h2>
                      <p>
                        数值在 0–1 之间。留白表示尚未提供，明确设定后才会保存。
                      </p>
                    </div>
                    <div className="creation-traits-grid">
                      {Object.entries(traits).map(([key, label]) => {
                        const value = definition?.core_persona?.personality?.[key];
                        return (
                          <div className="creation-trait" key={key}>
                            <div>
                              <strong>{label}</strong>
                              {typeof value === "number" ? (
                                <span>{value.toFixed(2)}</span>
                              ) : (
                                <Button
                                  type="button"
                                  variant="ghost"
                                  disabled={
                                    busy ||
                                    locked ||
                                    !!definitionError ||
                                    (value !== undefined && value !== null)
                                  }
                                  onClick={() =>
                                    applyDefinition(
                                      setPath(
                                        copy(definition),
                                        ["core_persona", "personality", key],
                                        0.5,
                                      ),
                                    )
                                  }
                                >
                                  待补充 · 设定
                                </Button>
                              )}
                            </div>
                            <Slider
                              aria-label={label}
                              min={0}
                              max={1}
                              step={0.01}
                              value={[typeof value === "number" ? value : 0]}
                              disabled={
                                busy ||
                                locked ||
                                typeof value !== "number" ||
                                !!definitionError ||
                                d.previewJsonText !== undefined
                              }
                              onValueChange={([v]) => {
                                applyDefinition(setPath(copy(definition),['core_persona','personality',key],v));
                              }}
                            />
                          </div>
                        );
                      })}
                    </div>
                    <div className="creation-divider" />
                    <div className="creation-section-intro">
                      <h2>表达与相处</h2>
                      <p>保留已提供的表达方式、数值与未知扩展。</p>
                    </div>
                    <div className="creation-form-grid">
                      {fields
                        .filter(
                          (f) =>
                            f.path[0] === "behavioral_policy" ||
                            (f.path[0] === "personality" &&
                              f.path.length !== 2) ||
                            (f.path[0] === "personality" &&
                              !Object.hasOwn(traits, f.path.at(-1))),
                        )
                        .map((f) => (
                          <ProfileField
                            key={JSON.stringify(f.path)}
                            field={f}
                            disabled={busy || locked}
                          />
                        ))}
                    </div>
                  </>
                )}
                {reviewTab === "life" && (
                  <div className="creation-stack">
                    <div className="creation-section-intro">
                      <h2>生活有自己的纹理</h2>
                      <p>偏好、日常约定和初始衣物，保留各自完整结构。</p>
                    </div>
                    {Object.entries(lifeFields).map(([key, label]) =>
                      renderStructure(
                        label,
                        ["core_persona", "life_profile", key],
                        definition?.core_persona?.life_profile?.[key],
                      ),
                    )}
                    {definition?.core_persona?.life_profile &&
                      Object.keys(definition.core_persona.life_profile)
                        .filter((k) => !Object.hasOwn(lifeFields, k))
                        .map((k) =>
                          renderStructure(
                            k,
                            ["core_persona", "life_profile", k],
                            definition.core_persona.life_profile[k],
                          ),
                        )}
                  </div>
                )}
                {reviewTab === "relationships" && (
                  <div className="creation-stack">
                    <div className="creation-section-intro">
                      <h2>人格、关系与最初的动机</h2>
                      <p>初始目标与意图的采用选项不会删除初始关系。</p>
                    </div>
                    {renderStructure(
                      "人格档案与切换规则",
                      ["core_persona", "personality_system"],
                      definition?.core_persona?.personality_system,
                    )}
                    {[
                      ["initial_relationships", "初始关系"],
                      ["initial_goals", "初始目标"],
                      ["initial_intentions", "初始意图"],
                    ].map(([key, label]) =>
                      renderStructure(label, [key], definition?.[key]),
                    )}
                    <label className="creation-adopt">
                      <Checkbox
                        aria-label="采用已预览的初始目标和意图"
                        checked={d.includeGoals !== false}
                        disabled={busy || locked}
                        onCheckedChange={(v) => {
                          d.includeGoals = v === true;
                          c.persist();
                        }}
                      />
                      <span>
                        <strong>采用已预览的初始目标和意图</strong>
                        <small>
                          关闭时仅不采用目标和意图，初始关系仍然保留。
                        </small>
                      </span>
                    </label>
                    {definition?.actor_user &&
                      renderStructure(
                        "关于你",
                        ["actor_user"],
                        definition.actor_user,
                      )}
                  </div>
                )}
                {reviewTab === "json" && (
                  <div className="creation-stack">
                    <WorkspaceInfo>
                      这是本次预览的完整嵌套设定。未知扩展保留，分析来源由原记录维护。明确应用后同步各主题。
                    </WorkspaceInfo>
                    <Label>
                      完整设定 JSON
                      <Textarea
                        className="creation-json"
                        aria-label="完整设定 JSON"
                        rows={16}
                        spellCheck={false}
                        disabled={busy || locked || !!definitionError}
                        value={
                          d.previewJsonText ??
                          JSON.stringify(definition, null, 2)
                        }
                        onChange={(e) => {
                          if (d.previewJsonText === undefined)
                            d.previewJsonBase = JSON.stringify(definition);
                          d.previewJsonText = e.target.value;
                          c.persist();
                        }}
                      />
                    </Label>
                    <div className="creation-action-row">
                      <Button
                        variant="outline"
                        disabled={
                          busy ||
                          locked ||
                          d.previewJsonText === undefined ||
                          !!definitionError
                        }
                        onClick={() => {
                          try {
                            if (
                              d.previewJsonBase !== JSON.stringify(definition)
                            )
                              throw Error(
                                "其他分组已有新修改，请保留本次 JSON，并还原为当前设定后重新核对。",
                              );
                            applyDefinition(JSON.parse(d.previewJsonText), {
                              consumeJson: true,
                            });
                          } catch (cause) {
                            m.showError(cause.message);
                          }
                        }}
                      >
                        应用 JSON 修改
                      </Button>
                      <Button
                        variant="ghost"
                        disabled={busy || locked}
                        onClick={() => {
                          delete d.previewJsonText;
                          delete d.previewJsonBase;
                          c.persist();
                        }}
                      >
                        还原为当前设定
                      </Button>
                      <Button
                        variant="ghost"
                        disabled={!definition || !!definitionError}
                        onClick={() =>
                          download(definition, "character-preview.json")
                        }
                      >
                        导出本次预览
                      </Button>
                    </div>
                    <JsonDetails
                      value={
                        d.analysis && {
                          analysis_id: d.analysis.analysis_id,
                          correlation_id: d.analysis.correlation_id,
                          schema_version: d.analysis.schema_version,
                        }
                      }
                      title="原分析来源"
                    />
                  </div>
                )}
              </Surface>
            ))}
          {index === 3 && (
            <>
              {m.recoveryPending?.map((entry) => (
                <WorkspaceInfo key={entry.key}>
                  <p>
                    原「{entry.title}
                    」恢复提交仍待核对，当前准备状态与原提交结果分别保留。
                  </p>
                  {entry.error ? (
                    <InlineFeedback role="alert">{entry.error}</InlineFeedback>
                  ) : (
                    <>
                      <JsonDetails
                        value={entry.original.value}
                        title="原恢复提交"
                      />
                      <Button
                        variant="outline"
                        disabled={busy}
                        onClick={(event) => {
                          recoveryTrigger.current = event.currentTarget;
                          void (entry.original.rejection
                            ? c.reviewRejectedRecovery(entry)
                            : c.recoverStage(entry));
                        }}
                      >
                        {entry.original.rejection
                          ? "核对已拒绝的恢复提交"
                          : "核对原恢复提交"}
                      </Button>
                      {entry.original.rejection && (
                        <p>
                          原恢复因版本冲突明确被拒绝，需要核对最新版本后继续。
                        </p>
                      )}
                    </>
                  )}
                </WorkspaceInfo>
              ))}
              <Surface className="creation-preparation">
                <div className="creation-progress-heading">
                  <div>
                    <span className="replica-eyebrow">PREPARING A LIFE</span>
                    <h2>
                      {m.progress?.complete
                        ? `${name}的故事，从这里开始。`
                        : `为${name}准备第一天`}
                    </h2>
                    <p>
                      {m.progress?.complete
                        ? "每一项准备均已读回确认。"
                        : `${done} / 7 项已确认完成`}
                    </p>
                  </div>
                  <span className="creation-progress-number">
                    {done}
                    <small> / 7</small>
                  </span>
                </div>
                {m.progress ? (
                  <PreparationSteps
                    stages={m.progress.stages}
                    busy={busy}
                    onRecover={c.recoverStage}
                    replica
                  />
                ) : pending && !busy ? (
                  <WorkspaceInfo>
                    原创建结果尚未确认。请在上方核对原提交，原身份与输入保持。
                  </WorkspaceInfo>
                ) : (
                  <LoadingState
                    label={
                      pending ? "正在核对原创建结果…" : "正在读取实际准备进度…"
                    }
                  />
                )}
                {d.worldSource && !worldBindingConfirmed && (
                  <WorkspaceInfo>
                    <p>
                      角色已建立。世界配置复用仍需核对，原角色与原请求保留。
                    </p>
                    {m.worldSourceReview ? (
                      <>
                        <SemanticTree value={m.worldSourceReview.preview} />
                        <Button
                          variant="outline"
                          disabled={busy}
                          onClick={() => void c.confirmWorldSourceReview()}
                        >
                          确认新版配置并继续复用
                        </Button>
                      </>
                    ) : (
                      <Button
                        variant="outline"
                        disabled={busy}
                        onClick={() => void c.reviewWorldSource()}
                      >
                        核对已有世界的最新配置
                      </Button>
                    )}
                  </WorkspaceInfo>
                )}
                {d.activatedId && worldBindingConfirmed && (
                  <div className="creation-meet">
                    <div>
                      <strong>
                        {m.progress?.complete
                          ? "这一次，换你说你好。"
                          : "已经可以开始相处了。"}
                      </strong>
                      <p>
                        {m.progress?.complete
                          ? "你的第一句话，会成为这个故事的一部分。"
                          : "剩余的生活与视觉准备会保留实际进度。"}
                      </p>
                    </div>
                    <Button size="lg" asChild>
                      <a
                        href={
                          "#/c/" + encodeURIComponent(d.activatedId) + "/chat"
                        }
                      >
                        {m.progress?.complete ? "去见" + name : "先开始相处"}
                        <ArrowRight size={15} />
                      </a>
                    </Button>
                  </div>
                )}
              </Surface>
              <div className="creation-progress-actions">
                <Button
                  variant="outline"
                  onClick={(e) => {
                    snapshotTrigger.current = e.currentTarget;
                    setSnapshotOpen(true);
                  }}
                >
                  查看创建资料
                </Button>
                <Button
                  variant="outline"
                  disabled={busy || !d.activatedId}
                  onClick={() => void c.refreshProgress()}
                >
                  重新读取进度
                </Button>
                {d.activatedId && (
                  <Button variant="ghost" asChild>
                    <a
                      href={
                        "#/c/" + encodeURIComponent(d.activatedId) + "/profile"
                      }
                    >
                      角色详情
                    </a>
                  </Button>
                )}
                <Button variant="ghost" asChild>
                  <a href="#/library">保留进度，返回角色库</a>
                </Button>
                {d.activatedId && !c.archived && !pending && (
                  <Button
                    variant="ghost"
                    disabled={busy}
                    onClick={(e) => {
                      resetTrigger.current = e.currentTarget;
                      setResetOpen(true);
                    }}
                  >
                    开启另一段相识
                  </Button>
                )}
              </div>
            </>
          )}
        </div>
        <aside className="creation-aside">
          <Surface>
            <div className="replica-eyebrow">YOUR NEW CHAPTER</div>
            {index >= 2 && d.mode !== "blank" ? (
              <div className="creation-portrait">
                <WorkspaceCharacterSketch />
                <span>形象示意 · 非实际形象</span>
              </div>
            ) : d.worldChoiceConfirmed ? (
              <div className="creation-aside-world">
                <WorldArt
                  kind={/森林|forest/i.test(worldName) ? "forest" : "city"}
                />
              </div>
            ) : (
              <div className="creation-aside-empty">
                <Globe2 size={38} />
              </div>
            )}
            <h3>
              {index >= 2
                ? name
                : worldName === "待选择"
                  ? "故事，正在等待开始"
                  : worldName}
            </h3>
            <p>
              {index >= 2
                ? fields.find((f) => f.path.join(".") === "identity.occupation")
                    ?.value || "一位等待慢慢认识的新朋友"
                : "给日常一个可以发生的地方。"}
            </p>
            <div className="creation-divider" />
            <KeyValue label="生活世界">{worldName}</KeyValue>
            <KeyValue label="认识方式">
              {sources.find((s) => s[0] === d.mode)?.[1]}
            </KeyValue>
            {index >= 1 && <KeyValue label="生活时区">{d.timezone}</KeyValue>}
            {index >= 2 && <KeyValue label="角色分组">{groupName}</KeyValue>}
            <div className="creation-aside-note">
              <Sprout size={23} />
              每一个细节，
              <br />
              都可以成为日后相处的回声。
            </div>
          </Surface>
          <div className="creation-local-note">
            <ShieldCheck size={13} />
            <span>草稿与原请求按当前账号保留</span>
          </div>
        </aside>
      </div>
      {index < 3 && (
        <footer className="creation-footer">
          <div>
            <span className="creation-saved-notice">
              <ShieldCheck size={13} />
              草稿自动保留
            </span>
            <small>
              {index === 2
                ? "确认后将创建角色，并开始准备生活。"
                : "返回与切换页面，保留这次输入。"}
            </small>
          </div>
          <div className="creation-action-row">
            {index > 0 && (
              <Button
                variant="outline"
                disabled={busy || locked}
                onClick={back}
              >
                <ArrowLeft size={15} />
                上一步
              </Button>
            )}
            <Button
              size="lg"
              disabled={
                busy ||
                locked ||
                !m.canContinue ||
                (index === 1 &&
                  m.configured === false &&
                  d.mode === "description") ||
                (index === 2 && d.previewJsonText !== undefined)
              }
              data-testid={
                index === 0
                  ? "wizard-world-next"
                  : index === 1
                    ? "wizard-analyze"
                    : "wizard-activate"
              }
              onClick={next}
            >
              {busy && <LoaderCircle className="animate-spin" size={15} />}{" "}
              {index === 0
                ? "继续，认识角色"
                : index === 1
                  ? d.mode === "blank"
                    ? "预览空白角色"
                    : "整理并预览"
                  : "确认，准备相遇"}
              <ArrowRight size={15} />
            </Button>
          </div>
        </footer>
      )}
      <WorkspaceDialog
        open={!!m.recoveryReview}
        onOpenChange={(open) => {
          if (!open) c.recoveryReview.value = null;
        }}
        title="核对最新准备状态"
        description="原恢复已经明确拒绝。解除旧记录后，按最新准备状态决定是否再次恢复。"
        className="replica-creation"
        returnFocus={() => recoveryTrigger.current}
        footer={
          <>
            <Button
              variant="outline"
              onClick={() => (c.recoveryReview.value = null)}
            >
              取消
            </Button>
            <Button
              disabled={busy}
              onClick={() => void c.confirmRejectedRecovery()}
            >
              已核对，解除被拒绝的恢复提交
            </Button>
          </>
        }
      >
        <SemanticTree value={m.recoveryReview?.actual} />
        <JsonDetails
          value={m.recoveryReview?.original}
          title="已拒绝的原恢复提交"
        />
      </WorkspaceDialog>
      <WorkspaceDialog
        open={snapshotOpen}
        onOpenChange={setSnapshotOpen}
        title="这次相遇的起点"
        description="查看已保存的原创建请求，返回不会再次创建角色。"
        wide
        className="replica-creation"
        returnFocus={() => snapshotTrigger.current}
        footer={
          (d.activationSnapshot || pending?.value) && (
            <Button
              variant="outline"
              onClick={() =>
                download(
                  d.activationSnapshot || pending.value,
                  "creation-snapshot.json",
                )
              }
            >
              导出原创建资料
            </Button>
          )
        }
      >
        {d.activationSnapshot || pending?.value ? (
          <div className="creation-stack">
            <SemanticTree value={d.activationSnapshot || pending.value} />
            <JsonDetails
              value={d.activationSnapshot || pending.value}
              title="原请求完整 JSON"
            />
          </div>
        ) : (
          <WorkspaceInfo>
            这条历史创建尚未保留原请求快照。可以查看现有角色资料和实际准备状态。
          </WorkspaceInfo>
        )}
      </WorkspaceDialog>
      <WorkspaceDialog
        open={resetOpen}
        onOpenChange={setResetOpen}
        title="开启另一段新的相识"
        description="已创建角色及其准备进度保留，接下来使用新的创建草稿。"
        className="replica-creation"
        returnFocus={() => resetTrigger.current}
        footer={
          <>
            <Button variant="outline" onClick={() => setResetOpen(false)}>
              取消
            </Button>
            <Button
              onClick={() => {
                m.newDraft();
                setResetOpen(false);
                setReviewTab("identity");
                setWorldTab("existing");
              }}
            >
              开始新的创建
            </Button>
          </>
        }
      >
        <p>可以稍后从原角色的初始化进度继续核对。</p>
      </WorkspaceDialog>
      <WorkspaceDialog
        open={!!structured}
        onOpenChange={(open) => {
          if (!open) setStructured(null);
        }}
        title={"编辑" + (structured?.label || "内容")}
        description="保留原数据类型、完整结构与未知扩展。"
        className="replica-creation"
        returnFocus={() => structuredTrigger.current}
        footer={
          <>
            <Button variant="outline" onClick={() => setStructured(null)}>
              取消
            </Button>
            <Button
              disabled={busy || locked}
              onClick={() => {
                try {
                  if (JSON.stringify(definition) !== structured.base)
                    throw Error("预览设定已变化，请保留输入并重新核对。");
                  const value = JSON.parse(structured.text);
                  if (
                    (structured.kind === "array" && !Array.isArray(value)) ||
                    (structured.kind === "object" &&
                      (value === null ||
                        typeof value !== "object" ||
                        Array.isArray(value))) ||
                    (structured.kind === "null" && value !== null) ||
                    (!["array", "object", "missing", "null"].includes(
                      structured.kind,
                    ) &&
                      typeof value !== structured.kind)
                  )
                    throw Error("请保留原数据类型。");
                  if (
                    applyDefinition(
                      setPath(copy(definition), structured.path, value),
                    )
                  )
                    setStructured(null);
                } catch (cause) {
                  setStructuredError(cause.message);
                }
              }}
            >
              应用修改
            </Button>
          </>
        }
      >
        <Textarea
          aria-label={"编辑" + (structured?.label || "内容")}
          className="creation-json"
          rows={13}
          spellCheck={false}
          value={structured?.text || ""}
          onChange={(e) =>
            setStructured({ ...structured, text: e.target.value })
          }
        />
        {structuredError && (
          <InlineFeedback role="alert">{structuredError}</InlineFeedback>
        )}
      </WorkspaceDialog>
    </div>
  );
}

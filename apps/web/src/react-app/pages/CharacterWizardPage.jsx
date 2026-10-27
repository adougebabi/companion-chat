import {Tabs,TabsList,TabsTrigger} from '@/components/ui/tabs';
import {Questionnaire,QuestionnaireItem,QuestionnaireTitle,QuestionnaireChoices,QuestionnaireChoice,QuestionnaireChoiceDescription} from '@/components/ui/questionnaire';
import {InlineFeedback as OperationStatus} from "@/react-app/shared/Presentation";
import {InlineFeedback,ReadState} from "@/react-app/shared/Presentation";
import {Surface as CardSurface} from "@/react-app/controls";
import {ArrowUpRightIcon as ExternalLinkIcon,MoreHorizontalIcon as MoreActionsIcon} from "lucide-react";
import WorkspaceBreadcrumb from '@/react-app/shared/WorkspaceBreadcrumb';
import ChoiceCards from '@/react-app/shared/ChoiceCards';
import {hasVisibleSemanticValue} from '@/react-app/shared/SemanticTree';
import { reusableWorlds } from "@/approved-ui/world-sources.mjs";
import { useController } from "@/react-app/page";
import { selector, readValue, elementRef } from "@/state/view-model";
import { cn } from "@/lib/utils";
import {
  ActionButton,
  Surface,
  FieldInput,
  ChoiceSelect,
  Textarea,
  Disclosure,
} from "@/react-app/controls";
import {
  displayText,
  eventModifiers,
  listValues,
  Fragment,
} from "@/react-app/render-helpers";
import { directory, activationFromAnalysis } from "@/v3/providers/real.ts";
import { list } from "@/v3/proto/store.js";
import { semanticLabel } from "@/v3/proto/components/semantic-fields.js";
import { errorMessage } from "@/v3/domain-presentation.mjs";
import { createCreationController } from "@/state/creation-controller.mjs";
import PreparationSteps from "@/react-app/shared/PreparationSteps";
import ProfileForm from "@/react-app/shared/ProfileForm";
import SemanticTree from "@/react-app/shared/SemanticTree";
import { migrateCreationRecovery } from "@/approved-ui/creation-recovery.mjs";
function createCharacterWizardPageController(__props, context, lifecycle) {
  const w = lifecycle.workspace,
    owner = w.state.session?.actorId,
    migrated = migrateCreationRecovery(owner, localStorage, sessionStorage),
    c = createCreationController({
      owner,
      client: w.client,
      directory,
      storage: localStorage,
      analysisToActivation: activationFromAnalysis,
      ownerCurrent: () => w.state.session?.actorId === owner,
      refreshDirectory: w.refreshDirectory,
      label: semanticLabel,
      query: new URLSearchParams(location.hash.split("?")[1] || ""),
      today: (zone) => w.day(new Date(), zone),
      cell: lifecycle.cell,
      model: lifecycle.model,
    });
  const entryUrl = new URL(location.href);
  const [entryPath, entrySearch = ""] = entryUrl.hash.split("?");
  const entryQuery = new URLSearchParams(entrySearch);
  if (entryQuery.has("new")) {
    entryQuery.delete("new");
    entryUrl.hash = entryPath + (entryQuery.size ? "?" + entryQuery : "");
    history.replaceState(history.state, "", entryUrl);
  }
  const {
      draft,
      step,
      busy,
      error,
      feedback,
      fieldErrors,
      progress,
      previewFields,
      pending,
      analysisStartedAt,
      worldSourceReview,
    } = c,
    existingWorlds = lifecycle.cell([]),
    worldChoice = lifecycle.cell(!!draft.worldSource || !!draft.worldResourceId || draft.step > 0 || !!draft.activatedId),
    worlds = lifecycle.cell([]),
    templates = lifecycle.cell([]),
    worldCursor = lifecycle.cell(""),
    templateCursor = lifecycle.cell(""),
    sourceError = lifecycle.cell(""),
    existingWorldError = lifecycle.cell(""),
    templateError = lifecycle.cell(""),
    existingWorldLoading = lifecycle.cell(false),
    configured = lifecycle.cell(null);
  const wizardRoot = lifecycle.cell(null);
  const analysisNow = lifecycle.cell(Date.now());
  lifecycle.observe(
    step,
    () => {
      const root = wizardRoot.value,
        heading = root?.querySelector("h2");
      if (heading) {
        heading.tabIndex = -1;
        heading.focus({
          preventScroll: true,
        });
        root.scrollIntoView({
          block: "start",
        });
      }
    },
    {
      flush: "post",
    },
  );
  let timer,
    active = true,
    reading = false;
  const modes = [
    ["blank", "空白创建", "从名字开始，其他资料可以在相处中逐渐补充。"],
    ["description", "一段描述", "由模型把身份、性格与生活整理成完整设定。"],
    ["json", "JSON 导入", "导入已有的人物资料，核对后再创建。"],
    ["template", "角色模板", "选择一份已发布版本，先解析与审阅。"],
  ];
  const groups = selector(() =>
    list("groups").filter((g) => g.id !== "ungrouped"),
  );
  const navSteps = ["生活世界", "认识角色", "资料预览", "准备相遇"],
    currentStep = selector(
      () =>
        ({
          world: 0,
          person: 1,
          analysis: 1,
          preview: 2,
          progress: 3,
          complete: 3,
        })[step.value] ?? 0,
    ),
    stateLabel = (s) =>
      ({
        ready: "已完成",
        running: "进行中",
        pending: "等待中",
        failed: "失败",
        unknown: "状态待确认",
      })[s] || "状态待确认";
  const configHref = (section) =>
    "#/settings?section=" + encodeURIComponent(section || "模型用途");
  let worldReadEpoch = 0;
  async function loadExistingWorlds() {
    const token = ++worldReadEpoch;
    const characters = w.state.characters.map(({id,name}) => ({id,name}));
    existingWorldLoading.value = true;
    try {
      const result = await reusableWorlds(w.client, characters);
      if (!active || !c.live() || token !== worldReadEpoch) return;
      existingWorlds.value = result.items;
      existingWorldError.value = result.unavailable ? `${result.unavailable} 个角色的世界暂未读到，已读到的世界仍可选择。` : "";
    } catch (cause) { if (active && c.live() && token === worldReadEpoch) existingWorldError.value = errorMessage(cause); }
    finally { if (active && c.live() && token === worldReadEpoch) existingWorldLoading.value = false; }
  }
  lifecycle.observe(() => w.state.characters.map(({id,name}) => [id,name]), () => void loadExistingWorlds(), {immediate:true});
  function chooseWorld(world){c.selectWorld(world);worldChoice.value=true;}
  function chooseExistingWorld(world){c.selectExistingWorld(world);worldChoice.value=true;}
  async function loadSources(kind, more = false) {
    try {
      const result = await w.client.listStudioResources({
        kind,
        published: true,
        cursor: more
          ? kind === "world"
            ? worldCursor.value
            : templateCursor.value
          : "",
      });
      if (!active || !c.live()) return;
      if (kind === "world") {
        worlds.value = more ? [...worlds.value, ...result.items] : result.items;
        worldCursor.value = result.next_cursor || "";
      } else {
        templates.value = more
          ? [...templates.value, ...result.items]
          : result.items;
        templateCursor.value = result.next_cursor || "";
      }
      (kind === "world" ? sourceError : templateError).value = "";
    } catch (cause) {
      if (active && c.live()) (kind === "world" ? sourceError : templateError).value = errorMessage(cause);
    }
  }
  function templateChanged(event) {
    const value = templates.value.find((t) => t.id === event.target.value);
    if (!value) return;
    draft.templateId = value.id;
    draft.templateVersion = value.published_version;
    c.invalidate();
  }
  function prev() {
    if (pending.value || draft.activatedId) return;
    if (step.value === "person") step.value = "world";
    else if (step.value === "preview") {
      step.value = "person";
    }
    c.persist();
  }
  async function poll() {
    analysisNow.value = Date.now();
    if (
      !active ||
      document.hidden ||
      reading ||
      busy.value ||
      !draft.activatedId ||
      progress.value?.complete
    )
      return;
    reading = true;
    try {
      await c.refreshProgress();
    } finally {
      reading = false;
    }
  }
  lifecycle.observe(previewFields, () => c.persistPreview(), {
    deep: true,
  });
  const visibility = () => {
    if (!document.hidden) void poll();
  };
  lifecycle.onStart(() => {
    void loadSources("world");
    void loadSources("character_template");
    void w.services.configurationClient
      .read()
      .then((value) => {
        if (c.live())
          configured.value = value.bindings.some(
            (b) => b.role === "generic_llm",
          );
      })
      .catch(() => {});
    if (draft.activatedId) void c.refreshProgress();
    timer = setInterval(() => void poll(), 5000);
    document.addEventListener("visibilitychange", visibility);
  });
  lifecycle.onDispose(() => {
    active = false;
    clearInterval(timer);
    document.removeEventListener("visibilitychange", visibility);
    c.dispose();
  });
  return (_ctx, _cache) => {
    const worldOptions=[...existingWorlds.value.map(world=>({value:'existing:'+world.fluctlightId+':'+world.id,label:world.name,description:'来自 '+world.characterName+' · 复用背景和地点，独立生活记录',select:()=>chooseExistingWorld(world)})),...worlds.value.map(world=>({value:'definition:'+world.id+':'+world.published_version,label:world.name,description:'已发布底稿 · 版本 '+world.published_version,select:()=>chooseWorld(world)})),{value:'default',label:'使用日常生活世界',description:'建立独立的生活世界，地点可以继续补充。',select:()=>chooseWorld(null)}];
    const worldValue=!worldChoice.value?'':draft.worldSource?'existing:'+draft.worldSource.fluctlightId+':'+draft.worldSource.worldId:draft.useDefaultWorld?'default':'definition:'+draft.worldResourceId+':'+draft.worldResourceVersion;

    return (
      <div ref={elementRef(wizardRoot)} className="character-workflow">
        <WorkspaceBreadcrumb parentLabel="角色库" parentHref="#/library" currentLabel={draft.name||"新的相识"}/>
      <Tabs value={'step-'+currentStep.value} aria-label="创建步骤"><TabsList className="h-auto group-data-horizontal/tabs:h-auto w-full flex-wrap justify-start gap-2">{navSteps.map((label,index)=><TabsTrigger key={label} value={'step-'+index} disabled={index!==currentStep.value} aria-current={index===currentStep.value?'step':undefined} className="min-h-11 flex-none gap-2"><span aria-hidden="true">{index<currentStep.value?'✓':index+1}</span>{label}</TabsTrigger>)}</TabsList></Tabs>
        {readValue(step) === "world" ? (
          <Questionnaire
            key={0}
            className="character-form rounded-xl border bg-card text-card-foreground shadow-sm"
            noValidate
            onSubmit={eventModifiers(
              ($event) => worldChoice.value && readValue(c).next(),
              ["prevent"],
            )}
          >
            <header>
              <small>{"第 1 / 4 步"}</small>
              <h2>{"生活世界"}</h2>
              <p className="muted">
                {
                  " 世界是生活的背景、地点和规则。先选一份底稿，再慢慢认识这个人。 "
                }
              </p>
            </header>
            <section className="character-stack">
              <h3>已有生活世界</h3>
              {existingWorldLoading.value && <ReadState role="status">正在读取已有世界…</ReadState>}
              {existingWorldError.value && <InlineFeedback  role="alert">{existingWorldError.value}</InlineFeedback>}
              {!existingWorldLoading.value && !existingWorldError.value && !existingWorlds.value.length && <p className="muted">还没有角色关联的生活世界，可以选择下面的已发布底稿，或创建新的世界设定。</p>}
              <p className="muted">只复用世界背景、规则和地点；人物、物品实例、事件模板、运行位置与生活历史不复制。</p>
              <QuestionnaireItem name="world-source" required disabled={readValue(busy)||!!readValue(pending)}><QuestionnaireTitle>生活世界来源</QuestionnaireTitle><QuestionnaireChoices className="sm:grid-cols-2">{worldOptions.map(option=><QuestionnaireChoice key={option.value} value={option.value} checked={worldValue===option.value} onChange={()=>option.select()}><span>{option.label}</span><QuestionnaireChoiceDescription>{option.description}</QuestionnaireChoiceDescription></QuestionnaireChoice>)}</QuestionnaireChoices></QuestionnaireItem>
              <ActionButton variant="ghost" type="button"  onClick={()=>void loadExistingWorlds()}>重新读取已有世界</ActionButton>
              {!worlds.value.length&&!sourceError.value&&<p className="muted">创作中心的世界草稿需要先发布，才会出现在可选底稿中。</p>}
              {worldCursor.value ? (
                <ActionButton variant="outline"
                  key={1}
                  type="button"

                  onClick={($event) => loadSources("world", true)}
                >
                  {"读取更多世界"}
                </ActionButton>
              ) : null}
              {sourceError.value ? (
                <InlineFeedback key={2} >
                  {displayText(sourceError.value) + " "}
                  <ActionButton variant="ghost"
                    type="button"

                    onClick={($event) => loadSources("world")}
                  >
                    {"重新读取"}
                  </ActionButton>
                </InlineFeedback>
              ) : null}
              <ActionButton variant="ghost" asChild >
                <a  href="#/studio/edit/new?kind=world">
                  {"创建世界设定 "}<ExternalLinkIcon className="inline size-3.5 shrink-0" aria-hidden="true"/>
                </a>
              </ActionButton>
            </section>
            {Object.keys(readValue(fieldErrors)).length ? (
              <InlineFeedback key={0}  role="alert">
                <Fragment>
                  {listValues(readValue(fieldErrors)).map((message, key) => {
                    return <li key={key}>{displayText(message)}</li>;
                  })}
                </Fragment>
              </InlineFeedback>
            ) : null}
            {readValue(error) ? (
              <InlineFeedback key={1}  role="alert">
                {displayText(readValue(error))}
              </InlineFeedback>
            ) : null}
            <footer className="wizard-footer">
              <ActionButton variant="outline" asChild >
                <a  href="#/library">
                  {"保留草稿并返回"}
                </a>
              </ActionButton>
              <span className="spacer" />
              <ActionButton variant="default"

                data-testid="wizard-world-next"
                disabled={!worldChoice.value || readValue(busy)}
              >
                {"下一步"}
              </ActionButton>
            </footer>
          </Questionnaire>
        ) : readValue(step) === "person" || readValue(step) === "analysis" ? (
          <Surface
            key={1}
            as="form"
            className="character-form"
            noValidate=""
            onSubmit={eventModifiers(
              ($event) => readValue(c).analyze(),
              ["prevent"],
            )}
          >
            <header>
              <small>{"第 2 / 4 步"}</small>
              <h2>{"这是一个什么样的人？"}</h2>
              <p className="muted">
                {displayText(
                  readValue(draft).useDefaultWorld
                    ? "生活世界：日常生活世界"
                    : "生活世界：" +
                        (readValue(draft).worldName || "所选已发布底稿"),
                )}
              </p>
            </header>
            <fieldset
              className="character-fieldset character-stack"
              disabled={readValue(busy) || !!readValue(pending)}
            >
              <ChoiceCards label="创建方式" value={draft.mode} disabled={readValue(busy)||!!readValue(pending)} options={modes.map(([value,label,note])=>({value,label,description:note}))} onChange={value=>c.changeMode(value)}/>
              <div className="character-field-grid">
                <label>
                  {"名字" +
                    displayText(
                      readValue(draft).mode === "blank"
                        ? "（必填）"
                        : "（可选）",
                    )}
                  <FieldInput
                    data-testid="wizard-name"
                    maxLength="200"
                    placeholder="你想怎么称呼对方？"
                    value={readValue(draft).name}
                    onChange={(event) => {
                      (($event) => (readValue(draft).name = $event))(
                        event.target.value,
                      );
                      (($event) => readValue(c).invalidate())(event);
                    }}
                  />
                  {readValue(error) &&
                  readValue(draft).mode === "blank" &&
                  !readValue(draft).name.trim() ? (
                    <InlineFeedback key={0} >
                      {"请填写角色名字。"}
                    </InlineFeedback>
                  ) : null}
                </label>
                <label>
                  {"生活时区（必填）"}
                  <FieldInput
                    aria-label="生活时区"
                    placeholder="例如 Asia/Shanghai"
                    value={readValue(draft).timezone}
                    onChange={(event) => {
                      (($event) => (readValue(draft).timezone = $event))(
                        event.target.value,
                      );
                      (($event) => readValue(c).invalidate())(event);
                    }}
                  />
                  {readValue(error) &&
                  (!readValue(draft).timezone ||
                    readValue(error).includes("时区")) ? (
                    <InlineFeedback key={0} >
                      {displayText(
                        readValue(error).includes("时区")
                          ? readValue(error)
                          : "请填写生活时区。",
                      )}
                    </InlineFeedback>
                  ) : null}
                </label>
              </div>
              <label>
                {"角色分组"}
                <ChoiceSelect
                  value={readValue(draft).groupId}
                  onChange={(event) => {
                    (($event) => (readValue(draft).groupId = $event))(
                      event.target.value,
                    );
                    (($event) => readValue(c).persist())(event);
                  }}
                >
                  <option value="">{"未分组"}</option>
                  <Fragment>
                    {listValues(groups.value).map((group) => {
                      return (
                        <option key={group.id} value={group.id}>
                          {displayText(group.name)}
                        </option>
                      );
                    })}
                  </Fragment>
                </ChoiceSelect>
              </label>
              {readValue(draft).mode === "description" ? (
                <Fragment key={0}>
                  <label>
                    {"描述你想认识的人"}
                    <Textarea
                      data-testid="wizard-description"
                      rows="7"
                      placeholder="身份、背景、性格和生活可以写在同一段描述里。"
                      value={readValue(draft).bio}
                      onChange={(event) => {
                        (($event) => (readValue(draft).bio = $event))(
                          event.target.value,
                        );
                        (($event) => readValue(c).invalidate())(event);
                      }}
                    />
                    {readValue(error) && !readValue(draft).bio.trim() ? (
                      <InlineFeedback key={0} >
                        {"请写一段人物描述。"}
                      </InlineFeedback>
                    ) : null}
                  </label>
                  <Disclosure title={<>{"可选：一些相处期望"}</>}>
                    <div className="character-stack">
                      <label>
                        {"性格期望"}
                        <FieldInput
                          value={readValue(draft).traits}
                          onChange={(event) => {
                            (($event) => (readValue(draft).traits = $event))(
                              event.target.value,
                            );
                            (($event) => readValue(c).invalidate())(event);
                          }}
                        />
                      </label>
                      <label>
                        {"说话方式"}
                        <Textarea
                          rows="2"
                          value={readValue(draft).speakingStyle}
                          onChange={(event) => {
                            (($event) =>
                              (readValue(draft).speakingStyle = $event))(
                              event.target.value,
                            );
                            (($event) => readValue(c).invalidate())(event);
                          }}
                        />
                      </label>
                      <label>
                        {"核心价值观"}
                        <Textarea
                          rows="2"
                          value={readValue(draft).coreValues}
                          onChange={(event) => {
                            (($event) =>
                              (readValue(draft).coreValues = $event))(
                              event.target.value,
                            );
                            (($event) => readValue(c).invalidate())(event);
                          }}
                        />
                      </label>
                      <small className="muted">
                        {
                          "这些内容会加入描述，由模型整理；不会把标签直接写成人格数值。"
                        }
                      </small>
                    </div>
                  </Disclosure>
                </Fragment>
              ) : readValue(draft).mode === "json" ? (
                <Fragment key={1}>
                  <ActionButton variant="ghost" asChild >
                    <a

                      href="/templates/yaoguang-character-init-template.v2.json"
                      download=""
                    >
                      {"下载当前 JSON 模板"}
                    </a>
                  </ActionButton>
                  <label>
                    {"选择 JSON 文件"}
                    <FieldInput
                      type="file"
                      accept="application/json,.json"
                      onChange={($event) =>
                        readValue(c).importFile($event.target.files?.[0])
                      }
                    />
                  </label>
                  <p className="muted">
                    {displayText(readValue(draft).fileName || "尚未选择文件") +
                      " · 上限 60000 UTF-8 字节 "}
                  </p>
                  <label>
                    {"核对或修正 JSON "}
                    <Textarea
                      data-testid="wizard-json"
                      rows="10"
                      spellCheck="false"
                      value={readValue(draft).json}
                      onChange={(event) => {
                        (($event) => (readValue(draft).json = $event))(
                          event.target.value,
                        );
                        (($event) => readValue(c).invalidate())(event);
                      }}
                    />
                  </label>
                  <p className="muted">
                    {
                      "填写已知的人物资料，保留模板结构；不要导入聊天存档或其他分析记录。"
                    }
                  </p>
                </Fragment>
              ) : readValue(draft).mode === "template" ? (
                <Fragment key={2}>
                  <label>
                    {"已发布角色模板"}
                    <ChoiceSelect
                      value={readValue(draft).templateId || ""}
                      onChange={templateChanged}
                    >
                      <option value="">{"请选择一份模板"}</option>
                      <Fragment>
                        {listValues(templates.value).map((template) => {
                          return (
                            <option key={template.id} value={template.id}>
                              {displayText(template.name) +
                                " · " +
                                displayText(template.published_version)}
                            </option>
                          );
                        })}
                      </Fragment>
                    </ChoiceSelect>
                  </label>
                  {readValue(draft).templateId ? (
                    <p key={0} className="muted">
                      {" 使用精确发布版本 " +
                        displayText(readValue(draft).templateVersion)}
                    </p>
                  ) : null}
                  {templateError.value && <InlineFeedback  role="alert">{templateError.value}<ActionButton variant="ghost" type="button" onClick={()=>loadSources("character_template")}>重新读取模板</ActionButton></InlineFeedback>}
                  {templateCursor.value ? (
                    <ActionButton variant="outline"
                      key={1}
                      type="button"

                      onClick={($event) =>
                        loadSources("character_template", true)
                      }
                    >
                      {"读取更多模板"}
                    </ActionButton>
                  ) : null}
                  {!templates.value.length ? (
                    <p key={2} className="muted">
                      {" 还没有已发布的角色模板。可以选择其他创建方式，或"}
                      <ActionButton variant="ghost" asChild >
                        <a

                          href="#/studio/edit/new?kind=character_template"
                        >
                          {"去创作中心发布角色模板"}
                        </a>
                      </ActionButton>
                      {"。 "}
                    </p>
                  ) : null}
                </Fragment>
              ) : (
                <p key={3} className="muted">
                  {
                    " 空白创建从名字和生活时区开始，其他资料可以稍后在角色详情里补充。 "
                  }
                </p>
              )}
            </fieldset>
            {configured.value === false &&
            readValue(draft).mode === "description" ? (
              <p key={0} className="approved-guidance">
                {
                  " 还没有配置通用模型。描述分析需要模型服务；也可以先选择空白创建。"
                }
                <ActionButton variant="ghost" asChild >
                  <a  href={configHref("模型用途")}>
                    {"先连接模型 "}<ExternalLinkIcon className="inline size-3.5 shrink-0" aria-hidden="true"/>
                  </a>
                </ActionButton>
              </p>
            ) : null}
            {readValue(step) === "analysis" ? (
              <OperationStatus variant="default" key={1} className="approved-guidance" role="status">
                {" 正在整理人物设定，已等待 " +
                  displayText(
                    Math.max(
                      0,
                      Math.floor(
                        (analysisNow.value -
                          (readValue(analysisStartedAt) || analysisNow.value)) /
                          1000,
                      ),
                    ),
                  ) +
                  " 秒。原输入已保留。 "}
                <ActionButton variant="ghost"
                  type="button"

                  onClick={($event) => readValue(c).cancelAnalysis()}
                >
                  {"停止本次等待"}
                </ActionButton>
              </OperationStatus>
            ) : null}
            {readValue(error) && !Object.keys(readValue(fieldErrors)).length ? (
              <InlineFeedback key={2}  role="alert">
                {displayText(readValue(error)) + " "}
                {readValue(feedback)?.type === "configuration" ? (
                  <ActionButton variant="ghost" asChild  key={0}>
                    <a
                      key={0}

                      href={configHref("模型用途")}
                    >
                      {"去配置中心 "}<ExternalLinkIcon className="inline size-3.5 shrink-0" aria-hidden="true"/>
                    </a>
                  </ActionButton>
                ) : null}
              </InlineFeedback>
            ) : null}
            <footer className="wizard-footer">
              <ActionButton variant="outline"
                type="button"

                disabled={readValue(busy)}
                onClick={prev}
              >
                {"上一步"}
              </ActionButton>
              <span className="spacer" />
              <ActionButton variant="default"

                disabled={
                  readValue(busy) ||
                  !!readValue(pending) ||
                  (readValue(draft).mode === "template" &&
                    (!readValue(draft).templateId ||
                      !readValue(draft).templateVersion))
                }
                data-testid="wizard-analyze"
              >
                {displayText(
                  readValue(busy)
                    ? "整理中…"
                    : readValue(draft).mode === "blank"
                      ? "预览空白角色"
                      : "整理并预览资料",
                )}
              </ActionButton>
            </footer>
          </Surface>
        ) : readValue(step) === "preview" ? (
          <Surface
            key={2}
            as="form"
            className="character-form"
            noValidate=""
            onSubmit={eventModifiers(
              ($event) => readValue(c).activate(),
              ["prevent"],
            )}
          >
            <header>
              <small>{"第 3 / 4 步"}</small>
              <h2>{"在相遇之前，先认识一下"}</h2>
              <p className="muted">
                {" 这是根据你的资料整理的角色。确认后会开始准备衣柜、日程和形象；你可以在下一步查看进度或恢复失败环节。 "}
              </p>
            </header>
            {readValue(draft).mode === "blank" ? (
              <Fragment key={0}>
                <h3>{displayText(readValue(draft).name)}</h3>
                <p>{"生活时区：" + displayText(readValue(draft).timezone)}</p>
                <p className="muted">
                  {" 性格从服务端默认起点开始，经历与认识在后续互动中形成。 "}
                </p>
              </Fragment>
            ) : (
              <Fragment key={1}>
                {hasVisibleSemanticValue(draft.analysis?.core_persona?.personality)&&<section className="character-stack"><h3>性格概览</h3><p className="muted">根据当前资料整理，性格维度在这里供你核对。</p><SemanticTree value={draft.analysis.core_persona.personality}/></section>}
                <ProfileForm
                  fields={readValue(previewFields)}
                  primaryOnly
                  disabled={readValue(busy) || !!readValue(pending)}
                />

                <Disclosure title="更多设定"><div className="character-stack"><ProfileForm fields={readValue(previewFields)} additionalOnly disabled={readValue(busy)||!!readValue(pending)}/>
                <Disclosure title={<>{"初始关系、目标与意图"}</>}>
                  <SemanticTree
                    value={{
                      initial_relationships:
                        readValue(draft).analysis?.initial_relationships,
                      initial_goals: readValue(draft).analysis?.initial_goals,
                      initial_intentions:
                        readValue(draft).analysis?.initial_intentions,
                    }}
                  />
                </Disclosure>
                <label className="check-field">
                  <FieldInput
                    type="checkbox"
                    disabled={readValue(busy) || !!readValue(pending)}
                    onChange={($event) => readValue(c).persist()}
                    modelValue={readValue(draft).includeGoals}
                    onModelChange={($event) =>
                      (readValue(draft).includeGoals = $event)
                    }
                  />
                  {"采用已预览的初始目标和意图"}
                </label>
                <Disclosure title={<>{"完整设定与分析来源"}</>}>
                  <pre className="approved-json">
                    {displayText(
                      JSON.stringify(readValue(draft).analysis, null, 2),
                    )}
                  </pre>
                </Disclosure>
                </div></Disclosure>
              </Fragment>
            )}
            {readValue(error) ? (
              <InlineFeedback key={2}  role="alert">
                {displayText(readValue(error))}
              </InlineFeedback>
            ) : null}
            <footer className="wizard-footer">
              <ActionButton variant="outline"
                type="button"

                disabled={readValue(busy) || !!readValue(pending)}
                onClick={prev}
              >
                {"返回修改"}
              </ActionButton>
              <span className="spacer" />
              <ActionButton variant="default"

                data-testid="wizard-activate"
                disabled={readValue(busy) || !!readValue(pending)}
              >
                {displayText(readValue(busy) ? "正在激活…" : "确认，开始准备")}
              </ActionButton>
            </footer>
          </Surface>
        ) : (
          <Surface key={3} as="section" className="character-preparation">
            <header>
              <small>
                {displayText(readValue(draft).name || "角色") + "的新生活"}
              </small>
              <h2>
                {displayText(
                  readValue(progress)?.complete
                    ? "准备好了，很高兴遇见你"
                    : readValue(progress)?.blocker
                      ? "准备已暂停，需要恢复"
                      : "一点点，准备好相遇",
                )}
              </h2>
              <p className="muted">
                {displayText(
                  readValue(progress)?.complete
                    ? "每个准备环节都已读回确认，接下来一起走进日常。"
                    : readValue(progress)?.blocker
                      ? `「${readValue(progress).blocker.title}」尚未完成，后续等待环节不会自行继续。请先恢复此环节，角色与原任务均已保留。`
                      : "角色与原任务会保留。遇到问题时，可单独恢复尚未完成的环节。",
                )}
              </p>
            </header>
            <PreparationSteps
              stages={readValue(progress)?.stages || []}
              busy={readValue(busy)}
              onRecover={readValue(c).recoverStage}
            />
            {!readValue(progress) ? (
              <ReadState key={0} className="muted" role="status">
                {displayText(
                  readValue(busy)
                    ? "正在确认原创建结果…"
                    : "正在读取实际准备进度…",
                )}
              </ReadState>
            ) : null}
            {readValue(error) ? (
              <InlineFeedback key={1}  role="alert">
                {displayText(readValue(error))}
              </InlineFeedback>
            ) : null}
            {draft.worldSource && !draft.worldSelectionConfirmed && <CardSurface as="section" className="character-stack">
              <p>角色已建立。世界配置复用仍需核对，原角色和原提交身份均已保留。</p>
              {worldSourceReview.value ? <><SemanticTree value={worldSourceReview.value.preview}/><ActionButton variant="outline" type="button"  disabled={readValue(busy)} onClick={()=>c.confirmWorldSourceReview()}>确认新版配置并继续复用</ActionButton></> : <ActionButton variant="outline" type="button"  disabled={readValue(busy)} onClick={()=>c.reviewWorldSource()}>核对已有世界的最新配置</ActionButton>}
            </CardSurface>}
            <div className="preparation-done row">
              {readValue(draft).activatedId &&
              readValue(draft).worldSelectionConfirmed ? (
                <ActionButton variant="default" asChild  key={0}>
                  <a
                    key={0}

                    href={
                      "#/c/" +
                      encodeURIComponent(readValue(draft).activatedId) +
                      "/chat"
                    }
                  >
                    {"先开始相处"}
                  </a>
                </ActionButton>
              ) : null}
              {readValue(draft).activatedId ? (
                <ActionButton variant="outline" asChild  key={1}>
                  <a
                    key={1}

                    href={
                      "#/c/" +
                      encodeURIComponent(readValue(draft).activatedId) +
                      "/profile"
                    }
                  >
                    {"角色详情"}
                  </a>
                </ActionButton>
              ) : null}
              <ActionButton variant="outline"

                disabled={readValue(busy)}
                onClick={($event) => readValue(c).refreshProgress()}
              >
                {"重新读取进度"}
              </ActionButton>
              {readValue(progress)?.complete ? (
                <ActionButton variant="ghost"
                  key={2}

                  onClick={($event) => (readValue(c).newDraft(), worldChoice.value=false)}
                >
                  {"创建新的角色"}
                </ActionButton>
              ) : null}
            </div>
          </Surface>
        )}
        {readValue(pending) ? (
          <Surface key={4} as="section" className="approved-guidance">
            <p>
              {
                " 原激活结果尚未确认，当前资料和请求身份已保留。请先查询或重试原提交，避免创建第二个角色。 "
              }
            </p>
            {!readValue(pending).rejection ? (
              <ActionButton variant="outline"
                key={0}

                disabled={readValue(busy)}
                onClick={($event) => readValue(c).activate(true)}
              >
                {"查询或重试原激活"}
              </ActionButton>
            ) : (
              <ActionButton variant="outline"
                key={1}

                onClick={($event) => readValue(c).reanalyze()}
              >
                {"保留输入并重新整理"}
              </ActionButton>
            )}
          </Surface>
        ) : null}
        <ActionButton variant="ghost" asChild >
          <a  href="#/library">
            {"保留草稿并返回角色库"}
          </a>
        </ActionButton>
        {readValue(draft).activatedId &&
        !readValue(c).archived &&
        !readValue(pending) ? (
          <ActionButton variant="ghost"
            key={5}

            onClick={($event) => (readValue(c).newDraft(), worldChoice.value=false)}
          >
            {"再认识一个人（保留当前进度）"}
          </ActionButton>
        ) : null}
      </div>
    );
  };
}
export default function CharacterWizardPage(input = {}) {
  return useController(createCharacterWizardPageController, {
    ...input,
  });
}

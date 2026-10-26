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
    } = c,
    worlds = lifecycle.cell([]),
    templates = lifecycle.cell([]),
    worldCursor = lifecycle.cell(""),
    templateCursor = lifecycle.cell(""),
    sourceError = lifecycle.cell(""),
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
      sourceError.value = "";
    } catch (cause) {
      if (active) sourceError.value = errorMessage(cause);
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
    return (
      <div ref={elementRef(wizardRoot)} className="character-workflow">
        <div className="character-breadcrumb">
          <ActionButton asChild className="text-button">
            <a className="text-button" href="#/library">
              {"角色库"}
            </a>
          </ActionButton>
          <span>{"›"}</span>
          <small>
            {displayText(
              readValue(draft).activatedId ? "为新的生活做准备" : "新的相识",
            )}
          </small>
        </div>
        <ol className="wizard-steps">
          <Fragment>
            {listValues(navSteps).map((label, index) => {
              return (
                <li
                  key={label}
                  className={cn({
                    active: index === currentStep.value,
                    complete: index < currentStep.value,
                  })}
                  aria-current={
                    index === currentStep.value ? "step" : undefined
                  }
                >
                  <span>{displayText(index + 1)}</span>
                  {displayText(label)}
                </li>
              );
            })}
          </Fragment>
        </ol>
        {readValue(step) === "world" ? (
          <Surface
            key={0}
            as="form"
            className="card character-form"
            noValidate=""
            onSubmit={eventModifiers(
              ($event) => readValue(c).next(),
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
            <ActionButton
              type="button"
              className={cn([
                "button",
                {
                  primary: readValue(draft).useDefaultWorld,
                },
              ])}
              aria-pressed={readValue(draft).useDefaultWorld}
              onClick={($event) => readValue(c).selectWorld(null)}
            >
              {"使用日常生活世界"}
            </ActionButton>
            <small className="muted">
              {"会为新角色建立独立的生活世界，具体地点可在相处中继续补充。"}
            </small>
            <div className="character-stack">
              <Fragment>
                {listValues(worlds.value).map((world) => {
                  return (
                    <ActionButton
                      key={world.id}
                      type="button"
                      className="world-choice button"
                      aria-pressed={
                        readValue(draft).worldResourceId === world.id &&
                        !readValue(draft).useDefaultWorld
                      }
                      onClick={($event) => readValue(c).selectWorld(world)}
                    >
                      <b>{displayText(world.name)}</b>
                      <small>
                        {"已发布版本 " + displayText(world.published_version)}
                      </small>
                    </ActionButton>
                  );
                })}
              </Fragment>
              {!worlds.value.length && !sourceError.value ? (
                <p key={0} className="muted">
                  {" 还没有已发布的世界底稿，也可以先使用日常生活世界。 "}
                </p>
              ) : null}
              {worldCursor.value ? (
                <ActionButton
                  key={1}
                  type="button"
                  className="button"
                  onClick={($event) => loadSources("world", true)}
                >
                  {"读取更多世界"}
                </ActionButton>
              ) : null}
              {sourceError.value ? (
                <p key={2} className="inline-error">
                  {displayText(sourceError.value) + " "}
                  <ActionButton
                    type="button"
                    className="text-button"
                    onClick={($event) => loadSources("world")}
                  >
                    {"重新读取"}
                  </ActionButton>
                </p>
              ) : null}
              <ActionButton asChild className="text-button">
                <a className="text-button" href="#/studio/edit/new?kind=world">
                  {"创建世界设定 ↗"}
                </a>
              </ActionButton>
            </div>
            {Object.keys(readValue(fieldErrors)).length ? (
              <ul key={0} className="inline-error" role="alert">
                <Fragment>
                  {listValues(readValue(fieldErrors)).map((message, key) => {
                    return <li key={key}>{displayText(message)}</li>;
                  })}
                </Fragment>
              </ul>
            ) : null}
            {readValue(error) ? (
              <p key={1} className="inline-error" role="alert">
                {displayText(readValue(error))}
              </p>
            ) : null}
            <footer className="wizard-footer">
              <ActionButton asChild className="button">
                <a className="button" href="#/library">
                  {"保留草稿并返回"}
                </a>
              </ActionButton>
              <span className="spacer" />
              <ActionButton
                className="button primary"
                data-testid="wizard-world-next"
              >
                {"下一步"}
              </ActionButton>
            </footer>
          </Surface>
        ) : readValue(step) === "person" || readValue(step) === "analysis" ? (
          <Surface
            key={1}
            as="form"
            className="card character-form"
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
              <div className="creation-mode-choices">
                <Fragment>
                  {listValues(modes).map(([value, label, note]) => {
                    return (
                      <ActionButton
                        key={value}
                        type="button"
                        className={cn([
                          "button creation-mode",
                          {
                            active: readValue(draft).mode === value,
                          },
                        ])}
                        aria-pressed={readValue(draft).mode === value}
                        onClick={($event) => readValue(c).changeMode(value)}
                      >
                        <b>{displayText(label)}</b>
                        <small>{displayText(note)}</small>
                      </ActionButton>
                    );
                  })}
                </Fragment>
              </div>
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
                    <small key={0} className="inline-error">
                      {"请填写角色名字。"}
                    </small>
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
                    <small key={0} className="inline-error">
                      {displayText(
                        readValue(error).includes("时区")
                          ? readValue(error)
                          : "请填写生活时区。",
                      )}
                    </small>
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
                      <small key={0} className="inline-error">
                        {"请写一段人物描述。"}
                      </small>
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
                  <ActionButton asChild className="text-button">
                    <a
                      className="text-button"
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
                  {templateCursor.value ? (
                    <ActionButton
                      key={1}
                      type="button"
                      className="button"
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
                      <ActionButton asChild className="text-button">
                        <a
                          className="text-button"
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
                <ActionButton asChild className="text-button">
                  <a className="text-button" href={configHref("模型用途")}>
                    {"先连接模型 ↗"}
                  </a>
                </ActionButton>
              </p>
            ) : null}
            {readValue(step) === "analysis" ? (
              <p key={1} className="approved-guidance" role="status">
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
                <ActionButton
                  type="button"
                  className="text-button"
                  onClick={($event) => readValue(c).cancelAnalysis()}
                >
                  {"停止本次等待"}
                </ActionButton>
              </p>
            ) : null}
            {readValue(error) && !Object.keys(readValue(fieldErrors)).length ? (
              <p key={2} className="inline-error" role="alert">
                {displayText(readValue(error)) + " "}
                {readValue(feedback)?.type === "configuration" ? (
                  <ActionButton asChild className="text-button" key={0}>
                    <a
                      key={0}
                      className="text-button"
                      href={configHref("模型用途")}
                    >
                      {"去配置中心 ↗"}
                    </a>
                  </ActionButton>
                ) : null}
              </p>
            ) : null}
            <footer className="wizard-footer">
              <ActionButton
                type="button"
                className="button"
                disabled={readValue(busy)}
                onClick={prev}
              >
                {"上一步"}
              </ActionButton>
              <span className="spacer" />
              <ActionButton
                className="button primary"
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
            className="card character-form"
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
                {" 先核对身份、表达和生活设定，确认后开始准备相遇。 "}
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
                <ProfileForm
                  fields={readValue(previewFields)}
                  disabled={readValue(busy) || !!readValue(pending)}
                />
                <Disclosure title={<>{"共同性格与人格档案（只读）"}</>}>
                  <SemanticTree
                    value={{
                      personality:
                        readValue(draft).analysis?.core_persona?.personality,
                      personality_system:
                        readValue(draft).analysis?.core_persona
                          ?.personality_system,
                    }}
                  />
                </Disclosure>
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
              </Fragment>
            )}
            {readValue(error) ? (
              <p key={2} className="inline-error" role="alert">
                {displayText(readValue(error))}
              </p>
            ) : null}
            <footer className="wizard-footer">
              <ActionButton
                type="button"
                className="button"
                disabled={readValue(busy) || !!readValue(pending)}
                onClick={prev}
              >
                {"返回修改输入"}
              </ActionButton>
              <span className="spacer" />
              <ActionButton
                className="button primary"
                data-testid="wizard-activate"
                disabled={readValue(busy) || !!readValue(pending)}
              >
                {displayText(readValue(busy) ? "正在激活…" : "确认并开始准备")}
              </ActionButton>
            </footer>
          </Surface>
        ) : (
          <Surface key={3} as="section" className="card character-preparation">
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
              <p key={0} className="muted" role="status">
                {displayText(
                  readValue(busy)
                    ? "正在确认原创建结果…"
                    : "正在读取实际准备进度…",
                )}
              </p>
            ) : null}
            {readValue(error) ? (
              <p key={1} className="inline-error" role="alert">
                {displayText(readValue(error))}
              </p>
            ) : null}
            <div className="preparation-done row">
              {readValue(draft).activatedId &&
              readValue(draft).worldSelectionConfirmed ? (
                <ActionButton asChild className="button primary" key={0}>
                  <a
                    key={0}
                    className="button primary"
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
                <ActionButton asChild className="button" key={1}>
                  <a
                    key={1}
                    className="button"
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
              <ActionButton
                className="button"
                disabled={readValue(busy)}
                onClick={($event) => readValue(c).refreshProgress()}
              >
                {"重新读取进度"}
              </ActionButton>
              {readValue(progress)?.complete ? (
                <ActionButton
                  key={2}
                  className="text-button"
                  onClick={($event) => readValue(c).newDraft()}
                >
                  {"创建新的角色"}
                </ActionButton>
              ) : null}
            </div>
          </Surface>
        )}
        {readValue(pending) ? (
          <Surface key={4} as="section" className="card approved-guidance">
            <p>
              {
                " 原激活结果尚未确认，当前资料和请求身份已保留。请先查询或重试原提交，避免创建第二个角色。 "
              }
            </p>
            {!readValue(pending).rejection ? (
              <ActionButton
                key={0}
                className="button"
                disabled={readValue(busy)}
                onClick={($event) => readValue(c).activate(true)}
              >
                {"查询或重试原激活"}
              </ActionButton>
            ) : (
              <ActionButton
                key={1}
                className="button"
                onClick={($event) => readValue(c).reanalyze()}
              >
                {"保留输入并重新整理"}
              </ActionButton>
            )}
          </Surface>
        ) : null}
        <ActionButton asChild className="text-button">
          <a className="text-button" href="#/library">
            {"保留草稿并返回角色库"}
          </a>
        </ActionButton>
        {readValue(draft).activatedId &&
        !readValue(c).archived &&
        !readValue(pending) ? (
          <ActionButton
            key={5}
            className="text-button"
            onClick={($event) => readValue(c).newDraft()}
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

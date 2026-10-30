import { Component } from 'react';
import {Button} from '@/components/ui/button';
import { useController,useWorkspace } from "../page";
import { selector, elementRef } from "@/state/view-model";
import { reusableWorlds } from "@/approved-ui/world-sources.mjs";
import { directory, activationFromAnalysis } from "@/v3/providers/real.ts";
import { list } from "@/v3/proto/store.js";
import { semanticLabel } from "@/v3/proto/components/semantic-fields.js";
import { errorMessage } from "@/v3/domain-presentation.mjs";
import { createCreationController } from "@/state/creation-controller.mjs";
import { migrateCreationRecovery } from "@/approved-ui/creation-recovery.mjs";
import CharacterWizardView from "../views/CharacterWizard";
import CreationRecoveryFeedback from '../shared/CreationRecoveryFeedback';
import {inspectCreationRecovery,creationRecoveryProblem} from '@/approved-ui/creation-recovery-state.mjs';
function createCharacterWizardPageController(__props, context, lifecycle) {
  const w = lifecycle.workspace,
    owner = w.state.session?.actorId,
    migrated = migrateCreationRecovery(owner, localStorage, sessionStorage,{resume:new URLSearchParams(location.hash.split('?')[1]||'').get('resume')||''}),
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
    worldChoice = lifecycle.cell(
      !!draft.worldSource ||
        !!draft.worldResourceId ||
        !!draft.worldChoiceConfirmed ||
        !!draft.activatedId,
    ),
    worlds = lifecycle.cell([]),
    templates = lifecycle.cell([]),
    worldCursor = lifecycle.cell(""),
    templateCursor = lifecycle.cell(""),
    sourceError = lifecycle.cell(""),
    existingWorldError = lifecycle.cell(""),
    templateError = lifecycle.cell(""),
    existingWorldLoading = lifecycle.cell(false),
    sourceLoading = lifecycle.cell(false),
    templateLoading = lifecycle.cell(false),
    configured = lifecycle.cell(null),
    worldsConfirmed = lifecycle.cell(false),
    publishedWorldsConfirmed = lifecycle.cell(false),
    worldReadNotice = lifecycle.cell("");
  if (!draft.activatedId && !pending.value && !worldChoice.value)
    step.value = "world";
  const wizardRoot = lifecycle.cell(null);
  const analysisNow = lifecycle.cell(Date.now());
  lifecycle.observe(
    step,
    () => {
      const root = wizardRoot.value,
        heading = root?.querySelector("h1");
      if (heading) {
        heading.tabIndex = -1;
        heading.focus({
          preventScroll: true,
        });
        root.closest(".workspace-page")?.scrollTo({ top: 0 });
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
  const groups = lifecycle.cell(
      list("groups").filter((g) => g.id !== "ungrouped"),
    ),
    groupsLoading = lifecycle.cell(false),
    groupsError = lifecycle.cell("");
  let groupsEpoch = 0;
  async function loadGroups() {
    const token = ++groupsEpoch;
    groupsLoading.value = true;
    try {
      const value = await w.client.listActorGroups();
      if (!active || !c.live() || token !== groupsEpoch) return;
      if (!Array.isArray(value)) throw Error("角色分组尚未完整读到。");
      groups.value = value.filter((g) => g.id !== "ungrouped");
      groupsError.value = "";
    } catch (cause) {
      if (active && c.live() && token === groupsEpoch)
        groupsError.value = errorMessage(cause);
    } finally {
      if (active && c.live() && token === groupsEpoch)
        groupsLoading.value = false;
    }
  }
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
  async function loadExistingWorlds(refresh = false) {
    existingWorldLoading.value = true;
    worldsConfirmed.value = false;
    if (refresh) {
      try {
        await w.refreshDirectory?.();
      } catch (cause) {
        if (c.live()) {
          existingWorldError.value = errorMessage(cause);
          existingWorldLoading.value = false;
        }
        return;
      }
    }
    const token = ++worldReadEpoch,
      characters = w.state.characters.map(({ id, name }) => ({ id, name }));
    try {
      if (w.state.errors?.bootstrap)
        throw Error("角色目录尚未读到，请重新读取后选择世界。");
      const result = await reusableWorlds(
        w.client,
        characters,
        existingWorlds.value,
      );
      if (!active || !c.live() || token !== worldReadEpoch) return;
      existingWorlds.value = result.items;
      worldsConfirmed.value = result.unavailable === 0;
      existingWorldError.value = result.unavailable
        ? `${result.unavailable} 个角色的世界暂未读到（${result.unavailableNames.join("、")}），已读到的世界仍可选择。`
        : "";
      worldReadNotice.value = result.unavailable
        ? "上次已读到的世界保留，暂不可确认没有其他世界。"
        : `已读取 ${characters.length} 个角色的世界目录，共 ${result.items.length} 个生活世界。`;
      if (
        !busy.value &&
        !draft.activatedId &&
        !pending.value &&
        draft.worldSource &&
        !result.items.some(
          (world) =>
            world.id === draft.worldSource.worldId,
        )
      ) {
        worldChoice.value = false;
        step.value = "world";
      }
    } catch (cause) {
      if (active && c.live() && token === worldReadEpoch)
        existingWorldError.value = errorMessage(cause);
    } finally {
      if (active && c.live() && token === worldReadEpoch)
        existingWorldLoading.value = false;
    }
  }
  lifecycle.observe(
    () => w.state.characters.map(({ id, name }) => [id, name]),
    () => void loadExistingWorlds(),
    { immediate: true },
  );
  function chooseWorld(world) {
    c.selectWorld(world);
    worldChoice.value = true;
    draft.worldChoiceConfirmed = true;
    c.persist();
  }
  function chooseExistingWorld(world) {
    c.selectExistingWorld(world);
    worldChoice.value = true;
    draft.worldChoiceConfirmed = true;
    c.persist();
  }
  const sourceEpochs = { world: 0, character_template: 0 },
    failedSources = {};
  async function loadSources(kind, more = false, frozenCursor) {
    const token = ++sourceEpochs[kind],
      loading = kind === "world" ? sourceLoading : templateLoading;
    const cursor =
      frozenCursor ??
      (more
        ? kind === "world"
          ? worldCursor.value
          : templateCursor.value
        : "");
    loading.value = true;
    if (kind === "world") publishedWorldsConfirmed.value = false;
    try {
      const result = await w.client.listStudioResources({
        kind,
        published: true,
        cursor,
      });
      if (!active || !c.live() || sourceEpochs[kind] !== token) return;
      const previous = kind === "world" ? worlds.value : templates.value;
      const publications = await Promise.allSettled(
        result.items.map((row) =>
          directory.publication(row.id, row.published_version),
        ),
      );
      if (!active || !c.live() || sourceEpochs[kind] !== token) return;
      let unavailable = 0;
      const rows = result.items.map((row, index) => {
        const read = publications[index],
          publication = read.status === "fulfilled" ? read.value : null;
        if (
          publication?.id === row.id &&
          publication.version === row.published_version &&
          publication.kind === kind &&
          publication.immutable === true
        )
          return { ...row, publication };
        unavailable++;
        const retained = previous.find(
          (old) =>
            old.id === row.id &&
            old.published_version === row.published_version,
        )?.publication;
        return {
          ...row,
          publication: retained || null,
          publicationError: "精确发布定义尚未读到",
        };
      });
      if (kind === "world") {
        worlds.value = more
          ? [
              ...new Map(
                [...worlds.value, ...rows].map((row) => [row.id, row]),
              ).values(),
            ]
          : rows;
        worldCursor.value = result.next_cursor || "";
        publishedWorldsConfirmed.value = !more && !cursor &&
          result.available !== false && !result.next_cursor && unavailable === 0;
      } else {
        templates.value = more
          ? [
              ...new Map(
                [...templates.value, ...rows].map((row) => [row.id, row]),
              ).values(),
            ]
          : rows;
        templateCursor.value = result.next_cursor || "";
      }
      (kind === "world" ? sourceError : templateError).value = unavailable
        ? `${unavailable} 份精确发布定义暂未读到，已确认的来源仍可选择。`
        : "";
      if (unavailable) failedSources[kind] = { more, cursor };
      else delete failedSources[kind];
    } catch (cause) {
      if (active && c.live() && sourceEpochs[kind] === token) {
        failedSources[kind] = { more, cursor };
        (kind === "world" ? sourceError : templateError).value =
          errorMessage(cause);
      }
    } finally {
      if (active && c.live() && sourceEpochs[kind] === token)
        loading.value = false;
    }
  }
  function retrySources(kind) {
    const original = failedSources[kind];
    return loadSources(kind, original?.more ?? false, original?.cursor);
  }
  const selectedPublication = lifecycle.cell(null),
    selectedPublicationError = lifecycle.cell(""),
    selectedPublicationLoading = lifecycle.cell(false);
  let selectedPublicationEpoch = 0;
  async function readSelectedPublication() {
    const id = draft.worldResourceId,
      version = draft.worldResourceVersion,
      token = ++selectedPublicationEpoch;
    selectedPublication.value = null;
    selectedPublicationError.value = "";
    if (!id || !version) {
      selectedPublicationLoading.value = false;
      return;
    }
    selectedPublicationLoading.value = true;
    try {
      const value = await directory.publication(id, version);
      if (!active || !c.live() || token !== selectedPublicationEpoch) return;
      if (
        value.id !== id ||
        value.version !== version ||
        value.kind !== "world" ||
        value.immutable !== true
      )
        throw Error("所选世界的精确发布版本尚未完整确认。");
      selectedPublication.value = value;
      if (!draft.activatedId && !pending.value) {
        draft.worldName = value.name;
        c.persist();
      }
    } catch (cause) {
      if (active && c.live() && token === selectedPublicationEpoch)
        selectedPublicationError.value = errorMessage(cause);
    } finally {
      if (active && c.live() && token === selectedPublicationEpoch)
        selectedPublicationLoading.value = false;
    }
  }
  lifecycle.observe(
    () => [draft.worldResourceId, draft.worldResourceVersion],
    () => void readSelectedPublication(),
    { immediate: true },
  );
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
    void loadGroups();
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
  return () => {
    const canContinue =
      worldChoice.value &&
      (draft.worldSource
        ? existingWorlds.value.some(
            (world) =>
              world.id === draft.worldSource.worldId,
          )
        : draft.useDefaultWorld
          ? true
          : !!selectedPublication.value &&
            !selectedPublicationLoading.value &&
            !selectedPublicationError.value);
    return (
      <CharacterWizardView
        model={{
          w,
          c,
          draft,
          step: step.value,
          busy: busy.value,
          error: error.value,
          feedback: feedback.value,
          fieldErrors: fieldErrors.value,
          progress: progress.value,
          previewFields: previewFields.value,
          pending: pending.value,
          analysisStartedAt: analysisStartedAt.value,
          analysisNow: analysisNow.value,
          worldSourceReview: worldSourceReview.value,
          recoveryPending: c.recoveryPending.value,
          recoveryReview: c.recoveryReview.value,
          existingWorlds: existingWorlds.value,
          worlds: worlds.value,
          templates: templates.value,
          worldCursor: worldCursor.value,
          templateCursor: templateCursor.value,
          sourceError: sourceError.value,
          existingWorldError: existingWorldError.value,
          templateError: templateError.value,
          existingWorldLoading: existingWorldLoading.value,
          sourceLoading: sourceLoading.value,
          templateLoading: templateLoading.value,
          configured: configured.value,
          worldsConfirmed: worldsConfirmed.value,
          publishedWorldsConfirmed: publishedWorldsConfirmed.value,
          worldReadNotice: worldReadNotice.value,
          selectedPublication: selectedPublication.value,
          selectedPublicationError: selectedPublicationError.value,
          selectedPublicationLoading: selectedPublicationLoading.value,
          readSelectedPublication,
          currentStep: currentStep.value,
          canContinue,
          groups: groups.value,
          groupsLoading: groupsLoading.value,
          groupsError: groupsError.value,
          loadGroups,
          rootRef: elementRef(wizardRoot),
          chooseWorld,
          chooseExistingWorld,
          loadExistingWorlds,
          loadSources,
          retrySources,
          templateChanged,
          prev,
          configHref,
          showError: (message) => {
            error.value = message;
          },
          goStep: (index) => {
            if (
              busy.value ||
              pending.value ||
              draft.activatedId ||
              index >= currentStep.value
            )
              return;
            step.value =
              index === 0 ? "world" : index === 1 ? "person" : "preview";
            c.persist();
          },
          newDraft: () => {
            if (!c.newDraft()) return;
            worldChoice.value = false;
            draft.worldChoiceConfirmed = false;
            step.value = "world";
            c.persist();
          },
        }}
      />
    );
  };
}
function CharacterWizardReady(input = {}) {
  return useController(createCharacterWizardPageController, input);
}
class CreationEntryBoundary extends Component {
  state={error:null,attempt:0};
  static getDerivedStateFromError(error){return {error};}
  render(){
    if(!this.state.error)return <CharacterWizardReady key={this.state.attempt} {...this.props.input}/>;
    let recovery=this.state.error.recoveryState;
    if(!recovery){try{recovery=inspectCreationRecovery({owner:this.props.owner,storage:localStorage,resume:new URLSearchParams(location.hash.split('?')[1]||'').get('resume')||''});}catch{}}
    if(!recovery?.blocked)recovery=creationRecoveryProblem(this.state.error,[...(recovery?.records||[]),...(this.state.error.records||[])]);
    return <section className="companion-page"><header className="replica-page-heading"><div><div className="replica-eyebrow">CREATE / CONTINUE YOUR STORY</div><h1>创建记录暂时无法继续</h1><p>原输入和原请求会保留。当前没有发起新的创建。</p></div></header><CreationRecoveryFeedback state={recovery} onReload={()=>this.setState({error:null,attempt:this.state.attempt+1})}/><Button asChild variant="outline"><a href="#/library">返回角色库</a></Button></section>;
  }
}
export default function CharacterWizardPage(input={}){
  const w=useWorkspace();
  return <CreationEntryBoundary key={w.state.session?.actorId} owner={w.state.session?.actorId} input={input}/>;
}

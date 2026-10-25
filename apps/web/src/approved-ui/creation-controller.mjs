import { reactive, computed, ref } from "vue";
import {
  checkpointCreation,
  checkpointCreationAnalysis,
  selectCreationWorld,
} from "../v3/creation-draft.mjs";
import {
  parseCreationInput,
  parseCreationAnalysis,
  creationAnalysisRequest,
} from "../v3/creation-json.mjs";
import { finishCreationWorld } from "../v3/creation-world.mjs";
import { persistentCommand } from "../v3/providers/persistent-command.mjs";
import { creationFeedback } from "../v3/creation-feedback.mjs";
import { errorMessage } from "../v3/domain-presentation.mjs";
import { randomId } from "../random-id.mjs";
import { creationRecovery, creationHistoryKey } from "./creation-recovery.mjs";
import {
  profileFields,
  profileChanges,
  applyProfileChanges,
} from "./profile-fields.mjs";
import { readApprovedCreationProgress } from "./creation-progress.mjs";
const clone = (value) => JSON.parse(JSON.stringify(value));
export function createCreationController({
  owner,
  client,
  directory,
  storage,
  analysisToActivation,
  ownerCurrent,
  refreshDirectory = async () => {},
  label,
  query = new URLSearchParams(),
  today = (zone) =>
    new Intl.DateTimeFormat("en-CA", {
      timeZone: zone,
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
    }).format(new Date()),
}) {
  const recovery = creationRecovery({ owner, storage, query }),
    draft = reactive(recovery.read()),
    busy = ref(false),
    error = ref(""),
    feedback = ref(null),
    progress = ref(null),
    previewFields = ref([]),
    fieldErrors = ref({}),
    analysisStartedAt = ref(null),
    step = ref(
      draft.activatedId
        ? "progress"
        : draft.analysis
          ? "preview"
          : draft.v62Step || "world",
    );
  let active = true,
    readEpoch = 0,
    analysisEpoch = 0,
    analysisAbort = null;
  const live = () => active && ownerCurrent(),
    savedCurrent = (requestId) =>
      live() && recovery.read().requestId === requestId;
  const persist = () => {
    if (live()) {
      draft.v62Step = step.value;
      draft.step =
        step.value === "world"
          ? 0
          : step.value === "person"
            ? 1
            : step.value === "preview"
              ? 2
              : 3;
      recovery.save(clone(draft));
      if(draft.activatedId)storage.setItem(creationHistoryKey(owner,draft.activatedId),JSON.stringify(draft));
    }
  };
  const activation = persistentCommand({
    key: recovery.commandKey,
    storage: () => storage,
    preAdmissionConflicts: ["activation_analysis_stale"],
    execute: async (value) => {
      if (!live()) throw Error("所有者已变化，原创建请求已保留");
      const result = await directory.activate(value.body);
      if (!ownerCurrent())
        throw Error("所有者已变化，原激活结果仍需以原身份核对");
      checkpointCreation(recovery, value, result, ownerCurrent);
      return result;
    },
  });
  const pending = ref(activation.pending());
  if (pending.value) step.value = "progress";
  if (!pending.value && !draft.activatedId && !draft.analysis && step.value === "analysis") {
    // A persisted screen cannot restore an in-memory request or AbortController.
    step.value = "person";
    draft.requestId = randomId();
    error.value = "上次整理已停止等待，原输入已保留，请重新整理并审阅。";
  }
  if (
    query.get("worldResourceId") &&
    query.get("worldVersion") &&
    selectCreationWorld(draft, {
      resourceId: query.get("worldResourceId"),
      version: query.get("worldVersion"),
      pending: !!pending.value,
    })
  ) {
    draft.previewDraft = [];
    step.value = "person";
  }
  if (
    query.get("template") &&
    query.get("version") &&
    !draft.activatedId &&
    !pending.value
  ) {
    draft.mode = "template";
    draft.templateId = query.get("template");
    draft.templateVersion = query.get("version");
    step.value = "person";
  }
  function initializePreview() {
    if (!draft.analysis) return;
    if(!draft.analysis.core_persona.identity.timezone)draft.analysis.core_persona.identity.timezone=draft.timezone;
    previewFields.value = profileFields(draft.analysis.core_persona, label);
    const restored = new Map(
      (draft.previewDraft || []).map((f) => [JSON.stringify(f.path), f.value]),
    );
    for (const field of previewFields.value)
      if (restored.has(JSON.stringify(field.path)))
        field.value = restored.get(JSON.stringify(field.path));
  }
  if (draft.activatedId && !draft.activationDate)
    draft.activationDate = today(draft.timezone);
  if(!draft.timezone?.trim())draft.timezone=Intl.DateTimeFormat().resolvedOptions().timeZone||"Asia/Shanghai";
  initializePreview();
  persist();
  function fail(cause) {
    if (live()) {
      feedback.value = creationFeedback(cause);
      error.value = feedback.value.message;
      pending.value = activation.pending();
    }
  }
  function invalidate() {
    if (draft.activatedId || pending.value) return;
    fieldErrors.value={}; error.value=""; feedback.value=null;
    draft.analysis = null;
    draft.analysisText = "";
    draft.previewDraft = [];
    draft.templateReviewed = false;
    draft.requestId = randomId();
    previewFields.value = [];
    persist();
  }
  function selectWorld(world) {
    if (draft.activatedId || pending.value) return;
    if (!world) {
      if (!draft.useDefaultWorld) {
        draft.useDefaultWorld = true;
        draft.worldResourceId = "";
        draft.worldResourceVersion = "";
        draft.worldName = "";
        invalidate();
      }
    } else if (
      selectCreationWorld(draft, {
        resourceId: world.id,
        version: world.published_version,
        pending: !!pending.value,
      })
    ) {
      draft.worldName = world.name;
      draft.previewDraft = [];
      previewFields.value = [];
    }
    persist();
  }
  function changeMode(mode) {
    if (busy.value || pending.value || draft.activatedId) return;
    draft.modeDrafts ??= {};
    draft.modeDrafts[draft.mode] = {
      bio: draft.bio,
      json: draft.json,
      fileName: draft.fileName,
      traits: draft.traits,
      speakingStyle: draft.speakingStyle,
      coreValues: draft.coreValues,
    };
    draft.mode = mode;
    Object.assign(
      draft,
      {
        bio: "",
        json: "",
        fileName: "",
        traits: "",
        speakingStyle: "",
        coreValues: "",
      },
      draft.modeDrafts[mode],
    );
    invalidate();
  }
  function next() {
    error.value = "";
    if (step.value === "world") {
      if (
        !draft.useDefaultWorld &&
        (!draft.worldResourceId || !draft.worldResourceVersion)
      ) {
        error.value = "请选择一份已发布的世界底稿，或使用日常生活世界。";
        return;
      }
      step.value = "person";
      persist();
    }
  }
  async function importFile(file) {
    if (!file) return;
    try {
      if (file.size > 60000) throw Error("JSON 文件超过 60000 UTF-8 字节上限");
      const raw = await file.text();
      if (!live() || pending.value || draft.activatedId) return;
      draft.json = raw;
      draft.fileName = file.name;
      invalidate();
      parseCreationInput(raw);
      draft.jsonValid = true;
      persist();
      error.value = "";
    } catch (cause) {
      if (live()) {
        draft.jsonValid = false;
        fail(cause);
      }
    }
  }
  async function analyze() {
    if (!live() || busy.value || pending.value || draft.activatedId) return;
    const attempt=++analysisEpoch;
    error.value = "";
    feedback.value = null;
    try {
      fieldErrors.value={};
      if(!draft.timezone?.trim())fieldErrors.value.timezone="请填写生活时区。";
      if(draft.mode==="blank"&&!draft.name.trim())fieldErrors.value.name="请填写角色名字。";
      if(draft.mode==="description"&&!draft.bio.trim())fieldErrors.value.bio="请写一段人物描述。";
      if(Object.keys(fieldErrors.value).length)throw Error(Object.values(fieldErrors.value)[0]);
      try {
        new Intl.DateTimeFormat("zh-CN", { timeZone: draft.timezone });
      } catch {
        fieldErrors.value.timezone="请填写有效的 IANA 时区，例如 Asia/Shanghai。";
        throw Error(fieldErrors.value.timezone);
      }
      if (draft.mode === "blank") {
        if (!draft.name.trim()) throw Error("请填写角色名字。");
        step.value = "preview";
        persist();
        return;
      }
      if (
        !draft.useDefaultWorld &&
        (!draft.worldResourceId || !draft.worldResourceVersion)
      )
        throw Error("请先选择生活世界。");
      if (draft.mode === "description" && !draft.bio.trim())
        throw Error("请写一段想认识的人的描述。");
      busy.value = true;
      analysisStartedAt.value=Date.now();
      analysisAbort=new AbortController();
      const requestId = draft.requestId;
      draft.analysis = null;
      draft.previewDraft = [];
      previewFields.value = [];
      step.value = "analysis";
      persist();
      const request = creationAnalysisRequest(draft);
      if (request.description) {
        if (draft.speakingStyle?.trim())
          request.description +=
            "\n说话方式期望：" + draft.speakingStyle.trim();
        if (draft.coreValues?.trim())
          request.description += "\n核心价值观期望：" + draft.coreValues.trim();
        if (draft.worldResourceId) {
          const world = await directory.publication(
            draft.worldResourceId,
            draft.worldResourceVersion,
          );
          if (!savedCurrent(requestId)) return;
          request.description +=
            "\n所选世界设定：" +
            JSON.stringify({ name: world.name, body: world.body });
        }
        if (new TextEncoder().encode(request.description).length > 60000)
          throw Error(
            "描述与所选世界设定合计超过 60000 UTF-8 字节，请缩短本次输入。",
          );
      }
      const result = await directory.analyze(request,{signal:analysisAbort.signal});
      if(attempt!==analysisEpoch)return;
      const analysis = parseCreationAnalysis(JSON.stringify(result)),
        saved = recovery.read();
      if (saved.requestId === requestId && saved.name?.trim())
        analysis.core_persona.identity.name = saved.name.trim();
      if (
        !checkpointCreationAnalysis(
          recovery,
          requestId,
          analysis,
          ownerCurrent,
        ) ||
        !savedCurrent(requestId)
      )
        return;
      Object.assign(draft, recovery.read());
      draft.name = analysis.core_persona.identity.name || draft.name;
      initializePreview();
      step.value = "preview";
      persist();
    } catch (cause) {
      if (live() && attempt===analysisEpoch) {
        step.value = "person";
        fail(cause);
        persist();
      }
    } finally {
      if (live() && attempt===analysisEpoch) {busy.value = false;analysisStartedAt.value=null;analysisAbort=null;}
    }
  }
  function abortAnalysisObservation(){
    analysisEpoch++;analysisAbort?.abort();analysisAbort=null;
    busy.value=false;analysisStartedAt.value=null;
  }
  function cancelAnalysis(){
    if(!live()||!busy.value||step.value!=="analysis"||draft.activatedId)return;
    abortAnalysisObservation();step.value="person";
    invalidate();error.value="已停止等待这次整理，原输入已保留。";persist();
  }
  function persistPreview() {
    if (step.value !== "preview" || pending.value || draft.activatedId) return;
    draft.previewDraft = previewFields.value.map((f) => ({
      path: [...f.path],
      value: f.value,
    }));
    persist();
  }
  function applyPreview() {
    if (draft.mode === "blank") return;
    const changes = profileChanges(
      draft.analysis.core_persona,
      previewFields.value,
    );
    const next = applyProfileChanges(draft.analysis.core_persona, changes);
    draft.analysis = { ...draft.analysis, core_persona: next };
    draft.analysisText = JSON.stringify(draft.analysis, null, 2);
    draft.name = next.identity?.name || draft.name;
    draft.timezone = next.identity?.timezone || draft.timezone;
    draft.requestId = randomId();
    persist();
  }
  async function finishWorld() {
    const requestId = draft.requestId,
      fid = draft.activatedId;
    if (!fid || !live()) return;
    const current = () =>
      savedCurrent(requestId) && recovery.read().activatedId === fid;
    if (!draft.groupConfirmed) {
      await directory.move(fid, draft.activationGroupId || "");
      if (!current()) return;
      Object.assign(draft, recovery.read());
      draft.groupConfirmed = true;
      persist();
    }
    const result = await finishCreationWorld({
      client,
      recovery,
      storage,
      owner,
      ownerCurrent: current,
    });
    if (!current() || result.stale) return;
    Object.assign(draft, recovery.read());
    if (!result.configured) error.value = result.warning;
    persist();
  }
  async function activate(retry = false) {
    if (!live() || busy.value) return;
    busy.value = true;
    error.value = "";
    feedback.value = null;
    try {
      if (!draft.activatedId) {
        let value;
        if (retry) value = activation.pending()?.value;
        else {
          applyPreview();
          if (!draft.name?.trim()) throw Error("请在资料预览中填写角色名字。");
          const analysis =
            draft.includeGoals === false
              ? { ...draft.analysis, initial_goals: [], initial_intentions: [] }
              : draft.analysis;
          value = {
            body:
              draft.mode === "blank"
                ? {
                    requestId: draft.requestId,
                    initializationMode: "blank_slate",
                    name: draft.name.trim(),
                    initializationTimezone: draft.timezone,
                  }
                : analysisToActivation(
                    clone(analysis),
                    draft.name.trim(),
                    draft.requestId,
                    draft.timezone,
                  ),
            groupId: draft.groupId,
            worldResourceId: draft.worldResourceId,
            worldResourceVersion: draft.worldResourceVersion,
            useDefaultWorld: !!draft.useDefaultWorld,
          };
        }
        if (!value) throw Error("原创建请求尚未读取，请保留草稿核对。");
        const result = retry
          ? await activation.retry()
          : await activation.submit(clone(value));
        if (!savedCurrent(value.body.requestId)) return;
        Object.assign(draft, recovery.read());
        draft.activatedId = result.id;
        draft.activationDate ??= today(draft.timezone);
        persist();
      }
      step.value = "progress";
      persist();
      await finishWorld();
      if (live()) await refreshDirectory();
    } catch (cause) {
      if (live() && !draft.activatedId && !activation.pending() && [400, 422].includes(cause?.status)) {
        // A definitive pre-admission rejection is editable. Never discard an
        // unknown command outcome, which must keep its original identity.
        invalidate();
        step.value = "person";
        persist();
      }
      fail(cause);
    } finally {
      if (live()) {
        busy.value = false;
        pending.value = activation.pending();
        if (draft.activatedId) await refreshProgress();
      }
    }
  }
  async function refreshProgress() {
    if (!live() || !draft.activatedId) return;
    const token = ++readEpoch,
      requestId = draft.requestId;
    try {
      const result = await readApprovedCreationProgress(client, clone(draft));
      if (!savedCurrent(requestId) || token !== readEpoch) return;
      progress.value = result;
      step.value = result.complete ? "complete" : "progress";
      persist();
    } catch (cause) {
      if (live() && token === readEpoch) fail(cause);
    }
  }
  function reanalyze() {
    try {
      if (busy.value || draft.activatedId) return;
      if (activation.pending()) activation.releaseRejected();
      pending.value = null;
      step.value = "person";
      invalidate();
      error.value = "输入已保留，请重新整理并审阅。";
    } catch (cause) {
      fail(cause);
    }
  }
  async function recoverStage(stage) {
    if (!live() || busy.value || !draft.activatedId) return;
    const fid = draft.activatedId,
      key = `fluctlight-creation:${owner}:${fid}:recover:${stage.key}`;
    let command;
    busy.value = true;
    try {
      if(stage.key==='persona' || stage.personaCanRecover){
        command=persistentCommand({key,storage:()=>storage,execute:(value,idempotencyKey)=>client.recoverPersonaPreparation(fid,{...value,idempotencyKey})});
        if(command.pending())await command.retry();else await command.submit({expectedRevision:stage.revision});
      }
      else if (stage.key === "world") await finishWorld();
      else if (stage.key === "schedule") {
        command=persistentCommand({key,storage:()=>storage,execute:(value,idempotencyKey)=>client.recoverInitialSchedule(fid,{...value,idempotencyKey})});
        if(command.pending())await command.retry();else await command.submit({reason:"恢复原角色的初始日程任务"});
      }
      else if (stage.key === "wardrobe") {
        command = persistentCommand({
          key,
          storage: () => storage,
          execute: (value, idempotencyKey) =>
            client.prepareInventory(fid, { ...value, idempotencyKey }),
        });
        if (command.pending()) await command.retry();
        else {
          const wardrobe = await client.wardrobe(fid);
          if (!live()) return;
          await command.submit({
            expectedRevision: wardrobe.revision,
            reason: "恢复原角色的初始穿搭准备",
          });
        }
      } else if (["reference", "sheet"].includes(stage.key)) {
        command = persistentCommand({
          key,
          storage: () => storage,
          execute: (value, idempotencyKey) =>
            client.generateVisualIdentity(fid, { ...value, idempotencyKey }),
        });
        if (command.pending()) await command.retry();
        else {
          const [identity, brief] = await Promise.all([
            client.visualIdentityManagement(fid),
            client.visualBrief(fid),
          ]);
          if (!live()) return;
          await command.submit({
            expectedVisualRevision: identity.current_revision,
            expectedBriefRevision: brief.revision,
            reason: "恢复原角色的形象准备",
            ...(identity.session?.status === "awaiting_review"
              ? { supersedeSessionId: identity.session.id }
              : {}),
          });
        }
      }
      error.value = "";
    } catch (cause) {
      if (live()) {
        feedback.value = null;
        error.value = errorMessage(cause, "本次恢复结果尚未确认，原请求已保留，可重新读取或重试。");
      }
    } finally {
      if (live()) {
        busy.value = false;
        await refreshProgress();
      }
    }
  }
  function newDraft() {
    if (!draft.activatedId || pending.value || recovery.archived) return;
    recovery.clear();
    const next = recovery.read();
    for (const key of Object.keys(draft)) delete draft[key];
    Object.assign(draft, next);
    progress.value = null;
    step.value = "world";
    pending.value = null;
    previewFields.value = [];
    persist();
  }
  return {
    draft,
    step,
    busy,
    error,
    feedback,
    fieldErrors,
    archived:!!recovery.archived,
    progress,
    previewFields,
    pending,
    live,
    persist,
    persistPreview,
    invalidate,
    selectWorld,
    changeMode,
    next,
    importFile,
    analyze,
    cancelAnalysis,
    analysisStartedAt,
    activate,
    refreshProgress,
    recoverStage,
    reanalyze,
    newDraft,
    dispose() {
      const current = savedCurrent(draft.requestId);
      if (step.value === "analysis") {
        abortAnalysisObservation();
        if (current) {
          step.value = "person";
          invalidate();
        }
      }
      if (current) persist();
      active = false;
      readEpoch++;
    },
  };
}

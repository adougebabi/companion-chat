import { useController } from "@/react-app/page";
import { selector, jsonValueSnapshot } from "@/state/view-model";
import { persistentCommand } from "@/v3/providers/persistent-command.mjs";
import {
  personaPresentation,
  workingPersonaPresentation,
} from "@/approved-ui/profile-presentation.mjs";
import { profileSnapshotFromDetail } from "@/v3/proto/data/profile.js";
import {
  semanticLabel,
  hasSemanticValue,
} from "@/v3/proto/components/semantic-fields.js";
import {
  coreFromDetail,
  profileFields,
  profileChanges,
  profileSaveCommand,
  rebaseProfileFields,
  applyProfileChanges,
} from "@/approved-ui/profile-fields.mjs";
import { errorMessage, stateLabel } from "@/v3/domain-presentation.mjs";
import CharacterDetailPageView from "@/react-app/views/CharacterDetailPage";
export function createProfileModel(lifecycle) {
  const w = lifecycle.workspace,
    fid = w.state.selectedId,
    owner = w.state.session.actorId,
    scope = w.currentScope(),
    detail = lifecycle.cell(null),
    readingDetail = lifecycle.cell(false),
    acceptedRevision = lifecycle.cell(null),
    world = lifecycle.cell(null),
    worldLoading = lifecycle.cell(false),
    worldReady = lifecycle.cell(false),
    worldError = lifecycle.cell(""),
    editing = lifecycle.cell(false),
    fields = lifecycle.cell([]),
    base = lifecycle.cell(null),
    baseline = lifecycle.cell(null),
    saving = lifecycle.cell(false),
    readbackPending = lifecycle.cell(false),
    submittedFields = lifecycle.cell(null),
    pendingProfile = lifecycle.cell(null),
    error = lifecycle.cell(""),
    conflict = lifecycle.cell(false),
    last = lifecycle.cell(null),
    switching = lifecycle.cell(false),
    target = lifecycle.cell(""),
    reason = lifecycle.cell(""),
    confirm = lifecycle.cell(false),
    personaRecord = lifecycle.cell(null),
    personaSubmitted = lifecycle.cell(null),
    personaReceipt = lifecycle.cell(null),
    personaRequestId = lifecycle.cell(""),
    pendingPersona = lifecycle.cell(null),
    personaReadbackPending = lifecycle.cell(false),
    personaFeedback = lifecycle.cell("");
  let active = true,
    epoch = 0,
    reading = false,
    timer,
    personaEpoch = 0,
    worldEpoch = 0;
  const live = () => active && !!fid && w.currentScope() === scope;
  const snapshot = selector(() =>
      profileSnapshotFromDetail(detail.value || {}, w.current.value._directory),
    ),
    core = selector(() => coreFromDetail(detail.value || {})),
    personas = selector(() => snapshot.value.profiles),
    currentPersona = selector(() =>
      personas.value.find((p) => p.id === snapshot.value.activeProfileId),
    ),
    readOnly = selector(
      () =>
        snapshot.value.readOnly ||
        ["retired", "archived", "已退休", "已归档"].includes(
          w.current.value.status,
        ),
    );
  const sections = selector(() =>
    [
      ["identity", "身份与背景", core.value.identity],
      [
        "personality",
        "共同性格",
        detail.value?.personality || detail.value?.core_persona?.personality,
      ],
      ["policy", "表达与相处", core.value.behavioral_policy],
      ["life", "生活设定", core.value.life_profile],
      [
        "working",
        "当前表达画像",
        workingPersonaPresentation(
          snapshot.value.groups.find((group) => group.key === "working")?.value,
        ),
      ],
      [
        "current",
        "此刻状态",
        detail.value?.current_state || detail.value?.inner_state,
      ],
      ["developing", "发展中的自我", detail.value?.developing_self],
    ].filter(([, , value]) => hasSemanticValue(value)),
  );
  const personaValue = (persona) =>
    personaPresentation(persona, {
      ...core.value,
      personality:
        detail.value?.personality || detail.value?.core_persona?.personality,
    });
  const title = selector(
      () => detail.value?.identity?.name || w.current.value.name,
    ),
    subtitle = selector(() =>
      [
        { female: "女", male: "男" }[detail.value?.identity?.gender] ||
          detail.value?.identity?.gender,
        detail.value?.identity?.age != null
          ? detail.value.identity.age + " 岁"
          : "",
        detail.value?.identity?.occupation,
        stateLabel(detail.value?.status, ""),
      ]
        .filter(Boolean)
        .join(" · "),
    );
  const avatar = selector(
    () =>
      snapshot.value.avatar || w.current.value._directory?.avatar?.url || "",
  );
  const governance = (kind) =>
    "#/c/" +
    encodeURIComponent(fid) +
    "/governance" +
    (kind ? "?tab=" + encodeURIComponent(kind) : "");
  const profileChannel = persistentCommand({
    key:
      "fluctlight:approved-command:" +
      JSON.stringify([owner, fid, "profile:" + fid]),
    storage: () => sessionStorage,
    execute: async () => {
      throw Error("请通过原工作区命令恢复");
    },
  });
  function refreshPendingProfile() {
    try {
      pendingProfile.value = profileChannel.pending();
    } catch (cause) {
      error.value = errorMessage(cause);
    }
    return pendingProfile.value;
  }
  async function read({
    preserve = editing.value,
    minimumRevision = readbackPending.value ? acceptedRevision.value : null,
  } = {}) {
    if (!live()) return false;
    const token = ++epoch;
    readingDetail.value = true;
    try {
      const raw = await w.client.detail(fid);
      if (!live() || token !== epoch) return false;
      if (raw?.id !== fid) throw Error("角色资料范围不匹配，保留上次确认内容");
      if (
        Number.isSafeInteger(minimumRevision) &&
        (!Number.isSafeInteger(raw.current_revision) ||
          raw.current_revision < minimumRevision)
      )
        throw Error("已接受修订的版本尚未读回，请只读恢复结果");
      detail.value = raw;
      w.current.value.detail = raw;
      if (preserve) {
        base.value = coreFromDetail(raw);
        baseline.value = raw.current_revision;
        fields.value = rebaseProfileFields(
          base.value,
          fields.value,
          semanticLabel,
          readbackPending.value ? submittedFields.value : null,
        );
      }
      error.value = "";
      readbackPending.value = false;
      submittedFields.value = null;
      return true;
    } catch (cause) {
      if (live() && token === epoch) error.value = errorMessage(cause);
      return false;
    } finally {
      if (live() && token === epoch) readingDetail.value = false;
    }
  }
  function edit() {
    if (!live() || !detail.value || readOnly.value) return;
    base.value = coreFromDetail(detail.value);
    baseline.value = detail.value.current_revision;
    fields.value = profileFields(base.value, semanticLabel);
    editing.value = true;
    error.value = "";
    conflict.value = false;
  }
  async function save(original) {
    if (!live() || saving.value || readOnly.value || readbackPending.value)
      return;
    try {
      const changes = profileChanges(base.value, fields.value);
      if (
        !original &&
        !String(
          fields.value.find((field) => field.path.join(".") === "identity.name")
            ?.value || "",
        ).trim()
      )
        throw Error("请填写角色名字");
      if (!original && !Object.keys(changes).length) {
        editing.value = false;
        return;
      }
      if (!original && !Number.isSafeInteger(baseline.value))
        throw Error("资料版本尚未读取，请重新读取后保存");
      const submitted = JSON.parse(JSON.stringify(fields.value));
      saving.value = true;
      error.value = "";
      const record = w.command(
        "角色详情",
        "profile:" + fid,
        original || {
          changes,
          expectedRevision: baseline.value,
          reason: "所有者编辑角色设定资料",
        },
        profileSaveCommand({
          client: w.client,
          fid,
          live,
        }),
        {
          preAdmissionConflicts: [
            "revision_conflict",
            "foundation_proposal_stale",
          ],
          success: "角色资料已保存，并记录本次修订",
          onSaved: async (receipt, acknowledged) => {
            if (!live()) return;
            refreshPendingProfile();
            readbackPending.value = true;
            acceptedRevision.value = receipt.revision;
            submittedFields.value = acknowledged
              ? profileFields(
                  applyProfileChanges(base.value, acknowledged.changes),
                  semanticLabel,
                )
              : submitted;
            const readback = await read({
              preserve: true,
            });
            if (!readback) {
              if (!live()) return;
              if (
                readbackPending.value ||
                detail.value?.current_revision < receipt.revision
              )
                throw Error(
                  error.value || "已保存资料尚未读回，请重新读取结果",
                );
            }
            if (live()) {
              if (!Object.keys(profileChanges(base.value, fields.value)).length)
                editing.value = false;
              void w.refreshDirectory();
            }
          },
        },
      );
      last.value = record;
      await record.completion;
    } catch (cause) {
      if (live()) {
        error.value = errorMessage(cause);
        conflict.value = cause.status === 409;
      }
    } finally {
      if (live()) {
        saving.value = false;
        refreshPendingProfile();
      }
    }
  }
  async function resumeProfileSave() {
    const pending = refreshPendingProfile();
    if (!pending || !live() || readOnly.value || saving.value) return;
    if (!editing.value) edit();
    if (!editing.value) return;
    return save(jsonValueSnapshot(pending.value));
  }
  async function recoverProfileReadback() {
    if (!live()) return;
    if (last.value?.readbackPending) {
      await last.value.run();
      return;
    }
    const ok = await read({ preserve: true });
    if (
      ok &&
      editing.value &&
      !Object.keys(profileChanges(base.value, fields.value)).length
    )
      editing.value = false;
  }
  async function rebase() {
    try {
      if (last.value) await last.value.release();
      if (!live()) return;
      refreshPendingProfile();
      await read({
        preserve: true,
      });
      conflict.value = false;
    } catch (cause) {
      if (live()) error.value = errorMessage(cause);
    }
  }
  const personaSlot = "fluctlight-profile:" + owner + ":" + fid + ":persona";
  const personaChannel = persistentCommand({
    key:
      "fluctlight:approved-command:" +
      JSON.stringify([owner, fid, "persona-switch"]),
    storage: () => sessionStorage,
    execute: async () => {
      throw Error("请通过原工作区命令恢复");
    },
  });
  function refreshPendingPersona() {
    try {
      pendingPersona.value = personaChannel.pending();
    } catch (cause) {
      error.value = errorMessage(cause);
    }
    return pendingPersona.value;
  }
  function persistPersona() {
    if (!live()) return false;
    try {
      if (
        !target.value &&
        !reason.value &&
        !personaSubmitted.value &&
        !personaRequestId.value
      )
        sessionStorage.removeItem(personaSlot);
      else
        sessionStorage.setItem(
          personaSlot,
          JSON.stringify({
            target: target.value,
            reason: reason.value,
            confirm: confirm.value,
            submitted: personaSubmitted.value,
            requestId: personaRequestId.value,
          }),
        );
      return true;
    } catch (cause) {
      error.value = "人格切换草稿保存失败：" + errorMessage(cause);
      return false;
    }
  }
  function restorePersona() {
    try {
      const saved = JSON.parse(sessionStorage.getItem(personaSlot) || "null");
      if (saved) {
        if (
          typeof saved.target !== "string" ||
          typeof saved.reason !== "string"
        )
          throw Error("切换草稿无法读取，请核对原请求");
        target.value = saved.target;
        reason.value = saved.reason;
        confirm.value = !!saved.confirm;
        personaSubmitted.value = saved.submitted || null;
        personaRequestId.value =
          typeof saved.requestId === "string" ? saved.requestId : "";
        personaReadbackPending.value =
          !!personaRequestId.value && !!personaSubmitted.value;
      }
    } catch (cause) {
      error.value = errorMessage(cause);
    }
    const pending = refreshPendingPersona();
    if (pending && !personaSubmitted.value) {
      personaSubmitted.value = {
        target: pending.value.targetPersonaId,
        reason: pending.value.reason,
        body: pending.value,
      };
      if (!target.value) target.value = pending.value.targetPersonaId;
      if (!reason.value) reason.value = pending.value.reason;
    }
  }
  async function runPersona(body, submitted) {
    if (!live() || readOnly.value || switching.value) return;
    personaEpoch++;
    switching.value = true;
    personaSubmitted.value = submitted;
    personaFeedback.value = "";
    error.value = "";
    if (!persistPersona()) {
      switching.value = false;
      return;
    }
    try {
      const record = w.command(
        "角色详情",
        "persona-switch",
        body,
        async (value, key) => {
          if (!live() || readOnly.value)
            throw Error("账号、角色或可写状态已变化");
          const result = await w.client.switchPersona(fid, {
            ...value,
            idempotencyKey: key,
          });
          if (
            !result?.request_id ||
            result.request?.id !== result.request_id ||
            result.request?.kind !== "persona_switch" ||
            result.request?.fluctlight_id !== fid ||
            !["completed", "pending"].includes(result.status) ||
            !Number.isSafeInteger(result.revision) ||
            !Number.isSafeInteger(result.runtime_revision) ||
            (result.status === "completed" &&
              result.active_profile_id !== value.targetPersonaId)
          )
            throw Error("切换回执未完整确认，原目标与身份仍保留");
          if (live()) {
            personaReceipt.value = jsonValueSnapshot(result);
            personaRequestId.value = result.request_id;
            personaReadbackPending.value = true;
            persistPersona();
          }
          return result;
        },
        {
          onSaved: readPersonaOutcome,
          success: "人格切换请求已读回，请核对实际结果",
        },
      );
      personaRecord.value = record;
      const receipt = await record.completion;
      if (live() && receipt?.status === "completed")
        await readPersonaOutcome(receipt.request || receipt);
    } catch (cause) {
      if (live()) error.value = errorMessage(cause);
    } finally {
      if (live()) {
        switching.value = false;
        refreshPendingPersona();
        persistPersona();
      }
    }
  }
  async function resumePersona() {
    const pending = refreshPendingPersona();
    if (!pending || !live() || readOnly.value || switching.value) return;
    if (pending.rejection) {
      error.value =
        "原请求已明确因版本冲突被拒绝，请通过操作记录核对并释放旧基线。";
      return;
    }
    const body = jsonValueSnapshot(pending.value),
      saved = personaSubmitted.value;
    return runPersona(
      body,
      saved?.body && JSON.stringify(saved.body) === JSON.stringify(body)
        ? jsonValueSnapshot(saved)
        : { target: body.targetPersonaId, reason: body.reason, body },
    );
  }
  async function switchPersona() {
    if (
      !live() ||
      readOnly.value ||
      personaRecord.value?.requestUnresolved ||
      personaReadbackPending.value ||
      switching.value
    )
      return;
    if (refreshPendingPersona()) return resumePersona();
    if (!target.value || !reason.value.trim() || !confirm.value) return;
    if (reason.value.trim().length > 1000) {
      error.value = "切换原因最多1000个字符";
      return;
    }
    if (
      !personas.value.some((p) => p.id === target.value) ||
      target.value === snapshot.value.activeProfileId
    ) {
      error.value = "请选择当前角色可用的其他人格";
      return;
    }
    if (
      snapshot.value.switchTargetIds &&
      !snapshot.value.switchTargetIds.includes(target.value)
    ) {
      error.value = "该人格当前不能通过所有者指令切换";
      return;
    }
    const body = {
      targetPersonaId: target.value,
      expectedRevision: snapshot.value.foundationRevision,
      expectedRuntimeRevision: snapshot.value.runtimeRevision,
      reason: reason.value.trim(),
      confirm: true,
    };
    if (
      !Number.isSafeInteger(body.expectedRevision) ||
      !Number.isSafeInteger(body.expectedRuntimeRevision)
    ) {
      error.value = "人格或运行版本未读取完整，请重新读取";
      return;
    }
    return runPersona(body, {
      target: target.value,
      reason: reason.value,
      body,
    });
  }
  async function readPersonaOutcome(receipt) {
    if (!live()) return false;
    if (receipt) {
      const incoming = receipt.request || receipt;
      const incomingId = incoming.id || incoming.request_id;
      if (
        incomingId &&
        (!personaRequestId.value || incomingId !== personaRequestId.value)
      )
        return false;
      if (incoming.fluctlight_id && incoming.fluctlight_id !== fid) {
        error.value = "原切换请求范围不匹配";
        return false;
      }
      if (incoming.kind && incoming.kind !== "persona_switch") {
        error.value = "原请求类型不匹配";
        return false;
      }
      personaReceipt.value = jsonValueSnapshot(incoming);
    }
    const original = personaReceipt.value,
      submitted = personaSubmitted.value;
    if (!submitted) return false;
    const token = ++personaEpoch,
      result = original?.result || original,
      status = original?.status || result?.status;
    personaReadbackPending.value = true;
    const updated = await read();
    if (!live() || token !== personaEpoch) return false;
    if (!updated) {
      error.value =
        "命令已提交，但最新状态读取失败。原目标和原因已保留，请重新读取，确认前不要重复切换。";
      return false;
    }
    if (["rejected", "failed", "cancelled"].includes(status)) {
      personaReadbackPending.value = false;
      personaFeedback.value =
        "原切换请求" +
        (status === "rejected"
          ? "未被采纳"
          : status === "cancelled"
            ? "已取消"
            : "未完成") +
        "，当前人格以读回记录为准。";
      error.value = result?.error_code
        ? errorMessage(result.error_code, "这次人格切换未完成，请查看原请求。")
        : "这次人格切换未完成，请查看原请求。";
      confirm.value = false;
      personaSubmitted.value = null;
      personaRequestId.value = "";
      persistPersona();
      return true;
    }
    if (
      status !== "completed" ||
      result?.status !== "completed" ||
      result.active_profile_id !== submitted.target ||
      !Number.isSafeInteger(result.runtime_revision) ||
      !Number.isSafeInteger(result.revision)
    ) {
      error.value =
        "原切换尚未完整确认，请继续查询原请求；原目标和原因仍保留。";
      persistPersona();
      return false;
    }
    if (
      snapshot.value.runtimeRevision == null ||
      snapshot.value.foundationRevision == null ||
      snapshot.value.runtimeRevision < result.runtime_revision ||
      snapshot.value.foundationRevision < result.revision
    ) {
      error.value =
        "原请求已完成，已提交的人格版本尚未完整读回，请只读恢复结果。";
      persistPersona();
      return false;
    }
    if (
      snapshot.value.runtimeRevision === result.runtime_revision &&
      snapshot.value.activeProfileId !== result.active_profile_id
    ) {
      error.value = "人格版本与目标状态尚未确认一致，请继续只读核对。";
      return false;
    }
    personaReadbackPending.value = false;
    personaFeedback.value =
      snapshot.value.runtimeRevision === result.runtime_revision
        ? result.disposition === "no_change"
          ? "目标人格已经生效，本次无需新增切换。"
          : "目标人格已生效，已读取最新状态。"
        : "原切换已完成；当前人格已有后续变化，请以当前记录为准。";
    if (
      target.value === submitted.target &&
      reason.value === submitted.reason
    ) {
      target.value = "";
      reason.value = "";
      confirm.value = false;
    }
    personaSubmitted.value = null;
    personaRequestId.value = "";
    persistPersona();
    return true;
  }
  async function readWorld() {
    if (!live()) return;
    const token = ++worldEpoch;
    worldLoading.value = true;
    worldError.value = "";
    try {
      const value = await w.client.world(fid);
      if (live() && token === worldEpoch) {
        world.value = value.world;
        worldReady.value = true;
      }
    } catch (cause) {
      if (live() && token === worldEpoch)
        worldError.value = errorMessage(cause);
    } finally {
      if (live() && token === worldEpoch) worldLoading.value = false;
    }
  }
  async function pollPreparation() {
    if (
      !live() ||
      document.hidden ||
      reading ||
      editing.value ||
      saving.value ||
      switching.value ||
      personaReadbackPending.value ||
      !["pending", "running"].includes(
        detail.value?.persona_preparation?.status,
      )
    )
      return;
    reading = true;
    try {
      await read();
    } finally {
      reading = false;
    }
  }
  async function recoverPreparation() {
    if (
      !live() ||
      saving.value ||
      readOnly.value ||
      !detail.value?.persona_preparation?.can_recover
    )
      return;
    saving.value = true;
    try {
      const record = w.command(
        "角色详情",
        "persona-preparation:" + fid,
        {
          expectedRevision: detail.value.current_revision,
        },
        (value, idempotencyKey) =>
          w.client.recoverPersonaPreparation(fid, {
            ...value,
            idempotencyKey,
          }),
        {
          success: "已继续准备角色资料",
          onSaved: async () => {
            if (live() && !(await read()))
              throw Error(error.value || "准备请求已受理，读取尚未完成");
          },
        },
      );
      last.value = record;
      await record.completion;
    } catch (cause) {
      if (live()) error.value = errorMessage(cause);
    } finally {
      if (live()) saving.value = false;
    }
  }
  const visibility = () => {
    if (!document.hidden) void pollPreparation();
  };
  lifecycle.observe(
    () => [target.value, reason.value, confirm.value],
    () => {
      persistPersona();
    },
  );
  lifecycle.onStart(() => {
    refreshPendingProfile();
    restorePersona();
    void read();
    timer = setInterval(() => void pollPreparation(), 5000);
    document.addEventListener("visibilitychange", visibility);
    void readWorld();
  });
  lifecycle.onDispose(() => {
    active = false;
    epoch++;
    clearInterval(timer);
    document.removeEventListener("visibilitychange", visibility);
  });

  const model = {
    w,
    fid,
    owner,
    scope,
    live,
    detail,
    readingDetail,
    readOnly,
    world,
    worldLoading,
    worldReady,
    worldError,
    readWorld,
    editing,
    fields,
    last,
    saving,
    readbackPending,
    pendingProfile,
    resumeProfileSave,
    error,
    conflict,
    personaRecord,
    personaSubmitted,
    personaReceipt,
    personaRequestId,
    pendingPersona,
    resumePersona,
    personaReadbackPending,
    personaFeedback,
    switching,
    target,
    reason,
    confirm,
    snapshot,
    core,
    personas,
    currentPersona,
    sections,
    personaValue,
    title,
    subtitle,
    avatar,
    governance,
    read,
    edit,
    save,
    rebase,
    switchPersona,
    readPersonaOutcome,
    recoverPreparation,
    recoverProfileReadback,
  };
  model.render = () => <CharacterDetailPageView model={model} />;
  return model;
}
function createCharacterDetailPageController(props, context, lifecycle) {
  return createProfileModel(lifecycle).render;
}

export default function CharacterDetailPage(input = {}) {
  return useController(createCharacterDetailPageController, {
    ...input,
  });
}

import { proxy, snapshot, subscribe } from "valtio/vanilla";
import { persistentCommand } from "../v3/providers/persistent-command.mjs";
import {
  readStudioDraft,
  writeStudioDraft,
  studioWorkingDraftKey,
} from "../v3/proto/components/studio-working-draft.js";
import { studioDraftStorage } from "../v3/proto/components/studio-draft-storage.js";
import {
  worldDefinitionError,
  newWorldDefinition,
} from "../v3/proto/components/world-definition.js";
import { newEventDefinition } from "../v3/proto/components/event-definition.js";
import { newCharacterTemplate } from "../v3/proto/components/character-template.js";
import { newItemDefinition } from "../v3/proto/components/item-definition.js";
import { newPlaceDefinition } from "../v3/proto/components/place-definition.js";
import { newOutfitDefinition } from "../v3/proto/components/outfit-definition.js";
import { newVisualStyle } from "../v3/proto/components/visual-style.js";
import { errorMessage } from "../v3/domain-presentation.mjs";
const clone = (value) => JSON.parse(JSON.stringify(value)),
  same = (a, b) => JSON.stringify(a) === JSON.stringify(b);
export const studioKinds = {
  world: "世界",
  place: "地点",
  item: "物品",
  outfit: "服装方案",
  event_template: "事件模板",
  character_template: "角色模板",
  persona_fragment: "人格片段",
  visual_style: "视觉风格",
  npc: "世界人物",
};
export function studioDefaultBody(kind) {
  return (
    {
      world: newWorldDefinition,
      place: newPlaceDefinition,
      item: newItemDefinition,
      outfit: newOutfitDefinition,
      event_template: newEventDefinition,
      character_template: newCharacterTemplate,
      visual_style: newVisualStyle,
      persona_fragment: () => ({ schema: "persona-fragment.v1", changes: {} }),
      npc: () => ({ role: "", background: "" }),
    }[kind]?.() || {}
  );
}
export function createStudioEditor({
  owner,
  resourceId,
  kind = "event_template",
  service,
  ownerCurrent,
  navigate,
  storage = studioDraftStorage,
  jobId = "",
  allowNpc = false,
}) {
  const creating = resourceId === "new",
    slot = `fluctlight-studio:${owner}:${resourceId}`;
  const state = proxy({
    resource: null,
    draft: null,
    pending: null,
    loading: false,
    busy: false,
    error: "",
    notice: "",
    preview: null,
    jobId,
    definitionEpoch: 0,
    readbackPending: false,
    rebaseReview: null,
    publishReview: null,
    previewLoading: false,
    lastApplication: null,
  });
  let active = false,
    readEpoch = 0,
    previewEpoch = 0;
  const live = () => active && ownerCurrent();
  const receiptKey = slot + ":receipt";
  function cacheReceipt(value, result, key) {
    if (!ownerCurrent()) return;
    const pending = storage().getItem(slot + ":command");
    if (!pending || JSON.parse(pending).idempotencyKey !== key) return;
    const record = { value: clone(value), result: clone(result), key };
    storage().setItem(receiptKey, JSON.stringify(record));
    if (creating && value.operation === "save") {
      const draft = readStudioDraft(
        owner,
        resourceId,
        null,
        { kind, body: studioDefaultBody(kind) },
        storage,
      );
      Object.assign(draft, {
        acceptedResourceId: result.id,
        expectedVersion: result.draft_version || result.published_version,
      });
      writeStudioDraft(owner, resourceId, draft, storage);
      writeStudioDraft(owner, result.id, draft, storage);
      storage().setItem(
        `fluctlight-studio:${owner}:${result.id}:receipt`,
        JSON.stringify(record),
      );
    }
  }
  function validateReceipt(value, result) {
    if (!result || typeof result.id !== "string" || !result.id)
      throw Error("原操作回执缺少真实标识，请核对原提交。");
    if (
      value.operation === "save" &&
      ((value.id && result.id !== value.id) ||
        typeof (result.draft_version || result.published_version) !== "string")
    )
      throw Error("保存回执不完整，请核对原提交。");
    if (
      value.operation === "apply" &&
      (result.resource_id !== value.id ||
        result.resource_version !== value.body.resourceVersion ||
        !(
          result.applied === true ||
          (result.target_kind === "foundation" &&
            result.status === "proposed" &&
            result.proposal_id)
        ))
    )
      throw Error("应用回执的精确资源、版本或结果尚未确认。");
    if (value.operation === "publish" && result.id !== value.id)
      throw Error("发布回执资源不匹配。");
    return result;
  }
  const command = persistentCommand({
    key: slot + ":command",
    storage,
    execute: async (value, key) => {
      if (!live()) throw Error("会话或资源已切换，请返回原范围核对");
      let result;
      if (value.operation === "save") {
        result = await service.save(value.id, {
          ...value.body,
          idempotencyKey: key,
        });
      } else if (value.operation === "publish")
        result = await service.publish(value.id, value.revision, key);
      else if (value.operation === "prepare")
        result = await service.createJob({
          ...value.body,
          idempotencyKey: key,
        });
      else if (value.operation === "apply")
        result = await service.apply(value.id, {
          ...value.body,
          idempotencyKey: key,
        });
      else throw Error("未知创作操作");
      validateReceipt(value, result);
      cacheReceipt(value, result, key);
      return result;
    },
  });
  const syncPending = () => (state.pending = command.pending());
  function persist() {
    if (!live() || !state.draft) return false;
    try {
      writeStudioDraft(owner, resourceId, state.draft, storage);
      if (creating && state.draft.acceptedResourceId)
        writeStudioDraft(
          owner,
          state.draft.acceptedResourceId,
          state.draft,
          storage,
        );
      return true;
    } catch (cause) {
      state.error = "草稿保存失败：" + errorMessage(cause);
      return false;
    }
  }
  function adoptDefinition(value) {
    if (!live() || !state.draft) return;
    state.definitionEpoch++;
    edit({ name: value.name, body: clone(value.body), bodyText: undefined });
  }
  function parseBodyText() {
    if (state.draft?.bodyText === undefined) return true;
    try {
      const value = JSON.parse(state.draft.bodyText);
      if (!value || typeof value !== "object" || Array.isArray(value))
        throw Error("完整定义必须是 JSON 对象");
      state.definitionEpoch++;
      edit({ body: value, bodyText: undefined });
      return true;
    } catch (cause) {
      state.error = errorMessage(cause, "JSON 尚未修正，输入已保留。");
      return false;
    }
  }
  const content = () => ({
    name: state.draft.name,
    body: clone(state.draft.body),
    kind: state.draft.kind,
  });
  const dirty = () =>
    !state.resource ||
    !state.draft ||
    state.draft.bodyText !== undefined ||
    !same(
      {
        name: state.draft.name,
        body: state.draft.body,
        kind: state.draft.kind,
      },
      {
        name: state.resource.name,
        body: state.resource.body || {},
        kind: state.resource.kind,
      },
    );
  async function load() {
    const token = ++readEpoch;
    state.loading = true;
    try {
      const record = storedReceipt();
      if (record) {
        if (!state.draft)
          state.draft = readStudioDraft(
            owner,
            resourceId,
            null,
            { kind, body: studioDefaultBody(kind) },
            storage,
          );
        syncPending();
        await finishReceipt(record, token);
        return;
      }

      const latest = creating ? null : await service.resource(resourceId);
      if (!live() || token !== readEpoch) return;
      if (
        latest &&
        (latest.id !== resourceId ||
          (latest.owner_actor_id && latest.owner_actor_id !== owner))
      )
        throw Error("创作资源范围不匹配");
      if (latest?.kind === "npc" && !allowNpc) {
        navigate(
          "/studio/npc/" +
            encodeURIComponent(latest.id) +
            (state.jobId ? "?job=" + encodeURIComponent(state.jobId) : ""),
        );
        return;
      }
      const nextDraft = !state.draft
        ? readStudioDraft(
            owner,
            resourceId,
            latest,
            { kind, body: studioDefaultBody(kind) },
            storage,
          )
        : null;
      state.resource = latest ? clone(latest) : null;
      if (nextDraft) state.draft = nextDraft;
      syncPending();
      state.preview = null;
      previewEpoch++;
      state.readbackPending = false;
      state.error = "";
    } catch (cause) {
      if (live() && token === readEpoch) state.error = errorMessage(cause);
    } finally {
      if (live() && token === readEpoch) state.loading = false;
    }
  }
  function edit(values) {
    if (!live() || !state.draft || state.resource?.status === "archived")
      return;
    Object.assign(state.draft, values);
    persist();
  }
  function setKind(next) {
    if (
      !creating ||
      state.busy ||
      state.readbackPending ||
      state.draft?.acceptedResourceId ||
      command.pending()
    )
      return;
    state.definitionEpoch++;
    edit({ kind: next, body: studioDefaultBody(next), bodyText: undefined });
  }
  function setTarget(target) {
    edit({ target });
    state.preview = null;
    state.previewLoading = false;
    previewEpoch++;
  }
  function storedReceipt() {
    const raw = storage().getItem(receiptKey);
    if (!raw) return null;
    const record = JSON.parse(raw);
    if (!record?.value || !record?.result || typeof record.key !== "string")
      throw Error("原创作回执无法读取，请保留记录核对。");
    return record;
  }
  async function finishReceipt(record, token) {
    const { value, result } = record;
    validateReceipt(value, result);
    state.readbackPending = true;
    if (value.operation === "save") {
      state.draft.expectedVersion =
        result.draft_version || result.published_version;
      if (creating) state.draft.acceptedResourceId = result.id;
      persist();
      const actual = await service.resource(result.id);
      if (!live() || token !== readEpoch) return;
      if (
        actual.id !== result.id ||
        (actual.owner_actor_id && actual.owner_actor_id !== owner) ||
        (Number.isSafeInteger(result.revision) &&
          actual.revision < result.revision)
      )
        throw Error("保存后的真实资源版本尚未读回。");
      const unchanged =
        state.draft.bodyText === undefined &&
        same(
          content(),
          value.input || {
            name: value.body.name,
            body: value.body.body,
            kind: value.body.kind,
          },
        );
      if (unchanged) {
        state.draft.name = result.name ?? value.body.name;
        state.draft.body = clone(result.body ?? value.body.body);
      }
      state.resource = clone(actual);
      persist();
      if (creating) writeStudioDraft(owner, result.id, state.draft, storage);
      state.notice = unchanged
        ? "定义草稿已保存并读回。"
        : "原草稿已保存，后来输入的修改仍保留。";
    } else if (value.operation === "publish") {
      const actual = await service.resource(value.id);
      if (!live() || token !== readEpoch) return;
      if (actual.id !== value.id || !actual.published_version)
        throw Error("发布后的真实资源尚未读回。");
      if (value.version) {
        const publication = await service.publication(value.id, value.version);
        if (!live() || token !== readEpoch) return;
        if (
          publication.id !== value.id ||
          publication.version !== value.version ||
          publication.immutable !== true
        )
          throw Error("原精确发布版本尚未读回。");
      }
      state.resource = clone(actual);
      state.notice = "已发布确认的保存版本。应用仍需独立确认。";
    } else if (value.operation === "prepare") {
      const actual = await service.job(result.id);
      if (!live() || token !== readEpoch) return;
      if (
        actual.id !== result.id ||
        actual.resource_id !== value.id ||
        (actual.owner_actor_id && actual.owner_actor_id !== owner)
      )
        throw Error("原解析任务尚未完整读回。");
      state.jobId = result.id;
      state.notice = "解析任务已受理。请审阅结果，再采用到草稿、保存和发布。";
    } else {
      const actual = await service.resource(value.id);
      if (!live() || token !== readEpoch) return;
      if (
        actual.id !== value.id ||
        (actual.owner_actor_id && actual.owner_actor_id !== owner)
      )
        throw Error("应用后的资源来源尚未读回。");
      state.resource = clone(actual);
      state.lastApplication = {
        result: clone(result),
        fid: value.body.fluctlightId,
        version: value.body.resourceVersion,
        kind: value.kind || actual.kind,
      };
      if (state.draft.target === value.body.fluctlightId) state.preview = null;
      state.notice =
        result.target_kind === "foundation"
          ? "身份核心提议已保存，接受后才会生效。"
          : result.target_kind === "wardrobe_outfit"
            ? "服饰组合已保存，穿着需要独立请求。"
            : result.target_kind === "world_item"
              ? "物品已登记到世界目录，尚未取得。"
              : result.target_kind === "visual_brief"
                ? "视觉参考已保存，新的生成会话会使用此版本。"
                : "已确认原应用回执，请到目标记录核对当前值。";
    }
    if (!live() || token !== readEpoch) return;
    if (storedReceipt()?.key === record.key) storage().removeItem(receiptKey);
    if (creating && value.operation === "save") {
      const actualKey = `fluctlight-studio:${owner}:${result.id}:receipt`;
      const other = storage().getItem(actualKey);
      if (other && JSON.parse(other).key === record.key)
        storage().removeItem(actualKey);
      storage().removeItem(studioWorkingDraftKey(owner, resourceId));
      navigate("/studio/edit/" + encodeURIComponent(result.id));
    } else if (value.operation === "save") {
      const sourceKey = `fluctlight-studio:${owner}:new:receipt`,
        other = storage().getItem(sourceKey);
      if (other && JSON.parse(other).result?.id === result.id) {
        storage().removeItem(sourceKey);
        const raw = storage().getItem(studioWorkingDraftKey(owner, "new"));
        if (raw && JSON.parse(raw).acceptedResourceId === result.id)
          storage().removeItem(studioWorkingDraftKey(owner, "new"));
      }
    }
    state.readbackPending = false;
    state.error = "";
  }
  async function submit(value, retry = false) {
    if (!live() || state.busy) return;
    if (storedReceipt()) {
      state.error = "原操作已有回执，请先恢复实际结果读取。";
      state.readbackPending = true;
      return;
    }
    if (!retry && command.pending()) {
      state.error = "请先核对原提交，当前编辑仍保留。";
      return;
    }
    state.busy = true;
    state.error = "";
    state.notice = "";
    let acknowledged = false;
    try {
      await (retry ? command.retry() : command.submit(value));
      acknowledged = true;
      if (!live()) return;
      const record = storedReceipt();
      if (!record) throw Error("回执恢复记录缺失，请核对原操作。");
      await finishReceipt(record, ++readEpoch);
    } catch (cause) {
      if (live())
        state.error =
          (acknowledged ? "操作已有回执，后续读取未完成：" : "") +
          errorMessage(cause);
    } finally {
      if (live()) {
        state.busy = false;
        syncPending();
      }
    }
  }
  function save() {
    if (
      !state.draft ||
      state.busy ||
      state.readbackPending ||
      command.pending() ||
      !parseBodyText()
    )
      return;
    if (!state.draft.name.trim()) {
      state.error = "请填写资源名称。";
      return;
    }
    if (!state.draft.reason.trim()) {
      state.error = "请填写保存原因。";
      return;
    }
    const failure =
      state.draft.kind === "world"
        ? worldDefinitionError(state.draft.body)
        : "";
    if (failure) {
      state.error = failure;
      return;
    }
    if (!persist()) return;
    return submit({
      operation: "save",
      input: content(),
      id: state.resource?.id || null,
      body: {
        kind: state.draft.kind,
        name: state.draft.name,
        body: clone(state.draft.body),
        expectedVersion: state.draft.expectedVersion,
        reason: state.draft.reason,
      },
    });
  }
  function retry() {
    const pending = command.pending();
    if (!pending) {
      state.pending = null;
      state.error = "原提交记录已变化，请重新读取资源。";
      return;
    }
    return submit(pending.value, true);
  }
  async function reviewRebase() {
    if (
      !live() ||
      state.busy ||
      creating ||
      (command.pending() && !command.pending().rejection)
    )
      return;
    state.busy = true;
    try {
      const latest = await service.resource(resourceId);
      if (live())
        state.rebaseReview = {
          resource: latest,
          key: command.pending()?.idempotencyKey,
        };
    } catch (cause) {
      if (live()) state.error = errorMessage(cause);
    } finally {
      if (live()) state.busy = false;
    }
  }
  function confirmRebase() {
    const review = state.rebaseReview;
    if (!live() || !review || command.pending()?.idempotencyKey !== review.key)
      return;
    if (command.pending()?.rejection) command.releaseRejected();
    state.resource = review.resource;
    state.draft.expectedVersion =
      review.resource.draft_version || review.resource.published_version;
    state.rebaseReview = null;
    state.preview = null;
    previewEpoch++;
    persist();
    syncPending();
    state.notice = "已明确采用最新版本基线，当前草稿内容仍保留。";
  }
  function reviewPublish() {
    if (
      !state.resource ||
      state.busy ||
      dirty() ||
      !state.resource.draft_version ||
      state.resource.draft_version === state.resource.published_version
    )
      return;
    state.publishReview = { resource: clone(state.resource), draft: content() };
  }
  function confirmPublish() {
    const review = state.publishReview;
    if (!review || !live()) return;
    state.publishReview = null;
    if (!same(review.draft, content())) {
      state.error = "草稿已经变化，请重新核对保存版本。";
      return;
    }
    return submit({
      operation: "publish",
      id: review.resource.id,
      revision: review.resource.revision,
      version: review.resource.draft_version,
    });
  }
  async function readPreview() {
    if (!state.draft?.target || !state.resource?.published_version) {
      state.error = "请选择目标角色，并先发布定义。";
      return;
    }
    const fid = state.draft.target,
      id = state.resource.id,
      version = state.resource.published_version,
      token = ++previewEpoch;
    state.error = "";
    state.previewLoading = true;
    try {
      const [result, publication] = await Promise.all([
        service.preview(fid, id, version),
        service.publication(id, version),
      ]);
      if (!live() || token !== previewEpoch || fid !== state.draft.target)
        return;
      if (
        publication.id !== id ||
        publication.version !== version ||
        publication.immutable !== true
      )
        throw Error("精确发布版本尚未完整确认。");
      let targetWorld = null,
        lockedFields = {};
      if (publication.kind === "place" && result.runtime_id)
        lockedFields.name = "已有地点保留原名称身份，不能由定义升级改名。";
      if (publication.kind === "npc") {
        targetWorld = await service.world(fid);
        if (!live() || token !== previewEpoch) return;
        if (targetWorld.world?.fluctlight_id !== fid)
          throw Error("目标运行世界范围不匹配。");
        if (result.runtime_id) {
          const npc = await service.npc(fid, result.runtime_id);
          if (!live() || token !== previewEpoch) return;
          if (
            npc.id !== result.runtime_id ||
            npc.world_id !== targetWorld.world.id
          )
            throw Error("人物实例已不在预览世界。");
          for (const field of ["name", "role", "background"])
            if (Object.hasOwn(npc.local_overrides || {}, field))
              lockedFields[field] =
                "这项由运行人物的局部资料覆盖，应用将保留实际局部值。";
        }
      }
      state.preview = {
        ...result,
        publication,
        fid,
        version,
        viewEpoch: token,
        choices: {},
        memberItems: (result.members || []).map(() => ""),
        lockedFields,
        targetWorld,
        homePlaceId: "",
      };
    } catch (cause) {
      if (live() && token === previewEpoch) state.error = errorMessage(cause);
    } finally {
      if (live() && token === previewEpoch) state.previewLoading = false;
    }
  }
  async function prepare() {
    const preview = state.preview;
    if (!preview?.preparation_required) return;
    try {
      const publication = await service.publication(
        state.resource.id,
        preview.version,
      );
      if (!live() || state.preview !== preview) return;
      return submit({
        operation: "prepare",
        id: state.resource.id,
        body: {
          resourceId: state.resource.id,
          resourceRevision: state.resource.revision,
          kind: "assistant",
          purpose: "foundation_prepare",
          resourceVersion: preview.version,
          name: publication.name,
          body: publication.body,
          instruction:
            "按精确已发布文本解析完整结构化角色定义，保持来源意图，供所有者审阅后形成Foundation应用提案",
          reason: "所有者明确解析已发布文本模板",
        },
      });
    } catch (cause) {
      if (live()) state.error = errorMessage(cause);
    }
  }
  function chooseDifference(field, decision, viewEpoch) {
    const p = state.preview;
    if (
      !live() ||
      !p ||
      p.viewEpoch !== viewEpoch ||
      !p.diff.some((row) => row.field === field) ||
      !["keep", "override"].includes(decision)
    )
      return;
    if (p.lockedFields?.[field] && decision === "override") return;
    p.choices = Object.fromEntries([
      ...Object.entries(p.choices).filter(([key]) => key !== field),
      [field, decision],
    ]);
  }
  function chooseMember(index, value, viewEpoch) {
    const p = state.preview;
    if (
      !live() ||
      !p ||
      p.viewEpoch !== viewEpoch ||
      !Number.isInteger(index) ||
      index < 0 ||
      index >= p.memberItems.length
    )
      return;
    p.memberItems[index] = value;
  }
  async function apply() {
    const p = state.preview;
    if (!p || !live() || (!p.diff?.length && !p.create_required)) return;
    if (
      p.runtime_id &&
      p.diff.some(
        (row) =>
          !Object.hasOwn(p.choices, row.field) ||
          !["keep", "override"].includes(p.choices[row.field]),
      )
    ) {
      state.error = "请逐项选择如何处理实际差异。";
      return;
    }
    if (
      p.target_kind === "wardrobe_outfit" &&
      p.choices.members !== "keep" &&
      p.memberItems.some((id) => !id)
    ) {
      state.error = "请为每个成员明确选择实际库存服饰。";
      return;
    }
    if (p.publication?.kind === "npc") {
      let world;
      try {
        world = await service.world(p.fid);
      } catch (cause) {
        if (live()) state.error = errorMessage(cause);
        return;
      }
      if (!live() || state.preview?.viewEpoch !== p.viewEpoch) return;
      if (
        world.world?.id !== p.targetWorld?.world?.id ||
        world.active_binding?.revision !==
          p.targetWorld?.active_binding?.revision
      ) {
        state.error = "目标运行世界已变化，请重新读取人物应用预览。";
        return;
      }
    }
    return submit({
      operation: "apply",
      kind: p.publication?.kind,
      id: state.resource.id,
      body: {
        fluctlightId: p.fid,
        resourceVersion: p.version,
        expectedInstanceRevision: p.instance_revision,
        expectedRuntimeRevision: p.runtime_revision ?? 0,
        ...(p.target_kind === "wardrobe_outfit"
          ? {
              profileId: p.profile_id,
              expectedWardrobeRevision: p.wardrobe_revision,
              memberItems: p.members.map((member, index) => ({
                resourceId: member.resource_id,
                resourceVersion: member.resource_version,
                itemId: p.memberItems[index],
              })),
            }
          : {}),
        ...(p.publication?.kind === "npc" && !p.runtime_id
          ? { homePlaceId: p.homePlaceId || null, routine: [] }
          : {}),
        ...(p.world_id
          ? {
              worldId: p.world_id,
              expectedWorldRevision: p.world_binding_revision,
            }
          : {}),
        choices: Object.entries(p.choices).map(([field, decision]) => ({
          field,
          decision,
        })),
        reason: "所有者确认差异后应用已发布资源",
      },
    });
  }
  function setNPCHome(id, viewEpoch) {
    const p = state.preview;
    if (
      !live() ||
      !p ||
      p.viewEpoch !== viewEpoch ||
      p.publication?.kind !== "npc" ||
      p.runtime_id
    )
      return;
    if (
      id &&
      !p.targetWorld?.places?.some(
        (place) => place.id === id && place.status === "active",
      )
    )
      return;
    p.homePlaceId = id;
  }
  return {
    state,
    subscribe: (listener) => subscribe(state, listener),
    getSnapshot: () => snapshot(state),
    live,
    dirty,
    persist,
    edit,
    adoptDefinition,
    parseBodyText,
    setKind,
    setTarget,
    load,
    save,
    retry,
    reviewRebase,
    confirmRebase,
    reviewPublish,
    confirmPublish,
    readPreview,
    prepare,
    apply,
    chooseDifference,
    chooseMember,
    setNPCHome,
    start() {
      active = true;
      return load();
    },
    dispose() {
      active = false;
      readEpoch++;
      previewEpoch++;
    },
  };
}

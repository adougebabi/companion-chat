import { readInitializationProgress } from "../v3/initialization-progress.mjs";
import { errorMessage } from "../v3/domain-presentation.mjs";
import { wardrobePreparationStage } from './wardrobe-preparation.mjs';
async function part(run) {
  try {
    return { state: "ready", value: await run() };
  } catch (error) {
    return { state: "unknown", error, errorMessage: errorMessage(error) };
  }
}
const failed = (status) =>
  ["failed", "cancelled", "dead_letter", "deadletter"].includes(status);
const statusOf = (status) =>
  failed(status)
    ? "failed"
    : ["running", "started", "generating", "character_sheet_pending"].includes(
          status,
        )
      ? "running"
      : "pending";
export async function readApprovedCreationProgress(client, draft) {
  const fid = draft.activatedId;
  const [visual, wardrobe, schedule, lifecycle, world,detail] = await Promise.all([
    readInitializationProgress(client, fid),
    part(() => client.inventoryPreparation(fid)),
    part(() =>
      client.schedules(fid, {
        from: draft.activationDate,
        to: draft.activationDate,
      }),
    ),
    part(() => client.lifecycleDiagnostics({ fluctlightId: fid, limit: 100 })),
    part(() => client.world(fid)),
    part(() => client.detail(fid)),
  ]);
  const identity =
      visual.identity.state === "ready" ? visual.identity.value : null,
    session = identity?.session,
    waitingForReview = session?.status === "awaiting_review",
    task = visual.task.state === "ready" ? visual.task.value : null,
    visualState = statusOf(session?.status || task?.status),
    visualReason =
      failed(session?.status) || failed(task?.status)
        ? errorMessage(
            {
              code:
                session?.error_code ||
                session?.last_error ||
                task?.lastError ||
                task?.error_code,
            },
            "形象准备尚未完成，请检查图片服务后恢复。",
          )
        : visual.identity.state === "failed"
          ? visual.identity.message
          : "";
  const [reference, sheet] = await Promise.all([
    identity?.canonical_asset_id
      ? part(() => client.ownerMediaAsset(identity.canonical_asset_id, fid))
      : null,
    identity?.character_sheet_asset_id
      ? part(() =>
          client.ownerMediaAsset(identity.character_sheet_asset_id, fid),
        )
      : null,
  ]);
  const imageStage = (key, title, asset) => ({
    key,
    title,
    state: asset?.value?.readable
      ? "ready"
      : asset?.state === "unknown"
        ? "unknown"
        : visual.identity.state === "failed"
          ? "unknown"
          : visualState,
    note: asset?.value?.readable
      ? "已读回可以显示的图片"
      : asset?.errorMessage || visualReason || (waitingForReview
        ? key === "reference"
          ? "当前候选等待明确评审；接受后才会成为规范参考。"
          : "等待候选被接受后，继续准备角色设定图。"
        : "等待实际图片任务与产物"),
    href: `#/c/${encodeURIComponent(fid)}/media/identity`,
    linkLabel: waitingForReview && !asset?.value?.readable && asset?.state !== "unknown"
      ? "前往审阅候选" : undefined,
    configSection: "图片（ComfyUI）",
    retryable: visualState === "failed" && !!identity,
  });
  const scheduleTask = lifecycle.value?.workflowIntents
      ?.filter((t) => t.intentType === "schedule.current_day")
      .sort(
        (a, b) =>
          Date.parse(b.createdAt || b.startedAt || 0) -
          Date.parse(a.createdAt || a.startedAt || 0),
      )[0],
    accepted = schedule.value?.days?.find(
      (d) => d.local_date === draft.activationDate,
    )?.schedule;
  const preparation = wardrobe.value;
  const persona=detail.value?.persona_preparation;
  const worldConfirmed =
    typeof draft.worldRuntimeId === "string" &&
    draft.worldRuntimeId.trim().length > 0 &&
    world.value?.world?.id === draft.worldRuntimeId;
  const stages = [
    {
      key: "analysis",
      title: draft.mode === "blank" ? "确认空白角色资料" : "整理人物设定",
      state: "ready",
      note:
        draft.mode === "blank"
          ? "使用服务端空白初始设定"
          : "已确认本次审阅的资料",
    },
    {
      key: "world",
      title: "创建角色与生活世界",
      state:
        world.state === "unknown" || detail.state === "unknown"
          ? "unknown"
          : persona?.status === "failed" ? "failed"
          : worldConfirmed &&
              draft.groupConfirmed && persona?.status === 'ready'
            ? "ready"
            : persona?.status === 'running' ? 'running' : "pending",
      note:
        world.errorMessage || detail.errorMessage ||
        (persona?.status === 'failed' ? '角色已建立，人格资料整理未完成，可继续准备。' : persona?.status !== 'ready' ? '世界确认后将在后台整理人格资料；后续步骤按顺序继续。' : '') ||
        (!draft.groupConfirmed
          ? "原角色已建立，正在核对世界与分组"
          : "以实际选用的独立世界为准"),
      personaCanRecover: !!persona?.can_recover,
      revision: detail.value?.current_revision,
      retryable: !!persona?.can_recover || !(
        worldConfirmed && draft.groupConfirmed
      ),
    },
    wardrobePreparationStage(preparation,wardrobe.errorMessage),
    imageStage("reference", "形象参考图", reference),
    imageStage("sheet", "角色设定图", sheet),
    {
      key: "avatar",
      title: "显示头像",
      state:
        visual.avatar.state === "failed"
          ? "unknown"
          : visual.avatar.value?.avatar?.url
            ? "ready"
            : visualState,
      note:
        visual.avatar.state === "failed"
          ? visual.avatar.message
          : visual.avatar.value?.avatar?.url
            ? "已读回可显示头像"
            : visualReason || "等待头像生成并读回",
      href: `#/c/${encodeURIComponent(fid)}/media/avatar`,
      configSection: "图片（ComfyUI）",
    },
    {
      key: "schedule",
      title: "初始日程",
      state: accepted
        ? "ready"
        : schedule.state === "unknown"
          ? "unknown"
          : statusOf(scheduleTask?.status),
      note: accepted
        ? "已读回这一天的日程"
        : schedule.errorMessage || failed(scheduleTask?.status)
          ? schedule.errorMessage ||
            errorMessage(
              { code: scheduleTask?.lastError },
              "日程准备未完成，请查看原任务并恢复。",
            )
          : "等待已登记的日程任务",
      href: scheduleTask
        ? "#/diag/task/" + encodeURIComponent(scheduleTask.intentId)
        : undefined,
      configSection: "模型用途",
      retryable: !accepted && failed(scheduleTask?.status),
    },
  ];
  // Backend bootstrap is serial: persona -> wardrobe -> schedule -> images.
  // A missing renderer must not hold non-image life preparation indefinitely.
  stages.splice(3, 0, stages.pop());
  let blocker;
  for (const stage of stages) {
    if (blocker && stage.state === 'pending') {
      stage.state = 'blocked';
      stage.blockedBy = blocker.key;
      stage.note = `等待恢复「${blocker.title}」，此环节尚未开始。`;
      stage.retryable = false;
    }
    if (!blocker && stage.state === 'failed') blocker = stage;
  }
  return {
    fid,
    stages,
    complete: stages.every((s) => s.state === "ready"),
    blocker,
    visual,
    wardrobe,
    schedule,
    world,
  };
}

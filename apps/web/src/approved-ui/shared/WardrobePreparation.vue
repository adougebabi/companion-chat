<script setup>
import { computed, ref, onMounted, onUnmounted } from "vue";
import { useWorkspace } from "../workspace.mjs";
import { errorMessage, stateLabel } from "../../v3/domain-presentation.mjs";
import PreparationSteps from "./PreparationSteps.vue";
import { ActionFeedback } from "./components.mjs";
const w = useWorkspace(),
  fid = w.state.selectedId,
  scope = w.currentScope(),
  preparation = ref(null),
  readError = ref(""),
  busy = ref(false);
let active = true,
  reading = false,
  timer;
const live = () => active && w.currentScope() === scope;
const stages = computed(() => {
  const p = preparation.value;
  if (!p)
    return [
      {
        key: "wardrobe",
        title: "初始衣柜",
        state: "unknown",
        note: readError.value || "正在读取准备状态",
      },
    ];
  return [
    {
      key: "wardrobe",
      title: "初始衣柜",
      state:
        p.status === "completed"
          ? "ready"
          : p.can_recover || p.status === "failed"
            ? "failed"
            : ["generating", "awaiting_wear"].includes(p.status)
              ? "running"
              : "pending",
      note:
        p.can_recover || p.status === "failed"
          ? errorMessage(
              { code: p.error_code, message: p.result?.failure_reason },
              "初始穿搭准备未完成，请检查模型服务后恢复。",
            )
          : stateLabel(p.status, "等待准备任务"),
      configSection: "模型用途",
      href: p.task_id
        ? "#/diag/task/" + encodeURIComponent(p.task_id)
        : undefined,
      retryable: p.can_recover || p.status === "not_started",
    },
  ];
});
async function load() {
  if (!live() || reading) return;
  reading = true;
  try {
    const value = await w.services.inventoryClient.preparation(fid);
    if (live()) {
      preparation.value = value;
      readError.value = "";
    }
  } catch (cause) {
    if (live()) readError.value = errorMessage(cause);
  } finally {
    reading = false;
  }
}
async function recover() {
  if (busy.value) return;
  busy.value = true;
  try {
    const latest = await w.services.inventoryClient.current(fid);
    if (!live()) return;
    const record = w.command(
      "衣柜",
      "wardrobe-preparation",
      {
        expectedRevision: latest.revision,
        reason: "恢复当前角色的初始穿搭准备",
      },
      (body, key) =>
        w.client.prepareInventory(fid, { ...body, idempotencyKey: key }),
      {
        onSaved: async () => {
          if (live()) {
            await load();
            void w.loadWardrobe();
          }
        },
        success: "衣柜准备请求已受理，实际结果将继续更新",
      },
    );
    await record.completion;
  } catch (cause) {
    if (live()) readError.value = errorMessage(cause);
  } finally {
    if (live()) busy.value = false;
  }
}
const visibility = () => {
  if (!document.hidden) void load();
};
onMounted(() => {
  void load();
  timer = setInterval(() => {
    if (
      !document.hidden &&
      preparation.value &&
      ["queued", "generating", "awaiting_wear"].includes(
        preparation.value.status,
      ) &&
      !preparation.value.can_recover
    )
      void load();
  }, 5000);
  document.addEventListener("visibilitychange", visibility);
});
onUnmounted(() => {
  active = false;
  clearInterval(timer);
  document.removeEventListener("visibilitychange", visibility);
});
</script>
<template>
  <details
    class="card approved-preparation"
    :open="
      !!readError ||
      preparation?.status === 'failed' ||
      preparation?.can_recover
    "
  >
    <summary>
      初始穿搭准备
      <small>{{
        preparation ? stateLabel(preparation.status, "状态待确认") : "读取中"
      }}</small>
    </summary>
    <PreparationSteps :stages="stages" :busy="busy" @recover="recover" />
    <p v-if="readError" class="inline-error" role="alert">{{ readError }}</p>
    <ActionFeedback :labels="['衣柜']" /><button
      class="text-button"
      type="button"
      @click="load"
    >
      重新读取准备状态
    </button>
  </details>
</template>

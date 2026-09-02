<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { Plus, X } from "@lucide/vue";

import Badge from "@/components/ui/badge/Badge.vue";
import Button from "@/components/ui/button/Button.vue";
import Dialog from "@/components/ui/dialog/Dialog.vue";
import DialogClose from "@/components/ui/dialog/DialogClose.vue";
import DialogContent from "@/components/ui/dialog/DialogContent.vue";
import DialogDescription from "@/components/ui/dialog/DialogDescription.vue";
import DialogFooter from "@/components/ui/dialog/DialogFooter.vue";
import DialogHeader from "@/components/ui/dialog/DialogHeader.vue";
import DialogTitle from "@/components/ui/dialog/DialogTitle.vue";
import Input from "@/components/ui/input/Input.vue";
import Textarea from "@/components/ui/textarea/Textarea.vue";
import Select from "@/components/ui/select/Select.vue";
import SelectContent from "@/components/ui/select/SelectContent.vue";
import SelectItem from "@/components/ui/select/SelectItem.vue";
import SelectTrigger from "@/components/ui/select/SelectTrigger.vue";
import SelectValue from "@/components/ui/select/SelectValue.vue";
import { useConversationStore } from "../stores/conversations";
import { useControlCenterStore } from "../stores/control-center";
import { randomId } from "../random-id";
import GovernanceView from "./GovernanceView.vue";
import { fluctlightStatusLabel } from "../lib/fluctlight-status";

const props = defineProps<{ openGovernance?: boolean; openCreate?: number }>();
const emit = defineEmits<{ openChat: []; openDetails: []; openDiagnostics: [correlationId: string]; }>();
const store = useConversationStore();
const controlCenter = useControlCenterStore();

const showCreateForm = ref(false);
const showGroupForm = ref(false);
const showGovernance = ref(false);
const creationMode = ref<"blank_slate" | "llm_defined">("blank_slate");
const newFluctlightName = ref("");
const creationDescription = ref("");
const creationPreviewJson = ref("");
const creationInitialGoals = ref<Array<Record<string, unknown>>>([]);
const creationInitialIntentions = ref<Array<Record<string, unknown>>>([]);
const creationRequestId = ref<string | null>(null);
const creationDiagnosticsCorrelationId = ref("");
const defaultGroupId = computed(() => controlCenter.actorGroups.find((group) => group.name === "默认")?.id ?? controlCenter.actorGroups[0]?.id ?? "");
const orderedActorGroups = computed(() => [...controlCenter.actorGroups].sort((left, right) => { if (left.name === "默认") return -1; if (right.name === "默认") return 1; return left.name.localeCompare(right.name, "zh-CN"); }));

watch(() => props.openGovernance, (open) => {
  if (open) showGovernance.value = true;
}, { immediate: true });
watch(() => props.openCreate, (request) => {
  if (request) showCreateForm.value = true;
}, { immediate: true });

const selectedGroupStorageKey = "fluctlight.selected-group-id";
watch(() => controlCenter.actorGroups, (groups) => {
  if (!groups.length) {
    controlCenter.selectedActorGroupId = "";
    return;
  }
  const stored = localStorage.getItem(selectedGroupStorageKey);
  const currentIsValid = groups.some((group) => group.id === controlCenter.selectedActorGroupId);
  const preferred = groups.find((group) => group.id === stored)?.id ?? groups.find((group) => group.name === "默认")?.id ?? groups[0].id;
  if (!currentIsValid) controlCenter.selectedActorGroupId = preferred;
}, { deep: true, immediate: true });
watch(() => controlCenter.selectedActorGroupId, (groupId) => {
  if (groupId) localStorage.setItem(selectedGroupStorageKey, groupId);
});

const filteredFluctlights = computed(() => {
  const groupId = controlCenter.selectedActorGroupId;
  const group = controlCenter.actorGroups.find((item) => item.id === groupId);
  return [...store.fluctlights]
    .filter((item) => {
      if (group && !group.actor_ids.includes(item.id)) return false;
      return true;
    })
    .sort((left, right) => (Date.parse(right.last_conversation_at ?? "") || 0) - (Date.parse(left.last_conversation_at ?? "") || 0));
});

async function openFluctlight(id: string) {
  await store.selectFluctlight(id);
  emit("openChat");
}

async function openDetailsFor(id: string) {
  await store.selectFluctlight(id);
  emit("openDetails");
}

async function openGovernanceFor(id: string) {
  await store.selectFluctlight(id);
  await Promise.all([
    controlCenter.loadFluctlightDetail(id),
    controlCenter.loadAutonomyActions(id),
  ]);
  showGovernance.value = true;
}

async function activateCreatedFluctlight(body: {
  initializationMode: "blank_slate" | "llm_defined";
  name?: string;
  corePersona?: Record<string, unknown>;
  developingSelf?: Record<string, unknown>;
  initialGoals?: Array<Record<string, unknown>>;
  initialIntentions?: Array<Record<string, unknown>>;
}) {
  const requestId = creationRequestId.value ?? randomId();
  creationRequestId.value = requestId;
  const created = await controlCenter.activateFluctlight({ requestId, ...body });
  if (!created?.id) return;
  await store.bootstrap();
  await controlCenter.ensureDefaultGroup(store.fluctlights.map((item) => item.id));
  await store.selectFluctlight(created.id);
  newFluctlightName.value = "";
  creationDescription.value = "";
  creationPreviewJson.value = "";
  creationInitialGoals.value = [];
  creationInitialIntentions.value = [];
  creationRequestId.value = null;
  creationDiagnosticsCorrelationId.value = "";
  showCreateForm.value = false;
  emit("openChat");
}

async function createBlank() {
  const name = newFluctlightName.value.trim();
  if (name) await activateCreatedFluctlight({ initializationMode: "blank_slate", name });
}

async function analyzeDescription() {
  const description = creationDescription.value.trim();
  if (!description) return;
  const result = await controlCenter.analyzeFluctlight(description);
  const corePersona = result?.core_persona;
  const developingSelf = result?.developing_self;
  if (!corePersona || typeof corePersona !== "object" || Array.isArray(corePersona) || !developingSelf || typeof developingSelf !== "object" || Array.isArray(developingSelf)) {
    if (result) controlCenter.error = "初始化模型返回了不包含分层 Persona 的无效结果。";
    return;
  }
  const data = result as Record<string, unknown>;
  creationPreviewJson.value = JSON.stringify(data, null, 2);
  creationInitialGoals.value = Array.isArray(data.initial_goals) ? data.initial_goals.filter((item): item is Record<string, unknown> => Boolean(item) && typeof item === "object" && !Array.isArray(item)) : [];
  creationInitialIntentions.value = Array.isArray(data.initial_intentions) ? data.initial_intentions.filter((item): item is Record<string, unknown> => Boolean(item) && typeof item === "object" && !Array.isArray(item)) : [];
  creationDiagnosticsCorrelationId.value = "";
  creationRequestId.value = randomId();
}

async function activatePreview() {
  try {
    const foundation = JSON.parse(creationPreviewJson.value) as Record<string, unknown>;
    if (!foundation.core_persona || typeof foundation.core_persona !== "object" || Array.isArray(foundation.core_persona) || !foundation.developing_self || typeof foundation.developing_self !== "object" || Array.isArray(foundation.developing_self)) throw new Error("invalid_preview");
    await activateCreatedFluctlight({
      initializationMode: "llm_defined",
      corePersona: foundation.core_persona as Record<string, unknown>,
      developingSelf: foundation.developing_self as Record<string, unknown>,
      initialGoals: creationInitialGoals.value,
      initialIntentions: creationInitialIntentions.value,
    });
  } catch {
    controlCenter.error = "预览必须包含 core_persona 和 developing_self 对象。";
  }
}

async function openCreationDiagnostics() {
  if (!creationDiagnosticsCorrelationId.value) return;
  controlCenter.diagnosticsCorrelationFilter = creationDiagnosticsCorrelationId.value;
  emit("openDiagnostics", creationDiagnosticsCorrelationId.value);
}

async function createGroup() {
  const created = await controlCenter.createActorGroup();
  showGroupForm.value = false;
  if (created?.id) controlCenter.selectedActorGroupId = created.id;
}

function selectActorGroup(value: unknown) {
  if (typeof value !== "string" && typeof value !== "number") return;
  controlCenter.selectedActorGroupId = String(value);
}

function assignActorGroup(value: unknown, fluctlightId: string) {
  if (typeof value !== "string" && typeof value !== "number") return;
  const groupId = String(value);
  if (groupId && groupId !== "__none__") void controlCenter.assignActorGroupMember(groupId, fluctlightId);
}
</script>

<template>
  <section v-if="showGovernance" class="workspace-page" aria-live="polite">
    <GovernanceView @close="showGovernance = false" @retired="showGovernance = false" />
  </section>

  <section v-else class="page instances-page" aria-labelledby="instances-title">
    <header class="page-header instances-header">
      <div><p class="eyebrow">MESSAGES</p><h1 id="instances-title">最近</h1><p class="page-lede">选择一个人格继续对话，或进入编辑与治理。</p></div>
      <Button class="primary-icon-button" variant="default" size="icon-lg" type="button" :aria-expanded="showCreateForm" aria-controls="instance-create" aria-label="新建 Fluctlight" @click="showCreateForm = !showCreateForm"><Plus :size="22" :stroke-width="2" aria-hidden="true" /></Button>
    </header>

    <div class="mobile-group-tabs" role="tablist" aria-label="聊天分组">
      <Button v-for="group in orderedActorGroups" :key="group.id" class="mobile-group-tab" variant="ghost" :class="{ selected: controlCenter.selectedActorGroupId === group.id }" role="tab" :aria-selected="controlCenter.selectedActorGroupId === group.id" type="button" @click="controlCenter.selectedActorGroupId = group.id">{{ group.name }}</Button>
    </div>

    <div class="directory-toolbar group-toolbar">
      <label class="filter-field" for="instance-group">当前分组<Select :model-value="controlCenter.selectedActorGroupId || undefined" :disabled="!controlCenter.actorGroups.length" @update:model-value="selectActorGroup"><SelectTrigger id="instance-group" class="w-full"><SelectValue placeholder="默认分组" /></SelectTrigger><SelectContent><SelectItem v-for="group in orderedActorGroups" :key="group.id" :value="group.id">{{ group.name }}{{ group.id === defaultGroupId ? "（默认）" : "" }}</SelectItem></SelectContent></Select></label>
      <Button class="secondary-button" variant="outline" type="button" :aria-expanded="showGroupForm" @click="showGroupForm = !showGroupForm"><Plus :size="16" :stroke-width="2" aria-hidden="true" />新建分组</Button>
    </div>

    <form v-if="showGroupForm" class="group-create-inline" @submit.prevent="createGroup"><label for="actor-group-name">分组名称<Input id="actor-group-name" v-model="controlCenter.newActorGroupName" maxlength="128" placeholder="例如：工作、朋友" required /></label><Button class="primary-button" variant="default" type="submit" :disabled="controlCenter.saving || !controlCenter.newActorGroupName.trim()">创建分组</Button></form>

    <Dialog :open="showCreateForm" @update:open="showCreateForm = $event">
      <DialogContent id="instance-create" class="create-surface" :show-close-button="false" aria-modal="true" aria-labelledby="create-title" aria-describedby="create-description">
        <DialogHeader class="create-dialog-header">
          <div>
            <p class="eyebrow">CREATE</p>
            <DialogTitle id="create-title">创建 Fluctlight</DialogTitle>
            <DialogDescription id="create-description">用一个名字快速开始，或先描述你希望它如何生活。</DialogDescription>
          </div>
          <DialogClose as-child>
            <Button class="text-button create-dialog-close" variant="ghost" type="button"><X :size="15" :stroke-width="2" aria-hidden="true" />关闭</Button>
          </DialogClose>
        </DialogHeader>

        <div class="create-dialog-body">
          <div class="segmented-control" role="group" aria-label="Fluctlight 创建方式">
            <Button class="segment-button" variant="ghost" :class="{ selected: creationMode === 'blank_slate' }" type="button" @click="creationMode = 'blank_slate'">白纸创建</Button>
            <Button class="segment-button" variant="ghost" :class="{ selected: creationMode === 'llm_defined' }" type="button" @click="creationMode = 'llm_defined'">从描述创建</Button>
          </div>
          <p v-if="controlCenter.error" class="error-banner" role="alert">
            {{ controlCenter.error }}
            <Button v-if="controlCenter.analysisFailureCorrelationId" class="text-button" variant="link" type="button" @click="controlCenter.diagnosticsCorrelationFilter = controlCenter.analysisFailureCorrelationId; emit('openDiagnostics', controlCenter.analysisFailureCorrelationId)">查看本次失败诊断</Button>
          </p>

          <form v-if="creationMode === 'blank_slate'" id="blank-create-form" class="stack-form" @submit.prevent="createBlank">
            <label for="fluctlight-name">实例名称<Input id="fluctlight-name" v-model="newFluctlightName" type="text" maxlength="256" required placeholder="例如：苏洛星" /></label>
          </form>

          <form v-else id="analyze-description-form" class="stack-form" @submit.prevent="analyzeDescription">
            <label for="fluctlight-description">描述你希望创建的 Fluctlight<Textarea id="fluctlight-description" v-model="creationDescription" rows="5" maxlength="12000" placeholder="描述身份、经历、价值观、表达方式或你希望它如何生活..." /></label>
          </form>

          <form v-if="creationMode === 'llm_defined' && creationPreviewJson" id="activate-preview-form" class="stack-form preview-form" @submit.prevent="activatePreview">
            <label for="fluctlight-preview">可编辑的 Persona 分层预览<Textarea id="fluctlight-preview" v-model="creationPreviewJson" rows="12" spellcheck="false" /></label>
            <div v-if="creationInitialGoals.length || creationInitialIntentions.length" class="preview-summary">
              <strong>创建后会带入</strong>
              <span v-for="goal in creationInitialGoals" :key="String(goal.description)">目标：{{ String(goal.description) }}</span>
              <span v-for="intention in creationInitialIntentions" :key="String(intention.action)">意图：{{ String(intention.action) }}</span>
            </div>
          </form>
        </div>

        <DialogFooter class="create-dialog-footer">
          <DialogClose as-child>
            <Button class="secondary-button" variant="outline" type="button">取消</Button>
          </DialogClose>
          <Button v-if="creationMode === 'blank_slate'" class="primary-button" variant="default" type="submit" form="blank-create-form" :disabled="controlCenter.saving || controlCenter.loading || !newFluctlightName.trim()">创建并开始对话</Button>
          <template v-else-if="creationPreviewJson">
            <Button v-if="creationDiagnosticsCorrelationId" class="secondary-button" variant="outline" type="button" @click="openCreationDiagnostics">查看本次分析诊断</Button>
            <Button class="secondary-button" variant="outline" type="submit" form="analyze-description-form" :disabled="controlCenter.saving || !creationDescription.trim()">重新分析</Button>
            <Button class="primary-button" variant="default" type="submit" form="activate-preview-form" :disabled="controlCenter.saving">确认激活并开始对话</Button>
          </template>
          <Button v-else class="primary-button" variant="default" type="submit" form="analyze-description-form" :disabled="controlCenter.saving || !creationDescription.trim()">分析并生成预览</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <p v-if="controlCenter.error && !showCreateForm" class="error-banner" role="alert">{{ controlCenter.error }}</p>
    <section v-if="store.fluctlights.length && filteredFluctlights.length" class="instance-list" aria-label="Fluctlight 实例列表">
      <article v-for="fluctlight in filteredFluctlights" :key="fluctlight.id" class="instance-list-item" :class="{ selected: fluctlight.id === store.fluctlightId }">
        <Button class="instance-main justify-start" variant="ghost" type="button" @click="openFluctlight(fluctlight.id)"><span class="avatar persona-avatar">{{ String(fluctlight.identity.name ?? "F").slice(0, 1) }}</span><span class="instance-copy"><strong>{{ String(fluctlight.identity.name ?? fluctlight.id) }}</strong><small><Badge class="status-pill" variant="secondary" :class="{ paused: fluctlight.status === 'paused', muted: fluctlight.status === 'retired' }">{{ fluctlightStatusLabel(fluctlight.status) }}</Badge><template v-if="fluctlight.unread_count"> · {{ fluctlight.unread_count }} 条未读</template></small></span></Button>
        <div class="instance-actions"><Button class="text-button" variant="ghost" type="button" @click="openDetailsFor(fluctlight.id)">查看详情</Button><Button class="text-button" variant="ghost" type="button" @click="openGovernanceFor(fluctlight.id)">编辑与治理</Button><Select v-if="controlCenter.actorGroups.length" :aria-label="'为 ' + fluctlight.id + ' 指定分组'" @update:model-value="(value) => assignActorGroup(value, fluctlight.id)"><SelectTrigger class="instance-group-select"><SelectValue placeholder="加入分组..." /></SelectTrigger><SelectContent><SelectItem value="__none__">加入分组...</SelectItem><SelectItem v-for="group in orderedActorGroups.filter((item) => !item.actor_ids.includes(fluctlight.id))" :key="group.id" :value="group.id">{{ group.name }}</SelectItem></SelectContent></Select></div>
      </article>
    </section>
    <div v-else-if="!store.fluctlights.length" class="empty-panel"><span class="empty-mark" aria-hidden="true"><Plus :size="22" :stroke-width="2" /></span><h2>还没有 Fluctlight 实例</h2><p>创建第一个实例后，它会出现在这里。</p><Button class="primary-button" variant="default" type="button" @click="showCreateForm = true">创建第一个实例</Button></div>
    <div v-else class="empty-panel compact"><h2>当前分组没有实例</h2><p>新建或切换分组即可查看其他人格。</p></div>
  </section>
</template>

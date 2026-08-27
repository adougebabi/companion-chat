<script setup lang="ts">
import { computed, ref, watch } from "vue";

import { useConversationStore } from "../stores/conversations";
import { useControlCenterStore } from "../stores/control-center";
import { randomId } from "../random-id";
import GovernanceView from "./GovernanceView.vue";

const props = defineProps<{ openGovernance?: boolean }>();
const emit = defineEmits<{ openChat: []; openDetails: []; openDiagnostics: [correlationId: string]; }>();
const store = useConversationStore();
const controlCenter = useControlCenterStore();

const instanceSearch = ref("");
const showCreateForm = ref(false);
const showGovernance = ref(false);
const creationMode = ref<"blank_slate" | "llm_defined">("blank_slate");
const newFluctlightName = ref("");
const creationDescription = ref("");
const creationPreviewJson = ref("");
const creationInitialGoals = ref<Array<Record<string, unknown>>>([]);
const creationInitialIntentions = ref<Array<Record<string, unknown>>>([]);
const creationRequestId = ref<string | null>(null);
const creationDiagnosticsCorrelationId = ref("");
const creationFoundationProvenance = ref<Record<string, unknown>>({});

watch(() => props.openGovernance, (open) => {
  if (open) showGovernance.value = true;
}, { immediate: true });

const filteredFluctlights = computed(() => {
  const query = instanceSearch.value.trim().toLowerCase();
  const groupId = controlCenter.selectedActorGroupId;
  const group = controlCenter.actorGroups.find((item) => item.id === groupId);
  return [...store.fluctlights]
    .filter((item) => {
      if (group && !group.actor_ids.includes(item.id)) return false;
      if (!query) return true;
      return String(item.identity.name ?? "").toLowerCase().includes(query) || item.id.toLowerCase().includes(query);
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
  identity: Record<string, unknown>;
  personality?: Record<string, unknown>;
  behavioralPolicy?: Record<string, unknown>;
  lifeProfile?: Record<string, unknown>;
  foundationProvenance?: Record<string, unknown>;
  initialGoals?: Array<Record<string, unknown>>;
  initialIntentions?: Array<Record<string, unknown>>;
}) {
  const requestId = creationRequestId.value ?? randomId();
  creationRequestId.value = requestId;
  const created = await controlCenter.activateFluctlight({ requestId, ...body });
  if (!created?.id) return;
  await store.bootstrap();
  await store.selectFluctlight(created.id);
  newFluctlightName.value = "";
  creationDescription.value = "";
  creationPreviewJson.value = "";
  creationInitialGoals.value = [];
  creationInitialIntentions.value = [];
  creationRequestId.value = null;
  creationDiagnosticsCorrelationId.value = "";
  creationFoundationProvenance.value = {};
  showCreateForm.value = false;
  emit("openChat");
}

async function createBlank() {
  const name = newFluctlightName.value.trim();
  if (name) await activateCreatedFluctlight({ initializationMode: "blank_slate", identity: { name } });
}

async function analyzeDescription() {
  const description = creationDescription.value.trim();
  if (!description) return;
  const result = await controlCenter.analyzeFluctlight(description);
  const foundation = result?.foundation;
  if (!foundation || typeof foundation !== "object" || Array.isArray(foundation)) {
    if (result) controlCenter.error = "初始化模型返回了不包含 Foundation 的无效结果。";
    return;
  }
  const data = foundation as Record<string, unknown>;
  creationPreviewJson.value = JSON.stringify(data, null, 2);
  creationInitialGoals.value = Array.isArray(data.initial_goals) ? data.initial_goals.filter((item): item is Record<string, unknown> => Boolean(item) && typeof item === "object" && !Array.isArray(item)) : [];
  creationInitialIntentions.value = Array.isArray(data.initial_intentions) ? data.initial_intentions.filter((item): item is Record<string, unknown> => Boolean(item) && typeof item === "object" && !Array.isArray(item)) : [];
  const provenance = result.provenance;
  creationDiagnosticsCorrelationId.value = provenance && typeof provenance === "object" ? String((provenance as Record<string, unknown>).correlation_id ?? "") : "";
  creationFoundationProvenance.value = provenance && typeof provenance === "object" && !Array.isArray(provenance) && (provenance as Record<string, unknown>).foundation && typeof (provenance as Record<string, unknown>).foundation === "object" ? (provenance as Record<string, unknown>).foundation as Record<string, unknown> : {};
  creationRequestId.value = randomId();
}

async function activatePreview() {
  try {
    const foundation = JSON.parse(creationPreviewJson.value) as Record<string, unknown>;
    const required = ["identity", "personality", "behavioral_policy", "life_profile"];
    if (!required.every((key) => foundation[key] && typeof foundation[key] === "object" && !Array.isArray(foundation[key]))) throw new Error("invalid_preview");
    await activateCreatedFluctlight({
      initializationMode: "llm_defined",
      identity: foundation.identity as Record<string, unknown>,
      personality: foundation.personality as Record<string, unknown>,
      behavioralPolicy: foundation.behavioral_policy as Record<string, unknown>,
      lifeProfile: foundation.life_profile as Record<string, unknown>,
      foundationProvenance: (foundation.provenance ?? creationFoundationProvenance.value) as Record<string, unknown>,
      initialGoals: creationInitialGoals.value,
      initialIntentions: creationInitialIntentions.value,
    });
  } catch {
    controlCenter.error = "预览必须包含 identity、personality、behavioral_policy 和 life_profile 对象。";
  }
}

async function openCreationDiagnostics() {
  if (!creationDiagnosticsCorrelationId.value) return;
  controlCenter.diagnosticsCorrelationFilter = creationDiagnosticsCorrelationId.value;
  emit("openDiagnostics", creationDiagnosticsCorrelationId.value);
}
</script>

<template>
  <section v-if="showGovernance" class="workspace-page" aria-live="polite">
    <GovernanceView @close="showGovernance = false" @retired="showGovernance = false" />
  </section>

  <section v-else class="page instances-page" aria-labelledby="instances-title">
    <header class="page-header instances-header">
      <div><p class="eyebrow">YOUR FLUCTLIGHTS</p><h1 id="instances-title">实例</h1><p class="page-lede">选择一个人格继续对话，或进入编辑与治理。</p></div>
      <button class="primary-icon-button" type="button" :aria-expanded="showCreateForm" aria-controls="instance-create" aria-label="新建 Fluctlight" @click="showCreateForm = !showCreateForm">＋</button>
    </header>

    <div class="directory-toolbar">
      <label class="search-field" for="instance-search"><span aria-hidden="true">⌕</span><span class="sr-only">搜索实例</span><input id="instance-search" v-model="instanceSearch" type="search" placeholder="搜索实例或标识" /></label>
      <label class="filter-field" for="instance-group">分组<select id="instance-group" v-model="controlCenter.selectedActorGroupId"><option value="">全部实例</option><option v-for="group in controlCenter.actorGroups" :key="group.id" :value="group.id">{{ group.name }}</option></select></label>
    </div>

    <section v-if="showCreateForm" id="instance-create" class="create-surface" aria-labelledby="create-title">
      <div class="section-heading"><div><p class="eyebrow">CREATE</p><h2 id="create-title">创建 Fluctlight</h2></div><button class="text-button" type="button" @click="showCreateForm = false">关闭</button></div>
      <div class="segmented-control" role="group" aria-label="Fluctlight 创建方式"><button class="segment-button" :class="{ selected: creationMode === 'blank_slate' }" type="button" @click="creationMode = 'blank_slate'">白纸创建</button><button class="segment-button" :class="{ selected: creationMode === 'llm_defined' }" type="button" @click="creationMode = 'llm_defined'">从描述创建</button></div>
      <p v-if="controlCenter.error" class="error-banner" role="alert">{{ controlCenter.error }}</p>
      <form v-if="creationMode === 'blank_slate'" class="stack-form" @submit.prevent="createBlank"><label for="fluctlight-name">实例名称<input id="fluctlight-name" v-model="newFluctlightName" type="text" maxlength="256" required placeholder="例如：苏洛星" /></label><button class="primary-button" type="submit" :disabled="controlCenter.saving || controlCenter.loading || !newFluctlightName.trim()">创建并开始对话</button></form>
      <form v-else class="stack-form" @submit.prevent="analyzeDescription"><label for="fluctlight-description">描述你希望创建的 Fluctlight<textarea id="fluctlight-description" v-model="creationDescription" rows="5" maxlength="12000" placeholder="描述身份、经历、价值观、表达方式或你希望它如何生活..." /></label><button class="primary-button" type="submit" :disabled="controlCenter.saving || !creationDescription.trim()">分析并生成预览</button></form>
      <form v-if="creationMode === 'llm_defined' && creationPreviewJson" class="stack-form preview-form" @submit.prevent="activatePreview"><label for="fluctlight-preview">可编辑的基础预览<textarea id="fluctlight-preview" v-model="creationPreviewJson" rows="12" spellcheck="false" /></label><div v-if="creationInitialGoals.length || creationInitialIntentions.length" class="preview-summary"><strong>创建后会带入</strong><span v-for="goal in creationInitialGoals" :key="String(goal.description)">目标：{{ String(goal.description) }}</span><span v-for="intention in creationInitialIntentions" :key="String(intention.action)">意图：{{ String(intention.action) }}</span></div><button v-if="creationDiagnosticsCorrelationId" class="secondary-button" type="button" @click="openCreationDiagnostics">查看本次分析诊断</button><button class="primary-button" type="submit" :disabled="controlCenter.saving">确认激活并开始对话</button></form>
      <form class="group-form" @submit.prevent="controlCenter.createActorGroup"><label for="actor-group-name">新建实例分组<input id="actor-group-name" v-model="controlCenter.newActorGroupName" maxlength="128" placeholder="例如：工作、朋友" /></label><button class="secondary-button" type="submit" :disabled="controlCenter.saving || !controlCenter.newActorGroupName.trim()">创建分组</button></form>
    </section>

    <p v-if="controlCenter.error && !showCreateForm" class="error-banner" role="alert">{{ controlCenter.error }}</p>
    <section v-if="store.fluctlights.length" class="instance-list" aria-label="Fluctlight 实例列表">
      <article v-for="fluctlight in filteredFluctlights" :key="fluctlight.id" class="instance-list-item" :class="{ selected: fluctlight.id === store.fluctlightId }">
        <button class="instance-main" type="button" @click="openFluctlight(fluctlight.id)"><span class="avatar persona-avatar">{{ String(fluctlight.identity.name ?? "F").slice(0, 1) }}</span><span class="instance-copy"><strong>{{ String(fluctlight.identity.name ?? fluctlight.id) }}</strong><small>{{ fluctlight.status === "paused" ? "已暂停" : "可对话" }}<template v-if="fluctlight.unread_count"> · {{ fluctlight.unread_count }} 条未读</template></small></span><span class="instance-state">{{ fluctlight.id === store.fluctlightId ? "当前" : "" }}</span></button>
        <div class="instance-actions"><button class="text-button" type="button" @click="openDetailsFor(fluctlight.id)">查看详情</button><button class="text-button" type="button" @click="openGovernanceFor(fluctlight.id)">编辑与治理</button><select v-if="controlCenter.actorGroups.length" :aria-label="'为 ' + fluctlight.id + ' 指定分组'" @change="($event) => { const groupId = ($event.target as HTMLSelectElement).value; if (groupId) controlCenter.assignActorGroupMember(groupId, fluctlight.id) }"><option value="">加入分组...</option><option v-for="group in controlCenter.actorGroups.filter((item) => !item.actor_ids.includes(fluctlight.id))" :key="group.id" :value="group.id">{{ group.name }}</option></select></div>
      </article>
    </section>
    <div v-else class="empty-panel"><span class="empty-mark" aria-hidden="true">＋</span><h2>还没有 Fluctlight 实例</h2><p>创建第一个实例后，它会出现在这里。</p><button class="primary-button" type="button" @click="showCreateForm = true">创建第一个实例</button></div>
  </section>
</template>

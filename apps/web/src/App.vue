<script setup lang="ts">
import { nextTick, onMounted, ref } from "vue";
import type { BrowserMessage } from "@fluctlight/browser-client";

import { useConversationStore } from "./stores/conversations";
import { useControlCenterStore } from "./stores/control-center";
import { randomId } from "./random-id";
import { bffOrigin } from "./runtime-config";

const store = useConversationStore();
const controlCenter = useControlCenterStore();
type View = "chat" | "fluctlights" | "moments" | "diagnostics" | "settings";
const view = ref<View>("fluctlights");
const providerRoles = [
  { value: "initialization", label: "初始化" },
  { value: "cognitive_assessment", label: "认知判断" },
  { value: "action_realization", label: "回复生成" },
  { value: "reflection", label: "反思" },
  { value: "embedding", label: "Embedding" },
  { value: "media_prompt", label: "媒体提示词" },
] as const;
const selectedProviderRole = ref<(typeof providerRoles)[number]["value"]>("cognitive_assessment");
const roleEndpointId = ref("");
const roleModelId = ref("");
const roleTokenBudget = ref(2048);
const roleTimeoutSeconds = ref(60);
const endpointPickerId = ref("");
const endpointId = ref("primary");
const endpointUrl = ref("");
const endpointSecret = ref("");
const providerKind = ref("openai-compatible");
const comfyUiUrl = ref("");
const comfyUiWorkflow = ref("");
const newFluctlightName = ref("");
const creationMode = ref<"blank_slate" | "llm_defined">("blank_slate");
const creationDescription = ref("");
const creationPreviewJson = ref("");
const creationRequestId = ref<string | null>(null);
const instanceSearch = ref("");
const showCreateForm = ref(false);
const showInstanceDetails = ref(false);
const draft = ref("");
const authPassword = ref("");
const setupToken = ref("");
const newOwnerPassword = ref("");
const changedOwnerPassword = ref("");
const composer = ref<HTMLTextAreaElement | null>(null);
const transcript = ref<HTMLElement | null>(null);

async function send() {
  const text = draft.value;
  draft.value = "";
  await store.send(text);
  await nextTick();
  transcript.value?.scrollTo({ top: transcript.value.scrollHeight, behavior: "smooth" });
  composer.value?.focus();
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === "Enter" && !event.shiftKey) {
    event.preventDefault();
    void send();
  }
}

async function selectView(next: typeof view.value) {
  view.value = next;
  if (next === "fluctlights") {
    await controlCenter.loadFluctlightDetail(store.fluctlightId);
    await controlCenter.loadAutonomyActions(store.fluctlightId);
    await controlCenter.loadActorGroups();
  }
  if (next === "diagnostics") await controlCenter.loadDiagnostics();
  if (next === "moments") await controlCenter.loadMoments(store.fluctlightId);
  if (next === "settings") {
    await controlCenter.loadSettings();
    const preferredBinding = controlCenter.providerBindings.find(
      (binding) => binding.endpoint_id === "primary",
    ) ?? controlCenter.providerBindings[0];
    if (preferredBinding && providerRoles.some((providerRole) => providerRole.value === preferredBinding.role)) {
      selectedProviderRole.value = preferredBinding.role as (typeof providerRoles)[number]["value"];
    }
    selectEndpointForEditing(
      controlCenter.providerEndpoints.find((endpoint) => endpoint.id === "primary")?.id
        ?? controlCenter.providerEndpoints[0]?.id
        ?? "",
    );
    await selectProviderRole(selectedProviderRole.value);
    const comfy = controlCenter.settings?.values["media.comfyui"];
    if (comfy && typeof comfy === "object" && !Array.isArray(comfy)) {
      comfyUiUrl.value = String((comfy as Record<string, unknown>).baseUrl ?? "");
      const workflow = (comfy as Record<string, unknown>).workflow;
      comfyUiWorkflow.value = workflow && typeof workflow === "object" ? JSON.stringify(workflow, null, 2) : "";
    }
    const autonomy = controlCenter.settings?.values["product.autonomy"];
    const retention = controlCenter.settings?.values["diagnostics.retention"];
    controlCenter.autonomySettingsJson = JSON.stringify(
      autonomy && typeof autonomy === "object" && !Array.isArray(autonomy)
        ? autonomy
        : { mode: "active", allowed_actions: ["proactive_message", "memory_candidate", "relationship_candidate", "schedule_proposal", "media_request", "moment"], budget_remaining: 1 },
      null,
      2,
    );
    controlCenter.diagnosticsRetentionJson = JSON.stringify(
      retention && typeof retention === "object" && !Array.isArray(retention)
        ? retention
        : { retention_days: 30, max_rows: 10000 },
      null,
      2,
    );
  }
}

async function selectProviderRole(role: (typeof providerRoles)[number]["value"]) {
  selectedProviderRole.value = role;
  const binding = controlCenter.providerBindings.find((item) => item.role === role);
  roleEndpointId.value = binding?.endpoint_id
    ?? controlCenter.providerEndpoints.find((endpoint) => endpoint.id === "primary")?.id
    ?? controlCenter.providerEndpoints[0]?.id
    ?? "";
  roleModelId.value = binding?.model_id ?? "";
  roleTokenBudget.value = binding?.token_budget ?? 2048;
  roleTimeoutSeconds.value = binding?.timeout_seconds ?? 60;
  await controlCenter.loadProviderModels(roleEndpointId.value);
}

async function changeRoleEndpoint() {
  await controlCenter.loadProviderModels(roleEndpointId.value);
}

function selectEndpointForEditing(selectedId: string) {
  endpointPickerId.value = selectedId;
  const endpoint = controlCenter.providerEndpoints.find((item) => item.id === selectedId);
  if (!endpoint) return;
  endpointId.value = endpoint.id;
  endpointUrl.value = endpoint.base_url;
  providerKind.value = endpoint.kind;
  endpointSecret.value = "";
}

async function saveModelRole() {
  if (!roleEndpointId.value || !roleModelId.value.trim()) {
    controlCenter.error = "请为该模型角色选择 endpoint 和模型。";
    return;
  }
  await controlCenter.configureModelRole({
    role: selectedProviderRole.value,
    endpointId: roleEndpointId.value,
    modelId: roleModelId.value.trim(),
    tokenBudget: roleTokenBudget.value,
    timeoutSeconds: roleTimeoutSeconds.value,
  });
  if (!controlCenter.error) await selectProviderRole(selectedProviderRole.value);
}

async function saveProviderEndpoint() {
  if (!endpointId.value.trim() || !endpointUrl.value.trim()) {
    controlCenter.error = "请完整填写 endpoint 标识和服务地址。";
    return;
  }
  if (endpointSecret.value) {
    await controlCenter.saveSettings({}, { [`provider:${endpointId.value.trim()}`]: endpointSecret.value });
    if (controlCenter.error) return;
  }
  await controlCenter.configureProviderEndpoint({
    endpointId: endpointId.value.trim(),
    kind: providerKind.value,
    baseUrl: endpointUrl.value.trim(),
    secretPurpose: `provider:${endpointId.value.trim()}`,
  });
  if (!controlCenter.error) {
    endpointSecret.value = "";
    selectEndpointForEditing(endpointId.value.trim());
    await selectProviderRole(selectedProviderRole.value);
  }
}

async function saveMediaProvider() {
  if (!comfyUiUrl.value.trim() || !comfyUiWorkflow.value.trim()) {
    if (comfyUiUrl.value.trim() || comfyUiWorkflow.value.trim()) {
      controlCenter.error = "请同时填写 ComfyUI URL 和图片工作流。";
    }
    return;
  }
  try {
    const workflow = JSON.parse(comfyUiWorkflow.value);
    if (!workflow || typeof workflow !== "object" || Array.isArray(workflow)) throw new Error("workflow_not_object");
    await controlCenter.saveSettings({ "media.comfyui": { baseUrl: comfyUiUrl.value.trim(), workflow } });
  } catch {
    controlCenter.error = "ComfyUI 图片工作流必须是 JSON 对象。";
  }
}

async function activateCreatedFluctlight(body: {
  initializationMode: "blank_slate" | "llm_defined";
  identity: Record<string, unknown>;
  personality?: Record<string, unknown>;
  behavioralPolicy?: Record<string, unknown>;
}) {
  const requestId = creationRequestId.value ?? randomId();
  creationRequestId.value = requestId;
  const created = await controlCenter.activateFluctlight({ requestId, ...body });
  if (created?.id) {
    await store.bootstrap();
    await store.selectFluctlight(created.id);
    newFluctlightName.value = "";
    creationDescription.value = "";
    creationPreviewJson.value = "";
    creationRequestId.value = null;
    view.value = "chat";
  }
}

async function createFluctlightAndConversation() {
  const name = newFluctlightName.value.trim();
  if (!name) return;
  await activateCreatedFluctlight({ initializationMode: "blank_slate", identity: { name } });
}

async function analyzeFluctlightDescription() {
  const description = creationDescription.value.trim();
  if (!description) return;
  const result = await controlCenter.analyzeFluctlight(description);
  const foundation = result?.foundation;
  if (foundation && typeof foundation === "object" && !Array.isArray(foundation)) {
    creationPreviewJson.value = JSON.stringify(foundation, null, 2);
    creationRequestId.value = randomId();
  } else if (result) {
    controlCenter.error = "初始化模型返回了不包含 Foundation 的无效结果。";
  }
}

async function activatePreview() {
  try {
    const foundation = JSON.parse(creationPreviewJson.value) as Record<string, unknown>;
    const identity = foundation.identity;
    const personality = foundation.personality;
    const behavioralPolicy = foundation.behavioral_policy;
    if (!identity || typeof identity !== "object" || Array.isArray(identity) || !personality || typeof personality !== "object" || Array.isArray(personality) || !behavioralPolicy || typeof behavioralPolicy !== "object" || Array.isArray(behavioralPolicy)) throw new Error("invalid_preview");
    await activateCreatedFluctlight({ initializationMode: "llm_defined", identity: identity as Record<string, unknown>, personality: personality as Record<string, unknown>, behavioralPolicy: behavioralPolicy as Record<string, unknown> });
  } catch {
    controlCenter.error = "预览必须包含 identity、personality 和 behavioral_policy 三个对象。";
  }
}

async function openFluctlight(fluctlightId: string) {
  await store.selectFluctlight(fluctlightId);
  if (store.hasConversation) {
    view.value = "chat";
    await nextTick();
    transcript.value?.scrollTo({ top: transcript.value.scrollHeight, behavior: "auto" });
  }
  await Promise.all([
    controlCenter.loadFluctlightDetail(fluctlightId),
    controlCenter.loadAutonomyActions(fluctlightId),
  ]);
}

function prettyPayload(payload: Record<string, unknown>) {
  return JSON.stringify(payload, null, 2);
}

function mediaUrl(assetId: string) {
  return new URL(`/api/media/${encodeURIComponent(assetId)}`, bffOrigin).toString();
}

const displayLabels: Record<string, string> = {
  id: "标识", name: "名称", age: "年龄", gender: "性别", occupation: "职业",
  residence: "居住地", timezone: "时区", birthday: "生日", background: "背景",
  biography: "经历", core_values: "核心价值", worldview: "世界观", notes: "备注",
  openness: "开放性", conscientiousness: "尽责性", extraversion: "外向性",
  agreeableness: "宜人性", neuroticism: "情绪敏感度", curiosity: "好奇心",
  independence: "独立性", patience: "耐心", empathy: "共情", assertiveness: "主张性",
  humor: "幽默感", sociability: "社交性", risk_tolerance: "风险偏好",
  update_policy: "更新策略", response_style: "回复风格", message_length: "消息长度",
  emoji_frequency: "表情频率", punctuation_style: "标点风格", humor_style: "幽默风格",
  sarcasm_tendency: "讽刺倾向", directness: "直接性", initiative: "主动性",
  topic_initiation: "发起话题", silence_tolerance: "沉默容忍度",
  response_delay: "回复延迟", emotional_expression: "情绪表达",
  conflict_style: "冲突风格", refusal_style: "拒绝风格", intimacy_expression: "亲密表达",
};
const roleLabels: Record<string, string> = {
  initialization: "初始化", cognitive_assessment: "认知判断", action_realization: "回复生成",
  reflection: "反思", embedding: "Embedding", media_prompt: "媒体提示词",
};
function labelFor(key: string) { return displayLabels[key] ?? key; }
function roleLabel(role: string) { return roleLabels[role] ?? role; }
function deliveryStatus(message: BrowserMessage): "pending" | "sent" | "none" {
  if (message.kind !== "user") return "none";
  const latestUserMessage = [...store.messages].reverse().find((item) => item.kind === "user");
  return store.sending && latestUserMessage?.id === message.id ? "pending" : "sent";
}
function diagnosticFailureReason(errorCode: string) {
  const explanations: Record<string, string> = {
    cognitive_provider_response_is_missing_decision: "认知模型没有返回必需的 decision 对象，无法生成下一步动作。",
    perception_social_signals_must_be_a_list_of_references: "认知模型将 social_signals 返回成了非数组类型；该字段必须是字符串数组，空值应为 []。",
    activation_foundation_invalid: "预览 Foundation 的字段或嵌套值不符合合同。",
    activation_foundation_incomplete: "预览缺少完整人格或行为策略。",
    activation_persistence_failed: "Foundation 已通过校验，但无法写入 Fluctlight 数据。",
    initialization_foundation_invalid: "初始化模型返回的 Foundation 字段不符合合同。",
  };
  return explanations[errorCode] ?? "模型响应未通过结构化合同校验，详情见错误码。";
}
function filteredFluctlights() {
  const groupId = controlCenter.selectedActorGroupId;
  const query = instanceSearch.value.trim().toLowerCase();
  const visible = store.fluctlights.filter((item) => {
    if (!query) return true;
    const name = String(item.identity.name ?? "").toLowerCase();
    return name.includes(query) || item.id.toLowerCase().includes(query);
  });
  const newestFirst = (items: typeof visible) => [...items].sort((left, right) => {
    const leftActivity = Date.parse(left.last_conversation_at ?? "") || 0;
    const rightActivity = Date.parse(right.last_conversation_at ?? "") || 0;
    return rightActivity - leftActivity;
  });
  if (!groupId) return newestFirst(visible);
  const group = controlCenter.actorGroups.find((item) => item.id === groupId);
  return newestFirst(group ? visible.filter((item) => group.actor_ids.includes(item.id)) : visible);
}
function workflowIdFor(value: Record<string, unknown>) {
  const findId = (candidate: unknown): string => {
    if (!candidate || typeof candidate !== "object" || Array.isArray(candidate)) return "";
    const record = candidate as Record<string, unknown>;
    const id = record.workflow_id ?? record.workflowId;
    if (typeof id === "string" && id) return id;
    for (const nested of Object.values(record)) {
      const found = findId(nested);
      if (found) return found;
    }
    return "";
  };
  return findId(value);
}

async function signIn() {
  await store.login(authPassword.value);
  authPassword.value = "";
}

async function completeSetup() {
  await store.setup(setupToken.value, newOwnerPassword.value);
  setupToken.value = "";
  newOwnerPassword.value = "";
}

async function changeOwnerPassword() {
  const changed = await store.changePassword(changedOwnerPassword.value);
  if (changed) changedOwnerPassword.value = "";
}

onMounted(() => void store.initialize());
</script>

<template>
  <main class="shell" :class="{ 'chat-shell': view === 'chat' }">
    <section v-if="store.authenticated !== true" class="auth-panel" aria-labelledby="auth-title">
      <p class="eyebrow">FLUCTLIGHT</p>
      <h1 id="auth-title">{{ store.setupAvailable ? "创建所有者" : "所有者登录" }}</h1>
      <p class="auth-copy">{{ store.setupAvailable ? "输入由本机管理员签发的一次性设置令牌。" : "登录后管理 Fluctlight 实例，并与它们继续对话。" }}</p>
      <form v-if="store.setupAvailable" class="auth-form" @submit.prevent="completeSetup">
        <label for="setup-token">设置令牌</label>
        <input id="setup-token" v-model="setupToken" type="password" autocomplete="one-time-code" required :disabled="store.authLoading" />
        <label for="setup-password">所有者密码</label>
        <input id="setup-password" v-model="newOwnerPassword" type="password" autocomplete="new-password" minlength="6" required :disabled="store.authLoading" />
        <p v-if="store.authError" class="error-banner" role="alert">{{ store.authError }}</p>
        <button class="send-button" type="submit" :disabled="store.authLoading || !setupToken || newOwnerPassword.length < 6">{{ store.authLoading ? "正在创建..." : "创建所有者" }}</button>
      </form>
      <form v-else class="auth-form" @submit.prevent="signIn">
        <label for="auth-password">密码</label>
        <input id="auth-password" v-model="authPassword" type="password" autocomplete="current-password" minlength="6" required :disabled="store.authLoading" />
        <p v-if="store.authError" class="error-banner" role="alert">{{ store.authError }}</p>
        <button class="send-button" type="submit" :disabled="store.authLoading || authPassword.length < 6">{{ store.authLoading ? "正在登录..." : "登录" }}</button>
      </form>
    </section>
    <template v-else>
    <template v-if="view === 'chat'">
      <header class="chat-header">
        <button class="chat-back" type="button" aria-label="返回实例" @click="view = 'fluctlights'">‹</button>
        <span class="chat-avatar" aria-hidden="true">{{ String(store.selectedFluctlightName ?? 'F').slice(0, 1) }}</span>
        <div class="chat-header-copy"><strong>{{ store.selectedFluctlightName ?? '对话' }}</strong><small>摇光当前状态：{{ store.sending ? '思考中' : '就绪' }}</small></div>
      </header>
      <section ref="transcript" class="transcript" aria-live="polite" aria-label="对话记录">
      <div v-if="store.loading" class="empty-state">正在加载对话...</div>
      <button v-else-if="store.nextBeforeSequence" class="secondary-button load-older" type="button" @click="store.loadOlder">加载更早记录</button>
      <div v-else-if="!store.selectedFluctlight" class="empty-state">
        <span class="empty-mark" aria-hidden="true">+</span>
        <h2>还没有 Fluctlight 实例</h2>
        <p>先创建一个实例，再开始你们之间的对话。</p>
        <button class="send-button" type="button" @click="view = 'fluctlights'">创建 Fluctlight</button>
      </div>
      <div v-else-if="!store.messages.length" class="empty-state">
        <span class="empty-mark" aria-hidden="true">+</span>
        <h2>开始与 {{ store.selectedFluctlightName }} 对话</h2>
        <p>分享一件事、一个问题，或此刻正在发生的事情。</p>
      </div>
      <article
        v-for="message in store.messages"
        :key="message.id"
        class="message-row"
        :class="message.kind === 'user' ? 'from-user' : 'from-fluctlight'"
      >
        <div class="avatar" aria-hidden="true">{{ message.kind === "user" ? "Y" : "F" }}</div>
        <div class="message-bubble">
          <p>{{ message.text }}</p>
          <div v-if="message.attachmentRefs?.length" class="message-media"><img v-for="assetId in message.attachmentRefs" :key="assetId" :src="mediaUrl(assetId)" :alt="store.selectedFluctlightName + ' 生成的图片'" loading="lazy" /></div>
          <div class="message-meta"><time>{{ new Date(message.createdAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) }}</time><span v-if="deliveryStatus(message) !== 'none'" class="delivery-status" :class="deliveryStatus(message)" :aria-label="deliveryStatus(message) === 'pending' ? '已接收，处理中' : '已回复'">✓<span v-if="deliveryStatus(message) === 'sent'">✓</span></span></div>
        </div>
      </article>
      </section>

      <p v-if="store.error" class="error-banner" role="alert">{{ store.error }}</p>

      <form class="composer" @submit.prevent="send">
      <label class="sr-only" for="message-composer">消息</label>
      <textarea
        id="message-composer"
        ref="composer"
        v-model="draft"
        rows="1"
        maxlength="32000"
        placeholder="写一条消息..."
        :disabled="store.loading || !store.hasConversation || !store.selectedFluctlight"
        @keydown="onKeydown"
      />
      <div class="composer-footer">
        <label class="attachment-input" for="attachment-reference">
          <span aria-hidden="true">+</span>
          <input id="attachment-reference" v-model="store.attachmentRef" aria-label="附件引用" type="text" placeholder="附件引用" maxlength="512" :disabled="!store.hasConversation" />
        </label>
        <div class="composer-actions">
          <button v-if="store.sending" class="secondary-button" type="button" @click="store.cancel">取消</button>
          <button class="send-button" type="submit" :disabled="store.sending || !store.hasConversation || !store.selectedFluctlight || !draft.trim()">
            发送
          </button>
        </div>
      </div>
      </form>
    </template>

    <section v-else-if="view === 'fluctlights'" class="control-panel fluctlights-panel" aria-labelledby="fluctlights-title">
      <div class="list-header"><div><p class="eyebrow">ECHO</p><h2 id="fluctlights-title">聊天</h2></div><button class="round-action" type="button" aria-label="新建 Fluctlight" @click="showCreateForm = !showCreateForm">＋</button></div>
      <label class="search-box" for="instance-search"><span aria-hidden="true">⌕</span><input id="instance-search" v-model="instanceSearch" type="search" placeholder="搜索" /></label>
      <div v-if="showCreateForm" class="create-drawer">
      <div class="creation-mode" role="group" aria-label="Fluctlight 创建方式">
        <button class="secondary-button" :class="{ selected: creationMode === 'blank_slate' }" type="button" @click="creationMode = 'blank_slate'">白纸创建</button>
        <button class="secondary-button" :class="{ selected: creationMode === 'llm_defined' }" type="button" @click="creationMode = 'llm_defined'">从描述创建</button>
      </div>
      <p v-if="controlCenter.error" class="error-banner" role="alert">{{ controlCenter.error }}</p>
      <form v-if="creationMode === 'blank_slate'" class="actor-create-form" @submit.prevent="createFluctlightAndConversation">
        <label for="fluctlight-name">创建 Fluctlight</label>
        <div class="actor-create-row">
          <input id="fluctlight-name" v-model="newFluctlightName" type="text" maxlength="256" required placeholder="实例名称" />
          <button class="send-button" type="submit" :disabled="controlCenter.saving || controlCenter.loading || !newFluctlightName.trim()">创建并对话</button>
        </div>
      </form>
      <form v-else class="actor-create-form" @submit.prevent="analyzeFluctlightDescription">
        <label for="fluctlight-description">描述你希望创建的 Fluctlight</label>
        <textarea id="fluctlight-description" v-model="creationDescription" rows="5" maxlength="12000" placeholder="描述身份、经历、价值观、表达方式或你希望它如何生活..." />
        <button class="send-button" type="submit" :disabled="controlCenter.saving || !creationDescription.trim()">分析并生成预览</button>
      </form>
      <form v-if="creationMode === 'llm_defined' && creationPreviewJson" class="actor-create-form" @submit.prevent="activatePreview">
        <label for="fluctlight-preview">可编辑的基础预览</label>
        <textarea id="fluctlight-preview" v-model="creationPreviewJson" rows="14" spellcheck="false" />
        <button class="send-button" type="submit" :disabled="controlCenter.saving">确认激活并对话</button>
      </form>
      <form class="actor-create-form" @submit.prevent="controlCenter.createActorGroup"><label for="actor-group-name">实例分组</label><div class="actor-create-row"><input id="actor-group-name" v-model="controlCenter.newActorGroupName" maxlength="128" placeholder="新分组名称" /><button class="secondary-button" type="submit" :disabled="controlCenter.saving || !controlCenter.newActorGroupName.trim()">创建分组</button></div></form>
      </div>
      <label class="toggle-row">筛选分组<select v-model="controlCenter.selectedActorGroupId"><option value="">全部实例</option><option v-for="group in controlCenter.actorGroups" :key="group.id" :value="group.id">{{ group.name }}</option></select></label>
      <div v-if="store.fluctlights.length" class="actor-list">
        <div v-for="fluctlight in filteredFluctlights()" :key="fluctlight.id" class="actor-directory-row"><button class="actor-row" :class="{ selected: fluctlight.id === store.fluctlightId }" type="button" @click="openFluctlight(fluctlight.id)"><span class="avatar">{{ String(fluctlight.identity.name ?? 'F').slice(0, 1) }}</span><span class="actor-row-copy"><strong>{{ String(fluctlight.identity.name ?? fluctlight.id) }}</strong><small>摇光当前状态：{{ fluctlight.status }}</small></span><span class="row-trailing"><time>{{ fluctlight.id === store.fluctlightId ? '现在' : '—' }}</time><span v-if="fluctlight.unread_count" class="unread-badge">{{ fluctlight.unread_count }}</span></span></button><select v-if="controlCenter.actorGroups.length" :aria-label="'为 ' + fluctlight.id + ' 指定分组'" @change="($event) => { const groupId = ($event.target as HTMLSelectElement).value; if (groupId) controlCenter.assignActorGroupMember(groupId, fluctlight.id) }"><option value="">加入分组...</option><option v-for="group in controlCenter.actorGroups.filter((item) => !item.actor_ids.includes(fluctlight.id))" :key="group.id" :value="group.id">{{ group.name }}</option></select><button v-for="group in controlCenter.actorGroups.filter((item) => item.actor_ids.includes(fluctlight.id))" :key="group.id" class="secondary-button" type="button" :disabled="controlCenter.saving" @click="controlCenter.removeActorGroupMember(group.id, fluctlight.id)">移出 {{ group.name }}</button></div>
      </div>
      <button v-if="store.selectedFluctlight" class="detail-toggle" type="button" @click="showInstanceDetails = !showInstanceDetails">{{ showInstanceDetails ? '收起实例详情' : '管理实例详情' }}</button>
      <section v-if="store.selectedFluctlight && showInstanceDetails" class="fluctlight-detail" aria-labelledby="detail-title">
        <h3 id="detail-title">{{ store.selectedFluctlightName }} 的身份设定</h3>
        <dl><template v-for="(value, key) in store.selectedFluctlight.identity" :key="String(key)"><dt>{{ labelFor(String(key)) }}</dt><dd>{{ Array.isArray(value) ? value.join('、') : String(value ?? '未设定') }}</dd></template></dl>
        <template v-if="controlCenter.fluctlightDetail">
          <h3>人格与表达</h3>
          <dl><template v-for="(value, key) in controlCenter.fluctlightDetail.personality as Record<string, unknown>" :key="String(key)"><dt>{{ labelFor(String(key)) }}</dt><dd>{{ typeof value === 'object' ? '已配置' : String(value) }}</dd></template></dl>
          <h3>当前内在状态</h3>
          <dl><template v-for="(value, key) in (controlCenter.fluctlightDetail.inner_state as Record<string, unknown>).pad as Record<string, unknown>" :key="String(key)"><dt>PAD · {{ String(key) }}</dt><dd>{{ String(value) }}</dd></template><dt>情绪</dt><dd>{{ String(((controlCenter.fluctlightDetail.inner_state as Record<string, any>).mood?.label) ?? '未形成') }}</dd><dt>Context</dt><dd>{{ String((controlCenter.fluctlightDetail.context as Record<string, any>)?.scene ?? '待确认') }} · {{ String((controlCenter.fluctlightDetail.context as Record<string, any>)?.activity ?? '待规划') }}</dd></dl>
          <h3>目标与意图</h3>
          <p v-if="!(controlCenter.fluctlightDetail.goals as unknown[])?.length" class="field-note">当前没有活跃目标。</p>
          <ul v-else class="detail-list"><li v-for="goal in controlCenter.fluctlightDetail.goals as Array<Record<string, unknown>>" :key="String(goal.id)">{{ goal.description }} · {{ goal.status }} · {{ goal.progress }}</li></ul>
          <p v-if="!(controlCenter.fluctlightDetail.intentions as unknown[])?.length" class="field-note">当前没有待执行意图。</p>
          <ul v-else class="detail-list"><li v-for="intention in controlCenter.fluctlightDetail.intentions as Array<Record<string, unknown>>" :key="String(intention.id)">{{ intention.action }} · {{ intention.status }}</li></ul>
          <h3>今日 Schedule</h3>
          <p v-if="!controlCenter.fluctlightDetail.schedule" class="field-note">日程待生成，当前没有接受的本地日计划。</p>
          <ul v-else class="detail-list"><li v-for="item in (controlCenter.fluctlightDetail.schedule as Record<string, any>).items" :key="String(item.id)">{{ item.start_at }} - {{ item.end_at }} · {{ item.activity }} · {{ item.scene }}</li></ul>
          <button v-if="controlCenter.fluctlightDetail.schedule" class="secondary-button" type="button" :disabled="controlCenter.saving" @click="controlCenter.cancelSchedule(store.fluctlightId)">取消当前日程</button>
          <form class="revision-form" @submit.prevent="controlCenter.acceptSchedule(store.fluctlightId)"><textarea v-model="controlCenter.scheduleDraftJson" aria-label="完整日程 JSON" rows="8" spellcheck="false" placeholder='{"localDate":"2026-08-26","timezone":"Asia/Shanghai","items":[{"startAt":"2026-08-26T00:00:00+08:00","endAt":"2026-08-27T00:00:00+08:00","activity":"休息","scene":"家"}]}' /><button class="secondary-button" type="submit" :disabled="controlCenter.saving || !controlCenter.scheduleDraftJson.trim()">提交完整日程</button></form>
          <h3>Event 与 Presence</h3>
          <form class="revision-form" @submit.prevent="controlCenter.createLifeEvent(store.fluctlightId)"><input v-model="controlCenter.lifeEvent.kind" aria-label="Event 类型" maxlength="128" placeholder="Event 类型" /><input v-model="controlCenter.lifeEvent.startAt" aria-label="Event 开始时间" type="datetime-local" /><input v-model="controlCenter.lifeEvent.endAt" aria-label="Event 结束时间" type="datetime-local" /><input v-model="controlCenter.lifeEvent.scene" aria-label="Event 场景" maxlength="512" placeholder="场景（可选）" /><input v-model="controlCenter.lifeEvent.activity" aria-label="Event 活动" maxlength="512" placeholder="活动（可选）" /><input v-model="controlCenter.lifeEvent.location" aria-label="Event 地点" maxlength="512" placeholder="地点（可选）" /><button class="secondary-button" type="submit" :disabled="controlCenter.saving">创建确认 Event</button></form>
          <ul v-if="(controlCenter.fluctlightDetail.events as unknown[])?.length" class="detail-list"><li v-for="event in controlCenter.fluctlightDetail.events as Array<Record<string, unknown>>" :key="String(event.id)">{{ event.kind }} · {{ event.start_at }} - {{ event.end_at }} · {{ event.status }}<button v-if="event.status === 'confirmed'" class="secondary-button revision-accept" type="button" :disabled="controlCenter.saving" @click="controlCenter.cancelLifeEvent(store.fluctlightId, String(event.id))">取消 Event</button></li></ul>
          <form class="revision-form" @submit.prevent="controlCenter.setPresence(store.fluctlightId)"><input v-model="controlCenter.presence.userPresence" aria-label="用户 Presence" maxlength="128" placeholder="用户 Presence" /><input v-model="controlCenter.presence.currentTask" aria-label="当前任务 overlay" maxlength="512" placeholder="当前任务 overlay" /><button class="secondary-button" type="submit" :disabled="controlCenter.saving">更新 Presence</button></form>
          <h3>关系与记忆</h3>
          <p v-if="!(controlCenter.fluctlightDetail.relationships as unknown[])?.length" class="field-note">尚未形成关系状态。</p>
          <ul v-else class="detail-list"><li v-for="relationship in controlCenter.fluctlightDetail.relationships as Array<Record<string, unknown>>" :key="String(relationship.target_actor_id)">{{ relationship.target_actor_id }} · {{ relationship.trend }} · r{{ relationship.revision }}<input v-model="controlCenter.relationshipRollbackTargets[String(relationship.target_actor_id)]" aria-label="关系回滚目标 revision" type="number" min="0" step="1" placeholder="目标 r" /><button class="secondary-button revision-accept" type="button" :disabled="controlCenter.saving" @click="controlCenter.rollbackRelationship(store.fluctlightId, relationship)">回滚关系</button></li></ul>
          <p v-if="!(controlCenter.fluctlightDetail.memories as unknown[])?.length" class="field-note">暂无可展示的记忆。</p>
          <ul v-else class="detail-list"><li v-for="memory in controlCenter.fluctlightDetail.memories as Array<Record<string, unknown>>" :key="String(memory.id)">{{ memory.content }}<input v-model="controlCenter.memoryEdits[String(memory.id)]" :aria-label="'修正记忆 ' + memory.id" maxlength="4096" placeholder="修正内容" /><button class="secondary-button revision-accept" type="button" :disabled="controlCenter.saving" @click="controlCenter.reviseMemory(memory)">修正</button><button class="secondary-button revision-accept" type="button" :disabled="controlCenter.saving" @click="controlCenter.forgetMemory(memory)">遗忘</button></li></ul>
          <input v-model="controlCenter.governanceEvidence" aria-label="治理证据引用" maxlength="4096" placeholder="治理证据引用，以逗号分隔" />
          <h3>近期认知</h3>
          <p v-if="!(controlCenter.fluctlightDetail.cognition_history as unknown[])?.length" class="field-note">还没有完成的认知行动。</p>
          <ul v-else class="detail-list"><li v-for="action in controlCenter.fluctlightDetail.cognition_history as Array<Record<string, unknown>>" :key="String(action.id)">{{ action.action_type }} · {{ action.status }}</li></ul>
          <h3>自治动作</h3>
          <p v-if="!controlCenter.autonomyActions.length" class="field-note">当前没有待治理的自治动作。</p>
          <ul v-else class="detail-list"><li v-for="action in controlCenter.autonomyActions" :key="action.id">{{ action.action_type }} · {{ action.status }}<button v-if="action.status === 'frozen' || action.status === 'deferred'" class="secondary-button revision-accept" type="button" :disabled="controlCenter.saving || !controlCenter.governanceReason.trim()" @click="controlCenter.governAutonomyAction(action.id, 'paused', store.fluctlightId)">暂停</button><button v-if="action.status === 'frozen' || action.status === 'deferred' || action.status === 'paused'" class="secondary-button revision-accept" type="button" :disabled="controlCenter.saving || !controlCenter.governanceReason.trim()" @click="controlCenter.governAutonomyAction(action.id, 'cancelled', store.fluctlightId)">取消</button></li></ul>
          <h3>身份与人格修订记录</h3>
          <p v-if="!(controlCenter.fluctlightDetail.foundation_revisions as unknown[])?.length" class="field-note">还没有修订记录。</p>
          <ul v-else class="detail-list"><li v-for="revision in controlCenter.fluctlightDetail.foundation_revisions as Array<Record<string, unknown>>" :key="String(revision.id)">r{{ revision.revision }} · {{ revision.source }} · {{ revision.status }}<span v-if="Object.keys(revision.changes as Record<string, unknown>).length"> · {{ Object.keys(revision.changes as Record<string, unknown>).join('、') }}</span><span v-if="revision.reason"> · {{ revision.reason }}</span><template v-if="revision.status === 'proposed'"><button class="secondary-button revision-accept" type="button" :disabled="controlCenter.saving || !controlCenter.revisionReason.trim()" @click="controlCenter.acceptFoundationRevision(store.fluctlightId, String(revision.id))">接受</button><button class="secondary-button revision-accept" type="button" :disabled="controlCenter.saving || !controlCenter.revisionReason.trim()" @click="controlCenter.rejectFoundationRevision(store.fluctlightId, String(revision.id))">拒绝</button></template></li></ul>
          <form class="revision-form" @submit.prevent="controlCenter.submitFoundationRevision(store.fluctlightId)"><textarea v-model="controlCenter.revisionChangesJson" aria-label="基础修订 JSON" rows="5" placeholder='{"name":"新的名称"}' /><input v-model="controlCenter.revisionReason" aria-label="基础修订原因" maxlength="1024" placeholder="填写修订或接受原因" /><button class="secondary-button" type="submit" :disabled="controlCenter.saving || !controlCenter.revisionChangesJson.trim() || !controlCenter.revisionReason.trim()">提出修订</button></form>
          <form class="revision-form" @submit.prevent="controlCenter.rollbackFoundationRevision(store.fluctlightId)"><input v-model="controlCenter.rollbackTargetRevision" aria-label="回滚目标 revision" type="number" min="0" step="1" placeholder="目标 revision" /><button class="secondary-button" type="submit" :disabled="controlCenter.saving || !controlCenter.rollbackTargetRevision || !controlCenter.revisionReason.trim()">回滚到该 revision</button></form>
          <h3>运行状态治理</h3>
          <p class="field-note">当前状态：{{ controlCenter.fluctlightDetail.status }}。暂停会阻止新的自主外部行为，历史事实和已观察到的状态不会被删除。</p>
          <form class="governance-form" @submit.prevent="controlCenter.setFluctlightStatus(store.fluctlightId, controlCenter.fluctlightDetail?.status === 'paused' ? 'active' : 'paused')"><input v-model="controlCenter.governanceReason" aria-label="状态治理原因" maxlength="1024" placeholder="填写暂停或恢复的原因" /><button class="secondary-button" type="submit" :disabled="controlCenter.saving || !controlCenter.governanceReason.trim()">{{ controlCenter.fluctlightDetail.status === 'paused' ? '恢复自主性' : '暂停自主性' }}</button></form>
        </template>
      </section>
      <div v-else-if="!store.fluctlights.length" class="empty-state compact"><h2>还没有 Fluctlight 实例</h2><p>使用上方输入框创建第一个实例。</p></div>
    </section>

    <section v-else-if="view === 'moments'" class="control-panel moments-panel" aria-labelledby="moments-title">
      <div class="panel-heading"><div><p class="eyebrow">MOMENTS</p><h2 id="moments-title">动态</h2></div><div class="dashboard-legend"><span><i class="legend-dot online" />{{ controlCenter.moments.length }} 条动态</span><span>最近同步 · 刚刚</span></div></div>
      <div class="creation-mode" role="group" aria-label="动态范围"><button class="secondary-button" :class="{ selected: controlCenter.momentsScope === 'global' }" type="button" @click="controlCenter.momentsScope = 'global'; controlCenter.loadMoments(store.fluctlightId)">全部动态</button><button class="secondary-button" :class="{ selected: controlCenter.momentsScope === 'fluctlight' }" type="button" :disabled="!store.selectedFluctlight" @click="controlCenter.momentsScope = 'fluctlight'; controlCenter.loadMoments(store.fluctlightId)">当前实例</button></div>
      <label class="toggle-row"><input v-model="controlCenter.includeHiddenMoments" type="checkbox" @change="controlCenter.loadMoments(store.fluctlightId)" /> 显示已隐藏动态</label>
      <div v-if="controlCenter.loading" class="empty-state compact">正在加载动态...</div>
      <div v-else-if="controlCenter.momentsScope === 'fluctlight' && !store.selectedFluctlight" class="empty-state compact"><h2>请选择一个 Fluctlight 实例</h2></div>
      <div v-else-if="!controlCenter.moments.length" class="empty-state compact"><span class="empty-mark" aria-hidden="true">+</span><h2>暂无动态</h2><p>{{ controlCenter.momentsScope === 'global' ? '所有 Fluctlight 实例都还没有发布动态。' : store.selectedFluctlightName + ' 还没有发布动态。' }}</p></div>
      <p v-if="controlCenter.error" class="error-banner" role="alert">{{ controlCenter.error }}</p>
      <p v-if="controlCenter.momentNotice" class="field-note" role="status">{{ controlCenter.momentNotice }}</p>
      <div v-else class="moment-list"><article v-for="moment in controlCenter.moments" :key="moment.id" class="moment-row" :class="{ hidden: moment.status === 'hidden' }"><p>{{ moment.text }}</p><div v-if="moment.media.length" class="moment-media"><template v-for="asset in moment.media" :key="asset.id"><video v-if="asset.kind === 'video'" :src="mediaUrl(asset.id)" controls preload="metadata" /><audio v-else-if="asset.kind === 'audio'" :src="mediaUrl(asset.id)" controls preload="metadata" /><img v-else :src="mediaUrl(asset.id)" :alt="store.selectedFluctlightName + ' 的动态媒体'" loading="lazy" /></template></div><small>{{ moment.created_at }} · {{ moment.reaction_count }} 个反应<span v-if="controlCenter.momentsScope === 'global'"> · {{ moment.owner_fluctlight_id }}<span v-if="moment.unread_count"> · {{ moment.unread_count }} 条未读</span></span></small><div v-if="moment.comments.length" class="comment-list"><p v-for="comment in moment.comments" :key="comment.id"><strong>{{ comment.author_actor_id }}</strong> {{ comment.text }}</p></div><div class="moment-actions"><button class="secondary-button" type="button" @click="controlCenter.reactToMoment(moment.id, store.fluctlightId)">{{ moment.viewer_reaction ? '已赞' : '赞' }}</button><button class="secondary-button" type="button" @click="controlCenter.setMomentStatus(moment.id, moment.status === 'hidden' ? 'restore' : 'hide', store.fluctlightId)">{{ moment.status === 'hidden' ? '恢复' : '隐藏' }}</button><form @submit.prevent="controlCenter.commentOnMoment(moment.id, store.fluctlightId)"><input v-model="controlCenter.momentDrafts[moment.id]" :aria-label="'评论动态 ' + moment.id" maxlength="32000" placeholder="写评论..." :disabled="moment.status === 'hidden'" /><button class="secondary-button" type="submit" :disabled="moment.status === 'hidden' || !controlCenter.momentDrafts[moment.id]?.trim()">评论</button></form></div></article></div>
    </section>

    <section v-else-if="view === 'diagnostics'" class="control-panel" aria-labelledby="diagnostics-title">
      <div class="panel-heading panel-actions"><div><button class="back-link" type="button" @click="view = 'settings'">← 返回设置</button><p class="eyebrow">CONTROL CENTER</p><h2 id="diagnostics-title">诊断</h2></div><div class="diagnostic-actions"><button class="secondary-button" type="button" @click="controlCenter.exportDiagnostics">导出</button><button class="secondary-button" type="button" @click="controlCenter.clearDiagnostics">清空</button></div></div>
      <form class="diagnostic-filter" @submit.prevent="controlCenter.loadDiagnostics"><input v-model="controlCenter.diagnosticsCorrelationFilter" aria-label="Correlation ID 过滤" placeholder="按 Correlation ID 过滤" /><button class="secondary-button" type="submit">筛选</button></form>
      <p v-if="controlCenter.error" class="error-banner" role="alert">{{ controlCenter.error }}</p>
      <div v-if="controlCenter.loading" class="empty-state compact">正在加载诊断信息...</div>
      <div v-else-if="!controlCenter.diagnostics.length && !controlCenter.diagnosticModelRuns.length" class="empty-state compact"><h2>暂无诊断事件</h2><p>经脱敏的模型、对话和工作流事件会显示在这里。</p></div>
      <template v-else><div v-if="controlCenter.diagnosticModelRuns.length" class="diagnostic-list"><h3>模型运行</h3><article v-for="run in controlCenter.diagnosticModelRuns" :key="run.id" class="diagnostic-row"><div class="diagnostic-meta"><strong>{{ roleLabel(run.role) }}</strong><span>{{ run.status }}</span><small>{{ run.modelId }} · {{ run.correlationId }}</small></div><p v-if="run.errorCode" class="diagnostic-error"><strong>失败原因：</strong>{{ diagnosticFailureReason(run.errorCode) }}<code>{{ run.errorCode }}</code></p><details><summary>Prompt</summary><pre>{{ prettyPayload(run.prompt) }}</pre></details><details v-if="run.response"><summary>Response</summary><pre>{{ prettyPayload(run.response) }}</pre></details></article></div><div v-if="controlCenter.diagnostics.length" class="diagnostic-list"><h3>事件</h3><article v-for="event in controlCenter.diagnostics" :key="event.id" class="diagnostic-row"><div class="diagnostic-meta"><strong>{{ event.eventType }}</strong><span>{{ event.severity }}</span><small>{{ event.correlationId }}</small></div><p v-if="typeof event.payload.error_code === 'string'" class="diagnostic-error"><strong>失败原因：</strong>{{ diagnosticFailureReason(String(event.payload.error_code)) }}<code>{{ event.payload.error_code }}</code></p><pre>{{ prettyPayload(event.payload) }}</pre></article></div></template>
      <section class="fluctlight-detail"><h3>工作流控制</h3><p v-if="controlCenter.workflows.length" class="field-note">{{ controlCenter.workflows.length }} 个工作流记录已加载。</p><form class="revision-form" @submit.prevent="controlCenter.queryWorkflowStatus"><input v-model="controlCenter.workflowId" aria-label="工作流 ID" placeholder="工作流 ID" /><input v-model="controlCenter.workflowHistoryPoint" aria-label="Reset history point" type="number" min="1" step="1" placeholder="Reset history point" /><div class="diagnostic-actions"><button class="secondary-button" type="submit" :disabled="!controlCenter.workflowId.trim()">查询状态</button><button class="secondary-button" type="button" :disabled="!controlCenter.workflowId.trim()" @click="controlCenter.queryWorkflowHistory">历史</button><button class="secondary-button" type="button" :disabled="controlCenter.saving || !controlCenter.workflowId.trim()" @click="controlCenter.commandWorkflow('pause')">暂停</button><button class="secondary-button" type="button" :disabled="controlCenter.saving || !controlCenter.workflowId.trim()" @click="controlCenter.commandWorkflow('resume')">恢复</button><button class="secondary-button" type="button" :disabled="controlCenter.saving || !controlCenter.workflowId.trim()" @click="controlCenter.commandWorkflow('cancel')">取消</button><button class="secondary-button" type="button" :disabled="controlCenter.saving || !controlCenter.workflowId.trim()" @click="controlCenter.restartWorkflow">重启</button><button class="secondary-button" type="button" :disabled="controlCenter.saving || !controlCenter.workflowId.trim() || !controlCenter.workflowHistoryPoint" @click="controlCenter.resetWorkflow">Reset</button></div></form><pre v-if="controlCenter.workflowStatus">{{ prettyPayload(controlCenter.workflowStatus) }}</pre><pre v-if="controlCenter.workflowHistory">{{ prettyPayload(controlCenter.workflowHistory) }}</pre><ul v-if="controlCenter.workflows.length" class="detail-list"><li v-for="workflow in controlCenter.workflows" :key="workflowIdFor(workflow)"><button class="secondary-button" type="button" @click="controlCenter.workflowId = workflowIdFor(workflow); controlCenter.queryWorkflowStatus()">{{ workflowIdFor(workflow) || '未知工作流' }}</button></li></ul></section>
    </section>

    <section v-else class="control-panel" aria-labelledby="settings-title">
      <div class="panel-heading"><div><p class="eyebrow">SYSTEM</p><h2 id="settings-title">设置</h2></div><div class="settings-actions"><span class="status" :class="{ active: store.sending }"><span class="status-dot" aria-hidden="true" />{{ store.sending ? '正在思考' : '就绪' }}</span><button class="secondary-button" type="button" @click="store.logout">退出登录</button><button class="secondary-button" type="button" @click="view = 'diagnostics'">打开诊断中心</button></div></div>
      <p v-if="controlCenter.error" class="error-banner" role="alert">{{ controlCenter.error }}</p>
      <section class="settings-form" aria-labelledby="model-role-title">
        <div class="settings-section-heading"><p class="eyebrow">MODEL ROLES</p><h3 id="model-role-title">模型角色绑定</h3></div>
        <div class="creation-mode" role="tablist" aria-label="模型角色">
          <button v-for="providerRole in providerRoles" :key="providerRole.value" class="secondary-button" :class="{ selected: selectedProviderRole === providerRole.value }" type="button" role="tab" :aria-selected="selectedProviderRole === providerRole.value" @click="selectProviderRole(providerRole.value)">{{ providerRole.label }}</button>
        </div>
        <form class="settings-form" @submit.prevent="saveModelRole">
          <div class="settings-grid">
            <label>使用 Endpoint<select v-model="roleEndpointId" @change="changeRoleEndpoint"><option value="" disabled>请选择 endpoint</option><option v-for="endpoint in controlCenter.providerEndpoints" :key="endpoint.id" :value="endpoint.id">{{ endpoint.id }} · {{ endpoint.base_url }}</option></select></label>
            <label v-if="controlCenter.providerModels.length">模型 ID<select v-model="roleModelId"><option value="" disabled>请选择模型</option><option v-if="roleModelId && !controlCenter.providerModels.includes(roleModelId)" :value="roleModelId">{{ roleModelId }}（当前绑定）</option><option v-for="model in controlCenter.providerModels" :key="model" :value="model">{{ model }}</option></select></label>
            <label v-else>模型 ID<input v-model="roleModelId" type="text" maxlength="256" placeholder="模型标识" /></label>
            <label>Token 预算<input v-model.number="roleTokenBudget" type="number" min="1" step="1" /></label>
            <label>超时（秒）<input v-model.number="roleTimeoutSeconds" type="number" min="1" step="1" /></label>
          </div>
          <p v-if="controlCenter.providerModelsError" class="field-note">{{ controlCenter.providerModelsError }}</p>
          <p v-else-if="!controlCenter.providerModels.length" class="field-note">该 endpoint 尚未返回模型列表，可手动填写模型 ID。</p>
          <p v-else class="field-note">模型列表仅通过 endpoint 的模型清单读取；保存角色不会执行 LLM、流式输出或 Embedding 调用。</p>
          <button class="send-button" type="submit" :disabled="controlCenter.saving">{{ controlCenter.saving ? '正在保存...' : '保存当前角色绑定' }}</button>
        </form>
      </section>
      <form class="settings-form" @submit.prevent="saveProviderEndpoint">
        <div class="settings-section-heading"><p class="eyebrow">ENDPOINTS</p><h3>模型 Endpoint</h3></div>
        <label>已保存 Endpoint<select v-model="endpointPickerId" @change="selectEndpointForEditing(endpointPickerId)"><option value="">新建或手动编辑</option><option v-for="endpoint in controlCenter.providerEndpoints" :key="endpoint.id" :value="endpoint.id">{{ endpoint.id }} · {{ endpoint.base_url }}</option></select></label>
        <div class="settings-grid">
          <label>Endpoint 标识<input v-model="endpointId" type="text" maxlength="128" /></label>
          <label>协议类型<input v-model="providerKind" type="text" maxlength="64" /></label>
          <label>服务地址<input v-model="endpointUrl" type="url" placeholder="http://host:port/v1" /></label>
          <label>访问密钥<input v-model="endpointSecret" type="password" autocomplete="new-password" placeholder="仅写入，不会再次显示" /></label>
        </div>
        <p class="field-note">Endpoint 只管理地址、协议和访问密钥。保存现有 endpoint 会解除其全部模型角色绑定，之后请在上方逐个重新绑定；API Key 仅写入，不会返回浏览器。本地无认证 endpoint 可以不填写访问密钥。</p>
        <button class="send-button" type="submit" :disabled="controlCenter.saving">{{ controlCenter.saving ? '正在保存...' : '保存 Endpoint' }}</button>
      </form>
      <div class="binding-list" aria-label="当前模型角色绑定">
        <h3>当前模型角色绑定</h3>
        <p v-if="!controlCenter.providerBindings.length" class="field-note">尚未启用任何模型角色。</p>
        <div v-for="binding in controlCenter.providerBindings" :key="binding.role" class="binding-row"><strong>{{ roleLabel(binding.role) }}</strong><span>{{ binding.model_id }}</span><small>{{ binding.endpoint_id }} · {{ binding.endpoint_status }} · {{ binding.token_budget }} Token / {{ binding.timeout_seconds }} 秒</small></div>
      </div>
      <div class="binding-list" aria-label="已配置模型 Endpoint">
        <h3>已配置模型 Endpoint</h3>
        <p v-if="!controlCenter.providerEndpoints.length" class="field-note">尚未保存任何模型 Endpoint。</p>
        <div v-for="endpoint in controlCenter.providerEndpoints" :key="endpoint.id" class="binding-row"><strong>{{ endpoint.id }}</strong><span>{{ endpoint.kind }}</span><small>{{ endpoint.base_url }} · {{ endpoint.capability_status }} · {{ endpoint.secret_configured ? 'API Key 已保存' : '未保存 API Key' }}</small><small>{{ endpoint.roles.length ? endpoint.roles.map((item) => roleLabel(item.role) + ': ' + item.model_id).join(' · ') : '尚未绑定任何模型角色' }}</small></div>
      </div>
      <form class="settings-form" @submit.prevent="saveMediaProvider">
        <div class="settings-section-heading"><p class="eyebrow">MEDIA PROVIDER</p><h3>ComfyUI</h3></div>
        <label>ComfyUI URL<input v-model="comfyUiUrl" type="url" placeholder="http://comfyui:8188" /></label>
        <label>图片工作流<textarea v-model="comfyUiWorkflow" rows="8" spellcheck="false" placeholder='{"node": {"inputs": {"text": "{{prompt}}"}}}' /></label>
        <p class="field-note">ComfyUI 是媒体 Provider，不属于 LLM 模型角色。视频与 h3 配置将在同一媒体区域扩展。</p>
        <button class="send-button" type="submit" :disabled="controlCenter.saving">{{ controlCenter.saving ? '正在保存...' : '保存媒体设置' }}</button>
      </form>
      <form class="settings-form" @submit.prevent="controlCenter.saveOperationalSettings">
        <div class="settings-section-heading"><p class="eyebrow">AUTONOMY / DIAGNOSTICS</p><h3>运行策略</h3></div>
        <label>自治策略<textarea v-model="controlCenter.autonomySettingsJson" rows="5" spellcheck="false" placeholder='{"mode":"active","allowed_actions":["proactive_message"],"budget_remaining":1}' /></label>
        <label>诊断保留策略<textarea v-model="controlCenter.diagnosticsRetentionJson" rows="4" spellcheck="false" placeholder='{"retention_days":30,"max_rows":10000}' /></label>
        <button class="send-button" type="submit" :disabled="controlCenter.saving">保存运行策略</button>
      </form>
      <form class="settings-form" @submit.prevent="changeOwnerPassword">
        <div class="settings-section-heading"><p class="eyebrow">OWNER</p><h3>修改所有者密码</h3></div>
        <label for="owner-password">新密码</label>
        <input id="owner-password" v-model="changedOwnerPassword" type="password" autocomplete="new-password" minlength="6" required aria-describedby="owner-password-requirements" />
        <p id="owner-password-requirements" class="field-note">新密码至少 6 个字符。修改后会撤销当前所有会话，需要使用新密码重新登录。</p>
        <p v-if="store.authError" class="error-banner" role="alert">{{ store.authError }}</p>
        <button class="send-button" type="submit" :disabled="store.authLoading || !changedOwnerPassword">{{ store.authLoading ? "正在修改..." : "修改密码" }}</button>
      </form>
    </section>
    </template>

    <nav v-if="view !== 'chat'" class="tabbar" aria-label="Fluctlight 控制中心">
      <button v-for="item in [
        { id: 'fluctlights', label: '实例', icon: '◉' },
        { id: 'moments', label: '动态', icon: '✦' },
        { id: 'settings', label: '设置', icon: '⌘' },
      ]" :key="item.id" class="tab" :class="{ selected: view === item.id }" type="button" @click="selectView(item.id as View)">
        <span class="tab-icon" aria-hidden="true">{{ item.icon }}</span>{{ item.label }}
      </button>
    </nav>
  </main>
</template>

<style>
:root {
  --page-bg: #edf4f6;
  --surface-glass: rgb(255 255 255 / 56%);
  --surface-glass-strong: rgb(255 255 255 / 72%);
  --surface-border: rgb(255 255 255 / 82%);
  --surface-shadow: 0 20px 48px rgb(44 69 80 / 10%);
  --glass-blur: 20px;
  --ink: #182532;
  --muted-ink: #71838c;
  --accent: #2aabee;
  color: var(--ink);
  background: var(--page-bg);
  font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  font-synthesis: none;
}
* { box-sizing: border-box; }
body { margin: 0; min-width: 320px; background: radial-gradient(circle at 12% 0%, #dff0f1 0, transparent 34%), radial-gradient(circle at 90% 12%, #f8e8dd 0, transparent 32%), var(--page-bg); }
button, textarea, input { font: inherit; }
.shell {
  width: min(1100px, calc(100% - 32px));
  min-height: 100vh;
  margin: 0 auto;
  display: grid;
  grid-template-rows: auto 1fr auto auto;
  gap: 16px;
  padding: 24px 0 104px;
}
.shell.chat-shell { padding-bottom: 22px; }
.auth-panel {
  align-self: center;
  width: min(100%, 420px);
  margin: 0 auto;
  padding: 28px;
  border: 1px solid rgb(255 255 255 / 80%);
  border-radius: 24px;
  background: rgb(255 255 255 / 58%);
  box-shadow: 0 24px 70px rgb(41 67 79 / 14%);
  backdrop-filter: blur(22px);
}
.auth-panel h1 { margin: 0 0 8px; }
.auth-copy { color: #61717f; line-height: 1.5; }
.auth-form { display: grid; gap: 10px; margin-top: 22px; }
.auth-form label { color: #42535d; font-size: .86rem; }
.auth-form input { width: 100%; padding: 11px 12px; border: 1px solid rgb(160 184 190 / 48%); border-radius: 10px; background: rgb(255 255 255 / 46%); }
.topbar-actions { display: flex; align-items: center; gap: 12px; }
.topbar { display: flex; align-items: center; justify-content: space-between; gap: 20px; padding: 0 6px; }
.eyebrow { margin: 0 0 5px; color: #5b6c7b; font-size: 11px; font-weight: 750; letter-spacing: .14em; }
h1 { margin: 0; color: #18232e; font-size: clamp(1.35rem, 4vw, 1.9rem); letter-spacing: 0; }
.status { display: inline-flex; align-items: center; gap: 8px; color: #61717f; font-size: .86rem; }
.status-dot { width: 8px; height: 8px; border-radius: 50%; background: #a9b5be; }
.status.active .status-dot { background: #e59050; box-shadow: 0 0 0 4px #f8dfc9; }
.tabbar { position: fixed; z-index: 20; right: 50%; bottom: 16px; display: flex; width: min(520px, calc(100% - 28px)); gap: 8px; padding: 7px; border: 1px solid rgb(255 255 255 / 82%); border-radius: 22px; background: rgb(255 255 255 / 66%); box-shadow: 0 14px 34px rgb(44 69 80 / 16%); transform: translateX(50%); backdrop-filter: blur(22px); }
.tab { flex: 1 1 0; min-height: 46px; padding: 8px 14px; border: 0; border-radius: 15px; background: transparent; color: #6d7e88; cursor: pointer; font-size: .88rem; font-weight: 650; transition: .2s ease; }
.tab-icon { margin-right: 8px; color: #88a0a9; }
.tab:hover, .tab.selected { background: rgb(255 255 255 / 78%); color: #245763; box-shadow: 0 5px 14px rgb(43 75 84 / 10%); }
.tab.selected .tab-icon { color: #326b75; }
.control-panel { min-height: 420px; overflow-y: auto; padding: 26px 10px; border: 1px solid var(--surface-border); border-radius: 24px; background: var(--surface-glass); box-shadow: var(--surface-shadow); backdrop-filter: blur(var(--glass-blur)); }
.moments-panel { min-height: 0; max-height: calc(100vh - 150px); overflow: hidden; }
.moments-panel .moment-list { min-height: 0; max-height: calc(100vh - 330px); overflow-y: auto; padding: 2px 8px 8px 2px; overscroll-behavior: contain; }
.actor-create-form { display: grid; gap: 8px; max-width: 620px; margin-bottom: 18px; color: #42535d; font-size: .86rem; }
.actor-create-row { display: flex; gap: 8px; }
.actor-create-row input { flex: 1; min-height: 36px; padding: 0 10px; border: 1px solid #cbd5db; border-radius: 4px; color: #17232c; }
.actor-create-form textarea { width: 100%; min-height: 96px; padding: 10px; border: 1px solid #cbd5db; border-radius: 4px; color: #17232c; resize: vertical; }
.creation-mode { display: flex; gap: 8px; margin-bottom: 14px; }
.creation-mode .selected { border-color: #8cc4c4; background: rgb(224 244 242 / 78%); color: #245763; }
.panel-heading { display: flex; align-items: flex-end; justify-content: space-between; gap: 16px; margin-bottom: 22px; }
.panel-heading h2 { margin: 0; color: #1f2b35; font-size: 1.25rem; }
.panel-actions { align-items: center; }
.settings-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; justify-content: flex-end; }
.dashboard-legend { display: flex; align-items: center; gap: 14px; color: #71838c; font-size: .76rem; }
.dashboard-legend span { display: inline-flex; align-items: center; gap: 6px; }
.legend-dot { width: 7px; height: 7px; border-radius: 50%; background: #99a8ae; }
.legend-dot.online { background: #4d9ba0; box-shadow: 0 0 0 4px rgb(77 155 160 / 12%); }
.legend-dot.unread { background: #d89667; box-shadow: 0 0 0 4px rgb(216 150 103 / 12%); }
.metric-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; margin: 0 0 24px; }
.metric-card { min-width: 0; padding: 16px; border: 1px solid rgb(255 255 255 / 84%); border-radius: 17px; background: rgb(255 255 255 / 48%); box-shadow: inset 0 1px rgb(255 255 255 / 70%); }
.metric-card span, .metric-card small { display: block; color: #71838c; font-size: .72rem; }
.metric-card strong { display: block; overflow: hidden; margin: 7px 0 5px; color: #254954; font-size: 1.2rem; text-overflow: ellipsis; white-space: nowrap; }
.metric-card.accent { background: linear-gradient(145deg, rgb(225 244 241 / 72%), rgb(255 255 255 / 44%)); }
.list-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 14px; }
.list-header h2 { margin: 0; color: #202b35; font-size: 1.55rem; }
.round-action { display: grid; place-items: center; width: 38px; height: 38px; padding: 0; border: 0; border-radius: 50%; background: #2aabee; color: #fff; cursor: pointer; font-size: 1.45rem; line-height: 1; box-shadow: 0 5px 14px rgb(42 171 238 / 25%); }
.search-box { display: flex; align-items: center; gap: 8px; height: 40px; margin-bottom: 10px; padding: 0 13px; border: 1px solid rgb(213 221 226 / 80%); border-radius: 21px; background: rgb(255 255 255 / 75%); color: #9aa6ad; }
.search-box input { width: 100%; border: 0; outline: 0; background: transparent; color: #25343d; }
.archive-row { display: flex; align-items: center; gap: 10px; width: 100%; margin-bottom: 9px; padding: 9px 12px; border: 1px solid rgb(224 230 233 / 80%); border-radius: 12px; background: rgb(255 255 255 / 48%); color: #87949a; cursor: pointer; text-align: left; }
.archive-row span:nth-child(2) { flex: 1; }
.archive-icon { color: #9ea9ae; }
.create-drawer { margin-bottom: 14px; padding: 14px; border: 1px solid rgb(255 255 255 / 80%); border-radius: 16px; background: rgb(255 255 255 / 48%); }
.detail-toggle { margin: 14px auto 0; display: block; border: 0; background: transparent; color: #557980; cursor: pointer; font-size: .8rem; }
.row-trailing { display: grid; justify-items: end; gap: 5px; margin-left: 8px; color: #8b969c; font-size: .72rem; }
.unread-badge { display: grid; place-items: center; min-width: 21px; height: 21px; padding: 0 5px; border-radius: 11px; background: #b9bbc2; color: #fff; font-size: .68rem; font-weight: 700; }
.chat-header { display: flex; align-items: center; gap: 10px; min-height: 58px; padding: 7px 10px; border: 1px solid var(--surface-border); border-radius: 18px 18px 0 0; background: rgb(255 255 255 / 40%); box-shadow: 0 8px 20px rgb(44 69 80 / 7%); backdrop-filter: blur(var(--glass-blur)); }
.chat-back { border: 0; background: transparent; color: #2aabee; cursor: pointer; font-size: 2rem; line-height: 1; }
.chat-avatar { display: grid; place-items: center; width: 38px; height: 38px; border-radius: 50%; background: linear-gradient(145deg, #5bc1ca, #3d8e98); color: #fff; font-weight: 750; }
.chat-header-copy { display: grid; flex: 1; gap: 2px; }
.chat-header-copy strong { color: #23313a; font-size: .94rem; }
.chat-header-copy small { color: #84939a; font-size: .7rem; }
.empty-state.compact { min-height: 260px; }
.actor-list, .diagnostic-list { display: grid; gap: 10px; }
.actor-directory-row { display: flex; align-items: center; gap: 8px; }
.actor-directory-row .actor-row { flex: 1; min-width: 0; }
.actor-directory-row select { min-width: 124px; min-height: 36px; padding: 0 8px; border: 1px solid rgb(160 184 190 / 48%); border-radius: 10px; background: rgb(255 255 255 / 46%); color: #17232c; }
.actor-row { width: 100%; display: flex; align-items: center; gap: 12px; padding: 14px; border: 1px solid rgb(255 255 255 / 84%); border-radius: 18px; background: rgb(255 255 255 / 58%); color: inherit; text-align: left; cursor: pointer; box-shadow: 0 8px 22px rgb(44 69 80 / 6%); transition: .2s ease; }
.actor-row:hover, .actor-row.selected { border-color: #8ec7c6; background: rgb(240 251 249 / 82%); transform: translateY(-1px); }
.actor-row-copy { display: grid; gap: 3px; flex: 1; min-width: 0; }
.fluctlight-detail { margin-top: 24px; padding-top: 18px; border-top: 1px solid #d8e0e4; }
.fluctlight-detail h3 { margin: 0 0 12px; font-size: 1rem; }
.fluctlight-detail dl { display: grid; grid-template-columns: 130px minmax(0, 1fr); gap: 8px 14px; margin: 0; }
.fluctlight-detail dt { color: #667783; font-size: .82rem; }
.fluctlight-detail dd { margin: 0; overflow-wrap: anywhere; }
.detail-list { display: grid; gap: 6px; margin: 8px 0 16px; padding-left: 20px; }
.detail-list li { overflow-wrap: anywhere; }
.governance-form { display: flex; gap: 8px; margin: 8px 0 16px; }
.governance-form input { flex: 1; min-width: 0; min-height: 36px; padding: 0 10px; border: 1px solid #cbd5db; border-radius: 4px; }
.revision-form { display: grid; gap: 8px; margin: 8px 0 16px; }
.revision-form textarea, .revision-form input { width: 100%; min-height: 36px; padding: 9px 10px; border: 1px solid #cbd5db; border-radius: 4px; }
.revision-accept { margin-left: 8px; }
.load-older { display: block; margin: 0 auto 12px; }
.moment-list { display: grid; gap: 12px; }
.moment-row { padding: 18px; border: 1px solid rgb(255 255 255 / 84%); border-radius: 18px; background: rgb(255 255 255 / 58%); box-shadow: 0 10px 26px rgb(44 69 80 / 6%); }
.moment-row p { margin: 0 0 10px; line-height: 1.55; white-space: pre-wrap; }
.moment-row small { color: #778690; }
.moment-media { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 8px; margin: 10px 0; }
.moment-media img, .moment-media video { width: 100%; max-height: 360px; object-fit: cover; border-radius: 4px; background: #eef1f4; }
.moment-media audio { width: 100%; }
.moment-row.hidden { opacity: .66; background: #f6f7f8; }
.comment-list { display: grid; gap: 5px; margin-top: 12px; padding-top: 10px; border-top: 1px solid #e1e6e9; }
.comment-list p { margin: 0; font-size: .88rem; }
.toggle-row { display: flex; align-items: center; gap: 8px; margin: -10px 0 14px; color: #536670; font-size: .86rem; }
.moment-actions { display: flex; align-items: center; gap: 10px; margin-top: 12px; }
.moment-actions form { display: flex; flex: 1; gap: 8px; }
.moment-actions input { flex: 1; min-width: 0; min-height: 34px; padding: 0 9px; border: 1px solid #cbd5db; border-radius: 4px; }
.actor-row small, .diagnostic-meta small { color: #7b8992; font-size: .76rem; }
.state-label { color: #36707b; font-size: .78rem; }
.diagnostic-row { padding: 14px; border: 1px solid rgb(255 255 255 / 82%); border-radius: 16px; background: rgb(255 255 255 / 56%); }
.diagnostic-meta { display: flex; align-items: center; gap: 9px; flex-wrap: wrap; }
.diagnostic-meta span { color: #8e5c2e; font-size: .75rem; text-transform: uppercase; }
.diagnostic-row pre { max-height: 180px; overflow: auto; margin: 10px 0 0; padding: 10px; background: #f5f7f8; color: #41515c; font: .76rem/1.45 ui-monospace, SFMono-Regular, Menlo, monospace; white-space: pre-wrap; overflow-wrap: anywhere; }
.diagnostic-error { margin: 10px 0 0; color: #9c2f25; font-size: .86rem; line-height: 1.5; }
.diagnostic-error code { display: block; margin-top: 4px; color: #6f4a46; font: .76rem/1.35 ui-monospace, SFMono-Regular, Menlo, monospace; overflow-wrap: anywhere; }
.message-media { display: grid; gap: 8px; margin-top: 10px; }
.message-media img { display: block; width: min(100%, 360px); max-height: 460px; object-fit: cover; border: 1px solid #d8e0e4; border-radius: 6px; background: #f5f7f8; }
.diagnostic-actions, .diagnostic-filter { display: flex; gap: 8px; }
.diagnostic-filter { margin: -10px 0 14px; }
.diagnostic-filter input { flex: 1; min-width: 0; min-height: 36px; padding: 0 10px; border: 1px solid #cbd5db; border-radius: 4px; }
.settings-form { display: grid; gap: 16px; max-width: 720px; margin-bottom: 16px; padding: 20px; border: 1px solid rgb(255 255 255 / 82%); border-radius: 18px; background: rgb(255 255 255 / 56%); box-shadow: 0 10px 26px rgb(44 69 80 / 5%); }
.settings-section-heading h3 { margin: 0; color: #1f2b35; font-size: 1rem; }
.binding-list { display: grid; gap: 8px; max-width: 620px; margin: 0 0 16px; }
.binding-list h3 { margin: 0 0 4px; font-size: 1rem; }
.binding-row { display: grid; grid-template-columns: 150px minmax(0, 1fr); gap: 3px 12px; padding: 10px 12px; border: 1px solid #d8e0e4; border-radius: 6px; background: #fff; }
.binding-row small { grid-column: 1 / -1; color: #778690; overflow-wrap: anywhere; }
.settings-form label { display: grid; gap: 7px; color: #42535d; font-size: .86rem; }
.settings-form input { min-height: 40px; padding: 0 10px; border: 1px solid rgb(160 184 190 / 48%); border-radius: 10px; background: rgb(255 255 255 / 46%); color: #17232c; }
.settings-form select { min-height: 40px; padding: 0 10px; border: 1px solid rgb(160 184 190 / 48%); border-radius: 10px; background: rgb(255 255 255 / 46%); color: #17232c; }
.settings-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.field-note { margin: -3px 0 0; color: #778690; font-size: .78rem; }
.transcript { min-height: 420px; max-height: calc(100vh - 260px); overflow-y: auto; padding: 20px 10px; border: 1px solid rgb(255 255 255 / 78%); border-radius: 24px; background: rgb(255 255 255 / 30%); box-shadow: 0 22px 54px rgb(44 69 80 / 9%); backdrop-filter: blur(20px); }
.empty-state { min-height: 360px; display: grid; place-content: center; justify-items: center; color: #5d6d7b; text-align: center; }
.empty-mark { display: grid; place-items: center; width: 40px; height: 40px; margin-bottom: 16px; border: 1px solid #a7b6c0; border-radius: 50%; color: #3c7080; font-size: 1.5rem; }
.empty-state h2 { margin: 0; color: #1f2b35; font-size: 1.2rem; }
.empty-state p { max-width: 320px; margin: 8px 0 0; line-height: 1.5; }
.message-row { display: flex; gap: 12px; max-width: 78%; margin: 16px 0; }
.from-user { margin-left: auto; flex-direction: row-reverse; }
.avatar { flex: 0 0 30px; width: 30px; height: 30px; display: grid; place-items: center; border-radius: 50%; background: #d4e6e7; color: #27545e; font-size: .72rem; font-weight: 800; }
.from-user .avatar { background: #f5dfcf; color: #805033; }
.message-bubble { padding: 12px 15px 9px; border: 1px solid rgb(255 255 255 / 90%); border-radius: 8px 18px 18px 18px; background: rgb(255 255 255 / 78%); box-shadow: 0 6px 18px rgb(39 55 66 / 6%); }
.from-user .message-bubble { border-color: #e8d6c8; border-radius: 18px 8px 18px 18px; background: rgb(255 249 244 / 82%); }
.message-bubble p { margin: 0; line-height: 1.55; white-space: pre-wrap; overflow-wrap: anywhere; }
.attachment-chip { display: inline-block; margin-top: 9px; color: #36707b; font-size: .75rem; }
.error-banner { margin: 0; padding: 10px 12px; border-left: 3px solid #bd584f; background: #fff3f1; color: #8b3933; font-size: .88rem; }
.composer { padding: 14px 16px; border: 1px solid rgb(255 255 255 / 88%); border-radius: 20px; background: rgb(255 255 255 / 64%); box-shadow: 0 16px 34px rgb(39 55 66 / 9%); backdrop-filter: blur(18px); }
.composer textarea { display: block; width: 100%; min-height: 56px; resize: vertical; border: 0; outline: 0; color: #17232c; line-height: 1.5; }
.composer textarea::placeholder { color: #9aa7b0; }
.composer-footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-top: 10px; }
.attachment-input { display: flex; align-items: center; gap: 7px; min-width: 0; color: #637582; }
.attachment-input > span { display: grid; place-items: center; width: 24px; height: 24px; border: 1px solid #b8c5cc; border-radius: 50%; color: #47717c; }
.attachment-input input { width: min(260px, 35vw); border: 0; outline: 0; color: #41515c; font-size: .8rem; }
.composer-actions { display: flex; gap: 8px; }
.send-button, .secondary-button { min-height: 36px; padding: 0 14px; border-radius: 11px; cursor: pointer; transition: .2s ease; }
.send-button { border: 1px solid #326b75; background: linear-gradient(135deg, #367b84, #245763); color: #fff; box-shadow: 0 7px 16px rgb(43 98 105 / 20%); }
.send-button:disabled { cursor: not-allowed; opacity: .45; }
.secondary-button { border: 1px solid rgb(255 255 255 / 90%); background: rgb(255 255 255 / 58%); color: #435661; box-shadow: 0 5px 14px rgb(44 69 80 / 5%); }
.secondary-button:hover { background: rgb(255 255 255 / 82%); transform: translateY(-1px); }
.back-link { display: inline-flex; margin: 0 0 12px; padding: 0; border: 0; background: transparent; color: #50747c; cursor: pointer; font-size: .82rem; }
.message-meta { display: flex; justify-content: flex-end; align-items: center; gap: 7px; margin-top: 7px; color: #8a999f; font-size: .68rem; }
.delivery-status { color: #8a999f; letter-spacing: -2px; font-size: .72rem; line-height: 1; }
.delivery-status.pending { color: #d49564; letter-spacing: -1px; }
.delivery-status.sent { color: #3d8e98; }
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0,0,0,0); white-space: nowrap; border: 0; }
@media (max-width: 640px) {
  body { background: #eef3f7; }
  .shell { width: 100%; min-height: 100dvh; margin: 0; padding: 0 0 96px; gap: 0; display: flex; flex-direction: column; }
  .shell.chat-shell { padding-bottom: 12px; }
  .settings-actions { gap: 5px; }
  .settings-actions .status { font-size: .72rem; }
  .settings-actions .secondary-button { min-height: 32px; padding: 0 9px; font-size: .72rem; }
  .transcript { flex: 1; min-height: 0; max-height: none; padding: 14px 10px 18px; border: 0; border-radius: 0; background: radial-gradient(circle at 20% 10%, rgb(255 255 255 / 52%) 0, transparent 32%), #dfe9f1; box-shadow: none; }
  .chat-header { flex: 0 0 auto; min-height: 58px; border: 0; border-radius: 0; background: rgb(255 255 255 / 42%); box-shadow: 0 1px rgb(217 224 229 / 65%); backdrop-filter: blur(18px); }
  .chat-header + .transcript { min-height: 0; }
  .message-row { max-width: 92%; }
  .chat-shell .topbar { display: none; }
  .chat-shell .message-row .avatar { display: grid; width: 32px; height: 32px; flex-basis: 32px; font-size: .68rem; }
  .chat-shell .message-row { max-width: 90%; margin: 5px 0; gap: 7px; }
  .chat-shell .from-user { margin-left: auto; }
  .chat-shell .message-bubble { border: 0; border-radius: 5px 14px 14px 14px; padding: 8px 10px 6px; box-shadow: 0 1px 2px rgb(68 91 105 / 12%); background: #fff; font-size: .9rem; }
  .chat-shell .from-user .message-bubble { border-radius: 14px 5px 14px 14px; background: #effdde; }
  .control-panel { flex: 1; min-height: 0; padding: 16px 12px; border: 0; border-radius: 0; box-shadow: none; }
  .fluctlights-panel { padding: 12px 0 0; background: #fff; }
  .fluctlights-panel .list-header, .fluctlights-panel .search-box, .fluctlights-panel .archive-row { margin-left: 14px; margin-right: 14px; width: calc(100% - 28px); }
  .fluctlights-panel .list-header { margin-bottom: 10px; }
  .fluctlights-panel .list-header h2 { font-size: 1.45rem; }
  .fluctlights-panel .actor-list { margin-top: 4px; }
  .fluctlights-panel .actor-directory-row { gap: 0; border-bottom: 1px solid #edf0f2; }
  .fluctlights-panel .actor-row { min-height: 72px; padding: 10px 14px; border: 0; border-radius: 0; background: #fff; box-shadow: none; transform: none; }
  .fluctlights-panel .actor-row:hover, .fluctlights-panel .actor-row.selected { background: #f5f8fa; }
  .fluctlights-panel .avatar { width: 48px; height: 48px; flex-basis: 48px; background: linear-gradient(145deg, #dc8aed, #c64be6); color: #fff; font-size: 1rem; }
  .fluctlights-panel .actor-row-copy strong { color: #20262b; font-size: .94rem; }
  .fluctlights-panel .actor-row-copy small { color: #929da4; font-size: .78rem; }
  .fluctlights-panel .actor-directory-row > select, .fluctlights-panel .actor-directory-row > .secondary-button, .fluctlights-panel .toggle-row, .fluctlights-panel .metric-grid, .fluctlights-panel .fluctlight-detail, .fluctlights-panel .detail-toggle { display: none; }
  .row-trailing { margin-left: auto; }
  .composer { flex: 0 0 auto; padding: 6px 8px calc(6px + env(safe-area-inset-bottom)); border: 0; border-radius: 0; background: #f5f7f9; box-shadow: 0 -1px #d5dce2; }
  .composer textarea { min-height: 40px; max-height: 120px; padding: 9px 12px; border: 1px solid #d0d8df; border-radius: 20px; background: #fff; }
  .composer-footer { align-items: center; margin-top: 6px; }
  .attachment-input input { display: none; }
  .attachment-input > span { width: 32px; height: 32px; border: 1px solid #d0d8df; background: #fff; }
  .composer-actions .send-button { min-width: 40px; width: 40px; height: 40px; padding: 0; border-radius: 50%; font-size: 0; }
  .composer-actions .send-button::after { content: '➤'; font-size: 1rem; }
  .settings-actions { width: 100%; justify-content: flex-start; padding-top: 8px; border-top: 1px solid rgb(199 211 217 / 45%); }
  .settings-form { width: 100%; max-width: none; margin-bottom: 12px; padding: 16px; }
  .settings-form .settings-form { padding: 0; border: 0; background: transparent; box-shadow: none; }
  .settings-form textarea { max-width: 100%; }
  .binding-list { width: 100%; max-width: none; }
  .binding-row { grid-template-columns: 1fr; gap: 4px; }
  .binding-row small { grid-column: auto; }
  .settings-form .creation-mode { flex-wrap: wrap; }
  .settings-form .creation-mode .secondary-button { flex: 1 1 42%; min-width: 0; }
  .attachment-input input { width: min(180px, 45vw); }
  .settings-grid { grid-template-columns: 1fr; }
  .metric-grid { grid-template-columns: 1fr; }
  .dashboard-legend { display: none; }
  .tabbar { bottom: 10px; width: calc(100% - 20px); }
  .moments-panel { max-height: calc(100vh - 132px); padding-inline: 6px; }
  .moments-panel .moment-list { max-height: calc(100vh - 320px); }
  .send-button, .secondary-button { min-height: 40px; }
}
</style>

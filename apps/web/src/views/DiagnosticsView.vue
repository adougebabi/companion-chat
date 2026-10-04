<script setup lang="ts">
import { computed, onMounted, onUnmounted, watch } from "vue";

import Accordion from "@/components/ui/accordion/Accordion.vue";
import AccordionContent from "@/components/ui/accordion/AccordionContent.vue";
import AccordionItem from "@/components/ui/accordion/AccordionItem.vue";
import AccordionTrigger from "@/components/ui/accordion/AccordionTrigger.vue";
import Badge from "@/components/ui/badge/Badge.vue";
import Button from "@/components/ui/button/Button.vue";
import Input from "@/components/ui/input/Input.vue";
import { diagnosticsSections, type DiagnosticsSection } from "../app/navigation";
import type { BrowserDiagnosticAgentRun, BrowserDiagnosticModelRun } from "@fluctlight/browser-client";
import { formatInstantInZone } from "../lib/instant";
import { useControlCenterStore } from "../stores/control-center";

const props = defineProps<{ section?: DiagnosticsSection | null }>();
const emit = defineEmits<{ navigateSection: [section: DiagnosticsSection | null] }>();
const controlCenter = useControlCenterStore();
const currentSection = computed(() => props.section ?? null);
const scenarioLabels: Record<string, string> = { reply: "回复生成", autonomy_reply: "自治回复", cognitive_assessment: "认知判断", native_cognition: "原生事件认知", daily_review: "日评", schedule_generation: "计划生成", reflection: "反思", wake_up: "唤醒", initialization: "初始化", media_prompt: "媒体提示词", embedding: "Embedding" };
const bindingLabels: Record<string, string> = { generic_llm: "通用 LLM", embedding: "Embedding" };
const statusLabels: Record<string, string> = { queued: "排队中", running: "执行中", started: "已启动", scheduled: "已计划", retry: "待重试", completed: "已完成", no_op: "无操作", blocked: "已阻止", paused: "已暂停", inactive: "未激活", disabled: "已禁用", overdue: "已逾期", dead_letter: "已终止", failed: "失败", cancelled: "已取消", timeout: "超时" };
const mediaFailureStageLabels: Record<string, string> = { prepare: "准备提示词", submit: "提交 ComfyUI", poll_quality: "轮询或质量检查" };
const agentFailureStageLabels: Record<string, string> = { tool: "Tool 执行", model: "模型请求", model_input: "模型输入预算", cancellation: "取消", agent: "Agent 结果", unknown: "阶段未知" };
function scenarioLabel(scenario: string) { return scenarioLabels[scenario] ?? scenario; }
function bindingLabel(role: string) { return bindingLabels[role] ?? role; }
function statusLabel(status: string) { return statusLabels[status] ?? status; }
function statusClass(status: string) { return `run-${status}`; }
const queueSummary = computed(() => {
  const counts = new Map<string, number>();
  for (const run of controlCenter.diagnosticModelRuns) {
    const role = run.bindingRole || run.role;
    const count = Number(run.queuePendingCount ?? 0);
    if (Number.isFinite(count)) counts.set(role, Math.max(counts.get(role) ?? 0, count));
  }
  return [...counts.entries()].map(([role, count]) => `${bindingLabel(role)} ${count}`).join(" · ");
});
const viewerTimezone = computed(() => controlCenter.diagnosticDisplayTimezone);
const displayTimezones = [...new Set([controlCenter.diagnosticDisplayTimezone,"UTC","Asia/Shanghai","America/New_York","Europe/Berlin"])];
const modelRunGroups = computed(() => {
  const groups = new Map<string, { id: string; correlationId: string; runs: BrowserDiagnosticModelRun[]; agents: BrowserDiagnosticAgentRun[]; total: number }>();
  for (const run of controlCenter.diagnosticModelRuns) {
    const id = controlCenter.diagnosticsCorrelationFilter ? (run.logicalRunId || run.correlationId || run.id) : run.id;
    let group = groups.get(id);
    if (!group) {
      group = { id, correlationId: run.correlationId, runs: [], agents: [], total: 0 };
      groups.set(id, group);
    }
    group.runs.push(run);
    group.total = Math.max(group.total, Number(run.roundCount ?? 0));
  }
  if (controlCenter.diagnosticsCorrelationFilter) for (const group of groups.values()) {
    group.agents = controlCenter.diagnosticAgentRuns.filter((agent) => agent.correlationId === group.correlationId);
  }
  for (const group of groups.values()) group.runs.sort((a, b) => {
    if (a.sequence != null && b.sequence != null) return a.sequence - b.sequence || a.id.localeCompare(b.id);
    if (a.sequence != null) return -1;
    if (b.sequence != null) return 1;
    return (a.queuedAt || a.createdAt).localeCompare(b.queuedAt || b.createdAt) || a.id.localeCompare(b.id);
  });
  // The default list keeps the globally paged server order. A correlation
  // detail groups physical calls and orders steps by their execution sequence.
  return [...groups.values()];
});
function modelRoundLabel(run: BrowserDiagnosticModelRun, total: number): string {
  return run.sequence != null ? `第 ${run.sequence}/${Math.max(total, run.sequence)} 次` : "轮次未知";
}
function modelStageLabel(stage?: string): string {
  return ({ tool_request: "请求 Tool · 中间响应", final_response: "最终模型回答", failed: "请求失败", cancelled: "已取消", timeout: "请求超时", pending: "等待模型响应", unknown: "阶段未知" } as Record<string, string>)[stage || "unknown"] || "阶段未知";
}
function pretty(value: unknown): string { return JSON.stringify(value, null, 2) ?? String(value); }
function diagnosticText(value: unknown): string {
  let messages: unknown[] | null = null;
  if (Array.isArray(value)) messages = value;
  else if (value && typeof value === "object") {
    const nested = (value as Record<string, unknown>).messages;
    if (Array.isArray(nested)) messages = nested;
  }
  if (messages) return messages.map((message) => diagnosticText(message)).join("\n\n");
  if (!value || typeof value !== "object" || Array.isArray(value) || !("role" in value)) return pretty(value);
  const message = value as Record<string, unknown>;
  const content = Array.isArray(message.content)
    ? message.content.map((part) => {
        if (!part || typeof part !== "object") return pretty(part);
        const item = part as Record<string, unknown>;
        if (item.type === "text") return String(item.text ?? "");
        if (item.type === "image_url") return `[图片：${String((item.image_url as Record<string, unknown> | undefined)?.url ?? "REDACTED_IMAGE_DATA")}]`;
        return pretty(item);
      }).join("\n")
    : typeof message.content === "string" ? message.content : pretty(message.content);
  const extra = Object.fromEntries(Object.entries(message).filter(([key, item]) => key !== "role" && key !== "content" && item != null && item !== "" && (!Array.isArray(item) || item.length > 0)));
  return `[${String(message.role)}]\n${content}${Object.keys(extra).length ? `\n${pretty(extra)}` : ""}`;
}
function isMetadataOnlyPrompt(prompt: unknown): boolean {
  if (!prompt || typeof prompt !== "object") return false;
  if ("diagnostic_scope" in (prompt as Record<string, unknown>) && (prompt as Record<string, unknown>).diagnostic_scope === "metadata_only") return true;
  if (Array.isArray(prompt)) {
    const first = prompt[0] as Record<string, unknown> | undefined;
    if (first?.role === "diagnostic" && typeof first?.content === "object" && (first?.content as Record<string, unknown>)?.diagnostic_scope === "metadata_only") return true;
  }
  return false;
}
function formatRunTime(value: string): string {
  return formatInstantInZone(value, viewerTimezone.value);
}

function workflowIdFor(value: Record<string, unknown>): string {
  if (typeof value.workflow_id === "string") return value.workflow_id;
  if (typeof value.workflowId === "string") return value.workflowId;
  for (const nested of Object.values(value)) if (nested && typeof nested === "object" && !Array.isArray(nested)) { const id: string = workflowIdFor(nested as Record<string, unknown>); if (id) return id; }
  return "";
}
function applyDiagnosticsFilters() { void controlCenter.loadDiagnostics(); }
function clearLifecycleFilters() {
  controlCenter.diagnosticsFluctlightFilter = "";
  controlCenter.diagnosticsCorrelationFilter = "";
  controlCenter.diagnosticsIntentFilter = "";
  controlCenter.diagnosticsWorkflowFilter = "";
  controlCenter.diagnosticsRunFilter = "";
  controlCenter.diagnosticsSurfaceFilter = "";
  controlCenter.diagnosticsStatusFilter = "";
  void controlCenter.loadDiagnostics();
}
function openWorkflow(workflowId?: string) {
  if (!workflowId) return;
  controlCenter.workflowId = workflowId;
  emit("navigateSection", "workflows");
  void controlCenter.queryWorkflowStatus();
}
function openModelRun(correlationId: string) {
  controlCenter.diagnosticsCorrelationFilter = correlationId;
  emit("navigateSection", "model-runs");
  void controlCenter.loadDiagnostics();
}
let pollTimer: number | undefined;
onMounted(() => {
  const correlationId = new URLSearchParams(window.location.search).get("correlation_id") ?? "";
  controlCenter.diagnosticsCorrelationFilter = correlationId;
  void controlCenter.loadDiagnostics();
  if (currentSection.value === "workflows") void controlCenter.loadWorkflows();
  pollTimer = window.setInterval(() => {
    if (document.visibilityState === "visible") void controlCenter.loadDiagnostics(true);
  }, 2000);
});
watch(currentSection, (section) => {
  if (section === "workflows") void controlCenter.loadWorkflows();
});
onUnmounted(() => { if (pollTimer !== undefined) window.clearInterval(pollTimer); });
</script>

<template>
  <section class="page diagnostics-page" aria-labelledby="diagnostics-title">
    <header class="page-header">
      <div><h1 id="diagnostics-title">诊断中心</h1></div>
    </header>
    <p v-if="controlCenter.error" class="error-banner" role="alert">{{ controlCenter.error }}</p>
    <section v-if="!currentSection" class="diagnostics-overview" aria-labelledby="diagnostics-overview-title">
      <p class="eyebrow">OBSERVABILITY</p>
      <h2 id="diagnostics-overview-title">选择一个诊断项</h2>
      <p class="page-lede">从左侧列表选择模型运行、系统事件或工作流控制。</p>
      <nav class="diagnostics-mobile-section-list" aria-label="诊断选项">
        <Button v-for="section in diagnosticsSections" :key="section.id" class="diagnostics-mobile-section-link" variant="outline" type="button" @click="emit('navigateSection', section.id)">
          <span><strong>{{ section.label }}</strong><small>{{ section.description }}</small></span>
          <span aria-hidden="true">›</span>
        </Button>
      </nav>
    </section>

    <template v-else>
      <header class="diagnostics-detail-header">
        <Button class="back-link" variant="ghost" type="button" @click="emit('navigateSection', null)">‹ 返回诊断中心</Button>
        <div><p class="eyebrow">OBSERVABILITY</p><h2>{{ diagnosticsSections.find((item) => item.id === currentSection)?.label }}</h2><p class="field-note">{{ diagnosticsSections.find((item) => item.id === currentSection)?.description }}</p></div>
      </header>
      <form v-if="currentSection === 'lifecycle'" class="lifecycle-filter-grid" aria-label="生命周期过滤" @submit.prevent="applyDiagnosticsFilters">
        <label>摇光 ID<Input v-model="controlCenter.diagnosticsFluctlightFilter" placeholder="fluctlight_id" /></label>
        <label>关联 ID<Input v-model="controlCenter.diagnosticsCorrelationFilter" placeholder="correlation_id" /></label>
        <label>Intent ID<Input v-model="controlCenter.diagnosticsIntentFilter" placeholder="intent_id" /></label>
        <label>Workflow ID<Input v-model="controlCenter.diagnosticsWorkflowFilter" placeholder="workflow_id" /></label>
        <label>Run ID<Input v-model="controlCenter.diagnosticsRunFilter" placeholder="run_id" /></label>
        <label>生命周期<Input v-model="controlCenter.diagnosticsSurfaceFilter" placeholder="wake_up / reflection" /></label>
        <label>状态<Input v-model="controlCenter.diagnosticsStatusFilter" placeholder="retry / failed / no_op" /></label>
        <div class="lifecycle-filter-actions"><Button type="submit">应用过滤</Button><Button variant="outline" type="button" @click="clearLifecycleFilters">清除</Button><Button variant="outline" type="button" @click="controlCenter.exportDiagnostics">导出当前过滤</Button></div>
      </form>
      <div v-if="controlCenter.loading" class="empty-panel compact">正在加载诊断信息...</div>
      <div v-else-if="(currentSection === 'lifecycle' && !controlCenter.lifecycleDiagnostics.length && !controlCenter.workflowIntentSnapshots.length) || (currentSection === 'model-runs' && !controlCenter.diagnosticModelRuns.length && !controlCenter.diagnosticAgentRuns.length) || (currentSection === 'media-prompts' && !controlCenter.diagnosticMediaPrompts.length) || (currentSection === 'events' && !controlCenter.diagnostics.length)" class="empty-panel compact"><h2>暂无当前诊断记录</h2><p>{{ currentSection === 'lifecycle' ? '没有匹配的触发或工作流状态；可清除过滤查看全部。' : '该主题暂时没有可展示的诊断记录。' }}</p></div>
      <div v-else class="diagnostics-groups">
      <label>显示时区 <select v-model="controlCenter.diagnosticDisplayTimezone"><option v-for="zone in displayTimezones" :key="zone" :value="zone">{{ zone }}</option></select></label>
      <p v-if="controlCenter.diagnosticModelRuns.length > 20 || controlCenter.diagnosticAgentRuns.length > 20" class="field-note">已加载历史分页；点击刷新查看最新记录。</p>
      <Accordion :key="currentSection" type="single" :default-value="currentSection" class="diagnostics-accordion">
        <AccordionItem v-if="currentSection === 'lifecycle'" value="lifecycle" class="diagnostic-group diagnostics-drawer">
          <AccordionTrigger class="diagnostics-drawer-summary section-heading"><div><p class="eyebrow">LIFECYCLE</p><h2>生命周期时间线</h2></div><Badge class="count-pill" variant="secondary">{{ controlCenter.lifecycleDiagnostics.length }}</Badge></AccordionTrigger>
          <AccordionContent><div class="diagnostic-drawer-body lifecycle-timeline">
            <article v-for="event in controlCenter.lifecycleDiagnostics" :key="event.id" class="diagnostic-row lifecycle-row" :class="statusClass(event.status)">
              <div class="diagnostic-meta"><strong>{{ event.surface }} · {{ event.transition }}</strong><Badge class="status-pill" :class="statusClass(event.status)" variant="secondary">{{ statusLabel(event.status) }}</Badge><small><time :datetime="event.occurredAt || event.createdAt">{{ formatRunTime(event.occurredAt || event.createdAt) }}</time> · {{ event.correlationId }}</small></div>
              <p><strong>阶段：</strong>{{ event.stage || "unknown" }} <span v-if="event.reasonCode">· <strong>原因：</strong>{{ event.reasonCode }}</span></p>
              <p v-if="event.attempt || event.nextDueAt" class="diagnostic-meta-note"><template v-if="event.attempt">尝试 {{ event.attempt }}<template v-if="event.maxAttempts"> / {{ event.maxAttempts }}</template></template><template v-if="event.nextDueAt"> · 下次 {{ formatRunTime(event.nextDueAt) }}</template></p>
              <p v-if="event.safeCause || event.errorCode" class="diagnostic-error">{{ event.errorCode || event.reasonCode }}<template v-if="event.safeCause"> · {{ event.safeCause }}</template></p>
              <div class="diagnostic-actions"><Button v-if="event.workflowId" variant="outline" type="button" @click="openWorkflow(event.workflowId)">查看 Workflow</Button><Button v-if="event.modelRunId" variant="outline" type="button" @click="openModelRun(event.correlationId)">查看 Model Run</Button></div>
            </article>
            <section v-if="controlCenter.workflowIntentSnapshots.length" class="intent-snapshot-list" aria-labelledby="intent-snapshot-title"><h3 id="intent-snapshot-title">PostgreSQL Intent 快照</h3><article v-for="intent in controlCenter.workflowIntentSnapshots" :key="intent.intentId" class="diagnostic-row"><div class="diagnostic-meta"><strong>{{ intent.intentType }}</strong><Badge class="status-pill" :class="statusClass(intent.status)" variant="secondary">{{ statusLabel(intent.status) }}</Badge><small>{{ intent.intentId }} · 尝试 {{ intent.attemptCount }}</small></div><p v-if="intent.nextAttemptAt">下次调度：{{ formatRunTime(intent.nextAttemptAt) }}</p><p v-if="intent.lastError" class="diagnostic-error">{{ intent.lastError }}</p><Button variant="outline" type="button" @click="openWorkflow(intent.runtimeWorkflowId || intent.workflowId)">查看 Workflow</Button></article></section>
          </div></AccordionContent>
        </AccordionItem>
        <AccordionItem v-if="currentSection === 'agent-runs'" value="agent-runs" class="diagnostic-group diagnostics-drawer">
          <AccordionTrigger class="diagnostics-drawer-summary section-heading"><h2>Agent 运行</h2><Badge variant="secondary">{{ controlCenter.diagnosticAgentRuns.length }}</Badge></AccordionTrigger>
          <AccordionContent><div class="diagnostic-drawer-body">
            <article v-for="agent in controlCenter.diagnosticAgentRuns" :key="`${agent.fluctlightId}:${agent.agentId}:${agent.runId}`" class="diagnostic-row">
              <div class="diagnostic-meta"><strong>{{ agent.agentId }}</strong><Badge variant="secondary">{{ statusLabel(agent.status) }}</Badge><time :datetime="agent.startedAt">{{ formatRunTime(agent.startedAt) }}</time></div>
              <p v-if="agent.status === 'failed'" class="diagnostic-error">{{ agentFailureStageLabels[agent.failureStage || 'unknown'] || agent.failureStage }} · {{ agent.failureCode }}<template v-if="agent.safeCause"> · {{ agent.safeCause }}</template></p>
              <Button variant="outline" @click="openModelRun(agent.correlationId)">查看运行步骤</Button>
            </article>
            <Button v-if="controlCenter.diagnosticAgentCursor" variant="outline" :disabled="controlCenter.diagnosticPagesLoading" @click="controlCenter.loadMoreDiagnosticRuns('agent')">加载更多 Agent 记录</Button>
          </div></AccordionContent>
        </AccordionItem>
        <AccordionItem v-if="currentSection === 'model-runs' && modelRunGroups.length" value="model-runs" class="diagnostic-group diagnostics-drawer">
          <AccordionTrigger class="diagnostics-drawer-summary section-heading"><div><p class="eyebrow">AGENT & MODEL RUNS</p><h2>运行诊断<small v-if="queueSummary" class="queue-summary"> · 队列 {{ queueSummary }}</small></h2></div><Badge class="count-pill" variant="secondary">{{ modelRunGroups.length }}</Badge></AccordionTrigger>
          <AccordionContent><div class="diagnostic-drawer-body">
            <section v-for="group in modelRunGroups" :key="group.id" class="model-run-group" :aria-label="`逻辑运行 ${group.id}`">
              <h3>{{ scenarioLabel(group.runs[0]?.scenario || group.runs[0]?.role || group.agents[0]?.agentId || "agent") }} · {{ group.correlationId || group.id }}</h3>
              <article v-for="agent in group.agents" :key="agent.runId" class="diagnostic-row" :class="statusClass(agent.status)">
                <div class="diagnostic-meta"><strong>{{ agent.source === 'termination_event' ? 'Agent 终止记录' : '逻辑 Agent' }} · {{ agent.agentId }}</strong><Badge class="status-pill" :class="statusClass(agent.status)" variant="secondary">{{ statusLabel(agent.status) }}</Badge><small>Run {{ agent.runId }} · {{ agent.source === 'termination_event' ? '记录' : '开始' }} {{ formatRunTime(agent.startedAt) }}<template v-if="agent.finishedAt && agent.source !== 'termination_event'"> · 结束 {{ formatRunTime(agent.finishedAt) }}</template></small></div>
                <p v-if="agent.associationStatus === 'unknown'" class="field-note">与模型轮次的关联未知（旧记录）。</p>
                <p v-if="agent.status === 'failed'" class="diagnostic-error"><strong>失败阶段：</strong>{{ agentFailureStageLabels[agent.failureStage || 'unknown'] || agent.failureStage }} · <strong>错误：</strong>{{ agent.failureCode || 'agent_run_failed' }}<template v-if="agent.safeCause"> · {{ agent.safeCause }}</template></p>
              </article>
              <p v-if="group.total > group.runs.length" class="field-note">当前筛选只显示 {{ group.runs.length }}/{{ group.total }} 次模型请求。</p>
              <p v-if="!group.runs.length" class="field-note">此运行没有可关联的物理模型记录。</p>
              <article v-for="run in group.runs" :key="run.id" class="diagnostic-row">
                <div class="diagnostic-meta">
                  <strong>{{ modelRoundLabel(run, group.total) }} · {{ modelStageLabel(run.stage) }}</strong>
                  <Badge class="status-pill" :class="statusClass(run.status)" variant="secondary">{{ statusLabel(run.status) }}</Badge>
                  <Badge v-if="isMetadataOnlyPrompt(run.prompt)" variant="outline" class="meta-only-pill">历史元数据</Badge>
                  <small>绑定：{{ bindingLabel(run.bindingRole || run.role) }} · {{ run.modelId }}<template v-if="run.priority"> · 优先级 {{ run.priority }}</template><template v-if="run.queuePosition"> · 队列第 {{ run.queuePosition }}</template> · <time class="diagnostic-time" :datetime="run.createdAt">{{ formatRunTime(run.createdAt) }}</time><template v-if="run.queuedAt && run.queuedAt !== run.createdAt"> · 排队 {{ formatRunTime(run.queuedAt) }}</template><template v-if="run.startedAt"> · 开始 {{ formatRunTime(run.startedAt) }}</template><template v-if="run.completedAt"> · 结束 {{ formatRunTime(run.completedAt) }}</template></small>
                </div>
                <p v-if="run.errorCode" class="diagnostic-error"><strong>失败原因：</strong>{{ run.errorCode }}</p>
                <details><summary>查看本次 Prompt <small v-if="isMetadataOnlyPrompt(run.prompt)" class="prompt-meta-note">（历史元数据记录）</small></summary><p v-if="isMetadataOnlyPrompt(run.prompt)" class="metadata-only-hint">此历史记录只保存了元数据，原始提示词无法恢复。</p><pre>{{ diagnosticText(run.prompt) }}</pre><details><summary>查看消息结构</summary><pre>{{ pretty(run.prompt) }}</pre></details></details>
                <details v-if="run.response != null"><summary>查看本次 Response</summary><pre>{{ diagnosticText(run.response) }}</pre><details><summary>查看消息结构</summary><pre>{{ pretty(run.response) }}</pre></details></details>
                <p v-else class="field-note">本次请求尚无 Response。</p>
                <ul v-if="run.toolSummaries?.length" class="detail-list" aria-label="Tool 往返摘要"><li v-for="tool in run.toolSummaries" :key="tool.callId"><strong>{{ tool.capability }}</strong> · {{ tool.status }}<template v-if="tool.errorCode"> · {{ tool.errorCode }}</template></li></ul>
              </article>
            </section>
            <Button v-if="controlCenter.diagnosticModelCursor" variant="outline" :disabled="controlCenter.diagnosticPagesLoading" @click="controlCenter.loadMoreDiagnosticRuns('model')">加载更多模型记录</Button>
          </div></AccordionContent>
        </AccordionItem>
        <AccordionItem v-if="currentSection === 'media-prompts' && controlCenter.diagnosticMediaPrompts.length" value="media-prompts" class="diagnostic-group diagnostics-drawer">
          <AccordionTrigger class="diagnostics-drawer-summary section-heading"><div><p class="eyebrow">MEDIA PROMPTS</p><h2>媒体提示词</h2></div><Badge class="count-pill" variant="secondary">{{ controlCenter.diagnosticMediaPrompts.length }}</Badge></AccordionTrigger>
          <AccordionContent><div class="diagnostic-drawer-body"><article v-for="item in controlCenter.diagnosticMediaPrompts" :key="item.id" class="diagnostic-row"><div class="diagnostic-meta"><strong>{{ item.kind || "媒体生成" }}</strong><Badge class="status-pill" :class="statusClass(item.status)" variant="secondary">{{ statusLabel(item.status) }}</Badge><small><time class="diagnostic-time" :datetime="item.createdAt">{{ formatRunTime(item.createdAt) }}</time> · {{ item.correlationId }}</small></div><p v-if="item.providerPrompt" class="diagnostic-generated-prompt"><strong>媒体模型生成的 Prompt：</strong>{{ item.providerPrompt }}</p><p v-if="item.submittedPrompt && item.submittedPrompt !== item.providerPrompt" class="diagnostic-generated-prompt"><strong>ComfyUI 文本 Prompt：</strong>{{ item.submittedPrompt }}</p><p v-if="item.qualityVerdict" class="diagnostic-meta-note"><strong>质量检查：</strong>{{ item.qualityVerdict }}</p><p v-if="item.errorMessage" class="diagnostic-error"><strong>失败原因：</strong>{{ item.errorMessage }}<template v-if="item.failureStage">（阶段：{{ mediaFailureStageLabels[item.failureStage] || item.failureStage }}）</template><template v-if="item.attemptCount"> · 第 {{ item.attemptCount }} 次尝试</template></p><p v-if="item.modelRun?.status === 'failed' && !item.errorMessage" class="diagnostic-error"><strong>提示词模型失败：</strong>{{ item.modelRun.errorCode || "未知错误" }}</p><div v-if="item.status === 'failed'" class="diagnostic-actions"><Button class="secondary-button" variant="outline" type="button" :disabled="controlCenter.saving" @click="controlCenter.retryMediaPrompt(item.mediaIntentId)">{{ controlCenter.saving ? "重试中..." : "重试" }}</Button></div><details v-if="item.requestPayload"><summary>实际发送给 ComfyUI 的 JSON</summary><pre>{{ pretty(item.requestPayload) }}</pre></details><details v-else><summary>查看媒体请求</summary><pre>{{ pretty(item.prompt) }}</pre></details><details v-if="item.modelRun"><summary>查看提示词模型运行</summary><pre>{{ pretty(item.modelRun) }}</pre></details></article></div></AccordionContent>
        </AccordionItem>
        <AccordionItem v-if="currentSection === 'events' && controlCenter.diagnostics.length" value="events" class="diagnostic-group diagnostics-drawer">
          <AccordionTrigger class="diagnostics-drawer-summary section-heading"><div><p class="eyebrow">EVENTS</p><h2>系统事件</h2></div><Badge class="count-pill" variant="secondary">{{ controlCenter.diagnostics.length }}</Badge></AccordionTrigger>
          <AccordionContent><div class="diagnostic-drawer-body"><article v-for="event in controlCenter.diagnostics" :key="event.id" class="diagnostic-row"><div class="diagnostic-meta"><strong>{{ event.eventType }}</strong><Badge class="status-pill" variant="secondary">{{ event.severity }}</Badge><small>{{ event.correlationId }}</small></div><pre>{{ pretty(event.payload) }}</pre></article></div></AccordionContent>
        </AccordionItem>
      </Accordion>
      </div>

      <details v-if="currentSection === 'workflows'" class="advanced-section" open><summary class="section-heading"><div><p class="eyebrow">ADVANCED</p><h2>工作流控制</h2></div><span class="disclosure-icon" aria-hidden="true">⌄</span></summary><p class="field-note">仅在需要排查运行时问题时使用暂停、取消、重启或 Reset。</p><div class="action-grid"><Button class="secondary-button" variant="outline" type="button" :disabled="controlCenter.workflowListLoading" @click="controlCenter.loadWorkflows()">刷新工作流列表</Button></div><p v-if="controlCenter.workflowListLoading" class="field-note" role="status">正在加载工作流列表...</p><p v-if="controlCenter.diagnosticsWarning" class="error-banner" role="alert">{{ controlCenter.diagnosticsWarning }}</p><form class="stack-form" @submit.prevent="controlCenter.queryWorkflowStatus"><label for="workflow-id">工作流 ID<Input id="workflow-id" v-model="controlCenter.workflowId" placeholder="输入工作流 ID" /></label><label for="workflow-history-point">Reset history point<Input id="workflow-history-point" v-model="controlCenter.workflowHistoryPoint" type="number" min="1" step="1" /></label><div class="action-grid"><Button class="secondary-button" variant="outline" type="submit" :disabled="!controlCenter.workflowId.trim()">查询状态</Button><Button class="secondary-button" variant="outline" type="button" :disabled="!controlCenter.workflowId.trim()" @click="controlCenter.queryWorkflowHistory">查看历史</Button><Button class="secondary-button" variant="outline" type="button" :disabled="controlCenter.saving || !controlCenter.workflowId.trim()" @click="controlCenter.commandWorkflow('pause')">暂停</Button><Button class="secondary-button" variant="outline" type="button" :disabled="controlCenter.saving || !controlCenter.workflowId.trim()" @click="controlCenter.commandWorkflow('resume')">恢复</Button><Button class="secondary-button" variant="outline" type="button" :disabled="controlCenter.saving || !controlCenter.workflowId.trim()" @click="controlCenter.commandWorkflow('cancel')">取消</Button><Button class="secondary-button" variant="outline" type="button" :disabled="controlCenter.saving || !controlCenter.workflowId.trim()" @click="controlCenter.restartWorkflow">重启</Button><Button class="danger-outline-button" variant="outline" type="button" :disabled="controlCenter.saving || !controlCenter.workflowId.trim() || !controlCenter.workflowHistoryPoint" @click="controlCenter.resetWorkflow">Reset</Button></div></form><pre v-if="controlCenter.workflowStatus">{{ pretty(controlCenter.workflowStatus) }}</pre><pre v-if="controlCenter.workflowHistory">{{ pretty(controlCenter.workflowHistory) }}</pre><ul v-if="controlCenter.workflows.length" class="workflow-list"><li v-for="workflow in controlCenter.workflows" :key="workflowIdFor(workflow)"><Button class="text-button" variant="ghost" type="button" @click="controlCenter.workflowId = workflowIdFor(workflow); controlCenter.queryWorkflowStatus()">{{ workflowIdFor(workflow) || "未知工作流" }}</Button></li></ul></details>
    </template>
  </section>
</template>

<style scoped>
.model-run-group { display: grid; gap: 10px; padding-block: 12px; border-bottom: 1px solid var(--surface-border); }
.model-run-group h3 { margin: 0; overflow-wrap: anywhere; font-size: 0.95rem; }
.model-run-group .detail-list { margin: 8px 0 0; }
.diagnostics-overview {
  display: grid;
  max-width: 680px;
  align-content: center;
  min-height: 100%;
  gap: 8px;
  padding: 20px 0;
}

.diagnostics-overview h2,
.diagnostics-detail-header h2 {
  margin: 4px 0 0;
  color: var(--ink);
  font-size: clamp(1.35rem, 2vw, 1.8rem);
  letter-spacing: -.03em;
}

.diagnostics-mobile-section-list {
  display: grid;
  gap: 8px;
  margin-top: 20px;
}

.diagnostics-mobile-section-link {
  display: flex;
  min-height: 58px;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  padding: 10px 14px;
  text-align: left;
}

.diagnostics-mobile-section-link span:first-child {
  display: grid;
  min-width: 0;
  gap: 3px;
}

.diagnostics-mobile-section-link strong,
.diagnostics-mobile-section-link small {
  white-space: normal;
  overflow-wrap: anywhere;
  word-break: break-word;
}

.diagnostics-mobile-section-link small {
  color: var(--muted-ink);
  font-size: .78rem;
  line-height: 1.35;
}

.meta-only-pill {
  margin-left: 6px;
  font-size: .68rem;
  padding: 1px 6px;
}

.prompt-meta-note {
  color: var(--muted-ink);
  font-size: .75rem;
}

.metadata-only-hint {
  margin: 6px 0 8px;
  padding: 6px 10px;
  border-radius: 6px;
  background: var(--surface-subtle, rgba(0, 0, 0, 0.04));
  color: var(--muted-ink);
  font-size: .75rem;
  line-height: 1.4;
}

.diagnostics-detail-header {
  display: grid;
  gap: 10px;
  margin-bottom: 18px;
}

.diagnostics-detail-header + .diagnostics-groups .diagnostics-drawer-summary {
  display: none;
}

.diagnostics-detail-header .back-link {
  justify-self: start;
}

.queue-summary {
  margin-left: 8px;
  color: var(--muted-ink);
  font-size: .72rem;
  font-weight: 600;
  letter-spacing: 0;
}

.lifecycle-filter-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 10px;
  margin-bottom: 18px;
  padding: 14px;
  border: 1px solid var(--surface-border);
  border-radius: 14px;
  background: var(--surface);
}

.lifecycle-filter-grid label {
  display: grid;
  min-width: 0;
  gap: 5px;
  color: var(--muted-ink);
  font-size: .78rem;
}

.lifecycle-filter-actions,
.diagnostic-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: end;
}

.lifecycle-filter-actions {
  grid-column: 1 / -1;
}

.lifecycle-timeline,
.intent-snapshot-list {
  display: grid;
  gap: 10px;
}

.intent-snapshot-list {
  margin-top: 18px;
}

.lifecycle-row {
  min-width: 0;
  border-inline-start: 4px solid var(--surface-border);
}

.lifecycle-row.run-failed,
.lifecycle-row.run-overdue,
.lifecycle-row.run-dead_letter {
  border-inline-start-color: var(--danger, #c84c4c);
}

.lifecycle-row.run-retry,
.lifecycle-row.run-blocked {
  border-inline-start-color: var(--warning, #b27717);
}

.lifecycle-row.run-completed,
.lifecycle-row.run-no_op {
  border-inline-start-color: var(--success, #39745a);
}

.lifecycle-row small,
.intent-snapshot-list small,
.lifecycle-row p,
.intent-snapshot-list p {
  overflow-wrap: anywhere;
}

@media (min-width: 761px) {
  .diagnostics-overview {
    min-height: 70%;
  }

  .diagnostics-overview .diagnostics-mobile-section-list,
  .diagnostics-detail-header .back-link {
    display: none;
  }
}

@media (max-width: 760px) {
  .diagnostics-overview {
    min-height: auto;
    padding: 18px 14px 32px;
  }

  .diagnostics-detail-header {
    padding: 12px 14px 0;
  }

  .diagnostics-detail-header h2 {
    font-size: 1.2rem;
  }

  .lifecycle-filter-grid {
    grid-template-columns: 1fr;
    margin-inline: 14px;
  }

  .lifecycle-filter-actions > * {
    flex: 1 1 140px;
  }
}
</style>

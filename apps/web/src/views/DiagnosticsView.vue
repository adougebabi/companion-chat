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
import { useControlCenterStore } from "../stores/control-center";

const props = defineProps<{ section?: DiagnosticsSection | null }>();
const emit = defineEmits<{ navigateSection: [section: DiagnosticsSection | null] }>();
const controlCenter = useControlCenterStore();
const currentSection = computed(() => props.section ?? null);
const scenarioLabels: Record<string, string> = { reply: "回复生成", autonomy_reply: "自治回复", cognitive_assessment: "认知判断", native_cognition: "原生事件认知", daily_review: "日评", schedule_generation: "计划生成", reflection: "反思", wake_up: "唤醒", initialization: "初始化", media_prompt: "媒体提示词", embedding: "Embedding" };
const bindingLabels: Record<string, string> = { generic_llm: "通用 LLM", embedding: "Embedding" };
const statusLabels: Record<string, string> = { queued: "排队中", running: "执行中", started: "已启动", scheduled: "已计划", retry: "待重试", completed: "已完成", no_op: "无操作", blocked: "已阻止", paused: "已暂停", inactive: "未激活", disabled: "已禁用", overdue: "已逾期", dead_letter: "已终止", failed: "失败", cancelled: "已取消", timeout: "超时" };
const mediaFailureStageLabels: Record<string, string> = { prepare: "准备提示词", submit: "提交 ComfyUI", poll_quality: "轮询或质量检查" };
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
function pretty(value: unknown) { return JSON.stringify(value, null, 2); }
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
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "时间未知";
  return date.toLocaleString("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false });
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
    if (document.visibilityState === "visible") void controlCenter.loadDiagnostics();
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
      <div v-else-if="(currentSection === 'lifecycle' && !controlCenter.lifecycleDiagnostics.length && !controlCenter.workflowIntentSnapshots.length) || (currentSection === 'model-runs' && !controlCenter.diagnosticModelRuns.length) || (currentSection === 'media-prompts' && !controlCenter.diagnosticMediaPrompts.length) || (currentSection === 'events' && !controlCenter.diagnostics.length)" class="empty-panel compact"><h2>暂无当前诊断记录</h2><p>{{ currentSection === 'lifecycle' ? '没有匹配的触发或工作流状态；可清除过滤查看全部。' : '该主题暂时没有可展示的脱敏记录。' }}</p></div>
      <div v-else class="diagnostics-groups">
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
        <AccordionItem v-if="currentSection === 'model-runs' && controlCenter.diagnosticModelRuns.length" value="model-runs" class="diagnostic-group diagnostics-drawer">
          <AccordionTrigger class="diagnostics-drawer-summary section-heading"><div><p class="eyebrow">MODEL RUNS</p><h2>模型运行<small v-if="queueSummary" class="queue-summary"> · 队列 {{ queueSummary }}</small></h2></div><Badge class="count-pill" variant="secondary">{{ controlCenter.diagnosticModelRuns.length }}</Badge></AccordionTrigger>
          <AccordionContent><div class="diagnostic-drawer-body"><article v-for="run in controlCenter.diagnosticModelRuns" :key="run.id" class="diagnostic-row"><div class="diagnostic-meta"><strong>{{ scenarioLabel(run.scenario || run.role) }}</strong><Badge class="status-pill" :class="statusClass(run.status)" variant="secondary">{{ statusLabel(run.status) }}</Badge><Badge v-if="isMetadataOnlyPrompt(run.prompt)" variant="outline" class="meta-only-pill">安全脱敏</Badge><small>绑定：{{ bindingLabel(run.bindingRole || run.role) }} · {{ run.modelId }}<template v-if="run.priority"> · 优先级 {{ run.priority }}</template><template v-if="run.queuePosition"> · 队列第 {{ run.queuePosition }}</template> · <time class="diagnostic-time" :datetime="run.createdAt">{{ formatRunTime(run.createdAt) }}</time><template v-if="run.queuedAt && run.queuedAt !== run.createdAt"> · 排队 {{ formatRunTime(run.queuedAt) }}</template><template v-if="run.startedAt"> · 开始 {{ formatRunTime(run.startedAt) }}</template><template v-if="run.completedAt"> · 结束 {{ formatRunTime(run.completedAt) }}</template> · {{ run.correlationId }}</small></div><p v-if="run.errorCode" class="diagnostic-error"><strong>失败原因：</strong>{{ run.errorCode }}</p><details><summary>查看 Prompt <small v-if="isMetadataOnlyPrompt(run.prompt)" class="prompt-meta-note">（脱敏元数据）</small></summary><p v-if="isMetadataOnlyPrompt(run.prompt)" class="metadata-only-hint">注：该运行（如实例初始化或预算超限预检）原始提示词已安全脱敏，此处展示的是消息数、Token 估算及校验哈希。</p><pre>{{ pretty(run.prompt) }}</pre></details><details v-if="run.response"><summary>查看 Response</summary><pre>{{ pretty(run.response) }}</pre></details></article></div></AccordionContent>
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

<script setup lang="ts">
import { computed, onMounted, onUnmounted } from "vue";

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
const statusLabels: Record<string, string> = { queued: "排队中", running: "执行中", completed: "已完成", failed: "失败", cancelled: "已取消", timeout: "超时" };
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
let pollTimer: number | undefined;
onMounted(() => {
  const correlationId = new URLSearchParams(window.location.search).get("correlation_id") ?? "";
  controlCenter.diagnosticsCorrelationFilter = correlationId;
  void controlCenter.loadDiagnostics();
  pollTimer = window.setInterval(() => {
    if (document.visibilityState === "visible") void controlCenter.loadDiagnostics();
  }, 2000);
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
      <div v-if="controlCenter.loading" class="empty-panel compact">正在加载诊断信息...</div>
      <div v-else-if="(currentSection === 'model-runs' && !controlCenter.diagnosticModelRuns.length) || (currentSection === 'media-prompts' && !controlCenter.diagnosticMediaPrompts.length) || (currentSection === 'events' && !controlCenter.diagnostics.length)" class="empty-panel compact"><h2>暂无当前诊断记录</h2><p>该主题暂时没有可展示的脱敏记录。</p></div>
      <div v-else class="diagnostics-groups">
      <Accordion :key="currentSection" type="single" :default-value="currentSection" class="diagnostics-accordion">
        <AccordionItem v-if="currentSection === 'model-runs' && controlCenter.diagnosticModelRuns.length" value="model-runs" class="diagnostic-group diagnostics-drawer">
          <AccordionTrigger class="diagnostics-drawer-summary section-heading"><div><p class="eyebrow">MODEL RUNS</p><h2>模型运行<small v-if="queueSummary" class="queue-summary"> · 队列 {{ queueSummary }}</small></h2></div><Badge class="count-pill" variant="secondary">{{ controlCenter.diagnosticModelRuns.length }}</Badge></AccordionTrigger>
          <AccordionContent><div class="diagnostic-drawer-body"><article v-for="run in controlCenter.diagnosticModelRuns" :key="run.id" class="diagnostic-row"><div class="diagnostic-meta"><strong>{{ scenarioLabel(run.scenario || run.role) }}</strong><Badge class="status-pill" :class="statusClass(run.status)" variant="secondary">{{ statusLabel(run.status) }}</Badge><small>绑定：{{ bindingLabel(run.bindingRole || run.role) }} · {{ run.modelId }}<template v-if="run.priority"> · 优先级 {{ run.priority }}</template><template v-if="run.queuePosition"> · 队列第 {{ run.queuePosition }}</template> · <time class="diagnostic-time" :datetime="run.createdAt">{{ formatRunTime(run.createdAt) }}</time><template v-if="run.queuedAt && run.queuedAt !== run.createdAt"> · 排队 {{ formatRunTime(run.queuedAt) }}</template><template v-if="run.startedAt"> · 开始 {{ formatRunTime(run.startedAt) }}</template><template v-if="run.completedAt"> · 结束 {{ formatRunTime(run.completedAt) }}</template> · {{ run.correlationId }}</small></div><p v-if="run.errorCode" class="diagnostic-error"><strong>失败原因：</strong>{{ run.errorCode }}</p><details><summary>查看 Prompt</summary><pre>{{ pretty(run.prompt) }}</pre></details><details v-if="run.response"><summary>查看 Response</summary><pre>{{ pretty(run.response) }}</pre></details></article></div></AccordionContent>
        </AccordionItem>
        <AccordionItem v-if="currentSection === 'media-prompts' && controlCenter.diagnosticMediaPrompts.length" value="media-prompts" class="diagnostic-group diagnostics-drawer">
          <AccordionTrigger class="diagnostics-drawer-summary section-heading"><div><p class="eyebrow">MEDIA PROMPTS</p><h2>媒体提示词</h2></div><Badge class="count-pill" variant="secondary">{{ controlCenter.diagnosticMediaPrompts.length }}</Badge></AccordionTrigger>
          <AccordionContent><div class="diagnostic-drawer-body"><article v-for="item in controlCenter.diagnosticMediaPrompts" :key="item.id" class="diagnostic-row"><div class="diagnostic-meta"><strong>{{ item.kind || "媒体" }} · {{ item.status }}</strong><Badge class="status-pill" :class="statusClass(item.status)" variant="secondary">{{ statusLabel(item.status) }}</Badge><small><time class="diagnostic-time" :datetime="item.createdAt">{{ formatRunTime(item.createdAt) }}</time> · {{ item.correlationId }}</small></div><p v-if="item.providerPrompt" class="diagnostic-generated-prompt"><strong>提交给媒体服务的提示词：</strong>{{ item.providerPrompt }}</p><p v-if="item.submittedPrompt && item.submittedPrompt !== item.providerPrompt" class="diagnostic-generated-prompt"><strong>实际提交的提示词：</strong>{{ item.submittedPrompt }}</p><p v-if="item.modelRun?.status === 'failed'" class="diagnostic-error"><strong>提示词模型失败：</strong>{{ item.modelRun.errorCode || "未知错误" }}</p><details><summary>查看媒体请求</summary><pre>{{ pretty(item.prompt) }}</pre></details><details v-if="item.modelRun"><summary>查看提示词模型运行</summary><pre>{{ pretty(item.modelRun) }}</pre></details></article></div></AccordionContent>
        </AccordionItem>
        <AccordionItem v-if="currentSection === 'events' && controlCenter.diagnostics.length" value="events" class="diagnostic-group diagnostics-drawer">
          <AccordionTrigger class="diagnostics-drawer-summary section-heading"><div><p class="eyebrow">EVENTS</p><h2>系统事件</h2></div><Badge class="count-pill" variant="secondary">{{ controlCenter.diagnostics.length }}</Badge></AccordionTrigger>
          <AccordionContent><div class="diagnostic-drawer-body"><article v-for="event in controlCenter.diagnostics" :key="event.id" class="diagnostic-row"><div class="diagnostic-meta"><strong>{{ event.eventType }}</strong><Badge class="status-pill" variant="secondary">{{ event.severity }}</Badge><small>{{ event.correlationId }}</small></div><pre>{{ pretty(event.payload) }}</pre></article></div></AccordionContent>
        </AccordionItem>
      </Accordion>
      </div>

      <details v-if="currentSection === 'workflows'" class="advanced-section" open><summary class="section-heading"><div><p class="eyebrow">ADVANCED</p><h2>工作流控制</h2></div><span class="disclosure-icon" aria-hidden="true">⌄</span></summary><p class="field-note">仅在需要排查运行时问题时使用暂停、取消、重启或 Reset。</p><form class="stack-form" @submit.prevent="controlCenter.queryWorkflowStatus"><label for="workflow-id">工作流 ID<Input id="workflow-id" v-model="controlCenter.workflowId" placeholder="输入工作流 ID" /></label><label for="workflow-history-point">Reset history point<Input id="workflow-history-point" v-model="controlCenter.workflowHistoryPoint" type="number" min="1" step="1" /></label><div class="action-grid"><Button class="secondary-button" variant="outline" type="submit" :disabled="!controlCenter.workflowId.trim()">查询状态</Button><Button class="secondary-button" variant="outline" type="button" :disabled="!controlCenter.workflowId.trim()" @click="controlCenter.queryWorkflowHistory">查看历史</Button><Button class="secondary-button" variant="outline" type="button" :disabled="controlCenter.saving || !controlCenter.workflowId.trim()" @click="controlCenter.commandWorkflow('pause')">暂停</Button><Button class="secondary-button" variant="outline" type="button" :disabled="controlCenter.saving || !controlCenter.workflowId.trim()" @click="controlCenter.commandWorkflow('resume')">恢复</Button><Button class="secondary-button" variant="outline" type="button" :disabled="controlCenter.saving || !controlCenter.workflowId.trim()" @click="controlCenter.commandWorkflow('cancel')">取消</Button><Button class="secondary-button" variant="outline" type="button" :disabled="controlCenter.saving || !controlCenter.workflowId.trim()" @click="controlCenter.restartWorkflow">重启</Button><Button class="danger-outline-button" variant="outline" type="button" :disabled="controlCenter.saving || !controlCenter.workflowId.trim() || !controlCenter.workflowHistoryPoint" @click="controlCenter.resetWorkflow">Reset</Button></div></form><pre v-if="controlCenter.workflowStatus">{{ pretty(controlCenter.workflowStatus) }}</pre><pre v-if="controlCenter.workflowHistory">{{ pretty(controlCenter.workflowHistory) }}</pre><ul v-if="controlCenter.workflows.length" class="workflow-list"><li v-for="workflow in controlCenter.workflows" :key="workflowIdFor(workflow)"><Button class="text-button" variant="ghost" type="button" @click="controlCenter.workflowId = workflowIdFor(workflow); controlCenter.queryWorkflowStatus()">{{ workflowIdFor(workflow) || "未知工作流" }}</Button></li></ul></details>
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
  overflow-wrap: anywhere;
}

.diagnostics-mobile-section-link small {
  color: var(--muted-ink);
  font-size: .78rem;
  line-height: 1.35;
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
}
</style>

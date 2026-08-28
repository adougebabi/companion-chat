<script setup lang="ts">
import { onMounted } from "vue";

import Accordion from "@/components/ui/accordion/Accordion.vue";
import AccordionContent from "@/components/ui/accordion/AccordionContent.vue";
import AccordionItem from "@/components/ui/accordion/AccordionItem.vue";
import AccordionTrigger from "@/components/ui/accordion/AccordionTrigger.vue";
import Badge from "@/components/ui/badge/Badge.vue";
import Button from "@/components/ui/button/Button.vue";
import Input from "@/components/ui/input/Input.vue";
import { useControlCenterStore } from "../stores/control-center";

const controlCenter = useControlCenterStore();
const roleLabels: Record<string, string> = { initialization: "初始化", cognitive_assessment: "认知判断", action_realization: "回复生成", reflection: "反思", embedding: "Embedding", media_prompt: "媒体提示词" };
function roleLabel(role: string) { return roleLabels[role] ?? role; }
function pretty(value: Record<string, unknown>) { return JSON.stringify(value, null, 2); }
function workflowIdFor(value: Record<string, unknown>): string {
  if (typeof value.workflow_id === "string") return value.workflow_id;
  if (typeof value.workflowId === "string") return value.workflowId;
  for (const nested of Object.values(value)) if (nested && typeof nested === "object" && !Array.isArray(nested)) { const id: string = workflowIdFor(nested as Record<string, unknown>); if (id) return id; }
  return "";
}
onMounted(() => {
  const correlationId = new URLSearchParams(window.location.search).get("correlation_id") ?? "";
  controlCenter.diagnosticsCorrelationFilter = correlationId;
  void controlCenter.loadDiagnostics();
});
</script>

<template>
  <section class="page diagnostics-page" aria-labelledby="diagnostics-title">
    <header class="page-header">
      <div><h1 id="diagnostics-title">诊断中心</h1></div>
    </header>
    <p v-if="controlCenter.error" class="error-banner" role="alert">{{ controlCenter.error }}</p>
    <div v-if="controlCenter.loading" class="empty-panel compact">正在加载诊断信息...</div>
    <div v-else-if="!controlCenter.diagnostics.length && !controlCenter.diagnosticModelRuns.length" class="empty-panel compact"><h2>暂无诊断记录</h2><p>经脱敏的模型运行和系统事件会显示在这里。</p></div>
    <div v-else class="diagnostics-groups">
      <Accordion type="single" collapsible class="diagnostics-accordion">
        <AccordionItem v-if="controlCenter.diagnosticModelRuns.length" value="model-runs" class="diagnostic-group diagnostics-drawer">
          <AccordionTrigger class="diagnostics-drawer-summary section-heading"><div><p class="eyebrow">MODEL RUNS</p><h2>模型运行</h2></div><Badge class="count-pill" variant="secondary">{{ controlCenter.diagnosticModelRuns.length }}</Badge></AccordionTrigger>
          <AccordionContent><div class="diagnostic-drawer-body"><article v-for="run in controlCenter.diagnosticModelRuns" :key="run.id" class="diagnostic-row"><div class="diagnostic-meta"><strong>{{ roleLabel(run.role) }}</strong><Badge class="status-pill" variant="secondary">{{ run.status }}</Badge><small>{{ run.modelId }} · {{ run.correlationId }}</small></div><p v-if="run.errorCode" class="diagnostic-error"><strong>失败原因：</strong>{{ run.errorCode }}</p><details><summary>查看 Prompt</summary><pre>{{ pretty(run.prompt) }}</pre></details><details v-if="run.response"><summary>查看 Response</summary><pre>{{ pretty(run.response) }}</pre></details></article></div></AccordionContent>
        </AccordionItem>
        <AccordionItem v-if="controlCenter.diagnostics.length" value="events" class="diagnostic-group diagnostics-drawer">
          <AccordionTrigger class="diagnostics-drawer-summary section-heading"><div><p class="eyebrow">EVENTS</p><h2>系统事件</h2></div><Badge class="count-pill" variant="secondary">{{ controlCenter.diagnostics.length }}</Badge></AccordionTrigger>
          <AccordionContent><div class="diagnostic-drawer-body"><article v-for="event in controlCenter.diagnostics" :key="event.id" class="diagnostic-row"><div class="diagnostic-meta"><strong>{{ event.eventType }}</strong><Badge class="status-pill" variant="secondary">{{ event.severity }}</Badge><small>{{ event.correlationId }}</small></div><pre>{{ pretty(event.payload) }}</pre></article></div></AccordionContent>
        </AccordionItem>
      </Accordion>
    </div>

    <details class="advanced-section"><summary class="section-heading"><div><p class="eyebrow">ADVANCED</p><h2>工作流控制</h2></div><span class="disclosure-icon" aria-hidden="true">⌄</span></summary><p class="field-note">仅在需要排查运行时问题时使用暂停、取消、重启或 Reset。</p><form class="stack-form" @submit.prevent="controlCenter.queryWorkflowStatus"><label for="workflow-id">工作流 ID<Input id="workflow-id" v-model="controlCenter.workflowId" placeholder="输入工作流 ID" /></label><label for="workflow-history-point">Reset history point<Input id="workflow-history-point" v-model="controlCenter.workflowHistoryPoint" type="number" min="1" step="1" /></label><div class="action-grid"><Button class="secondary-button" variant="outline" type="submit" :disabled="!controlCenter.workflowId.trim()">查询状态</Button><Button class="secondary-button" variant="outline" type="button" :disabled="!controlCenter.workflowId.trim()" @click="controlCenter.queryWorkflowHistory">查看历史</Button><Button class="secondary-button" variant="outline" type="button" :disabled="controlCenter.saving || !controlCenter.workflowId.trim()" @click="controlCenter.commandWorkflow('pause')">暂停</Button><Button class="secondary-button" variant="outline" type="button" :disabled="controlCenter.saving || !controlCenter.workflowId.trim()" @click="controlCenter.commandWorkflow('resume')">恢复</Button><Button class="secondary-button" variant="outline" type="button" :disabled="controlCenter.saving || !controlCenter.workflowId.trim()" @click="controlCenter.commandWorkflow('cancel')">取消</Button><Button class="secondary-button" variant="outline" type="button" :disabled="controlCenter.saving || !controlCenter.workflowId.trim()" @click="controlCenter.restartWorkflow">重启</Button><Button class="danger-outline-button" variant="outline" type="button" :disabled="controlCenter.saving || !controlCenter.workflowId.trim() || !controlCenter.workflowHistoryPoint" @click="controlCenter.resetWorkflow">Reset</Button></div></form><pre v-if="controlCenter.workflowStatus">{{ pretty(controlCenter.workflowStatus) }}</pre><pre v-if="controlCenter.workflowHistory">{{ pretty(controlCenter.workflowHistory) }}</pre><ul v-if="controlCenter.workflows.length" class="workflow-list"><li v-for="workflow in controlCenter.workflows" :key="workflowIdFor(workflow)"><Button class="text-button" variant="ghost" type="button" @click="controlCenter.workflowId = workflowIdFor(workflow); controlCenter.queryWorkflowStatus()">{{ workflowIdFor(workflow) || "未知工作流" }}</Button></li></ul></details>
  </section>
</template>

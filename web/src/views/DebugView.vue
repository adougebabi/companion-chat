<script setup lang="ts">
import { computed } from 'vue';
import type { DebugContextState, DebugInspectorSnapshot, DurableJob, MediaJob, PersonaSummary, PromptRun } from '../components/types';

const props = withDefaults(defineProps<{
  persona?: PersonaSummary | null;
  personas?: PersonaSummary[];
  inspector?: DebugInspectorSnapshot | null;
  loading?: boolean;
  error?: string | null;
}>(), {persona: null, personas: () => [], inspector: null, loading: false, error: null});

const emit = defineEmits<{(event: 'refresh'): void; (event: 'select-persona', id: string): void}>();

const promptRuns = computed<PromptRun[]>(() => props.inspector?.promptRuns || []);
const jobs = computed<DurableJob[]>(() => props.inspector?.lifecycle?.jobs || []);
const mediaJobs = computed<MediaJob[]>(() => props.inspector?.mediaJobs || []);
const state = computed<DebugContextState>(() => props.inspector?.debugContext?.state || {});
const emergence = computed(() => props.inspector?.debugContext?.emergence || props.inspector?.lifecycle?.emergence || null);

function display(value: unknown, fallback = '未提供'): string {
  if (value === null || value === undefined || value === '') return fallback;
  if (typeof value === 'string') return value;
  if (typeof value === 'number' || typeof value === 'boolean') return String(value);
  try { return JSON.stringify(value, null, 2); } catch { return fallback; }
}

function json(value: unknown): string { return display(value); }

function dateTime(value: unknown, fallback = '未知时间'): string {
  if (typeof value !== 'string' || !value.trim()) return fallback;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? fallback : date.toLocaleString('zh-CN');
}

function runLabel(run: PromptRun): string {
  return `${run.operation || 'unknown'}${run.model ? ` · ${run.model}` : ''}`;
}

function runMeta(run: PromptRun): string {
  const parts = [run.status || 'unknown', dateTime(run.createdAt)];
  if (run.completedAt) parts.push(`完成 ${dateTime(run.completedAt)}`);
  if (run.jobId) parts.push(`job ${run.jobId}`);
  if (run.messageId) parts.push(`message ${run.messageId}`);
  return parts.join(' · ');
}

function detailsOpen(run: PromptRun): boolean {
  return run.status === 'failed' || run.status === 'running';
}

function jobStatusClass(status: unknown): string {
  return String(status || 'unknown').toLowerCase().replace(/[^a-z0-9_-]/g, '-');
}

function jobMeta(job: DurableJob): string {
  const attempts = `${job.attemptCount ?? 0}/${job.maxAttempts ?? 0}`;
  const times = [job.createdAt ? `创建 ${dateTime(job.createdAt)}` : '', job.completedAt ? `完成 ${dateTime(job.completedAt)}` : ''].filter(Boolean);
  return [`尝试 ${attempts}`, ...times].join(' · ');
}

function mediaMeta(job: MediaJob): string {
  return [job.provider || 'provider 未知', dateTime(job.createdAt)].join(' · ');
}
</script>

<template>
  <section class="debug-view">
    <header class="pane-header debug-header">
      <div class="header-copy"><h1>调试</h1><p>{{ persona?.name || '选择一个摇光实例后查看运行记录' }}</p></div>
      <label class="debug-persona-select"><span>摇光实例</span><select :value="persona?.id || ''" @change="emit('select-persona', ($event.target as HTMLSelectElement).value)"><option value="" disabled>选择实例</option><option v-for="item in personas" :key="item.id" :value="item.id">{{ item.name }}</option></select></label>
      <button class="refresh-button" type="button" aria-label="刷新调试数据" title="刷新调试数据" :disabled="loading || !persona" @click="emit('refresh')">↻</button>
    </header>

    <div v-if="loading" class="debug-empty" role="status">正在读取调试数据…</div>
    <div v-else-if="error" class="debug-empty debug-empty--error" role="alert">{{ error }} <button class="quiet" type="button" @click="emit('refresh')">重试</button></div>
    <div v-else-if="!persona" class="debug-empty">请先选择一个摇光实例。</div>

    <div v-else class="debug-scroll">
      <section class="debug-state-grid" aria-label="当前状态">
        <article class="debug-state-item"><small>当前状态</small><b>{{ display(state.situation) }}</b></article>
        <article class="debug-state-item"><small>场景</small><b>{{ display(state.scene) }}</b></article>
        <article class="debug-state-item"><small>服装</small><b>{{ display(state.outfit) }}</b></article>
        <article class="debug-state-item"><small>特殊状态</small><b>{{ display(state.special || state.mood) }}</b></article>
      </section>

      <section class="debug-section">
        <div class="debug-section-heading"><div><h2>Prompt Runs</h2><p>每条记录是一对完整的模型请求与响应，按实际调用时间倒序排列。</p></div></div>
        <div v-if="!promptRuns.length" class="debug-empty debug-empty--inline">暂无模型调用记录。</div>
        <div v-for="(run, index) in promptRuns" :key="run.id || `prompt-run-${index}`" class="debug-run">
          <details :open="detailsOpen(run)">
            <summary class="debug-run-summary"><span><b>{{ runLabel(run) }}</b><small>{{ runMeta(run) }}</small></span><i aria-hidden="true">⌄</i></summary>
            <div class="debug-run-body">
              <section><h3>请求入参</h3><pre>{{ json(run.request) }}</pre></section>
              <section><h3>响应出参</h3><pre>{{ json(run.response) }}</pre></section>
              <p v-if="run.error" class="debug-error">{{ run.error }}</p>
            </div>
          </details>
        </div>
      </section>

      <section class="debug-section">
        <div class="debug-section-heading"><div><h2>后台作业</h2><p>这里展示真实持久化作业；Flow 本身没有独立运行记录，不再用作业 JSON 冒充 Flow。</p></div></div>
        <div v-if="!jobs.length" class="debug-empty debug-empty--inline">暂无后台作业。</div>
        <div v-for="(job, index) in jobs" :key="job.id || `job-${index}`" class="debug-job-row">
          <div class="debug-job-main"><div><b>{{ job.jobType || 'unknown' }}</b><small>{{ job.id || '无 ID' }} · {{ jobMeta(job) }}</small></div><span :class="`debug-job-status debug-job-status--${jobStatusClass(job.status)}`">{{ job.status || 'unknown' }}</span></div>
          <small class="debug-job-links">{{ job.messageId ? `message ${job.messageId}` : '' }}{{ job.activityId ? `activity ${job.activityId}` : '' }}{{ job.runAfter ? ` · 排期 ${dateTime(job.runAfter)}` : '' }}</small>
          <details v-if="job.payloadSummary || job.resultSummary || job.error"><summary>查看作业详情</summary><div class="debug-job-details"><p v-if="job.error" class="debug-error">{{ job.error }}</p><div v-if="job.payloadSummary"><b>输入摘要</b><pre>{{ job.payloadSummary }}</pre></div><div v-if="job.resultSummary"><b>结果摘要</b><pre>{{ job.resultSummary }}</pre></div></div></details>
        </div>
      </section>

      <section class="debug-section">
        <div class="debug-section-heading"><div><h2>媒体作业</h2><p>媒体任务来自实际媒体队列，包含最终提示词、进度和失败信息。</p></div></div>
        <div v-if="!mediaJobs.length" class="debug-empty debug-empty--inline">暂无媒体作业。</div>
        <div v-for="(job, index) in mediaJobs" :key="job.id || `media-job-${index}`" class="debug-media-row">
          <div class="debug-job-main"><div><b>{{ job.kind || 'media' }} · {{ job.id || '无 ID' }}</b><small>{{ mediaMeta(job) }}</small></div><span :class="`debug-job-status debug-job-status--${jobStatusClass(job.status)}`">{{ job.status || 'unknown' }}</span></div>
          <p v-if="job.finalPrompt || job.prompt" class="debug-media-prompt">{{ job.finalPrompt || job.prompt }}</p>
          <p v-if="job.error" class="debug-error">{{ job.error }}</p>
        </div>
      </section>

      <section class="debug-section">
        <div class="debug-section-heading"><div><h2>人格涌现链路</h2><p>这里显示经过脱敏的 appraisal、记忆候选、自我模型和主体性状态，以及关联证据与失败原因。</p></div></div>
        <div v-if="!emergence" class="debug-empty debug-empty--inline">暂无涌现记录。</div>
        <pre v-else class="debug-emergence-summary">{{ json(emergence) }}</pre>
      </section>
    </div>
  </section>
</template>

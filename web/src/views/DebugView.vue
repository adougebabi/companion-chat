<script setup lang="ts">
import { computed } from 'vue';
import type { DebugInspectorSnapshot, PersonaSummary, PromptRun } from '../components/types';

const props = withDefaults(defineProps<{
  persona?: PersonaSummary | null;
  inspector?: DebugInspectorSnapshot | null;
  loading?: boolean;
  error?: string | null;
}>(), {persona: null, inspector: null, loading: false, error: null});
const emit = defineEmits<{(event: 'refresh'): void}>();

const jobs = computed(() => {
  const rows = props.inspector?.lifecycle?.jobs;
  return Array.isArray(rows) ? rows : [];
});
const promptRuns = computed(() => props.inspector?.promptRuns || []);

function json(value: unknown): string {
  if (value === null || value === undefined || value === '') return '未提供';
  if (typeof value === 'string') return value;
  try { return JSON.stringify(value, null, 2); } catch { return '无法展示'; }
}

function runLabel(run: PromptRun): string {
  return `${run.operation || 'unknown'} · ${run.status || 'unknown'}${run.model ? ` · ${run.model}` : ''}`;
}
</script>

<template>
  <section class="debug-view">
    <header class="pane-header debug-header">
      <div class="header-copy"><h1>调试</h1><p>{{ persona?.name || '选择一个摇光实例后查看完整运行链路' }}</p></div>
      <button class="refresh-button" type="button" aria-label="刷新调试数据" title="刷新调试数据" :disabled="loading || !persona" @click="emit('refresh')">↻</button>
    </header>
    <div v-if="loading" class="debug-empty" role="status">正在读取调试数据…</div>
    <div v-else-if="error" class="debug-empty debug-empty--error" role="alert">{{ error }} <button class="quiet" type="button" @click="emit('refresh')">重试</button></div>
    <div v-else-if="!persona" class="debug-empty">请先从左侧联系人列表选择一个摇光实例。</div>
    <div v-else class="debug-scroll">
      <section class="debug-summary-grid">
        <article class="debug-card"><small>当前状态</small><pre>{{ json(inspector?.debugContext?.layers) }}</pre></article>
        <article class="debug-card"><small>最近请求</small><pre>{{ json(inspector?.debugContext?.recentRequests) }}</pre></article>
        <article class="debug-card"><small>作业数量</small><strong>{{ jobs.length }}</strong><pre>{{ json(jobs.slice(0, 8)) }}</pre></article>
        <article class="debug-card"><small>媒体作业</small><strong>{{ inspector?.mediaJobs?.length || 0 }}</strong><pre>{{ json(inspector?.mediaJobs) }}</pre></article>
      </section>
      <section class="debug-section"><div class="debug-section-heading"><div><h2>Prompt Runs</h2><p>查看每次模型调用的请求、响应、状态和错误。</p></div><span>{{ promptRuns.length }}</span></div>
        <div v-if="!promptRuns.length" class="debug-empty debug-empty--inline">暂无 prompt run。模型调用失败前也可能没有完整响应。</div>
        <div v-for="(run, index) in promptRuns" :key="run.id || `prompt-run-${index}`" class="debug-run">
          <header><b>{{ runLabel(run) }}</b><small>{{ run.createdAt || '未知时间' }} <span v-if="run.completedAt">→ {{ run.completedAt }}</span></small></header>
          <details open><summary>请求入参</summary><pre>{{ json(run.request) }}</pre></details>
          <details><summary>响应出参</summary><pre>{{ json(run.response) }}</pre></details>
          <p v-if="run.error" class="debug-error">{{ run.error }}</p>
        </div>
      </section>
      <section class="debug-section"><div class="debug-section-heading"><div><h2>Flow / Jobs</h2><p>包括 proactive、timeline、media、retry 和 lease 状态。</p></div><span>{{ jobs.length }}</span></div><pre class="debug-large-pre">{{ json(jobs) }}</pre></section>
      <section class="debug-section"><div class="debug-section-heading"><div><h2>Context Layers</h2><p>模型实际收到的身份、生活状态、关系、能力和影响状态摘要。</p></div></div><pre class="debug-large-pre">{{ json(inspector?.debugContext) }}</pre></section>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import type { MediaJob } from '../types';

const props = defineProps<{ job: MediaJob }>();
const expanded = ref(false);
const progress = computed(() => props.job.progress || {});
const progressStage = computed(() => String(progress.value.stageLabel || progress.value.stage || props.job.status || '进度未知'));
const percent = computed(() => {
  const value = Number(progress.value.percent);
  return Number.isFinite(value) ? `${Math.round(value)}%` : '未提供';
});
const safeValue = (value: unknown, fallback: string) => {
  if (value === null || value === undefined || value === '') return fallback;
  if (typeof value === 'string') return value;
  try { return JSON.stringify(value, null, 2); } catch { return fallback; }
};
</script>

<template>
  <article class="media-job-card">
    <header class="media-job-card-header"><div><p>{{ job.kind || '媒体' }} · {{ job.provider || '未标识 provider' }}</p><small>{{ job.createdAt ? new Date(job.createdAt).toLocaleString('zh-CN') : '' }}</small></div><span class="media-job-status">{{ job.status || '未知' }}</span></header>
    <section class="media-job-prompt"><b>最终 provider 提示词</b><p>{{ job.finalPrompt || job.prompt || '最终提示词尚未持久化' }}</p></section>
    <dl class="media-job-meta"><div><dt>阶段</dt><dd>{{ progressStage }}</dd></div><div><dt>尝试</dt><dd>{{ progress.attempt || '未提供' }}</dd></div><div><dt>耗时</dt><dd>{{ progress.elapsedMs ? `${Math.round(Number(progress.elapsedMs) / 100) / 10}s` : '未提供' }}</dd></div><div><dt>进度</dt><dd>{{ percent }}</dd></div></dl>
    <section class="media-job-output"><b>最新输出</b><p>{{ progress.latestOutput || progress.output || '暂无本地输出' }}</p><small v-if="progress.latestStream">来源：{{ progress.latestStream }}</small></section>
    <p v-if="job.error" class="media-job-error"><b>失败说明</b>{{ job.error }}</p>
    <details class="media-job-diagnostics" :open="expanded" @toggle="expanded = ($event.target as HTMLDetailsElement).open"><summary>媒体概念与模板诊断</summary><div class="media-job-diagnostics-body"><div><b>触发</b><pre>{{ safeValue(job.trigger, '未提供') }}</pre></div><div><b>服务器事实信封</b><pre>{{ safeValue(job.envelope, '未提供') }}</pre></div><div><b>AI 摇光实例媒体概念</b><pre>{{ safeValue(job.personaConcept, '尚未生成') }}</pre></div><div><b>生图大师固定模板</b><pre>{{ safeValue(job.promptTemplate, '尚未生成') }}</pre></div><div><b>工作流摘要</b><pre>{{ safeValue(job.workflowSummary || job.workflow, '未提供') }}</pre></div></div></details>
  </article>
</template>


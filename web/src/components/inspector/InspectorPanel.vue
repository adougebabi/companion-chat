<script setup lang="ts">
import MediaJobCard from './MediaJobCard.vue';
import type { MediaJob, PersonaDetailData } from '../types';

withDefaults(defineProps<{
  persona?: PersonaDetailData | null;
  mediaJobs?: MediaJob[];
  lifecycle?: Record<string, unknown> | null;
  debugContext?: Record<string, unknown> | null;
  loading?: boolean;
  error?: string | null;
}>(), { persona: null, mediaJobs: () => [], lifecycle: null, debugContext: null, loading: false, error: null });
const emit = defineEmits<{ (event: 'close'): void; (event: 'refresh'): void; (event: 'retry'): void }>();

function summary(value: unknown): string {
  if (value === null || value === undefined || value === '') return '未提供';
  if (typeof value === 'string') return value;
  try { return JSON.stringify(value, null, 2); } catch { return '无法展示'; }
}
</script>

<template>
  <section class="inspector">
    <header><div><small>DEBUG INSPECTOR</small><h2 id="inspector-dialog-title">检查器</h2><p>{{ persona?.name || '摇光实例' }}</p></div><div class="inspector-header-actions"><button class="refresh-button" type="button" aria-label="刷新检查器" title="刷新检查器" :disabled="loading" @click="emit('refresh')">↻</button><button class="close-dialog" type="button" aria-label="关闭检查器" @click="emit('close')">×</button></div></header>
    <div class="inspector-scroll">
      <p v-if="error" class="wizard-error" role="alert">{{ error }} <button class="quiet" type="button" @click="emit('retry')">重试</button></p>
      <div v-if="loading" class="loading-state" role="status">正在刷新检查器…</div>
      <template v-else>
        <section><h3>当前状态</h3><dl class="debug-list"><li><b>场景</b><span>{{ persona?.state?.scene || persona?.state?.room || '未提供' }}</span></li><li><b>位置</b><span>{{ persona?.state?.location || '未提供' }}</span></li><li><b>状态来源</b><span>{{ summary(persona?.state?.source) }}</span></li></dl></section>
        <section><h3>生命周期摘要</h3><pre class="debug-list-pre">{{ summary(lifecycle) }}</pre></section>
        <section><h3>调试上下文</h3><pre class="debug-list-pre">{{ summary(debugContext) }}</pre></section>
        <section id="inspector-media-jobs"><h3>媒体任务</h3><div class="media-job-list"><MediaJobCard v-for="(job, index) in mediaJobs" :key="job.id || `media-job-${index}`" :job="job" /><p v-if="!mediaJobs.length" class="media-job-empty">暂无媒体作业。</p></div></section>
      </template>
    </div>
  </section>
</template>


<script setup lang="ts">
import { computed, ref } from 'vue';
import MediaJobCard from './MediaJobCard.vue';
import type { H3PreflightResult, InspectorActionResult, MediaJob, PersonaDetailData } from '../types';

const props = withDefaults(defineProps<{
  persona?: PersonaDetailData | null;
  mediaJobs?: MediaJob[];
  lifecycle?: Record<string, unknown> | null;
  debugContext?: Record<string, unknown> | null;
  loading?: boolean;
  error?: string | null;
  actionBusy?: boolean;
  h3Result?: H3PreflightResult | null;
  actionResult?: InspectorActionResult | null;
}>(), { persona: null, mediaJobs: () => [], lifecycle: null, debugContext: null, loading: false, error: null, actionBusy: false, h3Result: null, actionResult: null });

const emit = defineEmits<{
  (event: 'close'): void;
  (event: 'refresh'): void;
  (event: 'retry'): void;
  (event: 'h3-preflight'): void;
  (event: 'simulate', input: Record<string, unknown>): void;
  (event: 'debug-media', input: Record<string, unknown>): void;
}>();

const simulationKind = ref('routine');
const simulationSituation = ref('');
const simulationVisual = ref(false);
const mediaKind = ref<'image' | 'video'>('image');
const mediaRequest = ref('');

function summary(value: unknown): string {
  if (value === null || value === undefined || value === '') return '未提供';
  if (typeof value === 'string') return value;
  try { return JSON.stringify(value, null, 2); } catch { return '无法展示'; }
}

function submitSimulation(): void {
  emit('simulate', {kind: simulationKind.value, situation: simulationSituation.value.trim(), visual: simulationVisual.value, publish: true});
}

function submitMedia(): void {
  emit('debug-media', {kind: mediaKind.value, request: mediaRequest.value.trim()});
}

const preflightStatus = computed(() => {
  const result = props.h3Result;
  if (!result) return '';
  const checks = Object.entries(result.checks || {}).map(([name, check]) => {
    const label = ({executable: '可执行文件', modelDir: '模型目录', outputDir: '输出目录'} as Record<string, string>)[name] || name;
    return `${label}：${check.valid ? '通过' : check.error || '失败'}`;
  });
  const process = result.process?.error || (result.ok ? 'h3 --help 已成功启动。' : '检查失败。');
  return `${result.ok ? '检查通过。' : '检查未通过。'} ${checks.join('；')} ${process}`;
});
</script>

<template>
  <section class="inspector">
    <header>
      <div><small>LOCAL DEVELOPMENT INSPECTOR</small><h2 id="inspector-dialog-title">生命周期与调试</h2><p>{{ persona?.name || '摇光实例' }}</p></div>
      <div class="inspector-header-actions"><button class="refresh-button" type="button" aria-label="刷新检查器" title="刷新检查器" :disabled="loading || actionBusy" @click="emit('refresh')">↻</button><button class="close-dialog" type="button" aria-label="关闭检查器" @click="emit('close')">×</button></div>
    </header>
    <div class="inspector-scroll">
      <p class="inspector-warning">仅限本地开发：测试媒体请求会创建真实的耐久作业。</p>
      <p v-if="error" class="wizard-error" role="alert">{{ error }} <button class="quiet" type="button" @click="emit('retry')">重试</button></p>
      <div v-if="loading" class="loading-state" role="status">正在刷新检查器…</div>
      <template v-else>
        <section class="h3-preflight">
          <h3>h3 当前配置</h3>
          <p>此检查只验证当前服务的文件系统配置，并启动一次 <code>h3 --help</code>；不会创建媒体作业或资产。</p>
          <button class="quiet" type="button" :disabled="actionBusy" @click="emit('h3-preflight')">{{ actionBusy ? '检查中…' : '测试当前 h3 配置' }}</button>
          <p class="h3-preflight-result" role="status" aria-live="polite">{{ preflightStatus }}</p>
        </section>
        <section>
          <h3>当前状态</h3>
          <ul class="debug-list"><li><b>场景</b><span>{{ persona?.state?.scene || persona?.state?.room || '未提供' }}</span></li><li><b>位置</b><span>{{ persona?.state?.location || '未提供' }}</span></li><li><b>状态来源</b><pre>{{ summary(persona?.state?.source) }}</pre></li></ul>
        </section>
        <section>
          <h3>模拟允许事件</h3>
          <form class="simulate-form" @submit.prevent="submitSimulation">
            <label>事件类型<select v-model="simulationKind"><option value="routine">日常推进</option><option value="class">上学/课程</option><option value="shopping">逛街/购物</option><option value="social">社交活动</option><option value="mild_setback">轻度挫折</option></select></label>
            <label>状态说明<input v-model="simulationSituation" maxlength="240" placeholder="当前状态（可选）" /></label>
            <label class="visual-toggle"><input v-model="simulationVisual" type="checkbox" /> 为该动态生成活动图片</label>
            <button class="primary" type="submit" :disabled="actionBusy">模拟允许事件</button>
          </form>
        </section>
        <section>
          <h3>测试媒体作业</h3>
          <form class="test-media-form" @submit.prevent="submitMedia">
            <label>测试媒体<select v-model="mediaKind"><option value="image">图片</option><option value="video">视频</option></select></label>
            <label>测试画面说明<input v-model="mediaRequest" maxlength="500" placeholder="可选；会结合当前摇光实例状态" /></label>
            <button class="quiet" type="submit" :disabled="actionBusy">创建测试媒体作业</button>
          </form>
          <p v-if="actionResult" class="h3-preflight-result" role="status" aria-live="polite">动作已提交：{{ summary(actionResult) }}</p>
        </section>
        <section><h3>生命周期摘要</h3><pre class="debug-list-pre">{{ summary(lifecycle) }}</pre></section>
        <section><h3>调试上下文</h3><pre class="debug-list-pre">{{ summary(debugContext) }}</pre></section>
        <section class="inspector-media-region" id="inspector-media-jobs"><h3>媒体任务</h3><div class="media-job-list"><MediaJobCard v-for="(job, index) in mediaJobs" :key="job.id || `media-job-${index}`" :job="job" /><p v-if="!mediaJobs.length" class="media-job-empty">暂无媒体作业。</p></div></section>
      </template>
    </div>
  </section>
</template>

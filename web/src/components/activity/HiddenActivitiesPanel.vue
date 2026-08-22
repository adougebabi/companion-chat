<script setup lang="ts">
import type { ActivityItem } from '../types';

withDefaults(defineProps<{
  personaId?: string | null;
  items?: ActivityItem[];
  nextCursor?: string | null;
  loading?: boolean;
  loadingMore?: boolean;
  error?: string | null;
}>(), {personaId: null, items: () => [], nextCursor: null, loading: false, loadingMore: false, error: null});

const emit = defineEmits<{
  (event: 'close'): void;
  (event: 'load-more'): void;
  (event: 'retry'): void;
  (event: 'restore', id: string): void;
}>();

function timeText(value?: string | null): string {
  if (!value) return '';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleString('zh-CN');
}
</script>

<template>
  <section class="inspector">
    <header>
      <div><small>HIDDEN ACTIVITIES</small><h2 id="hidden-activities-title">已隐藏动态</h2><p>仅显示当前摇光实例的隐藏内容</p></div>
      <button class="close-dialog" type="button" aria-label="关闭已隐藏动态" @click="emit('close')">×</button>
    </header>
    <div class="hidden-list">
      <div v-if="loading" class="loading-state" role="status">正在加载已隐藏动态…</div>
      <div v-else-if="error" class="activity-feedback" role="alert"><span>{{ error }}</span><button class="quiet" type="button" @click="emit('retry')">重试</button></div>
      <p v-else-if="!items.length" class="muted">没有已隐藏动态。</p>
      <template v-else>
        <article v-for="item in items" :key="item.id">
          <p>{{ item.content || '（没有文字内容）' }}</p>
          <small>{{ timeText(item.createdAt) }}</small>
          <button class="quiet" type="button" @click="emit('restore', item.id)">恢复</button>
        </article>
      </template>
      <div v-if="nextCursor && !loading" class="load-more">
        <button class="quiet" type="button" :disabled="loadingMore" @click="emit('load-more')">{{ loadingMore ? '正在加载…' : '加载更多' }}</button>
      </div>
    </div>
  </section>
</template>

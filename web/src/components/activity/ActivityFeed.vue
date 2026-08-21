<script setup lang="ts">
import ActivityCard from './ActivityCard.vue';
import type { ActivityItem, PersonaSummary } from '../types';

const props = withDefaults(defineProps<{
  items?: ActivityItem[];
  personas?: PersonaSummary[];
  personaId?: string | null;
  nextCursor?: string | null;
  loading?: boolean;
  loadingMore?: boolean;
  error?: string | null;
  commentingId?: string | null;
  simplifiedMedia?: boolean;
}>(), { items: () => [], personas: () => [], personaId: null, nextCursor: null, loading: false, loadingMore: false, error: null, commentingId: null, simplifiedMedia: false });

const emit = defineEmits<{
  (event: 'refresh'): void;
  (event: 'load-more'): void;
  (event: 'retry'): void;
  (event: 'open-persona', id: string): void;
  (event: 'like', id: string): void;
  (event: 'hide', id: string): void;
  (event: 'comment', id: string): void;
  (event: 'cancel-comment'): void;
  (event: 'submit-comment', id: string, content: string): void;
  (event: 'chat', id: string): void;
}>();

function personaFor(item: ActivityItem): PersonaSummary | null {
  return item.persona || (item.personaId ? props.personas.find(persona => persona.id === item.personaId) || null : null);
}

function submitComment(id: string, content: string): void {
  emit('submit-comment', id, content);
}
</script>

<template>
  <div class="activity-stream">
    <div v-if="loading" class="activity-loading" role="status">正在加载动态…</div>
    <div v-else-if="error" class="activity-feedback" role="alert"><span>动态暂时无法加载。</span><button class="quiet" type="button" @click="emit('retry')">重试</button></div>
    <div v-else-if="!items.length" class="activity-empty">还没有动态。摇光实例的日常和事件会自然地出现在这里。</div>
    <ActivityCard
      v-for="item in items"
      v-else
      :key="item.id"
      :activity="item"
      :persona="personaFor(item)"
      :commenting="commentingId === item.id"
      :simplified-media="simplifiedMedia"
      @open-persona="emit('open-persona', $event)"
      @like="emit('like', $event)"
      @hide="emit('hide', $event)"
      @comment="emit('comment', $event)"
      @cancel-comment="emit('cancel-comment')"
      @submit-comment="submitComment"
      @chat="emit('chat', $event)"
    />
    <div v-if="nextCursor && !loading" class="load-more">
      <button class="quiet" type="button" :disabled="loadingMore" @click="emit('load-more')">{{ loadingMore ? '正在加载…' : '加载更多' }}</button>
    </div>
    <div v-else-if="items.length && !loading" class="feed-boundary">已经看到最早的动态</div>
  </div>
</template>

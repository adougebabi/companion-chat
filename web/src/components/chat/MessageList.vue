<script setup lang="ts">
import { nextTick, ref, watch } from 'vue';
import { useMessageHistory } from '../../composables/useMessageHistory';
import Avatar from '../Avatar.vue';
import MessageBubble from './MessageBubble.vue';
import type { Message, PersonaSummary } from '../types';

const props = withDefaults(defineProps<{
  messages?: Message[];
  persona: PersonaSummary;
  loading?: boolean;
  loadingOlder?: boolean;
  historyError?: string | null;
  hasMore?: boolean;
  simplifiedMedia?: boolean;
}>(), { messages: () => [], loading: false, loadingOlder: false, historyError: null, hasMore: false, simplifiedMedia: false });

const emit = defineEmits<{ (event: 'load-older'): void; (event: 'retry-history'): void; (event: 'prompt', value: string): void }>();
const stream = ref<HTMLElement | null>(null);
const history = useMessageHistory(() => props.persona.id, stream, {autoLoad: true, loadInitial: false});
const historySentinel = history.topSentinel;

function onScroll() {
  history.onScroll();
}

watch(() => [props.messages?.length, props.loadingOlder, props.persona.id], async () => {
  await nextTick();
  history.observeSentinel();
});

defineExpose({ stream, topSentinel: historySentinel, scrollToLatest: async () => { await nextTick(); if (stream.value) stream.value.scrollTop = stream.value.scrollHeight; } });
</script>

<template>
  <div ref="stream" class="message-stream" role="log" aria-live="polite" aria-relevant="additions text" @scroll.passive="onScroll">
    <div ref="historySentinel" class="history-sentinel" aria-hidden="true" />
    <div v-if="loading" class="message-loading" role="status">正在加载消息…</div>
    <div v-else-if="historyError" class="history-feedback" role="alert">
      <span>历史消息加载失败。</span><button class="quiet" type="button" @click="emit('retry-history')">重试</button>
    </div>
    <div v-else-if="loadingOlder" class="history-loading" role="status">正在加载更早的消息…</div>
    <div v-else-if="!hasMore && messages.length" class="history-boundary">已经是最早的消息</div>
    <template v-if="messages.length">
      <MessageBubble v-for="message in messages" :key="message.id" :message="message" :simplified-media="simplifiedMedia" />
    </template>
    <div v-else-if="!loading" class="chat-empty">
      <Avatar :persona="persona" size="large" /><h2>{{ persona.name }}</h2><p>{{ persona.currentSituation || '正在过自己的日常。' }}</p>
      <button class="soft-prompt" type="button" @click="emit('prompt', '今天过得怎么样？')">今天过得怎么样？</button>
    </div>
  </div>
</template>

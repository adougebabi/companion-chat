<script setup lang="ts">
import ChatHeader from './ChatHeader.vue';
import ChatComposer from './ChatComposer.vue';
import MessageList from './MessageList.vue';
import FeedbackBanner from '../feedback/FeedbackBanner.vue';
import type { Message, PersonaSummary } from '../types';

withDefaults(defineProps<{
  persona: PersonaSummary;
  messages?: Message[];
  draft?: string;
  loading?: boolean;
  loadingOlder?: boolean;
  historyError?: string | null;
  hasMore?: boolean;
  isSending?: boolean;
  isComposing?: boolean;
  debugInspector?: boolean;
  simplifiedMedia?: boolean;
  sendError?: string | null;
}>(), { messages: () => [], draft: '', loading: false, loadingOlder: false, historyError: null, hasMore: false, isSending: false, isComposing: false, debugInspector: false, simplifiedMedia: false, sendError: null });

const emit = defineEmits<{
  (event: 'back'): void;
  (event: 'profile'): void;
  (event: 'tools'): void;
  (event: 'load-older'): void;
  (event: 'retry-history'): void;
  (event: 'prompt', value: string): void;
  (event: 'update:draft', value: string): void;
  (event: 'submit'): void;
  (event: 'composition-start'): void;
  (event: 'composition-end', value: CompositionEvent): void;
  (event: 'selection-change', start: number, end: number): void;
  (event: 'dismiss-send-error'): void;
}>();

function forwardSelection(start: number, end: number) {
  emit('selection-change', start, end);
}
</script>

<template>
  <section class="chat-view">
    <ChatHeader :persona="persona" :debug-inspector="debugInspector" @back="emit('back')" @profile="emit('profile')" @tools="emit('tools')" />
    <MessageList :persona="persona" :messages="messages" :loading="loading" :loading-older="loadingOlder" :history-error="historyError" :has-more="hasMore" :simplified-media="simplifiedMedia" @load-older="emit('load-older')" @retry-history="emit('retry-history')" @prompt="emit('prompt', $event)" />
    <div class="chat-composer-area">
      <FeedbackBanner :message="sendError" tone="error" @dismiss="emit('dismiss-send-error')" />
      <ChatComposer :model-value="draft" :persona-name="persona.name" :disabled="isSending" :is-composing="isComposing" @update:model-value="emit('update:draft', $event)" @submit="emit('submit')" @composition-start="emit('composition-start')" @composition-end="emit('composition-end', $event)" @selection-change="forwardSelection" />
    </div>
  </section>
</template>

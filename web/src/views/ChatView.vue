<script setup lang="ts">
import ChatViewSurface from '../components/chat/ChatViewSurface.vue';
import type { Message, PersonaSummary } from '../components/types';

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
}>(), { messages: () => [], draft: '', loading: false, loadingOlder: false, historyError: null, hasMore: false, isSending: false, isComposing: false, debugInspector: false, simplifiedMedia: false });

const emit = defineEmits<{
  (event: 'back'): void; (event: 'profile'): void; (event: 'tools'): void; (event: 'load-older'): void; (event: 'retry-history'): void; (event: 'prompt', value: string): void; (event: 'update:draft', value: string): void; (event: 'submit'): void; (event: 'composition-start'): void; (event: 'composition-end'): void;
}>();
</script>

<template>
  <ChatViewSurface v-bind="$props" @back="emit('back')" @profile="emit('profile')" @tools="emit('tools')" @load-older="emit('load-older')" @retry-history="emit('retry-history')" @prompt="emit('prompt', $event)" @update:draft="emit('update:draft', $event)" @submit="emit('submit')" @composition-start="emit('composition-start')" @composition-end="emit('composition-end')" />
</template>

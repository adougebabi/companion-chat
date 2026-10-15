<script setup lang="ts">
import type { BrowserMessage } from "@fluctlight/browser-client";

import MessageComposer from "./MessageComposer.vue";
import MessageList from "./MessageList.vue";

defineProps<{
  title: string;
  subtitle: string;
  avatarLabel: string;
  messages: BrowserMessage[];
  loading: boolean;
  historyLoading: boolean;
  hasOlder: boolean;
  waiting: boolean;
  sending: boolean;
  canCancel: boolean;
  draft: string;
  disabled: boolean;
  streamingId?: string | null;
  selfLabel?: string;
  statusFor: (message: BrowserMessage) => "pending" | "failed" | "sent" | "none";
  mediaUrl?: (assetId: string) => string;
  emptyTitle?: string;
  emptyCopy?: string;
}>();
const emit = defineEmits<{
  "update:draft": [value: string];
  submit: [];
  cancel: [];
  keydown: [event: KeyboardEvent];
  loadOlder: [];
  focusComposer: [];
}>();
</script>
<template>
  <section class="chat-panel chatcol">
    <header class="chat-header chathead">
      <span class="chat-avatar av" aria-hidden="true">{{ avatarLabel }}</span>
      <span class="chat-header-copy grow">
        <strong id="chat-title">{{ title }}</strong>
        <small>{{ subtitle }}</small>
      </span>
      <slot name="actions" />
    </header>
    <slot name="summary" />
    <MessageList
      :messages="messages"
      :loading="loading"
      :history-loading="historyLoading"
      :has-older="hasOlder"
      :waiting="waiting"
      :name="title"
      :self-label="selfLabel ?? '我'"
      :peer-label="avatarLabel"
      :streaming-id="streamingId"
      :status-for="statusFor"
      :media-url="mediaUrl"
      @load-older="emit('loadOlder')"
    >
      <template #empty>
        <div class="big" aria-hidden="true">💬</div>
        <h2>{{ emptyTitle ?? "还没有消息" }}</h2>
        <p class="muted">{{ emptyCopy ?? "写下一句，就会进入真实会话。" }}</p>
        <button class="btn primary sm" type="button" @click="emit('focusComposer')">开始写下第一句话</button>
      </template>
    </MessageList>
    <slot name="notices" />
    <MessageComposer
      :draft="draft"
      :sending="sending"
      :can-cancel="canCancel"
      :disabled="disabled"
      @update:draft="emit('update:draft', $event)"
      @submit="emit('submit')"
      @cancel="emit('cancel')"
      @keydown="emit('keydown', $event)"
    />
  </section>
</template>

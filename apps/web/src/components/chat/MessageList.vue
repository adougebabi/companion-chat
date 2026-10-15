<script setup lang="ts">
import { computed, ref } from "vue";
import type { BrowserMessage } from "@fluctlight/browser-client";

import SkeletonBlock from "../feedback/SkeletonBlock.vue";
import LoadingDots from "../feedback/LoadingDots.vue";
import { useChatScroll } from "../../composables/use-chat-scroll";
import MessageBubble from "./MessageBubble.vue";

const props = defineProps<{
  messages: BrowserMessage[];
  loading: boolean;
  historyLoading: boolean;
  hasOlder: boolean;
  waiting: boolean;
  name: string;
  selfLabel: string;
  peerLabel: string;
  streamingId?: string | null;
  statusFor: (message: BrowserMessage) => "pending" | "failed" | "sent" | "none";
  mediaUrl?: (assetId: string) => string;
}>();
const emit = defineEmits<{ loadOlder: [] }>();

const scroller = ref<HTMLElement | null>(null);
const signature = computed(() => props.messages.map((message) => `${message.id}:${message.text?.length ?? 0}`).join("|"));
const { following, captureAnchor, scrollToLatest } = useChatScroll(scroller, signature);

function loadOlder() {
  captureAnchor();
  emit("loadOlder");
}

defineExpose({ scrollToLatest });
</script>
<template>
  <section ref="scroller" class="message-timeline stream" aria-live="polite" aria-label="对话记录">
    <div v-if="loading && !messages.length" class="chat-skeleton" role="status" aria-busy="true">
      <span class="sr-only">正在加载对话</span>
      <SkeletonBlock :lines="2" /><SkeletonBlock :lines="3" /><SkeletonBlock :lines="2" />
    </div>
    <div v-else-if="!messages.length" class="statebox chat-empty">
      <slot name="empty" />
    </div>
    <div v-else-if="hasOlder || historyLoading" class="load-more-container">
      <button class="btn sm load-older load-more-pill" type="button" :disabled="historyLoading" @click="loadOlder">{{ historyLoading ? "正在加载更早记录" : "加载更早记录" }}</button>
    </div>
    <MessageBubble
      v-for="message in messages"
      :key="message.id"
      :message="message"
      :mine="message.kind === 'user'"
      :name="name"
      :avatar-label="message.kind === 'user' ? selfLabel : peerLabel"
      :streaming="message.id === streamingId"
      :status="statusFor(message)"
      :media-url="mediaUrl"
    />
    <div v-if="waiting" class="message-row msg from-fluctlight" data-test="waiting-reply">
      <div class="avatar" aria-hidden="true">{{ peerLabel }}</div>
      <div class="message-bubble bubble"><LoadingDots label="正在等待回复" /></div>
    </div>
    <button v-if="!following" class="btn sm jump-latest" type="button" @click="scrollToLatest('smooth')">回到最新消息</button>
  </section>
</template>

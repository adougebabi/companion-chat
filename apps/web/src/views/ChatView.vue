<script setup lang="ts">
import { nextTick, ref } from "vue";
import type { BrowserMessage } from "@fluctlight/browser-client";

import { bffOrigin } from "../runtime-config";
import { useConversationStore } from "../stores/conversations";

const emit = defineEmits<{
  back: [];
  openDetails: [];
  openInstances: [];
}>();

const store = useConversationStore();
const draft = ref("");
const composer = ref<HTMLTextAreaElement | null>(null);
const transcript = ref<HTMLElement | null>(null);

async function send() {
  const text = draft.value.trim();
  if (!text) return;
  draft.value = "";
  await store.send(text);
  await nextTick();
  transcript.value?.scrollTo({ top: transcript.value.scrollHeight, behavior: "smooth" });
  composer.value?.focus();
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === "Enter" && !event.shiftKey) {
    event.preventDefault();
    void send();
  }
}

function mediaUrl(assetId: string) {
  return new URL(`/api/media/${encodeURIComponent(assetId)}`, bffOrigin).toString();
}

function deliveryStatus(message: BrowserMessage): "pending" | "sent" | "none" {
  if (message.kind !== "user") return "none";
  const latestUserMessage = [...store.messages].reverse().find((item) => item.kind === "user");
  return store.sending && latestUserMessage?.id === message.id ? "pending" : "sent";
}
</script>

<template>
  <section class="page chat-page" aria-labelledby="chat-title">
    <header class="chat-header">
      <button class="icon-button chat-back" type="button" aria-label="返回聊天列表" @click="emit('back')">‹</button>
      <button class="chat-profile" type="button" :disabled="!store.selectedFluctlight" @click="emit('openDetails')">
        <span class="chat-avatar" aria-hidden="true">{{ String(store.selectedFluctlightName ?? "F").slice(0, 1) }}</span>
        <span class="chat-header-copy">
          <strong id="chat-title">{{ store.selectedFluctlightName ?? "对话" }}</strong>
          <small>{{ store.sending ? "正在思考" : "已准备好回复" }}</small>
        </span>
      </button>
    </header>

    <section ref="transcript" class="message-timeline" aria-live="polite" aria-label="对话记录">
      <div v-if="store.loading" class="empty-state">正在加载对话...</div>
      <button v-else-if="store.nextBeforeSequence" class="secondary-button load-older" type="button" @click="store.loadOlder">加载更早记录</button>
      <div v-else-if="!store.selectedFluctlight" class="empty-state">
        <span class="empty-mark" aria-hidden="true">＋</span>
        <h2>还没有 Fluctlight 实例</h2>
        <p>先创建一个实例，再开始你们之间的对话。</p>
        <button class="primary-button" type="button" @click="emit('openInstances')">创建 Fluctlight</button>
      </div>
      <div v-else-if="!store.messages.length" class="empty-state">
        <span class="empty-mark" aria-hidden="true">＋</span>
        <h2>开始与 {{ store.selectedFluctlightName }} 对话</h2>
        <p>分享一件事、一个问题，或此刻正在发生的事情。</p>
        <button class="empty-cta" type="button" @click="composer?.focus()">开始写下第一句话</button>
      </div>

      <article
        v-for="message in store.messages"
        :key="message.id"
        class="message-row"
        :class="message.kind === 'user' ? 'from-user' : 'from-fluctlight'"
      >
        <div class="avatar" aria-hidden="true">{{ message.kind === "user" ? "我" : String(store.selectedFluctlightName ?? "F").slice(0, 1) }}</div>
        <div class="message-bubble">
          <p>{{ message.text }}</p>
          <div v-if="message.attachmentRefs?.length" class="message-media">
            <img v-for="assetId in message.attachmentRefs" :key="assetId" :src="mediaUrl(assetId)" :alt='`${store.selectedFluctlightName ?? "Fluctlight"} 生成的图片`' loading="lazy" />
          </div>
          <div class="message-meta">
            <time>{{ new Date(message.createdAt).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" }) }}</time>
            <span v-if='deliveryStatus(message) !== "none"' class="delivery-status" :class="deliveryStatus(message)" :aria-label='deliveryStatus(message) === "pending" ? "已接收，处理中" : "已回复"'>
              ✓<span v-if='deliveryStatus(message) === "sent"'>✓</span>
            </span>
          </div>
        </div>
      </article>
    </section>

    <p v-if="store.error" class="error-banner" role="alert">{{ store.error }}</p>

    <form class="message-composer" @submit.prevent="send">
      <label class="sr-only" for="message-composer">消息</label>
      <textarea
        id="message-composer"
        ref="composer"
        v-model="draft"
        rows="1"
        maxlength="32000"
        placeholder="写一条消息..."
        :disabled="store.loading || !store.hasConversation || !store.selectedFluctlight"
        @keydown="onKeydown"
      />
      <div class="composer-footer">
        <span class="composer-hint">Enter 发送 · Shift + Enter 换行</span>
        <div class="composer-actions">
          <button v-if="store.sending" class="secondary-button" type="button" @click="store.cancel">取消</button>
          <button class="primary-button send-button" type="submit" :disabled="store.sending || !store.hasConversation || !store.selectedFluctlight || !draft.trim()">发送</button>
        </div>
      </div>
    </form>
  </section>
</template>

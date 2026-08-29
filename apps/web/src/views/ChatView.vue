<script setup lang="ts">
import { nextTick, onMounted, ref, watch } from "vue";
import type { BrowserMessage } from "@fluctlight/browser-client";

import Button from "@/components/ui/button/Button.vue";
import Textarea from "@/components/ui/textarea/Textarea.vue";
import { bffOrigin } from "../runtime-config";
import { useConversationStore } from "../stores/conversations";
import { fluctlightStatusLabel } from "../lib/fluctlight-status";

const emit = defineEmits<{
  back: [];
  openDetails: [];
  openInstances: [];
}>();

const store = useConversationStore();
const draft = ref("");
type ComposerTarget = { focus?: () => void; $el?: unknown };
const composer = ref<ComposerTarget | null>(null);
const transcript = ref<HTMLElement | null>(null);

function focusComposer() {
  const target = composer.value;
  if (!target) return;
  if (typeof target.focus === "function") {
    target.focus();
    return;
  }
  if (target.$el instanceof HTMLElement) target.$el.focus();
}

async function send() {
  const text = draft.value.trim();
  if (!text) return;
  await store.send(text);
  if (!store.canRetry) draft.value = "";
  scrollToLatest("smooth");
  focusComposer();
}

function scrollToLatest(behavior: ScrollBehavior = "auto") {
  void nextTick().then(() => {
    requestAnimationFrame(() => {
      const element = transcript.value;
      if (!element) return;
      element.scrollTo({ top: element.scrollHeight, behavior });
      window.setTimeout(() => element.scrollTo({ top: element.scrollHeight, behavior: "auto" }), 120);
    });
  });
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

function deliveryStatus(message: BrowserMessage): "pending" | "failed" | "sent" | "none" {
  if (message.kind !== "user") return "none";
  if (
    store.canRetry &&
    store.retryTurn?.conversationId === message.conversationId &&
    store.retryTurn.text === message.text
  ) return "failed";
  const latestUserMessage = [...store.messages].reverse().find((item) => item.kind === "user");
  return store.sending && latestUserMessage?.id === message.id ? "pending" : "sent";
}

onMounted(() => scrollToLatest());
watch(() => store.fluctlightId, () => scrollToLatest());
watch(() => store.messages.length, (messageCount, previousCount) => {
  if (messageCount && previousCount === 0 && !store.loading) scrollToLatest();
});
</script>

<template>
  <section class="page chat-page" aria-labelledby="chat-title">
    <header class="chat-header">
      <Button class="icon-button chat-back" variant="ghost" type="button" aria-label="返回聊天列表" @click="emit('back')">‹</Button>
      <Button class="chat-profile" variant="ghost" type="button" :disabled="!store.selectedFluctlight" @click="emit('openDetails')">
        <span class="chat-avatar" aria-hidden="true">{{ String(store.selectedFluctlightName ?? "F").slice(0, 1) }}</span>
        <span class="chat-header-copy">
          <strong id="chat-title">{{ store.selectedFluctlightName ?? "选择会话" }}</strong>
          <small>{{ store.sending ? "正在思考" : store.selectedFluctlight ? fluctlightStatusLabel(store.selectedFluctlight.status) : "等待选择" }}</small>
        </span>
      </Button>
      <Button class="icon-button chat-more" variant="ghost" type="button" aria-label="查看对话详情" @click="emit('openDetails')">⋯</Button>
    </header>

    <section ref="transcript" class="message-timeline" aria-live="polite" aria-label="对话记录">
      <div v-if="store.loading" class="empty-state">正在加载对话...</div>
      <Button v-else-if="store.nextBeforeSequence" class="secondary-button load-older" variant="outline" type="button" @click="store.loadOlder">加载更早记录</Button>
      <div v-else-if="!store.selectedFluctlight" class="empty-state">
        <span class="empty-mark" aria-hidden="true">＋</span>
        <h2>选择一个会话进行聊天</h2>
        <p>从左侧最近会话中选择一个摇光，继续你们之间的对话。</p>
        <Button class="secondary-button" variant="outline" type="button" @click="emit('openInstances')">管理摇光实例</Button>
      </div>
      <div v-else-if="!store.messages.length" class="empty-state">
        <span class="empty-mark" aria-hidden="true">＋</span>
        <h2>开始与 {{ store.selectedFluctlightName }} 对话</h2>
        <p>分享一件事、一个问题，或此刻正在发生的事情。</p>
        <Button class="empty-cta" variant="ghost" type="button" @click="focusComposer">开始写下第一句话</Button>
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
            <span v-if='deliveryStatus(message) !== "none"' class="delivery-status" :class="deliveryStatus(message)" :aria-label='deliveryStatus(message) === "pending" ? "已接收，处理中" : deliveryStatus(message) === "failed" ? "发送失败，可重试" : "已回复"'>
              <span v-if='deliveryStatus(message) === "failed"'>!</span><template v-else>✓<span v-if='deliveryStatus(message) === "sent"'>✓</span></template>
            </span>
          </div>
        </div>
      </article>
    </section>

    <div v-if="store.error" class="error-banner" role="alert">
      <span>{{ store.error }}</span>
      <Button v-if="store.canRetry" class="secondary-button" variant="outline" type="button" :disabled="store.retrying" @click="store.retry">{{ store.retrying ? "重试中..." : "重试" }}</Button>
    </div>

    <form class="message-composer" @submit.prevent="send">
      <label class="sr-only" for="message-composer">消息</label>
      <Textarea
        id="message-composer"
        ref="composer"
        v-model="draft"
        rows="1"
        maxlength="32000"
        placeholder="写一条消息..."
        :disabled="store.loading || store.canRetry || !store.hasConversation || !store.selectedFluctlight"
        @keydown="onKeydown"
      />
      <div class="composer-footer">
        <span class="composer-hint">Enter 发送 · Shift + Enter 换行</span>
        <div class="composer-actions">
          <Button v-if="store.sending" class="secondary-button" variant="outline" type="button" @click="store.cancel">取消</Button>
          <Button class="primary-button send-button" type="submit" :disabled="store.sending || store.canRetry || !store.hasConversation || !store.selectedFluctlight || !draft.trim()">发送</Button>
        </div>
      </div>
    </form>
  </section>
</template>

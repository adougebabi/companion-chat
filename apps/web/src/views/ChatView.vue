<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import type { BrowserMessage } from "@fluctlight/browser-client";

import Button from "@/components/ui/button/Button.vue";
import Textarea from "@/components/ui/textarea/Textarea.vue";
import { apiOrigin } from "../runtime-config";
import { useConversationStore } from "../stores/conversations";
import { fluctlightStatusLabel } from "../lib/fluctlight-status";
import { messageTimeLabel } from "../lib/message-time";

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
const senderOptions = computed(() => {
  const participants = store.conversationParticipants;
  return participants
    .filter((participant) => participant.status === "active" && participant.actorId !== store.fluctlightId)
    .map((participant) => {
      if (participant.role === "owner") return { id: "", label: "我（actor_user）" };
      const fluctlight = store.fluctlights.find((item) => item.id === participant.actorId);
      const name = typeof fluctlight?.identity.name === "string" && fluctlight.identity.name.trim() ? fluctlight.identity.name : participant.actorId;
      return { id: participant.actorId, label: `${name}（${participant.actorId}）` };
    });
});

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
  if (!text || store.sending || store.hasPendingTurn) return;
  // The submitted text is already represented by the optimistic message and
  // retryTurn; keeping it in the editor makes a queued request look unsent.
  draft.value = "";
  await store.send(text);
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
  return new URL(`/api/media/${encodeURIComponent(assetId)}`, apiOrigin).toString();
}

function deliveryStatus(message: BrowserMessage): "pending" | "failed" | "sent" | "none" {
  if (message.kind !== "user") return "none";
	if (message.turnStatus === "pending" || message.turnStatus === "running") return "pending";
	if (message.turnStatus === "failed" || message.turnStatus === "cancelled") return "failed";
	if (message.turnStatus === "completed") return "sent";
	if (store.queuedMessageId === message.id) return "pending";
	if (
	  store.canRetry &&
	  store.retryTurn?.conversationId === message.conversationId &&
	  store.retryTurn.messageId === message.id
	) return "failed";
  const latestUserMessage = [...store.messages].reverse().find((item) => item.kind === "user");
  return store.sending && latestUserMessage?.id === message.id ? "pending" : "sent";
}

function deliveryLabel(message: BrowserMessage): string {
	const status = deliveryStatus(message);
	if (status === "pending") return "已接收，处理中";
	if (status === "failed") {
		if (message.turnRetryable === false) return "本次生成正在结束";
		return message.turnStatus === "cancelled" ? "回复已取消，可重试" : "回复失败，可重试";
	}
	return "已回复";
}

let refreshTimer: ReturnType<typeof setInterval> | null = null;
onMounted(() => {
	scrollToLatest();
	refreshTimer = setInterval(() => {
		if (document.visibilityState === "visible" && (store.hasPendingTurn || store.hasSettlingTurn)) void store.refreshActiveTurn();
	}, 2000);
});
onUnmounted(() => { if (refreshTimer) clearInterval(refreshTimer); });
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
			<time :datetime="message.senderSentAt ?? message.createdAt">{{ messageTimeLabel(message) }}</time>
            <span v-if='deliveryStatus(message) !== "none"' class="delivery-status" :class="deliveryStatus(message)" :aria-label="deliveryLabel(message)">
              <span v-if='deliveryStatus(message) === "failed"'>!</span><template v-else>✓<span v-if='deliveryStatus(message) === "sent"'>✓</span></template>
            </span>
          </div>
        </div>
      </article>
    </section>

    <div v-if="store.error || store.canRetry" class="error-banner" role="alert">
      <span>{{ store.error }}</span>
      <Button v-if="store.canRetry" class="secondary-button" variant="outline" type="button" :disabled="store.retrying" @click="store.retry">{{ store.retrying ? "重试中..." : store.retryIsModelFailure ? "重试回复" : "重新发送" }}</Button>
      <Button v-if="store.canRetry" class="secondary-button" variant="ghost" type="button" @click="store.dismissRetry">忽略这条</Button>
    </div>

    <form class="message-composer" @submit.prevent="send">
      <label class="sr-only" for="message-composer">消息</label>
      <div class="composer-row">
        <Textarea
          id="message-composer"
          ref="composer"
          v-model="draft"
          rows="1"
          maxlength="32000"
          placeholder="写一条消息..."
          :disabled="store.loading || !store.hasConversation || !store.selectedFluctlight"
          @keydown="onKeydown"
        />
        <div class="composer-actions">
          <Button v-if="store.canCancel" class="secondary-button" variant="outline" type="button" @click="store.cancel">取消</Button>
          <Button class="primary-button send-button" type="submit" :disabled="store.sending || store.hasPendingTurn || !store.hasConversation || !store.selectedFluctlight || !draft.trim()">发送</Button>
        </div>
      </div>
      <div class="composer-footer">
        <span class="composer-hint">Enter 发送 · Shift + Enter 换行</span>
        <label v-if="senderOptions.length > 1" class="sender-picker" for="conversation-sender">发送身份
          <select id="conversation-sender" v-model="store.senderActorId" :disabled="store.sending">
            <option v-for="option in senderOptions" :key="option.id || 'actor_user'" :value="option.id">{{ option.label }}</option>
          </select>
        </label>
      </div>
    </form>
  </section>
</template>

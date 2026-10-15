<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import type { BrowserMessage } from "@fluctlight/browser-client";

import Button from "@/components/ui/button/Button.vue";
import { MoreHorizontal } from "@lucide/vue";
import { apiOrigin } from "../runtime-config";
import { useConversationStore } from "../stores/conversations";
import { fluctlightStatusLabel } from "../lib/fluctlight-status";
import FeedbackNotice from "../components/product/FeedbackNotice.vue";
import ChoiceSelect from "../components/product/ChoiceSelect.vue";
import { userError } from "../lib/user-feedback";
import ChatPanel from "../components/chat/ChatPanel.vue";
import { deliveryStatus } from "../components/chat/delivery";

const emit = defineEmits<{
  back: [];
  openDetails: [];
  openInstances: [];
  openSection: [view: "life" | "growth" | "people" | "media" | "profile"];
}>();

const store = useConversationStore();
const draft = ref("");
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
  document.getElementById("message-composer")?.focus();
}

async function send() {
  const text = draft.value.trim();
  if (!text || store.sending) return;
  // The submitted text is already represented by the optimistic message and
  // retryTurn; keeping it in the editor makes a queued request look unsent.
  draft.value = "";
  await store.send(text);
  focusComposer();
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

function statusFor(message: BrowserMessage) {
  const latestUserMessage = [...store.messages].reverse().find((item) => item.kind === "user");
  return deliveryStatus(message, {
    sending: store.sending,
    queuedMessageId: store.queuedMessageId,
    canRetry: store.canRetry,
    retryMessageId: store.retryTurn?.messageId,
    latestUserMessageId: latestUserMessage?.id,
  });
}

const streamingId = computed(() => store.messages.find((message) => message.id.startsWith("stream-"))?.id ?? null);
const waiting = computed(() => store.sending && !streamingId.value);
const peerLabel = computed(() => String(store.selectedFluctlightName ?? "F").slice(0, 1));

let refreshTimer: ReturnType<typeof setInterval> | null = null;
onMounted(() => {
  refreshTimer = setInterval(() => {
    if (document.visibilityState === "visible" && (store.hasPendingTurn || store.hasSettlingTurn)) void store.refreshActiveTurn();
  }, 2000);
});
onUnmounted(() => { if (refreshTimer) clearInterval(refreshTimer); });
</script>

<template>
  <section class="page chat-page chat-workspace" aria-labelledby="chat-title">
    <ChatPanel
      :title="store.selectedFluctlightName ?? '选择会话'"
      :subtitle="store.sending ? '正在等待回复' : store.selectedFluctlight ? fluctlightStatusLabel(store.selectedFluctlight.status) : '等待选择'"
      :avatar-label="peerLabel"
      :messages="store.messages"
      :loading="store.loading"
      :history-loading="store.historyLoading"
      :has-older="Boolean(store.nextBeforeSequence)"
      :waiting="waiting"
      :sending="store.sending"
      :can-cancel="store.canCancel"
      :draft="draft"
      :disabled="store.loading || !store.hasConversation || !store.selectedFluctlight"
      :streaming-id="streamingId"
      :status-for="statusFor"
      :media-url="mediaUrl"
      :empty-title="store.selectedFluctlight ? `开始与 ${store.selectedFluctlightName} 对话` : '选择一个会话进行聊天'"
      :empty-copy="store.selectedFluctlight ? '分享一件事、一个问题，或此刻正在发生的事情。' : '从角色库选择一个角色，继续你们之间的对话。'"
      @update:draft="draft = $event"
      @submit="send"
      @cancel="store.cancel"
      @keydown="onKeydown"
      @load-older="store.loadOlder"
      @focus-composer="focusComposer"
    >
      <template #actions>
        <Button class="icon-button chat-more" variant="ghost" type="button" aria-label="查看对话详情" @click="emit('openDetails')"><MoreHorizontal :size="20" aria-hidden="true" /></Button>
      </template>
      <template #notices>
        <FeedbackNotice v-if="store.error || store.canRetry || store.retrying" :tone="store.retrying ? 'info' : 'error'" :message="store.retrying ? '正在提交重试请求并等待回复…' : userError(store.error, '回复未完成，可以重试。')" :detail="store.retrying ? '' : store.error">
          <Button v-if="store.canRetry || store.retrying" class="secondary-button" variant="outline" type="button" :disabled="store.retrying" @click="store.retry">{{ store.retrying ? "重试中..." : store.retryIsModelFailure ? "重试回复" : "重新发送" }}</Button>
          <Button v-if="store.canRetry" class="secondary-button" variant="ghost" type="button" @click="store.dismissRetry">忽略这条</Button>
        </FeedbackNotice>
        <div v-if="!store.selectedFluctlight && !store.loading" class="banner info">当前没有打开的会话。<button class="btn sm" type="button" @click="emit('openInstances')">前往角色库</button></div>
        <label v-if="senderOptions.length > 1" class="sender-picker" for="conversation-sender">发送身份
          <ChoiceSelect id="conversation-sender" :model-value="store.senderActorId ?? ''" :disabled="store.sending" :options="senderOptions.map(option => ({ value: option.id, label: option.label }))" @update:model-value="value => store.senderActorId = value || null" />
        </label>
      </template>
    </ChatPanel>

    <aside class="chatside stack">
      <div class="card">
        <h3>此刻</h3>
        <p class="small muted">生活此刻页尚未接入。这里不展示推测中的心情或位置。</p>
        <button class="btn sm" type="button" @click="emit('openSection', 'life')">打开生活</button>
      </div>
      <div class="card">
        <h3>进行中</h3>
        <p class="small muted">目标、穿着和相关记忆仍在既有治理里，聊天栏不另造一份。</p>
        <button class="btn ghost sm" type="button" @click="emit('openSection', 'growth')">记忆入口</button>
        <button class="btn ghost sm" type="button" @click="emit('openDetails')">查看资料</button>
      </div>
    </aside>
  </section>
</template>

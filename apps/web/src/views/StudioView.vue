<script setup lang="ts">
import { ref } from "vue";
import type { BrowserMessage } from "@fluctlight/browser-client";

import ChatPanel from "../components/chat/ChatPanel.vue";

const categories = ["角色模板", "人格片段", "地点设定", "物品", "事件模板", "视觉风格"] as const;
const selected = ref("");
const applyOpen = ref(new URLSearchParams(window.location.search).has("resource") || /\/studio\/apply\/?$/.test(window.location.pathname));
const applyResource = new URLSearchParams(window.location.search).get("resource") ?? "";
const draft = ref("");
const messages = ref<BrowserMessage[]>([]);
let sequence = 0;

function statusFor(): "none" { return "none"; }

function submit() {
  const text = draft.value.trim();
  if (!text) return;
  draft.value = "";
  const createdAt = new Date().toISOString();
  sequence += 1;
  messages.value = [...messages.value, {
    id: `studio-preview-user-${sequence}`,
    conversationId: "studio-preview",
    sequence,
    authorActorId: "owner-preview",
    kind: "user",
    text,
    attachmentRefs: [],
    createdAt,
  }];
  sequence += 1;
  messages.value = [...messages.value, {
    id: `studio-preview-assistant-${sequence}`,
    conversationId: "studio-preview",
    sequence,
    authorActorId: "studio-assistant",
    kind: "assistant",
    text: "设定助手的预览接口尚未接入。这段文字只留在本页，没有写入角色媒体、日记、记忆或生活事实。",
    attachmentRefs: [],
    createdAt,
  }];
}

function discard() {
  messages.value = [];
  draft.value = "";
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === "Enter" && !event.shiftKey) {
    event.preventDefault();
    submit();
  }
}
</script>

<template>
  <section class="page life-page studio-page" aria-labelledby="studio-title">
    <header class="pagehead">
      <h1 id="studio-title">创作中心</h1>
      <span class="badge b-plain">BP-15</span>
      <span class="badge b-preview">创作预览·不进入生活</span>
    </header>
    <p class="banner warn">试生成和设定助手只存在于本页。丢弃会清掉本地预览，不会改角色媒体库、日记、记忆或生活事实。</p>
    <div class="studio-split">
      <div class="stack">
        <article class="card">
          <h2>全局定义</h2>
          <p class="small muted">搜索范围：全局定义。新建、编辑、复制和搜索都尚未接入。</p>
          <div class="chips">
            <button v-for="item in categories" :key="item" class="chip" :class="{ on: selected === item }" type="button" @click="selected = item">{{ item }}</button>
          </div>
          <p v-if="selected" class="statebox">{{ selected }} 的资源编辑尚未接入。编辑定义不会自动改变已运行实例。</p>
        </article>
        <article class="card">
          <h2>事件模板</h2>
          <p class="small muted">试运行结果标「创作预览·不进入生活」，不写入世界事件。含永久伤害类型的模板在保存时会被拒绝，不能通过导入或复制绕过。</p>
          <p class="statebox">事件模板的保存与发布尚未接入。</p>
        </article>
        <article class="card">
          <h2>应用到实例</h2>
          <p class="small muted">差异预览、冲突逐项保留和版本记录都尚未接入。这里没有确认应用按钮。</p>
          <p v-if="applyResource" class="small">链接中的资源：{{ applyResource }}</p>
          <button class="btn sm" type="button" @click="applyOpen = !applyOpen">{{ applyOpen ? "收起说明" : "查看应用说明" }}</button>
          <p v-if="applyOpen" class="statebox">应用到实例尚未接入。不会把创作定义写入角色日程、关系或地点。</p>
        </article>
      </div>
      <div class="studio-preview">
        <ChatPanel
          title="设定助手"
          subtitle="创作预览·不进入生活"
          avatar-label="设"
          :messages="messages"
          :loading="false"
          :history-loading="false"
          :has-older="false"
          :waiting="false"
          :sending="false"
          :can-cancel="false"
          :draft="draft"
          :disabled="false"
          self-label="我"
          :status-for="statusFor"
          empty-title="预览会话是空的"
          empty-copy="写下设定想法。这段对话只留在本页，不会进入角色生活。"
          @update:draft="draft = $event"
          @submit="submit"
          @keydown="onKeydown"
        >
          <template #actions>
            <button class="btn sm" type="button" @click="discard">丢弃预览</button>
          </template>
        </ChatPanel>
      </div>
    </div>
    <p class="statebox">重新生成和存为资源素材尚未接入。当前回复不会调用生活会话。</p>
  </section>
</template>

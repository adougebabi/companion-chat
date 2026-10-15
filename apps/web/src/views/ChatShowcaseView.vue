<script setup lang="ts">
import { computed, ref } from "vue";
import type { BrowserMessage } from "@fluctlight/browser-client";

import ChatPanel from "../components/chat/ChatPanel.vue";
import BottomNav from "../components/layout/BottomNav.vue";
import WorkspaceNav from "../components/layout/WorkspaceNav.vue";

const scenarios = ["正常", "等待", "流式", "完成", "失败", "取消", "长文本", "窄容器"] as const;
type Scenario = (typeof scenarios)[number];
const initialScenario = scenarios.find((item) => item === new URLSearchParams(window.location.search).get("scene")) ?? "正常";
const scenario = ref<Scenario>(initialScenario);
const draft = ref("");
const drawerOpen = ref(new URLSearchParams(window.location.search).get("drawer") === "1");
const narrow = computed(() => scenario.value === "窄容器");

function message(partial: Partial<BrowserMessage> & Pick<BrowserMessage, "id" | "kind" | "text">): BrowserMessage {
  return {
    conversationId: "fixture-conversation",
    sequence: 1,
    authorActorId: partial.kind === "user" ? "owner" : "fixture-character",
    attachmentRefs: [],
    createdAt: "2026-10-15T01:00:00Z",
    senderTimezone: "Asia/Shanghai",
    senderUtcOffsetMinutes: 480,
    senderSentAt: "2026-10-15T09:00:00Z",
    ...partial,
  };
}

const history = [
  message({ id: "hist-user", kind: "user", text: "今天海边的风很大。", sequence: 1, turnStatus: "completed" }),
  message({ id: "hist-peer", kind: "assistant", text: "我在屋里听着雨。这句是历史消息，打开时应该直接显示，不重播打字。", sequence: 2 }),
];

const messages = computed(() => {
  if (scenario.value === "等待") return history;
  if (scenario.value === "流式") return [...history, message({ id: "stream-fixture", kind: "assistant", text: "这是正在接收的回复，只会轻量跟上，不会把积压的长句慢慢逐字拖很久。🌿", sequence: 3 })];
  if (scenario.value === "完成") return [...history, message({ id: "done-peer", kind: "assistant", text: "服务端已经校准过的完整回复。", sequence: 3 })];
  if (scenario.value === "失败") return [...history, message({ id: "fail-user", kind: "user", text: "再告诉我一件事。", sequence: 3, turnStatus: "failed", turnRetryable: true })];
  if (scenario.value === "取消") return [...history, message({ id: "cancel-user", kind: "user", text: "先停一下。", sequence: 3, turnStatus: "cancelled", turnRetryable: true })];
  if (scenario.value === "长文本" || scenario.value === "窄容器") {
    return [...history, message({ id: "long-peer", kind: "assistant", text: "这是一段很长的中文回复，用来检查换行、气泡宽度和窄容器。".repeat(12) + " 结尾有组合表情 👨‍👩‍👧。", sequence: 3 })];
  }
  return history;
});

function statusFor(item: BrowserMessage) {
  if (item.turnStatus === "failed" || item.turnStatus === "cancelled") return "failed" as const;
  if (item.turnStatus === "completed") return "sent" as const;
  if (item.kind === "user") return "sent" as const;
  return "none" as const;
}
</script>

<template>
  <div class="sage-shell showcase" data-test="chat-showcase">
    <header class="topbar">
      <span class="brand"><span class="brand-dot" aria-hidden="true"></span><span class="hide-n">摇光</span></span>
      <span class="charsw"><span class="av" aria-hidden="true">样</span><b>样本角色</b></span>
      <div class="scopebar"><span class="badge b-角色范围">角色范围</span><span class="hide-n">样本角色 · 组件测试</span></div>
      <span class="spacer"></span>
      <span class="badge b-preview">不是业务数据</span>
    </header>
    <div class="layout">
      <aside class="sidenav"><WorkspaceNav active-view="chat" character-name="样本角色" /></aside>
      <section class="main">
    <header class="pagehead">
      <h1>聊天组件场景</h1>
    </header>
    <div class="chips" role="tablist" aria-label="组件场景">
      <button v-for="item in scenarios" :key="item" class="chip" :class="{ on: scenario === item }" type="button" @click="scenario = item">{{ item }}</button>
    </div>
    <div class="showcase-frame" :class="{ narrow }">
      <ChatPanel
        title="样本角色"
        subtitle="远程私聊 · 组件场景"
        avatar-label="样"
        :messages="messages"
        :loading="false"
        :history-loading="false"
        :has-older="scenario === '正常'"
        :waiting="scenario === '等待'"
        :sending="scenario === '等待'"
        :can-cancel="scenario === '等待' || scenario === '流式'"
        :draft="draft"
        :disabled="false"
        :streaming-id="scenario === '流式' ? 'stream-fixture' : null"
        :status-for="statusFor"
        empty-title="还没有消息"
        empty-copy="这个场景只检查空状态，不会发送。"
        @update:draft="draft = $event"
        @submit="draft = ''"
        @cancel="scenario = '取消'"
        @load-older="scenario = '正常'"
      />
    </div>
    <button class="btn" type="button" @click="drawerOpen = true">打开抽屉</button>
      </section>
    </div>
    <template v-if="drawerOpen">
      <div class="scrim" @click="drawerOpen = false" />
      <aside class="drawer" role="dialog" aria-label="示例抽屉">
        <header><h2>对话详情</h2><button class="btn ghost sm" type="button" @click="drawerOpen = false">关闭</button></header>
        <div class="body"><p>这是正式外壳的抽屉，不是嵌入的 demo。样本内容不会写入会话。</p></div>
      </aside>
    </template>
    <BottomNav active-view="chat" @open-menu="drawerOpen = true" />
  </div>
</template>

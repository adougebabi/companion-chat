<script setup lang="ts">
import { computed } from "vue";

import type { WorkspaceView } from "../../app/navigation";
import { useConversationStore } from "../../stores/conversations";

const props = defineProps<{ activeView: WorkspaceView }>();
const emit = defineEmits<{ select: [fluctlightId: string] }>();
const store = useConversationStore();
const activeView = computed(() => props.activeView === "diagnostics" ? "settings" : props.activeView);
const items = computed(() => [...store.fluctlights].sort((a, b) => (Date.parse(b.last_conversation_at ?? "") || 0) - (Date.parse(a.last_conversation_at ?? "") || 0)));
function nameOf(item: (typeof store.fluctlights)[number]) { return String(item.identity.name ?? item.id); }
function timeOf(value?: string | null) { return value ? new Date(value).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" }) : ""; }
</script>

<template>
  <aside class="desktop-context-panel">
    <template v-if="activeView === 'settings'">
      <header class="desktop-context-header settings-context-header"><div><p class="eyebrow">FLUCTLIGHT</p><h2>设置</h2></div><button class="context-edit" type="button">编辑</button></header>
      <label class="context-search" for="desktop-settings-search"><span aria-hidden="true">⌕</span><span class="sr-only">搜索设置</span><input id="desktop-settings-search" type="search" placeholder="搜索" /></label>
      <div class="settings-account-card"><span class="avatar persona-avatar">{{ String(store.selectedFluctlightName ?? "我").slice(0, 1) }}</span><div><strong>{{ store.selectedFluctlightName ?? "所有者" }}</strong><small>{{ store.selectedFluctlight ? "Fluctlight 工作区" : "本地账户" }}</small></div><span>›</span></div>
      <nav class="settings-context-list" aria-label="设置分类"><button class="settings-context-link accent" type="button"><span>◌</span>更改个人资料颜色</button><button class="settings-context-link accent" type="button"><span>♙</span>添加帐号</button><button class="settings-context-link" type="button"><span class="settings-icon pink">●</span>我的资料<span>›</span></button><button class="settings-context-link" type="button"><span class="settings-icon gray">⚙</span>通用<span>›</span></button><button class="settings-context-link" type="button"><span class="settings-icon red">□</span>通知<span>›</span></button><button class="settings-context-link" type="button"><span class="settings-icon blue">▣</span>隐私和安全<span>›</span></button><button class="settings-context-link" type="button"><span class="settings-icon green">▤</span>数据和存储<span>›</span></button><button class="settings-context-link" type="button"><span class="settings-icon orange">▭</span>所有设备<span>›</span></button></nav>
    </template>
    <template v-else>
      <header class="desktop-context-header"><div><p class="eyebrow">MESSAGES</p><h2>聊天</h2></div><button class="context-compose" type="button" aria-label="新建 Fluctlight">＋</button></header>
      <div class="desktop-context-tabs"><span class="selected">最近</span><span>已归档</span></div>
      <div v-if="items.length" class="desktop-conversation-items"><button v-for="item in items" :key="item.id" class="desktop-conversation-item" :class="{ selected: item.id === store.fluctlightId }" type="button" @click="emit('select', item.id)"><span class="avatar persona-avatar">{{ nameOf(item).slice(0, 1) }}</span><span class="desktop-conversation-copy"><strong>{{ nameOf(item) }}</strong><small>{{ item.status === "paused" ? "已暂停" : "准备好回复" }}</small></span><span class="desktop-conversation-meta"><time>{{ timeOf(item.last_conversation_at) }}</time><b v-if="item.unread_count">{{ item.unread_count }}</b></span></button></div>
      <div v-else class="desktop-list-empty">还没有对话<br />先创建一个 Fluctlight。</div>
    </template>
  </aside>
</template>

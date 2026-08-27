<script setup lang="ts">
import { computed } from "vue";

import { useConversationStore } from "../../stores/conversations";

const emit = defineEmits<{ select: [fluctlightId: string] }>();
const store = useConversationStore();
const items = computed(() => [...store.fluctlights].sort((a, b) => (Date.parse(b.last_conversation_at ?? "") || 0) - (Date.parse(a.last_conversation_at ?? "") || 0)));
function nameOf(item: (typeof store.fluctlights)[number]) { return String(item.identity.name ?? item.id); }
function initialOf(item: (typeof store.fluctlights)[number]) { return nameOf(item).slice(0, 1); }
function timeOf(value?: string | null) { return value ? new Date(value).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" }) : ""; }
</script>

<template>
  <aside class="desktop-conversation-list" aria-label="最近对话">
    <header class="desktop-list-header"><div><p class="eyebrow">MESSAGES</p><h2>聊天</h2></div><span class="desktop-list-count">{{ items.length }}</span></header>
    <div class="desktop-list-tabs"><span class="selected">最近</span><span>未读</span></div>
    <div v-if="items.length" class="desktop-conversation-items"><button v-for="item in items" :key="item.id" class="desktop-conversation-item" :class="{ selected: item.id === store.fluctlightId }" type="button" @click="emit('select', item.id)"><span class="avatar persona-avatar">{{ initialOf(item) }}</span><span class="desktop-conversation-copy"><strong>{{ nameOf(item) }}</strong><small>{{ item.status === "paused" ? "已暂停" : "准备好回复" }}</small></span><span class="desktop-conversation-meta"><time>{{ timeOf(item.last_conversation_at) }}</time><b v-if="item.unread_count">{{ item.unread_count }}</b></span></button></div>
    <div v-else class="desktop-list-empty">还没有对话<br />先创建一个 Fluctlight。</div>
  </aside>
</template>

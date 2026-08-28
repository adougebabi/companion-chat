<script setup lang="ts">
import { computed, ref } from "vue";
import {
  Bell,
  ChevronRight,
  Database,
  Monitor,
  Palette,
  Plus,
  Search,
  Settings as SettingsIcon,
  ShieldCheck,
  UserPlus,
  UserRound,
} from "@lucide/vue";

import Badge from "@/components/ui/badge/Badge.vue";
import Button from "@/components/ui/button/Button.vue";
import Input from "@/components/ui/input/Input.vue";
import type { WorkspaceView } from "../../app/navigation";
import BottomNav from "./BottomNav.vue";
import { useConversationStore } from "../../stores/conversations";
import { fluctlightStatusLabel } from "../../lib/fluctlight-status";

const props = defineProps<{ activeView: WorkspaceView }>();
const emit = defineEmits<{ select: [fluctlightId: string]; navigate: [view: WorkspaceView] }>();
const store = useConversationStore();
const contextView = computed(() => props.activeView === "diagnostics" ? "settings" : props.activeView);
const chatSearch = ref("");
const items = computed(() => [...store.fluctlights].filter((item) => { const query = chatSearch.value.trim().toLowerCase(); return !query || String(item.identity.name ?? "").toLowerCase().includes(query) || item.id.toLowerCase().includes(query); }).sort((a, b) => (Date.parse(b.last_conversation_at ?? "") || 0) - (Date.parse(a.last_conversation_at ?? "") || 0)));
function nameOf(item: (typeof store.fluctlights)[number]) { return String(item.identity.name ?? item.id); }
function timeOf(value?: string | null) { return value ? new Date(value).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" }) : ""; }
</script>

<template>
  <aside class="desktop-context-panel">
    <template v-if="contextView === 'settings'">
      <header class="desktop-context-header settings-context-header"><div><p class="eyebrow">FLUCTLIGHT</p><h2>设置</h2></div><Button class="context-edit" variant="ghost" type="button">编辑</Button></header>
      <label class="context-search" for="desktop-settings-search"><Search :size="14" :stroke-width="2" aria-hidden="true" /><span class="sr-only">搜索设置</span><Input id="desktop-settings-search" type="search" placeholder="搜索" /></label>
      <div class="settings-account-card"><span class="avatar persona-avatar">{{ String(store.selectedFluctlightName ?? "我").slice(0, 1) }}</span><div><strong>{{ store.selectedFluctlightName ?? "所有者" }}</strong><small>{{ store.selectedFluctlight ? "Fluctlight 工作区" : "本地账户" }}</small></div><span aria-hidden="true"><ChevronRight :size="18" :stroke-width="2" /></span></div>
      <nav class="settings-context-list" aria-label="设置分类"><Button class="settings-context-link accent justify-normal" variant="ghost" type="button"><Palette :size="17" :stroke-width="2" aria-hidden="true" />更改个人资料颜色</Button><Button class="settings-context-link accent justify-normal" variant="ghost" type="button"><UserPlus :size="17" :stroke-width="2" aria-hidden="true" />添加帐号</Button><Button class="settings-context-link justify-normal" variant="ghost" type="button"><UserRound class="settings-icon pink" :size="17" :stroke-width="2" aria-hidden="true" />我的资料<ChevronRight :size="16" :stroke-width="2" aria-hidden="true" /></Button><Button class="settings-context-link justify-normal" variant="ghost" type="button"><SettingsIcon class="settings-icon gray" :size="17" :stroke-width="2" aria-hidden="true" />通用<ChevronRight :size="16" :stroke-width="2" aria-hidden="true" /></Button><Button class="settings-context-link justify-normal" variant="ghost" type="button"><Bell class="settings-icon red" :size="17" :stroke-width="2" aria-hidden="true" />通知<ChevronRight :size="16" :stroke-width="2" aria-hidden="true" /></Button><Button class="settings-context-link justify-normal" variant="ghost" type="button"><ShieldCheck class="settings-icon blue" :size="17" :stroke-width="2" aria-hidden="true" />隐私和安全<ChevronRight :size="16" :stroke-width="2" aria-hidden="true" /></Button><Button class="settings-context-link justify-normal" variant="ghost" type="button"><Database class="settings-icon green" :size="17" :stroke-width="2" aria-hidden="true" />数据和存储<ChevronRight :size="16" :stroke-width="2" aria-hidden="true" /></Button><Button class="settings-context-link justify-normal" variant="ghost" type="button"><Monitor class="settings-icon orange" :size="17" :stroke-width="2" aria-hidden="true" />所有设备<ChevronRight :size="16" :stroke-width="2" aria-hidden="true" /></Button></nav>
    </template>
    <template v-else>
      <header class="desktop-context-header"><div><p class="eyebrow">MESSAGES</p><h2>聊天</h2></div><Button class="context-compose" variant="ghost" size="icon-lg" type="button" aria-label="新建 Fluctlight"><Plus :size="18" :stroke-width="2" aria-hidden="true" /></Button></header>
      <label class="context-search" for="desktop-chat-search"><Search :size="14" :stroke-width="2" aria-hidden="true" /><span class="sr-only">搜索聊天</span><Input id="desktop-chat-search" v-model="chatSearch" type="search" placeholder="搜索" /></label>
      <div class="desktop-context-tabs"><span class="selected">最近</span><span>已归档</span></div>
      <div v-if="items.length" class="desktop-conversation-items"><Button v-for="item in items" :key="item.id" class="desktop-conversation-item justify-normal" variant="ghost" :class="{ selected: item.id === store.fluctlightId }" type="button" @click="emit('select', item.id)"><span class="avatar persona-avatar">{{ nameOf(item).slice(0, 1) }}</span><span class="desktop-conversation-copy"><strong>{{ nameOf(item) }}</strong><small><Badge variant="secondary" :class="{ paused: item.status === 'paused', muted: item.status === 'retired' }">{{ fluctlightStatusLabel(item.status) }}</Badge></small></span><span class="desktop-conversation-meta"><time>{{ timeOf(item.last_conversation_at) }}</time><b v-if="item.unread_count">{{ item.unread_count }}</b></span></Button></div>
      <div v-else class="desktop-list-empty">还没有对话<br />先创建一个 Fluctlight。</div>
    </template>
    <BottomNav class="desktop-context-nav" :active-view="props.activeView" @navigate="emit('navigate', $event)" />
  </aside>
</template>

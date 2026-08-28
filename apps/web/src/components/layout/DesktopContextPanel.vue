<script setup lang="ts">
import { computed, ref } from "vue";
import {
  Activity,
  Bot,
  ChevronRight,
  Gauge,
  Image,
  Link2,
  Plus,
  Search,
  Server,
  ShieldCheck,
  Workflow,
} from "@lucide/vue";

import Badge from "@/components/ui/badge/Badge.vue";
import Button from "@/components/ui/button/Button.vue";
import Input from "@/components/ui/input/Input.vue";
import { diagnosticsSections, settingsSections, type WorkspaceSection, type WorkspaceView } from "../../app/navigation";
import BottomNav from "./BottomNav.vue";
import { useConversationStore } from "../../stores/conversations";
import { useControlCenterStore } from "../../stores/control-center";
import { fluctlightStatusLabel } from "../../lib/fluctlight-status";

const props = defineProps<{ activeView: WorkspaceView; activeSection?: WorkspaceSection | null }>();
const emit = defineEmits<{ select: [fluctlightId: string]; navigate: [view: WorkspaceView]; navigateSection: [view: "settings" | "diagnostics", section: WorkspaceSection | null]; create: [] }>();
const store = useConversationStore();
const controlCenter = useControlCenterStore();
const contextView = computed(() => props.activeView);
const chatSearch = ref("");
const chatGroupId = ref<string | null>(null);
const items = computed(() => [...store.fluctlights].filter((item) => { const query = chatSearch.value.trim().toLowerCase(); return !query || String(item.identity.name ?? "").toLowerCase().includes(query) || item.id.toLowerCase().includes(query); }).sort((a, b) => (Date.parse(b.last_conversation_at ?? "") || 0) - (Date.parse(a.last_conversation_at ?? "") || 0)));
const groups = computed(() => [...controlCenter.actorGroups].sort((left, right) => { if (left.name === "默认") return -1; if (right.name === "默认") return 1; return left.name.localeCompare(right.name, "zh-CN"); }));
const visibleItems = computed(() => {
  if (!chatGroupId.value) return items.value;
  const group = groups.value.find((item) => item.id === chatGroupId.value);
  return group ? items.value.filter((item) => group.actor_ids.includes(item.id)) : items.value;
});
function nameOf(item: (typeof store.fluctlights)[number]) { return String(item.identity.name ?? item.id); }
function timeOf(value?: string | null) { return value ? new Date(value).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" }) : ""; }
function sectionIcon(section: WorkspaceSection) {
  if (section === "model-role") return Bot;
  if (section === "endpoint") return Server;
  if (section === "binding") return Link2;
  if (section === "media") return Image;
  if (section === "operations") return Gauge;
  if (section === "owner") return ShieldCheck;
  if (section === "model-runs") return Activity;
  if (section === "workflows") return Workflow;
  return Activity;
}
</script>

<template>
  <aside class="desktop-context-panel">
    <template v-if="contextView === 'settings'">
      <header class="desktop-context-header settings-context-header"><div><p class="eyebrow">CONTROL CENTER</p><h2>设置</h2><small>选择要管理的配置</small></div></header>
      <nav class="context-section-list" aria-label="设置选项">
        <Button v-for="section in settingsSections" :key="section.id" class="context-section-link justify-normal" variant="ghost" :class="{ selected: props.activeSection === section.id }" type="button" @click="emit('navigateSection', 'settings', section.id)">
          <component :is="sectionIcon(section.id)" :size="17" :stroke-width="2" aria-hidden="true" />
          <span><strong>{{ section.label }}</strong><small>{{ section.description }}</small></span>
          <ChevronRight :size="16" :stroke-width="2" aria-hidden="true" />
        </Button>
      </nav>
    </template>
    <template v-else-if="contextView === 'diagnostics'">
      <header class="desktop-context-header settings-context-header"><div><p class="eyebrow">OBSERVABILITY</p><h2>诊断中心</h2><small>按主题查看运行记录</small></div></header>
      <nav class="context-section-list" aria-label="诊断选项">
        <Button v-for="section in diagnosticsSections" :key="section.id" class="context-section-link justify-normal" variant="ghost" :class="{ selected: props.activeSection === section.id }" type="button" @click="emit('navigateSection', 'diagnostics', section.id)">
          <component :is="sectionIcon(section.id)" :size="17" :stroke-width="2" aria-hidden="true" />
          <span><strong>{{ section.label }}</strong><small>{{ section.description }}</small></span>
          <ChevronRight :size="16" :stroke-width="2" aria-hidden="true" />
        </Button>
      </nav>
    </template>
    <template v-else>
      <header class="desktop-context-header"><div><p class="eyebrow">MESSAGES</p><h2>最近</h2><small>选择一个会话开始聊天</small></div><Button class="context-compose" variant="ghost" size="icon-lg" type="button" aria-label="新建 Fluctlight" @click="emit('create')"><Plus :size="18" :stroke-width="2" aria-hidden="true" /></Button></header>
      <label class="context-search" for="desktop-chat-search"><Search :size="14" :stroke-width="2" aria-hidden="true" /><span class="sr-only">搜索聊天</span><Input id="desktop-chat-search" v-model="chatSearch" type="search" placeholder="搜索" /></label>
      <div class="context-group-tabs" role="tablist" aria-label="聊天分组">
        <Button class="context-group-tab" variant="ghost" :class="{ selected: chatGroupId === null }" role="tab" :aria-selected="chatGroupId === null" type="button" @click="chatGroupId = null">最近</Button>
        <Button v-for="group in groups" :key="group.id" class="context-group-tab" variant="ghost" :class="{ selected: chatGroupId === group.id }" role="tab" :aria-selected="chatGroupId === group.id" type="button" @click="chatGroupId = group.id">{{ group.name }}</Button>
      </div>
      <div v-if="visibleItems.length" class="desktop-conversation-items"><Button v-for="item in visibleItems" :key="item.id" class="desktop-conversation-item justify-normal" variant="ghost" :class="{ selected: item.id === store.fluctlightId }" type="button" @click="emit('select', item.id)"><span class="avatar persona-avatar">{{ nameOf(item).slice(0, 1) }}</span><span class="desktop-conversation-copy"><strong>{{ nameOf(item) }}</strong><small><Badge variant="secondary" :class="{ paused: item.status === 'paused', muted: item.status === 'retired' }">{{ fluctlightStatusLabel(item.status) }}</Badge></small></span><span class="desktop-conversation-meta"><time>{{ timeOf(item.last_conversation_at) }}</time><b v-if="item.unread_count">{{ item.unread_count }}</b></span></Button></div>
      <div v-else class="desktop-list-empty">暂无符合条件的会话<br />可以切换分组或创建一个 Fluctlight。</div>
    </template>
    <BottomNav class="desktop-context-nav" :active-view="props.activeView" @navigate="emit('navigate', $event)" />
  </aside>
</template>

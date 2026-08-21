<script setup lang="ts">
import Avatar from '../Avatar.vue';
import type { ContactGroup, PersonaSummary, ViewName } from '../types';

withDefaults(defineProps<{
  personas?: PersonaSummary[];
  groups?: ContactGroup[];
  activePersonaId?: string | null;
  activeGroupId?: string | null;
  currentView?: ViewName;
  loading?: boolean;
}>(), { personas: () => [], groups: () => [], activePersonaId: null, activeGroupId: null, currentView: 'contacts', loading: false });

const emit = defineEmits<{
  (event: 'select-persona', id: string): void;
  (event: 'create'): void;
  (event: 'navigate', view: ViewName): void;
  (event: 'open-groups'): void;
}>();
</script>

<template>
  <aside class="sidebar" aria-label="联系人列表">
    <header class="sidebar-header">
      <div><strong>摇光（Fluctlight）</strong><small>FLUCTLIGHT</small></div>
      <button class="icon-button" type="button" aria-label="创建摇光实例" title="创建摇光实例" @click="emit('create')">＋</button>
    </header>
    <nav class="sidebar-tabs" aria-label="侧栏视图">
      <button type="button" :class="{ active: currentView === 'contacts' }" @click="emit('navigate', 'contacts')">联系人</button>
      <button type="button" :class="{ active: currentView === 'activity' }" @click="emit('navigate', 'activity')">动态</button>
    </nav>
    <div v-if="loading" class="persona-list persona-list--loading" aria-label="正在加载联系人">
      <span v-for="item in 4" :key="item" class="persona-skeleton"><i /><span><b /><small /></span></span>
    </div>
    <div v-else class="persona-list">
      <button v-for="persona in personas" :key="persona.id" class="persona-row" :class="{ selected: persona.id === activePersonaId }" type="button" :aria-current="persona.id === activePersonaId ? 'page' : undefined" @click="emit('select-persona', persona.id)">
        <Avatar :persona="persona" />
        <span class="persona-copy"><b>{{ persona.name }}</b><small>{{ persona.currentSituation || persona.role || '开始聊天' }}</small></span>
        <em v-if="persona.unreadCount">{{ persona.unreadCount > 99 ? '99+' : persona.unreadCount }}</em>
      </button>
      <div v-if="!personas.length" class="empty-list">还没有摇光实例</div>
    </div>
    <button class="new-persona" type="button" @click="emit('create')">＋ 创建一个摇光实例</button>
    <button v-if="groups.length" class="sidebar-group-button" type="button" @click="emit('open-groups')">
      <span>分组</span><small>{{ groups.find(group => group.id === activeGroupId)?.name || '所有联系人' }}</small><i>›</i>
    </button>
  </aside>
</template>


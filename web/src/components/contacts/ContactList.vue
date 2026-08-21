<script setup lang="ts">
import Avatar from '../Avatar.vue';
import type { ContactGroup, PersonaSummary } from '../types';

const props = withDefaults(defineProps<{
  personas?: PersonaSummary[];
  groups?: ContactGroup[];
  selectedGroupId?: string | null;
  loading?: boolean;
}>(), { personas: () => [], groups: () => [], selectedGroupId: null, loading: false });
const emit = defineEmits<{ (event: 'select', id: string): void; (event: 'create'): void; (event: 'select-group', id: string | null): void }>();

const selectedGroup = () => props.groups.find(group => group.id === props.selectedGroupId) || null;
const visiblePersonas = () => selectedGroup() ? props.personas.filter(persona => String(persona.groupId || '') === String(props.selectedGroupId)) : props.personas;
</script>

<template>
  <div class="contacts-content">
    <div v-if="loading" class="contacts-skeleton" aria-label="正在加载联系人">
      <span v-for="item in 5" :key="item" class="contact-skeleton"><i /><span><b /><small /></span></span>
    </div>
    <div v-else-if="visiblePersonas().length" class="contacts-stream">
      <button v-for="persona in visiblePersonas()" :key="persona.id" class="contact-row" type="button" @click="emit('select', persona.id)">
        <Avatar :persona="persona" />
        <span class="persona-copy"><b>{{ persona.name }}</b><small>{{ persona.currentSituation || persona.role || '开始聊天' }}</small></span>
        <em v-if="persona.unreadCount">{{ persona.unreadCount > 99 ? '99+' : persona.unreadCount }}</em>
        <i aria-hidden="true">›</i>
      </button>
    </div>
    <div v-else class="contact-empty">
      <p>{{ selectedGroup() ? `“${selectedGroup()?.name}”里还没有摇光实例。` : '还没有摇光实例。' }}</p>
      <button class="quiet" type="button" @click="emit('create')">创建一个摇光实例</button>
    </div>
  </div>
</template>


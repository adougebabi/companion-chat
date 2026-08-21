<script setup lang="ts">
import ContactList from '../components/contacts/ContactList.vue';
import type { ContactGroup, PersonaSummary } from '../components/types';

withDefaults(defineProps<{ personas?: PersonaSummary[]; groups?: ContactGroup[]; selectedGroupId?: string | null; loading?: boolean }>(), { personas: () => [], groups: () => [], selectedGroupId: null, loading: false });
const emit = defineEmits<{ (event: 'select-persona', id: string): void; (event: 'create'): void; (event: 'select-group', id: string | null): void }>();
</script>

<template>
  <section class="contacts-view">
    <header class="pane-header contacts-header">
      <button class="contacts-title" type="button" aria-label="选择联系人分组" @click="emit('select-group', selectedGroupId || null)">
        <span class="header-copy"><h1>联系人</h1><p>{{ selectedGroupId ? `${groups.find(group => group.id === selectedGroupId)?.name || '联系人分组'} · ${personas.filter(persona => persona.groupId === selectedGroupId).length} 位摇光实例` : '所有摇光实例的聊天' }}</p></span>
      </button>
      <button class="text-icon" type="button" aria-label="创建摇光实例" title="创建摇光实例" @click="emit('create')">＋</button>
    </header>
    <ContactList :personas="personas" :groups="groups" :selected-group-id="selectedGroupId" :loading="loading" @select="emit('select-persona', $event)" @create="emit('create')" @select-group="emit('select-group', $event)" />
  </section>
</template>


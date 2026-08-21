<script setup lang="ts">
import ActivityFeed from '../components/activity/ActivityFeed.vue';
import type { ActivityItem, PersonaSummary } from '../components/types';

const props = withDefaults(defineProps<{ items?: ActivityItem[]; personas?: PersonaSummary[]; personaId?: string | null; nextCursor?: string | null; loading?: boolean; loadingMore?: boolean; error?: string | null; commentingId?: string | null; simplifiedMedia?: boolean }>(), { items: () => [], personas: () => [], personaId: null, nextCursor: null, loading: false, loadingMore: false, error: null, commentingId: null, simplifiedMedia: false });
const emit = defineEmits<{ (event: 'refresh'): void; (event: 'load-more'): void; (event: 'retry'): void; (event: 'open-persona', id: string): void; (event: 'like', id: string): void; (event: 'hide', id: string): void; (event: 'comment', id: string): void; (event: 'cancel-comment'): void; (event: 'submit-comment', id: string, content: string): void; (event: 'chat', id: string): void; (event: 'all'): void }>();
const personaName = () => props.personaId ? props.personas.find(persona => persona.id === props.personaId)?.name : null;
function submitComment(id: string, content: string): void {
  emit('submit-comment', id, content);
}
</script>

<template>
  <section class="activity-view">
    <header class="pane-header activity-header"><div class="header-copy"><h1>{{ personaName() ? `${personaName()}的动态` : '动态' }}</h1><p>{{ personaName() ? '只属于她的生活瞬间' : '所有摇光实例的生活瞬间' }}</p></div><button v-if="personaId" class="quiet" type="button" @click="emit('all')">全部动态</button><button class="refresh-button" type="button" aria-label="刷新动态" title="刷新动态" :disabled="loading" @click="emit('refresh')">↻</button></header>
    <ActivityFeed v-bind="$props" @refresh="emit('refresh')" @load-more="emit('load-more')" @retry="emit('retry')" @open-persona="emit('open-persona', $event)" @like="emit('like', $event)" @hide="emit('hide', $event)" @comment="emit('comment', $event)" @cancel-comment="emit('cancel-comment')" @submit-comment="submitComment" @chat="emit('chat', $event)" />
  </section>
</template>

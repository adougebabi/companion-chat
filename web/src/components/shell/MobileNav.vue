<script setup lang="ts">
import type { ViewName } from '../types';

defineProps<{ currentView: ViewName; activityUnread?: boolean }>();
const emit = defineEmits<{ (event: 'navigate', view: ViewName): void }>();
</script>

<template>
  <nav class="mobile-nav" aria-label="主导航">
    <button type="button" :class="{ active: currentView === 'contacts' }" aria-label="联系人" title="联系人" @click="emit('navigate', 'contacts')">⌁</button>
    <button type="button" :class="{ active: currentView === 'activity' }" aria-label="动态" title="动态" @click="emit('navigate', 'activity')">◉<i v-if="activityUnread" class="unread-dot" aria-label="有未读动态" /></button>
    <button type="button" :class="{ active: currentView === 'settings' }" aria-label="设置" title="设置" @click="emit('navigate', 'settings')">⚙</button>
  </nav>
</template>


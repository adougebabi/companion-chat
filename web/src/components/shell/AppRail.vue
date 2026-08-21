<script setup lang="ts">
import type { ViewName } from '../types';

withDefaults(defineProps<{ currentView: ViewName; activityUnread?: boolean }>(), { activityUnread: false });
const emit = defineEmits<{ (event: 'navigate', view: ViewName): void; (event: 'brand'): void }>();
</script>

<template>
  <aside class="rail" aria-label="主导航">
    <button class="brand" type="button" aria-label="摇光首页" @click="emit('brand')">摇</button>
    <button class="rail-action" :class="{ active: currentView === 'contacts' }" type="button" aria-label="联系人" title="联系人" @click="emit('navigate', 'contacts')">⌁</button>
    <button class="rail-action" :class="{ active: currentView === 'activity' }" type="button" aria-label="动态" title="动态" @click="emit('navigate', 'activity')">
      ◉<i v-if="activityUnread" class="unread-dot" aria-label="有未读动态" />
    </button>
    <span class="rail-spacer" />
    <button class="rail-action" :class="{ active: currentView === 'settings' }" type="button" aria-label="设置" title="设置" @click="emit('navigate', 'settings')">⚙</button>
  </aside>
</template>

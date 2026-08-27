<script setup lang="ts">
import BottomNav from "./BottomNav.vue";
import type { WorkspaceView } from "../../app/navigation";

defineProps<{ activeView: WorkspaceView; showNavigation?: boolean }>();
const emit = defineEmits<{ navigate: [view: WorkspaceView] }>();
</script>

<template>
  <main class="app-shell" :class="{ 'chat-shell': activeView === 'chat' }">
    <slot />
    <BottomNav
      v-if="showNavigation !== false && activeView !== 'chat'"
      :active-view="activeView"
      @navigate="emit('navigate', $event)"
    />
  </main>
</template>

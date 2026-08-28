<script setup lang="ts">
import BottomNav from "./BottomNav.vue";
import DesktopContextPanel from "./DesktopContextPanel.vue";
import type { WorkspaceView } from "../../app/navigation";

defineProps<{ activeView: WorkspaceView; showNavigation?: boolean }>();
const emit = defineEmits<{ navigate: [view: WorkspaceView]; selectInstance: [fluctlightId: string]; create: [] }>();
</script>

<template>
  <main class="app-shell" :class="{ 'chat-shell': activeView === 'chat', 'desktop-workspace': showNavigation }">
    <DesktopContextPanel v-if="showNavigation" :active-view="activeView" @select="emit('selectInstance', $event)" @navigate="emit('navigate', $event)" @create="emit('create')" />
    <section class="app-main-pane"><slot /></section>
    <BottomNav
      v-if="showNavigation !== false && activeView !== 'chat'"
      :active-view="activeView"
      @navigate="emit('navigate', $event)"
    />
  </main>
</template>

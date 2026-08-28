<script setup lang="ts">
import BottomNav from "./BottomNav.vue";
import DesktopContextPanel from "./DesktopContextPanel.vue";
import type { WorkspaceSection, WorkspaceView } from "../../app/navigation";

const props = defineProps<{ activeView: WorkspaceView; activeSection?: WorkspaceSection | null; showNavigation?: boolean }>();
const emit = defineEmits<{ navigate: [view: WorkspaceView]; navigateSection: [view: "settings" | "diagnostics", section: WorkspaceSection | null]; selectInstance: [fluctlightId: string]; create: [] }>();
</script>

<template>
  <main class="app-shell" :class="{ 'chat-shell': props.activeView === 'chat', 'desktop-workspace': props.showNavigation }">
    <DesktopContextPanel v-if="props.showNavigation" :active-view="props.activeView" :active-section="props.activeSection" @select="emit('selectInstance', $event)" @navigate="emit('navigate', $event)" @navigate-section="(view, section) => emit('navigateSection', view, section)" @create="emit('create')" />
    <section class="app-main-pane"><slot /></section>
    <BottomNav
      v-if="props.showNavigation !== false && props.activeView !== 'chat'"
      :active-view="props.activeView"
      @navigate="emit('navigate', $event)"
    />
  </main>
</template>

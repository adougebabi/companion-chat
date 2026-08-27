<script setup lang="ts">
import { computed } from "vue";
import { primaryNavigation, type WorkspaceView } from "../../app/navigation";

const props = defineProps<{ activeView: WorkspaceView }>();
const emit = defineEmits<{ navigate: [view: WorkspaceView] }>();
const activeNavigationView = computed<WorkspaceView>(() => props.activeView === "diagnostics" ? "settings" : props.activeView);
</script>

<template>
  <nav class="bottom-nav" aria-label="Fluctlight 主导航">
    <button
      v-for="item in primaryNavigation"
      :key="item.id"
      class="bottom-nav-item"
      :class="{ selected: activeNavigationView === item.id }"
      type="button"
      :aria-current="activeNavigationView === item.id ? 'page' : undefined"
      @click="emit('navigate', item.id)"
    >
      <span class="bottom-nav-icon" aria-hidden="true">{{ item.icon }}</span>
      <span>{{ item.label }}</span>
    </button>
  </nav>
</template>

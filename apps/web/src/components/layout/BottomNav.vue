<script setup lang="ts">
import { primaryNavigation, type WorkspaceView } from "../../app/navigation";

defineProps<{ activeView: WorkspaceView }>();
const emit = defineEmits<{ navigate: [view: WorkspaceView] }>();
</script>

<template>
  <nav class="bottom-nav" aria-label="Fluctlight 主导航">
    <button
      v-for="item in primaryNavigation"
      :key="item.id"
      class="bottom-nav-item"
      :class="{ selected: activeView === item.id }"
      type="button"
      :aria-current="activeView === item.id ? 'page' : undefined"
      @click="emit('navigate', item.id)"
    >
      <span class="bottom-nav-icon" aria-hidden="true">{{ item.icon }}</span>
      <span>{{ item.label }}</span>
    </button>
  </nav>
</template>

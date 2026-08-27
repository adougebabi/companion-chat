<script setup lang="ts">
import { computed } from "vue";

import { primaryNavigation, type WorkspaceView } from "../../app/navigation";

const props = defineProps<{ activeView: WorkspaceView }>();
const emit = defineEmits<{ navigate: [view: WorkspaceView] }>();
const activeView = computed(() => props.activeView === "diagnostics" ? "settings" : props.activeView);
</script>

<template>
  <aside class="desktop-rail" aria-label="工作区导航">
    <div class="desktop-brand"><span class="desktop-brand-mark">F</span><div><strong>FLUCTLIGHT</strong><small>PERSONAL WORKSPACE</small></div></div>
    <nav class="desktop-rail-nav">
      <button v-for="item in primaryNavigation" :key="item.id" type="button" :class="{ selected: activeView === item.id }" :aria-current="activeView === item.id ? 'page' : undefined" @click="emit('navigate', item.id)"><span aria-hidden="true">{{ item.icon }}</span>{{ item.label }}</button>
    </nav>
    <div class="desktop-rail-footer"><span class="rail-status-dot" />本地工作区</div>
  </aside>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { Activity, MessageCircle, Settings, Stethoscope } from "@lucide/vue";

import Button from "@/components/ui/button/Button.vue";
import { primaryNavigation, type WorkspaceView } from "../../app/navigation";

const props = defineProps<{ activeView: WorkspaceView }>();
const emit = defineEmits<{ navigate: [view: WorkspaceView] }>();
const activeNavigationView = computed<WorkspaceView>(() => props.activeView === "chat" ? "instances" : props.activeView);
const navigationIcons = {
  instances: MessageCircle,
  moments: Activity,
  settings: Settings,
  diagnostics: Stethoscope,
} as const;
</script>

<template>
  <nav class="bottom-nav" aria-label="Fluctlight 主导航">
    <Button
      v-for="item in primaryNavigation"
      :key="item.id"
      class="bottom-nav-item"
      variant="ghost"
      :class="{ selected: activeNavigationView === item.id }"
      type="button"
      :aria-current="activeNavigationView === item.id ? 'page' : undefined"
      @click="emit('navigate', item.id)"
    >
      <component :is="navigationIcons[item.id]" class="bottom-nav-icon" :size="18" :stroke-width="2" aria-hidden="true" />
      <span>{{ item.label }}</span>
    </Button>
  </nav>
</template>

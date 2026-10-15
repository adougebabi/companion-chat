<script setup lang="ts">
import { computed } from "vue";
import { Menu } from "@lucide/vue";

import Button from "@/components/ui/button/Button.vue";
import { mobileNavigation, type LifeSection, type WorkspaceView } from "../../app/navigation";

const props = defineProps<{ activeView: WorkspaceView; lifeSection?: LifeSection | null }>();
const emit = defineEmits<{ navigate: [view: WorkspaceView, section?: string]; openMenu: [] }>();
const activeNavigationView = computed<WorkspaceView>(() => props.activeView === "showcase" ? "chat" : props.activeView);
</script>

<template>
  <nav class="bottom-nav bottombar" aria-label="Fluctlight 主导航">
    <Button
      v-for="item in mobileNavigation"
      :key="item.key"
      class="bottom-nav-item"
      variant="ghost"
      :class="{ on: activeNavigationView === item.view && (!item.life || item.life === lifeSection), selected: activeNavigationView === item.view && (!item.life || item.life === lifeSection) }"
      type="button"
      :aria-current="activeNavigationView === item.view && (!item.life || item.life === lifeSection) ? 'page' : undefined"
      @click="emit('navigate', item.view, item.life)"
    >
      <span>{{ item.label }}</span>
    </Button>
    <Button class="bottom-nav-item" variant="ghost" type="button" data-test="nav-drawer" @click="emit('openMenu')"><Menu :size="16" aria-hidden="true" /><span>全部</span></Button>
  </nav>
</template>

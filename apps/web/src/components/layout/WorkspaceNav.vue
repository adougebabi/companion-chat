<script setup lang="ts">
import { ChevronRight } from "@lucide/vue";

import { globalNavigation, lifeSections, workspaceNavigation, type LifeSection, type WorkspaceView } from "../../app/navigation";

const props = defineProps<{ activeView: WorkspaceView; lifeSection?: LifeSection | null; characterName?: string }>();
const emit = defineEmits<{ navigate: [view: WorkspaceView, section?: string] }>();

function current(view: WorkspaceView) {
  return props.activeView === view;
}
</script>
<template>
  <div class="workspace-nav">
    <div class="sec">角色工作区<span v-if="characterName"> · {{ characterName }}</span></div>
    <template v-for="item in workspaceNavigation" :key="item.key">
      <button class="navlink" :class="{ on: current(item.view) }" type="button" @click="emit('navigate', item.view, item.view === 'life' ? (lifeSection ?? 'now') : undefined)">
        {{ item.label }}
      </button>
      <template v-if="item.view === 'life'">
        <button v-for="section in lifeSections" :key="section.id" class="navlink sub" :class="{ on: activeView === 'life' && lifeSection === section.id }" type="button" @click="emit('navigate', 'life', section.id)">{{ section.label }}</button>
      </template>
    </template>
    <div class="sec">全局</div>
    <button v-for="item in globalNavigation" :key="item.key" class="navlink" :class="{ on: current(item.view) }" type="button" @click="emit('navigate', item.view)">
      {{ item.label }}<ChevronRight class="nav-chevron" :size="14" aria-hidden="true" />
    </button>
  </div>
</template>

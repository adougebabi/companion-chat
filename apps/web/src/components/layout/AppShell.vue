<script setup lang="ts">
import { computed, ref } from "vue";
import { Bell, LogOut } from "@lucide/vue";

import { fluctlightStatusLabel } from "../../lib/fluctlight-status";
import { useConversationStore } from "../../stores/conversations";
import { useControlCenterStore } from "../../stores/control-center";
import type { LifeSection, WorkspaceSection, WorkspaceView } from "../../app/navigation";
import BottomNav from "./BottomNav.vue";
import DesktopContextPanel from "./DesktopContextPanel.vue";
import WorkspaceNav from "./WorkspaceNav.vue";

const props = defineProps<{ activeView: WorkspaceView; activeSection?: WorkspaceSection | null; lifeSection?: LifeSection | null; showNavigation?: boolean }>();
const emit = defineEmits<{
  navigate: [view: WorkspaceView, section?: string];
  navigateSection: [view: "settings" | "diagnostics", section: WorkspaceSection | null];
  selectInstance: [fluctlightId: string];
  logout: [];
}>();

const store = useConversationStore();
const controlCenter = useControlCenterStore();
const menuOpen = ref(false);
const switcherOpen = ref(false);
const characterName = computed(() => store.selectedFluctlightName ?? "选择角色");
const groups = computed(() => [...controlCenter.actorGroups].sort((left, right) => { if (left.name === "默认") return -1; if (right.name === "默认") return 1; return left.name.localeCompare(right.name, "zh-CN"); }));

function charactersIn(groupId: string) {
  const group = groups.value.find((item) => item.id === groupId);
  return store.fluctlights.filter((item) => group?.actor_ids.includes(item.id));
}
const ungrouped = computed(() => {
  const assigned = new Set(groups.value.flatMap((group) => group.actor_ids));
  return store.fluctlights.filter((item) => !assigned.has(item.id));
});

function choose(id: string) {
  switcherOpen.value = false;
  emit("selectInstance", id);
}
function go(view: WorkspaceView, section?: string) {
  menuOpen.value = false;
  emit("navigate", view, section);
}
</script>

<template>
  <div class="sage-shell" :class="{ 'chat-active': props.activeView === 'chat' || props.activeView === 'showcase', 'has-page-pad': props.activeView !== 'chat', 'sage-guest': props.showNavigation === false }">
    <header v-if="props.showNavigation" class="topbar">
      <button class="brand" type="button" @click="go('instances')"><span class="brand-dot" aria-hidden="true" /><span class="hide-n">摇光</span></button>
      <button class="charsw" type="button" data-test="charsw" :aria-expanded="switcherOpen" @click="switcherOpen = !switcherOpen">
        <span class="av" aria-hidden="true">{{ characterName.slice(0, 1) }}</span>
        <b>{{ characterName }}</b>
      </button>
      <div v-if="switcherOpen" class="popmenu char-menu" role="menu">
        <template v-for="group in groups" :key="group.id">
          <p class="sec">{{ group.name }}</p>
          <button v-for="item in charactersIn(group.id)" :key="item.id" type="button" :data-test="'switch-' + item.id" @click="choose(item.id)">
            <span class="av sm" aria-hidden="true">{{ String(item.identity.name ?? item.id).slice(0, 1) }}</span>
            <span class="grow">{{ item.identity.name ?? item.id }}</span>
            <span class="badge b-plain">{{ fluctlightStatusLabel(item.status) }}</span>
          </button>
        </template>
        <button v-for="item in ungrouped" :key="item.id" type="button" @click="choose(item.id)">
          <span class="av sm" aria-hidden="true">{{ String(item.identity.name ?? item.id).slice(0, 1) }}</span>
          <span class="grow">{{ item.identity.name ?? item.id }}</span>
          <span class="badge b-plain">{{ fluctlightStatusLabel(item.status) }}</span>
        </button>
        <button type="button" @click="go('instances')">角色库</button>
      </div>
      <div class="scopebar">
        <span class="badge b-角色范围">角色范围</span>
        <span class="hide-n">{{ characterName }}<template v-if="store.selectedFluctlight"> · {{ fluctlightStatusLabel(store.selectedFluctlight.status) }}</template></span>
      </div>
      <span class="spacer"></span>
      <button class="btn ghost icon-only bell" type="button" aria-label="需要你处理" data-test="bell" @click="go('pending')"><Bell :size="18" aria-hidden="true" /></button>
      <button class="btn ghost sm" type="button" @click="emit('logout')"><LogOut :size="16" aria-hidden="true" /><span class="hide-n">退出</span></button>
    </header>
    <div class="layout">
      <DesktopContextPanel v-if="props.showNavigation" :active-view="props.activeView" :active-section="props.activeSection" :life-section="props.lifeSection" @navigate="go" @navigate-section="(view, section) => emit('navigateSection', view, section)" />
      <section class="main app-main-pane"><slot /></section>
    </div>
    <BottomNav v-if="props.showNavigation" :active-view="props.activeView" :life-section="props.lifeSection" @navigate="go" @open-menu="menuOpen = true" />
    <template v-if="menuOpen">
      <div class="scrim" @click="menuOpen = false" />
      <aside class="drawer" role="dialog" aria-label="全部导航">
        <header><h2>导航</h2><button class="btn ghost sm" type="button" @click="menuOpen = false">关闭</button></header>
        <div class="body"><WorkspaceNav :active-view="props.activeView" :life-section="props.lifeSection" :character-name="characterName" @navigate="go" /></div>
      </aside>
    </template>
  </div>
</template>

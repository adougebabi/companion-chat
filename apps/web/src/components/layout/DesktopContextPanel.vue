<script setup lang="ts">
import { computed } from "vue";
import { Activity, Bot, ChevronRight, Gauge, Image, Link2, Server, ShieldCheck, Workflow } from "@lucide/vue";

import Button from "@/components/ui/button/Button.vue";
import { diagnosticsSections, settingsSections, type LifeSection, type WorkspaceSection, type WorkspaceView } from "../../app/navigation";
import { useConversationStore } from "../../stores/conversations";
import { useControlCenterStore } from "../../stores/control-center";
import WorkspaceNav from "./WorkspaceNav.vue";

const props = defineProps<{ activeView: WorkspaceView; activeSection?: WorkspaceSection | null; lifeSection?: LifeSection | null }>();
const emit = defineEmits<{ navigate: [view: WorkspaceView, section?: string]; navigateSection: [view: "settings" | "diagnostics", section: WorkspaceSection | null] }>();
const store = useConversationStore();
const controlCenter = useControlCenterStore();
const chatGroupId = computed(() => controlCenter.selectedActorGroupId || null);
const characterName = computed(() => store.selectedFluctlightName ?? "");

function sectionIcon(section: WorkspaceSection) {
  if (section === "model-role") return Bot;
  if (section === "endpoint") return Server;
  if (section === "binding") return Link2;
  if (section === "media") return Image;
  if (section === "operations") return Gauge;
  if (section === "owner") return ShieldCheck;
  if (section === "model-runs") return Activity;
  if (section === "media-prompts") return Image;
  if (section === "workflows") return Workflow;
  return Activity;
}
</script>

<template>
  <aside class="desktop-context-panel sidenav" aria-label="角色工作区导航">
    <template v-if="activeView === 'settings'">
      <header class="desktop-context-header settings-context-header"><div><p class="eyebrow">系统设置</p><h2>设置</h2><small>选择要管理的配置。保存会写入当前服务。</small></div></header>
      <nav class="context-section-list" aria-label="设置选项">
        <Button v-for="section in settingsSections" :key="section.id" class="context-section-link navlink justify-normal whitespace-normal h-auto" variant="ghost" :class="{ selected: props.activeSection === section.id, on: props.activeSection === section.id }" type="button" @click="emit('navigateSection', 'settings', section.id)">
          <component :is="sectionIcon(section.id)" :size="17" :stroke-width="2" aria-hidden="true" />
          <span><strong>{{ section.label }}</strong><small>{{ section.description }}</small></span>
          <ChevronRight :size="16" :stroke-width="2" aria-hidden="true" />
        </Button>
      </nav>
    </template>
    <template v-else-if="activeView === 'diagnostics'">
      <header class="desktop-context-header settings-context-header"><div><p class="eyebrow">运行诊断</p><h2>诊断中心</h2><small>按主题查看运行记录</small></div></header>
      <nav class="context-section-list" aria-label="诊断选项">
        <Button v-for="section in diagnosticsSections" :key="section.id" class="context-section-link navlink justify-normal whitespace-normal h-auto" variant="ghost" :class="{ selected: props.activeSection === section.id, on: props.activeSection === section.id }" type="button" @click="emit('navigateSection', 'diagnostics', section.id)">
          <component :is="sectionIcon(section.id)" :size="17" :stroke-width="2" aria-hidden="true" />
          <span><strong>{{ section.label }}</strong><small>{{ section.description }}</small></span>
          <ChevronRight :size="16" :stroke-width="2" aria-hidden="true" />
        </Button>
      </nav>
    </template>
    <WorkspaceNav v-else :active-view="activeView" :life-section="lifeSection" :character-name="characterName ?? undefined" @navigate="(view, section) => emit('navigate', view, section)" />
    <div class="context-group-tabs" role="tablist" aria-label="聊天分组">
      <span class="sec">当前分组</span>
      <span class="context-group-tab selected">{{ controlCenter.actorGroups.find((group) => group.id === chatGroupId)?.name ?? "全部角色" }}</span>
    </div>
  </aside>
</template>

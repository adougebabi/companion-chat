<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch, defineAsyncComponent } from "vue";

import { isDiagnosticsSection, isLifeSection, isSettingsSection, type DiagnosticsSection, type LifeSection, type SettingsSection, type WorkspaceSection, type WorkspaceView } from "./app/navigation";
import AuthPanel from "./components/auth/AuthPanel.vue";
import AppShell from "./components/layout/AppShell.vue";
import InstanceDetailsDialog from "./components/instances/InstanceDetailsDialog.vue";
import ChatView from "./views/ChatView.vue";
import ChatShowcaseView from "./views/ChatShowcaseView.vue";
import InstancesView from "./views/InstancesView.vue";
import MomentsView from "./views/MomentsView.vue";
import UnwiredView from "./views/UnwiredView.vue";
import LifeShowcaseView from "./views/LifeShowcaseView.vue";
import GrowthView from "./views/life/GrowthView.vue";
import LifeGoalsView from "./views/life/LifeGoalsView.vue";
import LifeNowView from "./views/life/LifeNowView.vue";
import LifeScheduleView from "./views/life/LifeScheduleView.vue";
import LifeWardrobeView from "./views/life/LifeWardrobeView.vue";
import MediaView from "./views/life/MediaView.vue";
import PeopleView from "./views/life/PeopleView.vue";
import ProfileView from "./views/life/ProfileView.vue";
import WorkspaceState from "./components/product/WorkspaceState.vue";
import { useConversationStore } from "./stores/conversations";
import { useControlCenterStore } from "./stores/control-center";

const DiagnosticsView = defineAsyncComponent({ loader: () => import("./views/DiagnosticsView.vue"), loadingComponent: WorkspaceState, errorComponent: WorkspaceState, delay: 100, timeout: 15000 });
const SettingsView = defineAsyncComponent({ loader: () => import("./views/SettingsView.vue"), loadingComponent: WorkspaceState, errorComponent: WorkspaceState, delay: 100, timeout: 15000 });

const store = useConversationStore();
const controlCenter = useControlCenterStore();
function characterIdFromLocation(): string | null {
  const match = window.location.pathname.match(/^\/c\/([^/]+)(?:\/|$)/);
  return match ? decodeURIComponent(match[1]) : null;
}

function lifeFromLocation(): LifeSection {
  const match = window.location.pathname.match(/\/life\/(now|schedule|goals|wardrobe|diary)\/?$/);
  return match && isLifeSection(match[1]) ? match[1] : "now";
}

function viewFromLocation(): WorkspaceView {
  const path = window.location.pathname;
  if (path.startsWith("/dev/chat-components")) return "showcase";
  if (path.startsWith("/dev/life-components")) return "lifeShowcase";
  if (path.startsWith("/chat") || /\/chat\/?$/.test(path)) return "chat";
  if (path.startsWith("/moments") || path.startsWith("/feed")) return "moments";
  if (path.startsWith("/diagnostics") || path.startsWith("/settings/diagnostics")) return "diagnostics";
  if (path.startsWith("/settings")) return "settings";
  if (path.startsWith("/studio")) return "studio";
  if (path.startsWith("/inbox")) return "inbox";
  if (/\/life(\/|$)/.test(path)) return "life";
  if (/\/world(\/|$)/.test(path)) return "world";
  if (/\/people(\/|$)/.test(path)) return "people";
  if (/\/growth(\/|$)/.test(path)) return "growth";
  if (/\/media(\/|$)/.test(path)) return "media";
  if (/\/profile(\/|$)/.test(path)) return "profile";
  return "instances";
}

function sectionFromLocation(view: WorkspaceView): WorkspaceSection | null {
  const params = new URLSearchParams(window.location.search);
  const section = params.get("section");
  if (!section && view === "diagnostics" && params.get("correlation_id")) return "model-runs";
  if (!section) return null;
  if (view === "settings" && isSettingsSection(section)) return section as SettingsSection;
  if (view === "diagnostics" && isDiagnosticsSection(section)) return section as DiagnosticsSection;
  return null;
}

function syncDiagnosticsFilterFromLocation() {
  if (window.location.pathname.startsWith("/diagnostics") || window.location.pathname.startsWith("/settings/diagnostics")) {
    controlCenter.diagnosticsCorrelationFilter = new URLSearchParams(window.location.search).get("correlation_id") ?? "";
  }
}

function pathForView(next: WorkspaceView, correlationId = "", section = "", fluctlightId = "") {
  const characterId = fluctlightId || characterIdFromLocation() || store.fluctlightId || "";
  const scoped = (suffix: string) => characterId ? `/c/${encodeURIComponent(characterId)}/${suffix}` : "/library";
  const path = next === "chat" ? (characterId ? scoped("chat") : "/chat")
    : next === "moments" ? "/feed"
    : next === "diagnostics" ? "/diagnostics"
    : next === "settings" ? "/settings"
    : next === "studio" ? "/studio"
    : next === "inbox" ? "/inbox"
    : next === "life" ? scoped(`life/${isLifeSection(section) ? section : "now"}`)
    : next === "world" ? scoped("world")
    : next === "people" ? scoped("people")
    : next === "growth" ? scoped("growth")
    : next === "media" ? scoped("media")
    : next === "profile" ? scoped("profile")
    : next === "showcase" ? "/dev/chat-components"
    : next === "lifeShowcase" ? "/dev/life-components"
    : "/library";
  const params = new URLSearchParams();
  if (next === "diagnostics" && correlationId.trim()) params.set("correlation_id", correlationId.trim());
  if ((next === "settings" || next === "diagnostics") && section.trim()) params.set("section", section.trim());
  const query = params.toString();
  return query ? `${path}?${query}` : path;
}

const view = ref<WorkspaceView>(viewFromLocation());
const lifeSection = ref<LifeSection>(lifeFromLocation());
const activeSection = ref<WorkspaceSection | null>(sectionFromLocation(view.value));
const routeCharacterMissing = ref(false);
const showDetails = ref(false);
const governanceRequest = ref(false);
const createRequest = ref(0);
const activeSettingsSection = computed<SettingsSection | null>(() => view.value === "settings" && activeSection.value && isSettingsSection(activeSection.value) ? activeSection.value as SettingsSection : null);
const activeDiagnosticsSection = computed<DiagnosticsSection | null>(() => view.value === "diagnostics" && activeSection.value && isDiagnosticsSection(activeSection.value) ? activeSection.value as DiagnosticsSection : null);
const activeViewLabel = computed(() => ({ chat: "对话", moments: "动态", instances: "角色库", diagnostics: "运行诊断", settings: "系统设置", studio: "创作中心", inbox: "通知", life: "生活", world: "世界", people: "人物关系", growth: "记忆成长", media: "媒体", profile: "角色资料", showcase: "组件场景", lifeShowcase: "生活组件场景" })[view.value]);

async function navigate(next: WorkspaceView, correlationId = "", section = "") {
  view.value = next;
  if (next === "life" && isLifeSection(section)) lifeSection.value = section;
  if (next === "life" && !section) lifeSection.value = lifeFromLocation();
  activeSection.value = next === "diagnostics" && correlationId.trim() ? "model-runs" : null;
  if (next === "diagnostics") controlCenter.diagnosticsCorrelationFilter = correlationId;
  const path = pathForView(next, correlationId, next === "life" ? lifeSection.value : activeSection.value ?? section);
  if (`${window.location.pathname}${window.location.search}` !== path) window.history.pushState({ view: next }, "", path);
  if (next === "instances") await controlCenter.loadActorGroups();
}

function onShellNavigate(next: WorkspaceView, section = "") {
  if ((next === "settings" || next === "diagnostics") && section && (isSettingsSection(section) || isDiagnosticsSection(section))) {
    navigateSection(next, section);
    return;
  }
  void navigate(next, next === "diagnostics" ? controlCenter.diagnosticsCorrelationFilter : "", section);
}

function navigateSection(next: "settings" | "diagnostics", section: WorkspaceSection | null) {
  view.value = next;
  activeSection.value = section;
  const correlationId = next === "diagnostics" ? controlCenter.diagnosticsCorrelationFilter : "";
  const path = pathForView(next, correlationId, section ?? "");
  if (`${window.location.pathname}${window.location.search}` !== path) window.history.pushState({ view: next, section }, "", path);
}

function onPopState() {
  view.value = viewFromLocation();
  lifeSection.value = lifeFromLocation();
  activeSection.value = sectionFromLocation(view.value);
  syncDiagnosticsFilterFromLocation();
  void syncCharacterFromLocation();
}

async function syncCharacterFromLocation() {
  if (view.value === "showcase" || store.authenticated !== true) return;
  const characterId = characterIdFromLocation();
  if (!characterId) {
    routeCharacterMissing.value = false;
    return;
  }
  if (!store.fluctlights.some((item) => item.id === characterId)) {
    routeCharacterMissing.value = true;
    return;
  }
  routeCharacterMissing.value = false;
  if (store.fluctlightId !== characterId) await store.selectFluctlight(characterId);
}

async function openDetails() {
  if (!store.fluctlightId) return;
  showDetails.value = true;
  await Promise.all([
    controlCenter.loadFluctlightDetail(store.fluctlightId),
    controlCenter.loadAutonomyActions(store.fluctlightId),
  ]);
}

async function openDesktopChat(fluctlightId: string) {
  await store.selectFluctlight(fluctlightId);
  await navigate("chat");
}

async function openCreateDialog() {
  createRequest.value += 1;
  if (view.value !== "instances") await navigate("instances");
}

async function openGovernance() {
  showDetails.value = false;
  governanceRequest.value = true;
  await navigate("instances");
  await controlCenter.loadCapabilityRequests();
  await nextTick();
  governanceRequest.value = false;
}

async function handleSetup(token: string, password: string) {
  await store.setup(token, password);
  if (store.authenticated) await controlCenter.ensureDefaultGroup(store.fluctlights.map((item) => item.id));
}

async function handleSignIn(password: string) {
  await store.login(password);
  if (store.authenticated) await controlCenter.ensureDefaultGroup(store.fluctlights.map((item) => item.id));
}

async function initializeWorkspace() {
  await store.initialize();
  if (store.authenticated) {
    await controlCenter.ensureDefaultGroup(store.fluctlights.map((item) => item.id));
    await syncCharacterFromLocation();
  }
}

onMounted(() => {
  window.addEventListener("popstate", onPopState);
  syncDiagnosticsFilterFromLocation();
  void initializeWorkspace();
});
onBeforeUnmount(() => window.removeEventListener("popstate", onPopState));
</script>

<template>
  <ChatShowcaseView v-if="view === 'showcase'" />
  <LifeShowcaseView v-else-if="view === 'lifeShowcase'" />
  <AppShell v-else :active-view="view" :active-section="activeSection" :life-section="lifeSection" :show-navigation="store.authenticated === true" @navigate="onShellNavigate" @navigate-section="navigateSection" @select-instance="openDesktopChat" @logout="store.logout">
    <AuthPanel
      v-if="store.authenticated !== true"
      :setup-available="store.setupAvailable"
      :loading="store.authLoading"
      :error="store.authError"
      @setup="handleSetup"
      @sign-in="handleSignIn"
    />

    <template v-else>
      <div v-if="routeCharacterMissing" class="banner err">链接中的角色不在当前目录，已停止显示其他角色的数据。</div>
      <ChatView v-if="view === 'chat'" @back="navigate('instances')" @open-details="openDetails" @open-instances="navigate('instances')" @open-section="(next) => navigate(next, '', next === 'life' ? 'now' : '')" />
      <MomentsView v-else-if="view === 'moments'" />
      <InstancesView v-else-if="view === 'instances'" :open-governance="governanceRequest" :open-create="createRequest" @creation-consumed="createRequest=0" @open-chat="navigate('chat')" @open-details="openDetails" @open-diagnostics="(correlationId) => navigate('diagnostics', correlationId)" />
      <DiagnosticsView v-else-if="view === 'diagnostics'" :section="activeDiagnosticsSection" @navigate-section="(section) => navigateSection('diagnostics', section)" />
      <SettingsView v-else-if="view === 'settings'" :section="activeSettingsSection" @navigate-section="(section) => navigateSection('settings', section)" @logout="store.logout" />
      <UnwiredView v-else-if="view === 'studio'" title="创作中心" bp="BP-15" note="资源编辑、试生成沙盒和应用到实例还没有接到现有接口。这里不会写入创作草稿或生活事实。" />
      <UnwiredView v-else-if="view === 'inbox'" title="通知中心" bp="BP-03" note="通知列表还没有服务接口。铃铛不会显示虚构未读数。动态仍可从导航进入。" action-label="打开动态" @action="navigate('moments')" />
      <LifeNowView v-else-if="view === 'life' && lifeSection === 'now' && !routeCharacterMissing" @open="onShellNavigate" />
      <LifeScheduleView v-else-if="view === 'life' && lifeSection === 'schedule' && !routeCharacterMissing" />
      <LifeGoalsView v-else-if="view === 'life' && lifeSection === 'goals' && !routeCharacterMissing" />
      <LifeWardrobeView v-else-if="view === 'life' && lifeSection === 'wardrobe' && !routeCharacterMissing" />
      <UnwiredView v-else-if="view === 'life' && lifeSection === 'diary'" title="日记" bp="BP-09" note="日记与共同经历还没有接到现有接口。" />
      <UnwiredView v-else-if="view === 'world'" title="世界" bp="BP-10" note="世界、地点和场景页面尚未接入。浏览这里不会移动角色。" />
      <PeopleView v-else-if="view === 'people' && !routeCharacterMissing" />
      <GrowthView v-else-if="view === 'growth' && !routeCharacterMissing" />
      <MediaView v-else-if="view === 'media' && !routeCharacterMissing" @open-settings="navigateSection('settings', 'media')" />
      <ProfileView v-else-if="view === 'profile' && !routeCharacterMissing" @open-media="navigate('media')" />
      <p v-else-if="routeCharacterMissing" class="statebox">已停止显示其他角色的数据。</p>
    </template>

    <InstanceDetailsDialog :open="showDetails" @close="showDetails = false" @manage="openGovernance" />
    <span class="sr-only" aria-live="polite">当前页面：{{ activeViewLabel }}</span>
  </AppShell>
</template>

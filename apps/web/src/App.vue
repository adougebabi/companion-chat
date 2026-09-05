<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";

import type { DiagnosticsSection, SettingsSection, WorkspaceSection, WorkspaceView } from "./app/navigation";
import AuthPanel from "./components/auth/AuthPanel.vue";
import AppShell from "./components/layout/AppShell.vue";
import InstanceDetailsDialog from "./components/instances/InstanceDetailsDialog.vue";
import ChatView from "./views/ChatView.vue";
import DiagnosticsView from "./views/DiagnosticsView.vue";
import InstancesView from "./views/InstancesView.vue";
import MomentsView from "./views/MomentsView.vue";
import SettingsView from "./views/SettingsView.vue";
import { useConversationStore } from "./stores/conversations";
import { useControlCenterStore } from "./stores/control-center";

const store = useConversationStore();
const controlCenter = useControlCenterStore();
function viewFromLocation(): WorkspaceView {
  if (window.location.pathname.startsWith("/chat")) return "chat";
  if (window.location.pathname.startsWith("/moments")) return "moments";
  if (window.location.pathname.startsWith("/settings/diagnostics")) return "diagnostics";
  if (window.location.pathname.startsWith("/settings")) return "settings";
  return "instances";
}

function sectionFromLocation(view: WorkspaceView): WorkspaceSection | null {
  const params = new URLSearchParams(window.location.search);
  const section = params.get("section");
  if (!section && view === "diagnostics" && params.get("correlation_id")) return "model-runs";
  if (!section) return null;
  if (view === "settings" && ["model-role", "endpoint", "binding", "media", "operations", "owner"].includes(section)) return section as SettingsSection;
  if (view === "diagnostics" && ["model-runs", "media-prompts", "events", "workflows"].includes(section)) return section as DiagnosticsSection;
  return null;
}

function syncDiagnosticsFilterFromLocation() {
  if (window.location.pathname.startsWith("/settings/diagnostics")) {
    controlCenter.diagnosticsCorrelationFilter = new URLSearchParams(window.location.search).get("correlation_id") ?? "";
  }
}

function pathForView(next: WorkspaceView, correlationId = "", section = "") {
  const path = next === "chat" ? "/chat" : next === "moments" ? "/moments" : next === "diagnostics" ? "/settings/diagnostics" : next === "settings" ? "/settings" : "/instances";
  const params = new URLSearchParams();
  if (next === "diagnostics" && correlationId.trim()) params.set("correlation_id", correlationId.trim());
  if ((next === "settings" || next === "diagnostics") && section.trim()) params.set("section", section.trim());
  const query = params.toString();
  return query ? `${path}?${query}` : path;
}

const view = ref<WorkspaceView>(viewFromLocation());
const activeSection = ref<WorkspaceSection | null>(sectionFromLocation(view.value));
const showDetails = ref(false);
const governanceRequest = ref(false);
const createRequest = ref(0);
const activeSettingsSection = computed<SettingsSection | null>(() => view.value === "settings" && activeSection.value && ["model-role", "endpoint", "binding", "media", "operations", "owner"].includes(activeSection.value) ? activeSection.value as SettingsSection : null);
const activeDiagnosticsSection = computed<DiagnosticsSection | null>(() => view.value === "diagnostics" && activeSection.value && ["model-runs", "media-prompts", "events", "workflows"].includes(activeSection.value) ? activeSection.value as DiagnosticsSection : null);
const activeViewLabel = computed(() => ({ chat: "聊天", moments: "动态", instances: "聊天", diagnostics: "诊断中心", settings: "设置" })[view.value]);

async function navigate(next: WorkspaceView, correlationId = "") {
  view.value = next;
  activeSection.value = next === "diagnostics" && correlationId.trim() ? "model-runs" : null;
  if (next === "diagnostics") controlCenter.diagnosticsCorrelationFilter = correlationId;
  const path = pathForView(next, correlationId, activeSection.value ?? "");
  if (`${window.location.pathname}${window.location.search}` !== path) window.history.pushState({ view: next }, "", path);
  if (next === "instances") await controlCenter.loadActorGroups();
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
  activeSection.value = sectionFromLocation(view.value);
  syncDiagnosticsFilterFromLocation();
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
  if (store.authenticated) await controlCenter.ensureDefaultGroup(store.fluctlights.map((item) => item.id));
}

onMounted(() => {
  window.addEventListener("popstate", onPopState);
  syncDiagnosticsFilterFromLocation();
  void initializeWorkspace();
});
onBeforeUnmount(() => window.removeEventListener("popstate", onPopState));
</script>

<template>
  <AppShell :active-view="view" :active-section="activeSection" :show-navigation="store.authenticated === true" @navigate="navigate" @navigate-section="navigateSection" @select-instance="openDesktopChat" @create="openCreateDialog">
    <AuthPanel
      v-if="store.authenticated !== true"
      :setup-available="store.setupAvailable"
      :loading="store.authLoading"
      :error="store.authError"
      @setup="handleSetup"
      @sign-in="handleSignIn"
    />

    <template v-else>
      <ChatView v-if="view === 'chat'" @back="navigate('instances')" @open-details="openDetails" @open-instances="navigate('instances')" />
      <MomentsView v-else-if="view === 'moments'" />
      <InstancesView v-else-if="view === 'instances'" :open-governance="governanceRequest" :open-create="createRequest" @open-chat="navigate('chat')" @open-details="openDetails" @open-diagnostics="(correlationId) => navigate('diagnostics', correlationId)" />
      <DiagnosticsView v-else-if="view === 'diagnostics'" :section="activeDiagnosticsSection" @navigate-section="(section) => navigateSection('diagnostics', section)" />
      <SettingsView v-else :section="activeSettingsSection" @navigate-section="(section) => navigateSection('settings', section)" @logout="store.logout" />
    </template>

    <InstanceDetailsDialog :open="showDetails" @close="showDetails = false" @manage="openGovernance" />
    <span class="sr-only" aria-live="polite">当前页面：{{ activeViewLabel }}</span>
  </AppShell>
</template>

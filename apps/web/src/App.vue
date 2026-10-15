<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from "vue";

import { isDiagnosticsSection, isLifeSection, isSettingsSection, type DiagnosticsSection, type LifeSection, type SettingsSection, type WorkspaceSection, type WorkspaceView } from "./app/navigation";
import { useConversationStore } from "./stores/conversations";
import { useControlCenterStore } from "./stores/control-center";
import { mountProduct } from "./v3/proto/app";

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
  if (/\/settings\/autonomy\/?$/.test(path) || /\/autonomy\/?$/.test(path)) return "autonomy";
  if (path.startsWith("/settings")) return "settings";
  if (path.startsWith("/studio")) return "studio";
  if (path.startsWith("/pending")) return "pending";
  if (path.startsWith("/data/rollback")) return "rollback";
  if (path.startsWith("/data")) return "data";
  if (path.startsWith("/inbox")) return "inbox";
  if (/\/life(\/|$)/.test(path)) return "life";
  if (/\/world(\/|$)/.test(path)) return "world";
  if (/\/people(\/|$)/.test(path)) return "people";
  if (/\/growth(\/|$)/.test(path)) return "growth";
  if (/\/media(\/|$)/.test(path)) return "media";
  if (/\/profile(\/|$)/.test(path)) return "profile";
  return "instances";
}

function legacyCanonicalPath(): string | null {
  const path = window.location.pathname.replace(/\/+$/, "") || "/";
  const search = window.location.search;
  if (path === "/instances") return `/library${search}`;
  if (path === "/moments") return `/feed${search}`;
  if (path === "/settings/diagnostics") return `/diagnostics${search}`;
  return null;
}

function applyLegacyRedirect() {
  const next = legacyCanonicalPath();
  if (!next || `${window.location.pathname}${window.location.search}` === next) return;
  window.history.replaceState({ view: viewFromLocation() }, "", next);
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
    : next === "autonomy" ? scoped("settings/autonomy")
    : next === "pending" ? "/pending"
    : next === "rollback" ? "/data/rollback"
    : next === "data" ? "/data"
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
const activeViewLabel = computed(() => ({ chat: "对话", moments: "动态", instances: "角色库", diagnostics: "运行诊断", settings: "系统设置", studio: "创作中心", inbox: "通知", life: "生活", world: "世界", people: "人物关系", growth: "记忆成长", media: "媒体", profile: "角色资料", autonomy: "自治权限", pending: "待处理", data: "数据工具", rollback: "时间线回档", showcase: "组件场景", lifeShowcase: "生活组件场景" })[view.value]);

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
  applyLegacyRedirect();
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
  applyLegacyRedirect();
  syncDiagnosticsFilterFromLocation();
  const host = document.getElementById("v3-root");
  if (host) void mountProduct(host);
});
onBeforeUnmount(() => window.removeEventListener("popstate", onPopState));
</script>

<template>
  <div id="v3-root"></div>
</template>

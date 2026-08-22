<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, reactive, ref, watch } from 'vue';
import { useAppStore } from '../stores/app';
import { useConversationsStore } from '../stores/conversations';
import { useActivitiesStore } from '../stores/activities';
import { useChatStream } from '../composables/useChatStream';
import { useComposer } from '../composables/useComposer';
import { useActivities } from '../composables/useActivities';
import { useBootstrap } from '../composables/useBootstrap';
import AppDialog from '../components/shell/AppDialog.vue';
import AppRail from '../components/shell/AppRail.vue';
import AppSidebar from '../components/shell/AppSidebar.vue';
import MobileNav from '../components/shell/MobileNav.vue';
import HiddenActivitiesPanel from '../components/activity/HiddenActivitiesPanel.vue';
import type { ContactGroup, H3PreflightResult, InspectorActionResult, Message, PersonaDetailData, PersonaSummary, SettingsSnapshot, ViewName } from '../components/types';
import ActivityView from '../views/ActivityView.vue';
import ChatView from '../views/ChatView.vue';
import ContactsView from '../views/ContactsView.vue';

const InspectorPanel = defineAsyncComponent(() => import('../components/inspector/InspectorPanel.vue'));
const PersonaDetail = defineAsyncComponent(() => import('../components/persona/PersonaDetail.vue'));
const PersonaWizard = defineAsyncComponent(() => import('../components/persona/PersonaWizard.vue'));
const SettingsForm = defineAsyncComponent(() => import('../components/settings/SettingsForm.vue'));
const SettingsView = defineAsyncComponent(() => import('../views/SettingsView.vue'));

// Stores own API calls and stream/history side effects. This shell only maps
// store snapshots to views and turns user intent into store actions.
const app = useAppStore();
const conversations = useConversationsStore();
const activities = useActivitiesStore();
const activityPersonaId = ref<string | null>(null);
const activePersonaId = computed<string | null>(() => app.activePersonaId);
const chatStream = useChatStream();
const activityFeed = useActivities(() => activityPersonaId.value, {autoLoad: false});
const composer = useComposer({
  send: async ({text}) => {
    const personaId = activePersonaId.value;
    if (!personaId) throw new Error('请先选择一个摇光实例');
    return chatStream.send({personaId, text});
  }
});
const bootstrap = useBootstrap({
  guard: () => ({isSending: composer.isSending.value, isComposing: composer.isComposing.value, draft: composer.draft.value})
});

const view = ref<ViewName>('contacts');
const selectedGroupId = computed<string | null>({
  get: () => app.activeGroupId,
  set: value => app.selectGroup(value)
});
const detailOpen = ref(false);
const detailLoading = ref(false);
const detail = ref<PersonaDetailData | null>(null);
const wizardOpen = ref(false);
const wizardStage = ref<'description' | 'preview'>('description');
const wizardDescription = ref('');
const wizardPreview = ref<Record<string, unknown> | null>(null);
const wizardBusy = ref(false);
const wizardError = ref<string | null>(null);
const settingsOpen = ref(false);
const settingsBusy = ref(false);
const settingsError = ref<string | null>(null);
const inspectorOpen = ref(false);
const inspectorLoading = ref(false);
const inspectorError = ref<string | null>(null);
const inspectorActionError = ref<string | null>(null);
const inspectorActionBusy = ref(false);
const inspectorPreflight = ref<H3PreflightResult | null>(null);
const inspectorActionResult = ref<InspectorActionResult | null>(null);
const hiddenOpen = ref(false);
const hiddenPersonaId = ref<string | null>(null);
const draftByPersona = reactive<Record<string, string>>({});
const draft = composer.draft;
const isComposing = composer.isComposing;
const isSending = composer.isSending;
const sendError = computed(() => composer.error.value || chatStream.error.value);

const boot = computed(() => app.boot);
const personas = computed<PersonaSummary[]>(() => app.personas);
const groups = computed<ContactGroup[]>(() => app.groups);
const settings = computed<SettingsSnapshot>(() => app.settings as SettingsSnapshot);
const activePersona = computed(() => personas.value.find(persona => persona.id === activePersonaId.value) || null);
const currentConversation = computed(() => {
  const id = activePersonaId.value;
  if (!id) return { items: [], nextCursor: null, hasMore: false, loadingInitial: false, loadingOlder: false, historyError: null, stream: { status: 'idle' } };
  return conversations.get(id) || { items: [], nextCursor: null, hasMore: false, loadingInitial: false, loadingOlder: false, historyError: null, stream: { status: 'idle' } };
});
const conversationMessages = computed<Message[]>(() => currentConversation.value.items as unknown as Message[] || []);
const wizardPreviewData = computed(() => wizardPreview.value || undefined);
const activityState = activityFeed.state;
const hiddenActivityState = computed(() => hiddenPersonaId.value ? activities.getHidden(hiddenPersonaId.value) : {
  items: [], nextCursor: null, hasMore: false, loading: false, loadingMore: false, error: null, pagesLoaded: 0
});

function objectValue(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {};
}

function navigate(nextView: ViewName) {
  view.value = nextView;
  if (nextView === 'activity') {
    activityPersonaId.value = null;
    void refreshActivities().catch(() => {});
  }
}

async function selectPersona(id: string) {
  const previousId = activePersonaId.value;
  if (previousId) draftByPersona[previousId] = draft.value;
  view.value = 'chat';
  app.selectPersona(id);
  composer.setDraft(draftByPersona[id] ?? '');
  await conversations.loadInitial(id).catch(() => undefined);
}

async function openDetail(id: string) {
  detailOpen.value = true;
  detailLoading.value = true;
  try { detail.value = await app.loadPersonaDetail(id); }
  catch { detail.value = personas.value.find(persona => persona.id === id) || null; }
  finally { detailLoading.value = false; }
}

function closeDetail() { detailOpen.value = false; }
function openWizard() { wizardDescription.value = ''; wizardPreview.value = null; wizardStage.value = 'description'; wizardError.value = null; wizardOpen.value = true; }

async function analyzePersona(description: string) {
  if (!description || wizardBusy.value) return;
  wizardBusy.value = true;
  wizardError.value = null;
  wizardDescription.value = description;
  try {
    const analysis = await app.analyzePersona(description);
    const preview = objectValue(analysis.preview || analysis);
    const blueprint = objectValue(preview.blueprint);
    const card = objectValue(objectValue(blueprint.characterCard).roleCore);
    const answers = objectValue(analysis.answers);
    const cast = Array.isArray(blueprint.supportingCast)
      ? blueprint.supportingCast.map(item => typeof item === 'string' ? item : objectValue(item).name).filter(Boolean)
      : [];
    const routine = Array.isArray(blueprint.routine)
      ? blueprint.routine.map(item => typeof item === 'string' ? item : objectValue(item).label).filter(Boolean)
      : [];
    const inferredFields = Array.isArray(analysis.inferredFields)
      ? analysis.inferredFields
      : Array.isArray(preview.inferredFields) ? preview.inferredFields : [];
    wizardPreview.value = {
      name: card.name || answers.name || preview.name || '',
      role: answers.role || preview.role || '',
      foundation: preview.foundation || '',
      interests: Array.isArray(blueprint.interests) ? blueprint.interests : [],
      visualBaseline: blueprint.visualBaseline || preview.visualBaseline || '',
      supportingCast: cast,
      routine,
      inferred: inferredFields.length > 0
    };
    wizardStage.value = 'preview';
  } catch (error) { wizardError.value = error instanceof Error ? error.message : '分析失败，请稍后重试。'; }
  finally { wizardBusy.value = false; }
}

async function createPersona(values: Record<string, unknown>) {
  if (wizardBusy.value) return;
  wizardBusy.value = true;
  wizardError.value = null;
  try {
    const created = await app.createPersona(values);
    wizardOpen.value = false;
    if (created?.id) await selectPersona(created.id);
    else await app.bootstrap({force: true});
  } catch (error) { wizardError.value = error instanceof Error ? error.message : '创建失败，请稍后重试。'; }
  finally { wizardBusy.value = false; }
}

async function saveSettings(nextSettings: SettingsSnapshot) {
  settingsBusy.value = true;
  settingsError.value = null;
  try { await app.updateSettings(nextSettings); settingsOpen.value = false; }
  catch (error) { settingsError.value = error instanceof Error ? error.message : '设置保存失败。'; }
  finally { settingsBusy.value = false; }
}

async function sendMessage() {
  await composer.submit().catch(() => undefined);
}

function dismissSendError() {
  composer.clearError();
  chatStream.clearError();
}

async function loadOlder() { if (activePersonaId.value) await conversations.loadOlder(activePersonaId.value).catch(() => undefined); }
async function retryHistory() {
  const id = activePersonaId.value;
  if (!id) return;
  const state = conversations.get(id);
  if (state?.nextCursor && state.hasMore) await conversations.loadOlder(id).catch(() => undefined);
  else await conversations.loadInitial(id).catch(() => undefined);
}

function prompt(value: string) { draft.value = value; }
function openSettings() { settingsError.value = null; settingsOpen.value = true; }
function openGroups() {
  if (typeof window === 'undefined' || !groups.value.length) return;
  const options = groups.value.map((group, index) => `${index + 1}. ${group.name}`).join('\n');
  const answer = window.prompt(`选择联系人分组（输入序号，留空显示全部；输入 + 创建分组）\n${options}`, '');
  if (answer === null) return;
  if (answer.trim() === '+') {
    const name = window.prompt('新分组名称');
    if (name?.trim()) void app.createContactGroup(name.trim()).catch(() => {});
    return;
  }
  const index = Number.parseInt(answer, 10);
  app.selectGroup(Number.isInteger(index) && index > 0 && groups.value[index - 1] ? groups.value[index - 1].id : null);
}

async function openInspector(id: string) {
  inspectorOpen.value = true;
  inspectorLoading.value = true;
  inspectorError.value = null;
  inspectorActionError.value = null;
  inspectorPreflight.value = null;
  inspectorActionResult.value = null;
  try { await app.loadInspector(id); }
  catch (error) { inspectorError.value = error instanceof Error ? error.message : '检查器暂时不可用。'; }
  finally { inspectorLoading.value = false; }
}

async function refreshInspector() { if (activePersonaId.value) await openInspector(activePersonaId.value); }
function openPersonaActivity(id: string) { detailOpen.value = false; activityPersonaId.value = id; view.value = 'activity'; void refreshActivities().catch(() => {}); }
async function deletePersona(id: string) {
  const deletingActive = activePersonaId.value === id;
  try {
    await app.deletePersona(id);
    detailOpen.value = false;
    if (deletingActive) {
      view.value = 'contacts';
      composer.setDraft('');
    }
  } catch {
    // The detail view owns its existing action feedback; deletion failure must
    // not navigate away from the current persona.
  }
}
function screenPersona(id: string) { void app.screenPersona(id).catch(() => {}); }
function saveGroup(id: string, groupId: string | null) { void app.assignPersonaGroup(id, groupId).catch(() => {}); }
function savePolicy(id: string, policy: string) { void app.updateImageGenerationPolicy(id, policy).catch(() => {}); }
function deleteMemory(personaId: string, memoryId: string) { void app.deleteMemory(personaId, memoryId).catch(() => {}); }
function rollbackEvolution(personaId: string, evolutionId: string) { void app.rollbackEvolution(personaId, evolutionId).catch(() => {}); }
function restoreFoundation(personaId: string, revisionId: string) { void app.restoreFoundation(personaId, revisionId).catch(() => {}); }
function editFoundation(id: string) { void app.editFoundation(id).catch(() => {}); }
function rescheduleSchedule(personaId: string, scheduleId: string) { void app.rescheduleSchedule(personaId, scheduleId).catch(() => {}); }
function cancelSchedule(personaId: string, scheduleId: string) { void app.cancelSchedule(personaId, scheduleId).catch(() => {}); }
async function hiddenActivities(id: string) {
  hiddenPersonaId.value = id;
  hiddenOpen.value = true;
  await activities.loadHiddenInitial(id).catch(() => undefined);
}
async function loadMoreHiddenActivities() {
  if (hiddenPersonaId.value) await activities.loadHiddenMore(hiddenPersonaId.value).catch(() => undefined);
}
async function retryHiddenActivities() {
  if (hiddenPersonaId.value) await activities.loadHiddenInitial(hiddenPersonaId.value).catch(() => undefined);
}
async function restoreHiddenActivity(id: string) {
  if (!hiddenPersonaId.value) return;
  try { await activities.restore(id, hiddenPersonaId.value); }
  catch (error) { inspectorActionError.value = error instanceof Error ? error.message : '恢复动态失败。'; }
}
function closeHiddenActivities() { hiddenOpen.value = false; hiddenPersonaId.value = null; }

async function runH3Preflight() {
  if (!activePersonaId.value) return;
  inspectorActionBusy.value = true;
  inspectorActionError.value = null;
  try { inspectorPreflight.value = await app.runH3Preflight(); }
  catch (error) { inspectorActionError.value = error instanceof Error ? error.message : 'h3 预检失败。'; }
  finally { inspectorActionBusy.value = false; }
}
async function simulateInspector(input: Record<string, unknown>) {
  if (!activePersonaId.value) return;
  inspectorActionBusy.value = true;
  inspectorActionError.value = null;
  try { inspectorActionResult.value = await app.simulatePersona(activePersonaId.value, {...input, publish: true}); await app.bootstrap({force: true}); }
  catch (error) { inspectorActionError.value = error instanceof Error ? error.message : '模拟事件失败。'; }
  finally { inspectorActionBusy.value = false; }
}
async function debugMedia(input: Record<string, unknown>) {
  if (!activePersonaId.value) return;
  inspectorActionBusy.value = true;
  inspectorActionError.value = null;
  try {
    const result = await app.debugMedia(activePersonaId.value, input);
    await refreshInspector();
    inspectorActionResult.value = result;
  }
  catch (error) { inspectorActionError.value = error instanceof Error ? error.message : '测试媒体作业失败。'; }
  finally { inspectorActionBusy.value = false; }
}
function activityLike(id: string) {
  const item = activityState.value.items.find(activity => activity.id === id);
  void activities.like(id, !(item?.liked ?? false), activityPersonaId.value).catch(() => {});
}
function activityHide(id: string) { void activities.hide(id, true, activityPersonaId.value).catch(() => {}); }
function activityComment(id: string) { activities.startComment(id); }
function activitySubmitComment(id: string, content: string) { void activities.comment(id, content, activityPersonaId.value).catch(() => {}); }
function cancelComment() { activities.cancelComment(); }
async function refreshActivities() { await activities.loadInitial(activityPersonaId.value); await activities.markRead().catch(() => {}); app.setActivityUnread(false); }
async function loadMoreActivities() { await activities.loadMore(activityPersonaId.value); }
async function retryActivities() { const state = activityState.value; if (state.nextCursor && state.hasMore) await activities.loadMore(activityPersonaId.value); else await activities.loadInitial(activityPersonaId.value); }
function showAllActivities() { activityPersonaId.value = null; void refreshActivities().catch(() => {}); }

onMounted(() => {
  void bootstrap.start().catch(() => {}).finally(() => bootstrap.startPolling());
});

watch(draft, value => {
  const id = activePersonaId.value;
  if (id) draftByPersona[id] = value;
});
</script>

<template>
  <div class="app-frame" :class="{ 'app-frame--chat': view === 'chat' }">
    <AppRail :current-view="view" :activity-unread="Boolean(app.activityUnread)" @navigate="navigate" @brand="navigate('contacts')" />
    <AppSidebar :personas="personas" :groups="groups" :active-persona-id="activePersonaId" :active-group-id="selectedGroupId" :current-view="view" :loading="boot === 'loading'" @navigate="navigate" @select-persona="selectPersona" @create="openWizard" @open-groups="openGroups" />
    <main class="main-pane">
      <div v-if="boot === 'error'" class="startup-error" role="alert"><h1>联系人暂时无法加载</h1><p>请检查服务状态后重试。</p><button class="primary" type="button" @click="bootstrap.start().catch(() => {})">重试</button></div>
      <ContactsView v-else-if="view === 'contacts'" :personas="personas" :groups="groups" :selected-group-id="selectedGroupId" :loading="boot === 'loading'" @select-persona="selectPersona" @create="openWizard" @select-group="openGroups" />
      <ChatView v-else-if="view === 'chat' && activePersona" :persona="activePersona" :messages="conversationMessages" :draft="draft" :loading="currentConversation.loadingInitial" :loading-older="currentConversation.loadingOlder" :history-error="currentConversation.historyError" :has-more="currentConversation.hasMore" :is-sending="isSending || currentConversation.stream?.status === 'sending'" :is-composing="isComposing" :send-error="sendError" :debug-inspector="Boolean(app.debugInspector)" :simplified-media="Boolean(settings.simplifiedMediaMode)" @back="navigate('contacts')" @profile="openDetail(activePersona.id)" @tools="app.debugInspector ? openInspector(activePersona.id) : openSettings()" @load-older="loadOlder" @retry-history="retryHistory" @prompt="prompt" @update:draft="composer.setDraft($event)" @submit="sendMessage" @composition-start="composer.onCompositionStart" @composition-end="composer.onCompositionEnd" @selection-change="composer.setSelection" @dismiss-send-error="dismissSendError" />
      <ActivityView v-else-if="view === 'activity'" :items="activityState.items" :personas="personas" :persona-id="activityPersonaId" :next-cursor="activityState.nextCursor" :loading="activityState.loading" :loading-more="activityState.loadingMore" :error="activityState.error" :commenting-id="activities.commentingId" :simplified-media="Boolean(settings.simplifiedMediaMode)" @refresh="refreshActivities" @load-more="loadMoreActivities" @retry="retryActivities" @open-persona="openDetail" @like="activityLike" @hide="activityHide" @comment="activityComment" @cancel-comment="cancelComment" @submit-comment="activitySubmitComment" @chat="selectPersona" @all="showAllActivities" />
      <SettingsView v-else @system="openSettings" @create="openWizard" @personas="navigate('contacts')" />
    </main>
    <MobileNav :current-view="view" :activity-unread="Boolean(app.activityUnread)" @navigate="navigate" />

    <AppDialog v-model:open="wizardOpen" size="large" labelled-by="persona-dialog-title"><PersonaWizard :stage="wizardStage" :description="wizardDescription" :preview="wizardPreviewData" :analyzing="wizardBusy && wizardStage === 'description'" :creating="wizardBusy && wizardStage === 'preview'" :error="wizardError" @close="wizardOpen = false" @analyze="analyzePersona" @back="wizardStage = 'description'" @create="createPersona" /></AppDialog>
    <AppDialog v-model:open="detailOpen" size="large" labelled-by="persona-detail-title"><PersonaDetail v-if="detail" :detail="detail" :groups="groups" :loading="detailLoading" @close="closeDetail" @chat="selectPersona" @activity="openPersonaActivity" @delete="deletePersona" @screen="screenPersona" @save-group="saveGroup" @save-policy="savePolicy" @delete-memory="deleteMemory" @rollback="rollbackEvolution" @restore-foundation="restoreFoundation" @edit-foundation="editFoundation" @reschedule="rescheduleSchedule" @cancel-schedule="cancelSchedule" @hidden-activities="hiddenActivities" /></AppDialog>
    <AppDialog v-model:open="settingsOpen" size="medium" labelled-by="settings-dialog-title"><SettingsForm :settings="settings" :saving="settingsBusy" :error="settingsError" @close="settingsOpen = false" @save="saveSettings" /></AppDialog>
    <AppDialog v-model:open="inspectorOpen" size="large" labelled-by="inspector-dialog-title"><InspectorPanel :persona="detail" :media-jobs="app.inspector?.mediaJobs || []" :lifecycle="app.inspector?.lifecycle" :debug-context="app.inspector?.debugContext" :loading="inspectorLoading" :error="inspectorError || inspectorActionError" :action-busy="inspectorActionBusy" :h3-result="inspectorPreflight" :action-result="inspectorActionResult" @close="inspectorOpen = false" @refresh="refreshInspector" @retry="refreshInspector" @h3-preflight="runH3Preflight" @simulate="simulateInspector" @debug-media="debugMedia" /></AppDialog>
    <AppDialog v-model:open="hiddenOpen" size="medium" labelled-by="hidden-activities-title"><HiddenActivitiesPanel :persona-id="hiddenPersonaId" :items="hiddenActivityState.items" :next-cursor="hiddenActivityState.nextCursor" :loading="hiddenActivityState.loading" :loading-more="hiddenActivityState.loadingMore" :error="hiddenActivityState.error || inspectorActionError" @close="closeHiddenActivities" @load-more="loadMoreHiddenActivities" @retry="retryHiddenActivities" @restore="restoreHiddenActivity" /></AppDialog>
  </div>
</template>

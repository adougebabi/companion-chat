import {computed, ref} from 'vue';
import {defineStore} from 'pinia';
import {getBootstrap} from '../api/client';
import {listActivityPage} from '../api/activities';
import {
  activateInterview,
  analyzePersona as analyzePersonaRequest,
  assignPersonaGroup,
  cancelSchedule as cancelScheduleRequest,
  createGroup,
  createPersona as createPersonaRequest,
  deleteMemory as deleteMemoryRequest,
  deletePersona as deletePersonaRequest,
  getPersona,
  loadInspector as loadInspectorRequest,
  h3Preflight as h3PreflightRequest,
  simulatePersona as simulatePersonaRequest,
  debugMedia as debugMediaRequest,
  normalizePersonaDetail,
  rescheduleSchedule as rescheduleScheduleRequest,
  restoreFoundation as restoreFoundationRequest,
  rollbackEvolution as rollbackEvolutionRequest,
  screenPersona as screenPersonaRequest,
  updateFoundation as updateFoundationRequest,
  updateImageGenerationPolicy as updateImageGenerationPolicyRequest,
  updateSettings as updateSettingsRequest
} from '../api/personas';
import type {BootstrapResponse, ContactGroup, PersonaSummary, PublicSettings} from '../types';
import type {MediaJob, PersonaDetailData} from '../components/types';

export type AppView = 'contacts' | 'chat' | 'activity' | 'settings';
export type BootStatus = 'idle' | 'loading' | 'ready' | 'error';

const ACTIVE_PERSONA_KEY = 'companion-active-persona';
const ACTIVE_GROUP_KEY = 'companion-active-group';

function readStorage(key: string): string | null {
  if (typeof localStorage === 'undefined') return null;
  return localStorage.getItem(key);
}

function writeStorage(key: string, value: string | null): void {
  if (typeof localStorage === 'undefined') return;
  if (value) localStorage.setItem(key, value);
  else localStorage.removeItem(key);
}

export const useAppStore = defineStore('app', () => {
  // Match the static shell's loading skeleton before the first bootstrap
  // request resolves; an idle state would briefly render an empty contacts view.
  const boot = ref<BootStatus>('loading');
  const error = ref<string | null>(null);
  const personas = ref<PersonaSummary[]>([]);
  const groups = ref<ContactGroup[]>([]);
  const settings = ref<PublicSettings>({});
  const activityUnread = ref(false);
  const defaultTimezone = ref('UTC');
  const debugInspector = ref(false);
  const currentView = ref<AppView>('contacts');
  const activePersonaId = ref<string | null>(readStorage(ACTIVE_PERSONA_KEY));
  const activeGroupId = ref<string | null>(readStorage(ACTIVE_GROUP_KEY));
  const detail = ref<PersonaDetailData | null>(null);
  const inspector = ref<{mediaJobs: MediaJob[]; lifecycle: Record<string, unknown> | null; debugContext: Record<string, unknown> | null} | null>(null);
  const lastInterviewId = ref<string | null>(null);
  let bootstrapRequest: Promise<BootstrapResponse> | null = null;

  const activePersona = computed(() => personas.value.find(item => item.id === activePersonaId.value) ?? null);
  const activeGroup = computed(() => groups.value.find(item => item.id === activeGroupId.value) ?? null);

  function applyBootstrap(snapshot: BootstrapResponse): void {
    personas.value = snapshot.personas;
    groups.value = snapshot.groups;
    settings.value = snapshot.settings;
    activityUnread.value = snapshot.activityUnread;
    defaultTimezone.value = snapshot.defaultTimezone || 'UTC';
    debugInspector.value = snapshot.debugInspector === true;

    const savedPersona = activePersonaId.value;
    if (savedPersona && personas.value.some(persona => persona.id === savedPersona)) {
      activePersonaId.value = savedPersona;
    } else {
      activePersonaId.value = personas.value[0]?.id ?? null;
      writeStorage(ACTIVE_PERSONA_KEY, activePersonaId.value);
    }

    const savedGroup = activeGroupId.value;
    if (savedGroup && groups.value.some(group => group.id === savedGroup)) {
      activeGroupId.value = savedGroup;
    } else {
      activeGroupId.value = groups.value.find(group => group.isDefault)?.id ?? groups.value[0]?.id ?? null;
      writeStorage(ACTIVE_GROUP_KEY, activeGroupId.value);
    }
  }

  async function bootstrap(options: {signal?: AbortSignal; force?: boolean} = {}): Promise<BootstrapResponse> {
    // `boot` starts as loading so the shell can render its skeleton. That is
    // not evidence that a request has started; coalesce only an actual
    // in-flight request so the first mount still fetches bootstrap data.
    if (bootstrapRequest) return bootstrapRequest;
    boot.value = 'loading';
    error.value = null;
    const performBootstrap = async (): Promise<BootstrapResponse> => {
      try {
        const snapshot = await getBootstrap({signal: options.signal});
        applyBootstrap(snapshot);
        boot.value = 'ready';
        return snapshot;
      } catch (caught) {
        if ((caught as Error)?.name === 'AbortError') throw caught;
        error.value = caught instanceof Error ? caught.message : '联系人加载失败';
        boot.value = 'error';
        throw caught;
      } finally {
        bootstrapRequest = null;
      }
    };
    bootstrapRequest = performBootstrap();
    return bootstrapRequest;
  }

  function selectPersona(personaId: string | null): void {
    activePersonaId.value = personaId;
    writeStorage(ACTIVE_PERSONA_KEY, personaId);
  }

  function selectGroup(groupId: string | null): void {
    activeGroupId.value = groupId;
    writeStorage(ACTIVE_GROUP_KEY, groupId);
  }

  async function loadPersonaDetail(personaId: string): Promise<PersonaDetailData> {
    const result = await getPersona(personaId);
    detail.value = result;
    return result;
  }

  async function analyzePersona(description: string): Promise<Record<string, unknown>> {
    const result = await analyzePersonaRequest(description);
    lastInterviewId.value = typeof result.interviewId === 'string'
      ? result.interviewId
      : typeof result.id === 'string' ? result.id : null;
    return result as Record<string, unknown>;
  }

  async function createPersona(input: Record<string, unknown>): Promise<PersonaSummary> {
    const interviewId = typeof input.interviewId === 'string' ? input.interviewId : lastInterviewId.value;
    const result = interviewId
      ? await activateInterview(interviewId, Object.fromEntries(Object.entries(input).filter(([key]) => !['interviewId', 'description', 'source', 'status', 'inferredFields', 'fieldSources', 'preview'].includes(key))))
      : await createPersonaRequest(input);
    lastInterviewId.value = null;
    await bootstrap({force: true});
    return result;
  }

  async function updateSettings(next: PublicSettings): Promise<PublicSettings> {
    const result = await updateSettingsRequest(next);
    settings.value = {...settings.value, ...result};
    debugInspector.value = result.debugInspector === true || next.debugInspector === true;
    return result;
  }

  async function createContactGroup(name: string): Promise<unknown> {
    const result = await createGroup(name);
    await bootstrap({force: true});
    return result;
  }

  async function deletePersona(personaId: string): Promise<void> {
    await deletePersonaRequest(personaId);
    if (activePersonaId.value === personaId) selectPersona(null);
    await bootstrap({force: true});
  }

  async function assignPersonaGroupAction(personaId: string, groupId: string | null): Promise<unknown> {
    if (!groupId) return null;
    const result = await assignPersonaGroup(personaId, groupId);
    if (detail.value?.id === personaId) detail.value = normalizePersonaDetail({...detail.value, groupId});
    await bootstrap({force: true});
    return result;
  }

  async function updateImageGenerationPolicyAction(personaId: string, policy: string): Promise<unknown> {
    const result = await updateImageGenerationPolicyRequest(personaId, policy);
    if (detail.value?.id === personaId) detail.value = normalizePersonaDetail({...detail.value, imageGenerationPolicy: policy});
    return result;
  }

  async function screenPersonaAction(personaId: string): Promise<unknown> {
    const current = personas.value.find(item => item.id === personaId);
    const screened = !(current?.screened ?? detail.value?.screened ?? false);
    const result = await screenPersonaRequest(personaId, screened);
    if (detail.value?.id === personaId) detail.value = normalizePersonaDetail({...detail.value, screened});
    await bootstrap({force: true});
    return result;
  }

  async function deleteMemoryAction(personaId: string, memoryId: string): Promise<void> {
    await deleteMemoryRequest(personaId, memoryId);
    await loadPersonaDetail(personaId);
  }

  async function rollbackEvolutionAction(personaId: string, evolutionId: string): Promise<unknown> {
    const result = await rollbackEvolutionRequest(personaId, evolutionId);
    await loadPersonaDetail(personaId);
    return result;
  }

  async function restoreFoundationAction(personaId: string, revisionId: string): Promise<unknown> {
    const result = await restoreFoundationRequest(personaId, revisionId);
    await loadPersonaDetail(personaId);
    return result;
  }

  async function editFoundationAction(personaId: string): Promise<unknown> {
    const current = detail.value?.id === personaId ? detail.value.foundation || '' : '';
    if (typeof window === 'undefined') return null;
    const next = window.prompt('修订身份核心', current);
    if (next === null || next.trim() === current.trim()) return null;
    const result = await updateFoundationRequest(personaId, next.trim());
    await loadPersonaDetail(personaId);
    return result;
  }

  async function rescheduleScheduleAction(personaId: string, scheduleId: string): Promise<unknown> {
    if (typeof window === 'undefined') return null;
    const next = window.prompt('输入新的开始时间（ISO 8601）');
    if (!next?.trim()) return null;
    const result = await rescheduleScheduleRequest(personaId, scheduleId, next.trim());
    await loadPersonaDetail(personaId);
    return result;
  }

  async function cancelScheduleAction(personaId: string, scheduleId: string): Promise<void> {
    await cancelScheduleRequest(personaId, scheduleId);
    await loadPersonaDetail(personaId);
  }

  async function loadHiddenActivities(personaId: string): Promise<unknown> {
    return listActivityPage({personaId, visibility: 'hidden', limit: 20});
  }

  async function loadInspector(personaId: string): Promise<void> {
    const result = await loadInspectorRequest(personaId);
    detail.value = result.persona;
    inspector.value = {mediaJobs: result.mediaJobs, lifecycle: result.lifecycle, debugContext: result.debugContext};
  }

  async function runH3Preflight() {
    return h3PreflightRequest();
  }

  async function simulatePersona(personaId: string, input: Record<string, unknown>) {
    return simulatePersonaRequest(personaId, input);
  }

  async function debugMedia(personaId: string, input: Record<string, unknown>) {
    return debugMediaRequest(personaId, input as {kind: 'image' | 'video'; [key: string]: unknown});
  }

  function setView(view: AppView): void {
    currentView.value = view;
  }

  function setActivityUnread(value: boolean): void {
    activityUnread.value = value;
  }

  return {
    boot,
    error,
    personas,
    groups,
    settings,
    activityUnread,
    defaultTimezone,
    debugInspector,
    currentView,
    activePersonaId,
    activeGroupId,
    activePersona,
    activeGroup,
    detail,
    inspector,
    bootstrap,
    applyBootstrap,
    selectPersona,
    selectGroup,
    setActivePersona: selectPersona,
    loadPersonaDetail,
    getPersona: loadPersonaDetail,
    analyzePersona,
    createPersona,
    updateSettings,
    createContactGroup,
    openGroupPicker: () => undefined,
    deletePersona,
    assignPersonaGroup: assignPersonaGroupAction,
    updateImageGenerationPolicy: updateImageGenerationPolicyAction,
    screenPersona: screenPersonaAction,
    deleteMemory: deleteMemoryAction,
    rollbackEvolution: rollbackEvolutionAction,
    restoreFoundation: restoreFoundationAction,
    editFoundation: editFoundationAction,
    rescheduleSchedule: rescheduleScheduleAction,
    cancelSchedule: cancelScheduleAction,
    loadHiddenActivities,
    loadInspector,
    runH3Preflight,
    simulatePersona,
    debugMedia,
    setView,
    setActivityUnread
  };
});

export default useAppStore;

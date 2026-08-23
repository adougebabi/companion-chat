import {encodePath, requestJson} from './client';
import type {PersonaSummary, PublicSettings} from '../types';
import type {DebugContext, DebugInspectorSnapshot, DebugLifecycle, DurableJob, H3PreflightResult, InspectorActionResult, PersonaDetailData, PromptRun} from '../components/types';

export interface PersonaAnalysis {
  id?: string;
  interviewId?: string;
  preview?: Record<string, unknown>;
  inferredFields?: string[];
  [key: string]: unknown;
}

function record(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? value as Record<string, unknown>
    : {};
}

function pick<T>(source: Record<string, unknown>, ...keys: string[]): T | undefined {
  for (const key of keys) {
    if (source[key] !== undefined) return source[key] as T;
  }
  return undefined;
}

export function normalizePersonaDetail(value: unknown, fallback?: (Pick<PersonaSummary, 'id' | 'name'> & Partial<PersonaSummary>) | null): PersonaDetailData {
  const source = record(value);
  const persona = record(source.persona);
  const base = {...(fallback || {}), ...persona, ...source};
  const id = String(base.id ?? fallback?.id ?? '');
  const foundationValue = pick<unknown>(source, 'foundation');
  const foundationSummary = record(source.foundationSummary);
  const foundation = typeof foundationValue === 'string'
    ? foundationValue
    : [foundationSummary.identity,
      Array.isArray(foundationSummary.routine) && foundationSummary.routine.length ? `日常节奏：${foundationSummary.routine.join(' · ')}` : '',
      Array.isArray(foundationSummary.interests) && foundationSummary.interests.length ? `喜欢：${foundationSummary.interests.join('、')}` : '']
      .filter(item => typeof item === 'string' && item.length > 0).join('\n');
  return {
    ...(base as PersonaSummary),
    id,
    name: String(base.name ?? fallback?.name ?? id),
    role: String(base.role ?? fallback?.role ?? ''),
    groupId: (base.groupId ?? fallback?.groupId ?? null) as string | null,
    screened: Boolean(base.screened ?? fallback?.screened),
    currentSituation: String(base.currentSituation ?? fallback?.currentSituation ?? ''),
    mood: String(base.mood ?? fallback?.mood ?? ''),
    unreadCount: Number(base.unreadCount ?? fallback?.unreadCount ?? 0),
    foundation: foundation || null,
    foundationRevisions: (pick<unknown[]>(source, 'foundationRevisions', 'foundation_versions') ?? []) as PersonaDetailData['foundationRevisions'],
    memories: (pick<unknown[]>(source, 'memories') ?? []) as PersonaDetailData['memories'],
    evolutions: (pick<unknown[]>(source, 'evolutions', 'relationshipChanges') ?? []) as PersonaDetailData['evolutions'],
    supportingCharacters: (pick<unknown[]>(source, 'supportingCharacters', 'supportingCast') ?? []) as PersonaDetailData['supportingCharacters'],
    state: (pick<PersonaDetailData['state']>(source, 'state', 'lifeState') ?? null),
    schedule: (pick<unknown[]>(source, 'schedule', 'schedules') ?? []) as PersonaDetailData['schedule'],
    inferredFields: (pick<string[]>(source, 'inferredFields') ?? []),
    lifecycle: (pick<Record<string, unknown>>(source, 'lifecycle') ?? null)
  };
}

export function getPersona(personaId: string, signal?: AbortSignal): Promise<PersonaDetailData> {
  return requestJson(`/api/companion/personas/${encodePath(personaId)}`, {signal})
    .then(payload => normalizePersonaDetail(payload));
}

export function analyzePersona(description: string, signal?: AbortSignal): Promise<PersonaAnalysis> {
  return requestJson('/api/companion/interviews/analyze', {
    method: 'POST', signal, body: JSON.stringify({description})
  }) as Promise<PersonaAnalysis>;
}

export function activateInterview(interviewId: string, overrides: Record<string, unknown>, signal?: AbortSignal): Promise<PersonaSummary> {
  return requestJson(`/api/companion/interviews/${encodePath(interviewId)}/activate`, {
    method: 'POST', signal, body: JSON.stringify({overrides})
  }).then(payload => {
    const source = record(payload);
    return (source.persona || source) as PersonaSummary;
  });
}

export function createPersona(input: Record<string, unknown>, signal?: AbortSignal): Promise<PersonaSummary> {
  return requestJson('/api/companion/personas', {method: 'POST', signal, body: JSON.stringify(input)})
    .then(payload => {
      const source = record(payload);
      return (source.persona || source) as PersonaSummary;
    });
}

export function createGroup(name: string, signal?: AbortSignal): Promise<unknown> {
  return requestJson('/api/companion/groups', {method: 'POST', signal, body: JSON.stringify({name})});
}

export function updateSettings(settings: PublicSettings, signal?: AbortSignal): Promise<PublicSettings> {
  return requestJson('/api/companion/settings', {method: 'PUT', signal, body: JSON.stringify(settings)});
}

export interface PersonaSimulationInput {
  kind?: string;
  situation?: string;
  mood?: string;
  scene?: string;
  visual?: boolean;
  publish?: boolean;
  [key: string]: unknown;
}

export interface DebugMediaInput {
  kind: 'image' | 'video';
  request?: string;
  count?: number;
  provider?: string;
  [key: string]: unknown;
}

export function h3Preflight(signal?: AbortSignal): Promise<H3PreflightResult> {
  return requestJson('/api/companion/h3-preflight', {method: 'POST', signal, body: JSON.stringify({} )}) as Promise<H3PreflightResult>;
}

export function simulatePersona(personaId: string, input: PersonaSimulationInput, signal?: AbortSignal): Promise<InspectorActionResult> {
  return requestJson(`/api/companion/personas/${encodePath(personaId)}/simulate`, {
    method: 'POST', signal, body: JSON.stringify(input)
  }) as Promise<InspectorActionResult>;
}

export function debugMedia(personaId: string, input: DebugMediaInput, signal?: AbortSignal): Promise<InspectorActionResult> {
  return requestJson(`/api/companion/personas/${encodePath(personaId)}/debug-media`, {
    method: 'POST', signal, body: JSON.stringify(input)
  }) as Promise<InspectorActionResult>;
}

export function assignPersonaGroup(personaId: string, groupId: string, signal?: AbortSignal): Promise<unknown> {
  return requestJson(`/api/companion/personas/${encodePath(personaId)}/group`, {method: 'PUT', signal, body: JSON.stringify({groupId})});
}

export function updateImageGenerationPolicy(personaId: string, policy: string, signal?: AbortSignal): Promise<unknown> {
  return requestJson(`/api/companion/personas/${encodePath(personaId)}/image-generation-policy`, {method: 'PUT', signal, body: JSON.stringify({policy})});
}

export function screenPersona(personaId: string, screened: boolean, signal?: AbortSignal): Promise<unknown> {
  return requestJson(`/api/companion/personas/${encodePath(personaId)}/screen`, {method: 'PUT', signal, body: JSON.stringify({screened})});
}

export function updateFoundation(personaId: string, foundation: string, signal?: AbortSignal): Promise<unknown> {
  return requestJson(`/api/companion/personas/${encodePath(personaId)}/foundation`, {method: 'PUT', signal, body: JSON.stringify({foundation})});
}

export function restoreFoundation(personaId: string, revisionId: string, signal?: AbortSignal): Promise<unknown> {
  return requestJson(`/api/companion/personas/${encodePath(personaId)}/foundation-revisions/${encodePath(revisionId)}/restore`, {method: 'POST', signal});
}

export function rollbackEvolution(personaId: string, evolutionId: string, signal?: AbortSignal): Promise<unknown> {
  return requestJson(`/api/companion/personas/${encodePath(personaId)}/evolutions/${encodePath(evolutionId)}/rollback`, {method: 'POST', signal});
}

export function deleteMemory(personaId: string, memoryId: string, signal?: AbortSignal): Promise<void> {
  return requestJson(`/api/companion/personas/${encodePath(personaId)}/memories/${encodePath(memoryId)}`, {method: 'DELETE', signal}).then(() => undefined);
}

export function deletePersona(personaId: string, signal?: AbortSignal): Promise<void> {
  return requestJson(`/api/companion/personas/${encodePath(personaId)}`, {method: 'DELETE', signal}).then(() => undefined);
}

export function rescheduleSchedule(personaId: string, scheduleId: string, startsAt: string, signal?: AbortSignal): Promise<unknown> {
  return requestJson(`/api/companion/personas/${encodePath(personaId)}/schedule/${encodePath(scheduleId)}`, {method: 'PATCH', signal, body: JSON.stringify({startsAt})});
}

export function cancelSchedule(personaId: string, scheduleId: string, signal?: AbortSignal): Promise<void> {
  return requestJson(`/api/companion/personas/${encodePath(personaId)}/schedule/${encodePath(scheduleId)}/cancel`, {method: 'POST', signal}).then(() => undefined);
}

function normalizeDebugContext(value: unknown): DebugContext {
  const source = record(value);
  const state = record(source.state);
  const normalizeState = (key: string): string | null => {
    const candidate = state[key];
    if (typeof candidate === 'string') return candidate;
    if (candidate === null || candidate === undefined) return null;
    if (typeof candidate === 'number' || typeof candidate === 'boolean') return String(candidate);
    const nested = record(candidate);
    for (const field of ['label', 'name', 'title', 'description', 'rationale', 'kind', 'type']) {
      if (typeof nested[field] === 'string') return nested[field] as string;
    }
    try { return JSON.stringify(candidate); } catch { return null; }
  };
  return {
    ...source,
    ...(Object.keys(state).length ? {
      state: {
        situation: normalizeState('situation'),
        scene: normalizeState('scene'),
        outfit: normalizeState('outfit'),
        special: normalizeState('special'),
        mood: normalizeState('mood')
      }
    } : {state: null}),
    mediaJobs: Array.isArray(source.mediaJobs) ? source.mediaJobs as DebugContext['mediaJobs'] : []
  };
}

function normalizeDurableJob(value: unknown): DurableJob {
  const source = record(value);
  const number = (key: string, fallback = 0): number => typeof source[key] === 'number' && Number.isFinite(source[key] as number)
    ? source[key] as number
    : Number(source[key] ?? fallback) || fallback;
  return {
    ...source,
    id: typeof source.id === 'string' ? source.id : null,
    jobType: typeof source.jobType === 'string' ? source.jobType : typeof source.job_type === 'string' ? source.job_type : 'unknown',
    status: typeof source.status === 'string' ? source.status : 'unknown',
    priority: number('priority', 0),
    runAfter: typeof source.runAfter === 'string' ? source.runAfter : typeof source.run_after === 'string' ? source.run_after : null,
    leaseExpiresAt: typeof source.leaseExpiresAt === 'string' ? source.leaseExpiresAt : typeof source.lease_expires_at === 'string' ? source.lease_expires_at : null,
    attemptCount: number('attemptCount', number('attempt_count', 0)),
    maxAttempts: number('maxAttempts', number('max_attempts', 0)),
    personaId: typeof source.personaId === 'string' ? source.personaId : typeof source.persona_id === 'string' ? source.persona_id : null,
    activityId: typeof source.activityId === 'string' ? source.activityId : typeof source.activity_id === 'string' ? source.activity_id : null,
    messageId: typeof source.messageId === 'string' ? source.messageId : typeof source.message_id === 'string' ? source.message_id : null,
    traceId: typeof source.traceId === 'string' ? source.traceId : typeof source.trace_id === 'string' ? source.trace_id : null,
    createdAt: typeof source.createdAt === 'string' ? source.createdAt : typeof source.created_at === 'string' ? source.created_at : null,
    updatedAt: typeof source.updatedAt === 'string' ? source.updatedAt : typeof source.updated_at === 'string' ? source.updated_at : null,
    completedAt: typeof source.completedAt === 'string' ? source.completedAt : typeof source.completed_at === 'string' ? source.completed_at : null,
    error: typeof source.error === 'string' ? source.error : null,
    payloadSummary: typeof source.payloadSummary === 'string' ? source.payloadSummary : null,
    resultSummary: typeof source.resultSummary === 'string' ? source.resultSummary : null
  };
}

function normalizeLifecycle(value: unknown): DebugLifecycle {
  const source = record(value);
  const rows = Array.isArray(source.jobs) ? source.jobs : [];
  return {...source, jobs: rows.map(normalizeDurableJob)};
}

export async function loadInspector(personaId: string, signal?: AbortSignal): Promise<{persona: PersonaDetailData; inspector: DebugInspectorSnapshot}> {
  const encoded = encodePath(personaId);
  const [personaPayload, debugContext, lifecycle, promptRuns] = await Promise.all([
    requestJson(`/api/companion/personas/${encoded}`, {signal}),
    requestJson(`/api/companion/personas/${encoded}/debug-context`, {signal}),
    requestJson(`/api/companion/personas/${encoded}/lifecycle`, {signal}),
    requestJson(`/api/companion/prompt-runs?personaId=${encodeURIComponent(personaId)}`, {signal})
  ]);
  const runs = Array.isArray(promptRuns) ? promptRuns : record(promptRuns).items;
  const promptRunItems = Array.isArray(runs) ? runs as PromptRun[] : [];
  const contextRecord = normalizeDebugContext(debugContext);
  const mediaJobs = Array.isArray(contextRecord.mediaJobs) ? contextRecord.mediaJobs : [];
  return {
    persona: normalizePersonaDetail(personaPayload),
    inspector: {
      personaId,
      debugContext: contextRecord,
      lifecycle: normalizeLifecycle(lifecycle),
      mediaJobs: mediaJobs as DebugInspectorSnapshot['mediaJobs'],
      promptRuns: promptRunItems
    }
  };
}

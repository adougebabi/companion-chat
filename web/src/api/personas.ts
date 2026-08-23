import {encodePath, requestJson} from './client';
import type {PersonaSummary, PublicSettings} from '../types';
import type {DebugInspectorSnapshot, H3PreflightResult, InspectorActionResult, PersonaDetailData, PromptRun} from '../components/types';

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

export async function loadInspector(personaId: string, signal?: AbortSignal): Promise<{persona: PersonaDetailData; inspector: DebugInspectorSnapshot}> {
  const encoded = encodePath(personaId);
  const [personaPayload, debugContext, lifecycle, promptRuns] = await Promise.all([
    requestJson(`/api/companion/personas/${encoded}`, {signal}),
    requestJson(`/api/companion/personas/${encoded}/debug-context`, {signal}),
    requestJson(`/api/companion/personas/${encoded}/lifecycle`, {signal}),
    requestJson(`/api/companion/prompt-runs?personaId=${encoded}`, {signal})
  ]);
  const runs = Array.isArray(promptRuns) ? promptRuns : record(promptRuns).items;
  const promptRunItems = Array.isArray(runs) ? runs as PromptRun[] : [];
  const contextRecord = record(debugContext);
  const mediaJobs = Array.isArray(contextRecord.mediaJobs) ? contextRecord.mediaJobs : [];
  return {
    persona: normalizePersonaDetail(personaPayload),
    inspector: {
      debugContext: contextRecord,
      lifecycle: record(lifecycle),
      mediaJobs: mediaJobs as DebugInspectorSnapshot['mediaJobs'],
      promptRuns: promptRunItems
    }
  };
}

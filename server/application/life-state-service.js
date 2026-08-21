function isRecord(value) { return Boolean(value) && typeof value === 'object' && !Array.isArray(value); }
function required(value, field) { if (typeof value !== 'string' || !value.trim()) throw Object.assign(new TypeError(`${field}不能为空`), {status: 400}); return value.trim(); }
function parse(value, fallback = {}) { if (value && typeof value === 'object') return value; try { return value ? JSON.parse(value) : fallback; } catch { return fallback; } }
function atFor(value, clock) { const result = value ? new Date(value) : new Date(clock()); if (!Number.isFinite(result.getTime())) throw new TypeError('生活状态时间无效'); return result; }

export function createLifeStateService({reader, resolver, stateRepository, clock = () => new Date().toISOString()} = {}) {
    if (!reader || typeof reader.readResolverInput !== 'function') throw new TypeError('Life state service requires life-world reader');
    if (typeof resolver !== 'function') throw new TypeError('Life state service requires a pure resolver');
    const state = stateRepository;

    function project(command = {}) {
        const personaId = required(command.personaId ?? command.persona_id, '人格 ID');
        const currentTime = atFor(command.at, clock);
        const input = reader.readResolverInput({personaId, at: currentTime});
        const resolved = resolver(input);
        const persisted = state?.read?.({personaId}) ?? {};
        return {
            ...persisted,
            personaId,
            situation: resolved.situation,
            mood: resolved.mood,
            appearance_json: JSON.stringify(resolved.appearance ?? {}),
            source_event_id: resolved.sourceEventId ?? resolved.eventId ?? null,
            resolved_source: resolved.source,
            sourceEvent: resolved.sourceEvent ?? null,
            resolved_source_id: resolved.sourceId ?? resolved.eventId ?? resolved.scheduleId ?? null,
            resolved_scene: resolved.scene,
            resolved_location: resolved.location,
            resolved_room: resolved.room,
            resolved_starts_at: resolved.startsAt ?? null,
            resolved_ends_at: resolved.endsAt ?? null,
            resolved_time_fact: resolved.timeFact ?? (resolved.endsAt ? 'known' : 'unknown'),
            resolved_next_boundary_at: resolved.nextBoundaryAt ?? resolved.endsAt ?? null,
            source: resolved.source,
            sourceId: resolved.sourceId ?? resolved.eventId ?? resolved.scheduleId ?? null,
            startsAt: resolved.startsAt ?? null,
            endsAt: resolved.endsAt ?? null,
            timeFact: resolved.timeFact ?? (resolved.endsAt ? 'known' : 'unknown'),
            nextBoundaryAt: resolved.nextBoundaryAt ?? resolved.endsAt ?? null,
            location: resolved.location,
            room: resolved.room,
            sharedScene: resolved.sharedScene ?? resolved.presence ?? input.presence ?? null,
            currentTime
        };
    }

    function resolvedStateFor(command = {}) { return project(command); }
    function stateShape(command = {}) {
        const value = project(command);
        const appearance = parse(value.appearance_json, {});
        const persisted = state?.read?.({personaId: value.personaId}) ?? {};
        const persistedAppearance = parse(persisted.appearance_json, {});
        const expired = value.resolved_ends_at && Date.parse(value.resolved_ends_at) <= value.currentTime.getTime();
        if ((value.resolved_source !== 'event' || expired) && (persisted.source_event_id || persisted.sourceEventId) && Object.keys(persistedAppearance).length && state?.updateProjection) {
            state.updateProjection({personaId: value.personaId, situation: value.situation, mood: value.mood, appearance: {}, checkpointAt: value.currentTime.toISOString(), updatedAt: value.currentTime.toISOString(), sourceEventId: value.source_event_id, sharedScene: value.sharedScene});
        }
        const sourceId = value.resolved_source_id ?? null;
        return {
            situation: value.situation,
            mood: value.mood,
            appearance,
            scene: value.resolved_scene,
            location: value.resolved_location,
            room: value.resolved_room,
            sharedScene: value.sharedScene,
            sourceId,
            startsAt: value.resolved_starts_at,
            endsAt: value.resolved_ends_at,
            timeFact: value.resolved_time_fact,
            nextBoundaryAt: value.resolved_next_boundary_at,
            sourceEventId: value.source_event_id ?? null,
            source: {kind: value.sourceEvent?.type ?? (value.resolved_source === 'event' ? 'event' : value.resolved_source), sourceId, eventId: value.source_event_id ?? null, startsAt: value.resolved_starts_at, endsAt: value.resolved_ends_at, timeFact: value.resolved_time_fact, nextBoundaryAt: value.resolved_next_boundary_at}
        };
    }
    function scheduledState(command = {}) { return project(command); }
    function sleepAvailability(command = {}) {
        const value = project(command);
        const sleeping = /睡|休息|躺|寝室|卧室/i.test(String(value.situation || '')) || value.resolved_source === 'shared_scene';
        return {sleeping, available: sleeping, nextBoundaryAt: value.resolved_next_boundary_at ?? null, endsAt: value.resolved_ends_at ?? null, timeFact: value.resolved_time_fact};
    }
    function reconcile(command = {}) {
        const value = project(command);
        if (state?.updateProjection) state.updateProjection({personaId: value.personaId, situation: value.situation, mood: value.mood, appearance: value.appearance_json, checkpointAt: value.currentTime.toISOString(), updatedAt: value.currentTime.toISOString(), sourceEventId: value.source_event_id, sharedScene: value.sharedScene});
        return value;
    }
    return Object.freeze({resolvedStateFor, stateShape, scheduledState, sleepAvailability, reconcilePersona: reconcile, recoverPersona: reconcile});
}

export default createLifeStateService;

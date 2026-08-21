function isRecord(value) { return Boolean(value) && typeof value === 'object' && !Array.isArray(value); }
function text(value, field, max = 240, required = false) { const result = typeof value === 'string' ? value.trim() : ''; if (required && !result) throw Object.assign(new TypeError(`${field}不能为空`), {status: 400}); return result.slice(0, max); }
function clockFor(clock) { if (typeof clock === 'function') return clock; if (clock?.now) return clock.now.bind(clock); return () => new Date().toISOString(); }
function idFor(id) { if (typeof id === 'function') return id; if (id?.next) return id.next.bind(id); return prefix => `${prefix}_${crypto.randomUUID()}`; }
function boundedAppearance(value) { if (!isRecord(value)) return {}; return Object.fromEntries(Object.entries(value).slice(0, 6).map(([key, item]) => [text(key, 'appearance key', 32), String(item).slice(0, 120)]).filter(([key, item]) => key && item)); }
function timestamp(value, field) { const date = new Date(value); if (!Number.isFinite(date.getTime())) throw Object.assign(new TypeError(`${field}必须是有效时间`), {status: 400}); return date.toISOString(); }

/** Records a generic life-event fact and returns post-commit projection/effect intents. */
export function createLifeEventFlow({repositories = {}, clock, idGenerator, transaction} = {}) {
    const lifeEvent = repositories.lifeEvent ?? repositories.life;
    const state = repositories.state;
    const activity = repositories.activity;
    const job = repositories.job ?? repositories.jobRepository;
    const persona = repositories.persona;
    const now = clockFor(clock);
    const nextId = idFor(idGenerator);
    if (!lifeEvent?.createEvent && !lifeEvent?.insertEvent) throw new TypeError('Life-event flow requires lifeEvent.createEvent()');
    const createFact = lifeEvent.createEvent ?? lifeEvent.insertEvent;

    function record(command = {}) {
        if (!isRecord(command)) throw Object.assign(new TypeError('Life-event command must be an object'), {status: 400});
        const personaId = text(command.personaId ?? command.persona_id, '人格 ID', 240, true);
        if (persona?.findActive && !persona.findActive(personaId)) throw Object.assign(new Error('人格不存在'), {status: 404});
        const createdAt = timestamp(command.occurredAt ?? command.createdAt ?? now(), '事件时间');
        const type = text(command.type ?? 'routine', '事件类型', 48, true);
        const resolvesAt = command.resolvesAt === undefined || command.resolvesAt === null || command.resolvesAt === '' ? null : timestamp(command.resolvesAt, '事件结束时间');
        if (resolvesAt && Date.parse(resolvesAt) <= Date.parse(createdAt)) throw Object.assign(new Error('事件结束时间必须晚于开始时间'), {status: 400});
        if (resolvesAt && Date.parse(resolvesAt) - Date.parse(createdAt) > 86_400_000) throw Object.assign(new Error('事件持续时间不能超过 24 小时'), {status: 400});
        const payload = {
            situation: text(command.situation ?? '正在忙自己的事', '事件情况', 100),
            mood: text(command.mood ?? '平静', '事件心情', 40),
            scene: text(command.scene ?? '日常场景', '事件场景', 120),
            appearance: boundedAppearance(command.appearance),
            source: text(command.source ?? 'engine', '事件来源', 80),
            rationale: text(command.rationale, '事件原因', 240),
            simulated: Boolean(command.simulated),
            ...(command.sceneRef ? {sceneRef: command.sceneRef} : {}),
            ...(command.eventFamily ? {eventFamily: text(command.eventFamily, 'eventFamily', 48)} : {})
        };
        const fact = createFact({
            id: command.id ?? command.eventId ?? nextId('event'),
            eventId: command.eventId ?? command.id,
            personaId,
            type,
            occurredAt: createdAt,
            resolvesAt,
            causationId: command.causationId ?? command.sourceMessageId ?? null,
            payload,
            payloadJson: JSON.stringify(payload),
            createdAt
        });
        const eventId = fact?.id ?? fact?.eventId ?? command.eventId;
        let stateProjection = null;
        if (state?.updateProjection) {
            const current = state.read?.({personaId});
            if (!current?.sharedScene && !current?.shared_scene_json) {
                stateProjection = state.updateProjection({personaId, situation: payload.situation, mood: payload.mood, appearance: payload.appearance, checkpointAt: createdAt, updatedAt: createdAt, sourceEventId: eventId});
            }
        }
        let activityRow = null;
        if (command.publish === true && activity?.insertActivity) {
            activityRow = activity.insertActivity({id: command.activityId ?? nextId('activity'), personaId, eventId, content: text(command.content ?? `${payload.situation}。`, '动态内容', 900), mediaMode: command.visual ? 'image_set' : 'none', mediaStatus: command.visual ? 'queued' : 'none', createdAt});
        }
        const effects = [];
        if (command.requestActivityDecision === true && job?.enqueue) effects.push(job.enqueue({id: nextId('job'), jobType: 'activity_decision', personaId, priority: 3, maxAttempts: 4, runAfter: createdAt, payload: {eventId}}));
        if (command.proactive === true && job?.enqueue) effects.push(job.enqueue({id: nextId('job'), jobType: 'proactive_message', personaId, priority: 2, maxAttempts: 4, runAfter: createdAt, payload: {eventId, fallbackText: text(command.proactiveText, '主动消息', 500)}}));
        return {eventId, activityId: activityRow?.id ?? null, event: fact, state: stateProjection, effects: effects.filter(Boolean)};
    }

    return Object.freeze({record, createEvent: record, insertEvent: record});
}

export const createLifeEventApplicationFlow = createLifeEventFlow;
export default createLifeEventFlow;

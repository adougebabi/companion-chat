import {randomUUID} from 'node:crypto';

const MAX_EVENT_DURATION_MS = 24 * 60 * 60 * 1_000;
const AUTOMATIC_SOURCES = new Set(['timeline', 'debug', 'engine']);
const SAFE_PREEMPTION_MODES = new Set(['none', 'overlay', 'replace', 'block']);
const INTRODUCIBLE_TYPES = new Set(['class', 'shopping', 'social', 'study']);
const BLOCKED_TERMS = /死亡|自杀|重伤|住院|诊断|手术|犯罪|违法|逮捕|巨额|破产|失业|退学|分手|绝交|怀孕|威胁|勒索|death|suicide|severe injury|hospital|diagnosis|surgery|crime|illegal|arrest|bankrupt|fired|expelled|breakup|blackmail/i;

function isRecord(value) { return Boolean(value) && typeof value === 'object' && !Array.isArray(value); }
function text(value, field, max = 240, required = false) {
    const result = typeof value === 'string' ? value.trim() : '';
    if (required && !result) throw Object.assign(new TypeError(`${field}不能为空`), {status: 400});
    return result.slice(0, max);
}
function clockFor(clock) {
    if (typeof clock === 'function') return () => new Date(clock()).toISOString();
    if (clock?.now) return () => new Date(clock.now()).toISOString();
    return () => new Date().toISOString();
}
function idFor(id) {
    if (typeof id === 'function') return id;
    if (id?.next) return id.next.bind(id);
    return prefix => `${prefix}_${randomUUID()}`;
}
function boundedAppearance(value) {
    if (!isRecord(value)) return {};
    return Object.fromEntries(Object.entries(value).slice(0, 6)
        .filter(([, item]) => ['string', 'number', 'boolean'].includes(typeof item))
        .map(([key, item]) => [text(key, 'appearance key', 32), String(item).trim().slice(0, 120)])
        .filter(([key, item]) => key && item));
}
function timestamp(value, field) {
    const date = new Date(value);
    if (!Number.isFinite(date.getTime())) throw Object.assign(new TypeError(`${field}必须是有效时间`), {status: 400});
    return date.toISOString();
}
function boundedOptional(value, field, max = 160) { return value === undefined || value === null || value === '' ? null : text(value, field, max); }
function json(value, fallback = {}) {
    if (isRecord(value) || Array.isArray(value)) return value;
    try { const parsed = JSON.parse(value || ''); return parsed ?? fallback; } catch { return fallback; }
}
function methodFor(source, names) {
    if (!source) return null;
    if (typeof source === 'function') return source;
    for (const name of names) if (typeof source[name] === 'function') return source[name].bind(source);
    return null;
}
function invoke(source, names, input, {optional = true} = {}) {
    const method = methodFor(source, names);
    if (!method) {
        if (optional) return undefined;
        throw new TypeError(`Life-event port requires ${names[0]}()`);
    }
    const result = method(input);
    if (result && typeof result.then === 'function') throw new TypeError('Life-event flow ports must be synchronous');
    return result;
}
function transactionRunner(transaction, work) {
    if (!transaction) return work();
    if (typeof transaction === 'function') {
        const wrapped = transaction(work);
        return typeof wrapped === 'function' ? wrapped() : wrapped;
    }
    if (typeof transaction.transaction === 'function') {
        const wrapped = transaction.transaction(work);
        return typeof wrapped === 'function' ? wrapped() : wrapped;
    }
    if (typeof transaction.run === 'function') return transaction.run(work);
    throw new TypeError('Life-event transaction must be a function or provide transaction()/run()');
}
function sceneRefFor(value) {
    if (!isRecord(value)) return null;
    const locationId = text(value.locationId ?? value.location_id, '地点 ID', 80);
    const roomId = text(value.roomId ?? value.room_id, '房间 ID', 80);
    if (!locationId || !roomId) throw statusError('sceneRef 必须包含地点和房间');
    return locationId && roomId ? {locationId, roomId} : null;
}
function eventDuration(type, resolvesAt, occurredAt) {
    if (resolvesAt !== undefined) return resolvesAt === null || resolvesAt === '' ? null : timestamp(resolvesAt, '事件结束时间');
    if (!['routine', 'schedule', 'recovery'].includes(type)) return new Date(Date.parse(occurredAt) + 2 * 60 * 60 * 1_000).toISOString();
    return null;
}
function statusError(message, status = 400) { return Object.assign(new Error(message), {status}); }

function activeSharedScene(value) {
    const scene = isRecord(value) ? value : null;
    if (!scene || !text(scene.eventId ?? scene.event_id, '共同场景事件 ID', 80)
        || (!text(scene.location, '共同场景地点', 160) && !text(scene.activity, '共同场景活动', 160))
        || !text(scene.situation, '共同场景情况', 240)) return null;
    if (scene.active === false || ['ended', 'inactive', 'closed', 'expired', 'cancelled', 'canceled'].includes(String(scene.status || '').toLowerCase())) return null;
    return scene;
}

/**
 * Generic life-event application boundary.
 *
 * The flow owns validation, idempotency, policy and fan-out intent creation;
 * repositories only persist their table-scoped rows. A caller-owned
 * transaction keeps the fact, state projection, activity and durable jobs
 * atomic. External providers are represented by frozen effect payloads only.
 */
export function createLifeEventFlow({repositories = {}, clock, idGenerator, transaction, policy = {}} = {}) {
    const lifeEvent = repositories.lifeEvent ?? repositories.life;
    const state = repositories.state;
    const activity = repositories.activity;
    const job = repositories.job ?? repositories.jobRepository;
    const conversation = repositories.conversation ?? repositories.conversationRepository;
    const persona = repositories.persona;
    const characters = repositories.supportingCharacter ?? repositories.supportingCharacters ?? repositories.character;
    const blueprint = repositories.blueprint ?? repositories.lifeBlueprint;
    const personaUpdater = repositories.personaUpdater ?? persona;
    const now = clockFor(clock);
    const nextId = idFor(idGenerator);
    const createFact = methodFor(lifeEvent, ['createEvent', 'insertEvent']);
    if (!createFact) throw new TypeError('Life-event flow requires lifeEvent.createEvent()');

    function activePersona(personaId) {
        const lookup = methodFor(persona, ['findActive', 'findById', 'get']);
        const found = lookup ? lookup(personaId) : null;
        if (lookup && !found) throw statusError('人格不存在', 404);
        return found ?? {id: personaId};
    }
    function ownedParticipants(personaId, candidateIds) {
        const ids = [...new Set((Array.isArray(candidateIds) ? candidateIds : [])
            .map(value => text(value, '参与者 ID', 120)).filter(Boolean))].slice(0, 4);
        if (!ids.length) return [];
        const listed = invoke(characters, ['findOwned', 'listOwned', 'findMany'], {personaId, ids});
        if (Array.isArray(listed)) return listed.map(row => row.id ?? row.characterId ?? row.character_id).filter(Boolean);
        // Never persist caller-provided participant IDs when ownership could
        // not be checked by the injected table-scoped port.
        return [];
    }
    function socialParticipants(personaId) {
        const listed = invoke(characters, ['list', 'listForPersona', 'forPersona'], {personaId, limit: 2});
        return (Array.isArray(listed) ? listed : []).slice(0, 2).map(row => row.id ?? row.characterId ?? row.character_id).filter(Boolean);
    }
    function validateAutomatic(personaId, event, source, blueprintValue) {
        if (!AUTOMATIC_SOURCES.has(source)) return;
        const safetyText = [event.type, event.situation, event.scene, event.content, event.recovery,
            ...(Array.isArray(event.effects) ? event.effects : [])].filter(Boolean).join(' ');
        const blocked = policy.blockedTerms instanceof RegExp ? policy.blockedTerms : BLOCKED_TERMS;
        if (blocked.test(safetyText)) throw statusError('自动事件包含不允许的高风险内容');
        if (event.sceneRef) {
            const scene = invoke(blueprint, ['resolveSceneRef', 'validateSceneRef'], {personaId, sceneRef: event.sceneRef});
            if (scene && !(scene.locationId && scene.roomId) && scene.valid !== true) throw statusError('自动事件引用了无效地点或房间');
            if (!scene) {
                const locations = Array.isArray(blueprintValue?.world?.locations) ? blueprintValue.world.locations : [];
                const location = locations.find(item => item.id === event.sceneRef.locationId);
                const room = location?.rooms?.find(item => item.id === event.sceneRef.roomId);
                if (!location || !room) throw statusError('自动事件引用了无效地点或房间');
            }
        }
        if (event.eventFamily === 'mild_setback' || event.type === 'mild_setback') {
            if (event.reversible !== true || !text(event.recovery, '恢复路径', 240, true)) throw statusError('轻度负向事件必须可逆且具备恢复路径');
        }
        if (!Number.isFinite(event.priority) || event.priority < 0 || event.priority > 100) throw statusError('自动事件优先级无效');
        if (!SAFE_PREEMPTION_MODES.has(event.preemptionMode)) throw statusError('自动事件抢占模式无效');
    }
    function introduceCharacter(personaId, eventId, input, createdAt) {
        if (!input || !INTRODUCIBLE_TYPES.has(input.type) || !characters) return null;
        const name = text(input.name, '新角色名称', 60);
        if (!name) return null;
        return invoke(characters, ['introduce', 'create', 'insert'], {
            id: nextId('support'), personaId, name,
            relationshipKind: text(input.relationshipKind ?? '新认识的朋友', '关系类型', 60),
            profile: {...(isRecord(input.profile) ? input.profile : {}), introducedBy: input.type},
            introducedEventId: eventId, createdAt, updatedAt: createdAt
        });
    }
    function addSupportingComment(activityId, personaId, characterId, type, createdAt) {
        if (!activityId || !characterId || !activity) return null;
        const existing = invoke(activity, ['listActivityComments', 'listComments'], {activityId, personaId, limit: 20}) ?? [];
        if (Array.isArray(existing) && existing.some(row => (row.authorKind ?? row.author_kind) === 'supporting_character')) return null;
        const messages = {social: '今天一起出来真的很放松。', shopping: '这件看起来很适合你！', class: '下次课见。'};
        return invoke(activity, ['insertActivityComment', 'insertComment'], {
            id: nextId('support_comment'), activityId, personaId,
            authorKind: 'supporting_character', supportingCharacterId: characterId,
            content: messages[type] || '今天也辛苦啦。', createdAt
        });
    }
    function proactiveAllowed(personaId, owner, createdAt, command) {
        if (typeof policy.proactiveAllowed === 'function') return policy.proactiveAllowed({personaId, persona: owner, createdAt, command}) !== false;
        if (owner?.screened_at || owner?.screenedAt) return false;
        const recent = invoke(conversation, ['listMessages', 'listForPersona', 'recentForPersona'], {personaId, limit: 100});
        const rows = Array.isArray(recent) ? recent : recent?.items;
        if (!Array.isArray(rows)) return true;
        const cutoff = Date.parse(createdAt) - 10 * 60 * 1_000;
        return !rows.some(row => (row.role ?? row.role_name) === 'user' && Date.parse(row.created_at ?? row.createdAt) >= cutoff);
    }

    function record(command = {}) {
        if (!isRecord(command)) throw statusError('Life-event command must be an object');
        const personaId = text(command.personaId ?? command.persona_id, '人格 ID', 240, true);
        const owner = activePersona(personaId);
        const createdAt = timestamp(command.occurredAt ?? command.createdAt ?? now(), '事件时间');
        const type = text(command.type ?? 'routine', '事件类型', 48, true);
        const source = text(command.source ?? 'engine', '事件来源', 80);
        const blueprintValue = invoke(blueprint, ['read', 'get', 'find'], {personaId}) ?? {};
        const sceneRef = sceneRefFor(command.sceneRef);
        const durationEnd = eventDuration(type, command.resolvesAt, createdAt);
        if (durationEnd && Date.parse(durationEnd) <= Date.parse(createdAt)) throw statusError('事件结束时间必须晚于开始时间');
        if (durationEnd && Date.parse(durationEnd) - Date.parse(createdAt) > MAX_EVENT_DURATION_MS) throw statusError('事件持续时间不能超过 24 小时');
        const priority = command.priority === undefined || command.priority === null || command.priority === '' ? 0 : Number(command.priority);
        if (!Number.isFinite(priority)) throw statusError('事件优先级无效');
        const boundedPriority = AUTOMATIC_SOURCES.has(source) ? priority : Math.max(0, Math.min(100, priority));
        const requestedPreemptionMode = text(command.preemptionMode ?? 'replace', '抢占模式', 32);
        if (AUTOMATIC_SOURCES.has(source) && !SAFE_PREEMPTION_MODES.has(requestedPreemptionMode)) throw statusError('事件抢占模式无效');
        const preemptionMode = SAFE_PREEMPTION_MODES.has(requestedPreemptionMode) ? requestedPreemptionMode : 'replace';
        const reversible = command.reversible === undefined ? true : Boolean(command.reversible);
        const rationale = text(command.rationale ?? '', '事件原因', 240);
        const participants = Array.isArray(command.participantIds)
            ? ownedParticipants(personaId, command.participantIds)
            : type === 'social' ? socialParticipants(personaId) : [];
        const eventId = command.eventId ?? command.id ?? nextId('event');
        const idempotencyKey = boundedOptional(command.idempotencyKey, 'idempotencyKey');
        const payload = {
            situation: text(command.situation ?? '正在忙自己的事', '事件情况', 100),
            mood: text(command.mood ?? '平静', '事件心情', 40),
            scene: text(command.scene ?? '日常场景', '事件场景', 120),
            appearance: boundedAppearance(command.appearance), source, simulated: Boolean(command.simulated), rationale,
            participants, priority: boundedPriority, reversible, preemptionMode,
            ...(sceneRef ? {sceneRef} : {}),
            ...(boundedOptional(command.eventFamily, 'eventFamily', 48) ? {eventFamily: boundedOptional(command.eventFamily, 'eventFamily', 48)} : {}),
            ...(idempotencyKey ? {idempotencyKey} : {}),
            ...(boundedOptional(command.templateId, 'templateId', 120) ? {templateId: boundedOptional(command.templateId, 'templateId', 120)} : {}),
            ...(boundedOptional(command.decisionId, 'decisionId', 160) ? {decisionId: boundedOptional(command.decisionId, 'decisionId', 160)} : {}),
            ...(boundedOptional(command.slotId, 'slotId', 160) ? {slotId: boundedOptional(command.slotId, 'slotId', 160)} : {}),
            ...(command.recovery ? {recovery: text(command.recovery, '恢复路径', 240)} : {})
        };
        validateAutomatic(personaId, {type, ...command, ...payload}, source, blueprintValue);
        if (Buffer.byteLength(JSON.stringify(payload), 'utf8') > 4_096) throw statusError('事件数据超过允许大小');
        const effects = [];
        let fact = null;
        let stateProjection = null;
        let activityRow = null;
        let introduced = null;
        const run = () => {
            if (idempotencyKey) {
                const existing = invoke(lifeEvent, ['findByIdempotencyKey'], {personaId, idempotencyKey});
                if (existing) return {eventId: existing.id, event: existing, state: null, activityId: null, effects: [], replayed: true};
            }
            fact = createFact.call(lifeEvent, {
                id: eventId, eventId, personaId, type, occurredAt: createdAt,
                resolvesAt: durationEnd, causationId: command.causationId ?? command.sourceMessageId ?? null,
                payload, payloadJson: JSON.stringify(payload), createdAt
            });
            introduced = introduceCharacter(personaId, fact?.id ?? eventId,
                isRecord(command.introducedCharacter) ? {...command.introducedCharacter, type} : null,
                createdAt);
            if (introduced?.id) {
                payload.participants = [...new Set([...participants, introduced.id])];
                invoke(lifeEvent, ['updateEvent'], {eventId: fact?.id ?? eventId, personaId, payload, payloadJson: JSON.stringify(payload)});
            }
            const current = invoke(state, ['read', 'get', 'findByPersona'], {personaId});
            const sharedScene = activeSharedScene(current?.sharedScene ?? json(current?.shared_scene_json, null));
            if (!sharedScene) {
                stateProjection = invoke(state, ['updateProjection', 'updateState', 'applyProjection'], {
                    personaId, situation: payload.situation, mood: payload.mood, appearance: payload.appearance,
                    checkpointAt: createdAt, updatedAt: createdAt, sourceEventId: fact?.id ?? eventId
                });
            }
            invoke(personaUpdater, ['touch', 'updateTimestamp', 'touchUpdatedAt'], {personaId, updatedAt: createdAt});
            if (command.publish === true) {
                activityRow = invoke(activity, ['insertActivity', 'create'], {
                    id: command.activityId ?? nextId('activity'), personaId, eventId: fact?.id ?? eventId,
                    content: text(command.content ?? `${owner.name ?? ''}${payload.situation}。`, '动态内容', 900),
                    mediaMode: command.visual ? (command.mediaCapabilityCall?.kind === 'video' ? 'video' : 'image_set') : 'none',
                    mediaStatus: command.visual ? 'queued' : 'none', createdAt
                });
                const characterId = payload.participants?.[0];
                if (characterId) addSupportingComment(activityRow?.id, personaId, characterId, type, createdAt);
                if (command.visual) {
                    const capability = isRecord(command.mediaCapabilityCall) ? command.mediaCapabilityCall : null;
                    const frozenCapability = capability
                        && ['image', 'video'].includes(capability.kind)
                        && isRecord(capability.personaMediaConcept)
                        && Object.hasOwn(capability, 'currentEvent')
                        && isRecord(capability.temporaryAppearance)
                        && Number.isInteger(Number(capability.count ?? 1))
                        && Number(capability.count ?? 1) >= 1
                        && Number(capability.count ?? 1) <= 3;
                    if (!frozenCapability) {
                        if (activityRow?.id) invoke(activity, ['updateActivity'], {id: activityRow.id, activityId: activityRow.id, personaId, mediaStatus: 'failed'});
                    } else {
                        effects.push({effectId: nextId('effect'), kind: capability.kind === 'video' ? 'activity_video' : 'activity_image',
                            idempotencyKey: `activity:${activityRow?.id}:${capability.kind}`, causationId: fact?.id ?? eventId,
                            payload: {
                                activityId: activityRow?.id, eventId: fact?.id ?? eventId, personaId,
                                kind: capability.kind, request: capability.request ?? '', count: capability.count ?? 1,
                                personaMediaConcept: capability.personaMediaConcept,
                                capabilityCall: capability
                            }});
                    }
                }
            }
            if (command.requestActivityDecision === true) effects.push({effectId: nextId('effect'), kind: 'activity_decision', idempotencyKey: `activity-decision:${fact?.id ?? eventId}`, causationId: fact?.id ?? eventId, payload: {eventId: fact?.id ?? eventId, personaId}});
            if (command.proactive === true && proactiveAllowed(personaId, owner, createdAt, command)) effects.push({effectId: nextId('effect'), kind: 'proactive_message', idempotencyKey: `proactive:${fact?.id ?? eventId}`, causationId: fact?.id ?? eventId, payload: {eventId: fact?.id ?? eventId, personaId, fallbackText: text(command.proactiveText ?? `${payload.situation}，忽然想和你说一声。`, '主动消息', 500)}});
            if (job?.enqueue) {
                for (const effect of effects) {
                    const queued = job.enqueue({
                        id: nextId('job'), jobType: effect.kind, personaId,
                        activityId: effect.payload?.activityId ?? null,
                        messageId: effect.payload?.messageId ?? null,
                        priority: effect.kind === 'proactive_message' ? 2 : 3,
                        maxAttempts: 4, runAfter: createdAt, payload: effect.payload
                    });
                    effect.job = queued;
                }
            }
            return {eventId: fact?.id ?? eventId, activityId: activityRow?.id ?? null, event: fact, state: stateProjection, effects, replayed: false, introducedCharacter: introduced ?? null};
        };
        return transactionRunner(transaction, run);
    }

    return Object.freeze({record, createEvent: record, insertEvent: record});
}

export const createLifeEventApplicationFlow = createLifeEventFlow;
export default createLifeEventFlow;

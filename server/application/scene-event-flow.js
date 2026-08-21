import {randomUUID} from 'node:crypto';

export const SCENE_EVENT_FLOW_VERSION = 1;
export const SCENE_EVENT_SCHEMA_VERSION = 1;
export const SCENE_EVENT_OPERATIONS = Object.freeze(['start', 'switch', 'end']);

const PRIVATE_PLAN = new WeakMap();

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function requiredText(value, field, maxLength = 240) {
    if (typeof value !== 'string' || value.trim() === '') throw new TypeError(`${field} must be a non-empty string`);
    const text = value.trim();
    if (text.length > maxLength) throw new RangeError(`${field} exceeds ${maxLength} characters`);
    return text;
}

function sceneInputText(value, field, limit) {
    if (value === undefined) return '';
    if (typeof value !== 'string') throw new TypeError(`${field} must be a string`);
    const text = value.trim();
    if (text.length > limit) throw new RangeError(`${field} exceeds ${limit} characters`);
    return text;
}

function boundedText(value, field, limit, {allowEmpty = false} = {}) {
    if (typeof value !== 'string') throw new TypeError(`${field} must be a string`);
    const text = value.trim();
    if (!allowEmpty && !text) throw new TypeError(`${field} must not be empty`);
    if (text.length > limit) throw new RangeError(`${field} exceeds ${limit} characters`);
    return text;
}

function timestamp(value, field) {
    const resolved = value instanceof Date ? value.toISOString() : value;
    if (typeof resolved !== 'string' || resolved.trim() === '' || !Number.isFinite(Date.parse(resolved))) {
        throw new TypeError(`${field} must be a valid timestamp`);
    }
    return resolved;
}

function clockFunction(clock) {
    if (clock === undefined) return () => new Date().toISOString();
    if (typeof clock === 'function') return () => timestamp(clock(), 'Scene-event clock value');
    if (isRecord(clock) && typeof clock.now === 'function') return () => timestamp(clock.now(), 'Scene-event clock value');
    throw new TypeError('Scene-event flow clock must be a function or provide now()');
}

function idFunction(idGenerator) {
    if (idGenerator === undefined) return prefix => `${prefix}_${randomUUID()}`;
    if (typeof idGenerator === 'function') return prefix => requiredText(idGenerator(prefix), 'Generated scene-event id');
    if (isRecord(idGenerator) && typeof idGenerator.next === 'function') return prefix => requiredText(idGenerator.next(prefix), 'Generated scene-event id');
    throw new TypeError('Scene-event flow idGenerator must be a function or provide next()');
}

function deepFreeze(value) {
    if (!value || typeof value !== 'object' || Object.isFrozen(value)) return value;
    Object.freeze(value);
    for (const child of Object.values(value)) deepFreeze(child);
    return value;
}

function resolveRepository(repositories, names, field, {optional = false} = {}) {
    const source = isRecord(repositories) ? repositories : {};
    for (const name of names) {
        if (source[name] !== undefined) {
            if (!isRecord(source[name]) && typeof source[name] !== 'function') throw new TypeError(`Scene-event flow ${field} must be an object`);
            return source[name];
        }
    }
    if (optional) return null;
    throw new TypeError(`Scene-event flow requires ${field}`);
}

function methodFor(repository, names, field, {optional = false} = {}) {
    if (typeof repository === 'function') return repository;
    if (isRecord(repository)) {
        for (const name of names) {
            if (repository[name] !== undefined) {
                if (typeof repository[name] !== 'function') throw new TypeError(`Scene-event flow ${field}.${name} must be a function`);
                return repository[name].bind(repository);
            }
        }
    }
    if (optional) return null;
    throw new TypeError(`Scene-event flow ${field} must provide ${names.join('() or ')}()`);
}

function normalizeSceneItems(value, field, limit, allowed = null) {
    if (value === undefined) return undefined;
    if (!Array.isArray(value) || value.length > limit) throw new TypeError(`${field} must be an array of at most ${limit} items`);
    if (allowed) {
        if (value.some(item => !allowed.includes(item))) throw new TypeError(`${field} contains an unsupported participant`);
        return [...new Set(value)];
    }
    if (value.some(item => typeof item !== 'string' || item.trim().length > 80)) throw new TypeError(`${field} contains an invalid item`);
    return value.map(item => item.trim()).filter(Boolean);
}

/** Normalize the decoded scene_event call without parsing provider streams. */
export function normalizeSceneEventCall(value) {
    if (!isRecord(value)) throw new Error('scene_event call must be an object');
    const allowed = ['operation', 'location', 'room', 'activity', 'situation', 'mood', 'objects', 'participants'];
    if (Object.keys(value).some(key => !allowed.includes(key))) throw new Error('scene_event call contains unsupported fields');
    const operation = value.operation;
    if (!SCENE_EVENT_OPERATIONS.includes(operation)) throw new Error('scene_event operation is invalid');
    const location = sceneInputText(value.location, 'scene_event.location', 160);
    const room = sceneInputText(value.room, 'scene_event.room', 120);
    const activity = sceneInputText(value.activity, 'scene_event.activity', 160);
    const situation = sceneInputText(value.situation, 'scene_event.situation', 240);
    const mood = sceneInputText(value.mood, 'scene_event.mood', 80);
    const objects = normalizeSceneItems(value.objects, 'scene_event.objects', 12) ?? [];
    const participants = normalizeSceneItems(value.participants, 'scene_event.participants', 2, ['user', 'persona']) ?? ['user', 'persona'];
    if (operation === 'end') return Object.freeze({operation});
    if ((!location && !activity) || !situation) throw new Error('scene_event start/switch requires location or activity and situation');
    return Object.freeze({operation, location, room, activity, situation, mood, objects, participants: participants.length ? participants : ['user', 'persona']});
}

function normalizeInjectedCall(normalizer, value) {
    const normalized = normalizer(value);
    if (normalized && typeof normalized.then === 'function') throw new TypeError('Scene-event normalizer must be synchronous');
    return normalizeSceneEventCall(normalized);
}

function provenanceFor(value = {}) {
    if (!isRecord(value)) throw new TypeError('Scene-event provenance must be an object');
    const source = value.source === undefined ? 'scene_event' : boundedText(value.source, 'Scene-event provenance source', 80);
    const callId = value.callId ?? value.call_id;
    const idempotencyKey = value.idempotencyKey ?? value.idempotency_key;
    return Object.freeze({
        source,
        ...(callId ? {callId: boundedText(callId, 'Scene-event provenance callId', 160)} : {}),
        ...(idempotencyKey ? {idempotencyKey: boundedText(idempotencyKey, 'Scene-event idempotencyKey', 160)} : {})
    });
}

function rowValue(row, camel, snake) {
    return row?.[camel] === undefined ? row?.[snake] : row[camel];
}

function sourceMessageId(row) {
    return rowValue(row, 'id', 'message_id') ?? row?.messageId;
}

function sourcePersonaId(row) {
    return rowValue(row, 'personaId', 'persona_id') ?? row?.ownerPersonaId;
}

function sourceRole(row) {
    return row?.role ?? row?.authorRole ?? row?.author_role;
}

function normalizeSource(row, personaId, expectedId) {
    if (!isRecord(row) || sourceMessageId(row) !== expectedId) throw new Error('scene_event source message does not exist');
    if (sourcePersonaId(row) !== personaId) throw new Error('scene_event source message does not belong to persona');
    if (sourceRole(row) !== 'user') throw new Error('scene_event source message must be user-owned');
    return Object.freeze({id: expectedId, personaId, role: 'user'});
}

function parseJson(value, fallback = {}) {
    if (isRecord(value) || Array.isArray(value)) return value;
    if (typeof value !== 'string' || !value.trim()) return fallback;
    try {
        const parsed = JSON.parse(value);
        return parsed === null ? fallback : parsed;
    } catch {
        return fallback;
    }
}

function normalizeScene(value) {
    if (!isRecord(value)) return null;
    const eventId = boundedText(value.eventId ?? value.event_id ?? '', 'Scene.eventId', 80, {allowEmpty: true});
    const location = boundedText(value.location ?? '', 'Scene.location', 160, {allowEmpty: true});
    const room = boundedText(value.room ?? '', 'Scene.room', 120, {allowEmpty: true});
    const activity = boundedText(value.activity ?? '', 'Scene.activity', 160, {allowEmpty: true});
    const situation = boundedText(value.situation ?? '', 'Scene.situation', 240, {allowEmpty: true});
    if (!eventId || ((!location && !activity) || !situation)) return null;
    return {
        location, room, activity, situation,
        mood: boundedText(value.mood ?? '', 'Scene.mood', 80, {allowEmpty: true}),
        objects: Array.isArray(value.objects) ? value.objects.map(item => boundedText(item, 'Scene.object', 80)).slice(0, 12) : [],
        participants: Array.isArray(value.participants) ? [...new Set(value.participants.filter(item => ['user', 'persona'].includes(item)))].slice(0, 2) : ['user', 'persona'],
        startedAt: boundedText(value.startedAt ?? value.started_at ?? '', 'Scene.startedAt', 80, {allowEmpty: true}),
        eventId
    };
}

function projectionSnapshot(row) {
    const rawJson = rowValue(row, 'sharedSceneJson', 'shared_scene_json') || '{}';
    const storedScene = row?.sharedScene ?? row?.shared_scene;
    return {
        sourceEventId: rowValue(row, 'sourceEventId', 'source_event_id') || null,
        sharedSceneJson: rawJson,
        sharedScene: normalizeScene(typeof storedScene === 'string' ? parseJson(storedScene, null) : storedScene ?? parseJson(rawJson, null))
    };
}

function sameSceneProjection(left, right) {
    return JSON.stringify(left || null) === JSON.stringify(right || null);
}

function eventPayload(row) {
    return parseJson(rowValue(row, 'payloadJson', 'payload_json'), {});
}

function eventOperation(row) {
    const payload = eventPayload(row);
    return SCENE_EVENT_OPERATIONS.includes(payload.operation)
        ? payload.operation
        : row?.type === 'shared_scene_end' ? 'end' : 'start';
}

function eventReplayResult(personaId, existing, stateRead) {
    const payload = eventPayload(existing);
    const operation = eventOperation(existing);
    const current = stateRead ? projectionSnapshot(stateRead(personaId)) : null;
    return {
        eventId: existing.id,
        operation,
        scene: operation === 'end' ? null : payload.nextScene || current?.sharedScene || null,
        previousScene: payload.previousScene || null,
        replayed: true
    };
}

function idempotentEventLookup(lifeEventFind, personaId, idempotencyKey) {
    if (!idempotencyKey) return null;
    if (!lifeEventFind) throw new TypeError('Scene-event flow requires lifeEvent.findByIdempotencyKey() for idempotent calls');
    const existing = lifeEventFind.length >= 2
        ? lifeEventFind(personaId, idempotencyKey)
        : lifeEventFind({personaId, idempotencyKey});
    if (existing && typeof existing.then === 'function') throw new TypeError('Scene-event idempotency lookup must be synchronous');
    return existing || null;
}

function transactionRunner(transaction, work) {
    if (!transaction) return work();
    if (typeof transaction === 'function') {
        const result = transaction(work);
        return typeof result === 'function' ? result() : result;
    }
    if (isRecord(transaction) && typeof transaction.transaction === 'function') {
        const result = transaction.transaction(work);
        return typeof result === 'function' ? result() : result;
    }
    if (isRecord(transaction) && typeof transaction.run === 'function') return transaction.run(work);
    throw new TypeError('Scene-event transaction must be a function or provide transaction()/run()');
}

function assertPlan(plan) {
    if (!isRecord(plan) || !PRIVATE_PLAN.has(plan)) throw new TypeError('scene_event plan is invalid');
    return PRIVATE_PLAN.get(plan);
}

function personaFor(personaLookup, personaId, supplied) {
    if (!personaLookup) {
        if (!isRecord(supplied)) throw new TypeError('Scene-event flow requires persona repository or command.persona');
        return supplied;
    }
    const persona = personaLookup(personaId);
    if (persona && typeof persona.then === 'function') throw new TypeError('Scene-event persona lookup must be synchronous');
    if (!persona) throw new Error('scene_event persona does not exist');
    return persona;
}

function sourceFor(sourceLookup, command, personaId, messageId) {
    if (!sourceLookup) {
        if (!isRecord(command.sourceMessage)) throw new TypeError('Scene-event flow requires message repository or command.sourceMessage');
        return normalizeSource(command.sourceMessage, personaId, messageId);
    }
    const source = sourceLookup.length >= 2
        ? sourceLookup(messageId, {personaId})
        : sourceLookup({id: messageId, messageId, personaId});
    if (source && typeof source.then === 'function') throw new TypeError('Scene-event source lookup must be synchronous');
    return normalizeSource(source, personaId, messageId);
}

export function createSceneEventFlow({
    repositories,
    clock,
    idGenerator,
    normalizeCall,
    normalizeSceneEventCall: injectedNormalizer,
    scheduledState,
    transaction
} = {}) {
    if (!isRecord(repositories)) throw new TypeError('Scene-event flow repositories must be an object');
    const personaRepository = resolveRepository(repositories, ['personaRepository', 'personas', 'persona'], 'persona repository', {optional: true});
    const messageRepository = resolveRepository(repositories, ['messageRepository', 'sourceMessageRepository', 'messages', 'conversationRepository', 'conversation'], 'message repository', {optional: true});
    const lifeEventRepository = resolveRepository(repositories, ['lifeEventRepository', 'lifeEvent', 'life'], 'life-event repository');
    const stateRepository = resolveRepository(repositories, ['stateRepository', 'personaStateRepository', 'state'], 'state repository');
    const personaLookup = personaRepository ? methodFor(personaRepository, ['findActive', 'findById', 'requirePersona', 'get'], 'persona repository', {optional: true}) : null;
    const sourceLookup = messageRepository ? methodFor(messageRepository, ['findById', 'findMessage', 'findSourceMessage', 'getMessage', 'getById'], 'message repository', {optional: true}) : null;
    const lifeEventCreate = methodFor(lifeEventRepository, ['createEvent', 'insertEvent', 'record'], 'life-event repository');
    const lifeEventFind = methodFor(lifeEventRepository, ['findByIdempotencyKey', 'findByIdempotency', 'findByDedupeKey', 'findByPayload', 'findByProvenance'], 'life-event repository', {optional: true});
    const stateRead = methodFor(stateRepository, ['readProjection', 'read', 'findState', 'findByPersona', 'get'], 'state repository');
    const stateUpdate = methodFor(stateRepository, ['updateProjection', 'applyProjection', 'setProjection', 'updateState', 'update'], 'state repository');
    const personaTouch = personaRepository ? methodFor(personaRepository, ['touch', 'updateUpdatedAt', 'touchUpdatedAt'], 'persona repository', {optional: true}) : null;
    const now = clockFunction(clock);
    const generateId = idFunction(idGenerator);
    const normalizer = normalizeCall ?? injectedNormalizer;
    if (normalizer !== undefined && typeof normalizer !== 'function') throw new TypeError('Scene-event flow normalizeCall must be a function');
    if (scheduledState !== undefined && typeof scheduledState !== 'function') throw new TypeError('Scene-event flow scheduledState must be a function');

    function plan(input, value, causationId, provenance = {}) {
        const command = typeof input === 'string'
            ? {personaId: input, call: value, sourceMessageId: causationId, provenance}
            : input;
        if (!isRecord(command)) throw new TypeError('Scene-event command must be an object');
        const personaId = requiredText(command.personaId ?? command.persona_id, 'Scene-event personaId');
        const persona = personaFor(personaLookup, personaId, command.persona);
        const sourceMessage = requiredText(command.sourceMessageId ?? command.source_message_id ?? command.causationId ?? command.causation_id, 'Scene-event sourceMessageId');
        const source = sourceFor(sourceLookup, command, personaId, sourceMessage);
        const callValue = command.call ?? command.value ?? command.sceneEvent ?? command.scene_event ?? command.arguments;
        if (callValue === undefined) throw new TypeError('Scene-event call is required');
        const call = normalizer ? normalizeInjectedCall(normalizer, callValue) : normalizeSceneEventCall(callValue);
        const provenanceValue = provenanceFor(command.provenance ?? command);
        const existing = idempotentEventLookup(lifeEventFind, personaId, provenanceValue.idempotencyKey);
        const projection = projectionSnapshot(stateRead(personaId));
        const existingPayload = existing ? eventPayload(existing) : null;
        const replayed = Boolean(existing);
        const replayOperation = existing ? eventOperation(existing) : null;
        const operation = replayed && replayOperation ? replayOperation : call.operation;
        const previousScene = replayed ? existingPayload.previousScene || null : projection.sharedScene;
        const createdAt = timestamp(rowValue(existing, 'createdAt', 'created_at') || now(), 'Scene-event createdAt');
        const eventId = existing?.id || generateId('event');
        const fallback = replayed || !scheduledState ? null : scheduledState(persona, new Date(createdAt));
        if (fallback && typeof fallback.then === 'function') throw new TypeError('Scene-event scheduledState must be synchronous');
        const nextScene = replayed
            ? (operation === 'end' ? null : existingPayload.nextScene || projection.sharedScene)
            : operation === 'end' ? null : {
                location: call.location,
                room: call.room,
                activity: call.activity,
                situation: call.situation || fallback?.situation || '',
                mood: call.mood || fallback?.mood || '平静',
                objects: call.objects || [],
                participants: call.participants || projection.sharedScene?.participants || ['user', 'persona'],
                startedAt: createdAt,
                eventId
            };
        if (!replayed && operation !== 'end' && !nextScene.situation) throw new Error('Scene-event requires a situation after scheduledState fallback');
        const payload = replayed ? existingPayload : {
            schemaVersion: SCENE_EVENT_SCHEMA_VERSION,
            operation: call.operation,
            source: provenanceValue.source,
            causationId: source.id,
            eventId,
            ...(provenanceValue.callId ? {capabilityCallId: provenanceValue.callId} : {}),
            ...(provenanceValue.idempotencyKey ? {idempotencyKey: provenanceValue.idempotencyKey} : {}),
            location: nextScene?.location || '',
            room: nextScene?.room || '',
            activity: nextScene?.activity || '',
            situation: nextScene?.situation || fallback?.situation || '',
            mood: nextScene?.mood || fallback?.mood || '平静',
            objects: nextScene?.objects || [],
            participants: nextScene?.participants || projection.sharedScene?.participants || ['user', 'persona'],
            startedAt: nextScene?.startedAt || null,
            nextScene,
            previousScene: call.operation === 'switch' || call.operation === 'end' ? projection.sharedScene : null
        };
        const expected = {
            sourceEventId: projection.sourceEventId,
            sharedSceneJson: projection.sharedSceneJson,
            sharedScene: previousScene
        };
        const preview = {eventId, operation, scene: nextScene, previousScene, replayed};
        const planValue = deepFreeze({
            type: 'scene_event_plan',
            version: SCENE_EVENT_FLOW_VERSION,
            personaId,
            sourceMessageId: source.id,
            call,
            provenance: provenanceValue,
            operation,
            eventId,
            createdAt,
            preallocatedIds: {eventId},
            idempotencyKey: provenanceValue.idempotencyKey || '',
            expected,
            scene: nextScene,
            previousScene,
            nextScene,
            payload,
            preview,
            previewResult: preview,
            replayed,
            replayCandidate: replayed
        });
        PRIVATE_PLAN.set(planValue, {persona, source, call, provenance: provenanceValue, expected, previousScene, nextScene, payload, eventId, createdAt, operation});
        return planValue;
    }

    function applyWithin(planValue, state) {
        const persona = personaFor(personaLookup, planValue.personaId, state.persona);
        const source = sourceFor(sourceLookup, {sourceMessage: state.source}, planValue.personaId, planValue.sourceMessageId);
        const existing = idempotentEventLookup(lifeEventFind, planValue.personaId, state.provenance.idempotencyKey);
        if (existing) return eventReplayResult(planValue.personaId, existing, stateRead);
        const current = projectionSnapshot(stateRead(planValue.personaId));
        const expected = state.expected;
        if (current.sourceEventId !== (expected.sourceEventId || null)
            || current.sharedSceneJson !== (expected.sharedSceneJson || '{}')
            || !sameSceneProjection(current.sharedScene, expected.sharedScene)) {
            throw new Error('scene_event plan does not match the current scene projection');
        }
        if (!state.eventId || !state.createdAt || !state.operation || !isRecord(state.payload)) throw new TypeError('scene_event plan contents are invalid');
        const nextScene = state.operation === 'end' ? null : state.nextScene || null;
        const event = lifeEventCreate({
            eventId: state.eventId,
            id: state.eventId,
            personaId: planValue.personaId,
            type: state.operation === 'end' ? 'shared_scene_end' : 'shared_scene',
            occurredAt: state.createdAt,
            resolvesAt: null,
            causationId: source.id,
            payload: state.payload,
            createdAt: state.createdAt
        });
        if (event && typeof event.then === 'function') throw new TypeError('Scene-event repository writes must be synchronous');
        const update = stateUpdate({
            personaId: planValue.personaId,
            situation: state.payload.situation || '',
            mood: state.payload.mood || '平静',
            checkpointAt: state.createdAt,
            updatedAt: state.createdAt,
            sourceEventId: state.eventId,
            sharedScene: nextScene,
            sharedSceneJson: JSON.stringify(nextScene || {}),
            expected
        });
        if (update && typeof update.then === 'function') throw new TypeError('Scene-event state writes must be synchronous');
        if (update?.changes !== undefined && update.changes !== 1) throw new Error('scene_event plan does not match the current state projection');
        if (update?.changed !== undefined && !update.changed) throw new Error('scene_event plan does not match the current state projection');
        if (personaTouch) {
            const touched = personaTouch({personaId: planValue.personaId, updatedAt: state.createdAt});
            if (touched && typeof touched.then === 'function') throw new TypeError('Scene-event persona writes must be synchronous');
        }
        return {eventId: state.eventId, operation: state.operation, scene: nextScene, previousScene: state.previousScene || null};
    }

    function apply(planValue, options = {}) {
        const state = assertPlan(planValue);
        const settings = typeof options === 'function' ? {transaction: options} : options;
        if (!isRecord(settings)) throw new TypeError('Scene-event apply options must be an object');
        const runner = settings.transaction ?? settings.callerTransaction ?? settings.runInTransaction ?? settings.commit ?? transaction;
        return transactionRunner(runner, () => applyWithin(planValue, state));
    }

    return Object.freeze({
        version: SCENE_EVENT_FLOW_VERSION,
        plan,
        apply,
        normalizeSceneEventCall: normalizer || normalizeSceneEventCall,
        repositories: Object.freeze({persona: personaRepository, message: messageRepository, lifeEvent: lifeEventRepository, state: stateRepository})
    });
}

export const createCompanionSceneEventFlow = createSceneEventFlow;
export default createSceneEventFlow;

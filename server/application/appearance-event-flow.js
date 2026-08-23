import {randomUUID} from 'node:crypto';

export const APPEARANCE_EVENT_FLOW_VERSION = 1;
export const APPEARANCE_EVENT_SCHEMA_VERSION = 1;
export const APPEARANCE_EVENT_OPERATIONS = Object.freeze(['set', 'clear']);
export const APPEARANCE_EVENT_MAX_OUTFIT_LENGTH = 240;
export const APPEARANCE_EVENT_MAX_REASON_LENGTH = 240;

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

function optionalText(value, field, maxLength) {
    if (value === undefined) return undefined;
    if (typeof value !== 'string') throw new TypeError(`${field} must be a string`);
    const text = value.trim();
    if (!text) return undefined;
    if (text.length > maxLength) throw new RangeError(`${field} exceeds ${maxLength} characters`);
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
    if (typeof clock === 'function') return () => timestamp(clock(), 'Appearance-event clock value');
    if (isRecord(clock) && typeof clock.now === 'function') return () => timestamp(clock.now(), 'Appearance-event clock value');
    throw new TypeError('Appearance-event flow clock must be a function or provide now()');
}

function idFunction(idGenerator) {
    if (idGenerator === undefined) return prefix => `${prefix}_${randomUUID()}`;
    if (typeof idGenerator === 'function') return prefix => requiredText(idGenerator(prefix), 'Generated appearance-event id');
    if (isRecord(idGenerator) && typeof idGenerator.next === 'function') {
        return prefix => requiredText(idGenerator.next(prefix), 'Generated appearance-event id');
    }
    throw new TypeError('Appearance-event flow idGenerator must be a function or provide next()');
}

function deepFreeze(value) {
    if (!value || typeof value !== 'object' || Object.isFrozen(value)) return value;
    Object.freeze(value);
    for (const child of Object.values(value)) deepFreeze(child);
    return value;
}

function cloneValue(value) {
    if (Array.isArray(value)) return value.map(cloneValue);
    if (isRecord(value)) return Object.fromEntries(Object.entries(value).map(([key, child]) => [key, cloneValue(child)]));
    return value;
}

function resolveRepository(repositories, names, field, {optional = false} = {}) {
    const source = isRecord(repositories) ? repositories : {};
    for (const name of names) {
        if (source[name] !== undefined) {
            if (!isRecord(source[name]) && typeof source[name] !== 'function') throw new TypeError(`Appearance-event flow ${field} must be an object`);
            return source[name];
        }
    }
    if (optional) return null;
    throw new TypeError(`Appearance-event flow requires ${field}`);
}

function methodFor(repository, names, field, {optional = false} = {}) {
    if (typeof repository === 'function') return repository;
    if (isRecord(repository)) {
        for (const name of names) {
            if (repository[name] !== undefined) {
                if (typeof repository[name] !== 'function') throw new TypeError(`Appearance-event flow ${field}.${name} must be a function`);
                return repository[name].bind(repository);
            }
        }
    }
    if (optional) return null;
    throw new TypeError(`Appearance-event flow ${field} must provide ${names.join('() or ')}()`);
}

function sync(value, field) {
    if (value && typeof value.then === 'function') throw new TypeError(`${field} must be synchronous`);
    return value;
}

function rowValue(row, camel, snake) {
    return row?.[camel] === undefined ? row?.[snake] : row[camel];
}

function parseJson(value, fallback = {}) {
    if (isRecord(value)) return cloneValue(value);
    if (typeof value !== 'string' || !value.trim()) return fallback;
    try {
        const parsed = JSON.parse(value);
        return isRecord(parsed) ? parsed : fallback;
    } catch {
        return fallback;
    }
}

function appearanceFor(row) {
    const raw = rowValue(row, 'appearanceJson', 'appearance_json');
    const stored = row?.appearance;
    const appearance = isRecord(stored) ? stored : parseJson(raw, {});
    return {
        sourceEventId: rowValue(row, 'sourceEventId', 'source_event_id') || null,
        situation: row?.situation ?? row?.resolvedSituation ?? row?.resolved_situation ?? '',
        mood: row?.mood ?? '',
        appearance: cloneValue(appearance),
        appearanceJson: typeof raw === 'string' && raw.trim() ? raw : JSON.stringify(appearance)
    };
}

function sameAppearance(left, right) {
    return JSON.stringify(left || {}) === JSON.stringify(right || {});
}

function provenanceFor(value = {}, call) {
    if (!isRecord(value)) throw new TypeError('Appearance-event provenance must be an object');
    const source = value.source ?? call?.source ?? 'appearance_event';
    const callId = value.callId ?? value.call_id ?? call?.callId ?? call?.call_id;
    const idempotencyKey = value.idempotencyKey ?? value.idempotency_key
        ?? call?.idempotencyKey ?? call?.idempotency_key;
    return Object.freeze({
        source: requiredText(source, 'Appearance-event provenance source', 80),
        ...(callId === undefined || callId === null || callId === ''
            ? {} : {callId: requiredText(callId, 'Appearance-event provenance callId', 160)}),
        ...(idempotencyKey === undefined || idempotencyKey === null || idempotencyKey === ''
            ? {} : {idempotencyKey: requiredText(idempotencyKey, 'Appearance-event idempotencyKey', 240)})
    });
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
    if (!isRecord(row) || sourceMessageId(row) !== expectedId) throw new Error('appearance_event source message does not exist');
    if (sourcePersonaId(row) !== personaId) throw new Error('appearance_event source message does not belong to persona');
    if (sourceRole(row) !== 'user') throw new Error('appearance_event source message must be user-owned');
    return Object.freeze({id: expectedId, personaId, role: 'user'});
}

function personaFor(personaLookup, personaId, supplied) {
    if (!personaLookup) {
        if (!isRecord(supplied)) throw new TypeError('Appearance-event flow requires persona repository or command.persona');
        const suppliedId = supplied.id ?? supplied.personaId ?? supplied.persona_id;
        if (suppliedId !== undefined && suppliedId !== personaId) throw new Error('appearance_event persona does not belong to the command');
        return supplied;
    }
    const persona = sync(personaLookup(personaId), 'Appearance-event persona lookup');
    if (!persona) throw new Error('appearance_event persona does not exist');
    const returnedId = persona.id ?? persona.personaId ?? persona.persona_id;
    if (returnedId !== undefined && returnedId !== personaId) throw new Error('appearance_event persona does not belong to the command');
    return persona;
}

function sourceFor(sourceLookup, command, personaId, messageId) {
    if (!sourceLookup) {
        if (!isRecord(command.sourceMessage)) throw new TypeError('Appearance-event flow requires message repository or command.sourceMessage');
        return normalizeSource(command.sourceMessage, personaId, messageId);
    }
    const source = sourceLookup.length >= 2
        ? sourceLookup(messageId, {personaId})
        : sourceLookup({id: messageId, messageId, personaId});
    return normalizeSource(sync(source, 'Appearance-event source lookup'), personaId, messageId);
}

function idempotentLookup(findByIdempotencyKey, personaId, idempotencyKey) {
    if (!idempotencyKey) return null;
    if (!findByIdempotencyKey) throw new TypeError('Appearance-event flow requires lifeEvent.findByIdempotencyKey() for idempotent calls');
    const existing = findByIdempotencyKey.length >= 2
        ? findByIdempotencyKey(personaId, idempotencyKey)
        : findByIdempotencyKey({personaId, idempotencyKey});
    return sync(existing, 'Appearance-event idempotency lookup') || null;
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
    throw new TypeError('Appearance-event transaction must be a function or provide transaction()/run()');
}

function assertPlan(plan) {
    if (!isRecord(plan) || !PRIVATE_PLAN.has(plan)) throw new TypeError('appearance_event plan is invalid');
    return PRIVATE_PLAN.get(plan);
}

function eventPayload(row) {
    return parseJson(rowValue(row, 'payloadJson', 'payload_json'), {});
}

function eventOperation(row, fallback = 'set') {
    const operation = eventPayload(row).operation;
    return APPEARANCE_EVENT_OPERATIONS.includes(operation) ? operation : fallback;
}

function replayResult(existing) {
    const payload = eventPayload(existing);
    const appearance = parseJson(payload.nextAppearance ?? payload.appearance, {});
    return {
        eventId: existing.id,
        operation: eventOperation(existing),
        outfit: appearance.outfit ?? null,
        appearance,
        reason: payload.reason ?? null,
        replayed: true
    };
}

/** Normalize the decoded appearance_event call without parsing provider streams. */
export function normalizeAppearanceEventCall(value) {
    if (!isRecord(value)) throw new Error('appearance_event call must be an object');
    const allowed = ['operation', 'outfit', 'reason'];
    if (Object.keys(value).some(key => !allowed.includes(key))) throw new Error('appearance_event call contains unsupported fields');
    const operation = value.operation;
    if (!APPEARANCE_EVENT_OPERATIONS.includes(operation)) throw new Error('appearance_event operation is invalid');
    const suppliedOutfit = value.outfit === undefined ? undefined : requiredText(value.outfit, 'appearance_event.outfit', APPEARANCE_EVENT_MAX_OUTFIT_LENGTH);
    const reason = optionalText(value.reason, 'appearance_event.reason', APPEARANCE_EVENT_MAX_REASON_LENGTH);
    if (operation === 'set' && !suppliedOutfit) throw new Error('appearance_event set requires outfit');
    return Object.freeze({
        operation,
        ...(operation === 'set' ? {outfit: suppliedOutfit} : {}),
        ...(reason ? {reason} : {})
    });
}

export function createAppearanceEventFlow({
    repositories,
    clock,
    idGenerator,
    transaction,
    normalizeCall,
    normalizeAppearanceEventCall: injectedNormalizer
} = {}) {
    if (!isRecord(repositories)) throw new TypeError('Appearance-event flow repositories must be an object');
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
    if (normalizer !== undefined && typeof normalizer !== 'function') throw new TypeError('Appearance-event flow normalizeCall must be a function');

    function plan(input, value, sourceMessageIdValue, provenanceValue = {}) {
        const command = typeof input === 'string'
            ? {personaId: input, call: value, sourceMessageId: sourceMessageIdValue, provenance: provenanceValue}
            : input;
        if (!isRecord(command)) throw new TypeError('Appearance-event command must be an object');
        const personaId = requiredText(command.personaId ?? command.persona_id, 'Appearance-event personaId');
        const persona = personaFor(personaLookup, personaId, command.persona);
        const sourceMessageId = requiredText(
            command.sourceMessageId
                ?? command.source_message_id
                ?? command.causationUserMessageId
                ?? command.causation_user_message_id
                ?? command.causationId
                ?? command.causation_id,
            'Appearance-event sourceMessageId'
        );
        const source = sourceFor(sourceLookup, command, personaId, sourceMessageId);
        const callValue = command.call ?? command.value ?? command.appearanceEvent ?? command.appearance_event ?? command.arguments;
        if (callValue === undefined) throw new TypeError('Appearance-event call is required');
        const call = normalizer
            ? normalizeAppearanceEventCall(sync(normalizer(callValue), 'Appearance-event normalizer'))
            : normalizeAppearanceEventCall(callValue);
        const provenance = provenanceFor(command.provenance ?? command, callValue);
        const existing = idempotentLookup(lifeEventFind, personaId, provenance.idempotencyKey);
        const projection = appearanceFor(sync(stateRead(personaId), 'Appearance-event state lookup'));
        const existingPayload = existing ? eventPayload(existing) : null;
        if (existing && existingPayload.causationId && existingPayload.causationId !== source.id) {
            throw new Error('appearance_event idempotency key is bound to a different source message');
        }
        const replayed = Boolean(existing);
        const operation = replayed ? eventOperation(existing, call.operation) : call.operation;
        const previousAppearance = replayed
            ? parseJson(existingPayload.previousAppearance ?? existingPayload.beforeAppearance, {})
            : projection.appearance;
        const createdAt = timestamp(rowValue(existing, 'createdAt', 'created_at') || now(), 'Appearance-event createdAt');
        const eventId = existing?.id || generateId('event');
        const nextAppearance = replayed
            ? parseJson(existingPayload.nextAppearance ?? existingPayload.appearance, projection.appearance)
            : operation === 'set'
            ? {...cloneValue(projection.appearance), outfit: call.outfit}
            : Object.fromEntries(Object.entries(projection.appearance).filter(([key]) => key !== 'outfit').map(([key, value]) => [key, cloneValue(value)]));
        const payload = replayed ? existingPayload : {
            schemaVersion: APPEARANCE_EVENT_SCHEMA_VERSION,
            operation: call.operation,
            source: provenance.source,
            causationId: source.id,
            eventId,
            ...(provenance.callId ? {capabilityCallId: provenance.callId} : {}),
            ...(provenance.idempotencyKey ? {idempotencyKey: provenance.idempotencyKey} : {}),
            ...(call.outfit ? {outfit: call.outfit} : {outfit: null}),
            ...(call.reason ? {reason: call.reason} : {}),
            previousAppearance,
            nextAppearance
        };
        const expected = {
            sourceEventId: projection.sourceEventId,
            appearanceJson: projection.appearanceJson,
            appearance: projection.appearance
        };
        const preview = {
            eventId,
            operation,
            outfit: nextAppearance.outfit ?? null,
            appearance: nextAppearance,
            reason: payload.reason ?? null,
            replayed
        };
        const planValue = deepFreeze({
            type: 'appearance_event_plan',
            version: APPEARANCE_EVENT_FLOW_VERSION,
            schemaVersion: APPEARANCE_EVENT_SCHEMA_VERSION,
            personaId,
            sourceMessageId,
            call,
            provenance,
            operation,
            eventId,
            createdAt,
            idempotencyKey: provenance.idempotencyKey || '',
            expected,
            previousAppearance,
            nextAppearance,
            appearance: nextAppearance,
            outfit: nextAppearance.outfit ?? null,
            payload,
            preview,
            previewResult: preview,
            replayed,
            replayCandidate: replayed,
            preallocatedIds: {eventId}
        });
        PRIVATE_PLAN.set(planValue, {persona, source, call, provenance, expected, previousAppearance, nextAppearance, payload, eventId, createdAt, operation});
        return planValue;
    }

    function applyWithin(planValue, state) {
        personaFor(personaLookup, planValue.personaId, state.persona);
        sourceFor(sourceLookup, {sourceMessage: state.source}, planValue.personaId, planValue.sourceMessageId);
        const existing = idempotentLookup(lifeEventFind, planValue.personaId, state.provenance.idempotencyKey);
        if (existing) return replayResult(existing);
        const current = appearanceFor(sync(stateRead(planValue.personaId), 'Appearance-event state lookup'));
        const expected = state.expected;
        if (current.sourceEventId !== (expected.sourceEventId || null) || !sameAppearance(current.appearance, expected.appearance)) {
            throw new Error('appearance_event plan does not match the current appearance projection');
        }
        if (!state.eventId || !state.createdAt || !state.operation || !isRecord(state.payload)) throw new TypeError('appearance_event plan contents are invalid');
        const nextAppearance = state.operation === 'set'
            ? {...cloneValue(current.appearance), outfit: state.call.outfit}
            : Object.fromEntries(Object.entries(current.appearance).filter(([key]) => key !== 'outfit').map(([key, value]) => [key, cloneValue(value)]));
        const event = lifeEventCreate({
            eventId: state.eventId,
            id: state.eventId,
            personaId: planValue.personaId,
            type: 'appearance_change',
            occurredAt: state.createdAt,
            resolvesAt: null,
            causationId: state.source.id,
            payload: state.payload,
            createdAt: state.createdAt
        });
        sync(event, 'Appearance-event repository write');
        const update = stateUpdate({
            personaId: planValue.personaId,
            situation: current.situation,
            mood: current.mood,
            appearance: nextAppearance,
            appearanceJson: JSON.stringify(nextAppearance),
            checkpointAt: state.createdAt,
            updatedAt: state.createdAt,
            sourceEventId: state.eventId,
            expected
        });
        sync(update, 'Appearance-event state write');
        if (update?.changes !== undefined && update.changes !== 1) throw new Error('appearance_event plan does not match the current state projection');
        if (update?.changed !== undefined && !update.changed) throw new Error('appearance_event plan does not match the current state projection');
        if (personaTouch) {
            const touched = personaTouch({personaId: planValue.personaId, updatedAt: state.createdAt});
            sync(touched, 'Appearance-event persona write');
        }
        return {
            eventId: state.eventId,
            operation: state.operation,
            outfit: nextAppearance.outfit ?? null,
            appearance: nextAppearance,
            reason: state.payload.reason ?? null
        };
    }

    function apply(planValue, options = {}) {
        const state = assertPlan(planValue);
        const settings = typeof options === 'function' ? {transaction: options} : options;
        if (!isRecord(settings)) throw new TypeError('Appearance-event apply options must be an object');
        const runner = settings.transaction ?? settings.callerTransaction ?? settings.runInTransaction ?? settings.commit ?? transaction;
        return transactionRunner(runner, () => applyWithin(planValue, state));
    }

    return Object.freeze({
        version: APPEARANCE_EVENT_FLOW_VERSION,
        plan,
        apply,
        normalizeAppearanceEventCall: normalizer || normalizeAppearanceEventCall,
        repositories: Object.freeze({persona: personaRepository, message: messageRepository, lifeEvent: lifeEventRepository, state: stateRepository})
    });
}

export const createCompanionAppearanceEventFlow = createAppearanceEventFlow;
export default createAppearanceEventFlow;

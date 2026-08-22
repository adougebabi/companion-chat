import {createHash} from 'node:crypto';

import {
    MEMORY_EVENT_SCHEMA_VERSION,
    normalizeMemoryEventCall
} from './memory-service.js';

export const MEMORY_EVENT_FLOW_VERSION = 1;

const PRIVATE_PLAN = new WeakMap();
const MAX_IDEMPOTENCY_LENGTH = 240;
const MAX_CALL_ID_LENGTH = 160;

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function requiredText(value, field, maxLength = 240) {
    if (typeof value !== 'string' || value.trim() === '') throw new TypeError(`${field} must be a non-empty string`);
    const text = value.trim();
    if (text.length > maxLength) throw new RangeError(`${field} exceeds ${maxLength} characters`);
    return text;
}

function timestamp(value, field) {
    const resolved = value instanceof Date ? value.toISOString() : value;
    if (typeof resolved !== 'string' || resolved.trim() === '' || !Number.isFinite(Date.parse(resolved))) {
        throw new TypeError(`${field} must be a valid timestamp`);
    }
    return new Date(resolved).toISOString();
}

function clockFunction(clock) {
    if (clock === undefined) return () => new Date().toISOString();
    if (typeof clock === 'function') return () => timestamp(clock(), 'Memory-event clock value');
    if (isRecord(clock) && typeof clock.now === 'function') return () => timestamp(clock.now(), 'Memory-event clock value');
    throw new TypeError('Memory-event flow clock must be a function or provide now()');
}

function idFunction(idGenerator) {
    if (idGenerator === undefined) return prefix => `${prefix}_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`;
    if (typeof idGenerator === 'function') return prefix => requiredText(idGenerator(prefix), 'Generated memory id');
    if (isRecord(idGenerator) && typeof idGenerator.next === 'function') {
        return prefix => requiredText(idGenerator.next(prefix), 'Generated memory id');
    }
    throw new TypeError('Memory-event flow idGenerator must be a function or provide next()');
}

function resolveRepository(repositories, names, field, {optional = false} = {}) {
    const source = isRecord(repositories) ? repositories : {};
    for (const name of names) {
        if (source[name] !== undefined) {
            if (!isRecord(source[name]) && typeof source[name] !== 'function') {
                throw new TypeError(`Memory-event flow ${field} must be an object`);
            }
            return source[name];
        }
    }
    if (optional) return null;
    throw new TypeError(`Memory-event flow requires ${field}`);
}

function methodFor(repository, names, field, {optional = false} = {}) {
    if (typeof repository === 'function') return repository;
    if (isRecord(repository)) {
        for (const name of names) {
            if (repository[name] !== undefined) {
                if (typeof repository[name] !== 'function') throw new TypeError(`Memory-event flow ${field}.${name} must be a function`);
                return repository[name].bind(repository);
            }
        }
    }
    if (optional) return null;
    throw new TypeError(`Memory-event flow ${field} must provide ${names.join('() or ')}()`);
}

function sync(value, field) {
    if (value && typeof value.then === 'function') throw new TypeError(`${field} must be synchronous`);
    return value;
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
    throw new TypeError('Memory-event transaction must be a function or provide transaction()/run()');
}

function deepFreeze(value) {
    if (!value || typeof value !== 'object' || Object.isFrozen(value)) return value;
    Object.freeze(value);
    for (const child of Object.values(value)) deepFreeze(child);
    return value;
}

function stableMemoryId(personaId, idempotencyKey) {
    const digest = createHash('sha256').update(`${personaId}\u0000${idempotencyKey}`).digest('hex');
    return `memory_event_${digest}`;
}

function provenanceFor(value = {}, call) {
    if (!isRecord(value)) throw new TypeError('Memory-event provenance must be an object');
    const source = value.source ?? call?.source ?? 'native';
    const callId = value.callId ?? value.call_id ?? call?.callId ?? call?.call_id;
    const idempotencyKey = value.idempotencyKey ?? value.idempotency_key
        ?? call?.idempotencyKey ?? call?.idempotency_key;
    return Object.freeze({
        source: requiredText(source, 'Memory-event provenance source', 80),
        ...(callId === undefined || callId === null || callId === ''
            ? {} : {callId: requiredText(callId, 'Memory-event provenance callId', MAX_CALL_ID_LENGTH)}),
        idempotencyKey: requiredText(idempotencyKey, 'Memory-event idempotencyKey', MAX_IDEMPOTENCY_LENGTH)
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
    if (!isRecord(row) || sourceMessageId(row) !== expectedId) throw new Error('memory_event source message does not exist');
    if (sourcePersonaId(row) !== personaId) throw new Error('memory_event source message does not belong to persona');
    if (sourceRole(row) !== 'user') throw new Error('memory_event source message must be user-owned');
    return Object.freeze({id: expectedId, personaId, role: 'user'});
}

function personaFor(personaLookup, personaId, supplied) {
    if (!personaLookup) {
        if (!isRecord(supplied)) throw new TypeError('Memory-event flow requires persona repository or command.persona');
        const suppliedId = supplied.id ?? supplied.personaId ?? supplied.persona_id;
        if (suppliedId !== personaId) throw new Error('memory_event persona does not belong to the command');
        return supplied;
    }
    const persona = sync(personaLookup(personaId), 'Memory-event persona lookup');
    if (!persona) throw new Error('memory_event persona does not exist');
    const returnedId = persona.id ?? persona.personaId ?? persona.persona_id;
    if (returnedId !== undefined && returnedId !== personaId) throw new Error('memory_event persona does not belong to the command');
    return persona;
}

function sourceFor(sourceLookup, command, personaId, messageId) {
    if (!sourceLookup) {
        if (!isRecord(command.sourceMessage)) throw new TypeError('Memory-event flow requires message repository or command.sourceMessage');
        return normalizeSource(command.sourceMessage, personaId, messageId);
    }
    const source = sourceLookup.length >= 2
        ? sourceLookup(messageId, {personaId})
        : sourceLookup({id: messageId, messageId, personaId});
    return normalizeSource(sync(source, 'Memory-event source lookup'), personaId, messageId);
}

function idempotentLookup(findByIdempotencyKey, personaId, idempotencyKey) {
    if (!findByIdempotencyKey) return null;
    return sync(findByIdempotencyKey({personaId, idempotencyKey}), 'Memory-event idempotency lookup') || null;
}

function targetLookup(findByMemoryKey, personaId, memoryKey) {
    if (!findByMemoryKey) return null;
    return sync(findByMemoryKey({personaId, memoryKey}), 'Memory-event memory-key lookup') || null;
}

function assertOwnedRow(row, personaId, field) {
    if (!row) return null;
    const owner = rowValue(row, 'personaId', 'persona_id');
    if (owner !== personaId) throw new Error(`memory_event ${field} does not belong to persona`);
    return row;
}

function assertSourceMatch(row, sourceId) {
    if (!row) return null;
    const persistedSource = rowValue(row, 'sourceId', 'source_id');
    if (persistedSource && persistedSource !== sourceId) {
        throw new Error('memory_event idempotency key is bound to a different source message');
    }
    return row;
}

function assertPlan(plan) {
    if (!isRecord(plan) || !PRIVATE_PLAN.has(plan)) throw new TypeError('memory_event plan is invalid');
    return PRIVATE_PLAN.get(plan);
}

function resultRow(value) {
    if (!value) return null;
    return value.row ?? value.memory ?? value.result ?? value;
}

/**
 * Persona-scoped memory_event application flow. Planning is read-only and
 * applying a plan is synchronous so it can participate in the caller's
 * better-sqlite3 transaction alongside the assistant message.
 */
export function createMemoryEventFlow({
    repositories,
    clock,
    idGenerator,
    transaction,
    memoryService,
    normalizeCall
} = {}) {
    if (!isRecord(repositories)) throw new TypeError('Memory-event flow repositories must be an object');
    const memoryRepository = resolveRepository(repositories, ['memoryRepository', 'memory', 'memories'], 'memory repository');
    const personaRepository = resolveRepository(repositories, ['personaRepository', 'personas', 'persona'], 'persona repository', {optional: true});
    const messageRepository = resolveRepository(repositories, ['messageRepository', 'sourceMessageRepository', 'messages', 'conversationRepository', 'conversation'], 'message repository', {optional: true});
    const personaLookup = personaRepository ? methodFor(personaRepository, ['findActive', 'findById', 'requirePersona', 'get'], 'persona repository', {optional: true}) : null;
    const sourceLookup = messageRepository ? methodFor(messageRepository, ['findById', 'findMessage', 'findSourceMessage', 'getMessage', 'getById'], 'message repository', {optional: true}) : null;
    const memoryFindIdempotency = methodFor(memoryRepository, ['findByIdempotencyKey', 'findByIdempotency', 'findByDedupeKey'], 'memory repository', {optional: true});
    const memoryFindKey = methodFor(memoryRepository, ['findByMemoryKey', 'findByKey', 'findActiveByKey'], 'memory repository', {optional: true});
    const memoryInsert = methodFor(memoryRepository, ['insertMemory', 'insert', 'create', 'record'], 'memory repository', {optional: true});
    const memoryInsertIgnore = methodFor(memoryRepository, ['insertMemoryIgnore', 'insertIgnore', 'insertIdempotent'], 'memory repository', {optional: true});
    const memoryUpsert = methodFor(memoryRepository, ['upsertMemory', 'upsert', 'save'], 'memory repository', {optional: true});
    if (!memoryInsert && !memoryInsertIgnore && !memoryUpsert) throw new TypeError('Memory-event flow requires a memory write port');
    if (!memoryFindIdempotency) throw new TypeError('Memory-event flow requires a memory idempotency lookup port');
    const now = clockFunction(clock);
    const generateId = idFunction(idGenerator);
    const normalizer = normalizeCall
        ?? memoryService?.normalizeMemoryEvent
        ?? memoryService?.normalizeCall
        ?? normalizeMemoryEventCall;
    if (typeof normalizer !== 'function') throw new TypeError('Memory-event flow normalizeCall must be a function');

    function plan(input, value, sourceMessageIdValue, provenanceValue = {}) {
        const command = typeof input === 'string'
            ? {personaId: input, call: value, sourceMessageId: sourceMessageIdValue, provenance: provenanceValue}
            : input;
        if (!isRecord(command)) throw new TypeError('Memory-event command must be an object');
        const personaId = requiredText(command.personaId ?? command.persona_id, 'Memory-event personaId');
        const persona = personaFor(personaLookup, personaId, command.persona);
        const sourceMessageId = requiredText(
            command.sourceMessageId ?? command.source_message_id ?? command.causationId ?? command.causation_id,
            'Memory-event sourceMessageId'
        );
        const source = sourceFor(sourceLookup, command, personaId, sourceMessageId);
        const suppliedCall = command.call ?? command.value ?? command.memoryEvent ?? command.memory_event ?? command.arguments;
        const callValue = isRecord(suppliedCall?.memory) ? suppliedCall.memory : suppliedCall;
        if (!isRecord(callValue)) throw new TypeError('Memory-event call is required');
        const provenance = provenanceFor(command.provenance ?? command, callValue);
        const call = sync(normalizer(callValue, {sourceMessageId, idempotencyKey: provenance.idempotencyKey}), 'Memory-event normalizer');
        const normalizedCall = normalizeMemoryEventCall(call, {
            sourceMessageId,
            idempotencyKey: provenance.idempotencyKey
        });
        const existing = assertSourceMatch(assertOwnedRow(
            idempotentLookup(memoryFindIdempotency, personaId, normalizedCall.idempotencyKey), personaId, 'idempotent memory'
        ), normalizedCall.sourceId);
        const target = existing || (normalizedCall.operation === 'upsert'
            ? assertOwnedRow(targetLookup(memoryFindKey, personaId, normalizedCall.memoryKey), personaId, 'upsert target')
            : null);
        const deterministic = memoryRepository.idempotencyStorage === 'deterministic-id';
        const memoryId = existing?.id
            ?? target?.id
            ?? (deterministic
                ? stableMemoryId(personaId, normalizedCall.idempotencyKey)
                : generateId('memory'));
        const createdAt = timestamp(rowValue(existing, 'createdAt', 'created_at') || now(), 'Memory-event createdAt');
        const replayed = Boolean(existing);
        const memory = {
            id: memoryId,
            personaId,
            memoryKey: normalizedCall.memoryKey,
            value: normalizedCall.value,
            confidence: normalizedCall.confidence,
            status: 'active',
            sourceType: normalizedCall.sourceType,
            sourceId: normalizedCall.sourceId,
            idempotencyKey: normalizedCall.idempotencyKey,
            createdAt,
            updatedAt: createdAt
        };
        const preview = {memory, operation: normalizedCall.operation, replayed, changed: !replayed};
        const planValue = deepFreeze({
            type: 'memory_event_plan',
            version: MEMORY_EVENT_FLOW_VERSION,
            schemaVersion: MEMORY_EVENT_SCHEMA_VERSION,
            personaId,
            sourceMessageId,
            call: normalizedCall,
            provenance,
            operation: normalizedCall.operation,
            memoryId,
            idempotencyKey: normalizedCall.idempotencyKey,
            createdAt,
            memory,
            preview,
            previewResult: preview,
            replayed,
            replayCandidate: replayed,
            preallocatedIds: {memoryId}
        });
        PRIVATE_PLAN.set(planValue, {persona, source, call: normalizedCall, provenance, memory, target, existing, memoryId, createdAt});
        return planValue;
    }

    function applyWithin(planValue, state) {
        personaFor(personaLookup, planValue.personaId, state.persona ?? state.owner ?? undefined);
        sourceFor(sourceLookup, {sourceMessage: state.source}, planValue.personaId, planValue.sourceMessageId);
        const existing = assertSourceMatch(assertOwnedRow(
            idempotentLookup(memoryFindIdempotency, planValue.personaId, state.provenance.idempotencyKey),
            planValue.personaId, 'idempotent memory'
        ), state.call.sourceId);
        if (existing) {
            return {
                memoryId: existing.id,
                memory: existing,
                created: false,
                replayed: true,
                changed: false,
                idempotencyKey: state.provenance.idempotencyKey
            };
        }
        const target = state.call.operation === 'upsert'
            ? assertOwnedRow(targetLookup(memoryFindKey, planValue.personaId, state.call.memoryKey), planValue.personaId, 'upsert target')
            : null;
        const input = {
            id: target?.id ?? state.memoryId,
            personaId: planValue.personaId,
            memoryKey: state.call.memoryKey,
            value: state.call.value,
            confidence: state.call.confidence,
            status: 'active',
            sourceType: state.call.sourceType,
            sourceId: state.call.sourceId,
            idempotencyKey: state.call.idempotencyKey,
            createdAt: target?.createdAt ?? target?.created_at ?? state.createdAt,
            updatedAt: state.createdAt,
            supersededAt: null
        };
        const writer = state.call.operation === 'upsert'
            ? (memoryUpsert ?? memoryInsert)
            : (memoryInsertIgnore ?? memoryInsert);
        if (!writer) throw new TypeError(`Memory-event ${state.call.operation} requires a memory write port`);
        const written = sync(writer(input), 'Memory-event repository write');
        const row = resultRow(written) ?? input;
        const inserted = written?.changes === undefined ? true : written.changes > 0;
        if (!inserted) {
            const replay = assertSourceMatch(assertOwnedRow(
                idempotentLookup(memoryFindIdempotency, planValue.personaId, state.provenance.idempotencyKey),
                planValue.personaId, 'idempotent memory'
            ), state.call.sourceId);
            if (replay) return {
                memoryId: replay.id,
                memory: replay,
                created: false,
                replayed: true,
                changed: false,
                idempotencyKey: state.provenance.idempotencyKey
            };
        }
        return {
            memoryId: row.id ?? input.id,
            memory: row,
            created: inserted,
            replayed: false,
            changed: inserted,
            idempotencyKey: state.provenance.idempotencyKey
        };
    }

    function apply(planValue, options = {}) {
        const state = assertPlan(planValue);
        const settings = typeof options === 'function' ? {transaction: options} : options;
        if (!isRecord(settings)) throw new TypeError('Memory-event apply options must be an object');
        const runner = settings.transaction ?? settings.callerTransaction ?? settings.runInTransaction ?? settings.commit ?? transaction;
        return transactionRunner(runner, () => applyWithin(planValue, state));
    }

    return Object.freeze({
        version: MEMORY_EVENT_FLOW_VERSION,
        schemaVersion: MEMORY_EVENT_SCHEMA_VERSION,
        plan,
        apply,
        normalizeMemoryEventCall: normalizeMemoryEventCall,
        normalizeCall: normalizer,
        repositories: Object.freeze({persona: personaRepository, message: messageRepository, memory: memoryRepository})
    });
}

export const createMemoryFlow = createMemoryEventFlow;
export const createCompanionMemoryEventFlow = createMemoryEventFlow;
export default createMemoryEventFlow;

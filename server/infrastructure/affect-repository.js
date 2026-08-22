import {randomUUID} from 'node:crypto';

import {
    AFFECT_MODEL_VERSION,
    applyAffectEvent,
    createInitialAffectState,
    decayAffectState,
    normalizeAffectPolicy,
    normalizeAffectState,
    reduceAffectEvent,
    reduceDriveSignal
} from '../domain/affect-state.js';

function assertOpenDatabase(database) {
    if (!database || typeof database.prepare !== 'function' || typeof database.transaction !== 'function') {
        throw new TypeError('Affect repository requires an open database with transaction()');
    }
    return database;
}

function requiredText(value, field, {max = 256} = {}) {
    if (typeof value !== 'string' || value.trim() === '') throw new TypeError(`${field} must be non-empty`);
    const result = value.trim();
    if (result.length > max) throw new RangeError(`${field} exceeds ${max} characters`);
    return result;
}

function optionalText(value, field, {max = 256} = {}) {
    if (value === undefined || value === null || value === '') return null;
    return requiredText(value, field, {max});
}

function timestamp(value, field) {
    const date = value instanceof Date ? value : new Date(value);
    if (!Number.isFinite(date.getTime())) throw new TypeError(`${field} must be a valid timestamp`);
    return date.toISOString();
}

function clockFor(clock, now) {
    const source = clock ?? now;
    if (source === undefined) return () => new Date().toISOString();
    if (typeof source === 'function') return () => timestamp(source(), 'Affect repository clock value');
    if (source && typeof source.now === 'function') return () => timestamp(source.now(), 'Affect repository clock value');
    throw new TypeError('Affect repository clock must be a function or provide now()');
}

function idFor(id, idGenerator) {
    const source = id ?? idGenerator;
    if (source === undefined) return prefix => `${prefix}_${randomUUID()}`;
    if (typeof source === 'function') return prefix => requiredText(source(prefix), 'Generated affect id');
    if (source && typeof source.next === 'function') return prefix => requiredText(source.next(prefix), 'Generated affect id');
    throw new TypeError('Affect repository id must be a function or provide next()');
}

function ownerFrom(input) {
    if (typeof input === 'string') return requiredText(input, 'Persona.id');
    return requiredText(input?.personaId ?? input?.persona_id, 'Persona.id');
}

function parseJson(value, fallback, field) {
    if (value === undefined || value === null || value === '') return fallback;
    if (typeof value === 'object') return value;
    if (typeof value !== 'string') throw new TypeError(`${field} must be JSON or an object`);
    try {
        const parsed = JSON.parse(value);
        return parsed === null ? fallback : parsed;
    } catch {
        throw new TypeError(`${field} must contain valid JSON`);
    }
}

function jsonValue(value, fallback, field) {
    const parsed = parseJson(value, fallback, field);
    const serialized = JSON.stringify(parsed);
    if (serialized === undefined) throw new TypeError(`${field} could not be serialized`);
    if (serialized.length > 32_768) throw new RangeError(`${field} exceeds 32768 bytes`);
    return serialized;
}

function limitValue(value) {
    const limit = value === undefined ? 100 : Number(value);
    if (!Number.isInteger(limit) || limit < 1 || limit > 1000) throw new RangeError('Affect event limit must be an integer from 1 to 1000');
    return limit;
}

function rowToEvent(row) {
    if (!row) return null;
    return {
        id: row.id,
        personaId: row.persona_id,
        eventType: row.event_type,
        effectiveAt: row.effective_at,
        causationId: row.causation_id,
        sourceMessageId: row.source_message_id,
        idempotencyKey: row.idempotency_key,
        pleasureDelta: row.pleasure_delta,
        arousalDelta: row.arousal_delta,
        dominanceDelta: row.dominance_delta,
        drivesDelta: parseJson(row.drives_delta_json, {}, 'Affect event.drivesDelta'),
        payload: parseJson(row.payload_json, {}, 'Affect event.payload'),
        modelVersion: row.model_version,
        createdAt: row.created_at
    };
}

function rowToSnapshot(row, policy) {
    if (!row) return null;
    return normalizeAffectState({
        personaId: row.persona_id,
        pleasure: row.pleasure,
        arousal: row.arousal,
        dominance: row.dominance,
        drives: row.drives_json,
        revision: row.revision,
        effectiveAt: row.effective_at,
        updatedAt: row.updated_at,
        sourceEventId: row.source_event_id,
        modelVersion: row.model_version
    }, {policy});
}

function runTransaction(database, work) {
    return database.inTransaction ? work() : database.transaction(work)();
}

function hasTable(database, name) {
    return Boolean(database.prepare("SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?").get(name));
}

/**
 * Table-scoped repository for the materialized PAD/drives snapshot and its
 * append-only event log. The caller can compose `applyEvent` or `checkpoint`
 * inside a larger better-sqlite3 transaction.
 */
export function createAffectRepository({database, clock, now, id, idGenerator, policy, policyForPersona, blueprintRepository} = {}) {
    const db = assertOpenDatabase(database);
    const currentTime = clockFor(clock, now);
    const generateId = idFor(id, idGenerator);
    const resolvePolicy = typeof policyForPersona === 'function'
        ? policyForPersona
        : typeof policy === 'function'
            ? policy
            : blueprintRepository && typeof blueprintRepository.read === 'function'
                ? ({personaId}) => blueprintRepository.read({personaId})
                : () => policy ?? {};

    function readRow(personaId) {
        return db.prepare('SELECT * FROM companion_persona_affect_states WHERE persona_id = ?').get(personaId) ?? null;
    }

    function policyFor(input = {}) {
        const supplied = input.policy;
        if (supplied !== undefined) return normalizeAffectPolicy(supplied);
        return normalizeAffectPolicy(resolvePolicy({personaId: ownerFrom(input), at: input.at ?? input.effectiveAt ?? currentTime()}));
    }

    function readSnapshot(input = {}) {
        const owner = ownerFrom(input);
        const normalizedPolicy = policyFor({...input, personaId: owner});
        const row = readRow(owner);
        const requestedAt = timestamp(input.at ?? input.effectiveAt ?? currentTime(), 'Affect snapshot.at');
        const state = row
            ? rowToSnapshot(row, normalizedPolicy)
            : createInitialAffectState({personaId: owner, at: requestedAt, policy: normalizedPolicy});
        const at = row && new Date(requestedAt).getTime() < new Date(state.effectiveAt).getTime() ? state.effectiveAt : requestedAt;
        return decayAffectState(state, {at, policy: normalizedPolicy});
    }

    function findSnapshot(input = {}) {
        const owner = ownerFrom(input);
        return rowToSnapshot(readRow(owner), policyFor({...input, personaId: owner}));
    }

    function findEventByIdempotencyKey(input = {}) {
        const owner = ownerFrom(input);
        const key = requiredText(input.idempotencyKey ?? input.idempotency_key, 'Affect event.idempotencyKey', {max: 200});
        return rowToEvent(db.prepare(`
            SELECT * FROM companion_persona_affect_events
            WHERE persona_id = ? AND idempotency_key = ?
        `).get(owner, key));
    }

    function findEvent(input = {}) {
        const owner = ownerFrom(input);
        const eventId = requiredText(input.eventId ?? input.event_id ?? input.id, 'Affect event.id');
        return rowToEvent(db.prepare('SELECT * FROM companion_persona_affect_events WHERE id = ? AND persona_id = ?').get(eventId, owner));
    }

    function assertSourceMessageScope(personaId, sourceMessageId) {
        if (!sourceMessageId || !hasTable(db, 'companion_messages') || !hasTable(db, 'companion_conversations')) return;
        const row = db.prepare(`
            SELECT m.id
            FROM companion_messages m
            JOIN companion_conversations c ON c.id = m.conversation_id
            WHERE m.id = ? AND c.persona_id = ?
        `).get(sourceMessageId, personaId);
        if (!row) throw new Error('Affect source message does not belong to persona');
    }

    function normalizeEventInput(input = {}) {
        if (!input || typeof input !== 'object' || Array.isArray(input)) throw new TypeError('Affect event input must be an object');
        const personaId = ownerFrom(input);
        const eventId = requiredText(input.id ?? input.eventId ?? generateId('affect_event'), 'Affect event.id');
        const idempotencyKey = requiredText(input.idempotencyKey ?? input.idempotency_key, 'Affect event.idempotencyKey', {max: 200});
        const reduction = reduceAffectEvent(input);
        const effectiveAt = timestamp(input.effectiveAt ?? input.effective_at ?? input.occurredAt ?? input.occurred_at ?? currentTime(), 'Affect event.effectiveAt');
        const createdAt = timestamp(input.createdAt ?? input.created_at ?? currentTime(), 'Affect event.createdAt');
        const sourceMessageId = optionalText(input.sourceMessageId ?? input.source_message_id, 'Affect event.sourceMessageId', {max: 256});
        assertSourceMessageScope(personaId, sourceMessageId);
        return {
            id: eventId,
            personaId,
            eventType: reduction.eventType,
            effectiveAt,
            causationId: optionalText(input.causationId ?? input.causation_id, 'Affect event.causationId', {max: 256}),
            sourceMessageId,
            idempotencyKey,
            pleasureDelta: reduction.pleasureDelta,
            arousalDelta: reduction.arousalDelta,
            dominanceDelta: reduction.dominanceDelta,
            drivesDelta: reduction.drivesDelta,
            payload: parseJson(input.payload ?? input.payloadJson ?? input.payload_json, {}, 'Affect event.payload'),
            modelVersion: optionalText(input.modelVersion ?? input.model_version, 'Affect event.modelVersion', {max: 100}) ?? AFFECT_MODEL_VERSION,
            createdAt
        };
    }

    function normalizeDriveSignalInput(input = {}) {
        if (!input || typeof input !== 'object' || Array.isArray(input)) throw new TypeError('Drive signal input must be an object');
        const personaId = ownerFrom(input);
        const eventId = requiredText(input.id ?? input.eventId ?? generateId('affect_event'), 'Affect event.id');
        const idempotencyKey = requiredText(input.idempotencyKey ?? input.idempotency_key, 'Affect event.idempotencyKey', {max: 200});
        const reduction = reduceDriveSignal(input);
        const effectiveAt = timestamp(input.effectiveAt ?? input.effective_at ?? input.occurredAt ?? input.occurred_at ?? currentTime(), 'Affect event.effectiveAt');
        const createdAt = timestamp(input.createdAt ?? input.created_at ?? currentTime(), 'Affect event.createdAt');
        const sourceMessageId = optionalText(input.sourceMessageId ?? input.source_message_id, 'Affect event.sourceMessageId', {max: 256});
        assertSourceMessageScope(personaId, sourceMessageId);
        return {
            id: eventId,
            personaId,
            eventType: reduction.eventType,
            effectiveAt,
            causationId: optionalText(input.causationId ?? input.causation_id, 'Affect event.causationId', {max: 256}),
            sourceMessageId,
            idempotencyKey,
            pleasureDelta: reduction.pleasureDelta,
            arousalDelta: reduction.arousalDelta,
            dominanceDelta: reduction.dominanceDelta,
            drivesDelta: reduction.drivesDelta,
            payload: {
                ...parseJson(input.payload ?? input.payloadJson ?? input.payload_json, {}, 'Affect event.payload'),
                drive: reduction.drive,
                direction: reduction.direction,
                recognized: reduction.recognized
            },
            modelVersion: optionalText(input.modelVersion ?? input.model_version, 'Affect event.modelVersion', {max: 100}) ?? AFFECT_MODEL_VERSION,
            createdAt
        };
    }

    function insertEvent(event) {
        db.prepare(`
            INSERT INTO companion_persona_affect_events (
                id, persona_id, event_type, effective_at, causation_id, source_message_id,
                idempotency_key, pleasure_delta, arousal_delta, dominance_delta,
                drives_delta_json, payload_json, model_version, created_at
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        `).run(
            event.id, event.personaId, event.eventType, event.effectiveAt, event.causationId, event.sourceMessageId,
            event.idempotencyKey, event.pleasureDelta, event.arousalDelta, event.dominanceDelta,
            jsonValue(event.drivesDelta, {}, 'Affect event.drivesDelta'), jsonValue(event.payload, {}, 'Affect event.payload'),
            event.modelVersion, event.createdAt
        );
        return findEvent({personaId: event.personaId, eventId: event.id});
    }

    function writeSnapshot({personaId, state, expectedRevision, updatedAt}) {
        const revision = Number(expectedRevision);
        if (!Number.isSafeInteger(revision) || revision < 0) throw new TypeError('Affect snapshot.expectedRevision must be a non-negative integer');
        const at = timestamp(updatedAt ?? currentTime(), 'Affect snapshot.updatedAt');
        const existing = readRow(personaId);
        let result;
        if (existing) {
            result = db.prepare(`
                UPDATE companion_persona_affect_states
                SET pleasure = ?, arousal = ?, dominance = ?, drives_json = ?, revision = ?,
                    effective_at = ?, updated_at = ?, source_event_id = ?, model_version = ?
                WHERE persona_id = ? AND revision = ?
            `).run(
                state.pleasure, state.arousal, state.dominance, jsonValue(state.drives, {}, 'Affect snapshot.drives'),
                state.revision, state.effectiveAt, at, state.sourceEventId ?? null, state.modelVersion ?? AFFECT_MODEL_VERSION,
                personaId, revision
            );
        } else {
            if (revision !== 0) return {updated: false, changes: 0, snapshot: null};
            result = db.prepare(`
                INSERT INTO companion_persona_affect_states (
                    persona_id, pleasure, arousal, dominance, drives_json, revision,
                    effective_at, updated_at, source_event_id, model_version
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            `).run(
                personaId, state.pleasure, state.arousal, state.dominance, jsonValue(state.drives, {}, 'Affect snapshot.drives'),
                state.revision, state.effectiveAt, at, state.sourceEventId ?? null, state.modelVersion ?? AFFECT_MODEL_VERSION
            );
        }
        return {updated: result.changes === 1, changes: result.changes, snapshot: result.changes === 1 ? rowToSnapshot(readRow(personaId), policyFor({personaId})) : null};
    }

    function compareAndSwapSnapshot(input = {}) {
        const owner = ownerFrom(input);
        const normalizedPolicy = policyFor({...input, personaId: owner});
        const expectedRevision = input.expectedRevision ?? input.expected_revision;
        const current = readRow(owner);
        const base = current
            ? rowToSnapshot(current, normalizedPolicy)
            : createInitialAffectState({personaId: owner, at: input.at ?? currentTime(), policy: normalizedPolicy});
        const state = normalizeAffectState({...base, ...input, personaId: owner, revision: base.revision}, {policy: normalizedPolicy});
        return writeSnapshot({personaId: owner, state, expectedRevision: expectedRevision ?? base.revision, updatedAt: input.updatedAt ?? currentTime()});
    }

    function commitEvent(event, input = {}) {
        const work = () => {
            const existing = findEventByIdempotencyKey(event);
            if (existing) {
                const snapshot = readSnapshot({personaId: event.personaId, at: event.effectiveAt, policy: input.policy});
                return {created: false, idempotent: true, event: existing, snapshot};
            }
            const normalizedPolicy = policyFor({personaId: event.personaId, at: event.effectiveAt, policy: input.policy});
            const current = readRow(event.personaId);
            const currentState = current
                ? rowToSnapshot(current, normalizedPolicy)
                : createInitialAffectState({personaId: event.personaId, at: event.effectiveAt, policy: normalizedPolicy});
            const result = applyAffectEvent(currentState, event, {
                at: event.effectiveAt,
                policy: normalizedPolicy,
                delta: {
                    eventType: event.eventType,
                    pleasureDelta: event.pleasureDelta,
                    arousalDelta: event.arousalDelta,
                    dominanceDelta: event.dominanceDelta,
                    drivesDelta: event.drivesDelta
                }
            });
            const inserted = insertEvent(event);
            const write = writeSnapshot({
                personaId: event.personaId,
                state: result.state,
                expectedRevision: current?.revision ?? 0,
                updatedAt: event.createdAt
            });
            if (!write.updated) throw new Error('Affect snapshot revision conflict');
            return {created: true, idempotent: false, event: inserted, snapshot: write.snapshot, delta: result.delta};
        };
        return runTransaction(db, work);
    }

    function applyEvent(input = {}) {
        const event = normalizeEventInput(input);
        return commitEvent(event, input);
    }

    function applyDriveSignal(input = {}) {
        const event = normalizeDriveSignalInput(input);
        return commitEvent(event, input);
    }

    function checkpoint(input = {}) {
        const owner = ownerFrom(input);
        const normalizedPolicy = policyFor({...input, personaId: owner});
        const row = readRow(owner);
        const current = row
            ? rowToSnapshot(row, normalizedPolicy)
            : createInitialAffectState({personaId: owner, at: input.at ?? currentTime(), policy: normalizedPolicy});
        const requestedAt = timestamp(input.at ?? currentTime(), 'Affect checkpoint.at');
        const at = row && new Date(requestedAt).getTime() < new Date(current.effectiveAt).getTime() ? current.effectiveAt : requestedAt;
        const state = decayAffectState(current, {at, policy: normalizedPolicy});
        const expectedRevision = input.expectedRevision ?? input.expected_revision ?? current.revision;
        const work = () => writeSnapshot({personaId: owner, state, expectedRevision, updatedAt: input.updatedAt ?? currentTime()});
        return runTransaction(db, work);
    }

    function listEvents(input = {}) {
        const owner = ownerFrom(input);
        return db.prepare(`
            SELECT * FROM companion_persona_affect_events
            WHERE persona_id = ?
            ORDER BY effective_at DESC, id DESC
            LIMIT ?
        `).all(owner, limitValue(input.limit)).map(rowToEvent);
    }

    return Object.freeze({
        readSnapshot,
        read: readSnapshot,
        getSnapshot: readSnapshot,
        findSnapshot,
        compareAndSwapSnapshot,
        updateSnapshot: compareAndSwapSnapshot,
        checkpoint,
        applyEvent,
        applyDriveSignal,
        recordDriveSignal: applyDriveSignal,
        appendEvent: applyEvent,
        recordEvent: applyEvent,
        findEvent,
        findEventByIdempotencyKey,
        listEvents
    });
}

export const createCompanionAffectRepository = createAffectRepository;
export default createAffectRepository;

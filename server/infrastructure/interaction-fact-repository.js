import {randomUUID} from 'node:crypto';

import {INTERACTION_FACT_SCHEMA_VERSION, normalizeInteractionFact} from '../contracts/index.js';

function assertDatabase(database) {
    if (!database || typeof database.prepare !== 'function' || typeof database.transaction !== 'function') {
        throw new TypeError('Interaction fact repository requires an open database');
    }
    return database;
}

function requiredText(value, field, max = 240) {
    if (typeof value !== 'string' || !value.trim()) throw new TypeError(`${field} must be non-empty`);
    const result = value.trim();
    if (result.length > max) throw new RangeError(`${field} exceeds ${max} characters`);
    return result;
}

function optionalText(value, field, max = 240) {
    if (value === undefined || value === null || value === '') return null;
    return requiredText(value, field, max);
}

function timestamp(value, field) {
    const date = value instanceof Date ? value : new Date(value);
    if (!Number.isFinite(date.getTime())) throw new TypeError(`${field} must be a valid timestamp`);
    return date.toISOString();
}

function clockFor(clock) {
    if (clock === undefined) return () => new Date().toISOString();
    if (typeof clock === 'function') return () => timestamp(clock(), 'Interaction fact clock value');
    if (clock && typeof clock.now === 'function') return () => timestamp(clock.now(), 'Interaction fact clock value');
    throw new TypeError('Interaction fact clock must be a function or provide now()');
}

function idFor(value) {
    if (value === undefined) return prefix => `${prefix}_${randomUUID()}`;
    if (typeof value === 'function') return prefix => requiredText(value(prefix), 'Generated interaction fact id');
    if (value && typeof value.next === 'function') return prefix => requiredText(value.next(prefix), 'Generated interaction fact id');
    throw new TypeError('Interaction fact id must be a function or provide next()');
}

function transactionFor(database, transaction) {
    const runner = transaction ?? database;
    if (typeof runner === 'function') return callback => runner(callback);
    if (runner && typeof runner.transaction === 'function') {
        return callback => runner.inTransaction ? callback() : runner.transaction(callback)();
    }
    if (runner && typeof runner.run === 'function') return callback => runner.run(callback);
    return callback => callback();
}

function parseJson(value, fallback = {}) {
    if (value === null || value === undefined || value === '') return fallback;
    try { return typeof value === 'string' ? JSON.parse(value) : value; }
    catch { return fallback; }
}

function rowToFact(row) {
    if (!row) return null;
    return {
        id: row.id,
        schemaVersion: row.schema_version,
        personaId: row.persona_id,
        factType: row.fact_type,
        sourceMessageId: row.source_message_id,
        causationId: row.causation_id,
        idempotencyKey: row.idempotency_key,
        payload: parseJson(row.payload_json),
        source: row.source,
        modelVersion: row.model_version,
        evidenceRefs: parseJson(row.evidence_refs_json, []),
        revision: row.revision,
        createdAt: row.created_at,
        updatedAt: row.updated_at
    };
}

function owner(input) {
    return requiredText(typeof input === 'string' ? input : input?.personaId ?? input?.persona_id, 'Interaction fact personaId', 160);
}

export function createInteractionFactRepository({database, clock, now, id, idGenerator, transaction} = {}) {
    const db = assertDatabase(database);
    const currentTime = clockFor(clock ?? now);
    const nextId = idFor(id ?? idGenerator);
    const runTransaction = transactionFor(db, transaction);

    function findById(value, personaIdValue) {
        const personaId = owner(personaIdValue ?? value);
        const idValue = requiredText(typeof value === 'string' ? value : value?.id, 'Interaction fact id', 240);
        return rowToFact(db.prepare('SELECT * FROM companion_interaction_facts WHERE id = ? AND persona_id = ?').get(idValue, personaId));
    }

    function findByIdempotencyKey(input = {}) {
        const personaId = owner(input);
        const key = requiredText(input.idempotencyKey ?? input.idempotency_key, 'Interaction fact idempotencyKey', 240);
        return rowToFact(db.prepare('SELECT * FROM companion_interaction_facts WHERE persona_id = ? AND idempotency_key = ?').get(personaId, key));
    }

    function normalizeInput(input = {}) {
        const personaId = owner(input);
        const {
            id: _id,
            createdAt: _createdAt,
            created_at: _created_at,
            updatedAt: _updatedAt,
            updated_at: _updated_at,
            revision: _revision,
            ...contractInput
        } = input;
        const fact = normalizeInteractionFact({...contractInput, personaId}, {personaId, sourceMessageId: input.sourceMessageId ?? input.source_message_id});
        const createdAt = timestamp(input.createdAt ?? input.created_at ?? currentTime(), 'Interaction fact createdAt');
        return {
            ...fact,
            id: input.id === undefined || input.id === null ? nextId('interaction_fact') : requiredText(input.id, 'Interaction fact id'),
            schemaVersion: INTERACTION_FACT_SCHEMA_VERSION,
            createdAt,
            updatedAt: timestamp(input.updatedAt ?? input.updated_at ?? createdAt, 'Interaction fact updatedAt'),
            revision: Number.isInteger(input.revision) && input.revision >= 1 ? input.revision : 1
        };
    }

    function record(input = {}) {
        const fact = normalizeInput(input);
        return runTransaction(() => {
            const existing = findByIdempotencyKey(fact);
            if (existing) {
                if (existing.sourceMessageId && fact.sourceMessageId && existing.sourceMessageId !== fact.sourceMessageId) {
                    throw new Error('Interaction fact idempotency key is bound to a different source message');
                }
                return {created: false, replayed: true, changed: false, fact: existing};
            }
            const result = db.prepare(`
                INSERT INTO companion_interaction_facts (
                    id, schema_version, persona_id, fact_type, source_message_id, causation_id,
                    idempotency_key, payload_json, source, model_version, evidence_refs_json,
                    revision, created_at, updated_at
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            `).run(
                fact.id, fact.schemaVersion, fact.personaId, fact.factType, fact.sourceMessageId,
                fact.causationId, fact.idempotencyKey, JSON.stringify(fact.payload), fact.source,
                fact.modelVersion, JSON.stringify(fact.evidenceRefs), fact.revision, fact.createdAt, fact.updatedAt
            );
            return {created: result.changes === 1, replayed: false, changed: result.changes === 1, fact: findById(fact.id, fact.personaId)};
        });
    }

    function compareAndSwap(input = {}) {
        const current = findById(input.id, input.personaId ?? input.persona_id);
        if (!current) return {updated: false, changes: 0, fact: null};
        const expectedRevision = input.expectedRevision ?? input.expected_revision;
        if (!Number.isInteger(expectedRevision) || expectedRevision < 0) throw new TypeError('Interaction fact expectedRevision must be a non-negative integer');
        const updatedAt = timestamp(input.updatedAt ?? input.updated_at ?? currentTime(), 'Interaction fact updatedAt');
        const result = db.prepare(`
            UPDATE companion_interaction_facts
            SET payload_json = ?, evidence_refs_json = ?, revision = ?, updated_at = ?
            WHERE id = ? AND persona_id = ? AND revision = ?
        `).run(
            JSON.stringify(input.payload ?? current.payload), JSON.stringify(input.evidenceRefs ?? current.evidenceRefs),
            expectedRevision + 1, updatedAt, current.id, current.personaId, expectedRevision
        );
        return {updated: result.changes === 1, changes: result.changes, fact: result.changes === 1 ? findById(current.id, current.personaId) : current};
    }

    function list(input = {}) {
        const personaId = owner(input);
        const limit = Number(input.limit ?? 50);
        if (!Number.isInteger(limit) || limit < 1 || limit > 200) throw new RangeError('Interaction fact limit must be between 1 and 200');
        return db.prepare('SELECT * FROM companion_interaction_facts WHERE persona_id = ? ORDER BY created_at DESC, id DESC LIMIT ?').all(personaId, limit).map(rowToFact);
    }

    return Object.freeze({
        schemaVersion: INTERACTION_FACT_SCHEMA_VERSION,
        record,
        insert: record,
        create: record,
        findById,
        findByIdempotencyKey,
        findByIdempotency: findByIdempotencyKey,
        compareAndSwap,
        update: compareAndSwap,
        list
    });
}

export const createCompanionInteractionFactRepository = createInteractionFactRepository;
export default createInteractionFactRepository;

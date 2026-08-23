import {randomUUID} from 'node:crypto';

import {
    MEMORY_CONSOLIDATION_SCHEMA_VERSION,
    MEMORY_CONSOLIDATION_STATUSES,
    normalizeMemoryConsolidationCandidate
} from '../contracts/index.js';

function assertDatabase(database) {
    if (!database || typeof database.prepare !== 'function' || typeof database.transaction !== 'function') {
        throw new TypeError('Memory consolidation repository requires an open database');
    }
    return database;
}

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
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
    if (typeof clock === 'function') return () => timestamp(clock(), 'Memory consolidation clock value');
    if (clock && typeof clock.now === 'function') return () => timestamp(clock.now(), 'Memory consolidation clock value');
    throw new TypeError('Memory consolidation clock must be a function or provide now()');
}

function idFor(value) {
    if (value === undefined) return prefix => `${prefix}_${randomUUID()}`;
    if (typeof value === 'function') return prefix => requiredText(value(prefix), 'Generated memory consolidation id');
    if (value && typeof value.next === 'function') return prefix => requiredText(value.next(prefix), 'Generated memory consolidation id');
    throw new TypeError('Memory consolidation id must be a function or provide next()');
}

function runTransaction(database, transaction, work) {
    const runner = transaction ?? database;
    if (runner === database && database.inTransaction) return work();
    if (typeof runner === 'function') return runner(work);
    if (runner && typeof runner.transaction === 'function') {
        if (runner.inTransaction) return work();
        const result = runner.transaction(work);
        return typeof result === 'function' ? result() : result;
    }
    if (runner && typeof runner.run === 'function') return runner.run(work);
    return work();
}

function parseJson(value, fallback) {
    if (value === undefined || value === null || value === '') return fallback;
    if (typeof value !== 'string') return value;
    try { return JSON.parse(value); } catch { return fallback; }
}

function owner(input) {
    return requiredText(typeof input === 'string' ? input : input?.personaId ?? input?.persona_id, 'Memory consolidation personaId', 160);
}

function rowToCandidate(row) {
    if (!row) return null;
    return {
        id: row.id,
        schemaVersion: row.schema_version,
        personaId: row.persona_id,
        layer: row.layer,
        key: row.memory_key,
        value: parseJson(row.value_json, null),
        claim: row.claim,
        confidence: row.confidence,
        evidenceRefs: parseJson(row.evidence_refs_json, []),
        sourceFactRefs: parseJson(row.source_fact_refs_json, []),
        sourceMessageId: row.source_message_id,
        causationId: row.causation_id,
        idempotencyKey: row.idempotency_key,
        source: row.source,
        modelVersion: row.model_version,
        interactionFactId: row.interaction_fact_id,
        status: row.status,
        error: row.error,
        revision: row.revision,
        createdAt: row.created_at,
        updatedAt: row.updated_at
    };
}

function normalizeStatus(value) {
    if (!MEMORY_CONSOLIDATION_STATUSES.includes(value)) {
        throw new TypeError(`Unsupported memory consolidation status: ${String(value)}`);
    }
    return value;
}

export function createMemoryConsolidationRepository({database, clock, now, id, idGenerator, transaction} = {}) {
    const db = assertDatabase(database);
    const currentTime = clockFor(clock ?? now);
    const nextId = idFor(id ?? idGenerator);

    function findById(value, personaIdValue) {
        const personaId = owner(personaIdValue ?? value);
        const idValue = requiredText(typeof value === 'string' ? value : value?.id, 'Memory consolidation id');
        return rowToCandidate(db.prepare(
            'SELECT * FROM companion_memory_consolidation_candidates WHERE id = ? AND persona_id = ?'
        ).get(idValue, personaId));
    }

    function findByIdempotencyKey(input = {}) {
        const personaId = owner(input);
        const key = requiredText(input.idempotencyKey ?? input.idempotency_key, 'Memory consolidation idempotencyKey', 240);
        return rowToCandidate(db.prepare(
            'SELECT * FROM companion_memory_consolidation_candidates WHERE persona_id = ? AND idempotency_key = ?'
        ).get(personaId, key));
    }

    function normalizeInput(input = {}) {
        if (!isRecord(input)) throw new TypeError('Memory consolidation input must be an object');
        const personaId = owner(input);
        const {
            id: _id,
            status: _status,
            error: _error,
            createdAt: _createdAt,
            created_at: _created_at,
            updatedAt: _updatedAt,
            updated_at: _updated_at,
            revision: _revision,
            ...candidateInput
        } = input;
        const candidate = normalizeMemoryConsolidationCandidate({...candidateInput, personaId}, {
            personaId,
            sourceMessageId: input.sourceMessageId ?? input.source_message_id
        });
        const createdAt = timestamp(input.createdAt ?? input.created_at ?? currentTime(), 'Memory consolidation createdAt');
        const status = normalizeStatus(input.status ?? 'candidate');
        const revision = input.revision === undefined ? 1 : input.revision;
        if (!Number.isInteger(revision) || revision < 1) throw new TypeError('Memory consolidation revision must be a positive integer');
        return {
            ...candidate,
            id: input.id === undefined || input.id === null ? nextId('memory_consolidation') : requiredText(input.id, 'Memory consolidation id'),
            schemaVersion: MEMORY_CONSOLIDATION_SCHEMA_VERSION,
            status,
            error: optionalText(input.error, 'Memory consolidation error', 240),
            revision,
            createdAt,
            updatedAt: timestamp(input.updatedAt ?? input.updated_at ?? createdAt, 'Memory consolidation updatedAt')
        };
    }

    function insert(input = {}) {
        const candidate = normalizeInput(input);
        return runTransaction(db, transaction, () => {
            const existing = findByIdempotencyKey(candidate);
            if (existing) {
                if (existing.sourceMessageId && candidate.sourceMessageId && existing.sourceMessageId !== candidate.sourceMessageId) {
                    throw new Error('Memory consolidation idempotency key is bound to a different source message');
                }
                return {created: false, replayed: true, changed: false, candidate: existing};
            }
            const result = db.prepare(`
                INSERT INTO companion_memory_consolidation_candidates (
                    id, schema_version, persona_id, layer, memory_key, value_json, claim, confidence,
                    evidence_refs_json, source_fact_refs_json, source_message_id, causation_id,
                    idempotency_key, source, model_version, interaction_fact_id, status, error,
                    revision, created_at, updated_at
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            `).run(
                candidate.id, candidate.schemaVersion, candidate.personaId, candidate.layer,
                candidate.key, candidate.key === null ? null : JSON.stringify(candidate.value), candidate.claim,
                candidate.confidence, JSON.stringify(candidate.evidenceRefs), JSON.stringify(candidate.sourceFactRefs),
                candidate.sourceMessageId, candidate.causationId, candidate.idempotencyKey, candidate.source,
                candidate.modelVersion, candidate.interactionFactId, candidate.status, candidate.error,
                candidate.revision, candidate.createdAt, candidate.updatedAt
            );
            return {
                created: result.changes === 1,
                replayed: false,
                changed: result.changes === 1,
                candidate: findById(candidate.id, candidate.personaId)
            };
        });
    }

    function compareAndSwap(input = {}) {
        const personaId = owner(input);
        const candidateId = requiredText(input.id ?? input.candidateId, 'Memory consolidation id');
        const expectedRevision = input.expectedRevision ?? input.expected_revision;
        if (!Number.isInteger(expectedRevision) || expectedRevision < 0) {
            throw new TypeError('Memory consolidation expectedRevision must be a non-negative integer');
        }
        const current = findById(candidateId, personaId);
        if (!current) return {updated: false, changes: 0, candidate: null};
        const status = input.status === undefined ? current.status : normalizeStatus(input.status);
        const error = input.error === undefined ? current.error : optionalText(input.error, 'Memory consolidation error', 240);
        const evidenceRefs = input.evidenceRefs === undefined ? current.evidenceRefs : input.evidenceRefs;
        const sourceFactRefs = input.sourceFactRefs === undefined ? current.sourceFactRefs : input.sourceFactRefs;
        const value = input.value === undefined ? current.value : input.value;
        const claim = input.claim === undefined ? current.claim : input.claim;
        const updatedAt = timestamp(input.updatedAt ?? input.updated_at ?? currentTime(), 'Memory consolidation updatedAt');
        const result = db.prepare(`
            UPDATE companion_memory_consolidation_candidates
            SET value_json = ?, claim = ?, confidence = ?, evidence_refs_json = ?, source_fact_refs_json = ?,
                status = ?, error = ?, revision = ?, updated_at = ?
            WHERE id = ? AND persona_id = ? AND revision = ?
        `).run(
            current.key === null ? null : JSON.stringify(value), claim,
            input.confidence === undefined ? current.confidence : Number(input.confidence),
            JSON.stringify(evidenceRefs), JSON.stringify(sourceFactRefs), status, error,
            expectedRevision + 1, updatedAt, candidateId, personaId, expectedRevision
        );
        return {
            updated: result.changes === 1,
            changes: result.changes,
            candidate: findById(candidateId, personaId)
        };
    }

    function list(input = {}) {
        const personaId = owner(input);
        const limit = Number(input.limit ?? 50);
        if (!Number.isInteger(limit) || limit < 1 || limit > 200) throw new RangeError('Memory consolidation limit must be between 1 and 200');
        return db.prepare(`
            SELECT * FROM companion_memory_consolidation_candidates
            WHERE persona_id = ? ORDER BY created_at DESC, id DESC LIMIT ?
        `).all(personaId, limit).map(rowToCandidate);
    }

    return Object.freeze({
        schemaVersion: MEMORY_CONSOLIDATION_SCHEMA_VERSION,
        insert,
        record: insert,
        create: insert,
        findById,
        findByIdempotencyKey,
        findByIdempotency: findByIdempotencyKey,
        compareAndSwap,
        update: compareAndSwap,
        close: input => compareAndSwap({...input, status: input.status ?? 'rejected'}),
        reject: input => compareAndSwap({...input, status: 'rejected'}),
        list
    });
}

export const createCompanionMemoryConsolidationRepository = createMemoryConsolidationRepository;
export default createMemoryConsolidationRepository;

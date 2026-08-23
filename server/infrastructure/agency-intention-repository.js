import {randomUUID} from 'node:crypto';

import {
    AGENCY_INTENTION_SCHEMA_VERSION,
    AGENCY_INTENTION_STATUSES,
    normalizeAgencyIntention
} from '../contracts/index.js';

function requiredText(value, field, max = 240) {
    if (typeof value !== 'string' || !value.trim()) throw new TypeError(`${field} must be non-empty`);
    const text = value.trim();
    if (text.length > max) throw new RangeError(`${field} exceeds ${max} characters`);
    return text;
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
    if (typeof clock === 'function') return () => timestamp(clock(), 'Agency intention clock value');
    if (clock && typeof clock.now === 'function') return () => timestamp(clock.now(), 'Agency intention clock value');
    throw new TypeError('Agency intention clock must be a function or provide now()');
}

function idFor(value) {
    if (value === undefined) return prefix => `${prefix}_${randomUUID()}`;
    if (typeof value === 'function') return prefix => requiredText(value(prefix), 'Generated agency intention id');
    if (value && typeof value.next === 'function') return prefix => requiredText(value.next(prefix), 'Generated agency intention id');
    throw new TypeError('Agency intention id must be a function or provide next()');
}

function parseJson(value, fallback = null) {
    if (value === undefined || value === null || value === '') return fallback;
    try { return typeof value === 'string' ? JSON.parse(value) : value; } catch { return fallback; }
}

function owner(input) {
    return requiredText(typeof input === 'string' ? input : input?.personaId ?? input?.persona_id, 'Agency intention personaId', 160);
}

function rowToIntention(row) {
    if (!row) return null;
    return {
        id: row.id,
        schemaVersion: row.schema_version,
        personaId: row.persona_id,
        intent: row.intent,
        topic: row.topic,
        explanation: row.explanation,
        reasonCategory: row.reason_category,
        confidence: row.confidence,
        evidenceRefs: parseJson(row.evidence_refs_json, []),
        source: row.source,
        sourceMessageId: row.source_message_id,
        causationId: row.causation_id,
        idempotencyKey: row.idempotency_key,
        modelVersion: row.model_version,
        interactionFactId: row.interaction_fact_id,
        decision: parseJson(row.decision_json, null),
        status: row.status,
        error: row.error,
        revision: row.revision,
        createdAt: row.created_at,
        updatedAt: row.updated_at
    };
}

export function createAgencyIntentionRepository({database, clock, now, id, idGenerator, transaction} = {}) {
    if (!database || typeof database.prepare !== 'function' || typeof database.transaction !== 'function') {
        throw new TypeError('Agency intention repository requires an open database');
    }
    const db = database;
    const currentTime = clockFor(clock ?? now);
    const nextId = idFor(id ?? idGenerator);
    const runTransaction = work => {
        const runner = transaction ?? db;
        if (runner === db && db.inTransaction) return work();
        if (typeof runner === 'function') return runner(work);
        if (runner && typeof runner.transaction === 'function') {
            if (runner.inTransaction) return work();
            const result = runner.transaction(work);
            return typeof result === 'function' ? result() : result;
        }
        return work();
    };

    function findById(value, personaIdValue) {
        const personaId = owner(personaIdValue ?? value);
        const id = requiredText(typeof value === 'string' ? value : value?.id, 'Agency intention id');
        return rowToIntention(db.prepare('SELECT * FROM companion_agency_intentions WHERE id = ? AND persona_id = ?').get(id, personaId));
    }

    function findByIdempotencyKey(input = {}) {
        const personaId = owner(input);
        const key = requiredText(input.idempotencyKey ?? input.idempotency_key, 'Agency intention idempotencyKey');
        return rowToIntention(db.prepare('SELECT * FROM companion_agency_intentions WHERE persona_id = ? AND idempotency_key = ?').get(personaId, key));
    }

    function insert(input = {}) {
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
            ...contractInput
        } = input;
        const intention = normalizeAgencyIntention({...contractInput, personaId}, {personaId, sourceMessageId: input.sourceMessageId ?? input.source_message_id});
        const createdAt = timestamp(input.createdAt ?? input.created_at ?? currentTime(), 'Agency intention createdAt');
        const status = input.status ?? 'candidate';
        if (!AGENCY_INTENTION_STATUSES.includes(status)) throw new TypeError(`Unsupported agency intention status: ${String(status)}`);
        return runTransaction(() => {
            const existing = findByIdempotencyKey(intention);
            if (existing) {
                if (existing.sourceMessageId && intention.sourceMessageId && existing.sourceMessageId !== intention.sourceMessageId) {
                    throw new Error('Agency intention idempotency key is bound to a different source message');
                }
                return {created: false, replayed: true, changed: false, intention: existing};
            }
            const id = input.id === undefined ? nextId('agency_intention') : requiredText(input.id, 'Agency intention id');
            const revision = Number.isInteger(input.revision) && input.revision >= 1 ? input.revision : 1;
            const result = db.prepare(`
                INSERT INTO companion_agency_intentions (
                    id, schema_version, persona_id, intent, topic, explanation, reason_category, confidence,
                    evidence_refs_json, source, source_message_id, causation_id, idempotency_key, model_version,
                    interaction_fact_id, decision_json, status, error, revision, created_at, updated_at
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            `).run(
                id, AGENCY_INTENTION_SCHEMA_VERSION, personaId, intention.intent, intention.topic,
                intention.explanation, intention.reasonCategory, intention.confidence, JSON.stringify(intention.evidenceRefs),
                intention.source, intention.sourceMessageId, intention.causationId, intention.idempotencyKey,
                intention.modelVersion, intention.interactionFactId, intention.decision === null ? null : JSON.stringify(intention.decision),
                status, optionalText(input.error, 'Agency intention error'), revision, createdAt,
                timestamp(input.updatedAt ?? input.updated_at ?? createdAt, 'Agency intention updatedAt')
            );
            return {created: result.changes === 1, replayed: false, changed: result.changes === 1, intention: findById(id, personaId)};
        });
    }

    function compareAndSwap(input = {}) {
        const personaId = owner(input);
        const id = requiredText(input.id ?? input.intentionId, 'Agency intention id');
        const expectedRevision = input.expectedRevision ?? input.expected_revision;
        if (!Number.isInteger(expectedRevision) || expectedRevision < 0) throw new TypeError('Agency intention expectedRevision must be a non-negative integer');
        const current = findById(id, personaId);
        if (!current) return {updated: false, changes: 0, intention: null};
        const status = input.status ?? current.status;
        if (!AGENCY_INTENTION_STATUSES.includes(status)) throw new TypeError(`Unsupported agency intention status: ${String(status)}`);
        const updatedAt = timestamp(input.updatedAt ?? input.updated_at ?? currentTime(), 'Agency intention updatedAt');
        const result = db.prepare(`
            UPDATE companion_agency_intentions
            SET status = ?, decision_json = ?, error = ?, revision = ?, updated_at = ?
            WHERE id = ? AND persona_id = ? AND revision = ?
        `).run(
            status,
            input.decision === undefined ? (current.decision === null ? null : JSON.stringify(current.decision)) : JSON.stringify(input.decision),
            optionalText(input.error, 'Agency intention error'), expectedRevision + 1, updatedAt, id, personaId, expectedRevision
        );
        return {updated: result.changes === 1, changes: result.changes, intention: findById(id, personaId)};
    }

    function list(input = {}) {
        const personaId = owner(input);
        const limit = Number(input.limit ?? 50);
        if (!Number.isInteger(limit) || limit < 1 || limit > 200) throw new RangeError('Agency intention limit must be between 1 and 200');
        return db.prepare('SELECT * FROM companion_agency_intentions WHERE persona_id = ? ORDER BY created_at DESC, id DESC LIMIT ?').all(personaId, limit).map(rowToIntention);
    }

    return Object.freeze({schemaVersion: AGENCY_INTENTION_SCHEMA_VERSION, insert, record: insert, create: insert, findById, findByIdempotencyKey, findByIdempotency: findByIdempotencyKey, compareAndSwap, update: compareAndSwap, list});
}

export const createCompanionAgencyIntentionRepository = createAgencyIntentionRepository;
export default createAgencyIntentionRepository;

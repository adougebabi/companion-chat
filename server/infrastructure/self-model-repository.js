import {randomUUID} from 'node:crypto';

import {
    SELF_MODEL_CLAIM_SCHEMA_VERSION,
    SELF_MODEL_CLAIM_STATUSES,
    normalizeSelfModelClaim
} from '../contracts/index.js';

function assertDatabase(database) {
    if (!database || typeof database.prepare !== 'function' || typeof database.transaction !== 'function') {
        throw new TypeError('Self-model repository requires an open database');
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
    if (typeof clock === 'function') return () => timestamp(clock(), 'Self-model clock value');
    if (clock && typeof clock.now === 'function') return () => timestamp(clock.now(), 'Self-model clock value');
    throw new TypeError('Self-model clock must be a function or provide now()');
}

function idFor(value) {
    if (value === undefined) return prefix => `${prefix}_${randomUUID()}`;
    if (typeof value === 'function') return prefix => requiredText(value(prefix), 'Generated self-model claim id');
    if (value && typeof value.next === 'function') return prefix => requiredText(value.next(prefix), 'Generated self-model claim id');
    throw new TypeError('Self-model id must be a function or provide next()');
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

function parseJson(value, fallback = null) {
    if (value === null || value === undefined || value === '') return fallback;
    try { return typeof value === 'string' ? JSON.parse(value) : value; }
    catch { return fallback; }
}

function rowToClaim(row) {
    if (!row) return null;
    return {
        id: row.id,
        schemaVersion: row.schema_version,
        personaId: row.persona_id,
        category: row.category,
        claim: row.claim,
        summary: row.summary,
        confidence: row.confidence,
        evidenceRefs: parseJson(row.evidence_refs_json, []),
        source: row.source,
        uncertainty: parseJson(row.uncertainty_json, null),
        sourceMessageId: row.source_message_id,
        causationId: row.causation_id,
        idempotencyKey: row.idempotency_key,
        modelVersion: row.model_version,
        interactionFactId: row.interaction_fact_id,
        decayPolicy: parseJson(row.decay_policy_json, null),
        status: row.status,
        error: row.error,
        revision: row.revision,
        createdAt: row.created_at,
        updatedAt: row.updated_at
    };
}

function owner(input) {
    return requiredText(typeof input === 'string' ? input : input?.personaId ?? input?.persona_id, 'Self-model personaId', 160);
}

export function createSelfModelRepository({database, clock, now, id, idGenerator, transaction} = {}) {
    const db = assertDatabase(database);
    const currentTime = clockFor(clock ?? now);
    const nextId = idFor(id ?? idGenerator);
    const runTransaction = transactionFor(db, transaction);

    function findById(value, personaIdValue) {
        const personaId = owner(personaIdValue ?? value);
        const idValue = requiredText(typeof value === 'string' ? value : value?.id ?? value?.claimId, 'Self-model claim id');
        return rowToClaim(db.prepare('SELECT * FROM companion_self_model_claims WHERE id = ? AND persona_id = ?').get(idValue, personaId));
    }

    function findByIdempotencyKey(input = {}) {
        const personaId = owner(input);
        const key = requiredText(input.idempotencyKey ?? input.idempotency_key, 'Self-model claim idempotencyKey', 240);
        return rowToClaim(db.prepare('SELECT * FROM companion_self_model_claims WHERE persona_id = ? AND idempotency_key = ?').get(personaId, key));
    }

    function normalizeInput(input = {}) {
        const personaId = owner(input);
        const {
            id: _id,
            claimId: _claimId,
            status: _status,
            error: _error,
            createdAt: _createdAt,
            created_at: _created_at,
            updatedAt: _updatedAt,
            updated_at: _updated_at,
            revision: _revision,
            ...contractInput
        } = input;
        const claim = normalizeSelfModelClaim({...contractInput, personaId}, {
            personaId,
            sourceMessageId: input.sourceMessageId ?? input.source_message_id,
            causationId: input.causationId ?? input.causation_id,
            modelVersion: input.modelVersion ?? input.model_version
        });
        const status = input.status ?? 'candidate';
        if (!SELF_MODEL_CLAIM_STATUSES.includes(status)) throw new TypeError(`Unsupported self-model claim status: ${String(status)}`);
        const createdAt = timestamp(input.createdAt ?? input.created_at ?? currentTime(), 'Self-model claim createdAt');
        return {
            ...claim,
            id: input.id === undefined || input.id === null ? nextId('self_model_claim') : requiredText(input.id, 'Self-model claim id'),
            schemaVersion: SELF_MODEL_CLAIM_SCHEMA_VERSION,
            status,
            error: optionalText(input.error, 'Self-model claim error', 240),
            revision: Number.isInteger(input.revision) && input.revision >= 1 ? input.revision : 1,
            createdAt,
            updatedAt: timestamp(input.updatedAt ?? input.updated_at ?? createdAt, 'Self-model claim updatedAt')
        };
    }

    function insert(input = {}) {
        const claim = normalizeInput(input);
        return runTransaction(() => {
            const existing = findByIdempotencyKey(claim);
            if (existing) {
                if (existing.sourceMessageId && claim.sourceMessageId && existing.sourceMessageId !== claim.sourceMessageId) {
                    throw new Error('Self-model claim idempotency key is bound to a different source message');
                }
                return {created: false, replayed: true, changed: false, claim: existing};
            }
            const result = db.prepare(`
                INSERT INTO companion_self_model_claims (
                    id, schema_version, persona_id, category, claim, summary, confidence,
                    evidence_refs_json, source, uncertainty_json, source_message_id, causation_id,
                    idempotency_key, model_version, interaction_fact_id, decay_policy_json,
                    status, error, revision, created_at, updated_at
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            `).run(
                claim.id, claim.schemaVersion, claim.personaId, claim.category, claim.claim, claim.summary,
                claim.confidence, JSON.stringify(claim.evidenceRefs), claim.source,
                claim.uncertainty === null ? null : JSON.stringify(claim.uncertainty), claim.sourceMessageId,
                claim.causationId, claim.idempotencyKey, claim.modelVersion, claim.interactionFactId,
                claim.decayPolicy === null ? null : JSON.stringify(claim.decayPolicy), claim.status, claim.error,
                claim.revision, claim.createdAt, claim.updatedAt
            );
            return {created: result.changes === 1, replayed: false, changed: result.changes === 1, claim: findById(claim.id, claim.personaId)};
        });
    }

    function compareAndSwap(input = {}) {
        const personaId = owner(input);
        const claimId = requiredText(input.id ?? input.claimId, 'Self-model claim id');
        const expectedRevision = input.expectedRevision ?? input.expected_revision;
        if (!Number.isInteger(expectedRevision) || expectedRevision < 0) throw new TypeError('Self-model claim expectedRevision must be a non-negative integer');
        const current = findById(claimId, personaId);
        if (!current) return {updated: false, changes: 0, claim: null};
        const status = input.status ?? current.status;
        if (!SELF_MODEL_CLAIM_STATUSES.includes(status)) throw new TypeError(`Unsupported self-model claim status: ${String(status)}`);
        const updatedAt = timestamp(input.updatedAt ?? input.updated_at ?? currentTime(), 'Self-model claim updatedAt');
        const result = db.prepare(`
            UPDATE companion_self_model_claims SET status = ?, error = ?, revision = ?, updated_at = ?
            WHERE id = ? AND persona_id = ? AND revision = ?
        `).run(status, optionalText(input.error, 'Self-model claim error', 240), expectedRevision + 1, updatedAt, claimId, personaId, expectedRevision);
        return {updated: result.changes === 1, changes: result.changes, claim: findById(claimId, personaId)};
    }

    function list(input = {}) {
        const personaId = owner(input);
        const limit = Number(input.limit ?? 50);
        if (!Number.isInteger(limit) || limit < 1 || limit > 200) throw new RangeError('Self-model claim limit must be between 1 and 200');
        return db.prepare('SELECT * FROM companion_self_model_claims WHERE persona_id = ? ORDER BY created_at DESC, id DESC LIMIT ?').all(personaId, limit).map(rowToClaim);
    }

    function listActive(input = {}) {
        const personaId = owner(input);
        const limit = Number(input.limit ?? 12);
        if (!Number.isInteger(limit) || limit < 1 || limit > 100) throw new RangeError('Self-model active claim limit must be between 1 and 100');
        return db.prepare(`
            SELECT * FROM companion_self_model_claims
            WHERE persona_id = ? AND status IN ('active', 'confirmed')
            ORDER BY updated_at DESC, id DESC LIMIT ?
        `).all(personaId, limit).map(rowToClaim);
    }

    return Object.freeze({
        schemaVersion: SELF_MODEL_CLAIM_SCHEMA_VERSION,
        insert,
        record: insert,
        create: insert,
        findById,
        findByIdempotencyKey,
        findByIdempotency: findByIdempotencyKey,
        compareAndSwap,
        update: compareAndSwap,
        list,
        listActive
    });
}

export const createSelfModelClaimRepository = createSelfModelRepository;
export const createCompanionSelfModelRepository = createSelfModelRepository;
export default createSelfModelRepository;

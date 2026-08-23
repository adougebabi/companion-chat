import {randomUUID} from 'node:crypto';

import {APPRAISAL_SCHEMA_VERSION, normalizeAppraisalCandidate} from '../contracts/index.js';

function assertDatabase(database) {
    if (!database || typeof database.prepare !== 'function' || typeof database.transaction !== 'function') {
        throw new TypeError('Appraisal repository requires an open database');
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
    if (typeof clock === 'function') return () => timestamp(clock(), 'Appraisal clock value');
    if (clock && typeof clock.now === 'function') return () => timestamp(clock.now(), 'Appraisal clock value');
    throw new TypeError('Appraisal clock must be a function or provide now()');
}

function idFor(value) {
    if (value === undefined) return prefix => `${prefix}_${randomUUID()}`;
    if (typeof value === 'function') return prefix => requiredText(value(prefix), 'Generated appraisal id');
    if (value && typeof value.next === 'function') return prefix => requiredText(value.next(prefix), 'Generated appraisal id');
    throw new TypeError('Appraisal id must be a function or provide next()');
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

function rowToAppraisal(row) {
    if (!row) return null;
    return {
        id: row.id,
        schemaVersion: row.schema_version,
        personaId: row.persona_id,
        interactionFactId: row.interaction_fact_id,
        sourceMessageId: row.source_message_id,
        causationId: row.causation_id,
        category: row.category,
        confidence: row.confidence,
        rationale: row.rationale,
        evidenceRefs: parseJson(row.evidence_refs_json, []),
        affectEvents: parseJson(row.affect_events_json, []),
        driveSignals: parseJson(row.drive_signals_json, []),
        idempotencyKey: row.idempotency_key,
        modelVersion: row.model_version,
        source: row.source,
        status: row.status,
        error: row.error,
        revision: row.revision,
        createdAt: row.created_at,
        updatedAt: row.updated_at
    };
}

function owner(input) {
    return requiredText(typeof input === 'string' ? input : input?.personaId ?? input?.persona_id, 'Appraisal personaId', 160);
}

const STATUSES = new Set(['candidate', 'applied', 'no_op', 'deferred', 'rejected']);

export function createAppraisalRepository({database, clock, now, id, idGenerator, transaction} = {}) {
    const db = assertDatabase(database);
    const currentTime = clockFor(clock ?? now);
    const nextId = idFor(id ?? idGenerator);
    const runTransaction = transactionFor(db, transaction);

    function findById(value, personaIdValue) {
        const personaId = owner(personaIdValue ?? value);
        const idValue = requiredText(typeof value === 'string' ? value : value?.id, 'Appraisal id');
        return rowToAppraisal(db.prepare('SELECT * FROM companion_appraisals WHERE id = ? AND persona_id = ?').get(idValue, personaId));
    }

    function findByIdempotencyKey(input = {}) {
        const personaId = owner(input);
        const key = requiredText(input.idempotencyKey ?? input.idempotency_key, 'Appraisal idempotencyKey', 240);
        return rowToAppraisal(db.prepare('SELECT * FROM companion_appraisals WHERE persona_id = ? AND idempotency_key = ?').get(personaId, key));
    }

    function normalizeInput(input = {}) {
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
        const appraisal = normalizeAppraisalCandidate({...contractInput, personaId}, {personaId, sourceMessageId: input.sourceMessageId ?? input.source_message_id});
        const createdAt = timestamp(input.createdAt ?? input.created_at ?? currentTime(), 'Appraisal createdAt');
        const status = input.status ?? 'candidate';
        if (!STATUSES.has(status)) throw new TypeError(`Unsupported appraisal status: ${String(status)}`);
        return {
            ...appraisal,
            id: input.id === undefined || input.id === null ? nextId('appraisal') : requiredText(input.id, 'Appraisal id'),
            schemaVersion: APPRAISAL_SCHEMA_VERSION,
            status,
            error: optionalText(input.error, 'Appraisal error', 240),
            revision: Number.isInteger(input.revision) && input.revision >= 1 ? input.revision : 1,
            createdAt,
            updatedAt: timestamp(input.updatedAt ?? input.updated_at ?? createdAt, 'Appraisal updatedAt')
        };
    }

    function insert(input = {}) {
        const appraisal = normalizeInput(input);
        return runTransaction(() => {
            const existing = findByIdempotencyKey(appraisal);
            if (existing) {
                if (existing.sourceMessageId && appraisal.sourceMessageId && existing.sourceMessageId !== appraisal.sourceMessageId) {
                    throw new Error('Appraisal idempotency key is bound to a different source message');
                }
                return {created: false, replayed: true, changed: false, appraisal: existing};
            }
            const result = db.prepare(`
                INSERT INTO companion_appraisals (
                    id, schema_version, persona_id, interaction_fact_id, source_message_id, causation_id,
                    category, confidence, rationale, evidence_refs_json, affect_events_json, drive_signals_json,
                    idempotency_key, model_version, source, status, error, revision, created_at, updated_at
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            `).run(
                appraisal.id, appraisal.schemaVersion, appraisal.personaId, appraisal.interactionFactId,
                appraisal.sourceMessageId, appraisal.causationId, appraisal.category, appraisal.confidence,
                appraisal.rationale, JSON.stringify(appraisal.evidenceRefs), JSON.stringify(appraisal.affectEvents),
                JSON.stringify(appraisal.driveSignals), appraisal.idempotencyKey, appraisal.modelVersion,
                appraisal.source, appraisal.status, appraisal.error, appraisal.revision, appraisal.createdAt, appraisal.updatedAt
            );
            return {created: result.changes === 1, replayed: false, changed: result.changes === 1, appraisal: findById(appraisal.id, appraisal.personaId)};
        });
    }

    function updateStatus(input = {}) {
        const personaId = owner(input);
        const appraisalId = requiredText(input.id ?? input.appraisalId, 'Appraisal id');
        const status = input.status;
        if (!STATUSES.has(status)) throw new TypeError(`Unsupported appraisal status: ${String(status)}`);
        const expectedRevision = input.expectedRevision ?? input.expected_revision;
        if (!Number.isInteger(expectedRevision) || expectedRevision < 1) throw new TypeError('Appraisal expectedRevision must be a positive integer');
        const updatedAt = timestamp(input.updatedAt ?? input.updated_at ?? currentTime(), 'Appraisal updatedAt');
        const result = db.prepare(`
            UPDATE companion_appraisals SET status = ?, error = ?, revision = ?, updated_at = ?
            WHERE id = ? AND persona_id = ? AND revision = ?
        `).run(status, optionalText(input.error, 'Appraisal error', 240), expectedRevision + 1, updatedAt, appraisalId, personaId, expectedRevision);
        return {updated: result.changes === 1, changes: result.changes, appraisal: findById(appraisalId, personaId)};
    }

    function compareAndSwap(input = {}) {
        return updateStatus(input);
    }

    function list(input = {}) {
        const personaId = owner(input);
        const limit = Number(input.limit ?? 50);
        if (!Number.isInteger(limit) || limit < 1 || limit > 200) throw new RangeError('Appraisal limit must be between 1 and 200');
        return db.prepare('SELECT * FROM companion_appraisals WHERE persona_id = ? ORDER BY created_at DESC, id DESC LIMIT ?').all(personaId, limit).map(rowToAppraisal);
    }

    return Object.freeze({
        schemaVersion: APPRAISAL_SCHEMA_VERSION,
        insert,
        record: insert,
        create: insert,
        findById,
        findByIdempotencyKey,
        findByIdempotency: findByIdempotencyKey,
        updateStatus,
        compareAndSwap,
        update: updateStatus,
        list
    });
}

export const createCompanionAppraisalRepository = createAppraisalRepository;
export default createAppraisalRepository;

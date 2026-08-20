function assertOpenDatabase(database) {
    if (!database || typeof database.prepare !== 'function') {
        throw new TypeError('Pending-event repository requires an open database');
    }
    return database;
}

function assertEnqueueJob(enqueueJob) {
    if (typeof enqueueJob !== 'function') {
        throw new TypeError('Pending-event repository requires an enqueueJob function');
    }
    return enqueueJob;
}

function payloadJsonFor(input) {
    if (input.payloadJson !== undefined) {
        if (typeof input.payloadJson !== 'string') throw new TypeError('Pending-event payloadJson must be a string');
        return input.payloadJson;
    }
    return JSON.stringify(input.payload === undefined ? {} : input.payload);
}

/**
 * Create a repository for the pending-event table and its durable jobs.
 *
 * The caller owns normalization, domain validation, transaction boundaries,
 * IDs, timestamps, and job policy. This adapter only performs parameterized
 * SQL against the existing database and asks the injected queue function to
 * create a missing linked job.
 */
export function createPendingEventRepository({database, enqueueJob} = {}) {
    const openDatabase = assertOpenDatabase(database);
    const enqueue = assertEnqueueJob(enqueueJob);

    function findByDedupeKey({personaId, dedupeKey, notBefore} = {}) {
        return openDatabase.prepare(`
            SELECT * FROM companion_pending_events
            WHERE persona_id = ? AND dedupe_key = ? AND not_before = ?
        `).get(personaId, dedupeKey, notBefore);
    }

    function insertPendingEvent(input = {}) {
        const createdAt = input.createdAt;
        const updatedAt = input.updatedAt === undefined ? createdAt : input.updatedAt;
        openDatabase.prepare(`
            INSERT INTO companion_pending_events (
                id, persona_id, source_message_id, status, summary,
                not_before, expires_at, dedupe_key, payload_json,
                created_at, updated_at
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        `).run(
            input.id,
            input.personaId,
            input.sourceMessageId ?? null,
            input.status,
            input.summary,
            input.notBefore,
            input.expiresAt,
            input.dedupeKey,
            payloadJsonFor(input),
            createdAt,
            updatedAt
        );
        return openDatabase.prepare('SELECT * FROM companion_pending_events WHERE id = ?').get(input.id);
    }

    function findLinkedJob({personaId, pendingEventId} = {}) {
        return openDatabase.prepare(`
            SELECT * FROM companion_jobs
            WHERE persona_id = ?
              AND job_type = ?
              AND json_extract(payload_json, '$.pendingEventId') = ?
            ORDER BY created_at DESC, id DESC
            LIMIT 1
        `).get(personaId, 'pending_event', pendingEventId);
    }

    function assertPendingEventScope({personaId, pendingEventId} = {}) {
        const pendingEvent = openDatabase.prepare(`
            SELECT id FROM companion_pending_events
            WHERE id = ? AND persona_id = ?
        `).get(pendingEventId, personaId);
        if (!pendingEvent) throw new Error('Pending event does not belong to persona');
    }

    function ensureLinkedJob({personaId, pendingEventId, job, jobInput} = {}) {
        assertPendingEventScope({personaId, pendingEventId});
        const existing = findLinkedJob({personaId, pendingEventId});
        if (existing) return {job: existing, created: false};

        const queued = enqueue(jobInput ?? job);
        const persisted = findLinkedJob({personaId, pendingEventId});
        return {job: persisted || queued || null, created: true};
    }

    return Object.freeze({findByDedupeKey, insertPendingEvent, findLinkedJob, ensureLinkedJob});
}

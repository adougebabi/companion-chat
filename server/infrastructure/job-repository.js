import {randomUUID} from 'node:crypto';

export const MAX_JOB_ERROR_LENGTH = 500;

function assertOpenDatabase(database) {
    if (!database || typeof database.prepare !== 'function' || typeof database.transaction !== 'function') {
        throw new TypeError('Job repository requires an open database');
    }
    return database;
}

function assertRecord(value, name) {
    if (!value || typeof value !== 'object' || Array.isArray(value)) {
        throw new TypeError(`${name} must be an object`);
    }
    return value;
}

function requiredText(value, name) {
    if (typeof value !== 'string' || !value) throw new TypeError(`${name} must be a non-empty string`);
    return value;
}

function timestamp(value, name) {
    const resolved = value instanceof Date ? value.toISOString() : value;
    return requiredText(resolved, name);
}

function valueFor(input, camelName, snakeName) {
    return input[camelName] === undefined ? input[snakeName] : input[camelName];
}

function makeClock(clock) {
    if (clock === undefined) return () => new Date().toISOString();
    if (typeof clock === 'function') return () => timestamp(clock(), 'Job clock value');
    if (clock && typeof clock.now === 'function') return () => timestamp(clock.now(), 'Job clock value');
    throw new TypeError('Job repository clock must be a function or {now()} object');
}

function makeIdGenerator(id, idGenerator) {
    const configured = id ?? idGenerator;
    if (configured === undefined) return prefix => `${prefix}_${randomUUID()}`;
    if (typeof configured === 'function') return prefix => requiredText(configured(prefix), 'Generated job id');
    if (configured && typeof configured.next === 'function') {
        return prefix => requiredText(configured.next(prefix), 'Generated job id');
    }
    throw new TypeError('Job repository id helper must be a function or {next()} object');
}

function jsonFor(input, objectName, jsonName, fallback) {
    const explicit = valueFor(input, jsonName, `${jsonName.replace(/[A-Z]/g, letter => `_${letter.toLowerCase()}`)}`);
    if (explicit !== undefined) {
        if (typeof explicit !== 'string') throw new TypeError(`${objectName}.${jsonName} must be a JSON string`);
        try {
            JSON.parse(explicit);
        } catch (error) {
            throw new TypeError(`${objectName}.${jsonName} must contain valid JSON`, {cause: error});
        }
        return explicit;
    }

    const value = input[objectName === 'Job' ? 'payload' : 'result'];
    const resolved = value === undefined ? fallback : value;
    try {
        const serialized = JSON.stringify(resolved);
        if (serialized === undefined) throw new TypeError('JSON.stringify returned undefined');
        return serialized;
    } catch (error) {
        throw new TypeError(`${objectName} ${jsonName} could not be serialized`, {cause: error});
    }
}

function optionalJson(input, objectName, camelName, snakeName) {
    const explicit = valueFor(input, camelName, snakeName);
    if (explicit === undefined) return undefined;
    if (typeof explicit === 'string') {
        try {
            JSON.parse(explicit);
        } catch (error) {
            throw new TypeError(`${objectName}.${camelName} must contain valid JSON`, {cause: error});
        }
        return explicit;
    }
    try {
        const serialized = JSON.stringify(explicit);
        if (serialized === undefined) throw new TypeError('JSON.stringify returned undefined');
        return serialized;
    } catch (error) {
        throw new TypeError(`${objectName}.${camelName} could not be serialized`, {cause: error});
    }
}

function boundedError(value) {
    if (value === undefined || value === null) return null;
    const text = value instanceof Error ? value.message : String(value);
    return text.slice(0, MAX_JOB_ERROR_LENGTH);
}

function integer(value, name, fallback, minimum) {
    const resolved = value === undefined ? fallback : value;
    if (!Number.isInteger(resolved) || resolved < minimum) {
        throw new RangeError(`${name} must be an integer >= ${minimum}`);
    }
    return resolved;
}

function idInput(first, second = {}) {
    if (typeof first === 'string') return {...second, id: first};
    return assertRecord(first, 'Job input');
}

function personaScope(personaId, fieldName = 'Job.personaId') {
    if (personaId === undefined) return {sql: '', values: []};
    if (personaId === null) return {sql: ' AND persona_id IS NULL', values: []};
    return {sql: ' AND persona_id = ?', values: [requiredText(personaId, fieldName)]};
}

function jobTypeScope(jobTypes) {
    if (jobTypes === undefined || jobTypes === null) return {sql: '', values: []};
    if (!Array.isArray(jobTypes) || !jobTypes.length) throw new TypeError('Job.jobTypes must be a non-empty array');
    const values = jobTypes.map((jobType, index) => requiredText(jobType, `Job.jobTypes[${index}]`));
    return {sql: ` AND job_type IN (${values.map(() => '?').join(', ')})`, values};
}

function payloadPath(value) {
    if (typeof value !== 'string' || !/^\$(?:\.[A-Za-z_][A-Za-z0-9_]*)+$/.test(value)) {
        throw new TypeError('Job.payloadPath must be a simple JSON path');
    }
    return value;
}

function leaseOwnerFor(input, generateId) {
    const configured = valueFor(input, 'leaseOwner', 'lease_owner') ?? input.owner;
    return requiredText(configured === undefined ? generateId('lease') : configured, 'Job.leaseOwner');
}

function leaseExpiryFor(input, leasedAt, candidate) {
    const configured = valueFor(input, 'leaseExpiresAt', 'lease_expires_at') ?? input.leaseUntil;
    if (configured !== undefined) return timestamp(configured, 'Job.leaseExpiresAt');

    const configuredDuration = input.leaseMs ?? input.leaseDurationMs;
    const duration = typeof configuredDuration === 'function' ? configuredDuration(candidate) : configuredDuration;
    if (!Number.isFinite(duration) || duration <= 0) {
        throw new RangeError('Job lease requires leaseExpiresAt or a positive leaseMs');
    }
    const leasedAtMs = Date.parse(leasedAt);
    if (!Number.isFinite(leasedAtMs)) throw new TypeError('Job lease clock value must be an ISO timestamp');
    return new Date(leasedAtMs + duration).toISOString();
}

function assertLeaseExpiryAfter(leaseExpiresAt, leasedAt) {
    const expiryMs = Date.parse(leaseExpiresAt);
    const leasedAtMs = Date.parse(leasedAt);
    if (Number.isFinite(expiryMs) && Number.isFinite(leasedAtMs) && expiryMs <= leasedAtMs) {
        throw new RangeError('Job lease must expire after its claim time');
    }
}

/**
 * Create a table-scoped adapter for durable companion jobs/effects.
 *
 * The database, ID source, clock, lease policy, retry policy, and provider
 * execution all belong to the caller. This module only validates storage
 * inputs and performs parameterized SQL against companion_jobs.
 */
export function createJobRepository({database, id, idGenerator, clock} = {}) {
    const openDatabase = assertOpenDatabase(database);
    const now = makeClock(clock);
    const generateId = makeIdGenerator(id, idGenerator);

    function find(first, second = {}) {
        const input = idInput(first, second);
        const idValue = requiredText(input.id, 'Job.id');
        const scope = personaScope(valueFor(input, 'personaId', 'persona_id'));
        return openDatabase.prepare(`
            SELECT * FROM companion_jobs
            WHERE id = ?${scope.sql}
        `).get(idValue, ...scope.values);
    }

    function findByPayload(input = {}) {
        assertRecord(input, 'Job payload lookup');
        const personaId = valueFor(input, 'personaId', 'persona_id');
        const jobType = requiredText(valueFor(input, 'jobType', 'job_type'), 'Job.jobType');
        const path = payloadPath(input.path ?? input.payloadPath);
        if (input.value === undefined) throw new TypeError('Job.payloadValue is required');
        const scope = personaScope(personaId);
        return openDatabase.prepare(`
            SELECT * FROM companion_jobs
            WHERE job_type = ?
              AND json_extract(payload_json, ?) = ?${scope.sql}
            ORDER BY created_at ASC, id ASC
            LIMIT 1
        `).get(jobType, path, input.value, ...scope.values);
    }

    function findLeased(input = {}) {
        assertRecord(input, 'Job lease lookup');
        const jobId = requiredText(input.id, 'Job.id');
        const owner = requiredText(valueFor(input, 'leaseOwner', 'lease_owner') ?? input.owner, 'Job.leaseOwner');
        const checkedAt = timestamp(input.now ?? now(), 'Job.checkedAt');
        const persona = personaScope(valueFor(input, 'personaId', 'persona_id'));
        const types = jobTypeScope(input.jobTypes ?? input.types);
        return openDatabase.prepare(`
            SELECT * FROM companion_jobs
            WHERE id = ? AND status = 'leased' AND lease_owner = ?
              AND lease_expires_at > ?${persona.sql}${types.sql}
        `).get(jobId, owner, checkedAt, ...persona.values, ...types.values);
    }

    function enqueue(input = {}) {
        assertRecord(input, 'Job input');
        const createdAt = timestamp(valueFor(input, 'createdAt', 'created_at') ?? now(), 'Job.createdAt');
        const updatedAt = timestamp(valueFor(input, 'updatedAt', 'updated_at') ?? createdAt, 'Job.updatedAt');
        const jobId = requiredText(input.id ?? generateId('job'), 'Job.id');
        const jobType = requiredText(valueFor(input, 'jobType', 'job_type'), 'Job.jobType');
        const runAfter = timestamp(valueFor(input, 'runAfter', 'run_after') ?? createdAt, 'Job.runAfter');
        const priority = integer(input.priority, 'Job.priority', 0, 0);
        const maxAttempts = integer(valueFor(input, 'maxAttempts', 'max_attempts'), 'Job.maxAttempts', 3, 1);
        const personaId = valueFor(input, 'personaId', 'persona_id') ?? null;
        const activityId = valueFor(input, 'activityId', 'activity_id') ?? null;
        const messageId = valueFor(input, 'messageId', 'message_id') ?? null;
        const traceId = valueFor(input, 'traceId', 'trace_id') ?? null;
        if (personaId !== null) requiredText(personaId, 'Job.personaId');
        if (activityId !== null) requiredText(activityId, 'Job.activityId');
        if (messageId !== null) requiredText(messageId, 'Job.messageId');
        if (traceId !== null) requiredText(traceId, 'Job.traceId');
        const payloadJson = jsonFor(input, 'Job', 'payloadJson', '{}');

        return openDatabase.transaction(() => {
            openDatabase.prepare(`
                INSERT INTO companion_jobs (
                    id, job_type, status, priority, run_after, lease_owner,
                    lease_expires_at, attempt_count, max_attempts, persona_id,
                    activity_id, message_id, trace_id, payload_json, result_json,
                    error, created_at, updated_at, completed_at
                ) VALUES (?, ?, 'queued', ?, ?, NULL, NULL, 0, ?, ?, ?, ?, ?, ?, NULL, NULL, ?, ?, NULL)
            `).run(
                jobId, jobType, priority, runAfter, maxAttempts, personaId,
                activityId, messageId, traceId, payloadJson, createdAt, updatedAt
            );
            return find({id: jobId, ...(personaId === null ? {personaId: null} : personaId ? {personaId} : {})});
        })();
    }

    function claim(input = {}) {
        assertRecord(input, 'Job claim input');
        const claimedAt = timestamp(input.now ?? now(), 'Job.claimedAt');
        const owner = leaseOwnerFor(input, generateId);
        const scope = personaScope(valueFor(input, 'personaId', 'persona_id'));

        return openDatabase.transaction(() => {
            const candidate = openDatabase.prepare(`
                SELECT * FROM companion_jobs
                WHERE ((status = 'queued' AND run_after <= ?)
                    OR (status = 'leased' AND lease_expires_at < ?))${scope.sql}
                ORDER BY run_after ASC, priority DESC, created_at ASC, id ASC
                LIMIT 1
            `).get(claimedAt, claimedAt, ...scope.values);
            if (!candidate) return null;

            const leaseExpiresAt = leaseExpiryFor(input, claimedAt, candidate);
            assertLeaseExpiryAfter(leaseExpiresAt, claimedAt);
            const changed = openDatabase.prepare(`
                UPDATE companion_jobs
                SET status = 'leased', lease_owner = ?, lease_expires_at = ?,
                    attempt_count = attempt_count + 1, updated_at = ?
                WHERE id = ?
                  AND ((status = 'queued' AND run_after <= ?)
                    OR (status = 'leased' AND lease_expires_at < ?))${scope.sql}
            `).run(owner, leaseExpiresAt, claimedAt, candidate.id, claimedAt, claimedAt, ...scope.values);
            if (!changed.changes) return null;
            return find({id: candidate.id, ...(valueFor(input, 'personaId', 'persona_id') !== undefined ? {personaId: valueFor(input, 'personaId', 'persona_id')} : {})});
        })();
    }

    function transitionInput(first, second = {}) {
        if (typeof first === 'string') return {...second, id: first};
        const source = assertRecord(first, 'Job transition input');
        const overrides = assertRecord(second, 'Job transition options');
        // A raw SQLite row is a useful lease token for callers, but its
        // persisted status/schedule/error are observations, not transition
        // commands. Keep only the guarded identity fields from that shape.
        const isRawRow = Object.hasOwn(source, 'lease_owner') || Object.hasOwn(source, 'created_at');
        const base = isRawRow
            ? {id: source.id, lease_owner: source.lease_owner, persona_id: source.persona_id}
            : source;
        return {...base, ...overrides};
    }

    function patchResult(first, second = {}) {
        const input = transitionInput(first, second);
        const jobId = requiredText(input.id, 'Job.id');
        const updatedAt = timestamp(input.now ?? now(), 'Job.updatedAt');
        const patch = input.patch ?? input.result;
        if (!patch || typeof patch !== 'object' || Array.isArray(patch)) throw new TypeError('Job.result patch must be an object');
        const persona = personaScope(valueFor(input, 'personaId', 'persona_id'));
        const types = jobTypeScope(input.jobTypes ?? input.types);
        const configuredOwner = valueFor(input, 'leaseOwner', 'lease_owner') ?? input.owner;
        const owner = configuredOwner === undefined ? null : requiredText(configuredOwner, 'Job.leaseOwner');
        const lease = owner === null ? {sql: '', values: []} : {
            sql: " AND status = 'leased' AND lease_owner = ? AND lease_expires_at > ?",
            values: [owner, updatedAt]
        };

        return openDatabase.transaction(() => {
            const active = openDatabase.prepare(`
                SELECT * FROM companion_jobs
                WHERE id = ?${persona.sql}${types.sql}${lease.sql}
            `).get(jobId, ...persona.values, ...types.values, ...lease.values);
            if (!active) return {changed: false, result: null, job: null};

            let current;
            try {
                current = active.result_json ? JSON.parse(active.result_json) : {};
            } catch {
                current = {};
            }
            const base = current && typeof current === 'object' && !Array.isArray(current) ? current : {};
            const next = {...base, ...Object.fromEntries(Object.entries(patch).filter(([, value]) => value !== undefined))};
            const resultJson = JSON.stringify(next);
            const changed = openDatabase.prepare(`
                UPDATE companion_jobs
                SET result_json = ?, updated_at = ?
                WHERE id = ?${persona.sql}${types.sql}${lease.sql}
            `).run(resultJson, updatedAt, jobId, ...persona.values, ...types.values, ...lease.values);
            const job = changed.changes ? find({id: jobId, ...(valueFor(input, 'personaId', 'persona_id') !== undefined ? {personaId: valueFor(input, 'personaId', 'persona_id')} : {})}) : null;
            return {changed: Boolean(changed.changes), result: next, job};
        })();
    }

    function completeQueued(input = {}) {
        assertRecord(input, 'Job queued completion');
        const personaId = requiredText(valueFor(input, 'personaId', 'persona_id'), 'Job.personaId');
        const jobType = requiredText(valueFor(input, 'jobType', 'job_type'), 'Job.jobType');
        const excludedId = valueFor(input, 'excludeId', 'exclude_id');
        if (excludedId !== undefined && excludedId !== null) requiredText(excludedId, 'Job.excludeId');
        const completedAt = timestamp(input.now ?? now(), 'Job.completedAt');
        const resultJson = optionalJson(input, 'Job', 'result', 'result_json') ?? '{}';
        const exclusion = excludedId === undefined || excludedId === null ? {sql: '', values: []} : {sql: ' AND id != ?', values: [excludedId]};
        return openDatabase.prepare(`
            UPDATE companion_jobs
            SET status = 'complete', result_json = ?, error = NULL, updated_at = ?, completed_at = ?
            WHERE persona_id = ? AND job_type = ? AND status = 'queued'${exclusion.sql}
        `).run(resultJson, completedAt, completedAt, personaId, jobType, ...exclusion.values);
    }

    function transitionJob(input, operation) {
        const jobId = requiredText(input.id, 'Job.id');
        const owner = requiredText(valueFor(input, 'leaseOwner', 'lease_owner') ?? input.owner, 'Job.leaseOwner');
        const settledAt = timestamp(input.now ?? now(), 'Job.settledAt');
        const personaId = valueFor(input, 'personaId', 'persona_id');
        const scope = personaScope(personaId);
        const resultJson = optionalJson(input, 'Job', 'result', 'result_json');
        const error = boundedError(input.error);

        return openDatabase.transaction(() => {
            const active = openDatabase.prepare(`
                SELECT * FROM companion_jobs
                WHERE id = ? AND status = 'leased' AND lease_owner = ?
                  AND lease_expires_at > ?${scope.sql}
            `).get(jobId, owner, settledAt, ...scope.values);
            if (!active) return {changed: false, status: null, job: null};

            const nextResultJson = resultJson === undefined ? active.result_json : resultJson;
            if (operation === 'retry') {
                const runAfter = timestamp(valueFor(input, 'runAfter', 'run_after') ?? input.retryAt ?? input.retry_at, 'Job.runAfter');
                const changed = openDatabase.prepare(`
                    UPDATE companion_jobs
                    SET status = 'queued', lease_owner = NULL, lease_expires_at = NULL,
                        run_after = ?, result_json = ?, error = ?, updated_at = ?, completed_at = NULL
                    WHERE id = ? AND status = 'leased' AND lease_owner = ?
                      AND lease_expires_at > ?${scope.sql}
                `).run(runAfter, nextResultJson, error, settledAt, jobId, owner, settledAt, ...scope.values);
                const job = changed.changes ? find({id: jobId, ...(personaId !== undefined ? {personaId} : {})}) : null;
                return {changed: Boolean(changed.changes), status: changed.changes ? 'queued' : null, job};
            }

            const status = input.status ?? (input.terminal === true && error ? 'failed' : 'complete');
            if (status !== 'complete' && status !== 'failed') {
                throw new RangeError('Job settlement status must be complete or failed');
            }
            const runAfter = timestamp(valueFor(input, 'runAfter', 'run_after') ?? settledAt, 'Job.runAfter');
            const completedAt = status === 'complete' ? settledAt : null;
            const changed = openDatabase.prepare(`
                UPDATE companion_jobs
                SET status = ?, lease_owner = NULL, lease_expires_at = NULL,
                    run_after = ?, result_json = ?, error = ?, updated_at = ?, completed_at = ?
                WHERE id = ? AND status = 'leased' AND lease_owner = ?
                  AND lease_expires_at > ?${scope.sql}
            `).run(status, runAfter, nextResultJson, error, settledAt, completedAt, jobId, owner, settledAt, ...scope.values);
            const job = changed.changes ? find({id: jobId, ...(personaId !== undefined ? {personaId} : {})}) : null;
            return {changed: Boolean(changed.changes), status: changed.changes ? status : null, job};
        })();
    }

    function settle(first, second = {}) {
        return transitionJob(transitionInput(first, second), 'settle');
    }

    function retry(first, second = {}) {
        return transitionJob(transitionInput(first, second), 'retry');
    }

    return Object.freeze({
        enqueue,
        find,
        findById: find,
        findByPayload,
        findLeased,
        patchResult,
        completeQueued,
        claim,
        settle,
        retry
    });
}

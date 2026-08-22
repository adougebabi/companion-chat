import {randomUUID} from 'node:crypto';

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function requiredText(value, field) {
    if (typeof value !== 'string' || value.trim() === '') throw new TypeError(`${field} must be a non-empty string`);
    return value.trim();
}

function clockFor(clock) {
    if (typeof clock === 'function') return () => new Date(clock()).toISOString();
    if (isRecord(clock) && typeof clock.now === 'function') return () => new Date(clock.now()).toISOString();
    return () => new Date().toISOString();
}

function idFor(id) {
    if (typeof id === 'function') return id;
    if (isRecord(id) && typeof id.next === 'function') return id.next.bind(id);
    return prefix => `${prefix}_${randomUUID()}`;
}

function valueFor(value, camel, snake) {
    return value?.[camel] === undefined ? value?.[snake] : value[camel];
}

function parseJson(value, fallback) {
    if (isRecord(value) || Array.isArray(value)) return value;
    if (typeof value !== 'string' || !value.trim()) return fallback;
    try {
        const parsed = JSON.parse(value);
        return parsed === null ? fallback : parsed;
    } catch {
        return fallback;
    }
}

function mediaKind(file, fallback = 'image') {
    return file?.format === 'video' || /(?:video|webm|mp4)/i.test(String(file?.filename || '')) ? 'video' : fallback;
}

function filesFor(value) {
    return Array.isArray(value) ? value.slice(0, 3).filter(file => isRecord(file)) : [];
}

/**
 * SQLite adapter for media worker projections and poll/retry outbox writes.
 * The media application service owns policy; this adapter only performs
 * guarded, persona-scoped persistence against the existing tables.
 */
export function createMediaJobRepository({database, jobRepository, activityRepository, conversationRepository, clock, id} = {}) {
    if (!database || typeof database.prepare !== 'function' || typeof database.transaction !== 'function') {
        throw new TypeError('Media job repository requires an open database');
    }
    if (!jobRepository || typeof jobRepository.enqueue !== 'function') throw new TypeError('Media job repository requires jobRepository.enqueue()');
    const now = clockFor(clock);
    const generateId = idFor(id);

    function rowValue(job, camel, snake) {
        return valueFor(job, camel, snake);
    }

    function scopedMessage(job) {
        const messageId = rowValue(job, 'messageId', 'message_id');
        const personaId = rowValue(job, 'personaId', 'persona_id');
        if (!messageId || !personaId) return null;
        return database.prepare(`
            SELECT messages.*
            FROM companion_messages messages
            JOIN companion_conversations conversations ON conversations.id = messages.conversation_id
            WHERE messages.id = ? AND conversations.persona_id = ?
        `).get(messageId, personaId);
    }

    function findByIdempotency({jobType, personaId, idempotencyKey} = {}) {
        if (!idempotencyKey || typeof jobRepository.findByPayload !== 'function') return null;
        return jobRepository.findByPayload({
            jobType,
            personaId,
            path: '$.idempotencyKey',
            value: idempotencyKey
        }) ?? null;
    }

    function deterministicPollKey({sourceJobId, externalId} = {}) {
        return `media:poll:${requiredText(sourceJobId, 'Media poll sourceJobId')}:${requiredText(externalId, 'Media poll externalId')}`;
    }

    function deterministicQualityRetryKey({sourceJobId, retryCount} = {}) {
        return `media:quality-retry:${requiredText(sourceJobId, 'Media quality retry sourceJobId')}:${Number(retryCount)}`;
    }

    function updateTarget({job, status, provider, externalId, promptId, attachments, error, now: at} = {}) {
        const jobPersonaId = requiredText(rowValue(job, 'personaId', 'persona_id'), 'Media job personaId');
        const activityId = rowValue(job, 'activityId', 'activity_id');
        const updatedAt = requiredText(at ?? now(), 'Media target.updatedAt');
        if (activityId) {
            const changed = database.prepare(`
                UPDATE companion_activities
                SET media_status = ?
                WHERE id = ? AND persona_id = ?
            `).run(status ?? 'none', activityId, jobPersonaId);
            return changed.changes
                ? {changed: true}
                : {changed: false, reason: 'activity_not_found'};
        }
        const message = scopedMessage(job);
        if (!message) return {changed: false, reason: 'message_not_found'};
        const generation = {
            ...parseJson(message.generation_json, {}),
            ...(status ? {status} : {}),
            ...(provider ? {provider} : {}),
            ...(externalId ? {externalId, promptId: promptId ?? externalId} : {}),
            ...(error ? {error: String(error).slice(0, 240)} : {})
        };
        const attachmentsJson = attachments === undefined ? message.attachments_json : JSON.stringify(attachments);
        const result = database.prepare(`
            UPDATE companion_messages
            SET generation_json = ?, attachments_json = ?
            WHERE id = ? AND conversation_id = (SELECT id FROM companion_conversations WHERE persona_id = ?)
        `).run(JSON.stringify(generation), attachmentsJson, message.id, jobPersonaId);
        return {changed: Boolean(result.changes), updatedAt};
    }

    function targetStatus({job} = {}) {
        const jobPersonaId = requiredText(rowValue(job, 'personaId', 'persona_id'), 'Media job personaId');
        const activityId = rowValue(job, 'activityId', 'activity_id');
        if (activityId) {
            return database.prepare(`
                SELECT media_status AS status
                FROM companion_activities
                WHERE id = ? AND persona_id = ?
            `).get(activityId, jobPersonaId)?.status ?? null;
        }
        const message = scopedMessage(job);
        if (!message) return null;
        return parseJson(message.generation_json, {})?.status ?? null;
    }

    function persistAssets({job, provider, files, now: at} = {}) {
        const providerId = requiredText(provider?.id ?? provider, 'Media asset provider');
        const mediaFiles = filesFor(files);
        const personaId = requiredText(rowValue(job, 'personaId', 'persona_id'), 'Media job personaId');
        const activityId = rowValue(job, 'activityId', 'activity_id');
        const createdAt = requiredText(at ?? now(), 'Media asset.createdAt');
        return database.transaction(() => {
            const assets = [];
            for (const [position, file] of mediaFiles.entries()) {
                const filename = requiredText(file.filename ?? file.path, 'Media asset.filename');
                const subfolder = String(file.subfolder ?? '').slice(0, 512);
                const fileType = String(file.type ?? 'output').slice(0, 32);
                const existing = database.prepare(`
                    SELECT id, media_kind FROM companion_media_assets
                    WHERE provider = ? AND filename = ? AND subfolder = ? AND file_type = ?
                `).get(providerId, filename, subfolder, fileType);
                const assetId = existing?.id ?? generateId('asset');
                const kind = existing?.media_kind ?? mediaKind(file, rowValue(job, 'jobType', 'job_type')?.includes('video') ? 'video' : 'image');
                if (!existing) {
                    database.prepare(`
                        INSERT INTO companion_media_assets
                            (id, provider, media_kind, filename, subfolder, file_type, locator_json, created_at)
                        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
                    `).run(assetId, providerId, kind, filename, subfolder, fileType, JSON.stringify(file), createdAt);
                }
                if (activityId && activityRepository?.insertActivityMedia) {
                    activityRepository.insertActivityMedia({activityId, personaId, mediaId: assetId, position});
                }
                assets.push({id: assetId, kind, url: `/api/companion/media/${assetId}`});
            }
            return assets;
        })();
    }

    function enqueuePoll({job, payload, now: at} = {}) {
        const jobType = String(rowValue(job, 'jobType', 'job_type') || 'chat_image').startsWith('activity_')
            ? 'activity_media_poll'
            : 'chat_media_poll';
        const sourceJobId = payload?.sourceJobId ?? payload?.source_job_id ?? rowValue(job, 'id', 'id');
        const externalId = payload?.externalId ?? payload?.external_id ?? payload?.promptId ?? payload?.prompt_id;
        const idempotencyKey = payload?.idempotencyKey ?? deterministicPollKey({sourceJobId, externalId});
        const jobPayload = {...payload, idempotencyKey};
        const existing = findByIdempotency({
            jobType,
            personaId: rowValue(job, 'personaId', 'persona_id') ?? null,
            idempotencyKey
        });
        if (existing) return {job: existing, created: false, replayed: true};
        const input = {
            jobType,
            personaId: rowValue(job, 'personaId', 'persona_id') ?? null,
            activityId: rowValue(job, 'activityId', 'activity_id') ?? null,
            messageId: rowValue(job, 'messageId', 'message_id') ?? null,
            priority: 4,
            maxAttempts: 60,
            runAfter: at ?? now(),
            payload: jobPayload,
            now: at ?? now()
        };
        try {
            return {job: jobRepository.enqueue(input), created: true, replayed: false};
        } catch (error) {
            // A concurrent/replayed enqueue may win between the lookup and
            // INSERT. Re-read the deterministic key before surfacing failure.
            const raced = findByIdempotency({
                jobType,
                personaId: rowValue(job, 'personaId', 'persona_id') ?? null,
                idempotencyKey
            });
            if (raced) return {job: raced, created: false, replayed: true};
            throw error;
        }
    }

    function enqueueQualityRetry({job, payload, now: at} = {}) {
        const sourceJobId = rowValue(job, 'id', 'id');
        const retryCount = Number(payload?.qualityRetryCount ?? payload?.quality_retry_count);
        const idempotencyKey = payload?.idempotencyKey
            ?? payload?.qualityRetryKey
            ?? deterministicQualityRetryKey({sourceJobId, retryCount});
        const jobPayload = {...payload, idempotencyKey, qualityRetryKey: idempotencyKey};
        const jobType = rowValue(job, 'jobType', 'job_type');
        const existing = findByIdempotency({
            jobType,
            personaId: rowValue(job, 'personaId', 'persona_id') ?? null,
            idempotencyKey
        });
        if (existing) return {job: existing, created: false, replayed: true};
        const input = {
            jobType,
            personaId: rowValue(job, 'personaId', 'persona_id') ?? null,
            activityId: rowValue(job, 'activityId', 'activity_id') ?? null,
            messageId: rowValue(job, 'messageId', 'message_id') ?? null,
            priority: Number(rowValue(job, 'priority', 'priority') || 4),
            maxAttempts: Number(rowValue(job, 'maxAttempts', 'max_attempts') || 3),
            payload: jobPayload,
            runAfter: at ?? now(),
            now: at ?? now()
        };
        try {
            return {job: jobRepository.enqueue(input), created: true, replayed: false};
        } catch (error) {
            const raced = findByIdempotency({
                jobType,
                personaId: rowValue(job, 'personaId', 'persona_id') ?? null,
                idempotencyKey
            });
            if (raced) return {job: raced, created: false, replayed: true};
            throw error;
        }
    }

    function findPoll({job, sourceJobId, externalId} = {}) {
        const sourceType = String(rowValue(job, 'jobType', 'job_type') || 'chat_image');
        const jobType = sourceType.startsWith('activity_') ? 'activity_media_poll' : 'chat_media_poll';
        const idempotencyKey = deterministicPollKey({sourceJobId, externalId});
        return findByIdempotency({
            jobType,
            personaId: rowValue(job, 'personaId', 'persona_id') ?? null,
            idempotencyKey
        });
    }

    function enqueuePollCompensation({job, payload, now: at} = {}) {
        const sourceJobId = payload?.sourceJobId ?? payload?.source_job_id;
        const externalId = payload?.externalId ?? payload?.external_id ?? payload?.promptId ?? payload?.prompt_id;
        const idempotencyKey = payload?.idempotencyKey
            ?? `media:poll-compensation:${requiredText(sourceJobId, 'Media compensation sourceJobId')}:${requiredText(externalId, 'Media compensation externalId')}`;
        const sourceJobType = rowValue(job, 'jobType', 'job_type');
        const activityId = rowValue(job, 'activityId', 'activity_id') ?? null;
        const messageId = rowValue(job, 'messageId', 'message_id') ?? null;
        const jobPayload = {...payload, sourceJobType, idempotencyKey};
        const existing = findByIdempotency({
            jobType: 'media_poll_compensation',
            personaId: rowValue(job, 'personaId', 'persona_id') ?? null,
            idempotencyKey
        });
        if (existing) return {job: existing, created: false, replayed: true};
        const input = {
            id: payload?.compensationJobId,
            jobType: 'media_poll_compensation',
            personaId: rowValue(job, 'personaId', 'persona_id') ?? null,
            activityId,
            messageId,
            priority: 5,
            maxAttempts: 60,
            runAfter: at ?? now(),
            payload: jobPayload,
            now: at ?? now()
        };
        delete input.id;
        try {
            return {job: jobRepository.enqueue(input), created: true, replayed: false};
        } catch (error) {
            const raced = findByIdempotency({
                jobType: 'media_poll_compensation',
                personaId: rowValue(job, 'personaId', 'persona_id') ?? null,
                idempotencyKey
            });
            if (raced) return {job: raced, created: false, replayed: true};
            throw error;
        }
    }

    return Object.freeze({
        recordResult({job, patch, leaseOwner, personaId, now: at} = {}) {
            return jobRepository.patchResult(job, {patch, leaseOwner, personaId, now: at ?? now()});
        },
        patchResult({job, patch, leaseOwner, personaId, now: at} = {}) {
            return jobRepository.patchResult(job, {patch, leaseOwner, personaId, now: at ?? now()});
        },
        updateTarget,
        settleTarget: updateTarget,
        targetStatus,
        persistAssets,
        saveAssets: persistAssets,
        enqueuePoll,
        schedulePoll: enqueuePoll,
        enqueueQualityRetry,
        scheduleQualityRetry: enqueueQualityRetry,
        findPoll,
        enqueuePollCompensation,
        findMessage({job} = {}) { return scopedMessage(job); },
        conversationRepository,
        activityRepository
    });
}

export default createMediaJobRepository;

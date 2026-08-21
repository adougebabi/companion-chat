import {randomUUID} from 'node:crypto';

/**
 * The chat media flow owns only the durable intent handoff.  Capability
 * decoding/normalization and media semantics are supplied by the caller; this
 * module allocates message/job identities and commits their linked rows.
 */
export const MEDIA_FLOW_VERSION = 1;
export const MEDIA_JOB_TYPES = Object.freeze({image: 'chat_image', video: 'chat_video'});

const PRIVATE_PLAN = new WeakMap();

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function requiredText(value, field, maxLength = 240) {
    if (typeof value !== 'string' || value.trim() === '') throw new TypeError(`${field} must be a non-empty string`);
    const text = value.trim();
    if (text.length > maxLength) throw new RangeError(`${field} exceeds ${maxLength} characters`);
    return text;
}

function boundedText(value, field, maxLength, {allowEmpty = false} = {}) {
    if (typeof value !== 'string') throw new TypeError(`${field} must be a string`);
    const text = value.trim();
    if (!allowEmpty && !text) throw new TypeError(`${field} must not be empty`);
    if (text.length > maxLength) throw new RangeError(`${field} exceeds ${maxLength} characters`);
    return text;
}

function timestamp(value, field) {
    const normalized = value instanceof Date ? value.toISOString() : value;
    if (typeof normalized !== 'string' || !normalized.trim() || !Number.isFinite(Date.parse(normalized))) {
        throw new TypeError(`${field} must be a valid timestamp`);
    }
    return normalized;
}

function clockFunction(clock) {
    if (clock === undefined) return () => new Date().toISOString();
    if (typeof clock === 'function') return () => timestamp(clock(), 'Media-flow clock value');
    if (isRecord(clock) && typeof clock.now === 'function') return () => timestamp(clock.now(), 'Media-flow clock value');
    throw new TypeError('Media-flow clock must be a function or provide now()');
}

function idFunction(idGenerator) {
    if (idGenerator === undefined) return prefix => `${prefix}_${randomUUID()}`;
    if (typeof idGenerator === 'function') return prefix => requiredText(idGenerator(prefix), 'Generated media-flow id');
    if (isRecord(idGenerator) && typeof idGenerator.next === 'function') {
        return prefix => requiredText(idGenerator.next(prefix), 'Generated media-flow id');
    }
    throw new TypeError('Media-flow idGenerator must be a function or provide next()');
}

function deepFreeze(value) {
    if (!value || typeof value !== 'object' || Object.isFrozen(value)) return value;
    Object.freeze(value);
    for (const child of Object.values(value)) deepFreeze(child);
    return value;
}

function resolveRepository(repositories, names, field, {optional = false} = {}) {
    const source = isRecord(repositories) ? repositories : {};
    for (const name of names) {
        if (source[name] !== undefined) {
            if (!isRecord(source[name]) && typeof source[name] !== 'function') {
                throw new TypeError(`Media-flow ${field} must be an object`);
            }
            return source[name];
        }
    }
    if (optional) return null;
    throw new TypeError(`Media-flow requires ${field}`);
}

function methodFor(repository, names, field, {optional = false} = {}) {
    if (typeof repository === 'function') return repository;
    if (isRecord(repository)) {
        for (const name of names) {
            if (repository[name] !== undefined) {
                if (typeof repository[name] !== 'function') throw new TypeError(`Media-flow ${field}.${name} must be a function`);
                return repository[name].bind(repository);
            }
        }
    }
    if (optional) return null;
    throw new TypeError(`Media-flow ${field} must provide ${names.join('() or ')}()`);
}

function syncResult(value, field) {
    if (value && typeof value.then === 'function') throw new TypeError(`Media-flow ${field} must be synchronous`);
    return value;
}

function parseJson(value, fallback) {
    if (value === undefined || value === null || value === '') return fallback;
    if (typeof value === 'object') return value;
    if (typeof value !== 'string') return fallback;
    try {
        const parsed = JSON.parse(value);
        return parsed === null ? fallback : parsed;
    } catch {
        return fallback;
    }
}

function defaultMessageShape(row) {
    if (!row) return null;
    const generationValue = row.generation ?? row.generationJson ?? row.generation_json;
    return {
        id: row.id,
        role: row.role,
        text: row.text,
        attachments: row.attachments ?? parseJson(row.attachmentsJson ?? row.attachments_json, []),
        generation: generationValue === undefined || generationValue === null
            ? undefined
            : parseJson(generationValue, {}),
        jobs: row.jobs ?? parseJson(row.jobsJson ?? row.jobs_json, []),
        proactiveEventId: row.proactiveEventId ?? row.proactive_event_id ?? undefined,
        proactivePendingEventId: row.proactivePendingEventId ?? row.proactive_pending_event_id ?? undefined,
        createdAt: row.createdAt ?? row.created_at,
        readAt: row.readAt ?? row.read_at ?? undefined
    };
}

function defaultMediaMessagePlaceholder({messageId, jobId, kind, provider, request, createdAt}) {
    const generation = {status: 'queued', kind, provider, ...(request ? {request} : {})};
    return {
        id: messageId,
        role: 'assistant',
        text: '',
        attachments: [],
        generation,
        jobs: [{id: jobId, kind, provider}],
        createdAt,
        readAt: createdAt
    };
}

function jobIdOf(job) {
    return job?.id ?? job?.jobId ?? job?.job_id ?? null;
}

function messageIdOf(message) {
    return message?.id ?? message?.messageId ?? message?.message_id ?? null;
}

function jobMessageId(job) {
    return job?.messageId ?? job?.message_id ?? null;
}

function jobTypeOf(job) {
    return job?.jobType ?? job?.job_type ?? null;
}

function messageJobs(message) {
    if (!message) return [];
    const jobs = message.jobs ?? parseJson(message.jobsJson ?? message.jobs_json, []);
    return Array.isArray(jobs) ? jobs : [];
}

function jobForResult(job, {kind, provider, jobType, messageId}) {
    const id = jobIdOf(job);
    if (!id) return null;
    return {
        id,
        jobType: jobTypeOf(job) || jobType,
        kind: job?.kind ?? kind,
        provider: job?.provider ?? provider,
        ...(messageId || jobMessageId(job) ? {messageId: messageId || jobMessageId(job)} : {}),
        ...(job?.status ? {status: job.status} : {})
    };
}

function provenanceFor(command, call) {
    const source = command.provenance ?? command;
    if (!isRecord(source)) throw new TypeError('Media-flow provenance must be an object');
    const sourceName = source.source === undefined ? 'media_event' : boundedText(source.source, 'Media-flow provenance source', 80);
    const callIdValue = source.callId ?? source.call_id ?? command.capabilityCallId ?? command.capabilityCall?.id;
    const idempotencyValue = source.idempotencyKey
        ?? source.idempotency_key
        ?? command.capabilityCall?.idempotencyKey
        ?? call?.idempotencyKey;
    const causationValue = source.causationUserMessageId
        ?? source.causation_user_message_id
        ?? source.causationId
        ?? source.causation_id
        ?? command.causationUserMessageId
        ?? command.causationId;
    return Object.freeze({
        source: sourceName,
        ...(callIdValue === undefined || callIdValue === null ? {} : {callId: boundedText(callIdValue, 'Media-flow provenance callId', 160)}),
        ...(idempotencyValue === undefined || idempotencyValue === null ? {} : {idempotencyKey: boundedText(idempotencyValue, 'Media-flow provenance idempotencyKey', 240)}),
        ...(causationValue === undefined || causationValue === null ? {} : {causationUserMessageId: boundedText(causationValue, 'Media-flow provenance causationUserMessageId', 160)})
    });
}

function normalizeInjectedCall(normalizer, value, reference) {
    if (typeof normalizer !== 'function') throw new TypeError('Media-flow requires normalizeMediaCapabilityCall()');
    const normalized = syncResult(normalizer(value, reference), 'normalizeMediaCapabilityCall');
    if (!isRecord(normalized)) throw new TypeError('Media capability call must be an object');
    if (!['image', 'video'].includes(normalized.kind)) throw new TypeError('Media capability kind must be image or video');
    const count = normalized.count === undefined ? 1 : normalized.count;
    if (!Number.isInteger(count) || count < 1 || count > 3) {
        throw new RangeError('Media capability count must be an integer from 1 to 3');
    }
    if (normalized.request !== undefined && typeof normalized.request !== 'string') {
        throw new TypeError('Media capability request must be a string');
    }
    return {...normalized, count};
}

function commandFor(input, value, provenance) {
    if (typeof input === 'string') return {personaId: input, call: value, provenance};
    return input;
}

function normalizeCommand(input, value, provenance, normalizer, reference) {
    const source = commandFor(input, value, provenance);
    if (!isRecord(source)) throw new TypeError('Media-flow command must be an object');
    const personaId = requiredText(source.personaId ?? source.persona_id, 'Media-flow personaId', 160);
    const rawCall = source.call
        ?? source.mediaCall
        ?? source.mediaCapabilityCall
        ?? source.capabilityCall?.arguments
        ?? source.arguments
        ?? source;
    const call = normalizeInjectedCall(normalizer, rawCall, reference);
    const provenanceValue = provenanceFor({...source, ...(source.provenance || {})}, call);
    const triggerValue = call.trigger ?? source.trigger ?? source.provenance?.trigger ?? 'explicit_user_request';
    const trigger = boundedText(triggerValue, 'Media-flow trigger', 80);
    return {source, personaId, call, provenance: provenanceValue, trigger};
}

function personaFor(personaLookup, personaId, supplied) {
    if (!personaLookup) {
        if (!isRecord(supplied)) throw new TypeError('Media-flow requires persona repository or command.persona');
        return supplied;
    }
    const persona = syncResult(personaLookup(personaId), 'persona lookup');
    if (!persona) throw new Error('Media-flow persona does not exist');
    return persona;
}

function idempotencyKeyFor(provenance, position) {
    if (!provenance.idempotencyKey) return null;
    return `${provenance.idempotencyKey}:${position}`;
}

function jobLookup(jobFindByPayload, personaId, jobType, assetKey) {
    if (!jobFindByPayload || !assetKey) return null;
    return syncResult(jobFindByPayload({
        personaId,
        jobType,
        path: '$.capabilityCall.idempotencyKey',
        value: assetKey
    }), 'job repository read');
}

function messageLookup(messageFind, personaId, job, fallbackMessageLookup) {
    const id = jobMessageId(job);
    if (id && messageFind) return syncResult(messageFind({id, messageId: id, personaId}), 'conversation repository read');
    if (fallbackMessageLookup && job) {
        return syncResult(fallbackMessageLookup({personaId, job}), 'conversation repository read');
    }
    return null;
}

function findMessageForAsset(messageByIdempotency, personaId, jobType, assetKey) {
    if (!messageByIdempotency || !assetKey) return null;
    return syncResult(messageByIdempotency({personaId, jobType, assetKey, idempotencyKey: assetKey}), 'conversation repository read');
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
    throw new TypeError('Media-flow transaction must be a function or provide transaction()/run()');
}

function assertPlan(plan) {
    if (!isRecord(plan) || !PRIVATE_PLAN.has(plan)) throw new TypeError('Media-flow plan is invalid');
    return PRIVATE_PLAN.get(plan);
}

function normalizeProvider(value) {
    const provider = typeof value === 'string' ? value.trim() : value?.id;
    return requiredText(provider || 'comfyui', 'Media-flow provider', 80);
}

function appendRows(conversationRepository, conversationId, rows, updatedAt) {
    if (!rows.length) return [];
    if (typeof conversationRepository.appendMessages === 'function') {
        return syncResult(conversationRepository.appendMessages({conversationId, messages: rows, updatedAt}), 'conversation repository write');
    }
    if (typeof conversationRepository.appendMessage !== 'function') {
        throw new TypeError('Media-flow conversation repository must provide appendMessages() or appendMessage()');
    }
    const inserted = rows.map(row => syncResult(conversationRepository.appendMessage(row), 'conversation repository write'));
    if (typeof conversationRepository.updateConversationTimestamp === 'function') {
        syncResult(conversationRepository.updateConversationTimestamp({conversationId, updatedAt}), 'conversation repository write');
    }
    return inserted;
}

/**
 * Compose the chat media plan/apply slice from pure application ports.
 *
 * `plan()` performs persona, capability, idempotency, and envelope reads only.
 * `apply()` is caller-transactional: it appends placeholders and enqueues the
 * linked durable jobs as one unit, repairing either side of a partial replay.
 */
export function createMediaFlow({
    repositories,
    clock,
    idGenerator,
    id,
    normalizeCall,
    normalizeMediaCapabilityCall,
    mediaConceptEnvelopeFor,
    mediaMessagePlaceholder = defaultMediaMessagePlaceholder,
    messageShape = defaultMessageShape,
    providerFor,
    transaction
} = {}) {
    if (!isRecord(repositories)) throw new TypeError('Media-flow repositories must be an object');
    if (typeof mediaMessagePlaceholder !== 'function') throw new TypeError('Media-flow mediaMessagePlaceholder must be a function');
    if (typeof messageShape !== 'function') throw new TypeError('Media-flow messageShape must be a function');
    if (mediaConceptEnvelopeFor !== undefined && typeof mediaConceptEnvelopeFor !== 'function') {
        throw new TypeError('Media-flow mediaConceptEnvelopeFor must be a function');
    }
    if (providerFor !== undefined && typeof providerFor !== 'function') throw new TypeError('Media-flow providerFor must be a function');

    const personaRepository = resolveRepository(repositories, ['personaRepository', 'personas', 'persona'], 'persona repository', {optional: true});
    const conversationRepository = resolveRepository(repositories, ['conversationRepository', 'conversation', 'messages'], 'conversation repository');
    const jobRepository = resolveRepository(repositories, ['jobRepository', 'job', 'effectRepository'], 'job repository');
    const personaLookup = personaRepository ? methodFor(personaRepository, ['findActive', 'findById', 'requirePersona', 'get'], 'persona repository', {optional: true}) : null;
    const jobFindByPayload = methodFor(jobRepository, ['findByPayload', 'findByIdempotencyKey', 'findByCapabilityIdempotencyKey'], 'job repository', {optional: true});
    const jobEnqueue = methodFor(jobRepository, ['enqueue', 'create', 'insert'], 'job repository');
    const messageFind = methodFor(conversationRepository, ['findMessage', 'findById', 'getMessage', 'getById'], 'conversation repository', {optional: true});
    const messageByIdempotency = methodFor(conversationRepository, ['findMessageByIdempotencyKey', 'findByCapabilityIdempotencyKey', 'findByJobIdempotencyKey'], 'conversation repository', {optional: true});
    const messageByJob = methodFor(conversationRepository, ['findMessageByJob', 'findByJob'], 'conversation repository', {optional: true});
    const now = clockFunction(clock);
    const generateId = idFunction(idGenerator ?? id);
    const normalizer = normalizeCall ?? normalizeMediaCapabilityCall;
    if (typeof normalizer !== 'function') throw new TypeError('Media-flow requires normalizeMediaCapabilityCall()');
    // A same-process replay can repair a missing job even when the existing
    // conversation adapter only exposes findMessage(id). The durable job's
    // capability key remains the authoritative replay key; this map only
    // remembers the placeholder identity allocated by an earlier plan.
    const plannedMessageIds = new Map();

    function resolveProvider(kind, command, call) {
        if (providerFor) {
            const resolved = syncResult(providerFor(kind, command, call), 'provider lookup');
            return normalizeProvider(resolved);
        }
        return normalizeProvider(command.provider ?? command.providerId ?? call.provider);
    }

    function buildEnvelope(persona, normalized) {
        if (mediaConceptEnvelopeFor) {
            const envelope = syncResult(mediaConceptEnvelopeFor(persona, {
                kind: normalized.call.kind,
                request: normalized.call.request || '',
                count: 1,
                trigger: normalized.trigger
            }), 'mediaConceptEnvelopeFor');
            if (!isRecord(envelope)) throw new TypeError('Media concept envelope must be an object');
            return envelope;
        }
        const supplied = normalized.source.envelope ?? normalized.source.mediaConceptEnvelope;
        if (!isRecord(supplied)) throw new TypeError('Media-flow requires mediaConceptEnvelopeFor() or command.envelope');
        return supplied;
    }

    function plan(input, value, provenance = {}) {
        const current = now();
        const normalized = normalizeCommand(input, value, provenance, normalizer, Date.parse(current));
        const persona = personaFor(personaLookup, normalized.personaId, normalized.source.persona);
        const kind = normalized.call.kind;
        const count = normalized.call.count;
        const provider = resolveProvider(kind, normalized.source, normalized.call);
        const jobType = MEDIA_JOB_TYPES[kind];
        const envelope = buildEnvelope(persona, normalized);
        const entries = [];

        for (let position = 0; position < count; position += 1) {
            const assetKey = idempotencyKeyFor(normalized.provenance, position);
            const existingJob = jobLookup(jobFindByPayload, normalized.personaId, jobType, assetKey);
            const linkedMessage = messageLookup(messageFind, normalized.personaId, existingJob, messageByJob);
            const indexedMessage = linkedMessage || findMessageForAsset(messageByIdempotency, normalized.personaId, jobType, assetKey);
            const existingMessage = linkedMessage || indexedMessage;
            const createdAt = existingMessage?.createdAt ?? existingMessage?.created_at ?? current;
            const plannedKey = assetKey ? `${normalized.personaId}:${assetKey}` : null;
            const messageId = messageIdOf(existingMessage)
                || jobMessageId(existingJob)
                || (plannedKey ? plannedMessageIds.get(plannedKey) : null)
                || generateId('message');
            if (plannedKey) plannedMessageIds.set(plannedKey, messageId);
            const jobId = jobIdOf(existingJob) || messageJobs(existingMessage).find(job => job?.id)?.id || generateId('job');
            const replayed = Boolean(existingJob && existingMessage);
            const generation = existingMessage?.generation ?? parseJson(existingMessage?.generationJson ?? existingMessage?.generation_json, null)
                ?? {status: 'queued', kind, provider, ...(normalized.call.request ? {request: normalized.call.request} : {})};
            const placeholder = existingMessage
                ? syncResult(messageShape(existingMessage), 'messageShape')
                : syncResult(mediaMessagePlaceholder({
                    messageId, jobId, kind, provider, request: normalized.call.request || '', createdAt
                }), 'mediaMessagePlaceholder');
            const capabilityCall = {
                ...normalized.call,
                count: 1,
                trigger: normalized.trigger,
                ...(normalized.provenance.callId ? {capabilityCallId: normalized.provenance.callId} : {}),
                ...(assetKey ? {idempotencyKey: assetKey} : {}),
                source: normalized.provenance.source,
                causationId: normalized.provenance.causationUserMessageId || null
            };
            const payload = {
                envelope,
                personaMediaConcept: normalized.call.personaMediaConcept,
                capabilityCall,
                kind,
                provider,
                trigger: normalized.trigger,
                qualityRetryCount: 0,
                maxQualityRetries: 1
            };
            const job = existingJob || {
                id: jobId,
                jobType,
                priority: 4,
                runAfter: createdAt,
                maxAttempts: 3,
                personaId: normalized.personaId,
                messageId,
                payload,
                createdAt,
                updatedAt: createdAt
            };
            entries.push({
                position,
                assetKey,
                replayed,
                repairMessage: Boolean(existingJob && !existingMessage),
                repairJob: Boolean(existingMessage && !existingJob),
                messageId,
                jobId,
                createdAt,
                generation,
                placeholder,
                message: placeholder,
                job,
                payload,
                existingJob,
                existingMessage
            });
        }

        const messages = entries.map(entry => entry.message);
        const jobs = entries.map(entry => entry.job);
        const previewResult = {
            jobId: entries[0]?.jobId || null,
            jobIds: entries.map(entry => entry.jobId),
            message: messages[0] || null,
            messages,
            jobs,
            kind,
            replayed: entries.length > 0 && entries.every(entry => entry.replayed)
        };
        const planValue = deepFreeze({
            type: 'media_plan',
            version: MEDIA_FLOW_VERSION,
            personaId: normalized.personaId,
            kind,
            count,
            provider,
            jobType,
            trigger: normalized.trigger,
            call: normalized.call,
            provenance: normalized.provenance,
            baseKey: normalized.provenance.idempotencyKey || null,
            sourceMessageId: normalized.provenance.causationUserMessageId || null,
            envelope,
            assetKeys: entries.map(entry => entry.assetKey),
            entries,
            jobId: entries[0]?.jobId || null,
            jobIds: entries.map(entry => entry.jobId),
            messageIds: messages.map(message => message.id),
            preallocatedIds: entries.map(entry => ({messageId: entry.messageId, jobId: entry.jobId})),
            jobs,
            message: messages[0] || null,
            messages,
            placeholders: messages,
            replayed: previewResult.replayed,
            preview: previewResult,
            previewResult
        });
        PRIVATE_PLAN.set(planValue, {persona, normalized, entries, envelope});
        return planValue;
    }

    function readExisting(entry, planValue) {
        const existingJob = entry.assetKey
            ? jobLookup(jobFindByPayload, planValue.personaId, planValue.jobType, entry.assetKey)
            : null;
        let existingMessage = messageLookup(messageFind, planValue.personaId, existingJob, messageByJob);
        existingMessage ||= findMessageForAsset(messageByIdempotency, planValue.personaId, planValue.jobType, entry.assetKey);
        if (!existingMessage && entry.messageId && messageFind && !existingJob) {
            existingMessage = syncResult(messageFind({id: entry.messageId, messageId: entry.messageId, personaId: planValue.personaId}), 'conversation repository read');
        }
        return {existingJob, existingMessage};
    }

    function applyWithin(planValue, state) {
        personaFor(personaLookup, planValue.personaId, state.persona);
        const applied = [];
        const rowsToInsert = [];
        const pending = [];

        for (const entry of state.entries) {
            const current = readExisting(entry, planValue);
            const existingJob = current.existingJob;
            const existingMessage = current.existingMessage;
            if (existingJob && existingMessage) {
                applied.push({entry, job: existingJob, message: syncResult(messageShape(existingMessage), 'messageShape'), replayed: true});
                continue;
            }

            const messageId = messageIdOf(existingMessage) || jobMessageId(existingJob) || entry.messageId;
            const jobId = jobIdOf(existingJob) || jobIdOf(entry.job) || entry.jobId;
            const placeholder = existingMessage
                ? syncResult(messageShape(existingMessage), 'messageShape')
                : entry.message || syncResult(mediaMessagePlaceholder({
                    messageId, jobId, kind: planValue.kind, provider: planValue.provider,
                    request: state.normalized.call.request || '', createdAt: entry.createdAt
                }), 'mediaMessagePlaceholder');
            const row = existingMessage ? null : {
                id: messageId,
                conversationId: null,
                role: placeholder.role || 'assistant',
                text: typeof placeholder.text === 'string' ? placeholder.text : '',
                attachmentsJson: JSON.stringify(placeholder.attachments || []),
                generationJson: placeholder.generation === undefined ? null : JSON.stringify(placeholder.generation || {}),
                jobsJson: JSON.stringify(placeholder.jobs || [{id: jobId, kind: planValue.kind, provider: planValue.provider}]),
                proactiveEventId: placeholder.proactiveEventId ?? null,
                proactivePendingEventId: placeholder.proactivePendingEventId ?? null,
                createdAt: placeholder.createdAt || entry.createdAt,
                readAt: placeholder.readAt ?? placeholder.createdAt ?? entry.createdAt
            };
            if (row) rowsToInsert.push(row);
            pending.push({entry, existingJob, existingMessage, placeholder, messageId, jobId, row});
        }

        let conversation = null;
        if (rowsToInsert.length) {
            const createdAt = rowsToInsert[0].createdAt;
            const updatedAt = rowsToInsert.at(-1).createdAt;
            const first = state.persona || {};
            conversation = syncResult(conversationRepository.getOrCreateConversation({
                personaId: planValue.personaId,
                id: generateId('conversation'),
                createdAt,
                updatedAt
            }), 'conversation repository write');
            if (!conversation?.id) throw new Error('Media-flow conversation repository did not return a conversation');
            for (const row of rowsToInsert) row.conversationId = conversation.id;
            const inserted = appendRows(conversationRepository, conversation.id, rowsToInsert, updatedAt);
            if (!Array.isArray(inserted) || inserted.length !== rowsToInsert.length) {
                throw new Error('Media-flow conversation repository returned an incomplete message batch');
            }
            for (const [index, item] of pending.entries()) item.inserted = inserted[index];
        }

        for (const item of pending) {
            let persistedJob = item.existingJob;
            if (!persistedJob) {
                const jobInput = {
                    ...item.entry.job,
                    id: item.jobId,
                    jobType: planValue.jobType,
                    personaId: planValue.personaId,
                    messageId: item.messageId,
                    payload: item.entry.payload,
                    runAfter: item.entry.createdAt,
                    createdAt: item.entry.createdAt,
                    updatedAt: item.entry.createdAt
                };
                persistedJob = syncResult(jobEnqueue(jobInput), 'job repository write') || jobInput;
            }
            const shaped = item.inserted ? syncResult(messageShape(item.inserted), 'messageShape') : item.placeholder;
            applied.push({entry: item.entry, job: persistedJob, message: shaped, replayed: false});
        }

        applied.sort((left, right) => left.entry.position - right.entry.position);
        const messages = applied.map(item => item.message);
        const jobs = applied.map(item => jobForResult(item.job, {
            kind: planValue.kind,
            provider: planValue.provider,
            jobType: planValue.jobType,
            messageId: item.message?.id || item.entry.messageId
        }));
        return {
            type: 'done',
            jobId: jobs[0]?.id || null,
            jobIds: jobs.map(job => job?.id).filter(Boolean),
            message: messages[0] || null,
            messages,
            jobs,
            kind: planValue.kind,
            provider: planValue.provider,
            count: messages.length,
            replayed: applied.length > 0 && applied.every(item => item.replayed)
        };
    }

    function apply(planValue, options = {}) {
        const state = assertPlan(planValue);
        const settings = typeof options === 'function' ? {transaction: options} : options;
        if (!isRecord(settings)) throw new TypeError('Media-flow apply options must be an object');
        const runner = settings.transaction ?? settings.callerTransaction ?? settings.runInTransaction ?? settings.commit ?? transaction;
        return transactionRunner(runner, () => applyWithin(planValue, state));
    }

    return Object.freeze({
        version: MEDIA_FLOW_VERSION,
        plan,
        apply,
        normalizeMediaCapabilityCall: normalizer,
        repositories: Object.freeze({persona: personaRepository, conversation: conversationRepository, job: jobRepository})
    });
}

export const createCompanionMediaFlow = createMediaFlow;
export default createMediaFlow;

import {randomUUID} from 'node:crypto';
import {createFlowEffectAdapter} from './flow-effect-adapter.js';

export const PENDING_EVENT_FLOW_VERSION = 1;
export const PENDING_EVENT_SCHEMA_VERSION = 1;
export const PENDING_EVENT_MAX_SUMMARY = 280;
export const PENDING_EVENT_MAX_DEDUPE_KEY = 120;
export const PENDING_EVENT_MAX_FUTURE_MS = 30 * 24 * 60 * 60 * 1_000;

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
    if (typeof clock === 'function') return () => timestamp(clock(), 'Pending-event clock value');
    if (isRecord(clock) && typeof clock.now === 'function') return () => timestamp(clock.now(), 'Pending-event clock value');
    throw new TypeError('Pending-event flow clock must be a function or provide now()');
}

function idFunction(idGenerator) {
    if (idGenerator === undefined) return prefix => `${prefix}_${randomUUID()}`;
    if (typeof idGenerator === 'function') return prefix => requiredText(idGenerator(prefix), 'Generated pending-event id');
    if (isRecord(idGenerator) && typeof idGenerator.next === 'function') return prefix => requiredText(idGenerator.next(prefix), 'Generated pending-event id');
    throw new TypeError('Pending-event flow idGenerator must be a function or provide next()');
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
            if (!isRecord(source[name]) && typeof source[name] !== 'function') throw new TypeError(`Pending-event flow ${field} must be an object`);
            return source[name];
        }
    }
    if (optional) return null;
    throw new TypeError(`Pending-event flow requires ${field}`);
}

function methodFor(repository, names, field, {optional = false} = {}) {
    if (typeof repository === 'function') return repository;
    if (isRecord(repository)) {
        for (const name of names) {
            if (repository[name] !== undefined) {
                if (typeof repository[name] !== 'function') throw new TypeError(`Pending-event flow ${field}.${name} must be a function`);
                return repository[name].bind(repository);
            }
        }
    }
    if (optional) return null;
    throw new TypeError(`Pending-event flow ${field} must provide ${names.join('() or ')}()`);
}

function normalizeAbsoluteTime(value, label, referenceMs) {
    const source = typeof value === 'string' ? value.trim() : '';
    if (!source || !/(?:Z|[+-]\d{2}:\d{2})$/i.test(source)) throw new Error(`${label} must include an explicit timezone offset`);
    const parsed = Date.parse(source);
    if (!Number.isFinite(parsed)) throw new Error(`${label} is invalid`);
    if (parsed > referenceMs + PENDING_EVENT_MAX_FUTURE_MS) throw new Error(`${label} cannot be more than 30 days in the future`);
    return new Date(parsed).toISOString();
}

/**
 * Minimal strict fallback contract for callers that have not yet injected the
 * legacy transport normalizer. The native/marker adapter remains the owner of
 * parsing; this function only validates an already-decoded pending call.
 */
export function normalizePendingEventCall(value, reference = Date.now()) {
    if (!isRecord(value)) throw new Error('Pending-event call must be a JSON object');
    if (value.schemaVersion !== PENDING_EVENT_SCHEMA_VERSION) throw new Error('Pending-event call schema version is invalid');
    const summary = boundedText(value.summary, 'Pending-event summary', PENDING_EVENT_MAX_SUMMARY);
    const dedupeKey = boundedText(value.dedupeKey, 'Pending-event dedupeKey', PENDING_EVENT_MAX_DEDUPE_KEY);
    const referenceMs = reference instanceof Date ? reference.getTime() : Number(reference);
    if (!Number.isFinite(referenceMs)) throw new TypeError('Pending-event reference time must be finite');
    const notBefore = normalizeAbsoluteTime(value.notBefore, 'Pending-event notBefore', referenceMs);
    const expiresAt = normalizeAbsoluteTime(value.expiresAt, 'Pending-event expiresAt', referenceMs);
    if (Date.parse(notBefore) < referenceMs - 60_000) throw new Error('Pending-event notBefore is too far in the past');
    if (Date.parse(expiresAt) <= Date.parse(notBefore)) throw new Error('Pending-event expiresAt must be after notBefore');
    return Object.freeze({schemaVersion: PENDING_EVENT_SCHEMA_VERSION, summary, notBefore, expiresAt, dedupeKey});
}

function normalizeInjectedCall(normalizer, value, referenceMs) {
    const normalized = normalizer(value, referenceMs);
    if (normalized && typeof normalized.then === 'function') throw new TypeError('Pending-event normalizer must be synchronous');
    // Re-validate the injected result at this boundary so an adapter cannot
    // bypass the flow's bounded schema contract.
    return normalizePendingEventCall(normalized, referenceMs);
}

function provenanceFor(value = {}) {
    if (!isRecord(value)) throw new TypeError('Pending-event provenance must be an object');
    const source = value.source === undefined ? 'pending_event' : boundedText(value.source, 'Pending-event provenance source', 80);
    const callId = value.callId === undefined && value.call_id === undefined
        ? undefined
        : boundedText(value.callId ?? value.call_id, 'Pending-event provenance callId', 160);
    const idempotencyKey = value.idempotencyKey === undefined && value.idempotency_key === undefined
        ? undefined
        : boundedText(value.idempotencyKey ?? value.idempotency_key, 'Pending-event provenance idempotencyKey', 160);
    return Object.freeze({source, ...(callId ? {callId} : {}), ...(idempotencyKey ? {idempotencyKey} : {})});
}

function sourceRowId(row) {
    return row?.id ?? row?.messageId ?? row?.message_id;
}

function sourceRowPersonaId(row) {
    return row?.personaId ?? row?.persona_id ?? row?.ownerPersonaId ?? row?.owner_persona_id;
}

function sourceRowRole(row) {
    return row?.role ?? row?.authorRole ?? row?.author_role;
}

function normalizeSource(row, personaId, sourceMessageId) {
    if (!isRecord(row)) throw new Error('Pending-event source message does not exist');
    const rowId = sourceRowId(row);
    if (rowId !== sourceMessageId) throw new Error('Pending-event source message does not match the requested id');
    const owner = sourceRowPersonaId(row);
    if (owner !== personaId) throw new Error('Pending-event source message does not belong to persona');
    const role = sourceRowRole(row);
    if (role !== 'user') throw new Error('Pending-event source message must be user-owned');
    return Object.freeze({id: sourceMessageId, personaId, role: 'user'});
}

function sourceLookupFor(repositories) {
    const source = resolveRepository(repositories, [
        'sourceMessageRepository', 'messageRepository', 'messages', 'conversationRepository', 'conversation'
    ], 'source message repository', {optional: true});
    if (!source) return null;
    return methodFor(source, ['findById', 'findMessage', 'getMessage', 'getById'], 'source message repository', {optional: true});
}

function readSource(sourceLookup, command, personaId, sourceMessageId) {
    if (!sourceLookup) {
        if (command.sourceMessage === undefined) throw new TypeError('Pending-event flow requires a source message repository or command.sourceMessage');
        return normalizeSource(command.sourceMessage, personaId, sourceMessageId);
    }
    const source = sourceLookup.length >= 2
        ? sourceLookup(sourceMessageId, {personaId})
        : sourceLookup({id: sourceMessageId, messageId: sourceMessageId, personaId});
    if (source && typeof source.then === 'function') throw new TypeError('Pending-event source lookup must be synchronous');
    return normalizeSource(source, personaId, sourceMessageId);
}

function pendingShape(row, fallback) {
    const source = row || fallback;
    if (!source) return null;
    return {
        id: source.id,
        personaId: source.personaId ?? source.persona_id,
        sourceMessageId: source.sourceMessageId ?? source.source_message_id ?? undefined,
        status: source.status,
        summary: source.summary,
        notBefore: source.notBefore ?? source.not_before,
        expiresAt: source.expiresAt ?? source.expires_at,
        dedupeKey: source.dedupeKey ?? source.dedupe_key,
        createdAt: source.createdAt ?? source.created_at ?? undefined,
        updatedAt: source.updatedAt ?? source.updated_at ?? undefined,
        triggeredAt: source.triggeredAt ?? source.triggered_at ?? undefined,
        consumedAt: source.consumedAt ?? source.consumed_at ?? undefined,
        cancelledAt: source.cancelledAt ?? source.cancelled_at ?? undefined
    };
}

function jobIdOf(job) {
    return job?.id ?? job?.jobId ?? job?.job_id ?? null;
}

function callFromCommand(command, injectedNormalizer, referenceMs) {
    const raw = command.call ?? command.value ?? command.pendingEvent ?? command.pending_event ?? command.arguments;
    if (raw === undefined) throw new TypeError('Pending-event command call is required');
    return injectedNormalizer ? normalizeInjectedCall(injectedNormalizer, raw, referenceMs) : normalizePendingEventCall(raw, referenceMs);
}

function normalizeCommand(input, injectedNormalizer, referenceMs) {
    if (!isRecord(input)) throw new TypeError('Pending-event command must be an object');
    const personaId = requiredText(input.personaId ?? input.persona_id, 'Pending-event personaId');
    const sourceMessageId = requiredText(input.sourceMessageId ?? input.source_message_id, 'Pending-event sourceMessageId');
    const call = callFromCommand(input, injectedNormalizer, referenceMs);
    const source = provenanceFor(input.provenance ?? input);
    return {personaId, sourceMessageId, call, provenance: source, sourceMessage: input.sourceMessage};
}

function planShape({personaId, source, call, pendingEventId, jobId, existing, existingJob}) {
    const fallback = {
        id: pendingEventId,
        personaId,
        sourceMessageId: source.id,
        status: 'pending',
        summary: call.summary,
        notBefore: call.notBefore,
        expiresAt: call.expiresAt,
        dedupeKey: call.dedupeKey,
        createdAt: null,
        updatedAt: null,
        triggeredAt: null,
        consumedAt: null,
        cancelledAt: null
    };
    const pendingEvent = pendingShape(existing, fallback);
    const job = {
        id: jobIdOf(existingJob) || jobId,
        jobType: 'pending_event',
        personaId,
        runAfter: existingJob?.runAfter ?? existingJob?.run_after ?? call.notBefore,
        replayed: Boolean(existing && existingJob)
    };
    const preview = {pendingEvent, job, created: !existing || !existingJob, replayed: Boolean(existing && existingJob)};
    return {pendingEvent, job, preview};
}

function assertPlan(plan) {
    if (!isRecord(plan) || !PRIVATE_PLAN.has(plan)) throw new TypeError('Pending-event plan is invalid');
    return PRIVATE_PLAN.get(plan);
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
    throw new TypeError('Pending-event transaction must be a function or provide transaction()/run()');
}

export function createPendingEventFlow({repositories, clock, idGenerator, normalizeCall, normalizePendingEventCall: injectedNormalizer, transaction, effectAdapter} = {}) {
    if (!isRecord(repositories)) throw new TypeError('Pending-event flow repositories must be an object');
    const pendingRepository = resolveRepository(repositories, ['pendingEventRepository', 'pendingRepository', 'pendingEvent', 'pending'], 'pending event repository');
    const pendingFind = methodFor(pendingRepository, ['findByDedupeKey'], 'pending event repository');
    const pendingInsert = methodFor(pendingRepository, ['insertPendingEvent'], 'pending event repository');
    const pendingLinked = methodFor(pendingRepository, ['findLinkedJob'], 'pending event repository');
    const pendingEnsure = methodFor(pendingRepository, ['ensureLinkedJob'], 'pending event repository', {optional: true});
    const jobRepository = resolveRepository(repositories, ['jobRepository', 'job', 'effectRepository'], 'job repository', {optional: true});
    const jobEnqueue = jobRepository ? methodFor(jobRepository, ['enqueue'], 'job repository', {optional: true}) : null;
    const effectsPort = effectAdapter ?? (jobEnqueue ? createFlowEffectAdapter({jobRepository, clock, idGenerator}) : null);
    const sourceLookup = sourceLookupFor(repositories);
    // Keep the life-event port in the composition dependency set without using
    // it to persist pending authorization: pending rows are not life facts.
    const lifeEventRepository = resolveRepository(repositories, ['lifeEventRepository', 'lifeEvent', 'life'], 'life-event repository', {optional: true});
    const now = clockFunction(clock);
    const generateId = idFunction(idGenerator);
    const normalizer = normalizeCall ?? injectedNormalizer;
    if (normalizer !== undefined && typeof normalizer !== 'function') throw new TypeError('Pending-event flow normalizeCall must be a function');
    if (!pendingEnsure && !effectsPort) throw new TypeError('Pending-event flow requires pending.ensureLinkedJob() or a flow effect adapter');

    function plan(first, value, sourceMessageId, provenance = {}) {
        const command = typeof first === 'string'
            ? {personaId: first, call: value, sourceMessageId, provenance}
            : first;
        const current = now();
        const normalized = normalizeCommand(command, normalizer, Date.parse(current));
        const source = readSource(sourceLookup, normalized, normalized.personaId, normalized.sourceMessageId);
        const existing = pendingFind({personaId: normalized.personaId, dedupeKey: normalized.call.dedupeKey, notBefore: normalized.call.notBefore});
        if (existing && typeof existing.then === 'function') throw new TypeError('Pending-event repository reads must be synchronous');
        const existingJob = existing ? pendingLinked({personaId: normalized.personaId, pendingEventId: existing.id}) : null;
        if (existingJob && typeof existingJob.then === 'function') throw new TypeError('Pending-event job lookup must be synchronous');
        const pendingEventId = existing?.id || generateId('pending_event');
        const jobId = jobIdOf(existingJob) || generateId('job');
        const shape = planShape({personaId: normalized.personaId, source, call: normalized.call, pendingEventId, jobId, existing, existingJob});
        const result = {
            type: 'pending_event_plan',
            version: PENDING_EVENT_FLOW_VERSION,
            personaId: normalized.personaId,
            sourceMessageId: normalized.sourceMessageId,
            call: normalized.call,
            provenance: normalized.provenance,
            pendingEvent: shape.pendingEvent,
            pendingEventId,
            jobId: shape.job.id,
            preallocatedIds: {pendingEventId, jobId: shape.job.id},
            job: shape.job,
            created: shape.preview.created,
            replayed: shape.preview.replayed,
            preview: shape.preview,
            previewResult: shape.preview,
            sourceMessage: source
        };
        const planValue = deepFreeze(result);
        PRIVATE_PLAN.set(planValue, {call: normalized.call, provenance: normalized.provenance, source});
        return planValue;
    }

    function applyWithin(planValue, state) {
        const source = readSource(sourceLookup, {sourceMessage: state.source}, planValue.personaId, planValue.sourceMessageId);
        const existing = pendingFind({personaId: planValue.personaId, dedupeKey: state.call.dedupeKey, notBefore: state.call.notBefore});
        if (existing && typeof existing.then === 'function') throw new TypeError('Pending-event repository reads must be synchronous');
        let row = existing;
        let created = false;
        if (!row) {
            const createdAt = now();
            const payload = {
                schemaVersion: PENDING_EVENT_SCHEMA_VERSION,
                summary: state.call.summary,
                notBefore: state.call.notBefore,
                expiresAt: state.call.expiresAt,
                dedupeKey: state.call.dedupeKey,
                sourceMessageId: source.id,
                ...(state.provenance.callId ? {capabilityCallId: state.provenance.callId} : {}),
                ...(state.provenance.idempotencyKey ? {idempotencyKey: state.provenance.idempotencyKey} : {}),
                source: state.provenance.source
            };
            row = pendingInsert({
                id: planValue.preallocatedIds.pendingEventId,
                personaId: planValue.personaId,
                sourceMessageId: source.id,
                status: 'pending',
                summary: state.call.summary,
                notBefore: state.call.notBefore,
                expiresAt: state.call.expiresAt,
                dedupeKey: state.call.dedupeKey,
                payload,
                createdAt,
                updatedAt: createdAt
            });
            created = true;
        }
        const jobInput = {
            id: planValue.preallocatedIds.jobId,
            jobType: 'pending_event',
            personaId: planValue.personaId,
            priority: 2,
            maxAttempts: 4,
            runAfter: row.notBefore ?? row.not_before ?? state.call.notBefore,
            payload: {
                pendingEventId: row.id,
                ...(state.provenance.idempotencyKey ? {idempotencyKey: state.provenance.idempotencyKey} : {})
            }
        };
        let linked;
        if (pendingEnsure && !effectAdapter) {
            linked = pendingEnsure({personaId: planValue.personaId, pendingEventId: row.id, job: jobInput});
        } else {
            const prior = pendingLinked({personaId: planValue.personaId, pendingEventId: row.id});
            if (prior) linked = {job: prior, created: false};
            else {
                const queued = effectsPort.publish({
                    ...jobInput,
                    effectId: `effect_pending_${row.id}`,
                    kind: 'pending_event',
                    capability: 'pending',
                    idempotencyKey: state.provenance.idempotencyKey ?? `pending:${row.id}`
                }, {personaId: planValue.personaId, causationId: state.provenance.idempotencyKey ?? row.id, now: row.updatedAt ?? now()});
                const persisted = pendingLinked({personaId: planValue.personaId, pendingEventId: row.id});
                linked = {job: persisted || queued?.job || queued || null, created: queued?.created ?? true};
            }
        }
        if (linked && typeof linked.then === 'function') throw new TypeError('Pending-event job settlement must be synchronous');
        const linkedJob = linked?.job ?? (jobIdOf(linked) ? linked : null);
        const linkedJobId = linked?.jobId ?? jobIdOf(linkedJob);
        if (linked?.created) created = true;
        return {pendingEvent: pendingShape(row), jobId: linkedJobId, created};
    }

    function apply(planValue, options = {}) {
        const state = assertPlan(planValue);
        const settings = typeof options === 'function' ? {transaction: options} : options;
        if (!isRecord(settings)) throw new TypeError('Pending-event apply options must be an object');
        const runner = settings.transaction ?? settings.callerTransaction ?? settings.runInTransaction ?? settings.commit ?? transaction;
        return transactionRunner(runner, () => applyWithin(planValue, state));
    }

    return Object.freeze({
        version: PENDING_EVENT_FLOW_VERSION,
        plan,
        apply,
        normalizePendingEventCall: normalizer || normalizePendingEventCall,
        repositories: Object.freeze({pending: pendingRepository, job: jobRepository, lifeEvent: lifeEventRepository})
    });
}

export const createCompanionPendingEventFlow = createPendingEventFlow;
export default createPendingEventFlow;

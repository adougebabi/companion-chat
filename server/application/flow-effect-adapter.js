/**
 * Shared adapter for durable effect intents.
 *
 * Application flows describe work as effect intents. This adapter is the one
 * place that turns an intent into the existing durable job representation and
 * applies the idempotency lookup before enqueueing it. Lease, retry, and
 * settlement remain owned by the runtime job dispatcher.
 */

const MAX_TEXT = 240;

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function text(value, field, maxLength = MAX_TEXT) {
    if (typeof value !== 'string' || !value.trim()) throw new TypeError(`${field} must be a non-empty string`);
    const result = value.trim();
    if (result.length > maxLength) throw new RangeError(`${field} exceeds ${maxLength} characters`);
    return result;
}

function optionalText(value, field, maxLength = MAX_TEXT) {
    if (value === undefined || value === null || value === '') return null;
    return text(value, field, maxLength);
}

function clockFor(clock) {
    if (typeof clock === 'function') return () => new Date(clock()).toISOString();
    if (isRecord(clock) && typeof clock.now === 'function') return () => new Date(clock.now()).toISOString();
    return () => new Date().toISOString();
}

function idFor(idGenerator) {
    if (typeof idGenerator === 'function') return idGenerator;
    if (isRecord(idGenerator) && typeof idGenerator.next === 'function') return idGenerator.next.bind(idGenerator);
    let sequence = 0;
    return prefix => `${prefix}_${Date.now()}_${sequence++}`;
}

function methodFor(source, names) {
    if (!source) return null;
    for (const name of names) {
        if (typeof source[name] === 'function') return source[name].bind(source);
    }
    return null;
}

function effectIdFor(effect, nextId) {
    return text(effect.effectId ?? effect.id ?? nextId('effect'), 'Effect id');
}

function payloadFor(effect) {
    if (effect.payload === undefined) return {};
    if (!isRecord(effect.payload)) throw new TypeError('Effect payload must be an object');
    return effect.payload;
}

function capabilityFor(flowId, effect) {
    if (effect?.capability) return effect.capability;
    if (flowId.includes('media')) return 'media';
    if (flowId.includes('pending')) return 'pending';
    if (flowId.includes('timeline')) return 'timeline';
    if (flowId.includes('relationship')) return 'relationship';
    if (flowId.includes('activity')) return 'activity';
    if (flowId.includes('proactive')) return 'proactive';
    if (flowId.includes('life')) return 'life';
    return 'conversation';
}

function normalizeEffect(effect, context, nextId, now) {
    if (!isRecord(effect)) throw new TypeError('Effect intent must be an object');
    const payload = payloadFor(effect);
    const personaId = optionalText(effect.personaId ?? payload.personaId ?? context.personaId, 'Effect personaId', 160);
    const kind = text(effect.jobType ?? effect.kind ?? effect.type, 'Effect kind', 120);
    const effectId = effectIdFor(effect, nextId);
    const idempotencyKey = text(
        effect.idempotencyKey
            ?? payload.idempotencyKey
            ?? `${kind}:${personaId ?? 'global'}:${effectId}`,
        'Effect idempotencyKey'
    );
    const createdAt = effect.createdAt ?? effect.created_at ?? context.now ?? now();
    const runAfter = effect.runAfter ?? effect.run_after ?? context.runAfter ?? createdAt;
    const jobPayload = {...payload, idempotencyKey};
    return {
        effectId,
        personaId,
        kind,
        capability: effect.capability ?? null,
        idempotencyKey,
        causationId: optionalText(effect.causationId ?? effect.causation_id ?? context.causationId ?? personaId, 'Effect causationId') ?? personaId,
        job: {
            ...effect.job,
            id: effect.job?.id ?? effect.jobId ?? effect.id ?? nextId('job'),
            jobType: kind,
            personaId,
            activityId: effect.activityId ?? payload.activityId ?? effect.job?.activityId ?? null,
            messageId: effect.messageId ?? payload.messageId ?? effect.job?.messageId ?? null,
            traceId: effect.traceId ?? context.traceId ?? null,
            priority: Number.isInteger(effect.priority) ? effect.priority : Number.isInteger(effect.job?.priority) ? effect.job.priority : 0,
            maxAttempts: Number.isInteger(effect.maxAttempts) ? effect.maxAttempts : Number.isInteger(effect.job?.maxAttempts) ? effect.job.maxAttempts : 3,
            runAfter,
            createdAt,
            updatedAt: effect.updatedAt ?? effect.updated_at ?? createdAt,
            payload: jobPayload
        }
    };
}

function readExisting(findByKey, findByPayload, effect) {
    if (!effect.personaId) return null;
    if (findByKey) {
        const found = findByKey({
            personaId: effect.personaId,
            idempotencyKey: effect.idempotencyKey,
            jobType: effect.kind,
            kind: effect.kind
        });
        if (found && typeof found.then === 'function') throw new TypeError('Effect idempotency lookup must be synchronous');
        if (found) return found;
    }
    if (!findByPayload) return null;
    const found = findByPayload({
        personaId: effect.personaId,
        jobType: effect.kind,
        path: '$.idempotencyKey',
        payloadPath: '$.idempotencyKey',
        value: effect.idempotencyKey
    });
    if (found && typeof found.then === 'function') throw new TypeError('Effect payload lookup must be synchronous');
    return found ?? null;
}

/**
 * Compose one generic effect publisher over the existing job repository.
 * `enqueue()` is intentionally synchronous because all existing repositories
 * participate in caller-owned SQLite transactions.
 */
export function createFlowEffectAdapter({jobRepository, effectRepository, repository, clock, idGenerator, id} = {}) {
    const jobs = jobRepository ?? effectRepository ?? repository ?? null;
    if (!isRecord(jobs)) throw new TypeError('Flow effect adapter requires a job repository');
    const enqueue = methodFor(jobs, ['enqueue', 'create', 'insert']);
    if (!enqueue) throw new TypeError('Flow effect adapter requires jobRepository.enqueue()');
    const findByKey = methodFor(jobs, ['findByIdempotencyKey', 'findByEffectIdempotencyKey']);
    const findByPayload = methodFor(jobs, ['findByPayload']);
    const completeQueued = methodFor(jobs, ['completeQueued', 'supersedeQueued', 'cancelQueued']);
    const now = clockFor(clock);
    const nextId = idFor(idGenerator ?? id);

    function enqueueEffect(effect, context = {}) {
        const normalized = normalizeEffect(effect, isRecord(context) ? context : {}, nextId, now);
        const existing = readExisting(findByKey, findByPayload, normalized);
        if (existing) return {job: existing, created: false, replayed: true, effect: normalized};
        const job = enqueue(normalized.job) ?? normalized.job;
        return {job, created: true, replayed: false, effect: normalized};
    }

    function publish(effect, context = {}) {
        return enqueueEffect(effect, context);
    }

    function enqueueMany(effects, context = {}) {
        if (!Array.isArray(effects)) throw new TypeError('Flow effects must be an array');
        return effects.map(effect => enqueueEffect(effect, context));
    }

    function supersede(input = {}) {
        if (!completeQueued) return null;
        const value = {...input};
        if (value.result === undefined) value.result = {skipped: 'superseded_by_newer_effect'};
        return completeQueued(value);
    }

    function dispatch(result, context = {}) {
        if (!isRecord(result)) throw new TypeError('Flow result must be an object');
        return enqueueMany(result.effects ?? [], context);
    }

    return Object.freeze({
        enqueue: enqueueEffect,
        publish,
        enqueueMany,
        publishMany: enqueueMany,
        dispatch,
        supersede,
        repository: jobs
    });
}

/** Register a flow-shaped use case as a single application registry step. */
export function registerFlowAdapter(registry, {id, version = 1, flow, execute, stepId} = {}) {
    if (!registry || typeof registry.register !== 'function' || typeof registry.has !== 'function') {
        throw new TypeError('Flow adapter requires a flow registry');
    }
    const flowId = text(id, 'Flow adapter id');
    if (flow === undefined || flow === null) return registry;
    if (!isRecord(flow) && typeof flow !== 'function') throw new TypeError('Flow adapter requires a flow');
    if (registry.has(flowId)) return registry;
    const run = typeof execute === 'function'
        ? execute
        : typeof flow === 'function'
            ? flow
            : typeof flow.run === 'function' ? flow.run.bind(flow)
                : typeof flow.execute === 'function' ? flow.execute.bind(flow)
                    : typeof flow.handle === 'function' ? flow.handle.bind(flow)
                        : null;
    if (!run) throw new TypeError(`Flow adapter ${flowId} requires run(), execute(), handle(), or execute option`);
    registry.register({
        id: flowId,
        version,
        layer: 'application',
        dependencies: [{id: 'flow-effect-contract', layer: 'contracts'}],
        steps: [{
            id: stepId ?? `${flowId}-execute`,
            layer: 'application',
            dependencies: [{id: 'flow-effect-contract', layer: 'contracts'}],
            async run(context, command) {
                const value = await run(command, context);
                const result = isRecord(value) && ['facts', 'projections', 'effects', 'presentation'].some(channel => Array.isArray(value[channel]))
                    ? value
                    : isRecord(value?.result) ? value.result : value;
                return {
                    facts: Array.isArray(result?.facts) ? result.facts : [],
                    projections: Array.isArray(result?.projections) ? result.projections : [],
                    effects: Array.isArray(result?.effects) ? result.effects.map(effect => ({...effect, capability: capabilityFor(flowId, effect)})) : [],
                    presentation: Array.isArray(result?.presentation) ? result.presentation : []
                };
            }
        }]
    });
    return registry;
}

export default createFlowEffectAdapter;

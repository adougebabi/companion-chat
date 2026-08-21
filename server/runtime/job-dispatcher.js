const DEFAULT_RETRY_DELAY_MS = 1_000;
const DEFAULT_MAX_RETRY_DELAY_MS = 15 * 60 * 1_000;
export const MAX_JOB_ERROR_LENGTH = 500;

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function requiredText(value, field) {
    if (typeof value !== 'string' || value.trim() === '') throw new TypeError(`${field} must be a non-empty string`);
    return value.trim();
}

function optionalText(value) {
    return typeof value === 'string' && value.trim() ? value.trim() : null;
}

function valueFor(value, camelName, snakeName) {
    return value?.[camelName] === undefined ? value?.[snakeName] : value[camelName];
}

function boundedError(error, limit = MAX_JOB_ERROR_LENGTH) {
    const source = error instanceof Error ? error.message : error;
    const text = String(source ?? 'Unknown job failure').replace(/\s+/g, ' ').trim() || 'Unknown job failure';
    return text.length <= limit ? text : `${text.slice(0, Math.max(0, limit - 3))}...`;
}

function resolveClock(clock) {
    if (clock === undefined) return () => new Date().toISOString();
    if (typeof clock === 'function') return () => clock();
    if (isRecord(clock) && typeof clock.now === 'function') return () => clock.now();
    throw new TypeError('Job dispatcher clock must be a function or provide now()');
}

function timestamp(value, field = 'Job time') {
    const resolved = value instanceof Date ? value.toISOString() : value;
    if (typeof resolved !== 'string' || !resolved.trim() || !Number.isFinite(Date.parse(resolved))) {
        throw new TypeError(`${field} must be a valid timestamp`);
    }
    return resolved;
}

function resolveCallback(options, repository, names, field) {
    for (const name of names) {
        if (options[name] !== undefined) {
            if (typeof options[name] !== 'function') throw new TypeError(`Job dispatcher ${field} must be a function`);
            return options[name];
        }
    }
    if (!repository) return null;
    for (const name of names) {
        if (repository[name] !== undefined) {
            if (typeof repository[name] !== 'function') throw new TypeError(`Job dispatcher repository.${name} must be a function`);
            return repository[name].bind(repository);
        }
    }
    return null;
}

function jobId(job) {
    return requiredText(job?.id, 'Claimed job id');
}

function jobType(job) {
    return requiredText(valueFor(job, 'jobType', 'job_type'), 'Claimed job type');
}

function jobOwner(job) {
    return optionalText(valueFor(job, 'leaseOwner', 'lease_owner'));
}

function jobExpiry(job) {
    return valueFor(job, 'leaseExpiresAt', 'lease_expires_at');
}

function personaId(job) {
    return valueFor(job, 'personaId', 'persona_id');
}

function contextOwner(context, job) {
    return optionalText(context?.leaseOwner ?? context?.owner) || jobOwner(job);
}

function contextNow(context, clock) {
    return timestamp(context?.now ?? clock(), 'Job dispatcher now');
}

function attemptsFor(job) {
    const attempt = Number(valueFor(job, 'attemptCount', 'attempt_count'));
    const maximum = Number(valueFor(job, 'maxAttempts', 'max_attempts'));
    return {
        attempt: Number.isInteger(attempt) && attempt >= 1 ? attempt : 1,
        maximum: Number.isInteger(maximum) && maximum >= 1 ? maximum : 1
    };
}

function retryAtFor(value, now, attempt, baseDelay, maxDelay) {
    const requested = value?.retryAt ?? value?.retry_at ?? value?.runAfter ?? value?.run_after;
    if (requested !== undefined) return timestamp(requested, 'Job retry time');
    const delay = Math.min(maxDelay, baseDelay * (2 ** Math.min(Math.max(attempt - 1, 0), 8)));
    return new Date(Date.parse(now) + delay).toISOString();
}

function registrationValue(type, handler, receiver) {
    const jobTypeValue = typeof type === 'object' && type !== null ? (type.jobType ?? type.job_type) : type;
    const configuredHandler = typeof type === 'object' && type !== null && handler === undefined ? type.handler ?? type.run : handler;
    const configuredReceiver = typeof type === 'object' && type !== null && receiver === undefined ? type.receiver : receiver;
    const normalizedType = requiredText(jobTypeValue, 'Job type');
    let resolvedHandler = configuredHandler;
    let resolvedReceiver = configuredReceiver;
    if (isRecord(configuredHandler) && typeof configuredHandler.handle === 'function') {
        resolvedReceiver = resolvedReceiver ?? configuredHandler;
        resolvedHandler = configuredHandler.handle;
    }
    if (typeof resolvedHandler !== 'function') throw new TypeError(`Job handler ${normalizedType} must be a function`);
    return {jobType: normalizedType, handler: resolvedHandler, receiver: resolvedReceiver};
}

function descriptorReceiver(value) {
    if (!isRecord(value)) return undefined;
    if (value.receiver !== undefined) return value.receiver;
    return typeof value.handler === 'function' || typeof value.handle === 'function' || typeof value.run === 'function'
        ? value
        : undefined;
}

function normalizeHandlerOutcome(value) {
    if (!isRecord(value)) return {status: 'complete', result: value};
    const requestedStatus = value.status ?? value.outcome;
    if (value.retry === true || requestedStatus === 'retry') {
        return {status: 'retry', result: value.result, error: value.error, retryAt: value.retryAt ?? value.retry_at ?? value.runAfter ?? value.run_after};
    }
    if (value.terminal === true || requestedStatus === 'failed' || requestedStatus === 'terminal') {
        return {status: 'failed', result: value.result, error: value.error || 'Job handler reported a terminal failure'};
    }
    return {status: 'complete', result: value.result === undefined && requestedStatus === 'complete' ? value : value.result === undefined ? value : value.result};
}

function normalizeHandlerError(error) {
    return {
        status: error?.terminal === true || error?.retryable === false ? 'failed' : 'error',
        error: boundedError(error),
        result: isRecord(error?.result) ? error.result : undefined,
        retryAt: error?.retryAt ?? error?.retry_at ?? error?.runAfter ?? error?.run_after
    };
}

/**
 * Register and execute durable jobs without knowing any feature or provider.
 *
 * The dispatcher deliberately accepts repository callbacks instead of a
 * database. A worker can inject `jobTick` or `runJob` into worker-runtime;
 * handlers only receive a claimed job and an abort/lease context.
 */
export function createJobDispatcher(options = {}) {
    if (!isRecord(options)) throw new TypeError('Job dispatcher options must be an object');
    const repository = options.jobRepository ?? options.effectRepository ?? options.repository ?? null;
    if (repository !== null && !isRecord(repository)) throw new TypeError('Job dispatcher repository must be an object');
    const clock = resolveClock(options.clock);
    const errorLimit = Number(options.maxErrorLength ?? options.errorLimit ?? MAX_JOB_ERROR_LENGTH);
    if (!Number.isInteger(errorLimit) || errorLimit < 16) throw new RangeError('Job dispatcher maxErrorLength must be an integer >= 16');
    const retryDelayMs = Number(options.retryDelayMs ?? DEFAULT_RETRY_DELAY_MS);
    const maxRetryDelayMs = Number(options.maxRetryDelayMs ?? DEFAULT_MAX_RETRY_DELAY_MS);
    if (!Number.isFinite(retryDelayMs) || retryDelayMs < 0) throw new RangeError('Job dispatcher retryDelayMs must be non-negative');
    if (!Number.isFinite(maxRetryDelayMs) || maxRetryDelayMs < retryDelayMs) throw new RangeError('Job dispatcher maxRetryDelayMs must be >= retryDelayMs');

    const claim = resolveCallback(options, repository, ['claimJob', 'claim'], 'claimJob');
    const findLeased = resolveCallback(options, repository, ['findLeased', 'getLeased', 'isClaimed'], 'lease lookup');
    const settle = resolveCallback(options, repository, ['settleJob', 'settle'], 'settleJob');
    const retry = resolveCallback(options, repository, ['retryJob', 'retry'], 'retryJob');
    const onRetry = resolveCallback(options, null, ['onRetry'], 'onRetry');
    const onTerminal = resolveCallback(options, null, ['onTerminal', 'onTerminalFailure'], 'onTerminal');
    const onSettled = resolveCallback(options, null, ['onSettled'], 'onSettled');
    const defaultReceiver = options.receiver;

    const handlers = new Map();
    let publicDispatcher;

    function register(type, handler, receiver) {
        const entry = registrationValue(type, handler, receiver ?? defaultReceiver);
        const existing = handlers.get(entry.jobType);
        if (existing) {
            if (existing.handler !== entry.handler || existing.receiver !== entry.receiver) {
                throw new Error(`Job type already registered: ${entry.jobType}`);
            }
            return publicDispatcher;
        }
        handlers.set(entry.jobType, Object.freeze(entry));
        return publicDispatcher;
    }

    function registerMany(input) {
        if (input === undefined || input === null) return;
        if (input instanceof Map) {
            for (const [type, value] of input) register(type, value?.handler ?? value?.run ?? value, descriptorReceiver(value));
            return;
        }
        if (Array.isArray(input)) {
            for (const value of input) register(value);
            return;
        }
        if (!isRecord(input)) throw new TypeError('Job dispatcher handlers must be a map, array, or object');
        for (const [type, value] of Object.entries(input)) {
            register(type, value?.handler ?? value?.run ?? value, descriptorReceiver(value));
        }
    }

    function activeClaim(job, context, {checkRepository = true} = {}) {
        const id = jobId(job);
        const owner = contextOwner(context, job);
        if (!owner) return {active: false, reason: 'missing_lease_owner', id, owner: null};
        const persistedOwner = jobOwner(job);
        if (persistedOwner && persistedOwner !== owner) return {active: false, reason: 'stale_owner', id, owner};
        if (job.status !== undefined && job.status !== 'leased') return {active: false, reason: 'not_leased', id, owner};
        if (context?.signal?.aborted) return {active: false, reason: 'aborted', id, owner};

        const now = contextNow(context, clock);
        const expiry = jobExpiry(job);
        if (expiry !== undefined && (!Number.isFinite(Date.parse(expiry)) || Date.parse(expiry) <= Date.parse(now))) {
            return {active: false, reason: 'expired_lease', id, owner, now};
        }
        if (!checkRepository || !findLeased) return {active: true, id, owner, now};
        const current = findLeased({id, leaseOwner: owner, personaId: personaId(job), now});
        if (current && typeof current.then === 'function') return current.then(value => value ? {active: true, id, owner, now, current: value} : {active: false, reason: 'stale_lease', id, owner, now});
        return current ? {active: true, id, owner, now, current} : {active: false, reason: 'stale_lease', id, owner, now};
    }

    function staleResult(job, state) {
        return {status: 'stale', reason: state.reason, changed: false, job, settlement: null};
    }

    async function settleResult(job, context, status, result, error, retryAt) {
        const state = await activeClaim(job, context);
        if (!state.active) return staleResult(job, state);
        const input = {
            id: state.id,
            personaId: personaId(job),
            leaseOwner: state.owner,
            status: status === 'failed' ? 'failed' : status === 'retry' ? undefined : 'complete',
            runAfter: status === 'retry' ? retryAt : state.now,
            ...(result !== undefined ? {result} : {}),
            ...(error ? {error: boundedError(error, errorLimit)} : {}),
            terminal: status === 'failed',
            now: state.now
        };
        let settlement;
        if (status === 'retry') {
            if (retry) settlement = await retry(input);
            else if (settle) settlement = await settle(input);
        } else if (settle) {
            settlement = await settle(input);
        }
        const changed = settlement?.changed === undefined ? Boolean(settlement) : Boolean(settlement.changed);
        const output = {status, changed, job, settlement: settlement ?? null, result: result ?? null, error: error ? boundedError(error, errorLimit) : null};
        if (status === 'retry' && onRetry) await onRetry(output, context);
        if (status === 'failed' && onTerminal) await onTerminal(output, context);
        if (onSettled) await onSettled(output, context);
        return output;
    }

    async function runJob(input, context = {}) {
        // Accept both worker-runtime's `(job, context)` form and a compact
        // claimed envelope for callers that pass lease metadata together.
        const wrapped = isRecord(input) && isRecord(input.job) && input.id === undefined;
        const job = wrapped ? input.job : input;
        const runContext = wrapped ? {...input, ...context, ...input.jobContext} : context;
        const type = jobType(job);
        const initial = await activeClaim(job, runContext);
        if (!initial.active) return staleResult(job, initial);
        const registration = handlers.get(type);
        if (!registration) {
            return settleResult(job, runContext, 'failed', {reason: 'unknown_job_type', jobType: type}, `Unknown job type: ${type}`);
        }

        let outcome;
        try {
            outcome = normalizeHandlerOutcome(await registration.handler.call(registration.receiver, job, runContext));
        } catch (error) {
            outcome = normalizeHandlerError(error);
        }
        if (outcome.status === 'complete') return settleResult(job, runContext, 'complete', outcome.result);

        const {attempt, maximum} = attemptsFor(job);
        const terminal = outcome.status === 'failed' || attempt >= maximum;
        if (terminal) return settleResult(job, runContext, 'failed', outcome.result, outcome.error || 'Job handler failed');
        const now = contextNow(runContext, clock);
        const retryAt = retryAtFor(outcome, now, attempt, retryDelayMs, maxRetryDelayMs);
        return settleResult(job, runContext, 'retry', outcome.result, outcome.error || 'Job handler failed', retryAt);
    }

    async function jobTick(context = {}) {
        if (!claim) throw new TypeError('Job dispatcher requires claimJob or repository.claim() for jobTick');
        if (context.signal?.aborted) return {status: 'aborted', job: null, changed: false};
        const job = await claim({
            ...context,
            leaseOwner: context.leaseOwner ?? context.owner,
            owner: context.owner ?? context.leaseOwner
        });
        if (!job) return null;
        return runJob(job, context);
    }

    function list() {
        return [...handlers.keys()];
    }

    function get(type) {
        const entry = handlers.get(type);
        return entry ? entry.handler : null;
    }

    publicDispatcher = {
        register,
        registerHandler: register,
        has: type => handlers.has(type),
        get,
        list,
        runJob,
        execute: runJob,
        dispatch: runJob,
        jobTick,
        tick: jobTick,
        createWorkerAdapters() {
            return Object.freeze({jobTick, runJob});
        }
    };
    registerMany(options.handlers ?? options.registry);
    return Object.freeze(publicDispatcher);
}

export const createDurableJobDispatcher = createJobDispatcher;
export default createJobDispatcher;

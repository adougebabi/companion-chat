const DEFAULT_POLL_INTERVAL_MS = 2_500;
const DEFAULT_LEASE_MS = 60_000;
const MAX_ERROR_LENGTH = 240;
const DEFAULT_DRAIN_TIMEOUT_MS = 5_000;

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function requiredText(value, field) {
    if (typeof value !== 'string' || value.trim() === '') throw new TypeError(`${field} must be a non-empty string`);
    return value.trim();
}

function positiveDuration(value, field, fallback) {
    const resolved = value === undefined ? fallback : value;
    if (!Number.isFinite(resolved) || resolved <= 0) throw new RangeError(`${field} must be a positive finite number`);
    return resolved;
}

function nonNegativeDuration(value, field, fallback) {
    const resolved = value === undefined ? fallback : value;
    if (!Number.isFinite(resolved) || resolved < 0) throw new RangeError(`${field} must be a non-negative finite number`);
    return resolved;
}

function resolveCallback(options, names) {
    for (const name of names) {
        if (options[name] !== undefined) {
            if (typeof options[name] !== 'function') throw new TypeError(`Worker runtime ${name} must be a function`);
            return options[name];
        }
    }
    return null;
}

function resolvePortCallback(options, port, names, field) {
    const direct = resolveCallback(options, names);
    if (direct) return direct;
    if (!port) return null;
    for (const name of names) {
        if (port[name] !== undefined) {
            if (typeof port[name] !== 'function') throw new TypeError(`Worker runtime ${field}.${name} must be a function`);
            return port[name].bind(port);
        }
    }
    return null;
}

function resolveTimers(options) {
    const timers = options.timers ?? options.timer ?? {};
    if (!isRecord(timers)) throw new TypeError('Worker runtime timers must be an object');
    const resolved = {
        setInterval: options.setInterval ?? timers.setInterval ?? globalThis.setInterval,
        clearInterval: options.clearInterval ?? timers.clearInterval ?? globalThis.clearInterval,
        setTimeout: options.setTimeout ?? timers.setTimeout ?? globalThis.setTimeout,
        clearTimeout: options.clearTimeout ?? timers.clearTimeout ?? globalThis.clearTimeout
    };
    for (const name of Object.keys(resolved)) {
        if (typeof resolved[name] !== 'function') throw new TypeError(`Worker runtime timers.${name} must be a function`);
    }
    return resolved;
}

function resolveClock(clock) {
    if (clock === undefined) return () => new Date().toISOString();
    if (typeof clock === 'function') return clock;
    if (isRecord(clock) && typeof clock.now === 'function') return () => clock.now();
    throw new TypeError('Worker runtime clock must be a function or provide now()');
}

function resolveAbortControllerFactory(options) {
    const configured = options.abortControllerFactory ?? options.createAbortController;
    if (configured !== undefined) {
        if (typeof configured !== 'function') throw new TypeError('Worker runtime abortControllerFactory must be a function');
        return configured;
    }
    return () => new AbortController();
}

function defaultOwner() {
    if (globalThis.crypto && typeof globalThis.crypto.randomUUID === 'function') {
        return `worker_${globalThis.crypto.randomUUID()}`;
    }
    return `worker_${Date.now().toString(36)}_${Math.random().toString(36).slice(2)}`;
}

function errorText(error) {
    const message = error instanceof Error ? error.message : String(error);
    const compact = message.replace(/\s+/g, ' ').trim() || 'Unknown worker runtime failure';
    return compact.length <= MAX_ERROR_LENGTH ? compact : `${compact.slice(0, MAX_ERROR_LENGTH - 3)}...`;
}

function isAbortError(error, signal) {
    return Boolean(signal?.aborted) || error?.name === 'AbortError' || error?.code === 'ABORT_ERR';
}

function contextFor({owner, signal, clock, leaseMs, runtime}) {
    return Object.freeze({
        owner,
        leaseOwner: owner,
        signal,
        now: clock(),
        leaseMs,
        runtime
    });
}

/**
 * Create the in-process owner for durable jobs.
 *
 * The runtime owns lifecycle and scheduling only. It never knows job types or
 * provider behavior. Callers can inject `jobTick`, or inject `claimJob` and
 * `runJob` to use the small default claim/execute adapter. Startup performs
 * lease recovery and arms the periodic timer; pass `runOnStart: true` when an
 * immediate first tick is part of the caller's policy.
 */
export function createWorkerRuntime(options = {}) {
    if (!isRecord(options)) throw new TypeError('Worker runtime options must be an object');

    const repository = options.jobRepository ?? options.effectRepository ?? null;
    if (repository !== null && !isRecord(repository)) throw new TypeError('Worker runtime jobRepository must be an object');

    const owner = requiredText(
        options.leaseOwner ?? options.workerOwner ?? options.owner ?? defaultOwner(),
        'Worker runtime leaseOwner'
    );
    const pollIntervalMs = positiveDuration(options.pollIntervalMs ?? options.intervalMs, 'Worker runtime pollIntervalMs', DEFAULT_POLL_INTERVAL_MS);
    const leaseMs = positiveDuration(options.leaseMs ?? options.leaseDurationMs, 'Worker runtime leaseMs', DEFAULT_LEASE_MS);
    const startupDelayMs = nonNegativeDuration(options.startupDelayMs, 'Worker runtime startupDelayMs', 0);
    const runOnStart = options.runOnStart === true;
    const clock = resolveClock(options.clock);
    const timers = resolveTimers(options);
    const abortControllerFactory = resolveAbortControllerFactory(options);
    const recoverLeases = resolvePortCallback(options, repository, ['recoverLeases', 'recoverExpiredLeases', 'reclaimExpiredLeases', 'recover'], 'jobRepository');
    const claimJob = resolvePortCallback(options, repository, ['claimJob', 'claim'], 'jobRepository');
    const runJob = resolvePortCallback(options, repository, ['runJob', 'executeJob', 'dispatchJob', 'handleJob'], 'jobRepository');
    const injectedTick = resolveCallback(options, ['jobTick', 'tick', 'processTick']);
    const onError = options.onError === undefined ? (error => {
        console.warn(`Companion worker runtime failed: ${errorText(error)}`);
    }) : options.onError;
    if (typeof onError !== 'function') throw new TypeError('Worker runtime onError must be a function');
    if (!injectedTick && (!claimJob || !runJob)) {
        throw new TypeError('Worker runtime requires jobTick or both claimJob and runJob callbacks');
    }

    let phase = 'idle';
    let startPromise = null;
    let stopPromise = null;
    let controller = null;
    let intervalHandle = null;
    let startupHandle = null;
    let tickPromise = null;
    let generation = 0;
    let tickCount = 0;
    let skippedTickCount = 0;
    let recoveryCount = 0;
    let failureCount = 0;

    function isCurrent(activeGeneration, signal) {
        return phase === 'running' && generation === activeGeneration && signal === controller?.signal && !signal.aborted;
    }

    function callbackContext(signal) {
        return contextFor({owner, signal, clock, leaseMs, runtime: publicRuntime});
    }

    async function invokeRecovery(activeGeneration, signal) {
        if (!recoverLeases || !isCurrent(activeGeneration, signal)) return null;
        const context = callbackContext(signal);
        const result = await recoverLeases(context);
        if (!isCurrent(activeGeneration, signal)) return null;
        recoveryCount += 1;
        return result;
    }

    async function invokeTick(activeGeneration, signal) {
        if (!isCurrent(activeGeneration, signal)) return null;
        const context = callbackContext(signal);
        if (injectedTick) return injectedTick(context);

        const job = await claimJob({...context, claim: claimJob, run: runJob});
        if (!isCurrent(activeGeneration, signal) || !job) return job ?? null;
        return runJob(job, context);
    }

    function reportFailure(error, context) {
        if (isAbortError(error, context?.signal)) return;
        failureCount += 1;
        try {
            onError(error, context);
        } catch {
            // Error reporting must not break the timer callback or lifecycle.
        }
    }

    function scheduleTick(activeGeneration, signal, {recover = true} = {}) {
        if (!isCurrent(activeGeneration, signal)) return null;
        if (tickPromise) {
            skippedTickCount += 1;
            return tickPromise;
        }
        tickCount += 1;
        let context;
        try {
            context = callbackContext(signal);
        } catch (error) {
            reportFailure(error, {owner, leaseOwner: owner, signal, leaseMs, runtime: publicRuntime});
            return Promise.resolve(undefined);
        }
        const pending = Promise.resolve()
            .then(() => recover ? invokeRecovery(activeGeneration, signal) : null)
            .then(() => invokeTick(activeGeneration, signal))
            .catch(error => {
                reportFailure(error, context);
                return undefined;
            })
            .finally(() => {
                if (tickPromise === pending) tickPromise = null;
            });
        tickPromise = pending;
        return pending;
    }

    function clearTimers() {
        if (startupHandle !== null) {
            timers.clearTimeout(startupHandle);
            startupHandle = null;
        }
        if (intervalHandle !== null) {
            timers.clearInterval(intervalHandle);
            intervalHandle = null;
        }
    }

    function armTimers(activeGeneration, signal, {skipInitialRecovery = false} = {}) {
        if (!isCurrent(activeGeneration, signal)) return;
        const armInterval = () => {
            if (!isCurrent(activeGeneration, signal) || intervalHandle !== null) return;
            intervalHandle = timers.setInterval(() => {
                scheduleTick(activeGeneration, signal);
            }, pollIntervalMs);
        };
        if (startupDelayMs > 0) {
            startupHandle = timers.setTimeout(() => {
                startupHandle = null;
                if (!isCurrent(activeGeneration, signal)) return;
                if (runOnStart) scheduleTick(activeGeneration, signal, {recover: false});
                armInterval();
            }, startupDelayMs);
            return;
        }
        if (runOnStart) scheduleTick(activeGeneration, signal, {recover: !skipInitialRecovery});
        armInterval();
    }

    function start() {
        if (phase === 'running') return Promise.resolve(false);
        if (phase === 'starting') return startPromise;
        if (phase === 'stopping') return stopPromise?.then(() => start());

        phase = 'starting';
        generation += 1;
        const activeGeneration = generation;
        try {
            controller = abortControllerFactory();
        } catch (error) {
            phase = 'idle';
            controller = null;
            return Promise.reject(error);
        }
        if (!controller || !controller.signal || typeof controller.abort !== 'function') {
            phase = 'idle';
            controller = null;
            return Promise.reject(new TypeError('Worker runtime abortControllerFactory must return {signal, abort()}'));
        }
        const signal = controller.signal;
        let pendingStart;
        pendingStart = (async () => {
            phase = 'running';
            try {
                if (!isCurrent(activeGeneration, signal)) return false;
                if (recoverLeases) await invokeRecovery(activeGeneration, signal);
                if (!isCurrent(activeGeneration, signal)) return false;
                armTimers(activeGeneration, signal, {skipInitialRecovery: Boolean(recoverLeases)});
                return true;
            } catch (error) {
                const ownsRuntime = generation === activeGeneration && controller?.signal === signal;
                if (ownsRuntime && phase !== 'stopping' && phase !== 'stopped') {
                    clearTimers();
                    if (!signal.aborted) controller?.abort();
                    phase = 'idle';
                    controller = null;
                }
                reportFailure(error, {owner, leaseOwner: owner, signal, leaseMs, runtime: publicRuntime});
                throw error;
            } finally {
                if (startPromise === pendingStart) startPromise = null;
            }
        })();
        startPromise = pendingStart;
        return startPromise;
    }

    function stop({waitForTasks = false, drainTimeoutMs = DEFAULT_DRAIN_TIMEOUT_MS} = {}) {
        if (phase === 'idle' || phase === 'stopped') return Promise.resolve(false);
        if (phase === 'stopping') return stopPromise;

        phase = 'stopping';
        clearTimers();
        const activeController = controller;
        const activeTick = tickPromise;
        if (activeController && !activeController.signal.aborted) activeController.abort();
        let drainHandle = null;
        const drain = waitForTasks && activeTick
            ? Promise.race([
                Promise.resolve(activeTick).catch(() => undefined),
                new Promise(resolve => {
                    drainHandle = timers.setTimeout(resolve, Math.max(0, Number(drainTimeoutMs) || DEFAULT_DRAIN_TIMEOUT_MS));
                })
            ]).finally(() => {
                if (drainHandle !== null) timers.clearTimeout(drainHandle);
            })
            : Promise.resolve();
        stopPromise = drain.then(() => {
            clearTimers();
            if (controller === activeController) controller = null;
            phase = 'stopped';
            stopPromise = null;
            return true;
        });
        return stopPromise;
    }

    const publicRuntime = {
        start,
        stop,
        tick() {
            if (phase !== 'running' || !controller) return Promise.resolve(null);
            return scheduleTick(generation, controller.signal);
        },
        get owner() {
            return owner;
        },
        get leaseOwner() {
            return owner;
        },
        get signal() {
            return controller?.signal ?? null;
        },
        get state() {
            return phase;
        },
        get isRunning() {
            return phase === 'running';
        },
        get stats() {
            return Object.freeze({tickCount, skippedTickCount, recoveryCount, failureCount});
        }
    };

    return Object.freeze(publicRuntime);
}

export const createWorkerRuntimeOwner = createWorkerRuntime;
export default createWorkerRuntime;

const DEFAULT_INTERVAL_MS = 60_000;
const DEFAULT_STARTUP_DELAY_MS = 0;
export const MAX_TASK_ERROR_LENGTH = 240;

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function requiredText(value, field) {
    if (typeof value !== 'string' || value.trim() === '') {
        throw new TypeError(`${field} must be a non-empty string`);
    }
    return value.trim();
}

function positiveDuration(value, field, fallback) {
    const resolved = value === undefined ? fallback : value;
    if (!Number.isFinite(resolved) || resolved <= 0) {
        throw new RangeError(`${field} must be a positive finite number`);
    }
    return resolved;
}

function nonNegativeDuration(value, field, fallback) {
    const resolved = value === undefined ? fallback : value;
    if (!Number.isFinite(resolved) || resolved < 0) {
        throw new RangeError(`${field} must be a non-negative finite number`);
    }
    return resolved;
}

function resolveTimers(options) {
    const configured = options.timers ?? options.timer ?? {};
    if (!isRecord(configured)) throw new TypeError('Task runtime timers must be an object');

    const timers = {
        setTimeout: options.setTimeout ?? configured.setTimeout ?? globalThis.setTimeout,
        clearTimeout: options.clearTimeout ?? configured.clearTimeout ?? globalThis.clearTimeout,
        setInterval: options.setInterval ?? configured.setInterval ?? globalThis.setInterval,
        clearInterval: options.clearInterval ?? configured.clearInterval ?? globalThis.clearInterval
    };
    for (const name of Object.keys(timers)) {
        if (typeof timers[name] !== 'function') throw new TypeError(`Task runtime timers.${name} must be a function`);
    }
    return timers;
}

function resolveAbortControllerFactory(options) {
    const configured = options.abortControllerFactory ?? options.createAbortController;
    if (configured === undefined) return () => new AbortController();
    if (typeof configured !== 'function') throw new TypeError('Task runtime abortControllerFactory must be a function');
    return configured;
}

function defaultOwner() {
    if (globalThis.crypto && typeof globalThis.crypto.randomUUID === 'function') {
        return `task_${globalThis.crypto.randomUUID()}`;
    }
    return `task_${Date.now().toString(36)}_${Math.random().toString(36).slice(2)}`;
}

function boundedErrorMessage(error) {
    const source = error instanceof Error ? error.message : String(error ?? 'Unknown task failure');
    const compact = source.replace(/\s+/g, ' ').trim() || 'Unknown task failure';
    if (compact.length <= MAX_TASK_ERROR_LENGTH) return compact;
    return `${compact.slice(0, MAX_TASK_ERROR_LENGTH - 3)}...`;
}

/**
 * Do not pass provider or persistence error objects through the task boundary.
 * The bounded error intentionally has no `cause`, stack, or copied properties.
 */
export class TaskRuntimeError extends Error {
    constructor(error) {
        super(boundedErrorMessage(error));
        this.name = 'TaskRuntimeError';
        this.code = 'TASK_RUNTIME_ERROR';
    }
}

function isAbortError(error, signal) {
    return Boolean(signal?.aborted) || error?.name === 'AbortError' || error?.code === 'ABORT_ERR';
}

/**
 * Own one periodic in-process task. The task owns its business behavior; this
 * module only owns its stable owner identity, timer lifecycle, cancellation,
 * and generation checks around restart.
 */
export function createTaskRuntime(options = {}) {
    if (!isRecord(options)) throw new TypeError('Task runtime options must be an object');
    if (typeof options.task !== 'function') throw new TypeError('Task runtime requires task()');

    const owner = requiredText(options.owner ?? options.taskOwner ?? defaultOwner(), 'Task runtime owner');
    const startupDelayMs = nonNegativeDuration(options.startupDelayMs, 'Task runtime startupDelayMs', DEFAULT_STARTUP_DELAY_MS);
    const intervalMs = positiveDuration(options.intervalMs, 'Task runtime intervalMs', DEFAULT_INTERVAL_MS);
    const timers = resolveTimers(options);
    const abortControllerFactory = resolveAbortControllerFactory(options);
    const onError = options.onError === undefined
        ? error => console.warn(`Task runtime failed: ${error.message}`)
        : options.onError;
    if (typeof onError !== 'function') throw new TypeError('Task runtime onError must be a function');

    let phase = 'idle';
    let generation = 0;
    let controller = null;
    let startPromise = null;
    let stopPromise = null;
    let startupHandle = null;
    let intervalHandle = null;
    let taskPromise = null;
    let taskCount = 0;
    let skippedTaskCount = 0;
    let failureCount = 0;

    function isCurrent(activeGeneration, signal) {
        return phase === 'running'
            && generation === activeGeneration
            && signal === controller?.signal
            && !signal.aborted;
    }

    function contextFor(signal, activeGeneration) {
        return Object.freeze({
            owner,
            signal,
            generation: activeGeneration,
            runtime: publicRuntime
        });
    }

    function reportError(error, context) {
        if (isAbortError(error, context?.signal)) return;
        const bounded = error instanceof TaskRuntimeError ? error : new TaskRuntimeError(error);
        failureCount += 1;
        try {
            onError(bounded, context);
        } catch {
            // Error observers must not break timer cleanup or lifecycle state.
        }
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

    function runTask(activeGeneration, signal) {
        if (!isCurrent(activeGeneration, signal)) return null;
        if (taskPromise) {
            skippedTaskCount += 1;
            return taskPromise;
        }

        taskCount += 1;
        const context = contextFor(signal, activeGeneration);
        let pending;
        pending = Promise.resolve()
            .then(() => {
                if (!isCurrent(activeGeneration, signal)) return undefined;
                return options.task(context);
            })
            .catch(error => {
                if (isCurrent(activeGeneration, signal)) reportError(error, context);
                return undefined;
            })
            .finally(() => {
                if (taskPromise === pending) taskPromise = null;
            });
        taskPromise = pending;
        return pending;
    }

    function armTimers(activeGeneration, signal) {
        if (!isCurrent(activeGeneration, signal)) return;
        const armInterval = () => {
            if (!isCurrent(activeGeneration, signal) || intervalHandle !== null) return;
            intervalHandle = timers.setInterval(() => {
                runTask(activeGeneration, signal);
            }, intervalMs);
        };

        if (startupDelayMs > 0) {
            startupHandle = timers.setTimeout(() => {
                startupHandle = null;
                if (!isCurrent(activeGeneration, signal)) return;
                runTask(activeGeneration, signal);
                armInterval();
            }, startupDelayMs);
            return;
        }

        runTask(activeGeneration, signal);
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
            return Promise.reject(new TypeError('Task runtime abortControllerFactory must return {signal, abort()}'));
        }

        const signal = controller.signal;
        let pendingStart;
        pendingStart = Promise.resolve().then(() => {
            if (generation !== activeGeneration || controller?.signal !== signal || signal.aborted) return false;
            phase = 'running';
            try {
                armTimers(activeGeneration, signal);
                return true;
            } catch (error) {
                const ownsRuntime = generation === activeGeneration && controller?.signal === signal;
                if (ownsRuntime) {
                    clearTimers();
                    if (!signal.aborted) controller.abort();
                    phase = 'idle';
                    controller = null;
                }
                reportError(error, {owner, signal, generation: activeGeneration, runtime: publicRuntime});
                throw error;
            }
        }).finally(() => {
            if (startPromise === pendingStart) startPromise = null;
        });
        startPromise = pendingStart;
        return startPromise;
    }

    function stop() {
        if (phase === 'idle' || phase === 'stopped') return Promise.resolve(false);
        if (phase === 'stopping') return stopPromise;

        phase = 'stopping';
        clearTimers();
        const activeController = controller;
        if (activeController && !activeController.signal.aborted) activeController.abort();
        taskPromise = null;
        stopPromise = Promise.resolve().then(() => {
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
        runNow() {
            if (!controller || phase !== 'running') return Promise.resolve(null);
            return runTask(generation, controller.signal);
        },
        get owner() {
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
            return Object.freeze({taskCount, skippedTaskCount, failureCount});
        }
    };

    return Object.freeze(publicRuntime);
}

export const createPeriodicTaskRuntime = createTaskRuntime;
export default createTaskRuntime;

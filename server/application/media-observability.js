const DEFAULT_PROGRESS_INTERVAL_MS = 1_000;
const MAX_PROGRESS_OUTPUT_LENGTH = 480;
const MAX_DEBUG_STRING_LENGTH = 2_000;
const MAX_DEBUG_ITEMS = 10;
const MAX_DEBUG_KEYS = 100;
const MAX_DEBUG_DEPTH = 8;
const MAX_ERROR_LENGTH = 240;
const MAX_OUTPUT_LINE_COUNT = 1_000_000;
const MAX_PROGRESS_STAGE_LENGTH = 80;

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function requiredFunction(value, field) {
    if (typeof value !== 'function') throw new TypeError(`Media observability requires ${field}()`);
    return value;
}

function requiredObject(value, field) {
    if (!isRecord(value)) throw new TypeError(`Media observability ${field} must be an object`);
    return value;
}

function syncValue(value, field) {
    if (value && typeof value.then === 'function') throw new TypeError(`Media observability ${field} must be synchronous`);
    return value;
}

function boundedText(value, limit, fallback = '') {
    if (value === undefined || value === null) return fallback;
    const text = String(value).replace(/\s+/g, ' ').trim();
    return text.slice(0, limit);
}

function boundedError(value) {
    const text = boundedText(value instanceof Error ? value.message : value, MAX_ERROR_LENGTH, 'Media job failed');
    return text || 'Media job failed';
}

function clockMilliseconds(value) {
    if (value instanceof Date) return value.getTime();
    if (typeof value === 'number' && Number.isFinite(value)) return value;
    if (typeof value === 'string') {
        const parsed = Date.parse(value);
        if (Number.isFinite(parsed)) return parsed;
    }
    return Date.now();
}

function boundedPercent(value) {
    const numeric = Number(value);
    if (!Number.isFinite(numeric)) return null;
    return Math.round(Math.max(0, Math.min(100, numeric)) * 100) / 100;
}

function boundedProgress(value) {
    if (!isRecord(value)) throw new TypeError('Media observability progress parser must return an object');
    const output = value.output === undefined ? undefined : boundedText(value.output, MAX_PROGRESS_OUTPUT_LENGTH);
    const percent = value.percent === undefined ? undefined : boundedPercent(value.percent);
    return {
        ...value,
        ...(output === undefined ? {} : {output}),
        ...(percent === undefined ? {} : {percent})
    };
}

function boundedPatch(value, parseProgress) {
    const patch = requiredObject(value, 'progress patch');
    const next = {...patch};
    if (next.output !== undefined) {
        const parsed = parseProgress(next.output);
        next.output = parsed.output ?? '';
        if (next.percent === undefined && parsed.percent !== undefined && parsed.percent !== null) next.percent = parsed.percent;
    }
    if (next.latestOutput !== undefined) next.latestOutput = boundedText(next.latestOutput, MAX_PROGRESS_OUTPUT_LENGTH);
    if (next.percent !== undefined) next.percent = boundedPercent(next.percent);
    if (next.stage !== undefined) next.stage = boundedText(next.stage, MAX_PROGRESS_STAGE_LENGTH);
    if (next.latestStream !== undefined && next.latestStream !== 'stdout' && next.latestStream !== 'stderr') delete next.latestStream;
    if (next.outputLineCountDelta !== undefined) {
        const count = Number(next.outputLineCountDelta);
        next.outputLineCountDelta = Number.isFinite(count)
            ? Math.max(0, Math.min(MAX_OUTPUT_LINE_COUNT, Math.floor(count)))
            : 0;
    }
    return next;
}

function boundedDebug(value, depth = 0, seen = new WeakSet()) {
    if (depth > MAX_DEBUG_DEPTH) return '[depth omitted]';
    if (typeof value === 'string') return value.slice(0, MAX_DEBUG_STRING_LENGTH);
    if (value === null || value === undefined || typeof value === 'number' || typeof value === 'boolean') return value;
    if (typeof value === 'bigint') return String(value).slice(0, MAX_DEBUG_STRING_LENGTH);
    if (typeof value !== 'object') return String(value).slice(0, MAX_DEBUG_STRING_LENGTH);
    if (seen.has(value)) return '[circular]';
    seen.add(value);
    if (Array.isArray(value)) return value.slice(0, MAX_DEBUG_ITEMS).map(item => boundedDebug(item, depth + 1, seen));
    const output = {};
    for (const [key, item] of Object.entries(value).slice(0, MAX_DEBUG_KEYS)) {
        output[String(key).slice(0, MAX_DEBUG_STRING_LENGTH)] = boundedDebug(item, depth + 1, seen);
    }
    return output;
}

function rejection() {
    return {changed: false, reason: 'lease_rejected'};
}

function activeLease(leaseGuard, job, operation) {
    const checked = syncValue(leaseGuard(job, {operation}), 'leaseGuard');
    if (!checked || checked.active === false || checked.valid === false || checked.ok === false) return null;
    if (isRecord(checked.activeJob)) return checked.activeJob;
    if (isRecord(checked.job)) return checked.job;
    return job;
}

/**
 * Build the application boundary for media execution diagnostics.
 *
 * The application layer owns coordination only. Parsing, persistence, lease
 * reads, process execution, and debug projection stay in injected adapters so
 * this module can be imported without runtime or infrastructure side effects.
 */
export function createMediaObservability({
    progressParser,
    progressWriter,
    leaseGuard,
    reporterFactory,
    settleJob,
    debugProjector,
    runProcess,
    now = () => Date.now(),
    progressIntervalMs = DEFAULT_PROGRESS_INTERVAL_MS
} = {}) {
    const parseInjected = requiredFunction(progressParser, 'progressParser');
    const write = requiredFunction(progressWriter, 'progressWriter');
    const guard = requiredFunction(leaseGuard, 'leaseGuard');
    const makeReporter = requiredFunction(reporterFactory, 'reporterFactory');
    const settleInjected = requiredFunction(settleJob, 'settleJob');
    const projectDebug = requiredFunction(debugProjector, 'debugProjector');
    const executeProcess = requiredFunction(runProcess, 'runProcess');
    const readNow = requiredFunction(now, 'now');
    const interval = Number(progressIntervalMs);
    if (!Number.isFinite(interval) || interval < 0) throw new RangeError('Media observability progressIntervalMs must be non-negative');

    function parseProgress(value) {
        return boundedProgress(syncValue(parseInjected(value), 'progressParser'));
    }

    function recordProgress(job, patch = {}) {
        requiredObject(job, 'job');
        const active = activeLease(guard, job, 'progress');
        if (!active) return rejection();
        const next = boundedPatch(patch, parseProgress);
        const result = syncValue(write(active, next, {operation: 'progress'}), 'progressWriter');
        return result === undefined ? {changed: false, progress: null, result: null} : result;
    }

    function settle(job, options = {}) {
        requiredObject(job, 'job');
        const active = activeLease(guard, job, 'settlement');
        if (!active) return rejection();
        const input = requiredObject(options, 'settlement options');
        const next = {...input};
        if (next.error !== undefined && next.error !== null) next.error = boundedError(next.error);
        if (next.terminal !== undefined) next.terminal = Boolean(next.terminal);
        if (next.progressStage !== undefined) next.progressStage = boundedText(next.progressStage, MAX_PROGRESS_STAGE_LENGTH);
        const result = syncValue(settleInjected(active, next, {operation: 'settlement'}), 'settleJob');
        return result === undefined ? {changed: false, status: null} : result;
    }

    function observeDebug(value, context = {}) {
        const projected = syncValue(projectDebug(value, context), 'debugProjector');
        return boundedDebug(projected);
    }

    function createReporter(job) {
        requiredObject(job, 'job');
        let pending = null;
        let pendingOutputCount = 0;
        let lastWriteAt = Number.NEGATIVE_INFINITY;

        const flush = (force = true) => {
            if (!pending) return {changed: true, progress: null};
            const patch = {...pending};
            if (pendingOutputCount) patch.outputLineCountDelta = (Number(patch.outputLineCountDelta) || 0) + pendingOutputCount;
            pending = null;
            pendingOutputCount = 0;
            const result = recordProgress(job, patch);
            if (result?.reason === 'lease_rejected') return result;
            lastWriteAt = clockMilliseconds(readNow());
            return result;
        };

        const report = (patch = {}, {force = false} = {}) => {
            const next = requiredObject(patch, 'report patch');
            pending = {...(pending || {}), ...next};
            const current = clockMilliseconds(readNow());
            if (!force && current - lastWriteAt < interval) return {changed: true, throttled: true};
            return flush(true);
        };

        report.stage = stage => report({stage}, {force: true});
        report.output = (stream, output) => {
            pendingOutputCount += 1;
            return report({output, latestStream: stream}, {force: false});
        };
        report.flush = () => flush(true);

        const helpers = Object.freeze({
            ...job,
            job,
            parseProgress,
            recordProgress: patch => recordProgress(job, patch),
            writeProgress: patch => recordProgress(job, patch),
            report,
            settle: options => settle(job, options),
            runProcess: executeProcess
        });
        const created = makeReporter.length >= 2
            ? makeReporter(job, helpers)
            : makeReporter(helpers);
        return created === undefined || created === null ? report : created;
    }

    function invokeProcess(...args) {
        return executeProcess(...args);
    }

    return Object.freeze({
        parseProgress,
        createReporter,
        recordProgress,
        settle,
        observeDebug,
        runProcess: invokeProcess
    });
}

export default createMediaObservability;

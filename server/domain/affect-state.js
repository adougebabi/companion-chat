/**
 * Pure PAD/drives state rules.
 *
 * PAD is kept in the conventional [-1, 1] range. Drives are unmet-need
 * pressures in [0, 1], so a negative event delta satisfies a drive and a
 * positive delta accumulates pressure. Persistence and provider concerns stay
 * out of this module.
 */

export const AFFECT_MODEL_VERSION = 'affect.v1';
export const PAD_AXES = Object.freeze(['pleasure', 'arousal', 'dominance']);
export const DRIVE_KEYS = Object.freeze(['social', 'exploration', 'rest']);
export const SUPPORTED_DRIVES = DRIVE_KEYS;

export const PAD_RANGE = Object.freeze({min: -1, max: 1});
export const PRESSURE_RANGE = Object.freeze({min: 0, max: 1});
export const DEFAULT_PAD_BASELINE = Object.freeze({pleasure: 0, arousal: 0, dominance: 0});
export const DEFAULT_DRIVE_BASELINE = Object.freeze({social: 0.5, exploration: 0.5, rest: 0.5});
export const DEFAULT_HALF_LIFE_MS = 24 * 60 * 60 * 1000;

const DEFAULT_DRIVE_WEIGHT = 1;
const MAX_EVENT_DELTA = 0.35;
const MAX_POLICY_HALF_LIFE_MS = 365 * 24 * 60 * 60 * 1000;

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function clone(value) {
    if (Array.isArray(value)) return value.map(clone);
    if (isRecord(value)) return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, clone(item)]));
    return value;
}

function numberOr(value, fallback) {
    const number = typeof value === 'number' ? value : Number(value);
    return Number.isFinite(number) ? number : fallback;
}

export function clamp(value, range = PAD_RANGE) {
    const number = numberOr(value, range.min);
    return Math.min(range.max, Math.max(range.min, number));
}

function boundedDelta(value) {
    return clamp(value, {min: -MAX_EVENT_DELTA, max: MAX_EVENT_DELTA});
}

function timeMs(value, field = 'Affect time') {
    const date = value instanceof Date ? value : new Date(value);
    if (!Number.isFinite(date.getTime())) throw new TypeError(`${field} must be a valid timestamp`);
    return date.getTime();
}

function isoTime(value, field = 'Affect time') {
    return new Date(timeMs(value, field)).toISOString();
}

function positiveHalfLife(value, fallback) {
    if (isRecord(value)) {
        const nested = value.ms ?? value.milliseconds ?? value.millis ?? value.seconds;
        if (nested !== undefined) {
            const multiplier = value.seconds !== undefined && value.ms === undefined && value.milliseconds === undefined && value.millis === undefined ? 1000 : 1;
            return Math.min(MAX_POLICY_HALF_LIFE_MS, Math.max(1, numberOr(nested, fallback) * multiplier));
        }
        const hours = value.hours;
        if (hours !== undefined) return Math.min(MAX_POLICY_HALF_LIFE_MS, Math.max(1, numberOr(hours, fallback / 3_600_000) * 3_600_000));
    }
    if (typeof value === 'string' && value.trim()) {
        const match = /^([0-9]+(?:\.[0-9]+)?)(ms|s|m|h|d)$/i.exec(value.trim());
        if (match) {
            const multiplier = {ms: 1, s: 1000, m: 60_000, h: 3_600_000, d: 86_400_000}[match[2].toLowerCase()];
            return Math.min(MAX_POLICY_HALF_LIFE_MS, Math.max(1, Number(match[1]) * multiplier));
        }
    }
    return Math.min(MAX_POLICY_HALF_LIFE_MS, Math.max(1, numberOr(value, fallback)));
}

function axisValue(value, fallback) {
    return value === undefined || value === null || value === '' ? fallback : clamp(value, PAD_RANGE);
}

function baselineFrom(source, fallback) {
    const value = isRecord(source) ? source : {};
    return Object.fromEntries(PAD_AXES.map(axis => [axis, axisValue(value[axis], fallback[axis])]));
}

function halfLifeFrom(source, fallback) {
    const value = isRecord(source) ? source : {};
    return Object.fromEntries(PAD_AXES.map(axis => [axis, positiveHalfLife(value[axis], fallback)]));
}

function driveConfig(source, key) {
    const value = isRecord(source?.[key]) ? source[key] : {};
    return {
        baseline: clamp(numberOr(value.baseline, DEFAULT_DRIVE_BASELINE[key]), PRESSURE_RANGE),
        weight: clamp(numberOr(value.weight, DEFAULT_DRIVE_WEIGHT), {min: 0, max: 1}),
        halfLife: positiveHalfLife(value.halfLife ?? value.halfLifeMs, DEFAULT_HALF_LIFE_MS)
    };
}

function policySource(input) {
    if (!isRecord(input)) return {};
    if (isRecord(input.affectPolicy)) return input.affectPolicy;
    if (isRecord(input.affect_policy)) return input.affect_policy;
    return input;
}

/**
 * Normalize the optional life-blueprint policy into one deterministic shape.
 * Numeric half-life values are milliseconds; explicit `seconds`, `hours`, and
 * unit-suffixed strings are accepted for blueprint ergonomics.
 */
export function normalizeAffectPolicy(input = {}) {
    const source = policySource(input);
    const baseline = baselineFrom(source.baseline ?? source.padBaseline ?? source.pad?.baseline, DEFAULT_PAD_BASELINE);
    const halfLife = halfLifeFrom(source.halfLife ?? source.padHalfLife ?? source.pad?.halfLife, DEFAULT_HALF_LIFE_MS);
    const configuredDrives = isRecord(source.drives) ? source.drives : {};
    const drives = Object.fromEntries(DRIVE_KEYS.map(key => [key, driveConfig(configuredDrives, key)]));
    const futureDrives = {
        ...(isRecord(source.futureDrives) ? clone(source.futureDrives) : {}),
        ...Object.fromEntries(Object.entries(configuredDrives)
            .filter(([key]) => !DRIVE_KEYS.includes(key))
            .map(([key, value]) => [key, clone(value)]))
    };
    return {
        modelVersion: typeof source.modelVersion === 'string' && source.modelVersion.trim() ? source.modelVersion.trim() : AFFECT_MODEL_VERSION,
        baseline,
        halfLife,
        drives,
        futureDrives
    };
}

function drivesFrom(value) {
    if (isRecord(value)) return clone(value);
    if (typeof value === 'string' && value.trim()) {
        try {
            const parsed = JSON.parse(value);
            return isRecord(parsed) ? parsed : {};
        } catch {
            return {};
        }
    }
    return {};
}

function stateValue(input = {}, key, fallback) {
    const value = input[key] ?? input.pad?.[key] ?? input[`pad_${key}`];
    return axisValue(value, fallback);
}

/** Convert a row-shaped or domain-shaped snapshot to the canonical state. */
export function normalizeAffectState(input = {}, {personaId, at, policy} = {}) {
    const normalizedPolicy = normalizeAffectPolicy(policy ?? input.policy ?? {});
    const owner = personaId ?? input.personaId ?? input.persona_id ?? null;
    const effectiveAt = input.effectiveAt ?? input.effective_at ?? at ?? new Date().toISOString();
    const drives = drivesFrom(input.drives ?? input.drives_json);
    for (const key of DRIVE_KEYS) {
        drives[key] = drives[key] === undefined || drives[key] === null || drives[key] === ''
            ? normalizedPolicy.drives[key].baseline
            : clamp(drives[key], PRESSURE_RANGE);
    }
    return {
        personaId: owner,
        pleasure: stateValue(input, 'pleasure', normalizedPolicy.baseline.pleasure),
        arousal: stateValue(input, 'arousal', normalizedPolicy.baseline.arousal),
        dominance: stateValue(input, 'dominance', normalizedPolicy.baseline.dominance),
        drives,
        revision: Math.max(0, Number.isSafeInteger(Number(input.revision)) ? Number(input.revision) : 0),
        effectiveAt: isoTime(effectiveAt, 'Affect state.effectiveAt'),
        updatedAt: input.updatedAt ?? input.updated_at ?? null,
        sourceEventId: input.sourceEventId ?? input.source_event_id ?? null,
        modelVersion: input.modelVersion ?? input.model_version ?? normalizedPolicy.modelVersion,
        pad: {
            pleasure: stateValue(input, 'pleasure', normalizedPolicy.baseline.pleasure),
            arousal: stateValue(input, 'arousal', normalizedPolicy.baseline.arousal),
            dominance: stateValue(input, 'dominance', normalizedPolicy.baseline.dominance)
        }
    };
}

export function createInitialAffectState({personaId = null, at = new Date().toISOString(), policy = {}} = {}) {
    const normalizedPolicy = normalizeAffectPolicy(policy);
    return normalizeAffectState({
        personaId,
        pleasure: normalizedPolicy.baseline.pleasure,
        arousal: normalizedPolicy.baseline.arousal,
        dominance: normalizedPolicy.baseline.dominance,
        drives: {
            ...Object.fromEntries(DRIVE_KEYS.map(key => [key, normalizedPolicy.drives[key].baseline])),
            ...Object.fromEntries(Object.entries(normalizedPolicy.futureDrives).map(([key, value]) => [
                key,
                isRecord(value) ? clamp(value.baseline, PRESSURE_RANGE) : value
            ]))
        },
        revision: 0,
        effectiveAt: at,
        modelVersion: normalizedPolicy.modelVersion
    }, {policy: normalizedPolicy});
}

function decayValue(value, baseline, halfLife, elapsedMs, range) {
    if (elapsedMs <= 0) return clamp(value, range);
    const factor = Math.pow(0.5, elapsedMs / halfLife);
    return clamp(baseline + (value - baseline) * factor, range);
}

/** Apply lazy exponential decay without changing revision or persistence. */
export function decayAffectState(input, {at, policy = {}} = {}) {
    const normalizedPolicy = normalizeAffectPolicy(policy);
    const state = normalizeAffectState(input, {policy: normalizedPolicy, at: at ?? input?.effectiveAt ?? input?.effective_at});
    const requestedAt = isoTime(at ?? state.effectiveAt, 'Affect decay.at');
    const effectiveAt = Math.max(timeMs(requestedAt), timeMs(state.effectiveAt));
    const elapsedMs = Math.max(0, effectiveAt - timeMs(state.effectiveAt));
    const next = {...state, effectiveAt: new Date(effectiveAt).toISOString()};
    for (const axis of PAD_AXES) {
        next[axis] = decayValue(state[axis], normalizedPolicy.baseline[axis], normalizedPolicy.halfLife[axis], elapsedMs, PAD_RANGE);
        next.pad[axis] = next[axis];
    }
    for (const key of DRIVE_KEYS) {
        const value = state.drives[key] ?? normalizedPolicy.drives[key].baseline;
        next.drives[key] = decayValue(value, normalizedPolicy.drives[key].baseline, normalizedPolicy.drives[key].halfLife, elapsedMs, PRESSURE_RANGE);
    }
    return next;
}

const EVENT_DELTAS = Object.freeze({
    social_connection: Object.freeze({pleasure: 0.12, arousal: 0.04, dominance: 0.02, drives: {social: -0.25}}),
    social_friction: Object.freeze({pleasure: -0.12, arousal: 0.06, dominance: -0.04, drives: {social: 0.22}}),
    exploration_discovery: Object.freeze({pleasure: 0.1, arousal: 0.1, dominance: 0.03, drives: {exploration: -0.25}}),
    exploration_blocked: Object.freeze({pleasure: -0.04, arousal: -0.08, dominance: -0.03, drives: {exploration: 0.2}}),
    restored: Object.freeze({pleasure: 0.08, arousal: -0.18, dominance: 0.03, drives: {rest: -0.3}}),
    fatigue: Object.freeze({pleasure: -0.08, arousal: 0.16, dominance: -0.05, drives: {rest: 0.24}})
});

const EVENT_ALIASES = Object.freeze({
    connection: 'social_connection', conversation: 'social_connection', social: 'social_connection',
    rejection: 'social_friction', social_rejection: 'social_friction',
    discovery: 'exploration_discovery', exploration: 'exploration_discovery', curiosity: 'exploration_discovery',
    stagnation: 'exploration_blocked', exploration_stagnation: 'exploration_blocked',
    rest: 'restored', sleep: 'restored', recovery: 'restored', rest_completed: 'restored',
    overwork: 'fatigue', rest_deprived: 'fatigue',
    positive: 'social_connection', joy: 'social_connection', negative: 'social_friction', stress: 'social_friction'
});

export const AFFECT_EVENT_TYPES = Object.freeze(Object.keys(EVENT_DELTAS));
export const DRIVE_SIGNAL_DIRECTIONS = Object.freeze(['increase_pressure', 'decrease_pressure', 'neutral']);

function eventType(input) {
    const value = input?.eventType ?? input?.event_type ?? input?.type;
    if (typeof value !== 'string' || !value.trim()) throw new TypeError('Affect event.type must be non-empty');
    const normalized = value.trim().toLowerCase();
    return EVENT_DELTAS[normalized] ? normalized : EVENT_ALIASES[normalized] ?? null;
}

/** Map an allowlisted event type to bounded server-owned deltas. */
export function reduceAffectEvent(input = {}) {
    if (!isRecord(input)) throw new TypeError('Affect event must be an object');
    if (Object.hasOwn(input, 'delta') || Object.hasOwn(input, 'padDelta') || Object.hasOwn(input, 'drivesDelta') || Object.hasOwn(input, 'pleasureDelta')) {
        throw new TypeError('Affect event deltas are server-owned');
    }
    const canonicalType = eventType(input);
    if (!canonicalType) throw new RangeError(`Unsupported affect event type: ${String(input.type ?? input.eventType)}`);
    const source = EVENT_DELTAS[canonicalType];
    return {
        eventType: canonicalType,
        pleasureDelta: boundedDelta(source.pleasure ?? 0),
        arousalDelta: boundedDelta(source.arousal ?? 0),
        dominanceDelta: boundedDelta(source.dominance ?? 0),
        drivesDelta: Object.fromEntries(Object.entries(source.drives ?? {}).map(([key, value]) => [key, boundedDelta(value)]))
    };
}

/**
 * Map a model drive signal to a small pressure delta. Unknown future keys are
 * retained in the returned event payload but deliberately do not affect the
 * materialized state until a registered policy exists.
 */
export function reduceDriveSignal(input = {}) {
    if (!isRecord(input)) throw new TypeError('Drive signal must be an object');
    const drive = input.drive;
    if (typeof drive !== 'string' || !/^[a-z][a-z0-9_:-]{0,79}$/.test(drive)) throw new TypeError('Drive signal drive is not supported');
    const direction = input.direction;
    if (!DRIVE_SIGNAL_DIRECTIONS.includes(direction)) throw new RangeError(`Unsupported drive signal direction: ${String(direction)}`);
    const delta = direction === 'increase_pressure' ? 0.18 : direction === 'decrease_pressure' ? -0.18 : 0;
    return {
        eventType: 'drive_signal',
        drive,
        direction,
        recognized: DRIVE_KEYS.includes(drive),
        pleasureDelta: 0,
        arousalDelta: 0,
        dominanceDelta: 0,
        drivesDelta: {[drive]: boundedDelta(delta)}
    };
}

/** Decay, reduce, clamp, and advance revision for one accepted event. */
export function applyAffectEvent(input, event, {at, policy = {}, delta} = {}) {
    const normalizedPolicy = normalizeAffectPolicy(policy);
    const current = decayAffectState(input, {at: at ?? event?.effectiveAt ?? event?.effective_at, policy: normalizedPolicy});
    const reduction = delta ?? reduceAffectEvent(event);
    const eventAt = isoTime(event.effectiveAt ?? event.effective_at ?? at ?? current.effectiveAt, 'Affect event.effectiveAt');
    const effectiveAt = new Date(Math.max(timeMs(eventAt), timeMs(current.effectiveAt))).toISOString();
    const next = {...current, effectiveAt, revision: current.revision + 1, sourceEventId: event.id ?? event.eventId ?? null, modelVersion: normalizedPolicy.modelVersion};
    next.pleasure = clamp(current.pleasure + reduction.pleasureDelta, PAD_RANGE);
    next.arousal = clamp(current.arousal + reduction.arousalDelta, PAD_RANGE);
    next.dominance = clamp(current.dominance + reduction.dominanceDelta, PAD_RANGE);
    next.pad = {pleasure: next.pleasure, arousal: next.arousal, dominance: next.dominance};
    next.drives = {...current.drives};
    for (const [key, delta] of Object.entries(reduction.drivesDelta)) {
        if (DRIVE_KEYS.includes(key)) next.drives[key] = clamp((current.drives[key] ?? normalizedPolicy.drives[key].baseline) + delta, PRESSURE_RANGE);
    }
    return {state: next, delta: reduction};
}

export const decayState = decayAffectState;
export const initialAffectState = createInitialAffectState;
export const reduceEvent = reduceAffectEvent;
export const applyEvent = applyAffectEvent;

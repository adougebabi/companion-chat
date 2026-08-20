import {CAPABILITY_NAMES} from '../contracts/index.js';

export const CAPABILITY_EFFECT_PLAN_VERSION = 1;
export const CAPABILITY_EFFECT_STATUSES = Object.freeze(['preview', 'committed', 'failed']);

const CAPABILITY_SET = new Set(CAPABILITY_NAMES);
const CAPABILITY_ORDER = new Map(CAPABILITY_NAMES.map((name, index) => [name, index]));
const SOURCE_NAMES = new Set(['native', 'marker']);
const MAX_ENTRIES = 16;
const MAX_ORDER = 1_000_000;
const MAX_ID_LENGTH = 240;
const MAX_CALL_ID_LENGTH = 160;
const MAX_ARGUMENTS_TEXT_LENGTH = 12_000;
const MAX_PREVIEW_TEXT_LENGTH = 4_000;
const MAX_PREVIEW_DEPTH = 5;
const MAX_PREVIEW_KEYS = 64;
const MAX_PREVIEW_ITEMS = 32;
const MAX_PROJECTION_TEXT_LENGTH = 480;
const MAX_PROJECTION_NODES = 160;
const MAX_ERROR_LENGTH = 240;

const PRIVATE_ENTRY = new WeakMap();
const PRIVATE_PLAN = new WeakMap();

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function boundedText(value, field, maxLength = MAX_ID_LENGTH, {allowEmpty = false} = {}) {
    if (typeof value !== 'string') throw new TypeError(`${field} must be a string`);
    const text = value.trim();
    if (!allowEmpty && !text) throw new TypeError(`${field} must not be empty`);
    if (text.length > maxLength) throw new RangeError(`${field} exceeds ${maxLength} characters`);
    return text;
}

function nullableText(value, field, maxLength) {
    if (value === undefined || value === null) return null;
    return boundedText(value, field, maxLength);
}

function assertCapability(value) {
    if (!CAPABILITY_SET.has(value)) throw new TypeError(`Unknown capability: ${String(value)}`);
    return value;
}

function assertSource(value) {
    if (!SOURCE_NAMES.has(value)) throw new TypeError(`Unsupported capability source: ${String(value)}`);
    return value;
}

function assertOrder(value, field) {
    if (!Number.isInteger(value) || value < 0 || value > MAX_ORDER) {
        throw new TypeError(`${field} must be a non-negative integer`);
    }
    return value;
}

function pick(...values) {
    return values.find(value => value !== undefined && value !== null);
}

function rejectUnsafeKey(key, field) {
    if (key === '__proto__' || key === 'prototype' || key === 'constructor') {
        throw new TypeError(`${field} contains an unsafe key`);
    }
}

function normalizeBoundedValue(value, field, depth = 0) {
    if (value === null || value === undefined) return null;
    if (depth > MAX_PREVIEW_DEPTH) throw new RangeError(`${field} exceeds nesting depth`);
    if (typeof value === 'string') {
        if (value.length > MAX_PREVIEW_TEXT_LENGTH) throw new RangeError(`${field} contains oversized text`);
        return value;
    }
    if (typeof value === 'boolean') return value;
    if (typeof value === 'number') {
        if (!Number.isFinite(value)) throw new TypeError(`${field} contains a non-finite number`);
        return value;
    }
    if (Array.isArray(value)) {
        if (value.length > MAX_PREVIEW_ITEMS) throw new RangeError(`${field} contains too many items`);
        return Object.freeze(value.map((item, index) => normalizeBoundedValue(item, `${field}[${index}]`, depth + 1)));
    }
    if (!isRecord(value)) throw new TypeError(`${field} must contain JSON-compatible values`);
    const keys = Object.keys(value).sort();
    if (keys.length > MAX_PREVIEW_KEYS) throw new RangeError(`${field} contains too many keys`);
    const normalized = {};
    for (const key of keys) {
        rejectUnsafeKey(key, field);
        normalized[key] = normalizeBoundedValue(value[key], `${field}.${key}`, depth + 1);
    }
    return Object.freeze(normalized);
}

function normalizeArguments(value, field) {
    if (value === undefined || value === null) return null;
    return normalizeBoundedValue(value, field);
}

function normalizePreallocatedIds(value, field) {
    if (value === undefined || value === null) return Object.freeze({});
    if (!isRecord(value)) throw new TypeError(`${field} must be an object`);
    const keys = Object.keys(value).sort();
    if (keys.length > 16) throw new RangeError(`${field} contains too many IDs`);
    const normalized = {};
    for (const key of keys) {
        rejectUnsafeKey(key, field);
        if (!/^[A-Za-z][A-Za-z0-9_]{0,47}Id$/.test(key)) {
            throw new TypeError(`${field}.${key} is not a supported ID field`);
        }
        normalized[key] = boundedText(value[key], `${field}.${key}`);
    }
    return Object.freeze(normalized);
}

function normalizeError(error) {
    const raw = error instanceof Error ? error.message : String(error);
    const compact = raw.replace(/\s+/g, ' ').trim() || 'Capability effect failed';
    const redacted = compact
        .replace(/((?:api[_-]?key|authorization|bearer|password|secret|token|credential)[^:=]{0,24})[:=]\s*[^\s,;]+/gi, '$1=[redacted]')
        .replace(/\bhttps?:\/\/[^\s]+/gi, '[provider-url]');
    return redacted.length <= MAX_ERROR_LENGTH ? redacted : `${redacted.slice(0, MAX_ERROR_LENGTH - 3)}...`;
}

function sensitiveProjectionKey(key) {
    return /argument|raw|dedupe|idempot|secret|token|password|authorization|credential|provider|callid|causation|prompt|cookie|header|config/i.test(key);
}

function projectBoundedValue(value, context = {nodes: 0}, depth = 0) {
    if (context.nodes++ >= MAX_PROJECTION_NODES || depth > MAX_PREVIEW_DEPTH) return null;
    if (value === null || value === undefined) return null;
    if (typeof value === 'string') {
        if (value.length <= MAX_PROJECTION_TEXT_LENGTH) return value;
        return `${value.slice(0, MAX_PROJECTION_TEXT_LENGTH - 3)}...`;
    }
    if (typeof value === 'boolean' || typeof value === 'number') return value;
    if (Array.isArray(value)) {
        return value.slice(0, MAX_PREVIEW_ITEMS).map(item => projectBoundedValue(item, context, depth + 1));
    }
    if (!isRecord(value)) return null;
    const result = {};
    for (const key of Object.keys(value).sort()) {
        if (sensitiveProjectionKey(key)) continue;
        const projected = projectBoundedValue(value[key], context, depth + 1);
        if (projected !== null || value[key] === null) result[key] = projected;
    }
    return result;
}

function resolveHandler(value) {
    const apply = value.apply;
    const handler = value.handler;
    if (apply !== undefined && typeof apply !== 'function') throw new TypeError('Capability effect apply must be a function');
    if (handler !== undefined && typeof handler !== 'function' && !(isRecord(handler) && typeof handler.apply === 'function')) {
        throw new TypeError('Capability effect handler must be a function or provide apply()');
    }
    if (typeof apply === 'function' && typeof handler === 'function' && apply !== handler) {
        throw new TypeError('Capability effect cannot define conflicting apply and handler callbacks');
    }
    if (typeof apply === 'function') return apply;
    if (typeof handler === 'function') return handler;
    if (isRecord(handler)) return handler.apply.bind(handler);
    return null;
}

function normalizeProvenance(value, fallbackOrder) {
    const call = isRecord(value.call) ? value.call : {};
    const declared = isRecord(value.provenance) ? value.provenance : {};
    const causation = pick(value.causation, declared.causation, call.causation);
    const causationId = pick(
        value.causationId,
        declared.causationId,
        call.causationId,
        typeof causation === 'string' ? causation : causation?.id,
        value.causationUserMessageId,
        declared.causationUserMessageId,
        call.causationUserMessageId
    );
    const normalizedCapability = assertCapability(pick(value.capability, value.name));
    const order = pick(
        value.order,
        value.index,
        declared.order,
        declared.index,
        call.order,
        call.index,
        CAPABILITY_ORDER.get(normalizedCapability),
        fallbackOrder
    );
    const source = assertSource(pick(value.source, declared.source, call.source, 'native'));
    const callId = nullableText(pick(value.callId, declared.callId, call.callId, call.id), 'capability callId', MAX_CALL_ID_LENGTH);
    const idempotencyKey = boundedText(
        pick(value.idempotencyKey, declared.idempotencyKey, call.idempotencyKey),
        'capability idempotencyKey'
    );
    const personaId = nullableText(pick(value.personaId, declared.personaId, call.personaId), 'capability personaId');
    const causationUserMessageId = nullableText(
        pick(value.causationUserMessageId, declared.causationUserMessageId, call.causationUserMessageId),
        'capability causationUserMessageId'
    );
    return {
        capability: normalizedCapability,
        order: assertOrder(order, 'capability order'),
        callId,
        idempotencyKey,
        causationId: boundedText(causationId, 'capability causationId'),
        causationUserMessageId,
        source,
        personaId
    };
}

function normalizePreviewResult(value) {
    return normalizeBoundedValue(value === undefined ? null : value, 'capability previewResult');
}

function previewIdsFrom(value, previewResult) {
    const fromPreview = isRecord(previewResult) ? pick(previewResult.preallocatedIds, previewResult.ids) : undefined;
    return normalizePreallocatedIds(pick(value.preallocatedIds, value.ids, fromPreview), 'capability preallocatedIds');
}

function normalizeEntry(value, fallbackOrder) {
    if (!isRecord(value)) throw new TypeError('Capability effect entry must be an object');
    const provenance = normalizeProvenance(value, fallbackOrder);
    const previous = PRIVATE_ENTRY.get(value);
    const status = previous?.state.status ?? value.status ?? 'preview';
    if (!CAPABILITY_EFFECT_STATUSES.includes(status)) throw new TypeError(`Unsupported capability effect status: ${String(status)}`);
    const previewResult = normalizePreviewResult(pick(value.previewResult, value.preview, value.result));
    const entryId = boundedText(
        pick(value.entryId, value.effectId, value.id, `${provenance.capability}:${provenance.order}`),
        'capability entryId'
    );
    const privateDetails = {
        handler: resolveHandler(value) ?? previous?.privateDetails.handler ?? null,
        arguments: normalizeArguments(pick(value.arguments, value.call?.arguments, previous?.privateDetails.arguments), 'capability arguments'),
        argumentsText: value.argumentsText === undefined
            ? previous?.privateDetails.argumentsText ?? null
            : boundedText(value.argumentsText, 'capability argumentsText', MAX_ARGUMENTS_TEXT_LENGTH, {allowEmpty: true})
    };
    const state = {
        status,
        error: previous?.state.error ?? (value.error === undefined || value.error === null ? null : normalizeError(value.error)),
        previewResult,
        preallocatedIds: previewIdsFrom(value, previewResult)
    };
    const entry = {
        entryId,
        order: provenance.order,
        capability: provenance.capability,
        provenance: Object.freeze(provenance),
        get status() {
            return state.status;
        },
        get previewResult() {
            return state.previewResult;
        },
        get preallocatedIds() {
            return state.preallocatedIds;
        },
        get error() {
            return state.error;
        }
    };
    Object.freeze(entry);
    PRIVATE_ENTRY.set(entry, {state, privateDetails, source: value});
    return entry;
}

function entryResult(entry) {
    const privateEntry = PRIVATE_ENTRY.get(entry);
    return {
        order: entry.order,
        capability: entry.capability,
        status: entry.status,
        ok: entry.status !== 'failed',
        result: projectBoundedValue(entry.previewResult),
        preallocatedIds: projectBoundedValue(entry.preallocatedIds),
        ...(privateEntry?.state.error ? {error: privateEntry.state.error} : {})
    };
}

function planStatus(entries) {
    if (entries.some(entry => entry.status === 'failed')) return 'failed';
    if (entries.length > 0 && entries.every(entry => entry.status === 'committed')) return 'committed';
    return 'preview';
}

function assertPlan(value) {
    if (!isRecord(value) || !PRIVATE_PLAN.has(value)) throw new TypeError('Capability effect plan is invalid');
    return PRIVATE_PLAN.get(value);
}

function serializedEntry(entry) {
    const privateEntry = PRIVATE_ENTRY.get(entry);
    return {
        entryId: entry.entryId,
        order: entry.order,
        capability: entry.capability,
        status: entry.status,
        provenance: entry.provenance,
        previewResult: entry.previewResult,
        preallocatedIds: entry.preallocatedIds,
        ...(privateEntry?.state.error ? {error: privateEntry.state.error} : {})
    };
}

function projection(planState) {
    const results = planState.entries.map(entryResult);
    const value = {version: CAPABILITY_EFFECT_PLAN_VERSION, status: planStatus(planState.entries), results};
    Object.defineProperties(value, {
        entries: {value: results, enumerable: false},
        items: {value: results, enumerable: false},
        effects: {value: results, enumerable: false}
    });
    return Object.freeze(value);
}

async function commitPlan(plan, options = {}) {
    const state = assertPlan(plan);
    if (state.entries.length === 0) return projection(state);
    if (planStatus(state.entries) === 'failed') throw new Error('Cannot commit a failed capability effect plan');
    if (planStatus(state.entries) === 'committed') return projection(state);
    if (options !== undefined && (!isRecord(options))) throw new TypeError('Capability effect commit options must be an object');

    for (const entry of state.entries) {
        if (entry.status === 'committed') continue;
        const privateEntry = PRIVATE_ENTRY.get(entry);
        if (typeof privateEntry.privateDetails.handler !== 'function') {
            privateEntry.state.status = 'failed';
            privateEntry.state.error = 'Capability effect has no apply handler';
            break;
        }
        try {
            const outcome = await privateEntry.privateDetails.handler({
                entry,
                entryId: entry.entryId,
                order: entry.order,
                capability: entry.capability,
                provenance: entry.provenance,
                previewResult: entry.previewResult,
                preallocatedIds: entry.preallocatedIds,
                arguments: privateEntry.privateDetails.arguments,
                argumentsText: privateEntry.privateDetails.argumentsText,
                ...options
            });
            if (outcome !== undefined) {
                const returnedResult = isRecord(outcome) && ('result' in outcome || 'previewResult' in outcome)
                    ? pick(outcome.result, outcome.previewResult)
                    : outcome;
                const nextResult = normalizePreviewResult(returnedResult);
                const returnedIds = isRecord(outcome)
                    ? pick(outcome.preallocatedIds, outcome.ids, isRecord(returnedResult) ? pick(returnedResult.preallocatedIds, returnedResult.ids) : undefined)
                    : undefined;
                privateEntry.state.previewResult = nextResult;
                if (returnedIds !== undefined) {
                    privateEntry.state.preallocatedIds = normalizePreallocatedIds(returnedIds, 'capability preallocatedIds');
                }
            }
            privateEntry.state.status = 'committed';
        } catch (error) {
            privateEntry.state.status = 'failed';
            privateEntry.state.error = normalizeError(error);
            break;
        }
    }
    return projection(state);
}

function failPlan(plan, error, entryId = null) {
    const state = assertPlan(plan);
    const normalizedError = normalizeError(error);
    const entries = entryId === null
        ? state.entries
        : state.entries.filter(entry => entry.entryId === boundedText(entryId, 'capability entryId'));
    if (entries.length === 0) throw new Error('Capability effect entry was not found');
    for (const entry of entries) {
        if (entry.status === 'committed') throw new Error('A committed capability effect cannot be failed');
        const privateEntry = PRIVATE_ENTRY.get(entry);
        privateEntry.state.status = 'failed';
        privateEntry.state.error = normalizedError;
    }
    return projection(state);
}

export function normalizeCapabilityEffectEntry(value, fallbackOrder = 0) {
    return normalizeEntry(value, fallbackOrder);
}

export function createCapabilityEffectPlan(options = {}) {
    if (!Array.isArray(options) && !isRecord(options)) throw new TypeError('Capability effect plan options must be an object or entries array');
    const entries = Array.isArray(options) ? options : options?.entries ?? [];
    if (!Array.isArray(entries)) throw new TypeError('Capability effect plan entries must be an array');
    if (entries.length > MAX_ENTRIES) throw new RangeError(`Capability effect plan cannot contain more than ${MAX_ENTRIES} entries`);
    const normalized = entries.map((entry, index) => normalizeEntry(entry, index));
    normalized.sort((left, right) => left.order - right.order || left.capability.localeCompare(right.capability) || left.entryId.localeCompare(right.entryId));

    const seen = new Map();
    for (const entry of normalized) {
        for (const key of [
            ['capability', entry.capability],
            ['order', String(entry.order)],
            ['entryId', entry.entryId],
            ['idempotencyKey', entry.provenance.idempotencyKey],
            ...(entry.provenance.callId ? [['callId', entry.provenance.callId]] : [])
        ]) {
            const [kind, value] = key;
            if (seen.has(`${kind}:${value}`)) throw new Error(`Duplicate capability effect ${kind}: ${value}`);
            seen.set(`${kind}:${value}`, entry.entryId);
        }
    }

    const state = {entries: Object.freeze(normalized.slice())};
    const plan = {
        version: CAPABILITY_EFFECT_PLAN_VERSION,
        entries: state.entries,
        get status() {
            return planStatus(state.entries);
        },
        resultProjection() {
            return projection(state);
        },
        toResultProjection() {
            return projection(state);
        },
        serialize() {
            return {
                version: CAPABILITY_EFFECT_PLAN_VERSION,
                status: planStatus(state.entries),
                entries: state.entries.map(serializedEntry)
            };
        },
        async commit(options = {}) {
            return commitPlan(plan, options);
        },
        async apply(options = {}) {
            return commitPlan(plan, options);
        },
        fail(error, entryId = null) {
            return failPlan(plan, error, entryId);
        },
        toJSON() {
            return plan.serialize();
        }
    };
    PRIVATE_PLAN.set(plan, state);
    return Object.freeze(plan);
}

export function normalizeCapabilityEffectPlan(value) {
    if (value && PRIVATE_PLAN.has(value)) return value;
    if (Array.isArray(value)) return createCapabilityEffectPlan(value);
    if (!isRecord(value)) throw new TypeError('Capability effect plan must be an object');
    return createCapabilityEffectPlan({entries: value.entries});
}

export function projectCapabilityEffectResult(value) {
    if (PRIVATE_ENTRY.has(value)) return entryResult(value);
    const entry = normalizeEntry(value, 0);
    return entryResult(entry);
}

export function projectCapabilityEffectPlan(value) {
    return projection(assertPlan(value));
}

export function serializeCapabilityEffectPlan(value) {
    const state = assertPlan(value);
    return {
        version: CAPABILITY_EFFECT_PLAN_VERSION,
        status: planStatus(state.entries),
        entries: state.entries.map(serializedEntry)
    };
}

/**
 * Stable contracts shared by the future backend composition root.
 *
 * This module is deliberately independent from Express, SQLite, providers,
 * and the current server.js entrypoint. The runtime checks keep JavaScript
 * callers honest until the backend is migrated to TypeScript.
 */

export const BACKEND_CONTRACT_VERSION = 1;

export const CAPABILITY_NAMES = Object.freeze([
    'scene_event',
    'media_event',
    'pending_event'
]);

export const CAPABILITY_SOURCES = Object.freeze(['native', 'marker']);
export const SSE_EVENT_TYPES = Object.freeze(['token', 'done', 'error']);

const FLOW_LAYERS = Object.freeze([
    'composition',
    'transport',
    'runtime',
    'application',
    'domain',
    'contracts',
    'infrastructure'
]);

const REQUIRED_PORT_METHODS = Object.freeze({
    clock: ['now'],
    idGenerator: ['next'],
    llm: ['complete', 'stream'],
    conversationRepository: ['appendMessage', 'listMessages'],
    identityRepository: ['findById'],
    memoryRepository: ['listActive'],
    lifeEventRepository: ['record'],
    scheduleRepository: ['list'],
    presenceRepository: ['read'],
    activityRepository: ['publish'],
    effectRepository: ['record', 'settle'],
    mediaProvider: ['submit'],
    assetRepository: ['find']
});

const ALLOWED_DEPENDENCY_LAYERS = Object.freeze({
    composition: FLOW_LAYERS,
    transport: ['application', 'contracts'],
    runtime: ['application', 'contracts', 'infrastructure'],
    application: ['domain', 'contracts'],
    domain: ['contracts'],
    contracts: [],
    infrastructure: ['contracts']
});

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function boundedText(value, field, maxLength = 240, {allowEmpty = false} = {}) {
    if (typeof value !== 'string') throw new TypeError(`${field} must be a string`);
    const text = value.trim();
    if (!allowEmpty && !text) throw new TypeError(`${field} must not be empty`);
    if (text.length > maxLength) throw new RangeError(`${field} exceeds ${maxLength} characters`);
    return text;
}

function boundedRawText(value, field, maxLength = 240) {
    if (typeof value !== 'string') throw new TypeError(`${field} must be a string`);
    if (value.length > maxLength) throw new RangeError(`${field} exceeds ${maxLength} characters`);
    return value;
}

function nullableText(value, field, maxLength = 240) {
    if (value === null || value === undefined) return null;
    return boundedText(value, field, maxLength);
}

function assertOneOf(value, values, field) {
    if (!values.includes(value)) throw new TypeError(`${field} is not supported`);
    return value;
}

function deepFreeze(value) {
    if (!value || typeof value !== 'object' || Object.isFrozen(value)) return value;
    Object.freeze(value);
    for (const child of Object.values(value)) deepFreeze(child);
    return value;
}

/**
 * @typedef {'scene_event'|'media_event'|'pending_event'} CapabilityName
 * @typedef {'native'|'marker'} CapabilitySource
 * @typedef {{
 *   id: string|null,
 *   index: number,
 *   name: CapabilityName,
 *   argumentsText: string,
 *   arguments: unknown,
 *   source: CapabilitySource,
 *   personaId: string,
 *   causationUserMessageId: string,
 *   idempotencyKey: string
 * }} CapabilityCall
 * @typedef {{
 *   name: CapabilityName,
 *   ok: boolean,
 *   callId: string|null,
 *   idempotencyKey: string,
 *   result: unknown,
 *   error: string|null
 * }} CapabilityResult
 * @typedef {{
 *   effectId: string,
 *   kind: string,
 *   capability: CapabilityName,
 *   idempotencyKey: string,
 *   causationId: string,
 *   payload: unknown
 * }} EffectIntent
 * @typedef {{facts: unknown[], projections: unknown[], effects: EffectIntent[], presentation: unknown[]}} StepResult
 */

/**
 * Normalize the canonical native/marker capability envelope. Provider
 * parsing and capability-specific schema validation stay outside this module.
 * A null arguments value is allowed so malformed or incomplete native calls
 * can be reported without turning them into executable effects.
 *
 * @param {unknown} value
 * @returns {CapabilityCall}
 */
export function normalizeCapabilityCall(value) {
    if (!isRecord(value)) throw new TypeError('CapabilityCall must be an object');
    const id = value.id === null || value.id === undefined ? null : boundedText(value.id, 'CapabilityCall.id', 160);
    if (!Number.isInteger(value.index) || value.index < 0) throw new TypeError('CapabilityCall.index must be a non-negative integer');
    const name = assertOneOf(value.name, CAPABILITY_NAMES, 'CapabilityCall.name');
    const source = assertOneOf(value.source, CAPABILITY_SOURCES, 'CapabilityCall.source');
    const argumentsText = boundedRawText(value.argumentsText, 'CapabilityCall.argumentsText', 12_000);
    const personaId = boundedText(value.personaId, 'CapabilityCall.personaId', 160);
    const causationUserMessageId = boundedText(value.causationUserMessageId, 'CapabilityCall.causationUserMessageId', 160);
    const idempotencyKey = boundedText(value.idempotencyKey, 'CapabilityCall.idempotencyKey', 240);
    return {
        id,
        index: value.index,
        name,
        argumentsText,
        arguments: value.arguments ?? null,
        source,
        personaId,
        causationUserMessageId,
        idempotencyKey
    };
}

/**
 * @param {unknown} value
 * @returns {CapabilityResult}
 */
export function normalizeCapabilityResult(value) {
    if (!isRecord(value)) throw new TypeError('CapabilityResult must be an object');
    if (typeof value.ok !== 'boolean') throw new TypeError('CapabilityResult.ok must be a boolean');
    return {
        name: assertOneOf(value.name, CAPABILITY_NAMES, 'CapabilityResult.name'),
        ok: value.ok,
        callId: nullableText(value.callId, 'CapabilityResult.callId', 160),
        idempotencyKey: boundedText(value.idempotencyKey, 'CapabilityResult.idempotencyKey', 240),
        result: value.result ?? null,
        error: value.error === null || value.error === undefined ? null : boundedText(value.error, 'CapabilityResult.error', 240)
    };
}

/**
 * @param {unknown} value
 * @returns {EffectIntent}
 */
export function normalizeEffectIntent(value) {
    if (!isRecord(value)) throw new TypeError('EffectIntent must be an object');
    return {
        effectId: boundedText(value.effectId, 'EffectIntent.effectId', 240),
        kind: boundedText(value.kind, 'EffectIntent.kind', 120),
        capability: assertOneOf(value.capability, CAPABILITY_NAMES, 'EffectIntent.capability'),
        idempotencyKey: boundedText(value.idempotencyKey, 'EffectIntent.idempotencyKey', 240),
        causationId: boundedText(value.causationId, 'EffectIntent.causationId', 240),
        payload: value.payload ?? null
    };
}

/**
 * @param {unknown} value
 * @returns {StepResult}
 */
export function normalizeStepResult(value) {
    if (!isRecord(value)) throw new TypeError('StepResult must be an object');
    for (const channel of ['facts', 'projections', 'effects', 'presentation']) {
        if (!Array.isArray(value[channel])) throw new TypeError(`StepResult.${channel} must be an array`);
    }
    const facts = value.facts.slice();
    const projections = value.projections.slice();
    const effects = value.effects.map(normalizeEffectIntent);
    const presentation = value.presentation.slice();
    return {facts, projections, effects, presentation};
}

/**
 * Normalize a dispatcher response at the native capability handoff. The
 * dispatcher owns transport normalization and capability validation; this
 * boundary only carries bounded results and post-commit effect intents.
 */
export function normalizeCapabilityDispatch(value) {
    if (!isRecord(value)) throw new TypeError('Capability dispatch result must be an object');
    if (!Array.isArray(value.results) || !Array.isArray(value.effects)) throw new TypeError('Capability dispatch result must include results and effects arrays');
    return {
        results: value.results.map(normalizeCapabilityResult),
        effects: value.effects.map(normalizeEffectIntent)
    };
}

export function assertCapabilityDispatcherPort(value) {
    if (!isRecord(value) || typeof value.dispatch !== 'function') throw new TypeError('CapabilityDispatcherPort.dispatch must be a function');
    return value;
}

export function validatePorts(value) {
    if (!isRecord(value)) throw new TypeError('Backend ports must be an object');
    for (const [portName, methods] of Object.entries(REQUIRED_PORT_METHODS)) {
        const port = value[portName];
        if (!isRecord(port)) throw new TypeError(`Missing backend port: ${portName}`);
        for (const method of methods) {
            if (typeof port[method] !== 'function') throw new TypeError(`${portName}.${method} must be a function`);
        }
    }
    return value;
}

export function validateLayerDependencies(layer, dependencies = []) {
    assertOneOf(layer, FLOW_LAYERS, 'layer');
    if (!Array.isArray(dependencies)) throw new TypeError('dependencies must be an array');
    const allowed = ALLOWED_DEPENDENCY_LAYERS[layer];
    for (const dependency of dependencies) {
        const dependencyLayer = typeof dependency === 'string' ? dependency : dependency?.layer;
        if (!allowed.includes(dependencyLayer)) throw new Error(`${layer} cannot depend on ${dependencyLayer || 'unknown'} layer`);
    }
    return true;
}

export function emptyStepResult() {
    return {facts: [], projections: [], effects: [], presentation: []};
}

/**
 * The descriptor is JSON-compatible so it can become a checked-in fixture or
 * be consumed by a later TypeScript/OpenAPI generation step.
 */
export const BACKEND_CONTRACT_BASELINE = deepFreeze({
    version: BACKEND_CONTRACT_VERSION,
    api: {
        health: {response: {ok: 'boolean', storage: 'string'}},
        bootstrap: {response: {settings: 'object', personas: 'array', groups: 'array', activityUnread: 'boolean', defaultTimezone: 'string', debugInspector: 'boolean'}},
        conversations: {response: {items: 'Message[]', nextCursor: 'string|null'}},
        chat: {
            request: {personaId: 'string', text: 'string', attachments: 'Attachment[]'},
            transport: 'sse'
        }
    },
    sse: {
        token: {type: 'token', token: 'string'},
        done: {type: 'done', messages: 'Message[]', message: 'Message|null', learned: 'array', jobs: 'array'},
        error: {type: 'error', error: 'string'}
    },
    capability: {
        call: {
            id: 'string|null', index: 'non-negative-integer', name: CAPABILITY_NAMES,
            argumentsText: 'string', arguments: 'unknown', source: CAPABILITY_SOURCES,
            personaId: 'string', causationUserMessageId: 'string', idempotencyKey: 'string'
        },
        result: {name: CAPABILITY_NAMES, ok: 'boolean', callId: 'string|null', idempotencyKey: 'string', result: 'unknown', error: 'string|null'},
        effectIntent: {effectId: 'string', kind: 'string', capability: CAPABILITY_NAMES, idempotencyKey: 'string', causationId: 'string', payload: 'unknown'}
    },
    ports: REQUIRED_PORT_METHODS
});

export function normalizeChatResult(value = {}) {
    if (!isRecord(value)) throw new TypeError('ChatResult must be an object');
    const messages = Array.isArray(value.messages)
        ? value.messages.slice()
        : value.message ? [value.message] : [];
    return {
        ...value,
        messages,
        message: messages[0] ?? null,
        learned: Array.isArray(value.learned) ? value.learned.slice() : [],
        jobs: Array.isArray(value.jobs) ? value.jobs.slice() : []
    };
}

export function normalizeSseEvent(value) {
    if (!isRecord(value)) throw new TypeError('SseEvent must be an object');
    const type = assertOneOf(value.type, SSE_EVENT_TYPES, 'SseEvent.type');
    if (type === 'token') return {type, token: boundedText(value.token, 'SseEvent.token', 12_000, {allowEmpty: true})};
    if (type === 'error') return {type, error: boundedText(value.error, 'SseEvent.error', 480)};
    return normalizeChatResult({...value, type});
}

export function sseToken(token) {
    return normalizeSseEvent({type: 'token', token});
}

export function sseDone(result = {}) {
    return normalizeSseEvent({...normalizeChatResult(result), type: 'done'});
}

export function sseError(error) {
    return normalizeSseEvent({type: 'error', error});
}

export {FLOW_LAYERS, REQUIRED_PORT_METHODS};

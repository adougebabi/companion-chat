/**
 * Stable contracts shared by the future backend composition root.
 *
 * This module is deliberately independent from Express, SQLite, providers,
 * and any executable entrypoint. The runtime checks keep JavaScript callers
 * honest until the backend is migrated to TypeScript.
 */

export const BACKEND_CONTRACT_VERSION = 1;

export const CAPABILITY_NAMES = Object.freeze([
    'scene_event',
    'appearance_event',
    'media_event',
    'pending_event'
]);

// Native capability calls use the universal transport-facing tool set. Effect
// intents also represent internal application work, so their capability
// namespace includes the registered domain effect owners.
export const EFFECT_CAPABILITY_NAMES = Object.freeze([
    ...CAPABILITY_NAMES,
    'memory_event',
    'affect',
    'life',
    'timeline',
    'relationship',
    'pending',
    'media',
    'proactive',
    'activity',
    'conversation'
]);

export const CAPABILITY_SOURCES = Object.freeze(['native', 'marker']);
export const SSE_EVENT_TYPES = Object.freeze(['token', 'done', 'error']);

/**
 * The structured turn contract is deliberately additive to the legacy
 * capability contract above. Existing chat flows still dispatch the three
 * transport capabilities from CAPABILITY_NAMES; structured control candidates
 * may additionally carry the application-owned affect/drive controls.
 */
export const STRUCTURED_TURN_SCHEMA_VERSION = 'companion.turn.v1';
export const TURN_SCHEMA_VERSION = STRUCTURED_TURN_SCHEMA_VERSION;
export const STRUCTURED_TURN_SOURCE_MODES = Object.freeze([
    'text',
    'native_tools',
    'structured_sidecar',
    'legacy_marker'
]);
export const STRUCTURED_CAPABILITY_NAMES = Object.freeze([
    ...CAPABILITY_NAMES,
    'memory_event',
    'affect_event',
    'drive_signal'
]);
export const AFFECT_EVENT_TYPES = Object.freeze([
    'social_connection',
    'social_friction',
    'exploration_discovery',
    'exploration_blocked',
    'restored',
    'fatigue'
]);
export const DRIVE_NAMES = Object.freeze(['social', 'exploration', 'rest']);
export const DRIVE_SIGNAL_DIRECTIONS = Object.freeze([
    'increase_pressure',
    'decrease_pressure',
    'neutral'
]);
export const MEMORY_WRITE_OPERATIONS = Object.freeze(['insert', 'upsert']);

export const STRUCTURED_TURN_LIMITS = Object.freeze({
    text: 12_000,
    token: 12_000,
    tokens: 256,
    messages: 20,
    affectEvents: 8,
    driveSignals: 8,
    memoryWrites: 8,
    capabilityCalls: 8,
    diagnostics: 8,
    diagnostic: 240,
    id: 160,
    key: 120,
    value: 4_000,
    argumentsText: 12_000,
    idempotencyKey: 240,
    modelVersion: 120,
    sourceType: 80,
    sourceId: 160
});

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

function boundedStructuredText(value, field, maxLength, {allowEmpty = false} = {}) {
    return boundedText(value, field, maxLength, {allowEmpty});
}

function boundedConfidence(value, field) {
    if (typeof value !== 'number' || !Number.isFinite(value) || value < 0 || value > 1) {
        throw new RangeError(`${field} must be a number between 0 and 1`);
    }
    return value;
}

function boundedStructuredArray(value, field, maxLength) {
    if (value === undefined || value === null) return [];
    if (!Array.isArray(value)) throw new TypeError(`${field} must be an array`);
    if (value.length > maxLength) throw new RangeError(`${field} exceeds ${maxLength} items`);
    return value;
}

function assertStructuredKeys(value, allowed, field) {
    for (const key of Object.keys(value)) {
        if (!allowed.includes(key)) throw new TypeError(`${field}.${key} is not supported`);
    }
}

function structuredContext(value, context = {}) {
    const source = isRecord(context) ? context : {};
    const input = isRecord(value) ? value : {};
    const candidatePersonaId = input.personaId ?? null;
    const scopedPersonaId = source.personaId ?? null;
    if (candidatePersonaId && scopedPersonaId && candidatePersonaId !== scopedPersonaId) {
        throw new TypeError('structured personaId does not match the application scope');
    }
    const candidateCausationId = input.causationId ?? input.causationUserMessageId ?? null;
    const scopedCausationId = source.causationId ?? source.causationUserMessageId ?? null;
    if (candidateCausationId && scopedCausationId && candidateCausationId !== scopedCausationId) {
        throw new TypeError('structured causationId does not match the application scope');
    }
    const candidateSourceMessageId = input.sourceMessageId ?? null;
    const scopedSourceMessageId = source.sourceMessageId ?? null;
    if (candidateSourceMessageId && scopedSourceMessageId && candidateSourceMessageId !== scopedSourceMessageId) {
        throw new TypeError('structured sourceMessageId does not match the application scope');
    }
    const personaId = scopedPersonaId ?? candidatePersonaId;
    const causationId = scopedCausationId ?? candidateCausationId;
    const sourceMessageId = scopedSourceMessageId ?? candidateSourceMessageId ?? causationId ?? null;
    const optionalText = (next, field, maxLength) => next === null || next === undefined || next === ''
        ? null
        : boundedStructuredText(next, field, maxLength);
    return {
        personaId: optionalText(personaId, 'structured personaId', STRUCTURED_TURN_LIMITS.id),
        causationId: optionalText(causationId, 'structured causationId', STRUCTURED_TURN_LIMITS.id),
        sourceMessageId: optionalText(sourceMessageId, 'structured sourceMessageId', STRUCTURED_TURN_LIMITS.sourceId),
        modelVersion: input.modelVersion ?? source.modelVersion ?? null
    };
}

function structuredIdempotencyKey(value, field) {
    return boundedStructuredText(value, field, STRUCTURED_TURN_LIMITS.idempotencyKey);
}

function boundedJsonValue(value, field, depth = 0) {
    if (value === null || typeof value === 'boolean') return value;
    if (typeof value === 'number') {
        if (!Number.isFinite(value)) throw new TypeError(`${field} must contain finite numbers`);
        return value;
    }
    if (typeof value === 'string') return boundedStructuredText(value, field, STRUCTURED_TURN_LIMITS.value, {allowEmpty: true});
    if (depth >= 4) throw new RangeError(`${field} exceeds nested value depth`);
    if (Array.isArray(value)) {
        if (value.length > STRUCTURED_TURN_LIMITS.messages) throw new RangeError(`${field} exceeds ${STRUCTURED_TURN_LIMITS.messages} items`);
        return value.map((item, index) => boundedJsonValue(item, `${field}[${index}]`, depth + 1));
    }
    if (!isRecord(value)) throw new TypeError(`${field} must be JSON-compatible`);
    const entries = Object.entries(value);
    if (entries.length > STRUCTURED_TURN_LIMITS.messages) throw new RangeError(`${field} has too many properties`);
    return Object.fromEntries(entries.map(([key, child]) => [
        boundedStructuredText(key, `${field} key`, STRUCTURED_TURN_LIMITS.key),
        boundedJsonValue(child, `${field}.${key}`, depth + 1)
    ]));
}

function normalizeOptionalModelVersion(value, fallback = null) {
    const next = value ?? fallback;
    return next === null || next === undefined
        ? null
        : boundedStructuredText(next, 'structured modelVersion', STRUCTURED_TURN_LIMITS.modelVersion);
}

/**
 * Normalize a model-proposed PAD event. The model supplies only an allowlisted
 * event type and confidence; PAD deltas remain a server-owned reducer concern.
 */
export function normalizeAffectEventCandidate(value, context = {}) {
    if (!isRecord(value)) throw new TypeError('Affect event candidate must be an object');
    assertStructuredKeys(value, ['type', 'eventType', 'confidence', 'sourceMessageId', 'personaId', 'causationId', 'idempotencyKey', 'modelVersion'], 'Affect event candidate');
    const type = assertOneOf(value.type ?? value.eventType, AFFECT_EVENT_TYPES, 'Affect event type');
    const ids = structuredContext(value, context);
    return {
        type,
        confidence: boundedConfidence(value.confidence, 'Affect event confidence'),
        personaId: ids.personaId,
        sourceMessageId: ids.sourceMessageId,
        causationId: ids.causationId,
        idempotencyKey: structuredIdempotencyKey(value.idempotencyKey, 'Affect event idempotencyKey'),
        modelVersion: normalizeOptionalModelVersion(value.modelVersion, ids.modelVersion)
    };
}

/**
 * Normalize a drive pressure signal. Unknown, syntactically safe keys are
 * retained for forward-compatible storage but are marked inactive until a
 * registered server policy exists.
 */
export function normalizeDriveSignalCandidate(value, context = {}) {
    if (!isRecord(value)) throw new TypeError('Drive signal candidate must be an object');
    assertStructuredKeys(value, ['drive', 'recognized', 'direction', 'confidence', 'sourceMessageId', 'personaId', 'causationId', 'idempotencyKey', 'modelVersion'], 'Drive signal candidate');
    const drive = boundedStructuredText(value.drive, 'Drive signal drive', STRUCTURED_TURN_LIMITS.key);
    if (!/^[a-z][a-z0-9_:-]{0,79}$/.test(drive)) throw new TypeError('Drive signal drive is not supported');
    const ids = structuredContext(value, context);
    return {
        drive,
        recognized: DRIVE_NAMES.includes(drive),
        direction: assertOneOf(value.direction, DRIVE_SIGNAL_DIRECTIONS, 'Drive signal direction'),
        confidence: boundedConfidence(value.confidence, 'Drive signal confidence'),
        personaId: ids.personaId,
        sourceMessageId: ids.sourceMessageId,
        causationId: ids.causationId,
        idempotencyKey: structuredIdempotencyKey(value.idempotencyKey, 'Drive signal idempotencyKey'),
        modelVersion: normalizeOptionalModelVersion(value.modelVersion, ids.modelVersion)
    };
}

/**
 * Normalize the durable-memory intent carried by `memory_event`. This is a
 * candidate only: repository ownership, duplicate handling, and transaction
 * commit remain application/infrastructure responsibilities.
 */
export function normalizeMemoryWriteCandidate(value, context = {}) {
    if (!isRecord(value)) throw new TypeError('Memory write candidate must be an object');
    assertStructuredKeys(value, ['operation', 'key', 'value', 'confidence', 'sourceType', 'sourceId', 'sourceMessageId', 'personaId', 'causationId', 'idempotencyKey', 'modelVersion'], 'Memory write candidate');
    const ids = structuredContext(value, context);
    const operation = assertOneOf(value.operation ?? 'upsert', MEMORY_WRITE_OPERATIONS, 'Memory write operation');
    const key = boundedStructuredText(value.key, 'Memory write key', STRUCTURED_TURN_LIMITS.key);
    if (value.value === undefined) throw new TypeError('Memory write value must be provided');
    const normalizedValue = boundedJsonValue(value.value, 'Memory write value');
    const sourceMessageId = value.sourceMessageId === undefined ? ids.sourceMessageId : value.sourceMessageId;
    const sourceId = value.sourceId === undefined
        ? sourceMessageId
        : boundedStructuredText(value.sourceId, 'Memory write sourceId', STRUCTURED_TURN_LIMITS.sourceId);
    return {
        operation,
        key,
        value: normalizedValue,
        confidence: boundedConfidence(value.confidence, 'Memory write confidence'),
        personaId: ids.personaId,
        sourceType: boundedStructuredText(value.sourceType ?? 'conversation', 'Memory write sourceType', STRUCTURED_TURN_LIMITS.sourceType),
        sourceId,
        sourceMessageId: sourceMessageId === null || sourceMessageId === undefined
            ? null
            : boundedStructuredText(sourceMessageId, 'Memory write sourceMessageId', STRUCTURED_TURN_LIMITS.sourceId),
        causationId: ids.causationId,
        idempotencyKey: structuredIdempotencyKey(value.idempotencyKey, 'Memory write idempotencyKey'),
        modelVersion: normalizeOptionalModelVersion(value.modelVersion, ids.modelVersion)
    };
}

/**
 * Normalize a structured capability candidate without executing it. Native
 * calls retain their old envelope so the existing dispatcher can continue to
 * own legacy capability validation and marker precedence.
 */
export function normalizeStructuredCapabilityCall(value, context = {}) {
    if (!isRecord(value)) throw new TypeError('Structured capability call must be an object');
    assertStructuredKeys(value, ['id', 'index', 'name', 'argumentsText', 'arguments', 'source', 'personaId', 'causationUserMessageId', 'causationId', 'idempotencyKey'], 'Structured capability call');
    const ids = structuredContext(value, context);
    const name = assertOneOf(value.name, STRUCTURED_CAPABILITY_NAMES, 'Structured capability call name');
    const source = value.source === undefined ? 'structured' : assertOneOf(value.source, ['native', 'structured'], 'Structured capability call source');
    const id = value.id === null || value.id === undefined ? null : boundedStructuredText(value.id, 'Structured capability call id', STRUCTURED_TURN_LIMITS.id);
    if (value.index !== undefined && value.index !== null && (!Number.isInteger(value.index) || value.index < 0)) {
        throw new TypeError('Structured capability call index must be a non-negative integer');
    }
    const argumentsText = value.argumentsText === undefined
        ? (() => { try { return JSON.stringify(value.arguments ?? {}); } catch { return ''; } })()
        : boundedStructuredText(value.argumentsText, 'Structured capability argumentsText', STRUCTURED_TURN_LIMITS.argumentsText, {allowEmpty: true});
    let args = value.arguments;
    if (args === undefined && argumentsText) {
        try { args = JSON.parse(argumentsText); } catch { throw new TypeError('Structured capability arguments must be valid JSON'); }
    }
    if (!isRecord(args)) throw new TypeError('Structured capability arguments must be an object');
    return {
        id,
        index: Number.isInteger(value.index) ? value.index : null,
        name,
        argumentsText,
        arguments: boundedJsonValue(args, 'Structured capability arguments'),
        source,
        personaId: ids.personaId,
        causationUserMessageId: ids.causationId,
        idempotencyKey: structuredIdempotencyKey(value.idempotencyKey, 'Structured capability idempotencyKey')
    };
}

function normalizeStructuredMessage(value, index) {
    if (!isRecord(value)) throw new TypeError(`Structured turn message ${index} must be an object`);
    const role = assertOneOf(value.role ?? 'assistant', ['assistant', 'user', 'tool'], `Structured turn message ${index} role`);
    const text = boundedStructuredText(value.text ?? value.content ?? '', `Structured turn message ${index} text`, STRUCTURED_TURN_LIMITS.text, {allowEmpty: true});
    return {
        role,
        text,
        ...(value.id === undefined ? {} : {id: boundedStructuredText(value.id, `Structured turn message ${index} id`, STRUCTURED_TURN_LIMITS.id)})
    };
}

function normalizeStructuredDiagnostics(value) {
    return boundedStructuredArray(value, 'Structured turn parseDiagnostics', STRUCTURED_TURN_LIMITS.diagnostics)
        .map((item, index) => boundedStructuredText(item, `Structured turn parseDiagnostics ${index}`, STRUCTURED_TURN_LIMITS.diagnostic));
}

function structuredControlInput(value) {
    const control = isRecord(value.control) ? value.control : value;
    return {
        affectEvents: control.affectEvents ?? value.affectEvents,
        driveSignals: control.driveSignals ?? value.driveSignals,
        memoryWrites: control.memoryWrites ?? value.memoryWrites,
        capabilityCalls: control.capabilityCalls ?? value.capabilityCalls ?? value.toolCalls
    };
}

function sourceModeFor(value, control) {
    if (value.sourceMode !== undefined) return assertOneOf(value.sourceMode, STRUCTURED_TURN_SOURCE_MODES, 'Structured turn sourceMode');
    if ((control.capabilityCalls || []).some(call => call?.source === 'native')) return 'native_tools';
    const hasStructuredControl = ['affectEvents', 'driveSignals', 'memoryWrites', 'capabilityCalls']
        .some(field => Array.isArray(control[field]) && control[field].length > 0);
    if (hasStructuredControl || value.structuredSidecar || value.structuredTurn || value.structured_turn) return 'structured_sidecar';
    if (/<(?:media-intent|pending-event|scene-event)>/i.test(value.text || '')) return 'legacy_marker';
    return 'text';
}

/**
 * Canonical, application-facing structured turn envelope. This function is a
 * strict validator: callers that receive optional provider control should use
 * normalizeStructuredTurnSafely() so a malformed sidecar can be dropped while
 * preserving the visible text.
 */
export function normalizeStructuredTurnEnvelope(value = {}, context = {}) {
    if (!isRecord(value)) throw new TypeError('Structured turn must be an object');
    assertStructuredKeys(value, ['schemaVersion', 'version', 'text', 'tokens', 'messages', 'control', 'affectEvents', 'driveSignals', 'memoryWrites', 'capabilityCalls', 'parseDiagnostics', 'diagnostics', 'sourceMode', 'structuredSidecar', 'structuredTurn', 'structured_turn'], 'Structured turn');
    const schemaVersion = value.schemaVersion ?? value.version;
    if (schemaVersion !== STRUCTURED_TURN_SCHEMA_VERSION) {
        throw new TypeError(`Structured turn schemaVersion must be ${STRUCTURED_TURN_SCHEMA_VERSION}`);
    }
    const tokens = boundedStructuredArray(value.tokens, 'Structured turn tokens', STRUCTURED_TURN_LIMITS.tokens)
        .map((token, index) => boundedRawText(token, `Structured turn token ${index}`, STRUCTURED_TURN_LIMITS.token));
    const text = value.text === undefined || value.text === null
        ? tokens.join('')
        : boundedStructuredText(value.text, 'Structured turn text', STRUCTURED_TURN_LIMITS.text, {allowEmpty: true});
    const controlInput = structuredControlInput(value);
    if (isRecord(value.control)) assertStructuredKeys(value.control, ['affectEvents', 'driveSignals', 'memoryWrites', 'capabilityCalls'], 'Structured turn control');
    const ids = structuredContext(value, context);
    const normalizedCalls = boundedStructuredArray(controlInput.capabilityCalls, 'Structured turn capabilityCalls', STRUCTURED_TURN_LIMITS.capabilityCalls)
        .map(candidate => normalizeStructuredCapabilityCall(candidate, ids));
    const toolAffectEvents = normalizedCalls
        .filter(call => call.name === 'affect_event')
        .map(call => {
            const args = isRecord(call.arguments?.event) ? call.arguments.event : call.arguments;
            return normalizeAffectEventCandidate({
                ...args,
                idempotencyKey: args.idempotencyKey ?? call.idempotencyKey
            }, ids);
        });
    const toolDriveSignals = normalizedCalls
        .filter(call => call.name === 'drive_signal')
        .map(call => {
            const args = isRecord(call.arguments?.signal) ? call.arguments.signal : call.arguments;
            return normalizeDriveSignalCandidate({
                ...args,
                idempotencyKey: args.idempotencyKey ?? call.idempotencyKey
            }, ids);
        });
    const affectEvents = [
        ...boundedStructuredArray(controlInput.affectEvents, 'Structured turn affectEvents', STRUCTURED_TURN_LIMITS.affectEvents)
            .map(candidate => normalizeAffectEventCandidate(candidate, ids)),
        ...toolAffectEvents
    ].slice(0, STRUCTURED_TURN_LIMITS.affectEvents);
    const driveSignals = [
        ...boundedStructuredArray(controlInput.driveSignals, 'Structured turn driveSignals', STRUCTURED_TURN_LIMITS.driveSignals)
            .map(candidate => normalizeDriveSignalCandidate(candidate, ids)),
        ...toolDriveSignals
    ].slice(0, STRUCTURED_TURN_LIMITS.driveSignals);
    const capabilityCalls = normalizedCalls.filter(call => call.name !== 'affect_event' && call.name !== 'drive_signal');
    const directMemoryWrites = boundedStructuredArray(controlInput.memoryWrites, 'Structured turn memoryWrites', STRUCTURED_TURN_LIMITS.memoryWrites)
        .map(candidate => normalizeMemoryWriteCandidate(candidate, ids));
    const callMemoryWrites = capabilityCalls
        .filter(call => call.name === 'memory_event')
        .map(call => {
            const args = isRecord(call.arguments?.memory) ? call.arguments.memory : call.arguments;
            return normalizeMemoryWriteCandidate({
                ...args,
                idempotencyKey: args.idempotencyKey ?? call.idempotencyKey
            }, {...ids, sourceMessageId: ids.sourceMessageId});
        });
    const memoryByKey = new Map();
    for (const candidate of [...directMemoryWrites, ...callMemoryWrites]) {
        if (!memoryByKey.has(candidate.idempotencyKey)) memoryByKey.set(candidate.idempotencyKey, candidate);
    }
    const messages = boundedStructuredArray(value.messages, 'Structured turn messages', STRUCTURED_TURN_LIMITS.messages)
        .map(normalizeStructuredMessage);
    const parseDiagnostics = normalizeStructuredDiagnostics(value.parseDiagnostics ?? value.diagnostics);
    const normalized = {
        schemaVersion: STRUCTURED_TURN_SCHEMA_VERSION,
        text,
        tokens: tokens.length ? tokens : text ? [text] : [],
        messages,
        control: {
            affectEvents,
            driveSignals,
            memoryWrites: [...memoryByKey.values()],
            capabilityCalls
        },
        parseDiagnostics,
        sourceMode: sourceModeFor(value, {capabilityCalls: normalizedCalls})
    };
    return normalized;
}

/**
 * Optional control is fail-closed. The visible completion remains usable, but
 * no candidate reaches an application effect planner when validation fails.
 */
export function normalizeStructuredTurnSafely(value = {}, context = {}) {
    const input = isRecord(value) ? value : {};
    const diagnostics = [];
    try {
        return {ok: true, value: normalizeStructuredTurnEnvelope(input, context)};
    } catch (error) {
        const message = boundedText(error instanceof Error ? error.message : error, 'Structured turn diagnostic', STRUCTURED_TURN_LIMITS.diagnostic);
        diagnostics.push(message);
        const tokens = Array.isArray(input.tokens) ? input.tokens.filter(token => typeof token === 'string').slice(0, STRUCTURED_TURN_LIMITS.tokens) : [];
        const text = typeof input.text === 'string' ? input.text.slice(0, STRUCTURED_TURN_LIMITS.text) : tokens.join('');
        return {
            ok: false,
            value: {
                schemaVersion: STRUCTURED_TURN_SCHEMA_VERSION,
                text,
                tokens: tokens.length ? tokens : text ? [text] : [],
                messages: [],
                control: {affectEvents: [], driveSignals: [], memoryWrites: [], capabilityCalls: []},
                parseDiagnostics: diagnostics,
                sourceMode: sourceModeFor(input, {capabilityCalls: []})
            },
            error: message
        };
    }
}

export function validateStructuredTurn(value = {}, context = {}) {
    const result = normalizeStructuredTurnSafely(value, context);
    return result.ok ? {ok: true, value: result.value, errors: []} : {ok: false, value: result.value, errors: [result.error]};
}

export const normalizeMemoryEventCandidate = normalizeMemoryWriteCandidate;
export const normalizeAffectEvent = normalizeAffectEventCandidate;
export const normalizeDriveSignal = normalizeDriveSignalCandidate;
export const normalizeStructuredTurn = normalizeStructuredTurnEnvelope;
export const normalizeStructuredTurnResult = normalizeStructuredTurnEnvelope;
export const normalizeTurnEnvelope = normalizeStructuredTurnEnvelope;

export const STRUCTURED_TURN_SCHEMA = deepFreeze({
    $id: STRUCTURED_TURN_SCHEMA_VERSION,
    type: 'object',
    additionalProperties: false,
    $defs: {
        affectEvent: {
            type: 'object',
            additionalProperties: false,
            required: ['type', 'confidence', 'idempotencyKey'],
            properties: {
                type: {enum: AFFECT_EVENT_TYPES},
                confidence: {type: 'number', minimum: 0, maximum: 1},
                personaId: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.id},
                sourceMessageId: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.sourceId},
                causationId: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.id},
                idempotencyKey: {type: 'string', minLength: 1, maxLength: STRUCTURED_TURN_LIMITS.idempotencyKey},
                modelVersion: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.modelVersion}
            }
        },
        driveSignal: {
            type: 'object',
            additionalProperties: false,
            required: ['drive', 'direction', 'confidence', 'idempotencyKey'],
            properties: {
                drive: {type: 'string', pattern: '^[a-z][a-z0-9_:-]{0,79}$'},
                recognized: {type: 'boolean'},
                direction: {enum: DRIVE_SIGNAL_DIRECTIONS},
                confidence: {type: 'number', minimum: 0, maximum: 1},
                personaId: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.id},
                sourceMessageId: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.sourceId},
                causationId: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.id},
                idempotencyKey: {type: 'string', minLength: 1, maxLength: STRUCTURED_TURN_LIMITS.idempotencyKey},
                modelVersion: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.modelVersion}
            }
        },
        memoryWrite: {
            type: 'object',
            additionalProperties: false,
            required: ['operation', 'key', 'value', 'confidence', 'idempotencyKey'],
            properties: {
                operation: {enum: MEMORY_WRITE_OPERATIONS},
                key: {type: 'string', minLength: 1, maxLength: STRUCTURED_TURN_LIMITS.key},
                value: {},
                confidence: {type: 'number', minimum: 0, maximum: 1},
                personaId: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.id},
                sourceType: {type: 'string', maxLength: STRUCTURED_TURN_LIMITS.sourceType},
                sourceId: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.sourceId},
                sourceMessageId: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.sourceId},
                causationId: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.id},
                idempotencyKey: {type: 'string', minLength: 1, maxLength: STRUCTURED_TURN_LIMITS.idempotencyKey},
                modelVersion: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.modelVersion}
            }
        },
        capabilityCall: {
            type: 'object',
            additionalProperties: false,
            required: ['name', 'arguments', 'idempotencyKey'],
            properties: {
                id: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.id},
                index: {type: ['integer', 'null'], minimum: 0},
                name: {enum: STRUCTURED_CAPABILITY_NAMES},
                arguments: {type: 'object'},
                argumentsText: {type: 'string', maxLength: STRUCTURED_TURN_LIMITS.argumentsText},
                source: {enum: ['native', 'structured']},
                personaId: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.id},
                causationUserMessageId: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.id},
                idempotencyKey: {type: 'string', minLength: 1, maxLength: STRUCTURED_TURN_LIMITS.idempotencyKey}
            }
        }
    },
    required: ['schemaVersion', 'text', 'control'],
    properties: {
        schemaVersion: {const: STRUCTURED_TURN_SCHEMA_VERSION},
        text: {type: 'string', maxLength: STRUCTURED_TURN_LIMITS.text},
        tokens: {type: 'array', maxItems: STRUCTURED_TURN_LIMITS.tokens, items: {type: 'string', maxLength: STRUCTURED_TURN_LIMITS.token}},
        messages: {type: 'array', maxItems: STRUCTURED_TURN_LIMITS.messages, items: {type: 'object'}},
        control: {
            type: 'object',
            additionalProperties: false,
            properties: {
                affectEvents: {type: 'array', maxItems: STRUCTURED_TURN_LIMITS.affectEvents, items: {$ref: '#/$defs/affectEvent'}},
                driveSignals: {type: 'array', maxItems: STRUCTURED_TURN_LIMITS.driveSignals, items: {$ref: '#/$defs/driveSignal'}},
                memoryWrites: {type: 'array', maxItems: STRUCTURED_TURN_LIMITS.memoryWrites, items: {$ref: '#/$defs/memoryWrite'}},
                capabilityCalls: {type: 'array', maxItems: STRUCTURED_TURN_LIMITS.capabilityCalls, items: {$ref: '#/$defs/capabilityCall'}}
            }
        },
        parseDiagnostics: {type: 'array', maxItems: STRUCTURED_TURN_LIMITS.diagnostics, items: {type: 'string', maxLength: STRUCTURED_TURN_LIMITS.diagnostic}},
        sourceMode: {enum: STRUCTURED_TURN_SOURCE_MODES}
    }
});

/**
 * @typedef {'scene_event'|'appearance_event'|'media_event'|'pending_event'} CapabilityName
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
export function normalizeCapabilityCall(value, {allowStructured = false} = {}) {
    if (!isRecord(value)) throw new TypeError('CapabilityCall must be an object');
    const id = value.id === null || value.id === undefined ? null : boundedText(value.id, 'CapabilityCall.id', 160);
    if (!Number.isInteger(value.index) || value.index < 0) throw new TypeError('CapabilityCall.index must be a non-negative integer');
    const names = allowStructured ? STRUCTURED_CAPABILITY_NAMES : CAPABILITY_NAMES;
    const sources = allowStructured ? [...CAPABILITY_SOURCES, 'structured'] : CAPABILITY_SOURCES;
    const name = assertOneOf(value.name, names, 'CapabilityCall.name');
    const source = assertOneOf(value.source, sources, 'CapabilityCall.source');
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
export function normalizeCapabilityResult(value, {allowStructured = false} = {}) {
    if (!isRecord(value)) throw new TypeError('CapabilityResult must be an object');
    if (typeof value.ok !== 'boolean') throw new TypeError('CapabilityResult.ok must be a boolean');
    return {
        name: assertOneOf(value.name, allowStructured ? STRUCTURED_CAPABILITY_NAMES : CAPABILITY_NAMES, 'CapabilityResult.name'),
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
        capability: assertOneOf(value.capability, EFFECT_CAPABILITY_NAMES, 'EffectIntent.capability'),
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
export function normalizeCapabilityDispatch(value, {allowStructured = false} = {}) {
    if (!isRecord(value)) throw new TypeError('Capability dispatch result must be an object');
    if (!Array.isArray(value.results) || !Array.isArray(value.effects)) throw new TypeError('Capability dispatch result must include results and effects arrays');
    return {
        results: value.results.map(result => normalizeCapabilityResult(result, {allowStructured})),
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
            transport: 'sse',
            control: {
                schemaVersion: STRUCTURED_TURN_SCHEMA_VERSION,
                mode: 'visible text stream + validated structured control'
            }
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
        effectIntent: {effectId: 'string', kind: 'string', capability: EFFECT_CAPABILITY_NAMES, idempotencyKey: 'string', causationId: 'string', payload: 'unknown'}
    },
    structuredTurn: STRUCTURED_TURN_SCHEMA,
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
    if (type === 'token') return {type, token: boundedRawText(value.token, 'SseEvent.token', 12_000)};
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

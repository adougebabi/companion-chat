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
    'memory_consolidation',
    'appraisal',
    'self_model',
    'agency_intention',
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
export const INTERACTION_FACT_SCHEMA_VERSION = 'companion.interaction-fact.v1';
export const APPRAISAL_SCHEMA_VERSION = 'companion.appraisal.v1';
export const MEMORY_CONSOLIDATION_SCHEMA_VERSION = 'companion.memory-consolidation.v1';
export const SELF_MODEL_CLAIM_SCHEMA_VERSION = 'companion.self-model.v1';
export const SELF_MODEL_SCHEMA_VERSION = SELF_MODEL_CLAIM_SCHEMA_VERSION;
export const AGENCY_INTENTION_SCHEMA_VERSION = 'companion.agency-intention.v1';
export const INTERACTION_FACT_TYPES = Object.freeze([
    'user_message',
    'confirmed_fact',
    'capability_result',
    'relationship_change',
    'time_boundary'
]);
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
export const MEMORY_CONSOLIDATION_STATUSES = Object.freeze([
    'candidate',
    'deferred',
    'rejected',
    'superseded',
    'promoted'
]);
export const SELF_MODEL_CLAIM_STATUSES = Object.freeze([
    'candidate',
    'active',
    'confirmed',
    'deferred',
    'rejected',
    'superseded'
]);
export const AGENCY_INTENTION_STATUSES = Object.freeze([
    'candidate',
    'qualified',
    'frozen',
    'delivered',
    'deferred',
    'rejected',
    'skipped'
]);

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
    sourceId: 160,
    appraisalCategory: 80,
    appraisalRationale: 1_000,
    appraisalEvidenceRefs: 8,
    appraisals: 8,
    interactionPayload: 4_000,
    memoryConsolidationLayer: 80,
    memoryConsolidationClaim: 4_000,
    memoryConsolidations: 8,
    memoryConsolidationEvidenceRefs: 16,
    memoryConsolidationSourceFactRefs: 16,
    memoryConsolidationError: 240,
    selfModelCategory: 80,
    selfModelClaim: 2_000,
    selfModelSummary: 1_000,
    selfModelEvidenceRefs: 16,
    selfModelSource: 80,
    selfModelUncertainty: 1_000,
    selfModelDecayPolicy: 240,
    selfModelClaims: 8,
    agencyIntentions: 8,
    agencyIntent: 80,
    agencyTopic: 240,
    agencyExplanation: 1_000,
    agencyEvidenceRefs: 16,
    agencyReason: 80
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

function boundedReferenceList(value, field) {
    const values = boundedStructuredArray(value, field, STRUCTURED_TURN_LIMITS.appraisalEvidenceRefs);
    return values.map((item, index) => boundedStructuredText(
        item,
        `${field} ${index}`,
        STRUCTURED_TURN_LIMITS.sourceId
    ));
}

/**
 * Normalize a server-observed interaction fact. Facts contain references and
 * bounded JSON only; semantic interpretation belongs to the model appraisal
 * contract below.
 */
export function normalizeInteractionFact(value, context = {}) {
    if (!isRecord(value)) throw new TypeError('Interaction fact must be an object');
    assertStructuredKeys(value, [
        'schemaVersion', 'version', 'factType', 'type', 'personaId', 'sourceMessageId',
        'sourceId', 'causationId', 'idempotencyKey', 'payload', 'source', 'modelVersion',
        'evidenceRefs'
    ], 'Interaction fact');
    const schemaVersion = value.schemaVersion ?? value.version;
    if (schemaVersion !== INTERACTION_FACT_SCHEMA_VERSION) {
        throw new TypeError(`Interaction fact schemaVersion must be ${INTERACTION_FACT_SCHEMA_VERSION}`);
    }
    const ids = structuredContext(value, context);
    const factType = assertOneOf(value.factType ?? value.type, INTERACTION_FACT_TYPES, 'Interaction fact type');
    const sourceMessageId = value.sourceMessageId === undefined || value.sourceMessageId === null
        ? value.sourceId === undefined || value.sourceId === null
            ? ids.sourceMessageId
            : boundedStructuredText(value.sourceId, 'Interaction fact sourceId', STRUCTURED_TURN_LIMITS.sourceId)
        : boundedStructuredText(value.sourceMessageId, 'Interaction fact sourceMessageId', STRUCTURED_TURN_LIMITS.sourceId);
    const payload = value.payload === undefined ? {} : boundedJsonValue(value.payload, 'Interaction fact payload');
    const serializedPayload = JSON.stringify(payload);
    if (serializedPayload.length > STRUCTURED_TURN_LIMITS.interactionPayload) {
        throw new RangeError(`Interaction fact payload exceeds ${STRUCTURED_TURN_LIMITS.interactionPayload} bytes`);
    }
    return {
        schemaVersion: INTERACTION_FACT_SCHEMA_VERSION,
        factType,
        personaId: ids.personaId,
        sourceMessageId,
        causationId: ids.causationId,
        idempotencyKey: structuredIdempotencyKey(value.idempotencyKey, 'Interaction fact idempotencyKey'),
        payload,
        source: boundedStructuredText(value.source ?? 'runtime', 'Interaction fact source', STRUCTURED_TURN_LIMITS.sourceType),
        modelVersion: normalizeOptionalModelVersion(value.modelVersion, ids.modelVersion),
        evidenceRefs: boundedReferenceList(value.evidenceRefs, 'Interaction fact evidenceRefs')
    };
}

/**
 * Normalize the LLM-owned appraisal. The server validates the envelope and
 * nested reducer candidates, but never maps category/rationale to a signal.
 */
export function normalizeAppraisalCandidate(value, context = {}) {
    if (!isRecord(value)) throw new TypeError('Appraisal candidate must be an object');
    assertStructuredKeys(value, [
        'schemaVersion', 'version', 'category', 'confidence', 'rationale', 'reason',
        'evidenceRefs', 'affectEvents', 'driveSignals', 'personaId', 'sourceMessageId',
        'causationId', 'idempotencyKey', 'modelVersion', 'source', 'interactionFactId'
    ], 'Appraisal candidate');
    const schemaVersion = value.schemaVersion ?? value.version;
    if (schemaVersion !== APPRAISAL_SCHEMA_VERSION) {
        throw new TypeError(`Appraisal schemaVersion must be ${APPRAISAL_SCHEMA_VERSION}`);
    }
    const ids = structuredContext(value, context);
    const category = boundedStructuredText(value.category, 'Appraisal category', STRUCTURED_TURN_LIMITS.appraisalCategory);
    const rationale = boundedStructuredText(
        value.rationale ?? value.reason ?? '',
        'Appraisal rationale',
        STRUCTURED_TURN_LIMITS.appraisalRationale,
        {allowEmpty: true}
    );
    const affectEvents = boundedStructuredArray(value.affectEvents, 'Appraisal affectEvents', STRUCTURED_TURN_LIMITS.affectEvents)
        .map(candidate => normalizeAffectEventCandidate(candidate, ids));
    const driveSignals = boundedStructuredArray(value.driveSignals, 'Appraisal driveSignals', STRUCTURED_TURN_LIMITS.driveSignals)
        .map(candidate => normalizeDriveSignalCandidate(candidate, ids));
    const interactionFactId = value.interactionFactId === undefined || value.interactionFactId === null
        ? null
        : boundedStructuredText(value.interactionFactId, 'Appraisal interactionFactId', STRUCTURED_TURN_LIMITS.id);
    return {
        schemaVersion: APPRAISAL_SCHEMA_VERSION,
        category,
        confidence: boundedConfidence(value.confidence, 'Appraisal confidence'),
        rationale,
        evidenceRefs: boundedReferenceList(value.evidenceRefs, 'Appraisal evidenceRefs'),
        affectEvents,
        driveSignals,
        personaId: ids.personaId,
        sourceMessageId: ids.sourceMessageId,
        causationId: ids.causationId,
        idempotencyKey: structuredIdempotencyKey(value.idempotencyKey, 'Appraisal idempotencyKey'),
        modelVersion: normalizeOptionalModelVersion(value.modelVersion, ids.modelVersion),
        source: boundedStructuredText(value.source ?? 'llm', 'Appraisal source', STRUCTURED_TURN_LIMITS.sourceType),
        interactionFactId
    };
}

/**
 * Normalize an LLM-proposed memory consolidation candidate. This is an
 * auditable ledger intent, not a write to companion_memories. The server only
 * enforces the versioned shape, bounded evidence, ownership context, and
 * revision/status fields; it does not decide which semantic candidates are
 * useful or durable.
 */
export function normalizeMemoryConsolidationCandidate(value, context = {}) {
    if (!isRecord(value)) throw new TypeError('Memory consolidation candidate must be an object');
    assertStructuredKeys(value, [
        'schemaVersion', 'version', 'layer', 'key', 'value', 'claim', 'confidence',
        'evidenceRefs', 'sourceFactRefs', 'sourceRefs', 'factRefs', 'personaId',
        'sourceMessageId', 'causationId', 'idempotencyKey', 'source', 'sourceType',
        'modelVersion', 'status', 'revision', 'interactionFactId'
    ], 'Memory consolidation candidate');
    const schemaVersion = value.schemaVersion ?? value.version;
    if (schemaVersion !== MEMORY_CONSOLIDATION_SCHEMA_VERSION) {
        throw new TypeError(`Memory consolidation schemaVersion must be ${MEMORY_CONSOLIDATION_SCHEMA_VERSION}`);
    }
    const ids = structuredContext(value, context);
    const layer = boundedStructuredText(
        value.layer,
        'Memory consolidation layer',
        STRUCTURED_TURN_LIMITS.memoryConsolidationLayer
    );
    const hasKey = value.key !== undefined && value.key !== null && value.key !== '';
    const hasClaim = value.claim !== undefined && value.claim !== null && value.claim !== '';
    if (hasKey === hasClaim) {
        throw new TypeError('Memory consolidation candidate must provide exactly one of key/value or claim');
    }
    const key = hasKey ? boundedStructuredText(value.key, 'Memory consolidation key', STRUCTURED_TURN_LIMITS.key) : null;
    const claim = hasClaim
        ? boundedStructuredText(value.claim, 'Memory consolidation claim', STRUCTURED_TURN_LIMITS.memoryConsolidationClaim)
        : null;
    if (key !== null && value.value === undefined) throw new TypeError('Memory consolidation value must be provided with key');
    if (claim !== null && value.value !== undefined) throw new TypeError('Memory consolidation claim cannot include value');
    const normalizedValue = key === null ? null : boundedJsonValue(value.value, 'Memory consolidation value');
    const evidenceRefs = boundedStructuredArray(
        value.evidenceRefs,
        'Memory consolidation evidenceRefs',
        STRUCTURED_TURN_LIMITS.memoryConsolidationEvidenceRefs
    ).map((item, index) => boundedStructuredText(
        item,
        `Memory consolidation evidenceRefs ${index}`,
        STRUCTURED_TURN_LIMITS.sourceId
    ));
    const sourceFactRefs = boundedStructuredArray(
        value.sourceFactRefs ?? value.sourceRefs ?? value.factRefs,
        'Memory consolidation sourceFactRefs',
        STRUCTURED_TURN_LIMITS.memoryConsolidationSourceFactRefs
    ).map((item, index) => boundedStructuredText(
        item,
        `Memory consolidation sourceFactRefs ${index}`,
        STRUCTURED_TURN_LIMITS.sourceId
    ));
    if (!evidenceRefs.length && !sourceFactRefs.length) {
        throw new TypeError('Memory consolidation candidate requires evidenceRefs or sourceFactRefs');
    }
    const status = assertOneOf(
        value.status ?? 'candidate',
        MEMORY_CONSOLIDATION_STATUSES,
        'Memory consolidation status'
    );
    const revision = value.revision === undefined ? 1 : value.revision;
    if (!Number.isInteger(revision) || revision < 1) {
        throw new TypeError('Memory consolidation revision must be a positive integer');
    }
    const interactionFactId = value.interactionFactId === undefined || value.interactionFactId === null
        ? null
        : boundedStructuredText(value.interactionFactId, 'Memory consolidation interactionFactId', STRUCTURED_TURN_LIMITS.id);
    return {
        schemaVersion: MEMORY_CONSOLIDATION_SCHEMA_VERSION,
        layer,
        key,
        value: normalizedValue,
        claim,
        confidence: boundedConfidence(value.confidence, 'Memory consolidation confidence'),
        evidenceRefs,
        sourceFactRefs,
        personaId: ids.personaId,
        sourceMessageId: ids.sourceMessageId,
        causationId: ids.causationId,
        idempotencyKey: structuredIdempotencyKey(value.idempotencyKey, 'Memory consolidation idempotencyKey'),
        source: boundedStructuredText(value.source ?? value.sourceType ?? 'llm', 'Memory consolidation source', STRUCTURED_TURN_LIMITS.sourceType),
        modelVersion: normalizeOptionalModelVersion(value.modelVersion, ids.modelVersion),
        status,
        revision,
        interactionFactId
    };
}

/**
 * Normalize an LLM-owned self-model claim. Semantic fields remain opaque to
 * the server: this boundary only limits their shape, binds the claim to the
 * application scope, and preserves the supplied evidence/uncertainty.
 */
export function normalizeSelfModelClaim(value, context = {}) {
    if (!isRecord(value)) throw new TypeError('Self-model claim must be an object');
    assertStructuredKeys(value, [
        'schemaVersion', 'version', 'category', 'claim', 'summary', 'confidence',
        'evidenceRefs', 'evidence', 'source', 'sourceType', 'uncertainty',
        'personaId', 'sourceMessageId', 'causationId', 'idempotencyKey',
        'modelVersion', 'interactionFactId', 'status', 'revision', 'decayPolicy'
    ], 'Self-model claim');
    const schemaVersion = value.schemaVersion ?? value.version;
    if (schemaVersion !== SELF_MODEL_CLAIM_SCHEMA_VERSION) {
        throw new TypeError(`Self-model claim schemaVersion must be ${SELF_MODEL_CLAIM_SCHEMA_VERSION}`);
    }
    const ids = structuredContext(value, context);
    const category = boundedStructuredText(value.category, 'Self-model claim category', STRUCTURED_TURN_LIMITS.selfModelCategory);
    const claim = boundedStructuredText(value.claim, 'Self-model claim claim', STRUCTURED_TURN_LIMITS.selfModelClaim);
    const summary = boundedStructuredText(value.summary, 'Self-model claim summary', STRUCTURED_TURN_LIMITS.selfModelSummary);
    if (value.source === undefined && value.sourceType === undefined) throw new TypeError('Self-model claim source must be provided');
    if (value.uncertainty === undefined) throw new TypeError('Self-model claim uncertainty must be provided');
    const evidenceInput = value.evidenceRefs ?? value.evidence;
    const evidenceRefs = boundedStructuredArray(
        evidenceInput,
        'Self-model claim evidenceRefs',
        STRUCTURED_TURN_LIMITS.selfModelEvidenceRefs
    ).map((item, index) => boundedStructuredText(
        item,
        `Self-model claim evidenceRefs ${index}`,
        STRUCTURED_TURN_LIMITS.sourceId
    ));
    if (!evidenceRefs.length) throw new TypeError('Self-model claim requires evidenceRefs');
    const uncertainty = boundedJsonValue(value.uncertainty, 'Self-model claim uncertainty');
    if (uncertainty !== null && JSON.stringify(uncertainty).length > STRUCTURED_TURN_LIMITS.selfModelUncertainty) {
        throw new RangeError(`Self-model claim uncertainty exceeds ${STRUCTURED_TURN_LIMITS.selfModelUncertainty} bytes`);
    }
    const status = assertOneOf(value.status ?? 'candidate', SELF_MODEL_CLAIM_STATUSES, 'Self-model claim status');
    const revision = value.revision === undefined ? 1 : value.revision;
    if (!Number.isInteger(revision) || revision < 1) throw new TypeError('Self-model claim revision must be a positive integer');
    const decayPolicy = value.decayPolicy === undefined || value.decayPolicy === null
        ? null
        : boundedJsonValue(value.decayPolicy, 'Self-model claim decayPolicy');
    if (decayPolicy !== null && JSON.stringify(decayPolicy).length > STRUCTURED_TURN_LIMITS.selfModelDecayPolicy) {
        throw new RangeError(`Self-model claim decayPolicy exceeds ${STRUCTURED_TURN_LIMITS.selfModelDecayPolicy} bytes`);
    }
    return {
        schemaVersion: SELF_MODEL_CLAIM_SCHEMA_VERSION,
        category,
        claim,
        summary,
        confidence: boundedConfidence(value.confidence, 'Self-model claim confidence'),
        evidenceRefs,
        source: boundedStructuredText(value.source ?? value.sourceType, 'Self-model claim source', STRUCTURED_TURN_LIMITS.selfModelSource),
        uncertainty,
        personaId: ids.personaId,
        sourceMessageId: ids.sourceMessageId,
        causationId: ids.causationId,
        idempotencyKey: structuredIdempotencyKey(value.idempotencyKey, 'Self-model claim idempotencyKey'),
        modelVersion: normalizeOptionalModelVersion(value.modelVersion, ids.modelVersion),
        interactionFactId: value.interactionFactId === undefined || value.interactionFactId === null
            ? null
            : boundedStructuredText(value.interactionFactId, 'Self-model claim interactionFactId', STRUCTURED_TURN_LIMITS.id),
        status,
        revision,
        decayPolicy
    };
}

/**
 * Normalize an LLM-owned agency intention. Qualification and delivery are
 * lifecycle states; the server never infers an intention from visible text.
 */
export function normalizeAgencyIntention(value, context = {}) {
    if (!isRecord(value)) throw new TypeError('Agency intention must be an object');
    assertStructuredKeys(value, [
        'schemaVersion', 'version', 'intent', 'topic', 'explanation', 'reason',
        'reasonCategory', 'confidence', 'evidenceRefs', 'source', 'personaId',
        'sourceMessageId', 'causationId', 'idempotencyKey', 'modelVersion',
        'status', 'revision', 'decision', 'interactionFactId'
    ], 'Agency intention');
    const schemaVersion = value.schemaVersion ?? value.version;
    if (schemaVersion !== AGENCY_INTENTION_SCHEMA_VERSION) {
        throw new TypeError(`Agency intention schemaVersion must be ${AGENCY_INTENTION_SCHEMA_VERSION}`);
    }
    const ids = structuredContext(value, context);
    const intent = boundedStructuredText(value.intent, 'Agency intention intent', STRUCTURED_TURN_LIMITS.agencyIntent);
    const topic = value.topic === undefined || value.topic === null || value.topic === ''
        ? null
        : boundedStructuredText(value.topic, 'Agency intention topic', STRUCTURED_TURN_LIMITS.agencyTopic);
    const explanation = boundedStructuredText(
        value.explanation ?? value.reason ?? '',
        'Agency intention explanation',
        STRUCTURED_TURN_LIMITS.agencyExplanation,
        {allowEmpty: true}
    );
    const reasonCategory = value.reasonCategory === undefined || value.reasonCategory === null || value.reasonCategory === ''
        ? null
        : boundedStructuredText(value.reasonCategory, 'Agency intention reasonCategory', STRUCTURED_TURN_LIMITS.agencyReason);
    const evidenceRefs = boundedStructuredArray(
        value.evidenceRefs,
        'Agency intention evidenceRefs',
        STRUCTURED_TURN_LIMITS.agencyEvidenceRefs
    ).map((item, index) => boundedStructuredText(item, `Agency intention evidenceRefs ${index}`, STRUCTURED_TURN_LIMITS.sourceId));
    if (!evidenceRefs.length) throw new TypeError('Agency intention requires evidenceRefs');
    const status = assertOneOf(value.status ?? 'candidate', AGENCY_INTENTION_STATUSES, 'Agency intention status');
    const revision = value.revision === undefined ? 1 : value.revision;
    if (!Number.isInteger(revision) || revision < 1) throw new TypeError('Agency intention revision must be a positive integer');
    return {
        schemaVersion: AGENCY_INTENTION_SCHEMA_VERSION,
        intent,
        topic,
        explanation,
        reasonCategory,
        confidence: boundedConfidence(value.confidence, 'Agency intention confidence'),
        evidenceRefs,
        source: boundedStructuredText(value.source ?? 'llm', 'Agency intention source', STRUCTURED_TURN_LIMITS.sourceType),
        personaId: ids.personaId,
        sourceMessageId: ids.sourceMessageId,
        causationId: ids.causationId,
        idempotencyKey: structuredIdempotencyKey(value.idempotencyKey, 'Agency intention idempotencyKey'),
        modelVersion: normalizeOptionalModelVersion(value.modelVersion, ids.modelVersion),
        status,
        revision,
        decision: value.decision === undefined ? null : boundedJsonValue(value.decision, 'Agency intention decision'),
        interactionFactId: value.interactionFactId === undefined || value.interactionFactId === null
            ? null
            : boundedStructuredText(value.interactionFactId, 'Agency intention interactionFactId', STRUCTURED_TURN_LIMITS.id)
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
        appraisals: control.appraisals ?? control.appraisal ?? value.appraisals ?? value.appraisal,
        memoryConsolidations: control.memoryConsolidations ?? value.memoryConsolidations,
        selfModelClaims: control.selfModelClaims ?? control.self_model_claims ?? value.selfModelClaims ?? value.self_model_claims,
        agencyIntentions: control.agencyIntentions ?? control.agency ?? value.agencyIntentions ?? value.agency,
        capabilityCalls: control.capabilityCalls ?? value.capabilityCalls ?? value.toolCalls
    };
}

function sourceModeFor(value, control) {
    if (value.sourceMode !== undefined) return assertOneOf(value.sourceMode, STRUCTURED_TURN_SOURCE_MODES, 'Structured turn sourceMode');
    if ((control.capabilityCalls || []).some(call => call?.source === 'native')) return 'native_tools';
    const hasStructuredControl = ['affectEvents', 'driveSignals', 'memoryWrites', 'appraisals', 'memoryConsolidations', 'selfModelClaims', 'agencyIntentions', 'capabilityCalls']
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
    assertStructuredKeys(value, ['schemaVersion', 'version', 'text', 'tokens', 'messages', 'control', 'affectEvents', 'driveSignals', 'memoryWrites', 'appraisals', 'appraisal', 'memoryConsolidations', 'selfModelClaims', 'self_model_claims', 'agencyIntentions', 'agency', 'capabilityCalls', 'parseDiagnostics', 'diagnostics', 'sourceMode', 'structuredSidecar', 'structuredTurn', 'structured_turn'], 'Structured turn');
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
    if (isRecord(value.control)) assertStructuredKeys(value.control, ['affectEvents', 'driveSignals', 'memoryWrites', 'appraisals', 'appraisal', 'memoryConsolidations', 'selfModelClaims', 'self_model_claims', 'agencyIntentions', 'agency', 'capabilityCalls'], 'Structured turn control');
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
    const appraisalByKey = new Map();
    for (const candidate of boundedStructuredArray(controlInput.appraisals, 'Structured turn appraisals', STRUCTURED_TURN_LIMITS.appraisals)) {
        const appraisal = normalizeAppraisalCandidate(candidate, ids);
        if (!appraisalByKey.has(appraisal.idempotencyKey)) appraisalByKey.set(appraisal.idempotencyKey, appraisal);
    }
    const appraisals = [...appraisalByKey.values()];
    for (const appraisal of appraisals) {
        for (const candidate of appraisal.affectEvents) {
            if (!affectEvents.some(existing => existing.idempotencyKey === candidate.idempotencyKey)) affectEvents.push(candidate);
        }
        for (const candidate of appraisal.driveSignals) {
            if (!driveSignals.some(existing => existing.idempotencyKey === candidate.idempotencyKey)) driveSignals.push(candidate);
        }
    }
    affectEvents.splice(STRUCTURED_TURN_LIMITS.affectEvents);
    driveSignals.splice(STRUCTURED_TURN_LIMITS.driveSignals);
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
    const consolidationByKey = new Map();
    for (const candidate of boundedStructuredArray(
        controlInput.memoryConsolidations,
        'Structured turn memoryConsolidations',
        STRUCTURED_TURN_LIMITS.memoryConsolidations
    )) {
        const consolidation = normalizeMemoryConsolidationCandidate(candidate, ids);
        if (!consolidationByKey.has(consolidation.idempotencyKey)) consolidationByKey.set(consolidation.idempotencyKey, consolidation);
    }
    const selfModelByKey = new Map();
    for (const candidate of boundedStructuredArray(
        controlInput.selfModelClaims,
        'Structured turn selfModelClaims',
        STRUCTURED_TURN_LIMITS.selfModelClaims
    )) {
        const claim = normalizeSelfModelClaim(candidate, ids);
        if (!selfModelByKey.has(claim.idempotencyKey)) selfModelByKey.set(claim.idempotencyKey, claim);
    }
    const agencyByKey = new Map();
    for (const candidate of boundedStructuredArray(
        controlInput.agencyIntentions,
        'Structured turn agencyIntentions',
        STRUCTURED_TURN_LIMITS.agencyIntentions
    )) {
        const intention = normalizeAgencyIntention(candidate, ids);
        if (!agencyByKey.has(intention.idempotencyKey)) agencyByKey.set(intention.idempotencyKey, intention);
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
            appraisals,
            memoryConsolidations: [...consolidationByKey.values()],
            selfModelClaims: [...selfModelByKey.values()],
            agencyIntentions: [...agencyByKey.values()],
            capabilityCalls
        },
        parseDiagnostics,
        sourceMode: sourceModeFor(value, {
            affectEvents,
            driveSignals,
            memoryWrites: [...memoryByKey.values()],
            appraisals,
            memoryConsolidations: [...consolidationByKey.values()],
            selfModelClaims: [...selfModelByKey.values()],
            agencyIntentions: [...agencyByKey.values()],
            capabilityCalls: normalizedCalls
        })
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
                control: {affectEvents: [], driveSignals: [], memoryWrites: [], appraisals: [], memoryConsolidations: [], selfModelClaims: [], agencyIntentions: [], capabilityCalls: []},
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
export const normalizeSelfModelClaimCandidate = normalizeSelfModelClaim;
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
        appraisal: {
            type: 'object',
            additionalProperties: false,
            required: ['schemaVersion', 'category', 'confidence', 'idempotencyKey'],
            properties: {
                schemaVersion: {const: APPRAISAL_SCHEMA_VERSION},
                category: {type: 'string', minLength: 1, maxLength: STRUCTURED_TURN_LIMITS.appraisalCategory},
                confidence: {type: 'number', minimum: 0, maximum: 1},
                rationale: {type: 'string', maxLength: STRUCTURED_TURN_LIMITS.appraisalRationale},
                reason: {type: 'string', maxLength: STRUCTURED_TURN_LIMITS.appraisalRationale},
                evidenceRefs: {type: 'array', maxItems: STRUCTURED_TURN_LIMITS.appraisalEvidenceRefs, items: {type: 'string', maxLength: STRUCTURED_TURN_LIMITS.sourceId}},
                affectEvents: {type: 'array', maxItems: STRUCTURED_TURN_LIMITS.affectEvents, items: {$ref: '#/$defs/affectEvent'}},
                driveSignals: {type: 'array', maxItems: STRUCTURED_TURN_LIMITS.driveSignals, items: {$ref: '#/$defs/driveSignal'}},
                personaId: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.id},
                sourceMessageId: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.sourceId},
                causationId: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.id},
                idempotencyKey: {type: 'string', minLength: 1, maxLength: STRUCTURED_TURN_LIMITS.idempotencyKey},
                modelVersion: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.modelVersion},
                source: {type: 'string', maxLength: STRUCTURED_TURN_LIMITS.sourceType},
                interactionFactId: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.id}
            }
        },
        memoryConsolidation: {
            type: 'object',
            additionalProperties: false,
            required: ['schemaVersion', 'layer', 'confidence', 'idempotencyKey'],
            properties: {
                schemaVersion: {const: MEMORY_CONSOLIDATION_SCHEMA_VERSION},
                layer: {type: 'string', minLength: 1, maxLength: STRUCTURED_TURN_LIMITS.memoryConsolidationLayer},
                key: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.key},
                value: {},
                claim: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.memoryConsolidationClaim},
                confidence: {type: 'number', minimum: 0, maximum: 1},
                evidenceRefs: {type: 'array', maxItems: STRUCTURED_TURN_LIMITS.memoryConsolidationEvidenceRefs, items: {type: 'string', maxLength: STRUCTURED_TURN_LIMITS.sourceId}},
                sourceFactRefs: {type: 'array', maxItems: STRUCTURED_TURN_LIMITS.memoryConsolidationSourceFactRefs, items: {type: 'string', maxLength: STRUCTURED_TURN_LIMITS.sourceId}},
                personaId: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.id},
                sourceMessageId: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.sourceId},
                causationId: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.id},
                idempotencyKey: {type: 'string', minLength: 1, maxLength: STRUCTURED_TURN_LIMITS.idempotencyKey},
                source: {type: 'string', maxLength: STRUCTURED_TURN_LIMITS.sourceType},
                modelVersion: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.modelVersion},
                status: {enum: MEMORY_CONSOLIDATION_STATUSES},
                revision: {type: 'integer', minimum: 1},
                interactionFactId: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.id}
            }
        },
        selfModelClaim: {
            type: 'object',
            additionalProperties: false,
            required: ['schemaVersion', 'category', 'claim', 'summary', 'confidence', 'evidenceRefs', 'source', 'uncertainty', 'idempotencyKey'],
            properties: {
                schemaVersion: {const: SELF_MODEL_CLAIM_SCHEMA_VERSION},
                category: {type: 'string', minLength: 1, maxLength: STRUCTURED_TURN_LIMITS.selfModelCategory},
                claim: {type: 'string', minLength: 1, maxLength: STRUCTURED_TURN_LIMITS.selfModelClaim},
                summary: {type: 'string', minLength: 1, maxLength: STRUCTURED_TURN_LIMITS.selfModelSummary},
                confidence: {type: 'number', minimum: 0, maximum: 1},
                evidenceRefs: {type: 'array', minItems: 1, maxItems: STRUCTURED_TURN_LIMITS.selfModelEvidenceRefs, items: {type: 'string', maxLength: STRUCTURED_TURN_LIMITS.sourceId}},
                source: {type: 'string', minLength: 1, maxLength: STRUCTURED_TURN_LIMITS.selfModelSource},
                uncertainty: {},
                personaId: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.id},
                sourceMessageId: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.sourceId},
                causationId: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.id},
                idempotencyKey: {type: 'string', minLength: 1, maxLength: STRUCTURED_TURN_LIMITS.idempotencyKey},
                modelVersion: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.modelVersion},
                interactionFactId: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.id},
                status: {enum: SELF_MODEL_CLAIM_STATUSES},
                revision: {type: 'integer', minimum: 1},
                decayPolicy: {}
            }
        },
        agencyIntention: {
            type: 'object',
            additionalProperties: false,
            required: ['schemaVersion', 'intent', 'confidence', 'evidenceRefs', 'idempotencyKey'],
            properties: {
                schemaVersion: {const: AGENCY_INTENTION_SCHEMA_VERSION},
                intent: {type: 'string', minLength: 1, maxLength: STRUCTURED_TURN_LIMITS.agencyIntent},
                topic: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.agencyTopic},
                explanation: {type: 'string', maxLength: STRUCTURED_TURN_LIMITS.agencyExplanation},
                reason: {type: 'string', maxLength: STRUCTURED_TURN_LIMITS.agencyExplanation},
                reasonCategory: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.agencyReason},
                confidence: {type: 'number', minimum: 0, maximum: 1},
                evidenceRefs: {type: 'array', minItems: 1, maxItems: STRUCTURED_TURN_LIMITS.agencyEvidenceRefs, items: {type: 'string', maxLength: STRUCTURED_TURN_LIMITS.sourceId}},
                source: {type: 'string', maxLength: STRUCTURED_TURN_LIMITS.sourceType},
                personaId: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.id},
                sourceMessageId: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.sourceId},
                causationId: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.id},
                idempotencyKey: {type: 'string', minLength: 1, maxLength: STRUCTURED_TURN_LIMITS.idempotencyKey},
                modelVersion: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.modelVersion},
                status: {enum: AGENCY_INTENTION_STATUSES},
                revision: {type: 'integer', minimum: 1},
                decision: {},
                interactionFactId: {type: ['string', 'null'], maxLength: STRUCTURED_TURN_LIMITS.id}
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
                appraisals: {type: 'array', maxItems: STRUCTURED_TURN_LIMITS.appraisals, items: {$ref: '#/$defs/appraisal'}},
                memoryConsolidations: {type: 'array', maxItems: STRUCTURED_TURN_LIMITS.memoryConsolidations, items: {$ref: '#/$defs/memoryConsolidation'}},
                selfModelClaims: {type: 'array', maxItems: STRUCTURED_TURN_LIMITS.selfModelClaims, items: {$ref: '#/$defs/selfModelClaim'}},
                agencyIntentions: {type: 'array', maxItems: STRUCTURED_TURN_LIMITS.agencyIntentions, items: {$ref: '#/$defs/agencyIntention'}},
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

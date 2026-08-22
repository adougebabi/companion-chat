export const MEMORY_EVENT_SCHEMA_VERSION = 1;
export const MEMORY_EVENT_MAX_KEY_LENGTH = 120;
export const MEMORY_EVENT_MAX_VALUE_LENGTH = 2_000;
export const MEMORY_EVENT_MAX_SOURCE_LENGTH = 80;
export const MEMORY_EVENT_MAX_SOURCE_ID_LENGTH = 240;
export const MEMORY_EVENT_MAX_IDEMPOTENCY_LENGTH = 240;
export const MEMORY_EVENT_MAX_PAYLOAD_BYTES = 8_192;

const MAX_ID_LENGTH = 240;
const MEMORY_OPERATIONS = new Set(['insert', 'upsert']);

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function statusError(message, status) {
    return Object.assign(new Error(message), {status});
}

function boundedText(value, field, maxLength, {allowEmpty = false} = {}) {
    if (typeof value !== 'string') throw statusError(`${field}必须是字符串`, 400);
    const normalized = value.trim();
    if (!allowEmpty && normalized === '') throw statusError(`${field}不能为空`, 400);
    if (normalized.length > maxLength) throw statusError(`${field}不能超过 ${maxLength} 个字符`, 400);
    return normalized;
}

function confidenceValue(value) {
    const confidence = Number(value);
    if (!Number.isFinite(confidence) || confidence < 0 || confidence > 1) {
        throw statusError('记忆置信度必须在 0 到 1 之间', 400);
    }
    return confidence;
}

function supportedFields(value) {
    const fields = new Set([
        'schemaVersion', 'schema_version', 'operation', 'memoryKey', 'memory_key', 'key',
        'value', 'confidence', 'sourceType', 'source_type', 'source', 'sourceId', 'source_id',
        'sourceMessageId', 'source_message_id',
        'idempotencyKey', 'idempotency_key'
    ]);
    for (const field of Object.keys(value)) {
        if (!fields.has(field)) throw statusError(`memory_event 包含不支持的字段: ${field}`, 400);
    }
}

/**
 * Validate the model-owned memory_event payload without touching storage.
 * Source IDs and idempotency keys are checked against the turn context in the
 * memory flow; this helper only normalizes the bounded event shape.
 */
export function normalizeMemoryEventCall(value, {sourceMessageId, idempotencyKey} = {}) {
    if (!isRecord(value)) throw statusError('memory_event 参数必须是 JSON 对象', 400);
    supportedFields(value);
    const schemaVersion = value.schemaVersion ?? value.schema_version ?? MEMORY_EVENT_SCHEMA_VERSION;
    if (schemaVersion !== MEMORY_EVENT_SCHEMA_VERSION) throw statusError('memory_event schemaVersion 不受支持', 400);
    const operation = value.operation === undefined ? 'upsert' : value.operation;
    if (!MEMORY_OPERATIONS.has(operation)) throw statusError('memory_event operation 不受支持', 400);
    const memoryKey = boundedText(
        value.memoryKey ?? value.memory_key ?? value.key,
        '记忆键', MEMORY_EVENT_MAX_KEY_LENGTH
    );
    const memoryValue = boundedText(value.value, '记忆内容', MEMORY_EVENT_MAX_VALUE_LENGTH);
    const confidence = confidenceValue(value.confidence);
    const sourceType = boundedText(
        value.sourceType ?? value.source_type ?? value.source ?? 'structured_turn',
        '记忆来源', MEMORY_EVENT_MAX_SOURCE_LENGTH
    );
    const providedSourceId = value.sourceId ?? value.source_id ?? value.sourceMessageId ?? value.source_message_id;
    const normalizedSourceId = providedSourceId === undefined || providedSourceId === null
        ? sourceMessageId
        : boundedText(providedSourceId, '记忆来源 ID', MEMORY_EVENT_MAX_SOURCE_ID_LENGTH);
    if (sourceMessageId !== undefined && sourceMessageId !== null
        && normalizedSourceId !== sourceMessageId) {
        throw statusError('记忆来源必须是当前回合的用户消息', 400);
    }
    const providedIdempotency = value.idempotencyKey ?? value.idempotency_key;
    const normalizedIdempotency = providedIdempotency === undefined || providedIdempotency === null
        ? idempotencyKey
        : boundedText(providedIdempotency, '记忆幂等键', MEMORY_EVENT_MAX_IDEMPOTENCY_LENGTH);
    if (idempotencyKey !== undefined && idempotencyKey !== null
        && normalizedIdempotency !== idempotencyKey) {
        throw statusError('记忆幂等键与 capability provenance 不一致', 400);
    }
    if (!normalizedSourceId) throw statusError('记忆来源消息不能为空', 400);
    if (!normalizedIdempotency) throw statusError('记忆幂等键不能为空', 400);
    const normalized = {
        schemaVersion: MEMORY_EVENT_SCHEMA_VERSION,
        operation,
        memoryKey,
        value: memoryValue,
        confidence,
        sourceType,
        sourceId: boundedText(normalizedSourceId, '记忆来源 ID', MEMORY_EVENT_MAX_SOURCE_ID_LENGTH),
        idempotencyKey: boundedText(normalizedIdempotency, '记忆幂等键', MEMORY_EVENT_MAX_IDEMPOTENCY_LENGTH)
    };
    if (Buffer.byteLength(JSON.stringify(normalized), 'utf8') > MEMORY_EVENT_MAX_PAYLOAD_BYTES) {
        throw statusError('memory_event 数据超过允许大小', 400);
    }
    return Object.freeze(normalized);
}

function requiredText(value, field) {
    if (typeof value !== 'string' || value.trim() === '') throw statusError(`${field}不能为空`, 400);
    const normalized = value.trim();
    if (normalized.length > MAX_ID_LENGTH) throw statusError(`${field}不能超过 ${MAX_ID_LENGTH} 个字符`, 400);
    return normalized;
}

function sourceFor(options, names) {
    const repositories = isRecord(options.repositories) ? options.repositories : {};
    for (const source of [options, repositories]) {
        for (const name of names) if (source[name] !== undefined) return source[name];
    }
    return undefined;
}

function methodFor(port, names, field) {
    if (typeof port === 'function') return port;
    if (isRecord(port)) {
        for (const name of names) if (typeof port[name] === 'function') return port[name].bind(port);
    }
    throw statusError(`记忆服务缺少 ${field} port`, 501);
}

function clockFor(value) {
    if (typeof value === 'function') return value;
    if (isRecord(value) && typeof value.now === 'function') return value.now.bind(value);
    return () => new Date().toISOString();
}

/**
 * Persona-private memory mutations stay behind the memory repository port.
 * The service only validates ownership and translates a no-op into 404.
 */
export function createMemoryService(options = {}) {
    if (!isRecord(options)) throw new TypeError('Memory service options must be an object');
    const memories = sourceFor(options, ['memory', 'memories', 'memoryRepository', 'memoryPort']);
    const personas = sourceFor(options, ['persona', 'personas', 'personaRepository', 'personaPort']);
    const now = clockFor(options.clock ?? options.now);
    const findPersona = personas && (typeof personas === 'function' ? personas : personas.findActive ?? personas.find ?? personas.get);

    function deleteMemory(command = {}) {
        if (!isRecord(command)) throw statusError('记忆删除请求必须是 JSON 对象', 400);
        const personaId = requiredText(command.personaId ?? command.persona_id, '人格 ID');
        const memoryId = requiredText(command.memoryId ?? command.memory_id ?? command.id, '记忆 ID');
        if (typeof findPersona === 'function') {
            const owner = findPersona.call(personas, personaId);
            if (owner && typeof owner.then === 'function') throw new TypeError('Memory persona lookup must be synchronous');
            if (!owner) throw statusError('人格不存在', 404);
        }
        const remove = methodFor(memories, ['deleteMemory', 'remove', 'delete', 'destroy'], 'delete');
        const result = remove({
            ...command,
            personaId,
            memoryId,
            updatedAt: command.updatedAt ?? now()
        });
        if (result && typeof result.then === 'function') {
            return result.then(value => {
                const changes = typeof value === 'number' ? value : value?.changes;
                if (changes === 0) throw statusError('记忆不存在', 404);
                return value;
            });
        }
        const changes = typeof result === 'number' ? result : result?.changes;
        if (changes === 0) throw statusError('记忆不存在', 404);
        return result;
    }

    return Object.freeze({
        delete: deleteMemory,
        deleteMemory,
        remove: deleteMemory,
        normalizeMemoryEvent: normalizeMemoryEventCall,
        normalizeCall: normalizeMemoryEventCall,
        validateMemoryEvent: normalizeMemoryEventCall,
        validate: normalizeMemoryEventCall
    });
}

export const createMemoryApplicationService = createMemoryService;
export default createMemoryService;

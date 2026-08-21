const MAX_ID_LENGTH = 240;

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function statusError(message, status) {
    return Object.assign(new Error(message), {status});
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

    return Object.freeze({delete: deleteMemory, deleteMemory, remove: deleteMemory});
}

export const createMemoryApplicationService = createMemoryService;
export default createMemoryService;

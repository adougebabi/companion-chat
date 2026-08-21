function assertOpenDatabase(database) {
    if (!database || typeof database.prepare !== 'function') {
        throw new TypeError('Memory repository requires an open database');
    }
    return database;
}

function requiredText(value, field) {
    if (typeof value !== 'string' || value.length === 0) {
        throw new TypeError(`${field} must be a non-empty string`);
    }
    return value;
}

function resolveClock(clock, now) {
    const source = clock ?? now;
    if (source === undefined) return () => new Date().toISOString();
    if (typeof source === 'function') return source;
    if (source && typeof source === 'object' && typeof source.now === 'function') return source.now.bind(source);
    throw new TypeError('Memory repository clock must be a function or provide now()');
}

function timestamp(value, field) {
    if (value instanceof Date) return value.toISOString();
    return requiredText(value, field);
}

function nullableText(value, field) {
    if (value === undefined || value === null) return null;
    return requiredText(value, field);
}

function limitValue(value) {
    const limit = value === undefined ? 20 : Number(value);
    if (!Number.isInteger(limit) || limit < 1) throw new RangeError('Memory limit must be a positive integer');
    return limit;
}

/**
 * Create a table-scoped adapter for persona memories.
 *
 * Memory policy, confidence interpretation, DTO shaping, and persona
 * existence errors remain with the caller. This adapter only performs
 * parameterized, persona-scoped SQL and returns raw rows/update results.
 */
export function createMemoryRepository({database, clock, now} = {}) {
    const openDatabase = assertOpenDatabase(database);
    const currentTime = resolveClock(clock, now);

    function listActive({personaId, limit} = {}) {
        const personaValue = requiredText(personaId, 'Persona.id');
        return openDatabase.prepare(`
            SELECT * FROM companion_memories
            WHERE persona_id = ? AND status = 'active'
            ORDER BY updated_at DESC, id DESC
            LIMIT ?
        `).all(personaValue, limitValue(limit));
    }

    function remove({personaId, memoryId, updatedAt} = {}) {
        const personaValue = requiredText(personaId, 'Persona.id');
        const memoryValue = requiredText(memoryId, 'Memory.id');
        const updatedValue = timestamp(updatedAt ?? currentTime(), 'Memory.updatedAt');
        return openDatabase.prepare(`
            UPDATE companion_memories
            SET status = 'deleted', updated_at = ?
            WHERE id = ? AND persona_id = ? AND status = 'active'
        `).run(updatedValue, memoryValue, personaValue);
    }

    function insert({
        id, personaId, memoryKey, value, confidence, status = 'active', sourceType,
        sourceId, createdAt, updatedAt, supersededAt
    } = {}) {
        const idValue = requiredText(id, 'Memory.id');
        const personaValue = requiredText(personaId, 'Persona.id');
        const keyValue = requiredText(memoryKey, 'Memory.memoryKey');
        const valueValue = requiredText(value, 'Memory.value');
        const confidenceValue = Number(confidence);
        if (!Number.isFinite(confidenceValue)) throw new TypeError('Memory.confidence must be finite');
        const statusValue = requiredText(status, 'Memory.status');
        const createdValue = timestamp(createdAt ?? currentTime(), 'Memory.createdAt');
        const updatedValue = timestamp(updatedAt ?? createdValue, 'Memory.updatedAt');
        const supersededValue = nullableText(supersededAt, 'Memory.supersededAt');
        openDatabase.prepare(`
            INSERT INTO companion_memories (
                id, persona_id, memory_key, value, confidence, status, source_type,
                source_id, created_at, updated_at, superseded_at
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        `).run(
            idValue, personaValue, keyValue, valueValue, confidenceValue, statusValue,
            nullableText(sourceType, 'Memory.sourceType'), nullableText(sourceId, 'Memory.sourceId'),
            createdValue, updatedValue, supersededValue
        );
        return openDatabase.prepare('SELECT * FROM companion_memories WHERE id = ?').get(idValue);
    }

    return Object.freeze({listActive, insert, insertMemory: insert, delete: remove});
}

export const createCompanionMemoryRepository = createMemoryRepository;
export default createMemoryRepository;

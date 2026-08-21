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

    return Object.freeze({listActive, delete: remove});
}

export const createCompanionMemoryRepository = createMemoryRepository;
export default createMemoryRepository;

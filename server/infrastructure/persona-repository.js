function assertOpenDatabase(database) {
    if (!database || typeof database.prepare !== 'function') {
        throw new TypeError('Persona repository requires an open database');
    }
    return database;
}

function requiredText(value, field) {
    if (typeof value !== 'string' || value.length === 0) {
        throw new TypeError(`${field} must be a non-empty string`);
    }
    return value;
}

function nullableText(value, field) {
    if (value === null || value === undefined) return null;
    return requiredText(value, field);
}

function resolveClock(clock, now) {
    const source = clock ?? now;
    if (source === undefined) return () => new Date().toISOString();
    if (typeof source === 'function') return source;
    if (source && typeof source === 'object' && typeof source.now === 'function') return source.now.bind(source);
    throw new TypeError('Persona repository clock must be a function or provide now()');
}

function timestamp(value, field) {
    if (value instanceof Date) return value.toISOString();
    return requiredText(value, field);
}

/**
 * Create a table-scoped adapter for active persona rows.
 *
 * The caller owns persona policy, summary shaping, group joins, and message
 * read behavior. This adapter only performs parameterized SQL against the
 * already-open companion_personas table and returns raw rows.
 */
export function createPersonaRepository({database, clock, now, id} = {}) {
    const openDatabase = assertOpenDatabase(database);
    const currentTime = resolveClock(clock, now);
    // The optional id dependency is accepted for composition compatibility;
    // these read/update operations do not create persona identifiers.
    void id;

    function findActive(personaId) {
        const idValue = requiredText(personaId, 'Persona.id');
        return openDatabase.prepare(`
            SELECT * FROM companion_personas
            WHERE id = ? AND enabled = 1 AND deleted_at IS NULL
        `).get(idValue);
    }

    function listActive() {
        return openDatabase.prepare(`
            SELECT * FROM companion_personas
            WHERE enabled = 1 AND deleted_at IS NULL
            ORDER BY created_at, id
        `).all();
    }

    function updateScreen({personaId, screenedAt, updatedAt} = {}) {
        const idValue = requiredText(personaId, 'Persona.id');
        const screenValue = nullableText(screenedAt, 'Persona.screenedAt');
        const updatedValue = timestamp(updatedAt ?? currentTime(), 'Persona.updatedAt');
        openDatabase.prepare(`
            UPDATE companion_personas
            SET screened_at = ?, updated_at = ?
            WHERE id = ?
        `).run(screenValue, updatedValue, idValue);
        return openDatabase.prepare('SELECT * FROM companion_personas WHERE id = ?').get(idValue);
    }

    return Object.freeze({findActive, listActive, updateScreen});
}

export const createCompanionPersonaRepository = createPersonaRepository;
export default createPersonaRepository;

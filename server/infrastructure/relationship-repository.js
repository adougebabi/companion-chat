import {randomUUID} from 'node:crypto';

function assertOpenDatabase(database) {
    if (!database || typeof database.prepare !== 'function') {
        throw new TypeError('Relationship repository requires an open database');
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
    throw new TypeError('Relationship repository clock must be a function or provide now()');
}

function resolveId(id, idGenerator) {
    const source = id ?? idGenerator;
    if (source === undefined) return prefix => `${prefix}_${randomUUID()}`;
    if (typeof source === 'function') return source;
    if (source && typeof source === 'object' && typeof source.next === 'function') return source.next.bind(source);
    throw new TypeError('Relationship repository id must be a function or provide next()');
}

function timestamp(value, field) {
    if (value instanceof Date) return value.toISOString();
    return requiredText(value, field);
}

function jsonValue(value, fallback) {
    if (typeof value === 'string') return value;
    return JSON.stringify(value === undefined ? fallback : value);
}

function limitValue(value) {
    const limit = value === undefined ? 12 : Number(value);
    if (!Number.isInteger(limit) || limit < 1) throw new RangeError('Relationship evolution limit must be a positive integer');
    return limit;
}

/**
 * Create a table-scoped adapter for relationship evolution records.
 *
 * Patch merging, deduplication, evidence policy, and browser summary shaping
 * remain with the caller. This adapter only serializes supplied JSON values,
 * performs parameterized SQL, and returns raw evolution rows.
 */
export function createRelationshipRepository({database, clock, now, id, idGenerator} = {}) {
    const openDatabase = assertOpenDatabase(database);
    const currentTime = resolveClock(clock, now);
    const generateId = resolveId(id, idGenerator);

    function activePatch({personaId} = {}) {
        const personaValue = requiredText(personaId, 'Persona.id');
        return openDatabase.prepare(`
            SELECT next_patch
            FROM companion_persona_evolutions
            WHERE persona_id = ? AND status = 'applied'
            ORDER BY created_at DESC, rowid DESC
            LIMIT 1
        `).get(personaValue);
    }

    function insertEvolution({id: evolutionId, personaId, reason, evidence, previousPatch, nextPatch, createdAt, status} = {}) {
        const idValue = requiredText(evolutionId ?? generateId('evolution'), 'Evolution.id');
        const personaValue = requiredText(personaId, 'Persona.id');
        const reasonValue = requiredText(reason, 'Evolution.reason');
        const createdValue = timestamp(createdAt ?? currentTime(), 'Evolution.createdAt');
        const statusValue = requiredText(status ?? 'applied', 'Evolution.status');
        openDatabase.prepare(`
            INSERT INTO companion_persona_evolutions
                (id, persona_id, reason, evidence_json, previous_patch, next_patch, status, created_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        `).run(
            idValue,
            personaValue,
            reasonValue,
            jsonValue(evidence, []),
            jsonValue(previousPatch, {}),
            jsonValue(nextPatch, {}),
            statusValue,
            createdValue
        );
        return openDatabase.prepare('SELECT * FROM companion_persona_evolutions WHERE id = ?').get(idValue);
    }

    function listRecent({personaId, limit} = {}) {
        const personaValue = requiredText(personaId, 'Persona.id');
        return openDatabase.prepare(`
            SELECT * FROM companion_persona_evolutions
            WHERE persona_id = ?
            ORDER BY created_at DESC, rowid DESC
            LIMIT ?
        `).all(personaValue, limitValue(limit));
    }

    return Object.freeze({activePatch, insertEvolution, listRecent});
}

export const createCompanionRelationshipRepository = createRelationshipRepository;
export default createRelationshipRepository;

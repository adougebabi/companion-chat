import {randomUUID} from 'node:crypto';

function assertOpenDatabase(database) {
    if (!database || typeof database.prepare !== 'function') {
        throw new TypeError('Group repository requires an open database');
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
    throw new TypeError('Group repository clock must be a function or provide now()');
}

function resolveId(id, idGenerator) {
    const source = id ?? idGenerator;
    if (source === undefined) return prefix => `${prefix}_${randomUUID()}`;
    if (typeof source === 'function') return source;
    if (source && typeof source === 'object' && typeof source.next === 'function') return source.next.bind(source);
    throw new TypeError('Group repository id must be a function or provide next()');
}

function timestamp(value, field) {
    if (value instanceof Date) return value.toISOString();
    return requiredText(value, field);
}

/**
 * Create a table-scoped adapter for contact groups and persona membership.
 *
 * Name policy, duplicate handling, group DTO shaping, and persona ownership
 * remain with the caller. This adapter only performs parameterized SQL and
 * returns raw group/persona rows from the already-open database.
 */
export function createGroupRepository({database, id, idGenerator, clock, now} = {}) {
    const openDatabase = assertOpenDatabase(database);
    const generateId = resolveId(id, idGenerator);
    const currentTime = resolveClock(clock, now);

    function list() {
        return openDatabase.prepare(`
            SELECT groups.id, groups.name, groups.is_default, groups.created_at, groups.updated_at,
                COUNT(personas.id) AS persona_count
            FROM companion_groups groups
            LEFT JOIN companion_personas personas
                ON personas.group_id = groups.id AND personas.enabled = 1 AND personas.deleted_at IS NULL
            GROUP BY groups.id
            ORDER BY groups.is_default DESC, groups.created_at, groups.id
        `).all();
    }

    function defaultGroup() {
        return openDatabase.prepare(`
            SELECT * FROM companion_groups
            WHERE is_default = 1
            ORDER BY created_at, id
            LIMIT 1
        `).get();
    }

    function find(groupId) {
        const idValue = requiredText(groupId, 'Group.id');
        return openDatabase.prepare('SELECT * FROM companion_groups WHERE id = ?').get(idValue);
    }

    function create({name} = {}) {
        const groupId = requiredText(generateId('group'), 'Group.id');
        const groupName = requiredText(name, 'Group.name');
        const createdAt = timestamp(currentTime(), 'Group.createdAt');
        openDatabase.prepare(`
            INSERT INTO companion_groups (id, name, is_default, created_at, updated_at)
            VALUES (?, ?, 0, ?, ?)
        `).run(groupId, groupName, createdAt, createdAt);
        return openDatabase.prepare('SELECT * FROM companion_groups WHERE id = ?').get(groupId);
    }

    function assignPersona({personaId, groupId, updatedAt} = {}) {
        const personaValue = requiredText(personaId, 'Persona.id');
        const groupValue = requiredText(groupId, 'Group.id');
        const updatedValue = timestamp(updatedAt ?? currentTime(), 'Persona.updatedAt');
        openDatabase.prepare(`
            UPDATE companion_personas
            SET group_id = ?, updated_at = ?
            WHERE id = ?
        `).run(groupValue, updatedValue, personaValue);
        return openDatabase.prepare('SELECT * FROM companion_personas WHERE id = ?').get(personaValue);
    }

    return Object.freeze({list, defaultGroup, create, find, assignPersona});
}

export const createCompanionGroupRepository = createGroupRepository;
export default createGroupRepository;

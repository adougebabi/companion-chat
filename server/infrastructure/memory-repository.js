import {createHash} from 'node:crypto';

/**
 * The original companion_memories table predates structured turns and has no
 * idempotency column. Keeping the fallback ID deterministic makes replay
 * durable without making this repository responsible for migrations.
 */
export function memoryEventIdFor(personaId, idempotencyKey) {
    const personaValue = requiredText(personaId, 'Persona.id');
    const keyValue = requiredText(idempotencyKey, 'Memory.idempotencyKey');
    const digest = createHash('sha256').update(`${personaValue}\u0000${keyValue}`).digest('hex');
    return `memory_event_${digest}`;
}

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

function tableColumns(database) {
    return new Set(database.prepare('PRAGMA table_info(companion_memories)').all().map(row => row.name));
}

function rowValue(row, camel, snake) {
    return row?.[camel] === undefined ? row?.[snake] : row[camel];
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
    const columns = tableColumns(openDatabase);
    const hasIdempotencyColumn = columns.has('idempotency_key');

    function listActive({personaId, limit} = {}) {
        const personaValue = requiredText(personaId, 'Persona.id');
        return openDatabase.prepare(`
            SELECT * FROM companion_memories
            WHERE persona_id = ? AND status = 'active'
            ORDER BY updated_at DESC, id DESC
            LIMIT ?
        `).all(personaValue, limitValue(limit));
    }

    function find({personaId, memoryId} = {}) {
        const personaValue = requiredText(personaId, 'Persona.id');
        const memoryValue = requiredText(memoryId, 'Memory.id');
        return openDatabase.prepare(`
            SELECT * FROM companion_memories
            WHERE id = ? AND persona_id = ?
            LIMIT 1
        `).get(memoryValue, personaValue);
    }

    function findByMemoryKey({personaId, memoryKey, includeDeleted = false} = {}) {
        const personaValue = requiredText(personaId, 'Persona.id');
        const keyValue = requiredText(memoryKey, 'Memory.memoryKey');
        const statusClause = includeDeleted ? '' : "AND status = 'active'";
        return openDatabase.prepare(`
            SELECT * FROM companion_memories
            WHERE persona_id = ? AND memory_key = ? ${statusClause}
            ORDER BY updated_at DESC, id DESC
            LIMIT 1
        `).get(personaValue, keyValue);
    }

    function findByIdempotencyKey({personaId, idempotencyKey} = {}) {
        const personaValue = requiredText(personaId, 'Persona.id');
        const keyValue = requiredText(idempotencyKey, 'Memory.idempotencyKey');
        if (hasIdempotencyColumn) {
            return openDatabase.prepare(`
                SELECT * FROM companion_memories
                WHERE persona_id = ? AND idempotency_key = ?
                ORDER BY updated_at DESC, id DESC
                LIMIT 1
            `).get(personaValue, keyValue);
        }
        return find({personaId: personaValue, memoryId: memoryEventIdFor(personaValue, keyValue)});
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
        sourceId, idempotencyKey, createdAt, updatedAt, supersededAt
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
        const fields = [
            'id', 'persona_id', 'memory_key', 'value', 'confidence', 'status', 'source_type',
            'source_id', 'created_at', 'updated_at', 'superseded_at'
        ];
        const values = [
            idValue, personaValue, keyValue, valueValue, confidenceValue, statusValue,
            nullableText(sourceType, 'Memory.sourceType'), nullableText(sourceId, 'Memory.sourceId'),
            createdValue, updatedValue, supersededValue
        ];
        if (hasIdempotencyColumn) {
            fields.push('idempotency_key');
            values.push(nullableText(idempotencyKey, 'Memory.idempotencyKey'));
        }
        openDatabase.prepare(`
            INSERT INTO companion_memories (${fields.join(', ')})
            VALUES (${fields.map(() => '?').join(', ')})
        `).run(...values);
        return openDatabase.prepare('SELECT * FROM companion_memories WHERE id = ?').get(idValue);
    }

    function insertIgnore(input = {}) {
        const idValue = requiredText(input.id, 'Memory.id');
        const personaValue = requiredText(input.personaId, 'Persona.id');
        const keyValue = requiredText(input.memoryKey, 'Memory.memoryKey');
        const valueValue = requiredText(input.value, 'Memory.value');
        const confidenceValue = Number(input.confidence);
        if (!Number.isFinite(confidenceValue)) throw new TypeError('Memory.confidence must be finite');
        const statusValue = requiredText(input.status ?? 'active', 'Memory.status');
        const createdValue = timestamp(input.createdAt ?? currentTime(), 'Memory.createdAt');
        const updatedValue = timestamp(input.updatedAt ?? createdValue, 'Memory.updatedAt');
        const supersededValue = nullableText(input.supersededAt, 'Memory.supersededAt');
        const fields = [
            'id', 'persona_id', 'memory_key', 'value', 'confidence', 'status', 'source_type',
            'source_id', 'created_at', 'updated_at', 'superseded_at'
        ];
        const values = [
            idValue, personaValue, keyValue, valueValue, confidenceValue, statusValue,
            nullableText(input.sourceType, 'Memory.sourceType'), nullableText(input.sourceId, 'Memory.sourceId'),
            createdValue, updatedValue, supersededValue
        ];
        if (hasIdempotencyColumn) {
            fields.push('idempotency_key');
            values.push(nullableText(input.idempotencyKey, 'Memory.idempotencyKey'));
        }
        const result = openDatabase.prepare(`
            INSERT OR IGNORE INTO companion_memories (${fields.join(', ')})
            VALUES (${fields.map(() => '?').join(', ')})
        `).run(...values);
        return {
            changes: result.changes,
            row: openDatabase.prepare('SELECT * FROM companion_memories WHERE id = ? AND persona_id = ?').get(idValue, personaValue)
        };
    }

    function upsert({
        id, personaId, memoryKey, value, confidence, status = 'active', sourceType,
        sourceId, idempotencyKey, createdAt, updatedAt, supersededAt
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
        const fields = [
            'id', 'persona_id', 'memory_key', 'value', 'confidence', 'status', 'source_type',
            'source_id', 'created_at', 'updated_at', 'superseded_at'
        ];
        const values = [
            idValue, personaValue, keyValue, valueValue, confidenceValue, statusValue,
            nullableText(sourceType, 'Memory.sourceType'), nullableText(sourceId, 'Memory.sourceId'),
            createdValue, updatedValue, supersededValue
        ];
        if (hasIdempotencyColumn) {
            fields.push('idempotency_key');
            values.push(nullableText(idempotencyKey, 'Memory.idempotencyKey'));
        }
        const updateFields = [
            'memory_key = excluded.memory_key',
            'value = excluded.value',
            'confidence = excluded.confidence',
            'status = excluded.status',
            'updated_at = excluded.updated_at',
            'superseded_at = excluded.superseded_at'
        ];
        if (hasIdempotencyColumn) updateFields.push('idempotency_key = excluded.idempotency_key');
        openDatabase.prepare(`
            INSERT INTO companion_memories (${fields.join(', ')})
            VALUES (${fields.map(() => '?').join(', ')})
            ON CONFLICT(id) DO UPDATE SET
                ${updateFields.join(',\n                ')}
        `).run(...values);
        return openDatabase.prepare('SELECT * FROM companion_memories WHERE id = ? AND persona_id = ?').get(idValue, personaValue);
    }

    return Object.freeze({
        listActive,
        find,
        get: find,
        findByMemoryKey,
        findByIdempotencyKey,
        insert,
        insertMemory: insert,
        insertIgnore,
        insertMemoryIgnore: insertIgnore,
        upsert,
        upsertMemory: upsert,
        delete: remove,
        idempotencyStorage: hasIdempotencyColumn ? 'column' : 'deterministic-id'
    });
}

export const createCompanionMemoryRepository = createMemoryRepository;
export default createMemoryRepository;

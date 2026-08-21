import {randomUUID} from 'node:crypto';

function assertOpenDatabase(database) {
    if (!database || typeof database.prepare !== 'function') {
        throw new TypeError('Life-event repository requires an open database');
    }
    return database;
}

function assertRecord(value, name) {
    if (!value || typeof value !== 'object' || Array.isArray(value)) {
        throw new TypeError(`${name} must be an object`);
    }
    return value;
}

function requiredText(value, field) {
    if (typeof value !== 'string' || value.length === 0) {
        throw new TypeError(`${field} must be a non-empty string`);
    }
    return value;
}

function textValue(value, field) {
    if (typeof value !== 'string') throw new TypeError(`${field} must be a string`);
    return value;
}

function resolveClock(clock, now) {
    const source = clock ?? now;
    if (source === undefined) return () => new Date().toISOString();
    if (typeof source === 'function') return () => timestamp(source(), 'Life-event clock value');
    if (source && typeof source.now === 'function') {
        return () => timestamp(source.now(), 'Life-event clock value');
    }
    throw new TypeError('Life-event repository clock must be a function or provide now()');
}

function resolveId(id, idGenerator) {
    const source = id ?? idGenerator;
    if (source === undefined) return prefix => `${prefix}_${randomUUID()}`;
    if (typeof source === 'function') return prefix => requiredText(source(prefix), 'Generated life-event id');
    if (source && typeof source.next === 'function') {
        return prefix => requiredText(source.next(prefix), 'Generated life-event id');
    }
    throw new TypeError('Life-event repository id must be a function or provide next()');
}

function timestamp(value, field) {
    if (value instanceof Date) return value.toISOString();
    return requiredText(value, field);
}

function valueFor(input, camelName, snakeName) {
    return input[camelName] === undefined ? input[snakeName] : input[camelName];
}

function jsonFor(input, camelName, snakeName, fallback) {
    const explicit = valueFor(input, camelName, snakeName);
    if (explicit !== undefined) {
        if (typeof explicit === 'string') return explicit;
        const serialized = JSON.stringify(explicit);
        if (serialized === undefined) throw new TypeError(`Life-event.${camelName} could not be serialized`);
        return serialized;
    }
    const serialized = JSON.stringify(fallback);
    if (serialized === undefined) throw new TypeError(`Life-event.${camelName} could not be serialized`);
    return serialized;
}

function idInput(first, second = {}) {
    if (typeof first === 'string') return {...second, eventId: first};
    return assertRecord(first, 'Life-event input');
}

function limitValue(value) {
    const limit = value === undefined ? 20 : Number(value);
    if (!Number.isInteger(limit) || limit < 1) throw new RangeError('Life-event limit must be a positive integer');
    return limit;
}

/**
 * Create a table-scoped adapter for life-event facts and the current persona
 * state projection. The caller owns domain validation and transaction scope;
 * this adapter only performs parameterized SQL and returns raw rows.
 */
export function createLifeEventRepository({database, clock, now, id, idGenerator} = {}) {
    const openDatabase = assertOpenDatabase(database);
    const currentTime = resolveClock(clock, now);
    const generateId = resolveId(id, idGenerator);

    function findById(first, second = {}) {
        const input = idInput(first, second);
        const eventId = requiredText(valueFor(input, 'eventId', 'event_id') ?? input.id, 'Life-event.id');
        const personaId = valueFor(input, 'personaId', 'persona_id');
        const scope = personaId === undefined || personaId === null
            ? {sql: '', values: []}
            : {sql: ' AND persona_id = ?', values: [requiredText(personaId, 'Persona.id')]};
        return openDatabase.prepare(`
            SELECT * FROM companion_life_events
            WHERE id = ?${scope.sql}
        `).get(eventId, ...scope.values);
    }

    function insertEvent(input = {}) {
        assertRecord(input, 'Life-event input');
        const eventId = requiredText(valueFor(input, 'eventId', 'event_id') ?? input.id ?? generateId('event'), 'Life-event.id');
        const personaId = requiredText(valueFor(input, 'personaId', 'persona_id'), 'Persona.id');
        const type = requiredText(input.type, 'Life-event.type');
        const createdAt = timestamp(valueFor(input, 'createdAt', 'created_at') ?? currentTime(), 'Life-event.createdAt');
        const occurredAt = timestamp(valueFor(input, 'occurredAt', 'occurred_at') ?? createdAt, 'Life-event.occurredAt');
        const resolvesAt = valueFor(input, 'resolvesAt', 'resolves_at');
        const causationId = valueFor(input, 'causationId', 'causation_id');
        const payloadJson = jsonFor(input, 'payloadJson', 'payload_json', input.payload ?? {});

        openDatabase.prepare(`
            INSERT INTO companion_life_events (
                id, persona_id, type, occurred_at, resolves_at, causation_id, payload_json, created_at
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        `).run(
            eventId,
            personaId,
            type,
            occurredAt,
            resolvesAt === null || resolvesAt === undefined ? null : timestamp(resolvesAt, 'Life-event.resolvesAt'),
            causationId === null || causationId === undefined ? null : requiredText(causationId, 'Life-event.causationId'),
            payloadJson,
            createdAt
        );
        return findById(eventId);
    }

    // createEvent is the lifecycle-facing name; insertEvent remains useful
    // when a caller already has a complete storage row.
    const createEvent = insertEvent;

    function listActive(input = {}) {
        assertRecord(input, 'Life-event list input');
        const personaId = requiredText(valueFor(input, 'personaId', 'persona_id'), 'Persona.id');
        const at = timestamp(input.at ?? input.occurredAt ?? input.occurred_at ?? currentTime(), 'Life-event.at');
        return openDatabase.prepare(`
            SELECT * FROM companion_life_events
            WHERE persona_id = ?
              AND occurred_at <= ?
              AND (resolves_at IS NULL OR resolves_at > ?)
            ORDER BY occurred_at DESC, id DESC
            LIMIT ?
        `).all(personaId, at, at, limitValue(input.limit));
    }

    function findByIdempotencyKey({personaId, idempotencyKey} = {}) {
        const owner = requiredText(personaId, 'Persona.id');
        const key = requiredText(idempotencyKey, 'Life-event.idempotencyKey');
        return openDatabase.prepare(`
            SELECT * FROM companion_life_events
            WHERE persona_id = ? AND json_extract(payload_json, '$.idempotencyKey') = ?
            ORDER BY created_at, id LIMIT 1
        `).get(owner, key);
    }

    function list(input = {}) {
        assertRecord(input, 'Life-event list input');
        const personaId = requiredText(valueFor(input, 'personaId', 'persona_id'), 'Persona.id');
        return openDatabase.prepare(`
            SELECT * FROM companion_life_events
            WHERE persona_id = ?
            ORDER BY occurred_at DESC, id DESC
            LIMIT ?
        `).all(personaId, limitValue(input.limit));
    }

    function updateEvent(first, second = {}) {
        const input = idInput(first, second);
        const eventId = requiredText(valueFor(input, 'eventId', 'event_id') ?? input.id, 'Life-event.id');
        const personaId = valueFor(input, 'personaId', 'persona_id');
        const assignments = [];
        const values = [];
        const columns = [
            ['type', 'type', 'Life-event.type', requiredText],
            ['occurred_at', 'occurredAt', 'Life-event.occurredAt', timestamp],
            ['resolves_at', 'resolvesAt', 'Life-event.resolvesAt', timestamp],
            ['causation_id', 'causationId', 'Life-event.causationId', requiredText],
            ['payload_json', 'payloadJson', 'Life-event.payloadJson', value => jsonFor({payloadJson: value}, 'payloadJson', 'payload_json', {})]
        ];
        for (const [column, camelName, field, normalize] of columns) {
            const snakeName = column;
            const value = camelName === 'payloadJson' && input.payload !== undefined
                ? input.payload
                : valueFor(input, camelName, snakeName);
            if (value === undefined) continue;
            assignments.push(`${column} = ?`);
            values.push(value === null && ['resolvesAt', 'causationId'].includes(camelName) ? null : normalize(value, field));
        }
        if (!assignments.length) return findById({eventId, personaId});
        const scope = personaId === undefined || personaId === null
            ? {sql: '', values: []}
            : {sql: ' AND persona_id = ?', values: [requiredText(personaId, 'Persona.id')]};
        openDatabase.prepare(`
            UPDATE companion_life_events
            SET ${assignments.join(', ')}
            WHERE id = ?${scope.sql}
        `).run(...values, eventId, ...scope.values);
        return findById({eventId, personaId});
    }

    function findState(personaId) {
        const personaValue = requiredText(personaId, 'Persona.id');
        return openDatabase.prepare(`
            SELECT * FROM companion_persona_states WHERE persona_id = ?
        `).get(personaValue);
    }

    function insertState(input = {}) {
        assertRecord(input, 'Life-event state input');
        const personaId = requiredText(valueFor(input, 'personaId', 'persona_id'), 'Persona.id');
        const checkpointAt = timestamp(valueFor(input, 'checkpointAt', 'checkpoint_at') ?? currentTime(), 'Life-event state.checkpointAt');
        const updatedAt = timestamp(valueFor(input, 'updatedAt', 'updated_at') ?? checkpointAt, 'Life-event state.updatedAt');
        openDatabase.prepare(`
            INSERT INTO companion_persona_states (
                persona_id, situation, mood, appearance_json, checkpoint_at, updated_at, source_event_id
            ) VALUES (?, ?, ?, ?, ?, ?, ?)
        `).run(
            personaId,
            textValue(input.situation ?? '', 'Life-event state.situation'),
            textValue(input.mood ?? '', 'Life-event state.mood'),
            jsonFor(input, 'appearanceJson', 'appearance_json', input.appearance ?? {}),
            checkpointAt,
            updatedAt,
            valueFor(input, 'sourceEventId', 'source_event_id') ?? null
        );
        return findState(personaId);
    }

    function updateState(input = {}) {
        assertRecord(input, 'Life-event state input');
        const personaId = requiredText(valueFor(input, 'personaId', 'persona_id'), 'Persona.id');
        const assignments = [];
        const values = [];
        const fields = [
            ['situation', 'situation', value => textValue(value, 'Life-event state.situation')],
            ['mood', 'mood', value => textValue(value, 'Life-event state.mood')],
            ['appearance_json', 'appearanceJson', value => jsonFor({appearanceJson: value}, 'appearanceJson', 'appearance_json', {})],
            ['checkpoint_at', 'checkpointAt', value => timestamp(value, 'Life-event state.checkpointAt')],
            ['updated_at', 'updatedAt', value => timestamp(value, 'Life-event state.updatedAt')],
            ['source_event_id', 'sourceEventId', value => value === null ? null : requiredText(value, 'Life-event state.sourceEventId')]
        ];
        for (const [column, camelName, normalize] of fields) {
            const value = valueFor(input, camelName, column);
            if (value === undefined) continue;
            assignments.push(`${column} = ?`);
            values.push(normalize(value));
        }
        if (!assignments.length) return findState(personaId);
        openDatabase.prepare(`
            UPDATE companion_persona_states SET ${assignments.join(', ')} WHERE persona_id = ?
        `).run(...values, personaId);
        return findState(personaId);
    }

    return Object.freeze({
        createEvent,
        insertEvent,
        findById,
        list,
        listActive,
        findByIdempotencyKey,
        updateEvent,
        findState,
        insertState,
        updateState
    });
}

export const createCompanionLifeEventRepository = createLifeEventRepository;
export default createLifeEventRepository;

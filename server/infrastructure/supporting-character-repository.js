function assertDatabase(database) {
    if (!database || typeof database.prepare !== 'function') throw new TypeError('Supporting-character repository requires an open database');
    return database;
}

function required(value, field) {
    if (typeof value !== 'string' || value.trim() === '') throw new TypeError(`${field} must be non-empty`);
    return value.trim();
}

function parse(value, fallback = {}) {
    try { return value ? JSON.parse(value) : fallback; } catch { return fallback; }
}

export function createSupportingCharacterRepository({database} = {}) {
    const db = assertDatabase(database);
    function row(value) {
        if (!value) return null;
        return {...value, personaId: value.persona_id, relationshipKind: value.relationship_kind,
            profile: parse(value.profile_json, {}), introducedEventId: value.introduced_event_id ?? null};
    }
    function findOwned({personaId, ids = []} = {}) {
        const owner = required(personaId, 'Persona.id');
        const values = [...new Set(ids.map(id => required(id, 'Supporting-character.id')))].slice(0, 8);
        if (!values.length) return [];
        const placeholders = values.map(() => '?').join(', ');
        return db.prepare(`SELECT * FROM companion_supporting_characters WHERE persona_id = ? AND id IN (${placeholders}) ORDER BY created_at, id`).all(owner, ...values).map(row);
    }
    function list({personaId, limit = 20} = {}) {
        const owner = required(personaId, 'Persona.id');
        const count = Math.max(1, Math.min(100, Number(limit) || 20));
        return db.prepare('SELECT * FROM companion_supporting_characters WHERE persona_id = ? ORDER BY created_at, id LIMIT ?').all(owner, count).map(row);
    }
    function create(input = {}) {
        const id = required(input.id, 'Supporting-character.id');
        const personaId = required(input.personaId, 'Persona.id');
        const name = required(input.name, 'Supporting-character.name').slice(0, 60);
        const relationshipKind = required(input.relationshipKind ?? '熟人', 'Supporting-character.relationshipKind').slice(0, 60);
        const createdAt = required(input.createdAt, 'Supporting-character.createdAt');
        const updatedAt = required(input.updatedAt ?? createdAt, 'Supporting-character.updatedAt');
        const introducedEventId = input.introducedEventId ?? input.introduced_event_id ?? null;
        if (introducedEventId !== null) {
            const event = db.prepare('SELECT id FROM companion_life_events WHERE id = ? AND persona_id = ?').get(required(introducedEventId, 'Supporting-character.introducedEventId'), personaId);
            if (!event) throw new TypeError('Supporting-character.introducedEventId must belong to the persona');
        }
        db.prepare(`
            INSERT INTO companion_supporting_characters
                (id, persona_id, name, relationship_kind, profile_json, introduced_event_id, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        `).run(id, personaId, name, relationshipKind, JSON.stringify(input.profile ?? {}), introducedEventId, createdAt, updatedAt);
        return row(db.prepare('SELECT * FROM companion_supporting_characters WHERE id = ?').get(id));
    }
    return Object.freeze({findOwned, list, listForPersona: list, create, insert: create, introduce: create});
}

export default createSupportingCharacterRepository;

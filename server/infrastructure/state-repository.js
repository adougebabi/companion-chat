function assertDatabase(database) {
    if (!database || typeof database.prepare !== 'function') throw new TypeError('State repository requires an open database');
    return database;
}

function text(value, field) {
    if (typeof value !== 'string' || value.trim() === '') throw new TypeError(`${field} must be non-empty`);
    return value.trim();
}

function json(value, fallback = {}) {
    if (value && typeof value === 'object') return value;
    try { return value ? JSON.parse(value) : fallback; } catch { return fallback; }
}

function clockFor(clock) {
    if (typeof clock === 'function') return clock;
    if (clock && typeof clock.now === 'function') return clock.now.bind(clock);
    return () => new Date().toISOString();
}

export function createStateRepository({database, clock} = {}) {
    const db = assertDatabase(database);
    const now = clockFor(clock);
    function read(input = {}) {
        const owner = text(typeof input === 'string' ? input : input.personaId ?? input.id, 'Persona.id');
        const row = db.prepare('SELECT * FROM companion_persona_states WHERE persona_id = ?').get(owner);
        if (!row) return null;
        return {...row, sharedScene: json(row.shared_scene_json, null)};
    }
    function updateProjection(input = {}) {
        const owner = text(input.personaId, 'Persona.id');
        const at = input.updatedAt ?? now();
        const current = read({personaId: owner}) ?? {};
        db.prepare(`
            INSERT INTO companion_persona_states (persona_id, situation, mood, appearance_json, checkpoint_at, updated_at, source_event_id, shared_scene_json)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?)
            ON CONFLICT(persona_id) DO UPDATE SET
                situation = excluded.situation, mood = excluded.mood, appearance_json = excluded.appearance_json,
                checkpoint_at = excluded.checkpoint_at, updated_at = excluded.updated_at,
                source_event_id = excluded.source_event_id, shared_scene_json = excluded.shared_scene_json
        `).run(
            owner,
            input.situation ?? current.situation ?? '',
            input.mood ?? current.mood ?? '平静',
            typeof input.appearanceJson === 'string' ? input.appearanceJson : JSON.stringify(input.appearance ?? json(current.appearance_json, {})),
            input.checkpointAt ?? at,
            at,
            input.sourceEventId ?? input.source_event_id ?? current.source_event_id ?? null,
            JSON.stringify(input.sharedScene ?? input.shared_scene ?? json(current.shared_scene_json, null) ?? {})
        );
        return read({personaId: owner});
    }
    return Object.freeze({read, get: read, findByPersona: read, updateProjection, update: updateProjection, updateState: updateProjection, applyProjection: updateProjection});
}

export default createStateRepository;

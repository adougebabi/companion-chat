function dbFor(database) { if (!database || typeof database.prepare !== 'function') throw new TypeError('Presence repository requires an open database'); return database; }
function parse(value, fallback = null) { try { return value ? JSON.parse(value) : fallback; } catch { return fallback; } }
export function createPresenceRepository({database} = {}) {
    const db = dbFor(database);
    function read(input = {}) {
        const owner = typeof input === 'string' ? input : input.personaId ?? input.id;
        const row = db.prepare('SELECT shared_scene_json FROM companion_persona_states WHERE persona_id = ?').get(owner);
        const scene = parse(row?.shared_scene_json, null);
        return scene ? {...scene, personaId: owner, active: true} : {personaId: owner, active: false};
    }
    return Object.freeze({read, find: read, findByPersona: read, get: read});
}
export default createPresenceRepository;

function dbFor(database) { if (!database || typeof database.prepare !== 'function') throw new TypeError('Blueprint repository requires an open database'); return database; }
function parse(value) { try { return value ? JSON.parse(value) : {}; } catch { return {}; } }
export function createBlueprintRepository({database} = {}) {
    const db = dbFor(database);
    function read(input = {}) {
        const owner = typeof input === 'string' ? input : input.personaId ?? input.id;
        const row = db.prepare('SELECT blueprint_json FROM companion_persona_life_blueprints WHERE persona_id = ?').get(owner);
        return row ? {...parse(row.blueprint_json), personaId: owner} : null;
    }
    return Object.freeze({read, get: read, find: read});
}
export default createBlueprintRepository;

function dbFor(database) { if (!database || typeof database.prepare !== 'function') throw new TypeError('Blueprint repository requires an open database'); return database; }
function isRecord(value) { return Boolean(value) && typeof value === 'object' && !Array.isArray(value); }
function parse(value) { try { return value ? JSON.parse(value) : {}; } catch { return {}; } }

function defaultBlueprint(name = '人格', role = '陪伴者', foundation = '') {
    return {
        schemaVersion: 2,
        timezone: 'Asia/Shanghai',
        foundation,
        identity: {name, role},
        interests: [],
        routine: [],
        world: {
            defaultSceneRef: {locationId: 'home', roomId: 'private_room'},
            locations: [{
                id: 'home', kind: 'home', name: '家中', isDefault: true,
                rooms: [{id: 'private_room', kind: 'private_room', name: '自己的宿舍房间', scene: `${name}在自己的宿舍房间里，保持自然的日常状态。`, activityTags: ['rest', 'chat']}]
            }]
        },
        fixedTimeEvents: [], dailyFlexibleEvents: [], randomPositiveEvents: [], randomNegativeEvents: [],
        supportingCast: [], generation: {source: 'safe-v2-fallback', usedFallback: true, validationWarnings: ['blueprint_missing_or_invalid']}
    };
}

function effectiveBlueprint(raw, identity = {}) {
    const fallback = defaultBlueprint(identity.name, identity.role, identity.foundation);
    const value = isRecord(raw) ? raw : {};
    const world = isRecord(value.world) ? value.world : {};
    const locations = Array.isArray(world.locations) && world.locations.length ? world.locations : fallback.world.locations;
    const defaultSceneRef = isRecord(world.defaultSceneRef) || typeof world.defaultSceneRef === 'string'
        ? world.defaultSceneRef
        : fallback.world.defaultSceneRef;
    return {
        ...fallback,
        ...value,
        schemaVersion: 2,
        timezone: typeof value.timezone === 'string' && value.timezone.trim() ? value.timezone : fallback.timezone,
        identity: {...fallback.identity, ...(isRecord(value.identity) ? value.identity : {})},
        world: {...fallback.world, ...world, defaultSceneRef, locations},
        generation: {...fallback.generation, ...(isRecord(value.generation) ? value.generation : {})}
    };
}

export function createBlueprintRepository({database} = {}) {
    const db = dbFor(database);
    function read(input = {}) {
        const owner = typeof input === 'string' ? input : input.personaId ?? input.id;
        const persona = db.prepare('SELECT name, role FROM companion_personas WHERE id = ? AND enabled = 1 AND deleted_at IS NULL').get(owner);
        if (!persona) return null;
        const row = db.prepare('SELECT blueprint_json FROM companion_persona_life_blueprints WHERE persona_id = ?').get(owner);
        const foundation = db.prepare('SELECT foundation FROM companion_persona_foundation_revisions WHERE persona_id = ? ORDER BY version DESC, created_at DESC, id DESC LIMIT 1').get(owner)?.foundation || '';
        return {...effectiveBlueprint(row ? parse(row.blueprint_json) : null, {...persona, foundation}), personaId: owner};
    }
    return Object.freeze({read, get: read, find: read});
}
export default createBlueprintRepository;

// Browser world wire shapes mirror the explicit Go world surface validators.
const text = (maxLength) => ({ type: 'string', ...(maxLength ? { maxLength } : {}) });
const integer = { type: 'integer', minimum: 0, maximum: 2147483647 };
const bool = { type: 'boolean' };
const object = { type: 'object', additionalProperties: true };
const nullable = (shape) => ({ anyOf: [shape, { type: 'null' }] });
const array = (items) => ({ type: 'array', items });
const ref = (name) => ({ $ref: `#/components/schemas/${name}` });
const shape = (properties, required = Object.keys(properties), additionalProperties = false) => ({ type: 'object', properties, required, additionalProperties });
const collection = { anyOf: [text(16000), { ...array({}), maxItems: 200 }], description: 'Arrays must also fit the Core 32000-byte encoded JSON bound.' };
const status = { type: 'string', enum: ['active', 'archived'] };
const mode = { type: 'string', enum: ['routine', 'fixed', 'unknown'] };
const command = { reason: { ...text(1000), minLength: 1 }, idempotencyKey: { ...text(256), minLength: 1 }, expectedRevision: integer };
const extras = { alias: text(256), group: text(256), floor: text(128), scene: text(1000), atmosphere: text(2000), layout: text(8000), furniture: collection, connections: collection };
const world = { id: text(), fluctlight_id: text(), name: text(), background: text(), rules: text(), flavor: text(), description: text(), item_pool: collection, revision: integer, source_resource_id: nullable(text()), source_version: nullable(text()) };
const worldWrite = { ...command, name: { ...text(256), minLength: 1 }, background: text(16000), rules: text(8000), flavor: text(1000), description: text(8000), itemPool: collection, venueCategories: { ...array(shape({id:{...text(80),minLength:1},name:{...text(128),minLength:1}})), maxItems:32, description:'Append categories while retaining existing IDs and names. Categories group concrete places; they are not enterable places.' } };
export const worldSchemas = {
  BrowserRuntimeWorld: shape({...world,venue_categories:array(shape({id:text(80),name:text(128)}))},Object.keys(world)),
  BrowserRuntimePlace: shape({venue_category_id:text(80),framework_source:object, id: text(), world_id: text(), name: text(), desc: text(), status, revision: integer, parent_place_id: nullable(text()), source_resource_id: nullable(text()), source_version: nullable(text()), ...extras, room_items: collection, connection_place_ids: array(text()) }, ['id', 'world_id', 'name', 'desc', 'status', 'revision', 'parent_place_id', 'source_resource_id', 'source_version']),
  BrowserNPCRoutineSlot: shape({ day: { type: 'string', enum: ['daily', 'weekday', 'weekend'] }, start: text(), end: text(), place_id: text(), activity: text() }),
  BrowserRuntimeNPC: shape({ id: text(), world_id: text(), actor_id: text(), name: text(), role: text(), bio: text(), home_place_id: nullable(text()), routine: array(ref('BrowserNPCRoutineSlot')), status, revision: integer, source_resource_id: text(), source_version: text(), confirmed_place_id: nullable(text()), confirmed_at: nullable(text()), full_fluctlight: bool, enabled: bool, position_mode: mode, local_overrides: object, runtime_clock:nullable(object),current_presence:nullable(ref('BrowserNPCPresence')),runtime_reference_ids:array(text()),planned_position: shape({window_start:text(),window_end:text(), place_id: text(), activity: text(), source: { type: 'string', enum: ['schedule_prediction', 'initial_place', 'fixed_prediction'] }, timezone: text() }, ['place_id', 'source']), encounters: array(shape({ id: text(), npc_id: text(), place_id: text(), life_event_id: text(), relationship_id: nullable(text()), memory_id: nullable(text()),source_kind:text(),opportunity_id:nullable(text()), summary: text(), evidence_refs: array(text()), occurred_at: text() })), relationship_context: object }, ['id', 'world_id', 'actor_id', 'name', 'role', 'bio', 'home_place_id', 'routine', 'status', 'revision', 'source_resource_id', 'source_version', 'confirmed_place_id', 'confirmed_at', 'full_fluctlight', 'enabled', 'position_mode', 'local_overrides']),
  BrowserWorldView: shape({ available: { type: 'boolean', const: true }, world: nullable(ref('BrowserRuntimeWorld')), places: array(ref('BrowserRuntimePlace')), npcs: array(ref('BrowserRuntimeNPC')), scenes: array({}), current_context: object }, ['available', 'world', 'places', 'npcs', 'scenes']),
  BrowserWorldCatalog: shape({ items: array(shape({ ...world, active: bool,venue_categories:array(shape({id:text(80),name:text(128)})) },[...Object.keys(world),'active'])), active_binding: nullable(shape({ world_id: text(), revision: integer })), next_cursor: { type: 'null' } }),
  BrowserWorldCommand: shape(command),
  BrowserWorldWriteBody: shape(worldWrite, ['name', ...Object.keys(command)]),
  BrowserWorldCreateBody: shape({...worldWrite,sourceWorldId:text(128),sourceFluctlightId:text(128),expectedSourceRevision:integer}, ['name', 'reason', 'idempotencyKey']),
  BrowserWorldSelectBody: shape({ ...command, worldId: { ...text(128), minLength: 1 } }),
  BrowserPlaceWriteBody: shape({ ...command, venueCategoryId:text(80),worldId: text(128), name: { ...text(256), minLength: 1 }, description: text(8000), parentPlaceId: nullable(text(128)), status, ...extras, roomItems: collection, connectionPlaceIds: { ...array(text(128)), maxItems: 200 } }, [...Object.keys(command), 'name']),
  BrowserWorldDeleteBody: shape({ ...command, worldId: text(128) }, Object.keys(command)),
  BrowserNPCWriteBody: shape({ ...command, name: text(256), role: text(512), background: text(16000), homePlaceId: nullable(text(128)), routine: { ...array(ref('BrowserNPCRoutineSlot')), maxItems: 42 }, status, enabled: bool, positionMode: mode }, [...Object.keys(command), 'name', 'role', 'background', 'homePlaceId', 'routine', 'status']),
  BrowserWorldWriteResult: shape({ id: text(), revision: integer, status: text(), audit_id: text(), world_id: text(), actor_id: text(), active: bool, life_write: bool, history_preserved: bool }, ['id', 'revision', 'status', 'audit_id']),
  BrowserWorldDeleteResult: shape({ id: text(), status: { type: 'string', enum: ['deleted'] }, audit_id: text(), actor_id: text() }, ['id', 'status', 'audit_id']),
  BrowserNearbyActors: shape({ place_id: nullable(text()), actors: array(shape({ actor_id: text(), npc_id: text(), name: text(), npc: bool, full_fluctlight: bool, since: nullable(text()), presence: { type: 'string', enum: ['predicted', 'confirmed'] }, source: text(), place_id: text(), event_id: text(),presence_id:text(),character_presence:text() }, ['actor_id', 'npc_id', 'name', 'npc', 'full_fluctlight', 'since', 'presence', 'source', 'place_id'])), available: bool, location_known: bool }),
};
const response = (name) => ({ responses: { 200: { description: 'Successful response', content: { 'application/json': { schema: ref(name) } } } } });
const operation = (operationId, result, body) => ({ operationId, ...response(result), ...(body ? { requestBody: { required: true, content: { 'application/json': { schema: ref(body) } } } } : {}) });
const base = '/api/fluctlights/{fluctlightId}';
export const worldPaths = {
  [`${base}/world`]: { get: operation('fluctlightWorld', 'BrowserWorldView'), put: operation('updateRuntimeWorld', 'BrowserWorldWriteResult', 'BrowserWorldWriteBody') },
  [`${base}/world/catalog`]: { get: operation('worldCatalog', 'BrowserWorldCatalog'), post: operation('createCatalogWorld', 'BrowserWorldWriteResult', 'BrowserWorldCreateBody') },
  [`${base}/world/catalog/{worldId}`]: { get: operation('catalogWorld', 'BrowserWorldView'), put: operation('updateCatalogWorld', 'BrowserWorldWriteResult', 'BrowserWorldWriteBody'), delete: operation('deleteCatalogWorld', 'BrowserWorldDeleteResult', 'BrowserWorldCommand') },
  [`${base}/world/active`]: { put: operation('selectWorld', 'BrowserWorldWriteResult', 'BrowserWorldSelectBody') },
  [`${base}/world/places`]: { post: operation('createWorldPlace', 'BrowserWorldWriteResult', 'BrowserPlaceWriteBody') },
  [`${base}/world/places/{placeId}`]: { get: operation('worldPlace', 'BrowserRuntimePlace'), put: operation('updateWorldPlace', 'BrowserWorldWriteResult', 'BrowserPlaceWriteBody'), delete: operation('deleteWorldPlace', 'BrowserWorldDeleteResult', 'BrowserWorldDeleteBody') },
  [`${base}/npcs/{npcId}`]: { get: operation('readRuntimeNPC', 'BrowserRuntimeNPC'), put: operation('updateRuntimeNPC', 'BrowserWorldWriteResult', 'BrowserNPCWriteBody'), delete: operation('deleteRuntimeNPC', 'BrowserWorldDeleteResult', 'BrowserWorldCommand') },
  [`${base}/nearby`]: { get: operation('fluctlightNearby', 'BrowserNearbyActors') },
};

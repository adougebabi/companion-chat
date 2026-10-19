# T04 world catalogue / WP05–06 domain slice

## Model and compatibility

- `0065_world_catalog` follows the unchanged `0064_world_runtime`. `life_worlds.fluctlight_id` retains character ownership, while `(fluctlight_id,id)` and `fluctlight_world_bindings` enforce an independently selected, same-owner world. Existing 0064 worlds are bound exactly once when the binding table is first created. Re-running migration never selects an unbound world or discards a world/place/source ID.
- Catalogue create/save and world-resource apply do not select a world. `PUT world/active` uses the binding revision; it does not create a movement Event, acquaintance, memory, body or wardrobe effect. The older `PUT world` remains a first-configuration compatibility command.
- World description/flavor/item pool are persisted. Place display properties use an explicitly validated details object. Existing UI text for connections is preserved as text; only `connection_place_ids` is a stable link. No matching by names occurs. Historical textual Event/Schedule locations remain unknown catalogue positions.
- NPC remains a separate runtime Actor, not a full Fluctlight. Enabled/archived/position mode and local overrides survive definition upgrades. Only changes against the running published definition count as local text overrides. Removal rejects encounter, relationship, Memory and conversation references; the retired Actor ID remains unrecycled.
- Shared resource Get obtains a row lock for save/publish/apply, including cross-character reference updates. Saving accepts the current draft version only. Publications remain immutable snapshots. Permanent-harm flags are rejected in the new runtime, publisher and application entrances, including stored/imported definitions.
- Parent ancestry rejects cycles. Place archive/removal checks stable NPC, child, explicit connection, encounter, Event-result and executable Schedule references, never textual name matches.
- Nearby returns `{place_id, actors}`. Current place resolves from the same valid Life Context Event and its stable result ID within the active binding. Explicit confirmed Event participants and NPC routine/fixed predictions are distinguished; inferred Events do not become confirmation. Missing stable identity yields unknown. GET writes no state. Prompt projection includes only active world setting, a bounded current-place projection and bounded already-known NPC summaries.

## API additions / UI mapping

All routes retain the existing Owner session, CSRF and in-process Backend transport.

- `GET/POST /api/fluctlights/{id}/world/catalog`: list or create (name, optional world fields, reason, idempotencyKey). Result: `items`, nullable `active_binding:{world_id,revision}`, `next_cursor:null`; create returns id/revision/active:false.
- `GET/PUT/DELETE /api/fluctlights/{id}/world/catalog/{worldId}`: inspect WorldView without selection; save or remove using expectedRevision/reason/idempotencyKey.
- `PUT /api/fluctlights/{id}/world/active`: worldId/expectedRevision/reason/idempotencyKey. Revision belongs to binding.
- `DELETE /api/fluctlights/{id}/world/places/{placeId}` and `DELETE /api/fluctlights/{id}/npcs/{npcId}`: expectedRevision/reason/idempotencyKey.
- World fields: flavor, description, itemPool (string or bounded array). Read names remain snake_case.
- Place fields: alias/group/floor/scene/atmosphere/layout; furniture/connections/roomItems (string or bounded array); connectionPlaceIds (stable IDs); optional worldId allows editing an unselected owned world. The UI `desc` maps to existing `description` write / `desc` read.
- NPC fields: enabled and positionMode (`routine`, `fixed`, `unknown`); UI labels must map explicitly. Read fields: enabled, position_mode, local_overrides. Home/routine/Actor identities remain unchanged on definition upgrades.
- Nearby actors retain actor_id/name/npc/full_fluctlight/since, adding npc_id/place_id/presence/source/event_id where applicable.

## Validation evidence

`go test -race ./internal/core ./internal/migrations -run 'TestWorld|TestNPCPlanned' -count=1 -v` ran against fresh, randomly named local PostgreSQL databases created by the existing test admin helper. It passed catalogue round trip/CAS/replay/scope, unbound edits, fields readback, parent cycle/reference deletion, published-draft CAS and concurrent saves, shared-resource concurrent application, local overrides and Actor preservation, unknown/confirmed nearby shape and GET no-write, NPC unused removal/referenced rejection, publication/application harm rejection, NPC encounter event/relationship/memory/outbox and replay, fresh migrations, a populated 0064-layout fixture upgrade, and repeated migration application. Log: `t04-world-catalog-0065-race.log`.

`go test -race ./internal/httpapi/browser -run 'TestWorldSurface|TestSurfaceRoutes' -count=1` passed. `go vet ./internal/core ./internal/migrations ./internal/httpapi/browser` passed. Migration package without DB env passed (DB-dependent tests skip in that run; the separate real-DB run above is the DB evidence).

The 0064 upgrade evidence is an isolated populated old-layout fixture, not a claim of upgrading the user's existing application database. No existing task database was reset, dropped or migrated.

## Remaining integration / acceptance scope

- Main session owns Browser OpenAPI / BrowserClient generation and production v3 provider wiring. Full browser race initially fails only route inventory until the eight new catalogue/selection/removal operations are generated.
- Automatic arrival → encounter opportunity → controlled decision remains a separate explicitly assigned slice. Current owner-confirmed encounters return durable refs and are queryable. New Event/Activity/Schedule writers must carry a validated stable place ID before their current text can project as catalogue presence; this slice deliberately does not infer IDs from text.
- This is domain and persistence evidence, not full WP05/WP06/WP12 UI or Worker acceptance, nor a claim of user acceptance.

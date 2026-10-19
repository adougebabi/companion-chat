import assert from "node:assert/strict";
import test from "node:test";
import { readFile } from "node:fs/promises";
import { BrowserClient, BrowserApiError, type WorldCatalog, type WorldView, type NearbyActors } from "../src/index.ts";

test("world catalog methods execute on BrowserClient with scoped URLs and camelCase mutation bodies", async () => {
  const calls: Array<{path: string; method: string; body: unknown}> = [];
  const client = new BrowserClient("http://local.test", async (input, init) => {
    calls.push({path: new URL(String(input)).pathname, method: init?.method ?? "GET", body: init?.body ? JSON.parse(String(init.body)) : null});
    return Response.json({id: "world-1", revision: 2, status: "saved", audit_id: "audit-1"});
  });
  const command = {reason: "Owner edit", idempotencyKey: "world-key", expectedRevision: 1};
  const worldBody = {...command, name: "Garden", flavor: "quiet", description: "shared space", itemPool: ["flower"]};
  const placeBody = {...command, worldId: "unselected-world", name: "Room", alias: "Study", group: "Home", floor: "2F", scene: "indoors", atmosphere: "quiet", layout: "open", furniture: ["desk"], connections: "door", roomItems: ["book"], connectionPlaceIds: ["hall"]};
  await client.worldCatalog("fl/1");
  await client.catalogWorld("fl/1", "world/1");
  await client.createCatalogWorld("fl/1", worldBody);
  await client.updateCatalogWorld("fl/1", "world/1", worldBody);
  await client.deleteCatalogWorld("fl/1", "world/1", command);
  await client.selectWorld("fl/1", {...command, worldId: "world/1"});
  await client.createWorldPlace("fl/1", placeBody);
  await client.updateWorldPlace("fl/1", "place/1", placeBody);
  await client.deleteWorldPlace("fl/1", "place/1", {...command, worldId: "unselected-world"});
  await client.deleteRuntimeNPC("fl/1", "npc/1", command);
  await client.updateRuntimeWorld("fl/1", worldBody);
  await client.updateRuntimeNPC("fl/1", "npc/1", {...command, name: "NPC", role: "neighbor", background: "local", homePlaceId: null, routine: [], status: "active", enabled: false, positionMode: "unknown"});
  assert.deepEqual(calls.map(({method,path}) => `${method} ${path}`), [
    "GET /api/fluctlights/fl%2F1/world/catalog", "GET /api/fluctlights/fl%2F1/world/catalog/world%2F1", "POST /api/fluctlights/fl%2F1/world/catalog", "PUT /api/fluctlights/fl%2F1/world/catalog/world%2F1", "DELETE /api/fluctlights/fl%2F1/world/catalog/world%2F1", "PUT /api/fluctlights/fl%2F1/world/active", "POST /api/fluctlights/fl%2F1/world/places", "PUT /api/fluctlights/fl%2F1/world/places/place%2F1", "DELETE /api/fluctlights/fl%2F1/world/places/place%2F1", "DELETE /api/fluctlights/fl%2F1/npcs/npc%2F1", "PUT /api/fluctlights/fl%2F1/world", "PUT /api/fluctlights/fl%2F1/npcs/npc%2F1",
  ]);
  assert.deepEqual(calls[3].body, worldBody);
  assert.deepEqual(calls[7].body, placeBody);
  assert.deepEqual(calls[8].body, {...command, worldId: "unselected-world"});
});

test("world reads preserve catalog binding, full snapshot, and evidence-based nearby presence", async () => {
  const catalog: WorldCatalog = {items: [], active_binding: {world_id: "world-1", revision: 7}, next_cursor: null};
  const view: WorldView = {available: true, world: null, places: [], npcs: [], scenes: []};
  const nearby: NearbyActors = {place_id: "room", available: true, location_known: true, actors: [{actor_id: "actor-1", npc_id: "npc-1", name: "NPC", npc: true, full_fluctlight: false, since: "", presence: "predicted", source: "schedule_prediction", place_id: "room"}]};
  const client = new BrowserClient("http://local.test", async (input) => Response.json(String(input).endsWith("/nearby") ? nearby : String(input).endsWith("/catalog") ? catalog : view));
  assert.deepEqual(await client.worldCatalog("fl-1"), catalog);
  assert.deepEqual(await client.catalogWorld("fl-1", "world-1"), view);
  assert.deepEqual(await client.world("fl-1"), view);
  assert.deepEqual(await client.nearby("fl-1"), nearby);
});

test("world mutation conflicts remain errors and OpenAPI declares real request/response schemas", async () => {
  const client = new BrowserClient("http://local.test", async () => Response.json({code: "resource_referenced", message: "Referenced"}, {status: 409}));
  await assert.rejects(client.deleteCatalogWorld("fl-1", "world-1", {reason:"delete", idempotencyKey:"key", expectedRevision:1}), (error: unknown) => error instanceof BrowserApiError && error.code === "resource_referenced");
  const api = JSON.parse(await readFile(new URL("../openapi.json", import.meta.url), "utf8"));
  const path = api.paths["/api/fluctlights/{fluctlightId}/world/catalog/{worldId}"];
  assert.equal(path.get.responses[200].content["application/json"].schema.$ref, "#/components/schemas/BrowserWorldView");
  assert.equal(path.delete.requestBody.content["application/json"].schema.$ref, "#/components/schemas/BrowserWorldCommand");
  assert.equal(api.components.schemas.BrowserPlaceWriteBody.properties.worldId.type, "string");
  assert.deepEqual(api.components.schemas.BrowserRuntimeNPC.properties.position_mode.enum, ["routine", "fixed", "unknown"]);
});

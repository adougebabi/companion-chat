package browser

import (
	"encoding/json"
	"testing"
)

func TestWorldSurfaceValidationKeepsDomainIdentityServerOwned(t *testing.T) {
	tests := []struct {
		method, path, body string
		valid              bool
	}{
		{"PUT", "/api/fluctlights/fl/world", `{"name":"World","flavor":"Quiet","description":"Setting","itemPool":"Books","expectedRevision":1,"idempotencyKey":"key","reason":"Edit"}`, true},
		{"PUT", "/api/fluctlights/fl/world/catalog/world", `{"name":"World","venueCategories":[{"id":"shops","name":"商店"}],"expectedRevision":1,"idempotencyKey":"key","reason":"Edit"}`, true},
		{"PUT", "/api/fluctlights/fl/world/catalog/world", `{"name":"World","venueCategories":"[]","expectedRevision":1,"idempotencyKey":"key","reason":"Edit"}`, false},
		{"PUT", "/api/fluctlights/fl/world/catalog/world", `{"name":"World","venueCategories":[{"id":"shops","name":"商店","places":[]}],"expectedRevision":1,"idempotencyKey":"key","reason":"Edit"}`, false},
		{"PUT", "/api/fluctlights/fl/world/active", `{"worldId":"world","expectedRevision":1,"idempotencyKey":"key","reason":"Select"}`, true},
		{"PUT", "/api/fluctlights/fl/world/places/place", `{"name":"Room","worldId":"other-world","furniture":"Desk","roomItems":"Lamp","connectionPlaceIds":["place-b"],"expectedRevision":1,"idempotencyKey":"key","reason":"Edit"}`, true},
		{"PUT", "/api/fluctlights/fl/npcs/npc", `{"name":"NPC","enabled":false,"positionMode":"fixed","expectedRevision":1,"idempotencyKey":"key","reason":"Edit"}`, true},
		{"PUT", "/api/fluctlights/fl/npcs/npc", `{"name":"NPC","actorId":"forged","expectedRevision":1,"idempotencyKey":"key","reason":"Edit"}`, false},
		{"PUT", "/api/fluctlights/fl/world/active", `{"worldId":"world","expectedRevision":1.5,"idempotencyKey":"key","reason":"Select"}`, false},
		{"PUT", "/api/fluctlights/fl/world", `{"name":"World","itemPool":{},"expectedRevision":1,"idempotencyKey":"key","reason":"Edit"}`, false},
		{"PUT", "/api/fluctlights/fl/world/items/item", `{"available":true,"placeId":"place","expectedRevision":1,"idempotencyKey":"item-config","reason":"Configure world supply"}`, true},
		{"PUT", "/api/fluctlights/fl/world/items/item", `{"available":true,"placeId":"place","ownership":"owned","expectedRevision":1,"idempotencyKey":"item-config","reason":"Cannot grant possession"}`, false},
		{"PUT", "/api/fluctlights/fl/world/items/item", `{"available":true,"placeId":"place","expectedRevision":0,"idempotencyKey":"item-config","reason":"Stale version"}`, false},
		{"DELETE", "/api/fluctlights/fl/npcs/npc", `{"expectedRevision":1,"idempotencyKey":"key","reason":"Remove"}`, true},
	}
	for _, tt := range tests {
		var body map[string]any
		if err := json.Unmarshal([]byte(tt.body), &body); err != nil {
			t.Fatal(err)
		}
		if actual := validWorldSurfaceBody(tt.method, tt.path, body); actual != tt.valid {
			t.Fatalf("%s %s valid=%v want=%v", tt.method, tt.path, actual, tt.valid)
		}
	}
}

func TestWorldCategoryTransportMapsOnlyTheDeclaredEnvelope(t *testing.T) {
	body := map[string]any{"venueCategories": []any{map[string]any{"id": "shops", "name": "商店"}}, "expectedRevision": float64(2)}
	mapped := surfaceCommandBody("/api/fluctlights/fl/world/catalog/world", body)
	if mapped["venue_categories"] == nil || mapped["expected_revision"] != float64(2) {
		t.Fatal(mapped)
	}
}

func TestWorldConfigurationCopyTransportRequiresCompleteClosedSource(t *testing.T) {
	path := "/api/fluctlights/target/world/catalog"
	base := map[string]any{"name": "独立世界", "sourceWorldId": "world-source", "sourceFluctlightId": "source", "expectedSourceRevision": float64(2), "reason": "沿用世界配置", "idempotencyKey": "copy-key"}
	if !validWorldSurfaceBody("POST", path, base) {
		t.Fatal("complete copy rejected")
	}
	mapped := surfaceCommandBody(path, base)
	if mapped["source_world_id"] != "world-source" || mapped["source_fluctlight_id"] != "source" || mapped["expected_source_revision"] != float64(2) {
		t.Fatal("source envelope not mapped", mapped)
	}
	for _, key := range []string{"name", "sourceWorldId", "sourceFluctlightId", "expectedSourceRevision", "reason", "idempotencyKey"} {
		body := map[string]any{}
		for k, v := range base {
			body[k] = v
		}
		delete(body, key)
		if validWorldSurfaceBody("POST", path, body) {
			t.Fatal("missing copy field admitted", key)
		}
	}
	for key, value := range map[string]any{"expectedSourceRevision": 1.5, "ownerActorId": "forged", "places": []any{}, "lifeHistory": []any{}} {
		body := map[string]any{}
		for k, v := range base {
			body[k] = v
		}
		body[key] = value
		if validWorldSurfaceBody("POST", path, body) {
			t.Fatal("invalid copy field admitted", key)
		}
	}
}

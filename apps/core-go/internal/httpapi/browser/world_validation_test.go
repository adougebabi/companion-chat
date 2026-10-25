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

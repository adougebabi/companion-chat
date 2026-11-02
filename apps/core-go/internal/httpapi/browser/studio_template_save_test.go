package browser

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestStudioCharacterTemplateSaveValidatesSchema(t *testing.T) {
	handler := newSurfaceHandler()
	valid := map[string]any{"schema": "character-template.v1", "description": "一位成年书店店主"}
	encode := func(value any) string {
		t.Helper()
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	created := surfaceInvoke(handler, http.MethodPost, "/api/studio/resources", encode(map[string]any{"idempotencyKey": "template-valid", "kind": "character_template", "name": "模板", "body": valid}), true, true)
	if created.code != http.StatusCreated {
		t.Fatalf("valid save: %d %s", created.code, created.body)
	}
	id := stringValue(created.json(t)["id"])
	invalid := []any{
		nil,
		map[string]any{"description": "缺少 schema"},
		map[string]any{"schema": "character-template.v2", "description": "错误版本"},
		map[string]any{"schema": 1, "description": "错误类型"},
		map[string]any{"schema": "character-template.v1"},
		map[string]any{"schema": "character-template.v1", "description": 42},
		map[string]any{"schema": "character-template.v1", "description": "描述", "initialization": map[string]any{}},
		map[string]any{"schema": "character-template.v1", "initialization": map[string]any{"core_persona": map[string]any{"identity": map[string]any{"name": []any{}}}}},
		map[string]any{"schema": "character-template.v1", "initialization": map[string]any{"core_persona": map[string]any{"identity": map[string]any{"name": "书店店主"}, "life_profile": map[string]any{"relationship_seeds": []any{"独居", "养一只灰猫"}}}, "initial_relationships": []any{}}},
		map[string]any{"schema": "character-template.v1", "initialization": map[string]any{"core_persona": map[string]any{"identity": map[string]any{"name": "书店店主"}, "life_profile": map[string]any{"relationship_seeds": []any{map[string]any{"target_actor_id": "actor_self"}}}}, "initial_relationships": []any{}}},
	}
	for i, definition := range invalid {
		for _, method := range []string{http.MethodPost, http.MethodPut} {
			t.Run(fmt.Sprintf("%s/%d", method, i), func(t *testing.T) {
				path := "/api/studio/resources"
				body := map[string]any{"idempotencyKey": fmt.Sprintf("invalid-%s-%d", method, i), "name": "坏模板", "body": definition}
				if method == http.MethodPost {
					body["kind"] = "character_template"
				} else {
					path += "/" + id
					body["expectedVersion"] = "1" // Omitted kind must still validate the saved kind.
					if definition == nil {
						// A metadata-only PUT legitimately retains its existing valid body.
						body["body"] = map[string]any{}
					}
				}
				response := surfaceInvoke(handler, method, path, encode(body), true, true)
				if response.code != http.StatusBadRequest {
					t.Fatalf("invalid schema saved: %d %s", response.code, response.body)
				}
			})
		}
	}
	read := surfaceInvoke(handler, http.MethodGet, "/api/studio/resources/"+id, "", true, false)
	if read.code != http.StatusOK || read.json(t)["revision"] != float64(1) || mapValue(read.json(t)["body"])["schema"] != "character-template.v1" {
		t.Fatalf("failed PUT changed the draft: %d %s", read.code, read.body)
	}
	updated := surfaceInvoke(handler, http.MethodPut, "/api/studio/resources/"+id, encode(map[string]any{"idempotencyKey": "template-valid-update", "expectedVersion": "1", "name": "新模板", "body": valid}), true, true)
	if updated.code != http.StatusOK || updated.json(t)["revision"] != float64(2) {
		t.Fatalf("valid PUT: %d %s", updated.code, updated.body)
	}
	metadata := surfaceInvoke(handler, http.MethodPut, "/api/studio/resources/"+id, encode(map[string]any{"idempotencyKey": "template-metadata", "expectedVersion": "2", "name": "仅改资源名"}), true, true)
	if metadata.code != http.StatusOK || mapValue(metadata.json(t)["body"])["schema"] != "character-template.v1" {
		t.Fatalf("metadata PUT lost valid existing definition: %d %s", metadata.code, metadata.body)
	}
	place := surfaceInvoke(handler, http.MethodPost, "/api/studio/resources", `{"idempotencyKey":"place-retag","kind":"place","name":"地点","body":{}}`, true, true)
	if place.code != http.StatusCreated {
		t.Fatalf("place fixture: %d %s", place.code, place.body)
	}
	retag := surfaceInvoke(handler, http.MethodPut, "/api/studio/resources/"+stringValue(place.json(t)["id"]), `{"idempotencyKey":"invalid-retag","kind":"character_template","name":"模板","expectedVersion":"1"}`, true, true)
	if retag.code != http.StatusBadRequest {
		t.Fatalf("retag bypassed schema validation: %d %s", retag.code, retag.body)
	}
}

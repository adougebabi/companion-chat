package core

import (
	"strings"
	"testing"
)

func TestPersonaDetailSectionsKeepProfileScopeAndFullFacts(t *testing.T) {
	source := map[string]any{
		"identity":     map[string]any{"name": "摇光"},
		"extensions":   map[string]any{"special_ritual": "睡前整理画稿", "source_text": "PRIVATE_CARD", "records": []any{map[string]any{"source_digest": "PRIVATE_DIGEST", "meaning": "稳定资料"}}},
		"life_profile": map[string]any{"preferences": map[string]any{"drink": "喜欢咖啡，但不喜欢甜咖啡"}},
		"personality":  map[string]any{"expression": "共享"},
		"personality_system": map[string]any{"profiles": []any{
			map[string]any{"id": "warm", "personality": map[string]any{"expression": "温暖"}, "secrets": map[string]any{"past": "只属于温暖人格", "source_text": "PRIVATE_PROFILE_CARD"}},
			map[string]any{"id": "cool", "personality": map[string]any{"expression": "冷淡"}, "secrets": map[string]any{"past": "只属于冷淡人格"}},
		}},
	}
	warm, err := personaDetailSections(source, "warm")
	if err != nil {
		t.Fatal(err)
	}
	if got := stringValue(mapValue(warm["personality"])["expression"]); got != "温暖" {
		t.Fatalf("warm personality = %q", got)
	}
	if got := stringValue(mapValue(mapValue(warm["life_profile"])["preferences"])["drink"]); got != "喜欢咖啡，但不喜欢甜咖啡" {
		t.Fatalf("preference lost: %q", got)
	}
	if got := stringValue(mapValue(warm["profile.secrets"])["past"]); got != "只属于温暖人格" {
		t.Fatalf("warm secret = %q", got)
	}
	if strings.Contains(jsonString(warm), "只属于冷淡人格") {
		t.Fatal("foreign profile leaked")
	}
	if encoded := jsonString(warm); strings.Contains(encoded, "PRIVATE_CARD") || strings.Contains(encoded, "PRIVATE_DIGEST") || strings.Contains(encoded, "PRIVATE_PROFILE_CARD") || !strings.Contains(encoded, "睡前整理画稿") {
		t.Fatalf("detail sanitizer leaked bookkeeping or removed semantic extension: %s", encoded)
	}
	if _, err := personaDetailSections(source, "missing"); err == nil {
		t.Fatal("missing profile accepted")
	}
}

func TestPersonaDetailOutputContractRejectsMissingVersionAndCursor(t *testing.T) {
	definition := personaDetailCapabilityDefinition()
	valid := map[string]any{"profile_id": "warm", "source_revision": 1, "overlay_revision": 0, "section_id": "identity", "cursor": 0, "content": "{}", "has_more": false, "next_cursor": 2}
	if err := definition.ValidateOutput(valid); err != nil {
		t.Fatalf("valid read rejected: %v", err)
	}
	missingVersion := cloneMap(valid)
	delete(missingVersion, "source_revision")
	if err := definition.ValidateOutput(missingVersion); err == nil {
		t.Fatal("read without source version passed")
	}
	wrongCursor := cloneMap(valid)
	wrongCursor["next_cursor"] = "2"
	if err := definition.ValidateOutput(wrongCursor); err == nil {
		t.Fatal("string cursor passed")
	}
	list := map[string]any{"profile_id": "warm", "source_revision": 1, "overlay_revision": 0, "sections": []any{map[string]any{"section_id": "identity", "description": "shared identity"}}}
	if err := definition.ValidateOutput(list); err != nil {
		t.Fatalf("valid list rejected: %v", err)
	}
}

func TestPersonaDetailIndependentToolReadsCanonicalSource(t *testing.T) {
	fixture := newIndependentToolE2EFixture(t, "persona_detail")
	source := map[string]any{
		"identity":     map[string]any{"name": "摇光"},
		"extensions":   map[string]any{"special_ritual": "睡前整理画稿", "appearance": map[string]any{"hair_length": "旧长发"}, "source_text": "PRIVATE_CARD", "nested": []any{map[string]any{"projection_digest": "PRIVATE_DIGEST", "meaning": "稳定资料", "hair_color": "旧红发"}}},
		"life_profile": map[string]any{"preferences": map[string]any{"drink": strings.Repeat("喜欢咖啡，但不喜欢甜咖啡。", 200)}},
		"personality_system": map[string]any{"profiles": []any{
			map[string]any{"id": "warm", "voice": "温暖"},
			map[string]any{"id": "cool", "voice": "冷淡", "secrets": map[string]any{"past": "冷淡人格私有经历"}},
		}},
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlights SET core_persona=$2 WHERE id=$1`, fixture.fluctlightID, jsonBytes(source)); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlight_personality_runtime SET active_profile_id='warm' WHERE fluctlight_id=$1`, fixture.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `INSERT INTO public.fluctlight_profile_habits(fluctlight_id,profile_id,revision,habits_json,source_kind) VALUES($1,'warm',0,'[]','initialization'),($1,'cool',0,'[]','initialization')`, fixture.fluctlightID); err != nil {
		t.Fatal(err)
	}
	request := fixture.request(personaDetailCapabilityName, "list", map[string]any{"operation": "list"})
	request.WorkingProfileID = "warm"
	listed, err := fixture.app.ExecuteTool(fixture.ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if listed.Result.Status != "completed" {
		t.Fatalf("list status = %s", listed.Result.Status)
	}
	if strings.Contains(jsonString(listed.Result.Output), "profile.secrets") {
		t.Fatal("foreign profile section listed")
	}
	request = fixture.request(personaDetailCapabilityName, "extensions", map[string]any{"operation": "read", "section_id": "extensions"})
	request.WorkingProfileID = "warm"
	extensions, err := fixture.app.ExecuteTool(fixture.ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if encoded := jsonString(extensions.Result.Output); strings.Contains(encoded, "PRIVATE_CARD") || strings.Contains(encoded, "PRIVATE_DIGEST") || strings.Contains(encoded, "旧长发") || strings.Contains(encoded, "旧红发") || !strings.Contains(encoded, "睡前整理画稿") {
		t.Fatalf("Tool leaked private source bookkeeping: %s", encoded)
	}
	request = fixture.request(personaDetailCapabilityName, "read", map[string]any{"operation": "read", "section_id": "life_profile"})
	request.WorkingProfileID = "warm"
	first, err := fixture.app.ExecuteTool(fixture.ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	firstOutput := mapValue(first.Result.Output)
	if !firstOutput["has_more"].(bool) {
		t.Fatal("long section was not marked as partial")
	}
	if !strings.Contains(stringValue(firstOutput["content"]), "喜欢咖啡") {
		t.Fatal("source content absent")
	}
	request = fixture.request(personaDetailCapabilityName, "next", map[string]any{"operation": "read", "section_id": "life_profile", "cursor": firstOutput["next_cursor"]})
	request.WorkingProfileID = "warm"
	next, err := fixture.app.ExecuteTool(fixture.ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if intValue(mapValue(next.Result.Output)["next_cursor"]) <= intValue(firstOutput["next_cursor"]) {
		t.Fatal("cursor did not advance")
	}
	request = fixture.request(personaDetailCapabilityName, "foreign-section", map[string]any{"operation": "read", "section_id": "profile.secrets"})
	request.WorkingProfileID = "warm"
	if _, err := fixture.app.ExecuteTool(fixture.ctx, request); err == nil {
		t.Fatal("foreign section succeeded")
	}
	request = fixture.request(personaDetailCapabilityName, "foreign-owner", map[string]any{"operation": "list"})
	request.AuthorizationActorID = fixture.foreignOwnerID
	if _, err := fixture.app.ExecuteTool(fixture.ctx, request); err == nil {
		t.Fatal("foreign owner succeeded")
	}
	stale := -1
	request = fixture.request(personaDetailCapabilityName, "stale", map[string]any{"operation": "list"})
	request.ExpectedCorePersonaRevision = &stale
	if receipt, err := fixture.app.ExecuteTool(fixture.ctx, request); err == nil || receipt.Result.ErrorCode != "persona_detail_source_changed" {
		t.Fatalf("stale source status=%s code=%s err=%v", receipt.Result.Status, receipt.Result.ErrorCode, err)
	}
	fixture.requireNoModelRuns(t)
}

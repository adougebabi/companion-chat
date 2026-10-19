package core

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestCharacterTemplateSourceRejectsArchiveAndRetainsOriginalDefinition(t *testing.T) {
	definition := map[string]any{"schema": "character-template.v1", "description": strings.Repeat("完整描述", 2500)}
	text, err := characterTemplateSource(definition)
	if err != nil || text != definition["description"] {
		t.Fatal("bounded long template was changed", err)
	}
	for _, field := range []string{"analysis_id", "actor_user", "messages", "current_state"} {
		bad := map[string]any{"schema": "character-template.v1", "initialization": map[string]any{"core_persona": map[string]any{"identity": map[string]any{"name": "Test"}}, field: map[string]any{}}}
		if _, err := characterTemplateSource(bad); err == nil {
			t.Fatal("template accepted credentials or runtime state", field)
		}
	}
	definition["initialization"] = map[string]any{"core_persona": map[string]any{}}
	if _, err := characterTemplateSource(definition); err == nil {
		t.Fatal("ambiguous source accepted")
	}
}

func TestStudioCatalogueFiltersBeforePagesAndReadsExactPublication(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'isolated-test-hash','catalogue-test')`, f.ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES('catalogue-compiler','openai_compatible','http://catalogue-compiler.invalid','catalogue-test-secret','ready',now())`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('cognitive_assessment','catalogue-compiler','catalogue-test-model','structured_output,tool_calling',8192,30,'{}')`); err != nil {
		t.Fatal(err)
	}
	// A controlled compiler response verifies persistence/replay only. It is
	// explicitly not evidence that a real initialization/compilation model ran.
	compileCalls := 0
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("persona_compilation_response", func(_ map[string]any) fakeProviderResult {
		compileCalls++
		return fakeProviderResult{Structured: map[string]any{"portrait_text": "Original template identity"}}
	})}
	create := func(kind, name string, definition map[string]any) map[string]any {
		t.Helper()
		r, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/studio/resources", nil, f.ownerID, map[string]any{"kind": kind, "name": name, "body": definition, "reason": "Catalogue test input", "idempotency_key": randomID("catalogue_test_")})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	template := map[string]any{"schema": "character-template.v1", "initialization": map[string]any{"schema_version": 2, "core_persona": map[string]any{"identity": map[string]any{"name": "Original template identity"}, "personality_system": map[string]any{"mode": "single"}}}}
	r := create("character_template", "Find %_ literal", template)
	id := stringValue(r["id"])
	pub, err := f.app.publishWorldResource(f.ctx, f.ownerID, id, map[string]any{"expected_revision": r["revision"], "reason": "Publish exact input", "idempotency_key": randomID("publish_")})
	if err != nil {
		t.Fatal(err)
	}
	// More recent nonmatching records must not consume a filtered page.
	for i := 0; i < 4; i++ {
		create("world", "Other definition", map[string]any{"description": "Catalogue test source"})
	}
	create("character_template", "Unpublished", map[string]any{})
	page, err := f.app.OwnerStudioResources(f.ctx, f.ownerID, url.Values{"kind": {"character_template"}, "q": {"%_"}, "published": {"true"}, "limit": {"1"}})
	if err != nil || len(arrayValue(page["items"])) != 1 || mapValue(arrayValue(page["items"])[0])["id"] != id || page["next_cursor"] != nil {
		t.Fatal("filter ran after page or treated wildcard as SQL", page, err)
	}
	// Editing the working draft cannot change a selected publication.
	updated, err := f.app.DispatchOwnerSurface(f.ctx, "PUT", "/internal/studio/resources/"+id, nil, f.ownerID, map[string]any{"name": "New draft", "kind": "character_template", "expected_version": r["draft_version"], "body": map[string]any{"schema": "character-template.v1", "description": "New unpublished description"}, "reason": "Keep publication immutable", "idempotency_key": randomID("update_")})
	if err != nil {
		t.Fatal(err)
	}
	exact, err := f.app.OwnerStudioPublication(f.ctx, f.ownerID, id, stringValue(pub["published_version"]))
	if err != nil || exact["name"] != "Find %_ literal" || jsonString(exact["body"]) != jsonString(template) {
		t.Fatal("working draft replaced published source", exact, err)
	}
	if _, err := f.app.OwnerStudioPublication(f.ctx, f.foreignOwnerID, id, stringValue(pub["published_version"])); err == nil {
		t.Fatal("foreign Owner read publication")
	}
	analysis, err := f.app.AnalyzePublishedCharacterTemplate(f.ctx, f.ownerID, id, stringValue(pub["published_version"]))
	if err != nil || mapValue(mapValue(analysis["core_persona"])["identity"])["name"] != "Original template identity" {
		t.Fatal("analysis did not consume exact publication", analysis, err)
	}
	var raw []byte
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT classification_evidence FROM public.fluctlight_initialization_sources WHERE id=$1 AND owner_actor_id=$2`, analysis["analysis_id"], f.ownerID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if mapValue(decodeObject(raw)["template_source"])["resource_id"] != id {
		t.Fatal("template producer source lost", string(raw))
	}
	initialization := cloneMap(analysis)
	delete(initialization, "analysis_id")
	delete(initialization, "correlation_id")
	requestID := "catalogue-activation-replay"
	fid := StableFluctlightID(f.ownerID, requestID)
	archived, err := f.app.studioCatalogCommand(f.ctx, f.ownerID, id, "archive", map[string]any{"expected_revision": updated["revision"], "reason": "Invalidate unlinked creation preview", "idempotency_key": randomID("archive_preview_")})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.app.CreateFluctlight(f.ctx, f.ownerID, fid, "Created template", "llm_defined", stringValue(analysis["analysis_id"]), initialization, nil, nil, "Asia/Shanghai")
	var blocked *SurfaceError
	if !errors.As(err, &blocked) || blocked.Code != "studio_resource_archived" || compileCalls != 0 {
		t.Fatal("archived preview activated or consumed compiler", err, compileCalls)
	}
	if _, err := f.app.studioCatalogCommand(f.ctx, f.ownerID, id, "restore", map[string]any{"expected_revision": archived["revision"], "reason": "Restore exact creation source", "idempotency_key": randomID("restore_preview_")}); err != nil {
		t.Fatal(err)
	}
	first, err := f.app.CreateFluctlight(f.ctx, f.ownerID, fid, "Created template", "llm_defined", stringValue(analysis["analysis_id"]), initialization, nil, nil, "Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	replay, err := f.app.CreateFluctlight(f.ctx, f.ownerID, fid, "Created template", "llm_defined", stringValue(analysis["analysis_id"]), initialization, nil, nil, "Asia/Shanghai")
	if err != nil || replay.ID != first.ID {
		t.Fatal("activation replay created or rejected identical character", replay, err)
	}
	if compileCalls != 1 {
		t.Fatal("replay repeated a completed compiler stage", compileCalls)
	}
	if _, err := f.app.AnalyzePublishedCharacterTemplate(f.ctx, f.ownerID, id, "missing"); err == nil {
		t.Fatal("missing version used a draft fallback")
	}
	_, err = f.app.OwnerStudioResources(f.ctx, f.ownerID, url.Values{"cursor": {"invalid"}})
	var invalidCursor *SurfaceError
	if !errors.As(err, &invalidCursor) || invalidCursor.Code != "studio_cursor_invalid" {
		t.Fatal("invalid cursor became an empty catalogue", err)
	}
}

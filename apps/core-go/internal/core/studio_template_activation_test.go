package core

import (
	"strings"
	"testing"
)

func TestStudioTemplateActivationRenamesProfilesAndCompiledPortrait(t *testing.T) {
	const templateName, chosenName = "FT-模板旧姓名", "FT-Studio·书店店主 雨"
	calls := 0
	router := newFakeProviderRouter().on("persona_compilation_response", func(payload map[string]any) fakeProviderResult {
		calls++
		// Echo only names actually delivered through the physical compiler
		// request, so the resulting portrait exposes a stale profile identity.
		request := jsonString(payload["messages"])
		if strings.Contains(request, templateName) || !strings.Contains(request, chosenName) {
			t.Fatalf("compiler consumed stale template names: %s", request)
		}
		return fakeProviderResult{Structured: map[string]any{"portrait_text": chosenName + "，一位温和的成年书店店主。"}}
	})
	ctx, repo, app := personaPreparationFixture(t, router)
	definition := map[string]any{"schema": "character-template.v1", "initialization": map[string]any{"core_persona": map[string]any{
		"identity": map[string]any{"name": templateName},
		"personality_system": map[string]any{"mode": "single", "active_profile_id": "default", "profiles": []any{map[string]any{
			"id": "default", "name": templateName, "identity": map[string]any{"name": templateName, "nickname": "小雨"},
			"personality": map[string]any{"description": "温和细致"},
		}}},
	}}}
	created, err := app.DispatchOwnerSurface(ctx, "POST", "/internal/studio/resources", nil, "prep-owner", map[string]any{"kind": "character_template", "name": "模板资源", "body": definition, "reason": "Regression fixture", "idempotency_key": "template-create"})
	if err != nil {
		t.Fatal(err)
	}
	published, err := app.publishWorldResource(ctx, "prep-owner", stringValue(created["id"]), map[string]any{"expected_revision": created["revision"], "reason": "Regression fixture", "idempotency_key": "template-publish"})
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := app.AnalyzePublishedCharacterTemplate(ctx, "prep-owner", stringValue(created["id"]), stringValue(published["published_version"]))
	if err != nil {
		t.Fatal(err)
	}
	source := stringValue(analysis["analysis_id"])
	delete(analysis, "analysis_id")
	delete(analysis, "correlation_id")
	before := jsonString(analysis)
	for i := 0; i < 2; i++ {
		fl, err := app.AdmitFluctlight(ctx, "prep-owner", "studio-renamed", chosenName, "llm_defined", source, analysis, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		profiles := arrayValue(mapValue(fl.CorePersona["personality_system"])["profiles"])
		if fl.Identity["name"] != chosenName || len(profiles) != 1 {
			t.Fatalf("activation identity: %#v", fl.CorePersona)
		}
		profile := mapValue(profiles[0])
		if profile["name"] != chosenName || mapValue(profile["identity"])["name"] != chosenName || profile["id"] != "default" || mapValue(profile["identity"])["nickname"] != "小雨" {
			t.Fatalf("profile name or independent identity changed: %#v", profile)
		}
	}
	if jsonString(analysis) != before {
		t.Fatal("activation rewrote frozen analysis")
	}
	var intent string
	if err := repo.Pool().QueryRow(ctx, `SELECT intent_id FROM public.platform_workflow_intents WHERE intent_type='persona.compile' AND payload->>'fluctlight_id'='studio-renamed'`).Scan(&intent); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := app.ProcessPersonaPreparationIntent(ctx, intent); err != nil {
			t.Fatal(err)
		}
	}
	var portrait, revisionProfileName, revisionIdentityName string
	if err := repo.Pool().QueryRow(ctx, `SELECT compiled_json->>'portrait_text' FROM public.fluctlight_working_personas WHERE fluctlight_id='studio-renamed' AND profile_id='default'`).Scan(&portrait); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(portrait, templateName) || !strings.Contains(portrait, chosenName) || calls != 1 {
		t.Fatalf("compiled portrait retained template name or replay compiled twice: %q calls=%d", portrait, calls)
	}
	if err := repo.Pool().QueryRow(ctx, `SELECT core_persona#>>'{personality_system,profiles,0,name}',core_persona#>>'{personality_system,profiles,0,identity,name}' FROM public.fluctlight_foundation_revisions WHERE fluctlight_id='studio-renamed' AND revision=0`).Scan(&revisionProfileName, &revisionIdentityName); err != nil || revisionProfileName != chosenName || revisionIdentityName != chosenName {
		t.Fatal("foundation revision retained old profile names", revisionProfileName, revisionIdentityName, err)
	}
	var frozen []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT structured_projection FROM public.fluctlight_initialization_sources WHERE id=$1`, source).Scan(&frozen); err != nil || !strings.Contains(string(frozen), templateName) {
		t.Fatal("source provenance was overwritten", string(frozen), err)
	}
}

package core

import "testing"

func TestCreationIdentityPreservesOwnerNameOverModelInterpretation(t *testing.T) {
	model := map[string]any{"name": "青岚", "nickname": "FT-青岚", "timezone": "Asia/Shanghai", "notes": "名称前缀不是可删除的标记"}
	before := jsonString(model)
	for _, name := range []string{"FT-青岚", " ＦＴ-青岚 ", "青岚·FT/01"} {
		got := creationIdentity(model, "actual-character", name)
		if got["name"] != name || got["nickname"] != "FT-青岚" || got["id"] != "actual-character" || got["notes"] != model["notes"] {
			t.Fatalf("Owner name or independent identity fields changed: %#v", got)
		}
	}
	if jsonString(model) != before {
		t.Fatal("creation rewrote the frozen initialization response")
	}
	if got := creationIdentity(model, "actual-character", ""); got["name"] != "青岚" {
		t.Fatalf("absent explicit name discarded model name: %#v", got)
	}
}

func TestLLMDefinedActivationPersistsVerbatimOwnerNameAndReplays(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	app := &App{DB: repository}
	const ownerID, characterID, name = "verbatim-owner", "verbatim-character", "FT-青岚"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active')`, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'hash','revision-1')`, ownerID); err != nil {
		t.Fatal(err)
	}
	analysis, err := app.ImportInitializationJSON(ctx, ownerID, map[string]any{"core_persona": map[string]any{"identity": map[string]any{"name": "青岚", "nickname": name}}})
	if err != nil {
		t.Fatal(err)
	}
	source := stringValue(analysis["analysis_id"])
	delete(analysis, "analysis_id")
	delete(analysis, "correlation_id")
	before := jsonString(analysis)
	for attempt := 0; attempt < 2; attempt++ {
		created, err := app.AdmitFluctlight(ctx, ownerID, characterID, name, "llm_defined", source, analysis, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if created.Identity["name"] != name || mapValue(created.CorePersona["identity"])["name"] != name || created.Identity["nickname"] != name {
			t.Fatalf("activation/replay used model-shortened name: %#v", created.Identity)
		}
	}
	var worlds int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.life_worlds`).Scan(&worlds); err != nil || worlds != 0 {
		t.Fatal("D8 LLM-defined activation implicitly created a world", worlds, err)
	}
	if jsonString(analysis) != before {
		t.Fatal("activation mutated the source card, breaking replay identity")
	}
	var persistedName, revisionName string
	if err := repository.Pool().QueryRow(ctx, `SELECT f.identity->>'name',r.core_persona->'identity'->>'name' FROM public.fluctlights f JOIN public.fluctlight_foundation_revisions r ON r.fluctlight_id=f.id AND r.revision=0 WHERE f.id=$1`, characterID).Scan(&persistedName, &revisionName); err != nil {
		t.Fatal(err)
	}
	if persistedName != name || revisionName != name {
		t.Fatalf("persistent authorities disagree: current=%q revision=%q", persistedName, revisionName)
	}
}

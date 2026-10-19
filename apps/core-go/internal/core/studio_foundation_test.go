package core

import (
	"errors"
	"github.com/jackc/pgx/v5"
	"net/http"
	"testing"
)

func TestFoundationNewProfileHabitsStageWithoutResettingExistingState(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	current, err := f.app.DB.GetFluctlight(f.ctx, f.fluctlightID, f.ownerID)
	if err != nil {
		t.Fatal(err)
	}
	core := cloneMap(current.CorePersona)
	core["personality_system"] = map[string]any{"mode": "multiple", "active_profile_id": "default", "profiles": []any{map[string]any{"id": "new-a", "voice": "calm"}, map[string]any{"id": "new-b", "voice": "direct"}}}
	core["life_profile"] = map[string]any{"life_habits": []any{"new profile initial habit"}}
	input, err := f.app.foundationCompilationInputForProfile(f.ctx, f.fluctlightID, core, current.CurrentRevision+1, "new-a", true)
	if err != nil || input.EffectiveLifeHabits == nil || jsonString(*input.EffectiveLifeHabits) != jsonString([]any{"new profile initial habit"}) {
		t.Fatal(input, err)
	}
	var exists bool
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_profile_habits WHERE fluctlight_id=$1 AND profile_id='new-a')`, f.fluctlightID).Scan(&exists); err != nil || exists {
		t.Fatal("precompile wrote runtime habits", exists, err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_profile_habits SET revision=7,habits_json='["existing effective habit"]' WHERE fluctlight_id=$1 AND profile_id='default'`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		return initializeAddedProfileHabitsTx(f.ctx, tx, f.fluctlightID, "foundation-new-profile", core)
	}); err != nil {
		t.Fatal(err)
	}
	habits, revision, err := readProfileHabits(f.ctx, f.repository.Pool(), f.fluctlightID, "default")
	if err != nil || revision != 7 || jsonString(habits) != jsonString([]any{"existing effective habit"}) {
		t.Fatal("existing habits reset", habits, revision, err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `DELETE FROM public.fluctlight_profile_habits WHERE fluctlight_id=$1 AND profile_id='default'`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.foundationCompilationInputForProfile(f.ctx, f.fluctlightID, core, current.CurrentRevision+1, "default", true); err == nil {
		t.Fatal("existing missing habits silently became new-profile defaults")
	}
}

func TestStudioFoundationPatchKeepsExplicitValuesAndStablePaths(t *testing.T) {
	p := worldPublication{Kind: "persona_fragment", Body: map[string]any{"schema": "persona-fragment.v1", "changes": map[string]any{"identity": map[string]any{"notes": "explicit", "custom/key~": nil}, "personality": map[string]any{"openness": 0.25}}}}
	patch, err := foundationDefinitionChanges(p)
	if err != nil || len(patch) != 2 {
		t.Fatal(patch, err)
	}
	fields := foundationDefinitionFields(map[string]any{"identity": map[string]any{"notes": "old"}}, patch)
	if len(fields) != 3 || fields[0].Key != "/identity/custom~1key~0" {
		t.Fatal(fields)
	}
	chosen := foundationFieldPatch(fields, map[string]string{"/identity/notes": "keep"})
	if _, present := mapValue(chosen["identity"])["notes"]; present {
		t.Fatal("keep field copied into patch", chosen)
	}
	if _, present := mapValue(chosen["identity"])["custom/key~"]; !present {
		t.Fatal("explicit null was omitted")
	}
	if mapValue(chosen["personality"])["openness"] != 0.25 {
		t.Fatal(chosen)
	}
	for _, key := range []string{"current_state", "provenance", "id", "analysis_id"} {
		bad := cloneMap(p.Body)
		bad["changes"] = map[string]any{key: map[string]any{"id": "forged"}}
		if _, err := foundationDefinitionChanges(worldPublication{Kind: p.Kind, Body: bad}); err == nil {
			t.Fatal("nondefinition admitted", key)
		}
	}
	if validateFoundationChanges(map[string]any{"personality_system": map[string]any{"active_profile_id": "forged"}}) == nil {
		t.Fatal("template grants current profile switching")
	}
	structured := worldPublication{Kind: "character_template", Body: map[string]any{"schema": "character-template.v1", "initialization": map[string]any{"core_persona": map[string]any{"identity": map[string]any{"name": "Explicit only"}, "personality_system": map[string]any{"mode": "single", "active_profile_id": "default"}}}}}
	changes, err := foundationDefinitionChanges(structured)
	if err != nil || len(mapValue(changes["identity"])) != 1 || changes["personality"] != nil {
		t.Fatal("application invented normalized fields", changes, err)
	}
	if _, exists := mapValue(changes["personality_system"])["active_profile_id"]; exists {
		t.Fatal("initial selector became runtime authority")
	}
}

func TestStudioFoundationProposalAcceptAndReplayPreserveEffectiveLife(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "foundation-studio-compiler")
	calls := 0
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("persona_compilation_response", func(_ map[string]any) fakeProviderResult {
		calls++
		return fakeProviderResult{Structured: map[string]any{"portrait_text": "Compiled accepted definition"}}
	})}
	current, err := f.app.DB.GetFluctlight(f.ctx, f.fluctlightID, f.ownerID)
	if err != nil {
		t.Fatal(err)
	}
	definition := map[string]any{"schema": "persona-fragment.v1", "changes": map[string]any{"identity": map[string]any{"name": "Published name"}, "personality": map[string]any{"openness": 0.17}}}
	r, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/studio/resources", nil, f.ownerID, map[string]any{"kind": "persona_fragment", "name": "Explicit fragment", "body": definition, "reason": "Test source", "idempotency_key": "foundation-source"})
	if err != nil {
		t.Fatal(err)
	}
	id := stringValue(r["id"])
	pub, err := f.app.publishWorldResource(f.ctx, f.ownerID, id, map[string]any{"expected_revision": r["revision"], "reason": "Publish", "idempotency_key": "foundation-publish"})
	if err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"fluctlight_id": f.fluctlightID, "resource_version": pub["published_version"], "expected_instance_revision": current.CurrentRevision, "reason": "Choose exact definition", "idempotency_key": "foundation-apply"}
	preview, err := f.app.DispatchWorldApply(f.ctx, f.ownerID, id, body, true)
	if err != nil || preview["target_kind"] != "foundation" {
		t.Fatal(preview, err)
	}
	choices := []any{}
	for _, raw := range arrayValue(preview["diff"]) {
		field := stringValue(mapValue(raw)["field"])
		decision := "override"
		if field == "/personality/openness" {
			decision = "keep"
		}
		choices = append(choices, map[string]any{"field": field, "decision": decision})
	}
	body["choices"] = choices
	before, bRev, wRev, err := f.app.readEffectiveLifeSnapshot(f.ctx, f.fluctlightID, f.app.now())
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := f.app.DispatchWorldApply(f.ctx, f.ownerID, id, body, false)
	if err != nil || proposal["applied"] != false || proposal["status"] != "proposed" || calls != 0 {
		t.Fatal("proposal claims application or runs model", proposal, calls, err)
	}
	pid := stringValue(proposal["proposal_id"])
	row, err := f.app.OwnerGovernanceObject(f.ctx, f.ownerID, f.fluctlightID, "foundation", pid)
	if err != nil || row["source_resource_id"] != id || row["source_version"] != pub["published_version"] || row["base_snapshot"] == nil {
		t.Fatal(row, err)
	}
	_, err = f.app.studioCatalogCommand(f.ctx, f.ownerID, id, "archive", map[string]any{"expected_revision": pub["revision"], "reason": "Pending source guard", "idempotency_key": "foundation-archive"})
	var blocked *SurfaceError
	if !errors.As(err, &blocked) || blocked.Code != "resource_referenced" {
		t.Fatal("pending source archived", err)
	}
	expected := current.CurrentRevision
	accepted, err := f.app.SetFoundationDecisionExpected(f.ctx, f.ownerID, f.fluctlightID, pid, "accept", "Accept exact source", &expected)
	if err != nil || accepted["status"] != "accepted" || calls == 0 {
		t.Fatal("compiler did not commit", accepted, calls, err)
	}
	after, err := f.app.DB.GetFluctlight(f.ctx, f.fluctlightID, f.ownerID)
	if err != nil || after.Identity["name"] != "Published name" || jsonString(after.Personality["openness"]) != jsonString(current.Personality["openness"]) {
		t.Fatal(after, err)
	}
	count := calls
	replay, err := f.app.DispatchWorldApply(f.ctx, f.ownerID, id, body, false)
	if err != nil || replay["proposal_id"] != pid || calls != count {
		t.Fatal("accepted replay changed command baseline", replay, err)
	}
	observed, ob, ow, err := f.app.readEffectiveLifeSnapshot(f.ctx, f.fluctlightID, f.app.now())
	if err != nil || bRev != ob || wRev != ow || jsonString(before["body_fields"]) != jsonString(observed["body_fields"]) || jsonString(before["worn_items"]) != jsonString(observed["worn_items"]) {
		t.Fatal("definition reset effective life", err)
	}
	refs, err := f.app.DispatchOwnerSurface(f.ctx, "GET", "/internal/studio/resources/"+id, nil, f.ownerID, nil)
	if err != nil || len(arrayValue(refs["referenced_by"])) != 1 || mapValue(arrayValue(refs["referenced_by"])[0])["currently_effective"] != true {
		t.Fatal(refs, err)
	}
}

func TestStudioTextPreparationDoesNotCreateInitializationCredential(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "studio-definition-parse")
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().otherwise(func(_ map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: map[string]any{"core_persona": map[string]any{"identity": map[string]any{"name": "Parsed fictional adult"}, "personality_system": map[string]any{"mode": "single"}}}}
	})}
	definition := map[string]any{"schema": "character-template.v1", "description": "Create the complete definition of a fictional adult named Parsed fictional adult."}
	r, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/studio/resources", nil, f.ownerID, map[string]any{"kind": "character_template", "name": "Text definition", "body": definition, "reason": "Test source", "idempotency_key": "parse-source"})
	if err != nil {
		t.Fatal(err)
	}
	pub, err := f.app.publishWorldResource(f.ctx, f.ownerID, stringValue(r["id"]), map[string]any{"expected_revision": r["revision"], "reason": "Publish text", "idempotency_key": "parse-publish"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := f.app.DispatchStudioJobSurface(f.ctx, "POST", "/internal/studio/jobs", nil, f.ownerID, map[string]any{"resource_id": r["id"], "resource_revision": pub["revision"], "resource_version": pub["published_version"], "purpose": "foundation_prepare", "kind": "assistant", "name": "Text definition", "body": definition, "instruction": "Parse exact published definition", "reason": "Explicit parse", "idempotency_key": "parse-job"})
	if err != nil {
		t.Fatal(err)
	}
	var before int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_initialization_sources WHERE owner_actor_id=$1`, f.ownerID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.ProcessStudioGeneration(f.ctx, "studio_intent:"+stringValue(job["id"])+"_attempt_1"); err != nil {
		t.Fatal(err)
	}
	read, err := f.app.StudioJob(f.ctx, f.ownerID, stringValue(job["id"]))
	if err != nil || read["status"] != "completed" || mapValue(mapValue(read["result"])["body"])["initialization"] == nil {
		t.Fatal(read, err)
	}
	var after int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_initialization_sources WHERE owner_actor_id=$1`, f.ownerID).Scan(&after); err != nil || after != before {
		t.Fatal("Studio invalidated Owner initialization source", before, after, err)
	}
}

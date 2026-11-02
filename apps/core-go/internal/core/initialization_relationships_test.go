package core

import (
	"errors"
	"net/http"
	"testing"
)

func TestInitializationRejectsMalformedRelationshipSeedsBeforePersistence(t *testing.T) {
	const path = "core_persona.life_profile.relationship_seeds"
	for _, tc := range []struct {
		name, validationType, path string
		seeds                      any
	}{
		{"p2 prose seeds", "type_invalid", path + "[0]", []any{"独居", "养一只灰猫"}},
		{"object collection", "type_invalid", path, map[string]any{"target_actor_id": "actor_user"}},
		{"null collection", "type_invalid", path, nil},
		{"mixed entries", "type_invalid", path + "[1]", []any{map[string]any{"target_actor_id": "actor_user"}, 42}},
		{"null entry", "type_invalid", path + "[0]", []any{nil}},
		{"missing target", "required_value_missing", path + "[0].target_actor_id", []any{map[string]any{"summary": "朋友"}}},
		{"self target", "reference_invalid", path + "[0].target_actor_id", []any{map[string]any{"target_actor_id": "actor_self"}}},
		{"foreign target", "reference_invalid", path + "[0].target_actor_id", []any{map[string]any{"target_actor_id": "someone_else"}}},
		{"unknown profile", "reference_invalid", path + "[0].profile_id", []any{map[string]any{"target_actor_id": "actor_user", "profile_id": "ghost"}}},
		{"profile type", "type_invalid", path + "[0].profile_id", []any{map[string]any{"target_actor_id": "actor_user", "profile_id": 7}}},
	} {
		for _, populated := range []bool{false, true} {
			t.Run(tc.name+"/primary="+jsonString(populated), func(t *testing.T) {
				value := map[string]any{"core_persona": defaultCorePersona("", "书店店主"), "initial_relationships": []any{}}
				mapValue(mapValue(value["core_persona"])["life_profile"])["relationship_seeds"] = tc.seeds
				if populated {
					value["initial_relationships"] = []any{map[string]any{"target_actor_id": "actor_user"}}
				}
				// No DB or Provider is installed: validation must reject before either is used.
				app := &App{}
				_, err := app.AnalyzeDescription(t.Context(), "owner", jsonString(value))
				var failure *initializationAnalysisError
				if !errors.As(err, &failure) || failure.Code != "initialization_persona_invalid" || failure.ValidationType != tc.validationType || failure.Path != tc.path || failure.Retryable || failure.CorrelationID == "" {
					t.Fatalf("expected precise validation before persistence: %#v, %v", failure, err)
				}
				// A malformed analysis retained by an older client must also fail
				// validation before compilation or transaction writes.
				_, err = app.AdmitFluctlight(t.Context(), "owner", "character", "书店店主", "llm_defined", "initialization_source_0123456789abcdef0123456789abcdef", value, nil, nil)
				if !errors.As(err, &failure) || failure.Path != tc.path {
					t.Fatalf("old analysis bypassed activation validation: %v", err)
				}
				_, err = characterTemplateSource(map[string]any{"schema": "character-template.v1", "initialization": value})
				var surface *SurfaceError
				if !errors.As(err, &surface) || surface.Status != http.StatusBadRequest || mapValue(surface.Details["validation_error"])["path"] != tc.path {
					t.Fatalf("template save did not preserve validation: %#v, %v", surface, err)
				}
			})
		}
	}
}

func TestInitializationRelationshipReferencesValidatedInPrimaryCollection(t *testing.T) {
	for _, item := range []map[string]any{
		{"target_actor_id": "actor_self"},
		{"target_actor_id": "foreign_actor"},
		{"target_actor_id": "actor_user", "profile_id": "ghost"},
	} {
		value := map[string]any{"core_persona": defaultCorePersona("", "书店店主"), "initial_relationships": []any{item}}
		_, err := prepareInitializationResponse(value)
		var failure *initializationAnalysisError
		if !errors.As(err, &failure) || failure.ValidationType != "reference_invalid" {
			t.Fatalf("invalid primary relationship reached activation: %#v, %v", item, err)
		}
	}
}

func TestInitializationValidRelationshipSeedsPreserveDefinition(t *testing.T) {
	for _, primary := range []string{"absent", "empty", "populated"} {
		t.Run(primary, func(t *testing.T) {
			persona := defaultCorePersona("", "书店店主")
			mapValue(persona["personality_system"])["profiles"] = []any{completeInitializationProfile("calm")}
			seeds := []any{
				map[string]any{"target_actor_id": "actor_user", "summary": "共同阅读"},
				map[string]any{"target_actor_id": "owner", "profile_id": "calm", "role": map[string]any{"label": "朋友"}},
			}
			mapValue(persona["life_profile"])["relationship_seeds"] = seeds
			value := map[string]any{"core_persona": persona}
			if primary == "empty" {
				value["initial_relationships"] = []any{}
			} else if primary == "populated" {
				value["initial_relationships"] = []any{map[string]any{"target_actor_id": "actor_user", "role": "熟人"}}
			}
			original := jsonString(value)
			prepared, err := prepareInitializationResponse(value)
			if err != nil || !validInitialization(prepared) {
				t.Fatal("valid relationship definitions rejected", err)
			}
			if jsonString(value) != original {
				t.Fatal("validation mutated the source")
			}
			preparedSeeds := arrayValue(mapValue(mapValue(prepared["core_persona"])["life_profile"])["relationship_seeds"])
			if len(preparedSeeds) != 2 || mapValue(preparedSeeds[0])["summary"] != "共同阅读" || mapValue(preparedSeeds[1])["profile_id"] != "calm" || mapValue(mapValue(preparedSeeds[1])["role"])["label"] != "朋友" {
				t.Fatal("valid seed semantics were lost", preparedSeeds)
			}
			if _, err := characterTemplateSource(map[string]any{"schema": "character-template.v1", "initialization": value}); err != nil {
				t.Fatal("valid template rejected on save", err)
			}
		})
	}
}

func TestInitializationValidRelationshipSeedsPersistSharedAndProfileScopes(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	owner := "relationship-seeds-owner"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active')`, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'isolated-test-hash','relationship-seeds-test')`, owner); err != nil {
		t.Fatal(err)
	}
	app := newTestApp(t, repository, nil)
	seedCognitiveProviderRole(t, ctx, repository, "relationship-seeds-compiler")
	app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("persona_compilation_response", func(_ map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: map[string]any{"portrait_text": "书店店主"}}
	})}
	persona := defaultCorePersona("", "书店店主")
	seeds := []any{
		map[string]any{"target_actor_id": "actor_user", "role": map[string]any{"label": "朋友"}, "summary": "共同阅读"},
		map[string]any{"target_actor_id": "owner", "profile_id": "default", "role": map[string]any{"label": "搭档"}, "metrics": map[string]any{"trust": 0.6}},
	}
	mapValue(persona["life_profile"])["relationship_seeds"] = seeds
	value := map[string]any{"core_persona": persona, "initial_relationships": []any{}}
	before := jsonString(value)
	analysis, err := app.AnalyzeDescription(ctx, owner, before)
	if err != nil {
		t.Fatal("valid fallback seeds rejected", err)
	}
	if jsonString(value) != before || jsonString(mapValue(mapValue(analysis["core_persona"])["life_profile"])["relationship_seeds"]) != jsonString(seeds) {
		t.Fatal("analysis rewrote valid source seeds")
	}
	delete(analysis, "correlation_id")
	source := stringValue(analysis["analysis_id"])
	delete(analysis, "analysis_id")
	created, err := app.CreateFluctlight(ctx, owner, "relationship-seeds-character", "书店店主", "llm_defined", source, analysis, nil, nil, "Asia/Shanghai")
	if err != nil {
		t.Fatal("valid seeds failed activation", err)
	}
	var count int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.relationships WHERE owner_fluctlight_id=$1 AND target_actor_id=$2 AND ((profile_id IS NULL AND role->>'label'='朋友' AND summary='共同阅读') OR (profile_id='default' AND role->>'label'='搭档' AND metrics->>'trust'='0.6'))`, created.ID, owner).Scan(&count); err != nil || count != 2 {
		t.Fatalf("valid relationship scope/content lost: count=%d err=%v", count, err)
	}
}

package core

import (
	"testing"
	"time"
)

// TestInitializationDefaultFieldSourcesAreExplicit pins R11's default-value fix
// at the exact field-path level: a numeric that the source never declared must be
// recorded as a server default, while a declared key must not be.
func TestInitializationDefaultFieldSourcesAreExplicit(t *testing.T) {
	provenance := defaultProvenance()
	personality := map[string]any{"openness": 0.5, "neuroticism": 0.5}
	policy := map[string]any{"initiative": 0.5, "directness": 0.6}
	declaredPersonality := map[string]any{"openness": 0.5}
	declaredPolicy := map[string]any{"directness": 0.6}

	recordInitializationDefaultFieldSources(provenance,
		"fx",
		personality, policy, declaredPersonality, declaredPolicy,
		[]any{
			map[string]any{"description": "declared numbers", "importance": 0.7, "urgency": 0.2},
			map[string]any{"description": "no numbers"},
		},
		[]any{
			map[string]any{"action": "declared", "confidence": 0.9},
			map[string]any{"action": "missing"},
		})

	sources := mapValue(provenance["field_sources"])
	want := map[string]string{
		"core_persona.personality.neuroticism":                    defaultFieldSource,
		"core_persona.personality.openness":                       "",
		"core_persona.behavioral_policy.initiative":               defaultFieldSource,
		"core_persona.behavioral_policy.directness":               "",
		"fluctlight_goals.goal_initial_fx_1.importance":           defaultFieldSource,
		"fluctlight_goals.goal_initial_fx_1.urgency":              defaultFieldSource,
		"fluctlight_goals.goal_initial_fx_0.importance":           "",
		"fluctlight_intentions.intention_initial_fx_1.confidence": defaultFieldSource,
		"fluctlight_intentions.intention_initial_fx_0.confidence": "",
	}
	for path, expected := range want {
		got := stringValue(sources[path])
		if got != expected {
			t.Fatalf("field source %s = %q, want %q", path, got, expected)
		}
	}
	// A declared number must never be silently downgraded to a default.
	if _, marked := sources["core_persona.personality.openness"]; marked {
		t.Fatal("a declared personality trait was recorded as a server default")
	}
}

func TestBlankActivationCapturesDeviceTimezoneForLifeContext(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	app := &App{DB: repository}
	ownerID, fluctlightID := "device-zone-owner", "device-zone-fluctlight"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active')`, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'hash','revision-1')`, ownerID); err != nil {
		t.Fatal(err)
	}
	created, err := app.CreateFluctlight(ctx, ownerID, fluctlightID, "摇光", "blank_slate", "", nil, nil, nil, "America/New_York")
	if err != nil || stringValue(created.Identity["timezone"]) != "America/New_York" {
		t.Fatalf("activation did not capture device timezone: identity=%#v err=%v", created.Identity, err)
	}
	_, life, err := app.readLifeContextSnapshotAt(ctx, fluctlightID, time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC))
	if err != nil || stringValue(life["timezone"]) != "America/New_York" || stringValue(life["local_date"]) != "2026-07-01" {
		t.Fatalf("life context ignored activation timezone: life=%#v err=%v", life, err)
	}
}

// TestCreateFluctlightRecordsPersonalityDefaultsEndToEnd proves the recorder is
// actually wired into creation: a blank-slate instance declares no traits, so
// every trait must carry the default marker in the persisted provenance.
func TestCreateFluctlightRecordsPersonalityDefaultsEndToEnd(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	app := &App{DB: repository}
	ownerID, fluctlightID := "default-source-owner", "default-source-fluctlight"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active')`, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'hash','revision-1')`, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.CreateFluctlight(ctx, ownerID, fluctlightID, "摇光", "blank_slate", "", nil, nil, nil); err != nil {
		t.Fatalf("blank-slate creation failed: %v", err)
	}
	var raw string
	if err := repository.Pool().QueryRow(ctx, `SELECT provenance::text FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	sources := mapValue(mapValue(decodeObject([]byte(raw)))["field_sources"])
	for _, path := range []string{
		"core_persona.personality.openness",
		"core_persona.personality.neuroticism",
		"core_persona.behavioral_policy.initiative",
	} {
		if stringValue(sources[path]) != defaultFieldSource {
			t.Fatalf("persisted provenance does not mark %s as a server default: %#v", path, sources)
		}
	}
}

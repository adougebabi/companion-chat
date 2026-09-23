package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// multiProfileWorkingPersonaProjection builds the projection the real
// multi-profile card produces: two declared profiles with different personas,
// one of them active.
func multiProfileWorkingPersonaProjection() ContextProjection {
	return ContextProjection{
		FluctlightID: "working-persona-fluctlight",
		CorePersona: map[string]any{
			"identity": map[string]any{
				"name":             "摇光",
				"background_story": "在一座临海小城长大。",
			},
			"life_profile": map[string]any{"preferences": map[string]any{"place": "安静的书房"}},
			"personality_system": map[string]any{
				"active_profile_id": "twilight",
				"profiles": []any{
					map[string]any{
						"id": "spark", "name": "星火",
						"personality": map[string]any{"traits": map[string]any{"openness": 0.9}, "voice": "语速快"},
					},
					map[string]any{
						"id": "twilight", "name": "暮光",
						"personality": map[string]any{
							"traits": map[string]any{"description": "安静、句子短、行事谨慎"},
							"voice":  map[string]any{"description": "轻声、停顿多"},
						},
						"behavioral_policy": map[string]any{"response_style": "克制"},
						"secrets":           map[string]any{"information_asymmetry": map[string]any{"owner": "隐藏"}},
						"visual_identity":   map[string]any{"avatar": "INTERNAL_ASSET"},
					},
				},
			},
			"extensions": map[string]any{"special_ritual": "睡前整理画稿", "source_text": "PRIVATE_CARD"},
		},
		Personality:        map[string]any{"traits": map[string]any{"openness": 0.4}},
		BehavioralPolicy:   map[string]any{"response_style": "中性"},
		PersonalityRuntime: map[string]any{"active_profile_id": "twilight", "revision": 4},
	}
}

func TestWorkingPersonaKeepsDescriptiveTraitsVerbatim(t *testing.T) {
	projection := multiProfileWorkingPersonaProjection()
	working, trace := projectWorkingPersona(projection, "twilight")

	body := mapValue(working[workingPersonaBodyKey])
	personality := mapValue(body["personality"])
	traits := mapValue(personality["traits"])
	if got := stringValue(traits[evolutionDescriptiveValueKey]); got != "安静、句子短、行事谨慎" {
		t.Fatalf("descriptive traits were not preserved verbatim: %#v", personality)
	}
	voice := mapValue(personality["voice"])
	if got := stringValue(voice[evolutionDescriptiveValueKey]); got != "轻声、停顿多" {
		t.Fatalf("descriptive voice was not preserved verbatim: %#v", personality)
	}
	// The active profile declares no numeric traits: a number must not be
	// invented to fill the gap (F11).
	if _, exists := traits["openness"]; exists {
		t.Fatalf("an unsupported numeric trait was invented: %#v", traits)
	}
	if stringValue(mapValue(body["behavioral_policy"])["response_style"]) != "克制" {
		t.Fatalf("the profile behavioural policy was dropped: %#v", body)
	}
	if !containsString(trace.Verbatim, "personality.traits.description") {
		t.Fatalf("the verbatim trace did not record the descriptive trait: %#v", trace.Verbatim)
	}
}

func TestWorkingPersonaExcludesOtherPersonasAndVisualAssets(t *testing.T) {
	projection := multiProfileWorkingPersonaProjection()
	working, trace := projectWorkingPersona(projection, "twilight")
	serialized := jsonString(working)

	for _, leaked := range []string{"spark", "星火", "语速快", "INTERNAL_ASSET"} {
		if strings.Contains(serialized, leaked) {
			t.Fatalf("working persona leaked %q: %s", leaked, serialized)
		}
	}
	if !strings.Contains(serialized, "安静、句子短、行事谨慎") {
		t.Fatalf("working persona lost the active profile description: %s", serialized)
	}
	// Preserved on purpose: the Main call's existing decision-input contract
	// requires the active profile's structured secrets to stay visible.
	if !strings.Contains(serialized, "隐藏") {
		t.Fatalf("working persona lost the active profile decision input: %s", serialized)
	}
	if _, exists := working["persistent_switch"]; exists {
		t.Fatalf("the projection must not synthesise a persistent switch section: %#v", working)
	}
	if !containsString(trace.Excluded, "visual_identity") {
		t.Fatalf("the exclusion trace did not record the visual asset: %#v", trace.Excluded)
	}
	if got := stringValue(working[workingPersonaProfileIDKey]); got != "twilight" {
		t.Fatalf("working persona profile id = %q", got)
	}
}

func TestWorkingPersonaIsDeterministicForTheSameInput(t *testing.T) {
	projection := multiProfileWorkingPersonaProjection()
	first, firstTrace := projectWorkingPersona(projection, "twilight")
	second, secondTrace := projectWorkingPersona(projection, "twilight")
	if jsonString(first) != jsonString(second) || jsonString(firstTrace) != jsonString(secondTrace) {
		t.Fatal("projectWorkingPersona is not deterministic for identical input")
	}
	// The projection must never mutate the projection it reads.
	if stringValue(mapValue(projection.CorePersona["identity"])["name"]) != "摇光" {
		t.Fatalf("the source projection was mutated: %#v", projection.CorePersona)
	}
}

func TestWorkingPersonaAppliesActiveOverlayOfTheActiveProfile(t *testing.T) {
	projection := ContextProjection{
		FluctlightID: "working-persona-overlay",
		CorePersona: map[string]any{
			"identity": map[string]any{"name": "摇光"},
			"personality_system": map[string]any{
				"active_profile_id": "spark",
				"profiles": []any{map[string]any{
					"id": "spark", "name": "星火",
					"personality": map[string]any{"traits": map[string]any{"openness": 0.4}},
				}},
			},
		},
		PersonalityRuntime: map[string]any{"active_profile_id": "spark", "revision": 7},
		EffectivePersona: map[string]any{
			"profile_ref": "personality:ctx_0123456789abcdef0123456789abcdef", "authority_revision": 3,
			"personality": map[string]any{"traits": map[string]any{"openness": 0.85}},
		},
	}
	working, trace := projectWorkingPersona(projection, "spark")
	body := mapValue(working[workingPersonaBodyKey])
	traits := mapValue(mapValue(body["personality"])["traits"])
	if got := numberOrZero(traits["openness"]); got != 0.85 {
		t.Fatalf("the active overlay was not applied: %#v", traits)
	}
	if !trace.OverlayApplied || trace.OverlayRevision != 3 {
		t.Fatalf("overlay trace not recorded: %#v", trace)
	}
	if got := intValue(working[workingPersonaOverlayKey]); got != 3 {
		t.Fatalf("overlay revision not carried on the working persona: %#v", working)
	}
}

func TestWorkingPersonaDoesNotApplyAnOverlayToANonActiveProfile(t *testing.T) {
	projection := multiProfileWorkingPersonaProjection()
	projection.EffectivePersona = map[string]any{
		"authority_revision": 5,
		"personality":        map[string]any{"traits": map[string]any{"openness": 0.95}},
	}
	working, trace := projectWorkingPersona(projection, "spark")
	if trace.OverlayApplied {
		t.Fatalf("an overlay must not apply to a non-active profile: %#v", trace)
	}
	if strings.Contains(jsonString(working), "0.95") {
		t.Fatalf("an overlay leaked into a non-active profile: %#v", working)
	}
}

// TestWorkingPersonaAppliesReScopedOverlayToNonActiveSubject pins F-05: when a
// projection has been re-scoped for a non-active subject — the Takeover Judge
// control view of the handover candidate — the composed overlay carries an
// explicit profile_id and applies to that subject. Without it the Judge would
// only ever see the candidate's declared baseline, never the stance B would
// actually speak with after its accepted evolution.
func TestWorkingPersonaAppliesReScopedOverlayToNonActiveSubject(t *testing.T) {
	projection := multiProfileWorkingPersonaProjection() // active profile is twilight
	projection.EffectivePersona = map[string]any{
		"profile_id": "spark", "authority_revision": 9,
		"personality":       map[string]any{"traits": map[string]any{"openness": 0.77}},
		"behavioral_policy": map[string]any{"response_style": "直接"},
	}
	working, trace := projectWorkingPersona(projection, "spark")
	body := mapValue(working[workingPersonaBodyKey])
	traits := mapValue(mapValue(body["personality"])["traits"])
	if got := numberOrZero(traits["openness"]); got != 0.77 {
		t.Fatalf("the re-scoped overlay was not applied to the non-active subject: %#v", traits)
	}
	if !trace.OverlayApplied || trace.OverlayRevision != 9 {
		t.Fatalf("the overlay trace was not recorded for the re-scoped subject: %#v", trace)
	}
	behavior := mapValue(body["behavioral_policy"])
	if got := stringValue(behavior["response_style"]); got != "直接" {
		t.Fatalf("the re-scoped behavioral overlay was not applied: %#v", behavior)
	}
	// The persistent active profile is not the subject of this re-scope, so
	// the guarantee that the candidate overlay stays on the candidate is
	// exercised end-to-end by TestTakeoverControlViewMergesBAcceptedOverlay,
	// where the projection is genuinely re-scoped and the active profile is a
	// separate identity rather than a contradictory EffectivePersona.
}

// TestWorkingPersonaIgnoresOverlayWithoutProfileIDForNonActiveSubject pins the
// fallback: an EffectivePersona that carries no profile_id (the legacy active
// overlay shape) still never applies to a non-active subject, so the existing
// guarantee is preserved.
func TestWorkingPersonaIgnoresOverlayWithoutProfileIDForNonActiveSubject(t *testing.T) {
	projection := multiProfileWorkingPersonaProjection()
	projection.EffectivePersona = map[string]any{
		"authority_revision": 5,
		"personality":        map[string]any{"traits": map[string]any{"openness": 0.95}},
	}
	working, trace := projectWorkingPersona(projection, "spark")
	if trace.OverlayApplied {
		t.Fatalf("a legacy overlay without profile_id must not apply to a non-active subject: %#v", trace)
	}
	if strings.Contains(jsonString(working), "0.95") {
		t.Fatalf("a legacy overlay leaked into a non-active subject: %#v", working)
	}
}

// TestWorkingPersonaCanonicalizesFlatTraitsShape pins the R11 shape fix: a
// declared baseline that stores traits flat and an overlay that stores them
// nested must reach the Provider under one canonical shape. Before the fix the
// model saw "openness" from the baseline and "traits.openness" from the overlay.
func TestWorkingPersonaCanonicalizesFlatTraitsShape(t *testing.T) {
	projection := ContextProjection{
		FluctlightID: "working-persona-flat-traits",
		CorePersona: map[string]any{
			"identity": map[string]any{"name": "摇光"},
			"personality_system": map[string]any{
				"mode": "single", "active_profile_id": "default", "profiles": []any{},
			},
		},
		Personality:        map[string]any{"openness": 0.7, "extraversion": 0.3},
		BehavioralPolicy:   map[string]any{"response_style": "克制"},
		PersonalityRuntime: map[string]any{"active_profile_id": "default", "revision": 2},
	}
	working, _ := projectWorkingPersona(projection, "")

	personality := mapValue(mapValue(working[workingPersonaBodyKey])["personality"])
	traits := mapValue(personality["traits"])
	if got := numberOrZero(traits["openness"]); got != 0.7 {
		t.Fatalf("flat trait was not mirrored into the canonical traits.* shape: %#v", personality)
	}
	if got := numberOrZero(traits["extraversion"]); got != 0.3 {
		t.Fatalf("flat trait was not mirrored into the canonical traits.* shape: %#v", personality)
	}
	// The flat key stays available for compatibility; the fix is additive.
	if got := numberOrZero(personality["openness"]); got != 0.7 {
		t.Fatalf("the flat trait key was dropped: %#v", personality)
	}
	// An empty expression carrier must not survive into the Provider payload.
	if _, exists := personality["expression"]; exists {
		t.Fatalf("an empty canonical carrier leaked into the persona: %#v", personality)
	}
}

// TestWorkingPersonaDoesNotInventTraitsFromProse is the F11 half of the same fix:
// when the subject declares traits as prose, the canonical
// materialisation must keep the prose verbatim and must not fabricate numbers
// from any flat sibling key.
func TestWorkingPersonaDoesNotInventTraitsFromProse(t *testing.T) {
	projection := ContextProjection{
		FluctlightID: "working-persona-prose-traits",
		CorePersona: map[string]any{
			"identity": map[string]any{"name": "摇光"},
			"personality_system": map[string]any{
				"active_profile_id": "twilight",
				"profiles": []any{map[string]any{
					"id": "twilight", "name": "暮光",
					"personality": map[string]any{"traits": "安静、句子短、行事谨慎"},
				}},
			},
		},
		Personality:        map[string]any{"openness": 0.4},
		BehavioralPolicy:   map[string]any{"response_style": "中性"},
		PersonalityRuntime: map[string]any{"active_profile_id": "twilight", "revision": 1},
	}
	working, _ := projectWorkingPersona(projection, "")

	personality := mapValue(mapValue(working[workingPersonaBodyKey])["personality"])
	traits := mapValue(personality["traits"])
	if got := stringValue(traits[evolutionDescriptiveValueKey]); got != "安静、句子短、行事谨慎" {
		t.Fatalf("prose traits were not preserved verbatim: %#v", personality)
	}
	if _, exists := traits["openness"]; exists {
		t.Fatalf("a numeric trait was fabricated from a flat sibling key: %#v", traits)
	}
}

func TestProviderSystemPersonaHasNoTakeoverRules(t *testing.T) {
	projection := multiProfileWorkingPersonaProjection()
	system := mapValue(projection.CorePersona["personality_system"])
	system["takeover_rules"] = []any{map[string]any{
		"id": "public_challenge", "kind": "turn_takeover", "version": personaTakeoverRuleVersion, "condition": "遇到强烈的公开质疑时星火可强制接管", "target_profile_id": "spark",
	}}
	projection.CorePersona["personality_system"] = system

	filtered := filterCorePersona(systemPersonaForLegacyProjectionForTest(projection, workingPersonaMainTurnSchema))
	serialized := jsonString(filtered)
	for _, leaked := range []string{"takeover_rules", "public_challenge", "强制接管"} {
		if strings.Contains(serialized, leaked) {
			t.Fatalf("takeover material reached the Main System Persona (%q): %s", leaked, serialized)
		}
	}
	if !strings.Contains(serialized, "安静、句子短、行事谨慎") {
		t.Fatalf("the working persona was dropped with the takeover rules: %s", serialized)
	}
}

func TestProviderSystemPromptHasSinglePersonaSource(t *testing.T) {
	projection := multiProfileWorkingPersonaProjection()
	rendered := renderProviderSystem(nil, filterCorePersona(systemPersonaForLegacyProjectionForTest(projection, workingPersonaMainTurnSchema)), nil, "cognitive_assessment")

	if strings.Count(rendered, "# 人格设定") != 1 {
		t.Fatalf("expected exactly one persona section: %s", rendered)
	}
	if !strings.Contains(rendered, "安静、句子短、行事谨慎") {
		t.Fatalf("the working persona was not rendered: %s", rendered)
	}
	if strings.Contains(rendered, "语速快") || strings.Contains(rendered, "INTERNAL_ASSET") {
		t.Fatalf("an inactive profile or a visual asset leaked into the system persona: %s", rendered)
	}
	// The roster survives as identifiers only, so Main can still name profiles.
	if !strings.Contains(rendered, "id: twilight") || strings.Contains(rendered, "描述性内容") {
		t.Fatalf("the identifier-only roster is wrong: %s", rendered)
	}
}

func TestProviderRuntimeContextCarriesNoSecondPersonaCopy(t *testing.T) {
	projection := multiProfileWorkingPersonaProjection()
	input := workingMemoryInputFromProjection(projection, nil, nil)
	for _, fragment := range input.RuntimeFacts {
		kind := stringValue(mapValue(fragment.Content)["kind"])
		switch kind {
		case "core_persona", "effective_persona", "personality_system":
			t.Fatalf("the runtime context repeats persona material (%q): %#v", kind, fragment.Content)
		}
	}
	// The runtime identity is still available so the model knows which profile
	// is speaking.
	compact := compactCognitionContext(projection)
	system := mapValue(compact["personality_system"])
	if stringValue(system["active_profile_id"]) != "twilight" {
		t.Fatalf("the runtime persona identity was dropped: %#v", system)
	}
	if _, exists := system["profiles"]; exists {
		t.Fatalf("the runtime persona still clones the profile roster: %#v", system)
	}
	if _, exists := system["active_profile"]; exists {
		t.Fatalf("the runtime persona still appends the active profile object: %#v", system)
	}
}

// denseLifeProfileFixture is a real-shaped dense card life_profile carrying
// every declared top-level key with sizeable descriptive content. It is the
// input for the F-06 convergence evidence.
func denseLifeProfileFixture() map[string]any {
	return map[string]any{
		"appearance": map[string]any{
			"description":              "栗色卷发、金棕色眼睛、165cm",
			"daily_outfit_preferences": []any{"宽松卫衣", "工装短裤", "沾颜料的帆布鞋"},
			"chest_cup":                "B",
		},
		"social_background": map[string]any{
			"birthplace": "成都",
			"childhood":  "童年长期搬家让她用绘画保留记忆",
			"occupation": "独立插画师",
		},
		"preferences": map[string]any{
			"place":           "安静的书房",
			"creative_medium": "数字插画",
		},
		"life_habits": map[string]any{
			"routines":       []any{"反复检查门锁"},
			"sleep_schedule": "晚睡晚起",
		},
		"recurring_commitments": map[string]any{
			"weekly_publish": "每周发一次画作",
		},
		"relationship_seeds": map[string]any{
			"actor_user": "信任但保留边界的合作伙伴",
		},
		"character_constraints": map[string]any{
			"safety":          "不泄露真实住址",
			"self_protection": "自我保护优先",
		},
		"media_preferences": map[string]any{
			"image_style": "水彩质感",
		},
	}
}

// TestWorkingPersonaLifeProfileKeepsOnlyDecisionKeys pins F-06: the Working
// Persona injects only the life_profile keys a per-turn decision needs
// (preferences + character_constraints). Appearance, social background, life
// habits, recurring commitments, relationship seeds and media preferences are
// large domain semantics that reach the model through their own task-scoped
// projections instead of being injected every turn (request.md:216,
// design.md:401).
func TestWorkingPersonaLifeProfileKeepsOnlyDecisionKeys(t *testing.T) {
	projection := ContextProjection{
		FluctlightID: "working-persona-life-profile-trim",
		CorePersona: map[string]any{
			"identity":           map[string]any{"name": "苏晚"},
			"life_profile":       denseLifeProfileFixture(),
			"personality_system": map[string]any{"mode": "single", "active_profile_id": "default", "profiles": []any{}},
		},
		Personality:        map[string]any{"openness": 0.7},
		BehavioralPolicy:   map[string]any{"response_style": "克制"},
		PersonalityRuntime: map[string]any{"active_profile_id": "default", "revision": 1},
	}
	working, _ := projectWorkingPersona(projection, "")
	shared := mapValue(working[workingPersonaSharedIdentityKey])
	life := mapValue(shared["life_profile"])
	if len(life) != 2 {
		t.Fatalf("the working persona life_profile must keep only preferences and character_constraints, got %d keys: %#v", len(life), life)
	}
	if _, ok := life["preferences"]; !ok {
		t.Fatalf("preferences must be kept: %#v", life)
	}
	if _, ok := life["character_constraints"]; !ok {
		t.Fatalf("character_constraints must be kept: %#v", life)
	}
	serialized := jsonString(working)
	for _, leaked := range []string{
		"栗色卷发",    // appearance
		"童年长期搬家",  // social_background
		"每周发一次画作", // recurring_commitments
		"反复检查门锁",  // life_habits
		"信任但保留边界", // relationship_seeds
		"水彩质感",    // media_preferences
	} {
		if strings.Contains(serialized, leaked) {
			t.Fatalf("the working persona leaked %q from a non-decision life_profile key: %s", leaked, serialized)
		}
	}
	if !strings.Contains(serialized, "安静的书房") {
		t.Fatalf("preferences were dropped from the working persona: %s", serialized)
	}
	if !strings.Contains(serialized, "不泄露真实住址") {
		t.Fatalf("character_constraints were dropped from the working persona: %s", serialized)
	}
}

type lifeProfileSizeReport struct {
	LifeProfileKeys  int `json:"life_profile_keys"`
	LifeProfileBytes int `json:"life_profile_bytes"`
	WorkingBytes     int `json:"working_persona_bytes"`
}

type lifeProfileBeforeAfterReport struct {
	Description string                `json:"description"`
	Before      lifeProfileSizeReport `json:"before"`
	After       lifeProfileSizeReport `json:"after"`
}

func lifeProfileSize(working map[string]any) lifeProfileSizeReport {
	shared := mapValue(working[workingPersonaSharedIdentityKey])
	life := mapValue(shared["life_profile"])
	lifeBytes := 0
	if len(life) > 0 {
		lifeBytes = len(jsonString(life))
	}
	return lifeProfileSizeReport{LifeProfileKeys: len(life), LifeProfileBytes: lifeBytes, WorkingBytes: len(jsonString(working))}
}

// TestWorkingPersonaLifeProfileBeforeAfterComparison records the F-06 Prompt
// convergence evidence on one real-shaped dense card: the same life_profile
// under the pre-F-06 full-injection projection (every key, metadata-only) and
// the post-F-06 decision-only allowlist. The report is written to testdata so
// the delivery evidence is machine-readable (request.md:321-327), not asserted
// in prose only.
func TestWorkingPersonaLifeProfileBeforeAfterComparison(t *testing.T) {
	lifeProfile := denseLifeProfileFixture()
	projection := ContextProjection{
		FluctlightID: "working-persona-life-profile-before-after",
		CorePersona: map[string]any{
			"identity":           map[string]any{"name": "苏晚"},
			"life_profile":       lifeProfile,
			"personality_system": map[string]any{"mode": "single", "active_profile_id": "default", "profiles": []any{}},
		},
		Personality:        map[string]any{"openness": 0.7},
		BehavioralPolicy:   map[string]any{"response_style": "克制"},
		PersonalityRuntime: map[string]any{"active_profile_id": "default", "revision": 1},
	}
	after, _ := projectWorkingPersona(projection, "")
	// The pre-F-06 baseline keeps the full life_profile (metadata-only
	// filter) so the comparison isolates the F-06 allowlist change; every
	// other Working Persona layer is identical to the post-F-06 projection.
	before := map[string]any{
		workingPersonaProfileIDKey: after[workingPersonaProfileIDKey],
		workingPersonaSharedIdentityKey: map[string]any{
			"identity":     mapValue(mapValue(projection.CorePersona)["identity"]),
			"life_profile": filterCorePersonaValue(lifeProfile),
		},
		workingPersonaBodyKey: mapValue(after[workingPersonaBodyKey]),
	}
	report := lifeProfileBeforeAfterReport{
		Description: "F-06 life_profile allowlist convergence on one dense card (preferences + character_constraints kept; appearance/social_background/life_habits/recurring_commitments/relationship_seeds/media_preferences deferred to task-scoped projections)",
		Before:      lifeProfileSize(before),
		After:       lifeProfileSize(after),
	}
	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", "life_profile_before_after_report.json")
	if err := os.WriteFile(path, []byte(jsonString(report)), 0o644); err != nil {
		t.Fatal(err)
	}
	if report.After.LifeProfileBytes >= report.Before.LifeProfileBytes {
		t.Fatalf("the F-06 allowlist did not converge the life_profile size (before=%d, after=%d): %s", report.Before.LifeProfileBytes, report.After.LifeProfileBytes, jsonString(report))
	}
	if report.After.LifeProfileKeys >= report.Before.LifeProfileKeys {
		t.Fatalf("the F-06 allowlist did not reduce the life_profile key count (before=%d, after=%d)", report.Before.LifeProfileKeys, report.After.LifeProfileKeys)
	}
}

func TestSystemPersonaPrunesSinglePersonalityAndDeduplicatesSwitchingRules(t *testing.T) {
	// Case 1: Single personality projection
	singleProjection := ContextProjection{
		FluctlightID: "single-persona-fluctlight",
		CorePersona: map[string]any{
			"identity": map[string]any{"name": "摇光单人格"},
			"personality_system": map[string]any{
				"mode":              "single",
				"active_profile_id": "default",
				"profiles": []any{
					map[string]any{
						"id":          "default",
						"name":        "默认人格",
						"personality": map[string]any{"traits": map[string]any{"description": "温和"}},
					},
				},
				"switching": map[string]any{
					"rules": []any{
						map[string]any{"id": "dummy_rule", "condition": "不应该存在"},
					},
				},
			},
		},
		PersonalityRuntime: map[string]any{"active_profile_id": "default"},
	}

	singleBundle := systemPersonaForLegacyProjectionForTest(singleProjection, workingPersonaMainTurnSchema)
	if _, exists := singleBundle["personality_system"]; exists {
		t.Fatalf("single personality leaked personality_system: %#v", singleBundle["personality_system"])
	}
	if _, exists := singleBundle[workingPersonaSwitchKey]; exists {
		t.Fatalf("single personality leaked persistent_switch: %#v", singleBundle[workingPersonaSwitchKey])
	}
	if working := mapValue(singleBundle[workingPersonaBodyKey]); len(working) == 0 {
		t.Fatalf("working persona body was missing in single personality bundle: %#v", singleBundle)
	}

	// Case 2: Multi-personality projection with switching rules
	multiProjection := ContextProjection{
		FluctlightID: "multi-persona-fluctlight",
		CorePersona: map[string]any{
			"identity": map[string]any{"name": "多重人格摇光"},
			"personality_system": map[string]any{
				"mode":              "multiple",
				"active_profile_id": "warm",
				"profiles": []any{
					map[string]any{
						"id":          "warm",
						"name":        "温柔",
						"personality": map[string]any{"traits": map[string]any{"description": "安静温和"}},
					},
					map[string]any{
						"id":          "guarded",
						"name":        "克制",
						"personality": map[string]any{"traits": map[string]any{"description": "冷淡防备"}},
					},
				},
				"switching": map[string]any{
					"rules": []any{
						map[string]any{"id": "stress_switch", "condition": "遭遇攻击", "target_profile_id": "guarded"},
					},
				},
				"conflict_resolution": map[string]any{"strategy": "dominant"},
			},
		},
		PersonalityRuntime: map[string]any{"active_profile_id": "warm", "revision": 1},
	}

	multiBundle := systemPersonaForLegacyProjectionForTest(multiProjection, workingPersonaMainTurnSchema)
	multiSystem := mapValue(multiBundle["personality_system"])
	if len(multiSystem) == 0 {
		t.Fatalf("multi-personality bundle missing personality_system: %#v", multiBundle)
	}
	// Verify switching was removed from personality_system to avoid duplication
	if _, exists := multiSystem["switching"]; exists {
		t.Fatalf("personality_system still duplicated switching rules: %#v", multiSystem["switching"])
	}
	// Verify persistent_switch contains the authorized switching rules
	switchSection := mapValue(multiBundle[workingPersonaSwitchKey])
	if len(switchSection) == 0 {
		t.Fatalf("persistent_switch section missing in multi-personality bundle: %#v", multiBundle)
	}
	rules := arrayValue(switchSection["rules"])
	if len(rules) != 1 || stringValue(mapValue(rules[0])["id"]) != "switch:stress_switch" {
		t.Fatalf("persistent_switch rules unexpected: %#v", switchSection)
	}
}

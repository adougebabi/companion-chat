package core

import (
	"context"
	"strings"
	"testing"
)

// This file pins F-05: the Takeover Judge control view of the handover
// candidate (B) must merge B's already-accepted Evolution Overlay, so the
// Judge sees the stance B would actually speak with instead of only B's
// declared baseline. Without the merge the Judge could approve a takeover
// based on a stance B no longer holds, or reject one based on a stance B has
// already grown into.
//
// The merge is read-only: the persistent active profile (A) and the current
// reply owner the Judge reasons about stay A, while only the
// handover_candidate stance is re-scoped to B's composed persona.

// TestTakeoverControlViewMergesBAcceptedOverlay seeds a real multi-profile
// fluctlight whose active profile is spark, gives the handover candidate
// twilight an accepted active Evolution Overlay on traits.openness, and
// asserts the Judge control view reports twilight's composed stance (overlay
// applied) while the current reply owner stays spark.
func TestTakeoverControlViewMergesBAcceptedOverlay(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	fluctlightID := "f05-takeover-overlay"
	ownerID := "f05-owner"
	conversationID := "f05-conv"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)

	// Give twilight (B) one accepted, active numeric overlay on
	// traits.openness. twilight's declared baseline openness is 0.3; the
	// overlay moves the composed value to 0.55. The Judge must see 0.55, not
	// 0.3, when it inspects the handover candidate.
	twilightRef := "personality:ctx_" + stableDigest(fluctlightID+"\x1f"+"twilight")
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_evolution_states(fluctlight_id,profile_id,profile_ref,revision,domain_revisions) VALUES($1,'twilight',$2,1,'{"personality":1}')`, fluctlightID, twilightRef); err != nil {
		t.Fatal(err)
	}
	overlayRef := "evolution_overlay:ctx_" + stableDigest(fluctlightID + "\x1f" + "twilight-openness")[:32]
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_evolution_overlays(id,ref,fluctlight_id,profile_id,kind,field_path,value_kind,semantic_direction,requested_delta,applied_delta,before_value,after_value,confidence,evidence_refs,evidence_windows,policy_version,base_revision,revision,status,supersedes,rollback_of,cooldown_until,created_at) VALUES('f05-twilight-openness',$2,$1,'twilight','personality','traits.openness','numeric','increase',0.25,0.25,$3,$4,0.9,'[]','[]','reflection.policy.v2',0,1,'active',NULL,NULL,now(),now())`, fluctlightID, overlayRef, 0.3, 0.55); err != nil {
		t.Fatal(err)
	}

	// The projection the Judge path receives: spark is the persistent active
	// profile, twilight is the declared handover candidate. The projection
	// carries no EffectivePersona for twilight yet — the control view
	// builder must compose it on demand.
	personalitySystem := map[string]any{
		"mode": "multiple", "active_profile_id": "spark",
		"profiles": []any{
			map[string]any{
				"id": "spark", "name": "星火",
				"personality":       map[string]any{"openness": 0.8, "signature_phrase": takeoverChainSparkMarker},
				"behavioral_policy": map[string]any{"directness": 0.9},
			},
			map[string]any{
				"id": "twilight", "name": "暮光",
				"personality":       map[string]any{"openness": 0.3, "signature_phrase": takeoverChainTwilightMarker},
				"behavioral_policy": map[string]any{"gentleness": 0.9},
			},
		},
		"switching": map[string]any{"rules": []any{
			map[string]any{"id": "safety", "condition": "收到明确的安全确认后由暮光主导", "target_profile_id": "twilight"},
		}},
	}
	projection := ContextProjection{
		FluctlightID: fluctlightID,
		CorePersona: map[string]any{
			"identity":           map[string]any{"name": "摇光"},
			"personality_system": personalitySystem,
		},
		PersonalityRuntime: map[string]any{"active_profile_id": "spark"},
	}

	app := &App{DB: repository}
	rule := personaSwitchRule{
		RuleID: "public-doubt", Kind: switchRuleTurnTakeover, DeclarationVersion: personaTakeoverRuleVersion,
		Condition:       "用户在强烈质疑本轮回复时由暮光接管",
		SourceProfileID: "spark", TargetProfileID: "twilight",
	}
	view, err := app.projectTakeoverControlView(ctx, rule, projection)
	if err != nil {
		t.Fatal(err)
	}

	// The control view keeps reporting the persistent active profile as the
	// current reply owner — the rescope is read-only and must not promote B
	// to the reply owner the Judge reasons about.
	if got := stringValue(view["current_reply_owner_profile_id"]); got != "spark" {
		t.Fatalf("the control view rescope promoted the candidate to reply owner: %q", got)
	}
	if got := stringValue(view["target_profile_id"]); got != "twilight" {
		t.Fatalf("the control view lost the target profile id: %q", got)
	}
	candidate := mapValue(view["handover_candidate"])
	if got := stringValue(candidate["profile_id"]); got != "twilight" {
		t.Fatalf("the handover candidate is not twilight: %#v", candidate)
	}
	stance := mapValue(candidate["stance"])
	personality := mapValue(stance["personality"])
	traits := mapValue(personality["traits"])
	if got := numberOrZero(traits["openness"]); got != 0.55 {
		t.Fatalf("the candidate's accepted overlay did not merge into the Judge stance (want 0.55, got %v): %#v", got, stance)
	}
	// The merged overlay value must be observable in the serialized control
	// view the Judge actually receives, not only in the parsed map.
	if !strings.Contains(jsonString(view), "\"openness\":0.55") {
		t.Fatalf("the merged overlay value is missing from the serialized control view: %s", jsonString(view))
	}
}

// TestTakeoverControlViewFallsBackToBaselineWithoutOverlay pins the fallback:
// when the handover candidate has no accepted overlay, the control view
// reports the candidate's declared baseline stance. This is both the
// no-overlay behaviour and the guarantee that the F-05 merge does not invent
// persona data when there is nothing to merge.
func TestTakeoverControlViewFallsBackToBaselineWithoutOverlay(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	fluctlightID := "f05-takeover-baseline"
	ownerID := "f05-baseline-owner"
	conversationID := "f05-baseline-conv"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)

	personalitySystem := map[string]any{
		"mode": "multiple", "active_profile_id": "spark",
		"profiles": []any{
			map[string]any{"id": "spark", "name": "星火", "personality": map[string]any{"openness": 0.8}, "behavioral_policy": map[string]any{"directness": 0.9}},
			map[string]any{"id": "twilight", "name": "暮光", "personality": map[string]any{"openness": 0.3}, "behavioral_policy": map[string]any{"gentleness": 0.9}},
		},
	}
	projection := ContextProjection{
		FluctlightID:       fluctlightID,
		CorePersona:        map[string]any{"identity": map[string]any{"name": "摇光"}, "personality_system": personalitySystem},
		PersonalityRuntime: map[string]any{"active_profile_id": "spark"},
	}
	app := &App{DB: repository}
	rule := personaSwitchRule{RuleID: "public-doubt", Kind: switchRuleTurnTakeover, DeclarationVersion: personaTakeoverRuleVersion, TargetProfileID: "twilight", SourceProfileID: "spark"}
	view, err := app.projectTakeoverControlView(ctx, rule, projection)
	if err != nil {
		t.Fatal(err)
	}

	candidate := mapValue(view["handover_candidate"])
	traits := mapValue(mapValue(mapValue(candidate["stance"])["personality"])["traits"])
	if got := numberOrZero(traits["openness"]); got != 0.3 {
		t.Fatalf("a candidate without an overlay must report the declared baseline (want 0.3, got %v): %#v", got, view)
	}
}

// TestTakeoverControlViewFailureDoesNotFallBackToBaseline verifies the
// security boundary behind F-05: an overlay read failure is distinct from an
// empty overlay state. Returning the declared baseline in this case would let
// the Judge reason about a stale B stance.
func TestTakeoverControlViewFailureDoesNotFallBackToBaseline(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	fluctlightID := "f05-overlay-load-failure"
	ownerID := "f05-overlay-load-owner"
	conversationID := "f05-overlay-load-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)

	projection := ContextProjection{
		FluctlightID:       fluctlightID,
		CorePersona:        map[string]any{"identity": map[string]any{"name": "摇光"}, "personality_system": map[string]any{"profiles": []any{map[string]any{"id": "spark", "name": "星火"}, map[string]any{"id": "twilight", "name": "暮光"}}}},
		PersonalityRuntime: map[string]any{"active_profile_id": "spark"},
	}
	rule := personaSwitchRule{RuleID: "public-doubt", Kind: switchRuleTurnTakeover, DeclarationVersion: personaTakeoverRuleVersion, TargetProfileID: "twilight", SourceProfileID: "spark"}

	cancelled := context.Background()
	ctx, cancel := context.WithCancel(cancelled)
	cancel()
	view, err := (&App{DB: repository}).projectTakeoverControlView(ctx, rule, projection)
	if err == nil {
		t.Fatalf("an overlay read failure must be returned, got view %#v", view)
	}
	if view != nil {
		t.Fatalf("a failed overlay read must not return a baseline control view: %#v", view)
	}
	if got := takeoverScopeErrorCode(err); got != takeoverOverlayLoadFailed {
		t.Fatalf("overlay read failure code=%q, want %q", got, takeoverOverlayLoadFailed)
	}
}

// TestJudgeSkipsProviderWhenTakeoverControlViewFails verifies that a failed
// control-view preflight enters the bounded Judge degradation path without
// making a request. The A candidate remains eligible for the normal keep-A
// settlement path, and the diagnostic identifies the failed overlay read.
func TestJudgeSkipsProviderWhenTakeoverControlViewFails(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	fluctlightID := "f05-overlay-judge-failure"
	ownerID := "f05-overlay-judge-owner"
	conversationID := "f05-overlay-judge-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)

	router := newFakeProviderRouter().on(takeoverJudgeSchemaName, func(map[string]any) fakeProviderResult {
		t.Fatalf("the Judge must not be called after control-view assembly fails")
		return fakeProviderResult{}
	})
	app := newTestApp(t, repository, router)
	projection := ContextProjection{
		FluctlightID:       fluctlightID,
		CurrentUserText:    "用户在质疑",
		CorePersona:        map[string]any{"identity": map[string]any{"name": "摇光"}, "personality_system": map[string]any{"profiles": []any{map[string]any{"id": "spark", "name": "星火"}, map[string]any{"id": "twilight", "name": "暮光"}}}},
		PersonalityRuntime: map[string]any{"active_profile_id": "spark"},
	}
	rule := personaSwitchRule{RuleID: "public-doubt", Kind: switchRuleTurnTakeover, DeclarationVersion: personaTakeoverRuleVersion, TargetProfileID: "twilight", SourceProfileID: "spark"}
	input := turnTakeoverInput{
		Projection:   projection,
		Scope:        turnPersonaScope{ActiveProfileID: "spark", ReplyOwnerProfileID: "spark"},
		ResponseMode: "final",
		Action:       "reply",
		Decision:     map[string]any{"visible_text": "保留 A", "visible_text_digest": stableDigest("保留 A")},
		Frozen:       frozenTurn{ID: "f05-overlay-judge-frozen"},
	}

	judgeCtx, cancel := context.WithCancel(context.Background())
	cancel()
	record, outcome, takeover, err := app.judgeTurnTakeover(judgeCtx, input, rule)
	if err != nil {
		t.Fatal(err)
	}
	if outcome != takeoverJudgeOutcomeUnavailable || takeover {
		t.Fatalf("control-view failure must degrade to keep A, outcome=%q takeover=%v record=%#v", outcome, takeover, record)
	}
	if got := stringValue(record["error_code"]); got != takeoverOverlayLoadFailed {
		t.Fatalf("control-view diagnostic=%q, want %q: %#v", got, takeoverOverlayLoadFailed, record)
	}
	if got := router.totalRequests(); got != 0 {
		t.Fatalf("control-view failure must make zero Provider requests, got %d", got)
	}
}

// TestTakeoverControlViewCompositionFailureIsNotAnEmptyOverlay verifies that a
// malformed persisted evolution state is surfaced as a composition failure.
// The state row exists, so treating this case as "no overlay" would hide data
// corruption and let a stale baseline reach the Judge.
func TestTakeoverControlViewCompositionFailureIsNotAnEmptyOverlay(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	fluctlightID := "f05-overlay-compose-failure"
	ownerID := "f05-overlay-compose-owner"
	conversationID := "f05-overlay-compose-conversation"
	takeoverChainSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_evolution_states(fluctlight_id,profile_id,profile_ref,revision,domain_revisions) VALUES($1,'twilight','corrupt-profile-ref',0,'{}')`, fluctlightID); err != nil {
		t.Fatal(err)
	}

	projection := ContextProjection{
		FluctlightID:       fluctlightID,
		CorePersona:        map[string]any{"identity": map[string]any{"name": "摇光"}, "personality_system": map[string]any{"profiles": []any{map[string]any{"id": "spark", "name": "星火"}, map[string]any{"id": "twilight", "name": "暮光"}}}},
		PersonalityRuntime: map[string]any{"active_profile_id": "spark"},
	}
	rule := personaSwitchRule{RuleID: "public-doubt", Kind: switchRuleTurnTakeover, DeclarationVersion: personaTakeoverRuleVersion, TargetProfileID: "twilight", SourceProfileID: "spark"}
	view, err := (&App{DB: repository}).projectTakeoverControlView(ctx, rule, projection)
	if err == nil {
		t.Fatalf("a malformed persisted state must return an error, got view %#v", view)
	}
	if view != nil {
		t.Fatalf("a composition failure must not return a baseline control view: %#v", view)
	}
	if got := takeoverScopeErrorCode(err); got != takeoverOverlayComposeFailed {
		t.Fatalf("overlay composition failure code=%q, want %q", got, takeoverOverlayComposeFailed)
	}
}

// TestTakeoverControlViewCloneFailureIsNotAnEmptyOverlay verifies that a
// projection codec failure is also fail-closed. This guards the other failure
// branch that previously silently returned a baseline view.
func TestTakeoverControlViewCloneFailureIsNotAnEmptyOverlay(t *testing.T) {
	projection := ContextProjection{
		FluctlightID:       "f05-overlay-clone-failure",
		CorePersona:        map[string]any{"identity": map[string]any{"unsupported": func() {}}},
		PersonalityRuntime: map[string]any{"active_profile_id": "spark"},
	}
	rule := personaSwitchRule{RuleID: "public-doubt", Kind: switchRuleTurnTakeover, DeclarationVersion: personaTakeoverRuleVersion, TargetProfileID: "twilight", SourceProfileID: "spark"}
	view, err := (&App{}).projectTakeoverControlView(context.Background(), rule, projection)
	if err == nil {
		t.Fatalf("a projection clone failure must return an error, got view %#v", view)
	}
	if view != nil {
		t.Fatalf("a clone failure must not return a baseline control view: %#v", view)
	}
	if got := takeoverScopeErrorCode(err); got != takeoverControlViewProjectionInvalid {
		t.Fatalf("projection clone failure code=%q, want %q", got, takeoverControlViewProjectionInvalid)
	}
}

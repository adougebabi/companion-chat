package core

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"
)

func TestEvolutionOverlayStageS09(t *testing.T) {
	now := time.Date(2026, 9, 11, 16, 0, 0, 0, time.UTC)
	state := PersonaEvolutionState{
		FluctlightID: "fl-s09", ProfileID: "default", ProfileRef: "personality:ctx_" + strings.Repeat("a", 32), Revision: 0,
		Personality:    map[string]any{"traits": map[string]any{"openness": 0.5}, "expression": map[string]any{"warmth": 0.6}},
		BehaviorPolicy: map[string]any{"communication": map[string]any{"tone": "克制"}, "response": map[string]any{"style": "简洁"}},
	}
	policy := EvolutionOverlayPolicy{NumericMinConfidence: 0.8, CategoricalMinConfidence: 0.9, NumericMinWindows: 2, CategoricalMinWindows: 3, MaxNumericDelta: 0.1, Cooldown: 24 * time.Hour}

	t.Run("forbidden and non-allowlisted paths are rejected", func(t *testing.T) {
		for _, path := range []string{"identity.name", "owner.permissions", "provider.endpoint", "character_constraints.safety", "traits.unknown"} {
			decision, err := CompileEvolutionOverlay(state, EvolutionOverlayRequest{ExpectedRevision: 0, Candidate: ReflectionOverlayCandidateV2{FieldPath: path, Direction: "increase", Strength: 1, Confidence: 1, EvidenceRefs: []string{"fact:1"}}, EvidenceWindows: []string{"w1", "w2"}, OccurredAt: now}, policy)
			if err != nil || decision.Disposition != EvolutionRejected {
				t.Fatalf("path %q decision=%#v err=%v", path, decision, err)
			}
		}
	})

	t.Run("numeric overlay requires windows confidence max-delta and cooldown", func(t *testing.T) {
		candidate := ReflectionOverlayCandidateV2{FieldPath: "traits.openness", Direction: "increase", Strength: 1, Confidence: 0.9, EvidenceRefs: []string{"fact:1", "fact:2"}}
		deferred, err := CompileEvolutionOverlay(state, EvolutionOverlayRequest{ExpectedRevision: 0, Candidate: candidate, EvidenceWindows: []string{"w1"}, OccurredAt: now}, policy)
		if err != nil || deferred.Disposition != EvolutionDeferred || deferred.ReasonCode != "cross_window_evidence_required" {
			t.Fatalf("single-window decision=%#v err=%v", deferred, err)
		}
		accepted, err := CompileEvolutionOverlay(state, EvolutionOverlayRequest{ExpectedRevision: 0, Candidate: candidate, EvidenceWindows: []string{"w1", "w2"}, OccurredAt: now}, policy)
		if err != nil || accepted.Disposition != EvolutionAccepted || accepted.Overlay == nil {
			t.Fatalf("accepted decision=%#v err=%v", accepted, err)
		}
		after, afterOK := numberFloat(accepted.Overlay.AfterValue)
		if math.Abs(accepted.Overlay.AppliedDelta-0.1) > 1e-9 || !afterOK || math.Abs(after-0.6) > 1e-9 {
			t.Fatalf("accepted decision=%#v err=%v", accepted, err)
		}
		next, err := ApplyEvolutionOverlay(state, accepted)
		if err != nil || next.Revision != 1 || len(next.Overlays) != 1 || state.Revision != 0 {
			t.Fatalf("next=%#v baseline=%#v err=%v", next, state, err)
		}
		cooldown, err := CompileEvolutionOverlay(next, EvolutionOverlayRequest{ExpectedRevision: 1, Candidate: candidate, EvidenceWindows: []string{"w3", "w4"}, OccurredAt: now.Add(time.Hour)}, policy)
		if err != nil || cooldown.Disposition != EvolutionDeferred || cooldown.ReasonCode != "cooldown_active" {
			t.Fatalf("cooldown=%#v err=%v", cooldown, err)
		}
	})

	t.Run("categorical overlay needs stronger multi-window agreement", func(t *testing.T) {
		candidate := ReflectionOverlayCandidateV2{FieldPath: "communication.tone", Direction: "toward", SemanticValue: "温和直接", Strength: 0.5, Confidence: 0.95, EvidenceRefs: []string{"fact:1", "fact:2", "fact:3"}}
		deferred, err := CompileEvolutionOverlay(state, EvolutionOverlayRequest{ExpectedRevision: 0, Candidate: candidate, EvidenceWindows: []string{"w1", "w2"}, OccurredAt: now}, policy)
		if err != nil || deferred.Disposition != EvolutionDeferred {
			t.Fatalf("categorical insufficient windows=%#v err=%v", deferred, err)
		}
		accepted, err := CompileEvolutionOverlay(state, EvolutionOverlayRequest{ExpectedRevision: 0, Candidate: candidate, EvidenceWindows: []string{"w1", "w2", "w3"}, OccurredAt: now}, policy)
		if err != nil || accepted.Disposition != EvolutionAccepted || accepted.Overlay.AfterValue != "温和直接" {
			t.Fatalf("categorical accepted=%#v err=%v", accepted, err)
		}
	})

	t.Run("effective view is profile-scoped provider-safe and frozen", func(t *testing.T) {
		candidate := ReflectionOverlayCandidateV2{FieldPath: "traits.openness", Direction: "increase", Strength: 1, Confidence: 0.9, EvidenceRefs: []string{"fact:1", "fact:2"}}
		accepted, _ := CompileEvolutionOverlay(state, EvolutionOverlayRequest{ExpectedRevision: 0, Candidate: candidate, EvidenceWindows: []string{"w1", "w2"}, OccurredAt: now}, policy)
		next, _ := ApplyEvolutionOverlay(state, accepted)
		effective, err := ComposeEffectivePersona(next)
		if err != nil || numberOrZero(mapValue(effective.Personality["traits"])["openness"]) != 0.6 || effective.AuthorityRevision != 1 {
			t.Fatalf("effective=%#v err=%v", effective, err)
		}
		providerJSON := jsonString(effective)
		for _, forbidden := range []string{"evidence_refs", "requested_delta", "applied_delta", "overlays", "memory"} {
			if strings.Contains(providerJSON, forbidden) {
				t.Fatalf("Provider effective view leaked %q: %s", forbidden, providerJSON)
			}
		}
		frozen, err := FreezeEffectivePersona(effective)
		if err != nil || frozen.Digest == "" {
			t.Fatalf("frozen=%#v err=%v", frozen, err)
		}
		mapValue(effective.Personality["traits"])["openness"] = 0.1
		if numberOrZero(mapValue(frozen.Snapshot.Personality["traits"])["openness"]) != 0.6 {
			t.Fatalf("live mutation changed frozen realization snapshot: %#v", frozen)
		}
		otherProfile := next
		otherProfile.ProfileID = "other"
		otherProfile.ProfileRef = "personality:ctx_" + strings.Repeat("b", 32)
		otherEffective, err := ComposeEffectivePersona(otherProfile)
		if err != nil || numberOrZero(mapValue(otherEffective.Personality["traits"])["openness"]) != 0.5 {
			t.Fatalf("overlay leaked across profile: %#v err=%v", otherEffective, err)
		}
	})

	t.Run("rollback appends a compensating revision without changing baseline", func(t *testing.T) {
		candidate := ReflectionOverlayCandidateV2{FieldPath: "traits.openness", Direction: "increase", Strength: 1, Confidence: 0.9, EvidenceRefs: []string{"fact:1", "fact:2"}}
		accepted, _ := CompileEvolutionOverlay(state, EvolutionOverlayRequest{ExpectedRevision: 0, Candidate: candidate, EvidenceWindows: []string{"w1", "w2"}, OccurredAt: now}, policy)
		next, _ := ApplyEvolutionOverlay(state, accepted)
		rolledBack, rollback, err := RollbackEvolutionOverlay(next, accepted.Overlay.ID, 1, []string{"owner:rollback"}, now.Add(25*time.Hour), time.Hour)
		if err != nil || rolledBack.Revision != 2 || len(rolledBack.Overlays) != 2 || rollback.RollbackOf != accepted.Overlay.ID || rollback.Revision != 2 {
			t.Fatalf("rollback state=%#v revision=%#v err=%v", rolledBack, rollback, err)
		}
		effective, err := ComposeEffectivePersona(rolledBack)
		if err != nil || numberOrZero(mapValue(effective.Personality["traits"])["openness"]) != 0.5 || numberOrZero(mapValue(state.Personality["traits"])["openness"]) != 0.5 {
			t.Fatalf("rollback effective=%#v original=%#v err=%v", effective, state, err)
		}
		if rolledBack.Overlays[0].Status != OverlaySuperseded || rolledBack.Overlays[1].Status != OverlayActive {
			t.Fatalf("rollback deleted or failed to supersede history: %#v", rolledBack.Overlays)
		}
	})
}

func TestReflectionOverlayEvidenceCountsHistoricalWindowsNotFacts(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	fluctlightID := "overlay-window-fluctlight"
	candidate := ReflectionOverlayCandidateV2{FieldPath: "traits.openness", Direction: "increase", Strength: 0.4, Confidence: 0.9, EvidenceRefs: []string{"sequence:5", "sequence:6"}}
	proposal := ReflectionProposalV2{SchemaVersion: reflectionProposalV2SchemaVersion, Summary: "历史窗口", PersonalityEvolutionCandidates: []ReflectionOverlayCandidateV2{candidate}}
	insert := func(id string, from, to int) {
		digest := stableDigest(id)
		_, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_reflection_proposals(id,fluctlight_id,from_sequence,to_sequence,base_state_revision,payload,evidence_refs,correlation_id,status,schema_version,context_snapshot,expected_revisions,result,model_version,prompt_version,policy_version,request_digest,idempotency_key) VALUES($1,$2,$3,$4,0,$5,'[]',$1,'no_change',$6,'{}','{}','{}','model','prompt',$7,$8,$1)`, id, fluctlightID, from, to, jsonBytes(proposal), reflectionProposalV2SchemaVersion, reflectionPolicyVersion, digest)
		if err != nil {
			t.Fatal(err)
		}
	}
	app := &App{DB: repository}
	windows, err := app.reflectionOverlayEvidenceWindows(context.Background(), fluctlightID, "sequence:5-6", EvolutionPersonality, candidate)
	if err != nil || len(windows) != 1 {
		t.Fatalf("current window facts counted as windows: %#v err=%v", windows, err)
	}
	insert("overlay-window-prior-1", 1, 2)
	windows, err = app.reflectionOverlayEvidenceWindows(ctx, fluctlightID, "sequence:5-6", EvolutionPersonality, candidate)
	if err != nil || len(windows) != 2 {
		t.Fatalf("one historical window count=%#v err=%v", windows, err)
	}
	insert("overlay-window-prior-2", 3, 4)
	windows, err = app.reflectionOverlayEvidenceWindows(ctx, fluctlightID, "sequence:5-6", EvolutionPersonality, candidate)
	if err != nil || len(windows) != 3 {
		t.Fatalf("two historical windows count=%#v err=%v", windows, err)
	}
}

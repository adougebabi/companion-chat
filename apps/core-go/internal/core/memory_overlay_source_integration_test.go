package core

import (
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestMemoryCorrectionInvalidatesPersonaOverlayAndHistoricalWindows(t *testing.T) {
	testMemoryOverlaySourcePropagation(t, false)
}

func TestDeferredMemoryCandidateCannotRetireAcceptedPersonaOverlay(t *testing.T) {
	testMemoryOverlaySourcePropagation(t, true)
}

func testMemoryOverlaySourcePropagation(t *testing.T, unrelatedCandidate bool) {
	f := seedWardrobeToolFixture(t)
	index := newContextReferenceIndex(ContextProjection{FluctlightID: f.fluctlightID, OwnerActorID: f.ownerID, ConversationID: f.conversationID, CurrentSpeaker: map[string]any{"actor_id": f.ownerID}, PersonalityRuntime: map[string]any{"active_profile_id": "default"}})
	var sources []MemoryApplyResult
	var refs []string
	keys := []string{"openness-one", "openness-two"}
	if unrelatedCandidate {
		keys = append(keys, "openness-distractor")
	}
	for _, key := range keys {
		command := PreparedMemoryMutation{SchemaVersion: memoryLifecycleSchemaVersion, Operation: MemoryCreate, OwnerFluctlightID: f.fluctlightID, OwnerActorID: f.ownerID, ActorID: f.ownerID, ActorRefs: []string{f.ownerID}, EventRefs: []string{}, EvidenceRefs: []string{"owner:" + key}, Visibility: "private", OccurredAt: time.Now().UTC(), SourceFactID: "owner:" + key, SemanticReason: "Owner confirmed profile evidence", IdempotencyKey: key, Semantic: &MemorySemanticInput{Type: "autobiographical", Content: "角色曾表示愿意尝试新事物 " + key, Confidence: 0.9, Importance: 0.9}}
		command.RequestDigest = memoryCommandDigest(command)
		created := applyMemoryLifecycleTestCommand(t, f.ctx, f.app, command)
		sources = append(sources, created)
		var raw []byte
		if err := f.repository.Pool().QueryRow(f.ctx, `SELECT to_jsonb(m) FROM public.memories m WHERE id=$1`, created.MemoryID).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		ref, err := index.add(ContextReferenceMemory, created.MemoryID, created.Revision, decodeObject(raw))
		if err != nil {
			t.Fatal(err)
		}
		refs = append(refs, ref)
	}
	baseline := PersonaEvolutionState{FluctlightID: f.fluctlightID, ProfileID: "default", ProfileRef: "personality:ctx_" + stableDigest(f.fluctlightID), Personality: map[string]any{"traits": map[string]any{"openness": 0.5}, "expression": map[string]any{"warmth": 0.4}}, BehaviorPolicy: map[string]any{"communication": map[string]any{"tone": "克制"}}}
	candidate := ReflectionOverlayCandidateV2{FieldPath: "traits.openness", Direction: "increase", Strength: 1, Confidence: 0.9, EvidenceRefs: refs[:2], SemanticReason: "由已确认的历史资料提议有限变化"}
	coordinator := EvolutionState{FluctlightID: f.fluctlightID, Watermark: 0, Revisions: map[string]int{string(EvolutionPersonality): 0, string(EvolutionAffectProfile): 0}}
	for step := 0; step < 2; step++ {
		from, to := step*2+1, step*2+2
		window := fmt.Sprintf("sequence:%d-%d", from, to)
		evolution, err := BuildEvolutionContext(EvolutionContext{ProviderRole: "reflection", FluctlightID: f.fluctlightID, SourceWindow: window, FromSequence: from, ToSequence: to, Watermark: from - 1, Evidence: []EvolutionEvidence{{Ref: fmt.Sprintf("sequence:%d", from), Kind: "conversation", Summary: "受控反思窗口", Sequence: from, OccurredAt: f.app.now()}}, ReferenceIndex: index, BaseRevisions: cloneRevisionMap(coordinator.Revisions)})
		if err != nil {
			t.Fatal(err)
		}
		proposal := ReflectionProposalV2{SchemaVersion: reflectionProposalV2SchemaVersion, Summary: "有限人格演化候选", PersonalityEvolutionCandidates: []ReflectionOverlayCandidateV2{candidate}}
		if unrelatedCandidate {
			proposal.PersonalityEvolutionCandidates = append(proposal.PersonalityEvolutionCandidates, ReflectionOverlayCandidateV2{FieldPath: candidate.FieldPath, Direction: candidate.Direction, Strength: 1, Confidence: 0.4, EvidenceRefs: []string{refs[2], refs[1]}, SemanticReason: "低置信度的独立候选"})
		}
		plan, err := CompileReflectionPlan(proposal, evolution, ReflectionPolicyV2{}, f.app.now())
		if err != nil || len(plan.Candidates) != len(proposal.PersonalityEvolutionCandidates) || plan.Candidates[0].Disposition != EvolutionAccepted {
			t.Fatalf("normal reflection compile: %#v err=%v", plan, err)
		}
		if unrelatedCandidate && plan.Candidates[1].Disposition != EvolutionDeferred {
			t.Fatalf("low-confidence candidate was accepted: %#v", plan.Candidates[1])
		}
		windows, err := f.app.reflectionOverlayEvidenceWindows(f.ctx, f.fluctlightID, window, EvolutionPersonality, candidate)
		if err != nil {
			t.Fatal(err)
		}
		decision, err := CompileEvolutionOverlay(baseline, EvolutionOverlayRequest{ExpectedRevision: baseline.Revision, Candidate: candidate, EvidenceWindows: windows, OccurredAt: f.app.now()}, EvolutionOverlayPolicy{})
		if err != nil {
			t.Fatal(err)
		}
		if step == 0 {
			if decision.Disposition != EvolutionDeferred || decision.ReasonCode != "cross_window_evidence_required" {
				t.Fatalf("single window bypassed policy: %#v", decision)
			}
			plan.Candidates[0].Disposition, plan.Candidates[0].ReasonCode = decision.Disposition, decision.ReasonCode
		} else if decision.Disposition != EvolutionAccepted || decision.Overlay == nil {
			t.Fatalf("two sourced windows were not accepted: %#v", decision)
		}
		if decision.Overlay != nil {
			hydrateReflectionChangedRefs(&plan, proposal, evolution, map[string]EvolutionOverlayDecision{plan.Candidates[0].CandidateID: decision})
		}
		nextCoordinator, result, err := ApplyReflectionPlan(coordinator, plan, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
			if decision.Overlay != nil {
				next, err := ApplyEvolutionOverlay(baseline, decision)
				if err != nil {
					return err
				}
				if _, err := persistEvolutionOverlayTx(f.ctx, tx, baseline, next, *decision.Overlay); err != nil {
					return err
				}
			}
			_, err := persistReflectionApplyTx(f.ctx, tx, evolution, proposal, plan, nextCoordinator, result, baseline.ProfileRef, "fixture-model", "reflection.v2")
			return err
		}); err != nil {
			t.Fatal(err)
		}
		coordinator = nextCoordinator
	}
	beforeState, err := loadPersonaEvolutionState(f.ctx, f.repository.Pool(), baseline)
	if err != nil {
		t.Fatal(err)
	}
	before, err := ComposeEffectivePersona(beforeState)
	if err != nil || numberOrZero(valueAtEvolutionPath(before.Personality, "traits.openness")) <= 0.5 || portraitOverlayRevision(beforeState) == 0 {
		t.Fatalf("valid overlay did not affect persona: %#v err=%v", before, err)
	}
	if unrelatedCandidate {
		revision := sources[2].Revision
		if _, err := f.app.ReviseMemory(f.ctx, f.ownerID, sources[2].MemoryID, "更正未采用候选的独立来源", &revision, []any{"owner:unrelated-correction"}); err != nil {
			t.Fatal(err)
		}
		state, err := loadPersonaEvolutionState(f.ctx, f.repository.Pool(), baseline)
		if err != nil {
			t.Fatal(err)
		}
		effective, err := ComposeEffectivePersona(state)
		if err != nil || numberOrZero(valueAtEvolutionPath(effective.Personality, "traits.openness")) <= 0.5 || portraitOverlayRevision(state) != 1 {
			t.Fatalf("deferred candidate's Memory retired the accepted candidate: %#v err=%v", effective, err)
		}
		windows, err := f.app.reflectionOverlayEvidenceWindows(f.ctx, f.fluctlightID, "sequence:5-6", EvolutionPersonality, candidate)
		if err != nil || len(windows) != 3 {
			t.Fatalf("unselected candidate poisoned actual historical evidence: %#v err=%v", windows, err)
		}
		return
	}
	// A later opposite-direction overlay stores an absolute after-value based
	// on the original field. Another independent field retains its own basis.
	f.app.Clock = fixedClock(f.app.now().Add(48 * time.Hour))
	for _, later := range []ReflectionOverlayCandidateV2{
		{FieldPath: "traits.openness", Direction: "decrease", Strength: 0.01, Confidence: 0.9, EvidenceRefs: []string{"owner:later-observation"}},
		{FieldPath: "expression.warmth", Direction: "increase", Strength: 0.01, Confidence: 0.9, EvidenceRefs: []string{"owner:independent-observation"}},
	} {
		decision, err := CompileEvolutionOverlay(beforeState, EvolutionOverlayRequest{ExpectedRevision: beforeState.Revision, Candidate: later, EvidenceWindows: []string{"later-window-one", "later-window-two"}, OccurredAt: f.app.now()}, EvolutionOverlayPolicy{})
		if err != nil || decision.Disposition != EvolutionAccepted {
			t.Fatalf("later valid overlay was not accepted: %#v err=%v", decision, err)
		}
		next, err := ApplyEvolutionOverlay(beforeState, decision)
		if err != nil {
			t.Fatal(err)
		}
		if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
			_, err := persistEvolutionOverlayTx(f.ctx, tx, beforeState, next, *decision.Overlay)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		beforeState = next
	}
	before, err = ComposeEffectivePersona(beforeState)
	if err != nil {
		t.Fatal(err)
	}
	beforeHash, err := FreezeEffectivePersona(before)
	if err != nil {
		t.Fatal(err)
	}
	revision := sources[0].Revision
	if _, err := f.app.ReviseMemory(f.ctx, f.ownerID, sources[0].MemoryID, "更正：这条尝新倾向判断没有依据", &revision, []any{"owner:openness-correction"}); err != nil {
		t.Fatal(err)
	}
	afterState, err := loadPersonaEvolutionState(f.ctx, f.repository.Pool(), baseline)
	if err != nil {
		t.Fatal(err)
	}
	after, err := ComposeEffectivePersona(afterState)
	if err != nil || numberOrZero(valueAtEvolutionPath(after.Personality, "traits.openness")) != 0.5 || numberOrZero(valueAtEvolutionPath(after.Personality, "expression.warmth")) <= 0.4 || portraitOverlayRevision(afterState) != 3 {
		t.Errorf("Memory correction left its persona overlay effective: %#v revision=%d err=%v", after, portraitOverlayRevision(afterState), err)
	}
	afterHash, err := FreezeEffectivePersona(after)
	if err != nil || afterHash.Digest == beforeHash.Digest {
		t.Fatalf("effective persona digest retained stale source values: %#v err=%v", afterHash, err)
	}
	windows, err := f.app.reflectionOverlayEvidenceWindows(f.ctx, f.fluctlightID, "sequence:5-6", EvolutionPersonality, candidate)
	if err != nil || len(windows) != 1 || windows[0] != "sequence:5-6" {
		t.Errorf("stale Memory reused through historical windows: %#v err=%v", windows, err)
	}
	fresh := ReflectionOverlayCandidateV2{FieldPath: "traits.openness", Direction: "increase", Strength: 0.01, Confidence: 0.9, EvidenceRefs: []string{"owner:fresh-basis"}}
	decision, err := CompileEvolutionOverlay(afterState, EvolutionOverlayRequest{ExpectedRevision: afterState.Revision, Candidate: fresh, EvidenceWindows: []string{"fresh-window-one", "fresh-window-two"}, OccurredAt: f.app.now().Add(48 * time.Hour)}, EvolutionOverlayPolicy{})
	if err != nil || decision.Disposition != EvolutionAccepted {
		t.Fatalf("fresh source basis was blocked: %#v err=%v", decision, err)
	}
	next, err := ApplyEvolutionOverlay(afterState, decision)
	if err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		_, err := persistEvolutionOverlayTx(f.ctx, tx, afterState, next, *decision.Overlay)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	recovered, err := loadPersonaEvolutionState(f.ctx, f.repository.Pool(), baseline)
	if err != nil {
		t.Fatal(err)
	}
	effective, err := ComposeEffectivePersona(recovered)
	if err != nil || numberOrZero(valueAtEvolutionPath(effective.Personality, "traits.openness")) <= 0.5 || portraitOverlayRevision(recovered) != 4 {
		t.Fatalf("fresh valid evolution could not resume: %#v err=%v", effective, err)
	}
}

package core

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestReflectionSlotUpdateRejectsFrozenRevisionDrift(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	fluctlightID := "slot-cas-fluctlight"
	entityID := reflectionSlotEntityID(EvolutionDrive, fluctlightID, "achievement")
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_drive_slots(id,fluctlight_id,key,label,description,value_schema,value,confidence,evidence_refs,revision) VALUES($1,$2,'achievement','Achievement','Goal pressure','pressure','{"pressure":0.4,"salience":0.5}','0.9','["sequence:1"]',2)`, entityID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	projection := ContextProjection{SchemaVersion: "fluctlight.context.v3", FluctlightID: fluctlightID, OwnerActorID: "slot-cas-owner", CurrentSpeaker: map[string]any{"actor_id": "slot-cas-owner"}}
	index := newContextReferenceIndex(projection)
	ref, err := index.add(ContextReferenceDrive, entityID, 1, map[string]any{"id": entityID, "key": "achievement", "value": map[string]any{"pressure": 0.4}, "revision": 1})
	if err != nil {
		t.Fatal(err)
	}
	proposal := ReflectionProposalV2{DriveCandidates: []ReflectionSlotCandidateV2{{Operation: "update", TargetRef: ref, Key: "achievement", SemanticValue: "Goal pressure", Direction: "increase", Strength: 0.5, Confidence: 0.9, EvidenceRefs: []string{"sequence:1"}}}}
	plan := ReflectionEvolutionPlan{ProposalID: "slot-cas-proposal", FluctlightID: fluctlightID, SourceWindow: "sequence:1-1", Candidates: []EvolutionCandidatePlan{{CandidateID: "slot-cas-candidate", Domain: EvolutionDrive, Index: 0, Operation: "update", TargetRef: ref, Disposition: EvolutionAccepted}}}
	evolution := EvolutionContext{FluctlightID: fluctlightID, Evidence: []EvolutionEvidence{{Ref: "sequence:1", Kind: "fact", Summary: "fact", Sequence: 1, OccurredAt: time.Now().UTC()}}, ReferenceIndex: index}
	err = withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		return (&App{DB: repository}).applyReflectionSlotCandidatesV2Tx(ctx, tx, fluctlightID, proposal, plan, evolution)
	})
	if err == nil || err.Error() != "reflection_slot_revision_stale" {
		t.Fatalf("stale slot update error=%v", err)
	}
}

func TestReflectionIntentionRejectsForeignNestedRefsBeforeApply(t *testing.T) {
	now := time.Now().UTC()
	projection := ContextProjection{SchemaVersion: "fluctlight.context.v3", FluctlightID: "nested-ref-fl", OwnerActorID: "nested-ref-owner", CurrentSpeaker: map[string]any{"actor_id": "nested-ref-owner"}}
	index := newContextReferenceIndex(projection)
	goalRef, err := index.add(ContextReferenceGoal, "goal-1", 1, map[string]any{"desired_outcome": "valid"})
	if err != nil {
		t.Fatal(err)
	}
	context, err := BuildEvolutionContext(EvolutionContext{ProviderRole: "reflection", FluctlightID: projection.FluctlightID, SourceWindow: "sequence:1-1", FromSequence: 1, ToSequence: 1, Watermark: 0, Evidence: []EvolutionEvidence{{Ref: "sequence:1", Kind: "fact", Summary: "fact", Sequence: 1, OccurredAt: now}}, ReferenceIndex: index, BaseRevisions: map[string]int{string(EvolutionIntention): 0}})
	if err != nil {
		t.Fatal(err)
	}
	proposal := ReflectionProposalV2{SchemaVersion: reflectionProposalV2SchemaVersion, Summary: "invalid nested ref", IntentionCandidates: []ReflectionIntentionCandidateV2{{Operation: "create", GoalRef: goalRef, ActionIntent: "act", ExpectedOutcome: "done", TypedTrigger: &TypedIntentionTrigger{Type: IntentionTriggerEvent, EventType: "event", EventRef: "scene:ctx_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, Expiration: ptrTime(now.Add(time.Hour)), Confidence: 0.9, EvidenceRefs: []string{"sequence:1"}, SemanticReason: "test"}}}
	if _, err := CompileReflectionPlan(proposal, context, ReflectionPolicyV2{}, now); err == nil {
		t.Fatal("foreign nested event_ref was accepted")
	}
}

func ptrTime(value time.Time) *time.Time { return &value }

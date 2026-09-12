package core

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestReflectionV2StageS08(t *testing.T) {
	now := time.Date(2026, 9, 11, 14, 0, 0, 0, time.UTC)
	projection := ContextProjection{
		FluctlightID: "fl-s08", OwnerActorID: "owner-s08", ConversationID: "conversation-s08",
		CurrentSpeaker: map[string]any{"actor_id": "owner-s08"}, PersonalityRuntime: map[string]any{"active_profile_id": "default"},
	}
	index := newContextReferenceIndex(projection)
	goalRef, err := index.add(ContextReferenceGoal, "goal-s08", 2, map[string]any{"desired_outcome": "finish"})
	if err != nil {
		t.Fatal(err)
	}
	memoryRef, err := index.add(ContextReferenceMemory, "memory-s08", 3, map[string]any{"content": "bounded memory"})
	if err != nil {
		t.Fatal(err)
	}
	context, err := BuildEvolutionContext(EvolutionContext{
		ProviderRole: "reflection", FluctlightID: "fl-s08", SourceWindow: "window-s08", FromSequence: 5, ToSequence: 6, Watermark: 4,
		Evidence:       []EvolutionEvidence{{Ref: "fact:5", Kind: "conversation", Summary: "事实一", Sequence: 5, OccurredAt: now.Add(-time.Minute)}, {Ref: "fact:6", Kind: "outcome", Summary: "事实二", Sequence: 6, OccurredAt: now}},
		ReferenceIndex: index,
		BaseRevisions:  map[string]int{string(EvolutionMemory): 3, string(EvolutionGoal): 2, string(EvolutionAffectProfile): 1, string(EvolutionPersonality): 4},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("closed schema rejects runtime ownership", func(t *testing.T) {
		valid := map[string]any{
			"schema_version": reflectionProposalV2SchemaVersion, "summary": "本窗口总结",
			"memory_candidates": []any{}, "relationship_observations": []any{}, "goal_candidates": []any{}, "intention_candidates": []any{},
			"emotional_summary":               map[string]any{"dominant_patterns": []any{}, "triggers": []any{}, "recovery_patterns": []any{}, "conflicts": []any{}, "evidence_refs": []any{}},
			"affect_recalibration_candidates": []any{}, "drive_candidates": []any{}, "preference_candidates": []any{}, "trigger_candidates": []any{},
			"developing_self_candidates": []any{}, "personality_evolution_candidates": []any{}, "behavior_policy_evolution_candidates": []any{},
		}
		raw, _ := json.Marshal(valid)
		if _, err := DecodeReflectionProposalV2(raw); err != nil {
			t.Fatalf("valid closed proposal rejected: %v", err)
		}
		valid["profile_id"] = "provider-owned"
		raw, _ = json.Marshal(valid)
		if _, err := DecodeReflectionProposalV2(raw); err == nil {
			t.Fatal("runtime-owned profile_id crossed closed proposal schema")
		}
	})

	proposal := ReflectionProposalV2{
		SchemaVersion: reflectionProposalV2SchemaVersion, Summary: "从事实与结果形成候选",
		MemoryCandidates:               []ReflectionMemoryCandidateV2{{Operation: "revise", TargetRef: memoryRef, Confidence: 0.9, Importance: 0.7, EvidenceRefs: []string{"fact:5"}, SemanticReason: "新事实修正"}},
		GoalCandidates:                 []ReflectionGoalCandidateV2{{Operation: "update", TargetRef: goalRef, Direction: "increase", Strength: 0.8, Confidence: 0.9, EvidenceRefs: []string{"fact:6"}, SemanticReason: "结果支持推进"}},
		AffectRecalibrationCandidates:  []ReflectionAffectCandidateV2{{Target: "baseline.pleasure", Direction: "increase", Strength: 0.3, Confidence: 0.7, EvidenceRefs: []string{"fact:5", "fact:6"}, SemanticReason: "跨事实但置信度不足"}},
		PersonalityEvolutionCandidates: []ReflectionOverlayCandidateV2{{FieldPath: "traits.openness", Direction: "increase", Strength: 0.2, Confidence: 0.9, EvidenceRefs: []string{"fact:5"}, SemanticReason: "单窗口不足"}},
	}

	t.Run("compile records accepted and deferred dispositions", func(t *testing.T) {
		plan, err := CompileReflectionPlan(proposal, context, ReflectionPolicyV2{}, now)
		if err != nil || plan.ProposalID == "" || len(plan.Candidates) != 4 {
			t.Fatalf("plan=%#v err=%v", plan, err)
		}
		byDomain := map[EvolutionDomain]EvolutionCandidatePlan{}
		for _, candidate := range plan.Candidates {
			byDomain[candidate.Domain] = candidate
		}
		if byDomain[EvolutionMemory].Disposition != EvolutionAccepted || byDomain[EvolutionGoal].Disposition != EvolutionAccepted || byDomain[EvolutionAffectProfile].Disposition != EvolutionDeferred || byDomain[EvolutionPersonality].Disposition != EvolutionDeferred {
			t.Fatalf("dispositions=%#v", byDomain)
		}
	})

	t.Run("foreign evidence invalidates the whole proposal", func(t *testing.T) {
		bad := proposal
		bad.GoalCandidates = append([]ReflectionGoalCandidateV2(nil), proposal.GoalCandidates...)
		bad.GoalCandidates[0].EvidenceRefs = []string{"fact:foreign"}
		if _, err := CompileReflectionPlan(bad, context, ReflectionPolicyV2{}, now); err == nil {
			t.Fatal("foreign evidence was silently dropped")
		}
	})

	t.Run("CAS and domain failure roll back watermark and revisions", func(t *testing.T) {
		plan, err := CompileReflectionPlan(proposal, context, ReflectionPolicyV2{}, now)
		if err != nil {
			t.Fatal(err)
		}
		state := EvolutionState{FluctlightID: "fl-s08", Watermark: 4, Revisions: cloneRevisionMap(context.BaseRevisions)}
		stale := state
		stale.Revisions = cloneRevisionMap(state.Revisions)
		stale.Revisions[string(EvolutionGoal)]++
		if _, _, err := ApplyReflectionPlan(stale, plan, nil); err == nil {
			t.Fatal("stale domain revision advanced reflection watermark")
		}
		failedState, _, err := ApplyReflectionPlan(state, plan, map[EvolutionDomain]EvolutionDomainApplier{EvolutionGoal: func(EvolutionCandidatePlan) error { return errors.New("forced apply failure") }})
		if err == nil || failedState.Watermark != state.Watermark || failedState.Revisions[string(EvolutionMemory)] != state.Revisions[string(EvolutionMemory)] {
			t.Fatalf("domain failure partially applied: state=%#v err=%v", failedState, err)
		}
		next, result, err := ApplyReflectionPlan(state, plan, nil)
		if err != nil || next.Watermark != 6 || next.Revisions[string(EvolutionMemory)] != 4 || next.Revisions[string(EvolutionGoal)] != 3 || result.Status != "applied" || result.Counts[EvolutionAffectProfile].Deferred != 1 || len(result.ChangedRefs) != 2 {
			t.Fatalf("apply next=%#v result=%#v err=%v", next, result, err)
		}
	})

	t.Run("empty proposal is explicit no_change and WakeUp cannot own reflection", func(t *testing.T) {
		empty := ReflectionProposalV2{SchemaVersion: reflectionProposalV2SchemaVersion, Summary: "没有足够变化"}
		plan, err := CompileReflectionPlan(empty, context, ReflectionPolicyV2{}, now)
		if err != nil {
			t.Fatal(err)
		}
		state := EvolutionState{FluctlightID: "fl-s08", Watermark: 4, Revisions: cloneRevisionMap(context.BaseRevisions)}
		next, result, err := ApplyReflectionPlan(state, plan, nil)
		if err != nil || result.Status != "no_change" || next.Watermark != 6 || len(result.ChangedRefs) != 0 {
			t.Fatalf("no_change next=%#v result=%#v err=%v", next, result, err)
		}
		wakeContext := context
		wakeContext.ProviderRole = "wake_up"
		if _, err := BuildEvolutionContext(wakeContext); err == nil || !strings.Contains(err.Error(), "identity") {
			t.Fatalf("WakeUp was allowed to own Reflection mutation: %v", err)
		}
	})

	t.Run("domain no-change and developing-self threshold stay truthful", func(t *testing.T) {
		memoryPlan := ReflectionEvolutionPlan{
			ProposalID: "proposal-memory-no-change", FluctlightID: context.FluctlightID, SourceWindow: context.SourceWindow,
			BaseWatermark: context.Watermark, ToSequence: context.ToSequence, ExpectedRevisions: cloneRevisionMap(context.BaseRevisions), PolicyVersion: reflectionPolicyVersion,
			Candidates: []EvolutionCandidatePlan{{CandidateID: "memory-no-change", Domain: EvolutionMemory, Disposition: EvolutionAccepted, TargetRef: memoryRef, ReasonCode: "policy_accepted"}},
		}
		reconcileReflectionMemoryDispositions(&memoryPlan, []MemoryApplyResult{{Disposition: "no_change", ReasonCode: "exact_duplicate"}})
		state := EvolutionState{FluctlightID: context.FluctlightID, Watermark: context.Watermark, Revisions: cloneRevisionMap(context.BaseRevisions)}
		next, result, err := ApplyReflectionPlan(state, memoryPlan, nil)
		if err != nil || result.Counts[EvolutionMemory].NoChange != 1 || result.Counts[EvolutionMemory].Applied != 0 || len(result.ChangedRefs) != 0 || next.Revisions[string(EvolutionMemory)] != state.Revisions[string(EvolutionMemory)] {
			t.Fatalf("memory no-change was reported as applied: next=%#v result=%#v err=%v", next, result, err)
		}

		selfProposal := ReflectionProposalV2{SchemaVersion: reflectionProposalV2SchemaVersion, Summary: "single evidence", DevelopingSelfCandidates: []ReflectionDevelopingSelfCandidateV2{{Operation: "create", Category: "habit", Claim: "test", Confidence: 0.9, EvidenceRefs: []string{"fact:5"}, SemanticReason: "one fact"}}}
		selfContext := context
		selfContext.BaseRevisions = cloneRevisionMap(context.BaseRevisions)
		selfContext.BaseRevisions[string(EvolutionDevelopingSelf)] = 0
		compiled, err := CompileReflectionPlan(selfProposal, selfContext, ReflectionPolicyV2{}, now)
		if err != nil || len(compiled.Candidates) != 1 || compiled.Candidates[0].Disposition != EvolutionDeferred || compiled.Candidates[0].ReasonCode != "cross_window_evidence_required" {
			t.Fatalf("single-evidence Developing Self was not deferred: %#v err=%v", compiled, err)
		}
	})
}

func TestReflectionProviderCompletionRejectsToolCalls(t *testing.T) {
	valid := ProviderCompletion{Structured: map[string]any{"schema_version": reflectionProposalV2SchemaVersion}}
	if err := validateReflectionProviderCompletion(valid); err != nil {
		t.Fatalf("valid Reflection completion rejected: %v", err)
	}
	withTool := valid
	withTool.ToolCalls = []CapabilityInvocation{{CallID: "unexpected", CapabilityName: "conversation.reply"}}
	if err := validateReflectionProviderCompletion(withTool); err == nil || err.Error() != "reflection_tool_call_forbidden" {
		t.Fatalf("unexpected Reflection Tool Call was not rejected: %v", err)
	}
	if err := validateReflectionProviderCompletion(ProviderCompletion{StructuredFallback: true}); err == nil || err.Error() != "reflection_structured_response_invalid" {
		t.Fatalf("invalid Reflection fallback was not rejected: %v", err)
	}
}

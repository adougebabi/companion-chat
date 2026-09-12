package core

import (
	"strings"
	"testing"
	"time"
)

func reflectionMemorySemanticCandidate(operation string) map[string]any {
	return map[string]any{
		"operation": operation, "type": "semantic", "content": "用户偏好安静且靠窗的位置",
		"confidence": 0.9, "importance": 0.8, "emotional_significance": 0.3,
		"evidence_refs": []any{"sequence:7"}, "semantic_reason": "窗口内的明确事实支持该变化",
	}
}

func reflectionProposalV2Fixture(memoryCandidates []any) map[string]any {
	return map[string]any{
		"schema_version": reflectionProposalV2SchemaVersion, "summary": "测试窗口的结构化反思",
		"active_memory_candidates": []any{}, "memory_candidates": memoryCandidates, "relationship_observations": []any{}, "goal_candidates": []any{}, "intention_candidates": []any{},
		"emotional_summary":               map[string]any{"dominant_patterns": []any{}, "triggers": []any{}, "recovery_patterns": []any{}, "conflicts": []any{}, "evidence_refs": []any{}},
		"affect_recalibration_candidates": []any{}, "drive_candidates": []any{}, "preference_candidates": []any{}, "trigger_candidates": []any{},
		"developing_self_candidates": []any{}, "personality_evolution_candidates": []any{}, "behavior_policy_evolution_candidates": []any{},
	}
}

func TestCompileReflectionActiveMemoryCommandsKeepsSeparateDomainAndFrozenIndex(t *testing.T) {
	index := ContextReferenceIndex{
		SchemaVersion: contextReferenceIndexVersion, FluctlightID: "fl-reflection", OwnerActorID: "owner-reflection",
		SpeakerActorID: "owner-reflection", ConversationID: "conversation-7", ActiveProfileID: "default", ByRef: map[string]ContextReference{},
	}
	snapshot := map[string]any{
		"id": "active-memory-a", "owner_fluctlight_id": "fl-reflection", "conversation_id": "conversation-7",
		"kind": "commitment", "content": "今晚早点睡", "status": "active", "confidence": 0.9, "importance": 0.8,
		"source_fact_id": "fact-1", "evidence_refs": []any{"sequence:1"}, "time_precision": "unknown", "timezone": "UTC", "revision": 2,
	}
	ref, err := index.add(ContextReferenceActiveMemory, "active-memory-a", 2, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	occurredAt := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	validUntil := occurredAt.Add(19 * time.Hour)
	candidates := []reflectionAcceptedActiveMemoryCandidate{
		{Index: 3, Candidate: ReflectionActiveMemoryCandidateV1{
			Operation: "create", Kind: "future_event", Content: "明早七点赶飞机", Confidence: 0.95, Importance: 1,
			OriginalTimeExpression: "明早七点", ValidUntil: &validUntil, TimePrecision: "exact",
			EvidenceRefs: []string{"sequence:7"}, SemanticReason: "窗口内有明确的未来事件",
		}},
		{Index: 5, Candidate: ReflectionActiveMemoryCandidateV1{
			Operation: "complete", TargetRef: ref, EvidenceRefs: []string{"sequence:8"}, SemanticReason: "承诺已经完成",
		}},
	}
	request := reflectionActiveMemoryCompileRequest{
		FluctlightID: "fl-reflection", OwnerActorID: "owner-reflection", ProposalID: "reflection-proposal", SourceWindow: "sequence:7-8",
		Timezone: "UTC", OccurredAt: occurredAt, ReferenceIndex: index,
		AllowedEvidence: map[string]struct{}{"sequence:7": {}, "sequence:8": {}},
		EvidenceScopes: map[string]reflectionMemoryEvidenceScope{
			"sequence:7": {FactID: "fact-7", ConversationID: "conversation-7", Known: true},
			"sequence:8": {FactID: "fact-8", ConversationID: "conversation-7", Known: true},
		},
	}
	commands, err := compileReflectionActiveMemoryCommands(candidates, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 2 || commands[0].Operation != ActiveMemoryCreate || commands[0].Semantic == nil || commands[0].SourceFactID != "fact-7" || !strings.HasSuffix(commands[0].IdempotencyKey, ":3") {
		t.Fatalf("create command = %#v", commands)
	}
	if commands[1].Operation != ActiveMemoryComplete || commands[1].Target == nil || commands[1].Target.ActiveMemoryID != "active-memory-a" || commands[1].Target.ExpectedRevision != 2 || commands[1].Semantic != nil || !strings.HasSuffix(commands[1].IdempotencyKey, ":5") {
		t.Fatalf("complete command = %#v", commands[1])
	}
	for _, command := range commands {
		if err := validatePreparedActiveMemoryMutation(command); err != nil {
			t.Fatalf("compiled command rejected: %v", err)
		}
	}
}

func TestCompileReflectionPlanTreatsActiveMemoryAsIndependentDomain(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	index := ContextReferenceIndex{SchemaVersion: contextReferenceIndexVersion, FluctlightID: "fl-reflection", OwnerActorID: "owner-reflection", SpeakerActorID: "owner-reflection", ByRef: map[string]ContextReference{}}
	evolution, err := BuildEvolutionContext(EvolutionContext{
		ProviderRole: "reflection", FluctlightID: "fl-reflection", SourceWindow: "sequence:7-7", FromSequence: 7, ToSequence: 7, Watermark: 6,
		Evidence:       []EvolutionEvidence{{Ref: "sequence:7", Kind: "conversation.turn", Summary: "明早七点赶飞机", Sequence: 7, OccurredAt: now}},
		ReferenceIndex: index, BaseRevisions: map[string]int{string(EvolutionActiveMemory): 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	proposal := ReflectionProposalV2{
		SchemaVersion: reflectionProposalV2SchemaVersion, Summary: "识别临时未来事件",
		ActiveMemoryCandidates: []ReflectionActiveMemoryCandidateV1{{
			Operation: "create", Kind: "future_event", Content: "明早七点赶飞机", Confidence: 0.95, Importance: 1,
			TimePrecision: "unknown", EvidenceRefs: []string{"sequence:7"}, SemanticReason: "当前仍有行为意义",
		}},
	}
	plan, err := CompileReflectionPlan(proposal, evolution, ReflectionPolicyV2{SupportedDomains: map[EvolutionDomain]bool{EvolutionActiveMemory: true}}, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Candidates) != 1 || plan.Candidates[0].Domain != EvolutionActiveMemory || plan.Candidates[0].Disposition != EvolutionAccepted {
		t.Fatalf("plan = %#v", plan)
	}
	if plan.Candidates[0].Domain == EvolutionMemory {
		t.Fatal("Active Memory was folded into durable Memory")
	}
}

func TestReflectionMemoryCandidateV2OperationShapesAreClosed(t *testing.T) {
	allowed := map[string]struct{}{"sequence:7": {}, "memory:ctx_0123456789abcdef0123456789abcdef": {}, "memory:ctx_abcdef0123456789abcdef0123456789": {}}
	target := "memory:ctx_0123456789abcdef0123456789abcdef"
	merge := "memory:ctx_abcdef0123456789abcdef0123456789"
	cases := []map[string]any{
		reflectionMemorySemanticCandidate("create"),
		{"operation": "confirm", "target_ref": target, "evidence_refs": []any{"sequence:7"}, "semantic_reason": "再次确认"},
		reflectionMemorySemanticCandidate("revise"),
		reflectionMemorySemanticCandidate("merge"),
		reflectionMemorySemanticCandidate("supersede"),
		{"operation": "deprecate", "target_ref": target, "evidence_refs": []any{"sequence:7"}, "semantic_reason": "事实已不再有效"},
	}
	cases[2]["target_ref"] = target
	cases[3]["target_ref"], cases[3]["merge_refs"] = target, []any{merge}
	cases[4]["target_ref"] = target
	for _, candidate := range cases {
		if err := validateReflectionMemoryCandidate(candidate, allowed); err != nil {
			t.Fatalf("%s candidate rejected: %v", candidate["operation"], err)
		}
	}
	for name, candidate := range map[string]map[string]any{
		"runtime visibility": {"operation": "deprecate", "target_ref": target, "evidence_refs": []any{"sequence:7"}, "semantic_reason": "过期", "visibility": "private"},
		"semantic confirm":   {"operation": "confirm", "target_ref": target, "type": "semantic", "content": "forbidden", "confidence": 1.0, "importance": 1.0, "emotional_significance": 0.0, "evidence_refs": []any{"sequence:7"}, "semantic_reason": "确认"},
		"missing merge":      reflectionMemorySemanticCandidate("merge"),
		"foreign evidence":   {"operation": "deprecate", "target_ref": target, "evidence_refs": []any{"sequence:999"}, "semantic_reason": "过期"},
	} {
		normalized := normalizeReflectionMemoryCandidate(candidate)
		if err := validateReflectionMemoryCandidate(normalized, allowed); err == nil {
			t.Fatalf("%s candidate was accepted: %#v", name, normalized)
		}
	}
}

func TestCompileReflectionMemoryCommandsBindsOpaqueRefsAndRejectsOverlap(t *testing.T) {
	index := ContextReferenceIndex{
		SchemaVersion: contextReferenceIndexVersion, FluctlightID: "fl-reflection", OwnerActorID: "owner-reflection",
		SpeakerActorID: "owner-reflection", ActiveProfileID: "default", ByRef: map[string]ContextReference{},
	}
	snapshot := func(id, content string) map[string]any {
		return map[string]any{
			"id": id, "owner_fluctlight_id": "fl-reflection", "type": "semantic", "content": content,
			"conversation_id": "conversation-7",
			"actor_refs":      []any{}, "event_refs": []any{}, "evidence_refs": []any{"sequence:1"}, "personality_perspectives": []any{},
			"confidence": 0.8, "importance": 0.7, "emotional_significance": 0.2,
			"visibility": "private", "status": "active", "revision": 0, "canonical_key": "key", "request_digest": "digest",
			"created_at": "2026-09-11T00:00:00Z",
		}
	}
	refA, err := index.add(ContextReferenceMemory, "memory-a", 0, snapshot("memory-a", "旧事实 A"))
	if err != nil {
		t.Fatal(err)
	}
	refB, err := index.add(ContextReferenceMemory, "memory-b", 0, snapshot("memory-b", "旧事实 B"))
	if err != nil {
		t.Fatal(err)
	}
	merge := reflectionMemorySemanticCandidate("merge")
	merge["target_ref"], merge["merge_refs"] = refA, []any{refB}
	proposal := map[string]any{"memory_candidates": []any{merge}}
	allowed := map[string]struct{}{"sequence:7": {}, refA: {}, refB: {}}
	request := reflectionMemoryCompileRequest{
		FluctlightID: "fl-reflection", OwnerActorID: "owner-reflection", ActiveProfileID: "default",
		ProposalID: "reflection-proposal", SourceWindow: "sequence:7-7", OccurredAt: time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC),
		ReferenceIndex: index, AllowedEvidence: allowed,
		EvidenceScopes: map[string]reflectionMemoryEvidenceScope{"sequence:7": {FactID: "fact-7", ConversationID: "conversation-7", Known: true}},
	}
	commands, err := compileReflectionMemoryCommands(proposal, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 1 || commands[0].Target == nil || commands[0].Target.MemoryID != "memory-a" || commands[0].Target.ExpectedRevision != 0 || commands[0].Target.Ref != refA || len(commands[0].MergeTargets) != 1 || commands[0].MergeTargets[0].MemoryID != "memory-b" || commands[0].MergeTargets[0].Ref != refB {
		t.Fatalf("compiled commands = %#v", commands)
	}
	if commands[0].OwnerFluctlightID != "fl-reflection" || commands[0].OwnerActorID != "owner-reflection" || commands[0].ActorID != "fl-reflection" || commands[0].ProposalID != "reflection-proposal" || commands[0].CandidateIndex != 0 || commands[0].SourceFactID != "fact-7" {
		t.Fatalf("runtime authority was not compiled: %#v", commands[0])
	}
	second := reflectionMemorySemanticCandidate("revise")
	second["target_ref"] = refA
	proposal["memory_candidates"] = []any{merge, second}
	if _, err := compileReflectionMemoryCommands(proposal, request); err == nil || err.Error() != "reflection_memory_target_overlap" {
		t.Fatalf("overlapping targets err=%v", err)
	}
	conflictingCreate := reflectionMemorySemanticCandidate("create")
	conflictingCreate["evidence_refs"] = []any{"sequence:7", refB}
	proposal["memory_candidates"] = []any{conflictingCreate}
	request.EvidenceScopes[refB] = reflectionMemoryEvidenceScope{ConversationID: "conversation-other", Known: true}
	if _, err := compileReflectionMemoryCommands(proposal, request); err == nil || !strings.Contains(err.Error(), "memory_evidence_conversation_conflict") {
		t.Fatalf("cross-conversation create err=%v", err)
	}
	request.EvidenceScopes[refB] = reflectionMemoryEvidenceScope{Known: false}
	if _, err := compileReflectionMemoryCommands(proposal, request); err == nil || !strings.Contains(err.Error(), "memory_evidence_scope_unknown") {
		t.Fatalf("unknown evidence scope err=%v", err)
	}
}

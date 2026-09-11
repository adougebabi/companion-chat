package core

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestContextReferenceInfluenceRoundTrip(t *testing.T) {
	projection := ContextProjection{
		SchemaVersion:  "fluctlight.context.v3",
		FluctlightID:   "fluctlight_internal_a",
		OwnerActorID:   "actor_internal_owner",
		ConversationID: "conversation_internal_a",
		PersonalityRuntime: map[string]any{
			"active_profile_id": "profile_a",
		},
		Memories: []map[string]any{{
			"id": "memory_internal_a", "revision": 3, "type": "semantic", "content": "相同语义",
		}},
		Goals: []map[string]any{{
			"id": "goal_internal_a", "revision": 7, "description": "相同语义", "status": "active",
		}},
		Intentions: []map[string]any{{
			"id": "intention_internal_a", "revision": 2, "goal_id": "goal_internal_a", "action": "稍后继续",
		}},
		Schedule: map[string]any{
			"id": "schedule_internal_a", "revision": 5, "local_date": "2026-09-10", "timezone": "Asia/Shanghai",
			"items": []any{map[string]any{"id": "schedule_item_internal_a", "scene": "书房", "activity": "阅读", "start_at": "2026-09-10T00:00:00+08:00", "end_at": "2026-09-11T00:00:00+08:00"}},
		},
		LifeContext: map[string]any{"source": "schedule", "schedule_id": "schedule_internal_a", "schedule_item_id": "schedule_item_internal_a", "schedule_revision": 5, "scene": "书房", "activity": "阅读"},
		RecentOutcomes: []map[string]any{{
			"id": "outcome_internal_a", "revision": 2, "capability_name": "memory_event", "status": "completed", "success_boundary": "memory_revision_committed", "observed": map[string]any{"status": "completed"},
		}},
		CurrentStateRevision: 4,
		InnerState:           map[string]any{"revision": 4, "mood": map[string]any{"label": "平静"}},
		AffectProfile: map[string]any{
			"revision": 1, "baseline_pad": map[string]any{"pleasure": 0.0, "arousal": 0.0, "dominance": 0.0},
			"decay_policy": map[string]any{"pad_half_life_seconds": 21600}, "regulation_policy": map[string]any{"strength": 0.2},
		},
		CurrentState: map[string]any{
			"authority": "transient_state",
			"data":      map[string]any{"inner_state": map[string]any{"revision": 4, "mood": map[string]any{"label": "平静"}}},
		},
	}
	mapValue(projection.CurrentState["data"])["affect_profile"] = projection.AffectProfile
	if err := buildContextReferenceIndex(&projection); err != nil {
		t.Fatalf("build reference index: %v", err)
	}

	memoryRef := stringValue(projection.Memories[0]["ref"])
	goalRef := stringValue(projection.Goals[0]["ref"])
	intentionRef := stringValue(projection.Intentions[0]["ref"])
	stateRef := stringValue(projection.InnerState["ref"])
	sceneRef := stringValue(projection.LifeContext["ref"])
	outcomeRef := stringValue(projection.RecentOutcomes[0]["ref"])
	affectProfileRef := stringValue(projection.AffectProfile["ref"])
	if memoryRef == "" || goalRef == "" || intentionRef == "" || stateRef == "" || sceneRef == "" || outcomeRef == "" || affectProfileRef == "" {
		t.Fatalf("expected opaque refs, got memory=%q goal=%q intention=%q state=%q scene=%q outcome=%q affect_profile=%q", memoryRef, goalRef, intentionRef, stateRef, sceneRef, outcomeRef, affectProfileRef)
	}
	if memoryRef == goalRef || strings.Contains(memoryRef, "memory_internal_a") || strings.Contains(goalRef, "goal_internal_a") {
		t.Fatalf("refs must be kind-separated and opaque: memory=%q goal=%q", memoryRef, goalRef)
	}
	if got := stringValue(projection.Intentions[0]["goal_ref"]); got != goalRef {
		t.Fatalf("intention goal ref=%q want %q", got, goalRef)
	}

	roundTripped, ok := contextProjectionFromValue(decodeObject(jsonBytes(projection)))
	if !ok {
		t.Fatal("projection round trip failed")
	}
	entry, ok := roundTripped.ReferenceIndex.ByRef[memoryRef]
	if !ok || entry.EntityID != "memory_internal_a" || entry.Revision != 3 || entry.Scope == "" {
		t.Fatalf("reference mapping did not round trip: %#v", entry)
	}
	if !json.Valid(entry.Snapshot) {
		t.Fatalf("reference snapshot is not valid JSON: %q", string(entry.Snapshot))
	}

	provider := compactCognitionContext(roundTripped)
	encoded := string(jsonBytes(provider))
	for _, forbidden := range []string{"context_reference_index", "memory_internal_a", "goal_internal_a", "intention_internal_a", "schedule_internal_a", "schedule_item_internal_a", "outcome_internal_a", "baseline_pad", "pad_half_life_seconds", "regulation_policy", "emotional_summary"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("provider context leaked %q: %s", forbidden, encoded)
		}
	}
	if !strings.Contains(encoded, memoryRef) || !strings.Contains(encoded, goalRef) || !strings.Contains(encoded, intentionRef) || !strings.Contains(encoded, stateRef) || !strings.Contains(encoded, sceneRef) || !strings.Contains(encoded, outcomeRef) || !strings.Contains(encoded, affectProfileRef) {
		t.Fatalf("provider context lost opaque refs: %s", encoded)
	}

	influences, err := validateDecisionInfluences([]any{
		map[string]any{"ref": memoryRef, "role": "grounds", "confidence": 0.9, "note": "该记忆提供事实依据"},
		map[string]any{"ref": goalRef, "role": "motivates", "confidence": 0.8, "note": "该目标决定后续方向"},
		map[string]any{"ref": intentionRef, "role": "motivates", "confidence": 0.8, "note": "该意图连接目标与行动"},
		map[string]any{"ref": stateRef, "role": "constrains", "confidence": 0.7, "note": "当前情绪约束表达"},
		map[string]any{"ref": sceneRef, "role": "grounds", "confidence": 0.9, "note": "当前场景提供事实上下文"},
		map[string]any{"ref": outcomeRef, "role": "grounds", "confidence": 0.9, "note": "最近行动结果提供反馈"},
		map[string]any{"ref": affectProfileRef, "role": "constrains", "confidence": 0.8, "note": "情绪基线和调节策略约束状态变化"},
	}, roundTripped.ReferenceIndex)
	if err != nil {
		t.Fatalf("validate influences: %v", err)
	}
	decision := map[string]any{"influences": influences, "context_projection": roundTripped}
	replayed := decodeObject(jsonBytes(decision))
	replayedProjection, ok := contextProjectionFromValue(replayed["context_projection"])
	if !ok || replayedProjection.ReferenceIndex.ByRef[memoryRef].Revision != 3 {
		t.Fatalf("frozen reference index did not survive replay: %#v", replayedProjection.ReferenceIndex)
	}
	if got := len(arrayValue(replayed["influences"])); got != 7 {
		t.Fatalf("frozen influences=%d want 7", got)
	}
}

func TestSameObservationDifferentFrozenSceneProducesDistinctInfluenceChain(t *testing.T) {
	build := func(scene, eventID, contextRevision string, revision int) ContextProjection {
		projection := ContextProjection{
			SchemaVersion: "fluctlight.context.v3", FluctlightID: "scene-influence-fluctlight",
			OwnerActorID: "scene-influence-owner", SourceFactID: "same-observation",
			LifeContextRevision: contextRevision,
			LifeContext: map[string]any{
				"source": "event", "authority_status": "confirmed", "scene": scene, "activity": "观察",
				"event_id": eventID, "event_revision": revision, "context_revision": contextRevision, "revision": revision,
				"effective_at": "2026-09-11T08:00:00Z", "expires_at": "2026-09-11T10:00:00Z",
			},
			CurrentState: map[string]any{"authority": "transient_state", "data": map[string]any{}},
		}
		mapValue(projection.CurrentState["data"])["life_context"] = projection.LifeContext
		if err := buildContextReferenceIndex(&projection); err != nil {
			t.Fatalf("build %s projection: %v", scene, err)
		}
		return projection
	}
	projectionA := build("书房", "event-scene-a", "life_ctx_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 3)
	projectionB := build("客厅", "event-scene-b", "life_ctx_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", 7)
	refA := stringValue(projectionA.LifeContext["ref"])
	refB := stringValue(projectionB.LifeContext["ref"])
	decisionA := map[string]any{"action_type": "no_op", "influences": []any{map[string]any{"ref": refA, "role": "constrains", "confidence": 0.9, "note": "书房场景约束本次观察"}}}
	decisionB := map[string]any{"action_type": "no_op", "influences": []any{map[string]any{"ref": refB, "role": "constrains", "confidence": 0.9, "note": "客厅场景约束本次观察"}}}
	influencesA, err := freezeDecisionInfluences(decisionA, projectionA, false)
	if err != nil {
		t.Fatal(err)
	}
	influencesB, err := freezeDecisionInfluences(decisionB, projectionB, false)
	if err != nil {
		t.Fatal(err)
	}
	if refA == "" || refB == "" || refA == refB || len(influencesA) != 1 || len(influencesB) != 1 || influencesA[0].Ref == influencesB[0].Ref {
		t.Fatalf("different frozen scenes shared influence chain: A=%#v B=%#v", influencesA, influencesB)
	}
	if _, err := freezeDecisionInfluences(map[string]any{"influences": []any{map[string]any{"ref": refA, "role": "constrains", "confidence": 0.9, "note": "错误复用旧场景"}}}, projectionB, false); err == nil {
		t.Fatal("scene A influence was accepted against frozen scene B")
	}
}

func TestValidateDecisionInfluencesFailsClosed(t *testing.T) {
	index := ContextReferenceIndex{
		SchemaVersion:   "fluctlight.context-reference-index.v1",
		FluctlightID:    "fluctlight_a",
		OwnerActorID:    "actor_a",
		ConversationID:  "conversation_a",
		ActiveProfileID: "profile_a",
		ByRef:           map[string]ContextReference{},
	}
	valid := ContextReference{
		Kind: ContextReferenceGoal, EntityID: "goal_a", Revision: 4,
		Scope:    contextReferenceScope("fluctlight_a", "actor_a", "conversation_a", "profile_a"),
		Snapshot: json.RawMessage(`{"description":"继续完成"}`),
	}
	valid.Ref = contextReferenceToken(valid.Kind, valid.EntityID, valid.Revision, valid.Scope, valid.Snapshot)
	index.ByRef[valid.Ref] = valid

	validInfluence := func() []any {
		return []any{map[string]any{"ref": valid.Ref, "role": "motivates", "confidence": 0.75, "note": "目标影响了选择"}}
	}
	cases := []struct {
		name  string
		value any
		index ContextReferenceIndex
	}{
		{name: "foreign ref", value: []any{map[string]any{"ref": "goal:ctx_0123456789abcdef0123456789abcdef", "role": "motivates", "confidence": 0.75, "note": "目标影响了选择"}}, index: index},
		{name: "stale revision token", value: []any{map[string]any{"ref": contextReferenceToken(valid.Kind, valid.EntityID, 3, valid.Scope, valid.Snapshot), "role": "motivates", "confidence": 0.75, "note": "旧版本"}}, index: index},
		{name: "duplicate", value: append(validInfluence(), validInfluence()...), index: index},
		{name: "bad role", value: []any{map[string]any{"ref": valid.Ref, "role": "causes", "confidence": 0.75, "note": "非法角色"}}, index: index},
		{name: "confidence below", value: []any{map[string]any{"ref": valid.Ref, "role": "motivates", "confidence": -0.01, "note": "非法置信度"}}, index: index},
		{name: "confidence above", value: []any{map[string]any{"ref": valid.Ref, "role": "motivates", "confidence": 1.01, "note": "非法置信度"}}, index: index},
		{name: "oversized ref", value: []any{map[string]any{"ref": strings.Repeat("x", maxContextReferenceRunes+1), "role": "grounds", "confidence": 0.5, "note": "过长"}}, index: index},
		{name: "oversized note", value: []any{map[string]any{"ref": valid.Ref, "role": "grounds", "confidence": 0.5, "note": strings.Repeat("注", maxDecisionInfluenceNoteRunes+1)}}, index: index},
		{name: "unknown field", value: []any{map[string]any{"ref": valid.Ref, "role": "grounds", "confidence": 0.5, "note": "合法", "entity_id": "forged"}}, index: index},
		{name: "not an array", value: map[string]any{"ref": valid.Ref}, index: index},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := validateDecisionInfluences(tc.value, tc.index); err == nil {
				t.Fatal("expected fail-closed validation error")
			}
		})
	}

	if influences, err := validateDecisionInfluences(validInfluence(), index); err != nil || len(influences) != 1 {
		t.Fatalf("valid influence rejected: influences=%#v err=%v", influences, err)
	}
	if influences, err := validateDecisionInfluences([]any{}, index); err != nil || len(influences) != 0 {
		t.Fatalf("empty reply/no-op influences must remain legal: influences=%#v err=%v", influences, err)
	}

	forged := index
	forged.ActiveProfileID = "profile_b"
	if _, err := validateDecisionInfluences(validInfluence(), forged); err == nil {
		t.Fatal("profile/scope-mismatched index must be rejected")
	}
}

func TestFreezeDecisionInfluencesUsesOnlyCoreOwnedIndex(t *testing.T) {
	projection := ContextProjection{
		SchemaVersion: "fluctlight.context.v3", FluctlightID: "fluctlight_a", OwnerActorID: "actor_a",
		Goals: []map[string]any{{"id": "goal_a", "revision": 1, "description": "完成闭环"}},
	}
	if err := buildContextReferenceIndex(&projection); err != nil {
		t.Fatal(err)
	}
	goalRef := stringValue(projection.Goals[0]["ref"])
	raw := []any{map[string]any{"ref": goalRef, "role": "motivates", "confidence": 0.8, "note": "目标决定行动"}}
	decision := map[string]any{"influences": raw}
	if _, err := freezeDecisionInfluences(decision, projection, true); err != nil {
		t.Fatalf("freeze influences: %v", err)
	}
	if stringValue(decision["context_reference_version"]) != contextReferenceIndexVersion {
		t.Fatalf("missing frozen reference version: %#v", decision)
	}
	if got := decisionServiceRefValues(decision["goal_refs"]); len(got) != 1 || got[0] != goalRef {
		t.Fatalf("goal service refs=%#v want %q", got, goalRef)
	}
	if err := validateFrozenDecisionInfluences(decision); err != nil {
		t.Fatalf("validate frozen influences: %v", err)
	}

	forged := map[string]any{
		"influences":              raw,
		"context_reference_index": map[string]any{"by_ref": map[string]any{goalRef: map[string]any{"entity_id": "foreign"}}},
	}
	if _, err := freezeDecisionInfluences(forged, projection, true); err == nil {
		t.Fatal("provider-supplied reference index must be rejected")
	}
	if _, err := freezeDecisionInfluences(map[string]any{"influences": []any{}, "cognitive_state_transition": "not_proposed"}, projection, false); err == nil {
		t.Fatal("Provider supplied a runtime-owned no-transition marker")
	}

	missing := map[string]any{"influences": []any{}}
	if _, err := freezeDecisionInfluences(missing, projection, true); err == nil {
		t.Fatal("required autonomous decision without influences must fail")
	}
	if _, err := freezeDecisionInfluences(missing, projection, false); err != nil {
		t.Fatalf("reply/no-op empty influences rejected: %v", err)
	}
}

func TestContextReferenceIndexContainsOnlyCurrentProviderScope(t *testing.T) {
	activeRelationship := map[string]any{"id": "relationship_active", "target_actor_id": "actor_user", "profile_id": "profile_a", "revision": 2, "summary": "active"}
	hiddenRelationship := map[string]any{"id": "relationship_hidden", "target_actor_id": "actor_user", "profile_id": "profile_b", "revision": 3, "summary": "hidden"}
	activeGoal := map[string]any{"id": "goal_active", "profile_id": "profile_a", "revision": 5, "description": "active"}
	hiddenGoal := map[string]any{"id": "goal_hidden", "profile_id": "profile_b", "revision": 6, "description": "hidden"}
	activeIntention := map[string]any{"id": "intention_active", "profile_id": "profile_a", "goal_id": "goal_active", "revision": 4, "action": "active"}
	hiddenIntention := map[string]any{"id": "intention_hidden", "profile_id": "profile_b", "goal_id": "goal_hidden", "revision": 4, "action": "hidden"}
	projection := ContextProjection{
		SchemaVersion: "fluctlight.context.v3", FluctlightID: "fluctlight_a", OwnerActorID: "actor_user", ConversationID: "conversation_a",
		PersonalityRuntime: map[string]any{"active_profile_id": "profile_a"},
		Relationships:      []map[string]any{hiddenRelationship, activeRelationship},
		Goals:              []map[string]any{activeGoal, hiddenGoal},
		Intentions:         []map[string]any{activeIntention, hiddenIntention},
		Schedule: map[string]any{
			"id": "schedule_a", "revision": 9,
			"items": []any{map[string]any{"id": "schedule_item_a", "scene": "书房", "activity": "阅读"}},
		},
		LifeContext:  map[string]any{"source": "schedule", "schedule_item_id": "schedule_item_a", "schedule_revision": 9, "scene": "书房", "activity": "阅读"},
		CurrentState: map[string]any{"authority": "transient_state", "data": map[string]any{"life_context": map[string]any{"source": "schedule", "schedule_item_id": "schedule_item_a", "schedule_revision": 9}}},
	}
	if err := buildContextReferenceIndex(&projection); err != nil {
		t.Fatal(err)
	}
	for _, visible := range []map[string]any{activeRelationship, activeGoal, activeIntention} {
		if stringValue(visible["ref"]) == "" {
			t.Fatalf("visible row did not receive ref: %#v", visible)
		}
	}
	for _, hidden := range []map[string]any{hiddenRelationship, hiddenGoal, hiddenIntention} {
		if stringValue(hidden["ref"]) != "" {
			t.Fatalf("hidden profile row received a valid ref: %#v", hidden)
		}
	}
	if ref := stringValue(projection.LifeContext["ref"]); ref == "" || projection.ReferenceIndex.ByRef[ref].Kind != ContextReferenceLifeContext {
		t.Fatalf("life context did not bind active schedule item: %#v", projection.LifeContext)
	}
	if itemRef := stringValue(projection.LifeContext["schedule_item_ref"]); itemRef == "" || projection.ReferenceIndex.ByRef[itemRef].Kind != ContextReferenceScheduleItem {
		t.Fatalf("life context did not reuse canonical schedule item ref: %#v", projection.LifeContext)
	}
	if stringValue(mapValue(mapValue(projection.CurrentState["data"])["life_context"])["ref"]) == "" {
		t.Fatal("canonical current-state life context lost source ref")
	}
	for _, entry := range projection.ReferenceIndex.ByRef {
		if strings.Contains(entry.EntityID, "hidden") {
			t.Fatalf("hidden profile entity entered reference index: %#v", entry)
		}
	}
}

func TestContextReferenceTokenChangesWhenDerivedSnapshotChangesWithoutDatabaseRevision(t *testing.T) {
	projection := func(pleasure float64, projectedAt string) ContextProjection {
		value := ContextProjection{
			SchemaVersion: "fluctlight.context.v3", FluctlightID: "fluctlight_a", OwnerActorID: "actor_a", CurrentStateRevision: 4,
			InnerState:   map[string]any{"revision": 4, "pad": map[string]any{"pleasure": pleasure}, "projected_at": projectedAt},
			CurrentState: map[string]any{"authority": "transient_state", "data": map[string]any{}},
		}
		if err := buildContextReferenceIndex(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	first := projection(0.8, "2026-09-10T08:00:00Z")
	second := projection(0.4, "2026-09-10T09:00:00Z")
	firstRef := stringValue(first.InnerState["ref"])
	secondRef := stringValue(second.InnerState["ref"])
	if firstRef == secondRef {
		t.Fatalf("derived snapshots at the same DB revision reused ref %q", firstRef)
	}
	if _, err := validateDecisionInfluences([]any{map[string]any{"ref": firstRef, "role": "grounds", "confidence": 0.9, "note": "旧状态"}}, second.ReferenceIndex); err == nil {
		t.Fatal("old derived-state ref remained valid in a newer wall-time projection")
	}
}

func TestDecisionInfluenceSchemaIsClosedAcrossCognitionSurfaces(t *testing.T) {
	schemas := map[string]map[string]any{
		"conversation": cognitiveTurnResponseSchema(),
		"daily_review": dailyReviewResponseSchema(),
		"wake_up":      wakeUpResponseSchema(),
		"native":       nativeCognitionResponseSchema(),
	}
	for name, schema := range schemas {
		t.Run(name, func(t *testing.T) {
			properties := mapValue(schema["properties"])
			influences := mapValue(properties["influences"])
			if stringValue(influences["type"]) != "array" {
				t.Fatalf("influences schema=%#v", influences)
			}
			item := mapValue(influences["items"])
			if additional, ok := item["additionalProperties"].(bool); !ok || additional {
				t.Fatalf("influence item must be closed: %#v", item)
			}
			for _, field := range []string{"ref", "role", "confidence", "note"} {
				if _, ok := mapValue(item["properties"])[field]; !ok || !containsSchemaRequired(item, field) {
					t.Fatalf("influence schema missing required %q: %#v", field, item)
				}
			}
			confidence := mapValue(mapValue(item["properties"])["confidence"])
			if number, ok := numberFloat(confidence["minimum"]); !ok || number != 0 {
				t.Fatalf("confidence minimum=%#v", confidence["minimum"])
			}
			if number, ok := numberFloat(confidence["maximum"]); !ok || number != 1 {
				t.Fatalf("confidence maximum=%#v", confidence["maximum"])
			}
		})
	}
}

func TestDriveSemanticSignalsResolveFrozenRefsAndCoreOwnsPressure(t *testing.T) {
	now := "2026-09-10T12:00:00Z"
	drives := mergeEffectiveDriveState(nil, nil)
	projection := ContextProjection{
		SchemaVersion: "fluctlight.context.v3", FluctlightID: "fluctlight_drive", OwnerActorID: "actor_drive", CurrentStateRevision: 2,
		InnerState: map[string]any{
			"revision": 2, "last_updated_at": now,
			"pad":  map[string]any{"pleasure": 0.0, "arousal": 0.0, "dominance": 0.0},
			"mood": map[string]any{"intensity": 0.0}, "momentum": map[string]any{"value": 0.0},
			"regulation": map[string]any{"stability": 0.0}, "drives": drives, "conflicts": []any{},
		},
		CurrentState: map[string]any{"authority": "transient_state", "data": map[string]any{}},
	}
	mapValue(projection.CurrentState["data"])["inner_state"] = projection.InnerState
	if err := buildContextReferenceIndex(&projection); err != nil {
		t.Fatal(err)
	}
	var socialRef string
	for _, raw := range arrayValue(projection.InnerState["drives"]) {
		drive := mapValue(raw)
		if stringValue(drive["key"]) == "social" {
			socialRef = stringValue(drive["ref"])
		}
	}
	if socialRef == "" || projection.ReferenceIndex.ByRef[socialRef].Kind != ContextReferenceDrive {
		t.Fatalf("built-in drive did not receive an opaque ref: %#v", projection.InnerState["drives"])
	}
	foreignAppraisal := map[string]any{
		"relevance": 0.5, "goal_congruence": 0.5, "reward": 0.5, "loss": 0.0, "social_threat": 0.0,
		"controllability": 0.5, "responsibility": 0.5, "relationship_significance": 0.0, "expected_effect": 0.5,
		"evidence_refs": []any{"memory:ctx_ffffffffffffffffffffffffffffffff"}, "event_kind": "test", "direction": "mixed",
	}
	if _, err := freezeDecisionInfluences(map[string]any{
		"appraisal":  foreignAppraisal,
		"influences": []any{map[string]any{"ref": socialRef, "role": "motivates", "confidence": 0.9, "note": "当前需要影响决定"}},
	}, projection, false); err == nil {
		t.Fatal("foreign appraisal evidence ref was accepted before freeze")
	}
	decision := map[string]any{
		"appraisal": map[string]any{
			"relevance": 0.5, "goal_congruence": 0.5, "reward": 0.5, "loss": 0.0, "social_threat": 0.0,
			"controllability": 0.5, "responsibility": 0.5, "relationship_significance": 0.0, "expected_effect": 0.5,
			"evidence_refs": []any{socialRef}, "event_kind": "test", "direction": "mixed",
			"drive_signals": []any{
				map[string]any{"ref": socialRef, "direction": "increase", "strength": 0.8, "confidence": 1.0, "evidence_refs": []any{socialRef}},
				map[string]any{"ref": socialRef, "direction": "decrease", "strength": 0.05, "confidence": 0.5, "evidence_refs": []any{socialRef}},
			},
		},
		"influences": []any{map[string]any{"ref": socialRef, "role": "motivates", "confidence": 0.9, "note": "社交需要影响行动选择"}},
	}
	if _, err := freezeDecisionInfluences(decision, projection, false); err != nil {
		t.Fatalf("freeze drive signals: %v", err)
	}
	signals, err := frozenDecisionDriveSignals(decision)
	if err != nil || len(signals) != 2 || signals[0].Key != "social" {
		t.Fatalf("resolved drive signals=%#v err=%v", signals, err)
	}
	current := cloneMap(projection.InnerState)
	for _, raw := range arrayValue(current["drives"]) {
		delete(mapValue(raw), "ref")
	}
	result, requested, applied := reduceAffectState(current, defaultAffectProfile(), affectReductionInput{Drives: signals}, mustTime(t, now))
	var social map[string]any
	for _, raw := range arrayValue(result["drives"]) {
		if drive := mapValue(raw); stringValue(drive["key"]) == "social" {
			social = drive
		}
	}
	if pressure := numberOrZero(social["pressure"]); pressure <= 0 || pressure > 1 {
		t.Fatalf("Core did not compute bounded drive pressure: %#v", social)
	}
	if requested["drive.social.pressure"] == nil || applied["drive.social.pressure"] == nil {
		t.Fatalf("drive transition audit missing: requested=%#v applied=%#v", requested, applied)
	}
	conflicts := arrayValue(result["conflicts"])
	if len(conflicts) != 1 || numberOrZero(mapValue(conflicts[0])["pressure"]) <= 0 {
		t.Fatalf("opposed semantic signals did not produce a Core-owned conflict: %#v", conflicts)
	}

	forged := cloneMap(decision)
	appraisal := mapValue(forged["appraisal"])
	forgedSignals := arrayValue(appraisal["drive_signals"])
	mapValue(forgedSignals[0])["raw_numeric_delta"] = 1.0
	appraisal["drive_signals"] = forgedSignals
	forged["appraisal"] = appraisal
	if err := validateFrozenDecisionInfluences(forged); err == nil {
		t.Fatal("Provider-owned raw drive delta was accepted")
	}
}

func TestDriveSemanticSignalSchemaIsClosedOnAppraisalSurfaces(t *testing.T) {
	for name, schema := range map[string]map[string]any{
		"conversation": cognitiveTurnResponseSchema(),
		"native":       nativeCognitionResponseSchema(),
	} {
		t.Run(name, func(t *testing.T) {
			appraisal := mapValue(mapValue(schema["properties"])["appraisal"])
			drives := mapValue(mapValue(appraisal["properties"])["drive_signals"])
			if stringValue(drives["type"]) != "array" || intValue(drives["maxItems"]) != maxDriveSemanticSignals {
				t.Fatalf("drive signal collection schema=%#v", drives)
			}
			item := mapValue(drives["items"])
			if additional, ok := item["additionalProperties"].(bool); !ok || additional {
				t.Fatalf("drive signal item must be closed: %#v", item)
			}
			for _, field := range []string{"ref", "direction", "strength", "confidence", "evidence_refs"} {
				if _, exists := mapValue(item["properties"])[field]; !exists || !containsSchemaRequired(item, field) {
					t.Fatalf("drive signal schema missing %q: %#v", field, item)
				}
			}
			if _, exists := mapValue(item["properties"])["raw_numeric_delta"]; exists {
				t.Fatal("Provider schema exposes a Core-owned raw numeric delta")
			}
		})
	}
}

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

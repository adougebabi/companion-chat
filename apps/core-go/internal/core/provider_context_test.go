package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCompactCognitionContextKeepsOnlyCanonicalLayersAndNonEmptyEvidence(t *testing.T) {
	projection := ContextProjection{
		SchemaVersion:          "fluctlight.context.v2",
		FluctlightID:           "fl-1",
		ConversationID:         "conversation-1",
		SourceFactID:           "fact-1",
		CurrentUserText:        "hello",
		ContextRevision:        4,
		CorePersonaRevision:    2,
		DevelopingSelfRevision: 3,
		CurrentStateRevision:   5,
		CorePersona: map[string]any{
			"authority": "hard_constraint",
			"data": map[string]any{
				"identity":          map[string]any{"name": "影者"},
				"personality":       map[string]any{"temperament": "冷静"},
				"behavioral_policy": map[string]any{"directness": 0.8},
			},
		},
		DevelopingSelf: []map[string]any{{"claim": "喜欢安静", "confidence": 0.6}},
		CurrentState: map[string]any{
			"authority": "transient_state",
			"data": map[string]any{
				"inner_state": map[string]any{
					"mood":            map[string]any{"label": "平静", "intensity": 0.4, "started_at": "db-only", "expected_decay_at": "db-only"},
					"pad":             map[string]any{"arousal": 0.1, "pleasure": 0.2, "dominance": 0.3},
					"regulation":      map[string]any{"stress": 0.1, "stability": 0.9, "natural_decay_rate": 0.25},
					"revision":        9,
					"last_updated_at": "db-only",
				},
				"life_context": map[string]any{"scene": "图书馆"},
			},
		},
		// These fields intentionally duplicate the canonical layer data. They must
		// not appear in the Provider-facing DTO.
		Identity:           map[string]any{"name": "影者"},
		Personality:        map[string]any{"temperament": "冷静"},
		BehavioralPolicy:   map[string]any{"directness": 0.8},
		InnerState:         map[string]any{"mood": map[string]any{"intensity": 0.4}},
		LifeContext:        map[string]any{"scene": "图书馆"},
		Capabilities:       []map[string]any{{"name": "memory_event"}},
		RecentMessages:     nil,
		Memories:           nil,
		Relationships:      nil,
		Hypotheses:         nil,
		DriveSlots:         nil,
		PreferenceSlots:    nil,
		TriggerPreferences: nil,
	}

	compact := compactCognitionContext(projection)
	for _, key := range []string{"identity", "personality", "behavioral_policy", "inner_state", "life_context", "capabilities", "recent_messages", "memories", "relationships", "hypotheses", "drive_slots", "preference_slots", "trigger_preferences"} {
		if _, ok := compact[key]; ok {
			t.Fatalf("compact context contains duplicate/empty field %q: %#v", key, compact)
		}
	}
	for _, key := range []string{"core_persona", "developing_self", "current_state"} {
		if _, ok := compact[key]; !ok {
			t.Fatalf("compact context is missing canonical field %q: %#v", key, compact)
		}
	}
	for _, key := range []string{"fluctlight_id", "conversation_id", "source_fact_id", "context_revision", "core_persona_revision", "developing_self_revision", "current_state_revision"} {
		if _, ok := compact[key]; ok {
			t.Fatalf("database/coordination field leaked into compact context: %q", key)
		}
	}
	state := mapValue(compact["current_state"])
	if mapValue(compact["core_persona"])["authority"] != "hard_constraint" || state["authority"] != "transient_state" {
		t.Fatalf("semantic authority labels were removed: %#v", compact)
	}
	if _, ok := mapValue(state["data"])["inner_state"]; !ok {
		t.Fatalf("current state lost inner_state: %#v", state)
	}
	if _, ok := mapValue(state["data"])["life_context"]; !ok {
		t.Fatalf("current state lost life_context: %#v", state)
	}
	inner := mapValue(mapValue(state["data"])["inner_state"])
	for _, key := range []string{"revision", "last_updated_at"} {
		if _, ok := inner[key]; ok {
			t.Fatalf("database inner-state field leaked: %q: %#v", key, inner)
		}
	}
	if _, ok := mapValue(inner["mood"])["started_at"]; ok {
		t.Fatal("mood persistence timestamp leaked into compact state")
	}
	if _, ok := mapValue(inner["regulation"])["natural_decay_rate"]; ok {
		t.Fatal("regulation control parameter leaked into compact state")
	}
	if _, ok := mapValue(mapValue(compact["core_persona"])["data"])["identity"].(map[string]any)["id"]; ok {
		t.Fatal("database identity id leaked into compact core persona")
	}
	encoded, err := json.Marshal(compact)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "schema_version") {
		t.Fatal("schema_version leaked into provider context")
	}
	if !strings.Contains(string(encoded), "hard_constraint") {
		t.Fatal("semantic authority label was lost")
	}
	if strings.Contains(string(encoded), "memory_event") {
		t.Fatal("native capability manifest leaked into compact user context")
	}
}

func TestCompactCognitionContextRetainsNonEmptySemanticCollections(t *testing.T) {
	projection := ContextProjection{
		SchemaVersion:      "fluctlight.context.v2",
		CorePersona:        map[string]any{"authority": "hard_constraint"},
		CurrentState:       map[string]any{"authority": "transient_state", "data": map[string]any{}},
		RecentMessages:     []map[string]any{{"id": "message-1", "kind": "user", "text": "hello", "created_at": "2026-09-03T00:00:00Z"}},
		Memories:           []map[string]any{{"id": "memory-1", "content": "fact"}},
		Relationships:      []map[string]any{{"actor_id": "actor-1", "status": "active", "summary": "协作"}},
		Hypotheses:         []map[string]any{{"content": "hypothesis"}},
		DriveSlots:         []map[string]any{{"key": "focus"}},
		PreferenceSlots:    []map[string]any{{"key": "quiet"}},
		TriggerPreferences: []map[string]any{{"key": "morning"}},
	}
	compact := compactCognitionContext(projection)
	for _, key := range []string{"memories", "relationships", "hypotheses", "drive_slots", "preference_slots", "trigger_preferences"} {
		if len(arrayValue(compact[key])) != 1 {
			t.Fatalf("non-empty collection %q was not retained: %#v", key, compact)
		}
	}
	if !strings.Contains(stringValue(compact["recent_messages"]), "[09-03 00:00:00] user: hello") {
		t.Fatalf("compact recent messages = %#v", compact["recent_messages"])
	}
}

func TestCompactCognitionContextRemovesDatabaseMetadataFromEvidence(t *testing.T) {
	compact := compactCognitionContext(ContextProjection{
		SchemaVersion: "fluctlight.context.v2",
		CorePersona:   map[string]any{"authority": "hard_constraint", "data": map[string]any{}},
		CurrentState:  map[string]any{"authority": "transient_state", "data": map[string]any{}},
		RecentMessages: []map[string]any{{
			"id": "message-db-id", "sequence": 3, "author_actor_id": "actor-db-id", "kind": "user",
			"text": "hello", "attachment_refs": []any{}, "created_at": "2026-09-03T00:00:00Z", "source": "message:message-db-id",
		}},
		Memories: []map[string]any{{
			"id": "memory-db-id", "type": "episodic", "content": "fact", "confidence": 0.9,
			"importance": 0.4, "emotional_significance": 0.2, "created_at": "2026-09-03T00:00:00Z",
			"evidence_refs": []any{"fact-1"}, "source": "memory:memory-db-id", "status": "active", "revision": 2,
			"visibility": "private", "conversation_id": "conversation-db-id", "event_refs": []any{"event-1"},
		}},
		DevelopingSelf: []map[string]any{{
			"id": "claim-db-id", "category": "preference", "claim": "喜欢安静", "value": "quiet",
			"confidence": 0.6, "evidence_refs": []any{"fact-1"}, "provenance": map[string]any{"source": "owner_defined"},
			"status": "uncertain", "revision": 4, "updated_at": "2026-09-03T00:00:00Z", "fluctlight_id": "fl-db-id",
		}},
	})
	message := stringValue(compact["recent_messages"])
	if !strings.Contains(message, "[09-03 00:00:00] user: hello") {
		t.Fatalf("message semantics changed: %#v", message)
	}
	memory := mapValue(arrayValue(compact["memories"])[0])
	for _, key := range []string{"id", "source", "status", "revision", "visibility", "conversation_id", "event_refs"} {
		if _, ok := memory[key]; ok {
			t.Fatalf("memory database field leaked: %q: %#v", key, memory)
		}
	}
	for _, key := range []string{"type", "content", "confidence", "importance", "emotional_significance"} {
		if _, ok := memory[key]; !ok {
			t.Fatalf("memory semantic field missing: %q: %#v", key, memory)
		}
	}
	claim := mapValue(arrayValue(compact["developing_self"])[0])
	for _, key := range []string{"id", "revision", "updated_at", "fluctlight_id"} {
		if _, ok := claim[key]; ok {
			t.Fatalf("Developing Self database field leaked: %q: %#v", key, claim)
		}
	}
}

func TestCompactCognitionContextRebuildsMissingCorePersonaEnvelope(t *testing.T) {
	compact := compactCognitionContext(ContextProjection{
		SchemaVersion:    "fluctlight.context.v2",
		Identity:         map[string]any{"name": "影者"},
		Personality:      map[string]any{"temperament": "冷静"},
		BehavioralPolicy: map[string]any{"directness": 0.8},
		CurrentState:     map[string]any{},
	})
	core := mapValue(compact["core_persona"])
	if core["authority"] != "hard_constraint" {
		t.Fatalf("core persona authority was removed: %#v", core["authority"])
	}
	data := mapValue(core["data"])
	for _, key := range []string{"identity", "personality", "behavioral_policy"} {
		if _, ok := data[key]; !ok {
			t.Fatalf("legacy core persona reconstruction missing %q: %#v", key, data)
		}
	}
	for _, key := range []string{"identity", "personality", "behavioral_policy"} {
		if _, ok := compact[key]; ok {
			t.Fatalf("reconstructed field was duplicated at top level: %q", key)
		}
	}
}

func TestCompactMessageTimeKeepsDateAndSecondsWithoutSequence(t *testing.T) {
	if got := compactMessageTime("2026-09-03T05:27:14.105684Z"); got != "09-03 05:27:14" {
		t.Fatalf("compact message time = %q", got)
	}
}

func TestCompactCognitionContextKeepsSemanticCurrentTimeAndTimezone(t *testing.T) {
	compact := compactCognitionContext(ContextProjection{
		CorePersona: map[string]any{"authority": "hard_constraint", "data": map[string]any{}},
		CurrentState: map[string]any{"authority": "transient_state", "data": map[string]any{
			"life_context": map[string]any{"instant": "2026-09-05T01:30:00Z", "current_time": "2026-09-05 09:30:00 CST", "timezone": "Asia/Shanghai", "scene": "书房"},
		}},
	})
	life := mapValue(mapValue(mapValue(compact["current_state"])["data"])["life_context"])
	if life["current_time"] != "2026-09-05 09:30:00 CST" || life["timezone"] != "Asia/Shanghai" || life["scene"] != "书房" {
		t.Fatalf("semantic current time context = %#v", life)
	}
	if _, ok := life["instant"]; ok {
		t.Fatal("raw instant metadata leaked alongside current_time")
	}
}

func TestCompactCognitionContextOmitsVisualIdentityWorkflowTimeline(t *testing.T) {
	compact := compactCognitionContext(ContextProjection{
		CorePersona:  map[string]any{"authority": "hard_constraint", "data": map[string]any{}},
		CurrentState: map[string]any{"authority": "transient_state", "data": map[string]any{}},
		VisualIdentity: map[string]any{
			"status":               "active",
			"identity_snapshot":    map[string]any{"identity": map[string]any{"name": "影者"}},
			"renderer_constraints": map[string]any{"schema_version": "visual-identity.v1", "chest_cup": "B", "chest_lora_weight": -3.0, "adapter_version": "chest-cup-adapter.v1"},
			"canonical_asset_id":   "asset-character-sheet",
			"active_session_id":    "visual_identity_session-1",
			"timeline":             []map[string]any{{"stage": "seed_requested", "stage_order": 20, "summary": "正在生成", "asset_ids": []any{"asset-1"}}},
		},
	})
	visual := mapValue(compact["visual_identity"])
	if visual["available"] != true {
		t.Fatalf("visual identity availability = %#v", visual["available"])
	}
	if _, ok := visual["timeline"]; ok {
		t.Fatal("visual identity workflow timeline leaked into cognition context")
	}
	if _, ok := visual["identity_snapshot"]; ok {
		t.Fatal("full visual identity snapshot leaked into cognition context")
	}
	constraints := mapValue(visual["renderer_constraints"])
	if constraints["chest_cup"] != "B" || constraints["chest_lora_weight"] != -3.0 {
		t.Fatalf("renderer constraints = %#v", constraints)
	}
	if _, ok := constraints["adapter_version"]; ok {
		t.Fatal("renderer adapter metadata leaked into cognition context")
	}
}

func TestCompactCognitionContextDoesNotRepeatCurrentUserMessage(t *testing.T) {
	compact := compactCognitionContext(ContextProjection{
		CurrentUserText: "最新消息",
		RecentMessages: []map[string]any{
			{"kind": "assistant", "text": "上一句", "created_at": "2026-09-05T01:29:00Z"},
			{"kind": "user", "text": "最新消息", "created_at": "2026-09-05T01:30:00Z"},
		},
	})
	if _, ok := compact["current_user_text"]; ok {
		t.Fatal("current user text was retained as a second context field")
	}
	recent := stringValue(compact["recent_messages"])
	if strings.Contains(recent, "最新消息") || !strings.Contains(recent, "上一句") {
		t.Fatalf("recent messages = %q", recent)
	}
}

func TestCompactResponsePlanForProviderRemovesProtocolFields(t *testing.T) {
	compact := compactResponsePlanForProvider(map[string]any{
		"schema_version": "fluctlight.response-plan.v1", "source_fact_id": "fact-1", "context_revision": 3,
		"answer_mode": "direct", "response_intent": "简短回复", "tone": "自然",
		"approved_claims":  []any{map[string]any{"kind": "observed_fact", "content": "已确认", "confidence": 0.9, "evidence_refs": []any{"fact-1"}}},
		"response_outline": []any{"先回应", "再说明"},
		"self_evaluation":  map[string]any{"mode": "accepted", "confidence": 0.8, "reason_codes": []any{"internal"}},
		"tool_calls":       []any{map[string]any{"id": "call-1", "name": "scene_event"}},
		"composite_action": map[string]any{"schema_version": "internal"},
	})
	for _, key := range []string{"schema_version", "source_fact_id", "context_revision", "tool_calls", "composite_action", "visible_text"} {
		if _, ok := compact[key]; ok {
			t.Fatalf("protocol field %q leaked: %#v", key, compact)
		}
	}
	claim := mapValue(arrayValue(compact["approved_claims"])[0])
	if claim["content"] != "已确认" || claim["confidence"] != 0.9 {
		t.Fatalf("compact claim = %#v", claim)
	}
	if _, ok := claim["evidence_refs"]; ok {
		t.Fatal("claim evidence refs leaked into realization prompt")
	}
}

func TestCompactToolResultsForProviderKeepsOnlyOutcome(t *testing.T) {
	compact := compactToolResultsForProvider([]ToolResultV1{{
		ToolCallID: "call-1", Name: "scene_event", Status: "completed", Output: map[string]any{"event_id": "event-1", "summary": "在书房"},
		ErrorCode: "", Retryable: false, ProviderRequestID: "provider-1", CorrelationID: "corr-1", SchemaVersion: ToolResultSchemaVersion,
	}})
	if len(compact) != 1 {
		t.Fatalf("compact tool results = %#v", compact)
	}
	item := compact[0]
	if item["name"] != "scene_event" || item["status"] != "completed" || stringValue(mapValue(item["output"])["summary"]) != "在书房" {
		t.Fatalf("compact tool result = %#v", item)
	}
	for _, key := range []string{"tool_call_id", "provider_request_id", "correlation_id", "schema_version", "retryable"} {
		if _, ok := item[key]; ok {
			t.Fatalf("tool protocol field %q leaked: %#v", key, item)
		}
	}
}

func TestCompactReflectionEvidenceUsesShortSequenceReferences(t *testing.T) {
	compact := compactReflectionEvidence([]map[string]any{{
		"id": "fact-long-id", "sequence": 7, "event_type": "conversation.turn",
		"payload": map[string]any{"turn_id": "turn-long-id", "text": "你好", "status": "processed", "summary": "有效事实"},
	}})
	if len(compact) != 1 || compact[0]["event_type"] != "conversation.turn" || compact[0]["evidence_ref"] != "sequence:7" {
		t.Fatalf("compact reflection evidence = %#v", compact)
	}
	payload := mapValue(compact[0]["payload"])
	if payload["summary"] != "有效事实" {
		t.Fatalf("compact reflection payload = %#v", payload)
	}
	for _, key := range []string{"id", "turn_id", "status"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("reflection payload field %q leaked: %#v", key, payload)
		}
	}
}

func TestCompactProviderFactRemovesTransportMetadata(t *testing.T) {
	fact := compactProviderFact([]byte(`{"source_fact_id":"fact-1","event_type":"presence.updated","payload":{"presence_id":"presence-1","current_task":"阅读","status":"active"}}`))
	encoded := string(jsonString(fact))
	if strings.Contains(encoded, "source_fact_id") || strings.Contains(encoded, "presence_id") || strings.Contains(encoded, "status") {
		t.Fatalf("provider fact metadata leaked: %s", encoded)
	}
	if !strings.Contains(encoded, "current_task") {
		t.Fatalf("provider fact lost semantic field: %s", encoded)
	}
}

func TestCompactCognitionContextIncludesGoalsAndIntentionsAsSemanticInputs(t *testing.T) {
	compact := compactCognitionContext(ContextProjection{
		CorePersona:  map[string]any{"authority": "hard_constraint", "data": map[string]any{}},
		CurrentState: map[string]any{"authority": "transient_state", "data": map[string]any{}},
		Goals: []map[string]any{{
			"id": "goal_1234567890abcdef", "description": "完成当前项目", "status": "active",
			"importance": 0.9, "urgency": 0.7, "progress": 0.2,
		}},
		Intentions: []map[string]any{{
			"id": "intention_1234567890abcdef", "goal_id": "goal_1234567890abcdef", "goal": "完成当前项目",
			"action": "检查待处理任务", "status": "pending", "confidence": 0.8, "expiration": "2026-09-05T00:00:00Z",
		}},
	})
	goals := arrayValue(compact["goals"])
	if len(goals) != 1 || stringValue(mapValue(goals[0])["description"]) != "完成当前项目" || stringValue(mapValue(goals[0])["state"]) != "active" {
		t.Fatalf("compact goals = %#v", compact["goals"])
	}
	intentions := arrayValue(compact["intentions"])
	if len(intentions) != 1 || stringValue(mapValue(intentions[0])["goal"]) != "完成当前项目" || stringValue(mapValue(intentions[0])["state"]) != "pending" || stringValue(mapValue(intentions[0])["deadline"]) == "" {
		t.Fatalf("compact intentions = %#v", compact["intentions"])
	}
	encoded, _ := json.Marshal(compact)
	for _, leaked := range []string{"goal_1234567890abcdef", "intention_1234567890abcdef", "goal_id", "status"} {
		if strings.Contains(string(encoded), leaked) {
			t.Fatalf("agency metadata leaked: %q in %s", leaked, encoded)
		}
	}
}

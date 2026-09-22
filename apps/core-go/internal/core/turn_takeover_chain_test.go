package core

import (
	"context"
	"strings"
	"testing"
)

const (
	takeoverChainSparkMarker    = "SPARK_SCOPE_MARKER"
	takeoverChainTwilightMarker = "TWILIGHT_SCOPE_MARKER"
)

func takeoverChainDefaultRules() []any {
	return []any{map[string]any{
		"id": "public-doubt", "kind": "turn_takeover", "version": personaTakeoverRuleVersion,
		"condition": "用户在强烈质疑本轮回复时由暮光接管", "target_profile_id": "twilight",
		"source_profile_id": "spark", "enabled": true,
	}}
}

func takeoverChainSeed(t *testing.T, ctx context.Context, repository *PostgresRepository, ownerID, fluctlightID, conversationID string) {
	t.Helper()
	takeoverChainSeedWithTakeoverRules(t, ctx, repository, ownerID, fluctlightID, conversationID, takeoverChainDefaultRules())
}

func takeoverChainSeedWithTakeoverRules(t *testing.T, ctx context.Context, repository *PostgresRepository, ownerID, fluctlightID, conversationID string, takeoverRules []any) {
	t.Helper()
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	personalitySystem := map[string]any{
		"mode": "multiple", "active_profile_id": "spark",
		"profiles": []any{
			map[string]any{"id": "spark", "name": "星火", "personality": map[string]any{"openness": 0.8, "signature_phrase": takeoverChainSparkMarker}, "behavioral_policy": map[string]any{"directness": 0.9}},
			map[string]any{"id": "twilight", "name": "暮光", "personality": map[string]any{"openness": 0.3, "signature_phrase": takeoverChainTwilightMarker}, "behavioral_policy": map[string]any{"gentleness": 0.9}},
		},
		"switching": map[string]any{"cooldown_seconds": 0, "rules": []any{
			map[string]any{"id": "safety", "condition": "收到明确的安全确认后由暮光主导", "target_profile_id": "twilight"},
		}},
	}
	if len(takeoverRules) > 0 {
		personalitySystem["takeover_rules"] = takeoverRules
	}
	corePersona := map[string]any{
		"identity": map[string]any{"name": "摇光"}, "life_profile": map[string]any{"city": "上海"},
		"personality_system": personalitySystem,
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active',$3,'{"timezone":"Asia/Shanghai"}','{}','{}','{}','{}')`, fluctlightID, ownerID, jsonString(corePersona)); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_inner_states(fluctlight_id,revision,pad,mood,momentum,regulation,drives,conflicts,last_updated_at) VALUES($1,0,'{"pleasure":0,"arousal":0,"dominance":0}','{"label":"neutral","intensity":0}','{"value":0,"trend":0}','{"stress":0,"stability":0}','[]','[]',now())`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_affect_profiles(fluctlight_id) VALUES($1)`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_personality_runtime(fluctlight_id,active_profile_id,revision) VALUES($1,'spark',0)`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversations(id,created_by_actor_id,title) VALUES($1,$2,'native persona tools')`, conversationID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_heads(conversation_id,next_sequence) VALUES($1,1)`, conversationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,'owner','active'),($1,$3,'member','active')`, conversationID, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	seedCognitiveProviderRole(t, ctx, repository, "native-persona-endpoint-"+fluctlightID)
}

func takeoverChainTurnPayload(fluctlightID, text, idempotencyKey, turnID string) map[string]any {
	return map[string]any{"fluctlight_id": fluctlightID, "text": text, "idempotency_key": idempotencyKey, "turn_id": turnID, "attachment_refs": []any{}}
}

func takeoverChainAssistantTexts(t *testing.T, ctx context.Context, repository *PostgresRepository, conversationID, turnID string) []string {
	t.Helper()
	rows, err := repository.Pool().Query(ctx, `SELECT text FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND turn_id=$2 ORDER BY sequence`, conversationID, turnID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			t.Fatal(err)
		}
		result = append(result, text)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func takeoverChainCount(t *testing.T, ctx context.Context, repository *PostgresRepository, query string, arguments ...any) int {
	t.Helper()
	var count int
	if err := repository.Pool().QueryRow(ctx, query, arguments...).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func nativePersonaToolCall(id, name string, arguments map[string]any) map[string]any {
	return map[string]any{"id": id, "type": "function", "function": map[string]any{"name": name, "arguments": jsonString(arguments)}}
}

func nativePersonaFinal() fakeProviderResult {
	return fakeProviderResult{Structured: map[string]any{
		"action_type": "reply", "response_intent": "finish after committed tools", "influences": []any{},
		"appraisal": map[string]any{
			"relevance": 0.5, "goal_congruence": 0.5, "reward": 0.5, "loss": 0.0,
			"social_threat": 0.0, "controllability": 0.8, "responsibility": 0.5,
			"relationship_significance": 0.7, "expected_effect": 0.7,
			"evidence_refs": []any{}, "event_kind": "conversation", "direction": "mixed", "drive_signals": []any{},
		},
	}}
}

func nativePersonaToolMessages(payload map[string]any) string {
	var toolMessages []any
	for _, raw := range arrayValue(payload["messages"]) {
		message := mapValue(raw)
		if stringValue(message["role"]) == "tool" {
			toolMessages = append(toolMessages, message)
		}
	}
	return jsonString(toolMessages)
}

func nativePersonaTrace(t *testing.T, ctx context.Context, repository *PostgresRepository, idempotencyKey string) []map[string]any {
	t.Helper()
	var raw []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT payload->'agent_result'->'capability_results' FROM public.cognition_inbox WHERE idempotency_key=$1 AND event_type='conversation.turn'`, idempotencyKey).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	items := arrayValue(decodeJSONValue(raw))
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, mapValue(item))
	}
	return result
}

func nativePersonaResultByName(t *testing.T, results []map[string]any, name string) map[string]any {
	t.Helper()
	for _, result := range results {
		if stringValue(result["capability_name"]) == name || stringValue(result["CapabilityName"]) == name {
			return result
		}
	}
	t.Fatalf("missing capability result %q in %#v", name, results)
	return nil
}

func TestNativePersonaTakeoverFeedsScopedToolsReplyAndFinal(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "native-chain-owner", "native-chain-fluctlight", "native-chain-conversation"
	takeoverScopeMatrixSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)

	step := 0
	var scriptFailure string
	router := newFakeProviderRouter().on(workingPersonaMainTurnSchema, func(payload map[string]any) fakeProviderResult {
		step++
		switch step {
		case 1:
			wire := jsonString(payload)
			if !strings.Contains(wire, personaTakeoverCapabilityName) {
				scriptFailure = "persona.takeover missing from first model request"
				return fakeProviderResult{Status: 500}
			}
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("native-takeover", personaTakeoverCapabilityName, map[string]any{
				"decision": takeoverDecisionTakeoverB, "rule_id": "public-doubt", "source_profile_id": "spark", "target_profile_id": "twilight", "reason": "controlled native handover",
			})}}
		case 2:
			tools := nativePersonaToolMessages(payload)
			if !strings.Contains(tools, personaTakeoverCapabilityName) || !strings.Contains(tools, "working_persona") || !strings.Contains(tools, "twilight") {
				t.Fatalf("takeover receipt was not fed to the next model decision: %s", tools)
			}
			return fakeProviderResult{ToolCalls: []map[string]any{
				nativePersonaToolCall("native-relationship", "relationship.lookup", map[string]any{"target_actor_id": ownerID}),
				nativePersonaToolCall("native-memory", "memory.recall", map[string]any{"intent": scopeMatrixMemoryContent}),
			}}
		case 3:
			tools := nativePersonaToolMessages(payload)
			if !strings.Contains(tools, scopeMatrixTwilightRelationshipSummary) || strings.Contains(tools, scopeMatrixSparkRelationshipSummary) {
				t.Fatalf("relationship lookup did not use the takeover working profile: %s", tools)
			}
			if !strings.Contains(tools, scopeMatrixMemoryContent) {
				t.Fatalf("memory recall result was not fed to the model: %s", tools)
			}
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("native-reply", "conversation.reply", map[string]any{"text": "暮光按自己的关系与记忆作答"})}}
		case 4:
			if tools := nativePersonaToolMessages(payload); !strings.Contains(tools, "conversation_message") {
				t.Fatalf("committed reply result missing from final model input: %s", tools)
			}
			return nativePersonaFinal()
		default:
			t.Fatalf("unexpected extra model decision %d", step)
			return fakeProviderResult{Status: 500}
		}
	})
	app := newTestApp(t, repository, router)
	result, err := app.HandleTurn(ctx, ownerID, conversationID, takeoverChainTurnPayload(fluctlightID, "你根本没在听。", "native-chain-turn", "native-chain-turn-1"))
	if err != nil {
		t.Fatal(err)
	}
	if scriptFailure != "" {
		t.Fatal(scriptFailure)
	}
	if stringValue(result.Assistant["text"]) != "暮光按自己的关系与记忆作答" || step != 4 {
		t.Fatalf("result=%#v model decisions=%d", result, step)
	}
	if active := readActiveProfileForGate(t, ctx, repository, fluctlightID); active != "spark" {
		t.Fatalf("run-local takeover changed persistent profile to %q", active)
	}
	results := nativePersonaTrace(t, ctx, repository, "native-chain-turn")
	for _, name := range []string{personaTakeoverCapabilityName, "relationship.lookup", "memory.recall", "conversation.reply"} {
		entry := nativePersonaResultByName(t, results, name)
		acting := firstString(entry["acting_profile_id"], stringValue(entry["ActingProfileID"]))
		if name == personaTakeoverCapabilityName {
			if acting != "spark" {
				t.Fatalf("takeover action must be attributed to source profile, got %q", acting)
			}
		} else if acting != "twilight" {
			t.Fatalf("%s acting profile=%q, want twilight", name, acting)
		}
	}
	if got := scopeMatrixInteractionFrequency(t, ctx, repository, scopeMatrixTwilightRelationshipID); got != 1 {
		t.Fatalf("reply relationship settlement did not follow actual acting profile: %d", got)
	}
	if got := scopeMatrixInteractionFrequency(t, ctx, repository, scopeMatrixSparkRelationshipID); got != 0 {
		t.Fatalf("persistent profile received takeover interaction: %d", got)
	}
}

func TestRejectedNativePersonaTakeoverCannotEscalateWorkingProfile(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "native-reject-owner", "native-reject-fluctlight", "native-reject-conversation"
	takeoverScopeMatrixSeed(t, ctx, repository, ownerID, fluctlightID, conversationID)
	step := 0
	router := newFakeProviderRouter().on(workingPersonaMainTurnSchema, func(payload map[string]any) fakeProviderResult {
		step++
		switch step {
		case 1:
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("rejected-takeover", personaTakeoverCapabilityName, map[string]any{
				"decision": takeoverDecisionTakeoverB, "rule_id": "public-doubt", "source_profile_id": "spark", "target_profile_id": "undeclared",
			})}}
		case 2:
			tools := nativePersonaToolMessages(payload)
			if !strings.Contains(tools, "status") || !strings.Contains(tools, "rejected") || !strings.Contains(tools, "persona_takeover_target_mismatch") {
				t.Fatalf("business rejection missing from next model input: %s", tools)
			}
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("source-relationship", "relationship.lookup", map[string]any{"target_actor_id": ownerID})}}
		case 3:
			tools := nativePersonaToolMessages(payload)
			if !strings.Contains(tools, scopeMatrixSparkRelationshipSummary) || strings.Contains(tools, scopeMatrixTwilightRelationshipSummary) {
				t.Fatalf("rejected takeover widened relationship scope: %s", tools)
			}
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("source-reply", "conversation.reply", map[string]any{"text": "星火继续本轮回复"})}}
		case 4:
			return nativePersonaFinal()
		default:
			t.Fatalf("unexpected extra model decision %d", step)
			return fakeProviderResult{Status: 500}
		}
	})
	app := newTestApp(t, repository, router)
	result, err := app.HandleTurn(ctx, ownerID, conversationID, takeoverChainTurnPayload(fluctlightID, "继续。", "native-reject-turn", "native-reject-turn-1"))
	if err != nil {
		t.Fatal(err)
	}
	if stringValue(result.Assistant["text"]) != "星火继续本轮回复" || readActiveProfileForGate(t, ctx, repository, fluctlightID) != "spark" {
		t.Fatalf("rejected takeover changed reply or durable profile: %#v", result)
	}
	results := nativePersonaTrace(t, ctx, repository, "native-reject-turn")
	rejected := nativePersonaResultByName(t, results, personaTakeoverCapabilityName)
	if firstString(rejected["status"], stringValue(rejected["Status"])) != "rejected" {
		t.Fatalf("takeover rejection was not retained: %#v", rejected)
	}
	lookup := nativePersonaResultByName(t, results, "relationship.lookup")
	if acting := firstString(lookup["acting_profile_id"], stringValue(lookup["ActingProfileID"])); acting != "spark" {
		t.Fatalf("post-rejection lookup acting profile=%q", acting)
	}
}

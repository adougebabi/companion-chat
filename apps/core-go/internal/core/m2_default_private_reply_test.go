package core

import (
	"strings"
	"testing"
)

// The default route is observed at the formal HandleTurn boundary. The model
// identifiers are deliberately arbitrary: selection must come from settings.
func TestDefaultPrivateReplyUsesConfiguredModelWithoutWriter(t *testing.T) {
	var models []string
	var directToolOffered bool
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		models = append(models, stringValue(payload["model"]))
		for _, raw := range arrayValue(payload["tools"]) {
			function := mapValue(mapValue(raw)["function"])
			if strings.ReplaceAll(stringValue(function["name"]), ".", "_") == "conversation_reply" {
				parameters := mapValue(function["parameters"])
				_, directToolOffered = mapValue(parameters["properties"])["decision"]
			}
		}
		if !directToolOffered {
			return fakeProviderResult{Status: 400}
		}
		decision := map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("default-direct-reply", conversationReplyCapabilityName, map[string]any{"text": "你好，今天还不错。", "decision": decision})}}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "default-private-main", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.model_roles SET model_id=CASE role WHEN 'conversation_reply' THEN 'selected-default-model' ELSE 'different-old-model' END WHERE role IN ('conversation_reply','cognitive_assessment')`); err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{"fluctlight_id": fid, "text": "你好", "turn_id": "default-private-main", "idempotency_key": "default-private-main", "attachment_refs": []any{}}
	result, err := app.HandleTurn(ctx, owner, conversation, payload)
	if err != nil || !directToolOffered || len(models) != 1 || models[0] != "selected-default-model" || stringValue(result.Assistant["text"]) != "你好，今天还不错。" {
		t.Fatalf("default direct reply: models=%v direct_tool=%t result=%#v err=%v", models, directToolOffered, result.Assistant, err)
	}
	var writerRuns, assessments, messages int
	if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.diagnostic_model_runs WHERE fluctlight_id=$1 AND role='conversation_reply'), (SELECT count(*) FROM public.cognition_assessments WHERE fluctlight_id=$1), (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$2 AND kind='assistant')`, fid, conversation).Scan(&writerRuns, &assessments, &messages); err != nil || writerRuns != 0 || assessments != 1 || messages != 1 {
		t.Fatalf("writer=%d assessments=%d messages=%d err=%v", writerRuns, assessments, messages, err)
	}
	replayed, err := app.HandleTurn(ctx, owner, conversation, payload)
	if err != nil || stringValue(replayed.Assistant["text"]) != "你好，今天还不错。" || len(models) != 1 {
		t.Fatalf("replay duplicated the model request: models=%v result=%#v err=%v", models, replayed.Assistant, err)
	}
	role := agentModelBindingKey(FormalAgentConversationCognition, "conversation_turn_response")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,timeout_seconds) VALUES($1,'mixed-endpoint-default-private-main','explicit-variant-model',30)`, role); err != nil {
		t.Fatal(err)
	}
	next, err := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "再聊一句", "turn_id": "default-private-next", "idempotency_key": "default-private-next", "attachment_refs": []any{}})
	if err != nil || len(models) != 2 || models[1] != "explicit-variant-model" || stringValue(next.Assistant["text"]) != "你好，今天还不错。" {
		t.Fatalf("explicit variant override was not respected: models=%v result=%#v err=%v", models, next.Assistant, err)
	}
}

// The existing setting remains a real switch. Changing it uses the formal
// configuration endpoint and affects new turns without rewriting other fields.
func TestPrivateReplyModeSwitchKeepsExplicitFactOnly(t *testing.T) {
	var requests []string
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		model := stringValue(payload["model"])
		requests = append(requests, model)
		if model == "explicit-writer" {
			return fakeProviderResult{Text: "这是独立回复。"}
		}
		for _, raw := range arrayValue(payload["tools"]) {
			function := mapValue(mapValue(raw)["function"])
			if strings.ReplaceAll(stringValue(function["name"]), ".", "_") != "conversation_reply" {
				continue
			}
			properties := mapValue(mapValue(function["parameters"])["properties"])
			if _, direct := properties["decision"]; direct {
				decision := map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}
				return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("switch-direct", conversationReplyCapabilityName, map[string]any{"text": "这是直接回复。", "decision": decision})}}
			}
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("switch-fact", conversationReplyCapabilityName, map[string]any{})}}
		}
		return fakeProviderResult{Status: 400}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "mode-switch", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	writerRole := agentModelBindingKey(FormalAgentConversationReply, "private_reply")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,timeout_seconds) VALUES($1,'mixed-endpoint-mode-switch','explicit-writer',30)`, writerRole); err != nil {
		t.Fatal(err)
	}
	setMode := func(mode, key string) {
		t.Helper()
		current, err := app.ReadSettings(ctx, owner)
		if err != nil {
			t.Fatal(err)
		}
		version := mapValue(current["versions"])[conversationReplySettingKey]
		if version == nil {
			version = 0
		}
		_, err = app.UpdateSettings(ctx, owner, map[string]any{"values": map[string]any{conversationReplySettingKey: map[string]any{"mode": mode, "temperature": 0.2}}, "expected_versions": map[string]any{conversationReplySettingKey: version}, "idempotency_key": key})
		if err != nil {
			t.Fatal(err)
		}
		current, err = app.ReadSettings(ctx, owner)
		settings := mapValue(mapValue(current["values"])[conversationReplySettingKey])
		if err != nil || stringValue(settings["mode"]) != mode || settings["temperature"] != 0.2 {
			t.Fatalf("saved settings changed: %#v err=%v", settings, err)
		}
	}
	setMode("fact_only", "select-fact-only")
	fact, err := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "第一条", "turn_id": "switch-fact", "idempotency_key": "switch-fact", "attachment_refs": []any{}})
	if err != nil || stringValue(fact.Assistant["text"]) != "这是独立回复。" || len(requests) != 2 || requests[1] != "explicit-writer" {
		t.Fatalf("fact_only was not retained: requests=%v result=%#v err=%v", requests, fact.Assistant, err)
	}
	setMode("direct", "select-direct")
	direct, err := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "第二条", "turn_id": "switch-direct", "idempotency_key": "switch-direct", "attachment_refs": []any{}})
	if err != nil || stringValue(direct.Assistant["text"]) != "这是直接回复。" || len(requests) != 3 || requests[2] == "explicit-writer" {
		t.Fatalf("direct did not take effect: requests=%v result=%#v err=%v", requests, direct.Assistant, err)
	}
}

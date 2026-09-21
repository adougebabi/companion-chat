package core

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStreamTurnFailureCodeDistinguishesMissingReplyToolAndCapabilityFailure(t *testing.T) {
	toolInvalid := newProviderToolCallNormalizationError(0, "id_required", errors.New(`tool call "model-id" id is required`))
	cases := []struct {
		name string
		err  error
		want string
	}{
		{name: "missing visible reply", err: errors.New("cognition_visible_text_missing"), want: "cognition_visible_text_missing"},
		{name: "invalid provider tool", err: toolInvalid, want: "tool_call_invalid"},
		{name: "media capability", err: newCapabilityError("media_intent_failed", true, errors.New("renderer unavailable")), want: "media_intent_failed"},
		{name: "unknown internal", err: errors.New("database connection detail"), want: "conversation_settlement_failed"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := streamTurnFailureCode(testCase.err); got != testCase.want {
				t.Fatalf("streamTurnFailureCode() = %q, want %q", got, testCase.want)
			}
		})
	}
}

// This is the production shape reported by the UI: the Provider emits native
// image and conversation.reply calls while the structured sidecar is the
// typed-empty fallback. Both deferred outputs must be frozen, settled against
// one assistant message, and leave durable media/workflow records.
func TestConversationTurnWithNativeImageAndReplyCommitsBothOutputs(t *testing.T) {
	ctx, repository, app, ownerID, fluctlightID, conversationID := setupMixedMediaReplyTurn(t, "native", fakeProviderResult{ToolCalls: mixedImageReplyToolCalls()})
	result, err := app.HandleTurn(ctx, ownerID, conversationID, map[string]any{
		"fluctlight_id": fluctlightID, "text": "给我看看", "idempotency_key": "mixed-turn-native", "turn_id": "mixed-turn-native-1", "attachment_refs": []any{},
	})
	if err != nil {
		t.Fatalf("mixed turn failed: %v", err)
	}
	if stringValue(result.Assistant["text"]) != "诶？真的要看啊。" {
		t.Fatalf("assistant result = %#v", result.Assistant)
	}
	assertMixedMediaReplyDurability(t, ctx, repository, fluctlightID, conversationID, "mixed-turn-native", "mixed-turn-native-1", result.Assistant)
}

func TestEinoADKStructuredContentToolCallsAreNotExecuted(t *testing.T) {
	structured := map[string]any{
		"response_mode":   "final",
		"action_type":     "reply",
		"response_intent": "同时发送图片和说明文字",
		"influences":      []any{},
		// Content.tool_calls is deliberately a pseudo request. Only the Eino
		// Message.ToolCalls channel may reach the capability runtime.
		"tool_calls": mixedImageReplyToolCalls(),
	}
	ctx, repository, app, ownerID, fluctlightID, conversationID := setupMixedMediaReplyTurn(t, "structured", fakeProviderResult{Structured: structured})
	result, err := app.HandleTurn(ctx, ownerID, conversationID, map[string]any{
		"fluctlight_id": fluctlightID, "text": "给我看看", "idempotency_key": "mixed-turn-structured", "turn_id": "mixed-turn-structured-1", "attachment_refs": []any{},
	})
	if err == nil || !strings.Contains(err.Error(), "cognition_visible_text_missing") {
		t.Fatalf("structured pseudo-tool turn did not fail clearly: result=%#v err=%v", result, err)
	}
	if len(result.Assistant) != 0 {
		t.Fatalf("structured pseudo-tool turn fabricated an assistant: %#v", result.Assistant)
	}
	var assistantCount, mediaCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversationID).Scan(&assistantCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.media_intents WHERE owner_fluctlight_id=$1`, fluctlightID).Scan(&mediaCount); err != nil {
		t.Fatal(err)
	}
	if assistantCount != 0 || mediaCount != 0 {
		t.Fatalf("structured pseudo-tool executed or published output: assistant=%d media=%d", assistantCount, mediaCount)
	}
}

func TestStreamTurnWithNativeImageAndReplyEmitsAssistantFrame(t *testing.T) {
	ctx, repository, app, ownerID, fluctlightID, conversationID := setupMixedMediaReplyTurn(t, "stream", fakeProviderResult{ToolCalls: mixedImageReplyToolCalls()})
	response := httptest.NewRecorder()
	err := app.StreamTurn(ctx, response, ownerID, conversationID, map[string]any{
		"fluctlight_id": fluctlightID, "text": "给我看看", "idempotency_key": "mixed-turn-stream", "turn_id": "mixed-turn-stream-1", "attachment_refs": []any{},
	})
	if err != nil {
		t.Fatalf("stream mixed turn failed: %v; body=%s", err, response.Body.String())
	}
	var frames []map[string]any
	scanner := bufio.NewScanner(response.Body)
	for scanner.Scan() {
		var frame map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &frame); err != nil {
			t.Fatalf("decode stream frame: %v; line=%s", err, scanner.Text())
		}
		frames = append(frames, frame)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(frames) != 4 {
		t.Fatalf("stream frames = %#v", frames)
	}
	if stringValue(frames[0]["type"]) != "action_result" || stringValue(frames[1]["type"]) != "token" || stringValue(frames[2]["type"]) != "action_result" || stringValue(frames[3]["type"]) != "completed" {
		t.Fatalf("stream frame types = %#v", frames)
	}
	assistant := mapValue(mapValue(frames[2]["payload"])["message"])
	if stringValue(assistant["kind"]) != "assistant" || stringValue(assistant["text"]) != "诶？真的要看啊。" {
		t.Fatalf("assistant stream frame = %#v", frames[2])
	}
	assertMixedMediaReplyDurability(t, ctx, repository, fluctlightID, conversationID, "mixed-turn-stream", "mixed-turn-stream-1", assistant)
}

func setupMixedMediaReplyTurn(t *testing.T, suffix string, providerResult fakeProviderResult) (context.Context, *PostgresRepository, *App, string, string, string) {
	t.Helper()
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "mixed-owner-"+suffix, "mixed-fluctlight-"+suffix, "mixed-conversation-"+suffix
	seedTurnConversation(t, ctx, repository, ownerID, fluctlightID, conversationID)
	seedCognitiveProviderRole(t, ctx, repository, "mixed-endpoint")
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.fluctlights SET identity=$2 WHERE id=$1`, fluctlightID, jsonBytes(map[string]any{
		"timezone":   "Asia/Shanghai",
		"appearance": map[string]any{"hair": "black hair", "outfit": "coat"},
	})); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('media.comfyui',$1)`, jsonString(map[string]any{
		"baseUrl":  "http://comfy.invalid",
		"workflow": map[string]any{"prompt": "{{prompt}}"},
	})); err != nil {
		t.Fatal(err)
	}

	router := newFakeProviderRouter().on("conversation_turn_response", func(_ map[string]any) fakeProviderResult {
		return providerResult
	})
	app := newTestApp(t, repository, router)
	return ctx, repository, app, ownerID, fluctlightID, conversationID
}

func mixedImageReplyToolCalls() []map[string]any {
	return []map[string]any{
		{"id": "call_image", "type": "function", "function": map[string]any{
			"name": "media.image.generate", "arguments": jsonString(map[string]any{"intent": "a fantasy swordsman WIP"}),
		}},
		{"id": "call_reply", "type": "function", "function": map[string]any{
			"name": "conversation.reply", "arguments": jsonString(map[string]any{"text": "诶？真的要看啊。"}),
		}},
	}
}

func assertMixedMediaReplyDurability(t *testing.T, ctx context.Context, repository *PostgresRepository, fluctlightID, conversationID, idempotencyKey, turnID string, assistant map[string]any) {
	t.Helper()
	var assistantCount, mediaCount, workflowCount, outboxCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND text=$2`, conversationID, "诶？真的要看啊。").Scan(&assistantCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.media_intents WHERE owner_fluctlight_id=$1 AND message_id=(SELECT id FROM public.conversation_messages WHERE conversation_id=$2 AND kind='assistant' AND turn_id=$3)`, fluctlightID, conversationID, turnID).Scan(&mediaCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='media.generation' AND payload->>'fluctlight_id'=$1`, fluctlightID).Scan(&workflowCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_outbox_events WHERE kind='media.intent.created' AND aggregate_id IN (SELECT id FROM public.media_intents WHERE owner_fluctlight_id=$1)`, fluctlightID).Scan(&outboxCount); err != nil {
		t.Fatal(err)
	}
	if assistantCount != 1 || mediaCount != 1 || workflowCount != 1 || outboxCount != 1 {
		t.Fatalf("mixed output durability assistant=%d media=%d workflow=%d outbox=%d assistant=%#v", assistantCount, mediaCount, workflowCount, outboxCount, assistant)
	}
	var mediaStatus string
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.media_intents WHERE owner_fluctlight_id=$1`, fluctlightID).Scan(&mediaStatus); err != nil {
		t.Fatal(err)
	}
	if mediaStatus != "pending" {
		t.Fatalf("media status=%q, want pending durable intent", mediaStatus)
	}
	var frozenPayload []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT payload FROM public.cognition_frozen_actions WHERE inbox_id=(SELECT id FROM public.cognition_inbox WHERE fluctlight_id=$1 AND idempotency_key=$2)`, fluctlightID, idempotencyKey).Scan(&frozenPayload); err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(frozenPayload, &payload); err != nil {
		t.Fatal(err)
	}
	if len(arrayValue(payload["capability_invocations"])) != 2 || len(arrayValue(payload["capability_results"])) != 2 {
		t.Fatalf("frozen capability envelope = %#v", payload)
	}
	if len(assistant) == 0 || stringValue(assistant["id"]) == "" {
		t.Fatalf("assistant id missing: %#v", assistant)
	}
}

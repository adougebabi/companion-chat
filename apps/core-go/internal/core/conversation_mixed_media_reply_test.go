package core

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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
		{name: "provider transport", err: fmt.Errorf("%w: %w", errProviderRequestFailed, errors.New("upstream unavailable")), want: "provider_request_failed"},
		{name: "legacy provider transport wrapper", err: errors.New("provider request failed: upstream unavailable"), want: "provider_request_failed"},
		{name: "provider suppression", err: fmt.Errorf("%w: %w", errProviderRequestFailed, errProviderPaused), want: "fluctlight_paused"},
		{name: "agent failure", err: errors.New("agent_run_failed: adk loop failed"), want: "agent_run_failed"},
		{name: "business final contract failure", err: fmt.Errorf("%w: invalid influence", errAgentFinalContractInvalid), want: "agent_final_contract_invalid"},
		{name: "final contract failure", err: errors.New("adk_final_output_invalid: schema mismatch"), want: "adk_final_output_invalid"},
		{name: "tool execution failure", err: errors.New("tool execution tool_execution_failed: dependency unavailable"), want: "tool_execution_failed"},
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
// typed-empty final contract. Both Tools commit through their own formal
// boundaries and leave durable message, media, workflow, and source records.
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
	if err == nil || !strings.Contains(err.Error(), "adk_final_output_invalid") {
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
	if token := stringValue(mapValue(frames[1]["payload"])["text"]); token != "诶？真的要看啊。" {
		t.Fatalf("stream token exposed non-committed content: %q", token)
	}
	assistant := mapValue(mapValue(frames[2]["payload"])["message"])
	if stringValue(assistant["kind"]) != "assistant" || stringValue(assistant["text"]) != "诶？真的要看啊。" {
		t.Fatalf("assistant stream frame = %#v", frames[2])
	}
	assertMixedMediaReplyDurability(t, ctx, repository, fluctlightID, conversationID, "mixed-turn-stream", "mixed-turn-stream-1", assistant)
}

func TestStreamTurnPreservesProviderResponseWhenCommittedReplyPrecedesFinalContractFailure(t *testing.T) {
	invalidFinal := fakeProviderResult{Structured: map[string]any{
		"action_type": "reply", "response_intent": "reply already committed", "visible_text": "", "influences": []any{
			map[string]any{"ref": "memory:ctx_00000000000000000000000000000000", "role": "grounds", "confidence": 0.9, "note": "unknown reference"},
		},
	}}
	ctx, repository, app, ownerID, fluctlightID, conversationID := setupMixedMediaReplyTurnWithTransport(t, "final-contract", newConversationToolLoopTransportWithFinal(fakeProviderResult{ToolCalls: []map[string]any{
		{"id": "call_reply_contract", "type": "function", "function": map[string]any{"name": "conversation.reply", "arguments": jsonString(map[string]any{"text": "已提交的回复"})}},
	}}, invalidFinal))
	response := httptest.NewRecorder()
	turnID := "final-contract-turn-1"
	if err := app.StreamTurn(ctx, response, ownerID, conversationID, map[string]any{
		"fluctlight_id": fluctlightID, "text": "触发后置合同失败", "idempotency_key": "final-contract-turn", "turn_id": turnID, "attachment_refs": []any{},
	}); err != nil {
		t.Fatalf("StreamTurn returned transport error: %v", err)
	}
	var frames []map[string]any
	scanner := bufio.NewScanner(response.Body)
	for scanner.Scan() {
		var frame map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &frame); err != nil {
			t.Fatal(err)
		}
		frames = append(frames, frame)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(frames) < 2 || stringValue(frames[len(frames)-1]["type"]) != "error" || stringValue(mapValue(frames[len(frames)-1]["payload"])["code"]) != "agent_final_contract_invalid" {
		t.Fatalf("terminal frames = %#v", frames)
	}
	var assistantCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND text='已提交的回复'`, conversationID).Scan(&assistantCount); err != nil || assistantCount != 1 {
		t.Fatalf("committed reply count=%d err=%v", assistantCount, err)
	}
	var inboxStatus, inboxCode string
	if err := repository.Pool().QueryRow(ctx, `SELECT status,COALESCE(error_code,'') FROM public.cognition_inbox WHERE fluctlight_id=$1 AND idempotency_key='final-contract-turn'`, fluctlightID).Scan(&inboxStatus, &inboxCode); err != nil || inboxStatus != "failed" || inboxCode != "agent_final_contract_invalid" {
		t.Fatalf("inbox status=%q code=%q err=%v", inboxStatus, inboxCode, err)
	}
	correlationID := "turn:" + turnID
	runs, err := app.ModelRunsFiltered(ctx, ownerID, 100, correlationID)
	if err != nil {
		t.Fatal(err)
	}
	visibleResponses := 0
	for _, run := range runs {
		if responseJSON, ok := run["response"].(json.RawMessage); ok && len(responseJSON) > 0 && string(responseJSON) != "null" {
			visibleResponses++
		}
		if stringValue(run["error_code"]) == "provider_request_failed" {
			t.Fatalf("business contract failure fabricated Provider failure: %#v", run)
		}
	}
	if visibleResponses < 2 {
		t.Fatalf("parent correlation responses=%d runs=%#v", visibleResponses, runs)
	}
	events, err := app.DiagnosticsFiltered(ctx, ownerID, 100, correlationID, fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	foundTermination := false
	for _, event := range events {
		payload := mapValue(event["payload"])
		if stringValue(event["event_type"]) == "agent.run.termination" && stringValue(payload["stage"]) == "final_contract" && stringValue(payload["reason"]) == "agent_final_contract_invalid" {
			foundTermination = true
		}
	}
	if !foundTermination {
		t.Fatalf("final contract diagnostic missing: %#v", events)
	}
}

func TestFailedConversationTurnCanBeRetriedAndStreamed(t *testing.T) {
	callCount := 0
	transport := projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return nil, errors.New("simulated network connection failure")
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		payload := decodeObject(body)
		content := jsonString(map[string]any{
			"action_type":     "reply",
			"response_intent": "回复用户",
			"visible_text":    "重试成功回复",
			"influences":      []any{},
		})
		if boolValue(payload["stream"]) {
			chunk := map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"role": "assistant", "content": content}, "finish_reason": "stop"}}}
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader("data: " + string(jsonBytes(chunk)) + "\n\ndata: [DONE]\n\n")), Request: request}, nil
		}
		envelope := map[string]any{
			"choices": []any{
				map[string]any{
					"finish_reason": "stop",
					"message": map[string]any{
						"role":    "assistant",
						"content": content,
					},
				},
			},
		}
		return embeddingHTTPResponse(request, http.StatusOK, string(jsonBytes(envelope))), nil
	})
	ctx, repository, app, ownerID, fluctlightID, conversationID := setupMixedMediaReplyTurnWithTransport(t, "retry", transport)

	firstResp := httptest.NewRecorder()
	turnPayload := map[string]any{
		"fluctlight_id": fluctlightID, "text": "你好", "idempotency_key": "retry-turn-key", "turn_id": "retry-turn-1", "attachment_refs": []any{},
	}

	// First attempt fails due to simulated provider network error.
	_ = app.StreamTurn(ctx, firstResp, ownerID, conversationID, turnPayload)
	var firstFrames []map[string]any
	scanner := bufio.NewScanner(firstResp.Body)
	for scanner.Scan() {
		var frame map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &frame); err == nil {
			firstFrames = append(firstFrames, frame)
		}
	}
	if len(firstFrames) == 0 {
		t.Fatalf("expected frames in first attempt, got 0")
	}
	lastFirstFrame := firstFrames[len(firstFrames)-1]
	if stringValue(lastFirstFrame["type"]) != "error" {
		t.Fatalf("first attempt should end in error, got %#v", lastFirstFrame)
	}

	// Verify inbox is marked failed.
	var status string
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.cognition_inbox WHERE fluctlight_id=$1 AND idempotency_key=$2`, fluctlightID, "retry-turn-key").Scan(&status); err != nil || status != "failed" {
		t.Fatalf("cognition_inbox status=%q err=%v, want failed", status, err)
	}

	// Second attempt (retry) with the same idempotency key and turn ID.
	secondResp := httptest.NewRecorder()
	err := app.StreamTurn(ctx, secondResp, ownerID, conversationID, turnPayload)
	if err != nil {
		t.Fatalf("second attempt StreamTurn failed: %v", err)
	}
	var secondFrames []map[string]any
	scanner = bufio.NewScanner(secondResp.Body)
	for scanner.Scan() {
		var frame map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &frame); err == nil {
			secondFrames = append(secondFrames, frame)
		}
	}
	if len(secondFrames) == 0 {
		t.Fatalf("expected frames in second attempt, got 0")
	}
	lastSecondFrame := secondFrames[len(secondFrames)-1]
	if stringValue(lastSecondFrame["type"]) != "completed" {
		t.Fatalf("second attempt should end in completed, got %#v", lastSecondFrame)
	}

	// Verify assistant message is now committed.
	var assistantCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND text=$2`, conversationID, "重试成功回复").Scan(&assistantCount); err != nil || assistantCount != 1 {
		t.Fatalf("committed assistant message count=%d err=%v", assistantCount, err)
	}
}

func setupMixedMediaReplyTurnWithTransport(t *testing.T, suffix string, transport http.RoundTripper) (context.Context, *PostgresRepository, *App, string, string, string) {
	t.Helper()
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "mixed-owner-"+suffix, "mixed-fluctlight-"+suffix, "mixed-conversation-"+suffix
	seedTurnConversation(t, ctx, repository, ownerID, fluctlightID, conversationID)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'hash','revision-1')`, ownerID); err != nil {
		t.Fatal(err)
	}
	seedCognitiveProviderRole(t, ctx, repository, "mixed-endpoint-"+suffix)
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.fluctlights SET identity=$2 WHERE id=$1`, fluctlightID, jsonBytes(map[string]any{
		"timezone":   "Asia/Shanghai",
		"appearance": map[string]any{"hair": "black hair", "outfit": "coat"},
	})); err != nil {
		t.Fatal(err)
	}
	app := newTestApp(t, repository, transport)
	return ctx, repository, app, ownerID, fluctlightID, conversationID
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

	app := newTestApp(t, repository, newConversationToolLoopTransport(providerResult))
	return ctx, repository, app, ownerID, fluctlightID, conversationID
}

func conversationPayloadHasToolResult(payload map[string]any) bool {
	for _, raw := range arrayValue(payload["messages"]) {
		if stringValue(mapValue(raw)["role"]) == "tool" {
			return true
		}
	}
	return false
}

func validToolOnlyConversationFinal() map[string]any {
	return map[string]any{
		"action_type": "reply", "response_intent": "committed native Tool results",
		"visible_text": "", "influences": []any{},
	}
}

func newConversationToolLoopTransport(first fakeProviderResult) http.RoundTripper {
	return newConversationToolLoopTransportWithFinal(first, fakeProviderResult{Structured: validToolOnlyConversationFinal()})
}

func newConversationToolLoopTransportWithFinal(first, final fakeProviderResult) http.RoundTripper {
	return projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		payload := decodeObject(body)
		result := first
		if len(first.ToolCalls) > 0 && conversationPayloadHasToolResult(payload) {
			result = final
		}
		content := result.Text
		if result.Structured != nil {
			content = jsonString(result.Structured)
		}
		toolCalls := fakeProviderNativeToolCalls(result.ToolCalls)
		finishReason := "stop"
		if len(toolCalls) > 0 {
			finishReason = "tool_calls"
		}
		if boolValue(payload["stream"]) {
			for index := range toolCalls {
				toolCalls[index]["index"] = index
			}
			delta := map[string]any{"role": "assistant", "content": content}
			if len(toolCalls) > 0 {
				delta["tool_calls"] = toolCalls
			}
			chunk := map[string]any{"choices": []any{map[string]any{"delta": delta, "finish_reason": finishReason}}}
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader("data: " + string(jsonBytes(chunk)) + "\n\ndata: [DONE]\n\n")), Request: request}, nil
		}
		message := map[string]any{"role": "assistant", "content": content, "tool_calls": toolCalls}
		envelope := map[string]any{"choices": []any{map[string]any{"finish_reason": finishReason, "message": message}}}
		return embeddingHTTPResponse(request, http.StatusOK, string(jsonBytes(envelope))), nil
	})
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
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.media_intents WHERE owner_fluctlight_id=$1 AND conversation_id=$2 AND message_id IS NULL`, fluctlightID, conversationID).Scan(&mediaCount); err != nil {
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
	var inboxPayload []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT payload FROM public.cognition_inbox WHERE fluctlight_id=$1 AND idempotency_key=$2`, fluctlightID, idempotencyKey).Scan(&inboxPayload); err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(inboxPayload, &payload); err != nil {
		t.Fatal(err)
	}
	agentResult := mapValue(payload["agent_result"])
	if len(arrayValue(agentResult["capability_invocations"])) != 2 || len(arrayValue(agentResult["capability_results"])) != 2 {
		t.Fatalf("agent capability envelope = %#v", payload)
	}
	var linked int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE id=$1 AND turn_id=$2 AND source_fact_id=(SELECT id FROM public.cognition_inbox WHERE fluctlight_id=$3 AND idempotency_key=$4) AND correlation_id='turn:' || $2`, stringValue(assistant["id"]), turnID, fluctlightID, idempotencyKey).Scan(&linked); err != nil || linked != 1 {
		t.Fatalf("assistant source linkage count=%d err=%v", linked, err)
	}
	if len(assistant) == 0 || stringValue(assistant["id"]) == "" {
		t.Fatalf("assistant id missing: %#v", assistant)
	}
}

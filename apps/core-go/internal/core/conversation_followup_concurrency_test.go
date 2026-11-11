package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// Three independently accepted messages retain their identities. The first
// model request is held until the other messages commit, simulating a late
// response without relying on Redis cancellation to enforce publication.
func TestDirectFollowupsSupersedeLateReplyAndKeepWholeInput(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	requests := make(chan map[string]any, 4)
	calls := 0
	transport := projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, err
		}
		calls++
		if calls == 1 {
			close(started)
			<-release
		} else {
			requests <- payload
		}
		text := "旧消息的迟到回复"
		if calls > 1 {
			text = "已收到三条消息。"
		}
		decision := map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}
		call := nativePersonaToolCall(fmt.Sprintf("followup-reply-%d", calls), conversationReplyCapabilityName, map[string]any{"text": text, "decision": decision})
		message := map[string]any{"role": "assistant", "content": "", "tool_calls": fakeProviderNativeToolCalls([]map[string]any{call})}
		envelope := map[string]any{"choices": []any{map[string]any{"finish_reason": "tool_calls", "message": message}}}
		return embeddingHTTPResponse(request, http.StatusOK, jsonString(envelope)), nil
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "direct-followups", transport)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	accept := func(id, text string) TurnResult {
		t.Helper()
		result, err := app.AcceptTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": text, "turn_id": id, "idempotency_key": id, "attachment_refs": []any{}})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	first := accept("followup-first", "同一句短消息")
	done := make(chan error, 1)
	go func() { _, err := app.ProcessCognitionInbox(ctx, first.InboxID); done <- err }()
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("first model request did not start")
	}
	second := accept("followup-second", "同一句短消息")
	third := accept("followup-third", "最后补充的信息")
	unblock()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("superseded model request did not finish")
	}
	if _, err := app.ProcessCognitionInbox(ctx, third.InboxID); err != nil {
		t.Fatalf("latest turn failed: %v", err)
	}
	var latest map[string]any
	select {
	case latest = <-requests:
	default:
		t.Fatal("latest physical request was not observed")
	}
	var messages []string
	for _, raw := range arrayValue(latest["messages"]) {
		message := mapValue(raw)
		if message["role"] == "user" {
			text := stringValue(message["content"])
			if strings.HasSuffix(text, "\n同一句短消息") {
				messages = append(messages, "同一句短消息")
			} else if text == "最后补充的信息" {
				messages = append(messages, text)
			}
		}
	}
	if fmt.Sprint(messages) != "[同一句短消息 同一句短消息 最后补充的信息]" {
		t.Fatalf("latest request lost, reordered or duplicated input: %#v", messages)
	}
	var statusA, statusB, statusC, codeA, codeB string
	if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT status FROM public.cognition_inbox WHERE id=$1),(SELECT status FROM public.cognition_inbox WHERE id=$2),(SELECT status FROM public.cognition_inbox WHERE id=$3),
		(SELECT COALESCE(error_code,'') FROM public.cognition_inbox WHERE id=$1),(SELECT COALESCE(error_code,'') FROM public.cognition_inbox WHERE id=$2)`, first.InboxID, second.InboxID, third.InboxID).Scan(&statusA, &statusB, &statusC, &codeA, &codeB); err != nil {
		t.Fatal(err)
	}
	var assistants, final int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE source_fact_id=$2 AND text='已收到三条消息。') FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversation, third.InboxID).Scan(&assistants, &final); err != nil {
		t.Fatal(err)
	}
	if statusA != "failed" || statusB != "failed" || codeA != "superseded_by_newer_turn" || codeB != "superseded_by_newer_turn" || statusC != "processed" || assistants != 1 || final != 1 || calls != 2 {
		t.Fatalf("statuses=%s/%s/%s codes=%s/%s assistants=%d latest=%d calls=%d", statusA, statusB, statusC, codeA, codeB, assistants, final, calls)
	}
	if _, err := app.ProcessCognitionInbox(ctx, third.InboxID); err != nil || calls != 2 {
		t.Fatalf("latest turn replay repeated model request: calls=%d err=%v", calls, err)
	}
}

func TestDirectFollowupKeepsPriorCommittedAction(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	calls := 0
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		calls++
		name := capabilityCatalogName
		args := map[string]any{"names": []any{"affect_event"}}
		switch calls {
		case 2:
			name = "affect_event"
			args = map[string]any{"event": map[string]any{"type": "calm", "confidence": 0.8}}
		case 3:
			close(started)
			<-release
			name = conversationReplyCapabilityName
			args = map[string]any{"text": "迟到的旧回复", "decision": map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}}
		default:
			if calls > 3 {
				name = conversationReplyCapabilityName
				args = map[string]any{"text": "我看到了新消息。", "decision": map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}}
			}
		}
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(fmt.Sprintf("followup-action-%d", calls), name, args)}}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "direct-action-followup", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	first, err := app.AcceptTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "先平静下来", "turn_id": "action-first", "idempotency_key": "action-first", "attachment_refs": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := app.ProcessCognitionInbox(ctx, first.InboxID); done <- err }()
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("prior action did not complete before the held reply")
	}
	var events, receipts int
	if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.fluctlight_inner_state_events WHERE fluctlight_id=$1 AND event_type='affect.event'),
		(SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$1 AND capability_name='affect_event')`, fid).Scan(&events, &receipts); err != nil || events != 1 || receipts != 1 {
		t.Fatalf("prior action did not commit: events=%d receipts=%d err=%v", events, receipts, err)
	}
	second, err := app.AcceptTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "我还有话说", "turn_id": "action-followup", "idempotency_key": "action-followup", "attachment_refs": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	unblock()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("old turn did not stop")
	}
	if _, err := app.ProcessCognitionInbox(ctx, second.InboxID); err != nil {
		t.Fatalf("follow-up turn failed: %v", err)
	}
	var assistants, latest int
	if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.fluctlight_inner_state_events WHERE fluctlight_id=$1 AND event_type='affect.event'),
		(SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$1 AND capability_name='affect_event'),
		(SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$2 AND kind='assistant'),
		(SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$2 AND source_fact_id=$3 AND text='我看到了新消息。')`, fid, conversation, second.InboxID).Scan(&events, &receipts, &assistants, &latest); err != nil {
		t.Fatal(err)
	}
	if events != 1 || receipts != 1 || assistants != 1 || latest != 1 || calls != 4 {
		t.Fatalf("events=%d receipts=%d assistants=%d latest=%d calls=%d", events, receipts, assistants, latest, calls)
	}
}

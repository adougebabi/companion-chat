package core

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestPostgresOwnerRequestLaneAllowsChatDuringSlowImageAndOutfitAssessment(t *testing.T) {
	for _, kind := range []string{"item_photo", "wardrobe_wear"} {
		t.Run(kind, func(t *testing.T) {
			f := seedWardrobeToolFixture(t)
			seedCognitiveProviderRole(t, f.ctx, f.repository, "owner-chat-lanes-"+f.suffix)
			// Both tasks retain the one documented physical Provider limit. Two
			// configured slots let this test isolate the per-character lanes.
			if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('llm.queue','{"generated_concurrency":2,"embedding_concurrency":1}') ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json`); err != nil {
				t.Fatal(err)
			}
			started, release := make(chan struct{}), make(chan struct{})
			var startOnce, releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			defer unblock()
			ownerRouter := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
				return fakeProviderResult{Structured: map[string]any{"decision": "rejected", "reason": "稍后再处理这个请求"}}
			})
			chatRouter := newFakeProviderRouter().onNativeConversationReply(func(map[string]any) nativeConversationReplyFixture {
				return nativeConversationReplyFixture{ReplyText: "我在，聊天可以继续。", Final: fakeProviderResult{Structured: map[string]any{"action_type": "reply", "response_intent": "及时回应", "influences": []any{}}}}
			})
			transport := projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				raw, err := io.ReadAll(request.Body)
				if err != nil {
					return nil, err
				}
				request.Body = io.NopCloser(bytes.NewReader(raw))
				if strings.Contains(string(raw), ownerLifeRequestFactType) {
					startOnce.Do(func() { close(started) })
					select {
					case <-release:
					case <-request.Context().Done():
						return nil, request.Context().Err()
					}
					return ownerRouter.RoundTrip(request)
				}
				return chatRouter.RoundTrip(request)
			})
			f.app.Provider.HTTP = &http.Client{Transport: transport}
			boots := wardrobeItemID(t, f, "黑色短靴")
			body := map[string]any{"item_id": boots, "slot": "shoes", "expected_revision": wardrobeRevision(t, f), "reason": "先提出一个慢请求", "idempotency_key": "slow-owner-request"}
			path := "/internal/fluctlights/" + f.fluctlightID + "/wardrobe/wear"
			if kind == "item_photo" {
				body = map[string]any{"prompt": "画这双靴子", "expected_item_revision": 0, "reason": "图片请求", "idempotency_key": "slow-owner-request"}
				path = "/internal/fluctlights/" + f.fluctlightID + "/wardrobe/items/" + boots + "/photos"
				var revision int
				if err := f.repository.Pool().QueryRow(f.ctx, `SELECT revision FROM public.fluctlight_wardrobe_items WHERE id=$1`, boots).Scan(&revision); err != nil {
					t.Fatal(err)
				}
				body["expected_item_revision"] = revision
			}
			admitted, err := f.app.DispatchOwnerSurface(f.ctx, http.MethodPost, path, nil, f.ownerID, body)
			if err != nil {
				t.Fatal(err)
			}
			inbox := stringValue(mapValue(admitted["request"])["source_fact_id"])
			ownerDone := make(chan error, 1)
			go func() { _, err := f.app.ProcessCognitionInbox(f.ctx, inbox); ownerDone <- err }()
			select {
			case <-started:
			case err := <-ownerDone:
				t.Fatalf("Owner request did not reach Provider: %v", err)
			case <-time.After(5 * time.Second):
				t.Fatal("Owner request did not start")
			}
			// Duplicate processing and later requests in the same lane still wait.
			duplicate, err := f.app.ProcessCognitionInbox(f.ctx, inbox)
			if err != nil || duplicate["status"] != "waiting_for_agent" {
				t.Fatalf("duplicate owner run=%#v err=%v", duplicate, err)
			}
			secondBody := cloneMap(body)
			secondBody["idempotency_key"] = "next-owner-request"
			second, err := f.app.DispatchOwnerSurface(f.ctx, http.MethodPost, path, nil, f.ownerID, secondBody)
			if err != nil {
				t.Fatal(err)
			}
			nextInbox := stringValue(mapValue(second["request"])["source_fact_id"])
			waiting, err := f.app.ProcessCognitionInbox(f.ctx, nextInbox)
			if err != nil || waiting["status"] != "waiting_for_agent" {
				t.Fatalf("same-lane ordering lost=%#v err=%v", waiting, err)
			}
			chat, err := f.app.AcceptTurn(f.ctx, f.ownerID, f.conversationID, map[string]any{"fluctlight_id": f.fluctlightID, "text": "先聊一句", "turn_id": "parallel-chat", "idempotency_key": "parallel-chat"})
			if err != nil {
				t.Fatal(err)
			}
			chatCtx, cancel := context.WithTimeout(f.ctx, 3*time.Second)
			defer cancel()
			result, err := f.app.ProcessCognitionInbox(chatCtx, chat.InboxID)
			if err != nil || result["status"] != "committed" {
				t.Fatalf("chat blocked by Owner work: %#v %v", result, err)
			}
			// Reflection may run after chat, but its cumulative cursor must not
			// skip older Owner evidence that is still in flight in another lane.
			reflection, err := f.app.ProcessReflection(f.ctx, f.fluctlightID, "lanes-reflection")
			if err != nil || reflection["status"] != "no_op" || intValue(reflection["watermark"]) != 0 {
				t.Fatalf("Reflection skipped unresolved source: %#v err=%v", reflection, err)
			}
			select {
			case err := <-ownerDone:
				t.Fatalf("slow Owner request should still be running: %v", err)
			default:
			}
			unblock()
			if err := <-ownerDone; err != nil {
				t.Fatal(err)
			}
			if _, err := f.app.ProcessCognitionInbox(f.ctx, nextInbox); err != nil {
				t.Fatal("next Owner request did not resume", err)
			}
			rows, err := f.repository.Pool().Query(f.ctx, reflectionSourceQuery, f.fluctlightID, 0)
			if err != nil {
				t.Fatal(err)
			}
			sources := map[string]bool{}
			for rows.Next() {
				var id, event, fingerprint string
				var sequence int
				var payload []byte
				var occurred time.Time
				if err := rows.Scan(&id, &sequence, &event, &payload, &occurred, &fingerprint); err != nil {
					rows.Close()
					t.Fatal(err)
				}
				sources[id] = true
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			rows.Close()
			if !sources[inbox] || !sources[nextInbox] || !sources[chat.InboxID] {
				t.Fatalf("later source settlement lost Reflection evidence: %#v", sources)
			}
			var scheduled bool
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM public.platform_workflow_intents WHERE intent_id=$1 AND status='pending' AND next_attempt_at IS NOT NULL AND payload->>'source_fact_id'=$2)`, "reflection_intent:owner-request:"+nextInbox, nextInbox).Scan(&scheduled); err != nil || !scheduled {
				t.Fatalf("settled Owner evidence did not schedule a new Reflection: %v %v", scheduled, err)
			}
		})
	}
}

func TestPostgresOwnerRequestLaneFencesStaleClaimBeforeToolAndSettlement(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	boots := wardrobeItemID(t, f, "黑色短靴")
	body := map[string]any{"item_id": boots, "slot": "shoes", "expected_revision": wardrobeRevision(t, f), "reason": "保留原请求", "idempotency_key": "fenced-owner"}
	admitted, err := f.app.DispatchOwnerSurface(f.ctx, http.MethodPost, "/internal/fluctlights/"+f.fluctlightID+"/wardrobe/wear", nil, f.ownerID, body)
	if err != nil {
		t.Fatal(err)
	}
	inbox := stringValue(mapValue(admitted["request"])["source_fact_id"])
	if _, _, err := f.app.claimCognitionInbox(f.ctx, inbox, "old-owner"); err != nil {
		t.Fatal(err)
	}
	oldCtx := context.WithValue(f.ctx, ownerRequestClaimKey{}, ownerRequestClaim{InboxID: inbox, Owner: f.fluctlightID, Token: "old-owner"})
	// A new owner acquires the expired claim. The old model result must not
	// mutate wardrobe or close the new owner's still-running request.
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.cognition_inbox SET claimed_by='new-owner',claimed_at=now() WHERE id=$1`, inbox); err != nil {
		t.Fatal(err)
	}
	before := wardrobeRevision(t, f)
	_, err = f.app.ExecuteTool(oldCtx, f.request(wardrobeWearCapabilityName, "stale-owner-wear", map[string]any{"mode": "partial", "item_ids": []string{boots}}))
	if !errors.Is(err, errOwnerRequestClaimLost) || wardrobeRevision(t, f) != before {
		t.Fatalf("stale model committed Tool: %v", err)
	}
	err = withTransaction(oldCtx, f.repository.Pool(), func(tx pgx.Tx) error {
		if err := fenceOwnerRequestClaimTx(oldCtx, tx); err != nil {
			return err
		}
		return settleOwnerLifeInboxTx(oldCtx, tx, inbox, map[string]any{"status": "completed"})
	})
	if !errors.Is(err, errOwnerRequestClaimLost) {
		t.Fatalf("stale owner settled inbox: %v", err)
	}
	var status, owner string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status,claimed_by FROM public.cognition_inbox WHERE id=$1`, inbox).Scan(&status, &owner); err != nil || status != "claimed" || owner != "new-owner" {
		t.Fatalf("successor claim overwritten: %s %s %v", status, owner, err)
	}
}

func TestPostgresOwnerRequestLaneKeepsOriginalCASAfterConcurrentWardrobeChange(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "owner-cas-lane-"+f.suffix)
	boots := wardrobeItemID(t, f, "黑色短靴")
	before := wardrobeRevision(t, f)
	admitted, err := f.app.DispatchOwnerSurface(f.ctx, http.MethodPost, "/internal/fluctlights/"+f.fluctlightID+"/wardrobe/wear", nil, f.ownerID, map[string]any{"item_id": boots, "slot": "shoes", "expected_revision": before, "reason": "先判断是否穿上", "idempotency_key": "owner-cas-lane"})
	if err != nil {
		t.Fatal(err)
	}
	inbox := stringValue(mapValue(admitted["request"])["source_fact_id"])
	started, release := make(chan struct{}), make(chan struct{})
	var startOnce, releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		startOnce.Do(func() { close(started) })
		<-release
		if !r6PayloadHasTool(payload, wardrobeWearCapabilityName) {
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("owner-cas-discover", capabilityCatalogName, map[string]any{"names": []string{wardrobeWearCapabilityName}})}}
		}
		if !payloadHasToolResult(payload) || !strings.Contains(jsonString(payload), "owner-cas-wear") {
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("owner-cas-wear", wardrobeWearCapabilityName, map[string]any{"mode": "partial", "item_ids": []string{boots}})}}
		}
		return fakeProviderResult{Structured: map[string]any{"decision": "deferred", "reason": "衣橱状态已变化，请重新核对"}}
	})
	f.app.Provider.HTTP = &http.Client{Transport: router}
	done := make(chan error, 1)
	go func() { _, err := f.app.ProcessCognitionInbox(f.ctx, inbox); done <- err }()
	select {
	case <-started:
	case err := <-done:
		t.Fatalf("Owner request did not start: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Owner request did not reach Provider")
	}
	// A concurrently admitted independent Tool changes the shared wardrobe while
	// the Owner model has its older frozen request and projection in flight.
	changed, err := f.app.ExecuteTool(f.ctx, f.request(wardrobeWearCapabilityName, "concurrent-full-outfit", map[string]any{"mode": "full", "item_ids": []string{wardrobeItemID(t, f, "银项链")}}))
	if err != nil || changed.Result.Status != "completed" {
		t.Fatalf("concurrent Tool=%#v err=%v", changed, err)
	}
	unblock()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := wardrobeRevision(t, f); got != before+1 {
		t.Fatalf("stale Owner request changed wardrobe: revision=%d want=%d", got, before+1)
	}
	var effects int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.tool_executions WHERE invocation->>'source_fact_id'=$1 AND capability_name=$2 AND result->>'status'='completed'`, inbox, wardrobeWearCapabilityName).Scan(&effects); err != nil || effects != 0 {
		t.Fatalf("stale Owner effect count=%d err=%v", effects, err)
	}
}

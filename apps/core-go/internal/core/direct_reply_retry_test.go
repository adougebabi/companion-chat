package core

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestDirectReplyRetryRequiresTerminalWorkflow(t *testing.T) {
	if !directReplyRetryFailure("agent_final_contract_invalid", "failed", "") {
		t.Fatal("terminal missing reply should qualify")
	}
	if !directReplyRetryFailure("direct_media_delivery_unresolved", "failed", "") {
		t.Fatal("terminal delivery check failure should qualify after a committed image")
	}
	if !directReplyRetryFailure("adk_run_failed", "failed", "provider_request_failed") ||
		directReplyRetryFailure("adk_run_failed", "failed", "") {
		t.Fatal("generic terminal code must have scoped provider failure evidence")
	}
	for _, status := range []string{"", "pending", "started", "running", "paused", "cancel_requested"} {
		if directReplyRetryFailure("agent_final_contract_invalid", status, "") {
			t.Errorf("active or unknown workflow %q qualified for recovery", status)
		}
	}
}

func TestDirectReplyRetryUsesExistingMediaWithoutReplayingTurn(t *testing.T) {
	t.Run("missing native reply", func(t *testing.T) { testDirectReplyRetryUsesExistingMedia(t, "missing") })
	t.Run("legacy provider failure", func(t *testing.T) { testDirectReplyRetryUsesExistingMedia(t, "provider") })
	t.Run("automatic capacity recovery", func(t *testing.T) { testDirectReplyRetryUsesExistingMedia(t, "capacity") })
}

func testDirectReplyRetryUsesExistingMedia(t *testing.T, failureMode string) {
	t.Helper()
	const recovered = "自拍任务已经提交，等图片生成后再发。"
	mainCalls := 0
	recoveryRound := 6
	if failureMode != "missing" {
		recoveryRound = 5
	}
	router := newFakeProviderRouter().
		on(directMediaDeliveryCheckSchema, func(map[string]any) fakeProviderResult {
			return fakeProviderResult{Structured: map[string]any{"claims": []any{}}}
		}).
		otherwise(func(payload map[string]any) fakeProviderResult {
			mainCalls++
			invoke := func(id, name string, args map[string]any) fakeProviderResult {
				return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(id, name, args)}}
			}
			reply := func(id, text string) fakeProviderResult {
				return invoke(id, conversationReplyCapabilityName, map[string]any{"text": text, "decision": map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}})
			}
			switch mainCalls {
			case recoveryRound:
				tools := arrayValue(payload["tools"])
				wire := jsonString(payload["messages"])
				if len(tools) != 1 || !r6PayloadHasTool(payload, conversationReplyCapabilityName) ||
					!strings.Contains(wire, "reply_recovery") || !strings.Contains(wire, "把任务回执、后台和传输过程留在内部") {
					t.Error("retry did not offer only the native reply tool with the actual media status")
				}
				return reply("retry-recovered", recovered)
			case 1:
				return reply("retry-prior", "拍好自拍再发给你。")
			case 2:
				return invoke("retry-catalog", capabilityCatalogName, map[string]any{"names": []any{"media.image.generate"}})
			case 3:
				plan := mediaTestPlan("person", "present", "yes", "full")
				plan["capture_method"], plan["viewpoint"], plan["framing"], plan["camera_angle"] = "handheld_selfie", "subject_facing", "full_body", "eye_level"
				return invoke("retry-image", "media.image.generate", map[string]any{"intent": "拍一张当前自拍", "media_plan": plan})
			case 4, 5:
				if failureMode == "capacity" {
					return reply("retry-capacity-check", recovered)
				}
				if failureMode == "provider" {
					return fakeProviderResult{Status: http.StatusBadRequest}
				}
				return fakeProviderResult{Text: "模型输出文字但没有调用原生发布工具。"}
			default:
				t.Errorf("unexpected model round %d", mainCalls)
				return fakeProviderResult{Status: 500}
			}
		})
	injectCapacityFailure := false
	transport := providerErrorTestTransport(func(request *http.Request) (*http.Response, error) {
		if injectCapacityFailure {
			body, err := io.ReadAll(request.Body)
			if err != nil {
				return nil, err
			}
			request.Body = io.NopCloser(bytes.NewReader(body))
			if providerWireSchemaName(decodeObject(body)) == directMediaDeliveryCheckSchema {
				injectCapacityFailure = false
				return embeddingHTTPResponse(request, http.StatusBadRequest,
					`{"error":{"message":"Prompt (2340 tokens) requires ~1687MB GPU memory but only ~0MB is available"}}`), nil
			}
		}
		return router.RoundTrip(request)
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "direct-reply-only-recovery", transport)
	now := time.Now()
	app.Clock = func() time.Time { return now }
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_visual_identities(id,fluctlight_id,status,identity_snapshot) VALUES('direct-retry-identity',$1,'active','{"identity":{"name":"Fixture identity"}}')`, fid); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('media.comfyui',$1)`, jsonString(map[string]any{"baseUrl": "http://comfy.invalid", "workflow": map[string]any{"prompt": "{{prompt}}"}})); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	if _, err := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "能发张自拍吗？", "idempotency_key": "retry-prior", "attachment_refs": []any{}}); err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{"fluctlight_id": fid, "text": "还没发哦", "turn_id": "retry-user-turn", "idempotency_key": "retry-user-key", "attachment_refs": []any{}}
	accepted, err := app.AcceptTurn(ctx, owner, conversation, payload)
	if err != nil {
		t.Fatal(err)
	}
	injectCapacityFailure = failureMode == "capacity"
	first, failure := app.ProcessCognitionInbox(ctx, accepted.InboxID)
	if failureMode == "capacity" {
		if failure != nil || first["status"] != "retry_scheduled" {
			t.Fatalf("the temporary verification failure should schedule reply-only recovery: status=%v err=%v", first["status"], failure)
		}
		early, err := app.ProcessCognitionInbox(ctx, accepted.InboxID)
		if err != nil || early["status"] != "retry_scheduled" || mainCalls != recoveryRound-1 {
			t.Fatalf("capacity retry started before its delay elapsed: status=%v calls=%d err=%v", early["status"], mainCalls, err)
		}
		replayed, err := app.HandleTurn(ctx, owner, conversation, payload)
		if err != nil || replayed.ReplyStatus != "retry_scheduled" || mainCalls != recoveryRound-1 {
			t.Fatalf("direct replay bypassed the retry delay: status=%v calls=%d err=%v", replayed.ReplyStatus, mainCalls, err)
		}
		now = now.Add(31 * time.Second)
	} else if failureMode == "provider" {
		var code string
		if err := repo.Pool().QueryRow(ctx, `SELECT error_code FROM public.cognition_inbox WHERE id=$1`, accepted.InboxID).Scan(&code); err != nil ||
			failure == nil || code != "provider_request_failed" {
			t.Fatalf("provider request failure did not survive settlement: failure=%v code=%s err=%v", failure, code, err)
		}
		if _, err := repo.Pool().Exec(ctx, `UPDATE public.cognition_inbox SET error_code='adk_run_failed' WHERE id=$1`, accepted.InboxID); err != nil {
			t.Fatal(err)
		}
	} else if failure == nil || !strings.Contains(failure.Error(), "conversation_native_output_missing") {
		t.Fatalf("expected an accepted media operation with no native reply, got %v", failure)
	}
	if failureMode != "capacity" {
		// The fixture invokes the activity directly. In production the dispatcher
		// records this terminal state before making the retry available.
		if _, err := repo.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='failed',completed_at=now() WHERE intent_id=$1`, "cognition_intent:"+accepted.InboxID); err != nil {
			t.Fatal(err)
		}
		page, err := repo.History(ctx, conversation, owner, nil, 10)
		if err != nil || len(page.Messages) != 3 || !page.Messages[2].TurnReplyRetryable || page.Messages[2].TurnRetryable {
			t.Fatalf("failed turn did not expose only reply recovery: %#v, %v", page.Messages, err)
		}
		if err := app.RetryDirectReply(ctx, "different-owner", conversation, accepted.TurnID); !errors.Is(err, ErrNotFound) {
			t.Fatalf("wrong actor could request recovery: %v", err)
		}
		if _, err := app.AcceptTurn(ctx, owner, conversation, payload); err != nil {
			t.Fatal(err)
		}
		var status string
		if err := repo.Pool().QueryRow(ctx, `SELECT status FROM public.cognition_inbox WHERE id=$1`, accepted.InboxID).Scan(&status); err != nil || status != "failed" {
			t.Fatalf("ordinary replay reopened the partial turn: status=%s err=%v", status, err)
		}
		for range 2 {
			if err := app.RetryDirectReply(ctx, owner, conversation, accepted.TurnID); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := app.ProcessCognitionInbox(ctx, accepted.InboxID); err != nil {
		t.Fatal(err)
	}
	if err := app.RetryDirectReply(ctx, owner, conversation, accepted.TurnID); err != nil {
		t.Fatalf("repeating an already completed recovery should report success: %v", err)
	}
	page, err := repo.History(ctx, conversation, owner, nil, 10)
	if err != nil || len(page.Messages) != 4 || page.Messages[2].TurnStatus != "completed" || page.Messages[3].Text != recovered ||
		page.Messages[2].ID != stringValue(accepted.UserMessage["id"]) {
		t.Fatalf("reply recovery did not finish the original turn: %#v, %v", page.Messages, err)
	}
	var intents int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.media_intents WHERE conversation_id=$1`, conversation).Scan(&intents); err != nil || intents != 1 || mainCalls != recoveryRound {
		t.Fatalf("reply retry produced additional work: images=%d calls=%d err=%v", intents, mainCalls, err)
	}
}

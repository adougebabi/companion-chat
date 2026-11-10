package core

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestDirectConversationUsesAndRechecksApprovedExpression(t *testing.T) {
	for _, scenario := range []string{"unchanged", "disabled", "revoked", "candidate_aba", "use_switch_aba", "source_aba", "unmatched", "prior_action", "prior_action_then_disable", "replay_after_disable", "cancel"} {
		t.Run(scenario, func(t *testing.T) {
			var ctx context.Context
			var cancelTurn context.CancelFunc
			var repo *PostgresRepository
			var app *App
			var owner, fid, conversation string
			var candidateID string
			var calls int
			current := "当前轮的问题"
			if scenario == "unmatched" {
				current = "聊点别的话题"
			}
			router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				if providerWireSchemaName(payload) == expressionLearningSchema {
					return fakeProviderResult{Structured: map[string]any{"candidates": []any{map[string]any{"source_ref": "s1", "excerpt": "慢慢来呀", "situation": "讨论当前进度"}}}}
				}
				calls++
				if calls > 3 {
					return fakeProviderResult{Status: 400}
				}
				wire := jsonString(payload["messages"])
				var suggestionMessages, currentMessages int
				for _, raw := range arrayValue(payload["messages"]) {
					message := mapValue(raw)
					content, role := stringValue(message["content"]), stringValue(message["role"])
					if role == "user" && content == current {
						currentMessages++
					}
					if strings.Contains(content, "expression_suggestions") {
						suggestionMessages++
						if role != "user" || !strings.Contains(content, "[STABLE TASK CONTEXT]") {
							t.Errorf("expression reference was not untrusted task data: role=%s", role)
						}
					}
				}
				if currentMessages != 1 {
					t.Errorf("current input appeared in %d user messages", currentMessages)
				}
				if scenario == "unmatched" {
					if suggestionMessages != 0 {
						t.Error("unmatched context received an expression suggestion")
					}
				} else {
					if suggestionMessages != 1 {
						t.Errorf("expression reference appeared in %d messages", suggestionMessages)
					}
					for _, required := range []string{"expression_suggestions", "慢慢来呀", "讨论当前进度"} {
						if !strings.Contains(wire, required) {
							t.Errorf("direct request lost expression data %q", required)
						}
					}
					for _, forbidden := range []string{candidateID, "source_fingerprint", "仅供审核的私有理由", "trigger_text"} {
						if strings.Contains(wire, forbidden) {
							t.Errorf("private expression binding reached the model: %s", forbidden)
						}
					}
				}
				if calls == 1 && scenario == "disabled" {
					if _, err := app.ReviewOwnerExpressionLearningCandidate(ctx, owner, fid, candidateID, "disable", map[string]any{"expected_revision": 2, "reason": "停止使用", "idempotency_key": "direct-use-disable"}); err != nil {
						t.Error(err)
					}
				}
				if calls == 1 && scenario == "revoked" {
					if _, err := app.ReviewOwnerExpressionLearningCandidate(ctx, owner, fid, candidateID, "revoke", map[string]any{"expected_revision": 2, "reason": "撤销", "idempotency_key": "direct-use-revoke"}); err != nil {
						t.Error(err)
					}
				}
				if calls == 1 && scenario == "candidate_aba" {
					if _, err := app.ReviewOwnerExpressionLearningCandidate(ctx, owner, fid, candidateID, "disable", map[string]any{"expected_revision": 2, "reason": "停用", "idempotency_key": "direct-candidate-off"}); err != nil {
						t.Error(err)
					}
					if _, err := app.ReviewOwnerExpressionLearningCandidate(ctx, owner, fid, candidateID, "enable", map[string]any{"expected_revision": 3, "reason": "重新启用", "trigger_text": "当前轮", "idempotency_key": "direct-candidate-on"}); err != nil {
						t.Error(err)
					}
				}
				if calls == 1 && scenario == "use_switch_aba" {
					if _, err := app.UpdateOwnerExpressionLearningSettings(ctx, owner, fid, map[string]any{"learning_enabled": true, "use_enabled": false, "expected_revision": 1, "idempotency_key": "direct-use-off"}); err != nil {
						t.Error(err)
					}
					if _, err := app.UpdateOwnerExpressionLearningSettings(ctx, owner, fid, map[string]any{"learning_enabled": true, "use_enabled": true, "expected_revision": 2, "idempotency_key": "direct-use-on"}); err != nil {
						t.Error(err)
					}
				}
				if calls == 1 && scenario == "source_aba" {
					if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_messages SET text='已撤回原文' WHERE id='direct-expression-source'`); err != nil {
						t.Error(err)
					}
					if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_messages SET text='慢慢来呀' WHERE id='direct-expression-source'`); err != nil {
						t.Error(err)
					}
				}
				if calls == 1 && scenario == "cancel" {
					cancelTurn()
				}
				if calls == 3 && scenario == "prior_action_then_disable" {
					if _, err := app.ReviewOwnerExpressionLearningCandidate(ctx, owner, fid, candidateID, "disable", map[string]any{"expected_revision": 2, "reason": "停止使用", "idempotency_key": "direct-action-disable"}); err != nil {
						t.Error(err)
					}
				}
				if (scenario == "prior_action" || scenario == "prior_action_then_disable") && calls <= 2 {
					name := capabilityCatalogName
					args := map[string]any{"names": []any{"affect_event"}}
					if calls == 2 {
						name = "affect_event"
						args = map[string]any{"event": map[string]any{"type": "calm", "confidence": 0.8}}
					}
					return fakeProviderResult{ToolCalls: []map[string]any{{"id": fmt.Sprintf("direct-expression-action-%d", calls), "type": "function", "function": map[string]any{"name": name, "arguments": jsonString(args)}}}}
				}
				decision := map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}
				args := map[string]any{"text": "我们慢慢来。", "decision": decision}
				return fakeProviderResult{ToolCalls: []map[string]any{{"id": fmt.Sprintf("direct-expression-%d", calls), "type": "function", "function": map[string]any{"name": conversationReplyCapabilityName, "arguments": jsonString(args)}}}}
			})
			ctx, repo, app, owner, fid, conversation = setupMixedMediaReplyTurnWithTransport(t, "direct-expression-"+scenario, router)
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES('direct-expression-source',$1,1,$2,'user','慢慢来呀','[]','direct-expression-source')`, conversation, owner); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_heads SET next_sequence=2 WHERE conversation_id=$1`, conversation); err != nil {
				t.Fatal(err)
			}
			persistConversationReplyMode(t, ctx, repo, "direct")
			if _, err := app.UpdateOwnerExpressionLearningSettings(ctx, owner, fid, map[string]any{"learning_enabled": true, "use_enabled": true, "expected_revision": 0, "idempotency_key": "direct-expression-settings"}); err != nil {
				t.Fatal(err)
			}
			extracted, err := app.ExtractOwnerExpressionCandidates(ctx, owner, fid, map[string]any{"source_message_id": "direct-expression-source", "idempotency_key": "direct-expression-extract"})
			if err != nil || len(arrayValue(extracted["candidate_ids"])) != 1 {
				t.Fatalf("expression extraction=%#v err=%v", extracted, err)
			}
			candidateID = stringValue(arrayValue(extracted["candidate_ids"])[0])
			if _, err := app.ReviewOwnerExpressionLearningCandidate(ctx, owner, fid, candidateID, "enable", map[string]any{"expected_revision": 1, "reason": "仅供审核的私有理由", "trigger_text": "当前轮", "idempotency_key": "direct-expression-enable"}); err != nil {
				t.Fatal(err)
			}
			turn := "direct-expression-" + scenario
			turnCtx, cancel := context.WithCancel(ctx)
			cancelTurn = cancel
			defer cancel()
			payload := map[string]any{"fluctlight_id": fid, "text": current, "idempotency_key": turn, "turn_id": turn, "attachment_refs": []any{}}
			result, err := app.HandleTurn(turnCtx, owner, conversation, payload)
			wantPublished := scenario == "unchanged" || scenario == "unmatched" || scenario == "prior_action" || scenario == "replay_after_disable"
			if (err == nil) != wantPublished {
				t.Fatalf("scenario=%s result=%#v err=%v", scenario, result, err)
			}
			if !wantPublished && scenario != "cancel" && !strings.Contains(err.Error(), "reply_expression_suggestion_stale") {
				t.Fatalf("scenario=%s did not preserve the stale expression reason: %v", scenario, err)
			}
			if scenario == "cancel" && !strings.Contains(err.Error(), "context canceled") {
				t.Fatalf("cancel did not stop the current turn: %v", err)
			}
			wantCalls := 1
			if scenario == "prior_action" || scenario == "prior_action_then_disable" {
				wantCalls = 3
			}
			if calls != wantCalls {
				t.Fatalf("scenario=%s made %d model requests", scenario, calls)
			}
			if scenario == "replay_after_disable" {
				if _, err := app.ReviewOwnerExpressionLearningCandidate(ctx, owner, fid, candidateID, "disable", map[string]any{"expected_revision": 2, "reason": "停用", "idempotency_key": "direct-replay-disable"}); err != nil {
					t.Fatal(err)
				}
				replayed, err := app.HandleTurn(ctx, owner, conversation, payload)
				if err != nil || stringValue(replayed.Assistant["text"]) != stringValue(result.Assistant["text"]) || calls != 1 {
					t.Fatalf("published reply was not replayed after disable: %#v err=%v calls=%d", replayed, err, calls)
				}
			}
			var messages, assessments, receipts, handled int
			if queryErr := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'),
				(SELECT count(*) FROM public.cognition_assessments WHERE fluctlight_id=$2),
				(SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$2 AND capability_name='conversation.reply' AND result->>'status'='completed'),
				(SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$2 AND capability_name='conversation.reply' AND result->>'status'='handled' AND result->'output'->>'warning_code'='reply_expression_suggestion_stale')`, conversation, fid).Scan(&messages, &assessments, &receipts, &handled); queryErr != nil {
				t.Fatal(queryErr)
			}
			wantCount := 0
			wantHandled := 1
			if wantPublished {
				wantCount = 1
				wantHandled = 0
			} else if scenario == "cancel" {
				wantHandled = 0
			}
			if messages != wantCount || assessments != wantCount || receipts != wantCount || handled != wantHandled {
				t.Fatalf("scenario=%s messages=%d assessments=%d receipts=%d handled=%d", scenario, messages, assessments, receipts, handled)
			}
			if scenario == "prior_action" || scenario == "prior_action_then_disable" {
				var events, actionReceipts, userMessages int
				if queryErr := repo.Pool().QueryRow(ctx, `SELECT
					(SELECT count(*) FROM public.fluctlight_inner_state_events WHERE fluctlight_id=$1 AND event_type='affect.event'),
					(SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$1 AND capability_name='affect_event' AND result->>'status'='completed'),
					(SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$2 AND kind='user' AND turn_id=$3)`, fid, conversation, turn).Scan(&events, &actionReceipts, &userMessages); queryErr != nil || events != 1 || actionReceipts != 1 || userMessages != 1 {
					t.Fatalf("scenario=%s events=%d action_receipts=%d user_messages=%d err=%v", scenario, events, actionReceipts, userMessages, queryErr)
				}
				if scenario == "prior_action_then_disable" {
					var status string
					var partial []byte
					if queryErr := repo.Pool().QueryRow(ctx, `SELECT status,payload->'agent_partial' FROM public.cognition_inbox WHERE fluctlight_id=$1 AND payload->>'turn_id'=$2`, fid, turn).Scan(&status, &partial); queryErr != nil || status != "failed" {
						t.Fatalf("prior action failure status=%s err=%v", status, queryErr)
					}
					found := false
					for _, raw := range arrayValue(decodeObject(partial)["capability_results"]) {
						item := mapValue(raw)
						if stringValue(item["capability_name"]) == "affect_event" && stringValue(item["status"]) == "completed" {
							found = true
						}
					}
					if !found {
						t.Fatalf("prior action receipt was not preserved in failed turn: %s", partial)
					}
				}
			}
		})
	}
}

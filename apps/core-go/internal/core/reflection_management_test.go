package core

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestOwnerReflectionV2ReadsAndFeedbackNativeConsumer(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(fmt.Sprint(failure), func(t *testing.T) {
			f := seedWardrobeToolFixture(t)
			endpoint := "reflection-read-" + f.suffix
			seedCognitiveProviderRole(t, f.ctx, f.repository, endpoint)
			if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy,context_window_tokens,max_input_tokens,prompt_budget_policy_version) SELECT 'reflection',provider_endpoint_id,model_id,'structured_output',token_budget,timeout_seconds,retry_policy,context_window_tokens,max_input_tokens,prompt_budget_policy_version FROM public.model_roles WHERE role='cognitive_assessment'`); err != nil {
				t.Fatal(err)
			}
			if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
				_, err := appendProcessedCognitionFactTx(f.ctx, tx, f.fluctlightID, "conversation.turn", map[string]any{"actor_id": f.ownerID, "conversation_id": f.conversationID, "text": "这是一条受控反思读面测试事实"}, "reflection-read-source")
				return err
			}); err != nil {
				t.Fatal(err)
			}
			reflectionCalls, nativeCalls := 0, 0
			router := newFakeProviderRouter().on("reflection_proposal_v2", func(_ map[string]any) fakeProviderResult {
				reflectionCalls++
				return fakeProviderResult{Structured: reflectionProposalV2Fixture([]any{})}
			}).on("native_cognition_response", func(payload map[string]any) fakeProviderResult {
				nativeCalls++
				if !strings.Contains(jsonString(payload), "FEEDBACK-CONSUMER-1016") {
					t.Error("actual native request lost feedback")
				}
				if failure {
					return fakeProviderResult{Err: errors.New("controlled provider failure")}
				}
				final := nativePersonaFinal()
				delete(final.Structured, "action_type")
				delete(final.Structured, "response_intent")
				for _, key := range []string{"attention", "thought", "desire", "agency"} {
					final.Structured[key] = "已考虑反馈，没有足够证据改变记忆或人格"
				}
				return final
			})
			f.app.Provider.HTTP = &http.Client{Transport: router}
			reflected, err := f.app.ProcessReflection(f.ctx, f.fluctlightID, "reflection-owner-read")
			if err != nil {
				t.Fatal(err)
			}
			id := stringValue(reflected["proposal_id"])
			if id == "" || reflectionCalls != 1 {
				t.Fatalf("no actual proposal %#v calls=%d", reflected, reflectionCalls)
			}
			catalog, err := f.app.OwnerReflectionCatalog(f.ctx, f.ownerID, f.fluctlightID, url.Values{"limit": {"1"}})
			if err != nil || len(arrayValue(catalog["items"])) != 1 || catalog["window"] == nil {
				t.Fatalf("catalog %#v %v", catalog, err)
			}
			detail, err := f.app.OwnerReflection(f.ctx, f.ownerID, f.fluctlightID, id)
			if err != nil || detail["source_window_status"] != "available" || stringValue(mapValue(detail["proposal"])["schema_version"]) != reflectionProposalV2SchemaVersion {
				t.Fatalf("detail %#v %v", detail, err)
			}
			if _, err := f.app.OwnerReflection(f.ctx, f.foreignOwnerID, f.fluctlightID, id); err == nil {
				t.Fatal("foreign reflection exposed")
			}
			path := "/internal/fluctlights/" + f.fluctlightID + "/reflections/" + id + "/feedback"
			body := map[string]any{"text": "FEEDBACK-CONSUMER-1016：我有不同看法，但不要求你强制成长。", "stance": "disagree", "idempotency_key": "reflection-feedback"}
			admitted, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, body)
			if err != nil {
				t.Fatal(err)
			}
			request := mapValue(admitted["request"])
			inbox := stringValue(request["source_fact_id"])
			requestID := stringValue(request["id"])
			if inbox == "" || mapValue(request["payload"])["reflection_id"] != id || nativeCalls != 0 {
				t.Fatalf("admission false success %#v", admitted)
			}
			replay, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, body)
			if err != nil || mapValue(replay["request"])["id"] != requestID {
				t.Fatal("feedback admission replay", err)
			}
			if _, err = f.app.ProcessCognitionInbox(f.ctx, inbox); err != nil {
				t.Fatal(err)
			}
			read, err := f.app.DispatchOwnerSurface(f.ctx, "GET", "/internal/owner-requests/"+requestID, nil, f.ownerID, nil)
			if err != nil {
				t.Fatal(err)
			}
			want := "completed"
			if failure {
				want = "failed"
			}
			if read["status"] != want || nativeCalls != 1 {
				t.Fatalf("feedback consumer status %#v calls=%d", read, nativeCalls)
			}
			count := nativeCalls
			if _, err = f.app.ProcessCognitionInbox(f.ctx, inbox); err != nil || nativeCalls != count {
				t.Fatalf("feedback replay called Provider %d %v", nativeCalls, err)
			}
			feedbacks, err := f.app.OwnerReflectionFeedback(f.ctx, f.ownerID, f.fluctlightID, id, nil)
			if err != nil || len(arrayValue(feedbacks["items"])) != 1 || mapValue(arrayValue(feedbacks["items"])[0])["status"] != want {
				t.Fatalf("feedback readback %#v %v", feedbacks, err)
			}
			// Simulate the final request projection being lost after native settlement.
			if _, err = f.repository.Pool().Exec(f.ctx, `UPDATE public.owner_surface_records SET status='pending',payload=payload||'{"status":"pending"}'::jsonb WHERE id=$1`, requestID); err != nil {
				t.Fatal(err)
			}
			if _, err = f.app.ProcessCognitionInbox(f.ctx, inbox); err != nil || nativeCalls != count {
				t.Fatalf("post-settlement recovery reran model %d %v", nativeCalls, err)
			}
			if _, err = f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/reflections/missing/feedback", nil, f.ownerID, map[string]any{"text": "feedback", "idempotency_key": "missing"}); err == nil {
				t.Fatal("missing proposal feedback accepted")
			}
		})
	}
}

func TestReflectionFeedbackRemainsOpinionInReflectionInput(t *testing.T) {
	rows := compactReflectionEvidenceV2([]map[string]any{{"sequence": 3, "event_type": ownerReflectionFeedbackFactType, "payload": map[string]any{"candidate": map[string]any{"feedback": "明确反馈", "stance": "disagree", "reflection_summary": "旧摘要", "request_id": "private-runtime-id"}}}})
	encoded := jsonString(rows)
	if !strings.Contains(encoded, "明确反馈") || !strings.Contains(encoded, "Opinion about") || strings.Contains(encoded, "private-runtime-id") {
		t.Fatalf("feedback semantics %#v", rows)
	}
}

package core

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestExpressionSuggestionPublicationRechecksOwnerControls(t *testing.T) {
	for _, scenario := range []string{"unchanged", "disabled", "use_switch_aba", "source_changed", "source_aba"} {
		t.Run(scenario, func(t *testing.T) {
			var writerWire string
			router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				if providerWireSchemaName(payload) == expressionLearningSchema {
					return fakeProviderResult{Structured: map[string]any{"candidates": []any{map[string]any{"source_ref": "s1", "excerpt": "慢慢来呀", "situation": "讨论当前进度"}}}}
				}
				writerWire = jsonString(payload)
				return fakeProviderResult{Text: "我们慢慢来。"}
			})
			ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "expression-use-"+scenario, router)
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES('expression-source',$1,1,$2,'user','慢慢来呀','[]','expression-source')`, conversation, owner); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_heads SET next_sequence=2 WHERE conversation_id=$1`, conversation); err != nil {
				t.Fatal(err)
			}
			var personaBefore string
			if err := repo.Pool().QueryRow(ctx, `SELECT core_persona::text FROM public.fluctlights WHERE id=$1`, fid).Scan(&personaBefore); err != nil {
				t.Fatal(err)
			}
			if _, err := app.UpdateOwnerExpressionLearningSettings(ctx, owner, fid, map[string]any{"learning_enabled": true, "use_enabled": true, "expected_revision": 0, "idempotency_key": "use-settings"}); err != nil {
				t.Fatal(err)
			}
			extracted, err := app.ExtractOwnerExpressionCandidates(ctx, owner, fid, map[string]any{"source_message_id": "expression-source", "idempotency_key": "use-extract"})
			if err != nil || len(arrayValue(extracted["candidate_ids"])) != 1 {
				t.Fatalf("extract=%#v err=%v", extracted, err)
			}
			candidateID := stringValue(arrayValue(extracted["candidate_ids"])[0])
			if _, err := app.ReviewOwnerExpressionLearningCandidate(ctx, owner, fid, candidateID, "enable", map[string]any{"expected_revision": 1, "reason": "私有审核理由", "trigger_text": "当前轮", "idempotency_key": "use-enable"}); err != nil {
				t.Fatal(err)
			}
			if selected, err := app.selectExpressionSuggestions(ctx, owner, fid, "another-speaker", conversation, "当前轮的问题"); err != nil || len(selected) != 0 {
				t.Fatalf("other Actor received suggestion: %#v err=%v", selected, err)
			}
			if selected, err := app.selectExpressionSuggestions(ctx, owner, fid, owner, conversation, "不相关的话题"); err != nil || len(selected) != 0 {
				t.Fatalf("unmatched context received suggestion: %#v err=%v", selected, err)
			}
			ctx, run, inv := draftRecoveryInvocation(t, ctx, app, owner, fid, conversation, "expression-turn")
			service := newFactOnlyReplyService(app)
			capability := factOnlyReplyCapability{service: service, run: run, publication: NewToolPublicationService(app)}
			prepared, err := capability.Prepare(ctx, inv, CapabilityContext{Life: &CurrentLifeContext{Data: map[string]any{}}})
			if err != nil {
				t.Fatal(err)
			}
			data, found, err := capabilityPreparedData(prepared, "fact_only_reply")
			if err != nil || !found || len(arrayValue(mapValue(mapValue(data)["draft"])["expression_suggestions"])) != 1 {
				t.Fatalf("draft did not freeze selected suggestion: %#v err=%v", data, err)
			}
			for _, required := range []string{"慢慢来呀", "讨论当前进度"} {
				if !strings.Contains(writerWire, required) {
					t.Fatalf("Writer request lost %q", required)
				}
			}
			for _, forbidden := range []string{candidateID, "source_fingerprint", "私有审核理由", "trigger_text"} {
				if strings.Contains(writerWire, forbidden) {
					t.Fatalf("Core-only expression data entered model request: %s", forbidden)
				}
			}
			switch scenario {
			case "disabled":
				if _, err := app.ReviewOwnerExpressionLearningCandidate(ctx, owner, fid, candidateID, "disable", map[string]any{"expected_revision": 2, "reason": "停止使用", "idempotency_key": "use-disable"}); err != nil {
					t.Fatal(err)
				}
			case "use_switch_aba":
				if _, err := app.UpdateOwnerExpressionLearningSettings(ctx, owner, fid, map[string]any{"learning_enabled": true, "use_enabled": false, "expected_revision": 1, "idempotency_key": "use-off"}); err != nil {
					t.Fatal(err)
				}
				if _, err := app.UpdateOwnerExpressionLearningSettings(ctx, owner, fid, map[string]any{"learning_enabled": true, "use_enabled": true, "expected_revision": 2, "idempotency_key": "use-on-again"}); err != nil {
					t.Fatal(err)
				}
			case "source_changed", "source_aba":
				if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_messages SET text='已撤回原文' WHERE id='expression-source'`); err != nil {
					t.Fatal(err)
				}
				if scenario == "source_aba" {
					if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_messages SET text='慢慢来呀' WHERE id='expression-source'`); err != nil {
						t.Fatal(err)
					}
				}
			}
			target := DirectToolTarget{AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation}
			err = withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
				_, err := capability.ExecuteDirectTx(ctx, tx, prepared, CapabilityContext{}, target)
				return err
			})
			if (err == nil) != (scenario == "unchanged") {
				t.Fatalf("scenario=%s publication error=%v", scenario, err)
			}
			var messages int
			var personaAfter string
			if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'),(SELECT core_persona::text FROM public.fluctlights WHERE id=$2)`, conversation, fid).Scan(&messages, &personaAfter); err != nil {
				t.Fatal(err)
			}
			wantMessages := 0
			if scenario == "unchanged" {
				wantMessages = 1
			}
			if messages != wantMessages || personaAfter != personaBefore {
				t.Fatalf("messages=%d want=%d persona changed=%v", messages, wantMessages, personaAfter != personaBefore)
			}
		})
	}
}

func TestExpressionSuggestionLegacyDraftWithoutBindingsIsAllowed(t *testing.T) {
	if err := (&App{}).validateExpressionSuggestions(context.Background(), "", "", "", "", nil); err != nil {
		t.Fatalf("old draft without suggestions no longer recovers: %v", err)
	}
}

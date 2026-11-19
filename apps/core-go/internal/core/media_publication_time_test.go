package core

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestMediaPublicationTimeRecordsNewLateAttachments(t *testing.T) {
	for _, target := range []string{"message", "moment"} {
		t.Run(target, func(t *testing.T) {
			ctx, repo, app, owner, fid, conversation := setupDirectPublicationToolTest(t, "publication-time-"+target)
			intentID, assetID, id := "publication-time-"+target, "asset_publication-time-"+target, "old-publication-target"
			var intent mediaIntent
			if target == "message" {
				seedDirectMediaEvidence(t, ctx, repo, app, fid, conversation, intentID, "completed", "窗外山景", false)
				if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,idempotency_key,created_at) VALUES($1,$2,77777,$3,'assistant','旧消息',$1,'2020-01-01')`, id, conversation, fid); err != nil {
					t.Fatal(err)
				}
				if _, err := repo.Pool().Exec(ctx, `UPDATE public.media_intents SET message_id=$2 WHERE id=$1`, intentID, id); err != nil {
					t.Fatal(err)
				}
				intent = mediaIntent{ID: intentID, Owner: fid, MessageID: &id}
			} else {
				seedHistoricalMomentImage(t, ctx, repo, app, fid, "publication-time", "窗外山景", false)
				intentID, assetID, id = "history-intent-publication-time", "asset_history-intent-publication-time", "history-moment-publication-time"
				if _, err := repo.Pool().Exec(ctx, `UPDATE public.moments SET owner_actor_id=$2,created_at='2020-01-01' WHERE id=$1`, id, owner); err != nil {
					t.Fatal(err)
				}
				intent = mediaIntent{ID: intentID, Owner: fid, MomentID: &id}
			}
			read := func() directMediaDelivery {
				t.Helper()
				var items []directMediaDelivery
				var err error
				if target == "message" {
					items, err = directMediaDeliveries(ctx, repo.Pool(), owner, owner, fid, conversation, false)
				} else {
					items, err = momentMediaDeliveriesForIntents(ctx, repo.Pool(), owner, fid, nil, false)
				}
				if err != nil || len(items) != 1 {
					t.Fatalf("delivery count=%d err=%v", len(items), err)
				}
				return items[0]
			}
			before := time.Now().Add(-time.Second)
			publishConcurrently := func() {
				t.Helper()
				var wg sync.WaitGroup
				errs := make(chan error, 4)
				for i := 0; i < 4; i++ {
					wg.Add(1)
					go func() { defer wg.Done(); errs <- app.publishMediaAsset(ctx, intent, assetID) }()
				}
				wg.Wait()
				close(errs)
				for err := range errs {
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			publishConcurrently()
			first := read()
			if first.Delivered.Before(before) || first.Delivered.After(time.Now().Add(time.Second)) {
				t.Fatalf("new attachment did not record actual publication time: %#v", first)
			}
			publishConcurrently()
			var refs int
			var attached []byte
			if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.media_references WHERE asset_id=$1 AND target_id=$2`, assetID, id).Scan(&refs); err != nil || refs != 1 {
				t.Fatalf("concurrent publication references=%d err=%v", refs, err)
			}
			statement := `SELECT attachment_refs FROM public.conversation_messages WHERE id=$1`
			if target == "moment" {
				statement = `SELECT media_asset_ids FROM public.moments WHERE id=$1`
			}
			if err := repo.Pool().QueryRow(ctx, statement, id).Scan(&attached); err != nil || len(decodeArray(attached)) != 1 {
				t.Fatalf("concurrent publication duplicated attachments: %s %v", attached, err)
			}
			again := read()
			if !again.Delivered.Equal(first.Delivered) || again.Ref != first.Ref {
				t.Fatal("retry changed publication time or evidence identity")
			}
			if _, err := repo.Pool().Exec(ctx, `UPDATE public.media_references SET published_at=published_at+interval '1 second' WHERE id=$1`, first.refID); err != nil {
				t.Fatal(err)
			}
			if _, err := directMediaDeliveryByRef([]directMediaDelivery{read()}, first.Ref); !errors.Is(err, ErrConflict) {
				t.Fatal("changed timestamp retained old evidence reference")
			}
			// Repairing a missing reference for an already attached image cannot invent time.
			if _, err := repo.Pool().Exec(ctx, `DELETE FROM public.media_references WHERE id=$1`, first.refID); err != nil {
				t.Fatal(err)
			}
			if err := app.publishMediaAsset(ctx, intent, assetID); err != nil {
				t.Fatal(err)
			}
			repaired := read()
			if !repaired.Delivered.IsZero() {
				t.Fatal("reference repair invented historical publication time")
			}
			// Null legacy records remain null on retry.
			if err := app.publishMediaAsset(ctx, intent, assetID); err != nil {
				t.Fatal(err)
			}
			if !read().Delivered.IsZero() {
				t.Fatal("retry backfilled an unknown legacy time")
			}
		})
	}
}

func TestMediaPublicationTimeRollsBackWithAttachment(t *testing.T) {
	ctx, repo, app, _, fid, conversation := setupDirectPublicationToolTest(t, "publication-rollback")
	seedDirectMediaEvidence(t, ctx, repo, app, fid, conversation, "publication-rollback", "completed", "山景", false)
	id := "publication-rollback-message"
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,idempotency_key) VALUES($1,$2,77777,$3,'assistant','旧消息',$1);`, id, conversation, fid); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `CREATE FUNCTION public.fail_media_ref() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'publication_injected_failure'; END $$; CREATE TRIGGER fail_media_ref BEFORE INSERT ON public.media_references FOR EACH ROW EXECUTE FUNCTION public.fail_media_ref()`); err != nil {
		t.Fatal(err)
	}
	if err := app.publishMediaAsset(ctx, mediaIntent{ID: "publication-rollback", Owner: fid, MessageID: &id}, "asset_publication-rollback"); err == nil {
		t.Fatal("publication error lost")
	}
	var attached bool
	var refs int
	if err := repo.Pool().QueryRow(ctx, `SELECT attachment_refs @> '["asset_publication-rollback"]'::jsonb FROM public.conversation_messages WHERE id=$1`, id).Scan(&attached); err != nil {
		t.Fatal(err)
	}
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.media_references WHERE target_id=$1`, id).Scan(&refs); err != nil || attached || refs != 0 {
		t.Fatalf("partial publication survived: attached=%t refs=%d err=%v", attached, refs, err)
	}
}

// The formal draft publisher creates the message and attachment together.
func TestMediaPublicationTimeFormalReplyReference(t *testing.T) {
	ctx, repo, app, owner, fid, conversation := setupDirectPublicationToolTest(t, "publication-formal")
	seedDirectMediaEvidence(t, ctx, repo, app, fid, conversation, "publication-formal", "completed", "山景", false)
	var id string
	if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
		r, e := NewToolPublicationService(app).PublishConversationReplyTx(ctx, tx, ConversationReplyPublication{AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, OperationID: "publication-formal", Text: "给你看看", AttachmentRefs: []string{"asset_publication-formal"}})
		id = r.ID
		return e
	}); err != nil {
		t.Fatal(err)
	}
	var at *time.Time
	if err := repo.Pool().QueryRow(context.Background(), `SELECT published_at FROM public.media_references WHERE target_id=$1`, id).Scan(&at); err != nil || at == nil {
		t.Fatalf("formal publication missing timestamp: %v", err)
	}
}

func TestMediaPublicationTimeDirectReplyRechecksTimestamp(t *testing.T) {
	const text = "我之前把窗外山景照片发给你了。"
	router := newFakeProviderRouter()
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "publication-time-direct-recheck", router)
	seedDirectMediaEvidence(t, ctx, repo, app, fid, conversation, "publication-recheck", "completed", "窗外山景", true)
	router.on(directMediaDeliveryCheckSchema, func(payload map[string]any) fakeProviderResult {
		var view map[string]any
		for _, raw := range arrayValue(payload["messages"]) {
			m := mapValue(raw)
			if m["role"] == "user" {
				view = decodeObject([]byte(stringValue(m["content"])))
			}
		}
		deliveries := arrayValue(view["committed_deliveries"])
		if len(deliveries) != 1 {
			t.Fatalf("expected actual delivery: %#v", deliveries)
		}
		if _, err := repo.Pool().Exec(ctx, `UPDATE public.media_references SET published_at=published_at+interval '1 second' WHERE asset_id='asset_publication-recheck'`); err != nil {
			t.Fatal(err)
		}
		return fakeProviderResult{Structured: map[string]any{"claims": []any{map[string]any{"quote": text, "delivery_ref": mapValue(deliveries[0])["ref"], "subject_quote": "窗外山景"}}}}
	})
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	router.otherwise(func(map[string]any) fakeProviderResult {
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(randomID("reply-time-"), conversationReplyCapabilityName, map[string]any{"text": text, "decision": map[string]any{"action_type": "reply", "response_intent": "回复", "influences": []any{}}})}}
	})
	_, err := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "窗外山景照片发过了吗？", "turn_id": "publication-time-recheck", "idempotency_key": "publication-time-recheck", "attachment_refs": []any{}})
	var messages int
	if e := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND text=$2`, conversation, text).Scan(&messages); e != nil {
		t.Fatal(e)
	}
	if err == nil || !strings.Contains(err.Error(), "direct_media_delivery_unresolved") || messages != 0 {
		t.Fatalf("changed timestamp passed direct publication: messages=%d err=%v", messages, err)
	}
}

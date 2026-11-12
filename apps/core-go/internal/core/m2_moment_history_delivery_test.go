package core

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func seedHistoricalMomentImage(t *testing.T, ctx context.Context, repo *PostgresRepository, app *App, fid, id, intent string, delivered bool) {
	t.Helper()
	momentID, intentID, assetID := "history-moment-"+id, "history-intent-"+id, "asset_history-intent-"+id
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.moments(id,owner_fluctlight_id,author_actor_id,text,visibility,status,media_asset_ids) VALUES($1,$2,$2,'旧动态','participants','visible','[]')`, momentID, fid); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,moment_id,status) VALUES($1,$2,'image','image/png',$3,$4,$5,$6,'completed')`, intentID, fid, jsonString(map[string]any{"intent": intent}), "history-provider-"+id, "history-workflow-"+id, momentID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.media_assets(id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status) VALUES($1,$2,'v1','image','image/png',1,'fixture','fixture','fixture',$3,$4,'ready')`, assetID, fid, "history-provider-"+id, "history-workflow-"+id); err != nil {
		t.Fatal(err)
	}
	if delivered {
		if err := app.publishMediaAsset(ctx, mediaIntent{ID: intentID, Owner: fid, MomentID: &momentID}, assetID); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMomentHistoricalEvidenceRequiresScopedCommittedImage(t *testing.T) {
	ctx, repo, app, owner, fid, _ := setupMixedMediaReplyTurnWithTransport(t, "m2-history-evidence", nil)
	seedHistoricalMomentImage(t, ctx, repo, app, fid, "actual", "窗外日落", true)
	seedHistoricalMomentImage(t, ctx, repo, app, fid, "ready-only", "未交付的山", false)
	seedHistoricalMomentImage(t, ctx, repo, app, fid, "failed-but-delivered", "已附图的湖", true)
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.media_intents SET status='failed' WHERE id='history-intent-failed-but-delivered'`); err != nil {
		t.Fatal(err)
	}
	seedHistoricalMomentImage(t, ctx, repo, app, fid, "hidden", "隐藏的海", true)
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.moments SET status='hidden' WHERE id='history-moment-hidden'`); err != nil {
		t.Fatal(err)
	}
	seedHistoricalMomentImage(t, ctx, repo, app, fid, "owner-post", "用户动态的猫", true)
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.moments SET author_actor_id=$1 WHERE id='history-moment-owner-post'`, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.media_assets(id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status) VALUES('history-unattributed',$1,'v1','image','image/png',1,'fixture','fixture','fixture','history-unattributed-provider','history-unattributed-workflow','ready')`, fid); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.moments(id,owner_fluctlight_id,author_actor_id,text,visibility,status,media_asset_ids) VALUES('history-unattributed-moment',$1,$1,'旧动态','participants','visible','["history-unattributed"]')`, fid); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.media_references(id,asset_id,owner_fluctlight_id,target_type,target_id) VALUES('history-unattributed-ref','history-unattributed',$1,'moment','history-unattributed-moment')`, fid); err != nil {
		t.Fatal(err)
	}
	deliveries, err := momentMediaDeliveriesForIntents(ctx, repo.Pool(), owner, fid, nil, false)
	seen := map[string]string{}
	for _, delivery := range deliveries {
		seen[delivery.intentID] = delivery.Intent
		if !delivery.Delivered.IsZero() || strings.Contains(delivery.Ref, delivery.intentID) {
			t.Fatalf("historical Moment evidence exposed an ID or invented attachment time: %#v", delivery)
		}
	}
	if err != nil || len(deliveries) != 2 || seen["history-intent-actual"] != "窗外日落" || seen["history-intent-failed-but-delivered"] != "已附图的湖" {
		t.Fatalf("wrong historical Moment evidence: %#v err=%v", deliveries, err)
	}
	foreign, err := momentMediaDeliveriesForIntents(ctx, repo.Pool(), "foreign-owner", fid, nil, false)
	if err != nil || len(foreign) != 0 {
		t.Fatalf("foreign Owner obtained historical evidence: %#v err=%v", foreign, err)
	}
}

func TestMomentHistoricalEvidenceIsRecheckedAndLockBusyIsRetryable(t *testing.T) {
	for _, scenario := range []string{"removed", "busy"} {
		t.Run(scenario, func(t *testing.T) {
			const text = "我之前已经把窗外日落照片发在另一条动态里。"
			router := newFakeProviderRouter()
			ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m2-history-"+scenario, router)
			seedHistoricalMomentImage(t, ctx, repo, app, fid, scenario, "窗外日落", true)
			var locked pgx.Tx
			defer func() {
				if locked != nil {
					_ = locked.Rollback(ctx)
				}
			}()
			router.on(directMediaDeliveryCheckSchema, func(payload map[string]any) fakeProviderResult {
				var view map[string]any
				for _, raw := range arrayValue(payload["messages"]) {
					message := mapValue(raw)
					if message["role"] == "user" {
						view = decodeObject([]byte(stringValue(message["content"])))
					}
				}
				deliveries := arrayValue(view["committed_deliveries"])
				if len(deliveries) != 1 || mapValue(deliveries[0])["delivery_scope"] != "moment" {
					t.Fatal("matching Moment evidence was not offered")
				}
				if scenario == "removed" {
					if _, err := repo.Pool().Exec(ctx, `DELETE FROM public.media_references WHERE asset_id=$1`, "asset_history-intent-"+scenario); err != nil {
						t.Fatal(err)
					}
				} else {
					var err error
					locked, err = repo.Pool().Begin(ctx)
					if err != nil {
						t.Fatal(err)
					}
					var id string
					if err := locked.QueryRow(ctx, `SELECT id FROM public.moments WHERE id=$1 FOR UPDATE`, "history-moment-"+scenario).Scan(&id); err != nil {
						t.Fatal(err)
					}
				}
				return fakeProviderResult{Structured: map[string]any{"claims": []any{map[string]any{"quote": text, "delivery_ref": mapValue(deliveries[0])["ref"], "subject_quote": "窗外日落"}}}}
			}).otherwise(func(map[string]any) fakeProviderResult {
				return fakeProviderResult{Text: text}
			})
			request := ToolExecutionRequest{CapabilityName: "moment.publish", OperationID: "m2-history-" + scenario, AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, Arguments: jsonBytes(map[string]any{})}
			receipt, err := app.ExecuteTool(ctx, request)
			var count int
			if queryErr := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.moments WHERE owner_fluctlight_id=$1`, fid).Scan(&count); queryErr != nil {
				t.Fatal(queryErr)
			}
			if scenario == "removed" {
				if err != nil || receipt.Result.Status != "handled" || stringValue(mapValue(receipt.Result.Output)["warning_code"]) != "direct_media_delivery_stale" || count != 1 {
					t.Fatalf("removed Moment evidence was accepted: %#v count=%d err=%v", receipt.Result, count, err)
				}
			} else if err == nil || receipt.Result.ErrorCode != "direct_media_delivery_busy" || !receipt.Result.Retryable || count != 1 {
				t.Fatalf("locked Moment evidence was not retryable: %#v count=%d err=%v", receipt.Result, count, err)
			}
		})
	}
}

func TestMomentHistoricalImageClaimUsesActualMomentDelivery(t *testing.T) {
	const text = "我之前已经把窗外日落照片发在另一条动态里。"
	var sawMomentEvidence bool
	router := newFakeProviderRouter().on(directMediaDeliveryCheckSchema, func(payload map[string]any) fakeProviderResult {
		var view map[string]any
		for _, raw := range arrayValue(payload["messages"]) {
			message := mapValue(raw)
			if message["role"] == "user" {
				view = decodeObject([]byte(stringValue(message["content"])))
			}
		}
		ref := ""
		for _, raw := range arrayValue(view["committed_deliveries"]) {
			delivery := mapValue(raw)
			if delivery["delivery_scope"] == "moment" && delivery["intent"] == "窗外日落" && delivery["delivered_at"] == nil {
				ref = stringValue(delivery["ref"])
				sawMomentEvidence = ref != ""
			}
		}
		return fakeProviderResult{Structured: map[string]any{"claims": []any{map[string]any{"quote": text, "delivery_ref": ref, "subject_quote": "窗外日落"}}}}
	}).otherwise(func(map[string]any) fakeProviderResult {
		return fakeProviderResult{Text: text}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m2-moment-history", router)
	seedHistoricalMomentImage(t, ctx, repo, app, fid, "actual", "窗外日落", true)
	request := ToolExecutionRequest{CapabilityName: "moment.publish", OperationID: "m2-moment-history", AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, Arguments: jsonBytes(map[string]any{})}
	receipt, err := app.ExecuteTool(ctx, request)
	var moments int
	if queryErr := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.moments WHERE owner_fluctlight_id=$1`, fid).Scan(&moments); queryErr != nil {
		t.Fatal(queryErr)
	}
	if err != nil || receipt.Result.Status != "completed" || !sawMomentEvidence || moments != 2 {
		t.Fatalf("matching historical Moment delivery was not usable: %#v evidence=%t moments=%d err=%v", receipt.Result, sawMomentEvidence, moments, err)
	}
}

func TestMomentHistoricalClaimCannotBorrowWrongDelivery(t *testing.T) {
	for _, sample := range []struct {
		name, text, source string
	}{
		{"current-post", "窗外日落照片在这条动态里。", "moment"},
		{"private-claim", "我之前把窗外日落照片发给你了。", "moment"},
		{"moment-claim", "我之前把窗外日落照片发在另一条动态里。", "conversation"},
		{"old-moment-claim", "我之前把窗外日落照片发在旧动态里。", "conversation"},
		{"unknown-delivery-time", "昨天我把窗外日落照片发在另一条动态里。", "moment"},
		{"wrong-subject", "我之前把海边照片发在另一条动态里。", "moment"},
		{"ready-only", "我之前把窗外日落照片发在另一条动态里。", "ready"},
	} {
		t.Run(sample.name, func(t *testing.T) {
			router := newFakeProviderRouter().on(directMediaDeliveryCheckSchema, func(payload map[string]any) fakeProviderResult {
				var view map[string]any
				for _, raw := range arrayValue(payload["messages"]) {
					message := mapValue(raw)
					if message["role"] == "user" {
						view = decodeObject([]byte(stringValue(message["content"])))
					}
				}
				ref := ""
				if deliveries := arrayValue(view["committed_deliveries"]); len(deliveries) > 0 {
					ref = stringValue(mapValue(deliveries[0])["ref"])
				}
				return fakeProviderResult{Structured: map[string]any{"claims": []any{map[string]any{"quote": sample.text, "delivery_ref": ref, "subject_quote": "窗外日落"}}}}
			}).otherwise(func(map[string]any) fakeProviderResult {
				return fakeProviderResult{Text: sample.text}
			})
			ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m2-history-wrong-"+sample.name, router)
			priorMoments := 1
			if sample.source == "conversation" {
				priorMoments = 0
				seedDirectMediaEvidence(t, ctx, repo, app, fid, conversation, "m2-history-chat-"+sample.name, "completed", "窗外日落", true)
			} else {
				seedHistoricalMomentImage(t, ctx, repo, app, fid, sample.name, "窗外日落", sample.source == "moment")
			}
			request := ToolExecutionRequest{CapabilityName: "moment.publish", OperationID: "m2-history-wrong-" + sample.name, AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, Arguments: jsonBytes(map[string]any{})}
			receipt, err := app.ExecuteTool(ctx, request)
			var moments int
			if queryErr := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.moments WHERE owner_fluctlight_id=$1`, fid).Scan(&moments); queryErr != nil {
				t.Fatal(queryErr)
			}
			if err != nil || receipt.Result.Status != "handled" || stringValue(mapValue(receipt.Result.Output)["warning_code"]) != "direct_media_delivery_unverified" || moments != priorMoments {
				t.Fatalf("wrong historical delivery was accepted: %#v moments=%d err=%v", receipt.Result, moments, err)
			}
		})
	}
}

func TestMomentHistoricalClaimCannotAddUnverifiedSubject(t *testing.T) {
	const text = "我之前把白猫和黑猫的照片发在另一条动态里。"
	router := newFakeProviderRouter().on(directMediaDeliveryCheckSchema, func(payload map[string]any) fakeProviderResult {
		var view map[string]any
		for _, raw := range arrayValue(payload["messages"]) {
			message := mapValue(raw)
			if message["role"] == "user" {
				view = decodeObject([]byte(stringValue(message["content"])))
			}
		}
		deliveries := arrayValue(view["committed_deliveries"])
		if len(deliveries) != 1 {
			t.Fatal("actual historical Moment was not offered")
		}
		return fakeProviderResult{Structured: map[string]any{"claims": []any{map[string]any{"quote": text, "delivery_ref": mapValue(deliveries[0])["ref"], "subject_quote": "白猫"}}}}
	}).otherwise(func(map[string]any) fakeProviderResult {
		return fakeProviderResult{Text: text}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m2-history-extra-subject", router)
	seedHistoricalMomentImage(t, ctx, repo, app, fid, "white-cat", "白猫", true)
	request := ToolExecutionRequest{CapabilityName: "moment.publish", OperationID: "m2-history-extra-subject", AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, Arguments: jsonBytes(map[string]any{})}
	receipt, err := app.ExecuteTool(ctx, request)
	var moments int
	if queryErr := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.moments WHERE owner_fluctlight_id=$1`, fid).Scan(&moments); queryErr != nil {
		t.Fatal(queryErr)
	}
	if err != nil || receipt.Result.Status != "handled" || stringValue(mapValue(receipt.Result.Output)["warning_code"]) != "direct_media_delivery_unverified" || moments != 1 {
		t.Fatalf("additional subject borrowed historical proof: %#v moments=%d err=%v", receipt.Result, moments, err)
	}
}

func TestMomentHistoricalClaimIsRejectedWhenExtractorMissesIt(t *testing.T) {
	const text = "昨天我把窗外日落照片发在另一条动态里。"
	router := newFakeProviderRouter().on(directMediaDeliveryCheckSchema, func(map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: map[string]any{"claims": []any{}}}
	}).otherwise(func(map[string]any) fakeProviderResult {
		return fakeProviderResult{Text: text}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m2-history-missed-claim", router)
	request := ToolExecutionRequest{CapabilityName: "moment.publish", OperationID: "m2-history-missed-claim", AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, Arguments: jsonBytes(map[string]any{})}
	receipt, err := app.ExecuteTool(ctx, request)
	var moments int
	if queryErr := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.moments WHERE owner_fluctlight_id=$1`, fid).Scan(&moments); queryErr != nil {
		t.Fatal(queryErr)
	}
	if err != nil || receipt.Result.Status != "handled" || stringValue(mapValue(receipt.Result.Output)["warning_code"]) != "direct_media_delivery_unverified" || moments != 0 {
		t.Fatalf("missed historical claim was published: %#v moments=%d err=%v", receipt.Result, moments, err)
	}
}

func TestMomentCannotInventTimeForLatePrivateAttachment(t *testing.T) {
	const text = "昨天我把窗外日落照片发给你了。"
	router := newFakeProviderRouter().on(directMediaDeliveryCheckSchema, func(payload map[string]any) fakeProviderResult {
		var view map[string]any
		for _, raw := range arrayValue(payload["messages"]) {
			message := mapValue(raw)
			if message["role"] == "user" {
				view = decodeObject([]byte(stringValue(message["content"])))
			}
		}
		deliveries := arrayValue(view["committed_deliveries"])
		if len(deliveries) != 1 || mapValue(deliveries[0])["delivered_at"] != nil {
			t.Fatalf("late attachment time must remain unknown: %#v", deliveries)
		}
		return fakeProviderResult{Structured: map[string]any{"claims": []any{map[string]any{"quote": text, "delivery_ref": mapValue(deliveries[0])["ref"], "subject_quote": "窗外日落"}}}}
	}).otherwise(func(map[string]any) fakeProviderResult {
		return fakeProviderResult{Text: text}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m2-history-late-private", router)
	const intentID, messageID = "m2-history-late-private-intent", "m2-history-late-private-message"
	seedDirectMediaEvidence(t, ctx, repo, app, fid, conversation, intentID, "completed", "窗外日落", false)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key,created_at) VALUES($1,$2,77777,$3,'assistant','旧消息','[]'::jsonb,$1,'2020-01-01T00:00:00Z')`, messageID, conversation, fid); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.media_intents SET message_id=$2 WHERE id=$1`, intentID, messageID); err != nil {
		t.Fatal(err)
	}
	targetMessage := messageID
	if err := app.publishMediaAsset(ctx, mediaIntent{ID: intentID, Owner: fid, MessageID: &targetMessage}, "asset_"+intentID); err != nil {
		t.Fatal(err)
	}
	request := ToolExecutionRequest{CapabilityName: "moment.publish", OperationID: "m2-history-late-private", AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, Arguments: jsonBytes(map[string]any{})}
	receipt, err := app.ExecuteTool(ctx, request)
	var moments int
	if queryErr := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.moments WHERE owner_fluctlight_id=$1`, fid).Scan(&moments); queryErr != nil {
		t.Fatal(queryErr)
	}
	if err != nil || receipt.Result.Status != "handled" || stringValue(mapValue(receipt.Result.Output)["warning_code"]) != "direct_media_delivery_unverified" || moments != 0 {
		t.Fatalf("unknown private attachment time became a precise claim: %#v moments=%d err=%v", receipt.Result, moments, err)
	}
}

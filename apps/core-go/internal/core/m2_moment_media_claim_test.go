package core

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
)

// The Provider is controlled; the database and independent Moment Tool path
// are real. A text claim cannot create an attachment or prove delivery.
func TestMomentWithoutMediaCannotPublishCompletedPhotoClaim(t *testing.T) {
	checkerCalls := 0
	router := newFakeProviderRouter().on(directMediaDeliveryCheckSchema, func(payload map[string]any) fakeProviderResult {
		checkerCalls++
		if !strings.Contains(jsonString(payload["messages"]), "动态正文") {
			t.Error("the checker was not given the Moment scope")
		}
		return fakeProviderResult{Structured: map[string]any{"claims": []any{map[string]any{"quote": "我已经把这张照片附在这条动态里了。", "delivery_ref": "", "subject_quote": ""}}}}
	}).otherwise(func(map[string]any) fakeProviderResult {
		return fakeProviderResult{Text: "我已经把这张照片附在这条动态里了。"}
	})
	ctx, repo, app, owner, fid, _ := setupMixedMediaReplyTurnWithTransport(t, "m2-moment-false-photo", router)
	role := agentModelBindingKey(FormalAgentConversationReply, "moment")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,timeout_seconds) VALUES($1,'mixed-endpoint-m2-moment-false-photo','m2-moment-claim-model',30)`, role); err != nil {
		t.Fatal(err)
	}
	request := ToolExecutionRequest{CapabilityName: "moment.publish", OperationID: "m2-moment-false-photo", AuthorizationActorID: owner, FluctlightID: fid, Arguments: jsonBytes(map[string]any{})}
	receipt, err := app.ExecuteTool(ctx, request)
	var moments, attachments int
	if queryErr := repo.Pool().QueryRow(ctx, `SELECT count(*),COALESCE(sum(jsonb_array_length(media_asset_ids)),0) FROM public.moments WHERE owner_fluctlight_id=$1`, fid).Scan(&moments, &attachments); queryErr != nil {
		t.Fatal(queryErr)
	}
	if err != nil || receipt.Result.Status != "handled" || stringValue(mapValue(receipt.Result.Output)["warning_code"]) != "direct_media_delivery_unverified" || moments != 0 || attachments != 0 || checkerCalls != 1 {
		t.Fatalf("unsupported photo completion was published: receipt=%#v err=%v moments=%d attachments=%d checks=%d", receipt.Result, err, moments, attachments, checkerCalls)
	}
}

func TestMomentExplicitAttachmentClaimIsRejectedWhenExtractorMissesIt(t *testing.T) {
	router := newFakeProviderRouter().on(directMediaDeliveryCheckSchema, func(map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: map[string]any{"claims": []any{}}}
	}).otherwise(func(map[string]any) fakeProviderResult {
		return fakeProviderResult{Text: "我已经把这张照片附在这条动态里了。"}
	})
	ctx, repo, app, owner, fid, _ := setupMixedMediaReplyTurnWithTransport(t, "m2-moment-missed-claim", router)
	request := ToolExecutionRequest{CapabilityName: "moment.publish", OperationID: "m2-moment-missed-claim", AuthorizationActorID: owner, FluctlightID: fid, Arguments: jsonBytes(map[string]any{})}
	receipt, err := app.ExecuteTool(ctx, request)
	var moments int
	if queryErr := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.moments WHERE owner_fluctlight_id=$1`, fid).Scan(&moments); queryErr != nil {
		t.Fatal(queryErr)
	}
	if err != nil || receipt.Result.Status != "handled" || stringValue(mapValue(receipt.Result.Output)["warning_code"]) != "direct_media_delivery_unverified" || moments != 0 {
		t.Fatalf("missed explicit claim was published: %#v moments=%d err=%v", receipt.Result, moments, err)
	}
}

func TestMomentMediaRejectionCountsPhysicalRounds(t *testing.T) {
	trace := &ADKCapabilityTrace{}
	invoker := &appADKCapabilityInvoker{trace: trace, scopeRegistry: mustCapabilityRegistry(builtinCapabilities(nil)...)}
	ctx := withADKCapabilityContext(context.Background(), invoker, trace, nil)
	appendRound := func(sequence uint64, count int) {
		calls := make([]schema.ToolCall, 0, count)
		for i := 0; i < count; i++ {
			calls = append(calls, schema.ToolCall{ID: fmt.Sprintf("moment-%d-%d", sequence, i), Function: schema.FunctionCall{Name: "moment.publish"}})
		}
		trace.RecordModelToolCalls("moment-provider", sequence, calls)
		for i, call := range calls {
			trace.Append(CapabilityInvocation{CallID: call.ID, CapabilityName: "moment.publish", Arguments: jsonBytes(map[string]any{"different": i})}, CapabilityResult{CallID: call.ID, CapabilityName: "moment.publish", Status: "handled", Output: map[string]any{"effect_applied": false, "warning_code": "direct_media_delivery_unverified"}})
		}
	}
	appendRound(1, 3)
	if conversationLoopNeedsReply(ctx) || conversationLoopNeedsReplyBeforeModelSequence(ctx, 1) {
		t.Fatal("calls in one response were counted as separate feedback rounds")
	}
	appendRound(2, 1)
	if conversationLoopNeedsReply(ctx) {
		t.Fatal("two rejected rounds exhausted the bound")
	}
	appendRound(3, 1)
	if !conversationLoopNeedsReply(ctx) || conversationLoopNeedsReplyBeforeModelSequence(ctx, 3) {
		t.Fatal("the third result did not apply only after its model response")
	}
}

func TestMomentWithOwnerSelectedAttachmentRemainsPublishable(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		t.Run(fmt.Sprintf("mixed-%t", mixed), func(t *testing.T) {
			checks := 0
			router := newFakeProviderRouter().on(directMediaDeliveryCheckSchema, func(map[string]any) fakeProviderResult {
				checks++
				return fakeProviderResult{Status: 500}
			}).otherwise(func(map[string]any) fakeProviderResult {
				return fakeProviderResult{Text: "我已经把这张照片附在这条动态里了。"}
			})
			ctx, repo, app, owner, fid, _ := setupMixedMediaReplyTurnWithTransport(t, "m2-moment-owner-media", router)
			app.Storage = &minio.Client{}
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.media_assets(id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status) VALUES('m2-owner-asset',$1,'v1','image','image/png',1,'fixture','fixture','fixture','m2-owner-provider','m2-owner-workflow','ready')`, fid); err != nil {
				t.Fatal(err)
			}
			selected := []any{"m2-owner-asset"}
			if mixed {
				if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.media_assets(id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status) VALUES('m2-owner-audio',$1,'v1','audio','audio/mpeg',1,'fixture','fixture','fixture','m2-audio-provider','m2-audio-workflow','ready')`, fid); err != nil {
					t.Fatal(err)
				}
				selected = append(selected, "m2-owner-audio")
			}
			payload := map[string]any{"kind": "moment_suggestion", "status": "pending", "source_fact_id": "m2-owner-source", "payload": map[string]any{"media_ids": selected}}
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.owner_surface_records(id,owner_actor_id,fluctlight_id,kind,resource_key,status,payload) VALUES('m2-owner-request',$1,$2,'owner_request','m2-owner-request','pending',$3)`, owner, fid, jsonBytes(payload)); err != nil {
				t.Fatal(err)
			}
			request := ToolExecutionRequest{CapabilityName: "moment.publish", OperationID: "m2-owner-moment", AuthorizationActorID: owner, FluctlightID: fid, EvidenceID: "m2-owner-source", TargetKind: "owner_moment_request", TargetRef: "m2-owner-request", Arguments: jsonBytes(map[string]any{})}
			receipt, err := app.ExecuteTool(ctx, request)
			if err != nil || receipt.Result.Status != "completed" || checks != 0 {
				t.Fatalf("Moment with selected attachment: %#v checks=%d err=%v", receipt.Result, checks, err)
			}
			id := stringValue(mapValue(receipt.Result.Output)["target_ref"])
			var assets []byte
			var refs int
			if err := repo.Pool().QueryRow(ctx, `SELECT media_asset_ids,(SELECT count(*) FROM public.media_references WHERE target_type='moment' AND target_id=$1) FROM public.moments WHERE id=$1`, id).Scan(&assets, &refs); err != nil || jsonString(decodeArray(assets)) != jsonString(selected) || refs != len(selected) {
				t.Fatalf("selected attachment was not committed: assets=%s refs=%d err=%v", assets, refs, err)
			}
		})
	}
}

func TestMomentAudioAttachmentDoesNotProvePhoto(t *testing.T) {
	for _, sample := range []struct {
		name, text, want string
		publish          bool
	}{
		{"false-claim", "我已经把这张照片附在这条动态里了。", "", false},
		{"placeholder", "今天听歌。\n[自拍照：站在窗边]", "今天听歌。", true},
		{"audio-discussion", "我把这段音频附在这条动态里，聊聊照片。", "我把这段音频附在这条动态里，聊聊照片。", true},
	} {
		t.Run(sample.name, func(t *testing.T) {
			router := newFakeProviderRouter().on(directMediaDeliveryCheckSchema, func(map[string]any) fakeProviderResult {
				claims := []any{}
				if !sample.publish {
					claims = append(claims, map[string]any{"quote": sample.text, "delivery_ref": "", "subject_quote": ""})
				}
				return fakeProviderResult{Structured: map[string]any{"claims": claims}}
			}).otherwise(func(map[string]any) fakeProviderResult {
				return fakeProviderResult{Text: sample.text}
			})
			ctx, repo, app, owner, fid, _ := setupMixedMediaReplyTurnWithTransport(t, "m2-moment-audio-"+sample.name, router)
			app.Storage = &minio.Client{}
			assetID, requestID, sourceID := "m2-audio-"+sample.name, "m2-audio-request-"+sample.name, "m2-audio-source-"+sample.name
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.media_assets(id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status) VALUES($1,$2,'v1','audio','audio/mpeg',1,'fixture','fixture','fixture',$3,$4,'ready')`, assetID, fid, "provider-"+sample.name, "workflow-"+sample.name); err != nil {
				t.Fatal(err)
			}
			payload := map[string]any{"kind": "moment_suggestion", "status": "pending", "source_fact_id": sourceID, "payload": map[string]any{"media_ids": []any{assetID}}}
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.owner_surface_records(id,owner_actor_id,fluctlight_id,kind,resource_key,status,payload) VALUES($1,$2,$3,'owner_request',$1,'pending',$4)`, requestID, owner, fid, jsonBytes(payload)); err != nil {
				t.Fatal(err)
			}
			request := ToolExecutionRequest{CapabilityName: "moment.publish", OperationID: "m2-audio-moment-" + sample.name, AuthorizationActorID: owner, FluctlightID: fid, EvidenceID: sourceID, TargetKind: "owner_moment_request", TargetRef: requestID, Arguments: jsonBytes(map[string]any{})}
			receipt, err := app.ExecuteTool(ctx, request)
			var moments int
			if queryErr := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.moments WHERE owner_fluctlight_id=$1`, fid).Scan(&moments); queryErr != nil {
				t.Fatal(queryErr)
			}
			if !sample.publish {
				if err != nil || receipt.Result.Status != "handled" || stringValue(mapValue(receipt.Result.Output)["warning_code"]) != "direct_media_delivery_unverified" || moments != 0 {
					t.Fatalf("audio authorized a photo claim: %#v moments=%d err=%v", receipt.Result, moments, err)
				}
				return
			}
			text := stringValue(mapValue(receipt.Result.Output)["text"])
			if err != nil || receipt.Result.Status != "completed" || moments != 1 || text != sample.want {
				t.Fatalf("audio retained a photo placeholder: %#v moments=%d err=%v", receipt.Result, moments, err)
			}
		})
	}
}

func TestMomentSelectedAttachmentChangeRollsBackPublication(t *testing.T) {
	for _, change := range []string{"selection", "empty-to-image", "image-kind", "audio-to-image", "version"} {
		t.Run(change, func(t *testing.T) {
			router := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
				if change == "empty-to-image" || change == "audio-to-image" {
					return fakeProviderResult{Text: "今天的动态。"}
				}
				return fakeProviderResult{Text: "我已经把这张照片附在这条动态里了。"}
			})
			ctx, repo, app, owner, fid, _ := setupMixedMediaReplyTurnWithTransport(t, "m2-moment-owner-stale", router)
			app.Storage = &minio.Client{}
			kind, mime := "image", "image/png"
			if change == "audio-to-image" {
				kind, mime = "audio", "audio/mpeg"
			}
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.media_assets(id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status) VALUES('m2-owner-stale-asset',$1,'v1',$2,$3,1,'fixture','fixture','fixture','m2-owner-stale-provider','m2-owner-stale-workflow','ready')`, fid, kind, mime); err != nil {
				t.Fatal(err)
			}
			selected := []any{"m2-owner-stale-asset"}
			if change == "empty-to-image" {
				selected = []any{}
			}
			payload := map[string]any{"kind": "moment_suggestion", "status": "pending", "source_fact_id": "m2-owner-stale-source", "payload": map[string]any{"media_ids": selected}}
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.owner_surface_records(id,owner_actor_id,fluctlight_id,kind,resource_key,status,payload) VALUES('m2-owner-stale-request',$1,$2,'owner_request','m2-owner-stale-request','pending',$3)`, owner, fid, jsonBytes(payload)); err != nil {
				t.Fatal(err)
			}
			scope := ToolExecutionRequest{CapabilityName: "moment.publish", OperationID: "m2-owner-stale", AuthorizationActorID: owner, FluctlightID: fid, EvidenceID: "m2-owner-stale-source", TargetKind: "owner_moment_request", TargetRef: "m2-owner-stale-request", Arguments: jsonBytes(map[string]any{})}
			ctx, err := app.withMomentWriterRun(ctx, scope)
			if err != nil {
				t.Fatal(err)
			}
			service := ctx.Value(momentWriterRunKey{}).(*factOnlyMomentService)
			capability := factOnlyMomentCapability{service: service}
			inv := CapabilityInvocation{CallID: "m2-owner-stale-call", CapabilityName: "moment.publish", SourceFactID: scope.EvidenceID, Arguments: scope.Arguments, Metadata: InvocationMetadata{AuthorizationActorID: owner, SubjectActorID: owner, FluctlightID: fid, OperationID: scope.OperationID}}
			prepared, err := capability.Prepare(ctx, inv, CapabilityContext{})
			if err != nil {
				t.Fatal(err)
			}
			if change == "selection" || change == "empty-to-image" {
				if change == "selection" {
					mapValue(payload["payload"])["media_ids"] = []any{}
				} else {
					mapValue(payload["payload"])["media_ids"] = []any{"m2-owner-stale-asset"}
				}
				if _, err := repo.Pool().Exec(ctx, `UPDATE public.owner_surface_records SET payload=$2 WHERE id=$1`, scope.TargetRef, jsonBytes(payload)); err != nil {
					t.Fatal(err)
				}
			} else {
				query := `UPDATE public.media_assets SET kind='audio',mime_type='audio/mpeg',revision=revision+1 WHERE id='m2-owner-stale-asset'`
				if change == "audio-to-image" {
					query = `UPDATE public.media_assets SET kind='image',mime_type='image/png',revision=revision+1 WHERE id='m2-owner-stale-asset'`
				} else if change == "version" {
					query = `UPDATE public.media_assets SET version='v2' WHERE id='m2-owner-stale-asset'`
				}
				if _, err := repo.Pool().Exec(ctx, query); err != nil {
					t.Fatal(err)
				}
			}
			target := DirectToolTarget{Kind: scope.TargetKind, Ref: scope.TargetRef, AuthorizationActorID: owner, FluctlightID: fid}
			err = withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
				_, err := capability.ExecuteDirectTx(ctx, tx, prepared, CapabilityContext{}, target)
				return err
			})
			var moments int
			if queryErr := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.moments WHERE owner_fluctlight_id=$1`, fid).Scan(&moments); queryErr != nil {
				t.Fatal(queryErr)
			}
			if err == nil || moments != 0 {
				t.Fatalf("changed selected attachment published the Moment: moments=%d err=%v", moments, err)
			}
		})
	}
}

func TestUnattachedMomentRechecksHistoricalDeliveryInTransaction(t *testing.T) {
	router := newFakeProviderRouter()
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m2-moment-history-stale", router)
	seedDirectMediaEvidence(t, ctx, repo, app, fid, conversation, "m2-history-stale", "completed", "窗外日落", true)
	router.on(directMediaDeliveryCheckSchema, func(payload map[string]any) fakeProviderResult {
		var view map[string]any
		for _, raw := range arrayValue(payload["messages"]) {
			message := mapValue(raw)
			if message["role"] == "user" {
				view = decodeObject([]byte(stringValue(message["content"])))
			}
		}
		deliveries := arrayValue(view["committed_deliveries"])
		if len(deliveries) != 1 {
			t.Fatal("the checker did not receive the committed historical delivery")
		}
		if _, err := repo.Pool().Exec(ctx, `DELETE FROM public.media_references WHERE asset_id='asset_m2-history-stale'`); err != nil {
			t.Fatal(err)
		}
		return fakeProviderResult{Structured: map[string]any{"claims": []any{map[string]any{"quote": "我之前已经把窗外日落照片发给你了。", "delivery_ref": mapValue(deliveries[0])["ref"], "subject_quote": "窗外日落"}}}}
	}).otherwise(func(map[string]any) fakeProviderResult {
		return fakeProviderResult{Text: "我之前已经把窗外日落照片发给你了。"}
	})
	request := ToolExecutionRequest{CapabilityName: "moment.publish", OperationID: "m2-moment-history-stale", AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, Arguments: jsonBytes(map[string]any{})}
	receipt, err := app.ExecuteTool(ctx, request)
	var moments int
	if queryErr := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.moments WHERE owner_fluctlight_id=$1`, fid).Scan(&moments); queryErr != nil {
		t.Fatal(queryErr)
	}
	if err != nil || receipt.Result.Status != "handled" || stringValue(mapValue(receipt.Result.Output)["warning_code"]) != "direct_media_delivery_stale" || moments != 0 {
		t.Fatalf("stale delivery authorized Moment publication: %#v moments=%d err=%v", receipt.Result, moments, err)
	}
}

func TestMomentHistoricalDeliveryLockBusyIsRetryable(t *testing.T) {
	router := newFakeProviderRouter()
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m2-moment-history-busy", router)
	seedDirectMediaEvidence(t, ctx, repo, app, fid, conversation, "m2-history-busy", "completed", "窗外日落", true)
	var lockTx pgx.Tx
	defer func() {
		if lockTx != nil {
			_ = lockTx.Rollback(ctx)
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
		if len(deliveries) != 1 {
			t.Fatal("the checker did not receive the historical delivery")
		}
		var err error
		lockTx, err = repo.Pool().Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var id string
		if err := lockTx.QueryRow(ctx, `SELECT id FROM public.media_intents WHERE id='m2-history-busy' FOR UPDATE`).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return fakeProviderResult{Structured: map[string]any{"claims": []any{map[string]any{"quote": "我之前已经把窗外日落照片发给你了。", "delivery_ref": mapValue(deliveries[0])["ref"], "subject_quote": "窗外日落"}}}}
	}).otherwise(func(map[string]any) fakeProviderResult {
		return fakeProviderResult{Text: "我之前已经把窗外日落照片发给你了。"}
	})
	request := ToolExecutionRequest{CapabilityName: "moment.publish", OperationID: "m2-moment-history-busy", AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, Arguments: jsonBytes(map[string]any{})}
	receipt, err := app.ExecuteTool(ctx, request)
	var moments int
	if queryErr := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.moments WHERE owner_fluctlight_id=$1`, fid).Scan(&moments); queryErr != nil {
		t.Fatal(queryErr)
	}
	if err == nil || receipt.Result.ErrorCode != "direct_media_delivery_busy" || !receipt.Result.Retryable || moments != 0 {
		t.Fatalf("busy delivery was not retryable: %#v moments=%d err=%v", receipt.Result, moments, err)
	}
}

func TestUnattachedMomentKeepsOrdinaryAndScopedHistoricalExpressions(t *testing.T) {
	for _, sample := range []struct {
		name, text, prior, subject string
		claim, publish             bool
		checks                     int
	}{
		{"ordinary", "今天的风很舒服。", "", "", false, true, 0},
		{"discussion", "我喜欢照片里这份宁静。", "", "", false, true, 1},
		{"post-discussion", "这条动态分享了我对照片的看法。", "", "", false, true, 1},
		{"post-discussion-order", "这条动态里放在最前面的是我对照片的看法。", "", "", false, true, 1},
		{"denial", "我还没有把照片发出去。", "", "", false, true, 1},
		{"future", "等明天我再把照片发给你。", "", "", false, true, 1},
		{"history-match", "我之前已经把窗外日落照片发给你了。", "窗外日落", "窗外日落", true, true, 1},
		{"history-mismatch", "我之前已经把海边照片发给你了。", "窗外日落", "窗外日落", true, false, 1},
		{"current-post-cannot-borrow-history", "我已经把窗外日落照片附在这条动态里了。", "窗外日落", "窗外日落", true, false, 1},
		{"current-post-presence-cannot-borrow-history", "窗外日落照片就在这条动态里了。", "窗外日落", "窗外日落", true, false, 1},
		{"current-post-location-cannot-borrow-history", "窗外日落照片在本条动态中。", "窗外日落", "窗外日落", true, false, 1},
	} {
		t.Run(sample.name, func(t *testing.T) {
			checks := 0
			router := newFakeProviderRouter().on(directMediaDeliveryCheckSchema, func(payload map[string]any) fakeProviderResult {
				checks++
				var view map[string]any
				for _, raw := range arrayValue(payload["messages"]) {
					message := mapValue(raw)
					if message["role"] == "user" {
						view = decodeObject([]byte(stringValue(message["content"])))
					}
				}
				claims := []any{}
				if sample.claim {
					ref := ""
					if deliveries := arrayValue(view["committed_deliveries"]); len(deliveries) > 0 {
						ref = stringValue(mapValue(deliveries[0])["ref"])
					}
					claims = append(claims, map[string]any{"quote": sample.text, "delivery_ref": ref, "subject_quote": sample.subject})
				}
				return fakeProviderResult{Structured: map[string]any{"claims": claims}}
			}).otherwise(func(map[string]any) fakeProviderResult {
				return fakeProviderResult{Text: sample.text}
			})
			ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m2-moment-claim-"+sample.name, router)
			if sample.prior != "" {
				seedDirectMediaEvidence(t, ctx, repo, app, fid, conversation, "m2-moment-history-"+sample.name, "completed", sample.prior, true)
			}
			request := ToolExecutionRequest{CapabilityName: "moment.publish", OperationID: "m2-moment-claim-" + sample.name, AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, Arguments: jsonBytes(map[string]any{})}
			receipt, err := app.ExecuteTool(ctx, request)
			var moments int
			if queryErr := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.moments WHERE owner_fluctlight_id=$1`, fid).Scan(&moments); queryErr != nil {
				t.Fatal(queryErr)
			}
			if sample.publish {
				if err != nil || receipt.Result.Status != "completed" || moments != 1 || checks != sample.checks {
					t.Fatalf("permitted Moment: %#v moments=%d checks=%d err=%v", receipt.Result, moments, checks, err)
				}
			} else if err != nil || receipt.Result.Status != "handled" || stringValue(mapValue(receipt.Result.Output)["warning_code"]) != "direct_media_delivery_unverified" || moments != 0 || checks != sample.checks {
				t.Fatalf("unsupported Moment: %#v moments=%d checks=%d err=%v", receipt.Result, moments, checks, err)
			}
		})
	}
}

func TestImplicitMomentMediaClaimUsesRecentHistory(t *testing.T) {
	for _, sample := range []struct{ name, history, text string }{
		{"chat", "之前那张照片呢？", "已经贴到聊天里了。"},
		{"current-post", "之前那张照片呢？", "就在这条动态里了。"},
		{"group-photo", "那张合照呢？", "已经在这条动态里了。"},
	} {
		for _, mode := range []string{"direct", "legacy"} {
			t.Run(sample.name+"/"+mode, func(t *testing.T) {
				checks := 0
				router := newFakeProviderRouter().on(directMediaDeliveryCheckSchema, func(map[string]any) fakeProviderResult {
					checks++
					return fakeProviderResult{Structured: map[string]any{"claims": []any{map[string]any{"quote": sample.text, "delivery_ref": "", "subject_quote": ""}}}}
				}).otherwise(func(map[string]any) fakeProviderResult {
					return fakeProviderResult{Text: sample.text}
				})
				suffix := sample.name + "-" + mode
				ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m2-moment-implicit-"+suffix, router)
				if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
					t.Fatal(err)
				}
				if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES($1,$2,1,$3,'user',$4,'[]',$1)`, "m2-moment-history-user-"+suffix, conversation, owner, sample.history); err != nil {
					t.Fatal(err)
				}
				persistConversationReplyMode(t, ctx, repo, mode)
				scope := ToolExecutionRequest{CapabilityName: "moment.publish", OperationID: "m2-moment-implicit-" + suffix, AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, Arguments: jsonBytes(map[string]any{})}
				if mode == "legacy" {
					scope.Arguments = jsonBytes(map[string]any{"text": sample.text})
				}
				var err error
				ctx, err = app.withPrivateConversationReplyRun(ctx, scope, "帮我发条动态。")
				if err != nil {
					t.Fatal(err)
				}
				receipt, err := app.ExecuteTool(ctx, scope)
				var moments int
				if queryErr := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.moments WHERE owner_fluctlight_id=$1`, fid).Scan(&moments); queryErr != nil {
					t.Fatal(queryErr)
				}
				if err != nil || receipt.Result.Status != "handled" || stringValue(mapValue(receipt.Result.Output)["warning_code"]) != "direct_media_delivery_unverified" || checks != 1 || moments != 0 {
					t.Fatalf("implicit media claim was not checked: %#v checks=%d moments=%d err=%v", receipt.Result, checks, moments, err)
				}
			})
		}
	}
}

func TestLegacyMomentUsesTheSameUnattachedMediaGuard(t *testing.T) {
	for _, sample := range []struct {
		name, text string
		publish    bool
		checks     int
	}{
		{"false-claim", "我已经把这张照片附在这条动态里了。", false, 1},
		{"denial", "我还没有把照片发出去。", true, 1},
		{"future", "等明天我再把照片发给你。", true, 1},
		{"ordinary-completion", "今天已经把书读完了。", true, 0},
	} {
		t.Run(sample.name, func(t *testing.T) {
			checks := 0
			router := newFakeProviderRouter().on(directMediaDeliveryCheckSchema, func(map[string]any) fakeProviderResult {
				checks++
				claims := []any{}
				if !sample.publish {
					claims = append(claims, map[string]any{"quote": sample.text, "delivery_ref": "", "subject_quote": ""})
				}
				return fakeProviderResult{Structured: map[string]any{"claims": claims}}
			})
			ctx, repo, app, owner, fid, _ := setupMixedMediaReplyTurnWithTransport(t, "m2-legacy-moment-"+sample.name, router)
			persistConversationReplyMode(t, ctx, repo, "legacy")
			request := ToolExecutionRequest{CapabilityName: "moment.publish", OperationID: "m2-legacy-moment-" + sample.name, AuthorizationActorID: owner, FluctlightID: fid, Arguments: jsonBytes(map[string]any{"text": sample.text})}
			receipt, err := app.ExecuteTool(ctx, request)
			var moments int
			if queryErr := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.moments WHERE owner_fluctlight_id=$1`, fid).Scan(&moments); queryErr != nil {
				t.Fatal(queryErr)
			}
			if sample.publish {
				if err != nil || receipt.Result.Status != "completed" || moments != 1 || checks != sample.checks {
					t.Fatalf("valid legacy Moment: %#v moments=%d checks=%d err=%v", receipt.Result, moments, checks, err)
				}
			} else if err != nil || receipt.Result.Status != "handled" || stringValue(mapValue(receipt.Result.Output)["warning_code"]) != "direct_media_delivery_unverified" || moments != 0 || checks != sample.checks {
				t.Fatalf("legacy false claim was published: %#v moments=%d checks=%d err=%v", receipt.Result, moments, checks, err)
			}
		})
	}
}

func TestDirectMomentMediaRejectionIsBounded(t *testing.T) {
	var mainCalls, momentCalls, checks int
	var forcedReply bool
	router := newFakeProviderRouter().on(directMediaDeliveryCheckSchema, func(map[string]any) fakeProviderResult {
		checks++
		return fakeProviderResult{Structured: map[string]any{"claims": []any{map[string]any{"quote": "我已经把照片附在这条动态里了。", "delivery_ref": "", "subject_quote": ""}}}}
	}).otherwise(func(payload map[string]any) fakeProviderResult {
		if payload["model"] == "m2-bounded-moment" {
			momentCalls++
			return fakeProviderResult{Text: "我已经把照片附在这条动态里了。"}
		}
		mainCalls++
		if mainCalls <= 3 {
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("bounded-moment-"+string(rune('0'+mainCalls)), "moment.publish", map[string]any{})}}
		}
		forcedReply = r6PayloadHasTool(payload, conversationReplyCapabilityName) && !r6PayloadHasTool(payload, "moment.publish")
		decision := map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("bounded-moment-reply", conversationReplyCapabilityName, map[string]any{"text": "这次先不发了。", "decision": decision})}}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m2-moment-bounded", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	role := agentModelBindingKey(FormalAgentConversationReply, "moment")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,timeout_seconds) VALUES($1,'mixed-endpoint-m2-moment-bounded','m2-bounded-moment',30)`, role); err != nil {
		t.Fatal(err)
	}
	turn, err := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "发一条动态。", "turn_id": "m2-moment-bounded", "idempotency_key": "m2-moment-bounded", "attachment_refs": []any{}})
	var moments int
	if queryErr := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.moments WHERE owner_fluctlight_id=$1`, fid).Scan(&moments); queryErr != nil {
		t.Fatal(queryErr)
	}
	if err != nil || stringValue(turn.Assistant["text"]) != "这次先不发了。" || moments != 0 || mainCalls != 4 || momentCalls != 1 || checks != 1 || !forcedReply {
		t.Fatalf("Moment rejection was not bounded: turn=%#v moments=%d main=%d writer=%d checks=%d forced=%t err=%v", turn.Assistant, moments, mainCalls, momentCalls, checks, forcedReply, err)
	}
}

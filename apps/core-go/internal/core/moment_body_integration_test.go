package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
)

func TestMomentPublicationFiltersNativeAndWriterText(t *testing.T) {
	for _, mode := range []string{"legacy", "fact_only"} {
		t.Run(mode, func(t *testing.T) {
			ctx, repo, app, owner, self, _ := setupDirectPublicationToolTest(t, "moment-body-"+mode)
			persistConversationReplyMode(t, ctx, repo, mode)
			var candidate string
			writerCalls := 0
			app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				writerCalls++
				if payload["max_tokens"] != nil || payload["max_completion_tokens"] != nil {
					t.Error("Moment body filtering introduced an output cap")
				}
				return fakeProviderResult{Text: candidate}
			})}
			ctx = withFrozenProviderAssignment(ctx, conversationReplyModelRole, providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "moment-body-writer", Timeout: time.Second})
			for _, sample := range []struct{ name, raw, want string }{
				{"heading", "# 发布动态\n今天也要好好生活。\n\n[自拍照：站在窗边微笑]\n# 认知\nprivate\n# 事务记录\nprivate", "今天也要好好生活。"},
				{"plain", "今天读到一个值得记下的段落。", "今天读到一个值得记下的段落。"},
				{"malformed-envelope", `{"text":"# 发布动态\n你好。\n# 事务记录\nprivate"`, "你好。"},
				{"fenced", "```markdown\n# 认知\nprivate\n# 发布动态\n留作稍后的讨论。\n# 事务记录\nprivate\n```", "留作稍后的讨论。"},
			} {
				t.Run(sample.name, func(t *testing.T) {
					candidate = sample.raw
					arguments := map[string]any{}
					if mode == "legacy" {
						arguments["text"] = sample.raw
					}
					request := ToolExecutionRequest{CapabilityName: "moment.publish", OperationID: "moment-body-" + sample.name, AuthorizationActorID: owner, FluctlightID: self, Arguments: json.RawMessage(jsonBytes(arguments))}
					before := writerCalls
					var id string
					for attempt := 0; attempt < 2; attempt++ {
						receipt, err := app.ExecuteTool(ctx, request)
						output := mapValue(receipt.Result.Output)
						if err != nil || receipt.Result.Status != "completed" || output["text"] != sample.want || toolBoolValue(output["replayed"]) != (attempt == 1) {
							t.Fatalf("Moment receipt = %#v; error=%v", receipt.Result, err)
						}
						if attempt == 0 {
							id = stringValue(output["target_ref"])
						} else if output["target_ref"] != id {
							t.Fatal("replay published another Moment", output)
						}
					}
					wantCalls := before
					if mode == "fact_only" {
						wantCalls++
					}
					if writerCalls != wantCalls {
						t.Fatalf("Writer calls=%d, want %d", writerCalls, wantCalls)
					}
					var stored string
					var media, outbox int
					if err := repo.Pool().QueryRow(ctx, `SELECT text,jsonb_array_length(media_asset_ids) FROM public.moments WHERE id=$1 AND owner_fluctlight_id=$2`, id, self).Scan(&stored, &media); err != nil || stored != sample.want || media != 0 {
						t.Fatalf("stored Moment=%q media=%d; error=%v", stored, media, err)
					}
					if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_outbox_events WHERE kind='moment.published' AND aggregate_id=$1`, id).Scan(&outbox); err != nil || outbox != 1 {
						t.Fatalf("Moment outbox=%d, want 1; error=%v", outbox, err)
					}
				})
			}
		})
	}
}

func TestMomentPublicationRejectsBodylessTextWithoutEffects(t *testing.T) {
	ctx, repo, app, owner, self, _ := setupDirectPublicationToolTest(t, "moment-bodyless")
	persistConversationReplyMode(t, ctx, repo, "legacy")
	for index, raw := range []string{"# 发布动态\n[自拍照：窗边]", "# 认知\nprivate\n# 事务记录\nprivate"} {
		receipt, err := app.ExecuteTool(ctx, ToolExecutionRequest{CapabilityName: "moment.publish", OperationID: fmt.Sprint("bodyless-", index), AuthorizationActorID: owner, FluctlightID: self, Arguments: json.RawMessage(jsonBytes(map[string]any{"text": raw}))})
		requireHandledToolNoEffect(t, receipt, err, "invalid_arguments")
	}
	var moments, outbox int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.moments WHERE owner_fluctlight_id=$1`, self).Scan(&moments); err != nil {
		t.Fatal(err)
	}
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_outbox_events WHERE kind='moment.published' AND fluctlight_id=$1`, self).Scan(&outbox); err != nil {
		t.Fatal(err)
	}
	if moments != 0 || outbox != 0 {
		t.Fatalf("bodyless output created effects: moments=%d outbox=%d", moments, outbox)
	}
}

func TestOwnerRequestedMomentBodyUsesAttachedMediaAndReplays(t *testing.T) {
	ctx, repo, app, owner, self, _ := setupDirectPublicationToolTest(t, "owner-moment-body")
	// Only the existing read projection checks this configuration; no object I/O.
	app.Storage = &minio.Client{}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.media_assets(id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status) VALUES('moment-body-asset',$1,'v1','image','image/png',68,'controlled-digest','isolated','not-fetched','moment-body-provider','moment-body-workflow','ready')`, self); err != nil {
		t.Fatal(err)
	}
	for _, attached := range []bool{false, true} {
		t.Run(fmt.Sprint("attached-", attached), func(t *testing.T) {
			id := fmt.Sprint("owner-moment-body-", attached)
			media := []any{}
			want := "今天也要好好生活。"
			if attached {
				media = append(media, "moment-body-asset")
				want += "\n\n[自拍照：站在窗边微笑]"
			}
			payload := map[string]any{"kind": "moment_suggestion", "status": "pending", "source_fact_id": "moment-body-source", "payload": map[string]any{"media_ids": media}}
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.owner_surface_records(id,owner_actor_id,fluctlight_id,kind,resource_key,status,payload) VALUES($1,$2,$3,'owner_request',$1,'pending',$4)`, id, owner, self, jsonBytes(payload)); err != nil {
				t.Fatal(err)
			}
			inv := CapabilityInvocation{CapabilityName: "moment.publish", SourceFactID: "moment-body-source"}
			target := DirectToolTarget{Kind: "owner_moment_request", Ref: id, AuthorizationActorID: owner, FluctlightID: self}
			var momentID string
			for attempt := 0; attempt < 2; attempt++ {
				text := "# 发布动态\n今天也要好好生活。\n\n[自拍照：站在窗边微笑]\n# 事务记录\nprivate"
				if attempt == 1 {
					text = "# 认知\nreplay must use the committed body"
				}
				err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
					resource, err := app.publishInvocationMomentTx(ctx, tx, inv, target, text)
					if err != nil {
						return err
					}
					if resource.Text != want || resource.Replayed != (attempt == 1) || (attempt == 1 && resource.ID != momentID) {
						return fmt.Errorf("owner Moment body/replay=%+v, want %q", resource, want)
					}
					momentID = resource.ID
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			var stored string
			var storedMedia []byte
			var refs int
			if err := repo.Pool().QueryRow(ctx, `SELECT text,media_asset_ids FROM public.moments WHERE id=$1 AND owner_request_id=$2`, momentID, id).Scan(&stored, &storedMedia); err != nil || stored != want || string(storedMedia) != jsonString(media) {
				t.Fatalf("owner Moment stored=%q media=%s, want=%q media=%s; error=%v", stored, storedMedia, want, jsonString(media), err)
			}
			if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.media_references WHERE target_type='moment' AND target_id=$1`, momentID).Scan(&refs); err != nil || refs != len(media) {
				t.Fatalf("attached media refs=%d, want %d; error=%v", refs, len(media), err)
			}
		})
	}
}

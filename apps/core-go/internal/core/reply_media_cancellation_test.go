package core

import (
	"fmt"
	"testing"
)

func TestReplyMediaTerminalStateDoesNotAttachCancelledOrFailedAsset(t *testing.T) {
	ctx, repo, app, owner, self, conversation := setupDirectPublicationToolTest(t, "cancelled-media")
	draft := "cancelled-media-draft"
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_reply_drafts(id,logical_key,fluctlight_id,conversation_id,authorization_actor_id,subject_actor_id,status,config_snapshot)
		VALUES($1,$1,$2,$3,$4,$4,'waiting_media','{}')`, draft, self, conversation, owner); err != nil {
		t.Fatal(err)
	}
	for index, test := range []struct {
		status, want, code string
		ready              bool
	}{
		{"pending", "pending", "", false},
		{"completed", "ready", "", true},
		{"completed", "failed", "media_asset_missing", false},
		{"cancelled", "cancelled", "media_generation_cancelled", true},
		{"failed", "failed", "media_generation_failed", true},
	} {
		mediaID := fmt.Sprintf("cancelled-media-%d", index)
		workflowID := fmt.Sprintf("cancelled-media-workflow-%d", index)
		providerID := fmt.Sprintf("cancelled-media-provider-%d", index)
		if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,status)
			VALUES($1,$2,'image','image/png','{}',$3,$4,$5)`, mediaID, self, providerID, workflowID, test.status); err != nil {
			t.Fatal(err)
		}
		if test.ready {
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.media_assets(id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status,ready_at)
				VALUES($1,$2,'v1','image','image/png',1,'sha','test','test',$3,$4,'ready',now())`, "asset_"+mediaID, self, providerID, workflowID); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_reply_actions(ref,draft_id,request_index,kind,intent,media_intent_id,status)
			VALUES($1,$2,$3,'image','窗外',$4,'bound')`, fmt.Sprintf("action-%d", index), draft, index, mediaID); err != nil {
			t.Fatal(err)
		}
	}
	states, err := app.readReplyActions(ctx, draft)
	if err != nil || len(states) != 5 {
		t.Fatalf("states=%#v err=%v", states, err)
	}
	for index, want := range []struct{ status, code string }{{"pending", ""}, {"ready", ""}, {"failed", "media_asset_missing"}, {"cancelled", "media_generation_cancelled"}, {"failed", "media_generation_failed"}} {
		state := states[index]
		if state.Status != want.status || state.ErrorCode != want.code || (state.AssetID != "") != (want.status == "ready") {
			t.Errorf("state %d = %#v, want %#v", index, state, want)
		}
	}
	var retained int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.media_assets WHERE id='asset_cancelled-media-3' AND status='ready'`).Scan(&retained); err != nil || retained != 1 {
		t.Fatalf("cancelled ready asset was lost: count=%d err=%v", retained, err)
	}
}

package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func seedDirectMediaEvidence(t *testing.T, ctx context.Context, repo *PostgresRepository, app *App, fid, conversation, id, status, intent string, delivered bool) {
	t.Helper()
	asset, request, workflow := "asset_"+id, "request-"+id, "workflow-"+id
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,conversation_id,status) VALUES($1,$2,'image','image/png',$3,$4,$5,$6,$7)`, id, fid, jsonString(map[string]any{"intent": intent}), request, workflow, conversation, status); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.media_assets(id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status) VALUES($1,$2,'v1','image','image/png',1,'fixture','fixture','fixture',$3,$4,'ready')`, asset, fid, request, workflow); err != nil {
		t.Fatal(err)
	}
	if delivered {
		if err := app.publishMediaAsset(ctx, mediaIntent{ID: id, Owner: fid, ConversationID: &conversation}, asset); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDirectMediaDeliveryEvidenceRequiresScopedCommittedAttachment(t *testing.T) {
	ctx, repo, app, owner, fid, conversation := setupDirectPublicationToolTest(t, "direct-delivery-evidence")
	seedDirectMediaEvidence(t, ctx, repo, app, fid, conversation, "pending-only", "pending", "未发送的海景", false)
	seedDirectMediaEvidence(t, ctx, repo, app, fid, conversation, "failed-only", "failed", "失败的猫", false)
	seedDirectMediaEvidence(t, ctx, repo, app, fid, conversation, "actual-delivery", "completed", "窗外的日落", true)
	otherConversation := "other-direct-delivery-conversation"
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversations(id,created_by_actor_id,title) VALUES($1,$2,'other')`, otherConversation, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_heads(conversation_id,next_sequence) VALUES($1,1)`, otherConversation); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,'owner','active'),($1,$3,'member','active')`, otherConversation, owner, fid); err != nil {
		t.Fatal(err)
	}
	seedDirectMediaEvidence(t, ctx, repo, app, fid, otherConversation, "other-conversation-photo", "completed", "另一会话的照片", true)

	items, err := directMediaDeliveries(ctx, repo.Pool(), owner, owner, fid, conversation, false)
	if err != nil || len(items) != 1 || items[0].intentID != "actual-delivery" || items[0].Intent != "窗外的日落" {
		t.Fatalf("committed delivery was not distinguished from ready/failed: items=%#v err=%v", items, err)
	}
	ref := items[0].Ref
	if strings.Contains(ref, "actual-delivery") || strings.Contains(ref, conversation) || strings.Contains(ref, owner) {
		t.Fatalf("model-visible reference exposed Core identity: %s", ref)
	}
	if _, err := directMediaDeliveryByRef(items, ref); err != nil {
		t.Fatalf("actual delivery reference was not resolved: %v", err)
	}
	if _, err := directMediaDeliveryByRef(items, "delivery:invented"); err == nil {
		t.Fatal("invented delivery reference was accepted")
	}
	foreign, err := directMediaDeliveries(ctx, repo.Pool(), "other-owner", owner, fid, conversation, false)
	if err != nil || len(foreign) != 0 {
		t.Fatalf("foreign Owner obtained delivery evidence: count=%d err=%v", len(foreign), err)
	}
	other, err := directMediaDeliveries(ctx, repo.Pool(), owner, owner, fid, otherConversation, false)
	if err != nil || len(other) != 1 || other[0].intentID != "other-conversation-photo" || other[0].Ref == ref {
		t.Fatalf("different conversation was not isolated: items=%#v err=%v", other, err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.media_intents SET prompt=$2 WHERE id=$1`, "actual-delivery", jsonString(map[string]any{"intent": "不同任务"})); err != nil {
		t.Fatal(err)
	}
	changed, err := directMediaDeliveries(ctx, repo.Pool(), owner, owner, fid, conversation, false)
	if err != nil || len(changed) != 1 {
		t.Fatalf("changed delivery was not reread: count=%d err=%v", len(changed), err)
	}
	if _, err := directMediaDeliveryByRef(changed, ref); err == nil {
		t.Fatal("stale reference survived a changed task intent")
	}
	if _, err := repo.Pool().Exec(ctx, `DELETE FROM public.media_references WHERE asset_id='asset_actual-delivery'`); err != nil {
		t.Fatal(err)
	}
	items, err = directMediaDeliveries(ctx, repo.Pool(), owner, owner, fid, conversation, false)
	if err != nil || len(items) != 0 {
		t.Fatalf("removed reference still counted as delivery: count=%d err=%v", len(items), err)
	}
}

func TestDirectMediaDeliveryEvidenceDoesNotInventTimeForLateAttachment(t *testing.T) {
	ctx, repo, app, owner, fid, conversation := setupDirectPublicationToolTest(t, "direct-late-attachment-time")
	const intentID = "late-attachment-time"
	const messageID = "late-attachment-message"
	seedDirectMediaEvidence(t, ctx, repo, app, fid, conversation, intentID, "completed", "窗外的山", false)
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
	items, err := directMediaDeliveries(ctx, repo.Pool(), owner, owner, fid, conversation, false)
	if err != nil || len(items) != 1 || !items[0].Delivered.IsZero() {
		t.Fatalf("late attachment must be delivered but its exact delivery time is unknown: items=%#v err=%v", items, err)
	}
}

func TestDirectMediaDeliveryEvidenceLocksTheCommittedRows(t *testing.T) {
	ctx, repo, app, owner, fid, conversation := setupDirectPublicationToolTest(t, "direct-delivery-lock")
	seedDirectMediaEvidence(t, ctx, repo, app, fid, conversation, "locked-delivery", "completed", "一张风景照", true)
	tx, err := repo.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	items, err := directMediaDeliveries(ctx, tx, owner, owner, fid, conversation, true)
	if err != nil || len(items) != 1 {
		t.Fatalf("transaction could not lock committed evidence: count=%d err=%v", len(items), err)
	}
	blockedCtx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	if _, err := repo.Pool().Exec(blockedCtx, `DELETE FROM public.media_references WHERE id=$1`, items[0].refID); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("concurrent deletion did not wait for the delivery fence: %v", err)
	}
	participantCtx, stopParticipant := context.WithTimeout(ctx, 250*time.Millisecond)
	defer stopParticipant()
	if _, err := repo.Pool().Exec(participantCtx, `UPDATE public.conversation_participants SET status='left' WHERE conversation_id=$1 AND actor_id=$2`, conversation, owner); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("participant revocation did not wait for the delivery fence: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `DELETE FROM public.media_references WHERE id=$1`, items[0].refID); err != nil {
		t.Fatalf("delivery fence was not released: %v", err)
	}
}

func TestDirectMediaDeliveryEvidenceRequiresPublishedDraftAndMatchingMessage(t *testing.T) {
	ctx, repo, app, owner, fid, conversation := setupDirectPublicationToolTest(t, "direct-draft-evidence")
	seedDirectMediaEvidence(t, ctx, repo, app, fid, conversation, "draft-photo", "completed", "雨后的街道", false)
	var messageID string
	if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
		result, err := NewToolPublicationService(app).PublishConversationReplyTx(ctx, tx, ConversationReplyPublication{AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, OperationID: "draft-photo-publish", Text: "这张街景送给你。", AttachmentRefs: []string{"asset_draft-photo"}})
		if err != nil {
			return err
		}
		messageID = result.ID
		if _, err := tx.Exec(ctx, `INSERT INTO public.conversation_reply_drafts(id,logical_key,fluctlight_id,conversation_id,authorization_actor_id,subject_actor_id,status,config_snapshot,result_message_id) VALUES('draft-photo','draft-photo-key',$1,$2,$3,$3,'published','{}',$4)`, fid, conversation, owner, messageID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO public.conversation_reply_actions(ref,draft_id,request_index,kind,intent,media_intent_id,status) VALUES('draft-photo-action','draft-photo',0,'image','雨后的街道','draft-photo','bound')`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	items, err := directMediaDeliveries(ctx, repo.Pool(), owner, owner, fid, conversation, false)
	if err != nil || len(items) != 1 || items[0].intentID != "draft-photo" || items[0].messageID != messageID {
		t.Fatalf("published draft attachment was not proven: items=%#v err=%v", items, err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES('other-active-subject','human','active')`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,'other-active-subject','member','active')`, conversation); err != nil {
		t.Fatal(err)
	}
	shared, err := directMediaDeliveries(ctx, repo.Pool(), owner, "other-active-subject", fid, conversation, false)
	if err != nil || len(shared) != 1 || shared[0].intentID != "draft-photo" || shared[0].Ref == items[0].Ref {
		t.Fatalf("shared conversation delivery or subject-scoped ref invalid: items=%#v err=%v", shared, err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_messages SET attachment_refs='[]'::jsonb WHERE id=$1`, messageID); err != nil {
		t.Fatal(err)
	}
	items, err = directMediaDeliveries(ctx, repo.Pool(), owner, owner, fid, conversation, false)
	if err != nil || len(items) != 0 {
		t.Fatalf("draft alone was accepted without its actual attachment: count=%d err=%v", len(items), err)
	}
}

func TestDirectMediaDeliveryEvidenceDoesNotTreatUserMessageAsOutgoing(t *testing.T) {
	ctx, repo, app, owner, fid, conversation := setupDirectPublicationToolTest(t, "direct-user-message")
	seedDirectMediaEvidence(t, ctx, repo, app, fid, conversation, "user-message-photo", "completed", "一张山景", false)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES('user-image-message',$1,1,$2,'user','我的消息',$3,'user-image-message')`, conversation, owner, jsonBytes([]string{"asset_user-message-photo"})); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.media_intents SET message_id='user-image-message',conversation_id=NULL WHERE id='user-message-photo'`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.media_references(id,asset_id,owner_fluctlight_id,target_type,target_id) VALUES('user-image-reference','asset_user-message-photo',$1,'conversation_message','user-image-message')`, fid); err != nil {
		t.Fatal(err)
	}
	items, err := directMediaDeliveries(ctx, repo.Pool(), owner, owner, fid, conversation, false)
	if err != nil || len(items) != 0 {
		t.Fatalf("user-authored message was treated as outgoing delivery: count=%d err=%v", len(items), err)
	}
}

func TestDirectMediaDeliveryEvidenceDoesNotWaitBehindMediaIntentLock(t *testing.T) {
	ctx, repo, app, owner, fid, conversation := setupDirectPublicationToolTest(t, "direct-delivery-nowait")
	seedDirectMediaEvidence(t, ctx, repo, app, fid, conversation, "busy-delivery", "completed", "远处的山", true)
	mediaTx, err := repo.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer mediaTx.Rollback(ctx)
	var id string
	if err := mediaTx.QueryRow(ctx, `SELECT id FROM public.media_intents WHERE id='busy-delivery' FOR UPDATE`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	replyTx, err := repo.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer replyTx.Rollback(ctx)
	if err := lockLifeContextTx(ctx, replyTx, fid); err != nil {
		t.Fatal(err)
	}
	_, err = directMediaDeliveriesForIntents(ctx, replyTx, owner, owner, fid, conversation, true, []string{"busy-delivery"})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "55P03" {
		t.Fatalf("busy media row did not return NOWAIT conflict: %v", err)
	}
	code, retryable := capabilityErrorInfo(directMediaDeliveryTxError(err), "", false)
	if code != "direct_media_delivery_busy" || !retryable {
		t.Fatalf("lock contention was misreported as missing evidence: code=%s retryable=%t", code, retryable)
	}
}

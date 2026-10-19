package core

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
)

type ownerVisualReviewFenceKey struct{}
type ownerVisualReviewFence struct {
	Owner                   string
	Attempt, VisualRevision int
	Reason                  string
}

func (a *App) dispatchOwnerVisualReview(ctx context.Context, method, path string, q url.Values, owner string, body map[string]any) (map[string]any, error) {
	parts := surfaceParts(path)
	fid, session := parts[1], parts[3]
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	attempt, ok := parseRevision(body["expected_attempt"])
	revision, revOK := parseRevision(body["expected_visual_revision"])
	key := strings.TrimSpace(stringValue(body["idempotency_key"]))
	reason := strings.TrimSpace(stringValue(body["reason"]))
	if !ok || attempt < 1 || !revOK || revision < 0 || key == "" || len(key) > 256 || reason == "" || len([]rune(reason)) > 1024 {
		return nil, invalid("manual visual review versions and reason required")
	}
	arguments := map[string]any{"review_source": "manual"}
	for _, key := range []string{"decision", "identity_match", "confidence", "observations", "missing_sections", "summary", "feedback"} {
		arguments[key] = body[key]
	}
	scope := context.WithValue(ctx, ownerVisualReviewFenceKey{}, ownerVisualReviewFence{Owner: owner, Attempt: attempt, VisualRevision: revision, Reason: reason})
	operation := "owner-visual-review:" + stableDigest(owner+"\x1f"+fid+"\x1f"+key)
	receipt, err := a.ExecuteTool(scope, ToolExecutionRequest{CapabilityName: visualIdentityCommitReviewCapabilityName, OperationID: operation, AuthorizationActorID: owner, FluctlightID: fid, TargetKind: "visual_identity_session", TargetRef: session, EvidenceID: "owner-review:" + stableDigest(jsonString(body)), Surface: CapabilitySurfaceVisualIdentity, Arguments: jsonBytes(arguments)})
	if err != nil {
		return nil, err
	}
	if receipt.Result.Status != "completed" && receipt.Result.Status != "accepted" {
		return nil, &SurfaceError{Status: 422, Code: firstString(receipt.Result.ErrorCode, "visual_review_not_applied"), Message: "人工评审未提交，请查看实际候选与任务状态", Details: map[string]any{"status": receipt.Result.Status, "result": receipt.Result.Output}}
	}
	return map[string]any{"fluctlight_id": fid, "session_id": session, "operation_id": operation, "replayed": receipt.Replayed, "result": receipt.Result.Output}, nil
}
func enqueueOwnerVisualResumeTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, session visualIdentityToolSession) error {
	_, ok := ctx.Value(ownerVisualReviewFenceKey{}).(ownerVisualReviewFence)
	if !ok {
		return nil
	}
	intent := "visual_review_resume:" + stableDigest(session.SessionID+":"+invocation.Metadata.OperationID)
	workflow := "visual_review_workflow_" + stableDigest(intent)
	if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'lifecycle','visual_identity.initialize',$3) ON CONFLICT(intent_id) DO NOTHING`, intent, workflow, jsonBytes(map[string]any{"intent_id": intent, "session_id": session.SessionID, "fluctlight_id": session.FluctlightID, "correlation_id": intent, "causation_id": invocation.SourceFactID})); err != nil {
		return err
	}
	return nil
}
func checkOwnerVisualReviewFenceTx(ctx context.Context, tx pgx.Tx, session visualIdentityToolSession) error {
	fence, ok := ctx.Value(ownerVisualReviewFenceKey{}).(ownerVisualReviewFence)
	if !ok {
		return nil
	}
	if err := requireOwnerMutableTx(ctx, tx, fence.Owner, session.FluctlightID); err != nil {
		return err
	}
	var revision int
	err := tx.QueryRow(ctx, `SELECT current_revision FROM public.fluctlight_visual_identities WHERE id=$1 FOR UPDATE`, session.ProfileID).Scan(&revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if revision != fence.VisualRevision || session.Attempt != fence.Attempt {
		return &SurfaceError{Status: 409, Code: "visual_review_conflict", Message: "候选尝试或规范图版本已变化，请重新核对实际图片后评审"}
	}
	return nil
}

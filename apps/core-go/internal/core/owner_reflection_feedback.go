package core

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const ownerReflectionFeedbackFactType = "reflection.owner_feedback.submitted"

func (a *App) dispatchOwnerFeedback(ctx context.Context, method, path string, query url.Values, owner string, body map[string]any) (map[string]any, error) {
	var out map[string]any
	var business error
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "owner-feedback:"+owner); err != nil {
			return err
		}
		if body == nil {
			body = map[string]any{}
		}
		var storage error
		out, business, storage = NewOwnerSurface(nil, a.surfaceHooks()).dispatchTx(context.WithValue(ctx, surfaceDBTxKey{}, tx), postgresSurfaceTx{tx}, method, path, query, owner, body)
		return storage
	})
	if err != nil {
		return nil, &SurfaceError{Status: 503, Code: "feedback_commit_failed", Message: "Feedback could not be committed"}
	}
	return out, business
}

func (a *App) enqueueOwnerFeedback(ctx context.Context, owner, fid, id, kind string, body map[string]any) (string, error) {
	tx, ok := SurfaceDBTx(ctx)
	if !ok {
		return "", errors.New("feedback_transaction_required")
	}
	if kind == "diary_reply" {
		return a.enqueueOwnerDiaryReply(ctx, tx, owner, fid, id, body)
	}
	if kind != "reflection_feedback" {
		return "", ErrInvalidArguments
	}
	text := strings.TrimSpace(stringValue(body["text"]))
	if text == "" || len([]rune(text)) > 2000 {
		return "", invalid("feedback text must contain 1–2000 characters")
	}
	if stance, present := body["stance"]; present && stance != "agree" && stance != "disagree" {
		return "", invalid("feedback stance is invalid")
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('fluctlight_lifecycle:' || $1))`, fid); err != nil {
		return "", err
	}
	var status string
	var summary []byte
	err := tx.QueryRow(ctx, `SELECT f.status,p.payload FROM public.cognition_reflection_proposals p JOIN public.fluctlights f ON f.id=p.fluctlight_id WHERE p.id=$1 AND p.fluctlight_id=$2 AND f.created_by_actor_id=$3 FOR SHARE OF f,p`, stringValue(body["reflection_id"]), fid, owner).Scan(&status, &summary)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", &SurfaceError{Status: 404, Code: "reflection_not_found", Message: "Reflection is unavailable"}
	}
	if err != nil {
		return "", err
	}
	if status == "retired" {
		return "", &SurfaceError{Status: 409, Code: "fluctlight_retired", Message: "A retired Fluctlight cannot process feedback"}
	}
	return a.enqueueNativeFactTx(ctx, tx, fid, "", id, ownerReflectionFeedbackFactType, "reflection-feedback:"+id, map[string]any{"request_id": id, "owner_actor_id": owner, "kind": kind, "reflection_id": body["reflection_id"], "reflection_summary": decodeObject(summary)["summary"], "feedback": text, "stance": body["stance"], "evidence_semantics": "Owner feedback on a past reflection; not an automatic factual correction or instruction to alter persona"})
}

func (a *App) processOwnerReflectionFeedback(ctx context.Context, inbox string, fact map[string]any) (map[string]any, error) {
	candidate := mapValue(fact["candidate"])
	id, owner, fid := stringValue(candidate["request_id"]), stringValue(candidate["owner_actor_id"]), stringValue(fact["fluctlight_id"])
	var raw []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT payload FROM public.owner_surface_records WHERE id=$1 AND owner_actor_id=$2 AND fluctlight_id=$3 AND kind='owner_request'`, id, owner, fid).Scan(&raw); err != nil {
		return nil, err
	}
	record := decodeObject(raw)
	kind := stringValue(record["kind"])
	sourceKey := "reflection_id"
	if kind == "diary_reply" {
		sourceKey = "diary_id"
	}
	if (kind != "reflection_feedback" && kind != "diary_reply") || record["source_fact_id"] != inbox || stringValue(mapValue(record["payload"])[sourceKey]) != stringValue(candidate[sourceKey]) {
		return nil, ErrUnauthorized
	}
	if record["status"] != "pending" {
		return mapValue(record["result"]), nil
	}
	// The existing native Agent owns normal appraisal, Tools and settlement.
	// Re-entry after a crash reads its processed/failed inbox instead of running
	// another model or replaying an already committed Tool effect.
	runErr := a.ProcessNativeCognitionFact(ctx, inbox)
	settleCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	status, reason, code := "completed", "反馈已进入真实认知处理；是否产生改变以实际候选和应用结果为准。", ""
	if runErr != nil {
		status, reason, code = "failed", "反馈处理失败；已提交的效果保留，请在诊断中查看。", kind+"_processing_failed"
	}
	refs := []any{}
	results, err := a.ownerLifeCommittedResults(settleCtx, owner, fid, inbox)
	if err != nil {
		return nil, err
	}
	for _, r := range results {
		if r.Status == "completed" {
			refs = append(refs, r.CallID)
		}
	}
	result := map[string]any{"request_id": id, "status": status, "source_fact_id": inbox, sourceKey: candidate[sourceKey], "reason": reason, "error_code": nilIfEmpty(code), "result_refs": refs}
	err = withTransaction(settleCtx, a.DB.Pool(), func(tx pgx.Tx) error {
		var current []byte
		if err := tx.QueryRow(settleCtx, `SELECT payload FROM public.owner_surface_records WHERE id=$1 AND owner_actor_id=$2 AND fluctlight_id=$3 FOR UPDATE`, id, owner, fid).Scan(&current); err != nil {
			return err
		}
		r := decodeObject(current)
		if r["status"] != "pending" {
			result = mapValue(r["result"])
			return nil
		}
		r["status"], r["result"], r["result_refs"], r["decision_reason"], r["decided_by"], r["updated_at"] = status, result, refs, reason, "agent", a.now().UTC().Format(time.RFC3339Nano)
		r["revision"] = intValue(r["revision"]) + 1
		if _, err := tx.Exec(settleCtx, `UPDATE public.owner_surface_records SET status=$3,payload=$4,revision=revision+1,updated_at=now() WHERE id=$1 AND owner_actor_id=$2`, id, owner, status, jsonBytes(r)); err != nil {
			return err
		}
		if runErr != nil {
			if _, err := tx.Exec(settleCtx, `UPDATE public.cognition_inbox SET status='failed',processed_at=now(),claimed_by=NULL,claimed_at=NULL,error_code=$2 WHERE id=$1 AND status IN ('pending','claimed')`, inbox, code); err != nil {
				return err
			}
		}
		return appendOwnerRequestNotificationTx(settleCtx, tx, fid, id, kind, status, inbox)
	})
	return result, err
}

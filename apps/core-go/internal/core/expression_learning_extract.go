package core

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (a *App) OwnerExpressionExtractionStatus(ctx context.Context, owner, fid, key string) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	key = strings.TrimSpace(key)
	if key == "" || len([]rune(key)) > 256 {
		return nil, invalid("操作标识无效")
	}
	var status string
	var raw []byte
	err := a.DB.Pool().QueryRow(ctx, `SELECT status,result FROM public.expression_learning_extractions WHERE owner_actor_id=$1 AND fluctlight_id=$2 AND idempotency_key=$3`, owner, fid, key).Scan(&status, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		// An unobserved request may still be arriving; absence is not proof of
		// non-admission and cannot authorize a replacement idempotency key.
		return map[string]any{"status": "not_found", "no_effect": false}, nil
	}
	if err != nil {
		return nil, err
	}
	result := map[string]any{"status": status, "no_effect": status == "failed"}
	if status == "completed" {
		result["result"] = decodeObject(raw)
	}
	return result, nil
}

// ExtractOwnerExpressionCandidates freezes an authorized human message and
// an Agent binding before calling the model. Only revalidated, unreviewed
// proposals are persisted; the task cannot alter persona or live facts.
func (a *App) ExtractOwnerExpressionCandidates(ctx context.Context, owner, fid string, body map[string]any) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	messageID := strings.TrimSpace(stringValue(body["source_message_id"]))
	key := strings.TrimSpace(stringValue(body["idempotency_key"]))
	if !eventKeys(body, "source_message_id", "idempotency_key") || messageID == "" || key == "" || len([]rune(key)) > 256 {
		return nil, invalid("需要真实来源消息和操作标识")
	}
	digest := stableDigest(jsonString(map[string]any{"fluctlight_id": fid, "source_message_id": messageID}))
	var previousDigest, previousStatus string
	var previousResult []byte
	err := a.DB.Pool().QueryRow(ctx, `SELECT request_digest,status,result FROM public.expression_learning_extractions WHERE owner_actor_id=$1 AND idempotency_key=$2`, owner, key).Scan(&previousDigest, &previousStatus, &previousResult)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		if previousDigest != digest {
			return nil, &SurfaceError{Status: 409, Code: "idempotency_conflict", Message: "操作标识已用于其他来源"}
		}
		if previousStatus == "completed" {
			result := decodeObject(previousResult)
			result["_replay"] = true
			return result, nil
		}
		if previousStatus == "failed" {
			return nil, &SurfaceError{Status: 409, Code: "expression_extraction_failed", Message: "原提取已失败，请核对来源后重新开始"}
		}
	}
	source, err := readExpressionLearningSource(ctx, a.DB.Pool(), owner, fid, messageID, false)
	if err != nil {
		return nil, err
	}
	lease := randomID("expression_extraction_")
	extractionID := "expression_extraction_" + stableDigest(owner+"\x1f"+key)
	var bindingRaw []byte
	var learningGeneration int64
	var replay map[string]any
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := requireOwnerMutableTx(ctx, tx, owner, fid); err != nil {
			return err
		}
		var enabled bool
		if err := tx.QueryRow(ctx, `SELECT learning_enabled,learning_generation FROM public.expression_learning_settings WHERE fluctlight_id=$1 FOR SHARE`, fid).Scan(&enabled, &learningGeneration); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if !enabled {
			return &SurfaceError{Status: 409, Code: "expression_learning_disabled", Message: "表达学习当前未启用"}
		}
		current, err := readExpressionLearningSource(ctx, tx, owner, fid, messageID, true)
		if err != nil {
			return err
		}
		if current.Fingerprint != source.Fingerprint || current.Revision != source.Revision {
			return &SurfaceError{Status: 409, Code: "expression_source_changed", Message: "来源已变化，请重新选择"}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.expression_learning_extractions(id,fluctlight_id,owner_actor_id,source_message_id,source_fingerprint,source_revision,learning_generation,idempotency_key,request_digest,status,lease_owner,lease_until)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'running',$10,now()+interval '10 minutes') ON CONFLICT(owner_actor_id,idempotency_key) DO NOTHING`, extractionID, fid, owner, messageID, source.Fingerprint, source.Revision, learningGeneration, key, digest, lease); err != nil {
			return err
		}
		var savedDigest, savedFingerprint, status, savedLease string
		var savedGeneration, savedSourceRevision int64
		var leaseLive bool
		var resultRaw []byte
		err = tx.QueryRow(ctx, `SELECT request_digest,source_fingerprint,source_revision,learning_generation,status,COALESCE(lease_owner,''),COALESCE(lease_until>now(),false),result,binding_snapshot FROM public.expression_learning_extractions WHERE owner_actor_id=$1 AND idempotency_key=$2 FOR UPDATE`, owner, key).Scan(&savedDigest, &savedFingerprint, &savedSourceRevision, &savedGeneration, &status, &savedLease, &leaseLive, &resultRaw, &bindingRaw)
		if err != nil {
			return err
		}
		if savedDigest != digest {
			return &SurfaceError{Status: 409, Code: "idempotency_conflict", Message: "操作标识已用于其他来源"}
		}
		if status == "completed" {
			replay = decodeObject(resultRaw)
			replay["_replay"] = true
			return nil
		}
		if status == "failed" {
			return &SurfaceError{Status: 409, Code: "expression_extraction_failed", Message: "原提取已失败，请核对来源后重新开始"}
		}
		if savedFingerprint != source.Fingerprint || savedSourceRevision != source.Revision || savedGeneration != learningGeneration {
			return &SurfaceError{Status: 409, Code: "expression_source_changed", Message: "来源或操作标识已变化"}
		}
		if status == "running" && leaseLive && savedLease != lease {
			return &SurfaceError{Status: 409, Code: "expression_extraction_in_progress", Message: "该来源的提取仍在进行"}
		}
		_, err = tx.Exec(ctx, `UPDATE public.expression_learning_extractions SET status='running',lease_owner=$2,lease_until=now()+interval '10 minutes',last_error=NULL,updated_at=now() WHERE id=$1`, extractionID, lease)
		return err
	})
	if err != nil || replay != nil {
		return replay, err
	}
	failed := func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_, _ = a.DB.Pool().Exec(cleanup, `UPDATE public.expression_learning_extractions SET status='failed',lease_owner=NULL,lease_until=NULL,last_error='expression_extraction_failed',updated_at=now() WHERE id=$1 AND status='running' AND lease_owner=$2`, extractionID, lease)
	}
	if a.Provider == nil {
		failed()
		return nil, errors.New("expression_learning_provider_unavailable")
	}
	var assignment providerAssignment
	if len(decodeObject(bindingRaw)) == 0 {
		assignment, err = a.Provider.assignment(withAgentModelRoute(ctx, FormalAgentExpressionLearning, expressionLearningSchema), "cognitive_assessment")
		if err == nil {
			safe := assignment
			safe.Secret = ""
			var tagCount int64
			tag, saveErr := a.DB.Pool().Exec(ctx, `UPDATE public.expression_learning_extractions SET binding_snapshot=$3,updated_at=now() WHERE id=$1 AND status='running' AND lease_owner=$2 AND lease_until>now()`, extractionID, lease, jsonBytes(safe))
			if saveErr == nil {
				tagCount = tag.RowsAffected()
			}
			if saveErr != nil {
				err = saveErr
			} else if tagCount != 1 {
				err = errors.New("expression_extraction_lease_lost")
			}
		}
	} else {
		if err = json.Unmarshal(bindingRaw, &assignment); err == nil {
			err = a.restoreAssignmentCredential(ctx, &assignment)
		}
	}
	if err != nil {
		failed()
		return nil, err
	}
	modelCtx := withAgentModelRoute(ctx, FormalAgentExpressionLearning, expressionLearningSchema)
	modelCtx = withFrozenProviderAssignment(modelCtx, "cognitive_assessment", assignment)
	modelCtx, stopLease := a.maintainExpressionExtractionLease(modelCtx, extractionID, lease, 30*time.Second)
	proposals, err := a.runExpressionLearningTask(modelCtx, []expressionLearningSource{{SourceID: source.ID, Text: source.Text}})
	if leaseErr := stopLease(); leaseErr != nil {
		failed()
		return nil, leaseErr
	}
	if err != nil {
		failed()
		return nil, err
	}
	var result map[string]any
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := requireOwnerMutableTx(ctx, tx, owner, fid); err != nil {
			return err
		}
		var liveLease, status string
		var live bool
		if err := tx.QueryRow(ctx, `SELECT COALESCE(lease_owner,''),status,COALESCE(lease_until>now(),false) FROM public.expression_learning_extractions WHERE id=$1 FOR UPDATE`, extractionID).Scan(&liveLease, &status, &live); err != nil {
			return err
		}
		if status != "running" || liveLease != lease || !live {
			return &SurfaceError{Status: 409, Code: "expression_extraction_lease_lost", Message: "提取任务已过期，请重试"}
		}
		var enabled bool
		var currentGeneration int64
		if err := tx.QueryRow(ctx, `SELECT learning_enabled,learning_generation FROM public.expression_learning_settings WHERE fluctlight_id=$1 FOR SHARE`, fid).Scan(&enabled, &currentGeneration); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		current, err := readExpressionLearningSource(ctx, tx, owner, fid, messageID, true)
		if err != nil {
			return err
		}
		if !enabled || currentGeneration != learningGeneration || current.Fingerprint != source.Fingerprint || current.Revision != source.Revision {
			return &SurfaceError{Status: 409, Code: "expression_source_changed", Message: "来源或学习设置已变化，候选未保存"}
		}
		ids := make([]any, 0, len(proposals))
		for _, proposal := range proposals {
			if proposal.SourceID != source.ID || !strings.Contains(source.Text, proposal.Excerpt) {
				return errors.New("expression_learning_candidate_source_invalid")
			}
			dedupe := stableDigest(jsonString([]any{source.ID, source.Fingerprint, source.Revision, proposal.Excerpt, proposal.Situation}))
			id := "expression_candidate_" + stableDigest(fid+":"+dedupe)
			_, err := tx.Exec(ctx, `INSERT INTO public.expression_learning_candidates(id,fluctlight_id,owner_actor_id,source_actor_id,source_conversation_id,source_message_id,source_fingerprint,source_revision,excerpt,situation,dedupe_key)
				VALUES($1,$2,$3,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(fluctlight_id,dedupe_key) DO NOTHING`, id, fid, owner, source.ConversationID, source.ID, source.Fingerprint, source.Revision, proposal.Excerpt, proposal.Situation, dedupe)
			if err != nil {
				return err
			}
			ids = append(ids, id)
		}
		result = map[string]any{"status": "completed", "candidate_ids": ids}
		_, err = tx.Exec(ctx, `UPDATE public.expression_learning_extractions SET status='completed',result=$3,lease_owner=NULL,lease_until=NULL,updated_at=now() WHERE id=$1 AND lease_owner=$2`, extractionID, lease, jsonBytes(result))
		return err
	})
	if err != nil {
		failed()
	}
	return result, err
}

// Renew only while this request owns the lease. Losing it cancels the model
// request, so a replacement attempt and the old attempt cannot both settle.
func (a *App) maintainExpressionExtractionLease(ctx context.Context, id, owner string, interval time.Duration) (context.Context, func() error) {
	work, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-work.Done():
				done <- nil
				return
			case <-ticker.C:
				beat, stop := context.WithTimeout(work, 5*time.Second)
				tag, err := a.DB.Pool().Exec(beat, `UPDATE public.expression_learning_extractions SET lease_until=now()+interval '10 minutes',updated_at=now() WHERE id=$1 AND status='running' AND lease_owner=$2 AND lease_until>now()`, id, owner)
				stop()
				if work.Err() != nil {
					done <- nil
					return
				}
				if err != nil || tag.RowsAffected() != 1 {
					cancel()
					if err == nil {
						err = errors.New("expression_extraction_lease_lost")
					}
					done <- err
					return
				}
			}
		}
	}()
	return work, func() error {
		cancel()
		return <-done
	}
}

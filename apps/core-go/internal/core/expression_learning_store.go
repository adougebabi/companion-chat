package core

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type expressionLearningStoredSource struct {
	ID, ConversationID, ActorID, Text, Fingerprint string
	Revision                                       int64
}

// Only an actual Owner-authored human message in a shared conversation is an
// eligible learning source. Outcome, draft and assistant text never enter here.
func readExpressionLearningSource(ctx context.Context, q currentAuthorityReader, owner, fid, messageID string, lock bool) (expressionLearningStoredSource, error) {
	var source expressionLearningStoredSource
	query := `SELECT m.id,m.conversation_id,m.author_actor_id,m.text,md5(m.text || m.attachment_refs::text),COALESCE(v.revision,0)
		FROM public.conversation_messages m
		LEFT JOIN public.expression_learning_source_versions v ON v.message_id=m.id
		JOIN public.actors actor ON actor.id=m.author_actor_id AND actor.actor_type='human' AND actor.status='active'
		JOIN public.fluctlights f ON f.id=$2 AND f.created_by_actor_id=$1
		WHERE m.id=$3 AND m.kind='user' AND m.author_actor_id=$1 AND btrim(m.text)<>''
		AND EXISTS(SELECT 1 FROM public.conversation_participants p WHERE p.conversation_id=m.conversation_id AND p.actor_id=$1 AND p.status='active')
		AND EXISTS(SELECT 1 FROM public.conversation_participants p WHERE p.conversation_id=m.conversation_id AND p.actor_id=$2 AND p.status='active')`
	if lock {
		query += ` FOR SHARE OF m`
	}
	err := q.QueryRow(ctx, query, owner, fid, messageID).Scan(&source.ID, &source.ConversationID, &source.ActorID, &source.Text, &source.Fingerprint, &source.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return source, &SurfaceError{Status: 404, Code: "expression_source_unavailable", Message: "来源消息不存在或不属于当前角色的真实交流"}
	}
	return source, err
}

func (a *App) OwnerExpressionLearningSources(ctx context.Context, owner, fid string, q url.Values) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	limit, err := reflectionReadPage(q)
	if err != nil {
		return nil, err
	}
	scope := stableDigest(jsonString([]any{"expression-sources", owner, fid}))
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil || (q.Get("cursor") != "" && (cursor.ID == "" || cursor.Time.IsZero())) {
		return nil, invalid("来源游标无效")
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT m.id,m.conversation_id,m.text,m.created_at FROM public.conversation_messages m
		JOIN public.actors actor ON actor.id=m.author_actor_id AND actor.actor_type='human' AND actor.status='active'
		WHERE m.author_actor_id=$1 AND m.kind='user' AND btrim(m.text)<>''
		AND EXISTS(SELECT 1 FROM public.conversation_participants p WHERE p.conversation_id=m.conversation_id AND p.actor_id=$1 AND p.status='active')
		AND EXISTS(SELECT 1 FROM public.conversation_participants p WHERE p.conversation_id=m.conversation_id AND p.actor_id=$2 AND p.status='active')
		AND ($3::text='' OR (m.created_at,m.id::text)<($4,$3::text)) ORDER BY m.created_at DESC,m.id DESC LIMIT $5`, owner, fid, cursor.ID, cursor.Time, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []any{}
	last := ownerReadCursor{Scope: scope}
	next := ""
	for rows.Next() {
		var id, conversation, text string
		var at time.Time
		if err := rows.Scan(&id, &conversation, &text, &at); err != nil {
			return nil, err
		}
		if len(items) == limit {
			next = encodeOwnerReadCursor(last)
			break
		}
		items = append(items, map[string]any{"id": id, "conversation_id": conversation, "text": text, "created_at": at.UTC().Format(time.RFC3339Nano)})
		last = ownerReadCursor{Scope: scope, ID: id, Time: at}
	}
	return map[string]any{"items": items, "next_cursor": next}, rows.Err()
}

func (a *App) OwnerExpressionLearningSource(ctx context.Context, owner, fid, messageID string) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	source, err := readExpressionLearningSource(ctx, a.DB.Pool(), owner, fid, messageID, false)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": source.ID, "conversation_id": source.ConversationID, "text": source.Text}, nil
}

func (a *App) OwnerExpressionLearningSettings(ctx context.Context, owner, fid string) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	var learning, use bool
	var revision int
	err := a.DB.Pool().QueryRow(ctx, `SELECT learning_enabled,use_enabled,revision FROM public.expression_learning_settings WHERE fluctlight_id=$1`, fid).Scan(&learning, &use, &revision)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	return map[string]any{"learning_enabled": learning, "use_enabled": use, "revision": revision}, nil
}

func (a *App) UpdateOwnerExpressionLearningSettings(ctx context.Context, owner, fid string, body map[string]any) (map[string]any, error) {
	expected, ok := parseRevision(body["expected_revision"])
	learning, learningOK := body["learning_enabled"].(bool)
	use, useOK := body["use_enabled"].(bool)
	key := strings.TrimSpace(stringValue(body["idempotency_key"]))
	if !eventKeys(body, "expected_revision", "learning_enabled", "use_enabled", "idempotency_key") || !ok || expected < 0 || !learningOK || !useOK || key == "" || len([]rune(key)) > 256 {
		return nil, invalid("需要配置版本、学习及使用开关和操作标识")
	}
	digest := stableDigest(jsonString(map[string]any{"kind": "expression-settings", "fid": fid, "expected": expected, "learning": learning, "use": use}))
	var result map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := requireOwnerMutableTx(ctx, tx, owner, fid); err != nil {
			return err
		}
		store := postgresSurfaceTx{tx}
		previous, saved, found, err := store.Idempotency(ctx, owner, key)
		if err != nil {
			return err
		}
		if found {
			if previous != digest {
				return conflict("expression_learning_settings", expected, -1)
			}
			result = saved
			result["_replay"] = true
			return nil
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.expression_learning_settings(fluctlight_id) VALUES($1) ON CONFLICT DO NOTHING`, fid); err != nil {
			return err
		}
		var current int
		if err := tx.QueryRow(ctx, `SELECT revision FROM public.expression_learning_settings WHERE fluctlight_id=$1 FOR UPDATE`, fid).Scan(&current); err != nil {
			return err
		}
		if current != expected {
			return conflict("expression_learning_settings", expected, current)
		}
		if _, err := tx.Exec(ctx, `UPDATE public.expression_learning_settings SET learning_generation=learning_generation+CASE WHEN learning_enabled IS DISTINCT FROM $2 THEN 1 ELSE 0 END,use_generation=use_generation+CASE WHEN use_enabled IS DISTINCT FROM $3 THEN 1 ELSE 0 END,learning_enabled=$2,use_enabled=$3,revision=revision+1,updated_at=now() WHERE fluctlight_id=$1`, fid, learning, use); err != nil {
			return err
		}
		result = map[string]any{"learning_enabled": learning, "use_enabled": use, "revision": current + 1}
		return store.SaveIdempotency(ctx, owner, key, digest, result)
	})
	return result, err
}

func (a *App) OwnerExpressionLearningCandidates(ctx context.Context, owner, fid string) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT c.id,c.source_message_id,c.source_conversation_id,c.source_actor_id,c.excerpt,c.situation,c.trigger_text,c.status,c.revision,
		EXISTS(SELECT 1 FROM public.conversation_messages m JOIN public.actors actor ON actor.id=m.author_actor_id AND actor.actor_type='human' AND actor.status='active'
		 WHERE m.id=c.source_message_id AND m.conversation_id=c.source_conversation_id AND m.author_actor_id=c.source_actor_id AND m.kind='user'
		AND md5(m.text || m.attachment_refs::text)=c.source_fingerprint AND COALESCE((SELECT v.revision FROM public.expression_learning_source_versions v WHERE v.message_id=m.id),0)=c.source_revision
		 AND EXISTS(SELECT 1 FROM public.conversation_participants p WHERE p.conversation_id=m.conversation_id AND p.actor_id=c.owner_actor_id AND p.status='active')
		 AND EXISTS(SELECT 1 FROM public.conversation_participants p WHERE p.conversation_id=m.conversation_id AND p.actor_id=c.fluctlight_id AND p.status='active')),
		COALESCE((SELECT m.text FROM public.conversation_messages m WHERE m.id=c.source_message_id AND m.conversation_id=c.source_conversation_id AND m.author_actor_id=c.source_actor_id),''),
		COALESCE((SELECT jsonb_agg(jsonb_build_object('revision',r.candidate_revision,'action',r.action,'reason',r.reason,'created_at',r.created_at) ORDER BY r.candidate_revision) FROM public.expression_learning_reviews r WHERE r.candidate_id=c.id),'[]'::jsonb)
		FROM public.expression_learning_candidates c WHERE c.fluctlight_id=$1 AND c.owner_actor_id=$2 ORDER BY c.updated_at DESC,c.id`, fid, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []any{}
	for rows.Next() {
		var id, sourceID, conversation, actor, excerpt, situation, trigger, status, sourceText string
		var revision int
		var sourceValid bool
		var reviews []byte
		if err := rows.Scan(&id, &sourceID, &conversation, &actor, &excerpt, &situation, &trigger, &status, &revision, &sourceValid, &sourceText, &reviews); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{"id": id, "source_message_id": sourceID, "source_conversation_id": conversation, "source_actor_id": actor, "source_text": sourceText, "source_valid": sourceValid, "excerpt": excerpt, "situation": situation, "trigger_text": trigger, "status": status, "revision": revision, "reviews": decodeArray(reviews)})
	}
	return map[string]any{"items": items}, rows.Err()
}

func (a *App) ReviewOwnerExpressionLearningCandidate(ctx context.Context, owner, fid, id, action string, body map[string]any) (map[string]any, error) {
	expected, ok := parseRevision(body["expected_revision"])
	reason := strings.TrimSpace(stringValue(body["reason"]))
	trigger := strings.TrimSpace(stringValue(body["trigger_text"]))
	key := strings.TrimSpace(stringValue(body["idempotency_key"]))
	if !eventKeys(body, "expected_revision", "reason", "trigger_text", "idempotency_key") || !ok || expected < 1 || reason == "" || len([]rune(reason)) > 1024 || key == "" || len([]rune(key)) > 256 || len([]rune(trigger)) > 256 || (action != "enable" && action != "disable" && action != "revoke") || (action == "enable" && trigger == "") {
		return nil, invalid("需要候选版本、审核理由、操作标识；启用时需要适用触发词")
	}
	digest := stableDigest(jsonString(map[string]any{"kind": "expression-review", "fid": fid, "id": id, "action": action, "expected": expected, "reason": reason, "trigger": trigger}))
	var result map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := requireOwnerMutableTx(ctx, tx, owner, fid); err != nil {
			return err
		}
		store := postgresSurfaceTx{tx}
		previous, saved, found, err := store.Idempotency(ctx, owner, key)
		if err != nil {
			return err
		}
		if found {
			if previous != digest {
				return conflict("expression_candidate", expected, -1)
			}
			result = saved
			result["_replay"] = true
			return nil
		}
		if action == "enable" {
			if err := lockExpressionSourceGenerationTx(ctx, tx, fid); err != nil {
				return err
			}
		}
		var status, messageID, fingerprint, excerpt, situation string
		var revision int
		var sourceRevision int64
		err = tx.QueryRow(ctx, `SELECT status,revision,source_message_id,source_fingerprint,source_revision,excerpt,situation FROM public.expression_learning_candidates WHERE id=$1 AND fluctlight_id=$2 AND owner_actor_id=$3 FOR UPDATE`, id, fid, owner).Scan(&status, &revision, &messageID, &fingerprint, &sourceRevision, &excerpt, &situation)
		if errors.Is(err, pgx.ErrNoRows) {
			return &SurfaceError{Status: 404, Code: "expression_candidate_not_found", Message: "表达候选不存在"}
		}
		if err != nil {
			return err
		}
		if revision != expected {
			return conflict("expression_candidate", expected, revision)
		}
		if status == "revoked" || (action == "disable" && status != "enabled") || (action == "enable" && status == "enabled") {
			return &SurfaceError{Status: 409, Code: "expression_candidate_transition_invalid", Message: "当前状态不支持此操作"}
		}
		if action == "enable" {
			if !expressionExcerptEligible(excerpt) || !expressionExcerptEligible(situation) {
				return &SurfaceError{Status: 409, Code: "expression_candidate_unsafe", Message: "该候选包含不适合作为表达参考的指令内容"}
			}
			source, err := readExpressionLearningSource(ctx, tx, owner, fid, messageID, false)
			var unavailable *SurfaceError
			if err != nil && !errors.As(err, &unavailable) {
				return err
			}
			if err != nil || source.Fingerprint != fingerprint || source.Revision != sourceRevision {
				return &SurfaceError{Status: 409, Code: "expression_source_changed", Message: "来源已变化，不能启用这条候选"}
			}
		}
		next := map[string]string{"enable": "enabled", "disable": "disabled", "revoke": "revoked"}[action]
		if _, err := tx.Exec(ctx, `UPDATE public.expression_learning_candidates SET status=$2,revision=revision+1,trigger_text=CASE WHEN $3='enable' THEN $4 ELSE trigger_text END,updated_at=now() WHERE id=$1`, id, next, action, trigger); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.expression_learning_reviews(id,candidate_id,candidate_revision,owner_actor_id,action,reason,idempotency_key,request_digest) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, randomID("expression_review_"), id, revision+1, owner, action, reason, key, digest); err != nil {
			return err
		}
		result = map[string]any{"id": id, "status": next, "revision": revision + 1, "action": action}
		return store.SaveIdempotency(ctx, owner, key, digest, result)
	})
	return result, err
}

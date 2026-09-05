package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (a *App) RevokeAll(ctx context.Context, actorID string) error {
	_, err := a.DB.Pool().Exec(ctx, `UPDATE public.auth_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE human_actor_id=$1`, actorID)
	a.authAudit(ctx, "revoke_all", actorID, auditResult(err), "")
	return err
}

func (a *App) RevokeCurrent(ctx context.Context, actorID, token string) error {
	if strings.TrimSpace(token) == "" {
		a.authAudit(ctx, "revoke_current", actorID, "failed", "token_invalid")
		return ErrUnauthorized
	}
	command, err := a.DB.Pool().Exec(ctx, `UPDATE public.auth_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE human_actor_id=$1 AND token_hash=$2`, actorID, digestToken(token))
	if err != nil {
		a.authAudit(ctx, "revoke_current", actorID, "failed", "persistence_failure")
		return err
	}
	if command.RowsAffected() == 0 {
		a.authAudit(ctx, "revoke_current", actorID, "failed", "session_not_found")
		return ErrUnauthorized
	}
	a.authAudit(ctx, "revoke_current", actorID, "success", "")
	return nil
}

func (a *App) ResetPassword(ctx context.Context, actorID, password string) error {
	if len(password) < 6 {
		a.authAudit(ctx, "reset_password", actorID, "failed", "password_invalid")
		return errors.New("password_invalid")
	}
	hash, err := hashArgon2ID(password)
	if err != nil {
		return err
	}
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		command, err := tx.Exec(ctx, `UPDATE public.owner_accounts SET credential_hash=$2,credential_revision=$3,updated_at=now() WHERE human_actor_id=$1`, actorID, hash, randomID("credential_"))
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return ErrUnauthorized
		}
		_, err = tx.Exec(ctx, `UPDATE public.auth_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE human_actor_id=$1`, actorID)
		return err
	})
	a.authAudit(ctx, "reset_password", actorID, auditResult(err), "")
	return err
}

func auditResult(err error) string {
	if err != nil {
		return "failed"
	}
	return "success"
}

func (a *App) CreateConversation(ctx context.Context, ownerID string, participants []any, title string) (map[string]any, error) {
	if len(participants) == 0 {
		return nil, errors.New("conversation_participants_required")
	}
	conversationID := randomID("conversation_")
	ids := make([]string, 0, len(participants)+1)
	ids = append(ids, ownerID)
	for _, raw := range participants {
		if id := stringValue(raw); id != "" && id != ownerID {
			ids = append(ids, id)
		}
	}
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		for _, id := range ids[1:] {
			var actorType, createdBy string
			if err := tx.QueryRow(ctx, `SELECT a.actor_type,COALESCE(f.created_by_actor_id,'') FROM public.actors a LEFT JOIN public.fluctlights f ON f.id=a.id WHERE a.id=$1`, id).Scan(&actorType, &createdBy); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return ErrNotFound
				}
				return err
			}
			if actorType == "fluctlight" && createdBy != ownerID {
				return ErrUnauthorized
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.conversations(id,created_by_actor_id,title,revision) VALUES($1,$2,$3,0)`, conversationID, ownerID, nullableString(title)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.conversation_heads(conversation_id,next_sequence) VALUES($1,1)`, conversationID); err != nil {
			return err
		}
		for i, id := range ids {
			role := "member"
			if i == 0 {
				role = "owner"
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,$3,'active') ON CONFLICT DO NOTHING`, conversationID, id, role); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.conversation_read_positions(conversation_id,actor_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, conversationID, id); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	page, err := a.DB.History(ctx, conversationID, ownerID, nil, 50)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": conversationID, "conversation": page.Conversation, "participants": page.Participants, "messages": page.Messages}, nil
}

func (a *App) MarkRead(ctx context.Context, actorID, conversationID string, payload map[string]any) error {
	read := intValue(payload["read_sequence"])
	delivered := intValue(payload["delivered_sequence"])
	if read < 0 || delivered < 0 || delivered < read {
		return errors.New("conversation_read_position_invalid")
	}
	command, err := a.DB.Pool().Exec(ctx, `UPDATE public.conversation_read_positions SET last_read_sequence=GREATEST(last_read_sequence,$3),last_delivered_sequence=GREATEST(last_delivered_sequence,$4),updated_at=now() WHERE conversation_id=$1 AND actor_id=$2`, conversationID, actorID, read, delivered)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (a *App) SetFluctlightStatus(ctx context.Context, actorID, id, status, reason string, expected *int) (map[string]any, error) {
	if status != "active" && status != "paused" {
		return nil, errors.New("status_invalid")
	}
	return a.setFluctlightLifecycle(ctx, actorID, id, status, reason, expected)
}

func (a *App) RetireFluctlight(ctx context.Context, actorID, id, reason string, expected *int) (map[string]any, error) {
	return a.setFluctlightLifecycle(ctx, actorID, id, "retired", reason, expected)
}

func (a *App) setFluctlightLifecycle(ctx context.Context, actorID, id, status, reason string, expected *int) (map[string]any, error) {
	if strings.TrimSpace(reason) == "" || len([]rune(reason)) > 1024 {
		return nil, errors.New("fluctlight_reason_invalid")
	}
	if expected == nil {
		return nil, errors.New("expected_revision_required")
	}
	var result map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var currentRevision, lifecycleRevision int
		var currentStatus, owner string
		var retiredAt *time.Time
		if err := tx.QueryRow(ctx, `SELECT created_by_actor_id,status,current_revision,lifecycle_revision,retired_at FROM public.fluctlights WHERE id=$1 FOR UPDATE`, id).Scan(&owner, &currentStatus, &currentRevision, &lifecycleRevision, &retiredAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if owner != actorID {
			return ErrUnauthorized
		}
		if currentStatus == "retired" && status != "retired" {
			return errors.New("fluctlight_retired")
		}
		// The public contract's expected_revision is the Fluctlight aggregate
		// revision exposed by detail.current_revision. Lifecycle transitions do
		// not create a foundation revision, so compare against that value while
		// retaining a separate lifecycle audit counter.
		if expected != nil && *expected != currentRevision {
			return ErrConflict
		}
		if currentStatus == status {
			result = map[string]any{"id": id, "status": currentStatus, "revision": lifecycleRevision, "current_revision": currentRevision, "reason": reason}
			if retiredAt != nil {
				result["retired_at"] = retiredAt.Format(time.RFC3339Nano)
			}
			return nil
		}
		lifecycleRevision++
		if err := tx.QueryRow(ctx, `UPDATE public.fluctlights SET status=$2::varchar,lifecycle_revision=$3,retired_at=CASE WHEN $2::varchar='retired' THEN COALESCE(retired_at,now()) ELSE retired_at END,updated_at=now() WHERE id=$1 RETURNING retired_at`, id, status, lifecycleRevision).Scan(&retiredAt); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_governance(id,fluctlight_id,revision,from_status,to_status,actor_id,reason) VALUES($1,$2,$3,$4,$5,$6,$7)`, randomID("fluctlight_governance_"), id, lifecycleRevision, currentStatus, status, actorID, nullableString(reason)); err != nil {
			return err
		}
		result = map[string]any{"id": id, "status": status, "revision": lifecycleRevision, "current_revision": currentRevision, "reason": reason}
		if retiredAt != nil {
			result["retired_at"] = retiredAt.Format(time.RFC3339Nano)
		}
		return nil
	})
	return result, err
}

func (a *App) ProposeFoundation(ctx context.Context, actorID, fluctlightID string, payload map[string]any) (map[string]any, error) {
	f, err := a.DB.GetFluctlight(ctx, fluctlightID, actorID)
	if err != nil {
		return nil, err
	}
	changes := mapValue(payload["changes"])
	if len(changes) == 0 {
		return nil, errors.New("foundation_changes_invalid")
	}
	if err := validateFoundationChanges(changes); err != nil {
		return nil, err
	}
	var current int
	var initializationMode, lifecycleStatus string
	var foundationCreatedAt time.Time
	if err := a.DB.Pool().QueryRow(ctx, `SELECT current_revision,initialization_mode,status,created_at FROM public.fluctlights WHERE id=$1 AND created_by_actor_id=$2`, fluctlightID, actorID).Scan(&current, &initializationMode, &lifecycleStatus, &foundationCreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if raw, ok := payload["expected_revision"]; !ok || raw == nil {
		return nil, errors.New("expected_revision_required")
	} else if intValue(raw) != current {
		return nil, ErrConflict
	}
	candidate := map[string]any{
		"identity":          mergeFoundationMap(f.Identity, mapValue(changes["identity"])),
		"personality":       mergeFoundationMap(f.Personality, mapValue(changes["personality"])),
		"behavioral_policy": mergeFoundationMap(f.BehavioralPolicy, mapValue(changes["behavioral_policy"])),
		"life_profile":      mergeFoundationMap(f.LifeProfile, mapValue(changes["life_profile"])),
		"provenance":        mergeFoundationMap(f.Provenance, mapValue(changes["provenance"])),
	}
	corePersona := mergeFoundationMap(f.CorePersona, map[string]any{
		"identity":          candidate["identity"],
		"personality":       candidate["personality"],
		"behavioral_policy": candidate["behavioral_policy"],
		"life_profile":      candidate["life_profile"],
	})
	if _, ok := corePersona["schema_version"]; !ok {
		corePersona["schema_version"] = 1
	}
	identityPatch := make(map[string]any)
	for key, value := range changes {
		if _, known := map[string]struct{}{"name": {}, "age": {}, "gender": {}, "occupation": {}, "residence": {}, "timezone": {}, "birthday": {}, "background": {}, "biography": {}, "core_values": {}, "worldview": {}, "notes": {}}[key]; known {
			identityPatch[key] = value
		}
	}
	if len(identityPatch) > 0 {
		candidate["identity"] = mergeFoundationMap(candidate["identity"].(map[string]any), identityPatch)
	}
	if value, ok := changes["identity"].(map[string]any); ok && len(value) > 0 {
		candidate["identity"] = mergeFoundationMap(f.Identity, value)
	}
	corePersona["identity"] = candidate["identity"]
	corePersona["personality"] = candidate["personality"]
	corePersona["behavioral_policy"] = candidate["behavioral_policy"]
	corePersona["life_profile"] = candidate["life_profile"]
	candidate["core_persona"] = corePersona
	id := randomID("foundation_revision_")
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		evidence := arrayValue(payload["evidence_refs"])
		if len(evidence) == 0 {
			evidence = []any{"owner:" + actorID}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_foundation_revisions(id,fluctlight_id,revision,base_revision,source,status,actor_id,initialization_mode,foundation_status,foundation_created_at,confidence,changes,core_persona,identity,personality,behavioral_policy,life_profile,provenance,evidence_refs,reason,idempotency_key) VALUES($1,$2,$3,$4,'human','proposed',$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`, id, fluctlightID, current+1, current, actorID, initializationMode, lifecycleStatus, foundationCreatedAt, jsonBytes(1.0), jsonBytes(changes), jsonBytes(candidate["core_persona"]), jsonBytes(candidate["identity"]), jsonBytes(candidate["personality"]), jsonBytes(candidate["behavioral_policy"]), jsonBytes(candidate["life_profile"]), jsonBytes(candidate["provenance"]), jsonBytes(evidence), stringValue(payload["reason"]), "foundation:"+id); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_foundation_governance(id,fluctlight_id,revision_id,action,actor_id,reason) VALUES($1,$2,$3,'propose',$4,$5)`, randomID("foundation_governance_"), fluctlightID, id, actorID, nullableString(stringValue(payload["reason"])))
		return err
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "fluctlight_id": fluctlightID, "revision": current + 1, "base_revision": current, "status": "proposed", "changes": changes, "core_persona": candidate["core_persona"], "identity": candidate["identity"], "personality": candidate["personality"], "behavioral_policy": candidate["behavioral_policy"], "life_profile": candidate["life_profile"], "provenance": candidate["provenance"]}, nil
}

func (a *App) SetFoundationDecision(ctx context.Context, actorID, fluctlightID, revisionID, action, reason string) (map[string]any, error) {
	return a.SetFoundationDecisionExpected(ctx, actorID, fluctlightID, revisionID, action, reason, nil)
}

func (a *App) SetFoundationDecisionExpected(ctx context.Context, actorID, fluctlightID, revisionID, action, reason string, expected *int) (map[string]any, error) {
	if action != "accept" && action != "reject" {
		return nil, errors.New("foundation_decision_invalid")
	}
	if strings.TrimSpace(reason) == "" || len([]rune(reason)) > 1024 {
		return nil, errors.New("foundation_reason_invalid")
	}
	var result map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var revision, baseRevision int
		var status string
		var corePersona, identity, personality, policy, life, provenance []byte
		if err := tx.QueryRow(ctx, `SELECT revision,base_revision,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance FROM public.fluctlight_foundation_revisions WHERE id=$1 AND fluctlight_id=$2 AND actor_id=$3 FOR UPDATE`, revisionID, fluctlightID, actorID).Scan(&revision, &baseRevision, &status, &corePersona, &identity, &personality, &policy, &life, &provenance); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if status != "proposed" {
			return errors.New("foundation_revision_not_proposed")
		}
		var current int
		var lifecycleStatus string
		if err := tx.QueryRow(ctx, `SELECT current_revision,status FROM public.fluctlights WHERE id=$1 AND created_by_actor_id=$2 FOR UPDATE`, fluctlightID, actorID).Scan(&current, &lifecycleStatus); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if expected != nil && current != *expected {
			return ErrConflict
		}
		if action == "accept" {
			if current != baseRevision {
				return ErrConflict
			}
			if expected != nil && current != *expected {
				return ErrConflict
			}
			if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_foundation_revisions SET status='accepted',foundation_status=$2,accepted_at=now(),reason=$3 WHERE id=$1`, revisionID, lifecycleStatus, nullableString(reason)); err != nil {
				return err
			}
			if len(decodeObject(corePersona)) == 0 {
				corePersona = jsonBytes(map[string]any{"schema_version": 1, "identity": decodeObject(identity), "personality": decodeObject(personality), "behavioral_policy": decodeObject(policy), "life_profile": decodeObject(life)})
			}
			if _, err := tx.Exec(ctx, `UPDATE public.fluctlights SET current_revision=$2,core_persona=$3,identity=$4,personality=$5,behavioral_policy=$6,life_profile=$7,provenance=$8,updated_at=now() WHERE id=$1`, fluctlightID, revision, corePersona, identity, personality, policy, life, provenance); err != nil {
				return err
			}
		} else if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_foundation_revisions SET status='rejected',foundation_status=$2,rejected_at=now(),reason=$3 WHERE id=$1`, revisionID, lifecycleStatus, nullableString(reason)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_foundation_governance(id,fluctlight_id,revision_id,action,actor_id,reason) VALUES($1,$2,$3,$4,$5,$6)`, randomID("foundation_governance_"), fluctlightID, revisionID, action, actorID, nullableString(reason)); err != nil {
			return err
		}
		result = map[string]any{"id": revisionID, "fluctlight_id": fluctlightID, "revision": revision, "status": action + "ed", "reason": reason}
		return nil
	})
	return result, err
}

func (a *App) RollbackFoundation(ctx context.Context, actorID, fluctlightID string, payload map[string]any) (map[string]any, error) {
	target, ok := nonNegativeRevision(payload["target_revision"])
	if !ok {
		return nil, errors.New("target_revision_invalid")
	}
	reason := stringValue(payload["reason"])
	if reason == "" || len([]rune(reason)) > 1024 {
		return nil, errors.New("foundation_reason_invalid")
	}
	expected, expectedOK := nonNegativeRevision(payload["expected_revision"])
	if !expectedOK {
		return nil, errors.New("expected_revision_invalid")
	}
	var result map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var current int
		if err := tx.QueryRow(ctx, `SELECT current_revision FROM public.fluctlights WHERE id=$1 AND created_by_actor_id=$2 FOR UPDATE`, fluctlightID, actorID).Scan(&current); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if current != expected {
			return ErrConflict
		}
		if target >= current {
			return errors.New("target_revision_invalid")
		}
		var sourceID, sourceStatus string
		var corePersona, identity, personality, policy, life, provenance, evidence []byte
		if err := tx.QueryRow(ctx, `SELECT id,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance,evidence_refs FROM public.fluctlight_foundation_revisions WHERE fluctlight_id=$1 AND revision=$2`, fluctlightID, target).Scan(&sourceID, &sourceStatus, &corePersona, &identity, &personality, &policy, &life, &provenance, &evidence); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if sourceStatus != "accepted" {
			return errors.New("foundation_target_not_accepted")
		}
		newRevision := current + 1
		newID := randomID("foundation_revision_")
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_foundation_revisions(id,fluctlight_id,revision,base_revision,source,status,actor_id,initialization_mode,foundation_status,foundation_created_at,confidence,changes,core_persona,identity,personality,behavioral_policy,life_profile,provenance,evidence_refs,reason,idempotency_key) SELECT $1,fluctlight_id,$2,$3,'owner_rollback','accepted',$4,initialization_mode,'active',foundation_created_at,confidence,jsonb_build_object('rollback_from_revision',$5),core_persona,identity,personality,behavioral_policy,life_profile,provenance,evidence_refs,$6,$7 FROM public.fluctlight_foundation_revisions WHERE id=$8`, newID, newRevision, current, actorID, target, nullableString(reason), "foundation-rollback:"+fluctlightID+":"+fmt.Sprint(target), sourceID); err != nil {
			return err
		}
		if len(decodeObject(corePersona)) == 0 {
			corePersona = jsonBytes(map[string]any{"schema_version": 1, "identity": decodeObject(identity), "personality": decodeObject(personality), "behavioral_policy": decodeObject(policy), "life_profile": decodeObject(life)})
		}
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlights SET current_revision=$2,core_persona=$3,identity=$4,personality=$5,behavioral_policy=$6,life_profile=$7,provenance=$8,updated_at=now() WHERE id=$1`, fluctlightID, newRevision, corePersona, identity, personality, policy, life, provenance); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_foundation_governance(id,fluctlight_id,revision_id,action,actor_id,reason) VALUES($1,$2,$3,'rollback',$4,$5)`, randomID("foundation_governance_"), fluctlightID, newID, actorID, nullableString(reason)); err != nil {
			return err
		}
		result = map[string]any{"id": newID, "fluctlight_id": fluctlightID, "revision": newRevision, "target_revision": target, "status": "accepted", "reason": reason}
		_ = evidence
		return nil
	})
	return result, err
}

func positiveRevision(value any) (int, bool) {
	parsed := intValue(value)
	return parsed, parsed > 0
}

func mergeFoundationMap(base, changes map[string]any) map[string]any {
	result := make(map[string]any, len(base)+len(changes))
	for key, value := range base {
		result[key] = value
	}
	for key, value := range changes {
		if nested, ok := value.(map[string]any); ok {
			if existing, ok := result[key].(map[string]any); ok {
				result[key] = mergeFoundationMap(existing, nested)
				continue
			}
		}
		result[key] = value
	}
	return result
}

func validateFoundationChanges(changes map[string]any) error {
	for key, value := range changes {
		switch key {
		case "identity", "personality", "behavioral_policy", "life_profile":
			if _, ok := value.(map[string]any); !ok {
				return errors.New("foundation_changes_invalid")
			}
		case "provenance":
			provenance, ok := value.(map[string]any)
			if !ok {
				return errors.New("foundation_changes_invalid")
			}
			if _, forbidden := provenance["self_model"]; forbidden {
				return errors.New("foundation_self_model_forbidden")
			}
		case "id", "initialization_mode", "status", "current_revision", "created_at":
			return errors.New("foundation_field_immutable")
		default:
			if _, known := map[string]struct{}{"name": {}, "age": {}, "gender": {}, "occupation": {}, "residence": {}, "timezone": {}, "birthday": {}, "background": {}, "biography": {}, "core_values": {}, "worldview": {}, "notes": {}}[key]; !known {
				return errors.New("foundation_field_invalid")
			}
		}
	}
	return nil
}

func (a *App) ReviseMemory(ctx context.Context, actorID, id, content string, expected *int, refs []any) (map[string]any, error) {
	if strings.TrimSpace(content) == "" || len([]rune(content)) > 32000 || len(refs) == 0 || expected == nil {
		return nil, errors.New("memory_revision_invalid")
	}
	var owner, old string
	var revision, newRevision int
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT owner_fluctlight_id,content,revision FROM public.memories WHERE id=$1 FOR UPDATE`, id).Scan(&owner, &old, &revision); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		var authorized bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlights WHERE id=$1 AND created_by_actor_id=$2)`, owner, actorID).Scan(&authorized); err != nil {
			return err
		}
		if !authorized {
			return ErrUnauthorized
		}
		if expected != nil && *expected != revision {
			return errors.New("memory_revision_stale")
		}
		newRevision = revision + 1
		if _, err := tx.Exec(ctx, `UPDATE public.memories SET content=$2,revision=$3,evidence_refs=$4,last_confirmed_at=now(),search_document=to_tsvector('simple',$2) WHERE id=$1`, id, content, newRevision, jsonBytes(refs)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.memory_revisions(id,memory_id,revision,base_revision,content,status,actor_id,evidence_refs,idempotency_key) VALUES($1,$2,$3,$4,$5,'active',$6,$7,$8) ON CONFLICT DO NOTHING`, randomID("memory_revision_"), id, newRevision, revision, content, actorID, jsonBytes(refs), "memory:"+id+":"+fmt.Sprint(newRevision)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.memory_embeddings SET status='stale' WHERE memory_id=$1 AND memory_revision<>$2 AND status <> 'stale'`, id, newRevision); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'lifecycle','memory.embedding',$3) ON CONFLICT DO NOTHING`, "memory_embedding_intent:"+id+":"+fmt.Sprint(newRevision), "memory_embedding:"+id+":"+fmt.Sprint(newRevision), jsonBytes(map[string]any{"memory_id": id, "revision": newRevision})); err != nil {
			return err
		}
		if err := appendOutboxTx(ctx, tx, "memory.revised", "memory", id, owner, actorID, "memory:"+id, "memory-outbox:"+id+":"+fmt.Sprint(newRevision), map[string]any{"memory_id": id, "revision": newRevision, "aggregate_sequence": newRevision*2 + 1}); err != nil {
			return err
		}
		return appendOutboxTx(ctx, tx, "memory.embedding.requested", "memory", id, owner, actorID, "memory:"+id, "memory-embedding:"+id+":"+fmt.Sprint(newRevision), map[string]any{"memory_id": id, "revision": newRevision, "aggregate_sequence": newRevision*2 + 2})
	})
	if err != nil {
		return nil, err
	}
	_ = old
	return map[string]any{"id": id, "content": content, "revision": newRevision, "status": "active"}, nil
}

func (a *App) ForgetMemory(ctx context.Context, actorID, id string, expected *int, refs []any) (map[string]any, error) {
	if len(refs) == 0 || expected == nil {
		return nil, errors.New("memory_forget_invalid")
	}
	var owner string
	var revision, newRevision int
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT owner_fluctlight_id,revision FROM public.memories WHERE id=$1 FOR UPDATE`, id).Scan(&owner, &revision); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		var authorized bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlights WHERE id=$1 AND created_by_actor_id=$2)`, owner, actorID).Scan(&authorized); err != nil {
			return err
		}
		if !authorized {
			return ErrUnauthorized
		}
		if expected != nil && *expected != revision {
			return errors.New("memory_revision_stale")
		}
		newRevision = revision + 1
		if _, err := tx.Exec(ctx, `UPDATE public.memories SET status='forgotten',revision=$2,evidence_refs=$3 WHERE id=$1`, id, newRevision, jsonBytes(refs)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.memory_revisions(id,memory_id,revision,base_revision,content,status,actor_id,evidence_refs,idempotency_key) SELECT $1,id,$2,$3,content,'forgotten',$4,$5,$6 FROM public.memories WHERE id=$7 ON CONFLICT DO NOTHING`, randomID("memory_revision_"), newRevision, revision, actorID, jsonBytes(refs), "memory-forget:"+id+":"+fmt.Sprint(newRevision), id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.memory_embeddings SET status='stale' WHERE memory_id=$1 AND status <> 'stale'`, id); err != nil {
			return err
		}
		return appendOutboxTx(ctx, tx, "memory.forgotten", "memory", id, owner, actorID, "memory:"+id, "memory-forget-outbox:"+id+":"+fmt.Sprint(newRevision), map[string]any{"memory_id": id, "revision": newRevision, "aggregate_sequence": newRevision*2 + 1})
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "status": "forgotten", "revision": newRevision, "evidence_refs": refs}, nil
}

func (a *App) GovernAutonomy(ctx context.Context, actorID, actionID, toStatus, reason string) (map[string]any, error) {
	if toStatus != "paused" && toStatus != "cancelled" && toStatus != "failed" && toStatus != "frozen" && toStatus != "completed" {
		return nil, errors.New("autonomy_status_invalid")
	}
	var fluctlightID, from string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT fluctlight_id,status FROM public.autonomy_actions WHERE id=$1`, actionID).Scan(&fluctlightID, &from); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if _, err := a.DB.GetFluctlight(ctx, fluctlightID, actorID); err != nil {
		return nil, ErrUnauthorized
	}
	if err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		command, err := tx.Exec(ctx, `UPDATE public.autonomy_actions SET status=$2::varchar,error_code=CASE WHEN $2::varchar='failed' THEN $3 ELSE error_code END WHERE id=$1 AND status=$4`, actionID, toStatus, reason, from)
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return errors.New("autonomy_status_stale")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.autonomy_governance(id,fluctlight_id,action_id,from_status,to_status,actor_id,reason) VALUES($1,$2,$3,$4,$5,$6,$7)`, randomID("autonomy_governance_"), fluctlightID, actionID, from, toStatus, actorID, reason); err != nil {
			return err
		}
		return appendOutboxTx(ctx, tx, "autonomy.governed", "autonomy_action", actionID, fluctlightID, actorID, "autonomy:"+actionID, "autonomy-governance:"+actionID+":"+toStatus, map[string]any{"from_status": from, "to_status": toStatus})
	}); err != nil {
		return nil, err
	}
	return map[string]any{"id": actionID, "status": toStatus, "from_status": from, "reason": reason}, nil
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func (a *App) ConfigureProviderRole(ctx context.Context, actorID string, payload map[string]any) error {
	if _, err := a.ReadSettings(ctx, actorID); err != nil {
		return err
	}
	role, endpoint, model := stringValue(payload["role"]), stringValue(payload["endpoint_id"]), stringValue(payload["model_id"])
	if !validProviderRole(role) || endpoint == "" || model == "" {
		return errors.New("provider_role_invalid")
	}
	bindingRole := providerBindingRole(role)
	budget := intValue(payload["token_budget"])
	timeout := intValue(payload["timeout_seconds"])
	if budget <= 0 {
		budget = 4096
	}
	if timeout <= 0 {
		timeout = 120
	}
	var endpointExists bool
	if err := a.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.provider_endpoints WHERE id=$1)`, endpoint).Scan(&endpointExists); err != nil {
		return err
	}
	if !endpointExists {
		return errors.New("provider_endpoint_not_found")
	}
	models, err := a.ProviderModels(ctx, actorID, endpoint)
	if err != nil {
		return fmt.Errorf("provider_models_unavailable: %w", err)
	}
	modelAvailable := false
	for _, raw := range arrayValue(models["models"]) {
		if stringValue(raw) == model {
			modelAvailable = true
			break
		}
	}
	if !modelAvailable {
		return errors.New("provider_model_not_available")
	}
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,token_budget,timeout_seconds,required_capabilities,retry_policy) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(role) DO UPDATE SET provider_endpoint_id=excluded.provider_endpoint_id,model_id=excluded.model_id,token_budget=excluded.token_budget,timeout_seconds=excluded.timeout_seconds,required_capabilities=excluded.required_capabilities,retry_policy=excluded.retry_policy`, bindingRole, endpoint, model, budget, timeout, stringValue(payload["required_capabilities"]), jsonString(mapValue(payload["retry_policy"]))); err != nil {
			return err
		}
		preflightID := "provider_preflight_" + stableDigest(bindingRole+":"+endpoint+":"+model)
		if _, err := tx.Exec(ctx, `INSERT INTO public.provider_preflights(id,role,result,capability_version,checked_at) VALUES($1,$2,'available',$3,now()) ON CONFLICT(id) DO UPDATE SET result='available',capability_version=excluded.capability_version,checked_at=now()`, preflightID, bindingRole, "models-v1"); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE public.provider_endpoints SET capability_status='available',checked_at=now() WHERE id=$1`, endpoint)
		return err
	})
}

func (a *App) ActorGroups(ctx context.Context, owner string) ([]map[string]any, error) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,name,created_at FROM public.actor_groups WHERE owner_actor_id=$1 ORDER BY created_at,id`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id, name string
		var created time.Time
		if err := rows.Scan(&id, &name, &created); err != nil {
			return nil, err
		}
		members := make([]string, 0)
		mrows, e := a.DB.Pool().Query(ctx, `SELECT actor_id FROM public.actor_group_members WHERE group_id=$1 ORDER BY actor_id`, id)
		if e != nil {
			return nil, e
		}
		for mrows.Next() {
			var m string
			_ = mrows.Scan(&m)
			members = append(members, m)
		}
		mrows.Close()
		out = append(out, map[string]any{"id": id, "name": name, "owner_actor_id": owner, "actor_ids": members, "created_at": created.Format(time.RFC3339Nano)})
	}
	return out, rows.Err()
}
func (a *App) CreateActorGroup(ctx context.Context, owner, name string) (map[string]any, error) {
	if strings.TrimSpace(name) == "" || len([]rune(name)) > 60 {
		return nil, errors.New("actor_group_name_invalid")
	}
	id := randomID("actor_group_")
	_, err := a.DB.Pool().Exec(ctx, `INSERT INTO public.actor_groups(id,owner_actor_id,name) VALUES($1,$2,$3)`, id, owner, name)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "name": name, "owner_actor_id": owner, "actor_ids": []string{}}, nil
}
func (a *App) SetActorGroupMember(ctx context.Context, owner, groupID, actorID string, add bool) error {
	var groupOwner string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT owner_actor_id FROM public.actor_groups WHERE id=$1`, groupID).Scan(&groupOwner); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if groupOwner != owner {
		return ErrUnauthorized
	}
	if add {
		_, err := a.DB.Pool().Exec(ctx, `INSERT INTO public.actor_group_members(group_id,actor_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, groupID, actorID)
		return err
	}
	_, err := a.DB.Pool().Exec(ctx, `DELETE FROM public.actor_group_members WHERE group_id=$1 AND actor_id=$2`, groupID, actorID)
	return err
}

func (a *App) RollbackRelationship(ctx context.Context, actorID, fluctlightID string, payload map[string]any) (map[string]any, error) {
	target := stringValue(payload["target_actor_id"])
	if target == "" {
		return nil, errors.New("relationship_target_invalid")
	}
	if _, err := a.DB.GetFluctlight(ctx, fluctlightID, actorID); err != nil {
		return nil, ErrUnauthorized
	}
	expected, ok := nonNegativeRevision(payload["expected_revision"])
	if !ok {
		return nil, errors.New("relationship_expected_revision_invalid")
	}
	targetRevision, ok := nonNegativeRevision(payload["target_revision"])
	if !ok {
		return nil, errors.New("relationship_target_revision_invalid")
	}
	evidence := arrayValue(payload["evidence_refs"])
	if len(evidence) == 0 {
		return nil, errors.New("relationship_evidence_required")
	}
	var result map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var id string
		var revision int
		var metrics, emotional []byte
		var trend string
		var summary *string
		if err := tx.QueryRow(ctx, `SELECT id,revision,metrics,trend,summary,emotional_association FROM public.relationships WHERE owner_fluctlight_id=$1 AND target_actor_id=$2 FOR UPDATE`, fluctlightID, target).Scan(&id, &revision, &metrics, &trend, &summary, &emotional); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if revision != expected {
			return ErrConflict
		}
		var sourceRevision int
		var sourceMetrics, sourceEmotional []byte
		var sourceTrend string
		var sourceSummary *string
		if err := tx.QueryRow(ctx, `SELECT revision,metrics,trend,summary,emotional_association FROM public.relationship_revisions WHERE relationship_id=$1 AND revision=$2`, id, targetRevision).Scan(&sourceRevision, &sourceMetrics, &sourceTrend, &sourceSummary, &sourceEmotional); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		newRevision := revision + 1
		refs := append(append([]any{}, evidence...), "rollback:"+fmt.Sprint(targetRevision))
		if _, err := tx.Exec(ctx, `UPDATE public.relationships SET metrics=$2,trend=$3,summary=$4,emotional_association=$5,revision=$6,updated_at=now() WHERE id=$1 AND revision=$7`, id, sourceMetrics, sourceTrend, sourceSummary, sourceEmotional, newRevision, expected); err != nil {
			return err
		}
		newRevisionID := randomID("relationship_revision_")
		if _, err := tx.Exec(ctx, `INSERT INTO public.relationship_revisions(id,relationship_id,revision,base_revision,metrics,trend,summary,emotional_association,evidence_refs,actor_id,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, newRevisionID, id, newRevision, expected, sourceMetrics, sourceTrend, sourceSummary, sourceEmotional, jsonBytes(refs), actorID, "relationship-rollback:"+id+":"+fmt.Sprint(targetRevision)+":"+fmt.Sprint(expected)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.relationship_governance(id,relationship_id,revision_id,action,actor_id,reason) VALUES($1,$2,$3,'rollback',$4,$5)`, randomID("relationship_governance_"), id, newRevisionID, actorID, nullableString(stringValue(payload["reason"]))); err != nil {
			return err
		}
		result = map[string]any{"id": id, "relationship_id": id, "revision": newRevision, "target_revision": sourceRevision, "status": "rolled_back"}
		_ = metrics
		_ = emotional
		return nil
	})
	return result, err
}

func nonNegativeRevision(value any) (int, bool) {
	parsed := intValue(value)
	return parsed, parsed >= 0
}

func (a *App) CreateLifeEvent(ctx context.Context, actorID, fluctlightID string, payload map[string]any) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fluctlightID, actorID); err != nil {
		return nil, err
	}
	start, e1 := parseScheduleTime(stringValue(payload["start_at"]))
	end, e2 := parseScheduleTime(stringValue(payload["end_at"]))
	if e1 != nil || e2 != nil || !end.After(start) {
		return nil, errors.New("life_event_time_invalid")
	}
	refs := arrayValue(payload["evidence_refs"])
	if len(refs) == 0 {
		return nil, errors.New("life_event_evidence_required")
	}
	idempotency := stringValue(payload["idempotency_key"])
	if idempotency == "" {
		idempotency = "owner:" + actorID + ":" + start.UTC().Format(time.RFC3339Nano) + ":" + stringValue(payload["kind"])
	}
	id := "event_" + stableDigest(fluctlightID+":"+idempotency)
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO public.life_events(id,fluctlight_id,kind,start_at,end_at,scene,activity,location,status,evidence_refs,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'confirmed',$9,$10) ON CONFLICT(fluctlight_id,idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING`, id, fluctlightID, stringValue(payload["kind"]), start, end, nullableString(stringValue(payload["scene"])), nullableString(stringValue(payload["activity"])), nullableString(stringValue(payload["location"])), jsonBytes(refs), idempotency); err != nil {
			return err
		}
		if _, err := a.enqueueNativeFactTx(ctx, tx, fluctlightID, stringValue(payload["conversation_id"]), "owner:"+actorID, "life.event.created", "life-event:"+idempotency, map[string]any{"event_id": id, "kind": stringValue(payload["kind"])}); err != nil {
			return err
		}
		return appendOutboxTx(ctx, tx, "life.event.created", "fluctlight", fluctlightID, fluctlightID, actorID, "life:"+id, "life-event:"+idempotency, map[string]any{"event_id": id, "aggregate_sequence": 1})
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "fluctlight_id": fluctlightID, "status": "confirmed", "idempotency_key": idempotency}, nil
}
func (a *App) CancelLifeEvent(ctx context.Context, actorID, fluctlightID, eventID string) error {
	if _, err := a.DB.GetFluctlight(ctx, fluctlightID, actorID); err != nil {
		return err
	}
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		cmd, err := tx.Exec(ctx, `UPDATE public.life_events SET status='cancelled' WHERE id=$1 AND fluctlight_id=$2 AND status <> 'cancelled'`, eventID, fluctlightID)
		if err != nil {
			return err
		}
		if cmd.RowsAffected() == 0 {
			return ErrNotFound
		}
		if _, err := a.enqueueNativeFactTx(ctx, tx, fluctlightID, "", eventID, "life.event.cancelled", "life-cancel:"+eventID, map[string]any{"event_id": eventID}); err != nil {
			return err
		}
		return appendOutboxTx(ctx, tx, "life.event.cancelled", "fluctlight", fluctlightID, fluctlightID, eventID, "life:"+eventID, "life-cancel:"+eventID, map[string]any{"event_id": eventID, "aggregate_sequence": 1})
	})
}
func (a *App) SetPresence(ctx context.Context, actorID, fluctlightID string, payload map[string]any) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fluctlightID, actorID); err != nil {
		return nil, err
	}
	if stringValue(payload["scene"]) != "" || stringValue(payload["activity"]) != "" || stringValue(payload["location"]) != "" {
		return nil, errors.New("presence_overlay_cannot_replace_scene")
	}
	idempotency := stringValue(payload["idempotency_key"])
	if idempotency == "" {
		idempotency = "owner:" + actorID + ":" + time.Now().UTC().Format(time.RFC3339Nano)
	}
	id := "presence_overlay_" + stableDigest(fluctlightID+":"+idempotency)
	var expiresAt any
	if raw := stringValue(payload["expires_at"]); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil || !parsed.After(time.Now().UTC()) {
			return nil, errors.New("presence_expiration_invalid")
		}
		expiresAt = parsed
	}
	currentTask := nullableString(stringValue(payload["current_task"]))
	userPresence := nullableString(stringValue(payload["user_presence"]))
	if currentTask == nil && userPresence == nil {
		return nil, errors.New("presence_overlay_fields_required")
	}
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO public.life_presence_overlays(id,fluctlight_id,actor_id,scene,activity,location,current_task,user_presence,expires_at) VALUES($1,$2,$3,NULL,NULL,NULL,$4,$5,$6) ON CONFLICT(id) DO NOTHING`, id, fluctlightID, actorID, currentTask, userPresence, expiresAt); err != nil {
			return err
		}
		if _, err := a.enqueueNativeFactTx(ctx, tx, fluctlightID, "", "owner:"+actorID, "life.presence.updated", "presence:"+idempotency, map[string]any{"overlay_id": id, "current_task": stringValue(payload["current_task"]), "user_presence": stringValue(payload["user_presence"])}); err != nil {
			return err
		}
		return appendOutboxTx(ctx, tx, "life.presence.updated", "fluctlight", fluctlightID, fluctlightID, actorID, "presence:"+id, "presence:"+idempotency, map[string]any{"overlay_id": id, "aggregate_sequence": 1})
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "fluctlight_id": fluctlightID, "actor_id": actorID, "current_task": payload["current_task"], "user_presence": payload["user_presence"], "expires_at": payload["expires_at"], "idempotency_key": idempotency}, nil
}
func (a *App) CancelSchedule(ctx context.Context, actorID, fluctlightID, scheduleID string) error {
	return a.CancelScheduleExpected(ctx, actorID, fluctlightID, scheduleID, nil)
}

func (a *App) CancelScheduleExpected(ctx context.Context, actorID, fluctlightID, scheduleID string, expected *int) error {
	if _, err := a.DB.GetFluctlight(ctx, fluctlightID, actorID); err != nil {
		return err
	}
	query := `UPDATE public.life_schedules SET status='cancelled' WHERE id=$1 AND fluctlight_id=$2 AND status='accepted'`
	args := []any{scheduleID, fluctlightID}
	if expected != nil {
		query += ` AND revision=$3`
		args = append(args, *expected)
	}
	cmd, err := a.DB.Pool().Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		if expected != nil {
			return ErrConflict
		}
		return ErrNotFound
	}
	return nil
}

func (a *App) AllMoments(ctx context.Context, actorID string, includeHidden bool, limit int) ([]map[string]any, error) {
	if limit < 1 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	query := `SELECT m.id,m.owner_fluctlight_id,m.author_actor_id,m.text,m.visibility,m.status,m.media_asset_ids,m.created_at
		FROM public.moments m
		JOIN public.fluctlights f ON f.id=m.owner_fluctlight_id
		WHERE f.created_by_actor_id=$1 AND f.status<>'retired' AND m.visibility IN ('owner','participants')`
	if !includeHidden {
		query += ` AND m.status<>'hidden'`
	}
	query += ` ORDER BY m.created_at DESC,m.id DESC LIMIT $2`
	rows, err := a.DB.Pool().Query(ctx, query, actorID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id, owner, author, text, visibility, status string
		var media []byte
		var created time.Time
		if err := rows.Scan(&id, &owner, &author, &text, &visibility, &status, &media, &created); err != nil {
			return nil, err
		}
		moment := map[string]any{"id": id, "owner_fluctlight_id": owner, "author_actor_id": author, "text": text, "visibility": visibility, "status": status, "media_asset_ids": decodeArray(media), "created_at": created.Format(time.RFC3339Nano)}
		if err := a.hydrateMoment(ctx, actorID, moment); err != nil {
			return nil, err
		}
		out = append(out, moment)
	}
	return out, rows.Err()
}
func (a *App) MarkMomentsRead(ctx context.Context, actorID, fluctlightID string) error {
	if _, err := a.DB.GetFluctlight(ctx, fluctlightID, actorID); err != nil {
		return err
	}
	_, err := a.DB.Pool().Exec(ctx, `INSERT INTO public.moment_unread_markers(owner_fluctlight_id,actor_id,last_seen_at) VALUES($1,$2,now()) ON CONFLICT(owner_fluctlight_id,actor_id) DO UPDATE SET last_seen_at=now()`, fluctlightID, actorID)
	return err
}
func (a *App) CommentMoment(ctx context.Context, actorID, momentID, text string) (map[string]any, error) {
	if len([]rune(strings.TrimSpace(text))) == 0 || len([]rune(text)) > 32000 {
		return nil, errors.New("moment_comment_invalid")
	}
	if err := a.authorizeMoment(ctx, actorID, momentID); err != nil {
		return nil, err
	}
	id := randomID("comment_")
	_, err := a.DB.Pool().Exec(ctx, `INSERT INTO public.moment_comments(id,moment_id,author_actor_id,text) VALUES($1,$2,$3,$4)`, id, momentID, actorID, strings.TrimSpace(text))
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "moment_id": momentID, "author_actor_id": actorID, "text": strings.TrimSpace(text)}, nil
}
func (a *App) ReactMoment(ctx context.Context, actorID, momentID, kind string) (map[string]any, error) {
	if kind != "like" && kind != "care" && kind != "celebrate" {
		return nil, errors.New("moment_reaction_invalid")
	}
	if err := a.authorizeMoment(ctx, actorID, momentID); err != nil {
		return nil, err
	}
	kind = strings.TrimSpace(kind)
	_, err := a.DB.Pool().Exec(ctx, `INSERT INTO public.moment_reactions(moment_id,actor_id,kind) VALUES($1,$2,$3) ON CONFLICT(moment_id,actor_id) DO UPDATE SET kind=excluded.kind,created_at=now()`, momentID, actorID, kind)
	if err != nil {
		return nil, err
	}
	return map[string]any{"moment_id": momentID, "kind": kind}, nil
}
func (a *App) SetMomentStatus(ctx context.Context, actorID, momentID, status string) error {
	if status != "visible" && status != "hidden" {
		return errors.New("moment_status_invalid")
	}
	cmd, err := a.DB.Pool().Exec(ctx, `UPDATE public.moments m SET status=$2 FROM public.fluctlights f WHERE m.id=$1 AND f.id=m.owner_fluctlight_id AND f.created_by_actor_id=$3`, momentID, status, actorID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (a *App) authorizeMoment(ctx context.Context, actorID, momentID string) error {
	var owner string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT f.created_by_actor_id FROM public.moments m JOIN public.fluctlights f ON f.id=m.owner_fluctlight_id WHERE m.id=$1`, momentID).Scan(&owner); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if owner != actorID {
		return ErrUnauthorized
	}
	return nil
}

func (a *App) hydrateMoment(ctx context.Context, actorID string, moment map[string]any) error {
	momentID := stringValue(moment["id"])
	commentsRows, err := a.DB.Pool().Query(ctx, `SELECT id,author_actor_id,text,created_at FROM public.moment_comments WHERE moment_id=$1 ORDER BY created_at,id`, momentID)
	if err != nil {
		return err
	}
	comments := make([]map[string]any, 0)
	for commentsRows.Next() {
		var id, author, text string
		var created time.Time
		if err := commentsRows.Scan(&id, &author, &text, &created); err != nil {
			commentsRows.Close()
			return err
		}
		comments = append(comments, map[string]any{"id": id, "moment_id": momentID, "author_actor_id": author, "text": text, "created_at": created.Format(time.RFC3339Nano)})
	}
	commentsRows.Close()
	if err := commentsRows.Err(); err != nil {
		return err
	}
	var reactionCount int
	var viewerReaction *string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM public.moment_reactions WHERE moment_id=$1`, momentID).Scan(&reactionCount); err != nil {
		return err
	}
	if err := a.DB.Pool().QueryRow(ctx, `SELECT kind FROM public.moment_reactions WHERE moment_id=$1 AND actor_id=$2`, momentID, actorID).Scan(&viewerReaction); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	moment["comments"] = comments
	moment["reaction_count"] = reactionCount
	moment["viewer_reaction"] = viewerReaction
	media := make([]map[string]any, 0)
	for _, raw := range arrayValue(moment["media_asset_ids"]) {
		assetID := stringValue(raw)
		if assetID == "" {
			continue
		}
		var kind, mime, version, status string
		var size int
		if err := a.DB.Pool().QueryRow(ctx, `SELECT kind,mime_type,version,status,byte_size FROM public.media_assets WHERE id=$1 AND owner_fluctlight_id=(SELECT owner_fluctlight_id FROM public.moments WHERE id=$2) AND status='ready'`, assetID, momentID).Scan(&kind, &mime, &version, &status, &size); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return err
		}
		media = append(media, map[string]any{"id": assetID, "kind": kind, "mime_type": mime, "version": version, "status": status, "byte_size": size})
	}
	moment["media"] = media
	return nil
}

func (a *App) Diagnostics(ctx context.Context, actorID string, limit int) ([]map[string]any, error) {
	return a.DiagnosticsFiltered(ctx, actorID, limit, "", "")
}

func (a *App) DiagnosticsFiltered(ctx context.Context, actorID string, limit int, correlationID, fluctlightID string) ([]map[string]any, error) {
	if err := a.requireOwner(ctx, actorID); err != nil {
		return nil, err
	}
	if limit < 1 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	query := `SELECT id,event_type,severity,fluctlight_id,causation_id,correlation_id,payload,created_at FROM public.diagnostic_events WHERE 1=1`
	args := make([]any, 0, 3)
	if strings.TrimSpace(correlationID) != "" {
		args = append(args, strings.TrimSpace(correlationID))
		query += fmt.Sprintf(" AND correlation_id=$%d", len(args))
	}
	if strings.TrimSpace(fluctlightID) != "" {
		args = append(args, strings.TrimSpace(fluctlightID))
		query += fmt.Sprintf(" AND fluctlight_id=$%d", len(args))
	}
	args = append(args, limit)
	query += fmt.Sprintf(" ORDER BY created_at DESC,id DESC LIMIT $%d", len(args))
	rows, err := a.DB.Pool().Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id, typ, severity, corr string
		var fluctlight, causation *string
		var payload []byte
		var created time.Time
		if err := rows.Scan(&id, &typ, &severity, &fluctlight, &causation, &corr, &payload, &created); err != nil {
			return nil, err
		}
		var decodedPayload any
		if json.Unmarshal(payload, &decodedPayload) != nil {
			decodedPayload = map[string]any{}
		}
		out = append(out, map[string]any{"id": id, "event_type": typ, "severity": severity, "fluctlight_id": fluctlight, "causation_id": causation, "correlation_id": corr, "payload": redactDiagnostic(decodedPayload), "created_at": created.Format(time.RFC3339Nano)})
	}
	return out, rows.Err()
}
func (a *App) ClearDiagnostics(ctx context.Context, actorID string) error {
	_, err := a.ClearDiagnosticsCount(ctx, actorID)
	return err
}

func (a *App) ClearDiagnosticsCount(ctx context.Context, actorID string) (int64, error) {
	if err := a.requireOwner(ctx, actorID); err != nil {
		return 0, err
	}
	var cleared int64
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		for _, table := range []string{"diagnostic_events", "diagnostic_model_runs", "diagnostic_turns", "diagnostic_workflow_links"} {
			command, err := tx.Exec(ctx, "DELETE FROM public."+table)
			if err != nil {
				return err
			}
			cleared += command.RowsAffected()
		}
		return nil
	})
	return cleared, err
}
func (a *App) ModelRuns(ctx context.Context, actorID string, limit int) ([]map[string]any, error) {
	return a.ModelRunsFiltered(ctx, actorID, limit, "")
}

func (a *App) ModelRunsFiltered(ctx context.Context, actorID string, limit int, correlationID string) ([]map[string]any, error) {
	if err := a.requireOwner(ctx, actorID); err != nil {
		return nil, err
	}
	if limit < 1 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	query := `WITH runs AS (
		SELECT id,role,binding_role,scenario,priority,endpoint_id,model_id,prompt,response,status,error_code,correlation_id,created_at,queued_at,started_at,completed_at,
			COUNT(*) FILTER (WHERE status IN ('queued','running')) OVER (PARTITION BY binding_role) AS queue_pending_count,
			CASE WHEN status IN ('queued','running') THEN ROW_NUMBER() OVER (
				PARTITION BY binding_role
				ORDER BY CASE WHEN status IN ('queued','running') THEN 0 ELSE 1 END, priority DESC, queued_at ASC, id ASC
			) END AS queue_position
		FROM public.diagnostic_model_runs`
	args := []any{}
	if strings.TrimSpace(correlationID) != "" {
		args = append(args, strings.TrimSpace(correlationID))
		query += ` WHERE correlation_id=$1`
	}
	args = append(args, limit)
	query += fmt.Sprintf(`
	)
	SELECT id,role,binding_role,scenario,priority,endpoint_id,model_id,prompt,response,status,error_code,correlation_id,created_at,queued_at,started_at,completed_at,queue_pending_count,queue_position
	FROM runs
	ORDER BY CASE WHEN status IN ('queued','running') THEN 0 ELSE 1 END,
		CASE WHEN status IN ('queued','running') THEN priority END DESC NULLS LAST,
		CASE WHEN status IN ('queued','running') THEN queued_at END ASC NULLS LAST,
		CASE WHEN status NOT IN ('queued','running') THEN completed_at END DESC NULLS LAST,
		created_at DESC,id DESC
	LIMIT $%d`, len(args))
	rows, err := a.DB.Pool().Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id, role, bindingRole, scenario, model, status, corr string
		var priority int
		var queuePendingCount, queuePosition *int64
		var endpoint, code *string
		var prompt, response []byte
		var created, queued time.Time
		var started, completed *time.Time
		if err := rows.Scan(&id, &role, &bindingRole, &scenario, &priority, &endpoint, &model, &prompt, &response, &status, &code, &corr, &created, &queued, &started, &completed, &queuePendingCount, &queuePosition); err != nil {
			return nil, err
		}
		row := map[string]any{"id": id, "role": role, "binding_role": bindingRole, "scenario": scenario, "priority": priority, "endpoint_id": endpoint, "model_id": model, "prompt": json.RawMessage(prompt), "response": json.RawMessage(response), "status": status, "error_code": code, "correlation_id": corr, "created_at": created.Format(time.RFC3339Nano), "queued_at": queued.Format(time.RFC3339Nano)}
		if queuePendingCount != nil {
			row["queue_pending_count"] = *queuePendingCount
		}
		if queuePosition != nil {
			row["queue_position"] = *queuePosition
		}
		if started != nil {
			row["started_at"] = started.Format(time.RFC3339Nano)
		}
		if completed != nil {
			row["completed_at"] = completed.Format(time.RFC3339Nano)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// MediaPromptsFiltered returns the media-generation prompt projection used by
// the diagnostics center. Media intent creation time is the stable initiation
// order; provider/model runs and ComfyUI submission events are optional
// enrichments of the same durable media intent rather than separate rows in
// the general model-run list.
func (a *App) MediaPromptsFiltered(ctx context.Context, actorID string, limit int) ([]map[string]any, error) {
	if err := a.requireOwner(ctx, actorID); err != nil {
		return nil, err
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 20 {
		limit = 20
	}
	rows, err := a.DB.Pool().Query(ctx, `
		SELECT mi.id,mi.owner_fluctlight_id,mi.kind,mi.mime_type,mi.prompt,mi.provider_prompt,
			mi.provider_request_id,COALESCE(mi.provider_job_id,''),mi.workflow_id,mi.status,COALESCE(mi.quality_verdict,''),COALESCE(mi.quality_retry_guidance,''),mi.created_at,
			COALESCE(event.id,''),COALESCE(event.submitted_prompt,''),COALESCE(event.request_payload,'{}'::jsonb),COALESCE(to_char(event.submitted_at,'YYYY-MM-DD"T"HH24:MI:SS.MS"Z"'),''),
			COALESCE(run.id,''),COALESCE(run.role,''),COALESCE(run.binding_role,''),COALESCE(run.scenario,''),COALESCE(run.priority,0),
			COALESCE(run.model_id,''),COALESCE(run.prompt,'null'::jsonb),COALESCE(run.response,'null'::jsonb),COALESCE(run.status,''),
			COALESCE(run.error_code,''),COALESCE(to_char(run.queued_at,'YYYY-MM-DD"T"HH24:MI:SS.MS"Z"'),to_char(mi.created_at,'YYYY-MM-DD"T"HH24:MI:SS.MS"Z"')),COALESCE(to_char(run.started_at,'YYYY-MM-DD"T"HH24:MI:SS.MS"Z"'),''),
			COALESCE(to_char(run.completed_at,'YYYY-MM-DD"T"HH24:MI:SS.MS"Z"'),''),LEFT(COALESCE(wi.last_error,''),1024),COALESCE(wi.status,''),COALESCE(wi.attempt_count,0),
			CASE WHEN COALESCE(wi.last_error,'')='' THEN '' WHEN mi.provider_job_id IS NOT NULL AND mi.provider_job_id<>'' THEN 'poll_quality' WHEN event.id IS NOT NULL AND event.id<>'' THEN 'submit' ELSE 'prepare' END
		FROM public.media_intents mi
		LEFT JOIN LATERAL (
			SELECT e.id,e.payload->>'prompt' AS submitted_prompt,e.payload->'request_payload' AS request_payload,e.created_at AS submitted_at
			FROM public.diagnostic_events e
			WHERE e.event_type='media.comfyui.prompt_submitted' AND e.correlation_id='media:'||mi.id
			ORDER BY e.created_at DESC,e.id DESC
			LIMIT 1
		) event ON true
		LEFT JOIN public.platform_workflow_intents wi ON wi.workflow_id=mi.workflow_id AND wi.intent_type='media.generation'
		LEFT JOIN LATERAL (
			SELECT r.id,r.role,r.binding_role,r.scenario,r.priority,r.model_id,r.prompt,r.response,r.status,r.error_code,r.queued_at,r.started_at,r.completed_at
			FROM public.diagnostic_model_runs r
			WHERE r.correlation_id='media:'||mi.id AND r.scenario='media_prompt'
			ORDER BY r.queued_at DESC,r.id DESC
			LIMIT 1
		) run ON true
		ORDER BY mi.created_at DESC,mi.id DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id, owner, kind, mime, prompt, providerPrompt, requestID, jobID, workflow, status, qualityVerdict, qualityGuidance, eventID, submittedPrompt, submittedAt, workflowError, workflowStatus, failureStage string
		var runID, runRole, runBinding, runScenario, runModel, runStatus, runError, runQueued, runStarted, runCompleted string
		var runPriority, workflowAttempts int
		var runPrompt, runResponse, submittedPayload []byte
		var created time.Time
		if err := rows.Scan(&id, &owner, &kind, &mime, &prompt, &providerPrompt, &requestID, &jobID, &workflow, &status, &qualityVerdict, &qualityGuidance, &created, &eventID, &submittedPrompt, &submittedPayload, &submittedAt, &runID, &runRole, &runBinding, &runScenario, &runPriority, &runModel, &runPrompt, &runResponse, &runStatus, &runError, &runQueued, &runStarted, &runCompleted, &workflowError, &workflowStatus, &workflowAttempts, &failureStage); err != nil {
			return nil, err
		}
		promptValue := any(prompt)
		if json.Valid([]byte(prompt)) {
			promptValue = json.RawMessage(prompt)
		}
		row := map[string]any{
			"id": id, "media_intent_id": id, "fluctlight_id": owner, "kind": kind, "mime_type": mime,
			"prompt": promptValue, "provider_prompt": providerPrompt, "submitted_prompt": submittedPrompt,
			"provider_request_id": requestID, "provider_job_id": jobID, "workflow_id": workflow, "status": status, "quality_verdict": qualityVerdict,
			"correlation_id": "media:" + id, "created_at": created.Format(time.RFC3339Nano),
		}
		if eventID != "" {
			row["submitted_event_id"] = eventID
			if json.Valid(submittedPayload) && string(submittedPayload) != "{}" {
				row["request_payload"] = json.RawMessage(submittedPayload)
			}
			row["submitted_at"] = submittedAt
		}
		if workflowError == "" && status == "failed" {
			if qualityVerdict == mediaQualityVerdictReject {
				workflowError = "媒体质量检查未通过"
				if qualityGuidance != "" {
					workflowError += ": " + qualityGuidance
				}
			} else {
				workflowError = "媒体生成失败，但没有记录更详细的错误信息"
			}
			if failureStage == "" {
				failureStage = "poll_quality"
			}
		}
		if workflowError != "" {
			row["error_message"] = workflowError
			row["failure_stage"] = failureStage
			row["workflow_status"] = workflowStatus
			row["attempt_count"] = workflowAttempts
		}
		if runID != "" {
			modelRun := map[string]any{
				"id": runID, "role": runRole, "binding_role": runBinding, "scenario": runScenario,
				"priority": runPriority, "model_id": runModel, "prompt": json.RawMessage(runPrompt),
				"response": json.RawMessage(runResponse), "status": runStatus, "error_code": runError,
				"queued_at": runQueued, "correlation_id": "media:" + id,
			}
			if runStarted != "" {
				modelRun["started_at"] = runStarted
			}
			if runCompleted != "" {
				modelRun["completed_at"] = runCompleted
			}
			row["model_run"] = modelRun
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// RetryMediaIntent resets one failed media target while preserving its frozen
// provider prompt. The existing workflow restart path is reused so a retry
// gets a new ComfyUI submission and fresh diagnostic event without creating a
// second media intent.
func (a *App) RetryMediaIntent(ctx context.Context, actorID, intentID string) (map[string]any, error) {
	if err := a.requireOwner(ctx, actorID); err != nil {
		return nil, err
	}
	var workflowID, mediaStatus, workflowStatus, ownerFluctlightID, providerRequestID string
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT workflow_id,status,owner_fluctlight_id,provider_request_id FROM public.media_intents WHERE id=$1 FOR UPDATE`, intentID).Scan(&workflowID, &mediaStatus, &ownerFluctlightID, &providerRequestID); errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		// Lock the workflow row separately. PostgreSQL cannot apply FOR UPDATE
		// to the nullable side of the outer join that is needed to repair legacy
		// media rows without a workflow intent.
		if err := tx.QueryRow(ctx, `SELECT status FROM public.platform_workflow_intents WHERE workflow_id=$1 AND intent_type='media.generation' FOR UPDATE`, workflowID).Scan(&workflowStatus); errors.Is(err, pgx.ErrNoRows) {
			workflowStatus = ""
		} else if err != nil {
			return err
		}
		if mediaStatus != "failed" && workflowStatus != "failed" {
			return ErrConflict
		}
		if workflowStatus == "started" || workflowStatus == "paused" || workflowStatus == "cancel_requested" {
			return ErrConflict
		}
		if _, err := tx.Exec(ctx, `UPDATE public.media_intents SET provider_job_id=NULL,status='pending',quality_verdict='',quality_candidate_sha256=NULL,quality_checked_at=NULL,revision=revision+1 WHERE id=$1`, intentID); err != nil {
			return err
		}
		if workflowStatus == "" {
			// Older media rows may predate the durable workflow-intent record.
			// Recreate that record from the media intent before restarting so the
			// retry does not get stuck at an irreversible not-found error.
			if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents (intent_id,workflow_id,task_queue,intent_type,payload) VALUES ($1,$2,'media','media.generation',jsonb_build_object('intent_id',$3,'provider_request_id',$4,'fluctlight_id',$5)) ON CONFLICT (workflow_id) DO NOTHING`, "media_workflow_intent:"+intentID, workflowID, intentID, providerRequestID, ownerFluctlightID); err != nil {
				return err
			}
		}
		command, err := tx.Exec(ctx, `UPDATE public.platform_workflow_intents SET status='pending',attempt_count=0,last_error=NULL,next_attempt_at=now(),started_at=NULL,completed_at=NULL WHERE workflow_id=$1 AND intent_type='media.generation'`, workflowID)
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return ErrNotFound
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if a.Workflows == nil {
		return map[string]any{"media_intent_id": intentID, "workflow_id": workflowID, "status": "retry_queued"}, nil
	}
	if _, err := a.WorkflowCommand(ctx, actorID, workflowID, "restart", nil); err != nil {
		bounded := visualIdentityBoundedText(strings.Join(strings.Fields(err.Error()), " "), 1024)
		if mediaRetryRestartIsPermanent(err) {
			if persistErr := a.persistPermanentMediaRetryFailure(ctx, intentID, workflowID, bounded); persistErr != nil {
				return nil, fmt.Errorf("%w; retry state persistence failed: %v", err, persistErr)
			}
			return nil, err
		}
		if _, persistErr := a.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='retry',last_error=$2,next_attempt_at=now()+interval '5 seconds' WHERE workflow_id=$1 AND intent_type='media.generation'`, workflowID, bounded); persistErr != nil {
			return nil, fmt.Errorf("%w; retry state persistence failed: %v", err, persistErr)
		}
		// The durable row is still queued for the Worker. A transient Temporal
		// restart failure should not make the browser retry button unusable; the
		// next dispatcher pass will retry using the same stable workflow ID.
		return map[string]any{"media_intent_id": intentID, "workflow_id": workflowID, "status": "retry_queued"}, nil
	}
	return map[string]any{"media_intent_id": intentID, "workflow_id": workflowID, "status": "retry_queued"}, nil
}

func (a *App) persistPermanentMediaRetryFailure(ctx context.Context, intentID, workflowID, message string) error {
	tx, err := a.DB.Pool().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE public.media_intents SET status='failed',revision=revision+1 WHERE id=$1 AND status='pending'`, intentID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE public.platform_workflow_intents SET status='failed',last_error=$2,completed_at=now() WHERE workflow_id=$1 AND intent_type='media.generation'`, workflowID, message); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func mediaRetryRestartIsPermanent(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"workflow payload is invalid",
		"unsupported workflow intent type",
		"workflow restart identity is incomplete",
		"media_intent_not_found",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func (a *App) DiagnosticsExport(ctx context.Context, actorID string) (map[string]any, error) {
	if err := a.requireOwner(ctx, actorID); err != nil {
		return nil, err
	}
	events, err := a.Diagnostics(ctx, actorID, 200)
	if err != nil {
		return nil, err
	}
	runs, err := a.ModelRuns(ctx, actorID, 200)
	if err != nil {
		return nil, err
	}
	mediaPrompts, err := a.MediaPromptsFiltered(ctx, actorID, 20)
	if err != nil {
		return nil, err
	}
	return map[string]any{"events": events, "model_runs": runs, "media_prompts": mediaPrompts}, nil
}

// RecoverStaleModelRuns closes lifecycle records left behind by a crashed API
// or Worker process. The in-memory queue cannot resume a lost callback, so a
// stale request is terminally marked with a bounded infrastructure reason;
// its owning domain workflow remains responsible for retrying the operation.
func (a *App) RecoverStaleModelRuns(ctx context.Context, olderThan time.Duration) (int64, error) {
	if olderThan <= 0 {
		olderThan = 15 * time.Minute
	}
	command, err := a.DB.Pool().Exec(ctx, `UPDATE public.diagnostic_model_runs SET status='failed',error_code='provider_process_restarted',completed_at=COALESCE(completed_at,now()) WHERE status IN ('queued','running') AND COALESCE(started_at,queued_at,created_at) < now()-$1::interval`, fmt.Sprintf("%d seconds", int64(olderThan/time.Second)))
	if err != nil {
		return 0, err
	}
	return command.RowsAffected(), nil
}

// PruneDiagnostics enforces the local retention contract without touching any
// domain audit/revision/evidence tables. It is safe to call from a periodic
// Worker tick; each DELETE is bounded by the configured row cap.
func (a *App) PruneDiagnostics(ctx context.Context, olderThan time.Duration, maxRows int) (int64, error) {
	if olderThan <= 0 {
		olderThan = 30 * 24 * time.Hour
	}
	if maxRows < 1 {
		maxRows = 10000
	}
	var removed int64
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		for _, table := range []string{"diagnostic_events", "diagnostic_model_runs", "diagnostic_turns", "diagnostic_workflow_links"} {
			command, err := tx.Exec(ctx, `DELETE FROM public.`+table+` WHERE created_at < now()-$1::interval OR id IN (SELECT id FROM public.`+table+` ORDER BY created_at DESC OFFSET $2)`, fmt.Sprintf("%d seconds", int64(olderThan/time.Second)), maxRows)
			if err != nil {
				return err
			}
			removed += command.RowsAffected()
		}
		return nil
	})
	return removed, err
}

var ErrWorkflowRuntime = errors.New("workflow_runtime_unavailable")

func (a *App) WorkflowList(ctx context.Context, actorID, query string) ([]map[string]any, error) {
	if err := a.requireOwner(ctx, actorID); err != nil {
		slog.Warn("workflow list owner authorization failed", "actor_id", actorID)
		_ = a.auditWorkflow(ctx, actorID, "list", "", false, map[string]any{"query": query})
		return nil, err
	}
	if a.Workflows == nil {
		return nil, ErrWorkflowRuntime
	}
	executions, err := a.Workflows.List(ctx, query, 200)
	if err != nil {
		_ = a.auditWorkflow(ctx, actorID, "list", "", true, map[string]any{"query": query, "error": "runtime_unavailable"})
		return nil, fmt.Errorf("%w: %v", ErrWorkflowRuntime, err)
	}
	if err := a.auditWorkflow(ctx, actorID, "list", "", true, map[string]any{"query": query, "count": len(executions)}); err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(executions))
	for _, execution := range executions {
		out = append(out, workflowExecutionMap(execution))
	}
	return out, nil
}

func (a *App) WorkflowStatus(ctx context.Context, actorID, wf string) (map[string]any, error) {
	if err := a.requireOwner(ctx, actorID); err != nil {
		_ = a.auditWorkflow(ctx, actorID, "status", wf, false, nil)
		return nil, err
	}
	intent, queue, intentType, payload, err := a.workflowIntent(ctx, wf)
	if err != nil {
		slog.Warn("workflow status intent lookup failed", "workflow_id", wf, "error", err)
		return nil, err
	}
	if a.Workflows == nil {
		return nil, ErrWorkflowRuntime
	}
	execution, err := a.Workflows.Status(ctx, wf, "")
	if err != nil {
		slog.Warn("workflow status runtime lookup failed", "workflow_id", wf, "error", err)
		_ = a.auditWorkflow(ctx, actorID, "status", wf, true, map[string]any{"error": "runtime_unavailable"})
		return nil, fmt.Errorf("%w: %v", ErrWorkflowRuntime, err)
	}
	if err := a.auditWorkflow(ctx, actorID, "status", wf, true, map[string]any{"status": execution.Status}); err != nil {
		return nil, err
	}
	value := workflowExecutionMap(execution)
	value["intent_id"] = intent
	value["task_queue"] = queue
	value["intent_type"] = intentType
	value["payload"] = json.RawMessage(payload)
	return value, nil
}

func (a *App) WorkflowHistory(ctx context.Context, actorID, wf string) (map[string]any, error) {
	if err := a.requireOwner(ctx, actorID); err != nil {
		_ = a.auditWorkflow(ctx, actorID, "history", wf, false, nil)
		return nil, err
	}
	if _, _, _, _, err := a.workflowIntent(ctx, wf); err != nil {
		return nil, err
	}
	if a.Workflows == nil {
		return nil, ErrWorkflowRuntime
	}
	history, err := a.Workflows.History(ctx, wf, "", 50)
	if err != nil {
		_ = a.auditWorkflow(ctx, actorID, "history", wf, true, map[string]any{"error": "runtime_unavailable"})
		return nil, fmt.Errorf("%w: %v", ErrWorkflowRuntime, err)
	}
	if err := a.auditWorkflow(ctx, actorID, "history", wf, true, map[string]any{"event_count": history.EventCount}); err != nil {
		return nil, err
	}
	return map[string]any{"workflow_id": history.WorkflowID, "run_id": history.RunID, "event_count": history.EventCount, "event_types": history.EventTypes}, nil
}

func (a *App) WorkflowCommand(ctx context.Context, actorID, wf, action string, payload map[string]any) (map[string]any, error) {
	if err := a.requireOwner(ctx, actorID); err != nil {
		_ = a.auditWorkflow(ctx, actorID, action, wf, false, map[string]any{"error": "forbidden"})
		return nil, err
	}
	intentID, queue, intentType, intentPayload, err := a.workflowIntent(ctx, wf)
	if err != nil {
		return nil, err
	}
	if a.Workflows == nil {
		return nil, ErrWorkflowRuntime
	}
	requestID := stringValue(payload["request_id"])
	if requestID == "" {
		requestID = stableDigest(fmt.Sprintf("%s:%s:%s:%s", actorID, action, wf, intentID))
	}
	var result map[string]any
	switch action {
	case "pause", "resume":
		if err := a.Workflows.Signal(ctx, wf, "", action, requestID); err != nil {
			return nil, a.workflowCommandFailure(ctx, actorID, action, wf, err)
		}
		intentStatus := "paused"
		if action == "resume" {
			intentStatus = "started"
		}
		if err := a.updateWorkflowIntentStatus(ctx, wf, intentStatus); err != nil {
			return nil, a.workflowCommandFailure(ctx, actorID, action, wf, err)
		}
		result = map[string]any{"workflow_id": wf, "operation": action, "status": "accepted"}
	case "cancel":
		if err := a.Workflows.Cancel(ctx, wf, "", requestID); err != nil {
			return nil, a.workflowCommandFailure(ctx, actorID, action, wf, err)
		}
		if err := a.updateWorkflowIntentStatus(ctx, wf, "cancel_requested"); err != nil {
			return nil, a.workflowCommandFailure(ctx, actorID, action, wf, err)
		}
		result = map[string]any{"workflow_id": wf, "operation": action, "status": "accepted"}
	case "reset":
		historyPoint, ok := positiveInt64(payload["history_point"])
		if !ok {
			return nil, a.workflowCommandFailure(ctx, actorID, action, wf, errors.New("history_point must be greater than zero"))
		}
		execution, err := a.Workflows.Reset(ctx, wf, "", historyPoint, "owner requested workflow reset", requestID)
		if err != nil {
			return nil, a.workflowCommandFailure(ctx, actorID, action, wf, err)
		}
		if err := a.updateWorkflowIntentStatus(ctx, wf, "started"); err != nil {
			return nil, a.workflowCommandFailure(ctx, actorID, action, wf, err)
		}
		result = workflowExecutionMap(execution)
		result["operation"] = action
	case "restart":
		execution, err := a.Workflows.Restart(ctx, WorkflowStart{WorkflowID: wf, TaskQueue: queue, IntentType: intentType, Payload: intentPayload})
		if err != nil {
			return nil, a.workflowCommandFailure(ctx, actorID, action, wf, err)
		}
		if err := a.updateWorkflowIntentStatus(ctx, wf, "started"); err != nil {
			return nil, a.workflowCommandFailure(ctx, actorID, action, wf, err)
		}
		result = workflowExecutionMap(execution)
		result["operation"] = action
	default:
		return nil, a.workflowCommandFailure(ctx, actorID, action, wf, errors.New("unsupported workflow command"))
	}
	if err := a.auditWorkflow(ctx, actorID, action, wf, true, map[string]any{"status": result["status"], "request_id": requestID}); err != nil {
		return nil, err
	}
	return result, nil
}

func (a *App) workflowCommandFailure(ctx context.Context, actorID, action, workflowID string, err error) error {
	_ = a.auditWorkflow(ctx, actorID, action, workflowID, true, map[string]any{"error": "command_failed"})
	return fmt.Errorf("workflow %s failed: %w", action, err)
}

func (a *App) requireOwner(ctx context.Context, actorID string) error {
	var owner string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT human_actor_id FROM public.owner_accounts LIMIT 1`).Scan(&owner); err != nil {
		return ErrUnauthorized
	}
	if owner == "" || owner != actorID {
		return ErrUnauthorized
	}
	return nil
}

func (a *App) workflowIntent(ctx context.Context, workflowID string) (string, string, string, []byte, error) {
	var intentID, queue, intentType string
	var payload []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT intent_id,task_queue,intent_type,payload FROM public.platform_workflow_intents WHERE workflow_id=$1 OR ('go:' || workflow_id)=$1 ORDER BY CASE WHEN workflow_id=$1 THEN 0 ELSE 1 END LIMIT 1`, workflowID).Scan(&intentID, &queue, &intentType, &payload); errors.Is(err, pgx.ErrNoRows) {
		return "", "", "", nil, ErrNotFound
	} else if err != nil {
		return "", "", "", nil, err
	}
	return intentID, queue, intentType, payload, nil
}

func (a *App) updateWorkflowIntentStatus(ctx context.Context, workflowID, status string) error {
	_, err := a.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status=$2,last_error=NULL WHERE workflow_id=$1 OR ('go:' || workflow_id)=$1`, workflowID, status)
	return err
}

func (a *App) auditWorkflow(ctx context.Context, actorID, action, workflowID string, authorized bool, details map[string]any) error {
	if details == nil {
		details = map[string]any{}
	}
	details["authorized"] = authorized
	encoded, err := json.Marshal(details)
	if err != nil {
		return err
	}
	auditID := "workflow_audit_" + stableDigest(strings.Join([]string{actorID, action, workflowID, string(encoded)}, ":"))
	_, err = a.DB.Pool().Exec(ctx, `INSERT INTO public.platform_workflow_management_audit(id,action,workflow_id,actor_id,authorized,details) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(id) DO UPDATE SET details=EXCLUDED.details,created_at=now()`, auditID, action, workflowID, actorID, strconv.FormatBool(authorized), encoded)
	return err
}

func workflowExecutionMap(execution WorkflowExecution) map[string]any {
	value := map[string]any{"workflow_id": execution.WorkflowID, "run_id": execution.RunID, "workflow_type": execution.WorkflowType, "task_queue": execution.TaskQueue, "status": execution.Status, "history_length": execution.HistoryLength}
	if execution.StartTime != nil {
		value["start_time"] = execution.StartTime.Format(time.RFC3339Nano)
	}
	if execution.CloseTime != nil {
		value["close_time"] = execution.CloseTime.Format(time.RFC3339Nano)
	}
	return value
}

func positiveInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), v > 0
	case int64:
		return v, v > 0
	case float64:
		return int64(v), v > 0 && v == float64(int64(v))
	case json.Number:
		i, err := v.Int64()
		return i, err == nil && i > 0
	case string:
		i, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return i, err == nil && i > 0
	default:
		return 0, false
	}
}

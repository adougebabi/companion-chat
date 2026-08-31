package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (a *App) RevokeAll(ctx context.Context, actorID string) error {
	_, err := a.DB.Pool().Exec(ctx, `UPDATE public.auth_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE human_actor_id=$1`, actorID)
	return err
}

func (a *App) RevokeCurrent(ctx context.Context, actorID, token string) error {
	if strings.TrimSpace(token) == "" {
		return ErrUnauthorized
	}
	command, err := a.DB.Pool().Exec(ctx, `UPDATE public.auth_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE human_actor_id=$1 AND token_hash=$2`, actorID, digestToken(token))
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrUnauthorized
	}
	return nil
}

func (a *App) ResetPassword(ctx context.Context, actorID, password string) error {
	if len(password) < 6 {
		return errors.New("password_invalid")
	}
	hash, err := hashArgon2ID(password)
	if err != nil {
		return err
	}
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE public.owner_accounts SET credential_hash=$2,credential_revision=$3,updated_at=now() WHERE human_actor_id=$1`, actorID, hash, randomID("credential_")); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE public.auth_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE human_actor_id=$1`, actorID)
		return err
	})
}

func (a *App) CreateConversation(ctx context.Context, ownerID string, participants []any, title string) (map[string]any, error) {
	conversationID := randomID("conversation_")
	ids := make([]string, 0, len(participants)+1)
	ids = append(ids, ownerID)
	for _, raw := range participants {
		if id := stringValue(raw); id != "" && id != ownerID {
			ids = append(ids, id)
		}
	}
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
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
	if status == "" {
		return nil, errors.New("status_invalid")
	}
	args := []any{id, actorID, status}
	query := `UPDATE public.fluctlights SET status=$3,updated_at=now(),retired_at=CASE WHEN $3='retired' THEN now() ELSE retired_at END WHERE id=$1 AND created_by_actor_id=$2`
	if expected != nil {
		query += ` AND current_revision=$4`
		args = append(args, *expected)
	}
	query += ` RETURNING current_revision,status`
	var revision int
	var actual string
	if err := a.DB.Pool().QueryRow(ctx, query, args...).Scan(&revision, &actual); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return map[string]any{"id": id, "status": actual, "current_revision": revision, "reason": reason}, nil
}

func (a *App) RetireFluctlight(ctx context.Context, actorID, id, reason string, expected *int) (map[string]any, error) {
	return a.SetFluctlightStatus(ctx, actorID, id, "retired", reason, expected)
}

func (a *App) ProposeFoundation(ctx context.Context, actorID, fluctlightID string, payload map[string]any) (map[string]any, error) {
	f, err := a.DB.GetFluctlight(ctx, fluctlightID, actorID)
	if err != nil {
		return nil, err
	}
	var current int
	_ = a.DB.Pool().QueryRow(ctx, `SELECT current_revision FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&current)
	id := randomID("foundation_revision_")
	now := time.Now().UTC()
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_foundation_revisions(id,fluctlight_id,revision,base_revision,source,status,actor_id,initialization_mode,foundation_status,foundation_created_at,confidence,changes,identity,personality,behavioral_policy,life_profile,provenance,evidence_refs,reason,idempotency_key) VALUES($1,$2,$3,$4,'owner','proposed',$5,$6,'pending',$7,'{}',$8,$9,$10,$11,$12,$13,$14,$15,$16)`, id, fluctlightID, current+1, current, actorID, "llm_defined", now, jsonBytes(payload["changes"]), jsonBytes(f.Identity), jsonBytes(f.Personality), jsonBytes(f.BehavioralPolicy), jsonBytes(f.LifeProfile), jsonBytes(f.Provenance), jsonBytes(arrayValue(payload["evidence_refs"])), stringValue(payload["reason"]), "foundation:"+id)
		return err
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "fluctlight_id": fluctlightID, "revision": current + 1, "status": "proposed"}, nil
}

func (a *App) SetFoundationDecision(ctx context.Context, actorID, fluctlightID, revisionID, action, reason string) (map[string]any, error) {
	var revision int
	var identity, personality, policy, life []byte
	err := a.DB.Pool().QueryRow(ctx, `SELECT revision,identity,personality,behavioral_policy,life_profile FROM public.fluctlight_foundation_revisions WHERE id=$1 AND fluctlight_id=$2 AND actor_id=$3`, revisionID, fluctlightID, actorID).Scan(&revision, &identity, &personality, &policy, &life)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if action == "accept" {
		err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_foundation_revisions SET status='accepted',foundation_status='active',accepted_at=now() WHERE id=$1`, revisionID); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `UPDATE public.fluctlights SET current_revision=$2,identity=$3,personality=$4,behavioral_policy=$5,life_profile=$6,updated_at=now() WHERE id=$1 AND created_by_actor_id=$7`, fluctlightID, revision, identity, personality, policy, life, actorID)
			return err
		})
	} else {
		_, err = a.DB.Pool().Exec(ctx, `UPDATE public.fluctlight_foundation_revisions SET status='rejected',foundation_status='rejected',rejected_at=now(),reason=$2 WHERE id=$1`, revisionID, reason)
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": revisionID, "revision": revision, "status": action + "ed", "reason": reason}, nil
}

func (a *App) ReviseMemory(ctx context.Context, actorID, id, content string, expected *int, refs []any) (map[string]any, error) {
	var owner, old string
	var revision, newRevision int
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT owner_fluctlight_id,content,revision FROM public.memories WHERE id=$1 FOR UPDATE`, id).Scan(&owner, &old, &revision); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if _, err := a.DB.GetFluctlight(ctx, owner, actorID); err != nil {
			return ErrUnauthorized
		}
		if expected != nil && *expected != revision {
			return errors.New("memory_revision_stale")
		}
		newRevision = revision + 1
		if _, err := tx.Exec(ctx, `UPDATE public.memories SET content=$2,revision=$3,last_confirmed_at=now(),search_document=to_tsvector('simple',$2) WHERE id=$1`, id, content, newRevision); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.memory_revisions(id,memory_id,revision,base_revision,content,status,actor_id,evidence_refs,idempotency_key) VALUES($1,$2,$3,$4,$5,'active',$6,$7,$8) ON CONFLICT DO NOTHING`, randomID("memory_revision_"), id, newRevision, revision, content, actorID, jsonBytes(refs), "memory:"+id+":"+fmt.Sprint(newRevision)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.memory_embeddings SET status='stale' WHERE memory_id=$1 AND memory_revision<>$2 AND status IN ('ready','completed')`, id, newRevision); err != nil {
			return err
		}
		return appendOutboxTx(ctx, tx, "memory.revised", "memory", id, owner, actorID, "memory:"+id, "memory-outbox:"+id+":"+fmt.Sprint(newRevision), map[string]any{"memory_id": id, "revision": newRevision})
	})
	if err != nil {
		return nil, err
	}
	_ = old
	return map[string]any{"id": id, "content": content, "revision": newRevision, "status": "active"}, nil
}

func (a *App) ForgetMemory(ctx context.Context, actorID, id string, expected *int, refs []any) (map[string]any, error) {
	var owner string
	var revision, newRevision int
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT owner_fluctlight_id,revision FROM public.memories WHERE id=$1 FOR UPDATE`, id).Scan(&owner, &revision); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if _, err := a.DB.GetFluctlight(ctx, owner, actorID); err != nil {
			return ErrUnauthorized
		}
		if expected != nil && *expected != revision {
			return errors.New("memory_revision_stale")
		}
		newRevision = revision + 1
		if _, err := tx.Exec(ctx, `UPDATE public.memories SET status='forgotten',revision=$2 WHERE id=$1`, id, newRevision); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.memory_revisions(id,memory_id,revision,base_revision,content,status,actor_id,evidence_refs,idempotency_key) SELECT $1,id,$2,$3,content,'forgotten',$4,$5,$6 FROM public.memories WHERE id=$7 ON CONFLICT DO NOTHING`, randomID("memory_revision_"), newRevision, revision, actorID, jsonBytes(refs), "memory-forget:"+id+":"+fmt.Sprint(newRevision), id); err != nil {
			return err
		}
		return appendOutboxTx(ctx, tx, "memory.forgotten", "memory", id, owner, actorID, "memory:"+id, "memory-forget-outbox:"+id+":"+fmt.Sprint(newRevision), map[string]any{"memory_id": id, "revision": newRevision})
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
		command, err := tx.Exec(ctx, `UPDATE public.autonomy_actions SET status=$2,error_code=CASE WHEN $2='failed' THEN $3 ELSE error_code END WHERE id=$1 AND status=$4`, actionID, toStatus, reason, from)
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
	if role == "" || endpoint == "" || model == "" {
		return errors.New("provider_role_invalid")
	}
	_, err := a.DB.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,token_budget,timeout_seconds,required_capabilities,retry_policy) VALUES($1,$2,$3,$4,$5,'','{}') ON CONFLICT(role) DO UPDATE SET provider_endpoint_id=excluded.provider_endpoint_id,model_id=excluded.model_id,token_budget=excluded.token_budget,timeout_seconds=excluded.timeout_seconds`, role, endpoint, model, intValue(payload["token_budget"]), intValue(payload["timeout_seconds"]))
	return err
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
		out = append(out, map[string]any{"id": id, "name": name, "owner_actor_id": owner, "members": members, "created_at": created.Format(time.RFC3339Nano)})
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
	return map[string]any{"id": id, "name": name, "owner_actor_id": owner, "members": []string{}}, nil
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
	var id string
	var revision int
	if err := a.DB.Pool().QueryRow(ctx, `SELECT id,revision FROM public.relationships WHERE owner_fluctlight_id=$1 AND target_actor_id=$2`, fluctlightID, target).Scan(&id, &revision); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if _, err := a.DB.GetFluctlight(ctx, fluctlightID, actorID); err != nil {
		return nil, ErrUnauthorized
	}
	_, err := a.DB.Pool().Exec(ctx, `INSERT INTO public.relationship_governance(id,relationship_id,action,actor_id,reason) VALUES($1,$2,'rollback',$3,$4)`, randomID("relationship_governance_"), id, actorID, stringValue(payload["reason"]))
	if err != nil {
		return nil, err
	}
	return map[string]any{"relationship_id": id, "revision": revision, "status": "rolled_back"}, nil
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
	id := randomID("event_")
	_, err := a.DB.Pool().Exec(ctx, `INSERT INTO public.life_events(id,fluctlight_id,kind,start_at,end_at,scene,activity,location,status,evidence_refs) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'confirmed',$9)`, id, fluctlightID, stringValue(payload["kind"]), start, end, nullableString(stringValue(payload["scene"])), nullableString(stringValue(payload["activity"])), nullableString(stringValue(payload["location"])), jsonBytes(arrayValue(payload["evidence_refs"])))
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "fluctlight_id": fluctlightID, "status": "confirmed"}, nil
}
func (a *App) CancelLifeEvent(ctx context.Context, actorID, fluctlightID, eventID string) error {
	if _, err := a.DB.GetFluctlight(ctx, fluctlightID, actorID); err != nil {
		return err
	}
	cmd, err := a.DB.Pool().Exec(ctx, `UPDATE public.life_events SET status='cancelled' WHERE id=$1 AND fluctlight_id=$2`, eventID, fluctlightID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
func (a *App) SetPresence(ctx context.Context, actorID, fluctlightID string, payload map[string]any) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fluctlightID, actorID); err != nil {
		return nil, err
	}
	id := "presence_" + stableDigest(fluctlightID)
	_, err := a.DB.Pool().Exec(ctx, `INSERT INTO public.life_presence(id,fluctlight_id,actor_id,current_task,user_presence) VALUES($1,$2,$3,$4,$5) ON CONFLICT(id) DO UPDATE SET current_task=excluded.current_task,user_presence=excluded.user_presence`, id, fluctlightID, fluctlightID, nullableString(stringValue(payload["current_task"])), nullableString(stringValue(payload["user_presence"])))
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "fluctlight_id": fluctlightID, "current_task": payload["current_task"], "user_presence": payload["user_presence"]}, nil
}
func (a *App) CancelSchedule(ctx context.Context, actorID, fluctlightID, scheduleID string) error {
	if _, err := a.DB.GetFluctlight(ctx, fluctlightID, actorID); err != nil {
		return err
	}
	cmd, err := a.DB.Pool().Exec(ctx, `UPDATE public.life_schedules SET status='cancelled' WHERE id=$1 AND fluctlight_id=$2`, scheduleID, fluctlightID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (a *App) AllMoments(ctx context.Context, actorID string, includeHidden bool, limit int) ([]map[string]any, error) {
	if limit < 1 {
		limit = 100
	}
	query := `SELECT id,owner_fluctlight_id,author_actor_id,text,visibility,status,media_asset_ids,created_at FROM public.moments WHERE 1=1`
	if !includeHidden {
		query += ` AND status<>'hidden'`
	}
	query += fmt.Sprintf(` ORDER BY created_at DESC,id DESC LIMIT %d`, limit)
	rows, err := a.DB.Pool().Query(ctx, query)
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
		out = append(out, map[string]any{"id": id, "owner_fluctlight_id": owner, "author_actor_id": author, "text": text, "visibility": visibility, "status": status, "media_asset_ids": decodeArray(media), "created_at": created.Format(time.RFC3339Nano)})
	}
	return out, rows.Err()
}
func (a *App) MarkMomentsRead(ctx context.Context, actorID, fluctlightID string) error {
	_, err := a.DB.Pool().Exec(ctx, `INSERT INTO public.moment_read_positions(owner_fluctlight_id,actor_id,last_seen_at) VALUES($1,$2,now()) ON CONFLICT(owner_fluctlight_id,actor_id) DO UPDATE SET last_seen_at=now()`, fluctlightID, actorID)
	return err
}
func (a *App) CommentMoment(ctx context.Context, actorID, momentID, text string) (map[string]any, error) {
	id := randomID("comment_")
	_, err := a.DB.Pool().Exec(ctx, `INSERT INTO public.moment_comments(id,moment_id,author_actor_id,text) VALUES($1,$2,$3,$4)`, id, momentID, actorID, text)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "moment_id": momentID, "author_actor_id": actorID, "text": text}, nil
}
func (a *App) ReactMoment(ctx context.Context, actorID, momentID, kind string) (map[string]any, error) {
	_, err := a.DB.Pool().Exec(ctx, `INSERT INTO public.moment_reactions(moment_id,actor_id,kind) VALUES($1,$2,$3) ON CONFLICT(moment_id,actor_id) DO UPDATE SET kind=excluded.kind`, momentID, actorID, kind)
	if err != nil {
		return nil, err
	}
	return map[string]any{"moment_id": momentID, "actor_id": actorID, "kind": kind}, nil
}
func (a *App) SetMomentStatus(ctx context.Context, actorID, momentID, status string) error {
	cmd, err := a.DB.Pool().Exec(ctx, `UPDATE public.moments m SET status=$2 FROM public.fluctlights f WHERE m.id=$1 AND f.id=m.owner_fluctlight_id AND f.created_by_actor_id=$3`, momentID, status, actorID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (a *App) Diagnostics(ctx context.Context, actorID string, limit int) ([]map[string]any, error) {
	rows, err := a.DB.Pool().Query(ctx, fmt.Sprintf(`SELECT id,event_type,severity,fluctlight_id,correlation_id,payload,created_at FROM public.diagnostic_events ORDER BY created_at DESC LIMIT %d`, limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id, typ, severity, corr string
		var fluctlight *string
		var payload []byte
		var created time.Time
		if err := rows.Scan(&id, &typ, &severity, &fluctlight, &corr, &payload, &created); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "event_type": typ, "severity": severity, "fluctlight_id": fluctlight, "correlation_id": corr, "payload": json.RawMessage(payload), "created_at": created.Format(time.RFC3339Nano)})
	}
	return out, rows.Err()
}
func (a *App) ClearDiagnostics(ctx context.Context, actorID string) error {
	_, err := a.DB.Pool().Exec(ctx, `DELETE FROM public.diagnostic_events`)
	return err
}
func (a *App) ModelRuns(ctx context.Context, actorID string, limit int) ([]map[string]any, error) {
	rows, err := a.DB.Pool().Query(ctx, fmt.Sprintf(`SELECT id,role,endpoint_id,model_id,status,error_code,correlation_id,created_at FROM public.diagnostic_model_runs ORDER BY created_at DESC LIMIT %d`, limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id, role, model, status, corr string
		var endpoint, code *string
		var created time.Time
		if err := rows.Scan(&id, &role, &endpoint, &model, &status, &code, &corr, &created); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "role": role, "endpoint_id": endpoint, "model_id": model, "status": status, "error_code": code, "correlation_id": corr, "created_at": created.Format(time.RFC3339Nano)})
	}
	return out, rows.Err()
}
func (a *App) DiagnosticsExport(ctx context.Context, actorID string) (map[string]any, error) {
	events, err := a.Diagnostics(ctx, actorID, 200)
	if err != nil {
		return nil, err
	}
	runs, err := a.ModelRuns(ctx, actorID, 200)
	if err != nil {
		return nil, err
	}
	return map[string]any{"events": events, "model_runs": runs}, nil
}
func (a *App) WorkflowList(ctx context.Context, actorID string) ([]map[string]any, error) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT intent_id,workflow_id,task_queue,intent_type,created_at FROM public.platform_workflow_intents ORDER BY created_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var intent, wf, q, t string
		var created time.Time
		if err := rows.Scan(&intent, &wf, &q, &t, &created); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"intent_id": intent, "workflow_id": wf, "task_queue": q, "intent_type": t, "created_at": created.Format(time.RFC3339Nano)})
	}
	return out, rows.Err()
}
func (a *App) WorkflowStatus(ctx context.Context, actorID, wf string) (map[string]any, error) {
	var intent, q, t string
	var payload []byte
	err := a.DB.Pool().QueryRow(ctx, `SELECT intent_id,task_queue,intent_type,payload FROM public.platform_workflow_intents WHERE workflow_id=$1`, wf).Scan(&intent, &q, &t, &payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"workflow_id": wf, "intent_id": intent, "task_queue": q, "intent_type": t, "status": "pending", "payload": json.RawMessage(payload)}, nil
}
func (a *App) WorkflowHistory(ctx context.Context, actorID, wf string) (map[string]any, error) {
	status, err := a.WorkflowStatus(ctx, actorID, wf)
	if err != nil {
		return nil, err
	}
	return map[string]any{"workflow_id": wf, "events": []any{status}}, nil
}
func (a *App) WorkflowCommand(ctx context.Context, actorID, wf, method, path string, payload map[string]any) (map[string]any, error) {
	status, err := a.WorkflowStatus(ctx, actorID, wf)
	if err != nil {
		return nil, err
	}
	action := path[strings.LastIndex(path, "/")+1]
	status["command"] = action
	status["status"] = action
	return status, nil
}

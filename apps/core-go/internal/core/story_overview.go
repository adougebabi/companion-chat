package core

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5"
)

// Same source-liveness contract as prompt retrieval, including legacy_unknown.
// This is a read predicate; it never repairs or reclassifies a memory.
const storyEffectiveMemorySQL = `m.status='active' AND m.provenance_status NOT IN ('pending','invalid') AND
 (m.provenance_status='legacy_unknown' OR (
 (SELECT count(*) FILTER (WHERE l.status='valid' AND public.memory_source_is_live(l.source_kind,l.source_id,l.source_revision,l.source_fingerprint)) FROM public.memory_source_links l WHERE l.memory_id=m.id AND l.memory_revision=m.revision)>0 AND
 (NOT EXISTS(SELECT 1 FROM public.conversation_daily_memories d WHERE d.memory_id=m.id) OR
 (SELECT count(*) FILTER (WHERE l.status='valid' AND public.memory_source_is_live(l.source_kind,l.source_id,l.source_revision,l.source_fingerprint))=count(*) FROM public.memory_source_links l WHERE l.memory_id=m.id AND l.memory_revision=m.revision))))`

const storySharedMessagesSQL = `SELECT m.* FROM public.conversation_messages m
 JOIN public.conversation_participants f ON f.conversation_id=m.conversation_id AND f.actor_id=$1
 JOIN public.conversation_participants h ON h.conversation_id=m.conversation_id AND h.actor_id=$2
 WHERE m.author_actor_id IN ($1,$2) AND m.kind IN ('user','assistant')
 AND m.created_at>=GREATEST(f.joined_at,h.joined_at)
 AND (f.left_at IS NULL OR m.created_at<=f.left_at) AND (h.left_at IS NULL OR m.created_at<=h.left_at)`

// All metrics and cited rows belong to one Owner-authorized repeatable snapshot.
// No narrative is generated and no relationship or life event is created here.
func (a *App) OwnerStoryOverview(ctx context.Context, owner, fid string, q url.Values) (map[string]any, error) {
	if len(q) > 0 {
		return nil, invalid("story overview does not accept filters")
	}
	tx, err := a.DB.Pool().BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var authorized bool
	if err = tx.QueryRow(ctx, `SELECT true FROM public.fluctlights WHERE id=$1 AND created_by_actor_id=$2`, fid, owner).Scan(&authorized); errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	zone, err := readLifeContextTimezoneWith(ctx, tx, fid)
	if err != nil {
		return nil, err
	}
	location, err := time.LoadLocation(zone)
	if err != nil {
		return nil, err
	}
	var asOf time.Time
	if err = tx.QueryRow(ctx, `SELECT transaction_timestamp()`).Scan(&asOf); err != nil {
		return nil, err
	}
	profile := "default"
	if err = tx.QueryRow(ctx, `SELECT COALESCE(active_profile_id,'default') FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1`, fid).Scan(&profile); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var first *time.Time
	var turns, memories, diaries int64
	if err = tx.QueryRow(ctx, `WITH shared AS (`+storySharedMessagesSQL+`), completed AS (
 SELECT DISTINCT u.conversation_id,u.turn_id FROM shared u WHERE u.kind='user' AND u.author_actor_id=$2 AND NULLIF(btrim(u.turn_id),'') IS NOT NULL
 AND EXISTS(SELECT 1 FROM shared r WHERE r.conversation_id=u.conversation_id AND r.turn_id=u.turn_id AND r.kind='assistant' AND r.author_actor_id=$1 AND NULLIF(btrim(r.text),'') IS NOT NULL))
 SELECT (SELECT min(created_at) FROM shared),(SELECT count(*) FROM completed)`, fid, owner).Scan(&first, &turns); err != nil {
		return nil, err
	}
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM public.memories m WHERE m.owner_fluctlight_id=$1 AND m.actor_refs ? $2 AND `+storyEffectiveMemorySQL, fid, owner).Scan(&memories); err != nil {
		return nil, err
	}
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_diary_days d JOIN public.fluctlight_diary_attempts g ON g.id=d.active_attempt_id WHERE d.fluctlight_id=$1 AND d.status='ready' AND NULLIF(btrim(g.body),'') IS NOT NULL`, fid).Scan(&diaries); err != nil {
		return nil, err
	}
	var relationRaw []byte
	err = tx.QueryRow(ctx, `SELECT jsonb_build_object('id',r.id,'profile_id',r.profile_id,'target_actor_id',r.target_actor_id,'role',r.role,'summary',r.summary,'metrics',r.metrics,'revision',r.revision,'provenance',r.provenance) FROM public.relationships r WHERE r.owner_fluctlight_id=$1 AND r.target_actor_id=$2 AND (r.profile_id=$3 OR r.profile_id IS NULL) ORDER BY CASE WHEN r.profile_id=$3 THEN 0 ELSE 1 END LIMIT 1`, fid, owner, profile).Scan(&relationRaw)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var relation any
	if len(relationRaw) > 0 {
		relation = decodeObject(relationRaw)
	}
	days := int64(0)
	var firstAt any
	if first != nil {
		from, to := first.In(location), asOf.In(location)
		fromDay := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
		toDay := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC)
		days = int64(toDay.Sub(fromDay)/(24*time.Hour)) + 1
		if days < 0 {
			days = 0
		}
		firstAt = first.UTC().Format(time.RFC3339Nano)
	}
	footprints := []any{}
	rows, err := tx.Query(ctx, `SELECT jsonb_build_object('id',m.id,'kind','memory','title',m.content,'revision',m.revision,'at',m.created_at,'source_refs',m.evidence_refs) FROM public.memories m WHERE m.owner_fluctlight_id=$1 AND m.actor_refs ? $2 AND `+storyEffectiveMemorySQL+` ORDER BY m.created_at DESC,m.id DESC LIMIT 5`, fid, owner)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			rows.Close()
			return nil, err
		}
		footprints = append(footprints, decodeObject(raw))
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = tx.Query(ctx, `SELECT jsonb_build_object('id',d.id,'kind','diary','title',left(g.body,180),'revision',d.revision,'at',NULL,'local_date',d.local_date::text) FROM public.fluctlight_diary_days d JOIN public.fluctlight_diary_attempts g ON g.id=d.active_attempt_id WHERE d.fluctlight_id=$1 AND d.status='ready' AND NULLIF(btrim(g.body),'') IS NOT NULL ORDER BY d.local_date DESC,d.id DESC LIMIT 3`, fid)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			rows.Close()
			return nil, err
		}
		footprints = append(footprints, decodeObject(raw))
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = tx.Query(ctx, `WITH shared AS (`+storySharedMessagesSQL+`) SELECT jsonb_build_object('id',id,'kind','remote_communication','title',text,'at',created_at,'conversation_id',conversation_id,'source_ref','message:'||id) FROM shared ORDER BY created_at DESC,id DESC LIMIT 3`, fid, owner)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			rows.Close()
			return nil, err
		}
		footprints = append(footprints, decodeObject(raw))
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}

	if len(relationRaw) > 0 {
		r := decodeObject(relationRaw)
		footprints = append(footprints, map[string]any{"id": r["id"], "kind": "relationship", "title": firstString(r["summary"], firstString(mapValue(r["role"])["label"], "当前关系记录")), "revision": r["revision"], "at": nil})
	}
	rows, err = tx.Query(ctx, `SELECT jsonb_build_object('id',p.request_id,'kind','promise','title',p.title,'revision',p.revision,'at',NULL,'status',r.status) FROM public.owner_promise_links p JOIN public.owner_surface_records r ON r.id=p.request_id WHERE p.owner_actor_id=$1 AND p.fluctlight_id=$2 AND p.profile_id=$3 AND p.cancelled_at IS NULL AND r.status IN ('accepted','awaiting_goal','deferred') ORDER BY p.created_at DESC,p.request_id DESC LIMIT 3`, owner, fid, profile)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			rows.Close()
			return nil, err
		}
		footprints = append(footprints, decodeObject(raw))
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	metric := func(value any) map[string]any {
		return map[string]any{"value": value, "available": true, "complete": true}
	}
	return map[string]any{"as_of": asOf.UTC().Format(time.RFC3339Nano), "timezone": zone, "fluctlight_id": fid, "actor_id": owner, "profile_id": profile, "first_communication_at": firstAt, "metrics": map[string]any{"companionship_days": metric(days), "completed_turns": metric(turns), "relationship": metric(relation), "memories": metric(memories), "diaries": metric(diaries)}, "footprints": footprints, "footprints_complete": false, "semantics": map[string]any{"communication": "actual_participation_intervals", "turns": "distinct_conversation_and_turn_with_persisted_owner_input_and_nonempty_character_reply", "memories": "active_owner_actor_refs_and_current_source_liveness", "diaries": "ready_day_with_nonempty_active_attempt_body", "copresent": false}}, nil
}

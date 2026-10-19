package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

const npcOpportunityFactType = "life.npc.opportunity"

func (a *App) createNPCOpportunityTx(ctx context.Context, tx pgx.Tx, owner, fid, world string, npc, presence map[string]any, at time.Time) error {
	_, life, err := resolveLifeContextSnapshotWith(ctx, tx, fid, at)
	if err != nil {
		return err
	}
	characterSource, err := npcCharacterPresenceWith(ctx, tx, fid, life, at)
	if err != nil {
		return err
	}
	if characterSource == nil || life["place_id"] != presence["place_id"] || stringValue(life["event_id"]) == "" {
		return nil
	}
	// An executed NPC presence and an effective character presence source form an opportunity,
	// never proof of acquaintance, interaction, or the Owner being physically here.
	var cooled bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.world_npc_opportunities WHERE fluctlight_id=$1 AND world_id=$2 AND npc_id=$3 AND cooldown_until>$4)`, fid, world, npc["id"], at).Scan(&cooled); err != nil {
		return err
	}
	if cooled {
		return nil
	}
	end, e1 := time.Parse(time.RFC3339Nano, stringValue(life["expires_at"]))
	npcEnd, e2 := time.Parse(time.RFC3339Nano, stringValue(presence["end_at"]))
	if e1 != nil || e2 != nil {
		return errors.New("npc_opportunity_source_window_invalid")
	}
	if npcEnd.Before(end) {
		end = npcEnd
	}
	if characterSource["kind"] == "active_activity" {
		activityEnd, err := time.Parse(time.RFC3339Nano, stringValue(characterSource["expires_at"]))
		if err != nil {
			return err
		}
		if activityEnd.Before(end) {
			end = activityEnd
		}
	}
	if !end.After(at) {
		return nil
	}
	profile, err := (&intentionService{}).resolveProfile(ctx, tx, fid, "")
	if err != nil {
		return err
	}
	id := "npc_opportunity_" + stableDigest(fmt.Sprint(fid, ":", profile, ":", presence["id"], ":", life["event_id"], ":", life["event_revision"]))
	cmd, err := tx.Exec(ctx, `INSERT INTO public.world_npc_opportunities(id,world_id,fluctlight_id,owner_actor_id,profile_id,npc_id,place_id,npc_presence_id,npc_revision,source_event_id,source_event_revision,expected_life_revision,expires_at,cooldown_until,created_at,available_at,character_presence_source,source_activity_id,source_activity_revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$15,$16,$17,$18) ON CONFLICT DO NOTHING`, id, world, fid, owner, profile, npc["id"], presence["place_id"], presence["id"], npc["revision"], life["event_id"], life["event_revision"], life["context_revision"], end, at.Add(30*time.Minute), at, characterSource["kind"], nullableString(stringValue(characterSource["activity_id"])), characterSource["activity_revision"])
	if err != nil || cmd.RowsAffected() == 0 {
		return err
	}
	return a.queueNPCOpportunityFactTx(ctx, tx, id, fid, 1, at)
}
func (a *App) queueNPCOpportunityFactTx(ctx context.Context, tx pgx.Tx, id, fid string, attempt int, at time.Time) error {
	source, err := a.enqueueNativeFactTx(ctx, tx, fid, "", id, npcOpportunityFactType, fmt.Sprintf("npc-opportunity:%s:%d", id, attempt), map[string]any{"opportunity_id": id, "attempt": attempt})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE public.world_npc_opportunities SET source_fact_id=$2,attempt_count=$3,status='pending',available_at=$4,revision=revision+1,updated_at=now() WHERE id=$1`, id, source, attempt, at)
	return err
}

func readNPCOpportunityWith(ctx context.Context, q lifeContextQuerier, owner, fid, id string, lock bool) (map[string]any, error) {
	suffix := ""
	if lock {
		suffix = " FOR UPDATE OF o"
	}
	var raw []byte
	err := q.QueryRow(ctx, `SELECT to_jsonb(o)||jsonb_build_object('npc',jsonb_build_object('actor_id',n.actor_id,'public_role',n.role,'known',EXISTS(SELECT 1 FROM public.relationships r WHERE r.owner_fluctlight_id=o.fluctlight_id AND r.target_actor_id=n.actor_id AND (r.profile_id IS NULL OR r.profile_id=o.profile_id))),'place_name',p.name) FROM public.world_npc_opportunities o JOIN public.world_npcs n ON n.id=o.npc_id AND n.world_id=o.world_id JOIN public.world_places p ON p.id=o.place_id AND p.world_id=o.world_id WHERE o.owner_actor_id=$1 AND o.fluctlight_id=$2 AND o.id=$3`+suffix, owner, fid, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return decodeObject(raw), nil
}
func (a *App) validateNPCOpportunityTx(ctx context.Context, tx pgx.Tx, p map[string]any, at time.Time) (map[string]any, error) {
	fid := stringValue(p["fluctlight_id"])
	var eligible bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.world_npc_effective_presence_sources pr JOIN public.world_npcs n ON n.id=pr.npc_id AND n.world_id=pr.world_id JOIN public.actors na ON na.id=n.actor_id AND na.actor_type='npc' AND na.status='active' JOIN public.world_places wp ON wp.id=pr.place_id AND wp.world_id=pr.world_id JOIN public.fluctlight_world_bindings b ON b.world_id=pr.world_id AND b.fluctlight_id=$2 JOIN public.life_events e ON e.id=$3 AND e.fluctlight_id=$2 JOIN public.fluctlights f ON f.id=$2 WHERE pr.id=$1 AND pr.npc_id=$4 AND pr.place_id=$5 AND pr.status='active' AND pr.start_at<=$6 AND pr.end_at>$6 AND pr.definition_revision=n.revision AND n.revision=$7 AND n.status='active' AND n.enabled AND wp.status='active' AND e.status IN ('confirmed','inferred') AND e.revision=$8 AND e.start_at<=$6 AND e.end_at>$6 AND (e.expires_at IS NULL OR e.expires_at>$6) AND f.status='active')`, p["npc_presence_id"], fid, p["source_event_id"], p["npc_id"], p["place_id"], at, p["npc_revision"], p["source_event_revision"]).Scan(&eligible)
	if err != nil {
		return nil, err
	}
	end, err := time.Parse(time.RFC3339Nano, stringValue(p["expires_at"]))
	if err != nil {
		return nil, err
	}
	if !eligible || !end.After(at) {
		return nil, newCapabilityError("npc_opportunity_expired", false, ErrConflict)
	}
	profile, err := (&intentionService{}).resolveProfile(ctx, tx, fid, "")
	if err != nil {
		return nil, err
	}
	if profile != p["profile_id"] {
		return nil, newCapabilityError("npc_opportunity_profile_changed", false, ErrConflict)
	}
	_, life, err := resolveLifeContextSnapshotWith(ctx, tx, fid, at)
	if err != nil {
		return nil, err
	}
	characterSource, err := npcCharacterPresenceWith(ctx, tx, fid, life, at)
	if err != nil {
		return nil, err
	}
	if characterSource == nil || characterSource["kind"] != p["character_presence_source"] || stringValue(characterSource["activity_id"]) != stringValue(p["source_activity_id"]) || intValue(characterSource["activity_revision"]) != intValue(p["source_activity_revision"]) || life["event_id"] != p["source_event_id"] || life["place_id"] != p["place_id"] {
		return nil, newCapabilityError("npc_opportunity_presence_changed", false, ErrConflict)
	}
	return life, nil
}

// Only explicit deferred decisions receive another bounded attempt. A failed
// Provider remains visible for manual recovery, rather than an endless loop.
func (a *App) RepairNPCOpportunities(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 100 {
		return 0, ErrInvalidArguments
	}
	// Select revoked sources as well as timers. Scanning all pending rows with
	// LIMIT would let valid old opportunities starve cancelled newer ones.
	// The transaction below still performs the full authoritative validation.
	rows, err := a.DB.Pool().Query(ctx, `SELECT o.id,o.owner_actor_id,o.fluctlight_id FROM public.world_npc_opportunities o
 WHERE o.status IN ('pending','deferred') AND (o.expires_at<=$1 OR (o.status='deferred' AND o.available_at<=$1)
 OR NOT EXISTS(SELECT 1 FROM public.life_events e WHERE e.id=o.source_event_id AND e.fluctlight_id=o.fluctlight_id AND e.revision=o.source_event_revision AND e.status IN ('confirmed','inferred') AND e.start_at<=$1 AND e.end_at>$1 AND (e.expires_at IS NULL OR e.expires_at>$1))
 OR NOT EXISTS(SELECT 1 FROM public.world_npc_effective_presence_sources pr JOIN public.world_npcs n ON n.id=pr.npc_id AND n.world_id=pr.world_id JOIN public.actors na ON na.id=n.actor_id AND na.actor_type='npc' AND na.status='active' JOIN public.world_places p ON p.id=pr.place_id AND p.world_id=pr.world_id JOIN public.fluctlight_world_bindings b ON b.world_id=pr.world_id AND b.fluctlight_id=o.fluctlight_id JOIN public.fluctlights f ON f.id=o.fluctlight_id AND f.status='active'
 WHERE pr.id=o.npc_presence_id AND pr.npc_id=o.npc_id AND pr.place_id=o.place_id AND pr.status='active' AND pr.start_at<=$1 AND pr.end_at>$1 AND pr.definition_revision=n.revision AND n.revision=o.npc_revision AND n.status='active' AND n.enabled AND p.status='active')
 OR o.profile_id<>COALESCE((SELECT active_profile_id FROM public.fluctlight_personality_runtime WHERE fluctlight_id=o.fluctlight_id),'default')
 OR (o.character_presence_source='active_activity' AND NOT EXISTS(SELECT 1 FROM public.fluctlight_life_activity_runs r WHERE r.id=o.source_activity_id AND r.fluctlight_id=o.fluctlight_id AND r.authority_event_id=o.source_event_id AND r.revision=o.source_activity_revision AND r.status IN ('in_progress','deferred') AND r.started_at<=$1 AND r.active_until>$1 AND (r.intention_id IS NULL OR EXISTS(SELECT 1 FROM public.fluctlight_intentions i WHERE i.id=r.intention_id AND i.fluctlight_id=r.fluctlight_id AND i.status IN ('qualified','due','in_progress'))))))
 ORDER BY o.available_at,o.id LIMIT $2`, a.now().UTC(), limit)
	if err != nil {
		return 0, err
	}
	type target struct{ id, owner, fid string }
	targets := []target{}
	for rows.Next() {
		var t target
		if err := rows.Scan(&t.id, &t.owner, &t.fid); err != nil {
			rows.Close()
			return 0, err
		}
		targets = append(targets, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	count := 0
	for _, target := range targets {
		err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			if err := lockLifeContextTx(ctx, tx, target.fid); err != nil {
				return err
			}
			p, err := readNPCOpportunityWith(ctx, tx, target.owner, target.fid, target.id, true)
			if err != nil {
				return err
			}
			if p["status"] != "pending" && p["status"] != "deferred" {
				return nil
			}
			if _, err := a.validateNPCOpportunityTx(ctx, tx, p, a.now().UTC()); err != nil {
				if !errors.Is(err, ErrConflict) {
					return err
				}
				_, err = tx.Exec(ctx, `UPDATE public.world_npc_opportunities SET status='expired',error_code='npc_opportunity_expired',revision=revision+1,updated_at=now() WHERE id=$1`, target.id)
				return err
			}
			if p["status"] == "deferred" && intValue(p["attempt_count"]) < 3 {
				return a.queueNPCOpportunityFactTx(ctx, tx, target.id, target.fid, intValue(p["attempt_count"])+1, a.now().UTC())
			}
			if p["status"] == "deferred" {
				_, err := tx.Exec(ctx, `UPDATE public.world_npc_opportunities SET status='failed',error_code='npc_opportunity_defer_limit',result='{"status":"failed","reason":"deferred_attempts_exhausted","acquaintance_created":false}',revision=revision+1,updated_at=now() WHERE id=$1`, target.id)
				return err
			}
			return nil
		})
		if err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

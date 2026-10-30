package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func npcDayMatches(kind string, day time.Weekday) bool {
	weekend := day == time.Saturday || day == time.Sunday
	return kind == "daily" || (kind == "weekday" && !weekend) || (kind == "weekend" && weekend)
}
func npcClockAt(day time.Time, minute int) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), minute/60, minute%60, 0, 0, day.Location())
}

// The next routine boundary limits a home/fixed interval. Prediction and the
// deterministic virtual-world executor use exactly the same routine semantics.
func npcNextBoundary(npc map[string]any, at time.Time) time.Time {
	next := npcClockAt(at.AddDate(0, 0, 1), 0)
	if npc["position_mode"] == "fixed" {
		return next
	}
	for day := 0; day <= 1; day++ {
		date := at.AddDate(0, 0, day)
		for _, raw := range arrayValue(npc["routine"]) {
			slot := mapValue(raw)
			minute, ok := parseNPCClock(stringValue(slot["start"]))
			if !ok || !npcDayMatches(stringValue(slot["day"]), date.Weekday()) {
				continue
			}
			start := npcClockAt(date, minute)
			if start.After(at) && start.Before(next) {
				next = start
			}
		}
	}
	return next
}

func (a *App) advanceNPCPresenceTx(ctx context.Context, tx pgx.Tx, fid, world string, npc map[string]any, at time.Time) (map[string]any, error) {
	id := stringValue(npc["id"])
	if npc["status"] != "active" || npc["enabled"] == false {
		return nil, invalidateNPCPresenceTx(ctx, tx, id, at, "npc_disabled")
	}
	var visitRaw []byte
	visitErr := tx.QueryRow(ctx, `SELECT to_jsonb(p) FROM public.world_npc_effective_presence_sources p WHERE npc_id=$1 AND definition_revision=$2 AND source_kind='world_event_execution' AND status='active' AND start_at<=$3 AND end_at>$3 ORDER BY start_at DESC LIMIT 1`, id, npc["revision"], at).Scan(&visitRaw)
	if visitErr == nil {
		return decodeObject(visitRaw), nil
	}
	if !errors.Is(visitErr, pgx.ErrNoRows) {
		return nil, visitErr
	}
	zone, err := readWorldTimezoneWith(ctx, tx, world)
	if err != nil {
		return nil, err
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return nil, err
	}
	planned := npcPlannedPosition(npc, at.In(loc))
	if planned == nil {
		return nil, invalidateNPCPresenceTx(ctx, tx, id, at, "npc_position_unknown")
	}
	place := stringValue(planned["place_id"])
	var valid bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.world_places WHERE id=$1 AND world_id=$2 AND status='active')`, place, world).Scan(&valid); err != nil {
		return nil, err
	}
	if !valid {
		return nil, invalidateNPCPresenceTx(ctx, tx, id, at, "place_inactive")
	}
	end, err := time.Parse(time.RFC3339Nano, stringValue(planned["window_end"]))
	if err != nil || !end.After(at) {
		return nil, ErrInvalidArguments
	}
	start, err := time.Parse(time.RFC3339Nano, stringValue(planned["window_start"]))
	if err != nil {
		return nil, err
	}
	source := "npc-routine:" + stableDigest(fmt.Sprint(id, ":", npc["revision"], ":", place, ":", end.UTC().Format(time.RFC3339Nano)))
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT to_jsonb(p) FROM public.world_npc_presence_runs p WHERE npc_id=$1 AND (source_ref=$2 OR source_ref LIKE $2||':segment:%') ORDER BY CASE WHEN status='active' THEN 0 ELSE 1 END,start_at DESC,id DESC LIMIT 1`, id, source).Scan(&raw)
	if err == nil {
		previous := decodeObject(raw)
		if previous["status"] == "active" {
			return previous, nil
		}
		// An interrupted interval resumes as a new executed segment. Keep the
		// cancelled segment, and never claim the time spent visiting as routine.
		source += ":segment:" + stableDigest(stringValue(previous["id"])+":"+at.UTC().Format(time.RFC3339Nano))
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	// Do not reconstruct time before the executor actually ran. Only the current
	// interval begins now; missed offline intervals have no executed presence.
	if err := invalidateNPCPresenceTx(ctx, tx, id, at, "routine_advanced"); err != nil {
		return nil, err
	}
	presence := "npc_presence_" + stableDigest(source)
	err = tx.QueryRow(ctx, `INSERT INTO public.world_npc_presence_runs(id,world_id,npc_id,actor_id,place_id,definition_revision,source_kind,source_ref,planned_start,planned_end,start_at,end_at,status,activity) VALUES($1,$2,$3,$4,$5,$6,'npc_routine_execution',$7,$8,$9,$10,$9,'active',$11) RETURNING to_jsonb(world_npc_presence_runs)`, presence, world, id, npc["actor_id"], place, intValue(npc["revision"]), source, start.UTC(), end.UTC(), at.UTC(), stringValue(planned["activity"])).Scan(&raw)
	if err != nil {
		return nil, err
	}
	return decodeObject(raw), nil
}
func invalidateNPCPresenceTx(ctx context.Context, tx pgx.Tx, npc string, at time.Time, reason string) error {
	_, err := tx.Exec(ctx, `UPDATE public.world_npc_presence_runs SET status=CASE WHEN end_at<=$2 THEN 'ended' ELSE 'cancelled' END,ended_at=LEAST(end_at,$2) WHERE npc_id=$1 AND status='active'`, npc, at.UTC())
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE public.world_npc_opportunities SET status='expired',error_code=$2,revision=revision+1,updated_at=now() WHERE npc_id=$1 AND status IN ('pending','deferred') AND npc_presence_id IN(SELECT id FROM public.world_npc_presence_runs WHERE npc_id=$1 AND status<>'active')`, npc, reason)
	return err
}

// Bounded repair drives actual runtime presence and opportunity production;
// all browser GETs remain pure reads. No model call occurs in this method.
func (a *App) RepairWorldNPCPresence(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 100 {
		return 0, ErrInvalidArguments
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT n.id,w.id,''::text,w.owner_actor_id FROM public.world_npcs n JOIN public.life_worlds w ON w.id=n.world_id LEFT JOIN public.world_npc_runtime_clocks c ON c.npc_id=n.id WHERE (c.next_attempt_at IS NULL OR c.next_attempt_at<=$1) ORDER BY COALESCE(c.last_tick_at,'epoch'::timestamptz),n.id LIMIT $2`, a.now().UTC(), limit)
	if err != nil {
		return 0, err
	}
	type target struct{ npc, world, fid, owner string }
	targets := []target{}
	for rows.Next() {
		var t target
		if err := rows.Scan(&t.npc, &t.world, &t.fid, &t.owner); err != nil {
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
	failures := []error{}
	for _, target := range targets {
		err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			if err := lockWorldRuntimeTx(ctx, tx, target.world); err != nil {
				return err
			}
			var raw []byte
			if err := tx.QueryRow(ctx, `SELECT to_jsonb(n) FROM public.world_npcs n WHERE id=$1 AND world_id=$2 FOR UPDATE`, target.npc, target.world).Scan(&raw); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return nil
				}
				return err
			}
			npc := decodeObject(raw)
			presence, err := a.advanceNPCPresenceTx(ctx, tx, target.fid, target.world, npc, a.now().UTC())
			if err != nil {
				return err
			}
			if presence != nil && presence["status"] == "active" {
				if err := a.createWorldMemberOpportunitiesTx(ctx, tx, target.world, npc, presence); err != nil {
					return err
				}
			}
			_, err = tx.Exec(ctx, `INSERT INTO public.world_npc_runtime_clocks(npc_id,world_id,last_tick_at) VALUES($1,$2,$3) ON CONFLICT(npc_id) DO UPDATE SET last_tick_at=EXCLUDED.last_tick_at,next_attempt_at=NULL,error_code=NULL,failure_count=0`, target.npc, target.world, a.now().UTC())
			return err
		})
		if err != nil {
			failures = append(failures, fmt.Errorf("NPC presence tick %s: %w", target.npc, err))
			// Operational failure is isolated from the rolled-back business tick.
			// Keep the real failure visible and back off this target, then allow
			// the rest of the batch to progress instead of silently returning [].
			_, recordErr := a.DB.Pool().Exec(ctx, `INSERT INTO public.world_npc_runtime_clocks(npc_id,world_id,last_tick_at,next_attempt_at,error_code,last_failure_at,failure_count) SELECT n.id,n.world_id,$3,$4,'npc_presence_tick_failed',$3,1 FROM public.world_npcs n WHERE n.id=$1 AND n.world_id=$2 ON CONFLICT(npc_id) DO UPDATE SET last_tick_at=EXCLUDED.last_tick_at,next_attempt_at=EXCLUDED.next_attempt_at,error_code=EXCLUDED.error_code,last_failure_at=EXCLUDED.last_failure_at,failure_count=world_npc_runtime_clocks.failure_count+1`, target.npc, target.world, a.now().UTC(), a.now().Add(time.Minute).UTC())
			if recordErr != nil {
				failures = append(failures, fmt.Errorf("record NPC tick backoff %s: %w", target.npc, recordErr))
			}
			continue
		}
		count++
	}
	return count, errors.Join(failures...)
}

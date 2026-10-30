package core

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

// RepairWorldEvents performs only committed, structured simulation effects.
// External cognition consumes the resulting native facts after these short
// transactions. It never generates or backfills an offline character routine.
func (a *App) RepairWorldEvents(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 100 {
		return 0, ErrInvalidArguments
	}
	failures := []error{}
	count := 0
	at := a.now().UTC()
	targets, err := readWorldObjects(ctx, a.DB.Pool(), `SELECT to_jsonb(t)||jsonb_build_object('fluctlight_id',f.id,'owner_actor_id',f.created_by_actor_id) FROM public.world_event_templates t JOIN public.life_worlds w ON w.id=t.world_id JOIN LATERAL(SELECT member.id,member.created_by_actor_id FROM public.fluctlight_world_bindings b JOIN public.fluctlights member ON member.id=b.fluctlight_id WHERE b.world_id=w.id AND member.status='active' ORDER BY member.id LIMIT 1) f ON true WHERE t.enabled AND t.definition->'trigger'->>'mode'='automatic' AND t.next_check_at<=$1 ORDER BY t.next_check_at,t.id LIMIT $2`, at, limit)
	if err != nil {
		return 0, err
	}
	for _, rawTarget := range targets {
		t := mapValue(rawTarget)
		err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			fid := stringValue(t["fluctlight_id"])
			if err := lockSharedWorldMembersTx(ctx, tx, fid, stringValue(t["world_id"])); err != nil {
				return err
			}
			current, err := readWorldTemplateWith(ctx, tx, fid, stringValue(t["id"]), true)
			if err != nil {
				return err
			}
			next, err := time.Parse(time.RFC3339Nano, stringValue(current["next_check_at"]))
			if err != nil {
				return err
			}
			if next.After(at) || current["enabled"] != true {
				return nil
			}
			var active bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_world_bindings b JOIN public.fluctlights f ON f.id=b.fluctlight_id WHERE b.fluctlight_id=$1 AND b.world_id=$2 AND f.status='active')`, fid, current["world_id"]).Scan(&active); err != nil {
				return err
			}
			if !active {
				return nil
			}
			d := mapValue(current["definition"])
			if mapValue(d["trigger"])["mode"] != "automatic" {
				return nil
			}
			if err := validateWorldEventDefinition(d); err != nil {
				return err
			}
			key := "automatic:" + stringValue(t["id"]) + ":" + fmt.Sprint(current["revision"]) + ":" + next.UTC().Format(time.RFC3339Nano)
			if _, err := a.queueWorldEventTx(ctx, tx, stringValue(t["owner_actor_id"]), fid, current, "automatic", key, at); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `UPDATE public.world_event_templates SET next_check_at=$2,last_checked_at=$3,error_code=NULL WHERE id=$1`, current["id"], at.Add(time.Duration(intValue(mapValue(d["trigger"])["interval_minutes"]))*time.Minute), at)
			return err
		})
		if err != nil {
			failures = append(failures, fmt.Errorf("queue world event %s: %w", t["id"], err))
			if _, e := a.DB.Pool().Exec(ctx, `UPDATE public.world_event_templates SET next_check_at=$2,last_checked_at=$3,error_code='world_event_queue_failed' WHERE id=$1`, t["id"], at.Add(time.Minute), at); e != nil {
				failures = append(failures, e)
			}
		}
	}
	runs, err := readWorldObjects(ctx, a.DB.Pool(), `SELECT to_jsonb(r) FROM public.world_event_runs r WHERE status='queued' AND due_at<=$1 ORDER BY due_at,id LIMIT $2`, at, limit)
	if err != nil {
		return count, errors.Join(append(failures, err)...)
	}
	for _, rawRun := range runs {
		run := mapValue(rawRun)
		if err := a.executeWorldEvent(ctx, run, at); err != nil {
			failures = append(failures, fmt.Errorf("execute world event %s: %w", run["id"], err))
			// Failure diagnostics persist outside the rolled-back effect transaction.
			if _, e := a.DB.Pool().Exec(ctx, `UPDATE public.world_event_runs SET status='failed',attempt_count=attempt_count+1,error_code='world_event_execution_failed',revision=revision+1,updated_at=now() WHERE id=$1 AND status='queued'`, run["id"]); e != nil {
				failures = append(failures, e)
			}
		} else {
			count++
		}
	}
	recovery, err := readWorldObjects(ctx, a.DB.Pool(), `SELECT to_jsonb(r) FROM public.world_event_runs r WHERE recovery_status='pending' AND end_at<=$1 AND (recovery_next_at IS NULL OR recovery_next_at<=$1) ORDER BY COALESCE(recovery_next_at,end_at),id LIMIT $2`, at, limit)
	if err != nil {
		return count, errors.Join(append(failures, err)...)
	}
	for _, rawRun := range recovery {
		run := mapValue(rawRun)
		if err := a.recoverWorldEvent(ctx, run, at); err != nil {
			failures = append(failures, fmt.Errorf("recover world event %s: %w", run["id"], err))
			if _, e := a.DB.Pool().Exec(ctx, `UPDATE public.world_event_runs SET recovery_next_at=$2::timestamptz+interval '5 minutes',recovery_result=jsonb_build_object('error_code','world_event_recovery_failed','last_attempt_at',$2::timestamptz),updated_at=now() WHERE id=$1 AND recovery_status='pending'`, run["id"], at); e != nil {
				failures = append(failures, e)
			}
		} else {
			count++
		}
	}
	return count, errors.Join(failures...)
}
func (a *App) executeWorldEvent(ctx context.Context, run map[string]any, at time.Time) error {
	if stringValue(run["fluctlight_id"]) == "" {
		return a.executeAmbientWorldEvent(ctx, run, at)
	}
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		fid := stringValue(run["fluctlight_id"])
		if err := lockSharedWorldMembersTx(ctx, tx, fid, stringValue(run["world_id"])); err != nil {
			return err
		}
		r, err := readWorldEventRunWith(ctx, tx, stringValue(run["owner_actor_id"]), fid, stringValue(run["template_id"]), stringValue(run["id"]), true)
		if err != nil {
			return err
		}
		if r["status"] != "queued" {
			return nil
		}
		t, err := readWorldTemplateWith(ctx, tx, fid, stringValue(r["template_id"]), true)
		if err != nil {
			return err
		}
		filtered := func(reason string) error {
			_, err := tx.Exec(ctx, `UPDATE public.world_event_runs SET status='filtered',result=jsonb_build_object('reason',$2::text,'life_write',false),started_at=$3,attempt_count=attempt_count+1,revision=revision+1,updated_at=now() WHERE id=$1`, r["id"], reason, at)
			return err
		}
		expires, err := time.Parse(time.RFC3339Nano, stringValue(r["expires_at"]))
		if err != nil {
			return err
		}
		if !expires.After(at) {
			return filtered("execution_window_expired")
		}
		var active bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_world_bindings b JOIN public.fluctlights f ON f.id=b.fluctlight_id WHERE b.fluctlight_id=$1 AND b.world_id=$2 AND f.status='active')`, fid, r["world_id"]).Scan(&active); err != nil {
			return err
		}
		if !active {
			return filtered("world_or_character_inactive")
		}
		if t["enabled"] != true || intValue(t["revision"]) != intValue(r["template_revision"]) {
			return filtered("template_disabled_or_changed")
		}
		d := mapValue(r["definition"])
		if err := validateWorldEventDefinition(d); err != nil {
			return err
		}
		if err := validateWorldEventReferencesTx(ctx, tx, stringValue(r["world_id"]), d); err != nil {
			return err
		}
		if t["last_triggered_at"] != nil {
			last, err := time.Parse(time.RFC3339Nano, stringValue(t["last_triggered_at"]))
			if err != nil {
				return err
			}
			cooldown := intValue(mapValue(d["trigger"])["cooldown_minutes"])
			duration := intValue(d["duration_minutes"])
			if cooldown < duration {
				cooldown = duration
			}
			if at.Before(last.Add(time.Duration(cooldown) * time.Minute)) {
				return filtered("cooldown_or_active_effect")
			}
		}
		_, life, err := resolveLifeContextSnapshotWith(ctx, tx, fid, at)
		if err != nil {
			return err
		}
		zone, err := readWorldTimezoneWith(ctx, tx, stringValue(r["world_id"]))
		if err != nil {
			return err
		}
		loc, err := time.LoadLocation(zone)
		if err != nil {
			return err
		}
		c := mapValue(d["condition"])
		if reason := worldEventConditionReason(c, life, at.In(loc)); reason != "" {
			return filtered(reason)
		}
		if c["require_presence"] == true || len(arrayValue(c["place_ids"])) > 0 {
			presence, err := npcCharacterPresenceWith(ctx, tx, fid, life, at)
			if err != nil {
				return err
			}
			if presence == nil {
				return filtered("confirmed_character_presence_required")
			}
		}
		probability, _ := eventNumber(mapValue(d["trigger"])["probability"])
		sample, _ := strconv.ParseUint(stableDigest(stringValue(r["source_key"]))[:8], 16, 32)
		if r["trigger_kind"] == "automatic" && float64(sample)/4294967296 >= probability {
			return filtered("probability_not_selected")
		}
		end := at.Add(time.Duration(intValue(d["duration_minutes"])) * time.Minute)
		bodyEffect := map[string]any(nil)
		kind := "world_event"
		recovery := "not_required"
		for _, raw := range arrayValue(d["effects"]) {
			e := mapValue(raw)
			if e["kind"] == "temporary_injury" {
				bodyEffect = mapValue(e["body_effect"])
				kind = "body_injury"
				recovery = "pending"
			}
		}
		attribution := lifeEventRuntimeEffect{Ambient: true, Facts: map[string]any{"run_id": r["id"], "world_id": r["world_id"], "template_id": r["template_id"], "template_revision": r["template_revision"], "description": d["description"], "effects": d["effects"], "source": "virtual_world_event", "owner_copresent": false}}
		eventCtx := context.WithValue(context.WithValue(ctx, lifeEventSourceKey{}, "world-event:"+stringValue(r["id"])), lifeEventRuntimeEffectKey{}, attribution)
		evidence := []any{"world-event-run:" + stringValue(r["id"])}
		if generation := stringValue(t["generation_id"]); generation != "" {
			evidence = append(evidence, "world-event-generation:"+generation)
		} else {
			evidence = append(evidence, "resource:"+stringValue(t["source_resource_id"])+":"+stringValue(t["source_version"]))
		}
		eventPayload := map[string]any{"kind": kind, "start_at": at.Format(time.RFC3339Nano), "end_at": end.Format(time.RFC3339Nano), "activity": d["description"], "evidence_refs": evidence, "expected_life_context_revision": life["context_revision"], "idempotency_key": "world-event:" + stringValue(r["id"])}
		if bodyEffect != nil {
			eventPayload["body_effect"] = bodyEffect
		}
		event, err := a.createLifeEventTx(eventCtx, tx, fid, fid, eventPayload)
		if err != nil {
			return err
		}
		result := map[string]any{"event_id": event["id"], "effects": d["effects"], "source": "virtual_world_event", "actual_start_at": at.Format(time.RFC3339Nano), "actual_end_at": end.Format(time.RFC3339Nano), "owner_copresent": false}
		if _, err := tx.Exec(ctx, `UPDATE public.world_event_runs SET status='applied',life_event_id=$2,started_at=$3,end_at=$4,result=$5,recovery_status=$6,error_code=NULL,attempt_count=attempt_count+1,revision=revision+1,updated_at=now() WHERE id=$1`, r["id"], event["id"], at, end, jsonBytes(result), recovery); err != nil {
			return err
		}
		for _, raw := range arrayValue(d["effects"]) {
			e := mapValue(raw)
			if e["kind"] == "npc_visit" {
				if err := a.executeWorldNPCVisitTx(ctx, tx, fid, r, e, at, end); err != nil {
					return err
				}
			}
		}
		_, err = tx.Exec(ctx, `UPDATE public.world_event_templates SET last_triggered_at=$2,error_code=NULL,updated_at=now() WHERE id=$1`, t["id"], at)
		return err
	})
}
func worldEventConditionReason(c, life map[string]any, at time.Time) string {
	start, _ := parseNPCClock(stringValue(c["start"]))
	end, _ := parseNPCClock(stringValue(c["end"]))
	minute := at.Hour()*60 + at.Minute()
	day := at.Weekday()
	if end < start && minute < end {
		day = at.AddDate(0, 0, -1).Weekday()
	}
	if !npcDayMatches(stringValue(c["day"]), day) {
		return "day_condition_not_met"
	}
	if (start < end && (minute < start || minute >= end)) || (start > end && minute < start && minute >= end) {
		return "time_condition_not_met"
	}
	places := arrayValue(c["place_ids"])
	if len(places) > 0 {
		matched := false
		for _, p := range places {
			if p == life["place_id"] {
				matched = true
			}
		}
		if !matched {
			return "place_condition_not_met"
		}
	}
	return ""
}
func (a *App) executeWorldNPCVisitTx(ctx context.Context, tx pgx.Tx, fid string, r, e map[string]any, at, end time.Time) error {
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT to_jsonb(n) FROM public.world_npcs n WHERE id=$1 AND world_id=$2 FOR UPDATE`, e["npc_id"], r["world_id"]).Scan(&raw); err != nil {
		return err
	}
	n := decodeObject(raw)
	if err := invalidateNPCPresenceTx(ctx, tx, stringValue(n["id"]), at, "world_event_visit"); err != nil {
		return err
	}
	id := "npc_presence_" + stableDigest(stringValue(r["id"])+":"+stringValue(n["id"]))
	if err := tx.QueryRow(ctx, `INSERT INTO public.world_npc_presence_runs(id,world_id,npc_id,actor_id,place_id,definition_revision,source_kind,source_ref,planned_start,planned_end,start_at,end_at,status,activity) VALUES($1,$2,$3,$4,$5,$6,'world_event_execution',$7,$8,$9,$8,$9,'active',$10) RETURNING to_jsonb(world_npc_presence_runs)`, id, r["world_id"], n["id"], n["actor_id"], e["place_id"], n["revision"], r["id"], at, end, e["value"]).Scan(&raw); err != nil {
		return err
	}
	return a.createWorldMemberOpportunitiesTx(ctx, tx, stringValue(r["world_id"]), n, decodeObject(raw))
}
func (a *App) recoverWorldEvent(ctx context.Context, run map[string]any, at time.Time) error {
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		fid := stringValue(run["fluctlight_id"])
		if err := lockLifeContextTx(ctx, tx, fid); err != nil {
			return err
		}
		r, err := readWorldEventRunWith(ctx, tx, stringValue(run["owner_actor_id"]), fid, stringValue(run["template_id"]), stringValue(run["id"]), true)
		if err != nil {
			return err
		}
		if r["recovery_status"] != "pending" {
			return nil
		}
		// Recovery closes an already committed physical effect even after a world
		// switch, template disable or retirement; it does not execute a new hazard.
		var stateRaw []byte
		if err := tx.QueryRow(ctx, `SELECT state_json FROM public.fluctlight_appearance_states WHERE fluctlight_id=$1 FOR UPDATE`, fid).Scan(&stateRaw); err != nil {
			return err
		}
		active := false
		for _, raw := range arrayValue(mapValue(decodeObject(stateRaw)["injuries"])["value"]) {
			injury := mapValue(raw)
			if injury["id"] == r["life_event_id"] && injury["status"] == "active" {
				active = true
			}
		}
		result := map[string]any{"status": "already_resolved"}
		if active {
			_, life, err := resolveLifeContextSnapshotWith(ctx, tx, fid, at)
			if err != nil {
				return err
			}
			end := at.Add(time.Second)
			result, err = a.createLifeEventTx(context.WithValue(context.WithValue(ctx, lifeEventSourceKey{}, "world-event-recovery:"+stringValue(r["id"])), lifeEventRuntimeEffectKey{}, lifeEventRuntimeEffect{Ambient: true, Facts: map[string]any{"recovery_of": r["life_event_id"], "run_id": r["id"], "source": "virtual_world_event_recovery"}}), tx, fid, fid, map[string]any{"kind": "body_recovery", "start_at": at.Format(time.RFC3339Nano), "end_at": end.Format(time.RFC3339Nano), "body_effect": map[string]any{"injury_event_id": r["life_event_id"]}, "evidence_refs": []any{r["life_event_id"]}, "expected_life_context_revision": life["context_revision"], "idempotency_key": "world-event-recovery:" + stringValue(r["id"])})
			if err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `UPDATE public.world_event_runs SET recovery_status='completed',recovery_result=$2,revision=revision+1,updated_at=now() WHERE id=$1`, r["id"], jsonBytes(result))
		return err
	})
}

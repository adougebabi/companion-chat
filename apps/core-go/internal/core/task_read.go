package core

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func (a *App) OwnerTask(ctx context.Context, owner, id string) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	var intent, workflow, kind, status, lastError string
	var payload []byte
	var attempts int
	var created time.Time
	var started, completed, next *time.Time
	err := a.DB.Pool().QueryRow(ctx, `SELECT i.intent_id,i.workflow_id,i.intent_type,i.status,i.payload,i.attempt_count,i.created_at,i.started_at,i.completed_at,i.next_attempt_at,COALESCE(i.last_error,'') FROM public.platform_workflow_intents i LEFT JOIN public.fluctlights f ON f.id=i.payload->>'fluctlight_id' WHERE (i.intent_id=$1 OR i.workflow_id=$1 OR 'go:'||i.workflow_id=$1) AND (f.created_by_actor_id=$2 OR (COALESCE(i.payload->>'fluctlight_id','')='' AND (COALESCE(i.payload->>'owner_actor_id','')='' OR i.payload->>'owner_actor_id'=$2)))`, id, owner).Scan(&intent, &workflow, &kind, &status, &payload, &attempts, &created, &started, &completed, &next, &lastError)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &SurfaceError{Status: 404, Code: "task_not_found", Message: "Task is unavailable"}
	}
	if err != nil {
		return nil, err
	}
	input := decodeObject(payload)
	source := compactStateMap(input, []string{"deletion_id", "asset_id", "inbox_id", "request_id", "source_fact_id", "action_id", "media_intent_id", "goal_id", "local_date", "correlation_id", "diary_attempt_id"})
	if lastError != "" {
		source["last_error"] = boundedLifecycleCause(lastError)
	}
	if kind == "cognition.processing" && stringValue(input["inbox_id"]) != "" {
		var raw []byte
		if err := a.DB.Pool().QueryRow(ctx, `SELECT payload FROM public.cognition_inbox WHERE id=$1 AND fluctlight_id=$2`, input["inbox_id"], input["fluctlight_id"]).Scan(&raw); err != nil {
			return nil, err
		}
		fact := decodeObject(raw)
		if fact["event_type"] == npcMomentFactType {
			candidate := mapValue(fact["candidate"])
			row, err := readNPCMomentWith(ctx, a.DB.Pool(), owner, stringValue(candidate["interaction_id"]), false)
			if err != nil {
				return nil, err
			}
			source["moment_id"], source["npc_interaction_id"], source["npc_id"], source["encounter_id"], source["copresent"] = row["moment_id"], row["id"], row["npc_id"], row["encounter_id"], false
		}
	}
	if kind == "diary.generate" {
		var diaryID string
		if err := a.DB.Pool().QueryRow(ctx, `SELECT diary_id FROM public.fluctlight_diary_attempts WHERE id=$1`, input["diary_attempt_id"]).Scan(&diaryID); err != nil {
			return nil, err
		}
		source["diary_id"] = diaryID
	}
	steps := []any{}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,event_type,COALESCE(payload->>'stage',''),COALESCE(payload->>'status',''),COALESCE(payload->>'reason_code',''),created_at FROM public.diagnostic_events WHERE event_type LIKE 'lifecycle.%' AND payload->>'intent_id'=$1 ORDER BY created_at DESC,id DESC LIMIT 101`, intent)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hasMore := false
	for rows.Next() {
		var event, eventType, stage, stageStatus, reason string
		var at time.Time
		if err := rows.Scan(&event, &eventType, &stage, &stageStatus, &reason, &at); err != nil {
			return nil, err
		}
		if len(steps) == 100 {
			hasMore = true
			break
		}
		steps = append(steps, map[string]any{"id": event, "event_type": eventType, "name": stage, "status": stageStatus, "reason_code": reason, "at": at.Format(time.RFC3339Nano)})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return map[string]any{"task_id": intent, "workflow_id": workflow, "fluctlight_id": input["fluctlight_id"], "kind": kind, "type": kind, "title": kind, "state": status, "status": status, "runs": attempts, "steps": steps, "has_more_steps": hasMore, "created_at": created.Format(time.RFC3339Nano), "started_at": started, "completed_at": completed, "next_attempt_at": next, "correlation_id": firstString(input["correlation_id"], intent), "source": source}, nil
}

// Resolve pointers from durable intent inputs rather than manufacturing a
// workflow ID from the request ID in the browser.
func (a *App) OwnerRequestTasks(ctx context.Context, owner, fid, requestID, inboxID string) ([]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT intent_id,workflow_id,intent_type,status,attempt_count FROM public.platform_workflow_intents WHERE payload->>'fluctlight_id'=$1 AND (payload->>'request_id'=$2 OR ($3<>'' AND payload->>'inbox_id'=$3)
 OR payload->>'intention_id'=(SELECT p.intention_id FROM public.owner_promise_links p WHERE p.request_id=$2 AND p.fluctlight_id=$1 AND p.owner_actor_id=$4)
 OR intent_id=(SELECT pr.id FROM public.owner_promise_links p JOIN public.goal_planning_events e ON e.fluctlight_id=p.fluctlight_id AND e.source_key=p.planning_source_key JOIN public.goal_planning_runs pr ON pr.fluctlight_id=e.fluctlight_id AND (pr.id=e.processed_run_id OR (e.processed_run_id IS NULL AND pr.watermark>=e.seq AND (pr.profile_id='' OR pr.profile_id=p.profile_id))) WHERE p.request_id=$2 AND p.fluctlight_id=$1 AND p.owner_actor_id=$4 ORDER BY pr.created_at DESC,pr.id DESC LIMIT 1)) ORDER BY created_at,intent_id`, fid, requestID, inboxID, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := []any{}
	for rows.Next() {
		var id, workflow, kind, status string
		var attempts int
		if err := rows.Scan(&id, &workflow, &kind, &status, &attempts); err != nil {
			return nil, err
		}
		tasks = append(tasks, map[string]any{"task_id": id, "workflow_id": workflow, "kind": kind, "status": status, "attempt_count": attempts})
	}
	return tasks, rows.Err()
}

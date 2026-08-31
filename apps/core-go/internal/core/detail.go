package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (a *App) FluctlightDetail(ctx context.Context, actorID, fluctlightID string) (map[string]any, error) {
	fluctlight, err := a.DB.GetFluctlight(ctx, fluctlightID, actorID)
	if err != nil {
		return nil, err
	}
	detail := map[string]any{
		"id":                fluctlight.ID,
		"identity":          fluctlight.Identity,
		"personality":       fluctlight.Personality,
		"behavioral_policy": fluctlight.BehavioralPolicy,
		"life_profile":      fluctlight.LifeProfile,
		"provenance":        fluctlight.Provenance,
		"status":            fluctlight.Status,
		"current_revision":  fluctlight.CurrentRevision,
		"self_model":        mapValue(fluctlight.Provenance["self_model"]),
	}
	inner, err := a.readInnerState(ctx, fluctlightID)
	if err != nil && err != ErrNotFound {
		return nil, err
	}
	if inner == nil {
		inner = map[string]any{}
	}
	detail["inner_state"] = inner
	goals, intentions, err := a.readAgency(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	detail["goals"] = goals
	detail["intentions"] = intentions
	detail["relationships"], err = a.readRelationships(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	detail["memories"], err = a.readMemories(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	detail["schedule"], err = a.readSchedule(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	detail["context"], err = a.resolveContext(ctx, fluctlightID, detail["schedule"])
	if err != nil {
		return nil, err
	}
	detail["hypotheses"], err = a.readActiveHypotheses(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	detail["events"], err = a.readEvents(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	detail["cognition_history"], err = a.readCognitionHistory(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	detail["foundation_revisions"], err = a.readFoundationRevisions(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	detail["evolution_revisions"], err = a.readEvolutionRevisions(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	return detail, nil
}

func (a *App) readEvolutionRevisions(ctx context.Context, fluctlightID string) ([]map[string]any, error) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,field,base_revision,revision,candidate_type,before_value,after_value,evidence_refs,source_window,status,created_at FROM public.fluctlight_evolution_revisions WHERE fluctlight_id=$1 ORDER BY revision DESC,created_at DESC LIMIT 100`, fluctlightID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var id, field, kind, source, status string
		var base, revision int
		var before, after, refs []byte
		var created time.Time
		if err := rows.Scan(&id, &field, &base, &revision, &kind, &before, &after, &refs, &source, &status, &created); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{"id": id, "field": field, "base_revision": base, "revision": revision, "candidate_type": kind, "before": decodeJSONValue(before), "after": decodeJSONValue(after), "evidence_refs": decodeArray(refs), "source_window": source, "status": status, "created_at": created.Format(time.RFC3339Nano)})
	}
	return result, rows.Err()
}

func resolveScheduleContext(value any) map[string]any {
	result := map[string]any{"source": "unknown", "scene": nil, "activity": nil, "location": nil, "instant": time.Now().UTC().Format(time.RFC3339Nano)}
	schedule, ok := value.(map[string]any)
	if !ok {
		return result
	}
	result["source"] = "schedule"
	now := time.Now().UTC()
	for _, raw := range arrayValue(schedule["items"]) {
		item := mapValue(raw)
		start, e1 := time.Parse(time.RFC3339Nano, stringValue(item["start_at"]))
		end, e2 := time.Parse(time.RFC3339Nano, stringValue(item["end_at"]))
		if e1 == nil && e2 == nil && !now.Before(start) && now.Before(end) {
			result["scene"] = item["scene"]
			result["activity"] = item["activity"]
			result["location"] = item["location"]
			break
		}
	}
	return result
}

func (a *App) readInnerState(ctx context.Context, fluctlightID string) (map[string]any, error) {
	var revision int
	var pad, mood, drives []byte
	var updated time.Time
	err := a.DB.Pool().QueryRow(ctx, `SELECT revision,pad,mood,drives,last_updated_at FROM public.fluctlight_inner_states WHERE fluctlight_id=$1`, fluctlightID).Scan(&revision, &pad, &mood, &drives, &updated)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return map[string]any{"pad": decodeObject(pad), "mood": decodeObject(mood), "drives": decodeArray(drives), "revision": revision, "last_updated_at": updated.Format(time.RFC3339Nano)}, nil
}

func (a *App) readAgency(ctx context.Context, fluctlightID string) ([]map[string]any, []map[string]any, error) {
	goalsRows, err := a.DB.Pool().Query(ctx, `SELECT id,description,status,importance,urgency,progress FROM public.fluctlight_goals WHERE fluctlight_id=$1 ORDER BY created_at`, fluctlightID)
	if err != nil {
		return nil, nil, err
	}
	goals := make([]map[string]any, 0)
	for goalsRows.Next() {
		var id, desc, status string
		var importance, urgency, progress []byte
		if err := goalsRows.Scan(&id, &desc, &status, &importance, &urgency, &progress); err != nil {
			goalsRows.Close()
			return nil, nil, err
		}
		goals = append(goals, map[string]any{"id": id, "description": desc, "status": status, "importance": jsonNumber(importance), "urgency": jsonNumber(urgency), "progress": jsonNumber(progress)})
	}
	goalsRows.Close()
	intentionRows, err := a.DB.Pool().Query(ctx, `SELECT id,goal_id,action,status,confidence FROM public.fluctlight_intentions WHERE fluctlight_id=$1 ORDER BY created_at`, fluctlightID)
	if err != nil {
		return nil, nil, err
	}
	intentions := make([]map[string]any, 0)
	for intentionRows.Next() {
		var id, goalID, action, status string
		var confidence []byte
		if err := intentionRows.Scan(&id, &goalID, &action, &status, &confidence); err != nil {
			intentionRows.Close()
			return nil, nil, err
		}
		intentions = append(intentions, map[string]any{"id": id, "goal_id": goalID, "action": action, "status": status, "confidence": jsonNumber(confidence)})
	}
	intentionRows.Close()
	return goals, intentions, nil
}

func (a *App) readRelationships(ctx context.Context, fluctlightID string) ([]map[string]any, error) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT target_actor_id,metrics,trend,summary,revision FROM public.relationships WHERE owner_fluctlight_id=$1 ORDER BY updated_at DESC`, fluctlightID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var target, trend string
		var metrics []byte
		var summary *string
		var rev int
		if err := rows.Scan(&target, &metrics, &trend, &summary, &rev); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"target_actor_id": target, "metrics": decodeObject(metrics), "trend": trend, "summary": summary, "revision": rev})
	}
	return out, nil
}

func (a *App) readMemories(ctx context.Context, fluctlightID string) ([]map[string]any, error) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,type,content,actor_refs,conversation_id,event_refs,evidence_refs,confidence,importance,emotional_significance,visibility,status,revision,created_at FROM public.memories WHERE owner_fluctlight_id=$1 AND status='active' ORDER BY created_at DESC,id DESC LIMIT 100`, fluctlightID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id, typ, content, visibility, status string
		var actorRefs, eventRefs, evidenceRefs []byte
		var conversationID *string
		var confidence, importance, emotional float64
		var rev int
		var created time.Time
		if err := rows.Scan(&id, &typ, &content, &actorRefs, &conversationID, &eventRefs, &evidenceRefs, &confidence, &importance, &emotional, &visibility, &status, &rev, &created); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "owner_fluctlight_id": fluctlightID, "type": typ, "content": content, "actor_refs": decodeArray(actorRefs), "conversation_id": conversationID, "event_refs": decodeArray(eventRefs), "evidence_refs": decodeArray(evidenceRefs), "confidence": confidence, "importance": importance, "emotional_significance": emotional, "visibility": visibility, "status": status, "revision": rev, "created_at": created.Format(time.RFC3339Nano)})
	}
	return out, nil
}

func (a *App) readSchedule(ctx context.Context, fluctlightID string) (map[string]any, error) {
	var id string
	var localDate time.Time
	var timezone, status string
	var reschedulePolicy []byte
	var rev int
	var identity []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT identity FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&identity); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	zone := stringValue(decodeObject(identity)["timezone"])
	if zone == "" {
		zone = "Asia/Shanghai"
	}
	zone = canonicalTimezone(zone)
	location, err := time.LoadLocation(zone)
	if err != nil {
		return nil, fmt.Errorf("schedule_timezone_invalid: %w", err)
	}
	localToday := time.Now().In(location).Format("2006-01-02")
	err = a.DB.Pool().QueryRow(ctx, `SELECT id,local_date,timezone,status,revision,reschedule_policy FROM public.life_schedules WHERE fluctlight_id=$1 AND status='accepted' AND local_date=$2 ORDER BY revision DESC LIMIT 1`, fluctlightID, localToday).Scan(&id, &localDate, &timezone, &status, &rev, &reschedulePolicy)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,start_at,end_at,activity,scene,status FROM public.life_schedule_items WHERE schedule_id=$1 ORDER BY start_at`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var itemID, activity, scene, itemStatus string
		var start, end time.Time
		if err := rows.Scan(&itemID, &start, &end, &activity, &scene, &itemStatus); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{"id": itemID, "start_at": start.Format(time.RFC3339Nano), "end_at": end.Format(time.RFC3339Nano), "activity": activity, "scene": scene, "status": itemStatus})
	}
	return map[string]any{"id": id, "local_date": localDate.Format("2006-01-02"), "timezone": timezone, "revision": rev, "status": status, "reschedule_policy": decodeJSONValue(reschedulePolicy), "items": items}, nil
}

func decodeJSONValue(value []byte) any {
	var result any
	if len(value) == 0 || json.Unmarshal(value, &result) != nil {
		return nil
	}
	return result
}

func (a *App) resolveContext(ctx context.Context, fluctlightID string, schedule any) (map[string]any, error) {
	now := time.Now().UTC()
	result := map[string]any{"source": "pending", "scene": nil, "activity": nil, "location": nil, "instant": now.Format(time.RFC3339Nano)}
	var scene, activity, location *string
	var eventKind, eventStatus string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT kind,status,scene,activity,location FROM public.life_events WHERE fluctlight_id=$1 AND status IN ('confirmed','inferred') AND start_at <= $2 AND end_at > $2 AND (expires_at IS NULL OR expires_at > $2) ORDER BY CASE WHEN status='confirmed' THEN 0 ELSE 1 END,start_at DESC,id DESC LIMIT 1`, fluctlightID, now).Scan(&eventKind, &eventStatus, &scene, &activity, &location); err == nil {
		result["source"] = "event"
		if eventStatus == "inferred" {
			result["source"] = "hypothesis"
		}
		result["event_kind"] = eventKind
		result["scene"], result["activity"], result["location"] = scene, activity, location
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	} else {
		result = contextFromSchedule(result, schedule, now)
	}
	var userPresence, currentTask *string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT current_task,user_presence FROM public.life_presence_overlays WHERE fluctlight_id=$1 AND (expires_at IS NULL OR expires_at > $2) ORDER BY created_at DESC,id DESC LIMIT 1`, fluctlightID, now).Scan(&currentTask, &userPresence); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if currentTask != nil || userPresence != nil {
		result["presence"] = map[string]any{"current_task": currentTask, "user_presence": userPresence}
		result["presence_overlay"] = true
	}
	return result, nil
}

func contextFromSchedule(result map[string]any, value any, now time.Time) map[string]any {
	schedule, ok := value.(map[string]any)
	if !ok || schedule == nil {
		return result
	}
	for _, raw := range arrayValue(schedule["items"]) {
		item := mapValue(raw)
		start, e1 := time.Parse(time.RFC3339Nano, stringValue(item["start_at"]))
		end, e2 := time.Parse(time.RFC3339Nano, stringValue(item["end_at"]))
		if e1 == nil && e2 == nil && !now.Before(start) && now.Before(end) {
			result["source"] = "schedule"
			result["scene"] = item["scene"]
			result["activity"] = item["activity"]
			result["location"] = item["location"]
			break
		}
	}
	return result
}

func (a *App) readEvents(ctx context.Context, fluctlightID string) ([]map[string]any, error) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,kind,start_at,end_at,scene,activity,location,status,evidence_refs FROM public.life_events WHERE fluctlight_id=$1 ORDER BY start_at DESC LIMIT 100`, fluctlightID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id, kind, status string
		var start, end time.Time
		var scene, activity, location *string
		var refs []byte
		if err := rows.Scan(&id, &kind, &start, &end, &scene, &activity, &location, &status, &refs); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "kind": kind, "start_at": start.Format(time.RFC3339Nano), "end_at": end.Format(time.RFC3339Nano), "scene": scene, "activity": activity, "location": location, "status": status, "evidence_refs": decodeArray(refs)})
	}
	return out, nil
}

func (a *App) readCognitionHistory(ctx context.Context, fluctlightID string) ([]map[string]any, error) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,action_type,status,error_code,frozen_at,completed_at FROM public.cognition_frozen_actions WHERE fluctlight_id=$1 ORDER BY frozen_at DESC LIMIT 50`, fluctlightID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id, typ, status string
		var code *string
		var frozen time.Time
		var completed *time.Time
		if err := rows.Scan(&id, &typ, &status, &code, &frozen, &completed); err != nil {
			return nil, err
		}
		var completedValue any
		if completed != nil {
			completedValue = completed.Format(time.RFC3339Nano)
		}
		out = append(out, map[string]any{"id": id, "action_type": typ, "status": status, "error_code": code, "frozen_at": frozen.Format(time.RFC3339Nano), "completed_at": completedValue})
	}
	return out, nil
}

func (a *App) readFoundationRevisions(ctx context.Context, fluctlightID string) ([]map[string]any, error) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,revision,source,status,changes,created_at,accepted_at,reason FROM public.fluctlight_foundation_revisions WHERE fluctlight_id=$1 ORDER BY revision DESC`, fluctlightID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id, source, status string
		var rev int
		var changes []byte
		var created, accepted *time.Time
		var reason *string
		if err := rows.Scan(&id, &rev, &source, &status, &changes, &created, &accepted, &reason); err != nil {
			return nil, err
		}
		var c, a any
		if created != nil {
			c = created.Format(time.RFC3339Nano)
		}
		if accepted != nil {
			a = accepted.Format(time.RFC3339Nano)
		}
		out = append(out, map[string]any{"id": id, "revision": rev, "source": source, "status": status, "changes": decodeObject(changes), "created_at": c, "accepted_at": a, "reason": reason})
	}
	return out, nil
}

func decodeArray(value []byte) []any {
	var result []any
	if json.Unmarshal(value, &result) != nil {
		return []any{}
	}
	return result
}
func jsonNumber(value []byte) any {
	var result any
	if json.Unmarshal(value, &result) != nil {
		return 0
	}
	return result
}

func (a *App) Moments(ctx context.Context, actorID, fluctlightID string) ([]map[string]any, error) {
	return a.MomentsWithOptions(ctx, actorID, fluctlightID, false, 100)
}

func (a *App) MomentsWithOptions(ctx context.Context, actorID, fluctlightID string, includeHidden bool, limit int) ([]map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fluctlightID, actorID); err != nil {
		return nil, err
	}
	if limit < 1 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	query := `SELECT id,author_actor_id,text,visibility,status,media_asset_ids,created_at FROM public.moments WHERE owner_fluctlight_id=$1 AND visibility IN ('owner','participants')`
	if !includeHidden {
		query += ` AND status='visible'`
	}
	query += ` ORDER BY created_at DESC,id DESC LIMIT $2`
	rows, err := a.DB.Pool().Query(ctx, query, fluctlightID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id, author, text, visibility, status string
		var media []byte
		var created time.Time
		if err := rows.Scan(&id, &author, &text, &visibility, &status, &media, &created); err != nil {
			return nil, err
		}
		moment := map[string]any{"id": id, "owner_fluctlight_id": fluctlightID, "author_actor_id": author, "text": text, "visibility": visibility, "status": status, "media_asset_ids": decodeArray(media), "created_at": created.Format(time.RFC3339Nano)}
		if err := a.hydrateMoment(ctx, actorID, moment); err != nil {
			return nil, err
		}
		out = append(out, moment)
	}
	return out, rows.Err()
}

func (a *App) AutonomyActions(ctx context.Context, actorID, fluctlightID string) ([]map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fluctlightID, actorID); err != nil {
		return nil, err
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,action_type,status,workflow_id,provider_request_id,created_at,settled_at,error_code FROM public.autonomy_actions WHERE fluctlight_id=$1 ORDER BY created_at DESC LIMIT 100`, fluctlightID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id, typ, status, wf, pr string
		var created time.Time
		var settled *time.Time
		var code *string
		if err := rows.Scan(&id, &typ, &status, &wf, &pr, &created, &settled, &code); err != nil {
			return nil, err
		}
		var settledValue any
		if settled != nil {
			settledValue = settled.Format(time.RFC3339Nano)
		}
		out = append(out, map[string]any{"id": id, "fluctlight_id": fluctlightID, "action_type": typ, "status": status, "workflow_id": wf, "provider_request_id": pr, "created_at": created.Format(time.RFC3339Nano), "settled_at": settledValue, "error_code": code})
	}
	return out, rows.Err()
}

var _ = fmt.Sprintf

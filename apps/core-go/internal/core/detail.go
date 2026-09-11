package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
		"core_persona":      fluctlight.CorePersona,
		"identity":          fluctlight.Identity,
		"personality":       fluctlight.Personality,
		"behavioral_policy": fluctlight.BehavioralPolicy,
		"life_profile":      fluctlight.LifeProfile,
		"provenance":        fluctlight.Provenance,
		"status":            fluctlight.Status,
		"current_revision":  fluctlight.CurrentRevision,
	}
	detail["personality_runtime"], err = a.readPersonalityRuntime(ctx, fluctlightID, stringValue(mapValue(fluctlight.CorePersona["personality_system"])["active_profile_id"]))
	if err != nil {
		return nil, err
	}
	inner, err := a.readInnerState(ctx, fluctlightID)
	if err != nil && err != ErrNotFound {
		return nil, err
	}
	if inner == nil {
		inner = map[string]any{}
	}
	detail["inner_state"] = inner
	claims, err := a.listDevelopingSelfClaims(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	detail["developing_self"] = map[string]any{"claims": claims}
	detail["developing_self_revisions"], err = a.listDevelopingSelfRevisions(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	developingSelfRevision := 0
	for _, claim := range claims {
		if claim.Revision > developingSelfRevision {
			developingSelfRevision = claim.Revision
		}
	}
	detail["core_persona_revision"] = fluctlight.CurrentRevision
	detail["developing_self_revision"] = developingSelfRevision
	detail["current_state_revision"] = intValue(inner["revision"])
	detail["drive_slots"], err = a.readDriveSlots(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	detail["preference_slots"], err = a.readPreferenceSlots(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	detail["trigger_preferences"], err = a.readTriggerPreferences(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	detail["visual_identity"], err = a.readVisualIdentityDetail(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	goals, intentions, err := a.readAgency(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	detail["goals"] = goals
	detail["intentions"] = intentions
	detail["relationships"], err = a.readRelationships(ctx, fluctlightID, actorID)
	if err != nil {
		return nil, err
	}
	detail["memories"], err = a.readMemories(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	detail["schedule"], detail["context"], err = a.readLifeContextSnapshotAt(ctx, fluctlightID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	detail["current_state"] = map[string]any{"inner_state": inner, "context": detail["context"]}
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
	detail["wake_ups"], err = a.readWakeUpHistory(ctx, fluctlightID)
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

func (a *App) readWakeUpHistory(ctx context.Context, fluctlightID string) ([]map[string]any, error) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,cycle,trigger_type,occurred_at,internal_dynamics,attention,thought,desire,agency,action_type,action_id,result,reflection_intent_id,status FROM public.cognition_wakeups WHERE fluctlight_id=$1 ORDER BY occurred_at DESC,id DESC LIMIT 100`, fluctlightID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var id, triggerType, actionType, status string
		var cycle int
		var occurred time.Time
		var internalDynamics, attention, thought, desire, agency, wakeResult []byte
		var actionID, reflectionIntent *string
		if err := rows.Scan(&id, &cycle, &triggerType, &occurred, &internalDynamics, &attention, &thought, &desire, &agency, &actionType, &actionID, &wakeResult, &reflectionIntent, &status); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{
			"id": id, "cycle": cycle, "trigger_type": triggerType, "occurred_at": occurred.Format(time.RFC3339Nano),
			"internal_dynamics": decodeJSONValue(internalDynamics), "attention": decodeJSONValue(attention), "thought": decodeJSONValue(thought), "desire": decodeJSONValue(desire), "agency": decodeJSONValue(agency),
			"action_type": actionType, "action_id": actionID, "result": decodeObject(wakeResult), "reflection_intent_id": reflectionIntent, "status": status,
		})
	}
	return result, rows.Err()
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
	var pad, mood, momentum, regulation, drives, conflicts []byte
	var updated time.Time
	err := a.DB.Pool().QueryRow(ctx, `SELECT revision,pad,mood,momentum,regulation,drives,conflicts,last_updated_at FROM public.fluctlight_inner_states WHERE fluctlight_id=$1`, fluctlightID).Scan(&revision, &pad, &mood, &momentum, &regulation, &drives, &conflicts, &updated)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	moodValue := decodeObject(mood)
	// Older rows were initialized with a nil mood label. Keep the read model
	// truthful and usable while affect_event gradually writes explicit labels.
	if stringValue(moodValue["label"]) == "" {
		moodValue["label"] = "平静"
		if stringValue(moodValue["source"]) == "" {
			moodValue["source"] = "server_default"
		}
	}
	return map[string]any{"pad": decodeObject(pad), "mood": moodValue, "momentum": decodeObject(momentum), "regulation": decodeObject(regulation), "drives": decodeArray(drives), "conflicts": decodeArray(conflicts), "revision": revision, "last_updated_at": updated.Format(time.RFC3339Nano)}, nil
}

func (a *App) readAgency(ctx context.Context, fluctlightID string) ([]map[string]any, []map[string]any, error) {
	goalsRows, err := a.DB.Pool().Query(ctx, `SELECT id,profile_id,scope,target_actor_id,description,status,importance,urgency,progress FROM public.fluctlight_goals WHERE fluctlight_id=$1 ORDER BY created_at`, fluctlightID)
	if err != nil {
		return nil, nil, err
	}
	goals := make([]map[string]any, 0)
	for goalsRows.Next() {
		var id, scope, desc, status string
		var profileID *string
		var targetActorID *string
		var importance, urgency, progress []byte
		if err := goalsRows.Scan(&id, &profileID, &scope, &targetActorID, &desc, &status, &importance, &urgency, &progress); err != nil {
			goalsRows.Close()
			return nil, nil, err
		}
		item := map[string]any{"id": id, "scope": scope, "description": desc, "status": status, "importance": jsonNumber(importance), "urgency": jsonNumber(urgency), "progress": jsonNumber(progress)}
		if profileID != nil && strings.TrimSpace(*profileID) != "" {
			item["profile_id"] = *profileID
		}
		if targetActorID != nil {
			item["target_actor_id"] = *targetActorID
		}
		goals = append(goals, item)
	}
	goalsRows.Close()
	intentionRows, err := a.DB.Pool().Query(ctx, `SELECT id,profile_id,goal_id,action,status,confidence FROM public.fluctlight_intentions WHERE fluctlight_id=$1 ORDER BY created_at`, fluctlightID)
	if err != nil {
		return nil, nil, err
	}
	intentions := make([]map[string]any, 0)
	for intentionRows.Next() {
		var id, action, status string
		var profileID, goalID *string
		var confidence []byte
		if err := intentionRows.Scan(&id, &profileID, &goalID, &action, &status, &confidence); err != nil {
			intentionRows.Close()
			return nil, nil, err
		}
		item := map[string]any{"id": id, "action": action, "status": status, "confidence": jsonNumber(confidence)}
		if profileID != nil && strings.TrimSpace(*profileID) != "" {
			item["profile_id"] = *profileID
		}
		if goalID != nil && strings.TrimSpace(*goalID) != "" {
			item["goal_id"] = *goalID
		}
		intentions = append(intentions, item)
	}
	intentionRows.Close()
	return goals, intentions, nil
}

func (a *App) readRelationships(ctx context.Context, fluctlightID, currentHumanActorID string) ([]map[string]any, error) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT r.id,r.profile_id,r.target_actor_id,COALESCE(a.actor_type,'unknown'),(r.target_actor_id=$2),r.role,r.metrics,r.trend,r.summary,r.emotional_association,r.provenance,r.revision FROM public.relationships r LEFT JOIN public.actors a ON a.id=r.target_actor_id WHERE r.owner_fluctlight_id=$1 ORDER BY r.updated_at DESC`, fluctlightID, currentHumanActorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var profileID *string
		var id, target, actorType, trend string
		var isCurrentUser bool
		var role, metrics, emotional, provenance []byte
		var summary *string
		var rev int
		if err := rows.Scan(&id, &profileID, &target, &actorType, &isCurrentUser, &role, &metrics, &trend, &summary, &emotional, &provenance, &rev); err != nil {
			return nil, err
		}
		item := map[string]any{"id": id, "target_actor_id": target, "target_actor_type": actorType, "is_current_user": isCurrentUser, "role": decodeObject(role), "metrics": decodeObject(metrics), "trend": trend, "summary": summary, "emotional_association": decodeObject(emotional), "provenance": decodeObject(provenance), "revision": rev}
		if profileID != nil && strings.TrimSpace(*profileID) != "" {
			item["profile_id"] = *profileID
		}
		out = append(out, item)
	}
	return out, nil
}

func (a *App) readMemories(ctx context.Context, fluctlightID string) ([]map[string]any, error) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,type,content,actor_refs,conversation_id,event_refs,evidence_refs,personality_perspectives,confidence,importance,emotional_significance,visibility,status,revision,created_at FROM public.memories WHERE owner_fluctlight_id=$1 AND status='active' ORDER BY created_at DESC,id DESC LIMIT 100`, fluctlightID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id, typ, content, visibility, status string
		var actorRefs, eventRefs, evidenceRefs, perspectives []byte
		var conversationID *string
		var confidence, importance, emotional float64
		var rev int
		var created time.Time
		if err := rows.Scan(&id, &typ, &content, &actorRefs, &conversationID, &eventRefs, &evidenceRefs, &perspectives, &confidence, &importance, &emotional, &visibility, &status, &rev, &created); err != nil {
			return nil, err
		}
		item := map[string]any{"id": id, "owner_fluctlight_id": fluctlightID, "type": typ, "content": content, "actor_refs": decodeArray(actorRefs), "conversation_id": conversationID, "event_refs": decodeArray(eventRefs), "evidence_refs": decodeArray(evidenceRefs), "confidence": confidence, "importance": importance, "emotional_significance": emotional, "visibility": visibility, "status": status, "revision": rev, "created_at": created.Format(time.RFC3339Nano)}
		if values := decodeArray(perspectives); len(values) > 0 {
			item["personality_perspectives"] = values
		}
		out = append(out, item)
	}
	return out, nil
}

func (a *App) readSchedule(ctx context.Context, fluctlightID string) (map[string]any, error) {
	schedule, _, err := a.readLifeContextSnapshotAt(ctx, fluctlightID, time.Now().UTC())
	return schedule, err
}

func scheduleContextNumber(value string) any {
	if parsed, ok := numberFloat(value); ok {
		return parsed
	}
	return normalizeScheduleScalar(value)
}

func decodeJSONValue(value []byte) any {
	var result any
	if len(value) == 0 || json.Unmarshal(value, &result) != nil {
		return nil
	}
	return result
}

func (a *App) resolveContext(ctx context.Context, fluctlightID string, schedule any) (map[string]any, error) {
	at := time.Now().UTC()
	timezone, err := readLifeContextTimezoneWith(ctx, a.DB.Pool(), fluctlightID)
	if err != nil {
		return nil, err
	}
	value, _ := schedule.(map[string]any)
	return resolveLifeContextAtWith(ctx, a.DB.Pool(), fluctlightID, value, timezone, at)
}

func contextFromSchedule(result map[string]any, value any, now time.Time) map[string]any {
	schedule, _ := value.(map[string]any)
	contextFromScheduleAt(result, schedule, now)
	return result
}

func (a *App) readEvents(ctx context.Context, fluctlightID string) ([]map[string]any, error) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,kind,start_at,end_at,scene,activity,location,status,revision,evidence_refs FROM public.life_events WHERE fluctlight_id=$1 ORDER BY start_at DESC LIMIT 100`, fluctlightID)
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
		var revision int
		if err := rows.Scan(&id, &kind, &start, &end, &scene, &activity, &location, &status, &revision, &refs); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "kind": kind, "start_at": start.Format(time.RFC3339Nano), "end_at": end.Format(time.RFC3339Nano), "scene": scene, "activity": activity, "location": location, "status": status, "revision": revision, "evidence_refs": decodeArray(refs)})
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
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,revision,source,status,changes,core_persona,created_at,accepted_at,reason FROM public.fluctlight_foundation_revisions WHERE fluctlight_id=$1 ORDER BY revision DESC`, fluctlightID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id, source, status string
		var rev int
		var changes, corePersona []byte
		var created, accepted *time.Time
		var reason *string
		if err := rows.Scan(&id, &rev, &source, &status, &changes, &corePersona, &created, &accepted, &reason); err != nil {
			return nil, err
		}
		var c, a any
		if created != nil {
			c = created.Format(time.RFC3339Nano)
		}
		if accepted != nil {
			a = accepted.Format(time.RFC3339Nano)
		}
		out = append(out, map[string]any{"id": id, "revision": rev, "source": source, "status": status, "changes": decodeObject(changes), "core_persona": decodeObject(corePersona), "created_at": c, "accepted_at": a, "reason": reason})
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

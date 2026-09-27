package core

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	lifeActivityStartCapabilityName   = "life.activity.start"
	lifeActivityAdvanceCapabilityName = "life.activity.advance"
)

type lifeActivityService struct{ app *App }
type lifeActivityStartCapability struct{ service *lifeActivityService }
type lifeActivityAdvanceCapability struct{ service *lifeActivityService }

func lifeActivityOutputSchema() map[string]any {
	return objectSchema(map[string]any{
		"activity_id":       map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
		"status":            map[string]any{"type": "string", "minLength": 1, "maxLength": 32},
		"not_before":        map[string]any{"type": "string"},
		"active_until":      map[string]any{"type": "string"},
		"event_id":          map[string]any{"type": "string"},
		"item_id":           map[string]any{"type": "string"},
		"body_revision":     map[string]any{"type": "integer"},
		"wardrobe_revision": map[string]any{"type": "integer"},
		"intention_id":      map[string]any{"type": "string"},
		"reason":            map[string]any{"type": "string"},
	}, []string{"activity_id", "status"}, false)
}

func lifeActivityStartDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: lifeActivityStartCapabilityName, Version: "v1", Type: CapabilityTypeAction,
		Description:   "Start one timed virtual activity and its current scene Event. Supply scene, activity and location for a change of place. Shopping needs category, slot and description; haircut needs desired_hair_length; hair_dye needs desired_hair_color and a due intention/schedule item. Accepted means started, not purchased.",
		Surfaces:      []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceNativeCognition},
		FailurePolicy: FailurePolicyOptionalInternal,
		InputSchema: map[string]any{
			"type": "object", "additionalProperties": false, "required": []any{"kind", "reason"},
			"properties": map[string]any{
				"kind":                enumStringSchema("virtual_shopping", "haircut", "hair_dye"),
				"intention_id":        map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
				"schedule_item_id":    map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
				"duration_minutes":    map[string]any{"type": "integer", "minimum": 15, "maximum": 240},
				"category":            map[string]any{"type": "string", "minLength": 1, "maxLength": 64},
				"slot":                map[string]any{"type": "string", "minLength": 1, "maxLength": 64},
				"description":         map[string]any{"type": "string", "minLength": 1, "maxLength": 512},
				"scene":               map[string]any{"type": "string", "minLength": 1, "maxLength": 512},
				"activity":            map[string]any{"type": "string", "minLength": 1, "maxLength": 512},
				"location":            map[string]any{"type": "string", "minLength": 1, "maxLength": 512},
				"desired_hair_length": map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
				"desired_hair_color":  map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
				"reason":              map[string]any{"type": "string", "minLength": 1, "maxLength": 500},
			},
		},
		OutputSchema: lifeActivityOutputSchema(), SideEffectClass: "native_projection", SuccessBoundary: "virtual_activity_started",
		CompletionBoundary: "virtual_activity_resolved", OutcomeReferenceField: "activity_id",
		ConcurrencyClass: "exclusive", SupportsRetry: true,
	}
}
func (c lifeActivityStartCapability) Definition() CapabilityDefinition {
	return lifeActivityStartDefinition()
}
func (c lifeActivityStartCapability) RequiredContext() []ContextSlot { return nil }
func (c lifeActivityStartCapability) Execute(_ context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return executeToolRequired(invocation)
}
func (c lifeActivityStartCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	if c.service == nil || c.service.app == nil || c.service.app.DB == nil {
		return failedCapabilityResult(invocation, "activity_unavailable", true), errors.New("activity service unavailable")
	}
	args, err := capabilityExecutionArguments(invocation, lifeActivityStartDefinition())
	if err != nil {
		return failedCapabilityResult(invocation, "invalid_arguments", false), err
	}
	kind := stringValue(args["kind"])
	request := map[string]any{"kind": kind}
	switch kind {
	case "virtual_shopping":
		for _, key := range []string{"category", "slot", "description"} {
			text := strings.TrimSpace(stringValue(args[key]))
			if text == "" {
				return failedCapabilityResult(invocation, "shopping_item_required", false), ErrInvalidArguments
			}
			request[key] = text
		}
	case "haircut":
		length := strings.TrimSpace(stringValue(args["desired_hair_length"]))
		if length == "" {
			return failedCapabilityResult(invocation, "haircut_length_required", false), ErrInvalidArguments
		}
		request["desired_hair_length"] = length
		if color := strings.TrimSpace(stringValue(args["desired_hair_color"])); color != "" {
			request["desired_hair_color"] = color
		}
	case "hair_dye":
		color := strings.TrimSpace(stringValue(args["desired_hair_color"]))
		if color == "" || strings.TrimSpace(stringValue(args["intention_id"])) == "" {
			return failedCapabilityResult(invocation, "hair_dye_plan_required", false), ErrInvalidArguments
		}
		if strings.TrimSpace(stringValue(args["desired_hair_length"])) != "" {
			return failedCapabilityResult(invocation, "hair_dye_length_forbidden", false), ErrInvalidArguments
		}
		request["desired_hair_color"] = color
	default:
		return failedCapabilityResult(invocation, "activity_kind_invalid", false), ErrInvalidArguments
	}
	fluctlightID := invocation.Metadata.FluctlightID
	profileID, err := (&intentionService{}).resolveProfile(ctx, tx, fluctlightID, invocation.Metadata.WorkingProfileID)
	if err != nil {
		return failedCapabilityResult(invocation, "activity_profile_unavailable", true), err
	}
	duration := intValue(args["duration_minutes"])
	if duration == 0 {
		duration = 60
	}
	if duration < 15 || duration > 240 {
		return failedCapabilityResult(invocation, "activity_duration_invalid", false), ErrInvalidArguments
	}
	activityID := "activity_" + stableDigest(fluctlightID+"\x1f"+capabilityOperationID(invocation))
	now := time.Now().UTC()
	notBefore := now.Add(time.Duration(duration) * time.Minute)
	scheduleItemID := strings.TrimSpace(stringValue(args["schedule_item_id"]))
	if err := lockLifeContextTx(ctx, tx, fluctlightID); err != nil {
		return failedCapabilityResult(invocation, "activity_context_lock_failed", true), err
	}
	schedule, life, err := resolveLifeContextSnapshotWith(ctx, tx, fluctlightID, now)
	if err != nil {
		return failedCapabilityResult(invocation, "activity_context_read_failed", true), err
	}
	activeUntil := notBefore.Add(30 * time.Minute)
	if boundary, parseErr := time.Parse(time.RFC3339Nano, stringValue(schedule["current_item_expires_at"])); parseErr == nil && boundary.Before(activeUntil) {
		activeUntil = boundary
	}
	if scheduleItemID == "" && stringValue(life["source"]) == "event" {
		if boundary, parseErr := time.Parse(time.RFC3339Nano, stringValue(life["expires_at"])); parseErr == nil && boundary.Before(activeUntil) {
			activeUntil = boundary
		}
	}
	if !activeUntil.After(now) {
		return failedCapabilityResult(invocation, "activity_window_ended", false), ErrConflict
	}
	scene := firstString(args["scene"], stringValue(life["scene"]))
	activity := firstString(args["activity"], kind)
	location := firstString(args["location"], stringValue(life["location"]))
	for key, value := range map[string]string{"scene": scene, "activity": activity, "location": location} {
		if value != "" {
			request[key] = value
		}
	}
	intentionID := strings.TrimSpace(stringValue(args["intention_id"]))
	if kind == "hair_dye" && scheduleItemID == "" {
		return failedCapabilityResult(invocation, "hair_dye_schedule_required", false), ErrInvalidArguments
	}
	if scheduleItemID != "" {
		var plannedIntention, scheduledKind, itemActivity, itemScene string
		var itemLocation *string
		var itemEnd time.Time
		var actionPlanRaw []byte
		if err := tx.QueryRow(ctx, `SELECT COALESCE(item.intention_id,''),item.action_plan->>'kind',item.action_plan,item.activity,item.scene,item.location,item.end_at FROM public.life_schedule_items item JOIN public.life_schedules s ON s.id=item.schedule_id WHERE item.id=$1 AND s.fluctlight_id=$2 AND s.status='accepted' AND item.start_at<=now() AND item.end_at>now() FOR UPDATE OF s,item`, scheduleItemID, fluctlightID).Scan(&plannedIntention, &scheduledKind, &actionPlanRaw, &itemActivity, &itemScene, &itemLocation, &itemEnd); err != nil {
			return failedCapabilityResult(invocation, "scheduled_activity_not_due", false), ErrConflict
		}
		if plannedIntention == "" || plannedIntention != intentionID || scheduledKind != kind || !scheduledActionMatchesArguments(decodeObject(actionPlanRaw), args) {
			return failedCapabilityResult(invocation, "scheduled_activity_plan_mismatch", false), ErrConflict
		}
		if stringValue(args["activity"]) == "" {
			activity, request["activity"] = itemActivity, itemActivity
		}
		if stringValue(args["scene"]) == "" {
			scene, request["scene"] = itemScene, itemScene
		}
		if stringValue(args["location"]) == "" {
			location = ""
			if itemLocation != nil {
				location = *itemLocation
			}
			request["location"] = location
		}
		if itemEnd.Before(activeUntil) {
			activeUntil = itemEnd
		}
		request["schedule_item_id"] = scheduleItemID
	}
	var dueContext map[string]any
	if intentionID != "" && invocation.Metadata.Surface == CapabilitySurfaceNativeCognition && invocation.Metadata.Source == "model_tool" && invocation.SourceFactID != "" {
		var eventType string
		var inboxPayload []byte
		readErr := tx.QueryRow(ctx, `SELECT event_type,payload FROM public.cognition_inbox WHERE id=$1 AND fluctlight_id=$2`, invocation.SourceFactID, fluctlightID).Scan(&eventType, &inboxPayload)
		if readErr != nil && !errors.Is(readErr, pgx.ErrNoRows) {
			return failedCapabilityResult(invocation, "activity_due_context_read_failed", true), readErr
		}
		if readErr == nil && eventType == intentionDueFactType {
			dueContext = mapValue(decodeObject(inboxPayload)["due_context"])
			candidate := mapValue(decodeObject(inboxPayload)["candidate"])
			if stringValue(dueContext["intention_id"]) != intentionID || stringValue(dueContext["attempt_id"]) != stringValue(candidate["attempt_id"]) || intValue(dueContext["intention_revision"]) != intValue(candidate["intention_revision"]) {
				return failedCapabilityResult(invocation, "activity_due_context_invalid", false), ErrConflict
			}
		}
	}
	if intentionID != "" {
		var expiration time.Time
		var constraintRaw []byte
		var intentionStatus string
		var triggerRaw []byte
		if err := tx.QueryRow(ctx, `SELECT expiration,capability_constraints,status,trigger FROM public.fluctlight_intentions WHERE id=$1 AND fluctlight_id=$2 AND COALESCE(profile_id,'')=$3 FOR UPDATE`, intentionID, fluctlightID, profileID).Scan(&expiration, &constraintRaw, &intentionStatus, &triggerRaw); err != nil {
			return failedCapabilityResult(invocation, "activity_intention_not_found", false), err
		}
		if !now.Before(expiration) {
			return failedCapabilityResult(invocation, "activity_intention_expired", false), ErrConflict
		}
		if kind == "hair_dye" {
			trigger := decodeObject(triggerRaw)
			dueAt, parseErr := time.Parse(time.RFC3339Nano, stringValue(trigger["due_at"]))
			if (intentionStatus != string(IntentionQualified) && intentionStatus != string(IntentionDue)) || stringValue(trigger["type"]) != string(IntentionTriggerTime) || parseErr != nil || now.Before(dueAt) {
				return failedCapabilityResult(invocation, "hair_dye_not_due", false), ErrConflict
			}
		}
		if dueContext != nil {
			var status, currentAttempt string
			var revision int
			if err := tx.QueryRow(ctx, `SELECT status,COALESCE(current_attempt_id,''),revision FROM public.fluctlight_intentions WHERE id=$1`, intentionID).Scan(&status, &currentAttempt, &revision); err != nil {
				return failedCapabilityResult(invocation, "activity_due_intention_read_failed", true), err
			}
			if status != string(IntentionDue) || currentAttempt != stringValue(dueContext["attempt_id"]) || revision != intValue(dueContext["intention_revision"]) {
				return failedCapabilityResult(invocation, "activity_due_intention_stale", false), ErrConflict
			}
		}
		constraints := decisionServiceRefValues(decodeArray(constraintRaw))
		if len(constraints) > 0 && !containsString(constraints, lifeActivityStartCapabilityName) {
			return failedCapabilityResult(invocation, "activity_intention_capability_forbidden", false), ErrUnauthorized
		}
		_, err := transitionLinkedIntentionTx(ctx, tx, fluctlightID, profileID, intentionID, IntentionStart,
			"activity-start:"+activityID, strings.TrimSpace(stringValue(args["reason"])), "activity-start:"+activityID, now)
		if err != nil {
			return failedCapabilityResultDetail(invocation, "activity_intention_not_ready", false, err.Error()), err
		}
	}
	result := map[string]any{"request": request, "reason": strings.TrimSpace(stringValue(args["reason"]))}
	eventID := "event_" + stableDigest(activityID+"\x1fauthority")
	if priorID := stringValue(life["event_id"]); priorID != "" {
		if _, err := tx.Exec(ctx, `UPDATE public.life_events SET end_at=LEAST(end_at,$3),expires_at=$3,revision=revision+1,updated_at=$3 WHERE id=$1 AND fluctlight_id=$2 AND status IN ('confirmed','inferred') AND end_at>$3`, priorID, fluctlightID, now); err != nil {
			return failedCapabilityResult(invocation, "activity_prior_event_end_failed", true), err
		}
		if err := c.service.app.closeActivityRunsForEventTx(ctx, tx, fluctlightID, priorID, now); err != nil {
			return failedCapabilityResult(invocation, "activity_prior_run_end_failed", true), err
		}
	}
	eventResult := map[string]any{"id": eventID, "activity_id": activityID, "status": "confirmed", "revision": 1,
		"expected_context_revision": life["context_revision"], "resulting_context_revision": life["context_revision"], "replayed": false}
	evidenceRefs := []any{"activity:" + activityID}
	if invocation.SourceFactID != "" {
		evidenceRefs = append(evidenceRefs, invocation.SourceFactID)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.life_events(id,fluctlight_id,kind,start_at,end_at,scene,activity,location,status,revision,evidence_refs,idempotency_key,request_digest,result,expires_at) VALUES($1,$2,'life_activity',$3,$4,$5,$6,$7,'confirmed',1,$8,$9,$10,$11,$4)`, eventID, fluctlightID, now, activeUntil, nullableString(scene), activity, nullableString(location), jsonBytes(evidenceRefs), "activity-authority:"+activityID, stableDigest(jsonString(result)), jsonBytes(eventResult)); err != nil {
		return failedCapabilityResult(invocation, "activity_authority_event_failed", true), err
	}
	_, resultingLife, err := resolveLifeContextSnapshotWith(ctx, tx, fluctlightID, now)
	if err != nil {
		return failedCapabilityResult(invocation, "activity_authority_context_failed", true), err
	}
	eventResult["resulting_context_revision"] = resultingLife["context_revision"]
	if _, err := tx.Exec(ctx, `UPDATE public.life_events SET result=$2 WHERE id=$1`, eventID, jsonBytes(eventResult)); err != nil {
		return failedCapabilityResult(invocation, "activity_authority_result_failed", true), err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_life_activity_runs(id,fluctlight_id,profile_id,kind,status,intention_id,scheduled_at,started_at,not_before,result_json,revision,operation_id,authority_event_id,active_until) VALUES($1,$2,$3,$4,'in_progress',$5,$6,$6,$7,$8,1,$9,$10,$11)`, activityID, fluctlightID, profileID, kind, nullableString(intentionID), now, notBefore, jsonBytes(result), capabilityOperationID(invocation), eventID, activeUntil); err != nil {
		return failedCapabilityResult(invocation, "activity_start_failed", true), err
	}
	if err := appendOutboxTx(ctx, tx, "life.activity.started", "life_activity", activityID, fluctlightID, invocation.SourceFactID,
		"activity:"+activityID, "activity-start:"+activityID, map[string]any{"activity_id": activityID, "kind": kind, "status": "in_progress", "not_before": notBefore.Format(time.RFC3339Nano)}); err != nil {
		return failedCapabilityResult(invocation, "activity_event_failed", true), err
	}
	receiptResult := CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "accepted",
		Output:            map[string]any{"activity_id": activityID, "status": "in_progress", "not_before": notBefore.Format(time.RFC3339Nano), "intention_id": intentionID, "event_id": eventID},
		ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "activity:" + activityID}
	if dueContext != nil {
		goalRef, intentionRef := stringValue(dueContext["goal_ref"]), stringValue(dueContext["intention_ref"])
		contextRefs, err := actionOutcomeContextReferences(dueContext["context_references"])
		if err != nil || contextRefs[goalRef].Kind != ContextReferenceGoal || contextRefs[intentionRef].Kind != ContextReferenceIntention || contextRefs[intentionRef].EntityID != intentionID || contextRefs[intentionRef].Revision != intValue(dueContext["intention_revision"]) || stringValue(decodeObject(contextRefs[intentionRef].Snapshot)["current_attempt_id"]) != stringValue(dueContext["attempt_id"]) {
			return failedCapabilityResult(invocation, "activity_due_references_invalid", false), ErrConflict
		}
		settlement := map[string]any{"status": "pending", "goal_refs": []string{goalRef}, "intention_refs": []string{intentionRef}, "context_references": contextRefs}
		outcomes, err := buildActionOutcomes("agent_native_"+stableDigest(invocation.SourceFactID), fluctlightID, invocation.SourceFactID, "no_op", []CapabilityResult{receiptResult}, settlement, c.service.app.capabilityRegistry())
		if err != nil {
			return failedCapabilityResult(invocation, "activity_due_outcome_invalid", true), err
		}
		if err := persistActionOutcomesTx(ctx, tx, outcomes); err != nil {
			return failedCapabilityResult(invocation, "activity_due_outcome_failed", true), err
		}
	}
	return receiptResult, nil
}

func lifeActivityAdvanceDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: lifeActivityAdvanceCapabilityName, Version: "v1", Type: CapabilityTypeAction,
		Description:   "Advance one already started virtual activity. Omit activity_id only when exactly one activity is current. Supply extend_minutes with a reason only for an explicit decision to continue past its current boundary; otherwise resolve the elapsed result. Ending an activity does not imply a purchase or body change.",
		Surfaces:      []CapabilitySurface{CapabilitySurfaceWakeUp, CapabilitySurfaceNativeCognition, CapabilitySurfaceConversation},
		FailurePolicy: FailurePolicyOptionalInternal,
		InputSchema:   objectSchema(map[string]any{"activity_id": map[string]any{"type": "string", "minLength": 1, "maxLength": 128}, "extend_minutes": map[string]any{"type": "integer", "minimum": 15, "maximum": 240}, "reason": map[string]any{"type": "string", "minLength": 1, "maxLength": 500}}, nil, false),
		OutputSchema:  lifeActivityOutputSchema(), SideEffectClass: "native_projection", SuccessBoundary: "virtual_activity_resolution_committed",
		ConcurrencyClass: "exclusive", SupportsRetry: true,
	}
}
func (c lifeActivityAdvanceCapability) Definition() CapabilityDefinition {
	return lifeActivityAdvanceDefinition()
}
func (c lifeActivityAdvanceCapability) RequiredContext() []ContextSlot { return nil }
func (c lifeActivityAdvanceCapability) Execute(_ context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return executeToolRequired(invocation)
}
func (c lifeActivityAdvanceCapability) Prepare(ctx context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityInvocation, error) {
	if c.service == nil || c.service.app == nil || c.service.app.DB == nil {
		return invocation, errors.New("activity service unavailable")
	}
	args, err := capabilityExecutionArguments(invocation, lifeActivityAdvanceDefinition())
	if err != nil {
		return invocation, err
	}
	app := c.service.app
	activityID := strings.TrimSpace(stringValue(args["activity_id"]))
	fluctlightID := invocation.Metadata.FluctlightID
	currentProfileID, profileErr := (&intentionService{}).resolveProfile(ctx, app.DB.Pool(), fluctlightID, invocation.Metadata.WorkingProfileID)
	if profileErr != nil {
		return invocation, profileErr
	}
	if activityID == "" {
		rows, queryErr := app.DB.Pool().Query(ctx, `SELECT r.id FROM public.fluctlight_life_activity_runs r WHERE r.fluctlight_id=$1 AND r.profile_id=$2 AND r.status IN ('in_progress','deferred') AND r.active_until>now() AND (r.authority_event_id IS NULL OR EXISTS(SELECT 1 FROM public.life_events e WHERE e.id=r.authority_event_id AND e.status IN ('confirmed','inferred') AND e.start_at<=now() AND e.end_at>now() AND (e.expires_at IS NULL OR e.expires_at>now()))) ORDER BY r.started_at DESC,r.id LIMIT 2`, fluctlightID, currentProfileID)
		if queryErr != nil {
			return invocation, queryErr
		}
		candidates := make([]string, 0, 2)
		for rows.Next() {
			var candidate string
			if err := rows.Scan(&candidate); err != nil {
				rows.Close()
				return invocation, err
			}
			candidates = append(candidates, candidate)
		}
		queryErr = rows.Err()
		rows.Close()
		if queryErr != nil {
			return invocation, queryErr
		}
		if len(candidates) == 0 {
			return invocation, newCapabilityError("activity_not_found", false, ErrNotFound)
		}
		if len(candidates) != 1 {
			return invocation, newCapabilityError("activity_selection_required", false, errors.New("multiple active activities; specify activity_id from current context"))
		}
		activityID = candidates[0]
	}
	var profileID, kind, status, intentionID string
	var startedAt, notBefore time.Time
	var revision int
	var resultRaw []byte
	err = app.DB.Pool().QueryRow(ctx, `SELECT profile_id,kind,status,COALESCE(intention_id,''),started_at,not_before,revision,result_json FROM public.fluctlight_life_activity_runs WHERE id=$1 AND fluctlight_id=$2`, activityID, fluctlightID).Scan(&profileID, &kind, &status, &intentionID, &startedAt, &notBefore, &revision, &resultRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return invocation, newCapabilityError("activity_not_found", false, ErrNotFound)
	}
	if err != nil {
		return invocation, err
	}
	if profileID != currentProfileID {
		return invocation, newCapabilityError("activity_not_found", false, ErrNotFound)
	}
	plan := map[string]any{"activity_id": activityID, "profile_id": profileID, "kind": kind, "status": status,
		"revision": revision, "not_before": notBefore.UTC().Format(time.RFC3339Nano)}
	if status != "in_progress" && status != "deferred" {
		return withCapabilityPreparedData(invocation, "activity_advance", plan)
	}
	authorityActive, err := activityAuthorityActiveWith(ctx, app.DB.Pool(), activityID, fluctlightID, time.Now().UTC())
	if err != nil {
		return invocation, err
	}
	if !authorityActive {
		plan["result"] = map[string]any{"status": "authority_ended"}
		return withCapabilityPreparedData(invocation, "activity_advance", plan)
	}
	if extendMinutes := intValue(args["extend_minutes"]); extendMinutes > 0 {
		if extendMinutes < 15 || extendMinutes > 240 || strings.TrimSpace(stringValue(args["reason"])) == "" {
			return invocation, newCapabilityError("activity_extension_invalid", false, ErrInvalidArguments)
		}
		plan["result"] = map[string]any{"status": "extended", "extend_minutes": extendMinutes, "reason": strings.TrimSpace(stringValue(args["reason"]))}
		return withCapabilityPreparedData(invocation, "activity_advance", plan)
	}
	if scheduleItemID := stringValue(mapValue(decodeObject(resultRaw)["request"])["schedule_item_id"]); intentionID != "" && scheduleItemID != "" {
		var accepted bool
		if err := app.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.life_schedule_items item JOIN public.life_schedules s ON s.id=item.schedule_id AND s.fluctlight_id=$1 AND s.status='accepted' WHERE item.id=$2 AND item.intention_id=$3)`, fluctlightID, scheduleItemID, intentionID).Scan(&accepted); err != nil {
			return invocation, err
		}
		var intentionStatus string
		var expiration time.Time
		if err := app.DB.Pool().QueryRow(ctx, `SELECT status,expiration FROM public.fluctlight_intentions WHERE id=$1 AND fluctlight_id=$2`, intentionID, fluctlightID).Scan(&intentionStatus, &expiration); err != nil {
			return invocation, err
		}
		if !accepted || intentionStatus != string(IntentionInProgress) || !time.Now().UTC().Before(expiration) {
			plan["result"] = map[string]any{"status": "cancelled"}
			return withCapabilityPreparedData(invocation, "activity_advance", plan)
		}
	}
	if time.Now().UTC().Before(notBefore) {
		plan["result"] = map[string]any{"status": "not_due"}
		return withCapabilityPreparedData(invocation, "activity_advance", plan)
	}
	request := mapValue(decodeObject(resultRaw)["request"])
	appearance, _, _, err := app.readEffectiveLifeSnapshot(ctx, fluctlightID, time.Now().UTC())
	if err != nil {
		return invocation, err
	}
	_, life, err := app.readLifeContextSnapshotAt(ctx, fluctlightID, time.Now().UTC())
	if err != nil {
		return invocation, err
	}
	outcomes, err := app.readRecentActionOutcomes(ctx, fluctlightID, 6)
	if err != nil {
		return invocation, err
	}
	result, err := app.RunVirtualActivityResultTask(ctx, VirtualActivityResultTaskInput{
		Kind: kind, Request: request, StartedAt: startedAt.UTC().Format(time.RFC3339Nano), NotBefore: notBefore.UTC().Format(time.RFC3339Nano),
		CurrentAppearance: appearance, CurrentLife: life, RecentOutcomes: outcomes,
	})
	if err != nil {
		return invocation, err
	}
	if err := validateVirtualActivityResult(kind, request, result); err != nil {
		return invocation, err
	}
	plan["result"] = result
	plan["request"] = request
	return withCapabilityPreparedData(invocation, "activity_advance", plan)
}

func validateVirtualActivityResult(kind string, request, result map[string]any) error {
	status := stringValue(result["status"])
	reason := strings.TrimSpace(stringValue(result["reason"]))
	if (status != "completed" && status != "failed" && status != "deferred") || reason == "" || len([]rune(reason)) > 500 {
		return errors.New("virtual_activity_result_status_invalid")
	}
	if status != "completed" {
		if len(mapValue(result["acquired_item"])) > 0 || stringValue(result["hair_length"]) != "" || stringValue(result["hair_color"]) != "" || stringValue(result["hair_style"]) != "" {
			return errors.New("virtual_activity_uncompleted_result_has_effect")
		}
		return nil
	}
	switch kind {
	case "virtual_shopping":
		item := mapValue(result["acquired_item"])
		if len(item) > 0 && (stringValue(item["category"]) != stringValue(request["category"]) || stringValue(item["slot"]) != stringValue(request["slot"]) || strings.TrimSpace(stringValue(item["description"])) == "" || len([]rune(stringValue(item["description"]))) > 512) {
			return errors.New("virtual_shopping_item_mismatch")
		}
		if stringValue(result["hair_length"]) != "" || stringValue(result["hair_color"]) != "" {
			return errors.New("virtual_shopping_body_effect_invalid")
		}
	case "haircut":
		if len(mapValue(result["acquired_item"])) > 0 || strings.TrimSpace(stringValue(result["hair_length"])) == "" || len([]rune(stringValue(result["hair_length"]))) > 128 {
			return errors.New("virtual_haircut_result_invalid")
		}
	case "hair_dye":
		color := strings.TrimSpace(stringValue(result["hair_color"]))
		if len(mapValue(result["acquired_item"])) > 0 || stringValue(result["hair_length"]) != "" || color == "" || color != strings.TrimSpace(stringValue(request["desired_hair_color"])) {
			return errors.New("virtual_hair_dye_color_mismatch")
		}
	default:
		return errors.New("virtual_activity_kind_invalid")
	}
	return nil
}

func (c lifeActivityAdvanceCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	if c.service == nil || c.service.app == nil || c.service.app.DB == nil {
		return failedCapabilityResult(invocation, "activity_unavailable", true), errors.New("activity service unavailable")
	}
	args, err := capabilityExecutionArguments(invocation, lifeActivityAdvanceDefinition())
	if err != nil {
		return failedCapabilityResult(invocation, "invalid_arguments", false), err
	}
	raw, found, err := capabilityPreparedData(invocation, "activity_advance")
	if err != nil || !found {
		return failedCapabilityResult(invocation, "activity_plan_missing", false), errors.New("activity plan missing")
	}
	plan := mapValue(raw)
	activityID := firstString(args["activity_id"], stringValue(plan["activity_id"]))
	if activityID != stringValue(plan["activity_id"]) {
		return failedCapabilityResult(invocation, "activity_plan_identity_mismatch", false), ErrInvalidArguments
	}
	fluctlightID := invocation.Metadata.FluctlightID
	var profileID, kind, status, intentionID string
	var resultRaw []byte
	var notBefore time.Time
	var revision int
	err = tx.QueryRow(ctx, `SELECT profile_id,kind,status,COALESCE(intention_id,''),not_before,revision,result_json FROM public.fluctlight_life_activity_runs WHERE id=$1 AND fluctlight_id=$2 FOR UPDATE`, activityID, fluctlightID).Scan(&profileID, &kind, &status, &intentionID, &notBefore, &revision, &resultRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return failedCapabilityResult(invocation, "activity_not_found", false), ErrNotFound
	}
	if err != nil {
		return failedCapabilityResult(invocation, "activity_read_failed", true), err
	}
	if profileID != stringValue(plan["profile_id"]) {
		return failedCapabilityResult(invocation, "activity_profile_changed", true), ErrConflict
	}
	if revision != intValue(plan["revision"]) || kind != stringValue(plan["kind"]) || status != stringValue(plan["status"]) {
		return failedCapabilityResult(invocation, "activity_revision_conflict", true), ErrConflict
	}
	if status != "in_progress" && status != "deferred" {
		return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "rejected", ErrorCode: "activity_already_resolved",
			Output: map[string]any{"activity_id": activityID, "status": status}, ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "activity:" + activityID}, nil
	}
	authorityActive, err := activityAuthorityActiveWith(ctx, tx, activityID, fluctlightID, time.Now().UTC())
	if err != nil {
		return failedCapabilityResult(invocation, "activity_authority_read_failed", true), err
	}
	if !authorityActive {
		return c.service.app.cancelActivityForEndedEventTx(ctx, tx, invocation, activityID, fluctlightID, intentionID, revision, resultRaw)
	}
	if scheduleItemID := stringValue(mapValue(decodeObject(resultRaw)["request"])["schedule_item_id"]); intentionID != "" && scheduleItemID != "" {
		var acceptedItemID string
		readErr := tx.QueryRow(ctx, `SELECT item.id FROM public.life_schedule_items item JOIN public.life_schedules s ON s.id=item.schedule_id AND s.fluctlight_id=$1 AND s.status='accepted' WHERE item.id=$2 AND item.intention_id=$3 FOR UPDATE OF s,item`, fluctlightID, scheduleItemID, intentionID).Scan(&acceptedItemID)
		if readErr != nil && !errors.Is(readErr, pgx.ErrNoRows) {
			return failedCapabilityResult(invocation, "activity_schedule_read_failed", true), readErr
		}
		if errors.Is(readErr, pgx.ErrNoRows) {
			if err := cancelScheduledIntentionTx(ctx, tx, fluctlightID, intentionID, "schedule-item:"+scheduleItemID,
				"stale-scheduled-run:"+activityID, time.Now().UTC()); err != nil {
				return failedCapabilityResult(invocation, "activity_intention_cancel_failed", true), err
			}
			return cancelUnsettledScheduledLifeActivityTx(ctx, tx, invocation, activityID, fluctlightID, intentionID, revision, resultRaw)
		}
		var intentionStatus string
		var expiration time.Time
		if err := tx.QueryRow(ctx, `SELECT status,expiration FROM public.fluctlight_intentions WHERE id=$1 AND fluctlight_id=$2 FOR UPDATE`, intentionID, fluctlightID).Scan(&intentionStatus, &expiration); err != nil {
			return failedCapabilityResult(invocation, "activity_intention_read_failed", true), err
		}
		if intentionStatus != string(IntentionInProgress) || !time.Now().UTC().Before(expiration) {
			return cancelUnsettledScheduledLifeActivityTx(ctx, tx, invocation, activityID, fluctlightID, intentionID, revision, resultRaw)
		}
	}
	result := mapValue(plan["result"])
	if stringValue(result["status"]) == "extended" {
		return extendLifeActivityTx(ctx, tx, invocation, activityID, fluctlightID, revision, result)
	}
	if time.Now().UTC().Before(notBefore) || stringValue(result["status"]) == "not_due" {
		return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "accepted",
			Output:            map[string]any{"activity_id": activityID, "status": status, "not_before": notBefore.UTC().Format(time.RFC3339Nano)},
			ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "activity:" + activityID}, nil
	}
	if err := validateVirtualActivityResult(kind, mapValue(plan["request"]), result); err != nil {
		return failedCapabilityResult(invocation, "activity_result_invalid", false), err
	}
	return c.service.applyActivityResultTx(ctx, tx, invocation, activityID, fluctlightID, profileID, intentionID, kind, revision, result)
}

func (s *lifeActivityService) applyActivityResultTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, activityID, fluctlightID, profileID, intentionID, kind string, revision int, result map[string]any) (CapabilityResult, error) {
	return applyVirtualActivityResultTx(ctx, tx, s.app, invocation, activityID, fluctlightID, profileID, intentionID, kind, revision, result)
}

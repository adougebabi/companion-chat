package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const scheduleActivityCapabilityName = "intention.schedule"

type scheduleActivityCapability struct {
	service scheduleActivityWriter
	planner SchedulePlanner
	intents *intentionService
}

type scheduleActivityWriter interface {
	currentAcceptedSchedule(context.Context, string) (map[string]any, error)
	currentAcceptedScheduleTx(context.Context, pgx.Tx, string) (map[string]any, error)
	acceptScheduleTx(context.Context, pgx.Tx, string, string, map[string]any) (map[string]any, error)
	replanScheduleTx(context.Context, pgx.Tx, string, string, map[string]any) (map[string]any, error)
}

func scheduledActivityDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: scheduleActivityCapabilityName, Version: "v1", Type: CapabilityTypeAction,
		Description:     "Active goal_ref: schedule action_plan. No goal_ref: request Planner, no action.",
		Surfaces:        []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceNativeCognition},
		FailurePolicy:   FailurePolicyRequiredForVisibleClaim,
		RequiredContext: []ContextSlot{SlotCurrentLife, SlotAgency},
		InputSchema: objectSchema(map[string]any{
			"goal_ref": stringSchema(), "stage_id": stringSchema(), "commitment_id": stringSchema(),
			"goal":               map[string]any{"type": "string", "minLength": 1, "maxLength": 2000},
			"action":             map[string]any{"type": "string", "minLength": 1, "maxLength": 2000},
			"expected_outcome":   map[string]any{"type": "string", "minLength": 1, "maxLength": 2000},
			"action_plan":        map[string]any{"type": "object", "additionalProperties": true},
			"preferred_start_at": map[string]any{"type": "string"},
			"reason":             map[string]any{"type": "string", "minLength": 1, "maxLength": 500},
		}, []string{"action", "expected_outcome", "action_plan", "reason"}, false),
		OutputSchema: map[string]any{"anyOf": []any{objectSchema(map[string]any{
			"goal_id": stringSchema(), "intention_id": stringSchema(), "schedule_id": stringSchema(),
			"schedule_item_id": stringSchema(), "start_at": stringSchema(), "status": enumStringSchema("scheduled"),
		}, []string{"goal_id", "intention_id", "schedule_id", "schedule_item_id", "start_at", "status"}, false), objectSchema(map[string]any{"status": enumStringSchema("planning_requested"), "scheduled": booleanSchema(), "activated": booleanSchema(), "intention_created": booleanSchema()}, []string{"status", "scheduled", "activated", "intention_created"}, false)}},
		SideEffectClass: "native_projection", SuccessBoundary: "scheduled_intention_or_planning_request_committed", ConcurrencyClass: "exclusive", SupportsRetry: true,
		ModelResultOmitFields: []string{"goal_id", "schedule_id"},
	}
}

func (c scheduleActivityCapability) Definition() CapabilityDefinition {
	return scheduledActivityDefinition()
}
func (c scheduleActivityCapability) RequiredContext() []ContextSlot {
	return scheduledActivityDefinition().RequiredContext
}
func (c scheduleActivityCapability) Execute(_ context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return executeToolRequired(invocation)
}

func scheduledActionPlanFromArguments(args map[string]any) (map[string]any, error) {
	plan := cloneMap(mapValue(args["action_plan"]))
	if plan == nil {
		plan = map[string]any{}
	}
	plan["capability"] = lifeActivityStartCapabilityName
	plan["reason"] = args["reason"]
	return plan, validateScheduledLifeActionPlan(plan)
}

func (c scheduleActivityCapability) Prepare(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityInvocation, error) {
	if c.service == nil || c.planner == nil || c.intents == nil || resolved.Life == nil || resolved.Agency == nil {
		return invocation, errors.New("scheduled_activity_context_unavailable")
	}
	args, err := capabilityExecutionArguments(invocation, scheduledActivityDefinition())
	if err != nil {
		return invocation, err
	}
	if invocation.Metadata.Source != "direct" && stringValue(args["goal_ref"]) == "" {
		if strings.TrimSpace(stringValue(args["goal"])) == "" {
			return invocation, ErrInvalidArguments
		}
		return withCapabilityPreparedData(invocation, "goal_planning_only", true)
	}
	actionPlan, err := scheduledActionPlanFromArguments(args)
	if err != nil {
		return invocation, err
	}
	current, err := c.service.currentAcceptedSchedule(ctx, invocation.Metadata.FluctlightID)
	if errors.Is(err, pgx.ErrNoRows) {
		current = map[string]any{"revision": 0, "local_date": resolved.Life.Data["local_date"], "timezone": resolved.Life.Data["timezone"]}
	} else if err != nil {
		return invocation, err
	}
	// The read model is a schedule version; the immutable-history boundary is
	// the time of this planning attempt, not a persisted schedule field.
	current["completed_before"] = c.intents.now().Format(instantLayout)
	resolved.Schedule = &ScheduleContext{Data: current}
	if raw, found, err := capabilityPreparedData(invocation, "scheduled_activity_plan"); err != nil {
		return invocation, err
	} else if found {
		if err := validateScheduledActivityPlan(mapValue(raw), args, actionPlan, resolved, c.intents.now()); err != nil {
			return invocation, err
		}
		return invocation, nil
	}
	planInput := SchedulePlanInput{Intent: stringValue(args["action"]), PlannedAction: map[string]any{
		"goal": args["goal"], "action": args["action"], "expected_outcome": args["expected_outcome"],
		"kind": actionPlan["kind"], "duration_minutes": actionPlan["duration_minutes"], "preferred_start_at": args["preferred_start_at"],
		"action_plan": actionPlan,
	}, Schedule: current, CurrentLife: resolved.Life.Data, Agency: resolved.Agency.Data,
		SourceFactID: invocation.SourceFactID, Timezone: stringValue(current["timezone"])}
	planned, err := c.planner.Plan(ctx, planInput)
	if err != nil {
		return invocation, newCapabilityError("schedule_activity_planner_failed", true, err)
	}
	if err := preserveScheduledActionPlans(planned, current); err != nil {
		return invocation, err
	}
	planned["local_date"] = stringValue(current["local_date"])
	planned["timezone"] = stringValue(current["timezone"])
	planned["expected_revision"] = intValue(current["revision"])
	planned["completed_before"] = firstString(current["completed_before"], c.intents.now().Format(instantLayout))
	planned["expected_life_context_revision"] = stringValue(resolved.Life.Data["context_revision"])
	planned["intent"] = stringValue(args["action"])
	planned["evidence_refs"] = []any{invocation.SourceFactID}
	planned["idempotency_key"] = "tool:" + capabilityOperationID(invocation)
	if err := validateScheduledActivityPlan(planned, args, actionPlan, resolved, c.intents.now()); err != nil {
		return invocation, err
	}
	return withCapabilityPreparedData(invocation, "scheduled_activity_plan", planned)
}

func validateScheduledActivityPlan(planned, args, actionPlan map[string]any, resolved CapabilityContext, at time.Time) error {
	if intValue(planned["expected_revision"]) == 0 {
		if resolved.Life == nil || stringValue(planned["expected_life_context_revision"]) != stringValue(resolved.Life.Data["context_revision"]) || stringValue(planned["local_date"]) != stringValue(resolved.Life.Data["local_date"]) || canonicalTimezone(stringValue(planned["timezone"])) != canonicalTimezone(stringValue(resolved.Life.Data["timezone"])) {
			return ErrConflict
		}
		if err := validateScheduleReplanItems(arrayValue(planned["items"])); err != nil {
			return err
		}
	} else if err := validatePreparedSchedulePlan(planned, map[string]any{"intent": args["action"]}, resolved); err != nil {
		return err
	}
	if _, err := scheduledAppointmentItem(planned, actionPlan, args, at); err != nil {
		return err
	}
	return nil
}

func scheduledAppointmentItem(plan, actionPlan, args map[string]any, at time.Time) (map[string]any, error) {
	var selected map[string]any
	for _, raw := range arrayValue(plan["items"]) {
		item := mapValue(raw)
		if selectedSlot, _ := item["planned_action_slot"].(bool); selectedSlot {
			if selected != nil {
				return nil, errors.New("schedule_action_slot_duplicate")
			}
			selected = item
		}
	}
	if selected == nil {
		return nil, errors.New("schedule_action_slot_missing")
	}
	start, err := parseScheduleTime(stringValue(selected["start_at"]))
	if err != nil || !start.After(at.Add(30*time.Second)) {
		return nil, errors.New("schedule_action_start_not_future")
	}
	end, err := parseScheduleTime(stringValue(selected["end_at"]))
	if err != nil || end.Sub(start) < time.Duration(intValue(actionPlan["duration_minutes"]))*time.Minute {
		return nil, errors.New("schedule_action_slot_too_short")
	}
	if preferred := strings.TrimSpace(stringValue(args["preferred_start_at"])); preferred != "" {
		parsed, err := parseScheduleTime(preferred)
		if err != nil || !parsed.Equal(start) {
			return nil, errors.New("schedule_action_preferred_time_mismatch")
		}
	}
	return selected, nil
}

func (c scheduleActivityCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	if c.service == nil || c.intents == nil {
		return failedCapabilityResult(invocation, "schedule_activity_unavailable", true), errors.New("schedule activity unavailable")
	}
	if raw, found, err := capabilityPreparedData(invocation, "goal_planning_only"); err != nil {
		return failedCapabilityResult(invocation, "invalid_prepared_data", false), err
	} else if found && raw == true {
		args, err := capabilityExecutionArguments(invocation, scheduledActivityDefinition())
		if err != nil {
			return failedCapabilityResult(invocation, "invalid_arguments", false), err
		}
		if err := requestGoalPlanningTx(ctx, tx, invocation.Metadata.FluctlightID, "scheduled-wish:"+capabilityOperationID(invocation), "cognition_wish", args); err != nil {
			return failedCapabilityResult(invocation, "planner_request_failed", true), err
		}
		return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: map[string]any{"status": "planning_requested", "scheduled": false, "activated": false, "intention_created": false}, ProviderRequestID: invocation.ProviderRequestID}, nil
	}
	raw, found, err := capabilityPreparedData(invocation, "scheduled_activity_plan")
	if err != nil || !found {
		return failedCapabilityResult(invocation, "schedule_activity_plan_missing", false), errors.New("prepared activity plan missing")
	}
	planned := mapValue(raw)
	args, err := capabilityExecutionArguments(invocation, scheduledActivityDefinition())
	if err != nil {
		return failedCapabilityResult(invocation, "invalid_arguments", false), err
	}
	actionPlan, err := scheduledActionPlanFromArguments(args)
	if err != nil {
		return failedCapabilityResult(invocation, "schedule_action_invalid", false), err
	}
	current, currentErr := c.service.currentAcceptedScheduleTx(ctx, tx, invocation.Metadata.FluctlightID)
	if errors.Is(currentErr, pgx.ErrNoRows) {
		current = map[string]any{"revision": 0, "local_date": resolved.Life.Data["local_date"], "timezone": resolved.Life.Data["timezone"]}
	} else if currentErr != nil {
		return failedCapabilityResult(invocation, "schedule_read_failed", true), currentErr
	}
	resolved.Schedule = &ScheduleContext{Data: current}
	if intValue(current["revision"]) != intValue(planned["expected_revision"]) {
		return failedCapabilityResult(invocation, "schedule_revision_stale", true), ErrConflict
	}
	if err := validateScheduledActivityPlan(planned, args, actionPlan, resolved, c.intents.now()); err != nil {
		return failedCapabilityResult(invocation, "schedule_activity_plan_stale", true), err
	}
	fluctlightID := invocation.Metadata.FluctlightID
	profileID, err := c.intents.resolveProfile(ctx, tx, fluctlightID, invocation.Metadata.WorkingProfileID)
	if err != nil {
		return failedCapabilityResult(invocation, "intention_profile_unavailable", true), err
	}
	created, err := c.intents.createIntentionTx(ctx, tx, invocation, profileID, args, stringValue(args["reason"]), []string{"tool-operation:" + stableDigest(fluctlightID+"\x1f"+capabilityOperationID(invocation))}, c.intents.now())
	if err != nil {
		return failedCapabilityResult(invocation, "intention_create_failed", false), err
	}
	intentionID, goalID := stringValue(mapValue(created.Output)["intention_id"]), stringValue(mapValue(created.Output)["goal_id"])
	if intentionID == "" || goalID == "" {
		return failedCapabilityResult(invocation, "intention_create_invalid", false), errors.New("created intention identity missing")
	}
	if reused, _ := mapValue(created.Output)["reused"].(bool); reused {
		var existingScheduleID, existingItemID string
		var existingStart time.Time
		lookupErr := tx.QueryRow(ctx, `SELECT s.id,item.id,item.start_at FROM public.life_schedule_items item JOIN public.life_schedules s ON s.id=item.schedule_id WHERE item.intention_id=$1 AND s.fluctlight_id=$2 AND s.status='accepted' AND item.end_at>$3 ORDER BY item.start_at DESC LIMIT 1`, intentionID, fluctlightID, c.intents.now()).Scan(&existingScheduleID, &existingItemID, &existingStart)
		if lookupErr == nil {
			return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: map[string]any{
				"goal_id": goalID, "intention_id": intentionID, "schedule_id": existingScheduleID,
				"schedule_item_id": existingItemID, "start_at": existingStart.UTC().Format(instantLayout), "status": "scheduled",
			}, ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "schedule:" + existingScheduleID}, nil
		}
		if !errors.Is(lookupErr, pgx.ErrNoRows) {
			return failedCapabilityResult(invocation, "scheduled_intention_lookup_failed", true), lookupErr
		}
		if stringValue(mapValue(created.Output)["status"]) != string(IntentionCandidate) {
			return failedCapabilityResult(invocation, "intention_already_active", false), ErrConflict
		}
	}
	item, err := scheduledAppointmentItem(planned, actionPlan, args, c.intents.now())
	if err != nil {
		return failedCapabilityResult(invocation, "schedule_action_slot_invalid", false), err
	}
	item["intention_id"] = intentionID
	item["action_plan"] = actionPlan
	delete(item, "planned_action_slot")
	var ownerID string
	if err := tx.QueryRow(ctx, `SELECT created_by_actor_id FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&ownerID); err != nil {
		return failedCapabilityResult(invocation, "schedule_owner_missing", false), err
	}
	planned["generated_from"] = "model_replan"
	planned["source_fact_id"] = invocation.SourceFactID
	planned["conversation_id"] = invocation.Metadata.ConversationID
	planned["trigger"] = "capability:intention.schedule"
	var result map[string]any
	if intValue(planned["expected_revision"]) == 0 {
		result, err = c.service.acceptScheduleTx(ctx, tx, ownerID, fluctlightID, planned)
	} else {
		result, err = c.service.replanScheduleTx(ctx, tx, ownerID, fluctlightID, planned)
	}
	if err != nil {
		return failedCapabilityResultDetail(invocation, "schedule_activity_accept_failed", true, err.Error()), err
	}
	var itemID string
	if err := tx.QueryRow(ctx, `SELECT id FROM public.life_schedule_items WHERE schedule_id=$1 AND intention_id=$2`, stringValue(result["id"]), intentionID).Scan(&itemID); err != nil {
		return failedCapabilityResult(invocation, "schedule_action_item_missing", true), err
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: map[string]any{
		"goal_id": goalID, "intention_id": intentionID, "schedule_id": result["id"], "schedule_item_id": itemID,
		"start_at": item["start_at"], "status": "scheduled",
	}, ProviderRequestID: invocation.ProviderRequestID, CorrelationID: fmt.Sprintf("schedule:%s", stringValue(result["id"]))}, nil
}

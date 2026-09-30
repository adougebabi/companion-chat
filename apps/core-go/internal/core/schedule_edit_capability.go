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

const scheduleEditCapabilityName = "schedule.edit"

type scheduleEditCapability struct {
	service scheduleEditService
	planner SchedulePlanner
	intents *intentionService
}

type scheduleEditService interface {
	currentAcceptedScheduleTx(context.Context, pgx.Tx, string) (map[string]any, error)
	applyScheduleReplanCapabilityWithTx(context.Context, pgx.Tx, CapabilityInvocation, CapabilityContext) (CapabilityResult, error)
}

func scheduleEditDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: scheduleEditCapabilityName, Version: "v1", Type: CapabilityTypeAction,
		Description:     "Edit one future Schedule item. Inspect first for item_id and revision. Set changes.start_at/end_at for move, changes.activity/scene/location for revise; cancel needs no changes.",
		Surfaces:        []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceNativeCognition},
		FailurePolicy:   FailurePolicyRequiredForVisibleClaim,
		RequiredContext: []ContextSlot{SlotSchedule, SlotCurrentLife, SlotAgency},
		InputSchema: objectSchema(map[string]any{
			"operation":         enumStringSchema("move", "revise", "cancel"),
			"item_id":           stringSchema(),
			"expected_revision": map[string]any{"type": "integer", "minimum": 1},
			"intent":            stringSchema(),
			"reason":            stringSchema(),
			"changes":           openObjectSchema(),
		}, []string{"operation", "item_id", "expected_revision", "intent", "reason"}, false),
		OutputSchema:    scheduleReplanCapabilityDefinition().OutputSchema,
		SideEffectClass: "native_projection", SuccessBoundary: "schedule_version_committed", ConcurrencyClass: "exclusive", SupportsRetry: true,
		ModelResultOmitFields: []string{"schedule_id", "revision", "previous_version"},
	}
}

func (c scheduleEditCapability) Definition() CapabilityDefinition { return scheduleEditDefinition() }
func (c scheduleEditCapability) RequiredContext() []ContextSlot {
	return scheduleEditDefinition().RequiredContext
}

func (c scheduleEditCapability) Prepare(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityInvocation, error) {
	if err := requireCapabilityContext(resolved, SlotSchedule, SlotCurrentLife, SlotAgency); err != nil {
		return invocation, err
	}
	args, err := capabilityExecutionArguments(invocation, scheduleEditDefinition())
	if err != nil {
		return invocation, err
	}
	if err := validateScheduleEditArguments(args); err != nil {
		return invocation, err
	}
	if intValue(args["expected_revision"]) != intValue(resolved.Schedule.Data["revision"]) {
		return invocation, newCapabilityError("schedule_edit_revision_stale", true, ErrConflict)
	}
	target := scheduleItemByID(resolved.Schedule.Data, stringValue(args["item_id"]))
	if target == nil {
		return invocation, newCapabilityError("schedule_item_not_found", false, ErrNotFound)
	}
	start, err := parseScheduleTime(stringValue(target["start_at"]))
	if err != nil || !start.After(time.Now().UTC()) {
		return invocation, newCapabilityError("schedule_edit_history_immutable", false, ErrConflict)
	}
	if raw, found, err := capabilityPreparedData(invocation, "schedule_plan"); err != nil {
		return invocation, err
	} else if found {
		if err := validateScheduleEditPlan(mapValue(raw), args, target, resolved); err != nil {
			return invocation, err
		}
		return invocation, nil
	}
	if c.planner == nil {
		return invocation, newCapabilityError("schedule_edit_planner_unavailable", true, errors.New("schedule planner is not configured"))
	}
	timezone := firstString(stringValue(resolved.Life.Data["timezone"]), stringValue(resolved.Schedule.Data["timezone"]))
	planned, err := c.planner.Plan(ctx, SchedulePlanInput{Intent: stringValue(args["intent"]), TargetEdit: args, Schedule: resolved.Schedule.Data, CurrentLife: resolved.Life.Data, Agency: resolved.Agency.Data, SourceFactID: invocation.SourceFactID, Timezone: timezone})
	if err != nil {
		return invocation, newCapabilityError("schedule_edit_planner_failed", true, err)
	}
	if err := preserveScheduledActionPlans(planned, resolved.Schedule.Data); err != nil {
		return invocation, newCapabilityError("schedule_edit_link_invalid", false, err)
	}
	planned["local_date"] = resolved.Schedule.Data["local_date"]
	planned["timezone"] = resolved.Schedule.Data["timezone"]
	planned["expected_revision"] = intValue(resolved.Schedule.Data["revision"])
	planned["completed_before"] = scheduleReplanCompletedBefore(resolved.Schedule.Data)
	planned["expected_life_context_revision"] = stringValue(resolved.Life.Data["context_revision"])
	planned["intent"] = stringValue(args["intent"])
	planned["reason"] = stringValue(args["reason"])
	planned["evidence_refs"] = []any{invocation.SourceFactID}
	planned["idempotency_key"] = "tool:" + capabilityOperationID(invocation)
	if err := validateScheduleEditPlan(planned, args, target, resolved); err != nil {
		return invocation, newCapabilityError("schedule_edit_plan_invalid", false, err)
	}
	return withCapabilityPreparedData(invocation, "schedule_plan", planned)
}

func validateScheduleEditArguments(args map[string]any) error {
	if len([]rune(stringValue(args["item_id"]))) > 128 || len([]rune(stringValue(args["intent"]))) > 4000 || len([]rune(stringValue(args["reason"]))) > 500 {
		return newCapabilityError("schedule_edit_arguments_invalid", false, ErrInvalidArguments)
	}
	changes := mapValue(args["changes"])
	switch stringValue(args["operation"]) {
	case "move":
		start, startErr := parseScheduleTime(stringValue(changes["start_at"]))
		end, endErr := parseScheduleTime(stringValue(changes["end_at"]))
		if startErr != nil || endErr != nil || !end.After(start) {
			return newCapabilityError("schedule_edit_time_invalid", false, ErrInvalidArguments)
		}
	case "revise":
		if strings.TrimSpace(stringValue(changes["activity"])) == "" && strings.TrimSpace(stringValue(changes["scene"])) == "" && strings.TrimSpace(stringValue(changes["location"])) == "" {
			return newCapabilityError("schedule_edit_changes_required", false, ErrInvalidArguments)
		}
	case "cancel":
	default:
		return newCapabilityError("schedule_edit_operation_invalid", false, ErrInvalidArguments)
	}
	for key, raw := range changes {
		switch key {
		case "start_at", "end_at", "activity", "scene", "location":
			if _, ok := raw.(string); !ok || len([]rune(stringValue(raw))) > 2000 {
				return newCapabilityError("schedule_edit_arguments_invalid", false, ErrInvalidArguments)
			}
		default:
			return newCapabilityError("schedule_edit_arguments_invalid", false, ErrInvalidArguments)
		}
	}
	return nil
}

func scheduleItemByID(schedule map[string]any, itemID string) map[string]any {
	for _, raw := range arrayValue(schedule["items"]) {
		item := mapValue(raw)
		if stringValue(item["id"]) == itemID {
			return item
		}
	}
	return nil
}

func validateScheduleEditPlan(planned, args, target map[string]any, resolved CapabilityContext) error {
	if err := validatePreparedSchedulePlan(planned, args, resolved); err != nil {
		return err
	}
	if stringValue(args["operation"]) == "cancel" {
		oldStart, startErr := parseScheduleTime(stringValue(target["start_at"]))
		oldEnd, endErr := parseScheduleTime(stringValue(target["end_at"]))
		if startErr != nil || endErr != nil || findScheduleItemWithFields(arrayValue(planned["items"]), target, oldStart, oldEnd) {
			return errors.New("cancelled item remains unchanged in replacement")
		}
	}
	matched := 0
	changes := mapValue(args["changes"])
	for _, raw := range arrayValue(planned["items"]) {
		item := mapValue(raw)
		if stringValue(item["source_item_id"]) != stringValue(args["item_id"]) {
			if stringValue(args["operation"]) == "cancel" && stringValue(target["intention_id"]) != "" && stringValue(item["intention_id"]) == stringValue(target["intention_id"]) {
				return errors.New("cancelled intention link remains in replacement")
			}
			continue
		}
		matched++
		if stringValue(args["operation"]) == "cancel" {
			return errors.New("cancelled item remains in replacement")
		}
		for _, key := range []string{"activity", "scene", "location"} {
			if requested, exists := changes[key]; exists && stringValue(item[key]) != stringValue(requested) {
				return fmt.Errorf("edited item %s does not match request", key)
			}
		}
		if stringValue(args["operation"]) == "move" {
			for _, key := range []string{"start_at", "end_at"} {
				want, wantErr := parseScheduleTime(stringValue(changes[key]))
				got, gotErr := parseScheduleTime(stringValue(item[key]))
				if wantErr != nil || gotErr != nil || !want.Equal(got) {
					return fmt.Errorf("edited item %s does not match request", key)
				}
			}
		}
		if stringValue(args["operation"]) == "revise" {
			oldStart, startErr := parseScheduleTime(stringValue(target["start_at"]))
			newStart, newStartErr := parseScheduleTime(stringValue(item["start_at"]))
			oldEnd, endErr := parseScheduleTime(stringValue(target["end_at"]))
			newEnd, newEndErr := parseScheduleTime(stringValue(item["end_at"]))
			if startErr != nil || newStartErr != nil || endErr != nil || newEndErr != nil || !oldStart.Equal(newStart) || !oldEnd.Equal(newEnd) {
				return errors.New("revised item changed its time")
			}
		}
		if link := stringValue(target["intention_id"]); link != "" && stringValue(item["intention_id"]) != link {
			return errors.New("edited item lost intention link")
		}
	}
	if stringValue(args["operation"]) == "cancel" && matched == 0 {
		return nil
	}
	if matched != 1 {
		return errors.New("edited item selection is ambiguous")
	}
	return nil
}

func (c scheduleEditCapability) Execute(_ context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return executeToolRequired(invocation)
}

func (c scheduleEditCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	if c.service == nil || tx == nil {
		return failedCapabilityResult(invocation, "schedule_edit_unavailable", true), errors.New("schedule service unavailable")
	}
	args, err := capabilityExecutionArguments(invocation, scheduleEditDefinition())
	if err != nil {
		return failedCapabilityResult(invocation, "invalid_arguments", false), err
	}
	rawPlan, found, err := capabilityPreparedData(invocation, "schedule_plan")
	if err != nil || !found {
		return failedCapabilityResult(invocation, "schedule_edit_plan_missing", false), errors.New("prepared schedule edit plan required")
	}
	current, err := c.service.currentAcceptedScheduleTx(ctx, tx, invocation.Metadata.FluctlightID)
	if err != nil {
		return failedCapabilityResult(invocation, "schedule_edit_schedule_read_failed", true), err
	}
	if intValue(current["revision"]) != intValue(args["expected_revision"]) {
		return failedCapabilityResult(invocation, "schedule_edit_revision_stale", true), ErrConflict
	}
	target := scheduleItemByID(current, stringValue(args["item_id"]))
	if target == nil {
		return failedCapabilityResult(invocation, "schedule_item_not_found", false), ErrNotFound
	}
	if err := validateScheduleEditPlan(mapValue(rawPlan), args, target, resolved); err != nil {
		return failedCapabilityResultDetail(invocation, "schedule_edit_plan_invalid", false, err.Error()), err
	}
	if stringValue(target["action_status"]) == "in_progress" {
		return failedCapabilityResult(invocation, "schedule_active_activity_replan_blocked", false), ErrConflict
	}
	if stringValue(args["operation"]) == "cancel" && stringValue(target["intention_id"]) != "" {
		if c.intents == nil {
			return failedCapabilityResult(invocation, "intention_unavailable", true), errors.New("intention service unavailable")
		}
		cancel := invocation
		cancel.CapabilityName = intentionDecideCapabilityName
		cancel.Metadata.OperationID = capabilityOperationID(invocation) + ":cancel-linked"
		cancel.Arguments, _ = json.Marshal(map[string]any{"operation": "cancel", "intention_id": target["intention_id"], "reason": args["reason"]})
		cancel.PreparedPayload = nil
		if _, err := (intentionDecideCapability{service: c.intents}).ExecuteTx(ctx, tx, cancel, CapabilityContext{}); err != nil {
			return failedCapabilityResultDetail(invocation, "schedule_edit_intention_cancel_failed", false, err.Error()), err
		}
	}
	return c.service.applyScheduleReplanCapabilityWithTx(ctx, tx, invocation, resolved)
}

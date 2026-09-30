package core

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// A schedule item is executable only when it carries this closed, Core-owned
// action plan and an explicit Intention link. Ordinary schedule prose is not
// an execution instruction.
func validateScheduledLifeActionPlan(plan map[string]any) error {
	if len(jsonBytes(plan)) > 4096 {
		return errors.New("schedule_action_payload_too_large")
	}
	if stringValue(plan["capability"]) != lifeActivityStartCapabilityName {
		return errors.New("schedule_action_capability_invalid")
	}
	for key := range plan {
		switch key {
		case "capability", "kind", "duration_minutes", "desired_hair_color", "desired_hair_length", "category", "slot", "description", "reason":
		default:
			return errors.New("schedule_action_field_invalid")
		}
	}
	minutes, numeric := numberFloat(plan["duration_minutes"])
	duration := int(minutes)
	if !numeric || math.Trunc(minutes) != minutes || duration < 15 || duration > 240 || strings.TrimSpace(stringValue(plan["reason"])) == "" || len([]rune(stringValue(plan["reason"]))) > 500 {
		return errors.New("schedule_action_duration_or_reason_invalid")
	}
	for key, limit := range map[string]int{"desired_hair_color": 128, "desired_hair_length": 128, "category": 64, "slot": 64, "description": 512} {
		if len([]rune(stringValue(plan[key]))) > limit {
			return errors.New("schedule_action_field_too_long")
		}
	}
	switch stringValue(plan["kind"]) {
	case "hair_dye":
		if strings.TrimSpace(stringValue(plan["desired_hair_color"])) == "" || stringValue(plan["desired_hair_length"]) != "" || stringValue(plan["category"]) != "" || stringValue(plan["slot"]) != "" || stringValue(plan["description"]) != "" {
			return errors.New("schedule_hair_dye_target_invalid")
		}
	case "haircut":
		if strings.TrimSpace(stringValue(plan["desired_hair_length"])) == "" || stringValue(plan["category"]) != "" || stringValue(plan["slot"]) != "" || stringValue(plan["description"]) != "" {
			return errors.New("schedule_haircut_target_invalid")
		}
	case "virtual_shopping":
		if stringValue(plan["desired_hair_length"]) != "" || stringValue(plan["desired_hair_color"]) != "" {
			return errors.New("schedule_shopping_target_invalid")
		}
		for _, key := range []string{"category", "slot", "description"} {
			if strings.TrimSpace(stringValue(plan[key])) == "" {
				return errors.New("schedule_shopping_target_invalid")
			}
		}
	default:
		return errors.New("schedule_action_kind_invalid")
	}
	return nil
}

func nullableJSON(value map[string]any) any {
	if len(value) == 0 {
		return nil
	}
	return jsonBytes(value)
}

func preserveScheduledActionPlans(planned, current map[string]any) error {
	links := make(map[string]map[string]any)
	for _, raw := range arrayValue(current["items"]) {
		item := mapValue(raw)
		if id := strings.TrimSpace(stringValue(item["intention_id"])); id != "" {
			links[id] = mapValue(item["action_plan"])
		}
	}
	for _, raw := range arrayValue(planned["items"]) {
		item := mapValue(raw)
		id := strings.TrimSpace(stringValue(item["intention_id"]))
		if id == "" {
			continue
		}
		plan, known := links[id]
		if !known {
			return errors.New("schedule_replan_intention_link_unknown")
		}
		if len(plan) == 0 {
			return errors.New("schedule_replan_action_plan_missing")
		}
		item["action_plan"] = cloneMap(plan)
	}
	return nil
}

func scheduledActionMatchesArguments(plan, args map[string]any) bool {
	if validateScheduledLifeActionPlan(plan) != nil || stringValue(plan["kind"]) != stringValue(args["kind"]) || intValue(plan["duration_minutes"]) != intValue(args["duration_minutes"]) {
		return false
	}
	for _, key := range []string{"desired_hair_color", "desired_hair_length", "category", "slot", "description"} {
		if strings.TrimSpace(stringValue(plan[key])) != strings.TrimSpace(stringValue(args[key])) {
			return false
		}
	}
	return true
}

func syncScheduledIntentionTx(ctx context.Context, tx pgx.Tx, intentionID, itemID string, startAt time.Time) error {
	current, err := loadIntentionAuthorityByIDTx(ctx, tx, intentionID)
	if err != nil {
		return err
	}
	if current.Status != IntentionCandidate && current.Status != IntentionQualified {
		return errors.New("scheduled_intention_not_plannable")
	}
	if !startAt.After(time.Now().UTC()) || !startAt.Before(current.Expiration) {
		return errors.New("scheduled_intention_time_invalid")
	}
	trigger := TypedIntentionTrigger{Type: IntentionTriggerTime, DueAt: &startAt}
	if current.Trigger.Type != IntentionTriggerTime || current.Trigger.DueAt == nil || !current.Trigger.DueAt.Equal(startAt) || current.PreferredTime == nil || !current.PreferredTime.Equal(startAt) || !containsString(current.CapabilityConstraints, lifeActivityStartCapabilityName) {
		updated, record, err := ApplyIntentionCommand(current, IntentionCommand{
			Operation: IntentionUpdate, ExpectedRevision: current.Revision,
			Patch:        IntentionPatch{Trigger: &trigger, PreferredTime: &startAt, CapabilityConstraints: []string{lifeActivityStartCapabilityName}},
			EvidenceRefs: []string{"schedule-item:" + itemID}, Reason: "accepted schedule item bound to intention", OccurredAt: time.Now().UTC(),
		})
		if err != nil {
			return err
		}
		if _, err := persistIntentionAuthorityTx(ctx, tx, &current, updated, record, fmt.Sprintf("schedule-intention:%s:%s:%d", intentionID, itemID, updated.Revision)); err != nil {
			return err
		}
		current = updated
	}
	if current.Status == IntentionCandidate {
		qualified, record, err := ApplyIntentionCommand(current, IntentionCommand{
			Operation: IntentionQualify, ExpectedRevision: current.Revision,
			EvidenceRefs: []string{"schedule-item:" + itemID}, Reason: "future scheduled action accepted", OccurredAt: time.Now().UTC(),
		})
		if err != nil {
			return err
		}
		if _, err := persistIntentionAuthorityTx(ctx, tx, &current, qualified, record, fmt.Sprintf("schedule-qualify:%s:%s:%d", intentionID, itemID, qualified.Revision)); err != nil {
			return err
		}
		current = qualified
	}
	return syncIntentionTriggerWorkflowTx(ctx, tx, current)
}

// Cancellation of an accepted schedule revokes its still-open executable
// intentions in the same transaction. A sole-intention goal is cancelled too;
// a goal shared with another open intention remains available to that work.
func cancelScheduledIntentionTx(ctx context.Context, tx pgx.Tx, fluctlightID, intentionID, evidenceRef, commandKey string, at time.Time) error {
	current, err := loadIntentionAuthorityByIDTx(ctx, tx, intentionID)
	if err != nil {
		return err
	}
	if current.FluctlightID != fluctlightID {
		return ErrConflict
	}
	if intentionStatusTerminal(current.Status) {
		return nil
	}
	_, err = transitionLinkedIntentionTx(ctx, tx, fluctlightID, current.ProfileID, intentionID, IntentionCancel,
		evidenceRef, "linked schedule was cancelled", commandKey, at)
	if err != nil {
		return err
	}
	if current.GoalEntityID == "" {
		return nil
	}
	var goalRevision, openIntentions int
	if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_goals WHERE id=$1 AND fluctlight_id=$2 FOR UPDATE`, current.GoalEntityID, fluctlightID).Scan(&goalRevision); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_intentions WHERE goal_id=$1 AND fluctlight_id=$2 AND status NOT IN ('completed','cancelled','expired')`, current.GoalEntityID, fluctlightID).Scan(&openIntentions); err != nil {
		return err
	}
	if openIntentions != 0 {
		return nil
	}
	goal, err := loadGoalAuthorityTx(ctx, tx, fluctlightID, "goal:ctx_"+stableDigest(current.GoalEntityID), ContextReference{EntityID: current.GoalEntityID, Revision: goalRevision})
	if err != nil {
		return err
	}
	if goalStatusTerminal(goal.Status) {
		return nil
	}
	next, record, err := ApplyGoalCommand(&goal, GoalCommand{Operation: GoalCancel, ExpectedRevision: goal.Revision,
		EvidenceRefs: []string{evidenceRef}, Reason: "linked schedule was cancelled", OccurredAt: at})
	if err != nil {
		return err
	}
	_, err = persistGoalAuthorityTx(ctx, tx, &goal, next, record, "scheduled-goal-cancel:"+commandKey)
	return err
}

func cancelScheduleLinkedIntentionsTx(ctx context.Context, tx pgx.Tx, fluctlightID, scheduleID string, at time.Time) error {
	rows, err := tx.Query(ctx, `SELECT DISTINCT intention_id FROM public.life_schedule_items WHERE schedule_id=$1 AND intention_id IS NOT NULL AND action_plan IS NOT NULL ORDER BY intention_id`, scheduleID)
	if err != nil {
		return err
	}
	var intentionIDs []string
	for rows.Next() {
		var intentionID string
		if err := rows.Scan(&intentionID); err != nil {
			rows.Close()
			return err
		}
		intentionIDs = append(intentionIDs, intentionID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, intentionID := range intentionIDs {
		if err := cancelScheduledIntentionTx(ctx, tx, fluctlightID, intentionID, "schedule:"+scheduleID,
			"schedule-cancel-intention:"+scheduleID+":"+intentionID, at); err != nil {
			return err
		}
	}
	return nil
}

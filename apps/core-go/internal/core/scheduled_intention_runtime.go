package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (a *App) processScheduledIntentionTrigger(ctx context.Context, intentionID, fluctlightID, ownerID string) (map[string]any, bool, error) {
	var itemID, profileID, status string
	var startAt, endAt time.Time
	var actionRaw, triggerRaw []byte
	err := a.DB.Pool().QueryRow(ctx, `
		SELECT item.id,COALESCE(i.profile_id,''),i.status,item.start_at,item.end_at,item.action_plan,i.trigger
		FROM public.life_schedule_items item
		JOIN public.life_schedules s ON s.id=item.schedule_id AND s.fluctlight_id=$2 AND s.status='accepted'
		JOIN public.fluctlight_intentions i ON i.id=item.intention_id AND i.fluctlight_id=s.fluctlight_id
		WHERE item.intention_id=$1 AND item.action_plan IS NOT NULL
		ORDER BY item.start_at DESC LIMIT 1`, intentionID, fluctlightID).Scan(&itemID, &profileID, &status, &startAt, &endAt, &actionRaw, &triggerRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		var evidenceRaw []byte
		if readErr := a.DB.Pool().QueryRow(ctx, `SELECT evidence_refs FROM public.fluctlight_intentions WHERE id=$1 AND fluctlight_id=$2`, intentionID, fluctlightID).Scan(&evidenceRaw); readErr != nil {
			return nil, true, readErr
		}
		for _, evidence := range decisionServiceRefValues(decodeArray(evidenceRaw)) {
			if strings.HasPrefix(evidence, "schedule-item:") {
				if err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
					return cancelScheduledIntentionTx(ctx, tx, fluctlightID, intentionID, evidence,
						"stale-schedule-intention:"+intentionID, a.now().UTC())
				}); err != nil {
					return nil, true, err
				}
				return map[string]any{"intention_id": intentionID, "status": "stale_schedule"}, true, nil
			}
		}
		return nil, false, nil
	}
	if err != nil {
		return nil, true, err
	}
	action := decodeObject(actionRaw)
	if err := validateScheduledLifeActionPlan(action); err != nil {
		return nil, true, err
	}
	trigger := decodeObject(triggerRaw)
	triggerAt, parseErr := time.Parse(time.RFC3339Nano, stringValue(trigger["due_at"]))
	if parseErr != nil || stringValue(trigger["type"]) != string(IntentionTriggerTime) || !triggerAt.Equal(startAt) || (status != string(IntentionQualified) && status != string(IntentionDue)) {
		return map[string]any{"intention_id": intentionID, "status": "stale_schedule"}, true, nil
	}
	now := a.now().UTC()
	if now.Before(startAt) {
		return map[string]any{"intention_id": intentionID, "status": "pending"}, true, nil
	}
	if !now.Before(endAt) {
		if err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			_, err := transitionLinkedIntentionTx(ctx, tx, fluctlightID, profileID, intentionID, IntentionPause,
				"schedule-item:"+itemID, "scheduled activity window was missed", "schedule-missed:"+itemID, now)
			return err
		}); err != nil {
			return nil, true, err
		}
		return map[string]any{"intention_id": intentionID, "status": "missed"}, true, nil
	}
	// A paused run can be closed before its planned result. Resuming the same
	// Intention must require a new schedule item, not replay the old start Tool.
	var cancelledRun bool
	if err := a.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_life_activity_runs WHERE fluctlight_id=$1 AND intention_id=$2 AND status='cancelled' AND result_json #>> '{request,schedule_item_id}'=$3)`, fluctlightID, intentionID, itemID).Scan(&cancelledRun); err != nil {
		return nil, true, err
	}
	if cancelledRun {
		if err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			return cancelScheduledIntentionTx(ctx, tx, fluctlightID, intentionID, "schedule-item:"+itemID,
				"cancelled-schedule-run:"+itemID, now)
		}); err != nil {
			return nil, true, err
		}
		return map[string]any{"intention_id": intentionID, "status": "stale_schedule"}, true, nil
	}
	arguments := cloneMap(action)
	delete(arguments, "capability")
	arguments["intention_id"] = intentionID
	arguments["schedule_item_id"] = itemID
	receipt, err := a.ExecuteTool(ctx, ToolExecutionRequest{
		AuthorizationPolicy: "autonomy", CapabilityName: lifeActivityStartCapabilityName, OperationID: "scheduled-start:" + itemID,
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID,
		EvidenceID: "schedule-item:" + itemID, Surface: CapabilitySurfaceNativeCognition,
		CorrelationID: "scheduled-intention:" + intentionID, Arguments: jsonBytes(arguments),
	})
	if err != nil {
		return nil, true, fmt.Errorf("scheduled start %s: %w", receipt.Result.ErrorCode, err)
	}
	activityID := stringValue(mapValue(receipt.Result.Output)["activity_id"])
	if receipt.Result.Status != "accepted" || activityID == "" {
		return nil, true, fmt.Errorf("scheduled activity start was not accepted: %s", receipt.Result.Status)
	}
	return map[string]any{"intention_id": intentionID, "status": "activity_started", "activity_id": activityID,
		"not_before": mapValue(receipt.Result.Output)["not_before"], "schedule_item_id": itemID}, true, nil
}

// ResolveScheduledLifeActivity executes the existing result Tool under a
// stable revision-scoped operation, so a deferred result may be attempted
// again without replaying its previous Tool receipt.
func (a *App) ResolveScheduledLifeActivity(ctx context.Context, activityID string) (map[string]any, error) {
	activityID = strings.TrimSpace(activityID)
	if activityID == "" {
		return nil, ErrNotFound
	}
	var fluctlightID, ownerID, profileID, status string
	var revision int
	var notBefore time.Time
	err := a.DB.Pool().QueryRow(ctx, `SELECT r.fluctlight_id,f.created_by_actor_id,r.profile_id,r.status,r.revision,r.not_before FROM public.fluctlight_life_activity_runs r JOIN public.fluctlights f ON f.id=r.fluctlight_id WHERE r.id=$1`, activityID).Scan(&fluctlightID, &ownerID, &profileID, &status, &revision, &notBefore)
	if err != nil {
		return nil, err
	}
	if status == "completed" || status == "failed" || status == "cancelled" {
		return map[string]any{"activity_id": activityID, "status": status}, nil
	}
	if a.now().UTC().Before(notBefore) {
		return map[string]any{"activity_id": activityID, "status": "pending", "not_before": notBefore.UTC().Format(instantLayout)}, nil
	}
	receipt, err := a.ExecuteTool(ctx, ToolExecutionRequest{
		CapabilityName:       lifeActivityAdvanceCapabilityName,
		OperationID:          fmt.Sprintf("scheduled-resolve:%s:%d", activityID, revision),
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID,
		EvidenceID: "activity:" + activityID, Surface: CapabilitySurfaceNativeCognition,
		CorrelationID: "scheduled-activity:" + activityID,
		Arguments:     jsonBytes(map[string]any{"activity_id": activityID}),
	})
	if err != nil {
		return nil, err
	}
	output := mapValue(receipt.Result.Output)
	return map[string]any{"activity_id": activityID, "status": firstString(output["status"], receipt.Result.Status), "not_before": output["not_before"]}, nil
}

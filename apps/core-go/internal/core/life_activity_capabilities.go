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
		Description:   "Start a bounded virtual shopping or haircut activity with an earliest completion time. Accepted means an activity exists, not that a purchase or haircut succeeded; this never makes a real payment.",
		Surfaces:      []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceNativeCognition},
		FailurePolicy: FailurePolicyOptionalInternal,
		InputSchema: objectSchema(map[string]any{
			"kind":                enumStringSchema("virtual_shopping", "haircut"),
			"intention_id":        map[string]any{"type": "string", "maxLength": 128},
			"duration_minutes":    map[string]any{"type": "integer", "minimum": 15, "maximum": 240},
			"category":            map[string]any{"type": "string", "maxLength": 64},
			"slot":                map[string]any{"type": "string", "maxLength": 64},
			"description":         map[string]any{"type": "string", "maxLength": 512},
			"desired_hair_length": map[string]any{"type": "string", "maxLength": 128},
			"desired_hair_color":  map[string]any{"type": "string", "maxLength": 128},
			"reason":              map[string]any{"type": "string", "minLength": 1, "maxLength": 500},
		}, []string{"kind", "reason"}, false),
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
	intentionID := strings.TrimSpace(stringValue(args["intention_id"]))
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
		if err := tx.QueryRow(ctx, `SELECT expiration,capability_constraints FROM public.fluctlight_intentions WHERE id=$1 AND fluctlight_id=$2 AND COALESCE(profile_id,'')=$3 FOR UPDATE`, intentionID, fluctlightID, profileID).Scan(&expiration, &constraintRaw); err != nil {
			return failedCapabilityResult(invocation, "activity_intention_not_found", false), err
		}
		if !now.Before(expiration) {
			return failedCapabilityResult(invocation, "activity_intention_expired", false), ErrConflict
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
	if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_life_activity_runs(id,fluctlight_id,profile_id,kind,status,intention_id,scheduled_at,started_at,not_before,result_json,revision,operation_id) VALUES($1,$2,$3,$4,'in_progress',$5,$6,$6,$7,$8,1,$9)`, activityID, fluctlightID, profileID, kind, nullableString(intentionID), now, notBefore, jsonBytes(result), capabilityOperationID(invocation)); err != nil {
		return failedCapabilityResult(invocation, "activity_start_failed", true), err
	}
	if err := appendOutboxTx(ctx, tx, "life.activity.started", "life_activity", activityID, fluctlightID, invocation.SourceFactID,
		"activity:"+activityID, "activity-start:"+activityID, map[string]any{"activity_id": activityID, "kind": kind, "status": "in_progress", "not_before": notBefore.Format(time.RFC3339Nano)}); err != nil {
		return failedCapabilityResult(invocation, "activity_event_failed", true), err
	}
	receiptResult := CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "accepted",
		Output:            map[string]any{"activity_id": activityID, "status": "in_progress", "not_before": notBefore.Format(time.RFC3339Nano), "intention_id": intentionID},
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
		Description:   "Advance one already started virtual activity. Before its earliest completion time it remains accepted; afterward a separate bounded result task can resolve success, failure, or deferral. Only a completed result changes body or inventory.",
		Surfaces:      []CapabilitySurface{CapabilitySurfaceWakeUp, CapabilitySurfaceNativeCognition, CapabilitySurfaceConversation},
		FailurePolicy: FailurePolicyOptionalInternal,
		InputSchema:   objectSchema(map[string]any{"activity_id": map[string]any{"type": "string", "minLength": 1, "maxLength": 128}}, []string{"activity_id"}, false),
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
	activityID := stringValue(args["activity_id"])
	fluctlightID := invocation.Metadata.FluctlightID
	var profileID, kind, status string
	var startedAt, notBefore time.Time
	var revision int
	var resultRaw []byte
	err = app.DB.Pool().QueryRow(ctx, `SELECT profile_id,kind,status,started_at,not_before,revision,result_json FROM public.fluctlight_life_activity_runs WHERE id=$1 AND fluctlight_id=$2`, activityID, fluctlightID).Scan(&profileID, &kind, &status, &startedAt, &notBefore, &revision, &resultRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return invocation, ErrNotFound
	}
	if err != nil {
		return invocation, err
	}
	plan := map[string]any{"activity_id": activityID, "profile_id": profileID, "kind": kind, "status": status,
		"revision": revision, "not_before": notBefore.UTC().Format(time.RFC3339Nano)}
	if status != "in_progress" && status != "deferred" {
		return withCapabilityPreparedData(invocation, "activity_advance", plan)
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
		if stringValue(item["category"]) != stringValue(request["category"]) || stringValue(item["slot"]) != stringValue(request["slot"]) || strings.TrimSpace(stringValue(item["description"])) == "" || len([]rune(stringValue(item["description"]))) > 512 {
			return errors.New("virtual_shopping_item_mismatch")
		}
		if stringValue(result["hair_length"]) != "" || stringValue(result["hair_color"]) != "" {
			return errors.New("virtual_shopping_body_effect_invalid")
		}
	case "haircut":
		if len(mapValue(result["acquired_item"])) > 0 || strings.TrimSpace(stringValue(result["hair_length"])) == "" || len([]rune(stringValue(result["hair_length"]))) > 128 {
			return errors.New("virtual_haircut_result_invalid")
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
	activityID := stringValue(args["activity_id"])
	if activityID != stringValue(plan["activity_id"]) {
		return failedCapabilityResult(invocation, "activity_plan_identity_mismatch", false), ErrInvalidArguments
	}
	fluctlightID := invocation.Metadata.FluctlightID
	var profileID, kind, status, intentionID string
	var notBefore time.Time
	var revision int
	err = tx.QueryRow(ctx, `SELECT profile_id,kind,status,COALESCE(intention_id,''),not_before,revision FROM public.fluctlight_life_activity_runs WHERE id=$1 AND fluctlight_id=$2 FOR UPDATE`, activityID, fluctlightID).Scan(&profileID, &kind, &status, &intentionID, &notBefore, &revision)
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
	result := mapValue(plan["result"])
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

package core

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func sceneCapabilityDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: "scene_event", Version: "v1", Type: CapabilityTypeAction,
		Description:     "Start, switch, or end the Fluctlight's current scene, activity, or location.",
		Surfaces:        []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceAutonomy, CapabilitySurfaceNativeCognition},
		FailurePolicy:   FailurePolicyRequiredForVisibleClaim,
		RequiredContext: []ContextSlot{SlotCurrentLife},
		InputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []any{"operation", "confidence"},
			"oneOf": []any{
				map[string]any{"required": []any{"operation", "scene", "activity"}, "properties": map[string]any{"operation": map[string]any{"type": "string", "enum": []any{"start", "switch"}}}},
				map[string]any{"required": []any{"operation"}, "properties": map[string]any{"operation": map[string]any{"type": "string", "enum": []any{"end"}}}},
			},
			"properties": map[string]any{
				"operation":  map[string]any{"type": "string", "enum": []any{"start", "switch", "end"}},
				"scene":      map[string]any{"type": "string", "minLength": 1, "maxLength": 512},
				"activity":   map[string]any{"type": "string", "minLength": 1, "maxLength": 512},
				"location":   map[string]any{"type": "string", "maxLength": 512},
				"confidence": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
			},
		},
		OutputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []any{"operation", "status", "event_id", "inbox_id", "event_revision", "expected_context_revision", "resulting_context_revision", "replayed"},
			"properties": map[string]any{
				"operation":                  map[string]any{"type": "string", "enum": []any{"start", "switch", "end"}},
				"status":                     map[string]any{"type": "string", "enum": []any{"confirmed", "inferred", "ended"}},
				"event_id":                   map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
				"inbox_id":                   map[string]any{"type": "string", "maxLength": 128},
				"event_revision":             map[string]any{"type": "integer", "minimum": 1},
				"expected_context_revision":  map[string]any{"type": "string", "minLength": 1, "maxLength": 64},
				"resulting_context_revision": map[string]any{"type": "string", "minLength": 1, "maxLength": 64},
				"replayed":                   map[string]any{"type": "boolean"},
			},
		},
		SideEffectClass: "native_projection", SuccessBoundary: "life_context_committed", ConcurrencyClass: "exclusive", SupportsCancel: false, SupportsRetry: true, RequiresPreflight: false,
		ProvenanceFields: []string{"evidence_refs", "idempotency_key"},
	}
}

func presenceCapabilityDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: "presence_event", Version: "v1", Type: CapabilityTypeAction,
		Description:     "Record a bounded temporary interaction presence overlay.",
		Surfaces:        []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceAutonomy, CapabilitySurfaceNativeCognition},
		FailurePolicy:   FailurePolicyRequiredForVisibleClaim,
		RequiredContext: []ContextSlot{SlotCurrentLife},
		InputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []any{"confidence"},
			"anyOf": []any{
				map[string]any{"required": []any{"user_presence"}},
				map[string]any{"required": []any{"current_task"}},
				map[string]any{"required": []any{"operation"}, "properties": map[string]any{"operation": map[string]any{"type": "string", "enum": []any{"clear"}}}},
			},
			"properties": map[string]any{
				"operation":     map[string]any{"type": "string", "enum": []any{"set", "clear"}},
				"user_presence": map[string]any{"type": "string", "maxLength": 128},
				"current_task":  map[string]any{"type": "string", "maxLength": 512},
				"expires_at":    map[string]any{"type": "string"},
				"confidence":    map[string]any{"type": "number", "minimum": 0, "maximum": 1},
			},
		},
		OutputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []any{"operation", "status", "overlay_id", "inbox_id", "overlay_revision", "expected_context_revision", "resulting_context_revision", "replayed"},
			"properties": map[string]any{
				"operation":                  map[string]any{"type": "string", "enum": []any{"set", "clear"}},
				"status":                     map[string]any{"type": "string", "enum": []any{"active", "cleared"}},
				"overlay_id":                 map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
				"inbox_id":                   map[string]any{"type": "string", "maxLength": 128},
				"overlay_revision":           map[string]any{"type": "integer", "minimum": 1},
				"expected_context_revision":  map[string]any{"type": "string", "minLength": 1, "maxLength": 64},
				"resulting_context_revision": map[string]any{"type": "string", "minLength": 1, "maxLength": 64},
				"replayed":                   map[string]any{"type": "boolean"},
			},
		},
		SideEffectClass: "native_projection", SuccessBoundary: "presence_overlay_committed", ConcurrencyClass: "exclusive", SupportsCancel: false, SupportsRetry: true, RequiresPreflight: false,
		ProvenanceFields: []string{"evidence_refs", "idempotency_key"},
	}
}

func (a *App) applySceneCapability(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	return failedCapabilityResult(invocation, "caller_transaction_required", false), newCapabilityError("caller_transaction_required", false, ErrConflict)
}

func (a *App) applySceneCapabilityTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	return a.applySceneCapabilityWithTx(ctx, tx, invocation, resolved)
}

func (a *App) applySceneCapabilityWithTx(ctx context.Context, callerTx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	if callerTx == nil {
		return failedCapabilityResult(invocation, "caller_transaction_required", false), newCapabilityError("caller_transaction_required", false, ErrConflict)
	}
	plan, err := scenePlanFromInvocation(invocation)
	if err != nil {
		return failedCapabilityResultDetail(invocation, "scene_plan_invalid", false, err.Error()), err
	}
	frozenLife := resolved.Life
	if frozenLife == nil || stringValue(frozenLife.Data["context_revision"]) != plan.ExpectedLifeContextRevision || stringValue(frozenLife.Data["source"]) != plan.ExpectedSource || stringValue(frozenLife.Data["event_id"]) != plan.ExpectedEventID || intValue(frozenLife.Data["event_revision"]) != plan.ExpectedEventRevision {
		return failedCapabilityResult(invocation, "scene_prepared_context_mismatch", false), newCapabilityError("scene_prepared_context_mismatch", false, ErrConflict)
	}
	fluctlightID, conversationID, sourceFactID := invocation.Metadata.FluctlightID, invocation.Metadata.ConversationID, invocation.SourceFactID
	eventID := "event_" + stableDigest(fluctlightID+":"+plan.IdempotencyKey)
	if err := lockLifeContextTx(ctx, callerTx, fluctlightID); err != nil {
		return failedCapabilityResult(invocation, "scene_persist_failed", true), err
	}
	var existingDigest string
	var existingResult []byte
	if replayErr := callerTx.QueryRow(ctx, `SELECT COALESCE(request_digest,''),result FROM public.life_events WHERE fluctlight_id=$1 AND idempotency_key=$2 FOR UPDATE`, fluctlightID, plan.IdempotencyKey).Scan(&existingDigest, &existingResult); replayErr == nil {
		if existingDigest != plan.RequestDigest {
			return failedCapabilityResult(invocation, "scene_idempotency_conflict", false), newCapabilityError("scene_idempotency_conflict", false, ErrConflict)
		}
		output := decodeObject(existingResult)
		if len(output) == 0 {
			return failedCapabilityResult(invocation, "scene_replay_result_invalid", false), errors.New("scene replay result invalid")
		}
		output["replayed"] = true
		return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: output, ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "scene:" + eventID}, nil
	} else if !errors.Is(replayErr, pgx.ErrNoRows) {
		return failedCapabilityResult(invocation, "scene_persist_failed", true), replayErr
	}
	applyAt := time.Now().UTC()
	if !plan.EndsAt.After(applyAt) {
		return failedCapabilityResult(invocation, "scene_plan_expired", false), newCapabilityError("scene_plan_expired", false, ErrConflict)
	}
	liveContext, err := a.requireLifeContextRevisionTx(ctx, callerTx, fluctlightID, plan.ExpectedLifeContextRevision, applyAt)
	if err != nil {
		if errors.Is(err, ErrLifeContextStale) {
			return failedCapabilityResult(invocation, "scene_context_stale", false), newCapabilityError("scene_context_stale", false, err)
		}
		return failedCapabilityResult(invocation, "scene_persist_failed", true), err
	}
	previousScene := map[string]any{"source": liveContext["source"], "scene": liveContext["scene"], "activity": liveContext["activity"], "location": liveContext["location"], "context_revision": liveContext["context_revision"]}
	if plan.Operation == "switch" || plan.Operation == "end" {
		if plan.ExpectedSource == "event" {
			updated, err := callerTx.Exec(ctx, `UPDATE public.life_events SET end_at=LEAST(end_at,$4),expires_at=$4,status='cancelled',revision=revision+1,updated_at=$4 WHERE id=$1 AND fluctlight_id=$2 AND revision=$3 AND status IN ('confirmed','inferred') AND start_at<=$4 AND end_at>$4 AND (expires_at IS NULL OR expires_at>$4)`, plan.ExpectedEventID, fluctlightID, plan.ExpectedEventRevision, applyAt)
			if err != nil {
				return failedCapabilityResult(invocation, "scene_persist_failed", true), err
			}
			if updated.RowsAffected() != 1 {
				return failedCapabilityResult(invocation, "scene_context_stale", false), newCapabilityError("scene_context_stale", false, ErrLifeContextStale)
			}
		} else if plan.Operation == "end" {
			return failedCapabilityResult(invocation, "scene_context_stale", false), newCapabilityError("scene_context_stale", false, ErrLifeContextStale)
		}
	}
	status := "inferred"
	startAt := applyAt
	endAt := plan.EndsAt
	expiresAt := any(plan.EndsAt)
	kind := "scene_inferred"
	var sceneValue, activityValue, locationValue any = nullableString(plan.Scene), nullableString(plan.Activity), nullableString(plan.Location)
	if plan.Operation == "end" {
		status, kind, endAt, expiresAt = "confirmed", "scene_end", applyAt.Add(time.Second), applyAt
		sceneValue, activityValue, locationValue = nil, nil, nil
	}
	inserted, err := callerTx.Exec(ctx, `INSERT INTO public.life_events(id,fluctlight_id,kind,start_at,end_at,scene,activity,location,status,revision,evidence_refs,idempotency_key,request_digest,result,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,1,$10,$11,$12,'{}',$13)`, eventID, fluctlightID, kind, startAt, endAt, sceneValue, activityValue, locationValue, status, jsonBytes(plan.EvidenceRefs), plan.IdempotencyKey, plan.RequestDigest, expiresAt)
	if err != nil {
		return failedCapabilityResult(invocation, "scene_persist_failed", true), err
	}
	if inserted.RowsAffected() != 1 {
		return failedCapabilityResult(invocation, "scene_persist_failed", true), ErrConflict
	}
	_, resultingContext, err := resolveLifeContextSnapshotWith(ctx, callerTx, fluctlightID, applyAt)
	if err != nil {
		return failedCapabilityResult(invocation, "scene_persist_failed", true), err
	}
	if plan.Operation != "end" && stringValue(resultingContext["event_id"]) != eventID {
		return failedCapabilityResult(invocation, "scene_resulting_context_invalid", false), newCapabilityError("scene_resulting_context_invalid", false, ErrConflict)
	}
	resultStatus := status
	if plan.Operation == "end" {
		resultStatus = "ended"
	}
	nativeFactKey := "scene:" + stableDigest(plan.IdempotencyKey)
	inboxID, err := a.enqueueNativeFactTx(ctx, callerTx, fluctlightID, conversationID, sourceFactID, "life.scene.updated", nativeFactKey, map[string]any{
		"event_id": eventID, "operation": plan.Operation, "scene": plan.Scene, "activity": plan.Activity, "location": plan.Location,
		"previous_scene": previousScene, "status": resultStatus, "expected_context_revision": plan.ExpectedLifeContextRevision,
		"resulting_context_revision": resultingContext["context_revision"],
	})
	if err != nil {
		return failedCapabilityResult(invocation, "scene_persist_failed", true), err
	}
	output := map[string]any{
		"event_id": eventID, "inbox_id": inboxID, "operation": plan.Operation, "status": resultStatus,
		"event_revision": 1, "expected_context_revision": plan.ExpectedLifeContextRevision,
		"resulting_context_revision": resultingContext["context_revision"], "replayed": false,
	}
	if _, err := callerTx.Exec(ctx, `UPDATE public.life_events SET result=$2 WHERE id=$1 AND revision=1`, eventID, jsonBytes(output)); err != nil {
		return failedCapabilityResult(invocation, "scene_persist_failed", true), err
	}
	if err := appendOutboxTx(ctx, callerTx, "life.scene.updated", "fluctlight", fluctlightID, fluctlightID, sourceFactID, "scene:"+eventID, "scene:"+fluctlightID+":"+stableDigest(plan.IdempotencyKey), map[string]any{"event_id": eventID, "operation": plan.Operation, "status": resultStatus, "aggregate_sequence": 1}); err != nil {
		return failedCapabilityResult(invocation, "scene_persist_failed", true), err
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: output, ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "scene:" + eventID}, nil
}

func normalizeSceneOperation(args map[string]any) (string, error) {
	operation := strings.TrimSpace(stringValue(args["operation"]))
	if operation == "" {
		return "", errors.New("operation is required and must be start, switch, or end")
	}
	if operation != "start" && operation != "switch" && operation != "end" {
		return "", errors.New("operation must be start, switch, or end")
	}
	return operation, nil
}

func (a *App) applyPresenceCapability(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	return failedCapabilityResult(invocation, "caller_transaction_required", false), newCapabilityError("caller_transaction_required", false, ErrConflict)
}

func (a *App) applyPresenceCapabilityTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	return a.applyPresenceCapabilityWithTx(ctx, tx, invocation, resolved)
}

func (a *App) applyPresenceCapabilityWithTx(ctx context.Context, callerTx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	if callerTx == nil {
		return failedCapabilityResult(invocation, "caller_transaction_required", false), newCapabilityError("caller_transaction_required", false, ErrConflict)
	}
	plan, err := presencePlanFromInvocation(invocation)
	if err != nil {
		return failedCapabilityResultDetail(invocation, "presence_plan_invalid", false, err.Error()), err
	}
	if resolved.Life == nil || stringValue(resolved.Life.Data["context_revision"]) != plan.ExpectedLifeContextRevision {
		return failedCapabilityResult(invocation, "presence_prepared_context_mismatch", false), newCapabilityError("presence_prepared_context_mismatch", false, ErrConflict)
	}
	fluctlightID, conversationID, sourceFactID := invocation.Metadata.FluctlightID, invocation.Metadata.ConversationID, invocation.SourceFactID
	overlayID := "presence_overlay_" + stableDigest(fluctlightID+":"+plan.IdempotencyKey)
	if err := lockLifeContextTx(ctx, callerTx, fluctlightID); err != nil {
		return failedCapabilityResult(invocation, "presence_persist_failed", true), err
	}
	var existingDigest string
	var existingResult []byte
	if replayErr := callerTx.QueryRow(ctx, `SELECT COALESCE(request_digest,''),result FROM public.life_presence_overlays WHERE fluctlight_id=$1 AND idempotency_key=$2 FOR UPDATE`, fluctlightID, plan.IdempotencyKey).Scan(&existingDigest, &existingResult); replayErr == nil {
		if existingDigest != plan.RequestDigest {
			return failedCapabilityResult(invocation, "presence_idempotency_conflict", false), newCapabilityError("presence_idempotency_conflict", false, ErrConflict)
		}
		output := decodeObject(existingResult)
		if len(output) == 0 {
			return failedCapabilityResult(invocation, "presence_replay_result_invalid", false), errors.New("presence replay result invalid")
		}
		output["replayed"] = true
		return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: output, ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "presence:" + overlayID}, nil
	} else if !errors.Is(replayErr, pgx.ErrNoRows) {
		return failedCapabilityResult(invocation, "presence_persist_failed", true), replayErr
	}
	applyAt := time.Now().UTC()
	if plan.Operation == "set" && (plan.ExpiresAt == nil || !plan.ExpiresAt.After(applyAt)) {
		return failedCapabilityResult(invocation, "presence_plan_expired", false), newCapabilityError("presence_plan_expired", false, ErrConflict)
	}
	if plan.Operation == "clear" && !plan.OccurredAt.Add(presenceDefaultDuration).After(applyAt) {
		return failedCapabilityResult(invocation, "presence_plan_expired", false), newCapabilityError("presence_plan_expired", false, ErrConflict)
	}
	if _, err := a.requireLifeContextRevisionTx(ctx, callerTx, fluctlightID, plan.ExpectedLifeContextRevision, applyAt); err != nil {
		if errors.Is(err, ErrLifeContextStale) {
			return failedCapabilityResult(invocation, "presence_context_stale", false), newCapabilityError("presence_context_stale", false, err)
		}
		return failedCapabilityResult(invocation, "presence_persist_failed", true), err
	}
	if _, err := callerTx.Exec(ctx, `UPDATE public.life_presence_overlays SET status='superseded',revision=revision+1,superseded_by_overlay_id=$2,updated_at=$3 WHERE fluctlight_id=$1 AND status='active'`, fluctlightID, overlayID, applyAt); err != nil {
		return failedCapabilityResult(invocation, "presence_persist_failed", true), err
	}
	status := "active"
	if plan.Operation == "clear" {
		status = "cleared"
	}
	inserted, err := callerTx.Exec(ctx, `INSERT INTO public.life_presence_overlays(id,fluctlight_id,actor_id,scene,activity,location,current_task,user_presence,status,revision,idempotency_key,request_digest,result,expires_at,created_at,updated_at) VALUES($1,$2,$3,NULL,NULL,NULL,$4,$5,$6,1,$7,$8,'{}',$9,$10,$10)`, overlayID, fluctlightID, plan.ActorID, nullableString(plan.CurrentTask), nullableString(plan.UserPresence), status, plan.IdempotencyKey, plan.RequestDigest, plan.ExpiresAt, applyAt)
	if err != nil {
		return failedCapabilityResult(invocation, "presence_persist_failed", true), err
	}
	if inserted.RowsAffected() != 1 {
		return failedCapabilityResult(invocation, "presence_persist_failed", true), ErrConflict
	}
	_, resultingContext, err := resolveLifeContextSnapshotWith(ctx, callerTx, fluctlightID, applyAt)
	if err != nil {
		return failedCapabilityResult(invocation, "presence_persist_failed", true), err
	}
	resultingPresence := mapValue(resultingContext["presence"])
	if (plan.Operation == "set" && stringValue(resultingPresence["id"]) != overlayID) || (plan.Operation == "clear" && len(resultingPresence) != 0) {
		return failedCapabilityResult(invocation, "presence_resulting_context_invalid", false), newCapabilityError("presence_resulting_context_invalid", false, ErrConflict)
	}
	nativeFactKey := "presence:" + stableDigest(plan.IdempotencyKey)
	inboxID, err := a.enqueueNativeFactTx(ctx, callerTx, fluctlightID, conversationID, sourceFactID, "life.presence.updated", nativeFactKey, map[string]any{
		"overlay_id": overlayID, "operation": plan.Operation, "current_task": plan.CurrentTask, "user_presence": plan.UserPresence,
		"expected_context_revision": plan.ExpectedLifeContextRevision, "resulting_context_revision": resultingContext["context_revision"],
	})
	if err != nil {
		return failedCapabilityResult(invocation, "presence_persist_failed", true), err
	}
	output := map[string]any{
		"overlay_id": overlayID, "inbox_id": inboxID, "operation": plan.Operation, "status": status,
		"overlay_revision": 1, "expected_context_revision": plan.ExpectedLifeContextRevision,
		"resulting_context_revision": resultingContext["context_revision"], "replayed": false,
	}
	if _, err := callerTx.Exec(ctx, `UPDATE public.life_presence_overlays SET result=$2 WHERE id=$1 AND revision=1`, overlayID, jsonBytes(output)); err != nil {
		return failedCapabilityResult(invocation, "presence_persist_failed", true), err
	}
	if err := appendOutboxTx(ctx, callerTx, "life.presence.updated", "fluctlight", fluctlightID, fluctlightID, sourceFactID, "presence:"+overlayID, "presence:"+fluctlightID+":"+stableDigest(plan.IdempotencyKey), map[string]any{"overlay_id": overlayID, "operation": plan.Operation, "status": status, "aggregate_sequence": 1}); err != nil {
		return failedCapabilityResult(invocation, "presence_persist_failed", true), err
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: output, ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "presence:" + overlayID}, nil
}

func (a *App) enqueueNativeFactTx(ctx context.Context, tx pgx.Tx, fluctlightID, conversationID, sourceFactID, eventType, idempotency string, candidate map[string]any) (string, error) {
	inboxID := "inbox_" + stableDigest("native:"+fluctlightID+":"+idempotency)
	var existing string
	if err := tx.QueryRow(ctx, `SELECT id FROM public.cognition_inbox WHERE fluctlight_id=$1 AND idempotency_key=$2`, fluctlightID, idempotency).Scan(&existing); err == nil {
		return existing, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_inbox_heads(fluctlight_id,next_sequence,last_processed_sequence) VALUES($1,1,0) ON CONFLICT DO NOTHING`, fluctlightID); err != nil {
		return "", err
	}
	var sequence int
	if err := tx.QueryRow(ctx, `SELECT next_sequence FROM public.cognition_inbox_heads WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&sequence); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `UPDATE public.cognition_inbox_heads SET next_sequence=$2 WHERE fluctlight_id=$1`, fluctlightID, sequence+1); err != nil {
		return "", err
	}
	depth := 0
	var parentPayload []byte
	if err := tx.QueryRow(ctx, `SELECT payload FROM public.cognition_inbox WHERE id=$1 AND fluctlight_id=$2`, sourceFactID, fluctlightID).Scan(&parentPayload); err == nil {
		depth = intValue(decodeObject(parentPayload)["native_cognition_depth"]) + 1
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	payload := map[string]any{"event_type": eventType, "fluctlight_id": fluctlightID, "conversation_id": conversationID, "source_fact_id": sourceFactID, "candidate": candidate, "idempotency_key": idempotency, "native_cognition_depth": depth}
	if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,now(),'pending')`, inboxID, fluctlightID, sequence, eventType, jsonBytes(payload), sourceFactID, eventType+":"+idempotency, idempotency); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'interaction','cognition.processing',$3) ON CONFLICT DO NOTHING`, "cognition_intent:"+inboxID, "cognition:"+inboxID, jsonBytes(map[string]any{"inbox_id": inboxID, "fluctlight_id": fluctlightID})); err != nil {
		return "", err
	}
	return inboxID, nil
}

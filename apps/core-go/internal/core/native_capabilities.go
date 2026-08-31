package core

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func sceneCapabilityManifest() CapabilityManifest {
	return CapabilityManifest{
		Name: "scene_event", Version: "v1",
		Description: "Record an evidence-backed scene/activity/location candidate for the Fluctlight life world.",
		Parameters: map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []any{"scene", "activity", "evidence_refs", "confidence"},
			"properties": map[string]any{
				"scene":    map[string]any{"type": "string", "minLength": 1, "maxLength": 512},
				"activity": map[string]any{"type": "string", "minLength": 1, "maxLength": 512},
				"location": map[string]any{"type": "string", "maxLength": 512},
				"kind":     map[string]any{"type": "string", "enum": []any{"confirmed", "observed", "inferred", "hypothesis"}},
				"start_at": map[string]any{"type": "string"}, "end_at": map[string]any{"type": "string"},
				"source_fact_id":  map[string]any{"type": "string", "minLength": 1},
				"evidence_refs":   map[string]any{"type": "array", "minItems": 1},
				"confidence":      map[string]any{"type": "number", "minimum": 0, "maximum": 1},
				"idempotency_key": map[string]any{"type": "string", "minLength": 1, "maxLength": 256},
			},
		},
		SideEffectClass: "native_projection", ConcurrencyClass: "exclusive", SupportsCancel: false, SupportsRetry: true, RequiresPreflight: false,
	}
}

func presenceCapabilityManifest() CapabilityManifest {
	return CapabilityManifest{
		Name: "presence_event", Version: "v1",
		Description: "Record a bounded temporary interaction presence overlay.",
		Parameters: map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []any{"evidence_refs", "confidence"},
			"properties": map[string]any{
				"user_presence":   map[string]any{"type": "string", "maxLength": 128},
				"current_task":    map[string]any{"type": "string", "maxLength": 512},
				"expires_at":      map[string]any{"type": "string"},
				"source_fact_id":  map[string]any{"type": "string", "minLength": 1},
				"evidence_refs":   map[string]any{"type": "array", "minItems": 1},
				"confidence":      map[string]any{"type": "number", "minimum": 0, "maximum": 1},
				"idempotency_key": map[string]any{"type": "string", "minLength": 1, "maxLength": 256},
			},
		},
		SideEffectClass: "native_projection", ConcurrencyClass: "exclusive", SupportsCancel: false, SupportsRetry: true, RequiresPreflight: false,
	}
}

func (a *App) applySceneCapability(ctx context.Context, fluctlightID, conversationID, sourceFactID string, call ToolCallV1) (ToolResultV1, error) {
	var args map[string]any
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return failedToolResult(call, "scene_arguments_invalid", false, err.Error()), err
	}
	if source := stringValue(args["source_fact_id"]); source != "" && source != sourceFactID {
		return failedToolResult(call, "scene_source_invalid", false, "source fact does not match current cognition fact"), errors.New("scene source fact invalid")
	}
	scene, activity := strings.TrimSpace(stringValue(args["scene"])), strings.TrimSpace(stringValue(args["activity"]))
	if scene == "" || activity == "" {
		return failedToolResult(call, "scene_fields_required", false, "scene and activity are required"), errors.New("scene and activity are required")
	}
	confidence, err := boundedNumberOrError(args["confidence"], -1)
	if err != nil || confidence < 0 {
		return failedToolResult(call, "scene_confidence_invalid", false, "confidence must be between 0 and 1"), errors.New("scene confidence invalid")
	}
	refs := arrayValue(args["evidence_refs"])
	if len(refs) == 0 {
		refs = []any{sourceFactID}
	}
	if !containsStringValue(refs, sourceFactID) {
		return failedToolResult(call, "scene_evidence_invalid", false, "evidence must include the current source fact"), errors.New("scene evidence invalid")
	}
	idempotency := stringValue(args["idempotency_key"])
	if idempotency == "" {
		idempotency = "tool:" + call.ID
	}
	start, end, err := capabilityTimeBounds(args["start_at"], args["end_at"])
	if err != nil {
		return failedToolResult(call, "scene_time_invalid", false, err.Error()), err
	}
	kind := firstString(args["kind"], "inferred")
	status := "inferred"
	if kind == "confirmed" || kind == "observed" {
		status = "confirmed"
	}
	eventID := "event_" + stableDigest(fluctlightID+":"+idempotency)
	inboxID := ""
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO public.life_events(id,fluctlight_id,kind,start_at,end_at,scene,activity,location,status,evidence_refs,idempotency_key,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) ON CONFLICT(fluctlight_id,idempotency_key) DO NOTHING`, eventID, fluctlightID, "scene_"+kind, start, end, scene, nullableString(stringValue(args["activity"])), nullableString(stringValue(args["location"])), status, jsonBytes(refs), idempotency, sceneExpiry(status, end)); err != nil {
			return err
		}
		inboxID, err = a.enqueueNativeFactTx(ctx, tx, fluctlightID, conversationID, sourceFactID, "life.scene.updated", "scene:"+idempotency, map[string]any{"event_id": eventID, "scene": scene, "activity": activity, "location": stringValue(args["location"]), "status": status})
		if err != nil {
			return err
		}
		return appendOutboxTx(ctx, tx, "life.scene.updated", "fluctlight", fluctlightID, fluctlightID, sourceFactID, "scene:"+eventID, "scene:"+idempotency, map[string]any{"event_id": eventID, "status": status, "aggregate_sequence": 1})
	})
	if err != nil {
		return failedToolResult(call, "scene_persist_failed", true, err.Error()), err
	}
	return ToolResultV1{ToolCallID: call.ID, Name: call.Name, Status: "completed", Output: map[string]any{"event_id": eventID, "inbox_id": inboxID, "status": status}, ProviderRequestID: call.ProviderRequestID, CorrelationID: "scene:" + eventID, SchemaVersion: ToolResultSchemaVersion}, nil
}

func (a *App) applyPresenceCapability(ctx context.Context, fluctlightID, conversationID, sourceFactID string, call ToolCallV1) (ToolResultV1, error) {
	var args map[string]any
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return failedToolResult(call, "presence_arguments_invalid", false, err.Error()), err
	}
	if source := stringValue(args["source_fact_id"]); source != "" && source != sourceFactID {
		return failedToolResult(call, "presence_source_invalid", false, "source fact does not match current cognition fact"), errors.New("presence source fact invalid")
	}
	userPresence := strings.TrimSpace(stringValue(args["user_presence"]))
	currentTask := strings.TrimSpace(stringValue(args["current_task"]))
	if userPresence == "" && currentTask == "" {
		return failedToolResult(call, "presence_fields_required", false, "user_presence or current_task is required"), errors.New("presence fields are required")
	}
	confidence, err := boundedNumberOrError(args["confidence"], -1)
	if err != nil || confidence < 0 {
		return failedToolResult(call, "presence_confidence_invalid", false, "confidence must be between 0 and 1"), errors.New("presence confidence invalid")
	}
	refs := arrayValue(args["evidence_refs"])
	if len(refs) == 0 {
		refs = []any{sourceFactID}
	}
	if !containsStringValue(refs, sourceFactID) {
		return failedToolResult(call, "presence_evidence_invalid", false, "evidence must include the current source fact"), errors.New("presence evidence invalid")
	}
	idempotency := stringValue(args["idempotency_key"])
	if idempotency == "" {
		idempotency = "tool:" + call.ID
	}
	var expires any
	if raw := stringValue(args["expires_at"]); raw != "" {
		parsed, parseErr := time.Parse(time.RFC3339, raw)
		if parseErr != nil || !parsed.After(time.Now().UTC()) {
			return failedToolResult(call, "presence_expiration_invalid", false, "expires_at must be a future RFC3339 timestamp"), errors.New("presence expiration invalid")
		}
		expires = parsed
	} else {
		expires = time.Now().UTC().Add(2 * time.Hour)
	}
	overlayID := "presence_overlay_" + stableDigest(fluctlightID+":"+idempotency)
	inboxID := ""
	if err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO public.life_presence_overlays(id,fluctlight_id,actor_id,scene,activity,location,current_task,user_presence,expires_at) VALUES($1,$2,$2,NULL,NULL,NULL,$3,$4,$5) ON CONFLICT(id) DO NOTHING`, overlayID, fluctlightID, nullableString(currentTask), nullableString(userPresence), expires); err != nil {
			return err
		}
		var err error
		inboxID, err = a.enqueueNativeFactTx(ctx, tx, fluctlightID, conversationID, sourceFactID, "life.presence.updated", "presence:"+idempotency, map[string]any{"overlay_id": overlayID, "current_task": currentTask, "user_presence": userPresence})
		if err != nil {
			return err
		}
		return appendOutboxTx(ctx, tx, "life.presence.updated", "fluctlight", fluctlightID, fluctlightID, sourceFactID, "presence:"+overlayID, "presence:"+idempotency, map[string]any{"overlay_id": overlayID, "aggregate_sequence": 1})
	}); err != nil {
		return failedToolResult(call, "presence_persist_failed", true, err.Error()), err
	}
	return ToolResultV1{ToolCallID: call.ID, Name: call.Name, Status: "completed", Output: map[string]any{"overlay_id": overlayID, "inbox_id": inboxID, "status": "active"}, ProviderRequestID: call.ProviderRequestID, CorrelationID: "presence:" + overlayID, SchemaVersion: ToolResultSchemaVersion}, nil
}

func capabilityTimeBounds(startValue, endValue any) (time.Time, time.Time, error) {
	now := time.Now().UTC()
	start := now
	if raw := stringValue(startValue); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("start_at must be RFC3339")
		}
		start = parsed
	}
	end := start.Add(30 * time.Minute)
	if raw := stringValue(endValue); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("end_at must be RFC3339")
		}
		end = parsed
	}
	if !end.After(start) {
		return time.Time{}, time.Time{}, errors.New("end_at must be after start_at")
	}
	return start, end, nil
}

func sceneExpiry(status string, end time.Time) any {
	if status == "confirmed" {
		return nil
	}
	return end.Add(24 * time.Hour)
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
	payload := map[string]any{"event_type": eventType, "fluctlight_id": fluctlightID, "conversation_id": conversationID, "source_fact_id": sourceFactID, "candidate": candidate, "idempotency_key": idempotency}
	if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,now(),'pending')`, inboxID, fluctlightID, sequence, eventType, jsonBytes(payload), sourceFactID, eventType+":"+idempotency, idempotency); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'interaction','cognition.processing',$3) ON CONFLICT DO NOTHING`, "cognition_intent:"+inboxID, "cognition:"+inboxID, jsonBytes(map[string]any{"inbox_id": inboxID, "fluctlight_id": fluctlightID})); err != nil {
		return "", err
	}
	return inboxID, nil
}

func (a *App) markNativeFactProcessed(ctx context.Context, inboxID string) error {
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var fluctlightID string
		var sequence int
		if err := tx.QueryRow(ctx, `SELECT fluctlight_id,sequence FROM public.cognition_inbox WHERE id=$1 FOR UPDATE`, inboxID).Scan(&fluctlightID, &sequence); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.cognition_inbox SET status='processed',processed_at=now() WHERE id=$1 AND status <> 'processed'`, inboxID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE public.cognition_inbox_heads SET last_processed_sequence=GREATEST(last_processed_sequence,$2) WHERE fluctlight_id=$1`, fluctlightID, sequence)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'lifecycle','reflection.run',$3) ON CONFLICT DO NOTHING`, "reflection_intent:native:"+inboxID, "reflection:native:"+inboxID, jsonBytes(map[string]any{"fluctlight_id": fluctlightID, "source_fact_id": inboxID}))
		return err
	})
}

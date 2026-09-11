package core

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func capabilityRequestDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name:          "capability.request",
		Version:       "v1",
		Type:          CapabilityTypeInternal,
		Description:   "Record a request for a missing capability for Owner review; this does not execute an external side effect.",
		Surfaces:      []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceAutonomy, CapabilitySurfaceNativeCognition},
		FailurePolicy: FailurePolicyOptionalInternal,
		InputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []any{"capability_key", "title", "description", "rationale"},
			"properties": map[string]any{
				"capability_key":   map[string]any{"type": "string", "pattern": "^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$"},
				"title":            map[string]any{"type": "string", "minLength": 1, "maxLength": 256},
				"description":      map[string]any{"type": "string", "minLength": 1, "maxLength": 4000},
				"rationale":        map[string]any{"type": "string", "minLength": 1, "maxLength": 4000},
				"desired_contract": map[string]any{"type": "object", "additionalProperties": true},
				"priority":         map[string]any{"type": "string", "enum": []any{"low", "normal", "high", "urgent"}},
			},
		},
		SideEffectClass: "native_projection", SuccessBoundary: "capability_request_recorded", ConcurrencyClass: "exclusive", SupportsCancel: false, SupportsRetry: true, RequiresPreflight: false,
		ProvenanceFields: []string{"evidence_refs", "idempotency_key"},
	}
}

type capabilityRequestService struct{ app *App }

func (service *capabilityRequestService) execute(ctx context.Context, invocation CapabilityInvocation) (CapabilityResult, error) {
	return service.executeWith(ctx, nil, invocation)
}

func (service *capabilityRequestService) executeWith(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation) (CapabilityResult, error) {
	fluctlightID, sourceFactID := invocation.Metadata.FluctlightID, invocation.SourceFactID
	args, err := capabilityExecutionArguments(invocation, capabilityRequestDefinition())
	if err != nil {
		return failedCapabilityResultDetail(invocation, "capability_request_arguments_invalid", false, err.Error()), err
	}
	key := strings.TrimSpace(stringValue(args["capability_key"]))
	if !validateSlotKey(key) {
		return failedCapabilityResultDetail(invocation, "capability_request_key_invalid", false, "capability_key is invalid"), errors.New("capability request key invalid")
	}
	title, err := boundedRequiredText(args["title"], 256)
	if err != nil {
		return failedCapabilityResultDetail(invocation, "capability_request_title_invalid", false, err.Error()), err
	}
	description, err := boundedRequiredText(args["description"], 4000)
	if err != nil {
		return failedCapabilityResultDetail(invocation, "capability_request_description_invalid", false, err.Error()), err
	}
	rationale, err := boundedRequiredText(args["rationale"], 4000)
	if err != nil {
		return failedCapabilityResultDetail(invocation, "capability_request_rationale_invalid", false, err.Error()), err
	}
	desiredContract := mapValue(args["desired_contract"])
	if desiredContract == nil {
		desiredContract = map[string]any{}
	}
	if len(jsonBytes(desiredContract)) > 16000 || containsSensitiveKey(desiredContract) {
		return failedCapabilityResultDetail(invocation, "capability_request_contract_invalid", false, "desired_contract must be a bounded object"), errors.New("capability request contract invalid")
	}
	priority := stringValue(args["priority"])
	if priority == "" {
		priority = "normal"
	}
	if priority != "low" && priority != "normal" && priority != "high" && priority != "urgent" {
		return failedCapabilityResultDetail(invocation, "capability_request_priority_invalid", false, "priority is invalid"), errors.New("capability request priority invalid")
	}
	sideEffectClass := stringValue(args["side_effect_class"])
	if sideEffectClass == "" {
		sideEffectClass = "unknown"
	}
	if len([]rune(sideEffectClass)) > 64 {
		return failedCapabilityResultDetail(invocation, "capability_request_side_effect_invalid", false, "side_effect_class is too long"), errors.New("capability request side effect invalid")
	}
	refs := arrayValue(args["evidence_refs"])
	if len(refs) == 0 || len(refs) > 20 {
		// The source fact is runtime-owned and is sufficient evidence when the
		// provider omitted optional evidence references.
		refs = []any{sourceFactID}
	}
	if !containsStringValue(refs, sourceFactID) {
		refs = append(refs, sourceFactID)
	}
	for _, ref := range refs {
		if text := stringValue(ref); text == "" || len([]rune(text)) > 256 {
			return failedCapabilityResultDetail(invocation, "capability_request_evidence_invalid", false, "evidence reference is invalid"), errors.New("capability request evidence invalid")
		}
	}
	idempotency := stringValue(args["idempotency_key"])
	if idempotency == "" {
		idempotency = "tool:" + invocation.CallID
	}
	if len([]rune(idempotency)) > 256 {
		return failedCapabilityResultDetail(invocation, "capability_request_idempotency_invalid", false, "idempotency_key is too long"), errors.New("capability request idempotency invalid")
	}
	requestID := "capability_request_" + stableDigest(fluctlightID+":"+idempotency)
	if service == nil || service.app == nil {
		return failedCapabilityResultDetail(invocation, "capability_request_unavailable", true, "capability request service is unavailable"), errors.New("capability request service unavailable")
	}
	if tx != nil {
		err = service.app.persistCapabilityRequestTx(ctx, tx, requestID, fluctlightID, sourceFactID, key, title, description, rationale, desiredContract, sideEffectClass, priority, refs, idempotency)
	} else {
		err = service.app.persistCapabilityRequest(ctx, requestID, fluctlightID, sourceFactID, key, title, description, rationale, desiredContract, sideEffectClass, priority, refs, idempotency)
	}
	if err != nil {
		return failedCapabilityResultDetail(invocation, "capability_request_persist_failed", true, err.Error()), err
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: map[string]any{"request_id": requestID, "capability_key": key, "status": "proposed"}, Retryable: false, ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "capability-request:" + requestID}, nil
}

func containsSensitiveKey(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", ""), "_", ""))
			if normalized == "password" || normalized == "token" || normalized == "secret" || normalized == "apikey" || normalized == "credential" || normalized == "script" || normalized == "code" {
				return true
			}
			if containsSensitiveKey(nested) {
				return true
			}
		}
	case []any:
		for _, nested := range typed {
			if containsSensitiveKey(nested) {
				return true
			}
		}
	}
	return false
}

func jsonUnmarshalObject(data []byte, target *map[string]any) error {
	if len(data) == 0 || len(data) > maxToolArgumentsBytes {
		return errors.New("arguments must be bounded")
	}
	if err := json.Unmarshal(data, target); err != nil || *target == nil {
		return errors.New("arguments must be an object")
	}
	return nil
}

func boundedRequiredText(value any, max int) (string, error) {
	text := strings.TrimSpace(stringValue(value))
	if text == "" || len([]rune(text)) > max {
		return "", errors.New("text is empty or too long")
	}
	return text, nil
}

func (a *App) persistCapabilityRequest(ctx context.Context, requestID, fluctlightID, sourceFactID, key, title, description, rationale string, desiredContract map[string]any, sideEffectClass, priority string, refs []any, idempotency string) error {
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		return a.persistCapabilityRequestTx(ctx, tx, requestID, fluctlightID, sourceFactID, key, title, description, rationale, desiredContract, sideEffectClass, priority, refs, idempotency)
	})
}

func (a *App) persistCapabilityRequestTx(ctx context.Context, tx pgx.Tx, requestID, fluctlightID, sourceFactID, key, title, description, rationale string, desiredContract map[string]any, sideEffectClass, priority string, refs []any, idempotency string) error {
	if _, err := tx.Exec(ctx, `INSERT INTO public.capability_requests(id,capability_key,title,description,rationale,desired_contract,side_effect_class,priority,fluctlight_id,source_fact_id,evidence_refs,status,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'proposed',$12) ON CONFLICT(fluctlight_id,idempotency_key) DO NOTHING`, requestID, key, title, description, rationale, jsonBytes(desiredContract), sideEffectClass, priority, fluctlightID, sourceFactID, jsonBytes(refs), idempotency); err != nil {
		return err
	}
	return appendOutboxTx(ctx, tx, "capability.requested", "capability_request", requestID, fluctlightID, sourceFactID, "capability-request:"+requestID, "capability-request:"+requestID, map[string]any{"request_id": requestID, "capability_key": key, "status": "proposed"})
}

func (a *App) ListCapabilityRequests(ctx context.Context, actorID string) ([]map[string]any, error) {
	if _, err := a.ReadSettings(ctx, actorID); err != nil {
		return nil, err
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT r.id,r.capability_key,r.title,r.description,r.rationale,r.desired_contract,r.side_effect_class,r.priority,r.fluctlight_id,r.source_fact_id,r.evidence_refs,r.status,r.review_note,r.reviewer_actor_id,r.capability_version,r.created_at,r.updated_at,COUNT(*) OVER(PARTITION BY r.capability_key) FROM public.capability_requests r ORDER BY CASE r.status WHEN 'proposed' THEN 0 WHEN 'reviewing' THEN 1 WHEN 'accepted' THEN 2 WHEN 'fulfilled' THEN 3 ELSE 4 END,r.updated_at DESC,r.id LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var id, key, title, description, rationale, sideEffect, priority, fluctlightID, sourceFactID, status string
		var desired, refs []byte
		var reviewNote, reviewer, version *string
		var created, updated time.Time
		var aggregateCount int
		if err := rows.Scan(&id, &key, &title, &description, &rationale, &desired, &sideEffect, &priority, &fluctlightID, &sourceFactID, &refs, &status, &reviewNote, &reviewer, &version, &created, &updated, &aggregateCount); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{"id": id, "capability_key": key, "title": title, "description": description, "rationale": rationale, "desired_contract": decodeObject(desired), "side_effect_class": sideEffect, "priority": priority, "fluctlight_id": fluctlightID, "source_fact_id": sourceFactID, "evidence_refs": decodeArray(refs), "status": status, "review_note": reviewNote, "reviewer_actor_id": reviewer, "capability_version": version, "aggregate_count": aggregateCount, "created_at": created.Format(time.RFC3339Nano), "updated_at": updated.Format(time.RFC3339Nano)})
	}
	return result, rows.Err()
}

func (a *App) ReviewCapabilityRequest(ctx context.Context, actorID, requestID, status, note, capabilityVersion string) (map[string]any, error) {
	if requestID == "" {
		return nil, errors.New("capability_request_id_required")
	}
	if status != "reviewing" && status != "accepted" && status != "rejected" && status != "fulfilled" && status != "cancelled" {
		return nil, errors.New("capability_request_status_invalid")
	}
	if strings.TrimSpace(note) == "" || len([]rune(note)) > 2000 || (status == "fulfilled" && strings.TrimSpace(capabilityVersion) == "") {
		return nil, errors.New("capability_request_review_invalid")
	}
	if _, err := a.ReadSettings(ctx, actorID); err != nil {
		return nil, err
	}
	var result map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var fluctlightID, previous, capabilityKey string
		if err := tx.QueryRow(ctx, `SELECT fluctlight_id,status,capability_key FROM public.capability_requests WHERE id=$1 FOR UPDATE`, requestID).Scan(&fluctlightID, &previous, &capabilityKey); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if status == "fulfilled" {
			capability, registered := a.capabilityRegistry().Lookup(capabilityKey)
			if !registered {
				return errors.New("capability_request_capability_unavailable")
			}
			if capabilityVersionValue := capability.Definition().Version; capabilityVersion != "" && capabilityVersionValue != capabilityVersion {
				return errors.New("capability_request_capability_version_mismatch")
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE public.capability_requests SET status=$2,review_note=$3,reviewer_actor_id=$4,capability_version=CASE WHEN $5='' THEN capability_version ELSE $5 END,updated_at=now() WHERE id=$1`, requestID, status, nullableString(note), actorID, capabilityVersion); err != nil {
			return err
		}
		if err := appendOutboxTx(ctx, tx, "capability.request.reviewed", "capability_request", requestID, fluctlightID, actorID, "capability-request-review:"+requestID, "capability-request-review:"+requestID+":"+status, map[string]any{"request_id": requestID, "from_status": previous, "to_status": status, "capability_version": capabilityVersion}); err != nil {
			return err
		}
		result = map[string]any{"id": requestID, "status": status, "from_status": previous, "review_note": note, "capability_version": nullableString(capabilityVersion)}
		return nil
	})
	return result, err
}

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

func scheduleReplanCapabilityManifest() CapabilityManifest {
	item := map[string]any{
		"type": "object", "additionalProperties": false,
		"required": []any{"start_at", "end_at", "activity", "scene", "item_type", "status", "priority", "flexibility", "interruption_cost"},
		"properties": map[string]any{
			"start_at":          map[string]any{"type": "string"},
			"end_at":            map[string]any{"type": "string"},
			"activity":          map[string]any{"type": "string", "minLength": 1, "maxLength": 512},
			"scene":             map[string]any{"type": "string", "minLength": 1, "maxLength": 512},
			"item_type":         map[string]any{"type": "string", "maxLength": 64},
			"status":            map[string]any{"type": "string", "maxLength": 32},
			"priority":          map[string]any{"type": "number", "minimum": 0, "maximum": 1},
			"flexibility":       map[string]any{"type": "number", "minimum": 0, "maximum": 1},
			"interruption_cost": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
		},
	}
	return CapabilityManifest{
		Name:        "schedule.replan",
		Version:     "v1",
		Description: "Replace the current day's schedule after a confirmed interruption or material scene change. Preserve all completed intervals exactly, and provide only a validated full-day replacement for the current and future intervals.",
		Parameters: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []any{"local_date", "expected_revision", "completed_before", "items", "evidence_refs"},
			"properties": map[string]any{
				"local_date":        map[string]any{"type": "string"},
				"timezone":          map[string]any{"type": "string", "maxLength": 128},
				"expected_revision": map[string]any{"type": "integer", "minimum": 0},
				"completed_before":  map[string]any{"type": "string"},
				"items":             map[string]any{"type": "array", "minItems": 1, "maxItems": 64, "items": item},
				"reschedule_policy": map[string]any{"type": "object"},
				"reason":            map[string]any{"type": "string", "maxLength": 2000},
				"evidence_refs":     map[string]any{"type": "array", "minItems": 1, "maxItems": 32, "items": map[string]any{"type": "string"}},
				"idempotency_key":   map[string]any{"type": "string", "maxLength": 256},
			},
		},
		OutputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []any{"schedule_id", "revision", "status"},
			"properties": map[string]any{
				"schedule_id":      map[string]any{"type": "string"},
				"revision":         map[string]any{"type": "integer"},
				"status":           map[string]any{"type": "string"},
				"completed_before": map[string]any{"type": "string"},
				"previous_version": map[string]any{"type": "string"},
			},
		},
		SideEffectClass: "native_projection", ConcurrencyClass: "exclusive", SupportsCancel: false, SupportsRetry: true,
	}
}

type scheduleReplanCapabilityExecutor struct{ app *App }

func (executor *scheduleReplanCapabilityExecutor) Manifest() CapabilityManifest {
	return scheduleReplanCapabilityManifest()
}

func (executor *scheduleReplanCapabilityExecutor) Execute(ctx context.Context, fluctlightID, conversationID, sourceFactID string, call ToolCallV1) (ToolResultV1, error) {
	return executor.app.applyScheduleReplanCapability(ctx, fluctlightID, conversationID, sourceFactID, call)
}

func (a *App) applyScheduleReplanCapability(ctx context.Context, fluctlightID, conversationID, sourceFactID string, call ToolCallV1) (ToolResultV1, error) {
	var args map[string]any
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return failedToolResult(call, "schedule_replan_arguments_invalid", false, err.Error()), err
	}
	items := arrayValue(args["items"])
	if len(items) == 0 {
		return failedToolResult(call, "schedule_replan_items_required", false, "items are required"), errors.New("schedule replan items are required")
	}
	if err := validateScheduleReplanItems(items); err != nil {
		return failedToolResult(call, "schedule_replan_items_invalid", false, err.Error()), err
	}
	current, err := a.currentAcceptedSchedule(ctx, fluctlightID)
	if err != nil {
		code := "schedule_replan_schedule_missing"
		if !errors.Is(err, pgx.ErrNoRows) {
			code = "schedule_replan_schedule_read_failed"
		}
		return failedToolResult(call, code, errors.Is(err, pgx.ErrNoRows) == false, err.Error()), err
	}
	ownerID := stringValue(current["owner_actor_id"])
	if ownerID == "" {
		return failedToolResult(call, "schedule_replan_owner_missing", false, "schedule owner is missing"), errors.New("schedule owner is missing")
	}
	payload := map[string]any{}
	for key, value := range args {
		payload[key] = value
	}
	if strings.TrimSpace(stringValue(payload["local_date"])) == "" || stringValue(payload["local_date"]) != stringValue(current["local_date"]) {
		return failedToolResult(call, "schedule_replan_local_date_invalid", false, "local_date must match the current local schedule date"), errors.New("schedule replan local date invalid")
	}
	if timezone := canonicalTimezone(stringValue(payload["timezone"])); timezone == "" || timezone != canonicalTimezone(stringValue(current["timezone"])) {
		return failedToolResult(call, "schedule_replan_timezone_invalid", false, "timezone must match the current schedule timezone"), errors.New("schedule replan timezone invalid")
	} else {
		payload["timezone"] = timezone
	}
	if _, ok := payload["expected_revision"]; !ok || intValue(payload["expected_revision"]) <= 0 || intValue(payload["expected_revision"]) != intValue(current["revision"]) {
		return failedToolResult(call, "schedule_replan_revision_invalid", true, "expected_revision must match the current accepted schedule"), ErrConflict
	}
	if strings.TrimSpace(stringValue(payload["completed_before"])) == "" {
		return failedToolResult(call, "schedule_replan_completed_before_required", false, "completed_before is required"), errors.New("completed_before is required")
	}
	if len(arrayValue(payload["evidence_refs"])) == 0 {
		return failedToolResult(call, "schedule_replan_evidence_required", false, "evidence_refs is required"), errors.New("evidence_refs is required")
	}
	if sourceFactID != "" && !containsStringValue(arrayValue(payload["evidence_refs"]), sourceFactID) {
		payload["evidence_refs"] = append(arrayValue(payload["evidence_refs"]), sourceFactID)
	}
	if mapValue(payload["reschedule_policy"]) == nil {
		payload["reschedule_policy"] = mapValue(current["reschedule_policy"])
	}
	payload["generated_from"] = "model_replan"
	payload["source_fact_id"] = sourceFactID
	payload["conversation_id"] = conversationID
	payload["trigger"] = "capability:schedule.replan"

	result, err := a.ReplanSchedule(ctx, ownerID, fluctlightID, payload)
	if err != nil {
		return failedToolResult(call, "schedule_replan_failed", errors.Is(err, ErrConflict), err.Error()), err
	}
	output := map[string]any{
		"schedule_id":      result["id"],
		"revision":         result["revision"],
		"status":           result["status"],
		"completed_before": payload["completed_before"],
	}
	if previous := stringValue(current["id"]); previous != "" {
		output["previous_version"] = previous
	}
	return ToolResultV1{ToolCallID: call.ID, Name: call.Name, Status: "completed", Output: output, ProviderRequestID: call.ProviderRequestID, CorrelationID: "schedule:" + stringValue(result["id"]), SchemaVersion: ToolResultSchemaVersion}, nil
}

// validateScheduleReplanItems is intentionally stricter than the generic tool
// envelope. The provider-facing manifest documents required item fields, but
// the common ToolCall validator only checks that the arguments are JSON. A
// replan must never silently turn a missing semantic field into AcceptSchedule's
// legacy default, otherwise the model could appear to have changed the plan
// while the persisted schedule loses its status/priority semantics.
func validateScheduleReplanItems(items []any) error {
	if len(items) == 0 || len(items) > 64 {
		return errors.New("schedule replan items must contain between 1 and 64 items")
	}
	for index, raw := range items {
		item := mapValue(raw)
		if len(item) == 0 {
			return fmt.Errorf("schedule replan item %d must be an object", index)
		}
		for _, field := range []string{"start_at", "end_at", "activity", "scene", "item_type", "status"} {
			if strings.TrimSpace(stringValue(item[field])) == "" {
				return fmt.Errorf("schedule replan item %d field %q is required", index, field)
			}
		}
		for _, field := range []string{"priority", "flexibility", "interruption_cost"} {
			rawValue, ok := item[field]
			if !ok || rawValue == nil {
				return fmt.Errorf("schedule replan item %d field %q is required", index, field)
			}
			value, ok := numberFloat(normalizeScheduleScalar(rawValue))
			if !ok || value < 0 || value > 1 {
				return fmt.Errorf("schedule replan item %d field %q must be a number between 0 and 1", index, field)
			}
		}
	}
	return nil
}

func (a *App) currentAcceptedSchedule(ctx context.Context, fluctlightID string) (map[string]any, error) {
	var ownerID, id, timezone string
	var localDate time.Time
	var revision int
	var policy []byte
	var identity []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT created_by_actor_id,identity FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&ownerID, &identity); err != nil {
		return nil, err
	}
	timezone = canonicalTimezone(stringValue(decodeObject(identity)["timezone"]))
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("schedule_timezone_invalid: %w", err)
	}
	currentDate := time.Now().In(location).Format("2006-01-02")
	if err := a.DB.Pool().QueryRow(ctx, `
		SELECT s.id,s.local_date,s.timezone,s.revision,s.reschedule_policy
		FROM public.life_schedules s
		WHERE s.fluctlight_id=$1 AND s.status='accepted' AND s.local_date=$2
		ORDER BY s.revision DESC
		LIMIT 1`, fluctlightID, currentDate).Scan(&id, &localDate, &timezone, &revision, &policy); err != nil {
		return nil, err
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,start_at,end_at,activity,scene,item_type,status,priority,flexibility,interruption_cost FROM public.life_schedule_items WHERE schedule_id=$1 ORDER BY start_at`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var itemID, activity, scene, itemType, itemStatus, priority, flexibility, interruptionCost string
		var start, end time.Time
		if err := rows.Scan(&itemID, &start, &end, &activity, &scene, &itemType, &itemStatus, &priority, &flexibility, &interruptionCost); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{
			"id": itemID, "start_at": start.Format(time.RFC3339Nano), "end_at": end.Format(time.RFC3339Nano),
			"activity": activity, "scene": scene, "item_type": itemType, "status": itemStatus,
			"priority": priority, "flexibility": flexibility, "interruption_cost": interruptionCost,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return map[string]any{
		"owner_actor_id":    ownerID,
		"id":                id,
		"local_date":        localDate.Format("2006-01-02"),
		"timezone":          timezone,
		"revision":          revision,
		"reschedule_policy": decodeJSONValue(policy),
		"items":             items,
	}, nil
}

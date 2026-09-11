package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func scheduleReplanCapabilityDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name:            "schedule.replan",
		Version:         "v1",
		Type:            CapabilityTypeAction,
		Description:     "Replan the current and future intervals of the local-day schedule.",
		Surfaces:        []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceAutonomy, CapabilitySurfaceNativeCognition},
		FailurePolicy:   FailurePolicyRequiredForVisibleClaim,
		RequiredContext: []ContextSlot{SlotSchedule, SlotCurrentLife, SlotAgency},
		InputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
			"required":   []any{"intent"},
			"properties": map[string]any{"intent": map[string]any{"type": "string", "minLength": 1, "maxLength": 4000}},
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
		SideEffectClass: "native_projection", SuccessBoundary: "schedule_version_committed", ConcurrencyClass: "exclusive", SupportsCancel: false, SupportsRetry: true,
	}
}

type SchedulePlanInput struct {
	Intent       string
	Schedule     map[string]any
	CurrentLife  map[string]any
	Agency       map[string]any
	SourceFactID string
	Timezone     string
}

type SchedulePlanner interface {
	Plan(context.Context, SchedulePlanInput) (map[string]any, error)
}

// providerSchedulePlanner is capability-local. Its strict planner schema is
// never included in a Main LLM catalog; the call happens before the schedule
// mutation transaction and returns a complete replacement DTO only.
type providerSchedulePlanner struct {
	provider *ProviderClient
}

func (planner providerSchedulePlanner) Plan(ctx context.Context, input SchedulePlanInput) (map[string]any, error) {
	if planner.provider == nil {
		return nil, errors.New("schedule_replan_planner_failed: provider unavailable")
	}
	messages := []map[string]any{
		{"role": "system", "content": "Return only a complete schedule replacement. Preserve completed history and use the supplied timezone and revision."},
		{"role": "user", "content": jsonString(map[string]any{
			"intent":       input.Intent,
			"schedule":     compactScheduleForProvider(input.Schedule),
			"current_life": compactLifeContext(input.CurrentLife),
			"agency":       compactSchedulePlannerAgency(input.Agency),
			"timezone":     input.Timezone,
		})},
	}
	return planner.provider.StructuredWithSchema(WithProviderScenario(ctx, "schedule_replan_planner"), "cognitive_assessment", messages, "schedule_replan_plan", schedulePlannerOutputSchema(), false)
}

func compactSchedulePlannerAgency(agency map[string]any) map[string]any {
	result := map[string]any{}
	toRows := func(value any) []map[string]any {
		rows := make([]map[string]any, 0)
		for _, raw := range arrayValue(value) {
			if row := mapValue(raw); len(row) > 0 {
				rows = append(rows, row)
			}
		}
		return rows
	}
	if goals := compactProviderGoals(toRows(agency["goals"])); len(goals) > 0 {
		result["goals"] = goals
	}
	if intentions := compactProviderIntentions(toRows(agency["intentions"])); len(intentions) > 0 {
		result["intentions"] = intentions
	}
	return result
}

func schedulePlannerOutputSchema() map[string]any {
	item := objectSchema(map[string]any{
		"start_at": stringSchema(), "end_at": stringSchema(), "activity": stringSchema(), "scene": stringSchema(), "location": stringSchema(),
		"item_type": stringSchema(), "status": stringSchema(), "priority": unitNumberSchema(), "flexibility": unitNumberSchema(), "interruption_cost": unitNumberSchema(),
	}, []string{"start_at", "end_at", "activity", "scene", "item_type", "status", "priority", "flexibility", "interruption_cost"}, false)
	return objectSchema(map[string]any{
		"local_date": stringSchema(), "timezone": stringSchema(), "expected_revision": integerSchema(),
		"completed_before": stringSchema(), "items": arraySchema(item), "reschedule_policy": openObjectSchema(),
	}, []string{"local_date", "timezone", "expected_revision", "completed_before", "items", "reschedule_policy"}, false)
}

func (a *App) applyScheduleReplanCapability(ctx context.Context, invocation CapabilityInvocation) (CapabilityResult, error) {
	return failedCapabilityResult(invocation, "caller_transaction_required", false), newCapabilityError("caller_transaction_required", false, ErrConflict)
}

func (a *App) applyScheduleReplanCapabilityTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	return a.applyScheduleReplanCapabilityWithTx(ctx, tx, invocation, resolved)
}

func (a *App) applyScheduleReplanCapabilityWithTx(ctx context.Context, callerTx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	fluctlightID, conversationID, sourceFactID := invocation.Metadata.FluctlightID, invocation.Metadata.ConversationID, invocation.SourceFactID
	rawPlan, found, err := capabilityPreparedData(invocation, "schedule_plan")
	if err != nil {
		return failedCapabilityResultDetail(invocation, "schedule_replan_arguments_invalid", false, err.Error()), err
	}
	args := mapValue(rawPlan)
	if !found || len(args) == 0 {
		return failedCapabilityResultDetail(invocation, "schedule_replan_planner_failed", true, "prepared schedule plan is required"), errors.New("schedule replan planner output is required")
	}
	if callerTx == nil {
		return failedCapabilityResult(invocation, "caller_transaction_required", false), newCapabilityError("caller_transaction_required", false, ErrConflict)
	}
	var providerArguments map[string]any
	if err := jsonUnmarshal(invocation.Arguments, &providerArguments); err != nil {
		return failedCapabilityResult(invocation, "schedule_replan_arguments_invalid", false), err
	}
	if err := validatePreparedSchedulePlan(args, providerArguments, resolved); err != nil {
		return failedCapabilityResult(invocation, "schedule_prepared_context_mismatch", false), newCapabilityError("schedule_prepared_context_mismatch", false, err)
	}
	frozenLifeContextRevision := stringValue(resolved.Life.Data["context_revision"])
	if _, err := a.requireLifeContextRevisionTx(ctx, callerTx, fluctlightID, frozenLifeContextRevision, time.Now().UTC()); err != nil {
		if errors.Is(err, ErrLifeContextStale) {
			return failedCapabilityResult(invocation, "schedule_replan_context_stale", false), newCapabilityError("schedule_replan_context_stale", false, err)
		}
		return failedCapabilityResult(invocation, "schedule_replan_schedule_read_failed", true), err
	}
	items := arrayValue(args["items"])
	if len(items) == 0 {
		return failedCapabilityResultDetail(invocation, "schedule_replan_planner_failed", true, "planner output items are required"), errors.New("schedule replan planner output is required")
	}
	if err := validateScheduleReplanItems(items); err != nil {
		return failedCapabilityResultDetail(invocation, "schedule_replan_planner_failed", true, err.Error()), err
	}
	current, err := a.currentAcceptedScheduleTx(ctx, callerTx, fluctlightID)
	if err != nil {
		code := "schedule_replan_schedule_missing"
		if !errors.Is(err, pgx.ErrNoRows) {
			code = "schedule_replan_schedule_read_failed"
		}
		return failedCapabilityResultDetail(invocation, code, errors.Is(err, pgx.ErrNoRows) == false, err.Error()), err
	}
	ownerID := stringValue(current["owner_actor_id"])
	if ownerID == "" {
		return failedCapabilityResultDetail(invocation, "schedule_replan_owner_missing", false, "schedule owner is missing"), errors.New("schedule owner is missing")
	}
	payload := map[string]any{}
	for key, value := range args {
		payload[key] = value
	}
	if strings.TrimSpace(stringValue(payload["local_date"])) == "" || stringValue(payload["local_date"]) != stringValue(current["local_date"]) {
		return failedCapabilityResultDetail(invocation, "schedule_replan_local_date_invalid", false, "local_date must match the current local schedule date"), errors.New("schedule replan local date invalid")
	}
	if timezone := canonicalTimezone(stringValue(payload["timezone"])); timezone == "" || timezone != canonicalTimezone(stringValue(current["timezone"])) {
		return failedCapabilityResultDetail(invocation, "schedule_replan_timezone_invalid", false, "timezone must match the current schedule timezone"), errors.New("schedule replan timezone invalid")
	} else {
		payload["timezone"] = timezone
	}
	if _, ok := payload["expected_revision"]; !ok || intValue(payload["expected_revision"]) <= 0 || intValue(payload["expected_revision"]) != intValue(current["revision"]) {
		return failedCapabilityResultDetail(invocation, "schedule_replan_revision_invalid", true, "expected_revision must match the current accepted schedule"), ErrConflict
	}
	if strings.TrimSpace(stringValue(payload["completed_before"])) == "" {
		return failedCapabilityResultDetail(invocation, "schedule_replan_completed_before_required", false, "completed_before is required"), errors.New("completed_before is required")
	}
	if len(arrayValue(payload["evidence_refs"])) == 0 {
		return failedCapabilityResultDetail(invocation, "schedule_replan_evidence_required", false, "evidence_refs is required"), errors.New("evidence_refs is required")
	}
	if sourceFactID != "" && !containsStringValue(arrayValue(payload["evidence_refs"]), sourceFactID) {
		payload["evidence_refs"] = append(arrayValue(payload["evidence_refs"]), sourceFactID)
	}
	if policy, ok := payload["reschedule_policy"].(map[string]any); !ok || policy == nil {
		return failedCapabilityResultDetail(invocation, "schedule_replan_planner_failed", true, "reschedule_policy is required"), errors.New("reschedule_policy is required")
	}
	payload["generated_from"] = "model_replan"
	payload["source_fact_id"] = sourceFactID
	payload["conversation_id"] = conversationID
	payload["trigger"] = "capability:schedule.replan"

	var result map[string]any
	result, err = a.replanScheduleTx(ctx, callerTx, ownerID, fluctlightID, payload)
	if err != nil {
		return failedCapabilityResultDetail(invocation, "schedule_replan_failed", errors.Is(err, ErrConflict), err.Error()), err
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
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: output, ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "schedule:" + stringValue(result["id"])}, nil
}

// validateScheduleReplanItems is intentionally stricter than the generic tool
// envelope. The provider-facing Definition documents required item fields, but
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
		if location, exists := item["location"]; exists && len([]rune(strings.TrimSpace(stringValue(location)))) > 512 {
			return fmt.Errorf("schedule replan item %d field %q is too long", index, "location")
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

type scheduleQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func (a *App) currentAcceptedSchedule(ctx context.Context, fluctlightID string) (map[string]any, error) {
	return currentAcceptedScheduleWith(ctx, a.DB.Pool(), fluctlightID)
}

func (a *App) currentAcceptedScheduleTx(ctx context.Context, tx pgx.Tx, fluctlightID string) (map[string]any, error) {
	return currentAcceptedScheduleWith(ctx, tx, fluctlightID)
}

func currentAcceptedScheduleWith(ctx context.Context, query scheduleQuerier, fluctlightID string) (map[string]any, error) {
	var ownerID, id, timezone string
	var localDate time.Time
	var revision int
	var policy []byte
	var identity []byte
	if err := query.QueryRow(ctx, `SELECT created_by_actor_id,identity FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&ownerID, &identity); err != nil {
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
	if err := query.QueryRow(ctx, `
		SELECT s.id,s.local_date,s.timezone,s.revision,s.reschedule_policy
		FROM public.life_schedules s
		WHERE s.fluctlight_id=$1 AND s.status='accepted' AND s.local_date=$2
		ORDER BY s.revision DESC
		LIMIT 1`, fluctlightID, currentDate).Scan(&id, &localDate, &timezone, &revision, &policy); err != nil {
		return nil, err
	}
	rows, err := query.Query(ctx, `SELECT id,start_at,end_at,activity,scene,location,item_type,status,priority,flexibility,interruption_cost FROM public.life_schedule_items WHERE schedule_id=$1 ORDER BY start_at`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var itemID, activity, scene, itemType, itemStatus, priority, flexibility, interruptionCost string
		var itemLocation *string
		var start, end time.Time
		if err := rows.Scan(&itemID, &start, &end, &activity, &scene, &itemLocation, &itemType, &itemStatus, &priority, &flexibility, &interruptionCost); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{
			"id": itemID, "start_at": start.Format(time.RFC3339Nano), "end_at": end.Format(time.RFC3339Nano),
			"activity": activity, "scene": scene, "location": nullablePointerValue(itemLocation), "item_type": itemType, "status": itemStatus,
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

package core

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

const scheduleInspectCapabilityName = "schedule.inspect"

type scheduleInspectService interface {
	currentAcceptedSchedule(context.Context, string) (map[string]any, error)
}

type scheduleInspectCapability struct{ service scheduleInspectService }

func scheduleInspectDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: scheduleInspectCapabilityName, Version: "v1", Type: CapabilityTypeQuery,
		Description:   "Read accepted Schedule: list 12, page via next_cursor; detail item_id before editing.",
		Surfaces:      []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceAutonomy, CapabilitySurfaceNativeCognition},
		FailurePolicy: FailurePolicyOptionalInternal,
		InputSchema: objectSchema(map[string]any{
			"operation": enumStringSchema("list", "detail"),
			"item_id":   map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
			"cursor":    map[string]any{"type": "integer", "minimum": 0, "maximum": 64},
		}, []string{"operation"}, false),
		OutputSchema: openObjectSchema(), SideEffectClass: "read_only", SuccessBoundary: "query_result_available",
		ConcurrencyClass: "parallel", SupportsRetry: true,
	}
}

func (c scheduleInspectCapability) Definition() CapabilityDefinition {
	return scheduleInspectDefinition()
}
func (c scheduleInspectCapability) RequiredContext() []ContextSlot { return nil }

func (c scheduleInspectCapability) Execute(ctx context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	if c.service == nil {
		return failedCapabilityResult(invocation, "schedule_unavailable", true), errors.New("schedule service unavailable")
	}
	args, err := capabilityExecutionArguments(invocation, scheduleInspectDefinition())
	if err != nil {
		return failedCapabilityResult(invocation, "invalid_arguments", false), err
	}
	schedule, err := c.service.currentAcceptedSchedule(ctx, invocation.Metadata.FluctlightID)
	if errors.Is(err, pgx.ErrNoRows) {
		return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed",
			Output:            map[string]any{"operation": args["operation"], "status": "schedule_pending", "items": []any{}},
			ProviderRequestID: invocation.ProviderRequestID}, nil
	}
	if err != nil {
		return failedCapabilityResult(invocation, "schedule_read_failed", true), err
	}
	output := map[string]any{
		"operation": args["operation"], "status": "accepted", "local_date": schedule["local_date"],
		"timezone": schedule["timezone"], "revision": schedule["revision"],
	}
	items := arrayValue(schedule["items"])
	if stringValue(args["operation"]) == "detail" {
		itemID := strings.TrimSpace(stringValue(args["item_id"]))
		if itemID == "" {
			return failedCapabilityResult(invocation, "schedule_item_selection_required", false), ErrInvalidArguments
		}
		for _, raw := range items {
			item := mapValue(raw)
			if stringValue(item["id"]) == itemID {
				output["item"] = scheduleInspectionItem(item)
				return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: output, ProviderRequestID: invocation.ProviderRequestID}, nil
			}
		}
		return failedCapabilityResult(invocation, "schedule_item_not_found", false), ErrNotFound
	}
	cursor := intValue(args["cursor"])
	if cursor < 0 || cursor > len(items) {
		return failedCapabilityResult(invocation, "schedule_cursor_invalid", false), ErrInvalidArguments
	}
	end := cursor + 12
	if end > len(items) {
		end = len(items)
	}
	page := make([]any, 0, end-cursor)
	for _, raw := range items[cursor:end] {
		page = append(page, scheduleInspectionItem(mapValue(raw)))
	}
	output["items"] = page
	output["has_more"] = end < len(items)
	output["next_cursor"] = end
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: output, ProviderRequestID: invocation.ProviderRequestID}, nil
}

func scheduleInspectionItem(source map[string]any) map[string]any {
	item := compactStateMap(source, []string{"id", "start_at", "end_at", "activity", "scene", "location", "item_type", "status", "priority", "flexibility", "interruption_cost", "intention_id", "action_status"})
	return item
}

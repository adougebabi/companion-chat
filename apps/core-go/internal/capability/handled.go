package capability

import (
	"context"
	"encoding/json"
	"log/slog"
)

const handledPreparationKey = "core_handled_no_effect"

func HandledCapabilityResult(inv CapabilityInvocation, code, disposition string, output map[string]any) CapabilityResult {
	payload := cloneMap(output)
	if payload == nil {
		payload = map[string]any{}
	}
	payload["handled"], payload["effect_applied"], payload["mutation_committed"] = true, false, false
	payload["warning_code"], payload["disposition"] = code, disposition
	return CapabilityResult{CallID: inv.CallID, CapabilityName: inv.CapabilityName, Status: "handled", Output: payload, ProviderRequestID: inv.ProviderRequestID, CorrelationID: inv.Metadata.CorrelationID}
}

func WithHandledPreparation(inv CapabilityInvocation, code, disposition string, output map[string]any) (CapabilityInvocation, error) {
	return WithCapabilityPreparedData(inv, handledPreparationKey, HandledCapabilityResult(inv, code, disposition, output).Output)
}

func PreparedHandledResult(inv CapabilityInvocation) (CapabilityResult, bool, error) {
	value, found, err := CapabilityPreparedData(inv, handledPreparationKey)
	if err != nil || !found {
		return CapabilityResult{}, false, err
	}
	output := mapValue(value)
	return HandledCapabilityResult(inv, stringValue(output["warning_code"]), stringValue(output["disposition"]), output), true, nil
}

// Only fresh model input is repaired. Prepared invocations retain their exact
// frozen digest; caller/server identity and malformed frozen state stay strict.
func NormalizeInvocation(ctx context.Context, inv CapabilityInvocation, definition CapabilityDefinition) (CapabilityInvocation, error) {
	if err := ctx.Err(); err != nil {
		return inv, err
	}
	if err := inv.ValidateEnvelope(definition); err != nil {
		return inv, err
	}
	if inv.Metadata.Source != "model_tool" || len(inv.PreparedPayload) != 0 {
		return inv, nil
	}
	normalized, err := NormalizeModelArguments(inv.Arguments, definition.InputSchema)
	if err != nil {
		if !json.Valid(inv.Arguments) {
			inv.Arguments = json.RawMessage(`{}`)
		}
		return WithHandledPreparation(inv, "invalid_arguments", "needs_input", map[string]any{"retry_same_arguments": false})
	}
	var original any
	_ = json.Unmarshal(inv.Arguments, &original)
	originalJSON, _ := json.Marshal(original)
	if string(normalized) != string(originalJSON) {
		slog.WarnContext(ctx, "tool.arguments.normalized", "capability", inv.CapabilityName, "call_id", inv.CallID, "original_bytes", len(inv.Arguments), "normalized_bytes", len(normalized))
	}
	inv.Arguments = normalized
	return inv, nil
}

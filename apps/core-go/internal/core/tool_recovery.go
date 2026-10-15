package core

import (
	"context"
	"encoding/json"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
	"log/slog"
)

// handledCapabilityResult reports a processed call without asserting an effect.
// Only use before mutations or after a transaction has rolled back. Real effects
// (including committed failed actions) must retain their original receipts.
func handledCapabilityResult(inv CapabilityInvocation, code, disposition string, output map[string]any) CapabilityResult {
	payload := cloneMap(output)
	if payload == nil {
		payload = map[string]any{}
	}
	payload["handled"] = true
	payload["effect_applied"] = false
	payload["mutation_committed"] = false
	payload["disposition"] = disposition
	payload["warning_code"] = code
	return CapabilityResult{CallID: inv.CallID, CapabilityName: inv.CapabilityName, Status: "handled", Output: payload, ProviderRequestID: inv.ProviderRequestID, CorrelationID: firstString(inv.Metadata.CorrelationID, "capability:"+inv.CallID)}
}

// This private key exists only inside Core's frozen preparation envelope.
// Model arguments cannot supply PreparedPayload or select this execution path.
const handledPreparationKey = "core_handled_no_effect"

func withHandledCapabilityPreparation(inv CapabilityInvocation, code, disposition string, output map[string]any) (CapabilityInvocation, error) {
	return capability.WithHandledPreparation(inv, code, disposition, output)
}

func preparedHandledCapabilityResult(inv CapabilityInvocation) (CapabilityResult, bool, error) {
	return capability.PreparedHandledResult(inv)
}

func normalizeNativeModelArguments(arguments json.RawMessage, definition CapabilityDefinition) (json.RawMessage, error) {
	normalized, err := capability.NormalizeModelArguments(arguments, definition.InputSchema)
	if err == nil && jsonString(decodeObject(arguments)) != string(normalized) {
		slog.Warn("tool.arguments.normalized", "capability", definition.Name, "original_bytes", len(arguments), "normalized_bytes", len(normalized))
	}
	return normalized, err
}

func normalizeCapabilityInvocation(ctx context.Context, inv CapabilityInvocation, definition CapabilityDefinition) (CapabilityInvocation, error) {
	return capability.NormalizeInvocation(ctx, inv, definition)
}

func withToolDefault(schema map[string]any, value any) map[string]any {
	schema["default"] = value
	return schema
}

func handledStaleReplyRecovery(result CapabilityResult) bool {
	out := mapValue(result.Output)
	return result.Status == "handled" && result.CapabilityName == conversationReplyCapabilityName && out["recovery_anchor"] == "stale_reply" && out["refresh_reply"] == true && out["effect_applied"] == false && out["mutation_committed"] == false
}

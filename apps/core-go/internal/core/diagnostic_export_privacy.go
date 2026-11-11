package core

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
)

// Export is a separate, deliberately narrow projection. Owner diagnostics may
// contain arbitrary Prompt, Tool and error text, including values in unknown
// keys, so export never recursively copies unrecognized fields.
type diagnosticExportSanitizer struct{ key [32]byte }

func newDiagnosticExportSanitizer() (*diagnosticExportSanitizer, error) {
	s := &diagnosticExportSanitizer{}
	if _, err := io.ReadFull(rand.Reader, s.key[:]); err != nil {
		return nil, err
	}
	return s, nil
}

// Equal identities remain linkable within this export, but the secret is not
// included in it and the labels cannot be used to call the product APIs.
func (s *diagnosticExportSanitizer) alias(value string) string {
	if value == "" {
		return ""
	}
	mac := hmac.New(sha256.New, s.key[:])
	_, _ = mac.Write([]byte(value))
	return "anon_" + hex.EncodeToString(mac.Sum(nil)[:16])
}

func diagnosticExportStatus(value any) string {
	status := stringValue(value)
	switch status {
	case "pending", "queued", "running", "started", "completed", "failed", "cancelled", "canceled", "timeout", "timed_out", "retry", "rejected", "accepted", "blocked", "superseded", "skipped", "unknown":
		return status
	default:
		return ""
	}
}

func diagnosticExportIdentity(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case *string:
		if v != nil {
			return *v
		}
	}
	return ""
}

func (s *diagnosticExportSanitizer) data(raw map[string]any) map[string]any {
	result := map[string]any{}
	sources := []map[string]any{raw, mapValue(raw["metrics"]), mapValue(raw["payload"])}
	for _, key := range []string{"id", "run_id", "logical_run_id", "parent_run_id", "model_call_id", "model_run_id", "provider_attempt_id", "provider_request_id", "correlation_id", "causation_id", "fluctlight_id", "conversation_id", "actor_id", "world_id", "workflow_id", "intent_id", "endpoint_id", "model_id", "agent_id", "capability_name", "call_id", "event_type"} {
		for _, source := range sources {
			if value := diagnosticExportIdentity(source[key]); value != "" {
				result[key] = s.alias(value)
				break
			}
		}
	}
	for _, source := range sources {
		if status := diagnosticExportStatus(source["status"]); status != "" {
			result["status"] = status
			break
		}
	}
	return result
}

func (s *diagnosticExportSanitizer) rows(value any) []map[string]any {
	items := arrayValue(value)
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, s.data(mapValue(item)))
	}
	return result
}

func diagnosticExportFilters(filters map[string]any) map[string]any {
	result := map[string]any{}
	for _, key := range []string{"limit", "fluctlight_id", "correlation_id", "intent_id", "workflow_id", "run_id", "surface", "status", "fluctlightId", "worldId", "intentType", "correlationId", "runId", "intentId", "workflowId", "eventType", "severity", "role", "identity", "action", "from", "to", "q", "qAny"} {
		value, found := filters[key]
		if !found {
			continue
		}
		if value == nil {
			continue
		}
		if text, ok := value.(string); ok && text == "" {
			continue
		}
		result[key] = "[REDACTED]"
	}
	return result
}

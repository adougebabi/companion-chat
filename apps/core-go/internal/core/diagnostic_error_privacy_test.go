package core

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestAgentDiagnosticProjectionPreservesLiveToolEvidence(t *testing.T) {
	trace := &ADKCapabilityTrace{}
	trace.RecordModelToolCalls("physical", 2, []schema.ToolCall{{ID: "ok"}, {ID: "failed"}})
	trace.Append(CapabilityInvocation{CallID: "ok", CapabilityName: "effect"}, CapabilityResult{CallID: "ok", CapabilityName: "effect", Status: "completed", Output: map[string]any{"value": int64(9007199254740993)}})
	trace.AppendInvocation(CapabilityInvocation{CallID: "failed", CapabilityName: "failure"})
	trace.AppendResultWithFailure(CapabilityResult{CallID: "failed", CapabilityName: "failure", Status: "failed", Output: map[string]any{"detail": "Authorization: Bearer synthetic-secret", "ordinary": "invalid argument"}}, errors.New("tool failed"))
	safe := sanitizeAgentRunResult(ADKStructuredTaskResult{Trace: trace})
	if safe.Trace == trace || mapValue(safe.Trace.Results[0].Output)["value"] != int64(9007199254740993) || mapValue(safe.Trace.Results[1].Output)["detail"] != "[REDACTED]" || mapValue(safe.Trace.Results[1].Output)["ordinary"] != "invalid argument" {
		t.Fatalf("diagnostic projection damaged evidence: %#v", safe.Trace.Results)
	}
	if mapValue(trace.Results[1].Output)["detail"] != "Authorization: Bearer synthetic-secret" || safe.Trace.ResultFailure("failed") == nil {
		t.Fatal("diagnostic projection changed live receipt or lost failure provenance")
	}
	if identity, ok := safe.Trace.ModelIdentity("failed"); !ok || identity.ProviderRequestID != "physical" || identity.ModelCallSequence != 2 {
		t.Fatalf("diagnostic projection lost physical call identity: %#v", identity)
	}
}

func TestAgentDiagnosticProjectionRedactsEncodedFailureOutput(t *testing.T) {
	const secret = "synthetic-bearer-value-74291"
	raw := json.RawMessage(`{"detail":"Authorization: Bearer ` + secret + `","count":9007199254740993}`)
	binary := []byte("Authorization: Bearer " + secret)
	trace := &ADKCapabilityTrace{}
	trace.AppendResult(CapabilityResult{CallID: "failed", Status: "failed", Output: map[string]any{"json": raw, "bytes": binary}})
	safe := sanitizeAgentRunResult(ADKStructuredTaskResult{Trace: trace})
	encoded, err := json.Marshal(safe.Trace.Results[0].Output)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), secret) || !strings.Contains(string(encoded), `"count":9007199254740993`) || !strings.Contains(string(encoded), `"bytes":"[REDACTED_BINARY]"`) {
		t.Fatalf("encoded failure output leaked or lost numeric evidence: %s", encoded)
	}
	if string(raw) != `{"detail":"Authorization: Bearer `+secret+`","count":9007199254740993}` || string(binary) != "Authorization: Bearer "+secret {
		t.Fatal("diagnostic projection changed the live encoded receipt")
	}
}

package core

import "encoding/json"

// ToolCallV1 is kept only as a compact test fixture for provider-shaped JSON.
// Production code normalizes directly into CapabilityInvocation and never
// accepts this fixture type.
type ToolCallV1 struct {
	ID                string
	Name              string
	Arguments         json.RawMessage
	SourceFactID      string
	ActionID          string
	ProviderRequestID string
	SchemaVersion     string
	Sequence          int
}

const ToolCallSchemaVersion = "fluctlight.tool-call.v1"

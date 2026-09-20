package capability

// OutputBindingV1 connects a Tool result to the Composite Action output. The
// reference is resolved by Core after the output resource (message or Moment)
// receives its durable ID.
type OutputBindingV1 struct {
	ToolCallID string `json:"tool_call_id"`
	TargetKind string `json:"target_kind"`
	TargetRef  string `json:"target_ref"`
}

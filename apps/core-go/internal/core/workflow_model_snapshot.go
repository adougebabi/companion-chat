package core

import "encoding/json"

const workflowModelSnapshotVersion = "v1"

// Credentials stay in the server secret store; these snapshots contain only
// the reference and immutable request configuration needed for recovery.
type workflowModelSnapshot struct {
	Version       string             `json:"version"`
	Agent         FormalAgentID      `json:"agent"`
	Variant       string             `json:"variant"`
	Role          string             `json:"role"`
	Assignment    providerAssignment `json:"assignment"`
	ConfigVersion int64              `json:"config_version,omitempty"`
	Concurrency   int                `json:"concurrency,omitempty"`
}

func newWorkflowModelSnapshot(agent FormalAgentID, variant, role string, assignment providerAssignment, version int64, concurrency int) workflowModelSnapshot {
	assignment.Secret = ""
	return workflowModelSnapshot{Version: workflowModelSnapshotVersion, Agent: agent, Variant: variant, Role: role, Assignment: assignment, ConfigVersion: version, Concurrency: concurrency}
}

func readWorkflowModelSnapshot(provenance map[string]any, key, stage string, agent FormalAgentID, variant, role string, invalid error) (workflowModelSnapshot, bool, error) {
	var snapshot workflowModelSnapshot
	container, exists := provenance[key]
	if !exists {
		return snapshot, false, nil
	}
	snapshots, ok := container.(map[string]any)
	if !ok || snapshots == nil {
		return snapshot, true, invalid
	}
	raw, found := snapshots[stage]
	if !found {
		return snapshot, false, nil
	}
	if err := json.Unmarshal(jsonBytes(raw), &snapshot); err != nil || snapshot.Version != workflowModelSnapshotVersion || snapshot.Agent != agent || snapshot.Variant != variant || snapshot.Role != role || snapshot.Assignment.Role != providerBindingRole(role) || snapshot.Assignment.EndpointID == "" || snapshot.Assignment.ModelID == "" || snapshot.Assignment.BaseURL == "" || snapshot.Assignment.Timeout <= 0 || snapshot.Assignment.Secret != "" || (role == conversationReplyModelRole && (snapshot.Concurrency < 1 || snapshot.Concurrency > 8)) {
		return workflowModelSnapshot{}, true, invalid
	}
	return snapshot, true, nil
}

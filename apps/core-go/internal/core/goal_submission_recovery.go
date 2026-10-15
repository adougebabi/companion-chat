package core

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/cloudwego/eino/schema"
)

const goalEvaluationNoProgressLimit = 3

// This budget belongs exclusively to Goal Evaluation. Only durable accepted
// targets reset it; replayed receipts and assistant prose cannot show progress.
type goalEvaluationProgress struct {
	mu       sync.Mutex
	accepted int
	attempts int
	stalled  int
	stopped  bool
}

func (p *goalEvaluationProgress) decide(accepted, attempts int, missing bool) physicalModelRequestDecision {
	p.mu.Lock()
	defer p.mu.Unlock()
	if accepted > p.accepted {
		p.stalled = 0
	} else if attempts > p.attempts {
		p.stalled += attempts - p.attempts
	}
	p.accepted, p.attempts = accepted, attempts
	if p.stalled >= goalEvaluationNoProgressLimit {
		p.stopped = true
	}
	if p.stopped {
		return physicalModelRequestDecision{ToolChoice: schema.ToolChoiceForbidden, AllowedToolNames: []string{}, ToolCallFallback: schema.AssistantMessage(`{"summary":"本轮已停止重复提交；保留实际已接受结果，未解决评估等待可用证据。"}`, nil)}
	}
	if missing {
		return physicalModelRequestDecision{ToolChoice: schema.ToolChoiceForced, OmitResponseFormat: true}
	}
	return physicalModelRequestDecision{ToolChoice: schema.ToolChoiceAllowed}
}
func (p *goalEvaluationProgress) awaitingEvidence() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stopped
}

func goalEvaluationNativeAttempts(ctx context.Context) int {
	bridge, ok := adkCapabilityContext(ctx)
	if !ok || bridge.Trace == nil {
		return 0
	}
	_, results := bridge.Trace.Snapshot()
	count := 0
	for _, result := range results {
		if isGoalEvaluationTool(result.CapabilityName) {
			count++
		}
	}
	return count
}

func goalSubmissionReplay(prior goalSubmissionRecord, digest, name string) map[string]any {
	output := cloneMap(prior.Output)
	output["replayed"], output["ignored_payload_change"] = true, prior.Digest != digest
	output["capability_name"], output["submission_kind"] = name, goalSubmissionKind(name)
	output["new_mutation_committed"] = false
	return output
}
func goalSubmissionKind(name string) string {
	switch name {
	case goalPlanSubmit:
		return "plan"
	case goalObjectSubmit:
		return "object"
	case goalEvaluationSubmit:
		return "root"
	}
	return ""
}

func goalEvaluationUnresolvedWireRefs(binding *goalEvaluationWireBinding, records map[string]goalSubmissionRecord) []string {
	_, missing := goalEvaluationSubmissionCoverage(binding.snapshot, records)
	refs := make([]string, 0, len(missing))
	for _, id := range missing {
		refs = append(refs, binding.goalRefsByID[id])
	}
	return refs
}

// Only representation unambiguous in this frozen target is adapted. Unbound
// refs and contradictory duplicates cannot replace existing real judgments.
func normalizeGoalSubmission(binding *goalEvaluationWireBinding, entry goalEvaluationGoal, name string, args map[string]any) (map[string]any, error) {
	out := cloneMap(args)
	if name == goalPlanSubmit {
		return out, nil
	}
	kind, objectID := "goal", entry.GoalID
	if name == goalObjectSubmit {
		kind = stringValue(out["object_kind"])
		ref := stringValue(out["object_ref"])
		objects := binding.stagesByRef
		if kind == "commitment" {
			objects = binding.commitmentsByRef
		}
		object, ok := objects[ref]
		if !ok || object.GoalID != entry.GoalID || (kind != "stage" && kind != "commitment") {
			return nil, errors.New("goal_evaluation_wire_object_ref_invalid")
		}
		objectID = object.ID
	}
	values := []any{}
	seen := map[string]map[string]any{}
	known := false
	for _, raw := range arrayValue(out["judgments"]) {
		value := cloneMap(mapValue(raw))
		ref := strings.TrimSpace(stringValue(value["criterion_ref"]))
		criterion, ok := binding.criteriaByRef[ref]
		if !ok || criterion.GoalID != entry.GoalID || criterion.ObjectKind != kind || criterion.ObjectID != objectID {
			return nil, errors.New("goal_evaluation_wire_criterion_ref_invalid")
		}
		value["criterion_ref"] = ref
		if prior, exists := seen[ref]; exists {
			a, b := cloneMap(prior), cloneMap(value)
			delete(a, "evidence_refs")
			delete(b, "evidence_refs")
			if jsonString(a) != jsonString(b) {
				return nil, errors.New("goal_evaluation_wire_criterion_duplicate_conflict")
			}
			prior["evidence_refs"] = mergeStableRefs(decisionServiceRefValues(prior["evidence_refs"]), decisionServiceRefValues(value["evidence_refs"]))
			continue
		}
		seen[ref] = value
		values = append(values, value)
		known = known || stringValue(value["verdict"]) != "unknown"
	}
	out["judgments"] = values
	if name == goalEvaluationSubmit && !known && stringValue(out["impact"]) == "no_change" {
		out["impact"] = "needs_evidence"
		if stringValue(out["wait_condition"]) == "" && stringValue(out["blocker"]) == "" && stringValue(out["next_step"]) == "" {
			out["wait_condition"] = "等待可用证据，按下一评估周期复核"
		}
	}
	return out, nil
}

func goalSubmissionAwaitingEvidence(binding *goalEvaluationWireBinding, entry goalEvaluationGoal, name string, args map[string]any, reason string) map[string]any {
	refs := []string{}
	kind, objectID := "goal", entry.GoalID
	if name == goalObjectSubmit {
		kind = stringValue(args["object_kind"])
		objects := binding.stagesByRef
		if kind == "commitment" {
			objects = binding.commitmentsByRef
		}
		objectID = objects[stringValue(args["object_ref"])].ID
	}
	for _, ref := range sortedGoalBindingRefs(binding.criteriaByRef) {
		c := binding.criteriaByRef[ref]
		if c.GoalID == entry.GoalID && c.ObjectKind == kind && c.ObjectID == objectID {
			refs = append(refs, ref)
		}
	}
	output := map[string]any{"goal_ref": binding.goalRefsByID[entry.GoalID], "capability_name": name, "submission_kind": goalSubmissionKind(name), "handled": true, "disposition": "awaiting_evidence", "assessment_accepted": false, "mutation_committed": false, "reason": reason, "allowed_criterion_refs": refs}
	if name == goalObjectSubmit && len(refs) > 0 {
		output["object_ref"], output["object_kind"] = args["object_ref"], kind
	}
	return output
}

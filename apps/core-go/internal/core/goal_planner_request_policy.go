package core

import (
	"context"
	"errors"

	"github.com/cloudwego/eino/schema"
)

// This policy guards the Planner's evidence protocol, not its semantic decision.
// Once actual queries are consumed, no_change and awaiting_facts remain legal.
type goalPlannerPhysicalRequestPolicy struct{}

func (goalPlannerPhysicalRequestPolicy) DecidePhysicalModelRequest(ctx context.Context) (physicalModelRequestDecision, error) {
	adk, ok := adkCapabilityContext(ctx)
	if !ok || adk.Trace == nil {
		return physicalModelRequestDecision{}, errors.New("goal_planner_trace_missing")
	}
	return physicalModelRequestDecision{ToolChoice: schema.ToolChoiceAllowed}, nil
}

// Missing technical metadata is bound to the actual snapshot already consumed
// by this run. Never replace stale model intent with the latest database fence.
func bindGoalPlannerMetadata(ctx context.Context, inv CapabilityInvocation, args map[string]any) map[string]any {
	out := cloneMap(args)
	if stringValue(out["idempotency_key"]) == "" {
		out["idempotency_key"] = "planner:" + capabilityOperationID(inv)
	}
	adk, ok := adkCapabilityContext(ctx)
	if !ok || adk.Trace == nil {
		return out
	}
	calls, results := adk.Trace.Snapshot()
	current, currentKnown := adk.Trace.ModelIdentity(inv.CallID)
	if !currentKnown {
		return out
	}
	queries := map[string]bool{}
	for _, call := range calls {
		if call.CapabilityName == goalPlannerQuery && stringValue(decodeObject(call.Arguments)["section"]) == "snapshot" {
			queries[call.CallID] = true
		}
	}
	for i := len(results) - 1; i >= 0; i-- {
		result := results[i]
		if result.CapabilityName != goalPlannerQuery || result.Status != "completed" || !queries[result.CallID] {
			continue
		}
		identity, known := adk.Trace.ModelIdentity(result.CallID)
		if !known || identity.ModelCallSequence >= current.ModelCallSequence {
			continue
		}
		snapshot := mapValue(result.Output)
		if out["expected_version"] == nil {
			out["expected_version"] = snapshot["revision"]
		}
		if stringValue(out["expected_facts_revision"]) == "" {
			out["expected_facts_revision"] = snapshot["facts_revision"]
		}
		break
	}
	return out
}

// A read emitted beside a commit is not a post-commit snapshot, even if its
// callback happens to finish later. Only a later physical decision can reread.
func goalPlannerQueryCoverage(trace *ADKCapabilityTrace) (queries, commits int, reread bool) {
	calls, results := trace.Snapshot()
	invocations := map[string]CapabilityInvocation{}
	for _, call := range calls {
		invocations[call.CallID] = call
	}
	seen := map[string]bool{}
	commitSequence := uint64(0)
	for _, result := range results {
		if result.Status != "completed" || result.CapabilityName != goalPlannerCommit || seen[result.CallID] {
			continue
		}
		seen[result.CallID] = true
		commits++
		if identity, ok := trace.ModelIdentity(result.CallID); ok && identity.ModelCallSequence > commitSequence {
			commitSequence = identity.ModelCallSequence
		}
	}
	seen = map[string]bool{}
	for _, result := range results {
		if result.Status != "completed" || result.CapabilityName != goalPlannerQuery || seen[result.CallID] {
			continue
		}
		seen[result.CallID] = true
		queries++
		call, ok := invocations[result.CallID]
		if !ok || stringValue(decodeObject(call.Arguments)["section"]) != "snapshot" || commitSequence == 0 {
			continue
		}
		if identity, ok := trace.ModelIdentity(result.CallID); ok && identity.ModelCallSequence > commitSequence {
			reread = true
		}
	}
	return
}

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
	queries, commits, reread := goalPlannerQueryCoverage(adk.Trace)
	if queries < 2 || (commits > 0 && !reread) {
		return physicalModelRequestDecision{ToolChoice: schema.ToolChoiceForced, OmitResponseFormat: true, AllowedToolNames: []string{goalPlannerQuery}}, nil
	}
	return physicalModelRequestDecision{ToolChoice: schema.ToolChoiceAllowed}, nil
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

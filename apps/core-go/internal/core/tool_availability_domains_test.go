package core

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/jackc/pgx/v5/pgconn"
)

func requireHandledToolNoEffect(t *testing.T, receipt ToolExecutionReceipt, err error, code string) {
	t.Helper()
	out := mapValue(receipt.Result.Output)
	if err != nil || receipt.Result.Status != "handled" || receipt.Result.ErrorCode != "" || goalSubmissionReceiptCode(receipt.Result) != code || out["effect_applied"] != false || out["mutation_committed"] != false {
		t.Fatalf("expected handled no effect (%s): %#v %v", code, receipt, err)
	}
}

func TestGoalSubmissionThinRecoverySchema(t *testing.T) {
	root, _, plan := goalEvaluationToolSchemas()
	for _, tc := range []struct {
		name   string
		schema map[string]any
		args   map[string]any
	}{
		{"unknown root", root, map[string]any{"goal_ref": "goal:1", "impact": "needs_evidence", "judgments": []any{map[string]any{"criterion_ref": "criterion:1.1", "verdict": "unknown", "kind": "semantic", "subject": "actor_self", "discourse": "uncertain", "evidence_refs": []any{}, "reason": "No new evidence"}}}},
		{"stage skip", plan, map[string]any{"goal_ref": "goal:1", "reason": "No longer applicable", "stage": map[string]any{"operation": "skip", "object_ref": "stage:1.1", "reason": "No longer applicable"}}},
		{"commitment abandon", plan, map[string]any{"goal_ref": "goal:1", "reason": "No longer applicable", "commitment": map[string]any{"operation": "abandon", "object_ref": "commitment:1.1", "reason": "No longer applicable"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateNativeArguments(json.RawMessage(jsonBytes(tc.args)), CapabilityDefinition{InputSchema: tc.schema}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestToolAvailabilityAll49Names(t *testing.T) {
	r := mustCapabilityRegistry(builtinCapabilities(&App{})...)
	names := capabilityDefinitionNames(r.Definitions())
	names = append(names, (&replyImageRequestTool{}).Definition().Name)
	sort.Strings(names)
	want := append(append([]string{}, independentToolProductInventory...), replyImageRequestName)
	sort.Strings(want)
	if len(names) != 49 || !phase8EqualStrings(names, want) {
		t.Fatalf("tool availability inventory drift: %v", names)
	}
	if len(r.Catalog(CapabilitySurfaceConversation)) != 38 {
		t.Fatal("conversation inventory drift")
	}
}

func TestToolAvailabilityOutcomeCapacityIsAdvisory(t *testing.T) {
	refs := []string{}
	for n := 0; n < 80; n++ {
		refs = append(refs, "goal:ctx_"+strings.Repeat("a", 31)+string(rune('a'+n%26)))
	}
	r := CapabilityResult{CallID: "large", CapabilityName: wardrobeWearCapabilityName, Status: "completed", Output: map[string]any{"context": strings.Repeat("x", 70<<10)}}
	outcomes, err := buildActionOutcomes("action", "owner", "source", "capability", []CapabilityResult{r}, map[string]any{"goal_refs": refs}, mustCapabilityRegistry(wardrobeWearCapability{}))
	if err != nil || len(outcomes) != 2 || len(jsonBytes(outcomes[1].Observed)) < 70<<10 || len(outcomes[0].GoalRefs) != len(refs) {
		t.Fatal("outcome data rejected or truncated", err)
	}
}

func TestReplyOptionalEvidenceReconciliationKeepsExactScope(t *testing.T) {
	ref := "memory:ctx_valid"
	entry := ContextReference{Kind: ContextReferenceMemory, EntityID: "m1", Revision: 1, Snapshot: json.RawMessage(`{"value":"confirmed"}`)}
	live := ContextProjection{ReferenceIndex: ContextReferenceIndex{FluctlightID: "self", ConversationID: "conversation", ByRef: map[string]ContextReference{ref: entry}}}
	inv := CapabilityInvocation{Metadata: InvocationMetadata{Source: "model_tool", FluctlightID: "self", ConversationID: "conversation"}, ContextSnapshot: map[string]any{"context_reference_index": decodeObject(jsonBytes(live.ReferenceIndex))}}
	before := jsonString(inv.ContextSnapshot)
	got, report := reconcileReplyEvidence(inv, live, []string{ref, "message:old", ref, "cr_000000000000"})
	if len(got) != 1 || intValue(report["excluded_count"]) != 3 {
		t.Fatal(got, report)
	}
	if strings.Contains(jsonString(report), ref) || strings.Contains(jsonString(report), "message:old") {
		t.Fatal("raw hint leaked into diagnostics")
	}
	live.ReferenceIndex.ConversationID = "other"
	// The invocation/offered identity remains authoritative; a same-token live
	// entry with a changed snapshot must not become an authorized hint.
	live.ReferenceIndex.ByRef[ref] = ContextReference{Kind: entry.Kind, EntityID: "m2", Revision: 2, Snapshot: entry.Snapshot}
	got, _ = reconcileReplyEvidence(inv, live, []string{ref})
	if len(got) != 0 || jsonString(inv.ContextSnapshot) != before {
		t.Fatal("scope matcher or frozen context changed")
	}
}

func TestReplyOptionalAliasFailureCannotBlockOtherArguments(t *testing.T) {
	codec, _ := newProviderContextRefCodec(ContextReferenceIndex{ByRef: map[string]ContextReference{}})
	raw := json.RawMessage(`{"evidence_refs":["cr_000000000000"],"topic_key":"topic"}`)
	got, err := codec.decodeReplyArguments(raw)
	if err != nil || stringValue(decodeObject(got)["topic_key"]) != "topic" {
		t.Fatal(string(got), err)
	}
	if _, err = codec.decodeReplyArguments(json.RawMessage(`{"draft_ref":"cr_000000000000"}`)); err == nil {
		t.Fatal("unknown required draft alias was accepted")
	}
}

func TestToolBusinessRecoveryDoesNotHideDependencyOrAuthority(t *testing.T) {
	inv := CapabilityInvocation{CallID: "call", CapabilityName: "wardrobe.wear"}
	for _, err := range []error{context.Canceled, context.DeadlineExceeded, ErrUnauthorized, &pgconn.PgError{Code: "42P01", Message: "missing test table"}, errors.Join(ErrConflict, &pgconn.PgError{Code: "08006", Message: "connection failed during rollback"}), errors.New("actual provider failed")} {
		if _, ok := recoverToolBusinessResult(inv, failedCapabilityResult(inv, "life_state_sleeping", false), err); ok {
			t.Fatalf("real failure recovered as no effect: %v", err)
		}
	}
	for _, err := range []error{ErrInvalidArguments, ErrNotFound, ErrConflict, ErrCurrentFactsStale} {
		got, ok := recoverToolBusinessResult(inv, failedCapabilityResult(inv, "domain_precondition", false), err)
		if !ok || got.Status != "handled" || got.ErrorCode != "" || mapValue(got.Output)["effect_applied"] != false {
			t.Fatal(got, ok)
		}
	}
	for code, disposition := range toolSemanticRecovery {
		err := errors.Join(ErrCapabilityExecution, errors.New(code))
		got, ok := recoverToolBusinessResult(inv, failedCapabilityResult(inv, "domain_apply_failed", false), err)
		if !ok || goalSubmissionReceiptCode(got) != code || mapValue(got.Output)["disposition"] != disposition {
			t.Fatal(code, got, ok)
		}
	}
	got, ok := recoverToolBusinessResult(inv, failedCapabilityResult(inv, "context_stale", false), ErrLifeContextStale)
	if !ok || mapValue(got.Output)["refresh_context"] != true {
		t.Fatal("stale refresh signal lost", got)
	}
}

func TestPlannerMetadataUsesConsumedSnapshotOnly(t *testing.T) {
	trace := &ADKCapabilityTrace{}
	trace.RecordModelToolCalls("query-request", 1, []schema.ToolCall{{ID: "snapshot"}})
	trace.RecordModelToolCalls("commit-request", 2, []schema.ToolCall{{ID: "commit"}, {ID: "same-batch"}})
	for _, id := range []string{"snapshot", "same-batch"} {
		trace.AppendInvocation(CapabilityInvocation{CallID: id, CapabilityName: goalPlannerQuery, Arguments: json.RawMessage(`{"section":"snapshot"}`)})
		version := 1
		if id == "same-batch" {
			version = 2
		}
		trace.AppendResult(CapabilityResult{CallID: id, CapabilityName: goalPlannerQuery, Status: "completed", Output: map[string]any{"revision": version, "facts_revision": "facts-consumed"}})
	}
	ctx := WithADKCapabilityInvoker(context.Background(), &goalRequestPolicyInvoker{}, trace)
	inv := CapabilityInvocation{CallID: "commit", Metadata: InvocationMetadata{OperationID: "trusted-operation"}}
	got := bindGoalPlannerMetadata(ctx, inv, map[string]any{"reason": "plan"})
	if intValue(got["expected_version"]) != 1 || got["idempotency_key"] != "planner:trusted-operation" {
		t.Fatal(got)
	}
	got = bindGoalPlannerMetadata(ctx, inv, map[string]any{"expected_version": 0, "expected_facts_revision": "original-stale"})
	if intValue(got["expected_version"]) != 0 || got["expected_facts_revision"] != "original-stale" {
		t.Fatal("CAS was rebound", got)
	}
}

func TestCapabilityRequestKeepsCodeAndOmitsOnlySecretFields(t *testing.T) {
	args := map[string]any{"code": "print('example')", "nested": map[string]any{"script": "example", "api_key": "private-value"}}
	before := jsonString(args)
	got, count := sanitizeCapabilityRequestContract(args)
	if count != 1 || stringValue(got["code"]) == "" || stringValue(mapValue(got["nested"])["script"]) == "" || containsSensitiveKey(got) || jsonString(args) != before {
		t.Fatal(got, count)
	}
}

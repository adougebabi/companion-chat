package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

var errDirectToolDependency = errors.New("direct tool dependency unavailable")

type failingDirectQueryCapability struct{}

func (failingDirectQueryCapability) Definition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: "test.direct.dependency", Version: "v1", Type: CapabilityTypeQuery,
		Description: "Exercise direct dependency error identity.",
		InputSchema: objectSchema(map[string]any{}, nil, false), OutputSchema: openObjectSchema(),
		SideEffectClass: "read_only", FailurePolicy: FailurePolicyOptionalInternal,
	}
}

func (failingDirectQueryCapability) RequiredContext() []ContextSlot { return nil }

func (failingDirectQueryCapability) Execute(context.Context, CapabilityInvocation, CapabilityContext) (CapabilityResult, error) {
	return CapabilityResult{}, errDirectToolDependency
}

func TestDirectToolExecutionMemoryEventAndRecallOwnsCommitAndOperationReplay(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	ownerID := "actor_direct_tool_" + suffix
	fluctlightID := "fluctlight_direct_tool_" + suffix
	secret := "direct-tool-secret-" + suffix
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active',$3,'{}','{}','{}','{}','{}')`, fluctlightID, ownerID, json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}

	app := &App{DB: repository}
	app.ContextResolver = NewAppContextResolver(app)
	app.Capabilities = mustCapabilityRegistry(builtinCapabilities(app)...)
	runtime, err := NewCapabilityRuntime(app.Capabilities, app.ContextResolver)
	if err != nil {
		t.Fatal(err)
	}
	app.Runtime = runtime

	write := ToolExecutionRequest{
		CapabilityName: "memory_event", OperationID: "remember-secret-" + suffix,
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID,
		EvidenceID: "owner-command-" + suffix, Surface: CapabilitySurfaceNativeCognition,
		Arguments: jsonBytes(map[string]any{"content": secret, "type": "semantic", "confidence": 1.0, "importance": 1.0}),
	}
	first, err := app.ExecuteTool(ctx, write)
	if err != nil || first.Result.Status != "completed" {
		t.Fatalf("first memory write receipt=%#v err=%v", first, err)
	}
	memoryID := stringValue(mapValue(first.Result.Output)["memory_id"])
	if memoryID == "" || first.NativeToolCallID != "" || !strings.HasPrefix(first.ExecutionCallID, "direct_call_") {
		t.Fatalf("direct execution identity/result invalid: %#v", first)
	}
	var storedContent, storedSourceFact string
	if err := repository.Pool().QueryRow(ctx, `SELECT content,evidence_refs->>0 FROM public.memories WHERE id=$1`, memoryID).Scan(&storedContent, &storedSourceFact); err != nil {
		t.Fatal(err)
	}
	if storedContent != secret || storedSourceFact != write.EvidenceID {
		t.Fatalf("stored memory content=%q source=%q", storedContent, storedSourceFact)
	}
	var cognitionRows int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.cognition_inbox WHERE id=$1`, write.EvidenceID).Scan(&cognitionRows); err != nil {
		t.Fatal(err)
	}
	if cognitionRows != 0 {
		t.Fatalf("direct Tool fabricated %d cognition rows", cognitionRows)
	}

	retryThroughAgentAdapter := write
	retryThroughAgentAdapter.NativeToolCallID = "native-tool-retry-" + suffix
	retryThroughAgentAdapter.ProviderRequestID = "provider-retry-" + suffix
	replayed, err := app.ExecuteTool(ctx, retryThroughAgentAdapter)
	if err != nil || replayed.Result.Status != "completed" || replayed.NativeToolCallID != retryThroughAgentAdapter.NativeToolCallID || replayed.ExecutionCallID == first.ExecutionCallID || stringValue(mapValue(replayed.Result.Output)["memory_id"]) != memoryID || !boolValueForTest(mapValue(replayed.Result.Output)["replayed"]) {
		t.Fatalf("memory operation replay receipt=%#v err=%v", replayed, err)
	}
	conflict := write
	conflict.Arguments = jsonBytes(map[string]any{"content": secret + "-different", "type": "semantic", "confidence": 1.0, "importance": 1.0})
	if receipt, conflictErr := app.ExecuteTool(ctx, conflict); conflictErr == nil || receipt.Result.Status != "failed" {
		t.Fatalf("same operation with different payload must conflict: receipt=%#v err=%v", receipt, conflictErr)
	}

	recall, err := app.ExecuteTool(ctx, ToolExecutionRequest{
		CapabilityName: "memory.recall", OperationID: "recall-secret-" + suffix,
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID,
		Arguments: jsonBytes(map[string]any{"intent": secret}), Surface: CapabilitySurfaceConversation,
	})
	if err != nil || recall.Result.Status != "completed" {
		t.Fatalf("memory recall receipt=%#v err=%v", recall, err)
	}
	items := arrayValue(mapValue(recall.Result.Output)["items"])
	found := false
	for _, raw := range items {
		if strings.Contains(stringValue(mapValue(raw)["content"]), secret) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("recall did not observe committed secret: %#v", recall.Result.Output)
	}
}

func TestToolExecutionRequestRejectsMissingBusinessIdentity(t *testing.T) {
	for name, request := range map[string]ToolExecutionRequest{
		"operation":     {CapabilityName: "memory_event", AuthorizationActorID: "owner", FluctlightID: "fl", Arguments: json.RawMessage(`{}`)},
		"authorization": {CapabilityName: "memory_event", OperationID: "op", FluctlightID: "fl", Arguments: json.RawMessage(`{}`)},
		"resource":      {CapabilityName: "memory_event", OperationID: "op", AuthorizationActorID: "owner", Arguments: json.RawMessage(`{}`)},
	} {
		t.Run(name, func(t *testing.T) {
			if err := request.validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestCapabilityRuntimePreservesDirectDependencyCause(t *testing.T) {
	registry := mustCapabilityRegistry(failingDirectQueryCapability{})
	runtime, err := NewCapabilityRuntime(registry, NewStaticContextResolver(nil))
	if err != nil {
		t.Fatal(err)
	}
	invocation := CapabilityInvocation{
		CallID: "direct-call", CapabilityName: "test.direct.dependency", Arguments: json.RawMessage(`{}`),
		SourceFactID: "owner-command", Sequence: 0,
		Metadata: InvocationMetadata{Source: "direct", OperationID: "stable-operation", FluctlightID: "fl"},
	}
	result, err := runtime.Execute(context.Background(), invocation)
	if !errors.Is(err, errDirectToolDependency) || result.Status != "failed" {
		t.Fatalf("dependency cause/result lost: result=%#v err=%v", result, err)
	}
}

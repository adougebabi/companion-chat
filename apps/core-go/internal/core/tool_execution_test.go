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
	var targetRef string
	for _, raw := range items {
		if strings.Contains(stringValue(mapValue(raw)["content"]), secret) {
			found = true
			targetRef = stringValue(mapValue(raw)["ref"])
			break
		}
	}
	if !found {
		t.Fatalf("recall did not observe committed secret: %#v", recall.Result.Output)
	}
	corrected := "corrected-direct-tool-secret-" + suffix
	correction, err := app.ExecuteTool(ctx, ToolExecutionRequest{
		CapabilityName: "memory_event", OperationID: "correct-secret-" + suffix,
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID,
		EvidenceID: "owner-correction-" + suffix, Surface: CapabilitySurfaceNativeCognition,
		Arguments: jsonBytes(map[string]any{"operation": "revise", "target_ref": targetRef,
			"content": corrected, "type": "semantic", "confidence": 1.0, "importance": 1.0}),
	})
	if err != nil || correction.Result.Status != "completed" || intValue(mapValue(correction.Result.Output)["revision"]) != 1 {
		t.Fatalf("direct correction receipt=%#v err=%v", correction, err)
	}
	var correctedContent string
	if err := repository.Pool().QueryRow(ctx, `SELECT content FROM public.memories WHERE id=$1`, memoryID).Scan(&correctedContent); err != nil || correctedContent != corrected {
		t.Fatalf("corrected Memory was not durable: content=%q err=%v", correctedContent, err)
	}
	oldRecall, err := app.ExecuteTool(ctx, ToolExecutionRequest{
		CapabilityName: "memory.recall", OperationID: "recall-old-after-correction-" + suffix,
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID,
		Arguments: jsonBytes(map[string]any{"intent": secret}), Surface: CapabilitySurfaceConversation,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range arrayValue(mapValue(oldRecall.Result.Output)["items"]) {
		if stringValue(mapValue(raw)["content"]) == secret {
			t.Fatalf("old corrected Memory revived in recall: %#v", oldRecall.Result.Output)
		}
	}
	newRecall, err := app.ExecuteTool(ctx, ToolExecutionRequest{
		CapabilityName: "memory.recall", OperationID: "recall-new-after-correction-" + suffix,
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID,
		Arguments: jsonBytes(map[string]any{"intent": corrected}), Surface: CapabilitySurfaceConversation,
	})
	if err != nil || !strings.Contains(jsonString(mapValue(newRecall.Result.Output)["items"]), corrected) {
		t.Fatalf("corrected Memory unavailable: %#v err=%v", newRecall.Result.Output, err)
	}
}

func TestNativeMemoryCorrectionUsesFrozenPromptReference(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, factID := "frozen-ref-owner", "frozen-ref-fluctlight", "frozen-ref-correction-fact"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{}','{}','{}','{}','{}','{}')`, fluctlightID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status) VALUES($1,$2,1,'conversation.turn','{"text":"更正：猫叫奶酪"}',$1,$1,$1,now(),'processed')`, factID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	app := newTestApp(t, repository, nil)
	created, err := app.ExecuteTool(ctx, ToolExecutionRequest{
		CapabilityName: "memory_event", OperationID: "frozen-ref-seed", AuthorizationActorID: ownerID, FluctlightID: fluctlightID,
		EvidenceID: "owner-confirmed-cat", Surface: CapabilitySurfaceNativeCognition,
		Arguments: jsonBytes(map[string]any{"content": "用户的猫叫布丁", "type": "semantic", "confidence": 1.0, "importance": 0.6}),
	})
	if err != nil || created.Result.Status != "completed" {
		t.Fatalf("seed Memory: receipt=%#v err=%v", created, err)
	}
	memoryID := stringValue(mapValue(created.Result.Output)["memory_id"])
	index := ContextReferenceIndex{SchemaVersion: contextReferenceIndexVersion, FluctlightID: fluctlightID, OwnerActorID: ownerID, SpeakerActorID: ownerID, ByRef: map[string]ContextReference{}}
	visible := map[string]any{"id": memoryID, "revision": 0, "content": "用户的猫叫布丁"}
	ref, err := addReferenceToRow(&index, ContextReferenceMemory, visible, memoryID, 0)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := map[string]any{
		"identity":                map[string]any{"fluctlight_id": fluctlightID, "source_fact_id": factID},
		"core_persona":            map[string]any{"identity": map[string]any{"name": "摇光"}},
		"memory_scope":            map[string]any{"owner_actor_id": ownerID, "viewer_actor_ids": []any{ownerID}, "active_profile_id": "default"},
		"context_reference_index": index,
	}
	request := ToolExecutionRequest{
		CapabilityName: "memory_event", OperationID: "frozen-ref-correct", NativeToolCallID: "provider-call-1", ProviderRequestID: "provider-request-1",
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID, EvidenceID: factID, Surface: CapabilitySurfaceConversation,
		FrozenContextSnapshot: snapshot,
		Arguments:             jsonBytes(map[string]any{"operation": "revise", "target_ref": ref, "content": "用户的猫叫奶酪", "type": "semantic", "confidence": 1.0, "importance": 0.6}),
	}
	wrongScope := request
	wrongScope.FrozenContextSnapshot = cloneMap(snapshot)
	wrongScope.FrozenContextSnapshot["identity"] = map[string]any{"fluctlight_id": "foreign", "source_fact_id": factID}
	if _, err := app.ExecuteTool(ctx, wrongScope); !errors.Is(err, ErrInvalidArguments) {
		t.Fatalf("foreign frozen Tool snapshot accepted: %v", err)
	}
	corrected, err := app.ExecuteTool(ctx, request)
	if err != nil || corrected.Result.Status != "completed" || intValue(mapValue(corrected.Result.Output)["revision"]) != 1 {
		t.Fatalf("frozen prompt ref correction receipt=%#v err=%v", corrected, err)
	}
	var content, provenance, evidenceRef string
	if err := repository.Pool().QueryRow(ctx, `SELECT content,provenance_status,evidence_refs->>0 FROM public.memories WHERE id=$1`, memoryID).Scan(&content, &provenance, &evidenceRef); err != nil || content != "用户的猫叫奶酪" || provenance != "verified" || evidenceRef != factID {
		t.Fatalf("frozen ref correction not durable and sourced: content=%q provenance=%q evidence=%q err=%v", content, provenance, evidenceRef, err)
	}
	if _, err := repository.Pool().Exec(ctx, `DELETE FROM public.cognition_inbox WHERE id=$1`, factID); err != nil {
		t.Fatal(err)
	}
	plan, err := buildMemoryQueryPlan(MemoryForConversation, []string{ownerID}, MemoryConversationGlobalOnly, "", nil, "default", []MemoryQueryCue{{Kind: "test", Text: "奶酪"}}, 6, 2400)
	if err != nil {
		t.Fatal(err)
	}
	result, err := app.retrieveMemoryWithPlan(ctx, ownerID, fluctlightID, plan)
	if err != nil || len(result.Items) != 0 {
		t.Fatalf("deleted correction source still supports current Memory: %#v err=%v", result.Items, err)
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

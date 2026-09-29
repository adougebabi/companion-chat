package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
)

var errDirectToolDependency = errors.New("direct tool dependency unavailable")

func TestModelFacingStateReceiptsOmitFactsAlreadyInCurrentContext(t *testing.T) {
	reply := modelFacingToolResult(ToolExecutionReceipt{Result: CapabilityResult{Status: "completed", Output: map[string]any{
		"text": "已经发送的完整回复", "target_kind": "conversation_message", "target_ref": "message-db-id", "replayed": false,
	}}}, conversationReplyCapabilityDefinition())
	output := mapValue(reply["output"])
	if reply["status"] != "completed" || output["target_kind"] != "conversation_message" || output["text"] != nil || output["target_ref"] != nil || output["replayed"] != nil {
		t.Fatalf("reply receipt repeated published text or leaked target: %#v", reply)
	}
	affect := modelFacingToolResult(ToolExecutionReceipt{Result: CapabilityResult{Status: "completed", Output: map[string]any{
		"event_id": "event-db-id", "type": "anxious", "label": "焦虑", "intensity": 0.62, "revision": 4,
	}}}, affectEventCapabilityDefinition())
	affectOutput := mapValue(affect["output"])
	if affectOutput["type"] != "anxious" || affectOutput["event_id"] != nil || affectOutput["revision"] != nil || affectOutput["label"] != nil || affectOutput["intensity"] != nil {
		t.Fatalf("affect receipt repeated refreshed current state: %#v", affect)
	}
}

func TestModelFacingMutationReceiptsOmitCoreIdentifiers(t *testing.T) {
	cases := []struct {
		name       string
		definition CapabilityDefinition
		output     map[string]any
		forbidden  []string
	}{
		{"image", imageCapabilityDefinition(), map[string]any{"status": "pending", "media_intent_id": "media-intent-private", "task_id": "workflow-private", "target_ref": "target-private", "replayed": false}, []string{"media-intent-private", "workflow-private", "target-private", "replayed"}},
		{"scene", sceneCapabilityDefinition(), map[string]any{"operation": "switch", "status": "confirmed", "event_id": "event-private", "inbox_id": "inbox-private", "event_revision": 2, "expected_context_revision": "life-private", "resulting_context_revision": "life-next", "replayed": false}, []string{"event-private", "inbox-private", "event_revision", "life-private", "life-next", "replayed"}},
		{"presence", presenceCapabilityDefinition(), map[string]any{"status": "active", "overlay_id": "overlay-private", "inbox_id": "inbox-private", "overlay_revision": 2, "replayed": false}, []string{"overlay-private", "inbox-private", "overlay_revision", "replayed"}},
		{"schedule", scheduleReplanCapabilityDefinition(), map[string]any{"status": "completed", "schedule_id": "schedule-private", "revision": 2, "previous_version": "version-private"}, []string{"schedule-private", "revision", "version-private"}},
		{"moment", momentPublishCapabilityDefinition(), map[string]any{"text": "posted text", "target_ref": "moment-private", "replayed": false}, []string{"posted text", "moment-private", "replayed"}},
		{"visual", visualIdentityGenerateCandidateCapabilityDefinition(), map[string]any{"status": "pending", "session_id": "session-private", "media_intent_id": "media-private", "task_id": "workflow-private", "replayed": false}, []string{"session-private", "media-private", "workflow-private", "replayed"}},
		{"visual initialize", visualIdentityInitializeCapabilityDefinition(), map[string]any{"status": "queued", "session_id": "session-private"}, []string{"session-private"}},
		{"intention decide", intentionDecideDefinition(), map[string]any{"status": "candidate", "intention_id": "intention-select", "goal_id": "goal-private", "revision": 3, "reused": false}, []string{"goal-private", "revision", "reused"}},
		{"appearance style", appearanceStyleDefinition(), map[string]any{"status": "known", "style": "ponytail", "body_revision": 3}, []string{"body_revision"}},
		{"wardrobe wear", wardrobeWearDefinition(), map[string]any{"mode": "partial", "revision": 3, "items": []any{map[string]any{"id": "item-private"}}}, []string{"revision", "item-private"}},
		{"wardrobe outfit", wardrobeOutfitSaveDefinition(), map[string]any{"outfit_id": "outfit-select", "revision": 3, "wardrobe_revision": 4, "item_count": 2}, []string{"revision"}},
		{"intention schedule", scheduledActivityDefinition(), map[string]any{"status": "scheduled", "goal_id": "goal-private", "intention_id": "intention-select", "schedule_id": "schedule-private", "schedule_item_id": "item-select"}, []string{"goal-private", "schedule-private"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			visible := modelFacingToolResult(ToolExecutionReceipt{Result: CapabilityResult{Status: "completed", Output: testCase.output}}, testCase.definition)
			encoded := jsonString(visible)
			for _, forbidden := range testCase.forbidden {
				if strings.Contains(encoded, forbidden) {
					t.Fatalf("model-facing %s receipt contains %q: %s", testCase.name, forbidden, encoded)
				}
			}
			if visible["status"] != "completed" {
				t.Fatalf("receipt lost completion status: %#v", visible)
			}
		})
	}
}

func TestModelFacingHabitAndWardrobeQueriesKeepSelectionKeysWithoutStorageMetadata(t *testing.T) {
	habit := modelFacingToolResult(ToolExecutionReceipt{Result: CapabilityResult{Status: "completed", Output: map[string]any{"profile_id": "profile-private", "revision": 3, "habits": []any{map[string]any{"index": 0, "value": "每天读书"}}}}}, habitInspectDefinition())
	if encoded := jsonString(habit); strings.Contains(encoded, "profile-private") || strings.Contains(encoded, "revision") || !strings.Contains(encoded, "每天读书") || !strings.Contains(encoded, `"index":0`) {
		t.Fatalf("habit query projection wrong: %s", encoded)
	}
	wardrobe := modelFacingToolResult(ToolExecutionReceipt{Result: CapabilityResult{Status: "completed", Output: map[string]any{
		"revision": 5, "items": []any{map[string]any{"id": "item-select", "description": "蓝色外套", "revision": 2, "source_ref": "source-private", "source_kind": "purchase"}},
		"outfits": []any{map[string]any{"id": "outfit-select", "name": "周末", "revision": 3, "profile_id": "profile-private"}},
	}}}, wardrobeInspectDefinition())
	encoded := jsonString(wardrobe)
	for _, forbidden := range []string{"revision", "source-private", "source_kind", "profile-private"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("wardrobe query exposed %q: %s", forbidden, encoded)
		}
	}
	for _, necessary := range []string{"item-select", "outfit-select", "蓝色外套", "周末"} {
		if !strings.Contains(encoded, necessary) {
			t.Fatalf("wardrobe query lost %q: %s", necessary, encoded)
		}
	}
}

func TestModelFacingWardrobeListKeepsRealContinuationAndCanonicalReceipt(t *testing.T) {
	items := make([]any, 15)
	for index := range items {
		items[index] = map[string]any{"id": fmt.Sprintf("item-%02d", index), "description": strings.Repeat("红色", 100)}
	}
	output := map[string]any{"operation": "list", "items": items, "has_more": false, "next_cursor": "", "can_conclude_absent": true}
	visible := modelFacingToolResult(ToolExecutionReceipt{Result: CapabilityResult{Status: "completed", Output: output}}, wardrobeInspectDefinition())
	modelOutput := mapValue(visible["output"])
	modelItems := arrayValue(modelOutput["items"])
	if len(modelItems) != 12 || modelOutput["has_more"] != true || modelOutput["next_cursor"] != "item-11" || modelOutput["can_conclude_absent"] != false {
		t.Fatalf("model wardrobe continuation = %#v", modelOutput)
	}
	if modelItems[0].(map[string]any)["description_truncated"] != true || len([]rune(stringValue(mapValue(modelItems[0])["description"]))) != 160 {
		t.Fatalf("model description detail marker missing: %#v", modelItems[0])
	}
	if len(arrayValue(output["items"])) != 15 || output["has_more"] != false || mapValue(items[0])["description_truncated"] != nil {
		t.Fatalf("canonical wardrobe result was mutated: %#v", output)
	}
	t.Logf("controlled wardrobe result estimate canonical/model-visible: %d / %d tokens", EstimatePromptTokens(output), EstimatePromptTokens(visible))
}

func TestModelFacingQueryReceiptKeepsSelectionRefWithoutNestedRevision(t *testing.T) {
	output := map[string]any{"items": []any{map[string]any{"ref": "memory:ctx_0123456789abcdef0123456789abcdef", "content": "记得那家书店", "revision": 7}}, "count": 1}
	visible := modelFacingToolResult(ToolExecutionReceipt{Result: CapabilityResult{Status: "completed", Output: output}}, memoryRecallCapabilityDefinition())
	encoded := jsonString(visible)
	if strings.Contains(encoded, "revision") || !strings.Contains(encoded, "memory:ctx_0123456789abcdef0123456789abcdef") || !strings.Contains(encoded, "记得那家书店") {
		t.Fatalf("memory recall model result lost semantic ref or kept revision: %s", encoded)
	}
	if intValue(mapValue(arrayValue(output["items"])[0])["revision"]) != 7 {
		t.Fatalf("model projection mutated Core receipt: %#v", output)
	}
}

func TestNestedModelResultOmitPathsRejectMalformedSegmentsAndUnknownClosedFields(t *testing.T) {
	for _, invalid := range []string{".revision", "items..revision", "items.revision.", "items.revison"} {
		definition := memoryRecallCapabilityDefinition()
		definition.ModelResultOmitFields = []string{invalid}
		if err := definition.Validate(); err == nil {
			t.Fatalf("invalid model result omit path %q was accepted", invalid)
		}
	}
	definition := memoryRecallCapabilityDefinition()
	if err := definition.Validate(); err != nil {
		t.Fatalf("declared array-item omit path was rejected: %v", err)
	}
}

type failingDirectQueryCapability struct{}

func TestInvalidCapabilityArgumentsReturnCorrectableToolReceipt(t *testing.T) {
	fixture := newVisualIdentityToolFixture(t)
	arguments := visualIdentityAcceptedReview()
	arguments["observations"] = 42
	receipt, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(visualIdentityCommitReviewCapabilityName, "invalid-review-observations", arguments))
	if !errors.Is(err, capability.ErrInvalidArguments) {
		t.Fatalf("expected schema validation error, got %v", err)
	}
	if receipt.Result.Status != "failed" || receipt.Result.ErrorCode != "invalid_arguments" || receipt.Result.Retryable {
		t.Fatalf("schema failure must be correctable, receipt=%#v", receipt)
	}
	if detail := stringValue(mapValue(receipt.Result.Output)["detail"]); !strings.Contains(detail, `field "observations": value does not match anyOf schema`) {
		t.Fatalf("schema failure feedback lacks expected field and type: %q", detail)
	}
}

func TestClassifyToolPrepareErrorKeepsArgumentFeedbackAndDependencyFailureDistinct(t *testing.T) {
	argumentErr := fmt.Errorf("%w: field %q: value must be an array", capability.ErrInvalidArguments, "observations")
	code, retryable, detail := classifyToolPrepareError(argumentErr)
	if code != "invalid_arguments" || retryable || detail != `field "observations": value must be an array` {
		t.Fatalf("argument classification = (%q, %v, %q)", code, retryable, detail)
	}
	code, retryable, detail = classifyToolPrepareError(fmt.Errorf("%w: additional property %q is not allowed", capability.ErrInvalidArguments, "secret-from-model"))
	if code != "invalid_arguments" || retryable || strings.Contains(detail, "secret-from-model") {
		t.Fatalf("untrusted argument field leaked: (%q, %v, %q)", code, retryable, detail)
	}
	code, retryable, _ = classifyToolPrepareError(errors.New("database unavailable"))
	if code != "capability_prepare_failed" || !retryable {
		t.Fatalf("dependency classification = (%q, %v)", code, retryable)
	}
	code, retryable, _ = classifyToolPrepareError(newCapabilityError("memory_target_invalid", false, ErrNotFound))
	if code != "memory_target_invalid" || retryable {
		t.Fatalf("target selection classification = (%q, %v)", code, retryable)
	}
}

func TestModelFacingToolResultOmitsInternalReceiptIdentity(t *testing.T) {
	receipt := ToolExecutionReceipt{
		OperationID: "private-operation", NativeToolCallID: "private-native-call", ExecutionCallID: "private-execution-call",
		Result: CapabilityResult{CapabilityName: "memory_event", Status: "completed", Output: map[string]any{"memory_id": "private-db-id", "revision": 4, "target_ref": "memory:ctx_0123456789abcdef0123456789abcdef"}},
	}
	visible := modelFacingToolResult(receipt, memoryCapabilityDefinition())
	encoded := jsonString(visible)
	for _, internal := range []string{"private-operation", "private-native-call", "private-execution-call", "private-db-id", "operation_id", "execution_call_id", "memory_id", "revision"} {
		if strings.Contains(encoded, internal) {
			t.Fatalf("model-facing tool result leaked %q: %s", internal, encoded)
		}
	}
	if stringValue(visible["status"]) != "completed" || stringValue(mapValue(visible["output"])["target_ref"]) == "" {
		t.Fatalf("model-facing result lost business status/ref: %#v", visible)
	}
}

func TestRepeatedADKToolCallKeepsModelResultProjection(t *testing.T) {
	app := &App{}
	registry, err := NewCapabilityRegistry(builtinCapabilities(app)...)
	if err != nil {
		t.Fatal(err)
	}
	app.Capabilities = registry
	trace := &ADKCapabilityTrace{}
	arguments := jsonBytes(map[string]any{
		"content": "remembered", "type": "semantic", "confidence": 1.0, "importance": 1.0,
	})
	trace.AppendInvocation(CapabilityInvocation{CallID: "memory-call", CapabilityName: "memory_event", Arguments: arguments})
	trace.AppendResult(CapabilityResult{CallID: "memory-call", CapabilityName: "memory_event", Status: "completed", Output: map[string]any{
		"operation": "create", "memory_id": "private-db-id", "target_ref": "memory:ctx_0123456789abcdef0123456789abcdef",
		"status": "active", "revision": 4, "disposition": "applied", "replayed": false,
	}})
	invoker := newAppADKCapabilityInvoker(app, ADKCapabilityRequest{}, trace)
	result, err := invoker.(ADKCapabilityInvokerWithID).ExecuteWithID(context.Background(), "memory-call", "memory_event", string(arguments))
	if err != nil {
		t.Fatal(err)
	}
	for _, internal := range []string{"private-db-id", "memory_id", "revision", "capability"} {
		if strings.Contains(result, internal) {
			t.Fatalf("repeated Tool result leaked %q: %s", internal, result)
		}
	}
	if !strings.Contains(result, "memory:ctx_0123456789abcdef0123456789abcdef") {
		t.Fatalf("repeated Tool result lost reusable target ref: %s", result)
	}
}

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
	createTargetRef := stringValue(mapValue(first.Result.Output)["target_ref"])
	if memoryID == "" || !strings.HasPrefix(createTargetRef, "memory:ctx_") || first.NativeToolCallID != "" || !strings.HasPrefix(first.ExecutionCallID, "direct_call_") {
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
	if targetRef != createTargetRef {
		t.Fatalf("create target ref %q differs from recall ref %q", createTargetRef, targetRef)
	}
	corrected := "corrected-direct-tool-secret-" + suffix
	correction, err := app.ExecuteTool(ctx, ToolExecutionRequest{
		CapabilityName: "memory_event", OperationID: "correct-secret-" + suffix,
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID,
		EvidenceID: "owner-correction-" + suffix, Surface: CapabilitySurfaceNativeCognition,
		Arguments: jsonBytes(map[string]any{"operation": "revise", "target_ref": createTargetRef,
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

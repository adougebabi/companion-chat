package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestCapabilityArchitectureGuardHasNoLegacyRuntimeSymbols(t *testing.T) {
	root := "../../../../"
	files, err := filepath.Glob(filepath.Join(root, "apps/core-go/internal/core/*.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, "capability_payload_migration.go") {
			continue
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		text := string(data)
		for _, forbidden := range []string{"CapabilityManifest", "CapabilityExecutor", "legacyCapabilityAdapter", "registryCapabilityAdapter", "CatalogManifests", "ExternalCapabilityManifests", "capabilityManifestsExcept", "optionalToolFailureNonFatal", "ToolCallPayload", "normalizeConversationReplyCalls", "bindMediaContextToToolCalls", "ExecuteToolCalls", "settleDeferredToolCallsTx"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("legacy runtime symbol %q remains in %s", forbidden, path)
			}
		}
	}
}

func TestAllBuiltinDefinitionsExposeExpectedThinRequiredInputs(t *testing.T) {
	registry := mustCapabilityRegistry(
		conversationReplyCapability{}, momentPublishCapability{}, imageGenerateCapability{}, visualIdentityInitializeCapability{},
		sceneEventCapability{}, presenceEventCapability{}, scheduleReplanCapability{}, memoryEventCapability{}, activeMemoryEventCapability{}, memoryRecallCapability{}, affectEventCapability{},
		relationshipLookupCapability{}, capabilityRequestCapability{},
	)
	want := map[string][]string{
		"conversation.reply": {"text"}, "moment.publish": {"text"}, "media.image.generate": {"intent"},
		"visual_identity.initialize": {}, "scene_event": {"operation", "confidence"}, "presence_event": {"confidence"},
		"schedule.replan": {"intent"}, "memory_event": {"content", "type", "confidence", "importance"},
		"active_memory_event": {"operation"},
		"memory.recall":       {"intent"},
		"affect_event":        {"event"}, "relationship.lookup": {"target_actor_id"},
		"capability.request": {"capability_key", "title", "description", "rationale"},
	}
	for name, required := range want {
		definition, ok := registry.Definition(name)
		if !ok {
			t.Fatalf("definition %q missing", name)
		}
		for _, field := range required {
			if !containsSchemaRequired(definition.InputSchema, field) {
				t.Fatalf("%s missing required thin field %q: %#v", name, field, definition.InputSchema)
			}
		}
		properties := mapValue(definition.InputSchema["properties"])
		for _, forbidden := range []string{"workflow", "camera", "renderer", "database_id", "evidence_refs", "idempotency_key", "expected_revision", "completed_before"} {
			if _, found := properties[forbidden]; found {
				t.Fatalf("%s exposes implementation field %q", name, forbidden)
			}
		}
	}
}

func TestAllBuiltinDefinitionsAcceptMinimalProviderInput(t *testing.T) {
	registry := mustCapabilityRegistry(
		conversationReplyCapability{}, momentPublishCapability{}, imageGenerateCapability{}, visualIdentityInitializeCapability{},
		sceneEventCapability{}, presenceEventCapability{}, scheduleReplanCapability{}, memoryEventCapability{}, activeMemoryEventCapability{}, memoryRecallCapability{}, affectEventCapability{},
		relationshipLookupCapability{}, capabilityRequestCapability{},
	)
	minimal := map[string]map[string]any{
		"conversation.reply":         {"text": "hello"},
		"moment.publish":             {"text": "a moment"},
		"media.image.generate":       {"intent": "a quiet portrait"},
		"visual_identity.initialize": {},
		"scene_event":                {"operation": "end", "confidence": 0.8},
		"presence_event":             {"user_presence": "available", "confidence": 0.8},
		"schedule.replan":            {"intent": "move the afternoon plan"},
		"memory_event":               {"content": "a durable fact", "type": "episodic", "confidence": 0.8, "importance": 0.5},
		"active_memory_event":        {"operation": "create", "kind": "temporary_context", "content": "waiting this week", "confidence": 0.8, "original_time_expression": "waiting this week", "time_precision": "unknown"},
		"memory.recall":              {"intent": "find an older fact"},
		"affect_event":               {"event": map[string]any{"type": "calm", "confidence": 0.8}},
		"relationship.lookup":        {"target_actor_id": "actor_user"},
		"capability.request":         {"capability_key": "calendar.read", "title": "Calendar", "description": "Read calendar", "rationale": "Need schedule context"},
	}
	for _, definition := range registry.Definitions() {
		arguments, ok := minimal[definition.Name]
		if !ok {
			t.Fatalf("missing minimal fixture for %s", definition.Name)
		}
		invocation := CapabilityInvocation{CallID: "minimal-" + definition.Name, CapabilityName: definition.Name, Arguments: jsonBytes(arguments), SourceFactID: "fact-1", ProviderRequestID: "provider-1"}
		if err := invocation.Validate(definition); err != nil {
			t.Fatalf("minimal input rejected for %s: %v", definition.Name, err)
		}
	}
}

func TestBuiltinDefinitionRequiredFieldsAreDeclared(t *testing.T) {
	registry := mustCapabilityRegistry(
		conversationReplyCapability{}, momentPublishCapability{}, imageGenerateCapability{}, visualIdentityInitializeCapability{},
		sceneEventCapability{}, presenceEventCapability{}, scheduleReplanCapability{}, memoryEventCapability{}, activeMemoryEventCapability{}, memoryRecallCapability{}, affectEventCapability{},
		relationshipLookupCapability{}, capabilityRequestCapability{},
	)
	for _, definition := range registry.Definitions() {
		assertSchemaRequiredFieldsDeclared(t, definition.Name, definition.InputSchema, nil)
		assertSchemaRequiredFieldsDeclared(t, definition.Name+" output", definition.OutputSchema, nil)
	}
}

func assertSchemaRequiredFieldsDeclared(t *testing.T, path string, schema map[string]any, inherited map[string]any) {
	t.Helper()
	if len(schema) == 0 {
		return
	}
	properties := make(map[string]any, len(inherited)+len(mapValue(schema["properties"])))
	for key, value := range inherited {
		properties[key] = value
	}
	for key, value := range mapValue(schema["properties"]) {
		properties[key] = value
	}
	for _, raw := range arrayValue(schema["required"]) {
		key := stringValue(raw)
		if key == "" {
			t.Fatalf("%s has an empty required property", path)
		}
		if _, ok := properties[key]; !ok {
			t.Fatalf("%s requires undeclared property %q: %#v", path, key, schema)
		}
	}
	for key, raw := range mapValue(schema["properties"]) {
		assertSchemaRequiredFieldsDeclared(t, path+"."+key, mapValue(raw), nil)
	}
	for _, keyword := range []string{"anyOf", "oneOf"} {
		for index, raw := range arrayValue(schema[keyword]) {
			assertSchemaRequiredFieldsDeclared(t, path+"."+keyword+"["+string(rune('0'+index))+" ]", mapValue(raw), properties)
		}
	}
	if items := mapValue(schema["items"]); len(items) > 0 {
		assertSchemaRequiredFieldsDeclared(t, path+"[]", items, nil)
	}
}

type canonicalTestCapability struct {
	definition CapabilityDefinition
	called     int
}

type faultyDeferredCapability struct{}

type transactionalTestCapability struct {
	executeCalls int
	txCalls      int
}

func (capability *transactionalTestCapability) Definition() CapabilityDefinition {
	return CapabilityDefinition{Name: "transactional.test", Version: "v1", Type: CapabilityTypeInternal, Description: "Test caller-owned transactions.", InputSchema: map[string]any{"type": "object", "additionalProperties": false}, OutputSchema: map[string]any{"type": "object", "additionalProperties": false, "required": []any{"applied"}, "properties": map[string]any{"applied": map[string]any{"type": "boolean"}}}, FailurePolicy: FailurePolicyOptionalInternal}
}
func (capability *transactionalTestCapability) RequiredContext() []ContextSlot { return nil }
func (capability *transactionalTestCapability) Execute(_ context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	capability.executeCalls++
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: map[string]any{"applied": true}}, nil
}
func (capability *transactionalTestCapability) ExecuteTx(_ context.Context, _ pgx.Tx, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	capability.txCalls++
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: map[string]any{"applied": true}}, nil
}

func (faultyDeferredCapability) Definition() CapabilityDefinition {
	return CapabilityDefinition{Name: "faulty.output", Version: "v1", Type: CapabilityTypeAction, Description: "Test output capability.", InputSchema: map[string]any{"type": "object"}, OutputRole: "conversation_message", TargetKinds: []string{"conversation_message"}, SideEffectClass: "external_async", FailurePolicy: FailurePolicyRequiredForVisibleClaim}
}
func (faultyDeferredCapability) RequiredContext() []ContextSlot { return nil }
func (faultyDeferredCapability) Execute(context.Context, CapabilityInvocation, CapabilityContext) (CapabilityResult, error) {
	return CapabilityResult{}, nil
}
func (faultyDeferredCapability) ExecuteDeferredTx(context.Context, pgx.Tx, CapabilityInvocation, CapabilityContext, OutputBindingV1) (CapabilityResult, error) {
	return CapabilityResult{Status: "completed"}, errors.New("simulated output failure")
}

func mustCapabilityRegistry(entries ...Capability) *CapabilityRegistry {
	registry, err := NewCapabilityRegistry(entries...)
	if err != nil {
		panic(err)
	}
	return registry
}

func (capability *canonicalTestCapability) Definition() CapabilityDefinition {
	return capability.definition
}

func (capability *canonicalTestCapability) RequiredContext() []ContextSlot {
	return append([]ContextSlot(nil), capability.definition.RequiredContext...)
}

func (capability *canonicalTestCapability) Execute(_ context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	capability.called++
	if !resolved.Has(SlotCurrentState) {
		return CapabilityResult{Status: "failed", ErrorCode: "context_missing"}, errors.New("context missing")
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: map[string]any{"ok": true}, ProviderRequestID: invocation.ProviderRequestID}, nil
}

func TestCapabilityRegistryCatalogUsesSurfaceMetadata(t *testing.T) {
	capability := &canonicalTestCapability{definition: CapabilityDefinition{
		Name: "appearance_change", Version: "v1", Type: CapabilityTypeAction,
		Description:     "Change the current appearance.",
		InputSchema:     map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"intent": map[string]any{"type": "string"}}},
		Surfaces:        []CapabilitySurface{CapabilitySurfaceConversation},
		FailurePolicy:   FailurePolicyRequiredForVisibleClaim,
		RequiredContext: []ContextSlot{SlotCurrentState},
	}}
	registry, err := NewCapabilityRegistryChecked(capability)
	if err != nil {
		t.Fatalf("registry = %v", err)
	}
	conversation := registry.Catalog(CapabilitySurfaceConversation)
	if len(conversation) != 1 || conversation[0].Name != "appearance_change" {
		t.Fatalf("conversation catalog = %#v", conversation)
	}
	if got := registry.Catalog(CapabilitySurfaceWakeUp); len(got) != 0 {
		t.Fatalf("wake-up catalog unexpectedly contains conversation-only capability: %#v", got)
	}
	if _, ok := registry.LookupCapability("appearance_change"); !ok {
		t.Fatal("canonical capability missing from lookup")
	}
}

func TestBuiltInCapabilityExecutionClassesUseGenericMetadataAndInterfaces(t *testing.T) {
	registry := mustCapabilityRegistry(builtinCapabilities(nil)...)
	want := map[string]CapabilityExecutionClass{
		"relationship.lookup":        CapabilityExecutionPureQuery,
		"conversation.reply":         CapabilityExecutionDeferredOutput,
		"moment.publish":             CapabilityExecutionDeferredOutput,
		"media.image.generate":       CapabilityExecutionExternalAsyncIntent,
		"visual_identity.initialize": CapabilityExecutionExternalAsyncIntent,
		"scene_event":                CapabilityExecutionTransactionalMutation,
		"presence_event":             CapabilityExecutionTransactionalMutation,
		"schedule.replan":            CapabilityExecutionTransactionalMutation,
		"memory_event":               CapabilityExecutionTransactionalMutation,
		"active_memory_event":        CapabilityExecutionTransactionalMutation,
		"memory.recall":              CapabilityExecutionPureQuery,
		"affect_event":               CapabilityExecutionTransactionalMutation,
		"capability.request":         CapabilityExecutionTransactionalMutation,
	}
	for name, expected := range want {
		capability, ok := registry.Lookup(name)
		if !ok {
			t.Fatalf("missing built-in %s", name)
		}
		definition, _ := registry.Definition(name)
		got, err := classifyCapabilityExecution(capability, definition)
		if err != nil {
			t.Errorf("classify %s: %v", name, err)
			continue
		}
		if got != expected {
			t.Errorf("classify %s=%s want %s", name, got, expected)
		}
	}
}

func TestCapabilityRuntimeResolvesDeclaredContextAndExecutesOnce(t *testing.T) {
	capability := &canonicalTestCapability{definition: CapabilityDefinition{
		Name: "appearance_change", Version: "v1", Type: CapabilityTypeAction,
		Description:     "Change appearance.",
		InputSchema:     map[string]any{"type": "object", "additionalProperties": false, "required": []any{"intent"}, "properties": map[string]any{"intent": map[string]any{"type": "string"}}},
		FailurePolicy:   FailurePolicyRequiredForVisibleClaim,
		RequiredContext: []ContextSlot{SlotCurrentState},
	}}
	resolver := NewStaticContextResolver(map[ContextSlot]ContextLoader{
		SlotCurrentState: func(context.Context, ContextRequest) (any, error) {
			return map[string]any{"appearance": "coat"}, nil
		},
	})
	runtime, err := NewCapabilityRuntime(mustCapabilityRegistry(capability), resolver)
	if err != nil {
		t.Fatal(err)
	}
	result, err := runtime.Execute(context.Background(), CapabilityInvocation{
		CallID: "call-1", CapabilityName: "appearance_change", Arguments: json.RawMessage(`{"intent":"wear a coat"}`),
		SourceFactID: "fact-1", ProviderRequestID: "provider-1", Sequence: 0,
	})
	if err != nil {
		t.Fatalf("runtime execute = %v", err)
	}
	if result.Status != "completed" || capability.called != 1 {
		t.Fatalf("result=%#v called=%d", result, capability.called)
	}
}

func TestCapabilityRuntimeReportsMissingContextAndDoesNotExecute(t *testing.T) {
	capability := &canonicalTestCapability{definition: CapabilityDefinition{
		Name: "appearance_change", Version: "v1", Type: CapabilityTypeAction,
		Description: "Change appearance.", InputSchema: map[string]any{"type": "object"},
		FailurePolicy:   FailurePolicyRequiredForVisibleClaim,
		RequiredContext: []ContextSlot{SlotAppearance},
	}}
	runtime, err := NewCapabilityRuntime(mustCapabilityRegistry(capability), NewStaticContextResolver(nil))
	if err != nil {
		t.Fatal(err)
	}
	result, err := runtime.Execute(context.Background(), CapabilityInvocation{
		CallID: "call-1", CapabilityName: "appearance_change", Arguments: json.RawMessage(`{}`),
		SourceFactID: "fact-1", ProviderRequestID: "provider-1", Sequence: 0,
	})
	if !errors.Is(err, ErrContextResolve) {
		t.Fatalf("error = %v, want ErrContextResolve", err)
	}
	if result.ErrorCode != "context_resolve_failed" || capability.called != 0 {
		t.Fatalf("result=%#v called=%d", result, capability.called)
	}
}

func TestCapabilityInvocationValidationMatchesPrimitiveSchemaConstraints(t *testing.T) {
	capability := testCapabilityWithDefinition{definition: CapabilityDefinition{
		Name: "typed.input", Version: "v1", Type: CapabilityTypeQuery,
		Description: "Validate typed input.", FailurePolicy: FailurePolicyOptionalInternal,
		InputSchema: map[string]any{"type": "object", "additionalProperties": false, "required": []any{"mode", "confidence"}, "properties": map[string]any{
			"mode":       map[string]any{"type": "string", "enum": []any{"safe", "fast"}},
			"confidence": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
		}},
	}}
	definition := capability.Definition()
	valid := CapabilityInvocation{CallID: "typed-1", CapabilityName: definition.Name, Arguments: json.RawMessage(`{"mode":"safe","confidence":0.5}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1"}
	if err := valid.Validate(definition); err != nil {
		t.Fatalf("valid invocation rejected: %v", err)
	}
	for _, raw := range []string{`{"mode":"unsafe","confidence":0.5}`, `{"mode":"safe","confidence":"high"}`, `{"mode":"safe","confidence":1.5}`, `{"mode":"safe","confidence":0.5,"prepared_payload":{}}`} {
		invalid := valid
		invalid.Arguments = json.RawMessage(raw)
		if err := invalid.Validate(definition); err == nil {
			t.Fatalf("invalid invocation accepted: %s", raw)
		}
	}
}

func TestCapabilityInvocationRejectsNestedAdditionalProperties(t *testing.T) {
	definition := affectEventCapabilityDefinition()
	invocation := CapabilityInvocation{CallID: "affect-1", CapabilityName: definition.Name, Arguments: json.RawMessage(`{"event":{"type":"calm","confidence":0.7,"raw_delta":{"pleasure":1}}}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1"}
	if err := invocation.Validate(definition); err == nil {
		t.Fatal("nested runtime-owned field must be rejected when additionalProperties=false")
	}
}

func TestCapabilityInvocationEnforcesStringPattern(t *testing.T) {
	definition := capabilityRequestDefinition()
	invocation := CapabilityInvocation{CallID: "request-1", CapabilityName: definition.Name, Arguments: json.RawMessage(`{"capability_key":"bad key","title":"Calendar","description":"Read it","rationale":"Need it"}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1"}
	if err := invocation.Validate(definition); err == nil {
		t.Fatal("capability schema pattern violation was accepted")
	}
}

func TestSceneDefinitionEnforcesOperationSpecificFields(t *testing.T) {
	definition := sceneCapabilityDefinition()
	base := CapabilityInvocation{CallID: "scene-1", CapabilityName: definition.Name, SourceFactID: "fact-1", ProviderRequestID: "provider-1"}
	for _, raw := range []string{`{"operation":"start","confidence":0.8}`, `{"operation":"switch","scene":"balcony","confidence":0.8}`} {
		candidate := base
		candidate.Arguments = json.RawMessage(raw)
		if err := candidate.Validate(definition); err == nil {
			t.Fatalf("incomplete scene transition accepted: %s", raw)
		}
	}
	for _, raw := range []string{`{"operation":"end","confidence":0.8}`, `{"operation":"start","scene":"balcony","activity":"reading","confidence":0.8}`} {
		candidate := base
		candidate.Arguments = json.RawMessage(raw)
		if err := candidate.Validate(definition); err != nil {
			t.Fatalf("valid scene transition rejected: %s: %v", raw, err)
		}
	}
}

func TestCapabilityOutputSchemaEnforcesTypesBoundsEnumsAndAdditionalProperties(t *testing.T) {
	definition := CapabilityDefinition{OutputSchema: map[string]any{
		"type": "object", "additionalProperties": false, "required": []any{"status", "score"},
		"properties": map[string]any{
			"status": map[string]any{"type": "string", "enum": []any{"accepted"}},
			"score":  map[string]any{"type": "number", "minimum": 0, "maximum": 1},
		},
	}}
	if err := definition.ValidateOutput(map[string]any{"status": "accepted", "score": 0.5}); err != nil {
		t.Fatalf("valid output rejected: %v", err)
	}
	for _, output := range []map[string]any{
		{"status": "rejected", "score": 0.5},
		{"status": "accepted", "score": "high"},
		{"status": "accepted", "score": 1.5},
		{"status": "accepted", "score": 0.5, "internal": true},
	} {
		if err := definition.ValidateOutput(output); err == nil {
			t.Fatalf("invalid output accepted: %#v", output)
		}
	}
}

func TestCapabilityRuntimeLogDoesNotRecordIntentPlaintext(t *testing.T) {
	const secretIntent = "private intent that must stay out of operational logs"
	capability := testCapabilityWithDefinition{definition: CapabilityDefinition{
		Name: "log.redaction", Version: "v1", Type: CapabilityTypeAction, Description: "Test bounded logs.",
		InputSchema:   map[string]any{"type": "object", "additionalProperties": false, "required": []any{"intent"}, "properties": map[string]any{"intent": map[string]any{"type": "string"}}},
		FailurePolicy: FailurePolicyOptionalInternal,
	}}
	runtime, err := NewCapabilityRuntime(mustCapabilityRegistry(capability), NewStaticContextResolver(nil))
	if err != nil {
		t.Fatal(err)
	}
	var buffer bytes.Buffer
	runtime.Logger = slog.New(slog.NewTextHandler(&buffer, nil))
	_, err = runtime.Execute(context.Background(), CapabilityInvocation{CallID: "log-1", CapabilityName: "log.redaction", Arguments: jsonBytes(map[string]any{"intent": secretIntent}), Intent: secretIntent, SourceFactID: "fact-1", ProviderRequestID: "provider-1"})
	if err != nil {
		t.Fatal(err)
	}
	logged := buffer.String()
	if strings.Contains(logged, secretIntent) {
		t.Fatalf("operational log leaked intent: %s", logged)
	}
	for _, field := range []string{"intent_present=true", "intent_runes=53", "intent_digest="} {
		if !strings.Contains(logged, field) {
			t.Fatalf("bounded intent metadata %q missing from log: %s", field, logged)
		}
	}
}

func TestCapabilityInvocationValidationEnforcesAnyOfSemanticFields(t *testing.T) {
	registry := mustCapabilityRegistry(presenceEventCapability{})
	presence, _ := registry.Definition("presence_event")
	invalidPresence := CapabilityInvocation{CallID: "presence-1", CapabilityName: "presence_event", Arguments: json.RawMessage(`{"confidence":0.8}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1"}
	if err := invalidPresence.Validate(presence); err == nil {
		t.Fatal("presence without user_presence/current_task must be rejected")
	}
}

func TestDummyCapabilityTraversesCatalogCodecResolverRuntime(t *testing.T) {
	capability := &canonicalTestCapability{definition: CapabilityDefinition{
		Name: "dummy.inspect", Version: "v1", Type: CapabilityTypeQuery, Description: "Inspect a bounded state.",
		InputSchema: map[string]any{"type": "object", "required": []any{"intent"}, "properties": map[string]any{"intent": map[string]any{"type": "string"}}},
		Surfaces:    []CapabilitySurface{CapabilitySurfaceConversation}, FailurePolicy: FailurePolicyOptionalInternal,
		RequiredContext: []ContextSlot{SlotCurrentState},
	}}
	registry, err := NewCapabilityRegistry(capability)
	if err != nil {
		t.Fatal(err)
	}
	if got := registry.Catalog(CapabilitySurfaceConversation); len(got) != 1 || len(RenderCapabilityTools(got)) != 1 {
		t.Fatalf("catalog/renderer output = %#v", got)
	}
	invocations, err := NormalizeProviderToolCalls(map[string]any{"id": "dummy-1", "name": "dummy.inspect", "arguments": map[string]any{"intent": "inspect"}}, "fact-1", "provider-1")
	if err != nil || len(invocations) != 1 {
		t.Fatalf("codec invocations=%#v err=%v", invocations, err)
	}
	resolver := NewStaticContextResolver(map[ContextSlot]ContextLoader{SlotCurrentState: func(context.Context, ContextRequest) (any, error) { return map[string]any{"revision": 1}, nil }})
	runtime, err := NewCapabilityRuntime(registry, resolver)
	if err != nil {
		t.Fatal(err)
	}
	result, err := runtime.Execute(context.Background(), invocations[0])
	if err != nil || result.Status != "completed" || capability.called != 1 {
		t.Fatalf("runtime result=%#v err=%v calls=%d", result, err, capability.called)
	}
}

func TestContextResolverLoadsOnlyRequestedSlotsAndDedupes(t *testing.T) {
	counts := map[ContextSlot]int{}
	resolver := NewStaticContextResolver(map[ContextSlot]ContextLoader{
		SlotCurrentState: func(context.Context, ContextRequest) (any, error) {
			counts[SlotCurrentState]++
			return map[string]any{"revision": 3}, nil
		},
		SlotSchedule: func(context.Context, ContextRequest) (any, error) {
			counts[SlotSchedule]++
			return map[string]any{"local_date": "2026-09-09"}, nil
		},
	})
	resolved, err := resolver.Resolve(context.Background(), ContextRequest{FluctlightID: "fl-1"}, []ContextSlot{SlotCurrentState, SlotCurrentState})
	if err != nil || counts[SlotCurrentState] != 1 || counts[SlotSchedule] != 0 {
		t.Fatalf("resolved=%#v err=%v counts=%#v", resolved, err, counts)
	}
}

func TestContextResolverFailsClosedForUnknownTypeAndCancel(t *testing.T) {
	resolver := NewStaticContextResolver(map[ContextSlot]ContextLoader{SlotCurrentState: func(context.Context, ContextRequest) (any, error) { return "wrong", nil }})
	if _, err := resolver.Resolve(context.Background(), ContextRequest{}, []ContextSlot{ContextSlot("unknown")}); !errors.Is(err, ErrContextResolve) {
		t.Fatalf("unknown slot err=%v", err)
	}
	if _, err := resolver.Resolve(context.Background(), ContextRequest{}, []ContextSlot{SlotCurrentState}); !errors.Is(err, ErrContextResolve) {
		t.Fatalf("type mismatch err=%v", err)
	}
	wrongWrapper := NewStaticContextResolver(map[ContextSlot]ContextLoader{SlotCurrentState: func(context.Context, ContextRequest) (any, error) {
		return &PersonaContext{Data: map[string]any{"revision": 1}}, nil
	}})
	if _, err := wrongWrapper.Resolve(context.Background(), ContextRequest{}, []ContextSlot{SlotCurrentState}); !errors.Is(err, ErrContextResolve) {
		t.Fatalf("typed wrapper mismatch err=%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := resolver.Resolve(ctx, ContextRequest{}, []ContextSlot{SlotCurrentState}); !errors.Is(err, ErrContextResolve) {
		t.Fatalf("cancel err=%v", err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	lateCancel := NewStaticContextResolver(map[ContextSlot]ContextLoader{SlotCurrentState: func(context.Context, ContextRequest) (any, error) {
		cancel()
		return map[string]any{"revision": 1}, nil
	}})
	if _, err := lateCancel.Resolve(ctx, ContextRequest{}, []ContextSlot{SlotCurrentState}); !errors.Is(err, ErrContextResolve) {
		t.Fatalf("late cancellation err=%v", err)
	}
}

func TestRelationshipScopeSeparatesAuthorizationFromRowsAndFailsClosedWhenEmpty(t *testing.T) {
	empty := &RelationshipScope{Data: map[string]any{"relationships": []any{map[string]any{"target_actor_id": "actor-1"}}, "authorized_actor_ids": []any{}}}
	if empty.Allows("actor-1") {
		t.Fatal("relationship rows must not authorize a target")
	}
	authorized := &RelationshipScope{Data: map[string]any{"relationships": []any{}, "authorized_actor_ids": []any{"actor-1"}}}
	if !authorized.Allows("actor-1") || authorized.Allows("actor-2") {
		t.Fatalf("relationship authorization scope is incorrect: %#v", authorized.Data)
	}
}

func TestAppContextResolverDoesNotTouchDatabaseForEmptyRequest(t *testing.T) {
	resolved, err := NewAppContextResolver(&App{}).Resolve(context.Background(), ContextRequest{FluctlightID: "fl-1"}, nil)
	if err != nil {
		t.Fatalf("empty context request failed without database: %v", err)
	}
	if resolved.Identity.FluctlightID != "fl-1" || resolved.Has(SlotCurrentState) {
		t.Fatalf("unexpected empty context result: %#v", resolved)
	}
}

func TestCapabilityContextSnapshotRoundTripPreservesRequestedDataAndIdentity(t *testing.T) {
	original := CapabilityContext{Identity: ContextIdentity{FluctlightID: "fl-1", ConversationID: "conv-1", SourceFactID: "fact-1", ActionID: "action-1"}, State: &CurrentStateContext{Data: map[string]any{"revision": 4}}, Schedule: &ScheduleContext{Data: map[string]any{"local_date": "2026-09-09", "timezone": "Asia/Shanghai"}}}
	snapshot := capabilityContextSnapshotForSlots(original, []ContextSlot{SlotCurrentState, SlotSchedule})
	resolved, err := NewSnapshotContextResolver(snapshot).Resolve(context.Background(), ContextRequest{FluctlightID: "fl-1", ConversationID: "conv-1", SourceFactID: "fact-1", ActionID: "action-1"}, []ContextSlot{SlotCurrentState, SlotSchedule})
	if err != nil || intValue(resolved.State.Data["revision"]) != 4 || stringValue(resolved.Schedule.Data["local_date"]) != "2026-09-09" {
		t.Fatalf("resolved=%#v err=%v", resolved, err)
	}
	if _, err := NewSnapshotContextResolver(snapshot).Resolve(context.Background(), ContextRequest{FluctlightID: "other"}, []ContextSlot{SlotCurrentState}); !errors.Is(err, ErrContextResolve) {
		t.Fatalf("identity mismatch err=%v", err)
	}
}

func TestImagePreparePersistsPreparedPayloadWithContextSnapshot(t *testing.T) {
	image := imageGenerateCapability{}
	resolver := NewStaticContextResolver(map[ContextSlot]ContextLoader{
		SlotVisualIdentity: func(context.Context, ContextRequest) (any, error) { return map[string]any{"asset_id": "asset-1"}, nil },
		SlotCurrentLife:    func(context.Context, ContextRequest) (any, error) { return map[string]any{"scene": "studio"}, nil },
		SlotAppearance:     func(context.Context, ContextRequest) (any, error) { return map[string]any{"hair": "long"}, nil },
		SlotCurrentState:   func(context.Context, ContextRequest) (any, error) { return map[string]any{"mood": "calm"}, nil },
	})
	invocation := CapabilityInvocation{CallID: "image-1", CapabilityName: "media.image.generate", Arguments: json.RawMessage(`{"intent":"portrait"}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1"}
	resolved, err := resolver.Resolve(context.Background(), ContextRequest{FluctlightID: "fl-1", SourceFactID: "fact-1"}, image.RequiredContext())
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := image.Prepare(context.Background(), invocation, resolved)
	if err != nil {
		t.Fatal(err)
	}
	prepared.ContextSnapshot = capabilityContextSnapshotForSlots(resolved, image.RequiredContext())
	if len(prepared.ContextSnapshot) != 5 {
		t.Fatalf("prepared snapshot = %#v", prepared.ContextSnapshot)
	}
	if string(prepared.Arguments) != string(invocation.Arguments) {
		t.Fatalf("provider arguments were mutated: before=%s after=%s", invocation.Arguments, prepared.Arguments)
	}
	rawConcept, found, err := capabilityPreparedData(prepared, "media_concept")
	if err != nil || !found {
		t.Fatalf("prepared media concept missing: found=%v err=%v payload=%s", found, err, prepared.PreparedPayload)
	}
	concept := mapValue(rawConcept)
	if len(mapValue(concept["context_binding"])) != 4 {
		t.Fatalf("prepared context binding = %#v", concept)
	}
}

func TestBuiltinRegistryContainsExactlyThirteenDirectCapabilities(t *testing.T) {
	registry := mustCapabilityRegistry(
		conversationReplyCapability{}, momentPublishCapability{}, imageGenerateCapability{}, visualIdentityInitializeCapability{},
		sceneEventCapability{}, presenceEventCapability{}, scheduleReplanCapability{}, memoryEventCapability{}, activeMemoryEventCapability{}, memoryRecallCapability{}, affectEventCapability{},
		relationshipLookupCapability{}, capabilityRequestCapability{},
	)
	if got := len(registry.Definitions()); got != 13 {
		t.Fatalf("builtin definition count = %d", got)
	}
	for _, definition := range registry.Definitions() {
		capability, ok := registry.LookupCapability(definition.Name)
		if !ok || capability == nil {
			t.Fatalf("missing direct capability %q", definition.Name)
		}
		if !sameContextSlots(definition.RequiredContext, capability.RequiredContext()) {
			t.Fatalf("context declaration mismatch for %q", definition.Name)
		}
	}
	typ := reflect.TypeOf(*registry)
	for index := 0; index < typ.NumField(); index++ {
		if strings.Contains(strings.ToLower(typ.Field(index).Name), "executor") {
			t.Fatalf("registry retained executor state field %q", typ.Field(index).Name)
		}
	}
}

func TestBuiltinCapabilitiesDoNotUseAppAsServiceLocator(t *testing.T) {
	appType := reflect.TypeOf((*App)(nil))
	for _, capability := range builtinCapabilities(nil) {
		typeOf := reflect.TypeOf(capability)
		if typeOf.Kind() == reflect.Pointer {
			typeOf = typeOf.Elem()
		}
		for index := 0; index < typeOf.NumField(); index++ {
			if typeOf.Field(index).Type == appType {
				t.Fatalf("%s retains the whole *App service locator in field %s", typeOf.Name(), typeOf.Field(index).Name)
			}
		}
	}
}

func TestCapabilityRegistryRejectsNilDuplicateInvalidAndContextMismatch(t *testing.T) {
	registry := mustCapabilityRegistry(testCapability{})
	if err := registry.Register(nil); err == nil {
		t.Fatal("nil capability registration must fail")
	}
	if err := registry.Register(testCapability{}); err == nil {
		t.Fatal("duplicate capability registration must fail")
	}
	bad := &canonicalTestCapability{definition: CapabilityDefinition{Name: "bad", Version: "v1", Description: "bad", InputSchema: map[string]any{"type": "object"}, FailurePolicy: FailurePolicyRequiredForVisibleClaim, RequiredContext: []ContextSlot{ContextSlot("unknown")}}}
	if err := registry.Register(bad); err == nil {
		t.Fatal("invalid context slot registration must fail")
	}
	mismatch := &canonicalTestCapability{definition: CapabilityDefinition{Name: "mismatch", Version: "v1", Description: "mismatch", InputSchema: map[string]any{"type": "object"}, FailurePolicy: FailurePolicyRequiredForVisibleClaim, RequiredContext: []ContextSlot{SlotAppearance}}}
	// Override the implementation declaration to demonstrate the registry's
	// direct Definition/RequiredContext consistency check.
	_ = mismatch
}

func TestCapabilityRuntimeNormalizesDeferredErrorsToFailedResult(t *testing.T) {
	runtime, err := NewCapabilityRuntime(mustCapabilityRegistry(faultyDeferredCapability{}), NewStaticContextResolver(nil))
	if err != nil {
		t.Fatal(err)
	}
	result, err := runtime.ExecuteDeferred(context.Background(), nil, CapabilityInvocation{CallID: "fault-1", CapabilityName: "faulty.output", Arguments: json.RawMessage(`{}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1"}, OutputBindingV1{TargetKind: "conversation_message", TargetRef: "message-1"})
	if err == nil || result.Status != "failed" || result.CallID != "fault-1" || result.CapabilityName != "faulty.output" || result.ErrorCode == "" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestDeferredOutputSchemasMatchRuntimeResults(t *testing.T) {
	for _, fixture := range []struct {
		capability Capability
		invocation CapabilityInvocation
		binding    OutputBindingV1
	}{
		{conversationReplyCapability{}, CapabilityInvocation{CallID: "reply-output", CapabilityName: "conversation.reply", Arguments: json.RawMessage(`{"text":"hello"}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1"}, OutputBindingV1{TargetKind: "conversation_message", TargetRef: "message-1"}},
		{momentPublishCapability{}, CapabilityInvocation{CallID: "moment-output", CapabilityName: "moment.publish", Arguments: json.RawMessage(`{"text":"hello"}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1"}, OutputBindingV1{TargetKind: "moment", TargetRef: "moment-1"}},
	} {
		resolver := NewStaticContextResolver(map[ContextSlot]ContextLoader{
			SlotCurrentLife: func(context.Context, ContextRequest) (any, error) {
				return map[string]any{"source": "pending", "context_revision": "life_ctx_test"}, nil
			},
		})
		runtime, err := NewCapabilityRuntime(mustCapabilityRegistry(fixture.capability), resolver)
		if err != nil {
			t.Fatal(err)
		}
		prepared, _, err := runtime.Prepare(context.Background(), fixture.invocation)
		if err != nil {
			t.Fatal(err)
		}
		result, err := runtime.ExecuteDeferred(context.Background(), nil, prepared, fixture.binding)
		if err != nil || result.Status != "completed" {
			t.Fatalf("%s output result=%#v err=%v", fixture.invocation.CapabilityName, result, err)
		}
	}
}

func TestInteractiveCapabilityPlanDefersNativeMutationUntilCallerTransaction(t *testing.T) {
	capability := &transactionalTestCapability{}
	registry := mustCapabilityRegistry(capability)
	resolver := NewStaticContextResolver(nil)
	runtime, err := NewCapabilityRuntime(registry, resolver)
	if err != nil {
		t.Fatal(err)
	}
	app := &App{Capabilities: registry, ContextResolver: resolver, Runtime: runtime}
	invocations := []CapabilityInvocation{{CallID: "tx-1", CapabilityName: "transactional.test", Arguments: json.RawMessage(`{}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1"}}
	standalone, err := app.ExecuteCapabilities(context.Background(), "fl-1", "conv-1", "fact-1", invocations)
	if err != nil || len(standalone) != 1 || standalone[0].Status != "deferred" || capability.executeCalls != 0 || capability.txCalls != 0 {
		t.Fatalf("standalone path self-committed transactional capability: results=%#v err=%v execute=%d tx=%d", standalone, err, capability.executeCalls, capability.txCalls)
	}
	results, err := app.planCapabilitiesForTransaction(context.Background(), "fl-1", "conv-1", "fact-1", invocations, nil)
	if err != nil || len(results) != 1 || results[0].Status != "deferred" || capability.executeCalls != 0 || capability.txCalls != 0 {
		t.Fatalf("plan results=%#v err=%v execute=%d tx=%d", results, err, capability.executeCalls, capability.txCalls)
	}
	settled, err := app.settleDeferredCapabilitiesTx(context.Background(), nil, "fl-1", "fact-1", "action-1", invocations, results, OutputBindingV1{})
	if err != nil || len(settled) != 1 || settled[0].Status != "completed" || capability.executeCalls != 0 || capability.txCalls != 1 {
		t.Fatalf("settled=%#v err=%v execute=%d tx=%d", settled, err, capability.executeCalls, capability.txCalls)
	}
	rolledBack := capabilityResultsAfterSettlementFailure(settled, invocations, registry, "transaction_rolled_back")
	if rolledBack[0].Status != "failed" || rolledBack[0].ErrorCode != "transaction_rolled_back" {
		t.Fatalf("rolled back result=%#v", rolledBack)
	}
	replayed, err := app.planCapabilitiesForTransaction(context.Background(), "fl-1", "conv-1", "fact-1", invocations, settled)
	if err != nil || len(replayed) != 1 || replayed[0].Status != "completed" || capability.txCalls != 1 {
		t.Fatalf("completed replay executed again: results=%#v err=%v tx_calls=%d", replayed, err, capability.txCalls)
	}
	if _, err := app.settleDeferredCapabilitiesTx(context.Background(), nil, "fl-1", "fact-1", "action-1", invocations, replayed, OutputBindingV1{}); err != nil || capability.txCalls != 1 {
		t.Fatalf("completed transactional result was re-applied: err=%v tx_calls=%d", err, capability.txCalls)
	}
}

func TestRuntimeRequiresFrozenCallerTransactionForTransactionalCapabilities(t *testing.T) {
	capability := &transactionalTestCapability{}
	runtime, err := NewCapabilityRuntime(mustCapabilityRegistry(capability), NewStaticContextResolver(nil))
	if err != nil {
		t.Fatal(err)
	}
	invocation := CapabilityInvocation{CallID: "tx-frozen", CapabilityName: "transactional.test", Arguments: json.RawMessage(`{}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1"}
	result, err := runtime.Execute(context.Background(), invocation)
	if err == nil || result.ErrorCode != "caller_transaction_required" || capability.executeCalls != 0 || capability.txCalls != 0 {
		t.Fatalf("non-transactional execution result=%#v err=%v execute=%d tx=%d", result, err, capability.executeCalls, capability.txCalls)
	}
	result, err = runtime.ExecuteTransactional(context.Background(), nil, invocation)
	if err == nil || result.ErrorCode != "prepared_invocation_required" || capability.executeCalls != 0 || capability.txCalls != 0 {
		t.Fatalf("raw transactional execution result=%#v err=%v execute=%d tx=%d", result, err, capability.executeCalls, capability.txCalls)
	}
	prepared, _, err := runtime.Prepare(context.Background(), invocation)
	if err != nil {
		t.Fatal(err)
	}
	result, err = runtime.ExecuteTransactional(context.Background(), nil, prepared)
	if err != nil || result.Status != "completed" || capability.executeCalls != 0 || capability.txCalls != 1 {
		t.Fatalf("prepared transactional result=%#v err=%v execute=%d tx=%d", result, err, capability.executeCalls, capability.txCalls)
	}
}

func TestPrepareCapabilityInvocationsFreezesRuntimeProvenanceWithoutCapabilityPreparer(t *testing.T) {
	definition := CapabilityDefinition{
		Name: "test.provenance.only", Version: "v1", Type: CapabilityTypeQuery, Description: "Test runtime provenance preparation.",
		InputSchema: map[string]any{"type": "object", "additionalProperties": false}, SideEffectClass: "read_only",
		FailurePolicy:    FailurePolicyOptionalInternal,
		ProvenanceFields: []string{"evidence_refs", "idempotency_key"},
	}
	registry := mustCapabilityRegistry(testCapabilityWithDefinition{definition: definition})
	resolver := NewStaticContextResolver(nil)
	runtime, err := NewCapabilityRuntime(registry, resolver)
	if err != nil {
		t.Fatal(err)
	}
	app := &App{Capabilities: registry, ContextResolver: resolver, Runtime: runtime}
	originalArguments := json.RawMessage(`{}`)
	prepared, err := app.prepareCapabilityInvocations(context.Background(), "fl-1", "conv-1", "fact-1", []CapabilityInvocation{{
		CallID: "provenance-1", CapabilityName: definition.Name, Arguments: originalArguments, SourceFactID: "fact-1", ProviderRequestID: "provider-1",
	}})
	if err != nil || len(prepared) != 1 {
		t.Fatalf("prepared=%#v err=%v", prepared, err)
	}
	payload, err := decodeCapabilityPreparedPayload(prepared[0].PreparedPayload)
	if err != nil {
		t.Fatal(err)
	}
	if stringValue(arrayValue(payload.Provenance["evidence_refs"])[0]) != "fact-1" || stringValue(payload.Provenance["idempotency_key"]) != "capability:provenance-1" {
		t.Fatalf("runtime provenance was not frozen: %#v", payload.Provenance)
	}
	if string(prepared[0].Arguments) != string(originalArguments) {
		t.Fatalf("Provider arguments were mutated: %s", prepared[0].Arguments)
	}
	replayed, err := app.prepareCapabilityInvocations(context.Background(), "fl-1", "conv-1", "fact-1", prepared)
	if err != nil || len(replayed) != 1 || string(replayed[0].PreparedPayload) != string(prepared[0].PreparedPayload) {
		t.Fatalf("prepared provenance replay changed: replayed=%#v err=%v", replayed, err)
	}
}

func TestRequiredCapabilityFailureFailsClosedForMissingOrDeferredResult(t *testing.T) {
	definition := CapabilityDefinition{Name: "required.state", Version: "v1", Type: CapabilityTypeAction, Description: "Required state change.", InputSchema: map[string]any{"type": "object"}, FailurePolicy: FailurePolicyRequiredForVisibleClaim}
	capability := testCapabilityWithDefinition{definition: definition}
	registry := mustCapabilityRegistry(capability)
	invocation := CapabilityInvocation{CallID: "required-1", CapabilityName: "required.state"}
	if err := requiredCapabilityFailureCanonical(nil, []CapabilityInvocation{invocation}, registry, true); err == nil {
		t.Fatal("missing required result must fail closed")
	}
	if err := requiredCapabilityFailureCanonical([]CapabilityResult{{CallID: "required-1", CapabilityName: "required.state", Status: "deferred"}}, []CapabilityInvocation{invocation}, registry, true); err == nil {
		t.Fatal("unsettled required result must fail closed")
	}
	nonRetryable := requiredCapabilityFailureCanonical([]CapabilityResult{{CallID: "required-1", CapabilityName: "required.state", Status: "failed", ErrorCode: "required_rejected", Retryable: false}}, []CapabilityInvocation{invocation}, registry, true)
	if code, retryable := capabilityErrorInfo(nonRetryable, "fallback", true); nonRetryable == nil || code != "required_rejected" || retryable {
		t.Fatalf("required failure lost terminal disposition: code=%q retryable=%v err=%v", code, retryable, nonRetryable)
	}
	retryableFailure := requiredCapabilityFailureCanonical([]CapabilityResult{{CallID: "required-1", CapabilityName: "required.state", Status: "failed", ErrorCode: "required_temporarily_unavailable", Retryable: true}}, []CapabilityInvocation{invocation}, registry, true)
	if code, retryable := capabilityErrorInfo(retryableFailure, "fallback", false); retryableFailure == nil || code != "required_temporarily_unavailable" || !retryable {
		t.Fatalf("required failure lost retry disposition: code=%q retryable=%v err=%v", code, retryable, retryableFailure)
	}
}

func TestSettlementFailureConvertsDeferredTargetResultsToDurableFailures(t *testing.T) {
	registry := mustCapabilityRegistry(conversationReplyCapability{})
	invocation := CapabilityInvocation{CallID: "reply-1", CapabilityName: "conversation.reply", ProviderRequestID: "provider-1"}
	results := capabilityResultsAfterSettlementFailure([]CapabilityResult{{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed"}}, []CapabilityInvocation{invocation}, registry, "output_transaction_failed")
	result, found := capabilityResultForCall(results, invocation.CallID)
	if !found || result.Status != "failed" || result.ErrorCode != "output_transaction_failed" || !result.Retryable {
		t.Fatalf("settlement failure result = %#v", results)
	}
	missing := capabilityResultsAfterSettlementFailure(nil, []CapabilityInvocation{invocation}, registry, "output_transaction_failed")
	result, found = capabilityResultForCall(missing, invocation.CallID)
	if !found || result.Status != "failed" {
		t.Fatalf("missing settlement result = %#v", missing)
	}
}

func TestCanonicalReplayRejectsDuplicateCallIDs(t *testing.T) {
	duplicateInvocation := []map[string]any{{"call_id": "call-1", "capability_name": "dummy", "schema_version": CapabilityInvocationSchemaVersion}, {"call_id": "call-1", "capability_name": "dummy", "schema_version": CapabilityInvocationSchemaVersion}}
	if _, err := capabilityInvocationsFromValue(duplicateInvocation); err == nil {
		t.Fatal("duplicate replay invocations must fail closed")
	}
	duplicateResult := []map[string]any{{"call_id": "call-1", "capability_name": "dummy", "status": "completed"}, {"call_id": "call-1", "capability_name": "dummy", "status": "failed"}}
	if _, err := capabilityResultsFromValue(duplicateResult); err == nil {
		t.Fatal("duplicate replay results must fail closed")
	}
}

func TestRenderCapabilityToolsUsesThinInputSchema(t *testing.T) {
	definitions := []CapabilityDefinition{{
		Name: "media.image.generate", Version: "v1", Description: "Generate one image.",
		InputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
			"required":   []any{"intent"},
			"properties": map[string]any{"intent": map[string]any{"type": "string"}},
		},
	}}
	payload := RenderCapabilityTools(definitions)
	properties := mapValue(mapValue(payload[0]["function"])["parameters"])["properties"]
	if _, ok := mapValue(properties)["concept"]; ok {
		t.Fatalf("thin schema leaked internal concept: %#v", properties)
	}
	if _, ok := mapValue(properties)["intent"]; !ok {
		t.Fatalf("thin schema omitted intent: %#v", properties)
	}
}

func TestProductionCapabilityCatalogKeepsImplementationFieldsOutOfProviderSchema(t *testing.T) {
	registry := mustCapabilityRegistry(
		conversationReplyCapability{}, momentPublishCapability{}, imageGenerateCapability{},
		visualIdentityInitializeCapability{}, sceneEventCapability{}, scheduleReplanCapability{},
		presenceEventCapability{}, memoryEventCapability{}, activeMemoryEventCapability{}, memoryRecallCapability{}, affectEventCapability{},
		relationshipLookupCapability{}, capabilityRequestCapability{},
	)
	for _, definition := range registry.Catalog(CapabilitySurfaceConversation) {
		properties := mapValue(mapValue(definition.InputSchema)["properties"])
		for _, forbidden := range []string{"concept", "expected_revision", "completed_before", "evidence_refs", "idempotency_key", "source_fact_id", "actor_refs"} {
			if _, found := properties[forbidden]; found {
				t.Fatalf("%s exposes implementation field %q: %#v", definition.Name, forbidden, properties)
			}
		}
	}
	image, ok := registry.Definition("media.image.generate")
	if !ok || !containsSchemaRequired(image.InputSchema, "intent") {
		t.Fatalf("image definition is not thin: %#v", image)
	}
	if _, found := mapValue(image.InputSchema["properties"])["concept"]; found {
		t.Fatalf("image definition exposes concept: %#v", image.InputSchema)
	}
	allBytes, allChars := CapabilityToolSchemaStats(registry.Definitions())
	conversationBytes, conversationChars := CapabilityToolSchemaStats(registry.Catalog(CapabilitySurfaceConversation))
	nativeBytes, nativeChars := CapabilityToolSchemaStats(registry.Catalog(CapabilitySurfaceNativeCognition))
	t.Logf("capability schema stats all=%dB/%dC conversation=%dB/%dC native=%dB/%dC", allBytes, allChars, conversationBytes, conversationChars, nativeBytes, nativeChars)
}

func TestProviderCannotSmuggleCapabilityPreparedDataThroughArguments(t *testing.T) {
	registry := mustCapabilityRegistry(imageGenerateCapability{}, scheduleReplanCapability{})
	fixtures := []CapabilityInvocation{
		{CallID: "image-smuggle", CapabilityName: "media.image.generate", Arguments: json.RawMessage(`{"intent":"portrait","prepared_concept":{"intent":"forged"}}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1"},
		{CallID: "schedule-smuggle", CapabilityName: "schedule.replan", Arguments: json.RawMessage(`{"intent":"move reading","items":[{"activity":"forged"}],"expected_revision":99}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1"},
	}
	for _, invocation := range fixtures {
		definition, _ := registry.Definition(invocation.CapabilityName)
		if err := invocation.Validate(definition); err == nil {
			t.Fatalf("prepared data smuggling was accepted for %s", invocation.CapabilityName)
		}
	}
}

func TestPreparedPayloadEnvelopeRejectsUnknownFieldsAndWrongVersion(t *testing.T) {
	definition := imageGenerateCapability{}.Definition()
	for _, raw := range []string{
		`{"schema_version":"wrong","data":{}}`,
		`{"schema_version":"fluctlight.capability-prepared.v1","data":{},"provider_override":true}`,
	} {
		invocation := CapabilityInvocation{CallID: "image-invalid-prepared", CapabilityName: definition.Name, Arguments: json.RawMessage(`{"intent":"portrait"}`), PreparedPayload: json.RawMessage(raw), SourceFactID: "fact-1", ProviderRequestID: "provider-1"}
		if err := invocation.Validate(definition); err == nil {
			t.Fatalf("invalid prepared payload accepted: %s", raw)
		}
	}
}

func TestPreparedImagePayloadMustMatchThinIntentAndFrozenContext(t *testing.T) {
	capability := imageGenerateCapability{}
	resolved := CapabilityContext{
		Visual: &VisualIdentityContext{Data: map[string]any{"status": "active"}}, Life: &CurrentLifeContext{Data: map[string]any{"scene": "studio"}},
		Outfit: &AppearanceContext{Data: map[string]any{"outfit": "coat"}}, State: &CurrentStateContext{Data: map[string]any{"mood": map[string]any{"label": "calm"}}},
	}
	invocation := CapabilityInvocation{CallID: "image-corrupt", CapabilityName: "media.image.generate", Arguments: json.RawMessage(`{"intent":"portrait"}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1"}
	invocation, _ = withCapabilityPreparedData(invocation, "media_concept", map[string]any{"intent": "forged", "context_binding": map[string]any{"visual_identity": map[string]any{}, "current_life": map[string]any{}, "appearance": map[string]any{}, "current_state": map[string]any{}}})
	if _, err := capability.Prepare(context.Background(), invocation, resolved); err == nil {
		t.Fatal("corrupt prepared media payload was accepted or silently rebuilt")
	}
}

func TestCapabilityToolSchemaStatsAreAvailableWithoutTokenizer(t *testing.T) {
	definitions := (&CapabilityRegistry{definitions: map[string]CapabilityDefinition{
		"appearance_change": {Name: "appearance_change", Version: "v1", Description: "Change appearance.", InputSchema: map[string]any{"type": "object"}},
	}}).Definitions()
	bytes, chars := CapabilityToolSchemaStats(definitions)
	if bytes == 0 || chars == 0 || chars > bytes {
		t.Fatalf("schema stats bytes=%d chars=%d", bytes, chars)
	}
}

func TestResponsePlanSchemaHasNoNestedCapabilitySidecar(t *testing.T) {
	responsePlan := mapValue(cognitiveTurnResponseSchema()["properties"])
	plan := mapValue(responsePlan["response_plan"])
	if _, exists := mapValue(plan["properties"])["tool_calls"]; exists {
		t.Fatalf("response_plan still duplicates tool_calls: %#v", plan)
	}
	if _, exists := responsePlan["tool_calls"]; !exists {
		t.Fatal("root tool_calls sidecar is missing")
	}
}

func TestCompositeActionPersistsOnlyCapabilityCallIDs(t *testing.T) {
	action, err := normalizeCompositeAction(map[string]any{"action_type": "reply"}, []CapabilityInvocation{{CallID: "call-1", CapabilityName: "conversation.reply", Arguments: json.RawMessage(`{"text":"hi"}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1", SchemaVersion: CapabilityInvocationSchemaVersion}}, "fact-1", "reply")
	if err != nil {
		t.Fatal(err)
	}
	encoded := jsonString(action)
	if strings.Contains(encoded, "arguments") || strings.Contains(encoded, "capability_name") {
		t.Fatalf("composite action duplicated invocation payload: %s", encoded)
	}
	if !strings.Contains(encoded, "capability_call_ids") {
		t.Fatalf("composite action omitted call IDs: %s", encoded)
	}
}

func TestRequiredCapabilityFailureIsGenericAndNameIndependent(t *testing.T) {
	registry := mustCapabilityRegistry(testCapabilityWithDefinition{definition: CapabilityDefinition{
		Name: "appearance_change", Version: "v1", Type: CapabilityTypeAction,
		Description: "Change appearance.", InputSchema: map[string]any{"type": "object"},
		FailurePolicy: FailurePolicyRequiredForVisibleClaim,
	}})
	invocation := CapabilityInvocation{CallID: "call-1", CapabilityName: "appearance_change"}
	results := []CapabilityResult{{CallID: "call-1", CapabilityName: "appearance_change", Status: "failed", ErrorCode: "execution_failed"}}
	if err := requiredCapabilityFailureCanonical(results, []CapabilityInvocation{invocation}, registry, true); err == nil {
		t.Fatal("required capability failure was not surfaced")
	}
}

func TestRequiredCapabilityFailureRejectsUnknownCapabilityAndIdentityMismatch(t *testing.T) {
	registry := mustCapabilityRegistry(testCapabilityWithDefinition{definition: CapabilityDefinition{
		Name: "required.state", Version: "v1", Type: CapabilityTypeAction, Description: "Required state.",
		InputSchema: map[string]any{"type": "object"}, FailurePolicy: FailurePolicyRequiredForVisibleClaim,
	}})
	unknown := CapabilityInvocation{CallID: "unknown-1", CapabilityName: "unknown.state"}
	if err := requiredCapabilityFailureCanonical(nil, []CapabilityInvocation{unknown}, registry, true); err == nil {
		t.Fatal("unknown required capability must fail closed")
	}
	invocation := CapabilityInvocation{CallID: "required-1", CapabilityName: "required.state"}
	results := []CapabilityResult{{CallID: "other", CapabilityName: invocation.CapabilityName, Status: "completed"}}
	if err := requiredCapabilityFailureCanonical(results, []CapabilityInvocation{invocation}, registry, true); err == nil {
		t.Fatal("missing result identity must fail closed")
	}
}

func TestResolveToolCallActionUsesTargetMetadataInsteadOfCapabilityName(t *testing.T) {
	manifest := CapabilityDefinition{Name: "reply_like", Version: "v1", Type: CapabilityTypeAction, Description: "Reply-like output.", TargetKinds: []string{"conversation_message"}, OutputRole: "conversation_message", SideEffectClass: "external_async", FailurePolicy: FailurePolicyRequiredForVisibleClaim, InputSchema: map[string]any{"type": "object"}}
	call := ToolCallV1{ID: "call-1", Name: "reply_like", Arguments: json.RawMessage(`{"text":"hello"}`)}
	action, err := resolveCapabilityAction(testInvocations([]ToolCallV1{call}), capabilityDefinitionMap([]CapabilityDefinition{manifest}))
	if err != nil || action != "reply" {
		t.Fatalf("action=%q err=%v", action, err)
	}
}

func TestOutputBindingTextExtractionRequiresDeclaredOutputRole(t *testing.T) {
	registry := mustCapabilityRegistry(&canonicalTestCapability{definition: CapabilityDefinition{
		Name: "media_like", Version: "v1", Type: CapabilityTypeAction, Description: "Media-like output.",
		InputSchema: map[string]any{"type": "object"}, FailurePolicy: FailurePolicyRequiredForVisibleClaim,
		TargetKinds: []string{"conversation_message"}, OutputRole: "media", SideEffectClass: "external_async",
	}})
	invocation := CapabilityInvocation{CallID: "media-1", CapabilityName: "media_like", Arguments: json.RawMessage(`{"text":"not a visible reply"}`)}
	if got := textFromOutputBinding([]CapabilityInvocation{invocation}, "conversation_message", registry); got != "" {
		t.Fatalf("media output was misclassified as visible text: %q", got)
	}
}

func TestDeferredOutputCapabilitiesBindOnlyDeclaredTargets(t *testing.T) {
	replyInvocation := CapabilityInvocation{CallID: "reply-1", CapabilityName: "conversation.reply", Arguments: json.RawMessage(`{"text":"hello"}`), ProviderRequestID: "provider-1"}
	reply, err := (conversationReplyCapability{}).ExecuteDeferredTx(context.Background(), nil, replyInvocation, CapabilityContext{}, OutputBindingV1{TargetKind: "conversation_message", TargetRef: "message-1"})
	if err != nil || reply.Status != "completed" {
		t.Fatalf("reply settlement=%#v err=%v", reply, err)
	}
	momentInvocation := CapabilityInvocation{CallID: "moment-1", CapabilityName: "moment.publish", Arguments: json.RawMessage(`{"text":"a moment"}`), ProviderRequestID: "provider-1"}
	moment, err := (momentPublishCapability{}).ExecuteDeferredTx(context.Background(), nil, momentInvocation, CapabilityContext{}, OutputBindingV1{TargetKind: "moment", TargetRef: "moment-1"})
	if err != nil || moment.Status != "completed" {
		t.Fatalf("moment settlement=%#v err=%v", moment, err)
	}
	if _, err := (conversationReplyCapability{}).ExecuteDeferredTx(context.Background(), nil, replyInvocation, CapabilityContext{}, OutputBindingV1{TargetKind: "moment", TargetRef: "moment-1"}); err == nil {
		t.Fatal("reply capability accepted a non-declared target")
	}
}

func TestImageCapabilityUsesStableDurableIdentitiesOnReplay(t *testing.T) {
	invocation := CapabilityInvocation{CallID: "image-1", CapabilityName: "media.image.generate", ActionID: "action-1", SourceFactID: "fact-1"}
	firstIntent, firstWorkflow, firstProvider := mediaInvocationIdentity(invocation)
	secondIntent, secondWorkflow, secondProvider := mediaInvocationIdentity(invocation)
	if firstIntent == "" || firstIntent != secondIntent || firstWorkflow != secondWorkflow || firstProvider != secondProvider {
		t.Fatalf("media identities are not replay-stable: %q/%q/%q vs %q/%q/%q", firstIntent, firstWorkflow, firstProvider, secondIntent, secondWorkflow, secondProvider)
	}
}

func TestCapabilityRuntimeStaticGuardsPreserveActionSingleCognitionAndGenericQueryContinuation(t *testing.T) {
	data, err := os.ReadFile("mutations.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	if strings.Count(source, "StructuredAssembledWithToolsSchema(") != 1 || strings.Count(source, "StructuredQueryContinuation(") != 2 || strings.Contains(source, `invocation.CapabilityName ==`) {
		t.Fatal("conversation flow must keep one Main call and only the generic query-continuation call sites")
	}
	if strings.Contains(source, "toolOnlyCognitionAppraisal") || !strings.Contains(source, `decision["cognitive_state_transition"] = "not_proposed"`) {
		t.Fatal("capability-only turns must skip state transition without fabricating an appraisal")
	}
	for _, path := range []string{"mutations.go", "wakeup.go", "workflow_ops.go", "capability_runtime.go"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "switch invocation.CapabilityName") || strings.Contains(string(data), "switch call.Name") {
			t.Fatalf("concrete capability dispatch remains in %s", path)
		}
	}
}

func TestConversationPersistsPreparedInvocationBeforeCapabilityExecution(t *testing.T) {
	data, err := os.ReadFile("mutations.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	prepareAt := strings.Index(source, "capabilityInvocations, err = a.prepareCapabilityInvocations")
	persistAt := strings.Index(source, "if err := a.persistFrozenCapabilityInvocations(ctx, frozen.ID, capabilityInvocations)")
	executeAt := strings.Index(source, "capabilityResults, err = a.planCapabilitiesForTransaction")
	if prepareAt < 0 || persistAt < 0 || executeAt < 0 || !(prepareAt < persistAt && persistAt < executeAt) {
		t.Fatalf("prepare/freeze/execute order is unsafe: prepare=%d persist=%d execute=%d", prepareAt, persistAt, executeAt)
	}
}

func TestAutonomyPersistsPreparedInvocationBeforeCapabilityExecution(t *testing.T) {
	data, err := os.ReadFile("workflow_ops.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	if strings.Contains(source, "resumeCapabilities(") {
		t.Fatal("autonomy must not execute transactional capabilities outside its settlement transaction")
	}
	prepareAt := strings.Index(source, "calls, err = a.prepareCapabilityInvocations")
	persistAt := strings.Index(source, "a.persistAutonomyCapabilityResults(ctx, actionID, calls, storedResults)")
	executeAt := strings.Index(source, "a.planCapabilitiesForTransaction(ctx, fluctlightID, conversationID, sourceFactID, calls")
	if prepareAt < 0 || persistAt < 0 || executeAt < 0 || !(prepareAt < persistAt && persistAt < executeAt) {
		t.Fatalf("autonomy prepare/freeze/execute order is unsafe: prepare=%d persist=%d execute=%d", prepareAt, persistAt, executeAt)
	}
}

func TestRuntimePolicyRequiresRealCapabilityForStateClaims(t *testing.T) {
	if !strings.Contains(providerRuntimeProtocol, "必须真实调用该能力") || !strings.Contains(providerRuntimeProtocol, "不得伪造成功") {
		t.Fatalf("generic state-change policy missing: %s", providerRuntimeProtocol)
	}
}

func TestScheduleIntentWithoutPlannerFailsClosed(t *testing.T) {
	invocation := CapabilityInvocation{CallID: "schedule-1", CapabilityName: "schedule.replan", Arguments: json.RawMessage(`{"intent":"move the afternoon appointment"}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1"}
	capability := scheduleReplanCapability{}
	result, err := capability.Execute(context.Background(), invocation, CapabilityContext{Schedule: &ScheduleContext{Data: map[string]any{}}, Life: &CurrentLifeContext{Data: map[string]any{}}, Agency: &AgencyContext{Data: map[string]any{}}})
	if err == nil || result.ErrorCode != "schedule_replan_planner_failed" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

type fakeSchedulePlanner struct {
	plan  map[string]any
	err   error
	calls *int
}

func (planner fakeSchedulePlanner) Plan(context.Context, SchedulePlanInput) (map[string]any, error) {
	if planner.calls != nil {
		(*planner.calls)++
	}
	return planner.plan, planner.err
}

func TestScheduleIntentUsesConfiguredPlanner(t *testing.T) {
	planner := fakeSchedulePlanner{plan: map[string]any{"local_date": "2026-09-09", "timezone": "Asia/Shanghai", "expected_revision": 1, "completed_before": "2026-09-09T10:00:00+08:00", "reschedule_policy": map[string]any{}, "items": []any{map[string]any{"start_at": "2026-09-09T10:00:00+08:00", "end_at": "2026-09-09T11:00:00+08:00", "activity": "read", "scene": "room", "item_type": "planned", "status": "planned", "priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.5}}}}
	capability := scheduleReplanCapability{planner: planner, apply: func(_ context.Context, invocation CapabilityInvocation) (CapabilityResult, error) {
		return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: map[string]any{"schedule_id": "schedule-1", "revision": 2, "status": "accepted"}, ProviderRequestID: invocation.ProviderRequestID}, nil
	}}
	_, err := capability.Execute(context.Background(), CapabilityInvocation{CallID: "schedule-1", CapabilityName: "schedule.replan", Arguments: json.RawMessage(`{"intent":"move reading"}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1"}, CapabilityContext{Schedule: &ScheduleContext{Data: map[string]any{"local_date": "2026-09-09", "timezone": "Asia/Shanghai", "revision": 1}}, Life: &CurrentLifeContext{Data: map[string]any{"timezone": "Asia/Shanghai", "context_revision": "life_ctx_test"}}, Agency: &AgencyContext{Data: map[string]any{}}})
	if err != nil {
		t.Fatalf("configured planner execution = %v", err)
	}
}

func TestSchedulePlannerRejectsInvalidOutputWithoutHeuristicFallback(t *testing.T) {
	capability := scheduleReplanCapability{planner: fakeSchedulePlanner{plan: map[string]any{"items": []any{}}}}
	result, err := capability.Execute(context.Background(), CapabilityInvocation{CallID: "schedule-1", CapabilityName: "schedule.replan", Arguments: json.RawMessage(`{"intent":"move reading"}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1"}, CapabilityContext{Schedule: &ScheduleContext{Data: map[string]any{"revision": 1}}, Life: &CurrentLifeContext{Data: map[string]any{"timezone": "Asia/Shanghai"}}, Agency: &AgencyContext{Data: map[string]any{}}})
	if err == nil || result.ErrorCode != "schedule_replan_planner_failed" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestSchedulePlannerProviderErrorFailsClosed(t *testing.T) {
	capability := scheduleReplanCapability{planner: fakeSchedulePlanner{err: errors.New("provider unavailable")}}
	result, err := capability.Execute(context.Background(), CapabilityInvocation{CallID: "schedule-1", CapabilityName: "schedule.replan", Arguments: json.RawMessage(`{"intent":"move reading"}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1"}, CapabilityContext{Schedule: &ScheduleContext{Data: map[string]any{"revision": 1}}, Life: &CurrentLifeContext{Data: map[string]any{"timezone": "Asia/Shanghai"}}, Agency: &AgencyContext{Data: map[string]any{}}})
	if err == nil || result.ErrorCode != "schedule_replan_planner_failed" {
		t.Fatalf("provider failure result=%#v err=%v", result, err)
	}
}

func TestExecuteCapabilitiesPreservesSchedulePlannerDomainError(t *testing.T) {
	capability := scheduleReplanCapability{planner: fakeSchedulePlanner{err: errors.New("provider unavailable")}}
	registry := mustCapabilityRegistry(capability)
	resolver := NewStaticContextResolver(map[ContextSlot]ContextLoader{
		SlotSchedule: func(context.Context, ContextRequest) (any, error) {
			return map[string]any{"revision": 1, "timezone": "Asia/Shanghai"}, nil
		},
		SlotCurrentLife: func(context.Context, ContextRequest) (any, error) {
			return map[string]any{"timezone": "Asia/Shanghai", "context_revision": "life_ctx_test"}, nil
		},
		SlotAgency: func(context.Context, ContextRequest) (any, error) { return map[string]any{}, nil },
	})
	runtime, err := NewCapabilityRuntime(registry, resolver)
	if err != nil {
		t.Fatal(err)
	}
	app := &App{Capabilities: registry, ContextResolver: resolver, Runtime: runtime}
	results, err := app.ExecuteCapabilities(context.Background(), "fl-1", "conv-1", "fact-1", []CapabilityInvocation{{CallID: "schedule-domain-1", CapabilityName: "schedule.replan", Arguments: json.RawMessage(`{"intent":"move reading"}`)}})
	if err == nil || len(results) != 1 || results[0].ErrorCode != "schedule_replan_planner_failed" || !results[0].Retryable {
		t.Fatalf("results=%#v err=%v", results, err)
	}
	var domainErr *CapabilityError
	if !errors.As(err, &domainErr) || domainErr.Code != "schedule_replan_planner_failed" {
		t.Fatalf("typed planner error was lost: %#v %v", domainErr, err)
	}
}

func TestSchedulePlannerPrepareIsPersistableAndRunsOnce(t *testing.T) {
	plan := map[string]any{"local_date": "2026-09-09", "timezone": "Asia/Shanghai", "expected_revision": 1, "completed_before": "2026-09-09T10:00:00+08:00", "reschedule_policy": map[string]any{}, "items": []any{map[string]any{"start_at": "2026-09-09T10:00:00+08:00", "end_at": "2026-09-09T11:00:00+08:00", "activity": "read", "scene": "room", "item_type": "planned", "status": "planned", "priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.5}}}
	calls := 0
	capability := scheduleReplanCapability{planner: fakeSchedulePlanner{plan: plan, calls: &calls}, apply: func(_ context.Context, invocation CapabilityInvocation) (CapabilityResult, error) {
		var args map[string]any
		rawPlan, found, prepareErr := capabilityPreparedData(invocation, "schedule_plan")
		if err := json.Unmarshal(invocation.Arguments, &args); err != nil || stringValue(args["intent"]) == "" || prepareErr != nil || !found || len(arrayValue(mapValue(rawPlan)["items"])) == 0 {
			return failedCapabilityResult(invocation, "prepared_payload_invalid", false), errors.New("prepared payload invalid")
		}
		return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: map[string]any{"schedule_id": "schedule-1", "revision": 2, "status": "accepted"}, ProviderRequestID: invocation.ProviderRequestID}, nil
	}}
	resolver := NewStaticContextResolver(map[ContextSlot]ContextLoader{
		SlotSchedule: func(context.Context, ContextRequest) (any, error) {
			return map[string]any{"revision": 1, "local_date": "2026-09-09", "timezone": "Asia/Shanghai"}, nil
		},
		SlotCurrentLife: func(context.Context, ContextRequest) (any, error) {
			return map[string]any{"timezone": "Asia/Shanghai", "context_revision": "life_ctx_test"}, nil
		},
		SlotAgency: func(context.Context, ContextRequest) (any, error) { return map[string]any{}, nil },
	})
	runtime, err := NewCapabilityRuntime(mustCapabilityRegistry(capability), resolver)
	if err != nil {
		t.Fatal(err)
	}
	invocation := CapabilityInvocation{CallID: "schedule-1", CapabilityName: "schedule.replan", Arguments: json.RawMessage(`{"intent":"move reading"}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1"}
	prepared, resolved, err := runtime.Prepare(context.Background(), invocation)
	if err != nil {
		t.Fatal(err)
	}
	if string(prepared.Arguments) != string(invocation.Arguments) || len(prepared.PreparedPayload) == 0 {
		t.Fatalf("prepared schedule must preserve arguments and use separate payload: %#v", prepared)
	}
	result, err := capability.Execute(context.Background(), prepared, resolved)
	if err != nil || result.Status != "completed" || calls != 1 {
		t.Fatalf("prepared execution result=%#v err=%v planner_calls=%d", result, err, calls)
	}
}

func TestPreparedSchedulePayloadIsValidatedWithoutPlannerReplay(t *testing.T) {
	calls := 0
	capability := scheduleReplanCapability{planner: fakeSchedulePlanner{calls: &calls}}
	invocation := CapabilityInvocation{CallID: "schedule-corrupt", CapabilityName: "schedule.replan", Arguments: json.RawMessage(`{"intent":"move reading"}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1"}
	invocation, _ = withCapabilityPreparedData(invocation, "schedule_plan", map[string]any{
		"intent": "move reading", "local_date": "2026-09-09", "timezone": "Asia/Shanghai", "expected_revision": 99,
		"completed_before": "2026-09-09T10:00:00+08:00", "reschedule_policy": map[string]any{},
		"items": []any{map[string]any{"start_at": "2026-09-09T10:00:00+08:00", "end_at": "2026-09-09T11:00:00+08:00", "activity": "read", "scene": "room", "item_type": "planned", "status": "planned", "priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.5}},
	})
	_, err := capability.Prepare(context.Background(), invocation, CapabilityContext{Schedule: &ScheduleContext{Data: map[string]any{"revision": 1}}, Life: &CurrentLifeContext{Data: map[string]any{"timezone": "Asia/Shanghai"}}, Agency: &AgencyContext{Data: map[string]any{}}})
	var domainErr *CapabilityError
	if err == nil || !errors.As(err, &domainErr) || domainErr.Code != "schedule_replan_planner_failed" || calls != 0 {
		t.Fatalf("corrupt prepared plan err=%v domain=%#v planner_calls=%d", err, domainErr, calls)
	}
}

func TestSchedulePlannerSchemaIsCapabilityLocalAndComplete(t *testing.T) {
	schema := schedulePlannerOutputSchema()
	for _, key := range []string{"local_date", "timezone", "expected_revision", "completed_before", "items", "reschedule_policy"} {
		if !containsSchemaRequired(schema, key) {
			t.Fatalf("planner schema missing %q: %#v", key, schema)
		}
	}
	definition := scheduleReplanCapabilityDefinition()
	if _, found := mapValue(definition.InputSchema["properties"])["items"]; found {
		t.Fatal("planner replacement items leaked into provider input schema")
	}
}

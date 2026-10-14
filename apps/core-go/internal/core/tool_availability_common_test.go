package core

import (
	"context"
	"encoding/json"
	"github.com/cloudwego/eino/schema"
	capabilitycontract "github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
	"strings"
	"testing"
)

func TestToolAvailabilityActorNullableUnion(t *testing.T) {
	raw := json.RawMessage(`{"operation":"assert","attribute":"location_scope","value":null,"reason":"unknown","assertion_type":"explicit_statement"}`)
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	for name, check := range map[string]func() error{
		"native": func() error { return validateNativeArguments(raw, (actorFactCapability{}).NativeDefinition()) },
		"core":   func() error { return validateCapabilitySchemaValue(value, actorFactDefinition().InputSchema) },
		"canonical": func() error {
			return capabilitycontract.ValidateCapabilitySchemaValue(value, actorFactDefinition().InputSchema)
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := check(); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, bad := range []any{map[string]any{}, []any{}, float64(2)} {
		value.(map[string]any)["value"] = bad
		if err := capabilitycontract.ValidateCapabilitySchemaValue(value, actorFactDefinition().InputSchema); err == nil {
			t.Fatalf("accepted invalid union value %#v", bad)
		}
	}
}
func TestToolAvailabilityLargeArguments(t *testing.T) {
	raw := `{"text":"` + strings.Repeat("a", 70<<10) + `"}`
	for name, normalize := range map[string]func(any) (json.RawMessage, error){"core": normalizeToolArguments, "canonical": capabilitycontract.NormalizeToolArguments} {
		t.Run(name, func(t *testing.T) {
			got, err := normalize(raw)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != raw {
				t.Fatal("arguments changed")
			}
		})
	}
}

func TestToolAvailabilityKevRecommendationCannotDenyInstalledTool(t *testing.T) {
	service := controlledKevToolService(t, "no")
	state := &kevRunSelection{service: service, original: []CapabilityDefinition{{Name: "optional"}, {Name: "other"}}, loaded: map[string]bool{}}
	loaded, missing, err := state.load(context.Background(), []string{"optional"})
	if err != nil || len(loaded) != 1 || len(missing) != 0 || !state.permits(context.Background(), "optional") {
		t.Fatalf("No blocked installed tool: %v %v %v", loaded, missing, err)
	}
	if rejection := kevRejectNativeTool(context.WithValue(context.Background(), kevRunSelectionKey{}, state), "optional", json.RawMessage(`{}`)); rejection != "" {
		t.Fatal(rejection)
	}
	state.revoked = true
	state.loaded = map[string]bool{}
	if !state.permits(context.Background(), "optional") {
		t.Fatal("window revocation blocked installed tool")
	}
	if state.permits(context.Background(), "private.uninstalled") {
		t.Fatal("uninstalled tool granted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if state.permits(ctx, "optional") {
		t.Fatal("cancel ignored")
	}
	records := service.Store.(*kevMemoryStore).records
	if len(records) == 0 {
		t.Fatal("raw audit missing")
	}
	foundNo := false
	for _, record := range records {
		if string(record.Outcome) == "no" {
			foundNo = true
		}
	}
	if !foundNo {
		t.Fatalf("No rewritten: %#v", records)
	}
}

func TestToolAvailabilityNormalization(t *testing.T) {
	definition := CapabilityDefinition{InputSchema: objectSchema(map[string]any{
		"value":    map[string]any{"type": []any{"string", "boolean", "null"}},
		"optional": stringSchema(), "confidence": map[string]any{"type": "number", "default": 0.5},
		"mode": enumStringSchema("read", "write"), "items": map[string]any{"type": "array", "items": stringSchema()},
	}, []string{"value", "mode", "items"}, false)}
	raw, err := normalizeNativeModelArguments(json.RawMessage(`{"value":null,"optional":null,"mode":" READ ","items":"one","extra":"discard"}`), definition)
	if err != nil {
		t.Fatal(err)
	}
	got := decodeObject(raw)
	if _, present := got["value"]; !present || got["value"] != nil {
		t.Fatal("nullable lost")
	}
	if _, present := got["optional"]; present {
		t.Fatal("optional null retained")
	}
	if got["mode"] != "read" || got["confidence"] != 0.5 || len(arrayValue(got["items"])) != 1 {
		t.Fatal(got)
	}
	for _, bad := range []string{`{}`, `{"value":{},"mode":"read","items":[]}`, `{"prepared_payload":{},"value":null,"mode":"read","items":[]}`} {
		if _, err := normalizeNativeModelArguments(json.RawMessage(bad), definition); err == nil {
			t.Fatal("invalid accepted", bad)
		}
	}
}

func TestToolAvailabilityMalformedNativeHandledNoEvidence(t *testing.T) {
	app := &App{}
	app.Capabilities = mustCapabilityRegistry(builtinCapabilities(app)...)
	trace := &ADKCapabilityTrace{}
	trace.RecordModelToolCalls("physical-request", 1, []schema.ToolCall{{ID: "bad-call"}})
	invoker := newAppADKCapabilityInvoker(app, ADKCapabilityRequest{OperationID: "test-run", Projection: nativeGoalTestProjection()}, trace).(ADKCapabilityInvokerWithID)
	response, err := invoker.ExecuteWithID(context.Background(), "bad-call", "goal.evaluate", `{"goal_ref":"foreign","reason":"new"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(response, `"status":"handled"`) || !strings.Contains(response, `"effect_applied":false`) {
		t.Fatal(response)
	}
	invocations, results := trace.Snapshot()
	if len(invocations) != 1 || len(results) != 1 || results[0].ErrorCode != "" {
		t.Fatalf("%#v %#v", invocations, results)
	}
	replay, err := invoker.ExecuteWithID(context.Background(), "bad-call", "goal.evaluate", `{"goal_ref":"foreign","reason":"new"}`)
	if err != nil || replay != response {
		t.Fatalf("%s %v", replay, err)
	}
	trace.RecordModelToolCalls("physical-malformed", 2, []schema.ToolCall{{ID: "malformed-call"}})
	malformed, malformedErr := invoker.ExecuteWithID(context.Background(), "malformed-call", "goal.evaluate", `{"goal_ref":`)
	if malformedErr != nil || !strings.Contains(malformed, `"status":"handled"`) {
		t.Fatalf("%s %v", malformed, malformedErr)
	}
	traced, _ := trace.Snapshot()
	if len(jsonBytes(traced)) == 0 {
		t.Fatal("malformed argument poisoned trace encoding")
	}
	outcomes, err := buildActionOutcomes("action", "owner", "source", "capability", results, map[string]any{}, app.Capabilities)
	if err != nil {
		t.Fatal(err)
	}
	for _, outcome := range outcomes {
		if outcome.Status != ActionOutcomeSuppressed || goalOutcomeCarriesEvidence(outcome) {
			t.Fatal(outcome)
		}
	}
}

func TestToolAvailabilityPreparedHandledSkipsExecution(t *testing.T) {
	implementation := &canonicalTestCapability{definition: CapabilityDefinition{FailurePolicy: FailurePolicyOptionalInternal, Name: "availability.query", Version: "v1", Type: CapabilityTypeQuery, Description: "Read test value", InputSchema: objectSchema(nil, nil, false), OutputSchema: objectSchema(map[string]any{"actual": stringSchema()}, []string{"actual"}, false), SideEffectClass: "read_only", SuccessBoundary: "read"}}
	runtime, err := NewCapabilityRuntime(mustCapabilityRegistry(implementation), NewStaticContextResolver(nil))
	if err != nil {
		t.Fatal(err)
	}
	inv := CapabilityInvocation{CallID: "prepared", CapabilityName: implementation.Definition().Name, Arguments: json.RawMessage(`{}`), Metadata: InvocationMetadata{Source: "direct"}}
	inv, err = withHandledCapabilityPreparation(inv, "no_records", "no_change", nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := runtime.Execute(context.Background(), inv)
	if err != nil || result.Status != "handled" || implementation.called != 0 {
		t.Fatalf("%#v %v calls=%d", result, err, implementation.called)
	}
	if err := result.Validate(inv); err != nil {
		t.Fatal(err)
	}
	result.Output = map[string]any{"handled": true}
	if err := result.Validate(inv); err == nil {
		t.Fatal("incomplete no-effect receipt accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := runtime.Execute(ctx, inv); err == nil {
		t.Fatal("cancel hidden by handled marker")
	}
}

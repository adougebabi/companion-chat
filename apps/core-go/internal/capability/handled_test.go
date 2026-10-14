package capability

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type preparationNoEffectCapability struct {
	executed *int
	cancel   context.CancelFunc
}

func (c preparationNoEffectCapability) Definition() CapabilityDefinition {
	return CapabilityDefinition{Name: "test.preparation.handled", Version: "v1", Description: "No effect preparation regression", Type: CapabilityTypeQuery, InputSchema: map[string]any{"type": "object", "additionalProperties": false}, OutputSchema: map[string]any{"type": "object"}, Surfaces: []CapabilitySurface{CapabilitySurfaceConversation}, SideEffectClass: "read_only", SuccessBoundary: "query_result_available", ConcurrencyClass: "parallel", FailurePolicy: FailurePolicyOptionalInternal}
}
func (c preparationNoEffectCapability) RequiredContext() []ContextSlot { return nil }
func (c preparationNoEffectCapability) Prepare(_ context.Context, inv CapabilityInvocation, _ CapabilityContext) (CapabilityInvocation, error) {
	if c.cancel != nil {
		c.cancel()
	}
	return WithHandledPreparation(inv, "no_change", "no_change", nil)
}
func (c preparationNoEffectCapability) Execute(_ context.Context, inv CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	*c.executed++
	return FailedCapabilityResult(inv, "should_not_execute", false), errors.New("should not execute")
}
func TestExecuteHonorsFreshPreparerHandledAndCancellation(t *testing.T) {
	for _, cancelDuringPrepare := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0
		c := preparationNoEffectCapability{executed: &calls}
		if cancelDuringPrepare {
			c.cancel = cancel
		}
		registry, err := NewCapabilityRegistry(c)
		if err != nil {
			t.Fatal(err)
		}
		runtime, err := NewCapabilityRuntime(registry, NewStaticContextResolver(nil))
		if err != nil {
			t.Fatal(err)
		}
		result, err := runtime.Execute(ctx, CapabilityInvocation{CallID: "call", CapabilityName: c.Definition().Name, Arguments: json.RawMessage(`{}`), Metadata: InvocationMetadata{Source: "direct"}})
		cancel()
		if calls != 0 {
			t.Fatal("executed after handled Prepare")
		}
		if cancelDuringPrepare {
			if !errors.Is(err, context.Canceled) {
				t.Fatal("cancel hidden", result, err)
			}
		} else {
			if err != nil || result.Status != "handled" {
				t.Fatal(result, err)
			}
		}
	}
}

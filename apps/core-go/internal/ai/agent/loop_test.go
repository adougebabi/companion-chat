package agent

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
)

func TestTracePublishesFailureWithItsResultConcurrently(t *testing.T) {
	const count = 32
	trace := &ADKCapabilityTrace{}
	var group sync.WaitGroup
	for index := 0; index < count; index++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			id := fmt.Sprintf("call-%d", index)
			failure := fmt.Errorf("failure-%d", index)
			trace.AppendResultWithFailure(capability.CapabilityResult{CallID: id, Status: "failed", Retryable: true}, failure)
			result, found := trace.FindResult(id)
			if !found || result.CallID != id || !errors.Is(trace.ResultFailure(id), failure) {
				t.Errorf("result or its failure lost for %s", id)
			}
		}(index)
	}
	group.Wait()
	_, results := trace.Snapshot()
	if len(results) != count {
		t.Fatalf("recorded %d results, want %d", len(results), count)
	}
}

type dummyInvoker struct{}

func (d dummyInvoker) Execute(ctx context.Context, capabilityName string, argumentsJSON string) (string, error) {
	return "ok", nil
}

func (d dummyInvoker) ExecuteWithID(ctx context.Context, callID, capabilityName string, argumentsJSON string) (string, error) {
	return `{"result":"ok"}`, nil
}

func TestNewADKCapabilityTools(t *testing.T) {
	defs := []capability.CapabilityDefinition{
		{
			Name:        "test.tool",
			Description: "A test tool",
		},
	}

	invoker := dummyInvoker{}
	tools, err := NewADKCapabilityTools(defs, invoker)
	if err != nil {
		t.Fatalf("unexpected error creating tools: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	info, err := tools[0].Info(context.Background())
	if err != nil {
		t.Fatalf("unexpected info error: %v", err)
	}
	if info.Name != "test.tool" {
		t.Fatalf("expected test.tool, got %s", info.Name)
	}
}

func TestRunADKLoopRequiresModel(t *testing.T) {
	_, err := RunADKLoop(context.Background(), ADKLoopConfig{
		Name:        "agent",
		Description: "agent desc",
	}, []*schema.Message{schema.UserMessage("hi")})
	if err == nil {
		t.Fatal("expected error for missing model")
	}
}

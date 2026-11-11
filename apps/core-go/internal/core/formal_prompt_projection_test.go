package core

import (
	"context"
	"testing"
)

func TestFormalPromptProjectionIncludesInitializationWithoutChangingMessages(t *testing.T) {
	messages := (&PromptComposer{}).ComposeTaskMessages("initialization", initializationAnalysisMessages("一个有自己日常习惯的人"))
	before := jsonString(messages)
	ctx, err := projectFormalPromptBoundary(context.Background(), ADKStructuredTaskInput{
		AgentID: FormalAgentInitialization, Role: "initialization", SchemaName: "initialization_response",
		Prompt: PromptAssemblyResult{Messages: messages},
	})
	if err != nil {
		t.Fatal(err)
	}
	if jsonString(messages) != before {
		t.Fatal("read-only task projection changed raw initialization messages")
	}
	projection := mapValue(providerPromptDiagnostics(ctx)["context_projection"])
	if stringValue(projection["agent_id"]) != string(FormalAgentInitialization) || projection["task_variant"] != "initialization_response" {
		t.Fatalf("agent and variant are missing from the request trace: %#v", projection)
	}
	mapping := arrayValue(projection["source_map"])
	if len(mapping) != len(messages) || stringValue(mapValue(mapping[0])["source"]) != "trusted_task_configuration" || stringValue(mapValue(mapping[len(mapping)-1])["source"]) != "owner_description" || stringValue(projection["token_estimate_method"]) != "rune_heuristic" {
		t.Fatalf("initialization source mapping=%#v", projection)
	}
	if mapValue(mapping[len(mapping)-1])["source_status"] != "inferred_from_message_shape" || projection["slot_trace_status"] != "not_declared_by_caller" {
		t.Fatal("legacy message inference was reported as declared provenance")
	}
}

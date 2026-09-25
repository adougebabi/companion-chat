package core

import (
	"context"
	"testing"
)

func TestFormalPromptProjectionIncludesInitializationWithoutChangingMessages(t *testing.T) {
	messages := (&PromptComposer{}).ComposeTaskMessages("initialization", initializationAnalysisMessages("一个有自己日常习惯的人"))
	before := jsonString(messages)
	ctx, err := projectFormalPromptBoundary(context.Background(), ADKStructuredTaskInput{
		Role: "initialization", SchemaName: "initialization_response",
		Prompt: PromptAssemblyResult{Messages: messages},
	})
	if err != nil {
		t.Fatal(err)
	}
	if jsonString(messages) != before {
		t.Fatal("read-only task projection changed raw initialization messages")
	}
	projection := mapValue(providerPromptDiagnostics(ctx)["context_projection"])
	mapping := arrayValue(projection["source_map"])
	if len(mapping) != len(messages) || stringValue(mapValue(mapping[0])["source"]) != "trusted_task_configuration" || stringValue(mapValue(mapping[len(mapping)-1])["source"]) != "owner_description" || stringValue(projection["token_estimate_method"]) != "rune_heuristic" {
		t.Fatalf("initialization source mapping=%#v", projection)
	}
}

package core

import (
	"context"
	"strings"
	"testing"
)

func TestConversationRuntimeRequiresCapabilityContextForModelCatalog(t *testing.T) {
	registry := mustCapabilityRegistry(builtinCapabilities(nil)...)
	app := &App{Capabilities: registry, Provider: &ProviderClient{}}
	definition, ok := registry.Definition("memory.recall")
	if !ok {
		t.Fatal("memory.recall is not registered")
	}

	_, err := newConversationRuntime(app).RunMain(context.Background(), ConversationMainInput{
		Role:        "cognitive_assessment",
		Messages:    []map[string]any{{"role": "user", "content": "hello"}},
		Definitions: []CapabilityDefinition{definition},
		SchemaName:  "conversation_turn_response",
		Schema:      map[string]any{"type": "object"},
	})
	if err == nil || err.Error() != "conversation_runtime_capability_context_required" {
		t.Fatalf("err = %v", err)
	}
}

func TestConversationRuntimeRejectsCallerSchemaDivergenceBeforeProviderIO(t *testing.T) {
	registry := mustCapabilityRegistry(builtinCapabilities(nil)...)
	app := &App{Capabilities: registry, Provider: &ProviderClient{}}
	definition, ok := registry.Definition("memory.recall")
	if !ok {
		t.Fatal("memory.recall is not registered")
	}
	definition.Description = "caller supplied schema"

	_, err := newConversationRuntime(app).RunMain(context.Background(), ConversationMainInput{
		Role:        "cognitive_assessment",
		Messages:    []map[string]any{{"role": "user", "content": "hello"}},
		Definitions: []CapabilityDefinition{definition},
		SchemaName:  "conversation_turn_response",
		Schema:      map[string]any{"type": "object"},
		Capability: &ConversationCapabilityContext{
			FluctlightID: "fl-1", ConversationID: "conv-1", SourceFactID: "fact-1", ActionID: "action-1",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "conversation_runtime_capability_definition_mismatch") {
		t.Fatalf("err = %v", err)
	}
}

func TestConversationRuntimeRejectsDefinitionOutsideConversationSurface(t *testing.T) {
	registry := mustCapabilityRegistry(builtinCapabilities(nil)...)
	app := &App{Capabilities: registry, Provider: &ProviderClient{}}
	definition, ok := registry.Definition("moment.publish")
	if !ok {
		t.Fatal("moment.publish is not registered")
	}

	_, err := newConversationRuntime(app).RunMain(context.Background(), ConversationMainInput{
		Role:        "cognitive_assessment",
		Messages:    []map[string]any{{"role": "user", "content": "hello"}},
		Definitions: []CapabilityDefinition{definition},
		SchemaName:  "conversation_turn_response",
		Schema:      map[string]any{"type": "object"},
		Capability: &ConversationCapabilityContext{
			FluctlightID: "fl-1", ConversationID: "conv-1", SourceFactID: "fact-1", ActionID: "action-1",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "conversation_runtime_capability_definition_surface_forbidden") {
		t.Fatalf("err = %v", err)
	}
}

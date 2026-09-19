package core

import (
	"context"
	"testing"
)

func TestPersonaPolicyCapabilitiesStayOutOfConversationCatalog(t *testing.T) {
	registry := mustCapabilityRegistry(builtinCapabilities(nil)...)
	for _, name := range []string{personaTakeoverCapabilityName, personaSwitchCapabilityName} {
		definition, ok := registry.Definition(name)
		if !ok {
			t.Fatalf("missing policy capability %q", name)
		}
		if definition.Type != CapabilityTypeInternal || definition.SideEffectClass != "policy" {
			t.Fatalf("%s is not an internal policy capability: %#v", name, definition)
		}
		for _, surface := range []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceReflection} {
			for _, candidate := range registry.Catalog(surface) {
				if candidate.Name == name {
					t.Fatalf("policy capability %q leaked into %s catalog", name, surface)
				}
			}
		}
		for _, candidate := range registry.Catalog(CapabilitySurfaceAutonomy) {
			if candidate.Name == name {
				t.Fatalf("policy capability %q leaked into autonomy model catalog", name)
			}
		}
	}
}

func TestPersonaPolicyInvocationUsesStablePolicyIdentity(t *testing.T) {
	registry := mustCapabilityRegistry(builtinCapabilities(nil)...)
	resolver := NewStaticContextResolver(nil)
	runtime, err := NewCapabilityRuntime(registry, resolver)
	if err != nil {
		t.Fatal(err)
	}
	app := &App{Capabilities: registry, ContextResolver: resolver, Runtime: runtime}
	args := map[string]any{"decision": "switch", "target_profile_id": "profile-b"}
	firstInvocation, firstResult, err := app.executePersonaPolicyAction(context.Background(), personaSwitchCapabilityName, "fl-1", "conv-1", "fact-1", "action-1", args)
	if err != nil {
		t.Fatal(err)
	}
	secondInvocation, secondResult, err := app.executePersonaPolicyAction(context.Background(), personaSwitchCapabilityName, "fl-1", "conv-1", "fact-1", "action-1", args)
	if err != nil {
		t.Fatal(err)
	}
	if firstInvocation.CallID == "" || firstInvocation.CallID != secondInvocation.CallID {
		t.Fatalf("policy call identity is not stable: %q/%q", firstInvocation.CallID, secondInvocation.CallID)
	}
	if firstInvocation.Metadata.Source != "policy" || firstInvocation.Metadata.Surface != CapabilitySurfaceAutonomy {
		t.Fatalf("policy invocation provenance = %#v", firstInvocation.Metadata)
	}
	if firstResult.CallID != firstInvocation.CallID || firstResult.CapabilityName != personaSwitchCapabilityName || firstResult.Status != "deferred" || firstResult.Retryable {
		t.Fatalf("first policy result = %#v", firstResult)
	}
	if secondResult.CallID != secondInvocation.CallID || secondResult.CapabilityName != personaSwitchCapabilityName || secondResult.Status != "deferred" || secondResult.Retryable {
		t.Fatalf("second policy result = %#v", secondResult)
	}
	// A partially assembled App may have the runtime injected before the
	// convenience registry field. The policy bridge must use the runtime's
	// registry instead of dereferencing a nil App.Capabilities.
	partial := &App{Runtime: runtime}
	if _, _, err := partial.executePersonaPolicyAction(context.Background(), personaSwitchCapabilityName, "fl-1", "conv-1", "fact-1", "action-1", args); err != nil {
		t.Fatalf("runtime-only policy app failed: %v", err)
	}
}

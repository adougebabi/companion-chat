package core

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestCapabilityCatalogListingKeepsCompleteDirectoryWithoutSchemas(t *testing.T) {
	registry := mustCapabilityRegistry(builtinCapabilities(nil)...)
	for _, implementation := range builtinCapabilities(nil) {
		definition := implementation.Definition()
		if capabilityDirectoryPurposes[definition.Name] == "" {
			t.Fatalf("builtin lacks short directory purpose: %s", definition.Name)
		}
	}
	catalog := capabilityCatalogCapability{catalog: registry.Catalog}
	for _, surface := range capabilityUtilitySurfaces() {
		t.Run(string(surface), func(t *testing.T) {
			definitions := registry.Catalog(surface)
			for _, args := range []string{`{}`, `{"names":null}`, `{"names":[]}`} {
				inv := CapabilityInvocation{CallID: "list", CapabilityName: capabilityCatalogName, Arguments: json.RawMessage(args), Metadata: InvocationMetadata{Surface: surface}}
				result, err := catalog.Execute(context.Background(), inv, CapabilityContext{})
				if err != nil {
					t.Fatal(err)
				}
				items := arrayValue(mapValue(result.Output)["items"])
				if len(items) != len(definitions) || len(items) == 0 {
					t.Fatalf("incomplete directory: got %d want %d", len(items), len(definitions))
				}
				byName := map[string]map[string]any{}
				for _, raw := range items {
					item := mapValue(raw)
					if _, ok := item["parameters"]; ok {
						t.Fatal("unrequested schema leaked into listing")
					}
					byName[stringValue(item["name"])] = item
				}
				for _, definition := range definitions {
					if capabilityDirectoryPurposes[definition.Name] == "" {
						t.Fatalf("builtin lacks directory purpose: %s", definition.Name)
					}
					if item := byName[definition.Name]; item == nil || item["purpose"] != capabilityDirectoryPurpose(definition) {
						t.Fatalf("purpose missing/changed: %s", definition.Name)
					}
				}
				tokens := EstimatePromptTokens(result.Output)
				t.Logf("surface=%s tools=%d directory_tokens=%d", surface, len(items), tokens)
				if tokens > 4608 {
					t.Fatalf("directory alone exceeds 4608-token regression budget: %d", tokens)
				}
			}
		})
	}
}

func TestCapabilityCatalogNamedDisclosurePreservesCompleteSchemaAndUnavailable(t *testing.T) {
	definition := CapabilityDefinition{Name: "large.tool", Description: "Full schema", InputSchema: objectSchema(map[string]any{"nested": objectSchema(map[string]any{"source": stringSchema()}, []string{"source"}, false)}, []string{"nested"}, false)}
	catalog := capabilityCatalogCapability{catalog: func(CapabilitySurface) []CapabilityDefinition { return []CapabilityDefinition{definition} }}
	result, err := catalog.Execute(context.Background(), CapabilityInvocation{CallID: "named", CapabilityName: capabilityCatalogName, Arguments: json.RawMessage(`{"names":["large.tool","not.installed"]}`)}, CapabilityContext{})
	if err != nil {
		t.Fatal(err)
	}
	items := arrayValue(mapValue(result.Output)["items"])
	if len(items) != 1 || !reflect.DeepEqual(mapValue(items[0])["parameters"], definition.InputSchema) {
		t.Fatalf("named disclosure changed schema: %#v", result.Output)
	}
	missing := jsonString(mapValue(result.Output)["unavailable"])
	for _, want := range []string{"not.installed", "capability_not_installed", "tool_run_context_required"} {
		if !strings.Contains(missing, want) {
			t.Fatalf("disclosure/admission status lost %s: %s", want, missing)
		}
	}
}

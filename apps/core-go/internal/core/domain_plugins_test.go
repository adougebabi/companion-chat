package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	aiagent "github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/agent"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/decision"
	capabilitycontract "github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
)

func pluginTestContext(image, moment bool) context.Context {
	return context.WithValue(context.Background(), domainPluginScopeKey{}, domainPluginSettings{Image: image, Moment: moment})
}

func TestDomainPluginConfigurationAndRegistration(t *testing.T) {
	if defaultDomainPluginSettings() != (domainPluginSettings{Image: true, Moment: true}) {
		t.Fatal("both existing domains must remain enabled for installations without settings")
	}
	for _, raw := range []string{`null`, `{}`, `{"image":false}`, `{"image":false,"moment":null}`, `{"image":false,"moment":"true"}`, `{"image":true,"moment":true,"extra":1}`, `{"image":true,"moment":true} {}`} {
		if _, err := decodeDomainPluginSettings([]byte(raw)); err == nil {
			t.Errorf("accepted malformed settings %s", raw)
		}
	}
	settings, err := decodeDomainPluginSettings([]byte(`{"image":false,"moment":true}`))
	if err != nil || settings.Image || !settings.Moment {
		t.Fatalf("explicit settings changed: %#v %v", settings, err)
	}
	app := &App{}
	registry := app.capabilityRegistry()
	for _, plugin := range builtInDomainPlugins {
		registered := map[string]bool{}
		for _, capability := range plugin.Register(app, NewToolPublicationService(app)) {
			name := capability.Definition().Name
			registered[name] = true
			if _, ok := registry.Definition(name); !ok || !containsString(plugin.Capabilities, name) {
				t.Fatalf("%s registration or manifest missing %s", plugin.ID, name)
			}
		}
		for _, name := range plugin.Capabilities {
			if name != replyImageRequestName && !registered[name] {
				t.Fatalf("%s declares an unregistered capability %s", plugin.ID, name)
			}
		}
		for _, name := range plugin.NewWorkCapabilities {
			if !containsString(plugin.Capabilities, name) {
				t.Fatalf("%s gates undeclared capability %s", plugin.ID, name)
			}
		}
	}
	if got := domainPluginCatalog(settings, nil); len(got) != 2 || got[0]["health"] != "unconfigured" || got[0]["error_code"] != "media_not_configured" || got[1]["health"] != "not_checked" {
		t.Fatalf("plugin configuration status is incorrect: %#v", got)
	}
}

func TestDomainPluginCatalogDistinguishesConfigurationFromHealth(t *testing.T) {
	settings := domainPluginSettings{Image: true, Moment: true}
	for _, test := range []struct {
		name, health, errorCode string
		config                  map[string]any
	}{
		{"missing", "unconfigured", "media_not_configured", nil},
		{"invalid", "configuration_invalid", "media_configuration_invalid", map[string]any{"media.comfyui": map[string]any{"baseUrl": "http://comfy.invalid"}}},
		{"configured", "not_checked", "", map[string]any{"media.comfyui": map[string]any{"baseUrl": "http://comfy.invalid", "workflow": map[string]any{"prompt": "fixture"}}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			catalog := domainPluginCatalog(settings, test.config)
			image, moment := catalog[0], catalog[1]
			if image["health"] != test.health || image["error_code"] != test.errorCode || !reflect.DeepEqual(image["required_dependencies"], []string{"comfyui"}) {
				t.Fatalf("Image status = %#v", image)
			}
			if moment["health"] != "not_checked" || !reflect.DeepEqual(moment["optional_dependencies"], []string{"image"}) || len(moment["required_dependencies"].([]string)) != 0 {
				t.Fatalf("Moment incorrectly depends on image: %#v", moment)
			}
		})
	}
}

func TestDomainPluginDiscoveryAndExecutionAgree(t *testing.T) {
	app := &App{}
	registry := app.capabilityRegistry()
	definitions := registry.Catalog(CapabilitySurfaceConversation)
	config := decision.DefaultConfig()
	selection := &kevRunSelection{service: &decision.Service{Settings: &kevFixedSettings{config: config}}, original: definitions, loaded: map[string]bool{}, version: 1}
	ctx := context.WithValue(pluginTestContext(false, true), kevRunSelectionKey{}, selection)
	ctx = withCapabilityRunRegistry(ctx, registry)
	inv := CapabilityInvocation{CallID: "directory", CapabilityName: capabilityCatalogName, Arguments: json.RawMessage(`{"names":[]}`), Metadata: InvocationMetadata{Surface: CapabilitySurfaceConversation}}
	catalog := capabilityCatalogCapability{catalog: registry.Catalog, lookup: registry.Definition}
	result, err := catalog.Execute(ctx, inv, CapabilityContext{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range result.Output.(map[string]any)["items"].([]map[string]any) {
		if item["name"] == "media.image.generate" {
			found = item["status"] == "disabled"
		}
	}
	if !found {
		t.Fatal("directory did not disclose the installed but disabled image capability")
	}
	inv.Arguments = json.RawMessage(`{"names":["media.image.generate","missing.plugin.tool"]}`)
	for _, execute := range []func(context.Context, CapabilityInvocation, CapabilityContext) (CapabilityResult, error){catalog.Execute, (capabilityDiscoverCapability{catalog: registry.Catalog, lookup: registry.Definition}).Execute} {
		result, err = execute(ctx, inv, CapabilityContext{})
		if err != nil {
			t.Fatal(err)
		}
		unavailable := result.Output.(map[string]any)["unavailable"].([]toolDisclosureAvailability)
		codes := map[string]string{}
		for _, item := range unavailable {
			codes[item.Name] = item.ErrorCode
		}
		if codes["media.image.generate"] != "domain_plugin_disabled" || codes["missing.plugin.tool"] != "capability_not_installed" {
			t.Fatalf("disabled and missing collapsed: %#v", unavailable)
		}
	}
	if got := kevRejectNativeTool(ctx, "media.image.generate", nil); got != "" {
		t.Fatalf("installed call cannot reach durable receipt replay: %q", got)
	}
	if got := kevRejectNativeTool(ctx, "missing.plugin.tool", nil); got != "capability_installation_unknown" {
		t.Fatalf("missing call reported %q", got)
	}
	if got := kevRejectNativeTool(ctx, "moment.publish", nil); got != "" {
		t.Fatalf("independent Moment was disabled with Image: %q", got)
	}
	tool := &replyImageRequestTool{}
	_, err = tool.Execute(ctx, CapabilityInvocation{CallID: "private", CapabilityName: replyImageRequestName, Arguments: json.RawMessage(`{"intent":"photo"}`)}, CapabilityContext{})
	var capabilityErr *CapabilityError
	if !errors.As(err, &capabilityErr) || capabilityErr.Code != "domain_plugin_disabled" || len(tool.requests) != 0 {
		t.Fatalf("private Writer bypassed admission: %#v %v", tool.requests, err)
	}
}

func TestDomainPluginDisabledToolsOnPhysicalRequests(t *testing.T) {
	for _, settings := range []domainPluginSettings{{Image: false, Moment: true}, {Image: true, Moment: false}, {Image: false, Moment: false}} {
		for _, streaming := range []bool{false, true} {
			t.Run(fmt.Sprintf("image=%t/moment=%t/stream=%t", settings.Image, settings.Moment, streaming), func(t *testing.T) {
				config := decision.DefaultConfig()
				app := &App{Kev: &decision.Service{Settings: &kevFixedSettings{config}, Store: &kevMemoryStore{}}}
				ctx := pluginTestContext(settings.Image, settings.Moment)
				definitions := app.capabilityRegistry().Catalog(CapabilitySurfaceConversation)
				ctx, executors, err := app.prepareKevTools(ctx, ADKStructuredTaskInput{AgentID: FormalAgentConversationCognition, Definitions: definitions, Capability: &ADKCapabilityRequest{Surface: CapabilitySurfaceConversation, Projection: ContextProjection{FluctlightID: "self"}}})
				if err != nil {
					t.Fatal(err)
				}
				var seen []string
				provider := &ProviderClient{HTTP: &http.Client{Transport: providerErrorTestTransport(func(r *http.Request) (*http.Response, error) {
					var payload map[string]any
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						return nil, err
					}
					for _, raw := range arrayValue(payload["tools"]) {
						seen = append(seen, stringValue(mapValue(mapValue(raw)["function"])["name"]))
					}
					key := "message"
					if streaming {
						key = "delta"
					}
					encoded := jsonString(map[string]any{"choices": []any{map[string]any{"index": 0, key: map[string]any{"role": "assistant", "content": "controlled result"}, "finish_reason": "stop"}}})
					if streaming {
						encoded = "data: " + encoded + "\n\ndata: [DONE]\n\n"
					}
					response := embeddingHTTPResponse(r, 200, encoded)
					if streaming {
						response.Header.Set("Content-Type", "text/event-stream")
					}
					return response, nil
				})}}
				assignment := providerAssignment{Role: "generic_llm", BaseURL: "http://controlled.invalid", ModelID: "controlled-model", Timeout: time.Second, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1}
				inner, err := NewEinoModelFactory(provider.HTTP).NewChatModel(ctx, EinoModelConfig{BaseURL: assignment.BaseURL, Model: assignment.ModelID, HTTPClient: provider.HTTP})
				if err != nil {
					t.Fatal(err)
				}
				m := &queuedToolCallingChatModel{inner: inner, provider: provider, role: "cognitive_assessment", assignment: assignment, definitions: executors}
				infos, err := aiagent.CapabilityToolInfos(executors)
				if err != nil {
					t.Fatal(err)
				}
				if streaming {
					reader, err := m.Stream(ctx, []*schema.Message{schema.UserMessage("ordinary conversation")}, model.WithTools(infos))
					if err != nil {
						t.Fatal(err)
					}
					defer reader.Close()
					for {
						_, err := reader.Recv()
						if err == io.EOF {
							break
						}
						if err != nil {
							t.Fatal(err)
						}
					}
				} else if _, err := m.Generate(ctx, []*schema.Message{schema.UserMessage("ordinary conversation")}, model.WithTools(infos)); err != nil {
					t.Fatal(err)
				}
				want := []string{capabilitycontract.CapabilityWireName(capabilityCatalogName), capabilitycontract.CapabilityWireName(conversationReplyCapabilityName)}
				if settings.Moment {
					want = append(want, capabilitycontract.CapabilityWireName("moment.publish"))
				}
				sort.Strings(seen)
				sort.Strings(want)
				if !reflect.DeepEqual(seen, want) {
					t.Fatalf("physical tools=%v want=%v", seen, want)
				}
			})
		}
	}
}

func TestDomainPluginWriterAndVisualReplayBoundaries(t *testing.T) {
	input := replyContextFixture()
	input.ExpressionVersion = factOnlyReplyV8PromptVersion
	input.CapabilityFacts["image_generation_registered"] = true
	calls := 0
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		calls++
		for _, raw := range arrayValue(payload["tools"]) {
			name := stringValue(mapValue(mapValue(raw)["function"])["name"])
			if capabilitycontract.CapabilityNamesMatch(name, replyImageRequestName) {
				t.Error("disabled image handoff reached Writer transport")
			}
		}
		var messages strings.Builder
		for _, raw := range arrayValue(payload["messages"]) {
			messages.WriteString(stringValue(mapValue(raw)["content"]))
		}
		if !strings.Contains(messages.String(), `"image_generation_registered":false`) {
			t.Error("Writer did not receive the disabled capability fact")
		}
		return fakeProviderResult{Text: "普通文字回复。"}
	})
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
	_, err := app.RunConversationReplyTask(pluginTestContext(false, true), ConversationReplyTaskInput{Context: input, Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "controlled-reply", Timeout: time.Second}, Concurrency: 1})
	if err != nil || calls != 1 {
		t.Fatalf("ordinary Writer failed: calls=%d err=%v", calls, err)
	}
	ctx := context.WithValue(pluginTestContext(false, true), domainPluginVisualReplayKey{}, true)
	if !domainCapabilityEnabled(ctx, visualIdentityGenerateCandidateCapabilityName) {
		t.Fatal("accepted candidate cannot be replayed")
	}
	if err := requireDomainPluginEnabledTx(ctx, nil, "image"); err == nil {
		t.Fatal("replay authority permitted creation of a new image intent")
	}
}

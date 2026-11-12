package core

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/decision"
)

func TestCapabilityDiscoveryDistinguishesInstalledButUnauthorized(t *testing.T) {
	app := &App{}
	registry := app.capabilityRegistry()
	definitions := registry.Catalog(CapabilitySurfaceConversation)
	authorized := make([]CapabilityDefinition, 0, len(definitions))
	for _, definition := range definitions {
		if definition.Name != wardrobeWearCapabilityName {
			authorized = append(authorized, definition)
		}
	}
	for _, test := range []struct {
		name    string
		execute func(context.Context, CapabilityInvocation, CapabilityContext) (CapabilityResult, error)
	}{
		{"catalog", (capabilityCatalogCapability{catalog: registry.Catalog, lookup: registry.Definition}).Execute},
		{"discover", (capabilityDiscoverCapability{catalog: registry.Catalog, lookup: registry.Definition}).Execute},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := decision.DefaultConfig()
			config.Enabled = false
			selection := &kevRunSelection{service: &decision.Service{Settings: &kevFixedSettings{config: config}}, original: authorized, loaded: map[string]bool{}, version: 1}
			ctx := withCapabilityRunRegistry(pluginTestContext(false, true), registry)
			ctx = context.WithValue(ctx, kevRunSelectionKey{}, selection)
			invocation := CapabilityInvocation{CallID: "catalog-authority", CapabilityName: capabilityCatalogName,
				Arguments: json.RawMessage(`{"names":["goal_planner.query","wardrobe.wear","media.image.generate","memory.recall","not.installed"]}`),
				Metadata:  InvocationMetadata{Surface: CapabilitySurfaceConversation}}
			result, err := test.execute(ctx, invocation, CapabilityContext{})
			if err != nil {
				t.Fatal(err)
			}
			output := mapValue(result.Output)
			unavailable, ok := output["unavailable"].([]toolDisclosureAvailability)
			if !ok {
				t.Fatalf("unavailable results missing: %#v", output)
			}
			codes := make(map[string]string, len(unavailable))
			for _, item := range unavailable {
				codes[item.Name] = item.ErrorCode
			}
			for name, want := range map[string]string{
				"goal_planner.query":   "capability_not_authorized",
				"wardrobe.wear":        "capability_not_authorized",
				"media.image.generate": "domain_plugin_disabled",
				"not.installed":        "capability_not_installed",
			} {
				if codes[name] != want {
					t.Errorf("%s classified as %q, want %q", name, codes[name], want)
				}
			}
			if !selection.loaded["memory.recall"] || selection.loaded["goal_planner.query"] || selection.loaded["wardrobe.wear"] || selection.loaded["media.image.generate"] {
				t.Fatalf("discovery widened the tool window: %#v", selection.loaded)
			}
			for _, raw := range arrayValue(output["items"]) {
				item := mapValue(raw)
				if name := stringValue(item["name"]); name == "goal_planner.query" || name == "wardrobe.wear" || name == "media.image.generate" {
					t.Errorf("unauthorized or disabled schema disclosed: %s", name)
				}
			}
		})
	}
}

func TestStandaloneCapabilityCatalogUsesInstalledRegistry(t *testing.T) {
	app := &App{}
	registry := app.capabilityRegistry()
	app.Capabilities = registry
	for _, name := range []string{capabilityCatalogName, capabilityDiscoverName} {
		t.Run(name, func(t *testing.T) {
			implementation, ok := registry.LookupCapability(name)
			if !ok {
				t.Fatalf("missing builtin %s", name)
			}
			inv := CapabilityInvocation{CallID: "standalone-catalog", CapabilityName: name,
				Arguments: jsonBytes(map[string]any{"names": []string{capabilityCatalogName, capabilityDiscoverName, "goal_planner.query", "not.installed"}}), Metadata: InvocationMetadata{Surface: CapabilitySurfaceConversation}}
			result, err := implementation.Execute(pluginTestContext(true, true), inv, CapabilityContext{})
			if err != nil {
				t.Fatal(err)
			}
			unavailable := mapValue(result.Output)["unavailable"].([]toolDisclosureAvailability)
			codes := map[string]string{}
			for _, item := range unavailable {
				codes[item.Name] = item.ErrorCode
			}
			if codes["goal_planner.query"] != "capability_not_authorized" || codes["not.installed"] != "capability_not_installed" || codes[capabilityCatalogName] != "tool_run_context_required" || codes[capabilityDiscoverName] != "tool_run_context_required" {
				t.Fatalf("standalone registry classification = %#v", codes)
			}
		})
	}
}

func TestCapabilityCatalogUsesCompleteRegistryWhenRunRegistryIsNarrow(t *testing.T) {
	app := &App{}
	global := app.capabilityRegistry()
	app.Capabilities = global
	narrow, err := app.ownerLifeRegistry("owner", "fluctlight", "media_generate", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := narrow.Definition(wardrobeWearCapabilityName); ok {
		t.Fatal("fixture unexpectedly included the unauthorized tool")
	}
	implementation, ok := narrow.LookupCapability(capabilityCatalogName)
	if !ok {
		t.Fatal("narrow registry lacks catalog")
	}
	config := decision.DefaultConfig()
	config.Enabled = false
	selection := &kevRunSelection{service: &decision.Service{Settings: &kevFixedSettings{config: config}}, original: narrow.Catalog(CapabilitySurfaceNativeCognition), loaded: map[string]bool{}, version: 1}
	ctx := withCapabilityRunRegistry(pluginTestContext(true, true), narrow)
	ctx = context.WithValue(ctx, kevRunSelectionKey{}, selection)
	inv := CapabilityInvocation{CallID: "narrow-catalog", CapabilityName: capabilityCatalogName,
		Arguments: json.RawMessage(`{"names":["wardrobe.wear","not.installed"]}`), Metadata: InvocationMetadata{Surface: CapabilitySurfaceNativeCognition}}
	result, err := implementation.Execute(ctx, inv, CapabilityContext{})
	if err != nil {
		t.Fatal(err)
	}
	items := mapValue(result.Output)["unavailable"].([]toolDisclosureAvailability)
	codes := map[string]string{}
	for _, item := range items {
		codes[item.Name] = item.ErrorCode
	}
	if codes["wardrobe.wear"] != "capability_not_authorized" || codes["not.installed"] != "capability_not_installed" || selection.loaded["wardrobe.wear"] {
		t.Fatalf("narrow run misclassified or loaded a tool: %#v / %#v", codes, selection.loaded)
	}
}

func TestKevNativeRejectionDistinguishesUnauthorizedFromMissing(t *testing.T) {
	registry := (&App{}).capabilityRegistry()
	selection := &kevRunSelection{original: []CapabilityDefinition{{Name: capabilityCatalogName}}, loaded: map[string]bool{}}
	ctx := withCapabilityRunRegistry(context.Background(), registry)
	ctx = context.WithValue(ctx, kevRunSelectionKey{}, selection)
	if got := kevRejectNativeTool(ctx, "persona.switch", nil); got != "capability_not_authorized" {
		t.Fatalf("installed but unauthorized native Tool = %q", got)
	}
	if got := kevRejectNativeTool(ctx, "not.installed", nil); got != "capability_installation_unknown" {
		t.Fatalf("unverified missing Tool = %q", got)
	}
}

func TestNativeInvokerRejectsInstalledToolOutsideRunScope(t *testing.T) {
	app := &App{}
	registry := app.capabilityRegistry()
	app.Capabilities = registry
	selection := &kevRunSelection{original: []CapabilityDefinition{{Name: capabilityCatalogName}}, loaded: map[string]bool{}}
	ctx := withCapabilityRunRegistry(context.Background(), registry)
	ctx = context.WithValue(ctx, kevRunSelectionKey{}, selection)
	trace := &ADKCapabilityTrace{}
	trace.RecordModelToolCalls("provider-test", 1, []schema.ToolCall{{ID: "unauthorized-call", Function: schema.FunctionCall{Name: "moment.publish"}}})
	invoker := newAppADKCapabilityInvokerForContext(ctx, app, ADKCapabilityRequest{AuthorizationActorID: "owner", FluctlightID: "self", OperationID: "run", Surface: CapabilitySurfaceConversation}, trace).(*appADKCapabilityInvoker)
	result, err := invoker.ExecuteWithID(ctx, "unauthorized-call", "moment.publish", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	receipt, found := trace.FindResult("unauthorized-call")
	if !found || receipt.Status != "handled" || stringValue(mapValue(receipt.Output)["warning_code"]) != "capability_not_authorized" || !strings.Contains(result, "capability_not_authorized") || strings.Contains(result, "invalid_arguments") {
		t.Fatalf("native unauthorized result=%s receipt=%#v", result, receipt)
	}
}

func TestNativeInvokerReportsCrossSurfaceDenialWithoutSchemaDetails(t *testing.T) {
	app := &App{}
	registry := app.capabilityRegistry()
	app.Capabilities = registry
	planner, ok := registry.Definition(goalPlannerQuery)
	if !ok {
		t.Fatal("planner query is not installed")
	}
	selection := &kevRunSelection{original: []CapabilityDefinition{planner}, loaded: map[string]bool{}}
	ctx := withCapabilityRunRegistry(context.Background(), registry)
	ctx = context.WithValue(ctx, kevRunSelectionKey{}, selection)
	trace := &ADKCapabilityTrace{}
	trace.RecordModelToolCalls("provider-test", 1, []schema.ToolCall{{ID: "wrong-surface", Function: schema.FunctionCall{Name: goalPlannerQuery}}})
	invoker := newAppADKCapabilityInvokerForContext(ctx, app, ADKCapabilityRequest{AuthorizationActorID: "owner", FluctlightID: "self", OperationID: "run", Surface: CapabilitySurfaceConversation}, trace).(*appADKCapabilityInvoker)
	result, err := invoker.ExecuteWithID(ctx, "wrong-surface", goalPlannerQuery, `{}`)
	if err != nil {
		t.Fatal(err)
	}
	receipt, found := trace.FindResult("wrong-surface")
	if !found || receipt.Status != "handled" || stringValue(mapValue(receipt.Output)["warning_code"]) != "capability_not_authorized" || !strings.Contains(result, "capability_not_authorized") {
		t.Fatalf("cross-surface result=%s receipt=%#v", result, receipt)
	}
}

func TestNativeInvokerReportsDisabledBeforeInvalidArguments(t *testing.T) {
	app := &App{}
	registry := app.capabilityRegistry()
	app.Capabilities = registry
	image, ok := registry.Definition("media.image.generate")
	if !ok {
		t.Fatal("image capability is not installed")
	}
	selection := &kevRunSelection{original: []CapabilityDefinition{image}, loaded: map[string]bool{}}
	ctx := withCapabilityRunRegistry(pluginTestContext(false, true), registry)
	ctx = context.WithValue(ctx, kevRunSelectionKey{}, selection)
	trace := &ADKCapabilityTrace{}
	trace.RecordModelToolCalls("provider-test", 1, []schema.ToolCall{{ID: "disabled-image", Function: schema.FunctionCall{Name: "media.image.generate"}}})
	invoker := newAppADKCapabilityInvokerForContext(ctx, app, ADKCapabilityRequest{AuthorizationActorID: "owner", FluctlightID: "self", OperationID: "run", Surface: CapabilitySurfaceConversation}, trace).(*appADKCapabilityInvoker)
	result, err := invoker.ExecuteWithID(ctx, "disabled-image", "media.image.generate", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	receipt, found := trace.FindResult("disabled-image")
	if !found || receipt.Status != "handled" || stringValue(mapValue(receipt.Output)["warning_code"]) != "domain_plugin_disabled" || !strings.Contains(result, "domain_plugin_disabled") || strings.Contains(result, "invalid_arguments") {
		t.Fatalf("disabled native result=%s receipt=%#v", result, receipt)
	}
}

func TestCapabilityCatalogReportsUnknownWithoutInstallationInventory(t *testing.T) {
	catalog := capabilityCatalogCapability{catalog: func(CapabilitySurface) []CapabilityDefinition { return nil }}
	inv := CapabilityInvocation{CallID: "unknown-catalog", CapabilityName: capabilityCatalogName,
		Arguments: json.RawMessage(`{"names":["unverifiable.tool"]}`), Metadata: InvocationMetadata{Surface: CapabilitySurfaceConversation}}
	result, err := catalog.Execute(context.Background(), inv, CapabilityContext{})
	if err != nil {
		t.Fatal(err)
	}
	unavailable := mapValue(result.Output)["unavailable"].([]toolDisclosureAvailability)
	if len(unavailable) != 1 || unavailable[0].Status != "unknown" || unavailable[0].ErrorCode != "capability_installation_unknown" {
		t.Fatalf("unknown inventory was reported as fact: %#v", unavailable)
	}
}

func TestPostgresStandaloneCapabilityCatalogKeepsAuthorizationBoundaries(t *testing.T) {
	ctx, repo, app, owner, self, conversation := setupDirectPublicationToolTest(t, "catalog-authority")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES($1,$2) ON CONFLICT(key) DO UPDATE SET value_json=EXCLUDED.value_json`, domainPluginsSettingKey, `{"image":false,"moment":true}`); err != nil {
		t.Fatal(err)
	}
	request := ToolExecutionRequest{CapabilityName: capabilityCatalogName, OperationID: "catalog-authority-direct", AuthorizationActorID: owner,
		FluctlightID: self, ConversationID: conversation, Surface: CapabilitySurfaceConversation,
		Arguments: json.RawMessage(`{"names":["goal_planner.query","media.image.generate","memory.recall","not.installed"]}`)}
	receipt, err := app.ExecuteTool(ctx, request)
	if err != nil || receipt.Result.Status != "completed" {
		t.Fatalf("standalone discovery failed: %#v %v", receipt.Result, err)
	}
	output := mapValue(receipt.Result.Output)
	unavailable, ok := output["unavailable"].([]toolDisclosureAvailability)
	if !ok {
		t.Fatalf("missing discovery status: %#v", output)
	}
	codes := map[string]string{}
	for _, item := range unavailable {
		codes[item.Name] = item.ErrorCode
	}
	for name, want := range map[string]string{
		"goal_planner.query":   "capability_not_authorized",
		"media.image.generate": "domain_plugin_disabled",
		"memory.recall":        "tool_run_context_required",
		"not.installed":        "capability_not_installed",
	} {
		if codes[name] != want {
			t.Errorf("%s classified as %q, want %q", name, codes[name], want)
		}
	}
	if len(arrayValue(output["loaded"])) != 0 {
		t.Fatalf("standalone call created an Agent tool window: %#v", output["loaded"])
	}
	for _, raw := range arrayValue(output["items"]) {
		name := stringValue(mapValue(raw)["name"])
		if name == "goal_planner.query" || name == "media.image.generate" {
			t.Errorf("unauthorized or disabled schema disclosed: %s", name)
		}
	}
	const foreign = "catalog-foreign-owner"
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active')`, foreign); err != nil {
		t.Fatal(err)
	}
	request.OperationID = "catalog-authority-foreign"
	request.AuthorizationActorID = foreign
	if _, err := app.ExecuteTool(ctx, request); err == nil {
		t.Fatal("foreign owner queried another Fluctlight's capability catalog")
	}
}

func TestNativeInvokerUsesCompleteRegistryForNarrowRun(t *testing.T) {
	app := &App{}
	app.Capabilities = app.capabilityRegistry()
	narrow, err := app.ownerLifeRegistry("owner", "self", "media_generate", nil)
	if err != nil {
		t.Fatal(err)
	}
	selection := &kevRunSelection{original: narrow.Catalog(CapabilitySurfaceNativeCognition), loaded: map[string]bool{}}
	ctx := withCapabilityRunRegistry(context.Background(), narrow)
	ctx = context.WithValue(ctx, kevRunSelectionKey{}, selection)
	trace := &ADKCapabilityTrace{}
	trace.RecordModelToolCalls("provider-test", 1, []schema.ToolCall{{ID: "narrow-call", Function: schema.FunctionCall{Name: wardrobeWearCapabilityName}}})
	invoker := newAppADKCapabilityInvokerForContext(ctx, app, ADKCapabilityRequest{AuthorizationActorID: "owner", FluctlightID: "self", OperationID: "run", Surface: CapabilitySurfaceNativeCognition}, trace).(*appADKCapabilityInvoker)
	result, err := invoker.ExecuteWithID(ctx, "narrow-call", wardrobeWearCapabilityName, `{"mode":"partial","item_ids":["not-a-real-item"]}`)
	if err != nil {
		t.Fatal(err)
	}
	receipt, found := trace.FindResult("narrow-call")
	if !found || receipt.Status != "handled" || stringValue(mapValue(receipt.Output)["warning_code"]) != "capability_not_authorized" || !strings.Contains(result, "capability_not_authorized") || strings.Contains(result, "capability_not_installed") {
		t.Fatalf("narrow native result=%s receipt=%#v", result, receipt)
	}
	if mapValue(receipt.Output)["effect_applied"] != false || mapValue(receipt.Output)["mutation_committed"] != false {
		t.Fatalf("unauthorized tool was treated as an effect: %#v", receipt.Output)
	}
}

func TestFormalAgentWithoutKevRejectsToolOutsideStage(t *testing.T) {
	requests := 0
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		requests++
		if requests == 1 {
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("wrong-stage", visualIdentityCommitReviewCapabilityName, map[string]any{})}}
		}
		if !strings.Contains(jsonString(payload["messages"]), "capability_not_authorized") {
			t.Error("the next native request did not receive the denied Tool result")
		}
		return fakeProviderResult{Structured: map[string]any{"status": "waiting", "stage": "image_pending", "summary": "未执行非本阶段工具"}}
	})
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
	app.Capabilities = app.capabilityRegistry()
	definition, ok := app.Capabilities.Definition(visualIdentityGenerateCandidateCapabilityName)
	if !ok {
		t.Fatal("visual generation Tool is not installed")
	}
	ctx := withFrozenProviderAssignment(context.Background(), "cognitive_assessment", providerAssignment{Role: "cognitive_assessment", BaseURL: "http://controlled.invalid", ModelID: "fixture", Timeout: time.Second})
	result, err := app.RunFormalAgent(ctx, FormalAgentVisualIdentity, FormalAgentRunInput{
		Prompt:      PromptAssemblyResult{Messages: []map[string]any{{"role": "user", "content": "受控阶段测试"}}, ResponseFormat: visualIdentityAgentResponseSchema()},
		Definitions: []CapabilityDefinition{definition}, SchemaName: "visual_identity_agent_response",
		Capability: &ADKCapabilityRequest{AuthorizationActorID: "owner", FluctlightID: "self", OperationID: "visual-stage-test", Surface: CapabilitySurfaceVisualIdentity},
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt, found := result.Trace.FindResult("wrong-stage")
	if requests != 2 || !found || receipt.Status != "handled" || stringValue(mapValue(receipt.Output)["warning_code"]) != "capability_not_authorized" {
		t.Fatalf("stage request count=%d receipt=%#v", requests, receipt)
	}
}

func TestFormalAgentReplaysCommittedToolAfterPluginDisabled(t *testing.T) {
	ctx, repo, app, owner, self, _ := setupDirectPublicationToolTest(t, "formal-disabled-replay")
	persistConversationReplyMode(t, ctx, repo, "legacy")
	const root = "formal-disabled-replay-run"
	definitions := app.capabilityRegistry().Catalog(CapabilitySurfaceConversation)
	firstCtx, err := app.withDomainPluginScope(ctx)
	if err != nil {
		t.Fatal(err)
	}
	firstCtx = context.WithValue(firstCtx, kevRunSelectionKey{}, &kevRunSelection{original: definitions, loaded: map[string]bool{}})
	firstTrace := &ADKCapabilityTrace{}
	firstTrace.RecordModelToolCalls("first-provider", 1, []schema.ToolCall{{ID: "first-call", Function: schema.FunctionCall{Name: "moment.publish"}}})
	request := ADKCapabilityRequest{AuthorizationActorID: owner, SubjectActorID: owner, FluctlightID: self, OperationID: root,
		Surface: CapabilitySurfaceConversation, Projection: ContextProjection{OwnerActorID: owner, FluctlightID: self}}
	first := newAppADKCapabilityInvokerForContext(firstCtx, app, request, firstTrace).(*appADKCapabilityInvoker)
	if _, err := first.ExecuteWithID(firstCtx, "first-call", "moment.publish", `{"text":"同一次已提交动态"}`); err != nil {
		t.Fatal(err)
	}
	if receipt, found := firstTrace.FindResult("first-call"); !found || receipt.Status != "completed" {
		t.Fatalf("first Tool result: %#v", receipt)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES($1,$2) ON CONFLICT(key) DO UPDATE SET value_json=EXCLUDED.value_json`, domainPluginsSettingKey, `{"image":true,"moment":false}`); err != nil {
		t.Fatal(err)
	}
	requests := 0
	app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		requests++
		if requests == 1 {
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("replay-call", "moment.publish", map[string]any{"text": "同一次已提交动态"})}}
		}
		return fakeProviderResult{Structured: validToolOnlyConversationFinal()}
	})}
	ctx = withFrozenProviderAssignment(ctx, "cognitive_assessment", providerAssignment{Role: "cognitive_assessment", BaseURL: "http://controlled.invalid", ModelID: "fixture", Timeout: 10 * time.Second})
	moment, ok := app.capabilityRegistry().Definition("moment.publish")
	if !ok {
		t.Fatal("Moment Tool is not installed")
	}
	result, err := app.RunFormalAgent(ctx, FormalAgentConversationCognition, FormalAgentRunInput{
		Prompt:      PromptAssemblyResult{Messages: []map[string]any{{"role": "user", "content": "受控重放"}}, ResponseFormat: cognitiveTurnResponseSchema()},
		Definitions: []CapabilityDefinition{moment}, SchemaName: "conversation_turn_response", Capability: &request,
	})
	if err != nil {
		t.Fatal(err)
	}
	replayed, found := result.Trace.FindResult("replay-call")
	if requests != 2 || !found || replayed.Status != "completed" || !toolBoolValue(mapValue(replayed.Output)["replayed"]) {
		t.Fatalf("formal replay requests=%d receipt=%#v", requests, replayed)
	}
	var count int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.moments WHERE owner_fluctlight_id=$1 AND text='同一次已提交动态'`, self).Scan(&count); err != nil || count != 1 {
		t.Fatalf("formal replay duplicated or lost a committed Moment: count=%d err=%v", count, err)
	}
}

func TestCapabilityCatalogListsCurrentLoadedWindow(t *testing.T) {
	registry := (&App{}).capabilityRegistry()
	config := decision.DefaultConfig()
	config.Enabled = false
	selection := &kevRunSelection{service: &decision.Service{Settings: &kevFixedSettings{config: config}}, original: registry.Catalog(CapabilitySurfaceConversation), loaded: map[string]bool{}, version: 1}
	ctx := withCapabilityRunRegistry(pluginTestContext(false, true), registry)
	ctx = context.WithValue(ctx, kevRunSelectionKey{}, selection)
	if loaded, unavailable, err := selection.load(ctx, []string{"memory.recall"}); err != nil || len(loaded) != 1 || len(unavailable) != 0 {
		t.Fatalf("load selected Tool: %v %v %v", loaded, unavailable, err)
	}
	catalog := capabilityCatalogCapability{catalog: registry.Catalog, lookup: registry.Definition}
	result, err := catalog.Execute(ctx, CapabilityInvocation{CallID: "catalog-list", CapabilityName: capabilityCatalogName,
		Arguments: json.RawMessage(`{"names":[]}`), Metadata: InvocationMetadata{Surface: CapabilitySurfaceConversation}}, CapabilityContext{})
	if err != nil {
		t.Fatal(err)
	}
	output := mapValue(result.Output)
	loaded := arrayValue(output["loaded"])
	if output["loaded_state"] != "known" || output["loaded_scope"] != "run_disclosure_window_before_physical_policy" || !containsStringValue(loaded, capabilityCatalogName) || !containsStringValue(loaded, "memory.recall") || containsStringValue(loaded, "media.image.generate") {
		t.Fatalf("catalog lost current visible Tool window: %#v", loaded)
	}
	for _, raw := range arrayValue(output["items"]) {
		if _, exists := mapValue(raw)["parameters"]; exists {
			t.Fatal("listing loaded state leaked unrequested schemas")
		}
	}
	selection.service.Settings.(*kevFixedSettings).config.Enabled = true
	result, err = catalog.Execute(ctx, CapabilityInvocation{CallID: "catalog-after-settings-change", CapabilityName: capabilityCatalogName,
		Arguments: json.RawMessage(`{"names":[]}`), Metadata: InvocationMetadata{Surface: CapabilitySurfaceConversation}}, CapabilityContext{})
	if err != nil || containsStringValue(arrayValue(mapValue(result.Output)["loaded"]), "memory.recall") {
		t.Fatalf("catalog kept a revoked Tool window: %#v %v", result.Output, err)
	}
	selection.service = nil
	result, err = catalog.Execute(ctx, CapabilityInvocation{CallID: "catalog-unavailable-settings", CapabilityName: capabilityCatalogName,
		Arguments: json.RawMessage(`{"names":[]}`), Metadata: InvocationMetadata{Surface: CapabilitySurfaceConversation}}, CapabilityContext{})
	if err != nil || mapValue(result.Output)["loaded_state"] != "unknown" {
		t.Fatalf("catalog claimed a known Tool window without settings: %#v %v", result.Output, err)
	}
}

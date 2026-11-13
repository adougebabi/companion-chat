package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
)

func TestModelManagementReactHTTPPersistenceAndRuntime(t *testing.T) {
	webRoot := os.Getenv("FLUCTLIGHT_M4_WEB_ROOT")
	if webRoot == "" {
		t.Skip("FLUCTLIGHT_M4_WEB_ROOT must select the matching Web source with installed dependencies")
	}
	ctx, repo, server := m4IsolatedHTTPFixture(t)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES('m4-reflection','fluctlight','active'); INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES('m4-reflection','human-1','blank_slate','active','{"personality_system":{"active_profile_id":"default"}}','{}','{}','{}','{}','{}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_inner_states(fluctlight_id,revision,pad,mood,momentum,regulation,drives,conflicts,last_updated_at) VALUES('m4-reflection',0,'{"pleasure":0,"arousal":0,"dominance":0}','{"label":"neutral","intensity":0}','{"value":0,"trend":0}','{"stress":0,"stability":0}','[]','[]',now()); INSERT INTO public.fluctlight_affect_profiles(fluctlight_id) VALUES('m4-reflection')`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_profile_habits(fluctlight_id,profile_id,revision,habits_json,source_kind,source_ref) VALUES('m4-reflection','default',0,'[]','initialization','m4-fixture')`); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var models []string
	var chatRequests []string
	proposal := map[string]any{"schema_version": "fluctlight.reflection-proposal.v2", "summary": "Controlled management fixture", "emotional_summary": map[string]any{"dominant_patterns": []any{}, "triggers": []any{}, "recovery_patterns": []any{}, "conflicts": []any{}, "evidence_refs": []any{}}}
	for _, key := range []string{"active_memory_candidates", "memory_candidates", "relationship_observations", "goal_candidates", "intention_candidates", "affect_recalibration_candidates", "drive_candidates", "preference_candidates", "trigger_candidates", "developing_self_candidates", "personality_evolution_candidates", "behavior_policy_evolution_candidates"} {
		proposal[key] = []any{}
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/models" {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"id": "m4-primary"}, map[string]any{"id": "m4-override"}}})
			return
		}
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			http.Error(w, "invalid fixture request", 400)
			return
		}
		model, _ := request["model"].(string)
		mu.Lock()
		models = append(models, model)
		mu.Unlock()
		response := proposal
		format, _ := request["response_format"].(map[string]any)
		schema, _ := format["json_schema"].(map[string]any)
		if schema["name"] == "persona_compilation_response" {
			response = map[string]any{"portrait_text": "Synthetic fixture preserves the selected personality and its established habits."}
		}

		if schema["name"] == "expression_learning_v1" {
			response = map[string]any{"candidates": []any{map[string]any{"source_ref": "s1", "excerpt": "慢慢来呀", "situation": "讨论当前进度"}}}
		}
		tools, _ := request["tools"].([]any)
		if len(tools) > 0 {
			names := make([]any, 0, len(tools))
			for _, raw := range tools {
				tool, _ := raw.(map[string]any)
				function, _ := tool["function"].(map[string]any)
				names = append(names, function["name"])
			}
			t.Logf("M4_PROVIDER_TOOLS schema=%v names=%v", schema["name"], names)
		}
		for _, raw := range tools {
			tool, _ := raw.(map[string]any)
			function, _ := tool["function"].(map[string]any)
			if function["name"] != "conversation_reply" {
				continue
			}
			wire, _ := json.Marshal(request["messages"])
			mu.Lock()
			chatRequests = append(chatRequests, string(wire))
			callID := fmt.Sprintf("m4-expression-chat-%d", len(chatRequests))
			mu.Unlock()
			arguments, _ := json.Marshal(map[string]any{"text": "好，我知道了。", "decision": map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}})
			toolCall := map[string]any{"id": callID, "type": "function", "function": map[string]any{"name": function["name"], "arguments": string(arguments)}}
			message := map[string]any{"role": "assistant", "tool_calls": []any{toolCall}}
			choice := map[string]any{"index": 0, "finish_reason": "tool_calls", "message": message}
			_ = json.NewEncoder(w).Encode(map[string]any{"model": model, "choices": []any{choice}})
			return
		}
		content, _ := json.Marshal(response)
		if len(format) == 0 {
			content = []byte("Synthetic isolated management publication.")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "m4-controlled", "model": model, "choices": []any{map[string]any{"index": 0, "finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": string(content)}}}})
	}))
	defer provider.Close()
	app, err := core.NewApp(repo, "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", "service-key", "http://127.0.0.1:1", "us-east-1", "fixture", "fixture", "fixture", false)
	if err != nil {
		t.Fatal(err)
	}
	app.Provider.HTTP = provider.Client()
	server.app = app
	versions := func() any {
		settings, err := app.ReadSettings(ctx, "human-1")
		if err != nil {
			t.Fatal(err)
		}
		return settings["versions"]
	}
	if err := app.ConfigureProviderEndpoint(ctx, "human-1", map[string]any{"endpoint_id": "m4-local", "kind": "openai_compatible", "base_url": provider.URL + "/v1", "secret_purpose": "m4-no-secret", "expected_versions": versions(), "idempotency_key": "m4-local-endpoint"}); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"generic_llm", "conversation_reply"} {
		if err := app.ConfigureProviderRole(ctx, "human-1", map[string]any{"role": role, "endpoint_id": "m4-local", "model_id": "m4-primary", "timeout_seconds": 30, "expected_versions": versions(), "idempotency_key": "m4-default-" + role}); err != nil {
			t.Fatal(err)
		}
	}
	compiled, err := app.BackfillWorkingPersonas(ctx, "human-1", []string{"m4-reflection"}, false, true)
	if err != nil || len(compiled) != 1 || compiled[0].Status == "failed" {
		t.Fatal("isolated working persona setup", compiled, err)
	}
	run := func(expected string) {
		t.Helper()
		projection, err := app.BuildContextProjectionFor(ctx, core.ContextProjectionRequest{
			AuthorizationActorID: "human-1", SpeakerActorID: "human-1", FluctlightID: "m4-reflection",
			MemoryOperation: core.MemoryForReflection, MemoryConversationMode: core.MemoryConversationGlobalOnly,
		})
		if err != nil {
			t.Fatal("formal reflection context", err)
		}
		mu.Lock()
		before := len(models)
		mu.Unlock()
		result, err := app.RunReflectionProposalTask(ctx, core.ReflectionProposalTaskInput{
			Projection: projection,
			Evidence:   []map[string]any{{"sequence": 1, "event_type": "conversation.turn", "payload": map[string]any{"text": "Review a synthetic release plan."}}},
		})
		if err != nil {
			t.Fatal("formal reflection task", err)
		}
		encoded, err := json.Marshal(result.Completion.Structured)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := core.DecodeReflectionProposalV2(encoded); err != nil {
			t.Fatal("reflection output contract", err)
		}
		mu.Lock()
		defer mu.Unlock()
		if len(models) != before+1 || models[before] != expected {
			t.Fatal("runtime request model", models, expected)
		}
		t.Logf("M4_FORMAL reflection/reflection_proposal_v2 actual_request_model=%s contract=pass config_versions=%v", models[before], versions())
	}
	if manifest := os.Getenv("FLUCTLIGHT_M4_NATIVE_MANIFEST"); manifest != "" {
		runM4NativeManagementFixture(t, ctx, server, webRoot, manifest, run)
		return
	}
	browser := httptest.NewServer(server.Handler())
	defer browser.Close()
	run("m4-primary")
	for _, phase := range []string{"set", "clear"} {
		command := exec.CommandContext(ctx, "node", filepath.Join(webRoot, "test", "management-http-scenario.mjs"), phase)
		command.Dir = webRoot
		command.Env = append(os.Environ(), "FLUCTLIGHT_M4_HTTP_BASE="+browser.URL)
		output, err := command.CombinedOutput()
		t.Log(string(output))
		if err != nil {
			t.Fatal("React/real HTTP scenario", phase, err)
		}
		expected := "m4-override"
		if phase == "clear" {
			expected = "m4-primary"
		}
		run(expected)
	}
	t.Run("plugin settings page persistence and Tool consumption", func(t *testing.T) {
		if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversations(id,created_by_actor_id,title) VALUES('m4-conversation','human-1','Synthetic plugin fixture'); INSERT INTO public.conversation_heads(conversation_id,next_sequence) VALUES('m4-conversation',1); INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES('m4-conversation','human-1','owner','active'),('m4-conversation','m4-reflection','member','active')`); err != nil {
			t.Fatal(err)
		}
		for _, phase := range []string{"plugins-off", "moment-only"} {
			command := exec.CommandContext(ctx, "node", filepath.Join(webRoot, "test", "management-http-scenario.mjs"), phase)
			command.Dir = webRoot
			command.Env = append(os.Environ(), "FLUCTLIGHT_M4_HTTP_BASE="+browser.URL)
			output, err := command.CombinedOutput()
			t.Log(string(output))
			if err != nil {
				t.Fatal("plugin page", phase, err)
			}
			request := core.ToolExecutionRequest{CapabilityName: "moment.publish", OperationID: "m4-" + phase, AuthorizationActorID: "human-1", FluctlightID: "m4-reflection", Arguments: json.RawMessage(`{}`)}
			receipt, err := app.ExecuteTool(ctx, request)
			if phase == "plugins-off" {
				var denied *core.CapabilityError
				if !errors.As(err, &denied) || denied.Code != "domain_plugin_disabled" || receipt.Result.ErrorCode != "domain_plugin_disabled" {
					t.Fatal("disabled Moment runtime admission", receipt, err)
				}
			} else if err != nil || receipt.Result.Status != "completed" {
				t.Fatal("enabled text Moment runtime", receipt, err)
			}
			image := core.ToolExecutionRequest{CapabilityName: "media.image.generate", OperationID: "m4-image-" + phase, AuthorizationActorID: "human-1", FluctlightID: "m4-reflection", ConversationID: "m4-conversation", TargetKind: "conversation", TargetRef: "m4-conversation", Arguments: json.RawMessage(`{"intent":"Synthetic landscape","media_plan":{"primary_subject":"landscape","human_presence":"none","self_in_frame":"no","visible_scope":"unknown","capture_method":"unspecified","viewpoint":"unspecified","framing":"unspecified","camera_angle":"unspecified"}}`)}
			deniedReceipt, err := app.ExecuteTool(ctx, image)
			var denied *core.CapabilityError
			if !errors.As(err, &denied) || denied.Code != "domain_plugin_disabled" || deniedReceipt.Result.ErrorCode != "domain_plugin_disabled" {
				t.Fatal("disabled Image runtime admission", deniedReceipt, err)
			}
			run("m4-primary")
			t.Logf("M4_PLUGIN_RUNTIME phase=%s Moment=%s Image=domain_plugin_disabled other_reflection_contract=pass", phase, receipt.Result.Status)
		}
		var moments, images int
		if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.moments WHERE owner_fluctlight_id='m4-reflection'),(SELECT count(*) FROM public.media_intents WHERE owner_fluctlight_id='m4-reflection')`).Scan(&moments, &images); err != nil || moments != 1 || images != 0 {
			t.Fatal("plugin persistence", moments, images, err)
		}
	})

	t.Run("expression review page and next direct request", func(t *testing.T) {
		if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES('human-1','m4-reflection','m4-conversation')`); err != nil {
			t.Fatal(err)
		}
		if _, err := app.AcceptTurn(ctx, "human-1", "m4-conversation", map[string]any{"fluctlight_id": "m4-reflection", "text": "慢慢来呀", "turn_id": "m4-expression-source", "idempotency_key": "m4-expression-source", "attachment_refs": []any{}}); err != nil {
			t.Fatal(err)
		}
		for _, phase := range []string{"enable", "disable"} {
			command := exec.CommandContext(ctx, "node", filepath.Join(webRoot, "test", "expression-management-http-scenario.mjs"), phase)
			command.Dir = webRoot
			command.Env = append(os.Environ(), "FLUCTLIGHT_M4_HTTP_BASE="+browser.URL)
			output, err := command.CombinedOutput()
			t.Log(string(output))
			if err != nil {
				t.Fatal("expression review page", phase, err)
			}
			mu.Lock()
			before := len(chatRequests)
			mu.Unlock()
			id := "m4-expression-" + phase
			if _, err := app.HandleTurn(ctx, "human-1", "m4-conversation", map[string]any{"fluctlight_id": "m4-reflection", "text": "现在进度怎么样？", "turn_id": id, "idempotency_key": id, "attachment_refs": []any{}}); err != nil {
				t.Fatal("formal direct expression turn", phase, err)
			}
			mu.Lock()
			count := len(chatRequests)
			wire := ""
			if count > before {
				wire = chatRequests[before]
			}
			mu.Unlock()
			if count != before+1 {
				t.Fatal("unexpected main request count", count, before)
			}
			injected := strings.Contains(wire, "expression_suggestions")
			if injected != (phase == "enable") || (injected && !strings.Contains(wire, "慢慢来呀")) {
				t.Fatal("expression runtime consumption", phase, injected)
			}
			t.Logf("M4_EXPRESSION_RUNTIME phase=%s suggestions=%t formal_direct_contract=pass", phase, injected)
		}
	})

}

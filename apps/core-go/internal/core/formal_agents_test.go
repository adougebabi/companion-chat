package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestFormalConversationAgentRecallsRandomDatabaseSecretThroughRealToolBridge(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	ownerID := "formal_agent_owner_" + suffix
	fluctlightID := "formal_agent_fluctlight_" + suffix
	secret := "formal-agent-db-only-secret-" + suffix
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active',$3,'{}','{}','{}','{}','{}')`, fluctlightID, ownerID, json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	requestBodies := make([]string, 0, 2)
	requestIDs := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode Provider request: %v", err)
			return
		}
		body := jsonString(payload)
		mu.Lock()
		requestBodies = append(requestBodies, body)
		requestIDs = append(requestIDs, request.Header.Get("X-Fluctlight-Provider-Request-Id"))
		sequence := len(requestBodies)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if sequence == 1 {
			if strings.Contains(body, secret) {
				t.Error("random secret leaked into initial Agent input")
			}
			_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"","tool_calls":[{"id":"formal-recall-call","type":"function","function":{"name":"memory.recall","arguments":"{\"intent\":\"private code\"}"}}]}}]}`))
			return
		}
		if !strings.Contains(body, secret) {
			t.Error("real Tool receipt was not supplied to the next model request")
		}
		_, _ = w.Write([]byte(fmt.Sprintf(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":%q}}]}`, jsonString(map[string]any{"answer": secret}))))
	}))
	defer server.Close()

	endpointID := "formal-agent-endpoint-" + suffix
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible',$2,'formal-agent-secret','ready',now())`, endpointID, server.URL); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('cognitive_assessment',$1,'formal-agent-model','structured_output,tool_calling',4096,10,'{}')`, endpointID); err != nil {
		t.Fatal(err)
	}

	app := &App{DB: repository, Provider: &ProviderClient{DB: repository, HTTP: server.Client()}}
	app.ContextResolver = NewAppContextResolver(app)
	app.Capabilities = mustCapabilityRegistry(builtinCapabilities(app)...)
	runtime, err := NewCapabilityRuntime(app.Capabilities, app.ContextResolver)
	if err != nil {
		t.Fatal(err)
	}
	app.Runtime = runtime
	write, err := app.ExecuteTool(ctx, ToolExecutionRequest{
		CapabilityName: "memory_event", OperationID: "seed-random-secret-" + suffix,
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID, EvidenceID: "owner-command-" + suffix,
		Surface:   CapabilitySurfaceNativeCognition,
		Arguments: jsonBytes(map[string]any{"content": secret, "type": "semantic", "confidence": 1.0, "importance": 1.0}),
	})
	if err != nil || write.Result.Status != "completed" {
		t.Fatalf("seed memory receipt=%#v err=%v", write, err)
	}

	recallDefinition, ok := app.Capabilities.Definition("memory.recall")
	if !ok {
		t.Fatal("memory.recall is not registered")
	}
	responseSchema := objectSchema(map[string]any{"answer": stringSchema()}, []string{"answer"}, false)
	run, err := app.RunFormalAgent(WithProviderCorrelation(ctx, "formal-agent-secret-run-"+suffix), FormalAgentConversationCognition, FormalAgentRunInput{
		Prompt: PromptAssemblyResult{
			Messages:       []map[string]any{{"role": "system", "content": "Use the memory Tool before answering."}, {"role": "user", "content": "What is the private code?"}},
			ResponseFormat: responseSchema,
		},
		Definitions: []CapabilityDefinition{recallDefinition}, SchemaName: "conversation_turn_response",
		Capability: &ADKCapabilityRequest{
			AuthorizationActorID: ownerID, FluctlightID: fluctlightID, SourceFactID: "formal-agent-evidence-" + suffix,
			ActionID: "formal-agent-action-" + suffix, OperationID: "formal-agent-run-" + suffix,
			CorrelationID: "formal-agent-secret-run-" + suffix, Surface: CapabilitySurfaceConversation,
			Projection: ContextProjection{
				OwnerActorID: ownerID, FluctlightID: fluctlightID, SourceFactID: "formal-agent-evidence-" + suffix,
				CurrentSpeaker:       map[string]any{"actor_id": ownerID},
				MemoryRetrievalTrace: MemoryRetrievalTrace{ConversationMode: string(MemoryConversationGlobalOnly)},
				ReferenceIndex: ContextReferenceIndex{
					SchemaVersion: contextReferenceIndexVersion, FluctlightID: fluctlightID, OwnerActorID: ownerID,
					SpeakerActorID: ownerID, ByRef: map[string]ContextReference{},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if stringValue(run.Completion.Structured["answer"]) != secret {
		t.Fatalf("final Agent output did not use recalled secret: %#v", run.Completion.Structured)
	}
	if len(run.Completion.ToolCalls) != 1 || run.Trace == nil {
		t.Fatalf("formal Agent Tool trace missing: completion=%#v trace=%#v", run.Completion, run.Trace)
	}
	invocations, results := run.Trace.Snapshot()
	if len(invocations) != 1 || len(results) != 1 || results[0].Status != "completed" {
		t.Fatalf("formal Agent Tool receipt missing: invocations=%#v results=%#v", invocations, results)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(requestBodies) != 2 || requestIDs[0] == "" || requestIDs[0] == requestIDs[1] {
		t.Fatalf("physical Provider requests=%d ids=%#v", len(requestBodies), requestIDs)
	}
	if invocations[0].ProviderRequestID != requestIDs[0] || run.Completion.ToolCalls[0].ProviderRequestID != requestIDs[0] {
		t.Fatalf("ToolCall request identity=%q completion=%q first physical=%q", invocations[0].ProviderRequestID, run.Completion.ToolCalls[0].ProviderRequestID, requestIDs[0])
	}
	if invocations[0].Metadata.OperationID == "" || invocations[0].Metadata.OperationID == invocations[0].CallID {
		t.Fatalf("business operation identity was not separated from native call ID: %#v", invocations[0])
	}
}

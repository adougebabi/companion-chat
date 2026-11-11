package core

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type blockedPhysicalAttemptSupport struct {
	ProviderRuntimeSupport
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (s *blockedPhysicalAttemptSupport) RecordDiagnosticEvent(ctx context.Context, eventType, severity, fluctlightID, causationID, correlationID string, payload any) {
	if eventType == "provider.physical_attempt" {
		s.once.Do(func() { close(s.entered) })
		<-s.release
		return
	}
	s.ProviderRuntimeSupport.RecordDiagnosticEvent(ctx, eventType, severity, fluctlightID, causationID, correlationID, payload)
}

func TestPhysicalAttemptDiagnosticDoesNotBlockModel(t *testing.T) {
	ctx, repo := isolatedCoreTestRepository(t)
	seedCognitiveProviderRole(t, ctx, repo, "attempt-nonblocking-endpoint")
	support := &blockedPhysicalAttemptSupport{ProviderRuntimeSupport: newProviderRuntimeSupport(repo), entered: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(func() { close(support.release) })
	provider := &ProviderClient{DB: repo, runtime: support, HTTP: &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return embeddingHTTPResponse(request, http.StatusOK, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}]}`), nil
	})}}
	assignment, err := provider.assignment(ctx, "cognitive_assessment")
	if err != nil {
		t.Fatal(err)
	}
	call := upstreamRetryCall("http://turn.invalid", "cognitive_assessment")
	call.Assignment = assignment
	ctx = WithADKCapabilityInvoker(ctx, &adkFakeInvoker{}, &ADKCapabilityTrace{})
	done := make(chan error, 1)
	go func() {
		_, err := provider.generateWithEino(ctx, call)
		done <- err
	}()
	select {
	case <-support.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("physical diagnostic was not attempted")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("blocked diagnostic changed the model result: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("blocked diagnostic delayed the model result")
	}
}

func TestUpstreamRetriesRecordEachEffectiveModelConfiguration(t *testing.T) {
	withoutUpstreamRetryDelay(t)
	for _, streaming := range []bool{false, true} {
		name := "generate"
		if streaming {
			name = "stream"
		}
		t.Run(name, func(t *testing.T) {
			ctx, repo := isolatedCoreTestRepository(t)
			seedCognitiveProviderRole(t, ctx, repo, "attempt-configuration-endpoint")
			key := []byte("0123456789abcdef0123456789abcdef")
			const fixtureSecret = "attempt-configuration-fixture-secret"
			encrypted, err := encryptSecret(key, "turn-secret", fixtureSecret)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.setting_secrets(purpose,ciphertext,nonce) VALUES('turn-secret',$1,$2)`, encrypted.ciphertext, encrypted.nonce); err != nil {
				t.Fatal(err)
			}
			var calls atomic.Int32
			provider := &ProviderClient{DB: repo, SettingsKey: key, HTTP: &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				if calls.Add(1) == 1 {
					return embeddingHTTPResponse(request, http.StatusServiceUnavailable, `{"error":{"message":"temporary"}}`), nil
				}
				if streaming {
					return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader("data: {\"choices\":[{\"delta\":{\"role\":\"assistant\",\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")), Request: request}, nil
				}
				return embeddingHTTPResponse(request, http.StatusOK, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}]}`), nil
			})}}
			assignment, err := provider.assignment(ctx, "cognitive_assessment")
			if err != nil {
				t.Fatal(err)
			}
			call := upstreamRetryCall("http://turn.invalid", "cognitive_assessment")
			call.Assignment, call.EnableStreaming = assignment, streaming
			working, err := ResolveWorkingMemory(WorkingMemoryInput{RuntimeFacts: []PromptFragment{{Kind: PromptFragmentRuntimeFact, Required: true, Content: map[string]any{"kind": "current_state", "value": map[string]any{"scene": "书房"}}, SourceRefs: []string{"state:retry"}}}}, DefaultWorkingMemoryPolicy())
			if err != nil {
				t.Fatal(err)
			}
			assembly, err := AssemblePromptContext(PromptAssemblyInput{Role: "cognitive_assessment", OperationRules: []string{"test task"}, WorkingMemory: working, CurrentInput: "本次输入", Policy: DefaultPromptBudgetPolicy(0)})
			if err != nil {
				t.Fatal(err)
			}
			assembly = formalPromptAssembly(FormalAgentConversationCognition, assembly)
			ctx, err = projectFormalPromptBoundary(ctx, ADKStructuredTaskInput{AgentID: FormalAgentConversationCognition, Role: "cognitive_assessment", SchemaName: call.SchemaName, Prompt: assembly})
			if err != nil {
				t.Fatal(err)
			}
			call.Messages = assembly.Messages
			ctx = WithADKCapabilityInvoker(ctx, &adkFakeInvoker{}, &ADKCapabilityTrace{})
			if _, err := provider.generateWithEino(ctx, call); err != nil || calls.Load() != 2 {
				t.Fatalf("controlled retry calls=%d err=%v", calls.Load(), err)
			}
			// Re-enter the same logical task with the same correlation and model;
			// its next physical request must not collapse into the old evidence.
			if _, err := provider.generateWithEino(ctx, call); err != nil || calls.Load() != 3 {
				t.Fatalf("controlled reentry calls=%d err=%v", calls.Load(), err)
			}
			deadline := time.Now().Add(5 * time.Second)
			var count int
			for time.Now().Before(deadline) {
				if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.diagnostic_events WHERE event_type='provider.physical_attempt' AND correlation_id=$1`, call.CorrelationID).Scan(&count); err != nil {
					t.Fatal(err)
				}
				if count == 3 {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			rows, err := repo.Pool().Query(ctx, `SELECT payload FROM public.diagnostic_events WHERE event_type='provider.physical_attempt' AND correlation_id=$1 ORDER BY (payload->>'attempt')::integer`, call.CorrelationID)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			count = 0
			var modelCallID string
			attempts, physicalIDs := map[int]int{}, map[string]bool{}
			for rows.Next() {
				var raw []byte
				if err := rows.Scan(&raw); err != nil {
					t.Fatal(err)
				}
				count++
				payload := decodeObject(raw)
				configuration := mapValue(payload["model_configuration"])
				if count == 1 {
					modelCallID = stringValue(payload["model_call_id"])
				}
				attempts[intValue(payload["attempt"])]++
				physicalID := stringValue(payload["physical_attempt_id"])
				if physicalID == "" || physicalIDs[physicalID] {
					t.Fatalf("physical attempt %d reused its identity", count)
				}
				physicalIDs[physicalID] = true
				slots := arrayValue(payload["prompt_slots"])
				var runtimeFound, currentFound bool
				for _, rawSlot := range slots {
					slot := mapValue(rawSlot)
					refs := arrayValue(slot["source_refs"])
					runtimeFound = runtimeFound || slot["slot_id"] == "runtime_fact" && len(refs) == 1 && refs[0] == "state:retry" && slot["reason"] == "required_by_task"
					currentFound = currentFound || slot["slot_id"] == "current_input"
					if slot["slot_id"] == "tool_catalog" || slot["slot_id"] == "response_schema" {
						t.Fatalf("physical request without tools/schema recorded a nonexistent slot: %#v", slot)
					}
				}
				if !runtimeFound || !currentFound {
					t.Fatalf("physical attempt %d lost its actual slot sources: %#v", count, slots)
				}
				if modelCallID == "" || stringValue(payload["model_call_id"]) != modelCallID || configuration["model_id"] != assignment.ModelID || intValue(configuration["endpoint_revision"]) != int(assignment.EndpointRevision) || intValue(configuration["binding_revision"]) != int(assignment.BindingRevision) || intValue(configuration["credential_revision"]) != int(assignment.CredentialRevision) || strings.Contains(string(raw), fixtureSecret) {
					t.Fatalf("physical attempt %d did not record its effective configuration", count)
				}
			}
			if err := rows.Err(); err != nil || count != 3 || attempts[1] != 2 || attempts[2] != 1 {
				t.Fatalf("physical attempt records=%d err=%v", count, err)
			}
		})
	}
}

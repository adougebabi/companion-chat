package core

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestProviderToolCallFailuresPersistBoundedDiagnosticsForNativeAndStructuredSources(t *testing.T) {
	cases := []struct {
		name       string
		source     string
		wantReason string
		response   map[string]any
	}{
		{
			name:       "native",
			source:     "native",
			wantReason: "id_required",
			response: map[string]any{
				"content": "{}",
				"tool_calls": []any{map[string]any{
					"name":      "conversation.reply",
					"arguments": map[string]any{"text": "PRIVATE_NATIVE_ARGUMENT_CANARY"},
				}},
			},
		},
		{
			name:       "structured-sidecar",
			source:     "structured",
			wantReason: "id_required",
			response: map[string]any{
				"content": jsonString(map[string]any{
					"response_mode":   "final",
					"action_type":     "reply",
					"response_intent": "test",
					"tool_calls": []any{map[string]any{
						"name":      "conversation.reply",
						"arguments": map[string]any{"text": "PRIVATE_STRUCTURED_ARGUMENT_CANARY"},
					}},
					"influences": []any{},
				}),
				"tool_calls": []any{},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, repository := isolatedCoreTestRepository(t)
			suffix := stableDigest(t.Name() + time.Now().UTC().Format(time.RFC3339Nano))[:16]
			endpointID := "tool-diagnostic-endpoint-" + suffix
			if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible','http://tool-diagnostic.invalid','tool-diagnostic-secret','ready',now())`, endpointID); err != nil {
				t.Fatal(err)
			}
			if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('cognitive_assessment',$1,'tool-diagnostic-model','structured_output,tool_calling',4096,10,'{}')`, endpointID); err != nil {
				t.Fatal(err)
			}

			providerResponse := map[string]any{
				"choices": []any{map[string]any{
					"finish_reason": "stop",
					"message":       testCase.response,
				}},
			}
			providerHTTP := &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				return embeddingHTTPResponse(request, http.StatusOK, string(jsonBytes(providerResponse))), nil
			})}
			provider := &ProviderClient{DB: repository, HTTP: providerHTTP}
			correlationID := "tool-call-diagnostic:" + suffix
			providerCtx := WithProviderCorrelation(WithProviderScenario(ctx, "cognitive_assessment"), correlationID)
			_, err := provider.StructuredWithToolsSchema(
				providerCtx,
				"cognitive_assessment",
				[]map[string]any{{"role": "system", "content": "diagnostic test"}, {"role": "user", "content": "diagnostic input"}},
				[]CapabilityDefinition{conversationReplyCapabilityDefinition()},
				"conversation_turn_response",
				cognitiveTurnResponseSchema(),
				false,
			)
			if err == nil {
				t.Fatal("expected malformed tool call to fail closed")
			}

			var status, errorCode string
			var response []byte
			if err := repository.Pool().QueryRow(ctx, `SELECT status,COALESCE(error_code,''),COALESCE(response,'null'::jsonb) FROM public.diagnostic_model_runs WHERE correlation_id=$1 ORDER BY queued_at DESC LIMIT 1`, correlationID).Scan(&status, &errorCode, &response); err != nil {
				t.Fatal(err)
			}
			if status != providerRunFailed || errorCode != "tool_call_invalid" {
				t.Fatalf("tool call model run = status=%q error_code=%q", status, errorCode)
			}
			var diagnostic map[string]any
			if err := json.Unmarshal(response, &diagnostic); err != nil {
				t.Fatalf("decode tool call diagnostic: %v (%s)", err, string(response))
			}
			if stringValue(diagnostic["source"]) != testCase.source || stringValue(diagnostic["normalization_reason"]) != testCase.wantReason || intValue(diagnostic["failed_item_index"]) != 0 || intValue(diagnostic["call_count"]) != 1 {
				t.Fatalf("tool call diagnostic = %#v", diagnostic)
			}
			encoded := string(response)
			if strings.Contains(encoded, "PRIVATE_NATIVE_ARGUMENT_CANARY") || strings.Contains(encoded, "PRIVATE_STRUCTURED_ARGUMENT_CANARY") {
				t.Fatalf("tool call diagnostic leaked argument content: %s", encoded)
			}
		})
	}
}

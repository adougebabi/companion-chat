package core

import (
	"context"
	"net/http"
	"testing"
)

func TestProviderWirePayloadKeepsOutputReserveSeparateFromEstimatedInput(t *testing.T) {
	definitions := []CapabilityDefinition{conversationReplyCapabilityDefinition()}
	schema := objectSchema(map[string]any{"result": stringSchema()}, []string{"result"}, false)
	messages := []map[string]any{{"role": "system", "content": "system"}, {"role": "user", "content": "current"}}
	payload := providerChatPayloadWithSchema("model", messages, 4096, true, definitions, "cognitive_assessment", "test_schema", schema, false)
	if intValue(payload["max_tokens"]) != 4096 {
		t.Fatalf("output reserve = %#v", payload["max_tokens"])
	}
	tools := arrayValue(payload["tools"])
	responseFormat := mapValue(payload["response_format"])
	if len(tools) != 1 || len(responseFormat) == 0 {
		t.Fatalf("wire components missing: %#v", payload)
	}
	estimatedInput := EstimatePromptTokens(payload["messages"]) + EstimatePromptTokens(payload["tools"]) + EstimatePromptTokens(payload["response_format"])
	withoutOutputField := cloneMap(payload)
	delete(withoutOutputField, "max_tokens")
	if estimatedInput <= 0 || intValue(payload["max_tokens"]) == estimatedInput || withoutOutputField["max_tokens"] != nil {
		t.Fatalf("input/output budgets were conflated: input=%d payload=%#v", estimatedInput, payload)
	}
}

func TestProviderStreamingPayloadHonorsOutputReserve(t *testing.T) {
	payload := providerStreamingPayload("model", []map[string]any{{"role": "user", "content": "hello"}}, 4096)
	if streaming, _ := payload["stream"].(bool); !streaming || intValue(payload["max_tokens"]) != 4096 {
		t.Fatalf("streaming payload = %#v", payload)
	}
}

// PostgreSQL integration gate: intentionally deferred to S12 by implement.md.
func TestPostgresProviderRolePromptBudgetRoundTripAndValidation(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID := "prompt-budget-owner"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'hash','revision-1')`, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES('prompt-budget-provider','openai_compatible','http://prompt-budget.invalid','prompt-budget-secret','ready',now())`); err != nil {
		t.Fatal(err)
	}
	providerHTTP := &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return embeddingHTTPResponse(request, http.StatusOK, `{"data":[{"id":"prompt-budget-model"}]}`), nil
	})}
	app := &App{DB: repository, Provider: &ProviderClient{DB: repository, HTTP: providerHTTP}}
	payload := map[string]any{
		"role": "generic_llm", "endpoint_id": "prompt-budget-provider", "model_id": "prompt-budget-model",
		"token_budget": 4096, "timeout_seconds": 120, "context_window_tokens": 65536,
		"max_input_tokens": 49152, "prompt_budget_policy_version": promptBudgetPolicyVersionV1,
	}
	if err := app.ConfigureProviderRole(ctx, ownerID, payload); err != nil {
		t.Fatal(err)
	}
	assignment, err := app.Provider.assignment(context.Background(), "cognitive_assessment")
	if err != nil {
		t.Fatal(err)
	}
	if assignment.TokenBudget != 4096 || assignment.ContextWindowTokens != 65536 || assignment.MaxInputTokens != 49152 || assignment.PromptBudgetPolicyVersion != promptBudgetPolicyVersionV1 {
		t.Fatalf("assignment = %#v", assignment)
	}
	bindings, err := app.ProviderBindings(ctx, ownerID)
	if err != nil || len(bindings) != 1 || intValue(bindings[0]["output_reserve_tokens"]) != 4096 || intValue(bindings[0]["safety_margin_tokens"]) != 4096 {
		t.Fatalf("bindings = %#v err=%v", bindings, err)
	}
	invalid := cloneMap(payload)
	invalid["context_window_tokens"] = 55000
	if err := app.ConfigureProviderRole(ctx, ownerID, invalid); err == nil || err.Error() != "provider_prompt_budget_invalid" {
		t.Fatalf("invalid budget error = %v", err)
	}
}

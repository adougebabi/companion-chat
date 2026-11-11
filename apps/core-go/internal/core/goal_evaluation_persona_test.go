package core

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestGoalEvaluationPersonaPreservesOpaqueValues(t *testing.T) {
	portrait := ` {"id":"business-id","valid":false,"schema":{"properties":{"customer_id":{"type":"string"}}}} `
	legacy := `{"id":"business-id","name":"晴岚","score":0,"enabled":false}`
	persona := map[string]any{
		"profile_id":         "profile-secret",
		"working_persona":    map[string]any{"profile_id": "profile-secret", "portrait_text": portrait, "core_mechanisms": []string{legacy}, "preference": map[string]any{"source_ref": "business-ref", "revision": 2, "valid": false}},
		"personality_system": map[string]any{"mode": "multiple", "active_profile_id": "profile-secret", "profiles": []any{map[string]any{"id": "profile-secret", "name": "晴岚"}}},
	}
	before := jsonString(persona)
	view := mapValue(goalEvaluationPersonaSemantics(persona))
	body := mapValue(view["working_persona"])
	if body["portrait_text"] != portrait || !reflect.DeepEqual(body["core_mechanisms"], []string{legacy}) || !reflect.DeepEqual(body["preference"], map[string]any{"source_ref": "business-ref", "revision": 2, "valid": false}) {
		t.Fatalf("persona business values were changed: %#v", body)
	}
	wire := jsonString(view)
	for _, forbidden := range []string{"profile-secret", "profile_id"} {
		if strings.Contains(wire, forbidden) {
			t.Fatalf("persona bookkeeping leaked: %s", wire)
		}
	}
	if jsonString(persona) != before {
		t.Fatal("semantic projection mutated compiled persona")
	}
}

func TestGoalEvaluationPersonaSurvivesPhysicalProviderRequest(t *testing.T) {
	portrait := `{"id":"business-id","valid":false,"schema":{"properties":{"customer_id":{"type":"string"}}}}`
	working := renderCompiledWorkingPersona(CompiledWorkingPersona{ProfileID: "profile-secret", PortraitText: portrait})
	persona := goalEvaluationPersonaSemantics(systemPersonaForProjectionWithWorking(ContextProjection{}, "goal_evaluation_v1", working))
	assembled, err := AssemblePromptContext(PromptAssemblyInput{
		Role: "cognitive_assessment", OperationRules: []string{goalEvaluationInstruction}, CorePersona: mapValue(persona),
		CurrentInput: `{"goals":[]}`, Policy: DefaultPromptBudgetPolicy(4096),
	})
	if err != nil {
		t.Fatal(err)
	}
	var request map[string]any
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		request = payload
		return fakeProviderResult{Text: `{"summary":"ok"}`}
	})
	client := &ProviderClient{HTTP: &http.Client{Transport: router}}
	_, err = client.generateWithEino(context.Background(), EinoModelCall{
		Assignment: providerAssignment{Role: "cognitive_assessment", BaseURL: "http://controlled.invalid", ModelID: "fixture", Timeout: time.Second},
		Role:       "cognitive_assessment", Scenario: "goal_evaluation", Messages: assembled.Messages, JSONMode: true, SchemaName: "goal_evaluation_v1",
		ResponseSchema: objectSchema(map[string]any{"summary": stringSchema()}, []string{"summary"}, false),
	})
	if err != nil {
		t.Fatal(err)
	}
	messages := arrayValue(request["messages"])
	if len(messages) == 0 {
		t.Fatal("controlled provider did not receive messages")
	}
	system := stringValue(mapValue(messages[0])["content"])
	if !strings.Contains(system, jsonString(portrait)) || strings.Contains(system, "profile-secret") {
		t.Fatalf("physical request changed persona data or included its selector: %s", system)
	}
}

func TestGoalRuntimeOmitsNestedBookkeeping(t *testing.T) {
	execution := map[string]any{"stage": "waiting", "current_stage": map[string]any{"id": "stage-secret", "criteria_version": 8, "revision": 7, "status": "active", "purpose": "真实沟通"}, "commitments": []any{map[string]any{"id": "commit-secret", "profile_id": "profile-secret", "window_end": "2026-10-09", "status": "blocked"}}}
	view := jsonString(goalExecutionPromptView(execution))
	for _, forbidden := range []string{"stage-secret", "commit-secret", "profile-secret", "criteria_version", "revision", "profile_id"} {
		if strings.Contains(view, forbidden) {
			t.Fatalf("execution bookkeeping leaked: %s", view)
		}
	}
	for _, required := range []string{"waiting", "active", "真实沟通", "2026-10-09", "blocked"} {
		if !strings.Contains(view, required) {
			t.Fatalf("execution semantics lost: %s", view)
		}
	}
}

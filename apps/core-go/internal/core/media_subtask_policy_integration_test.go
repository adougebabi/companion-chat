package core

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestMediaStructuredSubtaskDoesNotInheritParentForcedToolPolicy(t *testing.T) {
	f := v3OwnerFixture(t, "media-policy")
	seedCognitiveProviderRole(t, f.ctx, f.repository, "media-policy-endpoint")
	calls := 0
	f.app.Provider.HTTP = &http.Client{Transport: projectHealthRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		body := decodeObject(raw)
		if len(arrayValue(body["tools"])) != 0 || body["tool_choice"] == "required" {
			return nil, errors.New("parent policy reached isolated media request")
		}
		return embeddingHTTPResponse(r, 200, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"{\"ok\":true}"}}]}`), nil
	})}
	parent := withPhysicalModelRequestPolicy(f.ctx, physicalModelRequestPolicyFunc(func(context.Context) (physicalModelRequestDecision, error) {
		return physicalModelRequestDecision{ToolChoice: schema.ToolChoiceForced, OmitResponseFormat: true}, nil
	}))
	result, err := f.app.mediaStructuredCall(parent, "media_policy_regression", "Return the bounded structure", map[string]any{"source": "fixture"}, objectSchema(map[string]any{"ok": map[string]any{"type": "boolean"}}, []string{"ok"}, false))
	if err != nil || result["ok"] != true || calls != 1 {
		t.Fatal("tool-free child inherited publication policy", result, calls, err)
	}
	cancelled, cancel := context.WithCancel(parent)
	cancel()
	if _, err = f.app.mediaStructuredCall(cancelled, "cancelled", "unused", nil, nil); !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatal("child lost original cancellation", calls, err)
	}
}

func TestPersonaCompilationSubtaskDoesNotInheritParentForcedToolPolicy(t *testing.T) {
	f := v3OwnerFixture(t, "persona-policy")
	seedCognitiveProviderRole(t, f.ctx, f.repository, "persona-policy-endpoint")
	calls := 0
	f.app.Provider.HTTP = &http.Client{Transport: projectHealthRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		body := decodeObject(raw)
		if len(arrayValue(body["tools"])) != 0 || body["tool_choice"] == "required" {
			return nil, errors.New("parent policy reached persona compiler")
		}
		return embeddingHTTPResponse(r, 200, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"{\"portrait_text\":\"摇光尊重边界，表达直接。\"}"}}]}`), nil
	})}
	parent := withPhysicalModelRequestPolicy(f.ctx, physicalModelRequestPolicyFunc(func(context.Context) (physicalModelRequestDecision, error) {
		return physicalModelRequestDecision{ToolChoice: schema.ToolChoiceForced, OmitResponseFormat: true}, nil
	}))
	input := PersonaCompilationInput{CorePersona: map[string]any{"identity": map[string]any{"name": "摇光"}}, ProfileID: "default", TargetBudgetRunes: 1024}
	result, err := f.app.CompileWorkingPersona(parent, input)
	if err != nil || result.PortraitText == "" || calls != 1 {
		t.Fatal("independent compiler inherited parent policy", result, calls, err)
	}
	cancelled, cancel := context.WithCancel(parent)
	cancel()
	if _, err := f.app.CompileWorkingPersona(cancelled, input); !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatal("compiler lost cancellation", calls, err)
	}
}

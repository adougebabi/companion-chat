package core

import (
	"context"
	"encoding/json"
	"fmt"

	openaiext "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	aiagent "github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/agent"
)

// physicalModelRequestPolicy is a task-owned, request-scoped decision made
// immediately before one physical Provider request. The common model runtime
// only applies transport options; it does not know which business task owns
// the policy or how that task determines its current state.
type physicalModelRequestPolicy interface {
	DecidePhysicalModelRequest(context.Context) (physicalModelRequestDecision, error)
}

type physicalModelRequestDecision struct {
	ToolChoice         schema.ToolChoice
	OmitResponseFormat bool
	// AllowedToolNames narrows this physical request's installed catalog only.
	// Nil preserves the task's catalog; it never grants a new capability.
	AllowedToolNames []string
	DisableThinking  bool
}

func (m *queuedToolCallingChatModel) physicalRequestOptions(ctx context.Context, opts []model.Option) (*queuedToolCallingChatModel, []model.Option, map[string]any, error) {
	if repair, _ := ctx.Value(finalRepairContextKey{}).(bool); repair {
		return m, finalRepairModelOptions(opts), m.responseFormat, nil
	}
	policy, ok := physicalModelRequestPolicyFromContext(ctx)
	if !ok {
		return m, opts, m.responseFormat, nil
	}
	decision, err := policy.DecidePhysicalModelRequest(ctx)
	if err != nil {
		return m, nil, nil, err
	}
	if decision.AllowedToolNames != nil {
		allowed := map[string]bool{}
		for _, name := range decision.AllowedToolNames {
			allowed[name] = true
		}
		definitions := []CapabilityDefinition{}
		for _, d := range m.definitions {
			if allowed[d.Name] {
				definitions = append(definitions, d)
				delete(allowed, d.Name)
			}
		}
		if len(allowed) != 0 || len(definitions) == 0 {
			return m, nil, nil, fmt.Errorf("physical_request_tool_catalog_invalid")
		}
		infos, err := aiagent.CapabilityToolInfos(definitions)
		if err != nil {
			return m, nil, nil, err
		}
		copyModel := *m
		copyModel.definitions = definitions
		m = &copyModel
		opts = append(opts, model.WithTools(infos))
	}
	ctx = withPhysicalModelRequestPolicy(ctx, physicalModelRequestPolicyFunc(func(context.Context) (physicalModelRequestDecision, error) { return decision, nil }))
	opts, format, err := applyPhysicalModelRequestPolicy(ctx, opts, m.responseFormat)
	return m, opts, format, err
}

type physicalModelRequestPolicyFunc func(context.Context) (physicalModelRequestDecision, error)

func (f physicalModelRequestPolicyFunc) DecidePhysicalModelRequest(ctx context.Context) (physicalModelRequestDecision, error) {
	return f(ctx)
}

type physicalModelRequestPolicyContextKey struct{}
type physicalModelRequestPolicyDisabled struct{}

func withPhysicalModelRequestPolicy(ctx context.Context, policy physicalModelRequestPolicy) context.Context {
	if policy == nil {
		return ctx
	}
	return context.WithValue(ctx, physicalModelRequestPolicyContextKey{}, policy)
}

func physicalModelRequestPolicyFromContext(ctx context.Context) (physicalModelRequestPolicy, bool) {
	policy, ok := ctx.Value(physicalModelRequestPolicyContextKey{}).(physicalModelRequestPolicy)
	return policy, ok && policy != nil
}

func withoutPhysicalModelRequestPolicy(ctx context.Context) context.Context {
	return context.WithValue(ctx, physicalModelRequestPolicyContextKey{}, physicalModelRequestPolicyDisabled{})
}

func applyPhysicalModelRequestPolicy(ctx context.Context, opts []model.Option, responseFormat map[string]any) ([]model.Option, map[string]any, error) {
	policy, ok := physicalModelRequestPolicyFromContext(ctx)
	if !ok {
		return opts, responseFormat, nil
	}
	decision, err := policy.DecidePhysicalModelRequest(ctx)
	if err != nil {
		return nil, nil, err
	}
	if decision.ToolChoice != "" {
		opts = append(opts, model.WithToolChoice(decision.ToolChoice))
	}
	if decision.ToolChoice == "" && !decision.OmitResponseFormat && !decision.DisableThinking {
		return opts, responseFormat, nil
	}
	if decision.ToolChoice != "" || decision.DisableThinking || decision.OmitResponseFormat {
		opts = append(opts, openaiext.WithRequestPayloadModifier(func(_ context.Context, _ []*schema.Message, raw []byte) ([]byte, error) {
			var body map[string]any
			if err := json.Unmarshal(raw, &body); err != nil {
				return nil, err
			}
			// The pinned OpenAI adapter turns forced choice with one Tool into
			// an object. Task-owned policies use the string protocol; the
			// physical catalog already expresses any allowed Tool restriction.
			switch decision.ToolChoice {
			case schema.ToolChoiceForced:
				body["tool_choice"] = "required"
			case schema.ToolChoiceAllowed:
				body["tool_choice"] = "auto"
			case schema.ToolChoiceForbidden:
				body["tool_choice"] = "none"
			}
			if decision.DisableThinking {
				body["enable_thinking"] = false
			}
			if decision.OmitResponseFormat {
				delete(body, "response_format")
			}
			return json.Marshal(body)
		}))
	}
	if decision.OmitResponseFormat {
		return opts, nil, nil
	}
	return opts, responseFormat, nil
}

// Final correction has no business admission or context refresh. Keep correlation,
// references and cancellation, but mask gates which can reinstall Tools or mutate
// persona state before a physical request.
type finalRepairContextKey struct{}

func finalRepairContext(ctx context.Context) context.Context {
	ctx = withoutPhysicalModelRequestPolicy(ctx)
	ctx = context.WithValue(ctx, kevRunSelectionKey{}, (*kevRunSelection)(nil))
	ctx = context.WithValue(ctx, kevPersonaKey{}, (*kevPersonaAdmission)(nil))
	if bridge, ok := adkCapabilityContext(ctx); ok {
		bridge.Refresh = nil
		ctx = context.WithValue(ctx, adkConversationContextKey{}, bridge)
	}
	return context.WithValue(ctx, finalRepairContextKey{}, true)
}

func finalRepairModelOptions(opts []model.Option) []model.Option {
	opts = append(append([]model.Option(nil), opts...), model.WithTools([]*schema.ToolInfo{}))
	return append(opts, openaiext.WithRequestPayloadModifier(func(_ context.Context, _ []*schema.Message, raw []byte) ([]byte, error) {
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			return nil, err
		}
		delete(body, "tools")
		delete(body, "tool_choice")
		delete(body, "parallel_tool_calls")
		body["enable_thinking"] = false
		return json.Marshal(body)
	}))
}

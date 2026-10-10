package core

import (
	"context"
	"encoding/json"

	openaiext "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
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
	if !decision.OmitResponseFormat {
		return opts, responseFormat, nil
	}
	opts = append(opts, openaiext.WithRequestPayloadModifier(func(_ context.Context, _ []*schema.Message, rawBody []byte) ([]byte, error) {
		var body map[string]any
		if err := json.Unmarshal(rawBody, &body); err != nil {
			return nil, err
		}
		delete(body, "response_format")
		return json.Marshal(body)
	}))
	return opts, nil, nil
}

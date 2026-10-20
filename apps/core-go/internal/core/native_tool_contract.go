package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	aiagent "github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/agent"
	capabilitycontract "github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
)

// A capability owns the model contract alongside its canonical direct contract.
// Binding runs before native-ID or durable receipt replay; it grants no scope.
type nativeToolContract interface {
	NativeDefinition() CapabilityDefinition
	BindNativeArguments(json.RawMessage, ContextProjection) (json.RawMessage, error)
	NativeOutput(any, ContextProjection) any
}

type nativeDefinitionProvider interface {
	NativeDefinitions([]CapabilityDefinition) []CapabilityDefinition
}

func (i *appADKCapabilityInvoker) NativeDefinitions(definitions []CapabilityDefinition) []CapabilityDefinition {
	result := append([]CapabilityDefinition(nil), definitions...)
	for position, definition := range result {
		capability, ok := i.scopeRegistry.Lookup(definition.Name)
		if scoped, exists := i.scopeRegistry.Definition(definition.Name); exists {
			result[position] = scoped
		}
		if contract, yes := capability.(nativeToolContract); ok && yes {
			result[position] = contract.NativeDefinition()
		}
	}
	return result
}
func nativeDefinitionsForContext(ctx context.Context, definitions []CapabilityDefinition) []CapabilityDefinition {
	if state, ok := adkCapabilityContext(ctx); ok {
		if provider, yes := state.Invoker.(nativeDefinitionProvider); yes {
			return provider.NativeDefinitions(definitions)
		}
	}
	return definitions
}
func (m *queuedToolCallingChatModel) nativeModelOptions(ctx context.Context, opts []model.Option) (*queuedToolCallingChatModel, []model.Option, error) {
	if len(m.definitions) == 0 {
		return m, opts, nil
	}
	definitions := nativeDefinitionsForContext(ctx, m.definitions)
	infos, err := aiagent.CapabilityToolInfos(definitions)
	if err != nil {
		return m, nil, err
	}
	copyModel := *m
	copyModel.definitions = definitions
	return &copyModel, append(append([]model.Option(nil), opts...), model.WithTools(infos)), nil
}
func validateNativeArguments(arguments json.RawMessage, definition CapabilityDefinition) error {
	var value any
	if err := json.Unmarshal(arguments, &value); err != nil {
		return errors.New("invalid_arguments: expected an object matching the native tool parameters")
	}
	value = prepareModelValue(value, definition.InputSchema)
	if err := capabilitycontract.ValidateCapabilitySchemaValue(value, definition.InputSchema); err != nil {
		return fmt.Errorf("invalid_arguments: %s", safeToolArgumentFeedback(err))
	}
	return nil
}

func nativeProjection(ctx context.Context) (ContextProjection, bool) {
	if state, ok := adkCapabilityContext(ctx); ok {
		if invoker, yes := state.Invoker.(*appADKCapabilityInvoker); yes {
			if fresh := state.Refresh.latestProjection(); fresh != nil {
				return *fresh, true
			}
			return invoker.request.Projection, true
		}
	}
	return ContextProjection{}, false
}

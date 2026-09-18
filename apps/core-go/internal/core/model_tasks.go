package core

import (
	"context"
	"errors"
	"strings"
)

// ModelTaskKind names the bounded responsibilities that may call a model.
// Tasks share EinoModelRuntime support but do not share prompts, schemas or
// domain state.
type ModelTaskKind string

const (
	ModelTaskMainConversation     ModelTaskKind = "main_conversation"
	ModelTaskQueryContinuation    ModelTaskKind = "query_continuation"
	ModelTaskTakeoverJudge        ModelTaskKind = "takeover_judge"
	ModelTaskTakeoverReply        ModelTaskKind = "takeover_reply"
	ModelTaskStructuredAssessment ModelTaskKind = "structured_assessment"
	ModelTaskText                 ModelTaskKind = "text"
	ModelTaskMultimodalAssessment ModelTaskKind = "multimodal_assessment"
	ModelTaskEmbedding            ModelTaskKind = "embedding"
	ModelTaskStream               ModelTaskKind = "stream"
)

// ModelTask is metadata for an operation-owned model call. It is deliberately
// small: no App, repository, prompt history or mutable Agent state is kept.
type ModelTask struct {
	Kind       ModelTaskKind
	Role       string
	Scenario   string
	SchemaName string
}

func (a *App) modelTaskProvider() (*ProviderClient, error) {
	if a == nil || a.Provider == nil {
		return nil, errors.New("model_task_provider_unavailable")
	}
	return a.Provider, nil
}

func (a *App) RunStructuredTask(ctx context.Context, task ModelTask, messages []map[string]any, schema map[string]any, thinking bool) (map[string]any, error) {
	provider, err := a.modelTaskProvider()
	if err != nil {
		return nil, err
	}
	return provider.StructuredWithSchema(WithProviderScenario(ctx, task.Scenario), task.Role, messages, task.SchemaName, schema, thinking)
}

func (a *App) RunInitializationTask(ctx context.Context, messages []map[string]any) (map[string]any, error) {
	provider, err := a.modelTaskProvider()
	if err != nil {
		return nil, err
	}
	return provider.Structured(WithProviderScenario(ctx, "initialization"), "initialization", messages)
}

func (a *App) RunStructuredToolsTask(ctx context.Context, task ModelTask, messages []map[string]any, definitions []CapabilityDefinition, schema map[string]any, assembled bool, thinking bool) (ProviderCompletion, error) {
	provider, err := a.modelTaskProvider()
	if err != nil {
		return ProviderCompletion{}, err
	}
	if strings.TrimSpace(task.Scenario) != "" {
		ctx = WithProviderScenario(ctx, task.Scenario)
	}
	if assembled {
		return provider.StructuredAssembledWithToolsSchema(ctx, task.Role, messages, definitions, task.SchemaName, schema, thinking)
	}
	return provider.StructuredWithToolsSchema(ctx, task.Role, messages, definitions, task.SchemaName, schema, thinking)
}

func (a *App) RunTextTask(ctx context.Context, task ModelTask, messages []map[string]any) (string, error) {
	provider, err := a.modelTaskProvider()
	if err != nil {
		return "", err
	}
	return provider.Text(WithProviderScenario(ctx, task.Scenario), task.Role, messages)
}

func (a *App) RunStreamTask(ctx context.Context, task ModelTask, messages []map[string]any, onChunk func(string) error) (string, error) {
	provider, err := a.modelTaskProvider()
	if err != nil {
		return "", err
	}
	return provider.StreamText(WithProviderScenario(ctx, task.Scenario), task.Role, messages, onChunk)
}

func (a *App) RunEmbeddingTask(ctx context.Context, text string) (string, []float64, error) {
	provider, err := a.modelTaskProvider()
	if err != nil {
		return "", nil, err
	}
	return provider.Embed(WithProviderScenario(ctx, "memory_retrieval"), text)
}

// RunFrozenEmbeddingTask keeps a workflow-pinned assignment while still
// exposing an operation-owned task boundary. It is used by durable embedding
// intents whose endpoint/model tuple must not be re-resolved on retry.
func (a *App) RunFrozenEmbeddingTask(ctx context.Context, text string, assignment providerAssignment) (string, []float64, error) {
	provider, err := a.modelTaskProvider()
	if err != nil {
		return "", nil, err
	}
	return provider.embedWithAssignment(ctx, text, assignment)
}

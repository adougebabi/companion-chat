package model

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	embeddingopenai "github.com/cloudwego/eino-ext/components/embedding/openai"
	openaiext "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
)

// EinoModelConfig is the transport-neutral configuration needed to construct
// an official Eino model component. Secret resolution and assignment lookup
// happen outside this value object.
type EinoModelConfig struct {
	APIKey              string
	BaseURL             string
	Model               string
	Timeout             time.Duration
	MaxCompletionTokens int
	HTTPClient          *http.Client
	ResponseFormat      *openaiext.ChatCompletionResponseFormat
	ExtraFields         map[string]any
}

// EinoModelFactory constructs official Eino components. It deliberately has
// no database or domain dependency, which keeps model construction testable
// and prevents prompt/task code from reaching into App.
type EinoModelFactory struct {
	HTTPClient *http.Client
}

func NewEinoModelFactory(client *http.Client) EinoModelFactory {
	return EinoModelFactory{HTTPClient: client}
}

func (f EinoModelFactory) NewChatModel(ctx context.Context, config EinoModelConfig) (model.ToolCallingChatModel, error) {
	if strings.TrimSpace(config.Model) == "" {
		return nil, errors.New("eino_model_required")
	}
	client := config.HTTPClient
	if client == nil {
		client = f.HTTPClient
	}
	if client == nil {
		client = &http.Client{Timeout: config.Timeout}
	}
	var maxCompletionTokens *int
	if config.MaxCompletionTokens > 0 {
		maxCompletionTokens = &config.MaxCompletionTokens
	}
	chat, err := openaiext.NewChatModel(ctx, &openaiext.ChatModelConfig{
		APIKey:              config.APIKey,
		BaseURL:             strings.TrimRight(config.BaseURL, "/"),
		Model:               config.Model,
		HTTPClient:          client,
		Timeout:             config.Timeout,
		MaxCompletionTokens: maxCompletionTokens,
		ResponseFormat:      config.ResponseFormat,
		ExtraFields:         cloneMap(config.ExtraFields),
	})
	if err != nil {
		return nil, fmt.Errorf("eino_chat_model_create: %w", err)
	}
	return chat, nil
}

func (f EinoModelFactory) NewEmbedder(ctx context.Context, config EinoModelConfig) (embedding.Embedder, error) {
	if strings.TrimSpace(config.Model) == "" {
		return nil, errors.New("eino_embedding_model_required")
	}
	client := config.HTTPClient
	if client == nil {
		client = f.HTTPClient
	}
	if client == nil {
		client = &http.Client{Timeout: config.Timeout}
	}
	emb, err := embeddingopenai.NewEmbedder(ctx, &embeddingopenai.EmbeddingConfig{
		APIKey:     config.APIKey,
		BaseURL:    strings.TrimRight(config.BaseURL, "/"),
		Model:      config.Model,
		HTTPClient: client,
		Timeout:    config.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("eino_embedder_create: %w", err)
	}
	return emb, nil
}

func cloneMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	res := make(map[string]any, len(value))
	for k, v := range value {
		res[k] = v
	}
	return res
}

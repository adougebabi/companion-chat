package core

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MemoryService coordinates memory lifecycle, semantic recall, embeddings, and active working memory.
// It decouples memory domain operations from the monolithic App struct.
type MemoryService struct {
	pool *pgxpool.Pool
	app  *App
}

// NewMemoryService constructs a MemoryService backed by the application's database pool and app runtime.
func NewMemoryService(pool *pgxpool.Pool, app *App) *MemoryService {
	return &MemoryService{
		pool: pool,
		app:  app,
	}
}

// RecordMemory creates a new declarative or episodic memory record.
func (s *MemoryService) RecordMemory(ctx context.Context, fluctlightID, actorID string, payload map[string]any) (map[string]any, error) {
	return s.app.RecordMemory(ctx, fluctlightID, actorID, payload)
}

// ReviseMemory updates the content or associations of an existing memory record.
func (s *MemoryService) ReviseMemory(ctx context.Context, actorID, id, content string, expected *int, refs []any) (map[string]any, error) {
	return s.app.ReviseMemory(ctx, actorID, id, content, expected, refs)
}

// ForgetMemory soft-deletes or marks a memory record as deprecated/forgotten.
func (s *MemoryService) ForgetMemory(ctx context.Context, actorID, id string, expected *int, refs []any) (map[string]any, error) {
	return s.app.ForgetMemory(ctx, actorID, id, expected, refs)
}

// RollbackMemory restores an earlier revision of a memory record.
func (s *MemoryService) RollbackMemory(ctx context.Context, actorID, memoryID string, targetRevision, expectedRevision int, evidenceRefs []any) (map[string]any, error) {
	return s.app.RollbackMemory(ctx, actorID, memoryID, targetRevision, expectedRevision, evidenceRefs)
}

// RetrieveMemoryContext performs ranked semantic retrieval across the user's memory store.
func (s *MemoryService) RetrieveMemoryContext(ctx context.Context, actorID, fluctlightID, conversationID, query string, limit, tokenBudget int) ([]map[string]any, error) {
	return s.app.RetrieveMemoryContext(ctx, actorID, fluctlightID, conversationID, query, limit, tokenBudget)
}

// RetrieveActiveMemories retrieves the current working set of active memories for prompt context.
func (s *MemoryService) RetrieveActiveMemories(ctx context.Context, query ActiveMemoryQuery) (ActiveMemoryRetrievalResult, error) {
	return s.app.retrieveActiveMemories(ctx, query)
}

// RetrieveMemoryWithPlan executes a planned memory query with token and ranking constraints.
func (s *MemoryService) RetrieveMemoryWithPlan(ctx context.Context, authorizationActorID, fluctlightID string, plan MemoryQueryPlan) (MemoryRetrievalResult, error) {
	return s.app.retrieveMemoryWithPlan(ctx, authorizationActorID, fluctlightID, plan)
}

// ProcessMemoryEmbedding generates and persists vector embeddings for a memory revision.
func (s *MemoryService) ProcessMemoryEmbedding(ctx context.Context, memoryID string) (map[string]any, error) {
	return s.app.ProcessMemoryEmbedding(ctx, memoryID)
}

// ProcessMemoryEmbeddingAt generates and persists vector embeddings for a specific revision.
func (s *MemoryService) ProcessMemoryEmbeddingAt(ctx context.Context, memoryID string, requestedRevision int) (map[string]any, error) {
	return s.app.ProcessMemoryEmbeddingAt(ctx, memoryID, requestedRevision)
}

// ProcessMemoryEmbeddingIntentAt executes a scheduled memory embedding workflow intent.
func (s *MemoryService) ProcessMemoryEmbeddingIntentAt(ctx context.Context, intentID, memoryID string, requestedRevision int, providerEndpointID, modelID string) (map[string]any, error) {
	return s.app.ProcessMemoryEmbeddingIntentAt(ctx, intentID, memoryID, requestedRevision, providerEndpointID, modelID)
}

// --- Capability Integration Methods (implementing memoryCapabilityService & activeMemoryCapabilityService) ---

func (s *MemoryService) prepareMemoryCapability(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityInvocation, error) {
	return s.app.prepareMemoryCapability(ctx, invocation, resolved)
}

func (s *MemoryService) applyMemoryCapability(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	return s.app.applyMemoryCapability(ctx, invocation, resolved)
}

func (s *MemoryService) applyMemoryCapabilityTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	return s.app.applyMemoryCapabilityTx(ctx, tx, invocation, resolved)
}

func (s *MemoryService) prepareActiveMemoryCapability(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityInvocation, error) {
	return s.app.prepareActiveMemoryCapability(ctx, invocation, resolved)
}

func (s *MemoryService) applyActiveMemoryCapability(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	return s.app.applyActiveMemoryCapability(ctx, invocation, resolved)
}

func (s *MemoryService) applyActiveMemoryCapabilityTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	return s.app.applyActiveMemoryCapabilityTx(ctx, tx, invocation, resolved)
}

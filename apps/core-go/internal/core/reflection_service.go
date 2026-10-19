package core

import (
	"context"
	"net/url"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (s *ReflectionService) Browse(ctx context.Context, owner, fid string, query url.Values) (map[string]any, error) {
	return s.app.OwnerReflectionCatalog(ctx, owner, fid, query)
}
func (s *ReflectionService) Read(ctx context.Context, owner, fid, id string) (map[string]any, error) {
	return s.app.OwnerReflection(ctx, owner, fid, id)
}
func (s *ReflectionService) Feedback(ctx context.Context, owner, fid, id string, query url.Values) (map[string]any, error) {
	return s.app.OwnerReflectionFeedback(ctx, owner, fid, id, query)
}

// ReflectionService coordinates reflection window evaluation, proposal generation,
// memory consolidation, and evolution overlay application.
type ReflectionService struct {
	pool *pgxpool.Pool
	app  *App
}

// NewReflectionService constructs a ReflectionService backed by the application infrastructure.
func NewReflectionService(pool *pgxpool.Pool, app *App) *ReflectionService {
	return &ReflectionService{
		pool: pool,
		app:  app,
	}
}

// ProcessReflection executes a triggered reflection cycle for a fluctlight.
func (s *ReflectionService) ProcessReflection(ctx context.Context, fluctlightID, correlationID string) (map[string]any, error) {
	return s.app.ProcessReflection(ctx, fluctlightID, correlationID)
}

// ClaimReflectionWindow acquires an exclusive processing claim on the reflection sliding window.
func (s *ReflectionService) ClaimReflectionWindow(ctx context.Context, fluctlightID string, watermark, stateRevision int, correlationID string) (context.Context, error) {
	return s.app.claimReflectionWindow(ctx, fluctlightID, watermark, stateRevision, correlationID)
}

// SetReflectionWindowIdle releases the exclusive reflection window processing state.
func (s *ReflectionService) SetReflectionWindowIdle(ctx context.Context, fluctlightID string) error {
	return s.app.setReflectionWindowIdle(ctx, fluctlightID)
}

// ApplyReflectionMemoryCommandsTx persists declarative memory mutations planned by reflection.
func (s *ReflectionService) ApplyReflectionMemoryCommandsTx(ctx context.Context, tx pgx.Tx, commands []PreparedMemoryMutation) ([]MemoryApplyResult, error) {
	return s.app.applyReflectionMemoryCommandsTx(ctx, tx, commands)
}

// ApplyReflectionActiveMemoryCommandsTx persists working memory mutations planned by reflection.
func (s *ReflectionService) ApplyReflectionActiveMemoryCommandsTx(ctx context.Context, tx pgx.Tx, commands []PreparedActiveMemoryMutation) ([]ActiveMemoryApplyResult, error) {
	return s.app.applyReflectionActiveMemoryCommandsTx(ctx, tx, commands)
}

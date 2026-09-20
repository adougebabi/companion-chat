package core

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// LifeContextService manages the life context model, authoritative revision gates,
// and scene/presence capability preparation.
type LifeContextService struct {
	pool *pgxpool.Pool
	app  *App
}

// NewLifeContextService constructs a LifeContextService backed by the application infrastructure.
func NewLifeContextService(pool *pgxpool.Pool, app *App) *LifeContextService {
	return &LifeContextService{
		pool: pool,
		app:  app,
	}
}

// ReadLifeContextSnapshotAt resolves the current schedule and life context snapshot at a given timestamp.
func (s *LifeContextService) ReadLifeContextSnapshotAt(ctx context.Context, fluctlightID string, at time.Time) (map[string]any, map[string]any, error) {
	return s.app.readLifeContextSnapshotAt(ctx, fluctlightID, at)
}

// ReadFoundationLifeSnapshotAt resolves foundation profile and life context at a given timestamp.
func (s *LifeContextService) ReadFoundationLifeSnapshotAt(ctx context.Context, fluctlightID, ownerActorID string, at time.Time) (Fluctlight, map[string]any, map[string]any, error) {
	return s.app.readFoundationLifeSnapshotAt(ctx, fluctlightID, ownerActorID, at)
}

// RequireLifeContextRevisionTx enforces optimistic concurrency revision on the life context within a transaction.
func (s *LifeContextService) RequireLifeContextRevisionTx(ctx context.Context, tx pgx.Tx, fluctlightID, expectedRevision string, at time.Time) (map[string]any, error) {
	return s.app.requireLifeContextRevisionTx(ctx, tx, fluctlightID, expectedRevision, at)
}

// ValidateLifeContextRevision verifies optimistic revision without an open transaction.
func (s *LifeContextService) ValidateLifeContextRevision(ctx context.Context, fluctlightID, expectedRevision string, at time.Time) error {
	return s.app.validateLifeContextRevision(ctx, fluctlightID, expectedRevision, at)
}

// RequireCognitionAuthorityRevisionsTx checks foundation, current state, and life context revisions atomically.
func (s *LifeContextService) RequireCognitionAuthorityRevisionsTx(ctx context.Context, tx pgx.Tx, fluctlightID string, expectedFoundation, expectedCurrentState int, expectedLife string, at time.Time) error {
	return s.app.requireCognitionAuthorityRevisionsTx(ctx, tx, fluctlightID, expectedFoundation, expectedCurrentState, expectedLife, at)
}

// --- Capability Integration Methods (implementing sceneCapabilityService & presenceCapabilityService) ---

func (s *LifeContextService) prepareSceneCapability(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityInvocation, error) {
	return s.app.prepareSceneCapability(ctx, invocation, resolved)
}

func (s *LifeContextService) applySceneCapability(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	return s.app.applySceneCapability(ctx, invocation, resolved)
}

func (s *LifeContextService) applySceneCapabilityTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	return s.app.applySceneCapabilityTx(ctx, tx, invocation, resolved)
}

func (s *LifeContextService) preparePresenceCapability(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityInvocation, error) {
	return s.app.preparePresenceCapability(ctx, invocation, resolved)
}

func (s *LifeContextService) applyPresenceCapability(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	return s.app.applyPresenceCapability(ctx, invocation, resolved)
}

func (s *LifeContextService) applyPresenceCapabilityTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	return s.app.applyPresenceCapabilityTx(ctx, tx, invocation, resolved)
}

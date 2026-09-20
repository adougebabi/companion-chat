package core

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ScheduleService coordinates schedule lifecycle, generation, replanning, and capability execution.
// It decouples schedule domain logic from the monolithic App struct.
type ScheduleService struct {
	pool    *pgxpool.Pool
	planner SchedulePlanner
	app     *App
}

// NewScheduleService constructs a ScheduleService backed by the application's database pool and planner.
func NewScheduleService(pool *pgxpool.Pool, planner SchedulePlanner, app *App) *ScheduleService {
	return &ScheduleService{
		pool:    pool,
		planner: planner,
		app:     app,
	}
}

// AcceptSchedule records an accepted schedule plan within an atomic transaction.
func (s *ScheduleService) AcceptSchedule(ctx context.Context, actorID, fluctlightID string, payload map[string]any) (map[string]any, error) {
	return s.app.acceptScheduleWithTx(ctx, nil, actorID, fluctlightID, payload)
}

// AcceptScheduleTx records an accepted schedule plan within an existing caller transaction.
func (s *ScheduleService) AcceptScheduleTx(ctx context.Context, tx pgx.Tx, actorID, fluctlightID string, payload map[string]any) (map[string]any, error) {
	return s.app.acceptScheduleWithTx(ctx, tx, actorID, fluctlightID, payload)
}

// ReplanSchedule applies an updated schedule plan within an atomic transaction.
func (s *ScheduleService) ReplanSchedule(ctx context.Context, actorID, fluctlightID string, payload map[string]any) (map[string]any, error) {
	return s.app.replanScheduleWithTx(ctx, nil, actorID, fluctlightID, payload)
}

// ReplanScheduleTx applies an updated schedule plan within an existing caller transaction.
func (s *ScheduleService) ReplanScheduleTx(ctx context.Context, tx pgx.Tx, actorID, fluctlightID string, payload map[string]any) (map[string]any, error) {
	return s.app.replanScheduleWithTx(ctx, tx, actorID, fluctlightID, payload)
}

// CurrentAcceptedSchedule retrieves the currently accepted schedule for the given fluctlight.
func (s *ScheduleService) CurrentAcceptedSchedule(ctx context.Context, fluctlightID string) (map[string]any, error) {
	return s.app.currentAcceptedSchedule(ctx, fluctlightID)
}

// CurrentAcceptedScheduleTx retrieves the currently accepted schedule within a transaction.
func (s *ScheduleService) CurrentAcceptedScheduleTx(ctx context.Context, tx pgx.Tx, fluctlightID string) (map[string]any, error) {
	return s.app.currentAcceptedScheduleTx(ctx, tx, fluctlightID)
}

// GenerateInitialSchedule generates and accepts the first schedule for a given local day.
func (s *ScheduleService) GenerateInitialSchedule(ctx context.Context, ownerID, fluctlightID, localDate, timezone, expectedLifeContextRevision string, identity, lifeProfile map[string]any) (map[string]any, error) {
	return s.app.generateInitialSchedule(ctx, ownerID, fluctlightID, localDate, timezone, expectedLifeContextRevision, identity, lifeProfile)
}

// EnsureCurrentDaySchedule ensures that a valid schedule exists for the current local day, generating one if needed.
func (s *ScheduleService) EnsureCurrentDaySchedule(ctx context.Context, fluctlightID string) (map[string]any, error) {
	return s.app.EnsureCurrentDaySchedule(ctx, fluctlightID)
}

// ApplyScheduleReplanCapability executes the schedule.replan capability outside a caller transaction.
func (s *ScheduleService) ApplyScheduleReplanCapability(ctx context.Context, invocation CapabilityInvocation) (CapabilityResult, error) {
	return s.app.applyScheduleReplanCapability(ctx, invocation)
}

// ApplyScheduleReplanCapabilityTx executes the schedule.replan capability within a caller transaction.
func (s *ScheduleService) ApplyScheduleReplanCapabilityTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	return s.app.applyScheduleReplanCapabilityTx(ctx, tx, invocation, resolved)
}

// Planner returns the schedule planner associated with this service.
func (s *ScheduleService) Planner() SchedulePlanner {
	if s == nil {
		return nil
	}
	return s.planner
}

// SetPlanner updates the schedule planner implementation.
func (s *ScheduleService) SetPlanner(planner SchedulePlanner) {
	if s != nil {
		s.planner = planner
	}
}

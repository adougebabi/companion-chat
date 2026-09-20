package core

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CognitionService coordinates asynchronous cognition, background wake-up,
// daily reviews, autonomous actions, and reflective learning.
// It decouples autonomous cognition orchestration from the monolithic App struct.
type CognitionService struct {
	pool *pgxpool.Pool
	app  *App
}

// NewCognitionService constructs a CognitionService backed by the application database and runtime.
func NewCognitionService(pool *pgxpool.Pool, app *App) *CognitionService {
	return &CognitionService{
		pool: pool,
		app:  app,
	}
}

// ProcessCognitionInbox processes an enqueued cognition inbox fact.
func (s *CognitionService) ProcessCognitionInbox(ctx context.Context, inboxID string) (map[string]any, error) {
	return s.app.ProcessCognitionInbox(ctx, inboxID)
}

// ProcessWakeUp executes a periodic or proactive wake-up cycle for a fluctlight.
func (s *CognitionService) ProcessWakeUp(ctx context.Context, fluctlightID string, cycle int) (map[string]any, error) {
	return s.app.ProcessWakeUp(ctx, fluctlightID, cycle)
}

// ProcessAutonomyAction executes a scheduled or spontaneous autonomous action.
func (s *CognitionService) ProcessAutonomyAction(ctx context.Context, actionID string) (map[string]any, error) {
	return s.app.ProcessAutonomyAction(ctx, actionID)
}

// ProcessDailyReview executes the end-of-day review and memory consolidation workflow.
func (s *CognitionService) ProcessDailyReview(ctx context.Context, fluctlightID, localDate string) (map[string]any, error) {
	return s.app.ProcessDailyReview(ctx, fluctlightID, localDate)
}

// ProcessReflection executes a reflection cycle triggered by significant events.
func (s *CognitionService) ProcessReflection(ctx context.Context, fluctlightID, triggerEventID string) (map[string]any, error) {
	return s.app.ProcessReflection(ctx, fluctlightID, triggerEventID)
}

// ProcessIntentionTrigger executes an intention trigger evaluation.
func (s *CognitionService) ProcessIntentionTrigger(ctx context.Context, intentionID string) (map[string]any, error) {
	return s.app.ProcessIntentionTrigger(ctx, intentionID)
}

// CancelLifecycleForCognition applies preemption to prevent concurrent background cycles from racing user cognition.
func (s *CognitionService) CancelLifecycleForCognition(ctx context.Context, fluctlightID, cause string) error {
	return s.app.CancelLifecycleForCognition(ctx, fluctlightID, cause)
}

// FailAutonomyAction records a failure on an autonomous action.
func (s *CognitionService) FailAutonomyAction(ctx context.Context, actionID, reason string) (map[string]any, error) {
	return s.app.FailAutonomyAction(ctx, actionID, reason)
}

// ProcessCapabilityAction executes a platform capability action.
func (s *CognitionService) ProcessCapabilityAction(ctx context.Context, actionID string) (map[string]any, error) {
	return s.app.ProcessCapabilityAction(ctx, actionID)
}

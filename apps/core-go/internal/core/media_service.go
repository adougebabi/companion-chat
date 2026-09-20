package core

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
)

// MediaService coordinates media intent processing, asset storage, quality evaluation,
// serving, and visual identity initialization. It decouples media operations from App.
type MediaService struct {
	pool    *pgxpool.Pool
	storage *minio.Client
	bucket  string
	app     *App
}

// NewMediaService constructs a MediaService backed by the application's infrastructure.
func NewMediaService(pool *pgxpool.Pool, storage *minio.Client, bucket string, app *App) *MediaService {
	return &MediaService{
		pool:    pool,
		storage: storage,
		bucket:  bucket,
		app:     app,
	}
}

// ProcessMediaIntent executes generation and storage for a pending media intent.
func (s *MediaService) ProcessMediaIntent(ctx context.Context, intentID string) (map[string]any, error) {
	return s.app.ProcessMediaIntent(ctx, intentID)
}

// RetryMediaIntent requests a retry for an eligible failed media intent.
func (s *MediaService) RetryMediaIntent(ctx context.Context, actorID, intentID string) (map[string]any, error) {
	return s.app.RetryMediaIntent(ctx, actorID, intentID)
}

// ServeMedia streams an asset from object storage with range header support.
func (s *MediaService) ServeMedia(ctx context.Context, writer http.ResponseWriter, assetID, rangeHeader string) error {
	return s.app.ServeMedia(ctx, writer, assetID, rangeHeader)
}

// MediaPromptsFiltered lists recent media prompts for an actor.
func (s *MediaService) MediaPromptsFiltered(ctx context.Context, actorID string, limit int) ([]map[string]any, error) {
	return s.app.MediaPromptsFiltered(ctx, actorID, limit)
}

// RecordMediaActivityFailure records a terminal workflow failure for a media intent.
func (s *MediaService) RecordMediaActivityFailure(ctx context.Context, intentID, message string) error {
	return s.app.RecordMediaActivityFailure(ctx, intentID, message)
}

// ProcessVisualIdentity advances an in-flight visual identity generation session.
func (s *MediaService) ProcessVisualIdentity(ctx context.Context, sessionID string) (map[string]any, error) {
	return s.app.ProcessVisualIdentity(ctx, sessionID)
}

// EnsureVisualIdentityInitialization ensures that a visual identity exists for the fluctlight.
func (s *MediaService) EnsureVisualIdentityInitialization(ctx context.Context, fluctlightID, triggerType, sourceFactID string) (string, error) {
	return s.app.EnsureVisualIdentityInitialization(ctx, fluctlightID, triggerType, sourceFactID)
}

// EnsureVisualIdentityInitializationWithPersona ensures visual identity initialization with explicit persona.
func (s *MediaService) EnsureVisualIdentityInitializationWithPersona(ctx context.Context, fluctlightID, triggerType, sourceFactID string, persona map[string]any) (string, error) {
	return s.app.EnsureVisualIdentityInitializationWithPersona(ctx, fluctlightID, triggerType, sourceFactID, persona)
}

// EnsureVisualIdentityInitializationWithPersonaTx ensures visual identity initialization within an existing transaction.
func (s *MediaService) EnsureVisualIdentityInitializationWithPersonaTx(ctx context.Context, tx pgx.Tx, fluctlightID, triggerType, sourceFactID string, persona map[string]any) (string, error) {
	return s.app.EnsureVisualIdentityInitializationWithPersonaTx(ctx, tx, fluctlightID, triggerType, sourceFactID, persona)
}

// --- Image Capability Integration Methods (implementing imageCapabilityService) ---

func (s *MediaService) preflightImageCapability(ctx context.Context) error {
	return s.app.preflightImageCapability(ctx)
}

func (s *MediaService) preflightImageCapabilityTx(ctx context.Context, tx pgx.Tx) error {
	return s.app.preflightImageCapabilityTx(ctx, tx)
}

func (s *MediaService) createMediaIntentTargetTx(ctx context.Context, tx pgx.Tx, fluctlightID string, concept map[string]any, id, workflowID, requestID, conversationID, messageID, momentID string) error {
	return s.app.createMediaIntentTargetTx(ctx, tx, fluctlightID, concept, id, workflowID, requestID, conversationID, messageID, momentID)
}

func (s *MediaService) requireLifeContextRevisionTx(ctx context.Context, tx pgx.Tx, fluctlightID, expected string, now time.Time) (map[string]any, error) {
	return s.app.requireLifeContextRevisionTx(ctx, tx, fluctlightID, expected, now)
}


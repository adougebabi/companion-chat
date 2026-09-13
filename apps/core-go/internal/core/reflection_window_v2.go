package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

type reflectionWindowLeaseContextKey struct{}

type reflectionWindowLease struct {
	FluctlightID  string
	CorrelationID string
	Token         time.Time
}

func (a *App) claimReflectionWindow(ctx context.Context, fluctlightID string, watermark, stateRevision int, correlationID string) (context.Context, error) {
	var leaseToken time.Time
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var status string
		var updatedAt time.Time
		err := tx.QueryRow(ctx, `SELECT status,updated_at FROM public.cognition_reflection_windows WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&status, &updatedAt)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil && status == "running" && time.Since(updatedAt) < 15*time.Minute {
			return ErrConflict
		}
		requestedToken := time.Now().UTC()
		if errors.Is(err, pgx.ErrNoRows) {
			return tx.QueryRow(ctx, `
				INSERT INTO public.cognition_reflection_windows(
					fluctlight_id,watermark,state_revision,status,updated_at
				) VALUES($1,$2,$3,'running',$4)
				RETURNING updated_at`,
				fluctlightID, watermark, stateRevision, requestedToken,
			).Scan(&leaseToken)
		}
		return tx.QueryRow(ctx, `
			UPDATE public.cognition_reflection_windows
			SET status='running',state_revision=$2,updated_at=$3
			WHERE fluctlight_id=$1
			RETURNING updated_at`,
			fluctlightID, stateRevision, requestedToken,
		).Scan(&leaseToken)
	})
	if err != nil {
		return ctx, err
	}
	lease := reflectionWindowLease{
		FluctlightID:  fluctlightID,
		CorrelationID: correlationID,
		Token:         leaseToken.UTC(),
	}
	return context.WithValue(ctx, reflectionWindowLeaseContextKey{}, lease), nil
}

func (a *App) setReflectionWindowIdle(ctx context.Context, fluctlightID string) error {
	lease, ok := ctx.Value(reflectionWindowLeaseContextKey{}).(reflectionWindowLease)
	if !ok || lease.FluctlightID != fluctlightID || lease.Token.IsZero() {
		return errors.New("reflection_window_lease_missing")
	}
	command, err := a.DB.Pool().Exec(ctx, `
		UPDATE public.cognition_reflection_windows
		SET status='idle',updated_at=now()
		WHERE fluctlight_id=$1
		  AND status='running'
		  AND updated_at=$2`, fluctlightID, lease.Token)
	if err == nil && command.RowsAffected() == 1 {
		return nil
	}
	if err == nil {
		err = errors.New("reflection_window_lease_conflict")
	}
	slog.Warn("Go Core Reflection window release failed",
		"fluctlight_id", fluctlightID,
		"correlation_id", lease.CorrelationID,
		"error_type", fmt.Sprintf("%T", err),
	)
	a.RecordLifecycleDiagnosticBestEffort(ctx, LifecycleDiagnostic{
		Surface: "reflection", Transition: LifecycleTransitionFailed, Severity: "error",
		FluctlightID: fluctlightID, CorrelationID: lease.CorrelationID,
		Stage: "window_release", Status: "failed",
		ReasonCode:    "reflection_window_lease_release_failed",
		ErrorCategory: "conflict", ErrorCode: "reflection_window_lease_release_failed",
		Retryable: true,
	})
	return err
}

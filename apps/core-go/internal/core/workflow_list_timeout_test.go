package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

type workflowListDeadlineProbe struct {
	WorkflowRuntime
	deadline  time.Time
	remaining time.Duration
	query     string
	limit     int
	block     bool
}

func (probe *workflowListDeadlineProbe) List(ctx context.Context, query string, limit int) ([]WorkflowExecution, error) {
	probe.deadline, _ = ctx.Deadline()
	probe.remaining = time.Until(probe.deadline)
	probe.query, probe.limit = query, limit
	if probe.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return []WorkflowExecution{{WorkflowID: "workflow-1", Status: "running"}}, nil
}

func TestWorkflowListBoundsTemporalDeadlineAndKeepsQuery(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	const ownerID = "workflow-list-owner"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active')`, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'hash','revision-1')`, ownerID); err != nil {
		t.Fatal(err)
	}
	probe := &workflowListDeadlineProbe{}
	app := &App{DB: repository, Workflows: probe}
	result, err := app.WorkflowList(ctx, ownerID, "WorkflowType='test'")
	if err != nil || len(result) != 1 || probe.query != "WorkflowType='test'" || probe.limit != 200 {
		t.Fatalf("workflow list contract changed: result=%#v query=%q limit=%d err=%v", result, probe.query, probe.limit, err)
	}
	if probe.deadline.IsZero() || probe.remaining <= 0 || probe.remaining > 5*time.Second {
		t.Fatalf("Temporal list has no bounded deadline: deadline=%s remaining=%s", probe.deadline, probe.remaining)
	}
	probe.block = true
	started := time.Now()
	_, err = app.WorkflowList(ctx, ownerID, "")
	elapsed := time.Since(started)
	if !errors.Is(err, ErrWorkflowRuntime) || elapsed > 6*time.Second {
		t.Fatalf("blocked Temporal list did not fail within the bound: elapsed=%s err=%v", elapsed, err)
	}
}

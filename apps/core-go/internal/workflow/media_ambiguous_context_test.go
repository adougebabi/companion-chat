package workflow

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
)

type ambiguousContextService struct {
	ApplicationService
	cause           error
	code            string
	retryable       bool
	calls, failures atomic.Int32
}

func (s *ambiguousContextService) ProcessMediaIntent(context.Context, string) (map[string]any, error) {
	count := s.calls.Add(1)
	if s.retryable && count > 1 {
		return map[string]any{"status": "ready"}, nil
	}
	if s.code == "" {
		return nil, s.cause
	}
	return nil, fmt.Errorf("controlled submission: %w", &core.CapabilityError{Code: s.code, Retryable: s.retryable, Cause: s.cause})
}
func (s *ambiguousContextService) RecordMediaActivityFailure(context.Context, string, string, string) error {
	s.failures.Add(1)
	return nil
}

func TestAmbiguousMediaContextCauseRemainsNonRetryable(t *testing.T) {
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		t.Run(cause.Error(), func(t *testing.T) {
			original := fmt.Errorf("outer: %w", &core.CapabilityError{Code: "media_submission_ambiguous", Cause: fmt.Errorf("inner: %w", cause)})
			converted := mediaActivityFailure(original)
			var failure *temporal.ApplicationError
			if !errors.As(converted, &failure) || !failure.NonRetryable() || failure.Type() != "media_submission_ambiguous" || !errors.Is(converted, cause) {
				t.Fatalf("lost classification/cause: %v", converted)
			}
			service := &ambiguousContextService{cause: cause, code: "media_submission_ambiguous"}
			previous := app()
			Configure(service)
			defer Configure(previous)
			var suite testsuite.WorkflowTestSuite
			env := suite.NewTestWorkflowEnvironment()
			env.RegisterActivity(ProcessMediaActivity)
			env.ExecuteWorkflow(MediaWorkflow, Input{IntentID: "ambiguous-context"})
			if env.GetWorkflowError() == nil || service.calls.Load() != 1 || service.failures.Load() != 1 {
				t.Fatalf("attempts=%d terminal=%d err=%v", service.calls.Load(), service.failures.Load(), env.GetWorkflowError())
			}
		})
	}
}

// Real task-owned Temporal worker, controlled domain service. This proves SDK
// serialization/retry behavior; durable media state is covered separately by
// TestM2RealTemporalMediaUnknownSubmissionPauses against disposable PostgreSQL.
func TestRealTemporalAmbiguousContextDoesNotRetry(t *testing.T) {
	address := os.Getenv("FLUCTLIGHT_TEMPORAL_TEST_ADDRESS")
	if address == "" {
		t.Skip("task-owned Temporal address required")
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		t.Fatal("task-owned loopback Temporal required")
	}
	c, err := client.Dial(client.Options{HostPort: address, Namespace: "default"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	for _, sample := range []struct {
		name            string
		cause           error
		code            string
		retryable       bool
		calls, failures int32
	}{
		{"ambiguous-cancel", context.Canceled, "media_submission_ambiguous", false, 1, 1},
		{"ambiguous-deadline", context.DeadlineExceeded, "media_submission_ambiguous", false, 1, 1},
		// Returning context.Canceled from a still-live activity is retried by
		// this SDK/server pair, unlike cancellation of the parent workflow.
		// Preserve that pre-existing behavior; only classified ambiguity changes.
		{"ordinary-returned-cancel", context.Canceled, "", false, 3, 1},
		{"safe-transport-retry", errors.New("controlled transport failure"), "provider_request_failed", true, 2, 0},
	} {
		t.Run(sample.name, func(t *testing.T) {
			service := &ambiguousContextService{cause: sample.cause, code: sample.code, retryable: sample.retryable}
			previous := app()
			Configure(service)
			defer Configure(previous)
			queue := fmt.Sprintf("firstbatch-context-%s-%d", sample.name, time.Now().UnixNano())
			w := worker.New(c, queue, worker.Options{})
			w.RegisterWorkflow(MediaWorkflow)
			w.RegisterActivity(ProcessMediaActivity)
			if err := w.Start(); err != nil {
				t.Fatal(err)
			}
			defer w.Stop()
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
			defer cancel()
			run, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: queue, TaskQueue: queue}, MediaWorkflow, Input{IntentID: queue})
			if err != nil {
				t.Fatal(err)
			}
			err = run.Get(ctx, nil)
			if sample.retryable && err != nil || !sample.retryable && err == nil {
				t.Fatalf("unexpected outcome: %v", err)
			}
			if sample.code == "media_submission_ambiguous" {
				var failure *temporal.ApplicationError
				if !errors.As(err, &failure) || !failure.NonRetryable() || failure.Type() != sample.code {
					t.Fatalf("wrong serialized error: %v", err)
				}
			}
			if service.calls.Load() != sample.calls || service.failures.Load() != sample.failures {
				t.Fatalf("calls=%d terminal=%d err=%v", service.calls.Load(), service.failures.Load(), err)
			}
			t.Logf("REAL_TEMPORAL_CONTEXT case=%s attempts=%d terminal_records=%d", sample.name, service.calls.Load(), service.failures.Load())
		})
	}
}

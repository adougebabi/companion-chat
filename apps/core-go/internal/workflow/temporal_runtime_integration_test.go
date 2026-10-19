package workflow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// Opt in only against a task-owned Temporal environment. The dedicated queue
// registers only the real control workflow; no domain dispatcher, Provider,
// media worker or model task can be started by this test.
func TestTemporalRuntimeRealControlRecovery(t *testing.T) {
	address := os.Getenv("FLUCTLIGHT_TEMPORAL_TEST_ADDRESS")
	if address == "" {
		t.Skip("task-owned FLUCTLIGHT_TEMPORAL_TEST_ADDRESS is required")
	}
	namespace := os.Getenv("FLUCTLIGHT_TEMPORAL_TEST_NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	c, err := client.Dial(client.Options{HostPort: address, Namespace: namespace})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	id := fmt.Sprintf("v3-nonllm-control-%d", time.Now().UnixNano())
	queue := id + "-only"
	startWorker := func() worker.Worker {
		w := worker.New(c, queue, worker.Options{})
		w.RegisterWorkflow(PlatformControlWorkflow)
		if err := w.Start(); err != nil {
			t.Fatal(err)
		}
		return w
	}
	w := startWorker()
	defer func() { w.Stop() }()
	runtime := NewTemporalRuntime(c, namespace, "v3-nonllm-regression")
	for _, workflowID := range []string{id, id + "-restart"} {
		defer func(wf string) {
			cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
			defer stop()
			_ = runtime.Terminate(cleanup, wf, "", "task-owned regression cleanup", "cleanup")
		}(workflowID)
	}
	waitStatus := func(wf, run, want string) core.WorkflowExecution {
		t.Helper()
		deadline := time.Now().Add(15 * time.Second)
		var last core.WorkflowExecution
		var lastErr error
		for time.Now().Before(deadline) && ctx.Err() == nil {
			last, lastErr = runtime.Status(ctx, wf, run)
			if lastErr == nil && last.Status == want {
				return last
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("workflow %s/%s did not reach %s: %#v %v", wf, run, want, last, lastErr)
		return last
	}
	spec := core.WorkflowStart{WorkflowID: id, TaskQueue: queue, IntentType: "platform.control", OwnerCommandID: id + ":start", Payload: []byte(`{"intent_id":"isolated-control-only"}`)}
	first, err := runtime.Restart(ctx, spec)
	if err != nil || first.RunID == "" || first.OwnerCommandID != spec.OwnerCommandID {
		t.Fatal("real start/command memo", first, err)
	}
	for cycle := 1; cycle <= 2; cycle++ {
		if err := runtime.Signal(ctx, id, first.RunID, "pause", fmt.Sprint("pause-", cycle)); err != nil {
			t.Fatal(err)
		}
		waitStatus(id, first.RunID, "paused")
		if cycle == 1 {
			w.Stop()
			w = startWorker()
			waitStatus(id, first.RunID, "paused")
		}
		if err := runtime.Signal(ctx, id, first.RunID, "resume", fmt.Sprint("resume-", cycle)); err != nil {
			t.Fatal(err)
		}
		waitStatus(id, first.RunID, "running")
	}
	var token []byte
	var previous, resetPoint int64
	events, pages := 0, 0
	for {
		page, err := runtime.HistoryPage(ctx, id, first.RunID, token, 2)
		if err != nil || page.RunID != first.RunID {
			t.Fatal("real paged history", page, err)
		}
		pages++
		for _, event := range page.Events {
			if event.ID <= previous {
				t.Fatal("history page duplicated or reordered", previous, event)
			}
			previous = event.ID
			events++
			if event.Resettable && resetPoint == 0 {
				resetPoint = event.ID
			}
		}
		token = page.NextToken
		if len(token) == 0 {
			break
		}
	}
	if pages < 2 || events < 5 || resetPoint == 0 {
		t.Fatal("real history/reset boundary missing", pages, events, resetPoint)
	}
	if _, err := runtime.Reset(ctx, id, first.RunID, 1, "invalid start event", "bad-reset"); !errors.Is(err, core.ErrWorkflowCommandRejected) {
		t.Fatal("arbitrary reset point accepted", err)
	}
	reset, err := runtime.Reset(ctx, id, first.RunID, resetPoint, "task-owned workflow history reset", "actual-reset")
	if err != nil || reset.RunID == "" || reset.RunID == first.RunID {
		t.Fatal("real reset did not create a distinct run", reset, err)
	}
	resetReplay, err := runtime.Reset(ctx, id, first.RunID, resetPoint, "task-owned workflow history reset", "actual-reset")
	if err != nil || resetReplay.RunID != reset.RunID {
		t.Fatal("same reset command produced another run", resetReplay, err)
	}
	if err := runtime.Terminate(ctx, id, reset.RunID, "finish isolated reset case", "terminate-reset"); err != nil {
		t.Fatal(err)
	}
	waitStatus(id, reset.RunID, "terminated")
	spec.WorkflowID, spec.OwnerCommandID = id+"-restart", id+":restart-command"
	restarted, err := runtime.Restart(ctx, spec)
	if err != nil || restarted.RunID == "" {
		t.Fatal("real restart", restarted, err)
	}
	replay, err := runtime.Restart(ctx, spec)
	if err != nil || replay.RunID != restarted.RunID {
		t.Fatal("same restart command duplicated execution", replay, err)
	}
	spec.OwnerCommandID += ":different"
	if _, err := runtime.Restart(ctx, spec); !errors.Is(err, core.ErrWorkflowCommandRejected) {
		t.Fatal("different command recovered another Owner's run", err)
	}
	if err := runtime.Cancel(ctx, spec.WorkflowID, restarted.RunID, "cancel-restart"); err != nil {
		t.Fatal(err)
	}
	waitStatus(spec.WorkflowID, restarted.RunID, "canceled")
	t.Logf("TEMPORAL_REAL_CONTROL_EVIDENCE workflow=%s first_run=%s reset_run=%s restart_run=%s history_events=%d history_pages=%d reset_point=%d worker_restart=true repeated_pause_resume=true models=0", id, first.RunID, reset.RunID, restarted.RunID, events, pages, resetPoint)
}

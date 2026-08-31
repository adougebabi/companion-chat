package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	enumspb "go.temporal.io/api/enums/v1"
	workflowapi "go.temporal.io/api/workflow/v1"
	workflowservice "go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
)

// This one-shot command fences executions owned by the retired runtime. It is
// deliberately opt-in: listing is safe, and --apply is required to send
// cancellation requests. Domain facts remain in PostgreSQL; no workflow
// history is replayed by the Go Worker.
func main() {
	apply := flag.Bool("apply", false, "cancel legacy executions")
	flag.Parse()
	address := firstEnv("TEMPORAL_ADDRESS", "temporal:7233")
	namespace := firstEnv("TEMPORAL_NAMESPACE", "default")
	databaseURL := strings.TrimSpace(os.Getenv("CORE_GO_DATABASE_URL"))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	temporalClient, err := client.Dial(client.Options{HostPort: address, Namespace: namespace})
	if err != nil {
		log.Fatal(err)
	}
	defer temporalClient.Close()
	var audit *pgxpool.Pool
	if databaseURL != "" {
		audit, err = pgxpool.New(ctx, databaseURL)
		if err != nil {
			log.Fatal(err)
		}
		defer audit.Close()
	}
	if *apply && audit == nil {
		log.Fatal("CORE_GO_DATABASE_URL is required when --apply is set so every fence outcome is audited")
	}
	var pageToken []byte
	for {
		response, listErr := temporalClient.ListWorkflow(ctx, &workflowservice.ListWorkflowExecutionsRequest{
			Query:         `ExecutionStatus="Running"`,
			NextPageToken: pageToken,
		})
		if listErr != nil {
			log.Fatal(listErr)
		}
		for _, info := range response.Executions {
			if info == nil || info.Execution == nil || info.Status != enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING || !legacy(info) {
				continue
			}
			id, runID, typ, queue := info.Execution.GetWorkflowId(), info.Execution.GetRunId(), info.Type.GetName(), info.TaskQueue
			if !*apply {
				fmt.Printf("legacy execution: workflow_id=%s run_id=%s type=%s queue=%s\n", id, runID, typ, queue)
				continue
			}
			result := fenceWorkflow(ctx, temporalClient, id, runID)
			resultJSON, marshalErr := json.Marshal(result)
			if marshalErr != nil {
				log.Fatal(marshalErr)
			}
			resultText := string(resultJSON)
			fmt.Printf("legacy execution: workflow_id=%s run_id=%s type=%s queue=%s result=%s\n", id, runID, typ, queue, resultText)
			_, auditErr := audit.Exec(ctx, `INSERT INTO public.platform_workflow_management_audit(id,action,workflow_id,actor_id,authorized,details) VALUES($1,'legacy_fence',$2,'migration','true',$3) ON CONFLICT(id) DO UPDATE SET details=EXCLUDED.details,created_at=now()`, "cutover_"+strings.ReplaceAll(runID, "-", ""), id, resultJSON)
			if auditErr != nil {
				log.Fatalf("recording legacy fence for %s: %v", id, auditErr)
			}
		}
		pageToken = response.NextPageToken
		if len(pageToken) == 0 {
			break
		}
	}
}

func fenceWorkflow(ctx context.Context, temporalClient client.Client, workflowID, runID string) map[string]any {
	result := map[string]any{"requested": false, "terminal": false}
	if err := temporalClient.CancelWorkflow(ctx, workflowID, runID); err != nil {
		result["error"] = err.Error()
		return result
	}
	result["requested"] = true
	// A graceful cancel needs a workflow task poller to observe the request.
	// During a cutover the retired worker is intentionally gone, so bound the
	// grace period and terminate the old execution if it cannot acknowledge.
	graceCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	status, waitErr := waitForTerminal(graceCtx, temporalClient, workflowID, runID)
	cancel()
	if waitErr == nil && status != enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING {
		result["status"] = status.String()
		result["terminal"] = true
		return result
	}
	if waitErr != nil {
		result["grace_error"] = waitErr.Error()
	}
	if err := temporalClient.TerminateWorkflow(ctx, workflowID, runID, "Go Core cutover fence"); err != nil {
		result["error"] = err.Error()
		return result
	}
	result["terminated"] = true
	terminalCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	status, waitErr = waitForTerminal(terminalCtx, temporalClient, workflowID, runID)
	cancel()
	result["status"] = status.String()
	result["terminal"] = waitErr == nil && status != enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING
	if waitErr != nil {
		result["error"] = waitErr.Error()
	}
	return result
}

func legacy(info *workflowapi.WorkflowExecutionInfo) bool {
	if info == nil || info.Execution == nil {
		return false
	}
	// Go cutover executions are explicitly namespaced. Any execution with a
	// non-Go ID is eligible for fencing, including executions on the canonical
	// interaction/lifecycle/media queues that the retired Python worker used.
	if strings.HasPrefix(info.Execution.GetWorkflowId(), "go:") {
		return false
	}
	name := info.Type.GetName()
	return strings.Contains(name, "DailyLifeReview") || strings.Contains(name, "CurrentDaySchedule") || strings.Contains(name, "PlatformControl") || strings.Contains(name, "Cognition") || strings.Contains(name, "Autonomy") || strings.Contains(name, "MediaGeneration") || strings.Contains(name, "Reflection") || strings.Contains(name, "MemoryEmbedding") || name == "MediaWorkflow" || name == "DailyReviewWorkflow"
}

func firstEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
func waitForTerminal(ctx context.Context, temporalClient client.Client, workflowID, runID string) (enumspb.WorkflowExecutionStatus, error) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		description, err := temporalClient.DescribeWorkflowExecution(ctx, workflowID, runID)
		if err != nil {
			return enumspb.WORKFLOW_EXECUTION_STATUS_UNSPECIFIED, err
		}
		status := description.WorkflowExecutionInfo.GetStatus()
		if status != enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING {
			return status, nil
		}
		select {
		case <-ctx.Done():
			return status, ctx.Err()
		case <-ticker.C:
		}
	}
}

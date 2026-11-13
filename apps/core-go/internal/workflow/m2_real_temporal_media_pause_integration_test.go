package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
)

// This opt-in scenario uses a task-owned Temporal server and disposable
// PostgreSQL database. The persisted fence represents a submission whose
// response may have been lost; no real renderer or model is contacted.
func TestM2RealTemporalMediaUnknownSubmissionPauses(t *testing.T) {
	if os.Getenv("FLUCTLIGHT_TEMPORAL_MEDIA_TEST") != "1" {
		t.Skip("set FLUCTLIGHT_TEMPORAL_MEDIA_TEST=1 for task-owned Temporal media verification")
	}
	address := strings.TrimSpace(os.Getenv("FLUCTLIGHT_TEMPORAL_TEST_ADDRESS"))
	host, _, err := net.SplitHostPort(address)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		t.Fatal("Temporal test address must be an explicit loopback IP and port")
	}
	databaseURL, err := url.Parse(os.Getenv("GO_CORE_TEST_DATABASE_URL"))
	if err != nil || databaseURL.Hostname() != "127.0.0.1" {
		t.Fatal("PostgreSQL test address must be loopback")
	}
	ctx, application := scheduleRecoveryRepository(t)
	var rendererRequests atomic.Int32
	renderer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		rendererRequests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer renderer.Close()
	config, err := json.Marshal(map[string]any{"baseUrl": renderer.URL, "workflow": map[string]any{"prompt": "{{prompt}}"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := application.DB.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('media.comfyui',$1) ON CONFLICT(key) DO UPDATE SET value_json=EXCLUDED.value_json`, string(config)); err != nil {
		t.Fatal(err)
	}
	application.Provider = &core.ProviderClient{DB: application.DB, HTTP: renderer.Client()}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	mediaID, workflowID, queue := "m2-temporal-media-"+suffix, "m2-temporal-workflow-"+suffix, "m2-temporal-queue-"+suffix
	if _, err := application.DB.Pool().Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,status,renderer_submission_started) VALUES($1,'retry-role','image','image/png','{}',$2,$3,'submitting',true)`, mediaID, "m2-temporal-request-"+suffix, workflowID); err != nil {
		t.Fatal(err)
	}
	c, err := client.Dial(client.Options{HostPort: address, Namespace: "default"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	Configure(application)
	defer Configure(nil)
	w := worker.New(c, queue, worker.Options{})
	w.RegisterWorkflow(MediaWorkflow)
	w.RegisterActivity(ProcessMediaActivity)
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()
	run, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: workflowID, TaskQueue: queue}, MediaWorkflow, Input{IntentID: mediaID})
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	err = run.Get(ctx, &result)
	var failure *temporal.ApplicationError
	if !errors.As(err, &failure) || failure.Type() != "media_submission_ambiguous" || !failure.NonRetryable() {
		t.Fatalf("unknown submission did not stop with non-retryable error: result=%#v err=%v", result, err)
	}
	var status, providerJobID string
	var marked bool
	var revision int
	if err := application.DB.Pool().QueryRow(ctx, `SELECT status,renderer_submission_started,COALESCE(provider_job_id,''),revision FROM public.media_intents WHERE id=$1`, mediaID).Scan(&status, &marked, &providerJobID, &revision); err != nil || status != "submission_unknown" || !marked || providerJobID != "" {
		t.Fatalf("uncertain media state status=%s marked=%t job=%q err=%v", status, marked, providerJobID, err)
	}
	iterator := c.GetWorkflowHistory(ctx, workflowID, run.GetRunID(), false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
	scheduled, started := 0, 0
	var startedAttempt int32
	for iterator.HasNext() {
		event, err := iterator.Next()
		if err != nil {
			t.Fatal(err)
		}
		switch event.GetEventType() {
		case enumspb.EVENT_TYPE_ACTIVITY_TASK_SCHEDULED:
			scheduled++
		case enumspb.EVENT_TYPE_ACTIVITY_TASK_STARTED:
			started++
			startedAttempt = event.GetActivityTaskStartedEventAttributes().GetAttempt()
		}
	}
	if scheduled != 1 || started != 1 || startedAttempt != 1 || rendererRequests.Load() != 0 {
		t.Fatalf("unexpected retry or renderer call: scheduled=%d started=%d attempt=%d renderer=%d", scheduled, started, startedAttempt, rendererRequests.Load())
	}
	_, replayErr := application.ProcessMediaIntent(ctx, mediaID)
	var capabilityErr *core.CapabilityError
	if !errors.As(replayErr, &capabilityErr) || capabilityErr.Code != "media_submission_ambiguous" || capabilityErr.Retryable || rendererRequests.Load() != 0 {
		t.Fatalf("unreviewed reprocessing did not remain paused: err=%v renderer=%d", replayErr, rendererRequests.Load())
	}
	for _, mode := range []string{"resume", "regenerate"} {
		_, recoveryErr := application.RetryOwnerDiagnosticMedia(ctx, "retry-owner", mediaID, map[string]any{"expected_revision": revision, "mode": mode, "idempotency_key": "m2-temporal-unreviewed-" + mode, "reason": "隔离测试中的未核对请求"})
		var surface *core.SurfaceError
		if !errors.As(recoveryErr, &surface) || surface.Code != "media_submission_review_required" {
			t.Fatalf("unreviewed %s was not rejected: %v", mode, recoveryErr)
		}
	}
	var recoveryAttempts int
	if err := application.DB.Pool().QueryRow(ctx, `SELECT count(*) FROM public.media_recovery_attempts WHERE media_intent_id=$1`, mediaID).Scan(&recoveryAttempts); err != nil || recoveryAttempts != 0 || rendererRequests.Load() != 0 {
		t.Fatalf("unreviewed recovery changed state: attempts=%d renderer=%d err=%v", recoveryAttempts, rendererRequests.Load(), err)
	}
	t.Logf("REAL_TEMPORAL_MEDIA_PAUSE status=%s scheduled=%d started=%d attempt=%d renderer_requests=%d recovery_attempts=%d workflow=%s", status, scheduled, started, startedAttempt, rendererRequests.Load(), recoveryAttempts, workflowID)
}

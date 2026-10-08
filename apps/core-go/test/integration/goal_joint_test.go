package integration_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/httpapi"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/migrations"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/platform"
	coreworkflow "github.com/fluctlight/local-ai-companion/apps/core-go/internal/workflow"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func TestGoalJointPostgresRedisTemporalWorkerRestart(t *testing.T) {
	databaseURL, temporalAddr, redisAddr := os.Getenv("GO_CORE_TEST_DATABASE_URL"), os.Getenv("GO_GOAL_TEST_TEMPORAL_ADDR"), os.Getenv("GO_GOAL_TEST_REDIS_ADDR")
	if databaseURL == "" || temporalAddr == "" || redisAddr == "" {
		t.Skip("task-owned PostgreSQL, Redis and Temporal endpoints are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	adminConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	adminConfig.ConnConfig.Database = "postgres"
	admin, err := pgxpool.NewWithConfig(ctx, adminConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	dbName := fmt.Sprintf("goal_joint_%d", time.Now().UnixNano())
	identifier := pgx.Identifier{dbName}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+identifier); err != nil {
		t.Fatal(err)
	}
	parsed.Path = "/" + dbName
	repo, err := core.NewPostgresRepository(ctx, parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		repo.Close()
		cleanup, stop := context.WithTimeout(context.Background(), 15*time.Second)
		defer stop()
		_, err := admin.Exec(cleanup, "DROP DATABASE "+identifier+" WITH (FORCE)")
		if err != nil {
			t.Error(err)
		}
	})
	if err := migrations.New(repo.Pool()).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	owner, fluctlightID := "goal-joint-owner-"+dbName, "goal-joint-self-"+dbName
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active')`, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'fixture','fixture')`, owner); err != nil {
		t.Fatal(err)
	}
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	app, err := core.NewApp(repo, key, "goal-joint-service", "127.0.0.1:9000", "test", "test", "test", "test", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.CreateFluctlight(ctx, owner, fluctlightID, "摇光", "blank_slate", "", nil, nil, nil, "Asia/Shanghai"); err != nil {
		t.Fatal(err)
	}
	conversationID, err := app.EnsureDirectConversation(ctx, owner, fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	desired, motivation := "实际明确表达自己的心意", "当前真实表达"
	created, err := app.ApplyOwnerGoalCommand(ctx, owner, fluctlightID, "", core.GoalOwnerCommand{Operation: "create", IdempotencyKey: "joint-goal", Reason: "联合验收", DesiredOutcome: &desired, Motivation: &motivation, SuccessCriteria: []string{"已实际表达"}})
	if err != nil {
		t.Fatal(err)
	}
	goalID := created["goal_id"].(string)
	// Exercise the actual authenticated HTTP boundary against the same authority.
	sessionToken := "joint-owner-session"
	hash := sha256.Sum256([]byte(sessionToken))
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.auth_sessions(id,token_hash,human_actor_id,expires_at) VALUES('joint-session',$1,$2,now()+interval '1 hour')`, hex.EncodeToString(hash[:]), owner); err != nil {
		t.Fatal(err)
	}
	api := httptest.NewServer(httpapi.NewApp(app, "goal-joint-service", slog.New(slog.NewTextHandler(io.Discard, nil))).Handler())
	defer api.Close()
	for _, step := range []struct {
		operation string
		revision  int
	}{{"pause", 1}, {"resume", 2}} {
		payload, _ := json.Marshal(core.GoalOwnerCommand{ExpectedRevision: step.revision, IdempotencyKey: "http-" + step.operation, Reason: "实际 HTTP 治理"})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, api.URL+"/internal/fluctlights/"+fluctlightID+"/goals/"+goalID+"/"+step.operation, bytes.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Fluctlight-Service-Key", "goal-joint-service")
		req.Header.Set("X-Fluctlight-Human-Session", sessionToken)
		resp, err := api.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("actual Owner HTTP %s status=%d body=%s", step.operation, resp.StatusCode, data)
		}
	}
	unauth, err := http.NewRequestWithContext(ctx, http.MethodGet, api.URL+"/internal/fluctlights/"+fluctlightID+"/goals/"+goalID, nil)
	if err != nil {
		t.Fatal(err)
	}
	unauth.Header.Set("X-Fluctlight-Service-Key", "goal-joint-service")
	response, err := api.Client().Do(unauth)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 401 {
		t.Fatalf("HTTP missing session allowed %d", response.StatusCode)
	}

	receipt, err := app.ExecuteTool(ctx, core.ToolExecutionRequest{CapabilityName: "conversation.reply", OperationID: "joint-expression", AuthorizationActorID: owner, FluctlightID: fluctlightID, ConversationID: conversationID, Surface: core.CapabilitySurfaceConversation, Arguments: []byte(`{"text":"我喜欢你，这是我清楚表达的心意。"}`)})
	if err != nil || receipt.Result.Status != "completed" {
		t.Fatalf("actual message %#v %v", receipt, err)
	}
	var requestID string
	if err := repo.Pool().QueryRow(ctx, `SELECT id FROM public.goal_evaluation_requests WHERE fluctlight_id=$1 AND status='pending'`, fluctlightID).Scan(&requestID); err != nil {
		t.Fatal(err)
	}
	// Only this Goal workflow participates in the isolated restart scenario.
	if _, err := repo.Pool().Exec(ctx, `DELETE FROM public.platform_workflow_intents WHERE intent_type<>'goal.evaluate'`); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	firstFailure := make(chan struct{}, 1)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if calls.Add(1) == 1 {
			firstFailure <- struct{}{}
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`{"error":{"message":"controlled temporary assessment failure"}}`))
			return
		}
		var wire map[string]any
		if err := json.NewDecoder(r.Body).Decode(&wire); err != nil {
			t.Error(err)
			w.WriteHeader(500)
			return
		}
		messages, _ := wire["messages"].([]any)
		var input map[string]any
		for i := len(messages) - 1; i >= 0; i-- {
			m, _ := messages[i].(map[string]any)
			if m["role"] == "user" {
				text, _ := m["content"].(string)
				_ = json.Unmarshal([]byte(text), &input)
				break
			}
		}
		goals, _ := input["goals"].([]any)
		sources, _ := input["sources"].([]any)
		if len(goals) != 1 {
			t.Errorf("assessment omitted original Goal: %#v", input)
			w.WriteHeader(500)
			return
		}
		g := goals[0].(map[string]any)
		criteria := g["criteria"].([]any)
		criterion := criteria[0].(map[string]any)
		proof := ""
		for _, value := range sources {
			s := value.(map[string]any)
			if s["kind"] == "message" && s["subject_actor_id"] == fluctlightID {
				proof, _ = s["ref"].(string)
			}
		}
		if proof == "" {
			t.Error("durable actual message disappeared after restart")
			w.WriteHeader(500)
			return
		}
		output := map[string]any{"evaluations": []any{map[string]any{"goal_id": g["goal_id"], "expected_revision": g["revision"], "criteria_version": g["criteria_version"], "judgments": []any{map[string]any{"criterion_id": criterion["id"], "verdict": "satisfied", "kind": "communication", "subject": "actor_self", "discourse": "assertion", "evidence_refs": []string{proof}, "reason": "真实已落库表达"}}, "impact": "completed", "blocker": "", "wait_condition": "", "next_step": "", "residual_motivation": ""}}, "plans": []any{}}
		encoded, _ := json.Marshal(output)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": string(encoded)}}}})
	}))
	defer provider.Close()
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status) VALUES('joint-provider','openai_compatible',$1,'fixture-unused','ready')`, provider.URL); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('cognitive_assessment','joint-provider','scripted','structured_output',4096,15,'{}')`); err != nil {
		t.Fatal(err)
	}
	temporalClient, err := client.Dial(client.Options{HostPort: temporalAddr, Namespace: "default"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(temporalClient.Close)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	coreworkflow.Configure(app)
	workers, _, err := coreworkflow.StartWorkers(ctx, temporalClient, logger)
	if err != nil {
		t.Fatal(err)
	}
	stopped := false
	stopWorkers := func(ws []worker.Worker) {
		for _, w := range ws {
			w.Stop()
		}
	}
	t.Cleanup(func() {
		if !stopped {
			stopWorkers(workers)
		}
	})
	if err := coreworkflow.EnsureWorkerDeploymentCurrentVersion(ctx, temporalClient.WorkerDeploymentClient().GetHandle(coreworkflow.WorkerDeploymentName), coreworkflow.WorkerDeploymentBuildID()); err != nil {
		t.Fatal(err)
	}
	dispatcher := &coreworkflow.Dispatcher{App: app, Client: temporalClient, Started: map[string]struct{}{}}
	awaitGoalJoint(t, ctx, func() bool {
		count, err := dispatcher.DispatchOnce(ctx, 10)
		if err != nil {
			t.Fatal(err)
		}
		return count > 0
	})
	select {
	case <-firstFailure:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	awaitGoalJoint(t, ctx, func() bool {
		var status string
		_ = repo.Pool().QueryRow(ctx, `SELECT status FROM public.goal_evaluation_requests WHERE id=$1`, requestID).Scan(&status)
		return status == "retry"
	})
	stopWorkers(workers)
	stopped = true
	var state string
	if err := repo.Pool().QueryRow(ctx, `SELECT status FROM public.fluctlight_goals WHERE id=$1`, goalID).Scan(&state); err != nil || state != "active" {
		t.Fatalf("failure fabricated completion %s %v", state, err)
	}
	// Advance only the isolated retry timer; no business operation is reissued.
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.goal_evaluation_requests SET available_at=now() WHERE id=$1`, requestID); err != nil {
		t.Fatal(err)
	}
	newApp, err := core.NewApp(repo, key, "goal-joint-service", "127.0.0.1:9000", "test", "test", "test", "test", false)
	if err != nil {
		t.Fatal(err)
	}
	coreworkflow.Configure(newApp)
	workers, _, err = coreworkflow.StartWorkers(ctx, temporalClient, logger)
	if err != nil {
		t.Fatal(err)
	}
	stopped = false
	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer redisClient.Close()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	publisher := platform.NewOutboxPublisher(repo.Pool(), redisClient, "goal-joint-publisher")
	publisher.Stream = "goal-joint-events:" + dbName
	if _, err := publisher.PublishOnce(ctx, 100); err != nil {
		t.Fatal(err)
	}
	awaitGoalJoint(t, ctx, func() bool {
		var status string
		_ = repo.Pool().QueryRow(ctx, `SELECT status FROM public.fluctlight_goals WHERE id=$1`, goalID).Scan(&status)
		return status == "completed"
	})
	if _, err := publisher.PublishOnce(ctx, 100); err != nil {
		t.Fatal(err)
	}
	consumer := platform.NewEventConsumer(repo.Pool(), redisClient, "goal-joint-group", "restart-consumer")
	consumer.Stream = publisher.Stream
	if _, err := consumer.ConsumeOnce(ctx, 100); err != nil {
		t.Fatal(err)
	}
	if _, err := consumer.ConsumeOnce(ctx, 100); err != nil {
		t.Fatal(err)
	}
	var resolutions, attempts, messages int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.goal_resolutions WHERE goal_id=$1`, goalID).Scan(&resolutions); err != nil {
		t.Fatal(err)
	}
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_intention_attempts WHERE fluctlight_id=$1`, fluctlightID).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversationID).Scan(&messages); err != nil {
		t.Fatal(err)
	}
	if resolutions != 1 || attempts != 0 || messages != 1 || calls.Load() != 2 {
		t.Fatalf("restart duplicated effects: resolutions=%d attempts=%d messages=%d assessments=%d", resolutions, attempts, messages, calls.Load())
	}
	var requestStatus string
	if err := repo.Pool().QueryRow(ctx, `SELECT status FROM public.goal_evaluation_requests WHERE id=$1`, requestID).Scan(&requestStatus); err != nil || requestStatus != "succeeded" {
		t.Fatalf("assessment not settled %s %v", requestStatus, err)
	}
	t.Logf("JOINT_EVIDENCE goal=%s request=%s Redis_stream=%s assessments=%d resolutions=%d", goalID, requestID, publisher.Stream, calls.Load(), resolutions)
}
func awaitGoalJoint(t *testing.T, ctx context.Context, predicate func() bool) {
	t.Helper()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if predicate() {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ticker.C:
		}
	}
}

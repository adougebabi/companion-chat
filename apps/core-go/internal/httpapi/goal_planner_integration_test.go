package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestGoalPlannerBrowserAPIUsesRealAuthorityAndVersions(t *testing.T) {
	databaseURL := os.Getenv("GO_CORE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GO_CORE_TEST_DATABASE_URL required; run with task-owned test PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.Database = "postgres"
	admin, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprint("lac_goal_api_", time.Now().UnixNano())
	quoted := pgx.Identifier{name}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+quoted); err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrations.New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	testURL, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	testURL.Path = "/" + name
	repo, err := core.NewPostgresRepository(ctx, testURL.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		repo.Close()
		pool.Close()
		cleanup, c := context.WithTimeout(context.Background(), 20*time.Second)
		defer c()
		if _, err := admin.Exec(cleanup, "DROP DATABASE "+quoted); err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	if _, err := pool.Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES('human-1','human','active'),('goal-api-instance','fluctlight','active'); INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES('goal-api-instance','human-1','blank_slate','active','{}','{}','{}','{}','{}','{}')`); err != nil {
		t.Fatal(err)
	}
	// Only session resolution is deterministic; HTTP/BFF/App/transactions/DB are production.
	server := New(fakeRepository{}, "service-key", nil)
	server.app = &core.App{DB: repo}
	server.SetBrowserBoundary("http://fluctlight.local", false)
	handler := server.Handler()
	request := func(method, path string, body any) (int, map[string]any) {
		t.Helper()
		data, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, strings.NewReader(string(data)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "http://fluctlight.local")
		req.Header.Set("X-CSRF-Token", "csrf")
		req.AddCookie(&http.Cookie{Name: "fluctlight_session", Value: "session"})
		req.AddCookie(&http.Cookie{Name: "fluctlight_csrf", Value: "csrf"})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		var result map[string]any
		_ = json.Unmarshal(response.Body.Bytes(), &result)
		return response.Code, result
	}
	for i := 0; i < 5; i++ {
		status, result := request(http.MethodPost, "/api/fluctlights/goal-api-instance/goals", map[string]any{"desiredOutcome": fmt.Sprint("goal-", i), "successCriteria": []string{"实际结果"}, "motivation": "Owner愿望", "reason": "create", "idempotencyKey": fmt.Sprint("goal-", i)})
		if status != 200 || result["status"] != "active" {
			t.Fatal("actual goal create", status, result)
		}
	}
	status, capacityError := request(http.MethodPost, "/api/fluctlights/goal-api-instance/goals", map[string]any{"desiredOutcome": "sixth", "successCriteria": []string{"实际结果"}, "motivation": "Owner愿望", "reason": "create", "idempotencyKey": "sixth"})
	if status != 409 || capacityError["code"] != "goal_capacity_exceeded" {
		t.Fatal("HTTP must distinguish capacity from stale revision", status, capacityError)
	}
	status, snapshot := request(http.MethodGet, "/api/fluctlights/goal-api-instance/goal-set", nil)
	if status != 200 || snapshot["active_count"] != float64(5) {
		t.Fatal("API count differs from DB", status, snapshot)
	}
	status, result := request(http.MethodPut, "/api/fluctlights/goal-api-instance/goal-set", map[string]any{"expectedVersion": snapshot["revision"], "expectedFactsRevision": snapshot["facts_revision"], "idempotencyKey": "switch-off", "reason": "Owner paused automatic planning", "autoPlanningEnabled": false})
	if status != 200 || result["auto_planning_enabled"] != false {
		t.Fatal("switch failed backend", status, result)
	}
	status, actor := request(http.MethodGet, "/api/fluctlights/goal-api-instance/actor-context/human-1", nil)
	if status != 200 {
		t.Fatal(status, actor)
	}
	body := map[string]any{"background": map[string]any{"location": "国外", "timezone": "Europe/Paris"}, "expectedContextVersion": actor["context_version"], "idempotencyKey": "context", "operation": "change", "reason": "Owner updated background"}
	status, saved := request(http.MethodPut, "/api/fluctlights/goal-api-instance/actor-context/human-1", body)
	if status != 200 || saved["context_version"] == actor["context_version"] {
		t.Fatal("Actor edit not persisted", status, saved)
	}
	body["idempotencyKey"] = "old-version"
	status, _ = request(http.MethodPut, "/api/fluctlights/goal-api-instance/actor-context/human-1", body)
	if status != 409 {
		t.Fatal("stale Actor write not conflict", status)
	}
	var count int
	var auto bool
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.fluctlight_goals WHERE fluctlight_id='goal-api-instance' AND status='active'),auto_planning_enabled FROM public.goal_set_policies WHERE fluctlight_id='goal-api-instance'`).Scan(&count, &auto); err != nil || count != 5 || auto {
		t.Fatal("HTTP effect differs from DB", count, auto, err)
	}
}

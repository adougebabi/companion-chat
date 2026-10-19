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

func TestAboutYouCompleteFieldsBrowserUsesRealAuthority(t *testing.T) {
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

	background := map[string]any{"name": "称呼", "full_name": "本人姓名", "age": nil, "gender": nil, "occupation": "工程师", "background": "背景", "interests": "绘画", "preferences": "明确偏好", "location_scope": "国外", "location": nil, "timezone": "Europe/Paris", "relationship_distance": "线上交流", "meeting_confirmed": false, "personality": "慢热", "habits": "每天读书", "relationship_description": "我把她当朋友", "shared_information": "愿意分享的信息", "wants_you_to_know": "最近忙于写作", "rules": "先询问\n再给建议"}
	status, initial := request(http.MethodGet, "/api/fluctlights/goal-api-instance/actor-context/human-1", nil)
	if status != 200 {
		t.Fatal(status, initial)
	}
	body := map[string]any{"background": background, "expectedContextVersion": initial["context_version"], "idempotencyKey": "complete-context", "operation": "correct", "reason": "本人自填资料"}
	status, saved := request(http.MethodPut, "/api/fluctlights/goal-api-instance/actor-context/human-1", body)
	if status != 200 {
		t.Fatal("all fields rejected by real Actor route", status, saved)
	}
	expected, _ := json.Marshal(background)
	actual, _ := json.Marshal(saved["background"])
	if string(expected) != string(actual) {
		t.Fatal("field loss", string(actual))
	}
	status, revision := request(http.MethodGet, "/api/fluctlights/goal-api-instance/goal-set", nil)
	if status != 200 {
		t.Fatal(status, revision)
	}
	ownerBody := map[string]any{"background": background, "expectedCurrentFactsRevision": revision["facts_revision"], "idempotencyKey": "complete-owner", "operation": "correct", "reason": strings.Repeat("明", 500)}
	status, owner := request(http.MethodPut, "/api/fluctlights/goal-api-instance/actor-user-background", ownerBody)
	if status != 200 {
		t.Fatal("all fields rejected by real Owner route", status, owner)
	}
	actual, _ = json.Marshal(owner["background"])
	if string(expected) != string(actual) {
		t.Fatal("Owner field loss", string(actual))
	}
	status, reread := request(http.MethodGet, "/api/fluctlights/goal-api-instance/actor-context/human-1", nil)
	actual, _ = json.Marshal(reread["background"])
	if status != 200 || string(expected) != string(actual) {
		t.Fatal("requery", status, reread)
	}
	body["idempotencyKey"] = "stale"
	status, _ = request(http.MethodPut, "/api/fluctlights/goal-api-instance/actor-context/human-1", body)
	if status != 409 {
		t.Fatal("stale draft did not conflict", status)
	}
	background["forged"] = nil
	ownerBody["idempotencyKey"] = "unknown-field"
	status, _ = request(http.MethodPut, "/api/fluctlights/goal-api-instance/actor-user-background", ownerBody)
	if status != 400 {
		t.Fatal("unknown field allowed", status)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.actor_facts WHERE owner_fluctlight_id='goal-api-instance' AND subject_actor_id='human-1' AND status='active'`).Scan(&count); err != nil || count != 19 {
		t.Fatal("actual fact count", count, err)
	}
	t.Log("Both authenticated public HTTP routes persisted all 19 fields, explicit null/false and newline rules through real in-process Core/PostgreSQL; stale request=409, unknown key=400")
}

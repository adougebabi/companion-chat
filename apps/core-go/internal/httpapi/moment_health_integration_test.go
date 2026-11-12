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

func momentHealthDisposableDatabaseURL(databaseURL, name string) (string, error) {
	testURL, err := url.Parse(databaseURL)
	if err != nil {
		return "", err
	}
	testURL.Path = "/" + name
	query := testURL.Query()
	query.Del("database")
	query.Set("dbname", name)
	testURL.RawQuery = query.Encode()
	parsed, err := pgxpool.ParseConfig(testURL.String())
	if err != nil {
		return "", err
	}
	if parsed.ConnConfig.Database != name {
		return "", fmt.Errorf("disposable database configuration does not select fixture database")
	}
	return testURL.String(), nil
}

func TestMomentHealthDisposableDatabaseURLOverridesDatabaseParameters(t *testing.T) {
	fixtureURL, err := momentHealthDisposableDatabaseURL("postgresql://localhost/original?dbname=wrong&database=also_wrong", "lac_moment_health_fixture")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := pgxpool.ParseConfig(fixtureURL)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.ConnConfig.Database != "lac_moment_health_fixture" {
		t.Fatalf("fixture database was not selected: database=%q", parsed.ConnConfig.Database)
	}
}

func TestMomentHealthBrowserRouteUsesSavedDatabaseSettings(t *testing.T) {
	databaseURL := os.Getenv("GO_CORE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GO_CORE_TEST_DATABASE_URL required for the disposable PostgreSQL fixture")
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
	name := fmt.Sprintf("lac_moment_health_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{name}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+quoted); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	var pool *pgxpool.Pool
	var repo *core.PostgresRepository
	t.Cleanup(func() {
		if repo != nil {
			repo.Close()
		}
		if pool != nil {
			pool.Close()
		}
		cleanup, stop := context.WithTimeout(context.Background(), 20*time.Second)
		defer stop()
		if _, err := admin.Exec(cleanup, "DROP DATABASE "+quoted); err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	config.ConnConfig.Database = name
	pool, err = pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrations.New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	testURL, err := momentHealthDisposableDatabaseURL(databaseURL, name)
	if err != nil {
		t.Fatal(err)
	}
	repo, err = core.NewPostgresRepository(ctx, testURL)
	if err != nil {
		t.Fatal(err)
	}
	var actualDatabase string
	if err := repo.Pool().QueryRow(ctx, "SELECT current_database()").Scan(&actualDatabase); err != nil || actualDatabase != name {
		t.Fatalf("repository is not connected to the disposable database: database=%q err=%v", actualDatabase, err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES('human-1','human','active'); INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES('human-1','fixture','fixture')`); err != nil {
		t.Fatal(err)
	}
	server := New(conversationSessionRepository{PostgresRepository: repo}, "service-key", nil)
	server.app = &core.App{DB: repo}
	server.SetBrowserBoundary("http://fluctlight.local", false)
	handler := server.Handler()
	request := func(method, path string, body any, session bool) (int, map[string]any) {
		t.Helper()
		data, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, strings.NewReader(string(data)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "http://fluctlight.local")
		req.Header.Set("X-CSRF-Token", "csrf")
		req.AddCookie(&http.Cookie{Name: "fluctlight_csrf", Value: "csrf"})
		if session {
			req.AddCookie(&http.Cookie{Name: "fluctlight_session", Value: "session"})
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		var result map[string]any
		_ = json.Unmarshal(response.Body.Bytes(), &result)
		return response.Code, result
	}
	if status, _ := request(http.MethodPost, "/api/settings/moment/test-health", map[string]any{}, false); status != http.StatusUnauthorized {
		t.Fatalf("anonymous Moment health status=%d", status)
	}
	status, initial := request(http.MethodPost, "/api/settings/moment/test-health", map[string]any{}, true)
	if status != http.StatusOK || initial["state"] != "ready" || initial["plugin_enabled"] != true || initial["data_read_tested"] != true || initial["publication_tested"] != false || initial["model_tested"] != false || initial["config_version"] != float64(0) {
		t.Fatalf("initial health status=%d result=%#v", status, initial)
	}
	status, saved := request(http.MethodPut, "/api/settings", map[string]any{"values": map[string]any{"plugins.domains": map[string]any{"image": false, "moment": false}}, "expectedVersions": map[string]any{"plugins.domains": 0}, "idempotencyKey": "moment-health-save"}, true)
	if status != http.StatusOK {
		t.Fatalf("save plugin setting status=%d result=%#v", status, saved)
	}
	var momentsBefore, receiptsBefore int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.moments),(SELECT count(*) FROM public.tool_executions)`).Scan(&momentsBefore, &receiptsBefore); err != nil {
		t.Fatal(err)
	}
	status, checked := request(http.MethodPost, "/api/settings/moment/test-health", map[string]any{}, true)
	if status != http.StatusOK || checked["state"] != "ready" || checked["plugin_enabled"] != false || checked["config_version"] != float64(1) || checked["publication_tested"] != false || checked["model_tested"] != false {
		t.Fatalf("saved health status=%d result=%#v", status, checked)
	}
	var momentsAfter, receiptsAfter int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.moments),(SELECT count(*) FROM public.tool_executions)`).Scan(&momentsAfter, &receiptsAfter); err != nil || momentsAfter != momentsBefore || receiptsAfter != receiptsBefore {
		t.Fatalf("health check changed business data: moments=%d/%d receipts=%d/%d err=%v", momentsBefore, momentsAfter, receiptsBefore, receiptsAfter, err)
	}
}

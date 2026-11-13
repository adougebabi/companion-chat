package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Uses a fresh migrated PostgreSQL database and the real browser/Core handlers.
// The session resolver is controlled. No Provider, Temporal or media request runs.
func TestDiagnosticDebugReadBrowserAndInternalHTTP(t *testing.T) {
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
	name := fmt.Sprintf("lac_diagnostic_debug_%d", time.Now().UnixNano())
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

	if _, err := pool.Exec(ctx, `INSERT INTO public.diagnostic_events(id,event_type,severity,correlation_id,payload) VALUES('debug-http-event','debug.fixture','info','debug-http','{"text":"SYNTHETIC_HTTP_PRIVATE"}')`); err != nil {
		t.Fatal(err)
	}
	server := New(conversationSessionRepository{PostgresRepository: repo}, "service-key", nil)
	server.app = &core.App{DB: repo}
	server.SetBrowserBoundary("http://fluctlight.local", false)
	handler := server.Handler()
	request := func(path string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(&http.Cookie{Name: "fluctlight_session", Value: "session"})
		req.Header.Set(serviceKeyHeader, "service-key")
		req.Header.Set(humanSessionHeader, "session")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}
	paths := []string{
		"/api/diagnostic-records/events", "/api/diagnostic-records/model-runs",
		"/api/diagnostic-records/media-prompts", "/api/diagnostics",
		"/api/diagnostics/model-runs", "/api/diagnostics/media-prompts",
		"/api/diagnostics/lifecycle", "/api/diagnostics/export",
		"/api/diagnostics/kev-decisions", "/api/diagnostic-export?q=SYNTHETIC_HTTP_PRIVATE",
		"/internal/diagnostics", "/internal/diagnostics/model-runs",
		"/internal/diagnostics/media-prompts", "/internal/diagnostics/lifecycle",
		"/internal/diagnostics/export", "/internal/diagnostics/kev-decisions",
	}
	for _, path := range paths {
		response := request(path)
		if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "diagnostic_debug_read_disabled") || strings.Contains(response.Body.String(), "SYNTHETIC_HTTP_PRIVATE") {
			t.Errorf("default permission %s: %d %s", path, response.Code, response.Body.String())
		}
	}
	server.app.DiagnosticDebugReadEnabled = true
	for _, path := range paths {
		response := request(path)
		if response.Code != http.StatusOK {
			t.Errorf("authorized entry %s: %d %s", path, response.Code, response.Body.String())
		}
	}
	for _, enabled := range []bool{true, false} {
		server.app.DiagnosticDebugReadEnabled = enabled
		response := request("/api/diagnostic-records/events?q=SYNTHETIC_HTTP_PRIVATE")
		if !enabled {
			if response.Code != http.StatusForbidden {
				t.Fatal("grant survived revocation", response.Code)
			}
			continue
		}
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "SYNTHETIC_HTTP_PRIVATE") {
			t.Fatal("authorized browser read", response.Code, response.Body.String())
		}
		var page map[string]any
		if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		ref := page["items"].([]any)[0].(map[string]any)["ref"].(string)
		detail := request("/api/diagnostic-records/events/" + ref)
		if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), "SYNTHETIC_HTTP_PRIVATE") {
			t.Fatal("authorized detail", detail.Code, detail.Body.String())
		}
	}
	export := request("/api/diagnostic-export?dataset=events")
	if export.Code != http.StatusOK || strings.Contains(export.Body.String(), "SYNTHETIC_HTTP_PRIVATE") || !strings.Contains(export.Body.String(), `"complete":true`) {
		t.Fatal("metadata export", export.Code, export.Body.String())
	}
	t.Run("readonly replay source reads preserve attempts and business state", func(t *testing.T) {
		server.app.DiagnosticDebugReadEnabled = true
		if _, err := pool.Exec(ctx, `INSERT INTO public.diagnostic_events(id,event_type,severity,correlation_id,payload) VALUES
   ('replay-request','adk.tool.requested','info','replay-http','{"call_id":"same-tool","status":"requested"}'),
   ('replay-failed','adk.tool.result','error','replay-http','{"call_id":"same-tool","status":"failed"}'),
   ('replay-succeeded','adk.tool.result','info','replay-http','{"call_id":"same-tool","status":"completed"}')`); err != nil {
			t.Fatal(err)
		}
		counts := func() []int64 {
			t.Helper()
			var tools, messages, media, commands int64
			if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.tool_executions),(SELECT count(*) FROM public.conversation_messages),(SELECT count(*) FROM public.media_intents),(SELECT count(*) FROM public.workflow_control_commands)`).Scan(&tools, &messages, &media, &commands); err != nil {
				t.Fatal(err)
			}
			return []int64{tools, messages, media, commands}
		}
		before := fmt.Sprint(counts())
		for _, kind := range []string{"events", "model-runs", "agent-runs", "turns", "links", "media-prompts", "workflow-intents", "media-recoveries", "kev"} {
			response := request("/api/diagnostic-records/" + kind + "?correlationId=replay-http&limit=30")
			if response.Code != http.StatusOK {
				t.Fatalf("replay source %s: %d %s", kind, response.Code, response.Body.String())
			}
			if kind == "events" {
				var result map[string]any
				if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if len(result["items"].([]any)) != 3 || result["next_cursor"] != "" || result["snapshot"] == "" || !strings.Contains(response.Body.String(), "failed") || !strings.Contains(response.Body.String(), "completed") {
					t.Fatal("replay lost separate stored attempts", response.Body.String())
				}
			}
		}
		if after := fmt.Sprint(counts()); after != before {
			t.Fatal("readonly source traversal changed business state", before, after)
		}
	})

}

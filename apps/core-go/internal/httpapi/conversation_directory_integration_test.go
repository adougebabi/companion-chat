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

func TestConversationBrowserDirectoryUsesPersistentCurrentSelection(t *testing.T) {
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
	name := fmt.Sprint("lac_conversation_api_", time.Now().UnixNano())
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
	if _, err := pool.Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES('human-1','human','active'),('conversation-api-instance','fluctlight','active'); INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES('conversation-api-instance','human-1','blank_slate','active','{}','{}','{}','{}','{}','{}')`); err != nil {
		t.Fatal(err)
	}
	// Only session resolution is deterministic; HTTP/BFF/App/transactions/DB are production.
	server := New(conversationSessionRepository{PostgresRepository: repo}, "service-key", nil)
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

	status, created := request(http.MethodPost, "/api/fluctlights/conversation-api-instance/conversations", map[string]any{"idempotencyKey": "session-one", "title": "First history"})
	if status != 200 {
		t.Fatal(status, created)
	}
	conversation := created["conversation"].(map[string]any)
	first := conversation["id"].(string)
	status, created = request(http.MethodPost, "/api/fluctlights/conversation-api-instance/conversations", map[string]any{"idempotencyKey": "session-two", "title": "Second history"})
	if status != 200 {
		t.Fatal(status, created)
	}
	second := created["conversation"].(map[string]any)["id"].(string)
	if first == second {
		t.Fatal("new command reused conversation")
	}
	status, directory := request(http.MethodGet, "/api/fluctlights/conversation-api-instance/conversations?limit=1", nil)
	if status != 200 || directory["currentConversationId"] != second || directory["nextCursor"] == "" {
		t.Fatal(status, directory)
	}
	status, conflict := request(http.MethodPost, "/api/fluctlights/conversation-api-instance/conversations", map[string]any{"idempotencyKey": "session-one", "title": "Different"})
	if status != 409 || conflict["code"] != "conversation_command_conflict" {
		t.Fatal("lost durable command conflict", status, conflict)
	}
	status, selected := request(http.MethodPost, "/api/fluctlights/conversation-api-instance/conversations/"+first+"/select", map[string]any{})
	if status != 200 || selected["conversation"].(map[string]any)["id"] != first {
		t.Fatal(status, selected)
	}
	status, current := request(http.MethodGet, "/api/fluctlights/conversation-api-instance/conversation", nil)
	if status != 200 || current["conversation"].(map[string]any)["id"] != first {
		t.Fatal("legacy current did not persist", status, current)
	}
	// Existing incoming history is a fixture; search, exact lookup and read all
	// cross the actual HTTP boundary and production application/database methods.
	if _, err := pool.Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,idempotency_key) VALUES('source-message',$1,1,'conversation-api-instance','assistant','needle historical fixture','source')`, first); err != nil {
		t.Fatal(err)
	}
	status, search := request(http.MethodGet, "/api/fluctlights/conversation-api-instance/messages/search?q=needle&limit=1", nil)
	if status != 200 {
		t.Fatal(status, search)
	}
	hits := search["items"].([]any)
	if len(hits) != 1 {
		t.Fatal(search)
	}
	hit := hits[0].(map[string]any)
	if hit["id"] != "source-message" || hit["conversationId"] != first || hit["sourceRef"] != "message:source-message" {
		t.Fatal("lost source identity", hit)
	}
	status, exact := request(http.MethodGet, "/api/conversations/"+first+"/messages/source-message", nil)
	if status != 200 || exact["text"] != hit["text"] {
		t.Fatal(status, exact)
	}
	status, directory = request(http.MethodGet, "/api/fluctlights/conversation-api-instance/conversations", nil)
	if status != 200 {
		t.Fatal(status, directory)
	}
	for _, raw := range directory["items"].([]any) {
		item := raw.(map[string]any)
		if item["conversation"].(map[string]any)["id"] == first && item["unreadCount"] != float64(1) {
			t.Fatal("unread projection", item)
		}
	}
	status, _ = request(http.MethodPost, "/api/conversations/"+first+"/read", map[string]any{"readSequence": 1})
	if status != 204 {
		t.Fatal("read default delivered", status)
	}
	status, directory = request(http.MethodGet, "/api/fluctlights/conversation-api-instance/conversations", nil)
	for _, raw := range directory["items"].([]any) {
		item := raw.(map[string]any)
		if item["conversation"].(map[string]any)["id"] == first && (item["unreadCount"] != float64(0) || item["lastReadSequence"] != float64(1)) {
			t.Fatal("read did not persist", item)
		}
	}
	status, _ = request(http.MethodGet, "/api/fluctlights/conversation-api-instance/messages/search?q=needle&cursor=invalid", nil)
	if status != 400 {
		t.Fatal("malformed cursor", status)
	}
	status, _ = request(http.MethodPost, "/api/fluctlights/conversation-api-instance/conversations", map[string]any{"idempotencyKey": "forged", "participantActorIds": []string{"other"}})
	if status != 400 {
		t.Fatal("forged scope accepted", status)
	}
}

type conversationSessionRepository struct{ *core.PostgresRepository }

func (r conversationSessionRepository) ResolveSession(_ context.Context, session string) (string, error) {
	if session != "session" {
		return "", core.ErrUnauthorized
	}
	return "human-1", nil
}

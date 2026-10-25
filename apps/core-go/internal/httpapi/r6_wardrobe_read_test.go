package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestR6WardrobeInternalAndBrowserReadSameActualWear(t *testing.T) {
	databaseURL := os.Getenv("GO_CORE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("task-owned PostgreSQL required")
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
	name := fmt.Sprint("lac_r6_wear_api_", time.Now().UnixNano())
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
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Path = "/" + name
	repo, err := core.NewPostgresRepository(ctx, parsed.String())
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
	if _, err := pool.Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES('human-1','human','active'),('r6-api','fluctlight','active');
 INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES('r6-api','human-1','blank_slate','active','{}','{"timezone":"Asia/Shanghai"}','{}','{}','{}','{}');
 INSERT INTO public.fluctlight_personality_runtime(fluctlight_id,active_profile_id,revision) VALUES('r6-api','default',0) ON CONFLICT DO NOTHING;
 INSERT INTO public.fluctlight_wardrobe_states(fluctlight_id,revision,wearing_state) VALUES('r6-api',1,'known') ON CONFLICT DO NOTHING;
 INSERT INTO public.fluctlight_wardrobe_items(id,fluctlight_id,category,slot,description,ownership,availability,source_kind,source_ref,source_item_key,item_kind,clothing_type,seasons) VALUES('r6-boots','r6-api','shoes','shoes','受控已穿短靴','owned','available','initialization','r6-seed','boots','wearable','shoes','{}');
 INSERT INTO public.fluctlight_worn_items(fluctlight_id,slot,item_id) VALUES('r6-api','shoes','r6-boots')`); err != nil {
		t.Fatal(err)
	}
	server := New(fakeRepository{}, "service-key", nil)
	server.app = &core.App{DB: repo}
	server.SetBrowserBoundary("http://fluctlight.local", false)
	read := func(browser bool) (int, map[string]any) {
		t.Helper()
		path := "/internal/fluctlights/r6-api/wardrobe"
		if browser {
			path = "/api/fluctlights/r6-api/wardrobe"
		}
		request := httptest.NewRequest(http.MethodGet, path, nil)
		if browser {
			request.Header.Set("Origin", "http://fluctlight.local")
			request.AddCookie(&http.Cookie{Name: "fluctlight_session", Value: "session"})
		} else {
			request.Header.Set(serviceKeyHeader, "service-key")
			request.Header.Set(humanSessionHeader, "session")
		}
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		var value map[string]any
		_ = json.Unmarshal(response.Body.Bytes(), &value)
		return response.Code, value
	}
	status, internal := read(false)
	if status != 200 {
		t.Fatal("internal read", status, internal)
	}
	status, browser := read(true)
	if status != 200 {
		t.Fatal("browser read", status, browser)
	}
	for _, key := range []string{"worn_items", "used_items"} {
		if internal[key] == nil || !reflect.DeepEqual(internal[key], browser[key]) {
			t.Fatal("transport current-wear shape differs", key, internal[key], browser[key])
		}
	}
	worn, ok := internal["worn_items"].([]any)
	if !ok || len(worn) != 1 || worn[0].(map[string]any)["item_id"] != "r6-boots" {
		t.Fatal("actual worn row lost", worn)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE public.fluctlight_worn_items RENAME TO r6_unavailable_worn_items`); err != nil {
		t.Fatal(err)
	}
	for _, browser := range []bool{false, true} {
		status, value := read(browser)
		if status == 200 {
			t.Fatal("read failure became successful empty wardrobe", browser, value)
		}
	}
}

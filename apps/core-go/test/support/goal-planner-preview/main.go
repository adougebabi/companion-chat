// Disposable production-build browser acceptance fixture. It has real auth,
// Core/BFF/DB and no Provider/Worker or external-action execution.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/httpapi"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func main() {
	raw := os.Getenv("GO_CORE_TEST_DATABASE_URL")
	u, err := url.Parse(raw)
	if err != nil || raw == "" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") {
		log.Fatal("loopback GO_CORE_TEST_DATABASE_URL is required")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	adminURL := *u
	adminURL.Path = "/postgres"
	admin, err := pgxpool.New(ctx, adminURL.String())
	if err != nil {
		log.Fatal(err)
	}
	defer admin.Close()
	name := fmt.Sprint("lac_goal_browser_", time.Now().UnixNano())
	quoted := pgx.Identifier{name}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+quoted); err != nil {
		log.Fatal(err)
	}
	u.Path = "/" + name
	repo, err := core.NewPostgresRepository(ctx, u.String())
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		repo.Close()
		cleanup, c := context.WithTimeout(context.Background(), 20*time.Second)
		defer c()
		_, _ = admin.Exec(cleanup, "DROP DATABASE "+quoted+" WITH (FORCE)")
	}()
	if err := migrations.New(repo.Pool()).Apply(ctx); err != nil {
		log.Fatal(err)
	}
	owner := "goal-browser-owner"
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active');`, owner); err != nil {
		log.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'fixture','fixture')`, owner); err != nil {
		log.Fatal(err)
	}
	app, err := core.NewApp(repo, base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32)), "browser-test-service", "127.0.0.1:9000", "test", "test", "test", "test", false)
	if err != nil {
		log.Fatal(err)
	}
	instance := "goal-browser-instance"
	if _, err := app.CreateFluctlight(ctx, owner, instance, "目标规划验证实例", "blank_slate", "", nil, nil, nil, "Asia/Shanghai"); err != nil {
		log.Fatal(err)
	}
	initialCount := 2
	if os.Getenv("GOAL_BROWSER_CAPACITY_FIXTURE") == "1" {
		initialCount = 5
	}
	for i := 0; i < initialCount; i++ {
		outcome, motive := fmt.Sprint("验证目标", i+1), "Owner有限测试愿望"
		if _, err := app.ApplyOwnerGoalCommand(ctx, owner, instance, "", core.GoalOwnerCommand{Operation: "create", IdempotencyKey: fmt.Sprint("fixture-", i), Reason: "隔离浏览器验收", DesiredOutcome: &outcome, Motivation: &motive, SuccessCriteria: []string{"真实观察对应结果"}}); err != nil {
			log.Fatal(err)
		}
	}
	token := "goal-browser-fixture-session"
	hash := sha256.Sum256([]byte(token))
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.auth_sessions(id,token_hash,human_actor_id,expires_at) VALUES('goal-browser-session',$1,$2,now()+interval '12 hours')`, hex.EncodeToString(hash[:]), owner); err != nil {
		log.Fatal(err)
	}
	address := "127.0.0.1:18867"
	backend := httpapi.NewApp(app, "browser-test-service", nil)
	backend.SetBrowserBoundary("http://"+address, false)
	api := backend.Handler()
	dist, err := filepath.Abs("../web/dist")
	if err != nil {
		log.Fatal(err)
	}
	files := http.FileServer(http.Dir(dist))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.SetCookie(w, &http.Cookie{Name: "fluctlight_session", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
			http.SetCookie(w, &http.Cookie{Name: "fluctlight_csrf", Value: "goal-browser-csrf", Path: "/", SameSite: http.SameSiteStrictMode})
		}
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/auth/") {
			api.ServeHTTP(w, r)
			return
		}
		files.ServeHTTP(w, r)
	})
	server := &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		c, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_ = server.Shutdown(c)
	}()
	fmt.Printf("GOAL_BROWSER_FIXTURE url=http://%s instance=%s database=%s production_build=%s\n", address, instance, name, dist)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Print(err)
	}
}

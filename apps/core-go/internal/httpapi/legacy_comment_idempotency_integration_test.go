package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Real HTTP/Browser/Core and disposable PostgreSQL. No model or renderer.
func TestLegacyMomentCommentIdempotencyHTTP(t *testing.T) {
	databaseURL := os.Getenv("GO_CORE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GO_CORE_TEST_DATABASE_URL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
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
	name := fmt.Sprintf("lac_legacy_comment_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{name}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+quoted); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	var repo *core.PostgresRepository
	t.Cleanup(func() {
		if repo != nil {
			repo.Close()
		}
		clean, stop := context.WithTimeout(context.Background(), 20*time.Second)
		defer stop()
		if _, err := admin.Exec(clean, "DROP DATABASE "+quoted); err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	isolated, err := momentHealthDisposableDatabaseURL(databaseURL, name)
	if err != nil {
		t.Fatal(err)
	}
	repo, err = core.NewPostgresRepository(ctx, isolated)
	if err != nil {
		t.Fatal(err)
	}
	if err = migrations.New(repo.Pool()).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES('human-1','human','active'),('other-owner','human','active'),('comment-role','fluctlight','active');
 INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES('human-1','fixture','fixture');
 INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES('comment-role','human-1','blank_slate','active','{}','{}','{}','{}','{}','{}');
 INSERT INTO public.moments(id,owner_actor_id,owner_fluctlight_id,author_actor_id,text,visibility,status,media_asset_ids) VALUES('comment-moment','human-1','comment-role','comment-role','original','private','visible','[]'),('other-moment','human-1','comment-role','comment-role','other','private','visible','[]'),('foreign-moment','other-owner',NULL,'other-owner','foreign','private','visible','[]');`); err != nil {
		t.Fatal(err)
	}
	app := &core.App{DB: repo}
	server := New(conversationSessionRepository{PostgresRepository: repo}, "service-key", nil)
	server.app = app
	server.SetBrowserBoundary("http://fluctlight.local", false)
	httpServer := httptest.NewServer(server.Handler())
	t.Cleanup(httpServer.Close)
	client := httpServer.Client()
	request := func(path string, body map[string]any, auth, csrf, internal bool) (int, map[string]any) {
		raw, _ := json.Marshal(body)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, httpServer.URL+path, strings.NewReader(string(raw)))
		if err != nil {
			return 0, map[string]any{"error": err.Error()}
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "http://fluctlight.local")
		if auth {
			if internal {
				req.Header.Set("x-fluctlight-service-key", "service-key")
				req.Header.Set("x-fluctlight-human-session", "session")
			} else {
				req.AddCookie(&http.Cookie{Name: "fluctlight_session", Value: "session"})
			}
		}
		if csrf {
			req.Header.Set("X-CSRF-Token", "csrf")
			req.AddCookie(&http.Cookie{Name: "fluctlight_csrf", Value: "csrf"})
		}
		resp, err := client.Do(req)
		if err != nil {
			return 0, map[string]any{"error": err.Error()}
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		result := map[string]any{}
		_ = json.Unmarshal(data, &result)
		return resp.StatusCode, result
	}
	path := "/api/moments/comment-moment/comments"
	body := func(key, text string) map[string]any { return map[string]any{"text": text, "idempotencyKey": key} }
	counts := func() (comments, revision, outbox, receipts int) {
		t.Helper()
		if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.moment_comments WHERE moment_id='comment-moment'),(SELECT revision FROM public.moments WHERE id='comment-moment'),(SELECT count(*) FROM public.platform_outbox_events WHERE kind='moment.comment.created'),(SELECT count(*) FROM public.owner_surface_idempotency WHERE owner_actor_id='human-1')`).Scan(&comments, &revision, &outbox, &receipts); err != nil {
			t.Fatal(err)
		}
		return
	}
	assertDelta := func(t *testing.T, before [4]int, want int) {
		t.Helper()
		a, b, c, d := counts()
		if a-before[0] != want || b-before[1] != want || c-before[2] != want || d-before[3] != want {
			t.Fatalf("side effect delta comments/revision/outbox/receipts=%v want each=%d", [4]int{a - before[0], b - before[1], c - before[2], d - before[3]}, want)
		}
	}
	snapshot := func() [4]int { a, b, c, d := counts(); return [4]int{a, b, c, d} }
	t.Run("same_key_replays_original_response", func(t *testing.T) {
		before := snapshot()
		status, first := request(path, body("legacy-once", "same comment"), true, true, false)
		if status != 200 {
			t.Fatalf("first=%d %#v", status, first)
		}
		status, second := request(path, body("legacy-once", "same comment"), true, true, false)
		if status != 200 || !reflect.DeepEqual(first, second) || len(second) != 4 {
			t.Fatalf("same operation created a different receipt: first=%#v second=%#v status=%d", first, second, status)
		}
		assertDelta(t, before, 1)
	})
	t.Run("same_key_changed_body_target_or_operation_conflicts", func(t *testing.T) {
		_, _ = request(path, body("legacy-conflict", "original"), true, true, false)
		before := snapshot()
		for _, sample := range []struct {
			path string
			body map[string]any
		}{{path, body("legacy-conflict", "changed")}, {"/api/moments/other-moment/comments", body("legacy-conflict", "original")}} {
			status, result := request(sample.path, sample.body, true, true, false)
			if status != 409 {
				t.Errorf("conflict=%d %#v", status, result)
			}
		}
		assertDelta(t, before, 0)
	})
	t.Run("concurrent_same_key_exactly_once", func(t *testing.T) {
		before := snapshot()
		var wg sync.WaitGroup
		results := make(chan string, 8)
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				status, result := request(path, body("legacy-concurrent", "concurrent"), true, true, false)
				if status != 200 {
					t.Errorf("concurrent=%d %#v", status, result)
				}
				results <- fmt.Sprint(result["id"])
			}()
		}
		wg.Wait()
		close(results)
		id := ""
		for got := range results {
			if id != "" && id != got {
				t.Errorf("different replay ids %s/%s", id, got)
			}
			id = got
		}
		assertDelta(t, before, 1)
	})
	t.Run("lost_response_and_internal_transport_share_receipt", func(t *testing.T) {
		before := snapshot()
		status, _ := request(path, body("legacy-lost", "keep original"), true, true, false)
		if status != 200 {
			t.Fatalf("first response status=%d", status)
		} // Deliberately discard the first receipt.
		status, result := request("/internal/moments/comment-moment/comments", map[string]any{"text": "keep original", "idempotency_key": "legacy-lost"}, true, false, true)
		if status != 200 || result["id"] == nil || len(result) != 4 {
			t.Fatalf("internal retry=%d %#v", status, result)
		}
		assertDelta(t, before, 1)
	})
	t.Run("old_unkeyed_calls_and_long_text_remain_compatible", func(t *testing.T) {
		before := snapshot()
		text := strings.Repeat("字", 32000)
		s1, a := request(path, map[string]any{"text": text}, true, true, false)
		s2, b := request(path, map[string]any{"text": text}, true, true, false)
		if s1 != 200 || s2 != 200 || a["id"] == b["id"] || a["text"] != text || len(a) != 4 {
			t.Fatalf("legacy compatibility status=%d/%d", s1, s2)
		}
		after := snapshot()
		if after[0]-before[0] != 2 || after[1]-before[1] != 2 || after[2]-before[2] != 2 || after[3] != before[3] {
			t.Fatalf("unkeyed request effects before=%v after=%v", before, after)
		}
	})
	t.Run("authentication_csrf_target_and_key_validation_have_no_effect", func(t *testing.T) {
		before := snapshot()
		for _, sample := range []struct {
			path                 string
			payload              map[string]any
			auth, csrf, internal bool
			status               int
		}{{path, body("anonymous", "x"), false, true, false, 401}, {path, body("no-csrf", "x"), true, false, false, 403}, {"/internal/moments/comment-moment/comments", map[string]any{"text": "x"}, false, false, true, 401}, {"/api/moments/foreign-moment/comments", body("foreign", "x"), true, true, false, 403}, {path, body("", "x"), true, true, false, 400}, {path, map[string]any{"text": "x", "idempotencyKey": 7}, true, true, false, 400}, {path, body(strings.Repeat("k", 257), "x"), true, true, false, 400}} {
			status, result := request(sample.path, sample.payload, sample.auth, sample.csrf, sample.internal)
			if status != sample.status {
				t.Errorf("denial want=%d got=%d %#v", sample.status, status, result)
			}
		}
		assertDelta(t, before, 0)
	})
	t.Run("distinct_keys_keep_intentional_identical_comments_and_unicode_keys", func(t *testing.T) {
		before := snapshot()
		for _, key := range []string{"intentional-repeat-1", strings.Repeat("键", 256)} {
			status, result := request(path, body(key, "same intentional text"), true, true, false)
			if status != 200 {
				t.Fatalf("independent comment=%d %#v", status, result)
			}
		}
		assertDelta(t, before, 2)
	})
	t.Run("legacy_key_cannot_be_reused_for_new_feed_operation", func(t *testing.T) {
		before := snapshot()
		status, result := request("/api/moment-feed/comment-moment/comments", map[string]any{"text": "original", "reason": "new feed", "expectedRevision": before[1], "idempotencyKey": "legacy-conflict"}, true, true, false)
		if status != 409 {
			t.Fatalf("cross-operation conflict=%d %#v", status, result)
		}
		assertDelta(t, before, 0)
	})
	t.Run("hidden_target_denies_new_operation_but_preserves_committed_receipt", func(t *testing.T) {
		_, first := request(path, body("legacy-visible", "before hide"), true, true, false)
		if _, err := repo.Pool().Exec(ctx, `UPDATE public.moments SET status='hidden' WHERE id='comment-moment'`); err != nil {
			t.Fatal(err)
		}
		defer repo.Pool().Exec(ctx, `UPDATE public.moments SET status='visible' WHERE id='comment-moment'`)
		before := snapshot()
		status, replay := request(path, body("legacy-visible", "before hide"), true, true, false)
		if status != 200 || !reflect.DeepEqual(first, replay) {
			t.Errorf("hidden committed replay=%d %#v", status, replay)
		}
		status, _ = request(path, body("legacy-hidden-new", "new"), true, true, false)
		if status != 409 {
			t.Errorf("hidden new=%d", status)
		}
		assertDelta(t, before, 0)
	})
	t.Run("outbox_failure_rolls_back_comment_revision_and_receipt", func(t *testing.T) {
		if _, err := repo.Pool().Exec(ctx, `CREATE FUNCTION public.reject_legacy_comment_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.kind='moment.comment.created' THEN RAISE EXCEPTION 'isolated injected outbox failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_legacy_comment_event BEFORE INSERT ON public.platform_outbox_events FOR EACH ROW EXECUTE FUNCTION public.reject_legacy_comment_event()`); err != nil {
			t.Fatal(err)
		}
		remove := func() {
			if _, err := repo.Pool().Exec(ctx, `DROP TRIGGER IF EXISTS reject_legacy_comment_event ON public.platform_outbox_events; DROP FUNCTION IF EXISTS public.reject_legacy_comment_event()`); err != nil {
				t.Error(err)
			}
		}
		defer remove()
		before := snapshot()
		status, _ := request(path, body("legacy-rollback", "atomic"), true, true, false)
		if status < 400 {
			t.Fatalf("injected outbox failure returned %d", status)
		}
		assertDelta(t, before, 0)
		remove()
		status, result := request(path, body("legacy-rollback", "atomic"), true, true, false)
		if status != 200 {
			t.Fatalf("rolled-back key was consumed: %d %#v", status, result)
		}
		assertDelta(t, before, 1)
	})
	t.Run("concurrent_key_reuse_with_common_owner_command_is_a_typed_conflict", func(t *testing.T) {
		barrier, err := repo.Pool().Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer barrier.Rollback(ctx)
		if _, err := barrier.Exec(ctx, `SELECT id FROM public.fluctlights WHERE id='comment-role' FOR UPDATE`); err != nil {
			t.Fatal(err)
		}
		key := "legacy-versus-common"
		newConversation := make(chan error, 1)
		go func() {
			_, err := app.NewDirectConversation(ctx, "human-1", "comment-role", "isolated command race", key)
			newConversation <- err
		}()
		waitFor := func(predicate func() bool) {
			t.Helper()
			deadline := time.Now().Add(5 * time.Second)
			for !predicate() {
				if time.Now().After(deadline) {
					t.Fatal("command did not reach the expected database barrier")
				}
				time.Sleep(5 * time.Millisecond)
			}
		}
		waitFor(func() bool {
			var blocked bool
			err := repo.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%SELECT id FROM public.fluctlights%FOR UPDATE%')`).Scan(&blocked)
			if err != nil {
				t.Fatal(err)
			}
			return blocked
		})
		comment := make(chan int, 1)
		go func() {
			status, _ := request(path, body(key, "must conflict with common command"), true, true, false)
			comment <- status
		}()
		commentStatus := 0
		waitFor(func() bool {
			select {
			case commentStatus = <-comment:
				return true
			default:
			}
			var blocked bool
			err := repo.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'SELECT pg_advisory_xact_lock(hashtextextended($1,0))%')`).Scan(&blocked)
			if err != nil {
				t.Fatal(err)
			}
			return blocked
		})
		if err := barrier.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		if commentStatus == 0 {
			commentStatus = <-comment
		}
		newErr := <-newConversation
		if commentStatus == 200 {
			if !errors.Is(newErr, core.ErrConflict) {
				t.Fatalf("other operation returned a storage failure instead of conflict: %v", newErr)
			}
		} else if commentStatus != 409 || newErr != nil {
			t.Fatalf("common command/comment outcomes: new=%v comment=%d", newErr, commentStatus)
		}
	})
	t.Run("new_feed_comments_keep_revision_receipts_and_conflict_with_legacy_reuse", func(t *testing.T) {
		before := snapshot()
		payload := map[string]any{"text": "new feed still works", "reason": "explicit comment", "expectedRevision": before[1], "idempotencyKey": "new-feed-key"}
		status, first := request("/api/moment-feed/comment-moment/comments", payload, true, true, false)
		if status != 200 || first["id"] != "comment-moment" {
			t.Fatalf("new feed=%d %#v", status, first)
		}
		status, _ = request("/api/moment-feed/comment-moment/comments", payload, true, true, false)
		if status != 200 {
			t.Fatalf("new feed replay=%d", status)
		}
		assertDelta(t, before, 1)
		status, _ = request(path, body("new-feed-key", "new feed still works"), true, true, false)
		if status != 409 {
			t.Fatalf("legacy reuse of feed key=%d", status)
		}
		assertDelta(t, before, 1)
	})
	t.Run("receipt_keys_are_actor_scoped_without_disclosing_another_actor_result", func(t *testing.T) {
		result, err := app.CommentMoment(ctx, "other-owner", "foreign-moment", "own independent comment", "legacy-once")
		if err != nil || result["author_actor_id"] != "other-owner" || result["moment_id"] != "foreign-moment" {
			t.Fatalf("actor-scoped receipt=%#v err=%v", result, err)
		}
		before := snapshot()
		status, _ := request("/api/moments/foreign-moment/comments", body("legacy-once", "own independent comment"), true, true, false)
		if status != 403 {
			t.Fatalf("foreign receipt exposed with status=%d", status)
		}
		assertDelta(t, before, 0)
	})
	t.Run("plugin_denial_does_not_consume_key_and_committed_replay_survives_disable", func(t *testing.T) {
		payload := body("legacy-before-disable", "already committed")
		status, first := request(path, payload, true, true, false)
		if status != 200 {
			t.Fatalf("before disable=%d %#v", status, first)
		}
		if _, err := app.UpdateSettings(ctx, "human-1", map[string]any{"values": map[string]any{"plugins.domains": map[string]any{"image": false, "moment": false}}, "expected_versions": map[string]any{"plugins.domains": 0}, "idempotency_key": "disable-comments"}); err != nil {
			t.Fatal(err)
		}
		before := snapshot()
		status, replay := request(path, payload, true, true, false)
		if status != 200 || !reflect.DeepEqual(first, replay) {
			t.Errorf("replay after disable=%d %#v", status, replay)
		}
		status, _ = request(path, body("legacy-after-disable", "must not create"), true, true, false)
		if status < 400 {
			t.Errorf("disabled new request=%d", status)
		}
		assertDelta(t, before, 0)
		if _, err := app.UpdateSettings(ctx, "human-1", map[string]any{"values": map[string]any{"plugins.domains": map[string]any{"image": false, "moment": true}}, "expected_versions": map[string]any{"plugins.domains": 1}, "idempotency_key": "enable-comments"}); err != nil {
			t.Fatal(err)
		}
		status, result := request(path, body("legacy-after-disable", "must not create"), true, true, false)
		if status != 200 {
			t.Fatalf("same rejected key after enable=%d %#v", status, result)
		}
	})
}

package core

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Only a loopback fixture is contacted. A reachable status endpoint does not
// establish that ComfyUI can render an image.
func TestSavedMediaConnectionRejectsStaleConfigurationResult(t *testing.T) {
	for _, firstFails := range []bool{false, true} {
		t.Run(fmt.Sprintf("old_probe_failed_%t", firstFails), func(t *testing.T) {
			started, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			defer unblock()
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/system_stats" {
					t.Errorf("unexpected probe: %s %s", r.Method, r.URL.Path)
					http.Error(w, "unexpected", http.StatusNotFound)
					return
				}
				if calls.Add(1) == 1 {
					close(started)
					<-release
					if firstFails {
						http.Error(w, "fixture failure", http.StatusServiceUnavailable)
						return
					}
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"system":{"fixture":true}}`))
			}))
			defer func() { unblock(); server.Close() }()
			ctx, repo, app, owner, _, _ := setupDirectPublicationToolTest(t, "health-version")
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'fixture','fixture')`, owner); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.Pool().Exec(ctx, `DELETE FROM public.runtime_settings WHERE key='media.comfyui'`); err != nil {
				t.Fatal(err)
			}
			save := func(version int, key, text string) {
				t.Helper()
				config := map[string]any{"baseUrl": server.URL, "workflow": map[string]any{"1": map[string]any{"class_type": "CLIPTextEncode", "inputs": map[string]any{"text": text}}}}
				if _, err := app.UpdateSettings(ctx, owner, map[string]any{"values": map[string]any{"media.comfyui": config}, "expected_versions": map[string]any{"media.comfyui": version}, "idempotency_key": key}); err != nil {
					t.Fatal(err)
				}
			}
			save(0, "health-v1", "{{prompt}}")
			type outcome struct {
				value map[string]any
				err   error
			}
			done := make(chan outcome, 1)
			go func() { value, err := app.TestSavedMediaConnection(ctx, owner); done <- outcome{value, err} }()
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("connection probe did not start")
			}
			save(1, "health-v2", "new {{prompt}}")
			unblock()
			var first outcome
			select {
			case first = <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("connection probe did not finish")
			}
			if first.err != nil || first.value["state"] != "configuration_changed" || intValue(first.value["tested_config_version"]) != 1 || intValue(first.value["config_version"]) != 2 || first.value["generation_tested"] != false {
				t.Fatalf("stale probe was presented as current: %#v err=%v", first.value, first.err)
			}
			current, err := app.TestSavedMediaConnection(ctx, owner)
			if err != nil || current["state"] != "reachable" || intValue(current["config_version"]) != 2 || current["generation_tested"] != false || calls.Load() != 2 {
				t.Fatalf("current probe=%#v calls=%d err=%v", current, calls.Load(), err)
			}
		})
	}
}

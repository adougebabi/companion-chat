package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

type m4NativeRequest struct {
	phase string
	done  chan struct{}
}

// This opt-in, loopback-only fixture serves the actual production Web build.
// The three trigger requests execute the same typed reflection acceptance as
// the automated React scenario. All data and Provider responses are synthetic.
func runM4NativeManagementFixture(t *testing.T, parent context.Context, server *Server, webRoot, manifest string, run func(string)) {
	t.Helper()
	ctx, cancel := context.WithCancel(parent)
	browser := httptest.NewUnstartedServer(nil)
	defer browser.Close()
	defer cancel()
	base := "http://" + browser.Listener.Addr().String()
	server.SetBrowserBoundary(base, false)
	backend := server.Handler()
	requests := make(chan m4NativeRequest)
	files := http.FileServer(http.Dir(filepath.Join(webRoot, "dist")))
	if _, err := os.Stat(filepath.Join(webRoot, "dist", "index.html")); err != nil {
		t.Fatal("matching production Web build required", err)
	}
	browser.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/__m4/start":
			http.SetCookie(w, &http.Cookie{Name: "fluctlight_session", Value: "session", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
			http.SetCookie(w, &http.Cookie{Name: "fluctlight_csrf", Value: "csrf", Path: "/", SameSite: http.SameSiteLaxMode})
			http.Redirect(w, r, "/#/diag", http.StatusSeeOther)
		case "/__m4/run", "/__m4/finish":
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			phase := r.URL.Query().Get("phase")
			if r.URL.Path == "/__m4/finish" {
				phase = "finish"
			}
			request := m4NativeRequest{phase: phase, done: make(chan struct{})}
			select {
			case requests <- request:
			case <-ctx.Done():
				http.Error(w, "fixture stopped", 503)
				return
			}
			select {
			case <-request.done:
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"phase": phase, "contract": "pass", "provider": "controlled_fixture"})
			case <-ctx.Done():
				http.Error(w, "fixture stopped", 503)
			}
		default:
			if isBrowserPath(r.URL.Path) {
				backend.ServeHTTP(w, r)
			} else {
				files.ServeHTTP(w, r)
			}
		}
	})
	browser.Start()
	encoded, err := json.Marshal(map[string]any{"base_url": base, "start_url": base + "/__m4/start", "data": "disposable PostgreSQL; synthetic session and Provider", "phases": []string{"initial", "override", "restored"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	t.Log("M4_NATIVE_READY", manifest)
	expectedPhases := []string{"initial", "override", "restored"}
	index := 0
	for {
		select {
		case request := <-requests:
			if request.phase == "finish" {
				if index != len(expectedPhases) {
					t.Fatal("native browser acceptance incomplete", index)
				}
				close(request.done)
				return
			}
			if index >= len(expectedPhases) || request.phase != expectedPhases[index] {
				t.Fatal("unexpected native fixture phase", request.phase, index)
			}
			model := "m4-primary"
			if request.phase == "override" {
				model = "m4-override"
			}
			run(model)
			t.Logf("M4_NATIVE_PHASE %s actual_request_model=%s contract=pass", request.phase, model)
			index++
			close(request.done)
		case <-ctx.Done():
			t.Fatal("native browser fixture timed out before completing three phases")
		}
	}
}

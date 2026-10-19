package browser

import (
	"context"
	"net/http"
	"testing"
)

type mediaUploadTestBackend struct {
	fakeBackend
	calls  int
	replay bool
}

func (b *mediaUploadTestBackend) UploadOwnerMedia(_ context.Context, session string, r *http.Request) (map[string]any, error) {
	b.calls++
	if session != "opaque" {
		return nil, &CoreError{Status: 401, Code: "unauthorized"}
	}
	return map[string]any{"asset_id": "controlled-asset", "replayed": b.replay}, nil
}
func TestMultipartMediaUploadUsesSessionAndCSRFBeforeBackend(t *testing.T) {
	backend := &mediaUploadTestBackend{}
	handler := newBrowserTestHandler(backend)
	for _, tc := range []struct {
		session, csrf bool
		want          int
	}{{false, false, 401}, {true, false, 403}, {true, true, 201}} {
		response := surfaceInvoke(handler, http.MethodPost, "/api/media", "", tc.session, tc.csrf)
		if response.code != tc.want {
			t.Fatalf("session=%v csrf=%v status=%d body=%s", tc.session, tc.csrf, response.code, response.body)
		}
	}
	if backend.calls != 1 {
		t.Fatal("unauthorized upload reached backend", backend.calls)
	}
	backend.replay = true
	response := surfaceInvoke(handler, http.MethodPost, "/api/media", "", true, true)
	if response.code != 200 || response.header.Get("Cache-Control") != "private, no-store" {
		t.Fatal("replay is not an explicit private receipt", response)
	}
}

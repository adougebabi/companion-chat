package browser

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOwnerPersonaSwitchValidatesDualCASBeforeCore(t *testing.T) {
	valid := `{"targetPersonaId":"b","expectedRevision":0,"expectedRuntimeRevision":0,"reason":"受控切换","confirm":true,"idempotencyKey":"owner-switch"}`
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"valid", valid, true},
		{"missing runtime", strings.Replace(valid, `"expectedRuntimeRevision":0,`, "", 1), false},
		{"fractional runtime", strings.Replace(valid, `"expectedRuntimeRevision":0`, `"expectedRuntimeRevision":0.5`, 1), false},
		{"negative foundation", strings.Replace(valid, `"expectedRevision":0`, `"expectedRevision":-1`, 1), false},
		{"unconfirmed", strings.Replace(valid, `"confirm":true`, `"confirm":false`, 1), false},
		{"forged owner", strings.Replace(valid, `"targetPersonaId"`, `"ownerActorId":"foreign","targetPersonaId"`, 1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			backend := &fakeBackend{}
			handler := New(Options{Backend: backend, TrustedOrigin: "http://fluctlight.local"}).Handler()
			req := httptest.NewRequest(http.MethodPost, "/api/fluctlights/fl-1/persona-switch", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", "http://fluctlight.local")
			req.Header.Set("X-CSRF-Token", "csrf")
			req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
			req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf"})
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			if tc.valid {
				if res.Code >= 400 {
					t.Fatalf("valid request: %d %s", res.Code, res.Body.String())
				}
			} else if res.Code != 400 {
				t.Fatalf("invalid request reached Core: %d %s", res.Code, res.Body.String())
			}
		})
	}
}

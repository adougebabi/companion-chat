package browser

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type unavailableSessionBackend struct{ fakeBackend }

func (b *unavailableSessionBackend) DoJSON(ctx context.Context, method, path, session string, body any) (map[string]any, error) {
	if path == "/internal/auth/session" {
		return nil, errors.New("controlled DB unavailability")
	}
	return b.fakeBackend.DoJSON(ctx, method, path, session, body)
}
func TestSessionDependencyFailurePreservesCookieForRecovery(t *testing.T) {
	handler := newBrowserTestHandler(&unavailableSessionBackend{})
	req := httptest.NewRequest(http.MethodGet, "https://api.test/auth/session", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "opaque"})
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusServiceUnavailable {
		t.Fatal(res.Code, res.Body.String())
	}
	for _, cookie := range res.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			t.Fatal("temporary failure modified session cookie")
		}
	}
}

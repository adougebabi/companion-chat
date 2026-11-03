package httpapi

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/httpapi/browser"
)

func TestBrowserWardrobeBoundaryPreservesSurfaceAndAuthenticationErrors(t *testing.T) {
	server := New(fakeRepository{}, "service-key", nil)
	server.app = &core.App{}
	backend := newBrowserBackend(server)
	for _, tc := range []struct {
		session, code string
		status        int
	}{
		{"session", "surface_unavailable", http.StatusServiceUnavailable},
		{"invalid-session", "unauthenticated", http.StatusUnauthorized},
	} {
		_, err := backend.DoJSON(context.Background(), http.MethodPost, "/internal/fluctlights/fl-1/wardrobe/wear", tc.session, map[string]any{})
		var public *browser.CoreError
		if !errors.As(err, &public) || public.Status != tc.status || public.Code != tc.code {
			t.Fatalf("wardrobe boundary replaced existing error: %#v (%v)", public, err)
		}
	}
}

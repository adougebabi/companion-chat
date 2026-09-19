package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServerHandlerMountsPublicBrowserBoundaryWithoutOpeningInternalRoutes(t *testing.T) {
	server := New(fakeRepository{}, "service-key", nil)
	server.SetBrowserBoundary("https://fluctlight.test", true)
	handler := server.Handler()

	public := httptest.NewRequest(http.MethodGet, "https://api.test/api/platform/ping", nil)
	publicResponse := httptest.NewRecorder()
	handler.ServeHTTP(publicResponse, public)
	if publicResponse.Code != http.StatusOK {
		t.Fatalf("public API status = %d, body = %s", publicResponse.Code, publicResponse.Body.String())
	}

	internal := httptest.NewRequest(http.MethodGet, "https://api.test/internal/platform/ping", nil)
	internalResponse := httptest.NewRecorder()
	handler.ServeHTTP(internalResponse, internal)
	if internalResponse.Code != http.StatusUnauthorized {
		t.Fatalf("internal route without service identity status = %d", internalResponse.Code)
	}
}

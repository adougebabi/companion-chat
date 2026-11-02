package browser

import (
	"net/http"
	"strings"
	"testing"
)

func TestWorldEventDiagnosticFiltersPassThroughBrowserBoundary(t *testing.T) {
	for _, kind := range []string{"workflow-intents", "world-event-generations"} {
		t.Run(kind, func(t *testing.T) {
			backend := &fakeBackend{}
			handler := newBrowserTestHandler(backend)
			query := "worldId=world%2Fone&intentType=world.events.generate&cursor=opaque%2F%2B%3Fcursor&limit=10"
			response := invokeBrowser(handler, http.MethodGet, "https://api.test/api/diagnostic-records/"+kind+"?"+query, "", nil, map[string]string{sessionCookieName: "opaque"})
			if response.Code != http.StatusOK {
				t.Fatal(response.Code, response.Body.String())
			}
			for _, call := range backend.calls {
				if strings.HasSuffix(call, "/internal/diagnostic-records/"+kind+"?"+query) {
					return
				}
			}
			t.Fatal("lost world/task filter or opaque cursor", backend.calls)
		})
	}
}

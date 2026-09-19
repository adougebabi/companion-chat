package httpapi

import (
	"os"
	"strings"
	"testing"
)

func TestBrowserBackendHasNoHTTPForwardingClient(t *testing.T) {
	source, err := os.ReadFile("browser_backend.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, forbidden := range []string{"http.Client", "http.NewRequest", ".Do(req)", strings.Join([]string{"CORE", "_BASE_URL"}, ""), strings.Join([]string{"Core", "BaseURL"}, "")} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("browser backend still contains forwarding implementation %q", forbidden)
		}
	}
	for _, required := range []string{"b.server.app.Login", "b.server.app.StreamTurn", "b.server.app.AuthorizeAsset", "b.server.repository.ResolveSession"} {
		if !strings.Contains(text, required) {
			t.Fatalf("browser backend is missing direct service evidence %q", required)
		}
	}
}

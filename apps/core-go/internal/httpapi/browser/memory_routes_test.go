package browser

import (
	"net/http/httptest"
	"testing"
)

func TestMemoryReadQueriesAreBoundedAndClosed(t *testing.T) {
	for _, query := range []string{"limit=0", "limit=101", "limit=-1", "limit=x", "revision=-1", "limit=5&limit=6", "unknown=1"} {
		r := httptest.NewRequest("GET", "/api/memories/mem/sources?"+query, nil)
		w := httptest.NewRecorder()
		if _, ok := memoryPageQuery(w, r, "revision"); ok && query != "revision=-1" {
			t.Fatalf("accepted %q", query)
		}
		if query == "revision=-1" {
			server := New(Options{Backend: &fakeBackend{}})
			if !server.memoryRoutes(w, r) || w.Code != 400 {
				t.Fatalf("revision accepted %d", w.Code)
			}
		}
	}
	for _, query := range []string{"", "limit=100&cursor=opaque", "revision=0"} {
		r := httptest.NewRequest("GET", "/api/memories/mem/sources?"+query, nil)
		w := httptest.NewRecorder()
		if _, ok := memoryPageQuery(w, r, "revision"); !ok {
			t.Fatalf("rejected %q", query)
		}
	}
}

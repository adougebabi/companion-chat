package bff

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// TestGoRouteInventoryMatchesBrowserOpenAPI makes the checked browser
// artifact the source of truth for method/path coverage. The existing route
// smoke still proves that every listed operation reaches a handler; this test
// additionally fails when the artifact gains or loses an operation.
func TestGoRouteInventoryMatchesBrowserOpenAPI(t *testing.T) {
	artifact := filepath.Join(repositoryRoot(t), "packages", "browser-client", "openapi.json")
	data, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatalf("read browser OpenAPI artifact: %v", err)
	}
	var document struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("decode browser OpenAPI artifact: %v", err)
	}

	want := make(map[string]struct{})
	for path, operations := range document.Paths {
		for method := range operations {
			want[strings.ToUpper(method)+" "+path] = struct{}{}
		}
	}

	got := make(map[string]struct{})
	for _, route := range browserRouteCases() {
		if route.method == http.MethodOptions {
			continue
		}
		matches := make([]string, 0, 1)
		for template := range document.Paths {
			if concretePathMatchesTemplate(route.path, template) {
				matches = append(matches, template)
			}
		}
		if len(matches) != 1 {
			t.Fatalf("route %s %s matched %d OpenAPI templates: %v", route.method, route.path, len(matches), matches)
		}
		got[strings.ToUpper(route.method)+" "+matches[0]] = struct{}{}
	}

	if len(got) != len(want) {
		t.Fatalf("route/OpenAPI operation count differs: Go=%d OpenAPI=%d\nGo-only=%v\nOpenAPI-only=%v", len(got), len(want), sortedSetDifference(got, want), sortedSetDifference(want, got))
	}
	if difference := sortedSetDifference(got, want); len(difference) > 0 {
		t.Fatalf("Go route operations not in Browser OpenAPI: %v", difference)
	}
	if difference := sortedSetDifference(want, got); len(difference) > 0 {
		t.Fatalf("Browser OpenAPI operations missing from Go route inventory: %v", difference)
	}
}

func TestKnownPathsAndInternalPathsNeverReachCoreOnWrongMethod(t *testing.T) {
	coreCalls := 0
	handler := testBFF(t, func(request *http.Request) (*http.Response, error) {
		coreCalls++
		return jsonResponse(http.StatusOK, `{}`), nil
	})

	wrongMethod := invoke(handler, http.MethodDelete, "http://gateway.test/api/fluctlights", "", nil, nil)
	if wrongMethod.Code != http.StatusNotFound && wrongMethod.Code != http.StatusMethodNotAllowed {
		t.Fatalf("wrong method status = %d, want 404 or 405", wrongMethod.Code)
	}
	internal := invoke(handler, http.MethodGet, "http://gateway.test/internal/fluctlights", "", nil, nil)
	if internal.Code != http.StatusNotFound {
		t.Fatalf("internal path status = %d, want 404", internal.Code)
	}
	if coreCalls != 0 {
		t.Fatalf("Core calls = %d, want 0", coreCalls)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", "..", ".."))
}

func concretePathMatchesTemplate(concrete, template string) bool {
	concreteParts, templateParts := splitPath(concrete), splitPath(template)
	if len(concreteParts) != len(templateParts) {
		return false
	}
	for index, part := range templateParts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			if concreteParts[index] == "" {
				return false
			}
			continue
		}
		if part != concreteParts[index] {
			return false
		}
	}
	return true
}

func sortedSetDifference(left, right map[string]struct{}) []string {
	result := make([]string, 0)
	for value := range left {
		if _, exists := right[value]; !exists {
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

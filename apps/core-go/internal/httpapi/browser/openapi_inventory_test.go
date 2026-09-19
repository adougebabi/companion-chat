package browser

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

func TestBrowserRouteMatrixMatchesOpenAPIArtifact(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller unavailable")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", "..", "..", ".."))
	data, err := os.ReadFile(filepath.Join(root, "packages", "browser-client", "openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
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
		matched := ""
		for template, operations := range document.Paths {
			if _, exists := operations[strings.ToLower(route.method)]; exists && concretePathMatchesTemplate(route.path, template) {
				if matched != "" {
					t.Fatalf("route %s %s matched both %s and %s", route.method, route.path, matched, template)
				}
				matched = template
			}
		}
		if matched == "" {
			t.Fatalf("route %s %s is missing from browser OpenAPI", route.method, route.path)
		}
		got[strings.ToUpper(route.method)+" "+matched] = struct{}{}
	}
	if difference := setDifference(got, want); len(difference) > 0 {
		t.Fatalf("browser route operations not in OpenAPI: %v", difference)
	}
	if difference := setDifference(want, got); len(difference) > 0 {
		t.Fatalf("OpenAPI operations missing from browser route matrix: %v", difference)
	}
}

func concretePathMatchesTemplate(concrete, template string) bool {
	concreteParts := strings.Split(strings.Trim(concrete, "/"), "/")
	templateParts := strings.Split(strings.Trim(template, "/"), "/")
	if len(concreteParts) != len(templateParts) {
		return false
	}
	for index, part := range templateParts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			continue
		}
		if part != concreteParts[index] {
			return false
		}
	}
	return true
}

func setDifference(left, right map[string]struct{}) []string {
	result := make([]string, 0)
	for value := range left {
		if _, exists := right[value]; !exists {
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

package gateway

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/fluctlight/local-ai-companion/apps/gateway-go/internal/config"
)

func TestLiveDoesNotContactCore(t *testing.T) {
	coreCalls := 0
	core := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		coreCalls++
	}))
	defer core.Close()

	server := newTestServer(t, core.URL, nil)
	response := request(server, http.MethodGet, "/health/live", "")

	assertStatus(t, response, http.StatusOK)
	assertBody(t, response, `{"status":"ok","role":"go-public-gateway"}`)
	if coreCalls != 0 {
		t.Fatalf("coreCalls = %d, want 0", coreCalls)
	}
}

func TestReadinessMapsCoreSuccessAndFailure(t *testing.T) {
	core := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/health/ready" {
			t.Errorf("core path = %q, want /health/ready", request.URL.Path)
		}
		response.WriteHeader(http.StatusOK)
	}))
	defer core.Close()

	server := newTestServer(t, core.URL, nil)
	ready := request(server, http.MethodGet, "/health/ready", "")
	assertStatus(t, ready, http.StatusOK)
	assertBody(t, ready, `{"status":"ready","role":"go-public-gateway"}`)

	core.Close()
	unavailable := request(server, http.MethodGet, "/health/ready", "")
	assertStatus(t, unavailable, http.StatusServiceUnavailable)
	assertBody(t, unavailable, `{"status":"unavailable","role":"go-public-gateway"}`)
}

func TestPingForwardsServiceKeyAndCoreResponse(t *testing.T) {
	const serviceKey = "test-service-key"
	core := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/internal/platform/ping" {
			t.Errorf("core path = %q, want /internal/platform/ping", request.URL.Path)
		}
		if got := request.Header.Get(serviceKeyHeader); got != serviceKey {
			t.Errorf("service key = %q, want %q", got, serviceKey)
		}
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(response, `{"status":"ok","role":"api"}`)
	}))
	defer core.Close()

	server := newTestServer(t, core.URL, func(settings *config.Config) {
		settings.CoreServiceKey = serviceKey
	})
	response := request(server, http.MethodGet, "/api/platform/ping", "")

	assertStatus(t, response, http.StatusOK)
	assertBody(t, response, `{"status":"ok","role":"api"}`)
}

func TestPingMapsCoreTransportFailure(t *testing.T) {
	server := newTestServer(t, "http://127.0.0.1:1", nil)
	response := request(server, http.MethodGet, "/api/platform/ping", "")

	assertStatus(t, response, http.StatusBadGateway)
	assertBody(t, response, `{"code":"core_unavailable","message":"Core platform is unavailable"}`)
}

func TestUnknownPathIsNotProxied(t *testing.T) {
	coreCalls := 0
	core := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		coreCalls++
	}))
	defer core.Close()

	server := newTestServer(t, core.URL, nil)
	response := request(server, http.MethodGet, "/internal/platform/ping", "")

	assertStatus(t, response, http.StatusNotFound)
	if coreCalls != 0 {
		t.Fatalf("coreCalls = %d, want 0", coreCalls)
	}
}

func TestKnownRouteRejectsNonGet(t *testing.T) {
	server := newTestServer(t, "http://127.0.0.1:1", nil)
	response := request(server, http.MethodPost, "/health/live", "")

	assertStatus(t, response, http.StatusMethodNotAllowed)
	if got := response.Header.Get("Allow"); got != http.MethodGet {
		t.Fatalf("Allow = %q, want GET", got)
	}
}

func newTestServer(t *testing.T, coreURL string, customize func(*config.Config)) *httptest.Server {
	t.Helper()
	parsed, err := url.Parse(coreURL)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	settings := config.Config{CoreBaseURL: parsed, CoreServiceKey: "default-service-key"}
	if customize != nil {
		customize(&settings)
	}
	return httptest.NewServer(NewServer(settings, &http.Client{}).Handler())
}

func request(server *httptest.Server, method string, path string, body string) *http.Response {
	request, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
	if err != nil {
		panic(err)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		panic(err)
	}
	return response
}

func assertStatus(t *testing.T, response *http.Response, want int) {
	t.Helper()
	if response.StatusCode != want {
		t.Fatalf("status = %d, want %d", response.StatusCode, want)
	}
}

func assertBody(t *testing.T, response *http.Response, want string) {
	t.Helper()
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if got := strings.TrimSpace(string(body)); got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

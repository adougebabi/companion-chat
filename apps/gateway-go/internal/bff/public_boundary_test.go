package bff

import (
	"bufio"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPublicHTTPPathThroughBFFToCore(t *testing.T) {
	var (
		sawFluctlight   bool
		sawConversation bool
		sawTurn         bool
		turnServiceKey  string
		turnSession     string
	)

	core := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get(serviceKeyHeader) != "service-key" {
			t.Errorf("Core service key = %q", request.Header.Get(serviceKeyHeader))
		}
		switch {
		case request.URL.Path == "/internal/auth/session":
			if request.Header.Get(humanSessionHeader) == "" {
				writeJSONResponse(response, http.StatusOK, `{"authenticated":false}`)
				return
			}
			writeJSONResponse(response, http.StatusOK, `{"authenticated":true,"actor_id":"owner"}`)
		case request.URL.Path == "/internal/auth/setup":
			writeJSONResponse(response, http.StatusOK, `{"authenticated":true,"actor_id":"owner","session_token":"session-opaque"}`)
		case request.URL.Path == "/internal/fluctlights" && request.Method == http.MethodPost:
			sawFluctlight = true
			if request.Header.Get(humanSessionHeader) != "session-opaque" {
				t.Errorf("Fluctlight session = %q", request.Header.Get(humanSessionHeader))
			}
			writeJSONResponse(response, http.StatusOK, `{"id":"fl-1","name":"Nova","status":"active"}`)
		case request.URL.Path == "/internal/conversations" && request.Method == http.MethodPost:
			sawConversation = true
			if request.Header.Get(humanSessionHeader) != "session-opaque" {
				t.Errorf("conversation session = %q", request.Header.Get(humanSessionHeader))
			}
			writeJSONResponse(response, http.StatusOK, conversationPageJSON)
		case strings.HasSuffix(request.URL.Path, "/turn") && request.Method == http.MethodPost:
			sawTurn = true
			turnServiceKey = request.Header.Get(serviceKeyHeader)
			turnSession = request.Header.Get(humanSessionHeader)
			response.Header().Set("Content-Type", "application/x-ndjson")
			response.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(response, "{\"type\":\"token\",\"turn_id\":\"turn-1\",\"sequence\":0,\"payload\":{\"text\":\"hello\"}}\n")
			if flusher, ok := response.(http.Flusher); ok {
				flusher.Flush()
			}
			_, _ = io.WriteString(response, "{\"type\":\"completed\",\"turn_id\":\"turn-1\",\"sequence\":1,\"payload\":{}}\n")
		default:
			writeJSONResponse(response, http.StatusNotFound, `{"detail":"not found"}`)
		}
	}))
	defer core.Close()

	coreURL, err := url.Parse(core.URL)
	if err != nil {
		t.Fatal(err)
	}
	bff := httptest.NewUnstartedServer(nil)
	origin := "http://" + bff.Listener.Addr().String()
	bff.Config.Handler = New(Options{
		CoreBaseURL:    coreURL,
		CoreServiceKey: "service-key",
		TrustedOrigin:  origin,
		SecureCookies:  false,
		Client:         core.Client(),
	}).Handler()
	bff.Start()
	defer bff.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := bff.Client()
	client.Jar = jar

	do := func(method, path, body string, mutation bool) (*http.Response, []byte) {
		t.Helper()
		request, err := http.NewRequest(method, bff.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if mutation {
			request.Header.Set("Origin", origin)
			request.Header.Set("X-CSRF-Token", cookieFromJar(t, jar, bff.URL, csrfCookieName))
			request.Header.Set("Content-Type", "application/json")
		}
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		return response, data
	}

	response, body := do(http.MethodGet, "/auth/session", "", false)
	if response.StatusCode != http.StatusUnauthorized || !strings.Contains(string(body), `"authenticated":false`) {
		t.Fatalf("anonymous session = %d %s", response.StatusCode, body)
	}
	if csrf := cookieFromJar(t, jar, bff.URL, csrfCookieName); csrf == "" {
		t.Fatal("anonymous session did not establish a CSRF cookie")
	}

	response, body = do(http.MethodPost, "/auth/setup", `{"setupToken":"setup-token-123456","password":"long-enough-password"}`, true)
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), `"authenticated":true`) {
		t.Fatalf("setup = %d %s", response.StatusCode, body)
	}
	if session := cookieFromJar(t, jar, bff.URL, sessionCookieName); session != "session-opaque" {
		t.Fatalf("session cookie = %q", session)
	}

	response, body = do(http.MethodGet, "/auth/session", "", false)
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), `"actorId":"owner"`) {
		t.Fatalf("authenticated session = %d %s", response.StatusCode, body)
	}

	response, body = do(http.MethodPost, "/api/fluctlights", `{"name":"Nova"}`, true)
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), `"id":"fl-1"`) {
		t.Fatalf("Fluctlight create = %d %s", response.StatusCode, body)
	}
	response, body = do(http.MethodPost, "/api/conversations", `{"participantActorIds":["fl-1"]}`, true)
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), `"conversation"`) {
		t.Fatalf("conversation create = %d %s", response.StatusCode, body)
	}
	response, body = do(http.MethodPost, "/api/conversations/conversation-1/turn", `{"text":"hello","fluctlightId":"fl-1","idempotencyKey":"turn-1"}`, true)
	if response.StatusCode != http.StatusOK || !strings.Contains(response.Header.Get("Content-Type"), "application/x-ndjson") {
		t.Fatalf("turn status/content type = %d %q", response.StatusCode, response.Header.Get("Content-Type"))
	}
	if !strings.Contains(string(body), `"turnId":"turn-1"`) || !strings.Contains(string(body), `"type":"completed"`) {
		t.Fatalf("browser turn stream = %s", body)
	}
	if !sawFluctlight || !sawConversation || !sawTurn {
		t.Fatalf("public chain calls = fluctlight:%t conversation:%t turn:%t", sawFluctlight, sawConversation, sawTurn)
	}
	if turnServiceKey != "service-key" || turnSession != "session-opaque" {
		t.Fatalf("turn identity headers = service:%q session:%q", turnServiceKey, turnSession)
	}
}

func TestPublicDisconnectCancelsCoreStreamAndStopsTranslation(t *testing.T) {
	body := &blockingStreamBody{
		first:  []byte("{\"type\":\"token\",\"turn_id\":\"turn-1\",\"sequence\":0,\"payload\":{\"text\":\"hello\"}}\n"),
		closed: make(chan struct{}),
	}
	upstreamCanceled := make(chan struct{})
	base, err := url.Parse("http://core.invalid")
	if err != nil {
		t.Fatal(err)
	}
	bff := httptest.NewUnstartedServer(nil)
	origin := "http://" + bff.Listener.Addr().String()
	bff.Config.Handler = New(Options{
		CoreBaseURL:    base,
		CoreServiceKey: "service-key",
		TrustedOrigin:  origin,
		SecureCookies:  false,
		Client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			go func() {
				<-request.Context().Done()
				close(upstreamCanceled)
			}()
			return &http.Response{
				StatusCode:    http.StatusOK,
				ContentLength: -1,
				Header:        http.Header{"Content-Type": []string{"application/x-ndjson"}},
				Body:          body,
			}, nil
		})},
	}).Handler()
	bff.Start()
	defer bff.Close()

	request, err := http.NewRequest(http.MethodPost, bff.URL+"/api/conversations/conversation-1/turn", strings.NewReader(`{"text":"hello","fluctlightId":"fl-1","idempotencyKey":"turn-1"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Origin", origin)
	request.Header.Set("X-CSRF-Token", "csrf")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "opaque"})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf"})
	response, err := bff.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(response.Body)
	line, err := reader.ReadBytes('\n')
	if err != nil || !strings.Contains(string(line), `"turnId":"turn-1"`) {
		t.Fatalf("first streamed frame = %q, err = %v", line, err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatalf("close browser response: %v", err)
	}

	waitForClosed(t, body.closed, "Core stream body close")
	waitForClosed(t, upstreamCanceled, "Core request cancellation")
}

type blockingStreamBody struct {
	mu     sync.Mutex
	first  []byte
	closed chan struct{}
	once   sync.Once
}

func (body *blockingStreamBody) Read(buffer []byte) (int, error) {
	body.mu.Lock()
	if len(body.first) > 0 {
		count := copy(buffer, body.first)
		body.first = body.first[count:]
		body.mu.Unlock()
		return count, nil
	}
	body.mu.Unlock()
	<-body.closed
	return 0, errors.New("stream closed")
}

func (body *blockingStreamBody) Close() error {
	body.once.Do(func() { close(body.closed) })
	return nil
}

func waitForClosed(t *testing.T, signal <-chan struct{}, label string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for %s", label)
	}
}

func cookieFromJar(t *testing.T, jar http.CookieJar, rawURL, name string) string {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	for _, cookie := range jar.Cookies(parsed) {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

func writeJSONResponse(response http.ResponseWriter, status int, body string) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_, _ = io.WriteString(response, body)
}

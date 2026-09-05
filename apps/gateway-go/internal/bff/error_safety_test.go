package bff

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPublicCoreErrorDetailsAreBoundedAndRedacted(t *testing.T) {
	longValue := strings.Repeat("x", maxPublicErrorString+100)
	handler := testBFF(t, func(request *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusUnprocessableEntity, `{"detail":{"code":"foundation_invalid","message":"provider response must stay private","details":{"field":"name","nested":{"token":"secret-token","safe":"`+longValue+`"},"raw_response":"provider output"}}}`), nil
	})

	response := invoke(handler, http.MethodPost, "http://gateway.test/api/fluctlight-creations/analysis", `{"description":"make one"}`, map[string]string{
		"Origin":       "https://fluctlight.local",
		"X-CSRF-Token": "csrf",
	}, map[string]string{
		sessionCookieName: "opaque",
		csrfCookieName:    "csrf",
	})
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "secret-token") || strings.Contains(response.Body.String(), "provider output") {
		t.Fatalf("sensitive details leaked: %s", response.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != "Fluctlight analysis was rejected" {
		t.Fatalf("unstable Core message crossed boundary: %#v", payload["message"])
	}
	details, ok := payload["details"].(map[string]any)
	if !ok || details["field"] != "name" {
		t.Fatalf("safe validation detail missing: %#v", payload["details"])
	}
	nested, ok := details["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested safe details missing: %#v", details)
	}
	if _, exists := nested["token"]; exists {
		t.Fatalf("nested token was not redacted: %#v", nested)
	}
	if got := nested["safe"].(string); len([]rune(got)) != maxPublicErrorString {
		t.Fatalf("long detail length = %d, want %d", len([]rune(got)), maxPublicErrorString)
	}
}

func TestMediaNilBodyReturnsBoundedError(t *testing.T) {
	handler := testBFF(t, func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, ContentLength: 0, Body: http.NoBody, Header: http.Header{}}, nil
	})
	response := invoke(handler, http.MethodGet, "http://gateway.test/api/media/asset-1", "", nil, map[string]string{sessionCookieName: "opaque"})
	if response.Code != http.StatusBadGateway || !strings.Contains(response.Body.String(), `"media_unavailable"`) {
		t.Fatalf("nil media body = %d %s", response.Code, response.Body.String())
	}
}

func TestMediaRetryForwardsBoundedFailureReason(t *testing.T) {
	handler := testBFF(t, func(request *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusConflict, `{"detail":{"code":"diagnostics_media_retry_failed","message":"diagnostics media retry failed","details":{"reason":"workflow restart failed: workflow is still running"}}}`), nil
	})
	response := invoke(handler, http.MethodPost, "http://gateway.test/api/diagnostics/media-prompts/media-1/retry", "{}", map[string]string{
		"Origin":       "https://fluctlight.local",
		"X-CSRF-Token": "csrf",
	}, map[string]string{
		sessionCookieName: "opaque",
		csrfCookieName:    "csrf",
	})
	if response.Code != http.StatusConflict {
		t.Fatalf("retry status = %d, body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "workflow restart failed: workflow is still running") {
		t.Fatalf("retry reason missing: %s", response.Body.String())
	}
}

func TestMediaStreamsRangeAndAllowListedHeaders(t *testing.T) {
	handler := testBFF(t, func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Range") != "bytes=0-3" {
			t.Fatalf("Core Range = %q", request.Header.Get("Range"))
		}
		headers := make(http.Header)
		headers.Set("Content-Type", "video/mp4")
		headers.Set("Content-Range", "bytes 0-3/8")
		headers.Set("Accept-Ranges", "bytes")
		headers.Set("ETag", `"asset-v1"`)
		headers.Set("X-Internal-Key", "must-not-leak")
		return &http.Response{
			StatusCode:    http.StatusPartialContent,
			ContentLength: 4,
			Header:        headers,
			Body:          io.NopCloser(strings.NewReader("data")),
		}, nil
	})
	response := invoke(handler, http.MethodGet, "http://gateway.test/api/media/asset-1", "", map[string]string{"Range": "bytes=0-3"}, map[string]string{sessionCookieName: "opaque"})
	if response.Code != http.StatusPartialContent || response.Body.String() != "data" {
		t.Fatalf("media response = %d %q", response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Range") != "bytes 0-3/8" || response.Header().Get("ETag") != `"asset-v1"` {
		t.Fatalf("media headers = %#v", response.Header())
	}
	if response.Header().Get("X-Internal-Key") != "" {
		t.Fatalf("internal media header leaked: %#v", response.Header())
	}
}

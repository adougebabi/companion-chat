package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

const providerHTTPErrorDetailLimit = 4096

// providerHTTPResponseError is deliberately independent of the SDK's APIError.
// The pinned Eino adapter otherwise unwraps a partially decoded APIError and
// loses the enclosing RequestError's real HTTP status and malformed body.
type providerHTTPResponseError struct {
	StatusCode int
	Detail     string
	Truncated  bool
	readErr    error
}

func (e *providerHTTPResponseError) Error() string {
	message := fmt.Sprintf("provider HTTP %d", e.StatusCode)
	if e.Detail != "" {
		message += ": " + e.Detail
	} else {
		message += ": empty error body"
	}
	if e.Truncated {
		message += " [truncated]"
	}
	if e.readErr != nil {
		// Retain the cause for errors.Is/As without leaking arbitrary reader text.
		message += " [error body read failed]"
	}
	return message
}

func (e *providerHTTPResponseError) Unwrap() error { return e.readErr }

func preserveProviderHTTPError(request *http.Request, response *http.Response) error {
	var body []byte
	var readErr error
	if response.Body != nil {
		body, readErr = io.ReadAll(io.LimitReader(response.Body, providerHTTPErrorDetailLimit+1))
		_ = response.Body.Close()
	}
	truncated := len(body) > providerHTTPErrorDetailLimit
	// Sanitize before applying the display bound; malformed/truncated credential
	// fields redact their entire remaining tail rather than expose a partial value.
	detail := sanitizeProviderHTTPErrorDetail(request, string(body), truncated || readErr != nil)
	if len(detail) > providerHTTPErrorDetailLimit {
		detail = detail[:providerHTTPErrorDetailLimit]
		truncated = true
	}
	detail = strings.ToValidUTF8(detail, "")
	return &providerHTTPResponseError{StatusCode: response.StatusCode, Detail: strings.TrimSpace(detail), Truncated: truncated, readErr: readErr}
}

var providerErrorCredentialTail = regexp.MustCompile(`(?i)(?:"?(?:authorization|proxy[-_ ]?authorization|(?:x[-_ ]?)?api[-_ ]?key|access[-_ ]?token|refresh[-_ ]?token|token|password|(?:client[-_ ]?)?secret|credential[s]?|cookie|set[-_ ]?cookie|session(?:[-_ ]?key)?|service[-_ ]?key)"?\s*[:=]|bearer\s+|sk-)[\s\S]*`)

func providerErrorCredentialField(key string) bool {
	key = strings.ToLower(strings.NewReplacer("-", "", "_", "", " ", "").Replace(key))
	switch key {
	case "authorization", "proxyauthorization", "apikey", "xapikey", "accesstoken", "refreshtoken", "token", "password", "secret", "clientsecret", "credential", "credentials", "cookie", "setcookie", "session", "sessionkey", "servicekey":
		return true
	}
	return false
}

func sanitizeProviderHTTPErrorDetail(request *http.Request, detail string, incomplete bool) string {
	// Cover actual configured credentials even when a provider echoes them in
	// an unlabelled sentence. Nothing from the request body is inspected.
	for key, values := range request.Header {
		if !providerErrorCredentialField(key) {
			continue
		}
		for _, value := range values {
			candidates := []string{value}
			if strings.EqualFold(key, "Authorization") || strings.EqualFold(key, "Proxy-Authorization") {
				if _, token, ok := strings.Cut(value, " "); ok {
					candidates = append(candidates, strings.TrimSpace(token))
				}
			}
			for _, candidate := range candidates {
				if candidate == "" {
					continue
				}
				encoded, _ := json.Marshal(candidate)
				for _, representation := range []string{candidate, string(encoded[1 : len(encoded)-1])} {
					if incomplete {
						// A bounded read may end within either a raw or JSON-escaped key.
						for n := min(len(representation), len(detail)); n > 0; n-- {
							if strings.HasSuffix(detail, representation[:n]) {
								detail = detail[:len(detail)-n] + "[REDACTED]"
								break
							}
						}
					}
					detail = strings.ReplaceAll(detail, representation, "[REDACTED]")
				}
			}
		}
	}
	var decoded any
	if json.Unmarshal([]byte(detail), &decoded) == nil {
		changed := false
		var redact func(any) any
		redact = func(value any) any {
			switch v := value.(type) {
			case map[string]any:
				for key, child := range v {
					if providerErrorCredentialField(key) {
						v[key], changed = "[REDACTED]", true
					} else {
						v[key] = redact(child)
					}
				}
			case []any:
				for i, child := range v {
					v[i] = redact(child)
				}
			case string:
				redacted := providerErrorCredentialTail.ReplaceAllString(v, "[REDACTED]")
				if redacted != v {
					changed = true
				}
				return redacted
			}
			return value
		}
		decoded = redact(decoded)
		if changed {
			encoded, _ := json.Marshal(decoded)
			detail = string(encoded)
		}
	} else {
		detail = providerErrorCredentialTail.ReplaceAllString(detail, "[REDACTED]")
	}
	// Existing diagnostics redaction also removes image payloads.
	return redactDiagnostic(detail).(string)
}

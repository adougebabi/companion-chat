package browser

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/config"
)

const (
	sessionCookieName = "fluctlight_session"
	csrfCookieName    = "fluctlight_csrf"
	csrfMaxAge        = 24 * 60 * 60
)

// commonResponseWriter delays the first header write long enough to attach a
// CSRF cookie to every response. The route may call issueCSRFCookie itself
// (login/logout/password rotation); the wrapper tracks that to avoid duplicate
// cookies.
type commonResponseWriter struct {
	http.ResponseWriter
	secure      bool
	request     *http.Request
	wroteHeader bool
	csrfIssued  bool
}

func (w *commonResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	if w.request != nil {
		if _, err := w.request.Cookie(csrfCookieName); err != nil {
			w.issueCSRF()
		}
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *commonResponseWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func (w *commonResponseWriter) Flush() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *commonResponseWriter) issueCSRF() {
	if w.csrfIssued {
		return
	}
	w.csrfIssued = true
	setCSRFCookie(w.ResponseWriter, newCSRFToken(), w.secure)
}

func newCSRFToken() string {
	var bytes [32]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		panic(fmt.Errorf("generate CSRF token: %w", err))
	}
	return base64.RawURLEncoding.EncodeToString(bytes[:])
}

func setCSRFCookie(response http.ResponseWriter, value string, secure bool) {
	if writer, ok := response.(*commonResponseWriter); ok {
		writer.csrfIssued = true
		response = writer.ResponseWriter
	}
	http.SetCookie(response, &http.Cookie{
		Name:     csrfCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   csrfMaxAge,
		Secure:   secure,
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	})
}

func setSessionCookie(response http.ResponseWriter, value string, secure bool) {
	http.SetCookie(response, &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		Secure:   secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(response http.ResponseWriter, secure bool) {
	http.SetCookie(response, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Secure:   secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func cookieValue(request *http.Request, name string) string {
	cookie, err := request.Cookie(name)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func csrfValid(request *http.Request, trustedOrigin string) bool {
	if !originAllowed(request.Header.Get("Origin"), trustedOrigin) {
		return false
	}
	cookie := cookieValue(request, csrfCookieName)
	header := request.Header.Get("X-CSRF-Token")
	if cookie == "" || header == "" {
		return false
	}
	left, right := []byte(cookie), []byte(header)
	if len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare(left, right) == 1
}

func corsHeaders(response http.ResponseWriter, request *http.Request, trustedOrigin string) {
	origin := request.Header.Get("Origin")
	if originAllowed(origin, trustedOrigin) {
		response.Header().Set("Access-Control-Allow-Origin", origin)
		response.Header().Set("Access-Control-Allow-Credentials", "true")
		response.Header().Add("Vary", "Origin")
	}
}

func optionsResponse(response http.ResponseWriter, request *http.Request, trustedOrigin string) {
	if !originAllowed(request.Header.Get("Origin"), trustedOrigin) {
		writeError(response, http.StatusForbidden, "invalid_origin", "Origin is not allowed")
		return
	}
	response.Header().Set("Access-Control-Allow-Origin", request.Header.Get("Origin"))
	response.Header().Set("Access-Control-Allow-Credentials", "true")
	response.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
	response.Header().Set("Access-Control-Allow-Headers", "content-type,range,x-csrf-token")
	response.WriteHeader(http.StatusNoContent)
}

func mutationGuard(response http.ResponseWriter, request *http.Request, trustedOrigin string) bool {
	if csrfValid(request, trustedOrigin) {
		return true
	}
	writeError(response, http.StatusForbidden, "invalid_origin", "Origin is not allowed")
	return false
}

func normalizeOrigin(value string) string { return strings.TrimSpace(value) }

func originAllowed(origin, trusted string) bool {
	if origin == "" || origin == "null" {
		return false
	}
	allowed, err := config.ParseTrustedOrigins(trusted)
	if err != nil {
		return false
	}
	for _, candidate := range allowed {
		if origin == candidate {
			return true
		}
	}
	return false
}

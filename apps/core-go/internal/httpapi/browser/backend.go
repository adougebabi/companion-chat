package browser

import (
	"context"
	"fmt"
	"net/http"
)

// Backend is the explicit in-process application boundary for browser HTTP.
// It intentionally exposes operations rather than URLs and owns no HTTP
// client, service-key forwarding, storage handle, or workflow implementation.
type Backend interface {
	Health(context.Context) error
	// DoJSON/DoAny/DoValue receive route-shaped operation keys solely so the
	// browser route inventory and direct App mapping stay auditable. A Backend
	// implementation must never interpret them as URLs or issue an HTTP call.
	DoJSON(context.Context, string, string, string, any) (map[string]any, error)
	DoAny(context.Context, string, string, string, any) (any, error)
	DoValue(context.Context, string, string, string, any, any) error
	StreamTurn(context.Context, string, string, map[string]any, http.ResponseWriter) error
	Media(context.Context, string, string, string, http.ResponseWriter) error
}

// CoreError is a bounded application error used only to select the public
// browser status/code mapping. Its message and details are never copied to
// the browser without the explicit sanitizer in this package.
type CoreError struct {
	Status  int
	Code    string
	Message string
	Details map[string]any
}

func (e *CoreError) Error() string {
	if e == nil {
		return "core operation failed"
	}
	if e.Message != "" {
		return fmt.Sprintf("core operation failed: %d %s: %s", e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("core operation failed: %d %s", e.Status, e.Code)
}

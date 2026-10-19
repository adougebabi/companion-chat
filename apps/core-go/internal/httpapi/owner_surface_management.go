package httpapi

import (
	"errors"
	"net/http"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
)

func (s *Server) ownerSurfaceManagement(w http.ResponseWriter, r *http.Request) {
	owner, ok := s.authorizeHuman(w, r)
	if !ok {
		return
	}
	body := map[string]any{}
	if r.Method != http.MethodGet {
		var valid bool
		body, valid = s.body(w, r)
		if !valid {
			return
		}
	}
	var value map[string]any
	var err error
	if core.MatchStudioJobSurface(r.Method, r.URL.Path) {
		value, err = s.app.DispatchStudioJobSurface(r.Context(), r.Method, r.URL.Path, r.URL.Query(), owner, body)
	} else if core.MatchWorldSurface(r.Method, r.URL.Path) {
		value, err = s.app.DispatchWorldSurface(r.Context(), r.Method, r.URL.Path, r.URL.Query(), owner, body)
	} else {
		value, err = s.app.DispatchOwnerSurface(r.Context(), r.Method, r.URL.Path, r.URL.Query(), owner, body)
	}
	if err != nil {
		var surface *core.SurfaceError
		if errors.As(err, &surface) {
			writeErrorDetails(w, surface.Status, surface.Code, surface.Details)
		} else {
			s.opError(w, err, "owner_surface_operation_failed")
		}
		return
	}
	status := 200
	if value["_status"] != nil {
		status = intValueHTTP(value["_status"])
		delete(value, "_status")
	}
	delete(value, "_replay")
	writeJSON(w, status, value)
}

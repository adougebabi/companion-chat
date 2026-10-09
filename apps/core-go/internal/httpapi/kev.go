package httpapi

import (
	"net/http"
)

func (s *Server) kevDecisions(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	filter := map[string]string{}
	for key, values := range r.URL.Query() {
		if len(values) > 0 {
			filter[key] = values[0]
		}
	}
	page, err := s.app.KevDecisionsPage(r.Context(), actor, filter)
	if err != nil {
		s.opError(w, err, "kev_diagnostics_failed")
		return
	}
	writeJSON(w, http.StatusOK, page)
}
func (s *Server) kevConnection(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	value, err := s.app.TestKevConnection(r.Context(), actor)
	if err != nil {
		s.opError(w, err, "kev_connection_failed")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

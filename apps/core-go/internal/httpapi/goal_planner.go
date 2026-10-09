package httpapi

import (
	"encoding/json"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"net/http"
)

func (s *Server) goalSet(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	owner := r.PathValue("fluctlightID")
	var value map[string]any
	var err error
	if r.Method == http.MethodGet {
		value, err = s.app.GoalSetSnapshot(r.Context(), actor, owner, "*")
	} else {
		var cmd core.GoalSetCommand
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024))
		d.DisallowUnknownFields()
		if d.Decode(&cmd) != nil {
			s.opError(w, core.ErrInvalidArguments, "goal_set_invalid")
			return
		}
		value, err = s.app.ApplyGoalSetCommand(r.Context(), actor, owner, cmd)
	}
	if err != nil {
		s.opError(w, err, "goal_set_failed")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) goalPlanning(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	owner := r.PathValue("fluctlightID")
	var value any
	var err error
	if r.Method == http.MethodGet {
		value, err = s.app.GoalPlanningHistory(r.Context(), actor, owner)
	} else {
		var body map[string]any
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body) != nil {
			s.opError(w, core.ErrInvalidArguments, "goal_planning_invalid")
			return
		}
		value, err = s.app.RequestGoalPlanning(r.Context(), actor, owner, stringValue(body["idempotency_key"]))
	}
	if err != nil {
		s.opError(w, err, "goal_planning_failed")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) actorContext(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	owner, target := r.PathValue("fluctlightID"), r.PathValue("actorID")
	var value map[string]any
	var err error
	if r.Method == http.MethodGet {
		value, err = s.app.ActorContext(r.Context(), actor, owner, target)
	} else {
		var body map[string]any
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&body) != nil {
			s.opError(w, core.ErrInvalidArguments, "actor_context_invalid")
			return
		}
		value, err = s.app.UpdateActorContext(r.Context(), actor, owner, target, body)
	}
	if err != nil {
		s.opError(w, err, "actor_context_failed")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

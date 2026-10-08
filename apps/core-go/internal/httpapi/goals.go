package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
)

func (s *Server) goals(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	limit := 20
	if value := r.URL.Query().Get("limit"); value != "" {
		var err error
		limit, err = strconv.Atoi(value)
		if err != nil {
			s.opError(w, core.ErrInvalidArguments, "goal_query_invalid")
			return
		}
	}
	page, err := s.app.ListGoals(r.Context(), actor, r.PathValue("fluctlightID"), r.URL.Query().Get("history") == "true", limit, r.URL.Query().Get("cursor"))
	if err != nil {
		s.opError(w, err, "goal_list_failed")
		return
	}
	writeJSON(w, http.StatusOK, page)
}
func (s *Server) goalDetail(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	result, err := s.app.GoalDetail(r.Context(), actor, r.PathValue("fluctlightID"), r.PathValue("goalID"))
	if err != nil {
		s.opError(w, err, "goal_detail_failed")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s *Server) goalHistory(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	limit := 20
	if value := r.URL.Query().Get("limit"); value != "" {
		var err error
		limit, err = strconv.Atoi(value)
		if err != nil {
			s.opError(w, core.ErrInvalidArguments, "goal_query_invalid")
			return
		}
	}
	result, err := s.app.GoalHistory(r.Context(), actor, r.PathValue("fluctlightID"), r.PathValue("goalID"), limit, r.URL.Query().Get("cursor"))
	if err != nil {
		s.opError(w, err, "goal_history_failed")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s *Server) goalCommand(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	var command core.GoalOwnerCommand
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&command); err != nil {
		s.opError(w, core.ErrInvalidArguments, "goal_command_invalid")
		return
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		s.opError(w, core.ErrInvalidArguments, "goal_command_invalid")
		return
	}
	operation := r.PathValue("operation")
	if operation == "" {
		operation = "update"
		if r.Method == http.MethodPost {
			operation = "create"
		}
	}
	switch operation {
	case "create", "update", "pause", "resume", "cancel", "abandon", "reassess":
	default:
		s.opError(w, core.ErrInvalidArguments, "goal_operation_invalid")
		return
	}
	if command.Operation != "" && command.Operation != operation {
		s.opError(w, core.ErrInvalidArguments, "goal_operation_invalid")
		return
	}
	command.Operation = operation
	result, err := s.app.ApplyOwnerGoalCommand(r.Context(), actor, r.PathValue("fluctlightID"), r.PathValue("goalID"), command)
	if err != nil {
		s.opError(w, err, "goal_command_failed")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) goalEvidence(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	limit := 20
	if value := r.URL.Query().Get("limit"); value != "" {
		var err error
		limit, err = strconv.Atoi(value)
		if err != nil {
			s.opError(w, core.ErrInvalidArguments, "goal_query_invalid")
			return
		}
	}
	result, err := s.app.GoalEvidence(r.Context(), actor, r.PathValue("fluctlightID"), r.PathValue("goalID"), limit, r.URL.Query().Get("cursor"))
	if err != nil {
		s.opError(w, err, "goal_evidence_failed")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

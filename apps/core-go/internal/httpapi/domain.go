package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
)

func (s *Server) body(response http.ResponseWriter, request *http.Request) (map[string]any, bool) {
	v, ok := readJSON(request)
	if !ok {
		writeError(response, http.StatusUnprocessableEntity, "core_request_validation_failed")
	}
	return v, ok
}
func expected(body map[string]any, key string) *int {
	if value, ok := body[key]; ok && value != nil {
		v := intValueHTTP(value)
		return &v
	}
	return nil
}

func intValueHTTP(value any) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		i, _ := v.Int64()
		return int(i)
	case string:
		i, _ := strconv.Atoi(v)
		return i
	default:
		return 0
	}
}
func (s *Server) opError(response http.ResponseWriter, err error, code string) {
	status := http.StatusUnprocessableEntity
	if errors.Is(err, core.ErrNotFound) {
		status = http.StatusNotFound
	}
	if errors.Is(err, core.ErrUnauthorized) {
		status = http.StatusForbidden
	}
	writeError(response, status, code)
}

func (s *Server) revokeAll(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	if err := s.app.RevokeAll(r.Context(), actor); err != nil {
		s.opError(w, err, "auth_revoke_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) revokeCurrent(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	if err := s.app.RevokeCurrent(r.Context(), actor, r.Header.Get(humanSessionHeader)); err != nil {
		s.opError(w, err, "auth_revoke_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) resetPassword(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	body, valid := s.body(w, r)
	if !valid {
		return
	}
	if err := s.app.ResetPassword(r.Context(), actor, stringValue(body["password"])); err != nil {
		s.opError(w, err, "password_change_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) configureProviderRole(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	body, valid := s.body(w, r)
	if !valid {
		return
	}
	if err := s.app.ConfigureProviderRole(r.Context(), actor, body); err != nil {
		s.opError(w, err, "provider_preflight_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"role": body["role"], "status": "available"})
}
func (s *Server) actorGroups(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	value, err := s.app.ActorGroups(r.Context(), actor)
	if err != nil {
		s.opError(w, err, "actor_groups_unavailable")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) createActorGroup(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	body, valid := s.body(w, r)
	if !valid {
		return
	}
	value, err := s.app.CreateActorGroup(r.Context(), actor, stringValue(body["name"]))
	if err != nil {
		s.opError(w, err, "actor_group_create_failed")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) addActorGroupMember(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	if err := s.app.SetActorGroupMember(r.Context(), actor, r.PathValue("groupID"), stringValue(mapValueMust(r)["actor_id"]), true); err != nil {
		s.opError(w, err, "actor_group_assign_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) removeActorGroupMember(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	if err := s.app.SetActorGroupMember(r.Context(), actor, r.PathValue("groupID"), r.PathValue("actorID"), false); err != nil {
		s.opError(w, err, "actor_group_remove_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func mapValueMust(r *http.Request) map[string]any { v, _ := readJSON(r); return v }
func (s *Server) createConversation(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	body, valid := s.body(w, r)
	if !valid {
		return
	}
	value, err := s.app.CreateConversation(r.Context(), actor, arrayValue(body["participant_actor_ids"]), stringValue(body["title"]))
	if err != nil {
		s.opError(w, err, "conversation_create_failed")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) markRead(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	body, valid := s.body(w, r)
	if !valid {
		return
	}
	if err := s.app.MarkRead(r.Context(), actor, r.PathValue("conversationID"), body); err != nil {
		s.opError(w, err, "conversation_read_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) setFluctlightStatus(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	body, valid := s.body(w, r)
	if !valid {
		return
	}
	value, err := s.app.SetFluctlightStatus(r.Context(), actor, r.PathValue("fluctlightID"), stringValue(body["status"]), stringValue(body["reason"]), expected(body, "expected_revision"))
	if err != nil {
		s.opError(w, err, "fluctlight_status_failed")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) retireFluctlight(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	body, _ := readJSON(r)
	value, err := s.app.RetireFluctlight(r.Context(), actor, r.PathValue("fluctlightID"), stringValue(body["reason"]), expected(body, "expected_revision"))
	if err != nil {
		s.opError(w, err, "fluctlight_retire_failed")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) proposeFoundation(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	body, valid := s.body(w, r)
	if !valid {
		return
	}
	value, err := s.app.ProposeFoundation(r.Context(), actor, r.PathValue("fluctlightID"), body)
	if err != nil {
		s.opError(w, err, "foundation_revision_failed")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) acceptFoundation(w http.ResponseWriter, r *http.Request) {
	s.foundationDecision(w, r, "accept")
}
func (s *Server) rejectFoundation(w http.ResponseWriter, r *http.Request) {
	s.foundationDecision(w, r, "reject")
}
func (s *Server) foundationDecision(w http.ResponseWriter, r *http.Request, action string) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	body, _ := readJSON(r)
	value, err := s.app.SetFoundationDecision(r.Context(), actor, r.PathValue("fluctlightID"), r.PathValue("revisionID"), action, stringValue(body["reason"]))
	if err != nil {
		s.opError(w, err, "foundation_revision_"+action+"_failed")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) rollbackFoundation(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	body, _ := readJSON(r)
	value, err := s.app.SetFoundationDecision(r.Context(), actor, r.PathValue("fluctlightID"), stringValue(body["revision_id"]), "accept", stringValue(body["reason"]))
	if err != nil {
		s.opError(w, err, "foundation_revision_rollback_failed")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) reviseMemory(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	body, valid := s.body(w, r)
	if !valid {
		return
	}
	value, err := s.app.ReviseMemory(r.Context(), actor, r.PathValue("memoryID"), stringValue(body["content"]), expected(body, "expected_revision"), arrayValue(body["evidence_refs"]))
	if err != nil {
		s.opError(w, err, "memory_revision_failed")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) forgetMemory(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	body, _ := readJSON(r)
	value, err := s.app.ForgetMemory(r.Context(), actor, r.PathValue("memoryID"), expected(body, "expected_revision"), arrayValue(body["evidence_refs"]))
	if err != nil {
		s.opError(w, err, "memory_forget_failed")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) rollbackRelationship(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	body, _ := readJSON(r)
	value, err := s.app.RollbackRelationship(r.Context(), actor, r.PathValue("fluctlightID"), body)
	if err != nil {
		s.opError(w, err, "relationship_rollback_failed")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) governAutonomy(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	body, _ := readJSON(r)
	value, err := s.app.GovernAutonomy(r.Context(), actor, r.PathValue("actionID"), stringValue(body["to_status"]), stringValue(body["reason"]))
	if err != nil {
		s.opError(w, err, "autonomy_governance_failed")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) createEvent(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	body, valid := s.body(w, r)
	if !valid {
		return
	}
	value, err := s.app.CreateLifeEvent(r.Context(), actor, r.PathValue("fluctlightID"), body)
	if err != nil {
		s.opError(w, err, "life_event_failed")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) cancelEvent(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	if err := s.app.CancelLifeEvent(r.Context(), actor, r.PathValue("fluctlightID"), r.PathValue("eventID")); err != nil {
		s.opError(w, err, "life_event_cancel_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) setPresence(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	body, valid := s.body(w, r)
	if !valid {
		return
	}
	value, err := s.app.SetPresence(r.Context(), actor, r.PathValue("fluctlightID"), body)
	if err != nil {
		s.opError(w, err, "life_presence_failed")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) cancelSchedule(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	if err := s.app.CancelSchedule(r.Context(), actor, r.PathValue("fluctlightID"), r.PathValue("scheduleID")); err != nil {
		s.opError(w, err, "schedule_cancel_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) allMoments(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	value, err := s.app.AllMoments(r.Context(), actor, r.URL.Query().Get("include_hidden") == "true", queryLimit(r))
	if err != nil {
		s.opError(w, err, "moments_unavailable")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) markMomentsRead(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	if err := s.app.MarkMomentsRead(r.Context(), actor, r.PathValue("fluctlightID")); err != nil {
		s.opError(w, err, "moment_read_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) commentMoment(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	body, valid := s.body(w, r)
	if !valid {
		return
	}
	value, err := s.app.CommentMoment(r.Context(), actor, r.PathValue("momentID"), stringValue(body["text"]))
	if err != nil {
		s.opError(w, err, "moment_comment_failed")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) reactMoment(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	body, _ := readJSON(r)
	value, err := s.app.ReactMoment(r.Context(), actor, r.PathValue("momentID"), firstString(body["kind"], "like"))
	if err != nil {
		s.opError(w, err, "moment_reaction_failed")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) hideMoment(w http.ResponseWriter, r *http.Request) {
	s.setMomentStatus(w, r, "hidden")
}
func (s *Server) restoreMoment(w http.ResponseWriter, r *http.Request) {
	s.setMomentStatus(w, r, "visible")
}
func (s *Server) setMomentStatus(w http.ResponseWriter, r *http.Request, status string) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	if err := s.app.SetMomentStatus(r.Context(), actor, r.PathValue("momentID"), status); err != nil {
		s.opError(w, err, "moment_status_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) diagnostics(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	value, err := s.app.Diagnostics(r.Context(), actor, queryLimit(r))
	if err != nil {
		s.opError(w, err, "diagnostics_unavailable")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) clearDiagnostics(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	if err := s.app.ClearDiagnostics(r.Context(), actor); err != nil {
		s.opError(w, err, "diagnostics_clear_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) modelRuns(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	value, err := s.app.ModelRuns(r.Context(), actor, queryLimit(r))
	if err != nil {
		s.opError(w, err, "diagnostics_model_runs_failed")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) exportDiagnostics(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	value, err := s.app.DiagnosticsExport(r.Context(), actor)
	if err != nil {
		s.opError(w, err, "diagnostics_export_failed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(value)
}
func (s *Server) workflowList(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	value, err := s.app.WorkflowList(r.Context(), actor)
	if err != nil {
		s.opError(w, err, "workflow_runtime_unavailable")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) workflowStatus(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	value, err := s.app.WorkflowStatus(r.Context(), actor, r.PathValue("workflowID"))
	if err != nil {
		s.opError(w, err, "workflow_status_failed")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) workflowHistory(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	value, err := s.app.WorkflowHistory(r.Context(), actor, r.PathValue("workflowID"))
	if err != nil {
		s.opError(w, err, "workflow_history_failed")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) workflowCommand(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	body, _ := readJSON(r)
	value, err := s.app.WorkflowCommand(r.Context(), actor, r.PathValue("workflowID"), r.Method, r.URL.Path, body)
	if err != nil {
		s.opError(w, err, "workflow_command_failed")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func queryLimit(r *http.Request) int {
	v, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if v < 1 {
		v = 100
	}
	if v > 200 {
		v = 200
	}
	return v
}
func _unusedHTTPDomain() { _ = strings.TrimSpace }

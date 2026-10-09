package browser

import (
	"errors"
	"net/http"
)

func validateGoalCommand(value map[string]any) bool {
	allowed := map[string]bool{"targetActorId": true, "ownerProtected": true, "candidateOnly": true, "expectedRevision": true, "idempotencyKey": true, "reason": true, "profileId": true, "desiredOutcome": true, "successCriteria": true, "motivation": true, "scope": true, "deadline": true, "clearDeadline": true, "deadlinePolicy": true, "reviewPolicy": true, "resetReviewCounters": true}
	for key := range value {
		if !allowed[key] {
			return false
		}
	}
	return validateString(value["reason"], 1, 1000) && validateString(value["idempotencyKey"], 1, 128)
}
func (s *Server) routeGoals(w http.ResponseWriter, r *http.Request) bool {
	parts := splitPath(r.URL.Path)
	if len(parts) < 4 || parts[0] != "api" || parts[1] != "fluctlights" || parts[3] != "goals" {
		return false
	}
	if len(parts) > 6 {
		writeError(w, 404, "not_found", "Route not found")
		return true
	}
	endpoint := "/internal/fluctlights/" + escape(parts[2]) + "/goals"
	if len(parts) > 4 {
		endpoint += "/" + escape(parts[4])
	}
	if len(parts) > 5 {
		endpoint += "/" + escape(parts[5])
	}
	w.Header().Set("Cache-Control", "no-store, private")
	if r.Method == http.MethodGet && (len(parts) == 4 || len(parts) == 5 || (len(parts) == 6 && (parts[5] == "history" || parts[5] == "evidence"))) {
		if r.URL.RawQuery != "" {
			endpoint += "?" + r.URL.RawQuery
		}
		s.callMap(w, r, endpoint, http.MethodGet, nil, goalRouteError, nil)
		return true
	}
	operation := ""
	if r.Method == http.MethodPost && len(parts) == 4 {
		operation = "create"
	}
	if r.Method == http.MethodPut && len(parts) == 5 {
		operation = "update"
	}
	if r.Method == http.MethodPost && len(parts) == 6 {
		switch parts[5] {
		case "pause", "resume", "cancel", "abandon", "reassess":
			operation = parts[5]
		}
	}
	if operation == "" {
		writeError(w, 405, "method_not_allowed", "Method not allowed")
		return true
	}
	body, ok := s.mutationBody(w, r, validateGoalCommand)
	if !ok {
		return true
	}
	mapped := map[string]any{"operation": operation}
	fields := map[string]string{"targetActorId": "target_actor_id", "ownerProtected": "owner_protected", "candidateOnly": "candidate_only", "expectedRevision": "expected_revision", "idempotencyKey": "idempotency_key", "reason": "reason", "profileId": "profile_id", "desiredOutcome": "desired_outcome", "successCriteria": "success_criteria", "motivation": "motivation", "scope": "scope", "deadline": "deadline", "clearDeadline": "clear_deadline", "deadlinePolicy": "deadline_policy", "reviewPolicy": "review_policy", "resetReviewCounters": "reset_review_counters"}
	for key, target := range fields {
		if v, present := body[key]; present {
			mapped[target] = v
		}
	}
	if operation != "create" {
		if _, present := body["expectedRevision"]; !present {
			writeError(w, 400, "invalid_request", "Expected revision is required")
			return true
		}
	}
	// Only Goal commands pass this boundary. Browser schedule injection stays forbidden.
	s.callMap(w, r, endpoint, r.Method, mapped, goalRouteError, nil)
	return true
}

func goalRouteError(w http.ResponseWriter, err error) {
	var backend *CoreError
	if errors.As(err, &backend) {
		status := backend.Status
		if status >= 500 {
			status = http.StatusBadGateway
		}
		writeErrorWithDetails(w, status, backend.Code, "Goal operation failed", backend.Details)
		return
	}
	writeError(w, 422, "goal_operation_failed", "Goal operation failed")
}

func actorContextRouteError(w http.ResponseWriter, err error) {
	var backend *CoreError
	if errors.As(err, &backend) {
		status := backend.Status
		if status >= 500 {
			status = http.StatusBadGateway
		}
		writeErrorWithDetails(w, status, backend.Code, "Actor背景未保存，请检查版本与权限", backend.Details)
		return
	}
	writeError(w, 422, "actor_context_failed", "Actor背景未保存")
}

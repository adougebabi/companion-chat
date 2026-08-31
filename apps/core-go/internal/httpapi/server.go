package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
)

const serviceKeyHeader = "X-Fluctlight-Service-Key"
const humanSessionHeader = "X-Fluctlight-Human-Session"

type Server struct {
	repository core.Repository
	app        *core.App
	serviceKey string
	logger     *slog.Logger
}

func NewApp(app *core.App, serviceKey string, logger *slog.Logger) *Server {
	server := New(app.DB, serviceKey, logger)
	server.app = app
	return server
}

func New(repository core.Repository, serviceKey string, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	server := &Server{repository: repository, serviceKey: serviceKey, logger: logger}
	return server
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", s.live)
	mux.HandleFunc("GET /health/ready", s.ready)
	mux.HandleFunc("GET /internal/platform/ping", s.ping)
	mux.HandleFunc("GET /internal/auth/session", s.session)
	mux.HandleFunc("POST /internal/auth/login", s.login)
	mux.HandleFunc("POST /internal/auth/setup", s.setup)
	mux.HandleFunc("GET /internal/auth/setup-status", s.setupStatus)
	mux.HandleFunc("POST /internal/auth/revoke-all", s.revokeAll)
	mux.HandleFunc("POST /internal/auth/revoke-current", s.revokeCurrent)
	mux.HandleFunc("POST /internal/auth/reset-password", s.resetPassword)
	mux.HandleFunc("GET /internal/settings", s.settings)
	mux.HandleFunc("PUT /internal/settings", s.updateSettings)
	mux.HandleFunc("GET /internal/providers/endpoints", s.providerEndpoints)
	mux.HandleFunc("PUT /internal/providers/endpoints", s.configureProviderEndpoint)
	mux.HandleFunc("GET /internal/providers", s.providerBindings)
	mux.HandleFunc("GET /internal/providers/endpoints/{endpointID}/models", s.providerModels)
	mux.HandleFunc("PUT /internal/providers/roles", s.configureProviderRole)
	mux.HandleFunc("GET /internal/actor-groups", s.actorGroups)
	mux.HandleFunc("POST /internal/actor-groups", s.createActorGroup)
	mux.HandleFunc("POST /internal/actor-groups/{groupID}/members", s.addActorGroupMember)
	mux.HandleFunc("DELETE /internal/actor-groups/{groupID}/members/{actorID}", s.removeActorGroupMember)
	mux.HandleFunc("POST /internal/fluctlights", s.createFluctlight)
	mux.HandleFunc("POST /internal/fluctlight-creations/analysis", s.analyzeCreation)
	mux.HandleFunc("POST /internal/fluctlight-creations/activate", s.activateCreation)
	mux.HandleFunc("GET /internal/fluctlights", s.listFluctlights)
	mux.HandleFunc("GET /internal/fluctlights/{fluctlightID}", s.getFluctlight)
	mux.HandleFunc("GET /internal/fluctlights/{fluctlightID}/detail", s.fluctlightDetail)
	mux.HandleFunc("GET /internal/fluctlights/{fluctlightID}/conversation", s.directConversation)
	mux.HandleFunc("PUT /internal/fluctlights/{fluctlightID}/status", s.setFluctlightStatus)
	mux.HandleFunc("POST /internal/fluctlights/{fluctlightID}/retire", s.retireFluctlight)
	mux.HandleFunc("POST /internal/fluctlights/{fluctlightID}/foundation-revisions", s.proposeFoundation)
	mux.HandleFunc("POST /internal/fluctlights/{fluctlightID}/foundation-revisions/{revisionID}/accept", s.acceptFoundation)
	mux.HandleFunc("POST /internal/fluctlights/{fluctlightID}/foundation-revisions/{revisionID}/reject", s.rejectFoundation)
	mux.HandleFunc("POST /internal/fluctlights/{fluctlightID}/foundation-revisions/rollback", s.rollbackFoundation)
	mux.HandleFunc("POST /internal/fluctlights/{fluctlightID}/relationships/rollback", s.rollbackRelationship)
	mux.HandleFunc("POST /internal/fluctlights/{fluctlightID}/events", s.createEvent)
	mux.HandleFunc("POST /internal/fluctlights/{fluctlightID}/events/{eventID}/cancel", s.cancelEvent)
	mux.HandleFunc("PUT /internal/fluctlights/{fluctlightID}/presence", s.setPresence)
	mux.HandleFunc("POST /internal/fluctlights/{fluctlightID}/schedules/{scheduleID}/cancel", s.cancelSchedule)
	mux.HandleFunc("GET /internal/fluctlights/{fluctlightID}/moments", s.moments)
	mux.HandleFunc("GET /internal/fluctlights/{fluctlightID}/autonomy-actions", s.autonomyActions)
	mux.HandleFunc("POST /internal/autonomy-actions/{actionID}/govern", s.governAutonomy)
	mux.HandleFunc("GET /internal/moments", s.allMoments)
	mux.HandleFunc("POST /internal/fluctlights/{fluctlightID}/moments/read", s.markMomentsRead)
	mux.HandleFunc("POST /internal/moments/{momentID}/comments", s.commentMoment)
	mux.HandleFunc("POST /internal/moments/{momentID}/reactions", s.reactMoment)
	mux.HandleFunc("POST /internal/moments/{momentID}/hide", s.hideMoment)
	mux.HandleFunc("POST /internal/moments/{momentID}/restore", s.restoreMoment)
	mux.HandleFunc("POST /internal/conversations", s.createConversation)
	mux.HandleFunc("POST /internal/conversations/{conversationID}/read", s.markRead)
	mux.HandleFunc("PUT /internal/memories/{memoryID}", s.reviseMemory)
	mux.HandleFunc("POST /internal/memories/{memoryID}/forget", s.forgetMemory)
	mux.HandleFunc("POST /internal/fluctlights/{fluctlightID}/schedules", s.schedule)
	mux.HandleFunc("GET /internal/conversations/{conversationID}/history", s.history)
	mux.HandleFunc("POST /internal/conversations/{conversationID}/turn", s.turn)
	mux.HandleFunc("GET /internal/media/{assetID}", s.media)
	mux.HandleFunc("GET /internal/diagnostics", s.diagnostics)
	mux.HandleFunc("DELETE /internal/diagnostics", s.clearDiagnostics)
	mux.HandleFunc("GET /internal/diagnostics/model-runs", s.modelRuns)
	mux.HandleFunc("GET /internal/diagnostics/export", s.exportDiagnostics)
	mux.HandleFunc("GET /internal/diagnostics/workflows", s.workflowList)
	mux.HandleFunc("GET /internal/diagnostics/workflows/{workflowID}/status", s.workflowStatus)
	mux.HandleFunc("GET /internal/diagnostics/workflows/{workflowID}/history", s.workflowHistory)
	mux.HandleFunc("POST /internal/diagnostics/workflows/{workflowID}/pause", s.workflowCommand)
	mux.HandleFunc("POST /internal/diagnostics/workflows/{workflowID}/resume", s.workflowCommand)
	mux.HandleFunc("POST /internal/diagnostics/workflows/{workflowID}/cancel", s.workflowCommand)
	mux.HandleFunc("POST /internal/diagnostics/workflows/{workflowID}/reset", s.workflowCommand)
	mux.HandleFunc("POST /internal/diagnostics/workflows/{workflowID}/restart", s.workflowCommand)
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Cache-Control", "no-store")
		mux.ServeHTTP(response, request)
	})
}

func (s *Server) live(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok", "role": "api"})
}

func (s *Server) ready(response http.ResponseWriter, request *http.Request) {
	if err := s.repository.Ping(request.Context()); err != nil {
		writeError(response, http.StatusServiceUnavailable, "core_go_not_ready")
		return
	}
	writeJSON(response, http.StatusOK, map[string]string{"status": "ready", "role": "api"})
}

func (s *Server) ping(response http.ResponseWriter, request *http.Request) {
	if !s.authorizeService(response, request) {
		return
	}
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok", "role": "api"})
}

func (s *Server) session(response http.ResponseWriter, request *http.Request) {
	if !s.authorizeService(response, request) {
		return
	}
	actorID, err := s.repository.ResolveSession(request.Context(), request.Header.Get(humanSessionHeader))
	if errors.Is(err, core.ErrUnauthorized) {
		writeJSON(response, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	if err != nil {
		writeError(response, http.StatusBadGateway, "core_go_session_failed")
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"authenticated": true, "actor_id": actorID})
}

func (s *Server) login(response http.ResponseWriter, request *http.Request) {
	if !s.authorizeService(response, request) || s.app == nil {
		return
	}
	body, ok := readJSON(request)
	if !ok {
		writeError(response, http.StatusBadRequest, "core_request_validation_failed")
		return
	}
	actorID, token, err := s.app.Login(request.Context(), stringValue(body["password"]))
	if err != nil {
		writeJSON(response, http.StatusUnauthorized, map[string]any{"authenticated": false})
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"authenticated": true, "actor_id": actorID, "session_token": token})
}

func (s *Server) setup(response http.ResponseWriter, request *http.Request) {
	if !s.authorizeService(response, request) || s.app == nil {
		return
	}
	body, ok := readJSON(request)
	if !ok {
		writeError(response, http.StatusBadRequest, "core_request_validation_failed")
		return
	}
	actorID, token, err := s.app.Setup(request.Context(), stringValue(body["setup_token"]), stringValue(body["password"]))
	if err != nil {
		writeError(response, http.StatusForbidden, "setup_unavailable")
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"authenticated": true, "actor_id": actorID, "session_token": token})
}

func (s *Server) setupStatus(response http.ResponseWriter, request *http.Request) {
	if !s.authorizeService(response, request) || s.app == nil {
		return
	}
	available, err := s.app.SetupAvailable(request.Context())
	if err != nil {
		writeError(response, http.StatusBadGateway, "setup_status_failed")
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"setup_available": available})
}

func (s *Server) settings(response http.ResponseWriter, request *http.Request) {
	actorID, ok := s.authorizeHuman(response, request)
	if !ok || s.app == nil {
		return
	}
	value, err := s.app.ReadSettings(request.Context(), actorID)
	if err != nil {
		writeError(response, http.StatusForbidden, "forbidden")
		return
	}
	writeJSON(response, http.StatusOK, value)
}

func (s *Server) updateSettings(response http.ResponseWriter, request *http.Request) {
	actorID, ok := s.authorizeHuman(response, request)
	if !ok || s.app == nil {
		return
	}
	body, valid := readJSON(request)
	if !valid {
		writeError(response, http.StatusBadRequest, "core_request_validation_failed")
		return
	}
	value, err := s.app.UpdateSettings(request.Context(), actorID, body)
	if err != nil {
		writeError(response, http.StatusUnprocessableEntity, "settings_update_failed")
		return
	}
	writeJSON(response, http.StatusOK, value)
}

func (s *Server) providerEndpoints(response http.ResponseWriter, request *http.Request) {
	actorID, ok := s.authorizeHuman(response, request)
	if !ok || s.app == nil {
		return
	}
	value, err := s.app.ProviderEndpoints(request.Context(), actorID)
	if err != nil {
		writeError(response, http.StatusForbidden, "provider_configuration_failed")
		return
	}
	writeJSON(response, http.StatusOK, value)
}
func (s *Server) providerBindings(response http.ResponseWriter, request *http.Request) {
	actorID, ok := s.authorizeHuman(response, request)
	if !ok || s.app == nil {
		return
	}
	value, err := s.app.ProviderBindings(request.Context(), actorID)
	if err != nil {
		writeError(response, http.StatusForbidden, "provider_configuration_failed")
		return
	}
	writeJSON(response, http.StatusOK, value)
}
func (s *Server) configureProviderEndpoint(response http.ResponseWriter, request *http.Request) {
	actorID, ok := s.authorizeHuman(response, request)
	if !ok || s.app == nil {
		return
	}
	body, valid := readJSON(request)
	if !valid {
		writeError(response, http.StatusBadRequest, "core_request_validation_failed")
		return
	}
	if err := s.app.ConfigureProviderEndpoint(request.Context(), actorID, body); err != nil {
		writeError(response, http.StatusUnprocessableEntity, "provider_configuration_failed")
		return
	}
	response.WriteHeader(http.StatusNoContent)
}
func (s *Server) providerModels(response http.ResponseWriter, request *http.Request) {
	actorID, ok := s.authorizeHuman(response, request)
	if !ok || s.app == nil {
		return
	}
	value, err := s.app.ProviderModels(request.Context(), actorID, request.PathValue("endpointID"))
	if err != nil {
		writeError(response, http.StatusBadGateway, "provider_models_unavailable")
		return
	}
	writeJSON(response, http.StatusOK, value)
}

func (s *Server) listFluctlights(response http.ResponseWriter, request *http.Request) {
	actorID, ok := s.authorizeHuman(response, request)
	if !ok {
		return
	}
	items, err := s.repository.ListFluctlights(request.Context(), actorID)
	if err != nil {
		s.logger.Error("list Core Go Fluctlights failed", "error", err)
		writeError(response, http.StatusBadGateway, "core_go_fluctlights_failed")
		return
	}
	writeJSON(response, http.StatusOK, items)
}

func (s *Server) getFluctlight(response http.ResponseWriter, request *http.Request) {
	actorID, ok := s.authorizeHuman(response, request)
	if !ok {
		return
	}
	item, err := s.repository.GetFluctlight(request.Context(), request.PathValue("fluctlightID"), actorID)
	if errors.Is(err, core.ErrNotFound) {
		writeError(response, http.StatusNotFound, "fluctlight_not_found")
		return
	}
	if err != nil {
		writeError(response, http.StatusBadGateway, "core_go_fluctlight_failed")
		return
	}
	writeJSON(response, http.StatusOK, item)
}

func (s *Server) fluctlightDetail(response http.ResponseWriter, request *http.Request) {
	actorID, ok := s.authorizeHuman(response, request)
	if !ok || s.app == nil {
		return
	}
	value, err := s.app.FluctlightDetail(request.Context(), actorID, request.PathValue("fluctlightID"))
	if errors.Is(err, core.ErrNotFound) {
		writeError(response, http.StatusNotFound, "fluctlight_not_found")
		return
	}
	if err != nil {
		writeError(response, http.StatusBadGateway, "fluctlight_detail_failed")
		return
	}
	writeJSON(response, http.StatusOK, value)
}

func (s *Server) createFluctlight(response http.ResponseWriter, request *http.Request) {
	actorID, ok := s.authorizeHuman(response, request)
	if !ok || s.app == nil {
		return
	}
	body, valid := readJSON(request)
	if !valid {
		writeError(response, http.StatusBadRequest, "core_request_validation_failed")
		return
	}
	item, err := s.app.CreateFluctlight(request.Context(), actorID, stringValue(body["id"]), stringValue(body["name"]), "blank_slate", nil, nil, nil)
	if err != nil {
		writeError(response, http.StatusConflict, "fluctlight_create_failed")
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"id": item.ID, "identity": item.Identity, "status": item.Status})
}

func (s *Server) analyzeCreation(response http.ResponseWriter, request *http.Request) {
	_, ok := s.authorizeHuman(response, request)
	if !ok || s.app == nil {
		return
	}
	body, valid := readJSON(request)
	if !valid {
		writeError(response, http.StatusBadRequest, "core_request_validation_failed")
		return
	}
	value, err := s.app.AnalyzeDescription(request.Context(), stringValue(body["description"]))
	if err != nil {
		writeError(response, http.StatusUnprocessableEntity, "initialization_foundation_invalid")
		return
	}
	writeJSON(response, http.StatusOK, value)
}

func (s *Server) activateCreation(response http.ResponseWriter, request *http.Request) {
	actorID, ok := s.authorizeHuman(response, request)
	if !ok || s.app == nil {
		return
	}
	body, valid := readJSON(request)
	if !valid {
		writeError(response, http.StatusBadRequest, "core_request_validation_failed")
		return
	}
	mode := firstString(body["initialization_mode"], "llm_defined")
	identity := mapValue(body["identity"])
	requestID := stringValue(body["request_id"])
	if requestID == "" {
		writeError(response, http.StatusUnprocessableEntity, "activation_request_id_required")
		return
	}
	foundation := map[string]any{"identity": identity, "personality": mapValue(body["personality"]), "behavioral_policy": mapValue(body["behavioral_policy"]), "life_profile": mapValue(body["life_profile"]), "provenance": mapValue(body["foundation_provenance"]), "initial_goals": arrayValue(body["initial_goals"]), "initial_intentions": arrayValue(body["initial_intentions"])}
	stable := core.StableFluctlightID(actorID, requestID)
	item, err := s.app.CreateFluctlight(request.Context(), actorID, stable, stringValue(identity["name"]), mode, foundation, arrayValue(body["initial_goals"]), arrayValue(body["initial_intentions"]))
	if err != nil {
		writeError(response, http.StatusUnprocessableEntity, "activation_foundation_invalid")
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"id": item.ID, "identity": item.Identity, "personality": item.Personality, "behavioral_policy": item.BehavioralPolicy, "life_profile": item.LifeProfile, "provenance": item.Provenance, "status": item.Status, "current_revision": item.CurrentRevision})
}

func (s *Server) directConversation(response http.ResponseWriter, request *http.Request) {
	actorID, ok := s.authorizeHuman(response, request)
	if !ok {
		return
	}
	conversationID, err := s.repository.DirectConversationID(request.Context(), actorID, request.PathValue("fluctlightID"))
	if errors.Is(err, core.ErrNotFound) {
		if s.app == nil {
			writeError(response, http.StatusNotFound, "conversation_not_found")
			return
		}
		conversationID, err = s.app.EnsureDirectConversation(request.Context(), actorID, request.PathValue("fluctlightID"))
		if err != nil {
			writeError(response, http.StatusNotFound, "conversation_not_found")
			return
		}
	}
	if err != nil {
		writeError(response, http.StatusBadGateway, "core_go_conversation_failed")
		return
	}
	page, err := s.repository.History(request.Context(), conversationID, actorID, nil, 50)
	if err != nil {
		writeError(response, http.StatusBadGateway, "core_go_conversation_failed")
		return
	}
	writeJSON(response, http.StatusOK, page)
}

func (s *Server) history(response http.ResponseWriter, request *http.Request) {
	actorID, ok := s.authorizeHuman(response, request)
	if !ok {
		return
	}
	var before *int
	beforeParam := request.URL.Query().Get("before_sequence")
	if beforeParam == "" {
		beforeParam = request.URL.Query().Get("before")
	}
	if raw := strings.TrimSpace(beforeParam); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			writeError(response, http.StatusBadRequest, "before_sequence_invalid")
			return
		}
		before = &value
	}
	limit := 50
	if raw := strings.TrimSpace(request.URL.Query().Get("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 200 {
			writeError(response, http.StatusBadRequest, "limit_invalid")
			return
		}
		limit = value
	}
	page, err := s.repository.History(request.Context(), request.PathValue("conversationID"), actorID, before, limit)
	if errors.Is(err, core.ErrNotFound) {
		writeError(response, http.StatusNotFound, "conversation_not_found")
		return
	}
	if err != nil {
		writeError(response, http.StatusBadGateway, "core_go_history_failed")
		return
	}
	writeJSON(response, http.StatusOK, page)
}

func (s *Server) moments(response http.ResponseWriter, request *http.Request) {
	actorID, ok := s.authorizeHuman(response, request)
	if !ok || s.app == nil {
		return
	}
	items, err := s.app.Moments(request.Context(), actorID, request.PathValue("fluctlightID"))
	if errors.Is(err, core.ErrNotFound) {
		writeError(response, http.StatusNotFound, "fluctlight_not_found")
		return
	}
	if err != nil {
		writeError(response, http.StatusBadGateway, "moments_failed")
		return
	}
	writeJSON(response, http.StatusOK, items)
}

func (s *Server) autonomyActions(response http.ResponseWriter, request *http.Request) {
	actorID, ok := s.authorizeHuman(response, request)
	if !ok || s.app == nil {
		return
	}
	items, err := s.app.AutonomyActions(request.Context(), actorID, request.PathValue("fluctlightID"))
	if err != nil {
		writeError(response, http.StatusBadGateway, "autonomy_actions_failed")
		return
	}
	writeJSON(response, http.StatusOK, items)
}

func (s *Server) schedule(response http.ResponseWriter, request *http.Request) {
	actorID, ok := s.authorizeHuman(response, request)
	if !ok || s.app == nil {
		return
	}
	body, valid := readJSON(request)
	if !valid {
		writeError(response, http.StatusBadRequest, "core_request_validation_failed")
		return
	}
	value, err := s.app.AcceptSchedule(request.Context(), actorID, request.PathValue("fluctlightID"), body)
	if err != nil {
		s.logger.Error("Go Core schedule acceptance failed", "fluctlight_id", request.PathValue("fluctlightID"), "error", err)
		writeError(response, http.StatusUnprocessableEntity, "schedule_accept_failed")
		return
	}
	writeJSON(response, http.StatusOK, value)
}

func (s *Server) turn(response http.ResponseWriter, request *http.Request) {
	actorID, ok := s.authorizeHuman(response, request)
	if !ok || s.app == nil {
		return
	}
	body, valid := readJSON(request)
	if !valid {
		writeError(response, http.StatusBadRequest, "core_request_validation_failed")
		return
	}
	response.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	if err := s.app.StreamTurn(request.Context(), response, actorID, request.PathValue("conversationID"), body); err != nil {
		if request.Context().Err() != nil {
			return
		}
		s.logger.Error("Go Core conversation turn failed", "error_type", fmt.Sprintf("%T", err), "error", err.Error())
		writeNDJSONError(response, stringValue(body["turn_id"]), err)
	}
}

func (s *Server) media(response http.ResponseWriter, request *http.Request) {
	actorID, ok := s.authorizeHuman(response, request)
	if !ok || s.app == nil {
		return
	}
	// Authorization is checked against the asset owner before streaming bytes.
	if err := s.app.AuthorizeAsset(request.Context(), actorID, request.PathValue("assetID")); err != nil {
		if errors.Is(err, core.ErrNotFound) {
			writeError(response, http.StatusNotFound, "media_not_found")
		} else {
			writeError(response, http.StatusForbidden, "media_forbidden")
		}
		return
	}
	if err := s.app.ServeMedia(request.Context(), response, request.PathValue("assetID"), request.Header.Get("Range")); err != nil && request.Context().Err() == nil {
		if errors.Is(err, core.ErrNotFound) {
			writeError(response, http.StatusNotFound, "media_not_found")
		} else if err.Error() == "invalid byte range" {
			response.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		}
	}
}

func (s *Server) authorizeService(response http.ResponseWriter, request *http.Request) bool {
	if request.Header.Get(serviceKeyHeader) != s.serviceKey || s.serviceKey == "" {
		writeError(response, http.StatusUnauthorized, "invalid_service_key")
		return false
	}
	return true
}

func (s *Server) authorizeHuman(response http.ResponseWriter, request *http.Request) (string, bool) {
	if !s.authorizeService(response, request) {
		return "", false
	}
	actorID, err := s.repository.ResolveSession(request.Context(), request.Header.Get(humanSessionHeader))
	if errors.Is(err, core.ErrUnauthorized) {
		writeError(response, http.StatusUnauthorized, "unauthenticated")
		return "", false
	}
	if err != nil {
		writeError(response, http.StatusBadGateway, "core_go_session_failed")
		return "", false
	}
	return actorID, true
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	if err := json.NewEncoder(response).Encode(value); err != nil {
		slog.Default().Error("write Core Go response failed", "error", err)
	}
}

func writeError(response http.ResponseWriter, status int, code string) {
	writeJSON(response, status, map[string]any{"detail": map[string]any{
		"code":    code,
		"message": humanMessage(code),
	}})
}

func humanMessage(code string) string {
	code = strings.ReplaceAll(code, "_", " ")
	if code == "" {
		return "Request failed"
	}
	return strings.ToUpper(code[:1]) + code[1:]
}

func readJSON(request *http.Request) (map[string]any, bool) {
	decoder := json.NewDecoder(io.LimitReader(request.Body, 4<<20))
	var value map[string]any
	if err := decoder.Decode(&value); err != nil || value == nil {
		return nil, false
	}
	return value, true
}

func writeNDJSONError(response http.ResponseWriter, turnID string, err error) {
	_ = json.NewEncoder(response).Encode(map[string]any{"type": "error", "turn_id": turnID, "sequence": 0, "payload": map[string]string{"code": "conversation_turn_failed", "message": "The turn could not be completed"}})
}

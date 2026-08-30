package httpapi

import (
	"encoding/json"
	"errors"
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
	serviceKey string
	logger     *slog.Logger
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
	mux.HandleFunc("GET /internal/fluctlights", s.listFluctlights)
	mux.HandleFunc("GET /internal/fluctlights/{fluctlightID}", s.getFluctlight)
	mux.HandleFunc("GET /internal/fluctlights/{fluctlightID}/conversation", s.directConversation)
	mux.HandleFunc("GET /internal/conversations/{conversationID}/history", s.history)
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Cache-Control", "no-store")
		mux.ServeHTTP(response, request)
	})
}

func (s *Server) live(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok", "role": "api-go"})
}

func (s *Server) ready(response http.ResponseWriter, request *http.Request) {
	if err := s.repository.Ping(request.Context()); err != nil {
		writeError(response, http.StatusServiceUnavailable, "core_go_not_ready")
		return
	}
	writeJSON(response, http.StatusOK, map[string]string{"status": "ready", "role": "api-go"})
}

func (s *Server) ping(response http.ResponseWriter, request *http.Request) {
	if !s.authorizeService(response, request) {
		return
	}
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok", "role": "api-go"})
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

func (s *Server) directConversation(response http.ResponseWriter, request *http.Request) {
	actorID, ok := s.authorizeHuman(response, request)
	if !ok {
		return
	}
	conversationID, err := s.repository.DirectConversationID(request.Context(), actorID, request.PathValue("fluctlightID"))
	if errors.Is(err, core.ErrNotFound) {
		writeError(response, http.StatusNotFound, "conversation_not_found")
		return
	}
	if err != nil {
		writeError(response, http.StatusBadGateway, "core_go_conversation_failed")
		return
	}
	writeJSON(response, http.StatusOK, map[string]string{"conversation_id": conversationID})
}

func (s *Server) history(response http.ResponseWriter, request *http.Request) {
	actorID, ok := s.authorizeHuman(response, request)
	if !ok {
		return
	}
	var before *int
	if raw := strings.TrimSpace(request.URL.Query().Get("before")); raw != "" {
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
	writeJSON(response, status, map[string]string{"error": code})
}

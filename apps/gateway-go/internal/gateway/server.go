package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/fluctlight/local-ai-companion/apps/gateway-go/internal/config"
)

const (
	serviceKeyHeader = "x-fluctlight-service-key"
	maxUpstreamBody  = 1 << 20
)

type roleResponse struct {
	Status string `json:"status"`
	Role   string `json:"role"`
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Server struct {
	coreBaseURL    *url.URL
	coreServiceKey string
	client         *http.Client
}

func NewServer(settings config.Config, client *http.Client) *Server {
	if client == nil {
		client = &http.Client{}
	}
	return &Server{
		coreBaseURL:    settings.CoreBaseURL,
		coreServiceKey: settings.CoreServiceKey,
		client:         client,
	}
}

func (server *Server) Handler() http.Handler {
	return http.HandlerFunc(server.route)
}

func (server *Server) route(response http.ResponseWriter, request *http.Request) {
	switch request.URL.Path {
	case "/health/live":
		server.live(response, request)
	case "/health/ready":
		server.ready(response, request)
	case "/api/platform/ping":
		server.ping(response, request)
	default:
		http.NotFound(response, request)
	}
}

func (server *Server) live(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(response)
		return
	}
	writeJSON(response, http.StatusOK, roleResponse{Status: "ok", Role: "go-public-gateway"})
}

func (server *Server) ready(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(response)
		return
	}

	upstream, err := server.getCore(request.Context(), "/health/ready")
	if err != nil {
		writeUnavailable(response)
		return
	}
	defer upstream.Body.Close()
	if upstream.StatusCode < http.StatusOK || upstream.StatusCode >= http.StatusMultipleChoices {
		writeUnavailable(response)
		return
	}

	writeJSON(response, http.StatusOK, roleResponse{Status: "ready", Role: "go-public-gateway"})
}

func (server *Server) ping(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(response)
		return
	}

	upstream, err := server.getCore(request.Context(), "/internal/platform/ping")
	if err != nil {
		writeJSON(response, http.StatusBadGateway, errorResponse{
			Code:    "core_unavailable",
			Message: "Core platform is unavailable",
		})
		return
	}
	defer upstream.Body.Close()

	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(upstream.StatusCode)
	if _, copyErr := io.CopyN(response, upstream.Body, maxUpstreamBody+1); copyErr != nil && copyErr != io.EOF {
		return
	}
}

func (server *Server) getCore(ctx context.Context, endpoint string) (*http.Response, error) {
	if server.coreBaseURL == nil {
		return nil, fmt.Errorf("core base URL is not configured")
	}

	target := *server.coreBaseURL
	target.Path = strings.TrimRight(target.Path, "/") + endpoint
	target.RawQuery = ""
	target.Fragment = ""

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set(serviceKeyHeader, server.coreServiceKey)
	return server.client.Do(request)
}

func writeUnavailable(response http.ResponseWriter) {
	writeJSON(response, http.StatusServiceUnavailable, roleResponse{Status: "unavailable", Role: "go-public-gateway"})
}

func methodNotAllowed(response http.ResponseWriter) {
	response.Header().Set("Allow", http.MethodGet)
	writeJSON(response, http.StatusMethodNotAllowed, errorResponse{
		Code:    "method_not_allowed",
		Message: "Only GET is supported",
	})
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

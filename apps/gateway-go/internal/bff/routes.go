package bff

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/fluctlight/local-ai-companion/apps/gateway-go/internal/platform"
)

// Options is the transport-only composition for the public BFF.  The BFF
// never receives a database, cache, object-store or workflow dependency.
type Options struct {
	CoreBaseURL    *url.URL
	CoreServiceKey string
	TrustedOrigin  string
	SecureCookies  bool
	Client         *http.Client
}

type Server struct {
	core          *coreClient
	trustedOrigin string
	secureCookies bool
}

func New(options Options) *Server {
	secure := options.SecureCookies
	if !secure && (options.TrustedOrigin == "" || strings.HasPrefix(options.TrustedOrigin, "https://")) {
		secure = true
	}
	return &Server{
		core:          newCoreClient(options.CoreBaseURL, options.CoreServiceKey, options.Client),
		trustedOrigin: normalizeOrigin(options.TrustedOrigin),
		secureCookies: secure,
	}
}

// NewServer is kept as an explicit constructor alias for callers migrating
// from the initial gateway package naming.
func NewServer(options Options) *Server { return New(options) }

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		corsHeaders(response, request, s.trustedOrigin)
		wrapped := &commonResponseWriter{ResponseWriter: response, secure: s.secureCookies, request: request}
		if request.Method == http.MethodOptions {
			optionsResponse(wrapped, request, s.trustedOrigin)
			return
		}
		s.route(wrapped, request)
	})
}

func (s *Server) route(response http.ResponseWriter, request *http.Request) {
	path := request.URL.Path

	if path == "/health/live" {
		if !method(response, request, http.MethodGet) {
			return
		}
		writeJSON(response, http.StatusOK, platform.Live(platform.RoleBFF))
		return
	}
	if path == "/health/ready" {
		if !method(response, request, http.MethodGet) {
			return
		}
		if !platform.IsReady(request.Context(), func(ctx context.Context) error {
			_, err := s.core.health(ctx, "/health/ready")
			return err
		}) {
			writeJSON(response, http.StatusServiceUnavailable, platform.Unavailable(platform.RoleBFF))
			return
		}
		writeJSON(response, http.StatusOK, platform.Ready(platform.RoleBFF))
		return
	}
	if path == "/api/platform/ping" {
		if !method(response, request, http.MethodGet) {
			return
		}
		value, err := s.core.doJSON(request.Context(), http.MethodGet, "/internal/platform/ping", "", nil)
		if err != nil {
			writeError(response, http.StatusBadGateway, "core_unavailable", "Core platform is unavailable")
			return
		}
		writeJSON(response, http.StatusOK, value)
		return
	}

	// Authentication has a small number of anonymous endpoints.
	if path == "/auth/session" {
		if !method(response, request, http.MethodGet) {
			return
		}
		value, err := s.core.doJSON(request.Context(), http.MethodGet, "/internal/auth/session", cookieValue(request, sessionCookieName), nil)
		if err != nil || !truthy(value["authenticated"]) {
			writeJSON(response, http.StatusUnauthorized, map[string]any{"authenticated": false})
			return
		}
		sessionResponse := map[string]any{"authenticated": true}
		if actorID := first(value, "actorId", "actor_id"); actorID != nil {
			sessionResponse["actorId"] = actorID
		}
		writeJSON(response, http.StatusOK, sessionResponse)
		return
	}
	if path == "/auth/setup-status" {
		if !method(response, request, http.MethodGet) {
			return
		}
		value, err := s.core.doJSON(request.Context(), http.MethodGet, "/internal/auth/setup-status", "", nil)
		if err != nil {
			writeError(response, http.StatusBadGateway, "core_unavailable", "Core authentication is unavailable")
			return
		}
		writeJSON(response, http.StatusOK, map[string]any{"setupAvailable": truthy(first(value, "setup_available", "setupAvailable"))})
		return
	}
	if path == "/auth/login" || path == "/auth/setup" {
		if !method(response, request, http.MethodPost) {
			return
		}
		body, ok := readBody(response, request)
		if !ok || !validatePassword(body["password"]) || (path == "/auth/setup" && !validateString(body["setupToken"], 16, 1<<20)) {
			if ok {
				writeError(response, http.StatusBadRequest, "invalid_request", "Request validation failed")
			}
			return
		}
		if !mutationGuard(response, request, s.trustedOrigin) {
			return
		}
		coreBody := map[string]any{"password": body["password"]}
		endpoint := "/internal/auth/login"
		if path == "/auth/setup" {
			endpoint = "/internal/auth/setup"
			coreBody["setup_token"] = body["setupToken"]
		}
		value, err := s.core.doJSON(request.Context(), http.MethodPost, endpoint, "", coreBody)
		if err != nil {
			writeJSON(response, http.StatusUnauthorized, map[string]any{"authenticated": false})
			return
		}
		setSessionCookie(response, stringValue(first(value, "session_token", "sessionToken")), s.secureCookies)
		setCSRFCookie(response, newCSRFToken(), s.secureCookies)
		authenticated := map[string]any{"authenticated": true}
		if actorID := first(value, "actor_id", "actorId"); actorID != nil {
			authenticated["actorId"] = actorID
		}
		writeJSON(response, http.StatusOK, authenticated)
		return
	}
	if path == "/auth/logout" || path == "/auth/revoke-all" || path == "/auth/password" {
		if !method(response, request, http.MethodPost) {
			return
		}
		var passwordBody map[string]any
		if path == "/auth/password" {
			var valid bool
			passwordBody, valid = readBody(response, request)
			if !valid || !validatePassword(passwordBody["password"]) {
				if valid {
					writeError(response, http.StatusBadRequest, "invalid_request", "Request validation failed")
				}
				return
			}
		}
		if !mutationGuard(response, request, s.trustedOrigin) {
			return
		}
		session := cookieValue(request, sessionCookieName)
		if path != "/auth/logout" && session == "" {
			writeError(response, http.StatusUnauthorized, "unauthenticated", "Authentication is required")
			return
		}
		if path == "/auth/password" {
			if _, err := s.core.doJSON(request.Context(), http.MethodPost, "/internal/auth/reset-password", session, map[string]any{"password": passwordBody["password"]}); err != nil {
				writeError(response, http.StatusForbidden, "password_change_failed", "Password could not be changed")
				return
			}
			clearSessionCookie(response, s.secureCookies)
			setCSRFCookie(response, newCSRFToken(), s.secureCookies)
			response.WriteHeader(http.StatusNoContent)
			return
		}
		if session != "" {
			endpoint := "/internal/auth/revoke-current"
			failureCode, failureMessage := "logout_failed", "Session could not be revoked"
			if path == "/auth/revoke-all" {
				endpoint, failureCode, failureMessage = "/internal/auth/revoke-all", "revoke_failed", "Session revocation failed"
			}
			if _, err := s.core.doJSON(request.Context(), http.MethodPost, endpoint, session, nil); err != nil {
				writeError(response, http.StatusForbidden, failureCode, failureMessage)
				return
			}
		}
		clearSessionCookie(response, s.secureCookies)
		setCSRFCookie(response, newCSRFToken(), s.secureCookies)
		response.WriteHeader(http.StatusNoContent)
		return
	}

	// The remaining browser API requires a resolved opaque session.
	if strings.HasPrefix(path, "/api/") {
		s.routeAPI(response, request)
		return
	}
	http.NotFound(response, request)
}

func (s *Server) routeAPI(response http.ResponseWriter, request *http.Request) {
	path := request.URL.Path
	methodName := request.Method

	if conversationID, ok := match(path, "/api/conversations/:conversationId/turn"); ok {
		if methodName != http.MethodPost {
			methodNotAllowed(response, http.MethodPost)
			return
		}
		body, valid := s.mutationBody(response, request, validateConversationTurn)
		if !valid {
			return
		}
		session := cookieValue(request, sessionCookieName)
		mapped := map[string]any{
			"text":            body["text"],
			"fluctlight_id":   body["fluctlightId"],
			"attachment_refs": stringArray(body["attachmentRefs"]),
			"idempotency_key": body["idempotencyKey"],
		}
		if value, exists := body["turnId"]; exists {
			mapped["turn_id"] = value
		}
		extra := make(http.Header)
		extra.Set("Accept", "application/x-ndjson")
		upstream, err := s.core.request(request.Context(), http.MethodPost, "/internal/conversations/"+escape(conversationID)+"/turn", session, mapped, extra)
		if err != nil {
			writeError(response, http.StatusBadGateway, "conversation_turn_failed", "The conversation turn failed")
			return
		}
		if upstream.Body == nil || upstream.ContentLength == 0 {
			if upstream.Body != nil {
				_ = upstream.Body.Close()
			}
			writeError(response, http.StatusBadGateway, "conversation_turn_failed", "The conversation turn failed")
			return
		}
		response.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
		if err := TranslateCoreNDJSON(request.Context(), upstream, response); err != nil && request.Context().Err() == nil {
			var translationErr *NdjsonTranslationError
			if errors.As(err, &translationErr) {
				writeError(response, http.StatusBadGateway, "conversation_turn_failed", "The conversation turn failed")
			}
			// Once the stream starts the translator owns the bounded protocol
			// error frame. A downstream writer failure only ends the response.
		}
		return
	}

	// Authenticated read-only routes.
	if path == "/api/settings" && methodName == http.MethodGet {
		s.callMap(response, request, "/internal/settings", http.MethodGet, nil, s.readOnlyError(http.StatusForbidden, "settings_unavailable", "Settings are unavailable"), func(value map[string]any) any { return mapSettings(value) })
		return
	}
	if path == "/api/capability-requests" && methodName == http.MethodGet {
		s.callAny(response, request, "/internal/capability-requests", http.MethodGet, nil, s.readOnlyError(http.StatusForbidden, "capability_requests_unavailable", "Capability requests are unavailable"), func(value any) any { return browserCapabilityRequests(value) })
		return
	}
	if path == "/api/providers/endpoints" && methodName == http.MethodGet {
		s.callAny(response, request, "/internal/providers/endpoints", http.MethodGet, nil, s.readOnlyError(http.StatusForbidden, "provider_configuration_failed", "Provider endpoints are unavailable"), nil)
		return
	}
	if path == "/api/providers" && methodName == http.MethodGet {
		s.callAny(response, request, "/internal/providers", http.MethodGet, nil, s.readOnlyError(http.StatusForbidden, "provider_configuration_failed", "Provider configuration is unavailable"), nil)
		return
	}
	if path == "/api/fluctlights" && methodName == http.MethodGet {
		s.callAny(response, request, "/internal/fluctlights", http.MethodGet, nil, s.readOnlyError(http.StatusForbidden, "fluctlights_unavailable", "Fluctlights are unavailable"), nil)
		return
	}
	if path == "/api/actor-groups" && methodName == http.MethodGet {
		s.callAny(response, request, "/internal/actor-groups", http.MethodGet, nil, s.readOnlyError(http.StatusForbidden, "actor_groups_unavailable", "Actor groups are unavailable"), nil)
		return
	}
	if path == "/api/moments" && methodName == http.MethodGet {
		includeHidden := request.URL.Query().Get("includeHidden") == "true"
		s.callAny(response, request, "/internal/moments?include_hidden="+strconv.FormatBool(includeHidden), http.MethodGet, nil, s.readOnlyError(http.StatusForbidden, "moments_unavailable", "Moments are unavailable"), nil)
		return
	}
	if path == "/api/diagnostics" && methodName == http.MethodGet {
		s.diagnostics(response, request)
		return
	}
	if path == "/api/diagnostics/model-runs" && methodName == http.MethodGet {
		s.diagnosticModelRuns(response, request)
		return
	}
	if path == "/api/diagnostics/media-prompts" && methodName == http.MethodGet {
		s.diagnosticMediaPrompts(response, request)
		return
	}
	if path == "/api/diagnostics/export" && methodName == http.MethodGet {
		s.diagnosticsExport(response, request)
		return
	}
	if path == "/api/diagnostics/workflows" && methodName == http.MethodGet {
		session, ok := s.requireSession(response, request)
		if !ok {
			return
		}
		query := request.URL.Query().Get("query")
		value, err := s.core.doAny(request.Context(), http.MethodGet, "/internal/diagnostics/workflows?query="+url.QueryEscape(query), session, nil)
		if err != nil {
			writeError(response, http.StatusBadGateway, "workflow_runtime_unavailable", "Workflow runtime is unavailable")
			return
		}
		writeJSON(response, http.StatusOK, value)
		return
	}
	if path == "/api/diagnostics" && methodName == http.MethodDelete {
		s.clearDiagnostics(response, request)
		return
	}

	// Provider and conversation collection operations.
	if path == "/api/providers/endpoints" && methodName == http.MethodPut {
		body, ok := s.mutationBody(response, request, validateProviderEndpoint)
		if !ok {
			return
		}
		s.callNoContent(response, request, "/internal/providers/endpoints", http.MethodPut, map[string]any{"endpoint_id": body["endpointId"], "kind": body["kind"], "base_url": body["baseUrl"], "secret_purpose": body["secretPurpose"]}, 403, "provider_configuration_failed", "Provider configuration failed")
		return
	}
	if endpointID, ok := match(path, "/api/providers/endpoints/:endpointId/models"); ok && methodName == http.MethodGet {
		if !validateString(endpointID, 1, 128) {
			writeError(response, http.StatusBadRequest, "invalid_request", "Request validation failed")
			return
		}
		session, valid := s.requireSession(response, request)
		if !valid {
			return
		}
		value, err := s.core.doJSON(request.Context(), http.MethodGet, "/internal/providers/endpoints/"+escape(endpointID)+"/models", session, nil)
		if err != nil {
			writeError(response, http.StatusUnprocessableEntity, "provider_models_unavailable", "Provider models are unavailable")
			return
		}
		writeJSON(response, http.StatusOK, map[string]any{"endpointId": first(value, "endpoint_id", "endpointId"), "models": value["models"]})
		return
	}
	if path == "/api/providers/roles" && methodName == http.MethodPut {
		body, ok := s.mutationBody(response, request, validateModelRole)
		if !ok {
			return
		}
		session, valid := s.requireSession(response, request)
		if !valid {
			return
		}
		value, err := s.core.doJSON(request.Context(), http.MethodPut, "/internal/providers/roles", session, map[string]any{"role": body["role"], "endpoint_id": body["endpointId"], "model_id": body["modelId"], "token_budget": body["tokenBudget"], "timeout_seconds": body["timeoutSeconds"]})
		if err != nil {
			providerRoleError(response, err)
			return
		}
		writeJSON(response, http.StatusOK, browserProviderPreflight(value))
		return
	}
	if path == "/api/settings" && methodName == http.MethodPut {
		body, ok := s.mutationBody(response, request, validateSettings)
		if !ok {
			return
		}
		mapped := map[string]any{"values": objectValue(body["values"]), "secrets": objectValue(body["secrets"]), "clear_secrets": stringArray(body["clearSecrets"])}
		s.callMap(response, request, "/internal/settings", http.MethodPut, mapped, s.readOnlyError(http.StatusForbidden, "settings_unavailable", "Settings are unavailable"), func(value map[string]any) any { return mapSettings(value) })
		return
	}
	if path == "/api/conversations" && methodName == http.MethodPost {
		body, ok := s.mutationBody(response, request, validateConversationCreate)
		if !ok {
			return
		}
		mapped := map[string]any{"participant_actor_ids": body["participantActorIds"]}
		if _, exists := body["title"]; exists {
			mapped["title"] = body["title"]
		}
		s.callMap(response, request, "/internal/conversations", http.MethodPost, mapped, s.readOnlyError(http.StatusBadGateway, "conversation_unavailable", "Conversation is unavailable"), func(value map[string]any) any { return browserPage(value) })
		return
	}
	if path == "/api/fluctlights" && methodName == http.MethodPost {
		body, ok := s.mutationBody(response, request, validateFluctlightCreate)
		if !ok {
			return
		}
		s.callMap(response, request, "/internal/fluctlights", http.MethodPost, body, s.readOnlyError(http.StatusConflict, "fluctlight_create_failed", "Fluctlight could not be created"), nil)
		return
	}
	if path == "/api/fluctlight-creations/analysis" && methodName == http.MethodPost {
		body, ok := s.mutationBody(response, request, func(value map[string]any) bool { return validateString(value["description"], 1, 12000) })
		if !ok {
			return
		}
		session, valid := s.requireSession(response, request)
		if !valid {
			return
		}
		value, err := s.core.doJSON(request.Context(), http.MethodPost, "/internal/fluctlight-creations/analysis", session, map[string]any{"description": body["description"]})
		if err != nil {
			s.creationError(response, err, "analysis")
			return
		}
		writeJSON(response, http.StatusOK, value)
		return
	}
	if path == "/api/fluctlight-creations/activate" && methodName == http.MethodPost {
		body, ok := s.mutationBody(response, request, validateActivation)
		if !ok {
			return
		}
		session, valid := s.requireSession(response, request)
		if !valid {
			return
		}
		mapped := map[string]any{"request_id": body["requestId"], "initialization_mode": body["initializationMode"], "name": body["name"], "core_persona": body["corePersona"], "developing_self": body["developingSelf"]}
		for from, to := range map[string]string{"initialGoals": "initial_goals", "initialIntentions": "initial_intentions"} {
			if value, exists := body[from]; exists {
				mapped[to] = value
			}
		}
		value, err := s.core.doJSON(request.Context(), http.MethodPost, "/internal/fluctlight-creations/activate", session, mapped)
		if err != nil {
			s.creationError(response, err, "activation")
			return
		}
		writeJSON(response, http.StatusOK, value)
		return
	}
	if fluctlightID, ok := match(path, "/api/fluctlights/:fluctlightId/developing-self"); ok && methodName == http.MethodGet {
		s.callMap(response, request, "/internal/fluctlights/"+escape(fluctlightID)+"/developing-self", http.MethodGet, nil, s.readOnlyError(http.StatusUnprocessableEntity, "developing_self_read_failed", "Developing Self could not be loaded"), nil)
		return
	}
	if fluctlightID, claimID, action, ok := developingSelfAction(path); ok && methodName == http.MethodPost {
		body, valid := s.mutationBody(response, request, func(value map[string]any) bool {
			return validateRevisionReason(value)
		})
		if !valid {
			return
		}
		mapped := map[string]any{"expected_revision": body["expectedRevision"], "reason": body["reason"]}
		s.callMap(response, request, "/internal/fluctlights/"+escape(fluctlightID)+"/developing-self/"+escape(claimID)+"/"+action, http.MethodPost, mapped, s.readOnlyError(http.StatusUnprocessableEntity, "developing_self_"+action+"_failed", "Developing Self claim governance failed"), nil)
		return
	}

	// Actor group, Fluctlight, Moment, Memory, Relationship, Life World and
	// Workflow routes are all explicit below.  Each handler keeps its own
	// request mapping and stable error table.
	if path == "/api/actor-groups" && methodName == http.MethodPost {
		body, ok := s.mutationBody(response, request, func(value map[string]any) bool { return validateString(value["name"], 1, 128) })
		if !ok {
			return
		}
		s.callMap(response, request, "/internal/actor-groups", http.MethodPost, map[string]any{"name": body["name"]}, s.readOnlyError(http.StatusUnprocessableEntity, "actor_group_create_failed", "Actor group could not be created"), nil)
		return
	}
	if groupID, ok := match(path, "/api/actor-groups/:groupId/members"); ok && methodName == http.MethodPost {
		body, valid := s.mutationBody(response, request, func(value map[string]any) bool { return validateString(value["actorId"], 1, 128) })
		if !valid {
			return
		}
		s.callNoContent(response, request, "/internal/actor-groups/"+escape(groupID)+"/members", http.MethodPost, map[string]any{"actor_id": body["actorId"]}, 422, "actor_group_assign_failed", "Actor could not be assigned")
		return
	}
	if groupID, actorID, ok := match2(path, "/api/actor-groups/:groupId/members/:actorId"); ok && methodName == http.MethodDelete {
		s.callNoContent(response, request, "/internal/actor-groups/"+escape(groupID)+"/members/"+escape(actorID), http.MethodDelete, nil, 422, "actor_group_remove_failed", "Actor could not be removed")
		return
	}

	if fluctlightID, ok := match(path, "/api/fluctlights/:fluctlightId"); ok && methodName == http.MethodGet {
		s.callMap(response, request, "/internal/fluctlights/"+escape(fluctlightID), http.MethodGet, nil, s.readOnlyError(http.StatusNotFound, "fluctlight_not_found", "Fluctlight is unavailable"), nil)
		return
	}
	if fluctlightID, ok := match(path, "/api/fluctlights/:fluctlightId/conversation"); ok && methodName == http.MethodGet {
		s.callMap(response, request, "/internal/fluctlights/"+escape(fluctlightID)+"/conversation", http.MethodGet, nil, s.readOnlyError(http.StatusNotFound, "fluctlight_conversation_unavailable", "Fluctlight conversation is unavailable"), func(value map[string]any) any { return browserPage(value) })
		return
	}
	if fluctlightID, ok := match(path, "/api/fluctlights/:fluctlightId/moments"); ok && methodName == http.MethodGet {
		includeHidden := request.URL.Query().Get("includeHidden") == "true"
		s.callAny(response, request, "/internal/fluctlights/"+escape(fluctlightID)+"/moments?include_hidden="+strconv.FormatBool(includeHidden), http.MethodGet, nil, s.readOnlyError(http.StatusNotFound, "fluctlight_moments_unavailable", "Fluctlight Moments are unavailable"), nil)
		return
	}
	if fluctlightID, ok := match(path, "/api/fluctlights/:fluctlightId/detail"); ok && methodName == http.MethodGet {
		s.callMap(response, request, "/internal/fluctlights/"+escape(fluctlightID)+"/detail", http.MethodGet, nil, s.readOnlyError(http.StatusNotFound, "fluctlight_not_found", "Fluctlight detail is unavailable"), nil)
		return
	}
	if fluctlightID, ok := match(path, "/api/fluctlights/:fluctlightId/autonomy-actions"); ok && methodName == http.MethodGet {
		s.callAny(response, request, "/internal/fluctlights/"+escape(fluctlightID)+"/autonomy-actions", http.MethodGet, nil, s.readOnlyError(http.StatusForbidden, "autonomy_actions_unavailable", "Autonomy actions are unavailable"), nil)
		return
	}
	if requestID, ok := match(path, "/api/capability-requests/:requestId/review"); ok && methodName == http.MethodPost {
		body, valid := s.mutationBody(response, request, validateCapabilityRequestReview)
		if !valid {
			return
		}
		s.callMap(response, request, "/internal/capability-requests/"+escape(requestID)+"/review", http.MethodPost, map[string]any{"status": body["status"], "note": body["note"], "capability_version": body["capabilityVersion"]}, s.readOnlyError(http.StatusUnprocessableEntity, "capability_request_review_failed", "Capability request could not be reviewed"), nil)
		return
	}
	if fluctlightID, ok := match(path, "/api/fluctlights/:fluctlightId/status"); ok && methodName == http.MethodPut {
		body, valid := s.mutationBody(response, request, validateFluctlightStatus)
		if !valid {
			return
		}
		s.callMap(response, request, "/internal/fluctlights/"+escape(fluctlightID)+"/status", http.MethodPut, map[string]any{"status": body["status"], "expected_revision": body["expectedRevision"], "reason": body["reason"]}, s.readOnlyError(422, "fluctlight_status_failed", "Fluctlight status could not be changed"), nil)
		return
	}
	if fluctlightID, ok := match(path, "/api/fluctlights/:fluctlightId/retire"); ok && methodName == http.MethodPost {
		body, valid := s.mutationBody(response, request, validateRevisionReason)
		if !valid {
			return
		}
		s.callMap(response, request, "/internal/fluctlights/"+escape(fluctlightID)+"/retire", http.MethodPost, map[string]any{"expected_revision": body["expectedRevision"], "reason": body["reason"]}, s.readOnlyError(422, "fluctlight_retire_failed", "Fluctlight could not be retired"), nil)
		return
	}
	if fluctlightID, ok := match(path, "/api/fluctlights/:fluctlightId/moments/read"); ok && methodName == http.MethodPost {
		s.callNoContent(response, request, "/internal/fluctlights/"+escape(fluctlightID)+"/moments/read", http.MethodPost, map[string]any{}, 422, "moment_read_failed", "Moments could not be marked read")
		return
	}

	// Foundation revisions, memories and relationship rollback.
	if fluctlightID, ok := match(path, "/api/fluctlights/:fluctlightId/foundation-revisions"); ok && methodName == http.MethodPost {
		body, valid := s.mutationBody(response, request, validateFoundationRevision)
		if !valid {
			return
		}
		s.callMap(response, request, "/internal/fluctlights/"+escape(fluctlightID)+"/foundation-revisions", http.MethodPost, map[string]any{"changes": body["changes"], "expected_revision": body["expectedRevision"], "reason": body["reason"]}, s.readOnlyError(422, "foundation_revision_failed", "Foundation revision could not be proposed"), nil)
		return
	}
	if fluctlightID, revisionID, action, ok := foundationAction(path); ok && methodName == http.MethodPost {
		body, valid := s.mutationBody(response, request, validateRevisionReason)
		if !valid {
			return
		}
		endpoint := "/internal/fluctlights/" + escape(fluctlightID) + "/foundation-revisions/" + escape(revisionID) + "/" + action
		code, message := "foundation_revision_"+action+"_failed", "Foundation revision could not be "+action+"ed"
		s.callMap(response, request, endpoint, http.MethodPost, map[string]any{"expected_revision": body["expectedRevision"], "reason": body["reason"]}, s.readOnlyError(422, code, message), nil)
		return
	}
	if fluctlightID, ok := match(path, "/api/fluctlights/:fluctlightId/foundation-revisions/rollback"); ok && methodName == http.MethodPost {
		body, valid := s.mutationBody(response, request, validateFoundationRollback)
		if !valid {
			return
		}
		s.callMap(response, request, "/internal/fluctlights/"+escape(fluctlightID)+"/foundation-revisions/rollback", http.MethodPost, map[string]any{"target_revision": body["targetRevision"], "expected_revision": body["expectedRevision"], "reason": body["reason"]}, s.readOnlyError(422, "foundation_revision_rollback_failed", "Foundation revision could not be rolled back"), nil)
		return
	}
	if memoryID, ok := match(path, "/api/memories/:memoryId"); ok && methodName == http.MethodPut {
		body, valid := s.mutationBody(response, request, validateMemoryRevision)
		if !valid {
			return
		}
		s.callMap(response, request, "/internal/memories/"+escape(memoryID), http.MethodPut, map[string]any{"expected_revision": body["expectedRevision"], "content": body["content"], "evidence_refs": body["evidenceRefs"]}, s.readOnlyError(422, "memory_revision_failed", "Memory could not be revised"), nil)
		return
	}
	if memoryID, ok := match(path, "/api/memories/:memoryId/forget"); ok && methodName == http.MethodPost {
		body, valid := s.mutationBody(response, request, validateMemoryForget)
		if !valid {
			return
		}
		s.callMap(response, request, "/internal/memories/"+escape(memoryID)+"/forget", http.MethodPost, map[string]any{"expected_revision": body["expectedRevision"], "evidence_refs": body["evidenceRefs"]}, s.readOnlyError(422, "memory_forget_failed", "Memory could not be forgotten"), nil)
		return
	}
	if fluctlightID, ok := match(path, "/api/fluctlights/:fluctlightId/relationships/rollback"); ok && methodName == http.MethodPost {
		body, valid := s.mutationBody(response, request, validateRelationshipRollback)
		if !valid {
			return
		}
		s.callMap(response, request, "/internal/fluctlights/"+escape(fluctlightID)+"/relationships/rollback", http.MethodPost, map[string]any{"target_actor_id": body["targetActorId"], "target_revision": body["targetRevision"], "expected_revision": body["expectedRevision"], "evidence_refs": body["evidenceRefs"]}, s.readOnlyError(422, "relationship_rollback_failed", "Relationship could not be rolled back"), nil)
		return
	}

	// Life-world and autonomy operations.
	if actionID, ok := match(path, "/api/autonomy-actions/:actionId/govern"); ok && methodName == http.MethodPost {
		body, valid := s.mutationBody(response, request, validateAutonomyGovernance)
		if !valid {
			return
		}
		s.callMap(response, request, "/internal/autonomy-actions/"+escape(actionID)+"/govern", http.MethodPost, body, s.readOnlyError(422, "autonomy_governance_failed", "Autonomy action could not be governed"), nil)
		return
	}
	if fluctlightID, ok := match(path, "/api/fluctlights/:fluctlightId/events"); ok && methodName == http.MethodPost {
		body, valid := s.mutationBody(response, request, validateLifeEvent)
		if !valid {
			return
		}
		mapped := map[string]any{"kind": body["kind"], "start_at": body["startAt"], "end_at": body["endAt"], "evidence_refs": body["evidenceRefs"]}
		for from, to := range map[string]string{"scene": "scene", "activity": "activity", "location": "location"} {
			if value, exists := body[from]; exists {
				mapped[to] = value
			}
		}
		s.callMap(response, request, "/internal/fluctlights/"+escape(fluctlightID)+"/events", http.MethodPost, mapped, s.readOnlyError(422, "life_event_failed", "Life event could not be created"), nil)
		return
	}
	if fluctlightID, eventID, ok := match2(path, "/api/fluctlights/:fluctlightId/events/:eventId/cancel"); ok && methodName == http.MethodPost {
		s.callNoContent(response, request, "/internal/fluctlights/"+escape(fluctlightID)+"/events/"+escape(eventID)+"/cancel", http.MethodPost, map[string]any{}, 422, "life_event_cancel_failed", "Life event could not be cancelled")
		return
	}
	if fluctlightID, ok := match(path, "/api/fluctlights/:fluctlightId/presence"); ok && methodName == http.MethodPut {
		body, valid := s.mutationBody(response, request, validatePresence)
		if !valid {
			return
		}
		mapped := map[string]any{}
		if value, exists := body["currentTask"]; exists {
			mapped["current_task"] = value
		}
		if value, exists := body["userPresence"]; exists {
			mapped["user_presence"] = value
		}
		s.callMap(response, request, "/internal/fluctlights/"+escape(fluctlightID)+"/presence", http.MethodPut, mapped, s.readOnlyError(422, "life_presence_failed", "Presence could not be updated"), nil)
		return
	}
	if fluctlightID, ok := match(path, "/api/fluctlights/:fluctlightId/schedules"); ok && methodName == http.MethodPost {
		body, valid := s.mutationBody(response, request, validateSchedule)
		if !valid {
			return
		}
		items := make([]any, 0)
		for _, value := range array(body["items"]) {
			item := objectValue(value)
			mapped := map[string]any{"start_at": item["startAt"], "end_at": item["endAt"], "activity": item["activity"], "scene": item["scene"]}
			for from, to := range map[string]string{"itemType": "item_type", "status": "status", "priority": "priority", "flexibility": "flexibility", "interruptionCost": "interruption_cost"} {
				if v, exists := item[from]; exists {
					mapped[to] = v
				}
			}
			items = append(items, mapped)
		}
		mapped := map[string]any{"local_date": body["localDate"], "timezone": body["timezone"], "items": items, "evidence_refs": body["evidenceRefs"]}
		if value, exists := body["expectedRevision"]; exists {
			mapped["expected_revision"] = value
		}
		if value, exists := body["completedBefore"]; exists {
			mapped["completed_before"] = value
		}
		s.callMap(response, request, "/internal/fluctlights/"+escape(fluctlightID)+"/schedules", http.MethodPost, mapped, s.readOnlyError(422, "schedule_accept_failed", "Schedule could not be accepted"), nil)
		return
	}
	if fluctlightID, scheduleID, ok := match2(path, "/api/fluctlights/:fluctlightId/schedules/:scheduleId/cancel"); ok && methodName == http.MethodPost {
		body, valid := s.mutationBody(response, request, validateExpectedRevision)
		if !valid {
			return
		}
		s.callNoContent(response, request, "/internal/fluctlights/"+escape(fluctlightID)+"/schedules/"+escape(scheduleID)+"/cancel", http.MethodPost, map[string]any{"expected_revision": body["expectedRevision"]}, 422, "schedule_cancel_failed", "Schedule could not be cancelled")
		return
	}

	// Moment interactions.
	if momentID, ok := match(path, "/api/moments/:momentId/comments"); ok && methodName == http.MethodPost {
		body, valid := s.mutationBody(response, request, func(value map[string]any) bool { return validateString(value["text"], 1, 32000) })
		if !valid {
			return
		}
		s.callMap(response, request, "/internal/moments/"+escape(momentID)+"/comments", http.MethodPost, map[string]any{"text": body["text"]}, s.readOnlyError(422, "moment_comment_failed", "Moment comment could not be saved"), nil)
		return
	}
	if momentID, ok := match(path, "/api/moments/:momentId/reactions"); ok && methodName == http.MethodPost {
		body, valid := s.mutationBody(response, request, validateReaction)
		if !valid {
			return
		}
		kind := "like"
		if value, exists := body["kind"]; exists {
			kind = stringValue(value)
		}
		s.callMap(response, request, "/internal/moments/"+escape(momentID)+"/reactions", http.MethodPost, map[string]any{"kind": kind}, s.readOnlyError(422, "moment_reaction_failed", "Moment reaction could not be saved"), nil)
		return
	}
	if momentID, action, ok := momentStatus(path); ok && methodName == http.MethodPost {
		s.callNoContent(response, request, "/internal/moments/"+escape(momentID)+"/"+action, http.MethodPost, map[string]any{}, 422, "moment_status_failed", "Moment status could not be changed")
		return
	}

	// Conversation history/read and browser page conversion.
	if conversationID, ok := match(path, "/api/conversations/:conversationId/messages"); ok && methodName == http.MethodGet {
		session, valid := s.requireSession(response, request)
		if !valid {
			return
		}
		query := url.Values{"limit": []string{strconv.Itoa(queryInt(request.URL.Query().Get("limit"), 50))}}
		if before := request.URL.Query().Get("beforeSequence"); before != "" {
			query.Set("before_sequence", before)
		}
		value, err := s.core.doJSON(request.Context(), http.MethodGet, "/internal/conversations/"+escape(conversationID)+"/history?"+query.Encode(), session, nil)
		if err != nil {
			writeError(response, 404, "conversation_not_found", "Conversation is unavailable")
			return
		}
		writeJSON(response, http.StatusOK, browserPage(value))
		return
	}
	if conversationID, ok := match(path, "/api/conversations/:conversationId/read"); ok && methodName == http.MethodPost {
		body, valid := s.mutationBody(response, request, validateReadPosition)
		if !valid {
			return
		}
		mapped := map[string]any{"read_sequence": body["readSequence"]}
		if value, exists := body["deliveredSequence"]; exists {
			mapped["delivered_sequence"] = value
		}
		s.callNoContent(response, request, "/internal/conversations/"+escape(conversationID)+"/read", http.MethodPost, mapped, 403, "conversation_read_failed", "Read state is unavailable")
		return
	}

	// Workflow diagnostics.
	if workflowID, action, ok := workflowRoute(path); ok {
		if methodName == http.MethodGet && (action == "status" || action == "history") {
			s.callMap(response, request, "/internal/diagnostics/workflows/"+escape(workflowID)+"/"+action, http.MethodGet, nil, s.readOnlyError(422, "workflow_"+action+"_failed", "Workflow "+action+" is unavailable"), nil)
			return
		}
		if methodName == http.MethodPost && (action == "pause" || action == "resume" || action == "cancel") {
			s.callNoContent(response, request, "/internal/diagnostics/workflows/"+escape(workflowID)+"/"+action, http.MethodPost, map[string]any{}, 422, "workflow_command_failed", "Workflow command failed")
			return
		}
	}
	if workflowID, ok := match(path, "/api/diagnostics/workflows/:workflowId/reset"); ok && methodName == http.MethodPost {
		body, valid := s.mutationBody(response, request, func(value map[string]any) bool { return validateInteger(value["historyPoint"], 1) })
		if !valid {
			return
		}
		s.callMap(response, request, "/internal/diagnostics/workflows/"+escape(workflowID)+"/reset", http.MethodPost, map[string]any{"history_point": body["historyPoint"]}, s.readOnlyError(422, "workflow_reset_failed", "Workflow reset failed"), nil)
		return
	}
	if workflowID, ok := match(path, "/api/diagnostics/workflows/:workflowId/restart"); ok && methodName == http.MethodPost {
		s.callMap(response, request, "/internal/diagnostics/workflows/"+escape(workflowID)+"/restart", http.MethodPost, map[string]any{}, s.readOnlyError(422, "workflow_restart_failed", "Workflow restart failed"), nil)
		return
	}
	if mediaIntentID, ok := match(path, "/api/diagnostics/media-prompts/:mediaIntentId/retry"); ok && methodName == http.MethodPost {
		s.callMap(response, request, "/internal/diagnostics/media-prompts/"+escape(mediaIntentID)+"/retry", http.MethodPost, map[string]any{}, s.readOnlyError(422, "diagnostics_media_retry_failed", "Media prompt retry failed"), nil)
		return
	}

	if assetID, ok := match(path, "/api/media/:assetId"); ok && methodName == http.MethodGet {
		s.media(response, request, assetID)
		return
	}

	// A known path with the wrong method should not fall through to Core.
	http.NotFound(response, request)
}

type routeError func(http.ResponseWriter, error)

func (s *Server) callMap(response http.ResponseWriter, request *http.Request, endpoint, methodName string, body any, onError routeError, mapper func(map[string]any) any) {
	session, ok := s.requireForMethod(response, request, methodName)
	if !ok {
		return
	}
	value, err := s.core.doJSON(request.Context(), methodName, endpoint, session, body)
	if err != nil {
		onError(response, err)
		return
	}
	if mapper != nil {
		writeJSON(response, http.StatusOK, mapper(value))
		return
	}
	writeJSON(response, http.StatusOK, value)
}

func (s *Server) callAny(response http.ResponseWriter, request *http.Request, endpoint, methodName string, body any, onError routeError, mapper func(any) any) {
	session, ok := s.requireForMethod(response, request, methodName)
	if !ok {
		return
	}
	value, err := s.core.doAny(request.Context(), methodName, endpoint, session, body)
	if err != nil {
		onError(response, err)
		return
	}
	if mapper != nil {
		value = mapper(value)
	}
	writeJSON(response, http.StatusOK, value)
}

func (s *Server) callNoContent(response http.ResponseWriter, request *http.Request, endpoint, methodName string, body any, status int, code, message string) {
	session, ok := s.requireForMethod(response, request, methodName)
	if !ok {
		return
	}
	if _, err := s.core.doJSON(request.Context(), methodName, endpoint, session, body); err != nil {
		writeError(response, status, code, message)
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func (s *Server) requireForMethod(response http.ResponseWriter, request *http.Request, methodName string) (string, bool) {
	if methodName != http.MethodGet && !mutationGuard(response, request, s.trustedOrigin) {
		return "", false
	}
	return s.requireSession(response, request)
}

func (s *Server) requireSession(response http.ResponseWriter, request *http.Request) (string, bool) {
	session := cookieValue(request, sessionCookieName)
	if session == "" {
		writeError(response, http.StatusUnauthorized, "unauthenticated", "Authentication is required")
		return "", false
	}
	return session, true
}

func (s *Server) mutationBody(response http.ResponseWriter, request *http.Request, validator func(map[string]any) bool) (map[string]any, bool) {
	body, ok := readBody(response, request)
	if !ok || !validator(body) {
		if ok {
			writeError(response, http.StatusBadRequest, "invalid_request", "Request validation failed")
		}
		return nil, false
	}
	if !mutationGuard(response, request, s.trustedOrigin) {
		return nil, false
	}
	if _, ok = s.requireSession(response, request); !ok {
		return nil, false
	}
	return body, true
}

func (s *Server) readOnlyError(status int, code, message string) routeError {
	return func(response http.ResponseWriter, _ error) { writeError(response, status, code, message) }
}

func providerRoleError(response http.ResponseWriter, err error) {
	var coreErr *CoreError
	if !errors.As(err, &coreErr) {
		writeError(response, http.StatusBadGateway, "provider_preflight_failed", "Provider preflight failed")
		return
	}
	status := http.StatusUnprocessableEntity
	switch {
	case coreErr.Status == http.StatusUnauthorized:
		status = http.StatusUnauthorized
	case coreErr.Status == http.StatusForbidden:
		status = http.StatusForbidden
	case coreErr.Status >= http.StatusInternalServerError:
		status = http.StatusBadGateway
	}
	messages := map[string]string{
		"provider_endpoint_invalid":    "Provider endpoint configuration is invalid",
		"provider_endpoint_not_found":  "Provider endpoint is not configured",
		"provider_model_not_available": "Selected model is not available on the provider endpoint",
		"provider_models_unavailable":  "Provider model list is unavailable",
		"provider_role_invalid":        "Provider role configuration is invalid",
		"provider_preflight_failed":    "Provider preflight failed",
	}
	code := coreErr.Code
	message, knownCode := messages[code]
	if !knownCode {
		code = "provider_preflight_failed"
		message = messages[code]
	}
	writeErrorWithDetails(response, status, code, message, coreErr.Details)
}

func (s *Server) diagnostics(response http.ResponseWriter, request *http.Request) {
	session, ok := s.requireSession(response, request)
	if !ok {
		return
	}
	query := url.Values{"limit": []string{strconv.Itoa(queryInt(request.URL.Query().Get("limit"), 100))}}
	if value := request.URL.Query().Get("correlationId"); value != "" {
		query.Set("correlation_id", value)
	}
	if value := request.URL.Query().Get("fluctlightId"); value != "" {
		query.Set("fluctlight_id", value)
	}
	var rows []map[string]any
	if err := s.core.doValue(request.Context(), http.MethodGet, "/internal/diagnostics?"+query.Encode(), session, nil, &rows); err != nil {
		diagnosticsError(response, err)
		return
	}
	result := make([]any, 0, len(rows))
	for _, row := range rows {
		result = append(result, browserDiagnostic(row))
	}
	writeJSON(response, http.StatusOK, result)
}

func (s *Server) diagnosticModelRuns(response http.ResponseWriter, request *http.Request) {
	session, ok := s.requireSession(response, request)
	if !ok {
		return
	}
	query := url.Values{"limit": []string{strconv.Itoa(queryInt(request.URL.Query().Get("limit"), 100))}}
	if value := request.URL.Query().Get("correlationId"); value != "" {
		query.Set("correlation_id", value)
	}
	var rows []map[string]any
	if err := s.core.doValue(request.Context(), http.MethodGet, "/internal/diagnostics/model-runs?"+query.Encode(), session, nil, &rows); err != nil {
		diagnosticsError(response, err)
		return
	}
	result := make([]any, 0, len(rows))
	for _, row := range rows {
		result = append(result, browserDiagnosticModelRun(row))
	}
	writeJSON(response, http.StatusOK, result)
}

func (s *Server) diagnosticMediaPrompts(response http.ResponseWriter, request *http.Request) {
	session, ok := s.requireSession(response, request)
	if !ok {
		return
	}
	query := url.Values{"limit": []string{strconv.Itoa(queryInt(request.URL.Query().Get("limit"), 20))}}
	var rows []map[string]any
	if err := s.core.doValue(request.Context(), http.MethodGet, "/internal/diagnostics/media-prompts?"+query.Encode(), session, nil, &rows); err != nil {
		diagnosticsError(response, err)
		return
	}
	result := make([]any, 0, len(rows))
	for _, row := range rows {
		result = append(result, browserDiagnosticMediaPrompt(row))
	}
	writeJSON(response, http.StatusOK, result)
}

func (s *Server) diagnosticsExport(response http.ResponseWriter, request *http.Request) {
	session, ok := s.requireSession(response, request)
	if !ok {
		return
	}
	query := url.Values{"limit": []string{strconv.Itoa(queryInt(request.URL.Query().Get("limit"), 500))}}
	if value := request.URL.Query().Get("correlationId"); value != "" {
		query.Set("correlation_id", value)
	}
	var value map[string]any
	if err := s.core.doValue(request.Context(), http.MethodGet, "/internal/diagnostics/export?"+query.Encode(), session, nil, &value); err != nil {
		diagnosticsError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, value)
}

func (s *Server) clearDiagnostics(response http.ResponseWriter, request *http.Request) {
	if !mutationGuard(response, request, s.trustedOrigin) {
		return
	}
	session, ok := s.requireSession(response, request)
	if !ok {
		return
	}
	value, err := s.core.doJSON(request.Context(), http.MethodDelete, "/internal/diagnostics", session, nil)
	if err != nil {
		writeError(response, http.StatusForbidden, "diagnostics_clear_failed", "Diagnostics could not be cleared")
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"cleared": numberValue(value["cleared"])})
}

func diagnosticsError(response http.ResponseWriter, err error) {
	var coreErr *CoreError
	if errors.As(err, &coreErr) {
		switch {
		case coreErr.Status == http.StatusUnauthorized:
			writeError(response, 401, "unauthenticated", "Authentication is required")
		case coreErr.Status == http.StatusForbidden:
			writeError(response, 403, "diagnostics_forbidden", "Diagnostics are available to the owner only")
		case coreErr.Status >= 500:
			writeError(response, 503, "diagnostics_runtime_unavailable", "Diagnostics runtime is unavailable")
		case coreErr.Status == http.StatusUnprocessableEntity:
			writeErrorWithDetails(response, 422, coreErr.Code, "Diagnostics request failed", coreErr.Details)
		default:
			writeErrorWithDetails(response, 502, coreErr.Code, "Diagnostics request failed", coreErr.Details)
		}
		return
	}
	writeError(response, 502, "diagnostics_unavailable", "Diagnostics are unavailable")
}

func (s *Server) creationError(response http.ResponseWriter, err error, operation string) {
	var coreErr *CoreError
	if errors.As(err, &coreErr) {
		if coreErr.Status == 401 {
			writeError(response, 401, "unauthenticated", "Authentication is required")
			return
		}
		if coreErr.Status == 422 {
			message := "Fluctlight analysis was rejected"
			if operation == "activation" {
				message = "Fluctlight activation was rejected"
			}
			writeErrorWithDetails(response, 422, coreErr.Code, message, coreErr.Details)
			return
		}
		if coreErr.Status >= 500 {
			message := "Fluctlight analysis service is unavailable"
			if operation == "activation" {
				message = "Fluctlight activation service is unavailable"
				writeError(response, http.StatusServiceUnavailable, coreErr.Code, message)
				return
			}
			writeErrorWithDetails(response, 503, coreErr.Code, message, coreErr.Details)
			return
		}
	}
	message, code := "Fluctlight analysis service is unavailable", "fluctlight_analysis_unavailable"
	if operation == "activation" {
		message, code = "Fluctlight activation service is unavailable", "fluctlight_activation_unavailable"
	}
	writeError(response, 502, code, message)
}

func (s *Server) media(response http.ResponseWriter, request *http.Request, assetID string) {
	session, ok := s.requireSession(response, request)
	if !ok {
		return
	}
	extra := make(http.Header)
	if value := request.Header.Get("Range"); value != "" {
		extra.Set("Range", value)
	}
	upstream, err := s.core.request(request.Context(), http.MethodGet, "/internal/media/"+escape(assetID), session, nil, extra)
	if err != nil {
		writeError(response, http.StatusNotFound, "media_unavailable", "Media is unavailable")
		return
	}
	if upstream.Body == nil || upstream.Body == http.NoBody || upstream.ContentLength == 0 {
		writeError(response, http.StatusBadGateway, "media_unavailable", "Media is unavailable")
		return
	}
	defer upstream.Body.Close()
	response.Header().Set("Content-Type", "application/octet-stream")
	for _, name := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "ETag"} {
		if value := upstream.Header.Get(name); value != "" {
			response.Header().Set(name, value)
		}
	}
	response.WriteHeader(upstream.StatusCode)
	_, _ = io.Copy(response, upstream.Body)
}

func method(response http.ResponseWriter, request *http.Request, allowed string) bool {
	if request.Method == allowed {
		return true
	}
	methodNotAllowed(response, allowed)
	return false
}

func methodNotAllowed(response http.ResponseWriter, allowed string) {
	response.Header().Set("Allow", allowed)
	writeError(response, http.StatusMethodNotAllowed, "method_not_allowed", "Method is not allowed")
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func writeError(response http.ResponseWriter, status int, code, message string) {
	writeJSON(response, status, map[string]any{"code": code, "message": message})
}

func writeErrorWithDetails(response http.ResponseWriter, status int, code, message string, details map[string]any) {
	value := map[string]any{"code": code, "message": message}
	if safeDetails := sanitizePublicErrorDetails(details); len(safeDetails) > 0 {
		value["details"] = safeDetails
	}
	writeJSON(response, status, value)
}

func readBody(response http.ResponseWriter, request *http.Request) (map[string]any, bool) {
	if request.Body == nil {
		writeError(response, 400, "invalid_request", "Request validation failed")
		return nil, false
	}
	defer request.Body.Close()
	data, err := io.ReadAll(io.LimitReader(request.Body, (1<<20)+1))
	if err != nil {
		writeError(response, 400, "invalid_request", "Request validation failed")
		return nil, false
	}
	if len(data) > 1<<20 {
		writeError(response, 400, "invalid_request", "Request validation failed")
		return nil, false
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		writeError(response, 400, "invalid_request", "Request validation failed")
		return nil, false
	}
	var body map[string]any
	if json.Unmarshal(data, &body) != nil || body == nil {
		writeError(response, 400, "invalid_request", "Request validation failed")
		return nil, false
	}
	return body, true
}

func match(path, pattern string) (string, bool) {
	pathParts, patternParts := splitPath(path), splitPath(pattern)
	if len(pathParts) != len(patternParts) {
		return "", false
	}
	for index := range patternParts {
		if strings.HasPrefix(patternParts[index], ":") {
			continue
		}
		if pathParts[index] != patternParts[index] {
			return "", false
		}
	}
	for index, part := range patternParts {
		if strings.HasPrefix(part, ":") {
			value, _ := url.PathUnescape(pathParts[index])
			return value, true
		}
	}
	return "", false
}

func match2(path, pattern string) (string, string, bool) {
	pathParts, patternParts := splitPath(path), splitPath(pattern)
	if len(pathParts) != len(patternParts) {
		return "", "", false
	}
	values := []string{}
	for index := range patternParts {
		if strings.HasPrefix(patternParts[index], ":") {
			value, _ := url.PathUnescape(pathParts[index])
			values = append(values, value)
		} else if pathParts[index] != patternParts[index] {
			return "", "", false
		}
	}
	if len(values) != 2 {
		return "", "", false
	}
	return values[0], values[1], true
}

func splitPath(path string) []string {
	if path == "/" || path == "" {
		return nil
	}
	if !strings.HasPrefix(path, "/") {
		return nil
	}
	return strings.Split(strings.TrimPrefix(path, "/"), "/")
}
func escape(value string) string { return url.PathEscape(value) }

func foundationAction(path string) (string, string, string, bool) {
	parts := splitPath(path)
	if len(parts) != 6 || parts[0] != "api" || parts[1] != "fluctlights" || parts[3] != "foundation-revisions" || (parts[5] != "accept" && parts[5] != "reject") {
		return "", "", "", false
	}
	return parts[2], parts[4], parts[5], true
}

func developingSelfAction(path string) (string, string, string, bool) {
	parts := splitPath(path)
	if len(parts) != 6 || parts[0] != "api" || parts[1] != "fluctlights" || parts[3] != "developing-self" || (parts[5] != "rollback" && parts[5] != "forget") {
		return "", "", "", false
	}
	return parts[2], parts[4], parts[5], true
}

func momentStatus(path string) (string, string, bool) {
	parts := splitPath(path)
	if len(parts) == 4 && parts[0] == "api" && parts[1] == "moments" && (parts[3] == "hide" || parts[3] == "restore") {
		return parts[2], parts[3], true
	}
	return "", "", false
}
func workflowRoute(path string) (string, string, bool) {
	parts := splitPath(path)
	if len(parts) == 5 && parts[0] == "api" && parts[1] == "diagnostics" && parts[2] == "workflows" {
		return parts[3], parts[4], true
	}
	return "", "", false
}

func validatePassword(value any) bool { return validateString(value, 6, 1<<20) }
func validateString(value any, min, max int) bool {
	text, ok := value.(string)
	return ok && utf16Length(text) >= min && utf16Length(text) <= max
}

func utf16Length(value string) int { return len(utf16.Encode([]rune(value))) }
func validateInteger(value any, min int) bool {
	number, ok := value.(float64)
	return ok && math.Trunc(number) == number && number >= float64(min)
}
func validateNumber(value any, min, max float64) bool {
	number, ok := value.(float64)
	return ok && !math.IsNaN(number) && number >= min && number <= max
}
func validateRevisionReason(value map[string]any) bool {
	return validateInteger(value["expectedRevision"], 0) && validateString(value["reason"], 1, 1024)
}
func validateExpectedRevision(value map[string]any) bool {
	return validateInteger(value["expectedRevision"], 0)
}
func validateProviderEndpoint(value map[string]any) bool {
	return validateString(value["endpointId"], 1, 128) && validateString(value["kind"], 1, 64) && validateString(value["baseUrl"], 8, 1<<20) && validateString(value["secretPurpose"], 1, 128)
}
func validateModelRole(value map[string]any) bool {
	role := stringValue(value["role"])
	legacy := map[string]struct{}{"initialization": {}, "cognitive_assessment": {}, "action_realization": {}, "interaction": {}, "reflection": {}, "media_prompt": {}}
	_, legacyRole := legacy[role]
	return (role == "generic_llm" || role == "embedding" || legacyRole) && validateString(value["endpointId"], 1, 128) && validateString(value["modelId"], 1, 256) && validateInteger(value["tokenBudget"], 1) && validateInteger(value["timeoutSeconds"], 1)
}
func validateSettings(value map[string]any) bool {
	if values, ok := value["values"]; ok && !isObject(values) {
		return false
	}
	if secrets, ok := value["secrets"]; ok && !isObject(secrets) {
		return false
	}
	if clear, ok := value["clearSecrets"]; ok && !isStringArray(clear) {
		return false
	}
	return true
}
func validateCapabilityRequestReview(value map[string]any) bool {
	if !validateString(value["status"], 1, 32) {
		return false
	}
	status := stringValue(value["status"])
	if status != "reviewing" && status != "accepted" && status != "rejected" && status != "fulfilled" && status != "cancelled" {
		return false
	}
	if note, exists := value["note"]; !exists || !validateString(note, 1, 2000) {
		return false
	}
	if version, exists := value["capabilityVersion"]; exists && !validateString(version, 0, 128) {
		return false
	}
	return status != "fulfilled" || validateString(value["capabilityVersion"], 1, 128)
}
func validateConversationCreate(value map[string]any) bool {
	actors, ok := value["participantActorIds"]
	if !ok || len(stringArray(actors)) != len(array(actors)) || len(array(actors)) < 1 || len(array(actors)) > 1 {
		return false
	}
	if title, exists := value["title"]; exists && !validateString(title, 0, 256) {
		return false
	}
	return true
}
func validateFluctlightCreate(value map[string]any) bool {
	if id, ok := value["id"]; ok && !validateString(id, 1, 128) {
		return false
	}
	if name, ok := value["name"]; ok && !validateString(name, 0, 256) {
		return false
	}
	return true
}
func validateActivation(value map[string]any) bool {
	mode := stringValue(value["initializationMode"])
	if !validateString(value["requestId"], 1, 256) || (mode != "blank_slate" && mode != "llm_defined") {
		return false
	}
	if mode == "llm_defined" && (!isObject(value["corePersona"]) || !isObject(value["developingSelf"])) {
		return false
	}
	if mode == "blank_slate" && !validateString(value["name"], 1, 256) {
		return false
	}
	if core, exists := value["corePersona"]; exists && !isObject(core) {
		return false
	}
	if self, exists := value["developingSelf"]; exists && !isObject(self) {
		return false
	}
	for _, key := range []string{"initialGoals", "initialIntentions"} {
		if item, exists := value[key]; exists {
			if _, ok := item.([]any); !ok {
				return false
			}
			for _, child := range array(item) {
				if !isObject(child) {
					return false
				}
			}
		}
	}
	return true
}
func validateFluctlightStatus(value map[string]any) bool {
	status := stringValue(value["status"])
	return (status == "active" || status == "paused") && validateRevisionReason(value)
}
func validateFoundationRevision(value map[string]any) bool {
	return isObject(value["changes"]) && len(objectValue(value["changes"])) > 0 && validateRevisionReason(value)
}
func validateFoundationRollback(value map[string]any) bool {
	return validateRevisionReason(value) && validateInteger(value["targetRevision"], 0)
}
func validateEvidence(value any) bool {
	values := stringArray(value)
	return len(values) == len(array(value)) && len(values) >= 1 && len(values) <= 32
}
func validateMemoryRevision(value map[string]any) bool {
	return validateInteger(value["expectedRevision"], 0) && validateString(value["content"], 1, 4096) && validateEvidence(value["evidenceRefs"])
}
func validateMemoryForget(value map[string]any) bool {
	return validateInteger(value["expectedRevision"], 0) && validateEvidence(value["evidenceRefs"])
}
func validateRelationshipRollback(value map[string]any) bool {
	return validateString(value["targetActorId"], 1, 128) && validateInteger(value["targetRevision"], 0) && validateInteger(value["expectedRevision"], 0) && validateEvidence(value["evidenceRefs"])
}
func validateAutonomyGovernance(value map[string]any) bool {
	status := stringValue(value["status"])
	return (status == "paused" || status == "deferred" || status == "cancelled") && validateString(value["reason"], 1, 1024)
}
func validateLifeEvent(value map[string]any) bool {
	if !validateString(value["kind"], 1, 128) || !validateString(value["startAt"], 1, 1<<20) || !validateString(value["endAt"], 1, 1<<20) || !validateEvidence(value["evidenceRefs"]) {
		return false
	}
	for _, key := range []string{"scene", "activity", "location"} {
		if item, exists := value[key]; exists && !validateString(item, 0, 512) {
			return false
		}
	}
	return true
}
func validatePresence(value map[string]any) bool {
	if task, ok := value["currentTask"]; ok && !validateString(task, 0, 512) {
		return false
	}
	if presence, ok := value["userPresence"]; ok && !validateString(presence, 0, 128) {
		return false
	}
	return true
}
func validateSchedule(value map[string]any) bool {
	items := array(value["items"])
	if len(items) < 1 || len(items) > 128 || !validateString(value["localDate"], 10, 10) || !validateString(value["timezone"], 1, 128) || !validateEvidence(value["evidenceRefs"]) {
		return false
	}
	for _, raw := range items {
		item := objectValue(raw)
		if !validateString(item["startAt"], 1, 1<<20) || !validateString(item["endAt"], 1, 1<<20) || !validateString(item["activity"], 1, 128) || !validateString(item["scene"], 1, 128) {
			return false
		}
		for _, key := range []string{"priority", "flexibility", "interruptionCost"} {
			if v, ok := item[key]; ok && !validateNumber(v, 0, 1) {
				return false
			}
		}
		for _, key := range []string{"itemType", "status"} {
			if value, exists := item[key]; exists && !validateString(value, 1, 128) {
				return false
			}
		}
	}
	if v, ok := value["expectedRevision"]; ok && !validateInteger(v, 0) {
		return false
	}
	return true
}
func validateReaction(value map[string]any) bool {
	if kind, ok := value["kind"]; ok {
		k := stringValue(kind)
		return k == "like" || k == "care" || k == "celebrate"
	}
	return true
}
func validateReadPosition(value map[string]any) bool {
	if !validateInteger(value["readSequence"], 0) {
		return false
	}
	if v, ok := value["deliveredSequence"]; ok && !validateInteger(v, 0) {
		return false
	}
	return true
}

func validateConversationTurn(value map[string]any) bool {
	if !validateString(value["text"], 1, 32000) || !validateString(value["fluctlightId"], 1, 128) || !validateString(value["idempotencyKey"], 1, 256) {
		return false
	}
	if attachmentRefs, ok := value["attachmentRefs"]; ok {
		if _, isArray := attachmentRefs.([]any); !isArray {
			return false
		}
		refs := stringArray(attachmentRefs)
		if len(refs) != len(array(attachmentRefs)) || len(refs) > 16 {
			return false
		}
		for _, ref := range refs {
			if !validateString(ref, 1, 512) {
				return false
			}
		}
	}
	if turnID, ok := value["turnId"]; ok && !validateString(turnID, 1, 256) {
		return false
	}
	return true
}

func array(value any) []any {
	if values, ok := value.([]any); ok {
		return values
	}
	return []any{}
}
func objectValue(value any) map[string]any {
	if values, ok := value.(map[string]any); ok {
		return values
	}
	return map[string]any{}
}
func object(value any) map[string]any { return objectValue(value) }
func isObject(value any) bool         { _, ok := value.(map[string]any); return ok }
func isStringArray(value any) bool {
	values, ok := value.([]any)
	if !ok {
		return false
	}
	for _, entry := range values {
		if _, ok := entry.(string); !ok {
			return false
		}
	}
	return true
}
func stringArray(value any) []any {
	values := array(value)
	result := make([]any, 0, len(values))
	for _, entry := range values {
		if _, ok := entry.(string); ok {
			result = append(result, entry)
		}
	}
	return result
}
func stringValue(value any) string { text, _ := value.(string); return text }
func first(value map[string]any, names ...string) any {
	for _, name := range names {
		if result, ok := value[name]; ok {
			return result
		}
	}
	return nil
}
func truthy(value any) bool { result, ok := value.(bool); return ok && result }
func numberValue(value any) any {
	if value == nil {
		return float64(0)
	}
	return value
}
func queryInt(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

func mapSettings(value map[string]any) map[string]any {
	configuredSecrets := stringArray(value["configured_secrets"])
	if _, exists := value["configured_secrets"]; !exists {
		configuredSecrets = stringArray(value["configuredSecrets"])
	}
	return map[string]any{"values": objectValue(value["values"]), "configuredSecrets": configuredSecrets}
}

package httpapi

// This file is the in-process implementation of the browser transport
// boundary. It replaces the former gateway -> Core HTTP client with explicit
// calls to the existing App/Repository services. No browser request is
// converted into a second HTTP request and no internal handler is invoked.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/httpapi/browser"
)

type browserBackend struct{ server *Server }

func newBrowserBackend(server *Server) browser.Backend { return &browserBackend{server: server} }

func (b *browserBackend) Health(ctx context.Context) error {
	if b == nil || b.server == nil || b.server.repository == nil {
		return errors.New("API repository is unavailable")
	}
	return b.server.repository.Ping(ctx)
}

func (b *browserBackend) DoJSON(ctx context.Context, method, endpoint, session string, body any) (map[string]any, error) {
	value, err := b.dispatch(ctx, method, endpoint, session, body)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return map[string]any{}, nil
	}
	result, ok := value.(map[string]any)
	if ok {
		return result, nil
	}
	return jsonMap(value)
}

func (b *browserBackend) DoAny(ctx context.Context, method, endpoint, session string, body any) (any, error) {
	return b.dispatch(ctx, method, endpoint, session, body)
}

func (b *browserBackend) DoValue(ctx context.Context, method, endpoint, session string, body any, out any) error {
	value, err := b.dispatch(ctx, method, endpoint, session, body)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, out)
}

func (b *browserBackend) StreamTurn(ctx context.Context, session, conversationID string, payload map[string]any, writer http.ResponseWriter) error {
	actorID, err := b.resolveActor(ctx, session)
	if err != nil {
		return err
	}
	writer.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	if err := b.server.app.StreamTurn(ctx, writer, actorID, conversationID, payload); err != nil {
		return browserConversationTurnError(err)
	}
	return nil
}

func browserConversationTurnError(err error) error {
	if err == nil {
		return nil
	}
	if code := core.ConversationTurnFailureCode(err); code != "" {
		return &browser.CoreError{Status: http.StatusBadGateway, Code: code, Message: "conversation turn failed"}
	}
	return err
}

func (b *browserBackend) Media(ctx context.Context, session, assetID, rangeHeader string, writer http.ResponseWriter) error {
	actorID, err := b.resolveActor(ctx, session)
	if err != nil {
		return err
	}
	if err := b.server.app.AuthorizeAsset(ctx, actorID, assetID); err != nil {
		return browserBackendError(err, "media_unavailable")
	}
	if err := b.server.app.ServeMedia(ctx, writer, assetID, rangeHeader); err != nil {
		return browserBackendError(err, "media_unavailable")
	}
	return nil
}

func (b *browserBackend) dispatch(ctx context.Context, method, endpoint, session string, body any) (any, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, browserBackendError(err, "core_request_invalid")
	}
	path := parsed.Path
	values := objectBody(body)

	// Anonymous authentication operations retain the public session contract.
	switch path {
	case "/internal/platform/ping":
		if method != http.MethodGet {
			return nil, browserBackendError(errors.New("method not allowed"), "method_not_allowed")
		}
		return map[string]any{"status": "ok", "role": "api"}, nil
	case "/internal/auth/session":
		if strings.TrimSpace(session) == "" {
			return map[string]any{"authenticated": false}, nil
		}
		actorID, resolveErr := b.server.repository.ResolveSession(ctx, session)
		if errors.Is(resolveErr, core.ErrUnauthorized) {
			return map[string]any{"authenticated": false}, nil
		}
		if resolveErr != nil {
			return nil, browserBackendError(resolveErr, "core_go_session_failed")
		}
		return map[string]any{"authenticated": true, "actor_id": actorID}, nil
	case "/internal/auth/setup-status":
		available, setupErr := b.server.app.SetupAvailable(ctx)
		if setupErr != nil {
			return nil, browserBackendError(setupErr, "setup_status_failed")
		}
		return map[string]any{"setup_available": available}, nil
	case "/internal/auth/login":
		actorID, token, loginErr := b.server.app.Login(ctx, stringValue(values["password"]))
		if loginErr != nil {
			return nil, &browser.CoreError{Status: http.StatusUnauthorized, Code: "auth_failed", Message: "authentication failed"}
		}
		return map[string]any{"authenticated": true, "actor_id": actorID, "session_token": token}, nil
	case "/internal/auth/setup":
		actorID, token, setupErr := b.server.app.Setup(ctx, stringValue(values["setup_token"]), stringValue(values["password"]))
		if setupErr != nil {
			return nil, &browser.CoreError{Status: http.StatusForbidden, Code: "setup_unavailable", Message: "setup unavailable"}
		}
		return map[string]any{"authenticated": true, "actor_id": actorID, "session_token": token}, nil
	}

	actorID, err := b.resolveActor(ctx, session)
	if err != nil {
		return nil, err
	}

	switch {
	case path == "/internal/auth/revoke-all":
		err = b.server.app.RevokeAll(ctx, actorID)
	case path == "/internal/auth/revoke-current":
		err = b.server.app.RevokeCurrent(ctx, actorID, session)
	case path == "/internal/auth/reset-password":
		err = b.server.app.ResetPassword(ctx, actorID, stringValue(values["password"]))
	case path == "/internal/settings" && method == http.MethodGet:
		return b.server.app.ReadSettings(ctx, actorID)
	case path == "/internal/settings" && method == http.MethodPut:
		return b.server.app.UpdateSettings(ctx, actorID, values)
	case path == "/internal/providers/endpoints" && method == http.MethodGet:
		return b.server.app.ProviderEndpoints(ctx, actorID)
	case path == "/internal/providers/endpoints" && method == http.MethodPut:
		err = b.server.app.ConfigureProviderEndpoint(ctx, actorID, values)
	case path == "/internal/providers" && method == http.MethodGet:
		return b.server.app.ProviderBindings(ctx, actorID)
	case strings.HasPrefix(path, "/internal/providers/endpoints/") && strings.HasSuffix(path, "/models"):
		parts := splitInternalPath(path)
		return b.server.app.ProviderModels(ctx, actorID, pathPart(parts, 3))
	case path == "/internal/providers/roles" && method == http.MethodPut:
		err = b.server.app.ConfigureProviderRole(ctx, actorID, values)
		if err == nil {
			return map[string]any{"role": values["role"], "available": true, "capability_version": "models-v1"}, nil
		}
	case path == "/internal/actor-groups" && method == http.MethodGet:
		return b.server.app.ActorGroups(ctx, actorID)
	case path == "/internal/actor-groups" && method == http.MethodPost:
		return b.server.app.CreateActorGroup(ctx, actorID, stringValue(values["name"]))
	case strings.HasPrefix(path, "/internal/actor-groups/") && strings.HasSuffix(path, "/members") && method == http.MethodPost:
		parts := splitInternalPath(path)
		err = b.server.app.SetActorGroupMember(ctx, actorID, pathPart(parts, 2), stringValue(values["actor_id"]), true)
	case strings.Contains(path, "/members/") && method == http.MethodDelete:
		parts := splitInternalPath(path)
		err = b.server.app.SetActorGroupMember(ctx, actorID, pathPart(parts, 2), pathPart(parts, 4), false)
	case path == "/internal/fluctlights" && method == http.MethodGet:
		return b.server.repository.ListFluctlights(ctx, actorID)
	case path == "/internal/fluctlights" && method == http.MethodPost:
		item, createErr := b.server.app.CreateFluctlight(ctx, actorID, stringValue(values["id"]), stringValue(values["name"]), "blank_slate", "", nil, nil, nil)
		if createErr != nil {
			return nil, browserBackendError(createErr, "fluctlight_create_failed")
		}
		return map[string]any{"id": item.ID, "identity": item.Identity, "status": item.Status}, nil
	case strings.HasPrefix(path, "/internal/fluctlights/") && method == http.MethodGet && !strings.Contains(path, "/conversation") && !strings.Contains(path, "/detail") && !strings.Contains(path, "/developing-self") && !strings.Contains(path, "/moments") && !strings.Contains(path, "/autonomy-actions"):
		parts := splitInternalPath(path)
		item, getErr := b.server.repository.GetFluctlight(ctx, pathPart(parts, 2), actorID)
		if getErr != nil {
			return nil, getErr
		}
		return jsonMap(item)
	case path == "/internal/fluctlight-creations/analysis":
		return b.server.app.AnalyzeDescription(ctx, actorID, stringValue(values["description"]))
	case path == "/internal/fluctlight-creations/activate":
		return b.activate(ctx, actorID, values)
	case strings.HasSuffix(path, "/developing-self") && method == http.MethodGet:
		parts := splitInternalPath(path)
		if _, err = b.server.app.DB.GetFluctlight(ctx, pathPart(parts, 2), actorID); err != nil {
			break
		}
		claims, claimsErr := b.server.app.ListDevelopingSelfClaims(ctx, pathPart(parts, 2))
		if claimsErr != nil {
			return nil, claimsErr
		}
		revisions, revisionsErr := b.server.app.ListDevelopingSelfRevisions(ctx, pathPart(parts, 2))
		if revisionsErr != nil {
			return nil, revisionsErr
		}
		return map[string]any{"claims": claims, "revisions": revisions}, nil
	case strings.Contains(path, "/developing-self/"):
		parts := splitInternalPath(path)
		action := pathPart(parts, 5)
		expected := intValue(values["expected_revision"])
		if action == "rollback" {
			return b.server.app.RollbackDevelopingSelfClaim(ctx, actorID, pathPart(parts, 4), expected, stringValue(values["reason"]))
		}
		return b.server.app.ForgetDevelopingSelfClaim(ctx, actorID, pathPart(parts, 4), expected, stringValue(values["reason"]))
	case strings.HasSuffix(path, "/conversation") && method == http.MethodGet:
		parts := splitInternalPath(path)
		conversationID, convErr := b.server.repository.DirectConversationID(ctx, actorID, pathPart(parts, 2))
		if errors.Is(convErr, core.ErrNotFound) {
			conversationID, convErr = b.server.app.EnsureDirectConversation(ctx, actorID, pathPart(parts, 2))
		}
		if convErr != nil {
			return nil, convErr
		}
		page, pageErr := b.server.repository.History(ctx, conversationID, actorID, nil, 50)
		if pageErr != nil {
			return nil, pageErr
		}
		return jsonMap(page)
	case strings.HasSuffix(path, "/detail") && method == http.MethodGet:
		parts := splitInternalPath(path)
		return b.server.app.FluctlightDetail(ctx, actorID, pathPart(parts, 2))
	case strings.HasPrefix(path, "/internal/fluctlights/") && strings.HasSuffix(path, "/autonomy-actions") && method == http.MethodGet:
		parts := splitInternalPath(path)
		return b.server.app.AutonomyActions(ctx, actorID, pathPart(parts, 2))
	case strings.HasPrefix(path, "/internal/fluctlights/") && strings.HasSuffix(path, "/wake-up") && method == http.MethodPost:
		parts := splitInternalPath(path)
		return b.server.app.TriggerWakeUp(ctx, actorID, pathPart(parts, 2))
	case strings.HasPrefix(path, "/internal/fluctlights/") && strings.HasSuffix(path, "/moments") && method == http.MethodGet:
		parts := splitInternalPath(path)
		return b.server.app.MomentsWithOptions(ctx, actorID, pathPart(parts, 2), parsed.Query().Get("include_hidden") == "true", queryLimitValue(parsed.Query().Get("limit")))
	case path == "/internal/capability-requests" && method == http.MethodGet:
		return b.server.app.ListCapabilityRequests(ctx, actorID)
	case strings.HasPrefix(path, "/internal/capability-requests/"):
		parts := splitInternalPath(path)
		return b.server.app.ReviewCapabilityRequest(ctx, actorID, pathPart(parts, 2), stringValue(values["status"]), stringValue(values["note"]), stringValue(values["capability_version"]))
	case strings.HasPrefix(path, "/internal/fluctlights/") && strings.HasSuffix(path, "/status"):
		parts := splitInternalPath(path)
		return b.server.app.SetFluctlightStatus(ctx, actorID, pathPart(parts, 2), stringValue(values["status"]), stringValue(values["reason"]), intPtr(values["expected_revision"]))
	case strings.HasSuffix(path, "/retire"):
		parts := splitInternalPath(path)
		return b.server.app.RetireFluctlight(ctx, actorID, pathPart(parts, 2), stringValue(values["reason"]), intPtr(values["expected_revision"]))
	case strings.HasSuffix(path, "/foundation-revisions") && method == http.MethodPost:
		parts := splitInternalPath(path)
		return b.server.app.ProposeFoundation(ctx, actorID, pathPart(parts, 2), values)
	case strings.Contains(path, "/foundation-revisions/") && (strings.HasSuffix(path, "/accept") || strings.HasSuffix(path, "/reject")):
		parts := splitInternalPath(path)
		return b.server.app.SetFoundationDecisionExpected(ctx, actorID, pathPart(parts, 2), pathPart(parts, 4), pathPart(parts, 5), stringValue(values["reason"]), intPtr(values["expected_revision"]))
	case strings.HasSuffix(path, "/foundation-revisions/rollback"):
		parts := splitInternalPath(path)
		return b.server.app.RollbackFoundation(ctx, actorID, pathPart(parts, 2), values)
	case strings.HasPrefix(path, "/internal/memories/") && method == http.MethodPut:
		parts := splitInternalPath(path)
		return b.server.app.ReviseMemory(ctx, actorID, pathPart(parts, 2), stringValue(values["content"]), intPtr(values["expected_revision"]), arrayValue(values["evidence_refs"]))
	case strings.HasSuffix(path, "/forget") && strings.HasPrefix(path, "/internal/memories/"):
		parts := splitInternalPath(path)
		return b.server.app.ForgetMemory(ctx, actorID, pathPart(parts, 2), intPtr(values["expected_revision"]), arrayValue(values["evidence_refs"]))
	case strings.HasSuffix(path, "/relationships/rollback"):
		parts := splitInternalPath(path)
		return b.server.app.RollbackRelationship(ctx, actorID, pathPart(parts, 2), values)
	case strings.Contains(path, "/relationships/") && method == http.MethodPut:
		parts := splitInternalPath(path)
		return b.server.app.EditRelationship(ctx, actorID, pathPart(parts, 2), pathPart(parts, 4), values)
	case strings.HasPrefix(path, "/internal/autonomy-actions/"):
		parts := splitInternalPath(path)
		return b.server.app.GovernAutonomy(ctx, actorID, pathPart(parts, 2), stringValue(values["status"]), stringValue(values["reason"]))
	case strings.HasSuffix(path, "/events") && method == http.MethodPost:
		parts := splitInternalPath(path)
		return b.server.app.CreateLifeEvent(ctx, actorID, pathPart(parts, 2), values)
	case strings.Contains(path, "/events/") && strings.HasSuffix(path, "/cancel"):
		parts := splitInternalPath(path)
		return b.server.app.CancelLifeEvent(ctx, actorID, pathPart(parts, 2), pathPart(parts, 4), values)
	case strings.HasSuffix(path, "/presence"):
		parts := splitInternalPath(path)
		return b.server.app.SetPresence(ctx, actorID, pathPart(parts, 2), values)
	case strings.HasSuffix(path, "/schedules") && method == http.MethodPost:
		parts := splitInternalPath(path)
		if strings.TrimSpace(stringValue(values["completed_before"])) != "" {
			return b.server.app.ReplanSchedule(ctx, actorID, pathPart(parts, 2), values)
		}
		return b.server.app.AcceptSchedule(ctx, actorID, pathPart(parts, 2), values)
	case strings.Contains(path, "/schedules/") && strings.HasSuffix(path, "/cancel"):
		parts := splitInternalPath(path)
		return b.server.app.CancelScheduleExpected(ctx, actorID, pathPart(parts, 2), pathPart(parts, 4), values)
	case path == "/internal/moments" && method == http.MethodGet:
		return b.server.app.AllMoments(ctx, actorID, parsed.Query().Get("include_hidden") == "true", queryLimitValue(parsed.Query().Get("limit")))
	case strings.HasSuffix(path, "/moments/read"):
		parts := splitInternalPath(path)
		err = b.server.app.MarkMomentsRead(ctx, actorID, pathPart(parts, 2))
	case strings.Contains(path, "/comments"):
		parts := splitInternalPath(path)
		return b.server.app.CommentMoment(ctx, actorID, pathPart(parts, 2), stringValue(values["text"]))
	case strings.Contains(path, "/reactions"):
		parts := splitInternalPath(path)
		return b.server.app.ReactMoment(ctx, actorID, pathPart(parts, 2), stringValue(values["kind"]))
	case strings.HasSuffix(path, "/hide"):
		parts := splitInternalPath(path)
		err = b.server.app.SetMomentStatus(ctx, actorID, pathPart(parts, 2), "hidden")
	case strings.HasSuffix(path, "/restore"):
		parts := splitInternalPath(path)
		err = b.server.app.SetMomentStatus(ctx, actorID, pathPart(parts, 2), "visible")
	case path == "/internal/conversations" && method == http.MethodPost:
		return b.server.app.CreateConversation(ctx, actorID, arrayValue(values["participant_actor_ids"]), stringValue(values["title"]))
	case strings.HasSuffix(path, "/read") && strings.Contains(path, "/internal/conversations/"):
		parts := splitInternalPath(path)
		err = b.server.app.MarkRead(ctx, actorID, pathPart(parts, 2), values)
	case strings.HasSuffix(path, "/history"):
		parts := splitInternalPath(path)
		before := intPtrFromQuery(parsed.Query().Get("before_sequence"))
		if before == nil {
			before = intPtrFromQuery(parsed.Query().Get("before"))
		}
		return jsonMapMust(b.server.repository.History(ctx, pathPart(parts, 2), actorID, before, queryLimitValue(parsed.Query().Get("limit"))))
	case path == "/internal/diagnostics" && method == http.MethodGet:
		return b.server.app.DiagnosticsFiltered(ctx, actorID, queryLimitValue(parsed.Query().Get("limit")), parsed.Query().Get("correlation_id"), parsed.Query().Get("fluctlight_id"))
	case path == "/internal/diagnostics" && method == http.MethodDelete:
		cleared, clearErr := b.server.app.ClearDiagnosticsCount(ctx, actorID)
		return map[string]any{"cleared": cleared}, clearErr
	case path == "/internal/diagnostics/lifecycle":
		return b.server.app.LifecycleDiagnostics(ctx, actorID, lifecycleFilter(parsed.Query()))
	case path == "/internal/diagnostics/model-runs":
		return b.server.app.ModelRunsFiltered(ctx, actorID, queryLimitValue(parsed.Query().Get("limit")), parsed.Query().Get("correlation_id"))
	case path == "/internal/diagnostics/media-prompts":
		return b.server.app.MediaPromptsFiltered(ctx, actorID, queryLimitValue(parsed.Query().Get("limit")))
	case strings.HasPrefix(path, "/internal/diagnostics/media-prompts/"):
		parts := splitInternalPath(path)
		return b.server.app.RetryMediaIntent(ctx, actorID, pathPart(parts, 3))
	case path == "/internal/diagnostics/export":
		return b.server.app.DiagnosticsExportFiltered(ctx, actorID, lifecycleFilter(parsed.Query()))
	case path == "/internal/diagnostics/workflows" && method == http.MethodGet:
		return b.server.app.WorkflowList(ctx, actorID, parsed.Query().Get("query"))
	case strings.HasPrefix(path, "/internal/diagnostics/workflows/") && strings.HasSuffix(path, "/status"):
		parts := splitInternalPath(path)
		return b.server.app.WorkflowStatus(ctx, actorID, pathPart(parts, 3))
	case strings.HasPrefix(path, "/internal/diagnostics/workflows/") && strings.HasSuffix(path, "/history"):
		parts := splitInternalPath(path)
		return b.server.app.WorkflowHistory(ctx, actorID, pathPart(parts, 3))
	case strings.HasPrefix(path, "/internal/diagnostics/workflows/"):
		parts := splitInternalPath(path)
		return b.server.app.WorkflowCommand(ctx, actorID, pathPart(parts, 3), pathPart(parts, 4), values)
	default:
		return nil, browserBackendError(fmt.Errorf("unsupported browser operation %s %s", method, path), "browser_operation_not_supported")
	}
	if err != nil {
		return nil, browserBackendError(err, "core_operation_failed")
	}
	return map[string]any{}, nil
}

func (b *browserBackend) resolveActor(ctx context.Context, session string) (string, error) {
	if strings.TrimSpace(session) == "" {
		return "", &browser.CoreError{Status: http.StatusUnauthorized, Code: "unauthenticated", Message: "authentication required"}
	}
	actorID, err := b.server.repository.ResolveSession(ctx, session)
	if err != nil {
		return "", browserBackendError(err, "unauthenticated")
	}
	return actorID, nil
}

func (b *browserBackend) activate(ctx context.Context, actorID string, body map[string]any) (map[string]any, error) {
	mode := stringValue(body["initialization_mode"])
	requestID := stringValue(body["request_id"])
	analysisID := stringValue(body["analysis_id"])
	corePersona := mapValue(body["core_persona"])
	identity := mapValue(corePersona["identity"])
	name := stringValue(body["name"])
	if name == "" {
		name = stringValue(identity["name"])
	}
	initialization := map[string]any{"schema_version": body["schema_version"], "core_persona": corePersona, "developing_self": mapValue(body["developing_self"]), "initial_goals": arrayValue(body["initial_goals"]), "initial_intentions": arrayValue(body["initial_intentions"]), "initial_relationships": arrayValue(body["initial_relationships"]), "extensions": mapValue(body["extensions"])}
	if mode == "blank_slate" {
		initialization = nil
	}
	item, err := b.server.app.CreateFluctlight(ctx, actorID, core.StableFluctlightID(actorID, requestID), name, mode, analysisID, initialization, arrayValue(body["initial_goals"]), arrayValue(body["initial_intentions"]))
	if err != nil {
		return nil, browserBackendError(err, "activation_persistence_failed")
	}
	return map[string]any{"id": item.ID, "core_persona": item.CorePersona, "identity": item.Identity, "personality": item.Personality, "behavioral_policy": item.BehavioralPolicy, "life_profile": item.LifeProfile, "provenance": item.Provenance, "status": item.Status, "current_revision": item.CurrentRevision}, nil
}

func browserBackendError(err error, fallback string) error {
	if err == nil {
		return nil
	}
	slog.Default().Error("browserBackend error", "err", err, "error_type", fmt.Sprintf("%T", err), "fallback", fallback)
	status := http.StatusBadGateway
	code := fallback
	var details map[string]any

	var detailed interface{ PublicDetails() map[string]any }
	if errors.As(err, &detailed) {
		status = http.StatusUnprocessableEntity
		details = detailed.PublicDetails()
		code = "activation_persona_invalid"
	} else {
		switch {
		case errors.Is(err, core.ErrActivationAnalysisRequired):
			status, code = http.StatusUnprocessableEntity, "activation_analysis_required"
		case errors.Is(err, core.ErrActivationAnalysisInvalid):
			status, code = http.StatusUnprocessableEntity, "activation_analysis_invalid"
		case errors.Is(err, core.ErrActivationAnalysisStale):
			status, code = http.StatusConflict, "activation_analysis_stale"
		case errors.Is(err, core.ErrActivationAnalysisConflict):
			status, code = http.StatusConflict, "activation_analysis_conflict"
		case errors.Is(err, core.ErrUnauthorized):
			status, code = http.StatusUnauthorized, "unauthenticated"
		case errors.Is(err, core.ErrNotFound):
			status, code = http.StatusNotFound, "not_found"
		case errors.Is(err, core.ErrConflict):
			status, code = http.StatusConflict, "activation_request_conflict"
		case errors.Is(err, context.Canceled):
			status, code = http.StatusRequestTimeout, "request_cancelled"
		case errors.Is(err, context.DeadlineExceeded):
			status, code = http.StatusGatewayTimeout, "request_timeout"
		}
	}
	if details == nil {
		details = map[string]any{}
	}
	if _, ok := details["cause"]; !ok && err != nil {
		details["cause"] = err.Error()
	}
	return &browser.CoreError{Status: status, Code: code, Message: err.Error(), Details: details}
}

func objectBody(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	if result, ok := value.(map[string]any); ok {
		return result
	}
	return map[string]any{}
}

func jsonMap(value any) (map[string]any, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(encoded, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func jsonMapMust(value any, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	return jsonMap(value)
}

func splitInternalPath(path string) []string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for index, value := range parts {
		parts[index], _ = url.PathUnescape(value)
	}
	return parts
}

func pathPart(parts []string, index int) string {
	if index < 0 || index >= len(parts) {
		return ""
	}
	return parts[index]
}

func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	case string:
		parsed, _ := strconv.Atoi(typed)
		return parsed
	default:
		return 0
	}
}

func intPtr(value any) *int {
	if value == nil {
		return nil
	}
	result := intValue(value)
	return &result
}

func intPtrFromQuery(value string) *int {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return nil
	}
	return &parsed
}

func queryLimitValue(value string) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 100
	}
	if parsed > 500 {
		return 500
	}
	return parsed
}

func lifecycleFilter(query url.Values) core.LifecycleDiagnosticsFilter {
	return core.LifecycleDiagnosticsFilter{Limit: queryLimitValue(query.Get("limit")), FluctlightID: query.Get("fluctlight_id"), CorrelationID: query.Get("correlation_id"), IntentID: query.Get("intent_id"), WorkflowID: query.Get("workflow_id"), RunID: query.Get("run_id"), Surface: query.Get("surface"), Status: query.Get("status")}
}

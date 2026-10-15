package browser

import (
	"errors"
	"mime"
	"net/http"
	"strings"
	"unicode"
)

type surfaceRoute struct {
	method  string
	pattern string
}

var surfaceRoutes = []surfaceRoute{
	{http.MethodGet, "/api/fluctlights/:fluctlightId/media"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/sidebar"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/nearby"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/diary"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/diary"},
	{http.MethodPut, "/api/fluctlights/:fluctlightId/diary/:entryId"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/world"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/world/templates"},
	{http.MethodPut, "/api/fluctlights/:fluctlightId/world/templates/:templateId"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/world/places/:placeId"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/autonomy-policy"},
	{http.MethodPut, "/api/fluctlights/:fluctlightId/autonomy-policy"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/desires"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/owner-requests"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/promises"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/moment-suggestions"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/persona-switch"},
	{http.MethodPut, "/api/fluctlights/:fluctlightId/visual-identity"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/wardrobe/wear"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/wardrobe/items/:itemId/use"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/wardrobe/items/:itemId/photos"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/schedules/:scheduleId/items"},
	{http.MethodPut, "/api/fluctlights/:fluctlightId/schedules/:scheduleId/items/:itemId"},
	{http.MethodDelete, "/api/fluctlights/:fluctlightId/schedules/:scheduleId/items/:itemId"},
	{http.MethodGet, "/api/fluctlights/:fluctlightId/reflections/:reflectionId"},
	{http.MethodPost, "/api/fluctlights/:fluctlightId/reflections/:reflectionId/feedback"},
	{http.MethodGet, "/api/notifications"},
	{http.MethodGet, "/api/notifications/unread-count"},
	{http.MethodPost, "/api/notifications/:notificationId/read"},
	{http.MethodGet, "/api/pending"},
	{http.MethodGet, "/api/pending/count"},
	{http.MethodGet, "/api/studio/resources"},
	{http.MethodPost, "/api/studio/resources"},
	{http.MethodGet, "/api/studio/resources/:resourceId"},
	{http.MethodPut, "/api/studio/resources/:resourceId"},
	{http.MethodPost, "/api/studio/resources/:resourceId/apply-preview"},
	{http.MethodPost, "/api/studio/resources/:resourceId/apply"},
	{http.MethodGet, "/api/studio/previews"},
	{http.MethodPost, "/api/studio/previews"},
	{http.MethodPost, "/api/studio/packs"},
	{http.MethodPost, "/api/studio/packs/validate"},
	{http.MethodGet, "/api/studio/packs/export"},
	{http.MethodGet, "/api/ops/backups"},
	{http.MethodPost, "/api/ops/backups"},
	{http.MethodPost, "/api/ops/backups/:backupId/restore"},
	{http.MethodGet, "/api/diagnostics/harm-intercepts"},
	{http.MethodGet, "/api/diagnostics/tasks/:taskId"},
	{http.MethodGet, "/api/settings/agents"},
	{http.MethodPost, "/api/memories/:memoryId/suggestions"},
	{http.MethodGet, "/api/owner-requests/:requestId"},
	{http.MethodPost, "/api/moments"},
	{http.MethodPost, "/api/media"},
	{http.MethodDelete, "/api/actor-groups/:groupId"},
}

func (s *Server) routeSurface(response http.ResponseWriter, request *http.Request) bool {
	for _, route := range surfaceRoutes {
		if route.method == request.Method && surfacePathMatches(request.URL.Path, route.pattern) {
			s.dispatchSurface(response, request)
			return true
		}
	}
	return false
}

func (s *Server) dispatchSurface(response http.ResponseWriter, request *http.Request) {
	session, ok := s.requireSession(response, request)
	if !ok {
		return
	}
	if request.Method != http.MethodGet && request.Method != http.MethodHead && !mutationGuard(response, request, s.trustedOrigin) {
		return
	}
	var body map[string]any
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		parsed, valid := surfaceBody(response, request)
		if !valid {
			return
		}
		body = snakeMap(parsed)
	}
	endpoint := "/internal" + strings.TrimPrefix(request.URL.Path, "/api")
	if request.URL.RawQuery != "" {
		endpoint += "?" + request.URL.RawQuery
	}
	result, err := s.backend.DoJSON(request.Context(), request.Method, endpoint, session, body)
	if err != nil {
		s.writeSurfaceError(response, err)
		return
	}
	if result == nil {
		result = map[string]any{}
	}
	if surfaceNotProvided(request.URL.Path, result) {
		writeError(response, http.StatusNotFound, "not_provided", "Notifications are not connected")
		return
	}
	replay, _ := result["_replay"].(bool)
	status := surfaceStatus(result["_status"])
	delete(result, "_status")
	delete(result, "_replay")
	delete(result, "__error")
	if replay {
		response.Header().Set("Idempotent-Replayed", "true")
	}
	if status == http.StatusNoContent {
		response.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(response, status, result)
}

func (s *Server) writeSurfaceError(response http.ResponseWriter, err error) {
	var coreErr *CoreError
	if errors.As(err, &coreErr) && coreErr.Status >= 400 {
		if coreErr.Replay {
			response.Header().Set("Idempotent-Replayed", "true")
		}
		message := coreErr.Message
		if message == "" {
			message = "Owner surface request failed"
		}
		writeErrorWithDetails(response, coreErr.Status, coreErr.Code, message, coreErr.Details)
		return
	}
	if s.publicUnauthorized(response, err) {
		return
	}
	writeError(response, http.StatusBadGateway, "surface_failed", "Owner surface request failed")
}

func surfaceNotProvided(path string, result map[string]any) bool {
	if strings.HasSuffix(path, "/notifications/unread-count") {
		_, ok := result["count"]
		return !ok
	}
	if strings.HasPrefix(path, "/api/notifications/") && strings.HasSuffix(path, "/read") {
		_, ok := result["id"]
		return !ok
	}
	return false
}

func surfaceStatus(value any) int {
	switch typed := value.(type) {
	case int:
		if typed >= 200 {
			return typed
		}
	case float64:
		if typed >= 200 {
			return int(typed)
		}
	}
	return http.StatusOK
}

func surfaceBody(response http.ResponseWriter, request *http.Request) (map[string]any, bool) {
	mediaType, _, _ := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if mediaType == "multipart/form-data" {
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			writeError(response, http.StatusBadRequest, "invalid_request", "Request validation failed")
			return nil, false
		}
		body := map[string]any{}
		if request.MultipartForm != nil {
			if values := request.MultipartForm.Value["idempotencyKey"]; len(values) > 0 {
				body["idempotencyKey"] = values[0]
			}
		}
		return body, true
	}
	return readBody(response, request)
}

func surfacePathMatches(path, pattern string) bool {
	pathParts, patternParts := splitPath(path), splitPath(pattern)
	if len(pathParts) != len(patternParts) {
		return false
	}
	for index, part := range patternParts {
		if strings.HasPrefix(part, ":") {
			if pathParts[index] == "" {
				return false
			}
			continue
		}
		if pathParts[index] != part {
			return false
		}
	}
	return true
}

func snakeMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(value))
	for key, child := range value {
		out[camelToSnake(key)] = snakeValue(child)
	}
	return out
}

func snakeValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return snakeMap(typed)
	case []any:
		items := make([]any, len(typed))
		for index, child := range typed {
			items[index] = snakeValue(child)
		}
		return items
	default:
		return value
	}
}

func camelToSnake(key string) string {
	var builder strings.Builder
	for index, char := range key {
		if unicode.IsUpper(char) {
			if index > 0 {
				builder.WriteByte('_')
			}
			builder.WriteRune(unicode.ToLower(char))
			continue
		}
		builder.WriteRune(char)
	}
	return builder.String()
}

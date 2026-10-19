package browser

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func (s *Server) conversationRoutes(w http.ResponseWriter, r *http.Request) bool {
	path := r.URL.Path
	if fid, ok := match(path, "/api/fluctlights/:fluctlightId/conversations"); ok {
		switch r.Method {
		case http.MethodGet:
			query, ok := conversationPageQuery(w, r, false)
			if !ok {
				return true
			}
			s.callMap(w, r, "/internal/fluctlights/"+escape(fid)+"/conversations?"+query.Encode(), http.MethodGet, nil, conversationRouteError(422, "conversation_directory_failed", "Conversation directory is unavailable"), browserDirectory)
		case http.MethodPost:
			body, ok := s.mutationBody(w, r, func(v map[string]any) bool {
				title, exists := v["title"]
				return onlyKeys(v, "idempotencyKey", "title") && validateString(v["idempotencyKey"], 1, 256) && (!exists || validateString(title, 0, 200))
			})
			if !ok {
				return true
			}
			s.callMap(w, r, "/internal/fluctlights/"+escape(fid)+"/conversations", http.MethodPost, map[string]any{"idempotency_key": body["idempotencyKey"], "title": body["title"]}, conversationRouteError(422, "conversation_create_failed", "Conversation could not be created"), func(v map[string]any) any { return browserPage(v) })
		default:
			writeError(w, 405, "method_not_allowed", "Method is not allowed")
		}
		return true
	}
	if fid, cid, ok := match2(path, "/api/fluctlights/:fluctlightId/conversations/:conversationId/select"); ok && r.Method == http.MethodPost {
		if _, ok := s.mutationBody(w, r, func(v map[string]any) bool { return len(v) == 0 }); !ok {
			return true
		}
		s.callMap(w, r, "/internal/fluctlights/"+escape(fid)+"/conversations/"+escape(cid)+"/select", http.MethodPost, map[string]any{}, conversationRouteError(404, "conversation_not_found", "Conversation is unavailable"), func(v map[string]any) any { return browserPage(v) })
		return true
	}
	if fid, ok := match(path, "/api/fluctlights/:fluctlightId/messages/search"); ok && r.Method == http.MethodGet {
		query, valid := conversationPageQuery(w, r, true)
		if !valid {
			return true
		}
		s.callMap(w, r, "/internal/fluctlights/"+escape(fid)+"/messages/search?"+query.Encode(), http.MethodGet, nil, conversationRouteError(422, "conversation_search_failed", "Message search is unavailable"), func(v map[string]any) any {
			items := []any{}
			for _, raw := range array(v["items"]) {
				items = append(items, browserMessage(object(raw)))
			}
			return map[string]any{"items": items, "nextCursor": v["next_cursor"]}
		})
		return true
	}
	if cid, mid, ok := match2(path, "/api/conversations/:conversationId/messages/:messageId"); ok && r.Method == http.MethodGet {
		s.callMap(w, r, "/internal/conversations/"+escape(cid)+"/messages/"+escape(mid), http.MethodGet, nil, conversationRouteError(404, "message_not_found", "Message is unavailable"), func(v map[string]any) any { return browserMessage(v) })
		return true
	}
	return false
}
func conversationPageQuery(w http.ResponseWriter, r *http.Request, search bool) (url.Values, bool) {
	q := r.URL.Query()
	limit := 50
	if raw := q.Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 200 {
			writeError(w, 400, "request_invalid", "Invalid page limit")
			return nil, false
		}
		limit = value
	}
	cursor := q.Get("cursor")
	if len(cursor) > 2048 {
		writeError(w, 400, "request_invalid", "Invalid cursor")
		return nil, false
	}
	result := url.Values{"limit": {strconv.Itoa(limit)}, "cursor": {cursor}}
	if search {
		text := q.Get("q")
		if strings.TrimSpace(text) == "" || !validateString(text, 1, 2000) {
			writeError(w, 400, "request_invalid", "Invalid search query")
			return nil, false
		}
		result.Set("q", text)
	}
	return result, true
}
func browserDirectory(v map[string]any) any {
	items := []any{}
	for _, raw := range array(v["items"]) {
		item := object(raw)
		page := browserPage(map[string]any{"conversation": item["conversation"]})
		out := map[string]any{"conversation": page["conversation"], "unreadCount": item["unread_count"], "lastReadSequence": item["last_read_sequence"], "lastDeliveredSequence": item["last_delivered_sequence"]}
		if last, ok := item["last_message"].(map[string]any); ok {
			out["lastMessage"] = browserMessage(last)
		}
		items = append(items, out)
	}
	return map[string]any{"items": items, "currentConversationId": v["current_conversation_id"], "nextCursor": v["next_cursor"]}
}

func onlyKeys(v map[string]any, keys ...string) bool {
	for k := range v {
		found := false
		for _, allowed := range keys {
			if k == allowed {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func conversationRouteError(status int, code, message string) routeError {
	return func(w http.ResponseWriter, err error) {
		var ce *CoreError
		if errors.As(err, &ce) {
			switch ce.Status {
			case 400, 401, 403, 404, 409, 422:
				writeError(w, ce.Status, ce.Code, message)
				return
			}
		}
		writeError(w, status, code, message)
	}
}

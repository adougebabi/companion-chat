package httpapi

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

func (b *browserBackend) conversationCommand(ctx context.Context, method, path string, q url.Values, owner string, body map[string]any) (any, bool, error) {
	parts := splitInternalPath(path)
	if len(parts) == 4 && parts[0] == "internal" && parts[1] == "fluctlights" && parts[3] == "conversations" {
		fid := parts[2]
		if method == http.MethodGet {
			v, err := b.server.app.ConversationDirectory(ctx, owner, fid, queryLimitValue(q.Get("limit")), q.Get("cursor"))
			return v, true, err
		}
		if method == http.MethodPost {
			v, err := b.server.app.NewDirectConversation(ctx, owner, fid, stringValue(body["title"]), stringValue(body["idempotency_key"]))
			return v, true, err
		}
	}
	if len(parts) == 6 && parts[0] == "internal" && parts[1] == "fluctlights" && parts[3] == "conversations" && parts[5] == "select" && method == http.MethodPost {
		v, err := b.server.app.SelectDirectConversation(ctx, owner, parts[2], parts[4])
		return v, true, err
	}
	if len(parts) == 5 && strings.HasPrefix(path, "/internal/fluctlights/") && parts[3] == "messages" && parts[4] == "search" && method == http.MethodGet {
		v, err := b.server.app.SearchConversationMessages(ctx, owner, parts[2], q.Get("q"), queryLimitValue(q.Get("limit")), q.Get("cursor"))
		return v, true, err
	}
	if len(parts) == 5 && parts[0] == "internal" && parts[1] == "conversations" && parts[3] == "messages" && method == http.MethodGet {
		v, err := b.server.app.ConversationMessage(ctx, owner, parts[2], parts[4])
		return v, true, err
	}
	return nil, false, nil
}

func (s *Server) conversationCommands(w http.ResponseWriter, r *http.Request) {
	owner, ok := s.authorizeHuman(w, r)
	if !ok || s.app == nil {
		return
	}
	body := map[string]any{}
	if r.Method == http.MethodPost {
		var valid bool
		body, valid = s.body(w, r)
		if !valid {
			return
		}
	}
	b := browserBackend{server: s}
	value, _, err := b.conversationCommand(r.Context(), r.Method, r.URL.Path, r.URL.Query(), owner, body)
	if err != nil {
		s.opError(w, err, "conversation_operation_failed")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

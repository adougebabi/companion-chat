package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/httpapi/browser"
)

func (b *browserBackend) memoryCommand(ctx context.Context, method, path string, q url.Values, owner string, body map[string]any) (any, bool, error) {
	p := splitInternalPath(path)
	var value any
	var err error
	matched := false
	if method == http.MethodGet && len(p) == 4 && p[0] == "internal" && p[1] == "fluctlights" && p[3] == "memories" {
		matched = true
		limit, e := memoryQueryInteger(q, "limit")
		if e != nil {
			return nil, true, memoryBoundaryError(e)
		}
		if raw := q.Get("aboutOwner"); raw != "" && raw != "true" && raw != "false" {
			return nil, true, memoryBoundaryError(core.ErrMemoryQueryInvalid)
		}
		value, err = b.server.app.Memory.Browse(ctx, owner, p[2], core.MemoryBrowseQuery{Query: q.Get("q"), Type: q.Get("type"), Status: q.Get("status"), Cursor: q.Get("cursor"), Limit: limit, AboutOwner: q.Get("aboutOwner") == "true"})
	} else if len(p) >= 3 && len(p) <= 4 && p[0] == "internal" && p[1] == "memories" {
		id := p[2]
		switch {
		case method == http.MethodGet && len(p) == 3:
			matched = true
			value, err = b.server.app.Memory.ReadManaged(ctx, owner, id)
		case method == http.MethodGet && len(p) == 4 && (p[3] == "revisions" || p[3] == "sources"):
			matched = true
			limit, e := memoryQueryInteger(q, "limit")
			if e != nil {
				return nil, true, memoryBoundaryError(e)
			}
			if p[3] == "revisions" {
				value, err = b.server.app.Memory.Revisions(ctx, owner, id, q.Get("cursor"), limit)
			} else {
				var revision *int
				if q.Has("revision") {
					n, e := memoryQueryInteger(q, "revision")
					if e != nil {
						return nil, true, memoryBoundaryError(e)
					}
					revision = &n
				}
				value, err = b.server.app.Memory.Sources(ctx, owner, id, revision, q.Get("cursor"), limit)
			}
		case method == http.MethodPut && len(p) == 3:
			matched = true
			value, err = b.server.app.Memory.ReviseMemory(ctx, owner, id, stringValue(body["content"]), intPtr(body["expected_revision"]), arrayValue(body["evidence_refs"]))
		case method == http.MethodPost && len(p) == 4 && p[3] == "forget":
			matched = true
			value, err = b.server.app.Memory.ForgetMemory(ctx, owner, id, intPtr(body["expected_revision"]), arrayValue(body["evidence_refs"]))
		case method == http.MethodPost && len(p) == 4 && p[3] == "rollback":
			matched = true
			if intPtr(body["target_revision"]) == nil || intPtr(body["expected_revision"]) == nil {
				return nil, true, memoryBoundaryError(core.ErrMemoryQueryInvalid)
			}
			value, err = b.server.app.Memory.RollbackMemory(ctx, owner, id, intValueHTTP(body["target_revision"]), intValueHTTP(body["expected_revision"]), arrayValue(body["evidence_refs"]))
		}
	}
	return value, matched, memoryBoundaryError(err)
}

func memoryQueryInteger(q url.Values, key string) (int, error) {
	if !q.Has(key) {
		return 0, nil
	}
	n, e := strconv.Atoi(q.Get(key))
	if e != nil || n < 0 {
		return 0, core.ErrMemoryQueryInvalid
	}
	return n, nil
}

func memoryBoundaryError(err error) error {
	if err == nil {
		return nil
	}
	status, code := http.StatusBadGateway, "memory_operation_failed"
	switch {
	case errors.Is(err, core.ErrNotFound):
		status, code = 404, "memory_not_found"
	case errors.Is(err, core.ErrUnauthorized):
		status, code = 403, "memory_forbidden"
	case errors.Is(err, core.ErrMemoryQueryInvalid):
		status, code = 400, "memory_query_invalid"
	case errors.Is(err, core.ErrConflict) || strings.Contains(err.Error(), "conflict") || strings.Contains(err.Error(), "stale"):
		status, code = 409, "memory_revision_conflict"
	case strings.HasPrefix(err.Error(), "memory_"):
		status, code = 422, "memory_command_invalid"
	}
	return &browser.CoreError{Status: status, Code: code, Message: code}
}

func (s *Server) memoryManagement(w http.ResponseWriter, r *http.Request) {
	owner, ok := s.authorizeHuman(w, r)
	if !ok {
		return
	}
	body := map[string]any{}
	if r.Method != http.MethodGet {
		var valid bool
		body, valid = s.body(w, r)
		if !valid {
			return
		}
	}
	b := browserBackend{server: s}
	value, _, err := b.memoryCommand(r.Context(), r.Method, r.URL.Path, r.URL.Query(), owner, body)
	if err != nil {
		var ce *browser.CoreError
		if errors.As(err, &ce) {
			writeError(w, ce.Status, ce.Code)
		} else {
			s.opError(w, err, "memory_operation_failed")
		}
		return
	}
	writeJSON(w, http.StatusOK, value)
}

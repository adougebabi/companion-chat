package browser

import (
	"net/http"
	"net/url"
	"strconv"
)

func (s *Server) memoryRoutes(w http.ResponseWriter, r *http.Request) bool {
	if fid, ok := match(r.URL.Path, "/api/fluctlights/:fluctlightId/memories"); ok && r.Method == http.MethodGet {
		q, valid := memoryPageQuery(w, r, "q", "type", "status", "aboutOwner")
		if !valid {
			return true
		}
		if !memoryFilter(q.Get("aboutOwner"), "true", "false") || !memoryFilter(q.Get("type"), "episodic", "semantic", "relationship", "autobiographical") || !memoryFilter(q.Get("status"), "active", "superseded", "deprecated", "forgotten") {
			writeError(w, 400, "memory_query_invalid", "Invalid memory filter")
			return true
		}
		s.callMap(w, r, "/internal/fluctlights/"+escape(fid)+"/memories?"+q.Encode(), http.MethodGet, nil, conversationRouteError(502, "memory_query_failed", "Memory could not be read"), nil)
		return true
	}
	if id, ok := match(r.URL.Path, "/api/memories/:memoryId"); ok && r.Method == http.MethodGet {
		s.callMap(w, r, "/internal/memories/"+escape(id), http.MethodGet, nil, conversationRouteError(502, "memory_read_failed", "Memory could not be read"), nil)
		return true
	}
	for _, kind := range []string{"revisions", "sources"} {
		if id, ok := match(r.URL.Path, "/api/memories/:memoryId/"+kind); ok && r.Method == http.MethodGet {
			q, valid := memoryPageQuery(w, r, "revision")
			if !valid {
				return true
			}
			if raw := q.Get("revision"); raw != "" {
				n, e := strconv.Atoi(raw)
				if e != nil || n < 0 {
					writeError(w, 400, "memory_query_invalid", "Invalid revision")
					return true
				}
			}
			s.callMap(w, r, "/internal/memories/"+escape(id)+"/"+kind+"?"+q.Encode(), http.MethodGet, nil, conversationRouteError(502, "memory_read_failed", "Memory history could not be read"), nil)
			return true
		}
	}
	if id, ok := match(r.URL.Path, "/api/memories/:memoryId/rollback"); ok && r.Method == http.MethodPost {
		body, valid := s.mutationBody(w, r, func(v map[string]any) bool {
			return onlyKeys(v, "targetRevision", "expectedRevision", "evidenceRefs") && validateMemoryForget(v) && validateInteger(v["targetRevision"], 0)
		})
		if !valid {
			return true
		}
		s.callMap(w, r, "/internal/memories/"+escape(id)+"/rollback", http.MethodPost, map[string]any{"target_revision": body["targetRevision"], "expected_revision": body["expectedRevision"], "evidence_refs": body["evidenceRefs"]}, conversationRouteError(502, "memory_rollback_failed", "Memory rollback could not be applied"), nil)
		return true
	}
	return false
}

func memoryFilter(value string, choices ...string) bool {
	if value == "" {
		return true
	}
	for _, v := range choices {
		if v == value {
			return true
		}
	}
	return false
}

func memoryPageQuery(w http.ResponseWriter, r *http.Request, extra ...string) (url.Values, bool) {
	q := r.URL.Query()
	allowed := append([]string{"limit", "cursor"}, extra...)
	for key, values := range q {
		if !memoryFilterKey(key, allowed) || len(values) != 1 {
			writeError(w, 400, "memory_query_invalid", "Invalid query")
			return nil, false
		}
	}
	if len(q.Get("cursor")) > 2048 || !validateString(q.Get("q"), 0, 500) {
		writeError(w, 400, "memory_query_invalid", "Invalid query")
		return nil, false
	}
	if raw := q.Get("limit"); raw != "" {
		n, e := strconv.Atoi(raw)
		if e != nil || n < 1 || n > 100 {
			writeError(w, 400, "memory_query_invalid", "Invalid page limit")
			return nil, false
		}
	}
	return q, true
}

func memoryFilterKey(key string, choices []string) bool {
	for _, v := range choices {
		if key == v {
			return true
		}
	}
	return false
}

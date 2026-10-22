package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

var ErrMemoryQueryInvalid = errors.New("memory_query_invalid")

// MemoryBrowseQuery is an Owner management read, never a prompt retrieval plan.
// It deliberately includes inactive and invalid-source records when requested.
type MemoryBrowseQuery struct {
	Query, Type, Status, Cursor string
	Limit                       int
	AboutOwner                  bool
}

type MemoryBrowsePage struct {
	Items      []map[string]any `json:"items"`
	NextCursor string           `json:"next_cursor"`
}

type memoryBrowseCursor = ownerReadCursor

func memoryPageLimit(n int) (int, error) {
	if n == 0 {
		return 30, nil
	}
	if n < 1 || n > 100 {
		return 0, ErrMemoryQueryInvalid
	}
	return n, nil
}

func decodeMemoryBrowseCursor(raw, scope string) (memoryBrowseCursor, error) {
	c, err := decodeOwnerReadCursor(raw, scope)
	if err != nil {
		return c, ErrMemoryQueryInvalid
	}
	return c, nil
}

func encodeMemoryBrowseCursor(c memoryBrowseCursor) string {
	return encodeOwnerReadCursor(c)
}

// Only product/audit fields cross the boundary; command digests and embedding
// vectors are not a memory's content or source and remain inside Core.
const managedMemoryProjection = `jsonb_build_object(
 'id',m.id,'owner_fluctlight_id',m.owner_fluctlight_id,'type',m.type,'content',m.content,
 'actor_refs',m.actor_refs,'conversation_id',m.conversation_id,'event_refs',m.event_refs,
 'evidence_refs',m.evidence_refs,'personality_perspectives',m.personality_perspectives,
 'confidence',m.confidence,'importance',m.importance,'emotional_significance',m.emotional_significance,
 'visibility',m.visibility,'status',m.status,'revision',m.revision,'provenance_status',m.provenance_status,
 'occurred_at',m.occurred_at,'created_at',m.created_at,'last_confirmed_at',m.last_confirmed_at,
 'superseded_by_memory_id',m.superseded_by_memory_id,'supersedes_memory_id',m.supersedes_memory_id,
 'revision_count',(SELECT count(*) FROM public.memory_revisions r WHERE r.memory_id=m.id),
 'source_count',(SELECT count(*) FROM public.memory_source_links l WHERE l.memory_id=m.id AND l.memory_revision=m.revision))`

func (s *MemoryService) Browse(ctx context.Context, owner, fid string, q MemoryBrowseQuery) (MemoryBrowsePage, error) {
	out := MemoryBrowsePage{Items: []map[string]any{}}
	limit, err := memoryPageLimit(q.Limit)
	if err != nil {
		return out, err
	}
	q.Query = strings.TrimSpace(q.Query)
	if utf8.RuneCountInString(q.Query) > 500 || !memoryBrowseEnum(q.Type, "episodic", "semantic", "relationship", "autobiographical") || !memoryBrowseEnum(q.Status, "active", "superseded", "deprecated", "forgotten") {
		return out, ErrMemoryQueryInvalid
	}
	if _, err := s.app.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return out, err
	}
	scopeFields := []any{"owner_memory", owner, fid, q.Query, q.Type, q.Status}
	if q.AboutOwner {
		scopeFields = append(scopeFields, "about_owner_effective")
	}
	scope := stableDigest(jsonString(scopeFields))
	c, err := decodeMemoryBrowseCursor(q.Cursor, scope)
	if err != nil || (q.Cursor != "" && (c.ID == "" || c.Time.IsZero())) {
		return out, ErrMemoryQueryInvalid
	}
	rows, err := s.pool.Query(ctx, `SELECT m.id,m.created_at,`+managedMemoryProjection+`
 FROM public.memories m JOIN public.fluctlights f ON f.id=m.owner_fluctlight_id
 WHERE f.created_by_actor_id=$1 AND m.owner_fluctlight_id=$2
 AND ($3='' OR strpos(lower(m.content),lower($3))>0) AND ($4='' OR m.type=$4) AND ($5='' OR m.status=$5)
 AND (NOT $9::boolean OR (m.actor_refs ? $1 AND `+storyEffectiveMemorySQL+`))
 AND ($6='' OR (m.created_at,m.id)<($7,$6)) ORDER BY m.created_at DESC,m.id DESC LIMIT $8`, owner, fid, q.Query, q.Type, q.Status, c.ID, c.Time, limit+1, q.AboutOwner)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	var last memoryBrowseCursor
	for rows.Next() {
		var id string
		var at time.Time
		var raw []byte
		if err := rows.Scan(&id, &at, &raw); err != nil {
			return out, err
		}
		if len(out.Items) == limit {
			out.NextCursor = encodeMemoryBrowseCursor(last)
			break
		}
		out.Items = append(out.Items, decodeObject(raw))
		last = memoryBrowseCursor{Scope: scope, Time: at, ID: id}
	}
	return out, rows.Err()
}

func memoryBrowseEnum(value string, choices ...string) bool {
	if value == "" {
		return true
	}
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}

func (s *MemoryService) ReadManaged(ctx context.Context, owner, id string) (map[string]any, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, `SELECT `+managedMemoryProjection+` FROM public.memories m JOIN public.fluctlights f ON f.id=m.owner_fluctlight_id WHERE m.id=$1 AND f.created_by_actor_id=$2`, id, owner).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return decodeObject(raw), nil
}

func (s *MemoryService) Revisions(ctx context.Context, owner, id, cursor string, limit int) (MemoryBrowsePage, error) {
	out := MemoryBrowsePage{Items: []map[string]any{}}
	limit, err := memoryPageLimit(limit)
	if err != nil {
		return out, err
	}
	if _, err = s.ReadManaged(ctx, owner, id); err != nil {
		return out, err
	}
	scope := stableDigest(jsonString([]any{"memory_revisions", owner, id}))
	c, err := decodeMemoryBrowseCursor(cursor, scope)
	if err != nil || (cursor != "" && c.Revision < 0) {
		return out, ErrMemoryQueryInvalid
	}
	rows, err := s.pool.Query(ctx, `SELECT r.revision,jsonb_build_object('revision',r.revision,'base_revision',r.base_revision,'operation',r.operation,'content',r.content,'status',r.status,'actor_id',r.actor_id,'evidence_refs',r.evidence_refs,'source_window',r.source_window,'proposal_id',r.proposal_id,'semantic_reason',r.semantic_reason,'related_memory_ids',r.related_memory_ids,'reason_code',r.reason_code,'schema_version',r.schema_version,'created_at',r.created_at)
 FROM public.memory_revisions r JOIN public.memories m ON m.id=r.memory_id JOIN public.fluctlights f ON f.id=m.owner_fluctlight_id
 WHERE r.memory_id=$1 AND f.created_by_actor_id=$2 AND ($3 OR r.revision<$4) ORDER BY r.revision DESC LIMIT $5`, id, owner, cursor == "", c.Revision, limit+1)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	var last int
	for rows.Next() {
		var revision int
		var raw []byte
		if err := rows.Scan(&revision, &raw); err != nil {
			return out, err
		}
		if len(out.Items) == limit {
			out.NextCursor = encodeMemoryBrowseCursor(memoryBrowseCursor{Scope: scope, Revision: last})
			break
		}
		out.Items = append(out.Items, decodeObject(raw))
		last = revision
	}
	return out, rows.Err()
}

func (s *MemoryService) Sources(ctx context.Context, owner, id string, revision *int, cursor string, limit int) (MemoryBrowsePage, error) {
	out := MemoryBrowsePage{Items: []map[string]any{}}
	limit, err := memoryPageLimit(limit)
	if err != nil {
		return out, err
	}
	current, err := s.ReadManaged(ctx, owner, id)
	if err != nil {
		return out, err
	}
	rev := intValue(current["revision"])
	if revision != nil {
		rev = *revision
	}
	if rev < 0 || rev > intValue(current["revision"]) {
		return out, ErrMemoryQueryInvalid
	}
	scope := stableDigest(jsonString([]any{"memory_sources", owner, id, rev}))
	c, err := decodeMemoryBrowseCursor(cursor, scope)
	if err != nil || (cursor != "" && c.ID == "") {
		return out, ErrMemoryQueryInvalid
	}
	rows, err := s.pool.Query(ctx, `SELECT l.source_ref,jsonb_build_object('memory_revision',l.memory_revision,'source_ref',l.source_ref,'source_kind',l.source_kind,'source_id',l.source_id,'source_revision',l.source_revision,'occurred_at',l.occurred_at,'recorded_at',l.recorded_at,'status',l.status,'live',l.status='valid' AND public.memory_source_is_live(l.source_kind,l.source_id,l.source_revision,l.source_fingerprint),'superseded_by_ref',l.superseded_by_ref,
 'conversation_id',CASE WHEN l.source_kind='message' THEN (SELECT cm.conversation_id FROM public.conversation_messages cm WHERE cm.id=l.source_id) ELSE NULL END)
 FROM public.memory_source_links l JOIN public.memories m ON m.id=l.memory_id JOIN public.fluctlights f ON f.id=m.owner_fluctlight_id
 WHERE l.memory_id=$1 AND l.memory_revision=$2 AND f.created_by_actor_id=$3 AND ($4='' OR l.source_ref>$4) ORDER BY l.source_ref LIMIT $5`, id, rev, owner, c.ID, limit+1)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	last := ""
	for rows.Next() {
		var ref string
		var raw []byte
		if err := rows.Scan(&ref, &raw); err != nil {
			return out, err
		}
		if len(out.Items) == limit {
			out.NextCursor = encodeMemoryBrowseCursor(memoryBrowseCursor{Scope: scope, ID: last})
			break
		}
		out.Items = append(out.Items, decodeObject(raw))
		last = ref
	}
	if err := rows.Err(); err != nil {
		return out, fmt.Errorf("memory sources: %w", err)
	}
	return out, nil
}

package core

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

const (
	RawHistoryConversationMessage RawHistoryKind = "conversation_message"
	RawHistoryCognitionFact       RawHistoryKind = "cognition_fact"
	RawHistoryCapabilityOutcome   RawHistoryKind = "capability_outcome"

	defaultRawHistoryLimit = 50
	maxRawHistoryLimit     = 200
	maxRawHistorySources   = 64
	maxRawHistoryQueryRune = 2000
)

type RawHistoryKind string

// RawHistoryEvent is a read-only envelope over an owning domain record. It is
// not a copied event-store row: SourceRef and Authority point back to the
// table that remains authoritative for the observation.
type RawHistoryEvent struct {
	SourceRef      string
	Kind           RawHistoryKind
	FluctlightID   string
	ConversationID string
	ActorID        string
	OccurredAt     time.Time
	Sequence       int64
	Content        map[string]any
	Authority      string
	Relevance      float64
}

type RawHistoryQuery struct {
	AuthorizationActorID string
	FluctlightID         string
	ConversationID       string
	BeforeOccurredAt     *time.Time
	BeforeSourceRef      string
	Limit                int
}

type RawHistorySearchQuery struct {
	AuthorizationActorID string
	FluctlightID         string
	ConversationID       string
	Query                string
	Limit                int
}

type RawHistorySourceQuery struct {
	AuthorizationActorID string
	FluctlightID         string
	ConversationID       string
	SourceRefs           []string
}

type RawHistoryReader interface {
	Recent(context.Context, RawHistoryQuery) ([]RawHistoryEvent, error)
	Search(context.Context, RawHistorySearchQuery) ([]RawHistoryEvent, error)
	ReadSources(context.Context, RawHistorySourceQuery) ([]RawHistoryEvent, error)
}

type PostgresRawHistoryReader struct {
	repository *PostgresRepository
}

func NewRawHistoryReader(repository *PostgresRepository) *PostgresRawHistoryReader {
	return &PostgresRawHistoryReader{repository: repository}
}

func (r *PostgresRawHistoryReader) Recent(ctx context.Context, query RawHistoryQuery) ([]RawHistoryEvent, error) {
	query.Limit = normalizeRawHistoryLimit(query.Limit)
	if err := r.validateScope(ctx, query.AuthorizationActorID, query.FluctlightID, query.ConversationID); err != nil {
		return nil, err
	}
	var before any
	if query.BeforeOccurredAt != nil {
		before = query.BeforeOccurredAt.UTC()
	}
	rows, err := r.repository.Pool().Query(ctx, `
WITH raw_events AS (
  SELECT 'message:' || m.id AS source_ref,
         'conversation_message'::text AS kind,
         $1::text AS fluctlight_id,
         m.conversation_id,
         m.author_actor_id AS actor_id,
         m.created_at AS occurred_at,
         m.sequence::bigint AS sequence,
         jsonb_strip_nulls(jsonb_build_object(
           'kind',m.kind,'text',m.text,'attachment_refs',m.attachment_refs,
           'turn_id',m.turn_id,'source_fact_id',m.source_fact_id,'correlation_id',m.correlation_id
         )) AS content,
         'conversation_messages'::text AS authority
  FROM public.conversation_messages m
  WHERE $2::text <> '' AND m.conversation_id=$2

  UNION ALL

  SELECT 'fact:' || i.id,
         'cognition_fact',
         i.fluctlight_id,
         COALESCE(i.payload->>'conversation_id',''),
         COALESCE(i.payload->>'actor_id',''),
         i.occurred_at,
         i.sequence::bigint,
         i.payload,
         'cognition_inbox'
  FROM public.cognition_inbox i
  WHERE i.fluctlight_id=$1
    AND ($2::text='' OR i.payload->>'conversation_id'=$2)

  UNION ALL

  SELECT 'outcome:' || o.id,
         'capability_outcome',
         o.fluctlight_id,
         COALESCE(i.payload->>'conversation_id',''),
         '',
         o.occurred_at,
         i.sequence::bigint,
         jsonb_strip_nulls(jsonb_build_object(
           'action_id',o.action_id,'call_id',o.call_id,'capability_name',o.capability_name,
           'status',o.status,'success_boundary',o.success_boundary,
           'completion_boundary',o.completion_boundary,'observed',o.observed,
           'error_code',o.error_code,'evidence_refs',o.evidence_refs
         )),
         'cognition_action_outcomes'
  FROM public.cognition_action_outcomes o
  JOIN public.cognition_frozen_actions f ON f.id=o.action_id
  JOIN public.cognition_inbox i ON i.id=f.inbox_id
  WHERE o.fluctlight_id=$1
    AND ($2::text='' OR i.payload->>'conversation_id'=$2)
)
SELECT source_ref,kind,fluctlight_id,conversation_id,actor_id,occurred_at,sequence,content,authority
FROM raw_events
WHERE $3::timestamptz IS NULL
   OR occurred_at < $3
   OR (occurred_at=$3 AND source_ref < $4)
ORDER BY occurred_at DESC,source_ref DESC
LIMIT $5`, query.FluctlightID, query.ConversationID, before, query.BeforeSourceRef, query.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events, err := scanRawHistoryEvents(rows, false)
	if err != nil {
		return nil, err
	}
	return normalizeRawHistoryEvents(events, query.Limit), nil
}

func (r *PostgresRawHistoryReader) Search(ctx context.Context, query RawHistorySearchQuery) ([]RawHistoryEvent, error) {
	query.Query = strings.TrimSpace(query.Query)
	query.Limit = normalizeRawHistoryLimit(query.Limit)
	if query.ConversationID == "" || query.Query == "" || len([]rune(query.Query)) > maxRawHistoryQueryRune {
		return nil, errors.New("raw_history_search_invalid")
	}
	if err := r.validateScope(ctx, query.AuthorizationActorID, query.FluctlightID, query.ConversationID); err != nil {
		return nil, err
	}
	rows, err := r.repository.Pool().Query(ctx, `
SELECT 'message:' || m.id,
       'conversation_message',
       $1::text,
       m.conversation_id,
       m.author_actor_id,
       m.created_at,
       m.sequence::bigint,
       jsonb_strip_nulls(jsonb_build_object(
         'kind',m.kind,'text',m.text,'attachment_refs',m.attachment_refs,
         'turn_id',m.turn_id,'source_fact_id',m.source_fact_id,'correlation_id',m.correlation_id
       )),
       'conversation_messages',
       ts_rank_cd(m.search_document,plainto_tsquery('simple'::regconfig,$3))::double precision
FROM public.conversation_messages m
WHERE m.conversation_id=$2
  AND m.search_document @@ plainto_tsquery('simple'::regconfig,$3)
ORDER BY ts_rank_cd(m.search_document,plainto_tsquery('simple'::regconfig,$3)) DESC,m.created_at DESC,m.id DESC
LIMIT $4`, query.FluctlightID, query.ConversationID, query.Query, query.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events, err := scanRawHistoryEvents(rows, true)
	if err != nil {
		return nil, err
	}
	return deduplicateRawHistoryEvents(events, query.Limit), nil
}

func (r *PostgresRawHistoryReader) ReadSources(ctx context.Context, query RawHistorySourceQuery) ([]RawHistoryEvent, error) {
	refs, err := normalizeRawHistorySourceRefs(query.SourceRefs)
	if err != nil {
		return nil, err
	}
	if err := r.validateScope(ctx, query.AuthorizationActorID, query.FluctlightID, query.ConversationID); err != nil {
		return nil, err
	}
	rows, err := r.repository.Pool().Query(ctx, `
WITH raw_events AS (
  SELECT 'message:' || m.id AS source_ref,
         'conversation_message'::text AS kind,
         $1::text AS fluctlight_id,
         m.conversation_id,
         m.author_actor_id AS actor_id,
         m.created_at AS occurred_at,
         m.sequence::bigint AS sequence,
         jsonb_strip_nulls(jsonb_build_object(
           'kind',m.kind,'text',m.text,'attachment_refs',m.attachment_refs,
           'turn_id',m.turn_id,'source_fact_id',m.source_fact_id,'correlation_id',m.correlation_id
         )) AS content,
         'conversation_messages'::text AS authority
  FROM public.conversation_messages m
  WHERE ('message:' || m.id)=ANY($3) AND ($2::text='' OR m.conversation_id=$2)

  UNION ALL

  SELECT 'fact:' || i.id,'cognition_fact',i.fluctlight_id,
         COALESCE(i.payload->>'conversation_id',''),COALESCE(i.payload->>'actor_id',''),
         i.occurred_at,i.sequence::bigint,i.payload,'cognition_inbox'
  FROM public.cognition_inbox i
  WHERE i.fluctlight_id=$1 AND ('fact:' || i.id)=ANY($3)
    AND ($2::text='' OR i.payload->>'conversation_id'=$2)

  UNION ALL

  SELECT 'outcome:' || o.id,'capability_outcome',o.fluctlight_id,
         COALESCE(i.payload->>'conversation_id',''),'',o.occurred_at,i.sequence::bigint,
         jsonb_strip_nulls(jsonb_build_object(
           'action_id',o.action_id,'call_id',o.call_id,'capability_name',o.capability_name,
           'status',o.status,'success_boundary',o.success_boundary,
           'completion_boundary',o.completion_boundary,'observed',o.observed,
           'error_code',o.error_code,'evidence_refs',o.evidence_refs
         )),'cognition_action_outcomes'
  FROM public.cognition_action_outcomes o
  JOIN public.cognition_frozen_actions f ON f.id=o.action_id
  JOIN public.cognition_inbox i ON i.id=f.inbox_id
  WHERE o.fluctlight_id=$1 AND ('outcome:' || o.id)=ANY($3)
    AND ($2::text='' OR i.payload->>'conversation_id'=$2)
)
SELECT source_ref,kind,fluctlight_id,conversation_id,actor_id,occurred_at,sequence,content,authority
FROM raw_events
ORDER BY occurred_at,source_ref`, query.FluctlightID, query.ConversationID, refs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events, err := scanRawHistoryEvents(rows, false)
	if err != nil {
		return nil, err
	}
	return deduplicateRawHistoryEvents(events, maxRawHistorySources), nil
}

func (r *PostgresRawHistoryReader) validateScope(ctx context.Context, actorID, fluctlightID, conversationID string) error {
	if r == nil || r.repository == nil || strings.TrimSpace(actorID) == "" || strings.TrimSpace(fluctlightID) == "" {
		return errors.New("raw_history_scope_invalid")
	}
	var owner bool
	if err := r.repository.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlights WHERE id=$1 AND created_by_actor_id=$2)`, fluctlightID, actorID).Scan(&owner); err != nil {
		return err
	}
	if !owner {
		return ErrUnauthorized
	}
	if conversationID == "" {
		return nil
	}
	var participantCount int
	if err := r.repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_participants WHERE conversation_id=$1 AND actor_id IN ($2,$3) AND status='active'`, conversationID, actorID, fluctlightID).Scan(&participantCount); err != nil {
		return err
	}
	if participantCount != 2 {
		return ErrUnauthorized
	}
	return nil
}

type rawHistoryRows interface {
	Next() bool
	Scan(...any) error
	Err() error
}

func scanRawHistoryEvents(rows rawHistoryRows, withRelevance bool) ([]RawHistoryEvent, error) {
	events := make([]RawHistoryEvent, 0)
	for rows.Next() {
		var event RawHistoryEvent
		var kind string
		var rawContent []byte
		var err error
		if withRelevance {
			err = rows.Scan(&event.SourceRef, &kind, &event.FluctlightID, &event.ConversationID, &event.ActorID, &event.OccurredAt, &event.Sequence, &rawContent, &event.Authority, &event.Relevance)
		} else {
			err = rows.Scan(&event.SourceRef, &kind, &event.FluctlightID, &event.ConversationID, &event.ActorID, &event.OccurredAt, &event.Sequence, &rawContent, &event.Authority)
		}
		if err != nil {
			return nil, err
		}
		event.Kind = RawHistoryKind(kind)
		if err := json.Unmarshal(rawContent, &event.Content); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func normalizeRawHistoryLimit(limit int) int {
	if limit <= 0 {
		return defaultRawHistoryLimit
	}
	if limit > maxRawHistoryLimit {
		return maxRawHistoryLimit
	}
	return limit
}

func normalizeRawHistorySourceRefs(values []string) ([]string, error) {
	if len(values) == 0 || len(values) > maxRawHistorySources {
		return nil, errors.New("raw_history_source_refs_invalid")
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || (!strings.HasPrefix(value, "message:") && !strings.HasPrefix(value, "fact:") && !strings.HasPrefix(value, "outcome:")) {
			return nil, errors.New("raw_history_source_ref_invalid")
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	if len(result) == 0 {
		return nil, errors.New("raw_history_source_refs_invalid")
	}
	return result, nil
}

func normalizeRawHistoryEvents(events []RawHistoryEvent, limit int) []RawHistoryEvent {
	result := deduplicateRawHistoryEvents(events, limit)
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].OccurredAt.Equal(result[j].OccurredAt) {
			return result[i].SourceRef < result[j].SourceRef
		}
		return result[i].OccurredAt.Before(result[j].OccurredAt)
	})
	return result
}

func deduplicateRawHistoryEvents(events []RawHistoryEvent, limit int) []RawHistoryEvent {
	if limit <= 0 || limit > len(events) {
		limit = len(events)
	}
	result := make([]RawHistoryEvent, 0, limit)
	seen := make(map[string]struct{}, limit)
	for _, event := range events {
		if event.SourceRef == "" {
			continue
		}
		if _, exists := seen[event.SourceRef]; exists {
			continue
		}
		seen[event.SourceRef] = struct{}{}
		result = append(result, event)
		if len(result) == limit {
			break
		}
	}
	return result
}

var _ RawHistoryReader = (*PostgresRawHistoryReader)(nil)

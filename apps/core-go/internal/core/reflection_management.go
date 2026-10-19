package core

import (
	"context"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

func reflectionReadPage(query url.Values) (int, error) {
	if !query.Has("limit") {
		return 30, nil
	}
	limit, err := strconv.Atoi(query.Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		return 0, invalid("limit must be between 1 and 100")
	}
	return limit, nil
}

// Read the actual V2 proposal and outcome. Compatibility text is only a view
// over its summary; candidates/dispositions/revisions retain their own shape.
const reflectionSummaryProjection = `jsonb_build_object('id',p.id,'fluctlight_id',p.fluctlight_id,'from_sequence',p.from_sequence,'to_sequence',p.to_sequence,'base_state_revision',p.base_state_revision,
 'summary',p.payload->'summary','title','反思窗口 '||p.from_sequence||'—'||p.to_sequence,'body',p.payload->'summary','state',p.status,'status',p.status,'growth',NULL,'at',p.created_at,'created_at',p.created_at,
 'source_refs',p.evidence_refs,'evidence_refs',p.evidence_refs,'schema_version',p.schema_version,'model_version',p.model_version,'prompt_version',p.prompt_version,'policy_version',p.policy_version,
 'result',p.result,'expected_revisions',p.expected_revisions,'correlation_id',p.correlation_id)`

func (a *App) OwnerReflectionCatalog(ctx context.Context, owner, fid string, query url.Values) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	limit, err := reflectionReadPage(query)
	if err != nil {
		return nil, err
	}
	scope := stableDigest(jsonString([]any{"owner_reflections", owner, fid}))
	cursor, err := decodeOwnerReadCursor(query.Get("cursor"), scope)
	if err != nil || (query.Get("cursor") != "" && (cursor.ID == "" || cursor.Time.IsZero())) {
		return nil, invalid("reflection cursor is invalid")
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT p.id,p.created_at,`+reflectionSummaryProjection+` FROM public.cognition_reflection_proposals p JOIN public.fluctlights f ON f.id=p.fluctlight_id
 WHERE p.fluctlight_id=$1 AND f.created_by_actor_id=$2 AND ($3='' OR (p.created_at,p.id)<($4,$3)) ORDER BY p.created_at DESC,p.id DESC LIMIT $5`, fid, owner, cursor.ID, cursor.Time, limit+1)
	if err != nil {
		return nil, err
	}
	items := []any{}
	next := ""
	last := ownerReadCursor{Scope: scope}
	for rows.Next() {
		var id string
		var at time.Time
		var raw []byte
		if err := rows.Scan(&id, &at, &raw); err != nil {
			rows.Close()
			return nil, err
		}
		if len(items) == limit {
			next = encodeOwnerReadCursor(last)
			break
		}
		items = append(items, decodeObject(raw))
		last = ownerReadCursor{Scope: scope, ID: id, Time: at}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	var window, latest []byte
	err = a.DB.Pool().QueryRow(ctx, `SELECT jsonb_build_object('watermark',w.watermark,'state_revision',w.state_revision,'status',w.status,'updated_at',w.updated_at,'evolution_revisions',w.evolution_revisions) FROM public.cognition_reflection_windows w JOIN public.fluctlights f ON f.id=w.fluctlight_id WHERE w.fluctlight_id=$1 AND f.created_by_actor_id=$2`, fid, owner).Scan(&window)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	err = a.DB.Pool().QueryRow(ctx, `SELECT jsonb_build_object('intent_id',i.intent_id,'workflow_id',i.workflow_id,'status',i.status,'attempt_count',i.attempt_count,'next_attempt_at',i.next_attempt_at,'started_at',i.started_at,'completed_at',i.completed_at,'created_at',i.created_at,'error_code',CASE WHEN i.last_error IS NOT NULL THEN 'reflection_task_failed' ELSE NULL END)
 FROM public.platform_workflow_intents i JOIN public.fluctlights f ON f.id=i.payload->>'fluctlight_id' WHERE i.intent_type='reflection.run' AND f.id=$1 AND f.created_by_actor_id=$2 ORDER BY i.created_at DESC,i.intent_id DESC LIMIT 1`, fid, owner).Scan(&latest)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	out := map[string]any{"items": items, "next_cursor": next, "window": nil, "latest_task": nil}
	if len(window) > 0 {
		out["window"] = decodeObject(window)
	}
	if len(latest) > 0 {
		out["latest_task"] = decodeObject(latest)
	}
	return out, nil
}

func (a *App) OwnerReflection(ctx context.Context, owner, fid, id string) (map[string]any, error) {
	var raw, contextRaw, proposal []byte
	err := a.DB.Pool().QueryRow(ctx, `SELECT `+reflectionSummaryProjection+`,p.context_snapshot,p.payload FROM public.cognition_reflection_proposals p JOIN public.fluctlights f ON f.id=p.fluctlight_id WHERE p.id=$1 AND p.fluctlight_id=$2 AND f.created_by_actor_id=$3`, id, fid, owner).Scan(&raw, &contextRaw, &proposal)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &SurfaceError{Status: 404, Code: "reflection_not_found", Message: "Reflection is unavailable"}
	}
	if err != nil {
		return nil, err
	}
	out := decodeObject(raw)
	var evolution EvolutionContext
	if err := jsonUnmarshal(contextRaw, &evolution); err != nil {
		return nil, errors.New("reflection_context_unreadable")
	}
	out["source_window"], out["source_window_status"] = nil, "legacy_snapshot_unavailable"
	if evolution.SchemaVersion != "" {
		out["source_window"] = map[string]any{"source_window": evolution.SourceWindow, "from_sequence": evolution.FromSequence, "to_sequence": evolution.ToSequence, "watermark": evolution.Watermark, "evidence": evolution.Evidence, "outcomes": evolution.Outcomes, "base_revisions": evolution.BaseRevisions}
		out["source_window_status"] = "available"
	}
	keys := make([]string, 0, len(evolution.ReferenceIndex.ByRef))
	for key := range evolution.ReferenceIndex.ByRef {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	refs := []any{}
	for _, key := range keys {
		r := evolution.ReferenceIndex.ByRef[key]
		refs = append(refs, map[string]any{"ref": r.Ref, "kind": r.Kind, "entity_id": r.EntityID, "revision": r.Revision})
	}
	out["references"] = refs
	out["proposal"] = decodeObject(proposal)
	rows, err := a.DB.Pool().Query(ctx, `SELECT jsonb_build_object('domain',d.domain,'candidate_id',d.candidate_id,'candidate_index',d.candidate_index,'disposition',d.disposition,'reason_code',d.reason_code,'target_ref',d.target_ref,'changed_ref',d.changed_ref,'base_revision',d.base_revision,'resulting_revision',d.resulting_revision,'evidence_refs',d.evidence_refs) FROM public.cognition_reflection_candidate_dispositions d JOIN public.fluctlights f ON f.id=d.fluctlight_id WHERE d.proposal_id=$1 AND d.fluctlight_id=$2 AND f.created_by_actor_id=$3 ORDER BY d.domain,d.candidate_index,d.id`, id, fid, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []any{}
	for rows.Next() {
		var record []byte
		if err := rows.Scan(&record); err != nil {
			return nil, err
		}
		items = append(items, decodeObject(record))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out["dispositions"] = items
	return out, nil
}

func (a *App) OwnerReflectionFeedback(ctx context.Context, owner, fid, id string, query url.Values) (map[string]any, error) {
	if _, err := a.OwnerReflection(ctx, owner, fid, id); err != nil {
		return nil, err
	}
	limit, err := reflectionReadPage(query)
	if err != nil {
		return nil, err
	}
	scope := stableDigest(jsonString([]any{"owner_reflection_feedback", owner, fid, id}))
	cursor, err := decodeOwnerReadCursor(query.Get("cursor"), scope)
	if err != nil || (query.Get("cursor") != "" && (cursor.ID == "" || cursor.Time.IsZero())) {
		return nil, invalid("feedback cursor is invalid")
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,created_at,payload FROM public.owner_surface_records WHERE owner_actor_id=$1 AND fluctlight_id=$2 AND kind='owner_request' AND payload->>'kind'='reflection_feedback' AND payload->'payload'->>'reflection_id'=$3 AND ($4='' OR (created_at,id)<($5,$4)) ORDER BY created_at DESC,id DESC LIMIT $6`, owner, fid, id, cursor.ID, cursor.Time, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []any{}
	next := ""
	last := ownerReadCursor{Scope: scope}
	for rows.Next() {
		var request string
		var at time.Time
		var raw []byte
		if err := rows.Scan(&request, &at, &raw); err != nil {
			return nil, err
		}
		if len(items) == limit {
			next = encodeOwnerReadCursor(last)
			break
		}
		items = append(items, decodeObject(raw))
		last = ownerReadCursor{Scope: scope, ID: request, Time: at}
	}
	return map[string]any{"items": items, "next_cursor": next}, rows.Err()
}

package core

import (
	"context"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (a *App) WorldNPCOpportunity(ctx context.Context, owner, fid, npc, id string) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	p, err := readNPCOpportunityWith(ctx, a.DB.Pool(), owner, fid, id, false)
	if err != nil {
		return nil, err
	}
	world, err := worldIDWith(ctx, a.DB.Pool(), fid)
	if err != nil {
		return nil, err
	}
	if p["npc_id"] != npc || p["world_id"] != world {
		return nil, ErrNotFound
	}
	tasks, err := readWorldObjects(ctx, a.DB.Pool(), `SELECT jsonb_build_object('id',intent_id) FROM public.platform_workflow_intents WHERE payload->>'fluctlight_id'=$1 AND payload->>'inbox_id'=$2 ORDER BY created_at DESC LIMIT 1`, fid, p["source_fact_id"])
	if err != nil {
		return nil, err
	}
	p["record_kind"] = "opportunities"
	if len(tasks) > 0 {
		p["task_id"] = mapValue(tasks[0])["id"]
	}
	return p, nil
}

func (a *App) WorldNPCRecords(ctx context.Context, owner, fid, npc, kind string, q url.Values) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	world, err := worldIDWith(ctx, a.DB.Pool(), fid)
	if err != nil {
		return nil, err
	}
	var exists bool
	if err := a.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.world_npcs WHERE id=$1 AND world_id=$2)`, npc, world).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}
	table, column := "", "created_at"
	projection := "to_jsonb(t)"
	memberFilter := "$6::text<>''"
	switch kind {
	case "encounters":
		table, column = "world_encounters", "occurred_at"
		memberFilter += " AND EXISTS(SELECT 1 FROM public.life_events le WHERE le.id=t.life_event_id AND le.fluctlight_id=$6)"
	case "presences":
		table = "world_npc_presence_runs"
	case "opportunities":
		table = "world_npc_opportunities"
		memberFilter += " AND t.fluctlight_id=$6"
		projection = `to_jsonb(t)||jsonb_build_object('task_id',(SELECT intent_id FROM public.platform_workflow_intents i WHERE i.payload->>'inbox_id'=t.source_fact_id AND i.payload->>'fluctlight_id'=t.fluctlight_id ORDER BY created_at DESC LIMIT 1))`
	default:
		return nil, ErrInvalidArguments
	}
	limit, err := reflectionReadPage(q)
	if err != nil {
		return nil, err
	}
	scope := stableDigest(jsonString([]string{"npc-records", owner, fid, world, npc, kind}))
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil || (q.Get("cursor") != "" && (cursor.ID == "" || cursor.Time.IsZero())) {
		return nil, invalid("invalid NPC record cursor")
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT t.id,t.`+column+`,`+projection+`||jsonb_build_object('record_kind','`+kind+`') FROM public.`+table+` t WHERE t.world_id=$1 AND t.npc_id=$2 AND `+memberFilter+` AND ($3='' OR (t.`+column+`,t.id)<($4,$3)) ORDER BY t.`+column+` DESC,t.id DESC LIMIT $5`, world, npc, cursor.ID, cursor.Time, limit+1, fid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return readGovernanceRows(rows, scope, limit)
}
func (a *App) retryNPCOpportunityTx(ctx context.Context, tx pgx.Tx, owner, fid, npc, id string, body map[string]any) (map[string]any, error) {
	if err := requireOwnerMutableTx(ctx, tx, owner, fid); err != nil {
		return nil, err
	}
	p, err := readNPCOpportunityWith(ctx, tx, owner, fid, id, true)
	if err != nil {
		return nil, err
	}
	expected, ok := parseRevision(body["expected_revision"])
	reason := strings.TrimSpace(stringValue(body["reason"]))
	if !ok || intValue(p["revision"]) != expected {
		return nil, conflict("npc_opportunity", expected, intValue(p["revision"]))
	}
	if p["npc_id"] != npc {
		return nil, ErrNotFound
	}
	if reason == "" || len([]rune(reason)) > 500 {
		return nil, invalid("retry reason required")
	}
	if p["status"] != "failed" && p["status"] != "deferred" {
		return nil, newCapabilityError("npc_opportunity_not_retryable", false, ErrConflict)
	}
	if _, err := a.validateNPCOpportunityTx(ctx, tx, p, a.now().UTC()); err != nil {
		return nil, err
	}
	if err := a.queueNPCOpportunityFactTx(ctx, tx, id, fid, intValue(p["attempt_count"])+1, a.now().UTC()); err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "revision": expected + 1, "status": "pending", "source": "owner_requested_reassessment", "acquaintance_created": false}, nil
}

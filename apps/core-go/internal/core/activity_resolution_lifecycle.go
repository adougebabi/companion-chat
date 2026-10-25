package core

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

func registerActivityResolutionTx(ctx context.Context, tx pgx.Tx, id, fid string, due time.Time) error {
	_, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status) VALUES($1,$2,'lifecycle','life.activity.resolve',$3,'pending') ON CONFLICT(intent_id) DO NOTHING`, "activity_resolution:"+id, "activity_resolution:"+id, jsonBytes(map[string]any{"fluctlight_id": fid, "activity_id": id, "due_at": due.UTC().Format(instantLayout)}))
	return err
}

// Repair old independently started activities too. A missing daily schedule or
// later unrelated chat never owns their result timer. Domain status still
// decides completion/cancellation; no acquisition is inferred by this sweep.
func (a *App) EnsureLifeActivityResolutionIntents(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT r.id,r.fluctlight_id,r.not_before FROM public.fluctlight_life_activity_runs r JOIN public.fluctlights f ON f.id=r.fluctlight_id AND f.status='active' LEFT JOIN public.platform_workflow_intents i ON i.intent_id='activity_resolution:'||r.id WHERE r.intention_id IS NULL AND (r.status IN ('in_progress','deferred') OR r.status='completed' AND r.result_json #>> '{request,wear_after_acquisition}'='true' AND NOT(r.result_json ? 'wear_followup')) AND (i.intent_id IS NULL OR i.status='failed' OR i.status='completed' AND r.status='completed') ORDER BY r.not_before,r.id LIMIT $1`, limit)
	if err != nil {
		return 0, err
	}
	type row struct {
		id, fid string
		due     time.Time
	}
	var candidates []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.fid, &r.due); err != nil {
			rows.Close()
			return 0, err
		}
		candidates = append(candidates, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	count := 0
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		for _, r := range candidates {
			if err := registerActivityResolutionTx(ctx, tx, r.id, r.fid, r.due); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE public.platform_workflow_intents SET status='retry',next_attempt_at=now()+interval '5 minutes',started_at=NULL,completed_at=NULL WHERE intent_id=$1 AND (status='failed' OR status='completed')`, "activity_resolution:"+r.id); err != nil {
				return err
			}
			count++
		}
		return nil
	})
	return count, err
}

// Wearing is a separate real Tool effect, authorized only by the model's
// frozen start decision. Its stable operation also recovers a crash between
// acquisition and wearing without granting/wearing the same thing twice.
func (a *App) completeActivityWearFollowup(ctx context.Context, id, fid, owner, profile string) (map[string]any, error) {
	var raw []byte
	var event string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT result_json,COALESCE(source_event_id,'') FROM public.fluctlight_life_activity_runs WHERE id=$1 AND fluctlight_id=$2 AND status='completed'`, id, fid).Scan(&raw, &event); err != nil {
		return nil, err
	}
	data := decodeObject(raw)
	if mapValue(data["request"])["wear_after_acquisition"] != true {
		return nil, nil
	}
	if prior := mapValue(data["wear_followup"]); len(prior) > 0 {
		return prior, nil
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND source_kind='purchase_result' AND source_ref=$2 AND item_kind='wearable' ORDER BY source_item_key,id`, fid, event)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var item string
		if err := rows.Scan(&item); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	out := map[string]any{"status": "not_acquired", "effect_applied": false}
	if len(ids) > 0 {
		receipt, err := a.ExecuteTool(ctx, ToolExecutionRequest{CapabilityName: wardrobeWearCapabilityName, OperationID: "activity-wear:" + id, AuthorizationActorID: owner, FluctlightID: fid, WorkingProfileID: profile, EvidenceID: "activity:" + id, Surface: CapabilitySurfaceNativeCognition, Arguments: jsonBytes(map[string]any{"mode": "partial", "item_ids": ids})})
		if err != nil {
			return nil, err
		}
		out = map[string]any{"status": receipt.Result.Status, "result": receipt.Result.Output, "item_ids": ids}
	}
	_, err = a.DB.Pool().Exec(ctx, `UPDATE public.fluctlight_life_activity_runs SET result_json=jsonb_set(result_json,'{wear_followup}',$3::jsonb,true),revision=revision+1 WHERE id=$1 AND fluctlight_id=$2 AND status='completed' AND NOT(result_json ? 'wear_followup')`, id, fid, jsonBytes(out))
	return out, err
}

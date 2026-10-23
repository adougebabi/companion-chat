package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

const personaPreparationIntent = "persona.compile"

// Maintenance callers can still require a fully compiled aggregate. Browser
// admission instead commits the source and its durable compilation together.
func (a *App) CreateFluctlight(ctx context.Context, actor, id, name, mode, source string, foundation map[string]any, goals, intentions []any, zone ...string) (Fluctlight, error) {
	return a.createFluctlight(ctx, actor, id, name, mode, source, foundation, goals, intentions, false, zone...)
}
func (a *App) AdmitFluctlight(ctx context.Context, actor, id, name, mode, source string, foundation map[string]any, goals, intentions []any, zone ...string) (Fluctlight, error) {
	return a.createFluctlight(ctx, actor, id, name, mode, source, foundation, goals, intentions, true, zone...)
}
func (a *App) SetFoundationDecisionExpected(ctx context.Context, actor, fid, proposal, action, reason string, expected *int) (map[string]any, error) {
	return a.setFoundationDecisionExpected(ctx, actor, fid, proposal, action, reason, expected, false)
}
func (a *App) AdmitFoundationDecisionExpected(ctx context.Context, actor, fid, proposal, action, reason string, expected *int) (map[string]any, error) {
	result, err := a.setFoundationDecisionExpected(ctx, actor, fid, proposal, action, reason, expected, true)
	if err != nil {
		return nil, err
	}
	if action == "accept" {
		result["persona_preparation"], err = a.readPersonaPreparation(ctx, fid, intValue(result["revision"]))
	}
	return result, err
}

func enqueuePersonaPreparationTx(ctx context.Context, tx pgx.Tx, owner, fid string, revision int, source, key string) (string, error) {
	id := "persona_compile_" + stableDigest(fmt.Sprintf("%s:%s:%d:%s", owner, fid, revision, key))
	payload := map[string]any{"intent_id": id, "fluctlight_id": fid, "owner_actor_id": owner, "expected_revision": revision, "source": source, "source_fact_id": id}
	if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'lifecycle',$3,$4) ON CONFLICT DO NOTHING`, id, "persona-compile:"+id, personaPreparationIntent, jsonBytes(payload)); err != nil {
		return "", err
	}
	if err := appendOutboxTx(ctx, tx, "persona.compilation.requested", "fluctlight", fid, fid, id, "persona-compile:"+id, "persona-compilation:"+id, payload); err != nil {
		return "", err
	}
	return id, nil
}

func (a *App) readPersonaPreparation(ctx context.Context, fid string, revision int) (map[string]any, error) {
	var id, workflow, status, last string
	var created time.Time
	err := a.DB.Pool().QueryRow(ctx, `SELECT intent_id,workflow_id,status,COALESCE(last_error,''),created_at FROM public.platform_workflow_intents WHERE intent_type=$1 AND payload->>'fluctlight_id'=$2 AND payload->>'expected_revision'=$3 ORDER BY created_at DESC,intent_id DESC LIMIT 1`, personaPreparationIntent, fid, fmt.Sprint(revision)).Scan(&id, &workflow, &status, &last, &created)
	if errors.Is(err, pgx.ErrNoRows) {
		return map[string]any{"status": "ready", "revision": revision, "can_recover": false}, nil
	}
	if err != nil {
		return nil, err
	}
	state := "pending"
	switch status {
	case "completed":
		state = "ready"
	case "started":
		state = "running"
	case "failed", "dead_letter", "cancelled", "terminated":
		state = "failed"
	}
	return map[string]any{"id": id, "workflow_id": workflow, "status": state, "intent_status": status, "revision": revision, "error_code": last, "created_at": created.UTC().Format(time.RFC3339Nano), "can_recover": state == "failed"}, nil
}

func readPersonaHabitsFingerprint(ctx context.Context, q DBTX, fid string) (string, error) {
	var raw []byte
	err := q.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(h) ORDER BY profile_id),'[]'::jsonb) FROM public.fluctlight_profile_habits h WHERE fluctlight_id=$1`, fid).Scan(&raw)
	return stableDigest(string(raw)), err
}

// No model construction lives here. The existing compiler remains the sole
// semantic implementation; this worker owns only durable source/CAS fencing.
func (a *App) ProcessPersonaPreparationIntent(ctx context.Context, id string) (map[string]any, error) {
	var raw []byte
	var status string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT payload,status FROM public.platform_workflow_intents WHERE intent_id=$1 AND intent_type=$2`, id, personaPreparationIntent).Scan(&raw, &status); err != nil {
		return nil, err
	}
	source := decodeObject(raw)
	owner, fid, revision := stringValue(source["owner_actor_id"]), stringValue(source["fluctlight_id"]), intValue(source["expected_revision"])
	if status == "completed" {
		return map[string]any{"status": "completed", "fluctlight_id": fid, "revision": revision}, nil
	}
	if status != "pending" && status != "started" && status != "retry" {
		return map[string]any{"status": "cancelled"}, nil
	}
	resource, err := a.DB.GetFluctlight(ctx, fid, owner)
	if err != nil {
		return nil, err
	}
	if resource.CurrentRevision != revision || resource.Status == "retired" {
		return a.finishPersonaPreparation(ctx, id, "superseded")
	}
	before, err := readPersonaHabitsFingerprint(ctx, a.DB.Pool(), fid)
	if err != nil {
		return nil, err
	}
	var mode string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT initialization_mode FROM public.fluctlights WHERE id=$1`, fid).Scan(&mode); err != nil {
		return nil, err
	}
	compiled, err := a.compileFoundationWorkingPersonas(ctx, fid, resource.CorePersona, revision, mode, true)
	if err != nil {
		a.recordPersonaPreparationFailure(ctx, id)
		return nil, err
	}
	result := map[string]any{"status": "completed", "fluctlight_id": fid, "revision": revision}
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := lockLifeContextTx(ctx, tx, fid); err != nil {
			return err
		}
		var current int
		var lifecycle string
		var core []byte
		if err := tx.QueryRow(ctx, `SELECT current_revision,core_persona,status FROM public.fluctlights WHERE id=$1 AND created_by_actor_id=$2 FOR UPDATE`, fid, owner).Scan(&current, &core, &lifecycle); err != nil {
			return err
		}
		var latest string
		if err := tx.QueryRow(ctx, `SELECT intent_id FROM public.platform_workflow_intents WHERE intent_type=$1 AND payload->>'fluctlight_id'=$2 AND payload->>'expected_revision'=$3 ORDER BY created_at DESC,intent_id DESC LIMIT 1`, personaPreparationIntent, fid, fmt.Sprint(revision)).Scan(&latest); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT status FROM public.platform_workflow_intents WHERE intent_id=$1 FOR UPDATE`, id).Scan(&status); err != nil {
			return err
		}
		if status != "pending" && status != "started" && status != "retry" {
			return ErrConflict
		}
		if lifecycle == "retired" || lifecycle == "archived" || current != revision || latest != id || stableDigest(jsonString(decodeObject(core))) != stableDigest(jsonString(resource.CorePersona)) {
			result["disposition"] = "superseded"
			_, err := tx.Exec(ctx, `UPDATE public.platform_workflow_intents SET status='completed',completed_at=now(),last_error='persona_preparation_superseded' WHERE intent_id=$1`, id)
			return err
		}
		after, err := readPersonaHabitsFingerprint(ctx, tx, fid)
		if err != nil {
			return err
		}
		if before != after {
			return ErrConflict
		}
		if err := verifyCompiledOverlayVersionsTx(ctx, tx, fid, resource.CorePersona, compiled); err != nil {
			return err
		}
		if err := verifyCompiledBudgetTx(ctx, tx, compiled); err != nil {
			return err
		}
		if err := insertCompiledWorkingPersonasTx(ctx, tx, fid, compiled); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE public.platform_workflow_intents SET status='completed',completed_at=now(),last_error=NULL WHERE intent_id=$1`, id)
		return err
	})
	if err != nil {
		a.recordPersonaPreparationFailure(ctx, id)
	}
	return result, err
}
func (a *App) recordPersonaPreparationFailure(ctx context.Context, id string) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, _ = a.DB.Pool().Exec(cleanup, `UPDATE public.platform_workflow_intents SET last_error='persona_compilation_failed' WHERE intent_id=$1 AND status IN ('pending','started','retry')`, id)
}
func (a *App) finishPersonaPreparation(ctx context.Context, id, disposition string) (map[string]any, error) {
	_, err := a.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='completed',completed_at=now(),last_error=$2 WHERE intent_id=$1 AND status IN ('pending','started','retry')`, id, "persona_preparation_"+disposition)
	return map[string]any{"status": "completed", "disposition": disposition}, err
}

func (a *App) RecoverPersonaPreparation(ctx context.Context, owner, fid string, expected int, key string) (map[string]any, error) {
	if expected < 0 || len(key) == 0 || len(key) > 256 {
		return nil, ErrInvalidArguments
	}
	var revision int
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := lockLifeContextTx(ctx, tx, fid); err != nil {
			return err
		}
		if err := requireOwnerMutableTx(ctx, tx, owner, fid); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT current_revision FROM public.fluctlights WHERE id=$1 AND created_by_actor_id=$2 FOR UPDATE`, fid, owner).Scan(&revision); err != nil {
			return err
		}
		if revision != expected {
			return ErrConflict
		}
		var status string
		err := tx.QueryRow(ctx, `SELECT status FROM public.platform_workflow_intents WHERE intent_type=$1 AND payload->>'fluctlight_id'=$2 AND payload->>'expected_revision'=$3 ORDER BY created_at DESC,intent_id DESC LIMIT 1 FOR UPDATE`, personaPreparationIntent, fid, fmt.Sprint(revision)).Scan(&status)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil && (status == "pending" || status == "started" || status == "retry" || status == "completed") {
			return nil
		}
		_, err = enqueuePersonaPreparationTx(ctx, tx, owner, fid, revision, "owner_recovery", key)
		return err
	})
	if err != nil {
		return nil, err
	}
	return a.readPersonaPreparation(ctx, fid, revision)
}

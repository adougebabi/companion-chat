package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/decision"
	"github.com/jackc/pgx/v5"
)

var errStudioMediaNotConfigured = errors.New("studio_media_not_configured")

// A Studio producer has no task Actor. In particular first-person wording in a
// definition cannot borrow the authenticated Human's or a character's body.
func (a *App) queueStudioMedia(ctx context.Context, owner, id, aid string, job map[string]any) (map[string]any, error) {
	snapshot := mapValue(job["snapshot"])
	mediaID := "studio_media_" + stableDigest(id)
	var exists bool
	if err := a.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.media_intents WHERE id=$1 AND studio_job_id=$2 AND studio_owner_actor_id=$3)`, mediaID, id, owner).Scan(&exists); err != nil {
		return nil, err
	}
	var concept map[string]any
	if !exists {
		config, err := a.runtimeValue(ctx, "media.comfyui")
		if err != nil {
			return nil, imageProviderUnavailable(err)
		}
		if _, _, err := comfyConfig(config); err != nil {
			return nil, errStudioMediaNotConfigured
		}
		var staged []byte
		if err := a.DB.Pool().QueryRow(ctx, `SELECT output FROM public.studio_job_attempts WHERE id=$1 AND job_id=$2`, aid, id).Scan(&staged); err != nil {
			return nil, err
		}
		concept = mapValue(decodeObject(staged)["media_concept"])
		if len(concept) == 0 {
			bindings := map[string]any{"first_person": "studio_definition", "task_actor": nil, "source_kind": "studio_definition", "source_resource_id": snapshot["resource_id"]}
			planCtx := withMediaReferents(WithProviderCorrelation(ctx, "media:"+mediaID), bindings)
			definition := map[string]any{"kind": snapshot["resource_kind"], "name": snapshot["name"], "body": snapshot["body"]}
			if snapshot["resource_kind"] == "outfit" {
				definition["verified_members"] = snapshot["allowed_references"]
			}
			text := jsonString(map[string]any{"instruction": snapshot["instruction"], "definition": definition, "scope": "fictional Studio definition, no task actor or current life"})
			plan, sources, err := ResolveMediaCapturePlan(planCtx, a.kevService(), decision.Scope{Agent: "media_prompt", RunID: mediaID, CorrelationID: "media:" + mediaID}, text, map[string]any{"self_in_frame": "no"}, a.parseMediaPlan)
			if err != nil {
				return nil, err
			}
			concept = map[string]any{"intent": stringValue(snapshot["instruction"]), "media_plan_version": mediaPlanVersion, "media_capture_plan": plan, "explicit_constraints": map[string]any{"self_in_frame": "no"}, "field_sources": sources, "context_binding": map[string]any{}, "referent_bindings": bindings, "source": "studio_definition", "studio_owner_actor_id": owner, "studio_source": map[string]any{"resource_id": snapshot["resource_id"], "resource_revision": snapshot["resource_revision"], "source_digest": job["source_digest"], "resource_kind": snapshot["resource_kind"], "name": snapshot["name"], "body": snapshot["body"]}}
			if snapshot["resource_kind"] == "outfit" {
				mapValue(concept["studio_source"])["verified_members"] = snapshot["allowed_references"]
			}
			if ref := mapValue(snapshot["reference_asset"]); len(ref) > 0 {
				concept["reference_asset"] = cloneMap(ref)
			}
		}
		if concept["studio_owner_actor_id"] != owner || mapValue(concept["studio_source"])["source_digest"] != job["source_digest"] {
			return nil, errors.New("studio_media_staged_source_conflict")
		}
		if err := validateStudioMediaConcept(concept); err != nil {
			return nil, err
		}
		if err := validateBranchedMediaConcept(concept); err != nil {
			return nil, err
		}
		savedStage, err := a.DB.Pool().Exec(ctx, `UPDATE public.studio_job_attempts a SET output=jsonb_build_object('media_concept',$2::jsonb) FROM public.studio_jobs j WHERE a.id=$1 AND j.id=a.job_id AND a.status='running' AND j.status='running' AND j.latest_attempt=a.attempt`, aid, jsonBytes(concept))
		if err != nil {
			return nil, err
		}
		if savedStage.RowsAffected() != 1 {
			return nil, ErrConflict
		}
	}
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		current, err := readStudioJobWith(ctx, tx, owner, id, true)
		if err != nil {
			return err
		}
		if current["status"] != "running" || intValue(current["latest_attempt"]) != intValue(job["latest_attempt"]) {
			return ErrConflict
		}
		if snapshot["resource_kind"] == "outfit" {
			mapValue(concept["studio_source"])["verified_members"] = snapshot["allowed_references"]
		}
		if ref := mapValue(snapshot["reference_asset"]); len(ref) > 0 {
			if err := a.validateStudioReferenceWith(ctx, tx, owner, stringValue(snapshot["resource_id"]), ref); err != nil {
				return err
			}
		}
		workflowID := fmt.Sprintf("studio-media:%s:%d", id, intValue(job["latest_attempt"]))
		// One durable image intent per product job. Retry retains prompt, provider
		// job, quality correction and the original ready asset. Regenerate is a new job.
		if !exists {
			_, err = tx.Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,media_scope,studio_owner_actor_id,studio_job_id,kind,mime_type,prompt,provider_request_id,workflow_id,status,revision) VALUES($1,NULL,'studio',$2,$3,'image','image/png',$4,$5,$6,'pending',0) ON CONFLICT(id) DO NOTHING`, mediaID, owner, id, jsonString(concept), "media:"+mediaID, workflowID)
			if err != nil {
				return err
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE public.media_intents SET status=CASE WHEN status='failed' THEN 'pending' ELSE status END,workflow_id=$2 WHERE id=$1 AND media_scope='studio' AND studio_job_id=$3 AND studio_owner_actor_id=$4`, mediaID, workflowID, id, owner); err != nil {
			return err
		}
		wi := "media_workflow_intent:" + aid
		if _, err = tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'media','media.generation',$3) ON CONFLICT(intent_id) DO NOTHING`, wi, workflowID, jsonBytes(map[string]any{"intent_id": mediaID, "studio_attempt_id": aid, "owner_actor_id": owner})); err != nil {
			return err
		}
		output := map[string]any{"media_intent_id": mediaID, "workflow_id": workflowID, "label": "虚构创作预览·不进入生活"}
		command, err := tx.Exec(ctx, `UPDATE public.studio_job_attempts SET output=$2,provenance=provenance||$3::jsonb WHERE id=$1 AND status='running'`, aid, jsonBytes(output), jsonBytes(map[string]any{"media_intent_id": mediaID, "media_workflow_intent_id": wi, "source_digest": job["source_digest"]}))
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return ErrConflict
		}
		_, err = tx.Exec(ctx, `UPDATE public.studio_jobs SET result=$2,updated_at=now() WHERE id=$1`, id, jsonBytes(output))
		return err
	})
	return map[string]any{"id": id, "status": "running", "media_intent_id": mediaID, "life_write": false}, err
}

func validateStudioMediaConcept(concept map[string]any) error {
	if concept["source"] != "studio_definition" {
		return errors.New("studio_media_source_invalid")
	}
	plan := mapValue(concept["media_capture_plan"])
	if plan["self_in_frame"] != "no" || len(mapValue(concept["context_binding"])) != 0 || len(arrayValue(concept["subject_contexts"])) != 0 || concept["task_actor_id"] != nil {
		return errors.New("studio_media_actor_forbidden")
	}
	source := mapValue(concept["studio_source"])
	if stringValue(source["resource_id"]) == "" || stringValue(source["source_digest"]) == "" {
		return errors.New("studio_media_source_invalid")
	}
	return nil
}

// Only retained, generated, versioned nonhuman Studio images qualify as scene,
// animal or object references. Uploaded/legacy/person images have no such proof.
func validateStudioReferenceWith(ctx context.Context, q lifeContextQuerier, owner string, reference map[string]any) error {
	if !eventKeys(reference, "asset_id", "purpose") || stringValue(reference["asset_id"]) == "" {
		return invalid("Invalid Studio image reference")
	}
	var raw string
	err := q.QueryRow(ctx, `SELECT i.prompt FROM public.studio_resource_materials m JOIN public.media_assets a ON a.id=m.asset_id JOIN public.media_intents i ON a.id='asset_'||i.id JOIN public.studio_jobs j ON j.id=m.job_id WHERE m.owner_actor_id=$1 AND m.released_at IS NULL AND a.id=$2 AND a.status='ready' AND a.media_scope='studio' AND i.media_scope='studio' AND a.studio_owner_actor_id=m.owner_actor_id AND i.studio_owner_actor_id=m.owner_actor_id AND a.studio_job_id=j.id AND i.studio_job_id=j.id AND j.owner_actor_id=m.owner_actor_id AND m.source_digest=j.source_digest`, owner, reference["asset_id"]).Scan(&raw)
	if err != nil {
		return errors.New("studio_reference_source_unavailable")
	}
	source := decodeObject([]byte(raw))
	if err := validateStudioMediaConcept(source); err != nil {
		return err
	}
	if err := validateMediaNormalization(source, mapValue(source["normalized_output"])); err != nil {
		return err
	}
	return validateMediaReferenceSource(reference, source)
}

func (a *App) validateStudioReferenceWith(ctx context.Context, q lifeContextQuerier, owner, target string, reference map[string]any) error {
	if reference["grant_id"] != nil || reference["grant_revision"] != nil {
		return a.validateExplicitMediaReference(ctx, q, owner, target, reference)
	}
	return validateStudioReferenceWith(ctx, q, owner, reference)
}
func saveStudioImageMaterialTx(ctx context.Context, tx pgx.Tx, owner string, job map[string]any) (map[string]any, error) {
	asset := stringValue(mapValue(job["result"])["asset_id"])
	var verified string
	if err := tx.QueryRow(ctx, `SELECT a.id FROM public.media_assets a JOIN public.media_intents i ON a.id='asset_'||i.id WHERE a.id=$1 AND a.status='ready' AND a.media_scope='studio' AND i.media_scope='studio' AND a.studio_owner_actor_id=$2 AND i.studio_owner_actor_id=$2 AND a.studio_job_id=$3 AND i.studio_job_id=$3 AND i.status='completed' FOR SHARE OF a`, asset, owner, job["id"]).Scan(&verified); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return retainStudioMaterialTx(ctx, tx, owner, job, nil, asset, "虚构创作图片·不进入生活")

}

// Completion and terminal reconciliation lock the product job first, matching
// discard/retry/save lock order. A late old attempt cannot settle a new attempt.
func (a *App) settleStudioMedia(ctx context.Context, intent mediaIntent, success bool, code string) error {
	if intent.StudioJobID == "" {
		return nil
	}
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		job, err := readStudioJobWith(ctx, tx, intent.StudioOwner, intent.StudioJobID, true)
		if err != nil {
			return err
		}
		if job["status"] != "running" {
			return nil
		}
		var currentWorkflow string
		if err := tx.QueryRow(ctx, `SELECT workflow_id FROM public.media_intents WHERE id=$1 AND studio_job_id=$2 AND studio_owner_actor_id=$3 FOR UPDATE`, intent.ID, intent.StudioJobID, intent.StudioOwner).Scan(&currentWorkflow); err != nil {
			return err
		}
		if currentWorkflow != intent.WorkflowID {
			return nil
		}
		var aid string
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT id,output FROM public.studio_job_attempts WHERE job_id=$1 AND attempt=$2 AND status='running' FOR UPDATE`, intent.StudioJobID, job["latest_attempt"]).Scan(&aid, &raw); err != nil {
			return err
		}
		output := decodeObject(raw)
		if output["media_intent_id"] != intent.ID || output["workflow_id"] != intent.WorkflowID {
			return nil
		}
		status := "failed"
		var failure any = code
		var ready bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.media_assets WHERE id=$1 AND status='ready' AND media_scope='studio' AND studio_job_id=$2 AND studio_owner_actor_id=$3)`, "asset_"+intent.ID, intent.StudioJobID, intent.StudioOwner).Scan(&ready); err != nil {
			return err
		}
		if success && !ready {
			return ErrNotFound
		}
		if ready {
			output["asset_id"] = "asset_" + intent.ID
			output["url"] = "/api/media/asset_" + intent.ID
			output["recovered_ready_asset"] = !success
			status = "completed"
			failure = nil
		}
		mediaStatus := "failed"
		if ready {
			mediaStatus = "completed"
		}
		if _, err := tx.Exec(ctx, `UPDATE public.media_intents SET status=$2,revision=revision+1 WHERE id=$1 AND workflow_id=$3 AND status IN ('pending','running','failed','completed')`, intent.ID, mediaStatus, intent.WorkflowID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.studio_job_attempts SET status=$2,output=$3,error_code=$4,finished_at=now() WHERE id=$1 AND status='running'`, aid, status, jsonBytes(output), failure); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE public.studio_jobs SET status=$2,result=$3,error_code=$4,revision=revision+1,updated_at=now() WHERE id=$1 AND status='running'`, intent.StudioJobID, status, jsonBytes(output), failure)
		return err
	})
}
func (a *App) ReconcileStudioMediaTerminal(ctx context.Context, workflowIntentID, code string) error {
	var mid, sourceWorkflow string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT payload->>'intent_id',workflow_id FROM public.platform_workflow_intents WHERE intent_id=$1`, workflowIntentID).Scan(&mid, &sourceWorkflow); err != nil {
		return err
	}
	intent, err := a.readMediaIntent(ctx, mid)
	if err != nil {
		return err
	}
	if intent.StudioJobID == "" {
		return nil
	}
	if intent.WorkflowID != sourceWorkflow {
		return nil
	}
	return a.settleStudioMedia(ctx, intent, false, "studio_media_"+code)
}

func (a *App) studioMediaActive(ctx context.Context, intent mediaIntent) error {
	if intent.StudioJobID == "" {
		return nil
	}
	var valid bool
	if err := a.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.studio_jobs WHERE id=$1 AND owner_actor_id=$2 AND status='running')`, intent.StudioJobID, intent.StudioOwner).Scan(&valid); err != nil {
		return err
	}
	if !valid {
		return errors.New("studio_media_no_longer_active")
	}
	return nil
}

// Error settlement uses its own small context because Activity cancellation
// must still leave an observable product failure for explicit retry.
func (a *App) failStudioMedia(ctx context.Context, intent mediaIntent) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = a.settleStudioMedia(cleanup, intent, false, "studio_media_generation_failed")
}

func mediaIntentScope(intent mediaIntent) string {
	if intent.StudioJobID != "" {
		return "studio"
	}
	return "fluctlight"
}

func (a *App) readStudioMediaReference(ctx context.Context, owner, assetID string) (string, []byte, error) {
	if err := a.AuthorizeAsset(ctx, owner, assetID); err != nil {
		return "", nil, err
	}
	return a.readScopedMediaAssetContent(ctx, owner, assetID, true)
}

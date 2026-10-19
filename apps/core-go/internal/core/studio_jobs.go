package core

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Studio owns definition artifacts only. Neither admission nor execution loads
// a Fluctlight projection, private conversation, Memory, or life state.
func MatchStudioJobSurface(method, path string) bool {
	p := strings.Split(strings.Trim(path, "/"), "/")
	return len(p) >= 3 && p[0] == "internal" && p[1] == "studio" && (p[2] == "jobs" || p[2] == "materials")
}

func validateStudioDraft(kind, name string, body map[string]any, allowIncomplete bool) error {
	if !validResourceKind(kind) || strings.TrimSpace(name) == "" || len([]rune(name)) > 200 || body == nil || len(jsonBytes(body)) > 64000 {
		return invalid("A bounded resource kind, name and definition object are required")
	}
	if containsPermanentHarm(map[string]any{"name": name, "body": body}) {
		return harmError()
	}
	if kind == "visual_style" && !allowIncomplete {
		return validateVisualStyle(body)
	}
	if kind == "item" && !allowIncomplete {
		return validateItemDefinition(body)
	}
	if kind == "outfit" && len([]rune(name)) > 128 {
		return invalid("Outfit name must fit the 128-character saved outfit contract")
	}
	if kind == "outfit" && !allowIncomplete {
		return validateOutfitDefinition(body)
	}
	if kind == "persona_fragment" && !allowIncomplete {
		_, err := foundationDefinitionChanges(worldPublication{Kind: kind, Body: body})
		return err
	}
	if kind == "character_template" {
		if allowIncomplete {
			return nil
		}
		_, err := characterTemplateSource(body)
		return err
	}
	if kind == "event_template" && !allowIncomplete {
		return validateWorldEventDefinition(body)
	}
	if !allowIncomplete && len(body) == 0 {
		return invalid("Generated definition must not be empty")
	}
	for key, limit := range map[string]int{"background": 16000, "rules": 8000, "description": 8000, "role": 512} {
		if _, err := boundedWorldText(body, key, limit, false); err != nil {
			return err
		}
	}
	return nil
}
func readStudioJobWith(ctx context.Context, q lifeContextQuerier, owner, id string, lock bool) (map[string]any, error) {
	sql := `SELECT to_jsonb(j) FROM public.studio_jobs j WHERE owner_actor_id=$1 AND id=$2`
	if lock {
		sql += " FOR UPDATE"
	}
	var raw []byte
	err := q.QueryRow(ctx, sql, owner, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return decodeObject(raw), err
}
func enqueueStudioAttemptTx(ctx context.Context, tx pgx.Tx, owner, id string, attempt int, output map[string]any) error {
	aid := fmt.Sprintf("%s_attempt_%d", id, attempt)
	if _, err := tx.Exec(ctx, `INSERT INTO public.studio_job_attempts(id,job_id,attempt,status,output) VALUES($1,$2,$3,'queued',$4)`, aid, id, attempt, jsonBytes(output)); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'lifecycle','studio.generate',$3)`, "studio_intent:"+aid, "studio:"+aid, jsonBytes(map[string]any{"owner_actor_id": owner, "studio_attempt_id": aid, "studio_job_id": id}))
	return err
}
func (a *App) DispatchStudioJobSurface(ctx context.Context, method, path string, q url.Values, owner string, body map[string]any) (map[string]any, error) {
	p := strings.Split(strings.Trim(path, "/"), "/")
	if !MatchStudioJobSurface(method, path) {
		return nil, ErrNotFound
	}
	if p[2] == "materials" && len(p) == 5 && (p[4] == "release" || p[4] == "retain") && method == "POST" {
		return a.setStudioMaterialRetention(ctx, owner, p[3], p[4] == "retain", body)
	}
	if method == "GET" {
		if p[2] == "materials" && len(p) == 4 {
			return readStudioMaterialWith(ctx, a.DB.Pool(), owner, p[3], false)
		}
		if p[2] == "materials" && len(p) == 5 && p[4] == "history" {
			return a.studioMaterialHistory(ctx, owner, p[3], q)
		}
		if p[2] == "jobs" && len(p) == 4 {
			return a.StudioJob(ctx, owner, p[3])
		}
		if len(p) != 3 {
			return nil, ErrNotFound
		}
		return a.studioJobPage(ctx, owner, p[2], q)
	}
	if method != "POST" {
		return nil, ErrNotFound
	}
	if containsPermanentHarm(body) {
		return nil, harmError()
	}
	ctx = context.WithValue(ctx, ownerCommandAuditActionKey{}, "studio_job")
	return a.worldCommand(ctx, owner, "", method+":"+path, body, func(ctx context.Context, tx pgx.Tx) (map[string]any, error) {
		if p[2] == "jobs" && len(p) == 3 {
			if err := lockStudioReferenceGraphTx(ctx, tx, owner); err != nil {
				return nil, err
			}
			if !eventKeys(body, "resource_id", "resource_revision", "kind", "name", "body", "instruction", "reference_asset", "reason", "idempotency_key", "purpose", "resource_version") {
				return nil, invalid("Unknown Studio job fields")
			}
			id := stringValue(body["resource_id"])
			rec, ok, err := (postgresSurfaceTx{tx}).Get(ctx, owner, id)
			if err != nil {
				return nil, err
			}
			if !ok || rec.Kind != "studio_resource" {
				return nil, ErrNotFound
			}
			if err := activeStudioResource(rec.Status); err != nil {
				return nil, err
			}
			expected, ok := parseRevision(body["resource_revision"])
			if !ok || expected != rec.Revision {
				return nil, conflict("studio_resource", expected, rec.Revision)
			}
			kind := stringValue(body["kind"])
			if kind != "assistant" && kind != "text_preview" && kind != "image_preview" {
				return nil, invalid("kind must be assistant, text_preview or image_preview")
			}
			name := stringValue(body["name"])
			definition, ok := body["body"].(map[string]any)
			if !ok {
				return nil, invalid("body must be an object")
			}
			rkind := stringValue(rec.Payload["kind"])
			purpose := stringValue(body["purpose"])
			if purpose != "" || body["resource_version"] != nil {
				if purpose != "foundation_prepare" || kind != "assistant" || body["reference_asset"] != nil {
					return nil, invalid("Definition preparation requires an isolated assistant and exact publication")
				}
				publication, err := readWorldPublication(ctx, tx, owner, id, stringValue(body["resource_version"]))
				if err != nil {
					return nil, err
				}
				if publication.Kind != "character_template" || publication.Body["description"] == nil || publication.Name != name || jsonString(publication.Body) != jsonString(definition) {
					return nil, invalid("Preparation must use the exact published text character template")
				}
				rkind = publication.Kind
			}
			if err := validateStudioDraft(rkind, name, definition, kind == "assistant"); err != nil {
				return nil, err
			}
			instruction := strings.TrimSpace(stringValue(body["instruction"]))
			if instruction == "" || len([]rune(instruction)) > 4000 {
				return nil, invalid("instruction must contain 1..4000 characters")
			}
			snapshot := map[string]any{"owner_actor_id": owner, "resource_id": id, "resource_revision": rec.Revision, "resource_kind": rkind, "name": name, "body": definition, "instruction": instruction}
			if purpose != "" {
				snapshot["purpose"], snapshot["resource_version"] = purpose, body["resource_version"]
			}
			allowed, err := freezeStudioReferencesTx(ctx, tx, owner, definition)
			if err != nil {
				return nil, err
			}
			snapshot["allowed_references"] = allowed
			if ref, exists := body["reference_asset"]; exists {
				reference, ok := ref.(map[string]any)
				if !ok || kind != "image_preview" {
					return nil, invalid("Image reference requires image_preview")
				}
				if err := a.validateStudioReferenceWith(ctx, tx, owner, id, reference); err != nil {
					return nil, err
				}
				snapshot["reference_asset"] = cloneMap(reference)
			}
			job := randomID("studio_job_")
			if _, err := tx.Exec(ctx, `INSERT INTO public.studio_jobs(id,owner_actor_id,resource_id,resource_revision,kind,snapshot,source_digest,status) VALUES($1,$2,$3,$4,$5,$6,$7,'queued')`, job, owner, id, rec.Revision, kind, jsonBytes(snapshot), stableDigest(jsonString(snapshot))); err != nil {
				return nil, err
			}
			if err := enqueueStudioAttemptTx(ctx, tx, owner, job, 1, map[string]any{}); err != nil {
				return nil, err
			}
			return readStudioJobWith(ctx, tx, owner, job, false)
		}
		if p[2] != "jobs" || len(p) != 5 {
			return nil, ErrNotFound
		}
		if !eventKeys(body, "expected_revision", "reason", "idempotency_key") {
			return nil, invalid("Unknown Studio control fields")
		}
		job, err := readStudioJobWith(ctx, tx, owner, p[3], true)
		if err != nil {
			return nil, err
		}
		expected, ok := parseRevision(body["expected_revision"])
		if !ok || expected != intValue(job["revision"]) {
			return nil, conflict("studio_job", expected, intValue(job["revision"]))
		}
		id := p[3]
		switch p[4] {
		case "retry":
			if job["status"] != "failed" {
				return nil, invalid("Only a failed job can be retried")
			}
			if job["kind"] == "image_preview" {
				var active bool
				if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.platform_workflow_intents w JOIN public.media_intents i ON i.id=w.payload->>'intent_id' WHERE i.studio_job_id=$1 AND i.studio_owner_actor_id=$2 AND w.intent_type='media.generation' AND w.status IN ('pending','retry','started','cancel_requested'))`, id, owner).Scan(&active); err != nil {
					return nil, err
				}
				if active {
					return nil, &SurfaceError{Status: 409, Code: "studio_media_execution_active", Message: "The original media workflow is still active or awaiting cancellation; query it before retrying"}
				}
			}
			previous := intValue(job["latest_attempt"])
			var raw []byte
			if err := tx.QueryRow(ctx, `SELECT output FROM public.studio_job_attempts WHERE job_id=$1 AND attempt=$2`, id, previous).Scan(&raw); err != nil {
				return nil, err
			}
			output := decodeObject(raw)
			if len(output) > 0 && job["kind"] != "image_preview" {
				if err := validateStudioOutput(stringValue(job["kind"]), mapValue(job["snapshot"]), output); err != nil {
					return nil, err
				}
			}
			if err := enqueueStudioAttemptTx(ctx, tx, owner, id, previous+1, output); err != nil {
				return nil, err
			}
			if len(output) > 0 {
				if _, err := tx.Exec(ctx, `UPDATE public.studio_job_attempts n SET provenance=p.provenance||jsonb_build_object('reused_from_attempt_id',p.id) FROM public.studio_job_attempts p WHERE n.job_id=$1 AND n.attempt=$2 AND p.job_id=n.job_id AND p.attempt=$3`, id, previous+1, previous); err != nil {
					return nil, err
				}
			}
			if _, err := tx.Exec(ctx, `UPDATE public.studio_jobs SET status='queued',latest_attempt=latest_attempt+1,revision=revision+1,error_code=NULL,updated_at=now() WHERE id=$1`, id); err != nil {
				return nil, err
			}
		case "discard":
			if job["status"] != "discarded" {
				if _, err := tx.Exec(ctx, `UPDATE public.studio_jobs SET status='discarded',result='{}',revision=revision+1,updated_at=now() WHERE id=$1`, id); err != nil {
					return nil, err
				}
				if _, err := tx.Exec(ctx, `UPDATE public.studio_job_attempts SET status='discarded',finished_at=COALESCE(finished_at,now()) WHERE job_id=$1 AND status IN ('queued','running')`, id); err != nil {
					return nil, err
				}
			}
		case "save-material":
			if job["status"] != "completed" || (job["kind"] != "text_preview" && job["kind"] != "image_preview") {
				return nil, invalid("Only completed text previews can be stored as material")
			}
			output := mapValue(job["result"])
			if job["kind"] == "image_preview" {
				return saveStudioImageMaterialTx(ctx, tx, owner, job)
			}
			if err := validateStudioOutput("text_preview", mapValue(job["snapshot"]), output); err != nil {
				return nil, err
			}
			return retainStudioMaterialTx(ctx, tx, owner, job, output["text"], nil, "虚构创作预览·不进入生活")

		default:
			return nil, ErrNotFound
		}
		return readStudioJobWith(ctx, tx, owner, id, false)
	})
}
func (a *App) StudioJob(ctx context.Context, owner, id string) (map[string]any, error) {
	job, err := readStudioJobWith(ctx, a.DB.Pool(), owner, id, false)
	if err != nil {
		return nil, err
	}
	attempts, err := readWorldObjects(ctx, a.DB.Pool(), `SELECT to_jsonb(a)||jsonb_build_object('intent_id',i.intent_id,'intent_status',i.status,'workflow_id',i.workflow_id,'intent_error',CASE WHEN i.last_error IS NULL THEN NULL WHEN i.last_error='workflow_status_unavailable' THEN 'workflow_status_unavailable' ELSE 'workflow_execution_failed' END) FROM public.studio_job_attempts a LEFT JOIN public.platform_workflow_intents i ON i.intent_id='studio_intent:'||a.id WHERE a.job_id=$1 ORDER BY a.attempt`, id)
	if err != nil {
		return nil, err
	}
	media, err := readWorldObjects(ctx, a.DB.Pool(), `SELECT jsonb_build_object('id',i.id,'status',i.status,'workflow_id',i.workflow_id,'provider_job_id',i.provider_job_id,'quality_verdict',i.quality_verdict,'workflows',COALESCE((SELECT jsonb_agg(jsonb_build_object('intent_id',w.intent_id,'workflow_id',w.workflow_id,'status',w.status,'error_code',CASE WHEN w.last_error IS NULL THEN NULL WHEN w.last_error='workflow_status_unavailable' THEN 'workflow_status_unavailable' ELSE 'workflow_execution_failed' END) ORDER BY w.created_at) FROM public.platform_workflow_intents w WHERE w.intent_type='media.generation' AND w.payload->>'intent_id'=i.id),'[]'::jsonb)) FROM public.media_intents i WHERE i.studio_job_id=$1 AND i.studio_owner_actor_id=$2`, id, owner)
	if err != nil {
		return nil, err
	}
	job["media_requests"] = media
	job["attempts"] = attempts
	job["label"] = "虚构创作预览·不进入生活"
	job["life_write"] = false
	return job, err
}
func (a *App) studioJobPage(ctx context.Context, owner, kind string, q url.Values) (map[string]any, error) {
	if kind == "materials" {
		return a.studioMaterialPage(ctx, owner, q)
	}
	table := "studio_jobs"
	if kind == "materials" {
		table = "studio_resource_materials"
	}
	at, id, ok := decodeCursor(q.Get("cursor"))
	if !ok {
		return nil, invalid("Invalid studio cursor")
	}
	if id == "" {
		at = time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	resource := q.Get("resource_id")
	limit := pageLimit(q)
	items, err := readWorldObjects(ctx, a.DB.Pool(), `SELECT to_jsonb(j) FROM public.`+table+` j WHERE owner_actor_id=$1 AND ($2='' OR resource_id=$2) AND (created_at,id)<($3,$4) ORDER BY created_at DESC,id DESC LIMIT $5`, owner, resource, at, id, limit+1)
	if err != nil {
		return nil, err
	}
	next := ""
	if len(items) > limit {
		items = items[:limit]
		last := mapValue(items[limit-1])
		t, err := time.Parse(time.RFC3339Nano, stringValue(last["created_at"]))
		if err != nil {
			return nil, err
		}
		next = encodeCursor(t, stringValue(last["id"]))
	}
	return map[string]any{"items": items, "next_cursor": next}, nil
}

package core

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
)

func ownerMediaRequestKind(kind string) bool { return kind == "media_generate" || kind == "item_photo" }
func (s *OwnerSurface) ownerMediaRequest(ctx context.Context, tx surfaceTx, op surfaceCall, owner string, body map[string]any) (map[string]any, error) {
	if !eventKeys(body, "prompt", "place_id", "expected_place_revision", "item_id", "expected_item_revision", "reference_asset", "reason", "idempotency_key") {
		return nil, invalid("Unknown image request field")
	}
	request := cloneMap(body)
	kind := "media_generate"
	if op.name == "photo" {
		kind = "item_photo"
		if request["item_id"] != nil {
			return nil, invalid("Photo target is bound by the route")
		}
		request["item_id"] = op.a
	}
	if s.hooks.EnqueueLifeRequest == nil {
		return nil, &SurfaceError{Status: 503, Code: "owner_media_request_unavailable", Message: "图片请求处理服务尚未就绪"}
	}
	queued, err := s.insertRequest(ctx, tx, owner, op.fid, kind, request)
	if err != nil {
		return nil, err
	}
	return withStatus(202, map[string]any{"request": queued.Payload, "audit_id": queued.Audit}), nil
}
func (a *App) validateOwnerMediaRequestTx(ctx context.Context, tx pgx.Tx, owner, fid, kind string, body map[string]any) error {
	prompt := strings.TrimSpace(stringValue(body["prompt"]))
	if prompt == "" || len([]rune(prompt)) > 2000 {
		return invalid("Image prompt must contain 1–2000 characters")
	}
	render := prompt
	if id := stringValue(body["item_id"]); id != "" {
		expected, ok := parseRevision(body["expected_item_revision"])
		if !ok || expected < 1 {
			return invalid("Explicit item revision required")
		}
		var raw []byte
		err := tx.QueryRow(ctx, `SELECT jsonb_build_object('id',id,'revision',revision,'item_kind',item_kind,'category',category,'slot',slot,'description',description,'ownership',ownership,'availability',availability) FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND id=$2 AND archived_at IS NULL FOR SHARE`, fid, id).Scan(&raw)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		item := decodeObject(raw)
		if expected != intValue(item["revision"]) {
			return conflict("wardrobe_item", expected, intValue(item["revision"]))
		}
		if previous := mapValue(body["_item_snapshot"]); len(previous) > 0 && jsonString(previous) != jsonString(item) {
			return &SurfaceError{Status: 409, Code: "media_request_item_changed", Message: "原图片请求的物品资料已变化，请重新核对；没有生成新图片"}
		}
		body["_item_snapshot"] = item
		render += "\n画面关联的物品定义：" + stringValue(item["description"]) + "。这是生成画面，不证明新取得、穿着或使用。"
	} else if kind == "item_photo" {
		return invalid("Actual item target required")
	}
	if place := stringValue(body["place_id"]); place != "" {
		if err := validateLifePlaceTx(ctx, tx, fid, place); err != nil {
			return err
		}
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT jsonb_build_object('id',id,'revision',revision,'name',name,'description',description) FROM public.world_places WHERE id=$1 FOR SHARE`, place).Scan(&raw); err != nil {
			return err
		}
		snapshot := decodeObject(raw)
		expected, ok := parseRevision(body["expected_place_revision"])
		if !ok || expected != intValue(snapshot["revision"]) {
			return conflict("world_place", expected, intValue(snapshot["revision"]))
		}
		if previous := mapValue(body["_place_snapshot"]); len(previous) > 0 && jsonString(previous) != jsonString(snapshot) {
			return &SurfaceError{Status: 409, Code: "media_request_place_changed", Message: "原图片请求的地点定义已变化，请核对后重新提出请求"}
		}
		body["_place_snapshot"] = snapshot
		render += "\n明确请求的画面地点：" + stringValue(snapshot["name"]) + "。这仅为画面场景，不表示角色移动。"
	}
	if len([]rune(render)) > 4000 {
		return invalid("Image request and selected sources exceed the bounded media intent")
	}
	if reference, exists := body["reference_asset"]; exists {
		ref, ok := reference.(map[string]any)
		if !ok {
			return invalid("Explicit reference grant required")
		}
		if err := a.validateExplicitMediaReferenceFor(ctx, tx, owner, "fluctlight", fid, ref); err != nil {
			return err
		}
	}
	body["_render_intent"] = render
	return nil
}

type ownerMediaRequestScopeKey struct{}
type ownerMediaRequestScope struct{ ID, Owner, FID, Inbox string }
type ownerMediaCapability struct{ ownerLifeCapability }

func (c ownerMediaCapability) Preflight(ctx context.Context, resolved CapabilityContext) error {
	if check, ok := c.Capability.(CapabilityPreflighter); ok {
		return check.Preflight(ctx, resolved)
	}
	return errors.New("media_preflight_unavailable")
}

var _ CapabilityPreflighter = ownerMediaCapability{}

func (c ownerMediaCapability) Prepare(ctx context.Context, i CapabilityInvocation, r CapabilityContext) (CapabilityInvocation, error) {
	args := decodeObject(i.Arguments)
	if strings.TrimSpace(stringValue(args["intent"])) != strings.TrimSpace(stringValue(c.request["_render_intent"])) || args["reference_asset"] != nil {
		return i, newCapabilityError("owner_media_intent_mismatch", false, ErrUnauthorized)
	}
	prepared, err := c.ownerLifeCapability.Prepare(ctx, i, r)
	if err != nil {
		return prepared, err
	}
	raw, found, err := capabilityPreparedData(prepared, "media_concept")
	if err != nil || !found {
		return prepared, ErrInvalidArguments
	}
	concept := cloneMap(mapValue(raw))
	if ref := mapValue(c.request["reference_asset"]); len(ref) > 0 {
		if ref["purpose"] == "person" && mediaPlanHasSelfIdentity(mapValue(concept["media_capture_plan"])) {
			return prepared, newCapabilityError("unverified_reference_cannot_identify_self", false, ErrUnauthorized)
		}
		concept["reference_asset"] = cloneMap(ref)
	}
	concept["owner_media_request"] = map[string]any{"request_id": c.request["_request_id"], "kind": c.kind, "item": c.request["_item_snapshot"], "requested_scene": c.request["_place_snapshot"], "reference_asset": c.request["reference_asset"], "semantics": "Image-only request; source definitions do not change Life, ownership, wearing or location. Explicit person references do not establish self identity."}
	if err := validateBranchedMediaConcept(concept); err != nil {
		return prepared, err
	}
	return withCapabilityPreparedData(prepared, "media_concept", concept)
}
func (c ownerMediaCapability) ExecuteDirectTx(ctx context.Context, tx pgx.Tx, i CapabilityInvocation, r CapabilityContext, target DirectToolTarget) (CapabilityResult, error) {
	id := stringValue(c.request["_request_id"])
	if target.Kind != "owner_media_request" || target.Ref != id || target.AuthorizationActorID != c.owner || target.FluctlightID != c.fid {
		return failedCapabilityResult(i, "owner_media_target_mismatch", false), ErrUnauthorized
	}
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT payload FROM public.owner_surface_records WHERE id=$1 AND owner_actor_id=$2 AND fluctlight_id=$3 AND kind='owner_request' FOR UPDATE`, id, c.owner, c.fid).Scan(&raw); err != nil {
		return failedCapabilityResult(i, "owner_media_request_missing", false), err
	}
	record := decodeObject(raw)
	if record["source_fact_id"] != i.SourceFactID || record["kind"] != c.kind {
		return failedCapabilityResult(i, "owner_media_origin_mismatch", false), ErrUnauthorized
	}
	var mid, workflow, status string
	err := tx.QueryRow(ctx, `SELECT id,workflow_id,status FROM public.media_intents WHERE owner_request_id=$1 AND owner_fluctlight_id=$2`, id, c.fid).Scan(&mid, &workflow, &status)
	if err == nil {
		return CapabilityResult{CallID: i.CallID, CapabilityName: i.CapabilityName, Status: "accepted", Output: map[string]any{"media_intent_id": mid, "task_id": workflow, "status": status, "target_kind": target.Kind, "target_ref": id, "replayed": true}, ProviderRequestID: i.ProviderRequestID}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return failedCapabilityResult(i, "owner_media_read_failed", true), err
	}
	if record["status"] != "pending" {
		return failedCapabilityResult(i, "owner_media_request_terminal", false), ErrConflict
	}
	if err := c.app.validateOwnerLifeRequestTx(ctx, tx, c.owner, c.fid, c.kind, c.request); err != nil {
		return failedCapabilityResult(i, "owner_media_request_stale", false), err
	}
	writer, ok := c.Capability.(DirectToolCapability)
	if !ok {
		return failedCapabilityResult(i, "owner_media_writer_missing", false), ErrCapabilityNotFound
	}
	ctx = context.WithValue(ctx, ownerMediaRequestScopeKey{}, ownerMediaRequestScope{id, c.owner, c.fid, i.SourceFactID})
	result, err := writer.ExecuteDirectTx(ctx, tx, i, r, target)
	if err != nil {
		return result, err
	}
	mid = stringValue(mapValue(result.Output)["media_intent_id"])
	if mid == "" {
		return result, errors.New("owner_media_intent_receipt_missing")
	}
	linked, err := tx.Exec(ctx, `UPDATE public.media_intents SET owner_request_id=$2,owner_item_id=$3 WHERE id=$1 AND owner_fluctlight_id=$4 AND owner_request_id IS NULL`, mid, id, nilIfEmpty(stringValue(c.request["item_id"])), c.fid)
	if err != nil {
		return result, err
	}
	if linked.RowsAffected() != 1 {
		return result, ErrConflict
	}
	_, err = tx.Exec(ctx, `UPDATE public.platform_workflow_intents SET payload=payload||jsonb_build_object('request_id',$2::text) WHERE intent_type='media.generation' AND payload->>'intent_id'=$1`, mid, id)
	return result, err
}
func authorizeOwnerMediaTargetTx(ctx context.Context, tx pgx.Tx, target MediaPublicationTarget) error {
	scope, ok := ctx.Value(ownerMediaRequestScopeKey{}).(ownerMediaRequestScope)
	if !ok || scope.ID != target.Ref || scope.Owner != target.AuthorizationActorID || scope.FID != target.FluctlightID || scope.Inbox == "" {
		return ErrUnauthorized
	}
	var valid bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.owner_surface_records WHERE id=$1 AND owner_actor_id=$2 AND fluctlight_id=$3 AND kind='owner_request' AND status='pending' AND payload->>'source_fact_id'=$4 AND payload->>'kind' IN ('media_generate','item_photo'))`, scope.ID, scope.Owner, scope.FID, scope.Inbox).Scan(&valid); err != nil {
		return err
	}
	if !valid {
		return ErrUnauthorized
	}
	return nil
}

func ownerMediaReadRoute(method string, p []string) bool {
	return method == "GET" && len(p) >= 3 && len(p) <= 4 && p[0] == "fluctlights" && p[2] == "media-generations"
}
func (a *App) ownerMediaGenerationRead(ctx context.Context, owner string, p []string, q url.Values) (map[string]any, error) {
	fid := p[1]
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	if len(p) == 3 {
		limit, err := reflectionReadPage(q)
		if err != nil {
			return nil, err
		}
		scope := stableDigest(jsonString([]any{"owner-media-requests", owner, fid}))
		cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
		if err != nil {
			return nil, err
		}
		rows, err := a.DB.Pool().Query(ctx, `SELECT id,created_at,payload FROM public.owner_surface_records WHERE owner_actor_id=$1 AND fluctlight_id=$2 AND kind='owner_request' AND payload->>'kind' IN ('media_generate','item_photo') AND ($3='' OR (created_at,id)<($4,$3)) ORDER BY created_at DESC,id DESC LIMIT $5`, owner, fid, cursor.ID, cursor.Time, limit+1)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return readGovernanceRows(rows, scope, limit)
	}
	var raw []byte
	err := a.DB.Pool().QueryRow(ctx, `SELECT payload FROM public.owner_surface_records WHERE id=$1 AND owner_actor_id=$2 AND fluctlight_id=$3 AND kind='owner_request' AND payload->>'kind' IN ('media_generate','item_photo')`, p[3], owner, fid).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	request := decodeObject(raw)
	tasks, err := a.OwnerRequestTasks(ctx, owner, fid, p[3], stringValue(request["source_fact_id"]))
	if err != nil {
		return nil, err
	}
	request["tasks"], request["tasks_state"] = tasks, "available"
	result := map[string]any{"fluctlight_id": fid, "request": request, "media": nil, "asset": nil, "factual": false, "label": "生成图·非事实"}
	err = a.DB.Pool().QueryRow(ctx, `SELECT jsonb_build_object('id',id,'status',status,'workflow_id',workflow_id,'provider_job_id',provider_job_id,'quality_verdict',quality_verdict,'item_id',owner_item_id,'source',prompt::jsonb->'owner_media_request') FROM public.media_intents WHERE owner_request_id=$1 AND owner_fluctlight_id=$2`, p[3], fid).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	media := decodeObject(raw)
	result["media"] = media
	var assetID string
	err = a.DB.Pool().QueryRow(ctx, `SELECT id FROM public.media_assets WHERE id=$1 AND owner_fluctlight_id=$2`, "asset_"+stringValue(media["id"]), fid).Scan(&assetID)
	if err == nil {
		asset, err := a.ownerMediaAssetWith(ctx, a.DB.Pool(), owner, assetID, false)
		if err != nil {
			return nil, err
		}
		result["asset"] = asset
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	return result, nil
}
func (a *App) publishOwnerMediaAsset(ctx context.Context, intent mediaIntent, asset string) error {
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var valid bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.media_assets m JOIN public.media_intents i ON m.id='asset_'||i.id JOIN public.owner_surface_records r ON r.id=i.owner_request_id WHERE i.id=$1 AND i.owner_request_id=$2 AND i.owner_fluctlight_id=$3 AND m.id=$4 AND m.owner_fluctlight_id=$3 AND m.media_scope='fluctlight' AND m.status='ready' AND r.fluctlight_id=$3 AND r.kind='owner_request' AND r.payload->>'kind' IN ('media_generate','item_photo'))`, intent.ID, intent.OwnerRequestID, intent.Owner, asset).Scan(&valid); err != nil {
			return err
		}
		if !valid {
			return ErrUnauthorized
		}
		targets := map[string]string{"owner_media_request": intent.OwnerRequestID}
		if intent.OwnerItemID != "" {
			targets["item"] = intent.OwnerItemID
		}
		for kind, id := range targets {
			if _, err := tx.Exec(ctx, `INSERT INTO public.media_references(id,asset_id,owner_fluctlight_id,target_type,target_id) VALUES($1,$2,$3,$4,$5) ON CONFLICT(id) DO NOTHING`, "media_ref_"+stableDigest(asset+":"+kind+":"+id), asset, intent.Owner, kind, id); err != nil {
				return err
			}
		}
		return nil
	})
}

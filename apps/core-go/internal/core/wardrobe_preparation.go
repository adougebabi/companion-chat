package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const wardrobePreparationInstruction = `You own initial wardrobe preparation for this fictional character. Use the supplied current persona, climate/season when known, current situation, registered clothing and tagged plans. Do not infer owned clothing from preferences. When starter_items_allowed is false, return items=[] and compose plans from existing items only. The Owner explicitly authorizes a modest persona-appropriate starter wardrobe when no complete usable outfit exists; this is initial fictional inventory, not a purchase, loan, payment or completed life activity. Reuse usable registered items first. Standard clothing types: outer, upper_body, underwear, panties, lower_body, socks, shoes, accessory. A plan can have several scenario tags and several accessories. Body coverage can use existing full_body garments; coat, socks, shoes and accessories depend on the situation rather than always being mandatory. Give every created garment a local key, concrete Chinese description, standard type and seasons (spring/summer/autumn/winter; unknown is []). Build 1–4 useful tagged plans using existing item IDs or the new local keys. No runtime IDs may be invented. In select_only mode create no items or plans: select one appropriate saved plan, or a coherent combination of current usable items, for the actual current context. Return Chinese reason, never claim any item is worn; only Core's separate wardrobe.wear Tool can establish that. Respect existing current wear and no gratuitous changes. Return only the requested JSON.`

func wardrobePreparationSchema() map[string]any {
	return objectSchema(map[string]any{
		"items":          map[string]any{"type": "array", "maxItems": 16, "items": objectSchema(map[string]any{"key": stringSchema(), "description": stringSchema(), "clothing_type": enumStringSchema("outer", "upper_body", "underwear", "panties", "lower_body", "socks", "shoes", "accessory"), "seasons": arraySchema(enumStringSchema("spring", "summer", "autumn", "winter"))}, []string{"key", "description", "clothing_type", "seasons"}, false)},
		"plans":          map[string]any{"type": "array", "maxItems": 4, "items": objectSchema(map[string]any{"name": stringSchema(), "tags": arraySchema(stringSchema()), "item_refs": arraySchema(stringSchema())}, []string{"name", "tags", "item_refs"}, false)},
		"wear_item_refs": arraySchema(stringSchema()), "reason": stringSchema(),
	}, []string{"items", "plans", "wear_item_refs", "reason"}, false)
}

func ensureWardrobePreparationTx(ctx context.Context, tx pgx.Tx, fid, source string) (string, error) {
	id := "wardrobe_preparation_" + stableDigest(fid)
	_, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_wardrobe_preparations(id,fluctlight_id,owner_actor_id,profile_id,expected_revision,source_ref,intent_id)
 SELECT $2,f.id,f.created_by_actor_id,r.active_profile_id,w.revision,$3,$4 FROM public.fluctlights f JOIN public.fluctlight_personality_runtime r ON r.fluctlight_id=f.id JOIN public.fluctlight_wardrobe_states w ON w.fluctlight_id=f.id WHERE f.id=$1 ON CONFLICT(fluctlight_id) DO NOTHING`, fid, id, source, "wardrobe_initialize:"+fid)
	if err != nil {
		return "", err
	}
	err = insertWardrobePreparationIntentTx(ctx, tx, id)
	return id, err
}

func insertWardrobePreparationIntentTx(ctx context.Context, tx pgx.Tx, id string) error {
	_, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status)
 SELECT intent_id,intent_id,'lifecycle','wardrobe.initialize',jsonb_build_object('fluctlight_id',fluctlight_id,'session_id',id,'correlation_id','wardrobe-preparation:'||id),'pending' FROM public.fluctlight_wardrobe_preparations WHERE id=$1 AND status<>'completed' ON CONFLICT(intent_id) DO NOTHING`, id)
	return err
}
func (a *App) OwnerWardrobePreparation(ctx context.Context, owner, fid string) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	var raw []byte
	err := a.DB.Pool().QueryRow(ctx, `SELECT (to_jsonb(p)-'proposal')||jsonb_build_object('can_recover',p.status<>'completed' AND (p.status='failed' OR COALESCE(w.status,'') IN ('failed','cancelled','dead_letter'))) FROM public.fluctlight_wardrobe_preparations p LEFT JOIN public.platform_workflow_intents w ON w.intent_id=p.intent_id WHERE p.fluctlight_id=$1`, fid).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return map[string]any{"status": "not_started", "fluctlight_id": fid}, nil
	}
	if err != nil {
		return nil, err
	}
	out := decodeObject(raw)
	out["task_id"] = out["intent_id"]
	return out, nil
}
func (a *App) RequestOwnerWardrobePreparation(ctx context.Context, owner, fid string, body map[string]any) (map[string]any, error) {
	tx, ok := SurfaceDBTx(ctx)
	if !ok {
		return nil, ErrInvalidArguments
	}
	if !eventKeys(body, "expected_revision", "reason", "idempotency_key") {
		return nil, ErrInvalidArguments
	}
	if err := lockLifeContextTx(ctx, tx, fid); err != nil {
		return nil, err
	}
	if err := requireOwnerMutableTx(ctx, tx, owner, fid); err != nil {
		return nil, err
	}
	expected, ok := parseRevision(body["expected_revision"])
	if !ok || strings.TrimSpace(stringValue(body["reason"])) == "" {
		return nil, ErrInvalidArguments
	}
	var revision int
	if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1 FOR UPDATE`, fid).Scan(&revision); err != nil {
		return nil, err
	}
	if expected != revision {
		return nil, conflict("wardrobe", expected, revision)
	}
	id, err := ensureWardrobePreparationTx(ctx, tx, fid, "owner:"+owner)
	if err != nil {
		return nil, err
	}
	// Explicit recovery receives a new execution identity. The previous failed
	// Run and ledger remain terminal; reconciliation cannot close a newly
	// reopened intent using the previous Run's terminal state.
	if _, err = resumeWardrobePreparationTx(ctx, tx, id, true); err != nil {
		return nil, err
	}
	if err = insertWardrobePreparationIntentTx(ctx, tx, id); err != nil {
		return nil, err
	}

	var raw []byte
	if err = tx.QueryRow(ctx, `SELECT to_jsonb(p)-'proposal' FROM public.fluctlight_wardrobe_preparations p WHERE id=$1`, id).Scan(&raw); err != nil {
		return nil, err
	}
	out := decodeObject(raw)
	out["task_id"] = out["intent_id"]
	return out, nil
}

// Manual recovery may explicitly resume a cancelled/exhausted initializer.
// Automatic recovery is bounded, delayed and never reverses cancellation.
func resumeWardrobePreparationTx(ctx context.Context, tx pgx.Tx, id string, explicit bool) (bool, error) {
	tag, err := tx.Exec(ctx, `UPDATE public.fluctlight_wardrobe_preparations p SET status=CASE WHEN result->'inventory_committed'='true'::jsonb THEN 'awaiting_wear' ELSE 'queued' END,error_code=NULL,attempt=attempt+1,intent_id='wardrobe_initialize:'||fluctlight_id||':attempt:'||(attempt+1)::text,updated_at=now() WHERE id=$1 AND status<>'completed' AND (status='failed' OR EXISTS(SELECT 1 FROM public.platform_workflow_intents w WHERE w.intent_id=p.intent_id AND w.status IN ('failed','cancelled','dead_letter'))) AND ($2 OR (attempt<3 AND updated_at<now()-GREATEST(attempt,1)*interval '1 minute' AND NOT EXISTS(SELECT 1 FROM public.platform_workflow_intents w WHERE w.intent_id=p.intent_id AND w.status IN ('cancelled','cancel_requested'))))`, id, explicit)
	return err == nil && tag.RowsAffected() == 1, err
}
func (a *App) RepairWardrobePreparations(ctx context.Context, limit int) (int, error) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT p.id,p.fluctlight_id,f.created_by_actor_id FROM public.fluctlight_wardrobe_preparations p JOIN public.fluctlights f ON f.id=p.fluctlight_id WHERE f.status='active' AND p.status<>'completed' AND p.attempt<3 AND p.updated_at<now()-GREATEST(p.attempt,1)*interval '1 minute' AND NOT EXISTS(SELECT 1 FROM public.platform_workflow_intents w WHERE w.intent_id=p.intent_id AND w.status IN ('cancelled','cancel_requested')) AND (p.status='failed' OR EXISTS(SELECT 1 FROM public.platform_workflow_intents w WHERE w.intent_id=p.intent_id AND w.status IN ('failed','dead_letter'))) ORDER BY p.updated_at,p.id LIMIT $1`, max(1, min(limit, 50)))
	if err != nil {
		return 0, err
	}
	type candidate struct{ id, fid, owner string }
	items := []candidate{}
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.id, &item.fid, &item.owner); err != nil {
			rows.Close()
			return 0, err
		}
		items = append(items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, item := range items {
		resumed := false
		err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			// Match the manual writer's lock order and recheck lifecycle before a new
			// attempt. Never hold a DB connection over Provider work.
			if err := lockLifeContextTx(ctx, tx, item.fid); err != nil {
				return err
			}
			var status string
			if err := tx.QueryRow(ctx, `SELECT status FROM public.fluctlights WHERE id=$1 AND created_by_actor_id=$2 FOR SHARE`, item.fid, item.owner).Scan(&status); errors.Is(err, pgx.ErrNoRows) {
				return nil
			} else if err != nil {
				return err
			}
			if status != "active" {
				return nil
			}
			var err error
			resumed, err = resumeWardrobePreparationTx(ctx, tx, item.id, false)
			if err != nil {
				return err
			}
			if resumed {
				if err := insertWardrobePreparationIntentTx(ctx, tx, item.id); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return count, err
		}
		if resumed {
			count++
		}
	}
	return count, nil
}

func (a *App) ProcessWardrobePreparation(ctx context.Context, id string) (result map[string]any, runErr error) {
	// A session advisory lock serializes only this initializer, never a domain
	// transaction over Provider I/O. A lost connection releases the lock.
	conn, err := a.DB.Pool().Acquire(ctx)
	if err != nil {
		return nil, err
	}
	var locked bool
	if err = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1,0))`, "wardrobe-preparation:"+id).Scan(&locked); err != nil {
		conn.Release()
		return nil, err
	}
	if !locked {
		conn.Release()
		return map[string]any{"status": "awaiting_wear", "reason": "preparation_already_running"}, nil
	}
	defer func() {
		release, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		if _, err := conn.Exec(release, `SELECT pg_advisory_unlock(hashtextextended($1,0))`, "wardrobe-preparation:"+id); err != nil {
			_ = conn.Conn().Close(release)
		}
		conn.Release()
	}()
	var raw []byte
	if err = a.DB.Pool().QueryRow(ctx, `SELECT to_jsonb(p) FROM public.fluctlight_wardrobe_preparations p WHERE id=$1`, id).Scan(&raw); err != nil {
		return nil, err
	}
	state := decodeObject(raw)
	fid, owner := stringValue(state["fluctlight_id"]), stringValue(state["owner_actor_id"])
	ctx = WithPromptDiagnostics(WithProviderCorrelation(ctx, "wardrobe-preparation:"+id), map[string]any{"fluctlight_id": fid, "owner_actor_id": owner, "wardrobe_preparation_id": id})
	defer func() {
		if errors.Is(runErr, ErrConflict) {
			recoverCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
			defer cancel()
			if _, err := a.DB.Pool().Exec(recoverCtx, `UPDATE public.fluctlight_wardrobe_preparations SET status=CASE WHEN result->'inventory_committed'='true'::jsonb THEN 'awaiting_wear' ELSE 'queued' END,updated_at=now() WHERE id=$1 AND status<>'completed'`, id); err == nil {
				result = map[string]any{"status": "awaiting_wear", "reason": "资料或衣柜已变化，将按最新状态继续准备"}
				runErr = nil
			} else {
				runErr = err
			}
		}
		if runErr != nil {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
			defer cancel()
			_, _ = a.DB.Pool().Exec(cleanup, `UPDATE public.fluctlight_wardrobe_preparations SET status='failed',error_code=$2,result=result||jsonb_build_object('failure_reason',$3::text),updated_at=now() WHERE id=$1 AND status<>'completed'`, id, "wardrobe_preparation_failed", ownerRequestFailureReason(runErr))
			a.recordDiagnosticEvent(cleanup, "wardrobe.preparation.failed", "error", fid, id, "wardrobe-preparation:"+id, map[string]any{"safe_cause": boundedOperationalCause(runErr.Error()), "error_code": "wardrobe_preparation_failed"})
		}
	}()
	if state["status"] == "completed" {
		return mapValue(state["result"]), nil
	}
	resource, err := a.DB.GetFluctlight(ctx, fid, owner)
	if err != nil {
		return nil, err
	}
	if resource.Status != "active" {
		return map[string]any{"status": "awaiting_wear", "reason": "角色当前未处于活动状态"}, nil
	}
	profile, err := newWardrobeService(a).effectiveProfileID(ctx, a.DB.Pool(), fid, "")
	if err != nil {
		return nil, err
	}
	sections, _, _, _, err := newPersonaDetailService(a).read(ctx, fid, owner, profile)
	if err != nil {
		return nil, err
	}
	_, life, err := a.readLifeContextSnapshotAt(ctx, fid, a.now())
	if err != nil {
		return nil, err
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT to_jsonb(i)||jsonb_build_object('source_verified',`+inventorySourceVerifiedSQL+`) FROM public.fluctlight_wardrobe_items i WHERE fluctlight_id=$1 AND item_kind='wearable' AND archived_at IS NULL ORDER BY id`, fid)
	if err != nil {
		return nil, err
	}
	items := []map[string]any{}
	for rows.Next() {
		var item []byte
		if err = rows.Scan(&item); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, decodeObject(item))
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	var revision int
	if err = a.DB.Pool().QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1`, fid).Scan(&revision); err != nil {
		return nil, err
	}
	saved := mapValue(state["result"])
	selectOnly := saved["inventory_committed"] == true
	if selectOnly && life["sleeping"] == true {
		return map[string]any{"status": "awaiting_wear", "reason": "方案已准备，等待清醒后结合当前情境换装"}, nil
	}
	// A committed wear invocation is frozen before execution. Recovery reuses
	// that operation rather than selecting and performing a second outfit.
	if selectOnly && stringValue(saved["wear_operation"]) != "" {
		return a.finishWardrobePreparationWear(ctx, id, fid, owner, profile, saved)
	}
	planRows, err := a.DB.Pool().Query(ctx, `SELECT `+inventoryOutfitProjection+` FROM public.fluctlight_wardrobe_outfits o WHERE o.fluctlight_id=$1 AND (o.profile_id='' OR o.profile_id=$2) ORDER BY o.id`, fid, profile)
	if err != nil {
		return nil, err
	}
	plans := []map[string]any{}
	for planRows.Next() {
		var plan []byte
		if err = planRows.Scan(&plan); err != nil {
			planRows.Close()
			return nil, err
		}
		plans = append(plans, decodeObject(plan))
	}
	err = planRows.Err()
	planRows.Close()
	if err != nil {
		return nil, err
	}
	if _, err = a.DB.Pool().Exec(ctx, `UPDATE public.fluctlight_wardrobe_preparations SET status=$2,error_code=NULL,updated_at=now() WHERE id=$1`, id, map[bool]string{true: "awaiting_wear", false: "generating"}[selectOnly]); err != nil {
		return nil, err
	}
	input := map[string]any{"select_only": selectOnly, "persona": sections, "current_life": life, "registered_items": items, "plans": plans, "starter_items_allowed": !usableWardrobeCoverage(items)}
	messages := (&PromptComposer{}).ComposeTaskMessages("cognitive_assessment", []map[string]any{{"role": "system", "content": wardrobePreparationInstruction}, {"role": "user", "content": jsonString(input)}})
	run, err := a.runFormalStructuredTask(ctx, FormalAgentWardrobePreparation, messages, nil, "wardrobe_preparation_v1", wardrobePreparationSchema(), false, nil)
	if err != nil {
		return nil, err
	}
	proposal := run.Completion.Structured
	if _, err = a.DB.Pool().Exec(ctx, `UPDATE public.fluctlight_wardrobe_preparations SET proposal=$2,updated_at=now() WHERE id=$1`, id, jsonBytes(proposal)); err != nil {
		return nil, err
	}
	if !selectOnly {
		return a.commitPreparedWardrobe(ctx, id, fid, owner, profile, revision, resource.CurrentRevision, proposal)
	}
	if len(arrayValue(proposal["items"])) > 0 || len(arrayValue(proposal["plans"])) > 0 {
		return nil, errors.New("wardrobe_select_only_created_inventory")
	}
	ids := decisionServiceRefValues(proposal["wear_item_refs"])
	if len(ids) == 0 || len(ids) > 16 {
		return nil, errors.New("wardrobe_wear_selection_missing")
	}
	allowed := map[string]bool{}
	selectedItems := []map[string]any{}
	for _, item := range items {
		if item["source_verified"] == true && item["availability"] == "available" && (item["ownership"] == "owned" || item["ownership"] == "borrowed") {
			allowed[stringValue(item["id"])] = true
		}
	}
	for _, itemID := range ids {
		if !allowed[itemID] {
			return nil, errors.New("wardrobe_wear_selection_unavailable")
		}
		for _, item := range items {
			if item["id"] == itemID {
				selectedItems = append(selectedItems, item)
			}
		}
	}
	if !wardrobeCoversBody(selectedItems) {
		return nil, errors.New("wardrobe_initial_wear_incomplete")
	}
	_, latestLife, err := a.readLifeContextSnapshotAt(ctx, fid, a.now())
	if err != nil {
		return nil, err
	}
	if latestLife["context_revision"] != life["context_revision"] {
		return map[string]any{"status": "awaiting_wear", "reason": "当前情境已变化，将按新情境重新选择"}, nil
	}
	saved["wear_item_ids"], saved["wear_reason"] = ids, stringValue(proposal["reason"])
	saved["wear_profile_id"] = profile
	saved["wear_fence"] = map[string]any{"revision": revision, "profile_id": profile, "life_revision": life["context_revision"]}
	saved["wear_operation"] = "wardrobe-initial-wear:" + id + ":" + stableDigest(jsonString([]any{ids, profile, revision, life["context_revision"]}))
	if _, err = a.DB.Pool().Exec(ctx, `UPDATE public.fluctlight_wardrobe_preparations SET result=$2,updated_at=now() WHERE id=$1`, id, jsonBytes(saved)); err != nil {
		return nil, err
	}
	return a.finishWardrobePreparationWear(ctx, id, fid, owner, profile, saved)
}

func (a *App) commitPreparedWardrobe(ctx context.Context, id, fid, owner, profile string, revision, foundationRevision int, proposal map[string]any) (map[string]any, error) {
	returnValue := map[string]any{}
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := lockLifeContextTx(ctx, tx, fid); err != nil {
			return err
		}
		if err := requireOwnerMutableTx(ctx, tx, owner, fid); err != nil {
			return err
		}
		var current, foundation int
		var active, status string
		if err := tx.QueryRow(ctx, `SELECT w.revision,f.current_revision,r.active_profile_id,f.status FROM public.fluctlight_wardrobe_states w JOIN public.fluctlights f ON f.id=w.fluctlight_id JOIN public.fluctlight_personality_runtime r ON r.fluctlight_id=f.id WHERE w.fluctlight_id=$1 FOR UPDATE OF w`, fid).Scan(&current, &foundation, &active, &status); err != nil {
			return err
		}
		if current != revision || foundation != foundationRevision || active != profile || status != "active" {
			return ErrConflict
		}
		refs := map[string]string{}
		inventory := map[string]map[string]any{}
		rows, err := tx.Query(ctx, `SELECT to_jsonb(i)||jsonb_build_object('source_verified',`+inventorySourceVerifiedSQL+`) FROM public.fluctlight_wardrobe_items i WHERE fluctlight_id=$1 AND item_kind='wearable' AND archived_at IS NULL`, fid)
		if err != nil {
			return err
		}
		for rows.Next() {
			var raw []byte
			if err = rows.Scan(&raw); err != nil {
				rows.Close()
				return err
			}
			item := decodeObject(raw)
			itemID := stringValue(item["id"])
			refs[itemID] = itemID
			inventory[itemID] = item
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		members := arrayValue(proposal["items"])
		existingItems := []map[string]any{}
		for _, item := range inventory {
			existingItems = append(existingItems, item)
		}
		if len(members) > 0 && usableWardrobeCoverage(existingItems) {
			return errors.New("wardrobe_starter_inventory_not_required")
		}
		if len(members) > 16 {
			return ErrInvalidArguments
		}
		for index, raw := range members {
			item := mapValue(raw)
			key := strings.TrimSpace(stringValue(item["key"]))
			kind := stringValue(item["clothing_type"])
			description := strings.TrimSpace(stringValue(item["description"]))
			if key == "" || refs[key] != "" || !clothingTypes[kind] || description == "" || len([]rune(description)) > 512 {
				return errors.New("wardrobe_initial_item_invalid")
			}
			_, seasons, err := wardrobeMetadata(map[string]any{"item_kind": "wearable", "slot": kind, "clothing_type": kind, "seasons": item["seasons"]})
			if err != nil {
				return err
			}
			itemID := "wardrobe_" + stableDigest(id+":"+fmt.Sprint(index))
			refs[key] = itemID
			inventory[itemID] = map[string]any{"id": itemID, "slot": kind, "ownership": "owned", "availability": "available", "source_verified": true}
			if _, err = tx.Exec(ctx, `INSERT INTO public.fluctlight_wardrobe_items(id,fluctlight_id,category,slot,description,ownership,availability,source_kind,source_ref,source_item_key,item_kind,clothing_type,seasons)VALUES($1,$2,$3::text,$3::text,$4,'owned','available','initialization',$5,$6,'wearable',$3::text,$7)`, itemID, fid, kind, description, "wardrobe-preparation:"+id, key, seasons); err != nil {
				return err
			}
		}
		if len(members) > 0 {
			if _, err = tx.Exec(ctx, `UPDATE public.fluctlight_wardrobe_states SET revision=revision+1,updated_at=now() WHERE fluctlight_id=$1`, fid); err != nil {
				return err
			}
			current++
		}
		plans := arrayValue(proposal["plans"])
		if len(plans) == 0 || len(plans) > 4 {
			return errors.New("wardrobe_initial_plans_missing")
		}
		planIDs := []string{}
		usableCoverage := false
		for index, raw := range plans {
			plan := mapValue(raw)
			tags, err := normalizedOutfitTags(plan["tags"])
			if err != nil {
				return err
			}
			itemIDs := []string{}
			members := []map[string]any{}
			usable := true
			for _, ref := range decisionServiceRefValues(plan["item_refs"]) {
				itemID := refs[ref]
				if itemID == "" {
					return errors.New("wardrobe_initial_plan_reference_invalid")
				}
				itemIDs = append(itemIDs, itemID)
				item := inventory[itemID]
				members = append(members, item)
				if item["source_verified"] != true || item["availability"] != "available" || (item["ownership"] != "owned" && item["ownership"] != "borrowed") {
					usable = false
				}
			}
			if usable && wardrobeCoversBody(members) {
				usableCoverage = true
			}
			out, err := newWardrobeService(a).saveOutfitTx(ctx, tx, wardrobeOutfitWrite{FluctlightID: fid, ProfileID: profile, Name: stringValue(plan["name"]), ItemIDs: itemIDs, Tags: tags, TagsSet: true, OperationID: id + ":" + fmt.Sprint(index), SourceRef: "wardrobe-preparation:" + id, ExpectedWardrobeRevision: current})
			if err != nil {
				return err
			}
			current = intValue(out["wardrobe_revision"])
			planIDs = append(planIDs, stringValue(out["outfit_id"]))
		}
		if !usableCoverage {
			return errors.New("wardrobe_starter_outfit_incomplete")
		}
		returnValue = map[string]any{"status": "awaiting_wear", "inventory_committed": true, "item_count": len(members), "plan_ids": planIDs, "reason": stringValue(proposal["reason"])}
		_, err = tx.Exec(ctx, `UPDATE public.fluctlight_wardrobe_preparations SET status='awaiting_wear',profile_id=$2,result=$3,error_code=NULL,updated_at=now() WHERE id=$1`, id, profile, jsonBytes(returnValue))
		return err
	})
	return returnValue, err
}

func (a *App) finishWardrobePreparationWear(ctx context.Context, id, fid, owner, profile string, saved map[string]any) (map[string]any, error) {
	request := ToolExecutionRequest{CapabilityName: wardrobeWearCapabilityName, OperationID: stringValue(saved["wear_operation"]), AuthorizationActorID: owner, FluctlightID: fid, WorkingProfileID: firstString(saved["wear_profile_id"], profile), EvidenceID: "wardrobe-preparation:" + id, CorrelationID: "wardrobe-preparation:" + id, Surface: CapabilitySurfaceNativeCognition, Arguments: jsonBytes(map[string]any{"mode": "full", "item_ids": saved["wear_item_ids"]})}
	request.SubjectActorID = owner
	// Replay the exact committed receipt even if its former speaking profile was
	// removed later. New effects still pass normal Tool admission and the fence.
	resource, err := a.DB.GetFluctlight(ctx, fid, owner)
	if err != nil {
		return nil, err
	}
	stored, authority, found, err := readToolExecution(ctx, a.DB.Pool(), request)
	if err != nil {
		return nil, err
	}
	var receipt ToolExecutionReceipt
	if found {
		callID := "direct_call_" + stableDigest(fid + "\x1f" + wardrobeWearCapabilityName + "\x1f" + request.OperationID)[:32]
		receipt = toolExecutionReceipt(request, callID, stored)
		receipt.Replayed = true
		receipt.AuthorityRevisions = authority
	} else {
		profiles := personalityProfileIDs(resource.CorePersona)
		_, declared := profiles[request.WorkingProfileID]
		if !declared && request.WorkingProfileID != initialPersonalityProfileID(resource.CorePersona) {
			return a.retireUnappliedWardrobeSelection(ctx, id, saved, "原选择的人格已移除，将按当前人格重新选择")
		}
		receipt, err = a.ExecuteTool(withWardrobeSelectionFence(ctx, saved), request)
	}
	if err != nil {
		return nil, err
	}
	if receipt.Result.Status != "completed" && !(receipt.Result.Status == "handled" && mapValue(receipt.Result.Output)["warning_code"] == "wearing_unchanged") {
		// Handled/no-effect receipts are durable too. Clear only their selection
		// for a later fresh context; a committed/unknown effect is never replayed
		// under a new identity.
		if receipt.Result.Status == "handled" && mapValue(receipt.Result.Output)["effect_applied"] == false && mapValue(receipt.Result.Output)["mutation_committed"] == false {
			return a.retireUnappliedWardrobeSelection(ctx, id, saved, "穿搭已准备，当前穿衣条件尚未满足")
		}
		return nil, errors.New("wardrobe_wear_not_committed")
	}
	saved = cloneMap(saved)
	saved["status"] = "completed"
	saved["wear_receipt"] = receipt
	delete(saved, "wear_operation")
	if _, err = a.DB.Pool().Exec(ctx, `UPDATE public.fluctlight_wardrobe_preparations SET status='completed',result=$2,error_code=NULL,updated_at=now() WHERE id=$1`, id, jsonBytes(saved)); err != nil {
		return nil, err
	}
	return saved, nil
}

// Only a missing receipt under this serialized preparation, or a durable
// handled/no-effect receipt, may retire a selection. Submitted effects replay.
func (a *App) retireUnappliedWardrobeSelection(ctx context.Context, id string, saved map[string]any, reason string) (map[string]any, error) {
	next := cloneMap(saved)
	next["last_unapplied_selection"] = map[string]any{"operation_id": saved["wear_operation"], "profile_id": saved["wear_profile_id"], "reason": reason}
	for _, field := range []string{"wear_operation", "wear_item_ids", "wear_profile_id", "wear_fence"} {
		delete(next, field)
	}
	if _, err := a.DB.Pool().Exec(ctx, `UPDATE public.fluctlight_wardrobe_preparations SET status='awaiting_wear',result=$2,updated_at=now() WHERE id=$1`, id, jsonBytes(next)); err != nil {
		return nil, err
	}
	return map[string]any{"status": "awaiting_wear", "reason": reason}, nil
}

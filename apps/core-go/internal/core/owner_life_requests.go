package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const ownerLifeRequestFactType = "life.owner_request.submitted"

func isOwnerLifeRequestKind(kind string) bool {
	switch kind {
	case "moment_suggestion", "media_generate", "item_photo", "wardrobe_wear", "item_use", "schedule_item_suggestion", "promise", "promise_reschedule", "inventory_borrow", "inventory_return", "inventory_remove", "inventory_outfit":
		return true
	}
	return false
}

// Serialize the idempotency read as well as the insert. Domain Life/wardrobe
// locks still protect commands from concurrent character mutations.
func (a *App) dispatchOwnerLifeRequest(ctx context.Context, method, path string, query url.Values, owner string, body map[string]any) (map[string]any, error) {
	var out map[string]any
	var business error
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "owner-life:"+owner); err != nil {
			return err
		}
		if body == nil {
			body = map[string]any{}
		}
		var storage error
		out, business, storage = NewOwnerSurface(nil, a.surfaceHooks()).dispatchTx(context.WithValue(ctx, surfaceDBTxKey{}, tx), postgresSurfaceTx{tx}, method, path, query, owner, body)
		return storage
	})
	if err != nil {
		return nil, ownerSurfaceStorageError(err)
	}
	return out, business
}

// The API only records a fact and its existing cognition workflow intent.
// Provider work is exclusively performed by the Worker, after this commits.
func (a *App) enqueueOwnerLifeRequest(ctx context.Context, owner, fid, id, kind string, body map[string]any) (string, error) {
	tx, ok := SurfaceDBTx(ctx)
	if !ok {
		return "", errors.New("owner_life_transaction_required")
	}
	if !isOwnerLifeRequestKind(kind) {
		return "", ErrInvalidArguments
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('fluctlight_lifecycle:' || $1))`, fid); err != nil {
		return "", err
	}
	if err := lockLifeContextTx(ctx, tx, fid); err != nil {
		return "", err
	}
	if err := a.validateOwnerLifeRequestTx(ctx, tx, owner, fid, kind, body); err != nil {
		return "", err
	}
	if kind == "promise" {
		if err := a.createPromiseLinkTx(ctx, tx, owner, fid, id, body); err != nil {
			return "", err
		}
	}
	return a.enqueueNativeFactTx(ctx, tx, fid, "", id, ownerLifeRequestFactType, "owner-life-request:"+id, map[string]any{"request_id": id, "owner_actor_id": owner, "kind": kind, "request": body})
}

func (a *App) validateOwnerLifeRequestTx(ctx context.Context, tx pgx.Tx, owner, fid, kind string, body map[string]any) error {
	if err := requireOwnerMutableTx(ctx, tx, owner, fid); err != nil {
		return err
	}
	reason := strings.TrimSpace(stringValue(body["reason"]))
	if kind != "moment_suggestion" && (reason == "" || len([]rune(reason)) > 500) {
		return invalid("reason must contain 1–500 characters")
	}
	if containsPermanentHarm(body) {
		return harmError()
	}
	switch kind {
	case "moment_suggestion":
		if text := strings.TrimSpace(stringValue(body["text"])); text == "" || len([]rune(text)) > 2000 || len(stringList(body["media_ids"])) > 8 {
			return invalid("Invalid Moment suggestion")
		}
		for _, id := range stringList(body["media_ids"]) {
			asset, err := a.ownerMediaAssetWith(ctx, tx, owner, stringValue(id), false)
			if err != nil {
				return err
			}
			if asset["readable"] != true {
				return invalid("Suggested media is not readable")
			}
		}
	case "media_generate", "item_photo":
		return a.validateOwnerMediaRequestTx(ctx, tx, owner, fid, kind, body)
	case "wardrobe_wear", "item_use":
		var revision int
		if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1 FOR UPDATE`, fid).Scan(&revision); err != nil {
			return err
		}
		if expected, present := body["expected_revision"]; present {
			value, ok := parseRevision(expected)
			if !ok {
				return invalid("expectedRevision is invalid")
			}
			if value != revision {
				return conflict("wardrobe", value, revision)
			}
		} else {
			body["expected_revision"] = revision
		}
		var itemKind, slot, ownership, availability string
		item := strings.TrimSpace(stringValue(body["item_id"]))
		err := tx.QueryRow(ctx, `SELECT item_kind,slot,ownership,availability FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND id=$2`, fid, item).Scan(&itemKind, &slot, &ownership, &availability)
		if errors.Is(err, pgx.ErrNoRows) {
			return &SurfaceError{Status: 404, Code: "wardrobe_item_not_found", Message: "Wardrobe item was not found"}
		}
		if err != nil {
			return err
		}
		if kind == "wardrobe_wear" && (itemKind != "wearable" || slot != stringValue(body["slot"])) {
			return invalid("item does not belong to the requested wearable slot")
		}
		if kind == "item_use" {
			if _, ok := body["active"].(bool); !ok {
				return invalid("active is required")
			}
			if itemKind != "object" {
				return invalid("ordinary item use requires an object")
			}
		}
		if kind == "wardrobe_wear" || body["active"] == true {
			if availability != "available" || (ownership != "owned" && ownership != "borrowed") {
				return &SurfaceError{Status: 409, Code: "wardrobe_item_unavailable", Message: "The requested item is not owned or available"}
			}
		}
	case "inventory_borrow", "inventory_return", "inventory_remove", "inventory_outfit":
		return a.validateOwnerInventoryRequestTx(ctx, tx, owner, fid, kind, body)
	case "schedule_item_suggestion":
		current, err := a.currentAcceptedScheduleTx(ctx, tx, fid)
		if err != nil {
			return err
		}
		expected, ok := parseRevision(body["expected_revision"])
		if !ok {
			return invalid("expectedRevision is required")
		}
		if current["id"] != body["schedule_id"] || intValue(current["revision"]) != expected {
			return conflict("schedule", expected, intValue(current["revision"]))
		}
		token := strings.TrimSpace(stringValue(body["expected_life_context_revision"]))
		if token == "" {
			return invalid("expectedLifeContextRevision is required")
		}
		if _, err := a.requireLifeContextRevisionTx(ctx, tx, fid, token, a.now().UTC()); err != nil {
			return &SurfaceError{Status: 409, Code: "life_context_revision_conflict", Message: "Life context changed; refresh the schedule"}
		}
		if err := validateLifePlaceTx(ctx, tx, fid, stringValue(body["place_id"])); err != nil {
			return err
		}
		op := stringValue(body["operation"])
		if op != "create" {
			target := scheduleItemByID(current, stringValue(body["item_id"]))
			if target == nil {
				return &SurfaceError{Status: 404, Code: "schedule_item_not_found", Message: "Schedule item was not found"}
			}
			end, _ := parseScheduleTime(stringValue(target["end_at"]))
			if !end.After(a.now()) {
				return &SurfaceError{Status: 409, Code: "schedule_edit_history_immutable", Message: "Completed schedule history cannot be edited"}
			}
		}
		if op != "cancel" {
			start, e1 := parseScheduleTime(stringValue(body["start_at"]))
			end, e2 := parseScheduleTime(stringValue(body["end_at"]))
			if e1 != nil || e2 != nil || !end.After(start) || strings.TrimSpace(stringValue(body["activity"])) == "" {
				return invalid("schedule item fields are invalid")
			}
			if start.Before(a.now()) {
				return &SurfaceError{Status: 409, Code: "schedule_edit_history_immutable", Message: "A new interval must start in the future"}
			}
		}
	case "promise":
		end, err := time.Parse(time.RFC3339Nano, stringValue(body["due_at"]))
		if strings.TrimSpace(stringValue(body["title"])) == "" || err != nil || !end.After(a.now()) {
			return invalid("promise title and future dueAt with explicit offset required")
		}
	case "promise_reschedule":
		return a.validatePromiseRescheduleTx(ctx, tx, owner, fid, body)
	}
	return nil
}

// A request-scoped view keeps unrelated publication/persona/media capabilities
// out of this run. Every mutation also validates the immutable request target
// and original CAS inside the normal Tool transaction.
type ownerLifeCapability struct {
	Capability
	app              *App
	owner, fid, kind string
	request          map[string]any
}

type ownerScheduleRequestKey struct{}

func ownerScheduleRequestFromContext(ctx context.Context) map[string]any {
	value, _ := ctx.Value(ownerScheduleRequestKey{}).(map[string]any)
	return compactStateMap(value, []string{"operation", "item_id", "start_at", "end_at", "activity", "scene", "location", "place_id"})
}

func (c ownerLifeCapability) Prepare(ctx context.Context, i CapabilityInvocation, r CapabilityContext) (CapabilityInvocation, error) {
	if c.kind == "schedule_item_suggestion" {
		ctx = context.WithValue(ctx, ownerScheduleRequestKey{}, c.request)
	}
	if p, ok := c.Capability.(CapabilityPreparer); ok {
		return p.Prepare(ctx, i, r)
	}
	// The request wrapper participates in the preparation protocol even when
	// its inner transactional Tool needs no external preparation. Freeze that
	// explicit state instead of returning an empty payload to the Runtime.
	return withCapabilityPreparedData(i, "owner_life_request", map[string]any{"state": "not_required", "kind": c.kind, "request_id": stringValue(c.request["_request_id"])})
}
func (c ownerLifeCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, i CapabilityInvocation, r CapabilityContext) (CapabilityResult, error) {
	if c.kind == "promise" || c.kind == "promise_reschedule" {
		if err := lockLifeContextTx(ctx, tx, c.fid); err != nil {
			return failedCapabilityResult(i, "promise_lock_failed", true), err
		}
	}
	if err := c.app.validateOwnerLifeRequestTx(ctx, tx, c.owner, c.fid, c.kind, c.request); err != nil {
		return failedCapabilityResult(i, "owner_life_request_stale", false), err
	}
	args := decodeObject(i.Arguments)
	switch c.kind {
	case "wardrobe_wear":
		ids := decisionServiceRefValues(args["item_ids"])
		if stringValue(args["mode"]) != "partial" || len(ids) != 1 || ids[0] != stringValue(c.request["item_id"]) || len(arrayValue(args["remove_slots"])) != 0 {
			return failedCapabilityResult(i, "owner_life_target_mismatch", false), ErrUnauthorized
		}
	case "item_use":
		op := "stop"
		if c.request["active"] == true {
			op = "start"
		}
		if args["item_id"] != c.request["item_id"] || args["operation"] != op {
			return failedCapabilityResult(i, "owner_life_target_mismatch", false), ErrUnauthorized
		}
	case "inventory_borrow", "inventory_return", "inventory_remove", "inventory_outfit":
		if !ownerInventoryArgumentsMatch(c.kind, c.request, args) {
			return failedCapabilityResult(i, "owner_life_target_mismatch", false), ErrUnauthorized
		}
	case "schedule_item_suggestion":
		raw, found, err := capabilityPreparedData(i, "schedule_plan")
		if err != nil || !found {
			return failedCapabilityResult(i, "owner_schedule_plan_missing", false), ErrInvalidArguments
		}
		plan := mapValue(raw)
		if err := validateOwnerScheduleRequestPlan(c.request, plan); err != nil {
			return failedCapabilityResult(i, "owner_schedule_target_mismatch", false), err
		}
	case "promise_reschedule":
		if args["operation"] != "update" || args["intention_id"] != c.request["intention_id"] {
			return failedCapabilityResult(i, "promise_target_mismatch", false), ErrUnauthorized
		}
		return c.app.applyPromiseRescheduleTx(ctx, tx, c.owner, c.fid, c.request, i)
	case "promise":
		ctx = context.WithValue(ctx, ownerPromiseContextKey{}, ownerPromiseScope{ID: stringValue(c.request["_request_id"]), Owner: c.owner, FluctlightID: c.fid})
		if args["operation"] != "create" || strings.TrimSpace(stringValue(args["action"])) != strings.TrimSpace(stringValue(c.request["title"])) {
			return failedCapabilityResult(i, "owner_life_target_mismatch", false), ErrUnauthorized
		}
	}
	writer, ok := c.Capability.(TransactionalCapability)
	if !ok {
		return failedCapabilityResult(i, "owner_life_writer_missing", false), ErrCapabilityNotFound
	}
	return writer.ExecuteTx(ctx, tx, i, r)
}

func validateOwnerScheduleRequestPlan(request, plan map[string]any) error {
	operation := stringValue(request["operation"])
	if operation == "cancel" {
		for _, raw := range arrayValue(plan["items"]) {
			item := mapValue(raw)
			if stringValue(item["source_item_id"]) == stringValue(request["item_id"]) {
				return ErrInvalidArguments
			}
		}
		return nil
	}
	for _, raw := range arrayValue(plan["items"]) {
		item := mapValue(raw)
		start, e1 := parseScheduleTime(stringValue(item["start_at"]))
		wantStart, e2 := parseScheduleTime(stringValue(request["start_at"]))
		end, e3 := parseScheduleTime(stringValue(item["end_at"]))
		wantEnd, e4 := parseScheduleTime(stringValue(request["end_at"]))
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil || !start.Equal(wantStart) || !end.Equal(wantEnd) {
			continue
		}
		match := true
		for _, key := range []string{"activity", "scene", "location", "place_id"} {
			if v, exists := request[key]; exists && stringValue(v) != stringValue(item[key]) {
				match = false
			}
		}
		if match {
			return nil
		}
	}
	return ErrInvalidArguments
}

func (a *App) ownerLifeRegistry(owner, fid, kind string, request map[string]any) (*CapabilityRegistry, error) {
	names := []string{capabilityCatalogName, wardrobeInspectCapabilityName, intentionInspectCapabilityName, "schedule.inspect"}
	if kind == "moment_suggestion" {
		names = append(names, "moment.publish")
	}
	switch kind {
	case "media_generate", "item_photo":
		names = append(names, "media.image.generate")
	case "wardrobe_wear":
		names = append(names, wardrobeWearCapabilityName)
	case "item_use":
		names = append(names, itemUseCapabilityName)
	case "inventory_borrow":
		names = append(names, wardrobeBorrowCapabilityName)
	case "inventory_return":
		names = append(names, wardrobeReturnCapabilityName)
	case "inventory_remove", "inventory_outfit":
		names = append(names, wardrobeWearCapabilityName)
	case "schedule_item_suggestion":
		if request["operation"] == "cancel" {
			names = append(names, scheduleEditCapabilityName)
		} else {
			names = append(names, "schedule.replan")
		}
	case "promise", "promise_reschedule":
		names = append(names, intentionDecideCapabilityName)
	}
	entries := []Capability{}
	for _, name := range names {
		entry, ok := a.capabilityRegistry().Lookup(name)
		if !ok {
			continue
		}
		if entry.Definition().Type == CapabilityTypeAction && kind != "moment_suggestion" {
			if ownerMediaRequestKind(kind) {
				entry = ownerMediaCapability{ownerLifeCapability{Capability: entry, app: a, owner: owner, fid: fid, kind: kind, request: request}}
			} else {
				entry = ownerLifeCapability{Capability: entry, app: a, owner: owner, fid: fid, kind: kind, request: request}
			}
		}
		entries = append(entries, entry)
	}
	return NewCapabilityRegistry(entries...)
}

func ownerLifeDecisionSchema() map[string]any {
	return objectSchema(map[string]any{"decision": enumStringSchema("accepted", "rejected", "deferred"), "reason": map[string]any{"type": "string", "minLength": 1, "maxLength": 500}}, []string{"decision", "reason"}, false)
}

func (a *App) runOwnerLifeRequestTask(ctx context.Context, owner, fid, inbox, kind string, request map[string]any) (ProjectionTaskResult, error) {
	registry, err := a.ownerLifeRegistry(owner, fid, kind, request)
	if err != nil {
		return ProjectionTaskResult{}, err
	}
	ctx = withCapabilityRunRegistry(ctx, registry)
	projectionRequest := ContextProjectionRequest{AuthorizationActorID: owner, SpeakerActorID: owner, TargetActorID: owner, TriggerSource: ownerLifeRequestFactType, FluctlightID: fid, SourceFactID: inbox, MemoryOperation: MemoryForNativeCognition, MemoryConversationMode: MemoryConversationGlobalOnly}
	projection, err := a.BuildContextProjectionFor(ctx, projectionRequest)
	if err != nil {
		return ProjectionTaskResult{}, err
	}
	if kind == "moment_suggestion" {
		ctx, err = a.withMomentWriterRun(ctx, ToolExecutionRequest{AuthorizationActorID: owner, SubjectActorID: owner, FluctlightID: fid, EvidenceID: inbox, Surface: CapabilitySurfaceNativeCognition, TargetKind: "owner_moment_request", TargetRef: stringValue(request["_request_id"])})
		if err != nil {
			return ProjectionTaskResult{}, err
		}
		registry = a.capabilityRegistryForContext(ctx)
	}
	rules := []string{providerContextAuthorityRule, "Assess the explicit Owner life request as the character. You may reject or defer; an Owner suggestion is not proof of agreement or execution. Use only the requested target. Inspect actual state as needed. To accept, invoke the corresponding installed Tool and consume its actual receipt. Wear uses partial with only the requested item ID; ordinary item use is separate. Inventory borrow must match the lender and all requested item descriptions exactly: an Owner suggestion is not proof that a lender agreed or a receipt occurred. Return only the exact requested IDs. Remove only the exact requested slots using partial without replacements. Outfit wear uses full with exactly its frozen member IDs and no extra removal. These are virtual-life actions, never external payment or fabricated business completion. Schedule requests must preserve exact requested interval/fields, immutable history, Intention/action_plan links, timezone and original CAS. For cancel call schedule.edit cancel for the exact item. For a promise call intention.decide create with action exactly equal to request.title, a concrete expected_outcome of at most 500 characters and a real goal_ref or meaningful goal. Core binds its exact window and promise identity; planning_requested is still provisional. For promise_reschedule call intention.decide update with exactly the requested intention_id; Core applies only the requested window. Never claim completion or proof of fulfillment. Tool effects are already committed: do not repeat them, and never claim success from prose or a plan. Final reason is a short public explanation, no private reasoning."}
	if ownerMediaRequestKind(kind) {
		rules = append(rules, "For this image request you may refuse or defer. To accept, call media.image.generate exactly once with intent equal to request._render_intent, including its exact source text. Do not invent reference asset IDs. If request.reference_asset is present, Core freezes it; do not put it in Tool arguments. A person reference may depict a fictional other person and cannot certify self identity. It creates a durable image task, never claims pixels are ready. The server binds this Owner request as its target; no conversation or Moment is created. Requested image scenes are imagined and do not move the character; an item image does not acquire, wear or use that item.")
	}
	if kind == "moment_suggestion" {
		rules = append(rules, "This request only suggests publishing a feed Moment. You may reject or defer. To accept call moment.publish once; the independent persona Writer decides its own text. Suggested text is a topic, never a factual claim or a mandatory draft. Core binds the request and selected attachments. It does not create a conversation, move anyone or establish acquaintance.")
	}
	rules = append(rules, "The final response has exactly decision (accepted, rejected, or deferred) and reason (a nonempty short public explanation). Do not emit event_type, action_status, effects, or mutation_committed; Core derives execution state from real Tool receipts, not from the final response.")
	input := jsonString(map[string]any{"event_type": ownerLifeRequestFactType, "kind": kind, "request": ownerLifePromptView(kind, request)})
	definitions := capabilityCatalog(registry, CapabilitySurfaceNativeCognition)
	schema := ownerLifeDecisionSchema()
	assembly, projection, err := a.assembleProjectionPromptForSurface(ctx, ProviderContextSurfaceNativeCognition, projection, "cognitive_assessment", rules, input, definitions, "owner_life_request_decision", schema)
	if err != nil {
		return ProjectionTaskResult{}, err
	}
	ctx = a.bindProjectionRefresh(ctx, projectionRequest, ProviderContextSurfaceNativeCognition, "cognitive_assessment", rules, input, definitions, "owner_life_request_decision", schema, nil)
	scope := formalTaskCapabilityRequest(FormalAgentNativeCognition, projection, CapabilitySurfaceNativeCognition)
	if ownerMediaRequestKind(kind) {
		scope.TargetKind = "owner_media_request"
		scope.TargetRef = stringValue(request["_request_id"])
	}
	if kind == "moment_suggestion" {
		scope.TargetKind = "owner_moment_request"
		scope.TargetRef = stringValue(request["_request_id"])
	}
	scope.AuthorizationPolicy = "" // Exact authenticated Owner request, not unrelated autonomous work.
	run, err := a.runFormalStructuredTask(withUpstreamModelRetry(ctx), FormalAgentNativeCognition, assembly.Messages, definitions, "owner_life_request_decision", schema, true, scope)
	return ProjectionTaskResult{Trace: run.Trace, Completion: run.Completion, Projection: projection, Diagnostics: assembly.Diagnostics}, err
}

// Read the operation ledger before any attempt to run the model. Recovery of a
// committed effect must never restart the Agent because its final JSON failed.
func (a *App) ownerLifeCommittedResults(ctx context.Context, owner, fid, inbox string) ([]CapabilityResult, error) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT result FROM public.tool_executions WHERE authorization_actor_id=$1 AND fluctlight_id=$2 AND invocation->>'source_fact_id'=$3 ORDER BY committed_at,operation_id`, owner, fid, inbox)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := []CapabilityResult{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var result CapabilityResult
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, rows.Err()
}

func ownerLifeActionNames(kind string) []string {
	return map[string][]string{"moment_suggestion": {"moment.publish"}, "media_generate": {"media.image.generate"}, "item_photo": {"media.image.generate"}, "inventory_borrow": {wardrobeBorrowCapabilityName}, "inventory_return": {wardrobeReturnCapabilityName}, "inventory_remove": {wardrobeWearCapabilityName}, "inventory_outfit": {wardrobeWearCapabilityName}, "wardrobe_wear": {wardrobeWearCapabilityName}, "item_use": {itemUseCapabilityName}, "schedule_item_suggestion": {"schedule.replan", scheduleEditCapabilityName}, "promise": {intentionDecideCapabilityName}, "promise_reschedule": {intentionDecideCapabilityName}}[kind]
}

func ownerLifeEffects(kind string, results []CapabilityResult) (bool, []any) {
	expected := ownerLifeActionNames(kind)
	refs := []any{}
	for _, result := range results {
		if !containsString(expected, result.CapabilityName) || (result.Status != "completed" && !(ownerMediaRequestKind(kind) && result.Status == "accepted")) {
			continue
		}
		output := mapValue(result.Output)
		if ownerMediaRequestKind(kind) && stringValue(output["media_intent_id"]) == "" {
			continue
		}
		if kind == "moment_suggestion" && (output["target_kind"] != "moment" || stringValue(output["target_ref"]) == "") {
			continue
		}
		// A planning request is provisional; only an actual Intention is accepted.
		if kind == "promise" && stringValue(output["intention_id"]) == "" {
			continue
		}
		refs = append(refs, result.CallID)
		for _, raw := range arrayValue(output["items"]) {
			if id := stringValue(mapValue(raw)["item_id"]); id != "" {
				refs = append(refs, id)
			}
		}
		for _, key := range []string{"event_id", "schedule_id", "intention_id", "media_intent_id", "target_ref"} {
			if id := stringValue(output[key]); id != "" {
				refs = append(refs, id)
			}
		}
	}
	return len(refs) > 0, refs
}

func ownerLifeSatisfiedNoChange(kind string, results []CapabilityResult) (bool, []any) {
	codes := map[string]bool{"wearing_unchanged": true, "item_use_unchanged": true, "item_already_inactive": true, "wardrobe_return_already_recorded": true}
	for _, result := range results {
		output := mapValue(result.Output)
		if containsString(ownerLifeActionNames(kind), result.CapabilityName) && result.Status == "handled" && output["disposition"] == "no_change" && codes[stringValue(output["warning_code"])] {
			return true, []any{result.CallID}
		}
	}
	return false, nil
}
func ownerLifeResultDetails(kind string, results []CapabilityResult) []any {
	details := []any{}
	for _, result := range results {
		if containsString(ownerLifeActionNames(kind), result.CapabilityName) && (result.Status == "completed" || result.Status == "handled" || (ownerMediaRequestKind(kind) && result.Status == "accepted")) {
			details = append(details, map[string]any{"capability_name": result.CapabilityName, "status": result.Status, "call_id": result.CallID, "output": result.Output})
		}
	}
	return details
}

func (a *App) processOwnerLifeRequest(ctx context.Context, inbox string, fact map[string]any) (map[string]any, error) {
	candidate := mapValue(fact["candidate"])
	id, owner, kind := stringValue(candidate["request_id"]), stringValue(candidate["owner_actor_id"]), stringValue(candidate["kind"])
	fid := stringValue(fact["fluctlight_id"])
	if id == "" || owner == "" || !isOwnerLifeRequestKind(kind) {
		return nil, ErrInvalidArguments
	}
	var raw []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT payload FROM public.owner_surface_records WHERE id=$1 AND owner_actor_id=$2 AND fluctlight_id=$3 AND kind='owner_request'`, id, owner, fid).Scan(&raw); err != nil {
		return nil, err
	}
	requestRecord := decodeObject(raw)
	if requestRecord["source_fact_id"] != inbox || requestRecord["kind"] != kind {
		return nil, ErrUnauthorized
	}
	if requestRecord["status"] != "pending" {
		result := mapValue(requestRecord["result"])
		err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error { return settleOwnerLifeInboxTx(ctx, tx, inbox, result) })
		return result, err
	}
	request := cloneMap(mapValue(requestRecord["payload"]))
	request["_request_id"] = id
	results, err := a.ownerLifeCommittedResults(ctx, owner, fid, inbox)
	if err != nil {
		return nil, err
	}
	effect, refs := ownerLifeEffects(kind, results)
	satisfied, noChangeRefs := ownerLifeSatisfiedNoChange(kind, results)
	decision := map[string]any{}
	var runErr error
	planning := ownerPromisePlanningPending(kind, results)
	if !effect && !satisfied && !planning {
		task, err := a.runOwnerLifeRequestTask(ctx, owner, fid, inbox, kind, request)
		runErr = err
		if err != nil {
			a.recordDiagnosticEvent(ctx, "owner.life.request.assessment_failed", "error", fid, id, "owner-life:"+inbox, map[string]any{"error_code": "owner_life_assessment_failed", "kind": kind, "safe_cause": boundedLifecycleCause(err.Error())})
		}
		decision = task.Completion.Structured
		// Durable receipts are authoritative even when the Agent/model fails after
		// committing a mutation. Queries do not count. Only explicit, target-scoped no-change receipts may settle an already satisfied request without asserting a mutation.
		results, err = a.ownerLifeCommittedResults(context.WithoutCancel(ctx), owner, fid, inbox)
		if err != nil {
			return nil, err
		}
		effect, refs = ownerLifeEffects(kind, results)
		satisfied, noChangeRefs = ownerLifeSatisfiedNoChange(kind, results)
		planning = ownerPromisePlanningPending(kind, results)
	}
	status, code, reason := "failed", "owner_life_request_not_applied", "The request did not produce a committed result"
	switch {
	case effect:
		status, code, reason = "completed", "", "The requested change was committed"
		if ownerMediaRequestKind(kind) {
			status, reason = "accepted", "图片生成任务已排队；图片尚未完成，实际结果需另行查询。"
		}
		if kind == "promise" {
			status, reason = "accepted", "An intention was recorded; fulfillment requires later evidence"
		}
	case planning:
		status, code, reason = "awaiting_goal", "", "The proposed action awaits explicit Planner goal binding; no accepted promise yet"
	case satisfied:
		status, code, reason = "completed", "", "The exact requested target state was already recorded; no new mutation was needed"
		refs = noChangeRefs
	case runErr != nil:
		code = "owner_life_assessment_failed"
		reason = ownerRequestFailureReason(runErr)
	case decision["decision"] == "rejected":
		status, code, reason = "rejected", "owner_life_request_declined", stringValue(decision["reason"])
	case decision["decision"] == "deferred":
		status, code, reason = "deferred", "owner_life_request_deferred", stringValue(decision["reason"])
	}
	disposition := "not_applied"
	if effect {
		disposition = "committed"
	} else if satisfied {
		disposition = "no_change"
	}
	result := map[string]any{"effects": ownerLifeResultDetails(kind, results), "disposition": disposition, "request_id": id, "status": status, "error_code": nilIfEmpty(code), "source_fact_id": inbox, "result_refs": refs, "mutation_committed": effect, "reason": reason}
	// Cancellation may interrupt the model, but already committed effects must
	// still become visible. This bounded cleanup cannot perform another effect.
	settleCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	err = withTransaction(settleCtx, a.DB.Pool(), func(tx pgx.Tx) error {
		var payload []byte
		if err := tx.QueryRow(settleCtx, `SELECT payload FROM public.owner_surface_records WHERE id=$1 AND owner_actor_id=$2 AND fluctlight_id=$3 FOR UPDATE`, id, owner, fid).Scan(&payload); err != nil {
			return err
		}
		record := decodeObject(payload)
		if record["status"] != "pending" {
			result = mapValue(record["result"])
			return settleOwnerLifeInboxTx(settleCtx, tx, inbox, result)
		}
		record["status"], record["result"], record["result_refs"], record["decided_by"], record["decision_reason"], record["updated_at"] = status, result, refs, "agent", reason, a.now().UTC().Format(time.RFC3339Nano)
		record["revision"] = intValue(record["revision"]) + 1
		if _, err := tx.Exec(settleCtx, `UPDATE public.owner_surface_records SET status=$3,payload=$4,revision=revision+1,updated_at=now() WHERE id=$1 AND owner_actor_id=$2`, id, owner, status, jsonBytes(record)); err != nil {
			return err
		}
		if err := settleOwnerLifeInboxTx(settleCtx, tx, inbox, result); err != nil {
			return err
		}
		return appendOwnerRequestNotificationTx(settleCtx, tx, fid, id, kind, status, inbox, reason)
	})
	return result, err
}

// User-visible categories only; raw provider/SQL errors remain in diagnostics.
func ownerRequestFailureReason(err error) string {
	text := err.Error()
	switch {
	case strings.Contains(text, "524"), strings.Contains(text, "gateway_timeout"):
		return "模型中转服务等待超时，请检查模型通道后重新提出请求。"
	case strings.Contains(text, "budget_exceeded"):
		return "模型服务未能处理本次资料，请查看本次运行记录。原资料和已完成步骤仍保留。"
	case errors.Is(err, errImageProviderNotConfigured):
		return "图片服务尚未配置，请先到图片（ComfyUI）配置连接，再恢复这一环节。"
	case strings.Contains(text, "wardrobe_initial_"), strings.Contains(text, "wardrobe_starter_"), strings.Contains(text, "wardrobe_wear_selection_"), strings.Contains(text, "wardrobe_select_only_"):
		return "初始穿搭还没有形成完整、可执行的方案。可以补充穿衣偏好或登记可用衣物，再恢复这一环节。"
	case errors.Is(err, context.DeadlineExceeded), strings.Contains(text, "timeout"), strings.Contains(text, "deadline exceeded"):
		return "模型服务响应超时，请检查服务后重试。"
	case errors.Is(err, context.Canceled):
		return "本次处理已中断，请刷新进度核对结果。"
	case strings.Contains(text, "adk_final_"), strings.Contains(text, "structured_response_invalid"):
		return "模型返回的内容格式不完整，原资料已保留。可以重新整理，或查看本次运行定位问题。"
	case errors.Is(err, ErrConflict):
		return "处理期间资料发生了变化，请刷新后重试。"
	default:
		return "本次请求未能完成判断，请查看关联诊断并检查模型服务。"
	}
}

func settleOwnerLifeInboxTx(ctx context.Context, tx pgx.Tx, inbox string, result map[string]any) error {
	if _, err := tx.Exec(ctx, `UPDATE public.cognition_inbox SET status='processed',processed_at=COALESCE(processed_at,now()),claimed_by=NULL,claimed_at=NULL,error_code=NULL,payload=jsonb_set(payload,'{agent_result}',$2::jsonb,true) WHERE id=$1`, inbox, jsonBytes(result)); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE public.cognition_inbox_heads h SET last_processed_sequence=GREATEST(h.last_processed_sequence,i.sequence) FROM public.cognition_inbox i WHERE h.fluctlight_id=i.fluctlight_id AND i.id=$1`, inbox)
	return err
}

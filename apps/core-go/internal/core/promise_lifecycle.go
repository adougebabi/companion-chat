package core

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type ownerPromiseContextKey struct{}
type ownerPromiseScope struct{ ID, Owner, FluctlightID string }

func promiseScope(ctx context.Context) (ownerPromiseScope, bool) {
	scope, ok := ctx.Value(ownerPromiseContextKey{}).(ownerPromiseScope)
	return scope, ok
}

func (a *App) createPromiseLinkTx(ctx context.Context, tx pgx.Tx, owner, fid, id string, body map[string]any) error {
	for key := range body {
		if key != "title" && key != "due_at" && key != "starts_at" && key != "reason" && key != "idempotency_key" {
			return invalid("unsupported promise proposal field")
		}
	}
	title := strings.TrimSpace(stringValue(body["title"]))
	end, e := time.Parse(time.RFC3339Nano, stringValue(body["due_at"]))
	start := a.now().UTC()
	if raw := stringValue(body["starts_at"]); raw != "" {
		var err error
		start, err = time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return invalid("startsAt requires an explicit timezone offset")
		}
	}
	if title == "" || len([]rune(title)) > 2000 || e != nil || !end.After(start) || !end.After(a.now()) {
		return invalid("promise title and future fulfillment window required")
	}
	zone, err := readLifeContextTimezoneWith(ctx, tx, fid)
	if err != nil {
		return err
	}
	profile, err := (&intentionService{}).resolveProfile(ctx, tx, fid, "")
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.owner_promise_links(request_id,owner_actor_id,fluctlight_id,profile_id,title,window_start,window_end,timezone) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id, owner, fid, profile, title, start, end, zone)
	if err != nil {
		return err
	}
	return appendPromiseHistoryTx(ctx, tx, id, "proposed", id, stringValue(body["reason"]), nil, a.now())
}
func readPromiseLinkTx(ctx context.Context, q lifeContextQuerier, owner, fid, id string, lock bool) (map[string]any, error) {
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	var raw []byte
	err := q.QueryRow(ctx, `SELECT to_jsonb(p) FROM public.owner_promise_links p WHERE owner_actor_id=$1 AND fluctlight_id=$2 AND request_id=$3`+suffix, owner, fid, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return decodeObject(raw), nil
}
func appendPromiseHistoryTx(ctx context.Context, tx pgx.Tx, id, op, source, reason string, before map[string]any, at time.Time) error {
	_, err := tx.Exec(ctx, `INSERT INTO public.owner_promise_history(id,request_id,fluctlight_id,revision,operation,source_ref,reason,before_value,after_value,occurred_at) SELECT $2,p.request_id,p.fluctlight_id,p.revision,$3,$4,$5,$6,to_jsonb(p),$7 FROM public.owner_promise_links p WHERE request_id=$1`, id, randomID("promise_history_"), op, source, reason, jsonBytes(before), at)
	return err
}

// Called only inside the actual intention Tool transaction after the character
// selected this action. Metadata comes from the trusted request, never model ids.
func preparePromiseIntentionTx(ctx context.Context, tx pgx.Tx, scope ownerPromiseScope, args map[string]any, profile string, at time.Time) (map[string]any, error) {
	p, err := readPromiseLinkTx(ctx, tx, scope.Owner, scope.FluctlightID, scope.ID, true)
	if err != nil {
		return nil, err
	}
	end, _ := time.Parse(time.RFC3339Nano, stringValue(p["window_end"]))
	active, err := (&intentionService{}).resolveProfile(ctx, tx, scope.FluctlightID, "")
	if err != nil {
		return nil, err
	}
	if p["cancelled_at"] != nil || !end.After(at) || p["profile_id"] != active || (profile != "" && p["profile_id"] != profile) {
		return nil, newCapabilityError("promise_scope_or_window_changed", false, ErrConflict)
	}
	if stringValue(p["intention_id"]) != "" {
		return nil, newCapabilityError("promise_already_accepted", false, ErrConflict)
	}
	if strings.TrimSpace(stringValue(args["action"])) != p["title"] || strings.TrimSpace(stringValue(args["expected_outcome"])) == "" || len([]rune(stringValue(args["expected_outcome"]))) > 500 {
		return nil, newCapabilityError("promise_target_mismatch", false, ErrInvalidArguments)
	}
	return p, nil
}
func requestPromisePlanningTx(ctx context.Context, tx pgx.Tx, scope ownerPromiseScope, args map[string]any, profile, reason string, at time.Time) (map[string]any, error) {
	p, err := preparePromiseIntentionTx(ctx, tx, scope, args, profile, at)
	if err != nil {
		return nil, err
	}
	if stringValue(p["planning_source_key"]) != "" {
		if p["proposed_goal"] != args["goal"] || p["proposed_action"] != args["action"] || p["expected_result"] != args["expected_outcome"] {
			return nil, ErrConflict
		}
		return map[string]any{"status": "planning_requested", "promise_request_id": scope.ID, "planning_source_key": p["planning_source_key"], "activated": false, "intention_created": false}, nil
	}
	key := "owner:promise:" + scope.ID
	_, err = tx.Exec(ctx, `UPDATE public.owner_promise_links SET proposed_goal=$2,proposed_action=$3,expected_result=$4,planning_source_key=$5,revision=revision+1,updated_at=now() WHERE request_id=$1`, scope.ID, args["goal"], args["action"], args["expected_outcome"], key)
	if err != nil {
		return nil, err
	}
	if err := requestGoalPlanningTx(ctx, tx, scope.FluctlightID, key, "owner_request", map[string]any{"promise_request_id": scope.ID, "desired_outcome": args["goal"], "action": args["action"], "success_criteria": []string{stringValue(args["expected_outcome"])}, "motivation": reason, "window_start": p["window_start"], "window_end": p["window_end"], "profile_id": profile}); err != nil {
		return nil, err
	}
	if err := appendPromiseHistoryTx(ctx, tx, scope.ID, "planning_requested", key, reason, p, at); err != nil {
		return nil, err
	}
	return map[string]any{"status": "planning_requested", "promise_request_id": scope.ID, "planning_source_key": key, "activated": false, "intention_created": false}, nil
}
func createPromiseCommitmentTx(ctx context.Context, tx pgx.Tx, scope ownerPromiseScope, parent GoalAuthority, action, expected, source string, at time.Time) (string, map[string]any, error) {
	p, err := readPromiseLinkTx(ctx, tx, scope.Owner, scope.FluctlightID, scope.ID, true)
	if err != nil {
		return "", nil, err
	}
	start, e1 := time.Parse(time.RFC3339Nano, stringValue(p["window_start"]))
	end, e2 := time.Parse(time.RFC3339Nano, stringValue(p["window_end"]))
	if e1 != nil || e2 != nil || !end.After(at) || p["cancelled_at"] != nil || parent.Status != GoalActive || (parent.ProfileID != "" && parent.ProfileID != p["profile_id"]) {
		return "", nil, newCapabilityError("promise_goal_or_window_unavailable", false, ErrConflict)
	}
	if action != p["title"] || expected == "" || stringValue(p["intention_id"]) != "" {
		return "", nil, newCapabilityError("promise_target_mismatch", false, ErrConflict)
	}
	key := "owner-promise:" + scope.ID
	plan := GoalCommitmentPlan{OwnerPromiseID: scope.ID, Operation: "create", ExpectedResult: expected, Criteria: []string{expected}, WindowStart: &start, WindowEnd: &end, OpportunityCondition: "Owner promise " + scope.ID, Reason: "Character accepted the exact Owner proposal"}
	if err := applyGoalCommitmentPlanTx(ctx, tx, parent, plan, key, at); err != nil {
		return "", nil, err
	}
	return "commitment_" + stableDigest(key+"\x1f"+parent.EntityID), p, nil
}
func finishPromiseAcceptanceTx(ctx context.Context, tx pgx.Tx, scope ownerPromiseScope, parent GoalAuthority, intentionID, commitmentID, action, expected, source string, before map[string]any, at time.Time) error {
	_, err := tx.Exec(ctx, `UPDATE public.owner_promise_links SET goal_id=$2,commitment_id=$3,intention_id=$4,proposed_goal=$5,proposed_action=$6,expected_result=$7,accepted_at=$8,revision=revision+1,updated_at=now() WHERE request_id=$1`, scope.ID, parent.EntityID, commitmentID, intentionID, parent.DesiredOutcome, action, expected, at)
	if err != nil {
		return err
	}
	return appendPromiseHistoryTx(ctx, tx, scope.ID, "accepted", source, "Intention and evidence-evaluated Commitment recorded", before, at)
}

type GoalPromiseBinding struct {
	RequestID        string `json:"request_id"`
	GoalID           string `json:"goal_id"`
	ExpectedRevision int    `json:"expected_revision"`
}

// Planner chooses the real goal explicitly; no title matching or invented Goal
// completion. Continuation reuses the already chosen action/outcome and commits
// all links with the Planner receipt, so final-output loss needs no new model.
func (a *App) bindPlannerPromiseTx(ctx context.Context, tx pgx.Tx, owner, fid, profile, run string, b GoalPromiseBinding) (map[string]any, error) {
	p, err := readPromiseLinkTx(ctx, tx, owner, fid, b.RequestID, true)
	if err != nil {
		return nil, err
	}
	if intValue(p["revision"]) != b.ExpectedRevision || p["cancelled_at"] != nil || stringValue(p["planning_source_key"]) == "" || p["profile_id"] != profile {
		return nil, newCapabilityError("promise_binding_stale", false, ErrConflict)
	}
	if run != "" {
		var captured bool
		err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.goal_planning_events e JOIN public.goal_planning_runs r ON r.fluctlight_id=e.fluctlight_id AND r.id=$3 WHERE e.fluctlight_id=$1 AND e.source_key=$2 AND e.seq<=r.watermark AND e.processed_run_id IS NULL)`, fid, p["planning_source_key"], run).Scan(&captured)
		if err != nil {
			return nil, err
		}
		if !captured {
			return nil, ErrUnauthorized
		}
	}
	var revision int
	if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND id=$2`, fid, b.GoalID).Scan(&revision); err != nil {
		return nil, err
	}
	parent, err := loadGoalAuthorityTx(ctx, tx, fid, "goal:ctx_"+stableDigest(b.GoalID), ContextReference{EntityID: b.GoalID, Revision: revision})
	if err != nil {
		return nil, err
	}
	scope := ownerPromiseScope{ID: b.RequestID, Owner: owner, FluctlightID: fid}
	at := a.now().UTC()
	source := "planner:" + run
	commitment, before, err := createPromiseCommitmentTx(ctx, tx, scope, parent, stringValue(p["proposed_action"]), stringValue(p["expected_result"]), source, at)
	if err != nil {
		return nil, err
	}
	start, _ := time.Parse(time.RFC3339Nano, stringValue(p["window_start"]))
	end, _ := time.Parse(time.RFC3339Nano, stringValue(p["window_end"]))
	id := "intention_promise_" + stableDigest(b.RequestID)
	intention := IntentionAuthority{EntityID: id, SchemaVersion: intentionAuthoritySchemaVersion, Ref: "intention:ctx_" + stableDigest(id), FluctlightID: fid, ProfileID: parent.ProfileID, GoalEntityID: parent.EntityID, GoalRef: parent.Ref, StageID: "", CommitmentID: commitment, ActionIntent: stringValue(p["proposed_action"]), ExpectedOutcome: stringValue(p["expected_result"]), Expiration: end, PreferredTime: &start, Trigger: TypedIntentionTrigger{Type: IntentionTriggerSemantic}, Status: IntentionCandidate, Confidence: 0.8, Revision: 1, EvidenceRefs: []string{b.RequestID, source}}
	created, record, err := CreateIntentionAuthority(intention, intention.EvidenceRefs, at)
	if err != nil {
		return nil, err
	}
	if _, err := persistIntentionAuthorityTx(ctx, tx, nil, created, record, "promise-bind:"+b.RequestID); err != nil {
		return nil, err
	}
	if err := syncIntentionTriggerWorkflowTx(ctx, tx, created); err != nil {
		return nil, err
	}
	if err := finishPromiseAcceptanceTx(ctx, tx, scope, parent, id, commitment, intention.ActionIntent, intention.ExpectedOutcome, source, before, at); err != nil {
		return nil, err
	}
	result := map[string]any{"request_id": b.RequestID, "status": "accepted", "goal_id": parent.EntityID, "commitment_id": commitment, "intention_id": id, "planner_run_id": run, "reason": "Planner bound the prior proposal to a real goal; fulfillment requires evidence"}
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT payload FROM public.owner_surface_records WHERE id=$1 AND owner_actor_id=$2 AND fluctlight_id=$3 FOR UPDATE`, b.RequestID, owner, fid).Scan(&raw); err != nil {
		return nil, err
	}
	root := decodeObject(raw)
	root["status"], root["result"], root["decision_reason"], root["decided_by"], root["updated_at"], root["revision"] = "accepted", result, result["reason"], "agent", at.Format(time.RFC3339Nano), intValue(root["revision"])+1
	if _, err := tx.Exec(ctx, `UPDATE public.owner_surface_records SET status='accepted',payload=$2,revision=revision+1,updated_at=now() WHERE id=$1`, b.RequestID, jsonBytes(root)); err != nil {
		return nil, err
	}
	if err := appendOwnerRequestNotificationTx(ctx, tx, fid, b.RequestID, "promise", "accepted", source); err != nil {
		return nil, err
	}
	return result, nil
}

const promiseProjectionSQL = `CASE WHEN p.request_id IS NULL THEN jsonb_build_object('request_id',r.id,'fluctlight_id',r.fluctlight_id,'title',r.payload->'payload'->>'title','request',r.payload,'legacy_unlinked',true) ELSE to_jsonb(p)||jsonb_build_object('request',r.payload,'commitment',(SELECT to_jsonb(c) FROM public.goal_commitments c WHERE c.id=p.commitment_id AND c.fluctlight_id=p.fluctlight_id),'intention',(SELECT to_jsonb(i) FROM public.fluctlight_intentions i WHERE i.id=p.intention_id AND i.fluctlight_id=p.fluctlight_id),'evaluations',COALESCE((SELECT jsonb_agg(to_jsonb(e) ORDER BY e.occurred_at,e.id) FROM public.goal_evaluations e WHERE e.fluctlight_id=p.fluctlight_id AND e.commitment_id=p.commitment_id),'[]'::jsonb),'planning',COALESCE((SELECT jsonb_agg(jsonb_build_object('event_id',e.seq,'processed_run_id',e.processed_run_id,'task_id',pr.id,'run_status',pr.status,'error_code',pr.error_code,'result',pr.result)) FROM public.goal_planning_events e LEFT JOIN LATERAL (SELECT r.* FROM public.goal_planning_runs r WHERE r.fluctlight_id=e.fluctlight_id AND (r.id=e.processed_run_id OR (e.processed_run_id IS NULL AND r.watermark>=e.seq AND (r.profile_id='' OR r.profile_id=p.profile_id))) ORDER BY r.created_at DESC,r.id DESC LIMIT 1) pr ON true WHERE e.fluctlight_id=p.fluctlight_id AND e.source_key=p.planning_source_key),'[]'::jsonb)) END`

func projectPromiseLifecycle(p map[string]any, at time.Time) map[string]any {
	status := stringValue(mapValue(p["request"])["status"])
	commitment := mapValue(p["commitment"])
	switch {
	case p["legacy_unlinked"] == true:
		status = "legacy_unlinked"
	case commitment["status"] == "completed":
		status = "fulfilled"
	case p["cancelled_at"] != nil || commitment["status"] == "cancelled":
		status = "cancelled"
	case stringValue(p["commitment_id"]) != "":
		end, _ := time.Parse(time.RFC3339Nano, stringValue(p["window_end"]))
		if !end.After(at) {
			status = "missed"
		} else if commitment["status"] == "paused" || commitment["status"] == "blocked" {
			status = stringValue(commitment["status"])
		} else {
			status = "accepted"
		}
	case stringValue(p["planning_source_key"]) != "":
		status = "awaiting_goal"
	}
	p["lifecycle"], p["fulfillment_authority"] = status, "goal_commitment_evaluation"
	return p
}
func (a *App) OwnerPromises(ctx context.Context, owner, fid string, q url.Values) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	limit, err := reflectionReadPage(q)
	if err != nil {
		return nil, err
	}
	scope := stableDigest(jsonString([]string{"promises", owner, fid}))
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil || (q.Get("cursor") != "" && (cursor.ID == "" || cursor.Time.IsZero())) {
		return nil, invalid("invalid promise cursor")
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT r.id,r.created_at,`+promiseProjectionSQL+` FROM public.owner_surface_records r LEFT JOIN public.owner_promise_links p ON p.request_id=r.id WHERE r.owner_actor_id=$1 AND r.fluctlight_id=$2 AND r.kind='owner_request' AND r.payload->>'kind'='promise' AND ($3='' OR (r.created_at,r.id)<($4,$3)) ORDER BY r.created_at DESC,r.id DESC LIMIT $5`, owner, fid, cursor.ID, cursor.Time, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	page, err := readGovernanceRows(rows, scope, limit)
	if err != nil {
		return nil, err
	}
	for _, raw := range arrayValue(page["items"]) {
		projectPromiseLifecycle(mapValue(raw), a.now())
	}
	return page, nil
}
func (a *App) OwnerPromise(ctx context.Context, owner, fid, id string) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	var raw []byte
	err := a.DB.Pool().QueryRow(ctx, `SELECT `+promiseProjectionSQL+` FROM public.owner_surface_records r LEFT JOIN public.owner_promise_links p ON p.request_id=r.id WHERE r.owner_actor_id=$1 AND r.fluctlight_id=$2 AND r.id=$3 AND r.kind='owner_request' AND r.payload->>'kind'='promise'`, owner, fid, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return projectPromiseLifecycle(decodeObject(raw), a.now()), nil
}
func (a *App) OwnerPromiseHistory(ctx context.Context, owner, fid, id string, q url.Values) (map[string]any, error) {
	if _, err := a.OwnerPromise(ctx, owner, fid, id); err != nil {
		return nil, err
	}
	limit, err := reflectionReadPage(q)
	if err != nil {
		return nil, err
	}
	scope := stableDigest(jsonString([]string{"promise-history", owner, fid, id}))
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil || (q.Get("cursor") != "" && (cursor.ID == "" || cursor.Time.IsZero())) {
		return nil, invalid("invalid promise history cursor")
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT h.id,h.occurred_at,to_jsonb(h) FROM public.owner_promise_history h WHERE fluctlight_id=$1 AND request_id=$2 AND ($3='' OR (occurred_at,id)<($4,$3)) ORDER BY occurred_at DESC,id DESC LIMIT $5`, fid, id, cursor.ID, cursor.Time, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return readGovernanceRows(rows, scope, limit)
}

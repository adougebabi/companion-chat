package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const CapabilitySurfaceGoalPlanner CapabilitySurface = "goal_planner"
const goalPlannerQuery = "goal_planner.query"
const goalPlannerCommit = "goal_planner.commit"

type goalPlannerCapability struct {
	service goalPlannerToolService
	name    string
}

func goalPlannerCapabilities(a *App) []Capability {
	return []Capability{goalPlannerQueryCapability{a}, goalPlannerCapability{a, goalPlannerCommit}}
}

// The query implementation intentionally has no transactional/direct mutation seam.
type goalPlannerToolService interface {
	executeGoalPlannerQuery(context.Context, CapabilityInvocation) (CapabilityResult, error)
	executeGoalPlannerCommitTx(context.Context, pgx.Tx, CapabilityInvocation, DirectToolTarget) (CapabilityResult, error)
}
type goalPlannerQueryCapability struct{ service goalPlannerToolService }

func (c goalPlannerQueryCapability) Definition() CapabilityDefinition {
	return (goalPlannerCapability{nil, goalPlannerQuery}).Definition()
}
func (c goalPlannerQueryCapability) RequiredContext() []ContextSlot { return nil }
func (c goalPlannerQueryCapability) Execute(ctx context.Context, inv CapabilityInvocation, cc CapabilityContext) (CapabilityResult, error) {
	if c.service == nil {
		return failedCapabilityResult(inv, "planner_unavailable", true), errors.New("planner unavailable")
	}
	return c.service.executeGoalPlannerQuery(ctx, inv)
}

func (c goalPlannerCapability) Definition() CapabilityDefinition {
	kind := CapabilityTypeQuery
	schema := objectSchema(map[string]any{"section": enumStringSchema("snapshot", "actor", "history", "persona", "life", "triggers", "goal"), "actor_id": stringSchema(), "goal_id": stringSchema(), "query": stringSchema(), "cursor": integerSchema()}, []string{"section"}, false)
	side, boundary, concurrency := "read_only", "query_result_available", "parallel"
	if c.name == goalPlannerCommit {
		kind = CapabilityTypeAction
		side, boundary, concurrency = "native_projection", "goal_set_committed", "exclusive"
		schema = objectSchema(map[string]any{"expected_version": integerSchema(), "expected_facts_revision": stringSchema(), "idempotency_key": stringSchema(), "reason": stringSchema(), "changes": map[string]any{"type": "array", "maxItems": 20, "items": objectSchema(map[string]any{"id": stringSchema(), "operation": enumStringSchema("create", "pause", "resume"), "expected_revision": integerSchema(), "desired_outcome": stringSchema(), "success_criteria": arraySchema(stringSchema()), "motivation": stringSchema(), "target_actor_id": stringSchema(), "activate": booleanSchema(), "source": openObjectSchema(), "scope": enumStringSchema("general", "personal", "relationship"), "deadline": stringSchema(), "deadline_policy": enumStringSchema("soft", "hard")}, []string{"id", "operation"}, false)}, "dependencies": openObjectSchema(), "order": arraySchema(stringSchema()), "reviewed_goal_ids": arraySchema(stringSchema()), "decision": stringSchema()}, []string{"expected_version", "expected_facts_revision", "idempotency_key", "reason"}, false)
	}
	return CapabilityDefinition{Name: c.name, Version: "v1", Type: kind, Description: map[bool]string{true: "Read authoritative planning snapshot, relevant Actor facts/relationships, stable persona or bounded history. Use actual results before deciding; history query/cursor is required for duplicate comparison.", false: "Atomically apply the goal set using expected snapshot versions and trusted run permission. No completion, evidence, external actions or relationship editing. Read snapshot after submission."}[kind == CapabilityTypeQuery], Surfaces: []CapabilitySurface{CapabilitySurfaceGoalPlanner}, FailurePolicy: FailurePolicyOptionalInternal, InputSchema: schema, OutputSchema: openObjectSchema(), SideEffectClass: side, SuccessBoundary: boundary, ConcurrencyClass: concurrency, SupportsRetry: true}
}
func (c goalPlannerCapability) RequiredContext() []ContextSlot { return nil }
func (c goalPlannerCapability) Execute(_ context.Context, inv CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return executeToolRequired(inv)
}
func (a *App) executeGoalPlannerQuery(ctx context.Context, inv CapabilityInvocation) (CapabilityResult, error) {
	if a == nil || a.DB == nil || a.DB.Pool() == nil {
		return failedCapabilityResult(inv, "planner_unavailable", true), errors.New("planner unavailable")
	}

	args, err := capabilityExecutionArguments(inv, (goalPlannerCapability{name: goalPlannerQuery}).Definition())
	if err != nil {
		return failedCapabilityResult(inv, "invalid_arguments", false), err
	}
	owner, actor, profile := inv.Metadata.FluctlightID, inv.Metadata.AuthorizationActorID, inv.Metadata.WorkingProfileID
	if _, err := a.DB.GetFluctlight(ctx, owner, actor); err != nil {
		return failedCapabilityResult(inv, "planner_scope_invalid", false), err
	}
	var result map[string]any
	switch stringValue(args["section"]) {
	case "snapshot":
		result, err = readGoalSetWith(ctx, a.DB.Pool(), owner, profile)
		if err == nil {
			result = compactGoalPlannerSnapshot(result)
		}
	case "goal":
		id := stringValue(args["goal_id"])
		if id == "" || len(id) > 128 {
			return failedCapabilityResult(inv, "invalid_arguments", false), ErrInvalidArguments
		}
		var raw []byte
		err = a.DB.Pool().QueryRow(ctx, `SELECT to_jsonb(g)-'request_digest'-'idempotency_key'-'evidence_refs' FROM public.fluctlight_goals g WHERE fluctlight_id=$1 AND id=$2 AND (profile_id IS NULL OR profile_id=$3)`, owner, id, profile).Scan(&raw)
		if err == nil {
			result = decodeObject(raw)
		}
	case "triggers":
		rows, e := a.DB.Pool().Query(ctx, `SELECT jsonb_build_object('reason',e.reason,'source_key',e.source_key,'payload',e.payload,'seq',e.seq) FROM public.goal_planning_events e WHERE fluctlight_id=$1 AND processed_run_id IS NULL AND (COALESCE(payload->>'profile_id','')='' OR payload->>'profile_id'=$2) AND (NOT payload ? 'goal_id' OR EXISTS(SELECT 1 FROM public.fluctlight_goals g WHERE g.id=e.payload->>'goal_id' AND g.fluctlight_id=e.fluctlight_id AND (g.profile_id IS NULL OR g.profile_id=$2))) ORDER BY seq DESC LIMIT 12`, owner, profile)
		err = e
		if err == nil {
			items := []any{}
			for rows.Next() {
				var raw []byte
				if err = rows.Scan(&raw); err != nil {
					break
				}
				items = append(items, decodeObject(raw))
			}
			rows.Close()
			result = map[string]any{"events": items}
		}
	case "persona":
		var revision, overlay int
		var resolved string
		result, revision, overlay, resolved, err = newPersonaDetailService(a).read(ctx, owner, actor, profile)
		if err == nil {
			delete(result, "current_appearance")
			result["source_revision"] = revision
			result["overlay_revision"] = overlay
			result["profile_id"] = resolved
		}
	case "life":
		result, _, _, err = readEffectiveLifeSnapshotWith(ctx, a.DB.Pool(), owner, a.now())
		if err == nil {
			schedule, life, lifeErr := a.readLifeContextSnapshotAt(ctx, owner, a.now())
			err = lifeErr
			if err == nil {
				result["current_life"] = compactStateMap(life, []string{"scene", "activity", "timezone", "current_time", "sleeping", "awake", "context_revision", "presence", "constraints"})
				items := arrayValue(schedule["items"])
				if len(items) > 8 {
					items = items[:8]
				}
				boundedItems := []any{}
				for _, item := range items {
					boundedItems = append(boundedItems, compactStateMap(item, []string{"start_at", "end_at", "activity", "scene", "status", "intention_id"}))
				}
				result["schedule_constraints"] = map[string]any{"timezone": schedule["timezone"], "local_date": schedule["local_date"], "revision": schedule["revision"], "items": boundedItems}
			}
		}
	case "actor":
		target := stringValue(args["actor_id"])
		if target == "" {
			target = actor
		}
		if err = a.requireVisibleActorWith(ctx, a.DB.Pool(), owner, actor, profile, target); err == nil {
			facts, factErr := readActorFactsWith(ctx, a.DB.Pool(), owner, target, a.now(), false, 32)
			err = factErr
			result = map[string]any{"actor_id": target, "facts": facts}
			if err == nil {
				rows, e := a.DB.Pool().Query(ctx, `SELECT to_jsonb(r) FROM public.relationships r WHERE owner_fluctlight_id=$1 AND target_actor_id=$2 AND (profile_id IS NULL OR profile_id=$3) ORDER BY updated_at DESC,id LIMIT 4`, owner, target, profile)
				err = e
				if err == nil {
					relations := []any{}
					for rows.Next() {
						var raw []byte
						if err = rows.Scan(&raw); err != nil {
							break
						}
						relations = append(relations, decodeObject(raw))
					}
					rows.Close()
					result["relationships"] = relations
				}
			}
		}
	case "history":
		cursor := intValue(args["cursor"])
		if cursor < 0 || cursor > 10000 {
			return failedCapabilityResult(inv, "invalid_arguments", false), ErrInvalidArguments
		}
		query := stringValue(args["query"])
		if len(query) > 256 {
			return failedCapabilityResult(inv, "invalid_arguments", false), ErrInvalidArguments
		}
		rows, e := a.DB.Pool().Query(ctx, `SELECT jsonb_build_object('id',g.id,'desired_outcome',g.desired_outcome,'status',g.status,'target_actor_id',g.target_actor_id,'criteria',g.success_criteria,'source',g.planner_source,'deadline',g.deadline,'resolution',(SELECT to_jsonb(r) FROM public.goal_resolutions r WHERE r.goal_id=g.id)) FROM public.fluctlight_goals g WHERE fluctlight_id=$1 AND (profile_id IS NULL OR profile_id=$2) AND ($3='' OR g.desired_outcome ILIKE '%'||$3||'%') ORDER BY g.updated_at DESC,g.id DESC LIMIT 11 OFFSET $4`, owner, profile, query, cursor)
		err = e
		if err == nil {
			items := []any{}
			for rows.Next() {
				var raw []byte
				if err = rows.Scan(&raw); err != nil {
					break
				}
				items = append(items, decodeObject(raw))
			}
			rows.Close()
			result = map[string]any{"items": items}
			if len(items) > 10 {
				result["items"] = items[:10]
				result["next_cursor"] = cursor + 10
			}
		}
	default:
		err = ErrInvalidArguments
	}
	if err != nil {
		return failedCapabilityResult(inv, "planner_query_failed", false), err
	}
	return goalToolResult(inv, result), nil
}
func (c goalPlannerCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, inv CapabilityInvocation, cc CapabilityContext) (CapabilityResult, error) {
	if c.name == goalPlannerQuery {
		return c.Execute(ctx, inv, cc)
	}
	return failedCapabilityResult(inv, "planner_run_target_required", false), ErrUnauthorized
}
func (c goalPlannerCapability) ExecuteDirectTx(ctx context.Context, tx pgx.Tx, inv CapabilityInvocation, _ CapabilityContext, target DirectToolTarget) (CapabilityResult, error) {
	if c.service == nil {
		return failedCapabilityResult(inv, "planner_unavailable", true), errors.New("planner unavailable")
	}
	return c.service.executeGoalPlannerCommitTx(ctx, tx, inv, target)
}
func (a *App) executeGoalPlannerCommitTx(ctx context.Context, tx pgx.Tx, inv CapabilityInvocation, target DirectToolTarget) (CapabilityResult, error) {
	if a == nil || a.DB == nil || tx == nil {
		return failedCapabilityResult(inv, "planner_unavailable", true), errors.New("planner service unavailable")
	}
	if a == nil || target.Kind != "goal_planning_run" || target.FluctlightID != inv.Metadata.FluctlightID || target.AuthorizationActorID != inv.Metadata.AuthorizationActorID {
		return failedCapabilityResult(inv, "planner_run_target_required", false), ErrUnauthorized
	}
	args, err := capabilityExecutionArguments(inv, (goalPlannerCapability{name: goalPlannerCommit}).Definition())
	if err != nil {
		return failedCapabilityResult(inv, "invalid_arguments", false), err
	}
	var cmd GoalSetCommand
	if json.Unmarshal(jsonBytes(args), &cmd) != nil {
		return failedCapabilityResult(inv, "invalid_arguments", false), ErrInvalidArguments
	}
	profile, err := (&intentionService{}).resolveProfile(ctx, tx, target.FluctlightID, inv.Metadata.WorkingProfileID)
	if err != nil {
		return failedCapabilityResult(inv, "planner_scope_invalid", false), err
	}
	result, err := a.applyGoalSetTx(ctx, tx, target.AuthorizationActorID, target.FluctlightID, profile, target.Ref, cmd)
	if err != nil {
		code, retry := capabilityErrorInfo(err, "goal_plan_rejected", false)
		return failedCapabilityResultDetail(inv, code, retry, err.Error()), err
	}
	return goalToolResult(inv, result), nil
}

const goalPlannerInstruction = `You are the independent GoalPlanner. Maintain one goal set; do not execute actions.
First query snapshot, triggers and stable persona, then current life constraints and relevant Actor facts/relations and history. Consume actual Tool results. Review existing valid unfinished goals before adding new ones. All profiles share at most five active goals; blocked/waiting goals occupy slots. Preserve older goals first by default; new goals append. Manual order and Owner pause/cancel/protection cannot be overridden. 'derived_from' is provenance, dependencies are AND prerequisites that must complete before new actions.
Propose observable finite outcomes with criteria, motivation, actual source refs, Actor and deduplication decision. Compare outcomes, Actor, window, criteria and conditions with active, paused, cancelled and completed history, not just title/hash. A completed expression does not prove acceptance. Never repeat pressure after rejection, invent resources/responses, change completion/progress/evidence or manufacture relationship facts. Stable interests can motivate real new directions but do not prove actions happened. Reuse candidate IDs using resume with current revision. New changes have a stable local id and operation=create. Pauses require legitimate reasons and no Owner protection.
In apply mode call goal_planner.commit using exact snapshot revision/facts_revision and run_id as idempotency_key, then query snapshot AGAIN and report only durable results. If commit fails, read and adjust at most once, or stop with failed/blocked. In suggestions mode do not commit. May retain fewer than five or zero goals if no viable direction: record reason and review condition. Tool/provider failure is failed, never no_viable_candidate. Do not force quota or create goals already satisfied. Return the final JSON contract.`

func (a *App) RunGoalPlannerAgent(ctx context.Context, owner, actor, profile, runRef, mode string) (map[string]any, error) {
	if a == nil || a.DB == nil {
		return nil, errors.New("goal_planner_unavailable")
	}
	if _, err := a.DB.GetFluctlight(ctx, owner, actor); err != nil {
		return nil, err
	}
	resolvedProfile, err := (&intentionService{}).resolveProfile(ctx, a.DB.Pool(), owner, profile)
	if err != nil {
		return nil, err
	}
	profile = resolvedProfile
	snapshot, err := readGoalSetWith(ctx, a.DB.Pool(), owner, profile)
	if err != nil {
		return nil, err
	}
	var triggerRaw []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT COALESCE(jsonb_agg(reason),'[]') FROM (SELECT DISTINCT reason FROM public.goal_planning_events WHERE fluctlight_id=$1 ORDER BY reason LIMIT 8) e`, owner).Scan(&triggerRaw); err != nil {
		return nil, err
	}
	snapshot["trigger_reasons"] = decodeArray(triggerRaw)
	stable, personaRevision, overlayRevision, _, personaErr := newPersonaDetailService(a).read(ctx, owner, actor, profile)
	if personaErr != nil {
		return nil, personaErr
	}
	delete(stable, "current_appearance")
	summary := jsonString(compactStateMap(stable, []string{"personality", "profile.personality", "behavioral_policy", "profile.behavioral_policy", "life_profile", "profile.stable_motivations"}))
	summaryRunes := []rune(summary)
	if len(summaryRunes) > 2000 {
		summaryRunes = summaryRunes[:2000]
	}
	snapshot["stable_persona_summary"] = string(summaryRunes)
	snapshot["persona_revision"] = personaRevision
	snapshot["overlay_revision"] = overlayRevision

	snapshot = compactGoalPlannerSnapshot(snapshot)
	schema := objectSchema(map[string]any{"decision": enumStringSchema("applied", "no_change", "no_viable_candidate", "awaiting_facts", "blocked_by_policy", "failed", "suggestions"), "reason": stringSchema(), "review_condition": stringSchema(), "suggestions": arraySchema(openObjectSchema())}, []string{"decision", "reason", "review_condition", "suggestions"}, false)
	messages := (&PromptComposer{}).ComposeTaskMessages("goal_planner", []map[string]any{{"role": "system", "content": goalPlannerInstruction}, {"role": "user", "content": jsonString(map[string]any{"run_id": runRef, "mode": mode, "snapshot": snapshot})}})
	run, err := a.RunFormalAgent(ctx, FormalAgentGoalPlanner, FormalAgentRunInput{Prompt: PromptAssemblyResult{Messages: formatProviderMessagesForRole(messages, "cognitive_assessment"), ResponseFormat: schema}, Definitions: capabilityCatalog(a.capabilityRegistry(), CapabilitySurfaceGoalPlanner), SchemaName: "goal_planner_v1", Capability: &ADKCapabilityRequest{TargetKind: "goal_planning_run", TargetRef: runRef, AuthorizationActorID: actor, FluctlightID: owner, SourceFactID: runRef, OperationID: runRef, CorrelationID: "goal-planner:" + runRef, Surface: CapabilitySurfaceGoalPlanner, Projection: ContextProjection{OwnerActorID: actor, FluctlightID: owner, SourceFactID: runRef, PersonalityRuntime: map[string]any{"active_profile_id": profile}}}})
	if err != nil {
		return nil, err
	}
	if run.Trace == nil {
		return nil, errors.New("goal_planner_trace_missing")
	}
	calls, results := run.Trace.Snapshot()
	queries, commits, afterCommit := 0, 0, false
	committedAt := -1
	for i, r := range results {
		if r.Status != "completed" {
			continue
		}
		if r.CapabilityName == goalPlannerQuery {
			queries++
			if committedAt >= 0 && i > committedAt {
				afterCommit = true
			}
		}
		if r.CapabilityName == goalPlannerCommit {
			commits++
			committedAt = i
		}
	}
	decision := stringValue(run.Completion.Structured["decision"])
	if strings.TrimSpace(stringValue(run.Completion.Structured["reason"])) == "" || len([]rune(stringValue(run.Completion.Structured["reason"]))) > 2000 {
		return nil, errors.New("goal_planner_reason_required")
	}
	if (decision == "no_viable_candidate" || decision == "awaiting_facts") && strings.TrimSpace(stringValue(run.Completion.Structured["review_condition"])) == "" {
		return nil, errors.New("goal_planner_review_condition_required")
	}

	if len(calls) == 0 || queries < 2 {
		return nil, errors.New("goal_planner_query_results_missing")
	}
	if decision == "applied" && (mode != "apply" || commits == 0 || !afterCommit) {
		return nil, errors.New("goal_planner_durable_result_missing")
	}
	if commits > 0 && decision != "applied" && decision != "no_change" {
		return nil, errors.New("goal_planner_commit_summary_mismatch")
	}
	result := run.Completion.Structured
	fresh, err := readGoalSetWith(ctx, a.DB.Pool(), owner, profile)
	if err != nil {
		return nil, err
	}
	result["snapshot"] = fresh
	result["tool_calls"] = len(calls)
	result["tool_results"] = len(results)
	return result, nil
}

func (a *App) RequestGoalPlanning(ctx context.Context, actor, owner, key string) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, owner, actor); err != nil {
		return nil, err
	}
	if key == "" || len(key) > 128 {
		return nil, ErrInvalidArguments
	}
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		return requestGoalPlanningTx(ctx, tx, owner, "owner:"+key, "owner_request", nil)
	})
	return map[string]any{"status": "requested", "idempotency_key": key}, err
}
func (a *App) ProcessGoalPlanningIntent(ctx context.Context, id string) (map[string]any, error) {
	var owner, actor, profile, mode string
	var leaseSeconds, maxAttempts, retrySeconds int
	var fence int
	var watermark int64
	var manual bool
	var prior map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT fluctlight_id FROM public.goal_planning_runs WHERE id=$1`, id).Scan(&owner); err != nil {
			return err
		}
		if err := ensureGoalSetTx(ctx, tx, owner); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT lease_seconds,max_attempts,retry_backoff_seconds FROM public.goal_set_policies WHERE fluctlight_id=$1`, owner).Scan(&leaseSeconds, &maxAttempts, &retrySeconds); err != nil {
			return err
		}
		var status string
		var claimed *time.Time
		var raw []byte
		var attempts int
		var available time.Time
		var notReady, liveClaim bool
		if err := tx.QueryRow(ctx, `SELECT status,claim_revision,claimed_at,watermark,result,attempt_count,available_at,available_at>now(),COALESCE(claimed_at>now()-($2::integer)*interval '1 second',false) FROM public.goal_planning_runs WHERE id=$1 FOR UPDATE`, id, leaseSeconds).Scan(&status, &fence, &claimed, &watermark, &raw, &attempts, &available, &notReady, &liveClaim); err != nil {
			return err
		}
		committed := decodeObject(raw)
		if committed["commit_succeeded"] == true {
			var committedProfile string
			if err := tx.QueryRow(ctx, `SELECT profile_id FROM public.goal_planning_runs WHERE id=$1`, id).Scan(&committedProfile); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE public.goal_planning_events SET processed_run_id=$3 WHERE fluctlight_id=$1 AND seq<=$2 AND processed_run_id IS NULL AND (COALESCE(payload->>'profile_id','')='' OR payload->>'profile_id'=$4)`, owner, watermark, id, committedProfile); err != nil {
				return err
			}

			prior = map[string]any{"status": "succeeded", "decision": "applied", "reason": "Recovered durable goal set commit", "snapshot": committed}
			_, err := tx.Exec(ctx, `UPDATE public.goal_planning_runs SET status='succeeded',result=$2,claimed_at=NULL WHERE id=$1`, id, jsonBytes(prior))
			return err
		}
		if status == "succeeded" || status == "failed" {
			prior = decodeObject(raw)
			return nil
		}
		if notReady || (status == "processing" && liveClaim) {
			prior = map[string]any{"status": "deferred"}
			return nil
		}
		if attempts >= maxAttempts {
			_, err := tx.Exec(ctx, `UPDATE public.goal_planning_runs SET status='failed',result='{"status":"failed","reason":"retry_exhausted"}' WHERE id=$1`, id)
			prior = map[string]any{"status": "failed", "reason": "retry_exhausted"}
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT f.created_by_actor_id,COALESCE(r.active_profile_id,''),CASE WHEN p.auto_planning_enabled THEN 'apply' ELSE 'suggestions' END FROM public.fluctlights f JOIN public.goal_set_policies p ON p.fluctlight_id=f.id LEFT JOIN public.fluctlight_personality_runtime r ON r.fluctlight_id=f.id WHERE f.id=$1`, owner).Scan(&actor, &profile, &mode); err != nil {
			return err
		}
		resolvedProfile, err := (&intentionService{}).resolveProfile(ctx, tx, owner, "")
		if err != nil {
			return err
		}
		profile = resolvedProfile
		var active bool
		if err := tx.QueryRow(ctx, `SELECT status='active' FROM public.fluctlights WHERE id=$1`, owner).Scan(&active); err != nil {
			return err
		}
		policy, err := a.evaluateAutonomyPolicyWithReader(ctx, tx, owner, "capability", a.now(), "", false)
		if err != nil {
			return err
		}
		if !active || !policy.Allowed {
			mode = "suggestions"
		}
		if err := tx.QueryRow(ctx, `SELECT COALESCE(max(seq),0) FROM public.goal_planning_events WHERE fluctlight_id=$1 AND processed_run_id IS NULL AND (COALESCE(payload->>'profile_id','')='' OR payload->>'profile_id'=$2)`, owner, profile).Scan(&watermark); err != nil {
			return err
		}
		if watermark == 0 {
			prior = map[string]any{"status": "awaiting_profile", "decision": "awaiting_facts", "reason": "Pending events belong to another private profile; retained until it becomes active"}
			_, err := tx.Exec(ctx, `UPDATE public.goal_planning_runs SET status='awaiting_profile',result=$2,claimed_at=NULL WHERE id=$1`, id, jsonBytes(prior))
			return err
		}
		var reviewTrigger, primaryTrigger bool
		var activeCount, capacity int
		if err := tx.QueryRow(ctx, `SELECT
			EXISTS(SELECT 1 FROM public.goal_planning_events WHERE fluctlight_id=$1 AND processed_run_id IS NULL AND seq<=$3 AND reason='owner_request' AND (COALESCE(payload->>'profile_id','')='' OR payload->>'profile_id'=$2)),
			EXISTS(SELECT 1 FROM public.goal_planning_events WHERE fluctlight_id=$1 AND processed_run_id IS NULL AND seq<=$3 AND reason IN ('actor_context_changed','profile_changed') AND (COALESCE(payload->>'profile_id','')='' OR payload->>'profile_id'=$2)),
			EXISTS(SELECT 1 FROM public.goal_planning_events WHERE fluctlight_id=$1 AND processed_run_id IS NULL AND seq<=$3 AND reason IN ('goal_lifecycle_changed','schedule_accepted_daily','initialization_completed','auto_planning_enabled','startup_recovery') AND (COALESCE(payload->>'profile_id','')='' OR payload->>'profile_id'=$2)),
			(SELECT count(*) FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND status='active'),
			(SELECT max_active_goals FROM public.goal_set_policies WHERE fluctlight_id=$1)`, owner, profile, watermark).Scan(&manual, &reviewTrigger, &primaryTrigger, &activeCount, &capacity); err != nil {
			return err
		}
		if !manual && !reviewTrigger && !primaryTrigger {
			prior = map[string]any{"status": "awaiting_trigger", "decision": "awaiting_facts", "reason": "Planning hints are retained until a goal ends, today's schedule is accepted, relevant context changes, or the Owner requests planning"}
			_, err := tx.Exec(ctx, `UPDATE public.goal_planning_runs SET status='awaiting_trigger',result=$2,claimed_at=NULL,updated_at=now() WHERE id=$1`, id, jsonBytes(prior))
			return err
		}
		if !manual && !reviewTrigger && activeCount >= capacity {
			prior = map[string]any{"status": "capacity_full", "decision": "no_change", "reason": "Active goal capacity is full; trigger retained until capacity is released"}
			_, err := tx.Exec(ctx, `UPDATE public.goal_planning_runs SET status='capacity_full',result=$2,claimed_at=NULL,updated_at=now() WHERE id=$1`, id, jsonBytes(prior))
			return err
		}
		if manual && activeCount >= capacity {
			mode = "suggestions"
		}
		// Automatic disabled requests do not spend a model call. Owner requests can suggest.
		if mode == "suggestions" {
			if !manual {
				prior = map[string]any{"status": "blocked_by_policy", "decision": "blocked_by_policy", "reason": firstString(policy.Reason, "Automatic planning is disabled"), "review_condition": "Restore authorized planning or wait for current policy conditions"}
				_, err := tx.Exec(ctx, `UPDATE public.goal_planning_runs SET status='blocked_by_policy',result=$2,available_at=now()+$3*interval '1 second' WHERE id=$1`, id, jsonBytes(prior), retrySeconds)
				return err
			}
		}
		fence++
		_, err = tx.Exec(ctx, `UPDATE public.goal_planning_runs SET status='processing',claim_revision=$2,claimed_at=now(),attempt_count=attempt_count+1,watermark=$3,mode=$4,profile_id=$5,updated_at=now() WHERE id=$1`, id, fence, watermark, mode, profile)
		return err
	})
	if err != nil {
		return nil, err
	}
	if prior != nil {
		return prior, nil
	}
	if !manual && a.kevService().Enabled(ctx, "goal.replenish_plan") {
		snapshot, snapshotErr := readGoalSetWith(ctx, a.DB.Pool(), owner, profile)
		if snapshotErr != nil {
			return nil, snapshotErr
		}
		allowed, until, gateErr := a.kevAutomaticGate(ctx, "goal.replenish_plan", owner, id, map[string]any{"profile": profile, "mode": mode, "event_watermark": watermark, "goal_set": compactGoalPlannerSnapshot(snapshot)})
		if gateErr != nil {
			return nil, gateErr
		}
		if !allowed {
			tag, err := a.DB.Pool().Exec(ctx, `UPDATE public.goal_planning_runs SET status='retry',claimed_at=NULL,attempt_count=GREATEST(attempt_count-1,0),available_at=$3,updated_at=now() WHERE id=$1 AND status='processing' AND claim_revision=$2`, id, fence, until)
			if err != nil {
				return nil, err
			}
			if tag.RowsAffected() != 1 {
				return nil, ErrConflict
			}
			return map[string]any{"status": "deferred", "reason": "kev_deferred", "not_before": until.Format(time.RFC3339Nano)}, nil
		}
	}
	result, runErr := a.RunGoalPlannerAgent(ctx, owner, actor, profile, id+"@"+strconv.Itoa(fence), mode)
	if runErr == nil && stringValue(result["decision"]) == "failed" {
		runErr = errors.New("goal_planner_tool_decision_failed:" + stringValue(result["reason"]))
	}

	settleErr := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := lockLifeContextTx(ctx, tx, owner); err != nil {
			return err
		}
		// A committed Tool is authoritative even if the final model response was lost.
		var committedRaw []byte
		if err := tx.QueryRow(ctx, `SELECT result FROM public.goal_planning_runs WHERE id=$1`, id).Scan(&committedRaw); err != nil {
			return err
		}
		committed := decodeObject(committedRaw)
		if runErr != nil && committed["commit_succeeded"] == true {
			result = map[string]any{"status": "succeeded", "decision": "applied", "reason": "Recovered the durable Tool commit after final response failure", "snapshot": committed}
			runErr = nil
		}
		status := "succeeded"
		code := ""
		if runErr != nil {
			status = "retry"
			if terminalGoalPlannerContractError(runErr) {
				status = "failed"
			}
			code = runErr.Error()
			result = map[string]any{"status": "deferred", "decision": "failed", "reason": code}
			if status == "failed" {
				result["status"] = "failed"
			}
		}
		tag, err := tx.Exec(ctx, `UPDATE public.goal_planning_runs SET status=$3,result=$4,error_code=$5,claimed_at=NULL,available_at=now()+$6*interval '1 second',updated_at=now() WHERE id=$1 AND claim_revision=$2 AND status='processing'`, id, fence, status, jsonBytes(result), nullableString(code), retrySeconds)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrConflict
		}
		if runErr == nil {
			if _, err := tx.Exec(ctx, `UPDATE public.goal_planning_events SET processed_run_id=$3 WHERE fluctlight_id=$1 AND seq<=$2 AND processed_run_id IS NULL AND (COALESCE(payload->>'profile_id','')='' OR payload->>'profile_id'=$4)`, owner, watermark, id, profile); err != nil {
				return err
			}
			var next int64
			if err := tx.QueryRow(ctx, `SELECT COALESCE(max(e.seq),0)
				FROM public.goal_planning_events e
				WHERE e.fluctlight_id=$1 AND e.processed_run_id IS NULL AND e.seq>$2
				AND (COALESCE(e.payload->>'profile_id','')='' OR e.payload->>'profile_id'=$3)
				AND (e.reason IN ('owner_request','actor_context_changed','profile_changed') OR
					(e.reason IN ('goal_lifecycle_changed','schedule_accepted_daily','initialization_completed','auto_planning_enabled','startup_recovery') AND
					 (SELECT count(*) FROM public.fluctlight_goals g WHERE g.fluctlight_id=$1 AND g.status='active') <
					 (SELECT max_active_goals FROM public.goal_set_policies p WHERE p.fluctlight_id=$1)))`, owner, watermark, profile).Scan(&next); err != nil {
				return err
			}
			if next > 0 { // preserve events that arrived during the model run
				nextID := "goal_planner_" + stableDigest(fmt.Sprint(owner, ":", next))
				if _, err := tx.Exec(ctx, `INSERT INTO public.goal_planning_runs(id,fluctlight_id,watermark) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, nextID, owner, next); err != nil {
					return err
				}
				_, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,next_attempt_at) VALUES($1,$2,'lifecycle','goal.plan',$3,now()+interval '2 seconds') ON CONFLICT DO NOTHING`, nextID, "goal-planner:"+nextID, jsonBytes(map[string]any{"fluctlight_id": owner}))
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
	if settleErr != nil {
		return nil, settleErr
	}
	return result, nil
}

// An unconsumed query or an unbacked final claim cannot become valid by replaying
// the same run. Keep the failure visible; a new authorized trigger can try again.
func terminalGoalPlannerContractError(err error) bool {
	if err == nil {
		return false
	}
	switch err.Error() {
	case "goal_planner_reason_required", "goal_planner_review_condition_required",
		"goal_planner_query_results_missing", "goal_planner_durable_result_missing",
		"goal_planner_commit_summary_mismatch":
		return true
	default:
		return false
	}
}

func (a *App) GoalPlanningHistory(ctx context.Context, actor, owner string) ([]map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, owner, actor); err != nil {
		return nil, err
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT to_jsonb(r) FROM public.goal_planning_runs r WHERE fluctlight_id=$1 ORDER BY created_at DESC,id DESC LIMIT 30`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []map[string]any{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		result = append(result, decodeObject(raw))
	}
	return result, rows.Err()
}

// RepairGoalPlanning runs on the existing Worker repair ticker. It reconciles
// durable work, and requests one initial review for old instances with no run;
// unchanged empty snapshots never create a periodic model-call loop.
func (a *App) RepairGoalPlanning(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 100 {
		return 0, ErrInvalidArguments
	}
	repaired := 0
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT f.id FROM public.fluctlights f JOIN public.goal_set_policies p ON p.fluctlight_id=f.id WHERE f.status='active' AND p.auto_planning_enabled AND ((SELECT count(*) FROM public.fluctlight_goals g WHERE g.fluctlight_id=f.id AND g.status='active') < p.max_active_goals OR EXISTS(SELECT 1 FROM public.goal_planning_events e WHERE e.fluctlight_id=f.id AND e.processed_run_id IS NULL AND e.reason IN ('actor_context_changed','profile_changed') AND (COALESCE(e.payload->>'profile_id','')='' OR e.payload->>'profile_id'=COALESCE((SELECT active_profile_id FROM public.fluctlight_personality_runtime WHERE fluctlight_id=f.id),'default')))) AND NOT EXISTS(SELECT 1 FROM public.goal_planning_runs r WHERE r.fluctlight_id=f.id AND r.status IN ('pending','processing','retry')) AND (NOT EXISTS(SELECT 1 FROM public.goal_planning_runs r WHERE r.fluctlight_id=f.id) OR EXISTS(SELECT 1 FROM public.goal_planning_events e WHERE e.fluctlight_id=f.id AND e.processed_run_id IS NULL AND e.reason IN ('goal_lifecycle_changed','schedule_accepted_daily','initialization_completed','auto_planning_enabled','actor_context_changed','profile_changed') AND (COALESCE(e.payload->>'profile_id','')='' OR e.payload->>'profile_id'=COALESCE((SELECT active_profile_id FROM public.fluctlight_personality_runtime WHERE fluctlight_id=f.id),'default')) AND NOT EXISTS(SELECT 1 FROM public.goal_planning_runs failed WHERE failed.fluctlight_id=f.id AND failed.status='failed' AND failed.watermark>=e.seq))) ORDER BY f.id LIMIT $1`, limit)
		if err != nil {
			return err
		}
		ids := []string{}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, id := range ids {
			policy, err := a.evaluateAutonomyPolicyWithReader(ctx, tx, id, "capability", a.now(), "", false)
			if err != nil {
				return err
			}
			if !policy.Allowed {
				continue
			}

			if err := func() error {
				var seq int64
				if err := tx.QueryRow(ctx, `SELECT COALESCE(max(e.seq),0)
					FROM public.goal_planning_events e
					JOIN public.goal_set_policies p ON p.fluctlight_id=e.fluctlight_id
					WHERE e.fluctlight_id=$1 AND e.processed_run_id IS NULL
					AND (COALESCE(e.payload->>'profile_id','')='' OR e.payload->>'profile_id'=COALESCE((SELECT active_profile_id FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1),'default'))
					AND (e.reason IN ('actor_context_changed','profile_changed') OR
						(e.reason IN ('goal_lifecycle_changed','schedule_accepted_daily','initialization_completed','auto_planning_enabled','startup_recovery') AND
						 (SELECT count(*) FROM public.fluctlight_goals g WHERE g.fluctlight_id=$1 AND g.status='active') < p.max_active_goals))
					AND NOT EXISTS(SELECT 1 FROM public.goal_planning_runs failed WHERE failed.fluctlight_id=e.fluctlight_id AND failed.status='failed' AND failed.watermark>=e.seq)`, id).Scan(&seq); err != nil {
					return err
				}
				if seq == 0 {
					return requestGoalPlanningTx(ctx, tx, id, "startup-recovery:"+id, "startup_recovery", nil)
				}
				// A previous recovery run may have stopped at a now-cleared fence
				// (for example capacity_full). Use a fresh operational run identity;
				// the durable event watermark and goal-set CAS retain idempotency.
				runID := randomID("goal_planner_repair_")
				_, err := tx.Exec(ctx, `INSERT INTO public.goal_planning_runs(id,fluctlight_id,watermark,available_at) VALUES($1,$2,$3,now()) ON CONFLICT DO NOTHING`, runID, id, seq)
				return err
			}(); err != nil {
				return err
			}
			repaired++
		}
		tag, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,next_attempt_at) SELECT id,'goal-planner:'||id,'lifecycle','goal.plan',jsonb_build_object('fluctlight_id',fluctlight_id),available_at FROM public.goal_planning_runs WHERE status IN ('pending','retry') AND attempt_count<5 ORDER BY available_at,id LIMIT $1 ON CONFLICT(intent_id) DO NOTHING`, limit)
		repaired += int(tag.RowsAffected())
		return err
	})
	return repaired, err
}

func compactGoalPlannerSnapshot(snapshot map[string]any) map[string]any {
	result := compactStateMap(snapshot, []string{"revision", "facts_revision", "active_count", "max_active_goals", "auto_planning_enabled", "ordering_mode", "capacity_violation", "trigger_reasons", "dependencies", "stable_persona_summary", "persona_revision", "overlay_revision", "merge_window_seconds", "retry_backoff_seconds", "max_attempts", "lease_seconds", "candidate_limit"})
	goals := []any{}
	counts := map[string]int{}
	for _, raw := range arrayValue(snapshot["goals"]) {
		g := mapValue(raw)
		status := stringValue(g["status"])
		if counts[status] >= 5 {
			continue
		}
		counts[status]++
		item := compactStateMap(g, []string{"id", "revision", "criteria_version", "profile_id", "target_actor_id", "status", "scope", "deadline", "effective_order", "owner_protected", "context_review_required"})
		for _, key := range []string{"desired_outcome", "motivation"} {
			text := []rune(stringValue(g[key]))
			if len(text) > 300 {
				text = text[:300]
			}
			item[key] = string(text)
		}
		goals = append(goals, item)
	}
	result["goals"] = goals
	result["detail_hint"] = "Use goal query by id for criteria/details; history is paged and candidates do not execute. Counts include hidden goals without their content."
	return result
}

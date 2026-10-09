package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type goalCapabilityService struct {
	repository *PostgresRepository
	clock      func() time.Time
}

func newGoalCapabilityService(app *App) *goalCapabilityService {
	if app == nil {
		return nil
	}
	return &goalCapabilityService{repository: app.DB, clock: app.now}
}
func (s *goalCapabilityService) now() time.Time {
	if s.clock != nil {
		return s.clock()
	}
	return time.Now().UTC()
}

type goalCapability struct {
	service *goalCapabilityService
	name    string
}

func (c goalCapability) Definition() CapabilityDefinition {
	fields := map[string]any{"goal_id": stringSchema(), "expected_revision": integerSchema(), "reason": stringSchema()}
	required := []string{"goal_id", "expected_revision", "reason"}
	description := "Queue evaluation for new relevant evidence or changed standards, not every ordinary action. Receipt is not Goal completion. Evaluation runs after this Agent ends; do not poll it within this run."
	sideEffect, boundary := "native_projection", "goal_evaluation_queued"
	capabilityType := CapabilityTypeAction
	if c.name == "goal.inspect" {
		fields = map[string]any{"operation": enumStringSchema("list", "detail"), "goal_id": stringSchema(), "include_closed": booleanSchema(), "cursor": integerSchema()}
		required = []string{"operation"}
		description = "Read Goals/results. List 10; reuse next_cursor; detail includes evidence."
		sideEffect, boundary = "read_only", "query_result_available"
		capabilityType = CapabilityTypeQuery
	}
	if c.name == "goal.decide" {
		fields["operation"] = enumStringSchema("create", "update", "pause", "resume", "cancel", "abandon")
		fields["desired_outcome"] = stringSchema()
		fields["success_criteria"] = arraySchema(stringSchema())
		fields["motivation"] = stringSchema()
		required = []string{"operation", "reason"}
		description = "Create/resume request Planner, not activation. Other commands need inspect ID/revision; no forced completion."
		boundary = "goal_revision_committed"
	}
	if c.name == "goal.review" {
		fields = map[string]any{"reason": stringSchema()}
		description = "Queue local-day review; no quota/forced completion."
		required = []string{"reason"}
		boundary = "goal_review_queued"
	}
	return CapabilityDefinition{Name: c.name, Version: "v1", Type: capabilityType, Description: description, Surfaces: []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceNativeCognition, CapabilitySurfaceAutonomy}, FailurePolicy: FailurePolicyOptionalInternal, InputSchema: objectSchema(fields, required, false), OutputSchema: openObjectSchema(), SideEffectClass: sideEffect, SuccessBoundary: boundary, ConcurrencyClass: map[bool]string{true: "parallel", false: "exclusive"}[capabilityType == CapabilityTypeQuery], SupportsRetry: true}
}
func (c goalCapability) RequiredContext() []ContextSlot { return nil }
func goalToolResult(inv CapabilityInvocation, output map[string]any) CapabilityResult {
	return CapabilityResult{CallID: inv.CallID, CapabilityName: inv.CapabilityName, Status: "completed", Output: output, ProviderRequestID: inv.ProviderRequestID}
}
func (c goalCapability) Execute(ctx context.Context, inv CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	if c.name != "goal.inspect" {
		return executeToolRequired(inv)
	}
	if c.service == nil || c.service.repository == nil {
		return failedCapabilityResult(inv, "goal_unavailable", true), errors.New("goal service unavailable")
	}
	args, err := capabilityExecutionArguments(inv, c.Definition())
	if err != nil {
		return failedCapabilityResult(inv, "invalid_arguments", false), err
	}
	owner := inv.Metadata.FluctlightID
	profile, err := (&intentionService{}).resolveProfile(ctx, c.service.repository.Pool(), owner, inv.Metadata.WorkingProfileID)
	if err != nil {
		return failedCapabilityResult(inv, "goal_scope_unavailable", true), err
	}
	var output map[string]any
	if stringValue(args["operation"]) == "detail" {
		id := stringValue(args["goal_id"])
		var raw []byte
		if err := c.service.repository.Pool().QueryRow(ctx, `SELECT to_jsonb(g)-'request_digest'-'idempotency_key' FROM public.fluctlight_goals g WHERE fluctlight_id=$1 AND id=$2 AND (profile_id IS NULL OR profile_id=$3)`, owner, id, profile).Scan(&raw); err != nil {
			return failedCapabilityResult(inv, "goal_not_found", false), err
		}
		goal := decodeObject(raw)
		execution, err := readGoalExecutionStateWith(ctx, c.service.repository.Pool(), owner, id, stringValue(goal["status"]), c.service.now())
		if err != nil {
			return failedCapabilityResult(inv, "goal_read_failed", true), err
		}
		goal["execution"] = execution
		for key, query := range map[string]string{
			"stages":      `SELECT COALESCE(jsonb_agg(v),'[]') FROM (SELECT to_jsonb(s) v FROM public.goal_stages s WHERE fluctlight_id=$1 AND goal_id=$2 ORDER BY updated_at DESC,id DESC LIMIT 12)x`,
			"commitments": `SELECT COALESCE(jsonb_agg(v),'[]') FROM (SELECT to_jsonb(s) v FROM public.goal_commitments s WHERE fluctlight_id=$1 AND goal_id=$2 ORDER BY updated_at DESC,id DESC LIMIT 12)x`,
			"reviews":     `SELECT COALESCE(jsonb_agg(v),'[]') FROM (SELECT to_jsonb(s) v FROM public.goal_reviews s WHERE fluctlight_id=$1 AND goal_id=$2 ORDER BY local_date DESC,id DESC LIMIT 6)x`,
		} {
			var data []byte
			if err := c.service.repository.Pool().QueryRow(ctx, query, owner, id).Scan(&data); err != nil {
				return failedCapabilityResult(inv, "goal_read_failed", true), err
			}
			goal[key] = decodeArray(data)
		}

		output = map[string]any{"goal": goal}
	} else {
		includeClosed, _ := args["include_closed"].(bool)
		offset := intValue(args["cursor"])
		if offset < 0 || offset > 10000 {
			return failedCapabilityResult(inv, "invalid_arguments", false), ErrInvalidArguments
		}
		rows, err := c.service.repository.Pool().Query(ctx, `SELECT to_jsonb(g)-'request_digest'-'idempotency_key' FROM public.fluctlight_goals g WHERE fluctlight_id=$1 AND (profile_id IS NULL OR profile_id=$2) AND ($3 OR status IN ('candidate','active','paused')) ORDER BY updated_at DESC,id DESC LIMIT 11 OFFSET $4`, owner, profile, includeClosed, offset)
		if err != nil {
			return failedCapabilityResult(inv, "goal_read_failed", true), err
		}
		defer rows.Close()
		items := []any{}
		more := false
		for rows.Next() {
			var raw []byte
			if err := rows.Scan(&raw); err != nil {
				return failedCapabilityResult(inv, "goal_read_failed", true), err
			}
			if len(items) == 10 {
				more = true
				break
			}
			items = append(items, decodeObject(raw))
		}
		if err := rows.Err(); err != nil {
			return failedCapabilityResult(inv, "goal_read_failed", true), err
		}
		output = map[string]any{"goals": items, "has_more": more}
		if more {
			output["next_cursor"] = offset + 10
		}
	}
	return goalToolResult(inv, output), nil
}
func (c goalCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, inv CapabilityInvocation, capCtx CapabilityContext) (CapabilityResult, error) {
	if c.name == "goal.inspect" {
		return c.Execute(ctx, inv, capCtx)
	}
	if c.service == nil || c.service.repository == nil {
		return failedCapabilityResult(inv, "goal_unavailable", true), errors.New("goal service unavailable")
	}
	args, err := capabilityExecutionArguments(inv, c.Definition())
	if err != nil {
		return failedCapabilityResult(inv, "invalid_arguments", false), err
	}
	owner := inv.Metadata.FluctlightID
	profile, err := (&intentionService{}).resolveProfile(ctx, tx, owner, inv.Metadata.WorkingProfileID)
	if err != nil {
		return failedCapabilityResult(inv, "goal_scope_unavailable", true), err
	}
	if err := lockLifeContextTx(ctx, tx, owner); err != nil {
		return failedCapabilityResult(inv, "goal_lock_failed", true), err
	}
	key := "goal-tool:" + stableDigest(owner+"\x1f"+capabilityOperationID(inv))
	reason := stringValue(args["reason"])
	if reason == "" {
		return failedCapabilityResult(inv, "invalid_arguments", false), ErrInvalidArguments
	}
	if c.name == "goal.review" {
		id, err := queueGoalReviewsTx(ctx, tx, owner, profile, "tool:"+key, c.service.now())
		if err != nil {
			return failedCapabilityResult(inv, "goal_review_failed", true), err
		}
		return goalToolResult(inv, map[string]any{"evaluation_request_id": id, "status": map[bool]string{true: "queued", false: "already_reviewed"}[id != ""]}), nil
	}
	id := stringValue(args["goal_id"])
	operation := stringValue(args["operation"])
	refs := []string{"tool-operation:" + stableDigest(key)}
	var current *GoalAuthority
	var next GoalAuthority
	var record GoalGovernanceRecord
	if c.name == "goal.decide" && operation == "create" {
		if inv.Metadata.Source != "direct" {
			if err := requestGoalPlanningTx(ctx, tx, owner, key, "cognition_wish", args); err != nil {
				return failedCapabilityResult(inv, "planner_request_failed", true), err
			}
			return goalToolResult(inv, map[string]any{"status": "planning_requested", "activated": false, "reason": "Independent GoalPlanner owns autonomous creation"}), nil
		}
		id = "goal_tool_" + stableDigest(key)
		next, record, err = CreateGoalAuthority(GoalAuthority{EntityID: id, SchemaVersion: goalAuthoritySchemaVersion, Ref: "goal:ctx_" + stableDigest(id), FluctlightID: owner, ProfileID: profile, DesiredOutcome: stringValue(args["desired_outcome"]), SuccessCriteria: decisionServiceRefValues(arrayValue(args["success_criteria"])), Motivation: stringValue(args["motivation"]), Scope: "general", Status: GoalActive, Revision: 1}, refs, c.service.now())
	} else {
		revision := intValue(args["expected_revision"])
		goal, loadErr := loadGoalAuthorityTx(ctx, tx, owner, "goal:ctx_"+stableDigest(id), ContextReference{EntityID: id, Revision: revision})
		if loadErr != nil {
			return failedCapabilityResult(inv, "goal_revision_conflict", false), loadErr
		}
		if goal.ProfileID != "" && goal.ProfileID != profile {
			return failedCapabilityResult(inv, "goal_scope_invalid", false), ErrUnauthorized
		}
		if operation == "resume" && inv.Metadata.Source != "direct" {
			if err := requestGoalPlanningTx(ctx, tx, owner, key, "goal_activation_suggested", args); err != nil {
				return failedCapabilityResult(inv, "planner_request_failed", true), err
			}
			return goalToolResult(inv, map[string]any{"status": "planning_requested", "activated": false, "goal_id": id}), nil
		}
		current = &goal
		if c.name == "goal.evaluate" {
			if goal.Status != GoalActive && goal.Status != GoalPaused {
				return failedCapabilityResult(inv, "goal_terminal", false), ErrInvalidArguments
			}
			requestID, err := queueGoalEvaluationTx(ctx, tx, owner, profile, "tool_evaluation", key, []string{id})
			if err != nil {
				return failedCapabilityResult(inv, "goal_evaluation_failed", true), err
			}
			return goalToolResult(inv, map[string]any{"goal_id": id, "evaluation_request_id": requestID, "status": "queued"}), nil
		}
		patch := GoalPatch{}
		if value, ok := args["desired_outcome"]; ok {
			text := stringValue(value)
			patch.DesiredOutcome = &text
		}
		if value, ok := args["motivation"]; ok {
			text := stringValue(value)
			patch.Motivation = &text
		}
		if value, ok := args["success_criteria"]; ok {
			patch.SuccessCriteria = decisionServiceRefValues(arrayValue(value))
		}
		next, record, err = ApplyGoalCommand(current, GoalCommand{Operation: GoalLifecycleOperation(operation), ExpectedRevision: revision, Patch: patch, EvidenceRefs: refs, Reason: reason, OccurredAt: c.service.now()})
	}
	if err != nil {
		return failedCapabilityResult(inv, "goal_command_rejected", false), err
	}
	record.ActorID, record.Source, record.Reason = owner, "tool", reason
	if _, err := persistGoalAuthorityTx(ctx, tx, current, next, record, key); err != nil {
		return failedCapabilityResult(inv, "goal_persist_failed", true), err
	}
	return goalToolResult(inv, map[string]any{"goal_id": id, "status": next.Status, "revision": next.Revision, "criteria_version": next.CriteriaVersion, "ref": next.Ref, "command_id": fmt.Sprint(key)}), nil
}

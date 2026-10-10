package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/jackc/pgx/v5"
)

const CapabilitySurfaceGoalEvaluation CapabilitySurface = "goal_evaluation"
const goalEvaluationSubmit = "goal.evaluation.submit"
const goalObjectSubmit = "goal.object.submit"
const goalPlanSubmit = "goal.plan.submit"
const goalEvaluationToolProtocol = "native_tools_v1"

// A session is installed only by the typed evaluation task, never by a Tool
// caller or model argument. Its wire binding remains immutable across calls.
type goalEvaluationSessionKey struct{}
type goalEvaluationSession struct {
	binding    *goalEvaluationWireBinding
	projection ContextProjection
	runID      string
}

type goalEvaluationPhysicalRequestPolicy struct {
	app     *App
	session *goalEvaluationSession
}

func (p goalEvaluationPhysicalRequestPolicy) DecidePhysicalModelRequest(ctx context.Context) (physicalModelRequestDecision, error) {
	if p.app == nil || p.app.DB == nil || p.app.DB.Pool() == nil || p.session == nil || p.session.binding == nil {
		return physicalModelRequestDecision{}, errors.New("goal_evaluation_unavailable")
	}
	snapshot := p.session.binding.snapshot
	var state string
	var claim int
	var raw []byte
	if err := p.app.DB.Pool().QueryRow(ctx, `SELECT status,claim_revision,result FROM public.goal_evaluation_requests WHERE id=$1 AND fluctlight_id=$2 AND profile_id=$3`, snapshot.RequestID, snapshot.FluctlightID, snapshot.ProfileID).Scan(&state, &claim, &raw); err != nil {
		return physicalModelRequestDecision{}, err
	}
	if state != "processing" || claim != snapshot.ClaimRevision {
		return physicalModelRequestDecision{}, newCapabilityError("goal_evaluation_claim_stale", false, ErrConflict)
	}
	// Frozen source/authority conflicts cannot be corrected inside this run. Let
	// the Agent produce its final summary so the existing finalizer can persist
	// the trace error and replace the stale request with a fresh snapshot. Root
	// coverage itself remains determined only by the durable submission journal.
	if goalEvaluationTraceNeedsFreshSnapshot(ctx) {
		return physicalModelRequestDecision{ToolChoice: schema.ToolChoiceAllowed}, nil
	}
	_, missing := goalEvaluationSubmissionCoverage(snapshot, goalSubmissionRecords(decodeObject(raw)))
	if len(missing) > 0 {
		return physicalModelRequestDecision{ToolChoice: schema.ToolChoiceForced, OmitResponseFormat: true}, nil
	}
	return physicalModelRequestDecision{ToolChoice: schema.ToolChoiceAllowed}, nil
}

func goalEvaluationTraceNeedsFreshSnapshot(ctx context.Context) bool {
	adkContext, ok := adkCapabilityContext(ctx)
	if !ok || adkContext.Trace == nil {
		return false
	}
	_, results := adkContext.Trace.Snapshot()
	for _, result := range results {
		if isGoalEvaluationTool(result.CapabilityName) && goalEvaluationStaleSubmissionError(result.ErrorCode) {
			return true
		}
	}
	return false
}

func isGoalEvaluationTool(name string) bool {
	return name == goalEvaluationSubmit || name == goalObjectSubmit || name == goalPlanSubmit
}

func authorizeGoalEvaluationTool(ctx context.Context, request ToolExecutionRequest) error {
	s, ok := ctx.Value(goalEvaluationSessionKey{}).(*goalEvaluationSession)
	agent, agentOK := formalAgentDefinitionFromContext(ctx)
	if !ok || s == nil || s.binding == nil || !agentOK || agent.ID != FormalAgentGoalEvaluation {
		return newCapabilityError("goal_evaluation_session_required", false, ErrUnauthorized)
	}
	snapshot := s.binding.snapshot
	if request.AgentID != FormalAgentGoalEvaluation || request.RunID != s.runID || request.Surface != CapabilitySurfaceGoalEvaluation ||
		request.NativeToolCallID == "" || request.ProviderRequestID == "" || request.TargetKind != "goal_evaluation_run" || request.TargetRef != snapshot.RequestID ||
		request.EvidenceID != snapshot.RequestID || request.FluctlightID != snapshot.FluctlightID || request.AuthorizationActorID != snapshot.OwnerActorID || request.WorkingProfileID != snapshot.ProfileID {
		return newCapabilityError("goal_evaluation_session_scope_invalid", false, ErrUnauthorized)
	}
	return nil
}

// Check before either receipt replay path; a receipt does not authorize a stale
// task claim to execute or recover another task's private submission.
func (a *App) authorizeGoalEvaluationClaim(ctx context.Context) error {
	if a == nil || a.DB == nil || a.DB.Pool() == nil {
		return errors.New("goal_evaluation_unavailable")
	}
	s := ctx.Value(goalEvaluationSessionKey{}).(*goalEvaluationSession)
	var state string
	var claim int
	err := a.DB.Pool().QueryRow(ctx, `SELECT status,claim_revision FROM public.goal_evaluation_requests WHERE id=$1 AND fluctlight_id=$2 AND profile_id=$3`, s.binding.snapshot.RequestID, s.binding.snapshot.FluctlightID, s.binding.snapshot.ProfileID).Scan(&state, &claim)
	if err != nil {
		return err
	}
	if state != "processing" || claim != s.binding.snapshot.ClaimRevision {
		return newCapabilityError("goal_evaluation_claim_stale", false, ErrConflict)
	}
	return nil
}

type goalEvaluationToolService interface {
	authorizeGoalEvaluationClaim(context.Context) error
	executeGoalEvaluationSubmissionTx(context.Context, pgx.Tx, CapabilityInvocation, DirectToolTarget) (CapabilityResult, error)
}
type goalEvaluationCapability struct {
	service goalEvaluationToolService
	name    string
}

func (c goalEvaluationCapability) AuthorizeToolExecution(ctx context.Context, request ToolExecutionRequest) error {
	if err := authorizeGoalEvaluationTool(ctx, request); err != nil {
		return err
	}
	if c.service == nil {
		return errors.New("goal_evaluation_unavailable")
	}
	return c.service.authorizeGoalEvaluationClaim(ctx)
}

func goalEvaluationCapabilities(a *App) []Capability {
	return []Capability{goalEvaluationCapability{a, goalEvaluationSubmit}, goalEvaluationCapability{a, goalObjectSubmit}, goalEvaluationCapability{a, goalPlanSubmit}}
}

// Canonical schemas use closed shapes and short references; run-specific
// ownership, versions and allowed references are checked by the frozen binding.
func goalEvaluationToolSchemas() (map[string]any, map[string]any, map[string]any) {
	j := goalCriterionJudgmentSchema([]string{"ref"}, []string{"ref"})
	jp := mapValue(j["properties"])
	jp["criterion_ref"] = stringSchema()
	jp["evidence_refs"] = arraySchema(stringSchema())
	review := goalReviewSchema([]string{"ref"}, []string{"ref"})
	rp := mapValue(review["properties"])
	rp["stage_ref"] = stringSchema()
	rp["evidence_refs"] = arraySchema(stringSchema())
	root := objectSchema(map[string]any{
		"goal_ref": stringSchema(), "judgments": arraySchema(j), "impact": enumStringSchema("progressed", "no_change", "blocked", "regressed", "needs_evidence", "completed"),
		"blocker": stringSchema(), "wait_condition": stringSchema(), "next_step": stringSchema(), "next_review_at": stringSchema(), "residual_motivation": stringSchema(), "review": review,
		"relationship_confirmation": objectSchema(map[string]any{"target_actor_ref": stringSchema(), "evidence_refs": arraySchema(stringSchema()), "label": stringSchema()}, []string{"target_actor_ref", "evidence_refs", "label"}, false),
		"followup":                  objectSchema(map[string]any{"desired_outcome": stringSchema(), "success_criteria": arraySchema(stringSchema()), "motivation": stringSchema()}, []string{"desired_outcome", "success_criteria", "motivation"}, false),
	}, []string{"goal_ref", "judgments", "impact", "blocker", "wait_condition", "next_step", "residual_motivation"}, false)
	object := objectSchema(map[string]any{"goal_ref": stringSchema(), "object_kind": enumStringSchema("stage", "commitment"), "object_ref": stringSchema(), "judgments": arraySchema(j), "completed": booleanSchema(), "reason": stringSchema()}, []string{"goal_ref", "object_kind", "object_ref", "judgments", "completed", "reason"}, false)
	stage := goalOperationPlanSchema(map[string]any{"object_ref": stringSchema(), "dependency_refs": arraySchema(stringSchema()), "purpose": stringSchema(), "strategy": stringSchema(), "entry_basis": stringSchema(), "exit_basis": stringSchema(), "criteria": arraySchema(stringSchema()), "reason": stringSchema()}, []string{"purpose", "strategy", "entry_basis", "exit_basis", "criteria", "reason"}, []string{"ref"}, []string{"adjust", "skip"})
	mapValue(mapValue(arrayValue(stage["anyOf"])[1])["properties"])["object_ref"] = stringSchema()
	commitment := goalOperationPlanSchema(map[string]any{"object_ref": stringSchema(), "reason": stringSchema(), "expected_result": stringSchema(), "criteria": arraySchema(stringSchema()), "window_start": stringSchema(), "window_end": stringSchema(), "opportunity_condition": stringSchema(), "blocker": stringSchema()}, []string{"expected_result", "criteria", "opportunity_condition", "blocker"}, []string{"ref"}, []string{"adjust", "abandon"})
	mapValue(mapValue(arrayValue(commitment["anyOf"])[1])["properties"])["object_ref"] = stringSchema()
	plan := objectSchema(map[string]any{"goal_ref": stringSchema(), "reason": stringSchema(), "next_step": stringSchema(), "wait_condition": stringSchema(), "next_review_at": stringSchema(), "stage": stage, "commitment": commitment}, []string{"goal_ref", "reason", "next_step", "wait_condition"}, false)
	return root, object, plan
}

func (c goalEvaluationCapability) Definition() CapabilityDefinition {
	root, object, plan := goalEvaluationToolSchemas()
	schema, description := root, "Submit one Goal's judgments and due review. Core validates frozen evidence and returns the committed Goal status. No Stage/Commitment judgments here."
	if c.name == goalObjectSubmit {
		schema, description = object, "Submit one frozen Stage or Commitment independently. This cannot complete the parent Goal. A rejection never undoes other submissions."
	}
	if c.name == goalPlanSubmit {
		schema, description = plan, "Submit one current plan for an active Goal independently. For review decision=adjust, submit this plan before the root evaluation. Do not plan a completed Goal."
	}
	return CapabilityDefinition{Name: c.name, Version: "v1", Type: CapabilityTypeAction, Description: description, Surfaces: []CapabilitySurface{CapabilitySurfaceGoalEvaluation}, FailurePolicy: FailurePolicyOptionalInternal, InputSchema: schema, OutputSchema: openObjectSchema(), SideEffectClass: "native_projection", SuccessBoundary: "goal_submission_committed", ConcurrencyClass: "exclusive", SupportsRetry: true}
}
func (c goalEvaluationCapability) RequiredContext() []ContextSlot { return nil }
func (c goalEvaluationCapability) Execute(_ context.Context, inv CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return executeToolRequired(inv)
}
func (c goalEvaluationCapability) ExecuteTx(_ context.Context, _ pgx.Tx, inv CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return failedCapabilityResult(inv, "goal_evaluation_session_required", false), ErrUnauthorized
}
func (c goalEvaluationCapability) ExecuteDirectTx(ctx context.Context, tx pgx.Tx, inv CapabilityInvocation, _ CapabilityContext, target DirectToolTarget) (CapabilityResult, error) {
	if c.service == nil {
		return failedCapabilityResult(inv, "goal_evaluation_unavailable", true), errors.New("goal_evaluation_unavailable")
	}
	return c.service.executeGoalEvaluationSubmissionTx(ctx, tx, inv, target)
}

// Accepted submission records share the request row and transaction with domain
// mutations. tool_executions separately preserves each real native receipt.
// No additional state store or migration is needed for this bounded journal.
type goalSubmissionRecord struct {
	Digest string         `json:"digest"`
	Output map[string]any `json:"output"`
}

func goalSubmissionRecords(result map[string]any) map[string]goalSubmissionRecord {
	values := map[string]goalSubmissionRecord{}
	_ = json.Unmarshal(jsonBytes(result["submissions"]), &values)
	return values
}
func goalSubmissionKey(name, goalID, objectRef string) string {
	return name + ":" + goalID + ":" + objectRef
}

func (a *App) executeGoalEvaluationSubmissionTx(ctx context.Context, tx pgx.Tx, inv CapabilityInvocation, target DirectToolTarget) (CapabilityResult, error) {
	s, ok := ctx.Value(goalEvaluationSessionKey{}).(*goalEvaluationSession)
	agent, agentOK := formalAgentDefinitionFromContext(ctx)
	if !ok || s == nil || s.binding == nil || !agentOK || agent.ID != FormalAgentGoalEvaluation || inv.Metadata.Source != "model_tool" || inv.Metadata.Surface != CapabilitySurfaceGoalEvaluation {
		return failedCapabilityResult(inv, "goal_evaluation_session_required", false), ErrUnauthorized
	}
	snapshot := s.binding.snapshot
	if target.Kind != "goal_evaluation_run" || target.Ref != snapshot.RequestID || target.FluctlightID != snapshot.FluctlightID || target.AuthorizationActorID != snapshot.OwnerActorID || inv.SourceFactID != snapshot.RequestID || inv.Metadata.WorkingProfileID != snapshot.ProfileID {
		return failedCapabilityResult(inv, "goal_evaluation_session_scope_invalid", false), ErrUnauthorized
	}
	var state string
	var claim int
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT status,claim_revision,result FROM public.goal_evaluation_requests WHERE id=$1 FOR UPDATE`, snapshot.RequestID).Scan(&state, &claim, &raw); err != nil {
		return CapabilityResult{}, err
	}
	if state != "processing" || claim != snapshot.ClaimRevision {
		return failedCapabilityResult(inv, "goal_evaluation_claim_stale", false), nil
	}
	result := decodeObject(raw)
	if result == nil {
		result = map[string]any{}
	}
	args, err := capabilityExecutionArguments(inv, (goalEvaluationCapability{name: inv.CapabilityName}).Definition())
	if err != nil {
		return failedCapabilityResult(inv, "invalid_arguments", false), nil
	}
	entry, ok := s.binding.goalsByRef[stringValue(args["goal_ref"])]
	if !ok {
		return failedCapabilityResult(inv, "goal_evaluation_wire_goal_ref_invalid", false), nil
	}
	records := goalSubmissionRecords(result)
	key := goalSubmissionKey(inv.CapabilityName, entry.GoalID, stringValue(args["object_ref"]))
	digest := stableDigest(jsonString(args))
	if prior, exists := records[key]; exists {
		if prior.Digest != digest {
			return failedCapabilityResult(inv, "goal_submission_conflict", false), nil
		}
		output := cloneMap(prior.Output)
		output["replayed"] = true
		return goalToolResult(inv, output), nil
	}
	// A nested transaction is a PostgreSQL savepoint. Correctable domain errors
	// roll back this call's writes before persisting its failed native receipt.
	sp, err := tx.Begin(ctx)
	if err != nil {
		return CapabilityResult{}, err
	}
	output, next, err := a.applyGoalSubmissionTx(ctx, sp, s, entry, args, inv.CapabilityName, result, records)
	if err != nil {
		if rollbackErr := sp.Rollback(ctx); rollbackErr != nil {
			return CapabilityResult{}, errors.Join(err, rollbackErr)
		}
		if goalSubmissionBusinessError(err) {
			return failedCapabilityResultDetail(inv, err.Error(), false, "No change from this submission committed; correct this call using the frozen refs. Previously accepted submissions remain committed."), nil
		}
		return CapabilityResult{}, err
	}
	if err := sp.Commit(ctx); err != nil {
		return CapabilityResult{}, err
	}
	records[key] = goalSubmissionRecord{Digest: digest, Output: output}
	result["submissions"] = records
	revisions := mapValue(result["goal_revisions"])
	if revisions == nil {
		revisions = map[string]any{}
	}
	revisions[entry.GoalID] = next.Revision
	result["goal_revisions"] = revisions
	if _, exists := records[goalSubmissionKey(goalEvaluationSubmit, entry.GoalID, "")]; exists {
		refreshed, err := refreshGoalAssessmentEntryTx(ctx, tx, next, admittedGoalEvaluationSources(snapshot))
		if err != nil {
			return CapabilityResult{}, err
		}
		memos := []goalAssessmentMemo{}
		_ = json.Unmarshal(jsonBytes(result["assessment_memos"]), &memos)
		kept := []goalAssessmentMemo{}
		for _, m := range memos {
			if m.GoalID != entry.GoalID {
				kept = append(kept, m)
			}
		}
		result["assessment_memos"] = goalAssessmentMemoValues(append(kept, goalAssessmentMemoFor(refreshed, admittedGoalEvaluationSources(snapshot), s.projection, a.now())))
	}
	result["status"], result["request_id"] = "partial", snapshot.RequestID
	accepted, missing := goalEvaluationSubmissionCoverage(snapshot, records)
	result["evaluated_goals"], result["unresolved_goals"] = accepted, missing
	outcomes := []map[string]any{}
	for _, goalID := range accepted {
		value := cloneMap(records[goalSubmissionKey(goalEvaluationSubmit, goalID, "")].Output)
		value["goal_id"] = goalID
		outcomes = append(outcomes, value)
	}
	result["goal_outcomes"] = outcomes
	_, err = tx.Exec(ctx, `UPDATE public.goal_evaluation_requests SET result=$2,updated_at=now() WHERE id=$1`, snapshot.RequestID, jsonBytes(result))
	return goalToolResult(inv, output), err
}

func goalSubmissionBusinessError(err error) bool {
	// Never turn database, cancellation, or dependency failures into model
	// success. Only closed domain errors can be corrected in this native loop.
	return err != nil && (strings.HasPrefix(err.Error(), "goal_") || errors.Is(err, ErrConflict) || errors.Is(err, ErrFoundationRevisionStale) || errors.Is(err, ErrLifeContextStale))
}

func (a *App) applyGoalSubmissionTx(ctx context.Context, tx pgx.Tx, s *goalEvaluationSession, entry goalEvaluationGoal, args map[string]any, name string, result map[string]any, records map[string]goalSubmissionRecord) (map[string]any, GoalAuthority, error) {
	if err := verifyGoalAssessmentProjectionAuthorityTx(ctx, tx, a, s.projection); err != nil {
		return nil, GoalAuthority{}, err
	}
	expected := entry.Goal.Revision
	if v := intValue(mapValue(result["goal_revisions"])[entry.GoalID]); v > 0 {
		expected = v
	}
	live, err := loadGoalAuthorityTx(ctx, tx, entry.Goal.FluctlightID, entry.Goal.Ref, ContextReference{EntityID: entry.GoalID, Revision: expected})
	if err != nil {
		if err.Error() == "reflection_goal_revision_stale" {
			err = errors.New("goal_submission_authority_stale")
		}
		return nil, live, err
	}
	if live.TargetActorID == "" {
		live.TargetActorID = entry.Goal.TargetActorID
	}
	if goalStatusTerminal(live.Status) {
		return nil, live, errors.New("goal_submission_parent_terminal")
	}
	sources := map[string]GoalSource{}
	for _, source := range admittedGoalEvaluationSources(s.binding.snapshot) {
		if source.Kind == "goal_revision" {
			continue
		}
		actual, err := readGoalSourceWith(ctx, tx, source.EventID, true)
		if err != nil {
			return nil, live, err
		}
		if goalAssessmentSourceFingerprint(actual) != goalAssessmentSourceFingerprint(source) {
			return nil, live, errors.New("goal_evaluation_source_stale")
		}
		sources[source.Ref] = actual
	}
	next := live
	switch name {
	case goalEvaluationSubmit:
		// Schema excludes subordinate assessments even though legacy hydration
		// remains available for tests and historical data.
		out, err := s.binding.hydrateOutput(map[string]any{"evaluations": []any{args}, "plans": []any{}})
		if err != nil {
			return nil, live, err
		}
		candidate := out.Evaluations[0]
		candidate.ExpectedRevision = live.Revision
		if candidate.RelationshipConfirmation != nil {
			candidate.RelationshipConfirmation.relationshipID, candidate.RelationshipConfirmation.relationshipRevision = entry.RelationshipID, entry.RelationshipRevision
		}
		if candidate.Review != nil && candidate.Review.Decision == "adjust" {
			if _, ok := records[goalSubmissionKey(goalPlanSubmit, entry.GoalID, "")]; !ok {
				return nil, live, errors.New("goal_review_adjust_plan_required")
			}
		}
		next, err = a.commitGoalEvaluationTx(ctx, tx, live, candidate, sources, s.binding.snapshot.RequestID)
		if err != nil {
			return nil, live, err
		}
		for _, review := range s.binding.snapshot.Reviews {
			if review.GoalID != entry.GoalID {
				continue
			}
			if err := commitGoalReviewTx(ctx, tx, entry.Goal, review, candidate.Review, sources, candidate.NextStep, candidate.NextReviewAt); err != nil {
				return nil, live, err
			}
			if candidate.Review != nil && ((next.Status == GoalActive && candidate.Review.Decision == "pause") || ((next.Status == GoalActive || next.Status == GoalPaused) && candidate.Review.Decision == "abandon")) {
				operation := GoalPause
				if candidate.Review.Decision == "abandon" {
					operation = GoalAbandon
				}
				updated, record, err := ApplyGoalCommand(&next, GoalCommand{Operation: operation, ExpectedRevision: next.Revision, EvidenceRefs: []string{"goal-review:" + review.ID}, Reason: candidate.Review.Explanation, OccurredAt: a.now()})
				if err != nil {
					return nil, live, err
				}
				if _, err := persistGoalAuthorityTx(ctx, tx, &next, updated, record, "review-decision:"+review.ID+fmt.Sprint(review.Revision)); err != nil {
					return nil, live, err
				}
				next = updated
			}
		}
		if next.Status == GoalActive && !goalHasNextState(next) {
			var scheduled bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_intentions WHERE fluctlight_id=$1 AND goal_id=$2 AND status IN ('qualified','due','in_progress') AND expiration>$3)`, next.FluctlightID, next.EntityID, a.now()).Scan(&scheduled); err != nil {
				return nil, live, err
			}
			if !scheduled {
				return nil, live, errors.New("goal_assessment_next_step_missing")
			}
		}
	case goalObjectSubmit:
		kind := stringValue(args["object_kind"])
		wire := cloneMap(args)
		delete(wire, "goal_ref")
		delete(wire, "object_kind")
		var candidate goalEvaluationWireObjectEvaluation
		if err := json.Unmarshal(jsonBytes(wire), &candidate); err != nil {
			return nil, live, errors.New("goal_evaluation_wire_output_invalid")
		}
		hydrated, err := s.binding.hydrateObjectEvaluation(entry.GoalID, kind, candidate)
		if err != nil {
			return nil, live, err
		}
		if err := applyGoalObjectEvaluationTx(ctx, tx, live, *hydrated, kind, sources, s.binding.snapshot.RequestID); err != nil {
			return nil, live, err
		}
		if err := persistGoalEvaluationEvidenceLinksTx(ctx, tx, live, goalEvidenceRefs(hydrated.Judgments), sources); err != nil {
			return nil, live, err
		}
		if kind == "stage" && hydrated.Completed && live.Status == GoalActive && live.CurrentStageID == hydrated.ID {
			next.CurrentStageID = ""
			next.Revision++
			record := GoalGovernanceRecord{GoalRef: live.Ref, Operation: GoalUpdate, FromStatus: live.Status, ToStatus: live.Status, BaseRevision: live.Revision, Revision: next.Revision, EvidenceRefs: live.EvidenceRefs, Reason: "current stage completed", PolicyVersion: goalEvaluationPolicyVersion, OccurredAt: a.now()}
			if _, err := persistGoalAuthorityTx(ctx, tx, &live, next, record, "goal-stage-pointer:"+s.binding.snapshot.RequestID+":"+hydrated.ID); err != nil {
				return nil, live, err
			}
		}
		return map[string]any{"goal_ref": args["goal_ref"], "object_ref": args["object_ref"], "object_kind": kind, "assessment_accepted": true, "completed": hydrated.Completed && live.Status == GoalActive, "parent_status": next.Status, "replayed": false}, next, nil
	case goalPlanSubmit:
		if live.Status != GoalActive {
			return nil, live, errors.New("goal_submission_parent_not_active")
		}
		out, err := s.binding.hydrateOutput(map[string]any{"evaluations": []any{}, "plans": []any{args}})
		if err != nil {
			return nil, live, err
		}
		plan := out.Plans[0]
		plan.ExpectedRevision = live.Revision
		next, err = applyGoalPlanTx(ctx, tx, live, plan, s.binding.snapshot.RequestID, a.now().UTC())
		if err != nil {
			return nil, live, err
		}
	default:
		return nil, live, ErrUnauthorized
	}
	return map[string]any{"goal_ref": args["goal_ref"], "assessment_accepted": true, "status": next.Status, "progress": next.Progress, "ready_for_settlement": next.ExecutionHint["ready_for_settlement"] == true, "replayed": false}, next, nil
}

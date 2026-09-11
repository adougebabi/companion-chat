package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func applyReflectionGoalCandidatesV2Tx(ctx context.Context, tx pgx.Tx, fluctlightID string, proposal ReflectionProposalV2, plan ReflectionEvolutionPlan, evolution EvolutionContext, occurredAt time.Time) error {
	index := evolution.ReferenceIndex
	for _, candidatePlan := range plan.Candidates {
		if candidatePlan.Domain != EvolutionGoal || candidatePlan.Disposition != EvolutionAccepted {
			continue
		}
		candidate := proposal.GoalCandidates[candidatePlan.Index]
		operation := GoalLifecycleOperation(candidate.Operation)
		commandKey := fmt.Sprintf("reflection:%s:goal:%d", plan.ProposalID, candidatePlan.Index)
		if operation == GoalCreate {
			entityID := "goal_reflection_" + stableDigest(commandKey)
			goal := GoalAuthority{
				EntityID: entityID, SchemaVersion: goalAuthoritySchemaVersion, Ref: "goal:ctx_" + stableDigest(entityID),
				FluctlightID: fluctlightID, ProfileID: activeEvolutionProfile(index), DesiredOutcome: candidate.DesiredOutcome,
				SuccessCriteria: append([]string(nil), candidate.SuccessCriteria...), Motivation: candidate.Motivation, Scope: "general",
				Importance: clampUnit(candidate.Strength), Urgency: clampUnit(candidate.Strength), Progress: 0,
				NeedsReflection: len(candidate.SuccessCriteria) == 0, Status: GoalActive, Revision: 1, EvidenceRefs: candidate.EvidenceRefs,
			}
			created, record, err := CreateGoalAuthority(goal, candidate.EvidenceRefs, occurredAt)
			if err != nil {
				return err
			}
			if _, err := persistGoalAuthorityTx(ctx, tx, nil, created, record, commandKey); err != nil {
				return err
			}
			continue
		}
		entry, ok := index.ByRef[candidate.TargetRef]
		if !ok || entry.Kind != ContextReferenceGoal {
			return errors.New("reflection_goal_target_invalid")
		}
		current, err := loadGoalAuthorityTx(ctx, tx, fluctlightID, candidate.TargetRef, entry)
		if err != nil {
			return err
		}
		patch := GoalPatch{}
		if candidate.DesiredOutcome != "" {
			value := candidate.DesiredOutcome
			patch.DesiredOutcome = &value
		}
		if candidate.SuccessCriteria != nil {
			patch.SuccessCriteria = append([]string(nil), candidate.SuccessCriteria...)
		}
		if candidate.Motivation != "" {
			value := candidate.Motivation
			patch.Motivation = &value
		}
		var next GoalAuthority
		var record GoalGovernanceRecord
		if len(candidate.OutcomeRefs) > 0 {
			if operation != GoalUpdate && operation != GoalComplete {
				return errors.New("reflection_goal_progress_operation_invalid")
			}
			outcomes, outcomeErr := reflectionGoalOutcomes(candidate.OutcomeRefs, evolution)
			if outcomeErr != nil {
				return outcomeErr
			}
			next, record, err = ApplyGoalProgress(current, GoalProgressProposal{
				GoalRef: candidate.TargetRef, OutcomeRefs: candidate.OutcomeRefs, CriterionIndexes: candidate.CriterionIndexes,
				Strength: candidate.Strength, Confidence: candidate.Confidence,
				Complete: candidate.Complete || operation == GoalComplete, EvidenceRefs: candidate.EvidenceRefs, OccurredAt: occurredAt,
			}, outcomes)
			if err == nil {
				if patch.DesiredOutcome != nil {
					next.DesiredOutcome = *patch.DesiredOutcome
				}
				if patch.SuccessCriteria != nil {
					next.SuccessCriteria = append([]string(nil), patch.SuccessCriteria...)
					next.NeedsReflection = len(next.SuccessCriteria) == 0
				}
				if patch.Motivation != nil {
					next.Motivation = *patch.Motivation
				}
				err = next.Validate()
			}
		} else {
			command := GoalCommand{Operation: operation, ExpectedRevision: current.Revision, Patch: patch, EvidenceRefs: candidate.EvidenceRefs, Reason: candidate.SemanticReason, OccurredAt: occurredAt}
			next, record, err = ApplyGoalCommand(&current, command)
		}
		if err != nil {
			return err
		}
		if _, err := persistGoalAuthorityTx(ctx, tx, &current, next, record, commandKey); err != nil {
			return err
		}
	}
	return nil
}

func reflectionGoalOutcomes(refs []string, evolution EvolutionContext) (map[string]ActionOutcome, error) {
	byID := make(map[string]ActionOutcome, len(evolution.Outcomes))
	for _, outcome := range evolution.Outcomes {
		byID[outcome.ID] = outcome
	}
	result := make(map[string]ActionOutcome, len(refs))
	for _, ref := range refs {
		entry, ok := evolution.ReferenceIndex.ByRef[ref]
		if !ok || entry.Kind != ContextReferenceOutcome {
			return nil, errors.New("reflection_goal_outcome_ref_invalid")
		}
		outcome, ok := byID[entry.EntityID]
		if !ok {
			return nil, errors.New("reflection_goal_outcome_missing")
		}
		result[ref] = outcome
	}
	return result, nil
}

func applyReflectionIntentionCandidatesV2Tx(ctx context.Context, tx pgx.Tx, fluctlightID string, proposal ReflectionProposalV2, plan ReflectionEvolutionPlan, index ContextReferenceIndex, occurredAt time.Time) error {
	for _, candidatePlan := range plan.Candidates {
		if candidatePlan.Domain != EvolutionIntention || candidatePlan.Disposition != EvolutionAccepted {
			continue
		}
		candidate := proposal.IntentionCandidates[candidatePlan.Index]
		operation := IntentionLifecycleOperation(candidate.Operation)
		commandKey := fmt.Sprintf("reflection:%s:intention:%d", plan.ProposalID, candidatePlan.Index)
		if operation == IntentionCreate {
			goalEntry, ok := index.ByRef[candidate.GoalRef]
			if !ok || goalEntry.Kind != ContextReferenceGoal || candidate.TypedTrigger == nil || candidate.Expiration == nil {
				return errors.New("reflection_intention_create_invalid")
			}
			entityID := "intention_reflection_" + stableDigest(commandKey)
			intention := IntentionAuthority{
				EntityID: entityID, GoalEntityID: goalEntry.EntityID, SchemaVersion: intentionAuthoritySchemaVersion,
				Ref: "intention:ctx_" + stableDigest(entityID), FluctlightID: fluctlightID, ProfileID: activeEvolutionProfile(index), GoalRef: candidate.GoalRef,
				ActionIntent: candidate.ActionIntent, ExpectedOutcome: candidate.ExpectedOutcome, CapabilityConstraints: append([]string(nil), candidate.CapabilityConstraints...),
				Trigger: *candidate.TypedTrigger, PreferredTime: candidate.PreferredTime, Expiration: candidate.Expiration.UTC(), Confidence: candidate.Confidence,
				Status: IntentionCandidate, Revision: 1, EvidenceRefs: candidate.EvidenceRefs,
			}
			created, record, err := CreateIntentionAuthority(intention, candidate.EvidenceRefs, occurredAt)
			if err != nil {
				return err
			}
			if _, err := persistIntentionAuthorityTx(ctx, tx, nil, created, record, commandKey); err != nil {
				return err
			}
			continue
		}
		entry, ok := index.ByRef[candidate.TargetRef]
		if !ok || entry.Kind != ContextReferenceIntention {
			return errors.New("reflection_intention_target_invalid")
		}
		current, err := loadIntentionAuthorityTx(ctx, tx, fluctlightID, candidate.TargetRef, candidate.GoalRef, entry)
		if err != nil {
			return err
		}
		patch := IntentionPatch{}
		if candidate.ActionIntent != "" {
			value := candidate.ActionIntent
			patch.ActionIntent = &value
		}
		if candidate.ExpectedOutcome != "" {
			value := candidate.ExpectedOutcome
			patch.ExpectedOutcome = &value
		}
		if candidate.CapabilityConstraints != nil {
			patch.CapabilityConstraints = append([]string(nil), candidate.CapabilityConstraints...)
		}
		if candidate.TypedTrigger != nil {
			trigger := *candidate.TypedTrigger
			patch.Trigger = &trigger
		}
		patch.PreferredTime, patch.Expiration = candidate.PreferredTime, candidate.Expiration
		command := IntentionCommand{Operation: operation, ExpectedRevision: current.Revision, Patch: patch, EvidenceRefs: candidate.EvidenceRefs, Reason: candidate.SemanticReason, OccurredAt: occurredAt}
		next, record, err := ApplyIntentionCommand(current, command)
		if err != nil {
			return err
		}
		if _, err := persistIntentionAuthorityTx(ctx, tx, &current, next, record, commandKey); err != nil {
			return err
		}
	}
	return nil
}

func applyReflectionAffectProfileV2Tx(ctx context.Context, tx pgx.Tx, fluctlightID string, proposal ReflectionProposalV2, plan ReflectionEvolutionPlan) error {
	accepted := make([]EvolutionCandidatePlan, 0)
	for _, candidate := range plan.Candidates {
		if candidate.Domain == EvolutionAffectProfile && candidate.Disposition == EvolutionAccepted {
			accepted = append(accepted, candidate)
		}
	}
	if len(accepted) == 0 {
		return nil
	}
	var baselineRaw, decayRaw, regulationRaw, summaryRaw, evidenceRaw []byte
	var revision int
	if err := tx.QueryRow(ctx, `SELECT baseline_pad,decay_policy,regulation_policy,emotional_summary,evidence_refs,revision FROM public.fluctlight_affect_profiles WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&baselineRaw, &decayRaw, &regulationRaw, &summaryRaw, &evidenceRaw, &revision); err != nil {
		return err
	}
	base := plan.ExpectedRevisions[string(EvolutionAffectProfile)]
	if revision != base {
		return errors.New("reflection_affect_profile_revision_stale")
	}
	baseline, decay, regulation, summary := decodeObject(baselineRaw), decodeObject(decayRaw), decodeObject(regulationRaw), decodeObject(summaryRaw)
	evidence := decisionServiceRefValues(decodeArray(evidenceRaw))
	for _, candidatePlan := range accepted {
		before := map[string]any{"baseline_pad": cloneMap(baseline), "decay_policy": cloneMap(decay), "regulation_policy": cloneMap(regulation), "emotional_summary": cloneMap(summary)}
		candidateEvidence := candidatePlan.EvidenceRefs
		if candidatePlan.Operation == "summarize" {
			summary = map[string]any{
				"dominant_patterns": proposal.EmotionalSummary.DominantPatterns, "triggers": proposal.EmotionalSummary.Triggers,
				"recovery_patterns": proposal.EmotionalSummary.RecoveryPatterns, "conflicts": proposal.EmotionalSummary.Conflicts,
			}
		} else {
			candidate := proposal.AffectRecalibrationCandidates[candidatePlan.Index-1]
			if err := applyAffectProfileRecalibration(baseline, decay, regulation, candidate); err != nil {
				return err
			}
			candidateEvidence = candidate.EvidenceRefs
		}
		evidence = mergeStableRefs(evidence, candidateEvidence)
		nextRevision := revision + 1
		after := map[string]any{"baseline_pad": cloneMap(baseline), "decay_policy": cloneMap(decay), "regulation_policy": cloneMap(regulation), "emotional_summary": cloneMap(summary)}
		field := "affect.emotional_summary"
		if candidatePlan.Operation == "recalibrate" {
			field = "affect." + candidatePlan.FieldPath
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_evolution_revisions(id,fluctlight_id,field,base_revision,revision,candidate_type,before_value,after_value,evidence_refs,source_window,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'accepted')`, "affect_evolution_"+stableDigest(plan.ProposalID+"\x1f"+candidatePlan.CandidateID), fluctlightID, field, revision, nextRevision, candidatePlan.Operation, jsonBytes(before), jsonBytes(after), jsonBytes(candidateEvidence), plan.SourceWindow); err != nil {
			return err
		}
		revision = nextRevision
	}
	command, err := tx.Exec(ctx, `UPDATE public.fluctlight_affect_profiles SET baseline_pad=$2,decay_policy=$3,regulation_policy=$4,emotional_summary=$5,evidence_refs=$6,revision=$7,updated_at=now() WHERE fluctlight_id=$1 AND revision=$8`, fluctlightID, jsonBytes(baseline), jsonBytes(decay), jsonBytes(regulation), jsonBytes(summary), jsonBytes(evidence), revision, base)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return errors.New("reflection_affect_profile_revision_stale")
	}
	return nil
}

func applyAffectProfileRecalibration(baseline, decay, regulation map[string]any, candidate ReflectionAffectCandidateV2) error {
	sign := 0.0
	switch strings.TrimSpace(candidate.Direction) {
	case "increase", "strengthen", "toward":
		sign = 1
	case "decrease", "weaken", "away":
		sign = -1
	case "maintain":
		return nil
	default:
		return errors.New("reflection_affect_direction_invalid")
	}
	step := sign * 0.1 * candidate.Strength * candidate.Confidence
	target := strings.TrimSpace(candidate.Target)
	switch target {
	case "baseline.pleasure", "baseline.arousal", "baseline.dominance":
		key := strings.TrimPrefix(target, "baseline.")
		baseline[key] = clampBipolar(numberOrZero(baseline[key]) + step)
	case "regulation.strength":
		regulation["strength"] = clampUnit(numberOrZero(regulation["strength"]) + step)
	case "decay.pad_half_life_seconds", "decay.momentum_half_life_seconds", "decay.mood_half_life_seconds", "decay.drive_half_life_seconds":
		key := strings.TrimPrefix(target, "decay.")
		current := numberOrZero(decay[key])
		if current <= 0 {
			return errors.New("reflection_affect_decay_baseline_invalid")
		}
		next := current * (1 + sign*0.25*candidate.Strength*candidate.Confidence)
		if next < 60 {
			next = 60
		}
		if next > 604800 {
			next = 604800
		}
		decay[key] = next
	default:
		return errors.New("reflection_affect_target_invalid")
	}
	return nil
}

func loadGoalAuthorityTx(ctx context.Context, tx pgx.Tx, fluctlightID, ref string, entry ContextReference) (GoalAuthority, error) {
	var goal GoalAuthority
	var profileID *string
	var targetActorID *string
	var criteria, importance, urgency, progress, evidence []byte
	var deadline *time.Time
	err := tx.QueryRow(ctx, `SELECT id,COALESCE(profile_id,'default'),scope,target_actor_id,desired_outcome,success_criteria,motivation,needs_reflection,importance,urgency,progress,deadline,status,evidence_refs,revision FROM public.fluctlight_goals WHERE id=$1 AND fluctlight_id=$2 FOR UPDATE`, entry.EntityID, fluctlightID).Scan(&goal.EntityID, &profileID, &goal.Scope, &targetActorID, &goal.DesiredOutcome, &criteria, &goal.Motivation, &goal.NeedsReflection, &importance, &urgency, &progress, &deadline, &goal.Status, &evidence, &goal.Revision)
	if err != nil {
		return GoalAuthority{}, err
	}
	if goal.Revision != entry.Revision {
		return GoalAuthority{}, errors.New("reflection_goal_revision_stale")
	}
	goal.SchemaVersion, goal.Ref, goal.FluctlightID, goal.ProfileID = goalAuthoritySchemaVersion, ref, fluctlightID, "default"
	if profileID != nil && strings.TrimSpace(*profileID) != "" {
		goal.ProfileID = *profileID
	}
	if targetActorID != nil {
		goal.TargetActorID = *targetActorID
	}
	goal.SuccessCriteria, goal.Importance, goal.Urgency, goal.Progress, goal.Deadline, goal.EvidenceRefs = decisionServiceRefValues(decodeArray(criteria)), numberOrZero(jsonNumber(importance)), numberOrZero(jsonNumber(urgency)), numberOrZero(jsonNumber(progress)), deadline, decisionServiceRefValues(decodeArray(evidence))
	return goal, goal.Validate()
}

func loadIntentionAuthorityTx(ctx context.Context, tx pgx.Tx, fluctlightID, ref, goalRef string, entry ContextReference) (IntentionAuthority, error) {
	var intention IntentionAuthority
	var profileID, goalID *string
	var constraints, triggerRaw, confidenceRaw, evidence []byte
	var preferred *time.Time
	err := tx.QueryRow(ctx, `SELECT id,profile_id,goal_id,action_intent,expected_outcome,capability_constraints,preferred_time,trigger,confidence,expiration,status,revision,evidence_refs,COALESCE(current_attempt_id,'') FROM public.fluctlight_intentions WHERE id=$1 AND fluctlight_id=$2 FOR UPDATE`, entry.EntityID, fluctlightID).Scan(&intention.EntityID, &profileID, &goalID, &intention.ActionIntent, &intention.ExpectedOutcome, &constraints, &preferred, &triggerRaw, &confidenceRaw, &intention.Expiration, &intention.Status, &intention.Revision, &evidence, &intention.LastAttemptID)
	if err != nil {
		return IntentionAuthority{}, err
	}
	if intention.Revision != entry.Revision {
		return IntentionAuthority{}, errors.New("reflection_intention_revision_stale")
	}
	intention.SchemaVersion, intention.Ref, intention.FluctlightID, intention.ProfileID, intention.GoalRef = intentionAuthoritySchemaVersion, ref, fluctlightID, "default", goalRef
	if profileID != nil && strings.TrimSpace(*profileID) != "" {
		intention.ProfileID = *profileID
	}
	if goalID != nil {
		intention.GoalEntityID = *goalID
	}
	intention.CapabilityConstraints, intention.PreferredTime, intention.Confidence, intention.EvidenceRefs = decisionServiceRefValues(decodeArray(constraints)), preferred, numberOrZero(jsonNumber(confidenceRaw)), decisionServiceRefValues(decodeArray(evidence))
	if err := json.Unmarshal(triggerRaw, &intention.Trigger); err != nil {
		return IntentionAuthority{}, err
	}
	return intention, intention.Validate()
}

func activeEvolutionProfile(index ContextReferenceIndex) string {
	if value := strings.TrimSpace(index.ActiveProfileID); value != "" {
		return value
	}
	return "default"
}

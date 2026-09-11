package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

func persistGoalAuthorityTx(ctx context.Context, tx pgx.Tx, before *GoalAuthority, after GoalAuthority, record GoalGovernanceRecord, commandKey string) (bool, error) {
	if tx == nil || strings.TrimSpace(after.EntityID) == "" || strings.TrimSpace(commandKey) == "" {
		return false, errors.New("goal_persistence_identity_invalid")
	}
	if err := after.Validate(); err != nil {
		return false, err
	}
	digest := stableDigest(jsonString(after))
	var storedDigest string
	if err := tx.QueryRow(ctx, `SELECT request_digest FROM public.fluctlight_goal_revisions WHERE idempotency_key=$1`, commandKey).Scan(&storedDigest); err == nil {
		if storedDigest != digest {
			return false, errors.New("goal_command_idempotency_conflict")
		}
		return true, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if record.GoalRef != after.Ref || record.Revision != after.Revision || record.EvidenceRefs == nil {
		return false, errors.New("goal_governance_record_invalid")
	}
	if before == nil {
		if record.Operation != GoalCreate || record.BaseRevision != 0 || after.Revision != 1 {
			return false, errors.New("goal_create_persistence_invalid")
		}
		command, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_goals(id,fluctlight_id,profile_id,source,scope,target_actor_id,description,desired_outcome,success_criteria,motivation,needs_reflection,importance,urgency,progress,deadline,status,evidence_refs,revision,idempotency_key,request_digest) VALUES($1,$2,$3,'evolution',$4,$5,$6,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`, after.EntityID, after.FluctlightID, after.ProfileID, after.Scope, nullableString(after.TargetActorID), after.DesiredOutcome, jsonBytes(stringSliceAny(after.SuccessCriteria)), after.Motivation, after.NeedsReflection, jsonBytes(after.Importance), jsonBytes(after.Urgency), jsonBytes(after.Progress), after.Deadline, after.Status, jsonBytes(stringSliceAny(after.EvidenceRefs)), after.Revision, commandKey, digest)
		if err != nil {
			return false, err
		}
		if command.RowsAffected() != 1 {
			return false, ErrConflict
		}
	} else {
		if before.EntityID != after.EntityID || before.Ref != after.Ref || record.BaseRevision != before.Revision || after.Revision != before.Revision+1 {
			return false, errors.New("goal_update_persistence_invalid")
		}
		command, err := tx.Exec(ctx, `UPDATE public.fluctlight_goals SET profile_id=$2,scope=$3,target_actor_id=$4,description=$5,desired_outcome=$5,success_criteria=$6,motivation=$7,needs_reflection=$8,importance=$9,urgency=$10,progress=$11,deadline=$12,status=$13,evidence_refs=$14,revision=$15,updated_at=now() WHERE id=$1 AND fluctlight_id=$16 AND revision=$17`, after.EntityID, after.ProfileID, after.Scope, nullableString(after.TargetActorID), after.DesiredOutcome, jsonBytes(stringSliceAny(after.SuccessCriteria)), after.Motivation, after.NeedsReflection, jsonBytes(after.Importance), jsonBytes(after.Urgency), jsonBytes(after.Progress), after.Deadline, after.Status, jsonBytes(stringSliceAny(after.EvidenceRefs)), after.Revision, after.FluctlightID, before.Revision)
		if err != nil {
			return false, err
		}
		if command.RowsAffected() != 1 {
			return false, errors.New("goal_revision_conflict")
		}
	}
	revisionID := "goal_revision_" + stableDigest(commandKey)
	_, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_goal_revisions(id,goal_id,fluctlight_id,from_status,to_status,actor_id,reason,operation,base_revision,snapshot,evidence_refs,idempotency_key,policy_version,request_digest) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, revisionID, after.EntityID, after.FluctlightID, record.FromStatus, record.ToStatus, after.FluctlightID, nullableString(record.Reason), record.Operation, record.BaseRevision, jsonBytes(after), jsonBytes(record.EvidenceRefs), commandKey, record.PolicyVersion, digest)
	return false, err
}

func persistIntentionAuthorityTx(ctx context.Context, tx pgx.Tx, before *IntentionAuthority, after IntentionAuthority, record IntentionGovernanceRecord, commandKey string) (bool, error) {
	if tx == nil || strings.TrimSpace(after.EntityID) == "" || strings.TrimSpace(commandKey) == "" {
		return false, errors.New("intention_persistence_identity_invalid")
	}
	if err := after.Validate(); err != nil {
		return false, err
	}
	digest := stableDigest(jsonString(after))
	var storedDigest string
	if err := tx.QueryRow(ctx, `SELECT request_digest FROM public.fluctlight_intention_revisions WHERE idempotency_key=$1`, commandKey).Scan(&storedDigest); err == nil {
		if storedDigest != digest {
			return false, errors.New("intention_command_idempotency_conflict")
		}
		return true, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if record.IntentionRef != after.Ref || record.Revision != after.Revision {
		return false, errors.New("intention_governance_record_invalid")
	}
	if before == nil {
		if record.Operation != IntentionCreate || record.BaseRevision != 0 || after.Revision != 1 {
			return false, errors.New("intention_create_persistence_invalid")
		}
		command, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_intentions(id,fluctlight_id,profile_id,goal_id,action,action_intent,expected_outcome,capability_constraints,preferred_time,trigger,confidence,expiration,evidence_refs,permission_snapshot,budget_snapshot,status,revision,current_attempt_id,idempotency_key,request_digest) VALUES($1,$2,$3,$4,$5::varchar,$5::text,$6,$7,$8,$9,$10,$11,$12,'{}','{}',$13,$14,$15,$16,$17)`, after.EntityID, after.FluctlightID, after.ProfileID, nullableString(after.GoalEntityID), after.ActionIntent, after.ExpectedOutcome, jsonBytes(stringSliceAny(after.CapabilityConstraints)), after.PreferredTime, jsonBytes(after.Trigger), jsonBytes(after.Confidence), after.Expiration, jsonBytes(stringSliceAny(after.EvidenceRefs)), after.Status, after.Revision, nullableString(after.LastAttemptID), commandKey, digest)
		if err != nil {
			return false, fmt.Errorf("persist Intention create status=%s revision=%d trigger_type=%s constraints=%d digest_length=%d: %w", after.Status, after.Revision, after.Trigger.Type, len(after.CapabilityConstraints), len(digest), err)
		}
		if command.RowsAffected() != 1 {
			return false, ErrConflict
		}
	} else {
		if before.EntityID != after.EntityID || before.Ref != after.Ref || record.BaseRevision != before.Revision || after.Revision != before.Revision+1 {
			return false, errors.New("intention_update_persistence_invalid")
		}
		command, err := tx.Exec(ctx, `UPDATE public.fluctlight_intentions SET profile_id=$2,goal_id=$3,action=$4::varchar,action_intent=$4::text,expected_outcome=$5,capability_constraints=$6,preferred_time=$7,trigger=$8,confidence=$9,expiration=$10,evidence_refs=$11,status=$12,revision=$13,current_attempt_id=$14,updated_at=now() WHERE id=$1 AND fluctlight_id=$15 AND revision=$16`, after.EntityID, after.ProfileID, nullableString(after.GoalEntityID), after.ActionIntent, after.ExpectedOutcome, jsonBytes(stringSliceAny(after.CapabilityConstraints)), after.PreferredTime, jsonBytes(after.Trigger), jsonBytes(after.Confidence), after.Expiration, jsonBytes(stringSliceAny(after.EvidenceRefs)), after.Status, after.Revision, nullableString(after.LastAttemptID), after.FluctlightID, before.Revision)
		if err != nil {
			return false, err
		}
		if command.RowsAffected() != 1 {
			return false, errors.New("intention_revision_conflict")
		}
	}
	revisionID := "intention_revision_" + stableDigest(commandKey)
	_, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_intention_revisions(id,intention_id,fluctlight_id,from_status,to_status,actor_id,reason,operation,base_revision,snapshot,evidence_refs,idempotency_key,policy_version,request_digest) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, revisionID, after.EntityID, after.FluctlightID, record.FromStatus, record.ToStatus, after.FluctlightID, nullableString(record.Reason), record.Operation, record.BaseRevision, jsonBytes(after), jsonBytes(record.EvidenceRefs), commandKey, record.PolicyVersion, digest)
	if err != nil {
		return false, err
	}
	if err := syncIntentionTriggerWorkflowTx(ctx, tx, after); err != nil {
		return false, err
	}
	return false, nil
}

func persistIntentionDueFactTx(ctx context.Context, tx pgx.Tx, app *App, intention IntentionAuthority, due IntentionDueFact) (IntentionAuthority, IntentionDueFact, string, bool, error) {
	if tx == nil || app == nil || strings.TrimSpace(intention.EntityID) == "" || due.EventType != intentionDueFactType || due.IntentionRef != intention.Ref || due.AttemptID == "" {
		return IntentionAuthority{}, IntentionDueFact{}, "", false, errors.New("intention_due_persistence_invalid")
	}
	var revision int
	var status, currentAttempt string
	if err := tx.QueryRow(ctx, `SELECT revision,status,COALESCE(current_attempt_id,'') FROM public.fluctlight_intentions WHERE id=$1 AND fluctlight_id=$2 FOR UPDATE`, intention.EntityID, intention.FluctlightID).Scan(&revision, &status, &currentAttempt); err != nil {
		return IntentionAuthority{}, IntentionDueFact{}, "", false, err
	}
	replayed := status == string(IntentionDue) && currentAttempt == due.AttemptID
	next := intention
	if !replayed {
		if revision != intention.Revision || status != string(IntentionQualified) {
			return IntentionAuthority{}, IntentionDueFact{}, "", false, errors.New("intention_due_revision_conflict")
		}
		next.Status = IntentionDue
		next.Revision++
		next.LastAttemptID = due.AttemptID
		command, err := tx.Exec(ctx, `UPDATE public.fluctlight_intentions SET status='due',revision=$3,current_attempt_id=$4,updated_at=now() WHERE id=$1 AND fluctlight_id=$2 AND revision=$5 AND status='qualified'`, intention.EntityID, intention.FluctlightID, next.Revision, due.AttemptID, intention.Revision)
		if err != nil || command.RowsAffected() != 1 {
			if err == nil {
				err = errors.New("intention_due_revision_conflict")
			}
			return IntentionAuthority{}, IntentionDueFact{}, "", false, err
		}
		digest := stableDigest(jsonString(next))
		_, err = tx.Exec(ctx, `INSERT INTO public.fluctlight_intention_revisions(id,intention_id,fluctlight_id,from_status,to_status,actor_id,reason,operation,base_revision,snapshot,evidence_refs,idempotency_key,policy_version,request_digest) VALUES($1,$2,$3,'qualified','due',$3,'typed trigger became due','mark_due',$4,$5,$6,$7,$8,$9)`, "intention_revision_"+stableDigest(due.ID), intention.EntityID, intention.FluctlightID, intention.Revision, jsonBytes(next), jsonBytes(next.EvidenceRefs), due.ID, goalProgressPolicyVersion, digest)
		if err != nil {
			return IntentionAuthority{}, IntentionDueFact{}, "", false, err
		}
	} else {
		next.Status = IntentionDue
		next.Revision = revision
		next.LastAttemptID = due.AttemptID
	}
	due.IntentionRevision = next.Revision
	payload := map[string]any{"intention_ref": next.Ref, "goal_ref": next.GoalRef, "intention_revision": next.Revision, "attempt_id": due.AttemptID, "trigger_type": due.TriggerType, "source_ref": due.SourceRef}
	inboxID, err := app.enqueueNativeFactTx(ctx, tx, next.FluctlightID, "", next.EntityID, intentionDueFactType, due.ID, payload)
	return next, due, inboxID, replayed, err
}

func persistIntentionAttemptSettlementTx(ctx context.Context, tx pgx.Tx, before IntentionAuthority, settlement IntentionAttemptSettlement) (bool, error) {
	if tx == nil || strings.TrimSpace(before.EntityID) == "" || settlement.Attempt.AttemptID == "" || settlement.Intention.EntityID != before.EntityID {
		return false, errors.New("intention_attempt_persistence_invalid")
	}
	var digest, status string
	if err := tx.QueryRow(ctx, `SELECT outcome_digest,status FROM public.fluctlight_intention_attempts WHERE attempt_id=$1`, settlement.Attempt.AttemptID).Scan(&digest, &status); err == nil {
		if digest != settlement.Attempt.OutcomeDigest || status != string(settlement.Attempt.Status) {
			return false, errors.New("intention_attempt_replay_conflict")
		}
		return true, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	var currentRevision int
	if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_intentions WHERE id=$1 AND fluctlight_id=$2 FOR UPDATE`, before.EntityID, before.FluctlightID).Scan(&currentRevision); err != nil {
		return false, err
	}
	if currentRevision != before.Revision || settlement.Intention.Revision != before.Revision+1 {
		return false, errors.New("intention_attempt_revision_conflict")
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_intention_attempts(attempt_id,fluctlight_id,intention_ref,goal_ref,action_id,outcome_id,outcome_digest,status,result,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, settlement.Attempt.AttemptID, before.FluctlightID, before.Ref, before.GoalRef, settlement.Attempt.ActionID, settlement.Attempt.OutcomeID, settlement.Attempt.OutcomeDigest, settlement.Attempt.Status, jsonBytes(settlement), settlement.Attempt.OccurredAt); err != nil {
		return false, err
	}
	command, err := tx.Exec(ctx, `UPDATE public.fluctlight_intentions SET status=$3,revision=$4,current_attempt_id=$5,evidence_refs=$6,updated_at=now() WHERE id=$1 AND fluctlight_id=$2 AND revision=$7`, before.EntityID, before.FluctlightID, settlement.Intention.Status, settlement.Intention.Revision, settlement.Attempt.AttemptID, jsonBytes(settlement.Intention.EvidenceRefs), before.Revision)
	if err != nil || command.RowsAffected() != 1 {
		if err == nil {
			err = errors.New("intention_attempt_revision_conflict")
		}
		return false, err
	}
	operation := IntentionUpdate
	if settlement.Intention.Status == IntentionCompleted {
		operation = IntentionComplete
	} else if settlement.Intention.Status == IntentionCancelled {
		operation = IntentionCancel
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.fluctlight_intention_revisions(id,intention_id,fluctlight_id,from_status,to_status,actor_id,reason,operation,base_revision,snapshot,evidence_refs,idempotency_key,policy_version,request_digest) VALUES($1,$2,$3,$4,$5,$3,'ActionOutcome settlement',$6,$7,$8,$9,$10,$11,$12)`, "intention_revision_"+stableDigest(settlement.Attempt.AttemptID), before.EntityID, before.FluctlightID, before.Status, settlement.Intention.Status, operation, before.Revision, jsonBytes(settlement.Intention), jsonBytes(settlement.Intention.EvidenceRefs), settlement.Attempt.AttemptID, goalProgressPolicyVersion, settlement.Attempt.OutcomeDigest)
	if err != nil {
		return false, err
	}
	if err := syncIntentionTriggerWorkflowTx(ctx, tx, settlement.Intention); err != nil {
		return false, err
	}
	return false, nil
}

func persistReflectionApplyTx(ctx context.Context, tx pgx.Tx, evolution EvolutionContext, proposal ReflectionProposalV2, plan ReflectionEvolutionPlan, next EvolutionState, result ReflectionApplyResult, profileRef, modelVersion, promptVersion string) (bool, error) {
	if tx == nil || plan.ProposalID == "" || plan.FluctlightID != evolution.FluctlightID || next.Watermark != plan.ToSequence || result.ProposalID != plan.ProposalID || !contextReferencePattern.MatchString(profileRef) {
		return false, errors.New("reflection_persistence_identity_invalid")
	}
	digest := stableDigest(jsonString(map[string]any{"context": evolution, "proposal": proposal, "plan": plan}))
	var storedDigest string
	if err := tx.QueryRow(ctx, `SELECT request_digest FROM public.cognition_reflection_proposals WHERE id=$1`, plan.ProposalID).Scan(&storedDigest); err == nil {
		if storedDigest != digest {
			return false, errors.New("reflection_proposal_replay_conflict")
		}
		return true, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.cognition_reflection_windows(fluctlight_id,watermark,state_revision,status,evolution_revisions) VALUES($1,$2,0,'idle',$3) ON CONFLICT(fluctlight_id) DO NOTHING`, evolution.FluctlightID, plan.BaseWatermark, jsonBytes(plan.ExpectedRevisions)); err != nil {
		return false, err
	}
	var watermark int
	var rawRevisions []byte
	if err := tx.QueryRow(ctx, `SELECT watermark,evolution_revisions FROM public.cognition_reflection_windows WHERE fluctlight_id=$1 FOR UPDATE`, evolution.FluctlightID).Scan(&watermark, &rawRevisions); err != nil {
		return false, err
	}
	if len(decodeObject(rawRevisions)) == 0 {
		rawRevisions = jsonBytes(plan.ExpectedRevisions)
		if _, err := tx.Exec(ctx, `UPDATE public.cognition_reflection_windows SET evolution_revisions=$2 WHERE fluctlight_id=$1`, evolution.FluctlightID, rawRevisions); err != nil {
			return false, err
		}
	}
	if watermark != plan.BaseWatermark || !jsonEqual(rawRevisions, plan.ExpectedRevisions) {
		return false, errors.New("reflection_apply_window_stale")
	}
	baseStateRevision := plan.ExpectedRevisions[string(EvolutionAffectProfile)]
	status := result.Status
	if status != "applied" && status != "no_change" {
		return false, errors.New("reflection_result_status_invalid")
	}
	_, err := tx.Exec(ctx, `INSERT INTO public.cognition_reflection_proposals(id,fluctlight_id,from_sequence,to_sequence,base_state_revision,payload,evidence_refs,correlation_id,status,schema_version,context_snapshot,expected_revisions,result,model_version,prompt_version,policy_version,request_digest,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`, plan.ProposalID, plan.FluctlightID, evolution.FromSequence, evolution.ToSequence, baseStateRevision, jsonBytes(proposal), jsonBytes(reflectionEvolutionEvidenceRefs(evolution)), "reflection:"+plan.SourceWindow, status, reflectionProposalV2SchemaVersion, jsonBytes(evolution), jsonBytes(plan.ExpectedRevisions), jsonBytes(result), strings.TrimSpace(modelVersion), strings.TrimSpace(promptVersion), reflectionPolicyVersion, digest, plan.ProposalID)
	if err != nil {
		return false, err
	}
	for _, candidate := range plan.Candidates {
		base := plan.ExpectedRevisions[string(candidate.Domain)]
		resulting := result.Revisions[string(candidate.Domain)]
		changedRef := ""
		if candidate.Disposition == EvolutionAccepted {
			changedRef = candidate.TargetRef
		}
		_, err := tx.Exec(ctx, `INSERT INTO public.cognition_reflection_candidate_dispositions(id,proposal_id,fluctlight_id,domain,candidate_id,candidate_index,disposition,reason_code,target_ref,changed_ref,base_revision,resulting_revision,evidence_refs) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, "reflection_disposition_"+stableDigest(plan.ProposalID+"\x1f"+candidate.CandidateID), plan.ProposalID, plan.FluctlightID, candidate.Domain, candidate.CandidateID, candidate.Index, candidate.Disposition, candidate.ReasonCode, nullableString(candidate.TargetRef), nullableString(changedRef), base, resulting, jsonBytes(candidate.EvidenceRefs))
		if err != nil {
			return false, err
		}
	}
	command, err := tx.Exec(ctx, `UPDATE public.cognition_reflection_windows SET watermark=$2,evolution_revisions=$3,status='idle',updated_at=now() WHERE fluctlight_id=$1 AND watermark=$4 AND evolution_revisions=$5`, plan.FluctlightID, next.Watermark, jsonBytes(next.Revisions), plan.BaseWatermark, jsonBytes(plan.ExpectedRevisions))
	if err != nil || command.RowsAffected() != 1 {
		if err == nil {
			err = errors.New("reflection_apply_window_stale")
		}
		return false, err
	}
	profileID := evolution.ReferenceIndex.ActiveProfileID
	if strings.TrimSpace(profileID) == "" {
		profileID = "default"
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.fluctlight_evolution_states(fluctlight_id,profile_id,profile_ref,revision,domain_revisions) VALUES($1,$2,$3,$4,$5) ON CONFLICT(fluctlight_id,profile_id) DO UPDATE SET profile_ref=EXCLUDED.profile_ref,revision=GREATEST(fluctlight_evolution_states.revision,EXCLUDED.revision),domain_revisions=fluctlight_evolution_states.domain_revisions || EXCLUDED.domain_revisions,updated_at=now()`, plan.FluctlightID, profileID, profileRef, maxDomainRevision(next.Revisions), jsonBytes(next.Revisions))
	return false, err
}

func persistEvolutionOverlayTx(ctx context.Context, tx pgx.Tx, before, after PersonaEvolutionState, overlay EvolutionOverlay) (bool, error) {
	if tx == nil || overlay.ID == "" || overlay.BaseRevision != before.Revision || overlay.Revision != after.Revision || after.Revision != before.Revision+1 {
		return false, errors.New("evolution_overlay_persistence_invalid")
	}
	var storedRef string
	var storedRevision int
	if err := tx.QueryRow(ctx, `SELECT ref,revision FROM public.fluctlight_evolution_overlays WHERE id=$1`, overlay.ID).Scan(&storedRef, &storedRevision); err == nil {
		if storedRef != overlay.Ref || storedRevision != overlay.Revision {
			return false, errors.New("evolution_overlay_replay_conflict")
		}
		return true, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_evolution_states(fluctlight_id,profile_id,profile_ref,revision,domain_revisions) VALUES($1,$2,$3,$4,'{}') ON CONFLICT(fluctlight_id,profile_id) DO NOTHING`, before.FluctlightID, before.ProfileID, before.ProfileRef, before.Revision); err != nil {
		return false, err
	}
	var currentRevision int
	if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_evolution_states WHERE fluctlight_id=$1 AND profile_id=$2 FOR UPDATE`, before.FluctlightID, before.ProfileID).Scan(&currentRevision); err != nil {
		return false, err
	}
	if currentRevision != before.Revision {
		return false, errors.New("evolution_overlay_revision_conflict")
	}
	if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_evolution_overlays SET status='superseded' WHERE fluctlight_id=$1 AND profile_id=$2 AND kind=$3 AND field_path=$4 AND status='active'`, overlay.FluctlightID, overlay.ProfileID, overlay.Kind, overlay.FieldPath); err != nil {
		return false, err
	}
	var requestedDelta, appliedDelta any
	if overlay.ValueKind == OverlayNumeric {
		requestedDelta, appliedDelta = overlay.RequestedDelta, overlay.AppliedDelta
	}
	_, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_evolution_overlays(id,ref,fluctlight_id,profile_id,kind,field_path,value_kind,semantic_direction,requested_delta,applied_delta,before_value,after_value,confidence,evidence_refs,evidence_windows,policy_version,base_revision,revision,status,supersedes,rollback_of,cooldown_until,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)`, overlay.ID, overlay.Ref, overlay.FluctlightID, overlay.ProfileID, overlay.Kind, overlay.FieldPath, overlay.ValueKind, overlay.SemanticDirection, requestedDelta, appliedDelta, jsonBytes(overlay.BeforeValue), jsonBytes(overlay.AfterValue), overlay.Confidence, jsonBytes(overlay.EvidenceRefs), jsonBytes(overlay.EvidenceWindows), overlay.PolicyVersion, overlay.BaseRevision, overlay.Revision, overlay.Status, nullableString(overlay.Supersedes), nullableString(overlay.RollbackOf), overlay.CooldownUntil, overlay.CreatedAt)
	if err != nil {
		return false, err
	}
	domainRevisions := map[string]int{string(overlay.Kind): overlay.Revision}
	command, err := tx.Exec(ctx, `UPDATE public.fluctlight_evolution_states SET profile_ref=$3,revision=$4,domain_revisions=domain_revisions || $5::jsonb,updated_at=now() WHERE fluctlight_id=$1 AND profile_id=$2 AND revision=$6`, overlay.FluctlightID, overlay.ProfileID, after.ProfileRef, after.Revision, jsonBytes(domainRevisions), before.Revision)
	if err != nil || command.RowsAffected() != 1 {
		if err == nil {
			err = errors.New("evolution_overlay_revision_conflict")
		}
		return false, err
	}
	return false, nil
}

func loadPersonaEvolutionState(ctx context.Context, query scheduleQuerier, baseline PersonaEvolutionState) (PersonaEvolutionState, error) {
	result := clonePersonaEvolutionState(baseline)
	var rawRevisions []byte
	if err := query.QueryRow(ctx, `SELECT profile_ref,revision,domain_revisions FROM public.fluctlight_evolution_states WHERE fluctlight_id=$1 AND profile_id=$2`, baseline.FluctlightID, baseline.ProfileID).Scan(&result.ProfileRef, &result.Revision, &rawRevisions); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return result, nil
		}
		return PersonaEvolutionState{}, err
	}
	rows, err := query.Query(ctx, `SELECT id,ref,kind,field_path,value_kind,semantic_direction,requested_delta,applied_delta,before_value,after_value,confidence,evidence_refs,evidence_windows,policy_version,base_revision,revision,status,COALESCE(supersedes,''),COALESCE(rollback_of,''),cooldown_until,created_at FROM public.fluctlight_evolution_overlays WHERE fluctlight_id=$1 AND profile_id=$2 ORDER BY revision`, baseline.FluctlightID, baseline.ProfileID)
	if err != nil {
		return PersonaEvolutionState{}, err
	}
	defer rows.Close()
	result.Overlays = nil
	for rows.Next() {
		var overlay EvolutionOverlay
		var requested, applied *float64
		var beforeRaw, afterRaw, evidenceRaw, windowsRaw []byte
		if err := rows.Scan(&overlay.ID, &overlay.Ref, &overlay.Kind, &overlay.FieldPath, &overlay.ValueKind, &overlay.SemanticDirection, &requested, &applied, &beforeRaw, &afterRaw, &overlay.Confidence, &evidenceRaw, &windowsRaw, &overlay.PolicyVersion, &overlay.BaseRevision, &overlay.Revision, &overlay.Status, &overlay.Supersedes, &overlay.RollbackOf, &overlay.CooldownUntil, &overlay.CreatedAt); err != nil {
			return PersonaEvolutionState{}, err
		}
		overlay.FluctlightID, overlay.ProfileID = baseline.FluctlightID, baseline.ProfileID
		if requested != nil {
			overlay.RequestedDelta = *requested
		}
		if applied != nil {
			overlay.AppliedDelta = *applied
		}
		overlay.BeforeValue, overlay.AfterValue = decodeJSONValue(beforeRaw), decodeJSONValue(afterRaw)
		overlay.EvidenceRefs, overlay.EvidenceWindows = decisionServiceRefValues(decodeArray(evidenceRaw)), decisionServiceRefValues(decodeArray(windowsRaw))
		result.Overlays = append(result.Overlays, overlay)
	}
	return result, rows.Err()
}

func reflectionEvolutionEvidenceRefs(context EvolutionContext) []string {
	refs := make([]string, 0, len(context.Evidence))
	for _, evidence := range context.Evidence {
		refs = append(refs, evidence.Ref)
	}
	return mergeStableRefs(refs)
}

func maxDomainRevision(revisions map[string]int) int {
	maximum := 0
	for _, revision := range revisions {
		if revision > maximum {
			maximum = revision
		}
	}
	return maximum
}

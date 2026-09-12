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

func compactReflectionEvidenceV2(evidence []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(evidence))
	for _, item := range evidence {
		eventType := boundedReflectionScalarText(item["event_type"], 128)
		entry := map[string]any{"event_type": eventType, "evidence_ref": "sequence:" + fmt.Sprint(item["sequence"])}
		if occurredAt, ok := item["occurred_at"].(time.Time); ok && !occurredAt.IsZero() {
			entry["occurred_at"] = occurredAt.UTC().Format(time.RFC3339Nano)
		}
		payload := mapValue(item["payload"])
		switch eventType {
		case "conversation.turn":
			if observation := boundedReflectionObservation(stringValue(payload["text"])); observation != "" {
				entry["observation"] = observation
			}
		case "autonomy.result":
			if outcomes := compactReflectionEvidencePayload(eventType, payload); !isEmptyReflectionProviderValue(outcomes) {
				entry["action_outcomes"] = outcomes
			}
		default:
			semantic := map[string]any{}
			for _, key := range []string{"kind", "scene", "activity", "location", "current_task", "user_presence", "summary", "status", "reason_code"} {
				if value := boundedReflectionScalar(payload[key], 2000); value != nil {
					semantic[key] = value
				}
			}
			if len(semantic) > 0 {
				entry["observation"] = semantic
			}
		}
		if appraisal := compactReflectionAppraisalV2(mapValue(item["appraisal"])); len(appraisal) > 0 {
			entry["appraisal"] = appraisal
		}
		result = append(result, entry)
	}
	return result
}

func boundedReflectionScalar(value any, maxRunes int) any {
	switch typed := value.(type) {
	case string:
		if text := boundedReflectionScalarText(typed, maxRunes); text != "" {
			return text
		}
	case bool:
		return typed
	case float64:
		if unitFinite(typed) {
			return typed
		}
	case int:
		return typed
	case json.Number:
		if parsed, err := typed.Float64(); err == nil && unitFinite(parsed) {
			return parsed
		}
	}
	return nil
}

func boundedReflectionScalarText(value any, maxRunes int) string {
	text := strings.TrimSpace(stringValue(value))
	if text == "" || maxRunes < 1 {
		return ""
	}
	if runes := []rune(text); len(runes) > maxRunes {
		return string(runes[:maxRunes])
	}
	return text
}

func boundedReflectionObservation(value string) string {
	value = strings.TrimSpace(value)
	const maxRunes = 8000
	if runes := []rune(value); len(runes) > maxRunes {
		return string(runes[:maxRunes])
	}
	return value
}

func compactReflectionAppraisalV2(appraisal map[string]any) map[string]any {
	result := map[string]any{}
	for _, key := range []string{"direction", "event_kind"} {
		if value := boundedReflectionScalarText(appraisal[key], 128); value != "" {
			result[key] = value
		}
	}
	for _, key := range []string{"confidence", "relevance", "goal_congruence", "controllability", "relationship_significance"} {
		if value := boundedReflectionScalar(appraisal[key], 0); value != nil {
			result[key] = value
		}
	}
	drives := make([]map[string]any, 0)
	for _, raw := range arrayValue(appraisal["drive_signals"]) {
		signal := mapValue(raw)
		compact := map[string]any{}
		if ref := boundedReflectionScalarText(signal["ref"], maxContextReferenceRunes); contextReferencePattern.MatchString(ref) {
			compact["ref"] = ref
		}
		if direction := boundedReflectionScalarText(signal["direction"], 128); direction != "" {
			compact["direction"] = direction
		}
		for _, key := range []string{"strength", "confidence"} {
			if value := boundedReflectionScalar(signal[key], 0); value != nil {
				compact[key] = value
			}
		}
		if len(compact) > 0 {
			drives = append(drives, compact)
		}
	}
	if len(drives) > 0 {
		result["drive_signals"] = drives
	}
	return result
}

func (a *App) processReflectionV2(
	ctx context.Context,
	fluctlightID string,
	ownerActorID string,
	correlationID string,
	watermark int,
	toSequence int,
	stateRevision int,
	evidence []map[string]any,
	projection ContextProjection,
	memoryAllowedEvidence map[string]struct{},
	memoryEvidenceScopes map[string]reflectionMemoryEvidenceScope,
) (map[string]any, error) {
	reflectionAt := time.Now().UTC()
	activeMemoryResult, err := a.retrieveActiveMemories(ctx, ActiveMemoryQuery{
		AuthorizationActorID: ownerActorID, OwnerFluctlightID: fluctlightID,
		ConversationID: projection.ConversationID, Cue: projection.CurrentUserText, At: reflectionAt, Limit: activeMemoryResultLimit,
	})
	if err != nil {
		_ = a.setReflectionWindowIdle(ctx, fluctlightID)
		return nil, err
	}
	referenceCapacity := maxContextReferences - len(projection.ReferenceIndex.ByRef)
	if referenceCapacity < 0 {
		referenceCapacity = 0
	}
	if len(activeMemoryResult.Items) > referenceCapacity {
		activeMemoryResult.Items = activeMemoryResult.Items[:referenceCapacity]
		activeMemoryResult.Trace.SelectedCount = len(activeMemoryResult.Items)
		activeMemoryResult.Trace.TruncatedReason = "context_reference_limit"
	}
	if err := addActiveMemoryReferences(&projection.ReferenceIndex, activeMemoryResult.Items); err != nil {
		_ = a.setReflectionWindowIdle(ctx, fluctlightID)
		return nil, err
	}
	providerEvidence := compactReflectionEvidenceV2(evidence)
	schema := reflectionProposalV2ProviderSchema()
	assembly, assembledProjection, err := a.assembleProjectionPrompt(ctx, projection, "reflection", []string{providerContextAuthorityRule, reflectionV2Instruction}, jsonString(map[string]any{"evidence": providerEvidence}), nil, "reflection_proposal_v2", schema)
	if err != nil {
		_ = a.setReflectionWindowIdle(ctx, fluctlightID)
		return nil, err
	}
	projection = assembledProjection
	providerCtx := WithPromptDiagnostics(WithProviderScenario(ctx, "reflection"), assembly.Diagnostics)
	completion, err := a.Provider.StructuredAssembledWithToolsSchema(
		providerCtx,
		"reflection",
		assembly.Messages,
		nil,
		"reflection_proposal_v2",
		schema,
		false,
	)
	if err != nil {
		_ = a.setReflectionWindowIdle(ctx, fluctlightID)
		if status, suppressed := providerSuppressionStatus(err); suppressed {
			reason := "fluctlight_not_active"
			if status == "paused" {
				reason = "fluctlight_paused"
			}
			return map[string]any{"fluctlight_id": fluctlightID, "correlation_id": correlationID, "status": status, "reason": reason}, nil
		}
		return nil, err
	}
	if completion.StructuredFallback || completion.Structured == nil {
		_ = a.setReflectionWindowIdle(ctx, fluctlightID)
		return nil, errors.New("reflection_structured_response_invalid")
	}
	proposal, err := DecodeReflectionProposalV2(jsonBytes(completion.Structured))
	if err != nil {
		_ = a.setReflectionWindowIdle(ctx, fluctlightID)
		return nil, err
	}
	baseRevisions, err := a.reflectionEvolutionRevisions(ctx, fluctlightID)
	if err != nil {
		_ = a.setReflectionWindowIdle(ctx, fluctlightID)
		return nil, err
	}
	evolutionEvidence := make([]EvolutionEvidence, 0, len(evidence))
	evolutionOutcomes := make([]ActionOutcome, 0)
	for index, item := range evidence {
		sequence := intValue(item["sequence"])
		occurredAt := time.Now().UTC()
		if value, ok := item["occurred_at"].(time.Time); ok && !value.IsZero() {
			occurredAt = value.UTC()
		}
		summary := jsonString(providerEvidence[index])
		if runes := []rune(summary); len(runes) > 2000 {
			summary = string(runes[:2000])
		}
		evolutionEvidence = append(evolutionEvidence, EvolutionEvidence{Ref: fmt.Sprintf("sequence:%d", sequence), Kind: stringValue(item["event_type"]), Summary: summary, Sequence: sequence, OccurredAt: occurredAt})
		if stringValue(item["event_type"]) == "autonomy.result" {
			for _, raw := range reflectionActionOutcomeValues(mapValue(item["payload"])) {
				var outcome ActionOutcome
				if encoded := jsonBytes(raw); json.Unmarshal(encoded, &outcome) != nil || outcome.Validate() != nil || outcome.FluctlightID != fluctlightID {
					_ = a.setReflectionWindowIdle(ctx, fluctlightID)
					return nil, errors.New("reflection_action_outcome_invalid")
				}
				evolutionOutcomes = append(evolutionOutcomes, outcome)
			}
		}
	}
	sourceWindow := fmt.Sprintf("sequence:%d-%d", watermark+1, toSequence)
	evolution, err := BuildEvolutionContext(EvolutionContext{
		ProviderRole: "reflection", FluctlightID: fluctlightID, SourceWindow: sourceWindow,
		FromSequence: watermark + 1, ToSequence: toSequence, Watermark: watermark,
		Evidence: evolutionEvidence, Outcomes: evolutionOutcomes, ReferenceIndex: projection.ReferenceIndex, BaseRevisions: baseRevisions,
	})
	if err != nil {
		_ = a.setReflectionWindowIdle(ctx, fluctlightID)
		return nil, err
	}
	plan, err := CompileReflectionPlan(proposal, evolution, ReflectionPolicyV2{SupportedDomains: map[EvolutionDomain]bool{
		EvolutionActiveMemory: true, EvolutionMemory: true, EvolutionGoal: true, EvolutionIntention: true, EvolutionAffectProfile: true,
		EvolutionRelationship: true, EvolutionDrive: true, EvolutionPreference: true, EvolutionTrigger: true, EvolutionDevelopingSelf: true,
		EvolutionPersonality: true, EvolutionBehaviorPolicy: true,
	}}, reflectionAt)
	if err != nil {
		_ = a.setReflectionWindowIdle(ctx, fluctlightID)
		return nil, err
	}
	for index := range plan.Candidates {
		candidatePlan := &plan.Candidates[index]
		if candidatePlan.Domain != EvolutionDevelopingSelf || candidatePlan.Disposition != EvolutionAccepted {
			continue
		}
		candidate := proposal.DevelopingSelfCandidates[candidatePlan.Index]
		for _, existing := range projection.DevelopingSelf {
			if stringValue(existing["category"]) == candidate.Category && stringValue(existing["claim"]) == candidate.Claim && numberOrZero(existing["confidence"]) == candidate.Confidence {
				candidatePlan.Disposition, candidatePlan.ReasonCode = EvolutionNoChange, "exact_duplicate"
				break
			}
		}
	}
	overlayBaseline := personaEvolutionBaselineFromProjection(projection)
	overlayState, err := loadPersonaEvolutionState(ctx, a.DB.Pool(), overlayBaseline)
	if err != nil {
		_ = a.setReflectionWindowIdle(ctx, fluctlightID)
		return nil, err
	}
	overlayDecisions := make(map[string]EvolutionOverlayDecision)
	workingOverlayState := clonePersonaEvolutionState(overlayState)
	for index := range plan.Candidates {
		candidatePlan := &plan.Candidates[index]
		if candidatePlan.Disposition != EvolutionAccepted || (candidatePlan.Domain != EvolutionPersonality && candidatePlan.Domain != EvolutionBehaviorPolicy) {
			continue
		}
		var candidate ReflectionOverlayCandidateV2
		if candidatePlan.Domain == EvolutionPersonality {
			candidate = proposal.PersonalityEvolutionCandidates[candidatePlan.Index]
		} else {
			candidate = proposal.BehaviorPolicyEvolutionCandidates[candidatePlan.Index]
		}
		evidenceWindows, windowErr := a.reflectionOverlayEvidenceWindows(ctx, fluctlightID, sourceWindow, candidatePlan.Domain, candidate)
		if windowErr != nil {
			_ = a.setReflectionWindowIdle(ctx, fluctlightID)
			return nil, windowErr
		}
		decision, compileErr := CompileEvolutionOverlay(workingOverlayState, EvolutionOverlayRequest{
			ExpectedRevision: workingOverlayState.Revision, Candidate: candidate,
			EvidenceWindows: evidenceWindows, OccurredAt: time.Now().UTC(),
		}, EvolutionOverlayPolicy{})
		if compileErr != nil {
			_ = a.setReflectionWindowIdle(ctx, fluctlightID)
			return nil, compileErr
		}
		if decision.Disposition != EvolutionAccepted {
			candidatePlan.Disposition, candidatePlan.ReasonCode = decision.Disposition, decision.ReasonCode
			continue
		}
		nextState, applyErr := ApplyEvolutionOverlay(workingOverlayState, decision)
		if applyErr != nil {
			_ = a.setReflectionWindowIdle(ctx, fluctlightID)
			return nil, applyErr
		}
		overlayDecisions[candidatePlan.CandidateID] = decision
		workingOverlayState = nextState
	}
	hydrateReflectionChangedRefs(&plan, proposal, evolution, overlayDecisions)
	activeMemoryCommands, err := compileReflectionActiveMemoryCommands(reflectionV2AcceptedActiveMemoryCandidates(proposal, plan), reflectionActiveMemoryCompileRequest{
		FluctlightID: fluctlightID, OwnerActorID: ownerActorID,
		ProposalID: plan.ProposalID, SourceWindow: sourceWindow, Timezone: stringValue(projection.LifeContext["timezone"]), OccurredAt: reflectionAt,
		ReferenceIndex: projection.ReferenceIndex, AllowedEvidence: memoryAllowedEvidence, EvidenceScopes: memoryEvidenceScopes,
	})
	if err != nil {
		_ = a.setReflectionWindowIdle(ctx, fluctlightID)
		return nil, err
	}
	memoryCommands, err := compileReflectionMemoryCommands(reflectionV2AcceptedMemoryProposal(proposal, plan), reflectionMemoryCompileRequest{
		FluctlightID: fluctlightID, OwnerActorID: ownerActorID,
		ActiveProfileID: projection.ReferenceIndex.ActiveProfileID,
		ProposalID:      plan.ProposalID, SourceWindow: sourceWindow, OccurredAt: reflectionAt,
		ReferenceIndex: projection.ReferenceIndex, AllowedEvidence: memoryAllowedEvidence,
		EvidenceScopes: memoryEvidenceScopes,
	})
	if err != nil {
		_ = a.setReflectionWindowIdle(ctx, fluctlightID)
		return nil, err
	}
	coordinator := EvolutionState{FluctlightID: fluctlightID, Watermark: watermark, Revisions: cloneRevisionMap(baseRevisions)}
	var nextCoordinator EvolutionState
	var applyResult ReflectionApplyResult
	profileID := projection.ReferenceIndex.ActiveProfileID
	if strings.TrimSpace(profileID) == "" {
		profileID = "default"
	}
	profileRef := "personality:ctx_" + stableDigest(fluctlightID+"\x1f"+profileID)
	var memoryResults []MemoryApplyResult
	var activeMemoryResults []ActiveMemoryApplyResult
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var latestStateRevision int
		if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_inner_states WHERE fluctlight_id=$1 FOR SHARE`, fluctlightID).Scan(&latestStateRevision); err != nil {
			return err
		}
		if latestStateRevision != stateRevision {
			return ErrConflict
		}
		activeMemoryResults, err = a.applyReflectionActiveMemoryCommandsTx(ctx, tx, activeMemoryCommands)
		if err != nil {
			return err
		}
		reconcileReflectionActiveMemoryDispositions(&plan, activeMemoryResults)
		memoryResults, err = a.applyReflectionMemoryCommandsTx(ctx, tx, memoryCommands)
		if err != nil {
			return err
		}
		reconcileReflectionMemoryDispositions(&plan, memoryResults)
		if err := applyReflectionGoalCandidatesV2Tx(ctx, tx, fluctlightID, proposal, plan, evolution, time.Now().UTC()); err != nil {
			return err
		}
		if err := applyReflectionIntentionCandidatesV2Tx(ctx, tx, fluctlightID, proposal, plan, projection.ReferenceIndex, time.Now().UTC()); err != nil {
			return err
		}
		if err := applyReflectionAffectProfileV2Tx(ctx, tx, fluctlightID, proposal, plan); err != nil {
			return err
		}
		if err := applyReflectionRelationshipObservationsV2Tx(ctx, tx, fluctlightID, proposal, plan, projection.ReferenceIndex); err != nil {
			return err
		}
		if err := a.applyReflectionSlotCandidatesV2Tx(ctx, tx, fluctlightID, proposal, plan, evolution); err != nil {
			return err
		}
		if err := a.applyReflectionDevelopingSelfV2Tx(ctx, tx, fluctlightID, proposal, plan, evolution); err != nil {
			return err
		}
		liveOverlayState, loadErr := loadPersonaEvolutionState(ctx, tx, overlayBaseline)
		if loadErr != nil {
			return loadErr
		}
		if liveOverlayState.Revision != overlayState.Revision {
			return errors.New("evolution_overlay_revision_conflict")
		}
		for _, candidatePlan := range plan.Candidates {
			decision, ok := overlayDecisions[candidatePlan.CandidateID]
			if !ok {
				continue
			}
			nextState, applyErr := ApplyEvolutionOverlay(liveOverlayState, decision)
			if applyErr != nil {
				return applyErr
			}
			if _, persistErr := persistEvolutionOverlayTx(ctx, tx, liveOverlayState, nextState, *decision.Overlay); persistErr != nil {
				return persistErr
			}
			liveOverlayState = nextState
		}
		nextCoordinator, applyResult, err = ApplyReflectionPlan(coordinator, plan, nil)
		if err != nil {
			return err
		}
		_, err = persistReflectionApplyTx(ctx, tx, evolution, proposal, plan, nextCoordinator, applyResult, profileRef, "configured-reflection-model", "reflection.v2")
		return err
	})
	if err != nil {
		_ = a.setReflectionWindowIdle(ctx, fluctlightID)
		return nil, err
	}
	return map[string]any{
		"fluctlight_id": fluctlightID, "correlation_id": correlationID, "status": applyResult.Status,
		"watermark": toSequence, "proposal_id": plan.ProposalID, "active_memory": reflectionActiveMemoryResultSummary(activeMemoryResults), "memory": reflectionMemoryResultSummary(memoryResults),
		"counts": applyResult.Counts, "changed_refs": applyResult.ChangedRefs, "revisions": applyResult.Revisions, "reason_codes": applyResult.ReasonCodes,
	}, nil
}

func reconcileReflectionActiveMemoryDispositions(plan *ReflectionEvolutionPlan, results []ActiveMemoryApplyResult) {
	if plan == nil {
		return
	}
	resultIndex := 0
	for index := range plan.Candidates {
		candidate := &plan.Candidates[index]
		if candidate.Domain != EvolutionActiveMemory || candidate.Disposition != EvolutionAccepted {
			continue
		}
		if resultIndex >= len(results) {
			return
		}
		result := results[resultIndex]
		resultIndex++
		switch result.Disposition {
		case "no_change":
			candidate.Disposition = EvolutionNoChange
		case "rejected":
			candidate.Disposition = EvolutionRejected
		case "deferred":
			candidate.Disposition = EvolutionDeferred
		}
		if result.ReasonCode != "" {
			candidate.ReasonCode = result.ReasonCode
		}
	}
}

func reconcileReflectionMemoryDispositions(plan *ReflectionEvolutionPlan, results []MemoryApplyResult) {
	if plan == nil {
		return
	}
	resultIndex := 0
	for index := range plan.Candidates {
		candidate := &plan.Candidates[index]
		if candidate.Domain != EvolutionMemory || candidate.Disposition != EvolutionAccepted {
			continue
		}
		if resultIndex >= len(results) {
			return
		}
		result := results[resultIndex]
		resultIndex++
		switch result.Disposition {
		case "no_change":
			candidate.Disposition = EvolutionNoChange
		case "rejected":
			candidate.Disposition = EvolutionRejected
		case "deferred":
			candidate.Disposition = EvolutionDeferred
		}
		if result.ReasonCode != "" {
			candidate.ReasonCode = result.ReasonCode
		}
	}
}

func (a *App) reflectionOverlayEvidenceWindows(ctx context.Context, fluctlightID, currentWindow string, domain EvolutionDomain, candidate ReflectionOverlayCandidateV2) ([]string, error) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT from_sequence,to_sequence,payload FROM public.cognition_reflection_proposals WHERE fluctlight_id=$1 AND schema_version=$2 ORDER BY to_sequence DESC LIMIT 64`, fluctlightID, reflectionProposalV2SchemaVersion)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	windows := []string{currentWindow}
	for rows.Next() {
		var fromSequence, toSequence int
		var raw []byte
		if err := rows.Scan(&fromSequence, &toSequence, &raw); err != nil {
			return nil, err
		}
		var prior ReflectionProposalV2
		if err := json.Unmarshal(raw, &prior); err != nil {
			return nil, errors.New("reflection_overlay_history_invalid")
		}
		items := prior.PersonalityEvolutionCandidates
		if domain == EvolutionBehaviorPolicy {
			items = prior.BehaviorPolicyEvolutionCandidates
		}
		for _, item := range items {
			if item.FieldPath == candidate.FieldPath && item.Direction == candidate.Direction && item.SemanticValue == candidate.SemanticValue {
				windows = append(windows, fmt.Sprintf("sequence:%d-%d", fromSequence, toSequence))
				break
			}
		}
	}
	return mergeStableRefs(windows), rows.Err()
}

func hydrateReflectionChangedRefs(plan *ReflectionEvolutionPlan, proposal ReflectionProposalV2, evolution EvolutionContext, overlays map[string]EvolutionOverlayDecision) {
	if plan == nil {
		return
	}
	affectRef := ""
	for ref, entry := range evolution.ReferenceIndex.ByRef {
		if entry.Kind == ContextReferenceAffectProfile {
			affectRef = ref
			break
		}
	}
	for index := range plan.Candidates {
		candidate := &plan.Candidates[index]
		if candidate.Disposition != EvolutionAccepted || candidate.TargetRef != "" {
			continue
		}
		switch candidate.Domain {
		case EvolutionMemory:
			candidate.TargetRef = "memory:ctx_" + stableDigest(plan.ProposalID+"\x1f"+candidate.CandidateID)
		case EvolutionActiveMemory:
			candidate.TargetRef = "active_memory:ctx_" + stableDigest(plan.ProposalID+"\x1f"+candidate.CandidateID)
		case EvolutionGoal:
			entityID := "goal_reflection_" + stableDigest(fmt.Sprintf("reflection:%s:goal:%d", plan.ProposalID, candidate.Index))
			candidate.TargetRef = "goal:ctx_" + stableDigest(entityID)
		case EvolutionIntention:
			entityID := "intention_reflection_" + stableDigest(fmt.Sprintf("reflection:%s:intention:%d", plan.ProposalID, candidate.Index))
			candidate.TargetRef = "intention:ctx_" + stableDigest(entityID)
		case EvolutionAffectProfile:
			candidate.TargetRef = affectRef
		case EvolutionDrive:
			entityID := "drive_slot_" + stableDigest(plan.FluctlightID+":"+proposal.DriveCandidates[candidate.Index].Key)
			candidate.TargetRef = "drive:ctx_" + stableDigest(entityID)
		case EvolutionPreference:
			entityID := "preference_slot_" + stableDigest(plan.FluctlightID+":"+proposal.PreferenceCandidates[candidate.Index].Key)
			candidate.TargetRef = "preference:ctx_" + stableDigest(entityID)
		case EvolutionTrigger:
			entityID := "trigger_preference_" + stableDigest(plan.FluctlightID+":"+proposal.TriggerCandidates[candidate.Index].Key)
			candidate.TargetRef = "trigger:ctx_" + stableDigest(entityID)
		case EvolutionDevelopingSelf:
			item := proposal.DevelopingSelfCandidates[candidate.Index]
			entityID := "self_claim_" + stableDigest(plan.FluctlightID+":"+item.Category+":"+item.Claim)
			candidate.TargetRef = "developing_self:ctx_" + stableDigest(entityID)
		case EvolutionPersonality, EvolutionBehaviorPolicy:
			if decision := overlays[candidate.CandidateID]; decision.Overlay != nil {
				candidate.TargetRef = decision.Overlay.Ref
			}
		}
	}
}

func reflectionActionOutcomeValues(payload map[string]any) []any {
	if values := arrayValue(payload["outcomes"]); len(values) > 0 {
		return values
	}
	if value, ok := payload["outcome"]; ok && value != nil {
		return []any{value}
	}
	return nil
}

func (a *App) reflectionEvolutionRevisions(ctx context.Context, fluctlightID string) (map[string]int, error) {
	result := map[string]int{}
	var raw []byte
	err := a.DB.Pool().QueryRow(ctx, `SELECT evolution_revisions FROM public.cognition_reflection_windows WHERE fluctlight_id=$1`, fluctlightID).Scan(&raw)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	for key, value := range decodeObject(raw) {
		result[key] = intValue(value)
	}
	for _, domain := range []EvolutionDomain{EvolutionActiveMemory, EvolutionMemory, EvolutionRelationship, EvolutionGoal, EvolutionIntention, EvolutionAffectProfile, EvolutionDrive, EvolutionPreference, EvolutionTrigger, EvolutionDevelopingSelf, EvolutionPersonality, EvolutionBehaviorPolicy} {
		if _, exists := result[string(domain)]; !exists {
			result[string(domain)] = 0
		}
	}
	return result, nil
}

func reflectionV2AcceptedActiveMemoryCandidates(proposal ReflectionProposalV2, plan ReflectionEvolutionPlan) []reflectionAcceptedActiveMemoryCandidate {
	accepted := make(map[int]struct{})
	for _, candidate := range plan.Candidates {
		if candidate.Domain == EvolutionActiveMemory && candidate.Disposition == EvolutionAccepted {
			accepted[candidate.Index] = struct{}{}
		}
	}
	items := make([]reflectionAcceptedActiveMemoryCandidate, 0, len(accepted))
	for index, candidate := range proposal.ActiveMemoryCandidates {
		if _, ok := accepted[index]; ok {
			items = append(items, reflectionAcceptedActiveMemoryCandidate{Index: index, Candidate: candidate})
		}
	}
	return items
}

func reflectionV2AcceptedMemoryProposal(proposal ReflectionProposalV2, plan ReflectionEvolutionPlan) map[string]any {
	accepted := map[int]struct{}{}
	for _, candidate := range plan.Candidates {
		if candidate.Domain == EvolutionMemory && candidate.Disposition == EvolutionAccepted {
			accepted[candidate.Index] = struct{}{}
		}
	}
	items := make([]any, 0, len(accepted))
	for index, candidate := range proposal.MemoryCandidates {
		if _, ok := accepted[index]; !ok {
			continue
		}
		item := map[string]any{"operation": candidate.Operation, "evidence_refs": stringSliceAny(candidate.EvidenceRefs), "semantic_reason": candidate.SemanticReason}
		if candidate.TargetRef != "" {
			item["target_ref"] = candidate.TargetRef
		}
		if len(candidate.MergeRefs) > 0 {
			item["merge_refs"] = stringSliceAny(candidate.MergeRefs)
		}
		switch MemoryOperation(candidate.Operation) {
		case MemoryCreate, MemoryRevise, MemoryMerge, MemorySupersede:
			item["type"], item["content"] = candidate.Type, candidate.Content
			item["confidence"], item["importance"], item["emotional_significance"] = candidate.Confidence, candidate.Importance, candidate.EmotionalSignificance
		}
		items = append(items, item)
	}
	return map[string]any{"memory_candidates": items}
}

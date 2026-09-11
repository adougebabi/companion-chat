package core

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type ActionOutcomeStatus string

const (
	ActionOutcomePending       ActionOutcomeStatus = "pending"
	ActionOutcomeCompleted     ActionOutcomeStatus = "completed"
	ActionOutcomeFailed        ActionOutcomeStatus = "failed"
	ActionOutcomeCancelled     ActionOutcomeStatus = "cancelled"
	ActionOutcomeSuppressed    ActionOutcomeStatus = "suppressed"
	ActionOutcomeUnknown       ActionOutcomeStatus = "unknown"
	actionPrimaryCallID                            = "action"
	actionOutcomeSchemaVersion                     = "fluctlight.action-outcome.v1"
)

type ActionOutcome struct {
	SchemaVersion      string                      `json:"schema_version"`
	ID                 string                      `json:"id"`
	FluctlightID       string                      `json:"fluctlight_id"`
	ActionID           string                      `json:"action_id"`
	CallID             string                      `json:"call_id"`
	CapabilityName     string                      `json:"capability_name,omitempty"`
	Status             ActionOutcomeStatus         `json:"status"`
	SuccessBoundary    string                      `json:"success_boundary"`
	CompletionBoundary string                      `json:"completion_boundary,omitempty"`
	ExternalRef        string                      `json:"external_ref,omitempty"`
	Expected           map[string]any              `json:"expected"`
	Observed           map[string]any              `json:"observed"`
	ErrorCode          string                      `json:"error_code,omitempty"`
	GoalRefs           []string                    `json:"goal_refs"`
	IntentionRefs      []string                    `json:"intention_refs"`
	EvidenceRefs       []string                    `json:"evidence_refs"`
	ContextReferences  map[string]ContextReference `json:"context_references"`
	Revision           int                         `json:"revision"`
	OccurredAt         time.Time                   `json:"occurred_at"`
}

func (outcome ActionOutcome) Validate() error {
	if outcome.SchemaVersion != actionOutcomeSchemaVersion || strings.TrimSpace(outcome.ID) == "" || strings.TrimSpace(outcome.FluctlightID) == "" || strings.TrimSpace(outcome.ActionID) == "" || strings.TrimSpace(outcome.CallID) == "" {
		return errors.New("action_outcome_identity_invalid")
	}
	switch outcome.Status {
	case ActionOutcomePending, ActionOutcomeCompleted, ActionOutcomeFailed, ActionOutcomeCancelled, ActionOutcomeSuppressed, ActionOutcomeUnknown:
	default:
		return errors.New("action_outcome_status_invalid")
	}
	if strings.TrimSpace(outcome.SuccessBoundary) == "" || len([]rune(outcome.SuccessBoundary)) > 128 || outcome.Revision < 1 || outcome.OccurredAt.IsZero() {
		return errors.New("action_outcome_contract_invalid")
	}
	if outcome.CompletionBoundary != "" {
		if len([]rune(outcome.CompletionBoundary)) > 128 || strings.TrimSpace(outcome.ExternalRef) == "" || len([]rune(outcome.ExternalRef)) > 256 {
			return errors.New("action_outcome_async_boundary_invalid")
		}
	} else if outcome.ExternalRef != "" {
		return errors.New("action_outcome_external_ref_invalid")
	}
	if len(outcome.GoalRefs) > 32 || len(outcome.IntentionRefs) > 32 || len(outcome.EvidenceRefs) > 64 || len(outcome.ContextReferences) > maxDecisionInfluences {
		return errors.New("action_outcome_references_too_large")
	}
	for ref, entry := range outcome.ContextReferences {
		if ref != entry.Ref || !contextReferencePattern.MatchString(ref) {
			return errors.New("action_outcome_context_reference_invalid")
		}
	}
	if len(jsonBytes(outcome.Expected)) > 32*1024 || len(jsonBytes(outcome.Observed)) > 64*1024 || len(jsonBytes(outcome.ContextReferences)) > maxContextReferenceIndexBytes {
		return errors.New("action_outcome_payload_too_large")
	}
	return nil
}

func buildActionOutcomes(actionID, fluctlightID, sourceFactID, actionType string, results []CapabilityResult, settlement map[string]any, registry *CapabilityRegistry) ([]ActionOutcome, error) {
	goalRefs := decisionServiceRefValues(settlement["goal_refs"])
	intentionRefs := decisionServiceRefValues(settlement["intention_refs"])
	contextReferences, err := actionOutcomeContextReferences(settlement["context_references"])
	if err != nil {
		return nil, err
	}
	evidenceRefs := []string{}
	if source := strings.TrimSpace(sourceFactID); source != "" {
		evidenceRefs = append(evidenceRefs, source)
	}
	createdAt := time.Now().UTC()
	makeOutcome := func(callID, capabilityName string, status ActionOutcomeStatus, boundary, errorCode string, observed map[string]any) (ActionOutcome, error) {
		outcome := ActionOutcome{
			SchemaVersion: actionOutcomeSchemaVersion,
			ID:            "outcome_" + stableDigest(actionID+"\x1f"+callID),
			FluctlightID:  fluctlightID, ActionID: actionID, CallID: callID, CapabilityName: capabilityName,
			Status: status, SuccessBoundary: boundary,
			Expected: map[string]any{"action_type": actionType}, Observed: observed, ErrorCode: errorCode,
			GoalRefs: goalRefs, IntentionRefs: intentionRefs, EvidenceRefs: evidenceRefs,
			ContextReferences: contextReferences, Revision: 1, OccurredAt: createdAt,
		}
		return outcome, outcome.Validate()
	}
	status := ActionOutcomeCompleted
	protocolStatus := firstString(settlement["action_status"], firstString(settlement["status"], "completed"))
	switch protocolStatus {
	case "failed", "rejected", "blocked":
		status = ActionOutcomeFailed
	case "cancelled", "cancel_requested":
		status = ActionOutcomeCancelled
	case "suppressed":
		status = ActionOutcomeSuppressed
	case "unknown":
		status = ActionOutcomeUnknown
	case "pending", "queued", "deferred":
		status = ActionOutcomePending
	}
	if stringValue(settlement["delivery_status"]) == "duplicate_suppressed" {
		status = ActionOutcomeSuppressed
	}
	observed := map[string]any{"status": protocolStatus}
	for _, key := range []string{"delivery_status", "message_id", "media_intent_id", "reason", "reason_code", "resulting_state_ref", "resulting_state_revision"} {
		if value, ok := settlement[key]; ok && value != nil && value != "" {
			observed[key] = value
		}
	}
	primary, err := makeOutcome(actionPrimaryCallID, "", status, "action_settled", stringValue(settlement["error_code"]), observed)
	if err != nil {
		return nil, err
	}
	outcomes := []ActionOutcome{primary}
	if len(results) == 0 {
		return outcomes, nil
	}
	for _, result := range results {
		if registry == nil {
			return nil, errors.New("action_outcome_registry_required")
		}
		definition, ok := registry.Definition(result.CapabilityName)
		if !ok || strings.TrimSpace(definition.SuccessBoundary) == "" {
			return nil, errors.New("action_outcome_success_boundary_missing")
		}
		status := ActionOutcomeUnknown
		switch result.Status {
		case "completed":
			status = ActionOutcomeCompleted
		case "failed", "rejected":
			status = ActionOutcomeFailed
		case "deferred":
			switch primary.Status {
			case ActionOutcomeCancelled:
				status = ActionOutcomeCancelled
			case ActionOutcomeFailed:
				status = ActionOutcomeFailed
			default:
				status = ActionOutcomePending
			}
		}
		externalRef := ""
		if definition.OutcomeReferenceField != "" {
			externalRef = stringValue(mapValue(result.Output)[definition.OutcomeReferenceField])
			if definition.CompletionBoundary != "" && result.Status == "completed" {
				if externalRef == "" {
					return nil, errors.New("action_outcome_external_ref_missing")
				}
				status = ActionOutcomePending
			}
		}
		observed := mapValue(result.Output)
		if observed == nil || len(observed) == 0 {
			observed = map[string]any{"status": result.Status}
		}
		outcome, err := makeOutcome(result.CallID, result.CapabilityName, status, definition.SuccessBoundary, result.ErrorCode, observed)
		if err != nil {
			return nil, err
		}
		outcome.CompletionBoundary = definition.CompletionBoundary
		outcome.ExternalRef = externalRef
		if err := outcome.Validate(); err != nil {
			return nil, err
		}
		outcomes = append(outcomes, outcome)
	}
	for _, outcome := range outcomes[1:] {
		if outcome.Status == ActionOutcomePending || outcome.Status == ActionOutcomeUnknown {
			outcomes[0].Status = ActionOutcomePending
			outcomes[0].SuccessBoundary = "action_in_progress"
			outcomes[0].Observed["status"] = "in_progress"
			break
		}
	}
	if err := outcomes[0].Validate(); err != nil {
		return nil, err
	}
	return outcomes, nil
}

func actionOutcomeContextReferences(value any) (map[string]ContextReference, error) {
	if value == nil {
		return map[string]ContextReference{}, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, errors.New("action_outcome_context_references_invalid")
	}
	result := make(map[string]ContextReference)
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, errors.New("action_outcome_context_references_invalid")
	}
	if len(result) > maxDecisionInfluences {
		return nil, errors.New("action_outcome_context_references_too_large")
	}
	for ref, entry := range result {
		if ref != entry.Ref || !contextReferencePattern.MatchString(ref) || len(entry.Snapshot) == 0 || !json.Valid(entry.Snapshot) {
			return nil, errors.New("action_outcome_context_references_invalid")
		}
	}
	return result, nil
}

func actionOutcomeRequestDigest(outcome ActionOutcome) string {
	return stableDigest(jsonString(map[string]any{
		"schema_version": outcome.SchemaVersion, "id": outcome.ID, "fluctlight_id": outcome.FluctlightID,
		"action_id": outcome.ActionID, "call_id": outcome.CallID, "capability_name": outcome.CapabilityName,
		"status": outcome.Status, "success_boundary": outcome.SuccessBoundary, "completion_boundary": outcome.CompletionBoundary, "external_ref": outcome.ExternalRef, "expected": outcome.Expected,
		"observed": outcome.Observed, "error_code": outcome.ErrorCode, "goal_refs": outcome.GoalRefs,
		"intention_refs": outcome.IntentionRefs, "evidence_refs": outcome.EvidenceRefs,
		"context_references": outcome.ContextReferences, "revision": outcome.Revision,
	}))
}

func persistActionOutcomesTx(ctx context.Context, tx pgx.Tx, outcomes []ActionOutcome) error {
	for _, outcome := range outcomes {
		if err := outcome.Validate(); err != nil {
			return err
		}
		digest := actionOutcomeRequestDigest(outcome)
		command, err := tx.Exec(ctx, `INSERT INTO public.cognition_action_outcomes(id,fluctlight_id,action_id,call_id,capability_name,status,success_boundary,completion_boundary,external_ref,expected,observed,error_code,goal_refs,intention_refs,evidence_refs,context_references,revision,occurred_at,request_digest) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19) ON CONFLICT(action_id,call_id) DO NOTHING`, outcome.ID, outcome.FluctlightID, outcome.ActionID, outcome.CallID, outcome.CapabilityName, outcome.Status, outcome.SuccessBoundary, nullableString(outcome.CompletionBoundary), nullableString(outcome.ExternalRef), jsonBytes(outcome.Expected), jsonBytes(outcome.Observed), nullableString(outcome.ErrorCode), jsonBytes(outcome.GoalRefs), jsonBytes(outcome.IntentionRefs), jsonBytes(outcome.EvidenceRefs), jsonBytes(outcome.ContextReferences), outcome.Revision, outcome.OccurredAt, digest)
		if err != nil {
			return err
		}
		if command.RowsAffected() == 0 {
			var existingDigest string
			if err := tx.QueryRow(ctx, `SELECT request_digest FROM public.cognition_action_outcomes WHERE action_id=$1 AND call_id=$2`, outcome.ActionID, outcome.CallID).Scan(&existingDigest); err != nil {
				return err
			}
			if existingDigest != digest {
				return errors.New("action_outcome_replay_conflict")
			}
		}
		if outcome.CallID == actionPrimaryCallID {
			if err := settleOutcomeIntentionAttemptsTx(ctx, tx, outcome); err != nil {
				return err
			}
		}
	}
	return nil
}

func settleOutcomeIntentionAttemptsTx(ctx context.Context, tx pgx.Tx, outcome ActionOutcome) error {
	if outcome.Status == ActionOutcomePending || outcome.Status == ActionOutcomeUnknown || len(outcome.IntentionRefs) == 0 {
		return nil
	}
	for _, intentionRef := range outcome.IntentionRefs {
		entry, ok := outcome.ContextReferences[intentionRef]
		if !ok || entry.Kind != ContextReferenceIntention {
			return errors.New("intention_outcome_context_reference_invalid")
		}
		snapshot := decodeObject(entry.Snapshot)
		goalRef := strings.TrimSpace(stringValue(snapshot["goal_ref"]))
		attemptID := strings.TrimSpace(stringValue(snapshot["current_attempt_id"]))
		if attemptID == "" {
			continue
		}
		digest := stableDigest(jsonString(map[string]any{"id": outcome.ID, "action_id": outcome.ActionID, "call_id": outcome.CallID, "status": outcome.Status, "success_boundary": outcome.SuccessBoundary, "error_code": outcome.ErrorCode, "revision": outcome.Revision}))
		var storedDigest string
		if err := tx.QueryRow(ctx, `SELECT outcome_digest FROM public.fluctlight_intention_attempts WHERE attempt_id=$1`, attemptID).Scan(&storedDigest); err == nil {
			if storedDigest != digest {
				return errors.New("intention_attempt_replay_conflict")
			}
			continue
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		current, err := loadIntentionAuthorityTx(ctx, tx, outcome.FluctlightID, intentionRef, goalRef, entry)
		if err != nil {
			return err
		}
		if current.LastAttemptID != attemptID || (current.Status != IntentionDue && current.Status != IntentionInProgress) {
			continue
		}
		frozen := FrozenIntentionAction{
			IntentionRef: intentionRef, GoalRef: goalRef, AttemptID: attemptID,
			ActionID: outcome.ActionID, OutcomeID: outcome.ID, CapabilityName: outcome.CapabilityName,
		}
		settlement, err := SettleIntentionAttempt(current, frozen, outcome, nil)
		if err != nil {
			return err
		}
		if _, err := persistIntentionAttemptSettlementTx(ctx, tx, current, settlement); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) persistStandaloneCapabilityOutcomes(ctx context.Context, fluctlightID string, invocations []CapabilityInvocation, results []CapabilityResult, executionErr error) error {
	type actionGroup struct {
		sourceFactID string
		results      []CapabilityResult
	}
	groups := make(map[string]*actionGroup)
	for _, invocation := range invocations {
		actionID := strings.TrimSpace(invocation.ActionID)
		if actionID == "" {
			continue
		}
		group := groups[actionID]
		if group == nil {
			group = &actionGroup{sourceFactID: invocation.SourceFactID}
			groups[actionID] = group
		}
		if result, ok := capabilityResultForCall(results, invocation.CallID); ok {
			group.results = append(group.results, result)
		}
	}
	if len(groups) == 0 {
		return nil
	}
	if a == nil || a.DB == nil {
		return errors.New("action_outcome_persistence_unavailable")
	}
	actionIDs := make([]string, 0, len(groups))
	for actionID := range groups {
		actionIDs = append(actionIDs, actionID)
	}
	sort.Strings(actionIDs)
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		for _, actionID := range actionIDs {
			group := groups[actionID]
			status := "completed"
			if executionErr != nil {
				status = "failed"
			}
			settlement := map[string]any{"status": status}
			if executionErr != nil {
				settlement["error_code"] = "capability_execution_failed"
			}
			outcomes, err := buildActionOutcomes(actionID, fluctlightID, group.sourceFactID, "capability", group.results, settlement, a.capabilityRegistry())
			if err != nil {
				return err
			}
			if err := persistActionOutcomesTx(ctx, tx, outcomes); err != nil {
				return err
			}
			factPayload := map[string]any{"action_id": actionID, "source_fact_id": group.sourceFactID, "capability_results": group.results, "outcomes": outcomes}
			factID, err := appendProcessedCognitionFactTx(ctx, tx, fluctlightID, "autonomy.result", factPayload, "action-result:"+actionID)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'lifecycle','reflection.run',$3) ON CONFLICT DO NOTHING`, "reflection_intent:action:"+actionID, "reflection:action:"+actionID, jsonBytes(map[string]any{"fluctlight_id": fluctlightID, "source_fact_id": factID, "action_id": actionID})); err != nil {
				return err
			}
			if err := appendOutboxTx(ctx, tx, "autonomy.result.recorded", "fluctlight", fluctlightID, fluctlightID, actionID, "action-result:"+actionID, "action-result:"+actionID, factPayload); err != nil {
				return err
			}
		}
		return nil
	})
}

func (a *App) settleActionOutcomeByExternalRefTx(ctx context.Context, tx pgx.Tx, externalRef string, status ActionOutcomeStatus, observed map[string]any, errorCode string) (bool, error) {
	externalRef = strings.TrimSpace(externalRef)
	if externalRef == "" {
		return false, errors.New("action_outcome_external_ref_required")
	}
	switch status {
	case ActionOutcomeCompleted, ActionOutcomeFailed, ActionOutcomeCancelled, ActionOutcomeUnknown:
	default:
		return false, errors.New("action_outcome_async_status_invalid")
	}
	var outcome ActionOutcome
	var currentStatus string
	var expectedRaw, observedRaw, goalRefsRaw, intentionRefsRaw, evidenceRefsRaw, contextReferencesRaw []byte
	err := tx.QueryRow(ctx, `SELECT id,fluctlight_id,action_id,call_id,capability_name,status,success_boundary,COALESCE(completion_boundary,''),COALESCE(external_ref,''),expected,observed,COALESCE(error_code,''),goal_refs,intention_refs,evidence_refs,context_references,revision,occurred_at FROM public.cognition_action_outcomes WHERE external_ref=$1 FOR UPDATE`, externalRef).Scan(
		&outcome.ID, &outcome.FluctlightID, &outcome.ActionID, &outcome.CallID, &outcome.CapabilityName,
		&currentStatus, &outcome.SuccessBoundary, &outcome.CompletionBoundary, &outcome.ExternalRef,
		&expectedRaw, &observedRaw, &outcome.ErrorCode, &goalRefsRaw, &intentionRefsRaw, &evidenceRefsRaw,
		&contextReferencesRaw, &outcome.Revision, &outcome.OccurredAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		// Not every media/visual workflow originates from a cognition Capability.
		// Absence of an ActionOutcome is a valid no-op for those independent flows.
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if currentStatus == string(ActionOutcomeCompleted) || currentStatus == string(ActionOutcomeFailed) || currentStatus == string(ActionOutcomeCancelled) {
		if currentStatus != string(status) {
			return false, errors.New("action_outcome_async_terminal_conflict")
		}
		currentObserved := decodeObject(observedRaw)
		for key, value := range observed {
			if existing, ok := currentObserved[key]; !ok || jsonString(existing) != jsonString(value) {
				return false, errors.New("action_outcome_async_terminal_conflict")
			}
		}
		if strings.TrimSpace(errorCode) != strings.TrimSpace(outcome.ErrorCode) {
			return false, errors.New("action_outcome_async_terminal_conflict")
		}
		return false, nil
	}
	if outcome.CompletionBoundary == "" {
		return false, errors.New("action_outcome_completion_boundary_missing")
	}
	outcome.SchemaVersion = actionOutcomeSchemaVersion
	outcome.Expected = decodeObject(expectedRaw)
	outcome.Observed = decodeObject(observedRaw)
	if outcome.Observed == nil {
		outcome.Observed = map[string]any{}
	}
	for key, value := range observed {
		outcome.Observed[key] = value
	}
	outcome.Observed["status"] = string(status)
	outcome.Status = status
	if status != ActionOutcomeUnknown {
		outcome.SuccessBoundary = outcome.CompletionBoundary
	}
	outcome.ErrorCode = strings.TrimSpace(errorCode)
	outcome.GoalRefs = decodeOutcomeStringArray(goalRefsRaw)
	outcome.IntentionRefs = decodeOutcomeStringArray(intentionRefsRaw)
	outcome.EvidenceRefs = decodeOutcomeStringArray(evidenceRefsRaw)
	if err := json.Unmarshal(contextReferencesRaw, &outcome.ContextReferences); err != nil {
		return false, errors.New("action_outcome_context_references_invalid")
	}
	outcome.Revision++
	outcome.OccurredAt = time.Now().UTC()
	if err := outcome.Validate(); err != nil {
		return false, err
	}
	command, err := tx.Exec(ctx, `UPDATE public.cognition_action_outcomes SET status=$2,success_boundary=$3,observed=$4,error_code=$5,revision=$6,occurred_at=$7,updated_at=now() WHERE id=$1 AND revision=$8 AND status IN ('pending','unknown')`, outcome.ID, outcome.Status, outcome.SuccessBoundary, jsonBytes(outcome.Observed), nullableString(outcome.ErrorCode), outcome.Revision, outcome.OccurredAt, outcome.Revision-1)
	if err != nil {
		return false, err
	}
	if command.RowsAffected() != 1 {
		return false, ErrConflict
	}
	if _, err := settleAggregateActionOutcomeTx(ctx, tx, outcome.ActionID); err != nil {
		return false, err
	}
	revisionKey := outcome.ID + ":" + strconv.Itoa(outcome.Revision)
	factPayload := map[string]any{"action_id": outcome.ActionID, "call_id": outcome.CallID, "outcome": outcome}
	factID, err := appendProcessedCognitionFactTx(ctx, tx, outcome.FluctlightID, "autonomy.result", factPayload, "action-outcome:"+revisionKey)
	if err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'lifecycle','reflection.run',$3) ON CONFLICT DO NOTHING`, "reflection_intent:outcome:"+revisionKey, "reflection:outcome:"+revisionKey, jsonBytes(map[string]any{"fluctlight_id": outcome.FluctlightID, "source_fact_id": factID, "action_id": outcome.ActionID, "outcome_id": outcome.ID, "outcome_revision": outcome.Revision})); err != nil {
		return false, err
	}
	if err := appendOutboxTx(ctx, tx, "action.outcome.updated", "action_outcome", outcome.ID, outcome.FluctlightID, outcome.ActionID, "action-outcome:"+outcome.ID, "action-outcome:"+revisionKey, map[string]any{"outcome_id": outcome.ID, "action_id": outcome.ActionID, "call_id": outcome.CallID, "status": outcome.Status, "revision": outcome.Revision, "aggregate_sequence": outcome.Revision}); err != nil {
		return false, err
	}
	return true, nil
}

func settleAggregateActionOutcomeTx(ctx context.Context, tx pgx.Tx, actionID string) (*ActionOutcome, error) {
	var open int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.cognition_action_outcomes WHERE action_id=$1 AND call_id<>$2 AND status IN ('pending','unknown')`, actionID, actionPrimaryCallID).Scan(&open); err != nil {
		return nil, err
	}
	if open > 0 {
		return nil, nil
	}
	var failed, cancelled, suppressed int
	if err := tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE status='failed'),count(*) FILTER (WHERE status='cancelled'),count(*) FILTER (WHERE status='suppressed') FROM public.cognition_action_outcomes WHERE action_id=$1 AND call_id<>$2`, actionID, actionPrimaryCallID).Scan(&failed, &cancelled, &suppressed); err != nil {
		return nil, err
	}
	var aggregate ActionOutcome
	var expectedRaw, observedRaw, goalRefsRaw, intentionRefsRaw, evidenceRefsRaw, contextRaw []byte
	var currentStatus string
	if err := tx.QueryRow(ctx, `SELECT id,fluctlight_id,action_id,call_id,status,success_boundary,expected,observed,COALESCE(error_code,''),goal_refs,intention_refs,evidence_refs,context_references,revision,occurred_at FROM public.cognition_action_outcomes WHERE action_id=$1 AND call_id=$2 FOR UPDATE`, actionID, actionPrimaryCallID).Scan(&aggregate.ID, &aggregate.FluctlightID, &aggregate.ActionID, &aggregate.CallID, &currentStatus, &aggregate.SuccessBoundary, &expectedRaw, &observedRaw, &aggregate.ErrorCode, &goalRefsRaw, &intentionRefsRaw, &evidenceRefsRaw, &contextRaw, &aggregate.Revision, &aggregate.OccurredAt); err != nil {
		return nil, err
	}
	if currentStatus != string(ActionOutcomePending) && currentStatus != string(ActionOutcomeUnknown) {
		return nil, nil
	}
	aggregate.SchemaVersion = actionOutcomeSchemaVersion
	aggregate.Expected, aggregate.Observed = decodeObject(expectedRaw), decodeObject(observedRaw)
	aggregate.GoalRefs, aggregate.IntentionRefs, aggregate.EvidenceRefs = decodeOutcomeStringArray(goalRefsRaw), decodeOutcomeStringArray(intentionRefsRaw), decodeOutcomeStringArray(evidenceRefsRaw)
	if err := json.Unmarshal(contextRaw, &aggregate.ContextReferences); err != nil {
		return nil, errors.New("action_outcome_context_references_invalid")
	}
	aggregate.Status, aggregate.SuccessBoundary, aggregate.ErrorCode = ActionOutcomeCompleted, "action_settled", ""
	if failed > 0 {
		aggregate.Status, aggregate.ErrorCode = ActionOutcomeFailed, "async_capability_failed"
	} else if cancelled > 0 {
		aggregate.Status, aggregate.ErrorCode = ActionOutcomeCancelled, "async_capability_cancelled"
	} else if suppressed > 0 {
		aggregate.Status = ActionOutcomeSuppressed
	}
	aggregate.Observed["status"] = string(aggregate.Status)
	aggregate.Revision++
	aggregate.OccurredAt = time.Now().UTC()
	if err := aggregate.Validate(); err != nil {
		return nil, err
	}
	command, err := tx.Exec(ctx, `UPDATE public.cognition_action_outcomes SET status=$2,success_boundary=$3,observed=$4,error_code=$5,revision=$6,occurred_at=$7,updated_at=now() WHERE id=$1 AND revision=$8 AND status IN ('pending','unknown')`, aggregate.ID, aggregate.Status, aggregate.SuccessBoundary, jsonBytes(aggregate.Observed), nullableString(aggregate.ErrorCode), aggregate.Revision, aggregate.OccurredAt, aggregate.Revision-1)
	if err != nil || command.RowsAffected() != 1 {
		if err == nil {
			err = ErrConflict
		}
		return nil, err
	}
	if err := settleOutcomeIntentionAttemptsTx(ctx, tx, aggregate); err != nil {
		return nil, err
	}
	return &aggregate, nil
}

func (a *App) markActionOutcomeUnknown(ctx context.Context, externalRef, reasonCode string) error {
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		_, err := a.settleActionOutcomeByExternalRefTx(ctx, tx, externalRef, ActionOutcomeUnknown, map[string]any{"status": "unknown", "reason_code": reasonCode}, reasonCode)
		return err
	})
}

func decodeOutcomeStringArray(value []byte) []string {
	var result []string
	if len(value) == 0 || json.Unmarshal(value, &result) != nil {
		return []string{}
	}
	return sortedUniqueStrings(result)
}

func (a *App) readRecentActionOutcomes(ctx context.Context, fluctlightID string, limit int) ([]map[string]any, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 24 {
		limit = 24
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,action_id,call_id,capability_name,status,success_boundary,expected,observed,COALESCE(error_code,''),goal_refs,intention_refs,evidence_refs,revision,occurred_at FROM public.cognition_action_outcomes WHERE fluctlight_id=$1 ORDER BY occurred_at DESC,id DESC LIMIT $2`, fluctlightID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var id, actionID, callID, capabilityName, status, boundary, errorCode string
		var expected, observed, goalRefs, intentionRefs, evidenceRefs []byte
		var revision int
		var occurredAt time.Time
		if err := rows.Scan(&id, &actionID, &callID, &capabilityName, &status, &boundary, &expected, &observed, &errorCode, &goalRefs, &intentionRefs, &evidenceRefs, &revision, &occurredAt); err != nil {
			return nil, err
		}
		item := map[string]any{
			"id": id, "action_id": actionID, "call_id": callID, "capability_name": capabilityName,
			"status": status, "success_boundary": boundary, "expected": decodeObject(expected),
			"observed": decodeObject(observed), "goal_refs": decodeArray(goalRefs), "intention_refs": decodeArray(intentionRefs),
			"evidence_refs": decodeArray(evidenceRefs),
			"revision":      revision, "occurred_at": occurredAt.Format(time.RFC3339Nano),
		}
		if errorCode != "" {
			item["error_code"] = errorCode
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func compactRecentActionOutcomes(values []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(values))
	for _, value := range values {
		item := map[string]any{}
		for _, key := range []string{"ref", "capability_name", "status", "success_boundary", "error_code", "goal_refs", "intention_refs", "occurred_at"} {
			if child, ok := value[key]; ok && child != nil && child != "" {
				item[key] = child
			}
		}
		if observed := providerSafeOutcomeSignal(mapValue(value["observed"]), []string{"status", "type", "label", "intensity", "target_kind", "delivery_status", "reason_code", "resulting_state_ref", "resulting_context_revision"}); len(observed) > 0 {
			item["observed"] = observed
		}
		if len(item) > 0 {
			result = append(result, item)
		}
	}
	return result
}

func providerSafeOutcomeSignal(value map[string]any, allowed []string) map[string]any {
	result := make(map[string]any)
	for _, key := range allowed {
		child, ok := value[key]
		if !ok || child == nil {
			continue
		}
		switch typed := child.(type) {
		case string:
			if text := strings.TrimSpace(typed); text != "" && len([]rune(text)) <= 128 {
				result[key] = text
			}
		case bool:
			result[key] = typed
		case float64:
			result[key] = typed
		case int:
			result[key] = typed
		}
	}
	return result
}

func sortedUniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

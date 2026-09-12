package core

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ProcessAutonomyAction settles a previously frozen action exactly once. The
// action row is authoritative; retries never re-run an already settled side
// effect and failed/cancelled actions remain observable instead of being
// silently converted into success.
func (a *App) ProcessAutonomyAction(ctx context.Context, actionID string) (map[string]any, error) {
	var fluctlightID, actionType, status, workflowID, providerRequestID string
	var payload, policySnapshotRaw, expectedRevisionsRaw []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT fluctlight_id,action_type,status,workflow_id,provider_request_id,payload,policy_snapshot,expected_revisions FROM public.autonomy_actions WHERE id=$1`, actionID).Scan(&fluctlightID, &actionType, &status, &workflowID, &providerRequestID, &payload, &policySnapshotRaw, &expectedRevisionsRaw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if status == "completed" || status == "failed" || status == "cancelled" || status == "paused" || status == "deferred" || status == "cancel_requested" {
		return map[string]any{"action_id": actionID, "action_type": actionType, "status": status}, nil
	}
	if status != "frozen" {
		return nil, fmt.Errorf("autonomy action is not executable: %s", status)
	}
	policyActionType := actionType
	reserved := false
	if value, ok := mapValue(decodeObject(policySnapshotRaw))["budget_reserved"].(bool); ok {
		reserved = value
	}
	policyEvaluator := a.evaluateAutonomyPolicy
	if reserved {
		policyEvaluator = a.evaluateAutonomyPolicyAllowReserved
	}
	policyDecision, policyErr := policyEvaluator(ctx, fluctlightID, policyActionType, time.Now().UTC(), actionID)
	if policyErr != nil {
		return nil, policyErr
	}
	if !policyDecision.Allowed {
		return a.failAutonomyAction(ctx, actionID, "policy_"+policyDecision.Reason)
	}
	expectedFoundationRevision, expectedCurrentStateRevision, expectedLifeContextRevision, revisionErr := cognitionAuthorityRevisionsFromValue(decodeObject(expectedRevisionsRaw))
	if revisionErr != nil {
		return a.failAutonomyAction(ctx, actionID, "cognition_authority_revisions_invalid")
	}
	if err := a.validateCognitionAuthorityRevisions(ctx, fluctlightID, expectedFoundationRevision, expectedCurrentStateRevision, expectedLifeContextRevision, time.Now().UTC()); err != nil {
		if code := cognitionAuthorityStaleCode(err); code != "" {
			return a.failAutonomyAction(ctx, actionID, code)
		}
		return nil, err
	}
	data := decodeObject(payload)
	if err := validateExecutableCapabilityPayload(data); err != nil {
		return a.failAutonomyAction(ctx, actionID, "capability_runtime_envelope_invalid")
	}
	if err := validateFrozenDecisionInfluences(data); err != nil {
		return a.failAutonomyAction(ctx, actionID, "context_reference_invalid")
	}
	calls, err := capabilityInvocationsFromValue(data["capability_invocations"])
	if err != nil {
		return a.failAutonomyAction(ctx, actionID, "capability_payload_invalid")
	}
	sourceFactID := firstString(data["source_fact_id"], actionID)
	storedResults, resultsErr := capabilityResultsFromValue(data["capability_results"])
	if resultsErr != nil {
		return a.failAutonomyAction(ctx, actionID, "capability_results_invalid")
	}
	duplicatePreflight := false
	if actionType == "proactive_message" {
		conversationID := stringValue(data["conversation_id"])
		text := stringValue(data["text"])
		if conversationID == "" || text == "" {
			return a.failAutonomyAction(ctx, actionID, "proactive_target_invalid")
		}
		_, duplicatePreflight, err = recentExactAssistantMessage(ctx, a.DB, conversationID, fluctlightID, text, proactiveMessageDuplicateWindow)
		if err != nil {
			return nil, err
		}
	}
	for index := range calls {
		calls[index].ActionID = actionID
	}
	if !duplicatePreflight {
		calls, err = a.prepareCapabilityInvocations(ctx, fluctlightID, stringValue(data["conversation_id"]), sourceFactID, calls, storedResults)
		if err != nil {
			code, _ := capabilityErrorInfo(err, "capability_prepare_failed", true)
			return a.failAutonomyAction(ctx, actionID, code)
		}
		if err := a.persistAutonomyCapabilityResults(ctx, actionID, calls, storedResults); err != nil {
			return nil, err
		}
	} else {
		// The locked transaction below will re-check the duplicate. Avoid planner
		// calls and capability preflight work for the common suppression path.
		calls = nil
	}
	data["capability_invocations"] = calls
	if actionType == "proactive_message" {
		conversationID := stringValue(data["conversation_id"])
		text := stringValue(data["text"])
		if conversationID == "" || text == "" {
			return a.failAutonomyAction(ctx, actionID, "proactive_target_invalid")
		}
		duplicateSuppressed := false
		duplicateMessageID := ""
		deferredCalls, _ := splitDeferredOutputCapabilities(calls, a.capabilityRegistry())
		capabilityResults := append([]CapabilityResult(nil), storedResults...)
		if len(calls) > 0 && !duplicatePreflight {
			capabilityResults, err = a.planCapabilitiesForTransaction(ctx, fluctlightID, conversationID, sourceFactID, calls, capabilityResults)
			if err != nil {
				return a.failAutonomyAction(ctx, actionID, "required_capability_failed")
			}
			if err := requiredCapabilityFailureCanonical(capabilityResults, calls, a.capabilityRegistry(), false); err != nil {
				return a.failAutonomyAction(ctx, actionID, "required_capability_failed")
			}
		}
		err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			if err := a.requireCognitionAuthorityRevisionsTx(ctx, tx, fluctlightID, expectedFoundationRevision, expectedCurrentStateRevision, expectedLifeContextRevision, time.Now().UTC()); err != nil {
				return err
			}
			messageID, duplicate, err := recentExactAssistantMessageTx(ctx, tx, conversationID, fluctlightID, text, proactiveMessageDuplicateWindow)
			if err != nil {
				return err
			}
			if duplicate {
				duplicateSuppressed = true
				duplicateMessageID = messageID
				deliveryResult := map[string]any{
					"status":          "completed",
					"action_status":   "completed",
					"delivery_status": "duplicate_suppressed",
					"message_id":      messageID,
				}
				command, err := tx.Exec(ctx, `UPDATE public.autonomy_actions SET payload=payload || $2::jsonb,status='completed',settled_at=now() WHERE id=$1 AND status='frozen'`, actionID, jsonBytes(map[string]any{"delivery_status": "duplicate_suppressed", "message_id": messageID, "capability_invocations_suppressed": true}))
				if err != nil {
					return err
				}
				if command.RowsAffected() != 1 {
					return ErrConflict
				}
				if err := a.settleWakeUpActionTx(ctx, tx, actionID, fluctlightID, deliveryResult); err != nil {
					return err
				}
				return appendOutboxTx(ctx, tx, "autonomy.action.completed", "autonomy_action", actionID, fluctlightID, actionID, "autonomy:"+actionID, "autonomy-outbox:"+actionID, map[string]any{"action_type": actionType, "status": "completed", "delivery_status": "duplicate_suppressed", "message_id": messageID, "aggregate_sequence": 1})
			}
			messageID, err = appendAssistantTxWithID(ctx, tx, conversationID, fluctlightID, text, "proactive:"+actionID)
			if err != nil {
				return err
			}
			binding := OutputBindingV1{TargetKind: "conversation_message", TargetRef: messageID}
			if len(deferredCalls) > 0 {
				for index := range deferredCalls {
					deferredCalls[index] = normalizeCapabilityInvocationMetadata(deferredCalls[index], fluctlightID, conversationID, firstString(data["source_fact_id"], actionID), actionID, index)
				}
				if err := validateCompositeOutputCapabilities(deferredCalls, binding.TargetKind, a.capabilityRegistry()); err != nil {
					return err
				}
			}
			if len(calls) > 0 {
				settled, settleErr := a.settleDeferredCapabilitiesTx(ctx, tx, fluctlightID, firstString(data["source_fact_id"], actionID), actionID, calls, capabilityResults, binding)
				if settleErr != nil {
					return settleErr
				}
				capabilityResults = settled
				if requiredErr := requiredCapabilityFailureCanonical(capabilityResults, calls, a.capabilityRegistry(), true); requiredErr != nil {
					return requiredErr
				}
			}
			if len(deferredCalls) > 0 {
				bound := make([]OutputBindingV1, 0, len(deferredCalls))
				for _, invocation := range deferredCalls {
					bound = append(bound, OutputBindingV1{ToolCallID: invocation.CallID, TargetKind: binding.TargetKind, TargetRef: binding.TargetRef})
				}
				if _, err := tx.Exec(ctx, `UPDATE public.autonomy_actions SET payload=jsonb_set(payload,'{output_bindings}',$2::jsonb,true) WHERE id=$1 AND status='frozen'`, actionID, jsonBytes(bound)); err != nil {
					return err
				}
			}
			if _, err := tx.Exec(ctx, `UPDATE public.autonomy_actions SET payload=jsonb_set(jsonb_set(payload,'{capability_invocations}',$2::jsonb,true),'{capability_results}',$3::jsonb,true) WHERE id=$1 AND status='frozen'`, actionID, jsonBytes(calls), jsonBytes(capabilityResults)); err != nil {
				return err
			}
			command, err := tx.Exec(ctx, `UPDATE public.autonomy_actions SET status='completed',settled_at=now() WHERE id=$1 AND status='frozen'`, actionID)
			if err != nil {
				return err
			}
			if command.RowsAffected() != 1 {
				return ErrConflict
			}
			if err := a.settleWakeUpActionTx(ctx, tx, actionID, fluctlightID, map[string]any{"status": "completed", "action_status": "completed", "message_id": messageID, "capability_results": capabilityResults}); err != nil {
				return err
			}
			if err := a.enqueueConversationSummaryIntentTx(ctx, tx, fluctlightID, conversationID, messageID); err != nil {
				return err
			}
			return appendOutboxTx(ctx, tx, "autonomy.action.completed", "autonomy_action", actionID, fluctlightID, actionID, "autonomy:"+actionID, "autonomy-outbox:"+actionID, map[string]any{"action_type": actionType, "status": "completed", "aggregate_sequence": 1})
		})
		if err != nil {
			failed := capabilityResultsAfterSettlementFailure(capabilityResults, calls, a.capabilityRegistry(), "capability_settlement_failed")
			_ = a.persistAutonomyCapabilityResults(ctx, actionID, calls, failed)
			code, retryable := capabilityFailureInfo(err, failed, calls, a.capabilityRegistry(), "capability_settlement_failed")
			if !retryable {
				return a.failAutonomyAction(ctx, actionID, code)
			}
			return nil, err
		}
		if duplicateSuppressed {
			return map[string]any{"action_id": actionID, "action_type": actionType, "status": "completed", "delivery_status": "duplicate_suppressed", "message_id": duplicateMessageID}, nil
		}
		return map[string]any{"action_id": actionID, "action_type": actionType, "status": "completed"}, nil
	}
	if actionType == "moment" {
		text := stringValue(data["text"])
		if text == "" {
			return a.failAutonomyAction(ctx, actionID, "moment_text_invalid")
		}
		momentID := "moment_" + stableDigest(actionID)
		deferredCalls, _ := splitDeferredOutputCapabilities(calls, a.capabilityRegistry())
		capabilityResults := append([]CapabilityResult(nil), storedResults...)
		if len(calls) > 0 {
			capabilityResults, err = a.planCapabilitiesForTransaction(ctx, fluctlightID, stringValue(data["conversation_id"]), sourceFactID, calls, capabilityResults)
			if err != nil {
				return a.failAutonomyAction(ctx, actionID, "required_capability_failed")
			}
			if err := requiredCapabilityFailureCanonical(capabilityResults, calls, a.capabilityRegistry(), false); err != nil {
				return a.failAutonomyAction(ctx, actionID, "required_capability_failed")
			}
		}
		settlementErr := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			if err := a.requireCognitionAuthorityRevisionsTx(ctx, tx, fluctlightID, expectedFoundationRevision, expectedCurrentStateRevision, expectedLifeContextRevision, time.Now().UTC()); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.moments(id,owner_fluctlight_id,author_actor_id,text,visibility,status,media_asset_ids) VALUES($1,$2,$2,$3,'participants','visible','[]') ON CONFLICT DO NOTHING`, momentID, fluctlightID, text); err != nil {
				return err
			}
			binding := OutputBindingV1{TargetKind: "moment", TargetRef: momentID}
			if len(deferredCalls) > 0 {
				for index := range deferredCalls {
					deferredCalls[index] = normalizeCapabilityInvocationMetadata(deferredCalls[index], fluctlightID, stringValue(data["conversation_id"]), firstString(data["source_fact_id"], actionID), actionID, index)
				}
				if err := validateCompositeOutputCapabilities(deferredCalls, binding.TargetKind, a.capabilityRegistry()); err != nil {
					return err
				}
			}
			if len(calls) > 0 {
				settled, settleErr := a.settleDeferredCapabilitiesTx(ctx, tx, fluctlightID, firstString(data["source_fact_id"], actionID), actionID, calls, capabilityResults, binding)
				if settleErr != nil {
					return settleErr
				}
				capabilityResults = settled
				if requiredErr := requiredCapabilityFailureCanonical(capabilityResults, calls, a.capabilityRegistry(), true); requiredErr != nil {
					return requiredErr
				}
			}
			if len(deferredCalls) > 0 {
				bound := make([]OutputBindingV1, 0, len(deferredCalls))
				for _, invocation := range deferredCalls {
					bound = append(bound, OutputBindingV1{ToolCallID: invocation.CallID, TargetKind: binding.TargetKind, TargetRef: binding.TargetRef})
				}
				if _, err := tx.Exec(ctx, `UPDATE public.autonomy_actions SET payload=jsonb_set(payload,'{output_bindings}',$2::jsonb,true) WHERE id=$1 AND status='frozen'`, actionID, jsonBytes(bound)); err != nil {
					return err
				}
			}
			if _, err := tx.Exec(ctx, `UPDATE public.autonomy_actions SET payload=jsonb_set(jsonb_set(payload,'{capability_invocations}',$2::jsonb,true),'{capability_results}',$3::jsonb,true) WHERE id=$1 AND status='frozen'`, actionID, jsonBytes(calls), jsonBytes(capabilityResults)); err != nil {
				return err
			}
			command, err := tx.Exec(ctx, `UPDATE public.autonomy_actions SET status='completed',settled_at=now() WHERE id=$1 AND status='frozen'`, actionID)
			if err != nil {
				return err
			}
			if command.RowsAffected() != 1 {
				return ErrConflict
			}
			if err := a.settleWakeUpActionTx(ctx, tx, actionID, fluctlightID, map[string]any{"status": "completed", "action_status": "completed", "moment_id": momentID, "capability_results": capabilityResults}); err != nil {
				return err
			}
			return appendOutboxTx(ctx, tx, "moment.published", "moment", momentID, fluctlightID, actionID, "autonomy:"+actionID, "moment-outbox:"+actionID, map[string]any{"moment_id": momentID, "action_id": actionID, "aggregate_sequence": 1})
		})
		if settlementErr != nil {
			failed := capabilityResultsAfterSettlementFailure(capabilityResults, calls, a.capabilityRegistry(), "capability_settlement_failed")
			_ = a.persistAutonomyCapabilityResults(ctx, actionID, calls, failed)
			code, retryable := capabilityFailureInfo(settlementErr, failed, calls, a.capabilityRegistry(), "capability_settlement_failed")
			if !retryable {
				return a.failAutonomyAction(ctx, actionID, code)
			}
			return nil, settlementErr
		}
		return map[string]any{"action_id": actionID, "action_type": actionType, "status": "completed", "moment_id": momentID}, nil
	}
	return a.failAutonomyAction(ctx, actionID, "unsupported_action_type")
}

func (a *App) ProcessCapabilityAction(ctx context.Context, actionID string) (map[string]any, error) {
	var fluctlightID, status string
	var payload, policySnapshotRaw, expectedRevisionsRaw []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT fluctlight_id,status,payload,policy_snapshot,expected_revisions FROM public.autonomy_actions WHERE id=$1`, actionID).Scan(&fluctlightID, &status, &payload, &policySnapshotRaw, &expectedRevisionsRaw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if status == "completed" || status == "failed" || status == "cancelled" || status == "paused" || status == "deferred" || status == "cancel_requested" {
		return map[string]any{"action_id": actionID, "action_type": "capability", "status": status}, nil
	}
	if status != "frozen" {
		return nil, fmt.Errorf("capability action is not executable: %s", status)
	}
	reserved := false
	if value, ok := mapValue(decodeObject(policySnapshotRaw))["budget_reserved"].(bool); ok {
		reserved = value
	}
	policyEvaluator := a.evaluateAutonomyPolicy
	if reserved {
		policyEvaluator = a.evaluateAutonomyPolicyAllowReserved
	}
	policyDecision, policyErr := policyEvaluator(ctx, fluctlightID, "capability", time.Now().UTC(), actionID)
	if policyErr != nil {
		return nil, policyErr
	}
	if !policyDecision.Allowed {
		return a.failAutonomyAction(ctx, actionID, "policy_"+policyDecision.Reason)
	}
	expectedFoundationRevision, expectedCurrentStateRevision, expectedLifeContextRevision, revisionErr := cognitionAuthorityRevisionsFromValue(decodeObject(expectedRevisionsRaw))
	if revisionErr != nil {
		return a.failAutonomyAction(ctx, actionID, "cognition_authority_revisions_invalid")
	}
	if err := a.validateCognitionAuthorityRevisions(ctx, fluctlightID, expectedFoundationRevision, expectedCurrentStateRevision, expectedLifeContextRevision, time.Now().UTC()); err != nil {
		if code := cognitionAuthorityStaleCode(err); code != "" {
			return a.failAutonomyAction(ctx, actionID, code)
		}
		return nil, err
	}
	data := decodeObject(payload)
	if err := validateExecutableCapabilityPayload(data); err != nil {
		return a.failAutonomyAction(ctx, actionID, "capability_runtime_envelope_invalid")
	}
	if err := validateFrozenDecisionInfluences(data); err != nil {
		return a.failAutonomyAction(ctx, actionID, "context_reference_invalid")
	}
	calls, invocationErr := capabilityInvocationsFromValue(data["capability_invocations"])
	if invocationErr != nil {
		return a.failAutonomyAction(ctx, actionID, "capability_payload_invalid")
	}
	if len(calls) == 0 {
		return a.failAutonomyAction(ctx, actionID, "capability_calls_empty")
	}
	var sourceFactID string
	sourceFactID = stringValue(data["source_fact_id"])
	if sourceFactID == "" && len(calls) > 0 {
		sourceFactID = calls[0].SourceFactID
	}
	if sourceFactID == "" {
		return a.failAutonomyAction(ctx, actionID, "capability_source_missing")
	}
	for index := range calls {
		calls[index].ActionID = actionID
	}
	results, resultsErr := capabilityResultsFromValue(data["capability_results"])
	if resultsErr != nil {
		return a.failAutonomyAction(ctx, actionID, "capability_results_invalid")
	}
	preparedCalls, prepareErr := a.prepareCapabilityInvocations(ctx, fluctlightID, stringValue(data["conversation_id"]), sourceFactID, calls, results)
	if prepareErr != nil {
		code, _ := capabilityErrorInfo(prepareErr, "capability_prepare_failed", true)
		return a.failAutonomyAction(ctx, actionID, code)
	}
	calls = preparedCalls
	data["capability_invocations"] = calls
	if err := a.persistAutonomyCapabilityResults(ctx, actionID, calls, results); err != nil {
		return nil, err
	}
	for _, invocation := range calls {
		definition, ok := a.capabilityRegistry().Definition(invocation.CapabilityName)
		if !ok || invocation.Validate(definition) != nil {
			return a.failAutonomyAction(ctx, actionID, "capability_unavailable")
		}
	}
	for index := range calls {
		calls[index] = normalizeCapabilityInvocationMetadata(calls[index], fluctlightID, stringValue(data["conversation_id"]), sourceFactID, actionID, index)
		calls[index].ActionID = actionID
	}
	var planErr error
	results, planErr = a.planCapabilitiesForTransaction(ctx, fluctlightID, stringValue(data["conversation_id"]), sourceFactID, calls, results)
	if planErr != nil {
		_ = a.persistAutonomyCapabilityResults(ctx, actionID, calls, results)
		code, retryable := capabilityFailureInfo(planErr, results, calls, a.capabilityRegistry(), "capability_plan_failed")
		if !retryable {
			return a.failAutonomyAction(ctx, actionID, code)
		}
		return nil, planErr
	}
	if requiredErr := requiredCapabilityFailureCanonical(results, calls, a.capabilityRegistry(), false); requiredErr != nil {
		_ = a.persistAutonomyCapabilityResults(ctx, actionID, calls, results)
		return a.failAutonomyAction(ctx, actionID, "required_capability_failed")
	}
	result := map[string]any{}
	settlementErr := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := a.requireCognitionAuthorityRevisionsTx(ctx, tx, fluctlightID, expectedFoundationRevision, expectedCurrentStateRevision, expectedLifeContextRevision, time.Now().UTC()); err != nil {
			return err
		}
		settled, settleErr := a.settleDeferredCapabilitiesTx(ctx, tx, fluctlightID, sourceFactID, actionID, calls, results, OutputBindingV1{TargetKind: "wake_up", TargetRef: actionID})
		if settleErr != nil {
			return settleErr
		}
		results = settled
		if requiredErr := requiredCapabilityFailureCanonical(results, calls, a.capabilityRegistry(), true); requiredErr != nil {
			return requiredErr
		}
		result = map[string]any{"status": "completed", "action_status": "completed", "capability_results": results}
		if _, err := tx.Exec(ctx, `UPDATE public.autonomy_actions SET payload=jsonb_set(jsonb_set(payload,'{capability_invocations}',$2::jsonb,true),'{capability_results}',$3::jsonb,true) WHERE id=$1 AND status='frozen'`, actionID, jsonBytes(calls), jsonBytes(results)); err != nil {
			return err
		}
		command, err := tx.Exec(ctx, `UPDATE public.autonomy_actions SET status='completed',settled_at=now() WHERE id=$1 AND status='frozen'`, actionID)
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return ErrConflict
		}
		return a.settleWakeUpActionTx(ctx, tx, actionID, fluctlightID, result)
	})
	if settlementErr != nil {
		failed := capabilityResultsAfterSettlementFailure(results, calls, a.capabilityRegistry(), "capability_settlement_failed")
		_ = a.persistAutonomyCapabilityResults(ctx, actionID, calls, failed)
		code, retryable := capabilityFailureInfo(settlementErr, failed, calls, a.capabilityRegistry(), "capability_settlement_failed")
		if !retryable {
			return a.failAutonomyAction(ctx, actionID, code)
		}
		return nil, settlementErr
	}
	return map[string]any{"action_id": actionID, "action_type": "capability", "status": "completed", "capability_results": results}, nil
}

func (a *App) persistAutonomyCapabilityResults(ctx context.Context, actionID string, invocations []CapabilityInvocation, results []CapabilityResult) error {
	if a == nil || a.DB == nil || strings.TrimSpace(actionID) == "" {
		return errors.New("autonomy_action_persistence_unavailable")
	}
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		command, err := tx.Exec(ctx, `UPDATE public.autonomy_actions SET payload=jsonb_set(jsonb_set(payload,'{capability_invocations}',$2::jsonb,true),'{capability_results}',$3::jsonb,true) WHERE id=$1 AND status='frozen'`, actionID, jsonBytes(invocations), jsonBytes(results))
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return ErrConflict
		}
		return nil
	})
}

func (a *App) failAutonomyAction(ctx context.Context, actionID, code string) (map[string]any, error) {
	result := map[string]any{"action_id": actionID, "status": "failed", "action_status": "failed", "error_code": code}
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var fluctlightID, actionType, status string
		var rawPayload []byte
		if err := tx.QueryRow(ctx, `SELECT fluctlight_id,action_type,status,payload FROM public.autonomy_actions WHERE id=$1 FOR UPDATE`, actionID).Scan(&fluctlightID, &actionType, &status, &rawPayload); err != nil {
			return err
		}
		if status != "frozen" && status != "running" {
			return ErrConflict
		}
		payload := decodeObject(rawPayload)
		causality, err := frozenDecisionCausality(payload)
		if err != nil {
			return err
		}
		settlement := cloneMap(result)
		for key, value := range causality {
			settlement[key] = value
		}
		capabilityResults, resultsErr := capabilityResultsFromValue(payload["capability_results"])
		if resultsErr != nil {
			capabilityResults = nil
			settlement["reason_code"] = "capability_results_invalid"
		}
		sourceFactID := firstString(payload["source_fact_id"], actionID)
		outcomes, outcomeErr := buildActionOutcomes(actionID, fluctlightID, sourceFactID, actionType, capabilityResults, settlement, a.capabilityRegistry())
		if outcomeErr != nil {
			outcomes, outcomeErr = buildActionOutcomes(actionID, fluctlightID, sourceFactID, actionType, nil, settlement, a.capabilityRegistry())
		}
		if outcomeErr != nil {
			return outcomeErr
		}
		if err := persistActionOutcomesTx(ctx, tx, outcomes); err != nil {
			return err
		}
		command, err := tx.Exec(ctx, `UPDATE public.autonomy_actions SET status='failed',error_code=$2,settled_at=now() WHERE id=$1 AND status IN ('frozen','running')`, actionID, code)
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return ErrConflict
		}
		if _, err := tx.Exec(ctx, `UPDATE public.cognition_wakeups SET result=result || $2::jsonb WHERE action_id=$1`, actionID, jsonBytes(settlement)); err != nil {
			return err
		}
		factPayload := map[string]any{"action_id": actionID, "result": settlement, "outcomes": outcomes}
		for key, value := range causality {
			factPayload[key] = value
		}
		factID, err := appendProcessedCognitionFactTx(ctx, tx, fluctlightID, "autonomy.result", factPayload, "action-result:"+actionID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'lifecycle','reflection.run',$3) ON CONFLICT DO NOTHING`, "reflection_intent:result:"+actionID, "reflection:result:"+actionID, jsonBytes(map[string]any{"fluctlight_id": fluctlightID, "source_fact_id": factID, "action_id": actionID})); err != nil {
			return err
		}
		return appendOutboxTx(ctx, tx, "autonomy.result.recorded", "fluctlight", fluctlightID, fluctlightID, actionID, "action-result:"+actionID, "action-result:"+actionID, factPayload)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (a *App) settleWakeUpActionTx(ctx context.Context, tx pgx.Tx, actionID, fluctlightID string, result map[string]any) error {
	var actionPayload []byte
	var actionType string
	if err := tx.QueryRow(ctx, `SELECT action_type,payload FROM public.autonomy_actions WHERE id=$1`, actionID).Scan(&actionType, &actionPayload); err != nil {
		return err
	}
	causality, err := frozenDecisionCausality(decodeObject(actionPayload))
	if err != nil {
		return err
	}
	settledResult := cloneMap(result)
	for key, value := range causality {
		settledResult[key] = value
	}
	if _, err := tx.Exec(ctx, `UPDATE public.cognition_wakeups SET result=result || $2::jsonb WHERE action_id=$1`, actionID, jsonBytes(settledResult)); err != nil {
		return err
	}
	results, err := capabilityResultsFromValue(settledResult["capability_results"])
	if err != nil {
		return err
	}
	sourceFactID := firstString(decodeObject(actionPayload)["source_fact_id"], actionID)
	outcomes, err := buildActionOutcomes(actionID, fluctlightID, sourceFactID, actionType, results, settledResult, a.capabilityRegistry())
	if err != nil {
		return err
	}
	if err := persistActionOutcomesTx(ctx, tx, outcomes); err != nil {
		return err
	}
	factPayload := map[string]any{"action_id": actionID, "result": settledResult, "outcomes": outcomes}
	for key, value := range causality {
		factPayload[key] = value
	}
	factID, err := appendProcessedCognitionFactTx(ctx, tx, fluctlightID, "autonomy.result", factPayload, "action-result:"+actionID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'lifecycle','reflection.run',$3) ON CONFLICT DO NOTHING`, "reflection_intent:result:"+actionID, "reflection:result:"+actionID, jsonBytes(map[string]any{"fluctlight_id": fluctlightID, "source_fact_id": factID, "action_id": actionID}))
	return err
}

func (a *App) ProcessReflection(ctx context.Context, fluctlightID, correlationID string) (map[string]any, error) {
	// Reflection is the evidence-windowed learning pass over processed
	// cognition facts. A wake-up may create the fact that feeds this window,
	// but reflection never substitutes for the periodic wake-up trigger.
	if fluctlightID == "" {
		return nil, fmt.Errorf("reflection_fluctlight_id_required")
	}
	fluctlight, err := a.readFluctlightByID(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	if fluctlight.Status == "paused" {
		return map[string]any{"fluctlight_id": fluctlightID, "correlation_id": correlationID, "status": "paused", "reason": "fluctlight_paused"}, nil
	}
	if fluctlight.Status != "active" {
		return map[string]any{"fluctlight_id": fluctlightID, "correlation_id": correlationID, "status": "inactive", "reason": "fluctlight_not_active"}, nil
	}
	ctx = WithProviderExecutionGuard(ctx, a.providerGuardForFluctlight(fluctlightID))
	var watermark, stateRevision int
	if err := a.DB.Pool().QueryRow(ctx, `SELECT watermark,state_revision FROM public.cognition_reflection_windows WHERE fluctlight_id=$1`, fluctlightID).Scan(&watermark, &stateRevision); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		_ = a.DB.Pool().QueryRow(ctx, `SELECT revision FROM public.fluctlight_inner_states WHERE fluctlight_id=$1`, fluctlightID).Scan(&stateRevision)
	}
	var currentStateRevision int
	if err := a.DB.Pool().QueryRow(ctx, `SELECT revision FROM public.fluctlight_inner_states WHERE fluctlight_id=$1`, fluctlightID).Scan(&currentStateRevision); err == nil && currentStateRevision > stateRevision {
		stateRevision = currentStateRevision
	}
	if err := a.claimReflectionWindow(ctx, fluctlightID, watermark, stateRevision); err != nil {
		return nil, err
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,sequence,event_type,payload,occurred_at FROM public.cognition_inbox WHERE fluctlight_id=$1 AND sequence>$2 AND status='processed' ORDER BY sequence LIMIT 20`, fluctlightID, watermark)
	if err != nil {
		_ = a.setReflectionWindowIdle(ctx, fluctlightID)
		return nil, err
	}
	evidence := make([]map[string]any, 0)
	allowedEvidence := make(map[string]struct{})
	memoryAllowedEvidence := make(map[string]struct{})
	memoryEvidenceScopes := make(map[string]reflectionMemoryEvidenceScope)
	toSequence := watermark
	for rows.Next() {
		var id string
		var sequence int
		var typ string
		var payload []byte
		var occurredAt time.Time
		if err := rows.Scan(&id, &sequence, &typ, &payload, &occurredAt); err != nil {
			rows.Close()
			_ = a.setReflectionWindowIdle(ctx, fluctlightID)
			return nil, err
		}
		evidenceRef := fmt.Sprintf("sequence:%d", sequence)
		allowedEvidence[evidenceRef] = struct{}{}
		memoryAllowedEvidence[evidenceRef] = struct{}{}
		decodedPayload := decodeJSONValue(payload)
		memoryEvidenceScopes[evidenceRef] = reflectionMemoryEvidenceScope{FactID: id, ConversationID: stringValue(mapValue(decodedPayload)["conversation_id"]), Known: true}
		evidence = append(evidence, map[string]any{"id": id, "sequence": sequence, "event_type": typ, "payload": decodedPayload, "occurred_at": occurredAt})
		if sequence > toSequence {
			toSequence = sequence
		}
	}
	rows.Close()
	// Appraisal is an authoritative semantic interpretation of each processed
	// fact. Merge it into the corresponding source event rather than appending a
	// second evidence row with the window's maximum sequence. This keeps one
	// evidence item per source sequence and prevents reflection prompts from
	// appearing to accumulate duplicate cognition records.
	appraisalRows, appraisalErr := a.DB.Pool().Query(ctx, `SELECT id,source_fact_id,payload,evidence_refs FROM public.cognition_appraisals WHERE fluctlight_id=$1 AND source_fact_id IN (SELECT id FROM public.cognition_inbox WHERE fluctlight_id=$1 AND sequence>$2 AND sequence<=$3 AND status='processed')`, fluctlightID, watermark, toSequence)
	if appraisalErr != nil {
		_ = a.setReflectionWindowIdle(ctx, fluctlightID)
		return nil, appraisalErr
	}
	appraisalsByFact := make(map[string]map[string]any)
	{
		for appraisalRows.Next() {
			var id, sourceFactID string
			var payload, refs []byte
			if scanErr := appraisalRows.Scan(&id, &sourceFactID, &payload, &refs); scanErr != nil {
				appraisalRows.Close()
				_ = a.setReflectionWindowIdle(ctx, fluctlightID)
				return nil, scanErr
			}
			appraisalsByFact[sourceFactID] = map[string]any{"payload": decodeJSONValue(payload), "evidence_refs": decodeArray(refs)}
		}
		appraisalRows.Close()
	}
	for _, item := range evidence {
		if appraisal := appraisalsByFact[stringValue(item["id"])]; len(appraisal) > 0 {
			item["appraisal"] = appraisal["payload"]
			item["appraisal_evidence_refs"] = appraisal["evidence_refs"]
		}
	}
	if len(evidence) == 0 {
		_ = a.setReflectionWindowIdle(ctx, fluctlightID)
		return map[string]any{"fluctlight_id": fluctlightID, "correlation_id": correlationID, "status": "no_op", "watermark": watermark}, nil
	}
	var ownerActorID string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT created_by_actor_id FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&ownerActorID); err != nil {
		_ = a.setReflectionWindowIdle(ctx, fluctlightID)
		return nil, err
	}
	conversationSet := make(map[string]struct{})
	memoryCues := make([]MemoryQueryCue, 0, len(evidence)*2)
	for _, item := range evidence {
		memoryCues = append(memoryCues, MemoryQueryCue{Kind: "reflection_event_type", Text: stringValue(item["event_type"])})
		payload := mapValue(item["payload"])
		conversationID := strings.TrimSpace(stringValue(payload["conversation_id"]))
		if conversationID == "" {
			sourceFactID := strings.TrimSpace(stringValue(payload["source_fact_id"]))
			if sourceFactID != "" {
				sourceErr := a.DB.Pool().QueryRow(ctx, `SELECT COALESCE(payload->>'conversation_id','') FROM public.cognition_inbox WHERE id=$1 AND fluctlight_id=$2`, sourceFactID, fluctlightID).Scan(&conversationID)
				if sourceErr != nil && !errors.Is(sourceErr, pgx.ErrNoRows) {
					_ = a.setReflectionWindowIdle(ctx, fluctlightID)
					return nil, sourceErr
				}
			}
		}
		if conversationID != "" {
			conversationSet[conversationID] = struct{}{}
			evidenceRef := "sequence:" + fmt.Sprint(item["sequence"])
			scope := memoryEvidenceScopes[evidenceRef]
			scope.ConversationID = conversationID
			memoryEvidenceScopes[evidenceRef] = scope
		}
		if appraisal := mapValue(item["appraisal"]); len(appraisal) > 0 {
			memoryCues = append(memoryCues, MemoryQueryCue{Kind: "reflection_appraisal", Text: jsonString(appraisal)})
		}
	}
	allowedConversationIDs := make([]string, 0, len(conversationSet))
	for conversationID := range conversationSet {
		allowedConversationIDs = append(allowedConversationIDs, conversationID)
	}
	sort.Strings(allowedConversationIDs)
	memoryMode := MemoryConversationGlobalOnly
	if len(allowedConversationIDs) > 0 {
		memoryMode = MemoryConversationAllowedSet
	}
	projection, err := a.BuildContextProjectionFor(ctx, ContextProjectionRequest{
		AuthorizationActorID: ownerActorID, SpeakerActorID: ownerActorID, FluctlightID: fluctlightID,
		SourceFactID: "reflection:" + fluctlightID, MemoryOperation: MemoryForReflection,
		MemoryConversationMode: memoryMode, AllowedConversationIDs: allowedConversationIDs, MemoryCues: memoryCues,
	})
	if err != nil {
		_ = a.setReflectionWindowIdle(ctx, fluctlightID)
		return nil, err
	}
	for ref, entry := range projection.ReferenceIndex.ByRef {
		allowedEvidence[ref] = struct{}{}
		switch entry.Kind {
		case ContextReferenceMemory:
			memoryAllowedEvidence[ref] = struct{}{}
			snapshot := decodeObject(entry.Snapshot)
			memoryEvidenceScopes[ref] = reflectionMemoryEvidenceScope{ConversationID: stringValue(snapshot["conversation_id"]), Known: true}
		case ContextReferenceOutcome:
			memoryAllowedEvidence[ref] = struct{}{}
			scope, scopeErr := a.reflectionOutcomeEvidenceScope(ctx, fluctlightID, entry)
			if scopeErr != nil {
				_ = a.setReflectionWindowIdle(ctx, fluctlightID)
				return nil, scopeErr
			}
			memoryEvidenceScopes[ref] = scope
		}
	}
	return a.processReflectionV2(ctx, fluctlightID, ownerActorID, correlationID, watermark, toSequence, stateRevision, evidence, projection, memoryAllowedEvidence, memoryEvidenceScopes)
}

func boundedNumber(value any, fallback float64) float64 {
	parsed, ok := numberFloat(value)
	if !ok || parsed < 0 || parsed > 1 {
		return fallback
	}
	return parsed
}

func defaultGoalNumber(value any, fallback float64) float64 {
	if parsed, ok := numberFloat(value); ok && parsed >= 0 && parsed <= 1 {
		return parsed
	}
	return fallback
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (a *App) ProcessMemoryEmbedding(ctx context.Context, memoryID string) (map[string]any, error) {
	return a.ProcessMemoryEmbeddingAt(ctx, memoryID, 0)
}

func (a *App) ProcessMemoryEmbeddingAt(ctx context.Context, memoryID string, requestedRevision int) (map[string]any, error) {
	return a.ProcessMemoryEmbeddingIntentAt(ctx, "", memoryID, requestedRevision, "", "")
}

// EnsureCurrentDaySchedule ensures that the persona has an LLM-generated,
// fully validated Schedule for its current local day. The Provider owns the
// semantic plan; Go only normalizes the structural JSON shape, validates the
// timezone/day coverage and commits the accepted projection transactionally.
func (a *App) EnsureCurrentDaySchedule(ctx context.Context, fluctlightID string) (map[string]any, error) {
	var timezone, ownerID string
	var identity, lifeProfile []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT created_by_actor_id,COALESCE(identity->>'timezone','Asia/Shanghai'),identity,life_profile FROM public.fluctlights WHERE id=$1 AND status <> 'retired'`, fluctlightID).Scan(&ownerID, &timezone, &identity, &lifeProfile); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	timezone = canonicalTimezone(timezone)
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, fmt.Errorf("schedule_timezone_invalid: %w", err)
	}
	projectionAt := time.Now().UTC()
	schedule, life, err := a.readLifeContextSnapshotAt(ctx, fluctlightID, projectionAt)
	if err != nil {
		return nil, err
	}
	if stringValue(life["timezone"]) != timezone {
		return nil, ErrLifeContextStale
	}
	localDate := stringValue(life["local_date"])
	if localDate == "" {
		return nil, errors.New("schedule_local_date_invalid")
	}
	if schedule != nil {
		return map[string]any{"fluctlight_id": fluctlightID, "local_date": localDate, "schedule_id": schedule["id"], "status": "ready"}, nil
	}
	generated, generateErr := a.generateInitialSchedule(ctx, ownerID, fluctlightID, localDate, timezone, stringValue(life["context_revision"]), decodeObject(identity), decodeObject(lifeProfile))
	if generateErr != nil {
		// Provider outage/invalid structured output and a projection that became
		// stale during planning are both durable pending states. The workflow
		// retries from a fresh snapshot and never commits the old plan.
		return map[string]any{"fluctlight_id": fluctlightID, "local_date": localDate, "timezone": timezone, "status": "pending", "error_code": "schedule_generation_failed"}, nil
	}
	return generated, nil
}

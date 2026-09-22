package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ProcessAutonomyAction resumes a frozen product command through the same
// standalone Tool boundary used by formal Agents. The action row remains the
// durable workflow authority, while ExecuteTool owns preparation, idempotency,
// the short mutation transaction, and publication.
func (a *App) ProcessAutonomyAction(ctx context.Context, actionID string) (map[string]any, error) {
	return a.processFrozenToolAction(ctx, actionID)
}

// ProcessCapabilityAction is the workflow alias for capability.action intents.
// Both intent kinds consume the same frozen command and never dispatch a second
// capability runtime.
func (a *App) ProcessCapabilityAction(ctx context.Context, actionID string) (map[string]any, error) {
	return a.processFrozenToolAction(ctx, actionID)
}

type frozenToolAction struct {
	ID                   string
	FluctlightID         string
	ActionType           string
	Status               string
	Payload              map[string]any
	PolicySnapshot       map[string]any
	ExpectedRevisions    map[string]any
	AuthorizationActorID string
	ConversationID       string
	SourceFactID         string
	CorrelationID        string
	Invocations          []CapabilityInvocation
	Results              []CapabilityResult
}

func (a *App) loadFrozenToolAction(ctx context.Context, actionID string) (frozenToolAction, error) {
	action := frozenToolAction{ID: strings.TrimSpace(actionID)}
	if action.ID == "" {
		return action, ErrInvalidArguments
	}
	var payloadRaw, policyRaw, revisionsRaw []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT fluctlight_id,action_type,status,payload,policy_snapshot,expected_revisions FROM public.autonomy_actions WHERE id=$1`, action.ID).Scan(
		&action.FluctlightID, &action.ActionType, &action.Status, &payloadRaw, &policyRaw, &revisionsRaw,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return action, ErrNotFound
		}
		return action, err
	}
	action.Payload = decodeObject(payloadRaw)
	action.PolicySnapshot = decodeObject(policyRaw)
	action.ExpectedRevisions = decodeObject(revisionsRaw)
	action.ConversationID = strings.TrimSpace(stringValue(action.Payload["conversation_id"]))
	action.SourceFactID = firstString(action.Payload["source_fact_id"], action.ID)
	action.CorrelationID = firstString(action.Payload["correlation_id"], "action-result:"+action.ID)
	if err := a.DB.Pool().QueryRow(ctx, `SELECT created_by_actor_id FROM public.fluctlights WHERE id=$1`, action.FluctlightID).Scan(&action.AuthorizationActorID); err != nil {
		return action, err
	}
	var err error
	action.Invocations, err = capabilityInvocationsFromValue(action.Payload["capability_invocations"])
	if err != nil {
		return action, errors.New("capability_payload_invalid")
	}
	action.Results, err = capabilityResultsFromValue(action.Payload["capability_results"])
	if err != nil {
		return action, errors.New("capability_results_invalid")
	}
	return action, nil
}

func (a *App) processFrozenToolAction(ctx context.Context, actionID string) (map[string]any, error) {
	action, err := a.loadFrozenToolAction(ctx, actionID)
	if err != nil {
		return nil, err
	}
	switch action.Status {
	case "completed", "failed", "cancelled", "paused", "deferred", "cancel_requested":
		return map[string]any{"action_id": action.ID, "action_type": action.ActionType, "status": action.Status}, nil
	case "frozen":
	default:
		return nil, fmt.Errorf("autonomy action is not executable: %s", action.Status)
	}
	if err := validateExecutableCapabilityPayload(action.Payload); err != nil {
		return a.failAutonomyAction(ctx, action.ID, "capability_runtime_envelope_invalid")
	}
	if err := validateFrozenDecisionInfluences(action.Payload); err != nil {
		return a.failAutonomyAction(ctx, action.ID, "context_reference_invalid")
	}
	reserved, _ := action.PolicySnapshot["budget_reserved"].(bool)
	policyEvaluator := a.evaluateAutonomyPolicy
	if reserved {
		policyEvaluator = a.evaluateAutonomyPolicyAllowReserved
	}
	policyDecision, err := policyEvaluator(ctx, action.FluctlightID, action.ActionType, a.now().UTC(), action.ID)
	if err != nil {
		return nil, err
	}
	if !policyDecision.Allowed {
		return a.failAutonomyAction(ctx, action.ID, "policy_"+policyDecision.Reason)
	}
	expectedFoundation, expectedState, expectedLife, err := cognitionAuthorityRevisionsFromValue(action.ExpectedRevisions)
	if err != nil {
		return a.failAutonomyAction(ctx, action.ID, "cognition_authority_revisions_invalid")
	}
	if err := a.validateCognitionAuthorityRevisions(ctx, action.FluctlightID, expectedFoundation, expectedState, expectedLife, a.now().UTC()); err != nil {
		if code := cognitionAuthorityStaleCode(err); code != "" {
			return a.failAutonomyAction(ctx, action.ID, code)
		}
		return nil, err
	}
	action.Invocations, err = materializeActionPublicationInvocation(action)
	if err != nil {
		return a.failAutonomyAction(ctx, action.ID, "action_output_invalid")
	}
	if action.ActionType == "proactive_message" {
		text := actionPublicationText(action.Invocations, conversationReplyCapabilityName)
		if action.ConversationID == "" || text == "" {
			return a.failAutonomyAction(ctx, action.ID, "proactive_target_invalid")
		}
		messageID, duplicate, duplicateErr := recentExactAssistantMessage(ctx, a.DB, action.ConversationID, action.FluctlightID, text, proactiveMessageDuplicateWindow)
		if duplicateErr != nil {
			return nil, duplicateErr
		}
		if duplicate {
			return a.settleFrozenToolAction(ctx, action, map[string]any{
				"delivery_status": "duplicate_suppressed", "message_id": messageID,
				"capability_invocations_suppressed": true,
			})
		}
	}
	if len(action.Invocations) == 0 {
		if action.ActionType != "no_op" {
			return a.failAutonomyAction(ctx, action.ID, "capability_calls_empty")
		}
		return a.settleFrozenToolAction(ctx, action, nil)
	}

	targetKind, targetRef := "", ""
	if action.ConversationID != "" {
		targetKind, targetRef = "conversation", action.ConversationID
	}
	for _, index := range actionExecutionOrder(action.Invocations, action.ActionType) {
		invocation := action.Invocations[index]
		if prior, found := capabilityResultForCall(action.Results, invocation.CallID); found && toolActionResultIsFinal(prior) {
			targetKind, targetRef = actionOutputTarget(prior, targetKind, targetRef)
			continue
		}
		arguments := invocation.Arguments
		if len(arguments) == 0 {
			arguments = json.RawMessage(`{}`)
		}
		operationID := strings.TrimSpace(invocation.Metadata.OperationID)
		if operationID == "" {
			operationID = "workflow_action:" + action.ID + ":" + firstString(invocation.CallID, fmt.Sprint(index))
		}
		surface := invocation.Metadata.Surface
		if surface == "" {
			surface = CapabilitySurfaceAutonomy
		}
		receipt, executeErr := a.ExecuteTool(ctx, ToolExecutionRequest{
			CapabilityName: invocation.CapabilityName, OperationID: operationID,
			AuthorizationActorID: action.AuthorizationActorID, FluctlightID: action.FluctlightID,
			ConversationID: action.ConversationID, TargetKind: targetKind, TargetRef: targetRef,
			EvidenceID: action.SourceFactID, Surface: surface, Arguments: arguments,
		})
		result := receipt.Result
		if result.CallID == "" {
			result = failedCapabilityResultDetail(invocation, "tool_execution_failed", true, errorText(executeErr))
		}
		result.CallID = invocation.CallID
		result.CapabilityName = invocation.CapabilityName
		if result.ProviderRequestID == "" {
			result.ProviderRequestID = invocation.ProviderRequestID
		}
		action.Results = replaceCapabilityResult(action.Results, result)
		targetKind, targetRef = actionOutputTarget(result, targetKind, targetRef)
		if executeErr != nil && result.Retryable {
			if persistErr := a.persistFrozenToolActionTrace(ctx, action); persistErr != nil {
				return nil, persistErr
			}
			return nil, executeErr
		}
	}
	if err := a.persistFrozenToolActionTrace(ctx, action); err != nil {
		return nil, err
	}
	if code, failed := requiredToolActionFailure(action.Results, action.Invocations, a.capabilityRegistry()); failed {
		return a.failAutonomyAction(ctx, action.ID, code)
	}
	return a.settleFrozenToolAction(ctx, action, actionSettlementOutput(action.Results))
}

func materializeActionPublicationInvocation(action frozenToolAction) ([]CapabilityInvocation, error) {
	invocations := append([]CapabilityInvocation(nil), action.Invocations...)
	requiredName := ""
	switch action.ActionType {
	case "proactive_message":
		requiredName = conversationReplyCapabilityName
	case "moment":
		requiredName = "moment.publish"
	default:
		return invocations, nil
	}
	for _, invocation := range invocations {
		if invocation.CapabilityName == requiredName {
			return invocations, nil
		}
	}
	text := strings.TrimSpace(stringValue(action.Payload["text"]))
	if text == "" {
		return nil, errors.New("action output text is missing")
	}
	arguments, err := json.Marshal(map[string]any{"text": text})
	if err != nil {
		return nil, err
	}
	return append([]CapabilityInvocation{{
		CallID: "action_output_" + stableDigest(action.ID)[:24], CapabilityName: requiredName,
		SchemaVersion: CapabilityInvocationSchemaVersion, Arguments: arguments,
		SourceFactID: action.SourceFactID, ActionID: action.ID,
		Metadata: InvocationMetadata{
			OperationID:  "workflow_action:" + action.ID + ":output",
			FluctlightID: action.FluctlightID, ConversationID: action.ConversationID,
			Surface: CapabilitySurfaceAutonomy, Source: "workflow_command",
		},
	}}, invocations...), nil
}

func actionExecutionOrder(invocations []CapabilityInvocation, actionType string) []int {
	order := make([]int, 0, len(invocations))
	publication := ""
	switch actionType {
	case "proactive_message":
		publication = conversationReplyCapabilityName
	case "moment":
		publication = "moment.publish"
	}
	if publication == "" {
		for _, invocation := range invocations {
			if invocation.CapabilityName == conversationReplyCapabilityName || invocation.CapabilityName == "moment.publish" {
				publication = invocation.CapabilityName
				break
			}
		}
	}
	if publication != "" {
		for index, invocation := range invocations {
			if invocation.CapabilityName == publication {
				order = append(order, index)
			}
		}
	}
	for index, invocation := range invocations {
		if invocation.CapabilityName != publication {
			order = append(order, index)
		}
	}
	return order
}

func actionPublicationText(invocations []CapabilityInvocation, capabilityName string) string {
	for _, invocation := range invocations {
		if invocation.CapabilityName != capabilityName {
			continue
		}
		var arguments map[string]any
		if json.Unmarshal(invocation.Arguments, &arguments) == nil {
			return strings.TrimSpace(stringValue(arguments["text"]))
		}
	}
	return ""
}

func toolActionResultIsFinal(result CapabilityResult) bool {
	if result.Status == "completed" || result.Status == "accepted" {
		return true
	}
	return (result.Status == "failed" || result.Status == "rejected") && !result.Retryable
}

func actionOutputTarget(result CapabilityResult, fallbackKind, fallbackRef string) (string, string) {
	output := mapValue(result.Output)
	kind, ref := strings.TrimSpace(stringValue(output["target_kind"])), strings.TrimSpace(stringValue(output["target_ref"]))
	if kind == "" || ref == "" {
		return fallbackKind, fallbackRef
	}
	return kind, ref
}

func actionSettlementOutput(results []CapabilityResult) map[string]any {
	output := map[string]any{}
	for _, result := range results {
		values := mapValue(result.Output)
		switch result.CapabilityName {
		case conversationReplyCapabilityName:
			output["delivery_status"] = "delivered"
			output["message_id"] = values["target_ref"]
		case "moment.publish":
			output["delivery_status"] = "published"
			output["moment_id"] = values["target_ref"]
		case "media.image.generate":
			output["media_intent_id"] = values["media_intent_id"]
		}
	}
	return output
}

func requiredToolActionFailure(results []CapabilityResult, invocations []CapabilityInvocation, registry *CapabilityRegistry) (string, bool) {
	if registry == nil {
		return "capability_not_found", true
	}
	for _, invocation := range invocations {
		definition, ok := registry.Definition(invocation.CapabilityName)
		if !ok {
			return "capability_not_found", true
		}
		if definition.FailurePolicy != FailurePolicyRequiredForVisibleClaim {
			continue
		}
		result, found := capabilityResultForCall(results, invocation.CallID)
		if !found {
			return "capability_result_missing", true
		}
		if result.Status != "completed" && result.Status != "accepted" {
			return firstString(result.ErrorCode, "capability_execution_failed"), true
		}
	}
	return "", false
}

func (a *App) persistFrozenToolActionTrace(ctx context.Context, action frozenToolAction) error {
	command, err := a.DB.Pool().Exec(ctx, `UPDATE public.autonomy_actions SET payload=jsonb_set(jsonb_set(payload,'{capability_invocations}',$2::jsonb,true),'{capability_results}',$3::jsonb,true) WHERE id=$1 AND status='frozen'`, action.ID, jsonBytes(action.Invocations), jsonBytes(action.Results))
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}

func (a *App) settleFrozenToolAction(ctx context.Context, action frozenToolAction, extra map[string]any) (map[string]any, error) {
	result := map[string]any{
		"action_id": action.ID, "action_type": action.ActionType,
		"status": "completed", "action_status": "completed",
		"capability_results": action.Results,
	}
	for key, value := range extra {
		result[key] = value
	}
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		command, err := tx.Exec(ctx, `UPDATE public.autonomy_actions SET payload=jsonb_set(jsonb_set(payload,'{capability_invocations}',$2::jsonb,true),'{capability_results}',$3::jsonb,true),status='completed',settled_at=now(),error_code=NULL WHERE id=$1 AND status='frozen'`, action.ID, jsonBytes(action.Invocations), jsonBytes(action.Results))
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return ErrConflict
		}
		if err := a.settleWakeUpActionTx(ctx, tx, action.ID, action.FluctlightID, result); err != nil {
			return err
		}
		return appendOutboxTx(ctx, tx, "autonomy.action.completed", "autonomy_action", action.ID, action.FluctlightID, action.ID, action.CorrelationID, "autonomy-outbox:"+action.ID, map[string]any{
			"action_type": action.ActionType, "status": "completed", "aggregate_sequence": 1,
		})
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func errorText(err error) string {
	if err == nil {
		return "tool execution failed"
	}
	return err.Error()
}

// FailAutonomyAction is the workflow-owned terminal failure boundary used when
// an outer durable intent exhausts its retry budget before the action worker
// can settle the frozen action itself.
func (a *App) FailAutonomyAction(ctx context.Context, actionID, code string) (map[string]any, error) {
	return a.failAutonomyAction(ctx, actionID, code)
}

func (a *App) failAutonomyAction(ctx context.Context, actionID, code string) (map[string]any, error) {
	result := map[string]any{"action_id": actionID, "status": "failed", "action_status": "failed", "error_code": code}
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var fluctlightID, actionType, status string
		var rawPayload, rawPolicySnapshot []byte
		if err := tx.QueryRow(ctx, `SELECT fluctlight_id,action_type,status,payload,policy_snapshot FROM public.autonomy_actions WHERE id=$1 FOR UPDATE`, actionID).Scan(&fluctlightID, &actionType, &status, &rawPayload, &rawPolicySnapshot); err != nil {
			return err
		}
		if status != "frozen" && status != "running" {
			return ErrConflict
		}
		payload := decodeObject(rawPayload)
		rootCorrelationID := firstString(payload["correlation_id"], "action-result:"+actionID)
		causality, err := frozenDecisionCausality(payload)
		if err != nil {
			return err
		}
		settlement := cloneMap(result)
		if policySnapshot := decodeObject(rawPolicySnapshot); len(policySnapshot) > 0 {
			settlement["policy_snapshot"] = policySnapshot
		}
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
		factPayload["correlation_id"] = rootCorrelationID
		for key, value := range causality {
			factPayload[key] = value
		}
		factID, err := appendProcessedCognitionFactTx(ctx, tx, fluctlightID, "autonomy.result", factPayload, "action-result:"+actionID)
		if err != nil {
			return err
		}
		if err := insertReflectionIntentTx(ctx, tx,
			"reflection_intent:result:"+actionID,
			"reflection:result:"+actionID,
			map[string]any{"fluctlight_id": fluctlightID, "source_fact_id": factID, "action_id": actionID, "correlation_id": rootCorrelationID, "causation_id": factID},
		); err != nil {
			return err
		}
		return appendOutboxTx(ctx, tx, "autonomy.result.recorded", "fluctlight", fluctlightID, fluctlightID, actionID, rootCorrelationID, "action-result:"+actionID, factPayload)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (a *App) settleWakeUpActionTx(ctx context.Context, tx pgx.Tx, actionID, fluctlightID string, result map[string]any) error {
	var actionPayload []byte
	var policySnapshotRaw []byte
	var actionType string
	if err := tx.QueryRow(ctx, `SELECT action_type,payload,policy_snapshot FROM public.autonomy_actions WHERE id=$1`, actionID).Scan(&actionType, &actionPayload, &policySnapshotRaw); err != nil {
		return err
	}
	payload := decodeObject(actionPayload)
	causality, err := frozenDecisionCausality(payload)
	if err != nil {
		return err
	}
	settledResult := cloneMap(result)
	if policySnapshot := decodeObject(policySnapshotRaw); len(policySnapshot) > 0 {
		settledResult["policy_snapshot"] = policySnapshot
	}
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
	sourceFactID := firstString(payload["source_fact_id"], actionID)
	rootCorrelationID := firstString(payload["correlation_id"], "action-result:"+actionID)
	outcomes, err := buildActionOutcomes(actionID, fluctlightID, sourceFactID, actionType, results, settledResult, a.capabilityRegistry())
	if err != nil {
		return err
	}
	if err := persistActionOutcomesTx(ctx, tx, outcomes); err != nil {
		return err
	}
	factPayload := map[string]any{"action_id": actionID, "result": settledResult, "outcomes": outcomes}
	factPayload["correlation_id"] = rootCorrelationID
	for key, value := range causality {
		factPayload[key] = value
	}
	factID, err := appendProcessedCognitionFactTx(ctx, tx, fluctlightID, "autonomy.result", factPayload, "action-result:"+actionID)
	if err != nil {
		return err
	}
	return insertReflectionIntentTx(ctx, tx,
		"reflection_intent:result:"+actionID,
		"reflection:result:"+actionID,
		map[string]any{"fluctlight_id": fluctlightID, "source_fact_id": factID, "action_id": actionID, "correlation_id": rootCorrelationID, "causation_id": factID},
	)
}

func (a *App) ProcessReflection(ctx context.Context, fluctlightID, correlationID string) (map[string]any, error) {
	// Reflection is the evidence-windowed learning pass over processed
	// cognition facts. A wake-up may create the fact that feeds this window,
	// but reflection never substitutes for the periodic wake-up trigger.
	if fluctlightID == "" {
		return nil, fmt.Errorf("reflection_fluctlight_id_required")
	}
	// Do not claim or advance a reflection window after cognition has taken the
	// lifecycle slot. This check deliberately precedes all reads and the window
	// lease so a cancelled activity is a no-op even when its Temporal cancel is
	// delivered a few milliseconds late.
	if a.lifecycleCancellationRequested(ctx, providerCancellationMarker(ctx)) {
		return map[string]any{"fluctlight_id": fluctlightID, "correlation_id": correlationID, "status": "cancelled", "reason": "superseded_by_cognition"}, nil
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
	if a.lifecycleCancellationRequested(ctx, providerCancellationMarker(ctx)) {
		return map[string]any{"fluctlight_id": fluctlightID, "correlation_id": correlationID, "status": "cancelled", "reason": "superseded_by_cognition"}, nil
	}
	ctx = WithProviderExecutionGuard(ctx, a.providerGuardForFluctlight(fluctlightID))
	var watermark, stateRevision int
	if err := a.DB.Pool().QueryRow(ctx, `SELECT watermark,state_revision FROM public.cognition_reflection_windows WHERE fluctlight_id=$1`, fluctlightID).Scan(&watermark, &stateRevision); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if err := a.DB.Pool().QueryRow(ctx, `SELECT revision FROM public.fluctlight_inner_states WHERE fluctlight_id=$1`, fluctlightID).Scan(&stateRevision); err != nil {
			return nil, err
		}
	}
	var currentStateRevision int
	if err := a.DB.Pool().QueryRow(ctx, `SELECT revision FROM public.fluctlight_inner_states WHERE fluctlight_id=$1`, fluctlightID).Scan(&currentStateRevision); err != nil {
		return nil, err
	} else if currentStateRevision > stateRevision {
		stateRevision = currentStateRevision
	}
	ctx, err = a.claimReflectionWindow(ctx, fluctlightID, watermark, stateRevision, correlationID)
	if err != nil {
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
	if err := rows.Err(); err != nil {
		rows.Close()
		_ = a.setReflectionWindowIdle(ctx, fluctlightID)
		return nil, err
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
		if err := appraisalRows.Err(); err != nil {
			appraisalRows.Close()
			_ = a.setReflectionWindowIdle(ctx, fluctlightID)
			return nil, err
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
		return map[string]any{"fluctlight_id": fluctlightID, "correlation_id": correlationID, "status": "no_op", "reason": "no_evidence", "watermark": watermark}, nil
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
	ctx = WithProviderCorrelation(ctx, correlationID)
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

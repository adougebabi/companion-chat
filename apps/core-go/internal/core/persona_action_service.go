package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const personaActionPlanVersion = "fluctlight.persona-action-plan.v1"

type preparedPersonaAction struct {
	SchemaVersion  string                   `json:"schema_version"`
	CapabilityName string                   `json:"capability_name"`
	OperationID    string                   `json:"operation_id"`
	FluctlightID   string                   `json:"fluctlight_id"`
	ConversationID string                   `json:"conversation_id,omitempty"`
	EvidenceID     string                   `json:"evidence_id"`
	OccurredAt     time.Time                `json:"occurred_at"`
	Decision       string                   `json:"decision"`
	Arguments      map[string]any           `json:"arguments"`
	RequestDigest  string                   `json:"request_digest"`
	SwitchPlan     *personalityDecisionPlan `json:"switch_plan,omitempty"`
	RejectionCode  string                   `json:"rejection_code,omitempty"`
}

type personaActionBusinessService struct{ app *App }

func newPersonaActionService(app *App) personaActionService {
	if app == nil {
		return nil
	}
	return &personaActionBusinessService{app: app}
}

func (service *personaActionBusinessService) preparePersonaAction(ctx context.Context, invocation CapabilityInvocation) (CapabilityInvocation, error) {
	if service == nil || service.app == nil || service.app.DB == nil || service.app.DB.Pool() == nil {
		return invocation, errors.New("persona_action_database_unavailable")
	}
	args, err := capabilityExecutionArguments(invocation, personaActionCapabilityDefinition(invocation.CapabilityName))
	if err != nil {
		return invocation, err
	}
	decision := strings.TrimSpace(stringValue(args["decision"]))
	operationID := strings.TrimSpace(capabilityOperationID(invocation))
	if decision == "" {
		return invocation, errors.New("persona_action_decision_required")
	}
	if operationID == "" || len([]rune(operationID)) > 256 || strings.TrimSpace(invocation.Metadata.FluctlightID) == "" || strings.TrimSpace(invocation.SourceFactID) == "" {
		return invocation, errors.New("persona_action_source_invalid")
	}
	plan := preparedPersonaAction{
		SchemaVersion: personaActionPlanVersion, CapabilityName: invocation.CapabilityName,
		OperationID: operationID, FluctlightID: strings.TrimSpace(invocation.Metadata.FluctlightID),
		ConversationID: strings.TrimSpace(invocation.Metadata.ConversationID), EvidenceID: strings.TrimSpace(invocation.SourceFactID),
		OccurredAt: service.app.now().UTC(), Decision: decision, Arguments: cloneMap(args),
	}
	plan.RequestDigest = personaActionRequestDigest(plan)
	switch invocation.CapabilityName {
	case personaSwitchCapabilityName:
		decisionInput := map[string]any{
			"decision": decision, "from_profile_id": strings.TrimSpace(stringValue(args["source_profile_id"])),
			"target_profile_id": strings.TrimSpace(stringValue(args["target_profile_id"])),
			"trigger_id":        strings.TrimSpace(stringValue(args["trigger_id"])), "reason": strings.TrimSpace(stringValue(args["reason"])),
		}
		switchPlan, prepareErr := service.app.preparePersonalityDecision(ctx, plan.FluctlightID, decisionInput)
		if prepareErr != nil {
			if code, ok := personaActionBusinessRejection(prepareErr); ok {
				plan.RejectionCode = code
			} else {
				return invocation, prepareErr
			}
		} else {
			plan.SwitchPlan = switchPlan
		}
	case personaTakeoverCapabilityName:
		if rejection, validateErr := validatePersonaTakeoverAction(ctx, service.app.DB.Pool(), plan.FluctlightID, args); validateErr != nil {
			return invocation, validateErr
		} else {
			plan.RejectionCode = rejection
		}
	default:
		return invocation, errors.New("persona_action_capability_invalid")
	}
	return withCapabilityPreparedData(invocation, "persona_action_plan", plan)
}

func personaActionRequestDigest(plan preparedPersonaAction) string {
	return stableDigest(jsonString(map[string]any{
		"schema_version": plan.SchemaVersion, "capability_name": plan.CapabilityName,
		"operation_id": plan.OperationID, "fluctlight_id": plan.FluctlightID,
		"conversation_id": plan.ConversationID, "evidence_id": plan.EvidenceID,
		"decision": plan.Decision, "arguments": plan.Arguments,
	}))
}

func personaActionBusinessRejection(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	code := strings.TrimSpace(err.Error())
	switch code {
	case "personality_decision_invalid", "personality_source_profile_required", "personality_target_profile_required",
		"personality_source_profile_stale", "personality_switch_cooldown", "personality_target_profile_not_found",
		"personality_trigger_not_found", "personality_trigger_required":
		return code, true
	default:
		return "", false
	}
}

type personaActionRowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func validatePersonaTakeoverAction(ctx context.Context, querier personaActionRowQuerier, fluctlightID string, args map[string]any) (string, error) {
	decision := strings.TrimSpace(stringValue(args["decision"]))
	validDecision := map[string]struct{}{
		takeoverDecisionNotApplicable: {}, takeoverDecisionSkipped: {}, takeoverDecisionJudgeKeptA: {},
		takeoverDecisionJudgeTakeoverBPending: {}, takeoverDecisionJudgeDegraded: {}, takeoverDecisionTakeoverB: {},
	}
	if _, ok := validDecision[decision]; !ok {
		return "persona_takeover_decision_invalid", nil
	}
	var rawPersona []byte
	if err := querier.QueryRow(ctx, `SELECT core_persona FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&rawPersona); err != nil {
		return "", err
	}
	corePersona := decodeObject(rawPersona)
	system := mapValue(corePersona["personality_system"])
	initialProfile := initialPersonalityProfileID(corePersona)
	activeProfile := initialProfile
	var cooldownUntil *time.Time
	if err := querier.QueryRow(ctx, `SELECT active_profile_id,cooldown_until FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1`, fluctlightID).Scan(&activeProfile, &cooldownUntil); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	if strings.TrimSpace(activeProfile) == "" {
		activeProfile = firstString(initialProfile, "default")
	}
	ruleID := strings.TrimSpace(stringValue(args["rule_id"]))
	targetID := strings.TrimSpace(stringValue(args["target_profile_id"]))
	sourceID := strings.TrimSpace(stringValue(args["source_profile_id"]))
	requiresRule := decision != takeoverDecisionNotApplicable && decision != takeoverDecisionSkipped
	if requiresRule && ruleID == "" {
		return "persona_takeover_rule_required", nil
	}
	if sourceID != "" && sourceID != activeProfile {
		return "persona_takeover_source_profile_stale", nil
	}
	if !requiresRule && ruleID == "" && targetID == "" {
		return "", nil
	}
	var matched map[string]any
	for _, raw := range arrayValue(system["takeover_rules"]) {
		candidate := mapValue(raw)
		if strings.TrimSpace(stringValue(candidate["id"])) == ruleID {
			matched = candidate
			break
		}
	}
	if len(matched) == 0 {
		return "persona_takeover_rule_not_found", nil
	}
	if stringValue(matched["kind"]) != string(switchRuleTurnTakeover) || stringValue(matched["version"]) != personaTakeoverRuleVersion {
		return "persona_takeover_rule_invalid", nil
	}
	if enabled, supplied := matched["enabled"].(bool); supplied && !enabled {
		return "persona_takeover_rule_not_applicable", nil
	}
	declaredTarget := strings.TrimSpace(stringValue(matched["target_profile_id"]))
	declaredSource := strings.TrimSpace(stringValue(matched["source_profile_id"]))
	if targetID == "" || targetID != declaredTarget {
		return "persona_takeover_target_mismatch", nil
	}
	if declaredSource != "" && declaredSource != activeProfile {
		return "persona_takeover_rule_not_applicable", nil
	}
	if sourceID != "" && declaredSource != "" && sourceID != declaredSource {
		return "persona_takeover_source_mismatch", nil
	}
	if _, ok := personalityProfileIDs(corePersona)[targetID]; !ok || targetID == activeProfile {
		return "persona_takeover_target_not_available", nil
	}
	if cooldownUntil != nil && time.Now().UTC().Before(cooldownUntil.UTC()) {
		return "persona_takeover_cooldown", nil
	}
	return "", nil
}

func (service *personaActionBusinessService) applyPersonaAction(_ context.Context, invocation CapabilityInvocation) (CapabilityResult, error) {
	return failedCapabilityResult(invocation, "caller_transaction_required", false), newCapabilityError("caller_transaction_required", false, ErrConflict)
}

func (service *personaActionBusinessService) applyPersonaActionTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation) (CapabilityResult, error) {
	if service == nil || service.app == nil || tx == nil {
		return failedCapabilityResult(invocation, "persona_action_unavailable", true), errors.New("persona action service unavailable")
	}
	rawPlan, found, err := capabilityPreparedData(invocation, "persona_action_plan")
	if err != nil || !found {
		return failedCapabilityResult(invocation, "persona_action_plan_invalid", false), errors.New("persona_action_plan_invalid")
	}
	var plan preparedPersonaAction
	if jsonUnmarshal(jsonBytes(rawPlan), &plan) != nil || validatePreparedPersonaAction(invocation, plan) != nil {
		return failedCapabilityResult(invocation, "persona_action_plan_invalid", false), errors.New("persona_action_plan_invalid")
	}
	if replay, replayed, replayErr := readPersonaActionReplayTx(ctx, tx, invocation, plan); replayErr != nil {
		return failedCapabilityResult(invocation, "persona_action_idempotency_conflict", false), replayErr
	} else if replayed {
		return replay, nil
	}
	if plan.CapabilityName == personaTakeoverCapabilityName && plan.RejectionCode == "" {
		var validationErr error
		plan.RejectionCode, validationErr = validatePersonaTakeoverAction(ctx, tx, plan.FluctlightID, plan.Arguments)
		if validationErr != nil {
			return failedCapabilityResult(invocation, "persona_takeover_validation_failed", true), validationErr
		}
	}
	output := map[string]any{
		"action": plan.CapabilityName, "decision": plan.Decision, "replayed": false,
		"disposition": "applied", "reason_code": "committed",
	}
	status := "completed"
	errorCode := ""
	if plan.RejectionCode != "" {
		status, errorCode = "rejected", plan.RejectionCode
		output["disposition"], output["reason_code"] = "rejected", plan.RejectionCode
	} else if plan.CapabilityName == personaSwitchCapabilityName {
		result, applyErr := service.app.applyPersonalityDecisionPlanTx(ctx, tx, plan.FluctlightID, plan.SwitchPlan)
		if applyErr != nil {
			if code, ok := personaActionBusinessRejection(applyErr); ok {
				status, errorCode = "rejected", code
				output["disposition"], output["reason_code"] = "rejected", code
			} else if applyErr.Error() == "personality_runtime_revision_conflict" {
				status, errorCode = "rejected", applyErr.Error()
				output["disposition"], output["reason_code"] = "rejected", applyErr.Error()
			} else {
				return failedCapabilityResult(invocation, "persona_switch_commit_failed", true), applyErr
			}
		} else {
			for key, value := range result {
				output[key] = value
			}
			if plan.SwitchPlan == nil || plan.SwitchPlan.TargetProfile == plan.SwitchPlan.CurrentProfile {
				output["disposition"], output["reason_code"] = "no_change", "profile_kept"
			}
		}
	} else {
		output["target_profile_id"] = strings.TrimSpace(stringValue(plan.Arguments["target_profile_id"]))
		output["source_profile_id"] = strings.TrimSpace(stringValue(plan.Arguments["source_profile_id"]))
		output["rule_id"] = strings.TrimSpace(stringValue(plan.Arguments["rule_id"]))
		if plan.Decision == takeoverDecisionNotApplicable || plan.Decision == takeoverDecisionSkipped || plan.Decision == takeoverDecisionJudgeKeptA || plan.Decision == takeoverDecisionJudgeDegraded {
			output["disposition"], output["reason_code"] = "no_change", plan.Decision
		}
	}
	if status == "completed" && stringValue(output["disposition"]) == "applied" {
		targetID := stringValue(plan.Arguments["target_profile_id"])
		if targetID != "" {
			var rawPersona []byte
			if err := tx.QueryRow(ctx, `SELECT core_persona FROM public.fluctlights WHERE id=$1`, plan.FluctlightID).Scan(&rawPersona); err != nil {
				return failedCapabilityResult(invocation, "persona_context_read_failed", true), err
			}
			for _, raw := range arrayValue(mapValue(decodeObject(rawPersona)["personality_system"])["profiles"]) {
				profile := mapValue(raw)
				if stringValue(profile["id"]) == targetID {
					persona := corePersonaData(decodeObject(rawPersona))
					baseline := personaEvolutionBaseline(plan.FluctlightID, mapValue(persona["personality"]), mapValue(persona["behavioral_policy"]), mapValue(persona["personality_system"]), map[string]any{"active_profile_id": targetID})
					state, loadErr := loadPersonaEvolutionState(ctx, tx, baseline)
					if loadErr != nil {
						return failedCapabilityResult(invocation, "persona_overlay_read_failed", true), loadErr
					}
					effective, composeErr := ComposeEffectivePersona(state)
					if composeErr != nil {
						return failedCapabilityResult(invocation, "persona_overlay_invalid", true), composeErr
					}
					working := cloneMap(profile)
					working["personality"], working["behavioral_policy"] = effective.Personality, effective.BehaviorPolicy
					output["working_persona"] = working
					break
				}
			}
		}
	}
	auditID := "persona_action_audit_" + stableDigest(personaActionAuditKey(plan))
	output["audit_id"] = auditID
	result := CapabilityResult{
		CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: status, ErrorCode: errorCode,
		Output: output, Retryable: false, ProviderRequestID: invocation.ProviderRequestID,
		CorrelationID: "persona-action:" + plan.OperationID,
	}
	if err := persistPersonaActionAuditTx(ctx, tx, invocation, plan, result, auditID); err != nil {
		return failedCapabilityResult(invocation, "persona_action_audit_failed", true), err
	}
	return result, nil
}

func validatePreparedPersonaAction(invocation CapabilityInvocation, plan preparedPersonaAction) error {
	if plan.SchemaVersion != personaActionPlanVersion || plan.CapabilityName != invocation.CapabilityName || plan.FluctlightID != invocation.Metadata.FluctlightID ||
		plan.OperationID != capabilityOperationID(invocation) || plan.EvidenceID != invocation.SourceFactID || strings.TrimSpace(plan.Decision) == "" || plan.OccurredAt.IsZero() ||
		plan.RequestDigest == "" || plan.RequestDigest != personaActionRequestDigest(plan) {
		return errors.New("persona_action_plan_invalid")
	}
	if plan.CapabilityName == personaSwitchCapabilityName && plan.RejectionCode == "" && plan.SwitchPlan == nil {
		return errors.New("persona_action_plan_invalid")
	}
	return nil
}

func personaActionAuditKey(plan preparedPersonaAction) string {
	return strings.Join([]string{"persona-action", plan.FluctlightID, plan.CapabilityName, plan.OperationID}, ":")
}

func readPersonaActionReplayTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, plan preparedPersonaAction) (CapabilityResult, bool, error) {
	var payloadRaw []byte
	err := tx.QueryRow(ctx, `SELECT payload FROM public.platform_outbox_events WHERE idempotency_key=$1 FOR UPDATE`, personaActionAuditKey(plan)).Scan(&payloadRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return CapabilityResult{}, false, nil
	}
	if err != nil {
		return CapabilityResult{}, false, err
	}
	payload := decodeObject(payloadRaw)
	if strings.TrimSpace(stringValue(payload["request_digest"])) != plan.RequestDigest {
		return CapabilityResult{}, false, newCapabilityError("persona_action_idempotency_conflict", false, ErrConflict)
	}
	stored := mapValue(payload["result"])
	output := cloneMap(mapValue(stored["output"]))
	output["replayed"] = true
	return CapabilityResult{
		CallID: invocation.CallID, CapabilityName: invocation.CapabilityName,
		Status: strings.TrimSpace(stringValue(stored["status"])), ErrorCode: strings.TrimSpace(stringValue(stored["error_code"])),
		Output: output, Retryable: false, ProviderRequestID: invocation.ProviderRequestID,
		CorrelationID: "persona-action:" + plan.OperationID,
	}, true, nil
}

func persistPersonaActionAuditTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, plan preparedPersonaAction, result CapabilityResult, auditID string) error {
	payload := map[string]any{
		"schema_version": personaActionPlanVersion, "capability_name": plan.CapabilityName,
		"operation_id": plan.OperationID, "request_digest": plan.RequestDigest,
		"evidence_id": plan.EvidenceID, "conversation_id": plan.ConversationID,
		"occurred_at":        plan.OccurredAt.UTC().Format(time.RFC3339Nano),
		"result":             map[string]any{"status": result.Status, "error_code": result.ErrorCode, "output": result.Output},
		"aggregate_sequence": 1,
	}
	command, err := tx.Exec(ctx, `INSERT INTO public.platform_outbox_events(id,kind,aggregate_type,aggregate_id,fluctlight_id,causation_id,correlation_id,idempotency_key,payload,attempt_policy,occurred_at) VALUES($1,$2,'persona_action',$3,$4,$5,$6,$7,$8,'{}',$9) ON CONFLICT(idempotency_key) DO NOTHING`,
		"outbox_"+stableDigest(personaActionAuditKey(plan)), plan.CapabilityName+".committed", auditID, plan.FluctlightID,
		plan.EvidenceID, firstString(invocation.Metadata.CorrelationID, "persona-action:"+plan.OperationID), personaActionAuditKey(plan), jsonBytes(payload), plan.OccurredAt.UTC())
	if err != nil {
		return err
	}
	if command.RowsAffected() == 1 {
		return nil
	}
	replay, found, replayErr := readPersonaActionReplayTx(ctx, tx, invocation, plan)
	if replayErr != nil {
		return replayErr
	}
	if !found || replay.Status != result.Status || jsonString(replay.Output) != jsonString(withPersonaReplayFlag(mapValue(result.Output), true)) {
		return fmt.Errorf("persona_action_idempotency_conflict: %w", ErrConflict)
	}
	return nil
}

func withPersonaReplayFlag(output map[string]any, replayed bool) map[string]any {
	copyOutput := cloneMap(output)
	copyOutput["replayed"] = replayed
	return copyOutput
}

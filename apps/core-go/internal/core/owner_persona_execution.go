package core

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const ownerPersonaSwitchIntent = "persona.owner_switch"

// Only declared persistent rules are eligible for assessment. Confirmation is
// evidence of the Owner's request, never evidence that an arbitrary condition
// (time, mood, safety, etc.) has become true.
func ownerPersonaSwitchRules(persona map[string]any, current, target string) []personaSwitchRule {
	result := []personaSwitchRule{}
	for _, rule := range normalizePersonaSwitchRules(persona, map[string]any{"active_profile_id": current}, current).Rules {
		if !rule.Enabled || (rule.Kind != switchRulePersistentSemantic && rule.Kind != switchRulePersistentDeterministic) || (rule.Source != personaSwitchSourceSwitching && rule.Source != personaSwitchSourceSwitchingList) {
			continue
		}
		if enabled, exists := rule.Raw["enabled"]; exists && enabled != true {
			continue
		}
		if rule.SourceProfileID != "" && rule.SourceProfileID != current {
			continue
		}
		if target != "" && rule.TargetProfileID != target {
			continue
		}
		result = append(result, rule)
	}
	return result
}

func (a *App) enqueueOwnerPersonaSwitchTx(ctx context.Context, tx pgx.Tx, owner, fid, current, target string, revision, runtimeRevision int, body map[string]any) (map[string]any, error) {
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT core_persona FROM public.fluctlights WHERE id=$1`, fid).Scan(&raw); err != nil {
		return nil, err
	}
	if len(ownerPersonaSwitchRules(decodeObject(raw), current, target)) == 0 {
		return nil, &SurfaceError{Status: 422, Code: "personality_trigger_not_found", Message: "No declared persistent rule admits assessment for this target"}
	}
	// Admission also checks cooldown, but does not claim a condition is satisfied.
	rule := ownerPersonaSwitchRules(decodeObject(raw), current, target)[0]
	if _, err := a.preparePersonalityDecisionQuery(ctx, tx, fid, map[string]any{"decision": "switch", "from_profile_id": current, "target_profile_id": target, "trigger_id": rule.RuleID}); err != nil {
		if code, ok := personaActionBusinessRejection(err); ok {
			return nil, &SurfaceError{Status: 409, Code: code, Message: "Persona switch precondition is not satisfied"}
		}
		return nil, err
	}
	operation := "owner-switch:" + stableDigest(owner+":"+fid+":"+stringValue(body["idempotency_key"]))
	id := "req_persona_" + stableDigest(operation)
	payload := map[string]any{"intent_id": id, "request_id": id, "fluctlight_id": fid, "owner_actor_id": owner, "source_profile_id": current, "target_profile_id": target, "expected_revision": revision, "expected_runtime_revision": runtimeRevision, "reason": body["reason"], "source_fact_id": id}
	if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'lifecycle',$3,$4) ON CONFLICT DO NOTHING`, id, operation, ownerPersonaSwitchIntent, jsonBytes(payload)); err != nil {
		return nil, err
	}
	if err := appendOutboxTx(ctx, tx, "owner.persona_switch.requested", "owner_request", id, fid, id, operation, "owner-persona-request:"+id, payload); err != nil {
		return nil, err
	}
	return map[string]any{"request_id": id, "status": "pending", "status_url": "/api/owner-requests/" + id, "source_fact_id": id, "disposition": "assessment_pending", "revision": revision, "runtime_revision": runtimeRevision, "active_profile_id": current, "previous_profile_id": current, "persona_action_audit_id": nil}, nil
}

// ProcessOwnerPersonaSwitchIntent is the Worker entry point. All Provider I/O
// is outside transactions; a late assessment must pass the original dual CAS
// under locks before the controlled persona writer is allowed to commit.
func (a *App) ProcessOwnerPersonaSwitchIntent(ctx context.Context, id string) (map[string]any, error) {
	var raw []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT payload FROM public.platform_workflow_intents WHERE intent_id=$1 AND intent_type=$2`, id, ownerPersonaSwitchIntent).Scan(&raw); err != nil {
		return nil, err
	}
	source := decodeObject(raw)
	owner, fid := stringValue(source["owner_actor_id"]), stringValue(source["fluctlight_id"])
	var requestRaw []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT payload FROM public.owner_surface_records WHERE id=$1 AND owner_actor_id=$2 AND fluctlight_id=$3 AND kind='owner_request'`, id, owner, fid).Scan(&requestRaw); err != nil {
		return nil, err
	}
	request := decodeObject(requestRaw)
	if request["status"] != "pending" {
		return mapValue(request["result"]), nil
	}
	projection, err := a.BuildContextProjectionFor(ctx, ContextProjectionRequest{AuthorizationActorID: owner, SpeakerActorID: owner, TargetActorID: owner, FluctlightID: fid, SourceFactID: id, TriggerSource: "owner_persona_request", CurrentUserText: jsonString(map[string]any{"event_type": "owner.persona_switch.requested", "request_id": id, "target_profile_id": source["target_profile_id"], "reason": source["reason"], "confirmed": true}), MemoryOperation: MemoryForNativeCognition, MemoryConversationMode: MemoryConversationGlobalOnly})
	if err != nil {
		return a.ownerPersonaAssessmentFailure(ctx, id, owner, fid, err)
	}
	// This source fact is an Owner command, not a conversation reply or a
	// capability grant. The existing bounded assessment remains tool-free.
	task, err := a.RunPersistentSwitchTask(ctx, PersistentSwitchTaskInput{InboxID: id, CurrentText: projection.CurrentUserText, ResponseIntent: "owner_persona_request: assess only the requested target against declared conditions; Owner confirmation alone does not satisfy other conditions; otherwise keep", Projection: projection})
	if err != nil {
		return a.ownerPersonaAssessmentFailure(ctx, id, owner, fid, err)
	}
	decision := mapValue(task.Completion.Structured["personality_decision"])
	if len(decision) == 0 {
		return a.ownerPersonaAssessmentFailure(ctx, id, owner, fid, errors.New("persistent_switch_assessment_missing"))
	}
	var result map[string]any
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var personaRaw []byte
		var foundation int
		var status string
		if err := tx.QueryRow(ctx, `SELECT core_persona,current_revision,status FROM public.fluctlights WHERE id=$1 AND created_by_actor_id=$2 FOR UPDATE`, fid, owner).Scan(&personaRaw, &foundation, &status); err != nil {
			return err
		}
		var runtime int
		var active string
		if err := tx.QueryRow(ctx, `SELECT active_profile_id,revision FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1 FOR UPDATE`, fid).Scan(&active, &runtime); errors.Is(err, pgx.ErrNoRows) {
			active = initialPersonalityProfileID(decodeObject(personaRaw))
		} else if err != nil {
			return err
		}
		var payloadRaw []byte
		if err := tx.QueryRow(ctx, `SELECT payload FROM public.owner_surface_records WHERE id=$1 AND owner_actor_id=$2 AND fluctlight_id=$3 FOR UPDATE`, id, owner, fid).Scan(&payloadRaw); err != nil {
			return err
		}
		request = decodeObject(payloadRaw)
		if request["status"] != "pending" {
			result = mapValue(request["result"])
			return nil
		}
		var intentStatus string
		if err := tx.QueryRow(ctx, `SELECT status FROM public.platform_workflow_intents WHERE intent_id=$1 AND intent_type=$2 FOR UPDATE`, id, ownerPersonaSwitchIntent).Scan(&intentStatus); err != nil {
			return err
		}
		code := ""
		switch {
		case intentStatus == "cancel_requested" || intentStatus == "cancelled" || intentStatus == "terminated" || intentStatus == "failed" || intentStatus == "dead_letter":
			code = "persona_switch_execution_cancelled"

		case status == "retired":
			code = "fluctlight_retired"
		case foundation != intValue(source["expected_revision"]):
			code = "foundation_revision_conflict"
		case runtime != intValue(source["expected_runtime_revision"]) || active != stringValue(source["source_profile_id"]):
			code = "personality_runtime_revision_conflict"
		case decision["decision"] == "keep":
			code = "persona_switch_condition_not_satisfied"
		case decision["decision"] != "switch":
			code = "personality_decision_invalid"
		case decision["from_profile_id"] != active || decision["target_profile_id"] != source["target_profile_id"]:
			code = "persona_switch_assessment_target_mismatch"
		}
		var matchedRule personaSwitchRule
		if code == "" {
			matched := false
			for _, rule := range ownerPersonaSwitchRules(decodeObject(personaRaw), active, stringValue(source["target_profile_id"])) {
				if persistentSwitchRuleIDMatches(stringValue(decision["trigger_id"]), strings.TrimPrefix(rule.RuleID, "switch:")) {
					matched = true
					matchedRule = rule
					break
				}
			}
			if !matched {
				code = "personality_trigger_not_found"
			}
		}
		if code != "" {
			result = map[string]any{"request_id": id, "status": "rejected", "error_code": code, "active_profile_id": active, "runtime_revision": runtime, "source_fact_id": id}
			return a.settleOwnerPersonaRequestTx(ctx, tx, id, owner, request, result)
		}
		plan, prepareErr := a.preparePersonalityDecisionQuery(ctx, tx, fid, decision)
		if prepareErr != nil {
			if code, ok := personaActionBusinessRejection(prepareErr); ok {
				result = map[string]any{"request_id": id, "status": "rejected", "error_code": code, "source_fact_id": id}
				return a.settleOwnerPersonaRequestTx(ctx, tx, id, owner, request, result)
			}
			return prepareErr
		}
		// Public runtime/history keep the Owner's reason. The private model
		// assessment text never becomes an Owner-facing switch reason.
		plan.Reason = stringValue(source["reason"])
		if seconds := matchedRule.CooldownSeconds; seconds > 0 && seconds <= 7*24*60*60 {
			until := a.now().UTC().Add(time.Duration(seconds) * time.Second)
			if plan.CooldownUntil == nil || until.After(*plan.CooldownUntil) {
				plan.CooldownUntil = &until
			}
		}
		operation := "owner-persona-execution:" + id
		args := map[string]any{"decision": "switch", "source_profile_id": active, "target_profile_id": plan.TargetProfile, "trigger_id": decision["trigger_id"], "reason": stringValue(source["reason"])}
		prepared := preparedPersonaAction{SchemaVersion: personaActionPlanVersion, CapabilityName: personaSwitchCapabilityName, OperationID: operation, FluctlightID: fid, EvidenceID: id, OccurredAt: a.now().UTC(), Decision: "switch", Arguments: args, SwitchPlan: plan}
		prepared.RequestDigest = personaActionRequestDigest(prepared)
		invocation := CapabilityInvocation{CallID: operation, CapabilityName: personaSwitchCapabilityName, SourceFactID: id, Arguments: jsonBytes(args), Metadata: InvocationMetadata{OperationID: operation, AuthorizationActorID: owner, FluctlightID: fid, CorrelationID: operation, Source: "owner_request_assessment"}}
		invocation, err = withCapabilityPreparedData(invocation, "persona_action_plan", prepared)
		if err != nil {
			return err
		}
		receipt, err := (&personaActionBusinessService{app: a}).applyPersonaActionTx(ctx, tx, invocation)
		if err != nil {
			return err
		}
		output := mapValue(receipt.Output)
		if output["disposition"] != "applied" {
			return errors.New("persona_owner_action_not_committed")
		}
		result = map[string]any{"request_id": id, "status": "completed", "disposition": "applied", "revision": foundation, "runtime_revision": plan.ResultingRevision, "active_profile_id": plan.TargetProfile, "previous_profile_id": active, "persona_action_audit_id": output["audit_id"], "source_fact_id": id}
		return a.settleOwnerPersonaRequestTx(ctx, tx, id, owner, request, result)
	})
	if err != nil {
		return a.ownerPersonaAssessmentFailure(ctx, id, owner, fid, err)
	}
	return result, nil
}

func (a *App) settleOwnerPersonaRequestTx(ctx context.Context, tx pgx.Tx, id, owner string, request, result map[string]any) error {
	request["status"], request["result"], request["decided_by"], request["updated_at"] = result["status"], result, "core_persona_assessment", a.now().UTC().Format(time.RFC3339Nano)
	request["revision"] = intValue(request["revision"]) + 1
	delete(request, "error_code")
	if audit := stringValue(result["persona_action_audit_id"]); audit != "" {
		request["result_refs"] = append(arrayValue(request["result_refs"]), audit)
	}

	// Only bounded public codes and committed runtime metadata cross this read
	// model. Model assessment/reasoning and Provider errors are never persisted here.
	_, err := tx.Exec(ctx, `UPDATE public.owner_surface_records SET status=$3,payload=$4,revision=revision+1,updated_at=now() WHERE id=$1 AND owner_actor_id=$2`, id, owner, result["status"], jsonBytes(request))
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE public.platform_workflow_intents SET status='completed',completed_at=COALESCE(completed_at,now()),last_error=NULL WHERE intent_id=$1 AND intent_type=$2`, id, ownerPersonaSwitchIntent)
	if err != nil {
		return err
	}
	return appendOwnerRequestNotificationTx(ctx, tx, stringValue(request["fluctlight_id"]), id, "persona_switch", stringValue(result["status"]), id)
}

func (a *App) ownerPersonaAssessmentFailure(ctx context.Context, id, owner, fid string, cause error) (map[string]any, error) {
	var result map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT payload FROM public.owner_surface_records WHERE id=$1 AND owner_actor_id=$2 AND fluctlight_id=$3 FOR UPDATE`, id, owner, fid).Scan(&raw); err != nil {
			return err
		}
		request := decodeObject(raw)
		if request["status"] != "pending" {
			result = mapValue(request["result"])
			return nil
		}
		attempts := intValue(request["assessment_attempts"]) + 1
		request["assessment_attempts"] = attempts
		if attempts >= 3 {
			result = map[string]any{"request_id": id, "status": "failed", "error_code": "persona_switch_assessment_failed", "retryable": true, "source_fact_id": id}
			return a.settleOwnerPersonaRequestTx(ctx, tx, id, owner, request, result)
		}
		request["error_code"] = "persona_switch_assessment_retrying"
		_, err := tx.Exec(ctx, `UPDATE public.owner_surface_records SET payload=$3,updated_at=now() WHERE id=$1 AND owner_actor_id=$2`, id, owner, jsonBytes(request))
		return err
	})
	if err != nil {
		return nil, err
	}
	if result != nil {
		return result, nil
	}
	// Bounded error keeps Temporal's retry mechanism; do not expose raw cause.
	_ = cause // Provider diagnostics own the detailed, redacted failure.
	a.RecordLifecycleDiagnosticBestEffort(ctx, LifecycleDiagnostic{Surface: "persona_owner_switch", Transition: LifecycleTransitionRetryScheduled, FluctlightID: fid, IntentID: id, CausationID: id, CorrelationID: "owner-persona-execution:" + id, Stage: "assessment", Status: "retry", ReasonCode: "persona_switch_assessment_retrying", ErrorCode: "persona_switch_assessment_retrying", Retryable: true})
	return nil, errors.New("persona_switch_assessment_retrying")
}

// FailOwnerPersonaSwitchIntent closes the Owner read model when Temporal
// exhausts retries or is cancelled before the Activity can settle it. It never
// undoes a committed persona action and never reopens a terminal request.
func (a *App) FailOwnerPersonaSwitchIntent(ctx context.Context, id string) (map[string]any, error) {
	var result map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var raw []byte
		var owner string
		err := tx.QueryRow(ctx, `SELECT owner_actor_id,payload FROM public.owner_surface_records WHERE id=$1 AND kind='owner_request' FOR UPDATE`, id).Scan(&owner, &raw)
		if err != nil {
			return err
		}
		request := decodeObject(raw)
		if request["kind"] != "persona_switch" {
			return errors.New("persona_request_scope_invalid")
		}
		if request["status"] != "pending" {
			result = mapValue(request["result"])
			return nil
		}
		result = map[string]any{"request_id": id, "status": "failed", "error_code": "persona_switch_execution_failed", "retryable": true, "source_fact_id": id}
		return a.settleOwnerPersonaRequestTx(ctx, tx, id, owner, request, result)
	})
	return result, err
}

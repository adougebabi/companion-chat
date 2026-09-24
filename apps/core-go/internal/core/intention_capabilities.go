package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	intentionInspectCapabilityName = "intention.inspect"
	intentionDecideCapabilityName  = "intention.decide"
)

type intentionService struct{ repository *PostgresRepository }
type intentionInspectCapability struct{ service *intentionService }
type intentionDecideCapability struct{ service *intentionService }

func intentionInspectDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: intentionInspectCapabilityName, Version: "v1", Type: CapabilityTypeQuery,
		Description:   "Read the current speaking profile's durable unfinished or specified intentions, their goals, status and linked activity. A plan is not a completed result.",
		Surfaces:      []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceNativeCognition},
		FailurePolicy: FailurePolicyOptionalInternal,
		InputSchema: objectSchema(map[string]any{
			"operation":      enumStringSchema("list", "detail"),
			"intention_id":   map[string]any{"type": "string", "maxLength": 128},
			"include_closed": map[string]any{"type": "boolean"},
		}, []string{"operation"}, false),
		OutputSchema: openObjectSchema(), SideEffectClass: "read_only", SuccessBoundary: "query_result_available",
		ConcurrencyClass: "parallel", SupportsRetry: true,
	}
}
func (c intentionInspectCapability) Definition() CapabilityDefinition {
	return intentionInspectDefinition()
}
func (c intentionInspectCapability) RequiredContext() []ContextSlot { return nil }
func (c intentionInspectCapability) Execute(ctx context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	if c.service == nil || c.service.repository == nil {
		return failedCapabilityResult(invocation, "intention_unavailable", true), errors.New("intention service unavailable")
	}
	args, err := capabilityExecutionArguments(invocation, intentionInspectDefinition())
	if err != nil {
		return failedCapabilityResult(invocation, "invalid_arguments", false), err
	}
	profileID, err := c.service.resolveProfile(ctx, c.service.repository.Pool(), invocation.Metadata.FluctlightID, invocation.Metadata.WorkingProfileID)
	if err != nil {
		return failedCapabilityResult(invocation, "intention_profile_unavailable", true), err
	}
	fluctlightID := invocation.Metadata.FluctlightID
	output := map[string]any{"operation": args["operation"], "profile_id": profileID}
	if stringValue(args["operation"]) == "detail" {
		id := strings.TrimSpace(stringValue(args["intention_id"]))
		if id == "" {
			return failedCapabilityResult(invocation, "intention_id_required", false), ErrInvalidArguments
		}
		var goal, action, expected, status string
		var revision int
		var expiration time.Time
		err := c.service.repository.Pool().QueryRow(ctx, `SELECT g.desired_outcome,i.action_intent,i.expected_outcome,i.status,i.revision,i.expiration FROM public.fluctlight_intentions i JOIN public.fluctlight_goals g ON g.id=i.goal_id AND g.fluctlight_id=i.fluctlight_id WHERE i.fluctlight_id=$1 AND i.id=$2 AND COALESCE(i.profile_id,'')=$3`, fluctlightID, id, profileID).Scan(&goal, &action, &expected, &status, &revision, &expiration)
		if errors.Is(err, pgx.ErrNoRows) {
			return failedCapabilityResult(invocation, "intention_not_found", false), ErrNotFound
		}
		if err != nil {
			return failedCapabilityResult(invocation, "intention_read_failed", true), err
		}
		output["intention"] = map[string]any{"id": id, "goal": goal, "action": action, "expected_outcome": expected, "status": status, "revision": revision, "expiration": expiration.UTC().Format(time.RFC3339Nano)}
		rows, err := c.service.repository.Pool().Query(ctx, `SELECT id,kind,status,scheduled_at,resolved_at FROM public.fluctlight_life_activity_runs WHERE fluctlight_id=$1 AND intention_id=$2 ORDER BY created_at DESC LIMIT 10`, fluctlightID, id)
		if err != nil {
			return failedCapabilityResult(invocation, "intention_activity_read_failed", true), err
		}
		defer rows.Close()
		activities := make([]map[string]any, 0)
		for rows.Next() {
			var activityID, kind, activityStatus string
			var scheduled time.Time
			var resolved *time.Time
			if err := rows.Scan(&activityID, &kind, &activityStatus, &scheduled, &resolved); err != nil {
				return failedCapabilityResult(invocation, "intention_activity_read_failed", true), err
			}
			entry := map[string]any{"id": activityID, "kind": kind, "status": activityStatus, "scheduled_at": scheduled.UTC().Format(time.RFC3339Nano)}
			if resolved != nil {
				entry["resolved_at"] = resolved.UTC().Format(time.RFC3339Nano)
			}
			activities = append(activities, entry)
		}
		if err := rows.Err(); err != nil {
			return failedCapabilityResult(invocation, "intention_activity_read_failed", true), err
		}
		output["activities"] = activities
	} else {
		includeClosed, _ := args["include_closed"].(bool)
		rows, err := c.service.repository.Pool().Query(ctx, `SELECT i.id,g.desired_outcome,i.action_intent,i.expected_outcome,i.status,i.revision,i.expiration FROM public.fluctlight_intentions i JOIN public.fluctlight_goals g ON g.id=i.goal_id AND g.fluctlight_id=i.fluctlight_id WHERE i.fluctlight_id=$1 AND COALESCE(i.profile_id,'')=$2 AND ($3 OR i.status NOT IN ('completed','cancelled','expired')) ORDER BY i.created_at DESC LIMIT 21`, fluctlightID, profileID, includeClosed)
		if err != nil {
			return failedCapabilityResult(invocation, "intention_read_failed", true), err
		}
		defer rows.Close()
		items := make([]map[string]any, 0)
		for rows.Next() {
			var id, goal, action, expected, status string
			var revision int
			var expiration time.Time
			if err := rows.Scan(&id, &goal, &action, &expected, &status, &revision, &expiration); err != nil {
				return failedCapabilityResult(invocation, "intention_read_failed", true), err
			}
			items = append(items, map[string]any{"id": id, "goal": goal, "action": action, "expected_outcome": expected, "status": status, "revision": revision, "expiration": expiration.UTC().Format(time.RFC3339Nano)})
		}
		if err := rows.Err(); err != nil {
			return failedCapabilityResult(invocation, "intention_read_failed", true), err
		}
		output["has_more"] = len(items) > 20
		if len(items) > 20 {
			items = items[:20]
		}
		output["intentions"] = items
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: output,
		ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "intention-query:" + stableDigest(invocation.CallID)}, nil
}

func intentionDecideDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: intentionDecideCapabilityName, Version: "v1", Type: CapabilityTypeAction,
		Description:   "Create, qualify, adjust, pause, resume or cancel a durable intention for the current speaking profile. Completion is reserved for a verified ActionOutcome; this Tool cannot claim that a plan already succeeded.",
		Surfaces:      []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceNativeCognition},
		FailurePolicy: FailurePolicyOptionalInternal,
		InputSchema: objectSchema(map[string]any{
			"operation":        enumStringSchema("create", "qualify", "update", "pause", "resume", "cancel"),
			"intention_id":     map[string]any{"type": "string", "maxLength": 128},
			"goal":             map[string]any{"type": "string", "maxLength": 2000},
			"action":           map[string]any{"type": "string", "maxLength": 2000},
			"expected_outcome": map[string]any{"type": "string", "maxLength": 2000},
			"reason":           map[string]any{"type": "string", "minLength": 1, "maxLength": 500},
		}, []string{"operation", "reason"}, false),
		OutputSchema: openObjectSchema(), SideEffectClass: "native_projection", SuccessBoundary: "intention_revision_committed",
		ConcurrencyClass: "exclusive", SupportsRetry: true,
	}
}
func (c intentionDecideCapability) Definition() CapabilityDefinition {
	return intentionDecideDefinition()
}
func (c intentionDecideCapability) RequiredContext() []ContextSlot { return nil }
func (c intentionDecideCapability) Execute(_ context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return executeToolRequired(invocation)
}
func (s *intentionService) resolveProfile(ctx context.Context, query DBTX, fluctlightID, requested string) (string, error) {
	if requested = strings.TrimSpace(requested); requested != "" {
		return requested, nil
	}
	var active string
	if err := query.QueryRow(ctx, `SELECT active_profile_id FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1`, fluctlightID).Scan(&active); err != nil {
		return "", err
	}
	return active, nil
}
func (c intentionDecideCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	if c.service == nil || c.service.repository == nil {
		return failedCapabilityResult(invocation, "intention_unavailable", true), errors.New("intention service unavailable")
	}
	args, err := capabilityExecutionArguments(invocation, intentionDecideDefinition())
	if err != nil {
		return failedCapabilityResult(invocation, "invalid_arguments", false), err
	}
	fluctlightID := invocation.Metadata.FluctlightID
	profileID, err := c.service.resolveProfile(ctx, tx, fluctlightID, invocation.Metadata.WorkingProfileID)
	if err != nil {
		return failedCapabilityResult(invocation, "intention_profile_unavailable", true), err
	}
	operation := stringValue(args["operation"])
	reason := strings.TrimSpace(stringValue(args["reason"]))
	if reason == "" {
		return failedCapabilityResult(invocation, "intention_reason_required", false), ErrInvalidArguments
	}
	at := time.Now().UTC()
	evidence := []string{"tool-operation:" + stableDigest(fluctlightID+"\x1f"+capabilityOperationID(invocation))}
	if operation == "create" {
		return c.service.createIntentionTx(ctx, tx, invocation, profileID, args, reason, evidence, at)
	}
	id := strings.TrimSpace(stringValue(args["intention_id"]))
	if id == "" {
		return failedCapabilityResult(invocation, "intention_id_required", false), ErrInvalidArguments
	}
	var goalID, storedProfile string
	var revision int
	err = tx.QueryRow(ctx, `SELECT goal_id,COALESCE(profile_id,''),revision FROM public.fluctlight_intentions WHERE id=$1 AND fluctlight_id=$2 FOR UPDATE`, id, fluctlightID).Scan(&goalID, &storedProfile, &revision)
	if errors.Is(err, pgx.ErrNoRows) || storedProfile != profileID {
		return failedCapabilityResult(invocation, "intention_not_found", false), ErrNotFound
	}
	if err != nil {
		return failedCapabilityResult(invocation, "intention_read_failed", true), err
	}
	current, err := loadIntentionAuthorityTx(ctx, tx, fluctlightID, "intention:ctx_"+stableDigest(id), "goal:ctx_"+stableDigest(goalID), ContextReference{EntityID: id, Revision: revision})
	if err != nil {
		return failedCapabilityResult(invocation, "intention_authority_invalid", true), err
	}
	command := IntentionCommand{ExpectedRevision: current.Revision, EvidenceRefs: evidence, Reason: reason, OccurredAt: at}
	switch operation {
	case "qualify":
		command.Operation = IntentionQualify
	case "update":
		command.Operation = IntentionUpdate
		if text := strings.TrimSpace(stringValue(args["action"])); text != "" {
			command.Patch.ActionIntent = &text
		}
		if text := strings.TrimSpace(stringValue(args["expected_outcome"])); text != "" {
			command.Patch.ExpectedOutcome = &text
		}
		if command.Patch.ActionIntent == nil && command.Patch.ExpectedOutcome == nil {
			return failedCapabilityResult(invocation, "intention_update_empty", false), ErrInvalidArguments
		}
	case "pause":
		command.Operation = IntentionPause
	case "resume":
		command.Operation = IntentionResume
	case "cancel":
		command.Operation = IntentionCancel
	default:
		return failedCapabilityResult(invocation, "intention_operation_invalid", false), ErrInvalidArguments
	}
	next, record, err := ApplyIntentionCommand(current, command)
	if err != nil {
		return failedCapabilityResultDetail(invocation, "intention_transition_rejected", false, err.Error()), err
	}
	if _, err := persistIntentionAuthorityTx(ctx, tx, &current, next, record, "tool:intention:"+stableDigest(fluctlightID+"\x1f"+capabilityOperationID(invocation))); err != nil {
		return failedCapabilityResult(invocation, "intention_persist_failed", true), err
	}
	if err := syncIntentionTriggerWorkflowTx(ctx, tx, next); err != nil {
		return failedCapabilityResult(invocation, "intention_trigger_failed", true), err
	}
	if err := appendOutboxTx(ctx, tx, "intention.revised", "fluctlight", fluctlightID, fluctlightID, invocation.SourceFactID,
		"intention:"+id, "intention-tool:"+id+":"+fmt.Sprint(next.Revision), map[string]any{"intention_id": id, "status": string(next.Status), "revision": next.Revision}); err != nil {
		return failedCapabilityResult(invocation, "intention_event_failed", true), err
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed",
		Output:            map[string]any{"intention_id": id, "goal_id": goalID, "status": string(next.Status), "revision": next.Revision},
		ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "intention:" + id}, nil
}

func (s *intentionService) createIntentionTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, profileID string, args map[string]any, reason string, evidence []string, at time.Time) (CapabilityResult, error) {
	fluctlightID := invocation.Metadata.FluctlightID
	goalText := strings.TrimSpace(stringValue(args["goal"]))
	action := strings.TrimSpace(stringValue(args["action"]))
	expected := strings.TrimSpace(stringValue(args["expected_outcome"]))
	if goalText == "" || action == "" || expected == "" {
		return failedCapabilityResult(invocation, "intention_create_fields_required", false), ErrInvalidArguments
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "intention-create:"+fluctlightID+":"+profileID+":"+strings.ToLower(goalText)+":"+strings.ToLower(action)); err != nil {
		return failedCapabilityResult(invocation, "intention_lock_failed", true), err
	}
	var existingID, existingGoalID, existingStatus string
	err := tx.QueryRow(ctx, `SELECT i.id,g.id,i.status FROM public.fluctlight_intentions i JOIN public.fluctlight_goals g ON g.id=i.goal_id AND g.fluctlight_id=i.fluctlight_id WHERE i.fluctlight_id=$1 AND COALESCE(i.profile_id,'')=$2 AND lower(trim(i.action_intent))=$3 AND lower(trim(g.desired_outcome))=$4 AND i.status NOT IN ('completed','cancelled','expired') ORDER BY i.created_at LIMIT 1 FOR UPDATE OF i`, fluctlightID, profileID, strings.ToLower(action), strings.ToLower(goalText)).Scan(&existingID, &existingGoalID, &existingStatus)
	if err == nil {
		return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed",
			Output:            map[string]any{"intention_id": existingID, "goal_id": existingGoalID, "status": existingStatus, "reused": true},
			ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "intention:" + existingID}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return failedCapabilityResult(invocation, "intention_lookup_failed", true), err
	}
	identity := stableDigest(fluctlightID + "\x1f" + profileID + "\x1f" + capabilityOperationID(invocation))
	goalID, intentionID := "goal_tool_"+identity, "intention_tool_"+identity
	goal := GoalAuthority{
		EntityID: goalID, SchemaVersion: goalAuthoritySchemaVersion, Ref: "goal:ctx_" + stableDigest(goalID),
		FluctlightID: fluctlightID, ProfileID: profileID, DesiredOutcome: goalText, SuccessCriteria: []string{expected},
		Motivation: reason, Scope: "personal", Importance: 0.5, Urgency: 0.5, Progress: 0,
		Status: GoalActive, Revision: 1, EvidenceRefs: evidence,
	}
	createdGoal, goalRecord, err := CreateGoalAuthority(goal, evidence, at)
	if err != nil {
		return failedCapabilityResult(invocation, "goal_create_invalid", false), err
	}
	if _, err := persistGoalAuthorityTx(ctx, tx, nil, createdGoal, goalRecord, "tool:goal:"+identity); err != nil {
		return failedCapabilityResult(invocation, "goal_persist_failed", true), err
	}
	intention := IntentionAuthority{
		EntityID: intentionID, GoalEntityID: goalID, SchemaVersion: intentionAuthoritySchemaVersion,
		Ref: "intention:ctx_" + stableDigest(intentionID), FluctlightID: fluctlightID, ProfileID: profileID, GoalRef: createdGoal.Ref,
		ActionIntent: action, ExpectedOutcome: expected, Trigger: TypedIntentionTrigger{Type: IntentionTriggerSemantic},
		Expiration: at.Add(30 * 24 * time.Hour), Confidence: 0.8, Status: IntentionCandidate, Revision: 1, EvidenceRefs: evidence,
	}
	created, record, err := CreateIntentionAuthority(intention, evidence, at)
	if err != nil {
		return failedCapabilityResult(invocation, "intention_create_invalid", false), err
	}
	if _, err := persistIntentionAuthorityTx(ctx, tx, nil, created, record, "tool:intention:"+identity); err != nil {
		return failedCapabilityResult(invocation, "intention_persist_failed", true), err
	}
	if err := appendOutboxTx(ctx, tx, "intention.created", "fluctlight", fluctlightID, fluctlightID, invocation.SourceFactID,
		"intention:"+intentionID, "intention-tool-created:"+intentionID, map[string]any{"intention_id": intentionID, "goal_id": goalID, "status": string(created.Status)}); err != nil {
		return failedCapabilityResult(invocation, "intention_event_failed", true), err
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed",
		Output:            map[string]any{"intention_id": intentionID, "goal_id": goalID, "status": string(created.Status), "reused": false},
		ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "intention:" + intentionID}, nil
}

func transitionLinkedIntentionTx(ctx context.Context, tx pgx.Tx, fluctlightID, profileID, intentionID string, operation IntentionLifecycleOperation, evidenceRef, reason, commandKey string, at time.Time) (IntentionAuthority, error) {
	var goalID, storedProfile string
	var revision int
	err := tx.QueryRow(ctx, `SELECT goal_id,COALESCE(profile_id,''),revision FROM public.fluctlight_intentions WHERE id=$1 AND fluctlight_id=$2 FOR UPDATE`, intentionID, fluctlightID).Scan(&goalID, &storedProfile, &revision)
	if errors.Is(err, pgx.ErrNoRows) || storedProfile != profileID {
		return IntentionAuthority{}, ErrNotFound
	}
	if err != nil {
		return IntentionAuthority{}, err
	}
	current, err := loadIntentionAuthorityTx(ctx, tx, fluctlightID, "intention:ctx_"+stableDigest(intentionID), "goal:ctx_"+stableDigest(goalID), ContextReference{EntityID: intentionID, Revision: revision})
	if err != nil {
		return IntentionAuthority{}, err
	}
	command := IntentionCommand{Operation: operation, ExpectedRevision: current.Revision, EvidenceRefs: []string{evidenceRef}, Reason: reason, OccurredAt: at}
	next, record, err := ApplyIntentionCommand(current, command)
	if err != nil {
		return IntentionAuthority{}, err
	}
	if _, err := persistIntentionAuthorityTx(ctx, tx, &current, next, record, commandKey); err != nil {
		return IntentionAuthority{}, err
	}
	if err := syncIntentionTriggerWorkflowTx(ctx, tx, next); err != nil {
		return IntentionAuthority{}, err
	}
	return next, nil
}

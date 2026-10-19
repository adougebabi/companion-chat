package core

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Serialize Owner command identities before the common idempotency read. Domain
// locks/CAS below remain necessary: cognition does not take this command lock.
func (a *App) dispatchOwnerPersonaSwitch(ctx context.Context, method, path string, query url.Values, owner string, body map[string]any) (map[string]any, error) {
	var out map[string]any
	var business error
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "owner-persona:"+owner); err != nil {
			return err
		}
		if body == nil {
			body = map[string]any{}
		}
		var storage error
		out, business, storage = NewOwnerSurface(nil, a.surfaceHooks()).dispatchTx(context.WithValue(ctx, surfaceDBTxKey{}, tx), postgresSurfaceTx{tx}, method, path, query, owner, body)
		if storage == nil && business == nil && out["status"] == "pending" && stringValue(out["request_id"]) == "" {
			out = nil
			business = &SurfaceError{Status: 409, Code: "persona_switch_legacy_pending", Message: "The previous persona request was not executed; refresh runtime and submit a new command", Replay: true}
		}
		return storage
	})
	if err != nil {
		return nil, &SurfaceError{Status: 503, Code: "persona_switch_unavailable", Message: "Persona switch could not be committed"}
	}
	return out, business
}

func (a *App) applyOwnerPersonaSwitch(ctx context.Context, owner, fid string, body map[string]any) (map[string]any, error) {
	tx, ok := SurfaceDBTx(ctx)
	if !ok {
		return nil, errors.New("persona_owner_transaction_required")
	}
	expected, valid := parseRevision(body["expected_revision"])
	if !valid {
		return nil, invalid("expectedRevision is required")
	}
	runtimeExpected, valid := parseRevision(body["expected_runtime_revision"])
	if !valid {
		return nil, invalid("expectedRuntimeRevision is required")
	}
	reason := strings.TrimSpace(stringValue(body["reason"]))
	if reason == "" || len([]rune(reason)) > 1000 {
		return nil, invalid("reason must contain 1–1000 characters")
	}
	var raw []byte
	var revision int
	var status string
	err := tx.QueryRow(ctx, `SELECT core_persona,current_revision,status FROM public.fluctlights WHERE id=$1 AND created_by_actor_id=$2 FOR UPDATE`, fid, owner).Scan(&raw, &revision, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &SurfaceError{Status: 404, Code: "fluctlight_not_found", Message: "Fluctlight is unavailable"}
	}
	if err != nil {
		return nil, err
	}
	if status == "retired" {
		return nil, &SurfaceError{Status: 409, Code: "fluctlight_retired", Message: "A retired Fluctlight cannot switch persona"}
	}
	if revision != expected {
		return nil, conflict("foundation", expected, revision)
	}
	// Lock the runtime even when the requested target is already current. Missing
	// legacy rows are fenced by the Foundation lock and insert CAS in the writer.
	var current string
	var runtimeRevision int
	err = tx.QueryRow(ctx, `SELECT active_profile_id,revision FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1 FOR UPDATE`, fid).Scan(&current, &runtimeRevision)
	persona := decodeObject(raw)
	if errors.Is(err, pgx.ErrNoRows) {
		current = initialPersonalityProfileID(persona)
	} else if err != nil {
		return nil, err
	}
	if runtimeRevision != runtimeExpected {
		return nil, conflict("personality_runtime", runtimeExpected, runtimeRevision)
	}
	target := strings.TrimSpace(stringValue(body["target_persona_id"]))
	if _, declared := personalityProfileIDs(persona)[target]; !declared {
		return nil, &SurfaceError{Status: 422, Code: "personality_target_profile_not_found", Message: "Target persona is not declared"}
	}
	trigger := ""
	if target != current {
		return a.enqueueOwnerPersonaSwitchTx(ctx, tx, owner, fid, current, target, revision, runtimeRevision, body)
	}
	plan, err := a.preparePersonalityDecisionQuery(ctx, tx, fid, map[string]any{"decision": "switch", "from_profile_id": current, "target_profile_id": target, "trigger_id": trigger, "reason": reason})
	if err != nil {
		if code, ok := personaActionBusinessRejection(err); ok {
			return nil, &SurfaceError{Status: 409, Code: code, Message: "Persona switch precondition is not satisfied"}
		}
		return nil, err
	}
	key := stringValue(body["idempotency_key"])
	operationID := "owner-switch:" + stableDigest(owner+":"+fid+":"+key)
	requestID := "req_persona_" + stableDigest(operationID)
	args := map[string]any{"decision": "switch", "source_profile_id": current, "target_profile_id": target, "trigger_id": trigger, "reason": reason}
	invocation := CapabilityInvocation{CallID: operationID, CapabilityName: personaSwitchCapabilityName, SourceFactID: requestID, Arguments: jsonBytes(args), Metadata: InvocationMetadata{OperationID: operationID, FluctlightID: fid, CorrelationID: operationID, Source: "owner_command"}}
	prepared := preparedPersonaAction{SchemaVersion: personaActionPlanVersion, CapabilityName: personaSwitchCapabilityName, OperationID: operationID, FluctlightID: fid, EvidenceID: requestID, OccurredAt: a.now().UTC(), Decision: "switch", Arguments: args, SwitchPlan: plan}
	prepared.RequestDigest = personaActionRequestDigest(prepared)
	invocation, err = withCapabilityPreparedData(invocation, "persona_action_plan", prepared)
	if err != nil {
		return nil, err
	}
	result, err := (&personaActionBusinessService{app: a}).applyPersonaActionTx(ctx, tx, invocation)
	if err != nil {
		return nil, err
	}
	output := mapValue(result.Output)
	if result.Status != "completed" && stringValue(output["disposition"]) != "no_change" {
		return nil, errors.New("persona_owner_action_not_committed")
	}
	return map[string]any{"request_id": requestID, "status": "completed", "disposition": output["disposition"], "revision": revision, "runtime_revision": plan.ResultingRevision, "active_profile_id": target, "previous_profile_id": plan.PreviousProfile, "persona_action_audit_id": output["audit_id"]}, nil
}

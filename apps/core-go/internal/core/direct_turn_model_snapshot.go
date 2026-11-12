package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

const (
	directTurnModelSnapshotContractKey = "_direct_turn_model_snapshot_contract"
	directTurnModelSnapshotKey         = "direct_turn_model_snapshot"
)

var (
	errDirectTurnModelSnapshotInvalid = errors.New("direct_turn_model_snapshot_invalid")
	errDirectTurnModelSnapshotMissing = errors.New("direct_turn_model_snapshot_missing")
	errDirectTurnSnapshotStale        = errors.New("direct_turn_snapshot_stale")
)

type directTurnClaimKey struct{}
type directTurnClaim struct{ InboxID, Owner string }

func directTurnClaimMatches(ctx context.Context, source, status, claimedBy string) bool {
	claim, _ := ctx.Value(directTurnClaimKey{}).(directTurnClaim)
	if claim.InboxID == source && claim.Owner != "" {
		return status == "claimed" && claim.Owner == claimedBy
	}
	return status != "claimed"
}

func directTurnSnapshotFromPayload(payload map[string]any) (any, bool, bool, error) {
	contract, marked := payload[directTurnModelSnapshotContractKey]
	if marked && contract != workflowModelSnapshotVersion {
		return nil, true, false, errDirectTurnModelSnapshotInvalid
	}
	partial, exists := payload["agent_partial"]
	if !exists {
		return nil, marked, false, nil
	}
	values, ok := partial.(map[string]any)
	if !ok || values == nil {
		if marked {
			return nil, true, false, errDirectTurnModelSnapshotInvalid
		}
		return nil, false, false, nil
	}
	snapshot, found := values[directTurnModelSnapshotKey]
	if found && !marked {
		return nil, false, true, errDirectTurnModelSnapshotInvalid
	}
	return snapshot, marked, found, nil
}

// A new direct turn owns one durable model binding across attempts. The
// snapshot is stored under agent_partial, which is excluded from cognition
// source fingerprints; it is not a model input or a committed business effect.
func (a *App) withDirectTurnCognitionSnapshot(ctx context.Context, input ConversationCognitionAgentInput, direct bool) (context.Context, error) {
	source := strings.TrimSpace(input.SourceFactID)
	if source == "" {
		return ctx, errDirectTurnModelSnapshotInvalid
	}
	read := func(q lifeContextQuerier, lock bool) (map[string]any, string, string, error) {
		query := `SELECT i.payload,i.status,COALESCE(i.claimed_by,'') FROM public.cognition_inbox i JOIN public.fluctlights f ON f.id=i.fluctlight_id WHERE i.id=$1 AND i.fluctlight_id=$2 AND i.event_type='conversation.turn' AND i.payload->>'conversation_id'=$3 AND f.created_by_actor_id=$4`
		if lock {
			query += ` FOR UPDATE OF i`
		}
		var raw []byte
		var status, claimedBy string
		if err := q.QueryRow(ctx, query, source, input.FluctlightID, input.ConversationID, input.AuthorizationActorID).Scan(&raw, &status, &claimedBy); err != nil {
			return nil, "", "", err
		}
		payload := decodeObject(raw)
		if stringValue(payload["text"]) != strings.TrimSpace(input.CurrentInput) || (status != "pending" && status != "claimed") {
			return nil, "", "", ErrConflict
		}
		return payload, status, claimedBy, nil
	}
	payload, status, claimedBy, err := read(a.DB.Pool(), false)
	if err != nil {
		if errors.Is(err, ErrConflict) {
			return ctx, errDirectTurnSnapshotStale
		}
		return ctx, err
	}
	if !directTurnClaimMatches(ctx, source, status, claimedBy) {
		return ctx, errDirectTurnSnapshotStale
	}
	_, marked, found, err := directTurnSnapshotFromPayload(payload)
	if err != nil {
		return ctx, err
	}
	if !direct && !found {
		return ctx, nil
	}
	if !marked {
		a.recordDiagnosticEvent(ctx, "direct_turn.model_binding.legacy_unfrozen", "warn", input.FluctlightID, source, "turn:"+stringValue(payload["turn_id"]), map[string]any{"reason_code": "legacy_model_binding_unavailable"})
		return ctx, nil
	}
	var candidate providerAssignment
	if !found {
		route := withAgentModelRoute(ctx, FormalAgentConversationCognition, replyCognitionSnapshotVariant)
		candidate, err = a.Provider.assignment(route, replyCognitionSnapshotRole)
		if err != nil {
			return ctx, err
		}
	}
	var selected any
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := requireConversationBudgetTx(ctx, tx, source, a.now().UTC(), true, true); err != nil {
			return err
		}
		current, status, claimedBy, err := read(tx, true)
		if err != nil {
			return err
		}
		if !directTurnClaimMatches(ctx, source, status, claimedBy) {
			return ErrConflict
		}
		selected, marked, found, err = directTurnSnapshotFromPayload(current)
		if err != nil || !marked {
			return errDirectTurnModelSnapshotInvalid
		}
		if found {
			return nil
		}
		if candidate.ModelID == "" {
			return ErrConflict
		}
		baseRun := "turn:" + stringValue(current["turn_id"])
		var started bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.agent_runs WHERE fluctlight_id=$1 AND agent_id=$2 AND correlation_id=$3 AND (run_id=$3 OR strpos(run_id,$3||':')=1))`, input.FluctlightID, FormalAgentConversationCognition, baseRun).Scan(&started); err != nil {
			return err
		}
		if started {
			return errDirectTurnModelSnapshotMissing
		}
		selected = newWorkflowModelSnapshot(FormalAgentConversationCognition, replyCognitionSnapshotVariant, replyCognitionSnapshotRole, candidate, 0, 0)
		_, err = tx.Exec(ctx, `UPDATE public.cognition_inbox SET payload=jsonb_set(payload,'{agent_partial}',COALESCE(payload->'agent_partial','{}'::jsonb)||jsonb_build_object($2::text,$3::jsonb),true) WHERE id=$1`, source, directTurnModelSnapshotKey, jsonBytes(selected))
		return err
	})
	if err != nil {
		if errors.Is(err, ErrConflict) {
			return ctx, errDirectTurnSnapshotStale
		}
		return ctx, err
	}
	restored, ok, err := a.restoreReplyCognitionSnapshot(ctx, map[string]any{replyCognitionSnapshotKey: selected})
	if err != nil || !ok {
		return ctx, fmt.Errorf("%w: %v", errDirectTurnModelSnapshotInvalid, err)
	}
	return restored, nil
}

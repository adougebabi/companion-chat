package core

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// lifeContextCommandResultTx implements the replay half of the owner/governance
// command boundary. The command ledger is deliberately separate from Event,
// Presence, and Schedule rows: those rows keep the result of their creation,
// while later cancel/supersede commands retain their own immutable result.
func lifeContextCommandResultTx(
	ctx context.Context,
	tx pgx.Tx,
	fluctlightID string,
	commandType string,
	targetID string,
	idempotencyKey string,
	requestDigest string,
	conflictCode string,
) (map[string]any, bool, error) {
	var storedType, storedTarget, storedDigest string
	var storedResult []byte
	err := tx.QueryRow(ctx, `
		SELECT command_type,COALESCE(target_id,''),request_digest,result
		FROM public.life_context_commands
		WHERE fluctlight_id=$1 AND idempotency_key=$2
		FOR UPDATE`, fluctlightID, idempotencyKey).Scan(&storedType, &storedTarget, &storedDigest, &storedResult)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if storedType != commandType || storedTarget != targetID || storedDigest != requestDigest {
		return nil, false, errors.New(conflictCode)
	}
	result := decodeObject(storedResult)
	if !validLifeContextCommandResult(result, storedTarget, idempotencyKey) {
		return nil, false, errors.New("life_context_replay_result_invalid")
	}
	result["replayed"] = true
	return result, true, nil
}

func storeLifeContextCommandResultTx(
	ctx context.Context,
	tx pgx.Tx,
	fluctlightID string,
	commandType string,
	targetID string,
	idempotencyKey string,
	requestDigest string,
	result map[string]any,
) error {
	if strings.TrimSpace(fluctlightID) == "" || strings.TrimSpace(commandType) == "" ||
		strings.TrimSpace(idempotencyKey) == "" || strings.TrimSpace(requestDigest) == "" || len(result) == 0 {
		return errors.New("life_context_command_invalid")
	}
	if !validLifeContextCommandResult(result, targetID, idempotencyKey) {
		return errors.New("life_context_command_result_invalid")
	}
	commandID := "life_command_" + stableDigest(fluctlightID+"\x1f"+idempotencyKey)
	inserted, err := tx.Exec(ctx, `
		INSERT INTO public.life_context_commands(
			id,fluctlight_id,command_type,target_id,idempotency_key,request_digest,result
		) VALUES($1,$2,$3,$4,$5,$6,$7)`,
		commandID, fluctlightID, commandType, nullableString(targetID), idempotencyKey, requestDigest, jsonBytes(result))
	if err != nil {
		return err
	}
	if inserted.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}

func validLifeContextCommandResult(result map[string]any, targetID, idempotencyKey string) bool {
	return len(result) > 0 &&
		stringValue(result["id"]) == strings.TrimSpace(targetID) &&
		stringValue(result["status"]) != "" &&
		intValue(result["revision"]) > 0 &&
		stringValue(result["expected_context_revision"]) != "" &&
		stringValue(result["resulting_context_revision"]) != "" &&
		stringValue(result["idempotency_key"]) == strings.TrimSpace(idempotencyKey) &&
		result["replayed"] == false
}

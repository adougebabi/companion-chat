package core

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"strings"
)

func (a *App) ActorContext(ctx context.Context, actor, owner, target string) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, owner, actor); err != nil {
		return nil, err
	}
	if err := a.requireVisibleActorWith(ctx, a.DB.Pool(), owner, actor, "*", target); err != nil {
		return nil, err
	}
	return a.readActorContextWith(ctx, a.DB.Pool(), owner, target)
}
func (a *App) readActorContextWith(ctx context.Context, q lifeContextQuerier, owner, target string) (map[string]any, error) {
	background, err := a.readActorUserBackground(ctx, q, target, owner, a.now())
	if err != nil {
		return nil, err
	}
	history, err := readActorFactsWith(ctx, q, owner, target, a.now(), true, 50)
	if err != nil {
		return nil, err
	}
	background["history"] = history
	background["actor_id"] = target
	rows, err := q.Query(ctx, `SELECT to_jsonb(r) FROM public.relationships r WHERE owner_fluctlight_id=$1 AND target_actor_id=$2 ORDER BY updated_at DESC,id LIMIT 20`, owner, target)
	if err != nil {
		return nil, err
	}
	relations := []any{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			rows.Close()
			return nil, err
		}
		relations = append(relations, decodeObject(raw))
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	background["relationships"] = relations
	background["context_version"] = stableDigest(jsonString(map[string]any{"facts": history, "relationships": relations}))
	rows, err = q.Query(ctx, `SELECT to_jsonb(v) FROM public.relationship_revisions v JOIN public.relationships r ON r.id=v.relationship_id WHERE r.owner_fluctlight_id=$1 AND r.target_actor_id=$2 ORDER BY v.created_at DESC,v.id DESC LIMIT 30`, owner, target)
	if err != nil {
		return nil, err
	}
	revisions := []any{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			rows.Close()
			return nil, err
		}
		revisions = append(revisions, decodeObject(raw))
	}
	rows.Close()
	background["relationship_history"] = revisions
	return background, rows.Err()
}
func (a *App) UpdateActorContext(ctx context.Context, actor, owner, target string, body map[string]any) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, owner, actor); err != nil {
		return nil, err
	}
	allowed := map[string]bool{"background": true, "expected_context_version": true, "idempotency_key": true, "reason": true, "operation": true}
	for key := range body {
		if !allowed[key] {
			return nil, ErrInvalidArguments
		}
	}
	settings := map[string]any{"background": body["background"]}
	if err := validateActorUserSettings(settings); err != nil {
		return nil, err
	}
	version, key, reason := stringValue(body["expected_context_version"]), stringValue(body["idempotency_key"]), strings.TrimSpace(stringValue(body["reason"]))
	operation := stringValue(body["operation"])
	if version == "" || key == "" || len(key) > 128 || reason == "" || len(reason) > 1000 || (operation != "correct" && operation != "change") {
		return nil, ErrInvalidArguments
	}
	digest := stableDigest(jsonString(body))
	var result map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := lockLifeContextTx(ctx, tx, owner); err != nil {
			return err
		}
		if err := a.requireVisibleActorWith(ctx, tx, owner, actor, "*", target); err != nil {
			return err
		}
		ledgerKey := "actor-context:" + target + ":" + key
		var saved string
		var raw []byte
		err := tx.QueryRow(ctx, `SELECT request_digest,result FROM public.actor_user_background_commands WHERE fluctlight_id=$1 AND idempotency_key=$2`, owner, ledgerKey).Scan(&saved, &raw)
		if err == nil {
			if saved != digest {
				return ErrConflict
			}
			result = decodeObject(raw)
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		current, err := a.readActorContextWith(ctx, tx, owner, target)
		if err != nil {
			return err
		}
		if stringValue(current["context_version"]) != version {
			return ErrConflict
		}
		previous := mapValue(current["background"])
		service := actorFactCapability{service: &actorFactService{repository: a.DB, clock: a.Clock}}
		for field, value := range mapValue(body["background"]) {
			if old, ok := previous[field]; ok && jsonString(old) == jsonString(value) {
				continue
			}
			transition := operation
			if _, exists := previous[field]; !exists {
				transition = "assert"
			}
			inv := CapabilityInvocation{CallID: ledgerKey + ":" + field, CapabilityName: actorFactCapabilityName, Arguments: jsonBytes(map[string]any{"actor_id": target, "operation": transition, "attribute": field, "value": value, "reason": reason}), Metadata: InvocationMetadata{Source: "direct", OperationID: ledgerKey + ":" + field, AuthorizationActorID: actor, SubjectActorID: target, FluctlightID: owner}}
			if _, err := service.ExecuteTx(ctx, tx, inv, CapabilityContext{}); err != nil {
				return err
			}
		}
		result, err = a.readActorContextWith(ctx, tx, owner, target)
		if err != nil {
			return err
		}
		result["review_status"] = "related_goals_require_review"
		_, err = tx.Exec(ctx, `INSERT INTO public.actor_user_background_commands(fluctlight_id,idempotency_key,actor_id,request_digest,result) VALUES($1,$2,$3,$4,$5)`, owner, ledgerKey, actor, digest, jsonBytes(result))
		return err
	})
	return result, err
}

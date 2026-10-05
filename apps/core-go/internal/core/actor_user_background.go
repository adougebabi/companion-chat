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

// Settings are owner-authored inputs. actor_facts remains the sole runtime
// authority; no copy is stored in the Fluctlight's own Foundation.
var actorUserBackgroundFields = []string{"name", "occupation", "background", "location_scope", "location", "timezone", "relationship_distance", "meeting_confirmed"}

func actorUserSettingsSchema() map[string]any {
	fields := map[string]any{}
	for _, key := range actorUserBackgroundFields {
		if key == "meeting_confirmed" {
			fields[key] = map[string]any{"type": []any{"boolean", "null"}}
		} else {
			fields[key] = map[string]any{"type": []any{"string", "null"}, "minLength": 1, "maxLength": 1024}
		}
	}
	return objectSchema(map[string]any{"background": objectSchema(fields, nil, false)}, []string{"background"}, false)
}

func validateActorUserSettings(value any) error {
	settings, ok := value.(map[string]any)
	if !ok || len(settings) != 1 {
		return fmt.Errorf("%w: actor_user must contain only background", ErrInvalidArguments)
	}
	background, ok := settings["background"].(map[string]any)
	if !ok {
		return fmt.Errorf("%w: actor_user.background must be an object", ErrInvalidArguments)
	}
	allowed := map[string]bool{}
	for _, key := range actorUserBackgroundFields {
		allowed[key] = true
	}
	for key, value := range background {
		if !allowed[key] {
			return fmt.Errorf("%w: actor_user.background.%s is unsupported", ErrInvalidArguments, key)
		}
		if value == nil {
			continue
		}
		if key == "meeting_confirmed" {
			if _, ok := value.(bool); !ok {
				return fmt.Errorf("%w: actor_user.background.%s must be boolean or null", ErrInvalidArguments, key)
			}
			continue
		}
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" || len([]rune(text)) > 1024 {
			return fmt.Errorf("%w: actor_user.background.%s must be a bounded string or null", ErrInvalidArguments, key)
		}
		if key == "timezone" {
			if text == "Local" {
				return fmt.Errorf("%w: actor_user.background.timezone must be an IANA zone", ErrInvalidArguments)
			}
			if _, err := time.LoadLocation(text); err != nil {
				return fmt.Errorf("%w: actor_user.background.timezone is invalid", ErrInvalidArguments)
			}
		}
	}
	return nil
}

func (a *App) readActorUserBackground(ctx context.Context, q lifeContextQuerier, actorID, fluctlightID string, at time.Time) (map[string]any, error) {
	facts, err := readActorFactsWith(ctx, q, fluctlightID, actorID, at, false, 32)
	if err != nil {
		return nil, err
	}
	background := map[string]any{}
	for _, fact := range facts {
		key := stringValue(fact["attribute"])
		for _, allowed := range actorUserBackgroundFields {
			if key == allowed {
				background[key] = fact["value"]
				break
			}
		}
	}
	return map[string]any{"background": background, "facts": facts}, nil
}

func (a *App) writeActorUserBackgroundTx(ctx context.Context, tx pgx.Tx, actorID, fluctlightID, operationID, transition, reason string, settings map[string]any) error {
	service := actorFactCapability{service: &actorFactService{repository: a.DB, clock: a.Clock}}
	current, err := a.readActorUserBackground(ctx, tx, actorID, fluctlightID, a.now().UTC())
	if err != nil {
		return err
	}
	previous := mapValue(current["background"])
	background := mapValue(settings["background"])
	keys := make([]string, 0, len(background))
	for key := range background {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		old, exists := previous[key]
		if exists && jsonString(old) == jsonString(background[key]) {
			continue
		}
		operation := transition
		if !exists {
			operation = "assert"
		}
		args := map[string]any{"operation": operation, "attribute": key, "value": background[key], "reason": reason}
		invocation := CapabilityInvocation{CallID: operationID + ":" + key, CapabilityName: actorFactCapabilityName, Arguments: jsonBytes(args), Metadata: InvocationMetadata{Source: "direct", OperationID: operationID + ":" + key, AuthorizationActorID: actorID, SubjectActorID: actorID, FluctlightID: fluctlightID, CorrelationID: operationID}}
		if _, err := service.ExecuteTx(ctx, tx, invocation, CapabilityContext{}); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) initializeActorUserBackgroundTx(ctx context.Context, tx pgx.Tx, actorID, fluctlightID string, foundation map[string]any) error {
	value, exists := foundation["actor_user"]
	if !exists {
		return nil
	}
	if err := validateActorUserSettings(value); err != nil {
		return err
	}
	return a.writeActorUserBackgroundTx(ctx, tx, actorID, fluctlightID, "initialization:"+fluctlightID+":actor_user", "assert", "Owner-approved initialization of actor_user background", mapValue(value))
}

// UpdateActorUserBackground is one owner-authorized, CAS-protected transaction.
// The command ledger is recovery/audit only; facts are the single authority.
func (a *App) UpdateActorUserBackground(ctx context.Context, actorID, fluctlightID string, body map[string]any) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fluctlightID, actorID); err != nil {
		return nil, err
	}
	allowed := map[string]bool{"background": true, "operation": true, "reason": true, "idempotency_key": true, "expected_current_facts_revision": true}
	for key := range body {
		if !allowed[key] {
			return nil, ErrInvalidArguments
		}
	}
	settings := map[string]any{"background": body["background"]}
	if err := validateActorUserSettings(settings); err != nil {
		return nil, err
	}
	transition, key, reason := stringValue(body["operation"]), strings.TrimSpace(stringValue(body["idempotency_key"])), strings.TrimSpace(stringValue(body["reason"]))
	if (transition != "correct" && transition != "change") || key == "" || len(key) > 256 || reason == "" || len([]rune(reason)) > 500 || stringValue(body["expected_current_facts_revision"]) == "" {
		return nil, ErrInvalidArguments
	}
	digest := stableDigest(jsonString(body))
	var result map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('fluctlight_lifecycle:' || $1))`, fluctlightID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "actor-facts:"+fluctlightID); err != nil {
			return err
		}
		var savedDigest string
		var raw []byte
		err := tx.QueryRow(ctx, `SELECT request_digest,result FROM public.actor_user_background_commands WHERE fluctlight_id=$1 AND idempotency_key=$2`, fluctlightID, key).Scan(&savedDigest, &raw)
		if err == nil {
			if savedDigest != digest {
				return ErrConflict
			}
			result = decodeObject(raw)
			result["replayed"] = true
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err := a.requireCurrentFactsRevisionTx(ctx, tx, fluctlightID, stringValue(body["expected_current_facts_revision"])); err != nil {
			return err
		}
		if err := a.writeActorUserBackgroundTx(ctx, tx, actorID, fluctlightID, "owner-background:"+stableDigest(fluctlightID+":"+key), transition, reason, settings); err != nil {
			return err
		}
		result, err = a.readActorUserBackground(ctx, tx, actorID, fluctlightID, a.now().UTC())
		if err != nil {
			return err
		}
		revision, err := readCurrentFactsRevisionWith(ctx, tx, fluctlightID)
		if err != nil {
			return err
		}
		result["current_facts_revision"] = revision
		result["replayed"] = false
		_, err = tx.Exec(ctx, `INSERT INTO public.actor_user_background_commands(fluctlight_id,idempotency_key,actor_id,request_digest,result) VALUES($1,$2,$3,$4,$5)`, fluctlightID, key, actorID, digest, jsonBytes(result))
		return err
	})
	return result, err
}

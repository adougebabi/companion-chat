package core

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func (a *App) runNPCOpportunityTask(ctx context.Context, owner, fid, inbox string, p map[string]any) (ProjectionTaskResult, error) {
	registry, err := NewCapabilityRegistry(newNPCEncounterCapability(a, stringValue(p["id"])))
	if err != nil {
		return ProjectionTaskResult{}, err
	}
	ctx = withCapabilityRunRegistry(ctx, registry)
	request := ContextProjectionRequest{AuthorizationActorID: owner, FluctlightID: fid, SourceFactID: inbox, TriggerSource: npcOpportunityFactType, TargetActorID: stringValue(mapValue(p["npc"])["actor_id"]), MemoryOperation: MemoryForNativeCognition, MemoryConversationMode: MemoryConversationGlobalOnly}
	projection, err := a.BuildContextProjectionFor(ctx, request)
	if err != nil {
		return ProjectionTaskResult{}, err
	}
	rules := []string{providerContextAuthorityRule, "Assess this actual virtual-world co-presence opportunity as the character. The Owner is not physically present. A routine prediction alone never creates an encounter; Core supplied executed NPC presence and a confirmed event or actually started effective character activity. You may pass or defer without acquaintance. For contact, choose only your own approach. The Tool resolves this NPC response independently from its private definition; you cannot choose or supply its response. Do not invent quoted dialogue, promises, biography knowledge, purchases or intimacy. Use only world.npc.encounter with the exact opportunity_id and revision. Do not pass npc_response; it is a server-owned prepared world result. Only an actual contact+accept Tool receipt establishes acquaintance; pass and declined must not be described as friendship. Consume the committed receipt before final settlement. Never repeat committed effects. Public reason only, no private reasoning."}
	input := jsonString(compactStateMap(p, []string{"id", "revision", "npc", "place_name", "place_id", "source_event_id", "character_presence_source", "source_activity_id", "npc_presence_id", "created_at", "expires_at", "profile_id"}))
	schema := objectSchema(map[string]any{"decision": enumStringSchema("settled", "deferred"), "reason": map[string]any{"type": "string", "minLength": 1, "maxLength": 500}}, []string{"decision", "reason"}, false)
	defs := capabilityCatalog(registry, CapabilitySurfaceNativeCognition)
	assembly, projection, err := a.assembleProjectionPromptForSurface(ctx, ProviderContextSurfaceNativeCognition, projection, "cognitive_assessment", rules, input, defs, "npc_opportunity_decision", schema)
	if err != nil {
		return ProjectionTaskResult{}, err
	}
	ctx = a.bindProjectionRefresh(ctx, request, ProviderContextSurfaceNativeCognition, "cognitive_assessment", rules, input, defs, "npc_opportunity_decision", schema, nil)
	scope := formalTaskCapabilityRequest(FormalAgentNativeCognition, projection, CapabilitySurfaceNativeCognition)
	run, err := a.runFormalStructuredTask(ctx, FormalAgentNativeCognition, assembly.Messages, defs, "npc_opportunity_decision", schema, true, scope)
	return ProjectionTaskResult{Trace: run.Trace, Completion: run.Completion, Projection: projection, Diagnostics: assembly.Diagnostics}, err
}

func (a *App) processNPCOpportunity(ctx context.Context, inbox string, fact map[string]any) (map[string]any, error) {
	id := stringValue(mapValue(fact["candidate"])["opportunity_id"])
	fid := stringValue(fact["fluctlight_id"])
	if id == "" || fid == "" {
		return nil, ErrInvalidArguments
	}
	var owner string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT owner_actor_id FROM public.world_npc_opportunities WHERE id=$1 AND fluctlight_id=$2`, id, fid).Scan(&owner); err != nil {
		return nil, err
	}
	var prepared map[string]any
	var prior map[string]any
	policyBlocked := false
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := lockLifeContextTx(ctx, tx, fid); err != nil {
			return err
		}
		p, err := readNPCOpportunityWith(ctx, tx, owner, fid, id, true)
		if err != nil {
			return err
		}
		prepared = p
		if p["source_fact_id"] != inbox || p["status"] != "pending" {
			prior = mapValue(p["result"])
			if len(prior) == 0 {
				prior = map[string]any{"opportunity_id": id, "status": p["status"], "reason": "superseded_fact"}
			}
			return settleOwnerLifeInboxTx(ctx, tx, inbox, prior)
		}
		if _, err := a.validateNPCOpportunityTx(ctx, tx, p, a.now().UTC()); err != nil {
			if !errors.Is(err, ErrConflict) {
				return err
			}
			prior = map[string]any{"opportunity_id": id, "status": "expired", "reason": "actual_presence_changed", "acquaintance_created": false}
			if _, err := tx.Exec(ctx, `UPDATE public.world_npc_opportunities SET status='expired',result=$2,error_code='npc_opportunity_expired',revision=revision+1,updated_at=now() WHERE id=$1`, id, jsonBytes(prior)); err != nil {
				return err
			}
			return settleOwnerLifeInboxTx(ctx, tx, inbox, prior)
		}
		policy, err := a.evaluateAutonomyPolicyWithReader(ctx, tx, fid, "capability", a.now(), "", false)
		if err != nil {
			return err
		}
		policyBlocked = !policy.Allowed
		return nil
	})
	if err != nil {
		return nil, err
	}
	if prior != nil {
		return prior, nil
	}
	decision := map[string]any{}
	var runErr error
	if !policyBlocked {
		task, err := a.runNPCOpportunityTask(ctx, owner, fid, inbox, prepared)
		runErr = err
		decision = task.Completion.Structured
	}
	settleCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	var result map[string]any
	err = withTransaction(settleCtx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := lockLifeContextTx(settleCtx, tx, fid); err != nil {
			return err
		}
		p, err := readNPCOpportunityWith(settleCtx, tx, owner, fid, id, true)
		if err != nil {
			return err
		}
		if p["source_fact_id"] != inbox || p["status"] != "pending" {
			result = mapValue(p["result"])
			if len(result) == 0 {
				result = map[string]any{"opportunity_id": id, "status": p["status"]}
			}
			return settleOwnerLifeInboxTx(settleCtx, tx, inbox, result)
		}
		status, code, reason := "failed", "npc_opportunity_not_settled", "No actual decision receipt was committed"
		if _, err := a.validateNPCOpportunityTx(settleCtx, tx, p, a.now().UTC()); err != nil {
			if !errors.Is(err, ErrConflict) {
				return err
			}
			status, code, reason = "expired", "npc_opportunity_expired", "Actual shared presence no longer holds"
		} else if runErr != nil {
			code = "npc_opportunity_assessment_failed"
		} else if policyBlocked {
			status, code, reason = "deferred", "npc_opportunity_policy_deferred", "Current autonomy policy blocks this automatic encounter"
		} else if decision["decision"] == "deferred" {
			status, code, reason = "deferred", "", stringValue(decision["reason"])
		}
		result = map[string]any{"opportunity_id": id, "status": status, "error_code": nilIfEmpty(code), "reason": reason, "source_fact_id": inbox, "acquaintance_created": false}
		if _, err := tx.Exec(settleCtx, `UPDATE public.world_npc_opportunities SET status=$2,result=$3,error_code=$4,available_at=$5,revision=revision+1,updated_at=now() WHERE id=$1`, id, status, jsonBytes(result), nilIfEmpty(code), a.now().Add(5*time.Minute)); err != nil {
			return err
		}
		return settleOwnerLifeInboxTx(settleCtx, tx, inbox, result)
	})
	return result, err
}

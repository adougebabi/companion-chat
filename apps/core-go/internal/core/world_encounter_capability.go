package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

const npcEncounterCapabilityName = "world.npc.encounter"

type npcEncounterCapability struct {
	service *npcEncounterService
	boundID string
}

func npcEncounterDefinition() CapabilityDefinition {
	return CapabilityDefinition{Name: npcEncounterCapabilityName, Version: "v1", Type: CapabilityTypeAction,
		Description: "Settle one actual co-presence opportunity. contact resolves the NPC response independently; pass records no encounter. Rechecks current world, profile, NPC and both presence sources. No Owner presence, movement or biography import.",
		Surfaces:    []CapabilitySurface{CapabilitySurfaceNativeCognition}, FailurePolicy: FailurePolicyOptionalInternal,
		InputSchema:  objectSchema(map[string]any{"opportunity_id": stringSchema(), "expected_revision": integerSchema(), "decision": enumStringSchema("contact", "pass"), "reason": map[string]any{"type": "string", "minLength": 1, "maxLength": 500}}, []string{"opportunity_id", "expected_revision", "decision", "reason"}, false),
		OutputSchema: openObjectSchema(), SideEffectClass: "native_projection", SuccessBoundary: "world_encounter_decision_committed", ConcurrencyClass: "exclusive", SupportsRetry: true}
}
func (c npcEncounterCapability) Definition() CapabilityDefinition { return npcEncounterDefinition() }
func (c npcEncounterCapability) RequiredContext() []ContextSlot   { return nil }
func (c npcEncounterCapability) Execute(_ context.Context, i CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return executeToolRequired(i)
}
func (c npcEncounterCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, i CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	if c.service == nil {
		return failedCapabilityResult(i, "npc_runtime_unavailable", true), ErrCapabilityNotFound
	}
	args, err := capabilityExecutionArguments(i, c.Definition())
	if err != nil {
		return failedCapabilityResult(i, "invalid_arguments", false), err
	}
	id := stringValue(args["opportunity_id"])
	owner, fid := i.Metadata.AuthorizationActorID, i.Metadata.FluctlightID
	if c.boundID != "" && id != c.boundID {
		return failedCapabilityResult(i, "npc_opportunity_target_mismatch", false), ErrUnauthorized
	}
	if err := lockLifeContextTx(ctx, tx, fid); err != nil {
		return failedCapabilityResult(i, "npc_opportunity_lock_failed", true), err
	}
	p, err := readNPCOpportunityWith(ctx, tx, owner, fid, id, true)
	if err != nil {
		return failedCapabilityResult(i, "npc_opportunity_unavailable", false), err
	}
	if p["status"] != "pending" {
		return handledCapabilityResult(i, "npc_opportunity_already_settled", "no_change", map[string]any{"detail": "Read the actual opportunity result; this call created no new acquaintance."}), nil
	}
	expected, ok := parseRevision(args["expected_revision"])
	if !ok || expected != intValue(p["revision"]) {
		return handledCapabilityResult(i, "npc_opportunity_revision_conflict", "awaiting_state", map[string]any{"detail": "The captured opportunity version changed; no encounter was executed."}), nil
	}
	if c.boundID != "" && p["source_fact_id"] != i.SourceFactID {
		return failedCapabilityResult(i, "npc_opportunity_fact_mismatch", false), ErrUnauthorized
	}
	at := c.service.clock().UTC()
	life, err := c.service.validate(ctx, tx, p, at)
	if err != nil {
		if errors.Is(err, ErrConflict) {
			return handledCapabilityResult(i, "npc_opportunity_expired", "awaiting_state", map[string]any{"detail": "The actual shared presence no longer holds; no encounter was executed."}), nil
		}
		return failedCapabilityResult(i, "npc_presence_read_failed", true), err
	}
	if profile := i.Metadata.WorkingProfileID; profile != "" && profile != p["profile_id"] {
		return failedCapabilityResult(i, "npc_opportunity_profile_mismatch", false), ErrUnauthorized
	}
	decision := stringValue(args["decision"])
	response := ""
	if decision == "contact" {
		raw, found, err := capabilityPreparedData(i, "npc_response")
		prepared := mapValue(raw)
		if err != nil || !found {
			return failedCapabilityResult(i, "npc_response_missing", false), ErrInvalidArguments
		}
		if prepared["state"] != "ready" || mapValue(p["npc_response"])["request_key"] != prepared["request_key"] || mapValue(p["npc_response"])["decision"] != prepared["decision"] || intValue(prepared["opportunity_revision"]) != intValue(p["revision"]) || intValue(prepared["npc_revision"]) != intValue(p["npc_revision"]) {
			return handledCapabilityResult(i, "npc_opportunity_expired", "awaiting_state", map[string]any{"detail": "The independently resolved source changed; no encounter was committed."}), nil
		}
		response = stringValue(prepared["decision"])
		if response != "accept" && response != "decline" {
			return failedCapabilityResult(i, "npc_response_invalid", false), ErrInvalidArguments
		}
	}

	result := map[string]any{"opportunity_id": id, "status": "passed", "reason": args["reason"], "acquaintance_created": false, "source_fact_id": i.SourceFactID}
	if decision == "contact" {
		start, e := time.Parse(time.RFC3339Nano, stringValue(p["created_at"]))
		if e != nil {
			return failedCapabilityResult(i, "npc_opportunity_time_invalid", false), e
		}
		if !at.After(start) {
			return handledCapabilityResult(i, "npc_opportunity_not_elapsed", "awaiting_state", map[string]any{"detail": "The observation interval has not elapsed; no historical duration was invented."}), nil
		}
		var actor, name string
		if err := tx.QueryRow(ctx, `SELECT actor_id,name FROM public.world_npcs WHERE id=$1 AND world_id=$2 FOR UPDATE`, p["npc_id"], p["world_id"]).Scan(&actor, &name); err != nil {
			return failedCapabilityResult(i, "npc_identity_read_failed", true), err
		}
		summary := fmt.Sprintf("在%s尝试与附近的人交流，对方没有接受这次接触。", p["place_name"])
		if response == "accept" {
			summary = fmt.Sprintf("在%s与%s相遇，互通姓名并进行了简短交流。", p["place_name"], name)
			if mapValue(p["npc"])["known"] == true {
				summary = fmt.Sprintf("在%s与%s进行了简短交流。", p["place_name"], name)
			}
		}
		result, err = c.service.commit(ctx, tx, npcEncounterCommit{owner: owner, fid: fid, world: stringValue(p["world_id"]), npc: stringValue(p["npc_id"]), actor: actor, name: name, place: stringValue(p["place_id"]), placeName: stringValue(p["place_name"]), summary: summary, refs: npcEncounterEvidenceRefs(p), start: start, end: at, lifeRevision: stringValue(life["context_revision"]), key: "opportunity:" + id, reason: "虚拟世界执行结果；角色和NPC的实际同地来源经Tool复核", attribution: npcEncounterAttribution{Kind: "virtual_world_encounter", ActorID: fid, SourceFactID: i.SourceFactID, OpportunityID: id, Direct: false, Establish: response == "accept"}})
		if err != nil {
			return failedCapabilityResult(i, "npc_encounter_write_failed", true), err
		}
		result["opportunity_id"], result["status"], result["acquaintance_created"], result["reason"], result["source_fact_id"] = id, map[bool]string{true: "contacted", false: "declined"}[response == "accept"], response == "accept", args["reason"], i.SourceFactID
	}
	if _, err := tx.Exec(ctx, `UPDATE public.world_npc_opportunities SET status=$2,result=$3,error_code=NULL,revision=revision+1,updated_at=now() WHERE id=$1`, id, result["status"], jsonBytes(result)); err != nil {
		return failedCapabilityResult(i, "npc_opportunity_write_failed", true), err
	}
	return CapabilityResult{CallID: i.CallID, CapabilityName: i.CapabilityName, Status: "completed", Output: result, ProviderRequestID: i.ProviderRequestID, CorrelationID: "npc-opportunity:" + id}, nil
}

func npcEncounterEvidenceRefs(p map[string]any) []any {
	refs := []any{p["source_event_id"], p["npc_presence_id"], p["id"]}
	if id := stringValue(p["source_activity_id"]); id != "" {
		refs = append(refs, "activity:"+id)
	}
	return refs
}

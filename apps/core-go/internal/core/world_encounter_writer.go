package core

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
)

type npcEncounterAttribution struct {
	Kind, ActorID, SourceFactID, OpportunityID string
	Direct, Establish                          bool
}

func (a *App) commitNPCEncounterTx(ctx context.Context, tx pgx.Tx, owner, fid, world, id, actor, name, place, placeName, summary string, refs []any, start, end time.Time, lifeRevision, key, reason string, attribution npcEncounterAttribution) (map[string]any, error) {
	var err error
	encounterID := randomID("encounter_")
	provenance := map[string]any{"source": attribution.Kind, "actor_id": attribution.ActorID, "evidence_refs": refs, "encounter_id": encounterID, "opportunity_id": nilIfEmpty(attribution.OpportunityID), "participants": []string{fid, actor}, "owner_copresent": false, "outcome": map[bool]string{true: "contacted", false: "declined"}[attribution.Establish]}
	eventResult, err := a.createLifeEventTx(context.WithValue(ctx, lifeEventSourceKey{}, attribution.SourceFactID), tx, attribution.ActorID, fid, map[string]any{
		"kind": "npc_encounter", "start_at": start.Format(time.RFC3339Nano), "end_at": end.Format(time.RFC3339Nano),
		"scene": "social_contact", "activity": summary, "location": placeName, "place_id": place, "evidence_refs": refs,
		"expected_life_context_revision": lifeRevision, "idempotency_key": "npc-encounter:" + key,
	})
	if err != nil {
		return nil, err
	}
	eventID := stringValue(eventResult["id"])
	eventResult["npc_id"], eventResult["npc_actor_id"], eventResult["place_id"], eventResult["encounter_id"], eventResult["provenance"] = id, actor, place, encounterID, provenance
	if _, err := tx.Exec(ctx, `UPDATE public.life_events SET result=$2 WHERE id=$1`, eventID, jsonBytes(eventResult)); err != nil {
		return nil, err
	}
	var relationshipID string
	if attribution.Establish {
		profile, e := (&intentionService{}).resolveProfile(ctx, tx, fid, "")
		if e != nil {
			return nil, e
		}
		err = tx.QueryRow(ctx, `SELECT id FROM public.relationships WHERE owner_fluctlight_id=$1 AND target_actor_id=$2 AND (profile_id IS NULL OR profile_id=$3) ORDER BY CASE WHEN profile_id=$3 THEN 0 ELSE 1 END,updated_at DESC LIMIT 1 FOR UPDATE`, fid, actor, profile).Scan(&relationshipID)
		if errors.Is(err, pgx.ErrNoRows) {
			relationshipID = randomID("relationship_")
			role := map[string]any{"label": "acquaintance"}
			if _, err = tx.Exec(ctx, `INSERT INTO public.relationships(id,owner_fluctlight_id,profile_id,target_actor_id,role,metrics,trend,summary,emotional_association,provenance,revision) VALUES($1,$2,NULL,$3,$4,'{}','stable',$5,'{}',$6,1)`, relationshipID, fid, actor, jsonBytes(role), summary, jsonBytes(provenance)); err != nil {
				return nil, err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO public.relationship_revisions(id,relationship_id,revision,base_revision,role,metrics,trend,summary,emotional_association,evidence_refs,actor_id,idempotency_key) VALUES($1,$2,1,0,$3,'{}','stable',$4,'{}',$5,$6,$7)`, randomID("relationship_revision_"), relationshipID, jsonBytes(role), summary, jsonBytes([]any{eventID}), attribution.ActorID, "npc-relationship:"+key); err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		}
	}
	command := PreparedMemoryMutation{SchemaVersion: memoryLifecycleSchemaVersion, Operation: MemoryCreate, OwnerFluctlightID: fid, OwnerActorID: owner, ActorID: attribution.ActorID, ActorRefs: []string{fid, actor}, EventRefs: []string{eventID}, EvidenceRefs: []string{eventID}, Semantic: &MemorySemanticInput{Type: "episodic", Content: summary, Confidence: 1, Importance: 0.5}, Visibility: "private", OccurredAt: end, SourceFactID: eventID, AuthenticatedDirect: attribution.Direct, SemanticReason: reason, IdempotencyKey: "npc-memory:" + key}
	command.RequestDigest = memoryCommandDigest(command)
	memory, err := a.applyMemoryCommandTx(ctx, tx, command)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.world_encounters(id,world_id,npc_id,place_id,life_event_id,relationship_id,memory_id,owner_actor_id,summary,evidence_refs,occurred_at,source_kind,opportunity_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, encounterID, world, id, place, eventID, nullableString(relationshipID), memory.MemoryID, owner, summary, jsonBytes(refs), end, attribution.Kind, nullableString(attribution.OpportunityID)); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE public.world_npcs SET confirmed_place_id=$2,confirmed_at=$3,revision=revision+CASE WHEN $4 THEN 1 ELSE 0 END,updated_at=now() WHERE id=$1 AND (confirmed_at IS NULL OR confirmed_at<=$3)`, id, place, end, attribution.Direct); err != nil {
		return nil, err
	}
	if attribution.Establish {
		if err := a.recordRelationshipInteractionTx(ctx, tx, fid, actor, true, ""); err != nil {
			return nil, err
		}
	}
	if err := appendOutboxTx(ctx, tx, "world.npc.encountered", "world", world, fid, eventID, encounterID, "world-encounter:"+key, eventResult); err != nil {
		return nil, err
	}
	return map[string]any{"id": encounterID, "event_id": eventID, "npc_id": id, "npc_name": func() any {
		if attribution.Establish {
			return name
		}
		return nil
	}(), "relationship_id": relationshipID, "memory_id": memory.MemoryID, "status": "recorded", "source": attribution.Kind, "outcome": map[bool]string{true: "contacted", false: "declined"}[attribution.Establish]}, nil
}

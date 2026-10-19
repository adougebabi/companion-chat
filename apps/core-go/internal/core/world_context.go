package core

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// Runtime world settings are environment context. NPC private definitions and
// catalogue browsing never become the character's acquaintance or memories.
func (a *App) worldEnvironmentContext(ctx context.Context, fid, profile string) (map[string]any, error) {
	var raw []byte
	var worldID string
	err := a.DB.Pool().QueryRow(ctx, `SELECT id,jsonb_build_object('name',name,'background',background,'rules',rules,'description',left(description,4000),'flavor',flavor,'source','owner_world_setting','revision',revision) FROM public.life_worlds WHERE id=(SELECT world_id FROM public.fluctlight_world_bindings WHERE fluctlight_id=$1)`, fid).Scan(&worldID, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	result := decodeObject(raw)
	known, err := readWorldObjects(ctx, a.DB.Pool(), `SELECT DISTINCT ON(n.id) jsonb_build_object('name',n.name,'actor_id',n.actor_id,'relationship_summary',left(r.summary,500),'source','recorded_relationship') FROM public.world_npcs n JOIN public.relationships r ON r.target_actor_id=n.actor_id JOIN public.life_worlds w ON w.id=n.world_id AND r.owner_fluctlight_id=w.fluctlight_id WHERE w.id=$1 AND n.status='active' AND n.enabled AND (r.profile_id IS NULL OR r.profile_id=$2) ORDER BY n.id,r.updated_at DESC LIMIT 16`, worldID, profile)
	if err != nil {
		return nil, err
	}
	_, life, err := a.readLifeContextSnapshotAt(ctx, fid, a.now())
	if err != nil {
		return nil, err
	}
	if place := stringValue(life["place_id"]); place != "" {
		places, err := readWorldObjects(ctx, a.DB.Pool(), `SELECT jsonb_build_object('id',p.id,'name',p.name,'description',left(p.description,1000),'atmosphere',p.details->'atmosphere','source',$3::text,'authority_status',$4::text) FROM public.world_places p WHERE p.id=$1 AND p.world_id=$2 AND p.status='active'`, place, worldID, life["source"], life["authority_status"])
		if err != nil {
			return nil, err
		}
		if len(places) > 0 {
			result["current_place"] = places[0]
		}
	}
	result["active_world_effects"] = life["world_effects"]
	characterSource, err := npcCharacterPresenceWith(ctx, a.DB.Pool(), fid, life, a.now().UTC())
	if err != nil {
		return nil, err
	}
	if characterSource != nil {
		nearby, err := readWorldObjects(ctx, a.DB.Pool(), `SELECT jsonb_build_object('actor_id',n.actor_id,'public_role',n.role,'known',EXISTS(SELECT 1 FROM public.relationships r WHERE r.owner_fluctlight_id=$2 AND r.target_actor_id=n.actor_id AND (r.profile_id IS NULL OR r.profile_id=$3)),'presence_source',p.source_kind,'presence_id',p.id,'knowledge','public_presence_only') FROM public.world_npc_effective_presence_sources p JOIN public.world_npcs n ON n.id=p.npc_id AND n.world_id=p.world_id WHERE p.world_id=$1 AND p.place_id=$4 AND p.status='active' AND p.start_at<=$5 AND p.end_at>$5 AND p.definition_revision=n.revision AND n.enabled AND n.status='active' ORDER BY p.start_at,p.id LIMIT 4`, worldID, fid, profile, life["place_id"], a.now().UTC())
		if err != nil {
			return nil, err
		}
		if len(nearby) > 0 {
			result["nearby_public_npcs"] = nearby
		}
		opportunities, err := readWorldObjects(ctx, a.DB.Pool(), `SELECT jsonb_build_object('opportunity_id',o.id,'revision',o.revision,'npc_actor_id',n.actor_id,'expires_at',o.expires_at,'status','opportunity_not_interaction') FROM public.world_npc_opportunities o JOIN public.world_npcs n ON n.id=o.npc_id AND n.revision=o.npc_revision JOIN public.world_npc_effective_presence_sources p ON p.id=o.npc_presence_id AND p.status='active' AND p.end_at>$6 WHERE o.fluctlight_id=$1 AND o.world_id=$2 AND o.profile_id=$3 AND o.place_id=$4 AND o.source_event_id=$5 AND o.status='pending' AND o.expires_at>$6 ORDER BY o.created_at,o.id LIMIT 4`, fid, worldID, profile, life["place_id"], life["event_id"], a.now().UTC())
		if err != nil {
			return nil, err
		}
		if len(opportunities) > 0 {
			result["npc_opportunities"] = opportunities
		}
	}
	if len(known) > 0 {
		result["known_npcs"] = known
	}
	return result, nil
}

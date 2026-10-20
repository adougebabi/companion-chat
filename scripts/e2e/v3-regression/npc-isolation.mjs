import {check,equal} from './runtime.mjs';
import {fixture,fidPath,resourcePath,ensureWorld,apply} from './domain.mjs';

// A blank-slate fixture deliberately has no world. Configure and read both
// bindings before testing NPC scope, otherwise a 422 tests a missing prerequisite.
export async function verifyNPCWorldIsolation(t,{fid,worldId,npcId}){
 const primaryPath=fidPath(fid),primary=await t.get(primaryPath+`/npcs/${encodeURIComponent(npcId)}`);
 equal(primary.world_id,worldId,'The source NPC must belong to the selected primary world');
 check(primary.actor_id&&primary.source_resource_id&&typeof primary.source_version==='string'&&primary.source_version.trim()!=='','NPC isolation requires an actual Actor and exact published definition source',undefined,primary);
 const secondary=await fixture(t,'secondary');
 check(secondary.id!==fid,'NPC isolation requires two different characters',undefined,{primary:fid,secondary:secondary.id});
 const secondaryPath=fidPath(secondary.id),world=await ensureWorld(t,secondary.id,'secondary-npc-world');
 check(world.id!==worldId,'NPC isolation requires two independent runtime worlds',undefined,{primary:worldId,secondary:world.id});
 for(const [path,expectedId,expectedWorld] of [[primaryPath,fid,worldId],[secondaryPath,secondary.id,world.id]]){
  const catalog=await t.get(path+'/world/catalog'),snapshot=await t.get(path+'/world');
  equal(catalog.active_binding?.world_id,expectedWorld,'Each character must have its own persisted active world binding before the scope check');
  equal(snapshot.world?.id,expectedWorld,'Selected world must read back before the scope check');
  equal(snapshot.world?.fluctlight_id,expectedId,'Runtime world ownership must match the URL character');
 }
 const resource=await t.get(resourcePath(primary.source_resource_id));
 const applied=await apply(t,`secondary-npc-${npcId}`,secondary.id,{...resource,published_version:primary.source_version});
 const other=await t.get(secondaryPath+`/npcs/${encodeURIComponent(applied.id)}`);
 check(other.id&&other.id!==primary.id&&other.actor_id&&other.actor_id!==primary.actor_id,'Applying the same definition in another world must create independent NPC and Actor identities',undefined,{primary:{id:primary.id,actorId:primary.actor_id},secondary:{id:other.id,actorId:other.actor_id}});
 equal(other.world_id,world.id,'The second NPC must belong to the second selected world');
 equal(other.source_resource_id,primary.source_resource_id,'Both NPCs must use the same published definition');
 equal(other.source_version,primary.source_version,'Both NPCs must use the same exact definition version');
 const encounters=await t.pages(secondaryPath+`/npcs/${encodeURIComponent(other.id)}/encounters?limit=100`);
 equal(encounters,[],'Applying the definition cannot copy encounters from the primary NPC');
 const detail=await t.get(secondaryPath+'/detail');
 check(Array.isArray(detail.relationships)&&Array.isArray(detail.memories),'Isolation requires actual relationship and memory readback');
 check(!detail.relationships.some(r=>[primary.actor_id,other.actor_id].includes(r.target_actor_id)),'Applying a definition cannot copy or establish acquaintance');
 check(!detail.memories.some(m=>[primary.actor_id,other.actor_id].some(id=>(m.actor_refs||[]).includes(id))),'Applying a definition cannot copy NPC interaction memories');
 await t.request(secondaryPath+`/npcs/${encodeURIComponent(primary.id)}`,{expected:[403,404]});
 await t.request(primaryPath+`/npcs/${encodeURIComponent(other.id)}`,{expected:[403,404]});
 return {primary:{fid,worldId,npcId:primary.id,actorId:primary.actor_id},secondary:{fid:secondary.id,worldId:world.id,npcId:other.id,actorId:other.actor_id},resourceId:primary.source_resource_id,resourceVersion:primary.source_version};
}

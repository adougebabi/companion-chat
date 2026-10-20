import {persistentCommand} from './providers/persistent-command.mjs';

// Activation is already committed. These commands resume independently and
// never create a second character when publication/application/selection fails.
export async function finishCreationWorld({client,recovery,storage,owner}){
 let draft=recovery.read();
 if(!draft.worldResourceId||!draft.worldResourceVersion)return {configured:false,warning:'尚未选择世界底稿，可在初始化向导继续补全'};
 const fid=draft.activatedId,source={resourceId:draft.worldResourceId,version:draft.worldResourceVersion};
 if(!fid)throw new Error('人物尚未激活，世界底稿已保留');
 const command=persistentCommand({key:`fluctlight-creation:${owner}:${fid}:world-command`,storage:()=>storage,execute:async(value,key)=>{
  const result=value.operation==='apply'?await client.applyStudioResource(value.resourceId,{...value.body,idempotencyKey:key}):await client.selectWorld(fid,{...value.body,idempotencyKey:key});
  const saved=recovery.read();
  if(saved.activatedId===fid&&saved.worldResourceId===source.resourceId&&saved.worldResourceVersion===source.version){
   if(value.operation==='apply'){if(!result.applied||!result.id)throw new Error('世界应用尚未确认，原请求已保留');saved.worldRuntimeId=result.id;}
   else saved.worldSelectionConfirmed=true;
   recovery.save(saved);
  }
  return result;
 }});
 if(command.pending())await command.retry();
 draft=recovery.read();
 if(!draft.worldRuntimeId){
  const catalog=await client.worldCatalog(fid),existing=catalog.items.find(w=>w.source_resource_id===source.resourceId&&w.source_version===source.version);
  if(existing){draft.worldRuntimeId=existing.id;recovery.save(draft);}
  else {const preview=await client.previewStudioApply(source.resourceId,{fluctlightId:fid,resourceVersion:source.version});await command.submit({operation:'apply',resourceId:source.resourceId,body:{fluctlightId:fid,resourceVersion:source.version,expectedInstanceRevision:preview.instance_revision,expectedRuntimeRevision:preview.runtime_revision??0,choices:(preview.diff||[]).map(d=>({field:d.field,decision:'keep'})),reason:'初始化人物的独立运行世界'}});}
 }
 draft=recovery.read();
 const catalog=await client.worldCatalog(fid);
 if(catalog.active_binding?.world_id!==draft.worldRuntimeId){
  if(draft.worldSelectionConfirmed)return {configured:false,warning:'人物后来选择了其他世界，当前选择已保留；可在世界管理核对'};
  await command.submit({operation:'select',body:{worldId:draft.worldRuntimeId,expectedRevision:catalog.active_binding?.revision??0,reason:'初始化后明确选用人物的独立世界'}});
 }
 const actual=await client.world(fid);
 if(actual.world?.id!==recovery.read().worldRuntimeId)throw new Error('人物已经创建，当前世界读回仍待确认；可继续使用人物并稍后重试世界步骤');
 return {configured:true,worldId:actual.world.id};
}

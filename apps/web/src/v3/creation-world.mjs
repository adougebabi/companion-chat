import {persistentCommand} from './providers/persistent-command.mjs';
import {finishReusedWorld} from './reuse-world.mjs';

// Activation is already committed. These commands resume independently and
// never create a second character when publication/application/selection fails.
export async function finishCreationWorld({client,recovery,storage,owner,ownerCurrent=()=>true}){
 let draft=recovery.read();
 if(draft.worldSource)return finishReusedWorld({client,recovery,storage,owner,ownerCurrent});
 if(draft.useDefaultWorld)return finishDefaultCreationWorld({client,recovery,storage,owner,ownerCurrent});
 if(!draft.worldResourceId||!draft.worldResourceVersion)return {configured:false,warning:'尚未选择世界底稿，可在初始化向导继续补全'};
 const fid=draft.activatedId,requestId=draft.requestId,source={resourceId:draft.worldResourceId,version:draft.worldResourceVersion};
 if(!fid)throw new Error('人物尚未激活，世界底稿已保留');
 const current=()=>{if(!ownerCurrent())return null;const value=recovery.read();return value.requestId===requestId&&value.activatedId===fid&&!value.useDefaultWorld&&value.worldResourceId===source.resourceId&&value.worldResourceVersion===source.version?value:null;};
 const stale={configured:false,stale:true};
 if(!current())return stale;
 const command=persistentCommand({key:`fluctlight-creation:${owner}:${fid}:world-command`,storage:()=>storage,execute:async(value,key)=>{
  const result=value.operation==='apply'?await client.applyStudioResource(value.resourceId,{...value.body,idempotencyKey:key}):await client.selectWorld(fid,{...value.body,idempotencyKey:key});
  const saved=current();
  if(saved){
   if(value.operation==='apply'){if(!result.applied||!result.id)throw new Error('世界应用尚未确认，原请求已保留');saved.worldRuntimeId=result.id;}
   else saved.worldSelectionConfirmed=true;
   recovery.save(saved);
  }
  return result;
 }});
 if(command.pending())await command.retry();
 draft=current();if(!draft)return stale;
 if(!draft.worldRuntimeId){
  const catalog=await client.worldCatalog(fid),existing=catalog.items.find(w=>w.source_resource_id===source.resourceId&&w.source_version===source.version);
  draft=current();if(!draft)return stale;
  if(existing){draft.worldRuntimeId=existing.id;recovery.save(draft);}
  else {const preview=await client.previewStudioApply(source.resourceId,{fluctlightId:fid,resourceVersion:source.version});if(!current())return stale;await command.submit({operation:'apply',resourceId:source.resourceId,body:{fluctlightId:fid,resourceVersion:source.version,expectedInstanceRevision:preview.instance_revision,expectedRuntimeRevision:preview.runtime_revision??0,choices:(preview.diff||[]).map(d=>({field:d.field,decision:'keep'})),reason:'初始化人物的独立运行世界'}});}
 }
 if(!current())return stale;
 const catalog=await client.worldCatalog(fid);
 draft=current();if(!draft)return stale;
 if(catalog.active_binding?.world_id!==draft.worldRuntimeId){
  if(draft.worldSelectionConfirmed)return {configured:false,warning:'人物后来选择了其他世界，当前选择已保留；可在世界管理核对'};
  await command.submit({operation:'select',body:{worldId:draft.worldRuntimeId,expectedRevision:catalog.active_binding?.revision??0,reason:'初始化后明确选用人物的独立世界'}});
 }
 if(!current())return stale;
 const actual=await client.world(fid);
 draft=current();if(!draft)return stale;
 if(actual.world?.id!==draft.worldRuntimeId)throw new Error('人物已经创建，当前世界读回仍待确认；可继续使用人物并稍后重试世界步骤');
 return {configured:true,worldId:actual.world.id};
}

// A new character can start with a minimal real world, independently from the
// advanced reusable-resource branch. No character/place facts are invented.
async function finishDefaultCreationWorld({client,recovery,storage,owner,ownerCurrent}){
 const first=recovery.read(),fid=first.activatedId,requestId=first.requestId;
 if(!fid)throw new Error('请先完成角色创建');
 const current=()=>{if(!ownerCurrent())return null;const value=recovery.read();return value.requestId===requestId&&value.activatedId===fid&&value.useDefaultWorld?value:null;};
 const stale={configured:false,stale:true};
 if(!current())return stale;
 const command=persistentCommand({key:`fluctlight-creation:${owner}:${fid}:default-world-command`,storage:()=>storage,execute:async(body,key)=>{
  const result=await client.updateRuntimeWorld(fid,{...body,idempotencyKey:key});
  if(!result?.id)throw new Error('生活世界的创建结果尚未确认');
  const saved=current();if(saved){saved.worldRuntimeId=result.id;recovery.save(saved);}return result;
 }});
 if(command.pending())await command.retry();
 if(!current())return stale;
 let actual=await client.world(fid),draft=current();
 if(!actual||actual.available===false||!Object.hasOwn(actual,'world')||actual.world!==null&&(!actual.world||typeof actual.world.id!=='string'||!actual.world.id.trim()))throw Error('生活世界读取结果不完整，请重新读取原创建步骤');
 if(!draft)return stale;
 if(!draft.worldRuntimeId&&actual.world?.id){draft.worldRuntimeId=actual.world.id;recovery.save(draft);}
 else if(!draft.worldRuntimeId){await command.submit({expectedRevision:0,name:'日常世界',background:'日常生活，具体地点和经历将在相处中逐渐补充。',reason:'为新角色建立默认生活世界'});if(!current())return stale;actual=await client.world(fid);}
 draft=current();if(!draft)return stale;
 if(actual.world?.id!==draft.worldRuntimeId)throw new Error('生活世界尚未读回，原角色和创建步骤已保留');
 draft.worldSelectionConfirmed=true;draft.worldName=actual.world.name||draft.worldName||'';recovery.save(draft);
 return {configured:true,worldId:draft.worldRuntimeId};
}

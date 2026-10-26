import {persistentCommand} from './providers/persistent-command.mjs';

// Explicit opt-in only. The unchanged default-world flow remains independent.
// Core copies saved world/place configuration; no NPCs, inventory or history.
export async function finishReusedWorld({client,recovery,storage,owner,ownerCurrent}){
 const initial=recovery.read(),fid=initial.activatedId,requestId=initial.requestId,source=initial.worldSource;
 if(!fid)throw Error('请先完成角色创建，原世界选择已保留');
 const current=()=>{if(!ownerCurrent())return null;const value=recovery.read();return value.requestId===requestId&&value.activatedId===fid&&JSON.stringify(value.worldSource)===JSON.stringify(source)?value:null;};
 const stale={configured:false,stale:true};
 if(!current())return stale;
 const command=persistentCommand({key:`fluctlight-creation:${owner}:${fid}:reused-world-command`,storage:()=>storage,execute:async(value,key)=>{
   const result=value.operation==='copy'?await client.createCatalogWorld(fid,{...value.body,idempotencyKey:key}):await client.selectWorld(fid,{...value.body,idempotencyKey:key});
   const saved=current();if(saved){if(value.operation==='copy'){if(!result.id)throw Error('世界配置复用结果尚未确认');saved.worldRuntimeId=result.id;}else saved.worldSelectionConfirmed=true;recovery.save(saved);}return result;
 }});
 if(command.pending())await command.retry();
 let draft=current();if(!draft)return stale;
 if(!draft.worldRuntimeId)await command.submit({operation:'copy',body:{name:source.name,sourceFluctlightId:source.fluctlightId,sourceWorldId:source.worldId,expectedSourceRevision:source.revision,reason:'复用已有世界与地点配置，保留独立生活记录'}});
 draft=current();if(!draft)return stale;
 const catalog=await client.worldCatalog(fid);draft=current();if(!draft)return stale;
 if(catalog.active_binding?.world_id!==draft.worldRuntimeId){
   if(draft.worldSelectionConfirmed)return {configured:false,warning:'角色后来选择了其他世界，请到世界页核对；复用结果已保留。'};
   await command.submit({operation:'select',body:{worldId:draft.worldRuntimeId,expectedRevision:catalog.active_binding?.revision||0,reason:'选用已复用的独立世界配置'}});
 }
 const actual=await client.world(fid);draft=current();if(!draft)return stale;
 if(actual.world?.id!==draft.worldRuntimeId)throw Error('世界选择尚未读回，原复用请求已保留');
 return {configured:true,worldId:draft.worldRuntimeId};
}

function reviewChannel({owner,storage},fid){return persistentCommand({key:`fluctlight-creation:${owner}:${fid}:reused-world-command`,storage:()=>storage,execute(){throw Error('此处只核对原提交，不执行写入');}});}
export async function reviewWorldReuse(options){
 const {client,recovery,ownerCurrent}=options,old=recovery.read(),channel=reviewChannel(options,old.activatedId),pending=channel.pending();
 if(!ownerCurrent()||!pending?.rejection||pending.value.operation!=='copy'||old.worldRuntimeId)throw Error('原复用提交尚未确认无效果，请先恢复或查询原提交。');
 const source=old.worldSource,view=await client.catalogWorld(source.fluctlightId,source.worldId);
 if(!ownerCurrent()||JSON.stringify(recovery.read())!==JSON.stringify(old))throw Error('创建资料已变化，请重新核对。');
 if(!view.world||!Number.isSafeInteger(view.world.revision))throw Error('原世界版本尚未读到，请重试。');
 return {requestId:old.requestId,fid:old.activatedId,idempotencyKey:pending.idempotencyKey,previous:source,source:{...source,name:view.world.name,revision:view.world.revision},preview:{名称:view.world.name,背景:view.world.background,规则:view.world.rules,地点:(view.places||[]).map(p=>({名称:p.alias||p.name,描述:p.desc||p.description,楼层:p.floor,陈设:p.furniture,布局:p.layout}))}};
}
export function confirmWorldReuse(options,review){
 const {recovery,ownerCurrent}=options,old=recovery.read(),channel=reviewChannel(options,old.activatedId),pending=channel.pending();
 if(!ownerCurrent()||old.requestId!==review.requestId||old.activatedId!==review.fid||old.worldRuntimeId||JSON.stringify(old.worldSource)!==JSON.stringify(review.previous)||pending?.idempotencyKey!==review.idempotencyKey)throw Error('原提交或创建资料已变化，请重新核对。');
 channel.releaseRejected();
 recovery.save({...old,worldSource:review.source,worldName:review.source.name});
}

import {studioClient} from '../services.js';
import {get} from '../store.js';
import {esc,toast} from '../ui.js';
import {persistentCommand} from '../../providers/persistent-command.mjs';
import {studioDraftStorage} from './studio-draft-storage.js';
import {mountWorldFrameworkGenerator} from './world-framework-generator.js';
import {WorldFrameworkPreview} from './world-venues.js';

export function mountRuntimeWorldFramework(box,{fid,world,alive=()=>box.isConnected,onApplied=()=>{}}){
 const actor=get('session')?.actorId,storage=studioDraftStorage(),key=`fluctlight-runtime-framework:${actor}:${fid}:${world.id}`;
 let active=true,busy=false,error='',generator=null,resource=null,state;
 try{state=JSON.parse(storage.getItem(key)||'null');}catch{error='原框架草稿暂不可读，请核对原生成任务。';}
 state||={resourceId:world.source_resource_id||'',framework:null};
 const save=()=>storage.setItem(key,JSON.stringify(state)),live=()=>active&&alive()&&get('session')?.actorId===actor;
 const body=()=>({background:world.background||world.description||'',rules:world.rules||'',description:world.description||'',flavor:world.flavor||'',...(state.framework?{framework:state.framework}:{})});
 const channel=persistentCommand({key:key+':command',storage:()=>storage,execute:async(value,idempotencyKey)=>{
  if(!live())throw new Error('身份或运行世界已变化，请返回原世界核对');
  const result=value.operation==='save'?await studioClient.save(value.id,{...value.body,idempotencyKey}):value.operation==='publish'?await studioClient.publish(value.id,value.revision,idempotencyKey):await studioClient.apply(value.id,{...value.body,idempotencyKey});
  if(value.operation==='save'){state.resourceId=result.id;save();}return result;
 }});
 async function ensureResource(draft){
  const current=state.resourceId?await studioClient.resource(state.resourceId):null;if(current?.body?.framework&&draft.body.framework===undefined)draft.body.framework=current.body.framework;if(current&&current.kind!=='world')throw new Error('世界底稿类型不匹配，请核对来源');
  if(!current||current.name!==draft.name||JSON.stringify(current.body)!==JSON.stringify(draft.body)){
   resource=await channel.submit({operation:'save',id:current?.id||null,body:{kind:'world',name:draft.name,body:draft.body,...(current?{expectedVersion:current.draft_version||current.published_version}:{}),reason:'保存世界基础框架底稿'}});
  }else resource=current;
  return resource;
 }
 function paint(){if(!live())return;generator?.dispose();const pending=channel.pending();box.innerHTML=`<div class="stack"><div data-generator></div>${state.framework?`<section class="card stack"><h2>待应用的基础框架</h2>${WorldFrameworkPreview(state.framework)}<p class="small muted">仅补充分类与缺少的基础地点；保留已有地点的正式名、描述和历史，也不改变她的位置。世界设定表单请另行保存。</p><button data-domain-ui="button" class="btn primary" data-apply ${busy?'disabled':''}>${busy?'正在保存并应用…':'保存并应用到此世界'}</button></section>`:''}${pending?.rejection?'<button data-domain-ui="button" class="btn" data-rebase>重新读取并核对最新版本</button>':pending?'<button data-domain-ui="button" class="btn" data-original>继续核对原保存或应用</button>':''}${error?`<p class="banner err" role="alert">${esc(error)}</p>`:''}</div>`;
  generator=mountWorldFrameworkGenerator(box.querySelector('[data-generator]'),{scope:'runtime:'+fid+':'+world.id,alive:live,readDraft:()=>({name:world.name,body:body()}),ensureResource,adopt:value=>{state.framework=value.body.framework;save();paint();}});
  box.querySelector('[data-rebase]')?.addEventListener('click',async()=>{if(busy)return;busy=true;try{const pending=channel.pending();if(!pending?.rejection)return;const value=pending.value;const current=await studioClient.resource(value.id||state.resourceId);if(current.kind!=='world')throw new Error('原底稿类型不是世界');if(value.operation==='apply')await studioClient.preview(fid,value.id,value.body.resourceVersion,world.id);if(!live())return;resource=current;channel.releaseRejected();error='已重新读取底稿、人物与世界版本；框架草稿保留。请核对后再次保存并应用。';}catch(e){if(live())error=e.message;}finally{busy=false;if(live())paint();}});
  box.querySelector('[data-apply]')?.addEventListener('click',()=>void apply(false));box.querySelector('[data-original]')?.addEventListener('click',()=>void apply(true));
 }
 async function apply(retry){if(busy||!live())return;busy=true;error='';paint();try{
  if(retry){await channel.retry();if(live())error='原提交已核对，可继续明确应用当前框架。';return;}
  const draft=await ensureResource({name:world.name,body:body()});resource=await studioClient.resource(draft.id);
  if(resource.draft_version!==resource.published_version)resource=await channel.submit({operation:'publish',id:resource.id,revision:resource.revision});
  const preview=await studioClient.preview(fid,resource.id,resource.published_version,world.id);if(!live())return;
  await channel.submit({operation:'apply',id:resource.id,body:{fluctlightId:fid,runtimeWorldId:world.id,resourceVersion:resource.published_version,expectedInstanceRevision:preview.instance_revision,expectedRuntimeRevision:preview.runtime_revision,choices:preview.diff.map(row=>({field:row.field,decision:row.field==='framework'?'override':'keep'})),reason:'为当前世界补充已审阅的基础场地和地点'}});
  state.framework=null;save();if(live()){toast('世界框架已应用，正在读取实际地点');await onApplied();}
 }catch(e){if(live())error=e.message;}finally{busy=false;if(live())paint();}}
 paint();return {dispose(){active=false;generator?.dispose();}};
}

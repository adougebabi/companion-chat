import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {persistentCommand} from '../v3/providers/persistent-command.mjs';
import {readStudioDraft,writeStudioDraft,studioWorkingDraftKey} from '../v3/proto/components/studio-working-draft.js';
import {studioDraftStorage} from '../v3/proto/components/studio-draft-storage.js';
import {worldDefinitionError,newWorldDefinition} from '../v3/proto/components/world-definition.js';
import {newEventDefinition} from '../v3/proto/components/event-definition.js';
import {newCharacterTemplate} from '../v3/proto/components/character-template.js';
import {newItemDefinition} from '../v3/proto/components/item-definition.js';
import {newPlaceDefinition} from '../v3/proto/components/place-definition.js';
import {newOutfitDefinition} from '../v3/proto/components/outfit-definition.js';
import {newVisualStyle} from '../v3/proto/components/visual-style.js';
import {errorMessage} from '../v3/domain-presentation.mjs';
const clone=value=>JSON.parse(JSON.stringify(value)),same=(a,b)=>JSON.stringify(a)===JSON.stringify(b);
export const studioKinds={world:'世界',place:'地点',item:'物品',outfit:'服装方案',event_template:'事件模板',character_template:'角色模板',persona_fragment:'人格片段',visual_style:'视觉风格'};
export function studioDefaultBody(kind){return ({world:newWorldDefinition,place:newPlaceDefinition,item:newItemDefinition,outfit:newOutfitDefinition,event_template:newEventDefinition,character_template:newCharacterTemplate,visual_style:newVisualStyle,persona_fragment:()=>({schema:'persona-fragment.v1',changes:{}})})[kind]?.()||{};}
export function createStudioEditor({owner,resourceId,kind='event_template',service,ownerCurrent,navigate,storage=studioDraftStorage,jobId='',allowNpc=false}){
 const creating=resourceId==='new',slot=`fluctlight-studio:${owner}:${resourceId}`;
 const state=proxy({resource:null,draft:null,pending:null,loading:false,busy:false,error:'',notice:'',preview:null,jobId,definitionEpoch:0,readbackPending:false,rebaseReview:null,publishReview:null});
 let active=false,readEpoch=0,previewEpoch=0;
 const live=()=>active&&ownerCurrent();
 const command=persistentCommand({key:slot+':command',storage,execute:async(value,key)=>{
  if(!live())throw Error('会话或资源已切换，请返回原范围核对');
  if(value.operation==='save'){const result=await service.save(value.id,{...value.body,idempotencyKey:key});if(!result||typeof result.id!=='string'||typeof (result.draft_version||result.published_version)!=='string')throw Error('保存回执不完整，请核对原提交；不会创建新请求');return result;}
  if(value.operation==='publish')return service.publish(value.id,value.revision,key);
  if(value.operation==='prepare')return service.createJob({...value.body,idempotencyKey:key});
  if(value.operation==='apply')return service.apply(value.id,{...value.body,idempotencyKey:key});
  throw Error('未知创作操作');
 }});
 const syncPending=()=>state.pending=command.pending();
 function persist(){if(!live()||!state.draft)return false;try{writeStudioDraft(owner,resourceId,state.draft,storage);return true;}catch(cause){state.error='草稿保存失败：'+errorMessage(cause);return false;}}
 function adoptDefinition(value){if(!live()||!state.draft)return;state.definitionEpoch++;edit({name:value.name,body:clone(value.body),bodyText:undefined});}
 function parseBodyText(){if(state.draft?.bodyText===undefined)return true;try{const value=JSON.parse(state.draft.bodyText);if(!value||typeof value!=='object'||Array.isArray(value))throw Error('完整定义必须是 JSON 对象');state.definitionEpoch++;edit({body:value,bodyText:undefined});return true;}catch(cause){state.error=errorMessage(cause,'JSON 尚未修正，输入已保留。');return false;}}
 const content=()=>({name:state.draft.name,body:clone(state.draft.body),kind:state.draft.kind});
 const dirty=()=>!state.resource||!state.draft||state.draft.bodyText!==undefined||!same({name:state.draft.name,body:state.draft.body},{name:state.resource.name,body:state.resource.body||{}});
 async function load(){const token=++readEpoch;state.loading=true;try{const latest=creating?null:await service.resource(resourceId);if(!live()||token!==readEpoch)return;
   if(latest&&(latest.id!==resourceId||latest.owner_actor_id&&latest.owner_actor_id!==owner))throw Error('创作资源范围不匹配');
   if(latest?.kind==='npc'&&!allowNpc){navigate('/studio/npc/'+encodeURIComponent(latest.id)+(state.jobId?'?job='+encodeURIComponent(state.jobId):''));return;}
   const nextDraft=!state.draft?readStudioDraft(owner,resourceId,latest,{kind,body:studioDefaultBody(kind)},storage):null;
   state.resource=latest?clone(latest):null;
   if(nextDraft)state.draft=nextDraft;
   syncPending();state.preview=null;previewEpoch++;state.readbackPending=false;state.error='';
  }catch(cause){if(live()&&token===readEpoch)state.error=errorMessage(cause);}finally{if(live()&&token===readEpoch)state.loading=false;}}
 function edit(values){if(!live()||!state.draft||state.resource?.status==='archived')return;Object.assign(state.draft,values);persist();}
 function setKind(next){if(!creating||state.busy||command.pending())return;state.definitionEpoch++;edit({kind:next,body:studioDefaultBody(next),bodyText:undefined});}
 function setTarget(target){edit({target});state.preview=null;previewEpoch++;}
 async function submit(value,retry=false){if(!live()||state.busy)return;if(!retry&&command.pending()){state.error='请先核对原提交，当前编辑仍保留。';return;}state.busy=true;state.error='';state.notice='';let acknowledged=false;
  try{const result=await(retry?command.retry():command.submit(value));acknowledged=true;if(!live())return;
   if(value.operation==='save'){
    const unchanged=state.draft.bodyText===undefined&&same(content(),{name:value.body.name,body:value.body.body,kind:value.body.kind});
    state.resource=result;state.draft.expectedVersion=result.draft_version||result.published_version;
    if(unchanged){state.draft.name=value.body.name;state.draft.body=clone(value.body.body);}persist();
    if(creating){writeStudioDraft(owner,result.id,state.draft,storage);storage().removeItem(studioWorkingDraftKey(owner,resourceId));navigate('/studio/edit/'+encodeURIComponent(result.id));return;}
    state.readbackPending=true;const actual=await service.resource(result.id);if(!live())return;state.resource=actual;state.readbackPending=false;
    state.notice=unchanged?'定义草稿已保存并读回。':'原草稿已保存，后来输入的修改仍保留。';
   }else if(value.operation==='publish'){state.resource=await service.resource(value.id);if(live())state.notice='已发布确认的保存版本。应用仍需独立确认。';}
   else if(value.operation==='prepare'){state.jobId=result.id;state.notice='解析任务已受理。请审阅结果，再采用到草稿、保存和发布。';}
   else if(result.target_kind==='foundation'&&result.status==='proposed'&&result.proposal_id){state.preview=null;state.notice='身份核心提议已保存，接受后才会生效。';}
   else{if(!result.applied)throw Error('尚未确认资源已应用，请读取实际状态');if(state.draft.target===value.body.fluctlightId)state.preview=null;state.notice=result.target_kind==='wardrobe_outfit'?'服饰组合已保存，穿着需要独立请求。':result.target_kind==='world_item'?'物品已登记到世界目录，尚未取得。':result.target_kind==='visual_brief'?'视觉参考已保存，新的生成会话会使用此版本。':'已应用确认的发布版本，请到目标世界核对。';}
  }catch(cause){if(live())state.error=(acknowledged?'操作已有回执，后续读取未完成：':'')+errorMessage(cause);}finally{if(live()){state.busy=false;syncPending();}}
 }
 function save(){if(!state.draft||state.busy||command.pending()||!parseBodyText())return;if(!state.draft.name.trim()){state.error='请填写资源名称。';return;}if(!state.draft.reason.trim()){state.error='请填写保存原因。';return;}const failure=state.draft.kind==='world'?worldDefinitionError(state.draft.body):'';if(failure){state.error=failure;return;}if(!persist())return;return submit({operation:'save',id:state.resource?.id||null,body:{kind:state.draft.kind,name:state.draft.name,body:clone(state.draft.body),expectedVersion:state.draft.expectedVersion,reason:state.draft.reason}});}
 function retry(){const pending=command.pending();if(!pending){state.pending=null;state.error='原提交记录已变化，请重新读取资源。';return;}return submit(pending.value,true);}
 async function reviewRebase(){if(!live()||state.busy||creating||command.pending()&&!command.pending().rejection)return;state.busy=true;try{const latest=await service.resource(resourceId);if(live())state.rebaseReview={resource:latest,key:command.pending()?.idempotencyKey};}catch(cause){if(live())state.error=errorMessage(cause);}finally{if(live())state.busy=false;}}
 function confirmRebase(){const review=state.rebaseReview;if(!live()||!review||command.pending()?.idempotencyKey!==review.key)return;if(command.pending()?.rejection)command.releaseRejected();state.resource=review.resource;state.draft.expectedVersion=review.resource.draft_version||review.resource.published_version;state.rebaseReview=null;state.preview=null;previewEpoch++;persist();syncPending();state.notice='已明确采用最新版本基线，当前草稿内容仍保留。';}
 function reviewPublish(){if(!state.resource||state.busy||dirty()||!state.resource.draft_version||state.resource.draft_version===state.resource.published_version)return;state.publishReview={resource:clone(state.resource),draft:content()};}
 function confirmPublish(){const review=state.publishReview;if(!review||!live())return;state.publishReview=null;if(!same(review.draft,content())){state.error='草稿已经变化，请重新核对保存版本。';return;}return submit({operation:'publish',id:review.resource.id,revision:review.resource.revision});}
 async function readPreview(){if(!state.draft?.target||!state.resource?.published_version){state.error='请选择目标角色，并先发布定义。';return;}const fid=state.draft.target,version=state.resource.published_version,token=++previewEpoch;state.error='';try{const result=await service.preview(fid,state.resource.id,version);if(live()&&token===previewEpoch&&fid===state.draft.target)state.preview={...result,fid,version,viewEpoch:token,choices:{},memberItems:(result.members||[]).map(()=> '')};}catch(cause){if(live()&&token===previewEpoch)state.error=errorMessage(cause);}}
 async function prepare(){const preview=state.preview;if(!preview?.preparation_required)return;try{const publication=await service.publication(state.resource.id,preview.version);if(!live()||state.preview!==preview)return;return submit({operation:'prepare',id:state.resource.id,body:{resourceId:state.resource.id,resourceRevision:state.resource.revision,kind:'assistant',purpose:'foundation_prepare',resourceVersion:preview.version,name:publication.name,body:publication.body,instruction:'按精确已发布文本解析完整结构化角色定义，保持来源意图，供所有者审阅后形成Foundation应用提案',reason:'所有者明确解析已发布文本模板'}});}catch(cause){if(live())state.error=errorMessage(cause);}}
 function chooseDifference(field,decision,viewEpoch){const p=state.preview;if(!live()||!p||p.viewEpoch!==viewEpoch||!p.diff.some(row=>row.field===field)||!['keep','override'].includes(decision))return;p.choices=Object.fromEntries([...Object.entries(p.choices).filter(([key])=>key!==field),[field,decision]]);}
 function chooseMember(index,value,viewEpoch){const p=state.preview;if(!live()||!p||p.viewEpoch!==viewEpoch||!Number.isInteger(index)||index<0||index>=p.memberItems.length)return;p.memberItems[index]=value;}
 function apply(){const p=state.preview;if(!p||!live()||!p.diff?.length&&!p.create_required)return;if(p.runtime_id&&p.diff.some(row=>!Object.hasOwn(p.choices,row.field)||!['keep','override'].includes(p.choices[row.field]))){state.error='请逐项选择如何处理实际差异。';return;}if(p.target_kind==='wardrobe_outfit'&&p.choices.members!=='keep'&&p.memberItems.some(id=>!id)){state.error='请为每个成员明确选择实际库存服饰。';return;}return submit({operation:'apply',id:state.resource.id,body:{fluctlightId:p.fid,resourceVersion:p.version,expectedInstanceRevision:p.instance_revision,expectedRuntimeRevision:p.runtime_revision??0,...(p.target_kind==='wardrobe_outfit'?{profileId:p.profile_id,expectedWardrobeRevision:p.wardrobe_revision,memberItems:p.members.map((member,index)=>({resourceId:member.resource_id,resourceVersion:member.resource_version,itemId:p.memberItems[index]}))}:{}),...(p.world_id?{worldId:p.world_id,expectedWorldRevision:p.world_binding_revision}:{}),choices:Object.entries(p.choices).map(([field,decision])=>({field,decision})),reason:'所有者确认差异后应用已发布资源'}});}
 return {state,subscribe:listener=>subscribe(state,listener),getSnapshot:()=>snapshot(state),live,dirty,persist,edit,adoptDefinition,parseBodyText,setKind,setTarget,load,save,retry,reviewRebase,confirmRebase,reviewPublish,confirmPublish,readPreview,prepare,apply,chooseDifference,chooseMember,start(){active=true;return load();},dispose(){active=false;readEpoch++;previewEpoch++;}};
}

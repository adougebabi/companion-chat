import {StudioComparison} from './studio-comparison.js';
import {studioClient} from '../services.js';
import {get} from '../store.js';
import {Field} from './forms.js';
import {esc,toast,confirm} from '../ui.js';
import {WorldFrameworkPreview} from './world-venues.js';
import {persistentCommand} from '../../providers/persistent-command.mjs';
import {studioDraftStorage} from './studio-draft-storage.js';

// Definition generation only. This panel never moves a character or writes
// runtime places; adoption goes through the owning setup/editor command.
export function mountWorldFrameworkGenerator(box,{scope,readDraft,ensureResource,adopt,alive=()=>box.isConnected}){
 const actor=get('session')?.actorId,storage=studioDraftStorage(),key=`fluctlight-world-framework:${actor}:${scope}`;
 let active=true,busy=false,timer=null,reads=0,job=null,error='',view,adopting=false;
 try{view=JSON.parse(storage.getItem(key)||'null');}catch{error='原世界生成草稿暂不可读，请核对已保存任务。';}
 view||={instruction:'请按世界主题扩展基础场地分类和具体地点，补齐楼层、描述、氛围、固定陈设和布局。',jobId:'',resourceId:''};
 const live=()=>active&&alive()&&get('session')?.actorId===actor;
 const save=()=>storage.setItem(key,JSON.stringify(view));
 const command=persistentCommand({key:key+':command',storage:()=>storage,execute:(value,idempotencyKey)=>{if(!live())throw new Error('身份或世界范围已变化，请返回原世界核对');return value.operation==='retry'?studioClient.retryJob(value.id,{expectedRevision:value.revision,reason:'恢复世界框架生成',idempotencyKey}):studioClient.createJob({...value.body,idempotencyKey});}});
 function paint(){if(!live()||adopting)return;const focused=box.contains((document.activeElement?.closest?.('[data-domain-react-control]')?.domainSource||document.activeElement))&&(document.activeElement?.closest?.('[data-domain-react-control]')?.domainSource||document.activeElement)?.name==='instruction',selection=focused?[(document.activeElement?.closest?.('[data-domain-react-control]')?.domainSource||document.activeElement).selectionStart,(document.activeElement?.closest?.('[data-domain-react-control]')?.domainSource||document.activeElement).selectionEnd]:null;const pending=command.pending();const output=job?.result,ready=job?.status==='completed'&&output?.disposition==='proposal'&&output.body?.framework?.venues?.length;
  box.innerHTML=`<section data-domain-ui="card" class="card stack"><div class="row"><h2 class="grow">生成基础场地与地点</h2><button data-domain-ui="button" class="btn sm" data-refresh>重新查询</button></div><p class="small muted">按世界主题生成固定分类和具体地点。生成结果先作为世界底稿，采用并保存后才形成地点；浏览不会移动角色。</p>${Field({label:'补充要求（可选）',name:'instruction',value:view.instruction,type:'textarea',attrs:{maxlength:4000}})}<div class="row"><button data-domain-ui="button" class="btn primary" data-generate ${busy?'disabled':''}>${busy?'正在受理…':'生成基础框架'}</button>${pending?.rejection?'<button data-domain-ui="button" class="btn" data-rebase>重新读取并核对最新版本</button>':pending?'<button data-domain-ui="button" class="btn" data-original>核对原生成请求</button>':''}${job?.status==='failed'?'<button data-domain-ui="button" class="btn" data-retry>恢复失败任务</button>':''}</div><p role="status">${job?esc({queued:'框架生成已排队',running:'正在生成场地和地点',completed:ready?'框架已生成，可以预览并采用':'本次生成已结束，请查看结果说明',failed:'框架未能生成',discarded:'本次结果已丢弃'}[job.status]||'任务状态待核对'):view.jobId?'正在读取原任务…':'尚未生成'}</p>${error?`<p data-domain-ui="alert" class="banner err" role="alert">${esc(error)}</p>`:''}${output?.reply?`<p>${esc(output.reply)}</p>`:''}${ready?`${WorldFrameworkPreview(output.body.framework)}<div data-framework-comparison></div>`:''}${view.jobId?`<a class="small" href="#/studio/edit/${encodeURIComponent(view.resourceId)}?job=${encodeURIComponent(view.jobId)}">查看原生成任务与诊断</a>`:''}</section>`;
  box.querySelector('[name=instruction]').oninput=e=>{view.instruction=e.target.value;save();};box.querySelector('[data-generate]').onclick=()=>void generate(false);box.querySelector('[data-refresh]').onclick=()=>{reads=0;void observe();};box.querySelector('[data-original]')?.addEventListener('click',()=>void generate(true));
  box.querySelector('[data-rebase]')?.addEventListener('click',async()=>{if(busy)return;busy=true;try{const pending=command.pending();if(!pending?.rejection)return;const value=pending.value;if(value.operation==='retry'){const current=await studioClient.job(value.id);if(!live())return;if(current.resource_id!==view.resourceId)throw new Error('原任务所属世界不一致');job=current;}else{const current=await studioClient.resource(value.body.resourceId);if(!live())return;if(current.kind!=='world')throw new Error('原底稿类型不是世界');}command.releaseRejected();error='已读取最新保存状态，原草稿和生成要求已保留。核对后可重新生成。';}catch(e){if(live())error=e.message;}finally{busy=false;if(live())paint();}});
  box.querySelector('[data-retry]')?.addEventListener('click',()=>void recover());
  updateComparison();if(selection){const input=box.querySelector('[name=instruction]');input.focus({preventScroll:true});input.setSelectionRange(...selection);}

 }
 function updateComparison(){
  const target=box.querySelector('[data-framework-comparison]'),output=job?.result;if(!live()||adopting||!target||!output?.body?.framework)return;
  const current=readDraft(),proposal={name:current.name,body:{...structuredClone(current.body),framework:structuredClone(output.body.framework)}};
  const opened=[...target.querySelectorAll('details')].map((n,i)=>n.open?i:-1);target.innerHTML=StudioComparison(current,proposal);target.querySelectorAll('details').forEach((n,i)=>{n.open=opened.includes(i);});
  target.querySelector('[data-adopt]').onclick=async()=>{
   const reviewed=JSON.stringify(readDraft()),selected=job.id;adopting=true;
   const accepted=await confirm({title:'采用框架到世界草稿',identity:'工作草稿',okText:'采用到草稿',text:'将使用所比较的基础框架替换本次框架草稿，其他世界资料保持。之后仍需保存。'});adopting=false;
   if(!accepted||!live())return;
   if(selected!==job?.id||reviewed!==JSON.stringify(readDraft())){error='世界草稿已变化，请核对新的比较后再采用。';updateComparison();return;}
   try{await adopt({...proposal,resourceId:view.resourceId,jobId:selected});if(live())toast('世界框架已采用到草稿，保存后生效');}catch(cause){error=cause.message;if(live())paint();}
  };
 }
 async function accept(receipt){view.jobId=receipt.id;view.resourceId=receipt.resource_id||view.resourceId;save();job=await studioClient.job(view.jobId);if(job.resource_id!==view.resourceId)throw new Error('世界生成任务范围不匹配');}
 async function generate(retry=false){if(busy||!live())return;busy=true;clearTimeout(timer);error='';paint();try{
   if(retry){await accept(await command.retry());}
   else{const draft=structuredClone(readDraft()),instruction=view.instruction;const resource=await ensureResource(draft);if(!live())return;view.resourceId=resource.id;save();const receipt=await command.submit({operation:'create',body:{resourceId:resource.id,resourceRevision:resource.revision,kind:'assistant',name:draft.name,body:draft.body,instruction:instruction||'按世界主题生成基础场地与地点框架',reason:'初始化世界基础场景'}});await accept(receipt);}
   reads=0;
  }catch(e){if(live())error=e.message;}finally{busy=false;if(live()){paint();schedule();}}}
 async function recover(){if(busy||!job||!live())return;busy=true;error='';paint();try{await accept(await command.submit({operation:'retry',id:job.id,revision:job.revision}));reads=0;}catch(e){error=e.message;}finally{busy=false;if(live()){paint();schedule();}}}
 async function observe(){if(!live()||busy||!view.jobId)return;try{const actual=await studioClient.job(view.jobId);if(!live())return;if(actual.resource_id!==view.resourceId||(actual.owner_actor_id&&actual.owner_actor_id!==actor))throw new Error('该生成任务属于其他世界或身份');job=actual;error='';}catch(e){if(live())error=e.message;}finally{if(live()){paint();schedule();}}}
 function schedule(){clearTimeout(timer);if(live()&&!document.hidden&&['queued','running'].includes(job?.status)&&++reads<80)timer=setTimeout(()=>void observe(),3000);}
 const visibility=()=>{clearTimeout(timer);if(!document.hidden)void observe();};document.addEventListener('visibilitychange',visibility);paint();if(view.jobId)void observe();
 return {generate:()=>generate(false),updateDraft:updateComparison,dispose(){active=false;clearTimeout(timer);document.removeEventListener('visibilitychange',visibility);}};
}

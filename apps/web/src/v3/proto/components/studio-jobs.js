import {studioMaterialRetention} from './studio-material-retention.js';
import {referenceFromGrant,mediaReferencePurposes} from './media-reference-grants.js';
import {studioClient} from '../services.js';
import {esc,confirm,requireOwner,toast} from '../ui.js';
import {Field,TextComposer,PromptChips,DiffRows} from './forms.js';
import {MessageLog} from './chat.js';
import {PreviewCard} from './cards.js';
import {JsonDetails} from './records.js';
import {persistentCommand} from '../../providers/persistent-command.mjs';
import {studioDraftStorage} from './studio-draft-storage.js';

const status={queued:'已排队',running:'执行中',completed:'已完成',failed:'失败',discarded:'已丢弃'};
const kindLabel={assistant:'助手建议',text_preview:'虚构文本预览',image_preview:'虚构图片预览'};
const mediaStatus={pending:'等待执行',started:'执行中',cancel_requested:'正在取消',ready:'图片已就绪',failed:'生成失败',cancelled:'已取消',completed:'执行完成',deadletter:'执行失败'};
const errorLabel={studio_media_not_configured:'图片生成尚未配置，请在配置完成后重试',studio_media_generation_failed:'图片生成失败，请核对实际执行状态后重试',studio_media_execution_active:'原图片任务仍在执行或取消中，请刷新原任务状态',workflow_status_unavailable:'暂时无法查询工作流状态，请稍后刷新'};
const json=v=>JSON.stringify(v,null,2);
// One mounted panel owns its requests/timer. It always reads the editor's
// current draft; accepted snapshots and generated suggestions stay separate.
export function mountStudioJobs(el,{actor,resource,readDraft,adopt,alive,jobId='',linkBase='',onSelect=()=>{},initialKind='assistant'}){
 const slot=`fluctlight-studio:${actor}:${resource?.id||'new'}:jobs`,storage=studioDraftStorage();
 let active=true,epoch=0,directoryEpoch=0,materialsEpoch=0,mounted=false,grantEpoch=0,referenceGrants=[],grantNext='',timer=null,busy=false,error='',job=null,list=[],materials=[],next='',materialNext='',reading=false;
 const readErrors={detail:'',directory:'',materials:'',references:''};
 let ui={instruction:'',kind:initialKind,selected:jobId,referenceAssetId:'',referencePurpose:'',referenceGrant:null,materialReleased:false};
 try{ui={...ui,...JSON.parse(storage.getItem(slot+':draft')||'{}')};if(jobId)ui.selected=jobId;}catch{error='工作室面板草稿不可读取，请核对原提交';}
 const live=()=>active&&alive()&&el.isConnected;
 const persist=()=>{if(live())storage.setItem(slot+':draft',json(ui));};
 const command=persistentCommand({key:slot+':command',storage:studioDraftStorage,execute:async(value,key)=>{
  if(!live())throw new Error('会话或资源范围已切换，请返回原资源重试');
  if(value.op==='create')return studioClient.createJob({...value.body,idempotencyKey:key});
  const body={expectedRevision:value.revision,reason:value.reason,idempotencyKey:key};
  if(value.op==='retry')return studioClient.retryJob(value.id,body);
  if(value.op==='discard')return studioClient.discardJob(value.id,body);
  if(value.op==='save-material')return studioClient.saveMaterial(value.id,body);
  throw new Error('未知工作室命令');
 }});
 function deepLink(id){return `${linkBase||`#/studio/edit/${encodeURIComponent(resource.id)}`}${linkBase?.includes('?')?'&':'?'}job=${encodeURIComponent(id)}`;}
 function select(id){ui.selected=id;onSelect(id);persist();if(linkBase&&live())history.replaceState(null,'',deepLink(id));job=null;epoch++;paint();void observe();}
 async function submit(value,retry=false){
  if(busy||!resource||!(await requireOwner())||!live()||busy)return;busy=true;clearTimeout(timer);epoch++;reading=false;error='';paint();
  try{const receipt=retry?await command.retry():await command.submit(value);if(!live())return;
   if(value.op==='save-material'){const actual=await studioClient.material(receipt.id);if(!live())return;toast(actual.released_at?'原保存请求已记录；素材目前已取消保留':'结果当前保留为此资源的定义素材','ok');}
   else {ui.selected=receipt.id;onSelect(receipt.id);persist();if(linkBase)history.replaceState(null,'',deepLink(receipt.id));}
   await Promise.all([directory(),readMaterials()]);
  }catch(e){if(live())error=e.message;}finally{busy=false;if(live()){paint();void observe();}}
 }
 async function observe(){
  if(!resource||!live()||busy||document.hidden)return;
  const selected=ui.selected,token=++epoch;reading=true;paint();
  try{const detail=selected?await studioClient.job(selected):null;
   if(!live()||token!==epoch||selected!==ui.selected)return;
   if(detail&&(detail.resource_id!==resource.id||(detail.owner_actor_id&&detail.owner_actor_id!==actor)))throw new Error('该任务属于其他资源，请从原资源查看');
   job=detail;readErrors.detail='';
  }catch(e){if(live()&&token===epoch)readErrors.detail=e.message;}
  finally{if(live()&&token===epoch){reading=false;paint();schedule();}}
 }
 async function directory(append=false){
  if(!resource||!live())return;const token=++directoryEpoch,cursor=append?next:undefined;
  try{const page=await studioClient.jobs({resource_id:resource.id,cursor});if(!live()||token!==directoryEpoch)return;
   list=append?[...list,...page.items.filter(item=>!list.some(old=>old.id===item.id))]:page.items;next=page.next_cursor||'';readErrors.directory='';
  }catch(e){if(live()&&token===directoryEpoch)readErrors.directory=e.message;}finally{if(live()&&token===directoryEpoch)paint();}
 }
 async function readMaterials(append=false){
  if(!resource||!live())return;const token=++materialsEpoch,cursor=append?materialNext:undefined;
  try{const page=await studioClient.materials({resource_id:resource.id,cursor,released:!!ui.materialReleased});if(!live()||token!==materialsEpoch)return;
   const owned=page.items.filter(item=>item.owner_actor_id===actor&&item.resource_id===resource.id);
   materials=append?[...materials,...owned.filter(item=>!materials.some(old=>old.id===item.id))]:owned;materialNext=page.next_cursor||'';readErrors.materials='';
  }catch(e){if(live()&&token===materialsEpoch)readErrors.materials=e.message;}finally{if(live()&&token===materialsEpoch)paint();}
 }
 async function readReferenceGrants(append=false){
  if(!resource||!live())return;const token=++grantEpoch;try{const page=await studioClient.referenceGrants({target_id:resource.id,target_scope:'studio_resource',cursor:append?grantNext:undefined});if(!live()||token!==grantEpoch)return;referenceGrants=append?[...referenceGrants,...page.items.filter(g=>!referenceGrants.some(old=>old.id===g.id))]:page.items;grantNext=page.next_cursor||'';readErrors.references='';}catch(e){if(live()&&token===grantEpoch)readErrors.references=e.message;}finally{if(live()&&token===grantEpoch)paint();}
 }
 async function refresh(){await Promise.all([observe(),directory(),readMaterials(),readReferenceGrants()]);}
 function schedule(){clearTimeout(timer);if(live()&&!busy&&!document.hidden&&job&&['queued','running'].includes(job.status))timer=setTimeout(()=>void observe(),3000);}
 function visibility(){clearTimeout(timer);if(!document.hidden&&!busy)void observe();}
 document.addEventListener('visibilitychange',visibility);
 function paint(){
  if(!live())return;let pending=null;try{pending=command.pending();}catch(e){error=e.message;}
  const displayError=[error,...Object.values(readErrors)].filter(Boolean).join('；');
  const out=job?.result||{},draft=readDraft();
  const messages=job?[{u:job.snapshot?.instruction||''},...(job.kind==='assistant'&&job.status==='completed'?[{a:String(out.reply||'')}]:[])]:[];
  if(!mounted){
   el.innerHTML=`<div class="card assistant"><div class="row"><h3 class="grow">创作助手与隔离沙盒</h3><button class="btn ghost sm" data-refresh ${reading?'disabled':''}>刷新状态</button></div><p class="small muted">使用当前表单草稿独立创作。结果不会进入生活；建议只有明确采用后才进入同一份工作草稿。新资源请先保存。</p>${Field({label:'任务',name:'kind',type:'select',value:ui.kind,options:[{value:'assistant',label:'助手：建议完整定义'},{value:'text_preview',label:'沙盒：虚构文本预览'},{value:'image_preview',label:'沙盒：虚构图片预览'}]})}<div data-image-reference>${Field({label:'已保存的图片参考（可不选）',name:'referenceAsset',type:'select',options:[{value:'',label:'不使用参考图片'}]})}${Field({label:'参考用途（选图后必选）',name:'referencePurpose',type:'select',value:ui.referencePurpose,options:[{value:'',label:'请选择用途'},{value:'scene',label:'场景'},{value:'animal',label:'动物'},{value:'object',label:'物体'},{value:'person',label:'人物（需明确授权）'}]})}<p class="small muted">已保留的非人物创作素材可按原来源引用。上传、人物或旧图须先明确授权给当前资源；读取权限不自动授予模型使用权。</p><a class="btn" href="#/studio/media?target=${encodeURIComponent(resource?.id||'')}">管理此资源的图片参考授权</a><button class="btn" type="button" data-more-grants hidden>更多已授权参考</button></div><div data-job-messages></div>${PromptChips(['检查定义的一致性并提出完整修改建议','保留字段和稳定引用，完善细节','写一段体现这份定义的虚构文本'])}${TextComposer({variant:'assistant',placeholder:'说明希望怎样修改或预览…',attrs:{'data-studio-compose':true}})}<p class="small muted">${resource?`资源 ${esc(resource.id)} · 已保存版本基线 v${esc(resource.revision)}`:'保存资源后可提交真实任务'}</p><div data-job-feedback></div></div><div data-job-result></div><div data-job-directory></div><div data-job-materials></div>`;
   mounted=true;
   const input=el.querySelector('[name=t]');input.value=ui.instruction;input.oninput=()=>{ui.instruction=input.value;persist();};
   el.querySelector('[name=kind]').onchange=e=>{ui.kind=e.target.value;persist();paint();};
   el.querySelector('[name=referenceAsset]').onchange=e=>{const grant=e.target.value.startsWith('grant:')?referenceGrants.find(g=>g.id===e.target.value.slice(6)):null;ui.referenceGrant=grant?referenceFromGrant(grant):null;ui.referenceAssetId=grant?grant.asset_id:e.target.value;ui.referencePurpose=grant?grant.purpose:ui.referencePurpose;el.querySelector('[name=referencePurpose]').value=ui.referencePurpose;persist();paint();};
   el.querySelector('[name=referencePurpose]').onchange=e=>{ui.referencePurpose=e.target.value;persist();};
   el.querySelectorAll('[data-chip]').forEach(b=>b.onclick=()=>{ui.instruction=b.dataset.chip;input.value=ui.instruction;persist();});
   el.querySelector('[data-studio-compose]').onsubmit=e=>{e.preventDefault();void create();};
   el.querySelector('[data-refresh]').onclick=()=>{error='';void refresh();};
  }
  el.querySelector('[data-image-reference]').hidden=ui.kind!=='image_preview';
  const reference=el.querySelector('[name=referenceAsset]');
  const imageMaterials=materials.filter(m=>m.asset_id&&!m.released_at&&m.owner_actor_id===actor&&m.resource_id===resource?.id);
  const options=[{value:'',label:'不使用参考图片'},...imageMaterials.map(m=>({value:m.asset_id,label:`${m.label} · ${m.id}`})),...referenceGrants.map(g=>({value:'grant:'+g.id,label:`已授权 · ${mediaReferencePurposes[g.purpose]} · ${g.asset_id}`}))];
  if(!ui.referenceGrant&&ui.referenceAssetId&&!imageMaterials.some(m=>m.asset_id===ui.referenceAssetId))options.push({value:ui.referenceAssetId,label:'已恢复的原参考（当前页未载入，请核对素材）'});
  if(ui.referenceGrant&&!referenceGrants.some(g=>g.id===ui.referenceGrant.grantId))options.push({value:'grant:'+ui.referenceGrant.grantId,label:'原已选授权（请刷新核对状态）'});
  const optionHTML=options.map(o=>`<option value="${esc(o.value)}">${esc(o.label)}</option>`).join('');
  if(reference.innerHTML!==optionHTML)reference.innerHTML=optionHTML;
  reference.value=ui.referenceGrant?'grant:'+ui.referenceGrant.grantId:ui.referenceAssetId;
  el.querySelector('[name=referencePurpose]').disabled=!!ui.referenceGrant;el.querySelector('[name=referencePurpose]').value=ui.referenceGrant?.purpose||ui.referencePurpose;
  el.querySelector('[data-more-grants]').hidden=!grantNext;el.querySelector('[data-more-grants]').onclick=()=>void readReferenceGrants(true);
  el.querySelector('[data-job-messages]').innerHTML=MessageLog(messages,{variant:'assistant'});
  el.querySelector('[data-job-feedback]').innerHTML=`<p role="alert" class="banner err" ${displayError?'':'hidden'}>${esc(displayError)}</p>${pending?`<div class="banner info">原提交结果尚未确认；当前输入单独保留。${JsonDetails(pending.value,'原提交内容')}<button class="btn" data-replay ${busy||pending.rejection?'disabled':''}>重试原提交</button>${pending.rejection?'<button class="btn" data-release>核对后解除已明确拒绝的原提交</button>':''}</div>`:''}`;
  el.querySelector('[data-job-result]').innerHTML=`${job?`<div class="card stack"><div class="row"><b class="grow">${esc(status[job.status]||job.status)}</b><a href="${esc(deepLink(job.id))}">任务永久链接</a></div><p class="small muted">${esc(job.id)} · 尝试 ${esc(job.latest_attempt)} · v${esc(job.revision)} · ${esc(errorLabel[job.error_code]||job.error_code||'')}</p>${job.kind==='assistant'&&job.status==='completed'&&out.disposition==='needs_input'?`<p class="banner info">需要补充创作输入，当前草稿保持。${esc(out.reply)}</p>${JsonDetails(out.warnings,'需要补充的信息')}`:''}${job.kind==='assistant'&&job.status==='completed'&&out.disposition==='proposal'?`${JsonDetails(out.warnings,'生成警告')}<h3>当前工作草稿 → 生成建议</h3>${DiffRows([{field:'name',before:draft.name,after:out.name},{field:'body',before:json(draft.body),after:json(out.body)}])}<button class="btn accent" data-adopt>明确采用完整建议到当前草稿</button>`:''}${job.kind==='text_preview'&&job.status==='completed'?PreviewCard({id:job.id,at:job.updated_at,text:out.text}):''}${job.kind==='image_preview'&&job.status==='completed'&&out.asset_id?PreviewCard({id:job.id,at:job.updated_at,img:studioClient.mediaUrl(out.asset_id),text:out.label}):''}${job.kind==='image_preview'?`<div class="stack"><h3>图片实际执行状态</h3>${(job.media_requests||[]).map(m=>`<div class="lrow"><div><b>${esc(mediaStatus[m.status]||m.status)}</b><p class="small muted">媒体任务 ${esc(m.id)} · 工作流 ${esc(m.workflow_id||'尚未分配')} · 生成任务 ${esc(m.provider_job_id||'尚未分配')}</p>${(m.workflows||[]).map(w=>`<p class="small">${esc(w.workflow_id)} · ${esc(mediaStatus[w.status]||status[w.status]||w.status)} ${esc(errorLabel[w.error_code]||w.error_code||'')}</p>`).join('')}</div></div>`).join('')||'<p class="small muted">尚无已受理的媒体执行记录；任务受理不代表图片完成。</p>'}</div>`:''}<div class="row">${job.status==='failed'?'<button class="btn" data-retry>重试失败任务（复用已完成阶段）</button>':''}${job.status==='completed'&&(job.kind==='text_preview'||(job.kind==='image_preview'&&out.asset_id))?`<button class="btn teal" data-material>存为资源${job.kind==='image_preview'?'图片':'文本'}素材</button>`:''}${job.status!=='discarded'?'<button class="btn danger" data-discard>丢弃任务结果</button>':''}<button class="btn" data-regenerate ${busy?'disabled':''}>按当前草稿重新生成新任务</button></div>${JsonDetails(job.attempts||[],'实际执行尝试与工作流状态')}${JsonDetails(job.snapshot,'本次冻结输入')}</div>`:''}`;
  el.querySelector('[data-job-directory]').innerHTML=`<div class="card"><h3>任务记录</h3>${list.map(item=>`<button class="lrow btn ghost" data-job="${esc(item.id)}"><span class="grow">${esc(kindLabel[item.kind]||item.kind)} · ${esc(item.created_at)}</span>${esc(status[item.status]||item.status)}</button>`).join('')||'<p class="small muted">尚无实际任务</p>'}${next?'<button class="btn" data-more>更多任务</button>':''}</div>`;
  el.querySelector('[data-job-materials]').innerHTML=`<div class="card"><h3>此资源的创作素材</h3><label class="toggle"><input type="checkbox" data-released-materials ${ui.materialReleased?'checked':''}>查看已取消保留的素材</label>${materials.map(m=>`<div class="lrow"><div><b>${esc(m.label)}</b><p class="small">${esc(m.text||'')}</p>${m.asset_id?`<img class="scene-img" data-preview-image src="${esc(studioClient.mediaUrl(m.asset_id))}" alt="已保存的虚构创作素材" loading="lazy"><p class="small muted" data-preview-image-error hidden>图片读取失败，请核对素材权限与任务状态后刷新。</p>`:''}<a href="${esc(deepLink(m.job_id))}">查看原任务</a><a class="btn" href="#/studio/material/${encodeURIComponent(m.id)}">来源与保留历史</a><button class="btn" type="button" data-material-retention="${esc(m.id)}">${m.released_at?'恢复保留':'取消保留'}</button></div></div>`).join('')||'<p class="small muted">尚无已保存素材</p>'}${materialNext?'<button class="btn" data-material-more>更多素材</button>':''}</div>`;
  el.querySelectorAll('[data-preview-image]').forEach(img=>{img.onerror=()=>{img.hidden=true;const notice=img.nextElementSibling;if(notice)notice.hidden=false;};});
  el.querySelector('[data-refresh]').disabled=reading||busy;
  el.querySelector('[data-studio-compose]').querySelector('button').disabled=busy||!resource;
  el.querySelector('[data-regenerate]')?.addEventListener('click',()=>void create());
  
  el.querySelectorAll('[data-job]').forEach(b=>b.onclick=()=>select(b.dataset.job));
  el.querySelector('[data-more]')?.addEventListener('click',()=>void directory(true));
  el.querySelector('[data-released-materials]').onchange=e=>{ui.materialReleased=e.target.checked;persist();void readMaterials();};
  el.querySelectorAll('[data-material-retention]').forEach(button=>button.onclick=()=>{const material=materials.find(m=>m.id===button.dataset.materialRetention);if(material&&live())studioMaterialRetention(material,()=>{if(live())return readMaterials();});});
  el.querySelector('[data-material-more]')?.addEventListener('click',()=>void readMaterials(true));
  el.querySelector('[data-replay]')?.addEventListener('click',()=>void submit(pending.value,true));
  el.querySelector('[data-release]')?.addEventListener('click',async()=>{if(!(await confirm({title:'解除已拒绝提交',text:'该首次提交已明确因版本冲突拒绝且无效果。核对最新资源或任务状态后解除；当前草稿保留。'}))||!live())return;try{command.releaseRejected();await refresh();}catch(e){error=e.message;paint();}});
  for(const [attr,op,reason] of [['retry','retry','所有者显式重试失败创作任务'],['discard','discard','所有者丢弃创作任务'],['material','save-material','所有者保存虚构创作结果为定义素材']])el.querySelector(`[data-${attr}]`)?.addEventListener('click',()=>void operate(op,reason));
  el.querySelector('[data-del]')?.addEventListener('click',()=>void operate('discard','所有者丢弃创作预览'));
  el.querySelector('[data-adopt]')?.addEventListener('click',async()=>{
   const selected=job;if(!adopt||!selected||selected.result?.disposition!=='proposal'||selected.resource_id!==resource.id)return;
   if(!(await confirm({title:'采用完整定义建议',text:'将以生成建议替换当前表单中的名称与完整定义。期间的新修改也会被此操作替换，请先比较差异。采用后仍须单独保存。'}))||!live()||job?.id!==selected.id)return;
   adopt({name:selected.result.name,body:structuredClone(selected.result.body)});
  });
 }
 async function create(){
  if(!resource||!ui.instruction.trim()) {error='请先保存资源，并填写创作指令';paint();return;}
  if(ui.kind==='image_preview'&&ui.referenceAssetId&&!ui.referenceGrant&&!['scene','animal','object'].includes(ui.referencePurpose)){error='选择参考图片后，请明确选择参考用途';paint();return;}
  const referenceAsset=ui.kind==='image_preview'&&ui.referenceAssetId?(ui.referenceGrant?structuredClone(ui.referenceGrant):{assetId:ui.referenceAssetId,purpose:ui.referencePurpose}):undefined;
  const d=readDraft();await submit({op:'create',body:{resourceId:resource.id,resourceRevision:resource.revision,kind:ui.kind,...(referenceAsset?{referenceAsset}:{}),name:d.name,body:structuredClone(d.body),instruction:ui.instruction,reason:'所有者提交隔离创作任务'}});
 }
 async function operate(op,reason){const target=job;if(!target||busy)return;if(op==='discard'&&!(await confirm({title:'丢弃创作结果',text:'丢弃后该任务的迟到结果不能重新发布。已明确保存的资源素材仍保留。'})))return;if(!live()||job?.id!==target.id)return;await submit({op,id:target.id,revision:target.revision,reason});}
 paint();void refresh();return {dispose(){active=false;epoch++;directoryEpoch++;materialsEpoch++;grantEpoch++;clearTimeout(timer);document.removeEventListener('visibilitychange',visibility);}};
}

import {studioClient} from '../services.js';
import {esc,requireOwner,confirm} from '../ui.js';
import {Field} from './forms.js';
import {JsonDetails} from './records.js';
import {mountCursorRecords} from './cursor-records.js';
import {studioDraftStorage} from './studio-draft-storage.js';
import {persistentCommand} from '../../providers/persistent-command.mjs';
import {get} from '../store.js';

export function mountResourceLifecycle(box,{actor,resource,alive,onChanged}){
 let active=true,busy=false,history=null,epoch=0,errorText='',lastCopy=null,receiptEpoch=0;
 const live=()=>active&&box.isConnected&&alive();
 const draftKey=`fluctlight-studio:${actor}:${resource.id}:lifecycle-draft`;
 const receiptKey=`fluctlight-studio:${actor}:${resource.id}:lifecycle-receipt`,commandKey=`fluctlight-studio:${actor}:${resource.id}:lifecycle`;
 const sameOwner=()=>get('session')?.actorId===actor;
 let copyDraft={name:resource.name+' 副本',source:resource.draft_version?`draft:${resource.draft_version}`:`published:${resource.published_version}`,reason:''};
 const stored=studioDraftStorage().getItem(draftKey);
 if(stored){try{copyDraft={...copyDraft,...JSON.parse(stored)};}catch{box.textContent='资源管理草稿不可读取，请保留原记录核对';return {dispose(){active=false;}};}}
 const command=persistentCommand({key:commandKey,storage:studioDraftStorage,execute:async(value,key)=>{
  if(!live())throw new Error('资源范围已切换，请返回原范围恢复原提交');
  const result=await studioClient[value.operation](resource.id,{...value.body,idempotencyKey:key});
  const pending=studioDraftStorage().getItem(commandKey);
  if(sameOwner()&&pending&&JSON.parse(pending).idempotencyKey===key){
   if(typeof result.id!=='string'||!result.id)throw new Error('操作回执缺少资源标识，请核对原提交');
   studioDraftStorage().setItem(receiptKey,JSON.stringify({sourceId:resource.id,targetId:result.id,operation:value.operation}));
  }
  return result;
 }});
 async function submit(operation,body,retry=false){
  if(busy||!(await requireOwner())||!live())return;
  busy=true;errorText='';paint();
  try{await (retry?command.retry():command.submit({operation,body}));
   if(sameOwner())window.dispatchEvent(new CustomEvent('fluctlight-studio-receipt',{detail:{actor,resourceId:resource.id}}));
  }catch(error){if(live())errorText=error.message;}finally{busy=false;if(live())paint();}
 }
 function paint(){
  if(!live())return;history?.dispose();const pending=command.pending(),awaitingReceipt=!!studioDraftStorage().getItem(receiptKey);
  const choices=[...(resource.draft_version?[{value:`draft:${resource.draft_version}`,label:`已保存草稿 ${resource.draft_version}`}]:[]),...(resource.published_version?[{value:`published:${resource.published_version}`,label:`已发布 ${resource.published_version}`}]:[])];
  if(!choices.some(c=>c.value===copyDraft.source))choices.push({value:copyDraft.source,label:`先前所选 ${copyDraft.source}`});
  box.innerHTML=`<div class="card stack"><h3>资源管理与发布历史</h3><p class="small muted">复制形成独立未发布草稿；删除采用归档，保留发布历史、任务及素材。已有实例或其他定义仍在引用时拒绝删除。</p>${resource.status==='archived'?'<p class="banner info">此定义已归档。历史和素材保留，恢复后才能继续编辑、发布或应用。</p>':''}<form data-copy class="stack">${Field({label:'副本名称',name:'name',value:copyDraft.name,attrs:{required:true,maxlength:200}})}${Field({label:'复制来源',name:'source',value:copyDraft.source,type:'select',options:choices})}${Field({label:'操作原因',name:'reason',value:copyDraft.reason,attrs:{required:true,maxlength:1000}})}<button data-domain-ui="button" class="btn" ${busy||pending||awaitingReceipt?'disabled':''}>创建独立副本</button></form><button data-domain-ui="button" class="btn ${resource.status==='archived'?'':'danger'}" data-archive ${busy||pending||awaitingReceipt?'disabled':''}>${resource.status==='archived'?'恢复此定义':'删除此定义（归档）'}</button>${lastCopy?`<p class="banner info">已创建独立副本：<a href="#/studio/${lastCopy.kind==='npc'?'npc':'edit'}/${encodeURIComponent(lastCopy.id)}">${esc(lastCopy.name)}</a></p>`:''}${awaitingReceipt?'<p class="banner info">上次操作已有回执，正在核对实际资源。<button data-domain-ui="button" type="button" class="btn" data-read-receipt>读取上次操作结果</button></p>':''}<p role="alert">${esc(errorText)}</p>${pending?`<div class="banner info">${JsonDetails(pending.value,'原管理提交')}<button data-domain-ui="button" class="btn" data-retry ${busy||pending.rejection?'disabled':''}>重试原管理提交</button>${pending.rejection?'<button data-domain-ui="button" class="btn" data-rebase>明确采用最新资源版本</button>':''}</div>`:''}<details data-domain-ui="disclosure"><summary>已发布版本（不含未发布工作草稿）</summary><div data-versions></div><div data-publication></div></details></div>`;
  const form=box.querySelector('[data-copy]');form.querySelectorAll('input,select,button').forEach(control=>control.disabled=busy||!!pending||awaitingReceipt);
  form.oninput=()=>{if(!live())return;for(const k of ['name','source','reason'])copyDraft[k]=form.elements[k].value;studioDraftStorage().setItem(draftKey,JSON.stringify(copyDraft));};
  form.onsubmit=e=>{e.preventDefault();if(!form.reportValidity())return;const [source,...rest]=form.elements.source.value.split(':');void submit('copy',{expectedRevision:resource.revision,source,sourceVersion:rest.join(':'),name:form.elements.name.value,reason:form.elements.reason.value});};
  box.querySelector('[data-archive]').onclick=async()=>{
   const operation=resource.status==='archived'?'restore':'archive';
   if(!(await confirm({title:operation==='archive'?'删除此定义（归档）':'恢复定义',text:operation==='archive'?'保留发布版本、任务及素材；不删除生活记录。有实例或定义引用时将拒绝操作。':'恢复目录可见性，既有实例不会自动应用。'}))||!live())return;
   void submit(operation,{expectedRevision:resource.revision,reason:operation==='archive'?'所有者归档未被引用的定义':'所有者恢复定义目录'});
  };
  box.querySelector('[data-read-receipt]')?.addEventListener('click',()=>void readReceipt());
  box.querySelector('[data-retry]')?.addEventListener('click',()=>void submit(pending.value.operation,pending.value.body,true));
  box.querySelector('[data-rebase]')?.addEventListener('click',async()=>{try{const latest=await studioClient.resource(resource.id);if(!live())return;command.releaseRejected();resource=latest;errorText='已采用最新管理版本，请检查并重新发起操作。';paint();}catch(error){errorText=error.message;paint();}});
  history=mountCursorRecords(box.querySelector('[data-versions]'),{alive:live,read:cursor=>studioClient.versions(resource.id,{cursor}),label:'发布版本',empty:'尚无已发布版本',render:r=>`<div class="lrow"><b class="grow">${esc(r.version)} · ${esc(r.name)}</b><button data-domain-ui="button" type="button" class="btn" data-publication-version="${esc(r.version)}">读取精确定义</button></div>`,bind:panel=>panel.querySelectorAll('[data-publication-version]').forEach(button=>button.onclick=async()=>{const token=++epoch;try{const exact=await studioClient.publication(resource.id,button.dataset.publicationVersion);if(live()&&token===epoch)box.querySelector('[data-publication]').innerHTML=JsonDetails(exact,'该不可变发布版本');}catch(error){if(live()&&token===epoch){errorText=error.message;box.querySelector('[data-publication]').textContent=errorText;}}})});
 }
 async function readReceipt(){
  const raw=studioDraftStorage().getItem(receiptKey);if(!raw||!live())return;
  const token=++receiptEpoch;
  try{const pointer=JSON.parse(raw);if(pointer.sourceId!==resource.id||!pointer.targetId)throw new Error('管理操作结果标识损坏，请保留原记录核对');
   const latest=await studioClient.resource(pointer.targetId);if(!live()||token!==receiptEpoch||!sameOwner()||studioDraftStorage().getItem(receiptKey)!==raw)return;
   studioDraftStorage().removeItem(receiptKey);
   if(pointer.operation==='copy'){lastCopy=latest;paint();}
   else {resource=latest;onChanged?.(latest);}
  }catch(error){if(live()&&token===receiptEpoch){errorText=`操作已有回执，结果读取未确认：${error.message}`;paint();}}
 }
 const onReceipt=event=>{if(event.detail?.actor===actor&&event.detail.resourceId===resource.id)void readReceipt();};
 window.addEventListener('fluctlight-studio-receipt',onReceipt);
 paint();void readReceipt();return {dispose(){active=false;epoch++;receiptEpoch++;history?.dispose();window.removeEventListener('fluctlight-studio-receipt',onReceipt);}};
}

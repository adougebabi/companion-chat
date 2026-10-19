import {drawer,esc,confirm,toast} from '../ui.js';
import {Field} from './forms.js';
import {JsonDetails} from './records.js';
import {get,subscribe} from '../store.js';
import {beginDraftSubmission,settleDraftSubmission} from './draft-submission.js';

const drafts=new Map();subscribe(()=>{if(!get('session'))drafts.clear();});
export function governanceCommand({scope,title,note,revision,read,execute,onSaved=()=>{},onRebase=()=>{},maxReason=1024,target,rebaseTarget,successMessage='操作已提交并保存',snapshot}){
 const key=JSON.stringify([get('session')?.actorId,scope]);if(!drafts.has(key))drafts.set(key,{reason:'',base:revision,target:structuredClone(target),pending:{}});
 const draft=drafts.get(key),value=()=>({expectedRevision:draft.base,reason:draft.reason,target:draft.target});
 const d=drawer({title,body:`<p>${esc(note)}</p><p class="small muted" data-version>当前对象 v${esc(draft.base)}</p><div data-snapshot>${draft.target!==undefined?JsonDetails({target:draft.target,page_snapshot:snapshot},'实际提交目标与页面资料'):snapshot?JsonDetails(snapshot,'核对当前对象'):''}</div>${Field({label:'理由',name:'reason',type:'textarea',value:draft.reason,attrs:{required:true,maxlength:maxReason}})}<p role="alert" data-error></p>`,footer:'<button class="btn" data-close>关闭</button><button class="btn" data-rebase hidden>读取最新版本并保留理由</button><button class="btn primary" data-submit>确认提交</button>'});
 const field=d.el.querySelector('[name=reason]'),submit=d.el.querySelector('[data-submit]'),rebase=d.el.querySelector('[data-rebase]'),errorBox=d.el.querySelector('[data-error]');
 field.oninput=()=>draft.reason=field.value;
 d.el.querySelector('[data-close]').onclick=()=>d.close();
 const settled=(error,saved,unchanged)=>{
  if(!d.el.isConnected)return;
  submit.disabled=!!draft.pending.command;rebase.disabled=!!draft.pending.command;
  if(error){errorBox.textContent=error.message;rebase.hidden=error.status!==409||['foundation_proposal_stale','fluctlight_retired','capability_review_transition_invalid'].includes(error.code);}
  else if(saved){if(unchanged)d.close();else{errorBox.textContent='原提交已保存，等待期间的新理由已保留；请核对最新版本再继续。';rebase.hidden=false;}}
  else if(draft.pending.command)errorBox.textContent='原提交正在处理。可以继续编辑理由；再次提交须等待实际结果。';
 };
 draft.onSettled=settled;settled();
 rebase.onclick=async()=>{
  if(draft.pending.command)return;rebase.disabled=true;
  try{const current=await read();if(!d.el.isConnected)return;d.el.querySelector('[data-snapshot]').innerHTML=JsonDetails(current,'最新对象（理由已保留）');if(await confirm({title:'采用最新对象版本',text:`请核对最新内容，再以 v${current.revision} 继续这次操作。理由保持。`,identity:'所有者修改'})&&d.el.isConnected){onRebase(current);if(rebaseTarget)draft.target=rebaseTarget(current,structuredClone(draft.target));draft.base=current.revision;d.el.querySelector('[data-snapshot]').innerHTML=JsonDetails({object:current,target:draft.target},'已明确采用的版本与目标');d.el.querySelector('[data-version]').textContent=`已明确采用当前对象 v${draft.base}`;errorBox.textContent='';rebase.hidden=true;}}
  catch(error){if(d.el.isConnected)errorBox.textContent=error.message;}
  finally{if(d.el.isConnected)rebase.disabled=false;}
 };
 submit.onclick=async()=>{
  if(draft.pending.command||!field.reportValidity())return;
  draft.reason=field.value;const submission=beginDraftSubmission(draft,'command',value());draft.onSettled?.();
  try{
   const {target:submittedTarget,...body}=submission.value;const result=await execute(body,submittedTarget);if(drafts.get(key)!==draft)return;
   const acknowledged=settleDraftSubmission(draft,submission,value());if(!acknowledged)return;
   if(acknowledged.unchanged)drafts.delete(key);
   draft.onSettled?.(null,result,acknowledged.unchanged);toast(typeof successMessage==='function'?successMessage(result):successMessage);
   try{await onSaved(result);}catch(error){toast(`操作已提交；重新读取失败：${error.message}`,'err');}
  }catch(error){if(drafts.get(key)===draft){settleDraftSubmission(draft,submission,null);draft.onSettled?.(error);}}
 };
 return d;
}

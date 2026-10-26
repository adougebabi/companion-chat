import {get,subscribe} from '../store.js';
import {Field} from './forms.js';
import {JsonDetails} from './records.js';
import {confirm,esc,toast} from '../ui.js';
import {beginDraftSubmission,settleDraftSubmission} from './draft-submission.js';
const drafts=new Map();subscribe(()=>{if(!get('session'))drafts.clear();});
// Shared presentation-only form. The supplied service owns persistence, CAS and
// command identity; this component owns unsaved inputs and response lifetimes.
export function mountVersionedFields(box,{scope,snapshot,values,fields,read,save,onSaved=()=>{},readOnly=false}){
 const key=JSON.stringify([get('session')?.actorId,scope]);
 if(!drafts.has(key))drafts.set(key,{values:structuredClone(values),base:snapshot.revision,reason:'',dirty:false,pending:{}});
 const draft=drafts.get(key);let observed=snapshot.revision;
 if(!draft.dirty&&!draft.pending.save&&observed>=draft.base){draft.values=structuredClone(values);draft.base=observed;}
 const current=()=>({values:draft.values,reason:draft.reason,expectedRevision:draft.base});
 box.innerHTML=`<form class="stack" data-versioned-form><p class="small muted" data-version>保存基线 v${esc(draft.base)}</p>${fields.map(([name,label,hint])=>Field({name,label,hint,type:'textarea',value:draft.values[name],attrs:{maxlength:1000,disabled:readOnly}})).join('')}${Field({label:'保存理由',name:'reason',value:draft.reason,attrs:{maxlength:1024,required:true,disabled:readOnly}})}<div class="row"><button data-domain-ui="button" class="btn" type="button" data-rebase>核对最新版本（保留草稿）</button><button data-domain-ui="button" class="btn primary" ${readOnly?'disabled':''}>保存定义</button></div><p role="alert" data-error></p><div data-latest></div></form>`;
 const form=box.querySelector('form'),submit=form.querySelector('[type=submit]')||form.querySelector('.primary'),rebase=form.querySelector('[data-rebase]'),error=form.querySelector('[data-error]');
 for(const [name]of fields)form.elements[name].oninput=()=>{draft.values[name]=form.elements[name].value;draft.dirty=true;};form.elements.reason.oninput=()=>{draft.reason=form.elements.reason.value;draft.dirty=true;};
 const settled=(failure,saved)=>{
  if(!box.isConnected)return;submit.disabled=readOnly||!!draft.pending.save||observed!==draft.base;rebase.disabled=!!draft.pending.save;
  form.querySelector('[data-version]').textContent=`保存基线 v${draft.base} · 最近观察 v${observed}`;
  if(failure)error.textContent=failure.message;else if(saved){if(!draft.dirty)form.elements.reason.value='';error.textContent=draft.dirty?'已保存原提交，等待期间的新输入保留，尚未保存。':'已持久保存；参考定义将在新的视觉会话中消费。';}else if(draft.pending.save)error.textContent='原提交处理中，可以继续编辑；新输入不会被本次提交清除。';else if(observed!==draft.base)error.textContent='版本已变化。草稿已保留，请核对最新内容再明确采用基线。';
 };draft.onSettled=settled;settled();
 rebase.onclick=async()=>{if(draft.pending.save||readOnly)return;rebase.disabled=true;try{const latest=await read();if(!box.isConnected)return;observed=latest.revision;form.querySelector('[data-latest]').innerHTML=JsonDetails(latest,'最新已保存定义（请对照草稿）');if(await confirm({title:'采用最新保存基线',text:`核对后以 v${latest.revision} 保存你的草稿。当前输入保留。`})&&box.isConnected){draft.base=latest.revision;error.textContent='已明确采用新基线，草稿保持。';}settled();}catch(e){settled(e);}finally{if(box.isConnected)rebase.disabled=false;}};
 form.onsubmit=async event=>{event.preventDefault();if(readOnly||draft.pending.save||!form.reportValidity())return;const submission=beginDraftSubmission(draft,'save',current());draft.onSettled?.();try{const saved=await save({...submission.value.values,expectedRevision:submission.value.expectedRevision,reason:submission.value.reason});if(drafts.get(key)!==draft)return;const result=settleDraftSubmission(draft,submission,current());if(!result)return;draft.base=saved.revision;draft.dirty=!result.unchanged;if(result.unchanged)draft.reason='';draft.onObserved?.(saved.revision);draft.onSettled?.(null,saved);toast('定义已保存');try{await onSaved(saved);}catch(e){toast(`已保存；关联数据重读失败：${e.message}`,'err');}}catch(e){if(drafts.get(key)===draft){settleDraftSubmission(draft,submission,null);draft.onSettled?.(e);}}};
 const observe=revision=>observed=revision;draft.onObserved=observe;
 return()=>{if(draft.onSettled===settled)delete draft.onSettled;if(draft.onObserved===observe)delete draft.onObserved;};
}

import {errorMessage} from '../../domain-presentation.mjs';
import {beginDraftSubmission,settleDraftSubmission} from './draft-submission.js';
import {Field} from './forms.js';
import {JsonDetails} from './records.js';
import {governanceClient} from '../services.js';
import {get,subscribe} from '../store.js';
import {drawer,esc,confirm,toast} from '../ui.js';
const drafts=new Map();subscribe(()=>{if(!get('session'))drafts.clear();});
export function foundationProposal(fid,current,onSaved){
 const key=JSON.stringify([get('session')?.actorId,fid]);
 if(!drafts.has(key))drafts.set(key,{changes:'{}',reason:'',revision:current.revision,pending:{}});
 const draft=drafts.get(key),value=()=>({changes:draft.changes,reason:draft.reason,revision:draft.revision});
 const d=drawer({title:'创建身份核心修订提议',body:`<p>保存提议后，可对照基线审核并接受。当前生活、物品和聊天历史各自保留。</p><p class="small muted" data-version>提议基线 v${esc(draft.revision)}</p>${JsonDetails({identity:current.identity,personality:current.personality,behavioral_policy:current.behavioral_policy,life_profile:current.life_profile},'核对当前可修订字段')}${Field({label:'字段变更（JSON）',name:'changes',type:'textarea',rows:10,value:draft.changes,hint:'按字段合并 identity、personality、behavioral_policy、life_profile。未提供的字段保留。请只填写要修改的字段；空字符串会明确清空该字段。',attrs:{required:true,maxlength:262144}})}${Field({label:'提议理由',name:'reason',type:'textarea',value:draft.reason,attrs:{required:true,maxlength:1024}})}<p role="alert" data-error></p>`,footer:'<button data-domain-ui="button" class="btn" data-close>关闭</button><button data-domain-ui="button" class="btn" data-rebase hidden>核对最新基线并保留草稿</button><button data-domain-ui="button" class="btn primary" data-submit>保存提议（不立即应用）</button>'});
 const changes=d.el.querySelector('[name=changes]'),reason=d.el.querySelector('[name=reason]'),submit=d.el.querySelector('[data-submit]'),rebase=d.el.querySelector('[data-rebase]'),error=d.el.querySelector('[data-error]');
 changes.oninput=()=>{draft.changes=changes.value;changes.setCustomValidity('');changes.removeAttribute('aria-invalid');};reason.oninput=()=>draft.reason=reason.value;d.el.querySelector('[data-close]').onclick=()=>d.close();
 const settled=(failure,saved,unchanged)=>{if(!d.el.isConnected)return;submit.disabled=rebase.disabled=!!draft.pending.proposal;if(failure){error.textContent=errorMessage(failure);rebase.hidden=failure.status!==409;}else if(saved){if(unchanged)d.close();else{error.textContent='原提议已保存，等待期间的新草稿已保留。请核对最新基线再创建新提议。';rebase.hidden=false;}}else if(draft.pending.proposal)error.textContent='原提议仍在提交，可继续编辑草稿；等待结果后再提交。';};
 draft.onSettled=settled;settled();
 rebase.onclick=async()=>{if(draft.pending.proposal)return;rebase.disabled=true;try{const latest=await governanceClient.current(fid);if(!d.el.isConnected)return;error.innerHTML=JsonDetails({identity:latest.identity,personality:latest.personality,behavioral_policy:latest.behavioral_policy,life_profile:latest.life_profile},'最新身份核心');if(await confirm({title:'采用最新提议基线',text:`请对照最新字段与保留的变更，再以 v${latest.revision} 创建提议。`})&&d.el.isConnected){draft.revision=latest.revision;d.el.querySelector('[data-version]').textContent=`已明确采用基线 v${draft.revision}`;rebase.hidden=true;}}catch(e){if(d.el.isConnected)error.textContent=errorMessage(e);}finally{if(d.el.isConnected)rebase.disabled=false;}};
 submit.onclick=async()=>{
  if(draft.pending.proposal||!changes.reportValidity()||!reason.reportValidity())return;
  let parsed;try{parsed=JSON.parse(changes.value);if(!parsed||Array.isArray(parsed)||typeof parsed!=='object'||!Object.keys(parsed).length)throw new Error('empty');}catch{const message='请填写有效的非空 JSON 对象，例如 {"identity":{"background_story":"新的经历"}}。草稿已保留。';error.textContent=message;changes.setCustomValidity(message);changes.setAttribute('aria-invalid','true');changes.reportValidity();changes.focus();return;}
  draft.changes=changes.value;draft.reason=reason.value;const submission=beginDraftSubmission(draft,'proposal',value());draft.onSettled?.();
  try{const saved=submission.value;const result=await governanceClient.propose(fid,{changes:parsed,reason:saved.reason,expectedRevision:saved.revision});if(drafts.get(key)!==draft)return;const acknowledged=settleDraftSubmission(draft,submission,value());if(!acknowledged)return;if(acknowledged.unchanged)drafts.delete(key);draft.onSettled?.(null,result,acknowledged.unchanged);toast('身份提议已持久保存，尚未应用');try{await onSaved(result);}catch(e){toast(`提议已保存；重新读取失败：${errorMessage(e)}`,'err');}}
  catch(e){if(drafts.get(key)===draft){settleDraftSubmission(draft,submission,null);draft.onSettled?.(e);}}
 };
 return d;
}

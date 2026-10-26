import {Field} from './forms.js';
import {VisualAsset,bindVisualAssetErrors} from './visual-identity-records.js';
import {beginDraftSubmission,settleDraftSubmission} from './draft-submission.js';
import {drawer,toast} from '../ui.js';
import {get,subscribe} from '../store.js';
import {visualClient} from '../services.js';
const drafts=new Map();subscribe(()=>{if(!get('session'))drafts.clear();});
export function visualReview(fid,identity,onSaved){
 const session=identity.session,attempt=session.attempts.find(a=>a.attempt_number===session.current_attempt);if(!attempt?.candidate_asset_id||attempt.decision)return;
 const key=JSON.stringify([get('session')?.actorId,fid,session.id,session.current_attempt]);if(!drafts.has(key))drafts.set(key,{decision:'accepted',identityMatch:'',confidence:'',observations:'',missingSections:'',summary:'',feedback:'',reason:'',pending:{}});
 const draft=drafts.get(key),names=['decision','identityMatch','confidence','observations','missingSections','summary','feedback','reason'];
 const d=drawer({title:'人工评审实际候选',body:`<p>请先查看实际候选再填写观察。此操作标记为Owner人工评审，不记录为模型视觉通过。</p>${VisualAsset(attempt.candidate_asset_id,'待人工评审的真实候选')}${Field({label:'评审决定',name:'decision',type:'select',value:draft.decision,options:[{value:'accepted',label:'接受这个候选'},{value:'regenerate',label:'需要重新生成'}]})}${[['identityMatch','身份符合程度'],['confidence','评审置信程度']].map(([name,label])=>Field({name,label:label+'（0—1）',type:'number',value:draft[name],attrs:{required:true,min:0,max:1,step:.01}})).join('')}${[['observations','你的实际观察（一行一项）',12000],['missingSections','缺失分区（一行一项，接受时必须为空）',3200],['summary','评审摘要',1000],['feedback','修正建议',2000],['reason','操作理由',1024]].map(([name,label,maxlength])=>Field({name,label,type:'textarea',value:draft[name],attrs:{required:['observations','summary','reason'].includes(name),maxlength}})).join('')}<p role="alert" data-error></p>`,footer:'<button data-domain-ui="button" class="btn" data-close>关闭</button><button data-domain-ui="button" class="btn primary" data-submit>提交人工评审</button>'});
 bindVisualAssetErrors(d.el);const submit=d.el.querySelector('[data-submit]'),error=d.el.querySelector('[data-error]');for(const name of names)d.el.querySelector(`[name=${name}]`).oninput=e=>draft[name]=e.target.value;d.el.querySelector('[data-close]').onclick=()=>d.close();
 const current=()=>Object.fromEntries(names.map(name=>[name,draft[name]]));
 const settled=(failure,saved,unchanged)=>{if(!d.el.isConnected)return;submit.disabled=!!draft.pending.review||!!draft.saved;if(failure)error.textContent=failure.message;else if(saved){if(unchanged)d.close();else error.textContent='原人工评审已提交，新输入保留；已决候选不能被覆盖，请重新读取下一阶段。';}else if(draft.pending.review)error.textContent='原评审正在提交，可继续编辑；等待实际结果后再操作。';};draft.onSettled=settled;settled();
 submit.onclick=async()=>{
  if(draft.pending.review||draft.saved)return;for(const name of names)if(!d.el.querySelector(`[name=${name}]`).reportValidity())return;
  const submission=beginDraftSubmission(draft,'review',current());draft.onSettled?.();
  const value=submission.value,lines=text=>text.split('\n').map(t=>t.trim()).filter(Boolean);
  try{const result=await visualClient.review(fid,session.id,{expectedAttempt:session.current_attempt,expectedVisualRevision:identity.current_revision,decision:value.decision,identityMatch:Number(value.identityMatch),confidence:Number(value.confidence),observations:lines(value.observations),missingSections:lines(value.missingSections),summary:value.summary,feedback:value.feedback,reason:value.reason});if(drafts.get(key)!==draft)return;const acknowledged=settleDraftSubmission(draft,submission,current());if(!acknowledged)return;draft.saved=true;if(acknowledged.unchanged)drafts.delete(key);draft.onSettled?.(null,result,acknowledged.unchanged);toast('人工评审已提交，后续任务按实际结果继续');try{await onSaved(result);}catch(e){toast(`评审已提交；重读失败：${e.message}`,'err');}}
  catch(e){if(drafts.get(key)===draft){settleDraftSubmission(draft,submission,null);draft.onSettled?.(e);}}
 };
 return d;
}

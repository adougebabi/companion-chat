import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {get,subscribe as subscribeProduct} from '../v3/proto/store.js';
import {persistentCommand} from '../v3/providers/persistent-command.mjs';
import {studioDraftStorage} from '../v3/proto/components/studio-draft-storage.js';
import {errorMessage} from '../v3/domain-presentation.mjs';
const drafts=new Map(),commands=new Map(),clone=value=>value===undefined?undefined:JSON.parse(JSON.stringify(value));
let previousOwner=get('session')?.actorId;
subscribeProduct(()=>{const owner=get('session')?.actorId;if(previousOwner&&previousOwner!==owner){drafts.clear();commands.clear();}previousOwner=owner;});
const memoryStorage={getItem:key=>commands.get(key)??null,setItem:(key,value)=>commands.set(key,value),removeItem:key=>commands.delete(key)};
/** Common command lifecycle only. Domain owners provide the exact body, APIs and baseline mapping. */
export function createDomainCommand({owner,scope,initial,build,execute,ownerCurrent,onSaved=()=>{},readLatest,adoptLatest,afterUnchanged,successMessage='原操作已有回执，请核对实际结果。',storageKey,commandKey,preAdmissionConflicts=['revision_conflict'],casOnly=false,validate=()=>{}}){
 const identity=JSON.stringify([owner,scope]),provider=storageKey?studioDraftStorage:()=>memoryStorage,key=commandKey||((storageKey||identity)+':command'),draftKey=storageKey?storageKey+':draft':identity;
 const outcomeKey=key+':outcome';
 let initialError='',restored,outcome={},draftStored=false;
 try{const raw=storageKey?provider().getItem(draftKey):drafts.get(draftKey);draftStored=!!raw;restored=raw?(storageKey?JSON.parse(raw):clone(raw)):clone(initial);if(!restored||typeof restored!=='object'||Array.isArray(restored))throw Error('操作草稿无法读取，请保留原记录核对');}catch(cause){initialError=errorMessage(cause);restored=null;}
 try{const raw=provider().getItem(outcomeKey);if(raw)outcome=JSON.parse(raw);if(!outcome||typeof outcome!=='object'||Array.isArray(outcome))throw Error('恢复记录格式无效');if(!draftStored&&outcome.accepted?.draftAtAck)restored=clone(outcome.accepted.draftAtAck);}catch(cause){initialError='原操作恢复记录无法读取：'+errorMessage(cause);}
 const state=proxy({draft:restored,pending:null,busy:false,error:initialError,notice:'',review:null,readbackPending:!!outcome.accepted,accepted:outcome.accepted||null,needsRebase:!!outcome.needsRebase,finished:false});
 let active=false,readEpoch=0,submittedDraft;
 const live=()=>active&&ownerCurrent();
 const command=persistentCommand({key,storage:provider,preAdmissionConflicts,execute:(value,key)=>{if(!live())throw Error('会话或对象范围已切换，请回原范围核对');return execute(value,key);}});
 function persist(){if(!live()||!state.draft)return false;try{if(storageKey)provider().setItem(draftKey,JSON.stringify(state.draft));else drafts.set(draftKey,clone(state.draft));return true;}catch(cause){state.error='操作草稿保存失败：'+errorMessage(cause);return false;}}
 function edit(values){if(!live()||!state.draft)return;if(state.finished){state.finished=false;state.needsRebase=true;}Object.assign(state.draft,values);persist();if(state.needsRebase)persistOutcome();}
 function persistOutcome(){if(!ownerCurrent())return;if(state.accepted||state.needsRebase)provider().setItem(outcomeKey,JSON.stringify({accepted:state.accepted,needsRebase:state.needsRebase}));else provider().removeItem(outcomeKey);}
 function syncOutcome(){const raw=provider().getItem(outcomeKey);if(!raw)return;const latest=JSON.parse(raw);state.accepted=clone(latest.accepted||null);state.readbackPending=!!latest.accepted;state.needsRebase=!!latest.needsRebase;state.pending=casOnly?null:command.pending();}
 function clearDraft(){if(storageKey)provider().removeItem(draftKey);else drafts.delete(draftKey);}
 async function finishReadback(){if(!state.accepted||!live())return;state.busy=true;try{await onSaved(clone(state.accepted.result),clone(state.accepted.value));if(!live())return;const unchanged=JSON.stringify(state.draft)===JSON.stringify(state.accepted.draftAtAck);state.finished=unchanged&&state.accepted.unchanged;state.needsRebase ||= !unchanged;state.readbackPending=false;state.accepted=null;persistOutcome();state.error='';}catch(cause){if(live())state.error='原操作已有回执；结果读取尚未完成：'+errorMessage(cause);}finally{if(live())state.busy=false;}}
 async function submit(retry=false){if(!live()||state.busy||!state.draft||state.finished||state.readbackPending)return;state.error='';state.notice='';let value,original;
  try{syncOutcome();if(state.readbackPending){state.notice='原操作已有回执，请只读恢复页面结果。';return;}original=(casOnly?null:command.pending());if(retry){if(!original){state.pending=null;state.error='原提交记录已变化，请重新读取对象。';return;}value=original.value;}else{if(original){state.error='请先核对原提交，当前输入仍保留。';return;}if(state.needsRebase){state.error='请先读取并确认最新对象，当前输入仍保留。';return;}validate(clone(state.draft));value=build(clone(state.draft));submittedDraft=clone(state.draft);if(!persist())return;if(casOnly)provider().setItem(outcomeKey,JSON.stringify({needsRebase:true}));}}
  catch(cause){state.error=errorMessage(cause);return;}
  state.busy=true;let acknowledged=false;
  try{const result=await(casOnly?execute(clone(value),undefined):retry?command.retry():command.submit(value));acknowledged=true;if(!ownerCurrent())return;if(!active){const latest=storageKey?provider().getItem(draftKey):drafts.get(draftKey);if(latest)state.draft=storageKey?JSON.parse(latest):clone(latest);}let unchanged=false;if(submittedDraft)unchanged=JSON.stringify(state.draft)===JSON.stringify(submittedDraft);else{try{unchanged=JSON.stringify(build(clone(state.draft)))===JSON.stringify(value);}catch{}}
   state.accepted={result:clone(result),value:clone(value),unchanged};state.readbackPending=true;
   if(unchanged){if(afterUnchanged)state.draft=afterUnchanged(clone(state.draft),clone(result));clearDraft();}else{state.needsRebase=true;persist();}
   state.accepted.draftAtAck=clone(state.draft);persistOutcome();if(!live())return;
   state.notice=unchanged?successMessage:'原操作已保存，等待期间的新输入仍保留，请核对最新版本后继续。';
   await onSaved(clone(result),clone(value));if(!live())return;const stillUnchanged=JSON.stringify(state.draft)===JSON.stringify(state.accepted.draftAtAck);state.needsRebase ||= !stillUnchanged;if(!stillUnchanged){persist();state.notice='原操作已保存，读回期间的新输入仍保留，请核对最新版本后继续。';}state.accepted=null;state.readbackPending=false;state.finished=unchanged&&stillUnchanged;persistOutcome();
  }catch(cause){if(ownerCurrent()){state.error=(acknowledged?'原操作已有回执；结果读取尚未完成：':'')+errorMessage(cause);if(casOnly&&!acknowledged&&![400,422].includes(cause.status)){state.needsRebase=true;persistOutcome();}}}finally{if(live()){state.busy=false;state.pending=(casOnly?null:command.pending());}}
 }
 async function reviewLatest(){if(!live()||state.busy||state.readbackPending||!readLatest||(casOnly?null:command.pending())&&!(casOnly?null:command.pending()).rejection)return;const token=++readEpoch;state.busy=true;try{const latest=await readLatest();if(live()&&token===readEpoch)state.review={object:clone(latest),key:(casOnly?null:command.pending())?.idempotencyKey};}catch(cause){if(live())state.error=errorMessage(cause);}finally{if(live()&&token===readEpoch)state.busy=false;}}
 function confirmLatest(){const review=state.review;if(!live()||!review||(casOnly?null:command.pending())?.idempotencyKey!==review.key)return;try{const next=adoptLatest?adoptLatest(clone(state.draft),clone(review.object)):state.draft;if((casOnly?null:command.pending())?.rejection)command.releaseRejected();state.draft=next;state.pending=(casOnly?null:command.pending());state.review=null;state.needsRebase=false;state.finished=false;submittedDraft=undefined;persist();persistOutcome();state.error='';state.notice='最新基线已明确确认，当前输入仍保留。';}catch(cause){state.error=errorMessage(cause);}}
 return {state,subscribe:listener=>subscribe(state,listener),getSnapshot:()=>snapshot(state),live,edit,persist,submit,finishReadback,reviewLatest,confirmLatest,start(){active=true;try{state.pending=(casOnly?null:command.pending());}catch(cause){state.error=errorMessage(cause);}},dispose(){active=false;readEpoch++;}};
}

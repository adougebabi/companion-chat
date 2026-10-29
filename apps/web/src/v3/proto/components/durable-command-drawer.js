import {errorMessage} from '../../domain-presentation.mjs';
import {get,subscribe} from '../store.js';
import {drawer,esc} from '../ui.js';
import {JsonDetails} from './records.js';
import {persistentCommand} from '../../providers/persistent-command.mjs';
import {studioDraftStorage} from './studio-draft-storage.js';

// Shared UI for commands whose unknown outcome must survive a reload. This
// retains the frozen input and its receipt until readback; neither is a
// second authoritative business snapshot.
export function durableCommandDrawer({slot,title,body,read,execute,onSaved=()=>{},button='确认提交',destructive=true,originalOnly=false,onPendingChange=()=>{},preAdmissionConflicts=['revision_conflict']}){
 const owner=get('session')?.actorId,key=`fluctlight-studio:${owner}:${slot}:command`,acceptedKey=key+':accepted';
 let active=true,busy=false,sessionValid=true,submittedInputs=null;
 // A logout followed by re-entry into the same Owner is a new session. An old
 // response must neither recreate its cleared slot nor clear a newer command.
 const unsubscribe=subscribe(()=>{if(get('session')?.actorId!==owner)sessionValid=false;});
 const current=()=>sessionValid&&get('session')?.actorId===owner;
 const storage=()=>({getItem:name=>current()?studioDraftStorage().getItem(name):null,setItem:(name,value)=>{if(current())studioDraftStorage().setItem(name,value);},removeItem:name=>{if(current())studioDraftStorage().removeItem(name);}});
 const command=persistentCommand({key,storage,preAdmissionConflicts,execute:async(value,idempotencyKey)=>{
  if(!current())throw new Error('会话已切换，请回到原会话核对');
  const result=await execute(value,idempotencyKey);
  const original=command.pending();
  if(current()&&original?.idempotencyKey===idempotencyKey)storage().setItem(acceptedKey,JSON.stringify({version:1,idempotencyKey,value,result,inputSnapshot:submittedInputs}));
  return result;
 }});
 const d=drawer({title,body:`${body}<div data-original></div><p data-domain-ui="alert" data-feedback-kind="information" hidden data-command-error></p><div data-command-diagnostic></div>`,footer:`<button data-domain-ui="button" class="btn" data-command-close>关闭</button><button data-domain-ui="button" class="btn" data-command-retry>核对或重试原提交</button><button data-domain-ui="button" class="btn" data-command-release hidden>解除已明确拒绝的原提交</button><button data-domain-ui="button" class="btn ${destructive?'danger':'primary'}" data-command-send>${esc(button)}</button>`,onClose:()=>{active=false;if(!busy)unsubscribe();}});
 const alive=()=>active&&d.el.isConnected&&current();
 let storageBlocked=false;
 function feedback(kind,text='',cause){
  if(!alive())return;const host=d.el.querySelector('[data-command-error]');host.setAttribute('data-feedback-kind',kind);host.textContent=text;host.hidden=!text;
  d.el.querySelector('[data-command-diagnostic]').innerHTML=cause?JsonDetails({name:cause.name,code:cause.code,message:cause.message},'技术详情'):'';
 }
 function accepted(){const raw=storage().getItem(acceptedKey);if(raw==null)return null;const result=JSON.parse(raw);if(result?.version!==1||!result.idempotencyKey||!result.value||typeof result.value!=='object'||Array.isArray(result.value)||!Object.hasOwn(result,'result'))throw Error('原回执记录无法读取，请先核对原记录；没有发起新提交。');return {raw,...result};}
 function readPending(){const raw=storage().getItem(key);if(raw!==null&&raw!==undefined&&!raw)throw Error('原提交记录为空，当前不能安全发起新提交。');return command.pending();}
 function paint(){
  if(!alive())return;let receipt,pending;
  try{receipt=accepted();pending=readPending();if(storageBlocked)feedback('information');storageBlocked=false;}
  catch(cause){storageBlocked=true;onPendingChange(true);feedback('error','原提交记录暂时无法读取，当前不会发起新提交。原记录仍保留，可在记录可读后重新读取。',cause);d.el.querySelector('[data-command-send]').disabled=true;d.el.querySelector('[data-command-send]').hidden=originalOnly;d.el.querySelector('[data-command-release]').hidden=true;const retry=d.el.querySelector('[data-command-retry]');retry.hidden=false;retry.disabled=busy;retry.textContent='重新读取原提交记录';return;}
  onPendingChange(!!receipt||!!pending);
  d.el.querySelector('[data-original]').innerHTML=receipt?JsonDetails(receipt.value,'原提交'):pending?JsonDetails(pending.value,'原提交'):'';
  const status=d.el.querySelector('[data-command-error]');
  if(!busy&&!status.textContent){if(receipt)feedback('action-required','原操作已有回执，请只读恢复页面结果。');else if(pending)feedback(pending.rejection?'warning':'action-required',pending.rejection?'原提交已明确被拒绝。原输入保留，可解除该提交后核对新草稿。':'原提交结果仍待核对，原输入和身份已保留。');else if(originalOnly)feedback('information','没有待核对的原提交。请关闭后先校验当前配置包，再创建导入。');}
  d.el.querySelector('[data-command-send]').disabled=busy||!!receipt||!!pending||originalOnly;d.el.querySelector('[data-command-send]').hidden=originalOnly;const retry=d.el.querySelector('[data-command-retry]');retry.textContent=receipt?'重新读取原结果':'核对或重试原提交';retry.hidden=!receipt&&!pending;retry.disabled=busy||!receipt&&(!pending||!!pending.rejection);d.el.querySelector('[data-command-release]').hidden=!!receipt||!pending?.rejection;d.el.querySelector('[data-command-release]').disabled=busy;
 }
 const inputs=()=>JSON.stringify([...d.el.querySelectorAll('input[name],select[name],textarea[name]')].filter(field=>!field.closest('[data-domain-react-control]')).map(field=>[field.name,field.value,field.checked]));
 async function send(retry){
  if(busy||!alive())return;
  let receipt;try{receipt=accepted();readPending();}catch{paint();return;}if(!retry&&(originalOnly||receipt))return;
  if(!retry&&![...d.el.querySelectorAll('input,select,textarea')].every(field=>field.reportValidity()))return;
  if(!receipt&&!retry)submittedInputs=inputs();busy=true;feedback('progress',receipt?'正在读取原操作结果…':retry?'正在核对原提交，使用原输入和身份…':'正在提交，等待实际回执…');paint();
  try{
   if(!receipt){if(retry)await command.retry();else await command.submit(read(d.el));}
   if(!alive())return;
   const confirmed=accepted();if(!confirmed)throw Error('原回执已变化，请关闭后核对原结果。');
   try{
    await onSaved(structuredClone(confirmed.result));
    if(current()&&storage().getItem(acceptedKey)===confirmed.raw)storage().removeItem(acceptedKey);
    if(alive()){let unchanged=confirmed.inputSnapshot!==null&&confirmed.inputSnapshot!==undefined?inputs()===confirmed.inputSnapshot:false;if(confirmed.inputSnapshot==null)try{unchanged=JSON.stringify(read(d.el))===JSON.stringify(confirmed.value);}catch{}if(unchanged)d.close();else feedback('information','原操作已保存；等待期间的新输入仍保留。');}
   }catch(error){if(alive())feedback('action-required','原操作已有回执，页面结果暂未读到。可以只读恢复，不会再次提交。',error);}
  }catch(error){if(alive()){let pending;try{pending=command.pending();}catch{}feedback(pending?(pending.rejection?'warning':'action-required'):'error',errorMessage(error),error);}}
  finally{busy=false;if(!active)unsubscribe();paint();}
 }
 d.el.querySelector('[data-command-close]').onclick=()=>d.close();d.el.querySelector('[data-command-send]').onclick=()=>void send(false);d.el.querySelector('[data-command-retry]').onclick=()=>storageBlocked?paint():void send(true);d.el.querySelector('[data-command-release]').onclick=async()=>{try{const original=command.pending();await onSaved();if(!alive())return;if(command.pending()?.idempotencyKey!==original?.idempotencyKey)throw Error('原提交记录已变化，请重新读取后核对。');command.releaseRejected();d.close();}catch(error){if(alive())feedback('error',errorMessage(error),error);}};paint();return d;
}

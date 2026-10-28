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
 const d=drawer({title,body:`${body}<div data-original></div><p data-domain-ui="alert" role="alert" data-command-error></p>`,footer:`<button data-domain-ui="button" class="btn" data-command-close>关闭</button><button data-domain-ui="button" class="btn" data-command-retry>核对或重试原提交</button><button data-domain-ui="button" class="btn" data-command-release hidden>核对新状态后解除已拒绝的原提交</button><button data-domain-ui="button" class="btn ${destructive?'danger':'primary'}" data-command-send>${esc(button)}</button>`,onClose:()=>{active=false;if(!busy)unsubscribe();}});
 const alive=()=>active&&d.el.isConnected&&current();
 function accepted(){const raw=storage().getItem(acceptedKey);if(!raw)return null;const result=JSON.parse(raw);if(result?.version!==1||!result.idempotencyKey||!result.value||!Object.hasOwn(result,'result'))throw Error('原回执记录无法读取，请先核对原记录；没有发起新提交。');return {raw,...result};}
 function paint(){if(!alive())return;const receipt=accepted(),pending=command.pending();onPendingChange(!!receipt||!!pending);d.el.querySelector('[data-original]').innerHTML=receipt?`<p>提交已有回执；只重新读取原结果，不再提交修改。</p>${JsonDetails(receipt.value,'原提交')}`:pending?`<p>原提交身份和输入已保留，先核对它的结果再发起新提交。</p>${JsonDetails(pending.value,'原提交')}`:originalOnly?'<p data-domain-ui="alert" class="banner info">没有待核对的原提交。请关闭后先校验配置包，再创建导入。</p>':'';d.el.querySelector('[data-command-send]').disabled=busy||!!receipt||!!pending||originalOnly;d.el.querySelector('[data-command-send]').hidden=originalOnly;const retry=d.el.querySelector('[data-command-retry]');retry.textContent=receipt?'重新读取原结果':'核对或重试原提交';retry.hidden=!receipt&&!pending;retry.disabled=busy||!receipt&&(!pending||!!pending.rejection);d.el.querySelector('[data-command-release]').hidden=!!receipt||!pending?.rejection;d.el.querySelector('[data-command-release]').disabled=busy;}
 const inputs=()=>JSON.stringify([...d.el.querySelectorAll('input[name],select[name],textarea[name]')].filter(field=>!field.closest('[data-domain-react-control]')).map(field=>[field.name,field.value,field.checked]));
 async function send(retry){
  if(busy||!alive())return;
  const receipt=accepted();if(!retry&&(originalOnly||receipt))return;
  if(!retry&&![...d.el.querySelectorAll('input,select,textarea')].every(field=>field.reportValidity()))return;
  if(!receipt&&!retry)submittedInputs=inputs();busy=true;paint();
  try{
   if(!receipt){if(retry)await command.retry();else await command.submit(read(d.el));}
   if(!alive())return;
   const confirmed=accepted();if(!confirmed)throw Error('原回执已变化，请关闭后核对原结果。');
   try{
    await onSaved(structuredClone(confirmed.result));
    if(current()&&storage().getItem(acceptedKey)===confirmed.raw)storage().removeItem(acceptedKey);
    if(alive()){let unchanged=confirmed.inputSnapshot!==null&&confirmed.inputSnapshot!==undefined?inputs()===confirmed.inputSnapshot:false;if(confirmed.inputSnapshot==null)try{unchanged=JSON.stringify(read(d.el))===JSON.stringify(confirmed.value);}catch{}if(unchanged)d.close();else d.el.querySelector('[data-command-error]').textContent='原操作已保存；等待期间的新输入仍保留。';}
   }catch(error){if(alive())d.el.querySelector('[data-command-error]').textContent='提交已有回执；重新读取失败：'+errorMessage(error);}
  }catch(error){if(alive())d.el.querySelector('[data-command-error]').textContent=errorMessage(error);}
  finally{busy=false;if(!active)unsubscribe();paint();}
 }
 d.el.querySelector('[data-command-close]').onclick=()=>d.close();d.el.querySelector('[data-command-send]').onclick=()=>void send(false);d.el.querySelector('[data-command-retry]').onclick=()=>void send(true);d.el.querySelector('[data-command-release]').onclick=async()=>{try{command.releaseRejected();await onSaved();if(alive())d.close();}catch(error){if(alive())d.el.querySelector('[data-command-error]').textContent=errorMessage(error);}};paint();return d;
}

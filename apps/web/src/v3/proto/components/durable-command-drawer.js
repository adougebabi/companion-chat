import {errorMessage} from '../../domain-presentation.mjs';
import {get} from '../store.js';
import {drawer,esc} from '../ui.js';
import {JsonDetails} from './records.js';
import {persistentCommand} from '../../providers/persistent-command.mjs';
import {studioDraftStorage} from './studio-draft-storage.js';

// Shared UI for commands whose unknown outcome must survive a reload. This
// persists only the original submitted input, never domain state.
export function durableCommandDrawer({slot,title,body,read,execute,onSaved=()=>{},button='确认提交',originalOnly=false,onPendingChange=()=>{},preAdmissionConflicts=['revision_conflict']}){
 const owner=get('session')?.actorId;let active=true,busy=false;
 const command=persistentCommand({key:`fluctlight-studio:${owner}:${slot}:command`,storage:studioDraftStorage,preAdmissionConflicts,execute:(value,key)=>{if(get('session')?.actorId!==owner)throw new Error('会话已切换，请回到原会话核对');return execute(value,key);}});
 const d=drawer({title,body:`${body}<div data-original></div><p role="alert" data-command-error></p>`,footer:`<button data-domain-ui="button" class="btn" data-command-close>关闭</button><button data-domain-ui="button" class="btn" data-command-retry>核对或重试原提交</button><button data-domain-ui="button" class="btn" data-command-release hidden>核对新状态后解除已拒绝的原提交</button><button data-domain-ui="button" class="btn danger" data-command-send>${esc(button)}</button>`,onClose:()=>{active=false;}});
 const alive=()=>active&&d.el.isConnected&&get('session')?.actorId===owner;
 function paint(){if(!alive())return;const pending=command.pending();onPendingChange(!!pending);d.el.querySelector('[data-original]').innerHTML=pending?`<p>原提交身份和输入已保留，先核对它的结果再发起新提交。</p>${JsonDetails(pending.value,'原提交')}`:originalOnly?'<p class="banner info">没有待核对的原提交。请关闭后先校验配置包，再创建导入。</p>':'';d.el.querySelector('[data-command-send]').disabled=busy||!!pending||originalOnly;d.el.querySelector('[data-command-send]').hidden=originalOnly;d.el.querySelector('[data-command-retry]').hidden=!pending;d.el.querySelector('[data-command-retry]').disabled=busy||!pending||!!pending.rejection;d.el.querySelector('[data-command-release]').hidden=!pending?.rejection;d.el.querySelector('[data-command-release]').disabled=busy;}
 async function send(retry){if(busy||!alive()||!retry&&originalOnly)return;if(!retry&&![...d.el.querySelectorAll('input,select,textarea')].every(field=>field.reportValidity()))return;busy=true;paint();try{const result=retry?await command.retry():await command.submit(read(d.el));if(!alive())return;try{await onSaved(result);if(alive())d.close();}catch(error){if(alive())d.el.querySelector('[data-command-error]').textContent='提交已有回执；重新读取失败：'+errorMessage(error);}}catch(error){if(alive())d.el.querySelector('[data-command-error]').textContent=errorMessage(error);}finally{busy=false;paint();}}
 d.el.querySelector('[data-command-close]').onclick=()=>d.close();d.el.querySelector('[data-command-send]').onclick=()=>void send(false);d.el.querySelector('[data-command-retry]').onclick=()=>void send(true);d.el.querySelector('[data-command-release]').onclick=async()=>{try{command.releaseRejected();await onSaved();if(alive())d.close();}catch(error){if(alive())d.el.querySelector('[data-command-error]').textContent=errorMessage(error);}};paint();return d;
}

import {inventoryClient} from '../services.js';
import {esc} from '../ui.js';
import {governanceCommand} from './governance-command.js';

const labels={not_started:'尚未准备初始穿搭',queued:'初始穿搭已排队',generating:'正在准备服装和方案',awaiting_wear:'方案已准备，等待按当前情境穿着',completed:'初始穿搭准备已完成',failed:'初始穿搭准备未完成'};
export function mountWardrobePreparation(box,{fid,alive=()=>box.isConnected,onCompleted=()=>{}}){
 let active=true,timer=null,reads=0,busy=false,previous=null;const live=()=>active&&alive();
 async function load(){
  if(!live()||busy)return;busy=true;clearTimeout(timer);
  try{const state=await inventoryClient.preparation(fid);if(!live())return;
   box.innerHTML=`<section class="card stack"><div class="row"><div class="grow"><h3>${esc(state.can_recover?'准备任务已停止，可继续恢复':labels[state.status]||'穿搭准备状态待核对')}</h3><p class="small muted">准备会结合人格、当前情境和已有服装。保存方案、实际穿上、之后的穿着变化分别记录。</p></div><button class="btn sm" data-refresh>重新读取</button>${(['not_started','failed'].includes(state.status)||state.can_recover)?`<button class="btn primary" data-prepare>${state.status==='failed'||state.can_recover?'恢复准备任务':'让她准备初始穿搭'}</button>`:''}</div>${state.result?.reason?`<p>${esc(state.result.reason)}</p>`:''}${state.task_id?`<a class="small" href="#/diag/task/${encodeURIComponent(state.task_id)}">查看准备进度与执行记录</a>`:''}${state.status==='failed'?`<p><a class="btn sm" href="#/diag/events?eventType=wardrobe.preparation.failed&q=${encodeURIComponent(state.id)}">查看本次失败原因</a></p>`:''}${state.status==='completed'?'<p class="small muted">这是初始准备的完成记录；此刻穿着以上方实际读回为准，不会因后来脱下衣服而重新初始化。</p>':''}</section>`;
   box.querySelector('[data-refresh]').onclick=()=>{reads=0;void load();};
   box.querySelector('[data-prepare]')?.addEventListener('click',async()=>{try{const current=await inventoryClient.current(fid);if(!live())return;governanceCommand({scope:[fid,'wardrobe-preparation'],title:'准备初始穿搭',defaultReason:'按当前人格和情境补全初始穿搭',note:'由独立穿搭 Agent 使用现有服装并补充必要的初始衣物，保存方案后通过真实穿衣工具执行。任务在后台运行。',revision:current.revision,read:()=>inventoryClient.current(fid),execute:body=>inventoryClient.prepare(fid,body),successMessage:'准备任务已受理，实际结果将在这里更新',onSaved:()=>{reads=0;void load();}});}catch(error){if(live())box.querySelector('section').insertAdjacentHTML('beforeend',`<p class="banner err">${esc(error.message)}</p>`);}});
   if(previous&&previous!=='completed'&&state.status==='completed')void onCompleted();previous=state.status;
   if(['queued','generating','awaiting_wear'].includes(state.status)&&++reads<40&&!document.hidden)timer=setTimeout(()=>void load(),3000);
  }catch(error){if(live()){box.innerHTML=`<section class="card"><p role="alert">穿搭准备状态读取失败：${esc(error.message)}</p><button class="btn sm" data-retry>重新读取</button></section>`;box.querySelector('[data-retry]').onclick=()=>void load();}}
  finally{busy=false;}
 }
 const visibility=()=>{clearTimeout(timer);if(!document.hidden&&reads<40)void load();};document.addEventListener('visibilitychange',visibility);void load();
 return {dispose(){active=false;clearTimeout(timer);document.removeEventListener('visibilitychange',visibility);}};
}

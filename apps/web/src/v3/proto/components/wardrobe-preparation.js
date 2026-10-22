import {errorMessage} from '../../domain-presentation.mjs';
import {JsonDetails} from './records.js';
import {inventoryClient} from '../services.js';
import {esc} from '../ui.js';
import {governanceCommand} from './governance-command.js';

const labels={not_started:'尚未准备初始穿搭',queued:'初始穿搭已排队',generating:'正在准备服装和方案',awaiting_wear:'方案已准备，等待按当前情境穿着',completed:'初始穿搭准备已完成',failed:'初始穿搭准备未完成'};
const statusLabels={not_started:'尚未开始',queued:'已排队',generating:'准备中',awaiting_wear:'等待穿着',completed:'已完成',failed:'未完成'};
export function mountWardrobePreparation(box,{fid,alive=()=>box.isConnected,onCompleted=()=>{},expanded=false,onToggle=()=>{}}){
 let active=true,timer=null,reads=0,busy=false,previous=null,rendered='';const live=()=>active&&alive();
 box.innerHTML=`<details class="wardrobe-preparation preparation-disclosure" ${expanded?'open':''}><summary><span class="preparation-summary-copy"><strong>初始穿搭准备与任务</strong><span data-preparation-subtitle>正在读取准备进度…</span></span><span class="preparation-status" data-preparation-status role="status">读取中</span><span class="preparation-chevron" aria-hidden="true"><svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="m6 9 6 6 6-6"/></svg></span></summary><section class="preparation-body" data-preparation-body><p class="small muted">正在读取准备状态…</p></section></details>`;
 const disclosure=box.querySelector('details'),body=box.querySelector('[data-preparation-body]');
 disclosure.addEventListener('toggle',()=>onToggle(disclosure.open));
 function feedback(message){let notice=body.querySelector('[data-preparation-error]');if(!notice){notice=document.createElement('p');notice.className='banner err';notice.dataset.preparationError='';notice.setAttribute('role','alert');body.append(notice);}notice.textContent=message;}
 async function load(){
  if(!live()||busy)return;busy=true;clearTimeout(timer);
  try{const state=await inventoryClient.preparation(fid);if(!live())return;
   const status=box.querySelector('[data-preparation-status]');
   status.textContent=state.can_recover?'可恢复':statusLabels[state.status]||'待核对';
   status.dataset.state=state.can_recover?'recoverable':state.status;
   box.querySelector('[data-preparation-subtitle]').textContent=state.can_recover?'准备任务已停止，可继续恢复':labels[state.status]||'穿搭准备状态待核对';
   if(state.status==='failed'||state.error_code)disclosure.open=true;
   const next=JSON.stringify(state);
   if(next!==rendered){
    rendered=next;
    body.innerHTML=`<dl class="preparation-facts"><dt>当前进度</dt><dd>${esc(state.can_recover?'准备任务已停止，可继续恢复':labels[state.status]||'穿搭准备状态待核对')}</dd><dt>准备依据</dt><dd>当前人格、当前情境与已有服装</dd><dt>结果记录</dt><dd>保存方案、实际穿上、之后的穿着变化分别记录。</dd>${state.result?.reason?`<dt>处理说明</dt><dd>${esc(errorMessage({message:state.result.reason},'本次准备未形成可用穿搭，请核对原任务。'))}</dd>`:''}${state.task_id?`<dt>实际任务</dt><dd><a href="#/diag/task/${encodeURIComponent(state.task_id)}">查看准备进度与执行记录</a></dd>`:''}${state.status==='failed'||state.error_code?`<dt>失败原因</dt><dd class="banner warn" role="alert">${esc(errorMessage({code:state.error_code,message:state.result?.reason},'初始穿搭准备未完成。请检查运行服务后，再明确恢复准备任务。'))}</dd>`:''}</dl>${JsonDetails(state,'原准备任务与完整处理结果')}${state.status==='completed'?'<p class="small muted preparation-note">这是初始准备的完成记录；此刻穿着以实际读回为准，不会因后来脱下衣服而重新初始化。</p>':''}<div class="preparation-actions"><button class="btn sm" data-refresh>重新读取</button>${(['not_started','failed'].includes(state.status)||state.can_recover)?`<button class="btn primary sm" data-prepare>${state.status==='failed'||state.can_recover?'恢复准备任务':'让她准备初始穿搭'}</button>`:''}</div>`;
    body.querySelector('[data-refresh]').onclick=()=>{reads=0;void load();};
    body.querySelector('[data-prepare]')?.addEventListener('click',async()=>{try{const current=await inventoryClient.current(fid);if(!live())return;governanceCommand({scope:[fid,'wardrobe-preparation'],title:'准备初始穿搭',defaultReason:'按当前人格和情境补全初始穿搭',note:'由独立穿搭 Agent 使用现有服装并补充必要的初始衣物，保存方案后通过真实穿衣工具执行。任务在后台运行。',revision:current.revision,read:()=>inventoryClient.current(fid),execute:body=>inventoryClient.prepare(fid,body),successMessage:'准备任务已受理，实际结果将在这里更新',onSaved:()=>{reads=0;void load();}});}catch(error){if(live())feedback(errorMessage(error));}});
   }
   body.querySelector('[data-preparation-read-error]')?.remove();
   if(previous&&previous!=='completed'&&state.status==='completed')void onCompleted();previous=state.status;
   if(['queued','generating','awaiting_wear'].includes(state.status)&&++reads<40&&!document.hidden)timer=setTimeout(()=>void load(),3000);
  }catch(error){if(live()){
   if(!rendered){box.querySelector('[data-preparation-status]').textContent='读取失败';box.querySelector('[data-preparation-subtitle]').textContent='准备状态暂不可读取';}
   let notice=body.querySelector('[data-preparation-read-error]');
   if(!notice){notice=document.createElement('div');notice.dataset.preparationReadError='';notice.className='preparation-read-error';notice.innerHTML='<p role="alert"></p><button class="btn sm" data-retry>重新读取</button>';body.append(notice);notice.querySelector('[data-retry]').onclick=()=>{reads=0;void load();};}
   notice.querySelector('p').textContent=`穿搭准备状态读取失败${rendered?'，保留上次读取内容':''}：${errorMessage(error)}`;
  }}finally{busy=false;}
 }
 const visibility=()=>{clearTimeout(timer);if(!document.hidden&&reads<40)void load();};document.addEventListener('visibilitychange',visibility);void load();
 return {dispose(){active=false;clearTimeout(timer);document.removeEventListener('visibilitychange',visibility);}};
}

import {errorMessage} from '../../domain-presentation.mjs';
import { esc, drawer } from '../ui.js';
import {InventoryEffects} from './inventory-effects.js';
import { lifeClient } from '../services.js';
import {userInstant} from '../../presentation-time.mjs';
import {requestDecisionText,RequestFailureLink,RequestDecisionSource} from './request-failure.js';

export function asyncPage(el, read, paint, {backHref='',backLabel='返回',errorTitle='读取失败',errorFallback='暂时无法读取，请重新尝试。'}={}) {
  let active=true, epoch=0, dispose;
  const alive=()=>active && el.isConnected;
  async function reload() {
    if(!alive())return;
    const version=++epoch;
    dispose?.(); dispose=null;
    el.innerHTML='<div class="statebox" role="status">正在读取…</div>';
    try {const value=await read();if(alive() && version===epoch)dispose=paint(value,reload,()=>alive()&&version===epoch);}
    catch(error) {if(!alive() || version!==epoch)return;el.innerHTML=`<div class="statebox" role="alert"><h2>${esc(errorTitle)}</h2><p>${esc(errorMessage(error,errorFallback))}</p>${backHref?`<a class="btn" href="${esc(backHref)}">${esc(backLabel)}</a>`:''}<button class="btn" data-read-retry>重新读取</button></div>`;el.querySelector('[data-read-retry]').onclick=reload;}
  }
  void reload();
  return()=>{active=false;epoch++;dispose?.();};
}

export function instant(value,zone) {
  return userInstant(value,zone);
}
const requestLabels={pending:'已受理',processing:'正在判断',accepted:'已接受，履行待证据',completed:'执行已提交',rejected:'未接受',deferred:'暂缓，尚未执行',awaiting_goal:'等待目标规划，尚未接受',cancelled:'已取消，事实与历史保留',failed:'执行失败'};
// Observation only. Closing the drawer stops polling, never cancels the job.
export function requestReceipt(fid,envelope,onTerminal=()=>{},options={}) {
  const d=drawer({title:options.title||'行动请求',body:'<div data-request></div>',footer:'<button class="btn" data-query>重新查询</button><button class="btn" data-close>关闭</button>'});
  let request=envelope.request, timer=null, reads=0, terminalNotified=false;
  const terminal=()=>!['pending','processing'].includes(request.status);
  function paint(error='') {
    if(!d.el.isConnected)return;
    d.el.querySelector('[data-request]').innerHTML=`<p><b>${esc(options.labels?.[request.status] || (request.status==='completed'&&request.result?.disposition==='no_change'?'目标状态已满足，无新增动作':requestLabels[request.status]) || request.status)}</b></p><p>${esc(requestDecisionText(request))}</p><p class="small muted">请求 ${esc(request.id)}</p>${request.result_refs?.length?`<p class="small">结果来源 ${request.result_refs.map(esc).join('、')}</p>`:''}<p class="small muted">${esc(options.note||'受理不代表执行完成。关闭后请求继续处理。')}</p><p><a href="#/diag/request/${encodeURIComponent(request.id)}">请求详情与关联任务</a></p>${RequestFailureLink(request)}${RequestDecisionSource(request)}${InventoryEffects(fid,request.result?.effects)}${request.tasks_state==='failed'?'<p class="banner warn">关联任务读取失败，请重新查询。</p>':request.tasks?.map(t=>`<p><a href="#/diag/task/${encodeURIComponent(t.task_id)}">${esc(t.kind)} · ${esc(t.status)}</a></p>`).join('')||''}${error?`<p class="banner err" role="alert">${esc(error)}</p>`:''}`;
    if(terminal()&&!terminalNotified){terminalNotified=true;void onTerminal(request);}
  }
  async function query() {
    clearTimeout(timer);if(!d.el.isConnected)return;
    try {request=await lifeClient.request(fid,request.id);paint();if(!terminal()&&++reads<60)timer=setTimeout(query,2000);}
    catch(error){paint(error.message);}
  }
  d.el.querySelector('[data-query]').onclick=()=>{reads=0;void query();};
  d.el.querySelector('[data-close]').onclick=()=>{clearTimeout(timer);d.close();};
  paint();if(!terminal())timer=setTimeout(query,2000);
  return d;
}

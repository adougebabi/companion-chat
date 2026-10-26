import {errorMessage,domainName,stateLabel} from '../../domain-presentation.mjs';
import {get} from '../store.js';
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
    el.innerHTML='<div data-domain-ui="skeleton" class="statebox" role="status">正在读取…</div>';
    try {const value=await read();if(alive() && version===epoch)dispose=paint(value,reload,()=>alive()&&version===epoch);}
    catch(error) {if(!alive() || version!==epoch)return;el.innerHTML=`<div class="statebox" role="alert"><h2>${esc(errorTitle)}</h2><p>${esc(errorMessage(error,errorFallback))}</p>${backHref?`<a data-domain-ui="button" class="btn" href="${esc(backHref)}">${esc(backLabel)}</a>`:''}<button data-domain-ui="button" class="btn" data-read-retry>重新读取</button></div>`;el.querySelector('[data-read-retry]').onclick=reload;}
  }
  void reload();
  return()=>{active=false;epoch++;dispose?.();};
}

export function instant(value,zone) {
  return userInstant(value,zone);
}
const requestLabels={pending:'等她回应',processing:'她正在考虑',accepted:'她已答应，等待实际行动',completed:'本次处理已结束',rejected:'她没有接受',deferred:'她想稍后再决定',awaiting_goal:'正在考虑如何安排',cancelled:'已取消',failed:'这次没有完成'};
// Observation only. Inline hosts and drawers share the same original request.
export function requestReceipt(fid,envelope,onTerminal=()=>{},options={}) {
  let active=true;
  const owner=get('session')?.actorId;
  const d=options.container?{el:options.container}:drawer({title:options.title||'处理进度',body:'<div data-request></div>',footer:'<button data-domain-ui="button" class="btn" data-query>刷新进度</button><button data-domain-ui="button" class="btn" data-close>关闭</button>',onClose:()=>dispose()});
  if(options.container)d.el.innerHTML='<section class="card stack" aria-live="polite"><div data-request></div><button data-domain-ui="button" type="button" class="btn" data-query>刷新进度</button></section>';
  let request=envelope.request, timer=null, reads=0, terminalNotified=false,busy=false;
  const live=()=>active&&d.el.isConnected&&get('session')?.actorId===owner&&(options.alive?.()??true);
  const terminal=()=>!['pending','processing'].includes(request.status);
  function paint(error='') {
    if(!live())return;
    d.el.querySelector('[data-request]').innerHTML=`<p><b>${esc(options.labels?.[request.status] || (request.status==='completed'&&request.result?.disposition==='no_change'?'目标状态已满足，无新增动作':requestLabels[request.status]) || stateLabel(request.status))}</b></p><p>${esc(requestDecisionText(request))}</p>${request.result_refs?.length?`<p class="small">结果来源 ${request.result_refs.map(esc).join('、')}</p>`:''}<p class="small muted">${esc(options.note||'她会自行决定是否行动。结果会保留，也可以在通知中查看。')}</p><p><a href="#/diag/request/${encodeURIComponent(request.id)}">请求详情与关联任务</a></p>${RequestFailureLink(request)}${RequestDecisionSource(request)}${InventoryEffects(fid,request.result?.effects)}${request.tasks_state==='failed'?'<p class="banner warn">关联任务读取失败，请重新查询。</p>':request.tasks?.map(t=>`<p><a href="#/diag/task/${encodeURIComponent(t.task_id)}">${esc(domainName(t.kind))} · ${esc(stateLabel(t.status))}</a></p>`).join('')||''}${error?`<p class="banner err" role="alert">${esc(error)}</p>`:''}`;
    if(request.status==='failed'&&request.result?.mutation_committed===false&&options.onRetry){const button=document.createElement('button');button.type='button';button.className='btn';button.dataset.domainUi='button';button.textContent='重试这条建议';button.onclick=async()=>{button.disabled=true;try{const latest=await lifeClient.request(fid,request.id);if(!live())return;if(latest.status!=='failed'||latest.result?.mutation_committed!==false){request=latest;paint();return;}await options.onRetry(latest);}catch(error){if(live()){paint(errorMessage(error));}}};d.el.querySelector('[data-request]').append(button);}
    if(reads>=150&&!terminal())d.el.querySelector('[data-request]').insertAdjacentHTML('beforeend','<p class="small muted">自动更新已暂停。你可以稍后回来，或点击刷新进度。</p>');
    if(terminal()&&!terminalNotified){terminalNotified=true;Promise.resolve().then(()=>{if(live())return onTerminal(request);}).catch(error=>paint(errorMessage(error)));}
  }
  async function query() {
    clearTimeout(timer);if(!live()||busy||globalThis.document?.hidden)return;busy=true;
    try {const next=await lifeClient.request(fid,request.id);if(!live())return;request=next;paint();}
    catch(error){paint('更新失败，上次状态已保留。'+errorMessage(error));}
    finally{busy=false;reads++;if(live()&&!terminal()&&!globalThis.document?.hidden&&reads<150)timer=setTimeout(query,2000);else if(live()&&reads>=150)paint();}
  }
  function visible(){clearTimeout(timer);if(!globalThis.document?.hidden&&reads<150)void query();}
  function dispose(){active=false;clearTimeout(timer);globalThis.document?.removeEventListener('visibilitychange',visible);}
  globalThis.document?.addEventListener('visibilitychange',visible);
  d.el.querySelector('[data-query]').onclick=()=>{reads=0;void query();};
  d.el.querySelector('[data-close]')?.addEventListener('click',()=>{dispose();d.close();});
  paint();if(!terminal())timer=setTimeout(query,2000);
  return {...d,dispose};
}

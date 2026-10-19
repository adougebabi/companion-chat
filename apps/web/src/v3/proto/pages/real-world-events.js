import * as store from '../store.js';
import {mockWorld} from '../services.js';
import {esc,badge} from '../ui.js';
import {asyncPage,instant} from '../components/async-page.js';
import {mountCursorRecords} from '../components/cursor-records.js';
import {JsonDetails} from '../components/records.js';
import {WorldEventCard} from '../components/world-events.js';

function scope(el,ctx){const owner=store.get('session')?.actorId,fid=ctx.charId;return {fid,zone:store.get('characters',fid)?.timezone,alive:()=>el.isConnected&&store.get('session')?.actorId===owner};}
export function renderRealWorldEvents(el,ctx){
 const s=scope(el,ctx);el.innerHTML=`<div class="card"><h3>世界变化与事件流</h3><p class="small muted">当前运行世界中的真实事件，并保留未记录世界归属的角色历史。浏览不会触发事件。计划、模板和被过滤的候选可在运行模板中分别查询。</p><a class="btn" href="#/c/${encodeURIComponent(s.fid)}/world/settings">运行模板与候选记录</a></div><div class="stack" data-events></div>`;
 const panel=mountCursorRecords(el.querySelector('[data-events]'),{alive:s.alive,read:cursor=>mockWorld.events(s.fid,cursor),render:e=>WorldEventCard(e,s),label:'世界事件',empty:'尚无实际事件。可以配置运行模板并受控触发，或等待已有生活活动产生真实结果。'});return()=>panel.dispose();
}
export function renderRealWorldEvent(el,ctx){
 const s=scope(el,ctx),id=ctx.params.evId;
 return asyncPage(el,()=>mockWorld.event(s.fid,id),(e)=>{
  if(!s.alive())return;
  const runtime=e.result?.runtime_effect;
  el.innerHTML=`<div class="card"><div class="row"><h2 class="grow">${esc(e.title)}</h2>${badge(e.status,'plain')}</div><p>${esc(instant(e.start_at,s.zone))}—${esc(instant(e.end_at,s.zone))}</p><p class="small muted">${e.context_scope==='ambient'?'环境或身体事件，保留角色原位置与活动权威':'生活事件'} · ${e.world_id?`运行世界 ${esc(e.world_id)}`:'历史世界归属未记录'}</p>${runtime?JsonDetails(runtime,'虚拟世界来源与效果'):''}${JsonDetails(e.evidence_refs,'原始证据引用')}${JsonDetails(e.result,'实际提交结果')}</div><div class="card"><h3>已记录的直接影响与结果</h3><p class="small muted">这里只显示领域中存在真实引用的对象；不会推断未记录的因果链。</p>${(e.impacts||[]).map(i=>`<div class="lrow"><span class="grow"><b>${esc(i.title||i.id)}</b><p class="small muted">${esc(i.type)} · ${esc(i.status||i.recovery_status||'已记录')}</p>${i.type==='world_event_execution'?`<a href="#/c/${encodeURIComponent(s.fid)}/world/settings?template=${encodeURIComponent(i.template_id)}&run=${encodeURIComponent(i.id)}">原执行及恢复结果</a>`:''}${i.type==='memory'?`<a href="#/c/${encodeURIComponent(s.fid)}/growth/memory/${encodeURIComponent(i.id)}">记忆来源与修订</a>`:''}${i.type==='npc_encounter'?`<a href="#/c/${encodeURIComponent(s.fid)}/people/npc/${encodeURIComponent(i.npc_id)}">相关人物与接触记录</a>`:''}${JsonDetails(i,'对象标识与关联')}</span></div>`).join('')||'<p class="small muted">尚无已记录的下游对象。事件已发生不保证每次都会形成记忆、日记或新关系。</p>'}${e.more_memory_impacts?'<p class="banner info">直接记忆引用超过当前详情上限，请到记忆浏览器继续查询。</p>':''}</div><a class="btn" href="#/c/${encodeURIComponent(s.fid)}/world/events">返回事件流</a>`;
 });
}

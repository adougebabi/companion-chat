import {get,subscribe} from '../store.js';
import {PositionSummary} from './routine.js';
import {lifeClient,mockClient} from '../services.js';
import {esc} from '../ui.js';
import {Avatar,bindAvatars} from './avatar.js';
import {stateLabel} from '../../domain-presentation.mjs';
import {userInstant,displayTimezone} from '../../presentation-time.mjs';
import {ScheduleTimeRange} from './schedule-records.js';
import {WearingSummary} from './inventory-records.js';

// Read-only context stays in this pane. Folding it does not discard its stack.
const contexts=new Map();subscribe(()=>{if(!get('session'))contexts.clear();});
export function mountChatLifeSummary(host,{ctx,scope,character,onFold,onClose}){
  let alive=true,epoch=0,snapshot=null,error='',loading=false,npcEpoch=0;const npcs=new Map(),npcErrors=new Map();let previousView='';
  const contextKey=JSON.stringify([get('session')?.actorId||'',ctx.charId,scope]);
  const view=contexts.get(contextKey)||{allDay:false,stack:[],folded:false};contexts.set(contextKey,view);const positions=view.positions||(view.positions=new Map());
  const base=`#/c/${encodeURIComponent(ctx.charId)}/`;
  const link=(href,label)=>`<a class="btn ghost sm" href="${base+href}">${esc(label)} →</a>`;
  function paint(){
    if(!alive)return;
    host.classList.toggle('is-folded',view.folded);onFold?.(view.folded);
    const previous=host.querySelector('.summary-pane-body');if(previous)positions.set(previousView,previous.scrollTop);
    const top=view.stack.at(-1),detail=snapshot?.detail,context=detail?.context;
    const schedule=detail?.schedule,zone=displayTimezone(schedule?.timezone,context?.timezone);
    const appearance=detail?.current_state?.appearance;
    const items=schedule?.items||[];
    const visible=view.allDay?items:items.filter(i=>!Number.isFinite(Date.parse(i.end_at))||Date.parse(i.end_at)>Date.now()).slice(0,3);
    let content='';
    previousView=top?top.type+':'+(top.id||''):'summary';
    if(top?.type==='npc'){const npc=npcs.get(top.id);content=npc?`<article class="summary-reading"><h3>${esc(npc.name)}</h3>${npcErrors.has(top.id)?`<p class="banner warn">更新失败，保留上次人物资料：${esc(npcErrors.get(top.id))}</p>`:''}<p>${esc(npc.role||'')}</p><p>${esc(npc.bio||'')}</p>${PositionSummary({npc,placeName:id=>ctx.store.get('places',id)?.name||'地点尚未确认',zone})}${link('people/npc/'+encodeURIComponent(top.id),'完整人物与关系来源')}</article>`:`<p>${esc(npcErrors.get(top.id)||'正在读取人物资料…')}</p><button class="btn" data-summary-npc-retry>重新读取人物</button>`;}else if(top?.type==='context'){
      content=`<article class="summary-reading"><h3>${esc(character.name)}的近况</h3><p>${esc(context?.activity||'当前活动尚未确认')}</p><p>${esc(context?.location||'实际位置尚未确认')}</p>${context?.place_id?`<button class="btn" data-summary-place="${esc(context.place_id)}">查看地点</button>`:''}${link('profile','完整角色资料')}</article>`;
    }else if(top?.type==='schedule'){
      const item=items.find(i=>i.id===top.id);
      content=item?`<article class="summary-reading"><h3>${esc(item.activity)}</h3><div class="summary-reading-time">${ScheduleTimeRange(item.start_at,item.end_at,zone)}</div><p>${esc(item.scene||'')}</p><p>${esc(item.location||'地点未确认')}</p><p class="small muted">${esc(summaryPlanStatus(item))}。这是安排，实际执行结果以记录为准。</p>${item.place_id?`<button class="btn" data-summary-place="${esc(item.place_id)}">查看地点</button>`:''}${link('life/schedule','完整日程与调整')}</article>`:'<p>这项安排已不在当前日程中。</p>';
    }else if(top?.type==='place'){
      const place=ctx.store.get('places',top.id);
      content=`<article class="summary-reading"><h3>${esc(place?.name||'地点详情')}</h3>${place?.desc||place?.description?`<p>${esc(place.desc||place.description)}</p>`:''}<p class="small muted">查看地点不会改变她的实际位置。</p>${link('world/place/'+encodeURIComponent(top.id),'完整地点资料')}</article>`;
    }else if(top?.type==='wear'){
      content=`<article class="summary-reading"><h3>当前穿着</h3>${wearing(appearance,true)}${link('life/wardrobe','为她搭配')}<p class="small muted">穿着以最近确认记录为准，保存方案不会改变实际穿着。</p></article>`;
    }else if(!detail){
      content=`<div class="statebox" role="${error?'alert':'status'}"><p>${esc(error||'正在读取生活摘要…')}</p>${error?'<button class="btn" data-summary-retry>重新读取</button>':''}</div>`;
    }else{
      content=`<div class="summary-overview">
      <button class="summary-identity" data-summary-context aria-label="查看${esc(character.name)}的近况">${Avatar({src:character.avatar,name:character.name})}<span class="summary-identity-copy"><b>${esc(character.name)}</b><span>${esc(context?.activity||'当前活动尚未确认')}</span></span><span class="summary-identity-arrow" aria-hidden="true">›</span></button>
      <div class="summary-situation" aria-label="双方情境"><span>你 · 远程交流</span><span>${context?.place_id?`<button data-summary-place="${esc(context.place_id)}">她 · ${esc(context?.location||'位置待确认')}</button>`:`她 · ${esc(context?.location||'实际位置待确认')}`}</span></div>
      <section class="summary-compact-card"><div class="summary-card-heading"><h3>今天的安排</h3><button class="btn ghost sm" data-summary-day aria-expanded="${view.allDay}">${view.allDay?'收起全天':'展开全天'}</button></div><div class="summary-agenda">${schedule?visible.map(item=>`<button class="summary-agenda-item" data-summary-schedule="${esc(item.id)}" title="${esc(summaryRange(item,zone))}" aria-label="${esc(item.activity+'，'+summaryRange(item,zone)+'，'+summaryPlanStatus(item))}"><span class="summary-agenda-time">${esc(userInstant(item.start_at,zone))}</span><span class="summary-agenda-line"><b>${esc(item.activity)}</b><span class="summary-agenda-state">${esc(summaryPlanStatus(item))}</span></span></button>`).join('')||'<p class="small muted">当前范围没有后续安排。</p>':'<p class="small muted">今天尚无已接受日程。</p>'}</div>${link('life/schedule','完整日程')}</section>
      <section class="summary-compact-card summary-wear"><div class="summary-card-heading"><h3>当前穿着</h3><span class="small muted">${appearance?.wearing_state==='known'?'已确认':'尚未确认'}</span></div>${wearing(appearance)}${appearance?.wearing_state==='known'?'<button class="btn ghost sm" data-summary-wear>查看当前穿着 →</button>':link('life/wardrobe','查看衣柜')}</section>
      <details class="summary-nearby" ${view.nearbyOpen?'open':''}><summary>她身边的人${snapshot.nearby?.actors?.length?` · ${snapshot.nearby.actors.length}`:''}</summary>${snapshot.nearby?(snapshot.nearby.actors||[]).map(n=>`<button class="summary-nearby-person" data-summary-npc="${esc(n.npc_id)}"><span>${esc(n.name)}</span><span class="small muted">${n.presence==='confirmed'?'已确认在场':'预计位置'}</span></button>`).join('')||'<p class="small muted">当前没有已读取到的附近人物。</p>':'<p class="small muted">附近人物尚未读到。</p>'}</details>
      <footer class="summary-about"><a href="${base}people/owner">关于你 ›</a></footer></div>`;
    }
    host.innerHTML=`<div class="summary-pane-head"><button class="btn summary-mobile-back" data-summary-close>返回对话</button>${top?'<button class="btn icon-only ghost" data-summary-back aria-label="返回上一对象">‹</button>':''}<h2>${top?'情境查阅':'生活摘要'}</h2><span class="spacer"></span>${top?'<button class="btn icon-only ghost" data-summary-home aria-label="回到生活摘要">⌂</button>':''}<button class="btn icon-only ghost" data-summary-fold aria-label="${view.folded?'展开':'折叠'}生活摘要" aria-expanded="${!view.folded}">${view.folded?'‹':'›'}</button></div><div class="summary-pane-body" ${view.folded?'hidden':''}>${error&&snapshot?`<p class="banner warn" role="alert">更新失败，保留上次读取内容。${esc(error)} <button class="btn sm" data-summary-retry>重试</button></p>`:''}${loading&&snapshot?'<p class="small muted" role="status">正在更新…</p>':''}${content}</div>`;
    bindAvatars(host);
    host.querySelector('.summary-nearby')?.addEventListener('toggle',event=>{view.nearbyOpen=event.currentTarget.open;});
    host.querySelector('[data-summary-context]')?.addEventListener('click',()=>{view.stack.push({type:'context'});paint();});
    const body=host.querySelector('.summary-pane-body');if(body)body.scrollTop=positions.get(previousView)||0;
    host.querySelectorAll('[data-summary-npc]').forEach(button=>button.onclick=()=>{view.stack.push({type:'npc',id:button.dataset.summaryNpc});paint();void loadNpc(button.dataset.summaryNpc);});host.querySelector('[data-summary-npc-retry]')?.addEventListener('click',()=>void loadNpc(top.id));
    host.querySelector('[data-summary-close]').onclick=()=>{host.classList.remove('is-open');onClose?.();};
    host.querySelector('[data-summary-fold]').onclick=()=>{view.folded=!view.folded;paint();host.querySelector('[data-summary-fold]')?.focus({preventScroll:true});};
    host.querySelector('[data-summary-back]')?.addEventListener('click',()=>{const previous=view.stack.pop();paint();const selector=previous.type==='context'?'[data-summary-context]':previous.type==='npc'?'[data-summary-npc="'+CSS.escape(previous.id)+'"]':previous.type==='schedule'?'[data-summary-schedule="'+CSS.escape(previous.id)+'"]':previous.type==='wear'?'[data-summary-wear]':'[data-summary-place="'+CSS.escape(previous.id)+'"]';host.querySelector(selector)?.focus({preventScroll:true});});
    host.querySelector('[data-summary-home]')?.addEventListener('click',()=>{view.stack=[];paint();});
    host.querySelector('[data-summary-day]')?.addEventListener('click',()=>{view.allDay=!view.allDay;paint();host.querySelector('[data-summary-day]')?.focus({preventScroll:true});});
    host.querySelector('[data-summary-wear]')?.addEventListener('click',()=>{view.stack.push({type:'wear'});paint();});
    host.querySelectorAll('[data-summary-schedule]').forEach(button=>button.onclick=()=>{view.stack.push({type:'schedule',id:button.dataset.summarySchedule});paint();});
    host.querySelectorAll('[data-summary-place]').forEach(button=>button.onclick=()=>{view.stack.push({type:'place',id:button.dataset.summaryPlace});paint();});
    host.querySelector('[data-summary-retry]')?.addEventListener('click',()=>void load());
  }
  function wearing(appearance){
    return WearingSummary(appearance?.worn_items,{known:appearance?.wearing_state==='known'});
  }
  async function loadNpc(id){const token=++npcEpoch;try{const npc=await mockClient.npc(ctx.charId,id);if(!alive||token!==npcEpoch)return;npcs.set(id,npc);npcErrors.delete(id);}catch(cause){if(alive&&token===npcEpoch)npcErrors.set(id,cause.message);}if(alive&&token===npcEpoch)paint();}
  async function load(){
    const request=++epoch;loading=true;paint();
    const results=await Promise.allSettled([lifeClient.detail(ctx.charId),lifeClient.nearby(ctx.charId)]);
    if(!alive||request!==epoch)return;
    error='';if(results[0].status==='fulfilled')snapshot={...snapshot,detail:results[0].value};else error=results[0].reason.message;
    if(results[1].status==='fulfilled')snapshot={...snapshot,nearby:results[1].value};else error+=(error?'；':'')+'附近人物读取失败：'+results[1].reason.message;
    loading=false;paint();
  }

  paint();void load();if(view.stack.at(-1)?.type==='npc')void loadNpc(view.stack.at(-1).id);
  return {open(){view.folded=false;host.classList.add('is-open');paint();},close(){host.classList.remove('is-open');},dispose(){const body=host.querySelector('.summary-pane-body');if(body)positions.set(previousView,body.scrollTop);alive=false;epoch++;npcEpoch++;}};
}

// Planned time never establishes an execution result.
export function summaryPlanStatus(item){
  if(item.action_status)return '实际 · '+stateLabel(item.action_status,'执行状态待确认');
  const label=stateLabel(item.status,'状态待确认');
  return label==='计划'?'计划':'计划 · '+label;
}
function summaryRange(item,zone){return userInstant(item.start_at,zone)+' 至 '+userInstant(item.end_at,zone);}

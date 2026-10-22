import { lifeClient } from '../services.js';
import { asyncPage,instant } from '../components/async-page.js';
import {enumLabel} from '../../../lib/fluctlight-display.ts';
import { esc,badge } from '../ui.js';

export function renderRealGoals(el,ctx) {
  let cursor='';
  return asyncPage(el,async()=>{const [page,detail]=await Promise.all([lifeClient.goals(ctx.charId,cursor),lifeClient.detail(ctx.charId)]);return {page,detail};},({page,detail},reload,alive)=>{
    const zone=detail.context?.timezone;
    el.innerHTML=`<div class="stack"><div class="row"><div class="grow"><h2 class="page-title">目标</h2><p class="small muted">她希望达成的事，以及当前的推进情况。</p></div><button class="btn" data-reload>刷新</button><a class="btn primary" href="#/c/${ctx.charId}/life/goal/new">新建目标</a><a class="btn" href="#/c/${ctx.charId}/life/planning">规划与候选</a></div><div class="goal-card-grid">${page.items.map(goal=>`<article class="card goal-reading-card"><div class="row"><h3 class="grow"><a href="#/c/${encodeURIComponent(ctx.charId)}/life/goal/${encodeURIComponent(goal.id)}">${esc(goal.desired_outcome)}</a></h3>${badge(enumLabel(goal.status))}</div><p>${esc(goal.motivation||'尚未记录动机')}</p>${goal.deadline?`<p class="small muted">希望完成于 ${esc(instant(goal.deadline,zone))}</p>`:''}<a class="small" href="#/c/${encodeURIComponent(ctx.charId)}/life/goal/${encodeURIComponent(goal.id)}">查看目标与进展 →</a></article>`).join('')||'<div class="statebox"><h3>当前范围尚无目标</h3><p>可以新建目标，或查看独立规划与候选。</p></div>'}</div><div class="row">${page.next_cursor?'<button class="btn" data-more>下一页目标</button>':''}${cursor?'<button class="btn" data-first>返回第一页</button>':''}</div><p class="small muted">意向和我们的约定已归入日程分区。<a href="#/c/${ctx.charId}/life/schedule">查看日程 →</a></p></div>`;
    el.querySelector('[data-reload]').onclick=reload;
    el.querySelector('[data-more]')?.addEventListener('click',()=>{cursor=page.next_cursor;void reload();});
    el.querySelector('[data-first]')?.addEventListener('click',()=>{cursor='';void reload();});
  });
}

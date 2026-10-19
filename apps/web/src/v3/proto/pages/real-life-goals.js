import { lifeClient } from '../services.js';
import { asyncPage,requestReceipt,instant } from '../components/async-page.js';
import {mountPromiseCatalogue,promiseForm} from '../components/promise-catalogue.js';
import { RecordDetails } from '../components/records.js';
import { esc,badge } from '../ui.js';

export function renderRealGoals(el,ctx) {
  let cursor='';
  return asyncPage(el,async()=>{const [page,detail]=await Promise.all([lifeClient.goals(ctx.charId,cursor),lifeClient.detail(ctx.charId)]);return {page,detail};},({page,detail},reload,alive)=>{
    const zone=detail.context?.timezone;
    el.innerHTML=`<div class="row" style="margin-bottom:12px"><span class="grow small muted">她想要的、她在做的、和你说好的</span><button class="btn" data-reload>重新读取</button><button class="btn primary" data-promise>和她约定</button></div><div class="grid g3"><div class="card"><h3>目标</h3>${page.items.map(goal=>`<div class="lrow"><div class="grow"><a href="#/c/${ctx.charId}/life/goal/${esc(goal.id)}"><b>${esc(goal.desired_outcome)}</b></a><div class="small muted">${esc(goal.motivation)}${goal.deadline?' · '+esc(instant(goal.deadline,zone)):''}</div></div>${badge(goal.status)}</div>`).join('')||'<p class="small muted">本页尚无目标，可以新建目标或请求独立规划。</p>'}${page.next_cursor?'<button class="btn sm" data-more>下一页目标</button>':''}${cursor?'<button class="btn sm" data-first>返回第一页</button>':''}</div><div class="card"><h3>意向</h3>${RecordDetails(detail.intentions || [])}<p class="small muted">意向不代表已经执行或完成。</p></div><div class="card" data-promise-catalogue></div></div><div class="row" style="margin-top:12px"><a class="btn" href="#/c/${ctx.charId}/life/goal/new">新建目标</a><a class="btn" href="#/c/${ctx.charId}/life/planning">独立规划与候选</a></div>`;
    el.querySelector('[data-reload]').onclick=reload;
    el.querySelector('[data-more]')?.addEventListener('click',()=>{cursor=page.next_cursor;void reload();});
    el.querySelector('[data-first]')?.addEventListener('click',()=>{cursor='';void reload();});
    const promises=mountPromiseCatalogue(el.querySelector('[data-promise-catalogue]'),{fid:ctx.charId,alive,onSaved:reload});
    el.querySelector('[data-promise]').disabled=ctx.char?.status==='已退休';
    el.querySelector('[data-promise]').onclick=()=>promiseForm(ctx.charId,{onSaved:envelope=>{if(alive())requestReceipt(ctx.charId,envelope,reload);}});
    return()=>promises.dispose();
  });
}

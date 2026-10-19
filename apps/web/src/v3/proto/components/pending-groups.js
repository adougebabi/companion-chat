import {operationsClient} from '../services.js';
import {mountCursorRecords} from './cursor-records.js';
import {esc,badge} from '../ui.js';
import {instant} from './async-page.js';

const labels={capability:'能力请求',persona_proposal:'人格与身份提案',studio_apply:'待应用定义',failed_task:'失败任务',owner_request:'正在处理的请求'};
export function mountPendingGroups(box,{fid='',alive=()=>true,compact=false}={}){
  let disposed=false,epoch=0;const panels=[];
  const current=()=>!disposed&&alive()&&box.isConnected;
  async function load(){
    const version=++epoch;panels.splice(0).forEach(p=>p.dispose());box.innerHTML='<p role="status">正在读取待处理来源…</p>';
    try{
      const page=await operationsClient.pending(fid);if(!current()||version!==epoch)return;
      box.innerHTML=`<p class="small muted">${page.partial?'部分来源读取失败，以下数量仅含可读取来源':'当前范围'} · ${esc(page.count)} 项。通知已读不会处理这些事项。</p>${page.groups.map(g=>`<section class="${compact?'':'card'}"><h3>${esc(labels[g.kind]||g.kind)} ${g.status==='ok'?badge(String(g.count)):badge('读取失败','bad')}</h3><div data-pending-kind="${esc(g.kind)}"></div></section>`).join('')}`;
      for(const group of page.groups){
        let initial=group;
        const panel=mountCursorRecords(box.querySelector(`[data-pending-kind="${group.kind}"]`),{alive:current,label:labels[group.kind],empty:'当前没有这类待处理事项。',read:async cursor=>{const g=initial||((await operationsClient.pending(fid,group.kind,cursor)).groups[0]);initial=null;if(g.status!=='ok')throw new Error(g.message||'该来源读取失败，不能判断是否处理完');return {items:g.items,next_cursor:g.next_cursor};},render:item=>`<div class="lrow"><a class="grow" href="${esc(item.href)}">${esc(item.title)}<div class="small muted">${esc(instant(item.created_at))} · ${esc(item.fluctlight_id||'系统任务')}</div></a>${badge(item.status)}<a class="btn sm" href="${esc(item.href)}">去处理</a></div>`});panels.push(panel);
      }
    }catch(error){if(!current()||version!==epoch)return;box.innerHTML=`<p class="banner err" role="alert">${esc(error.message)}</p><button class="btn" data-pending-retry>重新读取</button>`;box.querySelector('[data-pending-retry]').onclick=()=>void load();}
  }
  void load();return {reload:load,dispose(){disposed=true;epoch++;panels.forEach(p=>p.dispose());}};
}

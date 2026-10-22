import {operationsClient} from '../services.js';
import {mountCursorRecords} from './cursor-records.js';
import {esc,badge} from '../ui.js';
import {get} from '../store.js';
import {domainName,stateLabel,errorMessage} from '../../domain-presentation.mjs';
import {InstantLabel} from './instant-label.js';
import {JsonDetails} from './records.js';

const pendingTitle=value=>({'daily_review.current_day':'日常回顾','schedule.current_day':'当日日程规划','reflection.current_window':'当前阶段反思','visual_identity.prepare':'视觉身份准备'})[value]||domainName(value,'待处理任务');
const labels={capability:'能力请求',persona_proposal:'人格与身份提案',studio_apply:'待应用定义',failed_task:'失败任务',owner_request:'正在处理的请求'};
export function mountPendingGroups(box,{fid='',alive=()=>true,compact=false}={}){
  let disposed=false,epoch=0;const panels=[];
  const current=()=>!disposed&&alive()&&box.isConnected;
  async function load(){
    const version=++epoch;panels.splice(0).forEach(p=>p.dispose());box.innerHTML='<p role="status">正在读取待处理来源…</p>';
    try{
      const page=await operationsClient.pending(fid);if(!current()||version!==epoch)return;
      const actionable=page.groups.filter(g=>g.status!=='ok'||g.count>0),emptyGroups=page.groups.filter(g=>g.status==='ok'&&g.count===0);
      const section=g=>`<section class="${compact?'':'card'}"><h3>${esc(labels[g.kind]||g.kind)} ${g.status==='ok'?badge(String(g.count),'plain'):badge('读取失败','失败')}</h3><div data-pending-kind="${esc(g.kind)}"></div></section>`;
      box.innerHTML=`<div class="stack pending-groups"><p class="small muted">${page.partial?'部分来源读取失败，以下数量仅含可读取来源':'当前范围'} · ${esc(page.count)} 项。通知已读不会处理这些事项。</p>${actionable.map(section).join('')}${!actionable.length?'<p class="small muted">当前没有待处理事项。</p>':''}${emptyGroups.length?`<details class="${compact?'':'card'} pending-empty"><summary>没有待处理事项的分类 · ${emptyGroups.length}</summary><div class="stack">${emptyGroups.map(section).join('')}</div></details>`:''}</div>`;
      for(const group of page.groups){
        let initial=group;
        const panel=mountCursorRecords(box.querySelector(`[data-pending-kind="${group.kind}"]`),{alive:current,label:labels[group.kind],empty:'当前没有这类待处理事项。',read:async cursor=>{const g=initial||((await operationsClient.pending(fid,group.kind,cursor)).groups[0]);initial=null;if(g.status!=='ok')throw new Error(g.message||'该来源读取失败，不能判断是否处理完');return {items:g.items,next_cursor:g.next_cursor};},render:item=>`<div class="lrow pending-item"><div class="grow"><a href="${esc(item.href)}"><b>${esc(pendingTitle(item.title))}</b></a><div class="small muted">${InstantLabel(item.created_at)} · ${esc(item.fluctlight_id?get('characters',item.fluctlight_id)?.name||'角色资料待读取':'系统任务')}</div><div class="pending-state">${badge(stateLabel(item.status),'plain')}</div>${group.kind==='failed_task'?'<p class="small banner warn">执行未成功。具体原因和可用恢复方式请查看详情；本列表来源未提供失败原因。</p>':''}${JsonDetails(item,'事项来源与标识')}</div><a class="btn sm" href="${esc(item.href)}">${['failed_task','owner_request'].includes(group.kind)?'查看详情':'查看事项'}</a></div>`});panels.push(panel);
      }
    }catch(error){if(!current()||version!==epoch)return;box.innerHTML=`<p class="banner err" role="alert">${esc(errorMessage(error))}</p><button class="btn" data-pending-retry>重新读取</button>`;box.querySelector('[data-pending-retry]').onclick=()=>void load();}
  }
  void load();return {reload:load,dispose(){disposed=true;epoch++;panels.forEach(p=>p.dispose());}};
}

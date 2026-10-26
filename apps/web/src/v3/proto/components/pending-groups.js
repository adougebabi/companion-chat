import {pendingSummary,pendingCategory} from '../../pending-presentation.mjs';
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
      const summary=pendingSummary(page.groups);
      const actionable=page.groups.filter(g=>g.status!=='ok'||g.count>0),emptyGroups=page.groups.filter(g=>g.status==='ok'&&g.count===0);
      const section=g=>`<section class="${compact?'':'card'}"><h3>${esc(labels[g.kind]||g.kind)} ${g.status==='ok'?badge(String(g.count),'plain'):badge('读取失败','失败')}</h3><div data-pending-kind="${esc(g.kind)}"></div></section>`;
      const attention=actionable.filter(g=>pendingCategory(g.kind)==='attention'),progress=actionable.filter(g=>pendingCategory(g.kind)==='progress'),service=actionable.filter(g=>pendingCategory(g.kind)==='service');
      box.innerHTML=`<div class="stack pending-groups"><p>${summary.attentionKnown?(summary.attention?`${summary.attention} 件事需要你决定`:'暂时没有需要你处理的事'):'需要你决定的事项暂未完整读到'}</p>${attention.map(section).join('')}${progress.length?`<details data-domain-ui="disclosure"><summary>她正在处理的事 · ${summary.progressKnown?summary.progress:'暂未读全'}</summary>${progress.map(section).join('')}</details>`:''}${service.length?`<details data-domain-ui="disclosure" class="pending-service"><summary>服务状态 · ${summary.serviceKnown?summary.service+' 项运行异常':'暂不可读'}</summary><p class="small muted">这些由后台服务处理，不是你的待办。</p><a data-domain-ui="button" class="btn" href="#/diag">查看服务状态与恢复</a>${service.map(section).join('')}</details>`:''}${emptyGroups.length?`<details data-domain-ui="disclosure" class="pending-empty"><summary>其余分类</summary>${emptyGroups.map(section).join('')}</details>`:''}</div>`;

      for(const group of page.groups){
        let initial=group;
        const panel=mountCursorRecords(box.querySelector(`[data-pending-kind="${group.kind}"]`),{alive:current,label:labels[group.kind],empty:'当前没有这类待处理事项。',read:async cursor=>{const g=initial||((await operationsClient.pending(fid,group.kind,cursor)).groups[0]);initial=null;if(g.status!=='ok')throw new Error(g.message||'该来源读取失败，不能判断是否处理完');return {items:g.items,next_cursor:g.next_cursor};},render:item=>`<div class="lrow pending-item"><div class="grow"><a href="${esc(item.href)}"><b>${esc(pendingTitle(item.title))}</b></a><div class="small muted">${InstantLabel(item.created_at)} · ${esc(item.fluctlight_id?get('characters',item.fluctlight_id)?.name||'角色资料待读取':'系统任务')}</div><div class="pending-state">${badge(stateLabel(item.status),'plain')}</div>${group.kind==='failed_task'?'<p class="small banner warn">后台服务未完成这项工作，可在服务状态中查看恢复方式。</p>':''}${JsonDetails(item,'事项来源与标识')}</div><a data-domain-ui="button" class="btn sm" href="${esc(item.href)}">${['failed_task','owner_request'].includes(group.kind)?'查看详情':'查看事项'}</a></div>`});panels.push(panel);
      }
    }catch(error){if(!current()||version!==epoch)return;box.innerHTML=`<p class="banner err" role="alert">${esc(errorMessage(error))}</p><button data-domain-ui="button" class="btn" data-pending-retry>重新读取</button>`;box.querySelector('[data-pending-retry]').onclick=()=>void load();}
  }
  void load();return {reload:load,dispose(){disposed=true;epoch++;panels.forEach(p=>p.dispose());}};
}

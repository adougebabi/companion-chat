import {renderRealDiagnosticHome} from './real-diagnostics.js';
import {operationsClient} from '../services.js';
import * as store from '../store.js';
import {asyncPage,instant} from '../components/async-page.js';
import {CursorPager,bindCursorPager} from '../components/cursor-pager.js';
import {mountPendingGroups} from '../components/pending-groups.js';
import {JsonDetails} from '../components/records.js';
import {Tabs} from '../components/forms.js';
import {domainName,stateLabel} from '../../domain-presentation.mjs';
import {requestDecisionText,RequestFailureLink} from '../components/request-failure.js';
import {esc,badge,confirm} from '../ui.js';

const taskLink=id=>`#/diag/task/${encodeURIComponent(id)}`;
export function requestTaskLinks(request){return `<p><a href="#/diag/request/${encodeURIComponent(request.id)}">请求详情与执行结果</a></p>${request.tasks_state==='failed'?'<p class="banner warn">请求状态已读取，关联任务读取失败，请重新查询。</p>':request.tasks?.map(t=>`<p><a href="${taskLink(t.task_id)}">${esc(domainName(t.kind,'关联任务'))} · ${esc(stateLabel(t.status))}</a></p>`).join('')||''}`;}
async function refreshChrome(box){try{await operationsClient.refresh();}catch(error){if(box.isConnected)box.textContent=`操作已提交；汇总刷新失败：${error.message}`;}}

export function renderRealInbox(el,ctx){
  const unread=ctx.query.tab==='未读',cursors=[''];let at=0;
  return asyncPage(el,()=>operationsClient.notifications(cursors[at],unread),(page,reload,alive)=>{
    el.innerHTML=`<div class="grid g2"><div class="card">${Tabs({value:unread?'未读':'全部',items:['全部','未读'].map(value=>({value,label:value,href:`#/inbox?tab=${value}`}))})}<div class="list">${page.items.map(n=>`<div class="lrow" data-test="notice-${esc(n.id)}"><a class="grow" href="${esc(n.link)}"><b>${esc(n.title)}</b><p>${esc(n.body)}</p><span class="small muted">${esc(instant(n.source_occurred_at))}${n.quiet?' · 静音通知':''}</span></a>${n.read_at?badge('已读','plain'):`${badge('未读')}<button class="btn ghost sm" data-one="${esc(n.id)}">标为已读</button>`}</div>`).join('')||'<p class="small muted">当前筛选没有通知。继续交流和生活，已提交的事件会在这里产生通知。</p>'}</div>${CursorPager({more:!!page.next_cursor,previous:at>0,label:'通知'})}<div class="row"><button class="btn sm" data-read-all>将截至本次读取的通知全部标为已读</button><button class="btn sm" data-reload>重新读取</button></div><p role="status" data-feedback></p></div><div class="card"><h3>待处理事项</h3><div data-pending></div><a class="small" href="#/diag/pending">全部待处理 →</a></div></div>`;
    const pending=mountPendingGroups(el.querySelector('[data-pending]'),{alive,compact:true});
    const execute=async(button,run)=>{button.disabled=true;try{await run();await refreshChrome(el.querySelector('[data-feedback]'));if(alive())await reload();}catch(error){if(alive()){el.querySelector('[data-feedback]').textContent=error.message;button.disabled=false;}}};
    el.querySelectorAll('[data-one]').forEach(b=>b.onclick=()=>execute(b,()=>operationsClient.readNotification(b.dataset.one)));
    el.querySelector('[data-read-all]').onclick=e=>execute(e.target,()=>operationsClient.readAllNotifications(page.as_of));
    el.querySelector('[data-reload]').onclick=reload;
    bindCursorPager(el,{next:()=>{cursors[++at]=page.next_cursor;void reload();},previous:()=>{at--;void reload();}});
    return()=>pending.dispose();
  });
}
export function renderRealPending(el,ctx){
  const fid=ctx.query.charId||'';
  el.innerHTML=`<p class="small muted">待处理事项来自各领域的实际状态。处理后会从对应来源消失；单一来源失败分别显示。</p><div class="row"><a class="btn" href="#/diag/pending">所有角色</a>${store.list('characters').map(c=>`<a class="btn" href="#/diag/pending?charId=${encodeURIComponent(c.id)}">${esc(c.name)}</a>`).join('')}<button class="btn" data-reload>重新读取</button></div><div class="stack" data-pending></div>`;
  const pending=mountPendingGroups(el.querySelector('[data-pending]'),{fid});el.querySelector('[data-reload]').onclick=()=>pending.reload();return()=>pending.dispose();
}
export function renderRealRequest(el,ctx){
  return asyncPage(el,()=>operationsClient.request(ctx.params.requestId),(request,reload)=>{
    el.innerHTML=`<div class="card"><div class="row"><h2 class="grow">${esc(domainName(request.kind,'行动请求'))} ${badge(stateLabel(request.status))}</h2><button class="btn" data-reload>重新查询</button></div><p>${esc(requestDecisionText(request)||'尚无处理结论')}</p><p class="small muted">${esc(request.id)} · ${esc(instant(request.created_at))} · v${esc(request.revision)}</p>${requestTaskLinks(request)}${RequestFailureLink(request)}${JsonDetails(request.payload,'请求原始内容')}${JsonDetails(request.result||{status:request.status},'实际结果')}${JsonDetails(request.result_refs,'结果与事实来源')}<a class="btn" href="#/diag/pending">返回待处理</a>${request.fluctlight_id?`<a class="btn" href="#/c/${encodeURIComponent(request.fluctlight_id)}/life/now">角色此刻</a>`:''}</div>`;el.querySelector('[data-reload]').onclick=reload;
  });
}
export function renderRealTask(el,ctx){
  return asyncPage(el,()=>operationsClient.task(ctx.params.taskId),(task,reload,alive)=>{
    el.innerHTML=`<div class="card"><div class="row"><h2 class="grow">${esc(domainName(task.kind||task.type||task.title,'后台任务'))} ${badge(stateLabel(task.status))}</h2><button class="btn" data-reload>重新读取</button></div><p class="small muted">任务 ${esc(task.task_id)} · 工作流 ${esc(task.workflow_id)} · 已尝试 ${esc(task.runs)} 次</p><p class="small muted">创建 ${esc(instant(task.created_at))} · 开始 ${esc(instant(task.started_at))} · 结束 ${esc(instant(task.completed_at))}</p>${task.steps.map(s=>`<div class="lrow"><div class="grow"><b>${esc(domainName(s.event_type||s.name,'执行阶段记录'))}</b><p class="small muted">${esc(instant(s.at))}${s.reason_code?` · ${esc(stateLabel(s.reason_code,'有待核对的执行说明'))}`:''}</p></div>${badge(stateLabel(s.status,'已记录'))}</div>`).join('')||'<p class="small muted">没有保留的生命周期事件，请同时查询真实工作流历史。没有事件不代表没有执行。</p>'}${task.has_more_steps?'<p class="small muted">此处仅展示最近 100 条事件。完整轨迹请按该任务 ID 查询生命周期诊断。</p>':''}${JsonDetails(task,'任务完整状态与来源')}<div class="row"><a class="btn primary" href="#/diag/workflow/${encodeURIComponent(task.workflow_id)}">执行器状态、真实历史与受控恢复</a>${task.fluctlight_id?`<a class="btn" href="#/c/${encodeURIComponent(task.fluctlight_id)}/life/now">回到此刻</a>`:''}</div><div data-history-result></div><p role="status" data-feedback></p></div>`;
    el.querySelector('[data-reload]').onclick=reload;

  });
}
export function renderRealDiag(el,ctx){return renderRealDiagnosticHome(el,ctx);}

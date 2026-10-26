import {stateLabel,errorMessage} from '../../domain-presentation.mjs';
import {esc} from '../ui.js';
import {instant} from './async-page.js';
import {JsonDetails} from './records.js';

export function reflectionTaskPresentation(task){
 if(!task)return {attention:false,title:'还没有反思任务记录',description:'新的交流与经历可以成为之后反思的素材。'};
 const failed=!!task.error_code||['failed','dead_letter'].includes(task.status);
 const replaced=task.status==='superseded';
 const retry=!!task.next_attempt_at&&!['completed','cancelled','dead_letter','superseded'].includes(task.status);
 return {attention:failed||replaced,title:replaced?'原反思任务已被新的活动取代':failed?'最近一次反思没有完成':retry?'反思正在等待重试':stateLabel(task.status),description:[failed?'这次执行未形成可确认的新反思结果。':'',replaced?'旧任务不会继续应用；后续反思以新的交流和经历为准。':'',retry?'已安排重试：'+instant(task.next_attempt_at):'',task.status==='dead_letter'?'自动重试已用尽，请查看原运行记录并检查服务配置。':'',task.attempt_count==null?'尝试次数未记录。':Number(task.attempt_count)===0?'尚未开始执行。':`已尝试 ${task.attempt_count} 次。`].filter(Boolean).join(' ')};
}
export function ReflectionProgress(page){
 const task=page.latest_task,view=reflectionTaskPresentation(task);
 return `<section data-domain-ui="alert" class="${view.attention?'banner warn':'reflection-progress-summary'}" role="status"><h3>${esc(view.title)}</h3><p>${esc(view.description)}</p>${task?.error_code?`<p>${esc(errorMessage({code:task.error_code},'反思处理失败，请查看原运行记录。'))}</p>`:''}${task?.intent_id?`<a data-domain-ui="button" class="btn sm" href="#/diag/task/${encodeURIComponent(task.intent_id)}">查看原运行记录</a>`:''}</section>${task?`<div data-domain-ui="progress" data-value="${task.status==='completed'?100:['running','processing'].includes(task.status)?50:0}" aria-label="反思处理阶段"></div>`:''}${page.window?`<p>反思输入范围：${esc(stateLabel(page.window.status))}</p>${JsonDetails(page.window,'输入范围与版本')}`:'<p class="small muted">还没有已记录的反思输入窗口；这不代表任务没有执行。</p>'}${task?JsonDetails(task,'完整任务状态'):''}`;
}

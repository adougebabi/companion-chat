import {stateLabel,errorMessage} from '../../domain-presentation.mjs';
import {userInstant} from '../../presentation-time.mjs';
import {esc} from '../ui.js';
import {RawRecordDownload} from './records.js';

function reflectionInstant(value){
 return typeof value==='string'&&Number.isFinite(Date.parse(value))?userInstant(value):'';
}

export function reflectionTaskPresentation(task){
 if(!task||!Object.keys(task).length)return {attention:false,title:'还没有反思任务记录',description:'新的交流与经历可以成为之后反思的素材。',facts:[]};
 const failed=!!task.error_code||['failed','dead_letter'].includes(task.status);
 const replaced=task.status==='superseded';
 const nextAttempt=!['completed','cancelled','dead_letter','superseded'].includes(task.status)
  ?reflectionInstant(task.next_attempt_at):'';
 const attempts=task.attempt_count;
 return {
  attention:failed||replaced,
  title:replaced?'原反思任务已被新的活动取代':nextAttempt?'反思正在等待重试':failed?'最近一次反思没有完成':task.status?stateLabel(task.status):'',
  description:[
   failed?'这次执行未形成可确认的新反思结果。':'',
   replaced?'旧任务不会继续应用；后续反思以新的交流和经历为准。':'',
   nextAttempt?'已安排重试：'+nextAttempt:'',
   task.status==='dead_letter'?'自动重试已用尽，请查看原运行记录并检查服务配置。':'',
   attempts===0?'尚未开始执行。':Number.isInteger(attempts)&&attempts>0?`第 ${attempts} 次尝试。`:'',
  ].filter(Boolean).join(' '),
  facts:[
   ['开始时间',reflectionInstant(task.started_at)],
   ['完成时间',reflectionInstant(task.completed_at)],
   ['失败原因',failed?errorMessage({code:task.error_code},'反思处理失败，请查看原运行记录。'):''],
  ].filter(([,value])=>value),
 };
}

export function ReflectionProgress(page){
 const {latest_task:task,window}=page;
 const view=reflectionTaskPresentation(task);
 const facts=view.facts.map(([label,value])=>`<div><dt>${esc(label)}</dt><dd>${esc(value)}</dd></div>`).join('');
 const taskLink=task?.intent_id?`<a data-domain-ui="button" data-domain-variant="outline" data-domain-size="sm" href="#/diag/task/${encodeURIComponent(task.intent_id)}">查看原运行记录</a>`:'';
 const hasRecord=[task,window].some(record=>record&&Object.keys(record).length);
 return `<section data-domain-ui="alert" class="${view.attention?'banner warn':'reflection-progress-summary'}" role="status">
  ${view.title?`<h3>${esc(view.title)}</h3>`:''}
  ${view.description?`<p>${esc(view.description)}</p>`:''}
  ${facts?`<dl class="record-facts">${facts}</dl>`:''}
  ${taskLink}
 </section>
 ${task?.status?`<div data-domain-ui="progress" data-value="${task.status==='completed'?100:['running','processing'].includes(task.status)?50:0}" aria-label="反思处理阶段"></div>`:''}
 ${window?.status?`<p>反思输入范围：${esc(stateLabel(window.status))}</p>`:''}
 ${hasRecord?RawRecordDownload({latest_task:task,window},'原始反思任务记录（诊断）'):''}`;
}

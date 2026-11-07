import {ChevronDown} from 'lucide-react';
import {Button} from '@/components/ui/button';
import {Collapsible,CollapsibleTrigger,CollapsibleContent} from '@/components/ui/collapsible';
import {Disclosure} from '../controls';
import {Feedback,ReadState} from './Presentation';
import DiagnosticStatus from './DiagnosticStatus';
import RecordView from './RecordView';
import JsonDetails from './JsonDetails';
import {diagnosticTimeLabel,taskEventText,taskEventReason,taskRoundRecords} from '@/approved-ui/task-presentation.mjs';
import {callDurationLabel} from '@/approved-ui/task-model-call-presentation.mjs';

export default function TaskExecutionResults({task,steps,calls,available}:{task:any;steps:any[];calls:any;available:boolean}){
 const rounds=taskRoundRecords(calls.items),success=rounds.filter((row:any)=>row.status==='success').length,failure=rounds.filter((row:any)=>row.status==='failure').length;
 const lifecycle=<ol className="replica-task-history">{steps.map((entry:any,index:number)=><li key={entry.id??index}>{entry.at&&<time dateTime={entry.at}>{diagnosticTimeLabel(entry.at)}</time>}<p className="replica-task-history-text">{taskEventText(entry)}</p>{taskEventReason(entry)&&<p className="replica-task-history-reason">{taskEventReason(entry)}</p>}</li>)}</ol>;
 return <div className="replica-task-results">
  {steps.length>0?<section className="replica-task-lifecycle" aria-label="执行时间线">{lifecycle}</section>:<p className="replica-inbox-caption">尚无已记录的生命周期事件。</p>}
  <Disclosure title={`模型调用记录${calls.ready?` · ${calls.cursor?'已读取 ':''}${rounds.length} 次`:''}`}>
   <p className="replica-inbox-caption">以下只列实际模型调用；任务调度尝试与生命周期事件不计为模型轮次。</p>
   <dl className="replica-task-result-summary" aria-label="模型调用结果汇总">
    {[['调用',rounds.length],['成功',success],['失败',failure]].map(([label,value])=><div key={label}><dt>{calls.cursor?'已读取':''}{label}</dt><dd>{calls.ready?value:'—'}</dd></div>)}
   </dl>
   {calls.busy&&<ReadState>正在读取模型调用…</ReadState>}
   {calls.error&&<Feedback kind={calls.ready?'warning':'error'} actions={<Button variant="outline" onClick={calls.retry}>重新读取模型调用</Button>}>{calls.error}</Feedback>}
   {!available&&<p className="replica-inbox-caption">此任务尚未提供可关联的模型调用。</p>}
   {calls.ready&&!rounds.length&&<p className="replica-inbox-caption">{['failed','dead_letter'].includes(task.status)?'没有模型调用记录；任务可能在进入模型执行前已停止。':'没有已记录的模型调用。'}</p>}
   {rounds.length>0&&<ol className="replica-task-result-list">{rounds.map(({record,call,label,status}:any)=><li key={record.ref}>
    <Collapsible className="replica-task-result-row">
     <CollapsibleTrigger asChild><Button type="button" variant="ghost" className="replica-task-round-trigger"><span>{label}</span><DiagnosticStatus status={call.status} label={status==='success'?'成功':status==='failure'?'失败':undefined}/><span className="replica-task-round-duration">{callDurationLabel(call,{includeSource:false})}</span><ChevronDown size={14}/></Button></CollapsibleTrigger>
     <CollapsibleContent className="replica-task-round-content">
      {call.started_at&&<time dateTime={call.started_at}>{diagnosticTimeLabel(call.started_at)}</time>}
      <section><h3>输入</h3><RecordView value={call.prompt} emptyText="未记录输入"/></section>
      <section><h3>输出</h3><RecordView value={call.response} emptyText="未记录输出"/></section>
      <section><h3>错误</h3>{call.error||call.error_code?<><RecordView value={call.error} emptyText="未提供错误说明"/>{call.error_code&&<JsonDetails value={{error_code:call.error_code}} title="错误代码"/>}</>:<p className="replica-inbox-caption">未记录错误</p>}</section>
      <JsonDetails value={record} title="本轮原始记录"/>
     </CollapsibleContent>
    </Collapsible>
   </li>)}</ol>}
   {calls.cursor&&<Button variant="outline" disabled={calls.busy} onClick={calls.more}>读取更多模型调用</Button>}
  </Disclosure>
  {steps.length>0&&<JsonDetails value={steps} title="生命周期原始记录"/>}
 </div>;
}

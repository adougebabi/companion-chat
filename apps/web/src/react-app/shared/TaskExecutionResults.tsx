import {ChevronDown} from 'lucide-react';
import {Button} from '@/components/ui/button';
import {Collapsible,CollapsibleTrigger,CollapsibleContent} from '@/components/ui/collapsible';
import {Disclosure} from '../controls';
import {Feedback,ReadState} from './Presentation';
import DiagnosticStatus from './DiagnosticStatus';
import RecordView from './RecordView';
import JsonDetails from './JsonDetails';
import {diagnosticInstant,taskEventText,taskEventReason,taskRoundRecords} from '@/approved-ui/task-presentation.mjs';
import {callDurationLabel} from '@/approved-ui/task-model-call-presentation.mjs';

export default function TaskExecutionResults({task,steps,calls,available}:{task:any;steps:any[];calls:any;available:boolean}){
 const rounds=taskRoundRecords(calls.items),success=rounds.filter((row:any)=>row.status==='success').length,failure=rounds.filter((row:any)=>row.status==='failure').length;
 return <div className="replica-task-results">
  <dl className="replica-task-result-summary" aria-label="执行结果汇总">
   {[['轮次',rounds.length],['成功',success],['失败',failure]].map(([label,value])=><div key={label}><dt>{calls.cursor?'已读取':''}{label}</dt><dd>{calls.ready?value:'—'}</dd></div>)}
  </dl>
  {calls.busy&&<ReadState>正在读取执行轮次…</ReadState>}
  {calls.error&&<Feedback kind={calls.ready?'warning':'error'} actions={<Button variant="outline" onClick={calls.retry}>重新读取执行轮次</Button>}>{calls.error}</Feedback>}
  {!available&&<p className="replica-inbox-caption">此任务尚未提供可关联的模型轮次，生命周期记录保留在下方。</p>}
  {calls.ready&&!rounds.length&&<p className="replica-inbox-caption">{['failed','dead_letter'].includes(task.status)?'尚无模型执行记录；任务可能在进入模型执行前已停止。':'尚无已记录的模型执行轮次。'}任务调度尝试与生命周期事件不计为模型轮次。</p>}
  {rounds.length>0&&<><p className="replica-inbox-caption">每行对应一次实际模型调用；未记录的轮次序号以阅读顺序展示。</p><ol className="replica-task-result-list">{rounds.map(({record,call,label,status}:any)=><li key={record.ref}>
   <Collapsible className="replica-task-result-row">
    <CollapsibleTrigger asChild><Button type="button" variant="ghost" className="replica-task-round-trigger"><span>{label}</span><DiagnosticStatus status={call.status} label={status==='success'?'成功':status==='failure'?'失败':undefined}/><span className="replica-task-round-duration">{callDurationLabel(call,{includeSource:false})}</span><ChevronDown size={14}/></Button></CollapsibleTrigger>
    <CollapsibleContent className="replica-task-round-content">
     {call.started_at&&<time dateTime={call.started_at}>{diagnosticInstant(call.started_at)}</time>}
     <section><h3>输入</h3><RecordView value={call.prompt} emptyText="未记录输入"/></section>
     <section><h3>输出</h3><RecordView value={call.response} emptyText="未记录输出"/></section>
     <section><h3>错误</h3>{call.error||call.error_code?<><RecordView value={call.error} emptyText="未提供错误说明"/>{call.error_code&&<JsonDetails value={{error_code:call.error_code}} title="错误代码"/>}</>:<p className="replica-inbox-caption">未记录错误</p>}</section>
     <JsonDetails value={record} title="本轮原始记录"/>
    </CollapsibleContent>
   </Collapsible>
  </li>)}</ol></>}
  {calls.cursor&&<Button variant="outline" disabled={calls.busy} onClick={calls.more}>读取更多执行轮次</Button>}
  {steps.length>0&&<Disclosure title={`生命周期记录 · ${task.has_more_steps?'已读取 ':''}${steps.length} 条`}><ol className="replica-task-history">{steps.map((entry:any,index:number)=><li key={entry.id??index}><h3>{taskEventText(entry)}</h3>{entry.at&&<time dateTime={entry.at}>{diagnosticInstant(entry.at)}</time>}{taskEventReason(entry)&&<p>{taskEventReason(entry)}</p>}</li>)}</ol><JsonDetails value={steps} title="生命周期原始记录"/></Disclosure>}
 </div>;
}

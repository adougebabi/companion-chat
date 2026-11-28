import {ChevronDown} from 'lucide-react';
import {Button} from '@/components/ui/button';
import {Collapsible,CollapsibleTrigger,CollapsibleContent} from '@/components/ui/collapsible';
import {Feedback,ReadState} from './Presentation';
import DiagnosticStatus from './DiagnosticStatus';
import RecordView from './RecordView';
import JsonDetails from './JsonDetails';
import RawContent from './RawContent';
import {diagnosticTimeLabel,taskEventText,taskEventReason,taskRoundRecords,taskModelFailureReason} from '@/approved-ui/task-presentation.mjs';
import {callDurationLabel,callPurpose,modelCallGroups} from '@/approved-ui/task-model-call-presentation.mjs';

/**
 * Physical model calls of one task. Each call is one request, never the task
 * outcome. Raw input/output stay collapsed; the first failed call opens so its
 * error is visible at once. Read lazily: only when this section is shown.
 * Reflection tasks show readable input/output plus separate diagnostic downloads.
 */
export default function TaskExecutionResults({task,calls,available,isReflection=false}:{task:any;calls:any;available:boolean;isReflection?:boolean}){
 const rounds=taskRoundRecords(calls.items),failures=rounds.filter((row:any)=>row.status==='failure'),firstFailure=failures[0]?.record.ref;
 const count=calls.ready?`${calls.cursor?'已读取 ':''}${rounds.length} 次${failures.length?` · ${failures.length} 次失败`:''}`:'';
 const groups=modelCallGroups(rounds.map((row:any)=>row.record)),byRecord=new Map(rounds.map((row:any)=>[row.record,row]));
 const hasGroups=groups.some((group:any)=>group.associated);
 const renderRound=({record,call,label,status}:any)=><li key={record.ref} data-call-status={status}>
    <Collapsible className="replica-task-result-row" defaultOpen={record.ref===firstFailure}>
     <CollapsibleTrigger asChild><Button type="button" variant="ghost" className="replica-task-round-trigger"><span>{label}</span><DiagnosticStatus status={call.status} label={status==='success'?'成功':status==='failure'?'失败':undefined}/><span className="replica-task-round-duration">{callDurationLabel(call,{includeSource:false})}</span><ChevronDown size={14}/></Button></CollapsibleTrigger>
     <CollapsibleContent className="replica-task-round-content">
      {call.started_at&&<time dateTime={call.started_at}>开始于 {diagnosticTimeLabel(call.started_at)}</time>}
      {(call.error||call.error_code)&&<section className="replica-task-round-error"><h3>错误</h3><p>{taskModelFailureReason(call)}</p>{!isReflection&&call.error_code&&<JsonDetails value={{error_code:call.error_code}} title="错误代码" downloadOnly/>}</section>}
      {isReflection?<>
       <section><h3>输入</h3><RecordView value={call.prompt} emptyText="未记录输入" hideEmpty/></section>
       <section><h3>输出</h3><RecordView value={call.response} emptyText="未记录输出" hideEmpty/></section>
       <JsonDetails value={record} title="原始任务记录（诊断）" filename="reflection-task.json" downloadOnly/>
       <JsonDetails value={call.prompt??null} title="模型输入（诊断）" filename="model-input.json" downloadOnly downloadEmpty downloadNull/>
       <JsonDetails value={call.response??null} title="模型输出（诊断）" filename="model-output.json" downloadOnly downloadEmpty downloadNull/>
      </>:<>
       <RawContent label="输入" value={call.prompt} missing="未记录输入"><RecordView value={call.prompt} emptyText="未记录输入"/></RawContent>
       <RawContent label="输出" value={call.response} missing="未记录输出"><RecordView value={call.response} emptyText="未记录输出"/></RawContent>
       <JsonDetails value={record} title="本轮原始记录" downloadOnly/>
      </>}
     </CollapsibleContent>
    </Collapsible>
   </li>;
 return <div className="replica-task-results">
  <section className="replica-task-calls" aria-label="模型调用">
   <div className="replica-task-section-head"><h3>模型调用</h3>{count&&<span>{count}</span>}</div>
   <p className="replica-inbox-caption">每条只代表一次模型请求的结果；任务是否完成以上方任务状态为准。</p>
   {calls.busy&&!calls.ready&&<ReadState>正在读取模型调用…</ReadState>}
   {calls.error&&<Feedback kind={calls.ready?'warning':'error'} actions={<Button variant="outline" onClick={calls.retry}>重新读取模型调用</Button>}>{calls.error}</Feedback>}
   {!available&&<p className="replica-inbox-caption">此任务没有可关联的模型调用记录。</p>}
   {calls.ready&&!rounds.length&&<p className="replica-inbox-caption">{['failed','dead_letter'].includes(task.status)?'当前未读到模型调用记录；不能据此判断任务停在哪个阶段。':'还没有模型调用记录。'}</p>}
   {rounds.length>0&&(hasGroups?groups.map((group:any)=><section key={group.key} className="ui-stack" aria-label={callPurpose(group.records[0].data).label+'调用过程'}><header className="ui-actions justify-between"><h4>{callPurpose(group.records[0].data).label}</h4>{groups.length>1&&<span className="text-xs text-muted-foreground">{group.summary}</span>}</header>{group.repeatedSequence&&<p className="text-xs text-muted-foreground">存在重复调用序号，各次记录分别保留。</p>}{group.missingTime&&<p className="text-xs text-muted-foreground">部分调用时间未记录，先后顺序待核对。</p>}<ol className="replica-task-result-list">{group.records.map((record:any)=>renderRound(byRecord.get(record)))}</ol></section>):<ol className="replica-task-result-list">{rounds.map(renderRound)}</ol>)}
   {calls.cursor&&<Button variant="outline" disabled={calls.busy} onClick={calls.more}>读取更多模型调用</Button>}
  </section>
 </div>;
}

/** Lifecycle events of the task itself (queued, started, retried, failed…). */
export function TaskLifecycle({task,steps}:{task:any;steps:any[]}){
 return <div className="replica-task-results">
  <section className="replica-task-lifecycle" aria-label="生命周期事件">
   <div className="replica-task-section-head"><h3>生命周期事件</h3>{steps.length>0&&<span>{task.has_more_steps?`已读取 ${steps.length} 条`:`${steps.length} 条`}</span>}</div>
   {steps.length>0?<ol className="replica-task-history">{steps.map((entry:any,index:number)=><li key={entry.id??index}>{entry.at&&<time dateTime={entry.at}>{diagnosticTimeLabel(entry.at)}</time>}<p className="replica-task-history-text">{taskEventText(entry)}</p>{taskEventReason(entry)&&<p className="replica-task-history-reason">{taskEventReason(entry)}</p>}</li>)}</ol>:<p className="replica-inbox-caption">尚无已记录的生命周期事件。</p>}
   {steps.length>0&&<JsonDetails value={steps} title="生命周期原始记录" downloadOnly/>}
  </section>
 </div>;
}

import {Button} from '@/components/ui/button';
import {Badge} from '@/components/ui/badge';
import {Feedback,ReadState} from './Presentation';
import useWorkspaceRead from './useWorkspaceRead';
import {useWorkspace} from '../page';
import {diagnosticInstant} from '@/approved-ui/task-presentation.mjs';
import {stateLabel} from '@/v3/domain-presentation.mjs';

/** Stored wire values stay intact, including null, empty structures and unknown fields. */
function StoredValue({value,missing}:{value:unknown;missing:string}){
 if(value===undefined)return <p className="replica-inbox-caption">{missing}</p>;
 return <>{missing&&(value===null||value==='')&&<p className="replica-inbox-caption">{missing}（存储值为{value===null?' null':'空字符串'}。）</p>}<pre className="replica-task-wire-value">{typeof value==='string'?value:JSON.stringify(value,null,2)}</pre></>;
}
export default function TaskModelCalls({task}:{task:any}){
 const w=useWorkspace(),correlationId=typeof task.correlation_id==='string'?task.correlation_id:'';
 const calls=useWorkspaceRead({identity:['task-model-calls',task.task_id,correlationId],characterScoped:false,enabled:!!correlationId,keyOf:(row:any)=>row.ref,read:(cursor:string)=>w.services.diagnosticsClient.page('model-runs',{correlationId,limit:20,...(cursor?{cursor}:{})})});
 return <section className="replica-task-model-calls" aria-label="任务关联的模型调用">
 <div className="replica-task-calls-heading"><h3>任务关联的模型调用</h3>{correlationId&&<Button variant="outline" disabled={calls.busy} onClick={()=>void calls.load()}>刷新调用记录</Button>}</div>
 {!correlationId?<p>本任务未提供模型调用关联标识，无法读取关联 Request / Response。</p>:<>
 {calls.busy&&!calls.ready&&<ReadState>正在读取关联的模型调用…</ReadState>}
 {calls.error&&<Feedback kind={calls.ready?'warning':'error'} title="模型调用读取失败" actions={<Button variant="outline" disabled={calls.busy} onClick={()=>void calls.retry()}>重试原读取</Button>}>{calls.error}</Feedback>}
 {calls.ready&&!calls.items.length&&<p>此关联标识下暂未读到模型调用记录，不能据此判断模型是否已被调用。</p>}
 {calls.items.map((record:any,index:number)=>{const call=record.data||{},hasCode=call.error_code!==undefined&&call.error_code!==null&&call.error_code!=='',hasError=call.error!==undefined&&call.error!==null&&call.error!=='';return <article className="replica-task-model-call" key={record.ref}>
 <header><h4>模型调用 {index+1}{call.model_id?' · '+call.model_id:''}</h4><Badge variant={['error','failed'].includes(call.status)?'destructive':'secondary'}>{stateLabel(call.status,'状态未记录')}</Badge></header>
 {(call.started_at||call.created_at)&&<p className="replica-inbox-caption">{diagnosticInstant(call.started_at||call.created_at)}</p>}
 {(hasCode||hasError)&&<section className="replica-task-call-error"><h5>调用错误</h5>{hasCode&&<StoredValue value={call.error_code} missing=""/>}{hasError&&<StoredValue value={call.error} missing=""/>}</section>}
 <div className="replica-task-call-payloads">
 <section><h5>Request · 请求</h5><StoredValue value={call.prompt} missing="此调用未记录 Request（请求）。"/></section>
 <section><h5>Response · 响应</h5><StoredValue value={call.response} missing="此调用未记录 Response（响应）；请结合调用状态和错误查看。"/></section>
 </div>
 </article>;})}
 {calls.cursor&&<Button variant="outline" disabled={calls.busy} onClick={()=>void calls.more()}>{calls.busy?'正在读取…':'加载更多模型调用'}</Button>}
 <p className="replica-inbox-caption">按任务关联标识读取；同一关联中可能包含多次执行和工具调用。</p>
 </>}
 </section>;
}

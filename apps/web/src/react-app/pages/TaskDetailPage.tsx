import {createPortal} from "react-dom";
import {ArrowLeft,RefreshCw,Loader2,Check,ArrowUpRight} from "lucide-react";
import {useCallback,useEffect,useRef,useState,useSyncExternalStore} from 'react';
import {Tabs,TabsList,TabsTrigger,TabsContent} from "@/components/ui/tabs";
import {Button} from '@/components/ui/button';
import {AsyncTaskStatus,DetailFields,type AsyncTaskState} from '@/components/patterns';
import WorkspacePageHeader from "../shared/WorkspacePageHeader";
import JsonDetails from "../shared/JsonDetails";
import {ReadState,Feedback} from "../shared/Presentation";
import WorkflowRecovery from '../shared/WorkflowRecovery';
import MediaTaskRecovery from '../shared/MediaTaskRecovery';
import TaskExecutionResults,{TaskLifecycle} from '../shared/TaskExecutionResults';
import useWorkspaceRead from '../shared/useWorkspaceRead';
import {useWorkspace} from '../page';
import {domainName,errorMessage,stateLabel} from '@/v3/domain-presentation.mjs';
import {diagnosticInstant,taskDisplayTitle,taskStatusPresentation,taskEventTitle,taskStageRecords,mediaTaskStageRecords,taskFailureReason,taskNeedsModelConfiguration,taskSourceHref,taskTypeLabel,taskEventReason,taskSourceLabel} from '@/approved-ui/task-presentation.mjs';

const asyncStates:Record<string,AsyncTaskState>={waiting:'queued',running:'running',completed:'succeeded',attention:'failed',ended:'cancelled'};
/** What the task itself has produced or where it stopped. Model calls never decide this. */
function taskOutcome(task:any,view:any,failure:string){
 const at=(value:unknown)=>typeof value==='string'&&value?diagnosticInstant(value):'';
 if(task.status==='cancel_requested')return '已请求取消，等待执行器确认；确认前任务可能仍在执行。';
 switch(view.group){
  case 'waiting':return task.next_attempt_at?`等待下次尝试（${at(task.next_attempt_at)}）。`:'已受理，尚未开始执行。';
  case 'running':return `${task.started_at?'已于 '+at(task.started_at)+' 开始，':''}仍在执行；完成前不会显示为已完成。`;
  case 'completed':return task.completed_at?`已于 ${at(task.completed_at)} 完成。`:'任务已完成。';
  case 'ended':return `${task.completed_at?'已于 '+at(task.completed_at)+' ':''}结束，不会继续执行。`;
  case 'attention':return failure||'任务已暂停或受阻，可在下方查看可用操作。';
  default:return '';
 }
}
export default function TaskDetailPage({route,embedded=false,onTaskLoaded,onTaskRecovered,controlTarget}:{route:any;embedded?:boolean;onTaskLoaded?:(task:any)=>void;onTaskRecovered?:(id:string)=>void;controlTarget?:HTMLElement|null}){
 const w=useWorkspace();useSyncExternalStore(w.subscribe,w.getSnapshot,w.getSnapshot);
 const owner=w.state.session.actorId,id=route.feature.params.taskId;
 const [detailTab,setDetailTab]=useState('progress'),[technicalOpen,setTechnicalOpen]=useState(false),[taskDetailsTarget,setTaskDetailsTarget]=useState<HTMLDivElement|null>(null),[taskValue,setTask]=useState<any>(null),[taskScope,setTaskScope]=useState(''),[preparation,setPreparation]=useState<any>(null),[workflowSnapshot,setWorkflowSnapshot]=useState<any>(null),[error,setError]=useState(''),[readAt,setReadAt]=useState(''),[loading,setLoading]=useState(true),active=useRef(false),epoch=useRef(0);
 const scope=JSON.stringify([owner,id]),task=taskScope===scope?taskValue:null;
 const callsAvailable=!!task?.correlation_id&&typeof w.services.diagnosticsClient?.page==='function';
 const isReflection=(task?.kind||task?.type)==='reflection.run';
 const isMedia=(task?.kind||task?.type)==='media.generation',mediaId=isMedia&&workflowSnapshot?.intent?.intent_type==='media.generation'?workflowSnapshot.intent.payload?.intent_id:'';
 const mediaReader=useWorkspaceRead({identity:['task-media',id,mediaId],characterScoped:false,enabled:!!mediaId&&typeof w.services.diagnosticsClient?.page==='function',keyOf:(row:any)=>row.ref,read:()=>w.services.diagnosticsClient.page('media-prompts',{correlationId:'media:'+mediaId,limit:20})});
 const media=(mediaReader.items as any[]).find((record:any)=>record.id===mediaId&&record.data?.id===mediaId)?.data;
 const read=useCallback(async()=>{const token=++epoch.current;setLoading(true);try{const value=await w.services.operationsClient.task(id);if(!active.current||w.state.session?.actorId!==owner||token!==epoch.current)return;setTaskScope(scope);setTask(value);setReadAt(new Date().toISOString());onTaskLoaded?.(value);setPreparation(null);setError('');if(taskSourceHref(value)&&typeof w.client?.inventoryPreparation==='function'){try{const current=await w.client.inventoryPreparation(value.fluctlight_id);if(active.current&&token===epoch.current&&w.state.session?.actorId===owner&&current.task_id===value.task_id&&current.fluctlight_id===value.fluctlight_id)setPreparation(current);}catch{/* The task remains readable independently of its current domain projection. */}}}catch(cause){if(active.current&&token===epoch.current&&w.state.session?.actorId===owner)setError(errorMessage(cause));}finally{if(active.current&&token===epoch.current)setLoading(false);}},[w,owner,id,scope,onTaskLoaded]);
 useEffect(()=>{active.current=true;setTask(null);onTaskLoaded?.(null);setPreparation(null);setWorkflowSnapshot(null);setError('');setDetailTab('progress');setTechnicalOpen(false);void read();return()=>{active.current=false;epoch.current++;};},[read]);
 const returnTo=route.query.get('returnTo'),back=/^#?\/(?!\/)/.test(returnTo||'')?returnTo.replace(/^#/,''):'/diag',initialization=back.startsWith('/library/create');
 const statusView=taskStatusPresentation(task||{}),failed=['failed','dead_letter'].includes(statusView.status);
 const steps=Array.isArray(task?.steps)?task.steps.map((entry:any,index:number)=>({entry,index})).sort((a:any,b:any)=>{const first=Date.parse(a.entry.at),second=Date.parse(b.entry.at);return Number.isFinite(first)&&Number.isFinite(second)?first-second||a.index-b.index:Number.isFinite(first)?-1:Number.isFinite(second)?1:a.index-b.index;}).map(({entry}:any)=>entry):[],stages=isMedia?mediaTaskStageRecords(task,media):taskStageRecords(task,preparation),failure=taskFailureReason(task,preparation),source=taskSourceHref(task),title=taskDisplayTitle(task,w.state.characters);
 const calls=useWorkspaceRead({identity:['task-execution-rounds',id,task?.correlation_id],characterScoped:false,enabled:detailTab==='logs'&&callsAvailable,keyOf:(row:any)=>row.ref,read:(cursor:string)=>w.services.diagnosticsClient.page('model-runs',{correlationId:task.correlation_id,limit:20,...(cursor?{cursor}:{})})});
 const callsHref=task?.correlation_id?'#/diag?view=calls&correlationId='+encodeURIComponent(task.correlation_id)+'&returnTo='+encodeURIComponent('#/diag?view=tasks&taskId='+encodeURIComponent(id)+(returnTo?'&returnTo='+encodeURIComponent(returnTo):'')):'';
 const character=w.state.characters?.find((entry:any)=>entry.id===task?.fluctlight_id)?.name;
 const completed=stages.filter((entry:any)=>['completed','succeeded'].includes(entry.status)).length;
 const domainControl=source&&<div className="replica-task-recovery-controls"><Button asChild><a href={source}>{taskSourceLabel(task)}</a></Button></div>;
 const row=(entry:any,index:number,stage=false)=><li key={entry.id||index} className="replica-task-stage"><span className={"replica-stage-number"+(entry.status==='running'?' active':'')}>{entry.status==='running'?<Loader2 size={13} className="animate-spin"/>:['completed','succeeded'].includes(entry.status)?<Check size={13}/>:index+1}</span><div><h3>{stage?domainName(entry.name,'已记录环节'):taskEventTitle(entry)}</h3><p>{stateLabel(entry.status,'已记录')}{entry.reason_code?' · '+taskEventReason(entry):''}</p>{entry.detail&&<p>{entry.detail}</p>}{entry.at&&<time dateTime={entry.at} title={diagnosticInstant(entry.at)}>{diagnosticInstant(entry.at)}</time>}</div></li>;
 const recovered=(nextId:string)=>{if(onTaskRecovered)onTaskRecovered(nextId);else w.navigatePath('/diag/task/'+encodeURIComponent(nextId)+(returnTo?'?returnTo='+encodeURIComponent(returnTo):''));};
 const refresh=async()=>{await read();if(calls.ready)void calls.load();};
 const outcome=task?taskOutcome(task,statusView,failure):'';
 const refreshButton=<Button type="button" variant="link" size="sm" className="replica-task-refresh" disabled={loading} onClick={()=>void refresh()}>{loading?<Loader2 size={13} className="animate-spin"/>:<RefreshCw size={13}/>}{loading?'正在刷新…':'刷新任务状态'}</Button>;
 const readStamp=<span className="replica-task-read-at">{readAt&&<>状态读取于 <time dateTime={readAt}>{new Date(readAt).toLocaleTimeString('zh-CN',{hour12:false})}</time></>}{refreshButton}</span>;
 return <section className="replica-task-detail" data-testid="task-detail">
 {!embedded&&<WorkspacePageHeader eyebrow="WORKSPACE / FOLLOW THE PROGRESS" title={title} description={task?[character,taskTypeLabel(task)].filter(Boolean).join(' · '):'正在读取任务…'} actions={<Button type="button" variant="outline" onClick={()=>w.navigatePath(back)}><ArrowLeft size={14}/>{initialization?'返回初始化流程':'返回来源页'}</Button>}/>}
 {error&&<Feedback kind={task?'warning':'error'} title={task?'任务状态刷新失败':'任务暂时无法读取'} actions={<Button variant="outline" disabled={loading} onClick={()=>void read()}>重新读取任务</Button>}>{error}{task&&<span className="block">下方仍是上次读到的状态。</span>}</Feedback>}{loading&&!task&&<ReadState>正在读取任务…</ReadState>}
 {task&&<><div className={"replica-task-summary"+(embedded?" replica-task-summary-embedded":"")}>
 <div className="replica-task-overview" data-task-group={statusView.group}>
  {statusView.group==='unknown'?<Feedback kind="warning" title="状态待确认">{'后台返回的状态无法识别，可刷新后再看。'}{readStamp}</Feedback>:<AsyncTaskStatus state={asyncStates[statusView.group]} title={statusView.label} message={<>{outcome}{statusView.warning&&<span className="block">{statusView.warning}</span>}</>} updatedAt={readStamp}/>}
  {task.kind==='cognition.processing'&&(task.workflow_status||task.source?.workflow_status)==='completed'&&statusView.status==='failed'&&<Feedback kind="information">任务执行已经结束，但聊天回合失败。模型生成内容不代表回复已发送；消息结果以当前回合记录为准。</Feedback>}
  {failed&&taskNeedsModelConfiguration(task,preparation)&&<Button asChild variant="outline" className="self-start"><a href={'#/settings?section='+encodeURIComponent('模型用途')+'&returnTo='+encodeURIComponent(initialization?returnTo||back:'#/diag?view=tasks&taskId='+encodeURIComponent(id))}>{source?'去配置通用模型':'去配置模型用途'}</a></Button>}
 </div>
 <Tabs value={detailTab} onValueChange={value=>{setDetailTab(value);if(value!=='input')setTechnicalOpen(false);}}><TabsList variant="line"><TabsTrigger value="progress">执行阶段</TabsTrigger><TabsTrigger value="logs">模型调用</TabsTrigger><TabsTrigger value="input">任务信息</TabsTrigger></TabsList>
 <TabsContent value="progress" className="replica-task-progress-panel">{stages.length>0&&<><div className="replica-task-progress" role="progressbar" aria-label="已记录阶段" aria-valuenow={completed} aria-valuemin={0} aria-valuemax={stages.length}><i style={{width:`${completed/stages.length*100}%`}}/></div><ol className="replica-task-stages">{stages.map((entry:any,index:number)=>row(entry,index,true))}</ol></>}<TaskLifecycle task={task} steps={steps}/>{task.has_more_steps&&<Button type="button" variant="link" onClick={()=>w.navigatePath('/diag/lifecycle?intentId='+encodeURIComponent(task.task_id||id))}>查看完整生命周期</Button>}</TabsContent>
 <TabsContent value="logs"><TaskExecutionResults task={task} calls={calls} available={callsAvailable} downloadOnlyModelRecords={isReflection}/></TabsContent>
 <TabsContent value="input"><DetailFields items={[
  {label:'任务类型',value:taskTypeLabel(task)},
  {label:'关联角色',value:character||(task.fluctlight_id?'角色名称未读到':'未关联角色')},
  ...(Number.isInteger(task.runs)?[{label:'执行尝试',value:`${task.runs} 次`}]:[]),
  ...[['创建时间',task.created_at],['开始时间',task.started_at],['结束时间',task.completed_at],['下次尝试时间',task.next_attempt_at]].filter(([,value])=>value&&(!isReflection||typeof value==='string'&&Number.isFinite(Date.parse(value)))).map(([label,value])=>({label,value:<time dateTime={value}>{diagnosticInstant(value)}</time>})),
  ...(!task.workflow_id&&source?[{label:'处理入口',value:taskSourceLabel(task)}]:[]),
 ]}/>{task.workflow_id&&<div ref={setTaskDetailsTarget} className="replica-task-info-recovery"/>}{detailTab==='input'&&<div className="replica-task-source-info">{callsHref&&<Button asChild variant="link"><a href={callsHref}>查看关联模型调用 <ArrowUpRight size={12}/></a></Button>}<Button type="button" variant="link" aria-expanded={technicalOpen} onClick={()=>setTechnicalOpen(value=>!value)}>{technicalOpen?'收起任务原始记录':'查看完整任务记录'}</Button>{technicalOpen&&<><JsonDetails value={task} title={isReflection?"原始反思任务记录（诊断）":"完整任务信息与关联来源"} downloadOnly/>{(!isReflection||stages.length>0)&&<JsonDetails value={stages} title="业务阶段依据与来源" downloadOnly/>}</>}</div>}</TabsContent>
 </Tabs>
 </div>{task.workflow_id?<WorkflowRecovery key={owner+':'+task.workflow_id} workflowId={task.workflow_id} onChanged={async()=>{await read();if(isMedia&&mediaId)await mediaReader.load();}} onSnapshot={setWorkflowSnapshot} hideRecoveryLink={!!mediaId||workflowSnapshot?.recovery?.href==='/diag/task/'+encodeURIComponent(task.task_id)} presentation="task" controlTarget={controlTarget} detailsTarget={taskDetailsTarget} showDetails={detailTab==='input'} showTechnical={technicalOpen} domainRecovery={source?{href:source,label:taskSourceLabel(task)}:undefined}/>:domainControl&&(controlTarget?createPortal(domainControl,controlTarget):domainControl)}{isMedia&&mediaId&&<MediaTaskRecovery key={owner+':'+mediaId} mediaId={mediaId} workflowId={task.workflow_id} snapshot={workflowSnapshot} reader={mediaReader} controlTarget={controlTarget} onRecovered={recovered}/>}</>}
 </section>;
}

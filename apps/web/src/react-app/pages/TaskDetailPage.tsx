import TaskModelCalls from "../shared/TaskModelCalls";
import WorkspacePageHeader from "../shared/WorkspacePageHeader";
import {Tabs,TabsList,TabsTrigger,TabsContent} from "@/components/ui/tabs";
import JsonDetails from "../shared/JsonDetails";
import {ArrowLeft,RefreshCw,Check} from "lucide-react";
import {ReadState,Feedback} from "../shared/Presentation";
import {useCallback,useEffect,useRef,useState,useSyncExternalStore} from 'react';
import {Button} from '@/components/ui/button';
import {Badge} from '@/components/ui/badge';
import WorkflowRecovery from '../shared/WorkflowRecovery';
import {useWorkspace} from '../page';
import {domainName,errorMessage,stateLabel} from '@/v3/domain-presentation.mjs';
import {diagnosticInstant,taskDisplayTitle,taskEventTitle,taskStageRecords,taskFailureReason,taskNeedsModelConfiguration,taskSourceHref} from '@/approved-ui/task-presentation.mjs';
export default function TaskDetailPage({route,embedded=false,onTaskLoaded}:{route:any;embedded?:boolean;onTaskLoaded?:(task:any)=>void}){
 const w=useWorkspace();useSyncExternalStore(w.subscribe,w.getSnapshot,w.getSnapshot);
 const owner=w.state.session.actorId,id=route.feature.params.taskId;
 const [detailTab,setDetailTab]=useState('logs'),[taskValue,setTask]=useState<any>(null),[taskScope,setTaskScope]=useState(''),[preparation,setPreparation]=useState<any>(null),[error,setError]=useState(''),[loading,setLoading]=useState(true),active=useRef(false),epoch=useRef(0);
 const scope=JSON.stringify([owner,id]),task=taskScope===scope?taskValue:null;
 const read=useCallback(async()=>{const token=++epoch.current;setLoading(true);try{const value=await w.services.operationsClient.task(id);if(!active.current||w.state.session?.actorId!==owner||token!==epoch.current)return;setTaskScope(scope);setTask(value);onTaskLoaded?.(value);setPreparation(null);setError('');if(taskSourceHref(value)&&typeof w.client?.inventoryPreparation==='function'){try{const current=await w.client.inventoryPreparation(value.fluctlight_id);if(active.current&&token===epoch.current&&w.state.session?.actorId===owner&&current.task_id===value.task_id&&current.fluctlight_id===value.fluctlight_id)setPreparation(current);}catch{/* The task remains readable independently of its current domain projection. */}}}catch(cause){if(active.current&&token===epoch.current&&w.state.session?.actorId===owner)setError(errorMessage(cause));}finally{if(active.current&&token===epoch.current)setLoading(false);}},[w,owner,id,scope,onTaskLoaded]);
 useEffect(()=>{active.current=true;setTask(null);onTaskLoaded?.(null);setPreparation(null);setError('');setDetailTab('logs');void read();return()=>{active.current=false;epoch.current++;};},[read]);
 const returnTo=route.query.get('returnTo'),back=/^#?\/(?!\/)/.test(returnTo||'')?returnTo.replace(/^#/,''):'/diag',initialization=back.startsWith('/library/create');
 const failed=['failed','dead_letter'].includes(task?.status),times=task?[['创建',task.created_at],['开始',task.started_at],['结束',task.completed_at]].filter(([,value])=>value):[],elapsed=task?.started_at&&task?.completed_at?Math.max(0,Math.round((Date.parse(task.completed_at)-Date.parse(task.started_at))/1000)):null;
 const steps=Array.isArray(task?.steps)?task.steps:[],stages=taskStageRecords(task,preparation),failure=taskFailureReason(task,preparation),source=taskSourceHref(task),title=taskDisplayTitle(task,w.state.characters);
 const callsHref=task?.correlation_id?'#/diag?view=calls&correlationId='+encodeURIComponent(task.correlation_id)+'&returnTo='+encodeURIComponent('#/diag?view=tasks&taskId='+encodeURIComponent(id)+(returnTo?'&returnTo='+encodeURIComponent(returnTo):'')):'';
 const row=(entry:any,index:number,stage=false)=><li key={entry.id||index} className="replica-task-stage"><span className="replica-stage-number">{['completed','succeeded'].includes(entry.status)?<Check size={13}/>:index+1}</span><div><h3>{stage?domainName(entry.name,'已记录环节'):taskEventTitle(entry)}</h3><p>{stateLabel(entry.status,'已记录')}{entry.reason_code?' · '+errorMessage({code:entry.reason_code},'具体原因请查看本条原始记录。'):''}</p>{entry.at&&<time dateTime={entry.at} title={entry.at}>{diagnosticInstant(entry.at)}</time>}<JsonDetails value={entry.source||entry} title={stage?'阶段来源':'本条记录详情'}/></div></li>;
 return <section className="replica-task-detail" data-testid="task-detail">
 {!embedded&&<WorkspacePageHeader eyebrow="WORKSPACE / FOLLOW THE PROGRESS" title={title} description="查看实际任务与执行记录，已有角色、世界和已确认结果会保留。" actions={<><Button type="button" variant="outline" onClick={()=>w.navigatePath(back)}><ArrowLeft size={14}/>{initialization?'返回初始化流程':'返回来源页'}</Button><Button type="button" variant="outline" disabled={loading} onClick={()=>void read()}><RefreshCw size={14}/>重新读取任务</Button></>}/>}
 {error&&<Feedback kind={task?'warning':'error'} actions={<Button variant="outline" onClick={()=>void read()}>重新读取任务</Button>}>{error}</Feedback>}{loading&&!task&&<ReadState>正在读取任务…</ReadState>}
 {task&&<><div className={"replica-task-summary"+(embedded?" replica-task-summary-embedded":"")}><div className="replica-task-meta"><div className="flex flex-wrap items-center justify-between gap-3"><div><Badge variant={failed?'destructive':'secondary'}>{stateLabel(task.status)}</Badge></div><p>{Number.isInteger(task.runs)?`已尝试 ${task.runs} 次`:''}{elapsed!==null?` · 用时 ${elapsed} 秒`:''}</p></div></div><div className="grid gap-4">

 <Tabs value={detailTab} onValueChange={setDetailTab}><TabsList variant="line"><TabsTrigger value="progress">执行阶段</TabsTrigger><TabsTrigger value="logs">执行记录</TabsTrigger><TabsTrigger value="input">任务信息</TabsTrigger></TabsList>
 <TabsContent value="progress"> {failed&&<Feedback kind="error" title="本次任务未完成" actions={taskNeedsModelConfiguration(task,preparation)&&<Button asChild variant="outline"><a href={'#/settings?section='+encodeURIComponent('模型用途')+'&returnTo='+encodeURIComponent(initialization?returnTo||back:'#/diag?view=tasks&taskId='+encodeURIComponent(id))}>{source?'去配置通用模型':'去配置模型用途'}</a></Button>}>{failure}</Feedback>}{stages.length?<ol className="replica-task-stages">{stages.map((entry:any,index:number)=>row(entry,index,true))}</ol>:<p className="replica-inbox-caption">本任务尚未提供独立的阶段进度；实际状态变化保留在任务信息的生命周期记录中。</p>}<p className="replica-inbox-caption">仅显示已记录的环节状态，排队和已受理不表示完成。</p></TabsContent>
 <TabsContent value="logs"><TaskModelCalls key={owner+':'+task.task_id+':'+(task.correlation_id||'')} task={task}/></TabsContent>
 <TabsContent value="input"><dl className="replica-task-times">{times.map(([label,value])=><div key={label}><dt>{label}</dt><dd>{diagnosticInstant(value)}</dd></div>)}</dl>{callsHref&&<Button asChild variant="outline"><a href={callsHref}>查看本次模型调用与输入输出</a></Button>}<JsonDetails value={steps} title="生命周期记录（技术详情）"/><JsonDetails value={task} title="完整任务信息与关联来源"/></TabsContent>
 </Tabs>{task.has_more_steps&&<Button type="button" variant="link" onClick={()=>w.navigatePath('/diag/lifecycle?intentId='+encodeURIComponent(task.task_id||id))}>查看完整生命周期</Button>}
 </div></div>{task.workflow_id?<div className="replica-workflow-recovery"><WorkflowRecovery workflowId={task.workflow_id} onChanged={read} domainRecovery={source?{href:source,label:"查看初始衣柜与恢复"}:undefined}/></div>:source&&<Button asChild><a href={source}>前往初始衣柜恢复入口</a></Button>}</>}
 </section>;
}

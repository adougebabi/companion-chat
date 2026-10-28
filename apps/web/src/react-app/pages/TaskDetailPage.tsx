import WorkspacePageHeader from "../shared/WorkspacePageHeader";
import {Tabs,TabsList,TabsTrigger,TabsContent} from "@/components/ui/tabs";
import JsonDetails from "../shared/JsonDetails";
import {ArrowLeft,RefreshCw,Check,Clock3} from "lucide-react";
import {ReadState} from "@/react-app/shared/Presentation";
import {useCallback,useEffect,useRef,useState} from 'react';
import {Button} from '@/components/ui/button';
import {Card,CardHeader,CardTitle,CardContent} from '@/components/ui/card';
import {Badge} from '@/components/ui/badge';
import {Alert,AlertDescription} from '@/components/ui/alert';
import {Progress} from '@/components/ui/progress';
import {Disclosure} from '../controls';
import WorkflowRecovery from '../shared/WorkflowRecovery';
import {useWorkspace} from '../page';
import {domainName,errorMessage,stateLabel} from '@/v3/domain-presentation.mjs';
import {userInstant} from '@/v3/presentation-time.mjs';
export default function TaskDetailPage({route}:{route:any}){
 const w=useWorkspace(),owner=w.state.session.actorId,id=route.feature.params.taskId;
 const [detailTab,setDetailTab]=useState('progress');
 const [task,setTask]=useState<any>(null),[error,setError]=useState(''),[loading,setLoading]=useState(true),active=useRef(false),epoch=useRef(0);
 const read=useCallback(async()=>{const token=++epoch.current;setLoading(true);try{const value=await w.services.operationsClient.task(id);if(active.current&&w.state.session?.actorId===owner&&token===epoch.current){setTask(value);setError('');}}catch(cause){if(active.current&&token===epoch.current)setError(errorMessage(cause));}finally{if(active.current&&token===epoch.current)setLoading(false);}},[w,owner,id]);
 useEffect(()=>{active.current=true;void read();return()=>{active.current=false;epoch.current++;};},[read]);
 const returnTo=route.query.get('returnTo'),back=/^#?\/(?!\/)/.test(returnTo||'')?returnTo.replace(/^#/,''):'/diag';
 const raw=task?.source?.last_error||task?.last_error,failed=['failed','dead_letter'].includes(task?.status),complete=['completed','succeeded'].includes(task?.status);
 const times=task?[['创建',task.created_at],['开始',task.started_at],['结束',task.completed_at]].filter(([,value])=>value):[];
 const elapsed=task?.started_at&&task?.completed_at?Math.max(0,Math.round((Date.parse(task.completed_at)-Date.parse(task.started_at))/1000)):null;
 const steps=Array.isArray(task?.steps)?task.steps:[];
 const step=(entry:any,index:number)=><li key={entry.id||index} className="replica-task-stage"><span className="replica-stage-number">{['completed','succeeded'].includes(entry.status)?<Check size={13}/>:index+1}</span><div><h3>{domainName(entry.event_type||entry.name,'执行阶段')}</h3><p>{stateLabel(entry.status,'已记录')}{entry.reason_code?' · '+domainName(entry.reason_code,'原因已记录'):''}</p><time>{userInstant(entry.at)}</time></div></li>;
 return <section className="replica-task-detail" data-testid="task-detail">
 <WorkspacePageHeader eyebrow="WORKSPACE / FOLLOW THE PROGRESS" title={task?domainName(task.kind||task.type||task.title,'后台任务'):'原任务与恢复'} description="保留原任务和实际执行记录，沿已确认的输入继续。" actions={<><Button type="button" variant="outline" onClick={()=>w.navigatePath(back)}><ArrowLeft size={14}/>返回来源页</Button><Button type="button" variant="outline" disabled={loading} onClick={()=>void read()}><RefreshCw size={14}/>重新读取任务</Button></>}/>
 {error&&<Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}{loading&&!task&&<ReadState role="status">正在读取任务…</ReadState>}
 {task&&<><Card className="replica-task-summary"><CardHeader><div className="flex flex-wrap items-center justify-between gap-3"><Badge variant={failed?'destructive':'secondary'}>{stateLabel(task.status)}</Badge><p>{Number.isInteger(task.runs)?`已尝试 ${task.runs} 次`:''}{elapsed!==null?` · 用时 ${elapsed} 秒`:''}</p></div></CardHeader><CardContent>
 <Tabs value={detailTab} onValueChange={setDetailTab}><TabsList variant="line"><TabsTrigger value="progress">执行阶段</TabsTrigger><TabsTrigger value="logs">执行记录 <Badge variant="secondary">{steps.length}</Badge></TabsTrigger><TabsTrigger value="input">任务信息</TabsTrigger></TabsList>
 <TabsContent value="progress">{failed&&<Alert variant="destructive"><AlertDescription>{errorMessage(raw||'','任务未完成，请核对下方可用恢复操作。')}</AlertDescription></Alert>}{steps.length?<ol className="replica-task-stages">{steps.map(step)}</ol>:<p className="replica-inbox-caption">暂未读到阶段记录，请重新读取任务状态。</p>}<p className="replica-inbox-caption">只展示服务端已返回的阶段；排队和已受理不表示完成。</p></TabsContent>
 <TabsContent value="logs">{steps.length?<ol className="replica-task-stages">{steps.map(step)}</ol>:<p>暂未读到执行记录。</p>}</TabsContent>
 <TabsContent value="input"><dl className="replica-task-times">{times.map(([label,value])=><div key={label}><dt>{label}</dt><dd>{userInstant(value)}</dd></div>)}</dl><JsonDetails value={task} title="技术详情与实际任务信息"/></TabsContent>
 </Tabs>{task.has_more_steps&&<Button type="button" variant="link" onClick={()=>w.navigatePath('/diag/lifecycle?intentId='+encodeURIComponent(task.task_id||id))}>查看完整生命周期</Button>}
 </CardContent></Card>{task.workflow_id&&<div className="replica-workflow-recovery"><WorkflowRecovery workflowId={task.workflow_id} onChanged={read}/></div>}</>}
 </section>;
}

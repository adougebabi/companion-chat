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
 const [task,setTask]=useState<any>(null),[error,setError]=useState(''),[loading,setLoading]=useState(true),active=useRef(false),epoch=useRef(0);
 const read=useCallback(async()=>{const token=++epoch.current;setLoading(true);try{const value=await w.services.operationsClient.task(id);if(active.current&&w.state.session?.actorId===owner&&token===epoch.current){setTask(value);setError('');}}catch(cause){if(active.current&&token===epoch.current)setError(errorMessage(cause));}finally{if(active.current&&token===epoch.current)setLoading(false);}},[w,owner,id]);
 useEffect(()=>{active.current=true;void read();return()=>{active.current=false;epoch.current++;};},[read]);
 const returnTo=route.query.get('returnTo'),back=/^#?\/(?!\/)/.test(returnTo||'')?returnTo.replace(/^#/,''):'/diag';
 const raw=task?.source?.last_error||task?.last_error,failed=['failed','dead_letter'].includes(task?.status),complete=['completed','succeeded'].includes(task?.status);
 const times=task?[['创建',task.created_at],['开始',task.started_at],['结束',task.completed_at]].filter(([,value])=>value):[];
 const elapsed=task?.started_at&&task?.completed_at?Math.max(0,Math.round((Date.parse(task.completed_at)-Date.parse(task.started_at))/1000)):null;
 const steps=Array.isArray(task?.steps)?task.steps:[];
 const step=(entry:any,index:number)=><li key={entry.id||index} className="flex flex-wrap justify-between gap-2 border-b py-2"><span>{domainName(entry.event_type||entry.name,'执行阶段')} · {stateLabel(entry.status,'已记录')}</span><time>{userInstant(entry.at)}</time></li>;
 return <section className="grid min-w-0 gap-5" data-testid="task-detail"><div className="flex flex-wrap gap-3"><Button type="button" variant="outline" onClick={()=>w.navigatePath(back)}>返回来源页</Button><Button type="button" variant="outline" disabled={loading} onClick={()=>void read()}>重新读取任务</Button></div>{error&&<Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}{loading&&!task&&<ReadState role="status">正在读取任务…</ReadState>}{task&&<><Card><CardHeader><div className="flex flex-wrap items-center gap-3"><CardTitle>{domainName(task.kind||task.type||task.title,'后台任务')}</CardTitle><Badge variant={failed?'destructive':'secondary'}>{stateLabel(task.status)}</Badge></div><p className="text-sm text-muted-foreground">{Number.isInteger(task.runs)?`第 ${task.runs} 次尝试`:''}{elapsed!==null?` · 用时 ${elapsed>=60?Math.floor(elapsed/60)+' 分 ':''}${elapsed%60} 秒`:''}</p></CardHeader><CardContent className="grid gap-4"><Progress value={(complete?3:task.started_at?2:1)/3*100} aria-label="排队、执行、完成阶段"/><ol className="flex justify-between text-sm"><li>排队</li><li>{failed?'执行失败':'执行'}</li><li>{complete?'已完成':'完成'}</li></ol>{failed&&<Alert variant="destructive"><AlertDescription>{errorMessage(raw||'','任务未完成，请核对下方可用恢复操作。')}</AlertDescription></Alert>}<p className="text-sm text-muted-foreground">{times.map(([label,value])=>`${label} ${userInstant(value)}`).join(' · ')}</p><Disclosure title="技术详情"><pre className="approved-json">{JSON.stringify(task,null,2)}</pre></Disclosure></CardContent></Card>{task.workflow_id&&<WorkflowRecovery workflowId={task.workflow_id} onChanged={read}/>}<Card><CardHeader><CardTitle>最近执行阶段</CardTitle></CardHeader><CardContent>{steps.length?<><ol>{steps.slice(0,3).map(step)}</ol>{steps.length>3&&<Disclosure title={`更多已读取阶段（${steps.length-3} 条）`}><ol>{steps.slice(3).map(step)}</ol></Disclosure>}</>:<p>暂未读到阶段记录，请重新读取任务状态。</p>}{task.has_more_steps&&<Button type="button" variant="link" onClick={()=>w.navigatePath('/diag/lifecycle?intentId='+encodeURIComponent(task.task_id||id))}>查看完整生命周期</Button>}</CardContent></Card></>}</section>;
}

import {useEffect,useRef} from 'react';
import {Activity,ArrowRight,ArrowLeft,CheckCircle2,Clock3,RefreshCw,Server} from 'lucide-react';
import {Button} from '@/components/ui/button';
import {Tabs,TabsList,TabsTrigger} from '@/components/ui/tabs';
import {Input} from '@/components/ui/input';
import {Surface,ChoiceSelect,Badge} from '../controls';
import {useWorkspace} from '../page';
import useWorkspaceRead from './useWorkspaceRead';
import useManagementView from './useManagementView';
import useWorkspaceHash from './useWorkspaceHash';
import TaskRecordDialog from './TaskRecordDialog';
import {EmptyState,InlineFeedback} from './Presentation';
import LoadingState from './LoadingState';
import JsonDetails from './JsonDetails';
import {domainName,stateLabel} from '@/v3/domain-presentation.mjs';
import {diagnosticInstant} from '@/approved-ui/task-presentation.mjs';
export default function DiagnosticsWorkspace({view,children}){
 const taskTrigger=useRef(null),w=useWorkspace(),hash=useWorkspaceHash(),params=new URLSearchParams(hash.split('?')[1]||''),requested=params.get('view'),taskId=params.get('taskId'),returnTo=params.get('returnTo'),back=/^#?\/(?!\/)/.test(returnTo||'')?returnTo.replace(/^#/,''):'',initialization=back.startsWith('/library/create');
 const [preferences,setPreferences]=useManagementView('diagnostics',{mode:['tasks','calls','health'].includes(requested)?requested:params.has('correlationId')?'calls':'tasks',query:'',status:''});
 const {mode,query,status}=preferences,setMode=mode=>setPreferences({mode}),setQuery=query=>setPreferences({query}),setStatus=status=>setPreferences({status});
 useEffect(()=>{if(['tasks','calls','health'].includes(requested))setMode(requested);},[hash]);
 const tasks=useWorkspaceRead({identity:'diagnostic-tasks',characterScoped:false,enabled:mode==='tasks',keyOf:row=>row.ref,read:cursor=>w.services.diagnosticsClient.page('workflow-intents',{limit:20,...(cursor?{cursor}:{})})});
 const health=useWorkspaceRead({identity:'diagnostic-readiness',characterScoped:false,enabled:mode==='health',read:async()=>({items:[],result:await w.services.operationsClient.health(),next_cursor:''})});
 const rows=tasks.items.filter(row=>(!status||row.data?.status===status)&&(!query.trim()||JSON.stringify([row.data?.title,row.data?.intent_type,row.data?.fluctlight_id,row.ref]).toLowerCase().includes(query.trim().toLowerCase())));
 const failures=tasks.items.filter(r=>['failed','dead_letter','paused'].includes(r.data?.status)).length,running=tasks.items.filter(r=>['running','processing'].includes(r.data?.status)).length,completed=tasks.items.filter(r=>['completed','succeeded'].includes(r.data?.status)).length;
 const navigate=next=>{const query=new URLSearchParams([...params].filter(([key])=>['view','taskId','correlationId','returnTo'].includes(key)));for(const [key,value] of Object.entries(next)){if(value===null)query.delete(key);else query.set(key,String(value));}w.navigatePath('/diag?'+query);};
 return <div className="replica-diagnostics"><div className="replica-settings-head"><div><h2>诊断与任务</h2><p>查看执行到了哪里，在需要时继续原任务。</p></div><Activity size={25}/></div>
 {back&&<Surface className="replica-diagnostic-context"><div><h3>{initialization?'新相识 · 初始化任务':'来自业务页面的任务记录'}</h3><p>已有资料和原任务会保留，配置与恢复操作在实际业务入口完成。</p></div><Button variant="outline" onClick={()=>w.navigatePath(back)}><ArrowLeft size={14}/>{initialization?'返回初始化流程':'返回来源页'}</Button></Surface>}
 <Tabs value={mode} onValueChange={value=>{setMode(value);navigate({view:value,taskId:null});}}><TabsList variant="line"><TabsTrigger value="tasks">任务中心</TabsTrigger><TabsTrigger value="calls">模型调用记录</TabsTrigger><TabsTrigger value="health">服务诊断</TabsTrigger></TabsList></Tabs>
 {mode==='calls'&&<div className="replica-diagnostic-calls">{children}</div>}
 {mode==='tasks'&&<><div className="replica-data-stats">{[['需要处理',failures,'已读取的失败或暂停任务'],['正在执行',running,'以服务端记录为准'],['已经完成',completed,'已读取的完成记录']].map(([title,count,hint])=><Surface key={title}><span>{title}</span><strong>{tasks.ready?count:'—'}</strong><small>{hint}</small></Surface>)}</div>
 <Surface><div className="replica-task-toolbar"><Input aria-label="搜索任务或角色" placeholder="搜索任务或角色…" value={query} onChange={e=>setQuery(e.target.value)}/><ChoiceSelect aria-label="任务状态" value={status} onChange={e=>setStatus(e.target.value)}><option value="">所有状态</option>{[...new Set(tasks.items.map(r=>r.data?.status).filter(Boolean))].map(value=><option value={value} key={value}>{stateLabel(value)}</option>)}</ChoiceSelect><Button variant="ghost" onClick={tasks.load} disabled={tasks.busy} aria-label="重新读取运行任务"><RefreshCw size={15}/></Button></div>
 {tasks.busy&&<LoadingState label="正在读取运行任务…"/>}{tasks.error&&<InlineFeedback kind={tasks.ready?'warning':'error'}>{tasks.error}<Button variant="outline" onClick={tasks.retry}>重新读取原任务页</Button></InlineFeedback>}
 {rows.map(row=>{const id=row.data?.task_id||row.data?.intent_id||row.data?.id,name=w.state.characters.find(c=>c.id===row.data?.fluctlight_id)?.name;return <button key={row.ref} className="replica-task-row" onClick={event=>{taskTrigger.current=event.currentTarget;if(id)navigate({view:'tasks',taskId:id});else w.navigatePath('/diag/record/workflow-intents/'+encodeURIComponent(row.ref));}}><span className="replica-task-icon">{['completed','succeeded'].includes(row.data?.status)?<CheckCircle2 size={17}/>:<Clock3 size={17}/>}</span><div><h3>{name?name+' · ':''}{domainName(row.data?.title||row.data?.intent_type,'后台任务')}</h3><p>{diagnosticInstant(row.occurred_at||row.data?.created_at)} · 记录 {String(id||row.ref).slice(-8)}</p></div><Badge variant={['failed','dead_letter'].includes(row.data?.status)?'destructive':'secondary'}>{stateLabel(row.data?.status)}</Badge><ArrowRight size={14}/></button>;})}
 {tasks.ready&&!rows.length&&<EmptyState><h3>{query||status?'没有符合筛选的任务':'暂时没有运行任务'}</h3><p>实际任务记录会出现在这里；没有记录不代表业务从未发生。</p></EmptyState>}{tasks.cursor&&<Button variant="outline" disabled={tasks.busy} onClick={tasks.more}>读取更多任务</Button>}<Button variant="link" onClick={view.openTaskDiagnostics}>查看完整待处理事项与来源</Button></Surface></>}
 {mode==='health'&&<Surface><div className="replica-settings-head"><div><h2>工作区服务</h2><p>检查当前连接状态，模型与图片服务使用下方已保存的配置分别测试。</p></div><Button variant="outline" onClick={health.load} disabled={health.busy}>重新检查</Button></div>{health.busy&&<LoadingState label="正在读取服务状态…"/>}{health.error&&<InlineFeedback kind={health.ready?'warning':'error'}>{health.error}<Button variant="outline" onClick={health.retry}>重新读取服务状态</Button></InlineFeedback>}{health.ready&&<><div className="replica-service-health"><Server size={22}/><div><h3>工作区连接</h3><p>{stateLabel(health.extra.result?.status,'服务已返回检查结果')}</p></div></div><JsonDetails value={health.extra.result} title="完整连接检查记录"/></>}
 <div className="replica-task-toolbar"><Button variant="outline" onClick={()=>view.test('看图理解')}>测试已保存视觉配置</Button><Button variant="outline" onClick={()=>view.test('图片（ComfyUI）')}>测试已保存图片配置</Button><Button variant="outline" onClick={()=>view.test('高级')}>测试已保存辅助决策配置</Button></div>{view.testResult.value&&<InlineFeedback kind={view.testKind.value}>{view.testResult.value}</InlineFeedback>}{view.testRaw.value&&<JsonDetails value={view.testRaw.value} title="连接测试的完整记录"/>}</Surface>}
 {taskId&&<TaskRecordDialog taskId={taskId} returnTo={returnTo||undefined} returnFocus={()=>taskTrigger.current} onClose={()=>navigate({taskId:null})}/>}
 </div>;
}

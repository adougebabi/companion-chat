import {useEffect,useRef} from 'react';
import {Activity,ArrowLeft,CheckCircle2,Clock3,RefreshCw,Search,AlertCircle,Loader2,Pause} from 'lucide-react';
import {Button} from '@/components/ui/button';
import {Tabs,TabsList,TabsTrigger} from '@/components/ui/tabs';
import {Input} from '@/components/ui/input';
import {Surface,ChoiceSelect} from '../controls';
import {useWorkspace} from '../page';
import useWorkspaceRead from './useWorkspaceRead';
import useManagementView from './useManagementView';
import useWorkspaceHash from './useWorkspaceHash';
import TaskRecordDialog from './TaskRecordDialog';
import DiagnosticCreationContext from './DiagnosticCreationContext';
import {EmptyState,InlineFeedback} from './Presentation';
import LoadingState from './LoadingState';
import ServiceDiagnostics from './ServiceDiagnostics';
import DiagnosticStatus from './DiagnosticStatus';
import '../../styles/replica-diagnostics.css';
import {domainName,stateLabel} from '@/v3/domain-presentation.mjs';
import {diagnosticDateLabel,taskDisplayTitle,taskStatusLabel,taskTypeLabel} from '@/approved-ui/task-presentation.mjs';
export default function DiagnosticsWorkspace({view,children}){
 const taskTrigger=useRef(null),w=useWorkspace(),hash=useWorkspaceHash(),params=new URLSearchParams(hash.split('?')[1]||''),requested=params.get('view'),taskId=params.get('taskId'),correlationId=params.get('correlationId')||'',returnTo=params.get('returnTo'),back=/^#?\/(?!\/)/.test(returnTo||'')?returnTo.replace(/^#/,''):'',initialization=back.startsWith('/library/create');
 const [preferences,setPreferences]=useManagementView('diagnostics',{mode:['tasks','calls','health'].includes(requested)?requested:params.has('correlationId')?'calls':'tasks',query:'',status:'',kind:''});
 const {mode,query,status,kind}=preferences,setMode=mode=>setPreferences({mode}),setQuery=query=>setPreferences({query}),setStatus=status=>setPreferences({status}),setKind=kind=>setPreferences({kind});
 useEffect(()=>{if(['tasks','calls','health'].includes(requested))setMode(requested);},[hash]);
 const tasks=useWorkspaceRead({identity:['diagnostic-tasks',correlationId],characterScoped:false,enabled:mode==='tasks',keyOf:row=>row.ref,read:cursor=>w.services.diagnosticsClient.page('workflow-intents',{limit:20,...(correlationId?{correlationId}:{}),...(cursor?{cursor}:{})})});
 const characterName=row=>w.state.characters.find(c=>c.id===row.data?.fluctlight_id)?.name;
 const taskKind=row=>row.data?.kind||row.data?.intent_type||'';
 const rows=tasks.items.filter(row=>(!status||row.data?.status===status)&&(!kind||taskKind(row)===kind)&&(!query.trim()||JSON.stringify([row.data?.title,taskKind(row),characterName(row),row.data?.fluctlight_id,row.ref]).toLowerCase().includes(query.trim().toLowerCase())));
 const failures=tasks.items.filter(r=>['failed','dead_letter','paused'].includes(r.data?.status)).length,running=tasks.items.filter(r=>['running','processing'].includes(r.data?.status)).length,completed=tasks.items.filter(r=>['completed','succeeded'].includes(r.data?.status)).length;
 const navigate=next=>{const query=new URLSearchParams([...params].filter(([key])=>['view','taskId','correlationId','returnTo'].includes(key)));for(const [key,value] of Object.entries(next)){if(value===null)query.delete(key);else query.set(key,String(value));}w.navigatePath('/diag?'+query);};
 return <div className="replica-diagnostics"><div className="replica-settings-head"><div><h2>诊断与任务</h2><p>查看执行到了哪里，在需要时继续原任务。</p></div><span className="replica-diagnostic-environment">实际运行记录</span></div>
 {(mode==='tasks'||mode==='health')&&<DiagnosticCreationContext returnTo={returnTo||''}/>}
 {back&&<Surface className="replica-diagnostic-context"><div><h3>{initialization?'新相识 · 初始化任务':'来自业务页面的任务记录'}</h3><p>已有资料和原任务会保留，配置与恢复操作在实际业务入口完成。</p></div><Button variant="outline" onClick={()=>w.navigatePath(back)}><ArrowLeft size={14}/>{initialization?'返回初始化流程':'返回来源页'}</Button></Surface>}
 <Tabs value={mode} onValueChange={value=>{setMode(value);navigate({view:value,taskId:null});}}><TabsList variant="line"><TabsTrigger value="tasks">任务中心{tasks.ready&&<span className="replica-diagnostic-count" title="已读取任务数">{tasks.items.length}</span>}</TabsTrigger><TabsTrigger value="calls">模型调用记录</TabsTrigger><TabsTrigger value="health">服务诊断</TabsTrigger></TabsList></Tabs>
 {mode==='calls'&&<div className="replica-diagnostic-calls">{children}</div>}
 {mode==='tasks'&&<><div className="replica-data-stats">{[['需要处理',failures,tasks.cursor?'已读取的失败或暂停任务':'失败或暂停的任务'],['正在执行',running,tasks.cursor?'已读取的执行中任务':'以后台实际进度为准'],['已经完成',completed,tasks.cursor?'已读取的完成记录':'可查阅结果与执行记录']].map(([title,count,hint])=><div className="replica-diagnostic-metric" key={title}><span>{title}</span><strong>{tasks.ready?count:'—'}</strong><small>{hint}</small></div>)}</div>
 <Surface className="replica-diagnostic-task-surface"><div className="replica-task-toolbar"><div className="replica-diagnostic-search"><Search size={14}/><Input aria-label="搜索任务或角色" placeholder="搜索任务或角色…" value={query} onChange={e=>setQuery(e.target.value)}/><Button variant="ghost" onClick={tasks.load} disabled={tasks.busy} aria-label="重新读取运行任务"><RefreshCw size={15}/></Button></div><div className="replica-diagnostic-filters"><ChoiceSelect aria-label="任务状态" value={status} onChange={e=>setStatus(e.target.value)}><option value="">所有状态</option>{[...new Set(tasks.items.map(r=>r.data?.status).filter(Boolean))].map(value=><option value={value} key={value}>{stateLabel(value)}</option>)}</ChoiceSelect><ChoiceSelect aria-label="任务类型" value={kind} onChange={e=>setKind(e.target.value)}><option value="">所有类型</option>{[...new Set(tasks.items.map(taskKind).filter(Boolean))].map(value=><option value={value} key={value}>{taskTypeLabel({kind:value})}</option>)}</ChoiceSelect></div></div>
 {tasks.busy&&<LoadingState label="正在读取运行任务…"/>}{tasks.error&&<InlineFeedback kind={tasks.ready?'warning':'error'}>{tasks.error}<Button variant="outline" onClick={tasks.retry}>重新读取原任务页</Button></InlineFeedback>}
 <div className="replica-task-list">{rows.map(row=>{const id=row.data?.task_id||row.data?.intent_id||row.data?.id,name=characterName(row);return <button key={row.ref} className="replica-task-row" onClick={event=>{taskTrigger.current=event.currentTarget;if(id)navigate({view:'tasks',taskId:id});else w.navigatePath('/diag/record/workflow-intents/'+encodeURIComponent(row.ref));}}><span className="replica-task-icon">{['failed','dead_letter','error'].includes(row.data?.status)?<AlertCircle size={16}/>:['running','processing'].includes(row.data?.status)?<Loader2 size={16} className="animate-spin"/>:row.data?.status==='paused'?<Pause size={16}/>:['completed','succeeded'].includes(row.data?.status)?<CheckCircle2 size={16}/>:<Clock3 size={16}/>}</span><div><h3>{taskDisplayTitle({...row.data,kind:taskKind(row)},w.state.characters)}</h3><p>{[taskTypeLabel({kind:taskKind(row)}),name,diagnosticDateLabel(row.occurred_at||row.data?.created_at)].filter(Boolean).join(' · ')}</p></div><DiagnosticStatus status={row.data?.status} label={taskStatusLabel(row.data?.status)}/></button>;})}</div>
 {tasks.ready&&!rows.length&&<EmptyState><h3>{query||status||kind?'没有符合筛选的任务':'暂时没有运行任务'}</h3><p>实际任务记录会出现在这里；没有记录不代表业务从未发生。</p>{(query||status||kind)&&<Button variant="outline" onClick={()=>setPreferences({query:'',status:'',kind:''})}>清除筛选</Button>}</EmptyState>}{tasks.cursor&&<Button variant="outline" disabled={tasks.busy} onClick={tasks.more}>读取更多任务</Button>}</Surface><Button variant="link" className="replica-diagnostic-secondary-entry" onClick={view.openTaskDiagnostics}>查看完整待处理事项与来源</Button></>}
 {mode==='health'&&<ServiceDiagnostics view={view}/>}
 {taskId&&<TaskRecordDialog taskId={taskId} returnTo={returnTo||undefined} returnFocus={()=>taskTrigger.current} onClose={()=>navigate({taskId:null})}/>}
 </div>;
}

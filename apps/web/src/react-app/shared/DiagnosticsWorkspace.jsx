import {useEffect,useRef,useState} from 'react';
import {Activity,ArrowLeft,CheckCircle2,Clock3,RefreshCw,Search,AlertCircle,Loader2} from 'lucide-react';
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
import {diagnosticDateLabel,taskDisplayTitle,taskFailureReason,taskStatusLabel,taskTypeLabel,taskStatusPresentation,taskStatusGroups,taskStatusSummary,worldEventClockLabel,readAttentionTasks} from '@/approved-ui/task-presentation.mjs';
export default function DiagnosticsWorkspace({view,children}){
 const taskTrigger=useRef(null),w=useWorkspace(),hash=useWorkspaceHash(),params=new URLSearchParams(hash.split('?')[1]||''),requested=params.get('view'),taskId=params.get('taskId'),correlationId=params.get('correlationId')||'',returnTo=params.get('returnTo'),back=/^#?\/(?!\/)/.test(returnTo||'')?returnTo.replace(/^#/,''):'',initialization=back.startsWith('/library/create');
 const [preferences,setPreferences]=useManagementView('diagnostics',{mode:['tasks','calls','health'].includes(requested)?requested:params.has('correlationId')?'calls':'tasks',query:'',status:'',kind:'',world:''});
 const {mode,query,status,kind,world}=preferences,setMode=mode=>setPreferences({mode}),setQuery=query=>setPreferences({query}),setStatus=status=>setPreferences({status}),setKind=kind=>setPreferences({kind}),setWorld=world=>setPreferences({world});
 const [now,setNow]=useState(Date.now);
 useEffect(()=>{const timer=setInterval(()=>{if(!document.hidden)setNow(Date.now());},30000);return()=>clearInterval(timer);},[]);
 useEffect(()=>{if(['tasks','calls','health'].includes(requested))setMode(requested);},[hash]);
 const tasks=useWorkspaceRead({identity:['diagnostic-tasks',correlationId,kind,world],characterScoped:false,enabled:mode==='tasks',keyOf:row=>row.ref,read:cursor=>w.services.diagnosticsClient.page('workflow-intents',{limit:20,...(kind?{intentType:kind}:{}),...(world?{worldId:world}:{}),...(correlationId?{correlationId}:{}),...(cursor?{cursor}:{})})});
 // Permanent clocks predate most recent tasks. Read them independently so a
 // busy character cannot push world work out of the initial task list.
 const worldTasks=useWorkspaceRead({identity:['diagnostic-world-tasks',correlationId,world],characterScoped:false,enabled:mode==='tasks'&&!kind,keyOf:row=>row.ref,read:cursor=>w.services.diagnosticsClient.page('workflow-intents',{limit:20,intentType:'world.events.generate',...(world?{worldId:world}:{}),...(correlationId?{correlationId}:{}),...(cursor?{cursor}:{})})});
 const attentionAvailable=typeof w.services.operationsClient?.pending==='function';
 const attentionTasks=useWorkspaceRead({identity:['diagnostic-attention-tasks'],characterScoped:false,enabled:mode==='tasks'&&attentionAvailable,keyOf:row=>row.ref,read:()=>readAttentionTasks(w.services.operationsClient)});
 const taskIdentity=row=>row.data?.task_id||row.data?.intent_id||row.ref;
 const taskKind=row=>row.data?.kind||row.data?.intent_type||'';
 const taskTime=row=>{const occurred=Date.parse(row.occurred_at||'');return Number.isFinite(occurred)?occurred:Date.parse(row.data?.created_at||'');};
 const newestFirst=(left,right)=>{const first=taskTime(left),second=taskTime(right);if(Number.isFinite(first)!==Number.isFinite(second))return Number.isFinite(second)?1:-1;if(Number.isFinite(first)&&first!==second)return second-first;return String(taskIdentity(left)).localeCompare(String(taskIdentity(right)));};
 const recentItems=[...(kind?[]:worldTasks.items),...tasks.items];
 // The pending authority suppresses obsolete failures and adds older unresolved
 // work. The row projection and every summary then consume this same set.
 const retained=attentionAvailable&&attentionTasks.ready?recentItems.filter(row=>taskStatusPresentation({...row.data,kind:row.data?.kind||row.data?.intent_type}).group!=='attention'):recentItems;
 const mergedTasks=new Map();for(const row of [...retained,...attentionTasks.items]){const key=taskIdentity(row),previous=mergedTasks.get(key);mergedTasks.set(key,previous?{...previous,...row,data:{...previous.data,...row.data}}:row);}
 const mergedItems=[...mergedTasks.values()];
 // Permanent world clocks may stay pinned at the head; every other task is
 // globally ordered by the same creation time shown in its row.
 const taskItems=kind?mergedItems.sort(newestFirst):[
  ...mergedItems.filter(row=>taskKind(row)==='world.events.generate').sort(newestFirst),
  ...mergedItems.filter(row=>taskKind(row)!=='world.events.generate').sort(newestFirst)
 ],hasMoreTasks=!!(tasks.cursor||(!kind&&worldTasks.cursor));
 const taskWorld=row=>row.data?.world_id||row.data?.payload?.world_id||row.data?.source?.world_id||'';
 const worldOptions=useRef(new Map());
 if(worldOptions.current.owner!==w.state.session?.actorId){worldOptions.current=new Map();worldOptions.current.owner=w.state.session?.actorId;}
 for(const row of taskItems)if(taskWorld(row))worldOptions.current.set(taskWorld(row),row.data?.world_name||worldOptions.current.get(taskWorld(row))||'世界名称待读取');
 const characterName=row=>w.state.characters.find(c=>c.id===(row.data?.fluctlight_id||row.data?.payload?.fluctlight_id))?.name;
 const projected=taskItems.map(row=>({...row,presentation:taskStatusPresentation({...row.data,occurred_at:row.occurred_at,kind:taskKind(row)},now)}));
 const selectedStatusGroup=taskStatusGroups.some(group=>group.id===status)?status:status?taskStatusPresentation({status}).group:'';
 const rows=projected.filter(row=>(!correlationId||!row.sourceHref||row.data?.correlation_id===correlationId)&&(!selectedStatusGroup||row.presentation.group===selectedStatusGroup)&&(!kind||taskKind(row)===kind)&&(!world||taskWorld(row)===world)&&(!query.trim()||JSON.stringify([row.data?.title,taskKind(row),taskTypeLabel({kind:taskKind(row)}),row.data?.world_name,characterName(row),row.data?.fluctlight_id,row.ref,row.presentation.group==='attention'?taskFailureReason(row.data):'']).toLowerCase().includes(query.trim().toLowerCase())));
 const summary=taskStatusSummary(rows),tasksReady=tasks.ready&&(kind||worldTasks.ready)&&(!attentionAvailable||attentionTasks.ready);
 const navigate=next=>{const query=new URLSearchParams([...params].filter(([key])=>['view','taskId','correlationId','returnTo'].includes(key)));for(const [key,value] of Object.entries(next)){if(value===null)query.delete(key);else query.set(key,String(value));}w.navigatePath('/diag?'+query);};
 return <div className="replica-diagnostics"><div className="replica-management-section-header"><div className="replica-settings-head"><div><h2>诊断与任务</h2><p>查看执行到了哪里，在需要时继续原任务。</p></div><span className="replica-diagnostic-environment">实际运行记录</span></div>
 <Tabs value={mode} onValueChange={value=>{setMode(value);navigate({view:value,taskId:null});}}><TabsList variant="line"><TabsTrigger value="tasks">任务中心{tasksReady&&<span className="replica-diagnostic-count" title="当前筛选下已读取任务数">{rows.length}</span>}</TabsTrigger><TabsTrigger value="calls">模型调用记录</TabsTrigger><TabsTrigger value="health">服务诊断</TabsTrigger></TabsList></Tabs></div>
 {(mode==='tasks'||mode==='health')&&<DiagnosticCreationContext returnTo={returnTo||''}/>}
 {back&&<Surface className="replica-diagnostic-context"><div><h3>{initialization?'新相识 · 初始化任务':'来自业务页面的任务记录'}</h3><p>已有资料和原任务会保留，配置与恢复操作在实际业务入口完成。</p></div><Button variant="outline" onClick={()=>w.navigatePath(back)}><ArrowLeft size={14}/>{initialization?'返回初始化流程':'返回来源页'}</Button></Surface>}
 {mode==='calls'&&<div className="replica-diagnostic-calls">{children}</div>}
 {mode==='tasks'&&<><div className="replica-data-stats">{summary.map(({id,label,count,hint})=><div className="replica-diagnostic-metric" data-task-status-group={id} key={id}><span>{label}</span><strong>{tasksReady?count:'—'}</strong><small>{id==='attention'&&attentionAvailable?'与通知服务异常同源 · ':hasMoreTasks?'已读取 · ':''}{hint}</small></div>)}</div>
 <Surface className="replica-diagnostic-task-surface"><div className="replica-task-toolbar"><div className="replica-diagnostic-search"><Search size={14}/><Input aria-label="搜索任务或角色" placeholder="搜索任务或角色…" value={query} onChange={e=>setQuery(e.target.value)}/><Button variant="ghost" size="icon" onClick={()=>Promise.all([tasks.load(),...(!kind?[worldTasks.load()]:[]),...(attentionAvailable?[attentionTasks.load()]:[])])} disabled={tasks.busy||worldTasks.busy} aria-label="重新读取运行任务"><RefreshCw size={15}/></Button></div><div className="replica-diagnostic-filters"><ChoiceSelect aria-label="任务状态" value={selectedStatusGroup} onChange={e=>setStatus(e.target.value)}><option value="">所有状态</option>{taskStatusGroups.map(group=><option value={group.id} key={group.id}>{group.filterLabel||group.label}</option>)}</ChoiceSelect><ChoiceSelect aria-label="任务类型" value={kind} onChange={e=>setKind(e.target.value)}><option value="">所有类型</option>{[...new Set(['world.events.generate',kind,...taskItems.map(taskKind)].filter(Boolean))].map(value=><option value={value} key={value}>{taskTypeLabel({kind:value})}</option>)}</ChoiceSelect><ChoiceSelect aria-label="任务世界" value={world} onChange={e=>setWorld(e.target.value)}><option value="">所有世界</option>{[...worldOptions.current].map(([id,name])=><option value={id} key={id}>{name}</option>)}</ChoiceSelect></div></div>
 {attentionTasks.busy&&<LoadingState label="正在核对需要处理的任务…"/>}{attentionTasks.error&&<InlineFeedback kind="warning">{attentionTasks.error}<Button variant="outline" onClick={attentionTasks.retry}>重新读取需要处理的任务</Button></InlineFeedback>}
 {attentionTasks.extra.detail_errors>0&&<InlineFeedback kind="warning">部分任务详情暂未读到，已保留待处理来源与数量。<Button variant="outline" onClick={attentionTasks.load}>重新读取待处理任务详情</Button></InlineFeedback>}
 {tasks.busy&&<LoadingState label="正在读取运行任务…"/>}{tasks.error&&<InlineFeedback kind={tasks.ready?'warning':'error'}>{tasks.error}<Button variant="outline" onClick={tasks.retry}>重新读取原任务页</Button></InlineFeedback>}
 {!kind&&<>{worldTasks.busy&&<LoadingState label="正在读取世界事件任务…"/>}{worldTasks.error&&<InlineFeedback kind="warning">{worldTasks.error}<Button variant="outline" onClick={worldTasks.retry}>重新读取世界事件任务</Button></InlineFeedback>}{worldTasks.cursor&&<Button variant="outline" disabled={worldTasks.busy} onClick={worldTasks.more}>读取更多世界事件任务</Button>}</>}
 <div className="replica-task-list">{rows.map(row=>{const id=row.data?.task_id||row.data?.intent_id||row.data?.id,title=taskDisplayTitle({...row.data,kind:taskKind(row)},w.state.characters),state=row.presentation,metadata=[taskTypeLabel({kind:taskKind(row)}),characterName(row),row.data?.world_name,taskKind(row)==='world.events.generate'?worldEventClockLabel(row.occurred_at||row.data?.created_at,undefined,true):diagnosticDateLabel(row.occurred_at||row.data?.created_at),...(row.data?.last_cycle_status?[`上一轮：${taskStatusLabel(row.data.last_cycle_status)}`,`下一轮：${diagnosticDateLabel(row.data.next_cycle_at)}`]:[]),row.data?.task_detail_error?'任务详情暂未读到':'',state.warning].filter(value=>value&&!title.split(' · ').includes(value)),failure=state.group==='attention'?taskFailureReason(row.data):'',attempts=Number.isInteger(row.data?.runs)?row.data.runs:Number.isInteger(row.data?.attempt_count)?row.data.attempt_count:null;return <button key={row.ref} className="replica-task-row" data-task-status-group={state.group} onClick={event=>{taskTrigger.current=event.currentTarget;if(row.sourceHref&&!row.sourceHref.includes('/diag/task/'))w.navigatePath(row.sourceHref.replace(/^#/,''));else if(id)navigate({view:'tasks',taskId:id});else w.navigatePath('/diag/record/workflow-intents/'+encodeURIComponent(row.ref));}}><span className="replica-task-icon">{state.stale||['attention','unknown'].includes(state.group)?<AlertCircle size={16}/>:state.group==='running'?<Loader2 size={16} className="animate-spin"/>:state.group==='completed'?<CheckCircle2 size={16}/>:<Clock3 size={16}/>}</span><div><h3>{title}</h3><p>{metadata.join(' · ')}</p>{failure&&<p className="replica-task-failure-summary">{[attempts!==null&&attempts>0?`已尝试 ${attempts} 次`:'',`失败原因：${failure}`].filter(Boolean).join(' · ')}</p>}</div><DiagnosticStatus status={state.status} tone={state.tone} label={state.label}/></button>;})}</div>
 {tasksReady&&!rows.length&&<EmptyState><h3>{query||status||kind||world?'没有符合筛选的任务':'暂时没有运行任务'}</h3><p>实际任务记录会出现在这里；没有记录不代表业务从未发生。</p>{(query||status||kind||world)?<Button variant="outline" onClick={()=>setPreferences({query:'',status:'',kind:'',world:''})}>清除筛选</Button>:<><p>可以先创建一个角色，准备过程中的任务会在这里显示。</p><Button asChild><a href="#/library/create">去创建角色</a></Button></>}</EmptyState>}{tasks.cursor&&<Button variant="outline" disabled={tasks.busy} onClick={tasks.more}>读取更多任务</Button>}</Surface><Button variant="link" className="replica-diagnostic-secondary-entry" onClick={view.openTaskDiagnostics}>查看完整待处理事项与来源</Button></>}
 {mode==='health'&&<ServiceDiagnostics view={view}/>}
 {taskId&&<TaskRecordDialog taskId={taskId} returnTo={returnTo||undefined} returnFocus={()=>taskTrigger.current} onClose={()=>navigate({taskId:null})} presentation="dialog"/>}
 </div>;
}

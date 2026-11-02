import {domainName,errorMessage,stateLabel} from '../v3/domain-presentation.mjs';
// Every task belongs to exactly one group. Counts, filters and row presentation
// consume this projection together; an unrecognized backend state stays visible.
export const taskStatusGroups=Object.freeze([
 {id:'attention',label:'需要处理',hint:'失败、暂停或受阻的任务'},
 {id:'running',label:'正在执行',hint:'已开始和执行中的任务'},
 {id:'waiting',label:'等待中',hint:'排队、等待重试或下一轮的任务'},
 {id:'completed',label:'已经完成',filterLabel:'已完成',hint:'已完成的任务记录'},
 {id:'ended',label:'已经结束',hint:'取消、跳过或被替代的任务'},
 {id:'unknown',label:'状态待确认',hint:'未记录或尚未识别的任务状态'},
]);
const statusGroups={
 attention:['failed','dead_letter','deadletter','error','paused','blocked','timed_out','timeout'],
 running:['started','running','processing','generating'],
 waiting:['pending','queued','waiting','retry','manual_prepared','awaiting_kev'],
 completed:['completed','succeeded','success','admitted','no_event','applied'],
 ended:['cancelled','canceled','terminated','superseded','skipped','interrupted','expired','rejected','continued_as_new'],
};
const statusGroupByState=new Map(Object.entries(statusGroups).flatMap(([group,states])=>states.map(state=>[state,group])));
export const taskRunningStaleAfterMs=15*60*1000;
export function taskStatusPresentation(task={},now=Date.now()){
 const rawStatus=typeof task.status==='string'?task.status:'',kind=task.kind||task.intent_type||task.type;
 let status=rawStatus;
 if(kind==='world.events.generate'&&['started','running','processing','generating','waiting'].includes(status)){
  const cycle=task.payload?.cycle_status||task.source?.last_cycle_status||task.last_cycle_status;
  if(cycle==='generating')status='generating';
  else if(cycle&&(task.payload?.cycle_finished_at||task.source?.next_cycle_at||task.next_cycle_at))status='waiting';
  else if(!['waiting','generating'].includes(status))status='unknown'; // Workflow lifetime alone does not prove hourly generation.
 }
 const group=statusGroupByState.get(status)||'unknown';
 const started=Date.parse(kind==='world.events.generate'?(task.payload?.cycle_started_at||task.source?.cycle_started_at||''):(task.started_at||task.updated_at||task.created_at||''));
 const stale=group==='running'&&Number.isFinite(started)&&now-started>taskRunningStaleAfterMs;
 const statusLabel=taskStatusLabel(status),groupLabel={attention:'需要处理',running:'进行中',waiting:'等待开始',completed:'已完成',ended:'已经结束'};
 const label=group==='unknown'?'状态待确认':status==='generating'?'生成中':status==='waiting'&&kind!=='world.events.generate'?'等待中':statusLabel==='状态待确认'?groupLabel[group]:statusLabel;
 /** @type {'neutral'|'green'|'amber'|'red'|'blue'} */
 const tone=stale?'amber':({attention:'red',running:'blue',completed:'green',unknown:'amber'})[group]||'neutral';
 return {status,rawStatus,group,label,tone,stale,warning:stale?'运行超过 15 分钟，请核对执行记录':''};
}
export function taskStatusSummary(rows=[]){
 return taskStatusGroups.map(group=>({...group,count:rows.filter(row=>row.presentation.group===group.id).length}));
}
export function taskStatusLabel(status){
 const labels={waiting:'等待下一小时',generating:'正在生成',started:'进行中',admitted:'已通过审核',no_event:'本小时无事件',skipped:'已跳过',interrupted:'已中断',expired:'已过期',awaiting_kev:'等待审核',rejected:'未采纳',pending:'等待开始',queued:'等待开始',running:'进行中',processing:'进行中',paused:'已暂停',completed:'已完成',succeeded:'已完成',failed:'需要处理',dead_letter:'需要处理',cancelled:'已取消'};
 if(Object.hasOwn(labels,status))return labels[status];
 const fallback=stateLabel(status);return typeof fallback==='string'?fallback:'状态待确认';
}
export function taskDisplayTitle(task,characters=[]){
 if(!task)return '任务详情';
 if(typeof task.title==='string'&&task.title&&task.title!==task.kind&&task.title!==task.type)return task.title;
 const character=characters.find(row=>row.id===task.fluctlight_id)?.name;
 return [character,domainName(task.kind||task.type||task.title,'后台任务')].filter(Boolean).join(' · ');
}
export function diagnosticInstant(value){
 if(typeof value!=='string'||!Number.isFinite(Date.parse(value)))return '未记录时间';
 const parts=Object.fromEntries(new Intl.DateTimeFormat('en-CA',{timeZone:'Asia/Shanghai',year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',second:'2-digit',fractionalSecondDigits:3,hourCycle:'h23'}).formatToParts(new Date(value)).map(p=>[p.type,p.value]));
 return `${parts.year}-${parts.month}-${parts.day} ${parts.hour}:${parts.minute}:${parts.second}.${parts.fractionalSecond}（北京时间）`;
}
function diagnosticParts(value){
 if(typeof value!=='string'||!Number.isFinite(Date.parse(value)))return null;
 return Object.fromEntries(new Intl.DateTimeFormat('en-CA',{timeZone:'Asia/Shanghai',year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',second:'2-digit',fractionalSecondDigits:3,hourCycle:'h23'}).formatToParts(new Date(value)).map(part=>[part.type,part.value]));
}
export function diagnosticDateLabel(value,now=Date.now()){
 const parts=diagnosticParts(value);if(!parts)return '未记录时间';
 const today=diagnosticParts(new Date(now).toISOString());
 const sameDay=['year','month','day'].every(key=>parts[key]===today[key]);
 const date=sameDay?'今天':parts.year===today.year?`${parts.month}-${parts.day}`:`${parts.year}-${parts.month}-${parts.day}`;
 return `${date} ${parts.hour}:${parts.minute}`;
}
export function diagnosticTimeLabel(value){const parts=diagnosticParts(value);return parts?`${parts.hour}:${parts.minute}:${parts.second}${parts.fractionalSecond==='000'?'':'.'+parts.fractionalSecond}`:'未记录时间';}
export function taskEventTitle(entry){
 const event=domainName(entry.event_type,'');
 if(event)return event;
 const stage=domainName(entry.name,'');
 if(stage)return stage+' · '+stateLabel(entry.status,'状态记录');
 return stateLabel(entry.status,'任务状态')+'记录';
}
export function taskEventText(entry){
 const title=taskEventTitle(entry),state=stateLabel(entry.status,'已记录');
 const label=typeof title==='string'?title:'任务记录',status=typeof state==='string'?state:'已记录';
 return label.split(' · ').includes(status)||label===status+'记录'?label:label+' · '+status;
}
export function taskStageRecords(task,preparation){
 if(!['wardrobe.initialize','wardrobe_initialize','wardrobe_preparation'].includes(task?.kind||task?.type)||!task?.fluctlight_id||preparation?.task_id!==task?.task_id||preparation?.fluctlight_id!==task.fluctlight_id)return [];
 const committed=preparation?.result?.inventory_committed===true,failed=['failed','dead_letter'].includes(preparation.status),cancelled=['cancelled','canceled'].includes(preparation.status);
 const source={task_id:preparation.task_id,fluctlight_id:preparation.fluctlight_id,status:preparation.status,result:preparation.result};
 return [{id:'inventory',name:'整理衣物与穿搭方案',status:committed?'completed':cancelled?'cancelled':failed?'failed':['generating','running'].includes(preparation.status)?'running':'pending',source},{id:'wear',name:'确认初始穿着',status:preparation.status==='completed'&&committed?'completed':failed&&committed?'failed':cancelled&&committed?'cancelled':failed||cancelled?'blocked':preparation.status==='awaiting_wear'&&committed?'running':'pending',source}];
}
export function taskNeedsModelConfiguration(task,preparation){
 const code=preparation?.error_code||task?.error_code;
 if(['wardrobe_preparation_context_failed','wardrobe_provider_assignment_unavailable'].includes(code))return false;
 if(['initialization_role_unconfigured','wardrobe_provider_unconfigured'].includes(code))return true;
 const cause=[task?.source?.last_error,task?.last_error].filter(Boolean).join(' ');
 return /initialization_role_unconfigured|wardrobe_provider_unconfigured|provider role [\w.-]+ preflight failed|provider role [\w.-]+ unavailable:[^\n]*no rows in result set/.test(cause);
}
export function taskFailureReason(task,preparation){
 const current=preparation?.task_id===task?.task_id?preparation:null;
 const raw=task?.source?.last_error||task?.last_error;
 if(['wardrobe_preparation_context_failed','wardrobe_provider_assignment_unavailable','wardrobe_provider_unconfigured'].includes(current?.error_code)&&current?.result?.failure_reason)return current.result.failure_reason;
 if(taskNeedsModelConfiguration(task,current)){const reason=errorMessage({code:current?.error_code,message:raw}).replace('原资料与已完成步骤会保留。','已有角色、世界与准备记录会保留。');return reason.includes('已有角色、世界')?reason:reason+' 已有角色、世界与准备记录会保留。';}
 if(current?.result?.failure_reason)return current.result.failure_reason;
 const fallback='本次任务未完成，原资料和已确认的结果仍保留。请查看执行记录与业务恢复入口。';
 // A legacy assignment-read wrapper can contain a DB/connection failure. Do not
 // let the general message formatter turn it back into an unassigned-model claim.
 if(/provider role [\w.-]+ unavailable:/.test(String(raw||'')))return fallback;
 return errorMessage(raw,fallback);
}
export function taskSourceHref(task){
 if(!task?.fluctlight_id)return '';
 if(['wardrobe.initialize','wardrobe_initialize','wardrobe_preparation'].includes(task.kind||task.type))return '#/c/'+encodeURIComponent(task.fluctlight_id)+'/life/wardrobe';
 return '';
}

export function taskTypeLabel(task){const label=domainName(task?.kind||task?.type,'其他后台任务');return typeof label==='string'?label:'其他后台任务';}
export function taskSubtitle(task){return [taskTypeLabel(task),taskStatusPresentation(task||{}).label,diagnosticDateLabel(task?.created_at)].join(' · ');}
export function taskEventReason(entry){
 const reason=typeof entry.reason==='string'?entry.reason:'';
 const code=typeof entry.reason_code==='string'?entry.reason_code:'';
 if(!reason&&!code)return '';
 const fallback='';
 const message=errorMessage({message:reason,code},fallback);return typeof message==='string'?message:fallback;
}
export function taskRecoveryStrategy(snapshot,recovery){
 if(recovery?.mode==='domain_recovery')return '通过原业务入口核对准备进度并处理可恢复的环节，保留已确认结果。';
 const labels={pause:'暂停',resume:'继续执行',cancel:'请求取消',restart:'重试原任务',reset:'从已完成检查点重新执行'};
 const actions=(snapshot?.allowed_actions||[]).filter(action=>Object.hasOwn(labels,action)).map(action=>labels[action]);
 return actions.length?'当前支持'+actions.join('、')+'；操作受理后仍需读取实际任务结果。':'当前没有可用的通用恢复操作，可重新读取原任务状态。';
}
export function taskSourceLabel(task){return ['pending','queued'].includes(task?.status)&&(!Number.isInteger(task?.runs)||task.runs===0)?'查看初始衣柜准备':'查看初始衣柜与恢复';}

// Physical model records own rounds. Lifecycle events and dispatcher attempts do
// not carry enough information to reconstruct model calls or their outcomes.
export function taskRoundRecords(records=[]){
 return records.map((record,index)=>({record,call:record.data||{},index})).sort((a,b)=>{
  const start=Date.parse(a.call.started_at||a.record.occurred_at),end=Date.parse(b.call.started_at||b.record.occurred_at);
  return Number.isFinite(start)&&Number.isFinite(end)?start-end||a.index-b.index:a.index-b.index;
 }).map(({record,call},index)=>({record,call,label:Number.isInteger(call.sequence)&&call.sequence>=0?`第 ${call.sequence} 轮`:`调用记录 ${index+1}`,status:['completed','succeeded','success'].includes(call.status)?'success':['failed','error','dead_letter','timeout'].includes(call.status)?'failure':'other'}));
}

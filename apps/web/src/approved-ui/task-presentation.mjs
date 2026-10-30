import {domainName,errorMessage,stateLabel} from '../v3/domain-presentation.mjs';
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
export function taskEventTitle(entry){
 const event=domainName(entry.event_type,'');
 if(event)return event;
 const stage=domainName(entry.name,'');
 if(stage)return stage+' · '+stateLabel(entry.status,'状态记录');
 return stateLabel(entry.status,'任务状态')+'记录';
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

import {domainName,errorMessage,stateLabel} from '../v3/domain-presentation.mjs';
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
 if(['wardrobe.initialize','wardrobe_initialize','wardrobe_preparation'].includes(task?.kind||task?.type)&&preparation?.task_id===task?.task_id&&typeof preparation?.inventory_committed==='boolean'&&!(preparation.status==='completed'&&!preparation.inventory_committed)&&(!preparation.fluctlight_id||preparation.fluctlight_id===task.fluctlight_id)){
  const committed=preparation.inventory_committed,failed=['failed','dead_letter'].includes(preparation.status),cancelled=['cancelled','canceled'].includes(preparation.status),source={task_id:preparation.task_id,preparation_id:preparation.id,status:preparation.status,inventory_committed:committed};
  return [{id:'inventory',name:'整理衣物与穿搭方案',status:committed?'completed':cancelled?'cancelled':failed?'failed':['generating','running'].includes(preparation.status)?'running':'pending',source},{id:'readback',name:'核对准备结果',status:preparation.status==='completed'&&committed?'completed':cancelled&&committed?'cancelled':failed&&committed?'failed':failed||cancelled?'blocked':preparation.status==='awaiting_wear'?'running':'pending',source}];
 }
 const stages=new Map();
 for(const entry of [...(Array.isArray(task?.steps)?task.steps:[])].sort((a,b)=>(Date.parse(a.at)||0)-(Date.parse(b.at)||0))){
  if(typeof entry.name==='string'&&entry.name.trim())stages.set(entry.name,entry);
 }
 return [...stages.values()];
}
export function taskNeedsModelConfiguration(task,preparation){
 return /initialization_role_unconfigured|wardrobe_provider_unconfigured|provider role [\w.-]+ (?:unavailable|preflight failed)/.test([task?.source?.last_error,task?.last_error,preparation?.error_code].filter(Boolean).join(' '));
}
export function taskFailureReason(task,preparation){
 const current=preparation?.task_id===task?.task_id?preparation:null;
 const raw=task?.source?.last_error||task?.last_error;
 if(taskNeedsModelConfiguration(task,current)){const reason=errorMessage({code:current?.error_code,message:raw}).replace('原资料与已完成步骤会保留。','已有角色、世界与准备记录会保留。');return reason.includes('已有角色、世界')?reason:reason+' 已有角色、世界与准备记录会保留。';}
 if(current?.result?.failure_reason)return current.result.failure_reason;
 return errorMessage(raw,'本次任务未完成，原资料和已确认的结果仍保留。请查看执行记录与业务恢复入口。');
}
export function taskSourceHref(task){
 if(!task?.fluctlight_id)return '';
 if(['wardrobe.initialize','wardrobe_initialize','wardrobe_preparation'].includes(task.kind||task.type))return '#/c/'+encodeURIComponent(task.fluctlight_id)+'/life/wardrobe';
 return '';
}

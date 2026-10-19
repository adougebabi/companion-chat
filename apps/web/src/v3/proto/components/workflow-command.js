import {diagnosticsClient} from '../services.js';
import {esc} from '../ui.js';
import {Field} from './forms.js';
import {durableCommandDrawer} from './durable-command-drawer.js';
export const workflowActionLabels={pause:'暂停',resume:'恢复',cancel:'请求取消',restart:'重启',reset:'重置执行'};
export function workflowCommand(snapshot,action,{point,onSaved=()=>{}}={}){
 const wf=snapshot.intent.workflow_id,run=snapshot.runtime?.run_id;
 return durableCommandDrawer({slot:`workflow-control:${wf}`,title:workflowActionLabels[action]+'工作流',body:`<p>${esc(snapshot.control_semantics)}</p><p class="small muted">工作流 ${esc(wf)} · Run ${esc(run)} · 任务版本 ${esc(snapshot.intent.revision)}${point?` · 历史点 ${esc(point)}`:''}</p>${Field({label:'操作理由',name:'reason',type:'textarea',attrs:{required:true,maxlength:1000}})}`,read:el=>({action,body:{sourceRunId:run,expectedRevision:snapshot.intent.revision,reason:el.querySelector('[name=reason]').value,...(action==='reset'?{historyPoint:point}:{})}}),execute:(value,key)=>diagnosticsClient.control(wf,value.action,{...value.body,idempotencyKey:key}),onSaved});
}

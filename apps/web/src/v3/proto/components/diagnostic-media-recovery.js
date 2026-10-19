import {diagnosticsClient} from '../services.js';
import {esc} from '../ui.js';
import {Field} from './forms.js';
import {durableCommandDrawer} from './durable-command-drawer.js';
export function diagnosticMediaRecovery(record,onSaved=()=>{}){
 const d=record.data;
 return durableCommandDrawer({slot:`media-recovery:${d.id}`,preAdmissionConflicts:['revision_conflict','media_result_already_ready','media_not_retryable','media_execution_active'],title:'恢复原媒体任务',body:`<p>原目标 ${esc(d.id)} · v${esc(d.revision)}。恢复会创建真实的新执行尝试并保留原任务来源。排队回执不表示媒体生成或发布完成。</p><p>恢复原阶段保留已经生成的资产、生成服务任务与提示词。明确重新生成才会重新提交生成任务；已有就绪资产不能被覆盖。创作和视觉身份媒体须从各自原任务恢复。</p>${Field({label:'恢复方式',name:'mode',type:'select',value:'resume',options:[{value:'resume',label:'恢复原阶段与已保存结果'},{value:'regenerate',label:'明确重新生成（保留原尝试历史）'}]})}${Field({label:'恢复理由',name:'reason',type:'textarea',attrs:{required:true,maxlength:1000}})}`,read:el=>({expectedRevision:d.revision,mode:el.querySelector('[name=mode]').value,reason:el.querySelector('[name=reason]').value}),execute:(body,key)=>diagnosticsClient.mediaRetry(d.id,{...body,idempotencyKey:key}),onSaved});
}

// Deterministic API-boundary fixtures for the opt-in scenario preview mode
// (VITE_API_SCENARIOS=1, `pnpm dev:mock`). Shapes follow @fluctlight/browser-client
// types; nothing here is reachable from the production bundle.
import type {BrowserDiagnosticRecord,BrowserTask,BrowserWorkflowInspection} from '@fluctlight/browser-client';

export const SCENARIO_MARKER='fluctlight.api-scenarios.v1';
export const scenarioOwner='scenario-owner';
export const scenarioCharacter={id:'fl-scn-linxia',identity:{name:'林夏'},status:'active',unread_count:0,last_conversation_at:'2026-11-24T09:12:00Z'};

const at=(minute:number)=>new Date(Date.UTC(2026,10,24,8,minute,0)).toISOString();
const recent=(minutesAgo:number)=>new Date(Date.now()-minutesAgo*60_000).toISOString();
const longText=(label:string,lines:number)=>Array.from({length:lines},(_,i)=>`${label} 第 ${i+1} 段：林夏回顾了本周与你的对话、她在咖啡馆的排班和下周想去海边写生的计划，并逐条核对已有约定，保留原始措辞与上下文引用（reference-${i+1}）。`).join('\n');

type TaskSeed=BrowserTask&Record<string,unknown>;
function task(id:string,patch:Partial<TaskSeed>):TaskSeed{
 return {task_id:id,workflow_id:'go:wf-'+id,fluctlight_id:scenarioCharacter.id,kind:'goal.plan',type:'goal.plan',title:'',state:'',status:'queued',runs:0,steps:[],has_more_steps:false,created_at:at(0),started_at:null,completed_at:null,next_attempt_at:null,correlation_id:'corr-'+id,source:{},...patch} as TaskSeed;
}
const step=(id:string,event_type:string,status:string,minute:number,reason_code='')=>({id,event_type,name:event_type,status,reason_code,at:at(minute)});

export type CallSeed={sequence:number;status:string;minute:number;seconds:number;prompt?:unknown;response?:unknown;error?:string;error_code?:string;stage?:string};
export type ScenarioDataset={tasks:Record<string,TaskSeed>;calls:Record<string,CallSeed[]>;workflows:Record<string,BrowserWorkflowInspection>;media:Record<string,BrowserDiagnosticRecord[]>};

/** A fresh, mutable copy per page load: refresh/actions mutate it, a reload restores it. */
export function createDataset():ScenarioDataset{
 const tasks:Record<string,TaskSeed>={
  queued:task('queued',{status:'queued',title:'独立目标规划',steps:[step('s1','intent.accepted','queued',0)]}),
  // Relative to page load so it never trips the 15-minute staleness warning.
  running:task('running',{status:'running',runs:1,created_at:recent(3),started_at:recent(2),steps:[{...step('s1','intent.accepted','queued',0),at:recent(3)},{...step('s2','workflow.started','running',2),at:recent(2)}]}),
  succeeded:task('succeeded',{status:'completed',runs:1,started_at:at(2),completed_at:at(5),steps:[step('s1','intent.accepted','queued',0),step('s2','workflow.started','running',2),step('s3','workflow.completed','completed',5)]}),
  failed:task('failed',{status:'failed',runs:3,started_at:at(2),completed_at:at(9),last_error:'provider HTTP 503: upstream overloaded',error_code:'provider_unavailable',steps:[step('s1','intent.accepted','queued',0),step('s2','workflow.started','running',2),step('s3','workflow.retry','retry',5,'provider_unavailable'),step('s4','workflow.failed','failed',9,'provider_unavailable')]}),
  'no-calls':task('no-calls',{status:'failed',runs:1,started_at:at(1),completed_at:at(1),last_error:'目标资料缺失，规划未开始。',error_code:'goal_missing',steps:[step('s1','intent.accepted','queued',0),step('s2','workflow.failed','failed',1,'goal_missing')]}),
  media:task('media',{kind:'media.generation',type:'media.generation',status:'running',runs:1,created_at:recent(4),started_at:recent(3),steps:[{...step('s1','intent.accepted','queued',0),at:recent(4)},{...step('s2','workflow.started','running',0),at:recent(3)}]}),
  long:task('long',{status:'failed',runs:1,started_at:at(2),completed_at:at(4),last_error:longText('模型返回无法解析的结构',6),error_code:'schema_mismatch',steps:[step('s1','intent.accepted','queued',0),step('s2','workflow.failed','failed',4,'schema_mismatch')]}),
 };
 const call=(sequence:number,status:string,minute:number,seconds:number,extra:Partial<CallSeed>={}):CallSeed=>({sequence,status,minute,seconds,prompt:{messages:[{role:'system',content:'你是林夏的目标规划器。'},{role:'user',content:'请为本周目标给出三步计划。'}]},response:status==='completed'?{content:'1. 周二完成写生选点；2. 周四约你一起试拍；3. 周末整理作品。'}:undefined,stage:status==='completed'?'final_response':'failed',...extra});
 const calls:Record<string,CallSeed[]>={
  'corr-running':[call(1,'completed',2,3.2)],
  'corr-succeeded':[call(1,'completed',2,2.4),call(2,'completed',4,1.8)],
  'corr-failed':[call(1,'completed',2,2.1),call(2,'failed',5,30,{error:'provider HTTP 503: upstream overloaded',error_code:'provider_unavailable'}),call(3,'failed',9,30,{error:'provider HTTP 503: upstream overloaded',error_code:'provider_unavailable'})],
  'corr-long':[call(1,'failed',3,41.5,{prompt:longText('完整提示词',40),response:longText('模型原始输出',30),error:longText('结构校验错误',8),error_code:'schema_mismatch'})],
 };
 const workflows:Record<string,BrowserWorkflowInspection>={};
 for(const value of Object.values(tasks)){
  const failed=value.status==='failed';
  workflows[value.workflow_id]={intent:{intent_id:value.task_id,workflow_id:value.workflow_id,intent_type:value.kind,status:value.status,revision:1,task_queue:'scenario',payload:value.kind==='media.generation'?{intent_id:'media-1'}:{}},runtime:{run_id:'run-'+value.task_id,status:value.status},runtime_state:'available',allowed_actions:failed?['restart']:['queued','running'].includes(value.status)?['cancel']:[],recovery:{href:'',label:'',mode:'generic',reason:''},control_semantics:'scenario',commands:[],commands_has_more:false};
 }
 const media:Record<string,BrowserDiagnosticRecord[]>={'media:media-1':[{id:'media-1',ref:'media-1',kind:'media-prompts',occurred_at:recent(3),data:{id:'media-1',workflow_id:tasks.media.workflow_id,status:'running',revision:1,provider_prompt:'林夏在海边写生，傍晚逆光，胶片质感',provider_job_id:'job-media-1',prompt:{purpose:'moment'}}}]};
 return {tasks,calls,workflows,media};
}

export function callRecord(correlationId:string,seed:CallSeed,index:number):BrowserDiagnosticRecord{
 const started=at(seed.minute),completed=new Date(Date.parse(started)+seed.seconds*1000).toISOString();
 return {id:`${correlationId}-call-${index+1}`,ref:`${correlationId}-call-${index+1}`,kind:'model-runs',occurred_at:started,data:{correlation_id:correlationId,sequence:seed.sequence,status:seed.status,stage:seed.stage,model_id:'qwen3-32b',role:'goal_planner',started_at:started,completed_at:completed,latency_ms:Math.round(seed.seconds*1000),prompt:seed.prompt,response:seed.response,...(seed.error?{error:seed.error}:{}),...(seed.error_code?{error_code:seed.error_code}:{})}};
}

export type Scenario={id:string;label:string;hash:string;/** fail the first N reads of matching paths */failFirst?:{pattern:RegExp;times:number};delayMs?:number;/** reads of `running` until it completes */runningCompletesAfter?:number};
export const scenarios:Scenario[]=[
 {id:'task-queued',label:'1 排队等待',hash:'#/diag?view=tasks&taskId=queued'},
 {id:'task-running',label:'2/7 执行中 · 调用成功但未完成（刷新两次后完成）',hash:'#/diag?view=tasks&taskId=running',runningCompletesAfter:3},
 {id:'task-succeeded',label:'3 成功',hash:'#/diag?view=tasks&taskId=succeeded'},
 {id:'task-failed',label:'4/6 失败 · 多次调用部分失败',hash:'#/diag?view=tasks&taskId=failed'},
 {id:'task-no-calls',label:'5 失败 · 没有模型调用',hash:'#/diag?view=tasks&taskId=no-calls'},
 {id:'task-media',label:'媒体任务执行中（刷新只读取状态）',hash:'#/diag?view=tasks&taskId=media'},
 {id:'task-long',label:'8 超长输入 / 输出 / 错误',hash:'#/diag?view=tasks&taskId=long'},
 {id:'task-slow',label:'9a 请求加载中（每次读取延迟 2.5 秒）',hash:'#/diag?view=tasks&taskId=failed',delayMs:2500},
 {id:'task-read-error',label:'9b 任务读取失败（第一次），重试后恢复',hash:'#/diag?view=tasks&taskId=failed',failFirst:{pattern:/^\/api\/diagnostics\/tasks\//,times:1}},
 {id:'calls-read-error',label:'9c 模型调用读取失败（第一次），重试后恢复',hash:'#/diag?view=tasks&taskId=failed',failFirst:{pattern:/^\/api\/diagnostic-records\/model-runs\?.*correlationId=corr-failed/,times:1}},
 {id:'calls-failed',label:'诊断 · 多次调用部分失败（调用列表）',hash:'#/diag?view=calls&correlationId=corr-failed'},
 {id:'calls-long',label:'诊断 · 超长调用内容（调用列表）',hash:'#/diag?view=calls&correlationId=corr-long'},
 {id:'task-list',label:'任务中心列表',hash:'#/diag?view=tasks'},
];

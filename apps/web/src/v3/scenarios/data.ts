// Deterministic API-boundary fixtures for the opt-in scenario preview mode
// (VITE_API_SCENARIOS=1, `pnpm dev:mock`). Shapes follow @fluctlight/browser-client
// types; nothing here is reachable from the production bundle.
import type {BrowserDiagnosticRecord,BrowserNotification,BrowserPendingGroup,BrowserTask,BrowserVisualManagement,BrowserVisualSubmissionBody,BrowserWorkflowInspection} from '@fluctlight/browser-client';

export const SCENARIO_MARKER='fluctlight.api-scenarios.v1';
export const scenarioOwner='scenario-owner';
export const scenarioCharacter={id:'fl-scn-linxia',identity:{name:'林夏'},status:'active',unread_count:0,last_conversation_at:'2026-11-24T09:12:00Z'};

const at=(minute:number)=>new Date(Date.UTC(2026,10,24,8,minute,0)).toISOString();
const recent=(minutesAgo:number)=>new Date(Date.now()-minutesAgo*60_000).toISOString();
const longText=(label:string,lines:number)=>Array.from({length:lines},(_,i)=>`${label} 第 ${i+1} 段：林夏回顾了本周与你的对话、她在咖啡馆的排班和下周想去海边写生的计划，并逐条核对已有约定，保留原始措辞与上下文引用（reference-${i+1}）。`).join('\n');

type TaskSeed=BrowserTask&Record<string,unknown>;
// Mirrors apps/core-go/internal/core/task_read.go: title is the kind string, state
// mirrors status, and errors live in source.last_error / source.error_code.
function task(id:string,patch:Partial<TaskSeed>):TaskSeed{
 const kind=String(patch.kind||'goal.plan'),status=String(patch.status||'pending');
 return {task_id:id,workflow_id:'go:wf-'+id,fluctlight_id:scenarioCharacter.id,kind,type:kind,title:kind,state:status,status,runs:0,steps:[],has_more_steps:false,created_at:at(0),started_at:null,completed_at:null,next_attempt_at:null,correlation_id:'corr-'+id,source:{},...patch} as TaskSeed;
}
const step=(id:string,event_type:string,status:string,minute:number,reason_code='')=>({id,event_type,name:event_type,status,reason_code,at:at(minute)});

export type CallSeed={sequence:number;status:string;minute:number;seconds:number;prompt?:unknown;response?:unknown;error_code?:string;error?:string;stage?:string;role?:string};
export type ScenarioDataset={tasks:Record<string,TaskSeed>;calls:Record<string,CallSeed[]>;agentRuns:Record<string,BrowserDiagnosticRecord[]>;workflows:Record<string,BrowserWorkflowInspection>;media:Record<string,BrowserDiagnosticRecord[]>;visual:BrowserVisualManagement;visualResumes:BrowserVisualSubmissionBody[];notifications:BrowserNotification[];pending:BrowserPendingGroup[];readAll:unknown[]};

/** A fresh, mutable copy per page load: refresh/actions mutate it, a reload restores it. */
// Direct conversation for the chat scenario: one completed turn with a reply and
// one failed turn (Core keeps the user's message with turnStatus failed).
export function chatConversation(){
 const owner=scenarioOwner,fid=scenarioCharacter.id,created=at(9);
 const message=(id:string,sequence:number,author:string,kind:string,text:string,minute:number,turn:Record<string,unknown>)=>({id,conversationId:'conv-chat',sequence,authorActorId:author,kind,text,attachmentRefs:[],createdAt:at(minute),...turn});
 return {conversation:{id:'conv-chat',createdByActorId:owner,title:null,revision:3,createdAt:created,updatedAt:at(12)},
  participants:[{conversationId:'conv-chat',actorId:owner,role:'owner',status:'active',joinedAt:created},{conversationId:'conv-chat',actorId:fid,role:'fluctlight',status:'active',joinedAt:created}],
  messages:[
   message('chat-1',1,owner,'user','今天去看展了吗？',10,{turnId:'turn-ok',turnStatus:'completed'}),
   message('chat-2',2,fid,'assistant','去了，人不多，下次一起去。',11,{turnId:'turn-ok'}),
   message('chat-3',3,owner,'user','那周末一起去海边？',12,{turnId:'turn-failed',turnStatus:'failed',turnErrorCode:'provider_unavailable',turnRetryable:true}),
  ],nextBeforeSequence:null};
}

export function createDataset():ScenarioDataset{
 const tasks:Record<string,TaskSeed>={
  queued:task('queued',{status:'pending',steps:[step('s1','intent.accepted','pending',0)]}),
  // Relative to page load so it never trips the 15-minute staleness warning.
  running:task('running',{status:'running',runs:1,created_at:recent(3),started_at:recent(2),steps:[{...step('s1','intent.accepted','queued',0),at:recent(3)},{...step('s2','workflow.started','running',2),at:recent(2)}]}),
  succeeded:task('succeeded',{status:'completed',runs:1,started_at:at(2),completed_at:at(5),steps:[step('s1','intent.accepted','queued',0),step('s2','workflow.started','running',2),step('s3','workflow.completed','completed',5)]}),
  failed:task('failed',{status:'failed',runs:3,started_at:at(2),completed_at:at(9),source:{last_error:'provider HTTP 503: upstream overloaded',error_code:'provider_unavailable'},steps:[step('s1','intent.accepted','queued',0),step('s2','workflow.started','running',2),step('s3','workflow.retry','retry',5,'provider_unavailable'),step('s4','workflow.failed','failed',9,'provider_unavailable')]}),
  'no-calls':task('no-calls',{status:'failed',runs:1,started_at:at(1),completed_at:at(1),source:{last_error:'目标资料缺失，规划未开始。',error_code:'goal_missing'},steps:[step('s1','intent.accepted','queued',0),step('s2','workflow.failed','failed',1,'goal_missing')]}),
  media:task('media',{kind:'media.generation',status:'running',correlation_id:'media:media-1',source:{media_intent_id:'media-1',correlation_id:'media:media-1'},runs:1,created_at:recent(4),started_at:recent(3),steps:[{...step('s1','intent.accepted','queued',0),at:recent(4)},{...step('s2','workflow.started','running',0),at:recent(3)}]}),
  control:task('control',{kind:'platform.control',status:'failed',runs:1,started_at:at(2),completed_at:at(3),source:{last_error:'provider HTTP 503: upstream overloaded',error_code:'provider_unavailable'},steps:[step('s1','intent.accepted','pending',0),step('s2','workflow.started','running',2),step('s3','workflow.failed','failed',3,'provider_unavailable')]}),
  long:task('long',{status:'failed',runs:1,started_at:at(2),completed_at:at(4),source:{last_error:longText('模型返回无法解析的结构',6),error_code:'schema_mismatch'},steps:[step('s1','intent.accepted','queued',0),step('s2','workflow.failed','failed',4,'schema_mismatch')]}),
 };
 const call=(sequence:number,status:string,minute:number,seconds:number,extra:Partial<CallSeed>={}):CallSeed=>({sequence,status,minute,seconds,prompt:{messages:[{role:'system',content:'你是林夏的目标规划器。'},{role:'user',content:'请为本周目标给出三步计划。'}]},response:status==='completed'?{content:'1. 周二完成写生选点；2. 周四约你一起试拍；3. 周末整理作品。'}:undefined,stage:status==='completed'?'final_response':'failed',...extra});
 const calls:Record<string,CallSeed[]>={
  'corr-running':[call(1,'completed',2,3.2)],
  'corr-succeeded':[call(1,'completed',2,2.4),call(2,'completed',4,1.8)],
  'corr-failed':[call(1,'completed',2,2.1),call(2,'failed',5,30,{error_code:'provider_unavailable'}),call(3,'failed',9,30,{error_code:'provider_unavailable'})],
  'corr-long':[call(1,'failed',3,41.5,{prompt:longText('完整提示词',40),response:longText('模型原始输出',30),error_code:'schema_mismatch'})],
  // Chat turns (correlation id turn:<turnId>), read by the chat 本轮详情 drawer.
  'turn:turn-ok':[call(1,'completed',10,2.2,{role:'conversation_cognition',prompt:{messages:[{role:'user',content:'今天去看展了吗？'}]},response:{content:'想先回忆一下今天的安排。'}}),call(2,'completed',10,1.6,{role:'fact_only_reply',prompt:{messages:[{role:'user',content:'今天去看展了吗？'}]},response:{content:'去了，人不多，下次一起去。'}})],
  'turn:turn-failed':[call(1,'completed',12,2.0,{role:'conversation_cognition',prompt:{messages:[{role:'user',content:'那周末一起去海边？'}]},response:{content:'她想答应，但要先看看周末的排班。'}}),call(2,'failed',12,30,{role:'fact_only_reply',prompt:{messages:[{role:'user',content:'那周末一起去海边？'}]},error_code:'provider_unavailable',error:'provider HTTP 503: upstream overloaded'})],
 };
 const agentRun=(turn:string,agent:string,status:string,minute:number,extra:Record<string,unknown>={}):BrowserDiagnosticRecord=>({id:`${turn}-${agent}`,ref:`["run","${scenarioCharacter.id}","${agent}","${turn}-${agent}"]`,kind:'agent-runs',occurred_at:at(minute),data:{fluctlight_id:scenarioCharacter.id,agent_id:agent,run_id:`${turn}-${agent}`,correlation_id:'turn:'+turn,status,source:'agent_runs',started_at:at(minute),...extra}});
 const agentRuns:Record<string,BrowserDiagnosticRecord[]>={
  'turn:turn-ok':[agentRun('turn-ok','conversation_cognition','completed',10)],
  'turn:turn-failed':[agentRun('turn-failed','conversation_cognition','failed',12,{failure_code:'agent_run_failed',failure_stage:'reply'})],
 };
 // Mirrors apps/core-go/internal/core/workflow_controls.go: cancel only while the
 // runtime runs; restart/reset only for platform.control; domain kinds recover
 // through their business entry, everything else through the original task.
 const workflows:Record<string,BrowserWorkflowInspection>={};
 for(const value of Object.values(tasks))workflows[value.workflow_id]=workflowInspection(value);
 const media:Record<string,BrowserDiagnosticRecord[]>={'media:media-1':[{id:'media-1',ref:'media-1',kind:'media-prompts',occurred_at:recent(3),data:{id:'media-1',owner_fluctlight_id:scenarioCharacter.id,fluctlight_id:scenarioCharacter.id,correlation_id:'media:media-1',kind:'image',mime_type:'image/png',workflow_id:tasks.media.workflow_id,status:'pending',revision:1,provider_prompt:'林夏在海边写生，傍晚逆光，胶片质感',provider_job_id:'job-media-1',provider_request_id:'req-media-1',prompt:JSON.stringify({purpose:'moment'}),media_scope:'fluctlight'}}]};
 // Visual identity whose candidate submission was paused with an uncertain
 // provider acceptance: the owner must confirm the same-source image by hand.
 const visual:BrowserVisualManagement={schema_version:'v1',fluctlight_id:scenarioCharacter.id,id:'vi-1',status:'active',current_revision:4,identity_snapshot:{},renderer_constraints:{},generated_reviews:{},adapter_version:'scenario',active_session_id:'vs-1',session:{id:'vs-1',status:'running',last_error:null,current_attempt:2,created_at:at(0),task_id:null,candidate_submission_uncertain:true,attempts:[{id:'va-2',attempt_number:2,status:'running',candidate_asset_id:null,decision:null,brief_revision:3,created_at:at(1),review:null}],submission_recovery:{status:'available',stages:[{stage:'candidate',media_intent_id:'mi-visual-2',revision:5,ready:true}],workflow_id:'go:wf-visual-1',source_run_id:'run-visual-1',workflow_revision:7}}};
 // Inbox: shapes mirror apps/core-go/internal/core/notification_center.go
 // (title/body/link per source_kind) and pending_center.go (all five groups,
 // item id = target_kind:source_id, failed tasks link to #/diag/task/<intent>).
 const root='#/c/'+encodeURIComponent(scenarioCharacter.id);
 const notifications:BrowserNotification[]=[
  {id:'notice-reply',fluctlight_id:scenarioCharacter.id,title:'林夏 · 新回复',body:'有一条已保存的角色回复。',link:root+'/chat?conversation=conv-1&message=msg-9',quiet:false,read_at:null,created_at:recent(5),source_kind:'conversation.reply.published',source_id:'msg-9',source_event_id:'evt-reply-9',source_occurred_at:recent(5)},
  {id:'notice-moment',fluctlight_id:scenarioCharacter.id,title:'新动态',body:'角色发布了一条动态。',link:'#/feed/moment-3',quiet:false,read_at:recent(30),created_at:recent(60),source_kind:'moment.published',source_id:'moment-3',source_event_id:'evt-moment-3',source_occurred_at:recent(60)},
 ];
 const pending:BrowserPendingGroup[]=(['capability','persona_proposal','studio_apply','failed_task','owner_request'] as const).map(kind=>kind==='failed_task'
  ?{kind,status:'ok',count:1,next_cursor:'',items:[{id:'workflow:failed',source_id:'failed',fluctlight_id:scenarioCharacter.id,title:'goal.plan',status:'failed',href:'#/diag/task/failed',created_at:tasks.failed.created_at}]}
  :{kind,status:'ok',count:0,next_cursor:'',items:[]});
 return {tasks,calls,agentRuns,workflows,media,visual,visualResumes:[],notifications,pending,readAll:[]};
}

const recoveryReason='领域恢复保留原结果、attempt、版本和副作用围栏；工作流命令不代表业务已成功';
function workflowInspection(value:TaskSeed):BrowserWorkflowInspection{
 const runtimeStatus=value.status==='pending'?'':value.status==='started'?'running':value.status;
 const actions=[...(['running','paused'].includes(runtimeStatus)?['cancel']:[]),...(value.kind==='platform.control'?(['completed','failed','cancelled','terminated','timed_out'].includes(runtimeStatus)?['restart','reset']:['running','paused'].includes(runtimeStatus)?['reset']:[]):[])];
 const root='/c/'+encodeURIComponent(scenarioCharacter.id);
 const recovery=value.kind==='goal.plan'?{mode:'domain_recovery',href:root+'/life/goals',label:'查看目标、约定和原请求恢复',reason:recoveryReason}
  :value.kind==='media.generation'?{mode:'domain_recovery',href:'/diag/media-prompts?q=media-1',label:'查看媒体原任务和恢复',reason:recoveryReason}
  :{mode:'original_source',href:'/diag/task/'+encodeURIComponent(value.task_id),label:'查看原任务及业务来源',reason:'从原业务请求核对结果；不会通过通用重启绕过已提交的领域结果'};
 const source=value.source as {last_error?:string};
 return {intent:{intent_id:value.task_id,workflow_id:value.workflow_id,intent_type:value.kind,status:value.status,revision:1,task_queue:value.kind==='media.generation'?'media':'lifecycle',payload:value.kind==='media.generation'?{intent_id:'media-1'}:{},...(source.last_error?{last_error:source.last_error}:{})},
  runtime:runtimeStatus?{run_id:'run-'+value.task_id,workflow_id:value.workflow_id,workflow_type:value.kind,task_queue:'lifecycle',status:runtimeStatus,control_state:runtimeStatus==='running'?'running':'',control_actions:[],history_length:12,start_time:value.started_at,close_time:value.completed_at}:null,
  runtime_state:'available',allowed_actions:actions,recovery,control_semantics:'commands are receipts; the task state is read back separately',commands:[],commands_has_more:false} as BrowserWorkflowInspection;
}

export function callRecord(correlationId:string,seed:CallSeed,index:number):BrowserDiagnosticRecord{
 const started=at(seed.minute),completed=new Date(Date.parse(started)+seed.seconds*1000).toISOString();
 return {id:`${correlationId}-call-${index+1}`,ref:`${correlationId}-call-${index+1}`,kind:'model-runs',occurred_at:started,data:{id:`${correlationId}-call-${index+1}`,correlation_id:correlationId,logical_run_id:correlationId+'-run',model_call_id:`${correlationId}-mc-${index+1}`,physical_sequence:seed.sequence,sequence:seed.sequence,round_count:1,status:seed.status,stage:seed.stage,model_id:'qwen3-32b',role:seed.role||'goal_planner',binding_role:seed.role||'goal_planner',fluctlight_id:scenarioCharacter.id,queued_at:started,created_at:started,started_at:started,completed_at:completed,latency_ms:Math.round(seed.seconds*1000),prompt:seed.prompt,response:seed.response,...(seed.error_code?{error_code:seed.error_code}:{}),...(seed.error?{error:seed.error}:{})}};
}

export type Scenario={id:string;label:string;hash:string;/** fail the first N reads of matching paths */failFirst?:{pattern:RegExp;times:number};delayMs?:number;/** reads of `running` until it completes */runningCompletesAfter?:number};
export const scenarios:Scenario[]=[
 {id:'task-queued',label:'1 已受理未开始（pending）',hash:'#/diag?view=tasks&taskId=queued'},
 {id:'task-running',label:'2/7 执行中 · 调用成功但未完成（刷新两次后完成）',hash:'#/diag?view=tasks&taskId=running',runningCompletesAfter:3},
 {id:'task-succeeded',label:'3 成功',hash:'#/diag?view=tasks&taskId=succeeded'},
 {id:'task-failed',label:'4/6 失败 · 多次调用部分失败',hash:'#/diag?view=tasks&taskId=failed'},
 {id:'task-control-failed',label:'失败 · 可重试的运行时控制任务（platform.control）',hash:'#/diag?view=tasks&taskId=control'},
 {id:'task-no-calls',label:'5 失败 · 没有模型调用',hash:'#/diag?view=tasks&taskId=no-calls'},
 {id:'task-media',label:'媒体任务执行中（刷新只读取状态）',hash:'#/diag?view=tasks&taskId=media'},
 {id:'task-long',label:'8 超长输入 / 输出 / 错误',hash:'#/diag?view=tasks&taskId=long'},
 {id:'task-slow',label:'9a 请求加载中（每次读取延迟 2.5 秒）',hash:'#/diag?view=tasks&taskId=failed',delayMs:2500},
 {id:'task-read-error',label:'9b 任务读取失败（第一次），重试后恢复',hash:'#/diag?view=tasks&taskId=failed',failFirst:{pattern:/^\/api\/diagnostics\/tasks\//,times:1}},
 {id:'calls-read-error',label:'9c 模型调用读取失败（第一次），重试后恢复',hash:'#/diag?view=tasks&taskId=failed',failFirst:{pattern:/^\/api\/diagnostic-records\/model-runs\?.*correlationId=corr-failed/,times:1}},
 {id:'calls-failed',label:'诊断 · 多次调用部分失败（调用列表）',hash:'#/diag?view=calls&correlationId=corr-failed'},
 {id:'calls-long',label:'诊断 · 超长调用内容（调用列表）',hash:'#/diag?view=calls&correlationId=corr-long'},
 {id:'task-list',label:'任务中心列表',hash:'#/diag?view=tasks'},
 {id:'visual-submission-review',label:'视觉身份 · 候选图接单待人工核对',hash:'#/media?tab=identity&fid='+scenarioCharacter.id},
 {id:'inbox',label:'通知与待处理 · 一条未读回复与一项失败任务',hash:'#/inbox'},
 {id:'chat-turn-details',label:'聊天 · 一轮成功、一轮失败（本轮详情）',hash:'#/c/'+encodeURIComponent(scenarioCharacter.id)+'/chat'},
];

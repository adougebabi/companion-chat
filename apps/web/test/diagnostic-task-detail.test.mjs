import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {act,mountReact,installDOM} from './helpers/react-dom.mjs';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();env.restore();});
const button=(host,text)=>[...host.querySelectorAll('button')].find(node=>node.textContent.trim()===text);
async function tab(view,label){await act(async()=>[...document.querySelectorAll('[role=tab]')].find(node=>node.textContent.trim().startsWith(label)).dispatchEvent(new window.MouseEvent('mousedown',{bubbles:true,button:0})));await view.flush();}
function workspace(){const state=proxy({session:{actorId:'diagnostic-task-owner'},characters:[{id:'A',name:'原角色'}]});return {state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),navigatePath(){},services:{}};}
test('task status labels never expose inherited object properties for unknown wire values',async()=>{
 const {taskStatusLabel,taskEventText,taskRecoveryStrategy,taskTypeLabel}=await vite.ssrLoadModule('/src/approved-ui/task-presentation.mjs');
 for(const status of ['constructor','toString','__proto__','hasOwnProperty'])assert.equal(taskStatusLabel(status),'状态待确认');
 assert.equal(taskStatusLabel('running'),'进行中');assert.equal(taskStatusLabel('实际扩展状态'),'实际扩展状态');
 assert.equal(taskEventText({event_type:'lifecycle.wardrobe.started',status:'started'}),'穿搭准备 · 已开始');
 assert.equal(taskEventText({event_type:'lifecycle.wardrobe.failed',status:'failed'}),'穿搭准备 · 执行失败');
 assert.equal(taskEventText({event_type:'lifecycle.wardrobe.started',status:'running'}),'穿搭准备 · 已开始 · 执行中');
 assert.equal(taskEventText({status:'queued'}),'排队中记录');
 for(const action of ['constructor','toString','__proto__'])assert.equal(taskRecoveryStrategy({allowed_actions:[action]}),'当前没有可用的通用恢复操作，可重新读取原任务状态。');
 assert.match(taskRecoveryStrategy({allowed_actions:['constructor','restart']}),/^当前支持重试原任务；/);
 for(const kind of ['constructor','toString','__proto__'])assert.equal(taskTypeLabel({kind}),'其他后台任务');
});
for(const presentation of ['dialog','sheet'])test(`${presentation} task recovery is owned once and portaled outside its scrolling body across tabs`,async()=>{
 const {default:Record}=await vite.ssrLoadModule('/src/react-app/shared/TaskRecordDialog.tsx'),w=workspace();let workflowReads=0,modelReads=0;const writes=[];
 const original={task_id:'T-'+presentation,kind:'persona_compile',fluctlight_id:'A',correlation_id:'real correlation',workflow_id:'W-'+presentation,status:'failed',runs:2,steps:[{id:'later',event_type:'failed',status:'failed',at:'2026-10-30T01:02:00Z'},{id:'earlier',event_type:'queued',status:'queued',at:'2026-10-30T01:00:00Z'}],has_more_steps:true};
 w.services.operationsClient={task:async()=>original};w.services.diagnosticsClient={workflow:async()=>{workflowReads++;return {intent:{status:'failed',revision:7},runtime_state:'available',runtime:{run_id:'original-run',status:'failed'},allowed_actions:['restart'],commands:[]};},page:async()=>{modelReads++;return {items:[],next_cursor:''};},control:async(id,action,body)=>{writes.push({id,action,body:structuredClone(body)});if(writes.length===1)throw Error('fetch failed');return {status:'accepted'};}};
 const view=await mountReact(vite,Record,{workspace:w,props:{taskId:original.task_id,presentation,onClose(){},returnTo:'#/library/create?resume=original'}});
 try{
  const outer=document.querySelector('[role=dialog]'),body=outer.querySelector(presentation==='sheet'?'.creation-task-sheet-body':'.replica-dialog-body'),footer=outer.querySelector(`[data-slot=${presentation==='sheet'?'sheet':'dialog'}-footer]`);
  assert.ok(footer);assert.equal(body.contains(footer),false);assert.equal(outer.querySelector('[role=tab][data-state=active]').textContent,'执行阶段');assert.match(outer.textContent,/原角色 · 已尝试 2 次/);assert.equal(outer.querySelector('[role=progressbar]'),null);assert.ok(button(footer,'重试任务'));assert.equal(body.querySelector('[data-slot=card]'),null);
  await tab(view,'执行记录');assert.match(outer.textContent,/生命周期记录 · 已读取 2 条/);await view.click(button(outer,'生命周期记录 · 已读取 2 条'));assert.deepEqual([...outer.querySelectorAll('.replica-task-history time')].map(node=>node.dateTime),['2026-10-30T01:00:00Z','2026-10-30T01:02:00Z']);const events=[...outer.querySelectorAll('.replica-task-history time')].map(node=>node.textContent);assert.ok(events[0].includes('09:00:00'));assert.ok(events[1].includes('09:02:00'));
  await tab(view,'任务信息');const link=[...outer.querySelectorAll('a')].find(node=>node.textContent.trim()==='查看关联模型调用');assert.ok(link);assert.equal(link.getAttribute('target'),null);assert.ok(outer.querySelector('[role=tabpanel][data-state=active]').contains(link));const route=new URLSearchParams(link.hash.split('?')[1]);assert.equal(route.get('correlationId'),'real correlation');assert.match(route.get('returnTo'),/returnTo=/);await tab(view,'执行阶段');assert.equal(workflowReads,1);assert.equal(modelReads,1);
  const trigger=button(footer,'重试任务');await view.click(trigger);let confirmation=[...document.querySelectorAll('[role=dialog]')].at(-1);await view.input(confirmation.querySelector('textarea'),'原任务恢复');await view.click(button(confirmation,'取消'));await view.flush();assert.equal(document.activeElement,trigger);
  await view.click(trigger);confirmation=[...document.querySelectorAll('[role=dialog]')].at(-1);await view.input(confirmation.querySelector('textarea'),'原任务恢复');await view.click(button(confirmation,'确认重试任务'));assert.equal(writes.length,1);await view.click(confirmation.querySelector('[data-slot=dialog-close]'));await view.click(button(footer,'核对或重试原提交'));assert.equal(writes.length,2);assert.deepEqual(writes[1],writes[0]);assert.match(footer.textContent,/受理不表示执行完成/);assert.equal(button(footer,'重试任务').disabled,true);
 }finally{await view.dispose();}
});
test('task checkpoint reset uses the actual completed history point and keeps its confirmation locked while submitting',async()=>{
 const {default:Record}=await vite.ssrLoadModule('/src/react-app/shared/TaskRecordDialog.tsx'),w=workspace(),writes=[];let finish,historyReads=0;
 w.services.operationsClient={task:async()=>({task_id:'reset-task',kind:'persona_compile',workflow_id:'reset-workflow',status:'failed',steps:[]})};
 w.services.diagnosticsClient={workflow:async()=>({intent:{revision:12,status:'failed'},runtime_state:'available',runtime:{run_id:'reset-run',status:'failed'},allowed_actions:['reset'],commands:[]}),history:async(id,run,cursor)=>{historyReads++;assert.deepEqual([id,run,cursor],['reset-workflow','reset-run','']);return {items:[{id:42,resettable:true,type:'ActivityTaskCompleted',at:'2026-10-30T01:00:00Z',details:{kept:true}},{id:43,resettable:false,type:'ActivityTaskScheduled',details:{}}],next_cursor:''};},control:async(id,action,body)=>{writes.push({id,action,body});await new Promise(resolve=>finish=resolve);return {status:'accepted'};}};
 const view=await mountReact(vite,Record,{workspace:w,props:{taskId:'reset-task',onClose(){}}});
 try{
  const outer=document.querySelector('[role=dialog]'),footer=outer.querySelector('[data-slot=sheet-footer]');assert.equal(button(footer,'从检查点重新执行').disabled,true);await tab(view,'任务信息');await view.click(button(outer,'更多任务记录'));await view.click(button(outer,'执行历史与检查点'));await view.click(button(outer,'读取执行历史'));assert.equal(historyReads,1);
  const select=outer.querySelector('[aria-label="重置检查点"]');assert.ok(select);await view.click(select);const option=[...document.querySelectorAll('[role=option]')].find(node=>node.textContent.includes('事件 42'));assert.ok(option);assert.equal([...document.querySelectorAll('[role=option]')].some(node=>node.textContent.includes('事件 43')),false);await view.click(option);await tab(view,'执行阶段');assert.equal(button(footer,'从检查点重新执行').disabled,false);await view.click(button(footer,'从检查点重新执行'));
  const confirmation=[...document.querySelectorAll('[role=dialog]')].at(-1);await view.input(confirmation.querySelector('textarea'),'核对后的检查点');await view.click(button(confirmation,'确认从检查点重新执行'));assert.equal(writes.length,1);assert.equal(writes[0].body.historyPoint,42);assert.equal(writes[0].body.sourceRunId,'reset-run');assert.equal(writes[0].body.expectedRevision,12);assert.equal(confirmation.querySelector('[data-slot=dialog-close]'),null);assert.equal(button(confirmation,'取消').disabled,true);await act(async()=>finish());await view.flush();assert.match(footer.textContent,/受理不表示执行完成/);
 }finally{finish?.();await view.dispose();}
});

test('readable task title and subtitle keep exact IDs and raw recovery semantics in technical details',async()=>{
 const {default:Record}=await vite.ssrLoadModule('/src/react-app/shared/TaskRecordDialog.tsx'),w=workspace();
 const raw={task_id:'persona_compile_exact_internal_id',kind:'persona.compile',workflow_id:'raw-workflow',status:'failed',runs:3,created_at:'2026-10-30T01:00:00Z',steps:[{id:'exact-event-id',name:'persona_compile',status:'failed',reason_code:'unknown_failure_code',extra:{falseValue:false,zero:0}}]};
 w.services.operationsClient={task:async()=>raw};w.services.diagnosticsClient={workflow:async()=>({intent:{revision:1,status:'failed'},runtime_state:'available',runtime:{run_id:'raw-run',status:'failed'},control_semantics:'Activity retry runs in Runtime',allowed_actions:[],commands:[]})};
 const v=await mountReact(vite,Record,{workspace:w,props:{taskId:raw.task_id,onClose(){}}});
 try{const outer=document.querySelector('[role=dialog]'),description=outer.querySelector('[data-slot=sheet-description]');assert.match(outer.querySelector('[data-slot=sheet-title]').textContent,/人格资料整理/);assert.match(description.textContent,/人格资料整理 · 需要处理/);assert.doesNotMatch(description.textContent,/persona_compile_exact_internal_id/);
 await tab(v,'执行记录');assert.doesNotMatch(outer.textContent,/unknown_failure_code|exact-event-id|本条记录未提供可识别/);await v.click(button(outer,'生命周期记录 · 1 条'));assert.equal(button(outer,'生命周期原始记录').getAttribute('aria-expanded'),'false');await v.click(button(outer,'生命周期原始记录'));assert.match(outer.textContent,/exact-event-id/);assert.match(outer.textContent,/unknown_failure_code/);
 await tab(v,'任务信息');assert.doesNotMatch(outer.querySelector('.replica-task-times[data-continued]+div dl').textContent,/Activity|Runtime/);await v.click(button(outer,'更多任务记录'));await v.click(button(outer,'完整任务信息与关联来源'));assert.match(outer.textContent,/persona_compile_exact_internal_id/);await v.click(button(outer,'管理命令与技术详情'));assert.match(outer.textContent,/Activity retry runs in Runtime/);
 }finally{await v.dispose();}
});
test('task reasons translate only known causes, unknown types stay readable and pending source links do not claim recovery',async()=>{
 const {taskEventReason,taskTypeLabel,taskSourceLabel}=await vite.ssrLoadModule('/src/approved-ui/task-presentation.mjs');
 assert.match(taskEventReason({reason_code:'initialization_role_unconfigured'}),/通用模型/);assert.match(taskEventReason({reason:'已保留真实原因'}),/已保留真实原因/);assert.equal(taskEventReason({reason:'unrecognized raw cause'}),'');assert.equal(taskTypeLabel({kind:'unknown.internal'}),'其他后台任务');assert.equal(taskSourceLabel({status:'queued',runs:0}),'查看初始衣柜准备');
});

test('queued unattempted task uses a preparation link while retaining the backend domain target',async()=>{
 const {default:Record}=await vite.ssrLoadModule('/src/react-app/shared/TaskRecordDialog.tsx'),w=workspace();let destination='';w.navigatePath=path=>destination=path;
 w.services.operationsClient={task:async()=>({task_id:'queued-original',kind:'wardrobe.initialize',fluctlight_id:'A',workflow_id:'queued-workflow',status:'queued',runs:0,steps:[]})};
 w.services.diagnosticsClient={workflow:async()=>({intent:{status:'queued'},runtime_state:'unavailable',allowed_actions:[],commands:[],recovery:{mode:'domain_recovery',href:'/c/A/life/wardrobe',label:'恢复初始衣柜准备'}})};
 const v=await mountReact(vite,Record,{workspace:w,props:{taskId:'queued-original',onClose(){}}});
 try{const outer=document.querySelector('[role=dialog]');assert.match(outer.textContent,/已尝试 0 次/);assert.equal(button(outer,'恢复初始衣柜准备'),undefined);await v.click(button(outer,'查看初始衣柜准备'));assert.equal(destination,'/c/A/life/wardrobe');}finally{await v.dispose();}
});

test('failed media task recovers the original attempt and follows the accepted new task',async()=>{
 const {default:Record}=await vite.ssrLoadModule('/src/react-app/shared/TaskRecordDialog.tsx'),w=workspace(),writes=[],reads=[];
 const media={id:'media-original',owner_fluctlight_id:'A',workflow_id:'media-workflow',status:'failed',revision:8,prompt:'{"purpose":"chat"}',provider_prompt:'saved prompt',provider_job_id:'provider-job',execution:{status:'failed'}};
 w.services.operationsClient={task:async id=>({task_id:id,kind:'media.generation',fluctlight_id:'A',workflow_id:id==='new-media-task'?'new-media-workflow':'media-workflow',status:id==='new-media-task'?'pending':'failed',runs:1,steps:[]})};
 w.services.diagnosticsClient={workflow:async id=>({intent:{intent_type:'media.generation',revision:4,status:id==='media-workflow'?'failed':'pending',payload:{intent_id:'media-original',fluctlight_id:'A'}},runtime_state:'available',runtime:{run_id:'run-'+id,status:id==='media-workflow'?'failed':'running'},allowed_actions:[],commands:[],recovery:{mode:'domain_recovery',href:'/diag/media-prompts?q=media-original',label:'查看媒体原任务和恢复'}}),page:async(kind,options)=>{reads.push({kind,options});return {items:kind==='media-prompts'?[{id:media.id,ref:'media-ref',data:{...media}}]:[],next_cursor:''};},mediaRetry:async(id,body)=>{writes.push({id,body:structuredClone(body)});if(writes.length===1)throw Error('fetch failed');media.status='pending';media.workflow_id='new-media-workflow';media.revision++;return {media_intent_id:id,intent_id:'new-media-task',workflow_id:'new-media-workflow',status:'retry_queued',mode:body.mode};}};
 const view=await mountReact(vite,Record,{workspace:w,props:{taskId:'original-media-task',onClose(){}}});
 try{
  const outer=document.querySelector('[role=dialog]'),footer=outer.querySelector('[data-slot=sheet-footer]');assert.ok(reads.some(r=>r.kind==='media-prompts'&&r.options.correlationId==='media:media-original'));
  assert.ok(button(footer,'从失败处重试'));assert.ok(button(footer,'重新生成'));assert.match(outer.textContent,/保存生成提示词/);assert.match(outer.textContent,/提交生成请求/);
  await view.click(button(footer,'从失败处重试'));let dialog=[...document.querySelectorAll('[role=dialog]')].at(-1);await view.input(dialog.querySelector('textarea'),'恢复原来的生成任务');await view.click(button(dialog,'确认从失败处重试'));assert.equal(writes.length,1);assert.equal(writes[0].id,'media-original');assert.equal(writes[0].body.expectedRevision,8);assert.equal(writes[0].body.mode,'resume');assert.ok(writes[0].body.idempotencyKey);
  await view.click(dialog.querySelector('[data-slot=dialog-close]'));await view.click(button(footer,'核对或重试原提交'));assert.equal(writes.length,2);assert.deepEqual(writes[1],writes[0]);await view.flush();assert.match(outer.textContent,/等待开始|排队中/);assert.equal(button(footer,'从失败处重试'),undefined);
 }finally{await view.dispose();}
});

test('media retry uses explicit regenerate mode and keeps its original revision',async()=>{
 const {default:Record}=await vite.ssrLoadModule('/src/react-app/shared/TaskRecordDialog.tsx'),w=workspace(),writes=[];
 const media={id:'media-regenerate',owner_fluctlight_id:'A',workflow_id:'wf-regenerate',status:'failed',revision:12,prompt:'{"purpose":"chat"}',provider_prompt:'saved'};
 w.services.operationsClient={task:async id=>({task_id:id,kind:'media.generation',fluctlight_id:'A',workflow_id:id==='next-regenerate'?'wf-next':'wf-regenerate',status:id==='next-regenerate'?'pending':'failed',steps:[]})};
 w.services.diagnosticsClient={workflow:async id=>({intent:{intent_type:'media.generation',status:id==='wf-next'?'pending':'failed',payload:{intent_id:media.id}},runtime_state:'available',runtime:{run_id:'run',status:'failed'},allowed_actions:[],commands:[]}),page:async(kind)=>({items:kind==='media-prompts'?[{id:media.id,ref:'ref',data:media}]:[],next_cursor:''}),mediaRetry:async(id,body)=>{writes.push({id,body});media.status='pending';media.workflow_id='wf-next';return {media_intent_id:id,intent_id:'next-regenerate',workflow_id:'wf-next',status:'retry_queued',mode:body.mode};}};
 const view=await mountReact(vite,Record,{workspace:w,props:{taskId:'task-regenerate',onClose(){}}});
 try{const footer=document.querySelector('[data-slot=sheet-footer]');await view.click(button(footer,'重新生成'));const dialog=[...document.querySelectorAll('[role=dialog]')].at(-1);assert.match(dialog.textContent,/已就绪的资产不会被覆盖/);await view.input(dialog.querySelector('textarea'),'明确重新生成');await view.click(button(dialog,'确认重新生成'));assert.equal(writes.length,1);assert.deepEqual([writes[0].id,writes[0].body.mode,writes[0].body.expectedRevision],['media-regenerate','regenerate',12]);assert.ok(writes[0].body.idempotencyKey);}finally{await view.dispose();}
});

for(const kind of ['visual','studio','superseded','active'])test(`media recovery respects ${kind} domain and attempt boundaries`,async()=>{
 const {default:Record}=await vite.ssrLoadModule('/src/react-app/shared/TaskRecordDialog.tsx'),w=workspace();let writes=0,destination='';w.navigatePath=path=>destination=path;
 const id='media-'+kind,media={id,owner_fluctlight_id:'A',workflow_id:kind==='superseded'?'wf-new':'wf-'+kind,status:kind==='active'?'running':'failed',revision:3,prompt:kind==='visual'?'{"purpose":"visual_identity"}':'{"purpose":"chat"}',studio_job_id:kind==='studio'?'studio-job':null,execution:{intent_id:'new-attempt'}};
 w.services.operationsClient={task:async task=>({task_id:task,kind:'media.generation',fluctlight_id:'A',workflow_id:'wf-'+kind,status:'failed',steps:[]})};
 w.services.studioClient={job:async()=>({resource_id:'studio-resource'})};
 w.services.diagnosticsClient={workflow:async()=>({intent:{intent_type:'media.generation',status:'failed',payload:{intent_id:id}},runtime_state:'available',runtime:{run_id:'run',status:'failed'},allowed_actions:[],commands:[]}),page:async()=>({items:[{id,ref:'ref-'+id,data:media}],next_cursor:''}),mediaRetry:async()=>{writes++;return {};}};
 const view=await mountReact(vite,Record,{workspace:w,props:{taskId:'task-'+kind,onClose(){}}});
 try{const outer=document.querySelector('[role=dialog]'),footer=outer.querySelector('[data-slot=sheet-footer]');assert.equal(button(footer,'从失败处重试'),undefined);assert.equal(button(footer,'重新生成'),undefined);if(kind==='visual'){await view.click(button(footer,'查看视觉身份会话'));assert.equal(destination,'/c/A/media/identity/edit');}if(kind==='studio'){await view.click(button(footer,'查看创作任务'));assert.equal(destination,'/studio/preview?res=studio-resource&job=studio-job');}if(kind==='superseded')assert.ok(button(footer,'查看当前执行任务'));if(kind==='active')assert.match(footer.textContent,/媒体任务正在执行或等待/);assert.equal(writes,0);}finally{await view.dispose();}
});

test('media stage display uses only persisted checkpoints and never turns provider admission into a finished image',async()=>{
 const {mediaTaskStageRecords}=await vite.ssrLoadModule('/src/approved-ui/task-presentation.mjs');
 const task={kind:'media.generation',workflow_id:'original'},base={id:'media',workflow_id:'original',status:'failed'};
 assert.deepEqual(mediaTaskStageRecords(task,base).map(x=>[x.name,x.status]),[['媒体任务结果','failed']]);
 const stages=mediaTaskStageRecords(task,{...base,provider_prompt:'saved',provider_job_id:'job',quality_checked_at:'2026-11-06T00:00:00Z',quality_verdict:'skip'});
 assert.deepEqual(stages.map(x=>x.name),['保存生成提示词','提交生成请求','记录质量检查结果','媒体任务结果']);assert.match(stages[1].detail,/结果仍需核对/);assert.match(stages[2].detail,/未执行或不可用/);assert.equal(mediaTaskStageRecords(task,{...base,workflow_id:'new'}).length,0);
});

test('media recovery survives remount after an unknown transport result without changing the submission',async()=>{
 const {default:Record}=await vite.ssrLoadModule('/src/react-app/shared/TaskRecordDialog.tsx'),w=workspace(),writes=[];
 const media={id:'media-remount',owner_fluctlight_id:'A',workflow_id:'wf-remount',status:'failed',revision:7,prompt:'{}'};
 w.services.operationsClient={task:async id=>({task_id:id,kind:'media.generation',fluctlight_id:'A',workflow_id:id==='next-remount'?'wf-next-remount':'wf-remount',status:'failed',steps:[]})};
 w.services.diagnosticsClient={workflow:async id=>({intent:{intent_type:'media.generation',status:'failed',payload:{intent_id:media.id}},runtime_state:'available',runtime:{run_id:id,status:'failed'},allowed_actions:[],commands:[]}),page:async()=>({items:[{id:media.id,ref:'ref-remount',data:media}],next_cursor:''}),mediaRetry:async(id,body)=>{writes.push({id,body:structuredClone(body)});if(writes.length===1)throw Error('fetch failed');return {media_intent_id:id,intent_id:'next-remount',workflow_id:'wf-next-remount',status:'retry_queued',mode:body.mode};}};
 let view=await mountReact(vite,Record,{workspace:w,props:{taskId:'task-remount',onClose(){}}});
 try{let footer=document.querySelector('[data-slot=sheet-footer]');await view.click(button(footer,'从失败处重试'));const dialog=[...document.querySelectorAll('[role=dialog]')].at(-1);await view.input(dialog.querySelector('textarea'),'保留原始提交');await view.click(button(dialog,'确认从失败处重试'));assert.equal(writes.length,1);await view.dispose();view=await mountReact(vite,Record,{workspace:w,props:{taskId:'task-remount',onClose(){}}});footer=document.querySelector('[data-slot=sheet-footer]');assert.equal(button(footer,'从失败处重试'),undefined);assert.ok(button(footer,'核对或重试原提交'));await view.click(button(footer,'核对或重试原提交'));assert.equal(writes.length,2);assert.deepEqual(writes[1],writes[0]);}finally{await view.dispose();}
});

test('media revision conflict requires a fresh read and explicit release before a new submission',async()=>{
 const {default:Record}=await vite.ssrLoadModule('/src/react-app/shared/TaskRecordDialog.tsx'),w=workspace(),writes=[];let revision=4,mediaStatus='failed';
 const id='media-conflict';
 w.services.operationsClient={task:async task=>({task_id:task,kind:'media.generation',fluctlight_id:'A',workflow_id:'wf-conflict',status:'failed',steps:[]})};
 w.services.diagnosticsClient={workflow:async()=>({intent:{intent_type:'media.generation',status:'failed',payload:{intent_id:id}},runtime_state:'available',runtime:{run_id:'run',status:'failed'},allowed_actions:[],commands:[]}),page:async()=>({items:[{id,ref:'ref-conflict',data:{id,owner_fluctlight_id:'A',workflow_id:'wf-conflict',status:mediaStatus,revision,prompt:'{}'}}],next_cursor:''}),mediaRetry:async(mediaId,body)=>{writes.push({id:mediaId,body:structuredClone(body)});if(writes.length===1){revision=5;throw Object.assign(Error('媒体版本已变化'),{status:409,code:'revision_conflict'});}mediaStatus='pending';return {media_intent_id:mediaId,intent_id:'next-conflict',workflow_id:'wf-next-conflict',status:'retry_queued',mode:body.mode};}};
 const view=await mountReact(vite,Record,{workspace:w,props:{taskId:'task-conflict',onClose(){}}});
 try{const footer=document.querySelector('[data-slot=sheet-footer]');await view.click(button(footer,'从失败处重试'));let dialog=[...document.querySelectorAll('[role=dialog]')].at(-1);await view.input(dialog.querySelector('textarea'),'第一次恢复');await view.click(button(dialog,'确认从失败处重试'));assert.equal(writes.length,1);await view.click(dialog.querySelector('[data-slot=dialog-close]'));assert.equal(button(footer,'从失败处重试'),undefined);await view.click(button(footer,'核对新状态并审阅原提交'));dialog=[...document.querySelectorAll('[role=dialog]')].at(-1);await view.click(button(dialog,'已核对，解除被拒绝的原提交'));await view.click(button(footer,'从失败处重试'));dialog=[...document.querySelectorAll('[role=dialog]')].at(-1);await view.input(dialog.querySelector('textarea'),'核对后恢复');await view.click(button(dialog,'确认从失败处重试'));assert.equal(writes.length,2);assert.deepEqual(writes.map(x=>x.body.expectedRevision),[4,5]);assert.notEqual(writes[0].body.idempotencyKey,writes[1].body.idempotencyKey);}finally{await view.dispose();}
});

test('incomplete media recovery receipt retains the original command for safe replay',async()=>{
 const {default:Record}=await vite.ssrLoadModule('/src/react-app/shared/TaskRecordDialog.tsx'),w=workspace(),writes=[];
 const id='media-incomplete',media={id,owner_fluctlight_id:'A',workflow_id:'wf-incomplete',status:'failed',revision:2,prompt:'{}'};
 w.services.operationsClient={task:async task=>({task_id:task,kind:'media.generation',fluctlight_id:'A',workflow_id:'wf-incomplete',status:'failed',steps:[]})};
 w.services.diagnosticsClient={workflow:async()=>({intent:{intent_type:'media.generation',status:'failed',payload:{intent_id:id}},runtime_state:'available',runtime:{run_id:'run',status:'failed'},allowed_actions:[],commands:[]}),page:async()=>({items:[{id,ref:'ref-incomplete',data:media}],next_cursor:''}),mediaRetry:async(mediaId,body)=>{writes.push({id:mediaId,body:structuredClone(body)});if(writes.length===1)return {status:'retry_queued'};return {media_intent_id:id,intent_id:'next-incomplete',workflow_id:'wf-next-incomplete',mode:body.mode,status:'retry_queued'};}};
 const view=await mountReact(vite,Record,{workspace:w,props:{taskId:'task-incomplete',onClose(){}}});
 try{const footer=document.querySelector('[data-slot=sheet-footer]');await view.click(button(footer,'从失败处重试'));const dialog=[...document.querySelectorAll('[role=dialog]')].at(-1);await view.input(dialog.querySelector('textarea'),'核对恢复');await view.click(button(dialog,'确认从失败处重试'));assert.match(dialog.textContent,/回执尚未完整读到/);await view.click(dialog.querySelector('[data-slot=dialog-close]'));await view.click(button(footer,'核对或重试原提交'));assert.equal(writes.length,2);assert.deepEqual(writes[1],writes[0]);}finally{await view.dispose();}
});

for(const hasJob of [false,true])test(`media recovery warns about an unknown provider submission only when job id is ${hasJob?'saved':'missing'}`,async()=>{
 const {default:Record}=await vite.ssrLoadModule('/src/react-app/shared/TaskRecordDialog.tsx'),w=workspace();const id='media-ambiguity-'+hasJob;
 const media={id,owner_fluctlight_id:'A',workflow_id:'wf-ambiguity-'+hasJob,status:'failed',revision:2,prompt:'{}',provider_job_id:hasJob?'provider-job':''};
 w.services.operationsClient={task:async task=>({task_id:task,kind:'media.generation',fluctlight_id:'A',workflow_id:media.workflow_id,status:'failed',steps:[]})};
 w.services.diagnosticsClient={workflow:async()=>({intent:{intent_type:'media.generation',status:'failed',payload:{intent_id:id}},runtime_state:'available',runtime:{run_id:'run',status:'failed'},allowed_actions:[],commands:[]}),page:async()=>({items:[{id,ref:'ref-'+id,data:media}],next_cursor:''}),mediaRetry:async()=>{throw Error('unexpected write');}};
 const view=await mountReact(vite,Record,{workspace:w,props:{taskId:'task-ambiguity-'+hasJob,onClose(){}}});
 try{const footer=document.querySelector('[data-slot=sheet-footer]');await view.click(button(footer,'从失败处重试'));const dialog=[...document.querySelectorAll('[role=dialog]')].at(-1);if(hasJob)assert.doesNotMatch(dialog.textContent,/可能产生重复生成/);else assert.match(dialog.textContent,/可能产生重复生成/);await view.click(button(dialog,'取消'));}finally{await view.dispose();}
});

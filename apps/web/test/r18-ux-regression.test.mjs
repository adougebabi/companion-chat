import test from 'node:test';
import assert from 'node:assert/strict';
import {creationFeedback} from '../src/v3/creation-feedback.mjs';
import {connectionDraft,connectionDraftDirty,restoreConnectionDraft} from '../src/approved-ui/configuration-draft.mjs';
import {diagnosticInstant,taskEventTitle,taskStageRecords,taskFailureReason} from '../src/approved-ui/task-presentation.mjs';
import {wardrobePreparationStage} from '../src/approved-ui/wardrobe-preparation.mjs';
import {approvedHref} from '../src/approved-ui/routing.mjs';
import {feedFilters} from '../src/approved-ui/feed-presentation.mjs';

test('r18 workspace feed navigation starts with everyone while explicit role links keep their filter',()=>{
 assert.equal(approvedHref('动态','selected-role'),'#/feed');
 assert.equal(feedFilters(new URLSearchParams('fid=explicit-role')).fid,'explicit-role');
});

test('r18 provider configuration feedback never blames an internal provider path on user input',()=>{
 const feedback=creationFeedback({code:'initialization_role_unconfigured',details:{correlation_id:'original-correlation',retryable:true,validation_error:{type:'provider',path:'provider_response'}}});
 assert.equal(feedback.type,'configuration');assert.equal(feedback.path,'provider_response');assert.equal(feedback.correlationId,'original-correlation');assert.equal(feedback.retryable,true);assert.doesNotMatch(feedback.message,/provider_response|修正原输入|问题字段/);assert.match(feedback.message,/通用模型/);
 const input=creationFeedback({code:'initialization_input_invalid',details:{validation_error:{type:'input',path:'core_persona.identity.name'}}});assert.match(input.message,/core_persona.identity.name/);
});
const catalog={endpoints:[{id:'S',kind:'openai-compatible',base_url:'https://model.invalid/v1'}],bindings:[{role:'generic_llm',endpoint_id:'S',model_id:'model-original',timeout_seconds:120}],versions:{'endpoint:S':7,'binding:generic_llm':5,'product.wakeup':2}};
const form=()=>({endpoints:[{id:'S',name:'S',protocol:'openai-compatible',url:'https://model.invalid/v1',models:['read-only-directory'],modelsStatus:'ready'}],bindings:[{role:'generic_llm',endpoint:'S',model:'model-original',timeout:120}],versions:{...catalog.versions}});
test('r18 directory reads are not connection edits; actual edits retain their own CAS and unrelated versions stay current',()=>{
 const clean=connectionDraft(form());assert.equal(connectionDraftDirty(clean,catalog),false);assert.equal(restoreConnectionDraft(JSON.stringify(clean),catalog).dirty,false);
 const f=form();f.endpoints[0].url='https://new-input.invalid/v1';const draft=connectionDraft(f),next=structuredClone(catalog);next.versions['product.wakeup']=8;
 const unchangedConnection=restoreConnectionDraft(JSON.stringify(draft),next);assert.equal(unchangedConnection.stale,false);assert.equal(unchangedConnection.versions['product.wakeup'],8);assert.equal(unchangedConnection.endpoints[0].url,f.endpoints[0].url);
 next.versions['endpoint:S']=8;const stale=restoreConnectionDraft(JSON.stringify(draft),next);assert.equal(stale.stale,true);assert.equal(stale.versions['endpoint:S'],7);assert.equal(stale.versions['binding:generic_llm'],5);assert.equal(stale.versions['product.wakeup'],8);
 assert.equal(connectionDraftDirty({...clean,credentialPending:true},catalog),true);
});
test('r18 lifecycle events and named infrastructure records never become business phases; seconds distinguish events',()=>{
 const events=[{id:'new',event_type:'lifecycle.wardrobe.failed',status:'failed',at:'2026-10-29T00:00:45Z'},{id:'old',event_type:'lifecycle.wardrobe.started',status:'running',at:'2026-10-29T00:00:15Z'}];
 assert.equal(taskStageRecords({steps:events}).length,0);assert.notEqual(taskEventTitle(events[0]),taskEventTitle(events[1]));assert.doesNotMatch(taskEventTitle(events[0]),/^执行阶段$/);
 assert.match(diagnosticInstant(events[0].at),/08:00:45/);assert.match(diagnosticInstant(events[1].at),/08:00:15/);
 for(const name of ['dispatcher','workflow_start','activity','workflow_reconcile','初始衣柜'])assert.equal(taskStageRecords({steps:events.map(e=>({...e,name}))}).length,0);
});
test('r18 wardrobe configuration cause is adopted only from its exact original task and keeps domain failure data',()=>{
 const preparation={task_id:'original-task',status:'failed',can_recover:true,error_code:'wardrobe_preparation_failed',result:{failure_reason:'旧的通用失败说明'}},raw=JSON.stringify(preparation),task={task_id:'original-task',source:{last_error:'provider role generic_llm unavailable: no rows in result set'}};
 assert.match(wardrobePreparationStage(preparation,'',task).note,/通用模型.*就绪/);assert.equal(wardrobePreparationStage(preparation,'',{...task,task_id:'foreign-task'}).note,preparation.result.failure_reason);assert.match(taskFailureReason(task,null),/已有角色、世界/);assert.equal(JSON.stringify(preparation),raw);
});
test('r18 wardrobe phase display uses committed domain facts and never fabricates stage timestamps',()=>{
 const task={task_id:'original',kind:'wardrobe.initialize',fluctlight_id:'A',steps:[]},preparation={task_id:'original',fluctlight_id:'A',status:'failed',result:{inventory_committed:false}};
 let stages=taskStageRecords(task,preparation);assert.deepEqual(stages.map(s=>s.status),['failed','blocked']);assert.ok(stages.every(s=>s.at===undefined));
 stages=taskStageRecords(task,{...preparation,result:{inventory_committed:true}});assert.deepEqual(stages.map(s=>s.status),['completed','failed']);assert.equal(taskStageRecords(task,{...preparation,fluctlight_id:'foreign'}).length,0);
 assert.deepEqual(taskStageRecords(task,{...preparation,result:{inventory_committed:true},status:'completed'}).map(s=>s.status),['completed','completed']);
 assert.deepEqual(taskStageRecords(task,{...preparation,result:undefined}).map(s=>s.status),['failed','blocked']);
 assert.deepEqual(taskStageRecords(task,{...preparation,result:undefined,status:'completed'}).map(s=>s.status),['pending','pending']);
 assert.deepEqual(taskStageRecords(task,{...preparation,result:{inventory_committed:true},status:'awaiting_wear'}).map(s=>s.status),['completed','running']);
 assert.equal(taskStageRecords(task,{...preparation,task_id:'new-recovery-task'}).length,0);
 assert.equal(taskStageRecords(task,{...preparation,fluctlight_id:undefined}).length,0);
});

test('diagnostic primary dates stay compact while execution clocks retain recorded sub-second precision',async()=>{
 const {diagnosticDateLabel,diagnosticTimeLabel}=await import('../src/approved-ui/task-presentation.mjs');
 const now=Date.parse('2026-10-30T08:00:00Z');assert.equal(diagnosticDateLabel('2026-10-30T06:18:02.123Z',now),'今天 14:18');assert.equal(diagnosticDateLabel('2026-10-29T06:18:02Z',now),'10-29 14:18');assert.equal(diagnosticDateLabel('2025-10-29T06:18:02Z',now),'2025-10-29 14:18');assert.equal(diagnosticTimeLabel('2026-10-30T06:18:02Z'),'14:18:02');assert.equal(diagnosticTimeLabel('2026-10-30T06:18:02.123Z'),'14:18:02.123');assert.equal(diagnosticTimeLabel('bad'),'未记录时间');
});

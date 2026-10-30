import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {act,installDOM,mountReact} from './helpers/react-dom.mjs';
import {creationFixture} from './support/creation-fixture.mjs';
const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const {default:Record}=await vite.ssrLoadModule('/src/react-app/shared/TaskRecordDialog.tsx');
const {default:Wizard}=await vite.ssrLoadModule('/src/react-app/views/CharacterWizard.jsx');
test.after(async()=>{await vite.close();env.restore();});
const button=(host,text)=>[...host.querySelectorAll('button')].find(node=>node.textContent.trim()===text);
async function tab(view,label){await act(async()=>[...document.querySelectorAll('[role=tab]')].find(node=>node.textContent.trim()===label).dispatchEvent(new window.MouseEvent('mousedown',{bubbles:true,button:0})));await view.flush();}
function workspace(task,page=async()=>({items:[],next_cursor:''})){
 const state=proxy({session:{actorId:'r22-task-owner'},characters:[]});
 return {state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),services:{operationsClient:{task:async()=>task},diagnosticsClient:{page}}};
}
const lifecycle=()=>Array.from({length:9},(_,index)=>({id:'diagnostic_internal_'+index,event_type:'lifecycle.persona_compilation.failed',name:'workflow_describe',reason_code:'persona_compilation_retrying',status:'failed',at:'2026-10-30T01:00:00Z'}));
function wizardModel(f,stages,w){return {w,c:f.c,draft:f.c.draft,currentStep:3,step:'progress',busy:false,pending:null,error:'',fieldErrors:{},groups:[],existingWorlds:[],worlds:[],templates:[],previewFields:[],progress:{complete:false,stages},goStep(){},configHref:()=> '#/settings'};}

test('r22 execution results count physical calls, disclose each input/output/error and keep raw lifecycle closed',async()=>{
 const reads=[],task={task_id:'T',status:'failed',correlation_id:'exact-correlation',steps:lifecycle(),runs:3};
 const records=[{ref:'second',data:{sequence:2,status:'failed',started_at:'2026-10-30T01:00:02Z',completed_at:'2026-10-30T01:00:03.250Z',prompt:0,response:false,error:{message:'实际失败'},error_code:'raw_provider_error'}},{ref:'first',data:{sequence:1,status:'completed',latency_ms:0,started_at:'2026-10-30T01:00:00Z',prompt:'真实输入',response:'真实输出'}},{ref:'pending',data:{status:'pending',prompt:null,response:null}}];
 const w=workspace(task,async(kind,query)=>{reads.push({kind,query});return {items:records,next_cursor:''};}),v=await mountReact(vite,Record,{workspace:w,props:{taskId:'T',onClose(){}}});
 try{
  const sheet=document.querySelector('[data-slot=sheet-content]');assert.equal(sheet.dataset.side,'right');assert.deepEqual(reads,[]);
  await tab(v,'执行记录');assert.deepEqual(reads,[{kind:'model-runs',query:{correlationId:'exact-correlation',limit:20}}]);assert.deepEqual([...sheet.querySelectorAll('.replica-task-result-summary dd')].map(node=>node.textContent),['3','1','1']);assert.equal(sheet.querySelectorAll('.replica-task-result-list>li').length,3);
  assert.equal(sheet.querySelector('.replica-task-round-content').textContent,'');assert.doesNotMatch(sheet.textContent,/diagnostic_internal|persona_compilation_retrying|本条记录未提供可识别/);
  const rows=[...sheet.querySelectorAll('.replica-task-result-list>li')];assert.match(rows[0].textContent,/第 1 轮.*成功.*0 毫秒/);assert.match(rows[1].textContent,/第 2 轮.*失败.*1.25 秒/);assert.match(rows[2].textContent,/调用记录 3.*耗时未记录/);
  await v.click(rows[1].querySelector('[data-slot=collapsible-trigger]'));const content=rows[1].querySelector('.replica-task-round-content');assert.deepEqual([...content.querySelectorAll('section h3')].map(node=>node.textContent),['输入','输出','错误']);assert.match(content.querySelectorAll('section')[0].textContent,/输入0/);assert.match(content.querySelectorAll('section')[1].textContent,/输出否/);assert.match(content.textContent,/实际失败/);assert.doesNotMatch(content.textContent,/raw_provider_error/);assert.equal(button(content,'本轮原始记录').getAttribute('aria-expanded'),'false');
  await v.click(button(sheet,'生命周期记录 · 9 条'));assert.equal(sheet.querySelectorAll('.replica-task-history>li').length,9);assert.doesNotMatch(sheet.textContent,/diagnostic_internal|persona_compilation_retrying|本条记录未提供可识别/);assert.equal(button(sheet,'生命周期原始记录').getAttribute('aria-expanded'),'false');
  await v.click(button(sheet,'生命周期原始记录'));assert.match(sheet.textContent,/diagnostic_internal_0/);
 }finally{await v.dispose();}
});

test('r22 failed pre-model task reports zero physical rounds without turning nine lifecycle events into attempts',async()=>{
 const task={task_id:'pre-model',status:'failed',correlation_id:'pre-model-correlation',runs:2,steps:lifecycle()},v=await mountReact(vite,Record,{workspace:workspace(task),props:{taskId:task.task_id,onClose(){}}});
 try{await tab(v,'执行记录');const sheet=document.querySelector('[data-slot=sheet-content]');assert.deepEqual([...sheet.querySelectorAll('.replica-task-result-summary dd')].map(node=>node.textContent),['0','0','0']);assert.match(sheet.textContent,/进入模型执行前已停止/);assert.equal(sheet.querySelector('.replica-task-result-list'),null);assert.equal(sheet.querySelector('.replica-task-history'),null);}finally{await v.dispose();}
});

test('r22 task round pagination retries original correlation and discards responses after task changes',async()=>{
 let fail=true,late;const reads=[],task={task_id:'first',correlation_id:'first-correlation',status:'running',steps:[]};
 const w=workspace(task,async(kind,query)=>{reads.push({kind,...query});if(query.correlationId==='second-correlation')return {items:[{ref:'second',data:{status:'completed',prompt:'当前任务'}}],next_cursor:''};if(query.cursor){if(fail)throw Error('fetch failed');return new Promise(resolve=>late=resolve);}return {items:[{ref:'original',data:{status:'completed'}}],next_cursor:'original-cursor'};}),v=await mountReact(vite,Record,{workspace:w,props:{taskId:'first',onClose(){}}});
 try{
  await tab(v,'执行记录');const sheet=document.querySelector('[data-slot=sheet-content]');assert.equal(sheet.querySelector('.replica-task-result-summary dt').textContent,'已读取轮次');await v.click(button(sheet,'读取更多执行轮次'));assert.ok(button(sheet,'重新读取执行轮次'));assert.equal(sheet.querySelectorAll('.replica-task-result-list>li').length,1);
  fail=false;await act(async()=>button(sheet,'重新读取执行轮次').click());await v.flush();assert.equal(reads.at(-1).cursor,'original-cursor');assert.equal(reads.at(-1).correlationId,'first-correlation');
  w.services.operationsClient.task=async()=>({...task,task_id:'second',correlation_id:'second-correlation'});await v.update({taskId:'second',onClose(){}});await tab(v,'执行记录');await act(async()=>late({items:[{ref:'late-old',data:{status:'failed',prompt:'旧任务输入'}}],next_cursor:'stale'}));await v.flush();assert.equal(sheet.querySelectorAll('.replica-task-result-list>li').length,1);await v.click(sheet.querySelector('.replica-task-round-trigger'));assert.match(sheet.textContent,/当前任务/);assert.doesNotMatch(sheet.textContent,/旧任务输入/);assert.equal(button(sheet,'读取更多执行轮次'),undefined);
 }finally{late?.({items:[],next_cursor:''});await v.dispose();}
});

test('r22 wizard opens task as a right Sheet and restores its original trigger and hash',async()=>{
 const f=creationFixture();f.c.draft.activatedId='created';const stages=[{key:'world',title:'世界',state:'ready'},{key:'wardrobe',title:'初始衣柜',state:'failed',href:'#/diag/task/exact%2Ftask'}],w=workspace({task_id:'exact/task',status:'failed',steps:[]}),v=await mountReact(vite,Wizard,{workspace:w,props:{model:wizardModel(f,stages,w)}});
 try{location.hash='#/library/create?resume=created';const hash=location.hash,trigger=button(v.host,'查看任务记录');await v.click(trigger);const sheet=document.querySelector('[data-slot=sheet-content]');assert.equal(sheet.dataset.side,'right');assert.equal(document.querySelector('[data-slot=dialog-content]'),null);assert.equal(location.hash,hash);await act(async()=>sheet.dispatchEvent(new window.KeyboardEvent('keydown',{key:'Escape',bubbles:true})));await v.flush();assert.equal(document.querySelector('[role=dialog]'),null);assert.equal(document.activeElement,trigger);assert.equal(location.hash,hash);}finally{await v.dispose();f.c.dispose();}
});

test('r22 management task detail uses the same right Sheet and returns to its task row',async()=>{
 const {default:Host}=await vite.ssrLoadModule('/src/react-app/shared/DiagnosticsWorkspace.jsx');
 const w=workspace({task_id:'management-original',status:'failed',steps:[]},async()=>({items:[{ref:'management-row',data:{task_id:'management-original',kind:'persona.compile',status:'failed'}}],next_cursor:''}));
 w.navigatePath=path=>{location.hash='#'+path;window.dispatchEvent(new window.HashChangeEvent('hashchange'));};
 const v=await mountReact(vite,Host,{workspace:w,props:{view:{openTaskDiagnostics(){}}}});
 try{const trigger=v.host.querySelector('.replica-task-row');await v.click(trigger);const sheet=document.querySelector('[data-slot=sheet-content]');assert.equal(sheet?.dataset.side,'right');assert.ok(sheet.querySelector('[data-slot=sheet-footer]'));assert.equal(document.querySelector('[data-slot=dialog-content]'),null);await v.click(sheet.querySelector('[data-slot=sheet-close]'));assert.equal(document.querySelector('[role=dialog]'),null);assert.equal(document.activeElement,trigger);assert.equal(new URLSearchParams(location.hash.split('?')[1]).has('taskId'),false);}finally{await v.dispose();}
});

test('r22 skip explicitly defers waiting, persists progress and preserves failed task and original recovery request',async()=>{
 const f=creationFixture(),moves=[];f.c.draft.activatedId='created';
 const stages=[{key:'world',title:'世界',state:'ready'},{key:'character',title:'人格资料整理',state:'failed',retryable:true},{key:'wardrobe',title:'初始衣柜',state:'blocked'}];f.c.progress.value={complete:false,stages};
 const recoveryKey='fluctlight-creation:owner:created:recover:character',original='{"originalBody":{"expectedRevision":3},"idempotencyKey":"original-key"}';f.saved.setItem(recoveryKey,original);
 const w={navigatePath:path=>moves.push(path)},m=wizardModel(f,stages,w),v=await mountReact(vite,Wizard,{props:{model:m}});
 try{
  const trigger=button(v.host,'跳过此环节');assert.ok(trigger);await v.click(trigger);let dialog=document.querySelector('[role=dialog]');assert.match(dialog.textContent,/原任务与恢复入口保留/);await v.click(button(dialog,'继续处理'));assert.deepEqual(moves,[]);assert.equal(f.c.draft.preparationDeferrals,undefined);
  await v.click(trigger);dialog=document.querySelector('[role=dialog]');await v.click(button(dialog,'保留进度并跳过'));assert.deepEqual(moves,['/c/created/chat']);assert.ok(Date.parse(f.c.draft.preparationDeferrals.character));assert.equal(f.c.progress.value.complete,false);assert.equal(f.c.progress.value.stages[1].state,'failed');assert.equal(f.c.progress.value.stages[2].state,'blocked');assert.equal(f.saved.getItem(recoveryKey),original);assert.deepEqual(f.calls,[]);
  const archive=JSON.parse(f.saved.getItem('fluctlight-creation:owner:history:created'));assert.ok(archive.preparationDeferrals.character);assert.equal(archive.activatedId,'created');await v.update({model:{...m,draft:f.c.draft}});assert.match(v.host.textContent,/已跳过本次等待/);assert.ok(button(v.host,'重试此环节'));
  assert.equal(f.c.deferStage({key:'world'}),false);assert.equal(f.c.deferStage({key:'fabricated'}),false);
 }finally{await v.dispose();f.c.dispose();}
});

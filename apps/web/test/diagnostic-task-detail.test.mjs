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
 const {taskStatusLabel,taskEventText}=await vite.ssrLoadModule('/src/approved-ui/task-presentation.mjs');
 for(const status of ['constructor','toString','__proto__','hasOwnProperty'])assert.equal(taskStatusLabel(status),'状态待确认');
 assert.equal(taskStatusLabel('running'),'进行中');assert.equal(taskStatusLabel('实际扩展状态'),'实际扩展状态');
 assert.equal(taskEventText({event_type:'lifecycle.wardrobe.started',status:'started'}),'穿搭准备 · 已开始');
 assert.equal(taskEventText({event_type:'lifecycle.wardrobe.failed',status:'failed'}),'穿搭准备 · 执行失败');
 assert.equal(taskEventText({event_type:'lifecycle.wardrobe.started',status:'running'}),'穿搭准备 · 已开始 · 执行中');
 assert.equal(taskEventText({status:'queued'}),'排队中记录');
});
for(const presentation of ['dialog','sheet'])test(`${presentation} task recovery is owned once and portaled outside its scrolling body across tabs`,async()=>{
 const {default:Record}=await vite.ssrLoadModule('/src/react-app/shared/TaskRecordDialog.tsx'),w=workspace();let workflowReads=0,modelReads=0;const writes=[];
 const original={task_id:'T-'+presentation,kind:'persona_compile',fluctlight_id:'A',correlation_id:'real correlation',workflow_id:'W-'+presentation,status:'failed',runs:2,steps:[{id:'later',event_type:'failed',status:'failed',at:'2026-10-30T01:02:00Z'},{id:'earlier',event_type:'queued',status:'queued',at:'2026-10-30T01:00:00Z'}],has_more_steps:true};
 w.services.operationsClient={task:async()=>original};w.services.diagnosticsClient={workflow:async()=>{workflowReads++;return {intent:{status:'failed',revision:7},runtime_state:'available',runtime:{run_id:'original-run',status:'failed'},allowed_actions:['restart'],commands:[]};},page:async()=>{modelReads++;throw Error('task detail must not read model runs');},control:async(id,action,body)=>{writes.push({id,action,body:structuredClone(body)});if(writes.length===1)throw Error('fetch failed');return {status:'accepted'};}};
 const view=await mountReact(vite,Record,{workspace:w,props:{taskId:original.task_id,presentation,onClose(){},returnTo:'#/library/create?resume=original'}});
 try{
  const outer=document.querySelector('[role=dialog]'),body=outer.querySelector(presentation==='sheet'?'.creation-task-sheet-body':'.replica-dialog-body'),footer=outer.querySelector(`[data-slot=${presentation==='sheet'?'sheet':'dialog'}-footer]`);
  assert.ok(footer);assert.equal(body.contains(footer),false);assert.equal(outer.querySelector('[role=tab][data-state=active]').textContent,'执行阶段');assert.match(outer.textContent,/原角色 · 已尝试 2 次/);assert.equal(outer.querySelector('[role=progressbar]'),null);assert.ok(button(footer,'重试任务'));assert.equal(body.querySelector('[data-slot=card]'),null);
  await tab(view,'执行记录');assert.match(outer.textContent,/执行记录 已读取 2/);assert.deepEqual([...outer.querySelectorAll('.replica-task-history time')].map(node=>node.dateTime),['2026-10-30T01:00:00Z','2026-10-30T01:02:00Z']);const events=[...outer.querySelectorAll('.replica-task-history h3')].map(node=>node.textContent);assert.ok(events[0].includes('09:00:00'));assert.ok(events[1].includes('09:02:00'));
  await tab(view,'任务信息');const link=[...outer.querySelectorAll('a')].find(node=>node.textContent.trim()==='查看关联模型调用');assert.ok(link);assert.ok(outer.querySelector('[role=tabpanel][data-state=active]').contains(link));const route=new URLSearchParams(link.hash.split('?')[1]);assert.equal(route.get('correlationId'),'real correlation');assert.match(route.get('returnTo'),/returnTo=/);await tab(view,'执行阶段');assert.equal(workflowReads,1);assert.equal(modelReads,0);
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
  const outer=document.querySelector('[role=dialog]'),footer=outer.querySelector('[data-slot=dialog-footer]');assert.equal(button(footer,'从检查点重新执行').disabled,true);await tab(view,'任务信息');await view.click(button(outer,'更多任务记录'));await view.click(button(outer,'执行历史与检查点'));await view.click(button(outer,'读取执行历史'));assert.equal(historyReads,1);
  const select=outer.querySelector('[aria-label="重置检查点"]');assert.ok(select);await view.click(select);const option=[...document.querySelectorAll('[role=option]')].find(node=>node.textContent.includes('事件 42'));assert.ok(option);assert.equal([...document.querySelectorAll('[role=option]')].some(node=>node.textContent.includes('事件 43')),false);await view.click(option);await tab(view,'执行阶段');assert.equal(button(footer,'从检查点重新执行').disabled,false);await view.click(button(footer,'从检查点重新执行'));
  const confirmation=[...document.querySelectorAll('[role=dialog]')].at(-1);await view.input(confirmation.querySelector('textarea'),'核对后的检查点');await view.click(button(confirmation,'确认从检查点重新执行'));assert.equal(writes.length,1);assert.equal(writes[0].body.historyPoint,42);assert.equal(writes[0].body.sourceRunId,'reset-run');assert.equal(writes[0].body.expectedRevision,12);assert.equal(confirmation.querySelector('[data-slot=dialog-close]'),null);assert.equal(button(confirmation,'取消').disabled,true);await act(async()=>finish());await view.flush();assert.match(footer.textContent,/受理不表示执行完成/);
 }finally{finish?.();await view.dispose();}
});

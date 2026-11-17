import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {act,mountReact,installDOM} from './helpers/react-dom.mjs';
const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),envFile:false,appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();env.restore();});
const button=(text)=>[...document.querySelectorAll('button')].find(node=>node.textContent.trim()===text);
const task={task_id:'goal_planner_private',title:'goal.plan',kind:'goal.plan',status:'failed',workflow_id:'goal-planner:private',correlation_id:'diagnostic_private',created_at:'2026-11-16T00:00:00Z',started_at:'2026-11-16T00:01:00Z',completed_at:'2026-11-16T00:02:00Z',next_attempt_at:'2026-11-17T00:00:00Z',runs:1,has_more_steps:false,source:{last_error:'sql: no rows in result set',state:'failed'},steps:[{id:'private-event',event_type:'lifecycle.workflow.failed',status:'failed',at:'2026-11-16T00:02:00Z',reason_code:'workflow_describe_failed',safe_cause:'sql: no rows in result set'}]};
const forbidden=/sql:|goal\.plan|goal-planner:|goal_planner_private|diagnostic_private|workflow_describe_failed|lifecycle\.workflow\.failed|has_more_steps|next_attempt_at|last_error|2026-11-\d\dT/;
function businessText(host){return host.textContent;}
async function tab(v,label){await act(async()=>[...document.querySelectorAll('[role=tab]')].find(node=>node.textContent.trim().startsWith(label)).dispatchEvent(new window.MouseEvent('mousedown',{bubbles:true,button:0})));await v.flush();}
test('task summaries map planning errors and internal titles without claiming a running task stopped',async()=>{
 const {taskFailureReason,taskDisplayTitle,taskTypeLabel,taskEventReason,diagnosticInstant}=await vite.ssrLoadModule('/src/approved-ui/task-presentation.mjs');
 assert.equal(taskDisplayTitle(task),'独立目标规划');
 assert.equal(taskDisplayTitle({...task,kind:'unknown_wire_type'}),'独立目标规划');
 assert.equal(taskDisplayTitle({...task,title:'unknown_internal_title'}),'独立目标规划');
 assert.equal(taskTypeLabel(task),'独立目标规划');
 assert.equal(taskFailureReason(task),'规划任务没有找到对应资料，已停止。');
 assert.doesNotMatch(taskFailureReason({...task,status:'running'}),/已停止/);
 assert.doesNotMatch(taskFailureReason({...task,source:{last_error:'provider role generic_llm preflight failed: sql: no rows in result set'}}),/没有找到对应资料/);
 assert.doesNotMatch(taskFailureReason({...task,source:{last_error:'unexpected driver failure'}}),/[a-z]/i);
 assert.doesNotMatch(taskEventReason(task.steps[0]),forbidden);
 assert.match(diagnosticInstant(task.created_at),/2026-11-16 08:00:00\.000（北京时间）/);
});
test('task dialog keeps original fields out of the DOM through all detail disclosures',async()=>{
 const {default:Dialog}=await vite.ssrLoadModule('/src/react-app/shared/TaskRecordDialog.tsx');
 const state=proxy({session:{actorId:'fixture'},characters:[]});let reads=0;
 const w={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),navigatePath(){},services:{operationsClient:{task:async()=>task},diagnosticsClient:{workflow:async()=>({intent:{intent_type:'goal.plan',status:'failed'},runtime_state:'unavailable',runtime_error:{message:'workflow_describe_failed'},allowed_actions:[],commands:[]}),history:async()=>{reads++;return {items:[]};}}}};
 const v=await mountReact(vite,Dialog,{workspace:w,props:{taskId:task.task_id,onClose(){}}});
 try{
  const host=document.querySelector('[role=dialog]');
  assert.match(host.textContent,/独立目标规划/);assert.match(host.textContent,/规划任务没有找到对应资料，已停止/);assert.doesNotMatch(businessText(host),forbidden);
  await tab(v,'执行记录');assert.doesNotMatch(businessText(host),forbidden);
  await v.click(button('生命周期原始记录'));assert.equal(host.querySelector('pre, code'),null);assert.doesNotMatch(host.textContent,forbidden);assert.ok(button('下载原始记录'));
  await tab(v,'任务信息');assert.match(host.textContent,/2026-11-16 08:00:00\.000（北京时间）/);assert.match(host.textContent,/2026-11-17 08:00:00\.000（北京时间）/);
  await v.click(button('查看完整任务记录'));
  for(const title of ['完整任务信息与关联来源','业务阶段依据与来源','管理命令与技术详情'])await v.click(button(title));
  assert.doesNotMatch(businessText(host),forbidden);
  assert.equal(host.querySelector('pre, code'),null);assert.ok(button('下载原始记录'));assert.doesNotMatch(host.textContent,/原始 JSON/);assert.equal(reads,0);
  assert.equal(host.querySelector('.record-facts'),null);
 }finally{await v.dispose();}
});
test('download-only details preserve stage evidence in downloaded source without rendering fields',async()=>{
 const {default:Details}=await vite.ssrLoadModule('/src/react-app/shared/JsonDetails.tsx');
 const value=[{id:'private-stage',source:task}];
 const v=await mountReact(vite,Details,{props:{value,title:'业务阶段依据与来源',downloadOnly:true}});
 const originalCreate=URL.createObjectURL,originalRevoke=URL.revokeObjectURL,anchor=v.host.ownerDocument.defaultView.HTMLAnchorElement.prototype,originalClick=anchor.click;let blob,download;
 anchor.click=function(){download=this.download;};
 URL.createObjectURL=value=>{blob=value;return 'blob:fixture';};URL.revokeObjectURL=()=>{};
 try{await v.click(button('业务阶段依据与来源'));assert.equal(v.host.querySelector('pre, code'),null);assert.doesNotMatch(businessText(v.host),forbidden);await v.click(button('下载原始记录'));assert.deepEqual(JSON.parse(await blob.text()),value);assert.equal(download,'fluctlight-record.json');}finally{anchor.click=originalClick;URL.createObjectURL=originalCreate;URL.revokeObjectURL=originalRevoke;await v.dispose();}
});
test('task workflow history translates event headings and keeps exact evidence in downloads',async()=>{
 const {default:Recovery}=await vite.ssrLoadModule('/src/react-app/shared/WorkflowRecovery.tsx');
 const state=proxy({session:{actorId:'history-fixture'}});
 const event={resettable:true,id:123456789,type:'lifecycle.workflow.failed',at:task.completed_at,details:{reason_code:'workflow_describe_failed',error:'sql: no rows in result set'}};
 const w={state,services:{diagnosticsClient:{workflow:async()=>({intent:{status:'failed'},runtime_state:'available',runtime:{run_id:'private-run',status:'failed'},allowed_actions:['reset'],commands:[{id:'private-command',action:'internal_action',status:'failed',reason:'sql: no rows in result set'}]}),history:async()=>({items:[event],next_cursor:''})}}};
 const v=await mountReact(vite,Recovery,{workspace:w,props:{workflowId:task.workflow_id,presentation:'task'}});
 try{await v.click(button('执行历史与检查点'));await v.click(button('读取执行历史'));assert.match(v.host.textContent,/北京时间/);assert.doesNotMatch(businessText(v.host),forbidden);await v.click(button('事件详情'));assert.equal(v.host.querySelector('pre, code'),null);assert.ok(button('下载原始记录'));assert.doesNotMatch(businessText(v.host),forbidden);await v.click(button('管理命令与技术详情'));assert.doesNotMatch(businessText(v.host),/sql:|internal_action|123456789/);}finally{await v.dispose();}
});

import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {act,mountReact,installDOM} from './helpers/react-dom.mjs';

const env=installDOM();
const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),envFile:false,appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();env.restore();});
const button=(host,title)=>[...host.querySelectorAll('button')].find(node=>node.textContent.trim()===title);
const task={task_id:'goal_planner_private',kind:'goal.plan',title:'goal.plan',fluctlight_id:'private-character',status:'failed',workflow_id:'workflow-private',correlation_id:'correlation-private',created_at:'2026-11-16T00:00:00Z',source:{last_error:'sql: no rows in result set'},steps:[{id:'diagnostic-private',event_type:'lifecycle.workflow.failed',reason_code:'workflow_describe_failed',status:'failed'}]};
const command={id:'command-private',action:'private-action',status:'failed',reason:'sql: no rows in result set'};
const workflow={intent:{intent_type:'goal.plan',status:'failed'},runtime_state:'available',runtime:{run_id:'run-private',status:'failed'},allowed_actions:[],commands:[command]};
const event={id:'event-private',type:'lifecycle.workflow.failed',at:task.created_at,details:{error:'sql: no rows in result set',correlation_id:task.correlation_id}};
const forbidden=/sql: no rows|goal\.plan|goal_planner_private|workflow-private|correlation-private|diagnostic-private|command-private|private-action|run-private|event-private|原始 JSON/;
test('media failures retain recorded Provider HTTP status without exposing transport internals',async()=>{
 const {taskFailureReason,taskEventReason,taskModelFailureReason}=await vite.ssrLoadModule('/src/approved-ui/task-presentation.mjs');
 for(const entry of [{status:'queued',reason_code:'dispatcher_selected'},{status:'started',reason_code:'workflow_dispatched'},{status:'completed',reason_code:'workflow_terminal_completed'},{status:'unknown_state',reason_code:'unknown_internal_reason'}])assert.equal(taskEventReason(entry),'');
 assert.match(taskEventReason({status:'failed',reason_code:'unknown_internal_failure'}),/本次执行未完成/);
 assert.equal(taskEventReason({status:'started',reason_code:'workflow_dispatched',safe_cause:'已交给执行器。'}),'已交给执行器。');
 for(const status of [400,503]){
  const cause=`media prompt generation failed: Post "http://private-service/v1/chat/completions": provider HTTP ${status}: PRIVATE_RESPONSE_BODY node path: [private-node]`;
  for(const message of [taskFailureReason({kind:'media.generation',status:'failed',source:{last_error:cause}}),taskEventReason({safe_cause:cause})]){assert.match(message,new RegExp('HTTP '+status));assert.doesNotMatch(message,/private-service|PRIVATE_RESPONSE_BODY|private-node|node path/);}
 }
 assert.equal(taskFailureReason({kind:'media.generation',status:'failed',source:{}}),'失败原因尚未记录。');
 assert.doesNotMatch(taskFailureReason({kind:'media.generation',status:'failed',source:{last_error:'unknown raw transport error with internal-id'}}),/unknown raw|internal-id/);
 const mixed='图片生成失败 https://private-service/render PRIVATE_RESPONSE_BODY node path: [private-node]';
 for(const message of [taskFailureReason({kind:'media.generation',status:'failed',source:{last_error:mixed}}),taskEventReason({safe_cause:mixed}),taskModelFailureReason({error:{message:mixed}})])assert.doesNotMatch(message,/private-service|PRIVATE_RESPONSE_BODY|private-node|node path/);
});
function clean(host){assert.doesNotMatch(host.textContent,forbidden);assert.equal(host.querySelector('pre, code'),null);}
async function tab(v,host,title){await act(async()=>[...host.querySelectorAll('[role=tab]')].find(node=>node.textContent.trim().startsWith(title)).dispatchEvent(new window.MouseEvent('mousedown',{bubbles:true,button:0})));await v.flush();}

for(const presentation of ['sheet','dialog'])test(`${presentation}: every task detail disclosure keeps raw records only in downloads`,async()=>{
 const {default:Dialog}=await vite.ssrLoadModule('/src/react-app/shared/TaskRecordDialog.tsx');
 const state=proxy({session:{actorId:'fixture'},characters:[{id:task.fluctlight_id,name:'FT-白板'}]});
 const w={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),navigatePath(){},services:{operationsClient:{task:async()=>task},diagnosticsClient:{workflow:async()=>workflow,history:async()=>({items:[event]})}}};
 const v=await mountReact(vite,Dialog,{workspace:w,props:{taskId:task.task_id,presentation,onClose(){}}});
 const host=document.querySelector('[role=dialog]');
 const blobs=[],downloads=[],originalCreate=URL.createObjectURL,originalRevoke=URL.revokeObjectURL,anchor=window.HTMLAnchorElement.prototype,originalClick=anchor.click;
 URL.createObjectURL=blob=>{blobs.push(blob);return 'blob:fixture';};URL.revokeObjectURL=()=>{};anchor.click=function(){downloads.push(this.download);};
 try{
  assert.match(host.textContent,/FT-白板.*独立目标规划/);assert.match(host.textContent,/规划任务没有找到对应资料，已停止/);clean(host);
  await tab(v,host,'执行阶段');await v.click(button(host,'生命周期原始记录'));clean(host);await v.click(button(host,'下载原始记录'));
  assert.deepEqual(JSON.parse(await blobs.at(-1).text()),task.steps);
  await tab(v,host,'任务信息');assert.match(host.textContent,/2026-11-16 08:00:00\.000（北京时间）/);
  await v.click(button(host,'查看完整任务记录'));await v.click(button(host,'完整任务信息与关联来源'));clean(host);
  await v.click(button(host,'下载原始记录'));assert.deepEqual(JSON.parse(await blobs.at(-1).text()),task);
  await v.click(button(host,'业务阶段依据与来源'));clean(host);
  await v.click(button(host,'执行历史与检查点'));await v.click(button(host,'读取执行历史'));await v.click(button(host,'事件详情'));clean(host);
  await v.click(button(host,'管理命令与技术详情'));await v.click(button(host,'原命令回执'));await v.click(button(host,'完整恢复记录'));clean(host);
  const buttons=[...host.querySelectorAll('button')].filter(node=>node.textContent==='下载原始记录');assert.ok(buttons.length>=4);
  for(const node of buttons)await v.click(node);
  const records=await Promise.all(blobs.map(async blob=>JSON.parse(await blob.text())));
  for(const value of [task,event,command,workflow])assert.ok(records.some(record=>JSON.stringify(record)===JSON.stringify(value)));
  assert.ok(downloads.every(name=>name==='fluctlight-record.json'));clean(host);
 }finally{URL.createObjectURL=originalCreate;URL.revokeObjectURL=originalRevoke;anchor.click=originalClick;await v.dispose();}
});

test('default developer details still show complete fields; empty download-only details have no download',async()=>{
 const {default:Details}=await vite.ssrLoadModule('/src/react-app/shared/JsonDetails.tsx');
 const v=await mountReact(vite,Details,{props:{title:'开发者详情',value:task}});
 try{
  await v.click(button(v.host,'开发者详情'));assert.match(v.host.textContent,/sql: no rows in result set/);assert.match(v.host.textContent,/goal_planner_private/);assert.ok(button(v.host,'下载原始记录'));
  for(const value of [null,{},[]]){
   await v.update({title:'开发者详情',value,downloadOnly:true});assert.match(v.host.textContent,/尚未记录相关详情/);assert.equal(button(v.host,'下载原始记录'),undefined);clean(v.host);
  }
 }finally{await v.dispose();}
});

test('workflow developer card keeps original event and recovery JSON',async()=>{
 const {default:Recovery}=await vite.ssrLoadModule('/src/react-app/shared/WorkflowRecovery.tsx');
 const w={state:{session:{actorId:'fixture'}},services:{diagnosticsClient:{workflow:async()=>workflow,history:async()=>({items:[event]})}}};
 const v=await mountReact(vite,Recovery,{workspace:w,props:{workflowId:task.workflow_id}});
 try{
  await v.click(button(v.host,'执行历史与检查点'));await v.click(button(v.host,'读取执行历史'));await v.click(button(v.host,'事件详情'));
  assert.deepEqual(JSON.parse(v.host.querySelector('pre').textContent),event.details);
  await v.click(button(v.host,'管理命令与技术详情'));await v.click(button(v.host,'原命令回执'));
  const records=[...v.host.querySelectorAll('pre')].map(node=>JSON.parse(node.textContent));assert.deepEqual(records,[event.details,command,workflow]);
 }finally{await v.dispose();}
});

test('task model errors and raw model records stay readable and download-only',async()=>{
 const {default:Dialog}=await vite.ssrLoadModule('/src/react-app/shared/TaskRecordDialog.tsx');
 const record={ref:'private-model-record',data:{id:'private-model-id',sequence:1,status:'failed',prompt:0,response:false,error:{message:'sql: no rows in result set'},error_code:'internal_database_error'}};
 const state=proxy({session:{actorId:'fixture'},characters:[]});
 const w={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),navigatePath(){},services:{operationsClient:{task:async()=>({task_id:'model-error-task',status:'failed',correlation_id:'model-error-correlation',steps:[]})},diagnosticsClient:{page:async()=>({items:[record],next_cursor:''})}}};
 const v=await mountReact(vite,Dialog,{workspace:w,props:{taskId:'model-error-task',onClose(){}}});
 const host=document.querySelector('[role=dialog]'),blobs=[],create=URL.createObjectURL,revoke=URL.revokeObjectURL,anchor=window.HTMLAnchorElement.prototype,click=anchor.click;
 URL.createObjectURL=blob=>{blobs.push(blob);return 'blob:model-record';};URL.revokeObjectURL=()=>{};anchor.click=function(){};
 const noRaw=()=>{assert.doesNotMatch(host.textContent,/sql: no rows|private-model-record|private-model-id|internal_database_error/);assert.equal(host.querySelector('pre,code'),null);};
 try{
  await tab(v,host,'模型调用');assert.equal(host.querySelector('.replica-task-round-trigger').getAttribute('aria-expanded'),'true');noRaw();
  const raw=label=>[...host.querySelectorAll('.replica-raw-content-trigger')].find(node=>node.textContent.startsWith(label));await v.click(raw('输入'));await v.click(raw('输出'));noRaw();
  const bodies=[...host.querySelectorAll('.replica-raw-content-body')].map(node=>node.textContent);assert.deepEqual(bodies,['0','否']);assert.match(host.querySelector('.replica-task-round-error').textContent,/未完成|失败|没有找到|未找到|不可用/);
  await v.click(button(host,'错误代码'));await v.click(button(host,'本轮原始记录'));noRaw();
  for(const node of [...host.querySelectorAll('button')].filter(node=>node.textContent==='下载原始记录'))await v.click(node);
  const values=await Promise.all(blobs.map(async blob=>JSON.parse(await blob.text())));
  assert.deepEqual(values.find(value=>value?.ref===record.ref),record);assert.ok(values.some(value=>value?.error_code===record.data.error_code));noRaw();
 }finally{URL.createObjectURL=create;URL.revokeObjectURL=revoke;anchor.click=click;await v.dispose();}
});

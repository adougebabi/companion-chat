import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {act,mountReact,installDOM} from './helpers/react-dom.mjs';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();env.restore();});
const load=path=>vite.ssrLoadModule('/src/'+path),button=(host,text)=>[...host.querySelectorAll('button')].find(n=>n.textContent.trim()===text);
const call=(ref,data)=>({ref,kind:'model-runs',data});
function workspace(page){const state=proxy({session:{actorId:'calls-owner'},characters:[]});return {state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),services:{diagnosticsClient:{page}},navigatePath(){}};}
const task={task_id:'original-task',correlation_id:'exact/correlation?1',title:'初始衣柜准备',status:'failed',runs:2,created_at:'2026-10-30T01:00:00Z',steps:[{name:'workflow_start',event_type:'lifecycle.started'}]};
const payload={prompt:{messages:[{role:'system',content:'完整运行规则'},{role:'assistant',content:null,tool_calls:[{id:'physical-tool',function:{name:'inventory_lookup',arguments:'{"count":0}'}}]}],extra:{enabled:false,zero:0,nothing:null,list:[]}},response:{choices:[{message:{content:null,tool_calls:[{id:'response-tool',function:{name:'wear',arguments:'{"confirm":false}'}}]}}],extension:{kept:true}},status:'error',error_code:'provider_exact_code',error:{message:'精确调用错误',nested:{retry:false}},model_id:'actual-model'};
test('task dialog is wide, has the actual title, and defaults to full stored Request, Response and call errors',async()=>{
 const {default:Dialog}=await load('react-app/shared/TaskRecordDialog.tsx'),reads=[],w=workspace(async(kind,options)=>{reads.push([kind,options]);return {items:[call('physical',payload)],next_cursor:''};});
 w.services.operationsClient={task:async()=>task};let closed=false;
 const v=await mountReact(vite,Dialog,{workspace:w,props:{taskId:task.task_id,onClose:()=>closed=true}});
 try{const dialog=document.querySelector('[role=dialog]');assert.ok(dialog.classList.contains('replica-dialog-wide'));assert.equal(dialog.querySelector('[data-slot=dialog-title]').textContent,task.title);assert.equal(dialog.querySelectorAll('h3').length,1);assert.match(dialog.querySelector('[data-slot=dialog-description]').textContent,/original-task/);assert.equal(dialog.querySelector('[role=tab][data-state=active]').textContent,'执行记录');assert.equal(dialog.querySelector('[data-slot=card]'),null);
 const values=[...dialog.querySelectorAll('.replica-task-wire-value')].map(n=>n.textContent);assert.ok(values.includes(JSON.stringify(payload.prompt,null,2)));assert.ok(values.includes(JSON.stringify(payload.response,null,2)));assert.match(dialog.textContent,/provider_exact_code/);assert.match(dialog.textContent,/精确调用错误/);assert.doesNotMatch(dialog.textContent,/workflow_start/);assert.deepEqual(reads,[['model-runs',{correlationId:task.correlation_id,limit:20}]]);
 await act(async()=>dialog.dispatchEvent(new window.KeyboardEvent('keydown',{key:'Escape',bubbles:true})));await v.flush();assert.equal(closed,true);
 }finally{await v.dispose();}
});
test('model calls retry the original correlated cursor and retain confirmed rows on read-more failure',async()=>{
 const {default:Calls}=await load('react-app/shared/TaskModelCalls.tsx'),reads=[];let fail=true;
 const w=workspace(async(kind,options)=>{reads.push([kind,{...options}]);if(options.cursor&&fail)throw Error('fetch failed');return {items:[call(options.cursor?'second':'first',{prompt:options.cursor?'second-request':'confirmed-request',response:{count:0},status:'completed'})],next_cursor:options.cursor?'':'original-cursor'};});
 const v=await mountReact(vite,Calls,{workspace:w,props:{task}});
 try{await v.click(button(v.host,'加载更多模型调用'));assert.match(v.host.textContent,/confirmed-request/);assert.match(v.host.textContent,/模型调用读取失败/);assert.doesNotMatch(v.host.textContent,/暂未读到模型调用记录/);fail=false;await v.click(button(v.host,'重试原读取'));assert.match(v.host.textContent,/confirmed-request/);assert.match(v.host.textContent,/second-request/);assert.deepEqual(reads.map(([,o])=>o),[{correlationId:task.correlation_id,limit:20},{correlationId:task.correlation_id,limit:20,cursor:'original-cursor'},{correlationId:task.correlation_id,limit:20,cursor:'original-cursor'}]);assert.equal(button(v.host,'加载更多模型调用'),undefined);
 }finally{await v.dispose();}
});
test('missing association performs no global read; absent wire fields, null/empty stored values and empty results stay distinct',async()=>{
 const {default:Calls}=await load('react-app/shared/TaskModelCalls.tsx');let reads=0;
 const w=workspace(async()=>{reads++;return {items:[call('missing',{}),call('null',{prompt:null,response:[]}),call('raw',{prompt:'{"raw":  false}',response:''})],next_cursor:''};});
 const v=await mountReact(vite,Calls,{workspace:w,props:{task:{task_id:'unlinked'}}});
 try{assert.equal(reads,0);assert.match(v.host.textContent,/未提供模型调用关联标识/);await v.update({task});assert.equal(reads,1);assert.match(v.host.textContent,/未记录 Request/);assert.match(v.host.textContent,/未记录 Response/);const wire=[...v.host.querySelectorAll('pre')].map(n=>n.textContent);assert.deepEqual(wire,['null','[]','{"raw":  false}','']);
 await v.update({task:{...task,task_id:'empty',correlation_id:'empty'}});assert.equal(reads,2);
 }finally{await v.dispose();}
 const empty=await mountReact(vite,Calls,{workspace:workspace(async()=>({items:[],next_cursor:''})),props:{task}});try{assert.match(empty.host.textContent,/暂未读到模型调用记录/);assert.match(empty.host.textContent,/不能据此判断模型是否已被调用/);assert.doesNotMatch(empty.host.textContent,/读取失败/);}finally{await empty.dispose();}
});
test('initial read failure has its own retry and does not imply missing Request or Response',async()=>{
 const {default:Calls}=await load('react-app/shared/TaskModelCalls.tsx');let fail=true;
 const v=await mountReact(vite,Calls,{workspace:workspace(async()=>{if(fail)throw Error('fetch failed');return {items:[call('ok',{prompt:'after-retry',response:false})],next_cursor:''};}),props:{task}});
 try{assert.match(v.host.textContent,/模型调用读取失败/);assert.doesNotMatch(v.host.textContent,/未记录 Request|未记录 Response|暂未读到模型调用记录/);fail=false;await v.click(button(v.host,'重试原读取'));assert.match(v.host.textContent,/after-retry/);assert.equal(v.host.querySelectorAll('pre')[1].textContent,'false');}finally{await v.dispose();}
});
test('late task/correlation and Owner reads cannot replace the current model records; unmount retires reads',async()=>{
 const {default:Calls}=await load('react-app/shared/TaskModelCalls.tsx'),pending=[],reads=[];
 const w=workspace((kind,options)=>{reads.push([w.state.session.actorId,kind,options]);return new Promise(resolve=>pending.push(resolve));});
 const v=await mountReact(vite,Calls,{workspace:w,props:{task}});
 try{await v.update({task:{...task,task_id:'next-task',correlation_id:'next-correlation'}});await act(async()=>pending[0]({items:[call('old',{prompt:'STALE TASK'})],next_cursor:'stale-cursor'}));await v.flush();assert.doesNotMatch(v.host.textContent,/STALE TASK/);
 await act(async()=>w.state.session.actorId='next-owner');await v.flush();await act(async()=>pending[1]({items:[call('old-owner',{prompt:'STALE OWNER'})],next_cursor:''}));await v.flush();assert.doesNotMatch(v.host.textContent,/STALE OWNER/);await act(async()=>pending[2]({items:[call('new',{prompt:'CURRENT OWNER',response:0})],next_cursor:''}));await v.flush();assert.match(v.host.textContent,/CURRENT OWNER/);assert.deepEqual(reads.map(r=>r[2].correlationId),[task.correlation_id,'next-correlation','next-correlation']);
 await v.update({task:{...task,task_id:'unmounted',correlation_id:'unmounted'}});
 }finally{await v.dispose();}
 await act(async()=>pending.at(-1)({items:[call('late',{prompt:'after unmount'})],next_cursor:''}));
});
test('task detail does not read an old task correlation under a new Owner or adopt a late task response',async()=>{
 const {default:Task}=await load('react-app/pages/TaskDetailPage.tsx'),reads=[],pending=[];
 const w=workspace(async(kind,options)=>{reads.push([w.state.session.actorId,kind,options.correlationId]);return {items:[call('row',{prompt:'confirmed-call'})],next_cursor:''};});
 w.services.operationsClient={task:id=>new Promise(resolve=>pending.push({id,resolve}))};
 const route=id=>({feature:{params:{taskId:id}},query:new URLSearchParams()});
 const v=await mountReact(vite,Task,{workspace:w,props:{route:route(task.task_id)}});
 try{await act(async()=>pending[0].resolve(task));await v.flush();assert.deepEqual(reads,[['calls-owner','model-runs',task.correlation_id]]);
 await act(async()=>w.state.session.actorId='new-owner');await v.flush();assert.equal(reads.length,1);assert.doesNotMatch(v.host.textContent,/confirmed-call/);
 await v.update({route:route('next-task')});await act(async()=>pending[1].resolve({...task,correlation_id:'STALE ASSOCIATION'}));await v.flush();assert.equal(reads.length,1);
 await act(async()=>pending[2].resolve({...task,task_id:'next-task',correlation_id:'new-association'}));await v.flush();assert.deepEqual(reads[1],['new-owner','model-runs','new-association']);
 }finally{await v.dispose();}
});

test('refresh reads the current call response and null error columns are not rendered as failures',async()=>{
 const {default:Calls}=await load('react-app/shared/TaskModelCalls.tsx'),reads=[];let response='queued-body';
 const w=workspace(async(kind,options)=>{reads.push([kind,options]);return {items:[call('same-call',{prompt:{messages:[]},response,status:'completed',error_code:null,error:null})],next_cursor:''};});
 const v=await mountReact(vite,Calls,{workspace:w,props:{task}});
 try{assert.doesNotMatch(v.host.textContent,/调用错误/);response={choices:[{message:{content:'confirmed-response'}}]};await v.click(button(v.host,'刷新调用记录'));assert.match(v.host.textContent,/confirmed-response/);assert.doesNotMatch(v.host.textContent,/queued-body/);assert.deepEqual(reads,[['model-runs',{correlationId:task.correlation_id,limit:20}],['model-runs',{correlationId:task.correlation_id,limit:20}]]);}finally{await v.dispose();}
});
test('production enum task title is translated with the actual associated character',async()=>{
 const {default:Dialog}=await load('react-app/shared/TaskRecordDialog.tsx'),w=workspace(async()=>({items:[],next_cursor:''}));
 w.state.characters=[{id:'actual-role',name:'温知意'}];w.services.operationsClient={task:async()=>({...task,title:'wardrobe.initialize',kind:'wardrobe.initialize',fluctlight_id:'actual-role'})};
 const v=await mountReact(vite,Dialog,{workspace:w,props:{taskId:task.task_id,onClose(){}}});
 try{assert.equal(document.querySelector('[data-slot=dialog-title]').textContent,'温知意 · 初始穿搭准备');}finally{await v.dispose();}
});

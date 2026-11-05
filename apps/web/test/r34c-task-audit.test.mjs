import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {act,installDOM,mountReact} from './helpers/react-dom.mjs';
import {taskStatusPresentation,taskRunningStaleAfterMs,worldEventClockLabel,readAttentionTasks} from '../src/approved-ui/task-presentation.mjs';

const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();env.restore();});
const now=Date.parse('2026-11-02T14:00:00Z'),old=new Date(now-taskRunningStaleAfterMs-1).toISOString();

test('R34C every running task type warns after 15 minutes even without a valid started_at and after a recent status refresh',()=>{
 for(const kind of ['visual.identity.prepare','schedule.current_day','persona.compile','wardrobe.initialize','cognition.processing','new.running.type','world.events.generate']){
  for(const status of ['started','running','processing','generating']){
   const task={kind,status,created_at:old,started_at:'invalid',updated_at:new Date(now).toISOString(),...(kind==='world.events.generate'?{payload:{cycle_status:'generating'}}:{})};
   const view=taskStatusPresentation(task,now);
   assert.equal(view.group,'running',kind+':'+status);assert.equal(view.stale,true,kind+':'+status);assert.match(view.warning,/超过 15 分钟/);
   assert.equal(taskStatusPresentation({...task,status:'completed'},now).stale,false);
  }
 }
});
test('R34C world clock uses the original actual start and labels its hourly slot without changing either instant',()=>{
 assert.equal(worldEventClockLabel('2026-11-02T12:39:00Z'),'20:00 整点（20:39 开始）');
 assert.equal(worldEventClockLabel('2026-11-02T12:39:00Z','2026-11-02T12:00:00Z'),'20:00 整点（20:39 开始）');
 assert.equal(worldEventClockLabel('invalid'),'未记录时间');
});
test('R34C pending task source reads all original pages and rejects failed or repeating cursors',async()=>{
 const cursors=[];
 const api={pending:async(_fid,kind,cursor)=>{cursors.push([kind,cursor]);return {groups:[{kind,status:'ok',items:[{id:cursor?'request:b':'workflow:a',source_id:cursor?'b':'a',title:'schedule.current_day',status:'failed',href:cursor?'#/diag/request/b':'#/diag/task/a'}],next_cursor:cursor?'':'opaque/+?cursor'}]};},task:async id=>({task_id:id,kind:'schedule.current_day',status:'failed'})};
 const page=await readAttentionTasks(api);assert.equal(page.items.length,2);assert.deepEqual(cursors,[['failed_task',''],['failed_task','opaque/+?cursor']]);assert.equal(page.items[1].sourceHref,'#/diag/request/b');
 await assert.rejects(()=>readAttentionTasks({pending:async()=>({groups:[{kind:'failed_task',status:'failed',message:'原来源读取失败'}]})}),/原来源读取失败/);
 await assert.rejects(()=>readAttentionTasks({pending:async()=>({groups:[{kind:'failed_task',status:'ok',items:[],next_cursor:'repeat'}]})}),/分页重复/);
});
test('R34C unavailable task detail and concurrent recovery retain the authoritative pending snapshot and count',async()=>{
 const api={pending:async()=>({groups:[{kind:'failed_task',status:'ok',count:2,items:[{id:'workflow:a',source_id:'a',title:'visual.identity.prepare',status:'failed',href:'#/diag/task/a'},{id:'workflow:b',source_id:'b',title:'schedule.current_day',status:'paused',href:'#/diag/task/b'}]}]}),task:async id=>{if(id==='a')throw Error('详情连接暂不可用');return {task_id:id,kind:'schedule.current_day',status:'completed'};}};
 const page=await readAttentionTasks(api);
 assert.equal(page.items.length,2);assert.equal(page.detail_errors,1);assert.deepEqual(page.items.map(row=>row.data.status),['failed','paused']);assert.match(page.items[0].data.task_detail_error,/详情连接暂不可用/);assert.equal(page.items[0].sourceHref,'#/diag/task/a');
 assert.equal(page.items.filter(row=>taskStatusPresentation(row.data).group==='attention').length,2);
});
test('R34C diagnostics attention matches notification source across pages, removes obsolete failures and filters by world',async()=>{
 localStorage.clear();sessionStorage.clear();location.hash='#/diag';
 const {default:Host}=await vite.ssrLoadModule('/src/react-app/shared/DiagnosticsWorkspace.jsx');
 const state=proxy({session:{actorId:'r34c-tasks-owner'},characters:[]}),reads=[];
 const pendingRows=[{id:'workflow:older',source_id:'older',title:'visual.identity.prepare',status:'failed',href:'#/diag/task/older'},{id:'workflow:paused',source_id:'paused',title:'schedule.current_day',status:'paused',href:'#/diag/task/paused'}];
 const recent=[{ref:'obsolete',data:{intent_id:'obsolete',kind:'wardrobe.initialize',status:'failed'}},{ref:'complete',data:{intent_id:'complete',kind:'persona.compile',status:'completed'}},{ref:'clock',occurred_at:'2026-11-02T12:39:00Z',data:{intent_id:'clock',intent_type:'world.events.generate',status:'waiting',world_id:'A',world_name:'临海世界',created_at:'2026-11-02T12:39:00Z'}},...['visual.identity.prepare','schedule.current_day'].map(kind=>({ref:kind,data:{intent_id:kind,intent_type:kind,status:'running',created_at:old,updated_at:new Date().toISOString(),payload:{world_id:'B'},world_name:'山间世界'}}))];
 const api={pending:async(_fid,kind,cursor)=>({groups:[{kind,status:'ok',count:2,items:[pendingRows[cursor?1:0]],next_cursor:cursor?'':'original-page-2'}]}),task:async id=>({task_id:id,kind:id==='older'?'visual.identity.prepare':'schedule.current_day',status:id==='older'?'failed':'paused',created_at:old,source:{world_id:'A'}})};
 const w={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),navigatePath(){},services:{operationsClient:api,diagnosticsClient:{page:async(_kind,options)=>{reads.push(options);return {items:options.intentType?[]:recent,next_cursor:''};}}}};
 const v=await mountReact(vite,Host,{workspace:w,props:{view:{openTaskDiagnostics(){}}}});
 const attention=()=>Number(v.host.querySelector('[data-task-status-group=attention] strong').textContent);
 async function choose(label,text){await act(async()=>v.host.querySelector(`[role=combobox][aria-label="${label}"]`).dispatchEvent(new window.KeyboardEvent('keydown',{key:'ArrowDown',bubbles:true})));await v.flush();const option=[...document.querySelectorAll('[role=option]')].find(node=>node.textContent===text);assert.ok(option,text);await act(async()=>option.click());await v.flush();}
 try{
  assert.equal(attention(),2);assert.equal(v.host.querySelectorAll('.replica-task-row[data-task-status-group=attention]').length,2);assert.equal(v.host.querySelectorAll('.replica-task-row').length,6);assert.equal([...v.host.querySelectorAll('.replica-task-row')].filter(row=>row.textContent.includes('运行超过 15 分钟')).length,2);assert.match(v.host.textContent,/与通知服务异常同源/);assert.doesNotMatch(v.host.textContent,/初始穿搭准备/);assert.match(v.host.textContent,/20:00 整点（20:39 开始）/);
  await choose('任务世界','临海世界');assert.equal(attention(),2);assert.equal(v.host.querySelectorAll('.replica-task-row').length,3);assert.ok(reads.some(row=>row.worldId==='A'));
  await choose('任务状态','需要处理');assert.equal(v.host.querySelectorAll('.replica-task-row').length,2);assert.equal(attention(),2);
 }finally{await v.dispose();}
 const {default:Panel}=await vite.ssrLoadModule('/src/react-app/shared/WorldAutomaticEvents.jsx');
 w.state.selectedId='role';w.current={value:{id:'role'}};w.isCurrent=()=>true;w.client={ownerDiagnosticRecords:async kind=>({items:kind==='workflow-intents'?[{ref:'clock',data:{...recent[2].data,payload:{world_id:'A'}}}]:[{ref:'generation',data:{world_id:'A',status:'no_event',slot_at:'2026-11-02T12:00:00Z',created_at:'2026-11-02T12:39:00Z'}}],next_cursor:''})};
 const panel=await mountReact(vite,Panel,{workspace:w,props:{model:{w,world:{id:'A'}}}});
 try{await panel.click([...panel.host.querySelectorAll('button')].find(node=>node.textContent.trim()==='自动事件与运行记录'));assert.equal((panel.host.textContent.match(/20:00 整点（20:39 开始）/g)||[]).length,2);}finally{await panel.dispose();}
});
test('R34C late pending-source results cannot leak across Owner changes',async()=>{
 localStorage.clear();sessionStorage.clear();location.hash='#/diag';
 const {default:Host}=await vite.ssrLoadModule('/src/react-app/shared/DiagnosticsWorkspace.jsx');
 const state=proxy({session:{actorId:'r34c-old-owner'},characters:[]});let resolveOld;
 const oldPage=new Promise(resolve=>resolveOld=resolve);
 const w={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),navigatePath(){},services:{operationsClient:{pending:async()=>state.session.actorId==='r34c-old-owner'?oldPage:{groups:[{kind:'failed_task',status:'ok',count:0,items:[],next_cursor:''}]}},diagnosticsClient:{page:async()=>({items:[],next_cursor:''})}}};
 const v=await mountReact(vite,Host,{workspace:w,props:{view:{openTaskDiagnostics(){}}}});
 try{
  assert.equal(v.host.querySelector('[data-task-status-group=attention] strong').textContent,'—');assert.doesNotMatch(v.host.textContent,/暂时没有运行任务/);
  await act(async()=>state.session.actorId='r34c-new-owner');await v.flush();
  await act(async()=>resolveOld({groups:[{kind:'failed_task',status:'ok',count:1,items:[{id:'request:old',source_id:'old',title:'前一所有者的事项',status:'failed',href:'#/diag/request/old'}],next_cursor:''}]}));await v.flush();
  assert.doesNotMatch(v.host.textContent,/前一所有者的事项/);assert.equal(v.host.querySelector('[data-task-status-group=attention] strong').textContent,'0');
 }finally{await v.dispose();}
});

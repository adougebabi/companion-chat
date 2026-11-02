import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {act,installDOM,mountReact} from './helpers/react-dom.mjs';
import {taskStatusPresentation,taskStatusSummary,taskRunningStaleAfterMs} from '../src/approved-ui/task-presentation.mjs';

const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();env.restore();});
const now=Date.parse('2026-11-02T13:20:00Z');
test('D7 one taxonomy includes started, every unknown status, waiting, and terminal outcomes without dropping rows',()=>{
 const samples=[['started','running'],['running','running'],['processing','running'],['failed','attention'],['paused','attention'],['retry','waiting'],['pending','waiting'],['completed','completed'],['succeeded','completed'],['cancelled','ended'],['superseded','ended'],['provider_new_status','unknown'],['','unknown'],['__proto__','unknown']];
 const rows=samples.map(([status,expected])=>{const presentation=taskStatusPresentation({status},now);assert.equal(presentation.group,expected,status);return {presentation};});
 assert.equal(taskStatusSummary(rows).reduce((sum,group)=>sum+group.count,0),samples.length);
 assert.equal(taskStatusPresentation({status:'started'},now).label,'进行中');
 assert.equal(taskStatusPresentation({status:'waiting',kind:'persona.compile'},now).label,'等待中');
 const started_at=new Date(now-taskRunningStaleAfterMs).toISOString();
 assert.equal(taskStatusPresentation({status:'started',started_at},now).stale,false);
 assert.equal(taskStatusPresentation({status:'started',started_at},now+1).stale,true);
 assert.equal(taskStatusPresentation({status:'completed',started_at},now+1).stale,false);
 assert.equal(taskStatusPresentation({status:'started',started_at:'invalid'},now).stale,false);
});
test('D7 world clock presents actual hourly generation separately from a long-lived started workflow',()=>{
 const task={intent_type:'world.events.generate',status:'started',started_at:'2026-10-01T00:00:00Z'};
 const generating=taskStatusPresentation({...task,payload:{cycle_status:'generating',cycle_started_at:'2026-11-02T13:19:00Z'}},now);
 assert.equal(generating.label,'生成中');assert.equal(generating.group,'running');assert.equal(generating.stale,false);
 const waiting=taskStatusPresentation({...task,payload:{cycle_status:'no_event',cycle_finished_at:'2026-11-02T13:01:00Z'}},now);
 assert.equal(waiting.label,'等待下一小时');assert.equal(waiting.group,'waiting');assert.equal(waiting.stale,false);
 assert.equal(taskStatusPresentation({...task,status:'waiting',last_cycle_status:'no_event',next_cycle_at:'2026-11-02T14:00:00Z'},now).label,'等待下一小时');
 assert.equal(taskStatusPresentation({...task,status:'generating'},now).label,'生成中');
 assert.equal(taskStatusPresentation(task,now).group,'unknown','workflow started alone cannot establish the active hourly phase');
});
test('D7 rendered audit sample has 20 rows: seven running, six unknown, five completed, one failed and one waiting',async()=>{
 localStorage.clear();sessionStorage.clear();location.hash='#/diag';
 const {default:Host}=await vite.ssrLoadModule('/src/react-app/shared/DiagnosticsWorkspace.jsx');
 const recent=new Date(Date.now()-60000).toISOString(),old=new Date(Date.now()-taskRunningStaleAfterMs-60000).toISOString();
 const statuses=[...Array(7).fill('started'),...Array(6).fill('unrecognized_small_model_state'),...Array(5).fill('completed'),'failed','pending'];
 const records=statuses.map((status,index)=>({ref:'audit-'+index,data:{intent_id:'task-'+index,intent_type:'cognition.processing',status,started_at:index?recent:old}}));
 const state=proxy({session:{actorId:'d7-taxonomy-owner'},characters:[]});
 const w={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),navigatePath(){},services:{diagnosticsClient:{page:async(_kind,options)=>({items:options.intentType?[]:records,next_cursor:''})}}};
 const v=await mountReact(vite,Host,{workspace:w,props:{view:{openTaskDiagnostics(){}}}});
 const counts=()=>Object.fromEntries([...v.host.querySelectorAll('.replica-diagnostic-metric')].map(node=>[node.dataset.taskStatusGroup,Number(node.querySelector('strong').textContent)]));
 const consistent=()=>{const values=counts(),total=Object.values(values).reduce((a,b)=>a+b,0);assert.equal(total,v.host.querySelectorAll('.replica-task-row').length);assert.equal(Number(v.host.querySelector('.replica-diagnostic-count').textContent),total);return values;};
 async function choose(label){await act(async()=>v.host.querySelector('[aria-label="任务状态"][role=combobox]').dispatchEvent(new window.KeyboardEvent('keydown',{key:'ArrowDown',bubbles:true})));await v.flush();const option=[...document.querySelectorAll('[role=option]')].find(node=>node.textContent===label);assert.ok(option);await act(async()=>option.click());await v.flush();}
 try{
  assert.deepEqual(consistent(),{attention:1,running:7,waiting:1,completed:5,ended:0,unknown:6});
  assert.equal([...v.host.querySelectorAll('.replica-task-row')].filter(row=>row.textContent.includes('运行超过 15 分钟')).length,1);
  assert.equal(v.host.querySelectorAll('.replica-task-row[data-task-status-group=running] .replica-diagnostic-status').length,7);
  await choose('状态待确认');assert.equal(consistent().unknown,6);assert.equal(v.host.querySelectorAll('.replica-task-row').length,6);
  await choose('正在执行');assert.equal(consistent().running,7);assert.equal(v.host.querySelectorAll('.replica-task-row').length,7);
  await choose('所有状态');assert.equal(v.host.querySelectorAll('.replica-task-row').length,20);consistent();
 }finally{await v.dispose();}
});
test('D7 world event page and management rows render the same generation and waiting clock labels',async()=>{
 localStorage.clear();sessionStorage.clear();location.hash='#/diag';
 const records=[{ref:'generating-clock',data:{intent_id:'clock-generating',intent_type:'world.events.generate',status:'started',payload:{world_id:'world-A',cycle_status:'generating',cycle_started_at:new Date().toISOString()}}},{ref:'waiting-clock',data:{intent_id:'clock-waiting',intent_type:'world.events.generate',status:'waiting',world_id:'world-A',last_cycle_status:'no_event',next_cycle_at:'2026-11-02T14:00:00Z'}}];
 const state=proxy({session:{actorId:'d7-world-clock-owner'},selectedId:'A',characters:[{id:'A',name:'青梧'}]});
 const w={state,current:{get value(){return state.characters[0];}},subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),isCurrent:(owner,fid)=>owner===state.session.actorId&&fid===state.selectedId,navigatePath(){},services:{diagnosticsClient:{page:async()=>({items:records,next_cursor:''})}},client:{ownerDiagnosticRecords:async kind=>({items:kind==='workflow-intents'?records:[],next_cursor:''})}};
 const {default:Host}=await vite.ssrLoadModule('/src/react-app/shared/DiagnosticsWorkspace.jsx');
 let v=await mountReact(vite,Host,{workspace:w,props:{view:{openTaskDiagnostics(){}}}});
 try{assert.deepEqual([...v.host.querySelectorAll('.replica-task-row .replica-diagnostic-status')].map(node=>node.textContent),['生成中','等待下一小时']);}finally{await v.dispose();}
 const {default:Panel}=await vite.ssrLoadModule('/src/react-app/shared/WorldAutomaticEvents.jsx');
 v=await mountReact(vite,Panel,{workspace:w,props:{model:{w,fid:'A',world:{id:'world-A'}}}});
 try{assert.deepEqual([...v.host.querySelectorAll('[data-slot=badge]')].map(node=>node.textContent),['生成中','等待下一小时']);}finally{await v.dispose();}
});

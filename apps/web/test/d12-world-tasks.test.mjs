import test from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'vite';
import { fileURLToPath } from 'node:url';
import { proxy, snapshot, subscribe } from 'valtio/vanilla';
import { act, installDOM, mountReact } from './helpers/react-dom.mjs';
const env = installDOM(), vite = await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const load = path => vite.ssrLoadModule('/src/react-app/shared/' + path);
const button = (root,label) => [...root.querySelectorAll('button')].find(n=>n.textContent.trim()===label);
test.after(async()=>{await vite.close();env.restore();});
let sequence=0;
function workspace(){
 sessionStorage.clear(); localStorage.clear(); location.hash='#/diag';
 const state=proxy({session:{actorId:'d12-owner-'+(++sequence)},selectedId:'A',characters:[{id:'A',name:'青梧'}]});
 return {state,current:{get value(){return state.characters[0];}},subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),isCurrent:(owner,fid)=>owner===state.session?.actorId&&fid===state.selectedId,services:{},client:{},navigatePath(path){this.navigated=path;}};
}
const clock={ref:'world-clock-ref',occurred_at:'2026-10-01T00:00:00Z',data:{intent_id:'world_event_generation:world-A',intent_type:'world.events.generate',world_name:'临海世界',status:'waiting',last_cycle_status:'no_event',next_cycle_at:'2026-11-02T09:00:00Z',payload:{world_id:'world-A'}}};
async function choose(v,label,text){
 await act(async()=>v.host.querySelector(`[role=combobox][aria-label="${label}"]`).dispatchEvent(new window.KeyboardEvent('keydown',{key:'ArrowDown',bubbles:true})));
 await v.flush(); const option=[...document.querySelectorAll('[role=option]')].find(n=>n.textContent===text); assert.ok(option);
 await act(async()=>option.click()); await v.flush();
}
test('D12 task list includes an old world clock beyond 20 recent tasks and filters its actual kind on the server',async()=>{
 const {default:Host}=await load('DiagnosticsWorkspace.jsx'),w=workspace(),reads=[];
 const recent=Array.from({length:20},(_,i)=>({ref:'recent-'+i,data:{intent_id:'task-'+i,intent_type:'cognition.processing',status:'completed'}}));
 w.services.diagnosticsClient={page:async(kind,options)=>{reads.push({kind,...options});return {items:options.intentType?[clock]:recent,next_cursor:''};}};
 const v=await mountReact(vite,Host,{workspace:w,props:{view:{openTaskDiagnostics(){}}}});
 try{
  assert.equal(v.host.querySelectorAll('.replica-task-row').length,21);
  assert.match(v.host.textContent,/世界事件时钟/);assert.match(v.host.textContent,/上一轮：本小时无事件/);
  assert.ok(reads.some(r=>r.intentType==='world.events.generate'&&!r.fluctlightId));
  await v.input(v.host.querySelector('[aria-label="搜索任务或角色"]'),'临海世界');assert.equal(v.host.querySelectorAll('.replica-task-row').length,1);
  await v.input(v.host.querySelector('[aria-label="搜索任务或角色"]'),'');
  await choose(v,'任务类型','世界事件时钟');
  assert.equal(v.host.querySelectorAll('.replica-task-row').length,1); assert.equal(reads.at(-1).intentType,'world.events.generate');
  await v.click(v.host.querySelector('.replica-task-row'));
  assert.equal(new URLSearchParams(w.navigated.split('?')[1]).get('taskId'),'world_event_generation:world-A');
 }finally{await v.dispose();}
});
test('D12 world and Owner changes discard pending automatic-event reads',async()=>{
 const {default:Panel}=await load('WorldAutomaticEvents.jsx'),w=workspace();let resolve;
 const oldPage=new Promise(r=>resolve=r);
 w.client.ownerDiagnosticRecords=async(kind,options)=>options.worldId==='world-A'?oldPage:{items:[{ref:'fresh-'+kind,data:kind==='workflow-intents'?{...clock.data,payload:{world_id:'world-B'}}:{world_id:'world-B',title:'新世界的事件',status:'rejected',slot_at:'2026-11-02T08:00:00Z'}}],next_cursor:''};
 const v=await mountReact(vite,Panel,{workspace:w,props:{model:{w,fid:'A',world:{id:'world-A'}}}});
 try{
  await v.update({model:{w,fid:'A',world:{id:'world-B'}}});assert.match(v.host.textContent,/新世界的事件/);
  await act(async()=>resolve({items:[{ref:'obsolete',data:{world_id:'world-A',title:'过期世界记录'}}],next_cursor:''}));await v.flush();
  assert.doesNotMatch(v.host.textContent,/过期世界记录/);assert.match(v.host.textContent,/新世界的事件/);
  let resolveOwner;const oldOwner=w.state.session.actorId,oldOwnerPage=new Promise(r=>resolveOwner=r);
  w.client.ownerDiagnosticRecords=async()=>w.state.session.actorId===oldOwner?oldOwnerPage:{items:[],next_cursor:''};
  await v.click(button(v.host,'刷新自动事件'));
  await act(async()=>w.state.session.actorId='another-owner');await v.flush();
  await act(async()=>resolveOwner({items:[{ref:'old-owner',data:{world_id:'world-B',title:'前一会话的记录'}}],next_cursor:''}));await v.flush();
  assert.doesNotMatch(v.host.textContent,/过期世界记录|前一会话的记录|新世界的事件/);
 }finally{await v.dispose();}
});
test('D12 task reads deduplicate clocks, retain separate world pagination, and retry a failed original page',async()=>{
 const {default:Host}=await load('DiagnosticsWorkspace.jsx'),w=workspace(),reads=[];let fail=true;
 const older={...clock,ref:'world-B',data:{...clock.data,intent_id:'world_event_generation:world-B'}};
 w.services.diagnosticsClient={page:async(kind,options)=>{reads.push({...options});if(options.intentType&&options.cursor&&fail)throw Error('世界任务暂不可读');return {items:options.cursor?[older]:[clock],next_cursor:options.intentType&&!options.cursor?'opaque/world+cursor':''};}};
 const v=await mountReact(vite,Host,{workspace:w,props:{view:{openTaskDiagnostics(){}}}});
 try{
  assert.equal(v.host.querySelectorAll('.replica-task-row').length,1);
  await v.click(button(v.host,'读取更多世界事件任务'));assert.match(v.host.textContent,/世界任务暂不可读/);
  fail=false;await v.click(button(v.host,'重新读取世界事件任务'));
  assert.equal(reads.at(-1).cursor,'opaque/world+cursor');assert.equal(reads.at(-1).intentType,'world.events.generate');
  assert.equal(v.host.querySelectorAll('.replica-task-row').length,2);
 }finally{await v.dispose();}
});
test('D12 automatic event panel preserves scope, shows valid empty states and retries without a mutation',async()=>{
 const {default:Panel}=await load('WorldAutomaticEvents.jsx'),w=workspace(),reads=[];let wrong=true;
 w.client.ownerDiagnosticRecords=async(kind,options)=>{reads.push({kind,...options});return {items:kind==='workflow-intents'?[clock]:wrong?[{ref:'wrong',data:{world_id:'another-world',title:'禁止显示的其他世界'}}]:[],next_cursor:''};};
 const v=await mountReact(vite,Panel,{workspace:w,props:{model:{w,fid:'A',world:{id:'world-A'}}}});
 try{
  assert.match(v.host.textContent,/自动世界事件记录范围不匹配/);assert.doesNotMatch(v.host.textContent,/禁止显示的其他世界|尚无自动事件记录/);
  wrong=false;await v.click(button(v.host,'重新读取自动事件记录'));
  assert.ok(v.host.querySelector('[data-slot=empty]'));assert.match(v.host.textContent,/无需先创建事件模板/);
  assert.ok(reads.every(r=>r.worldId==='world-A'&&!r.fluctlightId));
  await v.click(button(v.host,'查看生成任务'));assert.match(w.navigated,/taskId=world_event_generation%3Aworld-A/);
 }finally{await v.dispose();}
});

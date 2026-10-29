import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {installDOM,mountReact} from './helpers/react-dom.mjs';
const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const {default:Inbox}=await vite.ssrLoadModule('/src/react-app/pages/InboxPage.jsx');
test.after(async()=>{await vite.close();env.restore();});
const button=(root,text)=>[...root.querySelectorAll('button')].find(b=>b.textContent.trim()===text);
function fixture(){
 const state=proxy({session:{actorId:'owner'},characters:[{id:'A',name:'林岚'}],selectedId:'A',notifications:[]}),calls=[];
 let failMore=true,failGroup=true;
 const item=id=>({id,title:'通知'+id,body:'实际正文'+id,fluctlight_id:'A',link:'/diag/task/task-'+id,created_at:'2026-10-27T00:00:00Z'});
 const groups=['capability','persona_proposal','studio_apply','failed_task','owner_request'];
 const api={notifications:async(cursor,unread,fid)=>{calls.push(['notices',cursor,fid]);if(cursor&&failMore){failMore=false;throw Error('分页暂不可读');}return {items:[fid?{...item('scoped-'+fid),fluctlight_id:fid}:item(cursor?'two':'one')],as_of:'server-as-of',next_cursor:cursor?'':'notice-cursor'};},pending:async(fid,kind,cursor)=>{calls.push(['pending',fid,kind,cursor]);if(kind==='capability'&&!cursor&&failGroup){failGroup=false;return {groups:[{kind,status:'ok',count:1,items:[{id:'p',title:'实际提议',href:'/diag/request/p',status:'pending'}],next_cursor:'pending-cursor'}]};}return {groups:(kind?[kind]:groups).map(kind=>({kind,status:kind==='capability'&&!cursor?'failed':'ok',message:'来源读取失败',count:kind==='capability'?1:0,items:[],next_cursor:''}))};},readNotification:async id=>{calls.push(['read',id]);},readAllNotifications:async before=>calls.push(['all',before])};
 api.task=async id=>{calls.push(['task',id]);return {task_id:id,fluctlight_id:'A',kind:'goal.plan',status:'completed',steps:[]};};
 const w={state,markNotificationRead:api.readNotification,markAllNotificationsRead:api.readAllNotifications,refreshNotificationCount:async()=>{},current:{value:{id:'A',name:'林岚'}},client:{},services:{operationsClient:api},subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),navigatePath:path=>calls.push(['go',path]),navigate:page=>calls.push(['page',page])};return {w,calls,api};
}
test('notification task opens in the same dialog and returns without another acknowledgement or navigation',async()=>{
 const {w,calls}=fixture(),ui=await mountReact(vite,Inbox,{workspace:w});
 try{
  const trigger=ui.host.querySelector('.replica-notice-main');
  await ui.click(trigger);assert.equal(document.querySelectorAll('[role=dialog]').length,1);
  await ui.click(button(document,'返回通知'));
  assert.ok(!document.querySelector('[role=dialog]'));assert.equal(document.activeElement,trigger);
  await ui.click(trigger);await ui.click(button(document,'查看原始来源'));
  assert.equal(document.querySelectorAll('[role=dialog]').length,1);
  assert.ok(document.querySelector('[role=dialog] [data-testid=task-detail]'));
  assert.deepEqual(calls.filter(c=>c[0]==='task'),[['task','task-one']]);
  assert.equal(calls.filter(c=>c[0]==='read').length,1);
  assert.deepEqual(calls.filter(c=>c[0]==='go'||c[0]==='page'),[]);
  assert.equal(ui.host.querySelectorAll('.replica-notice').length,1);
  await ui.click(button(document,'返回通知详情'));
  assert.match(document.querySelector('[role=dialog]').textContent,/实际正文one/);
  assert.ok(!document.querySelector('[role=dialog] [data-testid=task-detail]'));
  await ui.click(button(document,'返回通知'));
  assert.equal(document.activeElement,trigger);assert.equal(calls.filter(c=>c[0]==='read').length,1);
 }finally{await ui.dispose();}
});
test('each inbox reader retries its own failed source while preserving notification cursor and server read time',async()=>{
 const {w,calls}=fixture(),ui=await mountReact(vite,Inbox,{workspace:w});
 try{
  const group=ui.host.querySelector('.replica-pending-group');
  assert.match(group.textContent,/来源读取失败/);
  await ui.click(button(group,'重新读取通知来源'));assert.match(group.textContent,/实际提议/);
  assert.deepEqual(calls.filter(c=>c[0]==='notices').map(c=>c[1]),['']);
  await ui.click(button(ui.host,'读取更多通知'));
  const panel=ui.host.querySelector('.replica-notice-panel');
  assert.match(panel.textContent,/分页暂不可读/);assert.match(panel.textContent,/通知one/);
  const pendingReads=calls.filter(c=>c[0]==='pending').length;
  await ui.click(button(panel,'重新读取通知来源'));assert.match(panel.textContent,/通知two/);
  assert.equal(calls.filter(c=>c[0]==='pending').length,pendingReads);
  assert.deepEqual(calls.filter(c=>c[0]==='notices').map(c=>c[1]),['','notice-cursor','notice-cursor']);
  assert.ok(calls.some(c=>c[0]==='pending'&&c[2]==='capability'));
  await ui.click(button(ui.host,'全部标为已读'));assert.deepEqual(calls.find(c=>c[0]==='all'),['all','server-as-of']);
 }finally{await ui.dispose();}
});
test('inbox display filter survives returning from a source and clears with its Owner session',async()=>{const {w}=fixture();let ui=await mountReact(vite,Inbox,{workspace:w});try{const tab=[...ui.host.querySelectorAll('[role=tab]')].find(n=>n.textContent.includes('未读'));await (await import('react')).act(async()=>tab.dispatchEvent(new window.MouseEvent('mousedown',{bubbles:true,button:0})));await ui.flush();assert.equal(tab.getAttribute('aria-selected'),'true');}finally{await ui.dispose();}ui=await mountReact(vite,Inbox,{workspace:w});try{assert.equal([...ui.host.querySelectorAll('[role=tab]')].find(n=>n.textContent.includes('未读')).getAttribute('aria-selected'),'true');await (await import('react')).act(async()=>{w.state.session=null;});await ui.flush();}finally{await ui.dispose();}assert.equal(sessionStorage.getItem('fluctlight:management-view:owner:inbox'),null);});

test('role filtering starts its own server read and the global unread count is not the page length',async()=>{
 const {w,calls}=fixture();w.state.characters.push({id:'B',name:'季川'});w.state.notificationCount={count:61,alert_count:4};
 const ui=await mountReact(vite,Inbox,{workspace:w});
 try{
  assert.match([...ui.host.querySelectorAll('[role=tab]')].find(n=>n.textContent.includes('未读')).textContent,/61/);
  await ui.click(ui.host.querySelector('[aria-label="通知角色范围"]'));
  await ui.click([...document.querySelectorAll('[role=option]')].find(n=>n.textContent.trim()==='季川'));
  assert.ok(calls.some(c=>c[0]==='notices'&&c[2]==='B'));
  assert.match(ui.host.textContent,/通知scoped-B/);assert.doesNotMatch(ui.host.textContent,/实际正文one/);
  assert.match(ui.host.textContent,/包含其他角色/);
 }finally{await ui.dispose();}
});
test('unrecognized notification source names retain their content and a usable detail',async()=>{
 sessionStorage.clear();const {w,api}=fixture();api.notifications=async()=>({items:['constructor','__proto__'].map((source_kind,index)=>({id:'unknown-'+index,title:'原来源 '+source_kind,body:'保留这条实际通知正文',source_kind,fluctlight_id:'A',link:'/diag/task/task-one',created_at:'2026-10-27T00:00:00Z'})),as_of:'server-as-of',next_cursor:''});
 const ui=await mountReact(vite,Inbox,{workspace:w});try{assert.equal(ui.host.querySelectorAll('.replica-notice').length,2);await ui.click(ui.host.querySelectorAll('.replica-notice-main')[1]);assert.match(document.querySelector('[role=dialog]').textContent,/原来源 __proto__/);assert.match(document.querySelector('[role=dialog]').textContent,/保留这条实际通知正文/);await ui.click(button(document,'返回通知'));assert.equal(ui.host.querySelectorAll('.replica-notice').length,2);}finally{await ui.dispose();}
});

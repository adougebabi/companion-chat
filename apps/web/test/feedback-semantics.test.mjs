import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {installDOM,mountReact} from './helpers/react-dom.mjs';
import {act,React} from './helpers/react-dom.mjs';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
const environment=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const {Feedback,InlineFeedback,ResultFeedback}=await vite.ssrLoadModule('/src/react-app/shared/Presentation.tsx');
const {Button}=await vite.ssrLoadModule('/src/components/ui/button.tsx');
const {jsx}=await import('react/jsx-runtime');
test.after(async()=>{await new Promise(ok=>setTimeout(ok,25));await vite.close();environment.restore();});
test('business feedback preserves error emphasis while ordinary notices do not announce assertive failures',async()=>{
 for(const [kind,role] of [['information','note'],['action-required','note'],['warning','status'],['error','alert'],['success','status'],['progress','status']]){
  const view=await mountReact(vite,Feedback,{props:{kind,title:'当前业务状态',children:'真实状态说明'}});
  try{const node=view.host.querySelector('[data-feedback-kind]');assert.equal(node.getAttribute('role'),role);assert.equal(node.getAttribute('aria-live'),role==='alert'?'assertive':role==='status'?'polite':null);assert.match(node.textContent,/当前业务状态.*真实状态说明/s);}finally{await view.dispose();}
 }
 const view=await mountReact(vite,InlineFeedback,{props:{children:'实际读取失败'}});try{assert.equal(view.host.querySelector('[role=alert]').dataset.feedbackKind,'error');}finally{await view.dispose();}
});
test('recovery action executes only on explicit click and diagnostic disclosure stays separate from product copy',async()=>{
 let calls=0;const view=await mountReact(vite,Feedback,{props:{kind:'action-required',title:'有一份未完成资料',children:'上次输入已经保留。',actions:jsx(Button,{type:'button',onClick:()=>calls++,children:'继续创建'}),details:jsx('pre',{children:'原始诊断：storage_read_failed'})}});
 try{
  assert.equal(calls,0);assert.match(view.host.textContent,/上次输入已经保留/);assert.equal(view.host.querySelector('pre'),null);
  const buttons=()=>[...view.host.querySelectorAll('button')];await view.click(buttons().find(b=>b.textContent==='继续创建'));assert.equal(calls,1);
  await view.click(buttons().find(b=>b.textContent.includes('查看技术详情')));assert.match(view.host.querySelector('pre').textContent,/storage_read_failed/);
  await view.click(buttons().find(b=>b.textContent.includes('查看技术详情')));assert.equal(view.host.querySelector('pre'),null);assert.equal(calls,1);
 }finally{await view.dispose();}
});
test('Library adds valid historical progress links when the real role directory arrives after mounting',async()=>{
 const {default:Library}=await vite.ssrLoadModule('/src/react-app/pages/LibraryPage.jsx'),owner='late-directory-owner';
 localStorage.setItem(`fluctlight-creation:${owner}:history:late-role`,JSON.stringify({version:1,step:3,requestId:'original-history',activatedId:'late-role',mode:'blank',analysis:null}));
 const state=proxy({session:{actorId:owner},characters:[],selectedId:'',errors:{},loading:{}}),w={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),services:{configurationClient:{read:async()=>({bindings:[]})}},client:{},select(){},navigate(){}};
 const view=await mountReact(vite,Library,{workspace:w});
 try{
  assert.equal(view.host.querySelector('a[href*="resume="]'),null);
  await act(()=>{state.characters=[{id:'late-role',name:'后到目录的角色',status:'active',tags:[]}];});await view.flush();
  const link=view.host.querySelector('a[href="#/library/create?resume=late-role"]');assert.ok(link);assert.equal(link.textContent.trim(),'查看准备进度');assert.equal(JSON.parse(localStorage.getItem(`fluctlight-creation:${owner}:history:late-role`)).requestId,'original-history');
 }finally{await view.dispose();}
});
test('expiration of an earlier ordinary result cannot clear a newer feedback event',async()=>{
 const initial={kind:'success',text:'第一次保存已确认'},next={kind:'success',text:'第二次保存已确认'},timers=[],original=globalThis.setTimeout;
 globalThis.setTimeout=(handler,delay,...args)=>{if(delay===6000){timers.push(handler);return 0;}return original(handler,delay,...args);};
 function Harness(){const [notice,setNotice]=React.useState(initial);return React.createElement(React.Fragment,null,React.createElement(ResultFeedback,{notice,onDismiss:value=>setNotice(current=>current===value?null:current)}),React.createElement(Button,{onClick:()=>setNotice(next),children:'新的结果'}));}
 const view=await mountReact(vite,Harness);
 try{
  await view.click([...view.host.querySelectorAll('button')].find(b=>b.textContent==='新的结果'));assert.match(view.host.textContent,/第二次保存已确认/);
  await act(()=>timers[0]());await view.flush();assert.match(view.host.textContent,/第二次保存已确认/);
  await act(()=>timers.at(-1)());await view.flush();assert.equal(view.host.querySelector('[data-feedback-kind]'),null);
 }finally{await view.dispose();globalThis.setTimeout=original;}
});
test('accepted readback failure has one primary feedback and restores the real page result without another command',async()=>{
 const {ActionFeedback}=await vite.ssrLoadModule('/src/react-app/components.tsx');let reads=0;
 const state=proxy({session:{actorId:'feedback-action-owner'},selectedId:'A',operations:[{id:'original',fid:'A',label:'原操作',status:'success',requestStatus:'accepted',response:'请求已受理',readbackError:'原结果暂未读全',originalInput:'{"preserved":true}',refreshResult:()=>{reads++;state.operations[0].readbackError='';state.operations[0].originalInput='';}}]});
 const w={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state)},view=await mountReact(vite,ActionFeedback,{workspace:w,props:{labels:['原操作'],showStatus:true}});
 try{assert.equal(view.host.querySelectorAll('[data-feedback-kind]').length,1);assert.equal(view.host.querySelector('[data-feedback-kind]').dataset.feedbackKind,'action-required');assert.doesNotMatch(view.host.textContent,/请求已受理/);await view.click([...view.host.querySelectorAll('button')].find(b=>b.textContent==='查看等待核对的原提交'));assert.match(view.host.querySelector('pre').textContent,/preserved/);await view.click([...view.host.querySelectorAll('button')].find(b=>b.textContent.trim()==='重新读取页面结果'));assert.equal(reads,1);assert.ok(view.host.querySelector('[data-feedback-kind=action-required]')===null);assert.match(view.host.textContent,/请求已受理/);}finally{await view.dispose();}
});
test('r18 checking a retained reply exposes a visible result and targets the original turn without replay',async()=>{
 const {default:TurnFeedback}=await vite.ssrLoadModule('/src/react-app/components/ChatTurnFeedback.tsx'),calls=[];
 const message={id:'original-message',turnId:'original-turn',turnStatus:'failed',turnRetryable:false,turnErrorCode:'agent_run_failed'},before=JSON.stringify(message),store={sending:false,retrying:false,error:'',refreshActiveTurn:async(explicit,turnId)=>{calls.push([explicit,turnId]);return 'failed';},retryMessage:()=>{throw Error('status inspection must not replay');}};
 const view=await mountReact(vite,TurnFeedback,{props:{message,messages:[{kind:'user',turnId:'original-turn'}],store}});
 try{await view.click([...view.host.querySelectorAll('button')].find(b=>b.textContent.trim()==='查看状态'));assert.deepEqual(calls,[[true,'original-turn']]);assert.match(view.host.textContent,/已核对：执行失败/);assert.equal(JSON.stringify(message),before);}finally{await view.dispose();}
});
test('r18 a task notification opens its record in place and keeps read status separate from the task',async()=>{
 const {default:Inbox}=await vite.ssrLoadModule('/src/react-app/pages/InboxPage.jsx'),moves=[];let reads=0,tasks=0;
 const notice={id:'N',title:'初始衣柜需要处理',body:'原角色与世界会保留。',fluctlight_id:'A',created_at:'2026-10-29T00:00:00Z',read_at:null,source_kind:'owner.request.settled',link:'/diag/task/T'};
 const state=proxy({session:{actorId:'r18-notice-owner'},selectedId:'A',characters:[{id:'A',name:'原角色'}],notificationCount:{count:1}}),w={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),current:{value:{id:'A',name:'原角色'}},client:{},navigatePath:path=>moves.push(path),navigate(){},markNotificationRead:async()=>{reads++;return {id:'N',read_at:'2026-10-29T00:01:00Z'};},refreshNotificationCount(){},services:{operationsClient:{notifications:async()=>({items:[notice],as_of:'2026-10-29T00:00:00Z',next_cursor:''}),pending:async()=>({groups:[]}),task:async()=>{tasks++;return {task_id:'T',kind:'wardrobe.initialize',fluctlight_id:'A',status:'failed',steps:[],source:{last_error:'initialization_role_unconfigured'}};}}}};
 const view=await mountReact(vite,Inbox,{workspace:w});
 try{await view.click(view.host.querySelector('.replica-notice-main'));assert.equal(reads,1);await view.click([...document.querySelectorAll('[role=dialog] button')].find(b=>b.textContent.includes('查看原始来源')));assert.equal(tasks,1);assert.deepEqual(moves,[]);assert.match(document.querySelector('[role=dialog]').textContent,/通用模型/);await view.click([...document.querySelectorAll('[role=dialog] button')].find(b=>b.textContent.trim()==='返回通知详情'));assert.match(document.querySelector('[role=dialog]').textContent,/初始衣柜需要处理/);assert.equal(reads,1);assert.deepEqual(moves,[]);}finally{await view.dispose();}
});
test('r18 a historical reply without a turn identity cannot trigger the general queue observer',async()=>{
 const {default:TurnFeedback}=await vite.ssrLoadModule('/src/react-app/components/ChatTurnFeedback.tsx');let reads=0;
 const view=await mountReact(vite,TurnFeedback,{props:{message:{id:'legacy-message',turnStatus:'failed',turnRetryable:false,turnErrorCode:'agent_run_failed'},messages:[],store:{sending:false,retrying:false,error:'',refreshActiveTurn:async()=>{reads++;},retryMessage(){throw Error('must not replay');}}}});
 try{assert.ok(![...view.host.querySelectorAll('button')].some(b=>b.textContent.trim()==='查看状态'));assert.match(view.host.textContent,/暂时无法单独核对状态/);assert.equal(reads,0);}finally{await view.dispose();}
});

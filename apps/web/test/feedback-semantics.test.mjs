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

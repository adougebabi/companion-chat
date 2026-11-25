import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {act,installDOM,mountReact} from './helpers/react-dom.mjs';
const env=installDOM();
const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),envFile:false,appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const originalFetch=globalThis.fetch;
let fetchHandler;
globalThis.fetch=(...args)=>fetchHandler(...args);
const {createConversationController}=await vite.ssrLoadModule('/src/state/conversation.ts');
const {default:Chat}=await vite.ssrLoadModule('/src/react-app/pages/ChatPage.tsx');
test.after(async()=>{globalThis.fetch=originalFetch;await vite.close();env.restore();});
const conversation={id:'history-C',createdByActorId:'owner',revision:0};
const message=n=>({id:'history-'+n,conversationId:conversation.id,sequence:n,authorActorId:n%2?'owner':'A',kind:n%2?'user':'assistant',text:'历史正文 '+n,attachmentRefs:[],createdAt:'2026-11-25T00:00:00Z'});
const range=(a,b)=>Array.from({length:b-a+1},(_,i)=>message(a+i));
const page=(messages,nextBeforeSequence)=>({conversation,participants:[],messages,nextBeforeSequence});
const button=(root,text)=>[...root.querySelectorAll('button')].find(node=>node.textContent===text);
function store(){localStorage.clear();const s=createConversationController();s.fluctlightId='A';s.conversation=conversation;s.messages=range(101,150);s.nextBeforeSequence=101;return s;}
function workspace(s,archived=false){
 const state=proxy({session:{actorId:'owner'},selectedId:'A',characters:[{id:'A',name:'测试角色',initial:'角',color:'#eee',status:archived?'archived':'active',schedule:[],wearing:{},garments:[],_directory:{}}],errors:{},loading:{}});
 return {state,subscribe:f=>subscribe(state,f),getSnapshot:()=>snapshot(state),current:{get value(){return state.characters[0];}},currentScope:()=>JSON.stringify(['owner',state.selectedId]),conversation:()=>s,openConversation:async()=>{},clock:()=>'',detail:async()=>{},loadWardrobe:async()=>{},services:{mediaClient:{url:id=>id}}};
}

test('main chat loads older pages with progress, retry, stable draft and a separate read error',async()=>{
 const s=store(),calls=[];let finish;
 fetchHandler=async input=>{calls.push(new URL(String(input)).searchParams.get('beforeSequence'));return new Promise(resolve=>{finish=resolve;});};
 const view=await mountReact(vite,Chat,{workspace:workspace(s),props:{preview:false,onGo(){}}});
 try{
  const field=view.host.querySelector('textarea');await view.input(field,'尚未发送的草稿');
  await view.click(button(view.host,'加载更早消息'));
  assert.equal(button(view.host,'正在读取更早消息…').disabled,true);
  assert.deepEqual(calls,['101']);assert.equal(s.messages.length,50);
  await act(async()=>finish(Response.json({message:'temporary failure'},{status:503})));await view.flush();
  assert.match(view.host.querySelector('[role=alert]').textContent,/更早的对话记录/);
  assert.equal(s.error,'');assert.equal(s.nextBeforeSequence,101);assert.equal(s.messages.length,50);
  await view.click(button(view.host,'重试加载更早消息'));
  await act(async()=>finish(Response.json(page(range(51,100),51)))) ;await view.flush();
  assert.deepEqual(calls,['101','101']);assert.equal(s.messages.length,100);
  assert.match(view.host.textContent,/历史正文 51/);assert.match(view.host.textContent,/历史正文 150/);
  assert.equal(view.host.querySelector('textarea'),field);assert.equal(field.value,'尚未发送的草稿');
  await view.click(button(view.host,'加载更早消息'));
  await act(async()=>finish(Response.json(page(range(1,50),null))));await view.flush();
  assert.equal(s.messages.length,150);assert.deepEqual(calls,['101','101','51']);
  assert.equal(button(view.host,'加载更早消息'),undefined);assert.equal(s.historyError,'');
 }finally{await view.dispose();}
});

test('archived chat can still load older messages while its composer remains disabled',async()=>{
 const s=store();fetchHandler=async()=>Response.json(page(range(1,100),null));
 const view=await mountReact(vite,Chat,{workspace:workspace(s,true),props:{preview:false,onGo(){}}});
 try{assert.equal(view.host.querySelector('textarea').disabled,true);await view.click(button(view.host,'加载更早消息'));assert.match(view.host.textContent,/历史正文 1/);assert.equal(s.messages.length,150);}finally{await view.dispose();}
});

test('late history responses cannot enter a reopened conversation or clear its newer loading state',async()=>{
 const s=store(),pending=[];fetchHandler=()=>new Promise(resolve=>pending.push(resolve));
 const old=s.loadOlder();assert.equal(s.historyLoading,true);
 s.invalidateRequest();s.fluctlightId='B';s.conversation={id:'B'};
 s.invalidateRequest();s.fluctlightId='A';s.conversation=conversation;s.messages=range(101,150);s.nextBeforeSequence=101;
 const current=s.loadOlder();assert.equal(pending.length,2);
 pending[0](Response.json(page([message(1)],null)));await old;
 assert.equal(s.historyLoading,true);assert.equal(s.messages.length,50);assert.equal(s.nextBeforeSequence,101);
 pending[1](Response.json(page(range(51,100),51)));await current;
 assert.equal(s.historyLoading,false);assert.equal(s.messages.length,100);assert.equal(s.nextBeforeSequence,51);
 // Failed reads also remain local and never overwrite an existing turn failure.
 s.error='原有回复失败';fetchHandler=async()=>{throw Error('offline');};await s.loadOlder();
 assert.equal(s.error,'原有回复失败');assert.match(s.historyError,/更早的对话记录/);assert.equal(s.nextBeforeSequence,51);
});

test('sending and polling retain loaded history and never reset its oldest cursor',async()=>{
 const s=store();let newest=150,olderReads=[];const committed=new Map();
 fetchHandler=async(input,init={})=>{
  const url=new URL(String(input));
  if(url.pathname.endsWith('/turn')){
   const body=JSON.parse(init.body);const user={...message(++newest),text:body.text,turnId:body.turnId,idempotencyKey:body.idempotencyKey,turnStatus:'completed'};
   const assistant=message(++newest);committed.set(user.sequence,user);committed.set(assistant.sequence,assistant);
   const frames=[{type:'message',payload:{message:user}},{type:'message',payload:{message:assistant}},{type:'completed',payload:{}}];
   return new Response(frames.map((f,sequence)=>JSON.stringify({...f,sequence,turnId:body.turnId})).join('\n')+'\n');
  }
  if(url.pathname.endsWith('/read'))return new Response(null,{status:204});
  const before=Number(url.searchParams.get('beforeSequence'));
  if(before){olderReads.push(before);return Response.json(page(range(Math.max(1,before-50),before-1),before>51?before-50:null));}
  return Response.json(page(range(newest-49,newest).map(row=>committed.get(row.sequence)||row),newest-49));
 };
 await s.loadOlder();assert.equal(s.nextBeforeSequence,51);assert.equal(s.messages.length,100);
 assert.equal(await s.send('继续聊'),true,s.error);assert.equal(s.nextBeforeSequence,51);assert.equal(s.messages[0].sequence,51);
 await s.refreshActiveTurn(true);assert.equal(s.nextBeforeSequence,51);assert.equal(s.messages[0].sequence,51);
 await s.loadOlder();assert.deepEqual(olderReads,[101,51]);assert.equal(s.nextBeforeSequence,null);
 assert.equal(await s.send('已经翻到最早了'),true,s.error);assert.equal(s.nextBeforeSequence,null);
 assert.deepEqual(s.messages.map(m=>m.sequence),Array.from({length:154},(_,i)=>i+1));
});

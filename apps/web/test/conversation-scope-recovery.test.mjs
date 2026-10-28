import test from 'node:test';
import {ref as untracked} from 'valtio/vanilla';
import assert from 'node:assert/strict';

import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';

test('leaving a source page while its conversation read is pending cannot replace the selected role',async()=>{
  const old={window:globalThis.window,fetch:globalThis.fetch,storage:globalThis.localStorage};
  const saved=new Map();globalThis.window={location:{origin:'http://scope.test'}};globalThis.localStorage={getItem:key=>saved.get(key)??null,setItem:(key,value)=>saved.set(key,String(value)),removeItem:key=>saved.delete(key)};
  let release,entered;const started=new Promise(resolve=>{entered=resolve;});
  const delayed=new Promise(resolve=>{release=resolve;});
  globalThis.fetch=async()=>{entered();await delayed;return Response.json({conversation:{id:'conversation-A',revision:0},messages:[{id:'message-A',kind:'user',text:'A'}],participants:[],nextBeforeSequence:null});};
  const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
  try{
    const {createConversationController}=await vite.ssrLoadModule('/src/state/conversation.ts');const store=createConversationController();
    store.fluctlights=[{id:'role-A',identity:{name:'A'},status:'active'},{id:'role-B',identity:{name:'B'},status:'active'}];store.fluctlightId='role-B';store.conversation={id:'conversation-B',revision:0};store.messages=[{id:'message-B',kind:'user',text:'B'}];
    let current=true;const reading=store.selectFluctlight('role-A',{current:()=>current});await started;current=false;release();await reading;
    assert.equal(store.fluctlightId,'role-B');assert.equal(store.conversation.id,'conversation-B');assert.deepEqual(store.messages.map(m=>m.id),['message-B']);assert.equal(store.error,'');assert.equal(store.loading,false);assert.equal([...saved.values()].includes('role-A'),false);
  }finally{await vite.close();globalThis.window=old.window;globalThis.fetch=old.fetch;globalThis.localStorage=old.storage;}
});

test('explicit progress read closes a stuck observer only for its confirmed terminal turn',async()=>{
 const old={window:globalThis.window,fetch:globalThis.fetch,localStorage:globalThis.localStorage};const saved=new Map();globalThis.window={location:{origin:'http://scope.test'}};globalThis.localStorage={getItem:k=>saved.get(k)??null,setItem:(k,v)=>saved.set(k,String(v)),removeItem:k=>saved.delete(k)};
 let status='running';const user={id:'saved-user',kind:'user',conversationId:'conversation',sequence:1,authorActorId:'owner',text:'hello',turnId:'turn',idempotencyKey:'same',turnRetryable:true,createdAt:'2026-10-22T00:00:00Z',attachmentRefs:[]};
 globalThis.fetch=async()=>Response.json({conversation:{id:'conversation'},messages:[{...user,turnStatus:status,turnErrorCode:status==='failed'?'request_timeout':undefined}]});
 const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
 try{const {createConversationController}=await vite.ssrLoadModule('/src/state/conversation.ts');const state=createConversationController();state.fluctlightId='role';state.conversation={id:'conversation'};state.activeTurnId='turn';state.sending=true;state.abortController=untracked(new AbortController());const signal=state.abortController.signal;state.messages=[{...user,turnStatus:'running'},{id:'stream-one',kind:'assistant',text:'partial'}];
  await state.refreshActiveTurn(true);assert.equal(state.sending,true);assert.equal(signal.aborted,false);assert.equal(state.messages.at(-1).text,'partial');
  status='failed';await state.refreshActiveTurn(true);assert.equal(signal.aborted,true);assert.equal(state.sending,false);assert.equal(state.retryTurn.turnId,'turn');assert.equal(state.retryTurn.idempotencyKey,'same');assert.match(state.error,/request_timeout/);assert.equal(state.messages.some(m=>m.id==='stream-one'),false);
 }finally{await vite.close();Object.assign(globalThis,old);}
});

test('a failed initial conversation read releases loading and allows reopening the same role',async()=>{
 const old={window:globalThis.window,fetch:globalThis.fetch,localStorage:globalThis.localStorage};const saved=new Map();globalThis.window={location:{origin:'http://scope.test'}};globalThis.localStorage={getItem:key=>saved.get(key)??null,setItem:(key,value)=>saved.set(key,String(value)),removeItem:key=>saved.delete(key)};
 let reads=0;globalThis.fetch=async()=>++reads===1?Response.json({message:'temporarily unavailable'},{status:503}):Response.json({conversation:{id:'recovered-conversation',revision:0},messages:[{id:'saved-message',kind:'assistant',text:'已读回'}],participants:[],nextBeforeSequence:null});
 const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
 try{const {createConversationController}=await vite.ssrLoadModule('/src/state/conversation.ts');const store=createConversationController();store.fluctlights=[{id:'role',identity:{name:'角色'},status:'active'}];await store.selectFluctlight('role');assert.equal(store.loading,false);assert.equal(store.conversation,null);assert.match(store.error,/无法打开/);await store.selectFluctlight('role');assert.equal(store.loading,false);assert.equal(store.fluctlightId,'role');assert.equal(store.conversation.id,'recovered-conversation');assert.equal(store.messages[0].text,'已读回');assert.equal(store.error,'');assert.equal(reads,2);}finally{await vite.close();Object.assign(globalThis,old);}
});

import test from 'node:test';
import assert from 'node:assert/strict';
import {createPinia,setActivePinia} from 'pinia';
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
    setActivePinia(createPinia());const {useConversationStore}=await vite.ssrLoadModule('/src/stores/conversations.ts');const store=useConversationStore();
    store.fluctlights=[{id:'role-A',identity:{name:'A'},status:'active'},{id:'role-B',identity:{name:'B'},status:'active'}];store.fluctlightId='role-B';store.conversation={id:'conversation-B',revision:0};store.messages=[{id:'message-B',kind:'user',text:'B'}];
    let current=true;const reading=store.selectFluctlight('role-A',{current:()=>current});await started;current=false;release();await reading;
    assert.equal(store.fluctlightId,'role-B');assert.equal(store.conversation.id,'conversation-B');assert.deepEqual(store.messages.map(m=>m.id),['message-B']);assert.equal(store.error,'');assert.equal(store.loading,false);assert.equal([...saved.values()].includes('role-A'),false);
  }finally{await vite.close();globalThis.window=old.window;globalThis.fetch=old.fetch;globalThis.localStorage=old.storage;}
});

import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {memoryStorage} from './support/runtime.mjs';

test('workspace logout waits for the authoritative provider, retains failed-session state and clears on confirmed logout',async()=>{
 const previous={window:globalThis.window,document:globalThis.document,location:globalThis.location,localStorage:globalThis.localStorage,sessionStorage:globalThis.sessionStorage};
 globalThis.window={location:{origin:'http://account.test'},addEventListener(){},removeEventListener(){}};globalThis.document=new EventTarget();document.hidden=false;globalThis.location={hash:'#/settings'};globalThis.localStorage=memoryStorage();globalThis.sessionStorage=memoryStorage();
 const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});let w;
 try{
  const {createWorkspace}=await vite.ssrLoadModule('/src/state/workspace.mjs'),providers=await vite.ssrLoadModule('/src/v3/providers/real.ts'),store=await vite.ssrLoadModule('/src/v3/proto/store.js');
  w=createWorkspace();store.replaceSnapshot({...store.get(),session:{actorId:'owner'},characters:{role:{id:'role',name:'周屿'}},ui:{charId:'role'}});
  let calls=0;providers.auth.logout=async()=>{calls++;throw Error('controlled connection failure');};
  await assert.rejects(w.logout(),/controlled connection failure/);assert.equal(calls,1);assert.equal(location.hash,'#/settings');assert.equal(w.state.session.actorId,'owner');assert.equal(w.state.characters.length,1);
  providers.auth.logout=async()=>{calls++;store.replaceSnapshot({...store.get(),session:null,characters:{},ui:{charId:''}});};
  await w.logout();assert.equal(calls,2);assert.equal(location.hash,'#/auth');assert.equal(w.state.session,null);assert.equal(w.state.characters.length,0);
 }finally{w?.dispose();await vite.close();for(const [key,value]of Object.entries(previous))if(value===undefined)delete globalThis[key];else globalThis[key]=value;}
});

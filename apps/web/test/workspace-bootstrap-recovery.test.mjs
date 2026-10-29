import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {memoryStorage} from './support/runtime.mjs';
const previous=Object.fromEntries(['window','document','location','localStorage','sessionStorage'].map(k=>[k,globalThis[k]]));
globalThis.window={location:{origin:'http://bootstrap.test'},addEventListener(){},removeEventListener(){}};
globalThis.document=new EventTarget();document.hidden=false;globalThis.location={hash:'#/library'};
globalThis.localStorage=memoryStorage();globalThis.sessionStorage=memoryStorage();
const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const {createWorkspace}=await vite.ssrLoadModule('/src/state/workspace.mjs');
const real=await vite.ssrLoadModule('/src/v3/providers/real.ts');
const store=await vite.ssrLoadModule('/src/v3/proto/store.js');
// Provider registration owns an application-lifetime notification interval.
// This isolated test owns that application lifetime, so release it on teardown.
const providerIntervals=[],originalSetInterval=globalThis.setInterval;
try{
  globalThis.setInterval=(...args)=>{const timer=originalSetInterval(...args);providerIntervals.push(timer);return timer;};
  real.registerProviders();
}finally{globalThis.setInterval=originalSetInterval;}
test.after(async()=>{providerIntervals.forEach(clearInterval);await vite.close();for(const [key,value] of Object.entries(previous))if(value===undefined)delete globalThis[key];else globalThis[key]=value;});

test('failed required directory read recovers bootstrap and clears only its own failure',async()=>{
  const old={session:real.client.session,list:real.client.listFluctlights,count:real.client.unreadNotificationCount};
  let fail=true,reads=0;
  real.client.session=async()=>({authenticated:true,actorId:'bootstrap-owner'});
  real.client.listFluctlights=async()=>{reads++;if(fail)throw Object.assign(Error('directory unavailable'),{status:503});return [];};
  real.client.unreadNotificationCount=async()=>({count:0});
  store.replaceSnapshot({...store.get(),session:null,characters:{},ui:{charId:''}});
  const w=createWorkspace();
  try{
    await assert.rejects(w.bootstrap());assert.ok(w.state.errors.bootstrap);assert.equal(w.state.ready,true);assert.equal(w.state.characters.length,0);
    fail=false;
    await w.bootstrap();
    assert.equal(w.state.session.actorId,'bootstrap-owner');assert.equal(w.state.errors.bootstrap,undefined);
    w.state.errors.unrelated='keep this independent read failure';
    await w.bootstrap();assert.equal(w.state.errors.unrelated,'keep this independent read failure');assert.equal(reads,3);
  }finally{w.dispose();Object.assign(real.client,{session:old.session,listFluctlights:old.list,unreadNotificationCount:old.count});}
});

test('confirmed unauthenticated session clears old startup failure without reading a directory',async()=>{
  const old={session:real.client.session,list:real.client.listFluctlights};let reads=0;
  real.client.session=async()=>({authenticated:false});real.client.listFluctlights=async()=>{reads++;return [];};
  store.replaceSnapshot({...store.get(),session:null,characters:{},ui:{charId:''}});
  const w=createWorkspace();
  try{
    w.state.errors.bootstrap='an earlier transient failure';await w.bootstrap();
    assert.equal(w.state.session,null);assert.equal(w.state.errors.bootstrap,undefined);assert.equal(w.state.ready,true);assert.equal(reads,0);
  }finally{w.dispose();Object.assign(real.client,{session:old.session,listFluctlights:old.list});}
});
test('Now loads its real detail, schedule and wardrobe without unrelated goal, diary, memory or promise reads',async()=>{
 const services=await vite.ssrLoadModule('/src/v3/proto/services.js'),{workspaceReadFeedback}=await vite.ssrLoadModule('/src/approved-ui/read-feedback.mjs'),oldDetail=real.client.detail;
 const calls={detail:0,schedule:0,wardrobe:0,goals:0,diary:0,memory:0,promises:0};
 real.client.detail=async()=>{calls.detail++;return {id:'A',identity:{name:'此刻样本',timezone:'Asia/Shanghai'},context:{activity:'阅读',location:'书房'},inner_state:{mood:'平静'}};};
 const unrelated=key=>async()=>{calls[key]++;throw Error('unrelated optional read must not run for Now');};
 services.configureDomainService('life',{schedules:async()=>{calls.schedule++;return {days:[],timezone:'Asia/Shanghai'};},wardrobe:async()=>{calls.wardrobe++;return {revision:1,worn_items:[],used_items:[]};},goals:unrelated('goals'),promises:unrelated('promises')});
 services.configureDomainService('diary',{list:unrelated('diary')});services.configureDomainService('memory',{list:unrelated('memory')});
 services.configureDomainService('inventory',{clothingCatalogue:async()=>({items:[],next_cursor:''}),outfits:async()=>({items:[],next_cursor:'',profile_id:'profile'}),tags:async()=>({tags:[]})});
 const w=createWorkspace();store.replaceSnapshot({...store.get(),session:{actorId:'now-owner'},characters:{A:{id:'A',name:'此刻样本'}},ui:{charId:'A'}});
 try{
  await w.load('此刻');assert.deepEqual(calls,{detail:1,schedule:1,wardrobe:1,goals:0,diary:0,memory:0,promises:0});assert.equal(w.current.value.activity,'阅读');assert.equal(w.current.value.location,'书房');assert.ok(w.current.value._wardrobe);
  await w.loadLife();assert.equal(calls.promises,1);assert.equal(calls.goals,1);assert.equal(calls.diary,1);assert.equal(calls.memory,1);assert.ok(w.state.errors.promises);
  assert.ok(!workspaceReadFeedback(w.state,'此刻',w.currentScope()).errors.some(e=>e.key==='promises'));
 }finally{w.dispose();real.client.detail=oldDetail;}
});

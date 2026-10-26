import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {mountReact,installDOM} from './helpers/react-dom.mjs';
import {tick} from './support/runtime.mjs';
import {creationFixture,validAnalysis} from './support/creation-fixture.mjs';
import {creationDraft} from '../src/v3/creation-draft.mjs';

test('active creation ignores a stale analysis without activating or overwriting the replacement owner draft',async()=>{
 let finish;const response=new Promise(resolve=>finish=resolve),f=creationFixture(()=>response);f.c.selectWorld(null);f.c.draft.bio='原始描述';f.c.next();const waiting=f.c.analyze();await tick();
 const key='fluctlight-creation:owner:draft',newer={...JSON.parse(f.saved.getItem(key)),requestId:'replacement',name:'新草稿',bio:'保留新输入',analysis:null};f.saved.setItem(key,JSON.stringify(newer));
 finish(validAnalysis());await waiting;assert.equal(f.calls.length,0);assert.deepEqual(JSON.parse(f.saved.getItem(key)),newer);f.c.dispose();
});
test('active creation retains editable unnamed preview, blocks activation, then creates once and starts a fresh draft',async()=>{
 const f=creationFixture();f.c.selectWorld(null);f.c.draft.bio='人物原始描述';f.c.next();await f.c.analyze();assert.equal(f.c.step.value,'preview');assert.equal(f.c.draft.bio,'人物原始描述');assert.equal(f.calls.length,0);
 await f.c.activate();assert.equal(f.calls.length,0);assert.match(f.c.error.value,/填写角色名字/);
 const name=f.c.previewFields.value.find(field=>JSON.stringify(field.path)==='["identity","name"]');assert.ok(name);name.value='周屿';f.c.persistPreview();await f.c.activate();
 assert.equal(f.calls.length,1);assert.equal(f.c.draft.activatedId,'created');assert.equal(f.calls[0].name,'周屿');f.c.newDraft();assert.equal(f.c.step.value,'world');assert.equal(f.c.draft.mode,'description');assert.equal(f.c.draft.activatedId,null);assert.equal(f.c.draft.bio,'');f.c.dispose();
});
test('React library renders durable continuation and a role whose group has not loaded',async()=>{
 const environment=installDOM();const recovery=creationDraft({owner:'owner',storage:localStorage,timezone:'Asia/Shanghai'});recovery.save({...recovery.read(),name:'小林',wizardStep:2});
 const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
 let view;try{
  const {default:Library}=await vite.ssrLoadModule('/src/react-app/pages/LibraryPage.jsx');
  const state=proxy({session:{actorId:'owner'},characters:[{id:'role',groupId:'unloaded-group',name:'晚晴',status:'正常',tags:[],job:'',bio:''}],groups:[]});
  const w={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),services:{configurationClient:{read:async()=>({bindings:[]})}},select(){},navigate(){},navigatePath(){}};
  view=await mountReact(vite,Library,{workspace:w});assert.match(view.host.textContent,/继续创建与查看进度/);assert.match(view.host.textContent,/晚晴/);
 }finally{await view?.dispose();await vite.close();environment.restore();}
});

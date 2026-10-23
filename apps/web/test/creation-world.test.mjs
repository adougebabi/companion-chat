import test from 'node:test';
import assert from 'node:assert/strict';
import {finishCreationWorld} from '../src/v3/creation-world.mjs';

function fixture(){
 let draft={activatedId:'actual-character',worldResourceId:'world-definition',worldResourceVersion:'1'},active=null,applyCalls=0,selectCalls=0,failSelect=true;
 const saved=new Map(),storage={getItem:key=>saved.get(key)||null,setItem:(key,value)=>saved.set(key,value),removeItem:key=>saved.delete(key)};
 const recovery={read:()=>structuredClone(draft),save:value=>{draft=structuredClone(value);}};
 const client={worldCatalog:async()=>({items:[],active_binding:active}),previewStudioApply:async()=>({instance_revision:7,runtime_revision:0,diff:[]}),applyStudioResource:async(_id,body)=>{applyCalls++;assert.equal(body.fluctlightId,'actual-character');return {applied:true,id:'independent-runtime-world'};},selectWorld:async(fid,body)=>{selectCalls++;assert.equal(fid,'actual-character');if(failSelect){failSelect=false;throw new TypeError('response lost');}active={world_id:body.worldId,revision:1};return {id:body.worldId};},world:async()=>({world:active?{id:active.world_id}:null})};
 return {options:{client,recovery,storage,owner:'owner'},counts:()=>({applyCalls,selectCalls}),draft:()=>draft};
}
test('accepted character resumes world selection without activating or applying a second time',async()=>{
 const f=fixture();await assert.rejects(finishCreationWorld(f.options),/response lost/);
 assert.equal(f.draft().activatedId,'actual-character');assert.equal(f.draft().worldRuntimeId,'independent-runtime-world');
 assert.deepEqual(await finishCreationWorld(f.options),{configured:true,worldId:'independent-runtime-world'});
 assert.deepEqual(f.counts(),{applyCalls:1,selectCalls:2});
});
test('an unselected world stays an explicit incomplete step rather than invented runtime data',async()=>{
 const f=fixture();f.options.recovery.save({activatedId:'actual-character'});
 const result=await finishCreationWorld(f.options);assert.equal(result.configured,false);assert.match(result.warning,/尚未选择/);assert.deepEqual(f.counts(),{applyCalls:0,selectCalls:0});
});

test('default world is a real resumable write and confirms the selected binding',async()=>{
 let draft={activatedId:'actual-character',useDefaultWorld:true},world=null,unknown=true;const calls=[];
 const values=new Map(),storage={getItem:k=>values.get(k)??null,setItem:(k,v)=>values.set(k,v),removeItem:k=>values.delete(k)};
 const recovery={read:()=>structuredClone(draft),save:v=>draft=structuredClone(v)};
 const client={world:async()=>({world}),updateRuntimeWorld:async(fid,body)=>{assert.equal(fid,'actual-character');calls.push(structuredClone(body));world={id:'actual-default-world'};if(unknown){unknown=false;throw Error('response lost');}return {id:world.id};}};
 const options={client,recovery,storage,owner:'owner'};
 await assert.rejects(finishCreationWorld(options),/response lost/);assert.equal(draft.worldRuntimeId,undefined);
 assert.deepEqual(await finishCreationWorld(options),{configured:true,worldId:'actual-default-world'});assert.equal(draft.worldSelectionConfirmed,true);assert.deepEqual(calls[0],calls[1]);assert.equal(calls.length,2);
});
test('default world acceptance alone cannot fake a successful binding readback',async()=>{
 let draft={activatedId:'actual-character',useDefaultWorld:true};const values=new Map();const options={owner:'owner',storage:{getItem:k=>values.get(k)??null,setItem:(k,v)=>values.set(k,v),removeItem:k=>values.delete(k)},recovery:{read:()=>structuredClone(draft),save:v=>draft=v},client:{world:async()=>({world:null}),updateRuntimeWorld:async()=>({id:'created-but-unconfirmed'})}};
 await assert.rejects(finishCreationWorld(options),/尚未读回/);assert.equal(draft.activatedId,'actual-character');assert.equal(draft.worldRuntimeId,'created-but-unconfirmed');assert.notEqual(draft.worldSelectionConfirmed,true);
});

for(const phase of ['initial read','write','confirmation read'])test(`default world ${phase} ignores a replacement draft and changed Owner`,async()=>{
 for(const change of ['draft','owner']){
  let draft={requestId:'original-request',activatedId:'original-character',useDefaultWorld:true},ownerCurrent=true,reads=0,writes=0,release;
  const deferred=new Promise(resolve=>release=resolve),values=new Map();
  const recovery={read:()=>structuredClone(draft),save:value=>draft=structuredClone(value)};
  const client={world:async()=>{reads++;if(phase==='initial read'&&reads===1||phase==='confirmation read'&&reads===2)await deferred;return {world:reads>1?{id:'original-world'}:null};},updateRuntimeWorld:async()=>{writes++;if(phase==='write')await deferred;return {id:'original-world'};}};
  const pending=finishCreationWorld({client,recovery,owner:'owner',ownerCurrent:()=>ownerCurrent,storage:{getItem:k=>values.get(k)??null,setItem:(k,v)=>values.set(k,v),removeItem:k=>values.delete(k)}});
  while(!release||phase!=='initial read'&&writes===0||phase==='confirmation read'&&reads<2)await new Promise(resolve=>setImmediate(resolve));
  if(change==='draft')draft={requestId:'replacement-request',activatedId:null,useDefaultWorld:true,bio:'keep newer input'};else ownerCurrent=false;
  const before=structuredClone(draft);release();assert.equal((await pending).stale,true);assert.deepEqual(draft,before);
  if(phase==='initial read')assert.equal(writes,0);
 }
});

for(const phase of ['catalog','preview','apply','select'])test(`published world ${phase} stops after scope changes`,async()=>{
 const f=fixture();let release,ownerCurrent=true;const delayed=new Promise(resolve=>release=resolve),original=f.options.client;
 const name={catalog:'worldCatalog',preview:'previewStudioApply',apply:'applyStudioResource',select:'selectWorld'}[phase],run=original[name];let entered=false;
 original[name]=async(...args)=>{entered=true;await delayed;return run(...args);};
 f.options.ownerCurrent=()=>ownerCurrent;const pending=finishCreationWorld(f.options);while(!entered)await new Promise(resolve=>setImmediate(resolve));
 ownerCurrent=false;const before=structuredClone(f.draft());release();
 // A deliberately failed select retains its original pending request, but no
 // stale local receipt or later command may follow it.
 if(phase==='select')await assert.rejects(pending,/response lost/);else assert.equal((await pending).stale,true);
 assert.deepEqual(f.draft(),before);if(['catalog','preview'].includes(phase))assert.deepEqual(f.counts(),{applyCalls:0,selectCalls:0});else assert.equal(f.counts().selectCalls,phase==='select'?1:0);
});

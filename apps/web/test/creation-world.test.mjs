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

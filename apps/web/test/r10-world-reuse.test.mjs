import test from 'node:test';
import assert from 'node:assert/strict';
import {finishCreationWorld} from '../src/v3/creation-world.mjs';
import {reviewWorldReuse,confirmWorldReuse} from '../src/v3/reuse-world.mjs';
import {reusableWorlds} from '../src/approved-ui/world-sources.mjs';
import {memoryStorage} from './support/runtime.mjs';
function fixture(mode='unknown'){
 let draft={requestId:'original',activatedId:'new-role',useDefaultWorld:false,worldSource:{worldId:'existing-world',fluctlightId:'source-role',revision:1,name:'杭州生活'}},active=null,attempt=0,owner=true;
 const calls=[],saved=memoryStorage(),recovery={read:()=>structuredClone(draft),save:value=>draft=structuredClone(value)};
 const client={createCatalogWorld:async(fid,body)=>{calls.push([fid,structuredClone(body)]);attempt++;if(attempt===1&&mode==='unknown')throw Error('response lost');if(attempt===1&&mode==='conflict')throw Object.assign(Error('source changed'),{status:409,code:'revision_conflict'});if(mode==='unknown-then-conflict'){if(attempt===1)throw Error('response lost');throw Object.assign(Error('source changed'),{status:409,code:'revision_conflict'});}return {id:'copied-world'};},worldCatalog:async()=>({items:[],active_binding:active}),selectWorld:async(fid,body)=>{assert.equal(fid,'new-role');active={world_id:body.worldId,revision:1};return {id:body.worldId};},world:async()=>({world:active?{id:active.world_id}:null}),catalogWorld:async(fid,id)=>{assert.equal(fid,'source-role');assert.equal(id,'existing-world');return {world:{id,revision:2,name:'杭州生活·新版',background:'已保存的新背景',rules:'保留规则'},places:[{name:'湖边书店',floor:'2F'}]};}};
 return {options:{client,recovery,storage:saved,owner:'owner',ownerCurrent:()=>owner},calls,draft:()=>draft,setOwner:v=>owner=v};
}
test('world sources retain each owning role and expose partial read failure without inventing an empty/default world',async()=>{
 const calls=[];const result=await reusableWorlds({worldCatalog:async fid=>{calls.push(fid);if(fid==='failed')throw Error('offline');return {items:[{id:'world-'+fid,name:'同名世界',revision:1}]};}},[{id:'A',name:'苏晚'},{id:'B',name:'周屿'},{id:'failed',name:'不可读'}]);
 assert.deepEqual(calls,['A','B','failed']);assert.equal(result.items.length,2);assert.equal(result.unavailable,1);assert.equal(result.items[0].fluctlightId,'A');assert.equal(result.items[1].fluctlightId,'B');assert.equal(result.items[1].characterName,'周屿');
});
test('unknown copy result replays exact source/revision/body/key and binds only the returned independent world',async()=>{
 const f=fixture();await assert.rejects(finishCreationWorld(f.options),/response lost/);assert.equal(f.draft().worldRuntimeId,undefined);
 assert.deepEqual(await finishCreationWorld(f.options),{configured:true,worldId:'copied-world'});assert.equal(f.calls.length,2);assert.deepEqual(f.calls[0],f.calls[1]);assert.equal(f.calls[0][1].sourceWorldId,'existing-world');assert.equal(f.calls[0][1].expectedSourceRevision,1);assert.equal(f.draft().worldSelectionConfirmed,true);assert.equal(f.draft().activatedId,'new-role');
});
test('only explicit review of a known pre-admission conflict can adopt a new configuration and command key',async()=>{
 const f=fixture('conflict');await assert.rejects(finishCreationWorld(f.options),/source changed/);const original=f.calls[0][1];
 const review=await reviewWorldReuse(f.options);assert.equal(review.preview.地点[0].楼层,'2F');assert.equal(f.draft().worldSource.revision,1);assert.equal(f.calls.length,1);
 confirmWorldReuse(f.options,review);assert.equal(f.draft().worldSource.revision,2);assert.equal(f.draft().activatedId,'new-role');await finishCreationWorld(f.options);assert.equal(f.calls.length,2);assert.notEqual(f.calls[1][1].idempotencyKey,original.idempotencyKey);assert.equal(f.calls[1][1].expectedSourceRevision,2);assert.equal(f.calls[1][0],'new-role');
});
test('a conflict after an earlier unknown attempt never authorizes a replacement body/key',async()=>{
 const f=fixture('unknown-then-conflict');await assert.rejects(finishCreationWorld(f.options),/response lost/);await assert.rejects(finishCreationWorld(f.options),/source changed/);assert.deepEqual(f.calls[0],f.calls[1]);await assert.rejects(reviewWorldReuse(f.options),/尚未确认无效果/);assert.equal(f.draft().worldSource.revision,1);assert.equal(f.calls.length,2);
});
test('a changed Owner cannot confirm an old world review or start another write',async()=>{
 const f=fixture('conflict');await assert.rejects(finishCreationWorld(f.options));const review=await reviewWorldReuse(f.options);f.setOwner(false);assert.throws(()=>confirmWorldReuse(f.options,review),/已变化/);assert.deepEqual(await finishCreationWorld(f.options),{configured:false,stale:true});assert.equal(f.calls.length,1);assert.equal(f.draft().worldSource.revision,1);
});

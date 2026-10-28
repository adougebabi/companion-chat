import test from 'node:test';
import assert from 'node:assert/strict';
import {createNotificationCounter} from '../src/state/notification-counter.mjs';
const setup=read=>{const state={},counter=createNotificationCounter({read,changed:patch=>Object.assign(state,patch)});counter.reset('A');return {state,counter};};
test('authoritative count is independent of the bounded page and keeps quiet alerts separate',async()=>{
 const {state,counter}=setup(async()=>({count:91,alert_count:3}));await counter.refresh();assert.deepEqual(state.notificationCount,{count:91,alert_count:3});assert.equal(state.notificationCountLoading,false);
});
test('an old Owner response cannot replace a later session with the same Owner ID',async()=>{
 const pending=[];const {state,counter}=setup(()=>new Promise(resolve=>pending.push(resolve)));
 const first=counter.refresh();counter.reset('B');const second=counter.refresh();counter.reset('A');const third=counter.refresh();
 pending[2]({count:4,alert_count:2});await third;pending[0]({count:99,alert_count:99});pending[1]({count:88,alert_count:88});await Promise.all([first,second]);assert.equal(state.notificationCount.count,4);
});
test('a count read started before a confirmed write cannot resurrect the old badge',async()=>{
 let release,reads=0;const {state,counter}=setup(()=>++reads===1?new Promise(r=>release=r):Promise.resolve({count:2,alert_count:1}));
 const old=counter.refresh();const receipt=await counter.afterWrite(async()=>({id:'notice',read_at:'server-read-at'}));assert.equal(receipt.read_at,'server-read-at');release({count:39,alert_count:30});await old;assert.equal(state.notificationCount.count,2);
});
test('failed count readback preserves both the committed write and the last known count',async()=>{
 let fail=false,writes=0;const {state,counter}=setup(async()=>{if(fail)throw Error('count offline');return {count:7,alert_count:5};});await counter.refresh();fail=true;
 const result=await counter.afterWrite(async()=>{writes++;return {updated:5,before:'observed-snapshot'};});assert.deepEqual(result,{updated:5,before:'observed-snapshot'});await new Promise(r=>setImmediate(r));assert.equal(writes,1);assert.equal(state.notificationCount.count,7);assert.ok(state.notificationCountError);
});
test('a late write after logout does not read or update a reentered session',async()=>{
 let release,reads=0;const {state,counter}=setup(async()=>{reads++;return {count:1,alert_count:0};});
 const write=counter.afterWrite(()=>new Promise(r=>release=r));counter.reset(null);counter.reset('A');release({id:'original'});await write;assert.equal(reads,0);assert.equal(state.notificationCount,null);
});
test('invalid counts and disposed reads cannot replace confirmed data with a fabricated zero',async()=>{
 let value={count:5,alert_count:2};const {state,counter}=setup(async()=>value);await counter.refresh();value={count:-1,alert_count:0};await counter.refresh();assert.equal(state.notificationCount.count,5);assert.ok(state.notificationCountError);
 let release;const other=setup(()=>new Promise(r=>release=r));const read=other.counter.refresh();other.counter.dispose();release({count:0,alert_count:0});await read;assert.equal(other.state.notificationCount,null);
});

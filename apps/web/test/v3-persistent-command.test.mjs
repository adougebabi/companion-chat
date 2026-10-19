import {test} from 'node:test';
import assert from 'node:assert/strict';
import {persistentCommand} from '../src/v3/providers/persistent-command.mjs';

function memoryStorage(){const values=new Map();return {getItem:key=>values.get(key)??null,setItem:(key,value)=>values.set(key,value),removeItem:key=>values.delete(key)};}
test('response loss and a page refresh preserve immutable input and idempotency before later edits can submit',async()=>{
 const storage=memoryStorage(),calls=[],original={title:'原约定',dueAt:'2026-10-21T10:00:00+08:00',reason:'A'};
 let loseResponse=true;
 const options={key:'owner/fid/proposal',storage:()=>storage,execute:async(value,idempotencyKey)=>{calls.push({value,idempotencyKey});if(loseResponse)throw new TypeError('response lost after admission');return {request:{id:'actual-request',status:'pending'}};}};
 const first=persistentCommand(options);
 await assert.rejects(first.submit(original),/response lost/);
 original.title='编辑中的新约定';
 const reloaded=persistentCommand(options);
 assert.equal(reloaded.pending().value.title,'原约定');
 await assert.rejects(reloaded.submit(original),/原提议的受理结果尚未确认/);assert.equal(calls.length,1);
 loseResponse=false;const recovered=await reloaded.retry();
 assert.equal(recovered.request.id,'actual-request');assert.equal(calls.length,2);assert.equal(calls[0].idempotencyKey,calls[1].idempotencyKey);assert.deepEqual(calls[0].value,calls[1].value);assert.equal(reloaded.pending(),null);
 await reloaded.submit(original);assert.notEqual(calls[2].idempotencyKey,calls[0].idempotencyKey);assert.equal(calls[2].value.title,'编辑中的新约定');
});
test('storage failure prevents admission and a lost-auth retry retains the original identity',async()=>{
 let calls=0;
 const blocked=persistentCommand({key:'blocked',storage:()=>({getItem:()=>null,setItem:()=>{throw new Error('storage unavailable')},removeItem:()=>{}}),execute:async()=>calls++});
 await assert.rejects(blocked.submit({title:'A'}),/storage unavailable/);assert.equal(calls,0);
 const storage=memoryStorage();
 const unauthorized=persistentCommand({key:'auth',storage:()=>storage,execute:async()=>{throw Object.assign(new Error('session lost'),{status:401})}});
 await assert.rejects(unauthorized.submit({title:'A'}));assert.ok(unauthorized.pending());
});
test('only a first, explicitly rejected CAS command can release its identity for deliberate rebase',async()=>{
 const storage=memoryStorage(),calls=[];
 const reject=()=>Object.assign(new Error('stale'),{status:409,code:'revision_conflict'});
 const command=persistentCommand({key:'cas',storage:()=>storage,execute:async(value,key)=>{calls.push({value,key});throw reject();}});
 await assert.rejects(command.submit({expectedRevision:1}));const original=command.pending();
 await assert.rejects(command.retry(),/明确因版本冲突/);assert.equal(calls.length,1);assert.equal(command.pending().attempts,1);
 const refreshed=persistentCommand({key:'cas',storage:()=>storage,execute:async()=>({saved:true})});
 assert.equal(refreshed.pending().rejection.code,'revision_conflict');refreshed.releaseRejected();
 await refreshed.submit({expectedRevision:2});assert.equal(refreshed.pending(),null);assert.ok(original.idempotencyKey);
 let first=true;
 const unknown=persistentCommand({key:'unknown',storage:()=>storage,execute:async()=>{if(first){first=false;throw new TypeError('response lost');}throw reject();}});
 await assert.rejects(unknown.submit({expectedRevision:1}));await assert.rejects(unknown.retry());
 assert.throws(()=>unknown.releaseRejected(),/不能更换/);assert.equal(unknown.pending().attempts,2);
});
test('logout cleanup and a replacement command survive late responses from the old identity',async()=>{
 const storage=memoryStorage();let failOld;
 const old=persistentCommand({key:'scoped',storage:()=>storage,execute:()=>new Promise((resolve,reject)=>{failOld=reject;})});
 const pending=old.submit({payload:'old Owner draft'});storage.removeItem('scoped');
 failOld(Object.assign(new Error('late stale'),{status:409,code:'revision_conflict'}));await assert.rejects(pending);assert.equal(storage.getItem('scoped'),null);
 let completeOld,completeNew;
 const a=persistentCommand({key:'scoped',storage:()=>storage,execute:()=>new Promise(resolve=>{completeOld=resolve;})});
 const previous=a.submit({payload:'before logout'});storage.removeItem('scoped');
 const b=persistentCommand({key:'scoped',storage:()=>storage,execute:()=>new Promise(resolve=>{completeNew=resolve;})});
 const replacement=b.submit({payload:'new authenticated draft'});const identity=b.pending().idempotencyKey;
 completeOld({saved:true});await previous;assert.equal(b.pending().idempotencyKey,identity);
 completeNew({saved:true});await replacement;assert.equal(b.pending(),null);
});

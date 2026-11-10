import test from 'node:test';
import assert from 'node:assert/strict';
import {persistentCommand} from '../src/v3/providers/persistent-command.mjs';

function memoryStorage(){const rows=new Map();return {getItem:key=>rows.get(key)??null,setItem:(key,value)=>rows.set(key,value),removeItem:key=>rows.delete(key)};}

test('expression extraction keeps an unknown identity until a terminal no-effect receipt',async()=>{
 const storage=memoryStorage(),keys=[];let fail=true;
 const command=persistentCommand({key:'expression',storage:()=>storage,preAdmissionConflicts:[],execute:async(_body,key)=>{keys.push(key);if(fail)throw Object.assign(new Error('connection lost'),{status:0});return {status:'completed'};}});
 await assert.rejects(command.submit({source:'message-1'}));
 const original=command.pending().idempotencyKey;
 await command.confirmNoEffect(async()=>({status:'not_found',no_effect:false}));
 assert.equal(command.pending().idempotencyKey,original);
 assert.throws(()=>command.releaseRejected(),/尚未确认/);
 await command.confirmNoEffect(async()=>({status:'running',no_effect:false}));
 assert.equal(command.pending().idempotencyKey,original);
 await command.confirmNoEffect(async()=>({status:'failed',no_effect:true}));
 assert.equal(command.pending().rejection.confirmedNoEffect,true);
 command.releaseRejected();
 fail=false;
 await command.submit({source:'message-1'});
 assert.notEqual(keys.at(-1),original);
 assert.equal(command.pending(),null);
});

test('a source-change response does not prove the extraction was never admitted',async()=>{
 const storage=memoryStorage();
 const command=persistentCommand({key:'expression',storage:()=>storage,preAdmissionConflicts:[],execute:async()=>{throw Object.assign(new Error('source changed'),{status:409,code:'expression_source_changed'});}});
 await assert.rejects(command.submit({source:'message-1'}));
 assert.equal(command.pending().rejection,undefined);
 assert.throws(()=>command.releaseRejected(),/尚未确认/);
 await command.confirmNoEffect(async()=>({status:'failed',no_effect:true}));
 command.releaseRejected();
 assert.equal(command.pending(),null);
});

test('a restored old source-change rejection must be reconciled under the tightened rule',async()=>{
 const storage=memoryStorage();
 const execute=async()=>{throw Object.assign(new Error('source changed'),{status:409,code:'expression_source_changed'});};
 const old=persistentCommand({key:'expression',storage:()=>storage,preAdmissionConflicts:['expression_source_changed'],execute});
 await assert.rejects(old.submit({source:'message-1'}));
 const original=old.pending().idempotencyKey;
 const current=persistentCommand({key:'expression',storage:()=>storage,preAdmissionConflicts:[],execute});
 assert.equal(current.pending().idempotencyKey,original);
 assert.equal(current.pending().rejection,undefined);
 assert.throws(()=>current.releaseRejected(),/尚未确认/);
 await current.confirmNoEffect(async()=>({status:'failed',no_effect:true}));
 current.releaseRejected();
 assert.equal(current.pending(),null);
});

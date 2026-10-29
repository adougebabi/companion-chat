import test from 'node:test';
import assert from 'node:assert/strict';
import {inspectCreationRecovery,requireCreationRecovery} from '../src/approved-ui/creation-recovery-state.mjs';
import {creationRecovery,migrateCreationRecovery} from '../src/approved-ui/creation-recovery.mjs';
import {memoryStorage} from './support/runtime.mjs';
const owner='recovery-owner',draftKey=`fluctlight-creation:${owner}:draft`,commandKey=`fluctlight-creation:${owner}:command`;
const draft=(patch={})=>({version:1,step:1,requestId:'original-request',mode:'blank',name:'保留的名字',analysis:null,activatedId:null,...patch});
const command=(patch={})=>({version:1,idempotencyKey:'original-key',attempts:1,value:{body:{requestId:'original-request',name:'原提交的名字'}},...patch});
function fixture(raw,cmd){const storage=memoryStorage();if(raw!==undefined)storage.setItem(draftKey,typeof raw==='string'?raw:JSON.stringify(raw));if(cmd!==undefined)storage.setItem(commandKey,typeof cmd==='string'?cmd:JSON.stringify(cmd));return {storage,read:()=>inspectCreationRecovery({owner,storage})};}
test('creation entry distinguishes editable draft, original unknown admission and accepted preparation without writing storage',()=>{
 for(const [raw,cmd,state,label] of [[draft(),undefined,'draft','继续创建'],[draft(),command(),'activation-unknown','核对创建结果'],[draft({activatedId:'role'}),command(),'preparation','查看准备进度']]){
  const f=fixture(raw,cmd),before=[f.storage.getItem(draftKey),f.storage.getItem(commandKey)],result=f.read();assert.equal(result.state,state);assert.equal(result.actionLabel,label);assert.equal(result.blocked,false);assert.deepEqual([f.storage.getItem(draftKey),f.storage.getItem(commandKey)],before);
 }
 assert.equal(fixture().read().state,'empty');
});
test('malformed, incompatible and incomplete creation records retain exact bytes and never offer an executable continue action',()=>{
 for(const [raw,cmd,code] of [['',undefined,'creation_draft_json'],['{unreadable',undefined,'creation_draft_json'],[draft({version:2}),undefined,'creation_draft_version'],[draft({analysis:{}}),undefined,'creation_draft_invalid'],[draft({activatedId:42}),undefined,'creation_draft_invalid'],[draft({previewDraft:[null]}),undefined,'creation_draft_invalid'],[draft(),'broken-command','creation_command_invalid'],[undefined,command(),'creation_command_scope'],[draft({requestId:'newer-draft'}),command(),'creation_command_scope']]){
  const f=fixture(raw,cmd),before=[f.storage.getItem(draftKey),f.storage.getItem(commandKey)],result=f.read();assert.equal(result.state,code);assert.equal(result.blocked,true);assert.equal(result.href,undefined);assert.deepEqual([f.storage.getItem(draftKey),f.storage.getItem(commandKey)],before);assert.throws(()=>requireCreationRecovery({owner,storage:f.storage}));assert.deepEqual([f.storage.getItem(draftKey),f.storage.getItem(commandKey)],before);
 }
});
test('temporary storage failure is distinct and an authoritative reread can clear it without deleting a draft',()=>{
 const f=fixture(draft()),read=f.storage.getItem;let failed=true;f.storage.getItem=key=>{if(failed)throw new DOMException('Storage access unavailable','SecurityError');return read(key);};
 assert.equal(f.read().state,'creation_storage_unavailable');failed=false;assert.equal(f.read().state,'draft');assert.equal(JSON.parse(read(draftKey)).requestId,'original-request');
});
test('valid archived creation remains accessible when the independent current draft is corrupt',()=>{
 const f=fixture('{broken-current'),key=`fluctlight-creation:${owner}:history:old-role`,archived=draft({activatedId:'old-role',step:3});f.storage.setItem(key,JSON.stringify(archived));
 const recovery=creationRecovery({owner,storage:f.storage,query:new URLSearchParams({resume:'old-role'})});assert.equal(recovery.read().activatedId,'old-role');assert.equal(recovery.archived,true);assert.equal(f.storage.getItem(draftKey),'{broken-current');
 assert.equal(migrateCreationRecovery(owner,f.storage,memoryStorage(),{resume:'old-role'}).read().activatedId,'old-role');assert.throws(()=>creationRecovery({owner,storage:f.storage,query:new URLSearchParams({resume:'missing-role'})}));
});
test('an orphaned legacy session command blocks creation without pairing it to a new durable draft',()=>{
 const current=memoryStorage(),previous=memoryStorage(),original=JSON.stringify(command());previous.setItem(commandKey,original);
 for(const existing of [null,JSON.stringify(draft({requestId:'different-current'}))]){
  if(existing!==null)current.setItem(draftKey,existing);
  assert.throws(()=>migrateCreationRecovery(owner,current,previous),e=>e.code==='creation_command_scope'&&e.records[0].value===original);
  assert.equal(previous.getItem(commandKey),original);assert.equal(current.getItem(commandKey),null);assert.equal(current.getItem(draftKey),existing);
 }
});

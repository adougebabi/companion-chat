import test from 'node:test';
import assert from 'node:assert/strict';
import {creationDraft,checkpointCreation} from '../src/v3/creation-draft.mjs';
import {creationAnalysisRequest,parseCreationInput} from '../src/v3/creation-json.mjs';
import {persistentCommand} from '../src/v3/providers/persistent-command.mjs';

const storage=()=>{const data=new Map();return {getItem:k=>data.get(k)??null,setItem:(k,v)=>data.set(k,v),removeItem:k=>data.delete(k)};};
test('creation sources distinguish raw JSON, immutable publication and restored analysis authority',()=>{
 const json={core_persona:{identity:{name:'Source <script>'}}};
 assert.deepEqual(creationAnalysisRequest({mode:'json',json:JSON.stringify(json)}),{json});
 assert.deepEqual(creationAnalysisRequest({mode:'template',templateId:'resource',templateVersion:'1',body:{description:'new draft is irrelevant'}}),{resourceId:'resource',resourceVersion:'1'});
 assert.throws(()=>creationAnalysisRequest({mode:'analysis'}),/无需/);
 assert.throws(()=>creationAnalysisRequest({mode:'template',templateId:'resource'}),/发布/);
 for(const field of ['analysis_id','id','messages','current_state'])assert.throws(()=>parseCreationInput(JSON.stringify({...json,[field]:'archive'})),/运行存档/);
 assert.throws(()=>parseCreationInput(JSON.stringify({description:'中'.repeat(21000)})),/UTF-8/);
});
test('refresh keeps source, edited preview, original request and activated grouping checkpoint in the same Owner scope',()=>{
 const saved=storage(),first=creationDraft({owner:'owner-a',storage:saved,timezone:'Asia/Shanghai'}),draft=first.read();
 Object.assign(draft,{mode:'template',templateId:'resource',templateVersion:'1',step:3,analysisText:'edited preview',groupId:'group',activatedId:'created-character',activationGroupId:'group'});first.save(draft);
 const restored=creationDraft({owner:'owner-a',storage:saved}).read();
 assert.deepEqual(restored,draft);
 assert.notEqual(creationDraft({owner:'owner-b',storage:saved}).read().requestId,draft.requestId);
 assert.throws(()=>creationDraft({owner:'',storage:saved}),/登录/);
});
test('unknown activation response replays immutable payload despite newer draft or refresh',async()=>{
 const saved=storage(),recovery=creationDraft({owner:'owner',storage:saved}),draft=recovery.read(),calls=[];
 const body={requestId:draft.requestId,initializationMode:'blank_slate',name:'Original'};
 let unknown=true;
 const make=()=>persistentCommand({key:recovery.commandKey,storage:()=>saved,execute:async(value,key)=>{calls.push({value,key});if(unknown)throw new Error('response lost');return {id:'actual-character'};}});
 await assert.rejects(make().submit({body,groupId:'group'}),/response lost/);
 draft.name='Newer local draft';recovery.save(draft);
 await assert.rejects(make().submit({body:{...body,name:'Newer local draft'},groupId:'group'}),/原提议/);
 unknown=false;assert.deepEqual(await make().retry(),{id:'actual-character'});
 assert.equal(calls[0].key,calls[1].key);assert.deepEqual(calls[0].value,calls[1].value);
 assert.equal(recovery.read().name,'Newer local draft');
});
test('only the first known stale-analysis rejection permits explicit new analysis',async()=>{
 const saved=storage(),key='creation-rejection';let unavailable=false;
 const make=()=>persistentCommand({key,storage:()=>saved,preAdmissionConflicts:['activation_analysis_stale'],execute:async()=>{if(unavailable)throw new Error('unknown response');throw Object.assign(new Error('stale source'),{status:409,code:'activation_analysis_stale'});}});
 await assert.rejects(make().submit({body:{requestId:'first'}}),/stale/);assert.ok(make().pending().rejection);
 make().releaseRejected();assert.equal(make().pending(),null);
 unavailable=true;await assert.rejects(make().submit({body:{requestId:'second'}}),/unknown/);
 unavailable=false;await assert.rejects(make().retry(),/stale/);assert.equal(make().pending().rejection,undefined);
 assert.throws(()=>make().releaseRejected(),/尚未确认/);
});

test('activation after page disposal checkpoints before releasing original request and cannot be erased by an older page',async()=>{
 const saved=storage(),recovery=creationDraft({owner:'owner',storage:saved}),draft=recovery.read();recovery.save(draft);
 const submitted={body:{requestId:draft.requestId,initializationMode:'blank_slate',name:'Original'},groupId:'group-deleted-later'};
 let finish;const response=new Promise(resolve=>{finish=resolve;});
 const command=persistentCommand({key:recovery.commandKey,storage:()=>saved,execute:async value=>{const result=await response;checkpointCreation(recovery,value,result,()=>true);return result;}});
 const pending=command.submit(submitted);finish({id:'actual-created'});await pending;
 assert.equal(command.pending(),null);assert.equal(recovery.read().activatedId,'actual-created');
 recovery.save(draft);assert.equal(recovery.read().activatedId,'actual-created');
 assert.throws(()=>recovery.save({...draft,requestId:'new-id'}),/原角色已经激活，请先完成该角色的分组核对/);
 // An explicit replacement group only changes grouping on the same character.
 recovery.save({...recovery.read(),activationGroupId:''});assert.equal(recovery.read().activatedId,'actual-created');assert.equal(recovery.read().activationGroupId,'');
 const original=JSON.stringify(recovery.read());
 checkpointCreation(recovery,submitted,{id:'late-wrong-owner'},()=>false);assert.equal(JSON.stringify(recovery.read()),original);
 const other=creationDraft({owner:'other',storage:saved});checkpointCreation(other,submitted,{id:'late-new-draft'},()=>true);assert.equal(other.read().activatedId,null);
});

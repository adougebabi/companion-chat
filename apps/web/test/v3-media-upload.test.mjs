import test from 'node:test';
import assert from 'node:assert/strict';
import {mediaUploadController,describeMediaFile} from '../src/v3/providers/media-upload.mjs';
const memory=()=>{const m=new Map();return {getItem:k=>m.get(k)??null,setItem:(k,v)=>m.set(k,v),removeItem:k=>m.delete(k)};};
const file=()=>new File([new Uint8Array([137,80,78,71,13,10,26,10])],'original.png',{type:'image/png'});
async function fixture(){const storage=memory(),selected=file(),description=await describeMediaFile(selected),asset={id:'real-asset',origin:'owner_upload',status:'ready',source_filename:description.filename,sha256:description.sha256,byte_size:description.size};return {storage,selected,asset,description};}
test('media upload response loss recovers by original key without retaining or resending a File after refresh',async()=>{
 const f=await fixture(),keys=[];let admitted=false,uploads=0;
 const client={lookup:async key=>{keys.push(key);if(!admitted)throw Object.assign(new Error('missing'),{status:404});return {status:'ready',asset:f.asset};},upload:async()=>{uploads++;admitted=true;throw new TypeError('response lost');}};
 const options={slot:'owner/upload',storage:()=>f.storage,client,isOwner:()=>true};
 const first=mediaUploadController(options);first.select(f.selected);await assert.rejects(first.submit(),/response lost/);const pending=first.pending();assert.equal(pending.value.sha256,f.description.sha256);assert.equal(pending.value.file,undefined);
 const reloaded=mediaUploadController(options);const result=await reloaded.retry();assert.equal(result.id,'real-asset');assert.equal(uploads,1);assert.deepEqual(keys,[pending.idempotencyKey,pending.idempotencyKey]);assert.equal(reloaded.pending(),null);assert.equal(reloaded.lastAsset(),'real-asset');
});
test('unsettled upload needs the exact file and never switches identity on replacement bytes',async()=>{
 const f=await fixture(),sent=[];const client={lookup:async()=>{throw Object.assign(new Error('missing'),{status:404});},upload:async(file,description,key)=>{sent.push(key);throw new TypeError('storage disconnected');}};
 const opts={slot:'owner/upload',storage:()=>f.storage,client,isOwner:()=>true};const first=mediaUploadController(opts);first.select(f.selected);await assert.rejects(first.submit());const id=first.pending().idempotencyKey;
 const fresh=mediaUploadController(opts);await assert.rejects(fresh.retry(),/重新选择同一文件/);fresh.select(new File(['different'],'original.png',{type:'image/png'}));await assert.rejects(fresh.retry(),/原提交的名称、类型或内容不同/);assert.equal(sent.length,1);assert.equal(fresh.pending().idempotencyKey,id);
});
test('late media result cannot resurrect a logged-out Owner result pointer',async()=>{
 const f=await fixture();let finish,owner=true;
 const opts={slot:'owner/upload',storage:()=>f.storage,isOwner:()=>owner,client:{lookup:()=>new Promise(resolve=>{finish=resolve;})}};
 const upload=mediaUploadController(opts);upload.select(f.selected);const work=upload.submit();while(!finish)await new Promise(resolve=>setTimeout(resolve,0));owner=false;f.storage.removeItem('owner/upload:command');finish({status:'ready',asset:f.asset});await work;assert.equal(f.storage.getItem('owner/upload:result'),null);assert.equal(upload.pending(),null);
});
test('media upload rejects unsafe file size and does not discard a key reuse ambiguity',async()=>{
 await assert.rejects(describeMediaFile(new File([],'empty.png')));await assert.rejects(describeMediaFile({size:64*1024*1024+1}));
 const f=await fixture();const upload=mediaUploadController({slot:'owner/upload',storage:()=>f.storage,isOwner:()=>true,client:{lookup:async()=>{throw Object.assign(new Error('original key bound'),{status:422,code:'media_upload_key_reused'});}}});upload.select(f.selected);await assert.rejects(upload.submit());assert.ok(upload.pending());
});
test('a late missing-upload lookup cannot send the old file with a replacement Owner session',async()=>{
 const f=await fixture();let rejectLookup,owner=true,uploads=0;
 const upload=mediaUploadController({slot:'owner/upload',storage:()=>f.storage,isOwner:()=>owner,client:{lookup:()=>new Promise((_,reject)=>{rejectLookup=reject;}),upload:async()=>{uploads++;return {};}}});
 upload.select(f.selected);const work=upload.submit();while(!rejectLookup)await new Promise(resolve=>setTimeout(resolve,0));owner=false;f.storage.removeItem('owner/upload:command');rejectLookup(Object.assign(new Error('missing'),{status:404}));await assert.rejects(work,/会话或原提交已切换/);assert.equal(uploads,0);assert.equal(upload.pending(),null);
});

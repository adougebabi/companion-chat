import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {persistentCommand} from '../src/v3/providers/persistent-command.mjs';
test('retrying an unknown original metadata command preserves the newer persisted draft',async()=>{
 const values=new Map(),storage={getItem:k=>values.get(k)??null,setItem:(k,v)=>values.set(k,v),removeItem:k=>values.delete(k)},slot='fluctlight-media:owner:asset:metadata';
 const original={title:'original proposal',hidden:false,reason:'original reason',expectedAssetRevision:1,expectedMetadataRevision:0},draft={...original,title:'newer local draft',reason:'keep my edits'};
 storage.setItem(slot+':command',JSON.stringify({version:1,idempotencyKey:'original-key',attempts:1,value:original}));storage.setItem(slot+':draft',JSON.stringify(draft));
 const elements=Object.fromEntries(['title','hidden','reason'].map(name=>[name,{value:draft[name],checked:draft[name]}])),nodes=new Map();
 const host={isConnected:true,innerHTML:'',querySelector(selector){if(!nodes.has(selector))nodes.set(selector,selector==='form'?{elements,reportValidity:()=>true}:{disabled:false,textContent:''});return nodes.get(selector);}};
 let savedBody,finished=false,reads=0;
 const source=readFileSync(new URL('../src/v3/proto/components/media-metadata.js',import.meta.url),'utf8').replace(/^import .+;\s*$/gm,'').replaceAll('export function ','function ');
 const bindings={Field:()=>'',Toggle:()=>'',esc:String,confirm:async()=>true,get:()=>({actorId:'owner'}),mediaClient:{save:async(id,body)=>{savedBody=body;return {id,title:body.title,hidden:body.hidden,asset_revision:1,metadata_revision:1};},asset:async id=>{reads++;return {id,title:savedBody.title,hidden:savedBody.hidden,asset_revision:1,metadata_revision:1};}},persistentCommand,studioDraftStorage:()=>storage};
 const mount=Function(...Object.keys(bindings),source+'\nreturn mountMediaMetadata;')(...Object.values(bindings));
 mount(host,{asset:{id:'asset',title:'server',hidden:false,asset_revision:1,metadata_revision:0},onSaved:()=>{finished=true;}});host.querySelector('[data-retry]').onclick();
 for(let attempt=0;attempt<100&&!finished;attempt++)await new Promise(resolve=>setImmediate(resolve));
 assert.equal(finished,true,'metadata save did not finish: '+host.querySelector('[data-error]').textContent);assert.equal(reads,1,'successful save must read back actual asset');
 assert.equal(savedBody.title,'original proposal');assert.equal(savedBody.idempotencyKey,'original-key');assert.deepEqual(JSON.parse(storage.getItem(slot+':draft')),draft);assert.equal(elements.title.value,'newer local draft');assert.match(host.querySelector('[data-error]').textContent,/新草稿已保留/);
});

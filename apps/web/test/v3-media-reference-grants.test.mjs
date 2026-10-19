import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {persistentCommand} from '../src/v3/providers/persistent-command.mjs';
test('unknown grant creation retries the exact original target/use while newer choices remain a draft',async()=>{
 const values=new Map(),storage={getItem:k=>values.get(k)??null,setItem:(k,v)=>values.set(k,v),removeItem:k=>values.delete(k)},slot='fluctlight-media:owner:image:grants';
 const original={assetId:'image',expectedAssetRevision:1,targetScope:'studio_resource',targetId:'original-resource',purpose:'person',confirm:true,reason:'Original permission'},draft={targetId:'new-resource',targetName:'New resource',purpose:'object',confirm:true,reason:'New permission'};
 storage.setItem(slot+':command',JSON.stringify({version:1,idempotencyKey:'original-grant-key',attempts:1,value:{op:'create',body:original}}));storage.setItem(slot+':draft',JSON.stringify(draft));
 const nodes=new Map(),elements={purpose:{value:'object'},confirm:{checked:true},reason:{value:'New permission',reportValidity:()=>true}},host={isConnected:true,innerHTML:'',querySelector(selector){if(!nodes.has(selector))nodes.set(selector,selector==='[data-grant-form]'?{elements,reportValidity:()=>true}:{disabled:false,textContent:'',innerHTML:'',querySelectorAll:()=>[]});return nodes.get(selector);}};let sent,settled=false;
 const source=readFileSync(new URL('../src/v3/proto/components/media-reference-grants.js',import.meta.url),'utf8').replace(/^import .+;\s*$/gm,'').replaceAll('export function ','function ').replaceAll('export const ','const ');
 const bindings={get:key=>key==='session'?{actorId:'owner'}:{},mediaClient:{createGrant:async body=>{sent=body;return {id:'grant'};}},studioClient:{},esc:String,confirm:async()=>true,Field:()=>'',SearchForm:()=>'',Toggle:()=>'',mountCursorRecords:()=>({dispose(){},reload:async()=>{settled=true;}}),JsonDetails:JSON.stringify,persistentCommand,studioDraftStorage:()=>storage};
 const mount=Function(...Object.keys(bindings),source+'\nreturn mountReferenceGrantManager;')(...Object.values(bindings));mount(host,{asset:{id:'image',readable:true,kind:'image',byte_size:68,asset_revision:1}});host.querySelector('[data-retry]').onclick();while(!settled)await new Promise(resolve=>setImmediate(resolve));
 assert.deepEqual(sent,{...original,idempotencyKey:'original-grant-key'});assert.deepEqual(JSON.parse(storage.getItem(slot+':draft')),draft);assert.equal(elements.reason.value,'New permission');
});

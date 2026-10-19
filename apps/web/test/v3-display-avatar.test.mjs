import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {persistentCommand} from '../src/v3/providers/persistent-command.mjs';
function fixture(client){
 const values=new Map(),storage={getItem:k=>values.get(k)??null,setItem:(k,v)=>values.set(k,v),removeItem:k=>values.delete(k)},nodes=new Map(),elements={mode:{value:'asset'},reason:{value:'New reason'}};
 const host={isConnected:true,innerHTML:'',querySelector(selector){if(!nodes.has(selector))nodes.set(selector,selector==='[data-avatar-command]'?{elements,reportValidity:()=>true}:{disabled:false,textContent:''});return nodes.get(selector);}};
 const source=readFileSync(new URL('../src/v3/proto/components/display-avatar.js',import.meta.url),'utf8').replace(/^import .+;\s*$/gm,'').replaceAll('export function ','function ');
 const bindings={get:key=>key==='session'?{actorId:'owner'}:{status:'正常'},mediaClient:client,Field:()=>'',SearchForm:()=>'',esc:String,confirm:async()=>true,mountCursorRecords:()=>({dispose(){}}),MediaAssetCard:()=>'',bindMediaFailures:()=>{},persistentCommand,studioDraftStorage:()=>storage};
 const mount=Function(...Object.keys(bindings),source+'\nreturn mountDisplayAvatar;')(...Object.values(bindings));return {storage,host,mount,values,slot:'fluctlight-media:owner:role:avatar'};
}
test('display avatar retry uses original asset and preserves a newer selection and reason',async()=>{
 let sent,done=false;const f=fixture({selectAvatar:async(fid,body)=>{sent=body;return {revision:1,mode:'asset',selection:{asset_id:body.assetId}};}}),original={mode:'asset',assetId:'original-image',expectedAssetRevision:1,expectedRevision:0,reason:'Original reason'},draft={mode:'asset',selected:{id:'new-image',asset_revision:1,title:'New choice'},expectedRevision:0,reason:'New reason'};
 f.storage.setItem(f.slot+':command',JSON.stringify({version:1,idempotencyKey:'original-key',attempts:1,value:original}));f.storage.setItem(f.slot+':draft',JSON.stringify(draft));f.mount(f.host,{fid:'role',snapshot:{mode:'canonical',revision:0},onSaved:()=>{done=true;}});f.host.querySelector('[data-retry]').onclick();while(!done)await new Promise(resolve=>setImmediate(resolve));
 assert.equal(sent.assetId,'original-image');assert.equal(sent.idempotencyKey,'original-key');assert.deepEqual(JSON.parse(f.storage.getItem(f.slot+':draft')),draft);assert.match(f.host.querySelector('[data-error]').textContent,/新选择或理由已保留/);
});
test('disposed display editor cannot recreate cleared draft when an accepted response arrives',async()=>{
 let finish;const f=fixture({selectAvatar:()=>new Promise(resolve=>{finish=resolve;})});f.storage.setItem(f.slot+':draft',JSON.stringify({mode:'canonical',selected:null,expectedRevision:0,reason:'Original'}));const panel=f.mount(f.host,{fid:'role',snapshot:{mode:'canonical',revision:0}});f.host.querySelector('[data-avatar-command]').onsubmit({preventDefault(){}});while(!finish)await new Promise(resolve=>setImmediate(resolve));panel.dispose();f.values.clear();finish({revision:1,mode:'canonical'});await new Promise(resolve=>setImmediate(resolve));assert.equal(f.values.size,0);
});

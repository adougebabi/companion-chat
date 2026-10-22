import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {errorMessage,stateLabel} from '../src/v3/domain-presentation.mjs';
import {persistentCommand} from '../src/v3/providers/persistent-command.mjs';
const tick=()=>new Promise(resolve=>setImmediate(resolve));
function fixture(client,{inventory={}}={}){
 const values=new Map(),storage={getItem:k=>values.get(k)??null,setItem:(k,v)=>values.set(k,v),removeItem:k=>values.delete(k)},listeners=new Map();let owner='owner',previews=0,confirmations=0;
 const elements={prompt:{value:''},reason:{value:''},place:{value:'',innerHTML:''}};
 function host(){const nodes=new Map();return {isConnected:true,innerHTML:'',nodes,querySelector(selector){if(!nodes.has(selector))nodes.set(selector,selector==='form'?{elements,reportValidity:()=>true}:host());return nodes.get(selector);},querySelectorAll:()=>[],disabled:false,textContent:'',setAttribute(){}};}
 const source=readFileSync(new URL('../src/v3/proto/components/media-generation.js',import.meta.url),'utf8').replace(/^import .+;\s*$/gm,'').replaceAll('export function ','function ');
 const bindings={errorMessage,stateLabel,mountReferenceGrantPicker:()=>({value(){},dispose(){}}),referenceFromGrant:g=>({assetId:g.asset_id,purpose:g.purpose,grantId:g.id,grantRevision:g.revision}),get:key=>key==='session'?{actorId:owner}:{status:'正常'},mediaClient:{locations:async()=>({available:true,places:[]}),generation:async()=>({fluctlight_id:'role',request:{id:'actual-request',status:'accepted',tasks:[]},media:{id:'actual-intent',status:'pending'},asset:null}),...client},inventoryClient:inventory,esc:String,confirm:async()=>{confirmations++;return true;},Field:()=>'',SearchForm:()=>'',JsonDetails:JSON.stringify,MediaPreview:()=>{previews++;return 'actual-preview';},bindMediaFailures:()=>{},mediaDetailPath:()=>'',mountCursorRecords:()=>({dispose(){},reload:async()=>{}}),persistentCommand,studioDraftStorage:()=>storage,document:{hidden:false,addEventListener:(k,v)=>listeners.set(k,v),removeEventListener:k=>listeners.delete(k)},setTimeout:()=>1,clearTimeout:()=>{}};
 const mount=Function(...Object.keys(bindings),source+'\nreturn mountOwnerMediaRequest;')(...Object.values(bindings));return {values,storage,elements,mount,host:host(),owner(value){owner=value;},previews:()=>previews,confirmations:()=>confirmations,listeners};
}
test('original image acceptance retry preserves a newer draft and never presents an intent as image pixels',async()=>{
 const calls=[];let first=true,done=false;const f=fixture({generate:async(fid,body)=>{calls.push(structuredClone(body));if(first){first=false;throw new TypeError('response lost');}return {request:{id:'actual-request',fluctlight_id:fid,status:'pending'}};}}),slot='fluctlight-media:owner:role:request:general';
 f.storage.setItem(slot+':draft',JSON.stringify({prompt:'Original image',reason:'Original reason',item:null,place:null}));const panel=f.mount(f.host,{fid:'role',onSubmitted:()=>{done=true;}});
 f.host.querySelector('form').onsubmit({preventDefault(){}});await tick();await tick();assert.equal(calls.length,1);
 f.elements.prompt.value='Newer image draft';f.elements.prompt.oninput();f.host.querySelector('[data-retry]').onclick();while(!done)await tick();await tick();
 assert.deepEqual(calls[1],calls[0]);assert.equal(JSON.parse(f.storage.getItem(slot+':draft')).prompt,'Newer image draft');assert.equal(JSON.parse(f.storage.getItem(slot+':result')).requestId,'actual-request');assert.equal(f.previews(),0);assert.match(f.host.querySelector('[data-result]').innerHTML,/生成任务等待执行/);panel.dispose();assert.equal(f.listeners.size,0);
});
test('image command accepted after logout/disposal cannot recreate the previous Owner checkpoint or draft',async()=>{
 let finish;const f=fixture({generate:()=>new Promise(resolve=>{finish=resolve;})}),slot='fluctlight-media:owner:role:request:general';f.storage.setItem(slot+':draft',JSON.stringify({prompt:'Original',reason:'Reason',item:null,place:null}));const panel=f.mount(f.host,{fid:'role'});
 f.host.querySelector('form').onsubmit({preventDefault(){}});while(!finish)await tick();panel.dispose();f.owner('replacement');f.values.clear();finish({request:{id:'old-request',fluctlight_id:'role',status:'pending'}});await tick();assert.equal(f.values.size,0);
});

test('failed fresh location reads preserve a rejected original identity and never claim a successful rebase',async()=>{
 let reads=0;const f=fixture({locations:async()=>{if(++reads>1)throw new Error('fresh location read offline');return {available:true,places:[{id:'place',revision:1,name:'Old place',status:'active'}]};}}),slot='fluctlight-media:owner:role:request:general',draft={prompt:'Picture',reason:'Reason',item:null,place:{id:'place',revision:1,name:'Old place'}};
 f.storage.setItem(slot+':draft',JSON.stringify(draft));f.storage.setItem(slot+':command',JSON.stringify({version:1,idempotencyKey:'rejected-key',attempts:1,value:{prompt:'Picture',reason:'Reason',placeId:'place',expectedPlaceRevision:1},rejection:{status:409,code:'revision_conflict'}}));
 const panel=f.mount(f.host,{fid:'role'});await tick();await f.host.querySelector('[data-rebase]').onclick();assert.equal(f.confirmations(),0);assert.ok(JSON.parse(f.storage.getItem(slot+':command')).rejection);assert.equal(JSON.parse(f.storage.getItem(slot+':draft')).place.revision,1);assert.match(f.host.querySelector('[data-error]').textContent,/offline/);panel.dispose();
});
test('a delayed source rebase cannot restore an item that the user cleared while the read was pending',async()=>{
 let finish;const f=fixture({}, {inventory:{read:()=>new Promise(resolve=>{finish=resolve;})}}),slot='fluctlight-media:owner:role:request:general';f.storage.setItem(slot+':draft',JSON.stringify({prompt:'Picture',reason:'Reason',item:{id:'item-a',revision:1,description:'A'},place:null}));const panel=f.mount(f.host,{fid:'role'});await tick();
 const rebasing=f.host.querySelector('[data-rebase]').onclick();while(!finish)await tick();f.host.querySelector('[data-clear-item]').onclick();finish({id:'item-a',revision:2,description:'Latest A'});await rebasing;
 assert.equal(JSON.parse(f.storage.getItem(slot+':draft')).item,null);assert.equal(f.confirmations(),0);assert.match(f.host.querySelector('[data-error]').textContent,/新选择已保留/);panel.dispose();
});

test('resetting the current image draft does not erase an unknown original command identity',async()=>{
 const f=fixture({}),slot='fluctlight-media:owner:role:request:general';f.storage.setItem(slot+':draft',JSON.stringify({prompt:'Newer',reason:'New reason',item:null,place:null}));f.storage.setItem(slot+':command',JSON.stringify({version:1,idempotencyKey:'still-original',attempts:1,value:{prompt:'Original',reason:'Original reason'}}));const panel=f.mount(f.host,{fid:'role'});f.host.querySelector('[data-reset]').onclick();assert.equal(JSON.parse(f.storage.getItem(slot+':command')).idempotencyKey,'still-original');assert.equal(JSON.parse(f.storage.getItem(slot+':draft')).prompt,'');assert.match(f.host.querySelector('[data-error]').textContent,/原提交身份/);panel.dispose();
});

test('worker completion cannot hide a failed business image result near the submission',async()=>{
 const f=fixture({generation:async()=>({fluctlight_id:'role',request:{id:'failed-request',status:'completed',tasks:[{task_id:'worker',status:'completed'}],result:{disposition:'not_applied',error_code:'media_generation_unavailable'},decision_reason:'当前无法调用图像生成能力'},media:null,asset:null})});
 const panel=f.mount(f.host,{fid:'role',requestId:'failed-request'});await tick();
 const summary=f.host.querySelector('[data-business-result]');assert.equal(summary.className,'banner err');assert.match(summary.textContent,/当前无法调用图像生成能力/);assert.match(f.host.querySelector('[data-result]').innerHTML,/执行已结束（图片结果另行核对）/);assert.equal(f.previews(),0);panel.dispose();
});

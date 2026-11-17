import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {installDOM} from './helpers/react-dom.mjs';

const env=installDOM();
const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const shapes=JSON.parse(await readFile(new URL('./fixtures/p206a-shapes.json',import.meta.url),'utf8'));
const detail=Object.values(shapes)[0],catalog=Object.values(shapes)[1],items=Object.values(shapes)[2],fluctlights=Object.values(shapes)[3];
const {BrowserClient}=await vite.ssrLoadModule('/@fs/'+fileURLToPath(new URL('../../../packages/browser-client/src/index.ts',import.meta.url)));
const {createWorldProvider}=await vite.ssrLoadModule('/src/v3/providers/world.ts');
const {viewFromBundle}=await vite.ssrLoadModule('/src/v3/contract-map.mjs');
const store=await vite.ssrLoadModule('/src/v3/proto/store.js');
const {renderWorldItems}=await vite.ssrLoadModule('/src/v3-domain/world-items.js');
const originalFetch=globalThis.fetch;
const tick=()=>new Promise(resolve=>setImmediate(resolve));
const deferred=()=>{let resolve;const promise=new Promise(r=>{resolve=r;});return {promise,resolve};};
const json=(body,status=200)=>new Response(JSON.stringify(body),{status,headers:{'Content-Type':'application/json'}});
function disabled(el,value){
  const controls=el.querySelectorAll('[data-create], [data-search] input, [data-search] button');
  assert.ok(controls.length>=3);
  for(const control of controls)assert.equal(control.disabled,value);
}
function fixture({selected=true,failCatalog=false,waitCatalog=null}={}){
  const actualDetail=structuredClone(detail);
  if(!selected)delete actualDetail.context.world_id;
  const snapshot=viewFromBundle({fluctlights,details:{[detail.id]:actualDetail}});
  store.replaceSnapshot({...snapshot,session:{actorId:'p206a-owner'}});
  assert.equal(store.get('characters',detail.id).worldId,undefined);
  let failItems=false,waitItems=null;
  const calls=[];
  globalThis.fetch=async(url,init={})=>{
    assert.equal(init.method||'GET','GET','only reads are allowed');
    const path=new URL(String(url),'http://world.test').pathname;calls.push(path);
    if(path.endsWith('/world/catalog')){
      if(waitCatalog)await waitCatalog;
      return failCatalog?json({message:'INTERNAL private_key'},503):json({...catalog,active_binding:selected?catalog.active_binding:null});
    }
    if(path.endsWith('/world/items')){
      if(waitItems)await waitItems;
      return failItems?json({message:'Service Unavailable private_key'},503):json(items);
    }
    throw Error('Unexpected read '+path);
  };
  const provider=createWorldProvider({client:new BrowserClient('http://world.test'),refreshProduct:async()=>{}});provider.install();
  const el=document.createElement('div');document.body.append(el);
  const dispose=renderWorldItems(el,{charId:detail.id,query:{}});
  return {el,calls,set failItems(value){failItems=value;},set failCatalog(value){failCatalog=value;},set waitItems(value){waitItems=value;},set waitCatalog(value){waitCatalog=value;},dispose(){dispose();el.remove();store.replaceSnapshot({...store.get(),session:null});}};
}
test.after(async()=>{globalThis.fetch=originalFetch;await vite.close();env.restore();});

test('real detail without top-level world_id uses catalog binding and requests both registered items',async()=>{
  assert.equal(Object.hasOwn(detail,'world_id'),false);
  assert.equal(detail.context.world_id,catalog.active_binding.world_id);
  const f=fixture();
  try{
    disabled(f.el,true);await tick();
    assert.ok(f.calls.some(path=>path.endsWith('/world/catalog')));
    assert.equal(f.calls.filter(path=>path.endsWith('/world/items')).length,1);
    assert.equal(f.el.querySelectorAll('[data-items] h3').length,2);
    for(const item of items.items)assert.ok(f.el.textContent.includes(item.name));
    assert.doesNotMatch(f.el.textContent,/尚未选择运行世界/);disabled(f.el,false);
  }finally{f.dispose();}
});

test('empty active binding and absent context world show missing-world state without reading items',async()=>{
  const f=fixture({selected:false});
  try{
    await tick();assert.match(f.el.textContent,/尚未选择运行世界/);
    assert.equal(f.calls.filter(path=>path.endsWith('/world/items')).length,0);
    assert.equal(f.el.querySelector('[data-create], [data-search]'),null);
  }finally{f.dispose();}
});

test('selected world keeps item 503 and retry inside card, disables controls while reading or failed, then recovers',async()=>{
  const pending=deferred(),f=fixture();f.failItems=true;f.waitItems=pending.promise;
  try{
    await tick();disabled(f.el,true);
    assert.ok(f.el.querySelector('[data-items] [role=status]'));
    pending.resolve();await tick();
    const failure=f.el.querySelector('[data-panel-failure]');
    assert.ok(failure);assert.equal(failure.closest('[data-domain-ui=card]'),f.el.firstElementChild);
    assert.match(failure.textContent,/暂时无法读取世界物品目录/);
    assert.doesNotMatch(f.el.textContent,/尚未选择运行世界|Service Unavailable|INTERNAL|private_key/);
    disabled(f.el,true);
    f.failItems=false;f.waitItems=null;failure.querySelector('[data-panel-retry]').click();
    disabled(f.el,true);await tick();disabled(f.el,false);
    assert.equal(f.el.querySelector('[data-panel-failure]'),null);
    assert.equal(f.el.querySelectorAll('[data-items] h3').length,2);
    assert.equal(f.calls.filter(path=>path.endsWith('/world/items')).length,2);
  }finally{f.dispose();}
});

test('catalog read failure is recoverable and never misreported as missing binding',async()=>{
  const f=fixture({failCatalog:true});
  try{
    await tick();disabled(f.el,true);
    assert.doesNotMatch(f.el.textContent,/尚未选择运行世界|INTERNAL|private_key/);
    assert.equal(f.calls.filter(path=>path.endsWith('/world/items')).length,0);
    f.failCatalog=false;f.el.querySelector('[data-panel-retry]').click();await tick();
    assert.equal(f.el.querySelectorAll('[data-items] h3').length,2);disabled(f.el,false);
  }finally{f.dispose();}
});

test('leaving during catalog read prevents a late item request',async()=>{
  const pending=deferred(),f=fixture({waitCatalog:pending.promise});
  await tick();f.dispose();pending.resolve();await tick();
  assert.equal(f.calls.filter(path=>path.endsWith('/world/items')).length,0);
});

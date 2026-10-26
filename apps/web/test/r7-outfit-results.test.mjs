import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {React,renderReact} from './helpers/react-dom.mjs';
const nextTick=()=>Promise.resolve();


test('outfit receipts survive reload, retain exact outcomes and block duplicate admission', async () => {
  const keys=['window','document','location','sessionStorage','localStorage'];
  const previous=Object.fromEntries(keys.map(key=>[key,globalThis[key]]));
  const values=new Map();
  const storage={getItem:key=>values.get(key)??null,setItem:(key,value)=>values.set(key,String(value)),removeItem:key=>values.delete(key)};
  globalThis.window={location:{origin:'http://outfit.test'},addEventListener(){},removeEventListener(){}};
  globalThis.document=Object.assign(new EventTarget(),{hidden:false});
  globalThis.location={hash:''};globalThis.localStorage=storage;globalThis.sessionStorage=storage;
  const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
  let workspace;
  try {
    const {createWorkspace}=await vite.ssrLoadModule('/src/state/workspace.mjs');
    const store=await vite.ssrLoadModule('/src/v3/proto/store.js');
    const services=await vite.ssrLoadModule('/src/v3/proto/services.js');
    const {default:WardrobePage}=await vite.ssrLoadModule('/src/react-app/pages/WardrobePage.jsx');
    const receipts=new Map();
    let submissions=0,release;
    services.configureDomainService('inventory',{
      clothingCatalogue:async()=>({items:[]}),
      outfits:async()=>({items:[{id:'day',name:'日常搭配',revision:1,items:[]},{id:'run',name:'跑步搭配',revision:1,items:[]}],next_cursor:'',profile_id:'default'}),
      tags:async()=>({tags:[]}),
      action:async()=>{submissions++;return new Promise(resolve=>{release=resolve})},
    });
    services.configureDomainService('life',{
      wardrobe:async()=>({revision:1,worn_items:[]}),
      request:async(fid,id)=>structuredClone(receipts.get(id)),
    });
    const flush=async()=>{for(let i=0;i<3;i++){await new Promise(done=>setImmediate(done));await nextTick()}};
    const recreate=async()=>{
      workspace?.dispose();workspace=createWorkspace();
      store.replaceSnapshot({...store.get(),session:{actorId:'owner'},characters:{A:{id:'A',name:'周屿'}},ui:{charId:'A'}});
      workspace.state.outfitTab='方案库';await workspace.load('衣柜');await flush();
    };
    const render=()=>renderReact(vite,WardrobePage,workspace);
    await recreate();
    let html=await render();
    assert.match(html,/为周屿搭配/);assert.doesNotMatch(html,/为她搭配/);
    assert.match(html,/aria-label="请周屿换上「日常搭配」"/);
    assert.match(html,/aria-label="请周屿换上「跑步搭配」"/);
    for (const [index,status,result,expected] of [
      [1,'completed',{disposition:'committed',mutation_committed:true},'请求已完成'],
      [2,'completed',{disposition:'no_change',mutation_committed:false},'已经穿着这套搭配，本次无需换装'],
      [3,'failed',{disposition:'not_applied',mutation_committed:false,error_code:'owner_life_request_not_applied'},'没有可确认的已提交结果'],
    ]) {
      const id='request-'+index;
      const submit=(body,key)=>workspace.services.inventoryClient.action('A',{...body,idempotencyKey:key});
      const first=workspace.command('换装','wear-outfit:day',{outfitId:'day'},submit,{onSaved:()=>workspace.loadWardrobe()});
      const again=workspace.command('换装','wear-outfit:day',{outfitId:'day'},submit);
      assert.equal(again,first,'same-tick repeated command must keep the original submission');
      await flush();assert.equal(submissions,index);
      receipts.set(id,{id,kind:'inventory_outfit',payload:{outfit_id:'day'},status,result,decision_reason:status==='failed'?'The request did not produce a committed result':'The requested change was committed'});
      release({request:{id,status:'pending'}});await first.completion;await flush();
      html=await render();assert.match(html,new RegExp(expected));assert.doesNotMatch(html,/The requested|The request did/);
      const pointers=[...values.entries()].find(([key])=>key.startsWith('fluctlight:approved-receipts:'));
      const saved=JSON.parse(pointers[1]);assert.equal(saved.length,1);assert.equal(saved[0].id,id);
      assert.deepEqual(Object.keys(saved[0]).sort(),['id','label','operation','terminal']);
      await recreate();html=await render();assert.match(html,new RegExp(expected));assert.equal(submissions,index);
      await workspace.load('衣柜');document.dispatchEvent(new Event('visibilitychange'));await flush();
      assert.equal(workspace.state.operations.filter(row=>row.requestId===id).length,1);
      const recovered=workspace.state.operations.find(row=>row.requestId===id);
      recovered.run();assert.equal(location.hash,'#/diag/request/'+id);
    }
    // An accepted asynchronous intention without a mutation is not a satisfied outfit.
    receipts.set('accepted',{id:'accepted',status:'accepted',result:{mutation_committed:false}});
    const accepted=workspace.command('约定','promise:one',{},async()=>({request:{id:'accepted',status:'pending'}}));
    await accepted.completion;await flush();
    assert.match(accepted.response,/请求已接受/);assert.doesNotMatch(accepted.response,/已经穿着|目标状态已经满足/);

    // Legacy pointers bind in submission order, even if the older GET finishes last.
    workspace.dispose();
    const receiptKey='fluctlight:approved-receipts:'+JSON.stringify(['owner','A']);
    values.set(receiptKey,JSON.stringify([{id:'legacy-old',label:'换装'},{id:'legacy-new',label:'换装'}]));
    receipts.set('legacy-old',{id:'legacy-old',kind:'inventory_outfit',payload:{outfit_id:'day'},status:'failed',decision_reason:'旧请求失败'});
    receipts.set('legacy-new',{id:'legacy-new',kind:'inventory_outfit',payload:{outfit_id:'day'},status:'completed',result:{disposition:'no_change',mutation_committed:false}});
    let finishOld;
    services.configureDomainService('life',{
      wardrobe:async()=>({revision:1,worn_items:[]}),
      request:async(fid,id)=>id==='legacy-old'?new Promise(resolve=>{finishOld=()=>resolve(structuredClone(receipts.get(id)))}):structuredClone(receipts.get(id)),
    });
    await recreate();
    finishOld();await flush();
    assert.deepEqual(JSON.parse(values.get(receiptKey)).map(row=>row.id),['legacy-new']);
    html=await render();assert.match(html,/已经穿着这套搭配/);assert.doesNotMatch(html,/旧请求失败/);
    await recreate();
    assert.equal(workspace.state.operations.length,1);

    // A pending observation must reattach its record after an Owner round trip.
    workspace.dispose();
    values.set(receiptKey,JSON.stringify([{id:'still-pending',label:'换装',operation:'wear-outfit:day'}]));
    receipts.set('still-pending',{id:'still-pending',kind:'inventory_outfit',payload:{outfit_id:'day'},status:'processing'});
    await recreate();
    store.replaceSnapshot({...store.get(),session:{actorId:'other-owner'}});
    await workspace.load('衣柜');await flush();assert.equal(workspace.state.operations.length,0);
    store.replaceSnapshot({...store.get(),session:{actorId:'owner'},characters:{A:{id:'A',name:'周屿'},B:{id:'B',name:'另一个角色'}}});
    await workspace.load('衣柜');await flush();
    const pending=workspace.state.operations.find(row=>row.requestId==='still-pending');
    assert.ok(pending?.requestUnresolved);
    const previousSubmissions=submissions;
    assert.equal(workspace.command('换装','wear-outfit:day',{outfitId:'day'},()=>{submissions++;}),pending);
    assert.equal(submissions,previousSubmissions);
    workspace.select('B');await workspace.load('衣柜');await flush();assert.doesNotMatch(await render(),/换装请求已受理/);
    workspace.select('A');await workspace.load('衣柜');await flush();
    assert.equal(workspace.state.operations.filter(row=>row.requestId==='still-pending').length,1);
    receipts.set('still-pending',{id:'still-pending',kind:'inventory_outfit',payload:{outfit_id:'day'},status:'completed',result:{disposition:'committed',mutation_committed:true}});
    await pending.run();
    // Terminal result identity belongs to this Owner and character only.
    store.replaceSnapshot({...store.get(),session:{actorId:'other-owner'}});
    await workspace.load('衣柜');await flush();
    assert.equal(workspace.state.operations.length,0);
    assert.doesNotMatch(await render(),/没有可确认的已提交结果/);
  } finally {
    workspace?.dispose();await vite.close();
    for(const [key,value] of Object.entries(previous))if(value===undefined)delete globalThis[key];else globalThis[key]=value;
  }
});

import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {React,renderReact} from './helpers/react-dom.mjs';
const nextTick=()=>Promise.resolve();


test('actual wear survives incomplete reads, uses its own description and keeps each outfit request visible',async()=>{
 const keys=['window','document','location','sessionStorage','localStorage'],previous=Object.fromEntries(keys.map(k=>[k,globalThis[k]]));
 const values=new Map(),storage={getItem:k=>values.get(k)??null,setItem:(k,v)=>values.set(k,String(v)),removeItem:k=>values.delete(k)};
 globalThis.window={location:{origin:'http://wear.test'},addEventListener(){},removeEventListener(){}};
 globalThis.document=Object.assign(new EventTarget(),{hidden:false});globalThis.location={hash:''};globalThis.localStorage=storage;globalThis.sessionStorage=storage;
 const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
 let workspace;
 try{
  const {createWorkspace}=await vite.ssrLoadModule('/src/state/workspace.mjs'),store=await vite.ssrLoadModule('/src/v3/proto/store.js'),services=await vite.ssrLoadModule('/src/v3/proto/services.js');
  const {CurrentOutfit,ActionFeedback}=await vite.ssrLoadModule('/src/react-app/components.tsx');
  const {default:WardrobePage}=await vite.ssrLoadModule('/src/react-app/pages/WardrobePage.jsx');
  workspace=createWorkspace();store.replaceSnapshot({...store.get(),session:{actorId:'owner'},characters:{A:{id:'A',name:'苏晚'}},ui:{charId:'A'}});
  let wearing={revision:4,worn_items:[{item_id:'actual-boots',slot:'shoes',description:'已登记的黑色短靴'}]},receipt={id:'wear-request',status:'pending'},requestReadError=false,submissions=0;
  services.configureDomainService('inventory',{clothingCatalogue:async()=>({items:[]}),outfits:async()=>({items:[{id:'one',name:'短靴搭配',revision:2,items:[]}],next_cursor:'',profile_id:'default'}),tags:async()=>({tags:[]}),action:async()=>{submissions++;return {request:{id:'uncertain-request',status:'pending'}}}});
  workspace.client.detail=async()=>({identity:{name:'苏晚',timezone:'Asia/Shanghai'}});
  services.configureDomainService('life',{wardrobe:async()=>structuredClone(wearing),request:async()=>{if(requestReadError)throw Error('状态查询暂时断网');return structuredClone(receipt)},schedules:async()=>({days:[]}),goals:async()=>({items:[],next_cursor:''}),promises:async()=>({items:[],next_cursor:''})});
  for(const domain of ['diary','memory'])services.configureDomainService(domain,{list:async()=>({items:[],next_cursor:''})});
  const render=component=>renderReact(vite,component,workspace);
  assert.match(await render(CurrentOutfit),/状态待读取/);
  await workspace.load('此刻');
  assert.deepEqual(workspace.current.value.wearing['鞋子'],['actual-boots']);
  assert.match(await render(CurrentOutfit),/已登记的黑色短靴/);
  wearing={revision:4};await assert.rejects(workspace.loadWardrobe(),/当前穿着未能完整读回/);
  let html=await render(CurrentOutfit);assert.match(html,/已登记的黑色短靴/);assert.match(html,/保留上次确认/);
  wearing={revision:4,worn_items:[]};await workspace.loadWardrobe();html=await render(CurrentOutfit);assert.doesNotMatch(html,/已登记的黑色短靴/);assert.match(html,/未登记/);
  document.hidden=true;
  const record=workspace.command('换装','wear-outfit:one',{outfitId:'one'},async()=>({request:{id:'wear-request',status:'pending'}}),{onSaved:()=>workspace.loadWardrobe()});
  await record.completion;
  const CardFeedback=()=>React.createElement(ActionFeedback,{labels:['换装'],operation:'wear-outfit:one',showStatus:true});
  assert.match(await render(CardFeedback),/换装请求已受理/);
  assert.ok([...values.values()].some(v=>v.includes('wear-outfit:one')));
  record.operation=undefined; // A receipt checkpoint created before operation-scoped cards.
  receipt={id:'wear-request',kind:'inventory_outfit',payload:{outfit_id:'one'},status:'failed',decision_reason:'模型没有执行换装工具，原请求已保留。'};
  wearing={revision:4,worn_items:[null]};
  document.hidden=false;document.dispatchEvent(new Event('visibilitychange'));for(let i=0;i<3;i++){await new Promise(done=>setImmediate(done));await nextTick()}
  assert.equal(record.status,'error');
  assert.equal(record.operation,'wear-outfit:one');
  const later=workspace.command('衣柜','rename:another',{name:'其他方案'},async()=>({revision:5}));await later.completion;
  html=await render(CardFeedback);assert.match(html,/模型没有执行换装工具/);assert.match(html,/查看原请求/);
  assert.match(html,/请求状态已确认，但页面结果读回失败/);
  assert.match(html,/重新读取页面结果/);
  record.run();assert.equal(location.hash,'#/diag/request/wear-request');
  wearing={revision:4,worn_items:[]};await record.refreshResult();assert.equal(record.readbackError,'');assert.match(record.reason,/模型没有执行换装工具/);

  // A received request keeps its identity while its status endpoint is unavailable.
  requestReadError=true;
  const submit=(body,key)=>workspace.services.inventoryClient.action('A',{...body,idempotencyKey:key});
  const uncertain=workspace.command('换装','wear-outfit:one',{outfitId:'one'},submit,{onSaved:()=>workspace.loadWardrobe()});
  await uncertain.completion;await new Promise(done=>setImmediate(done));
  assert.equal(uncertain.requestUnresolved,true);assert.equal(uncertain.status,'error');
  assert.match(uncertain.reason,/请求已受理，处理状态暂未确认/);assert.equal(uncertain.actionText,'恢复查询原请求');
  assert.equal(workspace.command('换装','wear-outfit:one',{outfitId:'one'},submit),uncertain);
  assert.equal(submissions,1);
  workspace.state.outfitTab='方案库';
  html=await render(WardrobePage);assert.match(html,/<button[^>]*aria-label="请苏晚换上「短靴搭配」"[^>]*disabled[^>]*>请换上这套<\/button>/);assert.match(html,/恢复查询原请求/);

  // Reload from an old checkpoint without operation, then discover it from the real request payload.
  workspace.dispose();
  const checkpointKey=[...values.keys()].find(key=>key.startsWith('fluctlight:approved-receipts:'));
  values.set(checkpointKey,JSON.stringify([{id:'uncertain-request',label:'换装'}]));
  workspace=createWorkspace();store.replaceSnapshot({...store.get()});
  workspace.state.outfitTab='方案库';await workspace.load('衣柜');
  const restored=workspace.state.operations.find(row=>row.requestId==='uncertain-request');
  assert.ok(restored?.requestUnresolved);
  html=await render(WardrobePage);assert.match(html,/<button[^>]*aria-label="请苏晚换上「短靴搭配」"[^>]*disabled[^>]*>请换上这套<\/button>/);assert.match(html,/恢复查询原请求/);
  requestReadError=false;
  receipt={id:'uncertain-request',kind:'inventory_outfit',payload:{outfit_id:'one'},status:'completed',result:{mutation_committed:true}};
  wearing={revision:5};
  await restored.run();
  assert.equal(restored.operation,'wear-outfit:one');assert.equal(restored.status,'success');assert.equal(restored.requestUnresolved,false);
  assert.match(restored.readbackError,/页面结果读回失败/);assert.equal(submissions,1);
  restored.run();assert.equal(location.hash,'#/diag/request/uncertain-request');
  wearing={revision:5,worn_items:[{item_id:'actual-boots',slot:'shoes',description:'已登记的黑色短靴'}]};
  await restored.refreshResult();assert.equal(restored.readbackError,'');assert.match(await render(CurrentOutfit),/已登记的黑色短靴/);
 }finally{workspace?.dispose();await vite.close();for(const[k,v]of Object.entries(previous)){if(v===undefined)delete globalThis[k];else globalThis[k]=v}}
});

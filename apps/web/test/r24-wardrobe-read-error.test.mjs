import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {createServer} from './support/module-server.mjs';
import {fileURLToPath} from 'node:url';
import {act, installDOM, mountReact} from './helpers/react-dom.mjs';
import {readErrorCause, serviceUnavailableNotice, settingsHref} from '../src/approved-ui/service-unavailable.mjs';

const env=installDOM();
const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const load=path=>vite.ssrLoadModule('/src/'+path);
test.after(async()=>{await vite.close();env.restore();});

test('service classification prefers specific image services and never echoes unknown raw errors',()=>{
  for(const [error,kind] of [
    [Object.assign(new Error('provider role conversation_reply unconfigured'),{code:'role_unconfigured'}),'model'],
    [Object.assign(new Error('image provider model not configured'),{code:'image_provider_unconfigured'}),'media'],
    [Object.assign(new Error('Service Unavailable'),{status:503}),'service'],
    [new Error('Failed to fetch'),'network'],
    [Object.assign(new Error('revision mismatch'),{code:'browser_request_failed',status:500}),'unknown'],
    [new Error('INTERNAL_UNKNOWN secret_key=abc owner_id=private'),'unknown'],
  ]){
    const notice=serviceUnavailableNotice(readErrorCause(error),error.message);
    assert.equal(notice.kind,kind);
    assert.doesNotMatch(notice.reason,/INTERNAL|secret_key|owner_id|provider|unconfigured|Unavailable|Failed to fetch/);
    assert.match(settingsHref(notice.configSection),/^#\/settings\?section=/);
  }
  assert.doesNotMatch(serviceUnavailableNotice(null,'English backend error').reason,/English backend error/);
  assert.doesNotMatch(serviceUnavailableNotice({status:503}).reason,/尚未配置/,'generic failure does not establish missing configuration');
});

async function fixture(){
  const {createWorkspace}=await load('state/workspace.mjs'),store=await load('v3/proto/store.js'),{configureDomainService}=await load('v3/proto/services.js');
  sessionStorage.clear();localStorage.clear();
  const w=createWorkspace();
  store.replaceSnapshot({...store.get(),session:{actorId:'r24-owner'},characters:{A:{id:'A',name:'FT-苏晴'}},ui:{charId:'A'}});
  let failure=Object.assign(new Error('image provider model not configured INTERNAL_KEY'),{code:'image_provider_unconfigured',status:503}),reads=0;
  configureDomainService('inventory',{clothingCatalogue:async()=>({items:[]}),outfits:async()=>({items:[],next_cursor:'',profile_id:'default'}),tags:async()=>({tags:[]}),preparation:async()=>({status:'completed'})});
  configureDomainService('life',{wardrobe:async()=>{reads++;if(failure)throw failure;return {revision:1,worn_items:[]};}});
  await assert.rejects(w.loadWardrobe());
  return {w,get reads(){return reads;},succeed(){failure=null;}};
}

test('actual wardrobe retains header/tabs/filters, offers recovery on every tab and restores content after retry',async()=>{
  const f=await fixture(),{w}=f,{default:Page}=await load('react-app/pages/WardrobePage.jsx');
  const v=await mountReact(vite,Page,{workspace:w});
  try{
    assert.match(v.host.textContent,/FT-苏晴的衣柜/);
    assert.equal(v.host.querySelectorAll('[role=tab]').length,3);
    for(const label of ['搜索衣服','按类型筛选','按季节筛选'])assert.ok(v.host.querySelector(`[aria-label="${label}"]`));
    const notice=v.host.querySelector('[data-testid=wardrobe-read-error]');
    assert.match(notice.textContent,/图片服务/);
    assert.ok(v.host.querySelector('.wardrobe-filters').compareDocumentPosition(notice)&window.Node.DOCUMENT_POSITION_FOLLOWING);
    assert.equal(v.host.querySelector('[data-testid=current-outfit]').textContent,'暂未读取到穿着');
    for(const tab of ['全部衣服','方案库','为她搭配']){
      await act(async()=>{w.state.outfitTab=tab;});await v.flush();
      assert.equal(v.host.querySelectorAll('[data-testid=wardrobe-read-error]').length,1);
      assert.equal(v.host.querySelector('a')?.textContent,'前往管理中心');
      assert.equal(v.host.querySelector('a').hash,settingsHref('模型用途'));
      assert.doesNotMatch(v.host.textContent,/INTERNAL_KEY|image_provider|provider|没有符合|还没有符合|慢慢挑|方案名称|正在读取/);
      assert.equal(v.host.querySelector('.r-garment-grid,.r-outfit-grid,.r-compose-layout'),null);
    }
    const retry=()=>[...v.host.querySelectorAll('button')].find(b=>b.textContent==='重新读取');
    await v.click(retry());assert.equal(f.reads,2);
    f.succeed();await v.click(retry());assert.equal(f.reads,3);
    assert.equal(w.state.errorCauses.wardrobe,undefined);
    assert.equal(v.host.querySelector('[data-testid=wardrobe-read-error]'),null);
    assert.ok(v.host.querySelector('.r-compose-layout'));
  }finally{await v.dispose();w.dispose();}
});

test('shared and legacy wearing summaries show one unknown status, while confirmed empty/populated history is retained',async()=>{
  const f=await fixture(),{w}=f,{default:Summary}=await load('react-app/shared/WearingSummary.tsx'),{CurrentOutfit}=await load('react-app/components.tsx');
  try{
    for(const Component of [Summary,CurrentOutfit]){
      const v=await mountReact(vite,Component,{workspace:w,props:{character:{},error:'raw INTERNAL_KEY',loading:true}});
      try{
        const outfit=v.host.querySelector('[data-testid=current-outfit]');
        assert.equal(outfit.querySelectorAll('[role=status]').length,1);
        assert.match(outfit.textContent,/暂未读取到穿着/);
        assert.doesNotMatch(outfit.textContent,/保留上次|实际穿着尚未读到|状态待读取|正在读取|INTERNAL/);
      }finally{await v.dispose();}
    }
    for(const worn of [[],[{item_id:'private-item',description:'白色衬衫'}]]){
      const v=await mountReact(vite,Summary,{props:{character:{_wardrobe:{wearing:{worn_items:worn}}},error:'raw INTERNAL_KEY'}});
      try{
        assert.match(v.host.textContent,/保留上次确认/);
        assert.match(v.host.textContent,worn.length?/白色衬衫/:/还没有登记实际穿着/);
        assert.doesNotMatch(v.host.textContent,/暂未读取到穿着|private-item|INTERNAL_KEY/);
      }finally{await v.dispose();}
    }
    f.succeed();await w.loadWardrobe();w.state.errors.wardrobe='raw';w.state.errorScopes.wardrobe=w.currentScope();
    const {default:Page}=await load('react-app/pages/WardrobePage.jsx'),v=await mountReact(vite,Page,{workspace:w});
    try{
      assert.match(v.host.querySelector('[data-feedback-kind=warning]').textContent,/上次确认的衣服和方案仍保留/);
      assert.ok(v.host.querySelector('.r-garment-grid'));
      assert.equal(v.host.querySelector('[data-testid=wardrobe-read-error]'),null);
    }finally{await v.dispose();}
  }finally{w.dispose();}
});

test('chat and now consume the same scoped wearing error; App delegates wardrobe failures to its page',async()=>{
  const read=path=>readFile(new URL('../src/react-app/'+path,import.meta.url),'utf8');
  assert.match(await read('pages/ChatPage.tsx'),/<WearingSummary[^>]*error=\{currentReadError\(workspace.state,'wardrobe',scope\)\}/);
  assert.match(await read('views/NowPage.jsx'),/<WearingSummary[^>]*error=\{view.readError\('wardrobe'\)\}/);
  assert.match(await read('App.tsx'),/'媒体','衣柜'\].includes\(page\)/);
});

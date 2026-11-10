import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {installDOM,mountReact} from './helpers/react-dom.mjs';

const environment=installDOM();
const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();environment.restore();});

const button=text=>[...document.querySelectorAll('button')].find(node=>node.textContent.trim()===text);

test('source-change response keeps extraction identity until the server confirms no effect',async()=>{
 const {default:Panel}=await vite.ssrLoadModule('/src/react-app/pages/ExpressionLearningPanel.tsx');
 const ids=[];
 let status='running';
 const source={id:'message-1',text:'最近感觉还不错',created_at:'2026-11-10T00:00:00Z'};
 const service={
  expressionLearning:async()=>({settings:{learning_enabled:true,use_enabled:false,revision:1},items:[]}),
  expressionSources:async()=>({items:[source],next_cursor:''}),
  expressionSource:async()=>source,
  expressionExtractionStatus:async(_fid,key)=>{assert.equal(key,ids[0]);return {status,no_effect:status==='failed'};},
  extractExpressionLearning:async(_fid,_source,key)=>{
   ids.push(key);
   if(ids.length===1)throw Object.assign(new Error('来源或学习设置已变化'),{status:409,code:'expression_source_changed'});
   return {status:'completed',candidate_ids:[]};
  },
 };
 const workspace={state:{session:{actorId:'owner-expression-recovery'},characters:[{id:'character',status:'active'}]},services:{selfClient:service}};
 const view=await mountReact(vite,Panel,{workspace,props:{fid:'character'}});
 try{
  await view.click(button('从这条提取'));
  await view.click(button('确认提交'));
  assert.equal(ids.length,1);
  assert.ok(button('查询原提交状态'),'a source-change response must not be treated as proof of no effect');
  assert.equal(button('确认提交').disabled,true);
  await view.click(button('查询原提交状态'));
  assert.equal(ids.length,1);
  assert.ok(button('查询原提交状态'),'a running extraction must retain its original identity');
  assert.equal(button('同步资料，保留输入'),undefined);
  status='failed';
  await view.click(button('查询原提交状态'));
  await view.click(button('同步资料，保留输入'));
  await view.click(button('已核对，采用最新基线'));
  await view.click(button('确认提交'));
  assert.equal(ids.length,2);
  assert.notEqual(ids[1],ids[0]);
 }finally{await view.dispose();}
});

test('switching owner clears that owner’s expression recovery records',async()=>{
 const store=await vite.ssrLoadModule('/src/v3/proto/store.js');
 await vite.ssrLoadModule('/src/v3/proto/components/studio-draft-storage.js');
 const original=structuredClone(store.get());
 const owner='owner-expression-storage',other='another-owner';
 const key='fluctlight-studio:'+JSON.stringify([owner,'character','expression-learning','extract','message-1'])+':command';
 const otherKey='fluctlight-studio:'+JSON.stringify([other,'character','expression-learning','extract','message-1'])+':command';
 try{
  store.replaceSnapshot({...structuredClone(original),session:{actorId:owner}});
  sessionStorage.setItem(key,'owner recovery');
  sessionStorage.setItem(otherKey,'other recovery');
  store.replaceSnapshot({...structuredClone(original),session:{actorId:other}});
  assert.equal(sessionStorage.getItem(key),null);
  assert.equal(sessionStorage.getItem(otherKey),'other recovery');
 }finally{
  sessionStorage.removeItem(key);
  sessionStorage.removeItem(otherKey);
  store.replaceSnapshot(original);
 }
});

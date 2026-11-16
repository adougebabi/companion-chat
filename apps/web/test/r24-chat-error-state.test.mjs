import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {act,installDOM,mountReact} from './helpers/react-dom.mjs';
import {settingsHref} from '../src/approved-ui/service-unavailable.mjs';

const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),cacheDir:'/workspace/r24p2/tmp/vite-r24-chat-error',appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const load=path=>vite.ssrLoadModule('/src/'+path);
const {BrowserClient}=await vite.ssrLoadModule('/@fs'+fileURLToPath(new URL('../../../packages/browser-client/src/index.ts',import.meta.url)));
const provider=await load('v3/providers/real.ts'),{productConversation:store}=await load('state/conversation.ts'),{default:Chat}=await load('react-app/pages/ChatPage.tsx');
test.after(async()=>{await vite.close();env.restore();});
const button=(v,text)=>[...v.host.querySelectorAll('button')].find(n=>n.textContent.trim()===text);

for(const [label,error,reason] of [
  ['missing model',Object.assign(new Error('provider role conversation_reply unconfigured INTERNAL_KEY owner_id=private'),{code:'role_unconfigured',status:503}),/聊天模型尚未配置或暂时不可用/],
  ['generic 503',Object.assign(new Error('Service Unavailable INTERNAL_KEY'),{status:503}),/后台服务暂时不可用/],
  ['network',new Error('Failed to fetch INTERNAL_KEY'),/暂时无法连接到服务/],
  ['unknown',new Error('INTERNAL_KEY owner_id=private'),/暂时无法确认具体原因/],
])test(`chat ${label}: actual open path classifies failure, blocks sending and recovers without losing draft`,async()=>{
  sessionStorage.clear();localStorage.clear();store.$reset();
  const state=proxy({session:{actorId:'chat-owner'},characters:[{id:'A',name:'雨禾',initial:'雨',schedule:[],wearing:{}}],errors:{},loading:{}});
  const w={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),current:{get value(){return state.characters[0];}},currentScope:()=> 'chat-owner-A',conversation:()=>store,openConversation:provider.openConversation,clock:()=> '12:00',detail:async()=>{},loadWardrobe:async()=>{},services:{}};
  sessionStorage.setItem('fluctlight:approved-chat-draft:chat-owner-A','保留草稿');
  const original={session:provider.client.session,list:provider.client.listFluctlights,direct:BrowserClient.prototype.directConversation,refresh:store.refreshActiveTurn,send:store.send};
  let failure=error,reads=0,sends=0;
  provider.client.session=async()=>({authenticated:true,actorId:'chat-owner'});
  provider.client.listFluctlights=async()=>[{id:'A',identity:{name:'雨禾'}}];
  BrowserClient.prototype.directConversation=async()=>{reads++;if(failure)throw failure;return {conversation:{id:'C'},participants:[],messages:[],nextBeforeSequence:null};};
  Object.defineProperty(store,'send',{configurable:true,value:async()=>{sends++;}});
  let v;
  try{
    v=await mountReact(vite,Chat,{workspace:w,props:{preview:false,onGo(){}}});
    const alert=v.host.querySelector('[data-testid=chat-read-error]'),field=v.host.querySelector('textarea');
    assert.ok(alert);assert.equal(alert.getAttribute('role'),'alert');assert.match(alert.textContent,reason);
    assert.doesNotMatch(v.host.textContent,/INTERNAL_KEY|owner_id|private|conversation_reply|unconfigured|Service Unavailable|Failed to fetch|衣柜服务/);
    if(label==='generic 503')assert.doesNotMatch(alert.textContent,/尚未配置/);
    const config=[...alert.querySelectorAll('a')].find(a=>a.textContent==='去配置模型');
    assert.equal(config.hash,settingsHref('模型用途'));
    assert.ok(config.closest('div').classList.contains('flex-col'));
    assert.equal(alert.querySelector('[data-slot=alert-description]').contains(config),false);
    assert.equal(field.disabled,true);assert.match(field.placeholder,/对话暂不可用/);assert.equal(field.value,'保留草稿');assert.equal(button(v,'发送').disabled,true);
    await act(async()=>field.dispatchEvent(new window.KeyboardEvent('keydown',{key:'Enter',bubbles:true})));
    await v.click(button(v,'发送'));assert.equal(sends,0);
    await v.click(button(v,'重新读取'));assert.equal(reads,2);assert.ok(v.host.querySelector('[data-testid=chat-read-error]'));
    failure=null;await v.click(button(v,'重新读取'));assert.equal(reads,3);
    assert.equal(v.host.querySelector('[data-testid=chat-read-error]'),null);assert.ok(v.host.querySelector('.replica-chat-empty'));
    assert.equal(field.disabled,false);assert.equal(button(v,'发送').disabled,false);assert.equal(field.value,'保留草稿');
    // An explicit refresh can fail even after history was loaded. A stale
    // conversation must not leave either the button or Enter able to send.
    Object.defineProperty(store,'refreshActiveTurn',{configurable:true,value:async()=>{throw error;}});
    await act(async()=>{store.error='取消请求结果尚未确认，请刷新进度核对。原消息已保留。';});await v.flush();
    await v.click(button(v,'重新读取'));
    assert.ok(store.conversation);assert.equal(field.disabled,true);assert.equal(button(v,'发送').disabled,true);
    await act(async()=>field.dispatchEvent(new window.KeyboardEvent('keydown',{key:'Enter',bubbles:true})));assert.equal(sends,0);
    Object.defineProperty(store,'refreshActiveTurn',{configurable:true,value:async()=>{store.error='';}});
    await v.click(button(v,'重新读取'));assert.equal(field.disabled,false);assert.equal(field.value,'保留草稿');assert.equal(v.host.querySelector('[data-testid=chat-read-error]'),null);
  }finally{
    if(v)await v.dispose();
    provider.client.session=original.session;provider.client.listFluctlights=original.list;BrowserClient.prototype.directConversation=original.direct;
    Object.defineProperty(store,'refreshActiveTurn',{configurable:true,value:original.refresh});Object.defineProperty(store,'send',{configurable:true,value:original.send});
    store.$reset();
  }
});

// Exercise the provider/controller with HTTP-shaped failures, including the
// message polling failures. Auxiliary failures must leave chatting available.
for(const source of ['conversation','directory','messages','detail','wardrobe'])
for(const missingModel of [false,true])
test(`chat natural ${source} read: ${missingModel?'unconfigured model':'503'} ${['conversation','messages'].includes(source)?'uses the unified alert':'leaves chatting available'}`,async()=>{
  sessionStorage.clear();localStorage.clear();store.$reset();
  const state=proxy({characters:[{id:'A',name:'雨禾',initial:'雨',schedule:[],wearing:{}}],errors:{},loading:{}});
  const original={session:provider.client.session,list:provider.client.listFluctlights,direct:BrowserClient.prototype.directConversation,messages:BrowserClient.prototype.messages,interval:globalThis.setInterval,send:store.send};
  let failed=true,poll,sends=0;
  const reads=[];
  const errorBody=missingModel?{code:'role_unconfigured',message:'provider role conversation_reply unavailable INTERNAL_KEY'}:{code:'request_failed',message:'Service Unavailable INTERNAL_KEY'};
  const http=new BrowserClient('',async(path,options)=>{
    assert.ok(!options?.method||options.method==='GET','only local mocked reads are permitted');
    path=String(path);reads.push(path);
    const at=path.includes('/messages')?'messages':path.includes('/conversation')?'conversation':path.includes('/directory')?'directory':path.includes('/wardrobe')?'wardrobe':'detail';
    return failed&&at===source?Response.json(errorBody,{status:503}):Response.json(at==='directory'?[{id:'A',identity:{name:'雨禾'}}]:{conversation:{id:'C'},participants:[],messages:[],nextBeforeSequence:null});
  });
  provider.client.session=async()=>({authenticated:true,actorId:'chat-owner'});
  provider.client.listFluctlights=()=>http.json('/directory');
  BrowserClient.prototype.directConversation=function(id){return original.direct.call(http,id);};
  BrowserClient.prototype.messages=function(...args){return original.messages.apply(http,args);};
  Object.defineProperty(store,'send',{configurable:true,value:async()=>{sends++;}});
  globalThis.setInterval=(callback,ms,...args)=>ms===2000?(poll=callback,original.interval(()=>{},ms)):original.interval(callback,ms,...args);
  const w={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),current:{get value(){return state.characters[0];}},currentScope:()=> 'natural-A',conversation:()=>store,openConversation:provider.openConversation,clock:()=> '12:00',detail:()=>http.json('/detail'),loadWardrobe:()=>http.json('/wardrobe'),services:{}};
  let v;
  try{
    v=await mountReact(vite,Chat,{workspace:w,props:{preview:false,onGo(){}}});
    if(!['conversation','directory'].includes(source)){
      assert.equal(v.host.querySelector('[data-testid=chat-read-error]'),null);
      // Polling begins with an already loaded conversation and no snapshot error.
      assert.ok(store.conversation);assert.equal(store.error,'');
      await act(async()=>poll());await v.flush();
    }
    const alert=v.host.querySelector('[data-testid=chat-read-error]'),field=v.host.querySelector('textarea');
    if(['directory','detail','wardrobe'].includes(source)){
      assert.equal(alert,null);assert.equal(field.disabled,false);
      await v.input(field,'辅助读取失败时仍可聊天');assert.equal(button(v,'发送').disabled,false);
      assert.ok(v.host.querySelector('.replica-chat-empty'));
      assert.doesNotMatch(v.host.textContent,/INTERNAL_KEY|conversation_reply|role_unconfigured|Service Unavailable/);
      if(source==='directory')assert.equal(reads.some(path=>new URL(path,'http://react.test').pathname==='/directory'),false);
      else assert.ok(reads.some(path=>new URL(path,'http://react.test').pathname==='/'+source));
      return;
    }
    assert.ok(alert);assert.equal(alert.getAttribute('role'),'alert');
    assert.match(alert.textContent,missingModel?/模型服务尚未配置或暂时不可用/:/后台服务暂时不可用/);
    assert.ok([...alert.querySelectorAll('a')].some(a=>a.textContent==='去配置模型'&&a.hash===settingsHref('模型用途')));
    assert.ok(button(v,'重新读取'));assert.equal(button(v,'重新读取对话'),undefined);
    assert.doesNotMatch(v.host.textContent,/服务暂时无法连接，请稍后重新读取。|INTERNAL_KEY|conversation_reply|role_unconfigured|Service Unavailable/);
    assert.equal(field.disabled,true);assert.equal(button(v,'发送').disabled,true);assert.match(v.host.textContent,/对话恢复后才可发送/);
    assert.equal(v.host.querySelector('.replica-chat-empty'),null);
    await act(async()=>field.dispatchEvent(new window.KeyboardEvent('keydown',{key:'Enter',bubbles:true})));
    await v.click(button(v,'发送'));assert.equal(sends,0);
    await v.click(button(v,'重新读取'));assert.ok(v.host.querySelector('[data-testid=chat-read-error]'),'a failed retry must keep the alert');
    failed=false;await v.click(button(v,'重新读取'));
    assert.equal(v.host.querySelector('[data-testid=chat-read-error]'),null);assert.equal(field.disabled,false);
    assert.ok(reads.some(path=>path.includes(source==='directory'?'/directory':source==='messages'?'/messages':source==='conversation'?'/conversation':'/'+source)));
  }finally{
    if(v)await v.dispose();
    globalThis.setInterval=original.interval;
    provider.client.session=original.session;provider.client.listFluctlights=original.list;
    BrowserClient.prototype.directConversation=original.direct;BrowserClient.prototype.messages=original.messages;
    Object.defineProperty(store,'send',{configurable:true,value:original.send});store.$reset();
  }
});

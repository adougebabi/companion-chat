import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from './support/module-server.mjs';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {installDOM,mountReact} from './helpers/react-dom.mjs';

const environment=installDOM();
const originalFetch=globalThis.fetch;
let activeFetch;
globalThis.fetch=(...args)=>activeFetch(...args);
const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();globalThis.fetch=originalFetch;environment.restore();});

const button=(text,root)=>[...root.querySelectorAll('button')].find(node=>node.textContent.trim()===text);

async function withPage({unconfirmed=false},run){
 localStorage.clear();sessionStorage.clear();
 const conversation={id:'chat-c',createdByActorId:'owner',revision:1,createdAt:'2026-11-10T00:00:00Z',updatedAt:'2026-11-10T00:00:00Z'};
 const first={id:'message-a',conversationId:'chat-c',sequence:1,authorActorId:'owner',kind:'user',text:'先说第一句',attachmentRefs:[],createdAt:'2026-11-10T00:00:00Z',turnId:'turn-a',idempotencyKey:'key-a',turnStatus:'running'};
 const page={conversation,participants:[],messages:[first],nextBeforeSequence:null};
 if(unconfirmed)localStorage.setItem('fluctlight.retry-turn.v2',JSON.stringify({conversationId:'chat-c',fluctlightId:'character-a',text:'未确认的旧请求',turnId:'unknown-turn',idempotencyKey:'unknown-key',attachmentRefs:[]}));
 const calls=[];let cancellations=0;
 activeFetch=async(input,init={})=>{
  const url=new URL(String(input));
  if(url.pathname.endsWith('/cancel')){cancellations++;return Response.json({status:'cancelled'});}
  if(url.pathname.endsWith('/turn')){
   const request=JSON.parse(String(init.body));calls.push(request);
   const next={id:'message-b',conversationId:'chat-c',sequence:2,authorActorId:request.senderActorId||'owner',kind:'user',text:request.text,attachmentRefs:request.attachmentRefs,createdAt:'2026-11-10T00:00:01Z',turnId:request.turnId,idempotencyKey:request.idempotencyKey,turnStatus:'completed'};
   page.messages=[{...first,turnStatus:'failed',turnErrorCode:'superseded_by_newer_turn'},next];
   return new Response([JSON.stringify({type:'message',turnId:request.turnId,sequence:0,payload:{message:next}}),JSON.stringify({type:'completed',turnId:request.turnId,sequence:1,payload:{}}),''].join('\n'),{headers:{'content-type':'application/x-ndjson'}});
  }
  if(url.pathname.endsWith('/conversation')||url.pathname.endsWith('/messages'))return Response.json(page);
  if(url.pathname.endsWith('/read'))return new Response(null,{status:204});
  throw Error('unexpected local test request '+url.pathname);
 };
 const {createConversationController}=await vite.ssrLoadModule('/src/state/conversation.ts');
 const {default:Chat}=await vite.ssrLoadModule('/src/react-app/pages/ChatPage.tsx');
 const store=createConversationController();
 store.fluctlights=[{id:'character-a',identity:{name:'测试角色'},status:'active'}];
 const state=proxy({session:{actorId:'owner'},selectedId:'character-a',characters:[{id:'character-a',name:'测试角色',initial:'角',color:'#eee',schedule:[],wearing:{},garments:[],_directory:{}}],errors:{},loading:{}});
 const workspace={state,subscribe:listener=>subscribe(state,listener),getSnapshot:()=>snapshot(state),current:{get value(){return state.characters[0];}},currentScope:()=>JSON.stringify(['owner',state.selectedId]),conversation:()=>store,openConversation:(fid,live)=>store.selectFluctlight(fid,{current:live}),clock:()=>'',detail:async()=>{},loadWardrobe:async()=>{},services:{mediaClient:{url:id=>id}}};
 const view=await mountReact(vite,Chat,{workspace,props:{preview:false,onGo(){}}});
 try{await run({view,store,calls,cancellations:()=>cancellations,first});}
 finally{await view.dispose();localStorage.clear();sessionStorage.clear();}
}

test('reopened chat sends a follow-up through the real controller without cancelling',async()=>{
 await withPage({},async({view,store,calls,cancellations,first})=>{
  const field=view.host.querySelector('textarea');await view.input(field,'刚想起来还有一句');
  const submit=button('追加消息',view.host);assert.ok(submit);assert.equal(submit.disabled,false);
  await view.click(submit);
  assert.equal(cancellations(),0);assert.equal(calls.length,1);assert.equal(calls[0].text,'刚想起来还有一句');
  assert.notEqual(calls[0].turnId,first.turnId);assert.notEqual(calls[0].idempotencyKey,first.idempotencyKey);
  assert.equal(store.messages.filter(message=>message.id==='message-a').length,1);
  assert.equal(store.messages.filter(message=>message.turnId===calls[0].turnId).length,1);
  assert.equal(field.value,'');
 });
});

test('reopened chat retains its draft when an older local request is unconfirmed',async()=>{
 await withPage({unconfirmed:true},async({view,store,calls,cancellations})=>{
  const field=view.host.querySelector('textarea');await view.input(field,'还没有发送的新话');
  await view.click(button('追加消息',view.host));
  assert.equal(cancellations(),0);assert.equal(calls.length,0);assert.equal(field.value,'还没有发送的新话');
  assert.equal(store.retryTurn.idempotencyKey,'unknown-key');
 });
});

test('a draft-storage cleanup error does not interrupt the accepted chat stream',async()=>{
 await withPage({},async({view,store,calls})=>{
  const field=view.host.querySelector('textarea');await view.input(field,'本机草稿暂时无法清除');
  const prototype=Object.getPrototypeOf(sessionStorage),originalRemove=prototype.removeItem;
  let fail=true;
  prototype.removeItem=function(key){if(fail&&String(key).startsWith('fluctlight:approved-chat-draft:')){fail=false;throw Error('storage unavailable');}return originalRemove.call(this,key);};
  try{
   await view.click(button('追加消息',view.host));
   assert.equal(calls.length,1);
   assert.equal(store.messages.filter(message=>message.id==='message-b').length,1);
   assert.equal(store.retryTurn,null);
   assert.equal(field.value,'本机草稿暂时无法清除');
   assert.match(view.host.textContent,/本机草稿暂未清除/);
  }finally{prototype.removeItem=originalRemove;}
 });
});

import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from './support/module-server.mjs';
import {fileURLToPath} from 'node:url';
import {act,installDOM,mountReact} from './helpers/react-dom.mjs';

const env=installDOM();
const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const load=path=>vite.ssrLoadModule('/src/'+path);
const {createWorkspace}=await load('state/workspace.mjs');
const directory=await load('v3/proto/store.js');
const {configureDomainService}=await load('v3/proto/services.js');
const provider=await load('v3/providers/real.ts');
const {productConversation:store}=await load('state/conversation.ts');
const {default:Chat}=await load('react-app/pages/ChatPage.tsx');
const {BrowserClient}=await vite.ssrLoadModule('/@fs'+fileURLToPath(new URL('../../../packages/browser-client/src/index.ts',import.meta.url)));
const button=(v,text)=>[...v.host.querySelectorAll('button')].find(n=>n.textContent.trim()===text);
test.after(async()=>{await vite.close();env.restore();});

async function fixture(source){
  sessionStorage.clear();localStorage.clear();store.$reset();
  const w=createWorkspace();
  directory.replaceSnapshot({...directory.get(),session:{actorId:'isolation-owner'},characters:{A:{id:'A',name:'雨禾'}},ui:{charId:'A'}});
  const originals={session:provider.client.session,list:provider.client.listFluctlights,detail:provider.client.detail,direct:BrowserClient.prototype.directConversation,messages:BrowserClient.prototype.messages,interval:globalThis.setInterval};
  let conversationFailed=false,poll,directoryReads=0;
  const reads=[];
  const http=new BrowserClient('',async(path,options)=>{
    assert.ok(!options?.method||options.method==='GET','mock reads only; never generate');
    path=String(path);reads.push(path);
    const key=path.includes('/messages')?'messages':path.includes('/conversation')?'conversation':path.includes('/outfits')?'outfits':path.includes('/catalogue')?'catalogue':path.includes('/tags')?'tags':path.includes('/wardrobe')?'wardrobe':'detail';
    if(key===source||conversationFailed&&['conversation','messages'].includes(key))return Response.json({code:'request_failed',message:'Service Unavailable INTERNAL_KEY'},{status:503});
    const body={conversation:{conversation:{id:'C'},participants:[],messages:[],nextBeforeSequence:null},messages:{messages:[],nextBeforeSequence:null},detail:{identity:{name:'雨禾'}},wardrobe:{revision:1,worn_items:[]},outfits:{items:[],next_cursor:''},catalogue:{items:[]},tags:{tags:[]}}[key];
    return Response.json(body);
  });
  provider.client.session=async()=>({authenticated:true,actorId:'isolation-owner'});
  provider.client.listFluctlights=async()=>{directoryReads++;throw new Error('INTERNAL_KEY directory unavailable');};
  provider.client.detail=id=>http.json('/api/fluctlights/'+id);
  BrowserClient.prototype.directConversation=function(id){return originals.direct.call(http,id);};
  BrowserClient.prototype.messages=function(...args){return originals.messages.apply(http,args);};
  configureDomainService('inventory',{
    clothingCatalogue:()=>http.json('/inventory/catalogue'),outfits:()=>http.json('/inventory/outfits?cursor='),tags:()=>http.json('/inventory/tags'),
  });
  configureDomainService('life',{wardrobe:()=>http.json('/wardrobe')});
  globalThis.setInterval=(callback,ms,...args)=>ms===2000?(poll=callback,originals.interval(()=>{},ms)):originals.interval(callback,ms,...args);
  let v;
  return {w,reads,get directoryReads(){return directoryReads;},setConversationFailure(value){conversationFailed=value;},
    async mount(){v=await mountReact(vite,Chat,{workspace:w,props:{preview:true,onGo(){}}});return v;},
    async poll(){await act(async()=>poll());await v.flush();},
    async close(){if(v)await v.dispose();w.dispose();globalThis.setInterval=originals.interval;provider.client.session=originals.session;provider.client.listFluctlights=originals.list;provider.client.detail=originals.detail;BrowserClient.prototype.directConversation=originals.direct;BrowserClient.prototype.messages=originals.messages;store.$reset();},
  };
}

for(const source of ['outfits','detail','wardrobe','catalogue','tags','none'])
test(`empty conversation remains usable with ${source} auxiliary failure (outfits may omit profile_id)`,async()=>{
  const f=await fixture(source);
  try{
    await f.w.load('聊天');
    const v=await f.mount(),field=v.host.querySelector('textarea');
    assert.ok(store.conversation);assert.deepEqual([...store.messages],[]);
    assert.equal(field.disabled,false);assert.equal(v.host.querySelector('[data-testid=chat-read-error]'),null);
    await f.poll(); // Both initial load and summary polling isolate every reader.
    assert.equal(field.disabled,false);assert.equal(v.host.querySelector('[data-testid=chat-read-error]'),null);
    await v.input(field,'今天想聊聊');assert.equal(button(v,'发送').disabled,false);
    assert.equal(f.directoryReads,0,'opening a conversation does not depend on the directory');
    if(source==='detail')assert.match(v.host.querySelector('.replica-moment-card').textContent,/角色资料暂未读全/);
    else if(source!=='none')assert.match(v.host.querySelector('[data-testid=current-outfit]').textContent,/暂未读取到穿着/);
    else assert.match(v.host.querySelector('[data-testid=current-outfit]').textContent,/还没有登记实际穿着/);
    assert.ok(f.reads.some(p=>p.includes('/inventory/outfits?cursor=')),'detail failure does not skip wardrobe');
    assert.doesNotMatch(v.host.textContent,/INTERNAL_KEY|Service Unavailable/);
  }finally{await f.close();}
});

for(const source of ['detail','outfits'])
test(`typed draft survives conversation failure and retry while ${source} stays unavailable`,async()=>{
  const f=await fixture(source);
  try{
    await f.w.load('聊天');
    const v=await f.mount(),field=v.host.querySelector('textarea');
    await v.input(field,'这段草稿要保留');assert.equal(button(v,'发送').disabled,false);
    // Trigger an explicit read after typing; the mocked message read then fails.
    await act(async()=>{store.error='取消请求结果尚未确认，请刷新进度核对。原消息已保留。';});await v.flush();
    f.setConversationFailure(true);await v.click(button(v,'重新读取'));
    assert.equal(field.disabled,true);assert.equal(button(v,'发送').disabled,true);
    assert.ok(v.host.querySelector('[data-testid=chat-read-error]'));
    assert.equal(field.value,'这段草稿要保留');
    f.setConversationFailure(false);await v.click(button(v,'重新读取'));
    assert.equal(field.disabled,false);assert.equal(button(v,'发送').disabled,false);
    assert.equal(field.value,'这段草稿要保留');assert.equal(v.host.querySelector('[data-testid=chat-read-error]'),null);
    assert.ok(f.w.state.errors[source==='detail'?'detail':'wardrobe'],'auxiliary failure remains local after conversation recovery');
  }finally{await f.close();}
});

test('persona preparation detail refresh failure remains local',async()=>{
  const f=await fixture('detail');
  try{
    const v=await f.mount();
    await act(async()=>{store.error='persona_preparation_failed';});await v.flush();
    assert.ok(f.w.state.errors.detail);
    assert.equal(v.host.querySelector('textarea').disabled,false);
    assert.equal(v.host.querySelector('[data-testid=chat-read-error]'),null);
    await v.input(v.host.querySelector('textarea'),'保留当前输入');assert.equal(button(v,'发送').disabled,false);
    assert.match(v.host.querySelector('.replica-moment-card').textContent,/角色资料暂未读全/);
  }finally{await f.close();}
});

import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {React,act,installDOM,mountReact} from './helpers/react-dom.mjs';

const env=installDOM();
const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const load=path=>vite.ssrLoadModule('/src/'+path);
const {default:App}=await load('react-app/App.tsx');
const {productConversation:store}=await load('state/conversation.ts');
const directoryStore=await load('v3/proto/store.js');
const {BrowserClient}=await vite.ssrLoadModule('/@fs'+fileURLToPath(new URL('../../../packages/browser-client/src/index.ts',import.meta.url)));
// Match workspace-bootstrap-recovery: provider registration owns an app-lifetime
// notification interval, which this isolated App test must release.
const provider=await load('v3/providers/real.ts');
const providerIntervals=[],originalSetInterval=globalThis.setInterval;
try{
  globalThis.setInterval=(...args)=>{const timer=originalSetInterval(...args);providerIntervals.push(timer);return timer;};
  provider.registerProviders();
}finally{globalThis.setInterval=originalSetInterval;}
test.after(async()=>{providerIntervals.forEach(clearInterval);await vite.close();env.restore();});

// The detail is a top-level object; conversation/participants/messages are siblings.
// Directory membership and participant status do not indicate lifecycle status.
const archivedId='fluctlight_67b3aecda7065c41e8724d0a15796047';
const owner='human_293776c7339e39a3e0acb21ac020e42d';
const conversationId='conversation_539727d51617443036b918b5fd0dc95d';
const instant='2026-11-17T03:36:13.812+00:00';
const directory=[{id:'fl-chenmo',status:'active'},{id:'fl-suqing',status:'active'},{id:'fl-baiban',status:'active'},{id:'fluctlight_c187d20ec29d6dcbd36af29f09e98a36',status:'active'}];
const detail=(id,status)=>({id,status,identity:{id,name:'验收归档角色',age:null,background:null,background_story:null,biography:null,birthday:null,birthplace:null,blood_type:null,core_values:[],gender:null,height:null,height_cm:null,nickname:null,notes:null,occupation:null,residence:null,timezone:'Asia/Shanghai',worldview:null},context:{activity:null,authority_status:'pending',context_revision:'life_ctx_fixture',effective_at:'2026-11-16T16:00:00.000+00:00',expires_at:'2026-11-17T16:00:00.000+00:00',instant:'2026-11-17T03:50:45.090+00:00',local_date:'2026-11-17',location:null,revision:911438236,scene:null,source:'pending',timezone:'Asia/Shanghai'}});
const page=(messages,id=archivedId)=>({conversation:{createdAt:instant,createdByActorId:owner,id:conversationId,revision:0,title:null,updatedAt:instant},messages,nextBeforeSequence:null,participants:[{actorId:id,conversationId,joinedAt:instant,leftAt:null,role:'member',status:'active'},{actorId:owner,conversationId,joinedAt:instant,leftAt:null,role:'owner',status:'active'}]});
const history={id:'historical-message',conversationId,sequence:1,authorActorId:owner,kind:'user',text:'这是一条保留的历史消息',attachmentRefs:[],createdAt:instant};
const button=(v,text)=>[...v.host.querySelectorAll('button')].find(n=>n.textContent.trim()===text);
async function fixture({width=1280,status='retired',messages=[],detailStatus=200,id=archivedId}={}){
  sessionStorage.clear();localStorage.clear();store.$reset();
  const original=BrowserClient.prototype.json,reads=[];
  let failure=detailStatus;
  BrowserClient.prototype.json=async function(path,options){
    assert.ok(!options?.method||options.method==='GET','all API calls must be mocked GETs');
    reads.push(String(path));
    const pathname=String(path).split('?')[0];
    if(pathname==='/auth/session')return {authenticated:true,actorId:owner};
    if(pathname==='/api/fluctlights')return structuredClone(directory);
    if(pathname.endsWith('/detail')){
      if(failure!==200)throw Object.assign(new Error('Service Unavailable INTERNAL_KEY'),{status:failure});
      const requestedId=decodeURIComponent(pathname.split('/')[3]);
      return detail(requestedId,requestedId===id?status:'active');
    }
    if(pathname.endsWith('/conversation'))return page(structuredClone(messages),decodeURIComponent(pathname.split('/')[3]));
    if(pathname.endsWith('/messages'))return {messages:structuredClone(messages),nextBeforeSequence:null};
    if(pathname.endsWith('/wardrobe'))return {revision:1,worn_items:[],items:[]};
    if(pathname.endsWith('/outfits'))return {items:[],next_cursor:''};
    if(pathname.endsWith('/tags'))return {tags:[]};
    if(pathname.includes('unread'))return {count:0,alert_count:0};
    if(pathname.endsWith('/inventory'))return {items:[],next_cursor:''};
    if(pathname.includes('catalogue'))return {items:[]};
    throw new Error('Unexpected mocked GET: '+pathname);
  };
  function DirectApp(){
    window.innerWidth=width;
    historyObject();
    return React.createElement(App);
  }
  function historyObject(){window.history.replaceState(null,'',`#/c/${id}/chat`);}
  let v;
  try{
    v=await mountReact(vite,DirectApp);
    for(let i=0;i<40&&!v.host.querySelector('textarea')&&!/角色不存在|暂时无法读取角色/.test(v.host.textContent);i++)await v.flush();
    return {v,reads,succeed(){failure=200;},async close(){await v.dispose();BrowserClient.prototype.json=original;store.$reset();}};
  }catch(cause){if(v)await v.dispose();BrowserClient.prototype.json=original;throw cause;}
}

for(const width of [1280,390])for(const messages of [[history],[]])
test(`direct archived chat keeps shell and read-only ${messages.length?'history':'empty state'} at ${width}px`,async()=>{
  const f=await fixture({width,messages}),{v}=f;
  try{
    assert.ok(v.host.querySelector('.replica-topbar'));
    assert.ok(v.host.querySelector('[aria-label="打开导航"], [aria-label="收起导航"]'));
    if(width===1280)assert.ok(v.host.querySelector('.replica-sidebar'));
    assert.doesNotMatch(v.host.textContent,/角色不存在|INTERNAL_KEY|Service Unavailable/);
    assert.match(v.host.querySelector('[data-testid=chat-archived]').textContent,/角色已归档.*不能发送新消息/);
    assert.equal(v.host.querySelector('[data-testid=chat-read-error]'),null);
    assert.match(v.host.querySelector('.companion-message-list').textContent,messages.length?/这是一条保留的历史消息/:/还没有历史对话/);
    const field=v.host.querySelector('textarea');
    assert.equal(field.disabled,true);assert.equal(button(v,'发送').disabled,true);
    await v.input(field,'禁止发送的新消息');
    await act(async()=>field.dispatchEvent(new window.KeyboardEvent('keydown',{key:'Enter',bubbles:true})));
    await v.click(button(v,'发送'));
    assert.ok(f.reads.includes(`/api/fluctlights/${archivedId}/detail`));
    assert.ok(f.reads.includes(`/api/fluctlights/${archivedId}/conversation`));
    assert.deepEqual(Object.keys(directoryStore.get('characters')),directory.map(row=>row.id),'archive never enters the directory');
    // The chat character selector continues to contain only directory characters.
    await v.click(v.host.querySelector('[aria-label="切换聊天角色"]'));
    assert.equal([...document.querySelectorAll('[role=option]')].some(n=>n.textContent.includes('验收归档角色')),false);
    await act(async()=>document.dispatchEvent(new window.KeyboardEvent('keydown',{key:'Escape',bubbles:true})));
    await v.flush();
    const back=v.host.querySelector('[data-testid=chat-archived] a');
    assert.equal(back.hash,'#/library');
    assert.equal(back.textContent,'返回角色库');
    assert.equal(back.getAttribute('aria-disabled'),null);
  }finally{await f.close();}
});

test('detail 404 keeps the real character-not-found page',async()=>{
  const f=await fixture({detailStatus:404});
  try{assert.match(f.v.host.textContent,/角色不存在/);assert.equal(f.v.host.querySelector('textarea'),null);assert.equal(f.reads.some(path=>path.endsWith('/conversation')),false);assert.equal(f.v.host.querySelector('a').hash,'#/library');}finally{await f.close();}
});

test('detail service failure is retryable and is not reported as missing',async()=>{
  const f=await fixture({detailStatus:503});
  try{
    assert.match(f.v.host.textContent,/暂时无法读取角色/);assert.doesNotMatch(f.v.host.textContent,/角色不存在|INTERNAL_KEY|Service Unavailable/);
    f.succeed();await f.v.click(button(f.v,'重新读取'));
    for(let i=0;i<20&&!f.v.host.querySelector('textarea');i++)await f.v.flush();
    assert.ok(f.v.host.querySelector('[data-testid=chat-archived]'));
  }finally{await f.close();}
});

test('active directory character retains an enabled composer and has no archive notice',async()=>{
  const f=await fixture({id:directory[0].id,status:'active'}),{v}=f;
  try{
    assert.ok(v.host.querySelector('.replica-topbar'));assert.equal(v.host.querySelector('[data-testid=chat-archived]'),null);
    const field=v.host.querySelector('textarea');assert.equal(field.disabled,false);
    await v.input(field,'今天想聊聊');assert.equal(button(v,'发送').disabled,false);
    assert.doesNotMatch(v.host.textContent,/角色已归档/);
  }finally{await f.close();}
});


test('returning from an active chat selects the independently resolved archived character',async()=>{
  const f=await fixture(),{v}=f;
  const visit=async id=>{await act(async()=>{location.hash=`#/c/${id}/chat`;window.dispatchEvent(new window.Event('hashchange'));});await v.flush();};
  try{
    await visit(directory[0].id);
    assert.equal(v.host.querySelector('[data-testid=chat-archived]'),null);
    assert.equal(v.host.querySelector('textarea').disabled,false);
    await visit(archivedId);
    assert.match(v.host.querySelector('.companion-chat-person').textContent,/验收归档角色/);
    assert.ok(v.host.querySelector('[data-testid=chat-archived]'));
    assert.equal(v.host.querySelector('textarea').disabled,true);
    assert.equal(store.fluctlightId,archivedId);
  }finally{await f.close();}
});

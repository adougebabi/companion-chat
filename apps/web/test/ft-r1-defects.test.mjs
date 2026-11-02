import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {act,installDOM,mountReact} from './helpers/react-dom.mjs';

const environment=installDOM(),originalFetch=globalThis.fetch;
let handleRequest=()=>{throw Error('Unexpected request');};
globalThis.fetch=(...args)=>handleRequest(...args);
const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const load=path=>vite.ssrLoadModule('/src/'+path);
const {createConversationController}=await load('state/conversation.ts');
const {createWorkspace}=await load('state/workspace.mjs');
const domain=await load('v3/proto/store.js');
const {default:Chat}=await load('react-app/pages/ChatPage.tsx');
const button=(host,label)=>[...host.querySelectorAll('button')].find(node=>node.textContent.trim()===label);
test.after(async()=>{await vite.close();globalThis.fetch=originalFetch;environment.restore();});

async function chatFixture({readbackFailure=false}={}) {
  localStorage.clear();sessionStorage.clear();
  const w=createWorkspace(),requests=[];
  domain.replaceSnapshot({...domain.get(),session:{actorId:'ft-r1-owner'},characters:{A:{id:'A',name:'青岚',status:'正常'}},ui:{charId:'A'}});
  let status='failed',failRead=false;
  handleRequest=async(input,init={})=>{
    const url=String(input),method=init.method||'GET';
    requests.push({url,method,body:init.body?JSON.parse(init.body):undefined});
    if(url.endsWith('/persona-preparation/recover')) {
      status='pending';failRead=readbackFailure;
      return Response.json({status,revision:4,can_recover:false});
    }
    if(url.endsWith('/detail')) {
      if(failRead)throw Error('准备请求已有回执，状态读取暂时失败');
      return Response.json({current_revision:4,identity:{name:'青岚'},persona_preparation:{id:'persona:actual-task/A',status,revision:4,can_recover:status==='failed'}});
    }
    throw Error('Unexpected request '+url);
  };
  await w.detail();
  const store=createConversationController();store.fluctlightId='A';store.conversation={id:'C'};
  Object.assign(w,{conversation:()=>store,openConversation:async()=>{}});
  let view=await mountReact(vite,Chat,{workspace:w,props:{preview:false,onGo(){}}});
  return {w,store,get view(){return view;},requests,setStatus:value=>{status=value;},allowRead:()=>{failRead=false;},remount:async()=>{await view.dispose();view=await mountReact(vite,Chat,{workspace:w,props:{preview:false,onGo(){}}});},dispose:async()=>{await view.dispose();w.dispose();}};
}

test('D1 chat exposes persona recovery endpoint, retains draft, and enables sending only after readiness readback',async()=>{
  const f=await chatFixture();
  try {
    const field=f.view.host.querySelector('textarea');
    await f.view.input(field,'准备完成后再发送的草稿');
    assert.match(f.view.host.querySelector('[data-feedback-kind=action-required]').textContent,/人格资料准备失败/);
    const preparation=f.view.host.querySelector('[aria-label="人格准备与恢复"]');
    assert.ok(preparation.compareDocumentPosition(f.view.host.querySelector('[aria-label="聊天消息"]'))&Node.DOCUMENT_POSITION_FOLLOWING,'recovery renders before the chat messages without opening another page');
    const taskLink=[...preparation.querySelectorAll('a')].find(link=>link.textContent==='查看人格准备任务');
    const taskQuery=new URLSearchParams(taskLink.hash.split('?')[1]);
    assert.equal(taskQuery.get('taskId'),'persona:actual-task/A');assert.equal(taskQuery.get('returnTo'),'/c/A/chat');
    assert.equal([...preparation.querySelectorAll('a')].find(link=>link.textContent==='检查模型用途').hash,'#/settings?section='+encodeURIComponent('模型用途'));
    assert.equal(button(f.view.host,'发送').disabled,true);
    await f.view.click(button(f.view.host,'继续准备人格'));
    const posts=f.requests.filter(r=>r.method==='POST');
    assert.equal(posts.length,1);assert.match(posts[0].url,/\/api\/fluctlights\/A\/persona-preparation\/recover$/);
    assert.equal(posts[0].body.expectedRevision,4);assert.ok(posts[0].body.idempotencyKey);
    assert.match(f.view.host.textContent,/人格资料正在准备/);assert.equal(field.value,'准备完成后再发送的草稿');
    f.setStatus('ready');await f.view.click(button(f.view.host,'重新读取准备状态'));
    assert.doesNotMatch(f.view.host.textContent,/人格资料准备失败|人格资料正在准备/);
    assert.equal(button(f.view.host,'发送').disabled,false);assert.equal(field.value,'准备完成后再发送的草稿');
    assert.equal(f.requests.filter(r=>r.method==='POST').length,1);
  } finally {await f.dispose();}
});

test('D1 accepted persona recovery retries failed readback with GET only and keeps the original draft',async()=>{
  const f=await chatFixture({readbackFailure:true});
  try {
    await f.view.input(f.view.host.querySelector('textarea'),'保留原输入');
    await f.view.click(button(f.view.host,'继续准备人格'));
    assert.match(f.view.host.textContent,/状态读取暂时失败/);
    assert.equal(f.requests.filter(r=>r.method==='POST').length,1);
    f.allowRead();await f.view.click(button(f.view.host,'重新读取准备结果'));
    assert.equal(f.requests.filter(r=>r.method==='POST').length,1);
    assert.match(f.view.host.textContent,/人格资料正在准备/);
    assert.equal(f.view.host.querySelector('textarea').value,'保留原输入');
  } finally {await f.dispose();}
});

test('D1 remounted chat resumes accepted persona recovery with GET only and restores the draft',async()=>{
  const f=await chatFixture({readbackFailure:true});
  try {
    await f.view.input(f.view.host.querySelector('textarea'),'离开页面后仍保留的输入');
    await f.view.click(button(f.view.host,'继续准备人格'));
    assert.equal(f.requests.filter(r=>r.method==='POST').length,1);
    await f.remount();
    assert.equal(f.view.host.querySelector('textarea').value,'离开页面后仍保留的输入');
    f.allowRead();
    await f.view.click(button(f.view.host,'重新读取准备结果'));
    assert.equal(f.requests.filter(r=>r.method==='POST').length,1);
    assert.match(f.view.host.textContent,/人格资料正在准备/);
    assert.equal(f.view.host.querySelector('textarea').value,'离开页面后仍保留的输入');
  } finally {await f.dispose();}
});

test('D1 chat reads accepted recovery from an unmounted page and preserves GET-only retry when that read fails',async()=>{
  const f=await chatFixture({readbackFailure:true});
  try {
    await f.view.input(f.view.host.querySelector('textarea'),'跨页面保留原草稿');
    let sourcePageActive=true;
    let original;
    await act(async()=>{
      original=f.w.command('角色详情','persona-preparation:A',{expectedRevision:4},
        (value,idempotencyKey)=>f.w.client.recoverPersonaPreparation('A',{...value,idempotencyKey}),
        {onSaved:async()=>{if(sourcePageActive)await f.w.detail();}});
      await assert.rejects(original.completion,/状态读取暂时失败/);
    });
    sourcePageActive=false;
    await f.remount();
    await f.view.click(button(f.view.host,'重新读取准备结果'));
    assert.equal(original.readbackPending,true);
    assert.match(f.view.host.textContent,/状态读取暂时失败/);
    assert.equal(button(f.view.host,'发送').disabled,true);
    assert.equal(f.requests.filter(r=>r.method==='POST').length,1);
    f.allowRead();
    await f.view.click(button(f.view.host,'重新读取准备结果'));
    assert.equal(original.readbackPending,false);
    assert.match(f.view.host.textContent,/人格资料正在准备/);
    assert.equal(f.requests.filter(r=>r.method==='POST').length,1);
    assert.equal(f.view.host.querySelector('textarea').value,'跨页面保留原草稿');
  } finally {await f.dispose();}
});

test('D1 explicit preparation rejection is readable, removes optimistic turn, and does not acknowledge or lose the draft',async()=>{
  const {chatFailureText}=await load('v3/chat-feedback.mjs');
  for(const code of ['persona_preparation_failed','persona_preparation_pending']) {
    localStorage.clear();
    handleRequest=async(input)=>String(input).endsWith('/turn')?Response.json({code,message:'人格资料未准备完成'},{status:409}):Response.json({conversation:{id:'C'},messages:[],participants:[]});
    const c=createConversationController();c.fluctlightId='A';c.conversation={id:'C'};
    let accepted=0;
    assert.equal(await c.send('尚未被接受的输入',false,null,()=>accepted++),false);
    assert.equal(accepted,0);assert.equal(c.messages.length,0);assert.equal(c.retryTurn,null);
    assert.ok(c.error.includes(code));assert.match(chatFailureText(c.error),/人格资料/);
    assert.doesNotMatch(chatFailureText(c.error),/超时|连接失败/);
  }
});

test('D6 disabled vision explains preserved unreviewed candidate and offers actual settings and manual review entries',async()=>{
  const {VisualIdentityState}=await load('v3/proto/components/visual-identity-records.js');
  const {approvedRoute}=await load('approved-ui/routing.mjs');
  const host=document.createElement('div');
  host.innerHTML=VisualIdentityState({current_revision:0,status:'pending',session:{id:'S',status:'awaiting_review',last_error:'vision_disabled',current_attempt:1,attempts:[{attempt_number:1,candidate_asset_id:'saved-candidate',status:'awaiting_review'}]}});
  assert.ok([...host.querySelectorAll('.banner')].some(node=>node.textContent.includes('视觉理解未启用')));
  assert.match(host.textContent,/候选图片已保存，尚未经视觉检验/);
  assert.match(host.textContent,/独立视觉模型/);
  assert.equal(approvedRoute(host.querySelector('a[href*="section="]').hash).query.get('section'),'看图理解');
  assert.ok(host.querySelector('[data-manual-review]'));
  assert.match(host.querySelector('img').src,/saved-candidate/);
});

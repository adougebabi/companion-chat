import test from 'node:test';
import assert from 'node:assert/strict';
import {activeChatFixture} from './support/active-chat.mjs';
test('incoming chat request preserves and explicitly joins the existing draft without auto-send',async()=>{
 const f=await activeChatFixture(),props={preview:false,onGo(){}};
 try{
  const input=f.view.host.querySelector('textarea');await f.view.input(input,'正在写的草稿');
  await f.view.update({...props,route:{fid:'role',query:new URLSearchParams({say:'来自世界的请求'})}});
  assert.equal(input.value,'正在写的草稿');assert.match(f.view.host.textContent,/已保留你的草稿/);
  await f.view.click(f.button('加入草稿'));assert.equal(input.value,'正在写的草稿\n\n来自世界的请求');
  assert.equal(f.calls.some(c=>c[0]==='send'),false);
  await f.view.update({...props,route:{fid:'role',query:new URLSearchParams({say:'第二条请求'})}});
  await f.view.click(f.view.host.querySelector('[aria-label="放下这条请求"]'));
  assert.equal(input.value,'正在写的草稿\n\n来自世界的请求');
  await f.view.update({...props,route:{fid:'other-role',query:new URLSearchParams({say:'另一个角色的请求'})}});
  assert.equal(f.button('加入草稿'),undefined);assert.doesNotMatch(input.value,/另一个角色/);
  await f.view.input(input,'  ');
  await f.view.update({...props,route:{fid:'role',query:new URLSearchParams({say:'  保留来源空格  '})}});
  assert.equal(input.value,'  保留来源空格  ');assert.equal(f.calls.some(c=>c[0]==='send'),false);
 }finally{await f.dispose();}
});

test('next arrangement is a future accepted item rather than the current action',async()=>{
 const {nextScheduleItem,relativeTime,compactDateTime}=await import('../src/approved-ui/presentation.mjs');
 const now=Date.parse('2026-10-27T07:45:00Z');
 const row=(id,start,status=null)=>({id,_source:{start_at:start,action_status:status,status:'scheduled'}});
 const values=[row('current','2026-10-27T06:00:00Z','in_progress'),row('late','2026-10-27T12:00:00Z'),row('cancelled','2026-10-27T08:00:00Z','cancelled'),row('next','2026-10-27T09:30:00Z'),row('past','2026-10-27T02:00:00Z','completed')];
 assert.equal(nextScheduleItem(values,now).id,'next');assert.deepEqual(values.map(v=>v.id),['current','late','cancelled','next','past']);
 assert.equal(nextScheduleItem([values[0],values[2],values[4]],now),undefined);
 assert.equal(relativeTime('2026-10-27T07:45:00Z','Asia/Shanghai',now),'刚刚');
 assert.equal(relativeTime('2026-10-27T05:45:00Z','Asia/Shanghai',now),'2 小时前');
 assert.equal(relativeTime('2026-10-26T07:45:00Z','Asia/Shanghai',now),'昨天');
 assert.equal(relativeTime(undefined,'Asia/Shanghai',now),'');
 assert.equal(compactDateTime('2026-10-26T01:30:00Z'),'10/26 09:30');
 assert.equal(compactDateTime('2026-10-27T07:44:00Z','Asia/Shanghai',now,true),'今天 · 15:44');
 assert.equal(compactDateTime('2026-10-26T01:30:00Z','Asia/Shanghai',now,true),'昨天 · 09:30');
 assert.equal(compactDateTime(undefined),'');
 const {viewFromBundle}=await import('../src/v3/contract-map.mjs');
 const p=viewFromBundle({session:{actorId:'owner'},fluctlights:[{id:'ordinary-id',identity:{name:'林青',color:'#dce7d7',occupation:'插画师'},status:'active',last_conversation_at:'2026-10-27T07:45:00Z'}]});
 assert.equal(p.characters['ordinary-id'].lastConversationAt,'2026-10-27T07:45:00Z');
 assert.equal(p.characters['ordinary-id'].color,'#dce7d7');
 assert.equal(p.characters['ordinary-id'].job,'插画师');
});

test('chat keeps the real attachment inside its bubble, retries the image and restores dialog focus',async()=>{
 const f=await activeChatFixture();
 try{
  await f.update(()=>{f.store.messages=[{id:'image-message',kind:'assistant',text:'给你看看这里。',createdAt:'2026-10-27T07:44:00Z',attachmentRefs:['real-image']}];});
  const image=f.view.host.querySelector('.companion-bubble img');
  assert.ok(image);assert.equal(image.getAttribute('src'),'/api/media/real-image');
  assert.match(image.closest('.companion-bubble').textContent,/给你看看这里。.*查看这个片刻/s);
  await f.update(()=>image.dispatchEvent(new window.Event('error')));
  assert.match(f.view.host.textContent,/图片暂时无法读取/);
  assert.equal(f.view.host.querySelector('[aria-label="查看对话附件"]'),null);
  const retryButton=f.view.host.querySelector('[aria-label="重新读取图片"]');retryButton.focus();
  await f.view.click(retryButton);
  assert.equal(document.activeElement,retryButton);
  assert.equal(retryButton,f.view.host.querySelector('[aria-label="查看对话附件"]'));
  const retry=f.view.host.querySelector('.companion-bubble img');assert.notEqual(retry,image);
  assert.equal(retry.getAttribute('src'),'/api/media/real-image');
  await f.update(()=>retry.dispatchEvent(new window.Event('load')));
  const trigger=f.view.host.querySelector('[aria-label="查看对话附件"]');
  await f.view.click(trigger);
  assert.equal(document.querySelector('[role="dialog"] img').getAttribute('src'),'/api/media/real-image');
  await f.update(()=>document.dispatchEvent(new window.KeyboardEvent('keydown',{key:'Escape',bubbles:true})));
  assert.equal(document.querySelector('[role="dialog"]'),null);
  assert.equal(document.activeElement,trigger);
  assert.equal(f.calls.some(c=>c[0]==='send'),false);
 }finally{await f.dispose();}
});

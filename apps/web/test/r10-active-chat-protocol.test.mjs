import test from 'node:test';
import assert from 'node:assert/strict';
import {activeChatFixture as fixture} from './support/active-chat.mjs';
test('React chat opens the selected conversation and retains drafts while rendering original attachments',async()=>{
 const f=await fixture();try{
  assert.deepEqual(f.calls[0],['open','role',true]);const field=f.view.host.querySelector('textarea');await f.view.input(field,'未提交的草稿');
  await f.update(()=>{f.store.messages=[{id:'answer',kind:'assistant',text:'原始正文 <tag>',createdAt:'2026-10-26T04:00:00Z',attachmentRefs:['real-asset']}];});
  assert.equal(field.value,'未提交的草稿');assert.match(f.view.host.innerHTML,/原始正文 &lt;tag&gt;/);assert.match(f.view.host.innerHTML,/\/api\/media\/real-asset/);
  assert.equal(f.button('回复设置'),undefined);assert.equal(field.value,'未提交的草稿');
 }finally{await f.dispose();}
});
test('React stop button follows cancellation authority and delegates to the original controller',async()=>{
 const f=await fixture();let allowed=false;try{
  Object.defineProperty(f.store,'canCancel',{configurable:true,get:()=>allowed});
  await f.update(()=>{f.store.sending=true;f.store.attemptStartedAt=Date.now()-120000;f.store.messages=[{id:'pending',kind:'user',text:'等待',turnId:'turn-real',turnStatus:'running',createdAt:new Date(Date.now()-120000).toISOString()}];});
  assert.equal(f.button('停止本次生成'),undefined);await f.update(()=>{allowed=true;f.store.messages=[...f.store.messages];});
  assert.ok(f.button('停止本次生成'));await f.view.click(f.button('停止本次生成'));assert.deepEqual(f.calls.at(-1),['cancel']);
 }finally{await f.dispose();}
});
test('React failed-message retry and status reading preserve the original record identity',async()=>{
 const f=await fixture();try{
  const message={id:'original-user-message',kind:'user',text:'原消息',turnId:'original-turn',turnStatus:'failed',turnRetryable:true,turnErrorCode:'provider_http_error',createdAt:new Date().toISOString(),attachmentRefs:[]};
  await f.update(()=>{f.store.messages=[message];});await f.view.click(f.button('重试原回应'));assert.deepEqual(f.calls.at(-1),['retry','original-user-message']);
  await f.update(()=>{f.store.messages=[{...message,turnRetryable:false}];});await f.view.click(f.button('查看状态'));assert.deepEqual(f.calls.at(-1),['status',true]);
 }finally{await f.dispose();}
});

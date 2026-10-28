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
test('an unconfirmed cancellation stays visible while the original turn is still running',async()=>{
 const f=await fixture();try{
  Object.defineProperty(f.store,'canCancel',{configurable:true,get:()=>true});
  Object.defineProperty(f.store,'cancel',{configurable:true,value:async()=>{f.store.error='取消请求结果尚未确认，请刷新进度核对。原消息已保留。';}});
  await f.update(()=>{f.store.messages=[{id:'original-user',kind:'user',text:'保留的原消息',turnId:'original-turn',turnStatus:'running',createdAt:new Date().toISOString()}];});
  await f.view.click(f.button('停止本次生成'));await f.view.flush();
  const alert=f.view.host.querySelector('[role=alert]');assert.match(alert.textContent,/取消请求结果尚未确认，请刷新进度核对。原消息已保留。/);
  assert.ok(f.button('停止本次生成'));assert.equal(f.view.host.querySelectorAll('.companion-message.is-mine').length,1);assert.equal(f.button('重试原回应'),undefined);
  await f.view.click(f.button('重新读取对话'));assert.deepEqual(f.calls.at(-1),['status',true]);assert.equal(f.store.messages[0].turnStatus,'running');
 }finally{await f.dispose();}
});
test('React chat renders streaming deltas and replaces them with the committed reply without disturbing the draft',async()=>{
 const f=await fixture();try{const field=f.view.host.querySelector('textarea');await f.view.input(field,'下一句话的草稿');field.focus();field.setSelectionRange(2,4);await f.update(()=>{f.store.messages=[{id:'stream-current',kind:'assistant',text:'正在回应的第一段',createdAt:new Date().toISOString(),attachmentRefs:[]}];});assert.match(f.view.host.textContent,/正在回应的第一段/);await f.update(()=>{f.store.messages=[{...f.store.messages[0],text:'正在回应的第一段，继续增长。'}];});assert.match(f.view.host.textContent,/继续增长/);await f.update(()=>{f.store.messages=[{id:'committed-answer',kind:'assistant',text:'最终保存的回应',createdAt:new Date().toISOString(),attachmentRefs:[]}];});assert.doesNotMatch(f.view.host.textContent,/继续增长/);assert.match(f.view.host.textContent,/最终保存的回应/);assert.equal(f.view.host.querySelector('textarea'),field);assert.equal(field.value,'下一句话的草稿');assert.equal(field.selectionStart,2);assert.equal(field.selectionEnd,4);}finally{await f.dispose();}
});

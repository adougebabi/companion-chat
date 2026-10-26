import test from 'node:test';
import assert from 'node:assert/strict';
import {tick} from './support/sfc-harness.mjs';
import {activeChatFixture as fixture} from './support/active-chat.mjs';
test('actual chat opens the selected conversation and projects source messages without losing an in-progress draft',async()=>{
 const f=fixture();f.bindings.l.value='未提交的草稿';await f.mount();assert.deepEqual(f.calls[0],['open','role',true]);
 f.store.messages=[{id:'answer',kind:'assistant',text:'原始正文 <tag>',createdAt:'2026-10-26T04:00:00Z',attachmentRefs:['real-asset']}];f.project();
 assert.equal(f.bindings.l.value,'未提交的草稿');assert.equal(f.character.value.messages[0].text,'原始正文 <tag>');assert.deepEqual(f.character.value.messages[0].attachments,['real-asset']);const html=await f.html();assert.match(html,/原始正文 &lt;tag&gt;/);assert.match(html,/\/api\/media\/real-asset/);assert.match(html,/aria-label="聊天消息"/);f.dispose();
});
test('actual chat sends once, clears only acknowledged input and preserves typing during the accepted request',async()=>{
 const f=fixture();let accept,finish;f.store.send=(text,_a,_b,ack)=>{f.calls.push(['send',text]);accept=ack;return new Promise(resolve=>finish=resolve);};f.bindings.l.value='第一条消息';const sent=f.bindings.u();await tick();f.bindings.l.value='随后输入的新草稿';accept();finish();await sent;assert.deepEqual(f.calls[0],['send','第一条消息']);assert.equal(f.bindings.l.value,'随后输入的新草稿');f.dispose();
});
test('actual stop button follows protocol cancellation authority, then delegates to the original conversation store',async()=>{
 const f=fixture();f.store.sending=true;f.store.messages=[{id:'pending',kind:'user',text:'等待',turnId:'turn-real',turnStatus:'running',createdAt:new Date(Date.now()-120000).toISOString()}];f.project();
 assert.equal(f.bindings.waiting.value.showStop,false);assert.doesNotMatch(await f.html(),/停止本次生成/);f.store.canCancel=true;
 // The fake store is intentionally reactive just like Pinia in production.
 assert.equal(f.bindings.waiting.value.showStop,true);assert.match(await f.html(),/停止本次生成/);f.bindings.cancel();assert.deepEqual(f.calls.at(-1),['cancel']);f.dispose();
});
test('actual failed-message retry and status read keep the original record identity',async()=>{
 const f=fixture();f.store.messages=[{id:'original-user-message',kind:'user',text:'原消息',turnId:'original-turn',turnStatus:'failed',turnRetryable:true,turnErrorCode:'provider_http_error',createdAt:new Date().toISOString(),attachmentRefs:[]}];f.project();await f.character.value.messages[0].action.run();assert.deepEqual(f.calls.at(-1),['retry','original-user-message']);await f.bindings.readStatus();assert.deepEqual(f.calls.at(-1),['status',true]);f.dispose();
});

import test from 'node:test';
import assert from 'node:assert/strict';
import {setupSFC,tick} from './support/sfc-harness.mjs';

test('the actual media library starts with every character even from a character bookmark, and preserves explicit filtering',async()=>{
 for(const filter of ['', 'other-role']){
  const calls=[],w={state:{session:{actorId:'owner'},characters:[],route:{fid:'selected-role',query:new URLSearchParams(filter?{fid:filter}:{})}},services:{mediaClient:{list:async options=>{calls.push(options);return {items:[],next_cursor:''};}}}};
  const f=setupSFC(new URL('../src/approved-ui/pages/MediaPage.vue',import.meta.url),{useWorkspace:()=>w,errorMessage:e=>e.message,mountMediaUpload:()=>({dispose(){}})},['w','rows','cursor','query','kind','fid','origin','hidden','busy','error','uploadHost','uploadOpen','load','open','url','source']);
  await f.mount();await tick();
  assert.equal(f.state.fid.value,filter);assert.equal(calls[0].fluctlightId,filter||undefined);assert.equal(calls[0].scope,'all');
  const html=await f.html();assert.match(html,/全部角色/);assert.match(html,/上传素材/);f.unmount();
 }
});

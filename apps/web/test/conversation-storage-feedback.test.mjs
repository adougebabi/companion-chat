import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {installDOM} from './helpers/react-dom.mjs';
const environment=installDOM(),descriptor=Object.getOwnPropertyDescriptor(globalThis,'localStorage'),storage=globalThis.localStorage;
const original={conversationId:'original-conversation',fluctlightId:'original-role',text:'原请求正文',turnId:'original-turn',idempotencyKey:'original-key',attachmentRefs:[]};
const raw=JSON.stringify({'["original-role","original-conversation"]':original});storage.setItem('fluctlight.retry-turns.v3',raw);
let readable=false;Object.defineProperty(globalThis,'localStorage',{configurable:true,get(){if(!readable)throw new DOMException('Controlled storage getter failure','SecurityError');return storage;}});
const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const {createConversationController}=await vite.ssrLoadModule('/src/state/conversation.ts');
test.after(async()=>{readable=true;Object.defineProperty(globalThis,'localStorage',descriptor);await vite.close();environment.restore();});
test('unreadable original request storage cannot crash startup or admit a fresh turn; reread restores exact identity without changing its server state',async()=>{
 const c=createConversationController();c.conversation={id:'original-conversation'};c.fluctlightId='original-role';c.messages=[{id:'original-message',kind:'user',text:'原请求正文',turnId:'original-turn',turnStatus:'running'}];
 assert.match(c.recoveryStorageError,/原请求记录暂时无法读取/);assert.equal(await c.send('尚未提交的新输入'),false);assert.equal(c.error,c.recoveryStorageError);assert.equal(c.retryTurn,null);
 readable=true;assert.equal(c.rereadLocalRecovery(),true);assert.equal(c.recoveryStorageError,'');assert.equal(c.retryTurn.turnId,original.turnId);assert.equal(c.retryTurn.idempotencyKey,original.idempotencyKey);assert.equal(c.messages[0].turnStatus,'running');assert.equal(storage.getItem('fluctlight.retry-turns.v3'),raw);
});
test('runtime archive read failure blocks a queued send and never overwrites another original record, including a write-boundary failure',async()=>{
 readable=true;const proto=Object.getPrototypeOf(storage),get=proto.getItem;
 const foreign={...original,conversationId:'foreign-conversation',messageId:'foreign-message',createdAt:'2026-10-29T00:00:00Z'},foreignRaw=JSON.stringify({'["original-role","foreign-conversation"]':foreign});
 for(const failAt of [1,2]){
  const c=createConversationController();c.conversation={id:'current-conversation'};c.fluctlightId='current-role';c.messages=[];c.sending=true;
  storage.setItem('fluctlight.queued-turns.v2',foreignRaw);let reads=0,accepted=0;
  proto.getItem=function(key){if(key==='fluctlight.queued-turns.v2'&&++reads===failAt)throw new DOMException('Controlled runtime storage failure','SecurityError');return get.call(this,key);};
  try{assert.equal(await c.send('尚未发送的新追发',false,null,()=>accepted++),false);assert.equal(accepted,0);assert.equal(c.queuedTurn,null);assert.equal(c.messages.length,0);assert.match(c.recoveryStorageError,/原请求记录暂时无法读取/);assert.equal(get.call(storage,'fluctlight.queued-turns.v2'),foreignRaw);}
  finally{proto.getItem=get;c.sending=false;assert.equal(c.rereadLocalRecovery(),true);storage.removeItem('fluctlight.queued-turns.v2');}
 }
});

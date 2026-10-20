import test from 'node:test';
import assert from 'node:assert/strict';
import {webcrypto} from 'node:crypto';
import {readdir,readFile} from 'node:fs/promises';
import {randomId} from '../src/random-id.mjs';
import {creationDraft} from '../src/v3/creation-draft.mjs';
import {persistentCommand} from '../src/v3/providers/persistent-command.mjs';

const uuid=/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
async function withCrypto(value,run){
 const descriptor=Object.getOwnPropertyDescriptor(globalThis,'crypto');
 Object.defineProperty(globalThis,'crypto',{configurable:true,value});
 try{return await run();}finally{if(descriptor)Object.defineProperty(globalThis,'crypto',descriptor);else delete globalThis.crypto;}
}
test('native UUID retains its Crypto receiver',async()=>{
 const provider={randomUUID(){assert.equal(this,provider);return 'native-id';}};
 await withCrypto(provider,()=>assert.equal(randomId(),'native-id'));
});
test('HTTP-compatible secure random bytes produce unique v4 UUIDs without native randomUUID',async()=>{
 const provider={getRandomValues:bytes=>webcrypto.getRandomValues(bytes)};
 await withCrypto(provider,()=>{
  const ids=Array.from({length:1000},()=>randomId());
  assert.ok(ids.every(id=>uuid.test(id)));assert.equal(new Set(ids).size,ids.length);
 });
 await withCrypto({getRandomValues:bytes=>bytes.fill(255)},()=>assert.equal(randomId(),'ffffffff-ffff-4fff-bfff-ffffffffffff'));
});
test('missing secure randomness is an explicit error rather than a weak or fixed fallback',async()=>{
 for(const provider of [undefined,{}])await withCrypto(provider,()=>assert.throws(()=>randomId(),/安全随机数/));
});
test('HTTP draft creation and unknown-outcome retry keep the same persisted request identity',async()=>{
 await withCrypto({getRandomValues:bytes=>webcrypto.getRandomValues(bytes)},async()=>{
  const data=new Map(),storage={getItem:key=>data.get(key)||null,setItem:(key,value)=>data.set(key,value),removeItem:key=>data.delete(key)};
  const draft=creationDraft({owner:'owner',storage}).read();assert.match(draft.requestId,uuid);
  const keys=[];let fail=true;
  const command=persistentCommand({key:'http-command',storage:()=>storage,execute:async(_,key)=>{keys.push(key);if(fail)throw new Error('response lost');return {id:'server-generated'};}});
  await assert.rejects(command.submit({name:'original'}),/response lost/);
  const key=command.pending().idempotencyKey;assert.match(key,uuid);
  fail=false;assert.equal((await command.retry()).id,'server-generated');assert.deepEqual(keys,[key,key]);
 });
});
test('all browser sources use the shared adapter instead of calling native randomUUID directly',async()=>{
 const root=new URL('../src/',import.meta.url),violations=[];
 async function scan(folder){for(const entry of await readdir(folder,{withFileTypes:true})){
  const url=new URL(entry.name+(entry.isDirectory()?'/':''),folder);
  if(entry.isDirectory())await scan(url);
  else if(/\.(?:ts|js|mjs|vue)$/.test(entry.name)&&url.href!==new URL('random-id.mjs',root).href&&/\brandomUUID\b/.test(await readFile(url,'utf8')))violations.push(url.pathname);
 }}
 await scan(root);assert.deepEqual(violations,[],'A new direct native UUID dependency breaks HTTP deployments');
});

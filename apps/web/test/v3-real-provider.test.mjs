import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import * as store from '../src/v3/proto/store.js';

test('real facade starts without fixture facts and refuses arbitrary local writes',()=>{
  assert.equal(store.isMock,false);
  assert.deepEqual(store.list('characters'),[]);
  const snapshot=store.get();
  let called=false;
  assert.throws(()=>store.update(data=>{called=true;data.session={authenticated:true};},{identity:'系统运维'}),/尚未连接真实命令/);
  assert.equal(called,false);
  assert.equal(store.get(),snapshot);
  assert.equal(store.get('session'),null);
});
test('snapshot selection follows available server identities and never defaults to fixture c1',()=>{
  store.replaceSnapshot({...store.get(),characters:{'real-a':{id:'real-a'}},ui:{charId:'real-a',identity:'交流',notes:false}});
  assert.equal(store.get('ui').charId,'real-a');
  store.setUi({charId:'deleted'});
  store.replaceSnapshot({...store.get(),characters:{'real-b':{id:'real-b'}},ui:{charId:'real-b',identity:'交流',notes:false}});
  assert.equal(store.get('ui').charId,'real-b');
  assert.equal(store.find('c1'),null);
});
test('current chat uses protocol state while keeping the composer mounted during updates',async()=>{
  const source=await readFile(new URL('../src/v3/proto/pages/real-chat.js',import.meta.url),'utf8');
  assert.match(source,/openConversation\(c.id\)/);
  assert.match(source,/state\.\$subscribe\(update/);
  assert.match(source,/state\.send\(text\)/);
  assert.match(source,/state\.cancel\(\)/);
  assert.match(source,/state\.retry\(\)/);
  assert.match(source,/refreshActiveTurn\(\)/);
  const update=source.slice(source.indexOf('const update='),source.indexOf('const readScope='));
  assert.doesNotMatch(update,/el\.innerHTML|form\.innerHTML|form\.elements\.t\.value\s*=/);
});

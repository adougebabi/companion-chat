import assert from 'node:assert/strict';
import test from 'node:test';
import {readFile} from 'node:fs/promises';
import {Field} from '../src/v3/proto/components/forms.js';
import {CharacterCard} from '../src/v3/proto/components/character-card.js';

const mediaSource=(await readFile(new URL('../src/v3/proto/pages/real-media.js',import.meta.url),'utf8')).replace(/^import .*;\n/gm,'').replace(/^export /gm,'');
function mediaHarness(){
  const requests=[],uploads=[],form={},box={isConnected:true},host={isConnected:true,innerHTML:'',querySelector:selector=>selector==='[data-filters]'?form:box};
  const get=()=>({actorId:'owner'}),client={list:async value=>{requests.push(value);return {items:[]};},asset:async(...args)=>{requests.push(args);return {};}};
  const panel=(_box,options)=>{void options.read('');return {reload(){},dispose(){}};};
  const make=new Function('get','mediaClient','Field','esc','mountCursorRecords','MediaAssetCard','bindMediaFailures','mountMediaUpload','asyncPage',mediaSource+'\nreturn {renderRealMediaLibrary,renderRealMediaDetail};');
  const page=make(get,client,()=>'',String,panel,()=>'',()=>{},()=>{uploads.push(true);return {dispose(){}};},(_el,read)=>{void read();});
  return {host,requests,uploads,page};
}
test('global media library keeps its Owner scope even when a character remains selected in the shell',()=>{
  const h=mediaHarness();h.page.renderRealMediaLibrary(h.host,{charId:'selected-character',params:{},query:{}});
  assert.equal(h.requests[0].scope,'all');assert.equal(h.requests[0].fluctlightId,undefined);assert.equal(h.uploads.length,1);
  assert.match(h.host.innerHTML,/我的素材库/);assert.doesNotMatch(h.host.innerHTML,/角色媒体库/);
});
test('character media stays scoped while global media details do not inherit the selected character',()=>{
  const h=mediaHarness();h.page.renderRealMediaLibrary(h.host,{charId:'character-a',params:{charId:'character-a'},query:{}});
  assert.equal(h.requests[0].fluctlightId,'character-a');assert.equal(h.uploads.length,0);
  h.page.renderRealMediaDetail(h.host,{charId:'character-a',params:{mdId:'asset-1'},query:{}});
  assert.deepEqual(h.requests[1],['asset-1','']);
});
test('field hints retain caller error descriptions and each control has an independent accessible identity',()=>{
  const first=Field({label:'名称',name:'name',hint:'帮助文字',attrs:{required:true,'aria-describedby':'server-error'}});
  const second=Field({label:'名称',name:'name',hint:'另一项帮助'});
  const id=first.match(/<input[^>]* id="([^"]+)"/)[1],other=second.match(/<input[^>]* id="([^"]+)"/)[1];
  assert.notEqual(id,other);assert.match(first,new RegExp(`aria-describedby="server-error ${id}-hint"`));assert.match(first,/aria-required="true"/);
});
test('a character without an image still has a named conversation entry and separate management controls',()=>{
  const html=CharacterCard({id:'person',name:'<角色>',avatar:'',status:'正常',groupId:'ungrouped'},[]);
  assert.match(html,/aria-label="与&lt;角色&gt;对话"/);assert.match(html,/href="#\/c\/person\/chat"/);assert.match(html,/avatar-fallback/);
  assert.doesNotMatch(html,/<img[^>]*src=""/);assert.match(html,/data-lifecycle="person"/);assert.match(html,/data-grp="person"/);
});

import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {approvedRoute,workspaceReadingKey} from '../src/approved-ui/routing.mjs';
import {React,act,installDOM,mountReact} from './helpers/react-dom.mjs';
const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();env.restore();});
const {default:Library}=await vite.ssrLoadModule('/src/react-app/pages/LibraryPage.jsx');
function workspace(owner='replica-owner'){
 const state=proxy({session:{actorId:owner},selectedId:'A',characters:[{id:'A',name:'林青',job:'编辑',bio:'阅读和绘画',tags:['安静'],status:'正常'},{id:'B',name:'季川',job:'摄影师',bio:'山间行走',tags:['好奇'],status:'已暂停'}],errors:{},loading:{}}),moves=[],reads=[];
 return {state,moves,reads,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),services:{configurationClient:{read:async()=>({bindings:[]})}},select:id=>{state.selectedId=id;},navigate:page=>moves.push([page,state.selectedId]),load:async page=>reads.push(page)};
}
const button=(v,text)=>[...v.host.querySelectorAll('button')].find(node=>node.textContent===text);
test('library filters actual tags and text, switches views, navigates the chosen role and restores UI drafts per Owner',async()=>{
 const w=workspace();let v=await mountReact(vite,Library,{workspace:w});
 try{
  assert.equal(v.host.querySelectorAll('[data-testid^="library-card-"]').length,2);
  assert.equal(v.host.querySelector('.library-footer a').getAttribute('href'),'#/studio?kind=character_template');
  assert.match(v.host.querySelector('[data-testid="library-card-B"]').textContent,/已暂停/);
  await v.click(button(v,'好奇'));assert.equal(v.host.querySelectorAll('[data-testid^="library-card-"]').length,1);
  await v.click(button(v,'全部'));await v.input(v.host.querySelector('input[type=search]'),'编辑');
  assert.match(v.host.querySelector('[data-testid="library-grid"]').textContent,/林青/);assert.doesNotMatch(v.host.querySelector('[data-testid="library-grid"]').textContent,/季川/);
  await v.click(v.host.querySelector('[aria-label="列表视图"]'));assert.ok(v.host.querySelector('.character-list'));
  assert.deepEqual(JSON.parse(sessionStorage.getItem('fluctlight:library-view:replica-owner')),{search:'编辑',tag:'',view:'list'});
  await v.click(v.host.querySelector('[data-testid="library-card-A"] button:last-child'));assert.deepEqual(w.moves,[['聊天','A']]);
  await v.dispose();v=await mountReact(vite,Library,{workspace:w});assert.equal(v.host.querySelector('input').value,'编辑');assert.ok(v.host.querySelector('.character-list'));
  await v.input(v.host.querySelector('input'),'没有这位角色');assert.match(v.host.textContent,/没有找到这位相识/);await v.click(button(v,'清除筛选'));assert.equal(v.host.querySelectorAll('[data-testid^="library-card-"]').length,2);
  await v.click([...v.host.querySelector('[data-testid="library-card-B"]').querySelectorAll('button')].find(node=>node.textContent==='角色档案'));assert.deepEqual(w.moves.at(-1),['角色详情','B']);
  await v.input(v.host.querySelector('input'),'私有筛选');await v.dispose();v=await mountReact(vite,Library,{workspace:workspace('different-owner')});assert.equal(v.host.querySelector('input').value,'');
 }finally{await v.dispose();}
});
test('library loading and failure retain confirmed cards, recover through the real read action, and distinguish true empty directory',async()=>{
 const w=workspace('states-owner'),v=await mountReact(vite,Library,{workspace:w});
 try{
  await act(async()=>{w.state.loading.library=true;});await v.flush();assert.equal(v.host.querySelector('[aria-busy=true]')!==null,true);assert.match(v.host.textContent,/正在读取角色资料/);assert.equal(v.host.querySelectorAll('[data-testid^="library-card-"]').length,2);
  await act(async()=>{w.state.loading.library=false;w.state.errors.library='本次请求失败';});await v.flush();assert.match(v.host.querySelector('[role=alert]').textContent,/保留当前角色列表/);assert.equal(v.host.querySelectorAll('[data-testid^="library-card-"]').length,2);await v.click(button(v,'重新读取角色资料'));assert.deepEqual(w.reads,['角色库']);
  await act(async()=>{w.state.errors.library='';w.state.characters=[];w.state.selectedId='';});await v.flush();assert.match(v.host.textContent,/还没有一起生活的人/);assert.equal(v.host.querySelector('[data-testid=library-create-first]').getAttribute('href'),'#/library/create');assert.equal(v.host.querySelector('.library-resume'),null);
 }finally{await v.dispose();}
});
test('quick jump supports page and character selection and Escape closes without navigation',async()=>{
 const {default:QuickJump}=await vite.ssrLoadModule('/src/react-app/shared/WorkspaceQuickJump.tsx');const pages=[],roles=[];
 const v=await mountReact(vite,QuickJump,{props:{characters:[{id:'A',name:'林青'}],hasCharacter:true,onNavigate:page=>pages.push(page),onCharacter:id=>roles.push(id)}});
 try{
  const trigger=v.host.querySelector('button');trigger.focus();await v.click(trigger);const input=()=>document.querySelector('[aria-label="搜索页面或角色名字"]');await v.input(input(),'林青');await v.click([...document.querySelectorAll('[role=dialog] button')].find(node=>node.textContent.includes('进入聊天')));assert.deepEqual(roles,['A']);
  await act(async()=>window.dispatchEvent(new window.KeyboardEvent('keydown',{key:'k',ctrlKey:true,bubbles:true})));await v.flush();await v.input(input(),'日记');await act(async()=>document.dispatchEvent(new window.KeyboardEvent('keydown',{key:'Escape',bubbles:true})));await v.flush();assert.equal(document.querySelector('[role=dialog]'),null);assert.deepEqual(pages,[]);
  await v.click(trigger);await v.input(input(),'新的相识');await v.click([...document.querySelectorAll('[role=dialog] button')].find(node=>node.textContent.includes('新的相识')));assert.deepEqual(pages,['初始化角色']);
 }finally{await v.dispose();}
});

test('reading positions isolate Owners, character filters and sections while equivalent query order shares a position',()=>{
 const key=(owner,url)=>workspaceReadingKey(owner,approvedRoute(url));
 assert.notEqual(key('one','#/feed?fid=A'),key('one','#/feed?fid=B'));
 assert.notEqual(key('one','#/library'),key('two','#/library'));
 assert.notEqual(key('one','#/settings?section=服务地址'),key('one','#/settings?section=通知'));
 assert.equal(key('one','#/feed?fid=A&q=hello'),key('one','#/feed?q=hello&fid=A'));
});

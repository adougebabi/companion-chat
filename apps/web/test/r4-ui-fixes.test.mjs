import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {act,React,installDOM,mountReact} from './helpers/react-dom.mjs';

const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();env.restore();});
const load=path=>vite.ssrLoadModule('/src/'+path);
const button=(root,text)=>[...root.querySelectorAll('button')].find(node=>node.textContent.trim()===text);
const radio=(root,name)=>root.querySelector('[role=radio][aria-label="'+name+'"]');
function workspace(configuration={endpoints:[],bindings:[]}){
  sessionStorage.clear();localStorage.clear();
  const state=proxy({ready:true,session:{actorId:'r4-owner'},selectedId:'',characters:[],errors:{},loading:{}}),moves=[],mutations=[];
  return {state,moves,mutations,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),currentScope:()=>JSON.stringify(['r4-owner','']),
    services:{configurationClient:{read:async()=>{if(configuration instanceof Error)throw configuration;return configuration;}},studioClient:{},diagnosticsClient:{page:async()=>({items:[],next_cursor:''})}},
    client:{listStudioResources:async()=>({items:[],next_cursor:''}),listActorGroups:async()=>[],worldCatalog:async()=>({items:[]}),updateRuntimeWorld:async body=>mutations.push(body)},
    day:()=> '2026-11-02',refreshDirectory:async()=>{},navigatePath:path=>moves.push(path),select(){},navigate(){},load:async()=>{}};
}

test('P1 nginx revalidates direct and fallback HTML, caches successful hashed assets, and returns 404 for missing chunks',async()=>{
  const nginx=await readFile(new URL('../nginx.conf',import.meta.url),'utf8');
  const location=pattern=>nginx.match(new RegExp('location '+pattern+' \\{([^}]+)\\}'))?.[1];
  for(const block of [location('= /index\\.html'),location('/')]){
    assert.ok(block);assert.match(block,/add_header Cache-Control "no-cache" always;/);
  }
  assert.match(location('/'),/try_files \$uri \$uri\/ \/index\.html;/);
  const assets=location('/assets/');assert.match(assets,/public, max-age=31536000, immutable/);assert.match(assets,/try_files \$uri =404;/);
  assert.doesNotMatch(assets,/add_header Cache-Control [^;]+always;/,'a 404 must not receive a year-long immutable header');
  const entry=await readFile(new URL('../src/main.tsx',import.meta.url),'utf8');
  assert.match(entry,/<ChunkLoadBoundary><App\/><\/ChunkLoadBoundary>/,'the boundary must cover AuthPage and nested lazy routes');
});

test('P1 lazy import failures reload once across remounts, then show a working manual reload prompt',async()=>{
  sessionStorage.clear();
  const {default:Boundary,chunkReloadKey}=await load('react-app/shared/ChunkLoadBoundary.tsx');let reloads=0;
  const screen=()=>{
    const Lazy=React.lazy(()=>Promise.reject(new TypeError('Failed to fetch dynamically imported module: /assets/AuthPage-stale.js')));
    return ()=>React.createElement(Boundary,{reload:()=>reloads++},React.createElement(React.Suspense,{fallback:'loading'},React.createElement(Lazy)));
  };
  let v=await mountReact(vite,screen());
  try{assert.equal(reloads,1);assert.equal(sessionStorage.getItem(chunkReloadKey),'1');assert.match(v.host.textContent,/页面资源未能加载/);}finally{await v.dispose();}
  v=await mountReact(vite,screen());
  try{assert.equal(reloads,1,'the persisted session guard prevents a reload loop');assert.match(v.host.textContent,/自动刷新后仍未恢复/);await v.click(button(v.host,'重新加载页面'));assert.equal(reloads,2,'explicit user reload remains available');}finally{await v.dispose();}
});

test('P1 unrelated errors never auto-reload, and blocked session storage falls back without a reload loop',async()=>{
  const {reloadChunkOnce,isChunkLoadError}=await load('react-app/shared/ChunkLoadBoundary.tsx');let reloads=0;
  for(const message of ['Importing a module script failed.','error loading dynamically imported module','Loading chunk 2 failed.','Unable to preload CSS for /assets/style.css'])assert.equal(isChunkLoadError(new Error(message)),true,message);
  assert.equal(reloadChunkOnce(new Error('Business data unavailable'),()=>reloads++),false);
  const previous=Object.getOwnPropertyDescriptor(globalThis,'sessionStorage');
  Object.defineProperty(globalThis,'sessionStorage',{configurable:true,value:{getItem(){throw Error('storage blocked');}}});
  try{assert.equal(reloadChunkOnce(new Error('Failed to fetch dynamically imported module'),()=>reloads++),false);assert.equal(reloads,0);}finally{Object.defineProperty(globalThis,'sessionStorage',previous);}
});

test('P2 D8 world RadioGroup cards require explicit selection, retain the draft and explain returning from a new world',async()=>{
  const {default:Wizard}=await load('react-app/pages/CharacterWizardPage.jsx'),w=workspace(),v=await mountReact(vite,Wizard,{workspace:w});
  try{
    const group=v.host.querySelector('[data-slot=radio-group]');assert.equal(group.getAttribute('aria-label'),'选择生活世界');
    assert.equal(group.querySelectorAll('[role=radio]').length,2);assert.equal(group.querySelectorAll('[role=radio][aria-checked=true]').length,0);
    assert.equal(button(v.host,'继续，认识角色').disabled,true);
    await v.click(radio(group,'使用日常生活世界'));assert.equal(radio(group,'使用日常生活世界').getAttribute('aria-checked'),'true');
    assert.match(v.host.textContent,/将为 TA 准备一个日常生活世界/);assert.doesNotMatch(v.host.textContent,/世界创建与绑定将在原角色的准备流程中读回/);
    assert.equal(button(v.host,'继续，认识角色').disabled,false);
    const saved=Object.keys(localStorage).map(key=>[key,localStorage.getItem(key)]);
    await v.click(radio(group,'创建新的世界底稿'));
    assert.equal(radio(group,'创建新的世界底稿').getAttribute('aria-checked'),'true');assert.equal(radio(group,'使用日常生活世界').getAttribute('aria-checked'),'false');
    assert.equal(button(v.host,'继续，认识角色').disabled,true,'an unfinished world draft cannot advance with the previous daily selection');
    assert.match(group.textContent,/角色库.*继续创建.*返回本向导.*已发布的世界底稿/);
    assert.equal(v.host.querySelector('a[href="#/studio/edit/new?kind=world"]').textContent,'去创作中心创建');
    assert.deepEqual(Object.keys(localStorage).map(key=>[key,localStorage.getItem(key)]),saved,'the navigation choice preserves the saved role draft');assert.deepEqual(w.mutations,[]);
    await v.click(radio(group,'使用日常生活世界'));assert.equal(button(v.host,'继续，认识角色').disabled,false);assert.equal(group.querySelectorAll('[role=radio][aria-checked=true]').length,1);
  }finally{await v.dispose();}
});

test('P2 D8 world radio cards support keyboard selection through the shadcn Radix group',async()=>{
  const {default:Wizard}=await load('react-app/pages/CharacterWizardPage.jsx'),v=await mountReact(vite,Wizard,{workspace:workspace()});
  try{
    const daily=radio(v.host,'使用日常生活世界'),create=radio(v.host,'创建新的世界底稿');
    await v.click(daily);await act(async()=>{daily.focus();daily.dispatchEvent(new window.KeyboardEvent('keydown',{key:'ArrowRight',bubbles:true}));});await v.flush();
    assert.equal(document.activeElement,create);assert.equal(create.getAttribute('aria-checked'),'true');assert.equal(button(v.host,'继续，认识角色').disabled,true);
  }finally{await v.dispose();}
});

test('P2 library distinguishes missing service, saved-but-unbound purpose, configured model and unreadable configuration',async()=>{
  const {default:Library}=await load('react-app/pages/LibraryPage.jsx');
  for(const [configuration,state] of [[{endpoints:[],bindings:[]},'missing'],[{endpoints:[{id:'saved-provider'}],bindings:[]},'unbound'],[{endpoints:[{id:'saved-provider'}],bindings:[{role:'generic_llm',endpoint_id:'saved-provider',model_id:'model'}]},'bound'],[Error('offline'),'unknown']]){
    const v=await mountReact(vite,Library,{workspace:workspace(configuration)});
    try{
      const empty=v.host.querySelector('[data-slot=empty]');assert.ok(empty.querySelector('[data-testid=library-create-first]'));
      if(state==='missing'){assert.match(empty.textContent,/还没有连接通用模型/);assert.equal([...empty.querySelectorAll('a')].find(n=>n.textContent==='先连接模型').hash,'#/settings?section='+encodeURIComponent('服务地址'));}
      else if(state==='unbound'){assert.match(empty.textContent,/模型服务已保存，但尚未绑定通用模型用途/);assert.doesNotMatch(empty.textContent,/还没有连接通用模型|先连接模型/);assert.equal([...empty.querySelectorAll('a')].find(n=>n.textContent==='去绑定通用模型').hash,'#/settings?section='+encodeURIComponent('模型用途'));}
      else if(state==='bound'){assert.doesNotMatch(empty.textContent,/还没有连接通用模型|未绑定|先连接模型|去绑定通用模型|查看模型配置/);}
      else{assert.doesNotMatch(empty.textContent,/还没有连接通用模型|尚未绑定/);assert.ok([...empty.querySelectorAll('a')].find(n=>n.textContent==='查看模型配置'));}
    }finally{await v.dispose();}
  }
});

test('P2 world directory has one shadcn Empty CTA and shows filter guidance only for applied filters',async()=>{
  const {default:Studio}=await load('react-app/pages/StudioDirectory.jsx'),w=workspace(),v=await mountReact(vite,Studio,{workspace:w,props:{kind:'world'}});
  try{
    let empty=v.host.querySelector('[data-slot=empty]');assert.ok(empty.querySelector('.lucide-leaf'));assert.match(empty.textContent,/还没有世界底稿/);assert.doesNotMatch(v.host.textContent,/调整筛选|没有找到相关资源|新建资源/);
    assert.equal([...v.host.querySelectorAll('button')].filter(n=>n.textContent==='创建世界底稿').length,1);assert.equal(empty.querySelectorAll('button,a').length,1);
    await v.click(button(empty,'创建世界底稿'));assert.deepEqual(w.moves,['/studio/edit/new?kind=world']);
    await v.input(v.host.querySelector('[aria-label=搜索资源名称]'),'不存在');await v.click(v.host.querySelector('[aria-label=查询]'));
    empty=v.host.querySelector('[data-slot=empty]');assert.match(empty.textContent,/没有找到相关资源/);assert.match(empty.textContent,/清除筛选/);assert.equal(empty.querySelectorAll('button,a').length,1);assert.equal(button(v.host,'创建世界底稿'),undefined);
    await v.click(button(empty,'清除筛选'));empty=v.host.querySelector('[data-slot=empty]');assert.match(empty.textContent,/还没有世界底稿/);assert.equal(v.host.querySelector('[aria-label=搜索资源名称]').value,'');
    await v.click(v.host.querySelector('[role=checkbox]'));empty=v.host.querySelector('[data-slot=empty]');assert.match(empty.textContent,/没有找到相关资源/);await v.click(button(empty,'清除筛选'));assert.equal(v.host.querySelector('[role=checkbox]').getAttribute('aria-checked'),'false');assert.match(v.host.querySelector('[data-slot=empty]').textContent,/还没有世界底稿/);
  }finally{await v.dispose();}
});

test('P2 diagnostic tasks empty state links to creating a character; filtering retains clear-filter recovery',async()=>{
  const {default:Diagnostics}=await load('react-app/shared/DiagnosticsWorkspace.jsx'),w=workspace(),v=await mountReact(vite,Diagnostics,{workspace:w,props:{view:{openTaskDiagnostics(){}}}});
  try{
    let empty=v.host.querySelector('[data-slot=empty]');assert.match(empty.textContent,/暂时没有运行任务/);assert.match(empty.textContent,/可以先创建一个角色/);assert.equal(empty.querySelector('a').getAttribute('href'),'#/library/create');assert.equal(empty.querySelector('a').textContent,'去创建角色');
    await v.input(v.host.querySelector('[aria-label=搜索任务或角色]'),'不存在');empty=v.host.querySelector('[data-slot=empty]');assert.match(empty.textContent,/没有符合筛选的任务/);assert.equal(empty.querySelector('a'),null);await v.click(button(empty,'清除筛选'));assert.equal(v.host.querySelector('[data-slot=empty] a').getAttribute('href'),'#/library/create');
  }finally{await v.dispose();}
});
